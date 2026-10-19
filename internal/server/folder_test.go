package server

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

// readied readies a folder, and gives what it said.
func readied(t *testing.T, dir string, how Readying) (Identity, string, error) {
	t.Helper()
	var said bytes.Buffer
	id, err := Ready(dir, how, testkit.Monday, func(format string, args ...any) { fmt.Fprintf(&said, format+"\n", args...) })
	return id, said.String(), err
}

// commitOf gives the paths of the files the repository's one commit
// holds, and how many commits there are.
func commitOf(t *testing.T, dir string) ([]string, int) {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("%s is no repository of its own: %v", dir, err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	tree.Files().ForEach(func(f *object.File) error { files = append(files, f.Name); return nil })
	log, _ := r.Log(&git.LogOptions{})
	commits := 0
	log.ForEach(func(*object.Commit) error { commits++; return nil })
	slices.Sort(files)
	return files, commits
}

func TestServerFolderWithNoneNamed(t *testing.T) {
	// Run in a copy of the house, it is that copy.
	here := testkit.Folder(t, house(t))
	t.Chdir(here)
	if dir, err := Folder(""); err != nil || dir != here {
		t.Errorf("run in an office, the folder is %q, %v", dir, err)
	}
	// Run anywhere else, it is bulldog in the home.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	if dir, err := Folder(""); err != nil || dir != filepath.Join(home, "bulldog") {
		t.Errorf("run where there is no office, the folder is %q, %v", dir, err)
	}
	// A folder named is made whole.
	if dir, err := Folder("house"); err != nil || !filepath.IsAbs(dir) {
		t.Errorf("a folder named is %q, %v", dir, err)
	}
}

func TestServerNewFolderGivenTheExample(t *testing.T) {
	t.Parallel()
	for name, dir := range map[string]string{"an empty folder": t.TempDir(), "~/bulldog, not there": filepath.Join(t.TempDir(), "bulldog")} {
		_, said, err := readied(t, dir, Readying{Make: true})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(said, "there was no office in "+dir+", so the example was written there") || !strings.Contains(said, dir+" is a git repository now") {
			t.Errorf("%s: it said %q", name, said)
		}
		if _, err := os.Stat(filepath.Join(dir, "bridges")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: the example has a bridges folder: %v", name, err)
		}
		text, _ := os.ReadFile(filepath.Join(dir, office.OfficeFile))
		if strings.Contains(string(text), "bridges") || strings.Contains(string(text), "telegram") {
			t.Errorf("%s: the example's office.yaml names a bridge:\n%s", name, text)
		}
		files, commits := commitOf(t, dir)
		want := slices.Sorted(func(yield func(string) bool) {
			for name := range newHouse() {
				if !yield(name) {
					return
				}
			}
		})
		if commits != 1 || !slices.Equal(files, want) {
			t.Errorf("%s: the repository has %d commits, the first holding %v", name, commits, files)
		}
	}
}

func TestServerMissingFolderRefused(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "hous")
	_, _, err := readied(t, dir, Readying{})
	if err == nil || !strings.Contains(err.Error(), "corbie bulldog init "+dir) {
		t.Fatalf("a folder named that is not there was taken as %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the folder was made all the same: %v", err)
	}
}

func TestServerFolderThatIsNoOfficeUntouched(t *testing.T) {
	t.Parallel()
	dir := testkit.Folder(t, map[string]string{"notes.txt": "mine\n"})
	_, _, err := readied(t, dir, Readying{Make: true, Move: true})
	if err == nil || !strings.Contains(err.Error(), "no office.yaml") {
		t.Fatalf("a folder that is no office was taken as %v", err)
	}
	in, _ := os.ReadDir(dir)
	if len(in) != 1 || in[0].Name() != "notes.txt" {
		t.Errorf("something was written into it: %v", in)
	}
}

func TestServerNewOfficeHasOwnRepository(t *testing.T) {
	t.Parallel()
	top := testkit.Folder(t, map[string]string{"README": "another project\n"})
	if _, err := git.PlainInit(top, false); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(top, "offices", "house")
	if _, _, err := readied(t, dir, Readying{Make: true}); err != nil {
		t.Fatal(err)
	}
	if files, commits := commitOf(t, dir); commits != 1 || !slices.Contains(files, office.OfficeFile) {
		t.Errorf("the new office's repository has %d commits, holding %v", commits, files)
	}
}

func TestServerOfficeInNoRepositoryMadeOne(t *testing.T) {
	t.Parallel()
	dir := testkit.Folder(t, house(t))
	_, said, err := readied(t, dir, Readying{})
	if err != nil || !strings.Contains(said, "is a git repository now") {
		t.Fatalf("%v: %s", err, said)
	}
	files, commits := commitOf(t, dir)
	want := slices.Sorted(func(yield func(string) bool) {
		for name := range house(t) {
			if !yield(name) {
				return
			}
		}
	})
	if commits != 1 || !slices.Equal(files, want) {
		t.Errorf("the repository has %d commits, the first holding %v", commits, files)
	}
	testkit.Write(t, dir, map[string]string{".bulldog/later.txt": "Bulldog's own\n"})
	db, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := changes.Open(ctx, db, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if byHand, err := c.ByHand(); err != nil || len(byHand) != 0 {
		t.Errorf("edited by hand: %v, %v", byHand, err)
	}
}

func TestServerExampleReadsWell(t *testing.T) {
	t.Parallel()
	b := newBench(t, nil)
	s := b.serve()
	if !s.logged("so the example was written there") {
		t.Fatalf("the example was not written:\n%s", strings.Join(s.lines(), "\n"))
	}
	if ps := s.areasOf().Keeper.Problems(); len(ps) != 0 {
		t.Errorf("the example reads with problems: %v", ps)
	}
	if s.logged("a problem") {
		t.Errorf("the log names a problem:\n%s", strings.Join(s.lines(), "\n"))
	}
	s.answered(s.chat("kit", "", "is the kettle on?"))
}

func TestServerSaysOnlyWhatItServesSecondTime(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.listening(b.serve()).stop(); err != nil {
		t.Fatal(err)
	}
	s := b.serve()
	lines := s.lines()
	if len(lines) != 5 || index(lines, "on the Corbie") != 0 || index(lines, "The house:") != 1 || index(lines, "listening on") != 2 ||
		index(lines, "sam, open") != 3 || index(lines, "kit, open") != 4 {
		t.Errorf("served a second time, it said:\n%s", strings.Join(lines, "\n"))
	}
}

func TestServerOwnFolderHoldsWhatItShould(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := readied(t, dir, Readying{Make: true}); err != nil {
		t.Fatal(err)
	}
	db, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	for _, name := range append([]string{IdentityFile, StoreFile}, shipped.Names()...) {
		if _, err := os.Stat(filepath.Join(dir, shipped.Dir, name)); err != nil {
			t.Errorf(".bulldog/ holds no %s: %v", name, err)
		}
	}
	if files, _ := commitOf(t, dir); slices.ContainsFunc(files, func(f string) bool { return strings.HasPrefix(f, shipped.Dir) }) {
		t.Errorf("the commit holds Bulldog's own: %v", files)
	}
}

func TestServerShippedFilesMatchProgram(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.serve().stop(); err != nil {
		t.Fatal(err)
	}
	// Changed by hand, or left by an earlier version: its tools files
	// were flat, and a shipped folder held a file it no longer ships.
	b.write(map[string]string{".bulldog/tools/talk/talk.yaml": "# changed by hand\n", ".bulldog/ask.yaml": "", ".bulldog/talk.yaml": "# an earlier version's\n", ".bulldog/tools/talk/old.yaml": "# an earlier version's\n"})
	s := b.serve()
	s.areasOf()
	for _, name := range shipped.Names() {
		want, _ := shipped.File(name)
		if got, err := os.ReadFile(filepath.Join(b.dir, shipped.Dir, name)); err != nil || !bytes.Equal(got, want) {
			t.Errorf(".bulldog/%s is not as the program has it: %v", name, err)
		}
	}
	for _, gone := range []string{"talk.yaml", "tools/talk/old.yaml"} {
		if _, err := os.Stat(filepath.Join(b.dir, shipped.Dir, gone)); !os.IsNotExist(err) {
			t.Errorf(".bulldog/%s, which the program does not ship, is still there: %v", gone, err)
		}
	}
	for _, kept := range []string{IdentityFile, StoreFile} {
		if _, err := os.Stat(filepath.Join(b.dir, shipped.Dir, kept)); err != nil {
			t.Errorf(".bulldog/%s is not kept: %v", kept, err)
		}
	}
}

func TestServerIdentityNotFromName(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	if err := b.serve().stop(); err != nil {
		t.Fatal(err)
	}
	was := b.identity()
	text, _ := os.ReadFile(filepath.Join(b.dir, office.OfficeFile))
	b.write(map[string]string{office.OfficeFile: strings.Replace(string(text), "name: The house", "name: The flat", 1)})
	s := b.serve()
	s.answered(s.chat("kit", "", "is the kettle on?"))
	if b.identity() != was {
		t.Errorf("the identity is %s after a rename, and was %s", b.identity(), was)
	}
	if runs := b.listed(was, map[string]string{"request": "1"}); len(runs) != 1 {
		t.Errorf("the request's run under the identity: %v", kinds(runs))
	}
	// Two folders given the example, and an office whose name is no
	// letters, have identities of their own.
	one, _, _ := readied(t, t.TempDir(), Readying{Make: true})
	two, _, _ := readied(t, t.TempDir(), Readying{Make: true})
	odd := testkit.Folder(t, map[string]string{office.OfficeFile: "name: \"!!!\"\n"})
	three, _, err := readied(t, odd, Readying{})
	if err != nil {
		t.Fatal(err)
	}
	if one.ID == two.ID || len(one.ID) != 32 || len(three.ID) != 32 {
		t.Errorf("the identities are %q, %q and %q", one.ID, two.ID, three.ID)
	}
}

func TestServerIdentityOutlivesStore(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	s.answered(s.chat("kit", "", "is the kettle on?"))
	if err := s.stop(); err != nil {
		t.Fatal(err)
	}
	was := b.identity()
	for _, name := range []string{StoreFile, StoreFile + "-wal", StoreFile + "-shm"} {
		os.Remove(filepath.Join(b.dir, shipped.Dir, name))
	}
	b.serve().areasOf()
	if b.identity() != was {
		t.Errorf("a store made again came with the identity %s, not %s", b.identity(), was)
	}
}
