package changes

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

func TestBuilderAnswersEveryTool(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	o := f.keeper.Office()
	// The builder names the builder's tools file, and is offered all seven;
	// the clerk beside it names none of it, and is offered none.
	for _, tool := range Tools() {
		if !slices.Contains(o.Workers["builder"].Tools, tool) {
			t.Errorf("the builder is not offered %s: it holds %v", tool, o.Workers["builder"].Tools)
		}
		if slices.Contains(o.Workers["clerk"].Tools, tool) {
			t.Errorf("the clerk is offered %s", tool)
		}
	}
	if !slices.Equal(Tools(), shipped.Tools(shipped.Build)) {
		t.Errorf("Changes answers %v, and the builder's tools file holds %v", Tools(), shipped.Tools(shipped.Build))
	}
	calls := []struct {
		tool string
		args map[string]any
	}{
		{ToolFiles, nil},
		{ToolRead, map[string]any{"path": "office.yaml"}},
		{ToolWrite, map[string]any{"path": "notes/fuses.md", "text": "Breaker 4 is the kitchen.\n"}},
		{ToolRemove, map[string]any{"path": "clerk.yaml"}},
		{ToolCheck, nil},
		{ToolPut, map[string]any{"why": "Note the fuses, and let the clerk go"}},
		{ToolManual, map[string]any{"page": "office"}},
	}
	for _, c := range calls {
		if a := f.call(1, c.tool, c.args); a.Text == "" {
			t.Errorf("%s gave no answer", c.tool)
		}
	}
}

func TestChangesBuilderToolsAreRunsThatWait(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	corbie := testkit.NewCorbie(t, testkit.NewClock(testkit.Monday))
	var doc yaml.Node
	contents, _ := shipped.File(shipped.Build)
	if err := yaml.Unmarshal(contents, &doc); err != nil {
		t.Fatal(err)
	}
	under := func(n *yaml.Node, key string) *yaml.Node {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				if v := n.Content[i+1]; v.Kind == yaml.AliasNode {
					return v.Alias
				}
				return n.Content[i+1]
			}
		}
		t.Fatalf("no %s", key)
		return nil
	}
	call, _ := shipped.File(shipped.Call)
	d, err := machine.Parse(call, machine.Given(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range Tools() {
		// The job each tool runs, as the tools file writes it: the call
		// Bulldog ships, which waits on a ticket that Bulldog answers.
		job := under(under(doc.Content[0], tool), "job")
		if got := under(job, "machine").Value; got != "./"+shipped.Call {
			t.Errorf("%s's job runs %s", tool, got)
		}
		if got := under(under(job, "input"), "tool").Value; got != tool {
			t.Errorf("%s's job names the tool %s", tool, got)
		}
		run, err := corbie.Start(ctx, d, client.Start{Input: map[string]any{"tool": tool, "args": map[string]any{"page": "office"}, "unanswered": under(under(job, "input"), "unanswered").Value}, Labels: map[string]string{"kind": "test"}})
		if err != nil {
			t.Fatal(err)
		}
		ticket := corbie.Waiting(t, run, shipped.CallTicket)
		raw, _ := json.Marshal(map[string]any{"page": "office"})
		a, err := f.c.Answer(ctx, Call{Request: 1, Worker: "builder", Tool: tool, Args: raw})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := corbie.Reply(ctx, run, ticket, a.Text, 0); err != nil {
			t.Fatal(err)
		}
		if ended := corbie.Ended(t, run); ended.Status != "done" || ended.Output != a.Text {
			t.Errorf("%s's run ended %s with %v, and not with Bulldog's answer", tool, ended.Status, ended.Output)
		}
	}
}

func TestBuilderListsEveryFile(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "weather.yaml", "agent: weather\n")
	f.write(1, "clerk.yaml", "agent: clerk\nmodel: usual\nsystem: Money.\nprompt: ${input.said}\ninput: [said]\n")
	f.call(1, ToolRemove, map[string]any{"path": "bins.yaml"})
	lines := strings.Split(f.call(1, ToolFiles, nil).Text, "\n")
	has(t, lines, "weather.yaml: new")
	has(t, lines, "clerk.yaml: changed")
	has(t, lines, "bins.yaml: gone")
	has(t, lines, "steward.yaml: as it is")
	for file := range house(t) {
		has(t, lines, file+": ")
	}
	hasNo(t, lines, ".bulldog")
}

func TestBuilderReadsDraftOrFolder(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", "agent: clerk\n# drafted\n")
	f.call(1, ToolRemove, map[string]any{"path": "bins.yaml"})
	read := func(file string) string { return f.call(1, ToolRead, map[string]any{"path": file}).Text }
	if got := read("clerk.yaml"); got != "agent: clerk\n# drafted\n" {
		t.Errorf("the drafted file reads %q", got)
	}
	if got := read("steward.yaml"); got != house(t)["steward.yaml"] {
		t.Errorf("an untouched file reads %q", got)
	}
	if got := read("nowhere.yaml"); !strings.Contains(got, "there is no file nowhere.yaml") {
		t.Errorf("a file in neither reads %q", got)
	}
	if got := read("bins.yaml"); !strings.Contains(got, "your draft removes bins.yaml") {
		t.Errorf("a removed file reads %q", got)
	}
}

func TestBuilderWritesWholeFiles(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "weather.yaml", "agent: weather\n")
	f.write(1, "clerk.yaml", "agent: clerk\n")
	draft, _ := f.c.Drafted(ctx, 1)
	if len(draft) != 2 || draft[0].Path != "clerk.yaml" || draft[0].Does() != IsChanged || draft[1].Path != "weather.yaml" || draft[1].Does() != IsNew {
		t.Errorf("the draft is %+v", draft)
	}
	if got := f.call(1, ToolRead, map[string]any{"path": "weather.yaml"}).Text; got != "agent: weather\n" {
		t.Errorf("the new file reads %q", got)
	}
	if f.text("weather.yaml") != "" || f.text("clerk.yaml") != house(t)["clerk.yaml"] {
		t.Error("writing into the draft wrote into the folder")
	}
}

func TestBuilderRemovesFiles(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.call(1, ToolRemove, map[string]any{"path": "bins.yaml"})
	has(t, strings.Split(f.call(1, ToolFiles, nil).Text, "\n"), "bins.yaml: gone")
	check := f.check(1)
	has(t, check.Says, "bins leaves the house")
	if got := f.call(1, ToolRemove, map[string]any{"path": "nothing.yaml"}).Text; !strings.Contains(got, "nothing to remove") {
		t.Errorf("removing what is not there says %q", got)
	}
}

func TestBuilderChecksWithoutPutting(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	// A team file naming a person who is not in the office, and a worker
	// Corbie refuses: both in one answer, each with where it is.
	f.write(1, "garden/team.yaml", strings.Replace(house(t)["garden/team.yaml"], "people: [sam]", "people: [sam, jo]", 1))
	f.write(1, "clerk.yaml", strings.Replace(house(t)["clerk.yaml"], "prompt: ${input.said}", "prompt: ${input.nothing", 1))
	answer := f.call(1, ToolCheck, nil).Text
	for _, words := range [][]string{{"garden/team.yaml", "jo is nobody"}, {"clerk.yaml"}} {
		if !strings.Contains(answer, words[0]) || !strings.Contains(answer, words[len(words)-1]) {
			t.Errorf("the check does not say %q:\n%s", words, answer)
		}
	}
	check := f.check(1)
	has(t, problemLines(check.Problems), "garden/team.yaml", "jo is nobody")
	has(t, problemLines(check.Problems), "clerk.yaml")
	// A draft with no problems is given what it says, what lets more by
	// apart.
	f.write(2, "fixer.yaml", strings.Replace(house(t)["fixer.yaml"], "input: { circuit: \"${args.circuit}\" }, approve: true", "input: { circuit: \"${args.circuit}\" } ", 1))
	f.write(2, "notes/fuses.md", "Breaker 4.\n")
	answer = f.call(2, ToolCheck, nil).Text
	if !strings.Contains(answer, "What it lets more by") || !strings.Contains(answer, "cut_power no longer waits") || !strings.Contains(answer, "note fuses is new") {
		t.Errorf("the check says:\n%s", answer)
	}
	if strings.Index(answer, "note fuses") > strings.Index(answer, "What it lets more by") {
		t.Errorf("what does not let more by is said among what does:\n%s", answer)
	}
	if len(f.decided()) != 0 {
		t.Error("a check put the change to be decided")
	}
}

func TestBuilderReadsTheManual(t *testing.T) {
	t.Parallel()
	page := Manual("office")
	for _, example := range []string{"agent: weather", "small-orders.js", "flow: slack-out"} {
		if !strings.Contains(page, example) {
			t.Errorf("the office page holds no %q", example)
		}
	}
	if got := Manual("statechart"); !strings.Contains(got, "states") {
		t.Errorf("Corbie's page statechart reads %q", got)
	}
	got := Manual("nothing-at-all")
	if !strings.Contains(got, `"nothing-at-all" is not a page`) || !strings.Contains(got, "office") || !strings.Contains(got, "statechart") {
		t.Errorf("a name that is no page reads %q", got)
	}
}

func TestBuilderDraftOutlastsItsRun(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(7, "weather.yaml", "agent: weather\n")
	// The server stops and starts again: the store is the same.
	f.c = f.open()
	draft, err := f.c.Drafted(ctx, 7)
	if err != nil || len(draft) != 1 || draft[0].Text != "agent: weather\n" {
		t.Errorf("after a start the draft is %+v, %v", draft, err)
	}
	has(t, strings.Split(f.call(7, ToolFiles, nil).Text, "\n"), "weather.yaml: new")
}

func TestBuilderDraftKeepsWhatItWasDraftedAgainst(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", "agent: clerk\n# one\n")
	f.write(1, "clerk.yaml", "agent: clerk\n# two\n")
	draft, _ := f.c.Drafted(ctx, 1)
	if len(draft) != 1 || draft[0].Text != "agent: clerk\n# two\n" || draft[0].Was != house(t)["clerk.yaml"] || !draft[0].Had {
		t.Errorf("the draft is %+v", draft)
	}
}

func TestChangesUnchangedFileLeavesTheDraft(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", "agent: clerk\n")
	f.write(1, "clerk.yaml", house(t)["clerk.yaml"])
	f.write(1, "weather.yaml", "agent: weather\n")
	f.call(1, ToolRemove, map[string]any{"path": "weather.yaml"})
	if draft, _ := f.c.Drafted(ctx, 1); len(draft) != 0 {
		t.Errorf("the draft holds %+v", draft)
	}
	if got := f.call(1, ToolPut, map[string]any{"why": "Nothing"}); got.Put != 0 || !strings.Contains(got.Text, "empty") {
		t.Errorf("an empty draft is put: %+v", got)
	}
}

func TestChangesDraftRefusesWhatIsBulldogsOwn(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	for file, says := range map[string]string{
		"../outside.yaml":          "outside the office's folder",
		"/etc/passwd":              "outside the office's folder",
		".bulldog/talk.yaml":       "Bulldog's own",
		".bulldog/bulldog.db":      "Bulldog's own",
		".git/config":              "repository",
		"garden/.hidden.yaml":      "starts with a dot",
		"./steward.yaml":           "written plainly",
		"garden":                   "is a folder",
		"steward.yaml/inside.yaml": "is a file",
		"":                         "no path",
	} {
		for _, tool := range []string{ToolWrite, ToolRemove, ToolRead} {
			got := f.call(1, tool, map[string]any{"path": file, "text": "x"}).Text
			if tool == ToolRead && (says == "is a folder" || says == "is a file") {
				continue
			}
			if !strings.Contains(got, says) {
				t.Errorf("%s of %q says %q", tool, file, got)
			}
		}
	}
	if draft, _ := f.c.Drafted(ctx, 1); len(draft) != 0 {
		t.Errorf("the draft holds %+v", draft)
	}
}

func TestChangesDraftsAreKeptApart(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	one, two := house(t)["clerk.yaml"]+"# one\n", house(t)["clerk.yaml"]+"# two\n"
	f.write(1, "clerk.yaml", one)
	f.write(2, "clerk.yaml", two)
	if got := f.call(1, ToolRead, map[string]any{"path": "clerk.yaml"}).Text; got != one {
		t.Errorf("request 1 reads %q", got)
	}
	if got := f.call(2, ToolRead, map[string]any{"path": "clerk.yaml"}).Text; got != two {
		t.Errorf("request 2 reads %q", got)
	}
	if got := f.call(3, ToolFiles, nil).Text; !strings.Contains(got, "clerk.yaml: as it is") {
		t.Errorf("request 3 sees another's draft:\n%s", got)
	}
	f.put(1, "One")
	if draft, _ := f.c.Drafted(ctx, 2); len(draft) != 1 {
		t.Errorf("putting one request's draft changed another's: %+v", draft)
	}
}

func TestChangesBuilderWritesNothingWithoutAPerson(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	before := f.head().Hash
	f.write(1, "clerk.yaml", house(t)["clerk.yaml"]+"# rewritten\n")
	f.call(1, ToolRemove, map[string]any{"path": "tools/order.js"})
	f.write(1, "tools/order.js", "return 'ordered'\n")
	f.call(1, ToolRemove, map[string]any{"path": "garden/notes/shed.md"})
	f.call(1, ToolCheck, nil)
	p := f.put(1, "Rewrite the clerk")
	if f.text("clerk.yaml") != house(t)["clerk.yaml"] || f.text("garden/notes/shed.md") == "" || f.head().Hash != before {
		t.Fatal("the builder's tools wrote into the office")
	}
	// A yes from a rule or a worker writes nothing.
	for _, kind := range []string{office.IsRule, office.IsWorker} {
		if _, err := f.c.Decided(ctx, p.ID, Decision{By: "steward", Kind: kind, Yes: true}); err == nil || !strings.Contains(err.Error(), "only a person's yes") {
			t.Errorf("a %s's yes is taken: %v", kind, err)
		}
	}
	// Nor does a yes from someone the walk does not ask.
	if _, err := f.c.Decided(ctx, p.ID, Decision{By: "kit", Kind: office.IsPerson, Yes: true}); err == nil {
		t.Error("Kit's yes is taken, and the house asks Sam")
	}
	if f.text("clerk.yaml") != house(t)["clerk.yaml"] || f.head().Hash != before {
		t.Fatal("a yes that is not a person's wrote the change")
	}
	if o := f.yes(p.ID, "sam", ""); o.Ended != Written || f.text("garden/notes/shed.md") != "" || f.text("tools/order.js") != "return 'ordered'\n" {
		t.Errorf("Sam's yes ended it %+v", o)
	}
}
