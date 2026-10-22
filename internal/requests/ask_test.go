package requests

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/store"
)

// refund is Dana's question in #refunds on Slack, naming the bot.
func refund(thing string) Ask {
	return Ask{Team: "payments", By: By{Person: "dana"}, Said: "@bot was order 4411 refunded?", Mentioned: true, At: stamp,
		Place: at("#refunds", "C01/1700000000.0001", thing)}
}

// A request heard on a bridge is named from the thing, given what Bulldog
// knows of it, and its answer is carried back to where it was said,
// replying to the thing.
func TestAskFromABridge(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e, _ := o.run()
	run, err := e.Ask(ctx, refund("C01/1700000000.0002"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^client/` + regexp.QuoteMeta(store.Stamp(stamp)) + `-[0-9a-f]{16}$`).MatchString(run) {
		t.Errorf("the request is named %s", run)
	}
	said, context, request := o.given(run)
	if said != "@bot was order 4411 refunded?" {
		t.Errorf("the request is given %q", said)
	}
	for field, want := range map[string]string{
		"team":      "payments",
		"by":        "map[person:dana]",
		"heard":     "channel",
		"place":     "map[bridge:slack guests:false place:#refunds thing:C01/1700000000.0002 where:C01/1700000000.0001]",
		"teams":     "[map[about:One bot over every team. name:office]]",
		"chain":     "[]",
		"thread":    "[]",
		"open":      "[]",
		"docs":      o.office.ID + "/payments/docs",
		"mentioned": "true",
	} {
		if got := fmt.Sprint(request[field]); got != want {
			t.Errorf("the request's %s is %s, not %s", field, got, want)
		}
	}
	for _, words := range []string{"It is Monday 5 October 2026, 09:00, in UTC.", "You work for Payments (payments)", "Its people: Lee (lee), Priya (priya). Its workers: ledger.",
		"Dana (dana), of Work asked, at #refunds on slack.", "- office: One bot over every team."} {
		if !strings.Contains(context, words) {
			t.Errorf("the context does not say %q:\n%s", words, context)
		}
	}
	shown, _ := o.corbie.Show(ctx, run)
	if l := shown.Labels; len(l) != 4 || l["bulldog"] != o.office.ID || l["kind"] != "request" || l["team"] != "payments" || len(l["thread"]) != 16 {
		t.Errorf("the request is labelled %v", l)
	}

	sent := o.until(1)
	m := sent[0]
	if m.Bridge != "slack" || m.To != "C01/1700000000.0001" || m.Direct || m.Text != "Refunded £40 on 3 May." || m.Answers != "C01/1700000000.0002" {
		t.Errorf("the answer is carried as %+v", m)
	}
	if l := m.Labels; l["kind"] != "out" || l["bridge"] != "slack" || l["request"] != run || l["bulldog"] != o.office.ID {
		t.Errorf("the answer is carried by a run labelled %v", l)
	}
}

// TestRequestsStartedOnce: the same thing heard twice, by one engine or by
// a second after a restart, starts one request, whose answer is carried
// once. Asking in the chat starts a request each time.
func TestRequestsStartedOnce(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e, stop := o.run()
	first, err1 := e.Ask(ctx, refund("C01/1700000000.0002"))
	second, err2 := e.Ask(ctx, refund("C01/1700000000.0002"))
	if err1 != nil || err2 != nil || first != second {
		t.Fatalf("the thing heard twice is %s (%v) and %s (%v)", first, err1, second, err2)
	}
	o.until(1)
	stop()
	again, _ := o.run()
	if third, err := again.Ask(ctx, refund("C01/1700000000.0002")); err != nil || third != first {
		t.Errorf("heard after a restart, it is %s (%v)", third, err)
	}
	// What the second engine does after its first listing comes after
	// everything it would do twice.
	o.answered(o.chat(again, "office", "Anyone there?"))
	other, _ := again.Ask(ctx, refund("C01/1700000000.0003"))
	o.until(2)
	if n := len(o.requests("payments")); n != 2 {
		t.Errorf("payments has %d requests", n)
	}
	if sent := o.carrier.carried(); len(sent) != 2 || sent[1].Labels["request"] != other {
		t.Errorf("the carrier carried %+v", sent)
	}
	if a, b := o.chat(again, "office", "Hello?"), o.chat(again, "office", "Hello?"); a == b {
		t.Error("two questions in the chat are one request")
	}
}

// TestRequestsLabelsHoldNoWords: no label and no run name of the office
// holds a word of what was said, or a person's key or identity.
func TestRequestsLabelsHoldNoWords(t *testing.T) {
	t.Parallel()
	o := newOffice(t, func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.System, "You work for Work") && turn.Number == 1 {
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "Was order 4411 refunded?"})
		}
		if strings.Contains(turn.System, "You work for Payments") && turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"who": "lee", "question": "Did 4411 go through?"})
		}
		return testkit.Say("Refunded on 3 May.")
	}, nil)
	e, _ := o.run()
	run, err := e.Ask(ctx, Ask{Team: "office", By: By{Person: "dana"}, Said: "Was order 4411 refunded?", At: stamp,
		Place: &Place{Bridge: "telegram", Place: "direct", Where: "4401", Thing: "4401:77"}})
	if err != nil {
		t.Fatal(err)
	}
	asked := o.waitFor("Did 4411")
	o.until(2)
	if err := e.Answer(ctx, asked.ID, "lee", "Yes, refunded.", "4402:9"); err != nil {
		t.Fatal(err)
	}
	o.answered(run)
	o.until(3)
	runs, err := o.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": o.office.ID}, Finished: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		for name, value := range r.Labels {
			for _, word := range []string{"refund", "order", "dana", "lee", "U0DANA", "U0LEE", "4401:77"} {
				if strings.Contains(strings.ToLower(r.ID+" "+name+"="+value), strings.ToLower(word)) {
					t.Errorf("the run %s, labelled %s=%s, holds %q", r.ID, name, value, word)
				}
			}
		}
	}
}

// A request is given the requests before it in its thread, the newest
// within 20,000 characters, and its context says how many are left out.
func TestThreadNewestWithin(t *testing.T) {
	t.Parallel()
	o := newOffice(t, says, nil)
	e := o.engine()
	direct := func(n int) Ask {
		return Ask{Team: "office", By: By{Person: "dana"}, Said: fmt.Sprintf("%03d", n) + strings.Repeat("x", 997), At: stamp.Add(time.Duration(n) * time.Second),
			Place: &Place{Bridge: "telegram", Place: "direct", Where: "4401", Thing: fmt.Sprintf("4401:%d", n)}}
	}
	var runs []string
	for n := range 22 {
		run, err := e.Ask(ctx, direct(n))
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	for _, run := range runs {
		o.answered(run)
	}
	run, err := e.Ask(ctx, direct(22))
	if err != nil {
		t.Fatal(err)
	}
	_, context, request := o.given(run)
	thread := request["thread"].([]any)
	if len(thread) != 19 {
		t.Fatalf("the request is given %d of its thread", len(thread))
	}
	if newest := thread[18].(map[string]any); !strings.HasPrefix(newest["said"].(string), "021") || newest["answer"] != "Refunded £40 on 3 May." ||
		fmt.Sprint(newest["by"]) != "map[person:dana]" {
		t.Errorf("the newest of the thread is %v", newest)
	}
	if !strings.Contains(context, "in the same thread (3 earlier left out)") {
		t.Errorf("the context does not say 3 are left out:\n%s", context)
	}
	// Another chat is another thread.
	_, _, request = o.given(o.chat(e, "office", "Hello?"))
	if thread := request["thread"].([]any); len(thread) != 0 {
		t.Errorf("a new chat is given %d of a thread", len(thread))
	}
}

// asksTwice is an office whose bot asks payments what its prompt says, a
// question each, and payments' ledger answers each question with what it
// was given of its thread.
func asksTwice(turn testkit.Turn) testkit.Reply {
	if strings.Contains(turn.System, "You work for Payments") {
		return testkit.Say("Refunded £40.")
	}
	questions := strings.Split(turn.Prompt(), "; ")
	if turn.Number <= len(questions) {
		return testkit.Use("ask_team", map[string]any{"team": "payments", "question": questions[turn.Number-1]})
	}
	return testkit.Say("Told: " + turn.Last())
}

// TestRequestsSubRequestGivenOwnThreadOnly: a sub-request is given the
// requests the same asking request made of the same team, and of the
// requests above it only the asking team, the chain and where it began.
func TestRequestsSubRequestGivenOwnThreadOnly(t *testing.T) {
	t.Parallel()
	o := newOffice(t, asksTwice, nil)
	e, _ := o.run()
	first, err := e.Ask(ctx, Ask{Team: "office", By: By{Guest: "U0GUEST0"}, Said: "first?; second?", At: stamp,
		Place: &Place{Bridge: "slack", Place: "#help", Guests: true, Where: "C02/1", Thing: "C02/1"}})
	if err != nil {
		t.Fatal(err)
	}
	if answer := o.answered(first); !strings.Contains(answer, "Refunded £40.") {
		t.Errorf("the bot answered %q", answer)
	}
	o.answered(o.chat(e, "office", "third?"))
	subs := o.requests("payments")
	if len(subs) != 3 {
		t.Fatalf("payments was asked %d times", len(subs))
	}
	given := map[string]map[string]any{}
	for _, sub := range subs {
		said, context, request := o.given(sub.ID)
		given[said] = request
		for _, words := range []string{"first?; second?", "third?", "U0GUEST0", "C02/1"} {
			if words != said && strings.Contains(fmt.Sprint(request, context), words) {
				t.Errorf("the sub-request %q is given %q", said, words)
			}
		}
		if fmt.Sprintf("%v %v %v", request["by"], request["heard"], request["chain"]) != "map[team:office] team [office]" {
			t.Errorf("the sub-request %q is given %v, %v, %v", said, request["by"], request["heard"], request["chain"])
		}
		if sub.Labels["wait"] == "" {
			t.Errorf("the sub-request %q is labelled %v", said, sub.Labels)
		}
	}
	if fmt.Sprint(given["first?"]["place"]) != "map[bridge:slack guests:true place:#help]" || given["third?"]["place"] != nil {
		t.Errorf("the sub-requests are given the places %v and %v", given["first?"]["place"], given["third?"]["place"])
	}
	if thread := given["second?"]["thread"].([]any); len(thread) != 1 || thread[0].(map[string]any)["said"] != "first?" ||
		thread[0].(map[string]any)["answer"] != "Refunded £40." {
		t.Errorf("the second sub-request is given the thread %v", thread)
	}
	for _, said := range []string{"first?", "third?"} {
		if thread := given[said]["thread"].([]any); len(thread) != 0 {
			t.Errorf("the sub-request %q is given the thread %v", said, thread)
		}
	}
}

// TestRequestsAskInALoopRefused: a team asking itself, a team of its
// chain, or no team is refused, naming the teams it may ask, and starts
// nothing.
func TestRequestsAskInALoopRefused(t *testing.T) {
	t.Parallel()
	o := newOffice(t, func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			team, question, _ := strings.Cut(turn.Prompt(), ": ")
			return testkit.Use("ask_team", map[string]any{"team": team, "question": question})
		}
		return testkit.Say("Told: " + turn.Last())
	}, nil)
	e, _ := o.run()
	for prompt, want := range map[string]string{
		"office: hello?":   "Work cannot ask itself. It may ask payments.",
		"nowhere: hello?":  "There is no team nowhere. It may ask payments.",
		"payments: office": "The team office asked this request, so it is not asked again. It may ask no other team.",
	} {
		answer := o.answered(o.chat(e, "office", prompt))
		if !strings.Contains(answer, want) || !strings.Contains(answer, "refused") {
			t.Errorf("asked %q, the bot is told %s", prompt, answer)
		}
	}
	if n := len(o.requests("office")); n != 3 {
		t.Errorf("the office has %d requests", n)
	}
	if n := len(o.requests("payments")); n != 1 {
		t.Errorf("payments has %d requests", n)
	}
}

// TestRequestsAskingTeamFromLabel: the asking team is the wait's team
// label and the chain its request's, whatever the wait's input says.
func TestRequestsAskingTeamFromLabel(t *testing.T) {
	t.Parallel()
	o := newOffice(t, asksTwice, func(files map[string]string) {
		files["payments/team.yaml"] = "name: Payments\nabout: The ledger.\npeople: [lee]\nlead: forger\n"
		files["payments/forger.yaml"] = `flow: forger
stages:
  back:
    job:
      machine: ./.bulldog/wait/wait.yaml
      input: { to: { team: office }, said: "Back to you?", from: platform, chain: [], team: platform }
  up:
    job:
      machine: ./.bulldog/wait/wait.yaml
      input: { to: { team: platform }, said: "Is it up?", by: { team: office }, chain: [] }
output: { answer: "${stages.back.output.refused + ' ' + stages.up.output.answer}" }
`
		files["platform/team.yaml"] = "name: Platform\nabout: The servers.\nlead: ops\n"
		files["platform/ops.yaml"] = "flow: ops\nstages:\n  up: { job: { script: ./up.js } }\noutput: ${stages.up.output}\n"
		files["platform/up.js"] = `return "It is up."`
	})
	e, _ := o.run()
	answer := o.answered(o.chat(e, "office", "Is the charge service up?"))
	if !strings.Contains(answer, "The team office asked this request, so it is not asked again. It may ask platform. It is up.") {
		t.Errorf("the bot is told %s", answer)
	}
	asked := o.requests("platform")
	if len(asked) != 1 {
		t.Fatalf("platform has %d requests", len(asked))
	}
	_, _, request := o.given(asked[0].ID)
	if fmt.Sprintf("%v %v", request["by"], request["chain"]) != "map[team:payments] [office payments]" {
		t.Errorf("platform's request is by %v, with the chain %v", request["by"], request["chain"])
	}
	if slices.ContainsFunc(o.requests("office"), func(r *client.Run) bool { return r.Labels["wait"] != "" }) {
		t.Error("the office was asked back")
	}
}
