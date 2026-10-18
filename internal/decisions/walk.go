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

// A stage is one step of a walk: one rule or worker, or the people at the
// end of a list, asked all at once.
type stage struct {
	Kind   string
	Name   string
	People []string
	// Team and List are the list it is on.
	Team string
	List string
}

func (s stage) is(kind, name string) bool { return s.Kind == kind && s.Name == name }

// isPeople is the kind of a stage of people.
const isPeople = "people"

// walk gives whom a decision is asked of, in order, in the office as it is
// read: the list it is decided by, which for a call of a listed tool is
// its entry's when its team names none for the tool, then its team's
// everything, then the
// everything of each team around it out to the office, as far as the
// first people; a change, the walk it was put with. It ends at people, or
// with nobody where none is reached; why says why.
func walk(o *office.Office, dec *Decision) (stages []stage, why string) {
	if dec.Kind == OfChange {
		return listed(dec.Put.Walk, office.Top, "change", o), "nobody named on its walk is in the office"
	}
	t := o.Teams[dec.Team]
	if t == nil {
		return nil, "it is no longer in the office"
	}
	switch dec.Kind {
	case OfRelease:
		stages = listed(t.Release, t.Name, "release", o)
	case OfAction:
		keys := []string{dec.Job.key()}
		if dec.Job.Asked != nil && dec.Job.Asked.Entry != "" {
			keys = append(keys, dec.Job.Asked.Entry)
		}
		for _, key := range keys {
			if key == "everything" || key == "change" {
				continue
			}
			if ds, ok := o.List(t.Name, key); ok {
				stages = listed(ds, t.Name, key, o)
				break
			}
		}
	}
	if firstPeople(stages) == nil {
		stages = append(stages, outward(o, t.Name)...)
	}
	if firstPeople(stages) == nil {
		return stages, "nobody the lists name can be reached to decide it"
	}
	return stages, ""
}

// outward gives the stages of a team's everything, and of the everything
// of each team around it out to the office, as far as the first that
// names people.
func outward(o *office.Office, team string) []stage {
	var stages []stage
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
func firstPeople(stages []stage) []string {
	for _, s := range stages {
		if s.Kind == isPeople {
			return s.People
		}
	}
	return nil
}

// listed gives the stages of one list: each rule and worker, then its
// people together, those of the office alone.
func listed(ds []office.Decider, team, list string, o *office.Office) []stage {
	var stages []stage
	var people []string
	for _, d := range ds {
		switch {
		case d.Kind != office.IsPerson:
			stages = append(stages, stage{Kind: d.Kind, Name: d.Name, Team: team, List: list})
		case o.People[d.Name] != nil && !slices.Contains(people, d.Name):
			people = append(people, d.Name)
		}
	}
	if len(people) > 0 {
		stages = append(stages, stage{Kind: isPeople, People: people, Team: team, List: list})
	}
	return stages
}

// from gives where in a walk a decision goes on: at the rule or worker it
// waits on, where that is still on it; at its people, once it has reached
// people; or else after the last decider on it that has passed.
func from(stages []stage, dec *Decision) int {
	at := 0
	for i, s := range stages {
		if slices.ContainsFunc(dec.Passes, func(p Pass) bool { return s.is(p.Kind, p.Name) }) {
			at = i + 1
		}
	}
	switch {
	case dec.At == nil:
	case dec.At.Kind == isPeople:
		if i := slices.IndexFunc(stages, func(s stage) bool { return s.Kind == isPeople }); i >= 0 {
			return i
		}
	default:
		if i := slices.IndexFunc(stages, func(s stage) bool { return s.is(dec.At.Kind, dec.At.Name) }); i >= 0 {
			return i
		}
	}
	return at
}

// next gives the stage a decision is asked of next, skipping each that has
// passed, and false where the walk reaches nobody.
func next(stages []stage, dec *Decision) (stage, int, bool) {
	for i := from(stages, dec); i < len(stages); i++ {
		s := stages[i]
		if s.Kind != isPeople && slices.ContainsFunc(dec.Passes, func(p Pass) bool { return s.is(p.Kind, p.Name) }) {
			continue
		}
		return s, i, true
	}
	return stage{}, 0, false
}

// reach gives the people the walk comes to when every rule and worker on
// its way passes, who may decide it while it waits.
func reach(stages []stage, dec *Decision) []string { return firstPeople(stages[from(stages, dec):]) }

// ownWorker says whether a stage is the worker whose action a decision is,
// which never decides it.
func ownWorker(dec *Decision, s stage) bool {
	return s.Kind == office.IsWorker && dec.Whose.Kind == requests.IsWorker && dec.Whose.Name == s.Name
}

// Repeats is how Corbie names a job beyond its tool or step: one of
// several asked for in one step, a job asked again, and an attempt.
var Repeats = regexp.MustCompile(`(\.[0-9]+)?(#[0-9]+)?(@[0-9]+)?$`)

// key gives the list a held job is decided by: its tool's name, or its
// step's.
func (j *Job) key() string { return Repeats.ReplaceAllString(j.Name, "") }

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
