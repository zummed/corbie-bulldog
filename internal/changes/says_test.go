package changes

import (
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// saysOf gives what a person's change of whole files says, and what it
// lets more by: a file given as "" is removed.
func saysOf(t *testing.T, f *officeAt, changes map[string]string) Check {
	t.Helper()
	e := Edit{Files: map[string]*string{}}
	for file, text := range changes {
		if text == "" {
			e.Files[file] = nil
		} else {
			e.Files[file] = &text
		}
	}
	files, _, err := f.c.Edited(e)
	if err != nil {
		t.Fatal(err)
	}
	check, err := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	if err != nil {
		t.Fatal(err)
	}
	return check
}

// replaced gives a file of an office's with a text in it replaced.
func replaced(t *testing.T, files map[string]string, file string, pairs ...string) string {
	t.Helper()
	text := files[file]
	for i := 0; i+1 < len(pairs); i += 2 {
		if !strings.Contains(text, pairs[i]) {
			t.Fatalf("%s does not hold %q", file, pairs[i])
		}
		text = strings.Replace(text, pairs[i], pairs[i+1], 1)
	}
	return text
}

func TestChangesSayWhatChangesInTeams(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["garden/team.yaml"] += "disclosure: [sam]\n"
	f := newOffice(t, files)
	change := map[string]string{
		"pond/team.yaml": "name: The pond\nabout: The water.\npeople: [sam]\nlead: sam\n",
		"office.yaml": replaced(t, files, "office.yaml",
			"about: The house and everything in it, but the garden.", "about: The house, and the pond.",
			"lead: steward", "lead: clerk",
			"  everything: [sam]\n", "  everything: [sam, kit]\n",
			"  - { at: \"18:00\"", "  - { at: \"07:00\", to: bins, say: \"Morning\" }\n  - { at: \"18:00\"",
			"models:\n", "models:\n  quick: { name: haiku, service: claude-code }\n",
			"zone: Europe/London", "zone: Europe/Paris\nruns: 5"),
		"notes/rooms.md": "# Rooms\n\nAll of them.\n",
	}
	// The garden moves inside the pond.
	for file, text := range files {
		if rest, ok := strings.CutPrefix(file, "garden/"); ok {
			change[file] = ""
			text = strings.ReplaceAll(text, "../.bulldog/", "../../.bulldog/")
			if rest == "team.yaml" {
				text = strings.Replace(strings.Replace(text, "people: [sam]", "people: [sam, kit]", 1), "disclosure: [sam]\n", "", 1)
			}
			change["pond/garden/"+rest] = text
		}
	}
	check := saysOf(t, f, change)
	all := append(check.Says, check.LetsMoreBy...)
	has(t, all, "the pond is a new team, within the house")
	has(t, all, "the garden moves from within the house to within the pond")
	has(t, all, "the house looks after: The house, and the pond.")
	has(t, check.LetsMoreBy, "kit is named in the garden")
	has(t, check.Says, "clerk is the lead of the house")
	has(t, check.Says, "the house decides everything by sam, kit, where it was sam")
	has(t, check.LetsMoreBy, "the garden's disclosure check is taken away")
	has(t, check.Says, "the house has a routine: at 07:00, bins is told \"Morning\"")
	has(t, check.Says, "the house gives the model quick as { name: haiku, service: claude-code }")
	has(t, check.Says, "the house's note rooms is changed")
	has(t, check.Says, "the office's zone is now Europe/Paris, where it was Europe/London")
	has(t, check.Says, "the office now has 5 runs at work at once, where it had 3")
}

func TestChangesSayWhatChangesInPeople(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	check := saysOf(t, f, map[string]string{"office.yaml": replaced(t, house(t), "office.yaml",
		`  kit: { name: Kit, title: Lives here., telegram: "4472" }`, `  kit: { name: Kit, title: Lives here., telegram: "9999" }`+"\n"+`  jo: { name: Jo, telegram: "5555" }`)})
	has(t, check.Says, "jo joins the office")
	has(t, check.LetsMoreBy, "jo is 5555 on telegram")
	has(t, check.LetsMoreBy, "kit is now 9999 on telegram, where they were 4472")
	// Taking an identity away narrows.
	check = saysOf(t, f, map[string]string{"office.yaml": replaced(t, house(t), "office.yaml", `, telegram: "4472"`, ``)})
	has(t, check.Says, "kit is no longer reached on telegram, where they were 4472")
}

func TestChangesSayWhatChangesInWorkers(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	check := saysOf(t, f, map[string]string{"weather.yaml": "agent: weather\ninput: { said: string }\nmodel: usual\nsystem: The weather.\nprompt: ${input.said}\n"})
	has(t, check.Says, "weather joins the house")
	has(t, check.Says, "weather's model is sonnet (claude-code)")
	check = saysOf(t, f, map[string]string{"fixer.yaml": replaced(t, house(t), "fixer.yaml",
		"model: usual", `model: { name: sonnet, service: anthropic, address: "http://localhost:9", secret: TEST_KEY }`,
		`input: { circuit: "${args.circuit}" }, approve: true }`, `input: { circuit: "${args.circuit}" }, approve: true, allow: [net:board.local, "secret:BOARD_KEY@net:board.local"] }`,
		`input: { part: "${args.part}", pounds: "${args.pounds}" }, approve: true }`, `input: { part: "${args.part}", pounds: "${args.pounds}" } }`,
		"tools:\n", "tools:\n  lights: { does: The lights, args: {}, job: { script: ./tools/circuits.js } }\n")})
	has(t, check.LetsMoreBy, "fixer's model is now sonnet (anthropic) at http://localhost:9, with the secret TEST_KEY, where it was sonnet (claude-code)")
	has(t, check.LetsMoreBy, "fixer may connect to board.local")
	has(t, check.LetsMoreBy, "fixer may send the secret BOARD_KEY to board.local")
	has(t, check.Says, "fixer has the tool lights")
	has(t, check.LetsMoreBy, "fixer's tool order_part no longer waits for a yes")
	// A flow's stages and a statechart's jobs are said as an agent's tools
	// are.
	check = saysOf(t, f, map[string]string{
		"bins.yaml":  replaced(t, house(t), "bins.yaml", "  which: { job: { script: ./tools/bins.js, input: { request: \"${input.request}\" } } }", "  sort: { job: { script: ./tools/bins.js, input: { request: \"${input.request}\" }, approve: true } }", "output: ${stages.which.output}\n", "output: ${stages.sort.output}\nask: { machine: ./.bulldog/ask.yaml }\n"),
		"porch.yaml": porch(true),
	})
	has(t, check.Says, "bins has the stage sort, which waits for a yes")
	has(t, check.Says, "bins no longer has the stage which")
	has(t, check.Says, "porch has the job asking")
	has(t, check.Says, "porch has the job lock, which waits for a yes")
	// The house's usual at a new address is said for each worker of the
	// house that names it, and for no garden worker, whose usual is the
	// garden's.
	check = saysOf(t, f, map[string]string{"office.yaml": replaced(t, house(t), "office.yaml",
		"usual: { name: sonnet, service: claude-code }", `usual: { name: sonnet, service: anthropic, address: "http://localhost:9", secret: TEST_KEY }`)})
	for _, worker := range []string{"steward", "fixer", "clerk", "builder"} {
		has(t, check.LetsMoreBy, worker+"'s model is now sonnet (anthropic) at http://localhost:9")
	}
	hasNo(t, append(check.Says, check.LetsMoreBy...), "gardener")
	hasNo(t, append(check.Says, check.LetsMoreBy...), "groundsman")
	hasNo(t, append(check.Says, check.LetsMoreBy...), "sparks' model")
}

// porch is a statechart that asks sparks which circuit is the porch's, then
// cuts it, waiting for a yes or not.
func porch(waits bool) string {
	approve := ""
	if waits {
		approve = ", approve: true"
	}
	return "statechart: porch\ninput: { said: string }\ninitial: asking\nstates:\n" +
		"  asking:\n    job: { machine: ./sparks.yaml, input: { said: \"Which circuit is the porch?\" } }\n    on: { done: locking }\n" +
		"  locking:\n    jobs:\n      lock: { script: ./tools/cut-power.js, input: { circuit: porch }" + approve + " }\n    on: { done: locked }\n" +
		"  locked: { end: done }\n"
}

func TestChangesSayWhatChangesInRulesAndBridges(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	check := saysOf(t, f, map[string]string{"small-orders.js": house(t)["small-orders.js"] + "// changed\n"})
	has(t, check.LetsMoreBy, "the house's rule small-orders.js is changed, and decides on its order_part list")
	w := newOffice(t, work(t))
	files := work(t)
	check = saysOf(t, w, map[string]string{
		"payments/no-account-numbers.yaml": replaced(t, files, "payments/no-account-numbers.yaml", "allow: [net:screen.example.com,", "allow: [net:screen.example.com, net:other.example,"),
		"bridges/slack/out.yaml":           replaced(t, files, "bridges/slack/out.yaml", "allow: [net:slack.com,", "allow: [net:slack.com, net:files.slack.com,"),
		"office.yaml":                      replaced(t, files, "office.yaml", `      "#help": { guests: true }`, `      "#help": { guests: true }`+"\n"+`      "#deploys": { team: platform, guests: true }`),
	})
	has(t, check.LetsMoreBy, "Payments' rule payments/no-account-numbers.yaml", "disclosure check")
	has(t, check.LetsMoreBy, "Payments' rule payments/no-account-numbers.yaml may connect to other.example")
	has(t, check.LetsMoreBy, "slack's out machine may connect to files.slack.com")
	has(t, check.LetsMoreBy, "slack's settings are changed")
	has(t, check.Says, "slack listens in #deploys, for Platform, where guests may ask")
	// A heed is said, and so is the rule it names.
	check = saysOf(t, f, map[string]string{
		"garden/team.yaml": replaced(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\nheed: { rule: ./for-us.js, timeout: 5s }"),
		"garden/for-us.js": "return { says: \"act\" }\n",
	})
	has(t, check.Says, "the garden's heed is ./for-us.js, with { timeout: 5s }.")
	has(t, check.Says, "the garden's rule garden/for-us.js is new, on its heed.")
}

func TestChangesWhatLetsMoreByIsApartEverywhere(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	more := replaced(t, house(t), "fixer.yaml", `input: { part: "${args.part}", pounds: "${args.pounds}" }, approve: true }`, `input: { part: "${args.part}", pounds: "${args.pounds}" } }`)
	// To the builder.
	f.write(1, "fixer.yaml", more)
	f.write(1, "notes/fuses.md", "Breaker 4.\n")
	text := f.call(1, ToolCheck, nil).Text
	apart := strings.Index(text, "What it lets more by")
	if apart < 0 || strings.Index(text, "order_part no longer waits") < apart || strings.Index(text, "note fuses is new") > apart {
		t.Errorf("the builder is told:\n%s", text)
	}
	// To a person drafting.
	check := saysOf(t, f, map[string]string{"fixer.yaml": more, "notes/fuses.md": "Breaker 4.\n"})
	has(t, check.LetsMoreBy, "order_part no longer waits")
	has(t, check.Says, "note fuses is new")
	// To whoever decides, and in the commit.
	p := f.put(1, "Order alone")
	if len(p.LetsMoreBy) != 1 || len(p.Says) != 1 || p.LetsMoreBy[0] != check.LetsMoreBy[0] || p.Says[0] != check.Says[0] {
		t.Errorf("whoever decides is shown %v apart from %v", p.LetsMoreBy, p.Says)
	}
	f.yes(p.ID, "sam", "")
	h := f.history()[0]
	if len(h.LetsMoreBy) != 1 || h.LetsMoreBy[0] != check.LetsMoreBy[0] || len(h.Does) != 1 || h.Does[0] != check.Says[0] {
		t.Errorf("the commit says %v apart from %v", h.LetsMoreBy, h.Does)
	}
}

func TestChangesSaidFromTheFilesAlone(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "sparks.yaml", replaced(t, house(t), "sparks.yaml", "job: { script: ./tools/circuits.js }", "job: { script: ./tools/circuits.js, allow: [net:mains.example] }"))
	p := f.put(1, "A tiny tweak to the wording")
	has(t, p.LetsMoreBy, "sparks may connect to mains.example")
	hasNo(t, append(p.Says, p.LetsMoreBy...), "tiny tweak")
}

func TestChangesEveryFileIsSaid(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	check := saysOf(t, f, map[string]string{
		"tools/unused.js":   "return 1\n",
		"tools/circuits.js": "return {}\n",
		"tools/order.js":    "",
		"README.md":         "The house.\n",
	})
	has(t, check.Says, "tools/unused.js is new")
	has(t, check.Says, "tools/circuits.js is changed")
	has(t, check.Says, "tools/order.js is gone")
	has(t, check.Says, "README.md is new")
	// A rule taken off its list with its file is said once.
	check = saysOf(t, f, map[string]string{
		"small-orders.js": "",
		"office.yaml":     replaced(t, house(t), "office.yaml", "order_part: [./small-orders.js, sam]", "order_part: [sam]"),
	})
	has(t, check.Says, "rule small-orders.js is on none of its lists")
	hasNo(t, append(check.Says, check.LetsMoreBy...), "small-orders.js is gone")
}

func TestChangesWhatLetsMoreBy(t *testing.T) {
	t.Parallel()
	w := work(t)
	// A worker and a rule with a problem the office has now, that Corbie
	// could not read.
	broken := house(t)
	broken["clerk.yaml"] += "nonsense: 1\n"
	brokenRule := work(t)
	brokenRule["payments/no-account-numbers.yaml"] += "nonsense: 1\n"
	// The work office with the bridges Bulldog ships.
	shippedWork := testkit.Files(t, examples.Work())
	// The house with no bridge, to which one is added.
	bridgeless := map[string]string{}
	for file, text := range house(t) {
		if !strings.HasPrefix(file, "bridges/") {
			bridgeless[file] = text
		}
	}
	// The house with a flow's stage and a statechart's job that wait.
	waiting := house(t)
	waiting["bins.yaml"] = replaced(t, waiting, "bins.yaml", "\"${input.request}\" } } }", "\"${input.request}\" }, approve: true } }", "output: ${stages.which.output}\n", "output: ${stages.which.output}\nask: { machine: ./.bulldog/ask.yaml }\n")
	waiting["porch.yaml"] = porch(true)
	bridgeless["office.yaml"] = replaced(t, bridgeless, "office.yaml", "bridges:\n  telegram:\n    places:\n      direct: {}                   # as Bulldog ships it: Sam and Kit may ask the house in a direct message\n", "")
	advising := house(t)
	advising["office.yaml"] = replaced(t, advising, "office.yaml", "  everything: [sam]\n", "  everything: [sam]\n  cut_power: [clerk: advise, sam]\n")
	for _, c := range []struct {
		files  map[string]string
		change map[string]string
		says   string
	}{
		{house(t), map[string]string{"sparks.yaml": replaced(t, house(t), "sparks.yaml", "job: { script: ./tools/circuits.js }", "job: { script: ./tools/circuits.js, allow: [fs.read:/etc] }")}, "sparks may read /etc"},
		{house(t), map[string]string{"sparks.yaml": replaced(t, house(t), "sparks.yaml", "job: { script: ./tools/circuits.js }", `job: { script: ./tools/circuits.js, allow: ["secret:K@net:x.example"] }`)}, "sparks may send the secret K to x.example"},
		{house(t), map[string]string{"sparks.yaml": replaced(t, house(t), "sparks.yaml", "model: { name: haiku, service: claude-code }", `model: { name: haiku, service: anthropic, address: "http://localhost:9" }`)}, "sparks' model is now"},
		{house(t), map[string]string{"fixer.yaml": replaced(t, house(t), "fixer.yaml", `"${args.circuit}" }, approve: true }`, `"${args.circuit}" } }`)}, "cut_power no longer waits"},
		{waiting, map[string]string{"bins.yaml": house(t)["bins.yaml"]}, "bins' stage which no longer waits for a yes"},
		{waiting, map[string]string{"porch.yaml": porch(false)}, "porch's job lock no longer waits for a yes"},
		{house(t), map[string]string{"garden/team.yaml": replaced(t, house(t), "garden/team.yaml", "people: [sam]", "people: [sam, kit]")}, "kit is named in the garden"},
		{w, map[string]string{"platform/team.yaml": w["platform/team.yaml"] + "decide: { everything: [raj] }\n"}, "raj now stands as one of Platform's people"},
		{house(t), map[string]string{"office.yaml": replaced(t, house(t), "office.yaml", `"4472"`, `"4473"`)}, "kit is now 4473 on telegram"},
		{house(t), map[string]string{"office.yaml": replaced(t, house(t), "office.yaml", "order_part: [./small-orders.js, sam]", "order_part: [sam]")}, "decides order_part by sam, where it was ./small-orders.js, sam"},
		{house(t), map[string]string{"office.yaml": replaced(t, house(t), "office.yaml", "  everything: [sam]\n", "  everything: [./small-orders.js, sam]\n")}, "decides everything by ./small-orders.js, sam"},
		{house(t), map[string]string{"office.yaml": replaced(t, house(t), "office.yaml", "  order_part: [./small-orders.js, sam]\n", "")}, "order_part list is taken away"},
		{advising, map[string]string{"office.yaml": replaced(t, advising, "office.yaml", "cut_power: [clerk: advise, sam]", "cut_power: [clerk, sam]")}, "decides cut_power by clerk, sam, where it was clerk: advise, sam"},
		{house(t), map[string]string{"small-orders.js": house(t)["small-orders.js"] + "\n"}, "rule small-orders.js is changed"},
		{house(t), map[string]string{"sparks.yaml": replaced(t, house(t), "sparks.yaml", "model: { name: haiku, service: claude-code }", `model: { name: haiku, service: claude-code, secret: K }`)}, "sparks' model is now haiku (claude-code), with the secret K"},
		{house(t), map[string]string{"bridges/telegram/out.yaml": house(t)["bridges/telegram/out.yaml"] + "# x\n"}, "telegram's out machine is changed"},
		{house(t), map[string]string{"bridges/telegram/in.yaml": house(t)["bridges/telegram/in.yaml"] + "# x\n"}, "telegram's in machine is changed"},
		{inPlaceIn(t), map[string]string{"bridges/telegram/verify.js": "return input.heard // changed\n"}, "telegram's in machine is changed"},
		{bridgeless, map[string]string{"bridges/telegram/in.yaml": house(t)["bridges/telegram/in.yaml"], "bridges/telegram/out.yaml": house(t)["bridges/telegram/out.yaml"], "bridges/telegram/telegram.js": house(t)["bridges/telegram/telegram.js"], "office.yaml": house(t)["office.yaml"]}, "the bridge telegram is new"},
		{bridgeless, map[string]string{"office.yaml": testkit.Files(t, examples.House())["office.yaml"]}, "the bridge telegram is new"},
		// Mending a worker or a rule Corbie could not read says all it
		// may reach and every tool it has.
		{broken, map[string]string{"clerk.yaml": house(t)["clerk.yaml"] + "tools:\n  fetch: { does: Fetch, args: {}, job: { script: ./tools/order.js, allow: [net:evil.example] } }\n"}, "clerk may connect to evil.example"},
		{brokenRule, map[string]string{"payments/no-account-numbers.yaml": replaced(t, w, "payments/no-account-numbers.yaml", "allow: [net:screen.example.com,", "allow: [net:evil.example, net:screen.example.com,")}, "rule payments/no-account-numbers.yaml may connect to evil.example"},
		{house(t), map[string]string{"office.yaml": replaced(t, house(t), "office.yaml", "      direct: {}", "      direct: {}\n    token: other")}, "telegram's settings are changed"},
		{shippedWork, map[string]string{"office.yaml": replaced(t, shippedWork, "office.yaml", "./.bulldog/bridges/slack", "./.bulldog/bridges/telegram")}, "slack's settings are changed"},
		{shippedWork, map[string]string{"office.yaml": replaced(t, shippedWork, "office.yaml", "      platform/api: { team: platform }\n", "      platform/api: { team: platform }\n      platform/web: { team: platform }\n")}, "gitlab's settings are changed"},
	} {
		f := newOffice(t, c.files)
		check := saysOf(t, f, c.change)
		has(t, check.LetsMoreBy, c.says)
		hasNo(t, check.Says, c.says)
	}
	// A refusal taken from a job, or from the whole definition, of what
	// the job is allowed; and one of two refusals that are the same.
	refused := func(job, top string) map[string]string {
		files := house(t)
		files["sparks.yaml"] = replaced(t, files, "sparks.yaml", "job: { script: ./tools/circuits.js }", job, "agent: sparks\n", "agent: sparks\n"+top)
		return files
	}
	twice := refused("job: { script: ./tools/circuits.js, allow: [network], deny: [net:evil.example] }", "")
	twice["sparks.yaml"] = replaced(t, twice, "sparks.yaml", "tools:\n", "tools:\n  meter: { does: The meter, args: {}, job: { script: ./tools/circuits.js, allow: [network], deny: [net:evil.example] } }\n")
	// A tool that waits for a yes because the agent lists what it
	// declares to be asked about, as a permission or a group.
	askAbout := func(allow string) map[string]string {
		files := house(t)
		files["fixer.yaml"] = replaced(t, files, "fixer.yaml", `"${args.circuit}" }, approve: true }`, `"${args.circuit}" }`+allow+` }`, "agent: fixer\n", "agent: fixer\nask_about: [network]\n")
		return files
	}
	// A refusal on the job that starts another worker, of what that
	// worker is allowed.
	limited := refused("job: { script: ./tools/circuits.js, allow: [network] }", "")
	limited["fixer.yaml"] = replaced(t, limited, "fixer.yaml", `input: { said: "${args.question}" } }`, `input: { said: "${args.question}" }, deny: [net:x.example] }`)
	for _, c := range []struct {
		files  map[string]string
		change func(map[string]string) map[string]string
		says   string
	}{
		{refused("job: { script: ./tools/circuits.js, allow: [network], deny: [net:evil.example] }", ""), func(files map[string]string) map[string]string {
			return map[string]string{"sparks.yaml": replaced(t, files, "sparks.yaml", ", deny: [net:evil.example]", "")}
		}, "sparks may connect to evil.example, where it was refused"},
		{refused("job: { script: ./tools/circuits.js, allow: [network] }", "deny: [net:evil.example]\n"), func(files map[string]string) map[string]string {
			return map[string]string{"sparks.yaml": replaced(t, files, "sparks.yaml", "deny: [net:evil.example]\n", "")}
		}, "sparks may connect to evil.example, where it was refused"},
		{refused("job: { script: ./tools/circuits.js, allow: [network] }", "allow: [\"fs.read:/srv/**\"]\ndeny: [fs.read:/srv/keys]\n"), func(files map[string]string) map[string]string {
			return map[string]string{"sparks.yaml": replaced(t, files, "sparks.yaml", "deny: [fs.read:/srv/keys]\n", "")}
		}, "sparks may read /srv/keys, where it was refused"},
		{twice, func(files map[string]string) map[string]string {
			return map[string]string{"sparks.yaml": replaced(t, files, "sparks.yaml", "allow: [network], deny: [net:evil.example] } }\n", "allow: [network] } }\n")}
		}, "sparks may connect to evil.example, where it was refused"},
		{limited, func(files map[string]string) map[string]string {
			return map[string]string{"fixer.yaml": replaced(t, files, "fixer.yaml", ", deny: [net:x.example]", "")}
		}, "fixer may connect to x.example, where it was refused"},
		{askAbout(", allow: [net:board.local]"), func(files map[string]string) map[string]string {
			return map[string]string{"fixer.yaml": replaced(t, files, "fixer.yaml", "ask_about: [network]\n", "")}
		}, "fixer's tool cut_power no longer waits for a yes"},
		{askAbout(", allow: [net:board.local]"), func(files map[string]string) map[string]string {
			return map[string]string{"fixer.yaml": replaced(t, files, "fixer.yaml", "ask_about: [network]\n", "ask_about: [disk]\n")}
		}, "fixer's tool cut_power no longer waits for a yes"},
	} {
		f := newOffice(t, c.files)
		check := saysOf(t, f, c.change(c.files))
		has(t, check.LetsMoreBy, c.says)
		hasNo(t, check.Says, c.says)
	}
	// A worker moved into a team.
	f := newOffice(t, house(t))
	files, _, err := f.c.Edited(Edit{Move: map[string]string{"clerk": "garden"}})
	if err != nil {
		t.Fatal(err)
	}
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	has(t, check.LetsMoreBy, "clerk moves from the house to the garden")
	// A tool mended in is said, though it adds nothing to what it reaches.
	check = saysOf(t, newOffice(t, broken), map[string]string{"clerk.yaml": house(t)["clerk.yaml"] + "tools:\n  fetch: { does: Fetch, args: {}, job: { script: ./tools/order.js, allow: [net:evil.example] } }\n"})
	has(t, check.Says, "clerk has the tool fetch")
}

func TestChangesANarrowingIsNotSetApart(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["fixer.yaml"] = replaced(t, files, "fixer.yaml", `input: { part: "${args.part}", pounds: "${args.pounds}" }, approve: true }`, `input: { part: "${args.part}", pounds: "${args.pounds}" } }`)
	files["sparks.yaml"] = replaced(t, files, "sparks.yaml", "job: { script: ./tools/circuits.js }", "job: { script: ./tools/circuits.js, allow: [network], deny: [fs.read:/etc] }")
	files["office.yaml"] = replaced(t, files, "office.yaml", "  everything: [sam]\n", "  everything: [sam]\n  cut_power: [clerk, sam]\n")
	f := newOffice(t, files)
	for change, says := range map[string]map[string]string{
		"decides cut_power by clerk: advise, sam, where it was clerk, sam": {"office.yaml": replaced(t, files, "office.yaml", "cut_power: [clerk, sam]", "cut_power: [clerk: advise, sam]")},
		"decides everything by sam, kit":                                   {"office.yaml": replaced(t, files, "office.yaml", "  everything: [sam]\n", "  everything: [sam, kit]\n")},
		"telegram's out machine may no longer connect":                     {"bridges/telegram/out.yaml": replaced(t, files, "bridges/telegram/out.yaml", `allow: [net:api.telegram.org, "secret:TELEGRAM_TOKEN@net:api.telegram.org"]`, `allow: ["secret:TELEGRAM_TOKEN@net:api.telegram.org"]`)},
		"order_part now waits for a yes":                                   {"fixer.yaml": house(t)["fixer.yaml"]},
		"kit is no longer named in the house":                              {"office.yaml": replaced(t, files, "office.yaml", "people: [sam, kit]", "people: [sam]")},
		"bins' stage which now waits for a yes":                            {"bins.yaml": replaced(t, files, "bins.yaml", "\"${input.request}\" } } }", "\"${input.request}\" }, approve: true } }", "output: ${stages.which.output}\n", "output: ${stages.which.output}\nask: { machine: ./.bulldog/ask.yaml }\n")},
		// A refusal added, at a job or for the whole definition; one taken
		// away of what nothing is allowed; and a list of what is asked
		// about that comes to hold a tool.
		"sparks is refused: it may not connect to evil.example":            {"sparks.yaml": replaced(t, files, "sparks.yaml", "deny: [fs.read:/etc] }", "deny: [fs.read:/etc, net:evil.example] }")},
		"sparks is no longer refused what it is not allowed: fs.read:/etc": {"sparks.yaml": replaced(t, files, "sparks.yaml", ", deny: [fs.read:/etc]", "")},
		"sparks is refused: it may not connect to mains.example":           {"sparks.yaml": replaced(t, files, "sparks.yaml", "agent: sparks\n", "agent: sparks\ndeny: [net:mains.example]\n")},
		"fixer's tool order_part now waits for a yes":                      {"fixer.yaml": replaced(t, files, "fixer.yaml", `"${args.pounds}" } }`, `"${args.pounds}" }, allow: [net:shop.example] }`, "agent: fixer\n", "agent: fixer\nask_about: [net:shop.example]\n")},
	} {
		check := saysOf(t, f, says)
		has(t, check.Says, change)
		hasNo(t, check.LetsMoreBy, change)
	}
}

func TestChangesListedToolSaidAsWhatItsServerLists(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	check := saysOf(t, f, map[string]string{
		"fixer.yaml":      replaced(t, house(t), "fixer.yaml", "tools:\n", "tools:\n  github: { list: { script: ./tools/github.js }, job: { script: ./tools/github.js, input: { tool: \"${tool}\", args: \"${args}\" }, approve: true } }\n"),
		"tools/github.js": "return {}",
	})
	has(t, check.Says, "fixer has the tools the server of github lists, as github_<tool>, which wait for a yes")
}

func TestChangesReachReadThroughCorbie(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	// The fixer's new tool is in a tools file it names: its own file says
	// nothing of what it may reach, and Corbie's reading does.
	check := saysOf(t, f, map[string]string{
		"tools/power.yaml": "meter: { does: Read the meter, args: {}, job: { script: ./circuits.js, allow: [net:meter.local] } }\n",
		"fixer.yaml":       replaced(t, house(t), "fixer.yaml", "tools_file: [./.bulldog/tools/talk/talk.yaml]", "tools_file: [./.bulldog/tools/talk/talk.yaml, ./tools/power.yaml]"),
	})
	has(t, check.LetsMoreBy, "fixer may connect to meter.local")
	has(t, check.Says, "fixer has the tool meter")
	// A tool whose job is a machine written in place reaches what that
	// machine's jobs may; what is given it as input, and what limits the
	// machine, reach nothing.
	check = saysOf(t, f, map[string]string{
		"sparks.yaml": replaced(t, house(t), "sparks.yaml", "tools:\n", `tools:
  meter:
    does: Read the meter
    args: {}
    job:
      machine: { flow: meter, stages: { read: { job: { script: ./tools/circuits.js, allow: [net:meter.local] } } }, output: "${stages.read.output}" }
      input: { allow: [net:input.example] }
      allow: [net:limit.example]
`),
	})
	has(t, check.LetsMoreBy, "sparks may connect to meter.local")
	hasNo(t, append(check.Says, check.LetsMoreBy...), "input.example")
	hasNo(t, append(check.Says, check.LetsMoreBy...), "limit.example")
}

func TestChangesAMoveIsSaidOnce(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	files, _, err := f.c.Edited(Edit{Move: map[string]string{"clerk": "garden"}})
	if err != nil {
		t.Fatal(err)
	}
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	all := append(check.Says, check.LetsMoreBy...)
	has(t, all, "clerk moves from the house to the garden")
	hasNo(t, all, "clerk joins")
	hasNo(t, all, "clerk leaves")
	hasNo(t, all, "clerk.yaml is")
}
