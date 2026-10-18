package work

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// building has the steward hand what Kit wants to the builder, which
// makes its calls one a turn and says what it last heard.
func building(calls ...testkit.Reply) func(testkit.Turn) testkit.Reply {
	return func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "remember the fuses") && turn.Number == 1:
			return testkit.Use("builder", map[string]any{"wanted": "a note of where the fuses are"})
		case turn.Prompt() == "a note of where the fuses are" && turn.Number <= len(calls):
			return calls[turn.Number-1]
		case turn.Prompt() == "a note of where the fuses are":
			return testkit.Say("builder heard: " + turn.Last())
		}
		return testkit.Say("steward heard: " + turn.Last())
	}
}

var (
	writeFuses = testkit.Use("office_write", map[string]any{"path": "notes/fuses.md", "text": "The fuse board is in the hall.\n"})
	putFuses   = testkit.Use("office_put", map[string]any{"why": "Remember where the fuse board is"})
)

// change waits until a change put in a request waits on people, and
// gives it.
func (b *bench) change(n int64) *decisions.Decision {
	b.t.Helper()
	var dec *decisions.Decision
	b.until(fmt.Sprintf("a change waits on people in request %d", n), func() bool {
		for _, d := range must(b.d.OfRequest(n)) {
			if d.Kind == decisions.OfChange && d.Ended == "" && len(d.Asked) > 0 {
				dec = d
			}
		}
		return dec != nil
	})
	return dec
}

func TestTheBuildersToolsAreHandedToChanges(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(building(testkit.Use("office_files", map[string]any{})))
	n := b.chat("kit", "", "remember the fuses")
	holds(t, "the steward's answer", b.answered(n).Text, "steward heard: builder heard:", "office.yaml", "steward.yaml")
}

func TestAChangePutIsWaitedOnUntilItIsDecided(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(building(writeFuses, putFuses))
	n := b.chat("kit", "", "remember the fuses")
	dec := b.change(n)
	if dec.Put.By != "builder" || dec.Put.Request != n || strings.Join(dec.Asked, ",") != "sam" {
		t.Errorf("the change is put as %+v", dec.Put)
	}
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 0 {
		t.Fatalf("the builder did not wait on its change: %v", b.thread(n))
	}
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "yes", "good idea"); err != nil || !r.Taken {
		t.Fatalf("Sam's yes: %v %v", r, err)
	}
	holds(t, "the steward's answer", b.answered(n).Text, "builder heard: The change was written, with the yes of Sam,", "good idea")
	if got, err := os.ReadFile(filepath.Join(b.dir, "notes", "fuses.md")); err != nil || string(got) != "The fuse board is in the hall.\n" {
		t.Errorf("the note holds %q (%v)", got, err)
	}
	if runs := b.runs(n); len(runs) != 1 {
		t.Errorf("the outcome, given to the waiting tool, started a run: %+v", runs)
	}
}

func TestAChangeDecidedLateIsSaidInTheRequest(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(building(writeFuses, putFuses))
	n := b.chat("kit", "", "remember the fuses")
	dec := b.change(n)
	b.later(week)
	holds(t, "the steward's first answer", b.waitsOn(n, requests.KindAnswer).Text, "still open")
	b.until("the steward's run ends", func() bool { return b.runs(n)[0].Status == done })
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "refuse", "not now"); err != nil || !r.Taken {
		t.Fatalf("Sam's refusal: %v %v", r, err)
	}
	late := b.waitsOn(n, requests.KindLate)
	if late.By.Kind != requests.IsBulldog || late.To[0].Name != "builder" || !strings.Contains(late.Text, "The change was refused by Sam,") {
		t.Errorf("the late outcome is %+v", late)
	}
	run := b.started(n, 2)
	holds(t, "the steward's second run", fmt.Sprint(b.input(run.Name)["said"]), "The change was refused by Sam,", "not now")
}

func TestAChangeStopsTheRunsOfTheWorkersItTookAway(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	_, let := b.hold("shed", testkit.Say("done"))
	defer let()
	g := b.chat("sam", "garden", "is the shed locked?")
	gardener := b.started(g, 1)
	b.w.Told(changes.Outcome{Put: 99, By: "builder", Ended: changes.Written, Gone: []string{"gardener"}})
	if ended := b.corbie.Ended(t, gardener.Name); ended.Status != "cancelled" {
		t.Errorf("the gardener's run ended %s", ended.Status)
	}
	b.settled(g, requests.Answered)
	holds(t, "the garden's thread", strings.Join(b.thread(g), "\n"), "gardener was taken out of the office by a change")
}
