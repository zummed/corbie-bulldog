package server

import (
	"bytes"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// What Bulldog ships, as the one looking may see it.
type (
	shippedOut struct {
		Version string     `json:"version"`
		Things  []thingOut `json:"things"`
	}
	// thingOut is one thing Bulldog ships: a bridge, a tools file or a
	// rule, by its folder in .bulldog/, with its page, what it reaches as
	// shipped, its settings, whether the office uses it and where. The
	// people who decide a change also see whether the office holds a copy
	// of it, what changed in it since the version that ran before, and the
	// drafts that use and copy it.
	thingOut struct {
		Path     string            `json:"path"`
		Kind     string            `json:"kind"`
		Name     string            `json:"name"`
		Summary  string            `json:"summary"`
		Page     string            `json:"page"`
		Needs    reachOut          `json:"needs"`
		Settings []shipped.Setting `json:"settings"`
		Status   string            `json:"status"`
		Uses     []useOut          `json:"uses"`
		Changed  []shipped.Change  `json:"changed,omitempty"`
		Copies   []copyOut         `json:"copies,omitempty"`
		Use      *changes.Edit     `json:"use,omitempty"`
		Copy     *changes.Edit     `json:"copy,omitempty"`
	}
	// useOut is where the office names a shipped thing: a bridge's entry,
	// with what it reaches as its settings fill it to the people who
	// decide a change; a worker of a team that holds its tools; or a
	// team's heed or list of deciders, as its key.
	useOut struct {
		Bridge string    `json:"bridge,omitempty"`
		Team   string    `json:"team,omitempty"`
		Worker string    `json:"worker,omitempty"`
		As     string    `json:"as,omitempty"`
		Reach  *reachOut `json:"reach,omitempty"`
	}
	// copyOut is a bridge's folder of the office's own that is a copy of
	// one Bulldog ships: the version its page last names, and the files
	// that are not as Bulldog ships them now.
	copyOut struct {
		Folder  string   `json:"folder"`
		Version string   `json:"version,omitempty"`
		Differs []string `json:"differs"`
	}
)

// A use is one place the office names a shipped thing.
type use struct {
	bridge *office.Bridge
	team   *office.Team
	worker string
	as     string
}

// shipped gives each thing Bulldog ships as the one looking may see it:
// to everyone its page, what it reaches and its settings as shipped,
// whether it is used, and each bridge that runs it; a tools file's and
// a rule's uses to those who see how the team using it is written; and to
// the people who decide a change, its copies, what changed since the
// version that ran before, what each bridge reaches as its settings fill
// it, and the drafts that use and copy it.
func (s *site) shipped(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	decides := slices.Contains(changes.Deciders(o), person)
	sees := map[string]bool{}
	for _, name := range o.Order {
		people, err := s.Requests.People(name)
		if err != nil {
			return nil, err
		}
		sees[name] = decides || slices.Contains(people, person)
	}
	out := shippedOut{Version: s.version, Things: []thingOut{}}
	for _, thing := range shipped.Things() {
		page, _ := shipped.File(path.Join(thing, shipped.Page))
		t := thingOut{Path: thing, Kind: shipped.Kind(thing), Name: path.Base(thing), Summary: shipped.Summary(thing), Page: string(page),
			Needs: reachOf(shipped.Reach(thing)), Settings: orEmpty(shipped.SettingsOf(thing)), Status: "unused", Uses: []useOut{}}
		uses := usesOf(o, thing)
		if len(uses) > 0 {
			t.Status = "used"
		}
		for _, u := range uses {
			switch {
			case u.bridge != nil:
				seen := useOut{Bridge: u.bridge.Name}
				if decides {
					reach := reachOf(fillAll(shipped.Reach(thing), u.bridge.Settings))
					seen.Reach = &reach
				}
				t.Uses = append(t.Uses, seen)
			case sees[u.team.Name]:
				t.Uses = append(t.Uses, useOut{Team: u.team.Name, Worker: u.worker, As: u.as})
			}
		}
		if decides {
			for _, c := range shipped.Changed(thing) {
				if shipped.Newer(c.Version, s.before) {
					t.Changed = append(t.Changed, c)
				}
			}
			t.Copies = copiesOf(o, thing)
			if t.Status == "unused" && len(t.Copies) > 0 {
				t.Status = "copied"
			}
			t.Use, t.Copy = useDraft(o, thing), copyDraft(o, thing, uses)
		}
		out.Things = append(out.Things, t)
	}
	return out, nil
}

// usesOf gives where the office names a shipped thing: each bridge whose
// entry runs its folder; each worker that holds a tool of a tools file,
// by the tools Bulldog answers for it; each team whose heed, disclosure
// or list of deciders names a rule in its folder.
func usesOf(o *office.Office, thing string) []use {
	if o == nil {
		return nil
	}
	folder := path.Join(shipped.Dir, thing)
	var out []use
	switch shipped.Kind(thing) {
	case "bridge":
		for _, name := range o.BridgeOrder {
			if b := o.Bridges[name]; b.From == folder {
				out = append(out, use{bridge: b})
			}
		}
	case "tools":
		var tools []string
		for _, file := range shipped.ToolsFiles {
			if path.Dir(file) == thing {
				tools = shipped.Tools(file)
			}
		}
		for _, team := range o.Order {
			t := o.Teams[team]
			for _, name := range t.Workers {
				if slices.ContainsFunc(o.Workers[name].Held, func(tool office.Tool) bool {
					return tool.Kind == office.ToolBulldog && slices.Contains(tools, tool.Name)
				}) {
					out = append(out, use{team: t, worker: name})
				}
			}
		}
	case "rule":
		names := func(deciders []office.Decider) bool {
			return slices.ContainsFunc(deciders, func(d office.Decider) bool {
				return d.Kind == office.IsRule && path.Dir(d.Name) == folder
			})
		}
		for _, team := range o.Order {
			t := o.Teams[team]
			if t.Heed != nil && path.Dir(t.Heed.Rule) == folder {
				out = append(out, use{team: t, as: "heed"})
			}
			if t.HasDisclosure && names(t.Disclosure) {
				out = append(out, use{team: t, as: "disclosure"})
			}
			for _, l := range t.Decide {
				if names(l.Deciders) {
					out = append(out, use{team: t, as: l.Key})
				}
			}
		}
	}
	return out
}

// changedLines gives, for each thing Bulldog ships that the office uses,
// each line of its page's Changed newer than the version that ran before,
// to be said at the first start of a new version.
func changedLines(o *office.Office, before string) []string {
	var lines []string
	for _, thing := range shipped.Things() {
		if len(usesOf(o, thing)) == 0 {
			continue
		}
		for _, c := range shipped.Changed(thing) {
			if shipped.Newer(c.Version, before) {
				lines = append(lines, fmt.Sprintf("%s changed in %s, which the office uses: %s", thing, c.Version, c.Text))
			}
		}
	}
	return lines
}

// copiesOf gives the bridges' folders of the office's own that are copies
// of a bridge Bulldog ships, each told by its page's first line being the
// shipped bridge's.
func copiesOf(o *office.Office, thing string) []copyOut {
	if shipped.Kind(thing) != "bridge" {
		return nil
	}
	var out []copyOut
	for _, file := range o.Files {
		folder := path.Dir(file)
		if path.Base(file) != shipped.Page || path.Dir(folder) != "bridges" {
			continue
		}
		page, err := os.ReadFile(filepath.Join(o.Dir, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		first, _, _ := strings.Cut(string(page), "\n")
		if strings.TrimSpace(first) != shipped.Summary(thing) {
			continue
		}
		c := copyOut{Folder: folder, Version: shipped.Latest(shipped.ChangedIn(page)), Differs: []string{}}
		for _, name := range shipped.Files(thing) {
			if name == shipped.Settings {
				continue
			}
			ships, _ := shipped.File(path.Join(thing, name))
			own, err := os.ReadFile(filepath.Join(o.Dir, filepath.FromSlash(folder), name))
			if err != nil || !bytes.Equal(own, ships) {
				c.Differs = append(c.Differs, name)
			}
		}
		out = append(out, c)
	}
	return out
}

// useDraft gives the draft that uses a bridge Bulldog ships: its entry as
// shipped, under its own name in office.yaml, or, when that is taken, the
// first name after it that is free.
func useDraft(o *office.Office, thing string) *changes.Edit {
	if shipped.Kind(thing) != "bridge" {
		return nil
	}
	return &changes.Edit{Set: []changes.Setting{{Path: office.OfficeFile, Field: []string{"bridges", freeBridge(o, path.Base(thing))}, Value: shippedEntry(thing)}}}
}

// copyDraft gives the draft that copies a thing Bulldog ships into the
// office: a bridge's machines, script and page into its own folder, for
// each entry that runs it, or one new entry when none does, each entry
// without its from; a rule's files into the folder of each team that
// names it, and each line that names it naming the copy. A tools file is
// not copied, and neither is a rule no team names.
func copyDraft(o *office.Office, thing string, uses []use) *changes.Edit {
	e := &changes.Edit{Files: map[string]*string{}}
	write := func(dir string, leave string) {
		for _, name := range shipped.Files(thing) {
			if name != leave && name != shipped.Settings {
				contents, _ := shipped.File(path.Join(thing, name))
				text := string(contents)
				e.Files[path.Join(dir, name)] = &text
			}
		}
	}
	switch shipped.Kind(thing) {
	case "bridge":
		entries := map[string]any{}
		for _, u := range uses {
			entries[u.bridge.Name] = u.bridge.Settings
		}
		if len(entries) == 0 {
			entries[freeBridge(o, path.Base(thing))] = shippedEntry(thing)
		}
		for _, name := range slices.Sorted(maps.Keys(entries)) {
			write(path.Join("bridges", name), "")
			own := map[string]any{}
			if settings, ok := entries[name].(map[string]any); ok {
				maps.Copy(own, settings)
			}
			delete(own, "from")
			e.Set = append(e.Set, changes.Setting{Path: office.OfficeFile, Field: []string{"bridges", name}, Value: own})
		}
	case "rule":
		if len(uses) == 0 {
			return nil
		}
		copied := map[string]bool{}
		for _, u := range uses {
			t := u.team
			if !copied[t.Name] {
				write(t.Dir, shipped.Page)
				copied[t.Name] = true
			}
			switch u.as {
			case "heed":
				field := []string{"heed"}
				if len(t.Heed.Settings) > 0 {
					field = append(field, "rule")
				}
				e.Set = append(e.Set, changes.Setting{Path: t.File, Field: field, Value: "./" + path.Base(t.Heed.Rule)})
			case "disclosure":
				e.Set = append(e.Set, changes.Setting{Path: t.File, Field: []string{"disclosure"}, Value: renamed(t.Disclosure, thing)})
			default:
				i := slices.IndexFunc(t.Decide, func(l office.List) bool { return l.Key == u.as })
				e.Set = append(e.Set, changes.Setting{Path: t.File, Field: []string{"decide", u.as}, Value: renamed(t.Decide[i].Deciders, thing)})
			}
		}
	default:
		return nil
	}
	return e
}

// renamed gives a list of deciders as written, with each rule in a
// shipped rule's folder named as its copy in the team's own.
func renamed(deciders []office.Decider, thing string) []string {
	out := []string{}
	for _, d := range deciders {
		if d.Kind == office.IsRule && path.Dir(d.Name) == path.Join(shipped.Dir, thing) {
			out = append(out, "./"+path.Base(d.Name))
			continue
		}
		out = append(out, d.Written)
	}
	return out
}

// shippedEntry gives a bridge's entry as Bulldog ships it.
func shippedEntry(thing string) any {
	contents, _ := shipped.File(path.Join(thing, shipped.Settings))
	var v any
	if err := yaml.Unmarshal(contents, &v); err != nil {
		panic(fmt.Sprintf("the shipped %s/%s cannot be read: %v", thing, shipped.Settings, err))
	}
	return v
}

// freeBridge gives a name for a new bridge: the name given, or the first
// after it, as slack-2, that no bridge has and no folder of the office's
// bridges/ holds.
func freeBridge(o *office.Office, name string) string {
	taken := func(n string) bool {
		return o.Bridges[n] != nil || slices.ContainsFunc(o.Files, func(f string) bool { return strings.HasPrefix(f, "bridges/"+n+"/") })
	}
	free := name
	for i := 2; taken(free); i++ {
		free = name + "-" + strconv.Itoa(i)
	}
	return free
}
