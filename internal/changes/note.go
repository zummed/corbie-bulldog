package changes

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// A Note is a note's write: one of a team's notes, its whole new text,
// why, the worker that wrote it and the request it was written in.
type Note struct {
	Team    string
	Name    string
	Text    string
	Why     string
	Worker  string
	Request int64
}

// noteName is a note's plain name: no path, nothing that starts with a
// dot.
var noteName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ErrNoteAsItIs says a note holds the text it was to be written with
// already, so nothing is written or committed.
var ErrNoteAsItIs = errors.New("the note holds that text already, so nothing is written")

// WriteNote writes one of a team's notes, as notes/<name>.md in its
// folder, and commits it at once, by the worker, with its why and the
// request: the one write to the office's files that is not a decided
// change. It is written as a change is, whole beside itself and moved into
// place, never over an edit by hand not yet committed, in a commit of that
// one file. It stands apart from any draft. It gives the commit; a note
// that holds the text already is ErrNoteAsItIs, and a commit with an error
// is a note written and kept, the error being what went wrong after.
func (c *Changes) WriteNote(ctx context.Context, n Note) (string, error) {
	if !noteName.MatchString(n.Name) {
		return "", fmt.Errorf("%q is not a note's name: a note is named by a plain name, such as fuses, with no path in it", n.Name)
	}
	if strings.TrimSpace(n.Why) == "" {
		return "", fmt.Errorf("the note %s says no why: say why it is written, in a line", n.Name)
	}
	c.writing.Lock()
	defer c.writing.Unlock()
	o, err := c.office()
	if err != nil {
		return "", err
	}
	t := o.Teams[n.Team]
	if t == nil {
		return "", fmt.Errorf("the office has no team %s, so its note %s cannot be written", n.Team, n.Name)
	}
	file := path.Join(t.Dir, office.NotesDir, n.Name+".md")
	repo, err := openRepo(c.dir)
	if err != nil {
		return "", err
	}
	edited, err := repo.edited([]string{file})
	if err != nil {
		return "", fmt.Errorf("the office's repository cannot be read: %w", err)
	}
	if len(edited) > 0 {
		return "", fmt.Errorf("%s has an edit by hand that is not yet committed, so the note is not written over it: whoever edited it commits it, or undoes it, first", file)
	}
	folder, _, err := c.folder()
	if err != nil {
		return "", err
	}
	was, had := folder[file]
	if had && string(was) == n.Text {
		return "", fmt.Errorf("%s: %w", file, ErrNoteAsItIs)
	}
	f := File{Path: file, Text: n.Text, Was: string(was), Had: had}
	failed, after := writeFiles(c.dir, []File{f})
	if failed != nil {
		return "", errors.Join(failed, after)
	}
	does := fmt.Sprintf("%s note %s is %s.", of(called(o, n.Team)), n.Name, f.Does())
	var notes [][2]string
	if n.Request != 0 {
		notes = append(notes, [2]string{RequestOf, strconv.FormatInt(n.Request, 10)})
	}
	id, err := repo.commit([]string{file}, n.Worker, message(n.Why, []string{does}, nil, notes), c.Now())
	if id == "" {
		if back := putBack(c.dir, []File{f}); back != nil {
			return "", fmt.Errorf("the note could not be committed (%v), and could not be put back as it was: %w", err, back)
		}
		return "", fmt.Errorf("the note could not be committed, so it was put back as it was: %w", err)
	}
	if c.keeper != nil {
		c.keeper.Look()
	}
	return id, errors.Join(after, err)
}
