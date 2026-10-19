package office_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/agent"
	"github.com/zummed/corbie/client"
)

var ctx = context.Background()

// standIn gives the house with its models those of a stand-in model.
func standIn(t *testing.T, files map[string]string, m *testkit.Model) map[string]string {
	files = replace(t, files, "office.yaml", "{ name: sonnet, service: claude-code }", m.Line())
	return replace(t, files, "garden/team.yaml", "{ name: opus, service: claude-code }", m.Line())
}

// modelOf gives the model of an agent's definition as Corbie filled it out.
func modelOf(t *testing.T, w *office.Worker) agent.Model {
	t.Helper()
	var a agent.Agent
	if err := json.Unmarshal(w.Draft.Definition.Body, &a); err != nil {
		t.Fatal(err)
	}
	return a.Model
}

func TestOfficeWorkerSentAsWritten(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	fixer := o.Workers["fixer"]
	want := strings.Replace(string(fixer.Written), "model: usual", "model: { name: sonnet, service: claude-code }", 1)
	if string(fixer.Definition) != want {
		t.Errorf("the fixer is sent as\n%s\nand not as written, with its model filled in:\n%s", fixer.Definition, want)
	}
	if fixer.Model != "{ name: sonnet, service: claude-code }" || fixer.ModelName != "usual" {
		t.Errorf("the fixer's model is shown as %q, named %q", fixer.Model, fixer.ModelName)
	}
	// Sparks writes its model out, and is sent exactly as written.
	if sparks := o.Workers["sparks"]; string(sparks.Definition) != string(sparks.Written) || sparks.Model != "{ name: haiku, service: claude-code }" {
		t.Errorf("sparks is sent as %s, its model shown as %q", sparks.Definition, sparks.Model)
	}
	// A word written in quotes, or in a row, is filled in the same way.
	o = reads(t, replace(t, house(t), "clerk.yaml", "model: usual", `model: "usual"   # the house's`))
	if got := string(o.Workers["clerk"].Definition); !strings.Contains(got, `model: { name: sonnet, service: claude-code }   # the house's`) {
		t.Errorf("a quoted word is sent as %s", got)
	}
	// A word written so that its place cannot be found is the worker's
	// problem, and the definition is not written out again.
	o, problems := read(t, replace(t, house(t), "clerk.yaml", "model: usual", "model: >-\n  usual"))
	says(t, problems.OfWorker("clerk"), "clerk.yaml", "model: usual")
	if o.Workers["clerk"].Runs() || o.Workers["clerk"].Definition != nil || !o.Workers["fixer"].Runs() {
		t.Errorf("the clerk is sent as %s", o.Workers["clerk"].Definition)
	}
	// And what Corbie stores and runs is that definition.
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	model := testkit.NewModel(t, testkit.Script(testkit.Say("done")))
	o = reads(t, standIn(t, house(t), model))
	run, err := corbie.Start(ctx, o.Workers["clerk"].Draft, client.Start{Input: map[string]any{"said": "what is owed?"}, Labels: map[string]string{"test": "as-written"}})
	if err != nil {
		t.Fatal(err)
	}
	corbie.Ended(t, run)
	def, err := corbie.Definition(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if body := string(def.Body); !strings.Contains(body, model.URL) || !strings.Contains(body, "You keep the house's accounts.") {
		t.Errorf("Corbie ran %s", body)
	}
}

func TestOfficeDefinitionCarriesWorkerName(t *testing.T) {
	t.Parallel()
	o, problems := read(t, replace(t, house(t), "clerk.yaml", "agent: clerk", "agent: accounts"))
	says(t, problems.OfWorker("clerk"), "clerk.yaml", "accounts", "agent: clerk")
	if o.Workers["clerk"].Runs() || o.Workers["accounts"] != nil || !o.Workers["steward"].Runs() {
		t.Error("a worker whose definition carries another name runs, or stops another")
	}
}

func TestARefusedWorkerStopsItselfAndWhatRunsIt(t *testing.T) {
	t.Parallel()
	files := replace(t, house(t), "sparks.yaml", ", job: { script: ./tools/circuits.js }", "")
	files = replace(t, files, "sparks.yaml", "input: [said]", `input: [said, "no good"]`)
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the house is refused: %v", problems)
	}
	of := problems.OfWorker("sparks")
	says(t, of, "sparks.yaml", "a tool needs a job")
	says(t, of, "sparks.yaml", "input")
	if o.Workers["sparks"].Runs() || !o.Workers["clerk"].Runs() || !o.Workers["gardener"].Runs() {
		t.Error("sparks runs, or another worker does not")
	}
	// Corbie refuses a definition when it refuses one the definition runs,
	// and a worker it refuses has no draft to start. The fixer runs
	// sparks, and the steward the fixer, so neither can run either, and
	// each says once that sparks is why.
	if o.Workers["fixer"].Runs() || o.Workers["steward"].Runs() {
		t.Error("a worker runs with a colleague Corbie refuses")
	}
	if got := problems.OfWorker("fixer"); len(got) != 1 || !strings.Contains(got[0].Text, "it uses sparks") {
		t.Errorf("the fixer's problems are %v", got)
	}
	for _, name := range []string{"clerk", "bins", "builder", "gardener", "groundsman"} {
		if !o.Workers[name].Runs() {
			t.Errorf("%s does not run", name)
		}
	}
	// What is wrong in a definition a worker runs that is no worker's is
	// said on that definition's file.
	files = replace(t, house(t), "fixer.yaml", "machine: ./sparks.yaml", "machine: ./tools/meter.yaml")
	files["tools/meter.yaml"] = "flow: meter\ninput: [said, \"no good\"]\nstages: { read: { job: { script: ./circuits.js } } }\noutput: ${stages.read.output}\n"
	_, problems = read(t, files)
	if p := says(t, problems.OfWorker("fixer"), "input"); p.File != "tools/meter.yaml" || p.Place != "input[1]" {
		t.Errorf("the meter's problem is said as %#v", p)
	}
	o, problems = read(t, replace(t, house(t), "steward.yaml", "prompt: ${input.context}", "prompt: ${input.context"))
	says(t, problems.OfWorker("steward"), "steward.yaml", "prompt")
	if p := says(t, problems, "office.yaml", "lead", "steward", "cannot run"); p.Team != "office" || p.Refuses {
		t.Errorf("the lead's problem is %#v", p)
	}
	if !o.Workers["fixer"].Runs() {
		t.Error("the fixer does not run while the steward cannot")
	}
}

func TestWorkerStartedAtTopNamesTheAsk(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	if !o.Workers["steward"].AtTop || o.Workers["fixer"].AtTop || o.Workers["sparks"].AtTop || !o.Workers["bins"].AtTop {
		t.Error("the workers started at the top are not the lead and the routines' workers")
	}
	o, problems := read(t, replace(t, house(t), "steward.yaml", "ask: { machine: ./.bulldog/ask.yaml }", ""))
	says(t, problems.OfWorker("steward"), "steward.yaml", "ask", "ask: { machine: ./.bulldog/ask.yaml }")
	says(t, problems, "office.yaml", "lead", "cannot run")
	if o.Workers["steward"].Runs() || !o.Workers["fixer"].Runs() || !o.Workers["clerk"].Runs() {
		t.Error("the steward runs with no ask, or the others do not")
	}
	routine := `  - { at: "08:00", to: fixer, say: "Anything to mend?" }` + "\n"
	files := replace(t, house(t), "office.yaml", "routines:\n", "routines:\n"+routine)
	_, problems = read(t, files)
	says(t, problems.OfWorker("fixer"), "fixer.yaml", "ask")
	reads(t, replace(t, files, "fixer.yaml", "tools_file:", "ask: { machine: ./.bulldog/ask.yaml }\ntools_file:"))
	files = replace(t, house(t), "sparks.yaml", "prompt: ${input.said}", "prompt: ${input.said}\nask: { machine: ./tools/my-ask.yaml }")
	_, problems = read(t, with(files, map[string]string{"tools/my-ask.yaml": "statechart: my-ask\ninitial: yes\nstates: { yes: { end: done, output: { approve: [] } } }\n"}))
	says(t, problems.OfWorker("sparks"), "sparks.yaml", "ask", "the one Bulldog ships")
	// A lead within a team names the ask by its path from its folder.
	// What can be held may be far down, as permissions to be asked about
	// or in a machine in tools/.
	routine = `  - { at: "08:00", to: clerk, say: "Anything owed?" }` + "\n"
	files = replace(t, house(t), "office.yaml", "routines:\n", "routines:\n"+routine)
	reads(t, files)
	for name, change := range map[string]map[string]string{
		"ask_about": {"clerk.yaml": strings.Replace(house(t)["clerk.yaml"], "prompt:", "ask_about: [net:shop.example]\nprompt:", 1)},
		"tools/": {
			"clerk.yaml":     strings.Replace(house(t)["clerk.yaml"], "prompt: ${input.said}", "prompt: ${input.said}\ntools: { pay: { does: Pay, args: {}, job: { machine: ./tools/pay.yaml } } }", 1),
			"tools/pay.yaml": "flow: pay\nstages: { pay: { job: { script: ./order.js, approve: true } } }\n",
		},
	} {
		_, problems = read(t, with(files, change))
		if got := saying(problems.OfWorker("clerk"), "clerk.yaml", "ask: { machine: ./.bulldog/ask.yaml }"); len(got) != 1 {
			t.Errorf("with %s the clerk's problems are %v", name, problems.OfWorker("clerk"))
		}
	}
	files = replace(t, house(t), "garden/groundsman.yaml", "prompt: ${input.said}", "prompt: ${input.said}\ntools: { open_door: { does: Open the shed, args: {}, job: { script: ./open.js, approve: true } } }")
	files = with(files, map[string]string{"garden/open.js": "return 1"})
	_, problems = read(t, files)
	says(t, problems.OfWorker("gardener"), "ask: { machine: ../.bulldog/ask.yaml }")
	reads(t, replace(t, files, "garden/gardener.yaml", "tools_file:", "ask: { machine: ../.bulldog/ask.yaml }\ntools_file:"))
}

// asks stays until a run of the ask Bulldog ships waits on its ticket,
// below a run labelled for the test, and gives it with what it was told.
func asks(t *testing.T, corbie *testkit.Corbie, label string) (*client.Run, map[string]any) {
	t.Helper()
	var ask *client.Run
	corbie.Until(t, "an ask waits", func() bool {
		runs, err := corbie.List(ctx, client.Filter{Labels: map[string]string{"test": label}})
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range runs {
			if run.Machine == "ask" && slices.Contains(run.Tickets, shipped.AskTicket) {
				ask = run
				return true
			}
		}
		return false
	})
	var told map[string]any
	if _, err := corbie.Record(ctx, ask.ID, func(e client.Entry) {
		for _, event := range e.Events {
			if event.Name == "start" && told == nil {
				told, _ = event.Data.(map[string]any)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	return ask, told
}

func TestAHeldActionGoesToTheNearestAskAbove(t *testing.T) {
	t.Parallel()
	for _, fixerNamesAsk := range []bool{false, true} {
		t.Run(fmt.Sprint("fixer names the ask: ", fixerNamesAsk), func(t *testing.T) {
			clock := testkit.NewClock(testkit.Monday)
			corbie := testkit.NewCorbie(t, clock)
			model := testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
				switch {
				case strings.Contains(turn.System, "You look after the house") && turn.Number == 1:
					return testkit.Use("fixer", map[string]any{"job": "the kitchen socket sparked"})
				case strings.Contains(turn.System, "You do the repairs") && turn.Number == 1:
					return testkit.Use("cut_power", map[string]any{"circuit": "4"})
				}
				return testkit.Say("done: " + turn.Last())
			})
			files := standIn(t, house(t), model)
			if fixerNamesAsk {
				files = replace(t, files, "fixer.yaml", "tools_file:", "ask: { machine: ./.bulldog/ask.yaml }\ntools_file:")
			}
			o := reads(t, files)
			steward, err := corbie.Start(ctx, o.Workers["steward"].Draft, client.Start{
				Input: map[string]any{"said": "the kitchen socket sparked", "context": "Kit asks: the kitchen socket sparked"}, Labels: map[string]string{"test": "held"}})
			if err != nil {
				t.Fatal(err)
			}
			ask, told := asks(t, corbie, "held")
			asking, _ := told["asking"].(map[string]any)
			if asking["machine"] != "fixer" {
				t.Errorf("the ask is told %#v is asking", asking)
			}
			runs, err := corbie.List(ctx, client.Filter{Labels: map[string]string{"test": "held"}})
			if err != nil {
				t.Fatal(err)
			}
			var fixer string
			for _, run := range runs {
				if run.Machine == "fixer" {
					fixer = run.ID
				}
			}
			if ask.Parent != fixer {
				t.Errorf("the ask is below %s, not the fixer's run %s", ask.Parent, fixer)
			}
			jobs, _ := told["jobs"].([]any)
			if _, err := corbie.Reply(ctx, ask.ID, corbie.Waiting(t, ask.ID, shipped.AskTicket), map[string]any{"approve": []any{jobs[0].(map[string]any)["name"]}}, 0); err != nil {
				t.Fatal(err)
			}
			if ended := corbie.Ended(t, steward); ended.Status != "done" || !strings.Contains(fmt.Sprint(ended.Output), "Circuit 4 is off") {
				t.Errorf("the steward ended %s with %v %s", ended.Status, ended.Output, ended.Error)
			}
		})
	}
}
