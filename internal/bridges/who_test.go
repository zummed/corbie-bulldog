package bridges_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestBridgesHeardInNoListedPlace(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.delivered("slack", said("U0DANA00", "#random", "C9/1.0", "in random"))
	d.delivered("slack", said("U0DANA00", "direct", "U0DANA00", "to the bot"))
	d.delivered("slack", said("U0DANA00", "", "C9/2.0", "heard in no place"))
	if d.latest() != 0 || len(d.carry()) != 0 {
		t.Fatal("what Dana said outside #help is taken")
	}
	// What was carried to a person directly is answered from there.
	n := d.ask("C1/1.0", "refund one")
	sub, err := d.requests.Sub(n, d.requests.Worker("bot"), "payments", "refund one charge", "ticket-1")
	must(t, err)
	d.decide(sub.Number, "Refund one charge", "lee")
	_, err = d.requests.Put(sub.Number, d.requests.Worker("ledger"), requests.KindQuestion, d.requests.Person("priya"), "which card?", nil)
	must(t, err)
	out := d.carry()
	lee, priya := to(out, "slack", "U0LEE000"), to(out, "slack", "U0PRI000")
	if len(lee) != 1 || len(priya) != 1 {
		t.Fatalf("the out runs are %v", places(out))
	}
	d.ended(lee[0], map[string]any{"sent": "D1/1.0"})
	d.ended(priya[0], map[string]any{"sent": "D2/1.0"})
	d.delivered("slack", map[string]any{"id": "press", "kind": "choice", "from": "U0LEE000", "heard_in": "direct", "place": "U0LEE000", "answers": lee[0].Input["ref"], "choice": "yes"})
	d.delivered("slack", said("U0PRI000", "direct", "U0PRI000", "the visa", "answers", "D2/1.0"))
	if len(d.chosen) != 1 || d.chosen[0].Person != "lee" {
		t.Errorf("Lee's press is taken as %v", d.chosen)
	}
	if got := d.thread(sub.Number); got[len(got)-1] != "replied: the visa" {
		t.Errorf("payments' request holds %v", got)
	}
	// A choice made in a place no longer listed, on what was carried to
	// the group there, is not given to the decision, and is answered.
	listed := replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      \"#payments\": { team: payments }")
	g := open(t, listed)
	g.delivered("slack", said("U0LEE000", "#payments", "C7/1.0", "may I refund this one?"))
	_, err = g.requests.Write(1, requests.Writing{Kind: "decision", By: g.requests.Worker("ledger"), To: []requests.Who{requests.Identity("slack", "U0LEE000")}, Text: "Refund it?"})
	must(t, err)
	group := to(g.carry(), "slack", "C7/1.0")
	g.ended(group[0], map[string]any{"sent": "C7/2.0"})
	g.read(work(t))
	g.delivered("slack", map[string]any{"id": "press-unlisted", "kind": "choice", "from": "U0LEE000", "heard_in": "#payments", "place": "C7/1.0", "answers": group[0].Input["ref"], "choice": "yes"})
	if len(g.chosen) != 0 {
		t.Errorf("a press in a place no longer listed is given to the decision: %#v", g.chosen)
	}
	if back := texts(g.drain()); len(back) != 1 || !strings.HasPrefix(back[0], "slack:C7/1.0: ") {
		t.Errorf("the press is answered with %v", back)
	}
	// With direct listed, what Dana says to the bot directly begins a
	// request, its origin her direct place.
	e := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      direct: {}"))
	e.delivered("slack", said("U0DANA00", "direct", "U0DANA00", "to the bot"))
	if r, err := e.requests.Get(1); err != nil || r.Origin.Place != "U0DANA00" || r.Origin.Heard != office.Direct || r.Team != office.Top {
		t.Errorf("Dana's direct message is %#v, %v", r, err)
	}
}

func TestBridgesPlacesReadFromSettings(t *testing.T) {
	t.Parallel()
	d := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": {}`))
	// The in machine saying a team, or that guests may ask, changes nothing.
	d.delivered("slack", said("U0GUEST0", "#help", "C1/1.0", "a stranger", "team", "payments", "guests", true))
	if d.latest() != 0 {
		t.Error("a guest asked where the settings let no guest ask")
	}
	d.delivered("slack", said("U0DANA00", "#help", "C1/2.0", "Dana", "team", "payments"))
	if r, _ := d.requests.Get(1); r == nil || r.Team != office.Top {
		t.Errorf("Dana's request is %#v", r)
	}
}

func TestBridgesMatchedExactly(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	for i, id := range []string{"U0DANA00", "u0dana00", "U0GUEST0"} {
		d.delivered("slack", said(id, "#help", "C1/"+itoa(int64(i+1))+".0", "hello"))
	}
	for n, want := range map[int64]string{1: "person", 2: "guest", 3: "guest"} {
		if asks, _ := d.requests.Asks(n); asks.Kind != want {
			t.Errorf("request %d is asked by %#v", n, asks)
		}
	}
}

func TestBridgesGuestsAskOnlyWhereLet(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.delivered("slack", said("U0GUEST0", "#help", "C1/1.0", "a stranger asks"))
	if asks, _ := d.requests.Asks(1); asks.Kind != requests.IsGuest {
		t.Fatalf("the request is asked by %#v", asks)
	}
	d.answer(1, "bot", "an answer for the stranger")
	if got := places(d.carry()); !slices.Equal(got, []string{"slack:C1/1.0"}) {
		t.Errorf("the guest's answer is carried to %v", got)
	}
	e := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": {}`))
	e.delivered("slack", said("U0GUEST0", "#help", "C1/1.0", "a stranger asks"))
	if e.latest() != 0 || len(e.carry()) != 0 {
		t.Error("a guest asked in #help without guests")
	}
	h := open(t, house(t))
	h.hear("telegram", said("9001", "direct", "9001", "who are you?"))
	if h.latest() != 0 || len(h.carry()) != 0 {
		t.Error("a guest asked directly where direct takes no guests")
	}
	g := open(t, replace(t, house(t), "office.yaml", "direct: {}", "direct: { guests: true }"))
	g.hear("telegram", said("9001", "direct", "9001", "who are you?"))
	r, err := g.requests.Get(1)
	if err != nil || r.Origin.Place != "9001" {
		t.Fatalf("the guest's request is %#v, %v", r, err)
	}
	g.answer(1, "steward", "the house")
	if got := places(g.carry()); !slices.Equal(got, []string{"telegram:9001"}) {
		t.Errorf("the guest's answer is carried to %v", got)
	}
}

func TestBridgesGuestRequestShowsTheIdentity(t *testing.T) {
	t.Parallel()
	d := open(t, replace(t, house(t), "office.yaml", "direct: {}", "direct: { guests: true }"))
	d.hear("telegram", said("9001", "direct", "9001", "hello"))
	v, err := d.requests.View(1, "sam")
	must(t, err)
	if v.Asker.Kind != requests.IsGuest || v.Asker.Bridge != "telegram" || v.Asker.ID != "9001" {
		t.Errorf("Sam sees the asker as %#v", v.Asker)
	}
}

func TestBridgesGuestsOffUnlessSaid(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      \"#ops\": {}\n      \"#closed\": { guests: false }")
	d := open(t, files)
	d.delivered("slack", said("U0GUEST0", "#ops", "C2/1.0", "in ops"))
	d.delivered("slack", said("U0GUEST0", "#closed", "C3/1.0", "in closed"))
	if d.latest() != 0 || len(d.carry()) != 0 {
		t.Error("a guest asked where guests are not let")
	}
	d.delivered("slack", said("U0GUEST0", "#help", "C1/1.0", "in help"))
	if d.latest() != 1 {
		t.Error("a guest could not ask in #help")
	}
	// A decision carried to Dana's thread in #ops, where guests are not
	// let: a guest's press on it is given to no decision, and writes
	// nothing of the readers it gives, but is answered.
	d.delivered("slack", said("U0DANA00", "#ops", "C2/2.0", "may I restart it?"))
	n := d.latest()
	_, err := d.requests.Write(n, requests.Writing{Kind: "decision", By: d.requests.Worker("bot"), To: []requests.Who{requests.Identity("slack", "U0DANA00")}, Text: "Restart it?"})
	must(t, err)
	out := to(d.carry(), "slack", "C2/2.0")
	if len(out) != 1 {
		t.Fatal("the decision is not carried to Dana's thread")
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	d.delivered("slack", map[string]any{"id": "guest-press", "kind": "choice", "from": "U0GUEST0", "heard_in": "#ops", "place": "C2/2.0",
		"answers": out[0].Input["ref"], "choice": "yes", "readers": []any{"U0DANA00", "U0AL0000"}})
	if len(d.chosen) != 0 {
		t.Errorf("a guest's press in #ops is given to a decision as %#v", d.chosen)
	}
	readers, err := d.requests.ReadBy(n)
	must(t, err)
	if len(readers) != 1 || readers[0].Name != "dana" {
		t.Errorf("a guest's press in #ops wrote the readers %#v", readers)
	}
	if back := texts(d.carry()); len(back) != 1 || !strings.HasPrefix(back[0], "slack:C2/2.0: ") {
		t.Errorf("the guest's press is answered with %v", back)
	}
	// The same press in #help, which lets guests ask, is given to the
	// decision, which says what a guest's choice comes to.
	d.delivered("slack", said("U0DANA00", "#help", "C1/2.0", "may I restart it?"))
	m := d.latest()
	_, err = d.requests.Write(m, requests.Writing{Kind: "decision", By: d.requests.Worker("bot"), To: []requests.Who{requests.Identity("slack", "U0DANA00")}, Text: "Restart it?"})
	must(t, err)
	out = to(d.carry(), "slack", "C1/2.0")
	if len(out) != 1 {
		t.Fatal("the decision is not carried to Dana's thread in #help")
	}
	d.ended(out[0], map[string]any{"sent": "s2"})
	d.delivered("slack", map[string]any{"id": "guest-press-2", "kind": "choice", "from": "U0GUEST0", "heard_in": "#help", "place": "C1/2.0",
		"answers": out[0].Input["ref"], "choice": "yes"})
	if len(d.chosen) != 1 || d.chosen[0].Person != "" || d.chosen[0].Who != requests.Identity("slack", "U0GUEST0") {
		t.Errorf("a guest's press in #help is given as %#v", d.chosen)
	}
}

func TestBridgesIdentityIsTheSender(t *testing.T) {
	t.Parallel()
	d := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      \"#payments\": { team: payments }"))
	// Lee asks in a group, and is carried a choice there.
	d.delivered("slack", said("U0LEE000", "#payments", "C7/1.0", "may I refund this one?"))
	n := d.latest()
	if asks, _ := d.requests.Asks(n); asks.Name != "lee" {
		t.Errorf("Lee's message in a group is asked by %#v", asks)
	}
	choice, err := d.requests.Write(n, requests.Writing{Kind: "decision", By: d.requests.Worker("ledger"), To: []requests.Who{requests.Identity("slack", "U0LEE000")}, Text: "Refund it?"})
	must(t, err)
	out := to(d.carry(), "slack", "C7/1.0")
	if len(out) != 1 {
		t.Fatal("the choice is not carried to the group")
	}
	d.ended(out[0], map[string]any{"sent": "s1"})
	// Lee's press there is matched on his own identity, not on the group.
	d.delivered("slack", map[string]any{"id": "press", "kind": "choice", "from": "U0LEE000", "heard_in": "#payments", "place": "C7/1.0", "answers": out[0].Input["ref"], "choice": "yes"})
	if len(d.chosen) != 1 || d.chosen[0].Person != "lee" || d.chosen[0].Who != requests.Identity("slack", "U0LEE000") || d.chosen[0].Entry != choice.ID {
		t.Errorf("the press is taken as %#v", d.chosen)
	}
	d.delivered("slack", map[string]any{"id": "press2", "kind": "choice", "from": "U0PRI000", "heard_in": "#payments", "place": "C7/1.0", "answers": out[0].Input["ref"], "choice": "no"})
	if len(d.chosen) != 2 || d.chosen[1].Person != "priya" {
		t.Errorf("Priya's press there is taken as %#v", d.chosen)
	}
}

func TestBridgesEveryChoiceAnswered(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	d.decide(n, "Refund one charge", "lee")
	out := to(d.carry(), "slack", "U0LEE000")
	d.ended(out[0], map[string]any{"sent": "s1"})
	press := func(from, ref string) {
		d.delivered("slack", map[string]any{"id": "press-" + itoa(ids.Add(1)), "kind": "choice", "from": from, "heard_in": "direct", "place": "U0LEE000", "answers": ref, "choice": "yes"})
	}
	press("U0LEE000", out[0].Input["ref"].(string))
	press("U0LEE000", "r999")
	press("U0PRI000", out[0].Input["ref"].(string))
	d.choose = func(bridges.Choice) (string, error) { return "", errors.New("the store is locked: decisions_walks") }
	press("U0LEE000", out[0].Input["ref"].(string)+"")
	back := d.carry()
	var texts []string
	for _, s := range back {
		texts = append(texts, s.Input["to"].(string)+": "+textIn(s))
	}
	if len(back) != 2 {
		t.Fatalf("the sentences carried back are %v", texts)
	}
	for _, s := range back {
		d.ended(s, map[string]any{"sent": "s"})
	}
	for _, s := range d.carry() {
		texts = append(texts, s.Input["to"].(string)+": "+textIn(s))
		d.ended(s, map[string]any{"sent": "s"})
	}
	for _, s := range d.carry() {
		texts = append(texts, s.Input["to"].(string)+": "+textIn(s))
	}
	slices.Sort(texts)
	want := []string{
		"U0LEE000: Bulldog could not take that just now; the office's people have been told.",
		"U0LEE000: Taken: yes.",
		"U0LEE000: That choice is on nothing that waits, so it was not taken.",
		"U0PRI000: That was sent to someone else, so nothing you said was taken.",
	}
	if !slices.Equal(texts, want) {
		t.Errorf("the sentences carried back are\n%s", strings.Join(texts, "\n"))
	}
}

func TestBridgesIdentityOnlyFromInRun(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	d.decide(n, "Refund one charge", "lee")
	out := to(d.carry(), "slack", "U0LEE000")
	// The delivery claims to be Lee's press; the in machine finds it is not
	// Slack's, and hears nothing.
	claim := `{"type":"block_actions","user":{"id":"U0LEE000"},"actions":[{"value":"` + out[0].Input["ref"].(string) + `","action_id":"yes"}]}`
	s, err := d.Hear("slack", bridges.Delivery{Method: "POST", Body: claim, Headers: map[string]string{"X-Slack-Signature": "v0=forged"}})
	must(t, err)
	if s.Input["delivery"].(bridges.Delivery).Body != claim {
		t.Errorf("the delivery is handed as %#v", s.Input["delivery"])
	}
	_, err = d.InEnded("slack", "a run", bridges.End{Output: map[string]any{"heard": []any{}, "answer": map[string]any{"status": 401}}})
	must(t, err)
	if len(d.chosen) != 0 {
		t.Errorf("a choice was taken from the delivery: %#v", d.chosen)
	}
}

func TestBridgesRefusalIsASentence(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	d.hear("telegram", said("4472", "direct", "4472", "the socket sparked"))
	_, err := d.requests.Put(1, d.requests.Worker("steward"), requests.KindQuestion, d.requests.Person("kit"), "is the fuse box under the stairs?", nil)
	must(t, err)
	out := d.carry()
	d.ended(out[0], map[string]any{"sent": "s1"})
	// Sam answers what was sent to Kit: an error's text would name Kit,
	// the request and the store.
	d.hear("telegram", said("4471", "direct", "4472", "it is", "answers", "s1"))
	back := d.carry()
	if len(back) != 1 || back[0].Input["to"] != "4471" {
		t.Fatalf("carried back: %v", places(back))
	}
	text := textIn(back[0])
	if strings.Contains(text, "kit") || strings.Contains(text, "request") || strings.Count(text, ".") != 1 {
		t.Errorf("Sam is told %q", text)
	}
}

func TestBridgesReplyCheckedAgainstSightNow(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	d.hear("telegram", said("4471", "direct", "4471", "the socket sparked"))
	torch := said("4472", "direct", "4472", "where is the torch?", "threadless", true)
	d.hear("telegram", torch)
	_, err := d.requests.Put(1, d.requests.Worker("steward"), requests.KindQuestion, d.requests.Person("kit"), "is the fuse box under the stairs?", nil)
	must(t, err)
	out := d.carry()
	d.ended(out[0], map[string]any{"sent": "s1"})
	before := d.thread(1)
	// A reply in another's direct chat does not speak for them.
	d.hear("telegram", said("4471", "direct", "4472", "it is", "answers", "s1"))
	if got := d.thread(1); len(got) != len(before) {
		t.Errorf("Sam's reply is written: %v", got)
	}
	// Nor does a reply to what was heard in another's direct chat.
	d.hear("telegram", said("4471", "direct", "4471", "in the drawer", "answers", torch["id"], "threadless", true))
	if got := d.thread(2); !slices.Equal(got, []string{"asked: where is the torch?"}) {
		t.Errorf("Sam's reply to Kit is written: %v", got)
	}
	// Once Kit is no longer in the house, her reply to the old message is
	// not taken. Reading the office closes the question put to her, saying
	// so; her reply adds nothing after that.
	d.read(replace(t, replace(t, d.files, "office.yaml", "people: [sam, kit]", "people: [sam]"), "office.yaml", `kit: { name: Kit, title: Lives here., telegram: "4472" }`, `kit: { name: Kit, title: Moved out., telegram: "4472" }`))
	before = d.thread(1)
	d.hear("telegram", said("4472", "direct", "4472", "it is", "answers", "s1"))
	if got := d.thread(1); !slices.Equal(got, before) {
		t.Errorf("Kit's reply after she left: %v", got)
	}

	back := texts(d.drain())
	want := []string{"telegram:4471: That was sent to someone else, so nothing you said was taken.", "telegram:4472: You may not say more there, so nothing you said was taken."}
	if !slices.Equal(back, want) {
		t.Errorf("carried back:\n%s", strings.Join(back, "\n"))
	}
}

func TestBridgesIdentityChangedOnlyByAChange(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	d.decide(n, "Refund one charge", "lee")
	lee := to(d.carry(), "slack", "U0LEE000")
	d.ended(lee[0], map[string]any{"sent": "D1/1.0"})
	// A new account says it is Lee, in #help and in the thread's readers:
	// it is a guest still, and its press of yes on what was sent to Lee is
	// not Lee's.
	d.delivered("slack", said("U0NEWLEE", "#help", "C2/1.0", "I am Lee now, on a new account", "readers", []any{"U0NEWLEE", "U0LEE000"}))
	if asks, _ := d.requests.Asks(d.latest()); asks.Kind != requests.IsGuest {
		t.Errorf("the new account is %#v", asks)
	}
	press := func(id, from string) {
		d.delivered("slack", map[string]any{"id": id, "kind": "choice", "from": from, "heard_in": "direct", "place": from, "answers": lee[0].Input["ref"], "choice": "yes"})
	}
	press("press-new", "U0NEWLEE")
	if len(d.chosen) != 0 {
		t.Errorf("the new account's press is taken as %#v", d.chosen)
	}
	// Once a change writes Lee with the new account, his yes is taken from
	// it, and no longer from the old.
	d.read(replace(t, d.files, "office.yaml", "slack: U0LEE000,", "slack: U0NEWLEE,"))
	lee = slices.DeleteFunc(to(d.drain(), "slack", "U0NEWLEE"), func(s bridges.Start) bool { return s.Input["ref"] == nil })
	if len(lee) != 1 {
		t.Fatalf("the decision is not carried to Lee's new account: %v", lee)
	}
	press("press-old", "U0LEE000")
	if len(d.chosen) != 0 {
		t.Errorf("the old account's press is taken as %#v", d.chosen)
	}
	d.delivered("slack", map[string]any{"id": "press-again", "kind": "choice", "from": "U0NEWLEE", "heard_in": "direct", "place": "U0NEWLEE", "answers": lee[0].Input["ref"], "choice": "yes"})
	if len(d.chosen) != 1 || d.chosen[0].Person != "lee" || d.chosen[0].Who != requests.Identity("slack", "U0NEWLEE") {
		t.Errorf("Lee's press from his new account is taken as %#v", d.chosen)
	}
}

func TestBridgesRefusedAskingIsTrouble(t *testing.T) {
	t.Parallel()
	d := open(t, replace(t, work(t), "bot.yaml", "model: usual", "model: nosuchmodel"))
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "customer 4411 was charged twice"))
	if d.latest() != 0 {
		t.Fatal("a request was made for a lead that cannot run")
	}
	troubles, err := d.Troubles()
	must(t, err)
	if len(troubles) != 1 || !strings.Contains(troubles[0].Error, "cannot run until its file is mended") {
		t.Errorf("the troubles are %v", troubles)
	}
	if back := texts(d.drain()); !slices.Equal(back, []string{"slack:C1/1.0: Bulldog could not take that just now; the office's people have been told."}) {
		t.Errorf("Dana is told %v", back)
	}
}

func TestBridgesChoiceOnAQuestionAnswersIt(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	q, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, requests.Identity("slack", "U0DANA00"), "which card?", []string{"the visa", "the amex"})
	must(t, err)
	out := to(d.carry(), "slack", "C1/1.0")
	if len(out) != 1 || !slices.Equal(out[0].Input["choices"].([]string), []string{"the visa", "the amex"}) || out[0].Input["ref"] == nil {
		t.Fatalf("the question is carried as %v", out)
	}
	d.ended(out[0], map[string]any{"sent": "C1/2.0"})
	ref := out[0].Input["ref"].(string)
	press := func(from, choice string) {
		d.delivered("slack", map[string]any{"id": "press-" + itoa(ids.Add(1)), "kind": "choice", "from": from, "heard_in": "#help", "place": "C1/1.0",
			"answers": ref, "choice": choice})
	}
	// Al reads #help, and it does not wait on him; Dana's pick of what it
	// does not offer, as written or not, is not taken, nor a press on what
	// the service no longer shows; her pick is her answer; and her second
	// finds it waiting no longer.
	press("U0AL0000", "the visa")
	press("U0DANA00", "the diners")
	press("U0DANA00", "The amex")
	press("U0DANA00", "")
	press("U0DANA00", "the amex")
	press("U0DANA00", "the visa")
	if len(d.chosen) != 0 {
		t.Errorf("a pick on a question is given to a decision: %v", d.chosen)
	}
	thread, err := d.requests.Thread(n)
	must(t, err)
	var replies []requests.Entry
	for _, e := range thread {
		if e.Answers == q.ID {
			replies = append(replies, e)
		}
	}
	if len(replies) != 1 || replies[0].Kind != requests.KindReplied || replies[0].Text != "the amex" || replies[0].By != requests.Identity("slack", "U0DANA00") {
		t.Errorf("the question is answered %+v", replies)
	}
	var back []string
	for _, s := range d.drain() {
		back = append(back, textIn(s))
	}
	want := []string{
		"That was sent to someone else, so nothing you said was taken.",
		"That is not one of the choices the question offers, so it was not taken.",
		"That is not one of the choices the question offers, so it was not taken.",
		"That question is no longer there to press, so nothing you chose was taken.",
		"Taken: you answered the amex.",
		"That is no longer waiting on you, so nothing you said was taken.",
	}
	if !slices.Equal(back, want) {
		t.Errorf("the sentences carried back are %q", back)
	}
	// A question that offers nothing is carried with no choices, for a
	// reply.
	_, err = d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, requests.Identity("slack", "U0DANA00"), "anything else?", nil)
	must(t, err)
	if out := to(d.carry(), "slack", "C1/1.0"); len(out) != 1 || len(out[0].Input["choices"].([]string)) != 0 || out[0].Input["ref"] == nil {
		t.Errorf("a question with no choices is carried as %v", out)
	}
}

func TestBridgesADirectThreadIsItsOwnConversation(t *testing.T) {
	t.Parallel()
	d := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: true }
      direct: {}`))
	dm := func(place, text string, more ...any) {
		d.delivered("slack", said("U0DANA00", "direct", place, text, more...))
	}
	// Dana's first message to the app directly begins a request, said at
	// its own thread, which is where its answer goes, and not to her
	// directly.
	dm("D0DANA00/1.0", "is the API down?")
	first := d.latest()
	d.answer(first, "bot", "It is up.")
	out := to(d.carry(), "slack", "D0DANA00/1.0")
	if len(out) != 1 || out[0].Input["direct"] != false {
		t.Fatalf("the answer is carried as %v", out)
	}
	d.ended(out[0], map[string]any{"sent": "D0DANA00/1.5"})
	// Her next message, replying to nothing, begins a request of its own;
	// her reply in the first's thread goes on in the first.
	dm("D0DANA00/2.0", "and the ledger?")
	second := d.latest()
	dm("D0DANA00/1.0", "thanks", "answers", "D0DANA00/1.0")
	if second == first || d.latest() != second || !slices.Equal(d.thread(first)[2:], []string{"said: thanks"}) {
		t.Errorf("Dana's messages go to %d and %d: %v", first, second, d.thread(first))
	}
	// A question to her goes to its request's thread, and her reply there
	// answers it.
	_, err := d.requests.Put(second, d.requests.Worker("bot"), requests.KindQuestion, requests.Identity("slack", "U0DANA00"), "which ledger?", nil)
	must(t, err)
	for _, s := range to(d.carry(), "slack", "D0DANA00/2.0") {
		d.ended(s, map[string]any{"sent": "D0DANA00/2.5"})
	}
	dm("D0DANA00/2.0", "payments", "answers", "D0DANA00/2.0")
	if got := d.thread(second); got[len(got)-1] != "replied: payments" {
		t.Errorf("Dana's answer is %v", got)
	}
	// A question carried to Al directly is answered by his next message,
	// while it waits on him, and once it waits no longer his next begins a
	// request.
	_, err = d.requests.Put(second, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("al"), "may I refund it?", nil)
	must(t, err)
	for _, s := range to(d.carry(), "slack", "U0AL0000") {
		d.ended(s, map[string]any{"sent": "D0AL0000/3.0"})
	}
	before := d.latest()
	d.delivered("slack", said("U0AL0000", "direct", "D0AL0000/4.0", "yes"))
	if got := d.thread(second); got[len(got)-1] != "replied: yes" || d.latest() != before {
		t.Errorf("Al's answer is %v", got)
	}
	d.delivered("slack", said("U0AL0000", "direct", "D0AL0000/5.0", "one more thing"))
	if d.latest() == before {
		t.Errorf("Al's message after his answer is said in %d", second)
	}
}
