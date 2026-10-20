package work

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// A bridge's machines are run as any worker is, and labelled with the
// kinds the bridges name: one listening run of each bridge that listens,
// a hearing run for each delivery to a bridge's address, and an out run
// for each thing to carry. How each ended is handed back to the bridges.
// Nobody watches them: what a bridge heard is everything said in every
// place it listens in.

// bridging starts the runs the bridges ask for: listening runs after the
// office is read, when one ends, and at the time the bridges said to look
// again; out runs after the office is read, when a request changed, when
// one ends, and at the time the bridges said.
func (w *Work) bridging(ctx context.Context, touched bool) bool {
	now := w.clock.Now()
	did := false
	if w.listenNow || !w.listenAt.IsZero() && !now.Before(w.listenAt) {
		w.listenNow, w.listenAt = false, time.Time{}
		starts, at, err := w.bridges.Listen()
		if err != nil {
			w.Log("the bridges' listening runs could not be looked at, so they are looked at again in %v: %v", Again, err)
			at = now.Add(Again)
		}
		w.listenAt = at
		for _, s := range starts {
			w.startBridge(ctx, s, nil)
			did = true
		}
	}
	if touched || w.carryNow || !w.carryAt.IsZero() && !now.Before(w.carryAt) {
		w.carryNow, w.carryAt = false, time.Time{}
		starts, at, err := w.bridges.Carry()
		if err != nil {
			w.Log("what the bridges carry could not be looked at, so it is looked at again in %v: %v", Again, err)
			at = now.Add(Again)
		}
		w.carryAt = at
		for _, s := range starts {
			w.startBridge(ctx, s, nil)
			did = true
		}
	}
	return did
}

// Carry has the bridges asked now for the out runs to start, as when
// something is kept to be carried that no request's write tells of: a
// one-time sign-in link. It may be told on any goroutine.
func (w *Work) Carry() {
	w.mu.Lock()
	w.carry = true
	w.mu.Unlock()
	w.poke()
}

// Hear starts a bridge's in machine on a delivery to its address, and
// gives a channel sent what to answer the service once the run has ended
// and what it heard is taken: nil to say the delivery was received.
func (w *Work) Hear(s bridges.Start) (<-chan *bridges.Answer, error) {
	answered := make(chan *bridges.Answer, 1)
	if err := w.startBridge(context.Background(), s, answered); err != nil {
		return nil, err
	}
	return answered, nil
}

// startBridge starts a run a bridge asks for, noted before its start is
// sent and kept by the bridges as theirs. A start that cannot be sent is
// handed back to the bridges as a run that failed, which they try again
// after a pause.
func (w *Work) startBridge(ctx context.Context, s bridges.Start, answered chan *bridges.Answer) error {
	name, err := w.corbie.NewRun()
	if err != nil {
		return w.bridgeUnsent(s, "", fmt.Errorf("no name could be chosen for its run: %w", err))
	}
	labels := w.labels(s.Labels)
	n := &note{Name: name, Kind: s.Kind, Team: labels[requests.LabelTeam], Worker: s.Bridge, Labels: labels, Noted: w.clock.Now()}
	if err := w.keep(n); err != nil {
		return w.bridgeUnsent(s, "", fmt.Errorf("its run could not be noted: %w", err))
	}
	if err := w.bridges.Started(s, name); err != nil {
		w.endUnsent(name, err)
		return w.bridgeUnsent(s, "", fmt.Errorf("the bridges could not keep its run: %w", err))
	}
	if answered != nil {
		w.mu.Lock()
		w.hearing[name] = answered
		w.mu.Unlock()
	}
	if err := w.send(ctx, s.Machine, client.Start{Run: name, Input: s.Input, Labels: labels}); err != nil {
		w.mu.Lock()
		delete(w.hearing, name)
		w.mu.Unlock()
		failed := w.bridgeUnsent(s, name, fmt.Errorf("Corbie was not reached: %w", err))
		w.endUnsent(name, err)
		return failed
	}
	return nil
}

// endUnsent keeps a run whose start could not be sent as not sent.
func (w *Work) endUnsent(name string, why error) {
	w.sendingDone(name)
	now := w.clock.Now()
	if _, err := w.end(name, ending{status: unsent, how: why.Error(), started: now, ended: now}); err != nil {
		w.Log("the run %s, which was not sent, could not be kept so: %v", name, err)
	}
}

// bridgeUnsent hands a bridge's run that could not be started back to the
// bridges, as one that failed, and gives why.
func (w *Work) bridgeUnsent(s bridges.Start, run string, why error) error {
	why = fmt.Errorf("Bulldog could not start the %s run of the bridge %s: %w", s.Kind, s.Bridge, why)
	w.Log("%v", why)
	end := bridges.End{Failed: why.Error()}
	var err error
	if s.Kind == bridges.KindIn {
		_, err = w.bridges.InEnded(s.Bridge, run, end)
	} else {
		err = w.bridges.OutEnded(s.Thing, run, end)
	}
	if err != nil {
		w.Log("the bridge %s could not be told its run was not started: %v", s.Bridge, err)
	}
	return why
}

// handBack hands how a bridge's run ended back to the bridges, and says
// whether it was taken: one that could not be is taken again once Again
// has passed. What an in run heard closed at its source stops the
// requests it began, and a hearing run's service is told what to answer.
func (w *Work) handBack(ctx context.Context, n *note, end bridges.End) bool {
	end.Fetch = w.fetch(ctx, n.Name)
	var err error
	if n.Kind == bridges.KindIn {
		var taken bridges.Taken
		if taken, err = w.bridges.InEnded(n.Worker, n.Name, end); err == nil {
			w.listenNow = true
			w.heard(n.Name, taken.Answer)
			w.stopClosed(ctx, n.Worker, taken.Stopped)
		}
	} else {
		var thing int64
		if thing, err = strconv.ParseInt(n.Labels[bridges.LabelThing], 10, 64); err == nil {
			err = w.bridges.OutEnded(thing, n.Name, end)
		}
		w.carryNow = true
	}
	if err != nil {
		w.Log("how the %s run %s of the bridge %s ended could not be taken, so it is taken again in %v: %v", n.Kind, n.Name, n.Worker, Again, err)
		w.showAgain(n.Name)
		return false
	}
	return true
}

// fetch reads a blob of a run's record by the path it was shown with.
func (w *Work) fetch(ctx context.Context, run string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		r, _, err := w.corbie.Fetch(ctx, run, path)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	}
}

// stopClosed stops the runs of the requests a bridge heard closed at their
// source.
func (w *Work) stopClosed(ctx context.Context, bridge string, numbers []int64) {
	if len(numbers) == 0 {
		return
	}
	if err := w.stopped(ctx, numbers); err != nil {
		w.Log("the requests %v were closed at their source on %s, and their runs could not be stopped: %v", numbers, bridge, err)
	}
}

// A listening run may hand over what it heard while it goes on, by
// emitting it as heard. Its record is read as it is written, on a
// goroutine of its own, and each handover is taken on the driving one.
// Once the run has ended, its whole record is read for them again before
// how it ended is handed back, so a handover read late, or never because
// Bulldog was stopped, is taken all the same; a thing is taken once, by
// its id, however often it is read.

// A handover is one list of things a bridge's listening run handed over.
type handover struct {
	bridge string
	run    string
	heard  any
}

// follow has the record of a run of a bridge's in machine read as it is
// written, once, when it is seen going and it is the run the bridge
// listens by.
func (w *Work) follow(ctx context.Context, r *client.Run) {
	if r.Labels[bridges.LabelKind] != bridges.KindIn {
		return
	}
	w.mu.Lock()
	following := w.following[r.ID]
	w.mu.Unlock()
	if following {
		return
	}
	bridge := r.Labels[bridges.LabelBridge]
	switch run, err := w.bridges.Listening(bridge); {
	case err != nil:
		w.Log("whether %s is the listening run of the bridge %s could not be read, so what it hands over is taken when it ends: %v", r.ID, bridge, err)
		return
	case run != r.ID:
		return
	}
	w.mu.Lock()
	w.following[r.ID] = true
	w.mu.Unlock()
	w.followers.Go(func() { w.reading(ctx, bridge, r.ID) })
}

// reading hands the driving goroutine each list of things a listening run
// hands over, as its record is written, until the run ends. Reading that
// fails is begun again a second later, from the start of the record.
func (w *Work) reading(ctx context.Context, bridge, run string) {
	defer func() {
		w.mu.Lock()
		delete(w.following, run)
		w.mu.Unlock()
	}()
	for ctx.Err() == nil {
		_, err := w.corbie.Follow(ctx, run, w.Interval, func(e client.Entry) {
			if heard, ok := handedOver(run, e); ok {
				w.mu.Lock()
				w.handed = append(w.handed, handover{bridge: bridge, run: run, heard: heard})
				w.mu.Unlock()
				w.poke()
			}
		})
		if err == nil || ctx.Err() != nil || errors.Is(err, client.ErrNoRun) {
			return
		}
		w.Log("reading what the listening run %s of the bridge %s hands over: %v; trying again in a second", run, bridge, err)
		w.after(ctx, time.Second)
	}
}

// handedOver gives what an entry of a listening run's record hands over:
// a value one of its own jobs emitted as heard.
func handedOver(run string, e client.Entry) (any, bool) {
	return e.Data, e.ID == run && e.Job != "" && e.Name == bridges.EmitHeard
}

// handOver hands the bridges what their listening runs handed over since
// the last pass, and says whether there was any. What cannot be taken now
// is taken when its run ends.
func (w *Work) handOver(ctx context.Context) bool {
	w.mu.Lock()
	handed := w.handed
	w.handed = nil
	w.mu.Unlock()
	for _, h := range handed {
		taken, err := w.bridges.Handed(h.bridge, bridges.End{Handed: []any{h.heard}, Fetch: w.fetch(ctx, h.run)})
		if err != nil {
			w.Log("what the listening run %s of the bridge %s handed over could not be taken, so it is taken when the run ends: %v", h.run, h.bridge, err)
		}
		w.stopClosed(ctx, h.bridge, taken.Stopped)
	}
	return len(handed) > 0
}

// handedBy gives everything a bridge's listening run handed over, read
// from its record, once it has ended. Any other run hands over nothing.
func (w *Work) handedBy(ctx context.Context, n *note) ([]any, error) {
	if n.Kind != bridges.KindIn {
		return nil, nil
	}
	run, err := w.bridges.Listening(n.Worker)
	if err != nil || run != n.Name {
		return nil, err
	}
	var handed []any
	_, err = w.corbie.Record(ctx, n.Name, func(e client.Entry) {
		if heard, ok := handedOver(n.Name, e); ok {
			handed = append(handed, heard)
		}
	})
	return handed, err
}

// heard sends what to answer a delivery's service to whoever waits for
// it, once.
func (w *Work) heard(run string, answer *bridges.Answer) {
	w.mu.Lock()
	answered := w.hearing[run]
	delete(w.hearing, run)
	w.mu.Unlock()
	if answered != nil {
		answered <- answer
	}
}

// choosing has the bridges give a choice heard to the decision its entry
// asks, and carry each decision with its choices. Typed words are read as
// the choice, its first word, and a note, the rest.
func choosing(b *bridges.Bridges, d *decisions.Decisions, log func(string, ...any)) {
	b.Choices = func(e requests.Entry) []string {
		_, ok, err := d.OfEntry(e.ID)
		if err != nil {
			log("whether entry %d asks a decision could not be read, so it is carried with no choices: %v", e.ID, err)
		}
		if !ok {
			return nil
		}
		return decisions.Choices()
	}
	b.Choose = func(c bridges.Choice) (string, error) {
		dec, ok, err := d.OfEntry(c.Entry)
		switch {
		case err != nil:
			return "Your answer was not taken, since the decision could not be read: try again.", err
		case !ok:
			return "That is no decision of this office, so nothing was taken.", nil
		}
		says, note := c.Choice, ""
		if says == "" {
			words := strings.Fields(c.Text)
			if len(words) > 0 {
				says, note = strings.ToLower(strings.Trim(words[0], ".,!:;")), strings.Join(words[1:], " ")
			}
		}
		reply, err := d.Decide(context.Background(), dec.ID, c.Who, says, note)
		return reply.Text, err
	}
}
