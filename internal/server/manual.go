package server

import (
	"net/http"
	"slices"

	"github.com/zummed/corbie/manual"

	"github.com/zummed/corbie-bulldog/internal/changes"
)

// pageOut is a page of Bulldog's manual: its name, what it is in a line,
// and its text when it is the one asked for.
type pageOut struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Text    string `json:"text,omitempty"`
}

// manualPages gives Bulldog's pages of the manual, by name and summary,
// to everyone signed in.
func (s *site) manualPages(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	out := []pageOut{}
	for _, p := range changes.Pages() {
		out = append(out, pageOut{Name: p.Name, Summary: p.Summary})
	}
	return map[string]any{"pages": out}, nil
}

// manualPage gives one of Bulldog's pages of the manual whole, by its
// name; any other name is nothing there.
func (s *site) manualPage(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	pages := changes.Pages()
	at := slices.IndexFunc(pages, func(p manual.Page) bool { return p.Name == r.PathValue("page") })
	if at < 0 {
		return nil, errNotThere
	}
	p := pages[at]
	return pageOut{Name: p.Name, Summary: p.Summary, Text: p.Text}, nil
}
