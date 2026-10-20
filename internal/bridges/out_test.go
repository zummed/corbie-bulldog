package bridges_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestBridgesOutRunIsGivenWhatToCarry(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "customer 4411 was charged twice"))
	asked := d.thread(1)
	d.answer(1, "bot", "Duplicate confirmed; £40 refunded.")
	out := d.carry()
	if len(out) != 1 {
		t.Fatalf("the out runs are %v (thread %v)", out, asked)
	}
	s := out[0]
	if s.Bridge != "slack" || s.Kind != bridges.KindOut || s.Input["to"] != "C1/1.0" || textIn(s) != "Duplicate confirmed; £40 refunded." ||
		s.Input["direct"] != false || s.Input["ref"] != nil || s.Input["choices"] != nil {
		t.Errorf("the out run is given %#v", s.Input)
	}
	if !strings.HasPrefix(s.Input["answers"].(string), "m") {
		t.Errorf("the out run answers %v, not the reference of Dana's message", s.Input["answers"])
	}
	if s.Labels["kind"] != bridges.KindOut || s.Labels["team"] != office.Top || s.Labels["request"] != "1" || s.Labels["bridge"] != "slack" {
		t.Errorf("the out run is labelled %v", s.Labels)
	}
	if _, ok := s.Input["settings"].(map[string]any); !ok {
		t.Errorf("the out run is given the settings %#v", s.Input["settings"])
	}
}

func TestBridgesSentReferenceKept(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	d.hear("telegram", said("4472", "direct", "4472", "the socket sparked"))
	_, err := d.requests.Put(1, d.requests.Worker("steward"), requests.KindQuestion, d.requests.Person("sam"), "may I cut the kitchen circuit?", nil)
	must(t, err)
	out := d.carry()
	if len(out) != 1 || out[0].Input["to"] != "4471" {
		t.Fatalf("the out runs are %v", out)
	}
	d.ended(out[0], map[string]any{"sent": "s9"})
	d.hear("telegram", said("4471", "direct", "4471", "yes, cut it", "answers", "s9"))
	if got := d.thread(1); got[len(got)-1] != "replied: yes, cut it" {
		t.Errorf("the request holds %v", got)
	}
	d.answer(1, "steward", "Cut, and a socket is ordered.")
	out = d.carry()
	if len(out) != 1 || out[0].Input["to"] != "4472" {
		t.Fatalf("the out runs are %v", out)
	}
	d.ended(out[0], map[string]any{"refused": "the chat is gone"})
	cs, _ := d.Carried(1)
	last := cs[len(cs)-1]
	if last.State != bridges.GivenUp || last.Why != "the chat is gone" {
		t.Errorf("the refused thing is %#v", last)
	}
	// Sent as two messages, either answers to it.
	d.answer(1, "steward", "The socket is fitted.")
	out = d.carry()
	if len(out) != 1 {
		t.Fatalf("the out runs are %v", out)
	}
	d.ended(out[0], map[string]any{"sent": []any{"s10", "s11"}})
	if cs, _ := d.Carried(1); cs[len(cs)-1].State != bridges.Sent {
		t.Errorf("the thing sent as two messages is %#v", cs[len(cs)-1])
	}
	d.hear("telegram", said("4472", "direct", "4472", "thank you", "answers", "s10", "threadless", true))
	if got := d.thread(1); got[len(got)-1] != "said: thank you" || d.latest() != 1 {
		t.Errorf("the request holds %v, of %d requests", got, d.latest())
	}
}

func TestBridgesThingsGoWhereTheyAreFor(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	_, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("dana"), "which card?", nil)
	must(t, err)
	sub, err := d.requests.Sub(n, d.requests.Worker("bot"), "payments", "refund one charge", "ticket-1")
	must(t, err)
	d.decide(sub.Number, "Refund one charge", "lee", "priya")
	d.answer(sub.Number, "ledger", "Duplicate confirmed; £40 refunded.")
	got := texts(d.drain())
	want := []string{"slack:C1/1.0: which card?", "slack:U0LEE000: Refund one charge", "slack:U0PRI000: Refund one charge"}
	if !slices.Equal(got, want) {
		t.Errorf("what is carried is\n%s", strings.Join(got, "\n"))
	}
	if got := d.carried(sub.Number); !slices.Equal(got, []string{"chat:lee sent", "chat:priya sent", "slack:U0LEE000 sent", "slack:U0PRI000 sent"}) {
		t.Errorf("of payments' request, what is carried is %v", got)
	}
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("of Dana's request, what is carried is %v", got)
	}
	for _, person := range []string{"lee", "priya"} {
		if waits, _ := d.requests.WaitingOn(person); len(waits) != 1 {
			t.Errorf("what waits on %s on the screens is %v", person, waits)
		}
	}
}

func TestBridgesNoDirectPlaceCarriesToNoPerson(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", "lee:   { name: Lee, title: Payments., slack: U0LEE000 }", `lee:   { name: Lee, title: Payments., gitlab: "2001" }`)
	d := open(t, files)
	n := d.ask("C1/1.0", "refund one")
	sub, err := d.requests.Sub(n, d.requests.Worker("bot"), "payments", "refund one charge", "ticket-1")
	must(t, err)
	d.decide(sub.Number, "Refund one charge", "lee")
	if got := d.carry(); len(got) != 0 {
		t.Errorf("things are carried to %v", places(got))
	}
	if waits, _ := d.requests.WaitingOn("lee"); len(waits) != 1 {
		t.Errorf("what waits on Lee on the screens is %v", waits)
	}
}

func TestBridgesCarriedOnlyIfSeenAtSending(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	d.hear("telegram", said("4471", "direct", "4471", "the socket sparked"))
	_, err := d.requests.Put(1, d.requests.Worker("steward"), requests.KindTold, d.requests.Person("kit"), "the circuit is off", nil)
	must(t, err)
	first := d.carry()
	if len(first) != 1 || first[0].Input["to"] != "4472" {
		t.Fatalf("the out runs are %v", places(first))
	}
	// The second waits behind the first; by the time it would go, Kit is
	// no longer in the house.
	_, err = d.requests.Put(1, d.requests.Worker("steward"), requests.KindTold, d.requests.Person("kit"), "the socket is ordered", nil)
	must(t, err)
	if got := d.carry(); len(got) != 0 {
		t.Fatalf("things are carried to %v", places(got))
	}
	d.read(replace(t, replace(t, d.files, "office.yaml", "people: [sam, kit]", "people: [sam]"), "office.yaml", "  kit: { name: Kit, title: Lives here., telegram: \"4472\" }\n", ""))
	d.ended(first[0], map[string]any{"sent": "s1"})
	if got := d.carry(); len(got) != 0 {
		t.Errorf("things are carried to %v", places(got))
	}
	if got := d.carried(1); !slices.Equal(got, []string{"telegram:4472 sent", "telegram:4472 withdrawn"}) {
		t.Errorf("what was to be carried is %v", got)
	}
}

func TestBridgesNothingEchoed(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      \"#support\": { team: support }")
	d := open(t, files)
	n := d.ask("C1/1.0", "my own words")
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "more of my own words"))
	if got := d.carry(); len(got) != 0 {
		t.Errorf("Dana's words are carried to %v", places(got))
	}
	// Dana is support's lead: her answer, said in the thread
	// it answers, is not carried back there.
	d.delivered("slack", said("U0RAJ000", "#support", "C2/1.0", "who looks after refunds?"))
	support := d.latest()
	if support == n {
		t.Fatal("Raj's question made no request")
	}
	d.delivered("slack", said("U0DANA00", "#support", "C2/1.0", "payments do"))
	if got := d.thread(support); got[len(got)-1] != "answer: payments do" {
		t.Fatalf("support's request holds %v", got)
	}
	if got := d.carry(); len(got) != 0 {
		t.Errorf("Dana's answer is carried to %v", places(got))
	}
}

func TestBridgesQuestionCarriedOnce(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	d.hear("telegram", said("4472", "direct", "4472", "the socket sparked"))
	// The steward's own line leading to its question, and the fixer's
	// leading to the cut it asks a yes for, are its team's; the question
	// and the decision are each carried as themselves, once.
	lead := func(worker, text string) {
		_, err := d.requests.Write(1, requests.Writing{Kind: "said", By: d.requests.Worker(worker), To: []requests.Who{d.requests.Team(office.Top)}, Text: text, Read: true})
		must(t, err)
	}
	lead("steward", "I will ask Sam before I cut anything.")
	_, err := d.requests.Put(1, d.requests.Worker("steward"), requests.KindQuestion, d.requests.Person("sam"), "may I cut it?", nil)
	must(t, err)
	lead("fixer", "Cutting the kitchen circuit needs a yes.")
	d.decide(1, "Cut the kitchen circuit", "sam")
	out := d.drain()
	if got := texts(out); !slices.Equal(got, []string{"telegram:4471: Cut the kitchen circuit", "telegram:4471: may I cut it?"}) {
		t.Errorf("Sam is carried\n%s", strings.Join(got, "\n"))
	}
	if textIn(out[0]) != "may I cut it?" {
		t.Errorf("the decision is carried before the question")
	}
	if got := d.carried(1); !slices.Equal(got, []string{"chat:sam sent", "telegram:4471 sent", "chat:sam sent", "telegram:4471 sent"}) {
		t.Errorf("what is carried is %v", got)
	}
}

func TestBridgesInnerTalkNeverCarried(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	sub, err := d.requests.Sub(n, d.requests.Worker("bot"), "payments", "refund one charge", "ticket-1")
	must(t, err)
	if _, err := d.requests.Write(sub.Number, requests.Writing{Kind: "said", By: d.requests.Worker("ledger"), To: []requests.Who{d.requests.Team("payments")}, Text: "the ledger lines", Read: true}); err != nil {
		t.Fatal(err)
	}
	d.answer(sub.Number, "ledger", "refunded")
	if got := d.carry(); len(got) != 0 {
		t.Errorf("payments' inner talk is carried to %v", places(got))
	}
}

func TestBridgesPlaceFixedWithRequest(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	first := d.ask("C1/1.0", "first thread")
	d.ask("C1/2.0", "second thread")
	d.delivered("slack", said("U0DANA00", "#help", "C1/2.0", "say it here"))
	d.answer(first, "bot", "answered")
	if got := places(d.carry()); !slices.Equal(got, []string{"slack:C1/1.0"}) {
		t.Errorf("the first request's answer is carried to %v", got)
	}
	_, err := d.requests.Put(first, d.requests.Worker("bot"), requests.KindTold, d.requests.Person("al"), "for you", nil)
	must(t, err)
	if got := places(d.carry()); !slices.Equal(got, []string{"slack:U0AL0000"}) {
		t.Errorf("what is told to Al is carried to %v", got)
	}
}

func TestBridgesSignInLink(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	thing, err := d.SignIn("telegram", "sam", "https://house.example/signin/abc")
	must(t, err)
	out := d.carry()
	if len(out) != 1 || out[0].Thing != thing || out[0].Input["to"] != "4471" || out[0].Input["direct"] != true ||
		!strings.Contains(textIn(out[0]), "/signin/") || out[0].Labels["kind"] != bridges.KindSignIn {
		t.Fatalf("the link is carried by %#v", out)
	}
	if d.latest() != 0 {
		t.Error("the link is an entry of a request")
	}
	d.ended(out[0], map[string]any{"refused": "the bot was blocked"})
	if again := d.carry(); len(again) != 0 {
		t.Errorf("the link is carried again: %v", again)
	}
	if len(d.signedIn) != 1 || d.signedIn[0].Thing != thing || d.signedIn[0].Sent || d.signedIn[0].Refused != "the bot was blocked" {
		t.Errorf("the server is told %#v", d.signedIn)
	}
	// Nor is one that was sent, whose text is kept only until its run is
	// given it.
	sent, err := d.SignIn("telegram", "sam", "https://house.example/signin/sent")
	must(t, err)
	before, _, err := d.Carry()
	must(t, err)
	if text := d.textOf(sent); len(before) != 1 || !strings.Contains(text, "/signin/sent") {
		t.Fatalf("the link to be carried is kept as %q", text)
	}
	// Its one run given it, it is kept no longer.
	must(t, d.Started(before[0], "run-sent"))
	if text := d.textOf(sent); text != "" || !strings.Contains(textIn(before[0]), "/signin/sent") {
		t.Errorf("the link given to its run is kept as %q", text)
	}
	must(t, d.OutEnded(sent, "run-sent", bridges.End{Output: map[string]any{"sent": "m1"}}))
	if again := d.carry(); len(again) != 0 || len(d.signedIn) != 2 || !d.signedIn[1].Sent {
		t.Errorf("the link sent: %v, told %#v", again, d.signedIn)
	}
	d.signedIn = d.signedIn[:1]
	// One whose run failed is not carried again either.
	second, err := d.SignIn("telegram", "kit", "https://house.example/signin/def")
	must(t, err)
	out = d.carry()
	d.failed(out[0], "the job failed")
	if again := d.carry(); len(again) != 0 || len(d.signedIn) != 2 || d.signedIn[1].Thing != second || d.signedIn[1].Failed != "the job failed" {
		t.Errorf("the second link: %v, told %#v", again, d.signedIn)
	}
	// One given up with no run, its bridge stopped or gone before it was
	// carried, is told of too.
	third, err := d.SignIn("telegram", "sam", "https://house.example/signin/ghi")
	must(t, err)
	d.read(replace(t, d.files, "bridges/telegram/in.yaml", "input: { settings: any, cursor: any }", "input: { settings: any }"))
	if got := d.carry(); len(got) != 0 || len(d.signedIn) != 3 || d.signedIn[2].Thing != third || !strings.Contains(d.signedIn[2].Failed, "stopped") {
		t.Errorf("with the bridge stopped, the third link: %v, told %#v", places(got), d.signedIn)
	}
	d.read(house(t))
	fourth, err := d.SignIn("telegram", "sam", "https://house.example/signin/jkl")
	must(t, err)
	gone := replace(t, d.files, "office.yaml", "bridges:\n"+houseListing, "")
	gone = replace(t, replace(t, gone, "office.yaml", `, telegram: "4471"`, ""), "office.yaml", `, telegram: "4472"`, "")
	d.read(without(gone, "bridges/telegram"))
	if len(d.signedIn) != 4 || d.signedIn[3].Thing != fourth || !strings.Contains(d.signedIn[3].Failed, "not in the office") {
		t.Errorf("with the bridge gone, the fourth link: told %#v", d.signedIn)
	}
	var text string
	must(t, d.db.QueryRow(`SELECT group_concat(text, '') FROM bridges_things`).Scan(&text))
	if strings.Contains(text, "/signin/") {
		t.Errorf("a link's text is kept after its carrying: %q", text)
	}
}

// textOf gives the text a thing keeps to be carried.
func (d *desk) textOf(thing int64) string {
	d.t.Helper()
	var text string
	must(d.t, d.db.QueryRow(`SELECT text FROM bridges_things WHERE id = ?`, thing).Scan(&text))
	return text
}

// houseListing is how the house lists its Telegram bridge.
const houseListing = "  telegram:\n    places:\n      direct: {}                   # as Bulldog ships it: Sam and Kit may ask the house in a direct message\n"

func TestBridgesSignedInMayAskForAnother(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	// The server, told a link was refused, asks for one on another bridge
	// from where it is told.
	var again error
	d.SignedIn = func(s bridges.SignIn) {
		_, again = d.SignIn("telegram", "kit", "https://house.example/signin/next")
	}
	_, err := d.SignIn("telegram", "sam", "https://house.example/signin/abc")
	must(t, err)
	out := d.carry()
	d.ended(out[0], map[string]any{"refused": "the bot was blocked"})
	must(t, again)
	if got := places(d.carry()); !slices.Equal(got, []string{"telegram:4472"}) {
		t.Errorf("the next link is carried to %v", got)
	}
}

func TestBridgesSignInLinkOnlyDirect(t *testing.T) {
	t.Parallel()
	d := open(t, replace(t, work(t), "office.yaml", `al:    { name: Al, title: Runs the office., slack: U0AL0000, gitlab: "1041" }`, `al:    { name: Al, title: Runs the office., gitlab: "1041" }`))
	if _, err := d.SignIn("gitlab", "al", "https://work.example/signin/abc"); err == nil {
		t.Error("the GitLab bridge, with no direct place, takes a sign-in link")
	}
	if _, err := d.SignIn("slack", "al", "https://work.example/signin/abc"); err == nil {
		t.Error("a sign-in link goes to Slack, which does not know Al")
	}
	if got := d.carry(); len(got) != 0 {
		t.Errorf("things are carried to %v", places(got))
	}
}

func TestBridgesTicketNeverSentOut(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	sub, err := d.requests.Sub(n, d.requests.Worker("bot"), "payments", "refund one charge", "corbie-ticket-0f3a9")
	must(t, err)
	d.decide(sub.Number, "Refund one charge", "lee")
	_, err = d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("dana"), "which card?", nil)
	must(t, err)
	out := d.carry()
	if len(out) != 2 {
		t.Fatalf("the out runs are %v", places(out))
	}
	for _, s := range out {
		input, _ := json.Marshal(s.Input)
		labels, _ := json.Marshal(s.Labels)
		if strings.Contains(string(input)+textIn(s)+string(labels), "corbie-ticket-0f3a9") {
			t.Errorf("an out run is given the ticket: %s %s", input, labels)
		}
		if r, _ := s.Input["ref"].(string); !strings.HasPrefix(r, "r") {
			t.Errorf("an out run is given the reference %v", s.Input["ref"])
		}
	}
}

func TestBridgesDeliveredOnlyWhenSent(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.answer(d.ask("C1/1.0", "hello"), "bot", "hi")
	out := d.carry()
	if got := d.carried(1); !slices.Equal(got, []string{"slack:C1/1.0 sending"}) {
		t.Errorf("while its run goes, the answer is %v", got)
	}
	d.later(10 * time.Second)
	if again := d.carry(); len(again) != 0 {
		t.Errorf("the answer is carried by a second run: %v", again)
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	if got := d.carried(1); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("once its run ends, the answer is %v", got)
	}
}

func TestBridgesGivenUp(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.answer(d.ask("C1/1.0", "one"), "bot", "refused")
	d.answer(d.ask("C1/2.0", "two"), "bot", "failing")
	for _, s := range d.carry() {
		if s.Input["to"] == "C1/1.0" {
			d.ended(s, map[string]any{"refused": "the channel is archived"})
		} else {
			d.failed(s, "the job failed: 502")
		}
	}
	if got := d.carried(1); !slices.Equal(got, []string{"slack:C1/1.0 given up"}) {
		t.Errorf("the refused thing is %v", got)
	}
	var pauses []time.Duration
	runs := 1
	for {
		out, next, err := d.Carry()
		must(t, err)
		if len(out) > 0 {
			t.Fatal("a thing is carried again before its pause is over")
		}
		if next.IsZero() {
			break
		}
		pauses = append(pauses, next.Sub(d.clock.Now()))
		d.clock.Set(next)
		out = d.carry()
		if len(out) != 1 {
			t.Fatalf("the failing thing is not carried again: %v", out)
		}
		runs++
		d.failed(out[0], "the job failed: 502")
	}
	if runs != bridges.MaxRuns || !slices.IsSorted(pauses) || pauses[0] == pauses[len(pauses)-1] {
		t.Errorf("the failing thing had %d runs, paused %v", runs, pauses)
	}
	cs, _ := d.Carried(2)
	if len(cs) != 1 || cs[0].State != bridges.GivenUp || cs[0].Why != "the job failed: 502" || cs[0].Runs != bridges.MaxRuns {
		t.Errorf("the failing thing is %#v", cs)
	}
}

func TestBridgesInOrderOneAtATime(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "three answers")
	for _, text := range []string{"first", "second", "third"} {
		_, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindTold, d.requests.Person("dana"), text, nil)
		must(t, err)
	}
	var sent []string
	for i := 0; i < 20; i++ {
		out := d.carry()
		if len(out) > 1 {
			t.Fatalf("two things go to one place at once: %v", out)
		}
		if len(out) == 1 {
			if textIn(out[0]) == "second" {
				d.failed(out[0], "the job failed")
			} else {
				d.ended(out[0], map[string]any{"sent": "s"})
				sent = append(sent, textIn(out[0]))
			}
		}
		d.later(time.Minute)
	}
	if !slices.Equal(sent, []string{"first", "third"}) {
		t.Errorf("what was sent is %v", sent)
	}
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 sent", "slack:C1/1.0 given up", "slack:C1/1.0 sent"}) {
		t.Errorf("what was carried is %v", got)
	}
}

func TestBridgesOutRunReadAfterRestart(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.answer(d.ask("C1/1.0", "hello"), "bot", "hi")
	out := d.carry()
	d.restart()
	if again := d.carry(); len(again) != 0 {
		t.Errorf("the answer is carried again after a restart: %v", again)
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	if got := d.carried(1); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("the answer is %v", got)
	}
	if again := d.carry(); len(again) != 0 {
		t.Errorf("the answer is carried again: %v", again)
	}
}

func TestBridgesOutEndingTakenOnce(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.answer(d.ask("C1/1.0", "one"), "bot", "first")
	d.answer(d.ask("C1/2.0", "two"), "bot", "second")
	out := d.carry()
	if len(out) != 2 {
		t.Fatalf("the out runs are %v", out)
	}
	// Each ending is handed back twice, as when Bulldog stopped before it
	// kept that it took one: the second changes nothing.
	for range 2 {
		d.ended(to(out, "slack", "C1/1.0")[0], map[string]any{"sent": "s1"})
		d.failed(to(out, "slack", "C1/2.0")[0], "the job failed: 502")
	}
	if got := d.carried(1); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("the sent thing is %v", got)
	}
	cs, _ := d.Carried(2)
	if len(cs) != 1 || cs[0].State != bridges.Waiting || cs[0].Runs != 1 {
		t.Errorf("the failed thing is %#v", cs)
	}
	// A run that never carried the thing is refused still.
	if err := d.OutEnded(out[0].Thing, "another", bridges.End{Output: map[string]any{"sent": "s2"}}); err == nil {
		t.Error("the ending of a run that never carried the thing was taken")
	}
}

func TestBridgesNewlyReachedStartFromNow(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", "lee:   { name: Lee, title: Payments., slack: U0LEE000 }", "lee:   { name: Lee, title: Payments. }")
	d := open(t, files)
	n := d.ask("C1/1.0", "refund one")
	sub, err := d.requests.Sub(n, d.requests.Worker("bot"), "payments", "refund one charge", "ticket-1")
	must(t, err)
	for i := 0; i < 3; i++ {
		_, err := d.requests.Put(sub.Number, d.requests.Worker("ledger"), requests.KindTold, d.requests.Person("lee"), "an answer given before", nil)
		must(t, err)
	}
	d.decide(sub.Number, "Refund one charge", "lee")
	// Nothing is carried before the new reading, so what was written before
	// it is first looked at after it.
	d.read(replace(t, d.files, "office.yaml", "lee:   { name: Lee, title: Payments. }", "lee:   { name: Lee, title: Payments., slack: U0LEE000 }"))
	if got := texts(d.drain()); !slices.Equal(got, []string{"slack:U0LEE000: Refund one charge"}) {
		t.Errorf("Lee is carried %v", got)
	}
	// A bridge newly listed starts from now too: in a house whose Telegram
	// bridge is not yet listed, Kit is told something and asked something;
	// once it is listed, only the question is carried.
	h := open(t, replace(t, house(t), "office.yaml", "bridges:\n"+houseListing, ""))
	r, err := h.requests.AskInChat("sam", "", "the socket sparked", nil)
	must(t, err)
	_, err = h.requests.Put(r.Number, h.requests.Worker("steward"), requests.KindTold, h.requests.Person("kit"), "the circuit is off", nil)
	must(t, err)
	_, err = h.requests.Put(r.Number, h.requests.Worker("steward"), requests.KindQuestion, h.requests.Person("kit"), "is the fuse box under the stairs?", nil)
	must(t, err)
	h.read(house(t))
	if got := texts(h.drain()); !slices.Equal(got, []string{"telegram:4472: is the fuse box under the stairs?"}) {
		t.Errorf("once the bridge is listed, Kit is carried %v", got)
	}
}

func TestBridgesStoppedBridgeCarriesWhatWaitsOnceMended(t *testing.T) {
	t.Parallel()
	broken := `"#help": { team: billing }`
	d := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, broken))
	r, err := d.requests.AskInChat("al", "", "refund one", nil)
	must(t, err)
	d.decide(r.Number, "Refund one charge", "lee")
	if got := d.drain(); len(got) != 0 {
		t.Fatalf("a stopped bridge carries %v", places(got))
	}
	if got := d.carried(r.Number); !slices.Equal(got, []string{"chat:lee sent", "slack:U0LEE000 given up"}) {
		t.Errorf("while the bridge is stopped, the decision is %v", got)
	}
	d.read(work(t))
	if got := texts(d.drain()); !slices.Equal(got, []string{"slack:U0LEE000: Refund one charge"}) {
		t.Errorf("once the bridge is mended, Lee is carried %v", got)
	}
	// A question put to the asker goes to the origin, and so is carried
	// there once the bridge is mended, and never to the asker directly.
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	d.read(replace(t, work(t), "office.yaml", `"#help": { guests: true }`, broken))
	_, err = d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("dana"), "which card?", nil)
	must(t, err)
	if got := d.drain(); len(got) != 0 {
		t.Fatalf("a stopped bridge carries %v", places(got))
	}
	d.read(work(t))
	if got := texts(d.drain()); !slices.Equal(got, []string{"slack:C1/1.0: which card?"}) {
		t.Errorf("once the bridge is mended, Dana is carried %v", got)
	}
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 given up", "slack:C1/1.0 sent"}) {
		t.Errorf("the question to Dana is %v", got)
	}
}

func TestBridgesQuestionToAskerKeptAcrossRestart(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	_, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("dana"), "which card?", nil)
	must(t, err)
	if got := texts(d.drain()); !slices.Equal(got, []string{"slack:C1/1.0: which card?"}) {
		t.Fatalf("Dana is carried %v", got)
	}
	d.restart()
	if got := d.drain(); len(got) != 0 {
		t.Errorf("after a restart, things are carried to %v", places(got))
	}
	if got := d.carried(n); !slices.Equal(got, []string{"slack:C1/1.0 sent"}) {
		t.Errorf("after a restart, the question to Dana is %v", got)
	}
}

func TestBridgesOutRunGivenBlobs(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "send me the ledger")
	_, err := d.requests.Answer(n, d.requests.Worker("bot"), "Here it is.", []requests.File{{Name: "ledger.csv", Data: []byte("a,b")}, {Name: "note.txt", Data: []byte("hi")}})
	must(t, err)
	out := d.carry()
	if len(out) != 1 {
		t.Fatalf("the out runs are %v", places(out))
	}
	s := out[0]
	if text := textIn(s); text != "Here it is." {
		t.Errorf("the text is given as %q", text)
	}
	var files []string
	for _, f := range s.Input["files"].([]any) {
		f := f.(map[string]any)
		files = append(files, f["name"].(string)+": "+blobOf(f["data"]))
	}
	if !slices.Equal(files, []string{"ledger.csv: a,b", "note.txt: hi"}) {
		t.Errorf("the files are given as %q", files)
	}
}

func TestBridgesScanLosesNothing(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	one := d.ask("C1/1.0", "one")
	two := d.ask("C1/2.0", "two")
	d.drain()
	d.answer(one, "bot", "first")
	d.answer(two, "bot", "second")
	// The store fails while what is to be carried is looked at.
	_, err := d.db.Exec(`ALTER TABLE bridges_seen RENAME TO bridges_seen_away`)
	must(t, err)
	if _, _, err := d.Carry(); err == nil {
		t.Fatal("Carry did not fail")
	}
	_, err = d.db.Exec(`ALTER TABLE bridges_seen_away RENAME TO bridges_seen`)
	must(t, err)
	if got := places(d.carry()); !slices.Equal(got, []string{"slack:C1/1.0", "slack:C1/2.0"}) {
		t.Errorf("after the store failed, the answers are carried to %v", got)
	}
}

func TestBridgesBadThingBlocksNothing(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "hello")
	for _, text := range []string{"bad", "good"} {
		_, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindTold, d.requests.Person("al"), text, nil)
		must(t, err)
	}
	for i := 0; i < 10; i++ {
		for _, s := range d.carry() {
			if textIn(s) == "bad" {
				d.ended(s, map[string]any{"refused": "not_in_channel"})
			} else {
				d.ended(s, map[string]any{"sent": "s"})
			}
		}
	}
	if got := d.carried(n); !slices.Equal(got, []string{"chat:al sent", "slack:U0AL0000 given up", "chat:al sent", "slack:U0AL0000 sent"}) {
		t.Errorf("Al's things are %v", got)
	}
}

func TestBridgesSlowPlaceHoldsNoOther(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	one := d.ask("C1/1.0", "one")
	d.answer(one, "bot", "slow")
	out := d.carry()
	if len(out) != 1 {
		t.Fatalf("the out runs are %v", out)
	}
	// While the run to the first thread goes, another thread, a person and
	// what is heard go on.
	two := d.ask("C1/2.0", "two")
	d.answer(two, "bot", "quick")
	_, err := d.requests.Put(two, d.requests.Worker("bot"), requests.KindTold, d.requests.Person("al"), "for Al", nil)
	must(t, err)
	if got := places(d.carry()); !slices.Equal(got, []string{"slack:C1/2.0", "slack:U0AL0000"}) {
		t.Errorf("while one place is slow, things are carried to %v", got)
	}
	if listening := d.listen(); len(listening) != 1 || listening[0].Bridge != "gitlab" {
		t.Errorf("while carrying waits, the listening runs are %v", listening)
	}
}

func TestBridgesTakenBeforeCarried(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	taken := d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "my words"))
	if len(taken.Stopped) != 0 || len(d.thread(1)) != 1 {
		t.Fatalf("Dana's words are taken as %v", d.thread(1))
	}
	if got := d.carry(); len(got) != 0 {
		t.Errorf("Dana's words are carried to %v", places(got))
	}
}

func TestBridgesChatHasNoRuns(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	r, err := d.requests.AskInChat("kit", "", "the kitchen socket sparked", nil)
	must(t, err)
	if r.Origin.Kind != requests.InChat {
		t.Errorf("the request's origin is %#v", r.Origin)
	}
	if asks, _ := d.requests.Asks(r.Number); asks.Name != "kit" {
		t.Errorf("the request is asked by %#v", asks)
	}
	answer := d.answer(r.Number, "steward", "The circuit is off.")
	v, err := d.requests.View(r.Number, "kit")
	must(t, err)
	if !slices.ContainsFunc(v.Entries, func(e requests.Entry) bool { return e.ID == answer.ID }) {
		t.Error("the steward's answer is not on Kit's screens")
	}
	if got := d.carry(); len(got) != 0 {
		t.Errorf("the chat's answer is carried to %v", places(got))
	}
	cs, err := d.Carried(r.Number)
	must(t, err)
	if len(cs) != 1 || cs[0].Entry != answer.ID || cs[0].Bridge != bridges.Chat || cs[0].State != bridges.Sent || cs[0].Thing != 0 {
		t.Errorf("the chat's answer is carried as %#v", cs)
	}
	if starts := d.listen(); len(starts) != 1 || starts[0].Bridge != "telegram" {
		t.Errorf("the listening runs are %v", starts)
	}
}
