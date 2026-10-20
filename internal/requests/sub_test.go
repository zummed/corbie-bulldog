package requests_test

import (
	"slices"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestRequestsSubRequestMade(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	sub := got[*requests.Request](t)(d.Get(d.payments))
	if sub.Team != "payments" || sub.Asker != d.Worker("bot") || sub.Origin.Kind != "" || sub.Parent != d.dana || sub.Ticket != "ticket-payments" {
		t.Errorf("payments' sub-request is %+v", sub)
	}
	if first := d.thread(d.payments)[0]; first.Kind != requests.KindAsked || first.By != d.Worker("bot") || first.Text != "Refund one of 4411's two charges." {
		t.Errorf("payments' sub-request begins %+v", first)
	}
	leads := d.thread(d.dana)[1]
	if leads.Kind != requests.KindSub || leads.By != d.Worker("bot") || leads.To[0] != d.Team("payments") || leads.Sub != d.payments || leads.Read != "" {
		t.Errorf("Dana's request leads to payments' with %+v", leads)
	}
}

func TestRequestsOneSubRequestPerTicket(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	d.restart()
	again := got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "payments", "Refund one of 4411's two charges.", "ticket-payments"))
	if again.Number != d.payments || d.count("requests_requests") != 3 {
		t.Errorf("the ticket made a second sub-request, %d", again.Number)
	}
	leading := 0
	for _, e := range d.thread(d.dana) {
		if e.Kind == requests.KindSub {
			leading++
		}
	}
	if leading != 2 {
		t.Errorf("Dana's request has %d entries leading to sub-requests", leading)
	}
	if n, ok, err := d.OfTicket("ticket-payments"); err != nil || !ok || n != d.payments {
		t.Errorf("the ticket is kept with %d", n)
	}
	if _, err := d.Sub(d.dana, d.Worker("bot"), "payments", "Refund the other.", ""); err == nil || d.count("requests_requests") != 3 {
		t.Errorf("a sub-request with no ticket is told %v", err)
	}
}

func TestRequestsSubKeepsTopOrigin(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	top, depth, err := d.Top(d.payments)
	if err != nil || top.Number != d.dana || top.Origin.Place != thread || depth != 1 {
		t.Errorf("payments' sub-request's top is %+v, %d below", top, depth)
	}
	below := got[*requests.Request](t)(d.Sub(d.payments, d.Worker("ledger"), "platform", "When did the retry break?", "ticket-below")).Number
	top, depth, err = d.Top(below)
	if err != nil || top.Number != d.dana || top.Origin.Place != thread || depth != 2 {
		t.Errorf("payments' own sub-request's top is %+v, %d below", top, depth)
	}
}

func TestRequestsSubAnswerIsDone(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Duplicate confirmed; £40 refunded.", nil)))
	if d.state(d.payments).Is != requests.Answered || d.last(d.payments).By != d.Worker("ledger") {
		t.Errorf("payments' sub-request ends %+v", d.last(d.payments))
	}
	for _, p := range []string{"dana", "al"} {
		v := got[*requests.View](t)(d.View(d.dana, p))
		if v.Subs[0].Answer != "Duplicate confirmed; £40 refunded." || !holds(d.shows(d.payments, p), "£40 refunded") {
			t.Errorf("%s sees payments' answer as %+v", p, v.Subs[0])
		}
	}
}

func TestRequestsSubStaysWhenRunEnds(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decision", By: d.Worker("ledger"),
		To: []requests.Who{d.Person("lee"), d.Person("priya")}, Text: "Refund one charge", Waits: []requests.Who{d.Person("lee"), d.Person("priya")}})))
	must(t, d.Failed(d.dana, "cancelled in Corbie"))
	if st := d.state(d.payments); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"lee", "priya"}) {
		t.Errorf("payments' sub-request stands %+v", st)
	}
	if d.state(d.dana).Is == requests.Answered {
		t.Error("Dana's request is answered with its sub-requests open")
	}
}

func TestRequestsLateAnswerKept(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	q := got[*requests.Entry](t)(h.Put(n, h.Worker("fixer"), requests.KindQuestion, h.Person("kit"), "say when the socket arrives", nil))
	must(t, h.Failed(n, "the fixer's run failed"))
	if h.state(n).Is == requests.Answered {
		t.Fatal("the request is answered with a question open")
	}
	h.clock.Move(3 * 24 * time.Hour)
	reply := got[*requests.Entry](t)(h.Reply(q.ID, h.Person("kit"), "it came", nil, requests.Screens))
	if reply.To[0] != h.Worker("fixer") || reply.Answers != q.ID || h.state(n).Is != requests.Working {
		t.Errorf("Kit's late answer is %+v", reply)
	}

	d := workDay(t)
	must(t, errOf(d.Answer(d.dana, d.Worker("bot"), "Platform says MR 812; payments is still on it.", nil)))
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Duplicate confirmed; £40 refunded.", nil)))
	late := got[*requests.Entry](t)(d.Late(d.payments))
	if late.Request != d.dana || late.By != d.Worker("ledger") || late.To[0] != d.Worker("bot") || late.Text != "Duplicate confirmed; £40 refunded." ||
		late.Sub != d.payments || d.state(d.dana).Is != requests.Working {
		t.Errorf("payments' late answer is %+v", late)
	}
}

func TestRequestsLateAnswerKeptOnce(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, errOf(d.Answer(d.dana, d.Worker("bot"), "Platform says MR 812; payments is still on it.", nil)))
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Refunded.", nil)))
	first := got[*requests.Entry](t)(d.Late(d.payments))
	must(t, errOf(d.Answer(d.dana, d.Worker("bot"), "Refunded, and MR 812 broke it.", nil)))
	// Given again, as after a restart, it is the same entry, and the
	// answered request stays answered.
	again := got[*requests.Entry](t)(d.Late(d.payments))
	lates := func() []string {
		var texts []string
		for _, e := range d.thread(d.dana) {
			if e.Kind == requests.KindLate {
				texts = append(texts, e.Text)
			}
		}
		return texts
	}
	if again.ID != first.ID || !slices.Equal(lates(), []string{"Refunded."}) || d.state(d.dana).Is != requests.Answered {
		t.Errorf("the late answer given twice is %+v and %+v, in %v", first, again, lates())
	}
	// A later answer of the sub-request is late again.
	must(t, errOf(d.Say(d.payments, d.Worker("bot"), "and the other charge?", nil, requests.Where{}, false)))
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Kept.", nil)))
	if later := got[*requests.Entry](t)(d.Late(d.payments)); later.ID == first.ID || !slices.Equal(lates(), []string{"Refunded.", "Kept."}) {
		t.Errorf("the later answer is %+v, in %v", later, lates())
	}
}

func TestRequestsAnswerLeavesSubRequestsGoing(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, errOf(d.Answer(d.dana, d.Worker("bot"), "Refund under way.", nil)))
	if d.state(d.payments).Is != requests.Working || d.state(d.platform).Is != requests.Working {
		t.Error("answering the request closed what is beneath it")
	}
	must(t, errOf(d.Answer(d.platform, d.Worker("ops"), "MR 812.", nil)))
	must(t, errOf(d.Late(d.platform)))
	if d.state(d.dana).Is != requests.Working || d.last(d.dana).Text != "MR 812." {
		t.Error("platform's answer did not come back to Dana's request")
	}
}

func TestRequestsTreeListed(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	below := got[*requests.Request](t)(d.Sub(d.payments, d.Worker("ledger"), "platform", "When did the retry break?", "ticket-below")).Number
	must(t, errOf(d.Answer(d.platform, d.Worker("ops"), "MR 812.", nil)))
	tree := got[[]requests.Sub](t)(d.Tree(d.dana))
	if len(tree) != 2 || tree[0].Team != "payments" || tree[1].Team != "platform" || tree[0].Question != "Refund one of 4411's two charges." ||
		tree[0].State.Is != requests.Working || tree[1].State.Is != requests.Answered || tree[1].Answer != "MR 812." {
		t.Fatalf("Dana's request lists %+v", tree)
	}
	if len(tree[0].Subs) != 1 || tree[0].Subs[0].Number != below || tree[0].Subs[0].Team != "platform" {
		t.Errorf("payments' sub-request lists %+v", tree[0].Subs)
	}
}
