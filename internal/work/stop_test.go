package work

import (
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestAStopCancelsTheRequestsRuns(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	held, let := b.hold("", testkit.Say("done"))
	defer let()
	n := b.chat("dana", "", "customer 4411 was charged twice")
	bot := b.started(n, 1)
	b.until("the bot is held", func() bool { return held() == 1 })
	platform, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "platform", "what changed on Tuesday?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	pay, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "payments", "was 4411 charged twice?", "ticket-2")
	if err != nil {
		t.Fatal(err)
	}
	ops, ledger := b.started(platform.Number, 1), b.started(pay.Number, 1)
	b.until("every run is held", func() bool { return held() == 3 })
	question, err := b.reqs.Put(pay.Number, b.reqs.Worker("ledger"), requests.KindQuestion, b.reqs.Person("lee"), "refund it?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.w.Stop(ctx, n, b.reqs.Person("al")); err != nil {
		t.Fatal(err)
	}
	for _, run := range []KeptRun{bot, ops, ledger} {
		if ended := b.corbie.Ended(t, run.Name); ended.Status != "cancelled" {
			t.Errorf("%s's run ended %s", run.Worker, ended.Status)
		}
	}
	for _, r := range []int64{n, platform.Number, pay.Number} {
		b.settled(r, requests.Answered)
		got, err := b.reqs.Get(r)
		if err != nil || !got.Stopped {
			t.Errorf("request %d is not stopped: %v", r, err)
		}
		if th := b.thread(r); !strings.HasSuffix(th[len(th)-1], "stopped by Al") {
			t.Errorf("request %d ends %v", r, th)
		}
	}
	if waits, err := b.reqs.WaitingOn("lee"); err != nil || slices.ContainsFunc(waits, func(w requests.Wait) bool { return w.Entry == question.ID }) {
		t.Errorf("the question to Lee still waits: %v %v", waits, err)
	}
	for _, r := range b.runs(n) {
		if r.Status != cancelled {
			t.Errorf("the bot's run is kept as %+v", r)
		}
	}
}

func TestWorkStopSentOnce(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket", testkit.Say("done"))
	defer let()
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	b.until("the steward is held", func() bool { return held() == 1 })

	// A stop that cannot be noted is not sent, and whoever stopped it is
	// told.
	back := b.away("work_runs")
	if err := b.w.Stop(ctx, n, b.reqs.Person("sam")); err == nil {
		t.Fatal("a stop that could not be noted said nothing")
	}
	b.still()
	if got := b.reach.count("cancel"); got != 0 {
		t.Fatalf("a stop that was not noted sent %d cancels", got)
	}

	// Once it can be noted, it is sent. That it was sent cannot be kept,
	// nor how the run ended: neither sends it again.
	var gone func()
	b.reach.set(func(f *flaky) {
		f.cancelled = func(string) {
			if gone == nil {
				gone = b.away("work_runs")
			}
		}
	})
	back()
	b.later(Again)
	if ended := b.corbie.Ended(t, run.Name); ended.Status != "cancelled" {
		t.Errorf("the run ended %s", ended.Status)
	}
	b.until("the cancel could not be kept as sent", func() bool { return b.logged("was cancelled could not be kept") })
	b.still()
	b.reach.set(func(f *flaky) { f.cancelled = nil })
	gone()
	b.later(Again)
	b.settled(n, requests.Answered)
	for range 3 {
		b.w.Poke()
		b.later(Again)
	}
	b.still()
	if got := b.reach.count("cancel"); got != 1 {
		t.Errorf("the cancel was sent %d times", got)
	}
	if th := b.thread(n); !strings.HasSuffix(th[len(th)-1], "stopped by Sam") {
		t.Errorf("the request ends %v", th)
	}
}

func TestWorkRemovedWorkerRunsStopped(t *testing.T) {
	t.Parallel()
	t.Run("a worker", removedWorkersRunStopped)
	t.Run("a team", removedTeamsRunStopped)
}

func removedWorkersRunStopped(t *testing.T) {
	b := newBench(t, house(t))
	held, let := b.hold("lawn", testkit.Say("done"))
	defer let()
	n := b.chat("sam", "garden", "how is the lawn?")
	run := b.started(n, 1)
	b.until("the gardener is held", func() bool { return held() == 1 })
	b.rewrite(map[string]string{"garden/gardener.yaml": "",
		"garden/team.yaml": "name: The garden\nabout: The beds, the lawn and the shed.\npeople: [sam]\nlead: groundsman\nmodels:\n  usual: { name: opus }\n"})
	if b.office().Workers["gardener"] != nil {
		t.Fatalf("the gardener is still read: %v", b.keeper.Problems())
	}
	if ended := b.corbie.Ended(t, run.Name); ended.Status != "cancelled" {
		t.Errorf("the gardener's run ended %s", ended.Status)
	}
	b.settled(n, requests.Answered)
	if !slices.ContainsFunc(b.entries(n, requests.KindBulldog), func(e requests.Entry) bool {
		return strings.Contains(e.Text, "gardener is no longer a worker of the garden")
	}) {
		t.Errorf("the thread is %v", b.thread(n))
	}
}

func removedTeamsRunStopped(t *testing.T) {
	b := newBench(t, house(t))
	held, let := b.hold("lawn", testkit.Say("the lawn is fine"))
	defer let()
	n := b.chat("sam", "garden", "how is the lawn?")
	run := b.started(n, 1)
	b.until("the gardener is held", func() bool { return held() == 1 })
	b.rewrite(map[string]string{"garden/team.yaml": "", "garden/gardener.yaml": "", "garden/groundsman.yaml": "",
		"garden/notes/lawn.md": "", "garden/notes/shed.md": ""})
	if b.office().Teams["garden"] != nil {
		t.Fatalf("the garden is still read: %v", b.keeper.Problems())
	}
	if ended := b.corbie.Ended(t, run.Name); ended.Status != "cancelled" {
		t.Errorf("the gardener's run ended %s", ended.Status)
	}
	b.settled(n, requests.Answered)
	let()
	b.still()
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 0 {
		t.Errorf("a team that is gone answered: %v", b.thread(n))
	}
}

func TestARunGoesOnAsItWasStarted(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket", testkit.Say("done"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	first := b.started(n, 1)
	b.until("the steward is held", func() bool { return held() == 1 })
	files := house(t)
	b.rewrite(map[string]string{"steward.yaml": strings.Replace(files["steward.yaml"], "You look after the house", "You keep the house", 1)})
	let()
	b.settled(n, requests.Answered)
	b.say(n, "kit", "thanks")
	second := b.started(n, 2)
	b.settled(n, requests.Answered)
	for run, want := range map[string]string{first.Name: "You look after the house", second.Name: "You keep the house"} {
		def, err := b.corbie.Definition(ctx, run)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(def.Body), want) {
			t.Errorf("the run %s was started from %s", run, def.Body)
		}
	}
}
