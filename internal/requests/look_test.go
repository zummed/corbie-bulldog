package requests

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

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
	q, err := e.Request(ctx, run)
	if err != nil || q.Status != "waiting" || fmt.Sprint(q.Stages) != fmt.Sprint([]Stage{{Name: "heed", State: "passed"}, {Name: "work", State: "waiting"}, {Name: "check"}}) {
		t.Errorf("the waiting request is %+v (%v)", q, err)
	}

	if err := e.Answer(ctx, asked.ID, "lee", "yes", "U0LEE:7"); err != nil {
		t.Fatal(err)
	}
	o.ended(o.until(3)[2].Run)
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
	if len(look.Replies) != 1 || look.Replies[0].Bridge != "slack" || look.Replies[0].Status != "done" {
		t.Errorf("the answer was carried by %+v", look.Replies)
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

	all, err := e.Requests(ctx)
	if err != nil || len(all) != 1 || all[0].ID != run || all[0].Status != "answered" ||
		fmt.Sprint(all[0].Stages) != fmt.Sprint([]Stage{{Name: "heed", State: "passed"}, {Name: "work", State: "passed"}, {Name: "check", State: "passed"}}) {
		t.Errorf("the requests are %+v (%v)", all, err)
	}
	if _, err := e.Look(ctx, asked.ID); err != ErrNoRequest {
		t.Errorf("looking at a wait as a request is %v", err)
	}
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
	q, err := e.Request(ctx, run)
	if err != nil || q.Status != "noted" || q.Answer != nil || fmt.Sprint(q.Stages) != fmt.Sprint([]Stage{{Name: "heed", State: "no"}, {Name: "work", State: "skipped"}, {Name: "check", State: "skipped"}}) {
		t.Errorf("the noted request is %+v (%v)", q, err)
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
