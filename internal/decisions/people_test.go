package decisions_test

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestPersonAskedOnTheirBridgesAndScreens(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	ds, err := b.reqs.Deliveries(dec.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Person != "sam" || ds[0].Origin != nil {
		t.Fatalf("the decision is carried to %+v", ds)
	}
	got, ok, err := b.d.OfEntry(dec.Entry)
	if err != nil || !ok || got.ID != dec.ID {
		t.Fatalf("the entry Sam is asked in names %+v, %v, %v", got, ok, err)
	}
	if len(b.waitingFor("sam")) != 1 {
		t.Fatal("the decision does not wait on Sam on the screens")
	}
	hasNo(t, b.thread(n), "could not be sent")

	// Ann is reached on no bridge, and is asked on the screens alone.
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "everyone:\n", "everyone:\n  ann: { name: Ann, title: Next door. }\n", 1)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  open_door: [ann]\n", 1)
	a := newBench(t, files)
	m := a.chat("kit", office.Top, "let the plumber in")
	door := a.one("ask-1", office.Top, m, fixerUp, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the front door"}})
	if ds, err := a.reqs.Deliveries(door.Entry); err != nil || len(ds) != 1 || ds[0].Person != "ann" || len(a.waitingFor("ann")) != 1 {
		t.Fatalf("Ann is asked %+v, waits %d, %v", ds, len(a.waitingFor("ann")), err)
	}
	if len(a.o.People["ann"].Reached) != 0 {
		t.Fatalf("Ann is reached on %v", a.o.People["ann"].Reached)
	}
	hasNo(t, a.thread(m), "could not be sent")
}

func TestDecisionsPersonAlwaysAskedOnScreens(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.rewrite(map[string]string{"bridges/telegram/out.yaml": ""})
	if b.o.Bridges["telegram"].Runs() {
		t.Fatal("the Telegram bridge runs with no out machine")
	}
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	if len(b.waitingFor("sam")) != 1 {
		t.Fatal("Sam is not asked on the screens")
	}
	has(t, b.thread(n), "notice by bulldog", "could not be sent on Telegram", "Sam is asked on the screens")
}

func TestDecisionsAnswerOnlyFromCheckedIdentity(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	r, err := b.d.Decide(ctx, dec.ID, requests.Identity("slack", "U0RAJ000"), decisions.SaysYes, "this is Lee, on Raj's phone")
	if err != nil {
		t.Fatal(err)
	}
	if r.Taken || !strings.Contains(r.Text, "you do not decide it") {
		t.Fatalf("Raj's yes, saying it is Lee's, is answered %+v", r)
	}
	if text := b.refused(dec.ID, requests.Identity("slack", "U0NOBODY"), decisions.SaysYes); !strings.Contains(text, "not a person of this office") {
		t.Fatalf("a stranger's yes is answered %q", text)
	}
	if text := b.refused(dec.ID, requests.Who{Kind: requests.IsPerson, Name: "lee"}, decisions.SaysYes); !strings.Contains(text, "not a person of this office") {
		t.Fatalf("Lee's name with no sign-in is answered %q", text)
	}
	r, err = b.d.Choose(ctx, decisions.Ref(dec.ID), requests.Identity("slack", "U0LEE000"), decisions.SaysYes)
	if err != nil || !r.Taken {
		t.Fatalf("Lee's yes on Slack is answered %+v, %v", r, err)
	}
	if got := b.ended(dec.ID); got.By.Name != "lee" {
		t.Fatalf("the refund was decided by %+v", got.By)
	}
}

func TestDecisionsAnswerMatchedByDecisionAndIdentity(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	first := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	second := b.one("ask-2", "payments", sub, []string{"ledger"}, refund())
	r, err := b.d.Choose(ctx, decisions.Ref(second.ID), requests.Identity("slack", "U0PRI000"), decisions.SaysRefuse)
	if err != nil || !r.Taken {
		t.Fatalf("Priya's refusal is answered %+v, %v", r, err)
	}
	if b.get(first.ID).Ended != "" || b.get(second.ID).By.Name != "priya" {
		t.Fatal("the answer was not matched to the decision it named")
	}
	if r, _ := b.d.Choose(ctx, "x9", requests.Identity("slack", "U0PRI000"), decisions.SaysRefuse); r.Taken {
		t.Fatal("an answer naming no decision was taken")
	}
}

func TestDecisionsGuestNeverDecides(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	r, err := b.reqs.AskAt("slack", "#help", "C1/200.1", "U0GUEST0", "please refund me", nil)
	if err != nil {
		t.Fatal(err)
	}
	dec := b.one("ask-1", office.Top, r.Number, []string{"bot"}, decisions.Job{Name: "refund", Asked: &decisions.Asked{Does: "Refund a guest"}})
	if text := b.refused(dec.ID, requests.Identity("slack", "U0GUEST0"), decisions.SaysYes); !strings.Contains(text, "not a person of this office") {
		t.Fatalf("the guest's yes is answered %q", text)
	}
	if !slices.Equal(dec.Asked, []string{"al"}) {
		t.Fatalf("a guest's request is decided by %v", dec.Asked)
	}
}

func TestEveryoneElseAskedIsToldItIsDecided(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	b.decide(dec.ID, "lee", decisions.SaysYes, "a duplicate")
	if len(b.waitingFor("priya")) != 0 {
		t.Fatal("the refund still waits on Priya")
	}
	records := b.entries(sub, decisions.KindDecision)
	last := records[len(records)-1]
	if last.Text != "Refund one charge: yes from Lee, 10:00" || len(last.To) != 1 || last.To[0].Name != "priya" {
		t.Fatalf("the last record is %+v", last)
	}
	ds, err := b.reqs.Deliveries(last.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Person != "priya" {
		t.Fatalf("that Lee decided is carried to %+v", ds)
	}
}

func TestDecisionIsFinal(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	b.decide(dec.ID, "sam", decisions.SaysRefuse, "")
	b.clock.Move(1e9)
	r, err := b.d.Choose(ctx, decisions.Ref(dec.ID), requests.Identity("telegram", "4471"), decisions.SaysYes)
	if err != nil {
		t.Fatal(err)
	}
	if r.Taken || r.Text != "Not taken: it was decided already, refused by Sam at 10:00." {
		t.Fatalf("Sam's later yes is answered %+v", r)
	}
	if b.answers("ask-1") != 1 || len(b.verdict("ask-1").Approve) != 0 {
		t.Fatal("the later yes changed the answer")
	}
}

func TestDecisionsFirstOfTwoAnswersStands(t *testing.T) {
	t.Parallel()
	for range 10 {
		b := newBench(t, work(t))
		_, sub := b.helpRequest()
		dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
		var wg sync.WaitGroup
		replies := make([]decisions.Reply, 2)
		for i, who := range []string{"U0LEE000", "U0PRI000"} {
			wg.Go(func() {
				says := []string{decisions.SaysYes, decisions.SaysRefuse}[i]
				r, err := b.d.Choose(ctx, decisions.Ref(dec.ID), requests.Identity("slack", who), says)
				if err != nil {
					t.Error(err)
				}
				replies[i] = r
			})
		}
		wg.Wait()
		if replies[0].Taken == replies[1].Taken {
			t.Fatalf("of two answers together, %+v were taken", replies)
		}
		if got := b.entries(sub, decisions.KindDecided); len(got) != 1 {
			t.Fatalf("the thread holds %d decisions", len(got))
		}
		if b.answers("ask-1") != 1 {
			t.Fatalf("the ask was answered %d times", b.answers("ask-1"))
		}
	}
}

func TestDecisionsPersonCancelsRuleInFlight(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "a new cooker")
	dec := b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	rule := b.runner.next(t)
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	rule.cancelled(t)
	b.d.Close()
	if got := b.get(dec.ID); got.Ended != decisions.Yes || got.By.Name != "sam" || len(got.Passes) != 0 {
		t.Fatalf("the decision is %+v", got)
	}
}

func TestDecisionsOutsideDeciderSeesOnlyTheDecision(t *testing.T) {
	t.Parallel()
	b := newBench(t, withPayments(t, "people: [lee, priya]\nlead: ledger\ndecide: { everything: [lee, priya], refund: [raj] }\n"))
	_, sub := b.helpRequest()
	if _, err := b.reqs.Put(sub, b.reqs.Worker("ledger"), requests.KindQuestion, b.reqs.Person("lee"), "is ch_9 the one with card 4111?", nil); err != nil {
		t.Fatal(err)
	}
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	if !slices.Equal(dec.Asked, []string{"raj"}) {
		t.Fatalf("the refund is asked of %v", dec.Asked)
	}
	seen := b.seen(sub, "raj")
	has(t, seen, "decide: ", "Refund one charge", "ch_9")
	hasNo(t, seen, "card 4111")
	hasNo(t, seen, "after Tuesday's release")
	b.decide(dec.ID, "raj", decisions.SaysYes, "")
	has(t, b.seen(sub, "raj"), "decided: yes from Raj")
}

func TestDecisionsDetailsOnlyByDirectMessage(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	r, err := b.reqs.AskAt("slack", "#help", "C1/300.1", "U0AL0000", "redeploy the api", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.reqs.Readers("slack", "C1/300.1", []string{"U0AL0000", "U0DANA00"}); err != nil {
		t.Fatal(err)
	}
	dec := b.one("ask-1", office.Top, r.Number, []string{"bot"}, decisions.Job{Name: "deploy", Asked: &decisions.Asked{Does: "Deploy again", Args: raw(map[string]any{"secret": "s3"})}})
	ds, err := b.reqs.Deliveries(dec.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Person != "al" || ds[0].Origin != nil {
		t.Fatalf("the decision for Al, who asked in #help, is carried to %+v", ds)
	}
	hasNo(t, b.seen(r.Number, "dana"), "s3")
}

func TestDecisionEntriesAreByTheDeciderToWhoseItIs(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	b.decide(dec.ID, "lee", decisions.SaysYes, "")
	e := b.entries(sub, decisions.KindDecided)[0]
	if e.By.Name != "lee" || len(e.To) != 1 || e.To[0].Name != "ledger" || e.Text != "yes from Lee." {
		t.Fatalf("the decision's entry is %+v", e)
	}
	b.runner.answers(func(s decisions.Start) (string, error, bool) {
		return `{"says":"yes","note":"No account number."}`, nil, true
	})
	id, err := b.d.Disclose(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: b.reqs.Worker("ledger"), Text: "Refunded."})
	if err != nil {
		t.Fatal(err)
	}
	b.let(id)
	es := b.entries(sub, decisions.KindDecided)
	e = es[len(es)-1]
	if e.By.Kind != requests.IsRule || e.By.Name != "payments/no-account-numbers.yaml" || e.To[0].Name != "ledger" {
		t.Fatalf("the disclosure's entry is %+v", e)
	}
}

func TestRequestWaitsOnPeopleAskedAndWorksWithARule(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	if on := b.waitsOn(sub); !slices.Equal(on, []string{"lee", "priya"}) {
		t.Fatalf("payments' sub-request waits on %v", on)
	}
	h := newBench(t, house(t))
	n := h.chat("kit", office.Top, "a new cooker")
	h.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	h.runner.next(t)
	if st, err := h.reqs.State(n); err != nil || st.Is != requests.Working {
		t.Fatalf("while the rule runs the request is %+v, %v", st, err)
	}
}

func TestWhatWasDoneHoldsTheDecision(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	top, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	dana := b.seen(sub, "dana")
	has(t, dana, "decision: Refund one charge: waiting on Lee, Priya")
	for _, word := range []string{"ch_9", "duplicate of ch_8"} {
		hasNo(t, dana, word)
	}
	b.clock.Move(2 * 60e9)
	b.decide(dec.ID, "lee", decisions.SaysYes, "seen it before")
	dana = b.seen(sub, "dana")
	has(t, dana, "decision: Refund one charge: yes from Lee, 10:02")
	for _, word := range []string{"ch_9", "duplicate of ch_8", "seen it before"} {
		hasNo(t, dana, word)
	}
	lee := b.seen(sub, "lee")
	for _, word := range []string{"ch_9", "duplicate of ch_8", "seen it before", "Refund one charge: yes from Lee, 10:02"} {
		has(t, lee, word)
	}
	if v, err := b.reqs.View(top, "dana"); err != nil || len(v.Subs) != 1 {
		t.Fatalf("Dana's request is %+v, %v", v, err)
	}
}

func TestPersonShownTheHeldAction(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	text := b.waitingFor("lee")[0].Text
	for _, want := range []string{"Payments", "ledger", "Charge ch_9 is a duplicate of ch_8", "Refund one charge", `"charge":"ch_9"`,
		"asked by bot, of office: was 4411 charged twice?", "Asked: Lee, Priya.", "Answer yes or refuse"} {
		if !strings.Contains(text, want) {
			t.Errorf("Lee is not shown %q: %s", want, text)
		}
	}
	if strings.Contains(text, "after Tuesday's release") {
		t.Errorf("Lee is shown Dana's request, the office's: %s", text)
	}
	h := newBench(t, house(t))
	h.runner.answers(smallOrders)
	n := h.chat("kit", office.Top, "a new cooker")
	h.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	h.until("Sam is asked", func() bool { return len(h.waitingFor("sam")) == 1 })
	if text := h.waitingFor("sam")[0].Text; !strings.Contains(text, "small-orders.js passed it on: £300 is more than I order alone.") {
		t.Fatalf("Sam is not shown the rule's pass: %s", text)
	}
}
