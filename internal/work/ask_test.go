package work

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// held waits until an action held in a request waits on people, and
// gives it.
func (b *bench) held(n int64) *decisions.Decision {
	b.t.Helper()
	var dec *decisions.Decision
	b.until(fmt.Sprintf("an action waits on people in request %d", n), func() bool {
		for _, d := range must(b.d.OfRequest(n)) {
			if d.Kind == decisions.OfAction && d.Ended == "" && len(d.Asked) > 0 {
				dec = d
			}
		}
		return dec != nil
	})
	return dec
}

// machine gives the run of a machine below a request's runs.
func (b *bench) machine(n int64, name string) *client.Run {
	b.t.Helper()
	runs, err := b.corbie.List(ctx, client.Filter{Labels: map[string]string{LabelBulldog: identity, requests.LabelRequest: fmt.Sprint(n)}, Finished: true})
	if err != nil {
		b.t.Fatal(err)
	}
	i := slices.IndexFunc(runs, func(r *client.Run) bool { return r.Machine == name })
	if i < 0 {
		b.t.Fatalf("request %d has no run of %s", n, name)
	}
	return runs[i]
}

func TestAHeldJobIsAskedOfItsTeam(t *testing.T) {
	t.Parallel()
	for _, own := range []bool{false, true} {
		files := house(t)
		if own {
			files["fixer.yaml"] += "ask: { machine: ./.bulldog/ask.yaml }\n"
		}
		b := newBench(t, files)
		b.says(fixing("cut_power", map[string]any{"circuit": "4"}))
		n := b.chat("kit", "", "the kitchen socket sparked")
		dec := b.held(n)
		if dec.Whose.Name != "fixer" || dec.Team != "office" || dec.Job == nil || dec.Job.Name != "cut_power" || strings.Join(dec.Asked, ",") != "sam" {
			t.Errorf("the held job is decided as %+v", dec)
		}
		ask, err := b.corbie.Show(ctx, dec.Ask)
		if err != nil {
			t.Fatal(err)
		}
		// Corbie starts the ask, its own or the steward's, as a job of the
		// machine whose job it holds.
		if fixer := b.machine(n, "fixer"); ask.Machine != askMachine || ask.Parent != fixer.ID {
			t.Errorf("with the fixer naming the ask %v, the ask runs as %+v", own, ask)
		}
		if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "yes", ""); err != nil || !r.Taken {
			t.Fatalf("Sam's yes: %v %v", r, err)
		}
		holds(t, "the steward's answer", b.answered(n).Text, "Circuit 4 is off.")
	}
}

func TestAWorkerAskedToAdviseIsToldItsYesGoesOn(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  cut_power: [clerk: advise, sam]\n", 1)
	b := newBench(t, files)
	fix := fixing("cut_power", map[string]any{"circuit": "4"})
	told := make(chan string, 1)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.System, "You keep the house's accounts") {
			select {
			case told <- turn.Prompt():
			default:
			}
			return testkit.Say(`{ "says": "yes", "note": "A new socket is within this month's money." }`)
		}
		return fix(turn)
	})
	n := b.chat("kit", "", "the kitchen socket sparked")
	dec := b.held(n)
	holds(t, "what the clerk is told", <-told, "You advise on it: a refusal from you ends it. Your yes does not end it", "to Sam")
	if len(dec.Referrals) != 1 || !dec.Referrals[0].Recommends || dec.Referrals[0].Name != "clerk" || strings.Join(dec.Asked, ",") != "sam" {
		t.Fatalf("the clerk's yes made the decision %+v", dec)
	}
	holds(t, "what Sam is asked", b.waitsOn(n, decisions.KindDecide).Text, "clerk recommends yes: A new socket is within this month's money.")
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "yes", ""); err != nil || !r.Taken {
		t.Fatalf("Sam's yes: %v %v", r, err)
	}
	holds(t, "the steward's answer", b.answered(n).Text, "Circuit 4 is off.")
}

func TestAnAskThatEndsUnansweredIsWithdrawn(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("cut_power", map[string]any{"circuit": "4"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	dec := b.held(n)
	b.later(week)
	// The decision is withdrawn first, and Sam told he is no longer asked
	// after it.
	b.until("the held job is withdrawn, and Sam no longer asked", func() bool {
		d, err := b.d.Get(dec.ID)
		asked := slices.ContainsFunc(must(b.reqs.WaitingOn("sam")), func(w requests.Wait) bool { return w.Entry == dec.Entry })
		return err == nil && d.Ended == decisions.Withdrawn && !asked
	})
	holds(t, "the steward's answer", b.waitsOn(n, requests.KindAnswer).Text, "Nobody decided within 7 days")
}

func TestAStopEndsTheAskOfAHeldJob(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?") && turn.Number == 1:
			return testkit.Use("refund", map[string]any{"charge": "ch_2", "why": "charged twice"})
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("done")
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	dec := b.held(sub)
	if dec.Team != "payments" || dec.Whose.Name != "ledger" || strings.Join(dec.Asked, ",") != "lee,priya" {
		t.Errorf("the refund is decided as %+v", dec)
	}
	bot, ledger := b.runs(n)[0], b.runs(sub)[0]
	if err := b.w.Stop(ctx, n, b.reqs.Person("al")); err != nil {
		t.Fatal(err)
	}
	for _, run := range []string{bot.Name, ledger.Name, dec.Ask} {
		if ended := b.corbie.Ended(t, run); ended.Status != "cancelled" {
			t.Errorf("the run %s ended %s", run, ended.Status)
		}
	}
	// The refund is withdrawn first, and Lee and Priya told they are no
	// longer asked after it.
	b.until("the refund is withdrawn, and nobody asked", func() bool {
		return must(b.d.Get(dec.ID)).Ended == decisions.Withdrawn && len(must(b.reqs.WaitingOn("lee"))) == 0 && len(must(b.reqs.WaitingOn("priya"))) == 0
	})
	b.settled(sub, requests.Answered)
	b.settled(n, requests.Answered)
}

func TestARuleThatTakesTooLongRefers(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "order_part: [./small-orders.js, sam]", "order_part: [./slow.yaml, sam]", 1)
	files["slow.yaml"] = "statechart: slow\ninitial: waiting\nstates:\n  waiting: { ticket: never, on: { never: over } }\n  over: { end: done, output: { says: yes } }\n"
	b := newBench(t, files)
	b.says(fixing("order_part", map[string]any{"part": "socket", "pounds": 12}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	var rule *client.Run
	b.until("the rule runs", func() bool {
		runs := b.listed(map[string]string{LabelKind: decisions.Deciding})
		if len(runs) == 1 {
			rule = runs[0]
		}
		return rule != nil
	})
	b.later(decisions.RuleLimit)
	dec := b.held(n)
	if len(dec.Referrals) != 1 || !strings.Contains(dec.Referrals[0].Note, "did not end within 15 minutes") {
		t.Errorf("the rule's word is %+v", dec.Referrals)
	}
	if ended := b.corbie.Ended(t, rule.ID); ended.Status != "cancelled" {
		t.Errorf("the rule's run ended %s", ended.Status)
	}
}
