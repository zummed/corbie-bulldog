package changes

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/manual"
)

// OfficePage is the name of Bulldog's page of the manual, on how an office
// is written.
const OfficePage = "office"

// officePage says how an office is written, with a whole worker, rule,
// heed and bridge.
//
//go:embed office.md
var officePage string

// The office page, and the page of each thing Bulldog ships, are the
// manual's, beside Corbie's, wherever the manual is read: by a worker, by
// corbie bulldog manual and by a model.
func init() {
	manual.Add(func() []manual.Page {
		office := manual.Page{Name: OfficePage, Group: Group, Summary: "how an office is written, with a whole worker, rule, heed and bridge", Text: officePage}
		return append([]manual.Page{office}, shipped.Pages()...)
	})
}

// Group is the group of Bulldog's own pages of the manual.
const Group = "Bulldog"

// Pages gives Bulldog's pages of the manual, in the manual's order: the
// office page, the page of each thing Bulldog ships and the program's.
func Pages() []manual.Page {
	var out []manual.Page
	for _, p := range manual.Pages() {
		if p.Group == Group {
			out = append(out, p)
		}
	}
	return out
}

// Manual gives a page of the manual by its name, as corbie manual prints
// it. No name, or a name that is no page, gives the names there are.
func Manual(page string) string {
	page = strings.TrimSpace(page)
	if p, ok := manual.Find(page); ok && page != "" {
		return strings.TrimRight(p.Text, "\n") + "\n"
	}
	var b strings.Builder
	if page != "" {
		fmt.Fprintf(&b, "%q is not a page of the manual. ", page)
	}
	b.WriteString("The pages are:\n")
	for _, p := range manual.Pages() {
		fmt.Fprintf(&b, "  %-22s %s\n", p.Name, p.Summary)
	}
	return b.String()
}
