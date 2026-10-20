package bridges_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestBridgesShippedFoldersAreChecked(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{}
	for name, text := range examples.NewHouse() {
		files[name] = text
	}
	listing := "bridges:\n"
	for _, b := range shipped.Bridges() {
		_, settings, _ := shipped.Bridge(b)
		var entry map[string]any
		if err := yaml.Unmarshal([]byte(settings), &entry); err != nil {
			t.Fatalf("%s's entry cannot be read: %v", b, err)
		}
		flat, _ := json.Marshal(entry)
		listing += "  " + b + ": " + string(flat) + "\n"
	}
	files["office.yaml"] = []byte(strings.Replace(string(files["office.yaml"]), "models:", listing+"models:", 1))
	o, problems := office.Parse(t.TempDir(), files)
	if problems.Refuse() || len(o.Problems) > 0 {
		t.Fatalf("the shipped bridges read with\n%v", problems)
	}
	for name, want := range map[string][3]bool{"telegram": {true, false, true}, "slack": {true, false, true}, "gitlab": {true, false, false}, "jira": {true, false, false}} {
		b := o.Bridges[name]
		direct := slices.Contains(b.Out.Definition.Input, "direct")
		if !b.Runs() || b.Dir() != ".bulldog/bridges/"+name || b.Listens != want[0] || b.Hears != want[1] || direct != want[2] {
			t.Errorf("%s listens %v, hears %v, says things directly %v", name, b.Listens, b.Hears, direct)
		}
	}
	if p, ok := o.Place("telegram", office.Direct); !ok || p.Team != office.Top || p.Guests {
		t.Errorf("the shipped Telegram bridge's direct place is %#v", p)
	}
}

func TestBridgesShippedSiteFromSettings(t *testing.T) {
	t.Parallel()
	// No machine or script of GitLab's or Jira's names a host: each reaches
	// the site its settings give, and sends its token there alone.
	for name, token := range map[string]string{"gitlab": "GITLAB_TOKEN", "jira": "JIRA_TOKEN"} {
		files := folder(t, name)
		for _, which := range []string{"in.yaml", "out.yaml"} {
			text := files[which]
			if strings.Count(text, `"net:${input.settings.site}"`) != 1 || strings.Count(text, `"secret:`+token+`@net:${input.settings.site}"`) != 1 || strings.Count(text, "net:") != 2 {
				t.Errorf("%s's %s allows\n%s", name, which, text)
			}
		}
		if script := files[name+".js"]; !strings.Contains(script, `"https://" + input.settings.site`) || strings.Count(script, "://") != 1 {
			t.Errorf("%s's script names a host of its own", name)
		}
	}
	// Given the stand-in's address as its site, each answers there.
	gl, s := newGitLabBridge(t)
	sent(t, s.carry(map[string]any{"to": "platform/api#7", "text": "On it."}))
	if len(gl.notes) != 1 {
		t.Errorf("GitLab holds the comments %v", gl.notes)
	}
	j, js := newJiraBridge(t)
	sent(t, js.carry(map[string]any{"settings": jiraSettings, "to": "OPS-12", "text": "On it."}))
	if len(j.comments) != 1 {
		t.Errorf("Jira holds the comments %v", j.comments)
	}
	// Given no site, nothing is sent, and the failure names it.
	s.site = ""
	if r := s.carry(map[string]any{"to": "platform/api#7", "text": "On it."}); r.Status != "failed" || !strings.Contains(r.Error, "input.settings.site") || len(gl.notes) != 1 {
		t.Errorf("GitLab with no site ends %s: %s", r.Status, r.Error)
	}
}

func TestBridgesShippedReadsAnEarlierCursor(t *testing.T) {
	t.Parallel()
	// Each listening bridge goes on from a cursor as 0.1.0 gave it,
	// hearing only what came after it.
	tg, s := newTelegramBridge(t)
	tg.says(4471, "sam", "before")
	after := tg.says(4471, "sam", "after")
	if things, _ := s.heard(listen(float64(after))); len(things) != 1 || things[0]["text"] != "after" {
		t.Errorf("Telegram, from the 0.1.0 cursor %d, hears %v", after, things)
	}
	gl, g := newGitLabBridge(t)
	gl.happens(map[string]any{"action_name": "closed", "target_type": "Issue", "target_iid": 7, "author_id": 1187})
	gl.happens(map[string]any{"action_name": "reopened", "target_type": "Issue", "target_iid": 7, "author_id": 1187})
	if things, _ := g.heard(map[string]any{"settings": gitlabPlaces, "cursor": map[string]any{"platform/api": 501}}); len(things) != 1 || things[0]["status"] != "reopened" {
		t.Errorf("GitLab, from the 0.1.0 cursor of event 501, hears %v", things)
	}
	j, js := newJiraBridge(t)
	opened := time.Now().Truncate(time.Millisecond)
	j.opened(opened, "To Do", "new", []any{map[string]any{"id": "2001", "body": "after", "created": stamp(opened.Add(time.Second)), "author": map[string]any{"accountId": "5b10ac8d82e05b22cc7d4ef5"}}}, nil)
	cursor := map[string]any{"since": opened.UTC().Format("2006-01-02T15:04:05.000Z"), "seen": []any{"issue:10012"}}
	if things, _ := js.heard(map[string]any{"settings": jiraSettings, "cursor": cursor}); len(things) != 1 || things[0]["text"] != "after" {
		t.Errorf("Jira, from the 0.1.0 cursor %v, hears %v", cursor, things)
	}
}
