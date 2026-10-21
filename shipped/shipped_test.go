package shipped_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
	"go.yaml.in/yaml/v3"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// load reads the files of a test's definition, and Bulldog's own at
// .bulldog/ from what is shipped, as an office does.
func load(files map[string]string) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		if own, ok := strings.CutPrefix(path.Clean(name), shipped.Dir+"/"); ok {
			if contents, ok := shipped.File(own); ok {
				return contents, nil
			}
		}
		if contents, ok := files[path.Clean(name)]; ok {
			return []byte(contents), nil
		}
		return nil, fmt.Errorf("there is no %s", name)
	}
}

func parse(t *testing.T, src string, files map[string]string) *machine.Draft {
	t.Helper()
	d, err := machine.Parse([]byte(src), load(files))
	if err != nil {
		t.Fatalf("the definition is refused:\n%v", err)
	}
	return d
}

func TestEveryShippedFileIsADefinitionCorbieTakes(t *testing.T) {
	t.Parallel()
	for _, name := range []string{shipped.Ask, shipped.Live, shipped.Clock} {
		contents, _ := shipped.File(name)
		if _, err := machine.Parse(contents, load(nil)); err != nil {
			t.Errorf("%s is refused:\n%v", name, err)
		}
	}
	// Every tool Bulldog ships runs the one call, which waits on its
	// ticket, and says what its tool was given to say when no answer comes.
	contents, _ := shipped.File(shipped.Call)
	call := parse(t, string(contents), nil)
	body := string(call.Definition.Body)
	takes, _ := call.Definition.Input["properties"].(map[string]any)
	if call.Definition.Name != "call" || len(takes) != 3 || takes["tool"] == nil || takes["args"] == nil || takes["unanswered"] == nil ||
		!strings.Contains(body, `"ticket":"`+shipped.CallTicket+`"`) || !strings.Contains(body, `"too_long":"7d"`) || !strings.Contains(body, `"output":"${input.unanswered}"`) {
		t.Errorf("the call is %s, taking %v: %s", call.Definition.Name, call.Definition.Input, body)
	}
	for _, name := range shipped.ToolsFiles {
		parse(t, "agent: someone\nmodel: { name: claude-sonnet-5-5, secret: KEY }\nprompt: hi\ntools_file: [./.bulldog/"+name+"]\nask: { machine: ./.bulldog/ask.yaml }\n", nil)
		contents, _ := shipped.File(name)
		var tools map[string]struct {
			Job struct {
				Machine string
				Input   map[string]any
			}
		}
		if err := yaml.Unmarshal(contents, &tools); err != nil {
			t.Fatal(err)
		}
		for tool, written := range tools {
			if in := written.Job.Input; written.Job.Machine != "../../call.yaml" || in["tool"] != tool || in["args"] != "${args}" || in["unanswered"] == "" || in["unanswered"] == nil {
				t.Errorf("%s's %s runs %s, given %v", name, tool, written.Job.Machine, in)
			}
		}
	}
	// A worker in a team within names the files by their path from its own
	// folder, and its tools wait the same way.
	files := map[string]string{"garden/gardener.yaml": "agent: gardener\nmodel: { name: claude-sonnet-5-5, secret: KEY }\nprompt: hi\ntools_file: [../.bulldog/tools/talk/talk.yaml]\nask: { machine: ../.bulldog/ask.yaml }\n"}
	if _, err := machine.Parse([]byte(files["garden/gardener.yaml"]), func(name string) ([]byte, error) {
		return load(files)(path.Join("garden", name))
	}); err != nil {
		t.Errorf("a worker within a team cannot name what Bulldog ships:\n%v", err)
	}
}

func TestToolsFilesMarkTheirWritesForAYes(t *testing.T) {
	t.Parallel()
	for _, name := range shipped.ToolsFiles {
		contents, _ := shipped.File(name)
		var tools map[string]struct {
			Job struct{ Approve bool }
		}
		if err := yaml.Unmarshal(contents, &tools); err != nil {
			t.Fatal(err)
		}
		for tool, written := range tools {
			if want := tool == "write_doc" || tool == "put_draft"; written.Job.Approve != want {
				t.Errorf("%s's %s is marked approve %v", name, tool, written.Job.Approve)
			}
		}
	}
}

func TestToolsNamed(t *testing.T) {
	t.Parallel()
	for name, want := range map[string][]string{
		shipped.Talk:        {"ask_person", "ask_team", "read_thread", "tell"},
		shipped.Notes:       {"write_note"},
		shipped.DocsRead:    {"list_docs", "read_doc", "search_docs"},
		shipped.DocsWrite:   {"write_doc"},
		shipped.OfficeRead:  {"list_files", "manual", "read_file", "search_files"},
		shipped.OfficeWrite: {"check_draft", "put_draft", "remove_file", "write_file"},
		shipped.Ask:         nil,
	} {
		if got := shipped.Tools(name); !slices.Equal(got, want) {
			t.Errorf("%s holds %v, not %v", name, got, want)
		}
	}
	if _, ok := shipped.File("bulldog.db"); ok {
		t.Error("the store is given as a shipped file")
	}
}

func TestEveryShippedThingHasItsPage(t *testing.T) {
	t.Parallel()
	things := shipped.Things()
	if !slices.Equal(things, []string{"tools/docs", "tools/notes", "tools/office", "tools/talk", "bridges/gitlab", "bridges/jira", "bridges/slack", "bridges/telegram", "rules/for-us", "rules/mentioned", "signin/entra", "signin/google"}) {
		t.Errorf("Bulldog ships %v", things)
	}
	pages := shipped.Pages()
	for i, thing := range things {
		page, ok := shipped.File(path.Join(thing, shipped.Page))
		if !ok || i >= len(pages) || pages[i].Name != thing || pages[i].Group != "Bulldog" || pages[i].Text != string(page) {
			t.Errorf("%s has no page of the manual", thing)
			continue
		}
		if summary := pages[i].Summary; summary == "" || strings.HasPrefix(summary, "#") {
			t.Errorf("%s's page begins %q, not a line saying what it is", thing, summary)
		}
		sections := []string{"## Needs", "## Settings", "## Example", "## Changed"}
		if strings.HasPrefix(thing, "bridges/") {
			sections = []string{"## Needs", "## Settings", "## Leaves out", "## Example", "## Changed"}
		}
		for _, section := range sections {
			if !strings.Contains(string(page), "\n"+section+"\n") {
				t.Errorf("%s's page has no %s", thing, section)
			}
		}
	}
	// A tools file's page, and a rule's, shows it named by its path; a
	// bridge's entry is a mapping that lists its places.
	for _, file := range append(slices.Clone(shipped.ToolsFiles), shipped.Mentioned, shipped.ForUs) {
		page, _ := shipped.File(path.Join(path.Dir(file), shipped.Page))
		if !strings.Contains(string(page), "./"+path.Join(shipped.Dir, file)) {
			t.Errorf("the page of %s does not name it by its path", file)
		}
	}
	for _, name := range shipped.Bridges() {
		_, settings, _ := shipped.Bridge(name)
		var entry map[string]any
		if err := yaml.Unmarshal([]byte(settings), &entry); err != nil || entry["places"] == nil {
			t.Errorf("%s's entry is %q: %v", name, settings, err)
		}
	}
}

// reaches are what each thing Bulldog ships reaches, as its definitions
// write it, with the version of Bulldog whose Changed line says so. A
// thing whose reach changes says what it lets more by in a Changed line of
// a newer version, and is recorded here again with that version.
var reaches = map[string]struct {
	version string
	reach   []string
}{
	"tools/docs":       {"0.2.0", nil},
	"tools/notes":      {"0.2.0", nil},
	"tools/office":     {"0.3.0", nil},
	"tools/talk":       {"0.2.0", nil},
	"bridges/gitlab":   {"0.2.0", []string{"net:${input.settings.site}", "secret:GITLAB_TOKEN@net:${input.settings.site}"}},
	"bridges/jira":     {"0.2.0", []string{"net:${input.settings.site}", "secret:JIRA_TOKEN@net:${input.settings.site}"}},
	"bridges/slack":    {"0.2.0", []string{"net:files.slack.com", "net:slack.com", "net:wss-backup.slack.com", "net:wss-primary.slack.com", "secret:SLACK_APP_TOKEN@net:slack.com", "secret:SLACK_TOKEN@net:files.slack.com", "secret:SLACK_TOKEN@net:slack.com"}},
	"bridges/telegram": {"0.2.0", []string{"net:api.telegram.org", "secret:TELEGRAM_TOKEN@net:api.telegram.org"}},
	"rules/for-us":     {"0.2.0", []string{"net:api.typesafe.ai", "secret:TYPESAFE_KEY@net:api.typesafe.ai"}},
	"rules/mentioned":  {"0.2.0", nil},
	"signin/entra":     {"0.3.0", []string{"net:login.microsoftonline.com", "secret:ENTRA_CLIENT_SECRET@net:login.microsoftonline.com"}},
	"signin/google":    {"0.3.0", []string{"net:oauth2.googleapis.com", "secret:GOOGLE_CLIENT_SECRET@net:oauth2.googleapis.com"}},
}

func TestShippedReachChangeHasAChangedLine(t *testing.T) {
	t.Parallel()
	for _, thing := range shipped.Things() {
		was, ok := reaches[thing]
		if !ok {
			t.Errorf("what %s reaches is not recorded: record it with the version its page's Changed first names it in", thing)
			continue
		}
		changed := shipped.Changed(thing)
		if !slices.ContainsFunc(changed, func(c shipped.Change) bool { return c.Version == was.version }) {
			t.Errorf("%s is recorded as of %s, and its page has no Changed line for %s", thing, was.version, was.version)
		}
		now := shipped.Reach(thing)
		if slices.Equal(now, was.reach) {
			continue
		}
		if latest := shipped.Latest(changed); shipped.Newer(latest, was.version) {
			t.Errorf("%s now reaches %q, said in its Changed line for %s: record it here with %s", thing, now, latest, latest)
		} else {
			t.Errorf("%s now reaches %q, not %q, and its page has no Changed line newer than %s: say there what it lets more by, as %q", thing, now, was.reach, was.version, "0.3.0: may now connect to …")
		}
	}
}

func TestShippedSettingsAndChangesReadFromTheFiles(t *testing.T) {
	t.Parallel()
	got := map[string]any{}
	for _, s := range shipped.SettingsOf("rules/for-us") {
		got[s.Name] = s.Value
		if s.About == "" || strings.HasPrefix(s.About, "`") {
			t.Errorf("for-us's %s is said %q", s.Name, s.About)
		}
	}
	if len(got) != 4 || got["timeout"] != "10s" || got["threshold"] != 0.5 || got["model"] != "jev-latest" || got["question"] == nil {
		t.Errorf("for-us's settings are %v", got)
	}
	var names []string
	for _, s := range shipped.SettingsOf("bridges/gitlab") {
		names = append(names, s.Name)
		if s.Name == "site" && s.Value != "gitlab.com" {
			t.Errorf("gitlab's site is %v", s.Value)
		}
	}
	if !slices.Equal(names, []string{"from", "site", "places"}) {
		t.Errorf("gitlab's settings are %v", names)
	}
	if s := shipped.SettingsOf("rules/mentioned"); len(s) != 0 {
		t.Errorf("mentioned has the settings %v", s)
	}
	page := []byte("A copy.\n\n## Changed\n\n- 0.1.0: first shipped.\n- 0.2.0: run in place.\n- dev: not a version.\n")
	if c := shipped.ChangedIn(page); len(c) != 2 || shipped.Latest(c) != "0.2.0" || c[0].Text != "first shipped." {
		t.Errorf("the page's Changed lines are %v", c)
	}
	for _, v := range []struct {
		a, b  string
		newer bool
	}{{"0.2.0", "0.1.0", true}, {"0.10.0", "0.9.1", true}, {"0.2.0", "0.2.0", false}, {"0.2.0", "dev", false}, {"dev", "0.1.0", false}, {"0.2.0", "", false}} {
		if shipped.Newer(v.a, v.b) != v.newer {
			t.Errorf("%q newer than %q is not %v", v.a, v.b, v.newer)
		}
	}
}

func TestNotesToolSaysWhoReadsNotes(t *testing.T) {
	t.Parallel()
	contents, _ := shipped.File(shipped.Notes)
	for _, said := range []string{"reads the office's files", "whoever decides a change", "secret"} {
		if !strings.Contains(string(contents), said) {
			t.Errorf("the notes tool does not say %q", said)
		}
	}
}

func TestTheClockIsADefinitionCorbieTakes(t *testing.T) {
	t.Parallel()
	contents, _ := shipped.File(shipped.Clock)
	d, err := machine.Parse(contents, load(nil))
	if err != nil {
		t.Fatal(err)
	}
	if takes, _ := d.Definition.Input["properties"].(map[string]any); d.Definition.Name != "clock" || len(takes) != 1 || takes["due"] == nil {
		t.Errorf("the clock is %s, given %v", d.Definition.Name, d.Definition.Input)
	}
}

// child stays until a run below the root, of the machine named, waits on
// its ticket, and gives the run, its ticket's value and its input.
func child(t *testing.T, corbie *testkit.Corbie, label, machineName, ticket string) (string, string, map[string]any) {
	t.Helper()
	var found string
	corbie.Until(t, "a run of "+machineName+" is going", func() bool {
		runs, err := corbie.List(ctx, client.Filter{Labels: map[string]string{"test": label}})
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range runs {
			if run.Machine == machineName && run.Parent != "" && run.Status == "running" {
				found = run.ID
				return true
			}
		}
		return false
	})
	value := corbie.Waiting(t, found, ticket)
	var input map[string]any
	if _, err := corbie.Record(ctx, found, func(e client.Entry) {
		for _, event := range e.Events {
			if event.Name == "start" && input == nil {
				input, _ = event.Data.(map[string]any)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	return found, value, input
}

func TestAToolWaitsForBulldogsAnswer(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	model := testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_person", map[string]any{"question": "when?"})
		}
		return testkit.Say("told: " + turn.Last())
	})
	d := parse(t, "agent: fixer\nmodel: "+model.Line()+"\nprompt: go\ntools_file: [./.bulldog/tools/talk/talk.yaml]\n", nil)
	run, err := corbie.Start(ctx, d, client.Start{Labels: map[string]string{"test": "tool"}})
	if err != nil {
		t.Fatal(err)
	}
	call, ticket, input := child(t, corbie, "tool", "call", shipped.CallTicket)
	if input["tool"] != "ask_person" || input["args"].(map[string]any)["question"] != "when?" {
		t.Errorf("the call was given %#v", input)
	}
	if _, err := corbie.Reply(ctx, call, ticket, "Thursday", 0); err != nil {
		t.Fatal(err)
	}
	if ended := corbie.Ended(t, run); !strings.Contains(fmt.Sprint(ended.Output), "told: Thursday") {
		t.Errorf("the worker was given %#v", ended.Output)
	}
}

func TestAToolStopsWaitingAfterSevenDays(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	model := testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("ask_team", map[string]any{"team": "garden", "question": "beds?"})
		}
		return testkit.Say("told: " + turn.Last())
	})
	d := parse(t, "agent: fixer\nmodel: "+model.Line()+"\nprompt: go\ntools_file: [./.bulldog/tools/talk/talk.yaml]\n", nil)
	run, err := corbie.Start(ctx, d, client.Start{Labels: map[string]string{"test": "late"}})
	if err != nil {
		t.Fatal(err)
	}
	child(t, corbie, "late", "call", shipped.CallTicket)
	clock.Move(7 * 24 * time.Hour)
	if ended := corbie.Ended(t, run); !strings.Contains(fmt.Sprint(ended.Output), "still open") {
		t.Errorf("the worker was given %#v", ended.Output)
	}
}

func TestAHeldJobWaitsOnTheAsk(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	model := testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("cut_power", map[string]any{"circuit": "4"}).And("cut_power", map[string]any{"circuit": "5"})
		}
		return testkit.Say("then: " + turn.Last())
	})
	src := `agent: fixer
model: ` + model.Line() + `
prompt: go
ask: { machine: ./.bulldog/ask.yaml }
tools:
  cut_power: { does: Turn a circuit off, args: { circuit: string }, job: { script: ./cut.js, input: { circuit: "${args.circuit}" }, approve: true } }
`
	run, err := corbie.Start(ctx, parse(t, src, map[string]string{"cut.js": "return 'off ' + input.circuit"}), client.Start{Labels: map[string]string{"test": "held"}})
	if err != nil {
		t.Fatal(err)
	}
	ask, ticket, input := child(t, corbie, "held", "ask", shipped.AskTicket)
	jobs, _ := input["jobs"].([]any)
	if len(jobs) != 2 || input["asking"] == nil {
		t.Fatalf("the ask was given %#v", input)
	}
	approve := jobs[0].(map[string]any)["name"]
	if _, err := corbie.Reply(ctx, ask, ticket, map[string]any{"approve": []any{approve}, "note": "not today"}, 0); err != nil {
		t.Fatal(err)
	}
	ended := corbie.Ended(t, run)
	if said := fmt.Sprint(ended.Output); !strings.Contains(said, "off ") || !strings.Contains(said, "not today") {
		t.Errorf("the worker was given %#v", ended.Output)
	}
}

func TestAnAskNobodyAnswersDeniesAfterSevenDays(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	model := testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
		if turn.Number == 1 {
			return testkit.Use("cut_power", map[string]any{"circuit": "4"})
		}
		return testkit.Say("then: " + turn.Last())
	})
	src := `agent: fixer
model: ` + model.Line() + `
prompt: go
ask: { machine: ./.bulldog/ask.yaml }
tools:
  cut_power: { does: Turn a circuit off, args: { circuit: string }, job: { script: ./cut.js, approve: true } }
`
	run, err := corbie.Start(ctx, parse(t, src, map[string]string{"cut.js": "return 'off'"}), client.Start{Labels: map[string]string{"test": "unasked"}})
	if err != nil {
		t.Fatal(err)
	}
	child(t, corbie, "unasked", "ask", shipped.AskTicket)
	clock.Move(7 * 24 * time.Hour)
	if said := fmt.Sprint(corbie.Ended(t, run).Output); !strings.Contains(said, "Nobody decided within 7 days") {
		t.Errorf("the worker was given %s", said)
	}
}

func TestTheServersRunEndsAMinuteAfterItsLastRenewal(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	contents, _ := shipped.File(shipped.Live)
	d, err := machine.Parse(contents, load(nil))
	if err != nil {
		t.Fatal(err)
	}
	run, err := corbie.Start(ctx, d, client.Start{Labels: map[string]string{"test": "live"}})
	if err != nil {
		t.Fatal(err)
	}
	timers := func(n int) func() bool {
		return func() bool { shown, err := corbie.Show(ctx, run); return err == nil && shown.Timers == n }
	}
	corbie.Until(t, "the run is serving", timers(1))
	clock.Move(40 * time.Second)
	message, err := corbie.Send(ctx, run, shipped.Renew, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	corbie.Until(t, "the renewal is taken", func() bool {
		receipt, err := corbie.Receipt(ctx, message)
		return err == nil && receipt != nil
	})
	clock.Move(40 * time.Second)
	// Eighty seconds from the start, and forty from the renewal: still
	// serving. A minute after the renewal, it ends.
	if shown, err := corbie.Show(ctx, run); err != nil || shown.HasEnded() {
		t.Fatalf("the run ended with a renewal forty seconds old: %v %v", shown, err)
	}
	clock.Move(21 * time.Second)
	if ended := corbie.Ended(t, run); ended.Status != "done" {
		t.Errorf("the run ended %s", ended.Status)
	}
}

func TestWriteReplacesWhatIsThere(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Changed by hand, left by an earlier version, and the office's own.
	for name, text := range map[string]string{shipped.Talk: "changed by hand", "talk.yaml": "flat, as it once was", "tools/talk/old.yaml": "no longer shipped", "tools/build/build.yaml": "a folder no longer shipped", "identity": "the office's", "bulldog.db": "the office's"} {
		file := filepath.Join(dir, shipped.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := shipped.Write(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range shipped.Names() {
		written, err := os.ReadFile(filepath.Join(dir, shipped.Dir, name))
		if want, _ := shipped.File(name); err != nil || string(written) != string(want) {
			t.Errorf("%s is not as shipped: %v", name, err)
		}
	}
	for _, gone := range []string{"talk.yaml", "tools/talk/old.yaml", "tools/build"} {
		if _, err := os.Stat(filepath.Join(dir, shipped.Dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s is still there: %v", gone, err)
		}
	}
	for _, kept := range []string{"identity", "bulldog.db"} {
		if got, err := os.ReadFile(filepath.Join(dir, shipped.Dir, kept)); err != nil || string(got) != "the office's" {
			t.Errorf("%s is not kept: %q, %v", kept, got, err)
		}
	}
}
