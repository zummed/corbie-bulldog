package work

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/shipped"
)

// The machines Bulldog ships that wait on a ticket below a run, by their
// names.
const (
	callMachine = "call"
	askMachine  = "ask"
)

// update takes one change the watch saw.
func (w *Work) update(ctx context.Context, u client.Update) {
	switch {
	case u.Listed:
		w.listed, w.settled = true, false
		return
	case u.Run == nil:
		w.listed, w.settled = false, false
		w.roots, w.calls = map[string]bool{}, map[string]call{}
		return
	}
	r := u.Run
	if isLive(r) {
		// The server's own run is the server's to keep and end.
		return
	}
	if r.Parent != "" {
		w.child(ctx, r)
		return
	}
	if !r.HasEnded() {
		w.roots[r.ID] = true
		w.follow(ctx, r)
		if hasCome(r) {
			w.came[r.ID] = true
		}
		if w.settled {
			w.known(ctx, r.ID)
		}
		return
	}
	delete(w.roots, r.ID)
	delete(w.unknown, r.ID)
	w.ended(ctx, r.ID)
}

// child takes a run below a run Bulldog started: a shipped tool's call
// or an ask, waiting on its ticket, is done, and kept with the request it
// is for and whether it waits on a person, another team or a decision,
// which leaves the run above it not working; one that has ended lets its
// kept answer go. A call that cannot be read or done yet is looked at
// again.
func (w *Work) child(ctx context.Context, r *client.Run) {
	if r.HasEnded() {
		delete(w.calls, r.ID)
		delete(w.later, "call:"+r.ID)
		if err := w.callEnded(ctx, r); err != nil {
			w.Log("the ending of the call %s could not be taken: %v", r.ID, err)
		}
		return
	}
	if _, kept := w.calls[r.ID]; kept || w.isLater("call:"+r.ID) {
		return
	}
	var request int64
	var waits bool
	var err error
	switch {
	case r.Machine == askMachine && slices.Contains(r.Tickets, shipped.AskTicket):
		waits = true
		request, err = w.ask(ctx, r)
	case r.Machine == callMachine && slices.Contains(r.Tickets, shipped.CallTicket):
		request, waits, err = w.call(ctx, r)
	default:
		return
	}
	if err != nil {
		w.Log("the call %s could not be done yet, so it is looked at again in %v: %v", r.ID, Again, err)
		w.again("call:" + r.ID)
		return
	}
	w.calls[r.ID] = call{request: request, waits: waits}
	w.freed = w.freed || waits
}

// isLater says whether something is to be looked at again, at any time.
func (w *Work) isLater(key string) bool {
	_, ok := w.later[key]
	return ok
}

// waiting says whether the run on a request waits on a person, another
// team or a decision.
func (w *Work) waiting(request int64) bool {
	for _, c := range w.calls {
		if c.request == request && c.waits {
			return true
		}
	}
	return false
}

// settle is done once the watch has listed every run going: each run
// noted as going that is not is read for how it ended, each run going
// that was never noted is cancelled, every open request is looked at,
// and the cancels and what other areas told work before it stopped are
// done. One that cannot read what it noted is done again later.
func (w *Work) settle(ctx context.Context) {
	notes, err := w.goingNotes()
	if err != nil {
		w.Log("the runs noted as going could not be read, so they are read again in %v: %v", Again, err)
		w.again("settle")
		return
	}
	w.settled = true
	w.scan, w.cancelling, w.owing = true, true, true
	for _, n := range notes {
		if !w.roots[n.Name] {
			w.ended(ctx, n.Name)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(w.roots)) {
		w.known(ctx, id)
	}
	w.callsAway(ctx)
}

// callsAway takes up the calls kept that the listing did not show waiting:
// one that ended while Bulldog was away lets its kept answer go, and the
// requests of the rest, and every sub-request whose answers may not all
// have been given, are looked at for what came for them meanwhile. What
// cannot be read is taken up again later.
func (w *Work) callsAway(ctx context.Context) {
	calls, err := w.everyCall()
	if err == nil {
		var subs []int64
		subs, err = w.openSubs()
		for _, n := range subs {
			w.touch(n)
		}
	}
	if err != nil {
		w.Log("the calls kept could not be read, to take up what came for them while Bulldog was away, so they are read again in %v: %v", Again, err)
		w.again("away")
		return
	}
	for _, c := range calls {
		if c.Request != 0 {
			w.touch(c.Request)
		}
		if _, seen := w.calls[c.Run]; seen || w.isLater("call:"+c.Run) {
			continue
		}
		r, err := w.corbie.Show(ctx, c.Run)
		switch {
		case errors.Is(err, client.ErrNoRun):
			r = &client.Run{ID: c.Run, Status: "cancelled"}
		case err != nil:
			w.Log("the call %s could not be read, so it is looked at again in %v: %v", c.Run, Again, err)
			w.again("call:" + c.Run)
			continue
		}
		if r.HasEnded() {
			w.child(ctx, r)
		}
	}
}

// known cancels a run going at the top that Bulldog did not note for this
// office, or noted as never started: nothing else would end it.
func (w *Work) known(ctx context.Context, id string) {
	if w.unknown[id] {
		return
	}
	n, err := w.noted(id)
	if err != nil {
		w.Log("the run %s could not be looked up: %v", id, err)
		return
	}
	if n != nil && n.Status == going {
		return
	}
	w.unknown[id] = true
	w.Log("cancelling %s, a run with the office's label that Bulldog did not start for it", id)
	if _, err := w.corbie.Cancel(ctx, id, 0); err != nil {
		w.Log("the run %s could not be cancelled: %v", id, err)
		delete(w.unknown, id)
	}
}

// ended reads how a run noted as going ended, and takes it, once. A run
// whose record cannot be read is read again later, and never taken to
// have ended from what could not be read. A run whose start was taken as
// not sent, and which Corbie ran all the same, has how it ended kept with
// it, and nothing else.
func (w *Work) ended(ctx context.Context, name string) {
	n, err := w.noted(name)
	switch {
	case err != nil:
		w.Log("the run %s could not be looked up, so it is looked at again in %v: %v", name, Again, err)
		w.showAgain(name)
		return
	case n == nil || n.ended() && n.Status != unsent:
		return
	}
	run, err := w.corbie.Show(ctx, name)
	switch {
	case errors.Is(err, client.ErrNoRun) && n.Status == unsent:
		return
	case errors.Is(err, client.ErrNoRun):
		w.lost(ctx, n)
		return
	case err != nil:
		w.Log("the run %s could not be read, so it is read again in %v: %v", name, Again, err)
		w.showAgain(name)
		return
	case run.Status == "waiting":
		// Corbie has not taken its start up yet: it is looked at again,
		// until it is taken up or Corbie says it never will be.
		w.showAgain(name)
		return
	case !run.HasEnded():
		return
	}
	e := endingOf(run)
	if n.Status == unsent {
		e.how = "its start was taken as not sent, and Corbie ran it all the same: it ended " + cmp.Or(e.how, e.status)
		if err := w.endStray(name, e); err != nil {
			w.Log("how the run %s ended could not be kept: %v", name, err)
		}
		return
	}
	w.take(ctx, n, run, e)
}

func (w *Work) showAgain(name string) { w.again("show:" + name) }

// lost ends a noted run that Corbie never took up, or keeps no record of.
func (w *Work) lost(ctx context.Context, n *note) {
	how := "Corbie never took its start up, or keeps no record of it"
	if isBridge(n.Kind) && !w.handBack(ctx, n, bridges.End{Failed: how}) {
		return
	}
	now := w.clock.Now()
	e := ending{status: lost, how: how, started: n.Noted, ended: now}
	switch n.Kind {
	case KindHeed:
		w.heeded(n, nil, e)
		return
	case KindWork:
	default:
		w.endOther(n, e)
		return
	}
	first, err := w.endWork(n, e, false)
	if err != nil || !first {
		if err != nil {
			w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not keep that the run of %s was lost, and looks again in %v: %v", n.Worker, Again, err))
			w.showAgain(n.Name)
		}
		return
	}
	if n.Stop == stopped {
		return
	}
	if err := w.requests.Failed(n.Request, "its run was lost: "+how); err != nil {
		w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not say that the run of %s was lost: %v", n.Worker, err))
	}
}

// endOther keeps how a run ended that is no lead's or heed's on a
// request: a decider's, a bridge's or a clock's. A decider's that ended is
// told to whoever watches its request; a clock that ended has the clocks
// looked for, so its routine gets another.
func (w *Work) endOther(n *note, e ending) {
	first, err := w.end(n.Name, e)
	if err != nil {
		w.Log("how the run %s ended could not be kept, so it is read again in %v: %v", n.Name, Again, err)
		w.showAgain(n.Name)
		return
	}
	if first && n.Request != 0 {
		w.requests.Tell(n.Request)
	}
	if first && n.Kind == KindClock {
		w.clockSync = true
		if err := w.dropRounds(n.Name); err != nil {
			w.Log("the coming rounds of the clock %s, which ended, could not be let go: %v", n.Name, err)
		}
	}
}

// take takes how a run ended: it is kept with the run, a lead's run on a
// request writes its answer, or what failed, in the request, and a heed's
// what it said.
func (w *Work) take(ctx context.Context, n *note, run *client.Run, e ending) {
	if isBridge(n.Kind) {
		handed, err := w.handedBy(ctx, n)
		if err != nil {
			w.Log("what the %s run %s of the bridge %s handed over could not be read, so it is read again in %v: %v", n.Kind, n.Name, n.Worker, Again, err)
			w.showAgain(n.Name)
			return
		}
		end := bridges.End{Handed: handed, Failed: e.how}
		if e.status == done {
			end = bridges.End{Handed: handed, Output: run.Output}
		}
		if !w.handBack(ctx, n, end) {
			return
		}
	}
	switch n.Kind {
	case KindHeed:
		w.heeded(n, run, e)
		return
	case KindWork:
	default:
		w.endOther(n, e)
		return
	}
	first, err := w.endWork(n, e, false)
	switch {
	case err != nil:
		w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not keep how the run of %s ended, and reads it again in %v: %v", n.Worker, Again, err))
		w.showAgain(n.Name)
		return
	case !first:
		return
	}
	if err := w.said(ctx, n, run, e); err != nil {
		w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not write how the run of %s ended: %v", n.Worker, err))
	}
}

// said writes in a request how its run ended: the answer, by the worker;
// or, for a run that failed or was stopped for the files changing, what
// failed, to the team, and to the asker that it could not answer. A run a
// person stopped has its request answered as stopped already, and one
// that ended with an answer before its cancel reached it has that answer
// kept inside its team.
func (w *Work) said(ctx context.Context, n *note, run *client.Run, e ending) error {
	switch {
	case e.status == done && (n.Stop == "" || n.Stop == stopped):
		return w.answer(ctx, n, answerOf(run))
	case n.Stop == stopped:
		return nil
	case n.Stop != "":
		return w.requests.Failed(n.Request, n.Stop)
	case e.status == cancelled:
		return w.requests.Failed(n.Request, "its run was cancelled in Corbie")
	}
	return w.requests.Failed(n.Request, e.how)
}

// answer says a run's answer: through the team's disclosure check where it
// leaves the team, and otherwise at once. An empty answer from a lead
// started again on a refusal is said as the team not disclosing its
// answer; to a routine or at a channel place, it says nothing and is
// asked of no disclosure, and its request is answered; any other is said
// as the team having given none. Nothing leaves a team whose request
// was stopped: the answer is kept as what was read of it. What a team
// that has gone answers is said nowhere.
func (w *Work) answer(ctx context.Context, n *note, text string) error {
	r, err := w.requests.Get(n.Request)
	if err != nil {
		return err
	}
	team := w.office.Teams[r.Team]
	switch {
	case team == nil:
		return nil
	case r.Stopped:
		_, err := w.requests.Write(r.Number, requests.Writing{Kind: requests.KindBulldog, By: requests.Bulldog,
			To: []requests.Who{w.requests.Team(r.Team)}, Read: true,
			Text: fmt.Sprintf("%s ended once request %d was stopped, so what it answered is not said:\n\n%s", n.Worker, r.Number, text)})
		return err
	}
	empty := strings.TrimSpace(text) == ""
	if empty && n.Again {
		line, refused, err := w.decide.NotDisclosed(r.Number)
		if err != nil {
			return err
		}
		if refused {
			return w.notDisclosed(ctx, r, line)
		}
	}
	by := w.requests.Worker(n.Worker)
	if empty && r.Origin.Quiet() {
		_, err := w.requests.Answer(r.Number, by, "", nil)
		return err
	}
	if empty {
		text = team.Called + " gave no answer."
	}
	leaves, err := w.leaves(team, r, r.Asker)
	if err != nil {
		return err
	}
	if leaves {
		_, err := w.decide.Disclose(ctx, decisions.Said{What: decisions.SaysAnswer, Request: r.Number, By: by, Text: text})
		return err
	}
	_, err = w.requests.Answer(r.Number, by, text, nil)
	return err
}

// endingOf gives how a run ended, as it is kept.
func endingOf(run *client.Run) ending {
	status, how := outcome(run)
	return ending{status: status, how: how, started: run.Started, ended: run.Ended, tokens: tokensOf(run.Output)}
}

// outcome gives how a run ended, as it is kept: its status, and in words
// what failed, with what was refused, by which layer, and the line that
// would fix it.
func outcome(run *client.Run) (string, string) {
	switch run.Status {
	case "done":
		return done, ""
	case "cancelled":
		return cancelled, "it was cancelled"
	}
	how := run.Error
	switch run.Failure {
	case "timeout":
		how = "it ran out of time: " + run.Error
	case "denied":
		how = fmt.Sprintf("it was refused %s by the %s layer: %s", run.Refused, run.Layer, run.Error)
		if run.Fix != "" {
			how += "\nTo let it: " + run.Fix
		}
	}
	return failed, how
}

// answerOf gives a run's answer from its output: an agent's reply, or the
// output of a flow or a statechart. Text is the answer as it is; data
// with a text answer gives that; other data is written out.
func answerOf(run *client.Run) string {
	out := output(run)
	switch v := out.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any:
		if text, ok := v["answer"].(string); ok {
			return text
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Sprint(out)
	}
	return string(b)
}

// output gives what a run ended with: an agent's reply, or the output of
// a flow or a statechart.
func output(run *client.Run) any {
	if run.Type == "agent" {
		m, _ := run.Output.(map[string]any)
		return m["reply"]
	}
	return run.Output
}

// tokensOf gives the tokens an agent's run counted, from its output, also
// for one that failed.
func tokensOf(output any) int {
	m, _ := output.(map[string]any)
	tokens, _ := m["tokens"].(map[string]any)
	in, _ := tokens["input"].(float64)
	out, _ := tokens["output"].(float64)
	return int(in + out)
}

// cancels sends the cancel of each run being stopped, once, when runs
// were noted as stopping. A cancel sent that cannot be kept as sent is
// not sent again while Bulldog runs, and keeping it is tried again.
func (w *Work) cancels(ctx context.Context) bool {
	if !w.cancelling {
		return false
	}
	w.cancelling = false
	notes, err := w.toCancel()
	if err != nil {
		w.Log("the runs to stop could not be read, so they are read again in %v: %v", Again, err)
		w.again("cancels")
		return false
	}
	// A run whose ending is kept is cancelled no more.
	maps.DeleteFunc(w.cancelled, func(name string, _ bool) bool {
		return !slices.ContainsFunc(notes, func(n *note) bool { return n.Name == name })
	})
	did := false
	for _, n := range notes {
		if w.held("cancel:" + n.Name) {
			continue
		}
		did = true
		if !w.cancelled[n.Name] {
			if _, err := w.corbie.Cancel(ctx, n.Name, 0); err != nil {
				w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not cancel the run of %s, and tries again in %v: %v", n.Worker, Again, err))
				w.again("cancel:" + n.Name)
				continue
			}
		}
		if err := w.cancelSent(n.Name); err != nil {
			w.Log("that the run %s was cancelled could not be kept, so it is kept again in %v: %v", n.Name, Again, err)
			w.cancelled[n.Name] = true
			w.again("cancel:" + n.Name)
			continue
		}
		delete(w.cancelled, n.Name)
	}
	return did
}

// trouble says what went wrong in starting or ending a run to the people
// of the team it is for, as what was read of it, once for each thing that
// went wrong, and in the log.
func (w *Work) trouble(request int64, team, text string) {
	w.Log("request %d: %s", request, text)
	if request == 0 || w.troubled[request] == text {
		return
	}
	w.troubled[request] = text
	if _, err := w.requests.Write(request, requests.Writing{Kind: requests.KindBulldog, By: requests.Bulldog,
		To: []requests.Who{w.requests.Team(team)}, Read: true, Text: text}); err != nil {
		w.Log("request %d: what went wrong could not be said to %s: %v", request, team, err)
	}
}
