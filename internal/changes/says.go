package changes

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/agent"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/permission"
)

// A reading is the office read from its files: the office, and the files
// it was read from.
type reading struct {
	o     *office.Office
	files map[string][]byte
}

// telling gathers what a change says, with what lets more by apart, and
// the files each sentence is of; and what of Corbie's reading could not be
// read, which is Bulldog's own error.
type telling struct {
	says, more []string
	of         map[string]bool
	failed     []error
}

// model gives a worker's model, keeping what could not be read.
func (t *telling) model(w *office.Worker) (model, bool) {
	m, ok, err := modelOf(w)
	if err != nil {
		t.failed = append(t.failed, err)
	}
	return m, ok
}

// say says one sentence of the change, of some of its files. A sentence
// that lets more by is set apart.
func (t *telling) say(more bool, files []string, format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	into := &t.says
	if more {
		into = &t.more
	}
	if !slices.Contains(*into, s) {
		*into = append(*into, s)
	}
	for _, f := range files {
		t.of[f] = true
	}
}

// describe says what a change does, read from the office as it is and as
// the change would make it, and from nothing whoever drafted it said. Every
// file of the change is in at least one sentence. What of Corbie's reading
// could not be read is an error, since what the change lets by would
// otherwise be said as less than it is.
func describe(was, now reading, change []File) (says, more []string, err error) {
	t := &telling{of: map[string]bool{}}
	touched := map[string]bool{}
	for _, f := range change {
		touched[f.Path] = true
	}
	tellOffice(t, was.o, now.o)
	tellTeams(t, was.o, now.o)
	tellNotes(t, was.o, now.o, change)
	tellPeople(t, was.o, now.o)
	tellWorkers(t, was, now, touched)
	tellRules(t, was, now, touched)
	tellBridges(t, was, now, touched)
	for _, f := range change {
		if !t.of[f.Path] {
			t.say(false, nil, "%s is %s.", f.Path, f.Does())
		}
	}
	return t.says, t.more, errors.Join(t.failed...)
}

// called gives a team as a sentence names it: what it is called, as "the
// house" for "The house".
func called(o *office.Office, team string) string {
	t := o.Teams[team]
	if t == nil {
		return team
	}
	if rest, ok := strings.CutPrefix(t.Called, "The "); ok {
		return "the " + rest
	}
	return t.Called
}

// of gives the possessive of a name.
func of(name string) string {
	if strings.HasSuffix(name, "s") {
		return name + "'"
	}
	return name + "'s"
}

func tellOffice(t *telling, was, now *office.Office) {
	if was.Zone.String() != now.Zone.String() {
		t.say(false, []string{office.OfficeFile}, "the office's zone is now %s, where it was %s.", now.Zone, was.Zone)
	}
	if was.Runs != now.Runs {
		t.say(false, []string{office.OfficeFile}, "the office now has %d runs at work at once, where it had %d.", now.Runs, was.Runs)
	}
}

func tellTeams(t *telling, was, now *office.Office) {
	for _, name := range union(was.Order, now.Order) {
		a, b := was.Teams[name], now.Teams[name]
		switch {
		case a == nil:
			t.say(false, []string{b.File}, "%s is a new team, within %s.", called(now, name), called(now, b.Within))
			tellTeam(t, now, &office.Team{Name: name}, b, nil, seers(now, name))
			continue
		case b == nil:
			t.say(false, []string{a.File}, "%s is gone.", called(was, name))
			continue
		case a.Within != b.Within:
			t.say(false, []string{a.File, b.File}, "%s moves from within %s to within %s.", called(now, name), called(was, a.Within), called(now, b.Within))
		}
		tellTeam(t, now, a, b, seers(was, name), seers(now, name))
	}
}

// tellTeam says what changes in a team's file.
func tellTeam(t *telling, now *office.Office, a, b *office.Team, sawWas, sawNow []string) {
	name := called(now, b.Name)
	files := []string{b.File}
	if a.About != b.About && b.About != "" {
		t.say(false, files, "%s looks after: %s", name, b.About)
	} else if a.About != b.About {
		t.say(false, files, "%s says no longer what it looks after.", name)
	}
	if a.Called != b.Called && a.File != "" {
		t.say(false, files, "%s is now called %s, where it was called %s.", b.Name, b.Called, a.Called)
	}
	for _, p := range b.People {
		if !slices.Contains(a.People, p) {
			t.say(true, files, "%s is named in %s, and sees what it reads.", p, name)
		}
	}
	for _, p := range a.People {
		if !slices.Contains(b.People, p) && a.File != "" {
			t.say(false, files, "%s is no longer named in %s.", p, name)
		}
	}
	if len(b.People) == 0 {
		for _, p := range sawNow {
			if !slices.Contains(sawWas, p) {
				t.say(true, files, "%s now stands as one of %s people, since its decisions reach them, and sees what it reads.", p, of(name))
			}
		}
	}
	if a.Lead != b.Lead && b.Lead != "" {
		t.say(false, files, "%s is the lead of %s.", b.Lead, name)
	}
	keys := []string{}
	lists := func(team *office.Team) map[string][]office.Decider {
		m := map[string][]office.Decider{}
		for _, l := range team.Decide {
			m[l.Key] = l.Deciders
			if !slices.Contains(keys, l.Key) {
				keys = append(keys, l.Key)
			}
		}
		return m
	}
	la, lb := lists(a), lists(b)
	for _, key := range keys {
		da, hadA := la[key]
		db, hasB := lb[key]
		tellList(t, files, fmt.Sprintf("%s decides %s by", name, key), fmt.Sprintf("%s %s list", of(name), key), da, db, hadA, hasB, a.File == "")
	}
	tellList(t, files, fmt.Sprintf("%s release check is", of(name)), fmt.Sprintf("%s release check", of(name)), a.Release, b.Release, a.HasRelease, b.HasRelease, a.File == "")
	routine := func(r office.Routine) string { return r.At + "|" + r.To + "|" + r.Say }
	for _, r := range b.Routines {
		if !slices.ContainsFunc(a.Routines, func(x office.Routine) bool { return routine(x) == routine(r) }) {
			t.say(false, files, "%s has a routine: at %s, %s is told %q.", name, r.At, r.To, r.Say)
		}
	}
	for _, r := range a.Routines {
		if !slices.ContainsFunc(b.Routines, func(x office.Routine) bool { return routine(x) == routine(r) }) {
			t.say(false, files, "%s routine at %s, telling %s %q, is gone.", of(name), r.At, r.To, r.Say)
		}
	}
	model := func(team *office.Team, n string) (string, bool) {
		for _, m := range team.Models {
			if m.Name == n {
				return m.Text, true
			}
		}
		return "", false
	}
	var names []string
	for _, m := range append(slices.Clone(a.Models), b.Models...) {
		if !slices.Contains(names, m.Name) {
			names = append(names, m.Name)
		}
	}
	for _, n := range names {
		ma, hadA := model(a, n)
		mb, hasB := model(b, n)
		switch {
		case !hadA:
			t.say(false, files, "%s gives the model %s as %s.", name, n, mb)
		case !hasB:
			t.say(false, files, "%s gives the model %s no longer.", name, n)
		case ma != mb:
			t.say(false, files, "%s gives the model %s as %s, where it gave %s.", name, n, mb, ma)
		}
	}
}

// tellList says what changes in one list of deciders. A list lets more by
// when a decider is taken off it, when a rule or a worker comes before its
// first person who did not, or when it is taken away.
func tellList(t *telling, files []string, decides, list string, a, b []office.Decider, had, has, newTeam bool) {
	written := func(ds []office.Decider) string {
		var out []string
		for _, d := range ds {
			out = append(out, d.Written)
		}
		if len(out) == 0 {
			return "nobody"
		}
		return strings.Join(out, ", ")
	}
	switch {
	case !had && !has:
		return
	case had && !has:
		if !newTeam {
			t.say(true, files, "%s is taken away: it was %s.", list, written(a))
		}
		return
	case had && written(a) == written(b):
		return
	}
	// Who comes before the first person, in each.
	before := func(ds []office.Decider) []string {
		var out []string
		for _, d := range ds {
			if d.Kind == office.IsPerson {
				break
			}
			out = append(out, d.Name)
		}
		return out
	}
	more := false
	for _, d := range a {
		if !slices.ContainsFunc(b, func(x office.Decider) bool { return x.Name == d.Name }) {
			more = true
		}
	}
	for _, name := range before(b) {
		if !slices.Contains(before(a), name) {
			more = true
		}
	}
	if had {
		t.say(more, files, "%s %s, where it was %s.", decides, written(b), written(a))
	} else {
		t.say(more, files, "%s %s.", decides, written(b))
	}
}

// seers gives the people who see what a team reads: its own, or for a
// team with none, the people its decisions reach.
func seers(o *office.Office, team string) []string {
	t := o.Teams[team]
	if t == nil {
		return nil
	}
	if len(t.People) > 0 {
		return t.People
	}
	var out []string
	add := func(ds []office.Decider) {
		for _, d := range ds {
			if d.Kind == office.IsPerson && !slices.Contains(out, d.Name) {
				out = append(out, d.Name)
			}
		}
	}
	for _, l := range t.Decide {
		add(l.Deciders)
	}
	add(t.Release)
	for at := team; at != ""; at = o.Teams[at].Within {
		everything := o.Everything(at)
		add(everything)
		if slices.ContainsFunc(everything, func(d office.Decider) bool { return d.Kind == office.IsPerson }) || o.Teams[at] == nil {
			break
		}
	}
	return out
}

func tellNotes(t *telling, was, now *office.Office, change []File) {
	for _, f := range change {
		dir, name := path.Split(f.Path)
		dir = strings.TrimSuffix(dir, "/")
		if path.Base(dir) != office.NotesDir {
			continue
		}
		folder := strings.TrimSuffix(strings.TrimSuffix(dir, office.NotesDir), "/")
		o := now
		team := teamIn(o, folder)
		if team == "" {
			o, team = was, teamIn(was, folder)
		}
		if team == "" {
			continue
		}
		t.say(false, []string{f.Path}, "%s note %s is %s.", of(called(o, team)), strings.TrimSuffix(name, ".md"), f.Does())
	}
}

// teamIn gives the team whose folder a folder is, or nothing.
func teamIn(o *office.Office, dir string) string {
	for name, t := range o.Teams {
		if t.Dir == dir {
			return name
		}
	}
	return ""
}

func tellPeople(t *telling, was, now *office.Office) {
	files := []string{office.OfficeFile}
	for _, name := range union(was.Everyone, now.Everyone) {
		a, b := was.People[name], now.People[name]
		switch {
		case b == nil:
			t.say(false, files, "%s leaves the office.", name)
			continue
		case a == nil:
			t.say(false, files, "%s joins the office.", name)
			a = &office.Person{}
		}
		reached := func(p *office.Person, bridge string) (string, bool) {
			for _, id := range p.Reached {
				if id.Bridge == bridge {
					return id.ID, true
				}
			}
			return "", false
		}
		var bridges []string
		for _, id := range append(slices.Clone(a.Reached), b.Reached...) {
			if !slices.Contains(bridges, id.Bridge) {
				bridges = append(bridges, id.Bridge)
			}
		}
		for _, bridge := range bridges {
			ia, hadA := reached(a, bridge)
			ib, hasB := reached(b, bridge)
			switch {
			case !hadA:
				t.say(true, files, "%s is %s on %s.", name, ib, bridge)
			case !hasB:
				t.say(false, files, "%s is no longer reached on %s, where they were %s.", name, bridge, ia)
			case ia != ib:
				t.say(true, files, "%s is now %s on %s, where they were %s.", name, ib, bridge, ia)
			}
		}
	}
}

// A model is a worker's model, as Bulldog fills it in.
type model struct {
	Name, Service, Address, Secret string
}

func (m model) String() string {
	s := m.Name
	if m.Service != "" {
		s += " (" + m.Service + ")"
	}
	if m.Address != "" {
		s += " at " + m.Address
	}
	if m.Secret != "" {
		s += ", with the secret " + m.Secret
	}
	return s
}

// modelOf gives a worker's model as Corbie filled it out, or as Bulldog
// filled it in where Corbie did not, and whether it has one.
func modelOf(w *office.Worker) (model, bool, error) {
	if w.Draft != nil && w.Draft.Definition.Type == "agent" {
		var a agent.Agent
		if err := json.Unmarshal(w.Draft.Definition.Body, &a); err != nil {
			return model{}, false, fmt.Errorf("%s model cannot be read from Corbie's reading of it: %w", of(w.Name), err)
		}
		return model{a.Model.Name, a.Model.Service, a.Model.Address, a.Model.Secret}, true, nil
	}
	if w.Model == "" {
		return model{}, false, nil
	}
	var m struct{ Name, Service, Address, Secret string }
	if err := yaml.Unmarshal([]byte(w.Model), &m); err != nil {
		return model{}, false, fmt.Errorf("%s model, as Bulldog filled it in, cannot be read: %w", of(w.Name), err)
	}
	return model{m.Name, m.Service, m.Address, m.Secret}, true, nil
}

func tellWorkers(t *telling, was, now reading, touched map[string]bool) {
	// The addresses and secrets the office's models use now: a new worker
	// whose model uses one of them reaches nothing new.
	addresses, secrets := map[string]bool{}, map[string]bool{}
	for _, w := range was.o.Workers {
		if m, ok := t.model(w); ok {
			addresses[m.Address], secrets[m.Secret] = true, true
		}
	}
	for _, name := range union(workerNames(was.o), workerNames(now.o)) {
		a, b := was.o.Workers[name], now.o.Workers[name]
		switch {
		case b == nil:
			t.say(false, []string{a.File}, "%s leaves %s.", name, called(was.o, a.Team))
			continue
		case a == nil:
			files := []string{b.File}
			t.say(false, files, "%s joins %s.", name, called(now.o, b.Team))
			if m, ok := t.model(b); ok {
				t.say(m.Address != "" && !addresses[m.Address] || m.Secret != "" && !secrets[m.Secret], files, "%s model is %s.", of(name), m)
			}
			tellReach(t, files, name, reach{}, reachOf(now.o, b.File, b.Draft))
			tellTools(t, files, name, nil, b)
			continue
		}
		files := []string{b.File}
		if a.Team != b.Team {
			for _, f := range append(slices.Clone(a.Uses), b.Uses...) {
				if touched[f] {
					files = append(files, f)
				}
			}
			t.say(true, append(files, a.File), "%s moves from %s to %s, and sees what %s reads.", name, called(was.o, a.Team), called(now.o, b.Team), called(now.o, b.Team))
		}
		ma, okA := t.model(a)
		mb, okB := t.model(b)
		if okB && (!okA || ma != mb) {
			more := mb.Address != "" && mb.Address != ma.Address || mb.Secret != "" && mb.Secret != ma.Secret
			if okA {
				t.say(more, files, "%s model is now %s, where it was %s.", of(name), mb, ma)
			} else {
				t.say(more, files, "%s model is %s.", of(name), mb)
			}
		}
		// A worker Corbie could not read before reached nothing and held
		// no tool, since it was given no run; one it cannot read now has a
		// problem, and is said by it.
		if b.Draft != nil {
			var before *office.Worker
			var reached reach
			if a.Draft != nil {
				before, reached = a, reachOf(was.o, a.File, a.Draft)
			}
			tellReach(t, files, name, reached, reachOf(now.o, b.File, b.Draft))
			tellTools(t, files, name, before, b)
		}
	}
}

func workerNames(o *office.Office) []string {
	var names []string
	for _, team := range o.Order {
		names = append(names, o.Teams[team].Workers...)
	}
	return names
}

// tellReach says what a definition may reach that it could not, which
// lets more by, and what it may no longer reach. A refusal taken away lets
// more by where something it, or a machine it runs, is allowed takes in
// what was refused; one added narrows.
func tellReach(t *telling, files []string, who string, a, b reach) {
	for _, entry := range b.allow {
		if !slices.Contains(a.allow, entry) {
			t.say(true, files, "%s %s.", who, mayDo("may", entry))
		}
	}
	for _, entry := range a.allow {
		if !slices.Contains(b.allow, entry) {
			t.say(false, files, "%s %s.", who, mayDo("may no longer", entry))
		}
	}
	for _, entry := range fewer(a.deny, b.deny) {
		if permission.Declares(b.within, []string{entry}) {
			t.say(true, files, "%s %s, where it was refused.", who, mayDo("may", entry))
		} else {
			t.say(false, files, "%s is no longer refused what it is not allowed: %s.", who, entry)
		}
	}
	for _, entry := range fewer(b.deny, a.deny) {
		t.say(false, files, "%s is refused: it %s.", who, mayDo("may not", entry))
	}
}

// fewer gives the entries of one sorted list that the other has fewer of.
func fewer(a, b []string) []string {
	var out []string
	for i := 0; i < len(a); {
		j := i
		for j < len(a) && a[j] == a[i] {
			j++
		}
		if j-i > count(b, a[i]) {
			out = append(out, a[i])
		}
		i = j
	}
	return out
}

// count gives how often an entry is in a list.
func count(list []string, entry string) int {
	n := 0
	for _, e := range list {
		if e == entry {
			n++
		}
	}
	return n
}

// mayDo says what a permission lets a definition do, after a may.
func mayDo(may, entry string) string {
	kind, scope, _ := strings.Cut(entry, ":")
	switch kind {
	case "net":
		return may + " connect to " + scope
	case "fs.read":
		return may + " read " + scope
	case "fs.write":
		return may + " write " + scope
	case "exec":
		return may + " run " + scope
	case "env":
		return may + " read the environment's " + scope
	case "secret":
		name, to, ok := strings.Cut(scope, "@")
		if !ok {
			return may + " use the secret " + name
		}
		_, host, _ := strings.Cut(to, ":")
		if host == "" {
			host = to
		}
		return may + " send the secret " + name + " to " + host
	case "all":
		return may + " do anything"
	}
	return may + " use " + entry
}

// tellTools says each tool a worker gains or loses, and each that comes to
// wait for a yes, or no longer does, which lets more by.
func tellTools(t *telling, files []string, who string, a, b *office.Worker) {
	waitsA, waitsB := t.waits(a), t.waits(b)
	var had []string
	if a != nil {
		had = a.Tools
	}
	listed := map[string]bool{}
	for _, tool := range b.Held {
		listed[tool.Name] = tool.Listed
	}
	for _, tool := range b.Tools {
		switch {
		case !slices.Contains(had, tool) && listed[tool] && waitsB[tool]:
			t.say(false, files, "%s has the tools the server of %s lists, as %s_<tool>, which wait for a yes.", who, tool, tool)
		case !slices.Contains(had, tool) && listed[tool]:
			t.say(false, files, "%s has the tools the server of %s lists, as %s_<tool>.", who, tool, tool)
		case !slices.Contains(had, tool) && waitsB[tool]:
			t.say(false, files, "%s has the tool %s, which waits for a yes.", who, tool)
		case !slices.Contains(had, tool):
			t.say(false, files, "%s has the tool %s.", who, tool)
		case waitsA[tool] && !waitsB[tool]:
			t.say(true, files, "%s tool %s no longer waits for a yes.", of(who), tool)
		case !waitsA[tool] && waitsB[tool]:
			t.say(false, files, "%s tool %s now waits for a yes.", of(who), tool)
		}
	}
	for _, tool := range had {
		if !slices.Contains(b.Tools, tool) {
			t.say(false, files, "%s no longer has the tool %s.", who, tool)
		}
	}
}

// waits gives which of an agent's tools wait for a yes, keeping what
// could not be read.
func (t *telling) waits(w *office.Worker) map[string]bool {
	out, err := Waits(w)
	if err != nil {
		t.failed = append(t.failed, err)
	}
	return out
}

// Waits gives which of a worker's tools wait for a yes, as Corbie read it:
// those marked for approval, and those that declare a permission the agent
// lists to be asked about, a script's declaring the agent's own allow with
// its own, as Corbie holds it. A worker that is not an agent, or that
// Corbie could not read, has none.
func Waits(w *office.Worker) (map[string]bool, error) {
	out := map[string]bool{}
	if w == nil || w.Draft == nil || w.Draft.Definition.Type != "agent" {
		return out, nil
	}
	var a agent.Agent
	if err := json.Unmarshal(w.Draft.Definition.Body, &a); err != nil {
		return out, fmt.Errorf("which of %s tools wait for a yes cannot be read from Corbie's reading of it: %w", of(w.Name), err)
	}
	def := w.Draft.Definition
	for name, tool := range a.Tools {
		if tool.Job == nil {
			continue
		}
		allow := tool.Job.Allow
		if !tool.Job.Alone && !tool.Job.IsMachine() {
			allow = append(slices.Clone(def.Allow), allow...)
		}
		out[name] = tool.Job.Approve || permission.Declares(allow, def.AskAbout)
	}
	return out, nil
}

// changedIn says whether any file differs between two readings.
func changedIn(was, now reading, files []string) bool {
	for _, f := range files {
		a, hadA := was.files[f]
		b, hasB := now.files[f]
		if hadA != hasB || string(a) != string(b) {
			return true
		}
	}
	return false
}

func tellRules(t *telling, was, now reading, touched map[string]bool) {
	lists := func(o *office.Office, rule string) []string {
		r := o.Rules[rule]
		team := o.Teams[r.Team]
		var on []string
		for _, l := range team.Decide {
			if slices.ContainsFunc(l.Deciders, func(d office.Decider) bool { return d.Name == rule }) {
				on = append(on, l.Key+" list")
			}
		}
		if slices.ContainsFunc(team.Release, func(d office.Decider) bool { return d.Name == rule }) {
			on = append(on, "release check")
		}
		return on
	}
	for _, rule := range union(sortedKeys(was.o.Rules), sortedKeys(now.o.Rules)) {
		a, b := was.o.Rules[rule], now.o.Rules[rule]
		switch {
		case b == nil:
			t.say(false, []string{rule}, "%s rule %s is on none of its lists.", of(called(was.o, a.Team)), rule)
			continue
		case a == nil:
			t.say(false, []string{rule}, "%s rule %s is new, on its %s.", of(called(now.o, b.Team)), rule, strings.Join(lists(now.o, rule), " and "))
			tellReach(t, []string{rule}, of(called(now.o, b.Team))+" rule "+rule, reach{}, reachOf(now.o, rule, b.Draft))
			continue
		}
		files := union(uses(rule, a.Draft), uses(rule, b.Draft))
		var said []string
		for _, f := range files {
			if touched[f] {
				said = append(said, f)
			}
		}
		if changedIn(was, now, files) {
			t.say(true, said, "%s rule %s is changed, and decides on its %s: what a rule decides is read by no one but itself.", of(called(now.o, b.Team)), rule, strings.Join(lists(now.o, rule), " and "))
		}
		// A rule Corbie could not read before reached nothing, since it
		// could not be started.
		if b.Draft != nil {
			tellReach(t, said, of(called(now.o, b.Team))+" rule "+rule, reachOf(was.o, rule, a.Draft), reachOf(now.o, rule, b.Draft))
		}
	}
}

func tellBridges(t *telling, was, now reading, touched map[string]bool) {
	for _, name := range union(was.o.BridgeOrder, now.o.BridgeOrder) {
		a, b := was.o.Bridges[name], now.o.Bridges[name]
		dir := "bridges/" + name + "/"
		var files []string
		for f := range touched {
			if strings.HasPrefix(f, dir) {
				files = append(files, f)
			}
		}
		slices.Sort(files)
		settingsFiles := append(slices.Clone(files), office.OfficeFile)
		switch {
		case b == nil:
			t.say(false, settingsFiles, "the bridge %s is gone.", name)
			continue
		case a == nil:
			t.say(true, settingsFiles, "the bridge %s is new: what it lets in is its in machine's, which Bulldog does not read.", name)
			a = &office.Bridge{Name: name}
		}
		for _, which := range []string{"in", "out"} {
			file := dir + which + ".yaml"
			da, db := machineOf(a, which), machineOf(b, which)
			if was.o.Bridges[name] != nil && changedIn(was, now, union(uses(file, da), uses(file, db))) {
				t.say(true, files, "%s %s machine is changed.", of(name), which)
			}
			tellReach(t, files, of(name)+" "+which+" machine", reachOf(was.o, file, da), reachOf(now.o, file, db))
		}
		if was.o.Bridges[name] != nil && !reflect.DeepEqual(a.Settings, b.Settings) {
			t.say(true, []string{office.OfficeFile}, "%s settings are changed: Bulldog reads only the places they list.", of(name))
		}
		place := func(p office.Place) string {
			guests := "where only the people of the office may ask"
			if p.Guests {
				guests = "where guests may ask"
			}
			return fmt.Sprintf("%s listens in %s, for %s, %s", name, p.Name, called(now.o, p.Team), guests)
		}
		for _, p := range b.Places {
			i := slices.IndexFunc(a.Places, func(x office.Place) bool { return x.Name == p.Name })
			switch {
			case i < 0:
				t.say(false, []string{office.OfficeFile}, "%s.", place(p))
			case a.Places[i] != p:
				t.say(false, []string{office.OfficeFile}, "%s, where it was for %s.", place(p), called(was.o, a.Places[i].Team))
			}
		}
		for _, p := range a.Places {
			if !slices.ContainsFunc(b.Places, func(x office.Place) bool { return x.Name == p.Name }) {
				t.say(false, []string{office.OfficeFile}, "%s no longer listens in %s.", name, p.Name)
			}
		}
	}
}

// machineOf gives a bridge's in or out machine as Corbie read it, or nil.
func machineOf(b *office.Bridge, which string) *machine.Draft {
	if which == "in" {
		return b.In
	}
	return b.Out
}

// union gives the names in either list, in the order first seen.
func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
