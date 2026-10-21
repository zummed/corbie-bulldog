package work

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// marking gives the work office heeding what names the bot, with a
// stand-in Slack that listens and whose out machine takes working, as the
// shipped one's does.
func marking(t *testing.T) map[string]string {
	files := heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs})
	maps.Copy(files, standin.In("slack", standin.Marking(standin.Folder(false, true))))
	return files
}

// carryOn stays until the next out run of a bridge carries to a place,
// ends it as sent, and gives its input.
func (b *bench) carryOn(bridge, to string) map[string]any {
	b.t.Helper()
	run, in := b.carrying(bridge, to)
	b.carried(run, map[string]any{"sent": run})
	b.until("the out run "+run+" is taken", func() bool { return b.ended(run) })
	return in
}

// markOn stays until the next out run of a bridge carries to a place, ends
// it as sent, and gives the mark it carried, or its text.
func (b *bench) markOn(bridge, to string) string {
	b.t.Helper()
	in := b.carryOn(bridge, to)
	if w, ok := in["working"].(string); ok && in["answers"] == nil {
		return w
	}
	return fmt.Sprint(in["text"])
}

// outRuns counts the out runs of a bridge started.
func (b *bench) outRuns(bridge string) int {
	b.t.Helper()
	notes := b.w.runsOfKind(b.t, bridges.KindOut)
	return len(slices.DeleteFunc(notes, func(n *note) bool { return n.Worker != bridge }))
}

func TestWorkMarkedWhileTheLeadWorks(t *testing.T) {
	t.Parallel()
	b := newBench(t, marking(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("Duplicate confirmed.") })
	listening := b.listening("slack")
	// What the heed notes starts no run, and is never marked.
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "morning all"))
	n := b.at("slack", "C1/1.0")
	b.heeded(n, 1)
	b.settled(n, requests.Answered)
	if runs := b.outRuns("slack"); runs != 0 {
		t.Errorf("%d out runs of slack were started for what was noted", runs)
	}
	// What it lets through starts the lead, marked while it works: the
	// mark goes on, the answer goes, and the team is done after it.
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice"))
	b.started(n, 1)
	if in := b.carryOn("slack", "C1/1.0"); in["working"] != "working" || in["answers"] != nil || in["text"] != "" {
		t.Errorf("the first out run is given %v", in)
	}
	if in := b.carryOn("slack", "C1/1.0"); in["text"] != "Duplicate confirmed." || in["working"] != nil {
		t.Errorf("the second out run is given %v", in)
	}
	if got := b.markOn("slack", "C1/1.0"); got != "done" {
		t.Errorf("the third out run carries %q", got)
	}
	b.settled(n, requests.Answered)
	// A follow-up in the thread is marked the same.
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot and the second card?"))
	b.started(n, 2)
	got := []string{b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0")}
	if !slices.Equal(got, []string{"working", "Duplicate confirmed.", "done"}) {
		t.Errorf("the follow-up is carried as %v", got)
	}
	b.settled(n, requests.Answered)
	if runs := b.outRuns("slack"); runs != 6 {
		t.Errorf("%d out runs of slack were started", runs)
	}
}

func TestWorkMarkWaitsWhileAPersonIsAsked(t *testing.T) {
	t.Parallel()
	b := newBench(t, marking(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"question": "may I refund 4411?", "who": "Al"})
		}
		return testkit.Say("Refunded.")
	})
	listening := b.listening("slack")
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice"))
	n := b.at("slack", "C1/1.0")
	b.started(n, 1)
	if got := b.markOn("slack", "C1/1.0"); got != "working" {
		t.Errorf("as the bot starts, the thread is carried %q", got)
	}
	// The bot asks Al, directly: the thread waits on someone while the
	// question does, and the team works again once Al answers.
	if in := b.carryOn("slack", "U0AL0000"); in["text"] != "may I refund 4411?" {
		t.Errorf("Al is carried %v", in)
	}
	if got := b.markOn("slack", "C1/1.0"); got != "waiting" {
		t.Errorf("while Al is asked, the thread is carried %q", got)
	}
	q := b.waitsOn(n, requests.KindQuestion)
	if _, err := b.reqs.Reply(q.ID, b.reqs.Person("al"), "yes", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	got := []string{b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0")}
	if !slices.Equal(got, []string{"working", "Refunded.", "done"}) {
		t.Errorf("once Al answers, the thread is carried %v", got)
	}
	b.settled(n, requests.Answered)
	b.still()
	if runs := b.outRuns("slack"); runs != 6 {
		t.Errorf("%d out runs of slack were started", runs)
	}
}

func TestWorkNoMarkOnABridgeThatTakesNoWorking(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("The retry is mended.") })
	listening := b.listening("gitlab")
	b.hear(listening, 1, thing("said", "1041", "platform/api", "platform/api#7", "text", "the retry is broken"))
	n := b.at("gitlab", "platform/api#7")
	b.settled(n, requests.Answered)
	if in := b.carryOn("gitlab", "platform/api#7"); in["text"] != "The retry is mended." || in["working"] != nil {
		t.Errorf("the answer is carried as %v", in)
	}
	b.still()
	if runs := b.outRuns("gitlab"); runs != 1 {
		t.Errorf("%d out runs of gitlab were started", runs)
	}
}

func TestWorkMarkOffHoweverTheRunEnds(t *testing.T) {
	t.Parallel()
	b := newBench(t, marking(t))
	held, let := b.hold("", testkit.Say("done"))
	defer let()
	listening := b.listening("slack")
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice"))
	n := b.at("slack", "C1/1.0")
	b.started(n, 1)
	b.until("the bot is held", func() bool { return held() == 1 })
	if got := b.markOn("slack", "C1/1.0"); got != "working" {
		t.Errorf("the mark is %q", got)
	}
	// Al stops it while the bot works: Dana is told, and then the team is
	// done there.
	if err := b.w.Stop(ctx, n, b.reqs.Person("al")); err != nil {
		t.Fatal(err)
	}
	if in := b.carryOn("slack", "C1/1.0"); !strings.Contains(fmt.Sprint(in["text"]), "stopped by Al") || in["working"] != nil {
		t.Errorf("after the stop, the out run is given %v", in)
	}
	if got := b.markOn("slack", "C1/1.0"); got != "done" {
		t.Errorf("after the stop, the mark is %q", got)
	}
	b.settled(n, requests.Answered)

	// A run that fails, here at its turn limit, has Dana told, and then
	// the team is done there.
	files := marking(t)
	files["bot.yaml"] = strings.Replace(files["bot.yaml"], "model: usual", "model: usual\nlimits: { turns: 1 }", 1)
	f := newBench(t, files)
	f.says(func(testkit.Turn) testkit.Reply { return testkit.Use("list_docs", map[string]any{}) })
	failing := f.listening("slack")
	f.hand(failing, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice"))
	m := f.at("slack", "C1/1.0")
	if got := f.markOn("slack", "C1/1.0"); got != "working" {
		t.Errorf("the mark is %q", got)
	}
	if got := f.markOn("slack", "C1/1.0"); got == "done" || got == "working" {
		t.Errorf("after the failure, the thread is carried %q", got)
	}
	if got := f.markOn("slack", "C1/1.0"); got != "done" {
		t.Errorf("after the failure, the mark is %q", got)
	}
	f.settled(m, requests.Answered)
	if runs := f.runs(m); len(runs) != 1 || runs[0].Status != failed {
		t.Errorf("the bot's runs are %+v", runs)
	}
}

// quietAt stays until the work is still, and says whether no out run of a
// bridge carries to a place.
func (b *bench) quietAt(bridge, to string) bool {
	b.t.Helper()
	b.still()
	return !slices.ContainsFunc(b.going(bridges.KindOut, bridge), func(r *client.Run) bool { return b.input(r.ID)["to"] == to })
}

func TestWorkNoDoneWhileTheNextRunGoes(t *testing.T) {
	t.Parallel()
	b := newBench(t, marking(t))
	held, let := b.hold("charged twice", testkit.Say("Duplicate confirmed."))
	listening := b.listening("slack")
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice"))
	n := b.at("slack", "C1/1.0")
	b.started(n, 1)
	b.until("the bot is held", func() bool { return held() == 1 })
	if got := b.markOn("slack", "C1/1.0"); got != "working" {
		t.Errorf("as the bot starts, the thread is carried %q", got)
	}
	// Dana says more while the bot works. Its answer then reads as the
	// request's, and the next run, on what she said, starts on a request
	// answered: the team is done there only once that run has ended.
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot and the second card?"))
	again, letAgain := b.hold("second card", testkit.Say("Charged once."))
	let()
	b.started(n, 2)
	b.until("the bot is held again", func() bool { return again() == 1 })
	got := []string{b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0")}
	if !slices.Equal(got, []string{"Duplicate confirmed.", "done", "working"}) {
		t.Errorf("after the first run, the thread is carried %v", got)
	}
	if !b.quietAt("slack", "C1/1.0") {
		t.Errorf("while the next run goes, the thread is carried more")
	}
	letAgain()
	got = []string{b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0")}
	if !slices.Equal(got, []string{"Charged once.", "done"}) {
		t.Errorf("after the next run, the thread is carried %v", got)
	}
	b.settled(n, requests.Answered)
	if !b.quietAt("slack", "C1/1.0") {
		t.Errorf("once done, the thread is carried more")
	}
}

func TestWorkDoneAfterARunEndedWhileAway(t *testing.T) {
	t.Parallel()
	b := newBench(t, marking(t))
	held, let := b.hold("charged twice", testkit.Say("Duplicate confirmed."))
	listening := b.listening("slack")
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice"))
	n := b.at("slack", "C1/1.0")
	run := b.started(n, 1)
	b.until("the bot is held", func() bool { return held() == 1 })
	if got := b.markOn("slack", "C1/1.0"); got != "working" {
		t.Errorf("as the bot starts, the thread is carried %q", got)
	}
	// Bulldog stops, and the bot's run ends while it is away: once it is
	// started again, the answer goes, and then the team is done there.
	b.halt()
	let()
	b.corbie.Ended(t, run.Name)
	b.restart()
	got := []string{b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0")}
	if !slices.Equal(got, []string{"Duplicate confirmed.", "done"}) {
		t.Errorf("after the restart, the thread is carried %v", got)
	}
	b.settled(n, requests.Answered)
}

func TestWorkMarkWorkingWhileADisclosureWaits(t *testing.T) {
	t.Parallel()
	files, checking := marking(t), payments(t)
	for _, name := range []string{"payments/team.yaml", "payments/no-cards.js"} {
		files[name] = checking[name]
	}
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`, `"#help": { team: payments, guests: true }`, 1)
	b := newBench(t, files)
	b.says(func(testkit.Turn) testkit.Reply {
		return testkit.Say("Duplicate confirmed on card 4111 1111 1111 1111.")
	})
	listening := b.listening("slack")
	b.hand(listening, thing("said", "U0DANA00", "#help", "C1/1.0", "text", "customer 4411 was charged twice"))
	n := b.at("slack", "C1/1.0")
	if got := b.markOn("slack", "C1/1.0"); got != "working" {
		t.Errorf("as the ledger starts, the thread is carried %q", got)
	}
	// The ledger's answer names a card, so its disclosure check refers it
	// to Lee and Priya: Dana waits on payments, still at work, and her
	// thread is told nothing more.
	dec := b.disclosing(n, decisions.SaysAnswer)
	if in := b.carryOn("slack", "U0LEE000"); !strings.Contains(fmt.Sprint(in["text"]), "4111") {
		t.Errorf("Lee is carried %v", in)
	}
	if !b.quietAt("slack", "C1/1.0") {
		t.Errorf("while the disclosure waits, the thread is carried more")
	}
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("lee"), "yes", ""); err != nil || !r.Taken {
		t.Fatalf("Lee's yes: %v %v", r, err)
	}
	got := []string{b.markOn("slack", "C1/1.0"), b.markOn("slack", "C1/1.0")}
	if !strings.Contains(got[0], "4111") || got[1] != "done" {
		t.Errorf("once disclosed, the thread is carried %v", got)
	}
	b.settled(n, requests.Answered)
}
