package work

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// going gives the runs at the top of a kind that have not ended, of a
// bridge when one is named, oldest first.
func (b *bench) going(kind, bridge string) []*client.Run {
	b.t.Helper()
	labels := map[string]string{LabelBulldog: identity, LabelKind: kind}
	if bridge != "" {
		labels[bridges.LabelBridge] = bridge
	}
	runs, err := b.corbie.List(ctx, client.Filter{Labels: labels})
	if err != nil {
		b.t.Fatal(err)
	}
	runs = slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Parent != "" || r.HasEnded() })
	slices.SortFunc(runs, func(a, c *client.Run) int { return strings.Compare(a.ID, c.ID) })
	return runs
}

// listening stays until a bridge's listening run waits on what it hears,
// and gives it.
func (b *bench) listening(bridge string) string {
	b.t.Helper()
	run := ""
	b.until(bridge+" listens", func() bool {
		for _, r := range b.going(bridges.KindIn, bridge) {
			if slices.Contains(r.Tickets, standin.Heard) {
				run = r.ID
			}
		}
		return run != ""
	})
	return run
}

// hear has a stand-in in run end with the things it heard and a cursor.
func (b *bench) hear(run string, cursor any, things ...map[string]any) {
	b.t.Helper()
	heard := []any{}
	for _, t := range things {
		heard = append(heard, t)
	}
	b.answerTicket(run, standin.Heard, map[string]any{"heard": heard, "cursor": cursor})
}

// answerTicket answers the ticket a run waits on.
func (b *bench) answerTicket(run, ticket string, value any) {
	b.t.Helper()
	if _, err := b.corbie.Reply(ctx, run, b.corbie.Waiting(b.t, run, ticket), value, 0); err != nil {
		b.t.Fatal(err)
	}
}

// carrying stays until an out run of a bridge carries something to a
// place, and gives the run and its input.
func (b *bench) carrying(bridge, to string) (string, map[string]any) {
	b.t.Helper()
	run := ""
	var in map[string]any
	b.until(fmt.Sprintf("%s carries to %s", bridge, to), func() bool {
		for _, r := range b.going(bridges.KindOut, bridge) {
			if !slices.Contains(r.Tickets, standin.Carried) {
				continue
			}
			if got := b.input(r.ID); got["to"] == to {
				run, in = r.ID, got
			}
		}
		return run != ""
	})
	return run, in
}

// carried has a stand-in out run end with what it sent or why it was
// refused.
func (b *bench) carried(run string, out map[string]any) {
	b.t.Helper()
	b.answerTicket(run, standin.Carried, out)
}

// file gives a file of the office as its folder holds it.
func (b *bench) file(name string) string {
	b.t.Helper()
	text, err := os.ReadFile(filepath.Join(b.dir, filepath.FromSlash(name)))
	if err != nil {
		b.t.Fatal(err)
	}
	return string(text)
}

// thing gives a thing an in run hears, with an id of its own.
var things atomic.Int64

func thing(kind, from, heardIn, place string, more ...any) map[string]any {
	t := map[string]any{"id": fmt.Sprintf("t%d", things.Add(1)), "kind": kind, "from": from, "heard_in": heardIn, "place": place}
	for i := 0; i+1 < len(more); i += 2 {
		t[more[i].(string)] = more[i+1]
	}
	return t
}

// at gives the latest request whose origin is a place on a bridge, once
// there is one.
func (b *bench) at(bridge, place string) int64 {
	b.t.Helper()
	var n int64
	b.until(fmt.Sprintf("a request is asked on %s at %s", bridge, place), func() bool {
		got, ok, err := b.reqs.LatestAt(bridge, place)
		if err != nil {
			b.t.Fatal(err)
		}
		n = got
		return ok
	})
	return n
}

func TestABridgeListensOneRunAtATime(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("The socket is mended.") })
	first := b.listening("telegram")
	if in := b.input(first); in["cursor"] != nil || in["settings"] == nil {
		t.Errorf("the first listening run is given %v", in)
	}
	// What it heard is taken, its cursor kept, and the next run is given
	// it; never two at once.
	b.hear(first, 1, thing("said", "4472", "direct", "4472", "text", "the kitchen socket sparked"))
	n := b.at("telegram", "4472")
	second := b.listening("telegram")
	if in := b.input(second); in["cursor"] != float64(1) {
		t.Errorf("the second listening run is given %v", in)
	}
	if runs := b.going(bridges.KindIn, "telegram"); len(runs) != 1 {
		t.Errorf("%d listening runs of telegram go", len(runs))
	}
	if asks := b.entries(n, requests.KindAsked); len(asks) != 1 || asks[0].Text != "the kitchen socket sparked" || asks[0].By.ID != "4472" {
		t.Errorf("what Kit said is taken as %v", b.thread(n))
	}

	// The answer is carried by an out run to where it was asked, and its
	// ending is handed back: the thing is delivered.
	b.settled(n, requests.Answered)
	out, in := b.carrying("telegram", "4472")
	if in["text"] != "The socket is mended." || in["direct"] != true {
		t.Errorf("the answer is carried as %v", in)
	}
	b.carried(out, map[string]any{"sent": "m-1"})
	b.until("the answer is delivered", func() bool {
		cs := must(b.b.Carried(n))
		return slices.ContainsFunc(cs, func(c bridges.Carried) bool {
			return c.Bridge == "telegram" && c.State == bridges.Sent && c.Sent == "m-1"
		})
	})
	if notes := b.w.runsOfKind(t, bridges.KindOut); len(notes) != 1 || notes[0].Status != done || notes[0].Request != 0 {
		t.Errorf("the out run is kept as %+v", notes)
	}
}

// runsOfKind gives the runs of a kind noted, oldest first.
func (w *Work) runsOfKind(t *testing.T, kind string) []*note {
	t.Helper()
	notes, err := scanNotes(w.db.Query(noteColumns+` WHERE kind = ? ORDER BY rowid`, kind))
	if err != nil {
		t.Fatal(err)
	}
	return notes
}

func TestAListeningRunThatFailsIsStartedAgainAfterAPause(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	run := b.listening("telegram")
	for i, pause := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		b.answerTicket(run, standin.Heard, map[string]any{"fail": "Telegram said 502"})
		b.until("the failure is shown", func() bool {
			ts := must(b.b.Troubles())
			return len(ts) == 1 && ts[0].Failures == i+1 && strings.Contains(ts[0].Error, "Telegram said 502")
		})
		b.later(pause - time.Second)
		b.still()
		if got := len(b.w.runsOfKind(t, bridges.KindIn)); got != i+1 {
			t.Fatalf("after %d failures, %d listening runs were started before the pause of %v was over", i+1, got, pause)
		}
		b.later(time.Second)
		b.until("the next listening run is started", func() bool { return len(b.w.runsOfKind(t, bridges.KindIn)) == i+2 })
		run = b.listening("telegram")
	}
	b.hear(run, 1)
	b.until("the failure is no longer shown", func() bool { return len(must(b.b.Troubles())) == 0 })
}

func TestABridgeRunIsFoundByItsLabelsAfterARestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("The socket is mended.") })
	listening := b.listening("telegram")
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	if _, err := b.reqs.Put(n, b.reqs.Worker("steward"), requests.KindTold, b.reqs.Person("sam"), "the power is off at breaker 4", nil); err != nil {
		t.Fatal(err)
	}
	out, _ := b.carrying("telegram", "4471")

	// Bulldog is away while both runs end.
	b.halt()
	b.hear(listening, 7, thing("said", "4472", "direct", "4472", "text", "the boiler is cold"))
	b.carried(out, map[string]any{"sent": "m-2"})
	b.corbie.Ended(t, listening)
	b.corbie.Ended(t, out)
	b.restart()
	asked := b.at("telegram", "4472")
	if asked == n {
		t.Fatalf("what was heard while away is not taken")
	}
	next := b.listening("telegram")
	if next == listening || b.input(next)["cursor"] != float64(7) {
		t.Errorf("the next listening run %s is given %v", next, b.input(next))
	}
	b.until("the tell is delivered", func() bool {
		return slices.ContainsFunc(must(b.b.Carried(n)), func(c bridges.Carried) bool { return c.Sent == "m-2" })
	})
	for _, r := range b.going(bridges.KindOut, "telegram") {
		if b.input(r.ID)["to"] == "4471" {
			t.Errorf("the tell is carried again by %s", r.ID)
		}
	}
}

func TestAHearingRunAnswersItsService(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("It is up.") })
	start, err := b.b.Hear("slack", bridges.Delivery{Method: "POST", Body: `{"type":"event_callback"}`})
	if err != nil {
		t.Fatal(err)
	}
	answered, err := b.w.Hear(start)
	if err != nil {
		t.Fatal(err)
	}
	var runs []*client.Run
	b.until("the hearing run is started", func() bool {
		runs = b.going(bridges.KindIn, "slack")
		return len(runs) > 0
	})
	if len(runs) != 1 || b.input(runs[0].ID)["delivery"].(map[string]any)["body"] != `{"type":"event_callback"}` {
		t.Fatalf("the hearing runs are %v", runs)
	}
	b.answerTicket(runs[0].ID, standin.Heard, map[string]any{
		"heard":  []any{thing("said", "U0DANA00", "#help", "#help/1700.9", "text", "is the API down?")},
		"answer": map[string]any{"body": "ok-1"},
	})
	select {
	case a := <-answered:
		if a == nil || string(a.Body) != "ok-1" {
			t.Errorf("the service is answered %+v", a)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("the service is never answered")
	}
	n := b.at("slack", "#help/1700.9")
	b.started(n, 1)
	b.settled(n, requests.Answered)
}

func TestWorkRequestClosedAtItsSourceIsStopped(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "platform/api: { team: platform }", "platform/api: {}", 1)
	b := newBench(t, files)
	gate := make(chan struct{})
	defer close(gate)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "the deploy failed") && turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "is the pipeline red?"})
		case strings.Contains(turn.Prompt(), "is the pipeline red?"):
			// Ops is still reading GitLab when the issue is closed.
			<-gate
		}
		return testkit.Say("done")
	})
	link := map[string]any{"link": "https://gitlab.example.com/platform/api/-/issues/7"}
	b.hear(b.listening("gitlab"), 1, thing("said", "1187", "platform/api", "platform/api#7", "text", "the deploy failed", "about", link))
	n := b.at("gitlab", "platform/api#7")
	bot := b.started(n, 1)
	sub := b.subOf(n)
	ops := b.started(sub, 1)

	// A label put on the issue writes nothing and starts nothing.
	before := len(b.thread(n))
	b.hear(b.listening("gitlab"), 2, thing("status", "1187", "platform/api", "platform/api#7", "status", "labelled bug", "closed", false, "about", link))
	b.until("the status is taken", func() bool {
		r := must(b.reqs.Get(n))
		return r.Source != nil && r.Source.Status == "labelled bug"
	})
	if len(b.thread(n)) != before || len(b.runs(n)) != 1 {
		t.Errorf("a label wrote %v", b.thread(n))
	}

	// Raj closes it: the request is stopped as a person's stop is.
	b.hear(b.listening("gitlab"), 3, thing("status", "1187", "platform/api", "platform/api#7", "status", "closed", "closed", true, "about", link))
	for _, run := range []string{bot.Name, ops.Name} {
		if ended := b.corbie.Ended(t, run); ended.Status != "cancelled" {
			t.Errorf("the run %s ended %s", run, ended.Status)
		}
	}
	b.settled(sub, requests.Answered)
	b.settled(n, requests.Answered)
	holds(t, "the request", strings.Join(b.thread(n), "\n"), "closed at gitlab by Raj")
	if r := must(b.reqs.Get(sub)); !r.Stopped {
		t.Errorf("the sub-request is %+v", r)
	}
}

// subOf stays until a request has a sub-request, and gives the first.
func (b *bench) subOf(n int64) int64 {
	b.t.Helper()
	var sub int64
	b.until(fmt.Sprintf("request %d has a sub-request", n), func() bool {
		tree := must(b.reqs.Tree(n))
		if len(tree) > 0 {
			sub = tree[0].Number
		}
		return sub != 0
	})
	return sub
}

func TestNobodyWatchesABridgesRun(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("It is up.") })
	n, err := b.reqs.AskAt("slack", "#help", "#help/1700.4", "", "U0DANA00", "is the API down?", nil)
	if err != nil {
		t.Fatal(err)
	}
	b.settled(n.Number, requests.Answered)
	out, _ := b.carrying("slack", "#help/1700.4")
	listening := b.listening("gitlab")
	bot := b.runs(n.Number)[0].Name
	for run, may := range map[string]bool{bot: true, out: false, listening: false} {
		for _, person := range []string{"al", "dana"} {
			got, err := b.w.MayWatch(run, person)
			if err != nil {
				t.Fatal(err)
			}
			if want := may && person == "al"; got != want {
				t.Errorf("%s may watch %s: %v", person, run, got)
			}
		}
	}
	// What the request read never holds a bridge's run.
	records, err := b.reqs.WhatWasRead(n.Number, "al")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Run != bot {
		t.Errorf("what was read is %v", records)
	}
}
