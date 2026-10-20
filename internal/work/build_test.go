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
	"github.com/zummed/corbie-bulldog/shipped"
	"go.yaml.in/yaml/v3"
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

// asked waits until a held action in a request waits on people, and
// gives its decision.
func (b *bench) asked(n int64) *decisions.Decision {
	b.t.Helper()
	var dec *decisions.Decision
	b.until(fmt.Sprintf("a held action waits on people in request %d", n), func() bool {
		for _, d := range must(b.d.OfRequest(n)) {
			if d.Kind == decisions.OfAction && d.Ended == "" && len(d.Asked) > 0 {
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

func TestAHeldPutIsWrittenOnceItsJobRuns(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(building(writeFuses, putFuses))
	n := b.chat("kit", "", "remember the fuses")
	dec := b.asked(n)
	if dec.Put == nil || dec.Put.By != "builder" || dec.Put.Request != n || dec.Team != "office" || dec.Job.Name != "office_put" || strings.Join(dec.Asked, ",") != "sam" {
		t.Fatalf("the put is held as %+v", dec)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "notes", "fuses.md")); err == nil {
		t.Fatal("the held put was written before its yes")
	}
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "yes", "good idea"); err != nil || !r.Taken {
		t.Fatalf("Sam's yes: %v %v", r, err)
	}
	holds(t, "the steward's answer", b.answered(n).Text, "builder heard: The change was written, with the yes of Sam,", "good idea")
	if got, err := os.ReadFile(filepath.Join(b.dir, "notes", "fuses.md")); err != nil || string(got) != "The fuse board is in the hall.\n" {
		t.Errorf("the note holds %q (%v)", got, err)
	}
	if runs := b.runs(n); len(runs) != 1 {
		t.Errorf("the put started a run: %+v", runs)
	}
}

func TestAHeldPutRefusedIsDenied(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(building(writeFuses, putFuses))
	n := b.chat("kit", "", "remember the fuses")
	dec := b.asked(n)
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), "refuse", "not now"); err != nil || !r.Taken {
		t.Fatalf("Sam's refusal: %v %v", r, err)
	}
	holds(t, "the steward's answer", b.answered(n).Text, "builder heard:", "not now")
	if _, err := os.Stat(filepath.Join(b.dir, "notes", "fuses.md")); err == nil {
		t.Error("a refused put was written")
	}
}

// unheld gives a kit Bulldog ships as an office's own copy at tools/, its
// call named from there, with a tool's approve taken away or given.
func unheld(t *testing.T, kit, tool string, approve bool) string {
	t.Helper()
	contents, _ := shipped.File(kit)
	text := strings.ReplaceAll(string(contents), "../../call.yaml", "../.bulldog/call.yaml")
	var tools yaml.Node
	if err := yaml.Unmarshal([]byte(text), &tools); err != nil {
		t.Fatal(err)
	}
	root := tools.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != tool {
			continue
		}
		var def map[string]any
		if err := root.Content[i+1].Decode(&def); err != nil {
			t.Fatal(err)
		}
		job := def["job"].(map[string]any)
		delete(job, "approve")
		if approve {
			job["approve"] = true
		}
		if err := root.Content[i+1].Encode(def); err != nil {
			t.Fatal(err)
		}
	}
	out, err := yaml.Marshal(&tools)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestChangesAWriteWaitsByItsToolsFileAlone(t *testing.T) {
	t.Parallel()
	// As the build kit ships it, the builder's put is held, and written
	// only on Sam's yes: TestAHeldPutIsWrittenOnceItsJobRuns. The house's
	// own copy without approve has it written at once, with no decision.
	files := house(t)
	files["tools/build.yaml"] = unheld(t, shipped.Build, "office_put", false)
	files["builder.yaml"] = strings.Replace(files["builder.yaml"], "./.bulldog/tools/build/build.yaml", "./tools/build.yaml", 1)
	b := newBench(t, files)
	b.says(building(writeFuses, putFuses))
	n := b.chat("kit", "", "remember the fuses")
	holds(t, "the steward's answer", b.answered(n).Text, "builder heard: The change was written, and is kept as the commit")
	if got, err := os.ReadFile(filepath.Join(b.dir, "notes", "fuses.md")); err != nil || string(got) != "The fuse board is in the hall.\n" {
		t.Errorf("the note holds %q (%v)", got, err)
	}
	if decs := must(b.d.OfRequest(n)); len(decs) != 0 {
		t.Errorf("an unheld put was decided: %+v", decs)
	}
	// As the notes kit ships it, the steward's note is written at once;
	// the house's copy that marks it approve has it held for the house's
	// write_notes list, and written on Sam's yes.
	note := testkit.Use("write_notes", map[string]any{"name": "fuses", "text": "In the hall.\n", "why": "Where the fuses are"})
	noting := func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return note
		}
		return testkit.Say("steward heard: " + turn.Last())
	}
	plain := newBench(t, house(t))
	plain.says(noting)
	m := plain.chat("kit", "", "note the fuses")
	holds(t, "the steward's answer", plain.answered(m).Text, "steward heard: The note fuses of the house is written")
	files = house(t)
	files["tools/notes.yaml"] = unheld(t, "tools/notes/notes.yaml", "write_notes", true)
	files["steward.yaml"] = strings.Replace(files["steward.yaml"], "./.bulldog/tools/notes/notes.yaml", "./tools/notes.yaml", 1)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  write_notes: [kit]\n", 1)
	held := newBench(t, files)
	held.says(noting)
	m = held.chat("sam", "", "note the fuses")
	dec := held.asked(m)
	if dec.Job.Name != "write_notes" || dec.Put != nil || strings.Join(dec.Asked, ",") != "kit" {
		t.Fatalf("the note is held as %+v", dec)
	}
	if _, err := os.Stat(filepath.Join(held.dir, "notes", "fuses.md")); err == nil {
		t.Fatal("the held note was written before its yes")
	}
	if r, err := held.d.Decide(ctx, dec.ID, held.reqs.Person("kit"), "yes", ""); err != nil || !r.Taken {
		t.Fatalf("Kit's yes: %v %v", r, err)
	}
	holds(t, "the steward's answer", held.answered(m).Text, "steward heard: The note fuses of the house is written")
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
