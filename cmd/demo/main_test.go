package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// A written is what the demo prints, for the test to read.
type written struct {
	mu   sync.Mutex
	text strings.Builder
}

func (w *written) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.text.Write(p)
}

func (w *written) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.text.String()
}

// A person is someone signed in on the screens.
type person struct {
	t       *testing.T
	address string
	browser *http.Client
}

// do sends a request of the screens' API, and reads its answer into into.
func (p person) do(method, path string, body any, into any) {
	p.t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		p.t.Fatal(err)
	}
	r, err := http.NewRequest(method, p.address+path, bytes.NewReader(data))
	if err != nil {
		p.t.Fatal(err)
	}
	got, err := p.browser.Do(r)
	if err != nil {
		p.t.Fatal(err)
	}
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		p.t.Fatalf("%s %s gave %s", method, path, got.Status)
	}
	if err := json.NewDecoder(got.Body).Decode(into); err != nil {
		p.t.Fatal(err)
	}
}

// signIn presses the button of a person's key link.
func signIn(t *testing.T, address, link string) person {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	p := person{t, address, &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	pressed, err := p.browser.Post(link, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	pressed.Body.Close()
	if pressed.StatusCode != http.StatusSeeOther {
		t.Fatalf("pressing %s gave %s", link, pressed.Status)
	}
	return p
}

// TestDemoPlaysKitsRequest runs the demo on a Corbie of the test's own.
// Sam signs in with the link it prints and says yes to cutting the power,
// and Kit's request is answered.
func TestDemoPlaysKitsRequest(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	model := testkit.NewModel(t, play)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := "http://" + l.Addr().String()
	out := &written{}
	ctx, stop := context.WithCancel(context.Background())
	ended := make(chan error, 1)
	go func() { ended <- run(ctx, corbie.Client, model, l, out) }()
	defer func() {
		stop()
		if err := <-ended; err != nil {
			t.Errorf("the demo ended with %v", err)
		}
	}()

	links := regexp.MustCompile(`(?m)^  (Sam|Kit) +(\S+)$`)
	corbie.Until(t, "the demo prints the key links", func() bool { return len(links.FindAllStringSubmatch(out.String(), -1)) == 2 })
	printed := links.FindAllStringSubmatch(out.String(), -1)
	if printed[0][1] != "Sam" || !strings.HasPrefix(printed[0][2], address+"/key?") {
		t.Fatalf("the demo printed:\n%s", out)
	}
	sam := signIn(t, address, printed[0][2])

	// The fixer's cut_power waits on Sam, after small-orders.js and the
	// clerk passed.
	type card struct {
		Ref, Kind, Said string
		Choices         []string
	}
	type row struct{ ID, Team, Status string }
	var list struct {
		Waiting  []card
		Requests []row
	}
	corbie.Until(t, "a decision waits on Sam", func() bool {
		sam.do(http.MethodGet, "/api/requests", nil, &list)
		return len(list.Waiting) > 0
	})
	cut := list.Waiting[0]
	if cut.Kind != "decision" || !strings.Contains(cut.Said, "cut_power") || !strings.Contains(cut.Said, "It costs nothing, so Sam decides.") {
		t.Fatalf("Sam is asked %+v", cut)
	}
	sam.do(http.MethodPost, "/api/waits/"+cut.Ref+"/answer", map[string]string{"answer": "yes"}, &struct{}{})

	// The order of a socket for £12 passes small-orders.js, so nothing more
	// is asked of Sam, and Kit's request is answered.
	var kits row
	corbie.Until(t, "Kit's request is answered", func() bool {
		sam.do(http.MethodGet, "/api/requests", nil, &list)
		for _, r := range list.Requests {
			if r.Team == "office" {
				kits = r
			}
		}
		return kits.Status != "working" && kits.Status != "waiting" && kits.Status != ""
	})
	var view struct {
		Status string
		Source struct{ Said string }
		Answer struct{ Text string }
		Forks  []struct{ Kind, Status, Answer string }
	}
	sam.do(http.MethodGet, "/api/requests/"+kits.ID, nil, &view)
	if view.Status != "answered" || view.Source.Said != socket || !strings.Contains(view.Answer.Text, "ordered a new socket for £12") || len(list.Waiting) > 0 {
		t.Fatalf("Kit's request: %+v, and Sam is asked %+v", view, list.Waiting)
	}
	forks := map[string]string{}
	for _, f := range view.Forks {
		forks[f.Kind] = f.Status + " " + f.Answer
	}
	if len(view.Forks) != 2 || !strings.HasPrefix(forks["team"], "answered") || forks["decision"] != "answered yes (Sam)" {
		t.Fatalf("Kit's request forks: %+v", view.Forks)
	}
	if said := out.String(); !strings.HasSuffix(said, "what the house asks him.\n\n") {
		t.Fatalf("the demo said more than it prints when it starts:\n%s", said)
	}
}
