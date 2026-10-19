package office_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestAFolderThatIsNoOfficeIsRefused(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "nowhere")
	if _, err := office.Read(missing); err == nil || !strings.Contains(err.Error(), "is not there") {
		t.Errorf("a folder that is not there reads as %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("reading a folder that is not there made it")
	}
	dir := testkit.Folder(t, map[string]string{"notes.md": "mine", "tools/a.js": "return 1"})
	if _, err := office.Read(dir); err == nil || !strings.Contains(err.Error(), "holds no office.yaml") {
		t.Errorf("a folder with no office.yaml reads as %v", err)
	}
	if got := testkit.Files(t, os.DirFS(dir)); len(got) != 2 || got["notes.md"] != "mine" || got["tools/a.js"] != "return 1" {
		t.Errorf("the folder holds %v afterwards", got)
	}
}

func TestATeamWithinMayHoldTeams(t *testing.T) {
	t.Parallel()
	o := reads(t, with(house(t), map[string]string{"garden/beds/team.yaml": "lead: digger\n", "garden/beds/digger.yaml": "flow: digger\nstages: { dig: { job: ./dig.js } }\n", "garden/beds/dig.js": "return 1"}))
	if !slices.Equal(o.Order, []string{"office", "garden", "beds"}) || o.Teams["beds"].Within != "garden" || o.Teams["garden"].Within != "office" {
		t.Errorf("the teams are %v", o.Order)
	}
}

func TestWorkersAreTheMachinesDirectlyInATeamsFolder(t *testing.T) {
	t.Parallel()
	o := reads(t, with(house(t), map[string]string{"tools/check.yaml": "flow: check\nstages: { look: { job: ./look.js } }\n", "tools/look.js": "return 1"}))
	if got := o.TopTeam().Workers; !slices.Equal(got, []string{"bins", "builder", "clerk", "fixer", "sparks", "steward"}) {
		t.Errorf("the house's workers are %v", got)
	}
	if got := o.Teams["garden"].Workers; !slices.Equal(got, []string{"gardener", "groundsman"}) {
		t.Errorf("the garden's workers are %v", got)
	}
	for _, not := range []string{"crew", "small-orders", "check"} {
		if o.Workers[not] != nil {
			t.Errorf("%s is read as a worker", not)
		}
	}
	// A file that cannot be read as YAML is a worker only when its first
	// line names a type of machine.
	o, problems := read(t, with(house(t), map[string]string{"crew.yaml": "fixer: [\n", "clerk.yaml": "# the clerk\nagent: clerk\nmodel: [usual\n"}))
	if o == nil || o.Workers["crew"] != nil || len(problems.OfWorker("crew")) > 0 {
		t.Errorf("a tools file that cannot be read is read as a worker: %v", problems)
	}
	says(t, problems.OfTeam("office"), "crew.yaml", "not YAML")
	says(t, problems.OfWorker("clerk"), "clerk.yaml", "not YAML")
	if o.Workers["clerk"] == nil || o.Workers["clerk"].Runs() || o.Workers["steward"].Runs() {
		t.Error("a worker that cannot be read runs, or one that names what cannot be read")
	}
	w := reads(t, work(t))
	if w.Workers["no-account-numbers"] != nil || slices.Contains(w.Teams["payments"].Workers, "no-account-numbers") {
		t.Error("payments' rule is read as a worker")
	}
}

func TestNothingWhoseNameStartsWithADotIsTheOffice(t *testing.T) {
	t.Parallel()
	plain := reads(t, house(t))
	dotted := reads(t, with(house(t), map[string]string{
		".git/HEAD": "ref: refs/heads/main", ".bulldog/identity": "x", ".scratch/team.yaml": "lead: nobody\n",
		".scratch/odd.yaml": "agent: odd\n", "garden/.hidden.yaml": "flow: hidden\n",
	}))
	if !slices.Equal(plain.Order, dotted.Order) || len(plain.Workers) != len(dotted.Workers) {
		t.Errorf("the teams %v and workers %d differ from %v and %d", dotted.Order, len(dotted.Workers), plain.Order, len(plain.Workers))
	}
	for _, file := range dotted.Files {
		if strings.HasPrefix(file, ".") || strings.Contains(file, "/.") {
			t.Errorf("%s is among the office's files", file)
		}
	}
}

func TestOfficeStrayTeamFileRefused(t *testing.T) {
	t.Parallel()
	for file, said := range map[string]string{
		"tools/old/team.yaml":        "not a team's folder",
		"team.yaml":                  "top team's file is office.yaml",
		"garden/office.yaml":         "read only at the top",
		"bridges/telegram/team.yaml": "not a team's folder",
	} {
		problems := refused(t, with(house(t), map[string]string{file: "lead: nobody\n"}))
		if p := says(t, problems, said); p.File != file {
			t.Errorf("the problem is of %s, not %s", p.File, file)
		}
	}
}

func TestOfficeNothingNamedOutsideFolder(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.js"), []byte("return 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := replace(t, house(t), "sparks.yaml", "./tools/circuits.js", "../../outside/secret.js")
	files = replace(t, files, "clerk.yaml", "prompt: ${input.said}", "prompt: ${input.said}\ntools: { pay: { does: Pay, args: {}, job: { script: ./tools/link.js } } }")
	files = with(files, map[string]string{".scratch/x.js": "return 1"})
	dir := testkit.Folder(t, files)
	for link, target := range map[string]string{
		"tools/link.js": filepath.Join(outside, "secret.js"),
		"tools/gone.js": filepath.Join(dir, "tools", "nothing.js"),
		"tools/dot.js":  filepath.Join("..", ".scratch", "x.js"),
	} {
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(link))); err != nil {
			t.Fatal(err)
		}
	}
	o, err := office.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	says(t, o.Problems.OfWorker("sparks"), "sparks.yaml", "outside the office's folder")
	says(t, o.Problems.OfWorker("clerk"), "clerk.yaml", "no tools/link.js")
	says(t, o.Problems, "tools/link.js", "outside the office's folder")
	says(t, o.Problems, "tools/gone.js", "to nothing")
	says(t, o.Problems, "tools/dot.js", ".scratch/x.js", "starts with a dot")
	if slices.Contains(o.Files, "tools/link.js") || slices.Contains(o.Files, "tools/dot.js") || o.Workers["sparks"].Runs() || o.Workers["clerk"].Runs() {
		t.Error("what is not the office's was read")
	}
	// A rule's path that leaves the team's folder names nothing either.
	says(t, refused(t, replace(t, work(t), "payments/team.yaml", "[./no-account-numbers.yaml]", "[../bot.yaml]")), "../bot.yaml", "leaves payments's folder")
}

func TestOfficeOwnFilesByPathOnly(t *testing.T) {
	t.Parallel()
	// A folder of the office with bulldog in its name is the office's.
	files := with(house(t), map[string]string{"tools/bulldog/talk.yaml": "nothing: { does: Nothing, args: {}, job: ../n.js }\n", "tools/n.js": "return 1"})
	files = replace(t, files, "clerk.yaml", "prompt: ${input.said}", "prompt: ${input.said}\ntools_file: [./tools/bulldog/talk.yaml]")
	o := reads(t, files)
	if got := o.Workers["clerk"].Tools; !slices.Contains(got, "nothing") || slices.Contains(got, "ask_team") {
		t.Errorf("the clerk holds %v", got)
	}
	// .bulldog/ in a team within is no folder of Bulldog's.
	files = replace(t, house(t), "garden/gardener.yaml", "../.bulldog/talk.yaml", "./.bulldog/talk.yaml")
	o, problems := read(t, files)
	says(t, problems.OfWorker("gardener"), "garden/.bulldog/talk.yaml", "not a file of the office")
	if o.Workers["gardener"].Runs() {
		t.Error("the gardener runs, naming .bulldog/ within the garden")
	}
}
