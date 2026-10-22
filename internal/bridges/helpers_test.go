package bridges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// secret is the office's secret in every test.
var secret = []byte("the office's secret in the tests")

// A stand-in bridge hears what the test writes into its feed, a file of
// { things, refuse }. Its in hands over the things from its cursor on,
// each time its rest of a minute falls due, and fails with refuse when the
// feed gives one. Its out sends nothing: it gives { sent } for what it is
// given, and refuses a message that says "refuse".
var standIn = map[string]string{
	"in.yaml": `statechart: stand-in
keep: 1d
input: { settings: map, cursor: any }
initial: hearing
states:
  hearing:
    job:
      script: ./hear.js
      input: { feed: "${input.settings.feed}", cursor: "${states.hearing.output?.cursor ?? input.cursor}" }
      allow: ["fs.read:${input.settings.feed}"]
    on: { done: resting, failed: resting }
  resting:
    timers: { again: 1m }
    on: { again: hearing }
`,
	"hear.js": `const feed = fs.read(input.feed, { as: "json" })
if (feed.refuse) throw new Error(feed.refuse)
const from = input.cursor === null || input.cursor === undefined ? 0 : input.cursor
const things = feed.things.slice(from).map(t => Object.assign({}, t, {
  files: (t.files || []).map(f => ({ name: f.name, data: blobs.make(f.text, { type: "text/plain" }) })),
}))
if (things.length) emit("heard", { things: things, cursor: feed.things.length })
return { cursor: feed.things.length }
`,
	"out.yaml": `flow: stand-in-out
input: { settings: map, to: string, direct: boolean, text: string, choices: any, ref: any, answers: any }
stages:
  say:
    job:
      script: ./say.js
      input: { to: "${input.to}", text: "${input.text}", ref: "${input.ref}", answers: "${input.answers}" }
output: ${stages.say.output}
`,
	"say.js": `if (input.text.indexOf("refuse") >= 0) throw new Error("The stand-in refuses the message.")
return { sent: input.to + "/" + (input.ref || input.answers || "message") }
`,
}

// A world is the test office, served by an engine and the bridges on a
// Corbie of the test's own. The office's lead is Sam, so every request of
// the office is a question to Sam; the garden's lead is Kit. Its bridge
// pager is a stand-in that reaches people directly, where Sam and Kit ask
// the office directly, a guest asks it in #help, and a person asks the
// garden in #garden.
type world struct {
	t      *testing.T
	corbie *testkit.Corbie
	id     string                      // the office's id
	dir    string                      // the office's directory
	feeds  string                      // the stand-ins' feeds
	things map[string][]map[string]any // what each stand-in has heard, by bridge

	mu      sync.Mutex
	engine  *requests.Engine
	bridges *Bridges
	stop    func()
}

// theOffice is the test office's office.yaml. FEEDS stands for the
// directory of the stand-ins' feeds.
const theOffice = `name: The test office
about: Everything asked here.
everyone:
  sam: { name: Sam, pager: u-sam }
  kit: { name: Kit, pager: u-kit }
bridges:
  pager:
    feed: FEEDS/pager.json
    places:
      direct: {}
      "#help": { guests: true }
      "#garden": { team: garden }
people: [sam, kit]
lead: sam
`

// newWorld writes the test office, changed as the test changes it, and
// serves it.
func newWorld(t *testing.T, change func(files map[string]string)) *world {
	t.Helper()
	w := newOffice(t, change)
	if problems := w.reading().Office.Problems; len(problems) > 0 {
		t.Fatalf("the office has problems:\n%v", office.Problems(problems))
	}
	w.start()
	return w
}

// newOffice writes the test office, changed as the test changes it, with
// an empty feed for each bridge, and does not serve it yet.
func newOffice(t *testing.T, change func(files map[string]string)) *world {
	t.Helper()
	w := &world{t: t, corbie: testkit.NewCorbie(t), feeds: t.TempDir(), things: map[string][]map[string]any{}}
	files := map[string]string{
		"office.yaml":      theOffice,
		"garden/team.yaml": "name: The garden\nabout: The beds and the shed.\npeople: [kit]\nlead: kit\n",
	}
	for name, text := range standIn {
		files["bridges/pager/"+name] = text
	}
	if change != nil {
		change(files)
	}
	for name, text := range files {
		files[name] = strings.ReplaceAll(text, "FEEDS", w.feeds)
	}
	w.dir = testkit.Dir(t, files)
	r := w.reading()
	w.id = r.Office.ID
	for _, br := range r.Office.Bridges {
		w.write(br.Name)
	}
	return w
}

// reading reads the office as the program does: the office, then its
// bridges and its request machines.
func (w *world) reading() *requests.Reading {
	w.t.Helper()
	o, err := office.Read(w.dir, shipped.Files)
	if err != nil {
		w.t.Fatal(err)
	}
	Check(o)
	return requests.NewReading(o)
}

// start serves the office: an engine and the bridges, each running until
// the test ends or stops them.
func (w *world) start() {
	w.t.Helper()
	b := New(w.corbie.Client, secret)
	e := requests.New(w.corbie.Client, b, secret, w.reading())
	b.Engine = e
	b.Interval = 20 * time.Millisecond
	b.Report = func(err error) { w.t.Errorf("the bridges report: %v", err) }
	e.Report = func(err error) { w.t.Errorf("the engine reports: %v", err) }
	run, cancel := context.WithCancel(ctx)
	done := make(chan error, 2)
	go func() { done <- e.Run(run) }()
	go func() { done <- b.Run(run) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			if err := errors.Join(<-done, <-done); err != nil {
				w.t.Errorf("serving ended: %v", err)
			}
		})
	}
	w.t.Cleanup(stop)
	w.mu.Lock()
	w.engine, w.bridges, w.stop = e, b, stop
	w.mu.Unlock()
}

// restart stops serving the office, and serves it again.
func (w *world) restart() {
	w.t.Helper()
	w.mu.Lock()
	stop := w.stop
	w.mu.Unlock()
	stop()
	w.start()
}

// change writes changed files of the office, and serves the new reading.
func (w *world) change(files map[string]string) {
	w.t.Helper()
	for name, text := range files {
		files[name] = strings.ReplaceAll(text, "FEEDS", w.feeds)
	}
	testkit.Write(w.t, w.dir, files)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.engine.Swap(w.reading())
}

// write writes a stand-in's feed whole, with what it has heard and why it
// refuses, if it does.
func (w *world) write(bridge string, refuse ...string) {
	w.t.Helper()
	feed := map[string]any{"things": w.things[bridge], "refuse": strings.Join(refuse, "")}
	if w.things[bridge] == nil {
		feed["things"] = []any{}
	}
	data, err := json.Marshal(feed)
	if err != nil {
		w.t.Fatal(err)
	}
	file := filepath.Join(w.feeds, bridge+".json")
	if err := os.WriteFile(file+".new", data, 0o644); err != nil {
		w.t.Fatal(err)
	}
	if err := os.Rename(file+".new", file); err != nil {
		w.t.Fatal(err)
	}
}

// say has a stand-in hear things: once its in run rests, they are written
// into its feed, and the clock moves a minute, so the in hears again. A
// thing is said at the clock's time unless it gives its own.
func (w *world) say(bridge string, things ...map[string]any) {
	w.t.Helper()
	w.resting(bridge)
	for _, th := range things {
		if th["at"] == nil {
			th["at"] = w.corbie.Clock.Now().Format(time.RFC3339Nano)
		}
		w.things[bridge] = append(w.things[bridge], th)
	}
	w.write(bridge)
	w.corbie.Clock.Move(time.Minute)
}

// resting stays until a bridge's newest in run rests, waiting on its
// timer, and gives it.
func (w *world) resting(bridge string) *client.Run {
	w.t.Helper()
	return w.after(bridge, "")
}

// after stays until a bridge's newest in run is one started after a run,
// and rests, waiting on its timer, and gives it.
func (w *world) after(bridge, run string) *client.Run {
	w.t.Helper()
	var in *client.Run
	w.corbie.Until(w.t, "an in run of "+bridge+" after "+run+" rests", func() bool {
		runs := w.inRuns(bridge)
		if len(runs) == 0 || runs[len(runs)-1].ID <= run {
			return false
		}
		in = w.show(runs[len(runs)-1].ID)
		return in.Status == "running" && in.Timers > 0
	})
	return in
}

// state stays until a bridge stands as wanted: its status, with an error
// holding a text. How a bridge stands is kept in memory, where no write to
// the store tells of a change, so it is looked at every few milliseconds.
func (w *world) state(bridge, status, holds string) State {
	w.t.Helper()
	w.mu.Lock()
	b := w.bridges
	w.mu.Unlock()
	limit := time.Now().Add(testkit.Patience)
	for {
		s := b.State(bridge)
		if s.Status == status && strings.Contains(s.Error, holds) {
			return s
		}
		if time.Now().After(limit) {
			w.t.Fatalf("waited %v, and %s stands %+v, not %s with %q", testkit.Patience, bridge, s, status, holds)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// said is something said on a bridge: its id, who said it, the place and
// where, and the text.
func said(id, from, place, where, text string) map[string]any {
	return map[string]any{"id": id, "kind": "said", "from": from, "place": place, "where": where, "text": text, "mentioned": false}
}

// press is a press of a button: its id, who pressed it, the ref of the
// message it is on, and the choice.
func press(id, from, ref, choice string) map[string]any {
	return map[string]any{"id": id, "kind": "press", "from": from, "place": "direct", "where": from, "ref": ref, "choice": choice}
}

// list gives the office's runs a filter keeps, ended ones among them, by
// name.
func (w *world) list(filter client.Filter) []*client.Run {
	w.t.Helper()
	filter.Labels["bulldog"] = w.id
	filter.Finished = true
	runs, err := w.corbie.List(ctx, filter)
	if err != nil {
		w.t.Fatal(err)
	}
	runs = slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Parent != "" })
	slices.SortFunc(runs, func(a, b *client.Run) int { return strings.Compare(a.ID, b.ID) })
	return runs
}

// show gives a run as it stands.
func (w *world) show(run string) *client.Run {
	w.t.Helper()
	shown, err := w.corbie.Show(ctx, run)
	if err != nil {
		w.t.Fatal(err)
	}
	return shown
}

// input gives what a run was given, as it shows it.
func (w *world) input(run string) map[string]any {
	w.t.Helper()
	input, _ := w.show(run).Input.(map[string]any)
	return input
}

// inRuns gives a bridge's in runs, by name.
func (w *world) inRuns(bridge string) []*client.Run {
	w.t.Helper()
	return w.list(client.Filter{Labels: map[string]string{"kind": "bridge", "bridge": bridge}})
}

// requests gives the office's requests, by name.
func (w *world) requests() []*client.Run {
	w.t.Helper()
	return w.list(client.Filter{Labels: map[string]string{"kind": "request"}})
}

// request stays until a request of the office was asked with a text, and
// gives its run.
func (w *world) request(text string) *client.Run {
	w.t.Helper()
	var found *client.Run
	w.corbie.Until(w.t, "a request says "+text, func() bool {
		for _, run := range w.requests() {
			if w.input(run.ID)["said"] == text {
				found = run
				return true
			}
		}
		return false
	})
	return found
}

// given gives what a request's run was given as request.
func (w *world) given(run string) map[string]any {
	w.t.Helper()
	request, _ := w.input(run)["request"].(map[string]any)
	return request
}

// question stays until a question holding a text was carried, by an out
// run that ended done, and gives that run.
func (w *world) question(text string) *client.Run {
	w.t.Helper()
	var found *client.Run
	w.corbie.Until(w.t, "a question is carried that says "+text, func() bool {
		for _, run := range w.list(client.Filter{Labels: map[string]string{"kind": "out", "wait": ""}}) {
			if shown := w.show(run.ID); shown.Status == "done" && strings.Contains(fmt.Sprint(shown.Input), text) {
				found = shown
				return true
			}
		}
		return false
	})
	return found
}

// ref gives the ref a carried question holds, and sent, the id of the
// message that carried it.
func ref(question *client.Run) (string, string) {
	input, _ := question.Input.(map[string]any)
	output, _ := question.Output.(map[string]any)
	r, _ := input["ref"].(string)
	sent, _ := output["sent"].(string)
	return r, sent
}

// answered stays until a request has ended, and gives its answer.
func (w *world) answered(run string) string {
	w.t.Helper()
	output, _ := w.corbie.Ended(w.t, run).Output.(map[string]any)
	answer, _ := output["answer"].(string)
	return answer
}

// replies stays until a request's answer was carried by an out run that
// ended, and gives every such run.
func (w *world) replies(request string) []*client.Run {
	w.t.Helper()
	var runs []*client.Run
	w.corbie.Until(w.t, "the answer to "+request+" is carried", func() bool {
		runs = w.list(client.Filter{Labels: map[string]string{"kind": "out", "request": request}})
		return len(runs) > 0 && w.show(runs[0].ID).HasEnded()
	})
	return runs
}
