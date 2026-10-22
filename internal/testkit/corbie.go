package testkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/controller"
	"github.com/zummed/corbie/identity"
	"github.com/zummed/corbie/permission"
	"github.com/zummed/corbie/role"
	"github.com/zummed/corbie/runner"
	"github.com/zummed/corbie/secret"
	"github.com/zummed/corbie/store"
)

// Key is the controller secret every model of a test names, and KeyValue
// is its value.
const (
	Key      = "TEST_KEY"
	KeyValue = "sk-test"
)

// Patience is how long Until waits before the test fails. A test that
// passes never waits it out.
const Patience = time.Minute

// A Corbie is every role in the test's process, with the store in memory:
// a controller, a runner and a client of the controller. Every role keeps
// the same Clock. The ceiling of the runner and of the client is all, so a
// job may do whatever its definition and its start allow.
type Corbie struct {
	*client.Client
	Memory     *store.Memory
	Controller *controller.Controller
	Runner     *runner.Runner
	Clock      *Clock
}

// NewCorbie starts a Corbie with its clock at Monday, with the secret Key
// set on the controller, and stops it when the test ends.
func NewCorbie(t testing.TB) *Corbie {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	mem := store.NewMemory()
	ctl, err1 := identity.New("controller", identity.Controller)
	run, err2 := identity.New("runner", identity.Runner)
	cli, err3 := identity.New("client", identity.Client)
	if err := errors.Join(err1, err2, err3); err != nil {
		t.Fatal(err)
	}
	as := func(kind store.Kind, name string) store.Store {
		return mem.As(store.Party{Kind: kind, Name: name, Controller: ctl.Name})
	}
	all := permission.Layer{Allow: []string{"all"}}
	ctlStore := as(store.Controller, ctl.Name)
	if err := store.SetUp(ctx, ctlStore); err != nil {
		t.Fatal(err)
	}
	for _, trust := range []identity.Trust{{Card: run.Card}, {Card: cli.Card, Ceiling: all}} {
		if err := identity.WriteTrust(ctx, ctlStore, ctl, trust); err != nil {
			t.Fatal(err)
		}
	}
	if err := secret.Set(ctx, ctlStore, ctl, Key, KeyValue); err != nil {
		t.Fatal(err)
	}

	clock := NewClock(Monday)
	c := &Corbie{Memory: mem, Clock: clock}
	c.Controller = controller.New(ctl, ctlStore)
	c.Controller.Now = clock.Now
	c.Runner = runner.New(run, runner.Served{Controller: ctl.Card, Ceiling: all, Store: as(store.Runner, run.Name)})
	c.Runner.Now = clock.Now
	c.Runner.Command = []string{os.Args[0], runner.JobArg}
	c.Runner.Scratch = t.TempDir()
	c.Client = client.New(cli, ctl.Card, as(store.Client, cli.Name))
	c.Client.Now = clock.Now

	// A role passes again when its inbox is written, and when the clock
	// moves, since what falls due falls due by the test's clock.
	controllerWake := c.wake(ctx, ctlStore, store.Inbox(ctl.Name))
	runnerWake := c.wake(ctx, ctlStore, store.Inbox(run.Name))
	done := make(chan struct{})
	go func() {
		defer close(done)
		role.Run(ctx, clock.Now, func(name string, err error) { t.Logf("corbie's %s: %v", name, err) },
			role.Role{Name: "controller", Pass: c.Controller.Pass, Interval: client.LocalInterval, Notice: controllerWake},
			role.Role{Name: "runner", Pass: c.Runner.Pass, Interval: client.LocalInterval, Notice: runnerWake})
	}()
	t.Cleanup(func() {
		stop()
		<-done
	})
	return c
}

// wake gives a channel that is sent to when a name under the path is
// written or the clock moves, until the context ends.
func (c *Corbie) wake(ctx context.Context, s store.Store, under string) <-chan struct{} {
	written, stopWritten := s.Notice(under)
	moved, stopMoved := c.Clock.watch()
	ch := make(chan struct{}, 1)
	go func() {
		defer stopWritten()
		defer stopMoved()
		for {
			select {
			case <-ctx.Done():
				return
			case <-written:
			case <-moved:
			}
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	return ch
}

// Until stays until done gives true, calling it again each time anything
// is written to the store or the clock moves. After Patience the test
// fails, naming what it waited for.
func (c *Corbie) Until(t testing.TB, what string, done func() bool) {
	t.Helper()
	written, stopWritten := c.Memory.As(store.Party{Kind: store.Owner}).Notice("")
	defer stopWritten()
	moved, stopMoved := c.Clock.watch()
	defer stopMoved()
	limit := time.NewTimer(Patience)
	defer limit.Stop()
	for !done() {
		select {
		case <-written:
		case <-moved:
		case <-limit.C:
			t.Fatalf("waited %v, and still not: %s", Patience, what)
		}
	}
}

// Ended stays until a run has ended, and gives it.
func (c *Corbie) Ended(t testing.TB, run string) *client.Run {
	t.Helper()
	var shown *client.Run
	c.Until(t, "the run "+run+" has ended", func() bool {
		shown = c.show(t, run)
		return shown != nil && shown.HasEnded()
	})
	return shown
}

// Waiting stays until a run waits on a ticket, and gives the ticket's
// value, which answers it.
func (c *Corbie) Waiting(t testing.TB, run, ticket string) string {
	t.Helper()
	var value string
	c.Until(t, fmt.Sprintf("the run %s waits on the ticket %s", run, ticket), func() bool {
		if shown := c.show(t, run); shown == nil || !slices.Contains(shown.Tickets, ticket) {
			return false
		}
		tickets, err := c.Tickets(context.Background(), run)
		if err != nil {
			t.Fatal(err)
		}
		value = tickets[ticket]
		return value != ""
	})
	return value
}

// show gives a run as it is now, or nil while the store has no run of the
// name.
func (c *Corbie) show(t testing.TB, run string) *client.Run {
	t.Helper()
	shown, err := c.Show(context.Background(), run)
	if errors.Is(err, client.ErrNoRecord) || errors.Is(err, client.ErrNoRun) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return shown
}
