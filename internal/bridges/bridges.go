// Package bridges joins the office to outside services. It keeps one in
// run going for each bridge, takes each thing an in run hands over, as a
// new request or an answer to a wait, and carries a message out by a run
// of a bridge's out.
package bridges

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
)

// relook is how long the bridges wait, after something they did failed,
// before they do it again.
const relook = 30 * time.Second

// The Bridges of an office: a client of its Corbie, which hands what the
// bridges hear to the office's requests engine. Nothing important is kept
// in memory: what is kept here is read again from the store after a
// restart, and taking a thing twice changes nothing.
type Bridges struct {
	// Engine is the office's requests engine: the bridges hand it what
	// they hear, and read the office from its reading. It is set before
	// the bridges or the engine run.
	Engine *requests.Engine
	// Report is told what went wrong, for a person to read. By default
	// it is written to the standard log.
	Report func(error)
	// Interval is how often the bridges look at the store when nothing
	// written in this process wakes them.
	Interval time.Duration

	corbie  *client.Client
	secret  []byte
	wake    chan struct{}
	follows sync.WaitGroup
	// taking takes one emit of things at a time, so a thing is taken
	// after the things heard before it.
	taking sync.Mutex

	mu      sync.Mutex
	runs    map[string]*inRun    // the office's in runs this process follows, by name
	stands  map[string]*standing // how each bridge's runs have gone, by its name
	reading *requests.Reading    // the reading defs are of
	defs    map[string]*defs     // each bridge's definitions, by its name
}

// An inRun is one in run of the office, as this process follows it.
type inRun struct {
	bridge    string
	made      string
	given     any  // the cursor this process started it with
	cancelled bool // this process cancelled it
	ended     bool
	taken     bool // it has ended, and everything it handed over is taken
}

// A standing is how a bridge's in runs have gone lately.
type standing struct {
	run   string    // its newest in run: the one started last, or listed last
	ended int       // how many of its runs ended in a row, none going well
	next  time.Time // by Corbie's clock, when its next run may start
	err   string    // Corbie's error, while the bridge fails
}

// New makes the bridges of an office, a client of its Corbie, with the
// office's secret.
func New(corbie *client.Client, secret []byte) *Bridges {
	return &Bridges{Report: func(err error) { log.Print(err) }, Interval: 5 * time.Second,
		corbie: corbie, secret: secret, wake: make(chan struct{}, 1), runs: map[string]*inRun{}, stands: map[string]*standing{}}
}

// Run keeps the office's in runs going and takes what they hand over,
// until the context ends. It follows every in run of the office the store
// keeps, so what was heard while Bulldog was stopped is taken. It returns
// an error only when that first listing fails.
func (b *Bridges) Run(ctx context.Context) error {
	listed, err := b.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": b.Engine.Reading().Office.ID, "kind": "bridge"}, Finished: true})
	if err != nil {
		return err
	}
	defer b.follows.Wait()
	slices.SortFunc(listed, func(a, b *client.Run) int { return strings.Compare(a.ID, b.ID) })
	for _, run := range listed {
		if run.Parent == "" {
			b.mu.Lock()
			b.runs[run.ID] = &inRun{bridge: run.Labels["bridge"], made: run.Labels["made"], ended: run.HasEnded()}
			b.standing(run.Labels["bridge"]).run = run.ID
			b.mu.Unlock()
			b.follow(ctx, run.ID)
		}
	}
	tick := time.NewTicker(b.Interval)
	defer tick.Stop()
	var quiet time.Time
	for {
		if time.Now().After(quiet) {
			if err := b.keep(ctx); err != nil && ctx.Err() == nil {
				b.Report(err)
				quiet = time.Now().Add(relook)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		case <-b.wake:
		}
	}
}

// keep keeps one in run going for each bridge of the office that has no
// problem: the bridge's newest run, when it runs the bridge as the office
// has it. It cancels every other run. When a bridge has no run going, it
// starts one, given the last cursor of the bridge's newest run: at once,
// or after a pause when the bridge's runs keep ending.
func (b *Bridges) keep(ctx context.Context) error {
	r := b.Engine.Reading()
	all := b.definitions(r)
	now := b.corbie.Now()
	b.mu.Lock()
	going := map[string]bool{}
	var cancels []string
	for name, in := range b.runs {
		d, newest := all[in.bridge], name == b.newest(in.bridge)
		switch {
		case in.ended:
			if in.taken && !newest {
				delete(b.runs, name)
			}
			continue
		case in.cancelled:
		case newest && d.usable() && d.made == in.made:
		default:
			in.cancelled = true
			cancels = append(cancels, name)
		}
		going[in.bridge] = true
	}
	type start struct {
		bridge *office.Bridge
		defs   *defs
		last   string
	}
	var starts []start
	for _, br := range r.Office.Bridges {
		d, s := all[br.Name], b.standing(br.Name)
		switch {
		case going[br.Name] || !d.usable():
		case s.run != "" && b.runs[s.run].made == d.made && now.Before(s.next):
		default:
			starts = append(starts, start{br, d, s.run})
		}
	}
	b.mu.Unlock()

	var errs []error
	for _, name := range cancels {
		if _, err := b.corbie.Cancel(ctx, name, 0); err != nil {
			b.mu.Lock()
			b.runs[name].cancelled = false
			b.mu.Unlock()
			errs = append(errs, fmt.Errorf("the in run %s: %w", name, err))
		}
	}
	for _, s := range starts {
		if err := b.start(ctx, r.Office.ID, s.bridge, s.defs, s.last); err != nil {
			errs = append(errs, fmt.Errorf("the bridge %s: %w", s.bridge.Name, err))
		}
	}
	return errors.Join(errs...)
}

// start starts an in run of a bridge, given the last cursor of the
// bridge's last run, and follows it.
func (b *Bridges) start(ctx context.Context, id string, br *office.Bridge, d *defs, last string) error {
	cursor, err := b.cursor(ctx, last)
	if err != nil {
		return err
	}
	name, err := b.corbie.Start(ctx, d.in, client.Start{
		Input:  map[string]any{"settings": br.Settings, "cursor": cursor},
		Labels: map[string]string{"bulldog": id, "kind": "bridge", "bridge": br.Name, "made": d.made}})
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.runs[name] = &inRun{bridge: br.Name, made: d.made, given: cursor}
	b.standing(br.Name).run = name
	b.mu.Unlock()
	b.follow(ctx, name)
	return nil
}

// cursor gives the cursor a bridge's next in run is given: the last one
// its last run emitted, or the one that run was given when it emitted
// none. A bridge's first run is given none.
func (b *Bridges) cursor(ctx context.Context, last string) (any, error) {
	if last == "" {
		return nil, nil
	}
	b.mu.Lock()
	given := b.runs[last].given
	b.mu.Unlock()
	var heard *client.Entry
	run, err := b.corbie.Record(ctx, last, func(e client.Entry) {
		if e.Job != "" && e.Name == "heard" && (heard == nil || !before(e, *heard)) {
			heard = &e
		}
	})
	switch {
	case errors.Is(err, client.ErrNoRun):
		return given, nil
	case err != nil:
		return nil, err
	case heard != nil:
		data, _ := heard.Data.(map[string]any)
		return data["cursor"], nil
	}
	if input, ok := run.Input.(map[string]any); ok {
		if cursor, ok := input["cursor"]; ok {
			return cursor, nil
		}
	}
	return given, nil
}

// follow reads an in run's record in a goroutine of its own, as it is
// written, until the run has ended and everything it handed over is
// taken. What cannot be taken is reported, and the record is read again
// from the start a little later.
func (b *Bridges) follow(ctx context.Context, id string) {
	b.follows.Add(1)
	go func() {
		defer b.follows.Done()
		for {
			run, err := b.read(ctx, id)
			if ctx.Err() != nil {
				return
			}
			switch {
			case errors.Is(err, client.ErrNoRun):
				b.ended(id, nil, err)
				err = nil
			case run != nil && run.HasEnded():
				b.ended(id, run, nil)
			}
			if err == nil {
				b.mu.Lock()
				b.runs[id].taken = true
				b.mu.Unlock()
				return
			}
			b.Report(fmt.Errorf("the in run %s: %w", id, err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(relook):
			}
		}
	}()
}

// read follows an in run's record from the start, and acts on each entry,
// until the run has ended or an entry cannot be acted on.
func (b *Bridges) read(ctx context.Context, id string) (*client.Run, error) {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	var failed error
	run, err := b.corbie.Follow(ctx, id, b.Interval, func(e client.Entry) {
		if failed == nil {
			if failed = b.entry(ctx, id, e); failed != nil {
				stop()
			}
		}
	})
	return run, errors.Join(failed, err)
}

// entry acts on one entry of an in run's record: it takes what a heard
// emit hands over, and notes how the bridge goes from each job that ends.
func (b *Bridges) entry(ctx context.Context, id string, e client.Entry) error {
	switch {
	case e.Job != "" && e.Name == "heard":
		b.went(id, "")
		return b.heard(ctx, id, e.Data)
	case e.Step > 0:
		for _, event := range e.Events {
			switch {
			case event.From == "":
			case event.Name == "done":
				b.went(id, "")
			case event.Name == "failed":
				b.went(id, errorOf(event.Data))
			}
		}
	}
	return nil
}

// errorOf gives Corbie's error from a failed event's data.
func errorOf(data any) string {
	failed, _ := data.(map[string]any)
	if text, _ := failed["error"].(string); text != "" {
		return text
	}
	return "a job of the in run failed"
}

// went notes how a job of a bridge's newest run went: it failed, with
// Corbie's error, or, with none, it went well.
func (b *Bridges) went(id, err string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	in := b.runs[id]
	if id != b.newest(in.bridge) {
		return
	}
	s := b.standing(in.bridge)
	if s.err = err; err == "" {
		s.ended = 0
	}
}

// ended notes that an in run has ended. When the newest run of a bridge
// ends of itself, the bridge fails with Corbie's error, and its next run
// waits a pause that grows with each run that ends in a row.
func (b *Bridges) ended(id string, run *client.Run, gone error) {
	b.mu.Lock()
	in := b.runs[id]
	if in.ended {
		b.mu.Unlock()
		return
	}
	in.ended = true
	if id == b.newest(in.bridge) && !in.cancelled && (run == nil || run.Status != "cancelled") {
		s := b.standing(in.bridge)
		s.ended++
		end := b.corbie.Now()
		if run != nil && !run.Ended.IsZero() {
			end = run.Ended
		}
		s.next = end.Add(pause(s.ended))
		switch {
		case gone != nil:
			s.err = gone.Error()
		case run.Error != "":
			s.err = run.Error
		default:
			s.err = "its in run ended " + run.Status
		}
	}
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// pause is how long a bridge waits before it starts a run again, when
// that many of its runs ended in a row: a second, doubling up to a minute.
func pause(ended int) time.Duration {
	return min(time.Second<<min(ended-1, 6), time.Minute)
}

// newest gives the name of a bridge's newest in run, or "" for none. The
// caller holds mu.
func (b *Bridges) newest(bridge string) string {
	return b.standing(bridge).run
}

// standing gives how a bridge's runs have gone. The caller holds mu.
func (b *Bridges) standing(bridge string) *standing {
	if b.stands[bridge] == nil {
		b.stands[bridge] = &standing{}
	}
	return b.stands[bridge]
}

// A State is how a bridge's in run stands, for the office's page.
type State struct {
	Status string `json:"status"`         // running, failing or stopped
	Error  string `json:"error,omitzero"` // Corbie's error, while it fails
}

// State gives how a bridge stands. It is stopped when it has a problem or
// no in run is going. It is failing, with Corbie's error, from a job or a
// run of it that failed until one goes well, and running otherwise.
func (b *Bridges) State(bridge string) State {
	d := b.definitions(b.Engine.Reading())[bridge]
	switch {
	case d == nil:
		return State{Status: "stopped"}
	case d.err != nil:
		return State{Status: "failing", Error: d.err.Error()}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if s := b.stands[bridge]; s != nil && s.err != "" {
		return State{Status: "failing", Error: s.err}
	}
	for _, in := range b.runs {
		if in.bridge == bridge && !in.ended {
			return State{Status: "running"}
		}
	}
	return State{Status: "stopped"}
}

// before says whether one emit of a run was written before another: by its
// time, then by its job, a job of a state asked again coming after the
// one asked before it.
func before(a, b client.Entry) bool {
	if !a.Time.Equal(b.Time) {
		return a.Time.Before(b.Time)
	}
	aAsked, aAttempt := sequence(a.Job)
	bAsked, bAttempt := sequence(b.Job)
	return aAsked < bAsked || aAsked == bAsked && aAttempt < bAttempt
}

// sequence gives where a job comes among the jobs of its state, from its
// name: how many times the state asked for it, as hearing#2, and its
// attempt, as hearing#2@3.
func sequence(job string) (asked, attempt int) {
	asked, attempt = 1, 1
	if name, n, ok := strings.Cut(job, "@"); ok {
		attempt, _ = strconv.Atoi(n)
		job = name
	}
	if _, n, ok := strings.Cut(job, "#"); ok {
		asked, _ = strconv.Atoi(n)
	}
	return asked, attempt
}
