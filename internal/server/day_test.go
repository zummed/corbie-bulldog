package server

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// The two days of the README, served as the program serves an office and
// called as a browser calls it, each person signed in by the link the
// server printed for them, with a stand-in model and stand-in bridges.

// signIn signs a person in as their browser does: it opens the link the
// server printed for them, keeps the key the answer gives it to keep, and
// is shown the screens as that person.
func (s *serving) signIn(person string) *browser {
	s.b.t.Helper()
	link := ""
	for _, line := range s.lines() {
		if _, at, ok := strings.Cut(line, "bulldog: "+person+", open "); ok {
			link = at
		}
	}
	path, ok := strings.CutPrefix(link, s.address())
	if !ok || !strings.HasPrefix(path, "/?key=") {
		s.b.t.Fatalf("the server printed no link for %s:\n%s", person, strings.Join(s.lines(), "\n"))
	}
	kept := s.nobody().get(path).is(http.StatusOK).cookie()
	if kept == nil {
		s.b.t.Fatalf("opening %s's link gave the browser no key", person)
	}
	br := &browser{s: s, key: kept.Value, header: http.Header{}}
	if page := br.get("/").is(http.StatusOK); !strings.Contains(string(page.body), `<script src="/static/app.js"></script>`) {
		s.b.t.Fatalf("%s, signed in, is not shown the screens: %s", person, page.body)
	}
	var me meOut
	br.get("/api/me").is(http.StatusOK).into(&me)
	if me.Me.Name != person {
		s.b.t.Fatalf("%s's link signs in %s", person, me.Me.Name)
	}
	return br
}

// view gives a request as a browser is shown it.
func (br *browser) view(n int64) viewOut {
	br.s.b.t.Helper()
	var v viewOut
	br.get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	return v
}

// listed gives the numbers of the requests a browser is shown, as the
// address narrows them.
func (br *browser) listed(query string) []int64 {
	br.s.b.t.Helper()
	var list struct{ Requests []summaryOut }
	br.get("/api/requests" + query).is(http.StatusOK).into(&list)
	out := []int64{}
	for _, r := range list.Requests {
		out = append(out, r.Number)
	}
	return out
}

// until stays until a browser is shown a request as it is wanted, and
// gives it so.
func (br *browser) until(n int64, what string, wanted func(viewOut) bool) viewOut {
	br.s.b.t.Helper()
	var v viewOut
	br.s.b.until(fmt.Sprintf("request %d %s", n, what), func() bool {
		v = br.view(n)
		return wanted(v)
	})
	return v
}

// all gives every entry's text of a view, a line to each.
func (v viewOut) all() string {
	var b strings.Builder
	for _, e := range v.Entries {
		b.WriteString(e.Text + "\n")
	}
	return b.String()
}

// deliver ends each out run of a bridge that carries to a place as sent,
// one after another as they come, until one carries words that hold
// text, and gives its input.
func (b *bench) deliver(bridge, to, text string) map[string]any {
	b.t.Helper()
	for sent := 1; ; sent++ {
		run, in := b.waiting(bridges.KindOut, bridge, standin.Carried, to)
		b.answerTicket(run, standin.Carried, map[string]any{"sent": fmt.Sprintf("%s-%d", run, sent)})
		if words, _ := in["text"].(string); strings.Contains(words, text) {
			return in
		}
		b.until("the out run "+run+" has ended", func() bool {
			return !slices.ContainsFunc(b.going(bridges.KindOut, bridge), func(r *client.Run) bool { return r.ID == run })
		})
	}
}

// holds fails the test unless what is shown holds every part.
func holds(t *testing.T, what, shown string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(shown, part) {
			t.Errorf("%s does not hold %q:\n%s", what, part, shown)
		}
	}
}

// holdsNone fails the test if what is shown holds any part.
func holdsNone(t *testing.T, what, shown string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if strings.Contains(shown, part) {
			t.Errorf("%s holds %q:\n%s", what, part, shown)
		}
	}
}

// playHousesDay plays the house's workers through the README's day: the
// steward hands the socket to the fixer, who asks sparks which circuit,
// cuts it, orders a socket and asks Kit to say when it comes; on Monday
// the steward walks round.
func playHousesDay(turn testkit.Turn) testkit.Reply {
	switch {
	case strings.Contains(turn.System, "You look after the house") && strings.Contains(turn.Prompt(), "Walk round"):
		return testkit.Say("Nothing is open.")
	case strings.Contains(turn.System, "You look after the house"):
		if turn.Number == 1 {
			return testkit.Use("fixer", map[string]any{"job": "The kitchen socket sparked: mend it."})
		}
		return testkit.Say("The kitchen socket is mended, and circuit 4 is back on.")
	case strings.Contains(turn.System, "You do the repairs"):
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
		if turn.Number == 1 {
			return testkit.Use("circuits", map[string]any{})
		}
		return testkit.Say("The kitchen sockets are on circuit 4.")
	}
	return testkit.Say("done")
}

func TestServerTheHousesDay(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(playHousesDay)
	s := b.serve()
	kit, sam := s.signIn("kit"), s.signIn("sam")

	// 1. Kit types into the chat: a request to the house, which the
	// steward starts on.
	n := numbered(kit.post("/api/requests", map[string]string{"text": "the kitchen socket sparked"}))
	if v := kit.view(n); !v.Whole || v.Team != "office" || v.Asker.Name != "kit" || v.Origin.Kind != requests.InChat || v.Entries[0].Text != "the kitchen socket sparked" {
		t.Fatalf("Kit's request is %+v", v)
	}
	if !slices.Contains(kit.listed("?asked=me"), n) {
		t.Errorf("Kit's requests asked by her are %v", kit.listed("?asked=me"))
	}

	// 2 and 3. The fixer asks sparks which circuit, then asks to cut it,
	// which Corbie holds for Sam: Sam is shown it on the screens, with
	// the action as its tool describes itself, and is asked on Telegram.
	cut := sam.until(n, "asks Sam to cut the circuit", func(v viewOut) bool {
		return slices.ContainsFunc(v.Decisions, func(d decisionOut) bool { return d.Mine && strings.Contains(d.Does, "Turn one circuit off") })
	}).Decisions[0]
	if cut.Kind != decisions.OfAction || cut.Team != "office" || !slices.Equal(cut.Asked, []string{"sam"}) {
		t.Errorf("Sam is shown the cut as %+v", cut)
	}
	// Kit, of the house too, sees the cut and its walk, but is not asked it.
	if v := kit.view(n); len(v.Decisions) != 1 || v.Decisions[0].Mine || len(v.Decisions[0].Stages) != 1 || v.State != requests.Waiting || len(v.On) != 1 || v.On[0].Name != "sam" {
		t.Errorf("Kit is shown the request %s on %+v, with the decisions %+v", v.State, v.On, v.Decisions)
	}
	question := b.deliver("telegram", "4471", "Turn one circuit off")
	if question["direct"] != true || !slices.Equal(question["choices"].([]any), []any{"yes", "refuse"}) {
		t.Fatalf("the cut is carried to Sam as %v", question)
	}
	// Sam taps yes, and the in machine hands Bulldog his Telegram
	// identity.
	listening, _ := b.waiting(bridges.KindIn, "telegram", standin.Heard, "")
	b.answerTicket(listening, standin.Heard, map[string]any{"cursor": 1,
		"heard": []any{thing("choice", "4471", "direct", "4471", "answers", question["ref"], "choice", "yes")}})
	sam.until(n, "has the cut decided", func(v viewOut) bool {
		return slices.ContainsFunc(v.Decisions, func(d decisionOut) bool { return d.ID == cut.ID && d.Ended == decisions.Yes && d.By.Name == "sam" })
	})
	b.deliver("telegram", "4471", "Taken")

	// 4. The order is the small-orders rule's to say yes to, and Sam is
	// asked nothing of it.
	kit.until(n, "has the order decided", func(v viewOut) bool { return strings.Contains(v.all(), "Order a part: approved by") })

	// 5. The fixer asks Kit to say when the socket arrives: the request
	// waits on Kit until Thursday.
	var me meOut
	b.until("something waits on Kit", func() bool {
		kit.get("/api/me").is(http.StatusOK).into(&me)
		return me.Waiting == 1
	})
	if waiting := kit.listed("?waiting=me"); !slices.Equal(waiting, []int64{n}) {
		t.Errorf("what waits on Kit is %v", waiting)
	}
	v := kit.view(n)
	ask := v.Entries[slices.IndexFunc(v.Entries, func(e entryOut) bool { return e.Kind == requests.KindQuestion })]
	if !slices.Equal(v.Waits, []int64{ask.ID}) || ask.Text != "Say when the new socket arrives." || v.State != requests.Waiting || v.On[0].Name != "kit" {
		t.Errorf("Kit is shown %+v waiting on her, in a request %s on %+v", ask, v.State, v.On)
	}
	b.clock.Set(time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC))
	if v := kit.view(n); v.State != requests.Waiting {
		t.Errorf("on Thursday the request is %s", v.State)
	}
	kit.post("/api/requests/"+itoa(n)+"/entries", map[string]any{"text": "The socket came today.", "re": ask.ID}).is(http.StatusOK)

	// 6. The fixer finishes, and the steward answers Kit.
	s.answered(n)
	v = kit.view(n)
	if last := v.Entries[len(v.Entries)-1]; last.Kind != requests.KindAnswer || last.By.Name != "steward" || last.Text != "The kitchen socket is mended, and circuit 4 is back on." {
		t.Errorf("Kit is answered %+v", last)
	}
	holds(t, "what was done", v.all(), "Turn one circuit off: approved by Sam", "Order a part: approved by", "The socket came today.")

	// What the house read is seen by its people, and its runs watched by
	// them; nobody else is shown anything.
	if !v.Read || len(v.Records) == 0 || len(v.Runs) == 0 {
		t.Errorf("Kit, of the house, is shown what it read %v, %d records and the runs %+v", v.Read, len(v.Records), v.Runs)
	}
	holds(t, "what the house read", string(kit.get("/api/requests/"+itoa(n)).body), "The kitchen sockets are on circuit 4.")
	// A screen that shows none of what was read asks for it to be left
	// out, and is given the rest.
	var bare viewOut
	unread := kit.get("/api/requests/" + itoa(n) + "?records=no").is(http.StatusOK)
	unread.into(&bare)
	if !bare.Read || bare.Records != nil || len(bare.Runs) != len(v.Runs) || strings.Contains(string(unread.body), "The kitchen sockets are on circuit 4.") {
		t.Errorf("Kit, asking for no records, is given %s", unread.body)
	}
	kit.get("/api/requests/" + itoa(n) + "?records=yes").is(http.StatusBadRequest)
	steward := v.Runs[slices.IndexFunc(v.Runs, func(r runOut) bool { return r.Worker == "steward" })]
	watched(t, kit, steward.Name, http.StatusOK)
	s.nobody().get("/api/requests/" + itoa(n)).is(http.StatusUnauthorized)
	watched(t, s.nobody(), steward.Name, http.StatusUnauthorized)
	kit.get("/api/office/files").is(http.StatusNotFound)
	sam.get("/api/office/files").is(http.StatusOK)
	if garden := kit.get("/api/teams/garden").is(http.StatusOK).json(); garden["workers"] != nil || garden["called"] != "The garden" {
		t.Errorf("Kit, not in the garden, is shown it as %v", garden)
	}

	// 7. On Monday at nine the house's routine asks the steward what is
	// still open; its answer goes to Sam and Kit.
	b.clock.Set(time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC))
	walk := int64(0)
	b.until("the routine is asked", func() bool {
		var list struct{ Requests []summaryOut }
		sam.get("/api/requests?team=office").is(http.StatusOK).into(&list)
		for _, r := range list.Requests {
			if r.Origin != nil && r.Origin.Kind == requests.ByRoutine && r.Origin.At == "mon 09:00" {
				walk = r.Number
			}
		}
		return walk != 0
	})
	s.answered(walk)
	// Kit's screen is told of the request as its answer is carried on
	// Telegram, which writes nothing to the request itself.
	screen := kit.stream("/api/events")
	screen.next()
	for _, at := range []string{"4471", "4472"} {
		b.deliver("telegram", at, "Nothing is open.")
	}
	screen.until("the answer carried is told", changed(walk))
	answered := kit.until(walk, "is carried to Sam and Kit, on their screens and on Telegram", func(v viewOut) bool {
		sent := map[string]bool{}
		for _, c := range v.Entries[len(v.Entries)-1].Carried {
			sent[c.Bridge+" "+c.Person] = c.State == bridges.Sent
		}
		return len(sent) == 4 && sent["chat sam"] && sent["chat kit"] && sent["telegram sam"] && sent["telegram kit"]
	})
	if last := answered.Entries[len(answered.Entries)-1]; last.Text != "Nothing is open." || answered.Asker.Kind != requests.IsTeam {
		t.Errorf("the routine's request is %+v, answered %+v", answered.summaryOut, last)
	}
}

// refundsDay gives the work office as its day goes: the ledger's refund
// is referred by a rule to Lee or Priya, and payments' disclosure check is a
// rule that refers anything that names a card's or an account's
// number, and lets the rest go. Nothing stands in for the ledger's
// service, so its tools reach nothing: what the stand-in says of the
// charges and the refund is the whole of them.
func refundsDay(t *testing.T) map[string]string {
	files := refundOffice(t)
	files["payments/disclosure.js"] = `if (/(\d[ -]?){8,19}/.test(input.text)) return { says: "refer", note: "It names a number that may be an account's." }
return { says: "yes" }
`
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "decide: { everything: [lee, priya] }",
		"decide: { everything: [lee, priya], refund: [./refunds.js, lee, priya] }", 1)
	files["payments/refunds.js"] = `return { says: "refer", note: "A refund of " + input.action.args.charge + " is for Lee or Priya." }` + "\n"
	return files
}

func TestServerTheWorkOfficesRefundDay(t *testing.T) {
	t.Parallel()
	b := newBench(t, refundsDay(t))
	b.says(playRefund)
	s := b.serve()
	people := map[string]*browser{}
	for _, p := range []string{"al", "dana", "lee", "priya", "raj"} {
		people[p] = s.signIn(p)
	}
	dana, lee, al := people["dana"], people["lee"], people["al"]

	// 1. Dana writes in #help: the Slack bridge's address is called, its
	// in machine hands Bulldog what she said with her identity, and it is
	// a request to the office, from Dana in her thread.
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.1", "readers", []any{"U0DANA00", "U0AL0000", "U0RAJ000"},
		"text", "customer 4411 was charged twice after Tuesday's release; refund one and tell me what broke"))
	n := s.at("slack", "#help/1700.1")
	if v := dana.view(n); v.Team != "office" || v.Asker.Name != "dana" || v.Origin.Bridge != "slack" || v.Origin.Place != "#help/1700.1" {
		t.Fatalf("Dana's request is %+v", v.summaryOut)
	}
	if !slices.Contains(dana.listed("?asked=me"), n) {
		t.Errorf("Dana asked %v", dana.listed("?asked=me"))
	}

	// 2 and 3. The bot asks platform and payments, each a sub-request;
	// platform answers.
	platform, payments := s.subOf(n, "platform"), s.subOf(n, "payments")
	s.answered(platform)

	// 4. The ledger asks to refund the second charge: the rule refers it
	// on, with its note, and Lee and Priya are asked on Slack and on the
	// screens. Lee says yes on the screens.
	b.deliver("slack", "U0LEE000", "Refund one charge")
	for _, p := range []string{"lee", "priya"} {
		var me meOut
		people[p].get("/api/me").is(http.StatusOK).into(&me)
		if me.Waiting != 1 || !slices.Equal(people[p].listed("?waiting=me"), []int64{payments}) {
			t.Errorf("%s is shown %d waiting, in %v", p, me.Waiting, people[p].listed("?waiting=me"))
		}
	}
	refund := lee.view(payments).Decisions
	if len(refund) != 1 || !refund[0].Mine || refund[0].Kind != decisions.OfAction || len(refund[0].Referrals) != 1 ||
		refund[0].Referrals[0].Note != "A refund of ch_2 is for Lee or Priya." || !slices.Equal(refund[0].Asked, []string{"lee", "priya"}) {
		t.Fatalf("Lee is shown the refund as %+v", refund)
	}
	if d := dana.view(payments).Decisions; len(d) != 1 || d[0].Mine || d[0].Referrals != nil || d[0].Stages != nil || d[0].Args != nil {
		t.Errorf("Dana is shown the decisions %+v", d)
	}
	decide := "/api/requests/" + itoa(payments) + "/decisions/" + itoa(refund[0].ID)
	if r := dana.post(decide, map[string]string{"says": "yes"}); r.status != http.StatusConflict || r.json()["error"] == "" {
		t.Errorf("Dana's yes answered %d %s", r.status, r.body)
	}
	if r := lee.post(decide, map[string]string{"says": "yes", "note": "A duplicate, plainly."}).is(http.StatusOK).json(); r["said"] == "" {
		t.Errorf("Lee's yes is answered %v", r)
	}
	if d := people["priya"].view(payments).Decisions[0]; d.Mine || d.Ended != decisions.Yes || d.By.Name != "lee" || d.By.Note != "A duplicate, plainly." {
		t.Errorf("Priya is shown the refund as %+v", d)
	}

	// 5. The ledger answers; the disclosure rule finds no account number,
	// and the answer leaves payments.
	s.answered(payments)

	holds(t, "Lee's view of payments' request", lee.view(payments).all(), "ledger's answer to bot, of office: approved by disclosure.js")

	// 6. The bot's answer lands in Dana's thread.
	s.answered(n)
	b.deliver("slack", "#help/1700.1", "Merge request 812 broke the webhook retry")

	// 7. Dana and Al, and Raj, who reads #help, see what was done: two
	// sub-requests, the refund decided by Lee, and both answers. What was
	// read inside payments, the ledger's lines with the card's number, is
	// seen only by Lee and Priya.
	for _, p := range []string{"dana", "al", "raj"} {
		top := people[p].view(n)
		if !top.Whole || top.State != requests.Answered || top.Label != "" || len(top.Subs) != 2 || top.Answer == nil || top.Answer.State != answerSaid {
			t.Errorf("%s is shown Dana's request as %+v", p, top)
		}
		subs := map[string]string{}
		for _, sub := range top.Subs {
			subs[sub.Team] = sub.Answer
		}
		if subs["platform"] != "Merge request 812 broke the webhook retry." || subs["payments"] != "Duplicate confirmed; £40 refunded." {
			t.Errorf("%s is shown the sub-requests answered %v", p, subs)
		}
		sub := people[p].view(payments)
		holds(t, p+"'s view of payments' request", sub.all(), "Refund one charge, by its id: approved by Lee", "Duplicate confirmed; £40 refunded.")
		raw := string(people[p].get("/api/requests/" + itoa(payments)).body)
		holdsNone(t, p+"'s view of payments' request", raw, "4111", "a duplicate.")
		if sub.Read || sub.Records != nil || sub.Runs != nil {
			t.Errorf("%s is shown what payments read: %v, %d records, %d runs", p, sub.Read, len(sub.Records), len(sub.Runs))
		}
	}
	for _, p := range []string{"lee", "priya"} {
		sub := people[p].view(payments)
		if !sub.Read || len(sub.Records) == 0 {
			t.Errorf("%s is not shown what payments read", p)
		}
		if d := sub.Disclosure; d == nil || !d.Set || !d.Needed || len(d.Deciders) == 0 {
			t.Errorf("%s is shown payments' disclosure as %+v", p, d)
		}
		holds(t, p+"'s view of payments' request", string(people[p].get("/api/requests/"+itoa(payments)).body), "4111 1111 1111 1111")
		people[p].get("/api/requests/" + itoa(n)).is(http.StatusNotFound)
		people[p].get("/api/requests/" + itoa(platform)).is(http.StatusNotFound)
		if listed := people[p].listed(""); !slices.Equal(listed, []int64{payments}) {
			t.Errorf("%s is shown the requests %v", p, listed)
		}
	}
	// Al runs the office but is not named in payments: he does not see
	// inside it. Platform has no people, and its decisions reach Al: he
	// sees what it read, and Dana does not.
	ledger := lee.view(payments).Runs[0].Name
	watched(t, lee, ledger, http.StatusOK)
	watched(t, al, ledger, http.StatusNotFound)
	if team := al.get("/api/teams/payments").is(http.StatusOK).json(); team["in"] != false || team["at_work"] != nil {
		t.Errorf("Al is shown payments as %v", team)
	}
	if !al.view(platform).Read || dana.view(platform).Read {
		t.Error("what platform read is not shown to Al alone")
	}

	// A guest asks in #help: the bot is told it is a guest, the answer is
	// carried to the guest's thread, and the guest sees nothing else, nor
	// is seen by anyone who cannot read that thread.
	s.delivered("slack", thing("said", "U0GUEST1", "#help", "#help/1700.5", "text", "is the API down?"))
	guest := s.at("slack", "#help/1700.5")
	s.answered(guest)
	b.deliver("slack", "#help/1700.5", "Merge request 812")
	if v := al.view(guest); v.Asker.Kind != requests.IsGuest || v.Asker.ID != "U0GUEST1" {
		t.Errorf("Al is shown the guest's request as asked by %+v", v.Asker)
	}
	dana.get("/api/requests/" + itoa(guest)).is(http.StatusNotFound)
}

func TestServerDecisionGivenAsFarAsEachSees(t *testing.T) {
	t.Parallel()
	b := newBench(t, refundsDay(t))
	b.says(playRefund)
	s := b.serve()
	dana, lee, priya := s.signIn("dana"), s.signIn("lee"), s.signIn("priya")
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.1", "readers", []any{"U0DANA00", "U0AL0000", "U0RAJ000"},
		"text", "customer 4411 was charged twice after Tuesday's release; refund one and tell me what broke"))
	n := s.at("slack", "#help/1700.1")
	platform, payments := s.subOf(n, "platform"), s.subOf(n, "payments")
	s.answered(platform)
	b.deliver("slack", "U0LEE000", "Refund one charge")
	refund := lee.view(payments).Decisions
	if len(refund) != 1 {
		t.Fatalf("Lee is shown the decisions %+v", refund)
	}
	lee.post("/api/requests/"+itoa(payments)+"/decisions/"+itoa(refund[0].ID), map[string]string{"says": "yes", "note": "A duplicate, plainly."}).is(http.StatusOK)
	s.answered(payments)
	holds(t, "Lee's view of payments' request", lee.view(payments).all(), "ledger's answer to bot, of office: approved by disclosure.js")

	// Dana, who sees what was done, is given the refund as what it is, and
	// who ended it, with nothing of its notes, its walk or its arguments,
	// and nothing of payments' disclosure, which is what payments read.
	sub := dana.view(n).Subs[slices.IndexFunc(dana.view(n).Subs, func(s subOut) bool { return s.Team == "payments" })]
	for where, ds := range map[string][]decisionOut{"payments' request": dana.view(payments).Decisions, "her request": sub.Decisions} {
		if len(ds) != 1 || ds[0].Kind != decisions.OfAction || ds[0].Ended != decisions.Yes || ds[0].By == nil || ds[0].By.Name != "lee" ||
			ds[0].By.Note != "" || ds[0].Referrals != nil || ds[0].Stages != nil || ds[0].Args != nil || ds[0].Leaving != nil {
			t.Errorf("Dana is shown the decisions of %s as %+v", where, ds)
		}
	}
	holdsNone(t, "Dana's view of payments' request", string(dana.get("/api/requests/"+itoa(payments)).body), "A duplicate, plainly.", "is for Lee or Priya", "disclosure.js")
	// Lee, who decided it, and Priya, asked it, both of payments, are
	// given its notes, its walk and its arguments, and payments'
	// disclosure; neither was asked the disclosure, so neither is shown
	// what it let out.
	for _, p := range []*browser{lee, priya} {
		ds := p.view(payments).Decisions
		action := slices.IndexFunc(ds, func(d decisionOut) bool { return d.Kind == decisions.OfAction })
		disclosure := slices.IndexFunc(ds, func(d decisionOut) bool { return d.Kind == decisions.OfDisclosure })
		if len(ds) != 2 || action < 0 || disclosure < 0 {
			t.Fatalf("payments' people are shown the decisions %+v", ds)
		}
		a := ds[action]
		if a.By.Note != "A duplicate, plainly." || len(a.Referrals) != 1 || len(a.Stages) != 3 || !strings.Contains(string(a.Args), "ch_2") || a.Leaving != nil {
			t.Errorf("payments' people are shown the refund as %+v", a)
		}
		if d := ds[disclosure]; d.What != decisions.SaysAnswer || d.Ended != decisions.Yes || len(d.Stages) == 0 || d.Leaving != nil {
			t.Errorf("payments' people are shown its disclosure as %+v", d)
		}
	}
}

func TestServerARequestNotedQuietOrStopped(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "\nlead: bot\n", "\nlead: bot\nheed: ./for-us.js\n", 1)
	files["for-us.js"] = `if (String(input.said).includes("@bot")) return { says: "act" }
return { says: "note", note: "Not for the bot." }
`
	b := newBench(t, files)
	b.says(func(testkit.Turn) testkit.Reply { return testkit.Say("") })
	s := b.serve()
	dana, al := s.as("dana"), s.as("al")
	readers := []any{"U0DANA00", "U0AL0000"}

	// Dana's word to Raj in #help is noted: she sees it tagged Noted, and
	// the request Noted, with no answer to wait on; Al, of the office,
	// also sees the heed's run, which said note, and its note.
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.1", "readers", readers, "text", "Raj, I'll look at it"))
	noted := s.at("slack", "#help/1700.1")
	v := dana.until(noted, "is noted", func(v viewOut) bool { return v.Label == labelNoted })
	if v.State != requests.Answered || len(v.Entries) != 1 || !v.Entries[0].Noted || v.Answer != nil || v.Runs != nil {
		t.Errorf("Dana is shown the noted request as %+v", v)
	}
	holdsNone(t, "Dana's view of the noted request", string(dana.get("/api/requests/"+itoa(noted)).body), "Not for the bot.", "for-us.js")
	v = al.view(noted)
	if len(v.Runs) != 1 || v.Runs[0].Kind != "heed" || v.Runs[0].Says != "note" || v.Runs[0].How != "Not for the bot." || v.Runs[0].Worker != "for-us.js" ||
		len(v.Entries) != 2 || v.Entries[1].Kind != requests.KindNoted || v.Entries[1].Answers != v.Entries[0].ID {
		t.Errorf("Al is shown the noted request as %+v", v)
	}
	if v.Disclosure == nil || v.Disclosure.Set {
		t.Errorf("Al is shown the office's disclosure as %+v", v.Disclosure)
	}

	// Asked of the bot, which answers with nothing, the request is quiet.
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.2", "readers", readers, "text", "@bot is the API down?"))
	quiet := s.at("slack", "#help/1700.2")
	v = dana.until(quiet, "is answered with nothing", func(v viewOut) bool { return v.State == requests.Answered })
	if v.Label != labelQuiet || v.Answer == nil || v.Answer.State != answerQuiet || len(v.Entries) != 1 || v.Entries[0].Noted {
		t.Errorf("Dana is shown the quiet request as %+v", v)
	}
	if v := al.view(quiet); len(v.Runs) != 2 || v.Runs[0].Says != "act" || v.Runs[1].Kind != "work" {
		t.Errorf("Al is shown the quiet request's runs as %+v", v.Runs)
	}

	// Dana's list of what was asked at #help leaves the noted request out
	// and counts it, and holds it once the noted are asked for.
	var list struct {
		Requests []summaryOut
		Noted    int
	}
	dana.get("/api/requests?bridge=slack&place=%23help").is(http.StatusOK).into(&list)
	if len(list.Requests) != 1 || list.Requests[0].Number != quiet || list.Noted != 1 {
		t.Errorf("Dana's list of #help is %+v", list)
	}
	if listed := dana.listed("?bridge=slack&place=%23help&noted=yes"); !slices.Equal(listed, []int64{quiet, noted}) {
		t.Errorf("Dana's list of #help with the noted is %v", listed)
	}
	if listed := dana.listed("?bridge=gitlab"); len(listed) != 0 {
		t.Errorf("Dana's list of what was asked on GitLab is %v", listed)
	}
	dana.get("/api/requests?place=%23help").is(http.StatusBadRequest)
	dana.get("/api/requests?noted=no").is(http.StatusBadRequest)

	// Said more of, and stopped by Al, it is stopped.
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.2", "readers", readers, "text", "@bot hello?"))
	al.until(quiet, "is working again", func(v viewOut) bool { return v.State != requests.Answered })
	al.post("/api/requests/"+itoa(quiet)+"/stop", "").is(http.StatusOK)
	if v := dana.until(quiet, "is stopped", func(v viewOut) bool { return v.Label == labelStopped }); v.Answer == nil || v.Answer.State != answerSaid {
		t.Errorf("Dana is shown the stopped request as %+v", v)
	}
}
