// Package shipped holds what Bulldog ships, as one tree, which is
// .bulldog/ in every office: its own machines, flat at the top, the ask, a
// tool's call, a routine's clock and the server's own run; and one folder
// for each thing an office uses by its path there, a tools file, a bridge
// or a rule, with the thing's page as its README.md. Each tool is a run that
// only waits on a ticket, which Bulldog answers.
package shipped

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zummed/corbie/manual"
	"go.yaml.in/yaml/v3"
)

// Dir is the folder at the top of an office that holds Bulldog's own.
const Dir = ".bulldog"

// The files Bulldog ships, by their paths in Dir.
const (
	Ask   = "ask.yaml"
	Call  = "call.yaml"
	Live  = "live.yaml"
	Clock = "clock.yaml"
	Talk  = "tools/talk/talk.yaml"
	Notes = "tools/notes/notes.yaml"
	Build = "tools/build/build.yaml"

	Mentioned = "rules/mentioned/mentioned.js"
	ForUs     = "rules/for-us/for-us.yaml"
)

// Page is the file of a shipped folder that is its page of the manual, its
// first line saying what the thing is; Settings is a bridge's entry under
// bridges in office.yaml, as it is shipped.
const (
	Page     = "README.md"
	Settings = "settings.yaml"
)

// The tickets the shipped machines wait on, and the event the server's own
// run takes. A shipped tool's run is given its tool's name as the input
// tool, and what the model gave it as args.
const (
	AskTicket   = "decided"
	CallTicket  = "answer"
	ClockTicket = "came"
	Renew       = "renew"
)

// The tree is read from here, by the office's loader, and written into an
// office's .bulldog/ for people to read; they are the same.
//
//go:embed ask.yaml call.yaml live.yaml clock.yaml tools bridges/*/* rules
var tree embed.FS

// kinds are the folders of the tree that hold one folder for each thing.
var kinds = []string{"tools", "bridges", "rules"}

// old are the files earlier versions shipped flat at the top, which are
// taken away.
var old = []string{"talk.yaml", "notes.yaml", "build.yaml"}

// Names lists every file Bulldog ships, by its path in Dir.
func Names() []string {
	var names []string
	err := fs.WalkDir(tree, ".", func(name string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, name)
		}
		return err
	})
	if err != nil {
		panic(err)
	}
	return names
}

// File gives a file Bulldog ships, by its path in Dir.
func File(name string) ([]byte, bool) {
	if !fs.ValidPath(name) {
		return nil, false
	}
	contents, err := tree.ReadFile(name)
	return contents, err == nil
}

// Things lists the folders of the things Bulldog ships, by their paths in
// Dir, such as tools/talk, bridges/slack and rules/for-us.
func Things() []string {
	var things []string
	for _, kind := range kinds {
		entries, err := tree.ReadDir(kind)
		if err != nil {
			panic(err)
		}
		for _, e := range entries {
			if e.IsDir() {
				things = append(things, path.Join(kind, e.Name()))
			}
		}
	}
	return things
}

// Summary is the first line of a shipped thing's page, which says what it
// is.
func Summary(thing string) string {
	page, _ := File(path.Join(thing, Page))
	line, _, _ := strings.Cut(string(page), "\n")
	return strings.TrimSpace(line)
}

// Pages gives each shipped thing's page as a page of the manual, named by
// the thing's path.
func Pages() []manual.Page {
	var pages []manual.Page
	for _, thing := range Things() {
		page, _ := File(path.Join(thing, Page))
		pages = append(pages, manual.Page{Name: thing, Group: "Bulldog", Summary: Summary(thing), Text: string(page)})
	}
	return pages
}

// Tools gives the names of the tools a tools file Bulldog ships holds, in
// the order they sort, or none for a file that is not a tools file.
func Tools(name string) []string {
	if name != Talk && name != Notes && name != Build {
		return nil
	}
	contents, _ := File(name)
	var tools map[string]any
	if err := yaml.Unmarshal(contents, &tools); err != nil {
		panic(fmt.Sprintf("the shipped %s cannot be read: %v", name, err))
	}
	var names []string
	for tool := range tools {
		names = append(names, tool)
	}
	slices.Sort(names)
	return names
}

// Bridges lists the names of the bridges Bulldog ships.
func Bridges() []string {
	var names []string
	for _, thing := range Things() {
		if name, ok := strings.CutPrefix(thing, "bridges/"); ok {
			names = append(names, name)
		}
	}
	return names
}

// Bridge gives a bridge Bulldog ships, by its name: the files of its
// folder, and its entry under bridges in office.yaml as it is shipped.
func Bridge(name string) (fs.FS, string, bool) {
	if !slices.Contains(Bridges(), name) {
		return nil, "", false
	}
	folder, err := fs.Sub(tree, path.Join("bridges", name))
	if err != nil {
		panic(err)
	}
	settings, _ := fs.ReadFile(folder, Settings)
	return folder, string(settings), true
}

// Write writes every file Bulldog ships into Dir in an office's folder,
// whole, in place of what is there. The folders of the things it ships
// are written afresh, so no file an earlier version shipped stays in them;
// nothing else in Dir is touched but the files earlier versions shipped
// flat.
func Write(office string) error {
	dir := filepath.Join(office, Dir)
	for _, kind := range kinds {
		if err := os.RemoveAll(filepath.Join(dir, kind)); err != nil {
			return err
		}
	}
	for _, name := range old {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, name := range Names() {
		contents, _ := File(name)
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		temp := file + ".new"
		if err := os.WriteFile(temp, contents, 0o644); err != nil {
			return err
		}
		if err := os.Rename(temp, file); err != nil {
			os.Remove(temp)
			return err
		}
	}
	return nil
}
