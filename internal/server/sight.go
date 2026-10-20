package server

import (
	"encoding/json"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/permission"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
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
	// machineOut is one of a bridge's machines, by its file, with what it
	// reaches once Corbie has checked it.
	machineOut struct {
		File string `json:"file"`
		reachOut
	}
	// reachOut is the hosts a machine may connect to and each secret it
	// may send, with where.
	reachOut struct {
		Hosts   []string    `json:"hosts,omitzero"`
		Secrets []secretOut `json:"secrets,omitzero"`
	}
	// secretOut is a secret a machine may send, by its name, * for every
	// secret, and each place it may send it: a host, * for anywhere, or,
	// as written, a host over any connection, a program or a path.
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
		// LeadRuns says its lead can be started: a person always, and a
		// worker with no problem.
		LeadRuns bool   `json:"lead_runs"`
		Within   string `json:"within,omitempty"`
		// Reached are the people its decisions reach, who stand as its
		// people, for a team with none of its own.
		Reached []string `json:"reached,omitempty"`
		// Heed is there when it names one, with its rule's path and its
		// settings only to its people and the people who decide a change;
		// Discloses, to them alone, says whether it sets a disclosure.
		Heed      *heedOut `json:"heed,omitempty"`
		Discloses *bool    `json:"discloses,omitempty"`
		// Docs are given only to those who may open them, in the one
		// team's answer.
		Docs []docOut `json:"docs,omitempty"`
	}
	// heedOut is a team's heed: its rule's path, the folder of a rule
	// Bulldog ships, whether a problem stops it, and its settings.
	heedOut struct {
		Rule     string       `json:"rule,omitempty"`
		Shipped  string       `json:"shipped,omitempty"`
		Stopped  bool         `json:"stopped,omitempty"`
		Settings []settingOut `json:"settings,omitempty"`
	}
	// settingOut is a setting a rule is given: as the team file writes
	// it, or, marked as the default, the value it takes when not written,
	// or no value when nothing gives one, with what it is for when its
	// page says.
	settingOut struct {
		Name    string `json:"name"`
		Value   any    `json:"value,omitempty"`
		Default bool   `json:"default,omitempty"`
		About   string `json:"about,omitempty"`
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
		// they are in, and whether they decide a change; and so are who
		// they are on each bridge, in the order a link is tried, each list
		// that names them, and the teams whose walk ends at them.
		Teams      []string      `json:"teams,omitempty"`
		Decides    bool          `json:"decides_changes,omitempty"`
		Identities []identityOut `json:"identities,omitempty"`
		Lists      []namedOut    `json:"lists,omitempty"`
		Walks      []string      `json:"walks,omitempty"`
	}
	// identityOut is who a person is on a bridge, and whether the bridge
	// has a direct place a link can be sent to.
	identityOut struct {
		Bridge string `json:"bridge"`
		ID     string `json:"id"`
		Direct bool   `json:"direct"`
	}
	// namedOut is a team's list that names a person, by its key, and
	// where on it they are, from 1.
	namedOut struct {
		Team string `json:"team"`
		Key  string `json:"key"`
		At   int    `json:"at"`
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
	me.Identities, me.Lists, me.Walks = identitiesOf(o, person), listsNaming(o, person), walksTo(o, person)
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
		out.Teams = append(out.Teams, teamOf(o, o.Teams[name], people, me.Decides || slices.Contains(people, person)))
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
		out.In = machineOf(o, b.Dir()+"/in.yaml", b.In, b.Settings)
		out.Out = machineOf(o, b.Dir()+"/out.yaml", b.Out, b.Settings)
	}
	return out
}

// machineOf gives one of a bridge's machines by its file, and, once Corbie
// has checked it, the hosts it may connect to and each secret it may send,
// with where, as the bridge's settings fill them.
func machineOf(o *office.Office, file string, d *machine.Draft, settings any) *machineOut {
	if d == nil {
		return &machineOut{File: file}
	}
	return &machineOut{File: file, reachOut: reachOf(fillAll(changes.Allowed(o, file, d), settings))}
}

// fillAll gives permissions each as a bridge's settings fill it.
func fillAll(entries []string, settings any) []string {
	fill := machine.NewEvaluator(time.Time{}, nil)
	scope := machine.Scope{"input": map[string]any{bridges.InputSettings: settings}}
	var out []string
	for _, entry := range entries {
		out = append(out, filled(fill, scope, entry))
	}
	return out
}

// reachOf gives permissions as the hosts and secrets they reach, with
// where each secret may go: a group as what it stands for of these.
func reachOf(entries []string) reachOut {
	out := reachOut{Hosts: []string{}, Secrets: []secretOut{}}
	var all []string
	for _, entry := range entries {
		all = append(all, reached(entry)...)
	}
	slices.Sort(all)
	for _, entry := range slices.Compact(all) {
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

// filled gives a permission as Corbie fills it from a bridge's settings,
// given as its input in scope, when its job starts, or as written when it reads anything else of its
// input or the settings do not fill it to a permission.
func filled(fill *machine.Evaluator, scope machine.Scope, entry string) string {
	for _, r := range machine.Reads(entry, "input") {
		if r.Name != bridges.InputSettings {
			return entry
		}
	}
	v, err := fill.Text(entry, scope, entry)
	if s, ok := v.(string); ok && err == nil && permission.CheckWritten(s) == nil {
		return s
	}
	return entry
}

// reached gives a permission as the hosts and secrets it reaches: a group
// as what it stands for of them, and anything else as it is.
func reached(entry string) []string {
	switch entry {
	case "network":
		return []string{permission.Net + ":*"}
	case "all":
		return []string{permission.Net + ":*", permission.Secret + ":*@*"}
	}
	return []string{entry}
}

// teamOf gives a team as everyone sees it, with the people that stand as
// its people, as the requests area gives them; and, to whole, one of its
// people or of the people who decide a change, its heed's rule and
// settings, and whether it sets a disclosure.
func teamOf(o *office.Office, t *office.Team, people []string, whole bool) teamOut {
	out := teamOut{Name: t.Name, Called: t.Called, About: t.About, People: orEmpty(t.People), Lead: t.Lead, Within: t.Within,
		LeadRuns: o.People[t.Lead] != nil || o.Workers[t.Lead] != nil && o.Workers[t.Lead].Runs()}
	if len(t.People) == 0 {
		out.Reached = people
	}
	if t.Heed != nil {
		out.Heed = &heedOut{}
	}
	if whole {
		if t.Heed != nil {
			out.Heed = heedOf(o, t.Heed)
		}
		discloses := t.HasDisclosure
		out.Discloses = &discloses
	}
	return out
}

// heedOf gives a heed as its team's people see it: its rule, and its
// settings, each as the team file writes it, or else, of a setting the
// rule's definitions read, the value it takes when not written, marked as
// the default, or none when nothing gives one; a rule Bulldog ships says
// what each is for.
func heedOf(o *office.Office, h *office.Heed) *heedOut {
	out := &heedOut{Rule: h.Rule, Stopped: !h.Runs(), Settings: []settingOut{}}
	var known []shipped.Setting
	if thing, ok := strings.CutPrefix(path.Dir(h.Rule), shipped.Dir+"/"); ok {
		out.Shipped = thing
		known = shipped.SettingsOf(thing)
	} else if r := o.Rules[h.Rule]; r != nil && !r.Script {
		found := map[string]any{}
		if shipped.SettingsIn(r.Written, found) == nil {
			for _, name := range slices.Sorted(maps.Keys(found)) {
				known = append(known, shipped.Setting{Name: name, Value: found[name]})
			}
		}
	}
	for _, k := range known {
		v, written := h.Settings[k.Name]
		if !written {
			v = k.Value
		}
		out.Settings = append(out.Settings, settingOut{Name: k.Name, Value: v, Default: !written && v != nil, About: k.About})
	}
	for _, name := range slices.Sorted(maps.Keys(h.Settings)) {
		if !slices.ContainsFunc(known, func(k shipped.Setting) bool { return k.Name == name }) {
			out.Settings = append(out.Settings, settingOut{Name: name, Value: h.Settings[name]})
		}
	}
	return out
}

// identitiesOf gives who a person is on each bridge, in the order a
// one-time link is tried, and whether each bridge has a direct place.
func identitiesOf(o *office.Office, person string) []identityOut {
	out := []identityOut{}
	for _, id := range o.People[person].Reached {
		_, direct := o.Place(id.Bridge, office.Direct)
		out = append(out, identityOut{Bridge: id.Bridge, ID: id.ID, Direct: direct})
	}
	return out
}

// listsNaming gives each list of deciders, a disclosure among them, that
// names a person, with where on it they are.
func listsNaming(o *office.Office, person string) []namedOut {
	out := []namedOut{}
	for _, name := range o.Order {
		t := o.Teams[name]
		lists := t.Decide
		if t.HasDisclosure {
			lists = append([]office.List{{Key: "disclosure", Deciders: t.Disclosure}}, lists...)
		}
		for _, l := range lists {
			if i := slices.IndexFunc(l.Deciders, func(d office.Decider) bool { return d.Kind == office.IsPerson && d.Name == person }); i >= 0 {
				out = append(out, namedOut{Team: name, Key: l.Key, At: i + 1})
			}
		}
	}
	return out
}

// walksTo gives the teams whose decisions end at a person when every rule
// and worker on the way refers them.
func walksTo(o *office.Office, person string) []string {
	out := []string{}
	for _, name := range o.Order {
		if slices.Contains(o.Reached(name), person) {
			out = append(out, name)
		}
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
		File string `json:"file"`
		// Decide and Disclosure are its lists, each with who decides, in
		// the order the walk asks them out to the people it ends at.
		Decide     []listOut    `json:"decide"`
		Disclosure []deciderOut `json:"disclosure,omitempty"`
		Routines   []routineOut `json:"routines"`
		Models     []modelOut   `json:"models"`
		Rules      []ruleOut    `json:"rules"`
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
	// routineOut is a routine as written, in the office's zone, with
	// when it comes round next and its one status: due; skipped, when
	// the request it last opened is still open, and the next coming round
	// is let go by; or cannot run, when its worker cannot.
	routineOut struct {
		At     string    `json:"at"`
		To     string    `json:"to"`
		Say    string    `json:"say"`
		Zone   string    `json:"zone"`
		Next   time.Time `json:"next,omitzero"`
		Status string    `json:"status"`
		Open   int64     `json:"open,omitempty"`
	}
	modelOut struct {
		Name   string `json:"name"`
		Text   string `json:"text"`
		Secret string `json:"secret,omitempty"`
	}
	// ruleOut is a rule the team names, by its path, with the folder of
	// one Bulldog ships; how the team names it: its heed, its disclosure
	// or a list by its key; whether it is a bare script, given no
	// permission; what it reaches; and whether it can be started.
	ruleOut struct {
		Path    string   `json:"path"`
		Shipped string   `json:"shipped,omitempty"`
		As      []string `json:"as"`
		Script  bool     `json:"script,omitempty"`
		Reach   reachOut `json:"reach"`
		Runs    bool     `json:"runs"`
	}
	workerOut struct {
		Name string `json:"name"`
		Type string `json:"type"`
		File string `json:"file"`
		// Model is as it is sent; ModelName the word it is named by, and
		// ModelFrom the team that gives that word.
		Model     string `json:"model,omitempty"`
		ModelName string `json:"model_name,omitempty"`
		ModelFrom string `json:"model_from,omitempty"`
		// Secret is the secret its model names as the service's key.
		Secret string `json:"secret,omitempty"`
		// Definition is what its file writes of how it runs, and Uses
		// the files of the office it reads, its own among them.
		Definition definitionOut `json:"definition"`
		Uses       []string      `json:"uses"`
		Tools      []toolOut     `json:"tools"`
		// HeldHere are the colleagues it calls, as far down as they go,
		// whose jobs that wait for a yes go to its ask.
		HeldHere []string `json:"held_here"`
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
		// Reach is what a script's or another machine's job may reach,
		// what it is refused, and which of what it reaches is asked
		// about.
		Reach *toolReach `json:"reach,omitempty"`
	}
	toolReach struct {
		reachOut
		Deny     []string `json:"deny"`
		AskAbout []string `json:"ask_about"`
	}
	// definitionOut is what a worker's file writes of how it runs: an
	// agent's system and prompt, its inputs, its tools files, and whether
	// it names the ask Bulldog ships.
	definitionOut struct {
		System     string   `json:"system,omitempty"`
		Prompt     string   `json:"prompt,omitempty"`
		Inputs     []string `json:"inputs"`
		ToolsFiles []string `json:"tools_files"`
		Asks       bool     `json:"asks"`
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
	seen := teamOf(o, t, people, whole)
	if seen.Docs, err = s.docs(o, person, t.Name); err != nil {
		return nil, err
	}
	if !whole {
		return seen, nil
	}
	out := teamWhole{teamOut: seen, File: t.File, Decide: []listOut{}, Routines: []routineOut{}, Models: []modelOut{}, Rules: rulesOf(o, t), Workers: []workerOut{}, Notes: []noteOut{}}
	for _, l := range t.Decide {
		out.Decide = append(out.Decide, listOut{Key: l.Key, Deciders: walked(o, t.Name, l.Key)})
	}
	if t.HasDisclosure {
		out.Disclosure = deciders(o, decisions.Disclosure(o, t.Name))
	}
	for _, rt := range t.Routines {
		if out.Routines, err = s.routineOf(o, t.Name, rt, out.Routines); err != nil {
			return nil, err
		}
	}
	for _, m := range t.Models {
		out.Models = append(out.Models, modelOut{Name: m.Name, Text: m.Text, Secret: m.Secret})
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

// routineOf adds a routine to those given, with when it comes round next,
// and its one status.
func (s *site) routineOf(o *office.Office, team string, rt office.Routine, to []routineOut) ([]routineOut, error) {
	out := routineOut{At: rt.At, To: rt.To, Say: rt.Say, Zone: o.Zone.String(), Next: rt.Next(s.clock.Now(), o.Zone), Status: "due"}
	open, err := s.Requests.RoutineOpen(team, rt)
	if err != nil {
		return nil, err
	}
	switch w := o.Workers[rt.To]; {
	case w == nil || !w.Runs():
		out.Status = "cannot run"
	case open != 0:
		out.Status, out.Open = "skipped", open
	}
	return append(to, out), nil
}

// rulesOf gives each rule a team names, in the order it names them: its
// heed, its disclosure, then its lists.
func rulesOf(o *office.Office, t *office.Team) []ruleOut {
	out := []ruleOut{}
	add := func(rule, as string) {
		i := slices.IndexFunc(out, func(r ruleOut) bool { return r.Path == rule })
		if i < 0 {
			r := ruleOut{Path: rule, As: []string{}, Reach: reachOut{Hosts: []string{}, Secrets: []secretOut{}}}
			if thing, ok := strings.CutPrefix(path.Dir(rule), shipped.Dir+"/"); ok {
				r.Shipped = thing
			}
			if known := o.Rules[rule]; known != nil {
				r.Script, r.Runs = known.Script, known.Runs()
				if known.Draft != nil {
					r.Reach = reachOf(changes.Allowed(o, rule, known.Draft))
				}
			}
			i, out = len(out), append(out, r)
		}
		if !slices.Contains(out[i].As, as) {
			out[i].As = append(out[i].As, as)
		}
	}
	if t.Heed != nil {
		add(t.Heed.Rule, "heed")
	}
	lists := t.Decide
	if t.HasDisclosure {
		lists = append([]office.List{{Key: "disclosure", Deciders: t.Disclosure}}, lists...)
	}
	for _, l := range lists {
		for _, d := range l.Deciders {
			if d.Kind == office.IsRule {
				add(d.Name, l.Key)
			}
		}
	}
	return out
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

// workerOf gives a worker as it is written: its model as it is sent, with
// its secret; what its file writes of how it runs, and the files it
// reads; its tools, a flow's stages or a statechart's jobs, each with what
// it does and what its job is, what a script's or a machine's reaches, and
// who decides each that waits for a yes; the colleagues whose held jobs
// come to its ask; and the teams it may ask.
func workerOf(o *office.Office, w *office.Worker) workerOut {
	def, secret := definitionOf(w)
	out := workerOut{Name: w.Name, Type: w.Type, File: w.File, Model: w.Model, ModelName: w.ModelName, Secret: secret, Definition: def,
		Uses: orEmpty(w.Uses), Tools: []toolOut{}, HeldHere: heldAt(o, w), MayAsk: []string{}, Runs: w.Runs()}
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
		if t.Kind == office.ToolScript || t.Kind == office.ToolMachine {
			t.Reach = &toolReach{reachOut: reachOf(tool.Allow), Deny: orEmpty(tool.Deny), AskAbout: orEmpty(tool.AskAbout)}
		}
		out.Tools = append(out.Tools, t)
	}
	return out
}

// definitionOf gives what a worker's file writes of how it runs, as it is
// sent, and the secret its model names.
func definitionOf(w *office.Worker) (definitionOut, string) {
	out := definitionOut{Inputs: []string{}, ToolsFiles: []string{}}
	var def struct {
		System    string   `yaml:"system"`
		Prompt    any      `yaml:"prompt"`
		Input     []string `yaml:"input"`
		ToolsFile []string `yaml:"tools_file"`
		Ask       any      `yaml:"ask"`
		Model     any      `yaml:"model"`
	}
	if yaml.Unmarshal(w.Definition, &def) != nil {
		return out, ""
	}
	out.System, out.Inputs, out.ToolsFiles, out.Asks = def.System, orEmpty(def.Input), orEmpty(def.ToolsFile), def.Ask != nil
	switch p := def.Prompt.(type) {
	case nil:
	case string:
		out.Prompt = p
	default:
		text, _ := json.Marshal(p)
		out.Prompt = string(text)
	}
	secret := ""
	if m, ok := def.Model.(map[string]any); ok {
		secret, _ = m["secret"].(string)
	}
	return out, secret
}

// heldAt gives the colleagues a worker calls, and those they call, as far
// down as they go, that hold a job waiting for a yes and use its ask, since
// they name none of their own: none when it names none.
func heldAt(o *office.Office, w *office.Worker) []string {
	out := []string{}
	if w.Draft == nil || w.Draft.Definition.Ask == nil {
		return out
	}
	seen := map[string]bool{w.Name: true}
	var walk func(*office.Worker)
	walk = func(at *office.Worker) {
		for _, tool := range at.Held {
			c := o.Workers[tool.Worker]
			if tool.Kind != office.ToolWorker || c == nil || seen[c.Name] || c.Draft != nil && c.Draft.Definition.Ask != nil {
				continue
			}
			seen[c.Name] = true
			if slices.ContainsFunc(c.Held, func(t office.Tool) bool { return t.Waits }) {
				out = append(out, c.Name)
			}
			walk(c)
		}
	}
	walk(w)
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
// them.
func walked(o *office.Office, team, key string) []deciderOut {
	return deciders(o, decisions.Walk(o, team, key))
}

// deciders gives the stages of a walk as who decides: each rule, by its
// path in the office, and each worker, with the team it is of, then each
// person of the people it ends at.
func deciders(o *office.Office, stages []decisions.Stage) []deciderOut {
	out := []deciderOut{}
	for _, s := range stages {
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
