// Package office reads an office directory: its teams, people, workers,
// bridges, routines and models, with every problem found together. It
// serves the office's files, and what Bulldog ships, to Corbie's parser.
package office

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// An Office is one reading of an office directory. Read makes it, and
// nothing changes it afterwards but Stop.
type Office struct {
	Dir     string         // the office's directory
	ID      string         // the office's id, kept in .bulldog/id
	Name    string         // what office.yaml calls the office
	Zone    *time.Location // office.yaml's zone; UTC without one
	People  []Person       // everyone, in the order written
	Bridges []*Bridge      // in the order written
	Teams   []*Team        // the office first, then each team within, before the teams within it
	// Problems is every problem of the reading. Each stops only the
	// worker, rule or bridge it is of, and the teams that use it.
	Problems []Problem

	shipped fs.FS
}

// A Person is an entry of everyone in office.yaml.
type Person struct {
	ID         string            // the entry's key: sam
	Name       string            // what the entry calls them: Sam; the key without one
	Identities map[string]string // their lasting id on each bridge, by the bridge's name
}

// A Team is a directory with a team file. The top team is the office.
type Team struct {
	ID     string // its directory's name; office for the top team
	Path   string // its directory from the office's top: "" for the office, garden/beds
	File   string // its team file from the office's top: office.yaml, garden/team.yaml
	Within string // the team whose directory holds its own; "" for the office
	Name   string // what its file calls it; its ID without one
	About  string
	// People are the team's people: those its file names, or the
	// office's when it names none. They are asked its decisions and see
	// its requests.
	People   []string
	Lead     Lead
	Heed     []Decider // each decision is empty when the file does not write it
	Check    []Decider
	Decide   []Decider
	Routines []Routine
	// Models are models as Corbie writes them, by the one word a worker
	// names one by.
	Models   map[string]map[string]any
	Workers  []Worker  // the definitions directly in its directory, by file name
	Problems []Problem // what stops it: a team with a problem takes no requests
}

// A Lead takes every request to a team: a worker of the team, or a
// person. One field is set.
type Lead struct {
	Worker string
	Person string
}

// A Decider is one entry of a decision's list. One field is set.
type Decider struct {
	// Rule is a script, or a definition Bulldog ships, by its path from
	// the office's top: ./payments/no-account-numbers.js,
	// ./.bulldog/rules/for-us/for-us.yaml.
	Rule   string
	Worker string
	Person string
}

// A Routine is said to a team's lead at a time: a time of day, after a
// weekday if At gives one ("mon 09:00", "18:00"), in the office's zone.
type Routine struct {
	At  string
	Say string
}

// A Worker is a Corbie definition directly in a team's directory.
type Worker struct {
	Name string // its file's name without .yaml, and its definition's name
	Type string // agent, flow or statechart; "" when the file gives none
	File string // its file from the office's top: garden/gardener.yaml
	Team string // the team whose directory it is in
}

// A Bridge is a directory with in.yaml and out.yaml, named in bridges in
// office.yaml.
type Bridge struct {
	Name     string         // its key in bridges
	From     string         // from as written; "" for the office's own, at bridges/<name>
	Dir      string         // its directory from the office's top: .bulldog/bridges/slack, bridges/pager
	Settings map[string]any // its entry but from: what its in and its out are given
	Places   []Place        // in the order written
	Problems []Problem      // what stops it: a bridge with a problem hears nothing and sends nothing
}

// A Place is where a bridge takes requests.
type Place struct {
	Name   string // as the settings name it, such as "#help"; direct for direct messages
	Team   string // the team asked there; office without one
	Guests bool   // whether guests may ask there
}

// A Problem is one thing wrong in the office, at its file and, where YAML
// gives one, its line.
type Problem struct {
	File string `json:"file"` // from the office's top
	Line int    `json:"line,omitzero"`
	Text string `json:"text"`
}

func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Text)
	}
	return p.File + ": " + p.Text
}

// Problems are every problem of a reading, one to a line.
type Problems []Problem

func (ps Problems) Error() string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n")
}

// Team gives the team of an ID, or nil.
func (o *Office) Team(id string) *Team {
	for _, t := range o.Teams {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// Worker gives the worker of a name, of any team, or nil. Its Team is the
// team it is of.
func (o *Office) Worker(name string) *Worker {
	for _, t := range o.Teams {
		for i := range t.Workers {
			if t.Workers[i].Name == name {
				return &t.Workers[i]
			}
		}
	}
	return nil
}

// Person gives the person of an ID, or nil.
func (o *Office) Person(id string) *Person {
	for i := range o.People {
		if o.People[i].ID == id {
			return &o.People[i]
		}
	}
	return nil
}

// PersonOn gives the person whose entry gives an identity on a bridge, or
// nil. Identities are compared exactly as written.
func (o *Office) PersonOn(bridge, id string) *Person {
	for i := range o.People {
		if got, ok := o.People[i].Identities[bridge]; ok && got == id {
			return &o.People[i]
		}
	}
	return nil
}

// Bridge gives the bridge of a name, or nil.
func (o *Office) Bridge(name string) *Bridge {
	for _, b := range o.Bridges {
		if b.Name == name {
			return b
		}
	}
	return nil
}

// Model gives the model a worker of a team names by one word: the team's
// own first, then each team around it, the office's last.
func (o *Office) Model(t *Team, word string) (map[string]any, bool) {
	for ; t != nil; t = o.Team(t.Within) {
		if model, ok := t.Models[word]; ok {
			return model, true
		}
	}
	return nil, false
}

// MayAsk gives the people a team may ask: its people, and each person its
// team file names as its lead or as a decider.
func (t *Team) MayAsk() []string {
	people := slices.Clone(t.People)
	add := func(person string) {
		if person != "" && !slices.Contains(people, person) {
			people = append(people, person)
		}
	}
	add(t.Lead.Person)
	for _, d := range slices.Concat(t.Heed, t.Check, t.Decide) {
		add(d.Person)
	}
	return people
}

// Docs gives the kept path a team's docs are under: <office id>/<team>/docs.
func (o *Office) Docs(team string) string {
	return o.ID + "/" + team + "/docs"
}

// Allow gives the list every run Bulldog starts for a team is started
// with: everything but kept values, and those only under the team's own
// path.
func (o *Office) Allow(team string) []string {
	kept := o.ID + "/" + team + "/**"
	return []string{"network", "disk", "exec", "env", "secret:*@*", "kept.read:" + kept, "kept.write:" + kept}
}

// Stop adds problems found of a team after the reading, such as those
// Corbie's parser finds in its request machine, and so stops the team. A
// problem the office has already is not added again.
func (o *Office) Stop(t *Team, problems ...Problem) {
	for _, p := range problems {
		if !slices.Contains(t.Problems, p) {
			t.Problems = append(t.Problems, p)
		}
		if !slices.Contains(o.Problems, p) {
			o.Problems = append(o.Problems, p)
		}
	}
}

// Load gives a file by its path from the office's top, as Corbie's parser
// asks for it. A path with a .bulldog part is what Bulldog ships, from
// after its last .bulldog part; any other is the office's own file, and
// one that leaves the office's directory, by .. or by a link, is refused.
// A definition's model named by one word is filled in.
func (o *Office) Load(name string) ([]byte, error) {
	file, shipped, err := served(name)
	if err != nil {
		return nil, err
	}
	if shipped {
		src, err := fs.ReadFile(o.shipped, file)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("Bulldog ships nothing at ./.bulldog/%s", file)
		}
		return src, err
	}
	src, err := readIn(o.Dir, file)
	if err != nil {
		return nil, err
	}
	return o.withModel(file, src)
}

// served gives where a path from the office's top is served from: a path
// in what Bulldog ships, from after the path's last .bulldog part, or a
// path in the office's directory.
func served(name string) (file string, shipped bool, err error) {
	name = path.Clean(name)
	if name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return "", false, fmt.Errorf("%s leaves the office's directory", name)
	}
	parts := strings.Split(name, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == ".bulldog" {
			return path.Join(append([]string{"."}, parts[i+1:]...)...), true, nil
		}
	}
	return name, false, nil
}

// readIn reads a file by its path in a directory, and refuses one that
// leads out of the directory, by .. or by a link.
func readIn(dir, name string) ([]byte, error) {
	f, err := os.OpenInRoot(dir, filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// withModel fills in a definition's model named by one word, from the
// team nearest the file's directory, and leaves the rest of the file as it
// is written.
func (o *Office) withModel(file string, src []byte) ([]byte, error) {
	if !strings.HasSuffix(file, ".yaml") {
		return src, nil
	}
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil || len(doc.Content) == 0 {
		return src, nil // Corbie's parser says what is wrong with it
	}
	word := modelWord(doc.Content[0])
	if word == nil {
		return src, nil
	}
	t := o.nearest(path.Dir(file))
	model, ok := o.Model(t, word.Value)
	if !ok {
		return nil, noModel(t, word.Value)
	}
	return splice(src, word, model)
}

// modelWord gives the value of a definition's model when it is written as
// one word, the name of a model a team gives; nil otherwise.
func modelWord(n *yaml.Node) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Value == "model" && v.Kind == yaml.ScalarNode && v.Tag == "!!str" {
			return v
		}
	}
	return nil
}

func noModel(t *Team, word string) error {
	return fmt.Errorf("model: %s is not a model of %s or of a team around it; a team names its models under models", word, t.ID)
}

// nearest gives the team whose directory is nearest a directory, outward.
func (o *Office) nearest(dir string) *Team {
	for {
		if dir == "." {
			dir = ""
		}
		for _, t := range o.Teams {
			if t.Path == dir {
				return t
			}
		}
		dir = path.Dir(dir)
	}
}

// splice writes a model in place of the word that names it, on the same
// line, so the file is otherwise as it is written.
func splice(src []byte, word *yaml.Node, model map[string]any) ([]byte, error) {
	var n yaml.Node
	if err := n.Encode(model); err != nil {
		return nil, err
	}
	n.Style = yaml.FlowStyle
	written, err := yaml.Marshal(&n)
	if err != nil {
		return nil, err
	}
	token := word.Value
	switch word.Style {
	case yaml.DoubleQuotedStyle:
		token = `"` + token + `"`
	case yaml.SingleQuotedStyle:
		token = "'" + token + "'"
	}
	lines := strings.SplitAfter(string(src), "\n")
	line := []rune(lines[word.Line-1])
	start := len(string(line[:min(word.Column-1, len(line))]))
	at := lines[word.Line-1]
	if !strings.HasPrefix(at[start:], token) {
		return nil, fmt.Errorf("model: write the name of a model as one word, such as model: usual")
	}
	lines[word.Line-1] = at[:start] + strings.TrimSpace(string(written)) + at[start+len(token):]
	return []byte(strings.Join(lines, "")), nil
}
