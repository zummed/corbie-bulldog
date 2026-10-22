package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/agent"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/statechart"
)

// me gives who is looking, and the office.
func (s *Screens) me(r *http.Request, me string) (any, error) {
	o := s.engine.Reading().Office
	return map[string]any{
		"person": map[string]any{"id": me, "name": personName(o, me), "teams": teamsOf(o, me)},
		"office": map[string]any{"id": o.ID, "name": o.Name, "zone": o.Zone.String()},
	}, nil
}

// teamsOf gives the teams a person is of.
func teamsOf(o *office.Office, person string) []string {
	teams := []string{}
	for _, t := range o.Teams {
		if slices.Contains(t.People, person) {
			teams = append(teams, t.ID)
		}
	}
	return teams
}

// A room is a team drawn on the office's page, with the teams within it.
type room struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	About    string   `json:"about"`
	People   []string `json:"people"`
	Lead     lead     `json:"lead"`
	Teams    []room   `json:"teams"`
	Problems int      `json:"problems"`
}

type lead struct {
	Kind string `json:"kind"` // worker or person
	Name string `json:"name"`
}

func leadOf(t *office.Team) lead {
	if t.Lead.Person != "" {
		return lead{Kind: "person", Name: t.Lead.Person}
	}
	return lead{Kind: "worker", Name: t.Lead.Worker}
}

// roomOf gives a team as a room, with the teams within it.
func roomOf(o *office.Office, t *office.Team) room {
	r := room{ID: t.ID, Name: t.Name, About: t.About, People: list(t.People), Lead: leadOf(t), Teams: []room{}, Problems: len(t.Problems)}
	for _, within := range o.Teams {
		if within.Within == t.ID {
			r.Teams = append(r.Teams, roomOf(o, within))
		}
	}
	return r
}

// office gives the office: its teams as rooms, its people, its bridges
// with how each in run stands, and its problems.
func (s *Screens) office(r *http.Request, me string) (any, error) {
	o := s.engine.Reading().Office
	people := []map[string]any{}
	for _, p := range o.People {
		bridges := []string{}
		for _, b := range o.Bridges {
			if p.Identities[b.Name] != "" {
				bridges = append(bridges, b.Name)
			}
		}
		people = append(people, map[string]any{"id": p.ID, "name": p.Name, "teams": teamsOf(o, p.ID), "bridges": bridges})
	}
	bridged := []map[string]any{}
	for _, b := range o.Bridges {
		places := []map[string]any{}
		for _, p := range b.Places {
			places = append(places, map[string]any{"place": p.Name, "team": p.Team, "guests": p.Guests})
		}
		bridged = append(bridged, map[string]any{"name": b.Name, "from": or(b.From, "./"+b.Dir), "places": places, "in": s.bridges.State(b.Name)})
	}
	return map[string]any{"id": o.ID, "name": o.Name, "about": o.Teams[0].About, "zone": o.Zone.String(),
		"problems": list(o.Problems), "team": roomOf(o, o.Teams[0]), "people": people, "bridges": bridged}, nil
}

// A decider as a team file writes it.
type written struct {
	Kind string `json:"kind"` // rule, worker or person
	Name string `json:"name"`
}

func writtenOf(deciders []office.Decider) []written {
	out := []written{}
	for _, d := range deciders {
		switch {
		case d.Rule != "":
			out = append(out, written{"rule", d.Rule})
		case d.Worker != "":
			out = append(out, written{"worker", d.Worker})
		default:
			out = append(out, written{"person", d.Person})
		}
	}
	return out
}

// A colleague is a worker of the team that the lead, or another
// colleague, uses as a tool, with what it uses in turn.
type colleague struct {
	Name       string      `json:"name"`
	Colleagues []colleague `json:"colleagues"`
	Tools      []tool      `json:"tools"`
}

// A tool a worker may use: a job of its own, or of a file of tools.
type tool struct {
	Name string `json:"name"`
	Does string `json:"does"`
	Held bool   `json:"held"` // held for approval: by decide, or by the check for a question to a team
}

// team gives a team as its page shows it: its room with what is written
// for each node, its people, workers, routines, problems and request
// machine, and to its people its docs.
func (s *Screens) team(r *http.Request, me string) (any, error) {
	rd := s.engine.Reading()
	o := rd.Office
	t := o.Team(r.PathValue("team"))
	if t == nil {
		return nil, noTeam
	}
	people := []map[string]string{}
	for _, p := range t.People {
		people = append(people, map[string]string{"id": p, "name": personName(o, p)})
	}
	source := []map[string]any{}
	for _, b := range o.Bridges {
		for _, p := range b.Places {
			if p.Team == t.ID {
				source = append(source, map[string]any{"bridge": b.Name, "place": p.Name, "guests": p.Guests})
			}
		}
	}
	colleagues, tools := []colleague{}, []tool{}
	if m := rd.Machines[t.ID]; m.Draft != nil && t.Lead.Worker != "" {
		colleagues, tools = uses(o, t, m.Draft.Children["./"+o.Worker(t.Lead.Worker).File])
	}
	workers := []map[string]string{}
	for _, w := range t.Workers {
		definition, _ := o.Load(w.File)
		workers = append(workers, map[string]string{"name": w.Name, "type": w.Type, "definition": string(definition)})
	}
	routines := []map[string]any{}
	for _, routine := range t.Routines {
		// A routine whose at is not a time has no next; the team's problems say why.
		var next *time.Time
		if due, err := statechart.Next(routine.At+" "+o.Zone.String(), s.corbie.Now()); err == nil {
			next = &due
		}
		routines = append(routines, map[string]any{"at": routine.At, "say": routine.Say, "next": next})
	}
	var docs any
	if slices.Contains(t.People, me) {
		out, err := s.docs(r.Context(), rd, t.ID, map[string]any{"do": "list"})
		if err != nil {
			return nil, err
		}
		var listed struct {
			Names []string `json:"names"`
		}
		if err := decode(out, &listed); err != nil {
			return nil, err
		}
		docs = list(listed.Names)
	}
	return map[string]any{"id": t.ID, "name": t.Name, "about": t.About, "people": people,
		"flow": map[string]any{"source": source, "heed": writtenOf(t.Heed), "check": writtenOf(t.Check), "decide": writtenOf(t.Decide),
			"work": map[string]any{"lead": leadOf(t), "colleagues": colleagues, "tools": tools}},
		"workers": workers, "routines": routines, "problems": list(t.Problems),
		"machine": rd.Machines[t.ID].YAML, "docs": docs}, nil
}

// uses gives the colleagues and the other tools of a worker of a team,
// as the team's request machine sends it to Corbie, but those Corbie
// gives every agent: a colleague is a tool whose job is a worker of the
// team. A question to a team is held only for the team's check, so it is
// held only when the team has one.
func uses(o *office.Office, t *office.Team, worker *machine.Draft) ([]colleague, []tool) {
	colleagues, tools := []colleague{}, []tool{}
	var a agent.Agent
	if worker == nil || worker.Definition.Type != "agent" || json.Unmarshal(worker.Definition.Body, &a) != nil {
		return colleagues, tools
	}
	for _, name := range slices.Sorted(maps.Keys(a.Tools)) {
		job := a.Tools[name].Job
		if job == nil || strings.HasPrefix(job.Script, "corbie:") {
			continue // the tools every agent has, which no file of the office writes
		}
		if child := worker.Children[job.Machine]; child != nil {
			if w := o.Worker(child.Definition.Name); w != nil && w.Team == t.ID {
				c := colleague{Name: w.Name}
				c.Colleagues, c.Tools = uses(o, t, child)
				colleagues = append(colleagues, c)
				continue
			}
		}
		tools = append(tools, tool{Name: name, Does: a.Tools[name].Does, Held: job.Approve && (!asksTeam(job.Input) || t.Check != nil)})
	}
	return colleagues, tools
}

// asksTeam says whether a job's input names a team, as ask_team's does.
func asksTeam(input any) bool {
	in, _ := input.(map[string]any)
	to, _ := in["to"].(map[string]any)
	return to["team"] != nil
}

// docs runs the shipped docs.yaml for a team, started with the team's
// start list, and gives its output: Bulldog holds no key to kept values.
func (s *Screens) docs(ctx context.Context, rd *requests.Reading, team string, input map[string]any) (any, error) {
	o := rd.Office
	src, err := o.Load(".bulldog/docs/docs.yaml")
	if err != nil {
		return nil, err
	}
	draft, err := machine.Parse(src, func(name string) ([]byte, error) { return o.Load(".bulldog/docs/" + name) })
	if err != nil {
		return nil, err
	}
	input["place"] = o.Docs(team)
	id, err := s.corbie.Start(ctx, draft, client.Start{Allow: rd.Machines[team].Allow, Input: input,
		Labels: map[string]string{"bulldog": o.ID, "kind": "docs", "team": team}})
	if err != nil {
		return nil, err
	}
	run, err := s.corbie.Wait(ctx, id, s.Interval)
	switch {
	case err != nil:
		return nil, err
	case run.Status != "done":
		return nil, refusal{http.StatusBadRequest, fmt.Sprintf("The docs could not be %s: %s", map[string]string{"list": "listed", "read": "read", "write": "written"}[input["do"].(string)], run.Error)}
	}
	return run.Output, nil
}

// docOf gives the team and the name of the doc an address names, to the
// team's people; to anyone else, a doc that is not there.
func (s *Screens) docOf(r *http.Request, me string) (*requests.Reading, *office.Team, string, error) {
	rd := s.engine.Reading()
	t := rd.Office.Team(r.PathValue("team"))
	if t == nil || !slices.Contains(t.People, me) {
		return nil, nil, "", noDoc
	}
	return rd, t, r.PathValue("name"), nil
}

// readDoc reads one doc of a team, for its people.
func (s *Screens) readDoc(r *http.Request, me string) (any, error) {
	rd, t, name, err := s.docOf(r, me)
	if err != nil {
		return nil, err
	}
	out, err := s.docs(r.Context(), rd, t.ID, map[string]any{"do": "read", "name": name})
	if err != nil {
		return nil, err
	}
	var doc struct {
		Name string  `json:"name"`
		Text *string `json:"text"`
	}
	if err := decode(out, &doc); err != nil {
		return nil, err
	}
	if doc.Text == nil {
		return nil, noDoc
	}
	return doc, nil
}

// writeDoc writes one doc of a team, whole, for its people.
func (s *Screens) writeDoc(r *http.Request, me string) (any, error) {
	var body struct {
		Text *string `json:"text"`
	}
	if err := readJSON(r, 1<<20, &body); err != nil {
		return nil, err
	}
	rd, t, name, err := s.docOf(r, me)
	switch {
	case err != nil:
		return nil, err
	case body.Text == nil:
		return nil, refusal{http.StatusBadRequest, "Give the doc's text."}
	}
	_, err = s.docs(r.Context(), rd, t.ID, map[string]any{"do": "write", "name": name, "text": *body.Text})
	return struct{}{}, err
}

// decode reads a value as plain data into another shape.
func decode(v, into any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

// or gives a text, or another when it is empty.
func or(text, otherwise string) string {
	if text == "" {
		return otherwise
	}
	return text
}

// list gives a list, empty where there is none, as JSON writes it.
func list[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
