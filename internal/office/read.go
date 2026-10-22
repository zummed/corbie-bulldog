package office

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie/permission"
)

var (
	teamKeys = []string{"name", "about", "people", "lead", "heed", "check", "decide", "routines", "models"}
	topKeys  = []string{"zone", "everyone", "bridges"}
	reserved = []string{"office", "team", "bulldog", "chat", "guest", "wait", "ask", "clock"}
	types    = []string{"agent", "flow", "statechart"}
	weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	nameForm = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

const idFile = ".bulldog/id"

// Read reads the office in a directory, with what Bulldog ships served
// at ./.bulldog/. It makes the office's id the first time.
//
// A problem of office.yaml, of a team file or of the office's id refuses
// the reading: Read gives no office, and every problem found, as Problems.
// A problem of a worker, a rule or a bridge stops only it and the teams
// that use it, and is in the office's Problems.
func Read(dir string, shipped fs.FS) (*Office, error) {
	if _, err := os.Stat(filepath.Join(dir, "office.yaml")); errors.Is(err, fs.ErrNotExist) {
		return nil, Problems{{File: "office.yaml", Text: dir + " holds no office.yaml, so it is not an office"}}
	}
	r := &reader{o: &Office{Dir: dir, Zone: time.UTC, shipped: shipped}, nodes: map[*Team]*yaml.Node{}, used: map[string]string{}}
	o := r.o
	r.id()
	r.gather(&Team{ID: "office", File: "office.yaml"})
	top := o.Teams[0]
	r.zone(value(r.nodes[top], "zone"))
	bridges := r.pairs(refuses, top.File, value(r.nodes[top], "bridges"), "bridges")
	for _, p := range bridges {
		o.Bridges = append(o.Bridges, &Bridge{Name: p.key})
	}
	r.people(value(r.nodes[top], "everyone"))
	for _, t := range o.Teams[1:] {
		r.name(t.ID, "a team", refuses, t.File, nil)
	}
	for _, t := range o.Teams {
		for _, w := range t.Workers {
			r.name(w.Name, "a worker", stops{worker: w.Name}, w.File, nil)
		}
	}
	for i, p := range bridges {
		r.bridge(o.Bridges[i], p)
	}
	for _, t := range o.Teams {
		if n, ok := r.nodes[t]; ok {
			r.team(t, n)
		}
		if t.Name == "" {
			t.Name = t.ID
		}
		if len(t.People) == 0 {
			t.People = slices.Clone(top.People)
		}
	}
	o.Name = top.Name
	for _, t := range o.Teams {
		for i := range t.Workers {
			r.worker(t, &t.Workers[i])
		}
	}
	return r.done()
}

// A reader reads one office, and gathers its problems.
type reader struct {
	o     *Office
	nodes map[*Team]*yaml.Node // each team file that could be read, as YAML
	used  map[string]string    // what each name of a person, team or worker names
	found []found
}

// stops says what a problem stops: a team, a worker with the teams that
// use it, or a bridge. A problem that stops none of them refuses the
// reading.
type stops struct{ team, worker, bridge string }

var refuses = stops{}

type found struct {
	Problem
	stops stops
}

func (r *reader) problem(s stops, file string, n *yaml.Node, format string, args ...any) {
	p := Problem{File: file, Text: fmt.Sprintf(format, args...)}
	if n != nil {
		p.Line = n.Line
	}
	r.found = append(r.found, found{p, s})
}

// done gives each problem to what it stops, and gives the office, or
// every problem when one of them refuses the reading.
func (r *reader) done() (*Office, error) {
	o := r.o
	slices.SortStableFunc(r.found, func(a, b found) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
	refused := false
	for _, f := range r.found {
		o.Problems = append(o.Problems, f.Problem)
		switch {
		case f.stops.bridge != "":
			b := o.Bridge(f.stops.bridge)
			b.Problems = append(b.Problems, f.Problem)
		case f.stops.team != "":
			t := o.Team(f.stops.team)
			t.Problems = append(t.Problems, f.Problem)
		case f.stops.worker != "":
			for _, t := range o.Teams {
				if t.uses(f.stops.worker) {
					t.Problems = append(t.Problems, f.Problem)
				}
			}
		default:
			refused = true
		}
	}
	if refused {
		return nil, Problems(o.Problems)
	}
	return o, nil
}

// uses says whether a team uses a worker: one of its own, or one it names
// as a decider.
func (t *Team) uses(worker string) bool {
	for _, w := range t.Workers {
		if w.Name == worker {
			return true
		}
	}
	for _, d := range slices.Concat(t.Heed, t.Check, t.Decide) {
		if d.Worker == worker {
			return true
		}
	}
	return false
}

// id reads the office's id, and makes it the first time: 16 random hex
// digits.
func (r *reader) id() {
	root, err := os.OpenRoot(r.o.Dir)
	if err != nil {
		r.problem(refuses, idFile, nil, "%v", err)
		return
	}
	defer root.Close()
	written, err := root.ReadFile(idFile)
	if errors.Is(err, fs.ErrNotExist) {
		b := make([]byte, 8)
		rand.Read(b)
		written = []byte(hex.EncodeToString(b) + "\n")
		err = root.Mkdir(".bulldog", 0o755)
		if err == nil || errors.Is(err, fs.ErrExist) {
			err = root.WriteFile(idFile, written, 0o644)
		}
	}
	if err != nil {
		r.problem(refuses, idFile, nil, "%v", err)
		return
	}
	id := strings.TrimSpace(string(written))
	if strings.Contains(id, "/") || permission.CheckKeptPath(id) != nil {
		r.problem(refuses, idFile, nil, "the office's id is %q, and an id is one part of a kept path: small letters, digits, ., - and _, not starting with a dot", id)
		return
	}
	r.o.ID = id
}

// gather finds a team, its workers and the teams within it, and reads its
// team file as YAML.
func (r *reader) gather(t *Team) {
	o := r.o
	o.Teams = append(o.Teams, t)
	if src, err := readIn(o.Dir, t.File); err != nil {
		r.problem(refuses, t.File, nil, "%v", err)
	} else if n, ok := r.document(refuses, t.File, src); ok {
		r.nodes[t] = n
	}
	entries, err := os.ReadDir(filepath.Join(o.Dir, filepath.FromSlash(t.Path)))
	if err != nil {
		r.problem(refuses, t.File, nil, "%v", err)
		return
	}
	var within []*Team
	for _, e := range entries {
		file := path.Join(t.Path, e.Name())
		switch {
		case strings.HasPrefix(e.Name(), "."):
		case e.IsDir():
			if _, err := os.Stat(filepath.Join(o.Dir, filepath.FromSlash(file), "team.yaml")); err == nil {
				within = append(within, &Team{ID: e.Name(), Path: file, File: file + "/team.yaml", Within: t.ID})
			}
		case strings.HasSuffix(e.Name(), ".yaml") && file != t.File:
			t.Workers = append(t.Workers, Worker{Name: strings.TrimSuffix(e.Name(), ".yaml"), File: file, Team: t.ID})
		}
	}
	for _, w := range within {
		r.gather(w)
	}
}

// zone reads office.yaml's zone.
func (r *reader) zone(n *yaml.Node) {
	name := r.text(refuses, "office.yaml", n, "zone")
	if name == "" {
		return
	}
	zone, err := time.LoadLocation(name)
	if err != nil || name == "Local" {
		r.problem(refuses, "office.yaml", n, "zone: %s is not a time zone; a zone is named as Europe/London is", name)
		return
	}
	r.o.Zone = zone
}

// people reads everyone in office.yaml.
func (r *reader) people(n *yaml.Node) {
	named := map[string]string{} // the person each identity names, by bridge and identity
	for _, p := range r.pairs(refuses, "office.yaml", n, "everyone") {
		r.name(p.key, "a person", refuses, "office.yaml", p.k)
		person := Person{ID: p.key, Name: p.key, Identities: map[string]string{}}
		for _, e := range r.pairs(refuses, "office.yaml", p.v, p.key) {
			switch {
			case e.key == "name":
				person.Name = cmp.Or(r.text(refuses, "office.yaml", e.v, "name"), p.key)
			case r.o.Bridge(e.key) != nil:
				id := r.text(refuses, "office.yaml", e.v, e.key)
				both := e.key + " " + id
				switch {
				case id == "":
					r.problem(refuses, "office.yaml", e.k, "%s: %s is written with no id; under a bridge's name is the person's lasting id there", p.key, e.key)
				case named[both] != "":
					r.problem(refuses, "office.yaml", e.v, "%s %s names both %s and %s; one identity names one person", e.key, id, named[both], p.key)
				default:
					named[both] = p.key
					person.Identities[e.key] = id
				}
			default:
				r.problem(refuses, "office.yaml", e.k, "%s: %s is not a bridge of the office; a person's entry holds name and, under each bridge's name, their lasting id there", p.key, e.key)
			}
		}
		r.o.People = append(r.o.People, person)
	}
}

// name checks a name of a person, a team or a worker: its form, that it
// is not reserved, and that nothing else in the office has it.
func (r *reader) name(name, what string, s stops, file string, n *yaml.Node) {
	switch {
	case !nameForm.MatchString(name):
		r.problem(s, file, n, "%s is not a name %s can have: a name starts with a small letter, and holds small letters, figures and dashes", name, what)
	case slices.Contains(reserved, name):
		r.problem(s, file, n, "%s is reserved, and is not a name %s can have (the reserved names are %s)", name, what, strings.Join(reserved, ", "))
	case r.used[name] != "":
		r.problem(s, file, n, "%s is the name of %s already; each name is used once in the office", name, r.used[name])
	default:
		r.used[name] = what
	}
}

// bridge reads a bridge's entry in bridges, and finds its directory.
func (r *reader) bridge(b *Bridge, entry pair) {
	s := stops{bridge: b.Name}
	at := entry.k
	b.Settings = map[string]any{}
	var places *yaml.Node
	for _, e := range r.pairs(s, "office.yaml", entry.v, b.Name) {
		if e.key == "from" {
			b.From, at = r.text(s, "office.yaml", e.v, "from"), e.v
			continue
		}
		var v any
		if err := e.v.Decode(&v); err != nil {
			r.problem(s, "office.yaml", e.v, "%s: %v", e.key, err)
		}
		b.Settings[e.key] = v
		if e.key == "places" {
			places = e.v
		}
	}
	for _, p := range r.pairs(s, "office.yaml", places, "places") {
		place := Place{Name: p.key, Team: "office"}
		for _, e := range r.pairs(s, "office.yaml", p.v, p.key) {
			switch e.key {
			case "team":
				place.Team = cmp.Or(r.text(s, "office.yaml", e.v, "team"), "office")
				if r.o.Team(place.Team) == nil {
					r.problem(s, "office.yaml", e.v, "%s: team %s is not a team of the office", p.key, place.Team)
				}
			case "guests":
				if e.v.Tag != "!!bool" || e.v.Decode(&place.Guests) != nil {
					r.problem(s, "office.yaml", e.v, "%s: guests is true or false", p.key)
				}
			}
		}
		b.Places = append(b.Places, place)
	}
	switch {
	case b.From == "":
		b.Dir = "bridges/" + b.Name
	case !relative(b.From):
		r.problem(s, "office.yaml", at, "from: %s is a path from the office's top, starting ./, such as ./.bulldog/bridges/slack", b.From)
		return
	default:
		b.Dir = path.Clean(b.From)
	}
	for _, f := range []string{"in.yaml", "out.yaml"} {
		if _, err := r.o.Load(b.Dir + "/" + f); err != nil {
			r.problem(s, "office.yaml", at, "%s: ./%s/%s cannot be read, and a bridge is a directory with in.yaml and out.yaml: %v", b.Name, b.Dir, f, err)
		}
	}
}

// team reads a team file.
func (r *reader) team(t *Team, n *yaml.Node) {
	hasLead := false
	for _, p := range r.pairs(refuses, t.File, n, "a team file") {
		switch p.key {
		case "name":
			t.Name = r.text(refuses, t.File, p.v, "name")
		case "about":
			t.About = r.text(refuses, t.File, p.v, "about")
		case "people":
			for _, item := range r.list(refuses, t.File, p.v, "people") {
				if r.o.Person(item.Value) == nil {
					r.problem(refuses, t.File, item, "people: %s is not a person of the office; every person is written in everyone, in office.yaml", item.Value)
					continue
				}
				t.People = append(t.People, item.Value)
			}
		case "lead":
			hasLead = true
			r.lead(t, p.v)
		case "heed":
			t.Heed = r.deciders(t, p.v, "heed")
		case "check":
			t.Check = r.deciders(t, p.v, "check")
		case "decide":
			t.Decide = r.deciders(t, p.v, "decide")
		case "routines":
			r.routines(t, p.v)
		case "models":
			r.models(t, p.v)
		case "zone", "everyone", "bridges":
			if t.Within != "" {
				r.problem(refuses, t.File, p.k, "%s is written in office.yaml only", p.key)
			}
		default:
			r.problem(refuses, t.File, p.k, "%s is not a key of a team file (a team file takes %s, and office.yaml takes %s as well)",
				p.key, strings.Join(teamKeys, ", "), strings.Join(topKeys, ", "))
		}
	}
	if !hasLead {
		r.problem(refuses, t.File, nil, "%s has no lead; every team has one, a worker of the team or a person", t.ID)
	}
}

// lead reads a team's lead.
func (r *reader) lead(t *Team, n *yaml.Node) {
	name := r.text(refuses, t.File, n, "lead")
	w := r.o.Worker(name)
	switch {
	case name == "":
		r.problem(refuses, t.File, n, "lead names no one; a team's lead is a worker of the team or a person")
	case w != nil && w.Team == t.ID:
		t.Lead.Worker = name
	case w != nil:
		r.problem(refuses, t.File, n, "lead: %s is a worker of %s; a team's lead is a worker of its own, or a person", name, w.Team)
	case r.o.Person(name) != nil:
		t.Lead.Person = name
	default:
		r.problem(refuses, t.File, n, "lead: %s is neither a worker of %s nor a person", name, t.ID)
	}
}

// deciders reads a decision: an ordered list of rules, workers and
// people, in which people may only come last.
func (r *reader) deciders(t *Team, n *yaml.Node, what string) []Decider {
	var list []Decider
	person := "" // the first person of the list
	for _, item := range r.list(refuses, t.File, n, what) {
		var d Decider
		switch word := item.Value; {
		case relative(word):
			d.Rule = r.rule(t, item)
		case r.o.Worker(word) != nil:
			d.Worker = word
		case r.o.Person(word) != nil:
			d.Person = word
		default:
			r.problem(refuses, t.File, item, "%s: %s is neither a rule nor a worker nor a person; a rule is named by its path, starting ./", what, word)
			continue
		}
		if person != "" && d.Person == "" {
			r.problem(refuses, t.File, item, "%s: %s comes after %s, and people may only end a list", what, item.Value, person)
		}
		if person == "" {
			person = d.Person
		}
		list = append(list, d)
	}
	return list
}

// rule finds a rule by its path from its team file's directory, and gives
// its path from the office's top. A rule that cannot be read is a problem
// of the rule, and stops the team.
func (r *reader) rule(t *Team, n *yaml.Node) string {
	s := stops{team: t.ID}
	file, shipped, err := served(path.Join(t.Path, n.Value))
	switch {
	case err != nil:
		r.problem(s, t.File, n, "%s: %v", n.Value, err)
		return n.Value
	case shipped:
		file = ".bulldog/" + file
	case strings.HasSuffix(file, ".yaml"):
		r.problem(s, t.File, n, "%s is a definition of the office's own: a rule is a script, or a definition Bulldog ships, and a worker is named by its name", n.Value)
		return "./" + file
	}
	if _, err := r.o.Load(file); err != nil {
		r.problem(s, t.File, n, "%s cannot be read: %v", n.Value, err)
	}
	return "./" + file
}

// routines reads a team's routines.
func (r *reader) routines(t *Team, n *yaml.Node) {
	for _, item := range r.items(refuses, t.File, n, "routines") {
		var rt Routine
		for _, e := range r.pairs(refuses, t.File, item, "a routine") {
			switch e.key {
			case "at":
				rt.At = r.text(refuses, t.File, e.v, "at")
			case "say":
				rt.Say = r.text(refuses, t.File, e.v, "say")
			default:
				r.problem(refuses, t.File, e.k, "%s is not a key of a routine; a routine is { at, say }", e.key)
			}
		}
		switch {
		case rt.At == "" || rt.Say == "":
			r.problem(refuses, t.File, item, "a routine gives at and say, such as { at: \"mon 09:00\", say: \"What is still open?\" }")
		case !timeOfDay(rt.At):
			r.problem(refuses, t.File, item, "at: %s is not a time; write a time of day, such as \"18:00\", after a weekday if it has one, such as \"mon 09:00\"", rt.At)
		}
		t.Routines = append(t.Routines, rt)
	}
}

// timeOfDay says whether a routine's at is a time of day, after a weekday
// if it gives one: "18:00", "mon 09:00".
func timeOfDay(at string) bool {
	parts := strings.Fields(at)
	if len(parts) == 2 && slices.Contains(weekdays, parts[0]) {
		parts = parts[1:]
	}
	if len(parts) != 1 {
		return false
	}
	clock, err := time.Parse("15:04", parts[0])
	return err == nil && clock.Format("15:04") == parts[0]
}

// models reads a team's models.
func (r *reader) models(t *Team, n *yaml.Node) {
	for _, p := range r.pairs(refuses, t.File, n, "models") {
		var model map[string]any
		if p.v.Kind != yaml.MappingNode || p.v.Decode(&model) != nil {
			r.problem(refuses, t.File, p.v, "models: %s is a model as Corbie writes one, such as { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }", p.key)
			continue
		}
		if t.Models == nil {
			t.Models = map[string]map[string]any{}
		}
		t.Models[p.key] = model
	}
}

// worker reads a worker's first line, its type and its name, and finds the
// model it names by one word.
func (r *reader) worker(t *Team, w *Worker) {
	s := stops{worker: w.Name}
	src, err := readIn(r.o.Dir, w.File)
	if err != nil {
		r.problem(s, w.File, nil, "%v", err)
		return
	}
	n, ok := r.document(s, w.File, src)
	if !ok {
		return
	}
	if n == nil || n.Kind != yaml.MappingNode || len(n.Content) == 0 {
		r.problem(s, w.File, n, "a worker is a Corbie definition, whose first line gives its type and its name, such as agent: %s", w.Name)
		return
	}
	kind, name := n.Content[0], n.Content[1]
	if !slices.Contains(types, kind.Value) {
		r.problem(s, w.File, kind, "the first line gives %s; a worker's first line gives its type, agent, flow or statechart, and its name, such as agent: %s", kind.Value, w.Name)
		return
	}
	w.Type = kind.Value
	if name.Value != w.Name {
		r.problem(s, w.File, name, "the definition's name is %s and its file's is %s; a worker's definition has its file's name", name.Value, w.Name)
	}
	if word := modelWord(n); word != nil {
		if _, ok := r.o.Model(t, word.Value); !ok {
			r.problem(s, w.File, word, "%v", noModel(t, word.Value))
		}
	}
}

// document reads a file as YAML, and gives the node it holds: nil for a
// file that holds nothing. A file that is not YAML is a problem, and gives
// false.
func (r *reader) document(s stops, file string, src []byte) (*yaml.Node, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		p := Problem{File: file, Text: err.Error()}
		// The YAML reader says where: "yaml: line 3: did not find expected key".
		if rest, ok := strings.CutPrefix(p.Text, "yaml: line "); ok {
			if at, text, ok := strings.Cut(rest, ": "); ok {
				if line, err := strconv.Atoi(at); err == nil {
					p.Line, p.Text = line, text
				}
			}
		}
		r.found = append(r.found, found{p, s})
		return nil, false
	}
	if len(doc.Content) == 0 {
		return nil, true
	}
	return doc.Content[0], true
}

type pair struct {
	key  string
	k, v *yaml.Node
}

// pairs gives the keys and values of a mapping in the order written.
// Nothing written is an empty mapping. Anything else that is not a
// mapping, and a key written twice, is a problem.
func (r *reader) pairs(s stops, file string, n *yaml.Node, what string) []pair {
	if n == nil || n.Tag == "!!null" {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		r.problem(s, file, n, "%s is written as a mapping, and is %s", what, describe(n))
		return nil
	}
	var out []pair
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if slices.ContainsFunc(out, func(p pair) bool { return p.key == k.Value }) {
			r.problem(s, file, k, "%s is written twice in %s", k.Value, what)
			continue
		}
		out = append(out, pair{k.Value, k, v})
	}
	return out
}

// items gives the items of a list. Nothing written is an empty list, and
// anything else that is not a list is a problem.
func (r *reader) items(s stops, file string, n *yaml.Node, what string) []*yaml.Node {
	if n == nil || n.Tag == "!!null" {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		r.problem(s, file, n, "%s is written as a list, such as [a, b], and is %s", what, describe(n))
		return nil
	}
	return n.Content
}

// list gives the items of a list of words: names and paths.
func (r *reader) list(s stops, file string, n *yaml.Node, what string) []*yaml.Node {
	var words []*yaml.Node
	for _, item := range r.items(s, file, n, what) {
		if item.Kind != yaml.ScalarNode || item.Value == "" {
			r.problem(s, file, item, "%s: each item is a name or a path, and this is %s", what, describe(item))
			continue
		}
		words = append(words, item)
	}
	return words
}

// text gives a value written as text; nothing written is "". Anything
// else is a problem.
func (r *reader) text(s stops, file string, n *yaml.Node, what string) string {
	if n == nil || n.Tag == "!!null" {
		return ""
	}
	if n.Kind != yaml.ScalarNode {
		r.problem(s, file, n, "%s is written as text, and is %s", what, describe(n))
		return ""
	}
	return n.Value
}

// value gives the value of a key of a mapping, or nil.
func value(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// describe says what a node is, for a problem to say what was found.
func describe(n *yaml.Node) string {
	switch {
	case n == nil || n.Tag == "!!null":
		return "nothing"
	case n.Kind == yaml.MappingNode:
		return "a mapping"
	case n.Kind == yaml.SequenceNode:
		return "a list"
	}
	return strconv.Quote(n.Value)
}

func relative(name string) bool {
	return strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../")
}
