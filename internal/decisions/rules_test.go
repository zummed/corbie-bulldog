package decisions_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
)

func TestRuleStartedWithTheDecision(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "socket", 12))
	c := b.runner.next(t)
	want := map[string]string{"team": office.Top, "request": fmt.Sprint(n), "kind": "decide", "decision": fmt.Sprint(dec.ID), "step": "1"}
	if c.Kind != office.IsRule || c.Name != "small-orders.js" || c.Team != office.Top || fmt.Sprint(c.Labels) != fmt.Sprint(want) {
		t.Fatalf("the rule is started as %s %s of %s, labelled %v", c.Kind, c.Name, c.Team, c.Labels)
	}
	in := c.Input
	action := in["action"].(map[string]any)
	request := in["request"].(map[string]any)
	if in["kind"] != decisions.OfAction || in["team"] != office.Top || request["number"] != n || request["asker"] != "Kit" ||
		action["name"] != "order_part" || action["does"] != "Order a part" || action["args"].(map[string]any)["pounds"] != 12.0 ||
		in["said"] != "A new socket is needed." || len(in["passes"].([]map[string]any)) != 0 || len(in["pass_goes_to"].([]map[string]any)) != 1 {
		t.Fatalf("the rule is given %v", in)
	}
	c.says(decisions.SaysYes, "Under £20.")
	b.ended(dec.ID)
	has(t, b.thread(n), "decided by small-orders.js", "yes from small-orders.js: Under £20.")

	w := newBench(t, work(t))
	_, sub := w.helpRequest()
	id, err := w.d.Release(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: w.reqs.Worker("ledger"), Text: "Duplicate confirmed; £40 refunded."})
	if err != nil {
		t.Fatal(err)
	}
	r := w.runner.next(t)
	if r.Name != "payments/no-account-numbers.yaml" || r.Labels["team"] != "payments" || r.Labels["kind"] != "decide" || r.Input["kind"] != decisions.OfRelease ||
		r.Input["text"] != "Duplicate confirmed; £40 refunded." {
		t.Fatalf("the release rule is started as %s, labelled %v, given %v", r.Name, r.Labels, r.Input)
	}
	r.says(decisions.SaysYes, "No account number.")
	if l := w.let(id); !l.Released || l.By != "no-account-numbers.yaml" {
		t.Fatalf("the release ended %+v", l)
	}
}

func TestRuleResultReadByItsThreshold(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	for _, c := range []struct {
		leaks float64
		yes   bool
	}{{0.01, true}, {0.9, false}} {
		b.runner.answers(func(s decisions.Start) (string, error, bool) {
			if c.leaks < 0.05 {
				return `{"says":"yes","note":"No account number."}`, nil, true
			}
			return fmt.Sprintf(`{"says":"pass","note":"It may name an account number, %.2f."}`, c.leaks), nil, true
		})
		id, err := b.d.Release(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: b.reqs.Worker("ledger"), Text: "Refunded."})
		if err != nil {
			t.Fatal(err)
		}
		if c.yes {
			if l := b.let(id); !l.Released {
				t.Fatalf("at %v the answer was not let go", c.leaks)
			}
			continue
		}
		b.until("Lee and Priya are asked", func() bool { return len(b.waitingFor("lee")) == 1 })
		if !strings.Contains(b.waitingFor("priya")[0].Text, "It may name an account number, 0.90.") {
			t.Fatalf("Priya is not shown the rule's note: %s", b.waitingFor("priya")[0].Text)
		}
	}
}

func TestDecisionsFailedDeciderPasses(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [./a.js, sam]\n  order_part: [reviewer, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	cases := []struct {
		end  func(c *call)
		note string
	}{
		{func(c *call) { c.out <- reply{err: errors.New("Error: thrown at a.js:1")} }, "it failed: Error: thrown at a.js:1"},
		{func(c *call) { b.due(); b.clock.Move(decisions.RuleLimit); b.due(); b.due() }, "it did not end within 15 minutes of its start"},
		{func(c *call) {
			c.out <- reply{err: fmt.Errorf("%w: there is no file a.js in the office", decisions.ErrNotStarted)}
		}, "it could not be started: there is no file a.js in the office"},
		{func(c *call) { c.out <- reply{out: `{"ok":true}`} }, `its answer was not a decision, which is { says: yes, refuse or pass, note }: {"ok":true}`},
	}
	for i, c := range cases {
		run := fmt.Sprint("ask-", i)
		dec := b.one(run, office.Top, n, fixerUp, cutPower("4"))
		call := b.runner.next(t)
		c.end(call)
		b.until("the walk reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
		if got := b.get(dec.ID).Passes; len(got) != 1 || !strings.HasPrefix(got[0].Note, c.note) {
			t.Fatalf("case %d passes %+v", i, got)
		}
		has(t, b.thread(n), "pass from a.js: "+c.note)
	}
	dec := b.one("ask-worker", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	worker := b.runner.next(t)
	b.clock.Move(decisions.RuleLimit)
	b.due()
	if got := b.get(dec.ID); got.At == nil || got.At.Name != "reviewer" {
		t.Fatalf("the reviewer was given up on after 15 minutes: %+v", got)
	}
	b.clock.Move(decisions.WorkerLimit)
	b.due()
	worker.cancelled(t)
	b.until("the walk reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	if got := b.get(dec.ID).Passes; !strings.Contains(got[0].Note, "within an hour") {
		t.Fatalf("the worker passes %+v", got)
	}
}

// due passes what has run too long, as work does each time its clock moves.
func (b *bench) due() {
	b.t.Helper()
	if err := b.d.Due(ctx); err != nil {
		b.t.Fatal(err)
	}
}

func TestWorkerDecidesFromItsReply(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("6"))
	r := b.runner.next(t)
	if r.Kind != office.IsWorker || r.Name != "reviewer" || !strings.Contains(r.Said, "Turn one circuit off") || r.Labels["kind"] != "decide" {
		t.Fatalf("the reviewer is started as %s %s, told %q, labelled %v", r.Kind, r.Name, r.Said, r.Labels)
	}
	r.out <- reply{out: jsonOf(`{ "says": "refuse", "note": "That circuit feeds the freezer." }`)}
	if got := b.ended(dec.ID); got.Ended != decisions.Refused || got.By.Note != "That circuit feeds the freezer." {
		t.Fatalf("the decision is %+v, by %+v", got, got.By)
	}
	free := b.one("ask-2", office.Top, n, fixerUp, cutPower("4"))
	b.runner.next(t).out <- reply{out: jsonOf("I think it is fine.")}
	b.until("the walk reaches Sam", func() bool { return len(b.get(free.ID).Asked) == 1 })
	has(t, b.thread(n), "pass from reviewer: its answer was not a decision")

	// A long reply is cut between letters, never inside one.
	long := b.one("ask-3", office.Top, n, fixerUp, cutPower("5"))
	b.runner.next(t).out <- reply{out: jsonOf(strings.Repeat("Über ", 100))}
	b.until("the walk reaches Sam", func() bool { return len(b.get(long.ID).Asked) == 1 })
	if note := b.get(long.ID).Passes[0].Note; !utf8.ValidString(note) || !strings.HasSuffix(note, "…") {
		t.Fatalf("the reply is cut as %q", note)
	}

	w := newBench(t, withPayments(t, "people: [lee, priya]\nlead: ledger\nrelease: [ops, lee, priya]\n"))
	_, sub := w.helpRequest()
	if _, err := w.d.Release(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: w.reqs.Worker("ledger"), Text: "Refunded."}); err != nil {
		t.Fatal(err)
	}
	ops := w.runner.next(t)
	if ops.Name != "ops" || ops.Team != "payments" || ops.Labels["team"] != "payments" {
		t.Fatalf("ops is started as %s of %s, labelled %v", ops.Name, ops.Team, ops.Labels)
	}
}

func TestDecisionsRuleGetsNoGrant(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "a new cooker")
	dec := b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	c := b.runner.next(t)
	if c.Kind != office.IsRule || c.Name != "small-orders.js" || !b.o.Rules["small-orders.js"].Script {
		t.Fatalf("the rule is started as %s %s", c.Kind, c.Name)
	}
	for key := range c.Input {
		if strings.Contains(key, "allow") || strings.Contains(key, "permission") {
			t.Fatalf("the rule is given %s", key)
		}
	}
	c.out <- reply{err: errors.New("the job was denied: net:shop.example is not allowed; add net:shop.example to the job's allow")}
	b.until("the walk reaches Sam", func() bool { return len(b.get(dec.ID).Asked) == 1 })
	has(t, b.thread(n), "pass from small-orders.js: it failed: the job was denied: net:shop.example is not allowed")
}

func TestDecisionsWorkerDeciderRunBelongsToTeamDecidedFor(t *testing.T) {
	t.Parallel()
	b := newBench(t, withPayments(t, "people: [lee, priya]\nlead: ledger\ndecide: { everything: [ops, lee, priya] }\n"))
	_, sub := b.helpRequest()
	b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	c := b.runner.next(t)
	if c.Name != "ops" || c.Team != "payments" || c.Labels["team"] != "payments" || c.Labels["request"] != fmt.Sprint(sub) {
		t.Fatalf("ops decides as %s, labelled %v", c.Team, c.Labels)
	}
	for person, may := range map[string]bool{"lee": true, "priya": true, "al": false} {
		got, err := b.reqs.MayWatch(c.Labels, person)
		if err != nil {
			t.Fatal(err)
		}
		if got != may {
			t.Errorf("%s may watch ops deciding for payments: %v", person, got)
		}
	}
	if c.Input["team"] != "payments" || c.Input["notes"] != nil {
		t.Fatalf("ops is given %v", c.Input)
	}
}

func TestDecisionsDeciderResultTakenOnceFromItsRun(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "a new cooker")
	dec := b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	rule := b.runner.next(t)
	b.decide(dec.ID, "sam", decisions.SaysRefuse, "too dear")
	rule.says(decisions.SaysYes, "fine")
	b.d.Close()
	if got := b.get(dec.ID); got.Ended != decisions.Refused || got.By.Name != "sam" {
		t.Fatalf("a rule's result came after Sam decided, and the decision is %+v", got)
	}

	b = newBench(t, house(t))
	n = b.chat("kit", office.Top, "a new cooker")
	dec = b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	rule = b.runner.next(t)
	b.runner.endsWhileAway(rule, decisions.SaysPass, "£300 is more than I order alone.")
	rule.says(decisions.SaysPass, "£300 is more than I order alone.")
	b.until("the pass is taken", func() bool { return len(b.get(dec.ID).Passes) == 1 })
	b.restart()
	b.d.Close()
	if got := b.get(dec.ID); len(got.Passes) != 1 || !slices.Equal(got.Asked, []string{"sam"}) {
		t.Fatalf("the rule's one pass was taken %d times: %+v", len(got.Passes), got)
	}
	if got := b.entries(n, decisions.KindPassed); len(got) != 1 {
		t.Fatalf("the thread holds %d passes", len(got))
	}
}
