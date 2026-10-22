package requests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
)

// says gives each turn of the ledger what it is asked about: an account
// number when asked about the account, nothing when asked for nothing,
// and the refund otherwise.
func says(turn testkit.Turn) testkit.Reply {
	switch prompt := turn.Prompt(); {
	case strings.Contains(prompt, "account"):
		return testkit.Say("It is 12345678.")
	case strings.Contains(prompt, "nothing"):
		return testkit.Say("")
	}
	return testkit.Say("Refunded £40 on 3 May.")
}

// guest is a guest's request in #refunds on Slack, naming the bot or not.
func guest(mentioned bool) map[string]any {
	return map[string]any{"by": map[string]any{"guest": "U0GUEST0"}, "heard": "channel", "mentioned": mentioned,
		"place": map[string]any{"bridge": "slack", "place": "#refunds", "guests": true, "where": "C01/1700000000.0001"}}
}

func TestRequestAnswered(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, nil)
	out := w.output(w.ask("payments", "@bot was order 4411 refunded?", guest(true)))
	if out["outcome"] != "answered" || out["answer"] != "Refunded £40 on 3 May." {
		t.Errorf("the request gave %v", out)
	}
	if n := len(w.model.Turns()); n != 1 {
		t.Errorf("the ledger took %d turns", n)
	}
}

func TestRequestNotedWhenHeedSaysNo(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, nil)
	out := w.output(w.ask("payments", "lunch?", guest(false)))
	if out["outcome"] != "noted" || out["answer"] != nil {
		t.Errorf("the request gave %v", out)
	}
	if n := len(w.model.Turns()); n != 0 {
		t.Errorf("the ledger worked on a noted request, %d turns", n)
	}
}

func TestHeedOnlyAtAChannel(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, nil)
	// None of these names the bot, and only a channel has a heed.
	for _, heard := range []map[string]any{
		{"heard": "direct", "by": map[string]any{"person": "dana"}},
		{"heard": "team", "by": map[string]any{"team": "office"}, "chain": []any{"office"}},
		{"heard": "chat"},
	} {
		out := w.output(w.ask("payments", "Was order 4411 refunded?", heard))
		if out["outcome"] != "answered" || out["answer"] != "Refunded £40 on 3 May." {
			t.Errorf("a request heard %v gave %v", heard["heard"], out)
		}
	}
}

// The shipped for-us is given the request and what was said. It says yes
// to a message that names the bot without asking its model.
func TestShippedHeedRule(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, func(files map[string]string) {
		files["office.yaml"] = strings.Replace(files["office.yaml"], "lead: bot", "lead: bot\nheed: [./.bulldog/rules/for-us/for-us.yaml]", 1)
	})
	out := w.output(w.ask("office", "@bot was order 4411 refunded?", guest(true)))
	if out["outcome"] != "answered" || out["answer"] != "Refunded £40 on 3 May." {
		t.Errorf("the request gave %v", out)
	}
}

// TestRequestsOnlyOutputLeaves: the check withholds the lead's answer, and
// the output, the one thing that leaves, holds only the fixed line.
func TestRequestsOnlyOutputLeaves(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, nil)
	run := w.ask("payments", "@bot what is the account of order 4411?", guest(true))
	out := w.output(run)
	if out["outcome"] != "withheld" || out["answer"] != "Payments did not let its answer out." {
		t.Errorf("the request gave %v", out)
	}
	if strings.Contains(fmt.Sprint(out), "12345678") {
		t.Errorf("the output holds what the team read: %v", out)
	}
}

func TestCheckSkippedForTheTeamsOwn(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, nil)
	for _, asker := range []map[string]any{
		{"by": map[string]any{"person": "lee"}},
		{"by": map[string]any{"routine": "mon 09:00"}, "heard": "routine"},
	} {
		out := w.output(w.ask("payments", "What is the account of order 4411?", asker))
		if out["outcome"] != "answered" || out["answer"] != "It is 12345678." {
			t.Errorf("a request by %v gave %v", asker["by"], out)
		}
	}
	// At a channel, people outside the team read the answer, and it is
	// checked whoever asked.
	lee := guest(true)
	lee["by"] = map[string]any{"person": "lee"}
	if out := w.output(w.ask("payments", "@bot what is the account of order 4411?", lee)); out["outcome"] != "withheld" {
		t.Errorf("Lee's request at a channel gave %v", out)
	}
	// An empty answer is not checked, and says nothing.
	out := w.output(w.ask("payments", "Say nothing.", nil))
	if out["outcome"] != "answered" || out["answer"] != "" {
		t.Errorf("an empty answer gave %v", out)
	}
}

// TestRequestsErrorNeverSentToAsker: a lead, a heed rule and a check rule
// that fail each fail the request with the fixed line. The error stays in
// the run's record, for the team's people.
func TestRequestsErrorNeverSentToAsker(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, func(files map[string]string) {
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "lead: ledger", "lead: looker", 1)
		files["payments/looker.yaml"] = "flow: looker\nstages:\n  look: { job: { script: ./look.js, input: { said: \"${input.said}\" } } }\noutput: ${stages.look.output}\n"
		files["payments/look.js"] = `if (input.said.includes("fail")) throw new Error("the ledger is down, card 4111 waits")
return { answer: "Refunded £40 on 3 May." }`
		files["office.yaml"] = strings.Replace(files["office.yaml"], "lead: bot", "lead: bot\nheed: [./throws.js]\n", 1)
		files["throws.js"] = `throw new Error("the heed rule broke")`
		files["platform/team.yaml"] = "name: Platform\nabout: The servers.\nlead: ops\ncheck: [./throws.js]\n"
		files["platform/ops.yaml"] = "flow: ops\nstages:\n  up: { job: { script: ./up.js } }\noutput: ${stages.up.output}\n"
		files["platform/up.js"] = `return "The charge service is up."`
		files["platform/throws.js"] = `throw new Error("the check rule broke")`
	})
	for _, c := range []struct{ team, said, error string }{
		{"payments", "Please fail.", "the ledger is down, card 4111 waits"},
		{"office", "Anyone there?", "the heed rule broke"},
		{"platform", "Is the charge service up?", "the check rule broke"},
	} {
		given := map[string]any{"by": map[string]any{"person": "lee"}}
		if c.team == "office" {
			given = guest(true)
		}
		run := w.ask(c.team, c.said, given)
		out := w.output(run)
		team := w.office.Team(c.team).Name
		if out["outcome"] != "failed" || out["answer"] != team+" could not answer: its work failed." {
			t.Errorf("%s's request gave %v", c.team, out)
		}
		if strings.Contains(fmt.Sprint(out), c.error) {
			t.Errorf("%s's output holds its error: %v", c.team, out)
		}
		if !w.recorded(run, c.error) {
			t.Errorf("%s's record does not keep the error %q", c.team, c.error)
		}
	}
}

func TestTheLeadsAnswer(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, func(files map[string]string) {
		files["garden/team.yaml"] = "name: The garden\nabout: The beds.\npeople: [al]\nlead: al\ncheck: [./never.js]\n"
		files["garden/never.js"] = `return { says: "no" }`
		files["platform/team.yaml"] = "name: Platform\nabout: The servers.\nlead: ops\n"
		files["platform/ops.yaml"] = "flow: ops\nstages:\n  count: { job: { script: ./count.js } }\noutput: ${stages.count.output}\n"
		files["platform/count.js"] = `return { answer: "Done.", rows: 3 }`
	})
	// A person who leads is asked by wait.yaml what was said and the
	// context, and their answer is the team's.
	run := w.ask("garden", "When do we sow the beds?", map[string]any{"by": map[string]any{"person": "al"}})
	asked := w.waiting()
	input := asked.Input.(map[string]any)
	if fmt.Sprint(input["to"]) != "map[people:[al]]" || input["said"] != "When do we sow the beds?\n\nDana asks garden in the chat." {
		t.Errorf("the garden's lead is asked %v", input)
	}
	w.answer(asked, "In May.", "al")
	if out := w.output(run); out["outcome"] != "answered" || out["answer"] != "In May." {
		t.Errorf("the garden's request gave %v", out)
	}
	// A flow's text answer is its team's answer.
	if out := w.output(w.ask("platform", "Tidy the rows.", nil)); out["outcome"] != "answered" || out["answer"] != "Done." {
		t.Errorf("platform's request gave %v", out)
	}
}

// TestOfficeTeamRunsKeepToOwnDocs: a team's run keeps a value under its
// own path, and is refused another team's, though its definition allows
// every kept path.
func TestOfficeTeamRunsKeepToOwnDocs(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, func(files map[string]string) {
		files["payments/team.yaml"] = "name: Payments\nabout: The ledger.\npeople: [lee]\nlead: keeper\n"
		files["payments/keeper.yaml"] = `flow: keeper
stages:
  own: { job: { script: ./keep.js, input: { path: "${input.request.docs}/refunds.md" }, allow: ["kept.read:**", "kept.write:**"] } }
  other: { job: { script: ./keep.js, input: { path: "${input.request.docs.replace('/payments/', '/office/')}/refunds.md" }, allow: ["kept.read:**", "kept.write:**"] } }
output: { answer: "${stages.own.output + ' ' + (stages.other.failed ? 'refused' : stages.other.output)}" }
`
		files["payments/keep.js"] = `kept.write(input.path, { text: "Refunds take 3 days." })
return kept.read(input.path)`
	})
	out := w.output(w.ask("payments", "Keep the refunds doc.", nil))
	if out["answer"] != "Refunds take 3 days. refused" {
		t.Errorf("the request gave %v", out)
	}
}

// recorded says whether a text is in the record of a run or of a run
// below it.
func (w *world) recorded(run, text string) bool {
	w.t.Helper()
	found := false
	_, err := w.corbie.Record(ctx, run, func(e client.Entry) {
		if strings.Contains(fmt.Sprint(e.Events, e.Data), text) {
			found = true
		}
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return found
}
