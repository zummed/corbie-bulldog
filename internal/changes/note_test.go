package changes

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestChangesNoteIsWrittenAndCommitted(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	id, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4 is the kitchen.\n", Why: "Breaker 4 is the kitchen", Worker: "steward", Request: 14})
	if err != nil {
		t.Fatal(err)
	}
	if f.text("notes/fuses.md") != "Breaker 4 is the kitchen.\n" {
		t.Errorf("notes/fuses.md holds %q", f.text("notes/fuses.md"))
	}
	h := f.history()[0]
	if h.ID != id || h.By != "steward" || !slices.Equal(h.Files, []string{"notes/fuses.md"}) {
		t.Errorf("the newest commit is %+v", h)
	}
	if len(f.decided()) != 0 {
		t.Error("someone was asked to decide a note")
	}
	// The garden's note is the garden's.
	if _, err := f.c.WriteNote(ctx, Note{Team: "garden", Name: "fuses", Text: "The shed has none.\n", Why: "No fuses", Worker: "gardener"}); err != nil {
		t.Fatal(err)
	}
	if f.text("garden/notes/fuses.md") != "The shed has none.\n" || f.text("notes/fuses.md") != "Breaker 4 is the kitchen.\n" {
		t.Error("the garden's note was written elsewhere")
	}
	// A note written as it is writes nothing, and says so.
	before := f.head().Hash
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4 is the kitchen.\n", Why: "Again", Worker: "steward"}); !errors.Is(err, ErrNoteAsItIs) || f.head().Hash != before {
		t.Errorf("a note written as it is: %v", err)
	}
	for _, name := range []string{"../fuses", "old/fuses", ".fuses", ""} {
		if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: name, Text: "x", Why: "x", Worker: "steward"}); err == nil {
			t.Errorf("a note named %q was written", name)
		}
	}
}

func TestChangesNoteCommitSaysWhoAndWhy(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Breaker 4 is the kitchen\nIt tripped twice this week.", Worker: "steward", Request: 14}); err != nil {
		t.Fatal(err)
	}
	m := f.head().Message
	want := "Breaker 4 is the kitchen\n\nIt tripped twice this week.\n\nWhat it does:\n- the house's note fuses is new.\n\nRequest: 14\n"
	if m != want {
		t.Errorf("the note's commit is\n%s\nand not\n%s", m, want)
	}
	if strings.Contains(m, DecidedBy) || strings.Contains(m, NoteKey+":") {
		t.Errorf("the note's commit names a decision:\n%s", m)
	}
	h := f.history()[0]
	if h.By != "steward" || h.DecidedBy != "" || h.More != "It tripped twice this week." || h.Request != 14 {
		t.Errorf("the history shows %+v", h)
	}
	f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 5.\n", Why: "Five", Worker: "steward"})
	has(t, f.history()[0].Does, "the house's note fuses is changed.")
}

func TestChangesANoteStandsApartFromTheDraft(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(3, "notes/rooms.md", house(t)["notes/rooms.md"]+"- The meters are in the porch.\n")
	f.write(3, "clerk.yaml", clerkAgain(t, "unfinished"))
	p := f.put(3, "Rooms")
	f.write(4, "clerk.yaml", clerkAgain(t, "unput"))
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "rooms", Text: "# Rooms\n\nAll rewritten.\n", Why: "Rooms", Worker: "steward", Request: 3}); err != nil {
		t.Fatal(err)
	}
	// Nothing put or withdrawn, and the drafts are as they were.
	if len(f.decided()) != 1 || len(f.outcomes()) != 0 {
		t.Errorf("the note put %+v and told %+v", f.decided(), f.outcomes())
	}
	for request, n := range map[int64]int{3: 2, 4: 1} {
		if draft, _ := f.c.Drafted(ctx, request); len(draft) != n {
			t.Errorf("request %d's draft is %+v", request, draft)
		}
	}
	// The change drafted against the note's old text is not written.
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Why, "notes/rooms.md") {
		t.Errorf("it ended %+v", o)
	}
}

func TestChangesANoteGoesTheOneWay(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	// Never over an edit by hand.
	f.edit(map[string]string{"notes/rooms.md": "by hand\n"})
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "rooms", Text: "by the steward\n", Why: "Rooms", Worker: "steward"}); err == nil || !strings.Contains(err.Error(), "edit by hand") {
		t.Errorf("a note was written over an edit by hand: %v", err)
	}
	if f.text("notes/rooms.md") != "by hand\n" {
		t.Error("the hand edit was written over")
	}
	// In a commit of the one note, whatever else is staged.
	repo, _ := git.PlainOpen(f.dir)
	wt, _ := repo.Worktree()
	wt.Add("notes/rooms.md")
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Fuses", Worker: "steward"}); err != nil {
		t.Fatal(err)
	}
	h := f.head()
	if h.Author.Name != "steward" || h.Author.Email != Placeholder {
		t.Errorf("the note is by %s <%s>", h.Author.Name, h.Author.Email)
	}
	if got := f.history()[0].Files; !slices.Equal(got, []string{"notes/fuses.md"}) {
		t.Errorf("the note's commit holds %v", got)
	}
	// Whole beside itself: nothing is left beside the note.
	entries, _ := os.ReadDir(filepath.Join(f.dir, "notes"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("%s is left in notes/", e.Name())
		}
	}
}
