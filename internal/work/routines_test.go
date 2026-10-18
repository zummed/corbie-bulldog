package work

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

// The house's routines, as its files say them, and when each first comes
// after the test's clock starts, at ten in London on Monday 5 October.
const (
	walkRound = "Walk round: what is still open?"
	whichBin  = "Which bin goes out tonight?"
	weekend   = "What needs doing before the weekend?"
)

var (
	// Monday 12 October, nine in London.
	nextMonday = time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	// Monday 5 October, six in the evening in London.
	tonight = time.Date(2026, 10, 5, 17, 0, 0, 0, time.UTC)
	// Friday 9 October, four in the afternoon in London.
	friday = time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
)

// routined gives the requests a routine has opened, oldest first.
func (b *bench) routined(say string) []int64 {
	b.t.Helper()
	var ns []int64
	for n := int64(1); ; n++ {
		r, err := b.reqs.Get(n)
		if errors.Is(err, requests.ErrNotThere) {
			return ns
		}
		if err != nil {
			b.t.Fatal(err)
		}
		if r.Origin.Kind == requests.ByRoutine && r.Origin.Say == say {
			ns = append(ns, n)
		}
	}
}

// opened stays until a routine has opened so many requests, and gives
// the latest.
func (b *bench) opened(say string, count int) int64 {
	b.t.Helper()
	var ns []int64
	b.until(fmt.Sprintf("the routine %q has opened %d requests", say, count), func() bool {
		ns = b.routined(say)
		return len(ns) >= count
	})
	if len(ns) != count {
		b.t.Fatalf("the routine %q opened %v", say, ns)
	}
	return ns[count-1]
}

// clockOf gives the clock going for a routine of the office, by what it
// says, or nil.
func (b *bench) clockOf(say string) *client.Run {
	b.t.Helper()
	for key, wr := range routines(b.office()) {
		if wr.routine.Say != say {
			continue
		}
		for _, r := range b.going(KindClock, "") {
			if r.Labels[LabelRoutine] == key {
				return r
			}
		}
	}
	return nil
}

// waitsForItsTime stays until a routine's clock waits on its timer again,
// and on no ticket.
func (b *bench) waitsForItsTime(say string) *client.Run {
	b.t.Helper()
	var r *client.Run
	b.until(fmt.Sprintf("the clock of %q waits for its time", say), func() bool {
		r = b.clockOf(say)
		return r != nil && r.Timers == 1 && !slices.Contains(r.Tickets, shipped.ClockTicket)
	})
	return r
}

func TestEachRoutineHasOneClock(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	for _, say := range []string{walkRound, whichBin, weekend} {
		b.waitsForItsTime(say)
	}
	clocks := b.going(KindClock, "")
	if len(clocks) != 3 {
		t.Fatalf("%d clocks go", len(clocks))
	}
	monday, err := b.corbie.Show(ctx, b.clockOf(walkRound).ID)
	if err != nil {
		t.Fatal(err)
	}
	if due := monday.Input.(map[string]any)["due"]; monday.Machine != "clock" || due != "mon 09:00 Europe/London" {
		t.Errorf("the Monday clock is %s, due %v", monday.Machine, due)
	}
	if c := b.clockOf(weekend); c.Labels[LabelKind] != KindClock || c.Labels["team"] != "garden" {
		t.Errorf("the garden's clock is labelled %v", c.Labels)
	}
	// Every other run Bulldog starts is of a definition of the office as
	// it is read, with a model named by one word filled in.
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	b.listening("telegram")
	o := b.office()
	var drafts []*machine.Draft
	for _, w := range o.Workers {
		drafts = append(drafts, w.Draft)
	}
	for _, br := range o.Bridges {
		drafts = append(drafts, br.In, br.Out)
	}
	written := map[string]bool{}
	for _, d := range drafts {
		if d != nil {
			written[string(d.Definition.Body)] = true
		}
	}
	others := 0
	for _, r := range b.listed(nil) {
		if r.Labels[LabelKind] == KindClock {
			continue
		}
		others++
		if def, err := b.corbie.Definition(ctx, r.ID); err != nil || !written[string(def.Body)] {
			t.Errorf("the run %s, of kind %s, was started from a definition that is not the office's: %v", r.ID, r.Labels[LabelKind], err)
		}
	}
	if others != 2 {
		t.Errorf("%d runs other than clocks were looked at", others)
	}
}

func TestARoutineOpensItsRequestWhenItsTimeComes(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "Walk round") {
			return testkit.Say("Nothing is open.")
		}
		return testkit.Say("done")
	})
	b.waitsForItsTime(walkRound)
	b.clock.Set(nextMonday)
	n := b.opened(walkRound, 1)
	r := must(b.reqs.Get(n))
	if r.Team != "office" || r.Asker.Kind != requests.IsTeam || r.Origin.At != "mon 09:00" || r.Origin.To != "steward" {
		t.Errorf("the routine's request is %+v", r)
	}
	run := b.started(n, 1)
	if said := b.input(run.Name)["said"]; run.Worker != "steward" || said != walkRound {
		t.Errorf("the routine started %s with %q", run.Worker, said)
	}
	b.waitsForItsTime(walkRound)

	// Its answer goes to Sam and Kit.
	answer := b.answered(n)
	var people []string
	for _, d := range must(b.reqs.Deliveries(answer.ID)) {
		people = append(people, d.Person)
	}
	if slices.Sort(people); answer.Text != "Nothing is open." || !slices.Equal(people, []string{"kit", "sam"}) {
		t.Errorf("the routine's answer %q goes to %v", answer.Text, people)
	}
}

func TestWorkRoutineActedOnOnce(t *testing.T) {
	t.Parallel()
	for _, noted := range []bool{true, false} {
		b := newBench(t, house(t))
		_, let := b.hold("Walk round", testkit.Say("Nothing is open."))
		b.waitsForItsTime(walkRound)
		// The ticket cannot be answered: Bulldog stops after opening the
		// request, with the coming round noted, or before it was noted.
		b.reach.fail("reply", unreachable)
		b.clock.Set(nextMonday)
		n := b.opened(walkRound, 1)
		b.until("the clock's ticket could not be answered", func() bool { return b.logged("could not be acted on") })
		b.halt()
		if !noted {
			if _, err := b.db.Exec(`DELETE FROM work_rounds`); err != nil {
				t.Fatal(err)
			}
		}
		b.reach.fail("reply", nil)
		b.restart()
		b.waitsForItsTime(walkRound)
		if got := b.routined(walkRound); !slices.Equal(got, []int64{n}) {
			t.Errorf("with the round noted %v, the routine opened %v", noted, got)
		}
		let()
		b.settled(n, requests.Answered)
	}
}

func TestWorkRoutineWhileAwayActedOnOnce(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	for _, say := range []string{walkRound, whichBin, weekend} {
		b.waitsForItsTime(say)
	}
	b.halt()
	b.later(3 * 7 * 24 * time.Hour)
	b.restart()
	for _, say := range []string{walkRound, whichBin, weekend} {
		n := b.opened(say, 1)
		b.settled(n, requests.Answered)
		b.waitsForItsTime(say)
	}
	b.still()
	for _, say := range []string{walkRound, whichBin, weekend} {
		if got := b.routined(say); len(got) != 1 {
			t.Errorf("the routine %q opened %v", say, got)
		}
	}
}

func TestWorkRoutineLetGoWhileLastWorks(t *testing.T) {
	t.Parallel()
	const morning = "Morning round: is anything on?"
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "routines:\n",
		"routines:\n  - { at: \"07:00\", to: steward, say: \""+morning+"\" }\n", 1)
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "Morning round") && turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"question": "Is the boiler on?", "who": "sam"})
		}
		return testkit.Say("done")
	})
	// Tuesday 6 October, seven in London.
	tuesday := time.Date(2026, 10, 6, 6, 0, 0, 0, time.UTC)
	b.waitsForItsTime(morning)
	b.clock.Set(tuesday)
	n := b.opened(morning, 1)
	question := b.waitsOn(n, requests.KindQuestion)
	b.waitsForItsTime(morning)

	// The next morning its request still waits on Sam: the time is let go
	// by.
	b.clock.Set(tuesday.Add(24 * time.Hour))
	b.until("the next morning is let go by", func() bool {
		return b.logged(fmt.Sprintf("the routine at 07:00 of office came while its last request, %d, is still open", n))
	})
	b.waitsForItsTime(morning)
	if got := b.routined(morning); !slices.Equal(got, []int64{n}) {
		t.Errorf("the routine opened %v while its last request was open", got)
	}
	// Once that is answered, the next morning opens another.
	if _, err := b.reqs.Reply(question.ID, b.reqs.Person("sam"), "It is on.", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	b.settled(n, requests.Answered)
	b.clock.Set(tuesday.Add(48 * time.Hour))
	b.opened(morning, 2)

	// A worker that cannot run: its time is let go by too.
	b.waitsForItsTime(whichBin)
	before := b.routined(whichBin)
	for _, n := range before {
		b.settled(n, requests.Answered)
	}
	b.rewrite(map[string]string{"bins.yaml": "flow: bins\nstages: nothing\n"})
	b.clock.Set(tuesday.Add(72 * time.Hour))
	b.until("the bins' time is let go by", func() bool { return b.logged("bins cannot run") })
	b.waitsForItsTime(whichBin)
	if got := b.routined(whichBin); !slices.Equal(got, before) {
		t.Errorf("the bins routine opened %v, and before %v", got, before)
	}
}

func TestWorkClockNeverTwiceForOneTime(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.waitsForItsTime(whichBin)
	b.clock.Set(tonight)
	n := b.opened(whichBin, 1)
	b.waitsForItsTime(whichBin)
	b.later(time.Millisecond)
	b.still()
	if got := b.routined(whichBin); !slices.Equal(got, []int64{n}) {
		t.Errorf("the bins routine came %v for one time", got)
	}
	b.settled(n, requests.Answered)
	b.clock.Set(tonight.Add(24 * time.Hour))
	b.opened(whichBin, 2)
}

func TestWorkRoutineRequestIsTheTeams(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(turn testkit.Turn) testkit.Reply { return testkit.Say("Mow the lawn.") })
	b.waitsForItsTime(weekend)
	b.clock.Set(friday)
	n := b.opened(weekend, 1)
	if r := must(b.reqs.Get(n)); r.Team != "garden" || r.Asker.Kind != requests.IsTeam || r.Asker.Name != "garden" {
		t.Errorf("the garden's routine asked %+v", r)
	}
	readers := must(b.reqs.ReadBy(n))
	if len(readers) != 1 || readers[0].Name != "sam" {
		t.Errorf("the garden's routine is read by %v", readers)
	}
	answer := b.answered(n)
	ds := must(b.reqs.Deliveries(answer.ID))
	if len(ds) != 1 || ds[0].Person != "sam" {
		t.Errorf("the gardener's answer goes to %v", ds)
	}
}

func TestAClockFollowsItsRoutine(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	monday := b.waitsForItsTime(walkRound)
	bins := b.waitsForItsTime(whichBin)
	office := b.file("office.yaml")

	// A changed routine is a new one: its old clock is cancelled.
	changed := strings.Replace(office, walkRound, "Walk round: what is still broken?", 1)
	b.rewrite(map[string]string{"office.yaml": changed})
	if ended := b.corbie.Ended(t, monday.ID); ended.Status != "cancelled" {
		t.Errorf("the old Monday clock ended %s", ended.Status)
	}
	b.waitsForItsTime("Walk round: what is still broken?")

	// A routine taken out: its clock is cancelled.
	b.rewrite(map[string]string{"office.yaml": strings.Replace(changed, "  - { at: \"18:00\", to: bins, say: \"Which bin goes out tonight?\" }\n", "", 1)})
	if ended := b.corbie.Ended(t, bins.ID); ended.Status != "cancelled" {
		t.Errorf("the bins clock ended %s", ended.Status)
	}

	// A clock cancelled from outside is started again.
	garden := b.waitsForItsTime(weekend)
	if _, err := b.corbie.Cancel(ctx, garden.ID, 0); err != nil {
		t.Fatal(err)
	}
	b.corbie.Ended(t, garden.ID)
	b.until("the garden's clock is started again", func() bool {
		c := b.clockOf(weekend)
		return c != nil && c.ID != garden.ID
	})
	if len(b.going(KindClock, "")) != 2 {
		t.Errorf("the clocks going are %v", b.going(KindClock, ""))
	}
}

func TestWorkClocksSyncedOnReadOnly(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.waitsForItsTime(walkRound)
	b.still()
	lists := b.reach.count("list")
	for range 4 {
		b.later(Again)
	}
	b.still()
	if got := b.reach.count("list"); got != lists {
		t.Errorf("with nothing read, Corbie's runs were listed %d times", got-lists)
	}
	b.rewrite(map[string]string{"office.yaml": strings.Replace(b.file("office.yaml"), "everything in it, but the garden.", "everything in it.", 1)})
	b.still()
	if got := b.reach.count("list"); got == lists {
		t.Errorf("reading the office again did not look for the clocks")
	}
	lists = b.reach.count("list")
	for range 4 {
		b.later(Again)
	}
	b.still()
	if got := b.reach.count("list"); got != lists {
		t.Errorf("after the office was read, Corbie's runs were listed %d times more", got-lists)
	}
}
