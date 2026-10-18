package work

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/shipped"
)

// ask hands the jobs an ask's run holds to the decisions, which answer
// its ticket through answerAsk once each is decided, and gives the request
// of the run Bulldog noted at its top. An ask handed in again, as after a
// restart, is given the answer it was given before. An ask not below a
// run Bulldog noted for this office approves nothing, and is of no
// request.
func (w *Work) ask(ctx context.Context, r *client.Run) (int64, error) {
	in, err := w.inputOf(ctx, r.ID)
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return 0, err
	}
	asking, jobs, err := decisions.JobsOf(raw)
	if err != nil {
		return 0, w.approveNothing(ctx, r.ID, "Bulldog could not read what was held, so nothing was approved: "+err.Error())
	}
	machines, top, refusal, err := w.above(ctx, r)
	if err != nil {
		return 0, err
	}
	if refusal != "" {
		return 0, w.approveNothing(ctx, r.ID, refusal)
	}
	n, err := w.noted(top)
	switch {
	case err != nil:
		return 0, err
	case n == nil:
		return 0, w.approveNothing(ctx, r.ID, "This ask comes from a run Bulldog did not start for this office, so nothing was approved.")
	case n.Kind != KindWork && n.Kind != decisions.Deciding || n.ended():
		return 0, w.approveNothing(ctx, r.ID, "This ask comes from no run of a worker Bulldog is running, so nothing was approved.")
	}
	if err := w.keepCall(&callRow{Run: r.ID, Request: n.Request, Tool: askTool, State: waiting}); err != nil {
		return 0, err
	}
	if asking != "" && (len(machines) == 0 || machines[0] != asking) {
		machines = slices.Insert(machines, 0, asking)
	}
	return n.Request, w.decide.Held(ctx, decisions.Ask{Run: r.ID, Team: n.Team, Request: n.Request, Asking: machines, Jobs: jobs})
}

// approveNothing answers an ask that Bulldog will not hand on, with why.
func (w *Work) approveNothing(ctx context.Context, run, why string) error {
	if w.office != nil {
		w.Log("the ask %s: %s", run, why)
	}
	err := w.give(ctx, run, shipped.AskTicket, decisions.Verdict{Approve: []string{}, Note: why})
	if errors.Is(err, errGone) {
		return nil
	}
	return err
}

// answerAsk keeps what was decided of every job an ask holds, for the
// decisions, on whichever goroutine decided, and has the driving one
// answer the ask's ticket with it: a person deciding waits on nothing in
// Corbie.
func (w *Work) answerAsk(_ context.Context, run string, v decisions.Verdict) error {
	if v.Approve == nil {
		v.Approve = []string{}
	}
	if err := w.keepVerdict(run, v); err != nil {
		return err
	}
	w.owesNoted()
	return nil
}

// verdicts answers each ask's ticket with the verdict kept for it. An ask
// that has ended has nothing to answer; one Corbie cannot be reached for
// is answered again once Again has passed.
func (w *Work) verdicts(ctx context.Context) bool {
	owed, err := w.verdictsOwed()
	if err != nil {
		w.Log("the asks decided could not be read, so they are read again in %v: %v", Again, err)
		w.again("owed")
		return false
	}
	did := false
	for _, run := range slices.Sorted(maps.Keys(owed)) {
		key := "verdict:" + run
		if w.held(key) {
			continue
		}
		did = true
		err := w.give(ctx, run, shipped.AskTicket, owed[run])
		if errors.Is(err, errGone) {
			err = nil
		}
		if err == nil {
			err = w.verdictGiven(run)
		}
		if err != nil {
			w.Log("the ask %s is decided, but Corbie could not be answered, so it is answered again in %v: %v", run, Again, err)
			w.again(key)
		}
	}
	return did
}
