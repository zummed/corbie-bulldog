package work

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/store"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// The office's identity, as the server keeps it.
const identity = "0f4c2a9e"

// house gives the files of the house, and work those of the work office,
// each bridge a stand-in that reaches no service: the house's Telegram
// listens and says things to a person directly, the work office's Slack
// hears and does too, and its GitLab listens and has no direct place.
func house(t *testing.T) map[string]string {
	return standin.Instead(testkit.Files(t, examples.House()), "telegram", standin.Folder(false, true))
}

func work(t *testing.T) map[string]string {
	files := standin.Instead(testkit.Files(t, examples.Work()), "slack", standin.Folder(true, true))
	return standin.Instead(files, "gitlab", standin.Folder(false, false))
}

// A bench is an office in a folder of the test's own, with a whole Corbie
// and a stand-in model, every area in one store, and its work running, on
// a clock the test moves. What the stand-in answers is the test's to say.
type bench struct {
	t      *testing.T
	dir    string
	path   string
	clock  *testkit.Clock
	corbie *testkit.Corbie
	reach  *flaky
	model  *testkit.Model

	db     *sql.DB
	keeper *office.Keeper
	reqs   *requests.Store
	ch     *changes.Changes
	d      *decisions.Decisions
	b      *bridges.Bridges
	w      *Work
	stop   func()

	mu     sync.Mutex
	answer func(testkit.Turn) testkit.Reply
	logs   []string
	moved  chan struct{}
	// passes counts the passes the work has done.
	passes int
}

// newBench makes an office of the files, its models the stand-in, and
// starts its work.
func newBench(t *testing.T, files map[string]string) *bench {
	t.Helper()
	b := &bench{t: t, clock: testkit.NewClock(testkit.Monday), moved: make(chan struct{}, 1)}
	b.answer = func(testkit.Turn) testkit.Reply { return testkit.Say("done") }
	b.model = testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
		b.mu.Lock()
		answer := b.answer
		b.mu.Unlock()
		return answer(turn)
	})
	b.dir = testkit.Folder(t, withModel(files, b.model))
	if err := shipped.Write(b.dir); err != nil {
		t.Fatal(err)
	}
	if _, err := changes.Begin(b.dir, b.clock.Now()); err != nil {
		t.Fatal(err)
	}
	b.corbie = testkit.NewCorbie(t, b.clock)
	b.reach = &flaky{Client: b.corbie.Client, fails: map[string]error{}}
	b.keeper = office.Keep(b.dir)
	if b.keeper.Office() == nil {
		t.Fatalf("the office does not read: %v", b.keeper.Problems())
	}
	b.path = filepath.Join(b.dir, shipped.Dir, "bulldog.db")
	b.open()
	return b
}

// usual is a team's model named usual, which the bench makes the
// stand-in.
var usual = regexp.MustCompile(`(?m)^(\s*)usual: \{[^}\n]*\}`)

func withModel(files map[string]string, m *testkit.Model) map[string]string {
	out := map[string]string{}
	for name, text := range files {
		out[name] = usual.ReplaceAllString(text, "${1}usual: "+m.Line())
	}
	return out
}

// open opens the store and every area, and starts the work, as a server
// starting does.
func (b *bench) open() {
	b.t.Helper()
	db, err := sql.Open("sqlite", b.path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate")
	if err != nil {
		b.t.Fatal(err)
	}
	b.t.Cleanup(func() { db.Close() })
	b.db = db
	if b.reqs, err = requests.Open(db, b.clock.Now); err != nil {
		b.t.Fatal(err)
	}
	if b.ch, err = changes.Open(ctx, db, b.dir, b.keeper); err != nil {
		b.t.Fatal(err)
	}
	b.ch.Now = b.clock.Now
	if b.d, err = decisions.Open(db, b.reqs, b.ch); err != nil {
		b.t.Fatal(err)
	}
	b.d.Now = b.clock.Now
	b.ch.Decide = b.d.Change
	if b.b, err = bridges.Open(db, b.reqs, b.clock.Now); err != nil {
		b.t.Fatal(err)
	}
	if b.w, err = New(db, b.reach, b.clock, identity, Areas{Keeper: b.keeper, Requests: b.reqs, Decisions: b.d, Changes: b.ch, Bridges: b.b}); err != nil {
		b.t.Fatal(err)
	}
	b.w.Log = b.log
	b.w.passed = func() {
		b.mu.Lock()
		b.passes++
		b.mu.Unlock()
		b.poke()
	}
	b.reqs.Watch(func(int64) { b.poke() })
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := b.w.Run(runCtx); err != nil {
			b.log("run: %v", err)
		}
	}()
	b.stop = func() {
		cancel()
		<-done
		b.d.Close()
		b.b.Close()
	}
	b.t.Cleanup(b.halt)
	b.until("the office is read", func() bool {
		_, err := b.reqs.OpenOf(office.Top)
		return err == nil
	})
}

// halt stops the work and every area, as a server stopping does.
func (b *bench) halt() {
	if b.stop != nil {
		b.stop()
		b.stop = nil
	}
}

// restart stops everything and opens it again on the same store and the
// same Corbie.
func (b *bench) restart() {
	b.t.Helper()
	b.halt()
	b.db.Close()
	b.open()
}

func (b *bench) log(format string, args ...any) {
	b.mu.Lock()
	b.logs = append(b.logs, fmt.Sprintf(format, args...))
	b.mu.Unlock()
	b.poke()
}

func (b *bench) logged(part string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.ContainsFunc(b.logs, func(l string) bool { return strings.Contains(l, part) })
}

func (b *bench) poke() {
	select {
	case b.moved <- struct{}{}:
	default:
	}
}

// says has the stand-in answer every turn so.
func (b *bench) says(answer func(testkit.Turn) testkit.Reply) {
	b.mu.Lock()
	b.answer = answer
	b.mu.Unlock()
}

// until stays until done says so, looking again whenever Corbie's store
// is written, the clock moves, a request changes or the work has done a
// pass.
func (b *bench) until(what string, done func() bool) {
	b.t.Helper()
	written, stopWritten := b.corbie.Memory.As(store.Party{Kind: store.Owner}).Notice("")
	defer stopWritten()
	moved, stopMoved := b.clock.Watch()
	defer stopMoved()
	limit := time.NewTimer(testkit.Patience)
	defer limit.Stop()
	for !done() {
		select {
		case <-written:
		case <-moved:
		case <-b.moved:
		case <-limit.C:
			b.mu.Lock()
			logs := strings.Join(b.logs, "\n")
			b.mu.Unlock()
			b.t.Fatalf("waited %v, and still not: %s\nthe log:\n%s", testkit.Patience, what, logs)
		}
	}
}

// still stays until the work has done a whole pass begun after it was
// called, so all there was to do then is done.
func (b *bench) still() {
	b.t.Helper()
	count := func() int {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.passes
	}
	p := count()
	for i := range 2 {
		b.w.Poke()
		b.until("the work has done a pass", func() bool { return count() > p+i })
	}
}

// later moves the clock on.
func (b *bench) later(d time.Duration) { b.clock.Move(d) }

// office gives the office as it was last read.
func (b *bench) office() *office.Office { return b.keeper.Office() }

// rewrite writes files into the folder, the stand-in's model in them,
// removes those given as "", and has the work read the office again.
func (b *bench) rewrite(files map[string]string) {
	b.t.Helper()
	write := map[string]string{}
	for name, text := range files {
		if text == "" {
			if err := os.Remove(filepath.Join(b.dir, filepath.FromSlash(name))); err != nil {
				b.t.Fatal(err)
			}
			continue
		}
		write[name] = text
	}
	testkit.Write(b.t, b.dir, withModel(write, b.model))
	before := b.keeper.Office()
	b.w.Poke()
	b.until("the office is read again", func() bool { return b.keeper.Office() != before })
}

// chat is a request a person asks in the chat, of a team or the office.
func (b *bench) chat(person, team, text string) int64 {
	b.t.Helper()
	r, err := b.reqs.AskInChat(person, team, text, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	return r.Number
}

// say is a person saying more in a request, on the screens.
func (b *bench) say(n int64, person, text string) {
	b.t.Helper()
	if _, err := b.reqs.Say(n, b.reqs.Person(person), text, nil, requests.Screens); err != nil {
		b.t.Fatal(err)
	}
}

// runs gives the runs kept with a request.
func (b *bench) runs(n int64) []KeptRun {
	b.t.Helper()
	runs, err := b.w.Runs(n)
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(runs, func(r KeptRun) bool { return r.Kind != KindWork })
}

// started stays until a request has had so many runs, and gives the
// latest.
func (b *bench) started(n int64, count int) KeptRun {
	b.t.Helper()
	var runs []KeptRun
	b.until(fmt.Sprintf("request %d has had %d runs", n, count), func() bool {
		runs = b.runs(n)
		return len(runs) >= count
	})
	if len(runs) > count {
		b.t.Fatalf("request %d has had %d runs, not %d", n, len(runs), count)
	}
	return runs[count-1]
}

// settled stays until a request's runs have all ended and the request
// stands as said.
func (b *bench) settled(n int64, state string) {
	b.t.Helper()
	b.until(fmt.Sprintf("request %d is %s with no run going", n, state), func() bool {
		st, err := b.reqs.State(n)
		if err != nil {
			b.t.Fatal(err)
		}
		return st.Is == state && !slices.ContainsFunc(b.runs(n), func(r KeptRun) bool { return r.Status == going })
	})
}

// input gives the input a run was started with, and the text it carries
// read from its blob.
func (b *bench) input(run string) map[string]any {
	b.t.Helper()
	var in map[string]any
	b.until("the run "+run+" has started", func() bool {
		step, err := b.corbie.Step(ctx, run, 1)
		if errors.Is(err, client.ErrNoRecord) || errors.Is(err, client.ErrNoRun) {
			return false
		}
		if err != nil {
			b.t.Fatal(err)
		}
		in, _ = step.Events[0].Data.(map[string]any)
		return true
	})
	if blob, ok := in["text"].(map[string]any); ok {
		in["text"] = b.fetch(run, blob["$blob"].(string))
	}
	return in
}

// fetch gives a blob of a run's, by the path it is shown with.
func (b *bench) fetch(run, path string) string {
	b.t.Helper()
	r, _, err := b.corbie.Fetch(ctx, run, path)
	if err != nil {
		b.t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		b.t.Fatal(err)
	}
	return string(data)
}

// thread gives a request's thread as lines: kind, by, to, text.
func (b *bench) thread(n int64) []string {
	b.t.Helper()
	es, err := b.reqs.Thread(n)
	if err != nil {
		b.t.Fatal(err)
	}
	var lines []string
	for _, e := range es {
		var to []string
		for _, w := range e.To {
			to = append(to, w.Name)
		}
		lines = append(lines, fmt.Sprintf("%s by %s to %s: %s", e.Kind, cmpOr(e.By.Name, e.By.Kind), strings.Join(to, ","), e.Text))
	}
	return lines
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// entries gives a request's entries of a kind.
func (b *bench) entries(n int64, kind string) []requests.Entry {
	b.t.Helper()
	es, err := b.reqs.Thread(n)
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(es, func(e requests.Entry) bool { return e.Kind != kind })
}

// listed gives the runs at the top labelled with the office's identity
// and the labels given, those ended among them.
func (b *bench) listed(labels map[string]string) []*client.Run {
	b.t.Helper()
	filter := map[string]string{LabelBulldog: identity}
	for k, v := range labels {
		filter[k] = v
	}
	runs, err := b.corbie.List(ctx, client.Filter{Labels: filter, Finished: true})
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Parent != "" })
}

// listedAll gives every run labelled with the office's identity, those
// below others and those ended among them, as the watch lists them.
func (b *bench) listedAll() []*client.Run {
	b.t.Helper()
	runs, err := b.corbie.List(ctx, client.Filter{Labels: map[string]string{LabelBulldog: identity}, Finished: true})
	if err != nil {
		b.t.Fatal(err)
	}
	return runs
}

// hold has the stand-in hold each turn whose prompt holds a word until
// the test lets it go, and gives the func that lets them go, which the
// test's end calls too.
func (b *bench) hold(word string, then testkit.Reply) (held func() int, let func()) {
	gate := make(chan struct{})
	var mu sync.Mutex
	count := 0
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), word) {
			mu.Lock()
			count++
			mu.Unlock()
			b.poke()
			<-gate
			return then
		}
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

// away takes the work's tables out of the store's reach, as a store that
// cannot be read is, and gives the func that puts them back.
func (b *bench) away(tables ...string) func() {
	b.t.Helper()
	for _, table := range tables {
		if _, err := b.db.Exec(`ALTER TABLE ` + table + ` RENAME TO ` + table + `_away`); err != nil {
			b.t.Fatal(err)
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, table := range tables {
				if _, err := b.db.Exec(`ALTER TABLE ` + table + `_away RENAME TO ` + table); err != nil {
					b.t.Error(err)
				}
			}
		})
	}
}

// workTables are every table of the work's part of the store.
var workTables = []string{"work_runs", "work_requests", "work_calls", "work_given", "work_subs", "work_lets", "work_outcomes", "work_rounds"}

// logCount gives how many lines the work has logged.
func (b *bench) logCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.logs)
}

// logsSince gives the lines the work logged after so many.
func (b *bench) logsSince(count int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.logs[count:])
}

// A flaky is Corbie as work reaches it, which the test can make fail.
type flaky struct {
	*client.Client
	mu    sync.Mutex
	fails map[string]error
	calls map[string]int
	// starting is told of each start before it is sent, and fails it
	// with what it gives; lie sends it and says it failed.
	starting func(client.Start) error
	lie      bool
	// cancelled is told of each cancel once it is sent; showing may stand
	// in for what a run is shown as; replying is told of each reply before
	// it is sent, and fails it with what it gives.
	cancelled func(run string)
	showing   func(*client.Run) *client.Run
	replying  func(run string) error
}

func (f *flaky) failing(what string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[what]++
	return f.fails[what]
}

// fail makes a call of Corbie's fail with err, or work again for nil;
// "show call" fails only the showing of a run of a call.
func (f *flaky) fail(what string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fails[what] = err
}

func (f *flaky) count(what string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[what]
}

func (f *flaky) Start(ctx context.Context, d *machine.Draft, s client.Start) (string, error) {
	if err := f.failing("start"); err != nil {
		return "", err
	}
	f.mu.Lock()
	starting, lie := f.starting, f.lie
	f.mu.Unlock()
	if lie {
		if _, err := f.Client.Start(ctx, d, s); err != nil {
			return "", err
		}
		return "", errors.New("the receipt did not come")
	}
	if starting != nil {
		if err := starting(s); err != nil {
			return "", err
		}
	}
	return f.Client.Start(ctx, d, s)
}

func (f *flaky) Show(ctx context.Context, id string) (*client.Run, error) {
	if err := f.failing("show"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	showing := f.showing
	f.mu.Unlock()
	if showing != nil {
		r, err := f.Client.Show(ctx, id)
		if errors.Is(err, client.ErrNoRun) {
			r, err = &client.Run{ID: id}, nil
		}
		if err != nil {
			return nil, err
		}
		if shown := showing(r); shown != nil {
			return shown, nil
		}
	}
	r, err := f.Client.Show(ctx, id)
	if err == nil {
		if err := f.failing("show " + r.Machine); err != nil {
			return nil, err
		}
	}
	return r, err
}

func (f *flaky) Cancel(ctx context.Context, run string, life time.Duration) (string, error) {
	if err := f.failing("cancel"); err != nil {
		return "", err
	}
	message, err := f.Client.Cancel(ctx, run, life)
	f.mu.Lock()
	cancelled := f.cancelled
	f.mu.Unlock()
	if err == nil && cancelled != nil {
		cancelled(run)
	}
	return message, err
}

// set changes what the flaky Corbie does, under its lock.
func (f *flaky) set(change func(f *flaky)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func (f *flaky) List(ctx context.Context, filter client.Filter) ([]*client.Run, error) {
	if err := f.failing("list"); err != nil {
		return nil, err
	}
	return f.Client.List(ctx, filter)
}

func (f *flaky) Watch(ctx context.Context, filter client.Filter, interval time.Duration, tell func(client.Update)) error {
	if err := f.failing("watch"); err != nil {
		return err
	}
	return f.Client.Watch(ctx, filter, interval, tell)
}

func (f *flaky) Reply(ctx context.Context, run, ticket string, value any, life time.Duration) (string, error) {
	if err := f.failing("reply"); err != nil {
		return "", err
	}
	f.mu.Lock()
	replying := f.replying
	f.mu.Unlock()
	if replying != nil {
		if err := replying(run); err != nil {
			return "", err
		}
	}
	return f.Client.Reply(ctx, run, ticket, value, life)
}
