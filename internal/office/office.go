// Package office reads an office's folder: the teams in it, the people,
// the workers and the rules, the routines, the models, and the bridges,
// with every definition checked by Corbie as it would be before a start,
// and every problem found reported together. Nothing about the office is
// kept anywhere but its files: a reading is what they said when they were
// read, and a reading is made again whenever they change.
package office

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie/machine"
)

// Top is the name the top team is known by.
const Top = "office"

// The top team's file, and a team within's.
const (
	OfficeFile = "office.yaml"
	TeamFile   = "team.yaml"
)

// DefaultRuns is how many runs may be at work at once in an office that
// does not say.
const DefaultRuns = 3

// An Office is a folder as it was read well. Nothing in it changes once it
// is read: a later reading is another Office.
type Office struct {
	// Dir is the folder.
	Dir string
	// Zone is the office's time zone, in which its times are written and
	// shown.
	Zone *time.Location
	// Runs is how many runs may be at work at once in the whole office.
	Runs int

	// Teams are by the name each is known by, its folder's, the top team
	// as Top; Order has each before the teams within it.
	Teams map[string]*Team
	Order []string
	// People are by the name each is written under, in Everyone's order.
	People   map[string]*Person
	Everyone []string
	// Workers are by name. Each is a worker of one team.
	Workers map[string]*Worker
	// Rules are the rules the teams' lists name, by their paths in the
	// office.
	Rules map[string]*Rule
	// Bridges are by name, in the order the top team's file writes them.
	Bridges     map[string]*Bridge
	BridgeOrder []string

	// Files are the office's files, by their paths in it, in order.
	Files []string
	// Problems are those of the reading that stop one worker, rule or
	// bridge, or are shown on a team, and refuse nothing.
	Problems Problems
	// Checked are the files checked as Corbie checks them before a start,
	// each once, in the order they were checked: each definition by
	// Corbie, and each bare script, of which Corbie checks only that it
	// can be read, by its being a file of its team's folder.
	Checked []string
}

// A Team is a folder with a team file.
type Team struct {
	// Name is what it is known by: its folder's name, or Top.
	Name string
	// Called is what it is called: its name: in any words, or else its
	// folder's name.
	Called string
	// About is the one line saying what it looks after, if it gives one.
	About string
	// Dir is its folder in the office, "" for the top team; File is its
	// team file.
	Dir  string
	File string
	// Within is the team around it, "" for the top team.
	Within string
	// People are its people, as its people: names them.
	People []string
	// Lead is the worker or person that answers for it.
	Lead string
	// Decide are its lists of deciders, in the order written.
	Decide []List
	// Disclosure is its disclosure check, and HasDisclosure whether it sets one.
	Disclosure    []Decider
	HasDisclosure bool
	// Heed is the rule it names to weigh what is said at a channel place
	// before its lead starts on it, or nil.
	Heed     *Heed
	Routines []Routine
	// Models are the models it gives by name, in the order written.
	Models []Model
	// Workers are the names of its workers, in order.
	Workers []string
}

// A List is one list of deciders, under its key: everything, the name of a
// tool, or change.
type List struct {
	Key      string
	Deciders []Decider
}

// The kinds of decider. IsBulldog is Bulldog itself, which no list names,
// and which refuses what nobody can decide.
const (
	IsPerson  = "person"
	IsWorker  = "worker"
	IsRule    = "rule"
	IsBulldog = "bulldog"
)

// A Decider is one of those a list names: a person, a worker of any team,
// or a rule in the team's own folder or one Bulldog ships.
type Decider struct {
	// Written is the entry as the list writes it.
	Written string
	Kind    string
	// Name is the person's or the worker's name, or the rule's path in the
	// office.
	Name string
}

// A Heed is the one rule a team names, by its path, to weigh what is said
// at a channel place before its lead starts on it, with the settings handed
// to the rule. It is no decider: nothing waits on it.
type Heed struct {
	// Written is the rule's path as the team file writes it, and Rule its
	// path in the office, or under .bulldog/ for one Bulldog ships.
	Written string
	Rule    string
	// Settings are every key written beside rule, as written, handed to
	// the rule whole; a heed written as a bare path has none. Bulldog
	// reads none of them. Row is them written in a row, as a team file
	// writes them, { timeout: 5s }, or "" for none.
	Settings map[string]any
	Row      string
	// Line is where in the team file it is written.
	Line int
	// Problems are its own and its rule's. A heed with any is not asked.
	Problems Problems
}

// Runs says whether the heed may be asked.
func (h *Heed) Runs() bool { return len(h.Problems) == 0 }

// A Routine is something said to one of a team's workers at a set time.
type Routine struct {
	// At is the time as written: a time of day, with a weekday or not.
	At  string
	To  string
	Say string
	// Due is the time with the office's zone, as the routine's clock waits
	// for it.
	Due string
	// Line is where in the team file it is written.
	Line int
}

// A Model is a model a team gives by name, as Corbie writes one.
type Model struct {
	Name string
	Team string
	// Text is the model written in a row, as a team file writes it:
	// { name: sonnet, service: claude-code }.
	Text string
}

// A Person is someone real, written once under everyone.
type Person struct {
	// Name is what they are written under.
	Name string
	// Called is their name:, or else Name.
	Called string
	Title  string
	// Reached are who they are on each bridge that knows them, in the
	// order written.
	Reached []Identity
}

// An Identity is who someone is on one bridge, by the lasting id the
// service gives them, exactly as written.
type Identity struct {
	Bridge string
	ID     string
}

// A Worker is a Corbie machine directly in a team's folder.
type Worker struct {
	Name string
	Team string
	// File is its definition's file, by its path in the office.
	File string
	// Type is agent, flow or statechart.
	Type string
	// Written is the file as written, and Definition what is sent to
	// Corbie: the same, with a model named by one word filled in.
	Written    []byte
	Definition []byte
	// ModelName is the word its model is named by, if it is; Model is its
	// model as filled in, written in a row, for an agent.
	ModelName string
	Model     string
	// Draft is the definition filled out and checked by Corbie, nil while
	// it has a problem: such a worker is given no run.
	Draft *machine.Draft
	// Tools are what its definition runs by name, as Corbie fills it out:
	// an agent's tools, a flow's stages, a statechart's jobs; Held are the
	// same, each with what it does, what its job is and whether it waits
	// for a yes.
	Tools []string
	Held  []Tool
	// AtTop says Bulldog starts it at the top: it is its team's lead or
	// a routine's worker.
	AtTop bool
	// Uses are the files of the office its definition reads, its own among
	// them.
	Uses     []string
	Problems Problems
}

// Runs says whether the worker may be given a run.
func (w *Worker) Runs() bool { return w.Draft != nil }

// Part gives what one of the jobs its definition runs by name is called:
// an agent's tool, a flow's stage, a statechart's job.
func (w *Worker) Part() string {
	switch w.Type {
	case "flow":
		return "stage"
	case "statechart":
		return "job"
	}
	return "tool"
}

// A Tool is one job a worker's definition runs by name: an agent's tool,
// a flow's stage or a statechart's job. It has its name, an agent's does
// line as written, and what its job is, told by where the job resolves.
// Its Worker is the worker of the same team its job is, and its File the
// file of the office its job runs, by its path in the office; a machine
// written in place has neither. Waits says Corbie holds the job for a yes.
type Tool struct {
	Name   string
	Does   string
	Kind   string
	Worker string
	File   string
	Waits  bool
	// Args is the shape of its arguments, in full form, as Corbie fills
	// it out.
	Args map[string]any
	// Listed is a list: entry, which gives the tools its server lists,
	// each named after the entry, as github_create_issue.
	Listed bool
}

// What a tool's job is: a worker of the same team, another machine, a
// script, or one of the tools Bulldog ships, whose job is its call.
const (
	ToolWorker  = "worker"
	ToolMachine = "machine"
	ToolScript  = "script"
	ToolBulldog = "bulldog"
)

// A Rule is a decider in its team's folder, or one Bulldog ships: a bare
// script, given the decision and no permission, or a definition holding
// its own. A rule decides as the team whose decision it is, whichever
// team's folder holds it.
type Rule struct {
	// Team is the team whose folder holds it, or "" for a rule Bulldog
	// ships, which is of no team.
	Team string
	// Path is its file in the office, or under .bulldog/ for one Bulldog
	// ships.
	Path string
	// Script says it is a bare script.
	Script bool
	// Written is its file as written. A bare script is started as this,
	// with no definition around it.
	Written []byte
	// Draft is a rule that is a definition, filled out and checked by
	// Corbie with a model named by one word filled in. It is nil for a
	// bare script, and while the rule has a problem.
	Draft    *machine.Draft
	Problems Problems
	// Old are the lines of its files of the office written for an older
	// Bulldog, answering pass or reading passes: problems shown on each
	// team whose lists name it, which do not stop it.
	Old Problems
}

// Runs says whether the rule may be started: a bare script with no
// problem, and a definition once Corbie has checked it.
func (r *Rule) Runs() bool { return len(r.Problems) == 0 && (r.Script || r.Draft != nil) }

// A Bridge is named under bridges in the top team's file, and runs a
// folder: its own under bridges/, or one Bulldog ships, which its entry's
// from names.
type Bridge struct {
	Name string
	// From is the folder Bulldog ships that it runs, as
	// .bulldog/bridges/slack, or empty when it runs its own.
	From string
	// Settings are as written, handed whole to both machines.
	Settings any
	// Places are those it listens in, in the order written.
	Places []Place
	// Listens says its in machine takes a cursor, and Hears that it takes
	// a delivery.
	Listens bool
	Hears   bool
	// In and Out are its machines as Corbie checked them, nil while the
	// bridge has a problem: such a bridge carries nothing.
	In, Out  *machine.Draft
	Problems Problems
}

// Runs says whether the bridge may carry anything.
func (b *Bridge) Runs() bool { return b.In != nil && b.Out != nil && len(b.Problems) == 0 }

// Dir gives the folder the bridge runs, by its path in the office.
func (b *Bridge) Dir() string {
	if b.From != "" {
		return b.From
	}
	return "bridges/" + b.Name
}

// A Place is one a bridge listens in, by its name in the bridge's words.
type Place struct {
	Name string
	// Team is the team asked there.
	Team string
	// Guests says guests may ask there.
	Guests bool
}

// Direct is the place of what is said to a bridge directly.
const Direct = "direct"

// A Problem is one thing a reading finds wrong.
type Problem struct {
	// File is the file it is in, by its path in the office, and Line where
	// in it, when that is known. Place is where in a definition, as Corbie
	// says it.
	File  string
	Line  int
	Place string
	Text  string
	// Team is the team it is shown on, and Worker, Bridge or Rule the one
	// it stops; a rule is named by its path in the office.
	Team   string
	Worker string
	Bridge string
	Rule   string
	// Refuses says it is a problem of a team file, which refuses the
	// reading.
	Refuses bool
}

func (p Problem) String() string {
	text := p.Text
	if p.Place != "" {
		text = p.Place + ": " + text
	}
	switch {
	case p.File == "":
		return text
	case p.Line > 0:
		return fmt.Sprintf("%s:%d: %s", p.File, p.Line, text)
	}
	return p.File + ": " + text
}

// Problems are every problem a reading found, reported together.
type Problems []Problem

func (ps Problems) Error() string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n")
}

// Refuse says whether any of the problems refuses the reading.
func (ps Problems) Refuse() bool {
	return slices.ContainsFunc(ps, func(p Problem) bool { return p.Refuses })
}

// OfWorker gives the problems that stop one worker.
func (ps Problems) OfWorker(name string) Problems {
	return ps.of(func(p Problem) bool { return p.Worker == name })
}

// OfBridge gives the problems that stop one bridge.
func (ps Problems) OfBridge(name string) Problems {
	return ps.of(func(p Problem) bool { return p.Bridge == name })
}

// OfRule gives the problems that stop one rule, by its path in the office.
func (ps Problems) OfRule(path string) Problems {
	return ps.of(func(p Problem) bool { return p.Rule == path })
}

// OfTeam gives the problems shown on a team that stop no one worker or
// bridge: those of its team file and of its rules.
func (ps Problems) OfTeam(name string) Problems {
	return ps.of(func(p Problem) bool { return p.Team == name && p.Worker == "" && p.Bridge == "" })
}

func (ps Problems) of(match func(Problem) bool) Problems {
	var of Problems
	for _, p := range ps {
		if match(p) {
			of = append(of, p)
		}
	}
	return of
}

// TopTeam gives the top team.
func (o *Office) TopTeam() *Team { return o.Teams[Top] }

// Name is the office's name: what its top team is called.
func (o *Office) Name() string { return o.TopTeam().Called }

// Everything gives a team's everything list: as written, or else its
// people.
func (o *Office) Everything(team string) []Decider {
	t := o.Teams[team]
	if t == nil {
		return nil
	}
	if deciders, ok := o.List(team, "everything"); ok {
		return deciders
	}
	var people []Decider
	for _, name := range t.People {
		people = append(people, Decider{Written: name, Kind: IsPerson, Name: name})
	}
	return people
}

// Reached gives the people a team's decisions reach when every rule and
// worker on the way refers them: the people its everything names, or,
// where it names none, those of the first everything outward that does. A
// person not in the office is not reached.
func (o *Office) Reached(team string) []string {
	for t := o.Teams[team]; t != nil; t = o.Teams[t.Within] {
		var reached []string
		for _, d := range o.Everything(t.Name) {
			if d.Kind == IsPerson && o.People[d.Name] != nil && !slices.Contains(reached, d.Name) {
				reached = append(reached, d.Name)
			}
		}
		if len(reached) > 0 {
			return reached
		}
	}
	return nil
}

// List gives a team's list of a key, or nothing when it writes none.
func (o *Office) List(team, key string) ([]Decider, bool) {
	if t := o.Teams[team]; t != nil {
		for _, list := range t.Decide {
			if list.Key == key {
				return list.Deciders, true
			}
		}
	}
	return nil, false
}

// Model gives the model a team's definitions name by one word: its own
// team's, or else the nearest team's around it that gives the name.
func (o *Office) Model(team, word string) (Model, bool) {
	for t := o.Teams[team]; t != nil; t = o.Teams[t.Within] {
		for _, m := range t.Models {
			if m.Name == word {
				return m, true
			}
		}
	}
	return Model{}, false
}

// Person gives the person an identity on a bridge names, compared exactly
// as written, or nil for a guest.
func (o *Office) Person(bridge, id string) *Person {
	for _, name := range o.Everyone {
		p := o.People[name]
		if slices.Contains(p.Reached, Identity{bridge, id}) {
			return p
		}
	}
	return nil
}

// TeamsOf gives the teams a person is in, in the office's order.
func (o *Office) TeamsOf(person string) []string {
	var in []string
	for _, name := range o.Order {
		if slices.Contains(o.Teams[name].People, person) {
			in = append(in, name)
		}
	}
	return in
}

// Place gives a place a bridge listens in.
func (o *Office) Place(bridge, place string) (Place, bool) {
	if b := o.Bridges[bridge]; b != nil {
		for _, p := range b.Places {
			if p.Name == place {
				return p, true
			}
		}
	}
	return Place{}, false
}

// A Note is one of a team's notes: the file's name, without .md, and its
// text.
type Note struct {
	Name string
	Text string
}

// NotesDir is the folder in a team's folder that holds its notes.
const NotesDir = "notes"

// Notes reads a team's notes from its folder now, each whole, in the order
// their names sort. A team with no notes folder has none.
func (o *Office) Notes(team string) ([]Note, error) {
	t := o.Teams[team]
	if t == nil {
		return nil, fmt.Errorf("the office has no team %s", team)
	}
	dir := filepath.Join(o.Dir, filepath.FromSlash(path.Join(t.Dir, NotesDir)))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []Note
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || !entry.Type().IsRegular() {
			continue
		}
		text, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		notes = append(notes, Note{Name: strings.TrimSuffix(entry.Name(), ".md"), Text: string(text)})
	}
	return notes, nil
}

// A Doc is one of a team's docs: its name, the path of its file in the
// team's docs folder without .md, the team whose it is, and its size in
// bytes.
type Doc struct {
	Name string
	Team string
	Size int64
}

// DocsDir is the folder in a team's folder that holds its docs.
const DocsDir = "docs"

// Docs lists a team's docs from its folder now, in the order their names
// sort: every Markdown file under its docs folder, subfolders included,
// but for anything whose name starts with a dot and anything that is not
// a plain file. A team with no docs folder has none.
func (o *Office) Docs(team string) ([]Doc, error) {
	t := o.Teams[team]
	if t == nil {
		return nil, fmt.Errorf("the office has no team %s", team)
	}
	dir := filepath.Join(o.Dir, filepath.FromSlash(path.Join(t.Dir, DocsDir)))
	var docs []Doc
	err := filepath.WalkDir(dir, func(file string, entry os.DirEntry, err error) error {
		switch {
		case file == dir && os.IsNotExist(err):
			return filepath.SkipAll
		case err != nil:
			return err
		case file == dir:
			return nil
		case strings.HasPrefix(entry.Name(), "."):
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		case !entry.Type().IsRegular() || path.Ext(entry.Name()) != ".md":
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		docs = append(docs, Doc{Name: strings.TrimSuffix(filepath.ToSlash(name), ".md"), Team: team, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(docs, func(a, b Doc) int { return strings.Compare(a.Name, b.Name) })
	return docs, nil
}

// DocsFor gives the docs a worker of a team reads: its own team's, then
// those of each team around it, out to the office. A doc of a nearer team
// stands before one of the same name further out, which is left out.
func (o *Office) DocsFor(team string) ([]Doc, error) {
	var docs []Doc
	for t := o.Teams[team]; t != nil; t = o.Teams[t.Within] {
		own, err := o.Docs(t.Name)
		if err != nil {
			return nil, err
		}
		for _, d := range own {
			if !slices.ContainsFunc(docs, func(near Doc) bool { return near.Name == d.Name }) {
				docs = append(docs, d)
			}
		}
	}
	return docs, nil
}

// DocText reads a doc from its team's folder now. A doc's name is a path
// in its team's docs folder that never leaves it, and any other is
// refused.
func (o *Office) DocText(d Doc) (string, error) {
	t := o.Teams[d.Team]
	if t == nil {
		return "", fmt.Errorf("the office has no team %s", d.Team)
	}
	for _, part := range strings.Split(d.Name, "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("%q is not the name of a doc", d.Name)
		}
	}
	text, err := os.ReadFile(filepath.Join(o.Dir, filepath.FromSlash(path.Join(t.Dir, DocsDir, d.Name+".md"))))
	return string(text), err
}
