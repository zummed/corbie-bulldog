package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/server"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// A log the demo writes and the test reads, which tells each time it is
// written.
type log struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	written chan struct{}
}

func (l *log) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tell()
	return l.buf.Write(p)
}

func (l *log) tell() {
	select {
	case l.written <- struct{}{}:
	default:
	}
}

func (l *log) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func TestDemoServesTheHouseWithNothingSetUp(t *testing.T) {
	// Nothing of Corbie is set up, and no key or variable is set.
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{"CORBIE_CONFIG", "CORBIE_STORE", "ANTHROPIC_API_KEY", "BULLDOG_URL"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("BULLDOG_LISTEN", "127.0.0.1:0")
	out := &log{written: make(chan struct{}, 1)}
	begun := make(chan server.Areas, 1)
	ctx, stop := context.WithCancel(context.Background())
	ended := make(chan error, 1)
	go func() {
		ended <- run(ctx, out, func(a server.Areas) {
			a.Requests.Watch(func(int64) { out.tell() })
			begun <- a
		})
	}()
	defer func() {
		stop()
		if err := <-ended; err != nil {
			t.Errorf("the demo ended with %v", err)
		}
	}()
	select {
	case <-begun:
	case err := <-ended:
		t.Fatalf("the demo ended with %v:\n%s", err, out)
	}
	moved, unwatch := work.Wall{}.Watch()
	defer unwatch()
	until := func(what string, done func() bool) {
		t.Helper()
		limit := time.NewTimer(testkit.Patience)
		defer limit.Stop()
		for !done() {
			select {
			case <-out.written:
			case <-moved:
			case <-limit.C:
				t.Fatalf("waited %v, and still not: %s\n%s", testkit.Patience, what, out)
			}
		}
	}
	until("the links are printed", func() bool {
		return strings.Contains(out.String(), "sam, open http://127.0.0.1:") && strings.Contains(out.String(), "kit, open")
	})
	kit, sam := signIn(t, out.String(), "kit"), signIn(t, out.String(), "sam")

	// The demo opens with the first part of the house's day played, each
	// step after the one before it: the last is the bins routine, run now
	// by Sam, which is answered once its flow has run.
	asked := map[string]int64{}
	until("the day is opened, the bins routine answered", func() bool {
		var list struct{ Requests []summary }
		sam.do(t, http.MethodGet, "/api/requests", "", &list)
		bins := false
		for _, r := range list.Requests {
			if r.Parent == 0 {
				asked[r.First] = r.Number
			}
			if r.Origin != nil && r.Origin.Kind == requests.ByRoutine && r.Origin.To == "bins" {
				bins = r.State == requests.Answered
			}
		}
		return bins
	})
	view := func(b browser, first string) request {
		t.Helper()
		n, ok := asked[first]
		if !ok {
			t.Fatalf("the day opened with no request %q: %v\n%s", first, asked, out)
		}
		var v request
		b.do(t, http.MethodGet, fmt.Sprintf("/api/requests/%d", n), "", &v)
		return v
	}

	// Kit asked about the socket, with a photo of it; Sam said yes to
	// cutting its circuit, the small-orders rule said yes to the socket,
	// and the request waits on Kit to say it came. The cut was carried to
	// Sam on the stand-in Telegram, to his Telegram identity.
	socket := view(sam, Socket)
	if socket.State != requests.Waiting || len(socket.On) != 1 || socket.On[0].Name != "kit" {
		t.Errorf("the socket's request is %s on %+v", socket.State, socket.On)
	}
	if files := socket.Entries[0].Files; len(files) != 1 || files[0].Name != "socket.png" {
		t.Errorf("Kit's entry holds the files %+v", files)
	} else if _, err := png.Decode(bytes.NewReader(sam.send(t, http.MethodGet, files[0].URL, ""))); err != nil {
		t.Errorf("the photo on Kit's entry is no PNG: %v", err)
	}
	if !slices.ContainsFunc(socket.Decisions, func(d decision) bool {
		return strings.Contains(d.Does, "Turn one circuit off") && d.Ended == decisions.Yes && d.By != nil && d.By.Name == "sam"
	}) {
		t.Errorf("the socket's decisions are %+v", socket.Decisions)
	}
	thread := socket.all()
	for _, part := range []string{"Order a part: approved by", "Tell me when the new socket arrives, and I'll fit it."} {
		if !strings.Contains(thread, part) {
			t.Errorf("the socket's thread does not hold %q:\n%s", part, thread)
		}
	}
	if !slices.ContainsFunc(socket.Entries, func(e entry) bool {
		return slices.ContainsFunc(e.Carried, func(c carried) bool {
			return c.Bridge == "telegram" && c.Place == "4471" && c.Person == "sam" && c.State == bridges.Sending
		})
	}) {
		t.Errorf("nothing of the socket's request is carried to Sam on Telegram: %+v", socket.Entries)
	}

	// Sam asked about the fan: the small-orders rule referred the order to
	// him, with its note, and the request waits on his yes.
	fan := view(sam, Fan)
	if fan.State != requests.Waiting || len(fan.On) != 1 || fan.On[0].Name != "sam" {
		t.Errorf("the fan's request is %s on %+v", fan.State, fan.On)
	}
	if d := fan.Decisions; len(d) != 1 || !d[0].Mine || !slices.Equal(d[0].Asked, []string{"sam"}) || len(d[0].Referrals) != 1 ||
		d[0].Referrals[0].Note != "£48 is more than I order alone." {
		t.Errorf("Sam is shown the fan's order as %+v", d)
	}

	// Kit asked after the lawn: the house asked the garden, a
	// sub-request, whose gardener had the groundsman look, and the
	// garden's answer is passed on to Kit.
	lawn := view(kit, Lawn)
	if lawn.State != requests.Answered || len(lawn.Subs) != 1 || lawn.Subs[0].Team != "garden" || lawn.Subs[0].State != requests.Answered {
		t.Errorf("the lawn's request is %s, with the sub-requests %+v", lawn.State, lawn.Subs)
	}
	if last := lawn.Entries[len(lawn.Entries)-1].Text; !strings.Contains(last, "The garden says:") || !strings.Contains(last, "I walked the lawn") {
		t.Errorf("Kit is answered %q", last)
	}

	// Kit asks in the chat what the demo has no part for, and the
	// stand-in's answer, saying what it plays, is her request's answer.
	var more struct{ Number int64 }
	kit.do(t, http.MethodPost, "/api/requests", `{"text": "is the kettle on?"}`, &more)
	var v request
	until("Kit's request is answered", func() bool {
		kit.do(t, http.MethodGet, fmt.Sprintf("/api/requests/%d", more.Number), "", &v)
		return v.State == requests.Answered
	})
	if last := v.Entries[len(v.Entries)-1].Text; last != Stock {
		t.Errorf("Kit's request was answered %q", last)
	}
	if strings.Contains(out.String(), "could not be opened") {
		t.Errorf("the demo says:\n%s", out)
	}
}

// A request as the screens are given it.
type (
	summary struct {
		Number int64
		Parent int64
		State  string
		First  string
		Origin *requests.Origin
		On     []struct{ Name string }
	}
	request struct {
		summary
		Entries   []entry
		Subs      []struct{ Team, State string }
		Decisions []decision
	}
	decision struct {
		Does, Ended string
		Mine        bool
		Asked       []string
		Referrals   []struct{ Note string }
		By          *struct{ Name string }
	}
)

// all gives the text of every entry, a line to each.
func (r request) all() string {
	var b strings.Builder
	for _, e := range r.Entries {
		b.WriteString(e.Text + "\n")
	}
	return b.String()
}

// An entry of a request, as the screens are given it, with where it was
// carried.
type (
	entry struct {
		Text    string
		Files   []struct{ Name, URL string }
		Carried []carried
	}
	carried struct{ Bridge, Place, Person, State string }
)

// A browser is a person's, signed in with the link the demo printed for
// them.
type browser struct {
	address, key string
}

// signIn opens the link the demo printed for a person, and keeps the key
// the answer gives the browser to keep.
func signIn(t *testing.T, log, person string) browser {
	t.Helper()
	_, link, _ := strings.Cut(log, "bulldog: "+person+", open ")
	link, _, _ = strings.Cut(link, "\n")
	address, _, ok := strings.Cut(link, "/?key=")
	if !ok {
		t.Fatalf("the demo printed no link for %s:\n%s", person, log)
	}
	res, err := http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	for _, c := range res.Cookies() {
		if c.Name == server.Cookie {
			return browser{address: address, key: c.Value}
		}
	}
	t.Fatalf("%s's link answered %d with no key to keep", person, res.StatusCode)
	return browser{}
}

// do sends a request as the person, and reads the answer into out.
func (b browser) do(t *testing.T, method, path, body string, out any) {
	t.Helper()
	data := b.send(t, method, path, body)
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("%s %s answered %s: %v", method, path, data, err)
	}
}

// send sends a request as the person, and gives what they are answered.
func (b browser) send(t *testing.T, method, path, body string) []byte {
	t.Helper()
	req, err := http.NewRequest(method, b.address+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: server.Cookie, Value: b.key})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s %s answered %d: %s", method, path, res.StatusCode, data)
	}
	return data
}

func TestDemoEveryModelIsTheStandIn(t *testing.T) {
	t.Parallel()
	files, err := house(`{ name: stand-in, secret: TEST_KEY }`)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if strings.Contains(string(text), "claude-code") {
			t.Errorf("%s still names a real model:\n%s", name, text)
		}
	}
	// A model written over several lines, under any name a team gives,
	// is the stand-in as much as one written in a row.
	for _, name := range []string{"office.yaml", "garden/team.yaml", "sparks.yaml"} {
		if !strings.Contains(string(files[name]), "stand-in") {
			t.Errorf("%s is given no stand-in:\n%s", name, files[name])
		}
	}
	doc := []byte("models:\n  usual:\n    name: opus\n    service: claude-code\n  quick: { name: haiku, service: claude-code }\nmodel:\n  name: sonnet\n  service: claude-code\n")
	var n yaml.Node
	if err := yaml.Unmarshal(doc, &n); err != nil {
		t.Fatal(err)
	}
	var model yaml.Node
	yaml.Unmarshal([]byte(`{ name: stand-in }`), &model)
	standInFor(&n, model.Content[0])
	out, _ := yaml.Marshal(&n)
	if strings.Contains(string(out), "claude-code") || strings.Count(string(out), "stand-in") != 3 {
		t.Errorf("the models are given as:\n%s", out)
	}
}

// The stand-in plays each part of the house's day, worker by worker,
// turn by turn: each turn is told what the one before it brought back,
// and either uses a tool or says words that hold what is wanted.
func TestDemoPlaysTheHouseDay(t *testing.T) {
	t.Parallel()
	type turn struct{ told, does string }
	given := func(said, open string) string {
		return "Request 9. Asked by Kit, a person of the office, who is in The house.\n\n" + open +
			"New for you:\n[Mon 5 Oct 2026 10:00 BST] Kit to The house: " + said + "\n\nYour last reply is your answer, said to Kit in the chat."
	}
	for _, part := range []struct {
		name, worker, asked string
		turns               []turn
	}{
		{"the socket, to its end", "steward.yaml", given("the kitchen socket sparked", ""), []turn{
			{"", "use fixer"}, {"The new socket is fitted, and circuit 4 is back on.", "The fixer says: The new socket is fitted"}}},
		{"the fixer at the socket", "fixer.yaml", "A socket sparked: make it safe, and mend it.", []turn{
			{"", "use sparks"}, {"4 kitchen ring: sockets by the window and the hob.", "use cut_power"}, {"Circuit 4 is off.", "use order_part"},
			{"Ordered 13 A double socket for £12.", "use ask_person"}, {"It came this morning.", "The new socket is fitted"}}},
		{"the cut refused", "fixer.yaml", "A socket sparked: make it safe, and mend it.", []turn{
			{"", "use sparks"}, {"4 kitchen ring", "use cut_power"}, {"Sam said no.", "Circuit 4 is still on"}}},
		{"the socket's order refused", "fixer.yaml", "A socket sparked: make it safe, and mend it.", []turn{
			{"", "use sparks"}, {"4 kitchen ring", "use cut_power"}, {"Circuit 4 is off.", "use order_part"}, {"Sam said no.", "no new socket is ordered"}}},
		{"the fan, said yes to", "fixer.yaml", "The bathroom's extractor fan has stopped: replace it.", []turn{
			{"", "use order_part"}, {"Ordered extractor fan, 100 mm for £48.", "is ordered for £48"}}},
		{"the fan, refused", "fixer.yaml", "The bathroom's extractor fan has stopped: replace it.", []turn{
			{"", "use order_part"}, {"Sam said no.", "is not ordered, as £48 was not agreed"}}},
		{"money", "steward.yaml", given("what have we spent this month?", ""), []turn{
			{"", "use clerk"}, {"This month the house spent £12.", "The clerk says: This month the house spent £12."}}},
		{"the clerk", "clerk.yaml", "what have we spent this month?", []turn{{"", "spent £12"}}},
		{"the walk round", "steward.yaml", given("Walk round: what is still open?",
			"The open requests of The house, newest first; you act on none of them:\n- Request 2, asked by Sam, waiting: the bathroom fan has stopped\n\n"),
			[]turn{{"", "- Request 2, asked by Sam, waiting: the bathroom fan has stopped"}}},
		{"the shed", "steward.yaml", given("is the shed locked?", ""), []turn{
			{"", "use ask_team"}, {"The garden, answered by its gardener:\n\nThe shed is dry inside and locked.", "The garden says: The shed is dry inside and locked."}}},
		{"the garden, not answering", "steward.yaml", given("is the lawn ok?", ""), []turn{
			{"", "use ask_team"}, {"No answer came within 7 days.", "The garden says: No answer came within 7 days."}}},
		{"the gardener", "garden/gardener.yaml", given("can the hedge be trimmed?", ""), []turn{
			{"", "use groundsman"}, {"I trimmed the hedge along the lane.", "The groundsman has been out. I trimmed the hedge along the lane."}}},
		{"the groundsman at the hedge", "garden/groundsman.yaml", "can the hedge be trimmed?", []turn{{"", "I trimmed the hedge"}}},
		{"the groundsman, at no part he knows", "garden/groundsman.yaml", "the path was washed and pushed back", []turn{{"", "I did the heavy work asked of me."}}},
		{"the power, which is no money", "steward.yaml", given("the power is off in the kitchen", ""), []turn{{"", Stock}}},
		{"a fancy shower, which is no fan", "steward.yaml", given("the shower is broken, and so is the fancy light", ""), []turn{{"", Stock}}},
	} {
		t.Run(part.name, func(t *testing.T) {
			text, err := fs.ReadFile(examples.House(), part.worker)
			if err != nil {
				t.Fatal(err)
			}
			var worker struct{ System string }
			if err := yaml.Unmarshal(text, &worker); err != nil {
				t.Fatal(err)
			}
			said := func(text string) testkit.Message {
				return testkit.Message{Role: "user", Content: []map[string]any{{"type": "text", "text": text}}}
			}
			messages := []testkit.Message{said(part.asked)}
			for i, tn := range part.turns {
				if i > 0 {
					messages = append(messages, said(tn.told))
				}
				r := play(testkit.Turn{System: worker.System, Messages: messages, Number: i + 1})
				got := r.Text
				if len(r.Calls) == 1 {
					got = "use " + r.Calls[0].Name
				}
				if !strings.Contains(got, tn.does) {
					t.Fatalf("at turn %d, told %q, the %s gave %q, not %q", i+1, tn.told, part.worker, got, tn.does)
				}
			}
		})
	}
}
