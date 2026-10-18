package work

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/instance"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/shipped"
)

// KindLive is the kind of the server's own run, which says while it goes
// that a server is serving the office. Work leaves it to the server.
const KindLive = "live"

// Renewal is how often the server's own run is renewed, and Quiet how long
// it goes without a renewal before it ends itself, as the live machine
// Bulldog ships says.
const (
	Renewal = 20 * time.Second
	Quiet   = time.Minute
)

// ErrServed says another server is serving the office: its own run was
// renewed while this one waited on it.
var ErrServed = errors.New("another server is serving the office")

// isLive says whether a run is a server's own.
func isLive(r *client.Run) bool { return r.Labels[LabelKind] == KindLive }

// Serving looks in Corbie, by the office's identity, for a run of
// another server of the office, and waits on each that is going, saying
// so in the log: once one is renewed, another server is serving the
// office, and it gives ErrServed; once each has ended, or when there is
// none, it gives nil. A run neither renewed nor ended once Quiet and Again
// have passed is not being run, and it says so. It reads nothing but
// Corbie, so a server asks it before it opens or reads anything of the
// office.
func Serving(ctx context.Context, c Corbie, clock Clock, identity string, log func(format string, args ...any)) error {
	runs, err := c.List(ctx, client.Filter{Labels: map[string]string{LabelBulldog: identity, LabelKind: KindLive}})
	if err != nil {
		return fmt.Errorf("Corbie cannot be asked whether another server is serving the office: %w", err)
	}
	for _, r := range runs {
		if r.Parent != "" || r.HasEnded() {
			continue
		}
		if err := waitOnLive(ctx, c, clock, log, r.ID); err != nil {
			return err
		}
	}
	return nil
}

// waitOnLive follows another server's own run until it is renewed, it
// ends, or it has been neither for too long.
func waitOnLive(ctx context.Context, c Corbie, clock Clock, log func(format string, args ...any), id string) error {
	shown, err := c.Show(ctx, id)
	switch {
	case errors.Is(err, client.ErrNoRun):
		return nil
	case errors.Is(err, client.ErrNoRecord):
		shown = &client.Run{ID: id}
	case err != nil:
		return fmt.Errorf("the run %s, of a server of this office, cannot be read: %w", id, err)
	case shown.HasEnded():
		return nil
	}
	moved, unwatch := clock.Watch()
	defer unwatch()
	log("a server of this office has a run going in Corbie, %s: waiting up to a minute to see whether it is still serving the office", id)
	limit := clock.Now().Add(Quiet + Again)
	following, stop := context.WithCancel(ctx)
	defer stop()
	renewed := false
	go func() {
		for following.Err() == nil && clock.Now().Before(limit) {
			select {
			case <-following.Done():
			case <-moved:
			}
		}
		stop()
	}()
	run, err := c.Follow(following, id, client.LocalInterval, func(e client.Entry) {
		if e.ID == id && e.Step > shown.Step && slices.ContainsFunc(e.Events, func(a instance.Arrived) bool { return a.Name == shipped.Renew }) {
			renewed = true
			stop()
		}
	})
	switch {
	case renewed:
		return ErrServed
	case ctx.Err() != nil:
		return ctx.Err()
	case err != nil:
		return fmt.Errorf("the run %s, of a server of this office, cannot be followed: %w", id, err)
	case run != nil && run.HasEnded():
		return nil
	}
	return fmt.Errorf("the run %s, of a server of this office, has been neither renewed nor ended for %v, as it would be by a controller that is running: is no controller running it?", id, Quiet+Again)
}

// Live starts the server's own run, of the live machine Bulldog ships,
// labelled with the office's identity, and gives its name.
func (w *Work) Live(ctx context.Context) (string, error) {
	contents, _ := shipped.File(shipped.Live)
	draft, err := machine.Parse(contents, nil)
	if err != nil {
		return "", fmt.Errorf("the server's own run cannot be made: %w", err)
	}
	run, err := w.corbie.Start(ctx, draft, client.Start{Labels: w.labels(map[string]string{LabelKind: KindLive})})
	if err != nil {
		return "", fmt.Errorf("the server's own run cannot be started: %w", err)
	}
	return run, nil
}

// Renew keeps the server's own run going until the context ends, renewing
// it each time Renewal has passed by the office's clock, and then ends it.
// A run found to have ended is started again; a renewal that cannot be
// sent is said, and sent again at the next. The clock is watched before it
// gives back the func that stays until the run is ended, and says how.
func (w *Work) Renew(ctx context.Context, run string) (ended func() error) {
	moved, unwatch := w.clock.Watch()
	done := make(chan error, 1)
	go func() {
		defer unwatch()
		done <- w.renew(ctx, run, moved)
	}()
	return func() error { return <-done }
}

func (w *Work) renew(ctx context.Context, run string, moved <-chan struct{}) error {
	next := w.clock.Now().Add(Renewal)
	for {
		select {
		case <-ctx.Done():
			if _, err := w.corbie.Cancel(context.WithoutCancel(ctx), run, 0); err != nil {
				return fmt.Errorf("the server's own run, %s, could not be ended, and ends itself within a minute: %w", run, err)
			}
			return nil
		case <-moved:
		}
		if w.clock.Now().Before(next) || ctx.Err() != nil {
			continue
		}
		next = w.clock.Now().Add(Renewal)
		switch shown, err := w.corbie.Show(ctx, run); {
		case err == nil && shown.HasEnded():
			w.Log("the server's own run, %s, has ended (%s), so another is started", run, shown.Status)
			again, err := w.Live(ctx)
			if err != nil {
				w.Log("%v; it is tried again in %v", err, Renewal)
				continue
			}
			run = again
		case err != nil && !errors.Is(err, client.ErrNoRecord):
			w.Log("the server's own run, %s, cannot be read, and is renewed all the same: %v", run, err)
			fallthrough
		default:
			if _, err := w.corbie.Send(ctx, run, shipped.Renew, nil, Renewal); err != nil {
				w.Log("the server's own run, %s, could not be renewed, and is renewed again in %v: %v", run, Renewal, err)
			}
		}
	}
}
