package work

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestARequestStartsItsLead(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	if run.Worker != "steward" || run.Team != "office" {
		t.Errorf("the run is %+v", run)
	}
	var listed []*client.Run
	b.until("the run is listed", func() bool {
		listed = b.listed(map[string]string{LabelKind: KindWork})
		return len(listed) > 0
	})
	if len(listed) != 1 || listed[0].ID != run.Name {
		t.Fatalf("the runs listed are %v", listed)
	}
	want := map[string]string{LabelBulldog: identity, LabelKind: KindWork, requests.LabelTeam: "office", requests.LabelRequest: fmt.Sprint(n)}
	for k, v := range want {
		if listed[0].Labels[k] != v {
			t.Errorf("the label %s is %q, not %q", k, listed[0].Labels[k], v)
		}
	}
	if said := b.input(run.Name)["said"]; said != "the kitchen socket sparked" {
		t.Errorf("said is %q", said)
	}
	b.settled(n, requests.Answered)
}

func TestARequestAtAPlaceStartsTheTeamItNames(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`,
		`"#help": { guests: true }`+"\n      \"#refunds\": { team: payments }", 1)
	b := newBench(t, files)
	help, err := b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "customer 4411 was charged twice", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if run := b.started(help.Number, 1); run.Worker != "bot" || run.Team != "office" {
		t.Errorf("the run in #help is %+v", run)
	}
	refunds, err := b.reqs.AskAt("slack", "#refunds", "#refunds/1700.2", "", "U0PRI000", "refund 4411's second charge", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	run := b.started(refunds.Number, 1)
	if run.Worker != "ledger" || run.Team != "payments" {
		t.Errorf("the run in #refunds is %+v", run)
	}
	b.until("the ledger's run is listed with payments' label", func() bool {
		return len(b.listed(map[string]string{requests.LabelRequest: fmt.Sprint(refunds.Number), requests.LabelTeam: "payments"})) == 1
	})
}

func TestATeamWhoseLeadIsAPersonStartsNoRun(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, let := b.hold("charged twice", testkit.Say("asked"))
	defer let()
	n := b.chat("al", "", "customer 4411 was charged twice")
	b.started(n, 1)
	sub, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "support", "what did 4411 say?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	b.until("the sub-request waits on Dana", func() bool {
		st, err := b.reqs.State(sub.Number)
		return err == nil && st.Is == requests.Waiting && len(st.On) == 1 && st.On[0].Name == "dana"
	})
	if runs := b.runs(sub.Number); len(runs) != 0 {
		t.Errorf("a team whose lead is Dana started runs: %+v", runs)
	}
}

func TestSomethingSaidStartsTheLeadAgain(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(turn testkit.Turn) testkit.Reply { return testkit.Say("I will look at the socket.") })
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.started(n, 1)
	b.settled(n, requests.Answered)
	b.say(n, "kit", "it sparked again")
	run := b.started(n, 2)
	in := b.input(run.Name)
	if in["said"] != "it sparked again" {
		t.Errorf("said is %q", in["said"])
	}
	context, _ := in["context"].(string)
	if !strings.Contains(context, "the kitchen socket sparked") || !strings.Contains(context, "I will look at the socket.") ||
		!strings.Contains(context, "it sparked again") {
		t.Errorf("the context does not hold the whole thread:\n%s", context)
	}
	b.settled(n, requests.Answered)
}

func TestOneRunAtATimeForARequest(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket sparked", testkit.Say("on it"))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.started(n, 1)
	b.until("the steward's turn is held", func() bool { return held() == 1 })
	b.say(n, "kit", "it smells of burning")
	b.say(n, "kit", "and the fuse has gone")
	if runs := b.runs(n); len(runs) != 1 {
		t.Fatalf("a second run started while the first went: %+v", runs)
	}
	let()
	run := b.started(n, 2)
	if said := b.input(run.Name)["said"]; said != "Kit: it smells of burning\n\nKit: and the fuse has gone" {
		t.Errorf("said is %q", said)
	}
	b.settled(n, requests.Answered)
	if runs := b.runs(n); len(runs) != 2 {
		t.Errorf("the request had %d runs", len(runs))
	}
}

func TestWorkOneRunPerRequestNotPerWorker(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("please", testkit.Say("done"))
	defer let()
	first := b.chat("kit", "", "please mend the tap")
	second := b.chat("sam", "", "please mend the door")
	b.started(first, 1)
	b.started(second, 1)
	b.until("both runs of the steward are at their first turn", func() bool { return held() == 2 })
	let()
	b.settled(first, requests.Answered)
	b.settled(second, requests.Answered)
}

func TestTheOfficeRunsAtMostItsRunsAtOnce(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, kit]\nruns: 2", 1)
	b := newBench(t, files)
	gates := map[string]chan struct{}{}
	for _, word := range []string{"one", "two", "three", "four"} {
		gates[word] = make(chan struct{})
	}
	b.says(func(turn testkit.Turn) testkit.Reply {
		for word, gate := range gates {
			if strings.HasSuffix(newFor(turn.Prompt()), ": "+word) {
				if word == "one" && turn.Number == 1 {
					return testkit.Use("ask_person", map[string]any{"question": "which tap?"})
				}
				<-gate
				return testkit.Say("done " + word)
			}
		}
		return testkit.Say("done")
	})
	defer func() {
		for _, word := range []string{"two", "three", "four"} {
			select {
			case <-gates[word]:
			default:
				close(gates[word])
			}
		}
	}()
	var ns []int64
	for _, word := range []string{"one", "two", "three", "four"} {
		ns = append(ns, b.chat("kit", "", word))
	}
	b.started(ns[0], 1)
	b.started(ns[1], 1)
	// The first run waits on Kit, so the third starts; the fourth waits.
	b.started(ns[2], 1)
	b.still()
	if runs := b.runs(ns[3]); len(runs) != 0 {
		t.Fatalf("a run started past the office's runs: %+v", runs)
	}
	close(gates["two"])
	b.started(ns[3], 1)
}

func TestARequestWaitsItsTurnOldestFirst(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, kit]\nruns: 1", 1)
	b := newBench(t, files)
	held, let := b.hold("first", testkit.Say("done"))
	first := b.chat("kit", "", "first")
	b.started(first, 1)
	b.until("the first is held", func() bool { return held() == 1 })
	var ns []int64
	for _, word := range []string{"second", "third", "fourth"} {
		ns = append(ns, b.chat("sam", "", word))
	}
	b.still()
	if load := b.w.Load(); load != (Load{Working: 1, Queued: 3}) {
		t.Errorf("with one run at work and three waiting their turn, the load is %+v", load)
	}
	if queue := b.w.Queue(); !slices.Equal(queue, ns) {
		t.Errorf("the requests waiting their turn are %v, not %v", queue, ns)
	}
	let()
	for _, n := range ns {
		b.settled(n, requests.Answered)
	}
	b.still()
	if load := b.w.Load(); load != (Load{}) || len(b.w.Queue()) != 0 {
		t.Errorf("with every request answered, the load is %+v and the queue %v", load, b.w.Queue())
	}
	// One run at a time, so the model is asked in the order they started.
	var order []string
	for _, turn := range b.model.Turns() {
		for _, word := range []string{"second", "third", "fourth"} {
			if strings.HasSuffix(newFor(turn.Prompt()), ": "+word) {
				order = append(order, word)
			}
		}
	}
	if !slices.Equal(order, []string{"second", "third", "fourth"}) {
		t.Errorf("the requests that waited started in the order %v", order)
	}
}

func TestTheLoadKeepsItsLastCountWhileTheRunsCannotBeCounted(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("first", testkit.Say("done"))
	first := b.chat("kit", "", "first")
	b.started(first, 1)
	b.until("the first is held", func() bool { return held() == 1 })
	b.still()
	if load := b.w.Load(); load != (Load{Working: 1}) {
		t.Fatalf("with one run at work, the load is %+v", load)
	}
	back := b.away("work_runs")
	logs := b.logCount()
	second := b.chat("sam", "", "second")
	b.until("the runs going could not be counted", func() bool {
		return slices.ContainsFunc(b.logsSince(logs), func(l string) bool { return strings.Contains(l, "could not be counted") })
	})
	b.still()
	if load := b.w.Load(); load != (Load{Working: 1}) {
		t.Errorf("while the runs going cannot be counted, the load is %+v, not the last count", load)
	}
	back()
	b.later(Again)
	b.started(second, 1)
	let()
	b.settled(first, requests.Answered)
	b.settled(second, requests.Answered)
}

func TestTheOfficeRunsThreeWhenItSaysNone(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	_, let := b.hold("job", testkit.Say("done"))
	defer let()
	var ns []int64
	for i := range 4 {
		ns = append(ns, b.chat("kit", "", fmt.Sprintf("job %d", i)))
	}
	for _, n := range ns[:3] {
		b.started(n, 1)
	}
	b.still()
	if runs := b.runs(ns[3]); len(runs) != 0 {
		t.Fatalf("a fourth run started: %+v", runs)
	}
	let()
	b.started(ns[3], 1)
}

func TestWorkBrokenLeadAnswersWithItsProblem(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	held, let := b.hold("socket", testkit.Say("I have asked Kit."))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.started(n, 1)
	b.until("the steward's turn is held", func() bool { return held() == 1 })
	// The steward asks Kit, as its tool would, and ends: the request is
	// open, waiting on Kit, with nothing going. Then its file breaks.
	if _, err := b.reqs.Put(n, b.reqs.Worker("steward"), requests.KindQuestion, b.reqs.Person("kit"), "which socket?", nil); err != nil {
		t.Fatal(err)
	}
	let()
	b.settled(n, requests.Waiting)
	files := house(t)
	files["steward.yaml"] = strings.Replace(files["steward.yaml"], "model: usual", "model: nowhere", 1)
	b.rewrite(files)
	if b.office().Workers["steward"].Runs() {
		t.Fatal("the steward still runs")
	}
	told := func() int {
		return len(slices.DeleteFunc(b.thread(n), func(l string) bool { return !strings.Contains(l, "The house could not answer.") }))
	}
	b.until("Kit is told the house cannot answer", func() bool { return told() == 1 })
	if !slices.ContainsFunc(b.entries(n, requests.KindBulldog), func(e requests.Entry) bool {
		return e.Read == "office" && strings.Contains(e.Text, "steward cannot run") && strings.Contains(e.Text, "nowhere")
	}) {
		t.Errorf("the house is not told the problem: %v", b.thread(n))
	}
	// Read again, the same problem is not told again; something said is
	// answered again, and the broken steward is never started.
	b.w.Poke()
	b.say(n, "kit", "the one by the kettle")
	b.until("Kit is told again", func() bool { return told() == 2 })
	if runs := b.runs(n); len(runs) != 1 {
		t.Errorf("the broken steward was given a run: %+v", runs)
	}
}

func TestWorkNoConversationCarriedBetweenRuns(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("fixer", map[string]any{"job": "look at the socket, secret word PLOVER"})
		}
		return testkit.Say("mended")
	})
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.started(n, 1)
	b.settled(n, requests.Answered)
	b.say(n, "kit", "thank you")
	run := b.started(n, 2)
	b.settled(n, requests.Answered)
	in := b.input(run.Name)
	for _, key := range []string{"said", "context", "request"} {
		if strings.Contains(fmt.Sprint(in[key]), "PLOVER") {
			t.Errorf("the second run's %s holds the first run's conversation", key)
		}
	}
	for _, turn := range b.model.Turns() {
		if strings.Contains(turn.Prompt(), "thank you") && len(turn.Messages) > 1 && turn.Number == 1 {
			t.Errorf("the second run began with a conversation: %v", turn.Messages)
		}
	}
}

func TestWorkRunsInARowResetByAPerson(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	// Something no person says starts the steward again after each run:
	// the runs come in a row.
	again := func(i int) {
		if _, err := b.reqs.Write(n, requests.Writing{Kind: requests.KindSaid, By: b.reqs.Rule("small-orders.js", "office"),
			To: []requests.Who{b.reqs.Worker("steward")}, Text: fmt.Sprint("again ", i), Working: true}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= RunsInARow; i++ {
		b.started(n, i)
		b.settled(n, requests.Answered)
		again(i)
	}
	b.until("the request is stopped at the limit", func() bool {
		r, err := b.reqs.Get(n)
		return err == nil && r.Stopped
	})
	if runs := b.runs(n); len(runs) != RunsInARow {
		t.Fatalf("the request was given %d runs", len(runs))
	}
	if !slices.ContainsFunc(b.entries(n, requests.KindBulldog), func(e requests.Entry) bool {
		return e.Read == "office" && strings.Contains(e.Text, "20 runs in a row")
	}) {
		t.Errorf("the house is not told why: %v", b.thread(n))
	}
	b.say(n, "kit", "try once more")
	if run := b.started(n, RunsInARow+1); b.input(run.Name)["said"] != "try once more" {
		t.Errorf("the run after Kit spoke was given %q", b.input(run.Name)["said"])
	}
	b.settled(n, requests.Answered)
}
