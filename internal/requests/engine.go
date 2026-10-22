package requests

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync/atomic"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/store"
)

// relook is how long the engine waits, after something it did failed,
// before it acts on every run of the office again.
const relook = 30 * time.Second

// interval is how often the store is looked at when nothing written in
// this process wakes the look.
const interval = 5 * time.Second

// A Reading is one reading of the office, with the request machine of each
// of its teams.
type Reading struct {
	Office   *office.Office
	Machines map[string]*Machine
}

// NewReading makes the request machine of each team of an office. Make
// adds what it finds wrong to the office, so the office is given here
// before anything else reads it.
func NewReading(o *office.Office) *Reading {
	return &Reading{Office: o, Machines: Make(o)}
}

// Takes says whether a team takes requests: it is a team of the office,
// and nothing stops it.
func (r *Reading) Takes(team string) bool {
	m := r.Machines[team]
	return m != nil && m.Draft != nil
}

// A Carrier carries a message out on a bridge, by a run of the bridge's
// out.yaml. The program fills it with the bridges.
type Carrier interface {
	// Carry starts a run of the bridge's out under the name and with the
	// labels the message gives. The engine calls it only when no run has
	// the name.
	Carry(ctx context.Context, m Message) error
	// Direct says whether a bridge's out reaches a person directly.
	Direct(bridge string) bool
}

// A Message is one message carried out on a bridge: what its out.yaml is
// given but the settings, and the run that carries it.
type Message struct {
	Bridge  string
	To      string // where on the bridge, or with Direct a person's lasting id there
	Direct  bool
	Text    string
	Choices []string // offered as buttons
	Ref     string   // for a question, its wait's ref, which a press gives back
	Answers string   // the id of the thing it replies to
	Run     string   // the name of the run that carries it
	Labels  map[string]string
}

// An Engine is Bulldog driving Corbie for one office. It starts requests,
// follows every run of the office, answers the tickets they wait on and
// carries their answers out. It keeps nothing important in memory: after
// a restart it finds everything again in the store, and doing a thing
// twice changes nothing.
type Engine struct {
	corbie  *client.Client
	carrier Carrier
	secret  []byte
	reading atomic.Pointer[Reading]
	swapped chan struct{}
	// starting are the clocks this engine started that no listing has
	// shown yet, by their run, each with its routine's fingerprint. Only
	// Run's goroutine reads and writes it.
	starting map[string]string

	// Report is told what went wrong while the engine follows the runs,
	// for a person to read. By default it is written to the standard log.
	Report func(error)
}

// New makes an engine for the office of a reading, a client of its Corbie,
// with the office's secret.
func New(corbie *client.Client, carrier Carrier, secret []byte, r *Reading) *Engine {
	e := &Engine{corbie: corbie, carrier: carrier, secret: secret, swapped: make(chan struct{}, 1), starting: map[string]string{},
		Report: func(err error) { log.Print(err) }}
	e.reading.Store(r)
	return e
}

// Reading gives the office as last read.
func (e *Engine) Reading() *Reading { return e.reading.Load() }

// Swap puts a new reading of the office in place of the last, when the
// office is read again. The routines' clocks follow it.
func (e *Engine) Swap(r *Reading) {
	e.reading.Store(r)
	select {
	case e.swapped <- struct{}{}:
	default:
	}
}

// Run follows every run of the office, and acts on each as it changes,
// until the context ends. What fails is reported, and every run is acted
// on again a little later.
func (e *Engine) Run(ctx context.Context) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	updates := make(chan client.Update)
	watched := make(chan error, 1)
	filter := client.Filter{Labels: map[string]string{"bulldog": e.Reading().Office.ID}, Finished: true}
	go func() {
		watched <- e.corbie.Watch(ctx, filter, interval, func(u client.Update) {
			select {
			case updates <- u:
			case <-ctx.Done():
			}
		})
	}()
	var again <-chan time.Time
	for {
		var err error
		select {
		case <-ctx.Done():
			return <-watched
		case err := <-watched:
			return err
		case u := <-updates:
			switch {
			case u.Listed:
				err = e.keepClocks(ctx)
			case u.Run != nil:
				if err = e.take(ctx, u.Run, u.New); err != nil {
					err = fmt.Errorf("the run %s: %w", u.Run.ID, err)
				}
			}
		case <-e.swapped:
			err = e.keepClocks(ctx)
		case <-again:
			again = nil
			err = e.everything(ctx)
		}
		if err != nil && ctx.Err() == nil {
			e.Report(err)
			if again == nil {
				again = time.After(relook)
			}
		}
	}
}

// everything acts on every run of the office again, as the first listing
// after a start does.
func (e *Engine) everything(ctx context.Context) error {
	runs, err := e.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": e.Reading().Office.ID}, Finished: true})
	if err != nil {
		return err
	}
	var errs []error
	for _, run := range runs {
		if err := e.take(ctx, run, false); err != nil {
			errs = append(errs, fmt.Errorf("the run %s: %w", run.ID, err))
		}
	}
	return errors.Join(append(errs, e.keepClocks(ctx))...)
}

// take acts on one run of the office, as a listing gives it.
func (e *Engine) take(ctx context.Context, run *client.Run, new bool) error {
	r := e.Reading()
	switch {
	case run.Machine == "wait" && waits(run, "answer"):
		return e.asked(ctx, r, run)
	case run.Machine == "wait" && run.HasEnded():
		return e.waitEnded(ctx, run)
	case run.Parent != "":
	case run.Labels["kind"] == "request" && run.HasEnded():
		return e.ended(ctx, r, run)
	case run.Labels["kind"] == "clock" && run.HasEnded():
		return e.keepClocks(ctx)
	case run.Labels["kind"] == "clock" && new:
		return errors.Join(e.came(ctx, r, run), e.keepClocks(ctx))
	case run.Labels["kind"] == "clock":
		return e.came(ctx, r, run)
	}
	return nil
}

// waits says whether a run is going and waits on a ticket.
func waits(run *client.Run, ticket string) bool {
	return !run.HasEnded() && slices.Contains(run.Tickets, ticket)
}

// hash gives a short keyed hash of parts, keyed with the office's secret:
// what Bulldog writes in labels, run names and refs in place of what it was
// made from.
func (e *Engine) hash(parts ...string) string {
	mac := hmac.New(sha256.New, e.secret)
	for _, part := range parts {
		mac.Write([]byte(part))
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

// name gives the name of a run Bulldog starts in answer to something: the
// time of the thing, and a keyed hash of what it is.
func (e *Engine) name(at time.Time, parts ...string) string {
	return e.corbie.ID.Name + "/" + store.Stamp(at) + "-" + e.hash(parts...)
}

// named gives the time in a run's name.
func named(run string) time.Time {
	at, _ := store.InstanceTime(run)
	return at
}

// once starts a run under a name, unless a run of that name is there.
func (e *Engine) once(ctx context.Context, name string, start func() error) error {
	_, err := e.corbie.Show(ctx, name)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, client.ErrNoRun):
		return err
	}
	return start()
}

// labels gives the labels of a run Bulldog starts: the office's, the kind
// of run, and the pairs given.
func (e *Engine) labels(kind string, pairs ...string) map[string]string {
	labels := map[string]string{"bulldog": e.Reading().Office.ID, "kind": kind}
	for i := 0; i+1 < len(pairs); i += 2 {
		labels[pairs[i]] = pairs[i+1]
	}
	return labels
}

// roots lists the runs of the office that no run started, narrowed by a
// filter whose labels are added to the office's.
func (e *Engine) roots(ctx context.Context, filter client.Filter) ([]*client.Run, error) {
	labels := map[string]string{"bulldog": e.Reading().Office.ID}
	for name, value := range filter.Labels {
		labels[name] = value
	}
	filter.Labels = labels
	runs, err := e.corbie.List(ctx, filter)
	return slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Parent != "" }), err
}

// decode reads a value as plain data into another shape.
func decode(v, into any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}
