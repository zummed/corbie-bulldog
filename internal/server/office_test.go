package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// copyFolder copies a folder whole, links and all, as a person copying an
// office does.
func copyFolder(t *testing.T, from, to string) {
	t.Helper()
	if out, err := exec.Command("cp", "-a", from, to).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

// storeStamp gives the store file's size and time.
func storeStamp(t *testing.T, dir string) string {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, shipped.Dir, StoreFile))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(info.ModTime(), info.Size())
}

func TestServerCopiedOfficeRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.listening(b.serve()).stop(); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(t.TempDir(), "copy")
	copyFolder(t, b.dir, copied)
	before := storeStamp(t, copied)
	runs := len(b.listed(b.identity(), nil))
	original := b.dir
	b.dir = copied
	refused := b.serve().wait()
	if refused == nil || !strings.Contains(refused.Error(), "made in "+original) || !strings.Contains(refused.Error(), "remove "+copied+"/.bulldog/") ||
		!strings.Contains(refused.Error(), "corbie bulldog init "+copied) {
		t.Fatalf("the copy was served, or refused as %v", refused)
	}
	if storeStamp(t, copied) != before {
		t.Error("the copy's store was opened")
	}
	if len(b.listed(b.identity(), nil)) != runs {
		t.Error("serving the copy started a run")
	}
	// Check reads it as it reads any office.
	var listing strings.Builder
	if err := Check(copied, &listing); err != nil || !strings.Contains(listing.String(), "The house, the top team") {
		t.Errorf("the copy is checked as %v:\n%s", err, listing.String())
	}
}

func TestServerMovedOfficeTakenByInit(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	n := s.chat("kit", "", "is the kettle on?")
	s.answered(n)
	if err := s.stop(); err != nil {
		t.Fatal(err)
	}
	was := b.identity()
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(b.dir, moved); err != nil {
		t.Fatal(err)
	}
	b.dir = moved
	if err := b.serve().wait(); err == nil || !strings.Contains(err.Error(), "corbie bulldog init") {
		t.Fatalf("the moved office was served, or refused as %v", err)
	}
	_, said, err := readied(t, moved, Readying{Make: true, Move: true})
	if err != nil || !strings.Contains(said, "is taken as that office moved here") {
		t.Fatalf("init in the moved office said %q, %v", said, err)
	}
	s = b.serve()
	s.get(n)
	if b.identity() != was {
		t.Errorf("the identity is %s after the move, and was %s", b.identity(), was)
	}
}

func TestServerSecondServerOfOfficeRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	first := b.serve()
	first.areasOf()
	live := map[string]string{work.LabelKind: work.KindLive}
	second := b.start()
	b.until("the second server waits on the first's run", func() bool { return second.logged("waiting up to a minute") })
	b.clock.Move(work.Renewal)
	err := second.wait()
	if err == nil || !strings.Contains(err.Error(), "controller controller") || !strings.Contains(err.Error(), "stop the server already serving it") ||
		!strings.Contains(err.Error(), "remove its .bulldog/") {
		t.Fatalf("the second server ended with %v", err)
	}
	if second.logged("listening on") || len(b.listed(b.identity(), live)) != 1 {
		t.Errorf("the second server listened or started a run:\n%s", strings.Join(second.lines(), "\n"))
	}
	// The first serves on.
	first.answered(first.chat("kit", "", "is the kettle on?"))
	// Stopped, it ends its run, and a server started at once does not
	// wait out the minute.
	if err := first.stop(); err != nil {
		t.Fatal(err)
	}
	third := b.serve()
	third.areasOf()
	// One killed leaves its run going: a start waits, and goes on once the
	// run has ended itself.
	if err := third.stop(); err != nil {
		t.Fatal(err)
	}
	b.until("the stopped server's run has ended", func() bool {
		for _, r := range b.listed(b.identity(), map[string]string{work.LabelKind: work.KindLive}) {
			if !r.HasEnded() {
				return false
			}
		}
		return true
	})
	b.leftLive()
	fourth := b.start()
	b.until("a start waits on the run left", func() bool { return fourth.logged("waiting up to a minute") })
	b.clock.Move(work.Quiet)
	b.until("the start goes on", func() bool { return fourth.begun() != nil || fourth.ended() })
	fourth.areasOf()
}

func TestServerRefusedSecondServerLeavesTheStoreUntouched(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.serve().stop(); err != nil {
		t.Fatal(err)
	}
	// A bridge's run asked for and never named, as a server stopped in
	// between leaves it, is what a server taking the office asks for again.
	db := raw(t, b.dir)
	if res, err := db.Exec(`UPDATE bridges_bridges SET claimed = 1, run = ''`); err != nil {
		t.Fatal(err)
	} else if n, err := res.RowsAffected(); err != nil || n == 0 {
		t.Fatalf("no bridge's run is kept: %d, %v", n, err)
	}
	before := rows(t, db)
	// Another server serves the office: its run is renewed while this one
	// waits on it.
	run := b.leftLive()
	second := b.start()
	b.until("the second server waits on the other's run", func() bool { return second.logged("waiting up to a minute") })
	if _, err := b.corbie.Send(ctx, run, shipped.Renew, nil, work.Renewal); err != nil {
		t.Fatal(err)
	}
	if err := second.wait(); err == nil || !strings.Contains(err.Error(), "stop the server already serving it") {
		t.Fatalf("the second server ended with %v", err)
	}
	if after := rows(t, db); !slices.Equal(after, before) {
		t.Errorf("the refused server changed the store:\n%s\nfrom\n%s", strings.Join(after, "\n"), strings.Join(before, "\n"))
	}
	if second.logged("The house") {
		t.Errorf("the refused server read the office:\n%s", strings.Join(second.lines(), "\n"))
	}
}

// rows gives every row of every table of a store, as text, in order.
func rows(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var tables []string
	listed, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	for listed.Next() {
		var name string
		if err := listed.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := listed.Close(); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, table := range tables {
		got, err := db.Query(`SELECT * FROM "` + table + `"`)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := got.Columns()
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for got.Next() {
			values := make([]any, len(columns))
			to := make([]any, len(columns))
			for i := range values {
				to[i] = &values[i]
			}
			if err := got.Scan(to...); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, fmt.Sprintf("%s %v", table, values))
		}
		if err := got.Close(); err != nil {
			t.Fatal(err)
		}
		slices.Sort(lines)
		out = append(out, lines...)
	}
	return out
}

func TestServerOlderStoreRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.serve().stop(); err != nil {
		t.Fatal(err)
	}
	for _, area := range []string{Area, requests.Area} {
		db := raw(t, b.dir)
		if _, err := db.Exec(`UPDATE parts SET version = version - 1 WHERE area = ?`, area); err != nil {
			t.Fatal(err)
		}
		err := b.serve().wait()
		if err == nil || !strings.Contains(err.Error(), "remove .bulldog/bulldog.db and start again") {
			t.Errorf("a store whose %s were kept by an older Bulldog was taken as %v", area, err)
		}
		if _, err := db.Exec(`UPDATE parts SET version = version + 1 WHERE area = ?`, area); err != nil {
			t.Fatal(err)
		}
	}
}

// raw opens the store as a file of SQL, as nothing but the test reads it.
func raw(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, shipped.Dir, StoreFile)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestServerStoreIsOneFile(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	n := s.chat("kit", "", "is the kettle on?")
	s.answered(n)
	keys := map[string]string{}
	for _, p := range []string{"sam", "kit"} {
		keys[p], _ = keyOf(b.store(), p)
	}
	if err := s.stop(); err != nil {
		t.Fatal(err)
	}
	in, _ := os.ReadDir(filepath.Join(b.dir, shipped.Dir))
	var dbs []string
	for _, e := range in {
		if strings.HasSuffix(e.Name(), ".db") {
			dbs = append(dbs, e.Name())
		}
	}
	if len(dbs) != 1 || dbs[0] != StoreFile {
		t.Errorf(".bulldog/ holds the databases %v", dbs)
	}
	s = b.serve()
	s.get(n)
	for p, key := range keys {
		if again, err := keyOf(b.store(), p); err != nil || again != key || key == "" {
			t.Errorf("%s's key is %q after a restart, and was %q: %v", p, again, key, err)
		}
	}
	if runs, err := s.areasOf().Work.Runs(n); err != nil || len(runs) == 0 {
		t.Errorf("the request's runs after a restart: %v, %v", runs, err)
	}
}

// started gives how many runs the office has at the top, but the
// server's own.
func (b *bench) started() int {
	b.t.Helper()
	return len(slices.DeleteFunc(b.listed(b.identity(), nil), func(r *client.Run) bool { return r.Labels[work.LabelKind] == work.KindLive }))
}

// signsIn says whether a person's printed link signs them in.
func (s *serving) signsIn(person string) bool {
	s.b.t.Helper()
	key, err := keyOf(s.b.store(), person)
	if err != nil {
		s.b.t.Fatal(err)
	}
	r := s.nobody().get("/?key=" + key)
	return r.cookie() != nil && s.holding(r).whoIs() == person
}

func TestServerUnreadOfficeServedAllTheSame(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.serve().stop(); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(filepath.Join(b.dir, office.OfficeFile))
	broken := strings.Replace(string(text), "takes: steward\n", "", 1)
	b.write(map[string]string{office.OfficeFile: broken})
	// Corbie makes a run once it takes its start, so every run the first
	// server sent is listed before the runs are counted.
	var sent int
	if err := b.store().QueryRow(`SELECT count(*) FROM work_runs WHERE status != 'unsent'`).Scan(&sent); err != nil {
		t.Fatal(err)
	}
	b.until("the first server's runs are listed", func() bool { return b.started() >= sent })
	runs := b.started()
	s := b.serve()
	if !s.logged("does not read well") || !s.logged("a problem: office.yaml") || s.logged("open http") {
		t.Errorf("an office that does not read was logged as:\n%s", strings.Join(s.lines(), "\n"))
	}
	if r := s.nobody().get("/?key=anything"); r.cookie() != nil {
		t.Error("/?key=anything gave a key while the office does not read")
	}
	if s.signsIn("sam") {
		t.Error("Sam signs in while the office does not read")
	}
	b.clock.Move(time.Second)
	if b.started() != runs {
		t.Errorf("an office that does not read started a run: %v", kinds(b.listed(b.identity(), nil)))
	}
	// Mended, it reads, and the links are given, and Sam signs in.
	b.write(map[string]string{office.OfficeFile: string(text)})
	b.clock.Move(time.Second)
	b.until("the links are given once it reads", func() bool { return s.logged("sam, open") && s.logged("kit, open") })
	if !s.logged("The house: 2 teams") {
		t.Errorf("the office's line was not said once it read:\n%s", strings.Join(s.lines(), "\n"))
	}
	if !s.signsIn("sam") {
		t.Error("Sam does not sign in once the office reads")
	}
	s.answered(s.chat("kit", "", "is the kettle on?"))
}

func TestServerBrokenBridgeStopsOnlyItself(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.write(map[string]string{"bridges/telegram/out.yaml": ""})
	s := b.serve()
	if !s.logged("a problem: bridges/telegram") || !s.logged("sam, open") {
		t.Errorf("the house with a broken bridge was logged as:\n%s", strings.Join(s.lines(), "\n"))
	}
	if !s.signsIn("sam") {
		t.Error("Sam does not sign in to the house with a broken bridge")
	}
	s.answered(s.chat("kit", "", "is the kettle on?"))
	if runs := b.listed(b.identity(), map[string]string{"bridge": "telegram"}); len(runs) != 0 {
		t.Errorf("the broken bridge was run: %v", kinds(runs))
	}
}

func TestServerBadEditKeepsServing(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	text, _ := os.ReadFile(filepath.Join(b.dir, office.OfficeFile))
	b.write(map[string]string{office.OfficeFile: string(text) + "listen: 127.0.0.1:7499\n"})
	b.clock.Move(time.Second)
	b.until("the problem is logged", func() bool { return s.logged("a problem: office.yaml") && s.logged("listen") })
	if s.areasOf().Keeper.Office() == nil {
		t.Fatal("the office is no longer served")
	}
	s.answered(s.chat("kit", "", "is the kettle on?"))
	// Its screens work, and show the problem.
	s.as("kit").get("/").is(http.StatusOK)
	if me := s.as("sam").get("/api/me").is(http.StatusOK); !strings.Contains(string(me.body), "listen") {
		t.Errorf("Sam is not shown the problem: %s", me.body)
	}
}

func TestServerNewPersonGivenOneLink(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	text, _ := os.ReadFile(filepath.Join(b.dir, office.OfficeFile))
	b.write(map[string]string{office.OfficeFile: strings.Replace(string(text), "everyone:\n", "everyone:\n  jo: { name: Jo }\n", 1)})
	b.clock.Move(time.Second)
	b.until("Jo is given a link", func() bool { return s.logged("jo, open") })
	links := 0
	for _, l := range s.lines() {
		if strings.Contains(l, ", open http") {
			links++
		}
	}
	if links != 3 {
		t.Errorf("the links given are:\n%s", strings.Join(s.lines(), "\n"))
	}
}

func TestServerReusedNameGetsNewKey(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	db := b.store()
	kit, _ := keyOf(db, "kit")
	text, _ := os.ReadFile(filepath.Join(b.dir, office.OfficeFile))
	// A reading that fails destroys nothing.
	b.write(map[string]string{office.OfficeFile: strings.Replace(string(text), "  kit:", "  kit::", 1)})
	b.clock.Move(time.Second)
	b.until("the bad reading is logged", func() bool { return s.logged("a problem: office.yaml") })
	if again, err := keyOf(db, "kit"); err != nil || again != kit {
		t.Errorf("a reading that failed destroyed Kit's key: %v", err)
	}
	// A good one without her destroys it.
	without := strings.Replace(strings.Replace(string(text), "people: [sam, kit]", "people: [sam]", 1), "  kit: { name: Kit, title: Lives here., telegram: \"4472\" }\n", "", 1)
	b.write(map[string]string{office.OfficeFile: without})
	b.clock.Move(time.Second)
	b.until("Kit's key is destroyed", func() bool { return s.logged("kit is no longer in the office") })
	if _, err := keyOf(db, "kit"); err == nil {
		t.Error("Kit's key outlived her")
	}
	// Written again, she is given a new one.
	b.write(map[string]string{office.OfficeFile: string(text)})
	b.clock.Move(time.Second)
	b.until("Kit is given a link again", func() bool { return strings.Count(strings.Join(s.lines(), "\n"), "kit, open") == 2 })
	if again, err := keyOf(db, "kit"); err != nil || again == kit {
		t.Errorf("Kit, written again, has the key %q, as before: %v", again, err)
	}
}

func TestServerLiveRunNobodyRunsRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.serve().stop(); err != nil {
		t.Fatal(err)
	}
	// A run of the office's own kind that nothing renews and nothing ends,
	// as one is whose controller has stopped.
	stuck := "statechart: live\ninitial: serving\nstates:\n  serving:\n    on: { renew: serving, stop: gone }\n  gone: { end: done }\n"
	draft, err := machine.Parse([]byte(stuck), nil)
	if err != nil {
		t.Fatal(err)
	}
	left, err := b.corbie.Start(ctx, draft, client.Start{Labels: map[string]string{work.LabelBulldog: b.identity(), work.LabelKind: work.KindLive}})
	if err != nil {
		t.Fatal(err)
	}
	b.until("the run is listed as going", func() bool {
		return slices.ContainsFunc(b.listed(b.identity(), nil), func(r *client.Run) bool { return r.ID == left && !r.HasEnded() })
	})
	runs := len(b.listed(b.identity(), nil))
	s := b.start()
	b.until("the start waits on the run", func() bool { return s.logged("waiting up to a minute") })
	b.clock.Move(work.Quiet + work.Again + time.Second)
	err = s.wait()
	if err == nil || !strings.Contains(err.Error(), "is no controller running it?") {
		t.Fatalf("a start that found a run nobody runs ended with %v", err)
	}
	if s.logged("listening on") || len(b.listed(b.identity(), nil)) != runs {
		t.Errorf("the refused start listened or started a run:\n%s", strings.Join(s.lines(), "\n"))
	}
}
