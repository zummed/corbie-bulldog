package requests_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestRequestsStates(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the kitchen socket sparked", nil)).Number
	if st := h.state(n); st.Is != requests.Working {
		t.Errorf("while the steward's run goes on, the request stands %+v", st)
	}
	q := got[*requests.Entry](t)(h.Put(n, h.Worker("fixer"), requests.KindQuestion, h.Person("kit"), "tell me when it arrives", nil))
	if st := h.state(n); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"kit"}) {
		t.Errorf("with the fixer's question open, the request stands %+v", st)
	}
	must(t, errOf(h.Reply(q.ID, h.Person("kit"), "it came", nil, requests.Screens)))
	if st := h.state(n); st.Is != requests.Working {
		t.Errorf("with Kit's answer, the request stands %+v", st)
	}
	must(t, errOf(h.Answer(n, h.Worker("steward"), "The socket is replaced.", nil)))
	if st := h.state(n); st.Is != requests.Answered {
		t.Errorf("with the steward's answer, the request stands %+v", st)
	}
	// A question put after the answer waits on Kit, as every account says.
	must(t, errOf(h.Put(n, h.Worker("fixer"), requests.KindQuestion, h.Person("kit"), "really?", nil)))
	listed := got[[]requests.Summary](t)(h.List("kit", requests.Filter{WaitsOnMe: true}))
	if st := h.state(n); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"kit"}) || len(listed) != 1 || listed[0].State.Is != requests.Waiting {
		t.Errorf("with a question put after the answer, the request stands %+v, listed %+v", st, listed)
	}
	// So does a decision put after an answer.
	d := workDay(t)
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Refunded.", nil)))
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decision", By: d.Worker("ledger"), To: []requests.Who{d.Person("lee")},
		Text: "Refund the fee", Waits: []requests.Who{d.Person("lee")}})))
	if st := d.state(d.payments); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"lee"}) {
		t.Errorf("with a decision put after the answer, payments' sub-request stands %+v", st)
	}
	// An answer said while a question waits leaves the request waiting on
	// whoever it was put to, until they answer.
	d = workDay(t)
	q = got[*requests.Entry](t)(d.Put(d.payments, d.Worker("ledger"), requests.KindQuestion, d.Person("lee"), "which card?", nil))
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Refunded.", nil)))
	if st := d.state(d.payments); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"lee"}) {
		t.Errorf("with the ledger's answer said over its open question, payments' sub-request stands %+v", st)
	}
	must(t, errOf(d.Reply(q.ID, d.Person("lee"), "the visa", nil, requests.Screens)))
	if st := d.state(d.payments); st.Is != requests.Working {
		t.Errorf("with Lee's answer after the ledger's, payments' sub-request stands %+v", st)
	}
	// So does the answer of a lead who is a person: the request waits on
	// what is put to them, and no longer on them as its lead.
	support := got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "support", "What did we tell 4411?", "ticket-support")).Number
	decision := got[*requests.Entry](t)(d.Write(support, requests.Writing{Kind: "decision", By: d.Worker("bot"), To: []requests.Who{d.Person("dana")},
		Text: "Tell 4411 of the refund", Waits: []requests.Who{d.Person("dana")}}))
	must(t, errOf(d.Say(support, requests.Identity("slack", slackDana), "a refund", nil, requests.Where{Bridge: "slack", Place: "direct"})))
	if st := d.state(support); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"dana"}) {
		t.Errorf("with Dana's answer said over the decision put to her, support's sub-request stands %+v", st)
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("dana")); len(ws) != 1 || ws[0].Entry != decision.ID {
		t.Errorf("once Dana answered, what waits on her is %+v", ws)
	}
}

func TestRequestsWaitingNamesEveryone(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	decision := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "decision", By: d.Worker("ledger"),
		To: []requests.Who{d.Person("lee"), d.Person("priya")}, Text: "Refund one charge", Waits: []requests.Who{d.Person("lee"), d.Person("priya")}}))
	if st := d.state(d.payments); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"lee", "priya"}) {
		t.Errorf("with the refund put to Lee and Priya, payments' sub-request stands %+v", st)
	}
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decided", By: requests.Identity("slack", slackLee), To: []requests.Who{d.Worker("ledger")},
		Text: "Refund one charge: yes from Lee, 10:00", Answers: decision.ID})))
	if st := d.state(d.payments); st.Is != requests.Working {
		t.Errorf("with Lee's yes, payments' sub-request stands %+v", st)
	}
}

func TestRequestsWhatWaitsOnAPerson(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	decision := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "decision", By: d.Worker("ledger"),
		To: []requests.Who{d.Person("lee"), d.Person("priya")}, Text: "Refund one charge", Waits: []requests.Who{d.Person("lee"), d.Person("priya")}}))
	for _, p := range []string{"lee", "priya"} {
		if ws := got[[]requests.Wait](t)(d.WaitingOn(p)); len(ws) != 1 || ws[0].Entry != decision.ID || ws[0].Kind != "decision" {
			t.Errorf("what waits on %s is %+v", p, ws)
		}
	}
	var refusal requests.Refusal
	if _, err := d.Reply(decision.ID, d.Person("lee"), "yes", nil, requests.Screens); !errors.As(err, &refusal) || len(got[[]requests.Wait](t)(d.WaitingOn("lee"))) != 1 {
		t.Errorf("a reply to a decision is told %v", err)
	}
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decided", By: d.Person("priya"), To: []requests.Who{d.Worker("ledger")},
		Text: "Refund one charge: yes from Priya", Answers: decision.ID})))
	for _, p := range []string{"lee", "priya"} {
		if ws := got[[]requests.Wait](t)(d.WaitingOn(p)); len(ws) != 0 {
			t.Errorf("once Priya decided, what waits on %s is %+v", p, ws)
		}
	}
	support := got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "support", "What did we tell 4411?", "ticket-support")).Number
	if ws := got[[]requests.Wait](t)(d.WaitingOn("dana")); len(ws) != 1 || ws[0].Request != support || ws[0].Kind != requests.KindLead {
		t.Errorf("what waits on Dana is %+v", ws)
	}
	must(t, errOf(d.Say(support, requests.Identity("slack", slackDana), "a refund", nil, requests.Where{Bridge: "slack", Place: "direct"})))
	if ws := got[[]requests.Wait](t)(d.WaitingOn("dana")); len(ws) != 0 {
		t.Errorf("once Dana answered, what waits on her is %+v", ws)
	}
}

func TestRequestsWaitingOnAgreesEverywhere(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	decision := got[*requests.Entry](t)(d.Write(d.payments, requests.Writing{Kind: "decision", By: d.Worker("ledger"),
		To: []requests.Who{d.Person("lee")}, Text: "Refund one charge", Waits: []requests.Who{d.Person("lee")}}))
	q := got[*requests.Entry](t)(d.Put(d.payments, d.Worker("ledger"), requests.KindQuestion, d.Person("lee"), "which card?", nil))
	// The ledger answers while its question waits on Lee.
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Refunded.", nil)))
	support := got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "support", "What did we tell 4411?", "ticket-support")).Number
	numbers := func(p string, f requests.Filter) []int64 {
		var ns []int64
		for _, s := range got[[]requests.Summary](t)(d.List(p, f)) {
			if s.Whole {
				ns = append(ns, s.Number)
			}
		}
		return ns
	}
	for _, p := range []string{"lee", "dana", "al"} {
		waits := got[[]requests.Wait](t)(d.WaitingOn(p))
		// The list by state agrees: what waits on them is listed waiting,
		// and never answered, and what is listed waiting on them waits.
		waitingList, answeredList := numbers(p, requests.Filter{State: requests.Waiting}), numbers(p, requests.Filter{State: requests.Answered})
		for _, w := range waits {
			if d.sees(w.Request, p) && (!slices.Contains(waitingList, w.Request) || slices.Contains(answeredList, w.Request)) {
				t.Errorf("request %d waits on %s, and is listed waiting in %v and answered in %v", w.Request, p, waitingList, answeredList)
			}
		}
		for _, s := range got[[]requests.Summary](t)(d.List(p, requests.Filter{State: requests.Waiting})) {
			if slices.ContainsFunc(s.State.On, func(o requests.Party) bool { return o.Name == p }) &&
				!slices.ContainsFunc(waits, func(w requests.Wait) bool { return w.Request == s.Number }) {
				t.Errorf("request %d is listed waiting on %s, and nothing of it waits on them", s.Number, p)
			}
		}
		var waiting []int64
		for _, w := range waits {
			if !slices.Contains(waiting, w.Request) {
				waiting = append(waiting, w.Request)
			}
		}
		var listed []int64
		for _, s := range got[[]requests.Summary](t)(d.List(p, requests.Filter{WaitsOnMe: true})) {
			listed = append(listed, s.Number)
		}
		slices.Sort(waiting)
		slices.Sort(listed)
		if !slices.Equal(waiting, listed) {
			t.Errorf("%s's list of what waits holds %v, and what waits on them is %v", p, listed, waiting)
		}
		// What a bridge sends a person is what waits on them.
		for _, w := range waits {
			if w.Entry == 0 {
				continue
			}
			ds := got[[]requests.Delivery](t)(d.Deliveries(w.Entry))
			if !slices.ContainsFunc(ds, func(d requests.Delivery) bool { return d.Person == p }) {
				t.Errorf("entry %d waits on %s and is not carried to them: %+v", w.Entry, p, ds)
			}
		}
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("lee")); len(ws) != 2 || ws[0].Entry != decision.ID || ws[1].Entry != q.ID {
		t.Errorf("what waits on Lee is %+v", ws)
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("dana")); len(ws) != 1 || ws[0].Request != support {
		t.Errorf("what waits on Dana is %+v", ws)
	}
}

func TestRequestsWhoMaySayMore(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	kit := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	must(t, errOf(h.Answer(kit, h.Worker("steward"), "Fixed.", nil)))
	must(t, errOf(h.Say(kit, h.Person("kit"), "it sparked again", nil, requests.Screens)))
	if last := h.last(kit); h.state(kit).Is != requests.Working || last.By != h.Person("kit") || last.To[0] != h.Team("office") {
		t.Errorf("after Kit says more, the request ends %+v", last)
	}

	d := workDay(t)
	raj := requests.Identity("slack", slackRaj)
	if e := got[*requests.Entry](t)(d.Say(d.dana, raj, "same for 5512", nil, requests.Where{Bridge: "slack", Place: thread})); e.By != raj || e.To[0] != d.Team("office") || e.Read != "" {
		t.Errorf("Raj's entry in Dana's thread is %+v", e)
	}
	if e := got[*requests.Entry](t)(d.Say(d.dana, d.Person("raj"), "and 6623", nil, requests.Screens)); e.By != d.Person("raj") || e.To[0] != d.Team("office") {
		t.Errorf("Raj's entry on the screens is %+v", e)
	}
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Duplicate confirmed; £40 refunded.", nil)))
	if e := got[*requests.Entry](t)(d.Say(d.payments, d.Person("al"), "refund the fee too", nil, requests.Screens)); e.By != d.Person("al") || e.To[0] != d.Team("payments") || d.state(d.payments).Is != requests.Working {
		t.Errorf("Al's entry in payments' sub-request is %+v", e)
	}
	for _, who := range []requests.Who{d.Person("dana"), d.Person("raj"), raj, requests.Identity("slack", slackDana)} {
		may := got[bool](t)(d.MaySay(d.payments, who, requests.Screens))
		_, err := d.Say(d.payments, who, "me too", nil, requests.Screens)
		var refusal requests.Refusal
		if may || !errors.As(err, &refusal) {
			t.Errorf("%+v may say more in payments' sub-request: %v", who, err)
		}
	}
	// Through its runs, the asking team's workers may say more.
	if !got[bool](t)(d.MaySay(d.payments, d.Worker("bot"), requests.Where{})) || got[bool](t)(d.MaySay(d.payments, d.Worker("ops"), requests.Where{})) {
		t.Error("the workers who may say more in payments' sub-request are not the office's")
	}
}

func TestRequestsFailedRunLeavesAnswered(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	must(t, h.Failed(n, "the model refused: card 4111"))
	last := h.last(n)
	if h.state(n).Is != requests.Answered || last.By != requests.Bulldog || last.To[0] != h.Person("kit") || strings.Contains(last.Text, "4111") {
		t.Errorf("the failed request ends %+v", last)
	}
	if ds := got[[]requests.Delivery](t)(h.Deliveries(last.ID)); len(ds) != 1 || ds[0].Origin == nil || ds[0].Origin.Kind != requests.InChat {
		t.Errorf("Bulldog's entry to Kit goes to %+v", ds)
	}
	words := 0
	for _, e := range h.thread(n) {
		if strings.Contains(e.Text, "4111") {
			words++
			if e.Read != "office" || !slices.Equal(e.To, []requests.Who{h.Team("office")}) {
				t.Errorf("the error's words are in %+v", e)
			}
		}
	}
	if words != 1 {
		t.Errorf("the error's words are in %d entries", words)
	}
	must(t, errOf(h.Say(n, h.Person("kit"), "try again", nil, requests.Screens)))
	if h.state(n).Is != requests.Working {
		t.Error("saying more does not make the request working")
	}
}

func TestRequestsEmptyAnswerAtARoutineOrChannelSaysNothing(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	monday := got[*requests.Request](t)(h.AskByRoutine(office.Top, h.o.TopTeam().Routines[0], requests.Who{})).Number
	if e := got[*requests.Entry](t)(h.Answer(monday, h.Worker("steward"), " \n", nil)); e != nil {
		t.Errorf("the empty answer is the entry %+v", e)
	}
	if thread := h.thread(monday); h.state(monday).Is != requests.Answered || len(thread) != 1 || thread[0].Kind != requests.KindAsked {
		t.Errorf("the routine's request stands %+v, with the thread %+v", h.state(monday), thread)
	}
	if shown := h.shows(monday, "sam"); len(shown) != 1 {
		t.Errorf("Sam is shown %q", shown)
	}
	// At a channel place, too.
	garden := got[*requests.Request](t)(h.AskAt("telegram", "garden-chat", "garden-chat", "4472", "thanks", nil)).Number
	if e := got[*requests.Entry](t)(h.Answer(garden, h.Worker("gardener"), "", nil)); e != nil || h.state(garden).Is != requests.Answered || len(h.thread(garden)) != 1 {
		t.Errorf("the garden chat's request is answered with %+v, its thread %+v", e, h.thread(garden))
	}
	// Any other request's empty answer is an answer all the same: in the
	// chat, and in a direct message.
	kit := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	if e := got[*requests.Entry](t)(h.Answer(kit, h.Worker("steward"), "", nil)); e == nil || e.Kind != requests.KindAnswer || h.state(kit).Is != requests.Answered {
		t.Errorf("Kit's request is answered with %+v", e)
	}
	direct := got[*requests.Request](t)(h.AskAt("telegram", office.Direct, "4472", "4472", "the socket", nil)).Number
	if e := got[*requests.Entry](t)(h.Answer(direct, h.Worker("steward"), "", nil)); e == nil || e.Kind != requests.KindAnswer || h.state(direct).Is != requests.Answered {
		t.Errorf("Kit's direct message is answered with %+v", e)
	}
}

func TestRequestsStoppedOpenedOnlyByAPerson(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	n := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	stopped := got[[]int64](t)(h.Stop(n, h.Person("sam")))
	if !slices.Equal(stopped, []int64{n}) || h.last(n).Text != "stopped by Sam" {
		t.Fatalf("the stop stopped %v, ending %+v", stopped, h.last(n))
	}
	// The cancelled run says something before it ends.
	must(t, errOf(h.Answer(n, h.Worker("steward"), "Done!", nil)))
	must(t, errOf(h.Put(n, h.Worker("steward"), requests.KindTold, h.Person("kit"), "still here", nil)))
	if _, err := h.Put(n, h.Worker("fixer"), requests.KindQuestion, h.Person("kit"), "which socket?", nil); err == nil {
		t.Error("a question is put in a stopped request")
	}
	if r := got[*requests.Request](t)(h.Get(n)); !r.Stopped || !r.Answered {
		t.Errorf("what the cancelled run said opened the request: %+v", r)
	}
	// The cancelled run is reported failed: its team is told, and Bulldog's
	// last word to Kit is still who stopped it.
	must(t, h.Failed(n, "cancelled"))
	var toKit []string
	for _, e := range h.thread(n) {
		if e.By == requests.Bulldog && e.To[0] == h.Person("kit") {
			toKit = append(toKit, e.Text)
		}
	}
	if !slices.Equal(toKit, []string{"stopped by Sam"}) || !holds(h.shows(n, "sam"), "work failed: cancelled") {
		t.Errorf("after the stop, Bulldog tells Kit %q", toKit)
	}
	must(t, errOf(h.Say(n, h.Person("kit"), "please go on", nil, requests.Screens)))
	if r := got[*requests.Request](t)(h.Get(n)); r.Stopped || r.Answered {
		t.Errorf("Kit's saying more did not open the request: %+v", r)
	}
	// Stopping Dana's request cancels the bot's run: what it says after in
	// payments' sub-request opens nothing.
	d := workDay(t)
	must(t, errOf(d.Stop(d.dana, d.Person("al"))))
	must(t, errOf(d.Say(d.payments, d.Worker("bot"), "any news?", nil, requests.Where{})))
	if r := got[*requests.Request](t)(d.Get(d.payments)); !r.Stopped || !r.Answered {
		t.Errorf("what the cancelled run said opened the stopped sub-request: %+v", r)
	}
	// Lee stopping payments' sub-request cancels no run of the bot's, which
	// may say more there, and opens it.
	d = workDay(t)
	must(t, errOf(d.Stop(d.payments, d.Person("lee"))))
	must(t, errOf(d.Say(d.payments, d.Worker("bot"), "any news?", nil, requests.Where{})))
	if r := got[*requests.Request](t)(d.Get(d.payments)); r.Stopped || r.Answered || d.state(d.payments).Is != requests.Working {
		t.Errorf("what the asking team's run said did not open the stopped sub-request: %+v", r)
	}
}

func TestRequestsStopStopsBeneath(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, errOf(d.Write(d.payments, requests.Writing{Kind: "decision", By: d.Worker("ledger"),
		To: []requests.Who{d.Person("lee"), d.Person("priya")}, Text: "Refund one charge", Waits: []requests.Who{d.Person("lee"), d.Person("priya")}})))
	stopped := got[[]int64](t)(d.Stop(d.dana, d.Person("al")))
	if !slices.Equal(stopped, []int64{d.dana, d.payments, d.platform}) {
		t.Errorf("the stop stopped %v", stopped)
	}
	for _, n := range stopped {
		if d.state(n).Is != requests.Answered || d.last(n).Text != "stopped by Al" || d.last(n).By != requests.Bulldog {
			t.Errorf("request %d ends %+v", n, d.last(n))
		}
	}
	for _, p := range []string{"lee", "priya"} {
		if ws := got[[]requests.Wait](t)(d.WaitingOn(p)); len(ws) != 0 {
			t.Errorf("after the stop, what waits on %s is %+v", p, ws)
		}
	}
	// A sub-request answered over a question that still waits is still
	// open: the stop stops it, and closes the question.
	d = workDay(t)
	must(t, errOf(d.Put(d.payments, d.Worker("ledger"), requests.KindQuestion, d.Person("lee"), "which card?", nil)))
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Refunded.", nil)))
	stopped = got[[]int64](t)(d.Stop(d.dana, d.Person("al")))
	if !slices.Equal(stopped, []int64{d.dana, d.payments, d.platform}) {
		t.Errorf("the stop over the answered sub-request stopped %v", stopped)
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("lee")); len(ws) != 0 {
		t.Errorf("after the stop, what waits on Lee is %+v", ws)
	}
	if st := d.state(d.payments); st.Is != requests.Answered || d.last(d.payments).Text != "stopped by Al" {
		t.Errorf("payments' sub-request stands %+v, ending %+v", st, d.last(d.payments))
	}
	if tree := got[[]requests.Sub](t)(d.Tree(d.dana)); tree[0].Answer != "stopped by Al" {
		t.Errorf("payments' answer is %+v", tree[0])
	}
	// One answered with nothing waiting in it keeps its answer.
	d = workDay(t)
	must(t, errOf(d.Answer(d.payments, d.Worker("ledger"), "Refunded.", nil)))
	if stopped := got[[]int64](t)(d.Stop(d.dana, d.Person("al"))); !slices.Equal(stopped, []int64{d.dana, d.platform}) || d.last(d.payments).Text != "Refunded." {
		t.Errorf("the stop stopped %v, and payments' sub-request ends %+v", stopped, d.last(d.payments))
	}
}

func TestRequestsStopWithALineOfItsOwn(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	must(t, errOf(d.Put(d.payments, d.Worker("ledger"), requests.KindQuestion, d.Person("lee"), "which card?", nil)))
	const line = "the office did not disclose its answer"
	stopped := got[[]int64](t)(d.StopWith(d.dana, line))
	if !slices.Equal(stopped, []int64{d.dana, d.payments, d.platform}) {
		t.Errorf("the stop stopped %v", stopped)
	}
	var told []string
	for _, e := range d.thread(d.dana) {
		if e.By == requests.Bulldog {
			told = append(told, e.Text)
		}
	}
	if r := got[*requests.Request](t)(d.Get(d.dana)); !r.Stopped || d.state(d.dana).Is != requests.Answered || !slices.Equal(told, []string{line}) ||
		d.last(d.dana).To[0] != requests.Identity("slack", slackDana) {
		t.Errorf("Dana's request stands %+v, Bulldog saying %q", d.state(d.dana), told)
	}
	for _, n := range []int64{d.payments, d.platform} {
		if d.state(n).Is != requests.Answered || d.last(n).Text != "stopped by Bulldog" {
			t.Errorf("request %d ends %+v", n, d.last(n))
		}
	}
	if ws := got[[]requests.Wait](t)(d.WaitingOn("lee")); len(ws) != 0 {
		t.Errorf("after the stop, what waits on Lee is %+v", ws)
	}
}

func TestRequestsStopSubStopsOnlyBeneath(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	below := got[*requests.Request](t)(d.Sub(d.payments, d.Worker("ledger"), "platform", "When did the retry break?", "ticket-below")).Number
	stopped := got[[]int64](t)(d.Stop(d.payments, d.Person("lee")))
	if !slices.Equal(stopped, []int64{d.payments, below}) {
		t.Errorf("the stop stopped %v", stopped)
	}
	if d.state(d.dana).Is != requests.Working || d.state(d.platform).Is != requests.Working {
		t.Error("stopping payments' sub-request stopped what is above it or beside it")
	}
	tree := got[[]requests.Sub](t)(d.Tree(d.dana))
	if tree[0].Number != d.payments || tree[0].Answer != "stopped by Lee" {
		t.Errorf("payments' answer is %+v", tree[0])
	}
}

func TestRequestsSourceStatusShown(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	issue := "platform/api#51"
	r := got[*requests.Request](t)(d.AskAt("gitlab", "platform/api", issue, gitlabAl, "the retry is broken", nil))
	al := requests.Identity("gitlab", gitlabAl)
	must(t, errOf(d.Source("gitlab", issue, requests.Status{Status: "opened", By: al, At: d.clock.Now(), Link: "https://gitlab/51"})))
	d.clock.Move(time.Hour)
	told := 0
	stop := d.Watch(func(int64) { told++ })
	defer stop()
	must(t, errOf(d.Source("gitlab", issue, requests.Status{Status: "in review", By: al, At: d.clock.Now(), Link: "https://gitlab/51"})))
	v := got[*requests.View](t)(d.View(r.Number, "al"))
	if v.Source == nil || v.Source.Status != "in review" || !v.Source.At.Equal(d.clock.Now()) || v.Source.Link != "https://gitlab/51" || v.State.Is != requests.Working {
		t.Errorf("the request shows %+v, standing %+v", v.Source, v.State)
	}
	if len(d.thread(r.Number)) != 1 {
		t.Error("a status reported is an entry")
	}
	if told != 1 {
		t.Errorf("the watchers are told %d times", told)
	}
	later := got[*requests.Request](t)(d.AskAt("gitlab", "platform/api", issue, gitlabAl, "and the deploy", nil)).Number
	if n, ok, err := d.LatestAt("gitlab", issue); err != nil || !ok || n != later {
		t.Errorf("the latest request at the issue is %d", n)
	}
	if _, ok, err := d.LatestAt("gitlab", "platform/api#99"); err != nil || ok {
		t.Error("a request is found at an issue never asked at")
	}
	if stopped := got[[]int64](t)(d.Source("gitlab", "platform/api#99", requests.Status{Status: "closed", Closed: true, By: al})); len(stopped) != 0 || d.count("requests_requests") != 2 {
		t.Error("a status for no request made something")
	}
}

func TestRequestsClosedAtSourceStops(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	issue := "platform/api#51"
	r := got[*requests.Request](t)(d.AskAt("gitlab", "platform/api", issue, gitlabAl, "the retry is broken", nil)).Number
	sub := got[*requests.Request](t)(d.Sub(r, d.Worker("ops"), "payments", "were charges doubled?", "t")).Number
	al := requests.Identity("gitlab", gitlabAl)
	stopped := got[[]int64](t)(d.Source("gitlab", issue, requests.Status{Status: "closed", Closed: true, By: al, At: d.clock.Now()}))
	if !slices.Equal(stopped, []int64{r, sub}) || d.last(r).Text != "closed at gitlab by Al" || d.last(r).By != requests.Bulldog {
		t.Errorf("closing the issue stopped %v, ending %+v", stopped, d.last(r))
	}
	if st := d.state(r); st.Is != requests.Answered || len(st.On) != 0 {
		t.Errorf("the closed issue's request stands %+v", st)
	}
	if again := got[[]int64](t)(d.Source("gitlab", issue, requests.Status{Status: "reopened", By: al, At: d.clock.Now()})); len(again) != 0 || d.state(r).Is != requests.Answered {
		t.Error("reopening the issue started something")
	}
	must(t, errOf(d.Say(r, al, "still broken", nil, requests.Where{Bridge: "gitlab", Place: issue})))
	if d.state(r).Is != requests.Working {
		t.Error("Al's comment on the issue does not start its request again")
	}
}

func TestRequestsSourceStatusNeverSet(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	issue := "platform/api#51"
	r := got[*requests.Request](t)(d.AskAt("gitlab", "platform/api", issue, gitlabAl, "the retry is broken", nil)).Number
	st := requests.Status{Status: "opened", By: requests.Identity("gitlab", gitlabAl), At: d.clock.Now()}
	must(t, errOf(d.Source("gitlab", issue, st)))
	must(t, errOf(d.Answer(r, d.Worker("ops"), "Fixed in 813.", nil)))
	must(t, errOf(d.Stop(r, d.Person("al"))))
	must(t, errOf(d.Say(r, d.Person("al"), "and again", nil, requests.Screens)))
	if got := got[*requests.Request](t)(d.Get(r)).Source; got == nil || got.Status != "opened" || got.Closed {
		t.Errorf("the issue's status is %+v", got)
	}
}

func TestRequestsHeldAnswerNotHandedWaitsOnTheLeadAgain(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "support/team.yaml", "lead: dana", "lead: dana\ndisclosure: [./check.js]")
	files["support/check.js"] = `return { says: "yes" }`
	d := open(t, files)
	var handed []int64
	d.Held = func(e *requests.Entry) error {
		handed = append(handed, e.ID)
		return errors.New("the decisions cannot be read")
	}
	must(t, d.Readers("slack", "#help/1", []string{"U0DANA00"}))
	asked := got[*requests.Request](t)(d.AskAt("slack", "#help", "#help/1", "U0DANA00", "help", nil)).Number
	support := got[*requests.Request](t)(d.Sub(asked, d.Worker("bot"), "support", "What did we tell 4411?", "t")).Number
	e, err := d.Say(support, d.Person("dana"), "a refund", nil, requests.Screens)
	if e == nil || e.Kind != requests.KindHeld || err == nil || !strings.Contains(err.Error(), "the decisions cannot be read") || !slices.Equal(handed, []int64{e.ID}) {
		t.Fatalf("Dana's answer not handed is %+v, %v, handed %v", e, err, handed)
	}
	if st := d.state(support); st.Is != requests.Waiting || !slices.Equal(on(st), []string{"dana"}) {
		t.Errorf("with Dana's answer not handed, support's sub-request stands %+v", st)
	}
}
