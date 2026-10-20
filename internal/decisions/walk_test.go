package decisions_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// reviewer is a worker of the house that decides.
const reviewer = `agent: reviewer
input: [said]
model: usual
system: You review what the fixer would do. End with { "says": "yes", "refuse" or "refer", "note": why }.
prompt: ${input.said}
`

// withDecide gives the house's files with its office.yaml deciding as
// given, the reviewer and two rules beside it.
func withDecide(t *testing.T, decide string) map[string]string {
	t.Helper()
	files := house(t)
	text := files["office.yaml"]
	from, to := strings.Index(text, "decide:\n"), strings.Index(text, "routines:")
	files["office.yaml"] = text[:from] + decide + "\n" + text[to:]
	files["reviewer.yaml"] = reviewer
	files["a.js"] = `return { says: "refer", note: "unsure" }`
	files["b.js"] = `return { says: "refuse", note: "no" }`
	files["garden/a.js"] = `return { says: "refer", note: "unsure" }`
	return files
}

// withPayments gives the work office with payments' team file written
// so.
func withPayments(t *testing.T, team string) map[string]string {
	t.Helper()
	files := work(t)
	files["payments/team.yaml"] = team
	files["payments/refuse-all.js"] = `return { says: "refuse", note: "never" }`
	return files
}

func TestHeldActionDecidedByItsList(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  order_part: [./small-orders.js, sam]\n  cut_power: [reviewer, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "socket", 12))
	if c := b.runner.next(t); c.Kind != office.IsRule || c.Name != "small-orders.js" {
		t.Fatalf("order_part is first asked of %s %s", c.Kind, c.Name)
	}
	b.one("ask-2", office.Top, n, fixerUp, cutPower("4"))
	if c := b.runner.next(t); c.Kind != office.IsWorker || c.Name != "reviewer" {
		t.Fatalf("cut_power is first asked of %s %s", c.Kind, c.Name)
	}
	dec := b.one("ask-3", office.Top, n, fixerUp, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the front door"}})
	if !slices.Equal(dec.Asked, []string{"sam"}) {
		t.Fatalf("open_door is asked of %v, not of the house's everything", dec.Asked)
	}
}

// withGithub gives files with the fixer given the tools a GitHub server
// lists, each held for a yes.
func withGithub(files map[string]string) map[string]string {
	files["fixer.yaml"] = strings.Replace(files["fixer.yaml"], "tools:\n", "tools:\n  github: { list: { script: ./tools/github.js }, job: { script: ./tools/github.js, input: { tool: \"${tool}\", args: \"${args}\" }, approve: true } }\n", 1)
	files["tools/github.js"] = "return {}"
	return files
}

func TestHeldListedCallDecidedByItsEntrysList(t *testing.T) {
	t.Parallel()
	call := decisions.Job{Name: "github_create_issue", Asked: &decisions.Asked{Does: "Create an issue", Entry: "github", Tool: "create_issue"}}
	// A call of a tool a list: entry gives is decided by the entry's list
	// when the team names none for the tool itself.
	b := newBench(t, withGithub(withDecide(t, "decide:\n  everything: [sam]\n  github: [kit]")))
	n := b.chat("kit", office.Top, "file the bug")
	if dec := b.one("ask-1", office.Top, n, fixerUp, call); !slices.Equal(dec.Asked, []string{"kit"}) {
		t.Errorf("github_create_issue is asked of %v, not of the github list", dec.Asked)
	}
	b = newBench(t, withGithub(withDecide(t, "decide:\n  everything: [sam]\n  github: [sam]\n  github_create_issue: [kit]")))
	n = b.chat("kit", office.Top, "file the bug")
	if dec := b.one("ask-1", office.Top, n, fixerUp, call); !slices.Equal(dec.Asked, []string{"kit"}) {
		t.Errorf("github_create_issue is asked of %v, not of its own list", dec.Asked)
	}
}

func TestDisclosureAndChangeAskedOfTheirOwnLists(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	if got := b.waitingFor("lee"); len(got) != 1 {
		t.Fatalf("the refund waits on Lee %d times", len(got))
	}
	if len(b.runner.started()) != 0 {
		t.Fatalf("the held refund was put to %s", b.runner.started()[0].Name)
	}
	if _, err := b.d.Disclose(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: b.reqs.Worker("ledger"), Text: "Duplicate confirmed; £40 refunded."}); err != nil {
		t.Fatal(err)
	}
	if c := b.runner.next(t); c.Name != "payments/no-account-numbers.yaml" || c.Kind != office.IsRule {
		t.Fatalf("payments' answer is first put to %s %s", c.Kind, c.Name)
	}
}

func TestNoEverythingIsTheTeamsPeople(t *testing.T) {
	t.Parallel()
	b := newBench(t, withPayments(t, "people: [lee, priya]\nlead: ledger\n"))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	if !slices.Equal(dec.Asked, []string{"lee", "priya"}) {
		t.Fatalf("the refund is asked of %v", dec.Asked)
	}
}

func TestDecidersAskedInOrder(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  open_door: [./a.js, ./b.js, sam]"))
	n := b.chat("kit", office.Top, "let the plumber in")
	dec := b.one("ask-1", office.Top, n, fixerUp, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the front door"}})
	a := b.runner.next(t)
	if a.Name != "a.js" {
		t.Fatalf("%s was asked first", a.Name)
	}
	a.says(decisions.SaysRefer, "unsure, 0.6")
	c := b.runner.next(t)
	if c.Name != "b.js" {
		t.Fatalf("%s was asked second", c.Name)
	}
	referrals := c.Input["referrals"].([]map[string]any)
	if len(referrals) != 1 || referrals[0]["by"] != "a.js" || referrals[0]["note"] != "unsure, 0.6" {
		t.Fatalf("b.js was given the referrals %v", referrals)
	}
	c.says(decisions.SaysRefuse, "no plumber today")
	if got := b.ended(dec.ID); got.Ended != decisions.Refused || got.By.Called != "b.js" {
		t.Fatalf("the decision ended %+v", got)
	}
	if len(b.waitingFor("sam")) != 0 || slices.Contains(b.get(dec.ID).WasAsked, "sam") {
		t.Fatal("Sam was asked")
	}
	lines := b.thread(n)
	has(t, lines, "referred by a.js", "referred by a.js: unsure, 0.6")
	has(t, lines, "decided by b.js", "refused by b.js: no plumber today")
}

func TestRefusalEndsTheWalk(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [reviewer, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("6"))
	b.runner.next(t).says(decisions.SaysRefuse, "That circuit feeds the freezer.")
	v := b.verdict("ask-1")
	if len(v.Approve) != 0 || !strings.Contains(v.Note, "That circuit feeds the freezer.") {
		t.Fatalf("the ask is answered %+v", v)
	}
	if got := b.get(dec.ID); len(got.WasAsked) != 0 {
		t.Fatalf("%v were asked after the reviewer refused", got.WasAsked)
	}
}

func TestYesEndsTheWalkAndAReferralGoesToAPerson(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  order_part: [./small-orders.js, sam]\n  cut_power: [reviewer, sam]"))
	b.runner.answers(smallOrders)
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "socket", 12))
	if v := b.verdict("ask-1"); !slices.Equal(v.Approve, []string{"order_part"}) {
		t.Fatalf("the small order is answered %+v", v)
	}
	big := b.one("ask-2", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	b.until("the large order reaches Sam", func() bool { return len(b.waitingFor("sam")) == 1 })
	if !strings.Contains(b.waitingFor("sam")[0].Text, "£300 is more than I order alone.") {
		t.Fatalf("Sam is not shown the rule's note: %s", b.waitingFor("sam")[0].Text)
	}
	if b.answers("ask-2") != 0 {
		t.Fatal("the large order was answered before Sam")
	}
	b.decide(big.ID, "sam", decisions.SaysYes, "")
	b.verdict("ask-2")

	b.runner.answers(nil)
	b.one("ask-3", office.Top, n, fixerUp, cutPower("4"))
	b.runner.next(t).says(decisions.SaysYes, "breaker 4 is the kitchen")
	if v := b.verdict("ask-3"); !slices.Equal(v.Approve, []string{"cut_power"}) {
		t.Fatalf("the reviewer's yes is answered %+v", v)
	}
	dec := b.one("ask-4", office.Top, n, fixerUp, cutPower("6"))
	b.runner.next(t).says(decisions.SaysRefer, "it may feed the freezer, 0.6")
	b.until("the cut reaches Sam", func() bool { return len(b.waitingFor("sam")) == 1 && len(b.get(dec.ID).Asked) == 1 })
	if ws := b.waitingFor("sam"); !strings.Contains(ws[0].Text, "it may feed the freezer, 0.6") {
		t.Fatalf("Sam is not shown the reviewer's note: %s", ws[0].Text)
	}
}

func TestPeopleAskedAtOnceFirstAnswerDecides(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	dec := b.one("ask-1", "payments", sub, []string{"ledger"}, refund())
	ds, err := b.reqs.Deliveries(dec.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 || ds[0].Person != "lee" || ds[1].Person != "priya" {
		t.Fatalf("the decision is carried to %+v", ds)
	}
	if len(b.waitingFor("lee")) != 1 || len(b.waitingFor("priya")) != 1 {
		t.Fatal("the decision does not wait on both")
	}
	b.decide(dec.ID, "lee", decisions.SaysYes, "")
	b.clock.Move(60e9)
	text := b.refused(dec.ID, b.reqs.Person("priya"), decisions.SaysRefuse)
	if !strings.Contains(text, "approved by Lee at 10:00") {
		t.Fatalf("Priya is told %q", text)
	}
}

func TestEveryReferralGoesOutward(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  order_part: [./small-orders.js]"))
	b.runner.answers(smallOrders)
	n := b.chat("kit", office.Top, "a new cooker")
	dec := b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	b.until("the order reaches Sam", func() bool { return len(b.get(dec.ID).Asked) == 1 })

	w := newBench(t, work(t))
	r, err := w.reqs.Sub(w.chat("al", office.Top, "what broke?"), w.reqs.Worker("bot"), "platform", "what was deployed?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	ops := w.one("ask-1", "platform", r.Number, []string{"ops"}, decisions.Job{Name: "deploy", Asked: &decisions.Asked{Does: "Deploy again"}})
	if !slices.Equal(ops.Asked, []string{"al"}) {
		t.Fatalf("ops' held action is asked of %v, not Al", ops.Asked)
	}
}

func TestDisclosureReferredGoesToEverything(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	_, sub := b.helpRequest()
	id, err := b.d.Disclose(ctx, decisions.Said{What: decisions.SaysAnswer, Request: sub, By: b.reqs.Worker("ledger"), Text: "Card 4111 1111 1111 1111 was charged twice."})
	if err != nil {
		t.Fatal(err)
	}
	b.runner.next(t).says(decisions.SaysRefer, "It may name an account number, 0.90.")
	b.until("the disclosure reaches Lee and Priya", func() bool { return len(b.get(id).Asked) == 2 })
	if b.letsOf(id) != 0 {
		t.Fatal("the disclosure ended before Lee let it go")
	}
	b.decide(id, "lee", decisions.SaysYes, "")
	if l := b.let(id); !l.Disclosed {
		t.Fatalf("the disclosure ended %+v", l)
	}
}

func TestDecisionsEveryWalkEndsAtAPerson(t *testing.T) {
	t.Parallel()
	files := withDecide(t, "decide:\n  everything: [reviewer, sam]")
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "decide: { everything: [sam] }", "decide: { everything: [./a.js] }", 1)
	b := newBench(t, files)
	g := b.chat("sam", "garden", "open the shed")
	dec := b.one("ask-1", "garden", g, []string{"gardener"}, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Open the shed door"}})
	a := b.runner.next(t)
	if a.Name != "garden/a.js" {
		t.Fatalf("%s was asked first", a.Name)
	}
	a.says(decisions.SaysRefer, "unsure")
	r := b.runner.next(t)
	if r.Name != "reviewer" || r.Team != "garden" {
		t.Fatalf("%s of %s was asked second", r.Name, r.Team)
	}
	r.says(decisions.SaysRefer, "Sam should see this")
	b.until("the walk reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	lines := b.thread(g)
	has(t, lines, "referred by a.js: unsure")
	has(t, lines, "referred by reviewer: Sam should see this")

	gone := b.chat("sam", "garden", "and the gate")
	b.rewrite(map[string]string{"garden/team.yaml": "", "garden/gardener.yaml": "", "garden/groundsman.yaml": "", "garden/a.js": "",
		"garden/notes/lawn.md": "", "garden/notes/shed.md": ""})
	none := b.one("ask-2", "garden", gone, []string{"gardener"}, decisions.Job{Name: "open_gate", Asked: &decisions.Asked{Does: "Open the gate"}})
	if none.Ended != decisions.Refused || !strings.Contains(none.By.Note, "nobody can decide this") {
		t.Fatalf("a decision nobody can decide is %+v", none)
	}
}

func TestDecisionsARuleDecidesAsTheTeamWhoseDecisionItIs(t *testing.T) {
	t.Parallel()
	files := withDecide(t, "decide:\n  everything: [./a.js, sam]")
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "decide: { everything: [sam] }", "decide: { everything: [./a.js] }", 1)
	b := newBench(t, files)
	g := b.chat("sam", "garden", "open the shed")
	dec := b.one("ask-1", "garden", g, []string{"gardener"}, decisions.Job{Name: "open_door"})
	b.runner.next(t).says(decisions.SaysRefer, "unsure")
	top := b.runner.next(t)
	if top.Name != "a.js" || top.Team != "garden" {
		t.Fatalf("%s of %s was asked second", top.Name, top.Team)
	}
	top.says(decisions.SaysRefer, "unsure")
	b.until("the walk reaches Sam", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
	referrals := b.entries(g, decisions.KindReferred)
	if len(referrals) != 2 {
		t.Fatalf("the thread holds %d referrals", len(referrals))
	}
	for _, e := range referrals {
		if e.By.Kind != requests.IsRule || e.By.Team != "garden" || e.Read != "garden" {
			t.Errorf("a rule's referral on the garden's decision is by %+v, read by %q", e.By, e.Read)
		}
	}
	b.ofDecision(g, dec.ID, decisions.KindReferred)
}

// The walk is given by team and key with no decision, as each decision
// is walked: a team's list of the key, then everything outward to the
// first people.
func TestWalkGivenByTeamAndKey(t *testing.T) {
	t.Parallel()
	files := withDecide(t, "decide:\n  everything: [./a.js, sam, kit]\n  order_part: [./small-orders.js, reviewer]\n  change: [./b.js]")
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "decide: { everything: [sam] }", "decide: { everything: [./a.js] }", 1)
	b := newBench(t, files)
	rule := func(name, team, list string) decisions.Stage {
		return decisions.Stage{Kind: office.IsRule, Name: name, Team: team, List: list}
	}
	everything := []decisions.Stage{rule("a.js", office.Top, "everything"), {Kind: decisions.IsPeople, People: []string{"sam", "kit"}, Team: office.Top, List: "everything"}}
	for _, c := range []struct {
		team string
		keys []string
		want []decisions.Stage
	}{
		{office.Top, []string{"order_part"}, append([]decisions.Stage{rule("small-orders.js", office.Top, "order_part"),
			{Kind: office.IsWorker, Name: "reviewer", Team: office.Top, List: "order_part"}}, everything...)},
		{office.Top, []string{"open_door"}, everything},
		{office.Top, []string{"everything"}, everything},
		{office.Top, []string{"change"}, append([]decisions.Stage{rule("b.js", office.Top, "change")}, everything...)},
		{"garden", []string{"open_gate", "order_part"}, append([]decisions.Stage{rule("garden/a.js", "garden", "everything")}, everything...)},
		{"gone", []string{"open_door"}, nil},
	} {
		if got := decisions.Walk(b.o, c.team, c.keys...); !slices.EqualFunc(got, c.want, func(a, b decisions.Stage) bool {
			return a.Kind == b.Kind && a.Name == b.Name && slices.Equal(a.People, b.People) && a.Team == b.Team && a.List == b.List
		}) {
			t.Errorf("%s's walk of %v is %+v", c.team, c.keys, got)
		}
	}
}

func TestDecisionsReachOutward(t *testing.T) {
	t.Parallel()
	plain := newBench(t, work(t))
	for team, want := range map[string][]string{"platform": {"al"}, "payments": {"lee", "priya"}, office.Top: {"al"}} {
		if got := plain.o.Reached(team); !slices.Equal(got, want) {
			t.Fatalf("%s's decisions reach %v", team, got)
		}
	}
	files := withPayments(t, "people: [lee, priya]\nlead: ledger\n")
	files["platform/team.yaml"] += "decide: { everything: [./a.js] }\n"
	files["platform/a.js"] = `return { says: "refer" }`
	b := newBench(t, files)
	if got := b.o.Reached("platform"); !slices.Equal(got, []string{"al"}) {
		t.Fatalf("platform's decisions, past its rule, reach %v", got)
	}
	r, err := b.reqs.Sub(b.chat("al", office.Top, "what broke?"), b.reqs.Worker("bot"), "platform", "what was deployed?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	dec := b.one("ask-1", "platform", r.Number, []string{"ops"}, decisions.Job{Name: "deploy", Asked: &decisions.Asked{Does: "Deploy again"}})
	b.runner.next(t).says(decisions.SaysRefer, "")
	b.until("the walk reaches Al", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"al"}) })

	h := withDecide(t, "decide:\n  everything: [sam]")
	h["garden/team.yaml"] = "name: The garden\npeople: []\nlead: gardener\ndecide: { everything: [./a.js] }\nmodels:\n  usual: { name: opus, service: claude-code }\n"
	hb := newBench(t, h)
	if got := hb.o.Reached("garden"); !slices.Equal(got, []string{"sam"}) {
		t.Fatalf("the garden's decisions reach %v", got)
	}
}

func TestPersonSaysYesOrRefuses(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	if got := decisions.Choices(); !slices.Equal(got, []string{"yes", "refuse"}) {
		t.Fatalf("a person may answer %v", got)
	}
	r, err := b.d.Choose(ctx, decisions.Ref(dec.ID), requests.Identity("telegram", "4471"), "maybe")
	if err != nil {
		t.Fatal(err)
	}
	if r.Taken || !strings.Contains(r.Text, "answer yes or refuse") {
		t.Fatalf("a maybe is answered %+v", r)
	}
	if got := b.get(dec.ID); got.Ended != "" {
		t.Fatal("a maybe decided it")
	}
	if r, _ := b.d.Choose(ctx, decisions.Ref(dec.ID), requests.Identity("telegram", "4471"), decisions.SaysRefuse); !r.Taken {
		t.Fatalf("Sam's refusal is answered %+v", r)
	}
	door := b.one("ask-2", office.Top, n, fixerUp, decisions.Job{Name: "open_door", Asked: &decisions.Asked{Does: "Über the door, open it"}})
	if r, _ := b.d.Choose(ctx, decisions.Ref(door.ID), requests.Identity("telegram", "4471"), decisions.SaysYes); r.Text != "Taken: your yes on über the door, open it." {
		t.Fatalf("Sam's yes is answered %+v", r)
	}
}

func TestAnyoneTheWalkWouldReachMayDecide(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "a new cooker")
	dec := b.one("ask-1", office.Top, n, fixerUp, orderPart("order_part", "cooker", 300))
	rule := b.runner.next(t)
	has(t, b.seen(n, "kit"), "decision: Order a part")
	if text := b.refused(dec.ID, b.reqs.Person("kit"), decisions.SaysYes); !strings.Contains(text, "Sam does") {
		t.Fatalf("Kit is told %q", text)
	}
	b.decide(dec.ID, "sam", decisions.SaysYes, "go on")
	rule.cancelled(t)
	if got := b.get(dec.ID); got.Ended != decisions.Yes || got.By.Name != "sam" {
		t.Fatalf("the decision is %+v", got)
	}

	// Sam answers on Telegram, naming the decision, while the rule runs.
	other := b.one("ask-2", office.Top, n, fixerUp, orderPart("order_part", "oven", 400))
	rule = b.runner.next(t)
	r, err := b.d.Choose(ctx, decisions.Ref(other.ID), requests.Identity("telegram", "4471"), decisions.SaysRefuse)
	if err != nil || !r.Taken {
		t.Fatalf("Sam's refusal on Telegram is answered %+v, %v", r, err)
	}
	rule.cancelled(t)
	if got := b.get(other.ID); got.Ended != decisions.Refused || got.By.Name != "sam" {
		t.Fatalf("the decision is %+v", got)
	}
}

func TestDecisionsDeciderToldTruthAboutAfter(t *testing.T) {
	t.Parallel()
	cases := []struct{ decide, team, want string }{
		{"decide:\n  everything: [sam]\n  cut_power: [reviewer, sam]", office.Top, "Sam"},
		{"decide:\n  everything: [sam]\n  cut_power: [reviewer]", office.Top, "Sam, on the house's everything list"},
		// The reviewer is on both lists: having referred it on one, it is not
		// asked again on the other.
		{"decide:\n  everything: [reviewer, sam]\n  cut_power: [reviewer]", office.Top, "Sam, on the house's everything list"},
	}
	for _, c := range cases {
		b := newBench(t, withDecide(t, c.decide))
		n := b.chat("kit", office.Top, "the kitchen socket sparked")
		dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
		r := b.runner.next(t)
		goes := r.Input["referral_goes_to"].([]map[string]any)
		if len(goes) != 1 || goes[0]["to"] != "Sam" || !strings.Contains(r.Input["ends"].(string), c.want) {
			t.Fatalf("with %s the reviewer is told a referral goes to %v: %s", c.decide, goes, r.Input["ends"])
		}
		r.says(decisions.SaysRefer, "")
		b.until("the referral reaches whom it was told", func() bool { return slices.Equal(b.get(dec.ID).Asked, []string{"sam"}) })
		if starts := b.runner.started(); len(starts) != 1 {
			t.Fatalf("with %s, %d rules and workers were asked, not the one the reviewer was told of", c.decide, len(starts))
		}
	}

	// A person's own change: its last rule is told its referral is taken as
	// the person's yes, and it is.
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  change: [./no-secrets.js, sam]\n", 1)
	files["no-secrets.js"] = `return { says: "refer" }`
	own := newBench(t, files)
	p := own.putOwn("sam", "Kit helps in the garden")
	r := own.runner.next(t)
	goes := r.Input["referral_goes_to"].([]map[string]any)
	if len(goes) != 1 || goes[0]["to"] != "Sam" || !strings.Contains(r.Input["ends"].(string), "a referral is taken as their yes, and the change is written") {
		t.Fatalf("the rule on Sam's own change is told a referral goes to %v: %s", goes, r.Input["ends"])
	}
	r.says(decisions.SaysRefer, "")
	if o := own.outcome(p.ID); o.Ended != changes.Written || o.Decider != "sam" {
		t.Fatalf("Sam's own change ended %+v", o)
	}

	files = withDecide(t, "decide:\n  everything: [sam]")
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "decide: { everything: [sam] }", "decide: { everything: [./a.js] }", 1)
	b := newBench(t, files)
	g := b.chat("sam", "garden", "open the shed")
	b.one("ask-1", "garden", g, []string{"gardener"}, decisions.Job{Name: "open_door"})
	r = b.runner.next(t)
	goes = r.Input["referral_goes_to"].([]map[string]any)
	if len(goes) != 1 || goes[0]["team"] != office.Top || goes[0]["list"] != "everything" || goes[0]["to"] != "Sam" {
		t.Fatalf("the garden's rule is told a referral goes to %v", goes)
	}
}

func TestDecisionsWorkerNeverDecidesOwnAction(t *testing.T) {
	t.Parallel()
	b := newBench(t, withDecide(t, "decide:\n  everything: [sam]\n  cut_power: [fixer, sam]"))
	n := b.chat("kit", office.Top, "the kitchen socket sparked")
	dec := b.one("ask-1", office.Top, n, fixerUp, cutPower("4"))
	if len(b.runner.started()) != 0 {
		t.Fatalf("the fixer was asked to decide its own action")
	}
	if !slices.Equal(dec.Asked, []string{"sam"}) || len(dec.Referrals) != 1 || dec.Referrals[0].Name != "fixer" {
		t.Fatalf("the decision is %+v", dec)
	}
	has(t, b.thread(n), "referred by fixer: a worker never decides its own action")
}
