package shipped_test

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/permission"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// given gives a loader of the test's own files, and of what Bulldog ships
// under .bulldog/, as an office's loader serves them.
func given(t *testing.T, files map[string]string) func(string) ([]byte, error) {
	t.Helper()
	all := map[string]string{}
	for name, contents := range testkit.Files(t, shipped.Files) {
		all[".bulldog/"+name] = contents
	}
	maps.Copy(all, files)
	return machine.Given(all)
}

// parse fills out and checks a definition written at the top of an office.
func parse(t *testing.T, src string, files map[string]string) *machine.Draft {
	t.Helper()
	d, err := machine.Parse([]byte(src), given(t, files))
	if err != nil {
		t.Fatalf("the definition is refused:\n%v", err)
	}
	return d
}

// shippedDraft fills out and checks a definition Bulldog ships, by its
// path under .bulldog/.
func shippedDraft(t *testing.T, name string) *machine.Draft {
	t.Helper()
	src, err := shipped.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := machine.Parse(src, func(file string) ([]byte, error) { return shipped.Read(path.Join(path.Dir(name), file)) })
	if err != nil {
		t.Fatalf("%s is refused:\n%v", name, err)
	}
	return d
}

// start starts a run with a label of the test's own, and gives its name.
func start(t *testing.T, corbie *testkit.Corbie, d *machine.Draft, s client.Start) string {
	t.Helper()
	s.Labels = map[string]string{"test": t.Name()}
	run, err := corbie.Start(ctx, d, s)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// below stays until a run of the machine named is going below the test's
// runs, other than those given, and gives it.
func below(t *testing.T, corbie *testkit.Corbie, machineName string, not ...string) *client.Run {
	t.Helper()
	var found *client.Run
	corbie.Until(t, "a run of "+machineName+" is going", func() bool {
		runs, err := corbie.List(ctx, client.Filter{Labels: map[string]string{"test": t.Name()}})
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range runs {
			if run.Machine == machineName && run.Parent != "" && !run.HasEnded() && !slices.Contains(not, run.ID) {
				found = run
				return true
			}
		}
		return false
	})
	shown, err := corbie.Show(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	return shown
}

// approveAll is an ask that approves every job held, and says which.
const approveAll = `flow: approve
input: { asking: map, jobs: list }
stages:
  say: { job: { script: ./approve.js, input: { jobs: "${input.jobs}" } } }
output: { approve: "${input.jobs.map(j => j.name)}", note: "${stages.say.output}" }
`

// approved gives what each ask below the test's runs approved, with its
// note.
func approved(t *testing.T, corbie *testkit.Corbie) []string {
	t.Helper()
	runs, err := corbie.List(ctx, client.Filter{Labels: map[string]string{"test": t.Name()}, Finished: true})
	if err != nil {
		t.Fatal(err)
	}
	var asked []string
	for _, r := range runs {
		if r.Machine == "approve" {
			shown, err := corbie.Show(ctx, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			asked = append(asked, fmt.Sprint(shown.Output))
		}
	}
	return asked
}

// The team's start list: everything, but kept values only under its own
// path.
func startList(team string) *permission.Layer {
	return &permission.Layer{Allow: []string{"network", "disk", "exec", "env", "secret:*@*", "kept.read:o1/" + team + "/**", "kept.write:o1/" + team + "/**"}}
}

func TestEveryShippedDefinitionIsTaken(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"wait/wait.yaml", "clock/clock.yaml", "docs/docs.yaml", "rules/for-us/for-us.yaml",
		"bridges/slack/in.yaml", "bridges/slack/out.yaml", "bridges/telegram/in.yaml", "bridges/telegram/out.yaml",
		"bridges/gitlab/in.yaml", "bridges/gitlab/out.yaml", "bridges/jira/in.yaml", "bridges/jira/out.yaml"} {
		shippedDraft(t, name)
	}
	// A worker in a team within names the tools from its own directory by
	// ./.bulldog/, as the office's loader serves them.
	parse(t, `agent: ledger
model: { name: claude-sonnet-5-5, secret: KEY }
prompt: hi
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]
ask: { machine: ./approve.yaml }
`, map[string]string{"approve.yaml": approveAll, "approve.js": "return 'yes'"})
	// Each shipped directory has its page.
	var dirs []string
	err := fs.WalkDir(shipped.Files, ".", func(name string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Base(name) == "README.md" {
			dirs = append(dirs, path.Dir(name))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bridges/gitlab", "bridges/jira", "bridges/slack", "bridges/telegram", "clock", "docs", "rules/for-us", "rules/mentioned", "tools/docs", "tools/talk", "wait"}
	if !slices.Equal(dirs, want) {
		t.Errorf("the pages are in %v", dirs)
	}
	for _, dir := range dirs {
		page, _ := shipped.Read(dir + "/README.md")
		first, _, _ := strings.Cut(string(page), "\n")
		if !strings.Contains(first, ": ") || strings.HasPrefix(first, "#") {
			t.Errorf("%s's page begins %q, not a line saying what it is", dir, first)
		}
		for _, section := range []string{"## Needs", "## Settings", "## Example"} {
			if !strings.Contains(string(page), "\n"+section+"\n") {
				t.Errorf("%s's page has no %s", dir, section)
			}
		}
	}
	if _, err := shipped.Read("nothing.yaml"); err == nil {
		t.Error("a file Bulldog does not ship is read")
	}
}

func TestWaitGivesTheAnswer(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	run := start(t, corbie, shippedDraft(t, "wait/wait.yaml"), client.Start{Input: map[string]any{"to": map[string]any{"people": []any{"sam"}}, "said": "When may I come?", "choices": []any{"mon", "tue"}}})
	ticket := corbie.Waiting(t, run, "answer")
	answer := map[string]any{"answer": "tue", "by": "sam", "thing": "4471:52"}
	if _, err := corbie.Reply(ctx, run, ticket, answer, 0); err != nil {
		t.Fatal(err)
	}
	if ended := corbie.Ended(t, run); ended.Status != "done" || fmt.Sprint(ended.Output) != fmt.Sprint(answer) {
		t.Errorf("the wait ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
}

func TestWaitEndsAfterSevenDays(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	run := start(t, corbie, shippedDraft(t, "wait/wait.yaml"), client.Start{Input: map[string]any{"to": map[string]any{"team": "garden"}, "said": "Is the shed locked?"}})
	corbie.Waiting(t, run, "answer")
	corbie.Clock.Move(7*24*time.Hour - time.Second)
	if shown, err := corbie.Show(ctx, run); err != nil || shown.HasEnded() {
		t.Fatalf("the wait ended a second before 7 days: %v %v", shown, err)
	}
	corbie.Clock.Move(time.Second)
	ended := corbie.Ended(t, run)
	if out, _ := ended.Output.(map[string]any); ended.Status != "done" || out["answer"] != nil || out["note"] != "No answer came within 7 days." {
		t.Errorf("the wait ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
}

func TestClockFallsDueAndWaitsOnCame(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	// Monday is 10:00 in London: the clock falls due an hour on, at 11:00.
	run := start(t, corbie, shippedDraft(t, "clock/clock.yaml"), client.Start{Input: map[string]any{"at": "11:00 Europe/London"}})
	waits := func(what string, timers int, tickets ...string) {
		t.Helper()
		corbie.Until(t, what, func() bool {
			shown, err := corbie.Show(ctx, run)
			return err == nil && shown.Timers == timers && slices.Equal(shown.Tickets, tickets)
		})
	}
	waits("the clock waits for its time", 1)
	corbie.Clock.Move(time.Hour - time.Second)
	if shown, err := corbie.Show(ctx, run); err != nil || len(shown.Tickets) != 0 {
		t.Fatalf("the clock came a second early: %v %v", shown, err)
	}
	corbie.Clock.Move(time.Second)
	ticket := corbie.Waiting(t, run, "came")
	// Three days pass with nobody answering: the clock waits on came once.
	corbie.Clock.Move(3 * 24 * time.Hour)
	if shown, err := corbie.Show(ctx, run); err != nil || !slices.Equal(shown.Tickets, []string{"came"}) || shown.HasEnded() {
		t.Fatalf("after three days the clock is %v: %v", shown, err)
	}
	if _, err := corbie.Reply(ctx, run, ticket, nil, 0); err != nil {
		t.Fatal(err)
	}
	waits("the clock waits for its time again", 1)
	// It is Thursday 11:00 in London, the clock's time, so the next time is
	// Friday 11:00.
	corbie.Clock.Move(24*time.Hour - time.Second)
	if shown, err := corbie.Show(ctx, run); err != nil || len(shown.Tickets) != 0 {
		t.Fatalf("the clock came again a second before Friday 11:00: %v %v", shown, err)
	}
	corbie.Clock.Move(time.Second)
	if next := corbie.Waiting(t, run, "came"); next == ticket {
		t.Error("the clock waits on the same ticket again")
	}
}

func TestAskTeamWaitsForTheTeamsAnswer(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	model := testkit.NewModel(t, testkit.Script(
		testkit.Use("ask_team", map[string]any{"team": "garden", "question": "When do we sow the beds?"}),
		testkit.Say("done")))
	d := parse(t, `agent: steward
model: `+model.Line()+`
prompt: go
tools_file: [./.bulldog/tools/talk/talk.yaml]
ask: { machine: ./approve.yaml }
`, map[string]string{"approve.yaml": approveAll, "approve.js": "return input.jobs.map(j => j.name + ' ' + j.input.to.team).join()"})
	run := start(t, corbie, d, client.Start{})
	wait := below(t, corbie, "wait")
	if in, _ := wait.Input.(map[string]any); fmt.Sprint(in["to"]) != "map[team:garden]" || in["said"] != "When do we sow the beds?" {
		t.Errorf("the wait was given %#v", wait.Input)
	}
	ticket := corbie.Waiting(t, wait.ID, "answer")
	if _, err := corbie.Reply(ctx, wait.ID, ticket, map[string]any{"answer": "In May."}, 0); err != nil {
		t.Fatal(err)
	}
	if ended := corbie.Ended(t, run); ended.Status != "done" {
		t.Fatalf("the steward ended %s: %s", ended.Status, ended.Error)
	}
	// The question was held for the ask, and its answer is the tool's
	// result.
	turns := model.Turns()
	if len(turns) != 2 || !strings.Contains(turns[1].Last(), "In May.") {
		t.Fatalf("the model was given %#v", turns)
	}
	asked := approved(t, corbie)
	if len(asked) != 1 || !strings.Contains(asked[0], "ask_team garden") {
		t.Errorf("the ask approved %v", asked)
	}
}

func TestAskPersonOffersChoices(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	model := testkit.NewModel(t, testkit.Script(
		testkit.Use("ask_person", map[string]any{"who": "sam", "question": "Which socket?", "choices": []any{"kitchen", "hall"}}),
		testkit.Use("ask_person", map[string]any{"who": "kit", "question": "Is the hall dark?"}),
		testkit.Say("done")))
	d := parse(t, "agent: fixer\nmodel: "+model.Line()+"\nprompt: go\ntools_file: [./.bulldog/tools/talk/talk.yaml]\nask: { machine: ./approve.yaml }\n",
		map[string]string{"approve.yaml": approveAll, "approve.js": "return 'yes'"})
	run := start(t, corbie, d, client.Start{})
	var asked []string
	for _, want := range []struct {
		to, said, choices, answer string
	}{{"map[people:[sam]]", "Which socket?", "[kitchen hall]", "hall"}, {"map[people:[kit]]", "Is the hall dark?", "[]", "yes"}} {
		wait := below(t, corbie, "wait", asked...)
		asked = append(asked, wait.ID)
		in, _ := wait.Input.(map[string]any)
		if fmt.Sprint(in["to"]) != want.to || in["said"] != want.said || fmt.Sprint(in["choices"]) != want.choices {
			t.Errorf("the wait was given %#v", wait.Input)
		}
		ticket := corbie.Waiting(t, wait.ID, "answer")
		if _, err := corbie.Reply(ctx, wait.ID, ticket, map[string]any{"answer": want.answer, "by": "sam"}, 0); err != nil {
			t.Fatal(err)
		}
		corbie.Ended(t, wait.ID)
	}
	if ended := corbie.Ended(t, run); ended.Status != "done" {
		t.Fatalf("the fixer ended %s: %s", ended.Status, ended.Error)
	}
	if turns := model.Turns(); len(turns) != 3 || !strings.Contains(turns[1].Last(), "hall") || !strings.Contains(turns[2].Last(), "yes") {
		t.Errorf("the model was given %#v", turns)
	}
}

// ledger is a worker of payments that has the docs tools.
func ledger(model *testkit.Model) string {
	return `agent: ledger
model: ` + model.Line() + `
input: { said: string, request: map }
prompt: ${input.said}
tools_file: [./.bulldog/tools/docs/docs.yaml]
ask: { machine: ./approve.yaml }
`
}

func TestDocsToolsReadAndWriteTheTeamsDocs(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	model := testkit.NewModel(t, testkit.Script(
		testkit.Use("write_doc", map[string]any{"name": "refunds.md", "text": "Refunds within 30 days."}),
		testkit.Use("read_doc", map[string]any{"name": "refunds.md"}),
		testkit.Use("read_doc", map[string]any{"name": "nothing.md"}),
		testkit.Use("read_doc", nil),
		testkit.Use("write_doc", map[string]any{"name": "Refunds.MD", "text": "no"}),
		testkit.Say("done")))
	d := parse(t, ledger(model), map[string]string{"approve.yaml": approveAll, "approve.js": "return input.jobs.map(j => j.name).join()"})
	run := start(t, corbie, d, client.Start{Input: map[string]any{"said": "go", "request": map[string]any{"docs": "o1/payments/docs"}}, Allow: startList("payments")})
	if ended := corbie.Ended(t, run); ended.Status != "done" {
		t.Fatalf("the ledger ended %s: %s", ended.Status, ended.Error)
	}
	turns := model.Turns()
	if len(turns) != 6 {
		t.Fatalf("the model had %d turns", len(turns))
	}
	for i, want := range []string{`"name":"refunds.md"`, "Refunds within 30 days.", `"text":null`, `"names":["refunds.md"]`, "is not a kept path"} {
		if got := turns[i+1].Last(); !strings.Contains(got, want) {
			t.Errorf("turn %d was given %q, not %q", i+2, got, want)
		}
	}
	// Only write_doc was held for a yes, both times.
	asked := approved(t, corbie)
	if len(asked) != 2 || strings.Contains(fmt.Sprint(asked), "read_doc") {
		t.Errorf("the ask approved %v", asked)
	}
}

func TestDocsToolsKeepToOneKindEach(t *testing.T) {
	t.Parallel()
	src, _ := shipped.Read("tools/docs/docs.yaml")
	var tools map[string]struct {
		Job struct {
			Allow   []string
			Approve bool
		}
	}
	if err := yaml.Unmarshal(src, &tools); err != nil {
		t.Fatal(err)
	}
	if r := tools["read_doc"].Job; !slices.Equal(r.Allow, []string{"kept.read:${input.request.docs}/**"}) || r.Approve {
		t.Errorf("read_doc is allowed %v, held %v", r.Allow, r.Approve)
	}
	if w := tools["write_doc"].Job; !slices.Equal(w.Allow, []string{"kept.write:${input.request.docs}/**"}) || !w.Approve {
		t.Errorf("write_doc is allowed %v, held %v", w.Allow, w.Approve)
	}
}

func TestWriteDocNotWrittenWithoutAYes(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	model := testkit.NewModel(t, testkit.Script(
		testkit.Use("write_doc", map[string]any{"name": "refunds.md", "text": "Refund everything."}),
		testkit.Use("read_doc", map[string]any{"name": "refunds.md"}),
		testkit.Say("done")))
	refuse := `flow: approve
input: { asking: map, jobs: list }
stages:
  say: { job: { script: ./approve.js } }
output: { approve: [], note: "${stages.say.output}" }
`
	d := parse(t, ledger(model), map[string]string{"approve.yaml": refuse, "approve.js": "return 'Not today.'"})
	run := start(t, corbie, d, client.Start{Input: map[string]any{"said": "go", "request": map[string]any{"docs": "o1/payments/docs"}}, Allow: startList("payments")})
	if ended := corbie.Ended(t, run); ended.Status != "done" {
		t.Fatalf("the ledger ended %s: %s", ended.Status, ended.Error)
	}
	turns := model.Turns()
	if len(turns) != 3 || !strings.Contains(turns[1].Last(), "Not today.") || !strings.Contains(turns[2].Last(), `"text":null`) {
		t.Errorf("the model was given %#v", turns)
	}
}

func TestDocsMachineListsReadsAndWrites(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	d := shippedDraft(t, "docs/docs.yaml")
	docs := func(input map[string]any) *client.Run {
		t.Helper()
		input["place"] = "o1/garden/docs"
		return corbie.Ended(t, start(t, corbie, d, client.Start{Input: input, Allow: startList("garden")}))
	}
	if r := docs(map[string]any{"do": "write", "name": "beds.md", "text": "Sow in May."}); r.Status != "done" || fmt.Sprint(r.Output) != "map[name:beds.md]" {
		t.Fatalf("writing ended %s with %#v: %s", r.Status, r.Output, r.Error)
	}
	docs(map[string]any{"do": "write", "name": "shed.md", "text": "The key is under the pot."})
	for _, c := range []struct {
		input map[string]any
		want  string
	}{
		{map[string]any{"do": "read", "name": "beds.md"}, "map[name:beds.md text:Sow in May.]"},
		{map[string]any{"do": "read", "name": "lawn.md"}, "map[name:lawn.md text:<nil>]"},
		{map[string]any{"do": "list"}, "map[names:[beds.md shed.md]]"},
	} {
		if r := docs(c.input); r.Status != "done" || fmt.Sprint(r.Output) != c.want {
			t.Errorf("%v ended %s with %#v: %s", c.input, r.Status, r.Output, r.Error)
		}
	}
	// A name of more than one part, and one the kept rule refuses, are
	// refused with the rule.
	for _, name := range []string{"beds/lawn.md", "Beds.MD"} {
		if r := docs(map[string]any{"do": "write", "name": name, "text": "no"}); r.Status != "failed" || !strings.Contains(r.Error, "small letters a to z") && !strings.Contains(r.Error, "lower-case letters a to z") {
			t.Errorf("%s ended %s: %s", name, r.Status, r.Error)
		}
	}
	// A place another team's start list does not cover is refused.
	r := corbie.Ended(t, start(t, corbie, d, client.Start{Input: map[string]any{"place": "o1/payments/docs", "do": "list"}, Allow: startList("garden")}))
	if r.Status != "failed" || r.Failure != "denied" {
		t.Errorf("garden's start list listing payments' docs ended %s (%s): %s", r.Status, r.Failure, r.Error)
	}
}
