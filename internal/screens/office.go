package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/agent"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
	"go.yaml.in/yaml/v3"
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

// A tool the lead may use: a job of its own, or of a file of tools.
type tool struct {
	Name string `json:"name"`
	Does string `json:"does"`
	Held bool   `json:"held"` // marked for approval
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
	decide := writtenOf(t.Decide)
	if t.Decide == nil {
		decide = writtenOf(peopleOf(t.People))
	}
	colleagues, tools := uses(o, t, rd.Machines[t.ID])
	workers := []map[string]string{}
	for _, w := range t.Workers {
		definition, _ := o.Load(w.File)
		workers = append(workers, map[string]string{"name": w.Name, "type": w.Type, "model": modelOf(definition), "definition": string(definition)})
	}
	routines := []map[string]any{}
	for _, routine := range t.Routines {
		routines = append(routines, map[string]any{"at": routine.At, "say": routine.Say, "next": next(routine.At, o.Zone, s.corbie.Now())})
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
	return map[string]any{"id": t.ID, "name": t.Name, "about": t.About, "path": t.Path, "people": people,
		"flow": map[string]any{"source": source, "heed": writtenOf(t.Heed), "check": writtenOf(t.Check), "decide": decide,
			"work": map[string]any{"lead": leadOf(t), "colleagues": colleagues, "tools": tools}},
		"workers": workers, "routines": routines, "problems": list(t.Problems),
		"machine": rd.Machines[t.ID].YAML, "docs": docs}, nil
}

// peopleOf gives people as deciders.
func peopleOf(people []string) []office.Decider {
	var deciders []office.Decider
	for _, p := range people {
		deciders = append(deciders, office.Decider{Person: p})
	}
	return deciders
}

// uses gives the colleagues and the other tools of a team's lead, as its
// request machine sends it to Corbie, but those Corbie gives every agent:
// a colleague is a tool whose job is a worker of the team.
func uses(o *office.Office, t *office.Team, m *requests.Machine) ([]string, []tool) {
	colleagues, tools := []string{}, []tool{}
	if m.Draft == nil || t.Lead.Worker == "" {
		return colleagues, tools
	}
	led := m.Draft.Children["./"+o.Worker(t.Lead.Worker).File]
	var a agent.Agent
	if led == nil || led.Definition.Type != "agent" || json.Unmarshal(led.Definition.Body, &a) != nil {
		return colleagues, tools
	}
	names := make([]string, 0, len(a.Tools))
	for name := range a.Tools {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		job := a.Tools[name].Job
		if job == nil || strings.HasPrefix(job.Script, "corbie:") {
			continue // the tools every agent has, which no file of the office writes
		}
		if child := led.Children[job.Machine]; child != nil {
			if w := o.Worker(child.Definition.Name); w != nil && w.Team == t.ID {
				colleagues = append(colleagues, w.Name)
				continue
			}
		}
		tools = append(tools, tool{Name: name, Does: a.Tools[name].Does, Held: job.Approve})
	}
	return colleagues, tools
}

// modelOf gives the name of the model a definition names.
func modelOf(definition []byte) string {
	var d struct {
		Model any `yaml:"model"`
	}
	yaml.Unmarshal(definition, &d)
	switch m := d.Model.(type) {
	case string:
		return m
	case map[string]any:
		name, _ := m["name"].(string)
		return name
	}
	return ""
}

// next gives when a routine is next due: its time of day, after its
// weekday if it gives one, strictly after now, in the office's zone.
func next(at string, zone *time.Location, now time.Time) time.Time {
	weekday := -1
	days := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	if day, clock, ok := strings.Cut(at, " "); ok {
		weekday, at = days[day], clock
	}
	var hour, minute int
	fmt.Sscanf(at, "%d:%d", &hour, &minute)
	local := now.In(zone)
	for d := 0; d <= 8; d++ {
		due := time.Date(local.Year(), local.Month(), local.Day()+d, hour, minute, 0, 0, zone)
		if (weekday < 0 || int(due.Weekday()) == weekday) && due.After(now) {
			return due.UTC()
		}
	}
	return time.Time{}
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
