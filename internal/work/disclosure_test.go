package work

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// disclosing waits until a disclosure of a kind waits on people in a
// request, and gives it.
func (b *bench) disclosing(n int64, what string) *decisions.Decision {
	b.t.Helper()
	var dec *decisions.Decision
	b.until(fmt.Sprintf("a %s waits on its disclosure in request %d", what, n), func() bool {
		decs, err := b.d.OfRequest(n)
		if err != nil {
			b.t.Fatal(err)
		}
		for _, d := range decs {
			if d.Kind == decisions.OfDisclosure && d.Said.What == what && d.Ended == "" && len(d.Asked) > 0 {
				dec = d
			}
		}
		return dec != nil
	})
	return dec
}

func TestWhatAToolSaysOutsideItsTeamIsDisclosedFirst(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	var mu sync.Mutex
	var ledger []string
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "what changed for card 4111?"):
			return testkit.Say("Merge request 812 broke the webhook retry.")
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			mu.Lock()
			ledger = append(ledger, turn.Last())
			mu.Unlock()
			switch turn.Number {
			case 1:
				return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "what changed for card 4111?"})
			case 2:
				return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "and card 4111 again?"})
			case 3:
				return testkit.Use("ask_person", map[string]any{"question": "refund it?", "who": "Lee"})
			}
			return testkit.Say("Duplicate confirmed; refunded.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub

	// The question to platform names a card: the rule passes it, and it
	// waits on Lee and Priya, asked of nobody yet.
	first := b.disclosing(sub, decisions.SaysSub)
	if subs := b.entries(sub, requests.KindSub); len(subs) != 0 {
		t.Fatalf("platform was asked before the disclosure: %v", b.thread(sub))
	}
	holdsNone(t, "the bot's request", strings.Join(b.thread(n), "\n"), "4111")
	if r, err := b.d.Decide(ctx, first.ID, b.reqs.Person("lee"), "yes", ""); err != nil || !r.Taken {
		t.Fatalf("Lee's yes: %v %v", r, err)
	}
	asked := b.waitsOn(sub, requests.KindSub)
	if asked.By.Name != "ledger" || asked.Text != "what changed for card 4111?" {
		t.Errorf("platform is asked %+v", asked)
	}

	// Asked again, Priya refuses it: nothing is asked, and the refusal is
	// the tool's result, and an entry to the ledger seen only in payments.
	second := b.disclosing(sub, decisions.SaysSub)
	if r, err := b.d.Decide(ctx, second.ID, b.reqs.Person("priya"), "refuse", "no card numbers"); err != nil || !r.Taken {
		t.Fatalf("Priya's refusal: %v %v", r, err)
	}
	q := b.waitsOn(sub, requests.KindQuestion)
	if q.By.Name != "ledger" || q.To[0].Name != "lee" {
		t.Errorf("the ledger asked %+v", q)
	}
	if _, err := b.reqs.Reply(q.ID, b.reqs.Person("lee"), "yes", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	holds(t, "the bot's answer", b.answered(n).Text, "Duplicate confirmed; refunded.")
	mu.Lock()
	defer mu.Unlock()
	if len(ledger) != 4 {
		t.Fatalf("the ledger's turns were given %q", ledger)
	}
	holds(t, "the ledger's first result", ledger[1], "ops answered for Platform", "Merge request 812")
	holds(t, "the ledger's refusal", ledger[2], "not disclosed", "Priya", "no card numbers")
	var refusal *requests.Entry
	for _, e := range b.entries(sub, decisions.KindDecided) {
		if strings.Contains(e.Text, "no card numbers") {
			refusal = &e
		}
	}
	if refusal == nil || refusal.Read != "payments" || refusal.To[0].Name != "ledger" {
		t.Errorf("Priya's refusal is in the thread as %+v: %v", refusal, b.thread(sub))
	}
	if subs := b.entries(sub, requests.KindSub); len(subs) != 1 {
		t.Errorf("the refused question was asked: %v", b.thread(sub))
	}
	// The question to Lee, inside payments, waited on no disclosure.
	for _, d := range must(b.d.OfRequest(sub)) {
		if d.Said != nil && d.Said.What == decisions.SaysQuestion {
			t.Errorf("the question to Lee was held for disclosure: %+v", d)
		}
	}
	holdsNone(t, "the bot's request", strings.Join(b.thread(n), "\n"), "4111", "no card numbers")
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// screener is a stand-in screening service, answering how likely the text
// it is given names an account number.
type screener struct {
	*httptest.Server
	mu    sync.Mutex
	leaks float64
	sent  []map[string]any
	auth  []string
}

func newScreener(t *testing.T) *screener {
	s := &screener{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var sent map[string]any
		json.Unmarshal(body, &sent)
		s.mu.Lock()
		s.sent, s.auth = append(s.sent, sent), append(s.auth, r.Header.Get("Authorization"))
		leaks := s.leaks
		s.mu.Unlock()
		if r.Method != http.MethodPost || r.URL.Path != "/v1/screen" {
			http.Error(rw, "not here", 404)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]any{"likelihood": leaks})
	}))
	t.Cleanup(s.Close)
	return s
}

func TestARuleNamingItsHostReachesIt(t *testing.T) {
	t.Parallel()
	s := newScreener(t)
	host := strings.TrimPrefix(s.URL, "http://")
	files := work(t)
	files["payments/no-account-numbers.yaml"] = strings.Replace(files["payments/no-account-numbers.yaml"],
		`allow: [net:screen.example.com, "secret:SCREEN_KEY@net:screen.example.com"]`,
		fmt.Sprintf(`allow: [net:%s, "secret:%s@net.unencrypted:%s"]`, host, testkit.Key, host), 1)
	files["payments/no-account-numbers.js"] = strings.Replace(strings.Replace(files["payments/no-account-numbers.js"],
		`secrets.get("SCREEN_KEY")`, fmt.Sprintf(`secrets.get(%q)`, testkit.Key), 1), "https://screen.example.com", s.URL, 1)
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply { return testkit.Say("Duplicate confirmed; £40 refunded.") })

	// Told the answer leaks nothing, the rule says yes and it leaves.
	s.leaks = 0.01
	n := b.chat("al", "", "customer 4411 was charged twice")
	b.answered(n)
	sub, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "payments", "was 4411 charged twice?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	if a := b.answered(sub.Number); a.Text != "Duplicate confirmed; £40 refunded." {
		t.Errorf("payments answered %q", a.Text)
	}
	s.mu.Lock()
	if len(s.sent) != 1 || s.sent[0]["text"] != "Duplicate confirmed; £40 refunded." || s.auth[0] != "Bearer "+testkit.KeyValue {
		t.Errorf("the screening service was sent %v with %v", s.sent, s.auth)
	}
	s.leaks = 0.9
	s.mu.Unlock()

	// Told it may leak, the rule passes with its view, and Lee and Priya
	// are asked, shown it.
	other, err := b.reqs.Sub(n, b.reqs.Worker("bot"), "payments", "and the card?", "ticket-2")
	if err != nil {
		t.Fatal(err)
	}
	dec := b.disclosing(other.Number, decisions.SaysAnswer)
	if len(dec.Passes) != 1 || !strings.Contains(dec.Passes[0].Note, "0.90") || strings.Join(dec.Asked, ",") != "lee,priya" {
		t.Errorf("the disclosure stands as %+v", dec)
	}
	if decide := b.waitsOn(other.Number, decisions.KindDecide); !strings.Contains(decide.Text, "0.90") {
		t.Errorf("Lee and Priya are shown %v", b.thread(other.Number))
	}
}

func TestAnAnswerRefusedToTheLimitIsNotDisclosed(t *testing.T) {
	t.Parallel()
	files := payments(t)
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "disclosure: [./no-cards.js]", "disclosure: [./refuse-all.js]", 1)
	files["payments/refuse-all.js"] = "return { says: \"refuse\", note: \"never the card ending 1111\" }\n"
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			return testkit.Say("Duplicate on the card ending 1111.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	a := b.answered(n)
	if a.Text != "bot heard: payments did not disclose its answer (refuse-all.js)" {
		t.Errorf("the bot was given %q", a.Text)
	}
	if runs := b.runs(sub); len(runs) != RunsInARow {
		t.Errorf("the ledger was started %d times", len(runs))
	}
	holdsNone(t, "the bot's request", strings.Join(b.thread(n), "\n"), "1111")
	if told := b.toldAsker(sub); !slices.Equal(told, []string{"payments did not disclose its answer (refuse-all.js)"}) {
		t.Errorf("Bulldog told the bot %q", told)
	}
}

// toldAsker gives what Bulldog said to a request's asker, in order.
func (b *bench) toldAsker(n int64) []string {
	b.t.Helper()
	r := must(b.reqs.Get(n))
	var told []string
	for _, e := range b.entries(n, requests.KindBulldog) {
		if slices.Contains(e.To, r.Asker) {
			told = append(told, e.Text)
		}
	}
	return told
}

func TestALeadThatGivesUpOnARefusalIsNotDisclosed(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "the last four digits only"):
			return testkit.Say("")
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			return testkit.Say("Duplicate on card 4111 1111 1111 1111.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	dec := b.disclosing(sub, decisions.SaysAnswer)
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("lee"), "refuse", "the last four digits only"); err != nil || !r.Taken {
		t.Fatalf("Lee's refusal: %v %v", r, err)
	}
	if a := b.answered(n); a.Text != "bot heard: payments did not disclose its answer (Lee)" {
		t.Errorf("the bot was given %q", a.Text)
	}
	if runs := b.runs(sub); len(runs) != 2 {
		t.Errorf("the ledger was started %d times", len(runs))
	}
	holdsNone(t, "the bot's request", strings.Join(b.thread(n), "\n"), "4111", "the last four digits only", "gave no answer")
	if told := b.toldAsker(sub); !slices.Equal(told, []string{"payments did not disclose its answer (Lee)"}) {
		t.Errorf("Bulldog told the bot %q", told)
	}
}

func TestAStoppedRunsAnswerStaysInItsTeam(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	gate := make(chan struct{})
	var once sync.Once
	let := func() { once.Do(func() { close(gate) }) }
	defer let()
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			<-gate
			return testkit.Say("Duplicate confirmed on card 4111 1111 1111 1111.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	ledger := b.started(sub, 1)
	// Al stops it, and the cancels do not reach Corbie before the ledger
	// has answered.
	b.reach.fail("cancel", unreachable)
	if err := b.w.Stop(ctx, n, b.reqs.Person("al")); err != nil {
		t.Fatal(err)
	}
	b.until("a cancel was tried", func() bool { return b.reach.count("cancel") > 0 })
	let()
	if ended := b.corbie.Ended(t, ledger.Name); ended.Status != "done" {
		t.Fatalf("the ledger's run ended %s", ended.Status)
	}
	b.until("the ledger's ending is kept", func() bool { return b.runs(sub)[0].Status == done })
	b.still()
	b.reach.fail("cancel", nil)
	b.later(Again)
	b.settled(n, requests.Answered)
	// What the ledger answered is kept inside payments, and nothing of it
	// leaves.
	if !slices.ContainsFunc(b.entries(sub, requests.KindBulldog), func(e requests.Entry) bool {
		return e.Read == "payments" && strings.Contains(e.Text, "4111")
	}) {
		t.Errorf("the ledger's answer is not kept in payments: %v", b.thread(sub))
	}
	if answers := b.entries(sub, requests.KindAnswer); len(answers) != 0 {
		t.Errorf("the stopped run's answer was said: %v", answers)
	}
	for _, r := range []int64{n, sub} {
		v, err := b.reqs.View(r, "dana")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range v.Entries {
			holdsNone(t, fmt.Sprintf("what Dana sees of request %d", r), e.Text, "4111")
		}
	}
	if decs, err := b.d.OfRequest(sub); err != nil || len(decs) != 0 {
		t.Errorf("the stopped answer was handed to the disclosure check: %+v %v", decs, err)
	}
}

func TestARestartOnARefusalIsTriedAgainWhenItsStartFails(t *testing.T) {
	t.Parallel()
	b := newBench(t, payments(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "the last four digits only"):
			return testkit.Say("Duplicate confirmed on the card ending 1111.")
		case strings.Contains(turn.Prompt(), "was 4411 charged twice?"):
			return testkit.Say("Duplicate confirmed on card 4111 1111 1111 1111.")
		case turn.Number == 1:
			return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "was 4411 charged twice?"})
		}
		return testkit.Say("bot heard: " + turn.Last())
	})
	n := b.chat("dana", "", "customer 4411 was charged twice")
	sub := b.waitsOn(n, requests.KindSub).Sub
	dec := b.disclosing(sub, decisions.SaysAnswer)
	var mu sync.Mutex
	failing := true
	b.reach.set(func(f *flaky) {
		f.starting = func(s client.Start) error {
			mu.Lock()
			defer mu.Unlock()
			if in, _ := s.Input.(map[string]any); failing && strings.Contains(fmt.Sprint(in["said"]), "the last four digits only") {
				return unreachable
			}
			return nil
		}
	})
	if r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("lee"), "refuse", "the last four digits only"); err != nil || !r.Taken {
		t.Fatalf("Lee's refusal: %v %v", r, err)
	}
	b.until("payments is told the ledger could not start", func() bool {
		return slices.ContainsFunc(b.entries(sub, requests.KindBulldog), func(e requests.Entry) bool {
			return e.Read == "payments" && strings.Contains(e.Text, "could not start ledger")
		})
	})
	mu.Lock()
	failing = false
	mu.Unlock()
	b.later(Again)
	run := b.started(sub, 3)
	holds(t, "the ledger's run after its start failed", fmt.Sprint(b.input(run.Name)["said"]), "the last four digits only")
	if a := b.answered(n); !strings.Contains(a.Text, "ending 1111") {
		t.Errorf("the bot was given %q", a.Text)
	}
}

func TestALeadWhoIsAPersonAnsweringOnABridgeReachesItsDisclosureCheck(t *testing.T) {
	t.Parallel()
	files := work(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "platform/api: { team: platform }", "platform/api: { team: platform }\n      support/desk: { team: support }", 1)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "slack: U0DANA00 }", `slack: U0DANA00, gitlab: "1300" }`, 1)
	files["support/team.yaml"] += "disclosure: [./kindly.js]\n"
	files["support/kindly.js"] = `return input.text.includes("calm down") ? { says: "refuse", note: "say it kindly" } : { says: "yes" }`
	b := newBench(t, files)
	b.hear(b.listening("gitlab"), 1, thing("said", "1041", "support/desk", "support/desk#1", "text", "a customer is cross: what do we tell them?"))
	n := b.at("gitlab", "support/desk#1")

	// Dana answers in the issue, and the check refuses it: she is waited
	// on again, with the rule's word to her.
	b.hear(b.listening("gitlab"), 2, thing("said", "1300", "support/desk", "support/desk#1", "text", "tell them to calm down"))
	b.until("the check refuses Dana's answer", func() bool {
		return slices.ContainsFunc(b.thread(n), func(line string) bool {
			return strings.HasPrefix(line, decisions.KindDecided+" ") && strings.Contains(line, "to dana:") && strings.Contains(line, "say it kindly")
		})
	})
	b.settled(n, requests.Waiting)
	if st := must(b.reqs.State(n)); len(st.On) != 1 || st.On[0].Name != "dana" {
		t.Errorf("after the refusal the request stands %+v", st)
	}

	// Her next answer the check lets go, and it is the request's answer.
	b.hear(b.listening("gitlab"), 3, thing("said", "1300", "support/desk", "support/desk#1", "text", "we are sorry, and have refunded you"))
	b.settled(n, requests.Answered)
	thread := must(b.reqs.Thread(n))
	if last := thread[len(thread)-1]; last.Kind != requests.KindAnswer || last.By != requests.Identity("gitlab", "1300") || last.Text != "we are sorry, and have refunded you" {
		t.Errorf("the request's thread is %v", b.thread(n))
	}
}
