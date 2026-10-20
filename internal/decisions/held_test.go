package decisions_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// A held job becomes a decision of the team of the worker that holds it,
// whichever ask it reached, showing what the tool does, what it would be
// given and what the model said; nothing is answered until it is decided.
func TestHeldJobIsADecisionOfItsWorkersTeam(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	for _, asking := range [][]string{fixerUp, {"fixer"}} {
		run := "ask-" + asking[len(asking)-1]
		dec := b.one(run, office.Top, n, asking, cutPower("4"))
		if dec.Kind != decisions.OfAction || dec.Team != office.Top || dec.Whose.Name != "fixer" || dec.Does(b.o) != "Turn one circuit off" {
			t.Fatalf("the decision is %+v", dec)
		}
		if dec.Ended != "" || b.answers(run) != 0 {
			t.Fatalf("the job was answered before it was decided: %+v", dec)
		}
		ws := b.waitingFor("sam")
		text := ws[len(ws)-1].Text
		for _, want := range []string{"fixer", "cut_power", "Turn one circuit off", `"circuit":"4"`, "I will cut circuit 4 first.", "the kitchen socket sparked", "Kit"} {
			if !strings.Contains(text, want) {
				t.Errorf("Sam is not shown %q: %s", want, text)
			}
		}
	}
}

// A job held for a permission listed under ask_about reaches the ask as a
// job marked for approval does, and is decided the same way.
func TestAskAboutJobDecidedAsApproval(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "pay the electrician")
	job := decisions.Job{Name: "pay", Input: raw(map[string]any{"pounds": 80}),
		Asked: &decisions.Asked{Does: "Pay someone, at payments.example", Args: raw(map[string]any{"pounds": 80}), Said: "Paying the electrician."}}
	dec := b.one("ask-1", office.Top, n, fixerUp, job)
	if b.answers("ask-1") != 0 {
		t.Fatal("the job was answered before it was decided")
	}
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	if v := b.verdict("ask-1"); !slices.Equal(v.Approve, []string{"pay"}) {
		t.Fatalf("the ask is answered %+v", v)
	}
}

// Several jobs held from one turn are each a decision, and the ask is
// answered once, when the last is decided, naming those approved and
// joining the notes.
func TestSeveralJobsOneAnswer(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	second := cutPower("6")
	second.Name = "cut_power.1"
	decs := b.held(decisions.Ask{Run: "ask-1", Team: office.Top, Request: n, Asking: fixerUp, Jobs: []decisions.Job{cutPower("4"), second}})
	if len(decs) != 2 {
		t.Fatalf("%d decisions wait", len(decs))
	}
	b.decide(decs[0].ID, "sam", decisions.SaysYes, "")
	b.ended(decs[0].ID)
	if b.answers("ask-1") != 0 {
		t.Fatal("a yes on the first answered the ask")
	}
	b.decide(decs[1].ID, "sam", decisions.SaysRefuse, "not today")
	v := b.verdict("ask-1")
	if !slices.Equal(v.Approve, []string{"cut_power"}) || !strings.Contains(v.Note, "not today") || !strings.Contains(v.Note, "Sam") {
		t.Fatalf("the ask is answered %+v", v)
	}
	if b.answers("ask-1") != 1 {
		t.Fatalf("the ask was answered %d times", b.answers("ask-1"))
	}
}

func TestDecisionsAllHeldJobsWrittenBeforeAnyAsked(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	var seen int
	b.runner.answers(func(s decisions.Start) (string, error, bool) {
		decs, err := b.d.OfRequest(n)
		if err != nil {
			t.Error(err)
		}
		seen = len(decs)
		return smallOrders(s)
	})
	decs := b.held(decisions.Ask{Run: "ask-1", Team: office.Top, Request: n, Asking: fixerUp,
		Jobs: []decisions.Job{orderPart("order_part", "socket", 12), cutPower("4")}})
	if got := b.ended(decs[0].ID); got.Ended != decisions.Yes {
		t.Fatalf("the small order ended %+v", got)
	}
	if seen != 2 {
		t.Fatalf("the rule was asked while %d of the 2 jobs were decisions", seen)
	}
	if b.answers("ask-1") != 0 {
		t.Fatal("the rule's yes answered the ask before Sam decided the other job")
	}
	b.decide(decs[1].ID, "sam", decisions.SaysYes, "")
	if v := b.verdict("ask-1"); len(v.Approve) != 2 {
		t.Fatalf("the ask is answered %+v", v)
	}
}

func TestDecisionsOneJobOneDecision(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "two parts")
	b.runner.answers(smallOrders)
	decs := b.held(decisions.Ask{Run: "ask-1", Team: office.Top, Request: n, Asking: fixerUp,
		Jobs: []decisions.Job{orderPart("order_part", "socket", 12), orderPart("order_part.1", "cooker", 300)}})
	if got := b.ended(decs[0].ID); got.Ended != decisions.Yes {
		t.Fatalf("the small order ended %+v", got)
	}
	b.until("the large order reaches Sam", func() bool { return len(b.waitingFor("sam")) == 1 })
	for _, s := range b.runner.started() {
		action := s.Input["action"].(map[string]any)
		if _, ok := action["args"].(map[string]any); !ok || s.Input["jobs"] != nil {
			t.Fatalf("a rule was given more than one job: %v", s.Input)
		}
	}
	if len(b.runner.started()) != 2 {
		t.Fatalf("the rule was started %d times for two jobs", len(b.runner.started()))
	}
	if b.get(decs[1].ID).Ended != "" {
		t.Fatal("the referral on the large order was taken as the decision of both")
	}
}

func TestDecisionsHeldActionBelongsToNearestWorker(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, []string{"in-place-2", "fixer", "steward"}, cutPower("4"))
	if dec.Whose.Name != "fixer" || dec.Team != office.Top {
		t.Fatalf("the action is %s's of %s, not the fixer's of the house", dec.Whose.Name, dec.Team)
	}
	g := b.chat("sam", "garden", "dig the bed")
	dec = b.one("ask-2", "garden", g, []string{"groundsman", "gardener"}, decisions.Job{Name: "dig", Asked: &decisions.Asked{Does: "Dig a bed"}})
	if dec.Whose.Name != "groundsman" || dec.Team != "garden" {
		t.Fatalf("the action is %s's of %s, not the groundsman's of the garden", dec.Whose.Name, dec.Team)
	}
}

func TestDecisionsHeldJobWithoutACallShowsItsInput(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	job := decisions.Job{Name: "cut", Input: raw(map[string]any{"circuit": "4"})}
	dec := b.one("ask-1", office.Top, n, fixerUp, job)
	if dec.Does(b.o) != "cut" {
		t.Fatalf("a job with no call is described as %q", dec.Does(b.o))
	}
	ws := b.waitingFor("sam")
	if !strings.Contains(ws[0].Text, `{"circuit":"4"}`) {
		t.Fatalf("Sam is not shown the job's input: %s", ws[0].Text)
	}
}

// A held job of a team that has gone is refused by Bulldog, saying why,
// and never waits on anyone.
func TestNobodyCanDecideIsRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	g := b.chat("sam", "garden", "open the shed")
	files := map[string]string{}
	for name := range house(t) {
		if strings.HasPrefix(name, "garden/") {
			files[name] = ""
		}
	}
	b.rewrite(files)
	dec := b.one("ask-1", "garden", g, []string{"gardener"}, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the shed door"}})
	if dec.Ended != decisions.Refused || dec.By.Note != "nobody can decide this: garden is no longer in the office" {
		t.Fatalf("the decision is %+v, by %+v", dec, dec.By)
	}
	has(t, b.thread(g), "refused by Bulldog: nobody can decide this: garden is no longer in the office")
	v := b.verdict("ask-1")
	if len(v.Approve) != 0 || !strings.Contains(v.Note, "garden is no longer in the office") {
		t.Fatalf("the ask is answered %+v", v)
	}
}

func TestJobsOfReadsCorbiesAsk(t *testing.T) {
	t.Parallel()
	asking, jobs, err := decisions.JobsOf(json.RawMessage(`{"asking":{"machine":"fixer","instance":"i1"},
		"jobs":[{"name":"cut_power","input":{"circuit":"4"},"allow":[],"asked":{"does":"Turn one circuit off","args":{"circuit":"4"},"said":"now"},"script":"./tools/cut-power.js"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if asking != "fixer" || len(jobs) != 1 || jobs[0].Name != "cut_power" || jobs[0].Asked.Does != "Turn one circuit off" || jobs[0].Asked.Said != "now" {
		t.Fatalf("read %q %+v", asking, jobs)
	}
}
