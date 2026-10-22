package requests

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"go.yaml.in/yaml/v3"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// work is an office of two teams: the office, whose lead is the bot, and
// payments, which holds customer data. Its model is the test's, written
// in place of MODEL.
func work() map[string]string {
	return map[string]string{
		"office.yaml": `name: Work
about: One bot over every team.
everyone:
  dana: { name: Dana }
  lee: { name: Lee }
  priya: { name: Priya }
  al: { name: Al }
models:
  usual: MODEL
people: [dana]
lead: bot
`,
		"bot.yaml": `agent: bot
model: usual
system: ${input.context}
prompt: ${input.said}
tools_file: [./.bulldog/tools/talk/talk.yaml]
`,
		"payments/team.yaml": `name: Payments
about: Charges, refunds and the ledger. Holds customer data.
people: [lee, priya]
lead: ledger
heed: [./.bulldog/rules/mentioned/mentioned.js]
check: [./no-account-numbers.js, lee, priya]
decide: [./small-orders.js, lee]
`,
		"payments/ledger.yaml": `agent: ledger
model: usual
system: ${input.context}
prompt: ${input.said}
tools_file: [./.bulldog/tools/talk/talk.yaml]
tools:
  order_part:
    does: Order a part
    args: { pounds: number }
    job: { script: ./order.js, input: { pounds: "${args.pounds}" }, approve: true }
`,
		"payments/order.js": `return { ordered: input.pounds }`,
		"payments/no-account-numbers.js": `const text = input.answer ?? input.question.said
if (text.includes("12345678")) return { says: "no", note: "It holds an account number." }
return { says: "yes", note: "No account numbers." }
`,
		"payments/small-orders.js": `const pounds = input.jobs.map(j => (j.asked?.args ?? j.input).pounds ?? Infinity)
if (pounds.every(p => p <= 20)) return { says: "yes", note: "Under £20." }
return { says: "pass", note: "More than I order alone." }
`,
	}
}

// A world is an office of a test's own on a Corbie of its own, with a
// stand-in model.
type world struct {
	t      *testing.T
	corbie *testkit.Corbie
	model  *testkit.Model
	office *office.Office
	made   map[string]*Machine
}

// newWorld writes the work office, changed as the test changes it, reads
// it and makes its request machines. The model gives each turn what answer
// gives.
func newWorld(t *testing.T, answer func(testkit.Turn) testkit.Reply, change func(files map[string]string)) *world {
	t.Helper()
	model := testkit.NewModel(t, answer)
	files := work()
	if change != nil {
		change(files)
	}
	for name, text := range files {
		files[name] = strings.ReplaceAll(text, "MODEL", model.Line())
	}
	o, err := office.Read(testkit.Dir(t, files), shipped.Files)
	if err != nil {
		t.Fatal(err)
	}
	return &world{t: t, corbie: testkit.NewCorbie(t), model: model, office: o, made: Make(o)}
}

// ask starts a request of a team, as Bulldog does, and gives the run's
// name. The request is Dana's in the chat but for what the test gives.
func (w *world) ask(team, said string, given map[string]any) string {
	w.t.Helper()
	m := w.made[team]
	if m.Draft == nil {
		w.t.Fatalf("%s's request machine is not made:\n%v", team, office.Problems(w.office.Problems))
	}
	request := map[string]any{"team": team, "by": map[string]any{"person": "dana"}, "heard": "chat", "place": nil,
		"thread": []any{}, "teams": []any{}, "open": []any{}, "chain": []any{}, "docs": w.office.Docs(team), "mentioned": false}
	maps.Copy(request, given)
	run, err := w.corbie.Start(ctx, m.Draft, client.Start{
		Input:  map[string]any{"said": said, "context": "Dana asks " + team + " in the chat.", "request": request, "files": []any{}},
		Allow:  m.Allow,
		Labels: map[string]string{"bulldog": w.office.ID, "kind": "request", "team": team},
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return run
}

// output stays until a request has ended, and gives its output.
func (w *world) output(run string) map[string]any {
	w.t.Helper()
	ended := w.corbie.Ended(w.t, run)
	out, ok := ended.Output.(map[string]any)
	if ended.Status != "done" || !ok {
		w.t.Fatalf("the request ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
	return out
}

// waiting stays until a run of wait.yaml waits on its ticket, other than
// those given, and gives it with its input.
func (w *world) waiting(not ...string) *client.Run {
	w.t.Helper()
	var found string
	w.corbie.Until(w.t, "a wait waits on its answer", func() bool {
		runs, err := w.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": w.office.ID}})
		if err != nil {
			w.t.Fatal(err)
		}
		for _, r := range runs {
			if r.Machine == "wait" && slices.Contains(r.Tickets, "answer") && !slices.Contains(not, r.ID) {
				found = r.ID
				return true
			}
		}
		return false
	})
	shown, err := w.corbie.Show(ctx, found)
	if err != nil {
		w.t.Fatal(err)
	}
	return shown
}

// runs gives every run of a machine, by its name, going or ended.
func (w *world) runs(name string) []*client.Run {
	w.t.Helper()
	all, err := w.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": w.office.ID}, Finished: true})
	if err != nil {
		w.t.Fatal(err)
	}
	var runs []*client.Run
	for _, r := range all {
		if r.Machine == name {
			runs = append(runs, r)
		}
	}
	return runs
}

// answer answers a wait as Bulldog does.
func (w *world) answer(wait *client.Run, answer, by string) {
	w.t.Helper()
	ticket := w.corbie.Waiting(w.t, wait.ID, "answer")
	if _, err := w.corbie.Reply(ctx, wait.ID, ticket, map[string]any{"answer": answer, "by": by, "thing": "test"}, 0); err != nil {
		w.t.Fatal(err)
	}
}

func TestMakeTakesEveryTeam(t *testing.T) {
	t.Parallel()
	w := newWorld(t, testkit.Script(), nil)
	if len(w.office.Problems) > 0 {
		t.Fatalf("the work office has problems:\n%v", office.Problems(w.office.Problems))
	}
	for team, stages := range map[string][]string{"office": {"work"}, "payments": {"heed", "work", "check"}} {
		m := w.made[team]
		if m.Draft == nil {
			t.Fatalf("%s has no request machine", team)
		}
		var written struct {
			Flow   string
			Keep   string
			Ask    map[string]any
			Stages yaml.Node
		}
		if err := yaml.Unmarshal([]byte(m.YAML), &written); err != nil {
			t.Fatal(err)
		}
		var named []string
		for i := 0; i < len(written.Stages.Content); i += 2 {
			named = append(named, written.Stages.Content[i].Value)
		}
		if written.Flow != team || written.Keep != "30d" || written.Ask == nil || !slices.Equal(named, stages) {
			t.Errorf("%s's request machine is flow %s, keep %s, with the stages %v:\n%s", team, written.Flow, written.Keep, named, m.YAML)
		}
		if fmt.Sprint(m.Allow.Allow) != fmt.Sprint(w.office.Allow(team)) {
			t.Errorf("%s's runs are started with %v", team, m.Allow.Allow)
		}
	}
	// The office has no heed, so its work runs with no condition.
	if office := w.made["office"].YAML; !strings.Contains(office, "  work:\n    job:\n      machine: ./bot.yaml\n") {
		t.Errorf("the office's work is:\n%s", office)
	}
}
