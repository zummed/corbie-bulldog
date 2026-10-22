package requests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
)

// edit changes a file of the office and reads the office again.
func (o *driven) edit(file, old, new string) *Reading {
	o.t.Helper()
	text, err := os.ReadFile(filepath.Join(o.office.Dir, file))
	if err != nil {
		o.t.Fatal(err)
	}
	return o.read(func(files map[string]string) { files[file] = strings.Replace(string(text), old, new, 1) })
}

// clocks gives the office's clocks that run.
func (o *driven) clocks() []*client.Run {
	o.t.Helper()
	runs, err := o.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": o.office.ID, "kind": "clock"}})
	if err != nil {
		o.t.Fatal(err)
	}
	return runs
}

// clocked stays until the office's one running clock is not the one
// given, and gives it.
func (o *driven) clocked(not string) *client.Run {
	o.t.Helper()
	var clock *client.Run
	o.corbie.Until(o.t, "one clock runs, other than "+not, func() bool {
		clocks := o.clocks()
		if len(clocks) != 1 || clocks[0].ID == not {
			return false
		}
		clock = clocks[0]
		return true
	})
	return clock
}

// routines gives the requests of the office's routines, oldest first.
func (o *driven) routines() []string {
	o.t.Helper()
	var runs []string
	for _, r := range o.requests("office") {
		if _, _, request := o.given(r.ID); request["heard"] == "routine" {
			runs = append(runs, r.ID)
		}
	}
	return runs
}

// routinely is the office's bot, which says what is open when its routine
// asks, and otherwise asks the person its prompt names.
func routinely(turn testkit.Turn) testkit.Reply {
	if turn.Prompt() == "What is still open?" {
		return testkit.Say("Only the coffee.")
	}
	return asksPerson(turn)
}

// A routine's clock falls due at its time: one request of the team starts,
// given the team's running requests, and its answer goes to the team's
// people directly. Times that came while Bulldog was stopped give one
// request.
func TestRoutineFallsDueOnce(t *testing.T) {
	t.Parallel()
	o := newOffice(t, routinely, func(files map[string]string) {
		files["office.yaml"] += "routines:\n  - { at: \"mon 09:30\", say: \"What is still open?\" }\n"
	})
	e, stop := o.run()
	clock := o.clocked("")
	if l := clock.Labels; l["team"] != "office" || l["made"] != e.made(e.Reading(), o.office.Team("office"), o.office.Team("office").Routines[0]) {
		t.Errorf("the clock is labelled %v", l)
	}
	if shown, err := o.corbie.Show(ctx, clock.ID); err != nil || fmt.Sprint(shown.Input) != "map[at:mon 09:30 UTC]" {
		t.Errorf("the clock is given %v (%v)", shown.Input, err)
	}
	o.chat(e, "office", "dana: Coffee?")
	o.until(2)

	o.corbie.Clock.Move(30 * time.Minute)
	sent := o.until(4)
	runs := o.routines()
	if len(runs) != 1 {
		t.Fatalf("the routine asked %d times", len(runs))
	}
	said, context, request := o.given(runs[0])
	if said != "What is still open?" || fmt.Sprint(request["by"]) != "map[routine:mon 09:30]" {
		t.Errorf("the routine asked %q, by %v", said, request["by"])
	}
	if open := request["open"].([]any); len(open) != 1 || open[0].(map[string]any)["said"] != "dana: Coffee?" {
		t.Errorf("the routine is given the open requests %v", open)
	}
	if !strings.Contains(context, "Requests of Work still open: 1.") {
		t.Errorf("the routine's context is:\n%s", context)
	}
	for i, to := range []string{"4401", "U0DANA"} {
		if m := sent[2+i]; m.To != to || !m.Direct || m.Text != "Work, on \"What is still open?\":\n\nOnly the coffee." || m.Labels["request"] != runs[0] {
			t.Errorf("the routine's answer is carried as %+v", m)
		}
	}

	// Bulldog is stopped over two Mondays: the clock falls due once, and
	// one request starts when Bulldog starts again.
	o.corbie.Until(t, "the clock waits for its time", func() bool { return !waits(o.clocked(""), "came") })
	stop()
	o.corbie.Clock.Move(15 * 24 * time.Hour)
	o.corbie.Waiting(t, clock.ID, "came")
	o.run()
	o.corbie.Until(t, "the clock waits for its time again", func() bool {
		return len(o.routines()) == 2 && !waits(o.clocked(""), "came")
	})
	if runs := o.routines(); len(runs) != 2 {
		t.Errorf("the routine asked %d times", len(runs))
	}
	if clocks := o.clocks(); len(clocks) != 1 || clocks[0].ID != clock.ID {
		t.Errorf("the clocks are %v", clocks)
	}
}

// A changed routine has its clock cancelled and a new one started; a
// removed routine has its clock cancelled.
func TestRoutineChanged(t *testing.T) {
	t.Parallel()
	o := newOffice(t, routinely, func(files map[string]string) {
		files["office.yaml"] += "routines:\n  - { at: \"mon 09:30\", say: \"What is still open?\" }\n"
	})
	e, _ := o.run()
	monday := o.clocked("")
	e.Swap(o.edit("office.yaml", `"mon 09:30"`, `"tue 09:00"`))
	tuesday := o.clocked(monday.ID)
	if shown := o.ended(monday.ID); shown.Status != "cancelled" {
		t.Errorf("the Monday clock ended %s", shown.Status)
	}
	if tuesday.Labels["made"] == monday.Labels["made"] {
		t.Error("the Tuesday clock has the Monday clock's fingerprint")
	}
	e.Swap(o.edit("office.yaml", "routines:\n  - { at: \"tue 09:00\", say: \"What is still open?\" }\n", ""))
	if shown := o.ended(tuesday.ID); shown.Status != "cancelled" {
		t.Errorf("the Tuesday clock ended %s", shown.Status)
	}
	o.corbie.Until(t, "no clock runs", func() bool { return len(o.clocks()) == 0 })
	if n := len(o.runs("clock")); n != 2 {
		t.Errorf("%d clocks were started", n)
	}
}

// A noted request and one in the chat carry nothing back.
func TestNothingCarriedForNotedOrChat(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e, _ := o.run()
	noted := refund("C01/1700000000.0002")
	noted.Said, noted.Mentioned = "lunch?", false
	run, err := e.Ask(ctx, noted)
	if err != nil {
		t.Fatal(err)
	}
	if out := o.output(run); out["outcome"] != "noted" {
		t.Errorf("the message gave %v", out)
	}
	o.answered(o.chat(e, "payments", "Was order 4411 refunded?"))
	// What the engine does after this request ended comes after both.
	o.answered(o.chat(e, "payments", "Was order 4412 refunded?"))
	marker, err := e.Ask(ctx, refund("C01/1700000000.0003"))
	if err != nil {
		t.Fatal(err)
	}
	if sent := o.until(1); len(sent) != 1 || sent[0].Labels["request"] != marker {
		t.Errorf("the carrier carried %+v", sent)
	}
}

// R-requests-75: Bulldog stopped while payments' check waits on Lee, and
// started again: Lee's answer ends the request once, and the asker is
// answered once.
func TestAfterARestart(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, func(files map[string]string) {
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "check: [./no-account-numbers.js, lee, priya]", "check: [./passes.js, lee]", 1)
		files["payments/passes.js"] = `return { says: "pass", note: "Lee knows." }`
	})
	first, stop := o.run()
	run, err := first.Ask(ctx, refund("C01/1700000000.0002"))
	if err != nil {
		t.Fatal(err)
	}
	asked := o.waitFor("May Payments give this answer?")
	o.until(2)
	stop()
	again, _ := o.run()
	if err := again.Answer(ctx, asked.ID, "lee", "yes", "U0LEE:1"); err != nil {
		t.Fatal(err)
	}
	if out := o.output(run); out["outcome"] != "answered" {
		t.Errorf("the request gave %v", out)
	}
	sent := o.until(3)
	if len(sent) != 3 || sent[2].To != "C01/1700000000.0001" || sent[2].Text != "Refunded £40 on 3 May." {
		t.Errorf("the carrier carried %+v", sent)
	}
	if n := len(o.requests("payments")); n != 1 {
		t.Errorf("payments has %d requests", n)
	}
}
