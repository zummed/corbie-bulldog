package changes

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	_ "modernc.org/sqlite"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// An officeAt is an office in a folder of the test's own, in a repository
// of its own, with its changes in a store of its own: what was put to be
// decided, and every outcome told, are kept for the test to read.
type officeAt struct {
	t      *testing.T
	dir    string
	db     *sql.DB
	c      *Changes
	keeper *office.Keeper
	clock  *testkit.Clock

	mu   sync.Mutex
	puts []Put
	told []Outcome
}

// house gives the files of the house, and work those of the work office.
func house(t *testing.T) map[string]string { return testkit.Files(t, examples.House()) }
func work(t *testing.T) map[string]string  { return testkit.Files(t, examples.Work()) }

// newOffice writes an office's files into a folder, with what Bulldog
// ships, makes it a repository with its first commit, and opens its
// changes.
func newOffice(t *testing.T, files map[string]string) *officeAt {
	t.Helper()
	dir := testkit.Folder(t, files)
	if err := shipped.Write(dir); err != nil {
		t.Fatal(err)
	}
	clock := testkit.NewClock(testkit.Monday)
	if _, err := Begin(dir, clock.Now()); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, shipped.Dir, "bulldog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	f := &officeAt{t: t, dir: dir, db: db, clock: clock, keeper: office.Keep(dir)}
	f.c = f.open()
	return f
}

// open opens the office's changes again, as a server started again does.
func (f *officeAt) open() *Changes {
	f.t.Helper()
	c, err := Open(ctx, f.db, f.dir, f.keeper)
	if err != nil {
		f.t.Fatal(err)
	}
	c.Now = f.clock.Now
	c.Decide = func(p Put) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.puts = append(f.puts, p)
		return nil
	}
	c.Told = func(o Outcome) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.told = append(f.told, o)
	}
	c.Shipped = shippedBridge
	return c
}

// call calls a builder's tool as the builder, on a request, and gives
// its answer.
func (f *officeAt) call(request int64, tool string, args map[string]any) Answer {
	f.t.Helper()
	return f.callAs("builder", request, tool, args)
}

func (f *officeAt) callAs(worker string, request int64, tool string, args map[string]any) Answer {
	f.t.Helper()
	raw, _ := json.Marshal(args)
	a, err := f.c.Answer(ctx, Call{Request: request, Worker: worker, Tool: tool, Args: raw})
	if err != nil {
		f.t.Fatalf("%s: %v", tool, err)
	}
	return a
}

// write writes a file into the request's draft, as the builder.
func (f *officeAt) write(request int64, file, text string) {
	f.t.Helper()
	a := f.call(request, ToolWrite, map[string]any{"path": file, "text": text})
	if !strings.Contains(a.Text, "in your draft") && !strings.Contains(a.Text, "as it is") {
		f.t.Fatalf("writing %s: %s", file, a.Text)
	}
}

// put puts the request's draft, and gives the change put.
func (f *officeAt) put(request int64, why string) Put {
	f.t.Helper()
	a := f.call(request, ToolPut, map[string]any{"why": why})
	if a.Put == 0 {
		f.t.Fatalf("the draft was not put: %s", a.Text)
	}
	p, _, err := f.c.Put(ctx, a.Put)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

// yes is Sam's yes to a change put.
func (f *officeAt) yes(id int64, by, note string) Outcome {
	f.t.Helper()
	o, err := f.c.Decided(ctx, id, Decision{By: by, Kind: office.IsPerson, Called: f.called(by), Yes: true, Note: note})
	if err != nil {
		f.t.Fatal(err)
	}
	return o
}

// called gives what a person is called in the office as it is, as
// Decisions gives it.
func (f *officeAt) called(person string) string {
	f.t.Helper()
	o, err := f.c.office()
	if err != nil {
		f.t.Fatal(err)
	}
	p := o.People[person]
	if p == nil {
		f.t.Fatalf("%s is not a person of the office", person)
	}
	return p.Called
}

// text gives a file of the folder as it is now, or "" where it is not
// there.
func (f *officeAt) text(file string) string {
	contents, err := os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(file)))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return string(contents)
}

// edit writes files into the folder by hand.
func (f *officeAt) edit(files map[string]string) {
	f.t.Helper()
	testkit.Write(f.t, f.dir, files)
}

// commitByHand commits everything in the folder as a person would with
// git, outside Bulldog.
func (f *officeAt) commitByHand(why string) {
	f.t.Helper()
	repo, err := git.PlainOpen(f.dir)
	if err != nil {
		f.t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		f.t.Fatal(err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		f.t.Fatal(err)
	}
	who := &object.Signature{Name: "someone", Email: "someone@example.com", When: f.clock.Now()}
	if _, err := wt.Commit(why, &git.CommitOptions{Author: who, Committer: who}); err != nil {
		f.t.Fatal(err)
	}
}

// history gives the office's history, newest first.
func (f *officeAt) history() []Commit {
	f.t.Helper()
	h, err := f.c.History("sam")
	if err != nil {
		f.t.Fatal(err)
	}
	return h
}

// head gives the repository's last commit, read with the library alone.
func (f *officeAt) head() *object.Commit {
	f.t.Helper()
	repo, err := git.PlainOpen(f.dir)
	if err != nil {
		f.t.Fatal(err)
	}
	ref, err := repo.Head()
	if err != nil {
		f.t.Fatal(err)
	}
	c, err := repo.CommitObject(ref.Hash())
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

// decided gives the changes put to be decided so far.
func (f *officeAt) decided() []Put {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.puts)
}

// outcomes gives every outcome told so far.
func (f *officeAt) outcomes() []Outcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.told)
}

// check checks a request's draft as the builder.
func (f *officeAt) check(request int64) Check {
	f.t.Helper()
	files, err := f.c.Drafted(ctx, request)
	if err != nil {
		f.t.Fatal(err)
	}
	c, err := f.c.Check(ctx, files, Drafter{Name: "builder"})
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

// has fails unless one of the lines holds every one of the words.
func has(t *testing.T, lines []string, words ...string) string {
	t.Helper()
	for _, line := range lines {
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(line, w) }) {
			return line
		}
	}
	t.Fatalf("no line says %q; the lines are:\n%s", words, strings.Join(lines, "\n"))
	return ""
}

// hasNo fails if any of the lines holds every one of the words.
func hasNo(t *testing.T, lines []string, words ...string) {
	t.Helper()
	for _, line := range lines {
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(line, w) }) {
			t.Fatalf("a line says %q: %s", words, line)
		}
	}
}

// problemLines gives problems as lines.
func problemLines(ps office.Problems) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

// later moves the test's clock on a little.
func (f *officeAt) later() { f.clock.Move(time.Minute) }

// shippedBridge stands in for the bridges Bulldog ships: the Telegram
// bridge, as the house has it, with the settings it is shipped with.
func shippedBridge(name string) (fs.FS, string, bool) {
	if name != "telegram" {
		return nil, "", false
	}
	tree, err := fs.Sub(examples.House(), "bridges/telegram")
	if err != nil {
		panic(err)
	}
	return tree, "{ places: { direct: {} } }", true
}
