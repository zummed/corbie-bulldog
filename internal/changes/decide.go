package changes

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// A Put is a change put: its files fixed as they were when it was put,
// and what it says, as whoever decides is shown them.
type Put struct {
	ID int64 `json:"id"`
	// Request is the request it was drafted in; none for a person's change
	// from the screens.
	Request int64 `json:"request,omitzero"`
	// By is the worker or the person who drafted it. Person says it is a
	// person's own change, put by By from the screens.
	By     string `json:"by"`
	Person bool   `json:"person,omitzero"`
	// Tool is the tool that put it, office_put or write_doc; a person's
	// own change is put as office_put. Team is the team whose decision it
	// is, read from its files.
	Tool  string `json:"tool"`
	Team  string `json:"team"`
	Why   string `json:"why"`
	Files []File `json:"files"`
	// Says and LetsMoreBy are what it does, read from the files alone.
	Says       []string  `json:"says"`
	LetsMoreBy []string  `json:"lets_more_by"`
	At         time.Time `json:"at"`
}

// A Decision is how a decider ended a change: a yes, or a refusal, with
// their note. Bulldog, of the kind office.IsBulldog, refuses a change
// that cannot be put or that nobody is left to decide, and never says yes.
type Decision struct {
	By   string
	Kind string
	// Called is what the decider is called, as Bulldog says it to people:
	// a person's name as the office writes it, a rule's path, a worker's
	// name. Where it is not given, By is said.
	Called string
	Yes    bool
	Note   string
	At     time.Time
}

// How a change put ends.
const (
	Written    = "written"
	Refused    = "refused"
	NotWritten = "not written"
	Withdrawn  = "withdrawn"
)

// An Outcome is how a change put ended, as whoever drafted it is told.
type Outcome struct {
	Put     int64  `json:"put"`
	Request int64  `json:"request,omitzero"`
	By      string `json:"by"`
	Ended   string `json:"ended"`
	// Commit is the commit that keeps a change written.
	Commit string `json:"commit,omitzero"`
	// Decider is who decided it, Called what they are called, and Note
	// their note; nobody for a change whose job was not held.
	Decider string `json:"decider,omitzero"`
	Called  string `json:"called,omitzero"`
	Note    string `json:"note,omitzero"`
	// Why says why a change was not written, or was withdrawn.
	Why string `json:"why,omitzero"`
	// Gone are the workers a change written took away, whose runs are to
	// be cancelled.
	Gone []string  `json:"gone,omitzero"`
	At   time.Time `json:"at"`
}

// Text says the outcome as whoever drafted the change is told it.
func (o Outcome) Text() string {
	switch o.Ended {
	case Written:
		if o.Called == "" {
			return fmt.Sprintf("The change was written, and is kept as the commit %s.", o.Commit)
		}
		return fmt.Sprintf("The change was written, with the yes of %s, and is kept as the commit %s.%s", o.Called, o.Commit, noted(o.Note))
	case Refused:
		note := noted(o.Note)
		if o.Decider == bulldog && o.Note != "" {
			note = " Bulldog's note: " + o.Note
		}
		return fmt.Sprintf("The change was refused by %s, and nothing was written.%s The draft is kept, to be changed and put again.", o.Called, note)
	case NotWritten:
		return fmt.Sprintf("The change was not written: %s. The draft is kept, to be changed and put again.", o.Why)
	}
	return fmt.Sprintf("The change was withdrawn: %s.", o.Why)
}

// bulldog is what Bulldog is called as the decider of a change it refused.
const bulldog = "Bulldog"

func noted(note string) string {
	if note == "" {
		return ""
	}
	return " Their note: " + note
}

func isPerson(d office.Decider) bool { return d.Kind == office.IsPerson }

// Deciders gives the people who decide a change: those the top team's
// office_put list names; where it names no person, those its everything
// names; where neither names a person, the top team's people. They alone
// put a change from the screens and read the history.
func Deciders(o *office.Office) []string {
	people := func(ds []office.Decider) []string {
		var out []string
		for _, d := range ds {
			if isPerson(d) {
				out = append(out, d.Name)
			}
		}
		return out
	}
	if list, ok := o.List(office.Top, ToolPut); ok && len(people(list)) > 0 {
		return people(list)
	}
	if list, ok := o.List(office.Top, "everything"); ok && len(people(list)) > 0 {
		return people(list)
	}
	return slices.Clone(o.TopTeam().People)
}

// Holds says whether a tool is one of the writes this area fixes when
// Corbie holds its job, to be decided.
func Holds(tool string) bool { return tool == ToolPut || tool == ToolWriteDoc }

// A Hold is a held office_put or write_doc, as Decisions hands it here:
// the request, the worker whose job it is, the tool and what the model
// gave it.
type Hold struct {
	Request int64
	Worker  string
	Tool    string
	Args    json.RawMessage
}

// Held fixes a held write to be decided: the request's draft, checked, or
// the one doc write_doc writes. It gives the change put, with the team
// whose decision it is; or, put as nothing, why it cannot be, in words
// Bulldog refuses it with. A change put by the same tool from the same
// request that waited is withdrawn, and given, and not told: whoever
// decides withdraws its decision.
func (c *Changes) Held(ctx context.Context, h Hold) (Put, string, *Outcome, error) {
	why, files, said, err := c.toWrite(ctx, Call{Request: h.Request, Worker: h.Worker, Tool: h.Tool, Args: h.Args})
	if err != nil || said != "" {
		return Put{}, said, nil, err
	}
	p, check, withdrawn, err := c.put(ctx, Drafter{Name: h.Worker}, h.Request, h.Tool, why, files, "")
	if err != nil || p.ID == 0 {
		return Put{}, check, withdrawn, err
	}
	return p, "", withdrawn, nil
}

// toWrite reads what a call of office_put or write_doc would write: why,
// and the request's draft or the one doc. What cannot be written is said,
// with nothing to write.
func (c *Changes) toWrite(ctx context.Context, call Call) (string, []File, string, error) {
	var args struct {
		Name string `json:"name"`
		Text string `json:"text"`
		Why  string `json:"why"`
	}
	if len(call.Args) > 0 && string(call.Args) != "null" {
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return "", nil, fmt.Sprintf("%s could not read what it was given: %v", call.Tool, err), nil
		}
	}
	o, err := c.office()
	if err != nil {
		return "", nil, "", err
	}
	w := o.Workers[call.Worker]
	if w == nil {
		return "", nil, fmt.Sprintf("%s is no longer a worker of the office, so what it would write is not written", call.Worker), nil
	}
	if call.Tool == ToolPut {
		files, err := drafted(ctx, c.db, call.Request)
		for _, f := range files {
			if !o.Reaches(w.Team, f.Path) {
				return "", nil, fmt.Sprintf("the draft holds %s, outside what you reach, the files of your own team's folder and of the teams within it, so it is not put", f.Path), nil
			}
		}
		return args.Why, files, "", err
	}
	name := strings.TrimSuffix(strings.TrimSpace(args.Name), ".md")
	if problem := docName(name); problem != "" {
		return "", nil, problem, nil
	}
	file := path.Join(o.Teams[w.Team].Dir, office.DocsDir, name+".md")
	folder, _, err := c.folder()
	if err != nil {
		return "", nil, "", err
	}
	if problem := placeProblem(folder, file); problem != "" {
		return "", nil, problem, nil
	}
	was, had := folder[file]
	if had && string(was) == args.Text {
		return "", nil, fmt.Sprintf("the doc %s holds that text already, so nothing is written", name), nil
	}
	return args.Why, []File{{Path: file, Text: args.Text, Was: string(was), Had: had}}, "", nil
}

// docName says why a name is not a doc's, or nothing when it is: a path
// in docs/, written plainly, with no part that starts with a dot.
func docName(name string) string {
	switch {
	case name == "":
		return "a doc is named by its name as docs lists it, such as rooms/kitchen, and no name was given"
	case path.IsAbs(name) || path.Clean(name) != name || strings.Contains(name, "\\") || strings.HasPrefix(name, "../") || name == "..":
		return fmt.Sprintf("%q is not a doc's name: a doc is named by its path in your team's docs/, written plainly, such as rooms/kitchen", name)
	case hasDot(name):
		return fmt.Sprintf("%q is not a doc's name: nothing whose name starts with a dot is a doc", name)
	}
	return ""
}

// PutOwn puts a person's own change, drafted on the screens, to be
// decided. Only the people who decide a change put one. What is put is
// exactly what they were shown: the mark of their check must be that of
// the change as it is now, or nothing is put and they are told to look
// again; a change put with no mark was shown to nobody, and is not put.
// It is decided as an office_put of theirs would be, and one asked of
// nobody before them is written as it is put, with their yes. It gives
// the change put, or why it was not; an error beside a change put is what
// went wrong in deciding or writing it.
func (c *Changes) PutOwn(ctx context.Context, person, why string, files []File, mark string) (Put, error) {
	o, err := c.office()
	if err != nil {
		return Put{}, err
	}
	if !slices.Contains(Deciders(o), person) {
		return Put{}, refuse("%s does not decide changes to the office, so cannot put one: ask for it in a request, and a builder drafts it", person)
	}
	if c.Decide == nil {
		return Put{}, errNoDecide
	}
	p, said, _, err := c.put(ctx, Drafter{Name: person, Person: true}, 0, ToolPut, why, files, mark)
	if err != nil {
		return p, err
	}
	if p.ID == 0 {
		return Put{}, Refusal(said)
	}
	if err := c.Decide(p); err != nil {
		o := Outcome{Put: p.ID, By: person, Ended: NotWritten, Why: "it could not be put to be decided: " + err.Error(), At: c.Now()}
		if _, kept := endPut(ctx, c.db, o); kept != nil {
			return Put{}, errors.Join(err, kept)
		}
		return Put{}, fmt.Errorf("the change could not be put to be decided: %w", err)
	}
	return p, nil
}

// office reads the office in the folder now. An office whose reading is
// refused, by an edit by hand, is read as far as it goes: a change may
// mend it, so long as someone can be found to decide it.
func (c *Changes) office() (*office.Office, error) {
	folder, _, err := c.folder()
	if err != nil {
		return nil, err
	}
	o, problems := office.Parse(c.dir, folder)
	if problems.Refuse() && (o.TopTeam() == nil || len(Deciders(o)) == 0) {
		return nil, refuse("the office does not read, so nobody can be found to decide a change: %v", problems)
	}
	return o, nil
}

// put checks a change and keeps it as put, in place of any change put by
// the same tool from the same request that waits, which is withdrawn and
// given. A
// change that cannot be put is not, and what it gives says why. What is
// put is the change as checked, with what a worker it moves takes with
// it, and the team whose decision it is. One change is put at a time. A
// person's change is put only with the mark of the check they were shown.
func (c *Changes) put(ctx context.Context, by Drafter, request int64, tool, why string, files []File, mark string) (Put, string, *Outcome, error) {
	c.putting.Lock()
	defer c.putting.Unlock()
	check, err := c.Check(ctx, files, by)
	if err != nil {
		return Put{}, "", nil, err
	}
	if strings.TrimSpace(why) == "" {
		check.Refused = append(check.Refused, "it says no why: say why in words a person can decide on, its first line saying what it is")
	}
	if by.Person && mark != check.Mark {
		check.Refused = append(check.Refused, "the change is not as it was when it was shown: the folder has moved on since. Look at it again")
	}
	if !check.Fit() {
		return Put{}, check.Text(), nil, nil
	}
	o, err := c.office()
	if err != nil {
		return Put{}, "", nil, err
	}
	p := Put{Request: request, By: by.Name, Person: by.Person, Tool: tool, Team: o.Whose(paths(check.Files)), Why: strings.TrimSpace(why),
		Files: check.Files, Says: check.Says, LetsMoreBy: check.LetsMoreBy, At: c.Now()}
	withdrawn, err := c.keepInPlace(ctx, &p)
	if err != nil {
		return Put{}, "", nil, err
	}
	if request != 0 && tool == ToolPut {
		if err := keepTaken(ctx, c.db, request, files, p.Files); err != nil {
			return Put{}, "", withdrawn, fmt.Errorf("the change %d is put, but what it takes with a worker it moves could not be added to its draft: %w", p.ID, err)
		}
	}
	return p, "", withdrawn, nil
}

// keepInPlace keeps a change put, withdrawing the change put by the same
// tool from the same request that waits, and gives how that one ended.
// Both are kept, or neither.
func (c *Changes) keepInPlace(ctx context.Context, p *Put) (*Outcome, error) {
	c.deciding.Lock()
	defer c.deciding.Unlock()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var withdrawn *Outcome
	if p.Request != 0 {
		waiting, err := waitingOf(ctx, tx, p.Request, p.Tool)
		if err != nil {
			return nil, err
		}
		if waiting != 0 {
			o := Outcome{Put: waiting, Request: p.Request, By: p.By, Ended: Withdrawn, Why: "its draft was changed and put again, in its place", At: c.Now()}
			if ended, err := endPut(ctx, tx, o); err != nil {
				return nil, err
			} else if ended {
				withdrawn = &o
			}
		}
	}
	id, err := keepPut(ctx, tx, *p)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	p.ID = id
	return withdrawn, nil
}

// tell tells how a change ended.
func (c *Changes) tell(o Outcome) {
	if c.Told != nil {
		c.Told(o)
	}
}

// Put gives a change put, and how it ended, Ended "" while it waits.
func (c *Changes) Put(ctx context.Context, id int64) (Put, Outcome, error) {
	p, o, _, err := gotPut(ctx, c.db, id)
	return p, o, err
}

// Decided takes how a change put was decided. A refusal ends it, writing
// nothing. A yes on a person's own change writes it; a yes on a held write
// is kept, and the write its job then runs writes it, naming who decided.
// Bulldog's yes is never taken. A decision on a change that has ended
// changes nothing, and says how it ended. The outcome, once it ends, is
// told and given. A change written may come with an error all the same:
// it is written and kept, and the error is what went wrong after.
func (c *Changes) Decided(ctx context.Context, id int64, d Decision) (Outcome, error) {
	o, err := c.decided(ctx, id, d)
	if o.Ended != "" {
		c.tell(o)
	}
	return o, err
}

// decided takes a decision on a change, once: two answers that come
// together are taken one at a time, and the second finds it ended. It
// gives the outcome only when this decision ended it.
func (c *Changes) decided(ctx context.Context, id int64, d Decision) (Outcome, error) {
	c.deciding.Lock()
	defer c.deciding.Unlock()
	p, ended, yes, err := gotPut(ctx, c.db, id)
	switch {
	case err != nil:
		return Outcome{}, err
	case ended.Ended != "":
		return Outcome{}, fmt.Errorf("the change %d has ended already: it was %s", id, ended.Ended)
	case yes != nil:
		// Told again, as after a restart: its yes is kept already.
		return Outcome{}, nil
	case d.Yes && d.Kind == office.IsBulldog:
		return Outcome{}, fmt.Errorf("Bulldog only refuses a change nobody can decide, and its yes on the change %d is not taken", id)
	}
	if d.At.IsZero() {
		d.At = c.Now()
	}
	if d.Yes && !p.Person {
		return Outcome{}, keepYes(ctx, c.db, id, d)
	}
	return c.end(ctx, p, d)
}

// end ends a change with its decision: a refusal writes nothing, and a
// yes, or no decision at all for a write whose job was not held, writes
// it. Anything that stops it being written ends it as not written, with
// why.
func (c *Changes) end(ctx context.Context, p Put, d Decision) (Outcome, error) {
	o := Outcome{Put: p.ID, Request: p.Request, By: p.By, Decider: d.By, Called: cmp.Or(d.Called, d.By), Note: d.Note, At: cmp.Or(d.At, c.Now())}
	if d.Kind == office.IsBulldog {
		o.Decider, o.Called = bulldog, bulldog
	}
	if !d.Yes && d.By != "" {
		o.Ended = Refused
		_, err := endPut(ctx, c.db, o)
		return o, err
	}
	commit, gone, why, err := c.writeChange(ctx, p, d)
	switch {
	case commit != "":
		o.Ended, o.Commit, o.Gone = Written, commit, gone
	case err != nil:
		o.Ended, o.Why = NotWritten, err.Error()
	default:
		o.Ended, o.Why = NotWritten, why
	}
	if _, kept := endPut(ctx, c.db, o); kept != nil {
		return o, errors.Join(err, fmt.Errorf("how the change %d ended could not be kept: %w", p.ID, kept))
	}
	return o, err
}

// written does a call of office_put or write_doc once its job runs: it
// writes the change its yes was kept for, the one held from the same
// request with the same why, and writes nothing for one held with no yes
// kept; or, where none was held, its job waiting for no yes, it puts what
// the call would write and writes it at once. Its answer is the outcome.
func (c *Changes) written(ctx context.Context, call Call) (Answer, error) {
	var args struct {
		Why string `json:"why"`
	}
	_ = json.Unmarshal(call.Args, &args)
	c.deciding.Lock()
	p, yes, err := approved(ctx, c.db, call.Request, call.Tool, strings.TrimSpace(args.Why))
	c.deciding.Unlock()
	if err != nil {
		return Answer{}, err
	}
	if p != nil && yes == nil {
		return Answer{Text: "It was not written: it was held, and no yes is kept for it."}, nil
	}
	d := Decision{}
	if p == nil {
		why, files, said, err := c.toWrite(ctx, call)
		if err != nil || said != "" {
			return Answer{Text: sentence(said)}, err
		}
		put, check, withdrawn, err := c.put(ctx, Drafter{Name: call.Worker}, call.Request, call.Tool, why, files, "")
		if withdrawn != nil {
			c.tell(*withdrawn)
		}
		if err != nil {
			return Answer{}, err
		}
		if put.ID == 0 {
			return Answer{Text: "It was not written. " + check}, nil
		}
		p = &put
	} else {
		d = *yes
	}
	c.deciding.Lock()
	o, err := c.end(ctx, *p, d)
	c.deciding.Unlock()
	if o.Ended != "" {
		c.tell(o)
	}
	return Answer{Text: o.Text()}, err
}

// sentence gives words as a sentence, begun with a capital and ended with
// a full stop.
func sentence(s string) string {
	if s == "" {
		return s
	}
	s = strings.ToUpper(s[:1]) + s[1:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// writeChange writes a change on a yes and commits it, if it may still be:
// read again with the folder as it is now, it must leave no problem the
// office has not, every file must be as it was drafted against, and none
// may have an edit by hand. It gives the commit and the workers it took
// away, or why it was not written; an error is Bulldog's own. Once there is
// a commit the change is written, and an error beside it is what went wrong
// after.
func (c *Changes) writeChange(ctx context.Context, p Put, d Decision) (commit string, gone []string, why string, err error) {
	c.writing.Lock()
	defer c.writing.Unlock()
	folder, left, err := c.folder()
	if err != nil {
		return "", nil, "", err
	}
	check, err := c.check(folder, left, p.Files, Drafter{Name: p.By, Person: p.Person})
	if err != nil {
		return "", nil, "", err
	}
	if !check.Fit() {
		return "", nil, unfit(check), nil
	}
	repo, err := openRepo(c.dir)
	if err != nil {
		return "", nil, err.Error(), nil
	}
	failed, after := writeFiles(c.dir, p.Files)
	switch {
	case failed != nil && after != nil:
		return "", nil, "", fmt.Errorf("%v, and its files could not all be put back as they were, so the office now holds what nobody decided: %w", failed, after)
	case failed != nil:
		return "", nil, failed.Error(), nil
	}
	notes := [][2]string{{DecidedBy, d.By}, {NoteKey, d.Note}}
	if p.Request != 0 {
		notes = append(notes, [2]string{RequestOf, strconv.FormatInt(p.Request, 10)})
	}
	id, err := repo.commit(paths(p.Files), p.By, message(p.Why, p.Says, p.LetsMoreBy, notes), c.Now())
	if id == "" {
		if back := putBack(c.dir, p.Files); back != nil {
			return "", nil, "", fmt.Errorf("the change could not be committed (%v), and its files could not all be put back as they were, so the office now holds what nobody decided: %w", err, back)
		}
		return "", nil, "it could not be committed, so its files were put back as they were: " + err.Error(), nil
	}
	troubles := []error{after, err}
	if p.Request != 0 {
		if err := c.clearDraft(ctx, p); err != nil {
			troubles = append(troubles, fmt.Errorf("the change is written as %s, but its draft could not be cleared: %w", id, err))
		}
	}
	if c.keeper != nil {
		c.keeper.Look()
	}
	return id, check.Gone, "", errors.Join(troubles...)
}

// clearDraft takes out of a request's draft each file the change wrote as
// the draft holds it. What the draft holds otherwise of those files was
// drafted after, and stays, drafted now against what the change wrote.
func (c *Changes) clearDraft(ctx context.Context, p Put) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	draft, err := drafted(ctx, tx, p.Request)
	if err != nil {
		return err
	}
	for _, f := range draft {
		i := slices.IndexFunc(p.Files, func(w File) bool { return w.Path == f.Path })
		switch {
		case i < 0:
			continue
		case p.Files[i].Gone == f.Gone && p.Files[i].Text == f.Text:
			err = dropDraft(ctx, tx, p.Request, f.Path)
		default:
			f.Was, f.Had = p.Files[i].Text, !p.Files[i].Gone
			err = keepDraft(ctx, tx, p.Request, f)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
