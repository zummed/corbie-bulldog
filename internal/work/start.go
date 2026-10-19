package work

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// A ready is a request ready for its lead to start on it: what is new to
// the team in it, and its thread as it stands.
type ready struct {
	r      *requests.Request
	worker *office.Worker
	fresh  []requests.Entry
	thread []requests.Entry
	// again says it starts on a decider's refusal of what the team
	// would have said.
	again bool
}

// starts looks at each request that may have a run to start: one
// touched, written to or whose run ended, those waiting their turn once a
// run began to wait or ended, and, after the office is read or Bulldog is
// back, every open one. Those ready start oldest first, while the office
// has room for them; the rest wait their turn.
func (w *Work) starts(ctx context.Context, touched map[int64]bool) bool {
	scan := w.scan && !w.held("scan")
	if len(touched) == 0 && !w.freed && !scan {
		return false
	}
	w.freed = false
	room := w.office.Runs - w.working()
	candidates := maps.Clone(touched)
	if room > 0 {
		maps.Copy(candidates, w.queued)
	}
	if scan {
		if err := w.openRequests(candidates); err != nil {
			w.Log("the open requests could not be read, so they are looked at again in %v: %v", Again, err)
			w.again("scan")
		} else {
			w.scan = false
		}
	}
	var all []ready
	for _, n := range slices.Sorted(maps.Keys(candidates)) {
		delete(w.queued, n)
		if r := w.consider(ctx, n); r != nil {
			all = append(all, *r)
		}
	}
	did := len(candidates) > 0
	for _, r := range all {
		if room <= 0 {
			w.queued[r.r.Number] = true
			continue
		}
		if w.start(ctx, r) {
			room--
		}
	}
	return did
}

// openRequests adds every request open in a team of the office, and every
// one whose run ended and that was not looked at since.
func (w *Work) openRequests(candidates map[int64]bool) error {
	for _, team := range w.office.Order {
		open, err := w.requests.OpenOf(team)
		if err != nil {
			return err
		}
		for _, s := range open {
			candidates[s.Number] = true
		}
	}
	pending, err := w.pendingRequests()
	for _, n := range pending {
		candidates[n] = true
	}
	return err
}

// working counts the runs on requests that are working: going, and not
// waiting on a person, another team or a decision.
func (w *Work) working() int {
	notes, err := w.goingNotes()
	if err != nil {
		w.Log("the runs going could not be counted, so nothing starts until they are, in %v: %v", Again, err)
		w.again("scan")
		w.scan = true
		return w.office.Runs
	}
	count := 0
	for _, n := range notes {
		if n.Kind == KindWork && !w.waiting(n.Request) {
			count++
		}
	}
	return count
}

// consider says whether a request is ready for its lead to start on it:
// it has no run going, of its lead or of its team's heed, its lead is a
// worker, and something new was said to its team. What is new that the
// team's heed weighs is weighed first, and the lead starts only once the
// heed lets it through. A request whose lead cannot run is answered saying
// so, and one at the limit of runs in a row is stopped. A request that was
// stopped has its run noted as stopping, should its stop not have been.
// What cannot be read is looked at again later.
func (w *Work) consider(ctx context.Context, n int64) *ready {
	key := fmt.Sprintf("request:%d", n)
	unread := func(what string, err error) *ready {
		w.Log("request %d: %s could not be read, so it is looked at again in %v: %v", n, what, Again, err)
		w.again(key)
		return nil
	}
	r, err := w.requests.Get(n)
	switch {
	case errors.Is(err, requests.ErrNotThere):
		return nil
	case err != nil:
		return unread("the request", err)
	}
	notes, err := w.notesOf(n, KindWork)
	if err != nil {
		return unread("its runs", err)
	}
	heeds, err := w.notesOf(n, KindHeed)
	if err != nil {
		return unread("its heed's runs", err)
	}
	runs := append(slices.Clone(notes), heeds...)
	if i := slices.IndexFunc(runs, func(n *note) bool { return n.Status == going }); i >= 0 {
		if r.Stopped && runs[i].Stop == "" {
			w.stopRequestRun(r, runs[i])
		}
		// It is looked at again when that run ends.
		return nil
	}
	lead, person, err := w.requests.Lead(n)
	if err != nil {
		return unread("its lead", err)
	}
	thread, err := w.requests.Thread(n)
	if err != nil {
		return unread("its thread", err)
	}
	mark, err := w.mark(n)
	if err != nil {
		return unread("what its last run was given", err)
	}
	again, acted, err := w.isAgain(n)
	if err != nil {
		return unread("whether it starts on a refusal", err)
	}
	given, err := w.givenTo(n)
	if err != nil {
		return unread("what was given to its waiting tools", err)
	}
	last := mark
	if len(thread) > 0 {
		last = max(last, thread[len(thread)-1].ID)
	}
	said := fresh(r, thread, mark, again, given)
	worker := w.office.Workers[lead]
	switch {
	case lead == "" || person:
		// A team that has gone, or a lead who is a person: no run.
		w.dealtWith(n, mark)
		return nil
	case worker == nil || !worker.Runs():
		w.cannotRun(r, lead, said, last)
		return nil
	case len(said) == 0 || r.Stopped:
		w.dealtWith(n, mark)
		return nil
	case w.held(fmt.Sprintf("start:%d", n)):
		// Its start could not be sent: it is looked at again when its time
		// comes.
		return nil
	case inARow(notes, thread) >= RunsInARow:
		w.atLimit(ctx, r, last, again)
		return nil
	}
	rd := ready{r: r, worker: worker, fresh: said, thread: thread, again: again}
	if heed, rule := heedOf(w.office, rd); heed != nil && !acted {
		w.weigh(ctx, rd, heed, rule)
		return nil
	}
	return &rd
}

// stopRequestRun notes as stopping the run of a request that was stopped,
// whose stop could not be noted when it was asked.
func (w *Work) stopRequestRun(r *requests.Request, n *note) {
	if err := w.stopRun(n.Name, stopped); err != nil {
		w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog could not note that the run of %s is to stop, so it is not cancelled yet, and tries again in %v: %v",
			n.Worker, Again, err))
		w.again(fmt.Sprintf("stop:%d", r.Number))
		return
	}
	w.cancelling = true
}

// dealtWith keeps a request's thread as dealt with up to an entry.
func (w *Work) dealtWith(n, seen int64) {
	if err := w.dealt(n, seen); err != nil {
		w.Log("request %d could not be kept as looked at: %v", n, err)
	}
}

// cannotRun answers a request whose lead cannot run: the asker is told
// the team cannot answer, and the team's people are told the problem.
// One open with nothing new is answered once for each problem.
func (w *Work) cannotRun(r *requests.Request, lead string, fresh []requests.Entry, last int64) {
	why := problem(w.office, lead)
	if len(fresh) == 0 {
		told, err := w.told(r.Number)
		if err != nil {
			w.Log("request %d: %v", r.Number, err)
			return
		}
		st, err := w.requests.State(r.Number)
		if err != nil {
			w.Log("request %d: %v", r.Number, err)
			return
		}
		if told == why || st.Is == requests.Answered {
			w.dealtWith(r.Number, last)
			return
		}
	}
	if err := w.requests.Failed(r.Number, why); err != nil {
		w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog could not say that %s cannot answer: %v", lead, err))
		return
	}
	if err := w.keepTold(r.Number, why); err != nil {
		w.Log("request %d: %v", r.Number, err)
	}
	w.dealtWith(r.Number, last)
}

// atLimit stops a request given RunsInARow runs in a row with nothing
// said in it by a person: the team's people are told why, and the request
// is answered as stopped. Where the last run was started by a refused
// answer, the asker is told only the one line the decisions give, in
// place of who stopped it: that the team did not disclose its answer, and
// who refused it.
func (w *Work) atLimit(ctx context.Context, r *requests.Request, last int64, refused bool) {
	w.dealtWith(r.Number, last)
	why := fmt.Sprintf("Request %d was given %d runs in a row with nothing said in it by a person, so it is stopped. Say more in it to start it again.",
		r.Number, RunsInARow)
	if _, err := w.requests.Write(r.Number, requests.Writing{Kind: requests.KindBulldog, By: requests.Bulldog,
		To: []requests.Who{w.requests.Team(r.Team)}, Read: true, Text: why}); err != nil {
		w.Log("request %d: %v", r.Number, err)
	}
	var line string
	if refused {
		var err error
		if line, refused, err = w.decide.NotDisclosed(r.Number); err != nil {
			w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog could not read who refused request %d's answer, so it is stopped by Bulldog: %v", r.Number, err))
		}
	}
	var err error
	if refused {
		err = w.notDisclosed(ctx, r, line)
	} else {
		var numbers []int64
		if numbers, err = w.requests.Stop(r.Number, requests.Bulldog); err == nil {
			err = w.stopped(ctx, numbers)
		}
	}
	if err != nil {
		w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog could not stop request %d at the limit of runs in a row: %v", r.Number, err))
	}
}

// notDisclosed answers a request whose team did not disclose its answer: it
// is stopped, and its asker is told only the one line the decisions give,
// which stands in place of the team having given no answer.
func (w *Work) notDisclosed(ctx context.Context, r *requests.Request, line string) error {
	numbers, err := w.requests.StopWith(r.Number, line)
	if err != nil {
		return err
	}
	return w.stopped(ctx, numbers)
}

// fresh gives what was said to a request's team since its last run, or
// since it was last dealt with: what was asked, what was said more, what
// people answered, answers that came late, and, when what it would have
// said was refused, the decider's word. What was given to a waiting tool,
// and a message the team's heed noted, are not among it.
func fresh(r *requests.Request, thread []requests.Entry, mark int64, again bool, given map[int64]bool) []requests.Entry {
	var new []requests.Entry
	for _, e := range thread {
		if e.ID <= mark || given[e.ID] || e.Noted {
			continue
		}
		switch e.Kind {
		case requests.KindAsked, requests.KindSaid, requests.KindReplied, requests.KindLate:
		case decisions.KindDecided:
			if !again {
				continue
			}
		default:
			continue
		}
		if slices.ContainsFunc(e.To, func(to requests.Who) bool {
			return to.Kind == requests.IsTeam && to.Name == r.Team || to.Kind == requests.IsWorker && to.Team == r.Team
		}) {
			new = append(new, e)
		}
	}
	return new
}

// inARow counts the runs of a request since a person last said something
// in it.
func inARow(notes []*note, thread []requests.Entry) int {
	var since int64
	for _, e := range thread {
		if byPerson(e) {
			since = e.ID
		}
	}
	count := 0
	for _, n := range notes {
		if n.Status != unsent && n.Seen >= since {
			count++
		}
	}
	return count
}

// byPerson says whether an entry is something a person said in a
// request: what they asked, said more or answered. A decider's word is
// not.
func byPerson(e requests.Entry) bool {
	switch e.Kind {
	case requests.KindAsked, requests.KindSaid, requests.KindReplied:
		return e.By.Kind == requests.IsPerson || e.By.Kind == requests.IsIdentity
	}
	return false
}

// start starts a request's lead on it. The run is noted, with its team
// and request, before its start is sent; a start that cannot be sent is
// said to the team's people and tried again later, as what it was, and
// holds up no other.
func (w *Work) start(ctx context.Context, rd ready) bool {
	r := rd.r
	fail := func(what string, err error) bool {
		w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog could not start %s on request %d (%s), and tries again in %v: %v",
			rd.worker.Name, r.Number, what, Again, err))
		w.again(fmt.Sprintf("start:%d", r.Number))
		keep := w.pend
		if rd.again {
			keep = w.refused
		}
		if err := keep(r.Number); err != nil {
			w.Log("request %d: %v", r.Number, err)
		}
		return false
	}
	input, err := w.given(w.office, rd)
	if err != nil {
		return fail("what it is given could not be put together", err)
	}
	name, err := w.corbie.NewRun()
	if err != nil {
		return fail("no name could be chosen for its run", err)
	}
	labels := w.labels(requests.Labels(r))
	labels[LabelKind] = KindWork
	n := &note{Name: name, Kind: KindWork, Team: r.Team, Request: r.Number, Worker: rd.worker.Name, Labels: labels,
		Seen: rd.thread[len(rd.thread)-1].ID, Noted: w.clock.Now(), Again: rd.again}
	if err := w.keep(n); err != nil {
		return fail("its run could not be noted", err)
	}
	if _, err := w.corbie.Start(ctx, rd.worker.Draft, client.Start{Run: name, Input: input, Labels: labels}); err != nil {
		w.endUnsent(name, err)
		return fail("Corbie was not reached", err)
	}
	if err := w.sent(n); err != nil {
		w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog started %s on request %d, and could not keep that it did: %v", rd.worker.Name, r.Number, err))
	}
	delete(w.troubled, r.Number)
	return true
}

// tooDeep refuses a sub-request of a request that is already Depth levels
// below the request at the top of its tree, saying so in a sentence:
// teams that ask each other in a ring would otherwise ask for ever.
func (w *Work) tooDeep(n int64) error {
	_, depth, err := w.requests.Top(n)
	if err != nil {
		return err
	}
	if depth >= Depth {
		return requests.Refusal(fmt.Sprintf("Request %d is already %d levels of sub-requests below the request that began it, the most there may be, so no team is asked from it.",
			n, depth))
	}
	return nil
}
