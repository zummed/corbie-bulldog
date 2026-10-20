package decisions_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestDecisionWithdrawnWhenItsRunEnds(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	decs := b.held(decisions.Ask{Run: "ask-1", Team: office.Top, Request: n, Asking: fixerUp,
		Jobs: []decisions.Job{cutPower("4"), orderPart("order_part", "cooker", 300)}})
	rule := b.runner.next(t)
	entry := decs[0].Entry
	if err := b.d.AskEnded(ctx, "ask-1"); err != nil {
		t.Fatal(err)
	}
	rule.cancelled(t)
	for _, dec := range decs {
		if got := b.get(dec.ID); got.Ended != decisions.Withdrawn {
			t.Fatalf("a decision of the ended run is %+v", got)
		}
	}
	if len(b.waitingFor("sam")) != 0 {
		t.Fatal("the withdrawn decision still waits on Sam")
	}
	notices := b.entries(n, decisions.KindNotice)
	if len(notices) != 1 || notices[0].To[0].Name != "sam" || !strings.Contains(notices[0].Text, "no longer asked") {
		t.Fatalf("Sam is told %+v", notices)
	}
	if ds, err := b.reqs.Deliveries(notices[0].ID); err != nil || len(ds) != 1 || ds[0].Person != "sam" {
		t.Fatalf("the notice is carried to %+v, %v", ds, err)
	}
	r, err := b.d.Choose(ctx, decisions.Ref(decs[0].ID), requests.Identity("telegram", "4471"), decisions.SaysYes)
	if err != nil || r.Taken || !strings.Contains(r.Text, "withdrawn") {
		t.Fatalf("Sam's yes after is answered %+v, %v", r, err)
	}
	if b.answers("ask-1") != 0 {
		t.Fatal("an ask that ended was answered")
	}
	if ok, _, _ := b.d.OfEntry(entry); ok != nil && ok.Ended != decisions.Withdrawn {
		t.Fatal("the entry names a decision that waits")
	}
}

func TestDecisionsWrittenBeforeAnswered(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	b.mu.Lock()
	b.answer = func(ask string, v decisions.Verdict) error {
		if got, err := b.d.Get(dec.ID); err != nil || got.Ended != decisions.Yes {
			t.Errorf("the ask was answered before the decision was kept: %+v, %v", got, err)
		}
		return nil
	}
	b.mu.Unlock()
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	first := b.verdict("ask-1")
	b.restart()
	b.held(decisions.Ask{Run: "ask-1", Team: office.Top, Request: n, Asking: fixerUp, Jobs: []decisions.Job{cutPower("4")}})
	if b.answers("ask-1") != 2 || !slices.Equal(b.verdict("ask-1").Approve, first.Approve) {
		t.Fatalf("the ask found waiting again was answered %d times, %+v", b.answers("ask-1"), b.verdict("ask-1"))
	}

	held := b.one("ask-2", office.Top, n, fixerUp, cutPower("6"))
	if _, err := b.db.Exec(`CREATE TRIGGER full BEFORE UPDATE ON decisions_decisions BEGIN SELECT RAISE(FAIL, 'the disk is full'); END`); err != nil {
		t.Fatal(err)
	}
	r, err := b.d.Decide(ctx, held.ID, b.reqs.Person("sam"), decisions.SaysYes, "")
	if err == nil || r.Taken || !strings.Contains(r.Text, "not taken") {
		t.Fatalf("an answer that could not be kept is answered %+v, %v", r, err)
	}
	if _, err := b.db.Exec(`DROP TRIGGER full`); err != nil {
		t.Fatal(err)
	}
	if got := b.get(held.ID); got.Ended != "" || b.answers("ask-2") != 0 {
		t.Fatalf("the action is no longer held: %+v", got)
	}

	// The decision and the ask's answer are kept together, or neither is.
	last := b.one("ask-3", office.Top, n, fixerUp, cutPower("8"))
	if _, err := b.db.Exec(`CREATE TRIGGER lost BEFORE UPDATE ON decisions_asks BEGIN SELECT RAISE(FAIL, 'the disk is full'); END`); err != nil {
		t.Fatal(err)
	}
	r, err = b.d.Decide(ctx, last.ID, b.reqs.Person("sam"), decisions.SaysYes, "")
	if err == nil || r.Taken || !strings.Contains(r.Text, "not taken") {
		t.Fatalf("an answer whose ask's answer could not be kept is answered %+v, %v", r, err)
	}
	if _, err := b.db.Exec(`DROP TRIGGER lost`); err != nil {
		t.Fatal(err)
	}
	if got := b.get(last.ID); got.Ended != "" || b.answers("ask-3") != 0 {
		t.Fatalf("the decision was kept without its ask's answer: %+v", got)
	}
	b.decide(last.ID, "sam", decisions.SaysYes, "")
	b.restart()
	b.held(decisions.Ask{Run: "ask-3", Team: office.Top, Request: n, Asking: fixerUp, Jobs: []decisions.Job{cutPower("8")}})
	if v := b.verdict("ask-3"); b.answers("ask-3") != 2 || !slices.Equal(v.Approve, []string{"cut_power"}) {
		t.Fatalf("the ask found waiting again was answered %d times, %+v", b.answers("ask-3"), v)
	}
}

func TestEndedDecisionCarriedThroughARestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	b.mu.Lock()
	b.letFails = errors.New("work is not listening")
	b.mu.Unlock()
	text := "Duplicate confirmed; £40 refunded."
	id := b.ledgerAnswers(sub, text)
	b.runner.next(t).says(decisions.SaysYes, "No account number.")
	if err := b.trouble(); !strings.Contains(err.Error(), "work could not be told") {
		t.Fatalf("the trouble is %v", err)
	}
	b.mu.Lock()
	b.letFails = nil
	b.mu.Unlock()
	if got := b.answerOf(sub); got != text {
		t.Fatalf("the disclosed answer is %q", got)
	}
	b.restart()
	if l := b.let(id); !l.Disclosed || l.Said.Text != text {
		t.Fatalf("the disclosure is told %+v", l)
	}
	b.restart()
	if b.letsOf(id) != 1 || len(b.entries(sub, requests.KindAnswer)) != 1 || len(b.entries(sub, decisions.KindDecided)) != 1 {
		t.Fatalf("the disclosure was told %d times, its answer said %d times and its yes written %d times",
			b.letsOf(id), len(b.entries(sub, requests.KindAnswer)), len(b.entries(sub, decisions.KindDecided)))
	}

	h := newBench(t, house(t))
	n := h.chat("kit", office.Top, "let me into the garden")
	dec := h.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	h.d.Close()
	// Bulldog stops once Sam's yes is kept, before anyone is told of it.
	got := h.get(dec.ID)
	got.Ended, got.Decided, got.At = decisions.Yes, h.clock.Now(), nil
	got.By = &decisions.Word{Kind: office.IsPerson, Name: "sam", Called: "Sam", At: h.clock.Now()}
	if _, err := h.db.Exec(`UPDATE decisions_decisions SET ended = 'yes', owed = 'entries', data = ? WHERE id = ?`, jsonOf(got), dec.ID); err != nil {
		t.Fatal(err)
	}
	h.open()
	if o := h.outcome(dec.Put.ID); o.Ended != changes.Written || o.Decider != "sam" {
		t.Fatalf("the change ended %+v", o)
	}
	if len(h.waitingFor("sam")) != 0 {
		t.Fatal("the change still waits on Sam")
	}
	has(t, h.thread(n), "decided by sam", "approved by Sam")
}

func TestDecisionOutlivesARestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	b.restart()
	r, err := b.d.Choose(ctx, decisions.Ref(dec.ID), requests.Identity("slack", "U0LEE000"), decisions.SaysYes)
	if err != nil || !r.Taken {
		t.Fatalf("Lee's yes after a restart is answered %+v, %v", r, err)
	}
	if v := b.verdict("ask-1"); !slices.Equal(v.Approve, []string{"refund"}) || b.answers("ask-1") != 1 {
		t.Fatalf("the refund is answered %+v, %d times", v, b.answers("ask-1"))
	}

	h := newBench(t, house(t))
	n := h.chat("kit", office.Top, "a new cooker")
	big := h.one("ask-2", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	rule := h.runner.next(t)
	h.d.Close()
	h.runner.endsWhileAway(rule, decisions.SaysRefer, "£300 is more than I order alone.")
	h.open()
	h.until("the walk goes on to Sam", func() bool { return len(h.waitingFor("sam")) == 1 })
	starts := h.runner.started()
	if len(starts) != 2 || starts[0].Step != starts[1].Step || starts[0].Labels["step"] != starts[1].Labels["step"] {
		t.Fatalf("the rule was asked again as another run: %+v", starts)
	}
	if got := h.get(big.ID); len(got.Referrals) != 1 {
		t.Fatalf("the rule's referral was taken %d times", len(got.Referrals))
	}
}

func TestDecisionWalksOnAtAReading(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	team := "name: Payments\npeople: [lee, priya]\nlead: ledger\ndisclosure: [./no-account-numbers.yaml]\n"
	b.rewrite(map[string]string{"payments/team.yaml": team + "decide: { everything: [priya] }\n"})
	if on := b.waitsOn(sub); !slices.Equal(on, []string{"priya"}) {
		t.Fatalf("the refund waits on %v", on)
	}
	notices := b.entries(sub, decisions.KindNotice)
	if len(notices) != 1 || notices[0].To[0].Name != "lee" {
		t.Fatalf("the notices are %+v", notices)
	}
	if text := b.refused(dec.ID, requests.Identity("slack", "U0LEE000"), decisions.SaysYes); !strings.Contains(text, "no longer asked") {
		t.Fatalf("Lee's yes is answered %q", text)
	}
	b.rewrite(map[string]string{"payments/team.yaml": team + "decide: { everything: [raj] }\n"})
	if on := b.waitsOn(sub); !slices.Equal(on, []string{"raj"}) {
		t.Fatalf("the refund waits on %v", on)
	}
	if got := b.entries(sub, decisions.KindNotice); len(got) != 2 || got[1].To[0].Name != "priya" {
		t.Fatalf("the notices are %+v", got)
	}
	b.ofDecision(sub, dec.ID, decisions.KindNotice)
	ds, err := b.reqs.Deliveries(b.get(dec.ID).Entry)
	if err != nil || len(ds) != 1 || ds[0].Person != "raj" {
		t.Fatalf("the decision is carried to %+v, %v", ds, err)
	}

	id := b.ledgerAnswers(sub, "Duplicate confirmed; £40 refunded.")
	b.runner.next(t).says(decisions.SaysRefer, "0.9")
	b.until("Raj is asked", func() bool { return len(b.waitingFor("raj")) == 2 })
	b.rewrite(map[string]string{"payments/team.yaml": "name: Payments\npeople: [lee, priya]\nlead: ledger\ndecide: { everything: [raj] }\n"})
	if l := b.let(id); !l.Disclosed {
		t.Fatalf("the disclosure ended %+v", l)
	}
	if got := b.answerOf(sub); got != "Duplicate confirmed; £40 refunded." {
		t.Fatalf("the answer let go is %q", got)
	}

	h := newBench(t, house(t))
	n := h.chat("kit", office.Top, "a new cooker")
	order := h.one("ask-2", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	rule := h.runner.next(t)
	text := house(t)["office.yaml"]
	h.rewrite(map[string]string{"office.yaml": strings.Replace(text, "order_part: [./small-orders.js, sam]", "order_part: [sam]", 1)})
	rule.cancelled(t)
	if got := h.get(order.ID); !slices.Equal(got.Asked, []string{"sam"}) || len(got.Referrals) != 0 {
		t.Fatalf("with the rule taken off, the order is %+v", got)
	}
}

func TestStoreOfAnOlderBulldogRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if _, err := b.db.Exec(`UPDATE parts SET version = 0 WHERE area = 'decisions'`); err != nil {
		t.Fatal(err)
	}
	_, err := decisions.Open(b.db, b.reqs, b.ch)
	if err == nil || !strings.Contains(err.Error(), "remove .bulldog/bulldog.db and start again") {
		t.Fatalf("an older store is opened: %v", err)
	}
	if errors.Is(err, decisions.ErrNoDecision) {
		t.Fatal("the wrong error")
	}
}

func TestStoppedRequestWithdrawsItsDecisions(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	top, sub := b.helpRequest()
	held := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	id := b.ledgerAnswers(sub, "Card 4111 refunded.")
	b.runner.next(t).says(decisions.SaysRefer, "0.9")
	b.until("Lee is asked twice", func() bool { return len(b.waitingFor("lee")) == 2 })
	stopped, err := b.reqs.Stop(top, b.reqs.Person("al"))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.d.Stopped(ctx, stopped); err != nil {
		t.Fatal(err)
	}
	for _, dec := range []int64{held.ID, id} {
		if got := b.get(dec); got.Ended != decisions.Withdrawn {
			t.Fatalf("a decision of a stopped request is %+v", got)
		}
	}
	if len(b.waitingFor("lee")) != 0 || b.letsOf(id) != 0 {
		t.Fatal("a decision of a stopped request still waits, or was let go")
	}
}
