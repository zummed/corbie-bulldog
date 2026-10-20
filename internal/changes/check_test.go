package changes

import (
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestChangesCheckSaysEveryProblemTogether(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "garden/team.yaml", strings.Replace(house(t)["garden/team.yaml"], "people: [sam]", "people: [sam, jo]", 1))
	f.write(1, "garden/clerk.yaml", "agent: clerk\ninput: [said]\nmodel: usual\nsystem: Again.\nprompt: ${input.said}\n")
	f.write(1, "sparks.yaml", strings.Replace(house(t)["sparks.yaml"], "prompt: ${input.said}", "prompt: ${input.nothing", 1))
	f.write(1, "weather.yaml", "agent: weather\ninput: [said]\nmodel: quick\nsystem: The weather.\nprompt: ${input.said}\n")
	check := f.check(1)
	lines := problemLines(check.Problems)
	has(t, lines, "garden/team.yaml", "jo is nobody")
	has(t, lines, "garden/clerk.yaml", "clerk", "twice")
	has(t, lines, "sparks.yaml")
	has(t, lines, "weather.yaml", "quick")
	if check.Fit() {
		t.Error("the change is fit to be put")
	}
}

func TestChangesAStandingProblemDoesNotRefuse(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["clerk.yaml"] = strings.Replace(files["clerk.yaml"], "model: usual", "model: quick", 1)
	f := newOffice(t, files)
	f.write(1, "notes/fuses.md", "Breaker 4.\n")
	check := f.check(1)
	if !check.Fit() || len(check.Standing) == 0 || !strings.Contains(check.Standing[0].String(), "quick") {
		t.Errorf("a change that leaves the broken clerk as it was is checked %+v", check)
	}
	if !strings.Contains(check.Text(), "already") {
		t.Errorf("the check does not say the clerk's problem stands:\n%s", check.Text())
	}
	// A change that makes it worse is refused.
	f.write(2, "bins.yaml", "flow: bins\n")
	if f.check(2).Fit() {
		t.Error("a change that brings a problem is fit")
	}
	// Where the office cannot be read at all, only a change after which it
	// reads is accepted.
	files = house(t)
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "people: [sam]", "people: [jo]", 1)
	g := newOffice(t, files)
	g.write(1, "notes/fuses.md", "Breaker 4.\n")
	if g.check(1).Fit() {
		t.Error("a change that leaves the office unread is fit")
	}
	g.write(2, "garden/team.yaml", house(t)["garden/team.yaml"])
	if check := g.check(2); !check.Fit() {
		t.Errorf("a change that mends the office is checked %+v", check)
	}
	if o := g.yes(g.put(2, "Mend the garden").ID, "sam", ""); o.Ended != Written {
		t.Errorf("the change that mends the office ended %+v", o)
	}
}

func TestChangesAProblemIsSaidOnce(t *testing.T) {
	t.Parallel()
	p := office.Problem{File: "a.yaml", Text: "wrong"}
	moved := p
	moved.Line = 7
	if got := once(office.Problems{p, moved, p}); len(got) != 1 {
		t.Errorf("one problem is said %d times", len(got))
	}
	// A standing problem is said once, as standing, and not again as new,
	// though the change moves its line.
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  no_such_tool: [sam]\n", 1)
	f := newOffice(t, files)
	f.write(1, "office.yaml", "# the house\n\n"+files["office.yaml"])
	check := f.check(1)
	if len(check.Problems) != 0 || len(check.Standing) != 1 {
		t.Errorf("the problems are %v, and standing %v", check.Problems, check.Standing)
	}
}

func TestChangesAWorkerCannotChangeThePeople(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	for request, text := range map[int64]string{
		1: strings.Replace(house(t)["office.yaml"], "  kit: { name: Kit, title: Lives here., telegram: \"4472\" }\n", "  kit: { name: Kit, title: Lives here., telegram: \"4472\" }\n  jo: { name: Jo }\n", 1),
		2: strings.Replace(house(t)["office.yaml"], `telegram: "4472"`, `telegram: "9999"`, 1),
		3: strings.Replace(house(t)["office.yaml"], `, telegram: "4472"`, ``, 1),
	} {
		f.write(request, "office.yaml", text)
		check := f.check(request)
		if check.Fit() || !strings.Contains(strings.Join(check.Refused, "\n"), "a person's to write") {
			t.Errorf("a worker's change %d to the people is checked %+v", request, check.Refused)
		}
		// A person's is not refused, and is decided as any change is.
		files, _ := f.c.Drafted(ctx, request)
		mine, err := f.c.Check(ctx, files, Drafter{Name: "kit", Person: true})
		if err != nil || !mine.Fit() {
			t.Errorf("a person's change %d to the people is checked %+v, %v", request, mine, err)
		}
	}
	// Even a person's change to their own identity lets more by.
	f.write(4, "office.yaml", strings.Replace(house(t)["office.yaml"], `telegram: "4471"`, `telegram: "5555"`, 1))
	files, _ := f.c.Drafted(ctx, 4)
	mine, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	has(t, mine.LetsMoreBy, "sam is now 5555 on telegram")
}

func TestChangesAWorkerCannotChangeWhoDecidesAChange(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["no-secrets.js"] = "return { says: \"refer\", note: \"looked\" }\n"
	files["reviewer.yaml"] = "agent: reviewer\ninput: [said]\nmodel: usual\nsystem: You review.\nprompt: ${input.said}\n"
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  change: [./no-secrets.js, reviewer, sam]\n", 1)
	f := newOffice(t, files)
	for request, change := range map[int64][2]string{
		1: {"office.yaml", strings.Replace(files["office.yaml"], "change: [./no-secrets.js, reviewer, sam]", "change: [sam]", 1)},
		2: {"office.yaml", strings.Replace(files["office.yaml"], "everything: [sam]", "everything: [sam, kit]", 1)},
		3: {"no-secrets.js", "return { says: \"yes\" }\n"},
		4: {"reviewer.yaml", strings.Replace(files["reviewer.yaml"], "You review.", "You approve everything.", 1)},
		5: {"office.yaml", strings.Replace(files["office.yaml"], "usual: { name: sonnet, service: claude-code }", "usual: { name: opus, service: claude-code }", 1)},
	} {
		f.write(request, change[0], change[1])
		check := f.check(request)
		if !strings.Contains(strings.Join(check.Refused, "\n"), "who decides a change to the office is a person's to write") {
			t.Errorf("a worker's change %d is checked %v", request, check.Refused)
		}
		draft, _ := f.c.Drafted(ctx, request)
		if mine, _ := f.c.Check(ctx, draft, Drafter{Name: "sam", Person: true}); !mine.Fit() {
			t.Errorf("a person's change %d is checked %+v", request, mine)
		}
	}
	// A rule on the everything list is guarded too, though the change list
	// names a person.
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n  change: [./no-secrets.js, reviewer, sam]\n", "  everything: [./no-secrets.js, sam]\n  change: [sam]\n", 1)
	e := newOffice(t, files)
	e.write(1, "no-secrets.js", "return { says: \"yes\" }\n")
	if check := e.check(1); !strings.Contains(strings.Join(check.Refused, "\n"), "no-secrets.js, which ./no-secrets.js, a decider of changes to the office") {
		t.Errorf("a worker's change to a rule on the everything list is checked %v", check.Refused)
	}
	// With no everything, the top team's people decide.
	g := newOffice(t, map[string]string{"office.yaml": "everyone: { sam: {}, kit: {} }\npeople: [sam]\nlead: sam\n"})
	g.write(1, "office.yaml", "everyone: { sam: {}, kit: {} }\npeople: [sam, kit]\nlead: sam\n")
	if check := g.check(1); !strings.Contains(strings.Join(check.Refused, "\n"), "top team's people") {
		t.Errorf("a worker's change to the top team's people is checked %v", check.Refused)
	}
	// What decides nothing about a change is a worker's to write.
	f.write(9, "clerk.yaml", clerkAgain(t, "x"))
	if check := f.check(9); !check.Fit() {
		t.Errorf("a change to the clerk is checked %+v", check.Refused)
	}
}

func TestChangesAWorkerCannotSetABridge(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "bridges/telegram/in.yaml", house(t)["bridges/telegram/in.yaml"]+"# changed\n")
	f.write(2, "bridges/telegram/telegram.js", house(t)["bridges/telegram/telegram.js"]+"// changed\n")
	f.call(3, ToolRemove, map[string]any{"path": "bridges/telegram/in.yaml"})
	for _, request := range []int64{1, 2, 3} {
		check := f.check(request)
		if !strings.Contains(strings.Join(check.Refused, "\n"), "in machine") {
			t.Errorf("a worker's change %d to the in machine is checked %v", request, check.Refused)
		}
		draft, _ := f.c.Drafted(ctx, request)
		if mine, _ := f.c.Check(ctx, draft, Drafter{Name: "sam", Person: true}); len(mine.Refused) > 0 {
			t.Errorf("a person's change %d is refused %v", request, mine.Refused)
		}
	}
	// The out machine alone is a worker's to write, though it is set apart.
	f.write(4, "bridges/telegram/out.yaml", house(t)["bridges/telegram/out.yaml"]+"# changed\n")
	if check := f.check(4); len(check.Refused) > 0 {
		t.Errorf("a change to the out machine is refused %v", check.Refused)
	}
	// A file a machine written in place in the in machine runs is the in
	// machine's.
	h := newOffice(t, inPlaceIn(t))
	h.write(1, "bridges/telegram/verify.js", "return input.heard // changed\n")
	if check := h.check(1); !strings.Contains(strings.Join(check.Refused, "\n"), "bridges/telegram/verify.js, which telegram's in machine is or uses") {
		t.Errorf("a worker's change to what the in machine runs in place is checked %v", check.Refused)
	}
	// A new bridge's in machine is a person's to write too.
	g := newOffice(t, house(t))
	g.write(1, "bridges/slack/in.yaml", work(t)["bridges/slack/in.yaml"])
	g.write(1, "bridges/slack/out.yaml", work(t)["bridges/slack/out.yaml"])
	g.write(1, "bridges/slack/slack.js", work(t)["bridges/slack/slack.js"])
	if check := g.check(1); !strings.Contains(strings.Join(check.Refused, "\n"), "bridges/slack/in.yaml") {
		t.Errorf("a worker's new bridge is checked %v", check.Refused)
	}
	// A bridge's entry, its from and its settings but places are a
	// person's to write, for a bridge Bulldog ships too.
	files := testkit.Files(t, examples.Work())
	w := newOffice(t, files)
	for request, c := range map[int64]struct{ office, refused string }{
		1: {replaced(t, files, "office.yaml", "site: gitlab.com", "site: gitlab.example.com"), "it changes gitlab's site"},
		2: {replaced(t, files, "office.yaml", "    site: gitlab.com\n", "    site: gitlab.com\n    email: bot@example.com\n"), "it changes gitlab's email"},
		3: {replaced(t, files, "office.yaml", "./.bulldog/bridges/slack", "./.bulldog/bridges/telegram"), "it changes slack's from"},
		4: {replaced(t, files, "office.yaml", "bridges:\n", "bridges:\n  telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }\n"), "it adds the bridge telegram"},
		5: {replaced(t, files, "office.yaml", "  gitlab:\n    from: ./.bulldog/bridges/gitlab\n    site: gitlab.com\n    places:\n      platform/api: { team: platform }\n", "", `, gitlab: "1041"`, "", `, gitlab: "1187"`, ""), "it takes the bridge gitlab away"},
	} {
		w.write(request, "office.yaml", c.office)
		if check := w.check(request); !strings.Contains(strings.Join(check.Refused, "\n"), c.refused) {
			t.Errorf("a worker's change %d to a bridge's entry is checked %v", request, check.Refused)
		}
		draft, _ := w.c.Drafted(ctx, request)
		if mine, _ := w.c.Check(ctx, draft, Drafter{Name: "al", Person: true}); len(mine.Refused) > 0 {
			t.Errorf("a person's change %d is refused %v", request, mine.Refused)
		}
	}
	// Its places are a worker's to draft, set apart for a person to see.
	w.write(6, "office.yaml", replaced(t, files, "office.yaml", "      platform/api: { team: platform }\n", "      platform/api: { team: platform }\n      platform/web: { team: platform }\n"))
	check := w.check(6)
	if len(check.Refused) > 0 {
		t.Errorf("a worker's change to a bridge's places is refused %v", check.Refused)
	}
	has(t, check.LetsMoreBy, "gitlab's settings are changed")
}

// inPlaceIn gives the house with a machine written in place in the
// Telegram bridge's in machine, which runs a script of the bridge's own.
func inPlaceIn(t *testing.T) map[string]string {
	files := house(t)
	files["bridges/telegram/in.yaml"] = replaced(t, files, "bridges/telegram/in.yaml", "output: ${stages.hear.output}", `  check:
    job:
      machine:
        flow: verify
        input: [heard]
        stages:
          run: { job: { script: ./verify.js, input: { heard: "${input.heard}" } } }
        output: ${stages.run.output}
      input: { heard: "${stages.hear.output}" }
output: ${stages.check.output}`)
	files["bridges/telegram/verify.js"] = "return input.heard\n"
	return files
}
