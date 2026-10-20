package server

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/permission"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
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
	}
	officeOut struct {
		Name  string `json:"name"`
		About string `json:"about,omitempty"`
		Zone  string `json:"zone"`
		// Runs are how many runs it lets work at once, how many are at
		// work and how many requests wait their turn.
		Runs runsOut `json:"runs"`
		// Change is who decides a change, in the order the walk asks them.
		Change  []deciderOut `json:"change"`
		Bridges []bridgeOut  `json:"bridges"`
	}
	runsOut struct {
		Limit   int `json:"limit"`
		Working int `json:"working"`
		Queued  int `json:"queued"`
	}
	// bridgeOut is a bridge: the folder it runs, one Bulldog ships or its
	// own; whether it listens, or hears, at its address; its status,
	// running or stopped, and failing to the top team's people alone, with
	// what Corbie said; and the places it listens in. To the people who
	// decide a change it gives its settings as written, and its in and out
	// machines.
	bridgeOut struct {
		Name     string      `json:"name"`
		Folder   string      `json:"folder"`
		Shipped  bool        `json:"shipped"`
		Listens  bool        `json:"listens"`
		Hears    bool        `json:"hears"`
		Address  string      `json:"address,omitempty"`
		Status   string      `json:"status"`
		Failing  *failingOut `json:"failing,omitempty"`
		Places   []placeOut  `json:"places"`
		Settings any         `json:"settings,omitempty"`
		In       *machineOut `json:"in,omitempty"`
		Out      *machineOut `json:"out,omitempty"`
	}
	// machineOut is one of a bridge's machines, by its file, with the
	// hosts it may connect to and each secret it may send, with where,
	// once Corbie has checked it.
	machineOut struct {
		File    string      `json:"file"`
		Hosts   []string    `json:"hosts,omitzero"`
		Secrets []secretOut `json:"secrets,omitzero"`
	}
	// secretOut is a secret a machine may send, by its name, and each
	// place it may send it: a host, or * for anywhere.
	secretOut struct {
		Name string   `json:"name"`
		To   []string `json:"to"`
	}
	// placeOut is a place a bridge listens in, the team asked there, the
	// office when it names none, and whether guests may ask there.
	placeOut struct {
		Name   string `json:"name"`
		Team   string `json:"team"`
		Guests bool   `json:"guests"`
	}
	teamOut struct {
		Name   string   `json:"name"`
		Called string   `json:"called"`
		About  string   `json:"about,omitempty"`
		People []string `json:"people"`
		Lead   string   `json:"lead"`
		Within string   `json:"within,omitempty"`
		// Reached are the people its decisions reach, who stand as its
		// people, for a team with none of its own.
		Reached []string `json:"reached,omitempty"`
		// Heed is there when it names one, with its rule's path only to
		// its people and the people who decide a change; Discloses, to
		// them alone, says whether it sets a disclosure.
		Heed      *heedOut `json:"heed,omitempty"`
		Discloses *bool    `json:"discloses,omitempty"`
		// Docs are given only to those who may open them, in the one
		// team's answer.
		Docs []docOut `json:"docs,omitempty"`
	}
	heedOut struct {
		Rule string `json:"rule,omitempty"`
	}
	// docOut is one of a team's docs: its name, the path of its file in
	// the team's docs folder without .md, and its size in bytes.
	docOut struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
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
// waits on them and the problems they may see; to a team's people and the
// people who decide a change, its heed's rule and whether it sets a
// disclosure, and each bridge's settings and machines; and, to the top
// team's people, the bridges that are failing.
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
	load := s.Work.Load()
	out := meOut{Me: me, Teams: []teamOut{}, People: []personOut{}, Waiting: len(waits), Problems: problems,
		Office: officeOut{Name: o.Name(), About: o.TopTeam().About, Zone: o.Zone.String(),
			Runs:   runsOut{Limit: o.Runs, Working: load.Working, Queued: load.Queued},
			Change: walked(o, office.Top, "change")}}
	for _, name := range o.Order {
		people, err := s.Requests.People(name)
		if err != nil {
			return nil, err
		}
		out.Teams = append(out.Teams, teamOf(o.Teams[name], people, me.Decides || slices.Contains(people, person)))
	}
	for _, name := range o.Everyone {
		out.People = append(out.People, personOf(o, name))
	}
	top, err := s.Requests.People(office.Top)
	if err != nil {
		return nil, err
	}
	failing := map[string]*failingOut{}
	if slices.Contains(top, person) {
		troubles, err := s.Bridges.Troubles()
		if err != nil {
			return nil, err
		}
		for _, t := range troubles {
			failing[t.Bridge] = &failingOut{Error: t.Error, Since: t.Since.In(o.Zone), Failures: t.Failures}
		}
	}
	out.Office.Bridges = []bridgeOut{}
	for _, name := range o.BridgeOrder {
		out.Office.Bridges = append(out.Office.Bridges, s.bridgeOf(o, o.Bridges[name], failing[name], me.Decides))
	}
	return out, nil
}

// bridgeOf gives a bridge as everyone sees it: stopped while it has a
// problem, failing when the one looking is told it is, and otherwise
// running; and, to whole, one of the people who decide a change, its
// settings and its machines.
func (s *site) bridgeOf(o *office.Office, b *office.Bridge, failing *failingOut, whole bool) bridgeOut {
	out := bridgeOut{Name: b.Name, Folder: b.Dir(), Shipped: b.From != "", Listens: b.Listens, Hears: b.Hears, Status: "running", Places: []placeOut{}}
	if b.Hears {
		out.Address = s.url + "/bridges/" + b.Name
	}
	switch {
	case !b.Runs():
		out.Status = "stopped"
	case failing != nil:
		out.Status, out.Failing = "failing", failing
	}
	for _, p := range b.Places {
		out.Places = append(out.Places, placeOut{Name: p.Name, Team: p.Team, Guests: p.Guests})
	}
	if whole {
		out.Settings = b.Settings
		out.In = machineOf(o, b.Dir()+"/in.yaml", b.In)
		out.Out = machineOf(o, b.Dir()+"/out.yaml", b.Out)
	}
	return out
}

// machineOf gives one of a bridge's machines by its file, and, once Corbie
// has checked it, the hosts it may connect to and each secret it may send,
// with where.
func machineOf(o *office.Office, file string, d *machine.Draft) *machineOut {
	if d == nil {
		return &machineOut{File: file}
	}
	out := &machineOut{File: file, Hosts: []string{}, Secrets: []secretOut{}}
	for _, entry := range changes.Allowed(o, file, d) {
		kind, scope, _ := strings.Cut(entry, ":")
		switch kind {
		case permission.Net:
			out.Hosts = append(out.Hosts, scope)
		case permission.Secret:
			name, to, _ := strings.Cut(scope, "@")
			to = strings.TrimPrefix(to, permission.Net+":")
			i := slices.IndexFunc(out.Secrets, func(s secretOut) bool { return s.Name == name })
			if i < 0 {
				i = len(out.Secrets)
				out.Secrets = append(out.Secrets, secretOut{Name: name, To: []string{}})
			}
			out.Secrets[i].To = append(out.Secrets[i].To, to)
		}
	}
	return out
}

// teamOf gives a team as everyone sees it, with the people that stand as
// its people, as the requests area gives them; and, to whole, one of its
// people or of the people who decide a change, its heed's rule and
// whether it sets a disclosure.
func teamOf(t *office.Team, people []string, whole bool) teamOut {
	out := teamOut{Name: t.Name, Called: t.Called, About: t.About, People: orEmpty(t.People), Lead: t.Lead, Within: t.Within}
	if len(t.People) == 0 {
		out.Reached = people
	}
	if t.Heed != nil {
		out.Heed = &heedOut{}
	}
	if whole {
		if t.Heed != nil {
			out.Heed.Rule = t.Heed.Rule
		}
		discloses := t.HasDisclosure
		out.Discloses = &discloses
	}
	return out
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
	// toolOut is a tool, a flow's stage or a statechart's job, with what
	// it does and what its job is: a worker of the same team, named;
	// another machine or a script, by its file; or one of the tools
	// Bulldog ships. One that waits for a yes gives who decides it, in the
	// order its walk asks them, and none when nobody does and what waits
	// on it is refused; one that does not wait gives no list at all. A
	// listed tool is a list: entry, which stands for the tools its server
	// lists.
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
	whole := in || slices.Contains(changes.Deciders(o), person)
	seen := teamOf(t, people, whole)
	if seen.Docs, err = s.docs(o, person, t.Name); err != nil {
		return nil, err
	}
	if !whole {
		return seen, nil
	}
	out := teamWhole{teamOut: seen, File: t.File, Decide: []listOut{}, Routines: []routineOut{}, Models: []modelOut{}, Workers: []workerOut{}, Notes: []noteOut{}}
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
		out.Workers = append(out.Workers, workerOf(o, o.Workers[name]))
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

// docs gives a team's docs to one who may open them, and nothing to
// anyone else.
func (s *site) docs(o *office.Office, person, team string) ([]docOut, error) {
	opens, err := s.opensDocs(o, person, team)
	if err != nil || !opens {
		return nil, err
	}
	docs, err := o.Docs(team)
	if err != nil {
		return nil, err
	}
	out := []docOut{}
	for _, d := range docs {
		out = append(out, docOut{Name: d.Name, Size: d.Size})
	}
	return out, nil
}

// opensDocs says whether a person may open a team's docs: one of its
// people, or of a team within it, whose workers read them, or one who
// decides a change.
func (s *site) opensDocs(o *office.Office, person, team string) (bool, error) {
	if slices.Contains(changes.Deciders(o), person) {
		return true, nil
	}
	for name := range o.Teams {
		for u := o.Teams[name]; u != nil; u = o.Teams[u.Within] {
			if u.Name != team {
				continue
			}
			people, err := s.Requests.People(name)
			if err != nil {
				return false, err
			}
			if slices.Contains(people, person) {
				return true, nil
			}
		}
	}
	return false, nil
}

// doc gives one of a team's docs whole, to one who may open it.
func (s *site) doc(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	t := o.Teams[r.PathValue("team")]
	if t == nil {
		return nil, errNotThere
	}
	opens, err := s.opensDocs(o, person, t.Name)
	switch {
	case err != nil:
		return nil, err
	case !opens:
		return nil, errNotThere
	}
	docs, err := o.Docs(t.Name)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(docs, func(d office.Doc) bool { return d.Name == r.PathValue("doc") })
	if i < 0 {
		return nil, errNotThere
	}
	text, err := o.DocText(docs[i])
	if err != nil {
		return nil, err
	}
	return docText{Name: docs[i].Name, Text: text}, nil
}

// docText is one doc, whole.
type docText struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// workerOf gives a worker as it is written: its model as it is sent; its
// tools, a flow's stages or a statechart's jobs, each with what it does
// and what its job is, and who decides each that waits for a yes; and the
// teams it may ask.
func workerOf(o *office.Office, w *office.Worker) workerOut {
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
	for _, tool := range w.Held {
		t := toolOut{Name: tool.Name, Does: tool.Does, Kind: tool.Kind, Worker: tool.Worker, File: tool.File, Listed: tool.Listed, Waits: tool.Waits}
		if t.Waits {
			t.Deciders = walked(o, w.Team, tool.Name)
		}
		out.Tools = append(out.Tools, t)
	}
	return out
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

// walked gives who decides a key of a team, in the order the walk asks
// them: each rule, by its path in the office, and each worker, with the
// team it is of, then each person of the people it ends at.
func walked(o *office.Office, team, key string) []deciderOut {
	out := []deciderOut{}
	for _, s := range decisions.Walk(o, team, key) {
		if s.Kind != decisions.IsPeople {
			out = append(out, deciderOut{Kind: s.Kind, Name: s.Name, Team: teamOfWorker(o, s.Kind, s.Name)})
			continue
		}
		for _, name := range s.People {
			out = append(out, deciderOut{Kind: office.IsPerson, Name: name})
		}
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
