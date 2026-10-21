package work

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/store"
)

// A step makes its trips to Corbie for things that do not depend on one
// another side by side: it reads first what it will take, decides and
// writes each in turn, and then sends the answers to tickets it decided
// on. What a step read and has to send is its own, and gone when it ends.

// trips are the step's own trips to Corbie: the runs it read side by side,
// as Corbie showed each, the tickets each waits on, and the answers it
// decided on, sent side by side once it has decided everything.
type trips struct {
	shown   map[string]shown
	tickets map[string]ticketsRead
	replies []reply
}

// A shown is a run as Corbie showed it, or what kept it from being shown.
type shown struct {
	run *client.Run
	err error
}

// A ticketsRead is the tickets a run waits on, or what kept them from being read.
type ticketsRead struct {
	tickets map[string]string
	err     error
}

// A reply is an answer to a ticket the step decided on: the run, the
// ticket and the answer; the request of the run Bulldog noted at its top,
// so a tool of a run being stopped is given nothing, and 0 for an ask's
// verdict, which goes all the same; what is done once it has gone, or
// could not go; and why it cannot be sent, when it cannot.
type reply struct {
	run, ticket string
	value       any
	request     int64
	then        func(error)
	err         error
}

// errNotBegun is a trip not begun, as Bulldog stopped.
var errNotBegun = errors.New("it was not begun, as Bulldog was stopping")

// sideBySide makes n trips to Corbie side by side, at most as many at once
// as Corbie's own store.SideBySide, and waits for them all. Each gives its
// own error, whatever another gives; one not begun, as the context ended,
// gives errNotBegun.
func sideBySide(ctx context.Context, n int, trip func(i int) error) []error {
	errs := make([]error, n)
	for i := range errs {
		errs[i] = errNotBegun
	}
	store.Each(ctx, store.SideBySide, n, func(i int) error {
		errs[i] = trip(i)
		return nil
	})
	return errs
}

// readRuns reads side by side how Corbie shows each run named in show,
// and the tickets each run named in tickets waits on, for the step to take
// them; what the step read already is not read again. A run whose start
// is being sent is read when it is taken, beside whether it is still being
// sent, since a start that has gone in between is no longer.
func (w *Work) readRuns(ctx context.Context, show, tickets []string) {
	show = slices.DeleteFunc(slices.Compact(slices.Sorted(slices.Values(show))), func(id string) bool {
		_, read := w.trips.shown[id]
		return read || w.isSending(id)
	})
	tickets = slices.DeleteFunc(slices.Compact(slices.Sorted(slices.Values(tickets))), func(id string) bool {
		_, read := w.trips.tickets[id]
		return read
	})
	runs, waits := make([]*client.Run, len(show)), make([]map[string]string, len(tickets))
	errs := sideBySide(ctx, len(show)+len(tickets), func(i int) (err error) {
		if i < len(show) {
			runs[i], err = w.corbie.Show(ctx, show[i])
			return err
		}
		waits[i-len(show)], err = w.corbie.Tickets(ctx, tickets[i-len(show)])
		return err
	})
	for i, id := range show {
		w.trips.shown[id] = shown{runs[i], errs[i]}
	}
	for i, id := range tickets {
		w.trips.tickets[id] = ticketsRead{waits[i], errs[len(show)+i]}
	}
}

// show gives a run as Corbie shows it: as the step read it, or, for one it
// did not read, read now.
func (w *Work) show(ctx context.Context, id string) (*client.Run, error) {
	if s, read := w.trips.shown[id]; read {
		return s.run, s.err
	}
	return w.corbie.Show(ctx, id)
}

// ticketsOf gives the tickets a run waits on, each name with its value: as
// the step read them, or, for a run it did not read them of, read now.
func (w *Work) ticketsOf(ctx context.Context, id string) (map[string]string, error) {
	if h, read := w.trips.tickets[id]; read {
		return h.tickets, h.err
	}
	return w.corbie.Tickets(ctx, id)
}

// sendReplies sends the answers the step decided on, side by side, those
// to one run in turn, and does what each asks once it has gone or could
// not go. A tool of a run being stopped is given nothing, though its stop
// was noted only while the step decided, so the run cannot go on before
// its cancel reaches it; its kept answer goes when the run ends.
func (w *Work) sendReplies(ctx context.Context) bool {
	replies := w.trips.replies
	if len(replies) == 0 {
		return false
	}
	notes, err := w.goingNotes()
	if err != nil {
		// Nothing goes until it is known which runs are being stopped.
		for _, r := range replies {
			r.then(fmt.Errorf("the runs going could not be read, to give nothing to the tools of those being stopped: %w", err))
		}
		return true
	}
	stopping := map[int64]bool{}
	for _, n := range notes {
		if n.Kind == KindWork && n.Stop != "" {
			stopping[n.Request] = true
		}
	}
	replies = slices.DeleteFunc(replies, func(r reply) bool { return stopping[r.request] })
	byRun := map[string][]int{}
	errs := make([]error, len(replies))
	for i, r := range replies {
		errs[i] = cmp.Or(r.err, errNotBegun)
		if r.err == nil {
			byRun[r.run] = append(byRun[r.run], i)
		}
	}
	runs := slices.Sorted(maps.Keys(byRun))
	sideBySide(ctx, len(runs), func(i int) error {
		for _, at := range byRun[runs[i]] {
			r := replies[at]
			_, errs[at] = w.corbie.Reply(ctx, r.run, r.ticket, r.value, Life)
		}
		return nil
	})
	for i, r := range replies {
		r.then(errs[i])
	}
	return true
}
