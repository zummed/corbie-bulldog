package work

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zummed/corbie-bulldog/internal/requests"
)

// Stop stops a request for someone who may stop it, as the server judges:
// it and the sub-requests still open beneath it are answered as stopped
// by them, what they put to anyone is closed, their decisions are
// withdrawn, and their runs are cancelled in Corbie, each once. The runs
// are marked as stopping before any cancel is sent; a stop that cannot be
// marked sends no cancel, and the error says so. Its runs are marked when
// their requests are next looked at, and only then cancelled.
func (w *Work) Stop(ctx context.Context, n int64, by requests.Who) error {
	numbers, err := w.requests.Stop(n, by)
	if err != nil {
		return err
	}
	return w.stopped(ctx, numbers)
}

// stopped marks the runs of requests that were stopped as stopping, so
// their cancels are sent, and withdraws their decisions.
func (w *Work) stopped(ctx context.Context, numbers []int64) error {
	err := w.stopping(numbers, stopped)
	if err != nil {
		err = fmt.Errorf("the stop is kept, and its runs are not cancelled until Bulldog can note them as stopping: %w", err)
	} else {
		w.stopsNoted()
	}
	return errors.Join(err, w.decide.Stopped(ctx, numbers))
}

// A KeptRun is a run on a request as it is kept with it.
type KeptRun struct {
	Name   string
	Kind   string
	Team   string
	Worker string
	// Started and Ended are in the office's zone; Ended is zero while it
	// goes.
	Started time.Time
	Ended   time.Time
	// Status is going, done, failed, cancelled, lost or unsent, and How
	// says in words how it ended, for one that did not end done; for a
	// heed's that did, what it said, act or note, and its word.
	Status string
	How    string
	Tokens int
}

// Runs gives the runs of a request, oldest first: the worker, when each
// started and ended, how, and the tokens Corbie counted.
func (w *Work) Runs(n int64) ([]KeptRun, error) {
	notes, err := scanNotes(w.db.Query(noteColumns+` WHERE request = ? ORDER BY rowid`, n))
	if err != nil {
		return nil, err
	}
	zone := time.UTC
	if o := w.keeper.Office(); o != nil {
		zone = o.Zone
	}
	in := func(t time.Time) time.Time {
		if t.IsZero() {
			return t
		}
		return t.In(zone)
	}
	var runs []KeptRun
	for _, n := range notes {
		started := n.Started
		if started.IsZero() {
			started = n.Noted
		}
		runs = append(runs, KeptRun{Name: n.Name, Kind: n.Kind, Team: n.Team, Worker: n.Worker, Started: in(started), Ended: in(n.Ended),
			Status: n.Status, How: n.How, Tokens: n.Tokens})
	}
	return runs, nil
}
