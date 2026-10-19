package decisions

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// Read takes a new reading of the office, after Requests has taken it,
// and walks every decision that waits on over its lists as now read: who
// is no longer on them is no longer asked, a rule or worker no longer on
// them is cancelled, a disclosure whose team no longer sets a disclosure check
// is let go, and one that nobody can now decide is refused; an answer the
// requests hold that was never handed to its disclosure check, as when
// Bulldog stopped in between, is handed to it now. The first
// reading also takes up the runs of rules and workers that were deciding
// when Bulldog stopped, to read what they end with, and does what a
// disclosure or a change that ended before Bulldog stopped had still to do.
func (d *Decisions) Read(ctx context.Context, o *office.Office) error {
	return d.doing(ctx, func(fx *effects) error {
		d.state.Lock()
		first := d.office == nil
		d.office = o
		d.state.Unlock()
		if first {
			owed, err := some(d.db, `owed != ''`)
			if err != nil {
				return err
			}
			fx.owed = append(fx.owed, owed...)
		}
		decs, err := d.Waiting()
		if err != nil {
			return err
		}
		for _, dec := range decs {
			if err := d.advance(o, dec, fx); err != nil {
				return err
			}
		}
		held, err := d.requests.HeldAnswers()
		if err != nil {
			return err
		}
		for _, e := range held {
			if _, err := d.keepDisclosure(o, heldSaid(e), fx); err != nil {
				return err
			}
		}
		return nil
	})
}

// Due passes each rule that has not ended within RuleLimit of its start,
// and each worker asked to decide that has not within WorkerLimit,
// cancelling its run, and walks the decision on. Work calls it from its
// goroutine each time its clock moves; a call with nothing due changes
// nothing. It must not be called from a requests watcher: a write the
// decisions make to the requests tells its watchers in the middle of a
// change to the decisions, which Due would wait on for ever.
func (d *Decisions) Due(ctx context.Context) error {
	return d.changing(ctx, func(o *office.Office, fx *effects) error {
		decs, err := d.Waiting()
		if err != nil {
			return err
		}
		now := d.Now()
		for _, dec := range decs {
			if dec.At == nil || dec.At.Kind == isPeople {
				continue
			}
			limit, words := RuleLimit, "15 minutes"
			if dec.At.Kind == office.IsWorker {
				limit, words = WorkerLimit, "an hour"
			}
			if now.Before(dec.At.Since.Add(limit)) {
				continue
			}
			d.stopRun(dec.ID)
			p := Pass{Kind: dec.At.Kind, Name: dec.At.Name, Called: calledOf(o, dec.At.Kind, dec.At.Name), At: now,
				Note: fmt.Sprintf("it did not end within %s of its start, so it was cancelled", words)}
			dec.At = nil
			if err := d.pass(o, dec, p); err != nil {
				return err
			}
			if err := d.advance(o, dec, fx); err != nil {
				return err
			}
		}
		return nil
	})
}

// Watch has fn told the number of each decision a change touched, once
// for each, after it is kept. It gives a func that ends the watch.
func (d *Decisions) Watch(fn func(id int64)) func() {
	d.state.Lock()
	defer d.state.Unlock()
	if d.watchers == nil {
		d.watchers = map[int]func(int64){}
	}
	id := d.next
	d.next++
	d.watchers[id] = fn
	return func() {
		d.state.Lock()
		defer d.state.Unlock()
		delete(d.watchers, id)
	}
}

func (d *Decisions) tell(touched []int64) {
	d.state.Lock()
	var fns []func(int64)
	for _, id := range slices.Sorted(maps.Keys(d.watchers)) {
		fns = append(fns, d.watchers[id])
	}
	d.state.Unlock()
	for _, id := range touched {
		for _, fn := range fns {
			fn(id)
		}
	}
}
