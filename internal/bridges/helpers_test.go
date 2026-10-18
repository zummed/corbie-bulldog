package bridges_test

import (
	"database/sql"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// A desk is this area's part of a store on a file of the test's own, with
// the requests beside it and the office it was last given, on a clock the
// test moves. It stands in for whoever drives Corbie: it asks for runs,
// names them, and says how they ended.
type desk struct {
	*bridges.Bridges
	t        *testing.T
	db       *sql.DB
	file     string
	clock    *testkit.Clock
	requests *requests.Store
	files    map[string]string
	o        *office.Office
	runs     int
	chosen   []bridges.Choice
	signedIn []bridges.SignIn
	// choose is what Decisions says to a choice.
	choose func(bridges.Choice) (string, error)
}

// open opens a store on a new file and reads an office into it.
func open(t *testing.T, files map[string]string) *desk {
	t.Helper()
	d := &desk{t: t, file: filepath.Join(t.TempDir(), "bulldog.db"), clock: testkit.NewClock(testkit.Monday)}
	d.choose = func(c bridges.Choice) (string, error) { return "Taken: " + c.Choice + c.Text + ".", nil }
	d.connect()
	d.read(files)
	return d
}

func (d *desk) connect() {
	d.t.Helper()
	db, err := sql.Open("sqlite", d.file+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(func() { db.Close() })
	store, err := requests.Open(db, d.clock.Now)
	if err != nil {
		d.t.Fatal(err)
	}
	b, err := bridges.Open(db, store, d.clock.Now)
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(b.Close)
	b.Choose = func(c bridges.Choice) (string, error) {
		d.chosen = append(d.chosen, c)
		return d.choose(c)
	}
	b.Choices = func(e requests.Entry) []string {
		if e.Kind == "decision" {
			return []string{"yes", "no"}
		}
		return nil
	}
	b.SignedIn = func(s bridges.SignIn) { d.signedIn = append(d.signedIn, s) }
	d.db, d.requests, d.Bridges = db, store, b
}

// restart opens the file again, as a server started again would, and
// reads the office into it.
func (d *desk) restart() {
	d.t.Helper()
	d.Close()
	d.db.Close()
	d.connect()
	d.read(d.files)
}

// read reads an office's files, which must read, into the requests and
// then this area.
func (d *desk) read(files map[string]string) {
	d.t.Helper()
	o, problems := office.Parse(d.t.TempDir(), given(files))
	if problems.Refuse() {
		d.t.Fatalf("the office does not read:\n%v", problems)
	}
	d.files, d.o = files, o
	must(d.t, d.requests.Read(o))
	must(d.t, d.Read(o))
}

func given(files map[string]string) map[string][]byte {
	out := map[string][]byte{}
	for name, text := range files {
		out[name] = []byte(text)
	}
	return out
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// house gives the house's files, with a stand-in for its Telegram bridge,
// which listens and says things to a person directly.
func house(t *testing.T) map[string]string {
	files := without(testkit.Files(t, examples.House()), "bridges/telegram")
	maps.Copy(files, standin.In("telegram", standin.Folder(false, true)))
	return files
}

// work gives the work office's files, with stand-ins for its Slack bridge,
// which hears and says things to a person directly, and its GitLab
// bridge, which listens and has no direct place.
func work(t *testing.T) map[string]string {
	files := without(without(testkit.Files(t, examples.Work()), "bridges/slack"), "bridges/gitlab")
	maps.Copy(files, standin.In("slack", standin.Folder(true, true)))
	maps.Copy(files, standin.In("gitlab", standin.Folder(false, false)))
	return files
}

// replace gives files with one file's text changed: old must be in it.
func replace(t *testing.T, files map[string]string, file, old, new string) map[string]string {
	t.Helper()
	if !strings.Contains(files[file], old) {
		t.Fatalf("%s does not hold %q", file, old)
	}
	out := maps.Clone(files)
	out[file] = strings.Replace(files[file], old, new, 1)
	return out
}

// without gives files with every file under a folder taken out.
func without(files map[string]string, dir string) map[string]string {
	out := map[string]string{}
	for name, text := range files {
		if !strings.HasPrefix(name, dir+"/") {
			out[name] = text
		}
	}
	return out
}

// name gives a new run's name.
func (d *desk) name() string {
	d.runs++
	return fmt.Sprintf("run-%d", d.runs)
}

// listen asks for the listening runs to start now, and starts them.
func (d *desk) listen() []bridges.Start {
	d.t.Helper()
	starts, _, err := d.Listen()
	must(d.t, err)
	for i := range starts {
		starts[i].Labels["run"] = d.name()
		must(d.t, d.Started(starts[i], starts[i].Labels["run"]))
	}
	return starts
}

// listening starts the one listening run of a bridge.
func (d *desk) listening(bridge string) bridges.Start {
	d.t.Helper()
	for _, s := range d.listen() {
		if s.Bridge == bridge {
			return s
		}
	}
	d.t.Fatalf("no listening run of %s is started", bridge)
	return bridges.Start{}
}

// hear has a listening run of a bridge end with what it heard, and the
// cursor one higher than it was given.
func (d *desk) hear(bridge string, things ...map[string]any) bridges.Taken {
	d.t.Helper()
	s := d.listening(bridge)
	cursor, _ := s.Input["cursor"].(float64)
	taken, err := d.InEnded(bridge, s.Labels["run"], bridges.End{Output: heard(cursor+1, things...)})
	must(d.t, err)
	return taken
}

// delivered has a hearing run of a bridge end with what it heard.
func (d *desk) delivered(bridge string, things ...map[string]any) bridges.Taken {
	d.t.Helper()
	s, err := d.Hear(bridge, bridges.Delivery{Method: "POST", Body: "{}"})
	must(d.t, err)
	taken, err := d.InEnded(s.Bridge, d.name(), bridges.End{Output: heard(0, things...)})
	must(d.t, err)
	return taken
}

// heard gives an in run's output.
func heard(cursor float64, things ...map[string]any) map[string]any {
	list := []any{}
	for _, t := range things {
		list = append(list, t)
	}
	return map[string]any{"heard": list, "cursor": cursor}
}

// said gives a thing said, with an id of its own.
var ids atomic.Int64

func said(from, heardIn, place, text string, more ...any) map[string]any {
	t := map[string]any{"id": fmt.Sprintf("m%d", ids.Add(1)), "kind": "said", "from": from, "heard_in": heardIn, "place": place, "text": text}
	for i := 0; i+1 < len(more); i += 2 {
		t[more[i].(string)] = more[i+1]
	}
	return t
}

// carry asks for the out runs to start now, and starts them.
func (d *desk) carry() []bridges.Start {
	d.t.Helper()
	starts, _, err := d.Carry()
	must(d.t, err)
	for i := range starts {
		starts[i].Labels["run"] = d.name()
		must(d.t, d.Started(starts[i], starts[i].Labels["run"]))
	}
	return starts
}

// ended has an out run end with its output.
func (d *desk) ended(s bridges.Start, output map[string]any) {
	d.t.Helper()
	must(d.t, d.OutEnded(s.Thing, s.Labels["run"], bridges.End{Output: output}))
}

// failed has an out run fail.
func (d *desk) failed(s bridges.Start, why string) {
	d.t.Helper()
	must(d.t, d.OutEnded(s.Thing, s.Labels["run"], bridges.End{Failed: why}))
}

// to gives the out runs of a list that carry to a place on a bridge.
func to(starts []bridges.Start, bridge, place string) []bridges.Start {
	var out []bridges.Start
	for _, s := range starts {
		if s.Bridge == bridge && s.Input["to"] == place {
			out = append(out, s)
		}
	}
	return out
}

// places gives where each out run of a list carries to, as bridge:place.
func places(starts []bridges.Start) []string {
	var out []string
	for _, s := range starts {
		out = append(out, fmt.Sprintf("%s:%v", s.Bridge, s.Input["to"]))
	}
	slices.Sort(out)
	return out
}

// thread gives the texts of a request's entries, each as kind: text.
func (d *desk) thread(n int64) []string {
	d.t.Helper()
	es, err := d.requests.Thread(n)
	must(d.t, err)
	var out []string
	for _, e := range es {
		out = append(out, e.Kind+": "+e.Text)
	}
	return out
}

// latest gives the number of the latest request, or 0.
func (d *desk) latest() int64 {
	var n int64
	for {
		if _, err := d.requests.Get(n + 1); err != nil {
			return n
		}
		n++
	}
}

// ask has Dana ask in a thread of #help, and gives the request.
func (d *desk) ask(thread, text string) int64 {
	d.t.Helper()
	before := d.latest()
	d.delivered("slack", said("U0DANA00", "#help", thread, text))
	n := d.latest()
	if n == before {
		d.t.Fatalf("%q in %s made no request", text, thread)
	}
	return n
}

// answer has a request's taker, a worker, answer it.
func (d *desk) answer(n int64, worker, text string) *requests.Entry {
	d.t.Helper()
	e, err := d.requests.Answer(n, d.requests.Worker(worker), text, nil)
	must(d.t, err)
	return e
}

// decide writes a decision put to people, as Decisions writes one.
func (d *desk) decide(n int64, text string, people ...string) *requests.Entry {
	d.t.Helper()
	var whos []requests.Who
	for _, p := range people {
		whos = append(whos, d.requests.Person(p))
	}
	e, err := d.requests.Write(n, requests.Writing{Kind: "decision", By: requests.Bulldog, To: whos, Text: text, Read: true, Waits: whos})
	must(d.t, err)
	return e
}

// carried gives the states of what a request's entries were carried as,
// each as bridge:place state.
func (d *desk) carried(n int64) []string {
	d.t.Helper()
	cs, err := d.Carried(n)
	must(d.t, err)
	var out []string
	for _, c := range cs {
		out = append(out, fmt.Sprintf("%s:%s %s", c.Bridge, c.Place, c.State))
	}
	return out
}

// later moves the test's clock on.
func (d *desk) later(by time.Duration) { d.clock.Move(by) }

// drain carries everything there is to carry, each run ending with sent,
// until nothing is left, and gives every out run started, in the order
// they were.
func (d *desk) drain() []bridges.Start {
	d.t.Helper()
	var all []bridges.Start
	for {
		out := d.carry()
		if len(out) == 0 {
			return all
		}
		for _, s := range out {
			d.ended(s, map[string]any{"sent": fmt.Sprintf("s%d", s.Thing)})
		}
		all = append(all, out...)
	}
}

// texts gives what each out run of a list carries, as bridge:place: text.
func texts(starts []bridges.Start) []string {
	var out []string
	for _, s := range starts {
		out = append(out, fmt.Sprintf("%s:%v: %v", s.Bridge, s.Input["to"], textIn(s)))
	}
	slices.Sort(out)
	return out
}

// textIn gives the text an out run carries, which it is given as a blob.
func textIn(s bridges.Start) string { return blobOf(s.Input["text"]) }

// blobOf gives what a blob of a run's input holds, and leaves it to be
// read again.
func blobOf(v any) string {
	f, ok := v.(client.InputFile)
	if !ok {
		return fmt.Sprintf("not a blob: %#v", v)
	}
	data, _ := io.ReadAll(f.Contents)
	f.Contents.(io.Seeker).Seek(0, io.SeekStart)
	return string(data)
}
