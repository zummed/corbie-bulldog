package server

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestServerTeamHeedRulesModelsAndWorkersSeenByItsPeople(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["office.yaml"] += "heed: { rule: ./.bulldog/rules/for-us/for-us.yaml, timeout: 5s }\n"
	files["payments/team.yaml"] += "models:\n  fast: { name: jev-latest, secret: FAST_KEY }\n"
	s := newBench(t, files).serve()

	var office teamWhole
	s.as("al").get("/api/teams/office").is(http.StatusOK).into(&office)
	heed := office.Heed
	if heed == nil || heed.Rule != ".bulldog/rules/for-us/for-us.yaml" || heed.Shipped != "rules/for-us" || heed.Stopped {
		t.Fatalf("Al sees the office's heed as %+v", heed)
	}
	settings := map[string]settingOut{}
	for _, set := range heed.Settings {
		settings[set.Name] = set
	}
	if set := settings["timeout"]; set.Value != "5s" || set.Default || set.About == "" {
		t.Errorf("Al sees the heed's timeout as %+v", set)
	}
	if set := settings["threshold"]; set.Value != 0.5 || !set.Default {
		t.Errorf("Al sees the heed's threshold as %+v", set)
	}
	if set := settings["model"]; set.Value != "jev-latest" || !set.Default || len(heed.Settings) != 4 {
		t.Errorf("Al sees the heed's settings as %+v", heed.Settings)
	}
	forUs := ruleOut{Path: ".bulldog/rules/for-us/for-us.yaml", Shipped: "rules/for-us", As: []string{"heed"}, Runs: true,
		Reach: reachOut{Hosts: []string{"api.typesafe.ai"}, Secrets: []secretOut{{Name: "TYPESAFE_KEY", To: []string{"api.typesafe.ai"}}}}}
	if !reflect.DeepEqual(office.Rules, []ruleOut{forUs}) {
		t.Errorf("Al sees the office's rules as %+v", office.Rules)
	}
	// The bot's own ask_team takes platform and payments alone.
	if bot := office.Workers[slices.IndexFunc(office.Workers, func(w workerOut) bool { return w.Name == "bot" })]; !slices.Equal(bot.MayAsk, []string{"payments", "platform"}) {
		t.Errorf("Al sees the bot may ask %v", bot.MayAsk)
	}
	// Dana sees that the office names a heed, and nothing of it.
	dana := s.as("dana").get("/api/teams/office").is(http.StatusOK)
	for _, hidden := range []string{"for-us", "timeout", "threshold", "TYPESAFE_KEY"} {
		if strings.Contains(string(dana.body), hidden) {
			t.Errorf("Dana's /api/teams/office holds %q", hidden)
		}
	}

	var payments teamWhole
	s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&payments)
	if len(payments.Rules) != 1 || payments.Rules[0].Path != "payments/no-account-numbers.yaml" || !slices.Equal(payments.Rules[0].As, []string{"disclosure"}) {
		t.Errorf("Lee sees payments' rules as %+v", payments.Rules)
	}
	if i := slices.IndexFunc(payments.Models, func(m modelOut) bool { return m.Name == "fast" }); i < 0 || payments.Models[i].Secret != "FAST_KEY" {
		t.Errorf("Lee sees payments' models as %+v", payments.Models)
	}
	ledger := payments.Workers[slices.IndexFunc(payments.Workers, func(w workerOut) bool { return w.Name == "ledger" })]
	def := definitionOut{System: ledger.Definition.System, Prompt: "${input.context}", Inputs: []string{"said", "context"},
		ToolsFiles: []string{"../.bulldog/tools/talk/talk.yaml", "../.bulldog/tools/notes/notes.yaml"}, Asks: true}
	if ledger.Secret != testkit.Key || !reflect.DeepEqual(ledger.Definition, def) || !strings.HasPrefix(def.System, "You keep the ledger.") ||
		!slices.Contains(ledger.Uses, "payments/tools/ledger.js") || len(ledger.HeldHere) != 0 {
		t.Errorf("Lee sees the ledger as %+v", ledger)
	}
	refund := ledger.Tools[slices.IndexFunc(ledger.Tools, func(t toolOut) bool { return t.Name == "refund" })]
	reach := &toolReach{reachOut: reachOut{Hosts: []string{"ledger.example.com"}, Secrets: []secretOut{{Name: "LEDGER_KEY", To: []string{"ledger.example.com"}}}},
		Deny: []string{}, AskAbout: []string{}}
	if !reflect.DeepEqual(refund.Reach, reach) {
		t.Errorf("Lee sees the refund reach %+v", refund.Reach)
	}
	if ask := ledger.Tools[slices.IndexFunc(ledger.Tools, func(t toolOut) bool { return t.Name == "ask_team" })]; ask.Reach != nil {
		t.Errorf("Lee sees ask_team, one of Bulldog's, reach %+v", ask.Reach)
	}
}

func TestServerHeldHereAreTheColleaguesThatUseTheAsk(t *testing.T) {
	t.Parallel()
	var office teamWhole
	newBench(t, house(t)).serve().as("sam").get("/api/teams/office").is(http.StatusOK).into(&office)
	held := map[string][]string{}
	for _, w := range office.Workers {
		held[w.Name] = w.HeldHere
	}
	// The steward calls the fixer, whose cut and order wait for a yes, and
	// the builder, whose office_put does, and they go to the steward's ask,
	// since neither names one.
	if !slices.Equal(held["steward"], []string{"builder", "fixer"}) || len(held["fixer"]) != 0 {
		t.Errorf("Sam sees the held jobs of colleagues as %v", held)
	}
}

func TestServerRoutinesWithTheirNextTimeAndStatus(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["payments/team.yaml"] += "routines:\n  - { at: \"mon 09:00\", to: ledger, say: \"Refund ch_2.\" }\n"
	b := newBench(t, files)
	b.says(playRefund)
	s := b.serve()
	routine := func() routineOut {
		var payments teamWhole
		s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&payments)
		if len(payments.Routines) != 1 {
			t.Fatalf("Lee sees payments' routines as %+v", payments.Routines)
		}
		return payments.Routines[0]
	}
	london, _ := time.LoadLocation("Europe/London")
	now := b.clock.Now()
	r := routine()
	if next := r.Next.In(london); r.Status != "due" || r.Zone != "Europe/London" || !next.After(now) || next.Sub(now) > 7*24*time.Hour ||
		next.Weekday() != time.Monday || next.Hour() != 9 || next.Minute() != 0 {
		t.Errorf("Lee sees the routine as %+v", r)
	}
	// Run now, the ledger's refund waits for a yes, and the routine's next
	// coming round is let go by while its request is open.
	n := numbered(s.as("lee").post("/api/teams/payments/routines/now", map[string]string{"at": "mon 09:00", "to": "ledger", "say": "Refund ch_2."}))
	s.waitsOn(n, "lee")
	if r := routine(); r.Status != "skipped" || r.Open != n {
		t.Errorf("with its request open, Lee sees the routine as %+v", r)
	}
	// A routine whose worker cannot run cannot run.
	house := house(t)
	house["bins.yaml"] = "flow: bins\nthis is not a definition: [\n"
	var office teamWhole
	newBench(t, house).serve().as("sam").get("/api/teams/office").is(http.StatusOK).into(&office)
	i := slices.IndexFunc(office.Routines, func(r routineOut) bool { return r.To == "bins" })
	if i < 0 || office.Routines[i].Status != "cannot run" {
		t.Errorf("with bins.yaml broken, Sam sees the routines as %+v", office.Routines)
	}
}

func TestServerOneLookingSeesWhereTheyAreReachedAndWhatTheyDecide(t *testing.T) {
	t.Parallel()
	s := newBench(t, workOffice(t)).serve()
	al := s.meOf("al").Me
	ids := []identityOut{{Bridge: "slack", ID: "U0AL0000"}, {Bridge: "gitlab", ID: "1041"}}
	if !reflect.DeepEqual(al.Identities, ids) || !reflect.DeepEqual(al.Lists, []namedOut{{Team: "office", Key: "everything", At: 1}}) ||
		!slices.Contains(al.Walks, "office") || !slices.Contains(al.Walks, "platform") || slices.Contains(al.Walks, "payments") {
		t.Errorf("Al is told he is %+v", al)
	}
	lee := s.meOf("lee").Me
	if !reflect.DeepEqual(lee.Lists, []namedOut{{Team: "payments", Key: "everything", At: 1}}) || !slices.Equal(lee.Walks, []string{"payments"}) {
		t.Errorf("Lee is told he is %+v", lee)
	}
	// The house's Telegram bridge has a direct place, where a link is
	// sent.
	sam := newBench(t, house(t)).serve().meOf("sam").Me
	if !reflect.DeepEqual(sam.Identities, []identityOut{{Bridge: "telegram", ID: "4471", Direct: true}}) {
		t.Errorf("Sam is told he is reached as %+v", sam.Identities)
	}
}

func TestServerIdentitiesGivenToTheirPersonAlone(t *testing.T) {
	t.Parallel()
	s := newBench(t, workOffice(t)).serve()
	everyone := []string{"U0DANA00", "U0AL0000", "1041", "U0RAJ000", "1187", "U0LEE000", "U0PRI000"}
	for person, theirs := range map[string][]string{"dana": {"U0DANA00"}, "al": {"U0AL0000", "1041"}, "lee": {"U0LEE000"}, "raj": {"U0RAJ000", "1187"}} {
		body := string(s.as(person).get("/api/me").is(http.StatusOK).body)
		for _, id := range everyone {
			if strings.Contains(body, id) != slices.Contains(theirs, id) {
				t.Errorf("%s's /api/me holds %s: %v", person, id, !slices.Contains(theirs, id))
			}
		}
		// Al decides a change and sees every team: no other answer gives
		// anyone's identity, his own among them.
		for _, path := range []string{"/api/teams/office", "/api/teams/payments", "/api/teams/support", "/api/teams/platform", "/api/defined", "/api/shipped", "/api/requests"} {
			body := string(s.as(person).get(path).body)
			for _, id := range everyone {
				if strings.Contains(body, id) {
					t.Errorf("%s's %s holds %s", person, path, id)
				}
			}
		}
	}
	// The office's files hold everyone's, and only those who decide a
	// change read them.
	if file := string(s.as("al").get("/api/office/file?path=office.yaml").is(http.StatusOK).body); !strings.Contains(file, "U0LEE000") {
		t.Errorf("Al's office.yaml holds no identity of Lee's: %s", file)
	}
	s.as("lee").get("/api/office/file?path=office.yaml").is(http.StatusNotFound)
}

func TestServerHeedSettingWithNoDefaultHasNoValue(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["office.yaml"] += "heed: { rule: ./rules/heed.yaml, level: 3 }\n"
	files["rules/heed.yaml"] = "flow: heed\ninput:\n  type: object\n  properties:\n    settings:\n      type: object\n      properties:\n" +
		"        word: { type: string }\n        level: { type: number, default: 2 }\n        tone: { type: string, default: plain, description: How it is said. }\n" +
		"stages:\n  weigh:\n    job:\n      script: ./heed.js\n      input: { settings: \"${input.settings}\" }\n" +
		"output: ${stages.weigh.output}\n"
	files["rules/heed.js"] = "export default function () { return { says: 'act' }; }\n"
	s := newBench(t, files).serve()

	var office teamWhole
	s.as("al").get("/api/teams/office").is(http.StatusOK).into(&office)
	if office.Heed == nil {
		t.Fatal("Al sees no heed of the office")
	}
	want := []settingOut{{Name: "word"}, {Name: "level", Value: 3.0}, {Name: "tone", Value: "plain", Default: true, About: "How it is said."}}
	if !reflect.DeepEqual(office.Heed.Settings, want) {
		t.Errorf("Al sees the heed's settings as %+v", office.Heed.Settings)
	}
}

func TestServerDefinedByTheTeamsOneSees(t *testing.T) {
	t.Parallel()
	files := workOffice(t)
	files["docs/help/refunds.md"] = "# Refunds\n"
	s := newBench(t, files).serve()
	defined := func(person string) definedOut {
		var out definedOut
		s.as(person).get("/api/defined").is(http.StatusOK).into(&out)
		return out
	}
	tool := func(d definedOut, name string) definedRow {
		i := slices.IndexFunc(d.Tools, func(r definedRow) bool { return r.Name == name })
		if i < 0 {
			return definedRow{}
		}
		return d.Tools[i]
	}
	lee := defined("lee")
	if talk := tool(lee, ".bulldog/tools/talk/talk.yaml"); talk.Kind != "tools" || talk.Shipped != "tools/talk" || !slices.Equal(talk.Teams, []string{"payments"}) {
		t.Errorf("Lee sees the talk tools used by %+v", talk)
	}
	if ledger := tool(lee, "payments/tools/ledger.js"); ledger.Kind != "script" || !slices.Equal(ledger.Teams, []string{"payments"}) {
		t.Errorf("Lee sees the ledger's script as %+v", ledger)
	}
	if tool(lee, "platform/tools/gitlab.js").Name != "" || len(lee.Models) != 1 || !slices.Equal(lee.Models[0].Teams, []string{"payments"}) {
		t.Errorf("Lee sees the tools %+v and the models %+v", lee.Tools, lee.Models)
	}
	if !reflect.DeepEqual(lee.Docs, []definedRow{{Name: "help/refunds", Team: "office", Size: 10, Teams: []string{"office"}}}) {
		t.Errorf("Lee sees the docs %+v", lee.Docs)
	}
	al := defined("al")
	if talk := tool(al, ".bulldog/tools/talk/talk.yaml"); !slices.Equal(talk.Teams, []string{"office", "payments"}) {
		t.Errorf("Al, who decides a change, sees the talk tools used by %+v", talk)
	}
	if gitlab := tool(al, "platform/tools/gitlab.js"); !slices.Equal(gitlab.Teams, []string{"platform"}) {
		t.Errorf("Al sees platform's GitLab tool as %+v", gitlab)
	}
	if raj := defined("raj"); len(raj.Tools) != 0 || len(raj.Models) != 0 || len(raj.Docs) != 0 {
		t.Errorf("Raj, in no team, sees %+v", raj)
	}
}

func TestServerTeamWritesDecidedAsItsLists(t *testing.T) {
	t.Parallel()
	s := newBench(t, workOffice(t)).serve()
	var office teamWhole
	s.as("al").get("/api/teams/office").is(http.StatusOK).into(&office)
	// The bot holds write_doc, and the office writes no list of either, so
	// its everything decides both.
	al := []deciderOut{{Kind: "person", Name: "al"}}
	want := []listOut{{Key: "office_put", Deciders: al}, {Key: "write_doc", Deciders: al}}
	if !slices.Equal(office.DocWriters, []string{"bot"}) || !reflect.DeepEqual(office.Writes, want) {
		t.Errorf("Al sees the office's docs written by %v, and its writes decided as %+v", office.DocWriters, office.Writes)
	}
	var payments teamWhole
	s.as("lee").get("/api/teams/payments").is(http.StatusOK).into(&payments)
	if len(payments.DocWriters) != 0 || len(payments.Writes) != 1 || payments.Writes[0].Key != "office_put" {
		t.Errorf("Lee sees payments' docs written by %v, and its writes decided as %+v", payments.DocWriters, payments.Writes)
	}
}
