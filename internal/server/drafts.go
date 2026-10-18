package server

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	shippedbridges "github.com/zummed/corbie-bulldog/shipped/bridges"
)

// decider gives the office for one of the people who decide a change; to
// anyone else, what only they see is not there.
func (s *site) decider(person string) (*office.Office, error) {
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(changes.Deciders(o), person) {
		return nil, errNotThere
	}
	return o, nil
}

// A file of the office, as the people who decide a change see it.
type fileOf struct {
	Path   string `json:"path"`
	ByHand bool   `json:"by_hand,omitempty"`
}

// files gives every file of the office as its folder holds it now, notes
// among them, and which are edited by hand and not yet committed.
func (s *site) files(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.decider(person)
	if err != nil {
		return nil, err
	}
	folder, _, err := office.Folder(o.Dir)
	if err != nil {
		return nil, err
	}
	byHand, err := s.Changes.ByHand()
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for path := range folder {
		paths = append(paths, path)
	}
	for _, path := range byHand {
		if folder[path] == nil {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	out := []fileOf{}
	for _, path := range paths {
		out = append(out, fileOf{Path: path, ByHand: slices.Contains(byHand, path)})
	}
	return map[string]any{"files": out}, nil
}

// file gives one file of the office, as its folder holds it now.
func (s *site) file(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.decider(person)
	if err != nil {
		return nil, err
	}
	path := r.URL.Query().Get("path")
	folder, _, err := office.Folder(o.Dir)
	if err != nil {
		return nil, err
	}
	data, ok := folder[path]
	if !ok {
		return nil, errNotThere
	}
	return map[string]string{"path": path, "text": string(data)}, nil
}

// shippedBridges gives the bridges Bulldog ships, which a change may add.
func (s *site) shippedBridges(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	o, err := s.decider(person)
	if err != nil {
		return nil, err
	}
	type shipped struct {
		Name     string `json:"name"`
		Carries  string `json:"carries"`
		Settings string `json:"settings"`
		Added    bool   `json:"added"`
	}
	out := []shipped{}
	for _, b := range shippedbridges.All {
		_, added := o.Bridges[b.Name]
		out = append(out, shipped{Name: b.Name, Carries: b.Carries, Settings: b.Settings, Added: added})
	}
	return map[string]any{"bridges": out}, nil
}

// history gives every change to the office's files, newest first.
func (s *site) history(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.decider(person); err != nil {
		return nil, err
	}
	commits, err := s.Changes.History(person)
	if err != nil {
		return nil, err
	}
	return map[string]any{"history": orEmpty(commits)}, nil
}

// commit gives one change of the history, with each file it touched,
// before and after.
func (s *site) commit(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.decider(person); err != nil {
		return nil, err
	}
	c, versions, err := s.Changes.Commit(person, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"commit": c, "files": orEmpty(versions)}, nil
}

// A draft is a person's change as the screens hold it, and the why and
// the mark of its check when it is put.
type draft struct {
	changes.Edit
	Why  *string `json:"why"`
	Mark *string `json:"mark"`
}

// The draft's check, as the person is shown it.
type checkOut struct {
	changes.Check
	// Problems and Standing are the check's, as the screens show every
	// problem.
	Problems []problemOut `json:"problems"`
	Standing []problemOut `json:"standing"`
	// Told is anything of how the files were made that they are to know,
	// and Fit whether it may be put.
	Told []string `json:"told"`
	Fit  bool     `json:"fit"`
}

// edited reads a person's draft, to be checked or put, and makes its
// files, for one of the people who decide a change; anyone else is told
// to ask for one.
func (s *site) edited(person string, w http.ResponseWriter, r *http.Request, put bool) (draft, []changes.File, []string, error) {
	var d draft
	if err := readJSON(w, r, &d); err != nil {
		return d, nil, nil, err
	}
	if !put && (d.Why != nil || d.Mark != nil) {
		return d, nil, nil, unreadable("A check takes no why and no mark: they are given when it is put.")
	}
	o, err := s.office()
	if err != nil {
		return d, nil, nil, err
	}
	switch {
	case !slices.Contains(changes.Deciders(o), person):
		return d, nil, nil, refusal("You do not decide changes to the office, so you may not draft one here: ask for it in a request, and a builder drafts it.")
	case put && (d.Why == nil || strings.TrimSpace(*d.Why) == ""):
		return d, nil, nil, unreadable("The body gives no why: say why the change is made.")
	case put && d.Mark == nil:
		return d, nil, nil, unreadable("The body gives no mark: give the mark of the check you were shown.")
	}
	files, told, err := s.Changes.Edited(d.Edit)
	return d, files, told, err
}

// check reads what a person's draft would do, and writes nothing.
func (s *site) check(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	_, files, told, err := s.edited(person, w, r, false)
	if err != nil {
		return nil, err
	}
	c, err := s.Changes.Check(r.Context(), files, changes.Drafter{Name: person, Person: true})
	if err != nil {
		return nil, err
	}
	c.Files = orEmpty(c.Files)
	c.Refused, c.Says, c.LetsMoreBy = orEmpty(c.Refused), orEmpty(c.Says), orEmpty(c.LetsMoreBy)
	return checkOut{Check: c, Problems: problemsOf(c.Problems), Standing: problemsOf(c.Standing), Told: orEmpty(told), Fit: c.Fit()}, nil
}

// put puts a person's draft to be decided, as it was when they were shown
// its check, which its mark names, and gives what became of it.
func (s *site) put(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	d, files, _, err := s.edited(person, w, r, true)
	if err != nil {
		return nil, err
	}
	p, writing := s.Changes.PutOwn(r.Context(), person, *d.Why, files, *d.Mark)
	if p.ID == 0 {
		return nil, writing
	}
	_, out, err := s.Changes.Put(r.Context(), p.ID)
	if err != nil {
		return nil, err
	}
	became := map[string]any{"put": p.ID, "ended": out.Ended, "said": "It is put to be decided, and goes first to every rule and worker its walk asks."}
	if out.Ended != "" {
		became["said"] = out.Text()
	}
	if writing != nil {
		return nil, fmt.Errorf("the change was put to be decided, as change %d, and writing it failed: %w", p.ID, writing)
	}
	return became, nil
}
