package decisions

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// A Stage is one step of a walk: one rule or worker, or the people at the
// end of a list, asked all at once.
type Stage struct {
	// Kind is office.IsRule, office.IsWorker or IsPeople; Name is the
	// rule's path in the office or the worker's name, and People the
	// names of a stage of people.
	Kind   string
	Name   string
	People []string
	// Team and List are the list it is on, and Advises whether the rule or
	// worker is marked there to advise: its yes goes on as a
	// recommendation.
	Team    string
	List    string
	Advises bool
}

func (s Stage) is(kind, name string) bool { return s.Kind == kind && s.Name == name }

// IsPeople is the kind of a stage of people.
const IsPeople = "people"

// walk gives whom a decision is asked of, in order, in the office as it is
// read: a held action, as Walk gives it for its job's key, or for a call
// of a listed tool its entry's when its team names no list for the tool;
// a disclosure, its team's disclosure list and on outward as Walk goes; a
// change, the walk it was put with. It ends at people, or with nobody
// where none is reached; why says why.
func walk(o *office.Office, dec *Decision) (stages []Stage, why string) {
	if dec.Kind == OfChange {
		return listed(dec.Put.Walk, office.Top, "change", o), "nobody named on its walk is in the office"
	}
	t := o.Teams[dec.Team]
	if t == nil {
		return nil, "it is no longer in the office"
	}
	switch dec.Kind {
	case OfDisclosure:
		stages = Disclosure(o, t.Name)
	case OfAction:
		var keys []string
		for _, key := range []string{dec.Job.key(), dec.Job.entry()} {
			if key != "" && key != "change" {
				keys = append(keys, key)
			}
		}
		stages = Walk(o, t.Name, keys...)
	}
	if firstPeople(stages) == nil {
		return stages, "nobody the lists name can be reached to decide it"
	}
	return stages, ""
}

// Walked gives whom a decision is asked of, in order, in the office as it
// is read now, as its walk asks them.
func (dec *Decision) Walked(o *office.Office) []Stage {
	stages, _ := walk(o, dec)
	return stages
}

// Args gives the arguments a held action would be given, nil for any
// other decision.
func (dec *Decision) Args() json.RawMessage {
	if dec.Kind != OfAction || dec.Job == nil {
		return nil
	}
	return dec.Job.args()
}

// Walk gives whom a team's decision is asked of, in order, in the office
// as it is read, with no decision needed: the team's list of the first
// key it writes one of, a held job's name or a listed tool's entry, or
// change for a change of the top team; then its everything, and the
// everything of each team around it out to the office, as far as the
// first that names people. Of a team not in the office it gives nothing.
func Walk(o *office.Office, team string, keys ...string) []Stage {
	if o.Teams[team] == nil {
		return nil
	}
	var stages []Stage
	for _, key := range keys {
		if key == "everything" {
			continue
		}
		if ds, ok := o.List(team, key); ok {
			stages = listed(ds, team, key, o)
			break
		}
	}
	return onward(o, team, stages)
}

// Disclosure gives whom a team's disclosure is asked of, in order, in the
// office as it is read, with no decision needed: its disclosure list, then
// on outward as Walk goes. Of a team not in the office it gives nothing.
func Disclosure(o *office.Office, team string) []Stage {
	t := o.Teams[team]
	if t == nil {
		return nil
	}
	return onward(o, team, listed(t.Disclosure, team, "disclosure", o))
}

// onward gives the stages of a team's list and, where they reach no
// people, those of its everything and of the everything of each team
// around it out to the office, as far as the first that names people.
func onward(o *office.Office, team string, stages []Stage) []Stage {
	if firstPeople(stages) != nil {
		return stages
	}
	for t := o.Teams[team]; t != nil; t = o.Teams[t.Within] {
		got := listed(o.Everything(t.Name), t.Name, "everything", o)
		stages = append(stages, got...)
		if firstPeople(got) != nil {
			break
		}
	}
	return stages
}

// firstPeople gives the people of the first stage of people.
func firstPeople(stages []Stage) []string {
	for _, s := range stages {
		if s.Kind == IsPeople {
			return s.People
		}
	}
	return nil
}

// listed gives the stages of one list: each rule and worker, then its
// people together, those of the office alone.
func listed(ds []office.Decider, team, list string, o *office.Office) []Stage {
	var stages []Stage
	var people []string
	for _, d := range ds {
		switch {
		case d.Kind != office.IsPerson:
			stages = append(stages, Stage{Kind: d.Kind, Name: d.Name, Team: team, List: list, Advises: d.Advises})
		case o.People[d.Name] != nil && !slices.Contains(people, d.Name):
			people = append(people, d.Name)
		}
	}
	if len(people) > 0 {
		stages = append(stages, Stage{Kind: IsPeople, People: people, Team: team, List: list})
	}
	return stages
}

// from gives where in a walk a decision goes on: at the rule or worker it
// waits on, where that is still on it; at its people, once it has reached
// people; or else after the last decider on it that has referred it.
func from(stages []Stage, dec *Decision) int {
	at := 0
	for i, s := range stages {
		if slices.ContainsFunc(dec.Referrals, func(w Word) bool { return s.is(w.Kind, w.Name) }) {
			at = i + 1
		}
	}
	switch {
	case dec.At == nil:
	case dec.At.Kind == IsPeople:
		if i := slices.IndexFunc(stages, func(s Stage) bool { return s.Kind == IsPeople }); i >= 0 {
			return i
		}
	default:
		if i := slices.IndexFunc(stages, func(s Stage) bool { return s.is(dec.At.Kind, dec.At.Name) }); i >= 0 {
			return i
		}
	}
	return at
}

// next gives the stage a decision is asked of next, skipping each that has
// referred it, and false where the walk reaches nobody.
func next(stages []Stage, dec *Decision) (Stage, int, bool) {
	for i := from(stages, dec); i < len(stages); i++ {
		s := stages[i]
		if s.Kind != IsPeople && slices.ContainsFunc(dec.Referrals, func(w Word) bool { return s.is(w.Kind, w.Name) }) {
			continue
		}
		return s, i, true
	}
	return Stage{}, 0, false
}

// reach gives the people the walk comes to when every rule and worker on
// its way refers it, who may decide it while it waits.
func reach(stages []Stage, dec *Decision) []string { return firstPeople(stages[from(stages, dec):]) }

// ownWorker says whether a stage is the worker whose action a decision is,
// which never decides it.
func ownWorker(dec *Decision, s Stage) bool {
	return s.Kind == office.IsWorker && dec.Whose.Kind == requests.IsWorker && dec.Whose.Name == s.Name
}

// Repeats is how Corbie names a job beyond its tool or step: one of
// several asked for in one step, a job asked again, and an attempt.
var Repeats = regexp.MustCompile(`(\.[0-9]+)?(#[0-9]+)?(@[0-9]+)?$`)

// key gives the list a held job is decided by: its tool's name, or its
// step's.
func (j *Job) key() string { return Repeats.ReplaceAllString(j.Name, "") }

// entry gives the list: entry a held call of a listed tool names, or "".
func (j *Job) entry() string {
	if j.Asked == nil {
		return ""
	}
	return j.Asked.Entry
}

// args gives the arguments a held job would be given: what a model's call
// shows, or else the job's input.
func (j *Job) args() json.RawMessage {
	if j.Asked != nil && len(j.Asked.Args) > 0 {
		return j.Asked.Args
	}
	return j.Input
}

// does gives the action as its tool describes itself, or its name.
func (j *Job) does() string {
	if j.Asked != nil && j.Asked.Does != "" {
		return j.Asked.Does
	}
	return j.key()
}

// calledOf gives what a decider is called in a sentence: a person by
// their name, a rule by its file's name, a worker by its own.
func calledOf(o *office.Office, kind, name string) string {
	switch kind {
	case office.IsPerson:
		if p := o.People[name]; p != nil {
			return p.Called
		}
	case office.IsRule:
		return path.Base(name)
	case requests.IsBulldog:
		return "Bulldog"
	}
	return name
}

// called gives what a team is called, or its name where it has gone.
func called(o *office.Office, team string) string {
	if t := o.Teams[team]; t != nil {
		return t.Called
	}
	return team
}

// inSentence gives what a team is called as it is written inside a
// sentence: "The house" as "the house".
func inSentence(calledAs string) string {
	if rest, ok := strings.CutPrefix(calledAs, "The "); ok {
		return "the " + rest
	}
	return calledAs
}
