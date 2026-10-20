package work

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// The docs tools give a run the docs its team reads: its own team's, then
// those of each team around it, out to the office, the nearer team's doc
// of a name standing before one further out. Each result names its team,
// and nothing but a doc is read.

// listDocs lists the docs the calling worker's team reads, each with its
// team, its size and its first heading.
func (w *Work) listDocs(c *calling) (string, bool, error) {
	docs, err := w.office.DocsFor(c.team.Name)
	if err != nil {
		return "", false, err
	}
	if len(docs) == 0 {
		return fmt.Sprintf("Neither %s nor any team around it has docs.", inSentence(c.team.Called)), false, nil
	}
	var lines []string
	for _, d := range docs {
		text, err := w.office.DocText(d)
		if err != nil {
			return "", false, err
		}
		line := fmt.Sprintf("%s, of %s, %d characters", d.Name, calledTeam(w.office, d.Team), utf8.RuneCountInString(text))
		if h := heading(text); h != "" {
			line += ": " + h
		}
		lines = append(lines, line)
	}
	text, shown := fit(lines, DocsLimit)
	if left := len(lines) - shown; left > 0 {
		text += fmt.Sprintf("%d more docs are not listed; search_docs finds words in all of them.", left)
	}
	return text, false, nil
}

// readDoc gives one doc the calling worker's team reads, by its name,
// from a line on, as many whole lines as fit in DocsLimit characters, and
// how to read on.
func (w *Work) readDoc(c *calling) (string, bool, error) {
	name := strings.TrimSuffix(strings.TrimSpace(str(c.args["name"])), ".md")
	from, _ := c.args["from"].(float64)
	docs, err := w.office.DocsFor(c.team.Name)
	if err != nil {
		return "", false, err
	}
	i := slices.IndexFunc(docs, func(d office.Doc) bool { return d.Name == name })
	if i < 0 {
		return fmt.Sprintf("No doc you may read is called %q, so nothing was read: docs lists those you may.", name), false, nil
	}
	d := docs[i]
	text, err := w.office.DocText(d)
	if err != nil {
		return "", false, err
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	start := max(int(from), 1)
	if start > len(lines) {
		return fmt.Sprintf("%s, of %s, has %d lines, so nothing is from line %d on.", d.Name, calledTeam(w.office, d.Team), len(lines), start), false, nil
	}
	body, shown := fit(lines[start-1:], DocsLimit)
	end := start + shown - 1
	out := fmt.Sprintf("%s, of %s, lines %d to %d of %d:\n%s", d.Name, calledTeam(w.office, d.Team), start, end, len(lines), body)
	if end < len(lines) {
		out += fmt.Sprintf("%d more lines after line %d; read on from line %d.", len(lines)-end, end, end+1)
	}
	return out, false, nil
}

// searchDocs gives the lines of the docs the calling worker's team reads
// that hold the words asked for, in any case, each with its doc, its line
// and its team, as many as fit in DocsLimit characters.
func (w *Work) searchDocs(c *calling) (string, bool, error) {
	words := strings.TrimSpace(str(c.args["words"]))
	if words == "" {
		return "A search needs words, and this one had none, so nothing was searched.", false, nil
	}
	docs, err := w.office.DocsFor(c.team.Name)
	if err != nil {
		return "", false, err
	}
	var lines []string
	for _, d := range docs {
		text, err := w.office.DocText(d)
		if err != nil {
			return "", false, err
		}
		for i, line := range strings.Split(text, "\n") {
			if strings.Contains(strings.ToLower(line), strings.ToLower(words)) {
				lines = append(lines, fmt.Sprintf("%s:%d, of %s: %s", d.Name, i+1, calledTeam(w.office, d.Team), line))
			}
		}
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No line of the docs you may read holds %q.", words), false, nil
	}
	text, shown := fit(lines, DocsLimit)
	if left := len(lines) - shown; left > 0 {
		text += fmt.Sprintf("%d more lines hold it; search for something narrower, or read a doc with read_doc.", left)
	}
	return text, false, nil
}

// heading gives a doc's first heading, without its marks, or nothing.
func heading(text string) string {
	for line := range strings.Lines(text) {
		if h, ok := strings.CutPrefix(line, "#"); ok {
			return strings.TrimSpace(strings.TrimLeft(h, "#"))
		}
	}
	return ""
}
