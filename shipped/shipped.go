// Package shipped holds the definitions Bulldog ships, which an office's
// workers name by their path in .bulldog/: the ask, the talk tools, the
// notes tool and the builder's tools, with the call each tool runs; and
// those Bulldog starts itself, a
// routine's clock and the server's own run. Each tool is a run that only
// waits on a ticket, which Bulldog answers.
package shipped

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.yaml.in/yaml/v3"
)

// Dir is the folder at the top of an office that holds Bulldog's own.
const Dir = ".bulldog"

// The files Bulldog ships, by their names in Dir.
const (
	Ask   = "ask.yaml"
	Talk  = "talk.yaml"
	Notes = "notes.yaml"
	Build = "build.yaml"
	Live  = "live.yaml"
	Clock = "clock.yaml"
	Call  = "call.yaml"
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

// The files are read from here, by the office's loader, and written into
// an office's .bulldog/ for people to read; they are the same.
//
//go:embed *.yaml
var files embed.FS

// Names lists the files Bulldog ships.
func Names() []string { return []string{Ask, Talk, Notes, Build, Call, Live, Clock} }

// File gives a file Bulldog ships, by its name in Dir.
func File(name string) ([]byte, bool) {
	if !slices.Contains(Names(), name) {
		return nil, false
	}
	contents, err := files.ReadFile(name)
	return contents, err == nil
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

// Write writes every file Bulldog ships into Dir in an office's folder,
// whole, in place of what is there.
func Write(office string) error {
	dir := filepath.Join(office, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range Names() {
		contents, _ := File(name)
		file := filepath.Join(dir, name)
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
