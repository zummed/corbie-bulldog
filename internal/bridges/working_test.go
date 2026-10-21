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

// starts has a run of a request's lead start, as Work tells the bridges
// once its start is sent.
func (d *desk) starts(n int64) {
	d.t.Helper()
	d.going[n] = true
	must(d.t, d.Working(n))
}

// ends has the run of a request's lead end, as Work tells whoever watches
// the request once its ending is kept.
func (d *desk) ends(n int64) {
	d.going[n] = false
	d.requests.Tell(n)
}

// marks gives each mark kept of a request, as its text and its state.
func (d *desk) marks(n int64) []string {
	d.t.Helper()
	rows, err := d.db.Query(`SELECT text, state FROM bridges_things WHERE request = ? AND kind = 'mark' ORDER BY id`, n)
	must(d.t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text, state string
		must(d.t, rows.Scan(&text, &state))
		out = append(out, text+" "+state)
	}
	must(d.t, rows.Err())
	return out
}

func TestBridgesWorkingMarkedAtTheOrigin(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	// The lead starts on what Dana asked, and the bridge is told the team
	// works on it, once.
	d.starts(n)
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
	// The lead answers: the answer goes, and once its run has ended, the
	// team is done there, once.
	d.answer(n, "bot", "Duplicate confirmed; £40 refunded.")
	if got := marked(d.drain()); !slices.Equal(got, []string{"Duplicate confirmed; £40 refunded."}) {
		t.Errorf("after the answer, while the run goes, what is carried is %v", got)
	}
	d.ends(n)
	d.ends(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"done at C1/1.0"}) {
		t.Errorf("after the run, what is carried is %v", got)
	}
	// What is said more starts the next run, which is marked the same.
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "and the second card?"))
	d.starts(n)
	d.answer(n, "bot", "Charged once.")
	d.ends(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0", "Charged once.", "done at C1/1.0"}) {
		t.Errorf("the second run carries %v", got)
	}
	// Marks are shown on no request: what is carried of it is the
	// question, the decision, on the chat and to Al, and the answers.
	if got := d.carried(n); len(got) != 5 {
		t.Errorf("of Dana's request, what is carried is %v", got)
	}
	// A bridge whose out machine does not take working is never told.
	d.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	issue := d.latest()
	d.starts(issue)
	d.ends(issue)
	if got := d.carry(); len(got) != 0 {
		t.Errorf("GitLab is told %v", marked(got))
	}
}

func TestBridgesDoneOnceAnsweredWithNoRunGoing(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	d.starts(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0"}) {
		t.Fatalf("as the lead starts, what is carried is %v", got)
	}
	// Dana says more while the lead works, and its answer then reads as
	// the request's: the next run, on what she said, starts on a request
	// answered, and the team is done there only once that run has ended.
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "and the second card?"))
	d.answer(n, "bot", "Duplicate confirmed.")
	d.ends(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"Duplicate confirmed.", "done at C1/1.0"}) {
		t.Errorf("after the first run, what is carried is %v", got)
	}
	d.starts(n)
	d.requests.Tell(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0"}) {
		t.Errorf("while the next run goes, what is carried is %v", got)
	}
	d.answer(n, "bot", "The second was charged once.")
	if got := marked(d.drain()); !slices.Equal(got, []string{"The second was charged once."}) {
		t.Errorf("after the next answer, while its run goes, what is carried is %v", got)
	}
	// Bulldog stops between keeping that the run ended and anything
	// looking at the request: once started again, the team is done there.
	d.going[n] = false
	d.restart()
	if got := marked(d.drain()); !slices.Equal(got, []string{"done at C1/1.0"}) {
		t.Errorf("after a restart, what is carried is %v", got)
	}
	d.restart()
	if got := d.drain(); len(got) != 0 {
		t.Errorf("after another restart, what is carried is %v", marked(got))
	}
}

func TestBridgesMarkNeverHoldsUpAnAnswer(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	d.starts(n)
	// The mark fails: it is not tried again, and the answer goes next.
	out := d.carry()
	if len(out) != 1 || out[0].Input["working"] != "working" {
		t.Fatalf("the first out run is %v", marked(out))
	}
	d.failed(out[0], "the job failed: 502")
	d.answer(n, "bot", "Duplicate confirmed.")
	d.ends(n)
	out = d.carry()
	if got := marked(out); len(got) != 1 || got[0] != "Duplicate confirmed." {
		t.Fatalf("after the mark failed, what is carried is %v", got)
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	// The failed run may have made its mark all the same, so the team is
	// done there after the answer. That fails too: it is given up, and
	// kept again when the request is next looked at.
	out = d.carry()
	if len(out) != 1 || out[0].Input["working"] != "done" {
		t.Fatalf("the next out run is %v", marked(out))
	}
	d.failed(out[0], "Slack said ratelimited")
	if again := d.carry(); len(again) != 0 {
		t.Errorf("a given up mark is carried again at once: %v", marked(again))
	}
	d.requests.Tell(n)
	out = d.carry()
	if len(out) != 1 || out[0].Input["working"] != "done" {
		t.Fatalf("once the request is looked at again, the out run is %v", marked(out))
	}
	// That one is refused, and given up: the bridge is failing with the
	// service's words, and the answer is delivered.
	d.ended(out[0], map[string]any{"refused": "Slack refused: missing_scope"})
	if again := d.carry(); len(again) != 0 {
		t.Errorf("a given up mark is carried again: %v", marked(again))
	}
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("what is carried of the request is %v", got)
	}
	troubles, err := d.Troubles()
	must(t, err)
	if len(troubles) != 1 || troubles[0].Bridge != "slack" || !strings.Contains(troubles[0].Error, "Slack refused: missing_scope") || troubles[0].Failures != 3 {
		t.Errorf("the bridges failing are %+v", troubles)
	}
	// Once a mark goes, the bridge is no longer failing.
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "thanks"))
	d.starts(n)
	for _, s := range d.carry() {
		d.ended(s, map[string]any{"sent": "C1/1.0"})
	}
	if troubles, _ := d.Troubles(); len(troubles) != 0 {
		t.Errorf("once a mark went, the bridges failing are %+v", troubles)
	}

	// A mark for a bridge stopped by a problem is let go, and is no
	// failure of it; once the bridge is mended, the thread is told where
	// the request stands.
	broken := replace(t, marking(t), "office.yaml", `"#help": { guests: true }`, `"#help": { team: billing }`)
	s := open(t, marking(t))
	m := s.ask("C1/1.0", "customer 4411 was charged twice")
	s.starts(m)
	if got := marked(s.drain()); !slices.Equal(got, []string{"working at C1/1.0"}) {
		t.Fatalf("as the lead starts, what is carried is %v", got)
	}
	q, err := s.requests.Put(m, s.requests.Worker("bot"), requests.KindQuestion, s.requests.Person("dana"), "which card?", nil)
	must(t, err)
	s.read(broken)
	if got := s.drain(); len(got) != 0 {
		t.Errorf("a stopped bridge carries %v", marked(got))
	}
	_, err = s.requests.Reply(q.ID, s.requests.Person("dana"), "the visa", nil, requests.Screens)
	must(t, err)
	s.answer(m, "bot", "Duplicate confirmed.")
	s.ends(m)
	if got := s.drain(); len(got) != 0 {
		t.Errorf("a stopped bridge carries %v", marked(got))
	}
	if got := s.marks(m); !slices.Equal(got, []string{"working sent", "waiting withdrawn"}) {
		t.Errorf("while the bridge is stopped, the marks are %v", got)
	}
	if troubles, _ := s.Troubles(); len(troubles) != 0 {
		t.Errorf("while the bridge is stopped, the bridges failing are %+v", troubles)
	}
	s.read(marking(t))
	if got := marked(s.drain()); !slices.Equal(got, []string{"done at C1/1.0"}) {
		t.Errorf("once the bridge is mended, what is carried is %v", got)
	}
}

func TestBridgesDisclosureShownAsWorking(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	d.starts(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"working at C1/1.0"}) {
		t.Fatalf("as the lead starts, what is carried is %v", got)
	}
	// The lead's answer waits on Lee, for its team's disclosure check, once
	// its run has ended: Dana waits on the team, which is still at work.
	disclosure := d.disclose(n, "Duplicate confirmed on card 4111?", "lee")
	d.ends(n)
	if got := marked(d.drain()); !slices.Equal(got, []string{"Duplicate confirmed on card 4111?"}) {
		t.Errorf("while the disclosure waits, what is carried is %v", got)
	}
	must(t, d.requests.Rewait(disclosure.ID, nil))
	d.answer(n, "bot", "Duplicate confirmed.")
	if got := marked(d.drain()); !slices.Equal(got, []string{"Duplicate confirmed.", "done at C1/1.0"}) {
		t.Errorf("once disclosed, what is carried is %v", got)
	}
	// A decision put to someone who has since left the office still waits
	// on a person.
	d.delivered("slack", said("U0RAJ000", "#help", "C2/1.0", "the webhook retries twice"))
	r := d.latest()
	d.starts(r)
	d.decide(r, "restart the webhook?", "raj")
	var got []string
	for out := d.carry(); len(out) > 0; out = d.carry() {
		for _, s := range out {
			got = append(got, marked([]bridges.Start{s})...)
			if s.Input["working"] == "waiting" {
				d.failed(s, "the job failed: 502")
			} else {
				d.ended(s, map[string]any{"sent": "s"})
			}
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"restart the webhook?", "waiting at C2/1.0", "working at C2/1.0"}) {
		t.Errorf("as Raj is asked, what is carried is %v", got)
	}
	d.read(replace(t, marking(t), "office.yaml", `  raj:   { name: Raj, title: Backend., slack: U0RAJ000, gitlab: "1187", entra: raj@platform.example }`+"\n", ""))
	d.requests.Tell(r)
	if got := marked(to(d.drain(), "slack", "C2/1.0")); !slices.Equal(got, []string{"waiting at C2/1.0"}) {
		t.Errorf("once Raj has left, the thread is carried %v", got)
	}
	st, err := d.requests.State(r)
	must(t, err)
	if len(st.On) != 1 || st.On[0].Kind != requests.IsGone {
		t.Errorf("the request waits on %+v", st.On)
	}
}

func TestBridgesOlderMarksLetGo(t *testing.T) {
	t.Parallel()
	d := open(t, marking(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	// Marks of a message, as an older Bulldog kept them, still to be
	// carried when it is upgraded, are let go, and nothing is carried.
	for _, kind := range []string{"working", "not working"} {
		_, err := d.db.Exec(`INSERT INTO bridges_things(bridge, place, kind, request, entry, state, made) VALUES ('slack', 'C1/1.0', ?, ?, 1, 'waiting', '2026-10-20T09:00:00Z')`, kind, n)
		must(t, err)
	}
	if got := d.drain(); len(got) != 0 {
		t.Errorf("what is carried is %v", marked(got))
	}
	var left int
	must(t, d.db.QueryRow(`SELECT count(*) FROM bridges_things WHERE kind IN ('working', 'not working') AND state = 'withdrawn'`).Scan(&left))
	if left != 2 {
		t.Errorf("%d of the older marks are let go", left)
	}
	if troubles, _ := d.Troubles(); len(troubles) != 0 {
		t.Errorf("the bridges failing are %+v", troubles)
	}
}
