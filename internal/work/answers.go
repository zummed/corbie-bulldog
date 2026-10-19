package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/shipped"
)

// What a waiting tool waits for comes from elsewhere: a person's answer
// to a question, a sub-request's answer, a disclosure let go or refused, a
// change decided. Each goes to the tool while it waits, and otherwise
// into the request's thread, never both.

// answers gives the tools waiting in the requests that changed what came
// for them.
func (w *Work) answers(ctx context.Context, touched map[int64]bool) bool {
	did := false
	for _, n := range slices.Sorted(maps.Keys(touched)) {
		err := w.replies(ctx, n)
		err = errors.Join(err, w.deliver(ctx, n))
		if err != nil {
			w.Log("request %d: what came for a waiting tool could not be given, so it is looked at again in %v: %v", n, Again, err)
			w.again(fmt.Sprintf("request:%d", n))
		}
		did = true
	}
	return did
}

// settleCall keeps a call's answer, with the entry given to it, and gives
// it to the tool, and says whether the tool had stopped waiting, so the
// answer is for somewhere else. An answer that cannot be given now is
// given again later.
func (w *Work) settleCall(ctx context.Context, row *callRow, answer string, given *requests.Entry) (bool, error) {
	if err := w.answerCall(row, answer, given); err != nil {
		return false, err
	}
	switch err := w.give(ctx, row.Run, shipped.CallTicket, answer); {
	case errors.Is(err, errGone):
		return true, nil
	case err != nil:
		w.giveLater(row.Run, err)
	}
	return false, nil
}

// replies gives a person's answer to each question in a request whose
// tool waits on it, and keeps it as given, so it starts no run.
func (w *Work) replies(ctx context.Context, n int64) error {
	calls, err := w.callsOf(n)
	if err != nil {
		return err
	}
	calls = slices.DeleteFunc(calls, func(c *callRow) bool { return c.Tool != toolAskPerson || c.State != waiting || c.Entry == 0 })
	if len(calls) == 0 {
		return nil
	}
	thread, err := w.requests.Thread(n)
	if err != nil {
		return err
	}
	for _, c := range calls {
		i := slices.IndexFunc(thread, func(e requests.Entry) bool { return e.Answers == c.Entry })
		if i < 0 {
			continue
		}
		e := thread[i]
		text := e.Text
		if e.By.Kind != requests.IsBulldog {
			text = fmt.Sprintf("%s answered: %s", called(w.office, e.By), e.Text)
		}
		gone, err := w.settleCall(ctx, c, text, &e)
		if err != nil {
			return err
		}
		if gone {
			// The tool stopped waiting: the answer is for the next run.
			if err := w.dropCall(c.Run, e.ID); err != nil {
				return err
			}
			w.touch(n)
		}
	}
	return nil
}

// deliver gives a sub-request's answer to the tool that waits on it, or,
// once that has stopped waiting, to the request that asked it, as a late
// answer: each answer once, to one or the other. The tool of a request
// that was stopped is given nothing.
func (w *Work) deliver(ctx context.Context, n int64) error {
	r, err := w.requests.Get(n)
	switch {
	case errors.Is(err, requests.ErrNotThere):
		return nil
	case err != nil:
		return err
	case r.Parent == 0:
		return nil
	}
	thread, err := w.requests.Thread(n)
	if err != nil {
		return err
	}
	given, err := w.subGiven(n)
	if err != nil {
		return err
	}
	answer := lastAnswer(r, thread)
	if answer == nil || answer.ID <= given {
		return nil
	}
	row, err := w.callOf(r.Ticket)
	if err != nil {
		return err
	}
	if row != nil && row.Tool == toolAskTeam && row.State == waiting {
		// The runs of a request that was stopped are being cancelled, and
		// a tool of theirs is given nothing, so that none of them goes on.
		above, err := w.requests.Get(r.Parent)
		if err != nil {
			return err
		}
		if !above.Stopped {
			gone, err := w.settleCall(ctx, row, w.subText(r, *answer), answer)
			if err != nil {
				return err
			}
			if !gone {
				return w.subDone(n, answer.ID, !r.Answered)
			}
		}
		if err := w.dropCall(row.Run, 0); err != nil {
			return err
		}
	}
	return w.late(r, answer.ID)
}

// late hands a sub-request's answer that no tool waits for to the
// request that asked it, which starts its lead again; a request that
// was stopped is given nothing.
func (w *Work) late(r *requests.Request, answer int64) error {
	above, err := w.requests.Get(r.Parent)
	if err != nil {
		return err
	}
	if !above.Stopped {
		if _, err := w.requests.Late(r.Number); err != nil {
			return err
		}
	}
	return w.subDone(r.Number, answer, !r.Answered)
}

// lastAnswer gives a request's latest answer to its asker: its lead's, or
// Bulldog's word that it failed or was stopped.
func lastAnswer(r *requests.Request, thread []requests.Entry) *requests.Entry {
	for i, e := range slices.Backward(thread) {
		if e.Read != "" {
			continue
		}
		if e.Kind == requests.KindAnswer || e.Kind == requests.KindBulldog && slices.ContainsFunc(e.To, func(to requests.Who) bool { return sameWho(to, r.Asker) }) {
			return &thread[i]
		}
	}
	return nil
}

// subText gives a sub-request's answer as the tool that asked it is
// given it: the answer, and who gave it.
func (w *Work) subText(r *requests.Request, answer requests.Entry) string {
	if answer.By.Kind == requests.IsBulldog {
		return answer.Text
	}
	return fmt.Sprintf("%s answered for %s:\n\n%s", called(w.office, answer.By), calledTeam(w.office, r.Team), answer.Text)
}

// let is told how a disclosure ended, and keeps it, to be done on work's own
// goroutine; told twice, it is done once.
func (w *Work) let(ctx context.Context, l decisions.Let) error {
	if err := w.keepOwed("work_lets", l.Decision, l); err != nil {
		return err
	}
	w.owesNoted()
	return nil
}

// lets does what each disclosure that ended asks of work.
func (w *Work) lets(ctx context.Context) bool {
	owed, err := w.owed("work_lets")
	if err != nil {
		w.Log("the disclosures that ended could not be read, so they are read again in %v: %v", Again, err)
		w.again("owed")
		return false
	}
	did := false
	for _, id := range slices.Sorted(maps.Keys(owed)) {
		key := fmt.Sprintf("let:%d", id)
		if w.held(key) {
			continue
		}
		did = true
		var l decisions.Let
		err := json.Unmarshal([]byte(owed[id]), &l)
		if err == nil {
			err = w.letGo(ctx, l)
		}
		if err == nil {
			err = w.paid("work_lets", id)
		}
		if err != nil {
			w.trouble(l.Said.Request, w.teamOf(l.Said.Request), fmt.Sprintf("Bulldog could not do what the disclosure %d decided, and tries again in %v: %v", id, Again, err))
			w.again(key)
		}
	}
	return did
}

// letGo does what a disclosure decided. An answer let go was said by the
// decisions, and goes on to the tool that asked for it; one refused
// starts the lead again with the decider's word. A question, a tell or a
// sub-request let go is put or made now, and one refused is the waiting
// tool's result, or, once it no longer waits, starts the lead again.
func (w *Work) letGo(ctx context.Context, l decisions.Let) error {
	n := l.Said.Request
	if l.Said.What == decisions.SaysAnswer {
		if !l.Disclosed {
			if err := w.refused(n); err != nil {
				return err
			}
		}
		w.touch(n)
		return nil
	}
	row, err := w.callOf(l.Said.Ticket)
	switch {
	case err != nil:
		return err
	case row != nil && row.State != held:
		return nil
	case row == nil:
		// The tool stopped waiting: what was let go is still said, and a
		// refusal starts the lead again.
		row = &callRow{Run: l.Said.Ticket, Request: n, Worker: l.Said.By.Name}
	}
	if !l.Disclosed {
		if row.State == held {
			gone, err := w.settleCall(ctx, row, l.Text(), nil)
			if err != nil || !gone {
				return err
			}
		}
		if err := w.refused(n); err != nil {
			return err
		}
		w.touch(n)
		return nil
	}
	answer := ""
	switch l.Said.What {
	case decisions.SaysTell:
		_, err = w.putOnce(row, requests.KindTold, l.Said.To, l.Said.Text)
		answer = toldText(called(w.office, l.Said.To))
	case decisions.SaysQuestion:
		var e *requests.Entry
		if e, err = w.putOnce(row, requests.KindQuestion, l.Said.To, l.Said.Text); err == nil && row.State == held {
			row.State, row.Entry = waiting, e.ID
			err = w.keepCall(row)
			w.touch(n)
		}
	case decisions.SaysSub:
		if row.State == held {
			err = w.sub(row, n, l.Said.By, l.Said.Team, l.Said.Text)
		} else if sub, e := w.requests.Sub(n, l.Said.By, l.Said.Team, l.Said.Text, l.Said.Ticket); e != nil {
			err = e
		} else {
			err = w.keepSub(sub.Number)
		}
	default:
		return fmt.Errorf("nothing in work says what %s would %s", l.Said.By.Name, l.Said.What)
	}
	var refusal requests.Refusal
	if errors.As(err, &refusal) {
		answer, err = sentence(string(refusal)), nil
	}
	if err != nil || row.State != held || answer == "" {
		return err
	}
	if err := w.answerCall(row, answer, nil); err != nil {
		return err
	}
	w.giveSoon(ctx, row)
	return nil
}

// teamOf gives the team a request went to, "" when it cannot be read.
func (w *Work) teamOf(n int64) string {
	r, err := w.requests.Get(n)
	if err != nil {
		return ""
	}
	return r.Team
}

// Told is told how a change put ended, as Changes tells it, beside the
// decisions; it is kept, to be done on work's own goroutine.
func (w *Work) Told(out changes.Outcome) {
	w.decide.Told(out)
	if err := w.keepOwed("work_outcomes", out.Put, out); err != nil {
		w.Log("how the change %d ended could not be kept, so the builder that put it is not told: %v", out.Put, err)
		return
	}
	w.owesNoted()
}

// outcomes does what each change that ended asks of work: the runs of
// the workers it took away are stopped, and the builder that put it is
// told, by its waiting tool, or, once that no longer waits, in its
// request, which starts it again.
func (w *Work) outcomes(ctx context.Context) bool {
	owed, err := w.owed("work_outcomes")
	if err != nil {
		w.Log("the changes that ended could not be read, so they are read again in %v: %v", Again, err)
		w.again("owed")
		return false
	}
	did := false
	for _, id := range slices.Sorted(maps.Keys(owed)) {
		key := fmt.Sprintf("outcome:%d", id)
		if w.held(key) {
			continue
		}
		did = true
		var out changes.Outcome
		err := json.Unmarshal([]byte(owed[id]), &out)
		if err == nil {
			err = w.outcome(ctx, out)
		}
		if err == nil {
			err = w.paid("work_outcomes", id)
		}
		if err != nil {
			w.trouble(out.Request, w.teamOf(out.Request), fmt.Sprintf("Bulldog could not tell %s how the change %d ended, and tries again in %v: %v",
				out.By, id, Again, err))
			w.again(key)
		}
	}
	return did
}

func (w *Work) outcome(ctx context.Context, out changes.Outcome) error {
	if err := w.stopWorkers(out.Gone); err != nil {
		return err
	}
	if out.Request == 0 {
		return nil
	}
	row, err := w.callOfPut(out.Put)
	if err != nil {
		return err
	}
	if row != nil {
		if row.State != waiting {
			return nil
		}
		gone, err := w.settleCall(ctx, row, out.Text(), nil)
		if err != nil || !gone {
			return err
		}
		if err := w.dropCall(row.Run, 0); err != nil {
			return err
		}
	}
	_, err = w.requests.Write(out.Request, requests.Writing{Kind: requests.KindLate, By: requests.Bulldog, To: []requests.Who{w.requests.Worker(out.By)},
		Read: true, Working: true, Text: out.Text()})
	return err
}

// stopWorkers stops the runs on requests of workers a change took out of
// the office.
func (w *Work) stopWorkers(workers []string) error {
	if len(workers) == 0 {
		return nil
	}
	notes, err := w.goingNotes()
	if err != nil {
		return err
	}
	for _, n := range notes {
		if n.Kind == KindWork && n.Stop == "" && slices.Contains(workers, n.Worker) {
			if err := w.stopRun(n.Name, fmt.Sprintf("%s was taken out of the office by a change, so its run was stopped", n.Worker)); err != nil {
				return err
			}
		}
	}
	w.cancelling = true
	return nil
}

// callEnded lets a call's kept answer go once its run has ended. An ask
// that ended is told to the decisions, which withdraw what it held if it
// was never answered. An answer kept that never reached its tool, which
// ran out of time first, goes where a late answer goes.
func (w *Work) callEnded(ctx context.Context, r *client.Run) error {
	row, err := w.callOf(r.ID)
	if err != nil || row == nil {
		return err
	}
	if row.Tool == askTool {
		if err := w.decide.AskEnded(ctx, r.ID); err != nil {
			return err
		}
		return w.dropCall(r.ID, 0)
	}
	untaken := int64(0)
	if row.State == answered && r.Status == "done" {
		// What a run ended with is read from its record.
		if r, err = w.corbie.Show(ctx, r.ID); err != nil {
			return err
		}
	}
	if row.State == answered && r.Status == "done" && str(r.Output) != row.Answer {
		switch row.Tool {
		case toolAskPerson:
			given, err := w.givenTo(row.Request)
			if err != nil {
				return err
			}
			thread, err := w.requests.Thread(row.Request)
			if err != nil {
				return err
			}
			for _, e := range thread {
				if e.Answers == row.Entry && given[e.ID] {
					untaken = e.ID
				}
			}
			w.touch(row.Request)
		case toolAskTeam:
			sub, err := w.requests.Get(row.Sub)
			if err != nil {
				return err
			}
			given, err := w.subGiven(row.Sub)
			if err != nil {
				return err
			}
			if err := w.late(sub, given); err != nil {
				return err
			}
		}
	}
	return w.dropCall(r.ID, untaken)
}
