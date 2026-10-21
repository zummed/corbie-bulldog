package work

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

var unreachable = errors.New("the store is out of reach")

func TestWhatHappenedWhileAwayIsTakenUp(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket", testkit.Say("mended"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	b.until("the steward is held", func() bool { return held() == 1 })
	b.halt()
	waiting := b.chat("sam", "", "is the boiler serviced?")
	let()
	b.corbie.Ended(t, run.Name)
	b.restart()
	b.settled(n, requests.Answered)
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 1 || answers[0].Text != "mended" {
		t.Errorf("the ending while away was taken as %v", b.thread(n))
	}
	b.started(waiting, 1)
	b.settled(waiting, requests.Answered)
}

func TestWorkUnreadRecordNotTakenAsEnded(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket", testkit.Say("mended"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	b.until("the steward is held", func() bool { return held() == 1 })
	b.halt()
	let()
	b.corbie.Ended(t, run.Name)
	b.reach.fail("show", unreachable)
	b.restart()
	b.until("the record could not be read", func() bool { return b.logged("could not be read") })
	// The clock moves only once the pass that could not read it has said
	// when to look again, each time.
	b.still()
	shows := b.reach.count("show")
	b.later(Again / 2)
	b.still()
	if got := b.runs(n)[0]; got.Status != going || b.reach.count("show") != shows {
		t.Fatalf("the run is %+v, looked at %d times", got, b.reach.count("show")-shows)
	}
	b.later(Again / 2)
	b.until("the record is looked at again", func() bool { return b.reach.count("show") > shows })
	b.still()
	if got := b.runs(n)[0]; got.Status != going {
		t.Fatalf("a run whose record could not be read was taken as %+v", got)
	}
	b.reach.fail("show", nil)
	b.later(Again)
	b.settled(n, requests.Answered)
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 1 || answers[0].Text != "mended" {
		t.Errorf("the thread is %v", b.thread(n))
	}
}

// waiter is a chart that waits on a ticket for ever.
func waiter(t *testing.T) *machine.Draft {
	d, err := machine.Parse([]byte("statechart: waiter\ninitial: waiting\nstates:\n  waiting: { ticket: never, on: { never: over } }\n  over: { end: done }\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestWorkUnknownRunCancelled(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.halt()
	labels := map[string]string{LabelBulldog: identity, LabelKind: KindWork, requests.LabelTeam: "office", requests.LabelRequest: "99"}
	left, err := b.corbie.Start(ctx, waiter(t), client.Start{Labels: labels})
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.corbie.Start(ctx, waiter(t), client.Start{Labels: map[string]string{LabelBulldog: "another office", LabelKind: KindWork}})
	if err != nil {
		t.Fatal(err)
	}
	b.restart()
	if ended := b.corbie.Ended(t, left); ended.Status != "cancelled" {
		t.Errorf("the run left over ended %s", ended.Status)
	}
	// One that appears later is cancelled too; another office's is left.
	later, err := b.corbie.Start(ctx, waiter(t), client.Start{Labels: labels})
	if err != nil {
		t.Fatal(err)
	}
	if ended := b.corbie.Ended(t, later); ended.Status != "cancelled" {
		t.Errorf("the run started later ended %s", ended.Status)
	}
	if shown, err := b.corbie.Show(ctx, other); err != nil || shown.HasEnded() {
		t.Errorf("another office's run is %v (%v)", shown, err)
	}
}

func TestWorkNotedRunWithoutRecordEndedAsLost(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.halt()
	n := b.chat("kit", "", "the kitchen socket sparked")
	name, err := b.corbie.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	thread, err := b.reqs.Thread(n)
	if err != nil {
		t.Fatal(err)
	}
	// Noted, and never sent: Bulldog stopped in between.
	if err := b.w.keep(&note{Name: name, Kind: KindWork, Team: "office", Request: n, Worker: "steward",
		Labels: map[string]string{LabelBulldog: identity}, Seen: thread[0].ID, Noted: b.clock.Now()}); err != nil {
		t.Fatal(err)
	}
	// While Corbie says its start waits to be taken up, it is left so.
	b.reach.set(func(f *flaky) {
		f.showing = func(r *client.Run) *client.Run {
			if r.ID == name {
				return &client.Run{ID: name, Status: "waiting"}
			}
			return nil
		}
	})
	b.restart()
	b.until("the run is looked at", func() bool { return b.reach.count("show") > 0 })
	b.still()
	b.later(Again)
	b.still()
	if runs := b.runs(n); len(runs) != 1 || runs[0].Status != going {
		t.Fatalf("a run waiting to start was taken as %+v", runs)
	}
	b.reach.set(func(f *flaky) { f.showing = nil })
	b.later(Again)
	b.settled(n, requests.Answered)
	if runs := b.runs(n); len(runs) != 1 || runs[0].Status != lost {
		t.Errorf("the runs are %+v", runs)
	}
	if th := b.thread(n); !slices.ContainsFunc(th, func(l string) bool { return strings.Contains(l, "The house could not answer.") }) {
		t.Errorf("the thread is %v", th)
	}
}

func TestWorkARefusedStartIsLookedAt(t *testing.T) {
	t.Parallel()
	// A job marked for a yes, its ask taken out after it was checked:
	// Corbie's controller refuses it.
	held, err := machine.Parse([]byte("flow: refused\nask: { machine: { statechart: asker, initial: a, states: { a: { end: done } } } }\n"+
		"stages:\n  a:\n    job: { machine: { statechart: held, initial: a, states: { a: { end: done } } }, approve: true }\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	unasked := *held.Definition
	unasked.Ask = nil
	refused := &machine.Draft{Definition: &unasked, Files: held.Files, Children: held.Children}
	b := newBench(t, house(t))
	listening := b.listening("telegram")
	refusing := map[string]bool{KindWork: true, decisions.Deciding: true, bridges.KindIn: true}
	b.reach.set(func(f *flaky) {
		f.drafting = func(s client.Start) *machine.Draft {
			if refusing[s.Labels[LabelKind]] {
				return refused
			}
			return nil
		}
	})
	// sent stays until the start of the runs noted of a kind, so many of
	// them, has gone, and Corbie has refused it.
	sent := func(kind string, many int) {
		b.until("the start is refused", func() bool {
			notes := b.w.runsOfKind(t, kind)
			if len(notes) != many || b.w.isSending(notes[len(notes)-1].Name) {
				return false
			}
			r, err := b.corbie.Show(ctx, notes[len(notes)-1].Name)
			return err == nil && r.Status == "failed"
		})
		b.still()
	}

	// A lead's: its request is answered, its team told what Corbie said.
	n := b.chat("kit", "", "the kitchen socket sparked")
	sent(KindWork, 1)
	if r := must(b.reqs.Get(n)); r.Answered {
		t.Fatalf("the request was answered before its start was looked at: %v", b.thread(n))
	}
	b.later(Again)
	b.settled(n, requests.Answered)
	if th := b.thread(n); !slices.ContainsFunc(th, func(l string) bool { return strings.Contains(l, "did not pass its check") }) ||
		!slices.ContainsFunc(th, func(l string) bool { return strings.Contains(l, "The house could not answer.") }) {
		t.Errorf("the thread is %v", th)
	}
	if runs := b.runs(n); len(runs) != 1 || runs[0].Status != failed || runs[0].Started.IsZero() || runs[0].Ended.IsZero() {
		t.Errorf("the refused run is kept as %+v", runs)
	}

	// A decider's: it ends with what Corbie said.
	got := make(chan error, 1)
	go func() {
		_, err := b.w.decider(ctx, deciding(office.IsRule, "small-orders.js", "office", n, 1))
		got <- err
	}()
	sent(decisions.Deciding, 1)
	b.later(Again)
	if err := <-got; err == nil || !strings.Contains(err.Error(), "did not pass its check") {
		t.Errorf("the decider ended with %v", err)
	}

	// A bridge's listening run's: its failure is shown with what Corbie
	// said, and the bridge is listened to again after its pause.
	b.answerTicket(listening, standin.Heard, map[string]any{"fail": "Telegram said 502"})
	b.until("the failure is shown", func() bool {
		ts := must(b.b.Troubles())
		return len(ts) == 1 && ts[0].Failures == 1
	})
	b.later(time.Second)
	sent(bridges.KindIn, 2)
	b.later(Again)
	b.until("the refusal is shown", func() bool {
		ts := must(b.b.Troubles())
		return len(ts) == 1 && ts[0].Failures == 2 && strings.Contains(ts[0].Error, "did not pass its check")
	})
	b.reach.set(func(f *flaky) { f.drafting = nil })
	b.later(time.Minute)
	b.listening("telegram", listening)
}

func TestWorkAStartIsGoodForAnHour(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("tell", map[string]any{"text": "on it"}))
	b.answered(b.chat("kit", "", "the kitchen socket sparked"))
	var lives []time.Duration
	b.reach.set(func(f *flaky) { lives = slices.Clone(f.lives) })
	if len(lives) < 2 || slices.ContainsFunc(lives, func(l time.Duration) bool { return l != Life }) {
		t.Errorf("the starts and the replies sent were good for %v", lives)
	}

	// A start Corbie says was never taken up may be taken up until its
	// hour has passed.
	b = newBench(t, house(t))
	b.halt()
	n := b.chat("kit", "", "the kitchen socket sparked")
	name, err := b.corbie.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	thread, err := b.reqs.Thread(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.w.keep(&note{Name: name, Kind: KindWork, Team: "office", Request: n, Worker: "steward",
		Labels: map[string]string{LabelBulldog: identity}, Seen: thread[0].ID, Noted: b.clock.Now()}); err != nil {
		t.Fatal(err)
	}
	b.reach.fail("show", client.ErrNeverTaken)
	b.restart()
	b.until("the run is looked at", func() bool { return b.reach.count("show") > 0 })
	b.still()
	b.later(30 * time.Minute)
	b.still()
	if runs := b.runs(n); len(runs) != 1 || runs[0].Status != going {
		t.Fatalf("a start within its hour was taken as %+v", runs)
	}
	b.later(Life)
	b.settled(n, requests.Answered)
	if runs := b.runs(n); len(runs) != 1 || runs[0].Status != lost {
		t.Errorf("the runs are %+v", runs)
	}
}

func TestWorkRunBeingSentNotLost(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	b.halt()
	b.reach.fail("watch", unreachable)
	b.restart()
	b.until("watching is said to have failed", func() bool { return b.logged("trying again in a second") })
	// A delivery's start is held on its way to Corbie, after its run is
	// noted, while the first listing ends and Bulldog settles.
	var mu sync.Mutex
	hearing, shown := "", false
	gate := make(chan struct{})
	b.reach.set(func(f *flaky) {
		f.starting = func(s client.Start) error {
			if _, ok := s.Input.(map[string]any)["delivery"]; ok {
				mu.Lock()
				hearing = s.Run
				mu.Unlock()
				b.poke()
				<-gate
			}
			return nil
		}
		f.showing = func(r *client.Run) *client.Run {
			mu.Lock()
			shown = shown || r.ID == hearing
			mu.Unlock()
			return nil
		}
	})
	start, err := b.b.Hear("slack", bridges.Delivery{Method: "POST", Body: `{"type":"event_callback"}`})
	if err != nil {
		t.Fatal(err)
	}
	heard := make(chan (<-chan *bridges.Answer), 1)
	go func() {
		answered, err := b.w.Hear(start)
		if err != nil {
			t.Error(err)
		}
		heard <- answered
	}()
	b.until("the hearing run is on its way", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return hearing != ""
	})
	b.reach.fail("watch", nil)
	b.later(time.Second)
	b.until("the hearing run is looked at in settling", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return shown
	})
	b.still()
	close(gate)
	answered := <-heard
	select {
	case a := <-answered:
		t.Fatalf("the service was answered %+v before the run heard anything", a)
	default:
	}
	b.answerTicket(hearing, standin.Heard, map[string]any{
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
	b.at("slack", "#help/1700.9")
}

func TestWorkWatchBeginsAgain(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.halt()
	b.reach.fail("watch", unreachable)
	b.restart()
	b.until("watching is said to have failed", func() bool { return b.logged("trying again in a second") })
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	b.corbie.Ended(t, run.Name)
	b.reach.fail("watch", nil)
	b.later(time.Second)
	b.settled(n, requests.Answered)
}

func TestWorkWaitingWorkKeptInStore(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, kit]\nruns: 1", 1)
	b := newBench(t, files)
	held, let := b.hold("socket", testkit.Say("mended"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	first := b.started(n, 1)
	b.until("the steward is held", func() bool { return held() == 1 })
	b.say(n, "kit", "and the fuse")
	other := b.chat("sam", "", "is the boiler serviced?")
	b.halt()
	let()
	b.corbie.Ended(t, first.Name)
	b.restart()
	// The run's ending is taken, and what Kit said while it went is given
	// to the next, before Sam's request, which waited its turn.
	second := b.started(n, 2)
	if said := b.input(second.Name)["said"]; said != "and the fuse" {
		t.Errorf("the next run was given %q", said)
	}
	b.settled(n, requests.Answered)
	b.started(other, 1)
	b.settled(other, requests.Answered)
}

func TestWorkLooksOnEventsOnly(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	b.until("the watch has listed", func() bool { return b.reach.count("watch") == 1 })
	// The answer is carried to Kit after the request is answered: each
	// run that carries it ends, and its ending is taken, before nothing
	// is happening. Only the bridge's listening run and the routines'
	// clocks go on.
	b.until("no run is noted as going but those that go on", func() bool {
		going, err := b.w.goingNotes()
		return err == nil && !slices.ContainsFunc(going, func(n *note) bool { return n.Kind != bridges.KindIn && n.Kind != KindClock })
	})
	b.still()
	// Each start sent is looked at once, Again after it went.
	b.later(Again)
	b.still()
	shows, lists := b.reach.count("show"), b.reach.count("list")
	// With nothing happening, the clock moving reads nothing: not Corbie,
	// and not the work's part of the store, which is out of reach.
	back := b.away(workTables...)
	logged := b.logCount()
	for range 4 {
		b.later(Again)
		b.still()
	}
	back()
	if lines := b.logsSince(logged); len(lines) > 0 {
		t.Errorf("with nothing happening, the store was looked at:\n%s", strings.Join(lines, "\n"))
	}
	if b.reach.count("show") != shows || b.reach.count("list") != lists || b.reach.count("watch") != 1 {
		t.Errorf("with nothing happening, Corbie was looked at: %d shows, %d listings, %d watches",
			b.reach.count("show")-shows, b.reach.count("list")-lists, b.reach.count("watch"))
	}
	b.say(n, "kit", "thanks")
	b.started(n, 2)
}

func TestWorkUnsentStartTriedAgainAlone(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	var mu sync.Mutex
	failing := true
	b.reach.mu.Lock()
	b.reach.starting = func(s client.Start) error {
		mu.Lock()
		defer mu.Unlock()
		if in, _ := s.Input.(map[string]any); failing && in["said"] == "alpha" {
			return unreachable
		}
		return nil
	}
	b.reach.mu.Unlock()
	alpha := b.chat("kit", "", "alpha")
	beta := b.chat("sam", "", "beta")
	b.settled(beta, requests.Answered)
	b.until("the house is told alpha's start failed", func() bool {
		return slices.ContainsFunc(b.entries(alpha, requests.KindBulldog), func(e requests.Entry) bool {
			return strings.Contains(e.Text, "could not start steward")
		})
	})
	mu.Lock()
	failing = false
	mu.Unlock()
	b.later(Again - time.Second)
	b.still()
	if runs := b.runs(alpha); len(runs) != 1 {
		t.Fatalf("alpha was tried again before its time: %+v", runs)
	}
	b.later(time.Second)
	b.started(alpha, 2)
	b.settled(alpha, requests.Answered)
}

func TestWorkOneItemsErrorKeptToIt(t *testing.T) {
	t.Parallel()
	t.Run("starts", oneStartsErrorKeptToIt)
	t.Run("calls", oneCallsErrorKeptToIt)
	t.Run("rounds", oneRoundsErrorKeptToIt)
}

func oneStartsErrorKeptToIt(t *testing.T) {
	b := newBench(t, house(t))
	b.reach.mu.Lock()
	b.reach.starting = func(s client.Start) error {
		if in, _ := s.Input.(map[string]any); in["said"] == "alpha" {
			return unreachable
		}
		return nil
	}
	b.reach.mu.Unlock()
	alpha := b.chat("kit", "", "alpha")
	beta := b.chat("sam", "", "beta")
	gamma := b.chat("sam", "", "gamma")
	b.settled(beta, requests.Answered)
	b.settled(gamma, requests.Answered)
	for _, n := range []int64{beta, gamma} {
		if slices.ContainsFunc(b.entries(n, requests.KindBulldog), func(e requests.Entry) bool { return e.Read != "" }) {
			t.Errorf("request %d was told of another's trouble: %v", n, b.thread(n))
		}
	}
	if runs := b.runs(alpha); len(runs) != 1 || runs[0].Status != unsent {
		t.Errorf("alpha's runs are %+v", runs)
	}
}

func oneCallsErrorKeptToIt(t *testing.T) {
	b := newBench(t, house(t))
	b.says(fixing("ask_person", map[string]any{"question": "which socket?"}))
	kitchen := b.chat("kit", "", "the kitchen socket sparked")
	hall := b.chat("kit", "", "the hall socket sparked")
	kq, hq := b.waitsOn(kitchen, requests.KindQuestion), b.waitsOn(hall, requests.KindQuestion)
	// Kit's answer cannot be given to the kitchen's waiting tool.
	of := func(run string) string {
		r, err := b.corbie.Show(ctx, run)
		if err != nil {
			return ""
		}
		return r.Labels[requests.LabelRequest]
	}
	b.reach.set(func(f *flaky) {
		f.replying = func(run string) error {
			if of(run) == fmt.Sprint(kitchen) {
				return unreachable
			}
			return nil
		}
	})
	for _, q := range []requests.Entry{kq, hq} {
		if _, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "the one by the door", nil, requests.Screens); err != nil {
			t.Fatal(err)
		}
	}
	if a := b.answered(hall); !strings.Contains(a.Text, "the one by the door") {
		t.Errorf("the hall was answered %q", a.Text)
	}
	b.still()
	if r := must(b.reqs.Get(kitchen)); r.Answered {
		t.Fatalf("the kitchen was answered though its answer was not given: %v", b.thread(kitchen))
	}
	b.reach.set(func(f *flaky) { f.replying = nil })
	b.later(Again)
	if a := b.answered(kitchen); !strings.Contains(a.Text, "the one by the door") {
		t.Errorf("the kitchen was answered %q", a.Text)
	}
}

func oneRoundsErrorKeptToIt(t *testing.T) {
	const lock = "Are the doors locked?"
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  - { at: \"18:00\", to: bins, say: \"Which bin goes out tonight?\" }",
		"  - { at: \"18:00\", to: bins, say: \"Which bin goes out tonight?\" }\n  - { at: \"18:00\", to: steward, say: \""+lock+"\" }", 1)
	b := newBench(t, files)
	// Both clocks wait for their time before it comes: a clock started
	// after it waits for the next.
	b.waitsForItsTime(whichBin)
	locking := b.waitsForItsTime(lock).ID
	b.reach.set(func(f *flaky) {
		f.replying = func(run string) error {
			if run == locking {
				return unreachable
			}
			return nil
		}
	})
	b.clock.Set(tonight)
	b.opened(whichBin, 1)
	b.waitsForItsTime(whichBin)
	lockRequest := b.opened(lock, 1)
	b.still()
	if c := b.clockOf(lock); c == nil || !slices.Contains(c.Tickets, shipped.ClockTicket) {
		t.Fatalf("the lock's clock was answered though its answer could not be sent: %+v", c)
	}
	b.reach.set(func(f *flaky) { f.replying = nil })
	b.later(Again)
	b.waitsForItsTime(lock)
	if opened := b.routined(lock); len(opened) != 1 || opened[0] != lockRequest {
		t.Errorf("the lock's routine opened %v", opened)
	}
}

func TestWorkNoLockAcrossCorbie(t *testing.T) {
	t.Parallel()
	t.Run("a start", noLockAcrossAStart)
	t.Run("a person deciding", noLockAcrossADecision)
}

func noLockAcrossAStart(t *testing.T) {
	b := newBench(t, house(t))
	gate, sending := make(chan struct{}), make(chan struct{})
	b.reach.mu.Lock()
	b.reach.starting = func(s client.Start) error {
		if in, _ := s.Input.(map[string]any); in["said"] == "slow" {
			close(sending)
			<-gate
		}
		return nil
	}
	b.reach.mu.Unlock()
	defer close(gate)
	slow := b.chat("kit", "", "slow")
	<-sending
	// The screens and a stop wait on nothing in Corbie meanwhile.
	other := b.chat("sam", "", "is the boiler serviced?")
	if _, err := b.reqs.View(other, "sam"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.w.Runs(slow); err != nil {
		t.Fatal(err)
	}
	if err := b.w.Stop(ctx, other, b.reqs.Person("sam")); err != nil {
		t.Fatal(err)
	}
}

func noLockAcrossADecision(t *testing.T) {
	b := newBench(t, house(t))
	b.says(fixing("cut_power", map[string]any{"circuit": "4"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	dec := b.held(n)
	gate, replying := make(chan struct{}), make(chan struct{})
	var once sync.Once
	b.reach.set(func(f *flaky) {
		f.replying = func(run string) error {
			if run == dec.Ask {
				once.Do(func() { close(replying) })
				<-gate
			}
			return nil
		}
	})
	// Sam's yes is taken while Corbie has not been answered yet.
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "yes", ""); err != nil || !r.Taken {
		t.Fatalf("Sam's yes: %v %v", r, err)
	}
	<-replying
	if _, err := b.reqs.View(n, "sam"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.w.Runs(n); err != nil {
		t.Fatal(err)
	}
	close(gate)
	holds(t, "the steward's answer", b.answered(n).Text, "Circuit 4 is off.")
}

func TestWhatCouldNotBeReadOnTheWayBackIsReadAgain(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket", testkit.Say("mended"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	b.until("the steward is held", func() bool { return held() == 1 })
	b.halt()
	waiting := b.chat("sam", "", "is the boiler serviced?")
	let()
	b.corbie.Ended(t, run.Name)
	back := b.away("work_runs", "work_requests", "work_calls")
	b.restart()
	b.until("settling could not read the runs", func() bool { return b.logged("the runs noted as going could not be read") })
	b.still()
	back()
	b.later(Again)
	b.settled(n, requests.Answered)
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 1 || answers[0].Text != "mended" {
		t.Errorf("the ending while away was taken as %v", b.thread(n))
	}
	b.started(waiting, 1)
	b.settled(waiting, requests.Answered)
}

func TestWorkStepsUntilStill(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	gate := make(chan struct{})
	var once sync.Once
	let := func() { once.Do(func() { close(gate) }) }
	defer let()
	asking := fixing("ask_person", map[string]any{"question": "which socket?"})
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "boiler") {
			<-gate
			return testkit.Say("serviced")
		}
		return asking(turn)
	})
	b.waitsForItsTime(whichBin)
	ending := b.chat("sam", "", "is the boiler serviced?")
	run := b.started(ending, 1)
	// A call whose input cannot be read waits on its ticket, not done.
	b.reach.fail("show call", unreachable)
	socket := b.chat("kit", "", "the kitchen socket sparked")
	b.until("the fixer's question waits on its ticket", func() bool {
		calls := b.calls(socket)
		return len(calls) == 1 && slices.Contains(calls[0].Tickets, shipped.CallTicket)
	})
	b.halt()
	b.reach.fail("show call", nil)
	// While nothing drives the office: a run ends, a routine's time comes,
	// and three requests are asked.
	let()
	b.corbie.Ended(t, run.Name)
	b.clock.Set(tonight)
	b.until("the bins' clock waits on its ticket", func() bool {
		c := b.clockOf(whichBin)
		return c != nil && slices.Contains(c.Tickets, shipped.ClockTicket)
	})
	clock := b.clockOf(whichBin)
	var ns []int64
	for i := range 3 {
		ns = append(ns, b.chat("kit", "", fmt.Sprint("job ", i)))
	}
	// One pass, given what the watch would list, does it all before it
	// returns.
	b.w.updates = append(b.w.updates, client.Update{})
	for _, r := range b.listedAll() {
		b.w.updates = append(b.w.updates, client.Update{Run: r})
	}
	b.w.updates = append(b.w.updates, client.Update{Listed: true})
	b.w.pass(ctx, false)
	for _, n := range ns {
		runs := b.runs(n)
		if len(runs) != 1 || runs[0].Status != going {
			t.Fatalf("when the pass returned, request %d had the runs %+v", n, runs)
		}
		if _, err := b.corbie.Show(ctx, runs[0].Name); errors.Is(err, client.ErrNoRun) {
			t.Errorf("when the pass returned, the start of request %d was not sent", n)
		}
	}
	if questions := b.entries(socket, requests.KindQuestion); len(questions) != 1 {
		t.Errorf("when the pass returned, the fixer's call was not done: %v", b.thread(socket))
	}
	if answers := b.entries(ending, requests.KindAnswer); len(answers) != 1 || answers[0].Text != "serviced" {
		t.Errorf("when the pass returned, the boiler's ending was not taken: %v", b.thread(ending))
	}
	if opened := b.routined(whichBin); len(opened) != 1 {
		t.Errorf("when the pass returned, the bins' routine had opened %v", opened)
	}
	tickets, err := b.corbie.Tickets(ctx, clock.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, kept, err := b.w.round(clock.ID, tickets[shipped.ClockTicket]); err != nil || !kept && tickets[shipped.ClockTicket] != "" {
		t.Errorf("when the pass returned, the bins' coming round was not acted on: %v %v", tickets, err)
	}
}

func TestWorkTripsGoSideBySide(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, kit]\nruns: 10", 1)
	b := newBench(t, files)
	var mu sync.Mutex
	held, leads := 0, make(chan struct{})
	letLeads := sync.OnceFunc(func() { close(leads) })
	defer letLeads()
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case turn.Number > 1:
			return testkit.Say("done")
		case strings.Contains(turn.Prompt(), "zebra"):
			return testkit.Use("ask_person", map[string]any{"question": "which one?", "who": ""})
		}
		mu.Lock()
		held++
		mu.Unlock()
		b.poke()
		<-leads
		return testkit.Use("tell", map[string]any{"text": "on it", "who": ""})
	})
	jobs := []int64{b.chat("kit", "", "job 0"), b.chat("kit", "", "job 1")}
	stoppedJob := b.chat("kit", "", "job 2")
	stopping := b.chat("kit", "", "the zebra crossing")
	q := b.waitsOn(stopping, requests.KindQuestion)
	b.until("the three leads are held", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return held == 3
	})
	b.halt()
	// While nothing drives the office, each lead tells Kit something, Kit
	// answers a question of a request, Sam stops that request and the
	// third job's, and three more requests are asked.
	letLeads()
	waiting := func(n int64) string {
		for _, c := range b.calls(n) {
			if slices.Contains(c.Tickets, shipped.CallTicket) {
				return c.ID
			}
		}
		return ""
	}
	b.until("the three tells wait on their tickets", func() bool {
		return waiting(jobs[0]) != "" && waiting(jobs[1]) != "" && waiting(stoppedJob) != ""
	})
	waits := map[string]bool{waiting(jobs[0]): true, waiting(jobs[1]): true}
	if _, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "the left one", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int64{stopping, stoppedJob} {
		if err := b.w.Stop(ctx, n, b.reqs.Person("sam")); err != nil {
			t.Fatal(err)
		}
	}
	unanswered := map[string]bool{waiting(stopping): true, waiting(stoppedJob): true}
	var asked []int64
	for _, text := range []string{"alpha", "beta", "gamma"} {
		asked = append(asked, b.chat("sam", "", text))
	}
	// Each start, and each answer, waits until all are on their way.
	starting, replying, most := 0, 0, map[string]int{}
	var unkept []string
	toStopping := false
	starts, replies := make(chan struct{}), make(chan struct{})
	letStarts, letReplies := sync.OnceFunc(func() { close(starts) }), sync.OnceFunc(func() { close(replies) })
	// Trips made one after another would wait here for ever.
	defer letStarts()
	defer letReplies()
	b.reach.set(func(f *flaky) {
		f.starting = func(s client.Start) error {
			if in, _ := s.Input.(map[string]any); !slices.Contains([]any{"alpha", "beta", "gamma"}, in["said"]) {
				return nil
			}
			mu.Lock()
			starting++
			most["starts"] = max(most["starts"], starting)
			if starting == 3 {
				letStarts()
			}
			mu.Unlock()
			<-starts
			return nil
		}
		f.replying = func(run string) error {
			if unanswered[run] {
				mu.Lock()
				toStopping = true
				mu.Unlock()
			}
			if !waits[run] {
				return nil
			}
			row, err := b.w.callOf(run)
			mu.Lock()
			if err != nil || row == nil || row.State != answered {
				unkept = append(unkept, run)
			}
			replying++
			most["replies"] = max(most["replies"], replying)
			if replying == 2 {
				letReplies()
			}
			mu.Unlock()
			<-replies
			return nil
		}
	})
	b.w.updates = append(b.w.updates, client.Update{})
	for _, r := range b.listedAll() {
		b.w.updates = append(b.w.updates, client.Update{Run: r})
	}
	b.w.updates = append(b.w.updates, client.Update{Listed: true})
	passed := make(chan struct{})
	go func() {
		defer close(passed)
		b.w.pass(ctx, false)
	}()
	b.until("the three starts are on their way at once", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return most["starts"] == 3
	})
	b.until("the two answers are on their way at once", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return most["replies"] == 2
	})
	<-passed
	if len(unkept) > 0 {
		t.Errorf("the answers to %v were sent before they were kept", unkept)
	}
	if toStopping {
		t.Error("a tool of a run being stopped was given an answer")
	}
	for _, n := range asked {
		if runs := b.runs(n); len(runs) != 1 || runs[0].Status != going {
			t.Errorf("when the pass returned, request %d had the runs %+v", n, runs)
		}
	}
	for _, n := range jobs {
		if told := b.entries(n, requests.KindTold); len(told) != 1 {
			t.Errorf("when the pass returned, request %d had told %v", n, b.thread(n))
		}
	}
}

func TestWorkEndingAppliedOnceToItsOwnRun(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.reach.mu.Lock()
	b.reach.lie = true
	b.reach.mu.Unlock()
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.until("the start that was sent is kept as not sent", func() bool {
		runs := b.runs(n)
		return len(runs) == 1 && runs[0].Status == unsent
	})
	sent := b.runs(n)[0].Name
	b.corbie.Ended(t, sent)
	b.reach.mu.Lock()
	b.reach.lie = false
	b.reach.mu.Unlock()
	b.later(Again)
	b.started(n, 2)
	b.settled(n, requests.Answered)
	// Seen ending again after a restart, nothing is taken twice.
	b.restart()
	b.say(n, "kit", "thanks")
	b.started(n, 3)
	b.settled(n, requests.Answered)
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 2 {
		t.Errorf("the request was answered %d times: %v", len(answers), b.thread(n))
	}
	if runs := b.runs(n); runs[0].Status != unsent || runs[1].Status != done || runs[2].Status != done {
		t.Errorf("the runs are %+v", runs)
	}
	// The run taken as not sent keeps how it ended, and is still not
	// sent.
	if stray := b.runs(n)[0]; !strings.Contains(stray.How, "Corbie ran it all the same") || stray.Ended.IsZero() {
		t.Errorf("the run Corbie ran all the same is kept as %+v", stray)
	}
}

func TestADeciderIsWaitedOnAgainAfterARestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("sam", "garden", "may the gate be opened?")
	b.settled(n, requests.Answered)
	held, let := b.hold("Order a socket", testkit.Say(`{"says": "yes", "note": "fine"}`))
	s := deciding("worker", "clerk", "garden", n, 7)
	first, cancel := context.WithCancel(ctx)
	go b.w.decider(first, s)
	b.until("the clerk is held", func() bool { return held() == 1 })
	b.halt()
	cancel()
	b.restart()
	got := make(chan string, 1)
	go func() {
		out, err := b.w.decider(ctx, s)
		got <- fmt.Sprintf("%s %v", out, err)
	}()
	let()
	if out := <-got; out != `"{\"says\": \"yes\", \"note\": \"fine\"}" <nil>` {
		t.Errorf("the decider gave %s", out)
	}
	if runs := b.listed(map[string]string{LabelKind: "decide"}); len(runs) != 1 || runs[0].Status != "done" {
		t.Errorf("the deciders' runs are %+v", runs)
	}
}

func TestWorkDeciderToldByTheWatch(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("sam", "garden", "may the gate be opened?")
	b.settled(n, requests.Answered)
	held, let := b.hold("Order a socket", testkit.Say(`{"says": "yes", "note": "fine"}`))
	got := make(chan string, 1)
	go func() {
		out, err := b.w.decider(ctx, deciding("worker", "clerk", "garden", n, 7))
		got <- fmt.Sprintf("%s %v", out, err)
	}()
	b.until("the clerk is held", func() bool { return held() == 1 })
	for range 3 {
		b.later(time.Minute)
		b.still()
	}
	// It is looked at once, as every start is, and never waited on.
	if shown := b.reach.count("show clerk") + b.reach.count("wait"); shown != 1 {
		t.Errorf("the clerk's run was read or waited on %d times while it went", shown)
	}
	let()
	if out := <-got; out != `"{\"says\": \"yes\", \"note\": \"fine\"}" <nil>` {
		t.Errorf("the decider gave %s", out)
	}
	if shown, waited := b.reach.count("show clerk"), b.reach.count("wait"); shown != 2 || waited != 0 {
		t.Errorf("the clerk's run, once ended, was read %d times and waited on %d", shown, waited)
	}
}

func TestWorkSubRequestsAtMostEightDeep(t *testing.T) {
	t.Parallel()
	files := house(t)
	for i := 1; i <= 9; i++ {
		files[fmt.Sprintf("a%d/team.yaml", i)] = fmt.Sprintf("name: A%d\npeople: [sam]\nlead: sam\n", i)
		files[fmt.Sprintf("a%d/w%d.yaml", i, i)] = fmt.Sprintf("agent: w%d\ninput: { said: string }\nmodel: usual\nprompt: ${input.said}\n", i)
	}
	b := newBench(t, files)
	n := b.chat("sam", "a1", "begin")
	deep := func(n int64) error {
		_, above, err := b.reqs.Top(n)
		if err != nil {
			t.Fatal(err)
		}
		return tooDeep(n, above)
	}
	for i := 2; i <= 9; i++ {
		if err := deep(n); err != nil {
			t.Fatalf("asking from level %d was refused: %v", i-2, err)
		}
		sub, err := b.reqs.Sub(n, b.reqs.Worker(fmt.Sprintf("w%d", i-1)), fmt.Sprintf("a%d", i), "and the next?", fmt.Sprint("ticket-", i))
		if err != nil {
			t.Fatal(err)
		}
		n = sub.Number
	}
	err := deep(n)
	var refusal requests.Refusal
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "already 8 levels") {
		t.Errorf("asking from eight levels down gave %v", err)
	}
}

func TestAnOlderStoreIsRefused(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if _, err := b.db.Exec(`UPDATE parts SET version = 0 WHERE area = ?`, Area); err != nil {
		t.Fatal(err)
	}
	_, err := New(b.db, b.reach, b.clock, identity, Areas{Keeper: b.keeper, Requests: b.reqs, Decisions: b.d, Changes: b.ch, Bridges: b.b})
	if err == nil || !strings.Contains(err.Error(), "remove .bulldog/bulldog.db and start again") {
		t.Errorf("an older store gave %v", err)
	}
}

func TestWhatWasReadIsHandedToTheRequests(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	// A bridge's run on the request, which nobody may watch, and a run
	// noted and never sent.
	if _, err := b.corbie.Start(ctx, waiter(t), client.Start{Labels: map[string]string{LabelBulldog: identity, LabelKind: "bridge-in",
		requests.LabelTeam: "office", requests.LabelRequest: fmt.Sprint(n)}}); err != nil {
		t.Fatal(err)
	}
	name, err := b.corbie.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.w.keep(&note{Name: name, Kind: KindWork, Team: "office", Request: n, Worker: "steward",
		Labels: map[string]string{LabelBulldog: identity}, Noted: b.clock.Now()}); err != nil {
		t.Fatal(err)
	}
	records, err := b.reqs.WhatWasRead(n, "sam")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("what was read is %+v", records)
	}
	if r := records[0]; r.Worker != "steward" || r.Gone || len(r.Events) < 10 || !strings.Contains(string(r.Events), `"step"`) {
		t.Errorf("the steward's run is handed in as %+v", r)
	}
	if r := records[1]; r.Run != name || !r.Gone {
		t.Errorf("the run never sent is handed in as %+v", r)
	}
	if records, err := b.reqs.WhatWasRead(n, "kit"); err != nil || len(records) != 2 {
		t.Errorf("Kit, of the house, is handed %v (%v)", records, err)
	}
}
