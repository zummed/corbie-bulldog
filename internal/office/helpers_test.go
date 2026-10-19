package office_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// house and work give the files of the two offices the office page shows.
func house(t *testing.T) map[string]string { return testkit.Files(t, examples.House()) }
func work(t *testing.T) map[string]string  { return testkit.Files(t, examples.Work()) }

// own gives an office's files with a bridge that runs the folder Bulldog
// ships copied into bridges/<name>, the office's own, and its entry no
// longer naming the shipped one, as a person copies a bridge to change it.
func own(t *testing.T, files map[string]string, name string) map[string]string {
	t.Helper()
	tree, _, ok := shipped.Bridge(name)
	if !ok {
		t.Fatalf("Bulldog ships no bridge %s", name)
	}
	folder := testkit.Files(t, tree)
	delete(folder, shipped.Settings)
	return standin.Instead(files, name, folder)
}

// read writes an office's files into a folder of the test's own and reads
// it, giving the office, if it reads, and every problem found.
func read(t *testing.T, files map[string]string) (*office.Office, office.Problems) {
	t.Helper()
	o, err := office.Read(testkit.Folder(t, files))
	var problems office.Problems
	if errors.As(err, &problems) {
		return nil, problems
	}
	if err != nil {
		t.Fatal(err)
	}
	return o, o.Problems
}

// reads reads an office that must read with no problem at all.
func reads(t *testing.T, files map[string]string) *office.Office {
	t.Helper()
	o, problems := read(t, files)
	if o == nil || len(problems) > 0 {
		t.Fatalf("the office does not read well:\n%v", problems)
	}
	return o
}

// refused reads an office whose reading must be refused, and gives its
// problems.
func refused(t *testing.T, files map[string]string) office.Problems {
	t.Helper()
	o, problems := read(t, files)
	if o != nil {
		t.Fatalf("the office reads, with problems:\n%v", problems)
	}
	return problems
}

// saying gives the problems whose words hold every one of the words.
func saying(problems office.Problems, words ...string) office.Problems {
	var found office.Problems
	for _, p := range problems {
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(p.String(), w) }) {
			found = append(found, p)
		}
	}
	return found
}

// says fails unless one of the problems holds every one of the words.
func says(t *testing.T, problems office.Problems, words ...string) office.Problem {
	t.Helper()
	found := saying(problems, words...)
	if len(found) == 0 {
		t.Fatalf("no problem says %q; the problems are:\n%v", words, problems)
	}
	return found[0]
}

// with gives files with some changed: a file given as "" is removed.
func with(files map[string]string, changes map[string]string) map[string]string {
	out := map[string]string{}
	for name, contents := range files {
		out[name] = contents
	}
	for name, contents := range changes {
		if contents == "" {
			delete(out, name)
		} else {
			out[name] = contents
		}
	}
	return out
}

// replace gives files with one file's text changed: old must be in it.
func replace(t *testing.T, files map[string]string, file, old, new string) map[string]string {
	t.Helper()
	if !strings.Contains(files[file], old) {
		t.Fatalf("%s does not hold %q", file, old)
	}
	return with(files, map[string]string{file: strings.Replace(files[file], old, new, 1)})
}
