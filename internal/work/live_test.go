package work

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
)

// shownLive gives the server's own run as it stands.
func (b *bench) shownLive(run string) *client.Run {
	b.t.Helper()
	var shown *client.Run
	b.until("the server's own run "+run+" can be read", func() bool {
		r, err := b.corbie.Show(ctx, run)
		if errors.Is(err, client.ErrNoRecord) {
			return false
		}
		if err != nil {
			b.t.Fatal(err)
		}
		shown = r
		return true
	})
	return shown
}

// headed stays until a run going is listed, as another server finds it.
func (b *bench) headed(run string) {
	b.t.Helper()
	b.until("the run "+run+" is listed as going", func() bool {
		return slices.ContainsFunc(b.listed(map[string]string{LabelKind: KindLive}), func(r *client.Run) bool { return r.ID == run && !r.HasEnded() })
	})
}

func TestWorkLiveRunRenewedUntilStopped(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	run, err := b.w.Live(ctx)
	if err != nil {
		t.Fatal(err)
	}
	renewing, stop := context.WithCancel(ctx)
	ended := b.w.Renew(renewing, run)
	// The work settles with the run going, and leaves it to the server.
	b.still()
	step := b.shownLive(run).Step
	// Renewed each twenty seconds, it goes on well past a minute.
	for range 5 {
		b.later(Renewal)
		b.until("the server's own run is renewed", func() bool {
			r := b.shownLive(run)
			return r.Step > step
		})
		step = b.shownLive(run).Step
	}
	if r := b.shownLive(run); r.HasEnded() {
		t.Fatalf("the server's own run ended while it was renewed: %+v", r)
	}
	if b.logged(run) {
		t.Errorf("the work took the server's own run as its own: %v", b.logs)
	}
	// A server that stops ends it.
	stop()
	if err := ended(); err != nil {
		t.Fatal(err)
	}
	if r := b.corbie.Ended(t, run); r.Status != "cancelled" {
		t.Errorf("the stopped server's run ended %s", r.Status)
	}
}

func TestWorkLiveRunEndsItselfWithoutRenewal(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	run, err := b.w.Live(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b.shownLive(run)
	b.later(Quiet - time.Second)
	b.still()
	if r := b.shownLive(run); r.HasEnded() {
		t.Fatalf("the run ended before a minute had passed: %+v", r)
	}
	b.later(time.Second)
	if r := b.corbie.Ended(t, run); r.Status != "done" {
		t.Errorf("the run left without renewal ended %s", r.Status)
	}
}

func TestWorkServingWaitsOnAnotherServersRun(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	// With no other server's run, nothing is waited for.
	if err := Serving(ctx, b.reach, b.clock, identity, b.log); err != nil {
		t.Fatal(err)
	}
	// Another's run that is renewed while this one waits: it serves.
	other, err := b.w.Live(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b.headed(other)
	renewing, stop := context.WithCancel(ctx)
	renewed := b.w.Renew(renewing, other)
	waited := make(chan error, 1)
	go func() { waited <- Serving(ctx, b.reach, b.clock, identity, b.log) }()
	b.until("this one waits on the other's run", func() bool { return b.logged("waiting up to a minute") })
	b.later(Renewal)
	if err := <-waited; !errors.Is(err, ErrServed) {
		t.Fatalf("a renewed run of another server was taken as %v", err)
	}
	stop()
	if err := renewed(); err != nil {
		t.Fatal(err)
	}
	b.corbie.Ended(t, other)
	// A run left by a server that was killed: it ends itself within the
	// minute, and the wait is over.
	left, err := b.w.Live(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b.headed(left)
	count := b.logCount()
	go func() { waited <- Serving(ctx, b.reach, b.clock, identity, b.log) }()
	b.until("this one waits on the run left", func() bool { return len(b.logsSince(count)) > 0 })
	b.later(Quiet)
	if err := <-waited; err != nil {
		t.Fatalf("a run that ended itself was taken as %v", err)
	}
}
