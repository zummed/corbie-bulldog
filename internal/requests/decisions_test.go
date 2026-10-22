package requests

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// orders is a ledger that orders a part of the pounds its prompt names,
// then says what it was told of the order.
func orders(turn testkit.Turn) testkit.Reply {
	if turn.Number == 1 {
		var pounds int
		fmt.Sscanf(turn.Prompt(), "Order a part for £%d.", &pounds)
		return testkit.Use("order_part", map[string]any{"pounds": pounds})
	}
	return testkit.Say("Told: " + turn.Last())
}

func TestDecideAsksInOrder(t *testing.T) {
	t.Parallel()
	w := newWorld(t, orders, func(files map[string]string) {
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "decide: [./small-orders.js, lee]", "decide: [./small-orders.js, ./second.js, lee]", 1)
		files["payments/second.js"] = `return { says: "pass", note: "I was given " + JSON.stringify(input.notes) }`
	})
	// Under £20, the first rule says yes, and nobody else is asked.
	if out := w.output(w.ask("payments", "Order a part for £12.", nil)); !strings.Contains(fmt.Sprint(out["answer"]), `"ordered":12`) {
		t.Errorf("the small order gave %v", out)
	}
	if n := len(w.runs("wait")); n != 0 {
		t.Errorf("the small order asked %d people", n)
	}
	// Over it, each rule passes with its note, the second is given the
	// first's, and Lee is shown both.
	run := w.ask("payments", "Order a part for £35.", nil)
	asked := w.waiting()
	input := asked.Input.(map[string]any)
	want := `May these run?

order_part: {"pounds":35}

./payments/small-orders.js passed: More than I order alone.
./payments/second.js passed: I was given [{"kind":"rule","name":"./payments/small-orders.js","note":"More than I order alone.","says":"pass"}]`
	if fmt.Sprint(input["to"]) != "map[people:[lee]]" || input["said"] != want || fmt.Sprint(input["choices"]) != "[yes no]" || input["decision"] != "decide" {
		t.Errorf("Lee is asked %#v", input)
	}
	// Lee's wait is given the same notes and the held job as data.
	notes, _ := json.Marshal(input["notes"])
	if want := `[{"kind":"rule","name":"./payments/small-orders.js","note":"More than I order alone.","says":"pass"},` +
		`{"kind":"rule","name":"./payments/second.js","note":"I was given [{\"kind\":\"rule\",\"name\":\"./payments/small-orders.js\",\"note\":\"More than I order alone.\",\"says\":\"pass\"}]","says":"pass"}]`; string(notes) != want {
		t.Errorf("Lee's wait is given the notes %s", notes)
	}
	if jobs, _ := json.Marshal(input["jobs"]); string(jobs) != `[{"args":{"pounds":35},"does":"Order a part","name":"order_part"}]` {
		t.Errorf("Lee's wait is given the jobs %s", jobs)
	}
	w.answer(asked, "yes", "lee")
	if out := w.output(run); out["outcome"] != "answered" || !strings.Contains(fmt.Sprint(out["answer"]), `"ordered":35`) {
		t.Errorf("the order Lee approved gave %v", out)
	}
}

func TestPersonRefuses(t *testing.T) {
	t.Parallel()
	w := newWorld(t, orders, nil)
	run := w.ask("payments", "Order a part for £35.", nil)
	w.answer(w.waiting(), "no", "lee")
	out := w.output(run)
	if answer := fmt.Sprint(out["answer"]); !strings.Contains(answer, "denied") || strings.Contains(answer, "ordered") {
		t.Errorf("the order Lee refused gave %v", out)
	}
}

func TestEveryDeciderPassingIsNo(t *testing.T) {
	t.Parallel()
	w := newWorld(t, orders, func(files map[string]string) {
		files["payments/team.yaml"] = `name: Payments
about: The ledger.
people: [lee]
lead: ledger
heed: [./passes.js]
decide: [./small-orders.js]
`
		files["payments/passes.js"] = `return { says: "pass", note: "Not mine to say." }`
	})
	// Every decider of the decide passed: the order is not approved.
	out := w.output(w.ask("payments", "Order a part for £35.", nil))
	if answer := fmt.Sprint(out["answer"]); !strings.Contains(answer, "denied") || !strings.Contains(answer, "Every decider passed.") {
		t.Errorf("the order gave %v", out)
	}
	// Every decider of the heed passed: the request is noted.
	if out := w.output(w.ask("payments", "Order a part for £35.", guest(true))); out["outcome"] != "noted" {
		t.Errorf("the heed that passed gave %v", out)
	}
}

// With no people in the office or the team, and no decide, nobody decides,
// and a held job is not approved.
func TestNobodyToDecide(t *testing.T) {
	t.Parallel()
	w := newWorld(t, orders, func(files map[string]string) {
		files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [dana]\n", "", 1)
		files["payments/team.yaml"] = "name: Payments\nabout: The ledger.\nlead: ledger\n"
	})
	if !strings.Contains(w.made["payments"].YAML, "statechart: payments-decide\n") {
		t.Fatalf("payments' machine has no decide:\n%s", w.made["payments"].YAML)
	}
	out := w.output(w.ask("payments", "Order a part for £12.", nil))
	if answer := fmt.Sprint(out["answer"]); !strings.Contains(answer, "denied") || !strings.Contains(answer, "Every decider passed.") {
		t.Errorf("the order gave %v", out)
	}
}

func TestFailedDecideApprovesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t, orders, func(files map[string]string) {
		files["payments/small-orders.js"] = `throw new Error("the rule broke")`
	})
	out := w.output(w.ask("payments", "Order a part for £12.", nil))
	if answer := fmt.Sprint(out["answer"]); out["outcome"] != "answered" || !strings.Contains(answer, "denied") || strings.Contains(answer, "ordered") {
		t.Errorf("the order gave %v", out)
	}
}

// TestRequestsWorkerNeverDecidesOwnJob: mate, a decider of payments, is
// asked about a job its own machine holds. Its yes is read as a pass, and
// Lee decides.
func TestRequestsWorkerNeverDecidesOwnJob(t *testing.T) {
	t.Parallel()
	o := newOffice(t, func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.HasPrefix(turn.Prompt(), "Decide"):
			return testkit.Say(`{"says": "yes", "note": "My own fetch is fine."}`)
		case turn.Number == 1:
			return testkit.Use("fetch", nil)
		}
		return testkit.Say(`{"says": "yes", "note": "Fetched."}`)
	}, func(files map[string]string) {
		files["payments/team.yaml"] = "name: Payments\nabout: The ledger.\npeople: [lee]\nlead: boss\ndecide: [mate, lee]\n"
		// The boss asks about every job below it that may reach the rates.
		files["payments/boss.yaml"] = `flow: boss
ask_about: ["net:rates.example"]
stages:
  get: { job: { machine: ./mate.yaml, input: { said: "${input.said}" } } }
output: ${stages.get.output.reply.note}
`
		files["payments/mate.yaml"] = `agent: mate
model: usual
prompt: "${input.jobs ? 'Decide on ' + JSON.stringify(input.jobs) : input.said}"
reply: { says: string, note: "string?" }
tools:
  fetch:
    does: Fetch the rates
    args: {}
    job: { script: ./fetch.js, allow: ["net:rates.example"] }
`
		files["payments/fetch.js"] = `return "rates"`
	})
	e, _ := o.run()
	run := o.chat(e, "payments", "Fetch the rates.")
	asked := o.waitFor("May these run?")
	if said := fmt.Sprint(asked.Input.(map[string]any)["said"]); !strings.HasSuffix(said, "\n\nmate passed: The held job is its own.") {
		t.Errorf("Lee is asked %q", said)
	}
	if err := e.Answer(ctx, asked.ID, "lee", "yes", ""); err != nil {
		t.Fatal(err)
	}
	if answer := o.answered(run); answer != "Fetched." {
		t.Errorf("the request answered %q", answer)
	}
	look, err := e.Look(ctx, run)
	if err != nil || len(look.Decisions) != 1 || fmt.Sprint(look.Decisions[0].Words) != "[{worker mate pass The held job is its own.} {person lee yes }]" {
		t.Errorf("the decisions are %+v (%v)", look, err)
	}
}

// asks is a ledger that asks platform what its prompt says, then says what
// it was told.
func asks(turn testkit.Turn) testkit.Reply {
	if turn.Number == 1 {
		return testkit.Use("ask_team", map[string]any{"team": "platform", "question": turn.Prompt()})
	}
	return testkit.Say("Told: " + turn.Last())
}

// TestRequestsQuestionToATeamPassesCheck: payments' check is asked about
// its question to platform. On yes the wait starts; on no it never does,
// and the ledger is told why.
func TestRequestsQuestionToATeamPassesCheck(t *testing.T) {
	t.Parallel()
	w := newWorld(t, asks, nil)
	run := w.ask("payments", "Is the charge service up?", nil)
	asked := w.waiting()
	if input := asked.Input.(map[string]any); fmt.Sprint(input["to"]) != "map[team:platform]" || input["said"] != "Is the charge service up?" {
		t.Errorf("platform is asked %v", input)
	}
	w.answer(asked, "It is up.", "platform")
	if out := w.output(run); out["outcome"] != "answered" || !strings.Contains(fmt.Sprint(out["answer"]), "It is up.") {
		t.Errorf("the request gave %v", out)
	}

	run = w.ask("payments", "Why was account 12345678 charged twice?", nil)
	out := w.output(run)
	if answer := fmt.Sprint(out["answer"]); !strings.Contains(answer, "denied") || !strings.Contains(answer, "Payments did not let its question out.") {
		t.Errorf("the request gave %v", out)
	}
	if waits := w.runs("wait"); len(waits) != 1 {
		t.Errorf("platform was asked %d times", len(waits))
	}
	if checks := w.runs("payments-question"); len(checks) != 2 {
		t.Errorf("payments' check was asked about %d questions", len(checks))
	}
}

// The office has no check, so its question leaves at once.
func TestQuestionLeavesATeamWithNoCheck(t *testing.T) {
	t.Parallel()
	w := newWorld(t, asks, nil)
	run := w.ask("office", "Was order 4411 refunded?", nil)
	asked := w.waiting()
	w.answer(asked, "Refunded £40 on 3 May.", "payments")
	if out := w.output(run); !strings.Contains(fmt.Sprint(out["answer"]), "Refunded £40 on 3 May.") {
		t.Errorf("the request gave %v", out)
	}
}

// TestOfficeWriteDocWaitsForDecide: the ledger's write_doc waits for
// payments' decide, which is shown the whole new text, and is written on
// Lee's yes.
func TestOfficeWriteDocWaitsForDecide(t *testing.T) {
	t.Parallel()
	w := newWorld(t, func(turn testkit.Turn) testkit.Reply {
		switch turn.Number {
		case 1:
			return testkit.Use("write_doc", map[string]any{"name": "refunds.md", "text": "Refunds take 3 days."})
		case 2:
			return testkit.Use("read_doc", map[string]any{"name": "refunds.md"})
		}
		return testkit.Say("Read: " + turn.Last())
	}, func(files map[string]string) {
		files["payments/ledger.yaml"] = strings.Replace(files["payments/ledger.yaml"], "talk/talk.yaml]", "talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]", 1)
	})
	run := w.ask("payments", "Write down how long refunds take.", nil)
	asked := w.waiting()
	if said := fmt.Sprint(asked.Input.(map[string]any)["said"]); !strings.Contains(said, `write_doc: {"name":"refunds.md","text":"Refunds take 3 days."}`) {
		t.Errorf("Lee is asked %q", said)
	}
	w.answer(asked, "yes", "lee")
	if out := w.output(run); !strings.Contains(fmt.Sprint(out["answer"]), "Refunds take 3 days.") {
		t.Errorf("the request gave %v", out)
	}
}

// TestOfficeDeciderNeverAsksApproval: a worker named as a decider that
// holds a job, or that declares no reply, is a problem of the team file
// that names it, and stops that team alone.
func TestOfficeDeciderNeverAsksApproval(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, func(files map[string]string) {
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "decide: [./small-orders.js, lee]", "decide: [careful, lee]\nheed: [asker]", 1)
		files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "heed: [./.bulldog/rules/mentioned/mentioned.js]\n", "", 1)
		files["payments/careful.yaml"] = `agent: careful
model: usual
prompt: Decide.
reply: { says: string, note: "string?" }
tools_file: [./.bulldog/tools/talk/talk.yaml]
`
		files["payments/asker.yaml"] = "flow: asker\nask_about: [network]\nstages:\n  look: { job: { script: ./look.js } }\noutput: ${stages.look.output}\n"
		files["payments/look.js"] = `return { says: "yes" }`
	})
	var texts []string
	for _, p := range w.office.Team("payments").Problems {
		if p.File != "payments/team.yaml" {
			t.Errorf("a problem of %s: %s", p.File, p.Text)
		}
		texts = append(texts, p.Text)
	}
	want := []string{
		"heed: asker holds a job for approval, or lists ask_about; a decider never asks for approval itself",
		`heed: asker does not declare reply: { says: string, note: "string?" }, which a worker named as a decider declares`,
		"decide: careful holds a job for approval, or lists ask_about; a decider never asks for approval itself",
	}
	if !slices.Equal(texts, want) {
		t.Errorf("payments' problems are:\n%s", strings.Join(texts, "\n"))
	}
	if w.made["payments"].Draft != nil {
		t.Error("payments' request machine is made")
	}
	if w.made["office"].Draft == nil || len(w.office.Team("office").Problems) > 0 {
		t.Errorf("the office is stopped: %v", office.Problems(w.office.Team("office").Problems))
	}
}

// A problem Corbie's parser finds in a worker is a problem of the worker's
// file, and stops the team whose request machine uses it.
func TestCorbieProblemsStopTheTeam(t *testing.T) {
	t.Parallel()
	w := newWorld(t, says, func(files map[string]string) {
		files["payments/ledger.yaml"] += "budget: 40\n"
	})
	problems := w.office.Team("payments").Problems
	if len(problems) == 0 || w.made["payments"].Draft != nil {
		t.Fatal("payments is not stopped")
	}
	for _, p := range problems {
		if p.File != "payments/ledger.yaml" || !strings.Contains(p.Text, "budget") {
			t.Errorf("a problem: %v", p)
		}
	}
	if w.made["office"].Draft == nil {
		t.Errorf("the office is stopped: %v", office.Problems(w.office.Team("office").Problems))
	}
}
