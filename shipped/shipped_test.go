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
	if call.Definition.Name != "call" || !slices.Equal(call.Definition.Input, []string{"tool", "args", "unanswered"}) ||
		!strings.Contains(body, `"ticket":"`+shipped.CallTicket+`"`) || !strings.Contains(body, `"too_long":"7d"`) || !strings.Contains(body, `"output":"${input.unanswered}"`) {
		t.Errorf("the call is %s, taking %v: %s", call.Definition.Name, call.Definition.Input, body)
	}
	for _, name := range []string{shipped.Talk, shipped.Notes, shipped.Build} {
		parse(t, "agent: someone\nmodel: { name: claude-sonnet-5-5, secret: KEY }\nprompt: hi\ntools_file: [./.bulldog/"+name+"]\n", nil)
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

func TestToolsNamed(t *testing.T) {
	t.Parallel()
	for name, want := range map[string][]string{
		shipped.Talk:  {"ask_person", "ask_team", "read_thread", "tell"},
		shipped.Notes: {"write_notes"},
		shipped.Build: {"manual", "office_check", "office_files", "office_put", "office_read", "office_remove", "office_write"},
		shipped.Ask:   nil,
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
	if !slices.Equal(things, []string{"tools/build", "tools/notes", "tools/talk", "bridges/gitlab", "bridges/jira", "bridges/slack", "bridges/telegram"}) {
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
	// A tools file's page shows it named by its path; a bridge's entry is
	// a mapping that lists its places.
	for _, file := range []string{shipped.Talk, shipped.Notes, shipped.Build} {
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

func TestNotesToolSaysWhoReadsNotes(t *testing.T) {
	t.Parallel()
	contents, _ := shipped.File(shipped.Notes)
	for _, said := range []string{"builder", "whoever decides a change", "secret"} {
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
	if d.Definition.Name != "clock" || !slices.Equal(d.Definition.Input, []string{"due"}) {
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
	for name, text := range map[string]string{shipped.Talk: "changed by hand", "talk.yaml": "flat, as it once was", "tools/talk/old.yaml": "no longer shipped", "identity": "the office's", "bulldog.db": "the office's"} {
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
	for _, gone := range []string{"talk.yaml", "tools/talk/old.yaml"} {
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
