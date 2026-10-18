package bridges_test

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie-bulldog/shipped/bridges"
)

func TestBridgesShippedFoldersAreChecked(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{}
	for name, text := range examples.NewHouse() {
		files[name] = text
	}
	listing := "bridges:\n"
	for _, b := range bridges.All {
		listing += "  " + b.Name + ": " + b.Settings + "\n"
		for name, text := range folder(t, b.Name) {
			files["bridges/"+b.Name+"/"+name] = []byte(text)
		}
	}
	files["office.yaml"] = []byte(strings.Replace(string(files["office.yaml"]), "models:", listing+"models:", 1))
	o, problems := office.Parse(t.TempDir(), files)
	if problems.Refuse() || len(o.Problems) > 0 {
		t.Fatalf("the shipped bridges read with\n%v", problems)
	}
	for name, want := range map[string][3]bool{"telegram": {true, false, true}, "slack": {false, true, true}, "gitlab": {true, false, false}, "jira": {true, false, false}} {
		b := o.Bridges[name]
		direct := slices.Contains(b.Out.Definition.Input, "direct")
		if !b.Runs() || b.Listens != want[0] || b.Hears != want[1] || direct != want[2] {
			t.Errorf("%s listens %v, hears %v, says things directly %v", name, b.Listens, b.Hears, direct)
		}
	}
	if p, ok := o.Place("telegram", office.Direct); !ok || p.Team != office.Top || p.Guests {
		t.Errorf("the shipped Telegram bridge's direct place is %#v", p)
	}
}

func TestBridgesExamplesHoldTheShippedCopies(t *testing.T) {
	t.Parallel()
	for office, names := range map[string][]string{"house": {"telegram"}, "work": {"slack", "gitlab"}} {
		tree := examples.House()
		if office == "work" {
			tree = examples.Work()
		}
		for _, name := range names {
			sub, err := fs.Sub(tree, "bridges/"+name)
			if err != nil {
				t.Fatal(err)
			}
			got := testkit.Files(t, sub)
			want := folder(t, name)
			if len(got) != len(want) {
				t.Errorf("the %s's %s holds %d files", office, name, len(got))
			}
			for file, text := range want {
				if got[file] != text {
					t.Errorf("the %s's bridges/%s/%s is not the shipped one", office, name, file)
				}
			}
		}
	}
}

func TestBridgesShippedBridgeIsAddedByAChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files := map[string]string{}
	for name, text := range examples.NewHouse() {
		files[name] = string(text)
	}
	dir := testkit.Folder(t, files)
	if err := shipped.Write(dir); err != nil {
		t.Fatal(err)
	}
	clock := testkit.NewClock(testkit.Monday)
	if _, err := changes.Begin(dir, clock.Now()); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, shipped.Dir, "bulldog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	keeper := office.Keep(dir)
	c, err := changes.Open(ctx, db, dir, keeper)
	if err != nil {
		t.Fatal(err)
	}
	c.Now = clock.Now
	c.Decide = func(changes.Put) error { return nil }
	c.Told = func(changes.Outcome) {}
	c.Shipped = bridges.Bridge
	edited, _, err := c.Edited(changes.Edit{Bridge: "telegram"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range edited {
		paths = append(paths, f.Path)
		if f.Path == "office.yaml" && !strings.Contains(f.Text, "telegram: { places: { direct: {} } }") {
			t.Errorf("office.yaml would be\n%s", f.Text)
		}
	}
	if !slices.Equal(paths, []string{"bridges/telegram/in.yaml", "bridges/telegram/out.yaml", "bridges/telegram/telegram.js", "office.yaml"}) {
		t.Errorf("the draft holds %v", paths)
	}
	check, err := c.Check(ctx, edited, changes.Drafter{Name: "sam", Person: true})
	if err != nil || len(check.Refused) > 0 {
		t.Fatalf("the change is checked %+v, %v", check, err)
	}
	if _, err := c.PutOwn(ctx, "sam", "Telegram, for Sam and Kit", edited, check.Mark); err != nil {
		t.Fatal(err)
	}
	history, err := c.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].By != "sam" || !slices.Equal(history[0].Files, paths) {
		t.Errorf("the history is %+v", history)
	}
	keeper.Look()
	if o := keeper.Office(); o == nil || !o.Bridges["telegram"].Runs() {
		t.Fatalf("the house reads with %v", keeper.Problems())
	}
	// From then on the folder is the office's own: an edit to its out
	// machine is a change like any other, and what Bulldog ships is not
	// written over it.
	out := filepath.Join(dir, "bridges", "telegram", "out.yaml")
	written := testkit.Files(t, os.DirFS(dir))["bridges/telegram/out.yaml"]
	own := written + "# the house's own\n"
	edit, _, err := c.Edited(changes.Edit{Files: map[string]*string{"bridges/telegram/out.yaml": &own}})
	if err != nil {
		t.Fatal(err)
	}
	check, err = c.Check(ctx, edit, changes.Drafter{Name: "sam", Person: true})
	if err != nil || len(check.Refused) > 0 {
		t.Fatalf("an edit to the out machine is checked %+v, %v", check, err)
	}
	if _, err := c.PutOwn(ctx, "sam", "The house's own words", edit, check.Mark); err != nil {
		t.Fatal(err)
	}
	if err := shipped.Write(dir); err != nil {
		t.Fatal(err)
	}
	if got := testkit.Files(t, os.DirFS(dir))["bridges/telegram/out.yaml"]; !strings.HasSuffix(got, "# the house's own\n") {
		t.Errorf("%s is\n%s", out, got)
	}
}
