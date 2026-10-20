package office

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/agent"
	"github.com/zummed/corbie/machine"

	// The machine types a definition may be, and the plugins and model
	// services Corbie checks a definition against.
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/plugin/bundled"
	_ "github.com/zummed/corbie/statechart"
)

// isType says whether a definition's first key is a type of machine.
func isType(first string) bool { return slices.Contains(machine.Types(), first) }

// loader gives Corbie the files a definition in a folder of the office
// names, by their paths from that folder: the office's own, and Bulldog's
// at .bulldog/ at the top of the office, and nothing else. A definition it
// gives has its models named by one word filled in, from the team whose
// folder holds it. Each file of the office it gives is noted in uses.
func (r *reading) loader(dir string, uses *[]string) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		full := path.Join(dir, name)
		if own, ok := strings.CutPrefix(full, shipped.Dir+"/"); ok {
			if contents, ok := shipped.File(own); ok {
				return contents, nil
			}
			return nil, fmt.Errorf("Bulldog ships no %s: its tools files are %s, %s and %s, its ask %s, and its rules each in a folder of %s/rules/, each named by its path in %s/ at the top of the office, as ./%s", full,
				path.Join(shipped.Dir, shipped.Talk), path.Join(shipped.Dir, shipped.Notes), path.Join(shipped.Dir, shipped.Build), path.Join(shipped.Dir, shipped.Ask), shipped.Dir, shipped.Dir, path.Join(shipped.Dir, shipped.Talk))
		}
		switch {
		case path.IsAbs(full) || full == ".." || strings.HasPrefix(full, "../"):
			return nil, errors.New("it is outside the office's folder")
		case hasDot(full):
			return nil, fmt.Errorf("%s is not a file of the office: nothing whose name starts with a dot is the office's, but for what Bulldog ships at %s/ at the top", full, shipped.Dir)
		}
		contents, ok := r.files[full]
		if !ok {
			return nil, fmt.Errorf("there is no %s in the office", full)
		}
		if uses != nil && !slices.Contains(*uses, full) {
			*uses = append(*uses, full)
		}
		if ext := path.Ext(full); ext != ".yaml" && ext != ".yml" {
			return contents, nil
		}
		filled, _, _, problems := r.fill(contents, r.teamAt(dirOf(full)), full)
		if len(problems) > 0 {
			return nil, errors.New(strings.Join(problems, "; "))
		}
		return filled, nil
	}
}

// fill gives a definition with every model it names by one word written
// out as the nearest team's model of that name, its own team's first: the
// definition's own model, and that of each agent written in place in it.
// It gives too the definition's own model, as named and as filled in.
// Anything else is as written; a word it cannot fill is a problem.
func (r *reading) fill(src []byte, t *Team, file string) (filled []byte, word, model string, problems []string) {
	root, err := document(src)
	if err != nil || root == nil || root.Kind != yaml.MappingNode || len(root.Content) == 0 {
		return src, "", "", nil // what is wrong with it is Corbie's to say
	}
	var words []*yaml.Node
	var models []string
	for _, agent := range agents(root) {
		at := child(agent, "model")
		switch {
		case at == nil:
			continue
		case at.Kind == yaml.MappingNode && agent == root:
			model = inRow(at)
			continue
		case at.Kind != yaml.ScalarNode:
			continue
		}
		if agent == root {
			word = at.Value
		}
		m, ok := r.o.Model(t.Name, at.Value)
		if !ok {
			problems = append(problems, r.noModel(t, file, at.Value))
			continue
		}
		if agent == root {
			model = m.Text
		}
		words, models = append(words, at), append(models, m.Text)
	}
	if len(problems) > 0 {
		return nil, word, "", problems
	}
	if len(words) == 0 {
		return src, word, model, nil
	}
	if filled, ok := splice(src, words, models); ok {
		return filled, word, model, nil
	}
	return nil, word, "", []string{fmt.Sprintf("model: %s names a model by a word written so that Bulldog cannot find its place to fill it in: write the word on the line of its model:, as model: usual", file)}
}

// noModel says that no team from a definition's own out to the office
// gives a model a word names.
func (r *reading) noModel(t *Team, file, word string) string {
	var given []string
	for team := t; team != nil; team = r.o.Teams[team.Within] {
		for _, m := range team.Models {
			if !slices.Contains(given, m.Name) {
				given = append(given, m.Name)
			}
		}
	}
	where := "the office gives no model of that name"
	if t.Name != Top {
		where = "no team from " + t.Name + " out to the office gives a model of that name"
	}
	if len(given) > 0 {
		where += " (the names given are " + strings.Join(given, ", ") + ")"
	}
	return fmt.Sprintf("model: %s names its model %q, and %s: give it in a team's models, or write the model out, as { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }",
		file, word, where)
}

// agents gives the agents a definition holds: itself, when it is one, and
// each machine written in place in it, under a machine: key, that is one.
func agents(root *yaml.Node) []*yaml.Node {
	var found []*yaml.Node
	if root.Content[0].Value == "agent" {
		found = append(found, root)
	}
	var walk func(*yaml.Node)
	walk = func(node *yaml.Node) {
		for k, v := range pairs(node) {
			if k.Value == "machine" && v.Kind == yaml.MappingNode && child(v, "agent") != nil {
				found = append(found, v)
			}
		}
		for _, c := range node.Content {
			walk(c)
		}
	}
	walk(root)
	return found
}

// splice puts each model, written in a row, in place of the word written
// at the matching node, changing nothing else of the text. It says whether
// every word's place was found, and each is then a model.
func splice(src []byte, words []*yaml.Node, models []string) ([]byte, bool) {
	lines := bytes.SplitAfter(src, []byte("\n"))
	type cut struct {
		at, n int
		model string
	}
	var cuts []cut
	for i, at := range words {
		if at.Line < 1 || at.Line > len(lines) {
			return nil, false
		}
		start := len(bytes.Join(lines[:at.Line-1], nil))
		line := lines[at.Line-1]
		column := 0
		for c := 1; c < at.Column && column < len(line); c++ {
			_, size := utf8.DecodeRune(line[column:])
			column += size
		}
		n := tokenLength(line[column:], at)
		if n == 0 {
			return nil, false
		}
		cuts = append(cuts, cut{start + column, n, models[i]})
	}
	// From the last to the first, so each cut is where it was found.
	slices.SortFunc(cuts, func(a, b cut) int { return b.at - a.at })
	out := slices.Clone(src)
	for _, c := range cuts {
		out = slices.Concat(out[:c.at], []byte(c.model), out[c.at+c.n:])
	}
	root, err := document(out)
	if err != nil || root == nil {
		return nil, false
	}
	for _, agent := range agents(root) {
		if at := child(agent, "model"); at != nil && at.Kind == yaml.ScalarNode {
			return nil, false
		}
	}
	return out, true
}

// tokenLength gives how long a scalar is as written at the start of a
// line, or nothing when it cannot be told.
func tokenLength(rest []byte, at *yaml.Node) int {
	switch at.Style {
	case 0:
		if bytes.HasPrefix(rest, []byte(at.Value)) {
			return len(at.Value)
		}
	case yaml.DoubleQuotedStyle:
		for i := 1; i < len(rest); i++ {
			switch rest[i] {
			case '\\':
				i++
			case '"':
				return i + 1
			}
		}
	case yaml.SingleQuotedStyle:
		for i := 1; i < len(rest); i++ {
			if rest[i] == '\'' {
				if i+1 < len(rest) && rest[i+1] == '\'' {
					i++
					continue
				}
				return i + 1
			}
		}
	}
	return 0
}

// corbieSays gives what Corbie found wrong with a definition, as problems
// of its file, or of the file of a definition it runs, each where in the
// definition Corbie says it is.
func corbieSays(err error, p Problem) Problems {
	var found machine.Problems
	if !errors.As(err, &found) {
		p.Text = err.Error()
		return Problems{p}
	}
	var out Problems
	for _, problem := range found {
		p := p
		p.Place = problem.Place
		p.Text = machine.Problem{Text: problem.Text}.String() // on one line, as Corbie says it
		if problem.File != "" {
			p.File = path.Join(dirOf(p.File), problem.File)
		}
		out = append(out, p)
	}
	return out
}

// checkWorkers has Corbie check each worker, as it would before a start,
// with its models named by one word filled in. A worker Bulldog starts at
// the top that can hold an action names the ask Bulldog ships, and every
// ask a worker names is that one.
func (r *reading) checkWorkers() {
	for _, team := range r.o.Order {
		for _, name := range r.o.Teams[team].Workers {
			w := r.o.Workers[name]
			r.checkWorker(w)
			r.problems = append(r.problems, w.Problems...)
		}
	}
}

func (r *reading) checkWorker(w *Worker) {
	if len(w.Problems) > 0 {
		return
	}
	p := Problem{File: w.File, Team: w.Team, Worker: w.Name}
	stop := func(format string, args ...any) {
		p := p
		p.Text = fmt.Sprintf(format, args...)
		w.Problems = append(w.Problems, p)
	}
	filled, word, model, problems := r.fill(w.Written, r.o.Teams[w.Team], w.File)
	w.ModelName, w.Model = word, model
	if len(problems) > 0 {
		for _, problem := range problems {
			stop("%s", problem)
		}
		return
	}
	w.Definition = filled
	r.o.Checked = append(r.o.Checked, w.File)
	uses := []string{w.File}
	draft, err := machine.ParseMade(filled, r.loader(dirOf(w.File), &uses))
	slices.Sort(uses)
	w.Uses = uses
	if err != nil {
		w.Problems = append(w.Problems, r.colleagues(w, corbieSays(err, p))...)
		return
	}
	r.filledOut[w.Name] = true
	def := draft.Definition
	if def.Name != w.Name {
		stop("%s: the definition is called %s, and a worker's definition carries its file's name: write %s: %s", def.Type, def.Name, def.Type, w.Name)
	}
	ask := path.Join(relativeToTop(dirOf(w.File)), shipped.Dir, shipped.Ask)
	if !strings.HasPrefix(ask, "../") {
		ask = "./" + ask
	}
	if def.Ask != nil && (def.Ask.Machine == "" || path.Join(dirOf(w.File), def.Ask.Machine) != path.Join(shipped.Dir, shipped.Ask)) {
		stop("ask: %s names an ask of its own, and every ask a worker names is the one Bulldog ships: ask: { machine: %s }", w.Name, ask)
	}
	if w.AtTop && needsAsk(draft) {
		stop("ask: something %s runs can wait for a yes, and Bulldog starts it at the top, as its team's lead or a routine's worker, so it names the ask Bulldog ships: ask: { machine: %s }", w.Name, ask)
	}
	w.Held = r.toolsOf(w, def)
	w.Tools = toolNames(w.Held)
	if len(w.Problems) == 0 {
		w.Draft = draft
	}
}

// colleagues says once what Corbie found wrong in a worker's definition
// that is in the file of another worker it uses: that one cannot run, and
// its problems are said on it. Corbie refuses a definition when it refuses
// one the definition runs, so the worker cannot run either.
func (r *reading) colleagues(w *Worker, problems Problems) Problems {
	var kept Problems
	var used []string
	for _, p := range problems {
		other := r.workerOf(p.File)
		if other == nil || other == w {
			kept = append(kept, p)
			continue
		}
		if !slices.Contains(used, other.Name) {
			used = append(used, other.Name)
			p.File, p.Place = w.File, ""
			p.Text = fmt.Sprintf("it uses %s, in %s, which Corbie refuses, so Corbie refuses this definition too until %s is mended", other.Name, other.File, other.Name)
			kept = append(kept, p)
		}
	}
	return kept
}

// workerOf gives the worker whose file is a file of the office, or nil.
func (r *reading) workerOf(file string) *Worker {
	for _, w := range r.o.Workers {
		if w.File == file {
			return w
		}
	}
	return nil
}

// relativeToTop gives the way from a folder of the office to its top.
func relativeToTop(dir string) string {
	if dir == "" {
		return "."
	}
	return strings.Repeat("../", strings.Count(dir, "/")+1)
}

// needsAsk says whether a definition, or a machine it runs, as far down
// as they go, holds a job that waits for a yes with no ask of its own for
// it to go to.
func needsAsk(d *machine.Draft) bool {
	_, held := machine.Check(d.Definition)
	for name, c := range d.Children {
		if d.Definition.Ask != nil && d.Definition.Ask.Machine == name {
			continue
		}
		held = held || needsAsk(c)
	}
	return held && d.Definition.Ask == nil
}

// toolsOf gives the tools an agent's definition holds, as Corbie filled it
// out, but for those every agent has for blobs, in the order of their
// names. What each tool's job is, is told by where it resolves from the
// worker's folder: a tool whose job is the call Bulldog ships is Bulldog's;
// a script is a script; a machine that is a file of the office is a worker
// of the same team, or another machine; and a machine that is no file was
// written in place.
func (r *reading) toolsOf(w *Worker, def *machine.Definition) []Tool {
	if def.Type != "agent" {
		return nil
	}
	var a agent.Agent
	if json.Unmarshal(def.Body, &a) != nil {
		return nil
	}
	var tools []Tool
	for _, name := range slices.Sorted(maps.Keys(a.Tools)) {
		t := a.Tools[name]
		var script, machine string
		if t.Job != nil {
			script, machine = t.Job.Script, t.Job.Machine
		}
		if strings.HasPrefix(script, "corbie:") {
			continue
		}
		tool := Tool{Name: name, Does: t.Does, Kind: ToolMachine, Args: t.Args, Listed: t.List != nil}
		file := path.Join(dirOf(w.File), script+machine)
		_, inOffice := r.files[file]
		switch {
		case file == path.Join(shipped.Dir, shipped.Call):
			tool.Kind = ToolBulldog
		case script != "":
			tool.Kind, tool.File = ToolScript, file
		case inOffice:
			tool.File = file
			if other := r.workerOf(file); other != nil && other.Team == w.Team {
				tool.Kind, tool.Worker = ToolWorker, other.Name
			}
		}
		tools = append(tools, tool)
	}
	return tools
}

// toolNames gives the names of tools.
func toolNames(tools []Tool) []string {
	var out []string
	for _, t := range tools {
		out = append(out, t.Name)
	}
	return out
}

// checkRules has Corbie check each rule the lists and the heeds name that
// is a definition, as it is written, with its models named by one word
// filled in from its team's; a refused one is reported on each team that
// names it, and is a problem of each heed that names it. A rule Bulldog
// ships is of no team: it is read from what Bulldog ships, and checked as
// it ships, with nothing filled in, since it is the same rule whichever
// team names it. A bare script is kept as it is written, in no definition:
// Corbie checks of a script only that it can be read beside what names it,
// and the reading has already found it a file of its team's folder or one
// Bulldog ships.
func (r *reading) checkRules() {
	for _, team := range r.o.Order {
		t := r.o.Teams[team]
		var named []string
		for _, list := range append([]List{{Key: "disclosure", Deciders: t.Disclosure}}, t.Decide...) {
			for _, d := range list.Deciders {
				if d.Kind == IsRule && !slices.Contains(named, d.Name) {
					named = append(named, d.Name)
				}
			}
		}
		if t.Heed != nil && !slices.Contains(named, t.Heed.Rule) {
			named = append(named, t.Heed.Rule)
		}
		for _, name := range named {
			rule := r.o.Rules[name]
			if rule == nil {
				rule = r.checkRule(t, name)
				r.o.Rules[name] = rule
				r.o.Checked = append(r.o.Checked, name)
			}
			for _, p := range rule.Problems {
				p.Team = team
				r.problems = append(r.problems, p)
				if t.Heed != nil && t.Heed.Rule == name {
					t.Heed.Problems = append(t.Heed.Problems, p)
				}
			}
		}
	}
}

// checkRule reads and checks one rule a team names.
func (r *reading) checkRule(t *Team, name string) *Rule {
	rule := &Rule{Team: t.Name, Path: name, Script: path.Ext(name) == ".js", Written: r.files[name]}
	own, ships := strings.CutPrefix(name, shipped.Dir+"/")
	if ships {
		rule.Team = ""
		rule.Written, _ = shipped.File(own)
	}
	if rule.Script {
		return rule
	}
	p := Problem{File: name, Rule: name}
	filled := rule.Written
	if !ships {
		var problems []string
		filled, _, _, problems = r.fill(rule.Written, t, name)
		for _, problem := range problems {
			p.Text = problem
			rule.Problems = append(rule.Problems, p)
		}
	}
	if len(rule.Problems) == 0 {
		draft, err := machine.Parse(filled, r.loader(dirOf(name), nil))
		if err != nil {
			rule.Problems = corbieSays(err, p)
		} else {
			rule.Draft = draft
		}
	}
	return rule
}

// checkBridges has Corbie check both machines of each bridge, with their
// models named by one word filled in from the office's; the machines of a
// bridge Bulldog ships name none. An in machine takes a cursor, and
// listens, or a delivery, and hears.
func (r *reading) checkBridges() {
	for _, name := range r.o.BridgeOrder {
		b := r.o.Bridges[name]
		dir := b.Dir()
		for _, which := range []string{"in.yaml", "out.yaml"} {
			file := dir + "/" + which
			src, ok := r.files[file]
			if b.From != "" {
				src, ok = shipped.File(strings.TrimPrefix(file, shipped.Dir+"/"))
			}
			if !ok {
				continue
			}
			p := Problem{File: file, Team: Top, Bridge: name}
			r.o.Checked = append(r.o.Checked, file)
			filled := src
			if b.From == "" {
				var problems []string
				filled, _, _, problems = r.fill(src, r.o.Teams[Top], file)
				if len(problems) > 0 {
					for _, problem := range problems {
						p.Text = problem
						b.Problems = append(b.Problems, p)
					}
					continue
				}
			}
			draft, err := machine.Parse(filled, r.loader(dir, nil))
			if err != nil {
				b.Problems = append(b.Problems, corbieSays(err, p)...)
				continue
			}
			if which == "out.yaml" {
				b.Out = draft
				continue
			}
			b.In = draft
			b.Listens, b.Hears = slices.Contains(draft.Definition.Input, "cursor"), slices.Contains(draft.Definition.Input, "delivery")
			if b.Listens == b.Hears {
				p.Text = "input: an in machine takes cursor, and listens, or delivery, and hears, and this takes both or neither"
				b.Problems = append(b.Problems, p)
			}
		}
		if len(b.Problems) > 0 {
			b.In, b.Out = nil, nil
		}
		r.problems = append(r.problems, b.Problems...)
	}
}

// checkKeys reports a decide key that names no tool of the team's
// workers, and a team whose lead cannot run. While a worker's definition
// is one Corbie did not fill out, its tools are not known, and no key is
// said to name none.
func (r *reading) checkKeys() {
	for _, team := range r.o.Order {
		t := r.o.Teams[team]
		if w := r.o.Workers[t.Lead]; w != nil && !w.Runs() {
			r.report(Problem{File: t.File, Team: team, Text: fmt.Sprintf("lead: %s is the lead of %s, and cannot run until its problems are mended", w.Name, called(t))})
		}
		var tools, listed []string
		known := true
		for _, name := range t.Workers {
			known = known && r.filledOut[name]
			for _, tool := range r.o.Workers[name].Held {
				tools = append(tools, tool.Name)
				if tool.Listed {
					listed = append(listed, tool.Name)
				}
			}
		}
		if !known {
			continue
		}
		// A tool a list: entry gives is known only when a call is made, by
		// the entry's name and its own.
		holds := func(key string) bool {
			return slices.Contains(tools, key) || slices.ContainsFunc(listed, func(entry string) bool { return strings.HasPrefix(key, entry+"_") })
		}
		for _, list := range t.Decide {
			if list.Key != "everything" && list.Key != "change" && !holds(list.Key) {
				line := 0
				for k := range pairs(child(r.nodes[team], "decide")) {
					if k.Value == list.Key {
						line = k.Line
					}
				}
				r.report(Problem{File: t.File, Line: line, Team: team,
					Text: fmt.Sprintf("decide: %s names no tool any worker of %s holds, so it decides nothing", list.Key, called(t))})
			}
		}
	}
}
