// Package server is Bulldog's program put together: it readies an office's
// folder, opens its store and gives every area its part, reads the office,
// takes the address it listens on, and only then starts the office's work
// and its own run, which says while it goes that the office is served. It
// stops all of it, in order, when it is told to.
package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// Listen is where the server listens when it is told nowhere.
const Listen = "127.0.0.1:7410"

// Options are what a server is started with.
type Options struct {
	// Dir is the office's folder, readied, and Identity the office's.
	Dir      string
	Identity Identity
	// Corbie is the client of the Corbie the office is run on, and Store
	// the address of its store, as it may be shown.
	Corbie *client.Client
	Store  string
	// Clock is the office's clock: work.Wall, unless a test moves one.
	Clock work.Clock
	// Listen is the address to listen on, and URL how a person reaches
	// the server, the listen address when it is "".
	Listen string
	URL    string
	// Version is which Bulldog this is: dev when it is "".
	Version string
	// Out is the log.
	Out io.Writer
	// Begun, when it is given, is told the office's areas once its work
	// has started.
	Begun func(Areas)
}

// Areas are the parts of an office the server put together.
type Areas struct {
	work.Areas
	Work *work.Work
}

// Shown gives a store's address as it may be printed: a folder as it is,
// and anything else without the name, the password and the settings it
// may carry. An address that cannot be read is not repeated.
func Shown(address string) string {
	if !strings.Contains(address, "://") {
		return address
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme == "" {
		return "a store whose address cannot be read"
	}
	return u.Scheme + "://" + u.Host + u.Path
}

// Serve serves an office until the context ends, and then stops all it
// started, waits for it, and closes the store. A start goes in order: it
// says which Corbie the office is on, opens the store, makes sure no
// other server is serving the office, and only then reads the office's
// folder, opens every area in the store, and says what the reading found;
// it takes the address it listens on, says at the first start of a new
// version what changed in each thing Bulldog ships that the office uses,
// and only then starts its own run and the office's work, and gives each
// person the link they sign in with. An address that cannot be taken
// starts nothing.
func Serve(ctx context.Context, opt Options) error {
	say := Log(opt.Out)
	c := opt.Corbie
	if c.ID == nil {
		return errors.New("this computer's config has a client with no identity of its own, which can only look and can start no run: corbie client card makes one, for the controller to enrol")
	}
	controller := c.Controller.Name
	say("on the Corbie of the controller %s, whose store is %s", controller, Shown(opt.Store))
	db, err := OpenStore(opt.Dir)
	if err != nil {
		return err
	}
	defer db.Close()
	switch err := work.Serving(ctx, c, opt.Clock, opt.Identity.ID, say); {
	case errors.Is(err, work.ErrServed):
		return fmt.Errorf("another server is serving the office in %s, on the Corbie of the controller %s: stop the server already serving it; or, if this folder is a copy meant as an office of its own, remove its .bulldog/ and start again", opt.Dir, controller)
	case ctx.Err() != nil:
		return nil
	case err != nil:
		return err
	}
	keeper := office.Keep(opt.Dir)
	areas, err := open(ctx, db, opt, keeper)
	if err != nil {
		return err
	}
	// The areas are closed once all that was started has ended.
	defer func() {
		areas.Decisions.Close()
		areas.Bridges.Close()
	}()
	areas.Work.Log = say
	r := &readings{say: say, db: db, keeper: keeper, clock: opt.Clock, dir: opt.Dir, taken: areas.Work.Read, took: make(chan struct{}, 1), logged: map[string]bool{}}
	areas.Work.Took = r.tell
	r.office()
	r.problems()
	r.shown = keeper.Office()

	listener, err := net.Listen("tcp", opt.Listen)
	if err != nil {
		return fmt.Errorf("the server cannot listen on %s, so nothing is started: %w", opt.Listen, err)
	}
	r.url = strings.TrimRight(cmp.Or(opt.URL, "http://"+listener.Addr().String()), "/")
	say("listening on %s", listener.Addr())
	// The version is kept only by a start that listens, so one that
	// cannot still says what changed when it next starts.
	version := cmp.Or(opt.Version, "dev")
	before, upgraded, err := ran(db, version)
	if err != nil {
		listener.Close()
		return err
	}
	if upgraded {
		for _, line := range changedLines(keeper.Office(), before) {
			say("%s", line)
		}
	}
	s := &site{Areas: areas, db: db, clock: opt.Clock, url: r.url, say: say, screens: &screens{}, asked: make(chan string, Asked),
		version: version, before: before}
	areas.Bridges.SignedIn = s.signedIn
	r.changed = func() { s.screens.tell(Office) }
	areas.Work.Counted = func() { s.screens.tell(Runs) }
	serving, stopServing := context.WithCancel(ctx)
	defer stopServing()
	h := &http.Server{Handler: routes(s), BaseContext: func(net.Listener) context.Context { return serving }}
	listened := make(chan error, 1)
	go func() { listened <- h.Serve(listener) }()

	// What is started from here is stopped in the order it was started,
	// once the context ends or the server can no longer listen.
	live, err := areas.Work.Live(ctx)
	if err != nil {
		stopServing()
		h.Close()
		<-listened
		return err
	}
	living, stopLiving := context.WithCancel(context.WithoutCancel(ctx))
	defer stopLiving()
	renewed := areas.Work.Renew(living, live)
	running, stopRunning := context.WithCancel(context.WithoutCancel(ctx))
	defer stopRunning()
	var going sync.WaitGroup
	going.Go(func() {
		if err := areas.Work.Run(running); err != nil {
			say("the office's work stopped: %v", err)
		}
	})
	// The clock is watched before the watching starts, so that no move is
	// missed in between; the links are given once every area has taken the
	// reading.
	moved, unwatch := opt.Clock.Watch()
	defer unwatch()
	going.Go(func() { r.watch(running, moved) })
	going.Go(func() { s.signing(running) })
	if opt.Begun != nil {
		opt.Begun(areas)
	}

	var failed error
	select {
	case <-ctx.Done():
	case err := <-listened:
		failed = fmt.Errorf("the server stopped listening on %s: %w", listener.Addr(), err)
	}
	stopServing()
	if err := h.Shutdown(context.WithoutCancel(ctx)); err != nil {
		say("the server's answers could not all be finished: %v", err)
	}
	if failed == nil {
		if err := <-listened; !errors.Is(err, http.ErrServerClosed) {
			failed = fmt.Errorf("the server stopped listening on %s: %w", listener.Addr(), err)
		}
	}
	stopRunning()
	going.Wait()
	stopLiving()
	if err := renewed(); err != nil {
		say("%v", err)
	}
	return failed
}

// Log gives what says a line of the log, one line at a time however many
// say them at once.
func Log(out io.Writer) func(format string, args ...any) {
	var mu sync.Mutex
	return func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(out, "bulldog: "+format+"\n", args...)
	}
}
