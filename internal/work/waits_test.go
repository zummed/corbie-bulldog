package work

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// week is how long a shipped tool waits.
const week = 7 * 24 * time.Hour

func TestAShippedToolWaitsAWeek(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("ask_person", map[string]any{"question": "tell me when the socket arrives"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	q := b.waitsOn(n, requests.KindQuestion)
	b.later(week)
	// The question is still open, so the request waits on Kit.
	b.settled(n, requests.Waiting)
	holds(t, "the steward's answer", b.waitsOn(n, requests.KindAnswer).Text, "fixer heard: No answer came within 7 days", "still open")
	b.later(24 * time.Hour)
	reply, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "it came on day 8", nil, requests.Screens)
	if err != nil {
		t.Fatal(err)
	}
	run := b.started(n, 2)
	if said := fmt.Sprint(b.input(run.Name)["said"]); said != reply.Text {
		t.Errorf("the steward's second run is given %q", said)
	}
}

func TestAQuestionStaysOpenWhenItsRunEnds(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "refund 4411?") && turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"question": "may I refund it?", "who": "Lee"})
		}
		return testkit.Say("done")
	})
	n := b.chat("al", "", "customer 4411 was charged twice")
	b.answered(n)
	sub, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "payments", "refund 4411?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	q := b.waitsOn(sub.Number, requests.KindQuestion)
	b.later(week)
	b.until("the ledger's run ends", func() bool { runs := b.runs(sub.Number); return len(runs) == 1 && runs[0].Status == done })
	reply, err := b.reqs.Reply(q.ID, b.reqs.Person("lee"), "yes, refund it", nil, requests.Screens)
	if err != nil {
		t.Fatal(err)
	}
	if reply.To[0].Name != "ledger" {
		t.Errorf("Lee's answer is to %v", reply.To)
	}
	run := b.started(sub.Number, 2)
	if said := fmt.Sprint(b.input(run.Name)["said"]); said != "yes, refund it" {
		t.Errorf("the ledger's second run is given %q", said)
	}
}

func TestWorkKeptAnswerRemovedWhenCallEnds(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("tell", map[string]any{"text": "the power is off", "who": "Sam"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.answered(n)
	calls := b.calls(n)
	if len(calls) != 1 {
		t.Fatalf("the calls are %v", calls)
	}
	b.until("the call's kept answer goes", func() bool {
		row, err := b.w.callOf(calls[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		return row == nil
	})
}

func TestWorkCallSeenAgainGetsKeptAnswer(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.reach.fail("reply", errors.New("Corbie is out of reach"))
	b.says(fixing("tell", map[string]any{"text": "the power is off", "who": "Sam"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.waitsOn(n, requests.KindTold)
	b.until("the answer is kept", func() bool { return b.logged("could not be given") })
	b.halt()
	b.reach.fail("reply", nil)
	b.db.Close()
	b.open()
	holds(t, "the steward's answer", b.answered(n).Text, "fixer heard: Told Sam")
	if told := b.entries(n, requests.KindTold); len(told) != 1 {
		t.Errorf("the tell was done again: %v", b.thread(n))
	}
}

func TestWorkCallAndItsAnswerWrittenTogether(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.reach.fail("reply", errors.New("Corbie is out of reach"))
	b.says(fixing("tell", map[string]any{"text": "the power is off", "who": "Sam"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.waitsOn(n, requests.KindTold)
	b.until("the answer is kept", func() bool { return b.logged("could not be given") })
	b.halt()
	// As if Bulldog stopped once the tell was put and before its answer
	// was kept.
	if _, err := b.db.Exec(`UPDATE work_calls SET state = ?, answer = '' WHERE request = ?`, doing, n); err != nil {
		t.Fatal(err)
	}
	b.db.Close()
	b.reach.fail("reply", nil)
	b.open()
	holds(t, "the steward's answer", b.answered(n).Text, "fixer heard: Told Sam")
	if told := b.entries(n, requests.KindTold); len(told) != 1 {
		t.Errorf("the tell was done twice: %v", b.thread(n))
	}
}

func TestWorkCallFromUnknownRunRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, caller(house(t), "tell", "{ text: hello, who: Sam }"))
	n := b.chat("kit", "desk", "file the bill")
	b.answered(n)
	// A run with the office's label that Bulldog never noted, which it
	// cannot cancel.
	b.reach.fail("cancel", errors.New("Corbie is out of reach"))
	run, err := b.corbie.Start(ctx, b.office().Workers["filer"].Draft, client.Start{Input: map[string]any{"said": "x"},
		Labels: map[string]string{LabelBulldog: identity, LabelKind: KindWork, requests.LabelTeam: "desk", requests.LabelRequest: fmt.Sprint(n)}})
	if err != nil {
		t.Fatal(err)
	}
	ended := b.corbie.Ended(t, run)
	holds(t, "the unknown run's call", fmt.Sprint(ended.Output), "did not start for this office", "not done")
	if told := b.entries(n, requests.KindTold); len(told) != 1 {
		t.Errorf("the unknown run's call was done: %v", b.thread(n))
	}
}

func TestWorkUnreadableCallLookedAtAgain(t *testing.T) {
	t.Parallel()
	b := newBench(t, caller(house(t), "tell", "{ text: hello, who: Sam }"))
	b.reach.fail("show call", errors.New("the record is out of reach"))
	n := b.chat("kit", "desk", "file the bill")
	b.until("the call could not be read", func() bool { return b.logged("could not be done yet") })
	if told := b.entries(n, requests.KindTold); len(told) != 0 {
		t.Fatalf("the call was done unread: %v", b.thread(n))
	}
	b.reach.fail("show call", nil)
	b.later(Again)
	holds(t, "the desk's answer", b.answered(n).Text, "Told Sam")
}

// deep is a desk whose taker makes its call from below so many machines
// written in place.
func deep(files map[string]string, depth int) map[string]string {
	job := callChart("tell", "{ text: hello, who: Sam }")
	for i := depth; i >= 1; i-- {
		job = fmt.Sprintf("machine:\n  flow: x%d\n  stages:\n    a:\n      job:\n%s  output: ${stages.a.output}\n", i, indent(job, 8))
	}
	files = caller(files, "tell", "{}")
	files["desk/filer.yaml"] = "flow: filer\ninput: [said]\nstages:\n  a:\n    job:\n" + indent(job, 6) + "output: ${stages.a.output}\n"
	return files
}

func TestWorkCallerFoundWithinSixteenRuns(t *testing.T) {
	t.Parallel()
	// Corbie lets a root have eight levels of machines below it: a call
	// at the deepest finds the worker at the top.
	b := newBench(t, deep(house(t), 7))
	n := b.chat("kit", "desk", "file the bill")
	holds(t, "the call eight runs down", b.answered(n).Text, "Told Sam")

	// A run above it that is gone refuses it.
	w := &Work{corbie: chain{Corbie: b.reach, runs: map[string]*client.Run{"0": {ID: "0", Machine: "x0", Parent: "gone"}}}}
	if _, _, refusal, err := w.above(ctx, &client.Run{ID: "call", Parent: "0"}); err != nil || !strings.Contains(refusal, "is gone") {
		t.Errorf("a call below a run that is gone gave %q %v", refusal, err)
	}

	// Past sixteen runs up, the call is refused, whatever is above.
	for _, depth := range []int{Climb, Climb + 1} {
		runs := map[string]*client.Run{}
		for i := range depth {
			runs[fmt.Sprint(i)] = &client.Run{ID: fmt.Sprint(i), Machine: fmt.Sprintf("x%d", i), Parent: fmt.Sprint(i + 1)}
		}
		runs[fmt.Sprint(depth-1)].Parent = ""
		w := &Work{corbie: chain{Corbie: b.reach, runs: runs}}
		machines, top, refusal, err := w.above(ctx, &client.Run{ID: "call", Parent: "0"})
		switch {
		case err != nil:
			t.Fatal(err)
		case depth == Climb && (refusal != "" || len(machines) != Climb || top != fmt.Sprint(Climb-1)):
			t.Errorf("sixteen runs up gave %v %q %q", machines, top, refusal)
		case depth > Climb:
			holds(t, "seventeen runs up", refusal, "at most 16 runs up", "not done")
		}
	}
}

// A chain is a Corbie whose runs above a call are made up.
type chain struct {
	Corbie
	runs map[string]*client.Run
}

func (c chain) Show(_ context.Context, id string) (*client.Run, error) {
	if r := c.runs[id]; r != nil {
		return r, nil
	}
	return nil, client.ErrNoRun
}

func TestAWaitingCallWaitsOnAcrossARestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("ask_person", map[string]any{"question": "tell me when the socket arrives"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	q := b.waitsOn(n, requests.KindQuestion)
	b.restart()
	if _, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "it came today", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	if a := b.answered(n); a.Text != "steward heard: fixer heard: Kit answered: it came today" {
		t.Errorf("the steward answered %q", a.Text)
	}
	if runs := b.runs(n); len(runs) != 1 {
		t.Errorf("the request had the runs %+v", runs)
	}

	w := newBench(t, work(t))
	w.says(func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_team", map[string]any{"team": "support", "question": "has 4411 written in?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	r := w.chat("al", "", "customer 4411 was charged twice")
	sub := w.waitsOn(r, requests.KindSub).Sub
	w.halt()
	// Dana answers while Bulldog is away.
	reqs, err := requests.Open(w.db, w.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := reqs.Read(w.office()); err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Say(sub, reqs.Person("dana"), "Yes, on Tuesday.", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	w.db.Close()
	w.open()
	holds(t, "the bot's answer", w.answered(r).Text, "Dana answered for Support", "Yes, on Tuesday.")
	if late := w.entries(r, requests.KindLate); len(late) != 0 {
		t.Errorf("Dana's answer went into the thread as well: %v", w.thread(r))
	}
}
