// Package testkit is what Bulldog's tests run on: a whole Corbie in the
// test's process with its store in memory, a stand-in model the test
// answers, a clock the test moves, and directories written from a map of
// files. Nothing in it reaches the network or needs a key.
package testkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/zummed/corbie/runner"

	// The machine types a definition may be, and the plugins its scripts
	// may call, in the test binary and in each job it runs.
	_ "github.com/zummed/corbie/agent"
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/plugin/bundled"
	_ "github.com/zummed/corbie/statechart"
)

// Main is the TestMain of a package whose tests run a Corbie: the runner
// runs each job in a process of the test binary.
func Main(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == runner.JobArg {
		runner.JobMain()
		return
	}
	os.Exit(m.Run())
}

// Dir makes a directory of the test's own, writes the files into it, and
// gives its path.
func Dir(t testing.TB, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	Write(t, dir, files)
	return dir
}

// Write writes each file under its slash-separated path in a directory,
// making the directories on the way.
func Write(t testing.TB, dir string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Files reads every file of a tree into a map by its path, for a test to
// change before it writes them with Dir.
func Files(t testing.TB, tree fs.FS) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := fs.WalkDir(tree, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		contents, err := fs.ReadFile(tree, name)
		files[name] = string(contents)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
