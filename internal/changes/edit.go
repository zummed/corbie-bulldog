package changes

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// An Edit is a person's draft as the screens hold it: whole files written,
// or nil to remove one; fields of files set; workers moved to a team, by
// the team's name; workers taken away; and a bridge Bulldog ships added,
// by its name.
type Edit struct {
	Files  map[string]*string `json:"files,omitzero"`
	Set    []Setting          `json:"set,omitzero"`
	Move   map[string]string  `json:"move,omitzero"`
	Remove []string           `json:"remove,omitzero"`
	Bridge string             `json:"bridge,omitzero"`
}

// Edited gives the files a person's edit would change, each whole, against
// the folder as it is now, and anything they are to be told of how it was
// made, such as a file whose layout could not be kept. Bulldog keeps
// nothing of it: the screens hold it until it is put.
func (c *Changes) Edited(e Edit) ([]File, []string, error) {
	folder, _, err := c.folder()
	if err != nil {
		return nil, nil, err
	}
	o, problems := office.Parse(c.dir, folder)
	// The draft as it is made: a path's new text, or nil where it goes.
	draft := map[string]*string{}
	textOf := func(file string) (string, bool) {
		if t, ok := draft[file]; ok {
			if t == nil {
				return "", false
			}
			return *t, true
		}
		contents, ok := folder[file]
		return string(contents), ok
	}
	put := func(file string, t *string) error {
		if problem := refusedPath(file); problem != "" {
			return Refusal(problem)
		}
		if problem := placeProblem(folder, file); problem != "" {
			return Refusal(problem)
		}
		draft[file] = t
		return nil
	}
	var told []string
	for _, file := range sortedKeys(e.Files) {
		if err := put(file, e.Files[file]); err != nil {
			return nil, nil, err
		}
	}
	if (len(e.Move) > 0 || len(e.Remove) > 0) && problems.Refuse() {
		return nil, nil, refuse("the office does not read, so no worker of it can be moved or taken away: %v", problems)
	}
	for _, name := range e.Remove {
		w := o.Workers[name]
		if w == nil {
			return nil, nil, refuse("there is no worker %s to take away", name)
		}
		for _, file := range append([]string{w.File}, alone(o, w)...) {
			if err := put(file, nil); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, name := range sortedKeys(e.Move) {
		moved, err := movedFiles(o, name, e.Move[name], textOf)
		if err != nil {
			return nil, nil, err
		}
		for _, file := range sortedKeys(moved) {
			if err := put(file, moved[file]); err != nil {
				return nil, nil, err
			}
		}
	}
	// A folder under bridges/ that office.yaml does not name is read as a
	// bridge too, so a bridge is never added over one.
	if e.Bridge != "" && o.Bridges[e.Bridge] != nil {
		return nil, nil, refuse("the office has a bridge %s already, or a folder bridges/%s", e.Bridge, e.Bridge)
	}
	if e.Bridge != "" {
		kept, err := c.addBridge(e.Bridge, textOf, put)
		if err != nil {
			return nil, nil, err
		}
		if !kept {
			told = append(told, notKept(office.OfficeFile, []string{"bridges", e.Bridge}))
		}
	}
	for _, s := range e.Set {
		src, ok := textOf(s.Path)
		if !ok {
			return nil, nil, refuse("there is no file %s to set a field of", s.Path)
		}
		text, kept, err := setField(src, s.Field, s.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.Path, err)
		}
		if !kept {
			told = append(told, notKept(s.Path, s.Field))
		}
		if err := put(s.Path, &text); err != nil {
			return nil, nil, err
		}
	}
	var files []File
	for _, file := range sortedKeys(draft) {
		was, had := folder[file]
		f := File{Path: file, Was: string(was), Had: had}
		switch t := draft[file]; {
		case t == nil && !had:
			continue
		case t == nil:
			f.Gone = true
		case had && *t == string(was):
			continue
		default:
			f.Text = *t
		}
		files = append(files, f)
	}
	return files, told, nil
}

// notKept tells a person that a file is written plainly with a field set.
func notKept(file string, field []string) string {
	return fmt.Sprintf("%s is written plainly with %s set: how it was set out could not be kept and say the same", file, strings.Join(field, "."))
}

// movedFiles gives what moving a worker to a team writes: its definition
// and the files only it uses, in the new team's folder, each where it was
// in the old one, and gone from there; with every path its moved files
// name pointing where that file now is. A moved file is never written over
// one already there.
func movedFiles(o *office.Office, name, team string, textOf func(string) (string, bool)) (map[string]*string, error) {
	w := o.Workers[name]
	to := o.Teams[team]
	switch {
	case w == nil:
		return nil, refuse("there is no worker %s to move", name)
	case to == nil:
		return nil, refuse("there is no team %s to move %s to", team, name)
	case w.Team == team:
		return nil, refuse("%s is a worker of %s already", name, team)
	}
	where := takes(o, w, to.Dir)
	where[w.File] = path.Join(to.Dir, path.Base(w.File))
	out := map[string]*string{}
	for _, old := range sortedKeys(where) {
		now := where[old]
		if _, there := textOf(now); there {
			return nil, refuse("%s is there already, and moving %s would write over it: move or rename it first", now, name)
		}
		text, ok := textOf(old)
		if !ok {
			return nil, refuse("%s, which %s uses, is not there", old, name)
		}
		if ext := path.Ext(old); ext == ".yaml" || ext == ".yml" {
			repointed, err := repoint(text, old, now, where, textOf)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", old, err)
			}
			text = repointed
		}
		out[now] = &text
		out[old] = nil
	}
	return out, nil
}

// repoint gives a definition moved from one file to another with every
// path it names written from where it now is: to a file that moves with
// it, where that file goes, and to any other, where it stays.
func repoint(src, old, now string, where map[string]string, textOf func(string) (string, bool)) (string, error) {
	doc := &yaml.Node{}
	if err := yaml.Unmarshal([]byte(src), doc); err != nil {
		return "", refuse("it is not YAML that can be read: %v", err)
	}
	t := newText(src)
	type cut struct {
		at, n int
		with  string
	}
	var cuts []cut
	var failed error
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && (strings.HasPrefix(n.Value, "./") || strings.HasPrefix(n.Value, "../")) {
			target := path.Join(path.Dir(old), n.Value)
			_, isFile := textOf(target)
			_, isShipped := shipped.File(strings.TrimPrefix(target, shipped.Dir+"/"))
			if !isFile && !(strings.HasPrefix(target, shipped.Dir+"/") && isShipped) {
				return
			}
			if moved, ok := where[target]; ok {
				target = moved
			}
			value := relative(path.Dir(now), target)
			if value == n.Value {
				return
			}
			at := t.at(n.Line, n.Column)
			length := 0
			if at >= 0 {
				length = tokenLength(src[at:], n)
			}
			with, ok := inline(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: n.Style & (yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle)}, false)
			if length == 0 || !ok {
				failed = refuse("it names %s on line %d in a way Bulldog cannot rewrite in place: move it by writing its files whole", n.Value, n.Line)
				return
			}
			cuts = append(cuts, cut{at, length, with})
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc)
	if failed != nil {
		return "", failed
	}
	slices.SortFunc(cuts, func(a, b cut) int { return b.at - a.at })
	out := src
	for _, c := range cuts {
		out = out[:c.at] + c.with + out[c.at+c.n:]
	}
	return out, nil
}

// relative gives the path from a folder of the office to a file of it, as
// a definition names it: ./tools/x.js, or ../tools/x.js.
func relative(from, to string) string {
	up := ""
	for from != "." && from != "" && !strings.HasPrefix(to, from+"/") {
		from = path.Dir(from)
		up += "../"
	}
	rest := to
	if from != "." && from != "" {
		rest = strings.TrimPrefix(to, from+"/")
	}
	if up == "" {
		return "./" + rest
	}
	return up + rest
}

// addBridge puts a bridge Bulldog ships into a draft: its folder as
// Bulldog ships it, under bridges/<name>, and its entry under bridges in
// office.yaml, with the settings it is shipped with. It says whether
// office.yaml kept how it was set out.
func (c *Changes) addBridge(name string, textOf func(string) (string, bool), put func(string, *string) error) (kept bool, err error) {
	if c.Shipped == nil {
		return false, refuse("Bulldog ships no bridge %s", name)
	}
	tree, settings, ok := c.Shipped(name)
	if !ok {
		return false, refuse("Bulldog ships no bridge %s", name)
	}
	dir := path.Join("bridges", name)
	err = fs.WalkDir(tree, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		contents, err := fs.ReadFile(tree, file)
		if err != nil {
			return err
		}
		text := string(contents)
		return put(path.Join(dir, file), &text)
	})
	var refusal Refusal
	switch {
	case errors.As(err, &refusal):
		return false, err
	case err != nil:
		return false, fmt.Errorf("the shipped bridge %s cannot be read: %w", name, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(settings), &node); err != nil {
		return false, fmt.Errorf("the settings the bridge %s is shipped with cannot be read: %w", name, err)
	}
	if len(node.Content) == 0 {
		return false, fmt.Errorf("the bridge %s is shipped with no settings", name)
	}
	src, _ := textOf(office.OfficeFile)
	text, kept, err := setField(src, []string{"bridges", name}, node.Content[0])
	if err != nil {
		return false, fmt.Errorf("%s: %w", office.OfficeFile, err)
	}
	return kept, put(office.OfficeFile, &text)
}
