package office_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
)

func names(notes []office.Note) []string {
	var out []string
	for _, n := range notes {
		out = append(out, n.Name)
	}
	return out
}

func TestNotesAreReadFromTheFolderEachTime(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	notes, err := o.Notes("garden")
	if err != nil || !slices.Equal(names(notes), []string{"lawn", "shed"}) {
		t.Fatalf("the garden's notes are %v, %v", names(notes), err)
	}
	if err := os.WriteFile(filepath.Join(o.Dir, "garden", "notes", "beds.md"), []byte("Roses by the wall."), 0o644); err != nil {
		t.Fatal(err)
	}
	notes, err = o.Notes("garden")
	if err != nil || !slices.Equal(names(notes), []string{"beds", "lawn", "shed"}) || notes[0].Text != "Roses by the wall." {
		t.Errorf("after a note is written the garden's notes are %v, %v", notes, err)
	}
	if notes, err := reads(t, work(t)).Notes("support"); err != nil || len(notes) != 0 {
		t.Errorf("support's notes are %v, %v", notes, err)
	}
}

func TestNotesAreTheTeamsAndFilesOfTheOffice(t *testing.T) {
	t.Parallel()
	o := reads(t, with(house(t), map[string]string{"garden/notes/beds.md": "Roses by the wall."}))
	garden, _ := o.Notes("garden")
	house, _ := o.Notes("office")
	if !slices.Contains(names(garden), "beds") || slices.Contains(names(house), "beds") || !slices.Equal(names(house), []string{"rooms"}) {
		t.Errorf("the garden's notes are %v and the house's %v", names(garden), names(house))
	}
	if !slices.Contains(o.Files, "garden/notes/beds.md") {
		t.Error("a note is not among the office's files")
	}
	// The notes tool is had only by naming it.
	if slices.Contains(o.Workers["bins"].Tools, "write_note") || slices.Contains(o.Workers["fixer"].Tools, "write_note") {
		t.Error("a worker that names no notes.yaml has the notes tool")
	}
	if !slices.Contains(o.Workers["steward"].Tools, "write_note") || !slices.Contains(o.Workers["gardener"].Tools, "write_note") {
		t.Error("a worker that names notes.yaml has no notes tool")
	}
}
