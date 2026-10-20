package decisions_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// house gives the files of the house, and work those of the work office.
func house(t *testing.T) map[string]string { return testkit.Files(t, examples.House()) }
func work(t *testing.T) map[string]string  { return testkit.Files(t, examples.Work()) }

// A bench is an office in a folder of the test's own, in a repository of
// its own, with its requests, its changes and its decisions in one store,
// on a clock the test moves. A stand-in runs its rules and workers, and
// what work would be told is kept for the test to read.
type bench struct {
	t      *testing.T
	dir    string
	db     *sql.DB
	clock  *testkit.Clock
	o      *office.Office
	keeper *office.Keeper
	reqs   *requests.Store
	ch     *changes.Changes
	d      *decisions.Decisions
	runner *runner

	mu       sync.Mutex
	verdicts []answered
	lets     []decisions.Let
	told     []changes.Outcome
	troubles []error
	answer   func(ask string, v decisions.Verdict) error
	letFails error
	moved    chan struct{}
}

// answered is an ask's ticket answered, as work would answer it.
type answered struct {
	ask string
	v   decisions.Verdict
}

func newBench(t *testing.T, files map[string]string) *bench {
	t.Helper()
	dir := testkit.Folder(t, files)
	if err := shipped.Write(dir); err != nil {
		t.Fatal(err)
	}
	clock := testkit.NewClock(testkit.Monday)
	if _, err := changes.Begin(dir, clock.Now()); err != nil {
		t.Fatal(err)
	}
	b := &bench{t: t, dir: dir, clock: clock, keeper: office.Keep(dir), runner: newRunner(), moved: make(chan struct{}, 1)}
	b.open()
	return b
}

// open opens the store and every part of it, and reads the office into
// each, as a server starting does.
func (b *bench) open() {
	b.t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(b.dir, shipped.Dir, "bulldog.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate")
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
	d := b.d
	d.Now = b.clock.Now
	d.Run = b.runner.run
	d.Answer = func(_ context.Context, ask string, v decisions.Verdict) error {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.answer != nil {
			if err := b.answer(ask, v); err != nil {
				return err
			}
		}
		b.verdicts = append(b.verdicts, answered{ask, v})
		return nil
	}
	d.Let = func(_ context.Context, l decisions.Let) error {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.letFails != nil {
			return b.letFails
		}
		b.lets = append(b.lets, l)
		return nil
	}
	d.Trouble = func(err error) {
		b.mu.Lock()
		b.troubles = append(b.troubles, err)
		b.mu.Unlock()
		b.poke()
	}
	b.ch.Decide = d.Change
	b.ch.Told = func(o changes.Outcome) {
		b.mu.Lock()
		b.told = append(b.told, o)
		b.mu.Unlock()
		d.Told(o)
	}
	b.reqs.Watch(func(int64) { b.poke() })
	d.Watch(func(int64) { b.poke() })
	b.t.Cleanup(d.Close)
	b.read()
	b.t.Cleanup(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, err := range b.troubles {
			b.t.Errorf("trouble: %v", err)
		}
	})
}

func (b *bench) poke() {
	select {
	case b.moved <- struct{}{}:
	default:
	}
}

// read reads the office from its folder into every part.
func (b *bench) read() {
	b.t.Helper()
	o, err := office.Read(b.dir)
	if err != nil {
		b.t.Fatal(err)
	}
	b.o = o
	if err := b.reqs.Read(o); err != nil {
		b.t.Fatal(err)
	}
	if err := b.d.Read(ctx, o); err != nil {
		b.t.Fatal(err)
	}
}

// rewrite writes files into the folder, removes those given as "", and
// reads the office again.
func (b *bench) rewrite(files map[string]string) {
	b.t.Helper()
	for name, text := range files {
		if text == "" {
			if err := os.Remove(filepath.Join(b.dir, filepath.FromSlash(name))); err != nil {
				b.t.Fatal(err)
			}
			delete(files, name)
		}
	}
	testkit.Write(b.t, b.dir, files)
	b.read()
}

// restart stops the decisions and opens everything again, as a server
// started again does.
func (b *bench) restart() {
	b.t.Helper()
	b.d.Close()
	b.open()
}

// until stays until done says so, looking again each time a request or a
// decision changes.
func (b *bench) until(what string, done func() bool) {
	b.t.Helper()
	limit := time.NewTimer(testkit.Patience)
	defer limit.Stop()
	for !done() {
		select {
		case <-b.moved:
		case <-limit.C:
			b.t.Fatalf("waited %v, and still not: %s", testkit.Patience, what)
		}
	}
}

// trouble stays until Bulldog reports what went wrong where no caller
// waits, and gives it, so it fails no test.
func (b *bench) trouble() error {
	b.t.Helper()
	var err error
	b.until("trouble is reported", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		if len(b.troubles) > 0 {
			err, b.troubles = b.troubles[0], b.troubles[1:]
		}
		return err != nil
	})
	return err
}

// verdict stays until an ask is answered, and gives its answer.
func (b *bench) verdict(ask string) decisions.Verdict {
	b.t.Helper()
	var v *decisions.Verdict
	b.until("the ask "+ask+" is answered", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, a := range b.verdicts {
			if a.ask == ask {
				v = &a.v
			}
		}
		return v != nil
	})
	return *v
}

// answers gives how many times an ask was answered.
func (b *bench) answers(ask string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, a := range b.verdicts {
		if a.ask == ask {
			n++
		}
	}
	return n
}

// let stays until a disclosure has ended, and gives how.
func (b *bench) let(id int64) decisions.Let {
	b.t.Helper()
	var l *decisions.Let
	b.until(fmt.Sprintf("the disclosure %d has ended", id), func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, got := range b.lets {
			if got.Decision == id {
				l = &got
			}
		}
		return l != nil
	})
	return *l
}

// letsOf gives how many times a disclosure was told.
func (b *bench) letsOf(id int64) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, l := range b.lets {
		if l.Decision == id {
			n++
		}
	}
	return n
}

// ended stays until a decision has ended, and gives it.
func (b *bench) ended(id int64) *decisions.Decision {
	b.t.Helper()
	var dec *decisions.Decision
	b.until(fmt.Sprintf("decision %d has ended", id), func() bool {
		dec = b.get(id)
		return dec.Ended != ""
	})
	return dec
}

func (b *bench) get(id int64) *decisions.Decision {
	b.t.Helper()
	dec, err := b.d.Get(id)
	if err != nil {
		b.t.Fatal(err)
	}
	return dec
}

// held hands in an ask holding jobs, as work does, and gives its
// decisions, one for each job, in order.
func (b *bench) held(a decisions.Ask) []*decisions.Decision {
	b.t.Helper()
	if err := b.d.Held(ctx, a); err != nil {
		b.t.Fatal(err)
	}
	decs, err := b.d.OfRequest(a.Request)
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(decs, func(dec *decisions.Decision) bool { return dec.Ask != a.Run })
}

// one hands in an ask holding one job, and gives its decision.
func (b *bench) one(run string, team string, n int64, asking []string, job decisions.Job) *decisions.Decision {
	b.t.Helper()
	decs := b.held(decisions.Ask{Run: run, Team: team, Request: n, Asking: asking, Jobs: []decisions.Job{job}})
	return decs[0]
}

// decide is a person's answer from the screens, which must be taken.
func (b *bench) decide(id int64, person, says, note string) {
	b.t.Helper()
	r, err := b.d.Decide(ctx, id, b.reqs.Person(person), says, note)
	if err != nil {
		b.t.Fatal(err)
	}
	if !r.Taken {
		b.t.Fatalf("%s's %s was not taken: %s", person, says, r.Text)
	}
}

// refused is an answer that must not be taken, and gives what was said.
func (b *bench) refused(id int64, by requests.Who, says string) string {
	b.t.Helper()
	r, err := b.d.Decide(ctx, id, by, says, "")
	if err != nil {
		b.t.Fatal(err)
	}
	if r.Taken {
		b.t.Fatalf("an answer was taken that should not be: %s", r.Text)
	}
	return r.Text
}

// chat is a request a person asks in the chat.
func (b *bench) chat(person, team, text string) int64 {
	b.t.Helper()
	r, err := b.reqs.AskInChat(person, team, text, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	return r.Number
}

// thread gives a request's whole thread as lines: kind, by, text.
func (b *bench) thread(n int64) []string {
	b.t.Helper()
	es, err := b.reqs.Thread(n)
	if err != nil {
		b.t.Fatal(err)
	}
	var lines []string
	for _, e := range es {
		lines = append(lines, fmt.Sprintf("%s by %s: %s", e.Kind, cmpOr(e.By.Name, e.By.Kind), e.Text))
	}
	return lines
}

// entries gives the entries of a request of a kind.
func (b *bench) entries(n int64, kind string) []requests.Entry {
	b.t.Helper()
	es, err := b.reqs.Thread(n)
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(es, func(e requests.Entry) bool { return e.Kind != kind })
}

// seen gives what a person sees of a request as lines.
func (b *bench) seen(n int64, person string) []string {
	b.t.Helper()
	v, err := b.reqs.View(n, person)
	if err != nil {
		b.t.Fatal(err)
	}
	var lines []string
	for _, e := range v.Entries {
		lines = append(lines, e.Kind+": "+e.Text)
	}
	return lines
}

// waitsOn gives the names a request waits on.
func (b *bench) waitsOn(n int64) []string {
	b.t.Helper()
	st, err := b.reqs.State(n)
	if err != nil {
		b.t.Fatal(err)
	}
	var on []string
	for _, p := range st.On {
		on = append(on, p.Name)
	}
	return on
}

// waitingFor gives the decide entries waiting on a person.
func (b *bench) waitingFor(person string) []requests.Wait {
	b.t.Helper()
	ws, err := b.reqs.WaitingOn(person)
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(ws, func(w requests.Wait) bool { return w.Kind != decisions.KindDecide })
}

// has fails unless one of the lines holds every one of the words.
func has(t *testing.T, lines []string, words ...string) string {
	t.Helper()
	for _, line := range lines {
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(line, w) }) {
			return line
		}
	}
	t.Fatalf("no line says %q; the lines are:\n%s", words, strings.Join(lines, "\n"))
	return ""
}

// hasNo fails if any of the lines holds the word.
func hasNo(t *testing.T, lines []string, word string) {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, word) {
			t.Fatalf("a line says %q: %s", word, line)
		}
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// A runner stands in for work running rules and workers: each start waits
// for the test to say how it ends, unless an answer is set for it. A
// start with the labels of one before is that run, as a run already
// started is found by its labels.
type runner struct {
	calls chan *call

	mu     sync.Mutex
	starts []decisions.Start
	auto   func(s decisions.Start) (string, error, bool)
	done   map[string]reply
}

type call struct {
	decisions.Start
	ctx context.Context
	out chan reply
}

type reply struct {
	out string
	err error
}

func newRunner() *runner { return &runner{calls: make(chan *call, 64), done: map[string]reply{}} }

func labelsOf(s decisions.Start) string { return fmt.Sprint(s.Labels) }

func (r *runner) run(ctx context.Context, s decisions.Start) (json.RawMessage, error) {
	r.mu.Lock()
	r.starts = append(r.starts, s)
	auto := r.auto
	kept, ok := r.done[labelsOf(s)]
	r.mu.Unlock()
	if ok {
		return json.RawMessage(kept.out), kept.err
	}
	if auto != nil {
		if out, err, ok := auto(s); ok {
			return json.RawMessage(out), err
		}
	}
	c := &call{Start: s, ctx: ctx, out: make(chan reply, 1)}
	r.calls <- c
	select {
	case got := <-c.out:
		return json.RawMessage(got.out), got.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// answers sets how every later start ends, where answer says so.
func (r *runner) answers(answer func(s decisions.Start) (string, error, bool)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auto = answer
}

// next stays until a rule or worker is started, and gives its call.
func (r *runner) next(t *testing.T) *call {
	t.Helper()
	select {
	case c := <-r.calls:
		return c
	case <-time.After(testkit.Patience):
		t.Fatalf("waited %v, and no rule or worker was started", testkit.Patience)
		return nil
	}
}

// cancelled stays until a call's run is cancelled.
func (c *call) cancelled(t *testing.T) {
	t.Helper()
	select {
	case <-c.ctx.Done():
	case <-time.After(testkit.Patience):
		t.Fatalf("waited %v, and the run of %s was not cancelled", testkit.Patience, c.Name)
	}
}

// says ends a call with what it said.
func (c *call) says(says, note string) {
	c.out <- reply{out: jsonOf(map[string]any{"says": says, "note": note})}
}

// endsWhileAway has a run end while nobody waits on it: the next start
// with its labels is given what it ended with.
func (r *runner) endsWhileAway(c *call, says, note string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done[labelsOf(c.Start)] = reply{out: jsonOf(map[string]any{"says": says, "note": note})}
}

// started gives every start so far.
func (r *runner) started() []decisions.Start {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.starts)
}

func jsonOf(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func raw(v any) json.RawMessage { return json.RawMessage(jsonOf(v)) }

// smallOrders answers as small-orders.js does, for every start of it.
func smallOrders(s decisions.Start) (string, error, bool) {
	if s.Name != "small-orders.js" {
		return "", nil, false
	}
	action, _ := s.Input["action"].(map[string]any)
	args, _ := action["args"].(map[string]any)
	pounds, _ := args["pounds"].(float64)
	if pounds < 20 {
		return jsonOf(map[string]any{"says": "yes", "note": "Under £20."}), nil, true
	}
	return jsonOf(map[string]any{"says": "refer", "note": fmt.Sprintf("£%v is more than I order alone.", pounds)}), nil, true
}

// The jobs of the house's fixer, as Corbie gives the ask them.
func cutPower(circuit string) decisions.Job {
	return decisions.Job{Name: "cut_power", Input: raw(map[string]any{"circuit": circuit}),
		Asked: &decisions.Asked{Does: "Turn one circuit off", Args: raw(map[string]any{"circuit": circuit}), Said: "I will cut circuit " + circuit + " first."}}
}

func orderPart(name string, part string, pounds float64) decisions.Job {
	return decisions.Job{Name: name, Input: raw(map[string]any{"part": part, "pounds": pounds}),
		Asked: &decisions.Asked{Does: "Order a part", Args: raw(map[string]any{"part": part, "pounds": pounds}), Said: "A new socket is needed."}}
}

// refund is the ledger's held refund.
func refund() decisions.Job {
	return decisions.Job{Name: "refund", Input: raw(map[string]any{"charge": "ch_9", "why": "duplicate"}),
		Asked: &decisions.Asked{Does: "Refund one charge", Args: raw(map[string]any{"charge": "ch_9", "why": "duplicate"}),
			Said: "Charge ch_9 is a duplicate of ch_8, taken 20 seconds later."}}
}

// The fixer's ask in the house, the steward's, as the fixer's job reaches
// it.
var fixerUp = []string{"fixer", "steward"}

// helpRequest is Dana's request in #help, and the bot's sub-request of
// payments in it.
func (b *bench) helpRequest() (top, sub int64) {
	b.t.Helper()
	r, err := b.reqs.AskAt("slack", "#help", "C1/100.1", "", "U0DANA00", "customer 4411 was charged twice after Tuesday's release; refund one and tell me what broke.", nil, false)
	if err != nil {
		b.t.Fatal(err)
	}
	s, err := b.reqs.Sub(r.Number, b.reqs.Worker("bot"), "payments", "was 4411 charged twice?", "ticket-payments")
	if err != nil {
		b.t.Fatal(err)
	}
	return r.Number, s.Number
}
