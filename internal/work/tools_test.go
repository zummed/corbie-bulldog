package work

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

// fixing has the steward hand Kit's socket to the fixer, which makes one
// call; each then answers with what it last heard.
func fixing(tool string, args map[string]any) func(testkit.Turn) testkit.Reply {
	return func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "socket sparked") && turn.Number == 1:
			return testkit.Use("fixer", map[string]any{"job": "mend the kitchen socket"})
		case turn.Prompt() == "mend the kitchen socket" && turn.Number == 1:
			return testkit.Use(tool, args)
		case turn.Prompt() == "mend the kitchen socket":
			return testkit.Say("fixer heard: " + turn.Last())
		}
		return testkit.Say("steward heard: " + turn.Last())
	}
}

// answered gives a request's one answer, once it is answered.
func (b *bench) answered(n int64) requests.Entry {
	b.t.Helper()
	b.settled(n, requests.Answered)
	answers := b.entries(n, requests.KindAnswer)
	if len(answers) != 1 {
		b.t.Fatalf("request %d is answered %v", n, b.thread(n))
	}
	return answers[len(answers)-1]
}

// waitsOn stays until an entry of a kind is in a request, and gives it.
func (b *bench) waitsOn(n int64, kind string) requests.Entry {
	b.t.Helper()
	var e requests.Entry
	b.until(fmt.Sprintf("request %d has a %s", n, kind), func() bool {
		es := b.entries(n, kind)
		if len(es) > 0 {
			e = es[len(es)-1]
		}
		return len(es) > 0
	})
	return e
}

// calls gives the runs of the shipped tools' calls below a request's runs.
func (b *bench) calls(n int64) []*client.Run {
	b.t.Helper()
	runs, err := b.corbie.List(ctx, client.Filter{Labels: map[string]string{LabelBulldog: identity, requests.LabelRequest: fmt.Sprint(n)}, Finished: true})
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Machine != callMachine })
}

func TestAskPersonWaitsForTheAnswer(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("ask_person", map[string]any{"question": "tell me when the socket arrives"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	q := b.waitsOn(n, requests.KindQuestion)
	if q.By.Name != "fixer" || len(q.To) != 1 || q.To[0].Name != "kit" || q.Text != "tell me when the socket arrives" {
		t.Errorf("the question is %+v", q)
	}
	b.until("the request waits on Kit", func() bool {
		st, err := b.reqs.State(n)
		return err == nil && st.Is == requests.Waiting && len(st.On) == 1 && st.On[0].Name == "kit"
	})
	if _, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "it came today", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	if a := b.answered(n); a.Text != "steward heard: fixer heard: Kit answered: it came today" {
		t.Errorf("the steward answered %q", a.Text)
	}
	if runs := b.runs(n); len(runs) != 1 {
		t.Errorf("Kit's answer, given to the waiting tool, started another run: %+v", runs)
	}
}

func TestWorkAnswerGoesToOnePlace(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("ask_person", map[string]any{"question": "tell me when the socket arrives"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	q := b.waitsOn(n, requests.KindQuestion)
	reply, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "it came today", nil, requests.Screens)
	if err != nil {
		t.Fatal(err)
	}
	b.answered(n)
	// The answer went to the tool, and is no entry that starts the
	// lead: saying more starts a run given only what was said.
	b.say(n, "kit", "and the fuse?")
	run := b.started(n, 2)
	said := fmt.Sprint(b.input(run.Name)["said"])
	if said != "and the fuse?" || strings.Contains(said, reply.Text) {
		t.Errorf("the second run is given %q", said)
	}
}

func TestAskPersonIsRefusedForWhomItMayNotAsk(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	results := make(chan string, 4)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?") && turn.Number == 1:
			return testkit.Use("ask_person", map[string]any{"question": "refund it?", "who": "Dana"})
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?") && turn.Number == 2:
			results <- turn.Last()
			return testkit.Use("ask_person", map[string]any{"question": "refund it?"})
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?") && turn.Number == 3:
			results <- turn.Last()
			return testkit.Use("tell", map[string]any{"text": "refunded"})
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?") && turn.Number == 4:
			results <- turn.Last()
		}
		return testkit.Say("done")
	})
	n := b.chat("al", "", "customer 4411 was charged twice")
	b.answered(n)
	sub, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "payments", "was 4411 charged twice?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	b.settled(sub.Number, requests.Answered)
	dana, nobody, tell := <-results, <-results, <-results
	holds(t, "asking Dana", dana, "Dana is neither a person of Payments", "nobody was asked in their place", "Lee or Priya")
	holds(t, "asking the asker", nobody, "The asker of request", "is bot", "reads only your answer", "Lee or Priya")
	holds(t, "telling the asker", tell, "is bot", "reads only your answer", "nobody was told")
	for _, r := range []string{dana, nobody, tell} {
		if strings.Count(r, ". ") > 0 || !strings.HasSuffix(r, ".") {
			t.Errorf("the refusal %q is not one sentence", r)
		}
	}
	if es := slices.Concat(b.entries(sub.Number, requests.KindQuestion), b.entries(sub.Number, requests.KindTold)); len(es) != 0 {
		t.Errorf("someone was asked or told in their place: %v", b.thread(sub.Number))
	}
}

func TestWorkNoOneAskedInAnothersPlace(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("ask_person", map[string]any{"question": "is the board safe?", "who": "Al"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	a := b.answered(n)
	holds(t, "the fixer's result", a.Text, "Al is neither a person of the house", "nobody was asked in their place", "Sam or Kit")
	if qs := b.entries(n, requests.KindQuestion); len(qs) != 0 {
		t.Errorf("someone else was asked: %v", b.thread(n))
	}
}

func TestARoutinesRunAsksNobodyUnnamed(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	results := make(chan string, 1)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "Walk round") {
			if turn.Number == 1 {
				return testkit.Use("ask_person", map[string]any{"question": "anything open?"})
			}
			results <- turn.Last()
		}
		return testkit.Say("done")
	})
	r, err := b.reqs.AskByRoutine("office", b.office().TopTeam().Routines[0], requests.Who{})
	if err != nil {
		t.Fatal(err)
	}
	b.settled(r.Number, requests.Answered)
	holds(t, "the routine's question", <-results, "asked by the routine of the house", "nobody was asked", "Sam or Kit")
}

func TestWorkQuestionToUnreachablePersonRefused(t *testing.T) {
	t.Parallel()
	files := work(t)
	b := newBench(t, files)
	results := make(chan string, 1)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"question": "which customer are you?"})
		}
		results <- turn.Last()
		return testkit.Say("done")
	})
	// A guest asks in #help; then the Slack bridge stops with a problem,
	// and nothing reaches the guest, who is reached only there.
	_, let := b.hold("charged", testkit.Use("ask_person", map[string]any{"question": "which customer are you?"}))
	r, err := b.reqs.AskAt("slack", "#help", "#help/1700.3", "", "U0GUEST1", "I was charged twice", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	b.started(r.Number, 1)
	b.rewrite(map[string]string{"bridges/slack/in.yaml": ""})
	b.says(func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"question": "which customer are you?"})
		}
		results <- turn.Last()
		return testkit.Say("done")
	})
	let()
	b.settled(r.Number, requests.Answered)
	holds(t, "the bot's question", <-results, "is a guest", "on slack", "nobody was asked")
	if qs := b.entries(r.Number, requests.KindQuestion); len(qs) != 0 {
		t.Errorf("a question waits where nobody sees it: %v", b.thread(r.Number))
	}
}

func TestTellSaysAndGoesOn(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(fixing("tell", map[string]any{"text": "the power is off at breaker 4", "who": "Sam"}))
	n := b.chat("kit", "", "the kitchen socket sparked")
	a := b.answered(n)
	told := b.entries(n, requests.KindTold)
	if len(told) != 1 || told[0].By.Name != "fixer" || told[0].To[0].Name != "sam" || told[0].Text != "the power is off at breaker 4" {
		t.Errorf("the thread is %v", b.thread(n))
	}
	holds(t, "the fixer's result", a.Text, "Told Sam")
	// The call's run ended done with its answer as its output, which the
	// fixer was given.
	calls := b.calls(n)
	if len(calls) != 1 {
		t.Fatalf("the calls are %v", calls)
	}
	if call, err := b.corbie.Show(ctx, calls[0].ID); err != nil || call.Status != "done" || !strings.Contains(fmt.Sprint(call.Output), "Told Sam") {
		t.Errorf("the call ran as %+v (%v)", call, err)
	}
}

func TestAskTeamMakesASubRequest(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "charged twice") && turn.Number == 1 && !strings.Contains(turn.Prompt(), "was 4411"):
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			return testkit.Say("Duplicate confirmed; £40 refunded.")
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	s := b.waitsOn(n, requests.KindSub)
	if s.By.Name != "bot" || s.Text != "was 4411 charged twice?" || s.Sub == 0 {
		t.Fatalf("the sub-request is asked as %+v", s)
	}
	ledger := b.started(s.Sub, 1)
	if ledger.Worker != "ledger" || ledger.Team != "payments" {
		t.Errorf("the sub-request's run is %+v", ledger)
	}
	sub, err := b.reqs.Get(s.Sub)
	if err != nil || sub.Asker.Name != "bot" || sub.Team != "payments" {
		t.Errorf("the sub-request is %+v (%v)", sub, err)
	}
	a := b.answered(n)
	holds(t, "the bot's answer", a.Text, "ledger answered for Payments", "Duplicate confirmed; £40 refunded.")
	if late := b.entries(n, requests.KindLate); len(late) != 0 || len(b.runs(n)) != 1 {
		t.Errorf("payments' answer went to the tool and into the thread: %v", b.thread(n))
	}
}

func TestTheAnswerOfALeadWhoIsAPersonIsTheToolsResult(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_team", map[string]any{"team": "support", "question": "has 4411 written in?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("al", "", "customer 4411 was charged twice")
	s := b.waitsOn(n, requests.KindSub)
	if runs := b.runs(s.Sub); len(runs) != 0 {
		t.Errorf("a run started for a team whose lead is Dana: %+v", runs)
	}
	if _, err := b.reqs.Say(s.Sub, b.reqs.Person("dana"), "Yes, on Tuesday.", nil, requests.Screens, false); err != nil {
		t.Fatal(err)
	}
	holds(t, "the bot's answer", b.answered(n).Text, "Dana answered for Support", "Yes, on Tuesday.")
}

func TestAskTeamIsRefusedWithASentence(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	results := make(chan string, 2)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch turn.Number {
		case 1:
			return testkit.Use("ask_team", map[string]any{"team": "billing", "question": "is 4411 owed?"})
		case 2:
			results <- turn.Last()
			return testkit.Use("ask_team", map[string]any{"team": "office", "question": "is 4411 owed?"})
		}
		results <- turn.Last()
		return testkit.Say("done")
	})
	n := b.chat("al", "", "customer 4411 was charged twice")
	b.answered(n)
	billing, own := <-results, <-results
	holds(t, "asking billing", billing, `no team called "billing"`, "platform", "payments", "support")
	holds(t, "asking its own team", own, "your own team", "tools")
	if subs := b.entries(n, requests.KindSub); len(subs) != 0 {
		t.Errorf("a team was asked: %v", b.thread(n))
	}
}

func TestAskTeamIsRefusedEightLevelsDown(t *testing.T) {
	t.Parallel()
	files := house(t)
	for i := 1; i <= 9; i++ {
		files[fmt.Sprintf("a%d/team.yaml", i)] = fmt.Sprintf("name: A%d\npeople: [sam]\nlead: w%d\n", i, i)
		files[fmt.Sprintf("a%d/w%d.yaml", i, i)] = fmt.Sprintf("agent: w%d\ninput: [said]\nmodel: usual\nprompt: ${input.said}\ntools_file: [../.bulldog/tools/talk/talk.yaml]\n", i)
	}
	b := newBench(t, files)
	results := make(chan string, 1)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if level := strings.TrimPrefix(turn.Prompt(), "level "); level != turn.Prompt() && turn.Number == 1 {
			var i int
			fmt.Sscan(level, &i)
			return testkit.Use("ask_team", map[string]any{"team": fmt.Sprintf("a%d", i%9+1), "question": fmt.Sprintf("level %d", i+1)})
		}
		if strings.HasPrefix(turn.Prompt(), "level 9") {
			results <- turn.Last()
		}
		return testkit.Say("done")
	})
	b.chat("sam", "a1", "level 1")
	holds(t, "the ninth ask", <-results, "already 8 levels of sub-requests below")
}

func TestWorkATeamAboveIsNotAsked(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	results := make(chan string, 3)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			switch turn.Number {
			case 1:
				results <- turn.Prompt()
				return testkit.Use("ask_team", map[string]any{"team": "office", "question": "who is customer 4411?"})
			case 2:
				results <- turn.Last()
				return testkit.Use("ask_team", map[string]any{"team": "billing", "question": "is 4411 owed?"})
			}
			results <- turn.Last()
			return testkit.Say("Duplicate confirmed.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	context, above, billing := <-results, <-results, <-results
	holds(t, "the ledger's context", context, fmt.Sprintf("beneath request %d of Platform support", n), "- platform (", "- support (")
	holdsNone(t, "the ledger's context", context, "- office (")
	holds(t, "asking the office", above, fmt.Sprintf("This request came from Platform support, in request %d, and your answer goes back to it", n),
		"no team was asked")
	holds(t, "asking billing", billing, "you may ask platform or support.")
	b.answered(n)
	if subs := b.entries(sub, requests.KindSub); len(subs) != 0 {
		t.Errorf("a team was asked from payments: %v", b.thread(sub))
	}
	decs, err := b.d.OfRequest(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 || decs[0].Said.What != decisions.SaysAnswer {
		t.Errorf("payments' sub-request has the decisions %+v", decs)
	}
	run := b.runs(sub)[0]
	origin := b.input(run.Name)["request"].(map[string]any)["origin"].(map[string]any)
	if from, _ := json.Marshal(origin["above"]); string(from) != fmt.Sprintf(`[{"number":%d,"team":"office"}]`, n) {
		t.Errorf("the ledger is given it came from %s", from)
	}
}

func TestWorkOwnAskTeamNarrowsTheTeamsItMayAsk(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["bot.yaml"] += `tools:
  ask_team:
    does: Ask payments, and wait for its answer
    args:
      type: object
      properties:
        team: { type: string, enum: [payments] }
        question: { type: string }
      required: [team, question]
    job:
      machine: ./.bulldog/call.yaml
      input: { tool: ask_team, args: "${args}", unanswered: No answer came within 7 days. }
`
	b := newBench(t, files)
	if may := MayAsk(b.office(), "office", "bot", nil); len(may) != 1 || may[0].Name != "payments" {
		t.Errorf("the bot may ask %v", may)
	}
	if may := MayAsk(b.office(), "payments", "ledger", nil); len(may) != 3 {
		t.Errorf("the ledger may ask %v", may)
	}
	results := make(chan string, 1)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch turn.Number {
		case 1:
			return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "did the deploy break?"})
		case 2:
			results <- turn.Last()
		}
		return testkit.Say("done")
	})
	n := b.chat("al", "", "the deploy broke")
	holds(t, "the bot's context", b.prompt("the deploy broke"), "- payments (")
	holdsNone(t, "the bot's context", b.prompt("the deploy broke"), "- platform (", "- support (")
	holds(t, "asking platform", <-results, "at /team", "payments")
	b.answered(n)
	if subs := b.entries(n, requests.KindSub); len(subs) != 0 {
		t.Errorf("a team was asked: %v", b.thread(n))
	}
}

func TestWorkToolsActOnOwnRequestOnly(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	_, let := b.hold("socket", testkit.Say("done"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.started(n, 1)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "Walk round") && turn.Number == 1 {
			return testkit.Use("tell", map[string]any{"text": fmt.Sprintf("about request %d: it is in hand", n), "who": "Kit"})
		}
		return testkit.Say("done")
	})
	r, err := b.reqs.AskByRoutine("office", b.office().TopTeam().Routines[0], requests.Who{})
	if err != nil {
		t.Fatal(err)
	}
	b.settled(r.Number, requests.Answered)
	let()
	b.settled(n, requests.Answered)
	if told := b.entries(r.Number, requests.KindTold); len(told) != 1 {
		t.Errorf("the routine's tell is not in its own request: %v", b.thread(r.Number))
	}
	if told := b.entries(n, requests.KindTold); len(told) != 0 {
		t.Errorf("the routine's tell went into Kit's request: %v", b.thread(n))
	}
}

func TestWorkEveryShippedToolIsDone(t *testing.T) {
	t.Parallel()
	for _, file := range shipped.ToolsFiles {
		tools := shipped.Tools(file)
		if len(tools) == 0 {
			t.Errorf("%s holds no tool", file)
		}
		for _, tool := range tools {
			if !does(tool) {
				t.Errorf("Bulldog does not do %s, of %s", tool, file)
			}
		}
	}
	for _, tool := range []string{"read_notes", "ask", "call", ""} {
		if does(tool) {
			t.Errorf("Bulldog does %q, which no file it ships holds", tool)
		}
	}
	b := newBench(t, caller(house(t), "read_notes", "{}"))
	n := b.chat("kit", "desk", "file the bill")
	holds(t, "a tool in no file Bulldog ships", b.answered(n).Text, `Bulldog does no tool called "read_notes"`)
}

// caller is the house with a desk, whose lead is a flow that makes one
// call of a shipped tool, its waiting chart written in place, and answers
// with its result.
func caller(files map[string]string, tool, args string) map[string]string {
	files["desk/team.yaml"] = "name: The desk\nabout: Papers.\npeople: [sam, kit]\nlead: filer\n"
	files["desk/filer.yaml"] = "flow: filer\ninput: [said]\nstages:\n  a:\n    job:\n" + indent(callChart(tool, args), 6) + "output: ${stages.a.output}\n"
	return files
}

// callChart is a job that runs the shipped tools' waiting chart, written
// in place, with a tool and its arguments.
func callChart(tool, args string) string {
	return `machine:
  statechart: call
  input: [tool, args]
  initial: waiting
  states:
    waiting: { ticket: answer, timers: { too_long: 7d }, on: { answer: answered, too_long: unanswered } }
    answered: { end: done, output: "${event.data}" }
    unanswered: { end: done, output: still open }
input: { tool: ` + tool + `, args: ` + args + ` }
`
}

// indent puts spaces before each line of a text.
func indent(text string, spaces int) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		if line != "" {
			b.WriteString(strings.Repeat(" ", spaces) + line)
		}
	}
	return b.String()
}

func TestASubRequestThatFailsIsToldToTheTool(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["platform/ops.yaml"] = strings.Replace(files["platform/ops.yaml"], "model: usual", "model: usual\nlimits: { turns: 1 }", 1)
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "what changed on Tuesday?"):
			return testkit.Use("gitlab", map[string]any{"path": "projects/12/merge_requests"})
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "what changed on Tuesday?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	if a := b.answered(n); a.Text != "bot heard: Platform could not answer." {
		t.Errorf("the bot was given %q", a.Text)
	}
	var failed *requests.Entry
	for _, e := range b.entries(sub, requests.KindBulldog) {
		if e.Read == "platform" {
			failed = &e
		}
	}
	if failed == nil || !strings.Contains(failed.Text, "Platform's work failed") {
		t.Fatalf("platform's thread is %v", b.thread(sub))
	}
	if ds, err := b.reqs.Deliveries(failed.ID); err != nil || len(ds) != 1 || ds[0].Person != "al" {
		t.Errorf("what failed is carried to %+v (%v)", ds, err)
	}
	holdsNone(t, "the bot's request", strings.Join(b.thread(n), "\n"), "failed", "turns")
}
