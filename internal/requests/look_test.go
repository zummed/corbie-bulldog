package requests

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
)

// A request is given whole to the screens: each stage with what its
// deciders said, the waits below it, what the lead read and called, where
// its answer went, and its thread. The bridges find a carried question by
// its ref, by the message that carried it, and as the latest carried to a
// person.
func TestLookGivesTheWholeRequest(t *testing.T) {
	t.Parallel()
	o := newOffice(t, orders, nil)
	e, _ := o.run()
	ask := refund("C01/1700000000.0002")
	ask.Said = "Order a part for £35."
	ask.Files = []File{{Name: "part.txt", Data: client.InputFile{Name: "part.txt", Contents: strings.NewReader("a 13A socket")}}}
	run, err := e.Ask(ctx, ask)
	if err != nil {
		t.Fatal(err)
	}
	asked := o.waitFor("May these run?")
	sent := o.until(2) // to Lee on Telegram and on Slack
	for _, m := range sent {
		o.ended(m.Run)
	}

	waiting, err := e.Waiting(ctx, "lee")
	if err != nil || len(waiting) != 1 || waiting[0].ID != asked.ID || waiting[0].Request != run || waiting[0].Status != "waiting" {
		t.Fatalf("Lee is waited on by %+v (%v)", waiting, err)
	}
	if w := waiting[0]; w.Decision != "decide" || fmt.Sprint(w.Notes) != "[{rule ./payments/small-orders.js pass More than I order alone.}]" ||
		fmt.Sprint(w.Jobs) != "[{order_part Order a part map[pounds:35]}]" {
		t.Errorf("Lee is shown the notes %+v and the jobs %+v", w.Notes, w.Jobs)
	}
	if others, err := e.Waiting(ctx, "dana"); err != nil || len(others) != 0 {
		t.Errorf("Dana is waited on by %+v (%v)", others, err)
	}
	if w, err := e.WaitOf(ctx, sent[0].Ref); err != nil || w == nil || w.ID != asked.ID {
		t.Errorf("the ref names %+v (%v)", w, err)
	}
	if w, err := e.Replied(ctx, "slack", "lee", "U0LEE:2"); err != nil || w == nil || w.ID != asked.ID || w.Request != run {
		t.Errorf("the message sent on Slack is of %+v (%v)", w, err)
	}
	if w, err := e.Replied(ctx, "slack", "priya", "U0LEE:2"); err != nil || w != nil {
		t.Errorf("Priya's message is of %+v (%v)", w, err)
	}
	if w, err := e.Latest(ctx, "telegram", "lee", o.corbie.Clock.Now()); err != nil || w == nil || w.ID != asked.ID || w.Status != "waiting" || w.Request != run {
		t.Errorf("the latest question to Lee is %+v (%v)", w, err)
	}
	if w, err := e.Latest(ctx, "telegram", "lee", o.corbie.Clock.Now().Add(-time.Second)); err != nil || w != nil {
		t.Errorf("the latest question to Lee before it was carried is %+v (%v)", w, err)
	}
	if look, err := e.Look(ctx, run); err != nil || look.Status != "waiting" || states(look.Stages) != "heed:passed work:waiting check:" {
		t.Errorf("the waiting request is %+v (%v)", look, err)
	}

	if err := e.Answer(ctx, asked.ID, "lee", "yes", "U0LEE:7"); err != nil {
		t.Fatal(err)
	}
	answer := o.until(3)[2]
	o.ended(answer.Run)
	look, err := e.Look(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if look.Status != "answered" || *look.Answer != `Told: {"ordered":35}` || look.By.Person != "dana" || look.Heard != "channel" || look.Said != ask.Said ||
		fmt.Sprint(look.Files) != "[{part.txt text/plain 12}]" {
		t.Errorf("the request is %+v", look.Request)
	}
	heed, work, check := look.Stages[0], look.Stages[1], look.Stages[2]
	if heed.State != "passed" || fmt.Sprint(heed.Words) != "[{rule ./.bulldog/rules/mentioned/mentioned.js yes The message names the bot.}]" || heed.Started.IsZero() || heed.Ended.IsZero() {
		t.Errorf("the heed is %+v", heed)
	}
	if work.State != "passed" || work.Words != nil {
		t.Errorf("the work is %+v", work)
	}
	if check.State != "passed" || fmt.Sprint(check.Words) != "[{rule ./payments/no-account-numbers.js yes No account numbers.}]" {
		t.Errorf("the check is %+v", check)
	}
	if fmt.Sprint(look.Turns) != `[{user Order a part for £35.} {model Told: {"ordered":35}}]` {
		t.Errorf("the lead's conversation is %v", look.Turns)
	}
	if len(look.Calls) != 1 || look.Calls[0].Tool != "order_part" || fmt.Sprint(look.Calls[0].Args, look.Calls[0].Result) != "map[pounds:35] map[ordered:35]" {
		t.Errorf("the lead called %+v", look.Calls)
	}
	if len(look.Forks) != 1 {
		t.Fatalf("the request has the forks %+v", look.Forks)
	}
	if f := look.Forks[0]; f.ID != asked.ID || f.Decision != "decide" || f.Status != "answered" || *f.Answer != "yes" || f.By != "lee" || f.Request != run || f.Team != "payments" {
		t.Errorf("the decision is %+v", f)
	}
	if len(look.Replies) != 1 || look.Replies[0].Status != "done" || answer.Bridge != "slack" {
		t.Errorf("the answer was carried on %s by %+v", answer.Bridge, look.Replies)
	}
	if len(look.Thread) != 1 || look.Thread[0].ID != run {
		t.Errorf("the thread is %+v", look.Thread)
	}

	reader, file, err := e.File(ctx, run, 0)
	if err != nil {
		t.Fatal(err)
	}
	if held, _ := io.ReadAll(reader); string(held) != "a 13A socket" || file.Name != "part.txt" {
		t.Errorf("the file is %q, %+v", held, file)
	}
	if _, _, err := e.File(ctx, run, 1); err == nil {
		t.Error("a second file is given")
	}

	all, err := e.Newest(ctx, nil, 0)
	if err != nil || len(all) != 1 || all[0].ID != run || all[0].Status != "answered" ||
		fmt.Sprint(all[0].Stages) != fmt.Sprint([]Stage{{Name: "heed", State: "passed"}, {Name: "work", State: "passed"}, {Name: "check", State: "passed"}}) {
		t.Errorf("the requests are %+v (%v)", all, err)
	}
	if _, err := e.Look(ctx, asked.ID); err != ErrNoRequest {
		t.Errorf("looking at a wait as a request is %v", err)
	}
}

// states gives the state of each stage, as heed:passed work:waiting.
func states(stages []Stage) string {
	var each []string
	for _, s := range stages {
		each = append(each, s.Name+":"+s.State)
	}
	return strings.Join(each, " ")
}

// A stage that failed shows its error, and a heed that said no shows no.
func TestLookShowsFailedAndNoted(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, func(files map[string]string) {
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "check: [./no-account-numbers.js, lee, priya]", "check: [./throws.js]", 1)
		files["payments/throws.js"] = `throw new Error("the check rule broke")`
	})
	e, _ := o.run()
	run, _ := e.Ask(ctx, refund("C01/1700000000.0002"))
	o.output(run)
	look, err := e.Look(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	check := look.Stages[2]
	if look.Status != "failed" || check.State != "failed" || !strings.Contains(check.Error, "the check rule broke") ||
		fmt.Sprint(check.Words) != "[{rule ./payments/throws.js failed Error: the check rule broke at line 1, column 7}]" {
		t.Errorf("the request is %s, its check %+v", look.Status, check)
	}
	noted := refund("C01/1700000000.0003")
	noted.Said, noted.Mentioned, noted.Place.Where = "lunch?", false, "C01/1700000000.0003"
	run, _ = e.Ask(ctx, noted)
	o.output(run)
	look, err = e.Look(ctx, run)
	if err != nil || look.Status != "noted" || look.Answer != nil || states(look.Stages) != "heed:no work:skipped check:skipped" {
		t.Errorf("the noted request is %+v (%v)", look, err)
	}
}

// A file is carried from the run it came in, as a bridge hands over a file
// its in run heard, and saying more after a request in the chat joins its
// thread, of the same team.
func TestFileCarriedAndChatSaidMore(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e := o.engine()
	first, err := e.Ask(ctx, Ask{Team: "office", By: By{Person: "dana"}, Said: "The socket sparks.",
		Files: []File{{Name: "socket.txt", Data: client.InputFile{Name: "socket.txt", Contents: strings.NewReader("by the boiler")}}}})
	if err != nil {
		t.Fatal(err)
	}
	o.answered(first)
	shown, err := o.corbie.Show(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	blob := shown.Input.(map[string]any)["files"].([]any)[0].(map[string]any)["data"].(map[string]any)
	blob["run"] = first
	o.corbie.Clock.Move(time.Minute)
	more, err := e.Ask(ctx, Ask{Team: "payments", By: By{Person: "dana"}, Said: "Here it is again.", After: first,
		Files: []File{{Name: "socket.txt", Data: blob}}})
	if err != nil {
		t.Fatal(err)
	}
	o.answered(more)
	reader, file, err := e.File(ctx, more, 0)
	if err != nil {
		t.Fatal(err)
	}
	if held, _ := io.ReadAll(reader); string(held) != "by the boiler" || file.Name != "socket.txt" {
		t.Errorf("the file carried is %q, %+v", held, file)
	}
	q, err := e.Request(ctx, more)
	if err != nil || q.Team != "office" || q.Heard != "chat" {
		t.Errorf("saying more is %+v (%v)", q, err)
	}
	look, err := e.Look(ctx, more)
	if err != nil || len(look.Thread) != 2 || look.Thread[0].ID != first || look.Thread[1].ID != more {
		t.Errorf("the thread is %+v (%v)", look.Thread, err)
	}
	if _, err := e.Ask(ctx, Ask{Team: "office", By: By{Person: "dana"}, Said: "More?", After: "client/20261005T090000.000Z-0000000000"}); err != ErrNoRequest {
		t.Errorf("saying more after no request is %v", err)
	}
}

// Newest gives the newest requests that keep keeps, newest first, each
// with the state of its stages, and stops at its limit.
func TestNewestKeepsAndStops(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e := o.engine()
	var office []string
	for n := range 5 {
		team := []string{"office", "payments"}[n%2]
		run := o.chat(e, team, fmt.Sprintf("Question %d?", n))
		o.answered(run)
		if team == "office" {
			office = append(office, run)
		}
		o.corbie.Clock.Move(time.Minute)
	}
	newest, err := e.Newest(ctx, func(q *Request) bool { return q.Team == "office" }, 2)
	if err != nil || len(newest) != 2 || newest[0].ID != office[2] || newest[1].ID != office[1] || states(newest[0].Stages) != "work:passed" {
		t.Errorf("the newest of the office are %+v (%v)", newest, err)
	}
	if all, err := e.Newest(ctx, nil, 0); err != nil || len(all) != 5 {
		t.Errorf("the requests are %+v (%v)", all, err)
	}
}

// Look gives each decision on held jobs: a rule's yes, which put nothing
// to people, and Lee's no, with the wait that put it to him. The call Lee
// denied gives no result, and its error.
func TestLookGivesDecisions(t *testing.T) {
	t.Parallel()
	o := newOffice(t, orders, nil)
	e, _ := o.run()
	small := o.chat(e, "payments", "Order a part for £12.")
	o.answered(small)
	look, err := e.Look(ctx, small)
	if err != nil {
		t.Fatal(err)
	}
	if d := look.Decisions; len(d) != 1 || fmt.Sprint(d[0].Jobs) != "[order_part]" || d[0].Status != "answered" || d[0].Says != "yes" || d[0].Wait != nil ||
		fmt.Sprint(d[0].Words) != "[{rule ./payments/small-orders.js yes Under £20.}]" || len(look.Forks) != 0 {
		t.Errorf("the small order's decisions are %+v, its forks %+v", d, look.Forks)
	}

	big := o.chat(e, "payments", "Order a part for £35.")
	asked := o.waitFor("May these run?")
	if err := e.Answer(ctx, asked.ID, "lee", "no", ""); err != nil {
		t.Fatal(err)
	}
	o.answered(big)
	if look, err = e.Look(ctx, big); err != nil {
		t.Fatal(err)
	}
	if d := look.Decisions; len(d) != 1 || d[0].Says != "no" || d[0].Wait == nil || d[0].Wait.ID != asked.ID ||
		fmt.Sprint(d[0].Words) != "[{rule ./payments/small-orders.js pass More than I order alone.} {person lee no }]" {
		t.Errorf("the big order's decisions are %+v", d)
	}
	if c := look.Calls; len(c) != 1 || c[0].Result != nil || !strings.Contains(c[0].Error, "denied") {
		t.Errorf("the lead called %+v", c)
	}
}

// The calls a colleague made are under the call that asked it, each time
// it is asked, with each call of a tool it called twice in one turn.
func TestLookGivesColleaguesCalls(t *testing.T) {
	t.Parallel()
	o := newOffice(t, func(turn testkit.Turn) testkit.Reply {
		switch clerk := turn.System == "You are the clerk."; {
		case clerk && turn.Number == 1:
			return testkit.Use("stamp", map[string]any{"n": 1}).And("stamp", map[string]any{"n": 2})
		case clerk:
			return testkit.Say("Stamped.")
		case turn.Number <= 2:
			return testkit.Use("clerk", map[string]any{"said": "Stamp it."})
		}
		return testkit.Say("Done.")
	}, func(files map[string]string) {
		files["bot.yaml"] += `tools:
  clerk:
    does: Stamps things
    args: { said: string }
    job: { machine: ./clerk.yaml, input: { said: "${args.said}" } }
`
		files["clerk.yaml"] = `agent: clerk
model: usual
system: You are the clerk.
prompt: ${input.said}
tools:
  stamp:
    does: Stamps a page
    args: { n: number }
    job: { script: ./stamp.js, input: { n: "${args.n}" } }
`
		files["stamp.js"] = "return { stamped: input.n }"
	})
	e, _ := o.run()
	run := o.chat(e, "office", "Stamp the forms.")
	o.answered(run)
	look, err := e.Look(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range look.Calls {
		var under []string
		for _, u := range c.Calls {
			under = append(under, fmt.Sprint(u.Tool, " ", u.Result))
		}
		got = append(got, c.Tool+": "+strings.Join(under, ", "))
	}
	if want := "clerk: stamp map[stamped:1], stamp map[stamped:2]"; len(got) != 2 || got[0] != want || got[1] != want {
		t.Errorf("the lead called %q", got)
	}
}

// A rule may be a definition of the office's own. It runs as a machine,
// and its word is a rule's.
func TestOwnDefinitionAsRule(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, func(files map[string]string) {
		files["office.yaml"] = strings.Replace(files["office.yaml"], "lead: bot", "lead: bot\nheed: [./rules/welcome.yaml]", 1)
		files["rules/welcome.yaml"] = "flow: welcome\nstages:\n  say: { job: { script: ./welcome.js } }\noutput: ${stages.say.output}\n"
		files["rules/welcome.js"] = `return { says: "yes", note: "Welcome." }`
	})
	e, _ := o.run()
	run, err := e.Ask(ctx, Ask{Team: "office", By: By{Guest: "U0GUEST0"}, Said: "Anyone there?", At: stamp,
		Place: &Place{Bridge: "slack", Place: "#help", Guests: true, Where: "C02/1", Thing: "C02/1"}})
	if err != nil {
		t.Fatal(err)
	}
	if answer := o.answered(run); answer != "Refunded £40 on 3 May." {
		t.Errorf("the request answered %q", answer)
	}
	look, err := e.Look(ctx, run)
	if err != nil || fmt.Sprint(look.Stages[0].Words) != "[{rule ./rules/welcome.yaml yes Welcome.}]" {
		t.Errorf("the heed is %+v (%v)", look, err)
	}
}
