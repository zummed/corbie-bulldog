package office_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
)

func TestTheKeysATeamFileTakes(t *testing.T) {
	t.Parallel()
	reads(t, house(t))
	reads(t, work(t))
	if o := reads(t, replace(t, house(t), "office.yaml", "lead: steward", "lead: steward\nruns: 2")); o.Runs != 2 {
		t.Errorf("runs is %d", o.Runs)
	}
	reads(t, with(replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\nheed: ./for-us.js"), map[string]string{"garden/for-us.js": "return 1"}))
	if o := reads(t, house(t)); o.Runs != office.DefaultRuns {
		t.Errorf("runs is %d with none written", o.Runs)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", "lead: steward", "lead: steward\nruns: 0")), "office.yaml", "runs")
	says(t, refused(t, replace(t, house(t), "office.yaml", "lead: steward", "lead: steward\nbudget: 40")), "office.yaml", "budget")
	for key, line := range map[string]string{"zone": "zone: Europe/London", "everyone": "everyone: {}", "bridges": "bridges: {}", "runs": "runs: 2"} {
		says(t, refused(t, replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\n"+line)), "garden/team.yaml", key)
	}
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "decide: { everything: [sam] }", "decide: { change: [sam] }")), "garden/team.yaml", "change")
}

func TestOfficeUnknownKeyRefused(t *testing.T) {
	t.Parallel()
	p := says(t, refused(t, replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\ntakes: groundsman")), "garden/team.yaml", "takes")
	if !p.Refuses || p.Line == 0 {
		t.Errorf("the problem is %#v", p)
	}
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\nzone: UTC")), "garden/team.yaml", "zone", "only in the top team's file")
	// A key written twice, anywhere in a team file, is never read past.
	for file, change := range map[string][2]string{
		"office.yaml":      {"lead: steward", "lead: steward\nlead: clerk"},
		"garden/team.yaml": {"people: [sam]", "people: [sam]\npeople: [sam, kit]"},
	} {
		p := says(t, refused(t, replace(t, house(t), file, change[0], change[1])), file, "written twice")
		if !p.Refuses || p.Line == 0 {
			t.Errorf("the problem is %#v", p)
		}
	}
	for old, new := range map[string]string{
		"sam: { name: Sam,":        "sam: { name: Sam, name: Samuel,",
		"  everything: [sam]\n":    "  everything: [sam]\n  everything: [kit]\n",
		"  usual: { name: sonnet,": "  usual: { name: haiku, service: claude-code }\n  usual: { name: sonnet,",
		"direct: {}":               "direct: { team: garden, team: office }",
		`at: "18:00"`:              `at: "18:00", at: "19:00"`,
	} {
		says(t, refused(t, replace(t, house(t), "office.yaml", old, new)), "office.yaml", "written twice")
	}
}

func TestTeamsAreKnownByTheirFolders(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if !slices.Equal(o.Order, []string{"office", "payments", "platform", "support"}) {
		t.Errorf("the teams are %v", o.Order)
	}
	for team, called := range map[string]string{"office": "Platform support", "platform": "Platform", "payments": "Payments", "support": "Support"} {
		if got := o.Teams[team].Called; got != called {
			t.Errorf("%s is called %q", team, got)
		}
	}
	if o.Name() != "Platform support" {
		t.Errorf("the office is called %q", o.Name())
	}
	o = reads(t, replace(t, work(t), "support/team.yaml", "name: Support\n", ""))
	if got := o.Teams["support"].Called; got != "support" {
		t.Errorf("support with no name is called %q", got)
	}
}

func TestOfficeNameIsOnlyWords(t *testing.T) {
	t.Parallel()
	for _, name := range []string{`""`, `"🌱 !!"`, `The Garden, and the Pond`} {
		o := reads(t, replace(t, house(t), "garden/team.yaml", "name: The garden", "name: "+name))
		g := o.Teams["garden"]
		if g == nil || g.Name != "garden" || g.Dir != "garden" || o.Workers["gardener"].Team != "garden" || o.Teams["garden"].Lead != "gardener" {
			t.Errorf("named %s, the garden is %#v", name, g)
		}
	}
	if got := reads(t, replace(t, house(t), "garden/team.yaml", "name: The garden", `name: ""`)).Teams["garden"].Called; got != "garden" {
		t.Errorf("an empty name is called %q", got)
	}
}

func TestATeamsPeopleAreThoseItNames(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if !slices.Equal(o.Teams["payments"].People, []string{"lee", "priya"}) || !slices.Equal(o.TopTeam().People, []string{"al"}) {
		t.Errorf("the people are %v and %v", o.Teams["payments"].People, o.TopTeam().People)
	}
	if got := o.TeamsOf("al"); !slices.Equal(got, []string{"office"}) {
		t.Errorf("al is in %v", got)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", "people: [sam, kit]", "people: [sam, nobody]")), "office.yaml", "nobody")
}

func TestEveryTeamsLeadIsOneWorkerOrPerson(t *testing.T) {
	t.Parallel()
	says(t, refused(t, replace(t, house(t), "office.yaml", "lead: steward", "lead: groundsman")), "office.yaml", "groundsman")
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "lead: gardener\n", "")), "garden/team.yaml", "lead")
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: [gardener, groundsman]")), "garden/team.yaml", "lead")
	if o := reads(t, work(t)); o.Teams["support"].Lead != "dana" {
		t.Errorf("support's lead is %q", o.Teams["support"].Lead)
	}
}

func written(ds []office.Decider) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Written)
	}
	return out
}

func TestATeamNeedWriteNoDecide(t *testing.T) {
	t.Parallel()
	o := reads(t, replace(t, house(t), "garden/team.yaml", "decide: { everything: [sam] }\n", ""))
	if got := written(o.Everything("garden")); !slices.Equal(got, []string{"sam"}) {
		t.Errorf("the garden's everything is %v", got)
	}
	w := reads(t, work(t))
	if got := w.Everything("platform"); len(got) != 0 {
		t.Errorf("platform's everything is %v", got)
	}
	o = reads(t, replace(t, house(t), "office.yaml", "  everything: [sam]\n", ""))
	if got := written(o.Everything("office")); !slices.Equal(got, []string{"sam", "kit"}) {
		t.Errorf("the house's everything is %v", got)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [./small-orders.js]")), "office.yaml", "no person")
}

func TestDecideAndDisclosureAreListsOfDeciders(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	if d := o.TopTeam().Decide; len(d) != 2 || d[0].Key != "everything" || d[1].Key != "order_part" || !slices.Equal(written(d[1].Deciders), []string{"./small-orders.js", "sam"}) {
		t.Errorf("the house decides %#v", d)
	}
	p := reads(t, work(t)).Teams["payments"]
	if !p.HasDisclosure || len(p.Disclosure) != 1 || p.Disclosure[0].Kind != office.IsRule || p.Disclosure[0].Name != "payments/no-account-numbers.yaml" {
		t.Errorf("payments discloses by %#v", p.Disclosure)
	}
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "decide: { everything: [sam] }", "decide: { everything: sam }")), "garden/team.yaml", "everything")
	says(t, refused(t, replace(t, work(t), "payments/team.yaml", "disclosure: [./no-account-numbers.yaml]", "disclosure: { lee: yes }")), "payments/team.yaml", "disclosure")
}

func TestADeciderIsAPersonAWorkerOrARule(t *testing.T) {
	t.Parallel()
	reads(t, work(t))
	o := reads(t, house(t))
	if d := o.TopTeam().Decide[1].Deciders; d[0].Kind != office.IsRule || d[0].Name != "small-orders.js" || d[1].Kind != office.IsPerson {
		t.Errorf("order_part is decided by %#v", d)
	}
	reads(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[./small-orders.js]"))
	for list, entry := range map[string]string{"[./missing.js, lee]": "./missing.js", "[nobody]": "nobody", "[lee, lee]": "lee is written twice", "[../bot.yaml]": "../bot.yaml"} {
		problems := refused(t, replace(t, work(t), "payments/team.yaml", "everything: [lee, priya]", "everything: "+list))
		if p := says(t, problems, "payments/team.yaml", entry); len(problems) != 1 {
			t.Errorf("%s gives more than its own problem %v: %v", list, p, problems)
		}
	}
	// A path that is no rule takes no worker out of the office.
	problems := refused(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[./garden/gardener.yaml, sam]"))
	if p := says(t, problems, "office.yaml", "./garden/gardener.yaml", "within"); len(problems) != 1 {
		t.Errorf("a rule's path in a team within gives more than %v: %v", p, problems)
	}
	says(t, refused(t, with(replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[./garden/check.js, sam]"), map[string]string{"garden/check.js": "return 1"})),
		"office.yaml", "./garden/check.js", "within")
	// A rule is a script, .js, or a definition, .yaml.
	says(t, refused(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[./notes/rooms.md, sam]")), "office.yaml", "./notes/rooms.md", "is not a rule")
	o = reads(t, replace(t, work(t), "platform/team.yaml", "lead: ops", "lead: ops\ndecide: { everything: [bot, al] }"))
	if d := o.Everything("platform"); d[0].Kind != office.IsWorker || d[1].Kind != office.IsPerson {
		t.Errorf("platform's everything is %#v", d)
	}
}

func TestARuleOrAWorkerMayBeMarkedToAdvise(t *testing.T) {
	t.Parallel()
	o := reads(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[./small-orders.js, clerk: advise, sam]"))
	d := o.TopTeam().Decide[1].Deciders
	if len(d) != 3 || d[0].Advises || d[1].Kind != office.IsWorker || d[1].Name != "clerk" || !d[1].Advises || d[2].Advises {
		t.Errorf("order_part is decided by %#v", d)
	}
	// A rule may advise, by its path, and a definition that advises is a
	// rule all the same, and no worker; the same worker may advise on one
	// list and decide plainly on another.
	o = reads(t, replace(t, work(t), "payments/team.yaml", "[./no-account-numbers.yaml]", "[./no-account-numbers.yaml: advise]"))
	if p := o.Teams["payments"]; len(p.Disclosure) != 1 || p.Disclosure[0].Kind != office.IsRule || !p.Disclosure[0].Advises || o.Workers["no-account-numbers"] != nil {
		t.Errorf("payments discloses by %#v", p.Disclosure)
	}
	o = reads(t, replace(t, house(t), "office.yaml", "  order_part: [./small-orders.js, sam]", "  order_part: [clerk: advise, sam]\n  cut_power: [clerk, sam]"))
	if a, b := o.TopTeam().Decide[1].Deciders[0], o.TopTeam().Decide[2].Deciders[0]; !a.Advises || b.Advises {
		t.Errorf("the clerk is %#v on order_part and %#v on cut_power", a, b)
	}
	// Any other marking, a person marked to advise, and a mapping of more
	// than one decider are each a problem of the file, at its line.
	for list, words := range map[string][]string{
		"[clerk: yes, sam]":                       {"clerk is marked yes", "advise"},
		"[clerk: { advise: true }, sam]":          {"clerk is marked a mapping"},
		"[./small-orders.js, sam: advise]":        {"sam is a person", "does not advise"},
		"[{ clerk: advise, sparks: advise }, sam]": {"this is a mapping"},
	} {
		problems := refused(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", list))
		if p := says(t, problems, append([]string{"office.yaml", "order_part"}, words...)...); p.Line == 0 || len(problems) != 1 {
			t.Errorf("%s gives %v, not its own problem at its line: %v", list, p, problems)
		}
	}
}

func TestARuleMayBeOneBulldogShips(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"./.bulldog/rules/none/none.js", "./.bulldog/tools/talk/talk.yaml"} {
		problems := refused(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "["+entry+", sam]"))
		if p := says(t, problems, "office.yaml", entry, "no rule Bulldog ships", ".bulldog/rules/"); len(problems) != 1 {
			t.Errorf("%s gives more than its own problem %v: %v", entry, p, problems)
		}
	}
	// From a team within, Bulldog's rules are at the top of the office.
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "everything: [sam]", "everything: [../.bulldog/rules/none/none.js, sam]")),
		"garden/team.yaml", "../.bulldog/rules/none/none.js", "no rule Bulldog ships")
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "everything: [sam]", "everything: [./.bulldog/rules/none/none.js, sam]")),
		"garden/team.yaml", "./.bulldog/rules/none/none.js", "not a file of the office")
	// The heeds Bulldog ships are read from what it ships, a definition
	// and a bare script, and are of no team.
	for heed, script := range map[string]bool{"../.bulldog/rules/for-us/for-us.yaml": false, "../.bulldog/rules/mentioned/mentioned.js": true} {
		o := reads(t, heeded(t, "{ rule: "+heed+", timeout: 5s }", nil))
		h := o.Teams["garden"].Heed
		if h == nil || !h.Runs() || h.Settings["timeout"] != "5s" {
			t.Fatalf("the garden's heed is %#v", h)
		}
		if r := o.Rules[h.Rule]; r == nil || r.Team != "" || r.Script != script || !script && r.Draft == nil || len(r.Written) == 0 {
			t.Errorf("%s is %#v", heed, r)
		}
	}
}

func TestOfficeTopEverythingNamesAPerson(t *testing.T) {
	t.Parallel()
	says(t, refused(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [./small-orders.js]")), "office.yaml", "everything names no person")
	files := replace(t, work(t), "office.yaml", "people: [al]\n", "")
	files = replace(t, files, "office.yaml", "decide: { everything: [al] }\n", "")
	says(t, refused(t, files), "office.yaml", "everything names no person")
	reads(t, replace(t, work(t), "office.yaml", "decide: { everything: [al] }\n", ""))
}

func TestOfficePeopleLastInList(t *testing.T) {
	t.Parallel()
	p := says(t, refused(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[sam, ./small-orders.js]")), "office.yaml", "order_part", "./small-orders.js", "after sam")
	if p.Team != "office" {
		t.Errorf("the problem is on %q", p.Team)
	}
	says(t, refused(t, replace(t, work(t), "payments/team.yaml", "everything: [lee, priya]", "everything: [lee, bot]")), "payments/team.yaml", "bot", "after lee")
}

func TestOfficeUnknownDeciderIsAProblem(t *testing.T) {
	t.Parallel()
	p := says(t, refused(t, replace(t, house(t), "office.yaml", "[./small-orders.js, sam]", "[./small-order.js, sam]")), "office.yaml", "order_part", "./small-order.js")
	if p.Team != "office" || p.Line == 0 {
		t.Errorf("the problem is %#v", p)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [sma]")), "office.yaml", "everything", "sma", "nobody")
}

func TestOfficeRuleDefinitionIsNotAWorker(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if o.Workers["no-account-numbers"] != nil {
		t.Fatal("the rule is a worker")
	}
	r := o.Rules["payments/no-account-numbers.yaml"]
	if r == nil || r.Script || r.Draft == nil || r.Team != "payments" {
		t.Errorf("the rule is %#v", r)
	}
	// Named by the house too, a definition in its folder is a rule there.
	files := with(house(t), map[string]string{"reviewer.yaml": "flow: reviewer\nstages: { look: { job: ./small-orders.js } }\n"})
	files = replace(t, files, "office.yaml", "[./small-orders.js, sam]", "[./reviewer.yaml, sam]")
	if o := reads(t, files); o.Workers["reviewer"] != nil || o.Rules["reviewer.yaml"] == nil {
		t.Error("a definition named in a list is read as a worker")
	}
	// So is one a heed names.
	files = with(house(t), map[string]string{"reviewer.yaml": "flow: reviewer\nstages: { look: { job: ./small-orders.js } }\n"})
	files = replace(t, files, "office.yaml", "lead: steward", "lead: steward\nheed: { rule: ./reviewer.yaml }")
	if o := reads(t, files); o.Workers["reviewer"] != nil || o.Rules["reviewer.yaml"] == nil {
		t.Error("a definition a heed names is read as a worker")
	}
}

func TestOfficeRuleGrantedNothing(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	// A bare script is its file as written, and Bulldog writes no
	// definition around it.
	script := reads(t, house(t)).Rules["small-orders.js"]
	if script == nil || !script.Script || !script.Runs() || script.Problems != nil {
		t.Fatalf("the bare rule is %#v", script)
	}
	if script.Draft != nil {
		t.Errorf("a definition is written around the bare rule: %s", script.Draft.Definition.Body)
	}
	if string(script.Written) != house(t)["small-orders.js"] {
		t.Errorf("the bare rule is %q", script.Written)
	}
	// A definition holds what its own allow lines give, and nothing more.
	r := o.Rules["payments/no-account-numbers.yaml"]
	if body := string(r.Draft.Definition.Body); !strings.Contains(body, "net:screen.example.com") || !strings.Contains(body, "secret:SCREEN_KEY@net:screen.example.com") {
		t.Errorf("the rule's definition is %s", body)
	}
	if len(r.Draft.Definition.Allow) > 0 || len(r.Draft.Definition.Deny) > 0 {
		t.Errorf("the rule is given %v, and denied %v", r.Draft.Definition.Allow, r.Draft.Definition.Deny)
	}
}

func TestARuleWrittenForAnOlderBulldogIsAProblemThatDoesNotStopIt(t *testing.T) {
	t.Parallel()
	files := replace(t, house(t), "small-orders.js", `says: "refer"`, `says: "pass"`)
	o, problems := read(t, files)
	if o == nil || !o.Rules["small-orders.js"].Runs() {
		t.Fatalf("the house does not read, or its rule does not run: %v", problems)
	}
	if p := says(t, problems, "small-orders.js:6:", "answers pass", `says: "refer"`); p.Team != "office" || p.Rule != "" || p.Refuses {
		t.Errorf("the problem is %#v", p)
	}
	// A definition's files are read too, and a comment saying the rule
	// passes something on is no problem.
	files = replace(t, work(t), "payments/no-account-numbers.yaml", "input: ${input}", `input: { text: "${input.text}", before: "${input.passes}" }`)
	files = replace(t, files, "payments/no-account-numbers.js", `says: "refer"`, `says: 'pass'`)
	files = replace(t, files, "payments/no-account-numbers.js", "// Says yes", "// It passes nothing it is unsure of. Says yes")
	_, problems = read(t, files)
	says(t, problems, "payments/no-account-numbers.yaml:9:", "reads passes", "referrals")
	says(t, problems, "payments/no-account-numbers.js:11:", "answers pass")
	if found := saying(problems, "payments/"); len(found) != 2 {
		t.Errorf("the problems of payments' rule are %v", found)
	}
}

func TestADecideKeyThatNamesNoToolIsReportedOnItsTeam(t *testing.T) {
	t.Parallel()
	o, problems := read(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [sam]\n  order_prat: [sam]"))
	if o == nil || !o.Workers["steward"].Runs() {
		t.Fatalf("the house does not read: %v", problems)
	}
	if p := says(t, problems, "office.yaml", "order_prat"); p.Team != "office" || p.Refuses {
		t.Errorf("the problem is %#v", p)
	}
	// While a worker of the team cannot be read, no key is said to name
	// no tool.
	files := replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [sam]\n  order_prat: [sam]")
	_, problems = read(t, replace(t, files, "clerk.yaml", "prompt:", "promt:"))
	if len(saying(problems, "order_prat")) > 0 {
		t.Errorf("a key is said to name no tool while a worker cannot be read: %v", problems)
	}
	// A worker stopped for what its definition names, once Corbie has read
	// it, holds tools that are known.
	_, problems = read(t, replace(t, files, "steward.yaml", "ask: { machine: ./.bulldog/ask.yaml }", ""))
	says(t, problems.OfWorker("steward"), "ask")
	says(t, problems.OfTeam("office"), "order_prat")
}

func TestADecideKeyMayNameAToolOfAListedEntry(t *testing.T) {
	t.Parallel()
	// The fixer is given the tools a GitHub server lists, which are known
	// only when a call is made: a key named after the entry and a tool is
	// checked then, and one named after no entry is still reported.
	files := replace(t, house(t), "fixer.yaml", "tools:\n", "tools:\n  github: { list: { script: ./tools/github.js }, job: { script: ./tools/github.js, input: { tool: \"${tool}\", args: \"${args}\" }, approve: true } }\n")
	files["tools/github.js"] = "return {}"
	files = replace(t, files, "office.yaml", "everything: [sam]", "everything: [sam]\n  github_create_issue: [kit]\n  gitlab_x: [kit]")
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the house does not read: %v", problems)
	}
	if tools := o.Workers["fixer"].Held; !slices.ContainsFunc(tools, func(tool office.Tool) bool { return tool.Name == "github" && tool.Listed }) {
		t.Errorf("the fixer holds %+v", tools)
	}
	if found := saying(problems, "github_create_issue"); len(found) > 0 {
		t.Errorf("a key of a listed tool is said to name no tool: %v", found)
	}
	says(t, problems.OfTeam("office"), "gitlab_x", "names no tool")
}

func TestOfficeToolsFromFilledDefinition(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	steward := o.Workers["steward"].Tools
	for _, tool := range []string{"fixer", "clerk", "builder", "ask_team", "ask_person", "tell", "write_notes"} {
		if !slices.Contains(steward, tool) {
			t.Errorf("the steward holds %v, without %s", steward, tool)
		}
	}
	if slices.Contains(o.Workers["fixer"].Tools, "write_notes") || !slices.Contains(o.Workers["fixer"].Tools, "order_part") {
		t.Errorf("the fixer holds %v", o.Workers["fixer"].Tools)
	}
	// A tool given only by a tools file may be decided on.
	reads(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [sam]\n  clerk: [sam]"))
	waits := map[string]bool{}
	for _, tool := range o.Workers["fixer"].Held {
		waits[tool.Name] = tool.Waits
	}
	if !waits["cut_power"] || !waits["order_part"] || waits["sparks"] {
		t.Errorf("the fixer's tools that wait for a yes: %v", waits)
	}
}

func TestOfficeWhatAFlowsStagesAndAStatechartsJobsRun(t *testing.T) {
	t.Parallel()
	// runs fails unless a worker runs just what is wanted, by name, what
	// each job is and whether it waits for a yes.
	runs := func(o *office.Office, worker string, want ...office.Tool) {
		t.Helper()
		got := o.Workers[worker].Held
		if !slices.EqualFunc(got, want, func(a, b office.Tool) bool {
			return a.Name == b.Name && a.Kind == b.Kind && a.Worker == b.Worker && a.File == b.File && a.Waits == b.Waits
		}) {
			t.Errorf("%s runs %+v", worker, got)
		}
	}
	runs(reads(t, house(t)), "bins", office.Tool{Name: "which", Kind: office.ToolScript, File: "tools/bins.js"})

	// A held job is decided by its stage's name, so a key may name one.
	reads(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [sam]\n  which: [sam]"))
	_, problems := read(t, replace(t, house(t), "office.yaml", "everything: [sam]", "everything: [sam]\n  whcih: [sam]"))
	says(t, problems.OfTeam("office"), "whcih", "names no tool")

	// A worker a flow runs is called by it, and the flow, the lead, still
	// names the ask, since the fixer's cut_power waits for a yes.
	steward := "flow: steward\ninput: { said: string, context: string }\nstages:\n  mend: { job: { machine: ./fixer.yaml, input: { said: \"${input.said}\" } } }\n" +
		"output: ${stages.mend.output}\nask: { machine: ./.bulldog/ask.yaml }\n"
	runs(reads(t, with(house(t), map[string]string{"steward.yaml": steward})), "steward",
		office.Tool{Name: "mend", Kind: office.ToolWorker, Worker: "fixer", File: "fixer.yaml"})
	_, problems = read(t, with(house(t), map[string]string{"steward.yaml": strings.Replace(steward, "ask: { machine: ./.bulldog/ask.yaml }\n", "", 1)}))
	says(t, problems, "steward", "ask")

	// A statechart runs a state's one job by the state's name, and each
	// named job by its own.
	porch := "statechart: porch\ninput: { said: string }\ninitial: asking\nstates:\n" +
		"  asking:\n    job: { machine: ./sparks.yaml, input: { said: \"Which circuit is the porch?\" } }\n    on: { done: locking }\n" +
		"  locking:\n    jobs:\n      lock: { script: ./tools/cut-power.js, input: { circuit: porch }, approve: true }\n    on: { done: locked }\n" +
		"  locked: { end: done }\n"
	o := reads(t, replace(t, with(house(t), map[string]string{"porch.yaml": porch}), "office.yaml", "everything: [sam]", "everything: [sam]\n  lock: [kit]"))
	runs(o, "porch",
		office.Tool{Name: "asking", Kind: office.ToolWorker, Worker: "sparks", File: "sparks.yaml"},
		office.Tool{Name: "lock", Kind: office.ToolScript, File: "tools/cut-power.js", Waits: true})
}

func TestAboutIsOneLine(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if got := reads(t, house(t)).Teams["garden"].About; got != "The beds, the lawn and the shed." {
		t.Errorf("the garden's about is %q", got)
	}
	if o.Teams["support"].About == "" {
		t.Error("support's about is not read")
	}
	if got := o.Teams["office"].About; got != "Help with the platform and with payments, asked in #help." {
		t.Errorf("the office's about is %q", got)
	}
	if got := reads(t, replace(t, work(t), "support/team.yaml", "about: Customers' questions, and what they are told.\n", "")).Teams["support"].About; got != "" {
		t.Errorf("support's about is %q with none written", got)
	}
	p := says(t, refused(t, replace(t, house(t), "garden/team.yaml", "about: The beds, the lawn and the shed.", "about: |\n  The beds.\n  The lawn.")), "garden/team.yaml", "about", "garden")
	if !p.Refuses {
		t.Error("an about of two lines does not refuse the reading")
	}
}
