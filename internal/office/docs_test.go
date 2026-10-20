package office_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// docs gives docs each as its name, of its team, and its size.
func docs(ds []office.Doc) []string {
	var out []string
	for _, d := range ds {
		out = append(out, fmt.Sprintf("%s of %s, %d", d.Name, d.Team, d.Size))
	}
	return out
}

func TestDocsAreReadFromTheFolderEachTime(t *testing.T) {
	t.Parallel()
	o := reads(t, with(house(t), map[string]string{
		"docs/boiler.md":               "# The boiler\nService in May.\n",
		"docs/rooms/kitchen.md":        "# The kitchen\n",
		"docs/rooms/.draft.md":         "Not yet.",
		"docs/.old/attic.md":           "# The attic\n",
		"docs/plan.pdf":                "not prose",
		"garden/docs/boiler.md":        "# The shed's heater\n",
		"garden/docs/tools/mowers.md":  "# Mowers\n",
		"garden/notes/docs-are-not.md": "A note.",
	}))
	house, err := o.Docs("office")
	if want := []string{"boiler of office, 29", "rooms/kitchen of office, 14"}; err != nil || !slices.Equal(docs(house), want) {
		t.Errorf("the house's docs are %v, %v, not %v", docs(house), err, want)
	}
	if err := os.WriteFile(filepath.Join(o.Dir, "docs", "rooms", "hall.md"), []byte("The hall."), 0o644); err != nil {
		t.Fatal(err)
	}
	if house, err = o.Docs("office"); err != nil || !slices.Contains(docs(house), "rooms/hall of office, 9") {
		t.Errorf("after a doc is written the house's docs are %v, %v", docs(house), err)
	}
	// The garden's workers read the garden's first, then the house's; its
	// boiler stands before the house's.
	reach, err := o.DocsFor("garden")
	if want := []string{"boiler of garden, 20", "tools/mowers of garden, 9", "rooms/hall of office, 9", "rooms/kitchen of office, 14"}; err != nil || !slices.Equal(docs(reach), want) {
		t.Errorf("the garden reads %v, %v, not %v", docs(reach), err, want)
	}
	if text, err := o.DocText(reach[0]); err != nil || text != "# The shed's heater\n" {
		t.Errorf("the garden's boiler reads %q, %v", text, err)
	}
	if reach, err := o.DocsFor("office"); err != nil || len(reach) != 3 {
		t.Errorf("the house reads %v, %v", docs(reach), err)
	}
	for _, name := range []string{"../notes/rooms", "rooms/../../office", "/etc/passwd", ".old/attic", ""} {
		if text, err := o.DocText(office.Doc{Name: name, Team: "office"}); err == nil {
			t.Errorf("%q is read as a doc: %q", name, text)
		}
	}
	if !slices.Contains(o.Files, "garden/docs/tools/mowers.md") {
		t.Error("a doc is not among the office's files")
	}
	if ds, err := reads(t, work(t)).Docs("support"); err != nil || len(ds) != 0 {
		t.Errorf("support's docs are %v, %v", docs(ds), err)
	}
}
