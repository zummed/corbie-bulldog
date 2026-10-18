package office_test

import "testing"

func TestNamesAreSmallLettersFiguresAndDashes(t *testing.T) {
	t.Parallel()
	says(t, refused(t, replace(t, house(t), "office.yaml", "  sam: {", "  Sam: {")), "office.yaml", `"Sam"`)
	says(t, refused(t, with(house(t), map[string]string{"Fixer.yaml": house(t)["fixer.yaml"]})), "Fixer.yaml", `"Fixer"`)
	files := map[string]string{}
	for name, contents := range house(t) {
		if len(name) > 7 && name[:7] == "garden/" {
			name = "the garden/" + name[7:]
		}
		files[name] = contents
	}
	says(t, refused(t, files), "the garden/team.yaml", `"the garden"`)
	says(t, refused(t, replace(t, house(t), "office.yaml", "  telegram:\n", "  2way:\n")), "office.yaml", `"2way"`)
}

func TestOfficeNamesUsedOnce(t *testing.T) {
	t.Parallel()
	says(t, refused(t, with(house(t), map[string]string{"garden/steward.yaml": house(t)["steward.yaml"]})), "steward", "used twice")
	says(t, refused(t, with(house(t), map[string]string{"garden/sam.yaml": "flow: sam\nstages: { a: { job: ../tools/order.js } }\n"})), "sam", "used twice")
	says(t, refused(t, with(house(t), map[string]string{"garden/garden.yaml": "flow: garden\nstages: { a: { job: ./x.js } }\n", "garden/x.js": "return 1"})), "garden", "used twice")
	// Two teams, one within the other or side by side.
	for file, said := range map[string]string{"garden/garden/team.yaml": "the team garden/garden", "beds/team.yaml": "the team garden/beds"} {
		files := with(house(t), map[string]string{"garden/beds/team.yaml": "takes: sam\npeople: [sam]\n", file: "takes: sam\npeople: [sam]\n"})
		problems := refused(t, files)
		says(t, problems, "used twice", said)
		if got := saying(problems, "not a team's folder"); len(got) > 0 {
			t.Errorf("a team whose name is used twice is said to be in no team's folder: %v", got)
		}
	}
	// A team within may give a model's name again.
	reads(t, house(t))
}

func TestOfficeReservedNamesRefused(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"office", "team", "notes", "bridges", "chat", "routine", "bulldog", "guest"} {
		says(t, refused(t, replace(t, house(t), "office.yaml", "  kit: {", "  "+name+": {")), "office.yaml", `"`+name+`"`, "keeps for itself")
	}
	says(t, refused(t, with(house(t), map[string]string{"guest.yaml": "flow: guest\nstages: { a: { job: ./tools/order.js } }\n"})), "guest.yaml", "keeps for itself")
	says(t, refused(t, with(house(t), map[string]string{"chat/team.yaml": "takes: nobody\n"})), "chat/team.yaml", "keeps for itself")
	for _, name := range []string{"chat", "name", "title"} {
		says(t, refused(t, replace(t, house(t), "office.yaml", "  telegram:\n", "  "+name+":\n")), "office.yaml", name)
	}
}
