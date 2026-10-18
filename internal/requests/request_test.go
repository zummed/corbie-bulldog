package requests_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestRequestsAskedInTheChat(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	r := got[*requests.Request](t)(d.AskInChat("kit", "", "the kitchen socket sparked", nil))
	if r.Number == 0 || r.Team != office.Top || r.Asker != d.Person("kit") || r.Origin.Kind != requests.InChat {
		t.Fatalf("the request is %+v", r)
	}
	if d.count("requests_requests") != 1 {
		t.Error("more than one request is kept")
	}
	es := d.thread(r.Number)
	if len(es) != 1 || es[0].By != d.Person("kit") || !slices.Equal(es[0].To, []requests.Who{d.Team(office.Top)}) ||
		es[0].Text != "the kitchen socket sparked" || es[0].Kind != requests.KindAsked {
		t.Errorf("the thread is %+v", es)
	}
}

func TestRequestsNumbersGivenOnce(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	var numbers []int64
	for _, text := range []string{"one", "two", "three"} {
		numbers = append(numbers, got[*requests.Request](t)(d.AskInChat("kit", "", text, nil)).Number)
	}
	sub := got[*requests.Request](t)(d.Sub(numbers[0], d.Worker("steward"), "garden", "is the shed locked?", "t1"))
	numbers = append(numbers, sub.Number)
	d.restart()
	fifth := got[*requests.Request](t)(d.AskInChat("sam", "", "five", nil))
	numbers = append(numbers, fifth.Number)
	seen := map[int64]bool{}
	for _, n := range numbers {
		if seen[n] {
			t.Fatalf("a number is given twice: %v", numbers)
		}
		seen[n] = true
	}
	if r := got[*requests.Request](t)(d.Get(numbers[0])); r.Number != numbers[0] || requests.Labels(r)[requests.LabelRequest] != "1" {
		t.Errorf("the first request after the restart is %+v, labelled %v", r, requests.Labels(r))
	}
}

func TestRequestsGoToOneTeam(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	direct := got[*requests.Request](t)(d.AskAt("telegram", office.Direct, "4472", "4472", "hello", nil))
	garden := got[*requests.Request](t)(d.AskAt("telegram", "garden-chat", "garden-chat", "4472", "the hedge", nil))
	chat := got[*requests.Request](t)(d.AskInChat("kit", "", "the socket", nil))
	picked := got[*requests.Request](t)(d.AskInChat("kit", "garden", "the lawn", nil))
	friday := got[*requests.Request](t)(d.AskByRoutine("garden", d.o.Teams["garden"].Routines[0], requests.Who{}))
	for _, c := range []struct {
		r     *requests.Request
		team  string
		taker string
	}{{direct, office.Top, "steward"}, {garden, "garden", "gardener"}, {chat, office.Top, "steward"}, {picked, "garden", "gardener"}, {friday, "garden", "gardener"}} {
		taker, person, err := d.Taker(c.r.Number)
		if err != nil || c.r.Team != c.team || taker != c.taker || person {
			t.Errorf("request %d went to %s, taken by %s", c.r.Number, c.r.Team, taker)
		}
	}
}

func TestRequestsTakenByTakerOrRoutineWorker(t *testing.T) {
	t.Parallel()
	files := replace(t, house(t), "garden/team.yaml", `to: gardener, say: "What needs doing before the weekend?"`, `to: groundsman, say: "Mow?"`)
	d := open(t, files)
	kit := got[*requests.Request](t)(d.AskInChat("kit", "", "the socket", nil))
	monday := got[*requests.Request](t)(d.AskByRoutine(office.Top, d.o.TopTeam().Routines[0], requests.Who{}))
	mow := got[*requests.Request](t)(d.AskByRoutine("garden", d.o.Teams["garden"].Routines[0], requests.Who{}))
	for n, want := range map[int64]string{kit.Number: "steward", monday.Number: "steward", mow.Number: "groundsman"} {
		if taker, person, err := d.Taker(n); err != nil || taker != want || person {
			t.Errorf("request %d is taken by %s (%v, %v), not %s", n, taker, person, err, want)
		}
	}
	if monday.Asker != d.Team(office.Top) || monday.Origin.Kind != requests.ByRoutine || monday.Origin.At != "mon 09:00" {
		t.Errorf("the routine's request is %+v", monday)
	}
}

func TestRequestsPersonTakerWaitsAndAnswers(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	support := got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "support", "What did we tell 4411?", "ticket-support"))
	if taker, person, _ := d.Taker(support.Number); taker != "dana" || !person {
		t.Fatalf("support's sub-request is taken by %s", taker)
	}
	if st := d.state(support.Number); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"dana"}) {
		t.Fatalf("support's sub-request stands %+v", st)
	}
	must(t, errOf(d.Say(support.Number, d.Person("dana"), "That a refund is on its way.", nil, requests.Screens)))
	last := d.last(support.Number)
	if d.state(support.Number).Is != requests.Answered || last.Kind != requests.KindAnswer || last.By != d.Person("dana") || last.Text != "That a refund is on its way." {
		t.Errorf("support's sub-request ends %+v, standing %+v", last, d.state(support.Number))
	}
}

func errOf[T any](_ T, err error) error { return err }

func TestRequestsTeamWithNoTakerRefused(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	kit := got[*requests.Request](t)(d.AskInChat("kit", "", "the socket", nil)).Number
	kept := [2]int{d.count("requests_requests"), d.count("requests_entries")}
	_, err := d.AskInChat("kit", "billing", "a bill", nil)
	var refusal requests.Refusal
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "billing") {
		t.Errorf("a request to no team is not refused: %v", err)
	}
	// The garden's taker is no one, a person not of the garden, or a worker
	// of the house.
	for _, takes := range []string{"", "kit", "steward"} {
		o := *d.o
		o.Teams = map[string]*office.Team{}
		for name, team := range d.o.Teams {
			copied := *team
			o.Teams[name] = &copied
		}
		o.Teams["garden"].Takes = takes
		must(t, d.Read(&o))
		if _, err := d.AskInChat("kit", "garden", "the lawn", nil); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "takes") {
			t.Errorf("a request to the garden taken by %q is not refused: %v", takes, err)
		}
		if _, err := d.Sub(kit, d.Worker("steward"), "garden", "the lawn", "t-"+takes); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "takes") {
			t.Errorf("a sub-request of the garden taken by %q is not refused: %v", takes, err)
		}
	}
	// A taker that cannot run takes nothing, and a routine said to a worker
	// that cannot run asks nothing.
	broken := replace(t, house(t), "steward.yaml", "prompt: ${input.context}", "prompt: ${input.context")
	broken = replace(t, broken, "garden/gardener.yaml", "prompt: ${input.context}", "prompt: ${input.context")
	broken = replace(t, broken, "bins.yaml", "output: ${stages.which.output}", "output: ${stages.which.output")
	d.read(broken)
	for name, w := range map[string]*office.Worker{"steward": d.o.Workers["steward"], "gardener": d.o.Workers["gardener"], "bins": d.o.Workers["bins"]} {
		if w == nil || w.Runs() {
			t.Fatalf("the broken %s reads as %+v", name, w)
		}
	}
	for _, team := range []string{"", "garden"} {
		if _, err := d.AskInChat("kit", team, "the lawn", nil); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "cannot run") {
			t.Errorf("a request to %q, whose taker cannot run, is not refused: %v", team, err)
		}
	}
	if _, err := d.Sub(kit, d.Worker("steward"), "garden", "the lawn", "t-broken"); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "cannot run") {
		t.Errorf("a sub-request of the garden, whose taker cannot run, is not refused: %v", err)
	}
	for team, routine := range map[string]office.Routine{office.Top: d.o.TopTeam().Routines[1], "garden": d.o.Teams["garden"].Routines[0]} {
		if _, err := d.AskByRoutine(team, routine, requests.Who{}); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "said to "+routine.To+", which cannot run") {
			t.Errorf("%s's routine said to %s, which cannot run, is not refused: %v", team, routine.To, err)
		}
	}
	if now := [2]int{d.count("requests_requests"), d.count("requests_entries")}; now != kept {
		t.Errorf("%v rows are kept of what was refused, from %v", now, kept)
	}
}

func TestRequestsWorkersActInTheirOwnTeam(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	var refusal requests.Refusal
	before := [2]int{d.count("requests_requests"), d.count("requests_entries")}
	if _, err := d.Put(d.payments, d.Worker("ops"), requests.KindQuestion, d.Person("lee"), "which charge?", nil); !errors.As(err, &refusal) {
		t.Errorf("ops putting a question in payments' sub-request is told %v", err)
	}
	if _, err := d.Sub(d.payments, d.Worker("ops"), "support", "what was said?", "ticket-ops"); !errors.As(err, &refusal) {
		t.Errorf("ops asking from payments' sub-request is told %v", err)
	}
	for _, by := range []requests.Who{d.Worker("ops"), d.Worker("bot"), d.Person("lee")} {
		if _, err := d.Answer(d.payments, by, "done", nil); !errors.As(err, &refusal) {
			t.Errorf("%+v answering payments' sub-request is told %v", by, err)
		}
	}
	if now := [2]int{d.count("requests_requests"), d.count("requests_entries")}; now != before {
		t.Errorf("what was refused left %v rows, from %v", now, before)
	}
	// A routine's request is answered by the worker the routine names.
	files := replace(t, house(t), "garden/team.yaml", `to: gardener, say: "What needs doing before the weekend?"`, `to: groundsman, say: "Mow?"`)
	h := open(t, files)
	mow := got[*requests.Request](t)(h.AskByRoutine("garden", h.o.Teams["garden"].Routines[0], requests.Who{})).Number
	if _, err := h.Answer(mow, h.Worker("gardener"), "Mown.", nil); !errors.As(err, &refusal) {
		t.Errorf("the gardener answering the groundsman's routine is told %v", err)
	}
	must(t, errOf(h.Answer(mow, h.Worker("groundsman"), "Mown.", nil)))
	// Who may not say more is told why.
	for who, words := range map[requests.Who]string{d.Worker("ops"): "of workers, only those of the team that asked it",
		d.Team("platform"): "only people do", requests.Bulldog: "only people do", d.Rule("payments/no-account-numbers.yaml"): "only people do"} {
		if _, err := d.Say(d.payments, who, "me too", nil, requests.Where{}); !errors.As(err, &refusal) || !strings.Contains(err.Error(), words) {
			t.Errorf("%+v saying more in payments' sub-request is told %v", who, err)
		}
	}
	if r := d.Rule("payments/no-account-numbers.yaml"); r.Kind != requests.IsRule || r.Team != "payments" || r.Known == 0 {
		t.Errorf("payments' release rule is %+v", r)
	}
}

func TestRequestsAskerIsTheCheckedIdentity(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	r := got[*requests.Request](t)(d.Get(d.dana))
	if r.Asker != requests.Identity("slack", slackDana) {
		t.Errorf("Dana's request is asked by %+v", r.Asker)
	}
	h := open(t, house(t))
	monday := got[*requests.Request](t)(h.AskByRoutine(office.Top, h.o.TopTeam().Routines[0], requests.Who{}))
	if monday.Asker != h.Team(office.Top) {
		t.Errorf("the house's Monday routine is asked by %+v", monday.Asker)
	}
}

func TestRequestsAskerReadEachTime(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	if p := got[requests.Party](t)(d.Asks(d.dana)); p.Kind != requests.IsPerson || p.Name != "dana" || !d.sees(d.dana, "dana") {
		t.Fatalf("Dana's request is asked by %+v", p)
	}
	files := d.files
	d.read(replace(t, files, "office.yaml", "dana:  { name: Dana, title: Support., slack: U0DANA00 }", "dana:  { name: Dana, title: Support. }"))
	if p := got[requests.Party](t)(d.Asks(d.dana)); p.Kind != requests.IsGuest || p.Bridge != "slack" || p.ID != slackDana {
		t.Errorf("with her identity gone, Dana's request is asked by %+v", p)
	}
	if d.sees(d.dana, "dana") {
		t.Error("Dana still sees her request as its asker")
	}
	d.read(files)
	if !d.sees(d.dana, "dana") {
		t.Error("with her identity written back, Dana does not see her request")
	}
}

func TestRequestsReusedNameInheritsNothing(t *testing.T) {
	t.Parallel()
	files := house(t)
	d := open(t, files)
	kit := got[*requests.Request](t)(d.AskInChat("kit", "", "the socket", nil)).Number
	gone := replace(t, files, "office.yaml", "  kit: { name: Kit, title: Lives here., telegram: \"4472\" }\n", "")
	gone = replace(t, gone, "office.yaml", "people: [sam, kit]", "people: [sam]")
	d.read(gone)
	d.read(replace(t, files, "office.yaml", "people: [sam, kit]", "people: [sam]"))
	if d.sees(kit, "kit") {
		t.Error("a new Kit sees the old Kit's request")
	}
	if list := got[[]requests.Summary](t)(d.List("kit", requests.Filter{Asked: true})); len(list) != 0 {
		t.Errorf("a new Kit asked %+v", list)
	}
	if p := got[requests.Party](t)(d.Asks(kit)); p.Kind != requests.IsGone || p.Name != "kit" {
		t.Errorf("the old Kit's request is asked by %+v", p)
	}
	// A person called Bulldog is not Bulldog, and a worker is no person.
	d.read(replace(t, files, "office.yaml", "everyone:\n", "everyone:\n  bull: { name: Bulldog }\n"))
	r := got[*requests.Request](t)(d.AskInChat("sam", "", "the bins", nil)).Number
	must(t, d.Failed(r, "boom"))
	for _, e := range d.thread(r) {
		if e.Kind == requests.KindBulldog && (e.By.Kind != requests.IsBulldog || e.By == d.Person("bull")) {
			t.Errorf("Bulldog's entry is by %+v", e.By)
		}
	}
	if d.Worker("steward") == d.Person("steward") || d.Worker("steward").Kind != requests.IsWorker {
		t.Error("a worker is kept as a person")
	}
}

func TestRequestsRunToldWhoAsks(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	if p := got[requests.Party](t)(d.Asks(d.dana)); p.Kind != requests.IsPerson || p.Called != "Dana" || !slices.Equal(p.Teams, []string{"support"}) {
		t.Errorf("Dana's request is asked by %+v", p)
	}
	guest := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.02", "U0STRANGE", "hello?", nil))
	if p := got[requests.Party](t)(d.Asks(guest.Number)); p.Kind != requests.IsGuest || p.Bridge != "slack" || p.ID != "U0STRANGE" {
		t.Errorf("the guest's request is asked by %+v", p)
	}
	if p := got[requests.Party](t)(d.Asks(d.payments)); p.Kind != requests.IsWorker || p.Name != "bot" || p.Team != office.Top {
		t.Errorf("payments' sub-request is asked by %+v", p)
	}
	monday := got[*requests.Request](t)(d.AskByRoutine(office.Top, office.Routine{At: "mon 09:00", To: "bot", Say: "What is open?"}, requests.Who{}))
	if p := got[requests.Party](t)(d.Asks(monday.Number)); p.Kind != requests.IsTeam || p.Name != office.Top {
		t.Errorf("the routine's request is asked by %+v", p)
	}
}

func TestRequestsGuestJudgedByTheTeam(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	guest := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.02", "U0STRANGE", "refund me", nil))
	if taker, _, _ := d.Taker(guest.Number); taker != "bot" || d.state(guest.Number).Is != requests.Working {
		t.Errorf("the guest's request is taken by %s, standing %+v", taker, d.state(guest.Number))
	}
	// What needs a yes for a guest waits on the team's deciders, as for anyone.
	e := got[*requests.Entry](t)(d.Write(guest.Number, requests.Writing{Kind: "decision", By: d.Worker("bot"), To: []requests.Who{d.Person("al")},
		Text: "Refund a charge", Waits: []requests.Who{d.Person("al")}}))
	if st := d.state(guest.Number); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"al"}) || e == nil {
		t.Errorf("the guest's request stands %+v", st)
	}
}

func TestRequestsOriginKept(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	r := got[*requests.Request](t)(d.Get(d.dana))
	if r.Origin != (requests.Origin{Kind: requests.AtPlace, Bridge: "slack", Heard: "#help", Place: thread}) {
		t.Errorf("Dana's request's origin is %+v", r.Origin)
	}
	answer := got[*requests.Entry](t)(d.Answer(d.dana, d.Worker("bot"), "Refunded; MR 812 broke it.", nil))
	if ds := got[[]requests.Delivery](t)(d.Deliveries(answer.ID)); len(ds) != 1 || ds[0].Origin == nil || *ds[0].Origin != r.Origin {
		t.Errorf("the bot's answer goes to %+v", ds)
	}
	sub := got[*requests.Request](t)(d.Get(d.payments))
	if sub.Origin.Kind != "" || sub.Parent != d.dana {
		t.Errorf("payments' sub-request has the origin %+v", sub.Origin)
	}
	ledger := got[*requests.Entry](t)(d.Answer(d.payments, d.Worker("ledger"), "Duplicate confirmed; £40 refunded.", nil))
	if ds := got[[]requests.Delivery](t)(d.Deliveries(ledger.ID)); len(ds) != 0 {
		t.Errorf("payments' answer is carried to %+v", ds)
	}
	// Read for a whole request at once, each entry goes where it goes
	// alone.
	for _, n := range []int64{d.dana, d.payments} {
		thread, all, err := d.DeliveriesIn(n)
		must(t, err)
		if len(thread) == 0 || len(all) != len(thread) {
			t.Errorf("request %d's deliveries are %+v for %d entries", n, all, len(thread))
		}
		for _, e := range thread {
			if one := got[[]requests.Delivery](t)(d.Deliveries(e.ID)); !reflect.DeepEqual(all[e.ID], one) {
				t.Errorf("entry %d goes to %+v, and alone to %+v", e.ID, all[e.ID], one)
			}
		}
	}
	h := open(t, house(t))
	kit := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil))
	steward := got[*requests.Entry](t)(h.Answer(kit.Number, h.Worker("steward"), "Fixed.", nil))
	if ds := got[[]requests.Delivery](t)(h.Deliveries(steward.ID)); len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Kind != requests.InChat {
		t.Errorf("the steward's answer goes to %+v", ds)
	}
}

func TestRequestsOriginReaders(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	for _, p := range []string{"dana", "al", "raj"} {
		if !d.sees(d.dana, p) || !d.sees(d.payments, p) {
			t.Errorf("%s does not see Dana's request", p)
		}
	}
	must(t, d.Readers("slack", thread, []string{slackDana, slackAl, "U0STRANGE"}))
	var readers []string
	for _, p := range got[[]requests.Party](t)(d.ReadBy(d.dana)) {
		readers = append(readers, p.Name)
	}
	if !slices.Equal(readers, []string{"dana", "al"}) {
		t.Errorf("Dana's thread is read by %v", readers)
	}
	if d.sees(d.dana, "raj") {
		t.Error("Raj still sees Dana's request")
	}
	must(t, d.Readers("slack", thread, nil))
	if d.sees(d.dana, "raj") || !d.sees(d.dana, "dana") {
		t.Error("a report of no readers changed who reads the thread")
	}
	other := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.09", slackAl, "who reads this?", nil))
	if d.sees(other.Number, "raj") || d.sees(other.Number, "dana") || !d.sees(other.Number, "al") {
		t.Error("a thread never reported is read by more than its asker")
	}
	chat := got[*requests.Request](t)(d.AskInChat("dana", "", "in the chat", nil))
	if d.sees(chat.Number, "raj") || d.sees(chat.Number, "lee") || !d.sees(chat.Number, "dana") {
		t.Error("the chat is read by more than its asker")
	}
	h := open(t, house(t))
	friday := got[*requests.Request](t)(h.AskByRoutine("garden", h.o.Teams["garden"].Routines[0], requests.Who{}))
	if !h.sees(friday.Number, "sam") || h.sees(friday.Number, "kit") {
		t.Error("the garden's routine is not read by the garden's people alone")
	}
}

func TestRequestsSaidToAskerGoesToOrigin(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	kit := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil))
	q := got[*requests.Entry](t)(h.Put(kit.Number, h.Worker("fixer"), requests.KindQuestion, h.Person("kit"), "tell me when it arrives", nil))
	if q.Read != "" {
		t.Errorf("a question to the asker is what was read of %s", q.Read)
	}
	if ds := got[[]requests.Delivery](t)(h.Deliveries(q.ID)); len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Kind != requests.InChat {
		t.Errorf("the fixer's question goes to %+v", ds)
	}
	d := workDay(t)
	q = got[*requests.Entry](t)(d.Put(d.dana, d.Worker("bot"), requests.KindQuestion, requests.Identity("slack", slackDana), "which customer?", nil))
	if ds := got[[]requests.Delivery](t)(d.Deliveries(q.ID)); len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Place != thread {
		t.Errorf("the bot's question to Dana goes to %+v", ds)
	}
}

func TestRequestsEntryHoldsWhatItIs(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	d.clock.Move(5 * time.Hour)
	lee := requests.Identity("slack", slackLee)
	in := requests.Where{Bridge: "slack", Place: office.Direct}
	e := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "decided", By: lee, To: []requests.Who{d.Worker("ledger")}, Text: "yes", In: in}))
	kept := d.last(d.payments)
	if kept.ID != e.ID || kept.Request != d.payments || kept.By != lee || !slices.Equal(kept.To, []requests.Who{d.Worker("ledger")}) ||
		!kept.At.Equal(testkit.Monday.Add(5*time.Hour)) || kept.In != in || kept.Read != "" || kept.Text != "yes" {
		t.Errorf("Lee's yes is kept as %+v", kept)
	}
}

func TestRequestsEntriesInOrderNeverChanged(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "one", nil)).Number
	for _, text := range []string{"two", "three", "four", "five"} {
		must(t, errOf(h.Say(n, h.Person("kit"), text, nil, requests.Screens)))
	}
	var texts []string
	for _, e := range h.thread(n) {
		texts = append(texts, e.Text)
	}
	if !slices.Equal(texts, []string{"one", "two", "three", "four", "five"}) {
		t.Errorf("the thread reads %v", texts)
	}
	store := reflect.TypeOf(h.Store)
	for i := range store.NumMethod() {
		name := store.Method(i).Name
		for _, word := range []string{"Edit", "Remove", "Delete", "Change", "Update", "Unsay"} {
			if strings.Contains(name, word) {
				t.Errorf("the store has %s", name)
			}
		}
	}
}

func TestRequestsFilesSeenWithTheirEntry(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	statement := requests.File{Name: "statement.pdf", Data: []byte("%PDF 4411")}
	e := got[*requests.Entry](t)(d.Say(d.payments, d.Person("lee"), "the statement", []requests.File{statement}, requests.Screens))
	if len(e.Files) != 1 || e.Files[0].Name != "statement.pdf" || e.Read != "payments" {
		t.Fatalf("Lee's entry is %+v", e)
	}
	for _, p := range []string{"lee", "priya"} {
		if data := got[[]byte](t)(d.File(d.payments, e.ID, "statement.pdf", p)); string(data) != "%PDF 4411" {
			t.Errorf("%s is given %q", p, data)
		}
	}
	if _, err := d.File(d.payments, e.ID, "statement.pdf", "al"); !errors.Is(err, requests.ErrNotThere) {
		t.Errorf("Al asking for the statement is told %v", err)
	}
	if _, err := d.File(d.payments, e.ID, "nothing.pdf", "al"); !errors.Is(err, requests.ErrNotThere) {
		t.Errorf("Al asking for nothing is told %v", err)
	}
	if holds(d.shows(d.payments, "al"), "the statement") {
		t.Error("Al sees Lee's entry")
	}
}

func TestRequestsBulldogEntries(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	must(t, h.Failed(n, "turn limit reached: secret LEDGER_KEY"))
	es := h.thread(n)
	team, asker := es[1], es[2]
	if team.By != requests.Bulldog || team.Read != office.Top || !strings.Contains(team.Text, "LEDGER_KEY") {
		t.Errorf("Bulldog's entry to the house is %+v", team)
	}
	if asker.By != requests.Bulldog || asker.Read != "" || asker.To[0] != h.Person("kit") || strings.Contains(asker.Text, "LEDGER") ||
		asker.Text != "The house could not answer." {
		t.Errorf("Bulldog's entry to Kit is %+v", asker)
	}
}

func TestRequestsTooLargeRefusedNeverCut(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	big := strings.Repeat("x", requests.MaxText+1)
	_, err := h.AskInChat("kit", "", big, nil)
	var refusal requests.Refusal
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "1048576") {
		t.Errorf("a text too large is told %v", err)
	}
	if h.count("requests_requests") != 0 {
		t.Error("a request is kept of a text too large")
	}
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	huge := requests.File{Name: "photo.jpg", Data: make([]byte, requests.MaxFile+1)}
	if _, err := h.Say(n, h.Person("kit"), "a photo", []requests.File{huge}, requests.Screens); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "photo.jpg") {
		t.Errorf("a file too large is told %v", err)
	}
	if _, err := h.Say(n, h.Person("kit"), big, nil, requests.Screens); !errors.As(err, &refusal) {
		t.Errorf("a reply too large is told %v", err)
	}
	if len(h.thread(n)) != 1 || h.count("requests_files") != 0 {
		t.Error("something of what was too large is kept")
	}
}

func TestRequestsRefusedEntryLeavesNoFile(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	good := requests.File{Name: "a.jpg", Data: []byte("a")}
	huge := requests.File{Name: "b.jpg", Data: make([]byte, requests.MaxFile+1)}
	if _, err := h.Say(n, h.Person("kit"), "photos", []requests.File{good, huge}, requests.Screens); err == nil {
		t.Fatal("an entry with a file too large is kept")
	}
	_, err := h.db.Exec(`CREATE TRIGGER fail_file BEFORE INSERT ON requests_files WHEN NEW.name = 'bad.jpg' BEGIN SELECT RAISE(ABORT, 'the disk is full'); END`)
	must(t, err)
	if _, err := h.Say(n, h.Person("kit"), "photos", []requests.File{good, {Name: "bad.jpg", Data: []byte("b")}}, requests.Screens); err == nil {
		t.Fatal("an entry whose file cannot be written is kept")
	}
	if len(h.thread(n)) != 1 || h.count("requests_files") != 0 {
		t.Errorf("a refused entry left %d entries and %d files", len(h.thread(n)), h.count("requests_files"))
	}
}

func TestRequestsTimesInOfficeZone(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	e := h.thread(n)[0]
	if e.At.Location().String() != "Europe/London" || !e.At.Equal(testkit.Monday) || e.At.Hour() != 10 {
		t.Errorf("the entry is at %v", e.At)
	}
	if v := got[*requests.View](t)(h.View(n, "kit")); v.Made.Location().String() != "Europe/London" || v.Entries[0].At.Location().String() != "Europe/London" {
		t.Errorf("the view shows %v", v.Made)
	}
	var kept string
	must(t, h.db.QueryRow(`SELECT at FROM requests_entries`).Scan(&kept))
	if at, err := time.Parse(time.RFC3339Nano, kept); err != nil || !at.Equal(testkit.Monday) || !strings.HasSuffix(kept, "Z") {
		t.Errorf("the entry is kept at %s", kept)
	}
	// A status at the source is shown in the office's zone, however it was
	// reported.
	d := open(t, work(t))
	issue := "platform/api#51"
	r := got[*requests.Request](t)(d.AskAt("gitlab", "platform/api", issue, gitlabAl, "the retry is broken", nil)).Number
	reported := d.clock.Now().In(time.FixedZone("EST", -5*3600))
	must(t, errOf(d.Source("gitlab", issue, requests.Status{Status: "opened", By: requests.Identity("gitlab", gitlabAl), At: reported})))
	v := got[*requests.View](t)(d.View(r, "al"))
	list := got[[]requests.Summary](t)(d.List("al", requests.Filter{}))
	kept2 := got[*requests.Request](t)(d.Get(r))
	for what, at := range map[string]time.Time{"view": v.Source.At, "list": list[0].Source.At, "request": kept2.Source.At, "made": kept2.Made} {
		if at.Location().String() != "Europe/London" || !at.Equal(reported) {
			t.Errorf("the %s shows %v", what, at)
		}
	}
	var source string
	must(t, d.db.QueryRow(`SELECT source FROM requests_requests`).Scan(&source))
	if !strings.Contains(source, "Z\"") {
		t.Errorf("the status is kept as %s", source)
	}
}
