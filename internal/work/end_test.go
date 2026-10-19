package work

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// desk is a desk team whose lead is a flow running one script, given
// what is said.
func desk(files map[string]string, script string) map[string]string {
	files["desk/team.yaml"] = "name: The desk\nabout: Papers.\npeople: [sam, kit]\nlead: filer\n"
	files["desk/filer.yaml"] = "flow: filer\ninput: [said]\nstages:\n  a: { job: { script: ./filer.js, input: { said: \"${input.said}\" } } }\noutput: ${stages.a.output}\n"
	files["desk/filer.js"] = script
	return files
}

// limited gives the steward a limit of turns.
func limited(files map[string]string, turns int) map[string]string {
	files["steward.yaml"] = strings.Replace(files["steward.yaml"], "model: usual", fmt.Sprintf("model: usual\nlimits: { turns: %d }", turns), 1)
	return files
}

func TestARunsAnswerIsItsOutput(t *testing.T) {
	t.Parallel()
	b := newBench(t, desk(house(t), `if (input.said === "text") return "done"
if (input.said === "answer") return { answer: "done", spare: 1 }
return { n: 3 }
`))
	for said, want := range map[string]string{"text": "done", "answer": "done", "data": "{\n  \"n\": 3\n}"} {
		n := b.chat("sam", "desk", said)
		b.settled(n, requests.Answered)
		answers := b.entries(n, requests.KindAnswer)
		if len(answers) != 1 || answers[0].Text != want || answers[0].By.Name != "filer" {
			t.Errorf("%s is answered %v", said, b.thread(n))
		}
	}
}

func TestTheAnswerGoesToTheAsker(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("The socket is mended.") })
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	answers := b.entries(n, requests.KindAnswer)
	if len(answers) != 1 || answers[0].Text != "The socket is mended." || answers[0].By.Name != "steward" ||
		len(answers[0].To) != 1 || answers[0].To[0].Name != "kit" {
		t.Fatalf("the thread is %v", b.thread(n))
	}
	if ds, err := b.reqs.Deliveries(answers[0].ID); err != nil || len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Kind != requests.InChat {
		t.Errorf("the answer is carried to %v (%v)", ds, err)
	}

	r, err := b.reqs.AskByRoutine("office", b.office().TopTeam().Routines[0], requests.Who{})
	if err != nil {
		t.Fatal(err)
	}
	b.settled(r.Number, requests.Answered)
	answers = b.entries(r.Number, requests.KindAnswer)
	ds, err := b.reqs.Deliveries(answers[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var people []string
	for _, d := range ds {
		people = append(people, d.Person)
	}
	if slices.Sort(people); !slices.Equal(people, []string{"kit", "sam"}) {
		t.Errorf("the routine's answer goes to %v", people)
	}
}

func TestAnEmptyAnswerIsTheTeamGivingNone(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("") })
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 1 || answers[0].Text != "The house gave no answer." {
		t.Errorf("the thread is %v", b.thread(n))
	}
	// In a direct message, Kit is told so too.
	direct := must(b.reqs.AskAt("telegram", office.Direct, "4472", "", "4472", "the boiler is cold", nil)).Number
	b.settled(direct, requests.Answered)
	if answers := b.entries(direct, requests.KindAnswer); len(answers) != 1 || answers[0].Text != "The house gave no answer." {
		t.Errorf("the direct message's thread is %v", b.thread(direct))
	}
	// The bins flow says nothing on a Monday night: its routine's request
	// is answered, and nothing after its asking is said to Sam or Kit.
	b.waitsForItsTime(whichBin)
	b.clock.Set(tonight)
	bins := b.opened(whichBin, 1)
	b.settled(bins, requests.Answered)
	if thread := must(b.reqs.Thread(bins)); len(thread) != 1 || thread[0].Kind != requests.KindAsked {
		t.Errorf("the bins' thread is %v", b.thread(bins))
	}
	given := b.input(b.runs(bins)[0].Name)
	told := given["request"].(map[string]any)["answer_goes_to"].(string)
	holdsNone(t, "what the bins flow is told", told, "no answer")
}

func TestAnEmptyAnswerAtAChannelPlaceSaysNothing(t *testing.T) {
	t.Parallel()
	files := payments(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`,
		`"#help": { guests: true }`+"\n      \"#refunds\": { team: payments, guests: true }", 1)
	b := newBench(t, files)
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("") })
	// Dana says thanks in #help, and the bot replies with nothing: the
	// request is answered, and nothing is said there.
	help := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "thanks @bot", nil)).Number
	holds(t, "the bot's context", b.prompt("thanks @bot"), "Your last reply is your answer, said to Dana on slack, in #help/1700.1. An empty reply says nothing.")
	b.settled(help, requests.Answered)
	if thread := must(b.reqs.Thread(help)); len(thread) != 1 || thread[0].Kind != requests.KindAsked {
		t.Errorf("the thread in #help is %v", b.thread(help))
	}
	// A guest in #refunds is outside payments, yet the ledger's empty
	// answer is asked of no disclosure.
	refunds := must(b.reqs.AskAt("slack", "#refunds", "#refunds/1700.2", "", "U0GUEST1", "ok", nil)).Number
	b.settled(refunds, requests.Answered)
	if thread := must(b.reqs.Thread(refunds)); len(thread) != 1 {
		t.Errorf("the thread in #refunds is %v", b.thread(refunds))
	}
	if decs := must(b.d.OfRequest(refunds)); len(decs) != 0 {
		t.Errorf("the empty answer was asked of %+v", decs)
	}
}

// payments is the work office with a disclosure check that is a bare
// script: it lets go what names no card, and passes the rest to Lee or
// Priya.
func payments(t *testing.T) map[string]string {
	files := work(t)
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "disclosure: [./no-account-numbers.yaml]", "disclosure: [./no-cards.js]", 1)
	files["payments/no-cards.js"] = `if (String(input.text).includes("4111")) return { says: "pass", note: "It names a card." }
return { says: "yes", note: "No card." }
`
	return files
}

func TestAnAnswerInsideATeamGoesToNoDisclosureCheck(t *testing.T) {
	t.Parallel()
	const check = "Morning check of the ledger"
	files := payments(t)
	files["payments/team.yaml"] += "routines:\n  - { at: \"07:00\", to: ledger, say: \"" + check + "\" }\n"
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		return testkit.Say("Duplicate confirmed on card 4111 1111 1111 1111.")
	})
	// Lee, of payments, asks it, and its routine asks it for its people:
	// neither answer leaves payments, so neither waits on its disclosure.
	b.waitsForItsTime(check)
	lee := b.chat("lee", "payments", "was 4411 charged twice?")
	b.clock.Set(time.Date(2026, 10, 6, 6, 0, 0, 0, time.UTC))
	routine := b.opened(check, 1)
	for _, n := range []int64{lee, routine} {
		holds(t, "the ledger's answer", b.answered(n).Text, "card 4111")
		if decs := must(b.d.OfRequest(n)); len(decs) != 0 {
			t.Errorf("request %d made the decisions %+v", n, decs)
		}
	}
}

func TestAnAnswerLeavingATeamWithADisclosureCheckWaitsForIt(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "the last four digits only"):
			return testkit.Say("Duplicate confirmed on the card ending 1111; refunded.")
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			return testkit.Say("Duplicate confirmed on card 4111 1111 1111 1111.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("al", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	// The rule passes it to Lee and Priya; nothing leaves payments, and
	// the bot's tool still waits.
	dec := b.disclosing(sub, decisions.SaysAnswer)
	if answers := b.entries(sub, requests.KindAnswer); len(answers) != 0 {
		t.Fatalf("the answer left payments before it was disclosed: %v", b.thread(sub))
	}
	b.still()
	if calls := b.calls(n); len(calls) != 1 || calls[0].HasEnded() {
		t.Fatalf("the bot's tool does not wait: %+v", calls)
	}
	if r, err := b.reqs.Get(n); err != nil || r.Answered {
		t.Fatalf("the bot's request was answered meanwhile: %v", b.thread(n))
	}
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("lee"), "refuse", "the last four digits only"); err != nil || !r.Taken {
		t.Fatalf("Lee's refusal: %v %v", r, err)
	}
	// The ledger is started again with Lee's word, and what it says next
	// is let go by the rule, and is the bot's tool result.
	run := b.started(sub, 2)
	holds(t, "the ledger's second run's said", fmt.Sprint(b.input(run.Name)["said"]), "the last four digits only")
	if a := b.answered(n); !strings.HasPrefix(a.Text, "bot heard: ") || !strings.Contains(a.Text, "ending 1111") {
		t.Errorf("the bot was given %q", a.Text)
	}
	holdsNone(t, "the bot's request", strings.Join(b.thread(n), "\n"), "4111", "the last four digits only")
}

func TestARunThatFailsSaysSoTwice(t *testing.T) {
	t.Parallel()
	b := newBench(t, desk(house(t), `return env.get("HOME")`))
	n := b.chat("kit", "desk", "file the bill")
	b.settled(n, requests.Answered)
	var team, asker *requests.Entry
	for _, e := range b.entries(n, requests.KindBulldog) {
		switch {
		case e.Read == "desk":
			team = &e
		case len(e.To) == 1 && e.To[0].Name == "kit":
			asker = &e
		}
	}
	if team == nil || asker == nil {
		t.Fatalf("the thread is %v", b.thread(n))
	}
	holds(t, "the desk's entry", team.Text, "refused", "env", "layer", "To let it:")
	if asker.Text != "The desk could not answer." {
		t.Errorf("Kit is told %q", asker.Text)
	}
	if v, err := b.reqs.View(n, "kit"); err != nil || !slices.ContainsFunc(v.Entries, func(e requests.Entry) bool { return e.ID == team.ID }) {
		t.Errorf("Kit, of the desk, does not see what failed: %v", err)
	}
	b.say(n, "kit", "try again")
	b.started(n, 2)
	b.settled(n, requests.Answered)
}

func TestAFailureWithASubRequestOpenLeavesTheRequestOpen(t *testing.T) {
	t.Parallel()
	b := newBench(t, limited(house(t), 1))
	held, let := b.hold("socket", testkit.Use("clerk", map[string]any{"question": "how much is a socket?"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.until("the steward is held", func() bool { return held() == 1 })
	sub, err := b.reqs.Sub(n, b.reqs.Worker("steward"), "garden", "is the shed locked?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	_ = sub
	// The garden's lead is held too, so the sub-request stays open.
	shed := make(chan struct{})
	defer close(shed)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "is the shed locked?") {
			<-shed
		}
		return testkit.Use("clerk", map[string]any{"question": "how much is a socket?"})
	})
	let()
	b.until("the steward's run has failed", func() bool {
		runs := b.runs(n)
		return len(runs) == 1 && runs[0].Status == failed
	})
	b.until("the failure is written", func() bool { return len(b.entries(n, requests.KindBulldog)) == 2 })
	if r, err := b.reqs.Get(n); err != nil || r.Answered {
		t.Errorf("the request was answered with its sub-request open: %v", b.thread(n))
	}
}

func TestWorkAskerNotToldTheError(t *testing.T) {
	t.Parallel()
	b := newBench(t, desk(house(t), `throw new Error("card 4111 1111 1111 1111 was declined")`))
	n := b.chat("kit", "desk", "pay the bill")
	b.settled(n, requests.Answered)
	for _, e := range b.entries(n, requests.KindBulldog) {
		switch {
		case e.Read == "desk":
			holds(t, "what the desk is told", e.Text, "4111")
		default:
			holdsNone(t, "what Kit is told", e.Text, "4111", "declined")
		}
	}
	v, err := b.reqs.View(n, "kit")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range v.Entries {
		if e.Read == "" {
			holdsNone(t, "what was done", e.Text, "4111")
		}
	}
}

func TestWorkFailedRunNotTriedAgain(t *testing.T) {
	t.Parallel()
	b := newBench(t, desk(house(t), `return env.get("HOME")`))
	n := b.chat("kit", "desk", "file the bill")
	b.settled(n, requests.Answered)
	b.later(time.Hour)
	b.w.Poke()
	b.still()
	b.settled(n, requests.Answered)
	if runs := b.runs(n); len(runs) != 1 {
		t.Errorf("the failed run was tried again: %+v", runs)
	}
}

func TestEachRunIsKeptWithItsRequest(t *testing.T) {
	t.Parallel()
	b := newBench(t, limited(house(t), 2))
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "again") {
			return testkit.Use("clerk", map[string]any{"question": "what is owed?"})
		}
		return testkit.Say("mended")
	})
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	b.later(time.Minute)
	b.say(n, "kit", "it sparked again")
	b.started(n, 2)
	b.settled(n, requests.Answered)
	runs := b.runs(n)
	if len(runs) != 2 || runs[0].Status != done || runs[1].Status != failed || !strings.Contains(runs[1].How, "turn limit") {
		t.Fatalf("the runs are %+v", runs)
	}
	for _, r := range runs {
		if r.Worker != "steward" || r.Started.IsZero() || r.Ended.IsZero() || r.Tokens <= 0 || r.Started.Location().String() != "Europe/London" {
			t.Errorf("a run is kept as %+v", r)
		}
	}
	if !runs[1].Started.After(runs[0].Ended) {
		t.Errorf("the second run started at %v, before the first ended at %v", runs[1].Started, runs[0].Ended)
	}
}

func TestWorkStartErrorsReported(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.reach.fail("start", fmt.Errorf("the store is out of reach"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.until("the house is told the start failed", func() bool {
		return slices.ContainsFunc(b.entries(n, requests.KindBulldog), func(e requests.Entry) bool {
			return e.Read == "office" && strings.Contains(e.Text, "could not start steward") && strings.Contains(e.Text, "the store is out of reach")
		})
	})
	if !b.logged("could not start steward") {
		t.Error("the failed start is not in the log")
	}
	if ds, err := b.reqs.Deliveries(b.entries(n, requests.KindBulldog)[0].ID); err != nil || len(ds) != 2 {
		t.Errorf("what failed goes to %v (%v)", ds, err)
	}
	b.reach.fail("start", nil)
	b.later(Again)
	b.started(n, 2)
	b.settled(n, requests.Answered)
	if runs := b.runs(n); runs[0].Status != unsent || runs[1].Status != done {
		t.Errorf("the runs are %+v", runs)
	}

	// An ending that cannot be kept is said too, and kept once the store
	// is back.
	held, let := b.hold("again", testkit.Say("mended again"))
	b.say(n, "kit", "it sparked again")
	run := b.started(n, 3)
	b.until("the steward is held", func() bool { return held() == 1 })
	back := b.away("work_requests")
	let()
	b.corbie.Ended(t, run.Name)
	b.until("the ending could not be kept", func() bool { return b.logged("could not keep how the run of steward ended") })
	back()
	if !slices.ContainsFunc(b.entries(n, requests.KindBulldog), func(e requests.Entry) bool {
		return e.Read == "office" && strings.Contains(e.Text, "could not keep how the run of steward ended")
	}) {
		t.Errorf("the house is not told its ending could not be kept: %v", b.thread(n))
	}
	b.later(Again)
	b.settled(n, requests.Answered)
	if answers := b.entries(n, requests.KindAnswer); len(answers) != 2 || answers[1].Text != "mended again" {
		t.Errorf("the thread is %v", b.thread(n))
	}
}

func TestWorkNoRequestWorkingWithNothingToMoveIt(t *testing.T) {
	t.Parallel()
	b := newBench(t, desk(house(t), `if (input.said === "fail") throw new Error("no"); return input.said === "empty" ? "" : "filed"`))
	stuck := func(n int64) {
		t.Helper()
		b.until(fmt.Sprintf("request %d has nothing going and is not working", n), func() bool {
			st, err := b.reqs.State(n)
			if err != nil {
				t.Fatal(err)
			}
			return st.Is != requests.Working && !slices.ContainsFunc(b.runs(n), func(r KeptRun) bool { return r.Status == going })
		})
	}
	for _, said := range []string{"file it", "fail", "empty"} {
		stuck(b.chat("kit", "desk", said))
	}
	// What is said while a run goes starts the next run once it ends.
	held, let := b.hold("socket", testkit.Say("on it"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.until("the steward is held", func() bool { return held() == 1 })
	b.say(n, "kit", "hurry")
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("done") })
	let()
	b.started(n, 2)
	stuck(n)
}

func TestATeamWithNoPeopleIsToldThroughWhomItsDecisionsReach(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["platform/ops.yaml"] = "flow: ops\ninput: [said]\nstages:\n  a: { job: { script: ./ops.js } }\noutput: ${stages.a.output}\n"
	files["platform/ops.js"] = "return env.get(\"GITLAB_TOKEN\")\n"
	b := newBench(t, files)
	_, let := b.hold("charged twice", testkit.Say("asked platform"))
	n := b.chat("dana", "", "customer 4411 was charged twice")
	b.started(n, 1)
	sub, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "platform", "what changed on Tuesday?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	let()
	b.settled(sub.Number, requests.Answered)
	var told []string
	for _, e := range b.entries(sub.Number, requests.KindBulldog) {
		ds, err := b.reqs.Deliveries(e.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			if d.Person != "" {
				told = append(told, d.Person+": "+e.Text)
			}
		}
	}
	if len(told) != 1 || !strings.HasPrefix(told[0], "al: Platform's work failed") || !strings.Contains(told[0], "env") {
		t.Errorf("of what failed, the people were told %q", told)
	}
}
