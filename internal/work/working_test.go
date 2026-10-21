package work

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
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
	// mark goes on, the answer goes, and the mark comes off after it.
	asked := thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice")
	b.hand(listening, asked)
	b.started(n, 1)
	if in := b.carryOn("slack", "C1/1.0"); in["working"] != true || in["answers"] != asked["id"] || in["text"] != "" {
		t.Errorf("the first out run is given %v", in)
	}
	if in := b.carryOn("slack", "C1/1.0"); in["text"] != "Duplicate confirmed." || in["working"] != nil {
		t.Errorf("the second out run is given %v", in)
	}
	if in := b.carryOn("slack", "C1/1.0"); in["working"] != false || in["answers"] != asked["id"] {
		t.Errorf("the third out run is given %v", in)
	}
	b.settled(n, requests.Answered)
	// A follow-up in the thread is marked on its own.
	more := thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot and the second card?")
	b.hand(listening, more)
	b.started(n, 2)
	got := []any{b.carryOn("slack", "C1/1.0")["answers"], b.carryOn("slack", "C1/1.0")["text"], b.carryOn("slack", "C1/1.0")["answers"]}
	if !slices.Equal(got, []any{more["id"], "Duplicate confirmed.", more["id"]}) {
		t.Errorf("the follow-up is carried as %v", got)
	}
	b.settled(n, requests.Answered)
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
	asked := thing("said", "U0DANA00", "#help", "C1/1.0", "text", "@bot customer 4411 was charged twice")
	b.hand(listening, asked)
	n := b.at("slack", "C1/1.0")
	b.started(n, 1)
	b.until("the bot is held", func() bool { return held() == 1 })
	if in := b.carryOn("slack", "C1/1.0"); in["working"] != true || in["answers"] != asked["id"] {
		t.Errorf("the mark is given %v", in)
	}
	// Al stops it while the bot works: Dana is told, and then the mark
	// comes off.
	if err := b.w.Stop(ctx, n, b.reqs.Person("al")); err != nil {
		t.Fatal(err)
	}
	if in := b.carryOn("slack", "C1/1.0"); !strings.Contains(fmt.Sprint(in["text"]), "stopped by Al") || in["working"] != nil {
		t.Errorf("after the stop, the out run is given %v", in)
	}
	if in := b.carryOn("slack", "C1/1.0"); in["working"] != false || in["answers"] != asked["id"] {
		t.Errorf("the mark comes off as %v", in)
	}
	b.settled(n, requests.Answered)
}
