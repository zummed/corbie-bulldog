package changes

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestChangesCommitSaysWhoWhyAndWhoDecided(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(12, "clerk.yaml", clerkAgain(t, "x"))
	f.write(12, "fixer.yaml", strings.Replace(house(t)["fixer.yaml"], ", approve: true } }\n  order_part", " } }\n  order_part", 1))
	p := f.put(12, "Let the fixer cut power alone\n\nIt waits a week for a yes.\n\nAnd the clerk is changed.")
	f.yes(p.ID, "sam", "Good. Mind the cooker.")
	// Read back from the repository alone.
	c := f.head()
	lines := strings.Split(strings.TrimRight(c.Message, "\n"), "\n")
	if lines[0] != "Let the fixer cut power alone" || lines[1] != "" || lines[2] != "It waits a week for a yes." {
		t.Errorf("the commit begins:\n%s", c.Message)
	}
	if !strings.Contains(c.Message, "\nAnd the clerk is changed.\n") {
		t.Errorf("the rest of why is not in it:\n%s", c.Message)
	}
	if !strings.Contains(c.Message, "\nWhat it does:\n- clerk.yaml is changed.\n") || !strings.Contains(c.Message, "\nWhat it lets more by:\n- fixer's tool cut_power no longer waits for a yes.\n") {
		t.Errorf("what it does is not in it:\n%s", c.Message)
	}
	end := lines[len(lines)-3:]
	if !slices.Equal(end, []string{"Decided-by: sam", "Note: Good. Mind the cooker.", "Request: 12"}) {
		t.Errorf("it ends %q", end)
	}
	// What lets more by is apart in the four places it is shown.
	has(t, p.LetsMoreBy, "cut_power no longer waits")
	hasNo(t, p.Says, "cut_power")
	has(t, f.decided()[0].LetsMoreBy, "cut_power no longer waits")
	h := f.history()[0]
	if h.DecidedBy != "sam" || h.Note != "Good. Mind the cooker." || h.Request != 12 || h.By != "builder" || h.Why != "Let the fixer cut power alone" {
		t.Errorf("the history reads %+v", h)
	}
	if !slices.Equal(h.LetsMoreBy, p.LetsMoreBy) || !slices.Equal(h.Does, p.Says) {
		t.Errorf("the history says %v and %v, and the change said %v and %v", h.Does, h.LetsMoreBy, p.Says, p.LetsMoreBy)
	}
}

func TestChangesCommitIsByWhoMadeIt(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.yes(f.put(1, "X").ID, "sam", "")
	if c := f.head(); c.Author.Name != "builder" || c.Committer.Name != "builder" || !strings.Contains(c.Message, "Decided-by: sam") {
		t.Errorf("the builder's change is by %s, committed by %s:\n%s", c.Author.Name, c.Committer.Name, c.Message)
	}
	text := clerkAgain(t, "sam's")
	files, _, err := f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	if err != nil {
		t.Fatal(err)
	}
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	p, err := f.c.PutOwn(ctx, "sam", "Sam's own", files, check.Mark)
	if err != nil {
		t.Fatal(err)
	}
	// The house's walk has no rule before Sam: it is written as it is put.
	if len(p.Walk) != 0 {
		t.Errorf("Sam's own change is asked of %+v", p.Walk)
	}
	if c := f.head(); c.Author.Name != "sam" || !strings.Contains(c.Message, "Decided-by: sam") || strings.Contains(c.Message, "Request:") {
		t.Errorf("Sam's own change is by %s:\n%s", c.Author.Name, c.Message)
	}
}

func TestChangesACommitSaysItsSubjectOnce(t *testing.T) {
	t.Parallel()
	m := message("One line\nand a second", []string{"a"}, nil, nil)
	if strings.Count(m, "One line") != 1 || !strings.HasPrefix(m, "One line\n\nand a second\n") {
		t.Errorf("the message is:\n%s", m)
	}
	if m := message("Only a subject", []string{"a"}, nil, nil); strings.Count(m, "Only a subject") != 1 {
		t.Errorf("the message is:\n%s", m)
	}
}

func TestChangesCommitNotesReadBack(t *testing.T) {
	t.Parallel()
	m := message("Why", []string{"Request: 99 is said to be done.", "two"}, nil, [][2]string{{DecidedBy, "sam"}, {NoteKey, "on\ntwo lines"}, {RequestOf, ""}})
	lines := strings.Split(strings.TrimRight(m, "\n"), "\n")
	if got := lines[len(lines)-2:]; !slices.Equal(got, []string{"Decided-by: sam", "Note: on two lines"}) {
		t.Errorf("the notes are %q in:\n%s", got, m)
	}
	if !strings.Contains(m, "\n- Request: 99 is said to be done.\n") {
		t.Errorf("a line of what it does is not set apart from the notes:\n%s", m)
	}
	c := read(&object.Commit{Message: m}, nil)
	if c.DecidedBy != "sam" || c.Note != "on two lines" || c.Request != 0 || len(c.Notes) != 2 || len(c.Does) != 2 {
		t.Errorf("read back as %+v", c)
	}
}

func TestChangesACommitHoldsOnlyItsFiles(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	// A file staged by hand, in the office and beside it.
	f.edit(map[string]string{"bins.yaml": house(t)["bins.yaml"] + "# staged by hand\n"})
	repo, _ := git.PlainOpen(f.dir)
	wt, _ := repo.Worktree()
	if _, err := wt.Add("bins.yaml"); err != nil {
		t.Fatal(err)
	}
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.yes(f.put(1, "X").ID, "sam", "")
	if h := f.history()[0]; !slices.Equal(h.Files, []string{"clerk.yaml"}) {
		t.Errorf("the commit holds %v", h.Files)
	}
	status, err := wt.Status()
	if err != nil {
		t.Fatal(err)
	}
	if s := status.File("bins.yaml"); s.Staging != git.Modified {
		t.Errorf("what was staged by hand is %+v", s)
	}
	if s, shown := status["clerk.yaml"]; shown {
		t.Errorf("the committed file shows as %+v", s)
	}
}

func TestChangesTheHistoryHoldsNoIdentity(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.yes(f.put(1, "X").ID, "sam", "")
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Fuses", Worker: "steward", Request: 2}); err != nil {
		t.Fatal(err)
	}
	repo, _ := git.PlainOpen(f.dir)
	commits, _ := repo.Log(&git.LogOptions{})
	commits.ForEach(func(c *object.Commit) error {
		if c.Author.Email != Placeholder || c.Committer.Email != Placeholder {
			t.Errorf("%s is by %s <%s>, committed by <%s>", c.Hash, c.Author.Name, c.Author.Email, c.Committer.Email)
		}
		if strings.Contains(c.Message, "4471") {
			t.Errorf("%s names Sam's Telegram identity", c.Hash)
		}
		return nil
	})
}

func TestChangesHistoryKeptWithNoGitProgram(t *testing.T) {
	t.Setenv("PATH", "")
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	if o := f.yes(f.put(1, "X").ID, "sam", ""); o.Ended != Written {
		t.Fatal(o)
	}
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Fuses", Worker: "steward"}); err != nil {
		t.Fatal(err)
	}
	if h := f.history(); len(h) != 3 {
		t.Errorf("the history is %+v", h)
	}
	if byHand, err := f.c.ByHand(); err != nil || len(byHand) != 0 {
		t.Errorf("edited by hand: %v, %v", byHand, err)
	}
}

func TestChangesHistoryShowsEveryChange(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.write(1, "tools/logo.png", "\x89PNG\x00\x01")
	built := f.yes(f.put(1, "The builder's").ID, "sam", "")
	f.later()
	text := clerkAgain(t, "sam's")
	files, _, _ := f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	if _, err := f.c.PutOwn(ctx, "sam", "Sam's", files, check.Mark); err != nil {
		t.Fatal(err)
	}
	f.later()
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Fuses", Worker: "steward", Request: 14}); err != nil {
		t.Fatal(err)
	}
	h := f.history()
	if len(h) != 4 || h[0].By != "steward" || h[1].By != "sam" || h[2].By != "builder" || h[3].By != "bulldog" {
		t.Fatalf("the history is %+v", h)
	}
	if h[0].DecidedBy != "" || h[1].DecidedBy != "sam" || h[2].DecidedBy != "sam" || h[0].Why != "Fuses" {
		t.Errorf("who decided: %q, %q, %q", h[0].DecidedBy, h[1].DecidedBy, h[2].DecidedBy)
	}
	_, versions, err := f.c.Commit("sam", built.Commit)
	if err != nil || len(versions) != 2 {
		t.Fatalf("the builder's change shows %+v, %v", versions, err)
	}
	if v := versions[0]; v.Path != "clerk.yaml" || v.Was != house(t)["clerk.yaml"] || v.Now != clerkAgain(t, "x") {
		t.Errorf("the clerk shows as %+v", v)
	}
	if v := versions[1]; v.Path != "tools/logo.png" || !v.Binary || !v.New {
		t.Errorf("the logo shows as %+v", v)
	}
	if _, _, err := f.c.Commit("sam", "nonsense"); err == nil {
		t.Error("a commit that is not there is shown")
	}
}

func TestChangesOfficeWithinALargerRepository(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	if _, err := git.PlainInit(top, false); err != nil {
		t.Fatal(err)
	}
	testkit.Write(t, top, map[string]string{"README": "the project\n"})
	repo, _ := git.PlainOpen(top)
	wt, _ := repo.Worktree()
	wt.Add("README")
	who := &object.Signature{Name: "someone", Email: "someone@example.com", When: testkit.Monday}
	if _, err := wt.Commit("The project", &git.CommitOptions{Author: who, Committer: who}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(top, "house")
	testkit.Write(t, dir, house(t))
	wt.AddWithOptions(&git.AddOptions{All: true})
	if _, err := wt.Commit("The house", &git.CommitOptions{Author: who, Committer: who}); err != nil {
		t.Fatal(err)
	}
	testkit.Write(t, top, map[string]string{"README": "the project, edited by hand\n"})
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "bulldog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := Open(ctx, db, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Decide = func(Put) error { return nil }
	if _, err := c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Fuses", Worker: "steward"}); err != nil {
		t.Fatal(err)
	}
	h, err := c.History("sam")
	if err != nil || len(h) != 2 || h[0].Why != "Fuses" || h[1].Why != "The house" || !slices.Equal(h[0].Files, []string{"notes/fuses.md"}) {
		t.Errorf("the office's history is %+v, %v", h, err)
	}
	testkit.Write(t, dir, map[string]string{"clerk.yaml": "agent: clerk\n"})
	if byHand, err := c.ByHand(); err != nil || !slices.Equal(byHand, []string{"clerk.yaml"}) {
		t.Errorf("edited by hand: %v, %v", byHand, err)
	}
	if contents, _ := os.ReadFile(filepath.Join(top, "README")); string(contents) != "the project, edited by hand\n" {
		t.Error("the file outside the office was touched")
	}
	// Its history is the larger repository's, and none is begun in it.
	if _, err := Begin(dir, testkit.Monday); !errors.Is(err, ErrInRepository) {
		t.Errorf("a repository was begun within another: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Errorf("the office's folder holds a repository: %v", err)
	}
}

func TestChangesHistorySeenOnlyByWhoDecidesAChange(t *testing.T) {
	t.Parallel()
	// The house writes no change list, and Sam alone is on its everything:
	// only Sam decides a change, and so only Sam reads the history.
	f := newOffice(t, house(t))
	if got := Deciders(f.keeper.Office()); !slices.Equal(got, []string{"sam"}) {
		t.Errorf("the history is seen by %v", got)
	}
	h, err := f.c.History("sam")
	if err != nil || len(h) == 0 {
		t.Fatalf("Sam reads the history %v, %v", h, err)
	}
	if _, err := f.c.History("kit"); err == nil || !strings.Contains(err.Error(), "kit does not decide") {
		t.Errorf("Kit reads the history: %v", err)
	}
	if _, _, err := f.c.Commit("kit", h[0].ID); err == nil {
		t.Error("Kit reads a change of the history")
	}
	if _, _, err := f.c.Commit("sam", h[0].ID); err != nil {
		t.Errorf("Sam reads a change of the history: %v", err)
	}
	// In the work office, Al.
	w := newOffice(t, work(t))
	if got := Deciders(w.keeper.Office()); !slices.Equal(got, []string{"al"}) {
		t.Errorf("the work office's history is seen by %v", got)
	}
}

func TestChangesRefusalsAndWhatIsNotThereAreTyped(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	var refusal Refusal
	if _, _, err := f.c.Edited(Edit{Move: map[string]string{"nobody": "garden"}}); !errors.As(err, &refusal) {
		t.Errorf("moving a worker that is not there gives %v", err)
	}
	if _, _, err := f.c.Edited(Edit{Set: []Setting{{Path: "notes/rooms.md", Field: []string{"x"}, Value: "y"}}}); !errors.As(err, &refusal) {
		t.Errorf("setting a field of a file that is no mapping gives %v", err)
	}
	if _, err := f.c.PutOwn(ctx, "kit", "Clerk", nil, "mark"); !errors.As(err, &refusal) {
		t.Errorf("Kit's put gives %v", err)
	}
	if _, err := f.c.PutOwn(ctx, "sam", "", nil, ""); !errors.As(err, &refusal) {
		t.Errorf("a put that cannot be put gives %v", err)
	}
	if _, err := f.c.History("kit"); !errors.As(err, &refusal) {
		t.Errorf("Kit's history gives %v", err)
	}
	h, err := f.c.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"nonsense", strings.Repeat("0", 40)} {
		if _, _, err := f.c.Commit("sam", id); !errors.Is(err, ErrNotThere) {
			t.Errorf("the change %s gives %v", id, err)
		}
	}
	// A commit beside the office's history is not one of its changes.
	f.edit(map[string]string{"notes/rooms.md": "Changed.\n"})
	f.commitByHand("Beside")
	repo, err := git.PlainOpen(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(head.Name(), plumbing.NewHash(h[0].ID))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.c.Commit("sam", head.Hash().String()); !errors.Is(err, ErrNotThere) {
		t.Errorf("a commit beside the history gives %v", err)
	}
	// A history that cannot be read is a failure.
	if err := os.RemoveAll(filepath.Join(f.dir, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.c.Commit("sam", h[0].ID); err == nil || errors.As(err, &refusal) || errors.Is(err, ErrNotThere) {
		t.Errorf("a change of a history that cannot be read gives %v", err)
	}
}

func TestChangesHandEditsAreListed(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.edit(map[string]string{"garden/team.yaml": house(t)["garden/team.yaml"] + "# by hand\n", "garden/new.md": "new\n"})
	os.Remove(filepath.Join(f.dir, "bins.yaml"))
	byHand, err := f.c.ByHand()
	if err != nil || !slices.Equal(byHand, []string{"bins.yaml", "garden/new.md", "garden/team.yaml"}) {
		t.Errorf("edited by hand: %v, %v", byHand, err)
	}
	f.commitByHand("By hand")
	if byHand, _ := f.c.ByHand(); len(byHand) != 0 {
		t.Errorf("once committed, edited by hand: %v", byHand)
	}
	// What Bulldog keeps beside the office is never an edit by hand.
	f.edit(map[string]string{".bulldog/other": "x"})
	if byHand, _ := f.c.ByHand(); len(byHand) != 0 {
		t.Errorf("Bulldog's own is edited by hand: %v", byHand)
	}
}

func TestChangesAHandEditIsNotCommitted(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.edit(map[string]string{"bins.yaml": house(t)["bins.yaml"] + "# by hand\n"})
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.yes(f.put(1, "X").ID, "sam", "")
	if _, err := f.c.WriteNote(ctx, Note{Team: "office", Name: "fuses", Text: "Breaker 4.\n", Why: "Fuses", Worker: "steward"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.history()[:2] {
		if slices.Contains(c.Files, "bins.yaml") {
			t.Errorf("%s committed the hand edit", c.Why)
		}
	}
	if byHand, _ := f.c.ByHand(); !slices.Equal(byHand, []string{"bins.yaml"}) {
		t.Errorf("edited by hand: %v", byHand)
	}
	// The office's first commit is the one that commits what is there.
	if first := f.history()[2]; first.By != "bulldog" || !slices.Contains(first.Files, "bins.yaml") || slices.ContainsFunc(first.Files, func(f string) bool { return strings.HasPrefix(f, ".") }) {
		t.Errorf("the first commit is %+v", first)
	}
}

func TestChangesHandEditsAreCurrentAfterAChange(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.call(1, ToolRemove, map[string]any{"path": "garden/notes/shed.md"})
	f.yes(f.put(1, "X").ID, "sam", "")
	if byHand, err := f.c.ByHand(); err != nil || len(byHand) != 0 {
		t.Errorf("at once after a change, edited by hand: %v, %v", byHand, err)
	}
	repo, _ := git.PlainOpen(f.dir)
	wt, _ := repo.Worktree()
	status, _ := wt.Status()
	for file, s := range status {
		if !strings.HasPrefix(file, ".bulldog/") {
			t.Errorf("git sees %s as %+v", file, s)
		}
	}
}

func TestChangesStoreOfAnotherBulldogIsRefused(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "bulldog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE parts (area TEXT PRIMARY KEY, version INTEGER NOT NULL); INSERT INTO parts VALUES ('changes', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, db, t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "remove .bulldog/bulldog.db and start again") {
		t.Errorf("a store of another Bulldog is opened: %v", err)
	}
}

func TestChangesNewOfficeWithinARepositoryHasItsOwn(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	if _, err := git.PlainInit(top, false); err != nil {
		t.Fatal(err)
	}
	testkit.Write(t, top, map[string]string{"README": "the project\n"})
	dir := filepath.Join(top, "offices", "house")
	testkit.Write(t, dir, house(t))
	if _, err := BeginOwn(dir, testkit.Monday); err != nil {
		t.Fatal(err)
	}
	r, err := openRepo(dir)
	if err != nil || r.in != "" {
		t.Fatalf("the new office's repository is %+v, %v", r, err)
	}
	if _, err := BeginOwn(dir, testkit.Monday); err == nil {
		t.Error("a repository was begun again over the office's own")
	}
	outer, _ := git.PlainOpen(top)
	if head, err := outer.Head(); err == nil {
		t.Errorf("the larger repository was given a commit: %v", head)
	}
}
