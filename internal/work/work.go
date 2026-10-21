// Package work is Bulldog driving Corbie, and the one part of Bulldog that
// speaks to it. It starts a team's lead on a request with what it is
// given, watches every run of the office, takes how each ends into the
// request it was for, does each call of the tools Bulldog ships and hands
// each held job to the decisions, answering the tickets they wait on,
// cancels what is stopped, keeps each routine's clock, and starts the
// runs the other areas ask for, the bridges' among them. Nothing in Corbie
// calls it. What waits to start, and every run it started, is kept in its
// part of the store, so nothing is started twice and nothing is lost when
// Bulldog stops.
package work

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// The labels every run Bulldog starts carries, beside its team and its
// request, and the kinds of run this area starts of its own: a worker on
// a request, a team's heed weighing what was said in one, and a
// routine's clock.
const (
	LabelBulldog = "bulldog"
	LabelKind    = "kind"

	KindWork  = "work"
	KindHeed  = "heed"
	KindClock = "clock"
)

// The limits work keeps.
const (
	// Again is how long work waits before it looks again at what it could
	// not read or reach.
	Again = 30 * time.Second
	// RunsInARow is how many runs a request is given in a row with nothing
	// said in it by a person.
	RunsInARow = 20
	// Depth is how many levels of sub-requests may be below a request.
	Depth = 8
	// ThreadLimit, NotesLimit and OpenLimit are how much of a thread, of a
	// team's notes and of its open requests a run is given.
	ThreadLimit = 20_000
	NotesLimit  = 50_000
	OpenLimit   = 50
	// DocsLimit is how much one call of a docs tool gives.
	DocsLimit = 20_000
	// ToolWait is how long a shipped tool waits on its ticket.
	ToolWait = 7 * 24 * time.Hour
)

// Corbie is what work uses of Corbie's client: a *client.Client.
type Corbie interface {
	NewRun() (string, error)
	Start(ctx context.Context, draft *machine.Draft, start client.Start) (string, error)
	Show(ctx context.Context, id string) (*client.Run, error)
	Cancel(ctx context.Context, run string, life time.Duration) (string, error)
	List(ctx context.Context, filter client.Filter) ([]*client.Run, error)
	Watch(ctx context.Context, filter client.Filter, interval time.Duration, tell func(client.Update)) error
	Wait(ctx context.Context, id string, interval time.Duration) (*client.Run, error)
	Record(ctx context.Context, id string, tell func(client.Entry)) (*client.Run, error)
	Follow(ctx context.Context, id string, interval time.Duration, tell func(client.Entry)) (*client.Run, error)
	Tickets(ctx context.Context, id string) (map[string]string, error)
	Reply(ctx context.Context, run, ticket string, value any, life time.Duration) (string, error)
	Send(ctx context.Context, run, event string, data any, life time.Duration) (string, error)
	Fetch(ctx context.Context, id, path string) (io.Reader, client.File, error)
}

// A Clock is the one clock of the office, Corbie's and work's alike: what
// time it is, and a channel told each time it moves.
type Clock interface {
	Now() time.Time
	Watch() (<-chan struct{}, func())
}

// Wall is the computer's clock, told to move once a second.
type Wall struct{}

func (Wall) Now() time.Time { return time.Now() }

func (Wall) Watch() (<-chan struct{}, func()) {
	ticks := time.NewTicker(time.Second)
	moved, done := make(chan struct{}, 1), make(chan struct{})
	go func() {
		defer ticks.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticks.C:
				select {
				case moved <- struct{}{}:
				default:
				}
			}
		}
	}()
	var once sync.Once
	return moved, func() { once.Do(func() { close(done) }) }
}

// Areas are the parts of Bulldog work wires together.
type Areas struct {
	Keeper    *office.Keeper
	Requests  *requests.Store
	Decisions *decisions.Decisions
	Changes   *changes.Changes
	Bridges   *bridges.Bridges
}

// Work is one office's driving of Corbie.
type Work struct {
	// Log is told what went wrong, beside wherever a person reads it: the
	// standard log, unless the server says otherwise.
	Log func(format string, args ...any)
	// Interval is how often Corbie's store is looked at when nothing in
	// this process wakes the look.
	Interval time.Duration
	// Took, when it is set, is told each time every area has taken a new
	// reading of the office, which Read then gives. It is told on the
	// driving goroutine, so it only marks or wakes.
	Took func()
	// Counted, when it is set, is told each time how many runs are at
	// work, or which requests wait their turn, changes, which Load and
	// Queue then give. It is told on the driving goroutine, so it only marks or
	// wakes.
	Counted func()

	db       *sql.DB
	corbie   Corbie
	clock    Clock
	identity string
	keeper   *office.Keeper
	requests *requests.Store
	decide   *decisions.Decisions
	changes  *changes.Changes
	bridges  *bridges.Bridges

	// What follows is the driving goroutine's own, touched by nothing
	// else.
	office *office.Office
	// read is the office as the decisions and the bridges last took it.
	read    map[string]*office.Office
	listed  bool
	settled bool
	roots   map[string]bool
	// tree is where each run the watch has told of, and has not seen
	// end, sits: the run it is below, and its machine.
	tree    map[string]branch
	calls   map[string]call
	unknown map[string]bool
	// cancelled are the runs whose cancel went to Corbie and could not be
	// kept as sent.
	cancelled map[string]bool
	queued    map[int64]bool
	troubled  map[int64]string
	// later is what is looked at again at a time: what could not be read
	// or reached, by what it is (see lookAgain).
	later map[string]time.Time
	// scan says every open request is to be looked at; freed that a run
	// began to wait, so a request that waits its turn may start;
	// cancelling that runs are being stopped whose cancel may not have
	// been sent; owing that another area told work what it is to do.
	scan, freed, cancelling, owing bool
	// came are the clocks seen waiting on their ticket; clockSync says
	// the clocks are to be looked for.
	came      map[string]bool
	clockSync bool
	// listenNow and carryNow say the bridges are to be asked for their
	// runs now, and listenAt and carryAt when they said to ask again.
	listenNow, carryNow bool
	listenAt, carryAt   time.Time

	// passed is told each time a pass has done all there was to do.
	passed func()
	// running says Run is driving the office.
	running atomic.Bool
	// taken is the latest reading every area has taken.
	taken atomic.Pointer[office.Office]
	// load is how many runs were at work, and how many requests waited
	// their turn, when the runs going were last counted; queue is which
	// requests waited, in the order they start.
	load  atomic.Pointer[Load]
	queue atomic.Pointer[[]int64]

	// mu guards what other goroutines hand the driving one.
	mu      sync.Mutex
	updates []client.Update
	dirty   map[int64]bool
	look    bool
	stops   bool
	owes    bool
	carry   bool
	wake    chan struct{}
	// hearing are those waiting for what to answer a delivery's service,
	// by the run that hears it.
	hearing map[string]chan *bridges.Answer
	// signing are the sign-in methods' runs going, each with the browser
	// waiting for how it ended, by the run.
	signing map[string]signingRun
	// sending are the runs noted whose starts are being sent, which
	// Corbie may not hold yet.
	sending map[string]bool
	// following are the bridges' listening runs whose records are read as
	// they are written, and handed what they handed over, not yet taken.
	following map[string]bool
	handed    []handover
	// followers are the goroutines that read them.
	followers sync.WaitGroup
}

// A branch is where a run sits in the tree of runs Bulldog started: the
// run it is below, "" at the top, and its machine.
type branch struct {
	parent, machine string
}

// A call is a run below a run on a request that waits on its ticket: a
// shipped tool's, or the ask's, with the request of the run Bulldog noted
// at its top. Waits says it waits on a person, another team or a
// decision, so the run above it is not working.
type call struct {
	request int64
	waits   bool
}

// New makes the driving of an office's work in Corbie, labelling every
// run with the office's identity, and wires the areas to it: it runs the
// decisions' rules and deciding workers, answers Corbie with what they
// decide and is told what they let go, is told, beside the decisions, how
// each change put ended, and has the bridges hand the decisions each
// choice they hear. Nothing is started until Run.
func New(db *sql.DB, c Corbie, clock Clock, identity string, a Areas) (*Work, error) {
	if a.Keeper == nil || a.Requests == nil || a.Decisions == nil || a.Changes == nil || a.Bridges == nil {
		return nil, errors.New("the work needs every area of the office: its keeper, requests, decisions, changes and bridges")
	}
	if err := open(db); err != nil {
		return nil, err
	}
	w := &Work{Log: log.Printf, Interval: client.LocalInterval,
		db: db, corbie: c, clock: clock, identity: identity, keeper: a.Keeper, requests: a.Requests, decide: a.Decisions,
		changes: a.Changes, bridges: a.Bridges, read: map[string]*office.Office{},
		roots: map[string]bool{}, tree: map[string]branch{}, calls: map[string]call{}, unknown: map[string]bool{}, cancelled: map[string]bool{},
		queued: map[int64]bool{}, troubled: map[int64]string{}, later: map[string]time.Time{}, came: map[string]bool{},
		dirty: map[int64]bool{}, wake: make(chan struct{}, 1), hearing: map[string]chan *bridges.Answer{}, signing: map[string]signingRun{}, sending: map[string]bool{},
		following: map[string]bool{}}
	a.Requests.Runs = w.records
	a.Requests.Watch(w.touch)
	a.Decisions.Run = w.decider
	a.Decisions.Answer = w.answerAsk
	a.Decisions.Let = w.let
	a.Decisions.Trouble = func(err error) { w.Log("the decisions: %v", err) }
	a.Changes.Told = w.Told
	choosing(a.Bridges, a.Decisions, func(format string, args ...any) { w.Log(format, args...) })
	return w, nil
}

// labels gives the labels of a run: the office's, and those given.
func (w *Work) labels(given map[string]string) map[string]string {
	labels := maps.Clone(given)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[LabelBulldog] = w.identity
	return labels
}

// touch has a request looked at for a run to start. It is told by the
// requests' watch, on whichever goroutine wrote, so it only marks and
// wakes.
func (w *Work) touch(n int64) {
	w.mu.Lock()
	w.dirty[n] = true
	w.mu.Unlock()
	w.poke()
}

// Read gives the latest reading of the office that the requests, the
// decisions and the bridges have each taken, or nil before they have taken
// any. What another part does by the bridges as the office is read, such
// as sending a person a link where their entry says, is done by it.
func (w *Work) Read() *office.Office { return w.taken.Load() }

// A Load is how many runs are at work in the office, out of as many as it
// lets work at once, and how many requests wait their turn.
type Load struct {
	Working int
	Queued  int
}

// Load gives how many runs were at work, and how many requests waited
// their turn, when the runs going were last counted: none before then.
func (w *Work) Load() Load {
	if l := w.load.Load(); l != nil {
		return *l
	}
	return Load{}
}

// Queue gives the requests that waited their turn when the runs going
// were last counted, in the order they start, oldest first: none before
// then.
func (w *Work) Queue() []int64 {
	if q := w.queue.Load(); q != nil {
		return *q
	}
	return nil
}

// count keeps the load and the queue, and tells Counted when either
// changed.
func (w *Work) count(l Load, queue []int64) {
	old, was := w.load.Swap(&l), w.queue.Swap(&queue)
	if (old == nil || *old != l || !slices.Equal(*was, queue)) && w.Counted != nil {
		w.Counted()
	}
}

// Poke has the office's folder looked at again, and whatever waits on it
// done.
func (w *Work) Poke() {
	w.mu.Lock()
	w.look = true
	w.mu.Unlock()
	w.poke()
}

func (w *Work) poke() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// stopsNoted has the cancels of the runs noted as stopping sent, and
// owesNoted what another area told work done; each may be told on any
// goroutine.
func (w *Work) stopsNoted() {
	w.mu.Lock()
	w.stops = true
	w.mu.Unlock()
	w.poke()
}

func (w *Work) owesNoted() {
	w.mu.Lock()
	w.owes = true
	w.mu.Unlock()
	w.poke()
}

// Run drives the office until the context ends: it reads the office into
// every area, watches every run of the office in Corbie, and does what
// each change asks, on this one goroutine. Once it ends, the decisions
// are closed: the runs they wait on go on in Corbie, and are waited on
// again when the office is run again.
func (w *Work) Run(ctx context.Context) error {
	moved, unwatch := w.clock.Watch()
	defer unwatch()
	w.running.Store(true)
	var watching sync.WaitGroup
	watching.Go(func() { w.watch(ctx) })
	defer func() {
		watching.Wait()
		w.followers.Wait()
		w.running.Store(false)
		w.decide.Close()
	}()
	w.pass(ctx, true)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.wake:
			w.pass(ctx, false)
		case <-moved:
			w.pass(ctx, true)
		}
	}
}

// watch follows every run labelled with the office's identity, handing
// each change to the driving goroutine. Watching that cannot begin is
// begun again a second later.
func (w *Work) watch(ctx context.Context) {
	filter := client.Filter{Labels: map[string]string{LabelBulldog: w.identity}}
	for ctx.Err() == nil {
		// An update with no run says a listing begins: what was known of
		// the runs going is known again from it.
		w.mu.Lock()
		w.updates = append(w.updates, client.Update{})
		w.mu.Unlock()
		err := w.corbie.Watch(ctx, filter, w.Interval, func(u client.Update) {
			w.mu.Lock()
			w.updates = append(w.updates, u)
			w.mu.Unlock()
			w.poke()
		})
		if err == nil || ctx.Err() != nil {
			return
		}
		w.Log("watching the office's runs in Corbie: %v; trying again in a second", err)
		w.after(ctx, time.Second)
	}
}

// after stays until the clock has moved on by d, or the context ends.
func (w *Work) after(ctx context.Context, d time.Duration) {
	moved, unwatch := w.clock.Watch()
	defer unwatch()
	until := w.clock.Now().Add(d)
	for w.clock.Now().Before(until) {
		select {
		case <-ctx.Done():
			return
		case <-moved:
		}
	}
}

// pass does everything there is to do, until nothing is left. When the
// clock has moved, the folder is looked at again and the decisions are
// told the time; what waits on the time is done once it has come.
func (w *Work) pass(ctx context.Context, moved bool) {
	w.mu.Lock()
	look := w.look || moved
	w.look = false
	w.mu.Unlock()
	if look {
		w.keeper.Look()
	}
	if moved && w.office != nil {
		if err := w.decide.Due(ctx); err != nil {
			w.Log("the decisions' time limits: %v", err)
		}
	}
	for ctx.Err() == nil && w.step(ctx) {
	}
	if w.passed != nil {
		w.passed()
	}
}

// step does one round of what there is to do, and says whether it did
// anything. Each part of it does something only when something told it
// to: a change seen in Corbie, a request written to, another area, or the
// time something waited for having come.
func (w *Work) step(ctx context.Context) bool {
	did := w.reread(ctx)
	if w.office == nil {
		return false
	}
	w.mu.Lock()
	updates := w.updates
	w.updates = nil
	w.cancelling = w.cancelling || w.stops
	w.owing = w.owing || w.owes
	w.carryNow = w.carryNow || w.carry
	w.stops, w.owes, w.carry = false, false, false
	w.mu.Unlock()
	for _, u := range updates {
		w.update(ctx, u)
		did = true
	}
	did = w.handOver(ctx) || did
	if w.listed && !w.settled && !w.held("settle") {
		w.settle(ctx)
		did = true
	}
	did = w.lookAgain(ctx) || did
	did = w.cancels(ctx) || did
	if w.owing {
		w.owing = false
		did = w.lets(ctx) || did
		did = w.outcomes(ctx) || did
		did = w.verdicts(ctx) || did
	}
	w.mu.Lock()
	touched := w.dirty
	w.dirty = map[int64]bool{}
	w.mu.Unlock()
	did = w.answers(ctx, touched) || did
	did = w.starts(ctx, touched) || did
	did = w.clocks(ctx) || did
	did = w.comings(ctx) || did
	did = w.bridging(ctx, len(touched) > 0) || did
	return did
}

// again has something looked at again once Again has passed, and held
// says whether it waits for that time still.
func (w *Work) again(key string) { w.later[key] = w.clock.Now().Add(Again) }

func (w *Work) held(key string) bool {
	at, ok := w.later[key]
	return ok && w.clock.Now().Before(at)
}

// lookAgain does again what waited for its time, once it has come. A key
// says what it is, and of what: a run's ending to read again, a call to
// do or give its kept answer again, a request to look at again, cancels
// or what another area told work to do again, the calls kept to take up
// again, or the runs of workers no longer in the office to stop. The rest, the open requests, the clocks, settling and each
// clock's coming round, are held until their time by whatever does them.
func (w *Work) lookAgain(ctx context.Context) bool {
	now := w.clock.Now()
	did := false
	for _, key := range slices.Sorted(maps.Keys(w.later)) {
		if now.Before(w.later[key]) {
			continue
		}
		delete(w.later, key)
		did = true
		kind, what, _ := strings.Cut(key, ":")
		switch kind {
		case "show":
			w.ended(ctx, what)
		case "call":
			w.callAgain(ctx, what)
		case "give":
			w.giveAgain(ctx, what)
		case "request", "start", "stop":
			n, err := strconv.ParseInt(what, 10, 64)
			if err != nil {
				w.Log("%q, to be looked at again, names no request: %v", key, err)
				continue
			}
			w.touch(n)
		case "cancel", "cancels":
			w.cancelling = true
		case "let", "outcome", "verdict", "owed":
			w.owing = true
		case "away":
			w.callsAway(ctx)
		case "sweep":
			w.sweep(w.office)
		}
	}
	return did
}

// callAgain reads a call that could not be read or done, and does it.
func (w *Work) callAgain(ctx context.Context, run string) {
	r, err := w.corbie.Show(ctx, run)
	if err != nil {
		w.Log("the call %s could not be read, so it is looked at again in %v: %v", run, Again, err)
		w.again("call:" + run)
		return
	}
	w.child(ctx, r)
}

// giveAgain gives a call its kept answer, which could not be given.
func (w *Work) giveAgain(ctx context.Context, run string) {
	switch row, err := w.callOf(run); {
	case err != nil:
		w.Log("the answer kept for %s could not be read, and is given again in %v: %v", run, Again, err)
		w.again("give:" + run)
	case row != nil && row.State == answered:
		w.giveSoon(ctx, row)
	}
}
