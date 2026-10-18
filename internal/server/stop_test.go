package server

import (
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// hold has the stand-in hold every turn until the test lets it go, and
// gives how many it holds and the func that lets them go, which the test's
// end calls too.
func (b *bench) hold() (held func() int, let func()) {
	gate := make(chan struct{})
	var mu sync.Mutex
	count := 0
	b.says(func(turn testkit.Turn) testkit.Reply {
		mu.Lock()
		count++
		mu.Unlock()
		b.poke()
		<-gate
		return testkit.Say("done")
	})
	var once sync.Once
	b.t.Cleanup(func() { once.Do(func() { close(gate) }) })
	return func() int {
			mu.Lock()
			defer mu.Unlock()
			return count
		}, func() {
			once.Do(func() { close(gate) })
		}
}

func TestServerShutdownWaitsForEverything(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold()
	defer let()
	s := b.serve()
	s.chat("kit", "", "the kitchen socket sparked")
	b.until("the steward's turn is held", func() bool { return held() > 0 })
	address := s.lines()[index(s.lines(), "listening on")][len("bulldog: listening on "):]
	if err := s.stop(); err != nil {
		t.Fatalf("a server stopped with a request running ended with %v", err)
	}
	// The store is closed: the last to close it folds its log into it and
	// removes the log, and nothing has it open.
	store := filepath.Join(b.dir, shipped.Dir, StoreFile)
	for _, left := range []string{store + "-wal", store + "-shm"} {
		if _, err := os.Stat(left); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is left once the server stopped: %v", left, err)
		}
	}
	closed := storeStamp(t, b.dir)
	logged := s.lines()[len(s.lines())-1]
	// It listens no more.
	if _, err := http.Get("http://" + address + "/"); err == nil {
		t.Error("the stopped server still answers")
	}
	// The run it left goes on in Corbie and ends there, and nothing of the
	// server takes it: what it kept of the run is as it was.
	let()
	steward := b.listed(b.identity(), map[string]string{work.LabelKind: work.KindWork})
	if len(steward) != 1 {
		t.Fatalf("the runs of the request are %v", kinds(steward))
	}
	b.corbie.Ended(t, steward[0].ID)
	// Nothing reached the store once it was closed, though the run the
	// server noted ended meanwhile, and nothing more was logged.
	if again := storeStamp(t, b.dir); again != closed {
		t.Errorf("the store was written once the server stopped: %s, and %s when it stopped", again, closed)
	}
	if _, err := os.Stat(store + "-wal"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the store was opened once the server stopped: %v", err)
	}
	if lines := s.lines(); lines[len(lines)-1] != logged {
		t.Errorf("the server logged once it stopped:\n%s", strings.Join(lines, "\n"))
	}
	var status string
	if err := raw(t, b.dir).QueryRow(`SELECT status FROM work_runs WHERE name = ?`, steward[0].ID).Scan(&status); err != nil || status != "going" {
		t.Errorf("the stopped server's note of the run is %q, %v", status, err)
	}
	// Its own run is ended.
	for _, r := range b.listed(b.identity(), map[string]string{work.LabelKind: work.KindLive}) {
		if r := b.corbie.Ended(t, r.ID); r.Status != "cancelled" {
			t.Errorf("the server's own run ended %s", r.Status)
		}
	}
	// Served again, it takes up the run's ending.
	s = b.serve()
	s.answered(1)
}

func TestServerViewerRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	viewer := client.New(nil, b.corbie.Client.Controller, b.corbie.Client.Store)
	s := b.serve(func(o *Options) { o.Corbie = viewer })
	err := s.wait()
	if err == nil || !strings.Contains(err.Error(), "corbie client card") || s.logged("on the Corbie") {
		t.Errorf("a viewer was taken as %v:\n%s", err, strings.Join(s.lines(), "\n"))
	}
}

func TestServerStoreAddressShownWithoutWhatItCarries(t *testing.T) {
	t.Parallel()
	for address, shown := range map[string]string{
		"/home/sam/.config/corbie/store":                   "/home/sam/.config/corbie/store",
		"s3://key:secret@bucket/corbie?region=eu-west-2":   "s3://bucket/corbie",
		"https://sam:hunter2@store.example:8372/house?x=1": "https://store.example:8372/house",
		"s3://%zz:secret@bucket":                           "a store whose address cannot be read",
		"://secret":                                        "a store whose address cannot be read",
	} {
		if got := Shown(address); got != shown {
			t.Errorf("%q is shown as %q", address, got)
		}
	}
}

func TestServerLinksUseURL(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve(func(o *Options) { o.URL = "https://house.example/" })
	for _, l := range s.lines() {
		if strings.Contains(l, ", open ") && !strings.Contains(l, ", open https://house.example/?key=") {
			t.Errorf("a link is made without the url: %s", l)
		}
	}
	if !s.logged("sam, open https://house.example/?key=") {
		t.Errorf("no link for Sam:\n%s", strings.Join(s.lines(), "\n"))
	}
	// A one-time link is made with the url too, and never with the
	// address the server listens on.
	l := s.linkFor("kit")
	if !strings.HasPrefix(l.url(), "https://house.example/signin/") || strings.Contains(l.text, "127.0.0.1") {
		t.Errorf("a one-time link is sent as %q", l.text)
	}
}

func TestServerListenAddressTakenAtEachStart(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	for _, port := range []string{free(t), free(t)} {
		s := b.serve(func(o *Options) { o.Listen = port })
		if !s.logged("listening on " + port) {
			t.Errorf("told to listen on %s, it said:\n%s", port, strings.Join(s.lines(), "\n"))
		}
		if err := s.stop(); err != nil {
			t.Fatal(err)
		}
	}
}

// free gives an address nothing listens on.
func free(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestServerTakesTLSFromAProxy(t *testing.T) {
	t.Parallel()
	r, _ := http.NewRequest("GET", "http://house.example/", nil)
	if overTLS(r) {
		t.Error("a request with no header was taken as over TLS")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !overTLS(r) {
		t.Error("a request a proxy says came over TLS was not taken as such")
	}
}
