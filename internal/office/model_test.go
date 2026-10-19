package office_test

import (
	"strings"
	"testing"
)

func TestModelsAreGivenByName(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	if m := o.TopTeam().Models; len(m) != 1 || m[0].Name != "usual" || m[0].Text != "{ name: sonnet, service: claude-code }" {
		t.Errorf("the house's models are %#v", m)
	}
	if m := o.Teams["garden"].Models; len(m) != 1 || m[0].Text != "{ name: opus, service: claude-code }" {
		t.Errorf("the garden's models are %#v", m)
	}
	if m := reads(t, work(t)).TopTeam().Models; len(m) != 1 || m[0].Text != "{ name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }" {
		t.Errorf("the work office's models are %#v", m)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", "usual: { name: sonnet, service: claude-code }", "usual: sonnet")), "office.yaml", "models", "usual")
	says(t, refused(t, replace(t, house(t), "office.yaml", "usual: { name: sonnet, service: claude-code }", "Usual: { name: sonnet, service: claude-code }")), "office.yaml", "models", "Usual")
	// What a model holds is Corbie's to check, in each definition it is
	// filled into.
	o, problems := read(t, replace(t, house(t), "office.yaml", "usual: { name: sonnet, service: claude-code }", "usual: { name: sonnet, service: claude-code, address: https://x.example }"))
	if o == nil {
		t.Fatalf("the house is refused: %v", problems)
	}
	says(t, problems.OfWorker("fixer"), "fixer.yaml", "address")
	if o.Workers["fixer"].Runs() || !o.Workers["sparks"].Runs() || !o.Workers["gardener"].Runs() {
		t.Error("the model's refusal stops the wrong workers")
	}
}

func TestAOneWordModelIsTheNearestTeams(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	if got := o.Workers["fixer"].Model; got != "{ name: sonnet, service: claude-code }" {
		t.Errorf("the fixer's model is %q", got)
	}
	if got := o.Workers["gardener"].Model; got != "{ name: opus, service: claude-code }" {
		t.Errorf("the gardener's model is %q", got)
	}
	if !strings.Contains(string(o.Workers["fixer"].Written), "model: usual") || modelOf(t, o.Workers["fixer"]).Name != "sonnet" {
		t.Error("the fixer's file is not as written, or its definition is not filled")
	}
	beds := with(house(t), map[string]string{
		"garden/beds/team.yaml":   "lead: digger\n",
		"garden/beds/digger.yaml": "agent: digger\nmodel: usual\nprompt: dig\n",
	})
	if got := reads(t, beds).Workers["digger"].Model; got != "{ name: opus, service: claude-code }" {
		t.Errorf("a worker in a team with no models of its own is given %q", got)
	}
	noGarden := replace(t, house(t), "garden/team.yaml", "models:\n  usual: { name: opus, service: claude-code }   # the garden's own\n", "")
	if got := reads(t, noGarden).Workers["gardener"].Model; got != "{ name: sonnet, service: claude-code }" {
		t.Errorf("with the garden giving no models, the gardener is given %q", got)
	}
	// A machine in tools/ a worker runs is filled in from the team whose
	// folder holds it, and so is a rule's definition.
	files := with(house(t), map[string]string{"tools/helper.yaml": "agent: helper\nmodel: usual\nprompt: help\n"})
	files = replace(t, files, "clerk.yaml", "prompt: ${input.said}", "prompt: ${input.said}\ntools: { helper: { does: Help, args: {}, job: { machine: ./tools/helper.yaml } } }")
	o = reads(t, files)
	if child := o.Workers["clerk"].Draft.Children["./tools/helper.yaml"]; child == nil || !strings.Contains(string(child.Definition.Body), `"sonnet"`) {
		t.Errorf("the helper is %v", child)
	}
	// An agent written in place, in a worker or in a tools file, is filled
	// in from the team of the file it is written in.
	inline := "{ does: Help, args: {}, job: { machine: { agent: helper, model: usual, prompt: help } } }"
	files = replace(t, house(t), "garden/gardener.yaml", "tools:\n", "tools:\n  helper: "+inline+"\n")
	files = replace(t, files, "crew.yaml", "clerk:\n", "helper: "+inline+"\nclerk:\n")
	o = reads(t, files)
	if got := string(o.Workers["gardener"].Definition); !strings.Contains(got, "machine: { agent: helper, model: { name: opus, service: claude-code }, prompt: help }") || o.Workers["gardener"].Model != "{ name: opus, service: claude-code }" {
		t.Errorf("the gardener is sent as %s", got)
	}
	for w, model := range map[string]string{"gardener": `"opus"`, "steward": `"sonnet"`} {
		var helpers []string
		for _, child := range o.Workers[w].Draft.Children {
			if child.Definition.Name == "helper" {
				helpers = append(helpers, string(child.Definition.Body))
			}
		}
		if len(helpers) != 1 || !strings.Contains(helpers[0], model) {
			t.Errorf("%s's helpers are %v", w, helpers)
		}
	}
	// A rule's definition is filled in from its team's.
	files = replace(t, house(t), "garden/team.yaml", "decide: { everything: [sam] }", "decide: { everything: [./judge.yaml, sam] }")
	o = reads(t, with(files, map[string]string{"garden/judge.yaml": "agent: judge\nmodel: usual\nprompt: Decide.\n"}))
	if body := string(o.Rules["garden/judge.yaml"].Draft.Definition.Body); !strings.Contains(body, `"opus"`) {
		t.Errorf("the garden's rule is %s", body)
	}
}

func TestOfficeUnknownModelNameStopsWorker(t *testing.T) {
	t.Parallel()
	o, problems := read(t, replace(t, house(t), "clerk.yaml", "model: usual", "model: cheap"))
	if o == nil {
		t.Fatalf("the house is refused: %v", problems)
	}
	p := says(t, problems.OfWorker("clerk"), "clerk.yaml", `"cheap"`, "usual")
	if p.Refuses || o.Workers["clerk"].Runs() || !o.Workers["fixer"].Runs() {
		t.Errorf("the problem %v stops the wrong workers", p)
	}
	// A word is always a name, even one that is a model's own name.
	_, problems = read(t, replace(t, house(t), "clerk.yaml", "model: usual", "model: claude-sonnet-5-5"))
	says(t, problems.OfWorker("clerk"), "clerk.yaml", "claude-sonnet-5-5", "no model of that name")
	// The same of a bridge's machine, looked up from the office.
	files := with(own(t, house(t), "telegram"), map[string]string{"bridges/telegram/out.yaml": "agent: telegram-out\ninput: [settings, to, text]\nmodel: quick\nprompt: ${input.text}\n"})
	_, problems = read(t, files)
	if p := says(t, problems.OfBridge("telegram"), "bridges/telegram/out.yaml", `"quick"`); p.Bridge != "telegram" {
		t.Errorf("the problem is %#v", p)
	}
	// The same of a rule's definition, and of an agent written in place,
	// in a worker or in a tools file.
	files = replace(t, house(t), "garden/team.yaml", "decide: { everything: [sam] }", "decide: { everything: [./judge.yaml, sam] }")
	o, problems = read(t, with(files, map[string]string{"garden/judge.yaml": "agent: judge\nmodel: cheap\nprompt: Decide.\n"}))
	if o == nil {
		t.Fatalf("the house is refused: %v", problems)
	}
	says(t, problems.OfRule("garden/judge.yaml"), "garden/judge.yaml", `"cheap"`)
	if o.Rules["garden/judge.yaml"].Draft != nil || len(problems) != 1 || !o.Workers["gardener"].Runs() {
		t.Errorf("the rule's model stops more than the rule: %v", problems)
	}
	inline := "{ does: Help, args: {}, job: { machine: { agent: helper, model: cheap, prompt: help } } }"
	o, problems = read(t, replace(t, house(t), "clerk.yaml", "prompt: ${input.said}", "prompt: ${input.said}\ntools: { helper: "+inline+" }"))
	says(t, problems.OfWorker("clerk"), "clerk.yaml", `"cheap"`)
	if o.Workers["clerk"].Runs() || !o.Workers["fixer"].Runs() {
		t.Error("a word no team gives in place stops the wrong workers")
	}
	_, problems = read(t, replace(t, house(t), "crew.yaml", "clerk:\n", "helper: "+inline+"\nclerk:\n"))
	says(t, problems.OfWorker("steward"), "crew.yaml", `"cheap"`)
}
