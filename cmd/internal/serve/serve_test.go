package serve

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// A written is what a Bulldog writes, for a test to read.
type written struct {
	mu   sync.Mutex
	text strings.Builder
}

func (l *written) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(p)
}

// said gives what was written since it was asked last.
func (l *written) said() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	said := l.text.String()
	l.text.Reset()
	return said
}

func TestOfficeReadAgainWhenAFileChanges(t *testing.T) {
	t.Parallel()
	files := testkit.Files(t, examples.House)
	dir := testkit.Dir(t, files)
	first, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	said := &written{}
	b := New(testkit.NewCorbie(t).Client, first, []byte("the house's secret"), "http://127.0.0.1:8484", said)
	write := func(name, text string) {
		t.Helper()
		testkit.Write(t, dir, map[string]string{name: text})
		b.reread()
	}
	takes := func(r *requests.Reading, team string) bool { return r.Machines[team].Draft != nil }
	problems := func(r *requests.Reading) []string {
		var files []string
		for _, p := range r.Office.Problems {
			files = append(files, p.File)
		}
		return files
	}

	// With nothing changed, the office is not read again.
	if b.reread(); b.Engine.Reading() != first || said.said() != "" {
		t.Fatal("the office was read again with nothing changed")
	}

	// A problem of a team file leaves both teams as they were, and shows
	// the problem.
	write("garden/team.yaml", files["garden/team.yaml"]+"budget: 40\n")
	r := b.Engine.Reading()
	if !takes(r, "office") || !takes(r, "garden") || !slices.Equal(problems(r), []string{"garden/team.yaml"}) || r.Office.Team("garden").Name != "The garden" {
		t.Fatalf("a refused reading: the office takes requests %v, the garden %v, problems %v", takes(r, "office"), takes(r, "garden"), r.Office.Problems)
	}
	if text := said.said(); !strings.Contains(text, "refused, so the last good reading stays in force") || !strings.Contains(text, "garden/team.yaml:9: budget") {
		t.Fatalf("a refused reading said %q", text)
	}

	// Mended, the office is read again.
	write("garden/team.yaml", files["garden/team.yaml"])
	if r := b.Engine.Reading(); !takes(r, "office") || !takes(r, "garden") || len(r.Office.Problems) > 0 || !strings.Contains(said.said(), "The office was read again.") {
		t.Fatalf("the mended office has problems %v", r.Office.Problems)
	}

	// A broken worker stops only its team: the garden still takes
	// requests.
	write("fixer.yaml", "agent: fixer\nmodel: [usual\n")
	r = b.Engine.Reading()
	if takes(r, "office") || !takes(r, "garden") || !slices.Contains(problems(r), "fixer.yaml") {
		t.Fatalf("with fixer.yaml broken: the office takes requests %v, the garden %v, problems %v", takes(r, "office"), takes(r, "garden"), r.Office.Problems)
	}
	write("fixer.yaml", files["fixer.yaml"])

	// The office's id is never changed while it is served.
	id := b.Engine.Reading().Office.ID
	if err := os.WriteFile(filepath.Join(dir, ".bulldog", "id"), []byte("0123456789abcdef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write("office.yaml", files["office.yaml"]+"# changed\n")
	if r := b.Engine.Reading(); r.Office.ID != id || !takes(r, "office") || !slices.Equal(problems(r), []string{".bulldog/id"}) {
		t.Fatalf("with its id changed, the office is served as %s, with problems %v", r.Office.ID, r.Office.Problems)
	}
}
