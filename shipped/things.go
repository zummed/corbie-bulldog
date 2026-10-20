package shipped

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie/machine"
	"go.yaml.in/yaml/v3"

	// The machine types and plugins Corbie reads a shipped definition with.
	_ "github.com/zummed/corbie/agent"
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/plugin/bundled"
	_ "github.com/zummed/corbie/statechart"
)

// Kind gives what a shipped thing is, by its folder: a bridge, a tools
// file or a rule.
func Kind(thing string) string {
	kind, _, _ := strings.Cut(thing, "/")
	switch kind {
	case "bridges":
		return "bridge"
	case "rules":
		return "rule"
	}
	return kind
}

// filesOf gives the files of a shipped thing's folder, by their paths in
// Dir, in the order they sort.
func filesOf(thing string) []string {
	var files []string
	for _, name := range Names() {
		if path.Dir(name) == thing {
			files = append(files, name)
		}
	}
	return files
}

// Files gives the files of a shipped thing's folder by their names in it,
// the page and a bridge's entry among them.
func Files(thing string) []string {
	var names []string
	for _, name := range filesOf(thing) {
		names = append(names, path.Base(name))
	}
	return names
}

// Reach gives every permission a shipped thing's definitions are allowed,
// as written, sorted: what each job allows, and the whole definition, of
// its machines and those they run. A tools file's are those of each
// tool's job. A bare script is given none.
func Reach(thing string) []string {
	var allow []string
	for _, name := range filesOf(thing) {
		if ext := path.Ext(name); ext != ".yaml" || path.Base(name) == Settings {
			continue
		}
		if slices.Contains(ToolsFiles, name) {
			allow = append(allow, toolsReach(name)...)
			continue
		}
		allow = append(allow, allowed(name, parsed(name))...)
	}
	slices.Sort(allow)
	return slices.Compact(allow)
}

// parsed gives a shipped definition as Corbie reads it, with the files it
// names read from the shipped tree.
func parsed(name string) *machine.Draft {
	src, _ := File(name)
	d, err := machine.Parse(src, func(file string) ([]byte, error) {
		full := path.Join(path.Dir(name), file)
		if contents, ok := File(full); ok {
			return contents, nil
		}
		return nil, fmt.Errorf("Bulldog ships no %s", full)
	})
	if err != nil {
		panic(fmt.Sprintf("the shipped %s cannot be read: %v", name, err))
	}
	return d
}

// allowed gives what a definition in a file and every machine it runs are
// allowed, as written.
func allowed(file string, d *machine.Draft) []string {
	var out []string
	Machines(file, d, func(_ string, d *machine.Draft) bool {
		allow, _, _ := Own(d.Definition)
		out = append(out, allow...)
		return true
	})
	return out
}

// Machines walks a definition in a file and every machine it runs, as
// Corbie read them, calling at with each and the file it is written in. A
// machine named by its path is in that file, beside the one that names it;
// one written in place is in the file of the one it is written in. Where at
// says no, the machines that one runs are not walked. It is here so that
// what is shipped and what an office writes are read one way.
func Machines(file string, d *machine.Draft, at func(file string, d *machine.Draft) bool) {
	if d == nil || !at(file, d) {
		return
	}
	for _, name := range sortedKeys(d.Children) {
		in := file
		if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
			in = path.Join(path.Dir(file), name)
		}
		Machines(in, d.Children[name], at)
	}
}

// Own gives what a definition's own part may reach, as Corbie places it:
// what it is allowed and refused, by the lists written for the whole of
// it, those of its ask and those of its jobs; and within, the limits its
// machines' jobs put on those machines. Each job's deny is its own. A
// machine's job allows nothing of its own, its allow being a limit on the
// machine, whose own definition says what it is allowed; what it refuses,
// the machine is refused. An agent's turn is its model's, and the ask is
// the definition's own.
func Own(def *machine.Definition) (allow, deny, within []string) {
	allow = slices.Clone(def.Allow)
	deny = slices.Clone(def.Deny)
	if def.Ask != nil {
		allow = append(allow, def.Ask.Allow...)
		deny = append(deny, def.Ask.Deny...)
	}
	for _, j := range machine.Jobs(def) {
		switch {
		case j.Place == "turn" || j.Place == "ask":
			continue
		case j.Job.IsMachine():
			within = append(within, j.Job.Allow...)
		default:
			allow = append(allow, j.Job.Allow...)
		}
		deny = append(deny, j.Job.Deny...)
	}
	return allow, deny, within
}

// toolsReach gives what the tools of a shipped tools file allow: a
// script's job its own list, and a machine's what that machine allows.
func toolsReach(name string) []string {
	contents, _ := File(name)
	var tools map[string]struct {
		Job struct {
			Machine, Script string
			Allow           []string
		}
	}
	if err := yaml.Unmarshal(contents, &tools); err != nil {
		panic(fmt.Sprintf("the shipped %s cannot be read: %v", name, err))
	}
	var allow []string
	for _, tool := range tools {
		if tool.Job.Machine != "" {
			file := path.Join(path.Dir(name), tool.Job.Machine)
			allow = append(allow, allowed(file, parsed(file))...)
			continue
		}
		allow = append(allow, tool.Job.Allow...)
	}
	return allow
}

// A Setting is one a shipped thing is given: by its name, with the value
// it takes when none is written, if it has one, and its line on the page.
type Setting struct {
	Name  string `json:"name"`
	Value any    `json:"value,omitempty"`
	About string `json:"about,omitempty"`
}

// SettingsOf gives the settings of a shipped thing: a bridge's, each key
// of its entry as shipped, in order, with its value there; any other's,
// each setting its definitions read, sorted, with the default an
// expression that reads only it gives when it is not written. Each has
// its line from the page's Settings.
func SettingsOf(thing string) []Setting {
	var out []Setting
	if Kind(thing) == "bridge" {
		entry, _ := File(path.Join(thing, Settings))
		var doc yaml.Node
		if err := yaml.Unmarshal(entry, &doc); err != nil || len(doc.Content) == 0 {
			panic(fmt.Sprintf("the shipped %s/%s cannot be read: %v", thing, Settings, err))
		}
		m := doc.Content[0]
		for i := 0; i+1 < len(m.Content); i += 2 {
			var v any
			m.Content[i+1].Decode(&v)
			out = append(out, Setting{Name: m.Content[i].Value, Value: v})
		}
	} else {
		found := map[string]any{}
		for _, name := range filesOf(thing) {
			if path.Ext(name) != ".yaml" || slices.Contains(ToolsFiles, name) {
				continue
			}
			contents, _ := File(name)
			if err := SettingsIn(contents, found); err != nil {
				panic(fmt.Sprintf("the shipped %s cannot be read: %v", name, err))
			}
		}
		for _, name := range sortedKeys(found) {
			out = append(out, Setting{Name: name, Value: found[name]})
		}
	}
	about := pageLines(thing, "## Settings")
	for i := range out {
		line := about["`"+out[i].Name+"`"]
		// A line that begins with the default says it once more.
		if value, ok := strings.CutPrefix(line, "`"); ok {
			if _, rest, ok := strings.Cut(value, "`"); ok {
				line = strings.TrimLeft(rest, ".,; ")
			}
		}
		out[i].About = line
	}
	return out
}

// SettingsIn finds each setting a definition reads, as written, into
// found, with the default an expression that reads only it gives when it
// is not written.
func SettingsIn(contents []byte, found map[string]any) error {
	var def any
	if err := yaml.Unmarshal(contents, &def); err != nil {
		return err
	}
	readSettings(def, found)
	return nil
}

// readSettings finds each setting the text of a definition reads, with
// the default of a text that is one expression reading only it.
func readSettings(v any, found map[string]any) {
	switch v := v.(type) {
	case map[string]any:
		for _, item := range v {
			readSettings(item, found)
		}
	case []any:
		for _, item := range v {
			readSettings(item, found)
		}
	case string:
		var names []string
		for _, r := range machine.Reads(v, "input") {
			if r.Name == "settings" && r.Part != "" && r.Part != "*" {
				names = append(names, r.Part)
			}
		}
		for _, name := range names {
			if _, ok := found[name]; !ok {
				found[name] = nil
			}
		}
		if len(names) != 1 || !strings.HasPrefix(v, "${") || !strings.HasSuffix(v, "}") || strings.Count(v, "${") != 1 {
			return
		}
		scope := machine.Scope{"input": map[string]any{"settings": map[string]any{}}}
		if value, err := machine.NewEvaluator(time.Time{}, nil).Text(v, scope, v); err == nil && value != nil {
			found[names[0]] = value
		}
	}
}

// pageLines gives the lines of a section of a shipped thing's page that
// are a list, each by what it begins with before its first ": ", with
// what follows.
func pageLines(thing, heading string) map[string]string {
	page, _ := File(path.Join(thing, Page))
	out := map[string]string{}
	for _, line := range section(string(page), heading) {
		if key, text, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": "); ok && strings.HasPrefix(line, "- ") {
			out[key] = text
		}
	}
	return out
}

// section gives the lines of a page under a heading, up to the next.
func section(page, heading string) []string {
	var lines []string
	in := false
	for _, line := range strings.Split(page, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			in = strings.TrimSpace(line) == heading
		case in:
			lines = append(lines, line)
		}
	}
	return lines
}

// A Change is one line of a page's Changed: the version of Bulldog that
// made it, and what it changed.
type Change struct {
	Version string `json:"version"`
	Text    string `json:"text"`
}

// Changed gives a shipped thing's Changed lines, in the page's order.
func Changed(thing string) []Change {
	page, _ := File(path.Join(thing, Page))
	return ChangedIn(page)
}

// ChangedIn gives the Changed lines of a page, a shipped thing's or a
// copy of one, each of a version.
func ChangedIn(page []byte) []Change {
	var out []Change
	for _, line := range section(string(page), "## Changed") {
		version, text, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": ")
		if ok && strings.HasPrefix(line, "- ") && parts(version) != nil {
			out = append(out, Change{Version: version, Text: text})
		}
	}
	return out
}

// Newer says whether one version of Bulldog came after another; a
// version that is not numbers, as dev, is newer than none.
func Newer(version, than string) bool {
	a, b := parts(version), parts(than)
	if a == nil || b == nil {
		return false
	}
	return slices.Compare(a, b) > 0
}

// Latest gives the newest version Changed lines name, or "" for none.
func Latest(changes []Change) string {
	latest := ""
	for _, c := range changes {
		if latest == "" || Newer(c.Version, latest) {
			latest = c.Version
		}
	}
	return latest
}

// parts gives a version's numbers, as 0.2.0, or nil for one that is not
// written so.
func parts(version string) []int {
	var out []int
	for _, part := range strings.Split(version, ".") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil
		}
		out = append(out, n)
	}
	if len(out) != 3 {
		return nil
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
