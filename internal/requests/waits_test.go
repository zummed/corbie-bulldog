package requests

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
)

// asksPerson is a ledger that asks the person its prompt names, with the
// choices it gives, then says what it was told.
func asksPerson(turn testkit.Turn) testkit.Reply {
	if turn.Number == 1 {
		who, question, _ := strings.Cut(turn.Prompt(), ": ")
		return testkit.Use("ask_person", map[string]any{"who": who, "question": question, "choices": []string{"today", "tomorrow"}})
	}
	return testkit.Say("Told: " + turn.Last())
}

// TestRequestsAskOnlyTheTeamsOwnPeople: a team asks only its own people
// and those its team file names; any other ask is refused, and nobody is
// asked. A question goes to each person asked, in their direct messages on
// each bridge that reaches them directly, once.
func TestRequestsAskOnlyTheTeamsOwnPeople(t *testing.T) {
	t.Parallel()
	o := newOffice(t, asksPerson, func(files map[string]string) {
		files["office.yaml"] = strings.Replace(files["office.yaml"], `lee: { name: Lee, telegram: "4402", slack: U0LEE }`, `lee: { name: Lee, telegram: "4402", slack: U0LEE, gitlab: "1042" }`, 1)
	})
	// Until the engine refuses it, a question to a person the team may not
	// ask is put to no one: it is shown to no one, and taken from no one.
	idle := o.engine()
	first := o.chat(idle, "payments", "dana: Is Dana in?")
	unaskable := o.waitFor("Is Dana in?")
	if waiting, err := idle.Waiting(ctx, "dana"); err != nil || len(waiting) != 0 {
		t.Errorf("Dana is waited on by %+v (%v)", waiting, err)
	}
	if err := idle.Answer(ctx, unaskable.ID, "dana", "Yes.", ""); !errors.Is(err, ErrNoQuestion) {
		t.Errorf("Dana answering is %v", err)
	}
	if look, err := idle.Look(ctx, first); err != nil || len(look.Forks) != 0 {
		t.Errorf("the request's forks are %+v (%v)", look, err)
	}
	e, stop := o.run()
	answer := o.answered(first)
	if !strings.Contains(answer, "Payments may not ask dana: a team asks its own people, and those its team file names. It may ask lee, priya.") {
		t.Errorf("the ledger is told %s", answer)
	}
	run := o.chat(e, "payments", "lee: When is the refund?")
	asked := o.waitFor("When is the refund?")
	sent := o.until(2)
	ref := e.hash("ref", asked.ID)
	for i, to := range map[int]string{0: "4402", 1: "U0LEE"} {
		m := sent[i]
		bridge := map[int]string{0: "telegram", 1: "slack"}[i]
		if m.Bridge != bridge || m.To != to || !m.Direct || m.Text != "Payments asks:\n\nWhen is the refund?" || fmt.Sprint(m.Choices) != "[today tomorrow]" || m.Ref != ref {
			t.Errorf("the question is carried as %+v", m)
		}
		if l := m.Labels; l["kind"] != "out" || l["bridge"] != bridge || l["wait"] != asked.ID || l["to"] != e.hash("person", "lee") {
			t.Errorf("the question is carried by a run labelled %v", l)
		}
	}
	// A second engine carries nothing again; it carries Priya's question,
	// which comes after its first listing.
	stop()
	again, _ := o.run()
	o.chat(again, "payments", "priya: Can you check 4412?")
	if sent := o.until(3); len(sent) != 3 || sent[2].To != "U0PRIYA" {
		t.Errorf("the carrier carried %+v", sent)
	}
	if err := again.Answer(ctx, asked.ID, "lee", "Tomorrow, after the batch.", "4402:12"); err != nil {
		t.Fatal(err)
	}
	if answer := o.answered(run); !strings.Contains(answer, `"answer":"Tomorrow, after the batch.","by":"lee","thing":"4402:12"`) {
		t.Errorf("the ledger is told %s", answer)
	}
}

// TestRequestsOnlyTheAskedAnswer: a wait is answered only by a person it
// waits on, while it waits, and a decision only with one of its choices,
// whatever its case and the spaces around it.
// The first of the people asked together to answer gives the answer.
func TestRequestsOnlyTheAskedAnswer(t *testing.T) {
	t.Parallel()
	o := newOffice(t, orders, func(files map[string]string) {
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "decide: [./small-orders.js, lee]", "decide: [./small-orders.js, lee, priya]", 1)
	})
	e, _ := o.run()
	run := o.chat(e, "payments", "Order a part for £35.")
	asked := o.waitFor("May these run?")
	w, err := e.WaitOf(ctx, e.hash("ref", asked.ID))
	if err != nil || w == nil || w.Decision != "decide" || fmt.Sprint(w.People) != "[lee priya]" || w.Request != run || w.Team != "payments" {
		t.Fatalf("the wait is %+v (%v)", w, err)
	}
	for _, c := range []struct {
		person, answer string
		want           error
	}{
		{"dana", "yes", ErrNoQuestion},
		{"U0GUEST0", "yes", ErrNoQuestion},
		{"lee", "maybe", ErrNotAChoice},
	} {
		if err := e.Answer(ctx, asked.ID, c.person, c.answer, ""); !errors.Is(err, c.want) {
			t.Errorf("%s answering %q is %v", c.person, c.answer, err)
		}
	}
	if err := e.Answer(ctx, asked.ID, "lee", " Yes ", "U0LEE:1"); err != nil {
		t.Fatal(err)
	}
	if answer := o.answered(run); !strings.Contains(answer, `"ordered":35`) {
		t.Errorf("the order gave %q", answer)
	}
	if err := e.Answer(ctx, asked.ID, "priya", "no", ""); !errors.Is(err, ErrNoQuestion) {
		t.Errorf("Priya answering after Lee is %v", err)
	}
	if shown := o.ended(asked.ID); fmt.Sprint(shown.Output) != "map[answer:yes by:lee thing:U0LEE:1]" {
		t.Errorf("the decision ended with %v", shown.Output)
	}
	if err := e.Answer(ctx, "client/20261005T090000.000Z-0000000000", "lee", "yes", ""); !errors.Is(err, ErrNoQuestion) {
		t.Errorf("answering no wait is %v", err)
	}
}

// A request stopped while it waits on another team stops that team's
// request too, and the question that request put to Lee.
func TestStoppedRequestStopsItsSubRequest(t *testing.T) {
	t.Parallel()
	o := newOffice(t, func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.System, "You work for Payments") {
			return asksPerson(turn)
		}
		return asksTwice(turn)
	}, nil)
	e, _ := o.run()
	run := o.chat(e, "office", "lee: Was 4411 refunded?")
	asked := o.waitFor("people:[lee]")
	subs := o.requests("payments")
	if len(subs) != 1 {
		t.Fatalf("payments has %d requests", len(subs))
	}
	if look, err := e.Look(ctx, run); err != nil || look.Status != "waiting" || states(look.Stages) != "work:waiting" {
		t.Errorf("the request is %+v (%v)", look, err)
	}
	if err := e.Stop(ctx, run); err != nil {
		t.Fatal(err)
	}
	o.ended(run)
	if look, err := e.Look(ctx, run); err != nil || look.Status != "stopped" || states(look.Stages) != "work:stopped" {
		t.Errorf("the stopped request is %+v (%v)", look, err)
	}
	if shown := o.ended(subs[0].ID); shown.Status != "cancelled" {
		t.Errorf("payments' request ended %s", shown.Status)
	}
	if shown := o.ended(asked.ID); shown.Status != "cancelled" {
		t.Errorf("the question to Lee ended %s", shown.Status)
	}
	for _, id := range []string{run, subs[0].ID} {
		if q, err := e.Request(ctx, id); err != nil || q.Status != "stopped" || q.Answer != nil {
			t.Errorf("the request %s is %+v (%v)", id, q, err)
		}
	}
	if err := e.Stop(ctx, asked.ID); !errors.Is(err, ErrNoRequest) {
		t.Errorf("stopping a wait is %v", err)
	}
}

// An answer is carried while its request ended less than a day ago. A week
// later the run that carried it is past keeping and its request is not:
// started again, Bulldog acts on the request again and carries nothing.
func TestAnswerLetGoAfterADay(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e, stop := o.run()
	run, err := e.Ask(ctx, refund("C01/1700000000.0002"))
	if err != nil {
		t.Fatal(err)
	}
	out := o.until(1)[0].Run
	o.ended(out)
	stop()
	o.corbie.Clock.Move(8 * 24 * time.Hour)
	o.corbie.Until(t, "the run that carried the answer is past keeping", func() bool {
		_, err := o.corbie.Show(ctx, out)
		return errors.Is(err, client.ErrNoRun)
	})
	shown, err := o.corbie.Show(ctx, run)
	if err != nil {
		t.Fatalf("the request is not kept: %v", err)
	}
	again := o.engine()
	if err := again.ended(ctx, again.Reading(), shown); err != nil {
		t.Fatal(err)
	}
	if err := again.everything(ctx); err != nil {
		t.Fatal(err)
	}
	if sent := o.carrier.carried(); len(sent) != 1 {
		t.Errorf("the answer was carried %d times", len(sent))
	}
}

// A sub-request is kept longer than the wait that asked it. Started again
// after the wait is past keeping, Bulldog acts on the sub-request with
// nothing to answer, and reports nothing.
func TestSubRequestOutlivesItsWait(t *testing.T) {
	t.Parallel()
	o := newOffice(t, asksTwice, nil)
	e, stop := o.run()
	o.answered(o.chat(e, "office", "first?"))
	subs := o.requests("payments")
	if len(subs) != 1 {
		t.Fatalf("payments was asked %d times", len(subs))
	}
	o.ended(subs[0].ID)
	stop()
	o.corbie.Clock.Move(8 * 24 * time.Hour)
	o.corbie.Until(t, "the wait is past keeping", func() bool {
		_, err := o.corbie.Show(ctx, subs[0].Labels["wait"])
		return errors.Is(err, client.ErrNoRun)
	})
	if _, err := o.corbie.Show(ctx, subs[0].ID); err != nil {
		t.Fatalf("the sub-request is not kept: %v", err)
	}
	if err := o.engine().everything(ctx); err != nil {
		t.Errorf("acting on every run again: %v", err)
	}
}

// A question offers at most 8 choices, each of at most 75 characters;
// any other is refused, and nobody is asked.
func TestTooManyChoicesRefused(t *testing.T) {
	t.Parallel()
	o := newOffice(t, func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			choices := strings.Split(turn.Prompt(), ", ")
			return testkit.Use("ask_person", map[string]any{"who": "lee", "question": "Which one?", "choices": choices})
		}
		return testkit.Say("Told: " + turn.Last())
	}, nil)
	e, _ := o.run()
	for _, choices := range []string{"a, b, c, d, e, f, g, h, i", "a, " + strings.Repeat("b", 76)} {
		if answer := o.answered(o.chat(e, "payments", choices)); !strings.Contains(answer, "A question offers at most 8 choices, each of at most 75 characters.") {
			t.Errorf("offering %q, the ledger is told %s", choices, answer)
		}
	}
	if sent := o.carrier.carried(); len(sent) != 0 {
		t.Errorf("the carrier carried %+v", sent)
	}
}
