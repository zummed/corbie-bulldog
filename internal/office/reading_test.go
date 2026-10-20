package office_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestTheOfficeIsReadWhole(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	want := []string{"bins.yaml", "builder.yaml", "clerk.yaml", "fixer.yaml", "sparks.yaml", "steward.yaml",
		"garden/gardener.yaml", "garden/groundsman.yaml", "small-orders.js", ".bulldog/bridges/telegram/in.yaml", ".bulldog/bridges/telegram/out.yaml"}
	if got := slices.Sorted(slices.Values(o.Checked)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("Corbie checked %v", o.Checked)
	}
	for _, name := range []string{"fixer", "gardener", "steward"} {
		if w := o.Workers[name]; !strings.Contains(string(w.Definition), "service: claude-code }") {
			t.Errorf("%s is checked as %s", name, w.Definition)
		}
	}
	if !o.Rules["small-orders.js"].Script {
		t.Error("small-orders.js is not checked as the bare script it is")
	}
	if w := reads(t, work(t)); !slices.Contains(w.Checked, "payments/no-account-numbers.yaml") || w.Rules["payments/no-account-numbers.yaml"].Script {
		t.Error("payments' rule is not checked as a definition")
	}
}

func TestOfficeEveryProblemTogether(t *testing.T) {
	t.Parallel()
	files := replace(t, own(t, house(t), "telegram"), "office.yaml", "people: [sam, kit]", "people: [sam, nobody]")
	files = replace(t, files, "garden/team.yaml", "lead: gardener", "lead: gardener\nbudget: 3")
	files = replace(t, files, "clerk.yaml", "model: usual", "model: cheap")
	files = replace(t, files, "bridges/telegram/in.yaml", "script: ./telegram.js", "script: ./gone.js")
	problems := refused(t, files)
	for _, p := range []office.Problem{
		says(t, problems, "office.yaml", "nobody"),
		says(t, problems, "garden/team.yaml", "budget"),
		says(t, problems, "clerk.yaml", "cheap"),
		says(t, problems, "bridges/telegram/in.yaml", "gone.js"),
	} {
		if p.File == "" || p.Text == "" {
			t.Errorf("the problem %#v does not say its file and what is wrong", p)
		}
	}
	if p := says(t, problems, "office.yaml", "nobody"); p.Line != 14 {
		t.Errorf("the problem is said at line %d", p.Line)
	}
}

func TestOfficeChecksOffline(t *testing.T) {
	t.Parallel()
	// No Corbie is running, and the models are at addresses nothing
	// answers: the office reads all the same.
	files := replace(t, house(t), "office.yaml", "usual: { name: sonnet, service: claude-code }", `usual: { name: claude-sonnet-5-5, address: "http://127.0.0.1:9", secret: KEY }`)
	o := reads(t, files)
	for name, w := range o.Workers {
		if !w.Runs() {
			t.Errorf("%s was not read offline", name)
		}
	}
}

func TestOfficeBrokenDefinitionStopsItself(t *testing.T) {
	t.Parallel()
	// A worker no other runs, a rule and a bridge, each refused by Corbie.
	files := replace(t, own(t, house(t), "telegram"), "bins.yaml", "stages:", "stagse:")
	files = replace(t, files, "office.yaml", "[./small-orders.js, clerk: advise, sam]", "[./big-orders.yaml, sam]")
	files = with(files, map[string]string{"big-orders.yaml": "flow: big-orders\nstages: { look: { job: ./nowhere.js } }\n"})
	files = replace(t, files, "bridges/telegram/out.yaml", "flow: telegram-out", "flw: telegram-out")
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the reading is refused: %v", problems)
	}
	says(t, problems.OfWorker("bins"), "bins.yaml", "stagse")
	if p := says(t, problems.OfRule("big-orders.yaml"), "big-orders.yaml", "nowhere.js"); p.Team != "office" || p.Worker != "" || p.Refuses {
		t.Errorf("the rule's problem is %#v", p)
	}
	says(t, problems.OfTeam("office"), "big-orders.yaml")
	says(t, problems.OfBridge("telegram"), "bridges/telegram/out.yaml")
	if o.Workers["bins"].Runs() || o.Rules["big-orders.yaml"].Runs() || o.Bridges["telegram"].Runs() {
		t.Error("a broken definition runs")
	}
	// Each stops only itself: every other worker runs, with no problem.
	for _, name := range []string{"steward", "fixer", "sparks", "clerk", "builder", "gardener", "groundsman"} {
		if !o.Workers[name].Runs() || len(problems.OfWorker(name)) > 0 {
			t.Errorf("%s does not run: %v", name, problems.OfWorker(name))
		}
	}
	if len(problems) != len(problems.OfWorker("bins"))+len(problems.OfRule("big-orders.yaml"))+len(problems.OfBridge("telegram")) {
		t.Errorf("more is stopped than the broken files: %v", problems)
	}
}

// keeper writes an office into a folder and keeps it.
func keeper(t *testing.T, files map[string]string) (*office.Keeper, string) {
	dir := testkit.Folder(t, files)
	return office.Keep(dir), dir
}

func write(t *testing.T, dir, file, contents string) {
	t.Helper()
	testkit.Write(t, dir, map[string]string{file: contents})
}

func TestARefusedReadingLeavesTheLastGoodOne(t *testing.T) {
	t.Parallel()
	k, dir := keeper(t, house(t))
	first := k.Office()
	if first == nil {
		t.Fatal(k.Problems())
	}
	write(t, dir, "garden/steward.yaml", "agent: steward\nmodel: usual\nprompt: hi\n")
	if k.Look() {
		t.Error("a refused reading replaced the last")
	}
	if k.Office() != first || first.Workers["steward"].Team != "office" {
		t.Error("the office is not as it was last read well")
	}
	says(t, k.Problems(), "steward", "used twice")
}

func TestTheProblemsAreTheLatestReadings(t *testing.T) {
	t.Parallel()
	files := replace(t, house(t), "sparks.yaml", ", job: { script: ./tools/circuits.js }", "")
	k, dir := keeper(t, files)
	says(t, k.Problems(), "sparks.yaml")
	write(t, dir, "sparks.yaml", house(t)["sparks.yaml"])
	if !k.Look() || len(k.Problems()) > 0 {
		t.Errorf("once mended, the problems are %v", k.Problems())
	}
}

func TestNothingIsReadWellUntilItIs(t *testing.T) {
	t.Parallel()
	k, dir := keeper(t, replace(t, house(t), "office.yaml", "lead: steward\n", ""))
	if k.Office() != nil {
		t.Fatal("an office that never read well is given")
	}
	says(t, k.Problems(), "office.yaml", "lead")
	write(t, dir, "office.yaml", house(t)["office.yaml"])
	if !k.Look() || k.Office() == nil {
		t.Errorf("mended, it does not read: %v", k.Problems())
	}
	// No reading is kept from one keeper to the next.
	write(t, dir, "office.yaml", strings.Replace(house(t)["office.yaml"], "lead: steward\n", "", 1))
	if again := office.Keep(dir); again.Office() != nil {
		t.Error("a new keeper has a reading of the last")
	}
}

func TestAChangeReachesTheNextReading(t *testing.T) {
	t.Parallel()
	k, dir := keeper(t, house(t))
	// An office kept by a link to its folder is read, and looked at, where
	// the link points.
	link := filepath.Join(t.TempDir(), "house")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	linked := office.Keep(link)
	before := k.Office()
	write(t, dir, "fixer.yaml", strings.Replace(house(t)["fixer.yaml"], "You do the repairs.", "You do the repairs, carefully.", 1))
	for _, k := range []*office.Keeper{k, linked} {
		if !k.Look() {
			t.Fatal("the change was not noticed")
		}
		if !strings.Contains(string(k.Office().Workers["fixer"].Definition), "carefully") || strings.Contains(string(before.Workers["fixer"].Definition), "carefully") {
			t.Error("the new reading does not hold the change, or the old one does")
		}
		if k.Look() {
			t.Error("files that did not change were read again")
		}
	}
}

func TestOfficeRenameNoticed(t *testing.T) {
	t.Parallel()
	k, dir := keeper(t, house(t))
	if err := os.Rename(filepath.Join(dir, "garden", "notes", "shed.md"), filepath.Join(dir, "garden", "notes", "sheds.md")); err != nil {
		t.Fatal(err)
	}
	if !k.Look() {
		t.Error("a rename was not noticed")
	}
	clerk := filepath.Join(dir, "clerk.yaml")
	info, err := os.Stat(clerk)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clerk, []byte(strings.Replace(house(t)["clerk.yaml"], "accounts", "books", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	old := info.ModTime().Add(-time.Hour)
	if err := os.Chtimes(clerk, old, old); err != nil {
		t.Fatal(err)
	}
	if !k.Look() || !strings.Contains(string(k.Office().Workers["clerk"].Definition), "books") {
		t.Error("a write that left the file older was not noticed")
	}
}

func TestOfficeReadingReplacedWhole(t *testing.T) {
	t.Parallel()
	k, dir := keeper(t, house(t))
	// each writes one word into two files, the house's about and the
	// clerk's definition.
	each := func(say string) map[string]string {
		files := replace(t, house(t), "office.yaml", "but the garden.", "but the garden, "+say+".")
		files = replace(t, files, "clerk.yaml", "You keep", "You keep, "+say+",")
		return map[string]string{"office.yaml": files["office.yaml"], "clerk.yaml": files["clerk.yaml"]}
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	stopped := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}
	// held is what a reading says of the two files. A file is read while
	// it is written, so the clerk may be missing from a reading.
	held := func(o *office.Office) string {
		said := o.Teams["office"].About
		if clerk := o.Workers["clerk"]; clerk != nil {
			said += string(clerk.Definition)
		}
		return said
	}
	// Readers hold a reading while the files change and other looks read
	// them again: what they hold is never changed under them.
	for range 4 {
		wg.Go(func() {
			for !stopped() {
				o := k.Office()
				before := held(o)
				k.Office()
				if held(o) != before {
					t.Error("a reading changed while it was held")
					return
				}
			}
		})
	}
	for range 2 {
		wg.Go(func() {
			for !stopped() {
				k.Look()
			}
		})
	}
	// Each word is longer than the last, so each write changes the files'
	// sizes whatever their times.
	var last string
	for i := range 20 {
		last = "word" + strings.Repeat("o", i)
		testkit.Write(t, dir, each(last))
	}
	// A look that starts after the last write keeps its reading, whatever
	// looks were under way while the files were written.
	k.Look()
	close(stop)
	wg.Wait()
	o := k.Office()
	if !strings.Contains(o.Teams["office"].About, ", "+last+".") || !strings.Contains(string(o.Workers["clerk"].Definition), ", "+last+",") {
		t.Errorf("the office kept is not the last reading: %q and %q", o.Teams["office"].About, o.Workers["clerk"].Definition)
	}
}

func TestProblemsAreOfOneWorkerBridgeRuleOrTeam(t *testing.T) {
	t.Parallel()
	// A bridge may share a team's name, and its problems are its own.
	files := replace(t, house(t), "office.yaml", "bridges:\n", "bridges:\n  garden: {}\n")
	files = replace(t, files, "garden/groundsman.yaml", "prompt:", "promt:")
	o, problems := read(t, files)
	if o == nil {
		t.Fatalf("the house is refused: %v", problems)
	}
	bridge, team := problems.OfBridge("garden"), problems.OfTeam("garden")
	if len(bridge) != 1 || !strings.Contains(bridge[0].Text, "bridges/garden") {
		t.Errorf("the bridge's problems are %v", bridge)
	}
	if len(team) != 1 || !strings.Contains(team[0].Text, "lead: gardener") {
		t.Errorf("the garden's problems are %v", team)
	}
	if len(problems.OfWorker("groundsman")) != 1 || len(problems.OfWorker("gardener")) != 1 {
		t.Errorf("the garden's workers' problems are %v", problems)
	}
}
