package changes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// A Drafter is whoever drafted a change: a worker, with the builder's
// tools, or a person, on the screens.
type Drafter struct {
	Name   string
	Person bool
}

// A Check is what a change would do, read from the office as it would make
// it: every problem it would bring, those the office has already beside,
// what else keeps it from being put, and what it says, with what lets more
// by apart. It puts nothing to be decided.
type Check struct {
	Files []File `json:"files"`
	// Problems are those the office as it is does not have, each of which
	// refuses the change; Standing are those it has, which do not.
	Problems office.Problems `json:"problems"`
	Standing office.Problems `json:"standing"`
	// Refused says what else keeps it from being put.
	Refused    []string `json:"refused"`
	Says       []string `json:"says"`
	LetsMoreBy []string `json:"lets_more_by"`
	// Taken says what a worker the change moves takes with it, which the
	// check added to its files.
	Taken []string `json:"taken,omitzero"`
	// Mark is what was checked: the same files against the same folder
	// give the same mark, and a folder changed in any file another.
	Mark string `json:"mark"`
	// Gone are the workers it takes away.
	Gone []string `json:"gone,omitzero"`
}

// Fit says whether the change may be put to be decided.
func (c Check) Fit() bool { return len(c.Problems) == 0 && len(c.Refused) == 0 }

// Text says the check as the builder is told it.
func (c Check) Text() string {
	var b strings.Builder
	list := func(heading string, lines []string) {
		if len(lines) == 0 {
			return
		}
		b.WriteString(heading + "\n")
		for _, line := range lines {
			b.WriteString("- " + line + "\n")
		}
		b.WriteString("\n")
	}
	list("The office as your draft would make it has these problems, which it has not now. Mend each before you put it:", lines(c.Problems))
	list("It cannot be put:", c.Refused)
	list("A worker it moves takes with it the files only it used, and they are added to your draft:", c.Taken)
	list("The office has these problems already, and your draft leaves them as they were, so they do not stop it:", lines(c.Standing))
	list("What it does, as whoever decides is told:", c.Says)
	list("What it lets more by, set apart for whoever decides:", c.LetsMoreBy)
	if c.Fit() {
		b.WriteString("It reads well, and may be put with office_put.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// lines gives problems one to a line.
func lines(ps office.Problems) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

// Check reads the office as a change would make it, with every worker, rule
// and bridge checked by Corbie, and says every problem it would bring
// together, each once; what else keeps it from being put; and what it
// does. The office as it is is read from the folder now. A worker the change
// moves takes with it the files only it used, and the check's files are the
// change's with those added, in the order of their paths.
func (c *Changes) Check(ctx context.Context, files []File, by Drafter) (Check, error) {
	folder, left, err := c.folder()
	if err != nil {
		return Check{}, err
	}
	return c.check(folder, left, files, by)
}

func (c *Changes) check(folder map[string][]byte, left office.Problems, files []File, by Drafter) (Check, error) {
	out := Check{Refused: []string{}, Says: []string{}, LetsMoreBy: []string{}, Problems: office.Problems{}, Standing: office.Problems{}}
	was, wasProblems := office.Parse(c.dir, folder)
	wasProblems = once(append(slices.Clone(left), wasProblems...))
	files, out.Taken, out.Refused = withMoves(c.dir, was, folder, files)
	out.Files = files
	laid := overlay(folder, files)
	now, nowProblems := office.Parse(c.dir, laid)
	touched := map[string]bool{}
	for _, f := range files {
		touched[f.Path] = true
	}
	for _, p := range left {
		if !touched[p.File] {
			nowProblems = append(nowProblems, p)
		}
	}
	nowProblems = once(nowProblems)
	had := map[string]bool{}
	for _, p := range wasProblems {
		had[problemKey(p)] = true
	}
	// Where the office as it is cannot be read at all, only a change after
	// which it reads is accepted.
	unread := wasProblems.Refuse()
	for _, p := range nowProblems {
		if had[problemKey(p)] && !(unread && p.Refuses) {
			out.Standing = append(out.Standing, p)
		} else {
			out.Problems = append(out.Problems, p)
		}
	}
	if len(files) == 0 {
		out.Refused = append(out.Refused, "the draft is empty: it changes nothing")
	}
	for _, f := range files {
		contents, has := folder[f.Path]
		if has != f.Had || has && string(contents) != f.Was {
			out.Refused = append(out.Refused, fmt.Sprintf("%s is no longer as it was when it was drafted: it has changed in the folder since. Read it again, and draft it anew", f.Path))
		}
	}
	repo, err := openRepo(c.dir)
	if err != nil {
		out.Refused = append(out.Refused, err.Error())
	} else {
		edited, err := repo.edited(paths(files))
		if err != nil {
			return Check{}, fmt.Errorf("the office's repository cannot be read: %w", err)
		}
		for _, f := range edited {
			out.Refused = append(out.Refused, fmt.Sprintf("%s has an edit by hand that is not yet committed: commit it, or undo it, first", f))
		}
	}
	if !by.Person {
		out.Refused = append(out.Refused, guard(was, now, folder, laid, files)...)
	}
	out.Says, out.LetsMoreBy, err = describe(reading{was, folder}, reading{now, laid}, files)
	if err != nil {
		return Check{}, err
	}
	if out.Says == nil {
		out.Says = []string{}
	}
	if out.LetsMoreBy == nil {
		out.LetsMoreBy = []string{}
	}
	for _, name := range workerNames(was) {
		if now.Workers[name] == nil {
			out.Gone = append(out.Gone, name)
		}
	}
	out.Mark = mark(folder, files)
	return out, nil
}

// withMoves gives a change with what each worker it moves takes with it:
// each file only its definition used, in its old team's own folder, that
// the change leaves where it is and that nothing in the office as it would
// make it reads, goes to the same place in the new team's folder. One that
// would be written over a file there is not taken, and the change is
// refused, naming it. It says what was taken, and what was refused.
func withMoves(dir string, was *office.Office, folder map[string][]byte, files []File) (out []File, taken, refused []string) {
	out = slices.Clone(files)
	refused = []string{}
	touched := map[string]bool{}
	for _, f := range files {
		touched[f.Path] = true
	}
	now, _ := office.Parse(dir, overlay(folder, files))
	used := usedIn(now, "")
	for _, name := range workerNames(was) {
		a, b := was.Workers[name], now.Workers[name]
		if b == nil || a.Team == b.Team {
			continue
		}
		where := takes(was, a, now.Teams[b.Team].Dir)
		for _, f := range sortedKeys(where) {
			to := where[f]
			_, there := folder[to]
			switch {
			case touched[f] || used[f]:
				continue
			case there:
				refused = append(refused, fmt.Sprintf("%s is there already, and %s, which only %s used, would be written over it as %s moves: name one of them otherwise first", to, f, name, name))
				continue
			}
			out = append(out, File{Path: f, Gone: true, Was: string(folder[f]), Had: true})
			if !touched[to] {
				out = append(out, File{Path: to, Text: string(folder[f])})
			}
			taken = append(taken, fmt.Sprintf("%s goes with %s, to %s", f, name, to))
		}
	}
	slices.SortFunc(out, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return out, taken, refused
}

// mark gives what was checked: every file of the folder it was read with,
// and the change's files, each whole and with what each was.
func mark(folder map[string][]byte, files []File) string {
	h := sha256.New()
	field := func(s string) { fmt.Fprintf(h, "%d:%s", len(s), s) }
	for _, name := range sortedKeys(folder) {
		field(name)
		field(string(folder[name]))
	}
	field("the change")
	for _, f := range files {
		field(f.Path)
		field(f.Text)
		field(strconv.FormatBool(f.Gone))
		field(f.Was)
		field(strconv.FormatBool(f.Had))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// problemKey is what a problem is, wherever in its file it is found: a
// change above a problem moves its line, and it is the same problem.
func problemKey(p office.Problem) string { return p.File + "\x00" + p.Place + "\x00" + p.Text }

// once gives each problem once, even where two parts of a reading found
// it.
func once(ps office.Problems) office.Problems {
	var out office.Problems
	seen := map[string]bool{}
	for _, p := range ps {
		if key := problemKey(p); !seen[key] {
			seen[key] = true
			out = append(out, p)
		}
	}
	return out
}

func paths(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}

// guard says what in a worker's draft is a person's to write: the people
// of the office and how each is reached; who decides a change to the
// office, the files of the rules and workers on the top team's change and
// everything lists and the models those workers take; a bridge's in
// machine and the files it uses; and which bridges the office has, with
// every setting of each but its places.
func guard(was, now *office.Office, folder, laid map[string][]byte, files []File) []string {
	var out []string
	touched := map[string]bool{}
	for _, f := range files {
		touched[f.Path] = true
	}
	for _, name := range union(was.Everyone, now.Everyone) {
		a, b := was.People[name], now.People[name]
		switch {
		case a == nil:
			out = append(out, fmt.Sprintf("it adds %s to the office: the people of the office are a person's to write", name))
		case b == nil:
			out = append(out, fmt.Sprintf("it takes %s from the office: the people of the office are a person's to write", name))
		case !slices.Equal(sortedIDs(a), sortedIDs(b)):
			out = append(out, fmt.Sprintf("it changes how %s is reached: who a person is on each bridge is a person's to write, since a yes is taken from it", name))
		}
	}
	written := func(o *office.Office, key string) string {
		ds, ok := o.List(office.Top, key)
		if !ok {
			return "\x00none"
		}
		var names []string
		for _, d := range ds {
			names = append(names, d.Written)
		}
		return strings.Join(names, ",")
	}
	for _, key := range []string{"change", "everything"} {
		if written(was, key) != written(now, key) {
			out = append(out, fmt.Sprintf("it changes the office's %s list: who decides a change to the office is a person's to write", key))
		}
	}
	_, hadEverything := was.List(office.Top, "everything")
	_, hasEverything := now.List(office.Top, "everything")
	if (!hadEverything || !hasEverything) && was.TopTeam() != nil && now.TopTeam() != nil && !slices.Equal(was.TopTeam().People, now.TopTeam().People) {
		out = append(out, "it changes the top team's people, who decide what its everything list does not name: who decides a change to the office is a person's to write")
	}
	// The deciders on the change list and on the everything list, each
	// once.
	change, _ := was.List(office.Top, "change")
	var deciders []office.Decider
	for _, d := range append(slices.Clone(change), was.Everything(office.Top)...) {
		if !slices.ContainsFunc(deciders, func(x office.Decider) bool { return x.Kind == d.Kind && x.Name == d.Name }) {
			deciders = append(deciders, d)
		}
	}
	for _, d := range deciders {
		var theirs []string
		switch d.Kind {
		case office.IsRule:
			if r := was.Rules[d.Name]; r != nil {
				theirs = uses(d.Name, r.Draft)
			} else {
				theirs = []string{d.Name}
			}
		case office.IsWorker:
			w := was.Workers[d.Name]
			if w == nil {
				continue
			}
			theirs = append([]string{w.File}, w.Uses...)
			if b := now.Workers[d.Name]; b != nil && b.Model != w.Model {
				out = append(out, fmt.Sprintf("it changes the model %s takes, and %s decides changes to the office: who decides a change to the office is a person's to write", d.Name, d.Name))
			}
		}
		for _, f := range slices.Compact(slices.Sorted(slices.Values(theirs))) {
			if touched[f] {
				out = append(out, fmt.Sprintf("it changes %s, which %s, a decider of changes to the office, is or uses: who decides a change to the office is a person's to write", f, d.Written))
			}
		}
	}
	inMachines := map[string]bool{}
	for _, name := range union(was.BridgeOrder, now.BridgeOrder) {
		dir := "bridges/" + name + "/"
		file := dir + "in.yaml"
		var used []string
		for _, b := range []*office.Bridge{was.Bridges[name], now.Bridges[name]} {
			switch {
			case b != nil && b.From != "":
				// What Bulldog ships no draft touches.
				continue
			case b != nil && b.In != nil:
				used = append(used, uses(file, b.In)...)
				continue
			}
			// What a machine Corbie did not read uses is not known: every
			// file of its folder but the out machine may be its.
			for _, m := range []map[string][]byte{folder, laid} {
				for f := range m {
					if strings.HasPrefix(f, dir) && f != dir+"out.yaml" {
						used = append(used, f)
					}
				}
			}
		}
		used = append(used, file)
		inMachines[file] = true
		for _, f := range slices.Compact(slices.Sorted(slices.Values(used))) {
			if touched[f] {
				out = append(out, fmt.Sprintf("it changes %s, which %s in machine is or uses: an in machine says who a person is, and so is a person's to write", f, of(name)))
			}
		}
	}
	for _, name := range union(was.BridgeOrder, now.BridgeOrder) {
		const why = "a bridge's entry, its from and its settings but places are a person's to write"
		a, b := bridgeEntry(was, name), bridgeEntry(now, name)
		switch {
		case a == nil && b == nil:
		case a == nil:
			out = append(out, fmt.Sprintf("it adds the bridge %s: %s", name, why))
		case b == nil:
			out = append(out, fmt.Sprintf("it takes the bridge %s away: %s", name, why))
		default:
			var changed []string
			for key := range a {
				if key != "places" && !reflect.DeepEqual(a[key], b[key]) {
					changed = append(changed, key)
				}
			}
			for key := range b {
				if _, ok := a[key]; !ok && key != "places" {
					changed = append(changed, key)
				}
			}
			slices.Sort(changed)
			if len(changed) > 0 {
				out = append(out, fmt.Sprintf("it changes %s %s: %s", of(name), strings.Join(changed, ", "), why))
			}
		}
	}
	// A file newly under bridges/ whose bridge is not read as one still may
	// be an in machine.
	for _, f := range files {
		if rest, ok := strings.CutPrefix(f.Path, "bridges/"); ok && path.Base(rest) == "in.yaml" && strings.Count(rest, "/") == 1 && !inMachines[f.Path] {
			out = append(out, fmt.Sprintf("it changes %s, a bridge's in machine: an in machine says who a person is, and so is a person's to write", f.Path))
		}
	}
	return out
}

// bridgeEntry gives a bridge's settings as office.yaml writes them, or nil
// where office.yaml names no such bridge.
func bridgeEntry(o *office.Office, name string) map[string]any {
	b := o.Bridges[name]
	if b == nil {
		return nil
	}
	settings, _ := b.Settings.(map[string]any)
	return settings
}

func sortedIDs(p *office.Person) []string {
	var out []string
	for _, id := range p.Reached {
		out = append(out, id.Bridge+"\x00"+id.ID)
	}
	slices.Sort(out)
	return out
}

// unfit says why a change cannot be put, on one line.
func unfit(c Check) string {
	return strings.Join(append(lines(c.Problems), c.Refused...), "; ")
}
