package office

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // every zone is known, wherever Bulldog runs

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/shipped"
)

// Read reads the office in a folder. A reading that a problem in a team
// file refuses gives no office, and its error is the Problems found, every
// one of them, those that would stop only one worker or bridge among them.
// A folder that is not there, or holds no office.yaml, is refused as no
// office, and nothing is written into it.
func Read(dir string) (*Office, error) {
	files, left, err := Folder(dir)
	if err != nil {
		return nil, err
	}
	o, _ := Parse(dir, files)
	o.Problems = append(left, o.Problems...)
	if o.Problems.Refuse() {
		return nil, o.Problems
	}
	return o, nil
}

// Parse reads an office from its files, each by its path in the office,
// as if they were the folder dir: the office as a change would make it is
// read from the folder's files with the change laid over them. Nothing
// whose name starts with a dot is among them. It gives every problem
// found; an office whose problems refuse it is not to be used.
func Parse(dir string, given map[string][]byte) (*Office, Problems) {
	files := map[string][]byte{}
	for name, contents := range given {
		if !hasDot(name) {
			files[name] = contents
		}
	}
	r := &reading{files: files, o: &Office{
		Dir: dir, Zone: time.UTC, Runs: DefaultRuns,
		Teams: map[string]*Team{}, People: map[string]*Person{}, Workers: map[string]*Worker{},
		Rules: map[string]*Rule{}, Bridges: map[string]*Bridge{},
		Files: slices.Sorted(maps.Keys(files)),
	}, names: map[string]string{}, filledOut: map[string]bool{}, unfilled: map[string]string{}}
	if _, ok := files[OfficeFile]; !ok {
		r.refuse(Problem{File: OfficeFile, Text: "there is no office.yaml, the top team's file, so this is not an office"})
		return r.o, r.problems
	}
	r.findTeams()
	r.readTeamFiles()
	r.findWorkers()
	r.readLists()
	r.readLeads()
	r.readHeeds()
	r.readRoutines()
	r.readBridges()
	r.checkWorkers()
	r.checkRules()
	r.checkBridges()
	r.checkKeys()
	r.o.Problems = r.problems
	return r.o, r.problems
}

// A reading is an office being read, with what it has found wrong.
type reading struct {
	files    map[string][]byte
	o        *Office
	problems Problems
	// nodes are the team files as read, by team.
	nodes map[string]*yaml.Node
	// names gives, for each name used of a person, worker or team, where
	// it was first used.
	names map[string]string
	// filledOut are the workers whose definitions Corbie filled out, so
	// that the tools they hold are known.
	filledOut map[string]bool
	// unfilled gives, for each file of the office whose models could not
	// be filled in when another definition named it, what was said.
	unfilled map[string]string
}

func (r *reading) refuse(p Problem) {
	p.Refuses = true
	r.problems = append(r.problems, p)
}

func (r *reading) report(p Problem) { r.problems = append(r.problems, p) }

// teamFile gives the problem of a team's file, at a node of it.
func (r *reading) teamFile(t *Team, at *yaml.Node, format string, args ...any) {
	p := Problem{File: t.File, Team: t.Name, Text: fmt.Sprintf(format, args...)}
	if at != nil {
		p.Line = at.Line
	}
	r.refuse(p)
}

// nameForm is how the name of a person, a worker, a team, a bridge or a
// model is written.
var nameForm = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Reserved are the names no person, worker, team or bridge may have: they
// are files and folders of the office, the built-in chat, or who Bulldog
// itself speaks as.
var Reserved = []string{Top, "team", NotesDir, DocsDir, "bridges", "chat", "routine", "bulldog", "guest"}

// nameProblem says what is wrong with a name, or nothing.
func nameProblem(name string) string {
	switch {
	case !nameForm.MatchString(name):
		return fmt.Sprintf("%q is not a name: a name is small letters, figures and dashes, beginning with a letter", name)
	case slices.Contains(Reserved, name):
		return fmt.Sprintf("%q is a name Bulldog keeps for itself (%s)", name, strings.Join(Reserved, ", "))
	}
	return ""
}

// dirs gives the folders the office's files are in, each by its path,
// with the folders that hold them.
func (r *reading) dirs() map[string]bool {
	dirs := map[string]bool{"": true}
	for file := range r.files {
		for d := path.Dir(file); d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	return dirs
}

// findTeams finds the teams: the top team, and each folder directly inside
// a team's folder that holds a team.yaml. A team file anywhere else is a
// problem.
func (r *reading) findTeams() {
	top := &Team{Name: Top, File: OfficeFile}
	r.o.Teams[Top], r.o.Order = top, []string{Top}
	dirs := r.dirs()
	// held are the folders whose team files are where a team can be.
	held := map[string]bool{"": true}
	var walk func(t *Team)
	walk = func(t *Team) {
		for _, d := range slices.Sorted(maps.Keys(dirs)) {
			if d == "" || path.Dir(d) != dirOr(t.Dir) {
				continue
			}
			if _, ok := r.files[path.Join(d, TeamFile)]; !ok {
				continue
			}
			name := path.Base(d)
			within := &Team{Name: name, Dir: d, File: path.Join(d, TeamFile), Within: t.Name}
			held[d] = true
			if problem := nameProblem(name); problem != "" {
				r.refuse(Problem{File: within.File, Team: name, Text: "the team's folder: " + problem})
			}
			r.use(name, "the team "+d, within.File)
			if _, twice := r.o.Teams[name]; twice {
				continue
			}
			r.o.Teams[name] = within
			r.o.Order = append(r.o.Order, name)
			walk(within)
		}
	}
	walk(top)
	for _, file := range r.o.Files {
		switch base, d := path.Base(file), dirOf(file); {
		case base == TeamFile && d == "":
			r.refuse(Problem{File: file, Text: "a team.yaml at the top is no team: the top team's file is office.yaml"})
		case base == TeamFile && !held[d]:
			r.refuse(Problem{File: file, Text: fmt.Sprintf("a team.yaml is read only directly in a team's folder, and %s is not a team's folder", path.Dir(d))})
		case base == OfficeFile && d != "":
			r.refuse(Problem{File: file, Text: "office.yaml is the top team's file, and is read only at the top of the office"})
		}
	}
}

// dirOf gives the folder a file is in, "" at the top.
func dirOf(file string) string {
	if d := path.Dir(file); d != "." {
		return d
	}
	return ""
}

// dirOr gives a folder as path.Dir gives it: "." for the top.
func dirOr(d string) string {
	if d == "" {
		return "."
	}
	return d
}

// use notes that a name is used of a person, a worker or a team, in a
// file, saying so when it was used already.
func (r *reading) use(name, what, file string) {
	if before, ok := r.names[name]; ok {
		r.refuse(Problem{File: file, Text: fmt.Sprintf("the name %q is used twice, for %s and for %s: a name is used once in the whole office", name, before, what)})
		return
	}
	r.names[name] = what
}

// teamAt gives the team whose folder holds a folder most nearly.
func (r *reading) teamAt(dir string) *Team {
	at := r.o.Teams[Top]
	for _, t := range r.o.Teams {
		if t.Dir != "" && (dir == t.Dir || strings.HasPrefix(dir, t.Dir+"/")) && len(t.Dir) > len(at.Dir) {
			at = t
		}
	}
	return at
}

// The keys a team file takes, and those only the top team's takes.
var (
	teamKeys = []string{"name", "about", "people", "lead", "heed", "decide", "disclosure", "routines", "models"}
	topKeys  = []string{"zone", "everyone", "bridges", "runs"}
)

// readTeamFiles reads each team's file: what it is called, what it looks
// after, its people, its models, and at the top the office's own.
func (r *reading) readTeamFiles() {
	r.nodes = map[string]*yaml.Node{}
	for _, name := range r.o.Order {
		t := r.o.Teams[name]
		t.Called = path.Base(t.Dir)
		if t.Name == Top {
			t.Called = path.Base(r.o.Dir)
		}
		root, err := document(r.files[t.File])
		if err != nil {
			r.teamFile(t, nil, "it is not YAML that can be read: %v", err)
			continue
		}
		if root == nil {
			root = &yaml.Node{Kind: yaml.MappingNode}
		}
		if root.Kind != yaml.MappingNode {
			r.teamFile(t, root, "a team file is a mapping of keys, such as name: and lead:, and this is %s", describe(root))
			continue
		}
		r.nodes[name] = root
		for _, k := range twice(root) {
			r.teamFile(t, k, "%s is written twice in one mapping, and only one of them can be meant", k.Value)
		}
		for k := range pairs(root) {
			switch {
			case slices.Contains(teamKeys, k.Value):
			case slices.Contains(topKeys, k.Value) && t.Name == Top:
			case slices.Contains(topKeys, k.Value):
				r.teamFile(t, k, "%s is written only in the top team's file, office.yaml", k.Value)
			default:
				r.teamFile(t, k, "%s is not a key a team file takes (it takes %s, and the top team's also %s)",
					k.Value, strings.Join(teamKeys, ", "), strings.Join(topKeys, ", "))
			}
		}
		if node := child(root, "name"); node != nil {
			if s, ok := text(node); !ok {
				r.teamFile(t, node, "name is what the team is called, in words, and this is %s", describe(node))
			} else if s != "" {
				t.Called = s
			}
		}
		if node := child(root, "about"); node != nil {
			s, ok := text(node)
			switch {
			case !ok:
				r.teamFile(t, node, "about is one line saying what the team looks after, and this is %s", describe(node))
			case strings.Contains(strings.TrimRight(s, "\n"), "\n"):
				r.teamFile(t, node, "about is one line saying what %s looks after, and this is more than one", t.Name)
			default:
				t.About = strings.TrimSpace(s)
			}
		}
		r.readModels(t, child(root, "models"))
	}
	if top := r.nodes[Top]; top != nil {
		r.readOffice(r.o.Teams[Top], top)
	}
	for _, name := range r.o.Order {
		if root := r.nodes[name]; root != nil {
			r.readPeople(r.o.Teams[name], child(root, "people"))
		}
	}
}

// readModels reads the models a team gives by name.
func (r *reading) readModels(t *Team, node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.MappingNode {
		r.teamFile(t, node, "models gives models by name, as { usual: { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY } }, and this is %s", describe(node))
		return
	}
	for k, v := range pairs(node) {
		if !nameForm.MatchString(k.Value) {
			r.teamFile(t, k, "models: %q is not a name for a model: a name is small letters, figures and dashes, beginning with a letter", k.Value)
			continue
		}
		if v.Kind != yaml.MappingNode {
			r.teamFile(t, v, "models: %s is a model written as Corbie writes one, a mapping such as { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }, and this is %s", k.Value, describe(v))
			continue
		}
		m := Model{Name: k.Value, Team: t.Name, Text: inRow(v)}
		if secret := child(v, "secret"); secret != nil {
			m.Secret = secret.Value
		}
		t.Models = append(t.Models, m)
	}
}

// readOffice reads what only the top team's file gives: the zone, how
// many runs may be at work, and everyone.
func (r *reading) readOffice(t *Team, root *yaml.Node) {
	if node := child(root, "zone"); node != nil {
		s, ok := text(node)
		zone, err := time.LoadLocation(s)
		if !ok || s == "" || s == "Local" || err != nil {
			r.teamFile(t, node, "zone is a time zone by its name, such as Europe/London, and %s is not one", describe(node))
		} else {
			r.o.Zone = zone
		}
	}
	if node := child(root, "runs"); node != nil {
		var runs int
		if node.Kind != yaml.ScalarNode || node.Tag != "!!int" || node.Decode(&runs) != nil || runs < 1 {
			r.teamFile(t, node, "runs is how many runs may be at work at once, a whole number from 1, and this is %s", describe(node))
		} else {
			r.o.Runs = runs
		}
	}
	// The bridges' names are read first: a person's keys name them.
	bridges := child(root, "bridges")
	if bridges != nil && bridges.Kind != yaml.MappingNode {
		r.teamFile(t, bridges, "bridges names each bridge, with its settings, as { slack: { places: { \"#help\": {} } } }, and this is %s", describe(bridges))
		bridges = nil
	}
	for k := range pairs(bridges) {
		name := k.Value
		switch problem := nameProblem(name); {
		case problem != "":
			r.teamFile(t, k, "bridges: %s", problem)
		case name == "name" || name == "title":
			r.teamFile(t, k, "bridges: %q is a key of a person, and is no name for a bridge", name)
		case r.o.Bridges[name] != nil:
			// said with every key written twice
		default:
			r.o.Bridges[name] = &Bridge{Name: name}
			r.o.BridgeOrder = append(r.o.BridgeOrder, name)
		}
	}
	everyone := child(root, "everyone")
	if everyone != nil && everyone.Kind != yaml.MappingNode {
		r.teamFile(t, everyone, "everyone writes each person by a name of their own, as { sam: { name: Sam } }, and this is %s", describe(everyone))
		return
	}
	seen := map[Identity]string{}
	folders := r.bridgeFolders()
	for k, v := range pairs(everyone) {
		name := k.Value
		if problem := nameProblem(name); problem != "" {
			r.teamFile(t, k, "everyone: %s", problem)
			continue
		}
		if r.o.People[name] != nil {
			continue // said with every key written twice
		}
		p := &Person{Name: name, Called: name}
		if v.Kind != yaml.MappingNode && v.Tag != "!!null" {
			r.teamFile(t, v, "everyone: %s is a mapping, with name:, title: and who they are on each bridge, and this is %s", name, describe(v))
			continue
		}
		for key, value := range pairs(v) {
			s, ok := text(value)
			if value.Kind == yaml.ScalarNode && value.Tag != "!!null" {
				s, ok = value.Value, true
			}
			switch {
			case !ok:
				r.teamFile(t, value, "everyone: %s's %s is written as a word, and this is %s", name, key.Value, describe(value))
			case key.Value == "name":
				if s != "" {
					p.Called = s
				}
			case key.Value == "title":
				p.Title = s
			case r.o.Bridges[key.Value] == nil && !folders[key.Value]:
				r.teamFile(t, key, "everyone: %s gives %s, which is no bridge of the office: beside name and title, each key is a bridge's name, giving who %s is there", name, key.Value, name)
			default:
				id := Identity{key.Value, s}
				if other, shared := seen[id]; shared {
					r.teamFile(t, value, "everyone: %s and %s are both %s on %s: two people never share who they are on one bridge", other, name, s, key.Value)
					continue
				}
				seen[id] = name
				p.Reached = append(p.Reached, id)
			}
		}
		r.o.People[name] = p
		r.o.Everyone = append(r.o.Everyone, name)
		r.use(name, "the person "+name, OfficeFile)
	}
}

// readPeople reads a team's people.
func (r *reading) readPeople(t *Team, node *yaml.Node) {
	if node == nil {
		return
	}
	people, ok := words(node)
	if !ok {
		r.teamFile(t, node, "people is a list of the team's people, as [sam, kit], and this is %s", describe(node))
		return
	}
	for i, name := range people {
		switch {
		case r.o.People[name] == nil:
			r.teamFile(t, node.Content[i], "people: %s is nobody: each of a team's people is written under everyone in office.yaml", name)
		case slices.Contains(t.People, name):
			r.teamFile(t, node.Content[i], "people: %s is written twice", name)
		default:
			t.People = append(t.People, name)
		}
	}
}

// findWorkers finds each team's workers: the files directly in its folder
// whose definitions are machines, but for the rules its lists name. A file
// there that is not YAML that can be read is a worker with a problem when
// its first line names a type of machine, and otherwise a problem shown on
// the team.
func (r *reading) findWorkers() {
	rules := map[string]bool{}
	for _, name := range r.o.Order {
		t := r.o.Teams[name]
		for _, entry := range r.ruleEntries(name) {
			if full := path.Join(t.Dir, entry); r.rulePath(t, full) == "" {
				rules[full] = true
			}
		}
	}
	for _, file := range r.o.Files {
		ext := path.Ext(file)
		if (ext != ".yaml" && ext != ".yml") || rules[file] {
			continue
		}
		base := path.Base(file)
		if base == TeamFile || base == OfficeFile {
			continue
		}
		t := r.teamOfFolder(dirOf(file))
		if t == nil {
			continue
		}
		name := strings.TrimSuffix(base, ext)
		root, err := document(r.files[file])
		w := &Worker{Name: name, Team: t.Name, File: file, Written: r.files[file]}
		switch {
		case err != nil && !isType(firstKey(r.files[file])):
			r.report(Problem{File: file, Team: t.Name, Text: fmt.Sprintf("it is not YAML that can be read: %v", err)})
			continue
		case err != nil:
			w.Problems = append(w.Problems, Problem{File: file, Team: t.Name, Worker: name, Text: fmt.Sprintf("it is not YAML that can be read: %v", err)})
		case root == nil || root.Kind != yaml.MappingNode || len(root.Content) == 0 || !isType(root.Content[0].Value):
			continue // a tools file, or something a worker reads
		default:
			w.Type = root.Content[0].Value
		}
		if problem := nameProblem(name); problem != "" {
			r.refuse(Problem{File: file, Team: t.Name, Text: "the worker's file: " + problem})
		}
		r.use(name, "the worker "+file, file)
		if r.o.Workers[name] != nil {
			continue
		}
		r.o.Workers[name] = w
		t.Workers = append(t.Workers, name)
	}
}

// teamOfFolder gives the team whose folder a folder is, or nil.
func (r *reading) teamOfFolder(dir string) *Team {
	for _, t := range r.o.Teams {
		if t.Dir == dir {
			return t
		}
	}
	return nil
}

// ruleEntries gives the entries of a team's lists that are paths, and the
// path its heed names.
func (r *reading) ruleEntries(team string) []string {
	var entries []string
	root := r.nodes[team]
	if heed, ok := text(heedRule(child(root, "heed"))); ok && isPath(heed) {
		entries = append(entries, heed)
	}
	lists := []*yaml.Node{child(root, "disclosure")}
	for _, v := range pairs(child(root, "decide")) {
		lists = append(lists, v)
	}
	for _, list := range lists {
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range list.Content {
			if entry, _, problem := listEntry(item); problem == "" && isPath(entry) {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

// listEntry reads one entry of a list of deciders: a name or a path, or a
// rule or a worker marked to advise, as { reviewer: advise }. Its problem
// says what is wrong with an entry that is neither.
func listEntry(item *yaml.Node) (entry string, advises bool, problem string) {
	if s, ok := text(item); ok {
		return s, false, ""
	}
	if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
		return "", false, fmt.Sprintf("an entry is a decider, as sam, or a rule or worker marked to advise, as reviewer: advise, and this is %s", describe(item))
	}
	k, v := item.Content[0], item.Content[1]
	name, ok := text(k)
	if !ok {
		return "", false, fmt.Sprintf("an entry is a decider, as sam, or a rule or worker marked to advise, as reviewer: advise, and this is %s", describe(item))
	}
	if word, _ := text(v); word != "advise" {
		marked := describe(v)
		if v.Kind == yaml.ScalarNode && v.Tag != "!!null" {
			marked = v.Value
		}
		return name, false, fmt.Sprintf("%s is marked %s, and advise is the one word a decider is marked with, as reviewer: advise", name, marked)
	}
	return name, true, ""
}

// isPath says whether a list's entry is a path, which names a rule.
func isPath(entry string) bool {
	return strings.HasPrefix(entry, "./") || strings.HasPrefix(entry, "../")
}

// readLists reads each team's decide and disclosure lists.
func (r *reading) readLists() {
	for _, name := range r.o.Order {
		t, root := r.o.Teams[name], r.nodes[name]
		if root == nil {
			continue
		}
		if node := child(root, "decide"); node != nil {
			if node.Kind != yaml.MappingNode {
				r.teamFile(t, node, "decide gives lists of deciders by what they decide, as { everything: [sam] }, and this is %s", describe(node))
			}
			for k, v := range pairs(node) {
				if k.Value == "change" && t.Name != Top {
					r.teamFile(t, k, "decide: change is written only in the top team's file, office.yaml: a change to the office is the top team's to decide")
					continue
				}
				if deciders, ok := r.readList(t, "decide: "+k.Value, v); ok {
					t.Decide = append(t.Decide, List{Key: k.Value, Deciders: deciders})
				}
			}
		}
		if node := child(root, "disclosure"); node != nil {
			if deciders, ok := r.readList(t, "disclosure", node); ok {
				t.Disclosure, t.HasDisclosure = deciders, true
			}
		}
	}
	top := r.o.Teams[Top]
	if r.nodes[Top] != nil && !slices.ContainsFunc(r.o.Everything(Top), func(d Decider) bool { return d.Kind == IsPerson }) {
		r.teamFile(top, child(r.nodes[Top], "decide"), "the office's everything names no person, and it is where every decision nobody else takes ends: write a person in it, or give the top team people")
	}
}

// readList reads one list of deciders.
func (r *reading) readList(t *Team, what string, node *yaml.Node) ([]Decider, bool) {
	if node == nil || node.Kind != yaml.SequenceNode {
		r.teamFile(t, node, "%s is a list of deciders, as [./a-rule.js, sam], and this is %s", what, describe(node))
		return nil, false
	}
	var deciders []Decider
	person := ""
	good := true
	for _, at := range node.Content {
		entry, advises, problem := listEntry(at)
		if problem != "" {
			r.teamFile(t, at, "%s: %s", what, problem)
			good = false
			continue
		}
		d := Decider{Written: entry, Advises: advises}
		switch {
		case isPath(entry):
			d.Kind, d.Name = IsRule, path.Join(t.Dir, entry)
			if problem := r.rulePath(t, d.Name); problem != "" {
				r.teamFile(t, at, "%s: %s %s", what, entry, problem)
				good = false
				continue
			}
		case r.o.People[entry] != nil:
			d.Kind, d.Name = IsPerson, entry
		case r.o.Workers[entry] != nil:
			d.Kind, d.Name = IsWorker, entry
		default:
			r.teamFile(t, at, "%s: %s is nobody: a decider is a person under everyone, a worker of the office, or a rule, in %s's folder or among those Bulldog ships in %s/rules/, by its path, as ./a-rule.js", what, entry, called(t), shipped.Dir)
			good = false
			continue
		}
		if d.Kind == IsPerson && d.Advises {
			r.teamFile(t, at, "%s: %s is a person, who says yes or refuses and never refers, so does not advise: only a rule or a worker advises", what, entry)
			good = false
			continue
		}
		if slices.ContainsFunc(deciders, func(had Decider) bool { return had.Name == d.Name }) {
			r.teamFile(t, at, "%s: %s is written twice", what, entry)
			good = false
			continue
		}
		if d.Kind != IsPerson && person != "" {
			r.teamFile(t, at, "%s: %s comes after %s, a person, and would never be asked: in a list, every rule and worker comes before every person", what, entry, person)
			good = false
		}
		if d.Kind == IsPerson && person == "" {
			person = entry
		}
		deciders = append(deciders, d)
	}
	return deciders, good
}

// rulePath says what is wrong with the path of a rule, by its path in the
// office, or nothing. A rule is a bare script, a .js file, or a definition,
// in its team's own folder, or one Bulldog ships, in .bulldog/rules/ at the
// top of the office, which any team may name.
func (r *reading) rulePath(t *Team, full string) string {
	if own, ok := strings.CutPrefix(full, shipped.Dir+"/"); ok {
		_, ships := shipped.File(own)
		switch ext := path.Ext(full); {
		case !strings.HasPrefix(own, "rules/") || !ships:
			return fmt.Sprintf("is no rule Bulldog ships: those are each named by its path in %s/rules/ at the top of the office", shipped.Dir)
		case ext != ".js" && ext != ".yaml" && ext != ".yml":
			return "is not a rule: a rule is a bare script, a .js file, or a Corbie definition, a .yaml file"
		}
		return ""
	}
	inside := t.Dir == "" && !strings.HasPrefix(full, "../") && full != ".." || t.Dir != "" && strings.HasPrefix(full, t.Dir+"/")
	switch ext := path.Ext(full); {
	case !inside:
		return fmt.Sprintf("leaves %s's folder: a rule is in its team's own folder, or is one Bulldog ships in %s/rules/", called(t), shipped.Dir)
	case r.teamAt(dirOf(full)) != t:
		return fmt.Sprintf("is in the folder of %s, a team within %s: a rule is in its team's own folder, or is one Bulldog ships in %s/rules/", r.teamAt(dirOf(full)).Name, called(t), shipped.Dir)
	case hasDot(full):
		return "is not a file of the office: nothing whose name starts with a dot is the office's"
	case ext != ".js" && ext != ".yaml" && ext != ".yml":
		return "is not a rule: a rule is a bare script, a .js file, or a Corbie definition, a .yaml file"
	}
	if _, ok := r.files[full]; !ok {
		return "names nothing: there is no such file in the office"
	}
	return ""
}

// hasDot says whether any part of a path starts with a dot.
func hasDot(file string) bool {
	return slices.ContainsFunc(strings.Split(file, "/"), func(part string) bool { return strings.HasPrefix(part, ".") })
}

// readLeads reads each team's lead: the one worker or person who
// answers for it.
func (r *reading) readLeads() {
	for _, name := range r.o.Order {
		t, root := r.o.Teams[name], r.nodes[name]
		if root == nil {
			continue
		}
		node := child(root, "lead")
		lead, ok := text(node)
		switch {
		case node == nil:
			r.teamFile(t, nil, "it names no lead: every team has one worker or one person of its own who answers for it")
		case !ok || lead == "":
			r.teamFile(t, node, "lead is the one worker or person of the team who answers for it, and this is %s", describe(node))
		case slices.Contains(t.Workers, lead) || slices.Contains(t.People, lead):
			t.Lead = lead
			if w := r.o.Workers[lead]; w != nil && w.Team == t.Name {
				w.AtTop = true
			}
		default:
			r.teamFile(t, node, "lead: %s is not a worker or a person of %s", lead, called(t))
		}
	}
}

// heedRule gives the node a heed names its rule by: the heed itself, or its
// rule: when it is written with settings.
func heedRule(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.MappingNode {
		return child(node, "rule")
	}
	return node
}

// readHeeds reads each team's heed: one rule, named by its path as a list
// names one, written alone or as the rule: of a mapping whose other keys
// are settings, handed to the rule whole. A heed is asked only before a
// lead that is a worker: one before a person is a problem of the heed
// alone.
func (r *reading) readHeeds() {
	for _, name := range r.o.Order {
		t, node := r.o.Teams[name], child(r.nodes[name], "heed")
		if node == nil {
			continue
		}
		h := &Heed{Settings: map[string]any{}, Line: node.Line}
		rule := heedRule(node)
		if node.Kind == yaml.MappingNode {
			settings := &yaml.Node{Kind: yaml.MappingNode}
			for k, v := range pairs(node) {
				if k.Value != "rule" {
					settings.Content = append(settings.Content, k, v)
				}
			}
			if err := settings.Decode(&h.Settings); err != nil {
				r.teamFile(t, node, "heed: its settings cannot be read: %v", err)
				continue
			}
			if len(settings.Content) > 0 {
				h.Row = inRow(settings)
			}
		}
		written, ok := text(rule)
		if !ok || !isPath(written) {
			r.teamFile(t, node, "heed is one rule, by its path, as ./for-us.js, or with settings for it, as { rule: ./for-us.yaml, timeout: 5s }, and its rule is %s", describe(rule))
			continue
		}
		h.Written, h.Rule = written, path.Join(t.Dir, written)
		if problem := r.rulePath(t, h.Rule); problem != "" {
			r.teamFile(t, rule, "heed: %s %s", written, problem)
			continue
		}
		if r.o.People[t.Lead] != nil {
			p := Problem{File: t.File, Line: node.Line, Team: name,
				Text: fmt.Sprintf("heed: %s is not asked: %s's lead, %s, is a person, and a heed is asked only before a lead that is a worker", written, called(t), t.Lead)}
			h.Problems = append(h.Problems, p)
			r.report(p)
		}
		t.Heed = h
	}
}

// atForm is a routine's time as written: a time of day, with a weekday
// before it or not, and with a zone after it, which is a problem.
var atForm = regexp.MustCompile(`^(?:(mon|tue|wed|thu|fri|sat|sun) )?((?:[01][0-9]|2[0-3]):[0-5][0-9])( \S+)?$`)

// readRoutines reads each team's routines.
func (r *reading) readRoutines() {
	for _, name := range r.o.Order {
		t, root := r.o.Teams[name], r.nodes[name]
		node := child(root, "routines")
		if node == nil {
			continue
		}
		if node.Kind != yaml.SequenceNode {
			r.teamFile(t, node, "routines is a list, each as { at: \"mon 09:00\", to: a worker, say: what it is told }, and this is %s", describe(node))
			continue
		}
		for i, item := range node.Content {
			what := fmt.Sprintf("routines[%d]", i)
			if item.Kind != yaml.MappingNode {
				r.teamFile(t, item, "%s is a mapping, as { at: \"mon 09:00\", to: a worker, say: what it is told }, and this is %s", what, describe(item))
				continue
			}
			c := Routine{Line: item.Line}
			good := true
			for k, v := range pairs(item) {
				s, ok := text(v)
				switch {
				case !slices.Contains([]string{"at", "to", "say"}, k.Value):
					r.teamFile(t, k, "%s: %s is not a key a routine takes: it takes at, to and say", what, k.Value)
					good = false
				case !ok || s == "":
					r.teamFile(t, v, "%s: %s is written as words, and this is %s", what, k.Value, describe(v))
					good = false
				case k.Value == "at":
					c.At = s
				case k.Value == "to":
					c.To = s
				case k.Value == "say":
					c.Say = s
				}
			}
			for _, key := range []string{"at", "to", "say"} {
				if child(item, key) == nil {
					r.teamFile(t, item, "%s has no %s: a routine says at what time, to which worker and what is said", what, key)
					good = false
				}
			}
			if !good {
				continue
			}
			m := atForm.FindStringSubmatch(c.At)
			switch {
			case m == nil:
				r.teamFile(t, child(item, "at"), "%s: at %q is not a time of day, written as 18:00 or with a weekday before it, as \"mon 09:00\"", what, c.At)
				good = false
			case m[3] != "":
				r.teamFile(t, child(item, "at"), "%s: at %q names a zone, and a time the files write is in the office's zone, %s: write it without one", what, c.At, r.o.Zone)
				good = false
			}
			switch w := r.o.Workers[c.To]; {
			case w != nil && w.Team == t.Name:
				w.AtTop = true
			case r.o.People[c.To] != nil:
				r.teamFile(t, child(item, "to"), "%s: to %s is a person, and a routine is said to a worker of %s", what, c.To, called(t))
				good = false
			default:
				r.teamFile(t, child(item, "to"), "%s: to %s is not a worker of %s", what, c.To, called(t))
				good = false
			}
			if !good {
				continue
			}
			c.Due = c.At + " " + r.o.Zone.String()
			t.Routines = append(t.Routines, c)
		}
	}
}

// readBridges reads each bridge's settings and the places it listens in,
// and finds its folder. Whatever is wrong with a bridge stops only it.
func (r *reading) readBridges() {
	root := r.nodes[Top]
	node := child(root, "bridges")
	stop := func(b *Bridge, line int, file, format string, args ...any) {
		b.Problems = append(b.Problems, Problem{File: file, Line: line, Team: Top, Bridge: b.Name, Text: fmt.Sprintf(format, args...)})
	}
	// froms are the bridges whose entry has a from.
	froms := map[string]bool{}
	for k, v := range pairs(node) {
		b := r.o.Bridges[k.Value]
		if b == nil {
			continue
		}
		b.Line = k.Line
		if v.Kind != yaml.MappingNode {
			stop(b, v.Line, OfficeFile, "bridges: %s's settings are a mapping, as { places: { direct: {} } }, and these are %s", b.Name, describe(v))
		} else if err := v.Decode(&b.Settings); err != nil {
			stop(b, v.Line, OfficeFile, "bridges: %s's settings cannot be read: %v", b.Name, err)
		}
		if from := child(v, "from"); from != nil {
			s, _ := text(from)
			dir := path.Clean(s)
			froms[b.Name] = true
			if name, ok := strings.CutPrefix(dir, shipped.Dir+"/bridges/"); ok && slices.Contains(shipped.Bridges(), name) {
				b.From = dir
			} else {
				var ships []string
				for _, name := range shipped.Bridges() {
					ships = append(ships, "./"+path.Join(shipped.Dir, "bridges", name))
				}
				stop(b, from.Line, OfficeFile, "bridges: %s's from is %s, and from names a bridge Bulldog ships, by its folder: %s", b.Name, describe(from), strings.Join(ships, ", "))
			}
		}
		places := child(v, "places")
		if places != nil && places.Kind != yaml.MappingNode {
			stop(b, places.Line, OfficeFile, "bridges: %s's places are a mapping of the places it listens in, as { \"#help\": { team: support } }, and these are %s", b.Name, describe(places))
		}
		for pk, pv := range pairs(places) {
			p := Place{Name: pk.Value, Team: Top}
			if pv.Kind != yaml.MappingNode {
				stop(b, pv.Line, OfficeFile, "bridges: %s's place %s is a mapping, as { team: support, guests: true }, and this is %s", b.Name, p.Name, describe(pv))
				continue
			}
			good := true
			if team := child(pv, "team"); team != nil {
				s, ok := text(team)
				if !ok || r.o.Teams[s] == nil {
					stop(b, team.Line, OfficeFile, "bridges: %s's place %s names the team %s, which is no team of the office", b.Name, p.Name, describe(team))
					good = false
				}
				p.Team = s
			}
			if guests := child(pv, "guests"); guests != nil {
				if guests.Tag != "!!bool" || guests.Decode(&p.Guests) != nil {
					stop(b, guests.Line, OfficeFile, "bridges: %s's place %s has guests %s, and guests is true or false", b.Name, p.Name, describe(guests))
					good = false
				}
			}
			if good {
				b.Places = append(b.Places, p)
			}
		}
	}
	// The folders under bridges/, and the bridges they are. A bridge that
	// runs a folder Bulldog ships has none of its own.
	folders := r.bridgeFolders()
	for _, name := range r.o.BridgeOrder {
		b := r.o.Bridges[name]
		if froms[name] {
			if folders[name] {
				stop(b, 0, "bridges/"+name, "bridges/%s is not run: %s has a from, and a bridge with a from runs no folder of the office's own", name, name)
			}
			continue
		}
		if !folders[name] {
			stop(b, 0, OfficeFile, "there is no folder bridges/%s, which holds its in.yaml and out.yaml", name)
			continue
		}
		for _, machine := range []string{"in.yaml", "out.yaml"} {
			if _, ok := r.files["bridges/"+name+"/"+machine]; !ok {
				stop(b, 0, "bridges/"+name, "there is no %s in bridges/%s: a bridge's folder holds an in machine and an out machine", machine, name)
			}
		}
	}
	for _, folder := range slices.Sorted(maps.Keys(folders)) {
		if r.o.Bridges[folder] == nil {
			b := &Bridge{Name: folder}
			stop(b, 0, "bridges/"+folder, "bridges/%s is a bridge's folder that bridges in office.yaml does not name, so it carries nothing", folder)
			r.o.Bridges[folder] = b
			r.o.BridgeOrder = append(r.o.BridgeOrder, folder)
		}
	}
}

// bridgeFolders gives the names of the folders under bridges/. A person may
// be written with who they are on a bridge whose folder is there and whose
// entry is not: that is a problem of the bridge alone.
func (r *reading) bridgeFolders() map[string]bool {
	folders := map[string]bool{}
	for _, file := range r.o.Files {
		if rest, ok := strings.CutPrefix(file, "bridges/"); ok && strings.Contains(rest, "/") {
			folders[strings.Split(rest, "/")[0]] = true
		}
	}
	return folders
}

// called gives a team as a problem names it: the office, or its name.
func called(t *Team) string {
	if t.Name == Top {
		return "the office"
	}
	return t.Name
}
