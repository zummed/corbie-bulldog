package office_test

import (
	"slices"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
)

func TestEveryPersonIsWrittenOnce(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if len(o.People) != 5 || !slices.Equal(o.Everyone, []string{"al", "dana", "lee", "priya", "raj"}) {
		t.Errorf("the people are %v", o.Everyone)
	}
	al := o.People["al"]
	if al.Called != "Al" || al.Title != "Runs the office." || !slices.Equal(al.Reached, []office.Identity{{"slack", "U0AL0000"}, {"gitlab", "1041"}}) {
		t.Errorf("al is %#v", al)
	}
	o = reads(t, replace(t, work(t), "office.yaml", `{ name: Al, title: Runs the office., slack: U0AL0000, gitlab: "1041",`, `{ name: Al, gitlab: "1041", slack: U0AL0000,`))
	if got := o.People["al"].Reached; got[0].Bridge != "gitlab" {
		t.Errorf("al's bridges are %v", got)
	}
	o = reads(t, replace(t, work(t), "office.yaml", "{ name: Raj, title: Backend.,", "{ title: Backend.,"))
	if got := o.People["raj"].Called; got != "raj" {
		t.Errorf("raj with no name is called %q", got)
	}
	says(t, refused(t, replace(t, house(t), "garden/team.yaml", "lead: gardener", "lead: gardener\neveryone: { pat: {} }")), "garden/team.yaml", "everyone")
}

func TestAPersonsKeysNameTheOfficesBridges(t *testing.T) {
	t.Parallel()
	for entry, key := range map[string]string{
		`kit: { name: Kit, slack: U0KIT000 }`:        "slack",
		`kit: { name: Kit, gitlab: "1077" }`:         "gitlab",
		`kit: { name: Kit, email: kit@example.com }`: "email",
	} {
		says(t, refused(t, replace(t, house(t), "office.yaml", `kit: { name: Kit, title: Lives here., telegram: "4472" }`, entry)), "office.yaml", "kit", key, "no bridge")
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", `sam: { name: Sam, title: Owner. Decides money and the building., telegram: "4471", google: sam@example.com, password: true }`, `sam: { name: Sam, email: sam@example.com }`)), "office.yaml", "sam", "email")
}

func TestAPersonInNoTeamIsAPersonAllTheSame(t *testing.T) {
	t.Parallel()
	o := reads(t, replace(t, work(t), "payments/team.yaml", "everything: [lee, priya]", "everything: [raj, lee, priya]"))
	if o.People["raj"] == nil || len(o.TeamsOf("raj")) != 0 || o.Everything("payments")[0].Kind != office.IsPerson {
		t.Error("raj is not a person who may decide")
	}
}

func TestOfficeIdentityNamesOnePerson(t *testing.T) {
	t.Parallel()
	says(t, refused(t, replace(t, work(t), "office.yaml", "slack: U0RAJ000", "slack: U0LEE000")), "office.yaml", "lee", "raj", "U0LEE000")
	// Compared exactly as written: another case is another identity.
	o := reads(t, replace(t, work(t), "office.yaml", "slack: U0RAJ000", "slack: u0lee000"))
	if o.Person("slack", "U0LEE000").Name != "lee" || o.Person("slack", "u0lee000").Name != "raj" {
		t.Error("identities are not compared exactly as written")
	}
	// The same id on two bridges is two identities.
	reads(t, replace(t, work(t), "office.yaml", `gitlab: "1187"`, `gitlab: U0RAJ000`))
}

func TestOfficePersonReachedOnlyByEntry(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	if p := o.Person("slack", "U0DANA00"); p == nil || p.Name != "dana" {
		t.Errorf("U0DANA00 on slack is %v", p)
	}
	for _, guest := range []struct{ bridge, id string }{{"slack", "u0dana00"}, {"slack", "U0GUEST0"}, {"gitlab", "U0DANA00"}, {"chat", "dana"}, {"slack", "dana"}} {
		if p := o.Person(guest.bridge, guest.id); p != nil {
			t.Errorf("%s on %s is %s", guest.id, guest.bridge, p.Name)
		}
	}
	// Only a new reading of the files changes who someone is.
	changed := reads(t, replace(t, work(t), "office.yaml", "slack: U0DANA00", "slack: U0DANA01"))
	if changed.Person("slack", "U0DANA00") != nil || changed.Person("slack", "U0DANA01").Name != "dana" || o.Person("slack", "U0DANA00").Name != "dana" {
		t.Error("a person is reached by something but their entry")
	}
}

func TestOfficeReachedOutward(t *testing.T) {
	t.Parallel()
	o := reads(t, work(t))
	for team, want := range map[string][]string{office.Top: {"al"}, "payments": {"lee", "priya"}, "support": {"dana"}, "platform": {"al"}} {
		if got := o.Reached(team); !slices.Equal(got, want) {
			t.Errorf("%s's decisions reach %v", team, got)
		}
	}
	// A team's everything, where it writes one, comes before its people.
	o = reads(t, replace(t, work(t), "payments/team.yaml", "everything: [lee, priya]", "everything: [priya]"))
	if got := o.Reached("payments"); !slices.Equal(got, []string{"priya"}) {
		t.Errorf("payments' decisions reach %v", got)
	}
	// An everything of rules alone reaches nobody, and the walk goes on out.
	files := work(t)
	files["platform/team.yaml"] += "decide: { everything: [./a.js] }\n"
	files["platform/a.js"] = `return { says: "refer" }`
	if got := reads(t, files).Reached("platform"); !slices.Equal(got, []string{"al"}) {
		t.Errorf("platform's decisions, past its rule, reach %v", got)
	}
	// A person not in the office is not reached, as on a walk read before
	// they left.
	gone := &office.Office{
		Teams: map[string]*office.Team{
			office.Top: {Name: office.Top, People: []string{"al", "kit"}},
			"shed":     {Name: "shed", Within: office.Top, Decide: []office.List{{Key: "everything", Deciders: []office.Decider{{Kind: office.IsPerson, Name: "pat"}}}}},
		},
		People: map[string]*office.Person{"al": {Name: "al"}},
	}
	if got := gone.Reached("shed"); !slices.Equal(got, []string{"al"}) {
		t.Errorf("the shed's decisions reach %v", got)
	}
	if got := gone.Reached("nowhere"); got != nil {
		t.Errorf("a team not in the office has its decisions reach %v", got)
	}
}
