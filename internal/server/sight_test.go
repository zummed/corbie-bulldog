package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// refundOffice gives the work office with a disclosure check that lets
// everything payments says go, so that nothing in it needs a service.
func refundOffice(t *testing.T) map[string]string {
	files := workOffice(t)
	delete(files, "payments/no-account-numbers.yaml")
	delete(files, "payments/no-account-numbers.js")
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "disclosure: [./no-account-numbers.yaml]", "disclosure: [./disclosure.js]", 1)
	files["payments/disclosure.js"] = "return { says: \"yes\" }\n"
	return files
}

// playRefund plays the work office's workers: the bot asks platform and
// payments, ops answers, and the ledger asks to refund a charge, which
// waits for Lee or Priya, and then answers.
func playRefund(turn testkit.Turn) testkit.Reply {
	switch {
	case strings.Contains(turn.System, "You answer what is asked in #help"):
		if turn.Number == 1 && strings.Contains(turn.Prompt(), "charged twice") {
			return testkit.Use("ask_team", map[string]any{"team": "platform", "question": "What broke after Tuesday's release?"}).
				And("ask_team", map[string]any{"team": "payments", "question": "Was customer 4411 charged twice? Refund one."})
		}
		return testkit.Say("Merge request 812 broke the webhook retry, and payments refunded the second charge.")
	case strings.Contains(turn.System, "You look after GitLab"):
		return testkit.Say("Merge request 812 broke the webhook retry.")
	case strings.Contains(turn.System, "You keep the ledger"):
		if turn.Number == 1 {
			reply := testkit.Use("refund", map[string]any{"charge": "ch_2", "why": "charged twice, a minute apart"})
			reply.Text = "ch_2 is the same £40 as ch_1 on card 4111 1111 1111 1111, a minute later: a duplicate."
			return reply
		}
		return testkit.Say("Duplicate confirmed; £40 refunded.")
	}
	return testkit.Say("done")
}

// A day is the work office's day as far as the refund: Dana's request in
// #help, which Al and Raj read, and the sub-requests the bot asked of
// platform and of payments; platform has answered, and the ledger's refund
// waits for Lee or Priya.
type day struct {
	b                     *bench
	s                     *serving
	n, platform, payments int64
	refund                int64
}

func refundDay(t *testing.T, files map[string]string) day {
	b := newBench(t, files)
	b.says(playRefund)
	s := b.serve()
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.1", "readers", []any{"U0DANA00", "U0AL0000", "U0RAJ000"},
		"text", "customer 4411 was charged twice after Tuesday's release; refund one and tell me what broke"))
	d := day{b: b, s: s, n: s.at("slack", "#help/1700.1")}
	d.platform = s.subOf(d.n, "platform")
	d.payments = s.subOf(d.n, "payments")
	s.answered(d.platform)
	b.until("the refund waits on Lee and Priya", func() bool {
		ds, err := s.areasOf().Decisions.OfRequest(d.payments)
		if err != nil {
			t.Fatal(err)
		}
		for _, dec := range ds {
			if dec.Job != nil && dec.Job.Name == "refund" && slices.Contains(dec.Asked, "lee") {
				d.refund = dec.ID
			}
		}
		return d.refund != 0
	})
	return d
}

// decided has Lee say yes to the refund on the screens, and stays until
// Dana's request is answered.
func (d day) decided() {
	d.s.b.t.Helper()
	d.s.as("lee").post("/api/requests/"+itoa(d.payments)+"/decisions/"+itoa(d.refund), map[string]string{"says": "yes"}).is(http.StatusOK)
	d.s.answered(d.n)
}

func TestServerEveryoneSeesTheOffice(t *testing.T) {
	t.Parallel()
	s := newBench(t, workOffice(t)).serve()
	r := s.as("dana").get("/api/me").is(http.StatusOK)
	var me struct {
		Me     personOut   `json:"me"`
		Office officeOut   `json:"office"`
		Teams  []teamOut   `json:"teams"`
		People []personOut `json:"people"`
	}
	r.into(&me)
	if me.Me.Name != "dana" || !slices.Equal(me.Me.Teams, []string{"support"}) || me.Me.Decides || me.Office.Name != "Platform support" || me.Office.Zone != "Europe/London" {
		t.Errorf("Dana is told she is %+v, in %+v", me.Me, me.Office)
	}
	if len(me.Teams) != 4 {
		t.Fatalf("Dana sees the teams %+v", me.Teams)
	}
	payments := me.Teams[slices.IndexFunc(me.Teams, func(t teamOut) bool { return t.Name == "payments" })]
	if payments.About != "Charges, refunds and the ledger. Holds customer data." || !slices.Equal(payments.People, []string{"lee", "priya"}) ||
		payments.Lead != "ledger" || payments.Within != "office" {
		t.Errorf("Dana sees payments as %+v", payments)
	}
	raj := me.People[slices.IndexFunc(me.People, func(p personOut) bool { return p.Name == "raj" })]
	if raj.Title != "Backend." || !slices.Equal(raj.Bridges, []string{"slack", "gitlab"}) {
		t.Errorf("Dana sees Raj as %+v", raj)
	}
	for _, hidden := range []string{"U0RAJ000", "1187", "You keep the ledger", "A charge taken twice", "ledger.yaml"} {
		if strings.Contains(string(r.body), hidden) {
			t.Errorf("Dana's /api/me holds %q", hidden)
		}
	}
}

func TestServerTeamSeenByItsPeople(t *testing.T) {
	t.Parallel()
	files := refundOffice(t)
	files["payments/team.yaml"] += "models:\n  usual: { name: claude-haiku-4-5, secret: ANTHROPIC_KEY }\n"
	d := refundDay(t, files)
	var lee teamWhole
	d.s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&lee)
	ledger := lee.Workers[slices.IndexFunc(lee.Workers, func(w workerOut) bool { return w.Name == "ledger" })]
	refund := ledger.Tools[slices.IndexFunc(ledger.Tools, func(t toolOut) bool { return t.Name == "refund" })]
	if ledger.Type != "agent" || ledger.Model == "" || ledger.ModelName != "usual" || ledger.ModelFrom != "payments" ||
		!refund.Waits || !slices.Equal(refund.Deciders, []deciderOut{{Kind: "person", Name: "lee"}, {Kind: "person", Name: "priya"}}) {
		t.Errorf("Lee sees the ledger as %+v", ledger)
	}
	if !slices.Equal(lee.Disclosure, []deciderOut{{Kind: "rule", Name: "./disclosure.js"}}) || len(lee.Notes) != 1 || lee.Notes[0].Name != "refunds" {
		t.Errorf("Lee sees payments' disclosure %v and notes %v", lee.Disclosure, lee.Notes)
	}
	if !slices.ContainsFunc(lee.AtWork, func(a atWork) bool { return a.Worker == "ledger" && a.Request == d.payments }) {
		t.Errorf("Lee does not see the ledger at work on payments' request: %+v", lee.AtWork)
	}
	// The bot's model is the office's.
	var office teamWhole
	d.s.as("al").get("/api/teams/office").is(http.StatusOK).into(&office)
	if bot := office.Workers[0]; bot.Name != "bot" || bot.ModelFrom != "office" {
		t.Errorf("Al sees the bot as %+v", bot)
	}
	// Dana sees what /api/me gives of payments, and nothing more.
	dana := d.s.as("dana").get("/api/teams/payments").is(http.StatusOK).json()
	if dana["lead"] != "ledger" || dana["workers"] != nil || dana["notes"] != nil {
		t.Errorf("Dana sees payments as %v", dana)
	}
	// Al decides a change: he sees how payments is written, and not what
	// the ledger is doing.
	var al teamWhole
	r := d.s.as("al").get("/api/teams/payments").is(http.StatusOK)
	r.into(&al)
	if len(al.Workers) != 1 || len(al.Notes) != 1 || al.Problems == nil || al.Routines == nil {
		t.Errorf("Al sees payments as %+v", al)
	}
	if al.AtWork != nil || strings.Contains(string(r.body), "at_work") {
		t.Errorf("Al sees payments' workers at work: %s", r.body)
	}
}

func TestServerToolToldByWhereItResolves(t *testing.T) {
	t.Parallel()
	files := house(t)
	// The gardener is given the house's sparks, a machine of another team;
	// a tell of its own, written in place, which stands before the tell of
	// the talk tools Bulldog ships; and a wait on the clock Bulldog ships,
	// which is no call, so no tool Bulldog does.
	files["garden/gardener.yaml"] += `  sparks: { does: Which circuit is what, args: { question: string },
            job: { machine: ../sparks.yaml, input: { said: "${args.question}" } } }
  wait: { does: Wait until Monday, args: {}, job: { machine: ../.bulldog/clock.yaml, input: { due: "mon 09:00" } } }
  tell:
    does: Chalk it on the shed door
    args: { text: string }
    job:
      machine: { statechart: chalk, input: [text], initial: done, states: { done: { end: done, output: "${input.text}" } } }
      input: { text: "${args.text}" }
`
	s := newBench(t, files).serve()
	tools := func(team, worker string) map[string]toolOut {
		var whole teamWhole
		s.as("sam").get("/api/teams/" + team).is(http.StatusOK).into(&whole)
		out := map[string]toolOut{}
		for _, w := range whole.Workers {
			if w.Name == worker {
				for _, tool := range w.Tools {
					out[tool.Name] = tool
				}
			}
		}
		return out
	}
	shippedDoes := func(file, tool string) string {
		contents, _ := shipped.File(file)
		var tools map[string]struct{ Does string }
		if err := yaml.Unmarshal(contents, &tools); err != nil {
			t.Fatal(err)
		}
		return tools[tool].Does
	}
	for _, c := range []struct {
		team, worker string
		want         []toolOut
	}{
		{"office", "steward", []toolOut{
			{Name: "fixer", Does: "Mend something in the house. Say what is wrong and where", Kind: "worker", Worker: "fixer", File: "fixer.yaml"},
			{Name: "ask_team", Does: shippedDoes(shipped.Talk, "ask_team"), Kind: "bulldog"},
			{Name: "write_notes", Does: shippedDoes(shipped.Notes, "write_notes"), Kind: "bulldog"},
		}},
		{"office", "fixer", []toolOut{
			{Name: "sparks", Does: "Which circuit is what", Kind: "worker", Worker: "sparks", File: "sparks.yaml"},
			{Name: "cut_power", Does: "Turn one circuit off", Kind: "script", File: "tools/cut-power.js", Waits: true, Deciders: []deciderOut{{Kind: "person", Name: "sam"}}},
			{Name: "order_part", Does: "Order a part", Kind: "script", File: "tools/order.js", Waits: true, Deciders: []deciderOut{{Kind: "rule", Name: "./small-orders.js"}, {Kind: "person", Name: "sam"}}},
		}},
		{"garden", "gardener", []toolOut{
			{Name: "groundsman", Does: "Heavy work in the garden: digging, mowing and the shed", Kind: "worker", Worker: "groundsman", File: "garden/groundsman.yaml"},
			{Name: "sparks", Does: "Which circuit is what", Kind: "machine", File: "sparks.yaml"},
			{Name: "tell", Does: "Chalk it on the shed door", Kind: "machine"},
			{Name: "wait", Does: "Wait until Monday", Kind: "machine"},
			{Name: "ask_team", Does: shippedDoes(shipped.Talk, "ask_team"), Kind: "bulldog"},
		}},
	} {
		got := tools(c.team, c.worker)
		for _, want := range c.want {
			if !reflect.DeepEqual(got[want.Name], want) {
				t.Errorf("the %s's %s is %+v, not %+v", c.worker, want.Name, got[want.Name], want)
			}
		}
		if _, blob := got["blob_read"]; blob {
			t.Errorf("the %s is shown a tool every agent has for blobs", c.worker)
		}
	}
}

func TestServerBusyWorkerShownToItsTeam(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	for person, shown := range map[string]bool{"lee": true, "priya": true, "al": false, "dana": false, "raj": false} {
		body := string(d.s.as(person).get("/api/teams/payments").is(http.StatusOK).body)
		if strings.Contains(body, `"at_work"`) != shown {
			t.Errorf("%s is shown the ledger at work: %v", person, !shown)
		}
	}
	// Platform has no people; Al, whom its decisions reach, sees its
	// workers at work, and nobody else does.
	held, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	d.b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "is the pipeline green?") {
			close(held)
			<-release
		}
		return playRefund(turn)
	})
	d.s.as("raj").post("/api/requests", map[string]string{"team": "platform", "text": "is the pipeline green?"}).is(http.StatusOK)
	select {
	case <-held:
	case <-time.After(testkit.Patience):
		t.Fatal("ops was never asked")
	}
	for _, person := range []string{"dana", "raj"} {
		if strings.Contains(string(d.s.as(person).get("/api/teams/platform").body), "at_work") {
			t.Errorf("%s is shown platform's workers at work", person)
		}
	}
	if !strings.Contains(string(d.s.as("al").get("/api/teams/platform").body), `"worker":"ops"`) {
		t.Error("Al is not shown ops at work")
	}
}

func TestServerRequestsSeenAsRequestsSay(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	d.decided()
	var list struct{ Requests []summaryOut }
	d.s.as("dana").get("/api/requests?asked=me").is(http.StatusOK).into(&list)
	if len(list.Requests) != 1 || list.Requests[0].Number != d.n || list.Requests[0].State != "answered" || list.Requests[0].Origin.Place != "#help/1700.1" {
		t.Errorf("Dana asked %+v", list.Requests)
	}
	view := d.s.as("dana").get("/api/requests/" + itoa(d.n)).is(http.StatusOK)
	var v struct {
		Subs []struct{ Team, Answer string }
		Read bool
	}
	view.into(&v)
	if len(v.Subs) != 2 || v.Read || !strings.Contains(string(view.body), "Duplicate confirmed; £40 refunded.") || !strings.Contains(string(view.body), "Merge request 812") {
		t.Errorf("Dana sees her request as %s", view.body)
	}
	sub := d.s.as("dana").get("/api/requests/" + itoa(d.payments)).is(http.StatusOK)
	if !strings.Contains(string(sub.body), "Refund one charge, by its id: yes from Lee") {
		t.Errorf("Dana does not see the refund decided: %s", sub.body)
	}
	for _, r := range []reply{view, sub} {
		if strings.Contains(string(r.body), "4111") || strings.Contains(string(r.body), "a duplicate") {
			t.Errorf("Dana sees what payments read: %s", r.body)
		}
	}
	lee := d.s.as("lee").get("/api/requests/" + itoa(d.payments)).is(http.StatusOK)
	if !strings.Contains(string(lee.body), "a duplicate") || !strings.Contains(string(lee.body), `"read":true`) {
		t.Errorf("Lee does not see the ledger's lines: %s", lee.body)
	}
	d.s.as("lee").get("/api/requests/" + itoa(d.n)).is(http.StatusNotFound)
}

// watched fails the test unless a browser's watch of a run is answered
// with a status.
func watched(t *testing.T, br *browser, run string, status int) {
	t.Helper()
	if got := br.watches(run); got != status {
		t.Errorf("the watch of %s is answered %d, not %d", run, got, status)
	}
}

// runOf gives a request's runs of a worker, as the people of its team see
// them.
func (s *serving) runOf(person string, n int64, worker string) string {
	s.b.t.Helper()
	var v struct{ Runs []work.KeptRun }
	s.as(person).get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	for _, r := range v.Runs {
		if r.Worker == worker {
			return r.Name
		}
	}
	s.b.t.Fatalf("%s sees no run of %s in request %d: %+v", person, worker, n, v.Runs)
	return ""
}

func TestServerRunWatchedByItsTeam(t *testing.T) {
	t.Parallel()
	files := refundOffice(t)
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "decide: { everything: [lee, priya] }", "decide: { everything: [ops, lee, priya] }", 1)
	d := refundDay(t, files)
	ledger := d.s.runOf("lee", d.payments, "ledger")
	watched(t, d.s.as("lee"), ledger, http.StatusOK)
	watched(t, d.s.as("al"), ledger, http.StatusNotFound)
	ops := d.s.runOf("al", d.platform, "ops")
	watched(t, d.s.as("al"), ops, http.StatusOK)
	watched(t, d.s.as("dana"), ops, http.StatusNotFound)
	// Ops, asked to decide the refund, runs as payments': Lee watches it,
	// and Al does not.
	deciding := d.b.listed(d.b.identity(), map[string]string{work.LabelKind: decisions.Deciding})
	if len(deciding) != 1 || deciding[0].Labels["team"] != "payments" {
		t.Fatalf("the runs deciding are %+v", deciding)
	}
	watched(t, d.s.as("lee"), deciding[0].ID, http.StatusOK)
	watched(t, d.s.as("al"), deciding[0].ID, http.StatusNotFound)
	// Platform is renamed: its earlier run is watched by nobody.
	if err := os.Rename(filepath.Join(d.b.dir, "platform"), filepath.Join(d.b.dir, "infra")); err != nil {
		t.Fatal(err)
	}
	d.b.write(map[string]string{"office.yaml": strings.Replace(files["office.yaml"], "{ team: platform }", "{ team: infra }", 1)})
	d.b.until("every area has taken the renamed office", func() bool {
		d.b.clock.Move(0)
		o := d.s.areasOf().Work.Read()
		return o != nil && o.Teams["infra"] != nil
	})
	watched(t, d.s.as("al"), ops, http.StatusNotFound)
}

func TestServerFilesAndHistoryForDeciders(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	b.write(map[string]string{"bins.yaml": "agent: bins\nthis is not a definition: [\n"})
	b.until("the broken bins are read", func() bool {
		b.clock.Move(0)
		return len(s.areasOf().Keeper.Problems().OfWorker("bins")) > 0
	})
	var files struct{ Files []fileOf }
	s.as("sam").get("/api/office/files").is(http.StatusOK).into(&files)
	paths := map[string]bool{}
	for _, f := range files.Files {
		paths[f.Path] = f.ByHand
	}
	if _, ok := paths["notes/rooms.md"]; !ok || !paths["bins.yaml"] || paths["steward.yaml"] || len(paths) != len(house(t)) {
		t.Errorf("Sam sees the files %v", paths)
	}
	if text := s.as("sam").get("/api/office/file?path=notes/rooms.md").is(http.StatusOK).json()["text"]; text != house(t)["notes/rooms.md"] {
		t.Errorf("Sam reads the rooms note as %q", text)
	}
	var history struct{ History []struct{ ID, Why string } }
	s.as("sam").get("/api/history").is(http.StatusOK).into(&history)
	if len(history.History) != 1 {
		t.Errorf("Sam's history is %+v", history.History)
	}
	for _, path := range []string{"/api/office/files", "/api/office/file?path=notes/rooms.md", "/api/history", "/api/history/" + history.History[0].ID} {
		s.as("kit").get(path).is(http.StatusNotFound)
	}
	// Kit, of the house, sees its notes and its problems; not the
	// garden's, which she is not in.
	office := s.as("kit").get("/api/teams/office").is(http.StatusOK)
	if !strings.Contains(string(office.body), `"name":"rooms"`) || !strings.Contains(string(office.body), "bins.yaml") {
		t.Errorf("Kit sees the house as %s", office.body)
	}
	garden := s.as("kit").get("/api/teams/garden").is(http.StatusOK).json()
	if garden["notes"] != nil || garden["problems"] != nil {
		t.Errorf("Kit sees the garden as %v", garden)
	}
}

func TestServerFailingBridgeShownToTopTeam(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	s := b.serve()
	answered := make(chan reply, 1)
	go func() { answered <- s.nobody().post("/bridges/slack", "{}") }()
	run, _ := b.waiting(bridges.KindIn, "slack", standin.Heard, "")
	b.answerTicket(run, standin.Heard, map[string]any{"fail": "Slack's signature does not match"})
	<-answered
	b.until("the failure is kept", func() bool {
		troubles, err := s.areasOf().Bridges.Troubles()
		return err == nil && len(troubles) == 1
	})
	al := s.as("al").get("/api/me").is(http.StatusOK)
	if !strings.Contains(string(al.body), "Slack's signature does not match") {
		t.Errorf("Al is not shown the failing bridge: %s", al.body)
	}
	if failing := s.as("dana").get("/api/me").json()["failing"]; len(failing.([]any)) != 0 {
		t.Errorf("Dana is shown %v", failing)
	}
}

func TestServerProblemsShownToTheirTeams(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	s := b.serve()
	b.write(map[string]string{"payments/ledger.yaml": "agent: ledger\nsystem: [\n"})
	b.until("the broken ledger is read", func() bool {
		b.clock.Move(0)
		return len(s.areasOf().Keeper.Problems().OfWorker("ledger")) > 0
	})
	problems := func(person string) string {
		var me struct{ Problems []problemOut }
		s.as(person).get("/api/me").is(http.StatusOK).into(&me)
		data, _ := json.Marshal(me.Problems)
		return string(data)
	}
	for person, sees := range map[string]bool{"lee": true, "dana": false, "al": true} {
		if strings.Contains(problems(person), "payments/ledger.yaml") != sees {
			t.Errorf("%s sees the ledger's problem: %v", person, !sees)
		}
	}
	// A bridge's problem is the office's.
	b.write(map[string]string{"payments/ledger.yaml": workOffice(t)["payments/ledger.yaml"]})
	if err := os.RemoveAll(filepath.Join(b.dir, "bridges", "gitlab")); err != nil {
		t.Fatal(err)
	}
	b.until("the missing GitLab bridge is read", func() bool {
		b.clock.Move(0)
		o := s.areasOf().Work.Read()
		return o != nil && len(o.Problems.OfBridge("gitlab")) > 0
	})
	if !strings.Contains(problems("al"), `"bridge":"gitlab"`) || strings.Contains(problems("dana"), "gitlab") {
		t.Errorf("Al sees %s, and Dana %s", problems("al"), problems("dana"))
	}
	b.says(playRefund)
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1800.1", "text", "is the API up?"))
	s.answered(s.at("slack", "#help/1800.1"))
}

func TestServerDeciderGivenWithItsTeam(t *testing.T) {
	t.Parallel()
	files := house(t)
	// The garden's reviewer decides the house's orders after the
	// small-orders rule, and passes them to Sam.
	files["garden/reviewer.yaml"] = `agent: reviewer
input: [said]
model: usual
system: You review what is ordered. End with { "says": "yes", "refuse" or "pass", "note": why }.
prompt: ${input.said}
`
	files["office.yaml"] = strings.Replace(files["office.yaml"], "order_part: [./small-orders.js, sam]", "order_part: [./small-orders.js, reviewer, sam]", 1)
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.System, "You review what is ordered") {
			return testkit.Say(`{ "says": "pass", "note": "For Sam." }`)
		}
		return playHouse(turn)
	})
	s := b.serve()
	want := []deciderOut{{Kind: "rule", Name: "./small-orders.js"}, {Kind: "worker", Name: "reviewer", Team: "garden"}, {Kind: "person", Name: "sam"}}
	var whole teamWhole
	s.as("sam").get("/api/teams/office").is(http.StatusOK).into(&whole)
	list := whole.Decide[slices.IndexFunc(whole.Decide, func(l listOut) bool { return l.Key == "order_part" })]
	if !slices.Equal(list.Deciders, want) {
		t.Errorf("Sam is given the house's order_part as %+v", list.Deciders)
	}

	n := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "the boiler pump is dead"}))
	s.waitsOn(n, "sam")
	var v struct{ Decisions []decisionOut }
	s.as("sam").get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	if len(v.Decisions) != 1 || len(v.Decisions[0].Passes) != 2 || v.Decisions[0].Passes[1].Name != "reviewer" || v.Decisions[0].Passes[1].Team != "garden" ||
		v.Decisions[0].Passes[0].Team != "" {
		t.Errorf("Sam is shown the order as %+v", v.Decisions)
	}
}
