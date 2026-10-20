package server

import (
	"encoding/json"
	"net/http"
	"net/url"
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
	"github.com/zummed/corbie-bulldog/internal/requests"
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

// meOf gives what a person's /api/me holds.
func (s *serving) meOf(person string) meOut {
	s.b.t.Helper()
	var me meOut
	s.as(person).get("/api/me").is(http.StatusOK).into(&me)
	return me
}

// bridgeIn gives the bridge of a name as /api/me gives it.
func bridgeIn(t *testing.T, me meOut, name string) bridgeOut {
	t.Helper()
	i := slices.IndexFunc(me.Office.Bridges, func(b bridgeOut) bool { return b.Name == name })
	if i < 0 {
		t.Fatalf("%s is given no bridge %s: %+v", me.Me.Name, name, me.Office.Bridges)
	}
	return me.Office.Bridges[i]
}

// teamIn gives the team of a name as /api/me gives it.
func teamIn(t *testing.T, me meOut, name string) teamOut {
	t.Helper()
	i := slices.IndexFunc(me.Teams, func(u teamOut) bool { return u.Name == name })
	if i < 0 {
		t.Fatalf("%s is given no team %s", me.Me.Name, name)
	}
	return me.Teams[i]
}

func TestServerEveryoneSeesTheOffice(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["office.yaml"] += "heed: ./.bulldog/rules/mentioned/mentioned.js\n"
	s := newBench(t, files).serve()
	r := s.as("dana").get("/api/me").is(http.StatusOK)
	var me meOut
	r.into(&me)
	if me.Me.Name != "dana" || !slices.Equal(me.Me.Teams, []string{"support"}) || me.Me.Decides || me.Office.Name != "Platform support" || me.Office.Zone != "Europe/London" {
		t.Errorf("Dana is told she is %+v, in %+v", me.Me, me.Office)
	}
	if len(me.Teams) != 4 {
		t.Fatalf("Dana sees the teams %+v", me.Teams)
	}
	payments := teamIn(t, me, "payments")
	if payments.About != "Charges, refunds and the ledger. Holds customer data." || !slices.Equal(payments.People, []string{"lee", "priya"}) ||
		payments.Lead != "ledger" || !payments.LeadRuns || payments.Within != "office" || payments.Reached != nil || payments.Discloses != nil {
		t.Errorf("Dana sees payments as %+v", payments)
	}
	if platform := teamIn(t, me, "platform"); len(platform.People) != 0 || !slices.Equal(platform.Reached, []string{"al"}) {
		t.Errorf("Dana sees platform, which has no people, as %+v", platform)
	}
	if heed := teamIn(t, me, "office").Heed; heed == nil || heed.Rule != "" {
		t.Errorf("Dana sees the office's heed as %+v", heed)
	}
	raj := me.People[slices.IndexFunc(me.People, func(p personOut) bool { return p.Name == "raj" })]
	if raj.Title != "Backend." || !slices.Equal(raj.Bridges, []string{"slack", "gitlab"}) {
		t.Errorf("Dana sees Raj as %+v", raj)
	}
	if want := []deciderOut{{Kind: "person", Name: "al"}}; !reflect.DeepEqual(me.Office.OfficePut, want) {
		t.Errorf("Dana is told a change is decided by %+v", me.Office.OfficePut)
	}
	if me.Office.Runs != (runsOut{Limit: 3}) {
		t.Errorf("Dana is told the runs are %+v", me.Office.Runs)
	}
	// The tests' office runs stand-ins for the bridges, in folders of its
	// own.
	slack, gitlab := bridgeIn(t, me, "slack"), bridgeIn(t, me, "gitlab")
	if slack.Folder != "bridges/slack" || slack.Shipped || !slack.Hears || slack.Listens || !strings.HasSuffix(slack.Address, "/bridges/slack") ||
		slack.Status != "running" || !reflect.DeepEqual(slack.Places, []placeOut{{Name: "#help", Team: "office", Guests: true}}) {
		t.Errorf("Dana sees the Slack bridge as %+v", slack)
	}
	if gitlab.Folder != "bridges/gitlab" || !gitlab.Listens || gitlab.Hears || gitlab.Address != "" ||
		!reflect.DeepEqual(gitlab.Places, []placeOut{{Name: "platform/api", Team: "platform"}}) {
		t.Errorf("Dana sees the GitLab bridge as %+v", gitlab)
	}
	for _, hidden := range []string{"U0RAJ000", "1187", "You keep the ledger", "A charge taken twice", "ledger.yaml", "mentioned.js", "gitlab.com"} {
		if strings.Contains(string(r.body), hidden) {
			t.Errorf("Dana's /api/me holds %q", hidden)
		}
	}
	// The people of a team, and the people who decide a change, see its
	// heed's rule and whether it sets a disclosure.
	al := s.meOf("al")
	if heed := teamIn(t, al, "office").Heed; heed == nil || heed.Rule != ".bulldog/rules/mentioned/mentioned.js" {
		t.Errorf("Al sees the office's heed as %+v", heed)
	}
	if d := teamIn(t, al, "payments").Discloses; d == nil || !*d {
		t.Errorf("Al, who decides a change, is told payments discloses %v", d)
	}
	lee := s.meOf("lee")
	if d := teamIn(t, lee, "payments").Discloses; d == nil || !*d {
		t.Errorf("Lee is told payments discloses %v", d)
	}
	if u := teamIn(t, lee, "support"); u.Discloses != nil {
		t.Errorf("Lee sees support as %+v", u)
	}
	// The house's Telegram bridge listens in its direct messages, which
	// ask the house.
	house := newBench(t, house(t)).serve().meOf("sam")
	if telegram := bridgeIn(t, house, "telegram"); !telegram.Listens || !reflect.DeepEqual(telegram.Places, []placeOut{{Name: "direct", Team: "office"}}) {
		t.Errorf("Sam sees the Telegram bridge as %+v", telegram)
	}
}

func TestServerBridgeSettingsAndReachToChangeDeciders(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	job := `input: { fail: "${event.data.fail}" } }`
	if !strings.Contains(files["bridges/slack/in.yaml"], job) {
		t.Fatal("the stand-in's in machine has no job to allow anything")
	}
	files["bridges/slack/in.yaml"] = strings.Replace(files["bridges/slack/in.yaml"], job,
		`input: { fail: "${event.data.fail}" }, allow: [net:slack.com, "secret:SLACK_TOKEN@net:slack.com", "secret:SLACK_SIGNING_SECRET@*"] }`, 1)
	// The GitLab bridge's in machine reaches the site its settings name,
	// and a host its cursor gives, which is known only once it runs; its out machine reaches anything.
	files["bridges/gitlab/in.yaml"] = strings.Replace(files["bridges/gitlab/in.yaml"], job,
		`input: { fail: "${event.data.fail}" }, allow: ["net:${input.settings.site}", "net:${input.cursor}", "secret:GITLAB_TOKEN@net:${input.settings.site}", "secret:GITLAB_TOKEN@net.unencrypted:${input.settings.site}"] }`, 1)
	files["bridges/gitlab/out.yaml"] = strings.Replace(files["bridges/gitlab/out.yaml"], job,
		`input: { fail: "${event.data.fail}" }, allow: [network, all, "secret:SIGNER@exec:gpg"] }`, 1)
	s := newBench(t, files).serve()
	al := s.meOf("al")
	slack, gitlab := bridgeIn(t, al, "slack"), bridgeIn(t, al, "gitlab")
	if gitlab.Status != "running" {
		t.Fatalf("the GitLab bridge is %s", gitlab.Status)
	}
	want := map[string]any{"places": map[string]any{"#help": map[string]any{"guests": true}}}
	if !reflect.DeepEqual(slack.Settings, want) {
		t.Errorf("Al is given the Slack bridge's settings as %#v", slack.Settings)
	}
	in := &machineOut{File: "bridges/slack/in.yaml", reachOut: reachOut{Hosts: []string{"slack.com"},
		Secrets: []secretOut{{Name: "SLACK_SIGNING_SECRET", To: []string{"*"}}, {Name: "SLACK_TOKEN", To: []string{"slack.com"}}}}}
	if !reflect.DeepEqual(slack.In, in) {
		t.Errorf("Al is given the Slack bridge's in machine as %+v", slack.In)
	}
	if out := (&machineOut{File: "bridges/slack/out.yaml", reachOut: reachOut{Hosts: []string{}, Secrets: []secretOut{}}}); !reflect.DeepEqual(slack.Out, out) {
		t.Errorf("Al is given the Slack bridge's out machine as %+v", slack.Out)
	}
	if settings, ok := gitlab.Settings.(map[string]any); !ok || settings["site"] != "gitlab.com" {
		t.Errorf("Al is given the GitLab bridge's settings as %#v", gitlab.Settings)
	}
	in = &machineOut{File: "bridges/gitlab/in.yaml", reachOut: reachOut{Hosts: []string{"${input.cursor}", "gitlab.com"},
		Secrets: []secretOut{{Name: "GITLAB_TOKEN", To: []string{"net.unencrypted:gitlab.com", "gitlab.com"}}}}}
	if !reflect.DeepEqual(gitlab.In, in) {
		t.Errorf("Al is given the GitLab bridge's in machine as %+v", gitlab.In)
	}
	out := &machineOut{File: "bridges/gitlab/out.yaml", reachOut: reachOut{Hosts: []string{"*"},
		Secrets: []secretOut{{Name: "*", To: []string{"*"}}, {Name: "SIGNER", To: []string{"exec:gpg"}}}}}
	if !reflect.DeepEqual(gitlab.Out, out) {
		t.Errorf("Al is given the GitLab bridge's out machine as %+v", gitlab.Out)
	}
	r := s.as("dana").get("/api/me").is(http.StatusOK)
	var dana meOut
	r.into(&dana)
	for _, b := range dana.Office.Bridges {
		if b.Settings != nil || b.In != nil || b.Out != nil {
			t.Errorf("Dana is given the %s bridge as %+v", b.Name, b)
		}
	}
	for _, hidden := range []string{`"settings"`, "slack.com", "SLACK_TOKEN", "in.yaml"} {
		if strings.Contains(string(r.body), hidden) {
			t.Errorf("Dana's /api/me holds %s", hidden)
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
	if want := []deciderOut{{Kind: "rule", Name: "payments/disclosure.js"}, {Kind: "person", Name: "lee"}, {Kind: "person", Name: "priya"}}; !slices.Equal(lee.Disclosure, want) ||
		len(lee.Notes) != 1 || lee.Notes[0].Name != "refunds" {
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
	if al.AtWork != nil || al.Now != nil || strings.Contains(string(r.body), "at_work") || strings.Contains(string(r.body), `"now"`) {
		t.Errorf("Al sees payments' workers at work: %s", r.body)
	}
}

func TestServerTeamSeesWhereItsRequestsStandNow(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	// Lee sees payments' sub-request waiting on the refund, which waits on
	// Lee and Priya.
	var lee teamWhole
	d.s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&lee)
	i := slices.IndexFunc(lee.Now, func(n nowOut) bool { return n.Kind == decisions.OfAction })
	if i < 0 {
		t.Fatalf("Lee sees going on in payments %+v", lee.Now)
	}
	if refund := lee.Now[i]; refund.Request != d.payments || refund.Decision != d.refund || !strings.Contains(refund.Does, "Refund") ||
		!slices.Equal(refund.Asked, []string{"lee", "priya"}) || refund.First == "" {
		t.Errorf("Lee sees the refund as %+v", refund)
	}
	// Al, of the office, sees Dana's request waiting on payments, whose
	// refund waits on Lee and Priya; and platform's answer is no part of
	// it, since platform answered.
	var al teamWhole
	d.s.as("al").get("/api/teams/office").is(http.StatusOK).into(&al)
	var subs []nowOut
	for _, n := range al.Now {
		if n.Kind == nowSub {
			subs = append(subs, n)
		}
	}
	if len(subs) != 1 || subs[0].Request != d.n || subs[0].Sub != d.payments || subs[0].Team != "payments" || subs[0].State != requests.Waiting || len(subs[0].On) != 2 {
		t.Errorf("Al sees the sub-requests open %+v, of %+v", subs, al.Now)
	}
	// Al asks the office something it answers at once. Once the refund is
	// decided and Dana answered, the office holds her request as answered
	// last, before his, though his is the newer, and payments nothing
	// waiting.
	var asked struct{ Number int64 }
	d.s.as("al").post("/api/requests", map[string]string{"team": "office", "text": "what broke?"}).is(http.StatusOK).into(&asked)
	d.s.as("al").until(asked.Number, "is answered", func(v viewOut) bool { return v.State == requests.Answered })
	d.b.clock.Move(time.Minute)
	d.decided()
	al = teamWhole{}
	d.s.as("al").get("/api/teams/office").is(http.StatusOK).into(&al)
	if len(al.Now) != 2 || al.Now[0].Kind != nowAnswered || al.Now[0].Request != d.n || al.Now[0].Label != "" || al.Now[1].Request != asked.Number {
		t.Errorf("Al sees going on in the office %+v", al.Now)
	}
	lee = teamWhole{}
	d.s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&lee)
	if slices.ContainsFunc(lee.Now, func(n nowOut) bool { return n.Kind != nowAnswered }) {
		t.Errorf("Lee sees going on in payments, all of it answered, %+v", lee.Now)
	}
}

func TestServerTeamSeesItsHeedWeighAndItsLeadAnswer(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "\nlead: bot\n", "\nlead: bot\nheed: ./for-us.yaml\n", 1)
	files["for-us.yaml"] = "agent: for-us\ninput: { said: string }\nmodel: usual\nprompt: \"Weigh: ${input.said}\"\n"
	b := newBench(t, files)
	held, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "Weigh:") {
			close(held)
			<-release
			return testkit.Say(`{ "says": "note" }`)
		}
		return testkit.Say("")
	})
	s := b.serve()
	// While the heed weighs Dana's word at #help, Al, of the office, sees
	// it beside the office, by the heed's rule and its run.
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1700.1", "readers", []any{"U0DANA00"}, "text", "is the API down?"))
	n := s.at("slack", "#help/1700.1")
	select {
	case <-held:
	case <-time.After(testkit.Patience):
		t.Fatal("the heed was never asked")
	}
	var al teamWhole
	b.until("Al sees the heed weigh", func() bool {
		al = teamWhole{}
		s.as("al").get("/api/teams/office").is(http.StatusOK).into(&al)
		return slices.ContainsFunc(al.Now, func(x nowOut) bool { return x.Kind == nowHeed })
	})
	if x := al.Now[slices.IndexFunc(al.Now, func(x nowOut) bool { return x.Kind == nowHeed })]; x.Request != n || x.Rule != "for-us.yaml" || x.Run == "" {
		t.Errorf("Al sees the heed weigh as %+v", x)
	}
	// Raj's request to support waits on Dana, its lead: she sees it so,
	// and Al, not of support, sees nothing going on in it.
	asked := s.as("raj").post("/api/requests", map[string]string{"team": "support", "text": "a customer asks about an invoice"}).is(http.StatusOK)
	var made struct{ Number int64 }
	asked.into(&made)
	var dana teamWhole
	s.as("dana").get("/api/teams/support").is(http.StatusOK).into(&dana)
	if i := slices.IndexFunc(dana.Now, func(x nowOut) bool { return x.Kind == nowLead }); i < 0 || dana.Now[i].Request != made.Number ||
		len(dana.Now[i].On) != 1 || dana.Now[i].On[0].Name != "dana" {
		t.Errorf("Dana sees going on in support %+v", dana.Now)
	}
	if body := string(s.as("al").get("/api/teams/support").body); strings.Contains(body, `"now"`) {
		t.Errorf("Al is shown what is going on in support: %s", body)
	}
}

func TestServerNotesShownWithWhatARunIsGiven(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["payments/notes/a.md"] = strings.Repeat("é", 30_000)
	files["payments/notes/b.md"] = strings.Repeat("b", 30_000)
	files["payments/notes/c.md"] = strings.Repeat("c", 10)
	s := newBench(t, files).serve()
	var lee teamWhole
	s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&lee)
	type given struct {
		name        string
		size, given int
	}
	var seen []given
	for _, n := range lee.Notes {
		seen = append(seen, given{n.Name, n.Size, n.Given})
	}
	refunds := len([]rune(files["payments/notes/refunds.md"]))
	if want := []given{{"a", 30_000, 30_000}, {"b", 30_000, 20_000}, {"c", 10, 0}, {"refunds", refunds, 0}}; !slices.Equal(seen, want) {
		t.Errorf("Lee sees payments' notes as %v, not %v", seen, want)
	}
	if lee.Notes[1].Text != files["payments/notes/b.md"] {
		t.Errorf("Lee is not given b whole")
	}
	// The ledger names the notes kit.
	if !slices.Equal(lee.Writers, []string{"ledger"}) {
		t.Errorf("Lee sees payments' notes written by %#v", lee.Writers)
	}
	if dana := s.as("dana").get("/api/teams/payments").is(http.StatusOK).json(); dana["notes"] != nil || dana["writers"] != nil {
		t.Errorf("Dana sees payments' notes: %v, written by %v", dana["notes"], dana["writers"])
	}
	// In the house, the steward names it, and so writes the house's notes,
	// and the gardener the garden's; the fixer, naming only the talk tools,
	// writes none.
	h := newBench(t, house(t)).serve()
	for team, want := range map[string][]string{"office": {"steward"}, "garden": {"gardener"}} {
		var sam teamWhole
		h.as("sam").get("/api/teams/" + team).is(http.StatusOK).into(&sam)
		if !slices.Equal(sam.Writers, want) {
			t.Errorf("Sam sees the %s's notes written by %v, not %v", team, sam.Writers, want)
		}
	}
}

func TestServerToolToldByWhereItResolves(t *testing.T) {
	t.Parallel()
	files := house(t)
	// The fixer is given the docs tools too.
	files["fixer.yaml"] = strings.Replace(files["fixer.yaml"], "tools_file: [./.bulldog/tools/talk/talk.yaml]", "tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]", 1)
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
      machine: { statechart: chalk, input: { text: string }, initial: done, states: { done: { end: done, output: "${input.text}" } } }
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
			{Name: "ask_team", Does: shippedDoes(shipped.Talk, "ask_team"), Kind: "bulldog", Kit: "talk"},
			{Name: "write_notes", Does: shippedDoes(shipped.Notes, "write_notes"), Kind: "bulldog", Kit: "notes"},
		}},
		{"office", "fixer", []toolOut{
			{Name: "sparks", Does: "Which circuit is what", Kind: "worker", Worker: "sparks", File: "sparks.yaml"},
			{Name: "cut_power", Does: "Turn one circuit off", Kind: "script", File: "tools/cut-power.js", Waits: true, Deciders: []deciderOut{{Kind: "person", Name: "sam"}}},
			{Name: "order_part", Does: "Order a part", Kind: "script", File: "tools/order.js", Waits: true, Deciders: []deciderOut{{Kind: "rule", Name: "small-orders.js"}, {Kind: "worker", Name: "clerk", Team: "office", Advises: true}, {Kind: "person", Name: "sam"}}},
			{Name: "read_doc", Does: shippedDoes(shipped.Docs, "read_doc"), Kind: "bulldog", Kit: "docs"},
		}},
		{"office", "builder", []toolOut{
			{Name: "office_put", Does: shippedDoes(shipped.Build, "office_put"), Kind: "bulldog", Kit: "build", Waits: true, Deciders: []deciderOut{{Kind: "person", Name: "sam"}}},
		}},
		{"garden", "gardener", []toolOut{
			{Name: "groundsman", Does: "Heavy work in the garden: digging, mowing and the shed", Kind: "worker", Worker: "groundsman", File: "garden/groundsman.yaml"},
			{Name: "sparks", Does: "Which circuit is what", Kind: "machine", File: "sparks.yaml"},
			{Name: "tell", Does: "Chalk it on the shed door", Kind: "machine"},
			{Name: "wait", Does: "Wait until Monday", Kind: "machine"},
			{Name: "ask_team", Does: shippedDoes(shipped.Talk, "ask_team"), Kind: "bulldog", Kit: "talk"},
		}},
		{"office", "bins", []toolOut{
			{Name: "which", Kind: "script", File: "tools/bins.js"},
		}},
	} {
		got := tools(c.team, c.worker)
		for _, want := range c.want {
			tool := got[want.Name]
			if (tool.Reach != nil) != (want.Kind == "script" || want.Kind == "machine") {
				t.Errorf("the %s's %s is given what it reaches as %+v", c.worker, want.Name, tool.Reach)
			}
			if tool.Reach = nil; !reflect.DeepEqual(tool, want) {
				t.Errorf("the %s's %s is %+v, not %+v", c.worker, want.Name, got[want.Name], want)
			}
		}
		if _, blob := got["blob_read"]; blob {
			t.Errorf("the %s is shown a tool every agent has for blobs", c.worker)
		}
	}
	var whole teamWhole
	s.as("sam").get("/api/teams/office").is(http.StatusOK).into(&whole)
	for _, w := range whole.Workers {
		if !slices.Equal(w.MayAsk, []string{"garden"}) {
			t.Errorf("the %s may ask %v", w.Name, w.MayAsk)
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
		if strings.Contains(body, `"now"`) != shown {
			t.Errorf("%s is shown what is going on in payments: %v", person, !shown)
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
		if body := string(d.s.as(person).get("/api/teams/platform").body); strings.Contains(body, "at_work") || strings.Contains(body, `"now"`) {
			t.Errorf("%s is shown platform's workers at work: %s", person, body)
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
	if !strings.Contains(string(sub.body), "Refund one charge, by its id: approved by Lee") {
		t.Errorf("Dana does not see the refund decided: %s", sub.body)
	}
	var s struct {
		Entries   []entryOut
		Decisions []decisionOut
	}
	sub.into(&s)
	records := 0
	for _, e := range s.Entries {
		if e.Kind != decisions.KindDecision {
			continue
		}
		records++
		if !slices.ContainsFunc(s.Decisions, func(d decisionOut) bool { return d.ID == e.Decision && d.Kind == decisions.OfAction }) {
			t.Errorf("Dana's entry %q names decision %d, which she is not given as a held action: %+v", e.Text, e.Decision, s.Decisions)
		}
	}
	if records == 0 {
		t.Errorf("Dana is given no entry of the refund's decision: %s", sub.body)
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

func TestServerRequestWaitingItsTurnShownQueued(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, kit]\nruns: 1", 1)
	b := newBench(t, files)
	held, let := b.hold()
	s := b.serve()
	sam := s.as("sam")
	// How many wait their turn is counted apart from any write, and the
	// stream of changes says when it changes.
	changes := sam.stream("/api/events")
	first := s.chat("kit", "", "first")
	b.until("the first is held", func() bool { return held() == 1 })
	second, third := s.chat("sam", "", "second"), s.chat("sam", "", "third")
	for len(sam.listed("?state=queued")) != 2 {
		changes.until("the runs are counted again", func(e event) bool { return e.data == Runs })
	}
	if listed := sam.listed("?state=queued"); !slices.Equal(listed, []int64{third, second}) {
		t.Errorf("Sam's queued requests are %v", listed)
	}
	if listed := sam.listed("?state=working"); !slices.Equal(listed, []int64{first}) {
		t.Errorf("Sam's working requests are %v", listed)
	}
	if v := sam.view(third); v.State != requests.Working || v.Queued != 2 {
		t.Errorf("Sam is shown the third request %s, queued %d", v.State, v.Queued)
	}
	if v := sam.view(first); v.Queued != 0 {
		t.Errorf("Sam is shown the first request queued %d", v.Queued)
	}
	var house teamWhole
	sam.get("/api/teams/office").is(http.StatusOK).into(&house)
	var queued []nowOut
	for _, n := range house.Now {
		if n.Kind == nowQueued {
			queued = append(queued, nowOut{Request: n.Request, Queued: n.Queued})
		}
	}
	if want := []nowOut{{Request: third, Queued: 2}, {Request: second, Queued: 1}}; !reflect.DeepEqual(queued, want) {
		t.Errorf("Sam sees waiting their turn in the house %+v", queued)
	}
	let()
	s.answered(third)
	if listed := sam.listed("?state=queued"); len(listed) != 0 {
		t.Errorf("with every request answered, Sam's queued requests are %v", listed)
	}
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
	var run runOut
	d.s.as("lee").get("/api/runs/" + url.PathEscape(ledger)).is(http.StatusOK).into(&run)
	if run.Name != ledger || run.Team != "payments" || run.Request != d.payments || run.Kind != work.KindWork || run.Worker != "ledger" {
		t.Errorf("Lee is given the ledger's run as %+v", run)
	}
	d.s.as("al").get("/api/runs/" + url.PathEscape(ledger)).is(http.StatusNotFound)
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

func TestServerRunGivenWorkedOutWhenShown(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["notes/hours.md"] = "Support answers from nine to five.\n"
	// The bot names the notes kit, so each of its runs is given the
	// office's notes.
	files["bot.yaml"] = strings.Replace(files["bot.yaml"], "./.bulldog/tools/docs/docs.yaml]", "./.bulldog/tools/docs/docs.yaml, ./.bulldog/tools/notes/notes.yaml]", 1)
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case !strings.Contains(turn.System, "You answer what is asked in #help"):
			return testkit.Say("done")
		case strings.Contains(turn.Prompt(), "@bot thanks"):
			return testkit.Say("")
		case turn.Number == 1:
			return testkit.Use("read_doc", map[string]any{"name": "help/refunds.md"})
		}
		return testkit.Say("A charge taken twice is refunded within five working days.")
	})
	s := b.serve()
	al := s.as("al")
	readers := []any{"U0DANA00", "U0AL0000"}
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1800.1", "readers", readers, "text", "@bot how long does a refund take?"))
	n := s.at("slack", "#help/1800.1")
	s.answered(n)
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1800.1", "readers", readers, "text", "@bot thanks"))
	v := al.until(n, "has two runs ended", func(v viewOut) bool {
		return len(v.Runs) == 2 && v.Runs[0].Ended != nil && v.Runs[1].Ended != nil
	})
	given := func(run string) *givenOut {
		t.Helper()
		var out runOut
		al.get("/api/runs/" + url.PathEscape(run)).is(http.StatusOK).into(&out)
		if out.Given == nil {
			t.Fatalf("Al is given nothing of what %s was given", run)
		}
		return out.Given
	}
	texts := func(entries []entryOut) []string {
		var all []string
		for _, e := range entries {
			all = append(all, e.Text)
		}
		return all
	}

	// The first run was given the office's notes whole, read the refunds
	// doc, may ask the two teams the bot's ask_team takes, and was shown
	// nothing before what Dana asked, which was new for it, as hers.
	first := given(v.Runs[0].Name)
	if !slices.Equal(first.Notes, []noteGivenOut{{Name: "hours", Size: 35, Given: 35}}) {
		t.Errorf("the first run was given the notes %+v", first.Notes)
	}
	if !slices.Equal(first.Docs, []docReadOut{{Name: "help/refunds", Team: "office"}}) || first.RecordGone {
		t.Errorf("the first run read the docs %+v", first.Docs)
	}
	if !slices.Equal(first.MayAsk, []string{"payments", "platform"}) {
		t.Errorf("the first run may ask %v", first.MayAsk)
	}
	if len(first.Thread) != 0 || first.NotShown != 0 || !slices.Equal(texts(first.New), []string{"@bot how long does a refund take?"}) ||
		first.New[0].By.Kind != "person" || first.New[0].By.Name != "dana" {
		t.Errorf("the first run was shown %+v, and new for it was %+v", first.Thread, first.New)
	}

	// The second was shown the ask and the answer, and only her thanks was
	// new for it; it read no doc.
	second := given(v.Runs[1].Name)
	if !slices.Equal(texts(second.Thread), []string{"@bot how long does a refund take?", "A charge taken twice is refunded within five working days."}) ||
		!slices.Equal(texts(second.New), []string{"@bot thanks"}) || len(second.Docs) != 0 {
		t.Errorf("the second run was shown %v, new for it was %v, and it read %+v", texts(second.Thread), texts(second.New), second.Docs)
	}

	// Worked out when shown, a note written since is among the notes; and
	// Dana, who may not watch the run, is not given it.
	b.write(map[string]string{"notes/rota.md": "Al.\n"})
	b.until("the rota is among what the run is shown given", func() bool {
		b.clock.Move(0)
		return len(given(v.Runs[1].Name).Notes) == 2
	})
	s.as("dana").get("/api/runs/" + url.PathEscape(v.Runs[1].Name)).is(http.StatusNotFound)
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
	if slack := bridgeIn(t, s.meOf("al"), "slack"); slack.Status != "failing" || slack.Failing == nil || !strings.Contains(slack.Failing.Error, "Slack's signature does not match") {
		t.Errorf("Al is shown the failing bridge as %+v", slack)
	}
	if slack := bridgeIn(t, s.meOf("dana"), "slack"); slack.Status != "running" || slack.Failing != nil {
		t.Errorf("Dana is shown the failing bridge as %+v", slack)
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
	// Its machines, never checked, are given to Al by their files alone.
	if gitlab := bridgeIn(t, s.meOf("al"), "gitlab"); gitlab.Status != "stopped" ||
		!reflect.DeepEqual(gitlab.In, &machineOut{File: "bridges/gitlab/in.yaml"}) || !reflect.DeepEqual(gitlab.Out, &machineOut{File: "bridges/gitlab/out.yaml"}) {
		t.Errorf("Al sees the stopped GitLab bridge as %+v, in %+v, out %+v", gitlab, gitlab.In, gitlab.Out)
	}
	b.says(playRefund)
	s.delivered("slack", thing("said", "U0DANA00", "#help", "#help/1800.1", "text", "is the API up?"))
	s.answered(s.at("slack", "#help/1800.1"))
}

func TestServerDeciderGivenWithItsTeam(t *testing.T) {
	t.Parallel()
	files := house(t)
	// The garden's reviewer decides the house's orders after the
	// small-orders rule, and refers them to Sam.
	files["garden/reviewer.yaml"] = `agent: reviewer
input: { said: string }
model: usual
system: You review what is ordered. End with { "says": "yes", "refuse" or "refer", "note": why }.
prompt: ${input.said}
`
	files["office.yaml"] = strings.Replace(files["office.yaml"], "order_part: [./small-orders.js, clerk: advise, sam]", "order_part: [./small-orders.js, reviewer, sam]", 1)
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.System, "You review what is ordered") {
			return testkit.Say(`{ "says": "refer", "note": "For Sam." }`)
		}
		return playHouse(turn)
	})
	s := b.serve()
	want := []deciderOut{{Kind: "rule", Name: "small-orders.js"}, {Kind: "worker", Name: "reviewer", Team: "garden"}, {Kind: "person", Name: "sam"}}
	var whole teamWhole
	s.as("sam").get("/api/teams/office").is(http.StatusOK).into(&whole)
	list := whole.Decide[slices.IndexFunc(whole.Decide, func(l listOut) bool { return l.Key == "order_part" })]
	if !slices.Equal(list.Deciders, want) {
		t.Errorf("Sam is given the house's order_part as %+v", list.Deciders)
	}
	fixer := whole.Workers[slices.IndexFunc(whole.Workers, func(w workerOut) bool { return w.Name == "fixer" })]
	if tool := fixer.Tools[slices.IndexFunc(fixer.Tools, func(t toolOut) bool { return t.Name == "order_part" })]; !slices.Equal(tool.Deciders, want) {
		t.Errorf("Sam is given the fixer's order_part as decided by %+v", tool.Deciders)
	}

	n := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "the boiler pump is dead"}))
	s.waitsOn(n, "sam")
	var v struct{ Decisions []decisionOut }
	s.as("sam").get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	if len(v.Decisions) != 1 || len(v.Decisions[0].Referrals) != 2 || v.Decisions[0].Referrals[1].Name != "reviewer" || v.Decisions[0].Referrals[1].Team != "garden" ||
		v.Decisions[0].Referrals[0].Team != "" {
		t.Errorf("Sam is shown the order as %+v", v.Decisions)
	}
}

func TestServerDocsOpenToTheTeamWithinAndDeciders(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["docs/help/refunds.md"] = "# Refunds\nWithin 30 days.\n"
	files["payments/docs/ledger.md"] = "# The ledger\n"
	s := newBench(t, files).serve()
	docsOf := func(person, team string) []docOut {
		var seen teamOut
		s.as(person).get("/api/teams/" + team).is(http.StatusOK).into(&seen)
		return seen.Docs
	}
	// Lee, of payments, within the office, opens the office's docs, and
	// sees nothing else of how it is written.
	if docs := docsOf("lee", "office"); !slices.Equal(docs, []docOut{{Name: "help/refunds", Size: 26}}) {
		t.Errorf("Lee sees the office's docs as %+v", docs)
	}
	if office := s.as("lee").get("/api/teams/office").is(http.StatusOK).json(); office["workers"] != nil || office["notes"] != nil {
		t.Errorf("Lee sees the office as %v", office)
	}
	if doc := s.as("lee").get("/api/teams/office/docs/help/refunds").is(http.StatusOK).json(); doc["name"] != "help/refunds" || doc["text"] != files["docs/help/refunds.md"] {
		t.Errorf("Lee reads the office's doc as %v", doc)
	}
	// Dana, of support, opens the office's, and not payments'; Raj, in no
	// team, opens neither.
	if docs := docsOf("dana", "office"); len(docs) != 1 {
		t.Errorf("Dana sees the office's docs as %+v", docs)
	}
	for _, seen := range []struct{ person, team string }{{"dana", "payments"}, {"raj", "office"}, {"raj", "payments"}} {
		if docs := docsOf(seen.person, seen.team); docs != nil {
			t.Errorf("%s sees the docs of %s: %+v", seen.person, seen.team, docs)
		}
	}
	s.as("dana").get("/api/teams/payments/docs/ledger").is(http.StatusNotFound)
	s.as("raj").get("/api/teams/office/docs/help/refunds").is(http.StatusNotFound)
	// Al decides a change: he opens payments' docs too.
	if docs := docsOf("al", "payments"); !slices.Equal(docs, []docOut{{Name: "ledger", Size: 13}}) {
		t.Errorf("Al sees payments' docs as %+v", docs)
	}
	s.as("al").get("/api/teams/payments/docs/ledger").is(http.StatusOK)
	// Only a doc is opened by its name.
	for _, name := range []string{"help", "help/refunds.md", "office.yaml", "ledger"} {
		s.as("al").get("/api/teams/office/docs/" + name).is(http.StatusNotFound)
	}
}
