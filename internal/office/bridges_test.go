package office_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// broken reads an office that reads, with a problem stopping one bridge
// alone, and gives its problems.
func broken(t *testing.T, files map[string]string, bridge string, others ...string) office.Problems {
	t.Helper()
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the office is refused: %v", problems)
	}
	of := problems.OfBridge(bridge)
	if len(of) == 0 || o.Bridges[bridge].Runs() {
		t.Fatalf("%s carries, with the problems %v", bridge, problems)
	}
	for _, p := range of {
		if p.Team != "office" || p.Refuses {
			t.Errorf("the problem %v is not the office's, stopping %s alone", p, bridge)
		}
	}
	for _, other := range others {
		if !o.Bridges[other].Runs() {
			t.Errorf("%s does not carry", other)
		}
	}
	for name, w := range o.Workers {
		if !w.Runs() {
			t.Errorf("%s does not run", name)
		}
	}
	return of
}

func TestTheBridgesInTheFiles(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if !slices.Equal(o.BridgeOrder, []string{"slack", "gitlab"}) || !o.Bridges["slack"].Hears || !o.Bridges["gitlab"].Listens {
		t.Errorf("the bridges are %v", o.BridgeOrder)
	}
	if !reflect.DeepEqual(o.Bridges["slack"].Settings, map[string]any{"from": "./.bulldog/bridges/slack", "places": map[string]any{"#help": map[string]any{"guests": true}}}) {
		t.Errorf("slack's settings are %#v", o.Bridges["slack"].Settings)
	}
	files := own(t, work(t), "gitlab")
	if o := reads(t, files); o.Bridges["gitlab"].Dir() != "bridges/gitlab" || !o.Bridges["gitlab"].Runs() {
		t.Errorf("the office's own gitlab runs %s", o.Bridges["gitlab"].Dir())
	}
	says(t, broken(t, replace(t, files, "office.yaml", "  gitlab:\n    site: gitlab.com\n    places:\n      platform/api: { team: platform }\n", ""), "gitlab", "slack"), "bridges/gitlab", "does not name")
	says(t, broken(t, with(files, map[string]string{"bridges/gitlab/in.yaml": "", "bridges/gitlab/out.yaml": "", "bridges/gitlab/gitlab.js": "", "bridges/gitlab/README.md": ""}), "gitlab", "slack"), "no folder bridges/gitlab")
	says(t, broken(t, with(files, map[string]string{"bridges/gitlab/out.yaml": ""}), "gitlab", "slack"), "no out.yaml")
	says(t, broken(t, replace(t, files, "bridges/gitlab/in.yaml", "input: [settings, cursor]", "input: [settings, cursor, delivery]"), "gitlab", "slack"), "both or neither")
	says(t, broken(t, replace(t, files, "bridges/gitlab/in.yaml", "input: [settings, cursor]", "input: [settings]"), "gitlab", "slack"), "both or neither")
	says(t, broken(t, with(files, map[string]string{"bridges/jira/in.yaml": "flow: jira-in\n"}), "jira", "slack", "gitlab"), "bridges/jira", "does not name")
	// The house without its bridge reads with no problem.
	reads(t, stringsOf(examples.NewHouse()))
}

func TestABridgeRunsTheShippedFolderItsFromNames(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	for _, name := range []string{"slack", "gitlab"} {
		if b := o.Bridges[name]; b.From != ".bulldog/bridges/"+name || b.Dir() != b.From || !b.Runs() {
			t.Errorf("%s runs %s", name, b.Dir())
		}
	}
	// The key the bridge is under is its name, whichever folder it runs.
	files := replace(t, work(t), "office.yaml", "  gitlab:\n    from: ./.bulldog/bridges/gitlab\n", "  lab:\n    from: ./.bulldog/bridges/gitlab\n")
	files = replace(t, replace(t, files, "office.yaml", `gitlab: "1041"`, `lab: "1041"`), "office.yaml", `gitlab: "1187"`, `lab: "1187"`)
	o = reads(t, files)
	if b := o.Bridges["lab"]; b == nil || b.Dir() != ".bulldog/bridges/gitlab" || !b.Runs() {
		t.Errorf("the bridges are %v", o.BridgeOrder)
	}
	// from names only a folder Bulldog ships; one with a folder of the
	// office's own beside it runs neither.
	for _, from := range []string{"./bridges/gitlab", "./.bulldog/bridges/matrix", "./.bulldog/tools/talk", "../.bulldog/bridges/gitlab", "[gitlab]"} {
		says(t, broken(t, replace(t, work(t), "office.yaml", "from: ./.bulldog/bridges/gitlab", "from: "+from), "gitlab", "slack"), "office.yaml", "from", "./.bulldog/bridges/jira")
	}
	beside := with(work(t), map[string]string{"bridges/gitlab/in.yaml": "flow: gitlab-in\n"})
	says(t, broken(t, beside, "gitlab", "slack"), "bridges/gitlab", "bridges/gitlab is not run: gitlab has a from")
	problems := broken(t, replace(t, beside, "office.yaml", "from: ./.bulldog/bridges/gitlab", "from: ./bridges/gitlab"), "gitlab", "slack")
	says(t, problems, "office.yaml", "from", "./.bulldog/bridges/jira")
	if p := says(t, problems, "bridges/gitlab", "bridges/gitlab is not run: gitlab has a from"); strings.Contains(p.Text, "runs ./bridges/gitlab") {
		t.Errorf("the folder's problem is %q", p.Text)
	}
}

func stringsOf(files map[string][]byte) map[string]string {
	out := map[string]string{}
	for name, contents := range files {
		out[name] = string(contents)
	}
	return out
}

func TestABridgesSettingsAreItsOwnButForThePlaces(t *testing.T) {
	t.Parallel()
	o := reads(t, replace(t, work(t), "office.yaml", "  slack:\n    from: ./.bulldog/bridges/slack\n    places:\n      \"#help\": { guests: true }\n", "  slack: { from: ./.bulldog/bridges/slack, places: { \"#help\": { guests: true } }, anything: 3 }\n"))
	if !reflect.DeepEqual(o.Bridges["slack"].Settings, map[string]any{"from": "./.bulldog/bridges/slack", "places": map[string]any{"#help": map[string]any{"guests": true}}, "anything": 3}) {
		t.Errorf("slack's settings are %#v", o.Bridges["slack"].Settings)
	}
	if p, ok := o.Place("slack", "#help"); !ok || p.Team != "office" || !p.Guests {
		t.Errorf("#help is %#v", p)
	}
	if p, ok := o.Place("gitlab", "platform/api"); !ok || p.Team != "platform" || p.Guests {
		t.Errorf("platform/api is %#v", p)
	}
	slack := "  slack:\n    from: ./.bulldog/bridges/slack\n    places:\n      \"#help\": { guests: true }\n"
	says(t, broken(t, replace(t, work(t), "office.yaml", slack, "  slack: \"#help\"\n"), "slack", "gitlab"), "slack")
	says(t, broken(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { team: billing }`), "slack", "gitlab"), "slack", "billing")
	says(t, broken(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, `"#help": { guests: maybe }`), "slack", "gitlab"), "slack", "guests")
	says(t, broken(t, replace(t, work(t), "office.yaml", slack, "  slack: { from: ./.bulldog/bridges/slack, places: [\"#help\"] }\n"), "slack", "gitlab"), "slack", "places")
	// Settings with no places listen in none.
	o = reads(t, replace(t, house(t), "office.yaml", "    places:\n      direct: {}                   # as Bulldog ships it: Sam and Kit may ask the house in a direct message\n", ""))
	if len(o.Bridges["telegram"].Places) != 0 {
		t.Errorf("telegram listens in %v", o.Bridges["telegram"].Places)
	}
	if p, ok := reads(t, house(t)).Place("telegram", office.Direct); !ok || p.Team != "office" || p.Guests {
		t.Errorf("telegram's direct place is %#v", p)
	}
}

func TestABridgeCorbieRefusesCarriesNothing(t *testing.T) {
	t.Parallel()
	p := says(t, broken(t, replace(t, own(t, work(t), "gitlab"), "bridges/gitlab/in.yaml", "script: ./gitlab.js", "script: ./missing.js"), "gitlab", "slack"), "bridges/gitlab/in.yaml", "missing.js")
	if p.Bridge != "gitlab" {
		t.Errorf("the problem is %#v", p)
	}
}
