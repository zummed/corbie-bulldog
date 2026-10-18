package work

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// noting has the taker of a request write notes, one call a turn, and
// answer with every result it was given.
func noting(word string, writes ...map[string]any) (func(testkit.Turn) testkit.Reply, *[]string) {
	var results []string
	return func(turn testkit.Turn) testkit.Reply {
		if !strings.Contains(turn.Prompt(), word) {
			return testkit.Say("done")
		}
		if turn.Number > 1 {
			results = append(results, turn.Last())
		}
		if turn.Number <= len(writes) {
			return testkit.Use("write_notes", writes[turn.Number-1])
		}
		return testkit.Say(strings.Join(results, "\n"))
	}, &results
}

func TestWriteNotesWritesTheTeamsOwnNote(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	answer, _ := noting("fuse board", map[string]any{"name": "fuses", "text": "The fuse board is in the hall.\n", "why": "Kit said where it is"})
	b.says(answer)
	n := b.chat("kit", "", "the fuse board is in the hall, remember it")
	holds(t, "the steward's results", b.answered(n).Text, "The note fuses of the house is written")
	if got, err := os.ReadFile(filepath.Join(b.dir, "notes", "fuses.md")); err != nil || string(got) != "The fuse board is in the hall.\n" {
		t.Errorf("the house's note fuses holds %q (%v)", got, err)
	}
	history, err := b.ch.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	if c := history[0]; c.By != "steward" || c.Why != "Kit said where it is" || c.Request != n || !slices.Contains(c.Files, "notes/fuses.md") || c.DecidedBy != "" {
		t.Errorf("the note's commit is %+v", c)
	}
	// The tool says who reads notes, and that a secret goes in none.
	var described string
	for _, turn := range b.model.Turns() {
		for _, tool := range turn.ToolsGiven {
			if tool.Name == "write_notes" {
				described = tool.Description
			}
		}
	}
	holds(t, "write_notes as the model is given it", described, "the builder reads them", "whoever decides a change", "Never put a secret")

	// The garden's gardener writes the garden's note, never the house's.
	answer, _ = noting("shed key", map[string]any{"name": "fuses", "text": "The shed has no power.\n", "why": "Sam said so"})
	b.says(answer)
	g := b.chat("sam", "garden", "the shed key, and its power")
	b.answered(g)
	if got, err := os.ReadFile(filepath.Join(b.dir, "garden", "notes", "fuses.md")); err != nil || string(got) != "The shed has no power.\n" {
		t.Errorf("the garden's note fuses holds %q (%v)", got, err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.dir, "notes", "fuses.md")); string(got) != "The fuse board is in the hall.\n" {
		t.Errorf("the gardener wrote the house's note: %q", got)
	}
}

func TestWorkANoteReachesOnlyItsOwnTeam(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	answer, results := noting("shed key",
		map[string]any{"name": "../fuses", "text": "Up a level.", "why": "to reach the house"},
		map[string]any{"name": "old/fuses", "text": "Down a level.", "why": "to hide it"},
		map[string]any{"name": ".fuses", "text": "Hidden.", "why": "to hide it"})
	b.says(answer)
	before, err := b.ch.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	g := b.chat("sam", "garden", "the shed key")
	b.answered(g)
	if len(*results) != 3 {
		t.Fatalf("the gardener was given %q", *results)
	}
	for _, r := range *results {
		holds(t, "a note named with a path", r, "is not a note's name", "nothing was written")
	}
	after, err := b.ch.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("a note named with a path was committed: %+v", after[0])
	}
	for _, file := range []string{"fuses.md", "garden/fuses.md", "garden/notes/old/fuses.md", "garden/notes/.fuses.md"} {
		if _, err := os.Stat(filepath.Join(b.dir, file)); err == nil {
			t.Errorf("%s was written", file)
		}
	}
}

func TestOnlyAWorkerThatNamesTheNotesToolHasIt(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("tell", map[string]any{"text": "mended"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.answered(n)
	offered := map[string][]string{}
	for _, turn := range b.model.Turns() {
		switch {
		case strings.Contains(turn.Prompt(), "socket sparked"):
			offered["steward"] = turn.Tools
		case turn.Prompt() == "mend the kitchen socket":
			offered["fixer"] = turn.Tools
		}
	}
	for _, tool := range []string{"write_notes", "ask_team", "ask_person", "tell"} {
		if !slices.Contains(offered["steward"], tool) {
			t.Errorf("the steward is not offered %s: %v", tool, offered["steward"])
		}
	}
	for _, tool := range []string{"ask_team", "ask_person", "tell"} {
		if !slices.Contains(offered["fixer"], tool) {
			t.Errorf("the fixer is not offered %s: %v", tool, offered["fixer"])
		}
	}
	if slices.Contains(offered["fixer"], "write_notes") {
		t.Errorf("the fixer, which names only the talk tools, is offered write_notes")
	}

	// Payments names the notes tool nowhere: its notes stay as they were,
	// and the ledger is given them all the same.
	p := newBench(t, payments(t))
	notes, err := p.office().Notes("payments")
	if err != nil || len(notes) == 0 {
		t.Fatalf("payments' notes are %v (%v)", notes, err)
	}
	r := p.chat("lee", "payments", "was 4411 charged twice?")
	run := p.started(r, 1)
	p.settled(r, requests.Answered)
	holds(t, "the ledger's context", p.input(run.Name)["context"].(string), "## "+notes[0].Name)
	after, err := p.office().Notes("payments")
	if err != nil || len(after) != len(notes) || after[0].Text != notes[0].Text {
		t.Errorf("payments' notes changed: %v", after)
	}
}
