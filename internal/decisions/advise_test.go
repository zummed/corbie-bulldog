package decisions_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
)

func TestDecisionsAdvisingYesNeverApproves(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer: advise, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	b.runner.next(t).says(decisions.SaysYes, "breaker 4 is the kitchen ring only")
	b.until("the recommendation reaches Sam", func() bool { return len(b.waitingFor("sam")) == 1 && len(b.get(dec.ID).Asked) == 1 })
	if got := b.get(dec.ID); got.Ended != "" || len(got.Referrals) != 1 || !got.Referrals[0].Recommends {
		t.Fatalf("the reviewer's yes made the decision %+v", got)
	}
	if b.answers("ask-1") != 0 {
		t.Fatal("the reviewer's yes answered the ask")
	}
	if ws := b.waitingFor("sam"); len(ws) != 1 || !strings.Contains(ws[0].Text, "reviewer recommends yes: breaker 4 is the kitchen ring only") {
		t.Fatalf("Sam is asked with %+v", ws)
	}
	lines := b.thread(n)
	has(t, lines, "recommended by reviewer: reviewer recommends yes: breaker 4 is the kitchen ring only")
	hasNo(t, lines, "approved by reviewer")
	b.ofDecision(n, dec.ID, decisions.KindRecommended)
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	if v := b.verdict("ask-1"); !slices.Equal(v.Approve, []string{"cut_power"}) {
		t.Fatalf("Sam's yes is answered %+v", v)
	}
	if got := b.ended(dec.ID); got.By.Name != "sam" {
		t.Fatalf("the decision was ended by %+v", got.By)
	}

	// Last on its list, a rule's yes goes on as a referral would, to the
	// team's everything; on a disclosure, it lets nothing go.
	b = newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  open_door: [./b.js: advise]"))
	n = b.chat("kit", office.Top, "let the plumber in")
	dec = b.one("ask-1", office.Top, n, fixerUp, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the front door"}})
	b.runner.next(t).says(decisions.SaysYes, "the plumber is expected")
	b.until("the recommendation reaches the house's everything", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	has(t, b.thread(n), "b.js recommends yes: the plumber is expected")

	w := newBench(t, withPayments(t, "people: [lee, priya]\nlead: ledger\ndisclosure: [./no-account-numbers.yaml: advise]\n"))
	_, sub := w.helpRequest()
	id, err := w.d.Disclose(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: w.reqs.Worker("ledger"), Text: "Duplicate confirmed; £40 refunded."})
	if err != nil {
		t.Fatal(err)
	}
	w.runner.next(t).says(decisions.SaysYes, "No account number.")
	w.until("the disclosure reaches Lee and Priya", func() bool { return len(w.get(id).Asked) == 2 })
	if w.letsOf(id) != 0 {
		t.Fatal("the rule's yes let the answer go")
	}
}

func TestAdvisingRefusalEndsAndReferralGoesOn(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer: advise, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("6"))
	b.runner.next(t).says(decisions.SaysRefuse, "That circuit feeds the freezer.")
	if v := b.verdict("ask-1"); len(v.Approve) != 0 || !strings.Contains(v.Note, "refused by reviewer: That circuit feeds the freezer.") {
		t.Fatalf("the ask is answered %+v", v)
	}
	if got := b.get(dec.ID); len(got.WasAsked) != 0 {
		t.Fatalf("%v were asked after the reviewer refused", got.WasAsked)
	}
	dec = b.one("ask-2", office.Top, n, fixerUp, cutPower("4"))
	b.runner.next(t).says(decisions.SaysRefer, "it may feed the freezer, 0.6")
	b.until("the referral reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	has(t, b.thread(n), "referred by reviewer: it may feed the freezer, 0.6")
}

func TestAdvisingDeciderToldItsYesGoesOn(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer: advise, ./a.js, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	r := b.runner.next(t)
	goes := r.Input["referral_goes_to"].([]map[string]any)
	if r.Input["advises"] != true || len(goes) != 2 || goes[0]["to"] != "a.js" || goes[1]["to"] != "Sam" {
		t.Fatalf("the reviewer is given advises %v and referral_goes_to %v", r.Input["advises"], goes)
	}
	ends := r.Input["ends"].(string)
	for _, words := range []string{"a refusal from you ends it", "Your yes does not end it", "as a recommendation, with your note, to a.js"} {
		if !strings.Contains(ends, words) || !strings.Contains(r.Said, words) {
			t.Fatalf("the reviewer is told %q, and in words %q", ends, r.Said)
		}
	}
	r.says(decisions.SaysYes, "breaker 4 is the kitchen")
	a := b.runner.next(t)
	referrals := a.Input["referrals"].([]map[string]any)
	if a.Input["advises"] != false || len(referrals) != 1 || referrals[0]["by"] != "reviewer" || referrals[0]["says"] != decisions.SaysYes || referrals[0]["note"] != "breaker 4 is the kitchen" {
		t.Fatalf("a.js is given advises %v and the referrals %v", a.Input["advises"], referrals)
	}
	if !strings.Contains(a.Input["ends"].(string), "Your yes or your refusal ends it") {
		t.Fatalf("a.js is told %s", a.Input["ends"])
	}
}

func TestAWorkerMayAdviseOnOneListAndDecideOnAnother(t *testing.T) {
	t.Parallel()
	// The reviewer advises on cut_power and decides the house's
	// everything: once it has advised, it is not asked again on the walk.
	b := newBench(t, withDecide(t, "decide:\n  everything: [reviewer, sam]\n  cut_power: [reviewer: advise]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	b.runner.next(t).says(decisions.SaysYes, "breaker 4 is the kitchen")
	b.until("the recommendation reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	if starts := b.runner.started(); len(starts) != 1 {
		t.Fatalf("the reviewer was asked %d times", len(starts))
	}
	// On a held action of its everything alone, its yes ends it.
	dec = b.one("ask-2", office.Top, n, fixerUp, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the front door"}})
	r := b.runner.next(t)
	if r.Input["advises"] != false {
		t.Fatalf("the reviewer is told it advises on everything")
	}
	r.says(decisions.SaysYes, "the plumber is expected")
	if got := b.ended(dec.ID); got.Ended != decisions.Yes || got.By.Name != "reviewer" {
		t.Fatalf("the reviewer's yes on everything ended it %+v", got)
	}
}

func TestAnAdviserIsHeldToWhatItWasTold(t *testing.T) {
	t.Parallel()
	// The list is read again while the reviewer is asked, with it no longer
	// marked to advise: it was told its yes goes on, and so it does.
	files := withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer: advise, sam]")
	b := newBench(t, files)
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	r := b.runner.next(t)
	b.rewrite(withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer, sam]"))
	r.says(decisions.SaysYes, "breaker 4 is the kitchen")
	b.until("the recommendation reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	if b.answers("ask-1") != 0 {
		t.Fatal("the reviewer's yes answered the ask")
	}
}

func TestAnAdviserIsHeldToItAfterARestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer: advise, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	r := b.runner.next(t)
	b.d.Close()
	b.runner.endsWhileAway(r, decisions.SaysYes, "breaker 4 is the kitchen")
	b.open()
	b.until("the recommendation reaches Sam", func() bool { return len(b.waitingFor("sam")) == 1 })
	if got := b.get(dec.ID); got.Ended != "" || len(got.Referrals) != 1 || !got.Referrals[0].Recommends {
		t.Fatalf("the reviewer's yes, read after a restart, made the decision %+v", got)
	}
}
