package work

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// looking has the lead of a request make calls, one a turn, and answer
// with every result it was given, one to a part.
func looking(word string, calls ...testkit.Reply) func(testkit.Turn) testkit.Reply {
	var results []string
	return func(turn testkit.Turn) testkit.Reply {
		if !strings.Contains(turn.Prompt(), word) {
			return testkit.Say("done")
		}
		if turn.Number > 1 {
			results = append(results, turn.Last())
		}
		if turn.Number <= len(calls) {
			return calls[turn.Number-1]
		}
		return testkit.Say(strings.Join(results, "\n---\n"))
	}
}

// withDocs is the house with docs of its own and the garden's, and the
// steward and the gardener naming the docs tools.
func withDocs(t *testing.T) map[string]string {
	files := house(t)
	files["docs/boiler.md"] = "# The boiler\n\nService it in May.\nThe key is in the hall.\n"
	files["docs/rooms/kitchen.md"] = "# The kitchen\nThe fuse is 4.\n"
	files["garden/docs/boiler.md"] = "# The shed's heater\nOil, not gas.\n"
	files["garden/docs/tools/mowers.md"] = "Mowers are serviced in March.\n"
	files["steward.yaml"] = strings.Replace(files["steward.yaml"], "./.bulldog/tools/notes/notes.yaml]",
		"./.bulldog/tools/notes/notes.yaml, ./.bulldog/tools/docs/read.yaml, ./.bulldog/tools/docs/write.yaml]", 1)
	files["garden/gardener.yaml"] = strings.Replace(files["garden/gardener.yaml"], "../.bulldog/tools/notes/notes.yaml]",
		"../.bulldog/tools/notes/notes.yaml, ../.bulldog/tools/docs/read.yaml, ../.bulldog/tools/docs/write.yaml]\nask: { machine: ../.bulldog/ask.yaml }", 1)
	return files
}

func TestDocsToolsReadTheTeamsThenThoseAroundIt(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDocs(t))
	if tools := b.office().Workers["gardener"].Tools; !slices.Contains(tools, "read_doc") {
		t.Fatalf("the gardener, naming the docs tools, holds %v", tools)
	}
	if tools := b.office().Workers["fixer"].Tools; slices.Contains(tools, "list_docs") {
		t.Errorf("the fixer, naming no docs tools, holds %v", tools)
	}
	b.says(looking("heater",
		testkit.Use("list_docs", map[string]any{}),
		testkit.Use("read_doc", map[string]any{"name": "boiler"}),
		testkit.Use("search_docs", map[string]any{"words": "FUSE"}),
		testkit.Use("read_doc", map[string]any{"name": "rooms/kitchen.md", "from": 2}),
		testkit.Use("read_doc", map[string]any{"name": "rooms/kitchen", "from": 9})))
	n := b.chat("sam", "garden", "the heater in the shed?")
	parts := strings.Split(b.answered(n).Text, "\n---\n")
	if len(parts) != 5 {
		t.Fatalf("the gardener was given %q", parts)
	}
	if want := "boiler, of The garden, 34 characters: The shed's heater\ntools/mowers, of The garden, 30 characters\n" +
		"rooms/kitchen, of The house, 29 characters: The kitchen\n"; parts[0] != want {
		t.Errorf("docs gave the gardener %q, not %q", parts[0], want)
	}
	if want := "boiler, of The garden, lines 1 to 2 of 2:\n# The shed's heater\nOil, not gas.\n"; parts[1] != want {
		t.Errorf("read_doc gave the gardener %q, not %q", parts[1], want)
	}
	if want := "rooms/kitchen:2, of The house: The fuse is 4.\n"; parts[2] != want {
		t.Errorf("search_docs gave the gardener %q, not %q", parts[2], want)
	}
	if want := "rooms/kitchen, of The house, lines 2 to 2 of 2:\nThe fuse is 4.\n"; parts[3] != want {
		t.Errorf("read_doc from a line gave the gardener %q, not %q", parts[3], want)
	}
	holds(t, "read_doc past the end", parts[4], "rooms/kitchen, of The house, has 2 lines, so nothing is from line 9 on.")

	// The steward reads the house's boiler, and nothing of the garden's.
	b.says(looking("boiler", testkit.Use("list_docs", map[string]any{}), testkit.Use("read_doc", map[string]any{"name": "boiler"})))
	h := b.chat("kit", "", "when is the boiler serviced?")
	parts = strings.Split(b.answered(h).Text, "\n---\n")
	if want := "boiler, of The house, 57 characters: The boiler\nrooms/kitchen, of The house, 29 characters: The kitchen\n"; parts[0] != want {
		t.Errorf("docs gave the steward %q, not %q", parts[0], want)
	}
	holds(t, "the house's boiler", parts[1], "boiler, of The house, lines 1 to 4 of 4:\n# The boiler\n\nService it in May.")
}

func TestDocsReadOnFromWhereTheLimitCuts(t *testing.T) {
	t.Parallel()
	files := withDocs(t)
	long := strings.Repeat(strings.Repeat("x", 99)+"\n", 300)
	files["garden/docs/log.md"] = long
	b := newBench(t, files)
	b.says(looking("log",
		testkit.Use("read_doc", map[string]any{"name": "log"}),
		testkit.Use("read_doc", map[string]any{"name": "log", "from": 201}),
		testkit.Use("search_docs", map[string]any{"words": "xx"})))
	n := b.chat("sam", "garden", "read the log")
	parts := strings.Split(b.answered(n).Text, "\n---\n")
	holds(t, "the first part of the log", parts[0], "log, of The garden, lines 1 to 200 of 300:\n", "100 more lines after line 200; read on from line 201.")
	holds(t, "the rest of the log", parts[1], "log, of The garden, lines 201 to 300 of 300:\n")
	if strings.Contains(parts[1], "more lines") {
		t.Errorf("the rest of the log says there is more: %q", parts[1][len(parts[1])-80:])
	}
	holds(t, "a search past the limit", parts[2], "log:1, of The garden: xxx", "more lines hold it; search for something narrower")
}

func TestWorkADocsCallReachesOnlyDocs(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDocs(t))
	// A file beside the docs, and a link from them to a note.
	if err := os.Symlink(filepath.Join(b.dir, "garden", "notes", "shed.md"), filepath.Join(b.dir, "garden", "docs", "shed.md")); err != nil {
		t.Fatal(err)
	}
	var calls []testkit.Reply
	refused := []string{"../notes/shed", "../../notes/rooms", "/etc/hostname", "../team", "shed", ".hidden", "tools/../../notes/lawn"}
	for _, name := range refused {
		calls = append(calls, testkit.Use("read_doc", map[string]any{"name": name}))
	}
	calls = append(calls, testkit.Use("search_docs", map[string]any{"words": "hook by the back door"}))
	b.says(looking("shed", calls...))
	n := b.chat("sam", "garden", "where is the shed key?")
	parts := strings.Split(b.answered(n).Text, "\n---\n")
	if len(parts) != len(refused)+1 {
		t.Fatalf("the gardener was given %q", parts)
	}
	for i, name := range refused {
		holds(t, "read_doc of "+name, parts[i], "No doc you may read is called", "nothing was read")
	}
	holds(t, "a search of the garden's notes", parts[len(refused)], `No line of the docs you may read holds "hook by the back door".`)

	// The steward reads nothing of the garden's, a team within it.
	b.says(looking("mowers",
		testkit.Use("read_doc", map[string]any{"name": "tools/mowers"}),
		testkit.Use("search_docs", map[string]any{"words": "gas"})))
	h := b.chat("kit", "", "when are the mowers serviced?")
	parts = strings.Split(b.answered(h).Text, "\n---\n")
	holds(t, "the steward's read of the garden's doc", parts[0], `No doc you may read is called "tools/mowers"`)
	holds(t, "the steward's search for the garden's words", parts[1], `No line of the docs you may read holds "gas".`)
}
