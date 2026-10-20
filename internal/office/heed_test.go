package office_test

import (
	"testing"
)

// heeded gives the house with the garden heeding a rule in its folder.
func heeded(t *testing.T, heed string, files map[string]string) map[string]string {
	t.Helper()
	return with(replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\nheed: "+heed), files)
}

// forUs is a flow the garden may heed.
const forUs = "flow: for-us\nstages: { weigh: { job: ./for-us.js } }\n"

func TestATeamsHeedIsOneRule(t *testing.T) {
	t.Parallel()
	o := reads(t, heeded(t, "./for-us.js", map[string]string{"garden/for-us.js": "return { says: \"act\" }"}))
	h := o.Teams["garden"].Heed
	if h == nil || h.Rule != "garden/for-us.js" || h.Written != "./for-us.js" || h.Settings == nil || len(h.Settings) != 0 || h.Row != "" || !h.Runs() {
		t.Fatalf("the garden's heed is %#v", h)
	}
	if r := o.Rules["garden/for-us.js"]; r == nil || !r.Script || r.Team != "garden" {
		t.Errorf("the heed's rule is %#v", r)
	}
	if o.Teams["office"].Heed != nil {
		t.Errorf("the house heeds %#v", o.Teams["office"].Heed)
	}
	// With settings, handed on as written, and the rule a definition that
	// is no worker.
	o = reads(t, heeded(t, "{ rule: ./for-us.yaml, timeout: 5s, threshold: 0.6 }", map[string]string{
		"garden/for-us.yaml": forUs,
		"garden/for-us.js":   "return { says: \"act\" }",
	}))
	h = o.Teams["garden"].Heed
	if h == nil || h.Rule != "garden/for-us.yaml" || len(h.Settings) != 2 || h.Settings["timeout"] != "5s" || h.Settings["threshold"] != 0.6 || h.Row != "{ timeout: 5s, threshold: 0.6 }" {
		t.Fatalf("the garden's heed is %#v", h)
	}
	if r := o.Rules["garden/for-us.yaml"]; r == nil || r.Script || r.Draft == nil {
		t.Errorf("the heed's rule is %#v", r)
	}
	if o.Workers["for-us"] != nil {
		t.Error("the heed's rule is read as a worker")
	}
	// What is not one rule by its path, as a list would name it, refuses
	// the reading.
	for _, heed := range []string{"gardener", "[./for-us.js]", "{ timeout: 5s }", "./missing.js", "../small-orders.js"} {
		problems := refused(t, heeded(t, heed, map[string]string{"garden/for-us.js": "return 1"}))
		if p := says(t, problems, "garden/team.yaml", "heed"); len(problems) != 1 || p.Line == 0 {
			t.Errorf("heed: %s gives %v", heed, problems)
		}
	}
	// A rule Corbie refuses stops only the heed.
	o, problems := read(t, heeded(t, "./for-us.yaml", map[string]string{"garden/for-us.yaml": "flow: for-us\nstages: { weigh: {} }\n"}))
	if o == nil {
		t.Fatalf("the house does not read: %v", problems)
	}
	if h := o.Teams["garden"].Heed; h == nil || h.Runs() {
		t.Errorf("the garden's heed is %#v", h)
	}
	if found := problems.OfTeam("garden"); len(found) == 0 || found[0].File != "garden/for-us.yaml" {
		t.Errorf("the garden's problems are %v", found)
	}
	if !o.Workers["gardener"].Runs() {
		t.Error("the gardener does not run")
	}
	// Settings that do not fit what the rule takes stop only the heed,
	// said on the team file at the heed's line.
	o, problems = read(t, heeded(t, "{ rule: ../.bulldog/rules/for-us/for-us.yaml, threshold: high, treshold: 0.6 }", nil))
	if o == nil {
		t.Fatalf("the house does not read: %v", problems)
	}
	if h := o.Teams["garden"].Heed; h == nil || h.Runs() || len(h.Problems) != 1 {
		t.Errorf("the garden's heed is %#v", h)
	}
	if p := says(t, problems, "garden/team.yaml", "heed", "/settings/threshold", "treshold"); p.Line != o.Teams["garden"].Heed.Line || p.Refuses {
		t.Errorf("the heed's settings are said as %#v", p)
	}
	if o := reads(t, heeded(t, "{ rule: ../.bulldog/rules/for-us/for-us.yaml, timeout: 5s, threshold: 0.6 }", nil)); !o.Teams["garden"].Heed.Runs() {
		t.Errorf("the garden's heed with settings that fit is %#v", o.Teams["garden"].Heed)
	}
}

func TestOfficeHeedWithPersonLeadIsAProblemOfTheHeed(t *testing.T) {
	t.Parallel()
	files := with(replace(t, work(t), "support/team.yaml", "lead: dana", "lead: dana\nheed: ./for-us.js"), map[string]string{"support/for-us.js": "return 1"})
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the work office does not read: %v", problems)
	}
	p := says(t, problems.OfTeam("support"), "support/team.yaml", "heed", "./for-us.js", "dana", "person")
	if p.Refuses || p.Line == 0 {
		t.Errorf("the problem is %#v", p)
	}
	if len(problems) != 1 {
		t.Errorf("the problems are %v", problems)
	}
	h := o.Teams["support"].Heed
	if h == nil || h.Runs() {
		t.Errorf("support's heed is %#v", h)
	}
	if o.Teams["support"].Lead != "dana" {
		t.Errorf("support's lead is %q", o.Teams["support"].Lead)
	}
	// A worker lead's heed is asked.
	files = with(replace(t, work(t), "platform/team.yaml", "lead: ops", "lead: ops\nheed: ./for-us.js"), map[string]string{"platform/for-us.js": "return 1"})
	if h := reads(t, files).Teams["platform"].Heed; h == nil || !h.Runs() {
		t.Errorf("platform's heed is %#v", h)
	}
}
