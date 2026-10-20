package requests_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// decide puts a decision to people in a request: what was done holds the
// action as its tool describes it, and what a decider is shown, the
// arguments among it, is what was read of the team, said to them.
func (d *desk) decide(n int64, worker, action, shown string, people ...string) *requests.Entry {
	d.t.Helper()
	var whos []requests.Who
	for _, p := range people {
		whos = append(whos, d.Person(p))
	}
	record := got[*requests.Entry](d.t)(d.Write(n, requests.Writing{Kind: "decision", By: d.Worker(worker), To: whos, Text: action, Waits: whos}))
	must(d.t, errOf(d.Write(n, requests.Writing{Kind: "shown", By: requests.Bulldog, To: whos, Text: shown, Read: true})))
	return record
}

func TestRequestsTwoLayers(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	al := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.05", "", slackAl, "is the deploy done?", nil, false)).Number
	if first := d.thread(al)[0]; first.Read != "" {
		t.Errorf("Al's asking in #help is what was read of %s", first.Read)
	}
	if q := got[*requests.Entry](t)(d.Put(d.dana, d.Worker("bot"), requests.KindQuestion, d.Person("al"), "may I refund?", nil)); q.Read != office.Top {
		t.Errorf("the bot's question to Al is what was read of %q", q.Read)
	}
	record := d.decide(d.payments, "ledger", "Refund one charge", "the ledger's reasons: two charges of £40", "lee")
	if record.Read != "" || d.last(d.payments).Read != "payments" {
		t.Error("the decision's record and what a decider is shown are not in their layers")
	}
	// Payments asks platform something: until the disclosure lets it go, the
	// question is what was read of payments, and there is no sub-request.
	held := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "disclosure", By: d.Worker("ledger"), To: []requests.Who{d.Team("platform")},
		Text: "When did the retry break?", Read: true, Waits: []requests.Who{d.Person("lee")}}))
	for _, p := range []string{"al", "dana"} {
		if v := got[*requests.View](t)(d.View(d.payments, p)); len(v.Subs) != 0 || holds(d.shows(d.payments, p), "retry") {
			t.Errorf("%s sees %+v beneath payments, and %q", p, v.Subs, d.shows(d.payments, p))
		}
	}
	if !holds(d.shows(d.payments, "lee"), "retry") {
		t.Error("Lee does not see payments' held question")
	}
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "disclosed", By: d.Person("lee"), To: []requests.Who{d.Worker("ledger")}, Text: "yes",
		Read: true, Answers: held.ID})))
	below := got[*requests.Request](t)(d.Sub(d.payments, d.Worker("ledger"), "platform", "When did the retry break?", "ticket-below")).Number
	for _, p := range []string{"al", "dana"} {
		if v := got[*requests.View](t)(d.View(d.payments, p)); len(v.Subs) != 1 || v.Subs[0].Number != below {
			t.Errorf("once let go, %s sees %+v beneath payments", p, v.Subs)
		}
	}
}

func TestRequestsDecisionShowsNoArguments(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	record := d.decide(d.payments, "ledger", "Refund one charge", "refund(card: 4111 1111 1111 1111, pounds: 40); the ledger: a duplicate", "lee", "priya")
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decided", By: requests.Identity("slack", slackLee), To: []requests.Who{d.Worker("ledger")},
		Text: "Refund one charge: approved by Lee, 14:02", Answers: record.ID})))
	for _, p := range []string{"dana", "al"} {
		shown := d.shows(d.payments, p)
		if !holds(shown, "Refund one charge: approved by Lee, 14:02") || holds(shown, "4111") || holds(shown, "duplicate") {
			t.Errorf("%s is shown %q", p, shown)
		}
	}
	if shown := d.shows(d.payments, "lee"); !holds(shown, "4111") || !holds(shown, "approved by Lee") {
		t.Errorf("Lee is shown %q", shown)
	}
}

func TestRequestsHeldNotDone(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	disclosure := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "disclosure", By: d.Worker("ledger"),
		To: []requests.Who{d.Person("lee"), d.Person("priya")}, Text: "the ledger would answer: refunded card 4111", Read: true,
		Waits: []requests.Who{d.Person("lee"), d.Person("priya")}}))
	for _, p := range []string{"al", "dana"} {
		v := got[*requests.View](t)(d.View(d.dana, p))
		if v.Subs[0].State.Is != requests.Waiting || !slices.Equal(on(v.Subs[0].State), []string{"lee", "priya"}) || v.Subs[0].Answer != "" || holds(d.shows(d.payments, p), "4111") {
			t.Errorf("%s sees payments' sub-request as %+v", p, v.Subs[0])
		}
	}
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "refused", By: d.Person("lee"), To: []requests.Who{d.Worker("ledger")},
		Text: "no card numbers in an answer", Read: true, Answers: disclosure.ID, Working: true})))
	for _, p := range []string{"al", "dana"} {
		if shown := d.shows(d.payments, p); holds(shown, "4111") || holds(shown, "no card numbers") {
			t.Errorf("%s is shown %q", p, shown)
		}
	}
}

func TestRequestsRefusedDisclosure(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	// The ledger's answer is held, and put to Lee, as the disclosure check
	// puts it: it was never the request's answer.
	held := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "disclosure", By: d.Worker("ledger"), To: []requests.Who{d.Person("lee")},
		Text: "the ledger would answer: refunded card 4111", Read: true, Waits: []requests.Who{d.Person("lee")}}))
	refusal := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "refused", By: d.Person("lee"), To: []requests.Who{d.Worker("ledger")},
		Text: "no card numbers in an answer", Read: true, Answers: held.ID, Working: true}))
	if d.state(d.payments).Is != requests.Working || refusal.Read != "payments" {
		t.Errorf("after the refusal payments' sub-request stands %+v", d.state(d.payments))
	}
	for p, sees := range map[string]bool{"lee": true, "priya": true, "al": false, "dana": false} {
		shown := d.shows(d.payments, p)
		if holds(shown, "no card numbers") != sees || holds(shown, "refunded card 4111") != sees {
			t.Errorf("whether %s sees the refused answer and its refusal is not %v: %q", p, sees, shown)
		}
	}
	for _, p := range []string{"al", "dana"} {
		if v := got[*requests.View](t)(d.View(d.dana, p)); v.Subs[0].Answer != "" || v.Subs[0].State.Is != requests.Working {
			t.Errorf("%s sees payments' sub-request as %+v", p, v.Subs[0])
		}
	}
	// A lead who is a person, of a team that sets a disclosure check, has
	// their answer held as what was read of the team, and answered only
	// once the check lets it go.
	files := replace(t, work(t), "support/team.yaml", "lead: dana", "lead: dana\ndisclosure: [./check.js]")
	files["support/check.js"] = `return { says: "yes" }`
	w := open(t, files)
	must(t, w.Readers("slack", thread, []string{slackDana, slackAl}))
	dana := got[*requests.Request](t)(w.AskAt("slack", "#help", thread, "", slackDana, "help", nil, false)).Number
	support := got[*requests.Request](t)(w.Sub(dana, w.Worker("bot"), "support", "What did we tell 4411?", "t")).Number
	answer := got[*requests.Entry](t)(w.Say(support, w.Person("dana"), "a refund", []requests.File{{Name: "note.txt", Data: []byte("4411")}}, requests.Screens, false))
	if answer.Kind != requests.KindHeld || answer.Read != "support" || w.state(support).Is != requests.Working || holds(w.shows(support, "al"), "a refund") {
		t.Errorf("Dana's answer through a disclosure check is %+v, standing %+v", answer, w.state(support))
	}
	if ds := got[[]requests.Delivery](t)(w.Deliveries(answer.ID)); len(ds) != 0 {
		t.Errorf("the held answer is carried to %+v", ds)
	}
	must(t, errOf(w.Write(support, requests.Writing{Kind: "refused", By: w.Rule("support/check.js", "support"), To: []requests.Who{w.Person("dana")},
		Text: "say which refund", Read: true, Working: true})))
	if st := w.state(support); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"dana"}) {
		t.Errorf("after the refusal support's sub-request stands %+v", st)
	}
	again := got[*requests.Entry](t)(w.Say(support, w.Person("dana"), "the £40 refund", []requests.File{{Name: "note.txt", Data: []byte("4411")}}, requests.Screens, false))
	var refused requests.Refusal
	if _, err := w.Disclose(answer.ID); !errors.As(err, &refused) {
		t.Errorf("letting go an answer said again is told %v", err)
	}
	let := got[*requests.Entry](t)(w.Disclose(again.ID))
	if let.Kind != requests.KindAnswer || let.By != w.Person("dana") || let.Text != "the £40 refund" || len(let.Files) != 1 || let.Read != "" ||
		w.state(support).Is != requests.Answered || !holds(w.shows(support, "al"), "the £40 refund") {
		t.Errorf("the answer let go is %+v, standing %+v", let, w.state(support))
	}
	if data := got[[]byte](t)(w.File(support, let.ID, "note.txt", "al")); string(data) != "4411" {
		t.Errorf("the answer let go holds %q", data)
	}
}

func TestRequestsAnswerReadInsideItsTeamNotHeld(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "payments/team.yaml", "lead: ledger", "lead: lee")
	d := open(t, replace(t, files, "office.yaml", `"#help": { guests: true }`,
		`"#help": { guests: true }`+"\n      \"#refunds\": { team: payments }\n      \"#cards\": { team: payments, guests: true }"))
	// Priya asks payments in the chat, in a thread of #refunds only
	// payments reads, and in one whose readers were never reported, where
	// no guest may ask: Lee's answer is read only inside payments, and is
	// its answer at once.
	must(t, d.Readers("slack", "#refunds/1", []string{slackPriya, slackLee}))
	chat := got[*requests.Request](t)(d.AskInChat("priya", "payments", "was 4411 charged twice?", nil)).Number
	inside := got[*requests.Request](t)(d.AskAt("slack", "#refunds", "#refunds/1", "", slackPriya, "was 4412 charged twice?", nil, false)).Number
	unreported := got[*requests.Request](t)(d.AskAt("slack", "#refunds", "#refunds/3", "", slackPriya, "was 4414 charged twice?", nil, false)).Number
	for _, n := range []int64{chat, inside, unreported} {
		if e := got[*requests.Entry](t)(d.Say(n, d.Person("lee"), "yes, on card 4111", nil, requests.Screens, false)); e.Kind != requests.KindAnswer {
			t.Errorf("Lee's answer in request %d is %+v", n, e)
		}
	}
	// In a thread that someone the office cannot name reads too, Lee's
	// answer to Priya leaves payments, and is held.
	must(t, d.Readers("slack", "#refunds/2", []string{slackPriya, "U0STRANGE"}))
	outside := got[*requests.Request](t)(d.AskAt("slack", "#refunds", "#refunds/2", "", slackPriya, "was 4413 charged twice?", nil, false)).Number
	if e := got[*requests.Entry](t)(d.Say(outside, d.Person("lee"), "yes, on card 4111", nil, requests.Screens, false)); e.Kind != requests.KindHeld {
		t.Errorf("Lee's answer read outside payments is %+v", e)
	}
	// In a thread of #cards, where guests may ask, whose readers were
	// never reported, any guest may read Lee's answer, and it is held.
	anyone := got[*requests.Request](t)(d.AskAt("slack", "#cards", "#cards/1", "", slackPriya, "was 4415 charged twice?", nil, false)).Number
	if e := got[*requests.Entry](t)(d.Say(anyone, d.Person("lee"), "yes, on card 4111", nil, requests.Screens, false)); e.Kind != requests.KindHeld {
		t.Errorf("Lee's answer where any guest may read it is %+v", e)
	}
}

func TestRequestsWhatWasReadFromCorbie(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	var asked []map[string]string
	d.Runs = func(labels map[string]string) ([]requests.Record, error) {
		asked = append(asked, labels)
		return []requests.Record{{Run: "ledger-1", Team: labels[requests.LabelTeam], Worker: "ledger", Events: []byte(`[{"said":"4411"}]`)},
			{Run: "ledger-0", Team: labels[requests.LabelTeam], Gone: true}}, nil
	}
	records := got[[]requests.Record](t)(d.WhatWasRead(d.payments, "lee"))
	sub := got[*requests.Request](t)(d.Get(d.payments))
	if len(records) != 2 || records[1].Gone != true || len(asked) != 1 || asked[0][requests.LabelTeam] != "payments" ||
		asked[0][requests.LabelRequest] != requests.Labels(sub)[requests.LabelRequest] {
		t.Errorf("Lee is given %+v, read by %v", records, asked)
	}
	if records := got[[]requests.Record](t)(d.WhatWasRead(d.payments, "al")); len(records) != 0 || len(asked) != 1 {
		t.Errorf("Al is given %+v", records)
	}
	if _, err := d.WhatWasRead(d.payments, "nobody"); !errors.Is(err, requests.ErrNotThere) {
		t.Errorf("someone who sees nothing of it is told %v", err)
	}
	labels := requests.Labels(sub)
	if !got[bool](t)(d.MayWatch(labels, "lee")) || got[bool](t)(d.MayWatch(labels, "al")) {
		t.Error("the runs of payments' sub-request are watched by others than its people")
	}
	labels[requests.LabelTeam] = office.Top
	if got[bool](t)(d.MayWatch(labels, "al")) {
		t.Error("a run labelled with another team than its request's is watched")
	}
	// The bot deciding payments' refund runs as payments: what it read is
	// payments', seen by Lee and Priya and not by Al.
	d.Runs = func(labels map[string]string) ([]requests.Record, error) {
		return []requests.Record{{Run: "bot-decides", Team: labels[requests.LabelTeam], Worker: "bot", Events: []byte(`"two charges"`)}}, nil
	}
	for p, sees := range map[string]bool{"lee": true, "priya": true, "al": false} {
		records := got[[]requests.Record](t)(d.WhatWasRead(d.payments, p))
		if (len(records) == 1 && records[0].Worker == "bot") != sees || got[bool](t)(d.MayWatch(requests.Labels(sub), p)) != sees {
			t.Errorf("whether %s sees what the bot read deciding for payments is not %v", p, sees)
		}
	}
	// Platform has no people: Al, whom its decisions reach, watches its
	// runs, and Dana does not.
	platform := requests.Labels(got[*requests.Request](t)(d.Get(d.platform)))
	if !got[bool](t)(d.MayWatch(platform, "al")) || got[bool](t)(d.MayWatch(platform, "dana")) {
		t.Error("platform's runs are not watched by Al alone")
	}
}

func TestRequestsNoCopyOfRuns(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	said := "first"
	d.Runs = func(map[string]string) ([]requests.Record, error) {
		return []requests.Record{{Run: "ledger-1", Events: []byte(`"` + said + `"`)}}, nil
	}
	if r := got[[]requests.Record](t)(d.WhatWasRead(d.payments, "lee")); string(r[0].Events) != `"first"` {
		t.Fatalf("Lee is given %s", r[0].Events)
	}
	said = "second"
	if r := got[[]requests.Record](t)(d.WhatWasRead(d.payments, "lee")); string(r[0].Events) != `"second"` {
		t.Errorf("Lee is given %s, as kept", r[0].Events)
	}
	rows, err := d.db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'requests%' ORDER BY name`)
	must(t, err)
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		must(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	want := []string{"requests_entries", "requests_files", "requests_known", "requests_readers", "requests_requests", "requests_waits"}
	if !slices.Equal(tables, want) {
		t.Errorf("the requests keep the tables %v", tables)
	}
}

func TestRequestsWhoSeesWhatWasDone(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	for _, p := range []string{"dana", "al", "raj"} {
		for _, n := range []int64{d.dana, d.payments, d.platform} {
			if !d.sees(n, p) {
				t.Errorf("%s does not see request %d", p, n)
			}
		}
	}
	for _, p := range []string{"lee", "priya"} {
		if !d.sees(d.payments, p) || d.sees(d.dana, p) || d.sees(d.platform, p) {
			t.Errorf("%s does not see payments' sub-request alone", p)
		}
	}
}

func TestRequestsWhatWasReadSeenByTeamPeople(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	d.decide(d.payments, "ledger", "Refund one charge", "the ledger's conversation", "lee", "priya")
	for p, sees := range map[string]bool{"lee": true, "priya": true, "al": false, "dana": false} {
		if holds(d.shows(d.payments, p), "the ledger's conversation") != sees {
			t.Errorf("whether %s sees what payments read is not %v", p, sees)
		}
	}
	if people := got[[]string](t)(d.People("platform")); !slices.Equal(people, []string{"al"}) {
		t.Errorf("platform's people are %v", people)
	}
	must(t, errOf(d.Write(d.platform, requests.Writing{Kind: "said", By: d.Worker("ops"), To: []requests.Who{d.Team("platform")}, Text: "ops read the pipeline", Read: true})))
	must(t, d.Failed(d.platform, "the pipeline API said 500"))
	for p, sees := range map[string]bool{"al": true, "dana": false} {
		if holds(d.shows(d.platform, p), "ops read the pipeline") != sees || holds(d.shows(d.platform, p), "said 500") != sees {
			t.Errorf("whether %s sees what platform read is not %v", p, sees)
		}
	}
	if ds := got[[]requests.Delivery](t)(d.Deliveries(d.thread(d.platform)[2].ID)); len(ds) != 1 || ds[0].Person != "al" {
		t.Errorf("platform's failure is carried to %+v", ds)
	}
}

func TestRequestsEntryToSomeoneSeenByThem(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, d.Readers("slack", thread, []string{slackDana, slackAl}))
	record := d.decide(d.payments, "ledger", "Refund one charge", "refund(card 4111, £40)", "raj")
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decided", By: d.Person("raj"), To: []requests.Who{d.Worker("ledger")},
		Text: "Refund one charge: approved by Raj", Answers: record.ID})))
	must(t, errOf(d.Say(d.payments, d.Person("lee"), "inside payments", nil, requests.Screens, false)))
	v := got[*requests.View](t)(d.View(d.payments, "raj"))
	var texts []string
	for _, e := range v.Entries {
		texts = append(texts, e.Text)
	}
	if v.Whole || !slices.Equal(texts, []string{"Refund one charge", "refund(card 4111, £40)", "Refund one charge: approved by Raj"}) || v.Team != "" || len(v.Subs) != 0 {
		t.Errorf("Raj sees %+v", v)
	}
	if _, err := d.View(d.dana, "raj"); !errors.Is(err, requests.ErrNotThere) {
		t.Errorf("Raj looking at Dana's request is told %v", err)
	}
}

func TestRequestsNoSightInsideATeamWithin(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	garden := got[*requests.Request](t)(h.AskInChat("sam", "garden", "the hedge", nil))
	must(t, errOf(h.Write(garden.Number, requests.Writing{Kind: "said", By: h.Worker("gardener"), To: []requests.Who{h.Worker("groundsman")},
		Text: "the shed key is under the pot", Read: true})))
	if holds(h.shows(garden.Number, "kit"), "shed key") || got[bool](t)(h.MayWatch(requests.Labels(garden), "kit")) {
		t.Error("Kit, of the house around the garden, sees inside the garden")
	}
	if !holds(h.shows(garden.Number, "sam"), "shed key") {
		t.Error("Sam, of the garden, does not see inside it")
	}
	d := workDay(t)
	d.decide(d.payments, "ledger", "Refund one charge", "the ledger's lines", "lee")
	if holds(d.shows(d.payments, "al"), "the ledger's lines") || got[bool](t)(d.MayWatch(requests.Labels(got[*requests.Request](t)(d.Get(d.payments))), "al")) {
		t.Error("Al, of the office around payments, sees inside payments")
	}
}

func TestRequestsSightGoesDownNeverUp(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	for _, n := range []int64{d.dana, d.platform} {
		if _, err := d.View(n, "lee"); !errors.Is(err, requests.ErrNotThere) {
			t.Errorf("Lee looking at request %d is told %v", n, err)
		}
	}
	if v := got[*requests.View](t)(d.View(d.payments, "lee")); v.Parent != 0 || v.Origin != nil {
		t.Errorf("Lee is shown payments' sub-request as asked from %d", v.Parent)
	}
	if v := got[*requests.View](t)(d.View(d.payments, "dana")); v.Parent != d.dana {
		t.Errorf("Dana is shown payments' sub-request as asked from %d", v.Parent)
	}
	if v := got[*requests.View](t)(d.View(d.dana, "dana")); len(v.Subs) != 2 {
		t.Errorf("Dana sees beneath her request %+v", v.Subs)
	}
	for _, s := range got[[]requests.Summary](t)(d.List("lee", requests.Filter{})) {
		if s.Number != d.payments || s.Parent != 0 {
			t.Errorf("Lee's list holds %+v", s)
		}
	}
}

func TestRequestsSightFollowsTheFiles(t *testing.T) {
	t.Parallel()
	files := house(t)
	h := open(t, files)
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	must(t, h.Failed(n, "secret LEDGER_KEY wrong"))
	failure := h.thread(n)[1]
	var told []int64
	stop := h.WatchAs("sam", func(n int64, err error) {
		must(t, err)
		told = append(told, n)
	})
	defer stop()
	h.read(replace(t, files, "office.yaml", "people: [sam, kit]", "people: [kit]"))
	must(t, errOf(h.Say(n, h.Person("kit"), "it sparked again", nil, requests.Screens, false)))
	if h.sees(n, "sam") || h.shows(n, "sam") != nil || len(told) != 0 {
		t.Errorf("Sam, taken out of the house, sees %q of Kit's request, and is told of %v", h.shows(n, "sam"), told)
	}
	if ds := got[[]requests.Delivery](t)(h.Deliveries(failure.ID)); len(ds) != 1 || ds[0].Person != "kit" {
		t.Errorf("the house's failure is carried to %+v", ds)
	}
	h.read(files)
	must(t, errOf(h.Say(n, h.Person("kit"), "and again", nil, requests.Screens, false)))
	if !h.sees(n, "sam") || !slices.Equal(told, []int64{n}) || !holds(h.shows(n, "sam"), "LEDGER_KEY") || len(h.shows(n, "sam")) != 5 {
		t.Errorf("Sam, put back, sees %q and is told of %v", h.shows(n, "sam"), told)
	}
	// Platform's people are those its decisions reach, read each time.
	d := workDay(t)
	must(t, d.Failed(d.platform, "the pipeline API said 500"))
	failure = d.thread(d.platform)[1]
	if !holds(d.shows(d.platform, "al"), "said 500") {
		t.Fatal("Al does not see platform's failure")
	}
	d.read(replace(t, d.files, "office.yaml", "decide: { everything: [al] }", "decide: { everything: [raj] }"))
	if holds(d.shows(d.platform, "al"), "said 500") || !holds(d.shows(d.platform, "raj"), "said 500") {
		t.Errorf("with the office's everything Raj's, Al is shown %q and Raj %q", d.shows(d.platform, "al"), d.shows(d.platform, "raj"))
	}
	if ds := got[[]requests.Delivery](t)(d.Deliveries(failure.ID)); len(ds) != 1 || ds[0].Person != "raj" {
		t.Errorf("platform's failure is carried to %+v", ds)
	}
}

func TestRequestsUnseenSameAsAbsent(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	unseen := got[*requests.Request](t)(d.AskInChat("dana", "", "in the chat", nil)).Number
	var told []int64
	stop := d.WatchAs("raj", func(n int64, err error) {
		must(t, err)
		told = append(told, n)
	})
	defer stop()
	must(t, errOf(d.Say(unseen, d.Person("dana"), "more", nil, requests.Screens, false)))
	_, e1 := d.View(unseen, "raj")
	_, e2 := d.View(999, "raj")
	_, e3 := d.MaySay(unseen, d.Person("raj"), requests.Screens)
	_, e4 := d.MaySay(999, d.Person("raj"), requests.Screens)
	_, e5 := d.Say(unseen, d.Person("raj"), "hi", nil, requests.Screens, false)
	_, e6 := d.Say(999, d.Person("raj"), "hi", nil, requests.Screens, false)
	_, e7 := d.File(unseen, 1, "a", "raj")
	_, e8 := d.File(999, 1, "a", "raj")
	for i, err := range []error{e1, e2, e3, e4, e5, e6, e7, e8} {
		if !errors.Is(err, requests.ErrNotThere) || err.Error() != requests.ErrNotThere.Error() {
			t.Errorf("answer %d is %v", i, err)
		}
	}
	for _, s := range got[[]requests.Summary](t)(d.List("raj", requests.Filter{})) {
		if s.Number == unseen {
			t.Error("Raj's list holds Dana's chat")
		}
	}
	if len(told) != 0 || len(got[[]requests.Wait](t)(d.WaitingOn("raj"))) != 0 {
		t.Errorf("Raj is told of %v", told)
	}
}

func TestRequestsNothingLeaksThroughANotice(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	q := got[*requests.Entry](t)(d.Put(d.dana, d.Worker("bot"), requests.KindQuestion, d.Person("al"), "may I refund 4411?", nil))
	if ds := got[[]requests.Delivery](t)(d.Deliveries(q.ID)); len(ds) != 1 || ds[0].Person != "al" || ds[0].Origin != nil {
		t.Errorf("the bot's question to Al is carried to %+v", ds)
	}
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	must(t, h.Failed(n, "the key LEDGER is wrong"))
	team := h.thread(n)[1]
	if ds := got[[]requests.Delivery](t)(h.Deliveries(team.ID)); len(ds) != 2 || ds[0].Person != "sam" || ds[1].Person != "kit" || ds[0].Origin != nil || ds[1].Origin != nil {
		t.Errorf("the house's error is carried to %+v", ds)
	}
	said := got[*requests.Entry](t)(h.Say(n, h.Person("kit"), "again?", nil, requests.Screens, false))
	if ds := got[[]requests.Delivery](t)(h.Deliveries(said.ID)); len(ds) != 0 {
		t.Errorf("Kit's own words are carried to %+v", ds)
	}
}

func TestRequestsNoFallbackRecipient(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	before := len(d.thread(d.payments))
	var refusal requests.Refusal
	if _, err := d.Put(d.payments, d.Worker("ledger"), requests.KindQuestion, d.Person("dana"), "what did you tell them?", nil); !errors.As(err, &refusal) {
		t.Errorf("the ledger's question to Dana is told %v", err)
	}
	if _, err := d.Put(d.payments, d.Worker("ledger"), requests.KindQuestion, d.Worker("bot"), "which charge?", nil); !errors.As(err, &refusal) {
		t.Errorf("the ledger's question to the bot, its asker, is told %v", err)
	}
	if _, err := d.Put(d.payments, d.Worker("ledger"), requests.KindTold, d.Person("zed"), "hello", nil); !errors.As(err, &refusal) {
		t.Errorf("a tell to nobody is told %v", err)
	}
	if _, err := d.Write(d.payments, requests.Writing{Kind: "decision", Text: "x", Waits: []requests.Who{d.Person("zed")}}); !errors.As(err, &refusal) {
		t.Errorf("a decision waiting on nobody is told %v", err)
	}
	for _, to := range []requests.Who{d.Person("zed"), requests.Identity("slack", "U0NOBODY")} {
		if _, err := d.Write(d.payments, requests.Writing{Kind: "shown", By: requests.Bulldog, To: []requests.Who{d.Person("lee"), to}, Text: "x", Read: true}); !errors.As(err, &refusal) {
			t.Errorf("an entry to %+v, who is nobody, is told %v", to, err)
		}
	}
	if len(d.thread(d.payments)) != before || len(got[[]requests.Wait](t)(d.WaitingOn("lee"))) != 0 {
		t.Error("what was refused was given to someone")
	}
	// An entry may be to workers and teams, and to a guest who asked.
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "said", By: d.Worker("ledger"), To: []requests.Who{d.Worker("ledger"), d.Team("payments")}, Text: "y", Read: true})))
	guest := requests.Identity("slack", "U0STRANGE")
	n := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.07", "", guest.ID, "refund me", nil, false)).Number
	must(t, errOf(d.Write(n, requests.Writing{Kind: "not disclosed", By: requests.Bulldog, To: []requests.Who{guest}, Text: "the office did not disclose its answer"})))
}

// noSam gives the house with Sam gone from the office.
func noSam(t *testing.T, files map[string]string) map[string]string {
	files = replace(t, files, "office.yaml", "  sam: { name: Sam, title: Owner. Decides money and the building., telegram: \"4471\" }\n", "")
	files = replace(t, files, "office.yaml", "people: [sam, kit]", "people: [kit]")
	files = replace(t, files, "office.yaml", "everything: [sam]", "everything: [kit]")
	files = replace(t, files, "office.yaml", "order_part: [./small-orders.js, clerk: advise, sam]", "order_part: [./small-orders.js, clerk: advise, kit]")
	files = replace(t, files, "garden/team.yaml", "people: [sam]", "people: [kit]")
	return replace(t, files, "garden/team.yaml", "everything: [sam]", "everything: [kit]")
}

func TestRequestsQuestionToSomeoneGone(t *testing.T) {
	t.Parallel()
	files := house(t)
	h := open(t, files)
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	q := got[*requests.Entry](t)(h.Put(n, h.Worker("fixer"), requests.KindQuestion, h.Person("sam"), "may I cut circuit 4?", nil))
	h.read(noSam(t, files))
	last := h.last(n)
	if last.By != requests.Bulldog || last.Answers != q.ID || last.Text != "Sam is no longer in the office." || last.To[0] != h.Worker("fixer") || h.state(n).Is != requests.Working {
		t.Errorf("the question to Sam is answered %+v", last)
	}
	h.read(files)
	q = got[*requests.Entry](t)(h.Put(n, h.Worker("steward"), requests.KindQuestion, h.Person("sam"), "may I cut circuit 4?", nil))
	h.read(replace(t, files, "office.yaml", "people: [sam, kit]", "people: [kit]"))
	if last := h.last(n); last.Answers != q.ID || last.Text != "Sam is no longer in the house." || h.state(n).Is != requests.Working {
		t.Errorf("the question to Sam out of the house is answered %+v", last)
	}
	if ws := got[[]requests.Wait](t)(h.WaitingOn("sam")); len(ws) != 0 {
		t.Errorf("what waits on Sam is %+v", ws)
	}
	// A question put to Sam as he is on Telegram is answered so after a
	// restart on files without him.
	h.read(files)
	q = got[*requests.Entry](t)(h.Put(n, h.Worker("fixer"), requests.KindQuestion, requests.Identity("telegram", "4471"), "may I cut circuit 5?", nil))
	h.files = noSam(t, files)
	h.restart()
	if last := h.last(n); last.Answers != q.ID || last.Text != "Sam is no longer in the office." || h.state(n).Is != requests.Working {
		t.Errorf("after a restart, the question to Sam on Telegram is answered %+v", last)
	}
}

func TestRequestsDecisionPutToOthers(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	refund := d.decide(d.payments, "ledger", "Refund one charge", "refund(card 4111)", "lee", "priya")
	// Lee is taken off payments' decide list while staying in payments,
	// and the decision walks on to Priya alone.
	d.read(replace(t, d.files, "payments/team.yaml", "decide: { everything: [lee, priya] }", "decide: { everything: [priya] }"))
	must(t, d.Rewait(refund.ID, []requests.Who{d.Person("priya")}))
	if st := d.state(d.payments); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"priya"}) {
		t.Errorf("payments' sub-request stands %+v", st)
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("lee")); len(ws) != 0 {
		t.Errorf("what waits on Lee is %+v", ws)
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("priya")); len(ws) != 1 || ws[0].Entry != refund.ID {
		t.Errorf("what waits on Priya is %+v", ws)
	}
	var refusal requests.Refusal
	if err := d.Rewait(refund.ID, []requests.Who{d.Person("zed")}); !errors.As(err, &refusal) || len(got[[]requests.Wait](t)(d.WaitingOn("priya"))) != 1 {
		t.Errorf("a decision put to nobody of the office is told %v", err)
	}
	// Put to nobody, it is closed.
	must(t, d.Rewait(refund.ID, nil))
	if st := d.state(d.payments); st.Is != requests.Working || len(got[[]requests.Wait](t)(d.WaitingOn("priya"))) != 0 {
		t.Errorf("the decision put to nobody leaves payments' sub-request %+v", st)
	}
	if err := d.Rewait(refund.ID, []requests.Who{d.Person("priya")}); !errors.As(err, &refusal) {
		t.Errorf("a closed decision put to Priya again is told %v", err)
	}
	// One a decision is moved to sees it, and what answers it, once they
	// have decided.
	must(t, d.Readers("slack", thread, []string{slackDana, slackAl}))
	e := d.decide(d.payments, "ledger", "Refund the fee", "refund(fee 2)", "lee")
	must(t, d.Rewait(e.ID, []requests.Who{d.Person("raj")}))
	if !holds(d.shows(d.payments, "raj"), "Refund the fee") {
		t.Errorf("Raj, put the decision, is shown %q", d.shows(d.payments, "raj"))
	}
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decided", By: d.Person("raj"), To: []requests.Who{d.Worker("ledger")},
		Text: "Refund the fee: approved by Raj", Answers: e.ID})))
	if shown := d.shows(d.payments, "raj"); !slices.Equal(shown, []string{"Refund the fee", "Refund the fee: approved by Raj"}) {
		t.Errorf("Raj, having decided, is shown %q", shown)
	}
	if err := d.Rewait(e.ID, []requests.Who{d.Person("priya")}); !errors.As(err, &refusal) || len(got[[]requests.Wait](t)(d.WaitingOn("priya"))) != 0 {
		t.Errorf("a decision made, put to Priya again, is told %v", err)
	}
}

func TestRequestsTeamGone(t *testing.T) {
	t.Parallel()
	files := house(t)
	h := open(t, files)
	n := got[*requests.Request](t)(h.AskInChat("sam", "garden", "the hedge", nil)).Number
	must(t, errOf(h.Write(n, requests.Writing{Kind: "said", By: h.Worker("gardener"), To: []requests.Who{h.Worker("groundsman")}, Text: "trim it Tuesday", Read: true})))
	h.read(without(files, "garden"))
	if last := h.last(n); last.By != requests.Bulldog || last.Text != "The garden is no longer in the office." || h.state(n).Is != requests.Answered {
		t.Errorf("the request to the garden ends %+v", last)
	}
	if shown := h.shows(n, "sam"); !holds(shown, "the hedge") || holds(shown, "trim it") {
		t.Errorf("Sam is shown %q", shown)
	}
	if h.shows(n, "kit") != nil {
		t.Error("Kit sees the request to the garden")
	}
	// After a restart on files without the garden, it is said the same.
	h.read(files)
	later := got[*requests.Request](t)(h.AskInChat("sam", "garden", "the shed", nil)).Number
	h.files = without(files, "garden")
	h.restart()
	if last := h.last(later); last.Text != "The garden is no longer in the office." || h.state(later).Is != requests.Answered {
		t.Errorf("after a restart, the request to the garden ends %+v", last)
	}
	h.read(files)
	again := got[*requests.Request](t)(h.AskInChat("sam", "garden", "the lawn", nil)).Number
	must(t, errOf(h.Write(again, requests.Writing{Kind: "said", By: h.Worker("gardener"), To: []requests.Who{h.Worker("groundsman")}, Text: "mow it", Read: true})))
	must(t, errOf(h.Answer(again, h.Worker("gardener"), "Mown.", nil)))
	h.read(moved(files, "garden", "yard"))
	if holds(h.shows(again, "sam"), "mow it") || got[bool](t)(h.MayWatch(requests.Labels(got[*requests.Request](t)(h.Get(again))), "sam")) {
		t.Error("Sam, yard's person, sees what the garden read")
	}
	if h.last(again).Text != "Mown." {
		t.Error("an answered request to a team renamed is answered again")
	}
}

func TestRequestsGoneTeamReadByNobody(t *testing.T) {
	t.Parallel()
	files := house(t)
	h := open(t, files)
	n := got[*requests.Request](t)(h.AskInChat("kit", "garden", "the hedge", nil)).Number
	must(t, h.Failed(n, "the gardener's key is wrong"))
	if !holds(h.shows(n, "sam"), "key is wrong") {
		t.Fatal("Sam does not see what the garden read")
	}
	h.read(without(files, "garden"))
	for _, p := range []string{"sam", "kit"} {
		if holds(h.shows(n, p), "key is wrong") {
			t.Errorf("%s, of the house around it, sees what the garden read", p)
		}
	}
	if !h.sees(n, "kit") {
		t.Error("Kit, its asker, no longer sees what was done")
	}
}

func TestRequestsReusedTeamNameInheritsNothing(t *testing.T) {
	t.Parallel()
	files := house(t)
	h := open(t, files)
	n := got[*requests.Request](t)(h.AskInChat("sam", "garden", "the hedge", nil)).Number
	must(t, h.Failed(n, "the gardener's key is wrong"))
	h.read(without(files, "garden"))
	newGarden := replace(t, files, "garden/team.yaml", "people: [sam]", "people: [kit]")
	newGarden = replace(t, newGarden, "garden/team.yaml", "everything: [sam]", "everything: [kit]")
	h.read(newGarden)
	if h.shows(n, "kit") != nil {
		t.Errorf("Kit, of the new garden, is shown %q", h.shows(n, "kit"))
	}
	if list := got[[]requests.Summary](t)(h.List("kit", requests.Filter{Team: "garden"})); len(list) != 0 {
		t.Errorf("Kit lists the garden's %+v", list)
	}
}

func TestRequestsGuestAsksOnlyWhereGuestsMay(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	guest := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.07", "", "U0STRANGE", "is the API down?", nil, false))
	if guest.Team != office.Top || guest.Asker != requests.Identity("slack", "U0STRANGE") {
		t.Errorf("the guest's request is %+v", guest)
	}
	answer := got[*requests.Entry](t)(d.Answer(guest.Number, d.Worker("bot"), "No.", nil))
	if ds := got[[]requests.Delivery](t)(d.Deliveries(answer.ID)); len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Place != "#help/1700.07" {
		t.Errorf("the answer to the guest goes to %+v", ds)
	}
	var refusal requests.Refusal
	if _, err := d.AskAt("slack", office.Direct, "D0STRANGE", "", "U0STRANGE", "hello", nil, false); !errors.As(err, &refusal) {
		t.Errorf("a direct message is told %v", err)
	}
	if _, err := d.AskAt("gitlab", "platform/api", "platform/api#60", "", "9999", "broken", nil, false); !errors.As(err, &refusal) {
		t.Errorf("a guest's comment on platform/api is told %v", err)
	}
	if d.count("requests_requests") != 1 {
		t.Error("a request is made where guests may not ask")
	}
}

func TestRequestsGuestSaysMoreAtOrigin(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	place := requests.Where{Bridge: "slack", Place: "#help/1700.07"}
	guest := requests.Identity("slack", "U0STRANGE")
	n := got[*requests.Request](t)(d.AskAt("slack", "#help", place.Place, "", guest.ID, "refund me", nil, false)).Number
	q := got[*requests.Entry](t)(d.Put(n, d.Worker("bot"), requests.KindQuestion, guest, "which customer?", nil))
	if ds := got[[]requests.Delivery](t)(d.Deliveries(q.ID)); len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Place != place.Place {
		t.Errorf("the question to the guest goes to %+v", ds)
	}
	if e := got[*requests.Entry](t)(d.Say(n, guest, "customer 7", nil, place, false)); e.By != guest || e.To[0] != d.Team(office.Top) {
		t.Errorf("the guest's reply is %+v", e)
	}
	if _, err := d.Say(n, guest, "me again", nil, requests.Where{Bridge: "slack", Place: office.Direct}, false); !errors.Is(err, requests.ErrNotThere) {
		t.Errorf("the guest saying more elsewhere is told %v", err)
	}
}

func TestRequestsGuestLaterAPerson(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.07", "", "U0STRANGE", "refund me", nil, false)).Number
	d.read(replace(t, d.files, "office.yaml", "everyone:\n", "everyone:\n  jo:    { name: Jo, slack: U0STRANGE }\n"))
	if !d.sees(n, "jo") {
		t.Error("Jo does not see what she asked as a guest")
	}
	if list := got[[]requests.Summary](t)(d.List("jo", requests.Filter{Asked: true})); len(list) != 1 || list[0].Asker.Name != "jo" {
		t.Errorf("Jo asked %+v", list)
	}
}

func TestRequestsGuestAsksAndNothingMore(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	guest := requests.Identity("slack", "U0STRANGE")
	n := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1700.07", "", guest.ID, "refund me", nil, false)).Number
	var refusal requests.Refusal
	if _, err := d.Write(n, requests.Writing{Kind: "decision", Text: "Refund", To: []requests.Who{guest}, Waits: []requests.Who{guest}}); !errors.As(err, &refusal) {
		t.Errorf("a decision put to a guest is told %v", err)
	}
	if _, err := d.Put(d.dana, d.Worker("bot"), requests.KindQuestion, guest, "who are you?", nil); !errors.As(err, &refusal) {
		t.Errorf("a question to a guest away from their origin is told %v", err)
	}
	// A guest is reached at their origin alone: their answer from anywhere
	// else is not kept, and the question still waits on them there.
	q := got[*requests.Entry](t)(d.Put(n, d.Worker("bot"), requests.KindQuestion, guest, "which customer?", nil))
	for _, in := range []requests.Where{{Bridge: "slack", Place: office.Direct}, {Bridge: "slack", Place: thread}, requests.Screens} {
		if _, err := d.Reply(q.ID, guest, "customer 7", nil, in); !errors.Is(err, requests.ErrNotThere) {
			t.Errorf("the guest's answer from %+v is told %v", in, err)
		}
	}
	if st := d.state(n); st.Is != requests.Waiting || len(d.thread(n)) != 2 {
		t.Errorf("after the guest answered elsewhere, their request stands %+v", st)
	}
	must(t, errOf(d.Reply(q.ID, guest, "customer 7", nil, requests.Where{Bridge: "slack", Place: "#help/1700.07"})))
	if st := d.state(n); st.Is != requests.Working || d.last(n).By != guest {
		t.Errorf("after the guest answered at their origin, their request stands %+v", st)
	}
	for _, err := range []error{errOf(d.View(n, "")), errOf(d.WhatWasRead(n, ""))} {
		if !errors.Is(err, requests.ErrNotThere) {
			t.Errorf("one who is no person is told %v", err)
		}
	}
	if list := got[[]requests.Summary](t)(d.List("", requests.Filter{})); len(list) != 0 {
		t.Errorf("one who is no person lists %+v", list)
	}
}
