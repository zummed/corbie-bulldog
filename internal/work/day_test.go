package work

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// The two days of the README, driven through the areas as the server
// drives them, with a stand-in model and stand-in bridges and services.

// told keeps what each worker was last told, turn by turn, as the
// stand-in is shown it.
type told struct {
	mu   sync.Mutex
	last map[string]string
}

func (k *told) keep(worker string, turn testkit.Turn) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.last == nil {
		k.last = map[string]string{}
	}
	k.last[fmt.Sprintf("%s %d", worker, turn.Number)] = turn.Last()
}

func (k *told) get(worker string, number int) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.last[fmt.Sprintf("%s %d", worker, number)]
}

// deliver ends each out run that carries to a place on a bridge as sent,
// one after another as they come, until one carries words that hold
// text, and gives its input.
func (b *bench) deliver(bridge, to, text string) map[string]any {
	b.t.Helper()
	for sent := 1; ; sent++ {
		run, in := b.carrying(bridge, to)
		b.carried(run, map[string]any{"sent": fmt.Sprintf("%s-%d", run, sent)})
		if s, _ := in["text"].(string); strings.Contains(s, text) {
			return in
		}
		b.until("the out run "+run+" is taken", func() bool { return b.ended(run) })
	}
}

// ended says whether a run noted has ended.
func (b *bench) ended(run string) bool {
	n, err := b.w.noted(run)
	if err != nil {
		b.t.Fatal(err)
	}
	return n != nil && n.ended()
}

// decided stays until a request's decision of a job has ended, and gives
// it.
func (b *bench) decided(n int64, job string) *decisions.Decision {
	b.t.Helper()
	var dec *decisions.Decision
	b.until(fmt.Sprintf("the decision on %s in request %d has ended", job, n), func() bool {
		for _, d := range must(b.d.OfRequest(n)) {
			if d.Job != nil && d.Job.Name == job && d.Ended != "" {
				dec = d
			}
		}
		return dec != nil
	})
	return dec
}

func TestTheHousesDay(t *testing.T) {
	t.Parallel()
	files := house(t)
	// Sparks's own model is the stand-in too.
	files["sparks.yaml"] = strings.Replace(files["sparks.yaml"], "model: { name: haiku, service: claude-code }", "model: usual", 1)
	b := newBench(t, files)
	var k told
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.System, "You look after the house") && strings.Contains(turn.Prompt(), "Walk round"):
			k.keep("routine", turn)
			return testkit.Say("Nothing is open.")
		case strings.Contains(turn.System, "You look after the house"):
			k.keep("steward", turn)
			if turn.Number == 1 {
				return testkit.Use("fixer", map[string]any{"job": "The kitchen socket sparked: mend it."})
			}
			return testkit.Say("The kitchen socket is mended, and circuit 4 is back on.")
		case strings.Contains(turn.System, "You do the repairs"):
			k.keep("fixer", turn)
			switch turn.Number {
			case 1:
				return testkit.Use("sparks", map[string]any{"question": "Which circuit is the kitchen socket on?"})
			case 2:
				return testkit.Use("cut_power", map[string]any{"circuit": "4"})
			case 3:
				return testkit.Use("order_part", map[string]any{"part": "socket", "pounds": 12})
			case 4:
				return testkit.Use("ask_person", map[string]any{"question": "Say when the new socket arrives."})
			}
			return testkit.Say("The new socket is fitted.")
		case strings.Contains(turn.System, "You know the house's wiring"):
			k.keep("sparks", turn)
			if turn.Number == 1 {
				return testkit.Use("circuits", map[string]any{})
			}
			return testkit.Say("The kitchen sockets are on circuit 4.")
		}
		return testkit.Say("done")
	})
	listening := b.listening("telegram")

	// 1. Kit types into the chat, and the steward starts on it.
	n := b.chat("kit", "", "the kitchen socket sparked")
	steward := b.started(n, 1)
	if steward.Worker != "steward" {
		t.Fatalf("the request started %+v", steward)
	}

	// 2 and 3. The fixer asks sparks, a tool, then asks to cut circuit 4,
	// which waits for Sam, asked on Telegram.
	cut := b.held(n)
	if cut.Job.Name != "cut_power" || cut.Whose.Name != "fixer" || strings.Join(cut.Asked, ",") != "sam" {
		t.Fatalf("the held job is %+v", cut)
	}
	holds(t, "what sparks told the fixer", k.get("fixer", 2), "The kitchen sockets are on circuit 4.")
	if fixer, sparks := b.machine(n, "fixer"), b.machine(n, "sparks"); fixer.Parent != steward.Name || sparks.Parent != fixer.ID {
		t.Errorf("the fixer runs below %s and sparks below %s", fixer.Parent, sparks.Parent)
	}
	question := b.deliver("telegram", "4471", "Turn one circuit off")
	if !slices.Equal(question["choices"].([]any), []any{"yes", "refuse"}) || question["direct"] != true {
		t.Fatalf("the decision is carried to Sam as %v", question)
	}
	b.hear(listening, 1, thing("choice", "4471", "direct", "4471", "answers", question["ref"], "choice", "yes"))
	if d := b.decided(n, "cut_power"); d.Ended != decisions.Yes || d.By.Called != "Sam" {
		t.Errorf("the cut is decided %+v", d)
	}
	b.deliver("telegram", "4471", "Taken: your yes")

	// 4. The order is the small-orders rule's to say yes to.
	if d := b.decided(n, "order_part"); d.Ended != decisions.Yes || d.By.Kind != office.IsRule {
		t.Errorf("the order is decided %+v", d)
	}

	// 5. The fixer asks Kit to say when the socket arrives: the request
	// waits on Kit until Thursday.
	ask := b.waitsOn(n, requests.KindQuestion)
	holds(t, "the fixer's results", k.get("fixer", 3)+"\n"+k.get("fixer", 4), "Circuit 4 is off.", "Ordered socket for £12.")
	b.until("the request waits on Kit", func() bool {
		st := must(b.reqs.State(n))
		return st.Is == requests.Waiting && len(st.On) == 1 && st.On[0].Name == "kit"
	})
	b.clock.Set(time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC))
	if st := must(b.reqs.State(n)); st.Is != requests.Waiting {
		t.Errorf("on Thursday the request is %+v", st)
	}
	if _, err := b.reqs.Reply(ask.ID, b.reqs.Person("kit"), "The socket came today.", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}

	// 6. The fixer finishes, and the steward answers Kit.
	answer := b.answered(n)
	if answer.By.Name != "steward" || answer.To[0].Name != "kit" || answer.Text != "The kitchen socket is mended, and circuit 4 is back on." {
		t.Errorf("Kit is answered %+v", answer)
	}
	holds(t, "what Kit said, as the fixer was told", k.get("fixer", 5), "Kit answered: The socket came today.")
	holds(t, "what was done", strings.Join(b.thread(n), "\n"), "Turn one circuit off: approved by Sam", "Order a part: approved by")

	// 7. On Monday at nine the routine asks the steward what is still
	// open; its answer goes to Sam and Kit.
	b.clock.Set(nextMonday)
	walk := b.opened(walkRound, 1)
	routine := b.answered(walk)
	holds(t, "the routine's context", k.get("routine", 1), "Asked by The house itself, by its routine at mon 09:00", "## rooms")
	for _, at := range []string{"4471", "4472"} {
		b.deliver("telegram", at, "Nothing is open.")
	}
	if routine.Text != "Nothing is open." {
		t.Errorf("the routine is answered %q", routine.Text)
	}
}

// delivered has a bridge's address called: its hearing run, started
// through the work, ends with what it heard, and the service is answered.
func (b *bench) delivered(bridge string, things ...map[string]any) *bridges.Answer {
	b.t.Helper()
	start, err := b.b.Hear(bridge, bridges.Delivery{Method: "POST", Body: "{}"})
	if err != nil {
		b.t.Fatal(err)
	}
	answered, err := b.w.Hear(start)
	if err != nil {
		b.t.Fatal(err)
	}
	run := ""
	b.until("the hearing run waits", func() bool {
		for _, r := range b.going(bridges.KindIn, bridge) {
			if slices.Contains(r.Tickets, standin.Heard) {
				run = r.ID
			}
		}
		return run != ""
	})
	b.hear(run, nil, things...)
	select {
	case a := <-answered:
		return a
	case <-time.After(testkit.Patience):
		b.t.Fatalf("the hearing run %s is never taken", run)
		return nil
	}
}

// ledgerService is a stand-in for the ledger's service: a customer's
// charges, card numbers and all, and refunds.
type ledgerService struct {
	*httptest.Server
	mu       sync.Mutex
	refunded []string
}

func newLedger(t *testing.T) *ledgerService {
	l := &ledgerService{}
	l.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/customers/4411/charges":
			json.NewEncoder(rw).Encode([]map[string]any{
				{"id": "ch_2", "pounds": 40, "card": "4111 1111 1111 1111", "on": "Tuesday 14:01"},
				{"id": "ch_1", "pounds": 40, "card": "4111 1111 1111 1111", "on": "Tuesday 14:00"},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/refund"):
			l.mu.Lock()
			l.refunded = append(l.refunded, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/charges/"), "/refund"))
			l.mu.Unlock()
			json.NewEncoder(rw).Encode(map[string]any{"refunded": true, "pounds": 40})
		default:
			http.Error(rw, "not here", http.StatusNotFound)
		}
	}))
	t.Cleanup(l.Close)
	return l
}

// workDay gives the work office with its services stood in for: the
// ledger's, and the screening service for payments' disclosure check; and a rule that
// refers each refund on to Lee or Priya.
func workDay(t *testing.T, ledger *ledgerService, screen *screener) map[string]string {
	files := work(t)
	host := strings.TrimPrefix(ledger.URL, "http://")
	files["payments/ledger.yaml"] = strings.ReplaceAll(files["payments/ledger.yaml"],
		`allow: [net:ledger.example.com, "secret:LEDGER_KEY@net:ledger.example.com"]`,
		fmt.Sprintf(`allow: [net:%s, "secret:%s@net.unencrypted:%s"]`, host, testkit.Key, host))
	files["payments/tools/ledger.js"] = strings.ReplaceAll(strings.Replace(files["payments/tools/ledger.js"],
		`secrets.get("LEDGER_KEY")`, fmt.Sprintf(`secrets.get(%q)`, testkit.Key), 1), "https://ledger.example.com", ledger.URL)
	screenHost := strings.TrimPrefix(screen.URL, "http://")
	files["payments/no-account-numbers.yaml"] = strings.Replace(files["payments/no-account-numbers.yaml"],
		`allow: [net:screen.example.com, "secret:SCREEN_KEY@net:screen.example.com"]`,
		fmt.Sprintf(`allow: [net:%s, "secret:%s@net.unencrypted:%s"]`, screenHost, testkit.Key, screenHost), 1)
	files["payments/no-account-numbers.js"] = strings.Replace(strings.Replace(files["payments/no-account-numbers.js"],
		`secrets.get("SCREEN_KEY")`, fmt.Sprintf(`secrets.get(%q)`, testkit.Key), 1), "https://screen.example.com", screen.URL, 1)
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "decide: { everything: [lee, priya] }",
		"decide: { everything: [lee, priya], refund: [./refunds.js, lee, priya] }", 1)
	files["payments/refunds.js"] = `return { says: "refer", note: "A refund of " + input.action.args.charge + " is for Lee or Priya." }` + "\n"
	return files
}

func TestTheWorkOfficesRefundDay(t *testing.T) {
	t.Parallel()
	ledger, one := newLedger(t), newScreener(t)
	one.leaks = 0.01
	b := newBench(t, workDay(t, ledger, one))
	var k told
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.System, "You answer what is asked in #help") && strings.Contains(turn.Prompt(), "is the API down?"):
			k.keep("bot guest", turn)
			return testkit.Say("Al will look at the API, and say so in this thread.")
		case strings.Contains(turn.System, "You answer what is asked in #help"):
			k.keep("bot", turn)
			if turn.Number == 1 {
				return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "What broke after Tuesday's release?"}).
					And("ask_team", map[string]any{"team": "payments", "question": "Was customer 4411 charged twice? Refund one."})
			}
			return testkit.Say("Merge request 812 broke the webhook retry, and payments refunded the second charge.")
		case strings.Contains(turn.System, "You look after GitLab"):
			k.keep("ops", turn)
			return testkit.Say("Merge request 812 broke the webhook retry.")
		case strings.Contains(turn.System, "You keep the ledger"):
			k.keep("ledger", turn)
			switch turn.Number {
			case 1:
				return testkit.Use("charges", map[string]any{"customer": "4411"})
			case 2:
				reply := testkit.Use("refund", map[string]any{"charge": "ch_2", "why": "charged twice, a minute apart"})
				reply.Text = "ch_2 is the same £40 as ch_1, a minute later: a duplicate."
				return reply
			}
			return testkit.Say("Duplicate confirmed; £40 refunded.")
		}
		return testkit.Say("done")
	})

	// 1. Dana writes in #help; the Slack bridge's in machine hands it over
	// with her identity, and the bot starts on it.
	readers := []any{"U0DANA00", "U0AL0000", "U0RAJ000"}
	if a := b.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.1", "readers", readers,
		"text", "customer 4411 was charged twice after Tuesday's release; refund one and tell me what broke")); a != nil {
		t.Errorf("Slack is answered %+v", a)
	}
	n := b.at("slack", "#help/1700.1")
	if r := must(b.reqs.Get(n)); r.Team != "office" || r.Asker != requests.Identity("slack", "U0DANA00") {
		t.Fatalf("Dana's request is %+v", r)
	}
	b.started(n, 1)

	// 2. The bot asks platform and payments: each a sub-request with a run
	// of its own, told that #help will read its answer.
	var platform, payments int64
	b.until("the bot asks both teams", func() bool {
		for _, s := range must(b.reqs.Tree(n)) {
			switch s.Team {
			case "platform":
				platform = s.Number
			case "payments":
				payments = s.Number
			}
		}
		return platform != 0 && payments != 0
	})
	b.started(platform, 1)
	b.started(payments, 1)

	// 3. Platform's worker answers.
	b.answered(platform)
	holds(t, "what ops was told", k.get("ops", 1), "on slack, in #help/1700.1")

	// 4. The ledger reads the charges and asks to refund the second: the
	// rule refers it on, and Lee and Priya are asked directly on Slack.
	refund := b.held(payments)
	if refund.Job.Name != "refund" || len(refund.Referrals) != 1 || strings.Join(refund.Asked, ",") != "lee,priya" {
		t.Fatalf("the refund waits as %+v", refund)
	}
	toLee := b.deliver("slack", "U0LEE000", "Refund one charge")
	holds(t, "what Lee is shown", toLee["text"].(string), "ch_2", "a duplicate", "is for Lee or Priya")
	b.delivered("slack", thing("choice", "U0LEE000", "direct", "U0LEE000", "answers", toLee["ref"], "choice", "yes"))
	if d := b.decided(payments, "refund"); d.Ended != decisions.Yes || d.By.Called != "Lee" {
		t.Errorf("the refund is decided %+v", d)
	}

	// 5. The ledger answers; the disclosure rule finds no account number, and
	// the answer leaves payments.
	b.answered(payments)
	ledger.mu.Lock()
	if !slices.Equal(ledger.refunded, []string{"ch_2"}) {
		t.Errorf("the ledger refunded %v", ledger.refunded)
	}
	ledger.mu.Unlock()

	// 6. The bot's answer lands in Dana's thread.
	answer := b.answered(n)
	holds(t, "what the bot was told", k.get("bot", 2), "ops answered for Platform", "ledger answered for Payments", "Duplicate confirmed; £40 refunded.")
	b.deliver("slack", "#help/1700.1", answer.Text)

	// 7. Dana and Al, and Raj, who reads #help, see what was done; what was
	// read inside payments only Lee and Priya see.
	for _, person := range []string{"dana", "al", "raj"} {
		v, err := b.reqs.View(n, person)
		if err != nil || !v.Whole || len(v.Subs) != 2 {
			t.Fatalf("%s sees %+v (%v)", person, v, err)
		}
		sub := must(b.reqs.View(payments, person))
		done := ""
		for _, e := range sub.Entries {
			done += e.Text + "\n"
		}
		holds(t, person+"'s view of payments", done, "Refund one charge, by its id: approved by Lee", "Duplicate confirmed; £40 refunded.")
		holdsNone(t, person+"'s view of payments", done, "4111")
		if read := must(b.reqs.WhatWasRead(payments, person)); len(read) != 0 {
			t.Errorf("%s sees what payments read: %v", person, read)
		}
	}
	for _, person := range []string{"lee", "priya"} {
		var read strings.Builder
		for _, r := range must(b.reqs.WhatWasRead(payments, person)) {
			read.Write(r.Events)
		}
		holds(t, person+"'s sight of what payments read", read.String(), "4111 1111 1111 1111")
		if _, err := b.reqs.View(n, person); err == nil {
			t.Errorf("%s sees the request above payments'", person)
		}
		if _, err := b.reqs.View(platform, person); err == nil {
			t.Errorf("%s sees what platform was asked", person)
		}
	}

	// A guest asks in #help: the bot is told it is a guest, and the answer
	// is carried to the guest's thread.
	b.delivered("slack", thing("said", "U0GUEST1", "#help", "#help/1700.5", "text", "is the API down?"))
	guest := b.at("slack", "#help/1700.5")
	b.answered(guest)
	holds(t, "the bot's context for the guest", k.get("bot guest", 1), "Asked by a guest, whom slack vouches for as U0GUEST1")
	b.deliver("slack", "#help/1700.5", "Al will look at the API")
}
