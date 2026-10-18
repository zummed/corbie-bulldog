package office_test

import (
	"testing"
)

func TestRoutines(t *testing.T) {
	t.Parallel()
	o := reads(t, house(t))
	r := o.TopTeam().Routines
	if len(r) != 2 || r[0].At != "mon 09:00" || r[0].To != "steward" || r[0].Say != "Walk round: what is still open?" || r[1].To != "bins" {
		t.Errorf("the house's routines are %#v", r)
	}
	for routine, said := range map[string]string{
		`{ at: "mon thu 09:00", to: steward, say: Hello }`: "mon thu 09:00",
		`{ at: "25:00", to: steward, say: Hello }`:         "25:00",
		`{ at: "09:00", to: sam, say: Hello }`:             "sam is a person",
		`{ at: "09:00", to: gardener, say: Hello }`:        "gardener is not a worker",
		`{ at: "09:00", to: steward }`:                     "no say",
	} {
		files := replace(t, house(t), "office.yaml", "routines:\n", "routines:\n  - "+routine+"\n")
		p := says(t, refused(t, files), "office.yaml", "routines[0]", said)
		if p.Line == 0 {
			t.Errorf("%s gives no line", routine)
		}
	}
}

func TestARoutinesTimeIsOneCorbieTakes(t *testing.T) {
	t.Parallel()
	// 18:60 is written as a time of day, and Corbie would refuse it as a
	// timer's time when the routine's clock waits for it.
	files := replace(t, house(t), "office.yaml", `at: "18:00"`, `at: "18:60"`)
	p := says(t, refused(t, files), "office.yaml", "routines[1]", "not a time of day")
	if p.Team != "office" {
		t.Errorf("the problem is on %q", p.Team)
	}
	if c := reads(t, house(t)).TopTeam().Routines[0]; c.Due != "mon 09:00 Europe/London" {
		t.Errorf("the Monday routine is due %q", c.Due)
	}
}

func TestOfficeRoutineTimeInOfficeZone(t *testing.T) {
	t.Parallel()
	if c := reads(t, house(t)).Teams["garden"].Routines[0]; c.Due != "fri 16:00 Europe/London" {
		t.Errorf("the garden's routine is due %q", c.Due)
	}
	utc := reads(t, replace(t, house(t), "office.yaml", "zone: Europe/London\n", ""))
	if c := utc.TopTeam().Routines[0]; c.Due != "mon 09:00 UTC" || utc.Zone.String() != "UTC" {
		t.Errorf("with no zone the routine is due %q", c.Due)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", `at: "mon 09:00"`, `at: "mon 09:00 America/New_York"`)), "office.yaml", "names a zone", "Europe/London")
}

func TestTheOfficesZone(t *testing.T) {
	t.Parallel()
	if o := reads(t, house(t)); o.Zone.String() != "Europe/London" {
		t.Errorf("the zone is %v", o.Zone)
	}
	says(t, refused(t, replace(t, house(t), "office.yaml", "zone: Europe/London", "zone: Somewhere/Else")), "office.yaml", "zone", "Somewhere/Else")
	says(t, refused(t, replace(t, house(t), "office.yaml", "zone: Europe/London", "zone: Local")), "office.yaml", "zone")
}
