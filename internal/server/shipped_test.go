package server

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/shipped"
)

// shippedOffice gives the work office whose top team heeds with for-us,
// and whose GitLab bridge is its own folder, copied from Bulldog's at
// 0.1.0 and changed since.
func shippedOffice(t *testing.T) map[string]string {
	files := workOffice(t)
	files["office.yaml"] += "heed: ./.bulldog/rules/for-us/for-us.yaml\n"
	files["bridges/gitlab/README.md"] = shipped.Summary("bridges/gitlab") + "\n\n## Changed\n\n- 0.1.0: first shipped.\n"
	return files
}

// thingIn gives a shipped thing as /api/shipped gives it.
func thingIn(t *testing.T, out shippedOut, path string) thingOut {
	t.Helper()
	i := slices.IndexFunc(out.Things, func(th thingOut) bool { return th.Path == path })
	if i < 0 {
		t.Fatalf("no %s is given", path)
	}
	return out.Things[i]
}

func (s *serving) shippedOf(person string) shippedOut {
	s.b.t.Helper()
	var out shippedOut
	s.as(person).get("/api/shipped").is(http.StatusOK).into(&out)
	return out
}

func TestServerShippedSeenAsItsUsesAre(t *testing.T) {
	t.Parallel()
	s := newBench(t, shippedOffice(t)).serve(func(o *Options) { o.Version = "0.2.0" })

	dana := s.shippedOf("dana")
	if dana.Version != "0.2.0" || len(dana.Things) != len(shipped.Things()) {
		t.Errorf("Dana is given Bulldog %s and %d things", dana.Version, len(dana.Things))
	}
	forUs := thingIn(t, dana, "rules/for-us")
	settings := map[string]any{}
	for _, st := range forUs.Settings {
		settings[st.Name] = st.Value
	}
	needs := reachOut{Hosts: []string{"api.typesafe.ai"}, Secrets: []secretOut{{Name: "TYPESAFE_KEY", To: []string{"api.typesafe.ai"}}}}
	if forUs.Kind != "rule" || forUs.Status != "used" || len(forUs.Uses) != 0 || !reflect.DeepEqual(forUs.Needs, needs) ||
		settings["timeout"] != "10s" || settings["threshold"] != 0.5 || !strings.HasPrefix(forUs.Page, forUs.Summary) {
		t.Errorf("Dana is given for-us as %+v", forUs)
	}
	talk := thingIn(t, dana, "tools/talk")
	if talk.Status != "used" || len(talk.Uses) != 0 {
		t.Errorf("Dana is given the talk tools as %+v", talk)
	}
	gitlab := thingIn(t, dana, "bridges/gitlab")
	if gitlab.Status != "unused" || gitlab.Needs.Hosts[0] != "${input.settings.site}" {
		t.Errorf("Dana is given GitLab as %+v", gitlab)
	}
	for _, th := range dana.Things {
		if th.Changed != nil || th.Copies != nil || th.Use != nil || th.Copy != nil {
			t.Errorf("Dana is given what only the people who decide a change see of %s: %+v", th.Path, th)
		}
	}

	lee := s.shippedOf("lee")
	if uses := thingIn(t, lee, "tools/talk").Uses; !reflect.DeepEqual(uses, []useOut{{Team: "payments", Worker: "ledger"}}) {
		t.Errorf("Lee is given the talk tools' uses as %+v", uses)
	}
	if uses := thingIn(t, lee, "rules/for-us").Uses; len(uses) != 0 {
		t.Errorf("Lee is given for-us's uses as %+v", uses)
	}

	al := s.shippedOf("al")
	if uses := thingIn(t, al, "tools/talk").Uses; !reflect.DeepEqual(uses, []useOut{{Team: "office", Worker: "bot"}, {Team: "payments", Worker: "ledger"}}) {
		t.Errorf("Al is given the talk tools' uses as %+v", uses)
	}
	forUs = thingIn(t, al, "rules/for-us")
	if !reflect.DeepEqual(forUs.Uses, []useOut{{Team: "office", As: "heed"}}) || forUs.Use != nil || forUs.Copy == nil {
		t.Errorf("Al is given for-us as %+v", forUs)
	}
	gitlab = thingIn(t, al, "bridges/gitlab")
	if gitlab.Status != "copied" || len(gitlab.Copies) != 1 || gitlab.Copies[0].Folder != "bridges/gitlab" || gitlab.Copies[0].Version != "0.1.0" ||
		!slices.Contains(gitlab.Copies[0].Differs, "gitlab.js") {
		t.Errorf("Al is given GitLab as %+v", gitlab)
	}
	// The office's Slack is its own, so Use names another; nothing runs
	// Telegram, so Use and Copy give it its own name.
	slack := thingIn(t, al, "bridges/slack")
	if slack.Status != "unused" || slack.Use == nil || !slices.Equal(slack.Use.Set[0].Field, []string{"bridges", "slack-2"}) {
		t.Errorf("Al is given Slack as %+v", slack)
	}
	telegram := thingIn(t, al, "bridges/telegram")
	if telegram.Copy == nil || telegram.Copy.Files["bridges/telegram/telegram.js"] == nil || telegram.Copy.Files["bridges/telegram/settings.yaml"] != nil {
		t.Fatalf("Al is given Telegram's copy as %+v", telegram.Copy)
	}
	if talk := thingIn(t, al, "tools/talk"); talk.Copy != nil || talk.Use != nil {
		t.Errorf("Al is given a draft of the talk tools: %+v", talk)
	}

	// Each draft is one Al can check and put: Telegram used, Telegram
	// copied, its entry with no from, and for-us copied, the heed naming
	// the copy.
	var used, copied, rule checkOut
	s.as("al").post("/api/office/check", telegram.Use).is(http.StatusOK).into(&used)
	if !used.Fit || len(used.Files) != 1 || !strings.Contains(used.Files[0].Text, "from: ./.bulldog/bridges/telegram") {
		t.Errorf("using Telegram gives %+v", used)
	}
	s.as("al").post("/api/office/check", telegram.Copy).is(http.StatusOK).into(&copied)
	office := slices.IndexFunc(copied.Files, func(f changes.File) bool { return f.Path == "office.yaml" })
	if !copied.Fit || office < 0 || !strings.Contains(copied.Files[office].Text, "telegram:") || strings.Contains(copied.Files[office].Text, "from: ./.bulldog/bridges/telegram") {
		t.Errorf("copying Telegram gives %+v", copied)
	}
	s.as("al").post("/api/office/check", forUs.Copy).is(http.StatusOK).into(&rule)
	office = slices.IndexFunc(rule.Files, func(f changes.File) bool { return f.Path == "office.yaml" })
	if !rule.Fit || office < 0 || !strings.Contains(rule.Files[office].Text, "heed: ./for-us.yaml") ||
		!slices.ContainsFunc(rule.Files, func(f changes.File) bool { return f.Path == "for-us.js" }) {
		t.Errorf("copying for-us gives %+v", rule)
	}
}

func TestServerShippedChangedLinesAfterUpgrade(t *testing.T) {
	t.Parallel()
	b := newBench(t, shippedOffice(t))
	// The first start of an office says nothing of what changed: nothing
	// ran before it.
	first := b.serve(func(o *Options) { o.Version = "0.1.0" })
	if first.logged("changed in") || thingIn(t, first.shippedOf("al"), "rules/for-us").Changed != nil {
		t.Errorf("the first start says what changed:\n%s", strings.Join(first.lines(), "\n"))
	}
	first.stop()

	// At the first start of 0.2.0, what changed in 0.2.0 is said of each
	// thing the office uses, and of nothing else.
	upgraded := b.serve(func(o *Options) { o.Version = "0.2.0" })
	for _, said := range []string{"rules/for-us changed in 0.2.0, which the office uses: first shipped.", "tools/talk changed in 0.2.0"} {
		if !upgraded.logged(said) {
			t.Errorf("the start after the upgrade does not say %q", said)
		}
	}
	if upgraded.logged("bridges/jira changed") || upgraded.logged("tools/docs changed") {
		t.Error("the start after the upgrade says what changed in what the office does not use")
	}
	changed := func(s *serving, person string) []shipped.Change {
		return thingIn(t, s.shippedOf(person), "bridges/jira").Changed
	}
	if c := changed(upgraded, "al"); len(c) != 1 || c[0].Version != "0.2.0" {
		t.Errorf("Al is given Jira's changes as %+v", c)
	}
	if c := changed(upgraded, "dana"); c != nil {
		t.Errorf("Dana is given Jira's changes as %+v", c)
	}
	upgraded.stop()

	// Started again on 0.2.0, nothing is said, and what changed is still
	// shown while the version runs.
	again := b.serve(func(o *Options) { o.Version = "0.2.0" })
	if again.logged("changed in") {
		t.Error("a second start of the same version says what changed")
	}
	if c := changed(again, "al"); len(c) != 1 {
		t.Errorf("Al is given Jira's changes as %+v after a restart", c)
	}
}
