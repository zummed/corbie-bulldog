package server

import (
	"net/http"
	"slices"
	"time"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// What everyone signed in sees of the office.
type (
	meOut struct {
		Me       personOut    `json:"me"`
		Office   officeOut    `json:"office"`
		Teams    []teamOut    `json:"teams"`
		People   []personOut  `json:"people"`
		Waiting  int          `json:"waiting"`
		Problems []problemOut `json:"problems"`
		Failing  []failingOut `json:"failing"`
	}
	officeOut struct {
		Name  string `json:"name"`
		About string `json:"about,omitempty"`
		Zone  string `json:"zone"`
	}
	teamOut struct {
		Name   string   `json:"name"`
		Called string   `json:"called"`
		About  string   `json:"about,omitempty"`
		People []string `json:"people"`
		Lead   string   `json:"lead"`
		Within string   `json:"within,omitempty"`
	}
	personOut struct {
		Name    string   `json:"name"`
		Called  string   `json:"called"`
		Title   string   `json:"title,omitempty"`
		Bridges []string `json:"bridges"`
		// Teams and Decides are said only of the one looking: the teams
		// they are in, and whether they decide a change.
		Teams   []string `json:"teams,omitempty"`
		Decides bool     `json:"decides_changes,omitempty"`
	}
	problemOut struct {
		File   string `json:"file"`
		Line   int    `json:"line,omitempty"`
		Place  string `json:"place,omitempty"`
		Text   string `json:"text"`
		Team   string `json:"team"`
		Worker string `json:"worker,omitempty"`
		Bridge string `json:"bridge,omitempty"`
		Rule   string `json:"rule,omitempty"`
		Said   string `json:"said"`
	}
	failingOut struct {
		Bridge   string    `json:"bridge"`
		Error    string    `json:"error"`
		Since    time.Time `json:"since"`
		Failures int       `json:"failures"`
	}
)

// office gives the office as it was last read well, which is there for
// anyone signed in.
func (s *site) office() (*office.Office, error) {
	o := s.Keeper.Office()
	if o == nil {
		return nil, errNotThere
	}
	return o, nil
}

// me gives who is looking, the office as everyone sees it, how much
// waits on them, the problems they may see and, to the top team's
// people, the bridges that are failing.
func (s *site) me(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	waits, err := s.Requests.WaitingOn(person)
	if err != nil {
		return nil, err
	}
	problems, err := s.problems(o, person, "")
	if err != nil {
		return nil, err
	}
	me := personOf(o, person)
	me.Teams = o.TeamsOf(person)
	me.Decides = slices.Contains(changes.Deciders(o), person)
	out := meOut{Me: me, Office: officeOut{Name: o.Name(), About: o.TopTeam().About, Zone: o.Zone.String()},
		Teams: []teamOut{}, People: []personOut{}, Waiting: len(waits), Problems: problems, Failing: []failingOut{}}
	for _, name := range o.Order {
		out.Teams = append(out.Teams, teamOf(o.Teams[name]))
	}
	for _, name := range o.Everyone {
		out.People = append(out.People, personOf(o, name))
	}
	top, err := s.Requests.People(office.Top)
	if err != nil {
		return nil, err
	}
	if slices.Contains(top, person) {
		troubles, err := s.Bridges.Troubles()
		if err != nil {
			return nil, err
		}
		for _, t := range troubles {
			out.Failing = append(out.Failing, failingOut{Bridge: t.Bridge, Error: t.Error, Since: t.Since.In(o.Zone), Failures: t.Failures})
		}
	}
	return out, nil
}

func teamOf(t *office.Team) teamOut {
	return teamOut{Name: t.Name, Called: t.Called, About: t.About, People: orEmpty(t.People), Lead: t.Lead, Within: t.Within}
}

// personOf gives a person as everyone sees them: the bridges that reach
// them by name, and none of who they are there.
func personOf(o *office.Office, name string) personOut {
	p := o.People[name]
	out := personOut{Name: p.Name, Called: p.Called, Title: p.Title, Bridges: []string{}}
	for _, id := range p.Reached {
		out.Bridges = append(out.Bridges, id.Bridge)
	}
	return out
}

// problems gives the problems of the latest reading a person may see, of
// one team or of all: those of the teams they are among the people of,
// and every one to the people who decide a change.
func (s *site) problems(o *office.Office, person, team string) ([]problemOut, error) {
	all := slices.Contains(changes.Deciders(o), person)
	out := []problemOut{}
	for _, p := range s.Keeper.Problems() {
		of := p.Team
		if of == "" {
			of = office.Top
		}
		if team != "" && of != team {
			continue
		}
		if !all {
			people, err := s.Requests.People(of)
			if err != nil {
				return nil, err
			}
			if !slices.Contains(people, person) {
				continue
			}
		}
		out = append(out, problemOf(p))
	}
	return out, nil
}

// problemOf gives a problem as the screens show it, on its team, the
// office's when it names none.
func problemOf(p office.Problem) problemOut {
	of := p.Team
	if of == "" {
		of = office.Top
	}
	return problemOut{File: p.File, Line: p.Line, Place: p.Place, Text: p.Text, Team: of, Worker: p.Worker, Bridge: p.Bridge, Rule: p.Rule, Said: p.String()}
}

func problemsOf(ps office.Problems) []problemOut {
	out := []problemOut{}
	for _, p := range ps {
		out = append(out, problemOf(p))
	}
	return out
}

// How a team is written, as its people and the people who decide a change
// see it.
type (
	teamWhole struct {
		teamOut
		File       string       `json:"file"`
		Decide     []listOut    `json:"decide"`
		Disclosure []deciderOut `json:"disclosure,omitempty"`
		Routines   []routineOut `json:"routines"`
		Models     []modelOut   `json:"models"`
		Workers    []workerOut  `json:"workers"`
		Notes      []noteOut    `json:"notes"`
		Problems   []problemOut `json:"problems"`
		// In says the one looking is among its people, and AtWork, to
		// them alone, are its workers at work, and on what.
		In     bool     `json:"in"`
		AtWork []atWork `json:"at_work,omitempty"`
	}
	listOut struct {
		Key      string       `json:"key"`
		Deciders []deciderOut `json:"deciders"`
	}
	// deciderOut is a decider as the team file names it, and what it is: a
	// person, a rule by its path as written, or a worker, with the team it
	// is of.
	deciderOut struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
		Team string `json:"team,omitempty"`
	}
	routineOut struct {
		At  string `json:"at"`
		To  string `json:"to"`
		Say string `json:"say"`
	}
	modelOut struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	workerOut struct {
		Name string `json:"name"`
		Type string `json:"type"`
		File string `json:"file"`
		// Model is as it is sent; ModelName the word it is named by, and
		// ModelFrom the team that gives that word.
		Model     string    `json:"model,omitempty"`
		ModelName string    `json:"model_name,omitempty"`
		ModelFrom string    `json:"model_from,omitempty"`
		Tools     []toolOut `json:"tools"`
		// MayAsk are the teams it may ask, by name, before any request
		// above the one it works on leaves some out.
		MayAsk []string `json:"may_ask"`
		Runs   bool     `json:"runs"`
	}
	// toolOut is a tool with what it does and what its job is: a worker
	// of the same team, named; another machine or a script, by its file;
	// or one of the tools Bulldog ships. One that waits for a yes gives
	// who decides it, in order, as the team file names them, and none
	// when nobody does and what waits on it is refused; one that does not
	// wait gives no list at all. A listed tool is a list: entry, which
	// stands for the tools its server lists.
	toolOut struct {
		Name     string       `json:"name"`
		Does     string       `json:"does"`
		Kind     string       `json:"kind"`
		Worker   string       `json:"worker,omitempty"`
		File     string       `json:"file,omitempty"`
		Listed   bool         `json:"listed,omitempty"`
		Waits    bool         `json:"waits"`
		Deciders []deciderOut `json:"deciders"`
	}
	noteOut struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	atWork struct {
		Worker  string `json:"worker"`
		Request int64  `json:"request"`
		Run     string `json:"run"`
	}
)

// team gives one team as the one looking may see it: as everyone sees it,
// and to its people and the people who decide a change, how it is
// written; which of its workers are at work, and on what, only to its
// people.
func (s *site) team(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	t := o.Teams[r.PathValue("team")]
	if t == nil {
		return nil, errNotThere
	}
	people, err := s.Requests.People(t.Name)
	if err != nil {
		return nil, err
	}
	in := slices.Contains(people, person)
	if !in && !slices.Contains(changes.Deciders(o), person) {
		return teamOf(t), nil
	}
	out := teamWhole{teamOut: teamOf(t), File: t.File, Decide: []listOut{}, Routines: []routineOut{}, Models: []modelOut{}, Workers: []workerOut{}, Notes: []noteOut{}}
	for _, l := range t.Decide {
		out.Decide = append(out.Decide, listOut{Key: l.Key, Deciders: writtenAll(o, l.Deciders)})
	}
	if t.HasDisclosure {
		out.Disclosure = writtenAll(o, t.Disclosure)
	}
	for _, rt := range t.Routines {
		out.Routines = append(out.Routines, routineOut{At: rt.At, To: rt.To, Say: rt.Say})
	}
	for _, m := range t.Models {
		out.Models = append(out.Models, modelOut{Name: m.Name, Text: m.Text})
	}
	for _, name := range t.Workers {
		wk, err := workerOf(o, o.Workers[name])
		if err != nil {
			return nil, err
		}
		out.Workers = append(out.Workers, wk)
	}
	notes, err := o.Notes(t.Name)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		out.Notes = append(out.Notes, noteOut{Name: n.Name, Text: n.Text})
	}
	if out.Problems, err = s.problems(o, person, t.Name); err != nil {
		return nil, err
	}
	if out.In = in; in {
		if out.AtWork, err = s.atWork(t.Name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// workerOf gives a worker as it is written: its model as it is sent; its
// tools, each with what it does and what its job is, and who decides each
// that waits for a yes; and the teams it may ask.
func workerOf(o *office.Office, w *office.Worker) (workerOut, error) {
	out := workerOut{Name: w.Name, Type: w.Type, File: w.File, Model: w.Model, ModelName: w.ModelName, Tools: []toolOut{}, MayAsk: []string{},
		Runs: w.Runs()}
	for _, t := range work.MayAsk(o, w.Team, w.Name, nil) {
		out.MayAsk = append(out.MayAsk, t.Name)
	}
	if w.ModelName != "" {
		if m, ok := o.Model(w.Team, w.ModelName); ok {
			out.ModelFrom = m.Team
		}
	}
	waits, err := changes.Waits(w)
	if err != nil {
		return workerOut{}, err
	}
	for _, tool := range w.Held {
		t := toolOut{Name: tool.Name, Does: tool.Does, Kind: tool.Kind, Worker: tool.Worker, File: tool.File, Listed: tool.Listed, Waits: waits[tool.Name]}
		if t.Waits {
			t.Deciders = decidersOf(o, w.Team, tool.Name)
		}
		out.Tools = append(out.Tools, t)
	}
	return out, nil
}

// atWork gives the team's workers that are at work on its open requests,
// and on which.
func (s *site) atWork(team string) ([]atWork, error) {
	open, err := s.Requests.OpenOf(team)
	if err != nil {
		return nil, err
	}
	out := []atWork{}
	for _, sum := range open {
		runs, err := s.Work.Runs(sum.Number)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			if run.Ended.IsZero() && run.Kind == work.KindWork {
				out = append(out, atWork{Worker: run.Worker, Request: sum.Number, Run: run.Name})
			}
		}
	}
	return out, nil
}

// decidersOf gives who decides a key of a team, in order, each as the
// team file names them: its list of that key, else its everything, else
// the people its decisions reach from a team around it.
func decidersOf(o *office.Office, team, key string) []deciderOut {
	if list, ok := o.List(team, key); ok && key != "everything" {
		return writtenAll(o, list)
	}
	if deciders := o.Everything(team); len(deciders) > 0 {
		return writtenAll(o, deciders)
	}
	out := []deciderOut{}
	for _, name := range o.Reached(team) {
		out = append(out, deciderOut{Kind: office.IsPerson, Name: name})
	}
	return out
}

// writtenAll gives a list of deciders each as it is written.
func writtenAll(o *office.Office, deciders []office.Decider) []deciderOut {
	out := []deciderOut{}
	for _, d := range deciders {
		out = append(out, deciderOut{Kind: d.Kind, Name: d.Written, Team: teamOfWorker(o, d.Kind, d.Name)})
	}
	return out
}

// teamOfWorker gives the team a decider is of when it is a worker of the
// office, and nothing for anyone else.
func teamOfWorker(o *office.Office, kind, name string) string {
	if w := o.Workers[name]; kind == office.IsWorker && w != nil {
		return w.Team
	}
	return ""
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
