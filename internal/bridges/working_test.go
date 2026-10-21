package bridges_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// marking gives the work office with a stand-in Slack whose out machine
// takes working, as the shipped one's does, and a stand-in GitLab whose
// out machine does not.
func marking(t *testing.T) map[string]string {
	files := standin.Instead(testkit.Files(t, examples.Work()), "slack", standin.Marking(standin.Folder(true, true)))
	return standin.Instead(files, "gitlab", standin.Folder(false, false))
}

// marked gives what each out run of a list carries, as the mark it is,
// at its place, or as text.
func marked(starts []bridges.Start) []string {
	var out []string
	for _, s := range starts {
		if w, ok := s.Input["working"].(string); ok {
			out = append(out, w+" at "+s.Input["to"].(string))
		} else {
			out = append(out, textIn(s))
		}
	}
	return out
}

func TestBridgesWorkingMarkedAtTheOrigin(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	// The lead starts on what Dana asked, and the bridge is told the team
	// works on it, once.
	must(t, d.Working(n))
	must(t, d.Working(n))
	out := d.carry()
	if len(out) != 1 {
		t.Fatalf("the out runs are %v", marked(out))
	}
	s := out[0]
	if s.Input["to"] != "C1/1.0" || s.Input["working"] != "working" || s.Input["answers"] != nil || textIn(s) != "" ||
		s.Input["direct"] != false || s.Labels["request"] != "1" || s.Labels["kind"] != bridges.KindOut {
		t.Errorf("the mark is given %#v, labelled %v", s.Input, s.Labels)
	}
	d.ended(s, map[string]any{"sent": "C1/1.0"})
	// The lead asks Dana, in her thread: the question goes, and then the
	// thread waits on her; her answer there has the team working again.
	_, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("dana"), "which card?", nil)
	must(t, err)
	if got := marked(d.drain()); !slices.Equal(got, []string{"which card?", "waiting at C1/1.0"}) {
		t.Errorf("after the question, what is carried is %v", got)
	}
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "the visa"))
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0"}) {
		t.Errorf("after Dana's answer, what is carried is %v", got)
	}
	// A decision put to Al, directly, has the thread wait too, until it is
	// decided; what is said more meanwhile changes nothing.
	decision := d.decide(n, "refund £40?", "al")
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "any news?"))
	all := d.drain()
	if got := marked(to(all, "slack", "C1/1.0")); !slices.Equal(got, []string{"waiting at C1/1.0"}) {
		t.Errorf("after the decision, what is carried to the thread is %v", got)
	}
	if got := marked(to(all, "slack", "U0AL0000")); !slices.Equal(got, []string{"refund £40?"}) {
		t.Errorf("after the decision, what is carried to Al is %v", got)
	}
	must(t, d.requests.Rewait(decision.ID, nil))
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0"}) {
		t.Errorf("once decided, what is carried is %v", got)
	}
	// The lead answers and its run ends: the answer goes, and then the
	// team is done there, once.
	d.answer(n, "bot", "Duplicate confirmed; £40 refunded.")
	must(t, d.Worked(n))
	must(t, d.Worked(n))
	if got := marked(d.drain()); !slices.Equal(got, []string{"Duplicate confirmed; £40 refunded.", "done at C1/1.0"}) {
		t.Errorf("after the run, what is carried is %v", got)
	}
	// What is said more starts the next run, which is marked the same.
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "and the second card?"))
	must(t, d.Working(n))
	must(t, d.Worked(n))
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0", "done at C1/1.0"}) {
		t.Errorf("the second run's marks are %v", got)
	}
	// Marks are shown on no request: what is carried of it is the
	// question, the decision, on the chat and to Al, and the answer.
	if got := d.carried(n); len(got) != 4 {
		t.Errorf("of Dana's request, what is carried is %v", got)
	}
	// A bridge whose out machine does not take working is never told.
	d.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	issue := d.latest()
	must(t, d.Working(issue))
	must(t, d.Worked(issue))
	if got := d.carry(); len(got) != 0 {
		t.Errorf("GitLab is told %v", marked(got))
	}
}

func TestBridgesMarkNeverHoldsUpAnAnswer(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	must(t, d.Working(n))
	// The mark fails: it is not tried again, and the answer goes next.
	out := d.carry()
	if len(out) != 1 || out[0].Input["working"] != "working" {
		t.Fatalf("the first out run is %v", marked(out))
	}
	d.failed(out[0], "the job failed: 502")
	d.answer(n, "bot", "Duplicate confirmed.")
	must(t, d.Worked(n))
	out = d.carry()
	if got := marked(out); len(got) != 1 || got[0] != "Duplicate confirmed." {
		t.Fatalf("after the mark failed, what is carried is %v", got)
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	// The failed run may have made its mark all the same, so the team is
	// done there after the answer. That is refused, and given up: the
	// bridge is failing with the service's words, and the answer is
	// delivered.
	out = d.carry()
	if len(out) != 1 || out[0].Input["working"] != "done" {
		t.Fatalf("the last out run is %v", marked(out))
	}
	d.ended(out[0], map[string]any{"refused": "Slack refused: missing_scope"})
	if again := d.carry(); len(again) != 0 {
		t.Errorf("a given up mark is carried again: %v", marked(again))
	}
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("what is carried of the request is %v", got)
	}
	troubles, err := d.Troubles()
	must(t, err)
	if len(troubles) != 1 || troubles[0].Bridge != "slack" || !strings.Contains(troubles[0].Error, "Slack refused: missing_scope") || troubles[0].Failures != 2 {
		t.Errorf("the bridges failing are %+v", troubles)
	}
	// Once a mark goes, the bridge is no longer failing.
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "thanks"))
	must(t, d.Working(n))
	for _, s := range d.carry() {
		d.ended(s, map[string]any{"sent": "C1/1.0"})
	}
	if troubles, _ := d.Troubles(); len(troubles) != 0 {
		t.Errorf("once a mark went, the bridges failing are %+v", troubles)
	}
}
