package examples_test

import (
	"io/fs"
	"os"
	"slices"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/shipped"

	_ "github.com/zummed/corbie/agent"
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/statechart"
)

// Each office reads with no problem, and every team's request machine is
// made and taken by Corbie's parser.
func TestExamplesMakeEveryRequestMachine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		files fs.FS
		teams []string
	}{
		{"house", examples.House, []string{"office", "garden"}},
		{"work", examples.Work, []string{"office", "payments", "platform", "support"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir() // Bulldog writes the office's id into the directory it reads
			if err := os.CopyFS(dir, c.files); err != nil {
				t.Fatal(err)
			}
			o, err := office.Read(dir, shipped.Files)
			if err != nil {
				t.Fatalf("the office is refused:\n%v", err)
			}
			machines := requests.Make(o)
			if len(o.Problems) > 0 {
				t.Fatalf("the office has problems:\n%v", office.Problems(o.Problems))
			}
			var teams []string
			for _, team := range o.Teams {
				teams = append(teams, team.ID)
				if machines[team.ID].Draft == nil {
					t.Errorf("%s has no request machine", team.ID)
				}
			}
			if !slices.Equal(teams, c.teams) {
				t.Errorf("the teams are %v, not %v", teams, c.teams)
			}
		})
	}
}
