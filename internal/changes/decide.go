package changes

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// A Put is a change put to be decided: its files fixed as they were when
// it was put, and what it says, as whoever decides is shown them.
type Put struct {
	ID int64 `json:"id"`
	// Request is the request it was drafted in; none for a person's change
	// from the screens.
	Request int64 `json:"request,omitzero"`
	// By is the worker or the person who drafted it. Person says it is a
	// person's own change, put by By from the screens.
	By     string `json:"by"`
	Person bool   `json:"person,omitzero"`
	Why    string `json:"why"`
	Files  []File `json:"files"`
	// Says and LetsMoreBy are what it does, read from the files alone.
	Says       []string `json:"says"`
	LetsMoreBy []string `json:"lets_more_by"`
	// Walk are its deciders, in the order they are asked, read from the
	// office as it was when it was put. For a person's own change it holds
	// only the rules and workers before its first person: any refusal of
	// theirs ends it, and once every one has passed, it is decided by the
	// yes of By, a person. One whose walk is empty is written as it is put,
	// and never handed on to be decided.
	Walk []office.Decider `json:"walk"`
	At   time.Time        `json:"at"`
}

// A Decision is how a decider ended a change: a yes, or a refusal, with
// their note. A yes from a rule or a worker is a pass, and never comes
// here. Bulldog, of the kind office.IsBulldog, refuses a change that
// nobody on its walk is left to decide, and never says yes.
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
	// their note.
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

// Walk gives whom a change is asked of, in order, in the office as it is:
// the top team's change list, or its everything when it writes none; and
// where that names no person, the top team's everything after it, which
// always names one.
func Walk(o *office.Office) []office.Decider {
	list, ok := o.List(office.Top, "change")
	if !ok {
		return o.Everything(office.Top)
	}
	if slices.ContainsFunc(list, isPerson) {
		return list
	}
	return append(slices.Clone(list), o.Everything(office.Top)...)
}

func isPerson(d office.Decider) bool { return d.Kind == office.IsPerson }

// Deciders gives the people who decide a change: those the top team's
// change list names; where it names no person, those its everything names;
// where neither names a person, the top team's people. They alone put a
// change from the screens and read the history.
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
	if list, ok := o.List(office.Top, "change"); ok && len(people(list)) > 0 {
		return people(list)
	}
	if list, ok := o.List(office.Top, "everything"); ok && len(people(list)) > 0 {
		return people(list)
	}
	return slices.Clone(o.TopTeam().People)
}

// putDraft puts a request's draft to be decided, and has the call wait for
// how it ends.
func (c *Changes) putDraft(ctx context.Context, call Call, why string) (Answer, error) {
	files, err := drafted(ctx, c.db, call.Request)
	if err != nil {
		return Answer{}, err
	}
	p, check, err := c.put(ctx, Drafter{Name: call.Worker}, call.Request, why, files, "")
	if err != nil {
		return Answer{}, err
	}
	if p.ID == 0 {
		return Answer{Text: "It was not put to be decided. " + check}, nil
	}
	if err := keepTaken(ctx, c.db, call.Request, files, p.Files); err != nil {
		return Answer{}, fmt.Errorf("the change %d is put, but what it takes with a worker it moves could not be added to its draft: %w", p.ID, err)
	}
	return Answer{Text: "It is put to be decided.", Put: p.ID}, nil
}

// PutOwn puts a person's own change, drafted on the screens, to be
// decided. Only the people who decide a change put one. What is put is
// exactly what they were shown: the mark of their check must be that of
// the change as it is now, or nothing is put and they are told to look
// again; a change put with no mark was shown to nobody, and is not put.
// One that no rule or worker is asked of before them is written at once,
// with their yes, and how it ended is told; one that is asked of them waits
// for them. It gives the change put, or why it was not; an error beside a
// change put is what went wrong in writing it.
func (c *Changes) PutOwn(ctx context.Context, person, why string, files []File, mark string) (Put, error) {
	o, err := c.office()
	if err != nil {
		return Put{}, err
	}
	if !slices.Contains(Deciders(o), person) {
		return Put{}, refuse("%s does not decide changes to the office, so cannot put one: ask for it in a request, and a builder drafts it", person)
	}
	p, said, err := c.put(ctx, Drafter{Name: person, Person: true}, 0, why, files, mark)
	if err != nil {
		return p, err
	}
	if p.ID == 0 {
		return Put{}, Refusal(said)
	}
	return p, nil
}

// office reads the office in the folder now, to find who decides a change
// to it. An office whose reading is refused, by an edit by hand, is read
// as far as it goes: a change may mend it, so long as its walk still comes
// to a person.
func (c *Changes) office() (*office.Office, error) {
	folder, _, err := c.folder()
	if err != nil {
		return nil, err
	}
	o, problems := office.Parse(c.dir, folder)
	if problems.Refuse() && !slices.ContainsFunc(Walk(o), isPerson) {
		return nil, refuse("the office does not read, so nobody can be found to decide a change: %v", problems)
	}
	return o, nil
}

// put checks a change and puts it to be decided, in place of any change
// put from the same request that waits, which is withdrawn. A change that
// cannot be put is not, and what it gives says why. What is put is the
// change as checked, with what a worker it moves takes with it. One change
// is put at a time, so that one put after another from the same request
// withdraws only a change already handed on to be decided. A person's
// change is put only with the mark of the check they were shown, and is
// decided at once by their yes where nobody is asked of it before them;
// an error beside the change put is then what went wrong in writing it.
func (c *Changes) put(ctx context.Context, by Drafter, request int64, why string, files []File, mark string) (Put, string, error) {
	if c.Decide == nil {
		return Put{}, "", errNoDecide
	}
	c.putting.Lock()
	defer c.putting.Unlock()
	check, err := c.Check(ctx, files, by)
	if err != nil {
		return Put{}, "", err
	}
	if strings.TrimSpace(why) == "" {
		check.Refused = append(check.Refused, "it says no why: say why in words a person can decide on, its first line saying what it is")
	}
	if by.Person && mark != check.Mark {
		check.Refused = append(check.Refused, "the change is not as it was when it was shown: the folder has moved on since. Look at it again")
	}
	if !check.Fit() {
		return Put{}, check.Text(), nil
	}
	o, err := c.office()
	if err != nil {
		return Put{}, "", err
	}
	walk := Walk(o)
	if by.Person {
		if i := slices.IndexFunc(walk, isPerson); i >= 0 {
			walk = slices.Clone(walk[:i])
		}
	}
	p := Put{Request: request, By: by.Name, Person: by.Person, Why: strings.TrimSpace(why), Files: check.Files,
		Says: check.Says, LetsMoreBy: check.LetsMoreBy, Walk: walk, At: c.Now()}
	withdrawn, err := c.keepInPlace(ctx, &p)
	if err != nil {
		return Put{}, "", err
	}
	if withdrawn != nil {
		c.tell(*withdrawn)
	}
	if p.Person && len(p.Walk) == 0 {
		d := Decision{By: p.By, Kind: office.IsPerson, Yes: true}
		if person := o.People[p.By]; person != nil {
			d.Called = person.Called
		}
		_, err := c.Decided(ctx, p.ID, d)
		return p, "", err
	}
	if err := c.Decide(p); err != nil {
		o := Outcome{Put: p.ID, Request: request, By: by.Name, Ended: NotWritten, Why: "it could not be put to be decided: " + err.Error(), At: c.Now()}
		if _, kept := endPut(ctx, c.db, o); kept != nil {
			return Put{}, "", errors.Join(err, kept)
		}
		return Put{}, "", fmt.Errorf("the change could not be put to be decided: %w", err)
	}
	return p, "", nil
}

// keepInPlace keeps a change put, withdrawing the change put from the same
// request that waits, and gives how that one ended. Both are kept, or
// neither.
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
		waiting, err := waitingOf(ctx, tx, p.Request)
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
	return gotPut(ctx, c.db, id)
}

// Decided ends a change put with its decision: a refusal writes nothing,
// and a person's yes writes it. Only a person's yes writes a change, and
// only from a person its walk asks, or the person who put their own; a
// refusal is taken from a decider its walk asks, or from Bulldog where
// nobody on the walk is left to decide it. A decision on a change that has
// ended changes nothing, and says how it ended. Anything that stops a yes being written ends it as not written,
// with why. The outcome is told, and given. A change written may come with
// an error all the same: it is written and kept, and the error is what went
// wrong after, such as its draft not being cleared, for a person to read.
func (c *Changes) Decided(ctx context.Context, id int64, d Decision) (Outcome, error) {
	o, err := c.decided(ctx, id, d)
	if o.Ended != "" {
		c.tell(o)
	}
	return o, err
}

// decided ends a change with its decision, once: two answers that come
// together are taken one at a time, and the second finds it ended. It
// gives the outcome only when this decision ended it.
func (c *Changes) decided(ctx context.Context, id int64, d Decision) (Outcome, error) {
	c.deciding.Lock()
	defer c.deciding.Unlock()
	p, ended, err := gotPut(ctx, c.db, id)
	if err != nil {
		return Outcome{}, err
	}
	if ended.Ended != "" {
		return Outcome{}, fmt.Errorf("the change %d has ended already: it was %s", id, ended.Ended)
	}
	asked := slices.ContainsFunc(p.Walk, func(x office.Decider) bool { return x.Name == d.By && x.Kind == d.Kind }) ||
		p.Person && p.By == d.By && d.Kind == office.IsPerson || d.Kind == office.IsBulldog
	switch {
	case d.Yes && d.Kind == office.IsBulldog:
		return Outcome{}, fmt.Errorf("Bulldog only refuses a change nobody can decide, and its yes on the change %d is not taken", id)
	case d.Yes && d.Kind != office.IsPerson:
		return Outcome{}, fmt.Errorf("%s is a %s, and only a person's yes writes a change to the office: a yes from a rule or a worker is a pass", d.By, d.Kind)
	case !asked:
		return Outcome{}, fmt.Errorf("%s is not asked to decide the change %d, so what they decide is not taken", d.By, id)
	}
	if d.At.IsZero() {
		d.At = c.Now()
	}
	o := Outcome{Put: id, Request: p.Request, By: p.By, Decider: d.By, Called: cmp.Or(d.Called, d.By), Note: d.Note, At: d.At}
	if d.Kind == office.IsBulldog {
		o.Decider, o.Called = bulldog, bulldog
	}
	if !d.Yes {
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
		return o, errors.Join(err, fmt.Errorf("how the change %d ended could not be kept: %w", id, kept))
	}
	return o, err
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
