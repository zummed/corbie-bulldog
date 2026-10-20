package decisions_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// answerOf gives a request's answer by its lead, or "" while it has none.
func (b *bench) answerOf(n int64) string {
	b.t.Helper()
	got := b.entries(n, requests.KindAnswer)
	if len(got) == 0 {
		return ""
	}
	return got[len(got)-1].Text
}

// disclose hands what a team says outside itself to its disclosure check.
func (b *bench) disclose(s decisions.Said) int64 {
	b.t.Helper()
	id, err := b.d.Disclose(ctx, s)
	if err != nil {
		b.t.Fatal(err)
	}
	return id
}

// heldDisclosure gives the one disclosure a held answer was handed to as it
// was said.
func (b *bench) heldDisclosure(e *requests.Entry) int64 {
	b.t.Helper()
	decs, err := b.d.OfRequest(e.Request)
	if err != nil {
		b.t.Fatal(err)
	}
	var ids []int64
	for _, dec := range decs {
		if dec.Kind == decisions.OfDisclosure && dec.Said.Held == e.ID {
			ids = append(ids, dec.ID)
		}
	}
	if len(ids) != 1 {
		b.t.Fatalf("the held answer %d was handed to the disclosures %v", e.ID, ids)
	}
	return ids[0]
}

func (b *bench) ledgerAnswers(sub int64, text string) int64 {
	return b.disclose(decisions.Said{What: decisions.SaysAnswer, Request: sub, By: b.reqs.Worker("ledger"), Text: text})
}

func TestDisclosureHoldsWhatATeamSaysOutside(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	top, sub := b.helpRequest()
	id := b.ledgerAnswers(sub, "Duplicate confirmed; £40 refunded.")
	rule := b.runner.next(t)
	if b.answerOf(sub) != "" {
		t.Fatal("payments' answer was said before it was disclosed")
	}
	hasNo(t, b.seen(top, "dana"), "Duplicate confirmed")
	hasNo(t, b.seen(top, "al"), "Duplicate confirmed")
	rule.says(decisions.SaysYes, "No account number.")
	b.let(id)
	if got := b.answerOf(sub); got != "Duplicate confirmed; £40 refunded." {
		t.Fatalf("payments' answer is %q", got)
	}
	q := b.disclose(decisions.Said{What: decisions.SaysSub, Request: sub, By: b.reqs.Worker("ledger"), Team: "platform", Text: "what deployed on Tuesday?"})
	b.runner.next(t).says(decisions.SaysYes, "")
	if l := b.let(q); !l.Disclosed || l.Said.Team != "platform" || l.Said.Text != "what deployed on Tuesday?" {
		t.Fatalf("the question to platform ended %+v", l)
	}
}

func TestDisclosedAsDecidedRefusedGoesBack(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	top, sub := b.helpRequest()
	id := b.ledgerAnswers(sub, "Duplicate confirmed; card 4111 refunded £40.")
	b.runner.next(t).says(decisions.SaysPass, "It may name an account number, 0.90.")
	b.until("Lee is asked", func() bool { return len(b.waitingFor("lee")) == 1 })
	b.decide(id, "lee", decisions.SaysRefuse, "that names the card")
	l := b.let(id)
	if l.Disclosed || l.By != "Lee" || l.Note != "that names the card" {
		t.Fatalf("the disclosure ended %+v", l)
	}
	if b.answerOf(sub) != "" {
		t.Fatal("the refused answer was said")
	}
	e := b.entries(sub, decisions.KindDecided)[0]
	if e.By.Name != "lee" || e.To[0].Name != "ledger" || e.Read != "payments" || !strings.Contains(e.Text, "that names the card") {
		t.Fatalf("the refusal is %+v", e)
	}
	if st, _ := b.reqs.State(sub); st.Is != requests.Working {
		t.Fatalf("the refused request is %+v", st)
	}
	for _, person := range []string{"dana", "al"} {
		hasNo(t, b.seen(top, person), "card 4111")
		hasNo(t, b.seen(top, person), "that names the card")
	}
	again := b.ledgerAnswers(sub, "Duplicate confirmed; £40 refunded.")
	b.runner.next(t).says(decisions.SaysYes, "No account number.")
	b.let(again)
	if got := b.answerOf(sub); got != "Duplicate confirmed; £40 refunded." {
		t.Fatalf("the answer said is %q", got)
	}
}

func TestHeldAnswerOfALeadWhoIsAPersonRefused(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["support/team.yaml"] = "name: Support\npeople: [dana]\nlead: dana\ndisclosure: [./refuse-all.js]\n"
	files["support/refuse-all.js"] = `return { says: "refuse", note: "say it kindly" }`
	b := newBench(t, files)
	top := b.chat("al", "office", "a customer is cross")
	sub, err := b.reqs.Sub(top, b.reqs.Worker("bot"), "support", "what do we tell them?", "ticket-support")
	if err != nil {
		t.Fatal(err)
	}
	e, err := b.reqs.Say(sub.Number, b.reqs.Person("dana"), "tell them to calm down", nil, requests.Screens, false)
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != requests.KindHeld {
		t.Fatalf("Dana's answer is a %s", e.Kind)
	}
	id := b.heldDisclosure(e)
	b.runner.next(t).says(decisions.SaysRefuse, "say it kindly")
	b.let(id)
	ws, err := b.reqs.WaitingOn("dana")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(ws, func(w requests.Wait) bool { return w.Request == sub.Number && w.Kind == requests.KindLead }) {
		t.Fatalf("the sub-request does not wait on Dana again: %+v", ws)
	}
	r := b.entries(sub.Number, decisions.KindDecided)[0]
	if r.To[0].Name != "dana" || !strings.Contains(r.Text, "say it kindly") {
		t.Fatalf("the refusal is %+v", r)
	}
	b.runner.answers(func(decisions.Start) (string, error, bool) { return `{"says":"yes"}`, nil, true })
	e, err = b.reqs.Say(sub.Number, b.reqs.Person("dana"), "we are sorry, and have refunded you", nil, requests.Screens, false)
	if err != nil {
		t.Fatal(err)
	}
	id = b.heldDisclosure(e)
	b.let(id)
	if got := b.answerOf(sub.Number); got != "we are sorry, and have refunded you" {
		t.Fatalf("Dana's disclosed answer is %q", got)
	}
}

func TestDecisionsRefusedDisclosureNotToldOutside(t *testing.T) {
	t.Parallel()
	b := newBench(t, withPayments(t, "name: Payments\npeople: [lee, priya]\nlead: ledger\ndecide: { everything: [lee, priya] }\ndisclosure: [./refuse-all.js]\n"))
	top, sub := b.helpRequest()
	for range 3 {
		id := b.ledgerAnswers(sub, "Card 4111 was refunded.")
		b.runner.next(t).says(decisions.SaysRefuse, "never")
		b.let(id)
	}
	line, ok, err := b.d.NotDisclosed(sub)
	if err != nil || !ok || line != "payments did not disclose its answer (refuse-all.js)" {
		t.Fatalf("the line is %q, %v, %v", line, ok, err)
	}
	for _, person := range []string{"dana", "al"} {
		for _, word := range []string{"4111", "never", "refuse-all"} {
			hasNo(t, b.seen(top, person), word)
		}
	}
	v, err := b.reqs.View(top, "dana")
	if err != nil {
		t.Fatal(err)
	}
	if v.Subs[0].Answer != "" {
		t.Fatalf("Dana sees payments' answer as %q", v.Subs[0].Answer)
	}

	w := newBench(t, work(t))
	_, sub = w.helpRequest()
	id := w.ledgerAnswers(sub, "Card 4111 was refunded.")
	w.runner.next(t).says(decisions.SaysPass, "0.9")
	w.until("Lee is asked", func() bool { return len(w.waitingFor("lee")) == 1 })
	if _, ok, _ := w.d.NotDisclosed(sub); ok {
		t.Fatal("the line is there while the disclosure waits")
	}
	w.decide(id, "lee", decisions.SaysRefuse, "names the card")
	if line, ok, _ := w.d.NotDisclosed(sub); !ok || line != "payments did not disclose its answer (Lee)" {
		t.Fatalf("the line is %q", line)
	}
	w.ledgerAnswers(sub, "Refunded.")
	w.runner.next(t)
	if line, ok, _ := w.d.NotDisclosed(sub); ok {
		t.Fatalf("the line is %q while a later answer waits on its disclosure check", line)
	}
}

func TestDisclosureOfATeamWithNoDisclosureCheckRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	_, err := b.d.Disclose(ctx, decisions.Said{What: decisions.SaysAnswer, Request: n, By: b.reqs.Worker("steward"), Text: "Fixed."})
	if err == nil || !strings.Contains(err.Error(), "The house sets no disclosure check") {
		t.Fatalf("a disclosure of the house is %v", err)
	}
	if decs, err := b.d.OfRequest(n); err != nil || len(decs) != 0 {
		t.Fatalf("the request holds the decisions %+v, %v", decs, err)
	}
}

func TestRefusedQuestionStaysInside(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	top, sub := b.helpRequest()
	id := b.disclose(decisions.Said{What: decisions.SaysSub, Request: sub, By: b.reqs.Worker("ledger"), Team: "platform", Text: "which deploy touched card 4111?", Ticket: "t-7"})
	b.runner.next(t).says(decisions.SaysPass, "It may name a card number, 0.80.")
	b.until("Lee is asked", func() bool { return len(b.waitingFor("lee")) == 1 })
	b.decide(id, "lee", decisions.SaysRefuse, "no card numbers to platform")
	l := b.let(id)
	if l.Disclosed || l.Said.Ticket != "t-7" || !strings.Contains(l.Text(), "not disclosed") || !strings.Contains(l.Text(), "Lee") ||
		!strings.Contains(l.Text(), "no card numbers to platform") {
		t.Fatalf("the ledger's tool is told %q", l.Text())
	}
	e := b.entries(sub, decisions.KindDecided)[0]
	if e.To[0].Name != "ledger" || e.Read != "payments" {
		t.Fatalf("the refusal is %+v", e)
	}
	for _, person := range []string{"al", "dana"} {
		hasNo(t, b.seen(top, person), "4111")
		hasNo(t, b.seen(top, person), "no card numbers")
	}
}

func TestDisclosureDeciderShownWhatWouldLeave(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	b.ledgerAnswers(sub, "Duplicate confirmed; £40 refunded.")
	r := b.runner.next(t)
	rel := r.Input["disclosure"].(map[string]any)
	notes := r.Input["notes"].([]map[string]any)
	if rel["what"] != "answer" || rel["text"] != "Duplicate confirmed; £40 refunded." || rel["to"] != "bot, of office" || rel["read_at"] != "#help on slack" ||
		len(notes) != 1 || !strings.Contains(notes[0]["text"].(string), "A charge taken twice") || r.Input["request"].(map[string]any)["asked"] != "was 4411 charged twice?" {
		t.Fatalf("the rule is given %v", r.Input)
	}
	r.says(decisions.SaysYes, "")

	files := work(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`, `"#help": { team: payments, guests: true }`, 1)
	w := newBench(t, files)
	dana, err := w.reqs.AskAt("slack", "#help", "C1/400.1", "", "U0DANA00", "is 4411 refunded?", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if dana.Team != "payments" {
		t.Fatalf("Dana's request went to %s", dana.Team)
	}
	w.disclose(decisions.Said{What: decisions.SaysTell, Request: dana.Number, By: w.reqs.Worker("ledger"), To: dana.Asker, Text: "looking at the charges now"})
	r = w.runner.next(t)
	rel = r.Input["disclosure"].(map[string]any)
	if rel["what"] != "tell" || rel["to"] != "Dana" || rel["read_at"] != "#help on slack" || rel["said_in"] != dana.Number || len(r.Input["notes"].([]map[string]any)) != 1 {
		t.Fatalf("the rule is given %v", r.Input)
	}
	r.says(decisions.SaysYes, "")
	b.disclose(decisions.Said{What: decisions.SaysSub, Request: sub, By: b.reqs.Worker("ledger"), Team: "platform", Text: "what deployed?"})
	r = b.runner.next(t)
	rel = r.Input["disclosure"].(map[string]any)
	if rel["what"] != "sub" || rel["to"] != "Platform" || rel["said_in"] != sub {
		t.Fatalf("the rule is given %v", r.Input)
	}
	r.says(decisions.SaysPass, "unsure")
	b.until("Lee is asked", func() bool { return len(b.waitingFor("lee")) == 1 })
	if text := b.waitingFor("lee")[0].Text; !strings.Contains(text, "no-account-numbers.yaml passed it on: unsure") || !strings.Contains(text, "what deployed?") {
		t.Fatalf("Lee is shown %s", text)
	}
}

func TestDecisionsDisclosureLetsOutExactlyWhatWasDecided(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	top, sub := b.helpRequest()
	text := "Duplicate confirmed;  £40 refunded.\n\n— the ledger "
	id := b.ledgerAnswers(sub, text)
	b.runner.next(t).says(decisions.SaysYes, "")
	b.let(id)
	if got := b.answerOf(sub); got != text {
		t.Fatalf("what left is %q, not %q", got, text)
	}
	refused := b.ledgerAnswers(sub, "Card 4111 refunded.")
	b.runner.next(t).says(decisions.SaysRefuse, "it names card 4111")
	b.let(refused)
	for _, person := range []string{"dana", "al"} {
		hasNo(t, b.seen(top, person), "4111")
	}
	if got := b.answerOf(sub); got != text {
		t.Fatalf("after the refusal the answer is %q", got)
	}
}

func TestDecisionsDisclosureReadsCurrentNotes(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	b.ledgerAnswers(sub, "first")
	r := b.runner.next(t)
	if !strings.Contains(r.Input["notes"].([]map[string]any)[0]["text"].(string), "A charge taken twice") {
		t.Fatalf("the rule is given the notes %v", r.Input["notes"])
	}
	r.says(decisions.SaysYes, "")
	testkit.Write(t, b.dir, map[string]string{"payments/notes/refunds.md": "# Refunds\n\nNever say a customer's postcode.\n"})
	b.ledgerAnswers(sub, "second")
	r = b.runner.next(t)
	if !strings.Contains(r.Input["notes"].([]map[string]any)[0]["text"].(string), "Never say a customer's postcode.") {
		t.Fatalf("the rule is given the notes %v", r.Input["notes"])
	}
}

func TestHeldAnswerOfALeadWhoIsAPersonLeftByAStopIsHandedOnce(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["support/team.yaml"] = "name: Support\npeople: [dana]\nlead: dana\ndisclosure: [./yes.js]\n"
	files["support/yes.js"] = `return { says: "yes" }`
	b := newBench(t, files)
	top := b.chat("al", "office", "a customer is cross")
	sub, err := b.reqs.Sub(top, b.reqs.Worker("bot"), "support", "what do we tell them?", "ticket-support")
	if err != nil {
		t.Fatal(err)
	}
	// Bulldog stops after Dana's answer is kept and before it is handed to
	// the disclosure check.
	b.reqs.Held = func(*requests.Entry) error { return nil }
	e, err := b.reqs.Say(sub.Number, b.reqs.Person("dana"), "we are sorry, and have refunded you", nil, requests.Screens, false)
	if err != nil || e.Kind != requests.KindHeld {
		t.Fatalf("Dana's answer is %+v, %v", e, err)
	}
	if decs, err := b.d.OfRequest(sub.Number); err != nil || len(decs) != 0 {
		t.Fatalf("before the restart the sub-request's decisions are %v, %v", decs, err)
	}
	// Opened again, the decisions hand it to its check as they read the
	// office, once however often they read it.
	b.runner.answers(func(decisions.Start) (string, error, bool) { return `{"says":"yes"}`, nil, true })
	b.restart()
	id := b.heldDisclosure(e)
	b.read()
	if again := b.heldDisclosure(e); again != id {
		t.Errorf("read again, the held answer was handed to %d after %d", again, id)
	}
	b.let(id)
	if got := b.answerOf(sub.Number); got != "we are sorry, and have refunded you" {
		t.Fatalf("Dana's disclosed answer is %q", got)
	}
}
