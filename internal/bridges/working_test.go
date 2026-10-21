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

// ids gives the numbers of a request's entries.
func (d *desk) ids(n int64) []int64 {
	d.t.Helper()
	es, err := d.requests.Thread(n)
	must(d.t, err)
	var out []int64
	for _, e := range es {
		out = append(out, e.ID)
	}
	return out
}

// marked gives what each out run of a list carries, as working true or
// false on what it answers, or as text.
func marked(starts []bridges.Start) []string {
	var out []string
	for _, s := range starts {
		if w, ok := s.Input["working"].(bool); ok {
			out = append(out, map[bool]string{true: "working on ", false: "done with "}[w]+s.Input["answers"].(string))
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
	first := d.ids(n)
	// The lead starts on what Dana asked, and the bridge is told it works
	// on it.
	must(t, d.Working(n, first))
	out := d.carry()
	if len(out) != 1 {
		t.Fatalf("the out runs are %v", marked(out))
	}
	s := out[0]
	if s.Input["to"] != "C1/1.0" || s.Input["working"] != true || !strings.HasPrefix(s.Input["answers"].(string), "m") || textIn(s) != "" ||
		s.Input["direct"] != false || s.Labels["request"] != "1" || s.Labels["kind"] != bridges.KindOut {
		t.Errorf("the mark is given %#v, labelled %v", s.Input, s.Labels)
	}
	d.ended(s, map[string]any{"sent": s.Input["answers"]})
	// The lead answers and its run ends: the answer goes, and then the
	// mark comes off.
	d.answer(n, "bot", "Duplicate confirmed; £40 refunded.")
	must(t, d.Worked(n))
	if got := marked(d.drain()); !slices.Equal(got, []string{"Duplicate confirmed; £40 refunded.", "done with " + s.Input["answers"].(string)}) {
		t.Errorf("after the run, what is carried is %v", got)
	}
	// What is said more in the thread is marked on its own, when a run
	// starts on it; what is said on the screens is no message there.
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "and the second card?"))
	_, err := d.requests.Say(n, d.requests.Person("al"), "it was the visa", nil, requests.Screens, false)
	must(t, err)
	all := d.ids(n)
	must(t, d.Working(n, all[len(first)+1:]))
	must(t, d.Worked(n))
	if got := marked(d.drain()); len(got) != 2 || got[0] != "working on "+strings.TrimPrefix(got[1], "done with ") || got[0] == "working on "+s.Input["answers"].(string) {
		t.Errorf("the second run's marks are %v", got)
	}
	// Marks are shown on no request.
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("of Dana's request, what is carried is %v", got)
	}
	// A bridge whose out machine does not take working is never told.
	d.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	issue := d.latest()
	must(t, d.Working(issue, d.ids(issue)))
	must(t, d.Worked(issue))
	if got := d.carry(); len(got) != 0 {
		t.Errorf("GitLab is told %v", marked(got))
	}
}

func TestBridgesMarkNeverHoldsUpAnAnswer(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	must(t, d.Working(n, d.ids(n)))
	d.answer(n, "bot", "Duplicate confirmed.")
	must(t, d.Worked(n))
	// The mark fails: it is not tried again, and the answer goes next.
	out := d.carry()
	if len(out) != 1 || out[0].Input["working"] != true {
		t.Fatalf("the first out run is %v", marked(out))
	}
	d.failed(out[0], "the job failed: 502")
	out = d.carry()
	if got := marked(out); len(got) != 1 || got[0] != "Duplicate confirmed." {
		t.Fatalf("after the mark failed, what is carried is %v", got)
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	// The mark that comes off is refused: it is given up, the bridge is
	// failing with the service's words, and the answer is delivered.
	out = d.carry()
	if len(out) != 1 || out[0].Input["working"] != false {
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
	all := d.ids(n)
	must(t, d.Working(n, all[len(all)-1:]))
	for _, s := range d.carry() {
		d.ended(s, map[string]any{"sent": s.Input["answers"]})
	}
	if troubles, _ := d.Troubles(); len(troubles) != 0 {
		t.Errorf("once a mark went, the bridges failing are %+v", troubles)
	}
}
