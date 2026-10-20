package server

import (
	"cmp"
	"net/http"
	"path"
	"slices"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/shipped"
)

// What the office's teams define, in one view.
type (
	definedOut struct {
		Tools  []definedRow `json:"tools"`
		Models []definedRow `json:"models"`
		Docs   []definedRow `json:"docs"`
	}
	// definedRow is one thing the teams' files define, with the teams that
	// use it: a tool's file, a script or another machine, or a tools file
	// Bulldog ships, by its path in .bulldog/ and its folder there; a model
	// as it is sent; or a team's doc, by its name, with its size and the
	// teams whose workers read it.
	definedRow struct {
		Name    string   `json:"name"`
		Kind    string   `json:"kind,omitempty"`
		Shipped string   `json:"shipped,omitempty"`
		Team    string   `json:"team,omitempty"`
		Size    int64    `json:"size,omitempty"`
		Teams   []string `json:"teams"`
	}
)

// defined gives the tools and models the workers of each team the one
// looking sees how it is written use, each with those of its teams that
// use it, and the docs of each team whose docs they may open.
func (s *site) defined(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	decides := slices.Contains(changes.Deciders(o), person)
	out := definedOut{Tools: []definedRow{}, Models: []definedRow{}, Docs: []definedRow{}}
	add := func(rows []definedRow, row definedRow, team string) []definedRow {
		i := slices.IndexFunc(rows, func(r definedRow) bool { return r.Name == row.Name })
		if i < 0 {
			row.Teams = []string{}
			i, rows = len(rows), append(rows, row)
		}
		if !slices.Contains(rows[i].Teams, team) {
			rows[i].Teams = append(rows[i].Teams, team)
		}
		return rows
	}
	for _, name := range o.Order {
		people, err := s.Requests.People(name)
		if err != nil {
			return nil, err
		}
		if !decides && !slices.Contains(people, person) {
			continue
		}
		for _, worker := range o.Teams[name].Workers {
			w := o.Workers[worker]
			if w.Model != "" {
				out.Models = add(out.Models, definedRow{Name: w.Model}, name)
			}
			for _, tool := range w.Held {
				switch {
				case tool.Kind == office.ToolBulldog:
					if file := toolsFileOf(tool.Name); file != "" {
						out.Tools = add(out.Tools, definedRow{Name: path.Join(shipped.Dir, file), Kind: "tools", Shipped: path.Dir(file)}, name)
					}
				case (tool.Kind == office.ToolScript || tool.Kind == office.ToolMachine) && tool.File != "":
					out.Tools = add(out.Tools, definedRow{Name: tool.File, Kind: tool.Kind}, name)
				}
			}
		}
	}
	for _, name := range o.Order {
		opens, err := s.opensDocs(o, person, name)
		if err != nil {
			return nil, err
		}
		if !opens {
			continue
		}
		docs, err := o.Docs(name)
		if err != nil {
			return nil, err
		}
		readers := readersOf(o, name)
		for _, d := range docs {
			out.Docs = append(out.Docs, definedRow{Name: d.Name, Team: name, Size: d.Size, Teams: readers})
		}
	}
	byName := func(a, b definedRow) int { return cmp.Compare(a.Name, b.Name) }
	slices.SortFunc(out.Tools, byName)
	slices.SortFunc(out.Models, byName)
	return out, nil
}

// toolsFileOf gives the tools file Bulldog ships that holds a tool of a
// name, or "".
func toolsFileOf(tool string) string {
	for _, file := range shipped.ToolsFiles {
		if slices.Contains(shipped.Tools(file), tool) {
			return file
		}
	}
	return ""
}

// readersOf gives the teams whose workers read a team's docs: it and each
// team within it that has a worker holding the docs tools.
func readersOf(o *office.Office, team string) []string {
	docs := shipped.Tools(shipped.Docs)
	out := []string{}
	for _, name := range o.Order {
		within := false
		for u := o.Teams[name]; u != nil && !within; u = o.Teams[u.Within] {
			within = u.Name == team
		}
		if !within {
			continue
		}
		for _, worker := range o.Teams[name].Workers {
			if slices.ContainsFunc(o.Workers[worker].Held, func(t office.Tool) bool {
				return t.Kind == office.ToolBulldog && slices.Contains(docs, t.Name)
			}) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}
