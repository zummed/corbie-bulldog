package bridges_test

import (
	"bytes"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestBridgesMachinesHandedAsWritten(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["bridges/telegram/out.yaml"] = "agent: standin-out\ninput: { settings: any, to: string, direct: boolean, text: blob }\nmodel: usual\nprompt: ${input.text}\n"
	d := open(t, files)
	// parsed gives a machine of the bridge's folder as Corbie reads it from
	// text, with the folder's files.
	folder := map[string]string{}
	for name, text := range files {
		if rest, ok := strings.CutPrefix(name, "bridges/telegram/"); ok {
			folder[rest] = text
		}
	}
	parsed := func(src string) *machine.Draft {
		draft, err := machine.Parse([]byte(src), machine.Given(folder))
		must(t, err)
		return draft
	}
	in := d.listening("telegram")
	want := parsed(files["bridges/telegram/in.yaml"])
	if in.Machine != d.o.Bridges["telegram"].In || !bytes.Equal(in.Machine.Definition.Body, want.Definition.Body) || !reflect.DeepEqual(in.Machine.Files, want.Files) {
		t.Errorf("the in run is of %s, with %v", in.Machine.Definition.Body, slices.Collect(maps.Keys(in.Machine.Files)))
	}
	for name, contents := range in.Machine.Files {
		if !strings.HasSuffix(name, "standin.js") || string(contents) != files["bridges/telegram/standin.js"] {
			t.Errorf("the in run is handed %s as %q", name, contents)
		}
	}
	if _, err := d.SignIn("telegram", "sam", "a link"); err != nil {
		t.Fatal(err)
	}
	out := d.carry()
	if len(out) != 1 || out[0].Machine != d.o.Bridges["telegram"].Out {
		t.Fatalf("the out runs are %v", out)
	}
	// The out machine is as written, but for the office's usual model in
	// place of the word.
	want = parsed(strings.Replace(files["bridges/telegram/out.yaml"], "model: usual", "model: { name: sonnet, service: claude-code }", 1))
	if body := out[0].Machine.Definition.Body; !bytes.Equal(body, want.Definition.Body) {
		t.Errorf("the out machine is sent as\n%s\nand not as\n%s", body, want.Definition.Body)
	}
}

func TestBridgesSettingsHandedWhole(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", "  slack:\n    places:\n      \"#help\": { guests: true }\n", "  slack: { places: { \"#help\": { guests: true } }, anything: 3 }\n")
	d := open(t, files)
	s, err := d.Hear("slack", bridges.Delivery{Method: "POST"})
	must(t, err)
	want := map[string]any{"places": map[string]any{"#help": map[string]any{"guests": true}}, "anything": 3}
	if !reflect.DeepEqual(s.Input["settings"], want) || s.Machine != d.o.Bridges["slack"].In {
		t.Errorf("the in run is given %#v", s.Input["settings"])
	}
	d.answer(d.ask("C1/1.0", "hello"), "bot", "hi")
	out := d.carry()
	if len(out) != 1 || !reflect.DeepEqual(out[0].Input["settings"], want) || out[0].Machine != d.o.Bridges["slack"].Out {
		t.Errorf("the out runs are %#v", out)
	}
}

func TestBridgesSettingsPassedUnread(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", "  slack:\n    places:\n      \"#help\": { guests: true }\n",
		"  slack:\n    token: not-a-key\n    workspace: { id: T01, team: payments, guests: true }\n    places:\n      \"#help\": { guests: true, team: office, channel: C0HELP, readers: all }\n")
	d := open(t, files)
	s, err := d.Hear("slack", bridges.Delivery{})
	must(t, err)
	settings := s.Input["settings"].(map[string]any)
	if settings["token"] != "not-a-key" || !reflect.DeepEqual(settings["workspace"], map[string]any{"id": "T01", "team": "payments", "guests": true}) {
		t.Errorf("the settings are handed as %#v", settings)
	}
	// What the bridge's own settings say of a team or of guests is not read.
	n := d.ask("C1/1.0", "hello")
	if r, _ := d.requests.Get(n); r.Team != office.Top {
		t.Errorf("the request went to %s", r.Team)
	}
	// A bridge that runs the folder Bulldog ships is handed its from with
	// the rest.
	shipped := open(t, testkit.Files(t, examples.Work()))
	s = shipped.listening("slack")
	if settings := s.Input["settings"].(map[string]any); settings["from"] != "./.bulldog/bridges/slack" || s.Machine != shipped.o.Bridges["slack"].In {
		t.Errorf("the shipped Slack is handed %#v", settings)
	}
}

func TestBridgesIdentityByBridgeName(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "office.yaml", "  gitlab:\n", "  work-slack:\n    places:\n      \"#help\": { guests: true }\n  gitlab:\n")
	maps.Copy(files, standin.In("work-slack", standin.Folder(true, true)))
	d := open(t, files)
	n := d.ask("C1/1.0", "on slack")
	if asks, _ := d.requests.Asks(n); asks.Kind != "person" || asks.Name != "dana" {
		t.Errorf("on slack, U0DANA00 is %#v", asks)
	}
	d.delivered("work-slack", said("U0DANA00", "#help", "C1/2.0", "on work-slack"))
	if asks, _ := d.requests.Asks(d.latest()); asks.Kind != "guest" || asks.Bridge != "work-slack" || asks.ID != "U0DANA00" {
		t.Errorf("on work-slack, U0DANA00 is %#v", asks)
	}
}

func TestBridgesBrokenBridgeStopsOnlyItself(t *testing.T) {
	t.Parallel()
	files := replace(t, work(t), "bridges/gitlab/in.yaml", "input: { settings: any, cursor: any }", "input: { settings: any, cursor: any, delivery: any }")
	files = replace(t, files, "office.yaml", "  gitlab:\n", "  jira: { places: {} }\n  matrix: { places: {} }\n  gitlab:\n")
	maps.Copy(files, standin.In("teams", standin.Folder(false, true)))
	matrix := standin.In("matrix", standin.Folder(false, true))
	delete(matrix, "bridges/matrix/out.yaml")
	maps.Copy(files, matrix)
	d := open(t, files)
	for _, name := range []string{"gitlab", "jira", "teams", "matrix"} {
		if len(d.o.Problems.OfBridge(name)) == 0 {
			t.Errorf("%s has no problem", name)
		}
		if _, err := d.Hear(name, bridges.Delivery{}); err == nil {
			t.Errorf("a delivery to %s is heard", name)
		}
		if _, err := d.SignIn(name, "al", "a link"); err == nil {
			t.Errorf("%s carries a sign-in link", name)
		}
	}
	if starts := d.listen(); len(starts) != 0 {
		t.Errorf("listening runs are started: %v", starts)
	}
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	if r, _ := d.requests.Get(n); r.Team != office.Top {
		t.Errorf("Dana's request went to %s", r.Team)
	}
	if lead, _, _ := d.requests.Lead(n); lead != "bot" {
		t.Errorf("Dana's request goes to %s", lead)
	}
}

func TestBridgesChatNameReserved(t *testing.T) {
	t.Parallel()
	files := replace(t, house(t), "office.yaml", "  telegram:\n", "  chat:\n")
	if _, problems := office.Parse(t.TempDir(), given(files)); !strings.Contains(problems.Error(), "chat") {
		t.Errorf("a bridge named chat reads with %v", problems)
	}
	files = replace(t, house(t), "office.yaml", `telegram: "4471"`, `telegram: "4471", chat: sam`)
	if _, problems := office.Parse(t.TempDir(), given(files)); !strings.Contains(problems.Error(), "chat") {
		t.Errorf("a person written with a chat identity reads with %v", problems)
	}
}

func TestBridgesPlacesListed(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	if p, ok := d.o.Place("slack", "#help"); !ok || p.Team != office.Top || !p.Guests {
		t.Errorf("#help is %#v", p)
	}
	if p, ok := d.o.Place("gitlab", "platform/api"); !ok || p.Team != "platform" || p.Guests {
		t.Errorf("platform/api is %#v", p)
	}
	for _, wrong := range []string{`"#help": { team: billing }`, `"#help": { guests: maybe }`} {
		e := open(t, replace(t, work(t), "office.yaml", `"#help": { guests: true }`, wrong))
		if _, err := e.Hear("slack", bridges.Delivery{}); err == nil || e.o.Bridges["slack"].Runs() {
			t.Errorf("with %s the Slack bridge is run", wrong)
		}
		if len(e.listen()) != 1 {
			t.Errorf("with %s the GitLab bridge does not listen", wrong)
		}
	}
	e := open(t, replace(t, work(t), "office.yaml", "    places:\n      \"#help\": { guests: true }\n", "    places: [\"#help\"]\n"))
	if e.o.Bridges["slack"].Runs() || len(e.o.Problems.OfBridge("slack")) == 0 {
		t.Error("places as a list is no problem of the Slack bridge")
	}
	// The house's Telegram bridge listening in no place takes nothing said
	// to it directly.
	h := open(t, replace(t, house(t), "office.yaml", "  telegram:\n    places:\n      direct: {}                   # as Bulldog ships it: Sam and Kit may ask the house in a direct message\n", "  telegram: {}\n"))
	h.hear("telegram", said("4471", "direct", "4471", "the socket sparked"))
	if h.latest() != 0 {
		t.Error("what Sam said directly made a request")
	}
	g := open(t, house(t))
	g.hear("telegram", said("4471", "direct", "4471", "the socket sparked"))
	if r, err := g.requests.Get(1); err != nil || r.Team != office.Top || r.Origin.Place != "4471" {
		t.Errorf("what Sam said directly is %#v", r)
	}
}
