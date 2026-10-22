// Package requests runs each thing asked of a team as one run of the
// team's request machine. It makes the machines, starts and follows the
// runs, and answers the tickets they wait on.
package requests

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/agent"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/permission"
)

// A Machine is a team's request machine: one Corbie flow named after the
// team, with the stages heed, work and check and its decisions written in
// place, as if it stood at the office's top directory.
type Machine struct {
	// YAML is the machine as it is sent to Corbie, for the team's page.
	YAML string
	// Draft is the machine filled out and checked by Corbie's parser, to
	// start a request with; nil when the team has a problem, which stops
	// it.
	Draft *machine.Draft
	// Allow is the list every run Bulldog starts for the team is started
	// with: everything, but kept values only under the team's own path.
	Allow *permission.Layer
}

// Make makes the request machine of each team of an office, by the team's
// ID, and checks it with Corbie's parser. Each problem found, of a decider
// the team file names or of what Corbie's parser refuses, is added to the
// office with Stop, and stops the team. A team stopped already is not
// checked again. Make is called once for a reading, before the office is
// shared.
func Make(o *office.Office) map[string]*Machine {
	made := map[string]*Machine{}
	for _, t := range o.Teams {
		m := &Machine{Allow: &permission.Layer{Allow: o.Allow(t.ID)}}
		made[t.ID] = m
		src, err := encode(write(o, t))
		if err != nil {
			o.Stop(t, office.Problem{File: t.File, Text: "its request machine cannot be written: " + err.Error()})
			continue
		}
		m.YAML = string(src)
		if len(t.Problems) > 0 {
			continue
		}
		deciders := deciderProblems(o, t)
		draft, err := machine.Parse(src, o.Load)
		found := refused(t, err)
		if len(deciders) > 0 {
			// Corbie's parser refuses the ask Bulldog writes for a decider
			// that holds a job, which deciders already says, naming it.
			found = slices.DeleteFunc(found, func(p office.Problem) bool { return p.File == t.File && strings.HasPrefix(p.Text, "ask: ") })
		}
		o.Stop(t, append(deciders, found...)...)
		if len(t.Problems) == 0 {
			m.Draft = draft
		}
	}
	return made
}

// refused gives the problems Corbie's parser found in a team's request
// machine, each of the file it is in: a worker's, or the team file's, from
// which the machine is written.
func refused(t *office.Team, err error) []office.Problem {
	if err == nil {
		return nil
	}
	var found machine.Problems
	if !errors.As(err, &found) {
		return []office.Problem{{File: t.File, Text: err.Error()}}
	}
	problems := make([]office.Problem, len(found))
	for i, p := range found {
		problems[i] = office.Problem{File: p.File, Text: p.Text}
		if p.File == "" {
			problems[i].File = t.File
		}
		if p.Place != "" {
			problems[i].Text = p.Place + ": " + p.Text
		}
	}
	return problems
}

// deciderProblems gives the problems of the deciders a team file names: a
// worker or a rule that holds a job for approval or lists ask_about, and a
// worker that does not declare its reply as a decider's. Each is a problem
// of the team file.
func deciderProblems(o *office.Office, t *office.Team) []office.Problem {
	var problems []office.Problem
	for _, d := range []struct {
		name string
		list []office.Decider
	}{{"heed", t.Heed}, {"check", t.Check}, {"decide", t.Decide}} {
		for _, decider := range d.list {
			var file, name string
			switch {
			case decider.Worker != "":
				file, name = o.Worker(decider.Worker).File, decider.Worker
			case strings.HasSuffix(decider.Rule, ".yaml"):
				file, name = strings.TrimPrefix(decider.Rule, "./"), decider.Rule
			default:
				continue // a script or a person
			}
			draft := alone(o, file)
			if draft == nil {
				continue // Corbie's parser says what is wrong with it, in the request machine
			}
			if holds(draft) {
				problems = append(problems, office.Problem{File: t.File, Text: fmt.Sprintf(
					"%s: %s holds a job for approval, or lists ask_about; a decider never asks for approval itself", d.name, name)})
			}
			if decider.Worker != "" && !declaresReply(draft) {
				problems = append(problems, office.Problem{File: t.File, Text: fmt.Sprintf(
					`%s: %s does not declare reply: { says: string, note: "string?" }, which a worker named as a decider declares`, d.name, name)})
			}
		}
	}
	return problems
}

// alone fills out a definition of the office by itself, its paths read
// from its own directory. It gives nil when Corbie's parser refuses it.
func alone(o *office.Office, file string) *machine.Draft {
	src, err := o.Load(file)
	if err != nil {
		return nil
	}
	draft, err := machine.ParseMade(src, func(name string) ([]byte, error) { return o.Load(path.Join(path.Dir(file), name)) })
	if err != nil {
		return nil
	}
	return draft
}

// holds says whether a definition, or one below it, holds a job for
// approval or lists ask_about.
func holds(d *machine.Draft) bool {
	if _, approves := machine.Check(d.Definition); approves {
		return true
	}
	for _, child := range d.Children {
		if holds(child) {
			return true
		}
	}
	return false
}

// declaresReply says whether a definition is an agent that declares a
// reply with says, as a worker named as a decider does.
func declaresReply(d *machine.Draft) bool {
	var a agent.Agent
	if d.Definition.Type != "agent" || json.Unmarshal(d.Definition.Body, &a) != nil {
		return false
	}
	properties, _ := a.Reply["properties"].(map[string]any)
	_, ok := properties["says"]
	return ok
}
