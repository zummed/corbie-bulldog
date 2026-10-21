package examples_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestMain(m *testing.M) { testkit.Main(m) }

func read(t *testing.T, files map[string]string) *office.Office {
	t.Helper()
	o, err := office.Read(testkit.Folder(t, files))
	var problems office.Problems
	if errors.As(err, &problems) || err == nil && len(o.Problems) > 0 {
		t.Fatalf("the office does not read well:\n%v%v", problems, o.Problems)
	}
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestTheOfficesReadWithNoProblem(t *testing.T) {
	t.Parallel()
	house := read(t, testkit.Files(t, examples.House()))
	if house.Name() != "The house" || len(house.Teams) != 2 || len(house.Workers) != 8 || len(house.People) != 2 || house.Bridges["telegram"] == nil {
		t.Errorf("the house reads as %d teams, %d workers, %d people", len(house.Teams), len(house.Workers), len(house.People))
	}
	work := read(t, testkit.Files(t, examples.Work()))
	if house.SignIns["google"] == nil || len(house.People["sam"].Accounts) != 1 || !house.People["sam"].Password {
		t.Errorf("the house signs in by %v", house.SignInOrder)
	}
	if work.Name() != "Platform support" || len(work.Teams) != 4 || len(work.People) != 5 || len(work.Bridges) != 2 || work.SignIns["entra"] == nil || len(work.People["dana"].Accounts) != 1 {
		t.Errorf("the work office reads as %d teams and %d people", len(work.Teams), len(work.People))
	}
}

func TestTheNewHouseHasNoBridge(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for name, contents := range examples.NewHouse() {
		if strings.HasPrefix(name, "bridges/") {
			t.Errorf("the new house holds %s", name)
		}
		files[name] = string(contents)
	}
	if strings.Contains(files["office.yaml"], "bridges") || strings.Contains(files["office.yaml"], "telegram") || strings.Contains(files["office.yaml"], "google") || strings.Contains(files["office.yaml"], "signin") {
		t.Errorf("the new house's office.yaml is\n%s", files["office.yaml"])
	}
	o := read(t, files)
	if len(o.Bridges) != 0 || len(o.SignIns) != 0 || len(o.People["sam"].Reached) != 0 || len(o.People["sam"].Accounts) != 0 || !o.People["sam"].Password || len(o.Workers) != 8 {
		t.Errorf("the new house has %d bridges and %d sign-in methods", len(o.Bridges), len(o.SignIns))
	}
}
