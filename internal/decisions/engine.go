package decisions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// effects are what a change to a decision leads to beyond this area: done
// once the decision is kept, with no lock held, since each calls out. The
// verdicts are kept with their asks, and what the decisions owe with them.
type effects struct {
	verdicts []verdict
	owed     []*Decision
	touched  []int64
}

type verdict struct {
	ask string
	v   Verdict
}

func (fx *effects) touch(id int64) {
	if !slices.Contains(fx.touched, id) {
		fx.touched = append(fx.touched, id)
	}
}

// changing makes one change to the decisions, in the office as it was
// last read.
func (d *Decisions) changing(ctx context.Context, do func(o *office.Office, fx *effects) error) error {
	return d.doing(ctx, func(fx *effects) error {
		o, err := d.look()
		if err != nil {
			return err
		}
		return do(o, fx)
	})
}

// doing makes one change to the decisions, one at a time, and then does
// what it leads to.
func (d *Decisions) doing(ctx context.Context, do func(fx *effects) error) error {
	fx := &effects{}
	d.mu.Lock()
	err := do(fx)
	d.mu.Unlock()
	return errors.Join(err, d.after(ctx, fx))
}

// changingOne makes one change to one decision.
func (d *Decisions) changingOne(ctx context.Context, id int64, do func(o *office.Office, dec *Decision, fx *effects) error) error {
	return d.changing(ctx, func(o *office.Office, fx *effects) error {
		dec, err := load(d.db, id)
		if err != nil {
			return err
		}
		return do(o, dec, fx)
	})
}

// after does what changes to decisions led to: hands work what was
// decided of each ask whose jobs are all decided, to answer it with, and
// does what each decision that ended owes.
func (d *Decisions) after(ctx context.Context, fx *effects) error {
	var errs []error
	for _, v := range fx.verdicts {
		if d.Answer == nil {
			errs = append(errs, fmt.Errorf("the ask %s is decided, but nothing keeps what was decided to answer Corbie with", v.ask))
			continue
		}
		if err := d.Answer(ctx, v.ask, v.v); err != nil {
			errs = append(errs, fmt.Errorf("the ask %s is decided, but what was decided could not be kept to answer Corbie with: %w", v.ask, err))
		}
	}
	for _, dec := range fx.owed {
		errs = append(errs, d.settle(ctx, dec))
	}
	d.tell(fx.touched)
	return errors.Join(errs...)
}

// settle does what an ended decision still owes, keeping each step once
// it is done: how it ended is written in the request's thread; then a
// disclosed answer is said, and work told how the disclosure ended; or
// Changes is told how a change ended, unless it has ended there already.
func (d *Decisions) settle(ctx context.Context, dec *Decision) error {
	if dec.owed == oweEntries {
		o, err := d.look()
		if err != nil {
			return err
		}
		if dec.Ended == Withdrawn {
			err = d.withdrawn(o, dec)
		} else {
			err = d.ended(o, dec)
		}
		if err != nil {
			return err
		}
		if err := d.owe(dec, owedBeyond(dec)); err != nil {
			return err
		}
	}
	if dec.owed == oweSay {
		if err := d.say(dec); err != nil {
			return err
		}
		if err := d.owe(dec, oweLet); err != nil {
			return err
		}
	}
	switch dec.owed {
	case oweLet:
		if d.Let == nil {
			return fmt.Errorf("the disclosure %d ended, but nothing tells work", dec.ID)
		}
		if err := d.Let(ctx, letOf(dec)); err != nil {
			return fmt.Errorf("the disclosure %d ended, but work could not be told: %w", dec.ID, err)
		}
	case oweDecided:
		_, out, err := d.changes.Put(ctx, dec.Put.ID)
		if err != nil {
			return fmt.Errorf("the change %d was decided, but: %w", dec.Put.ID, err)
		}
		if out.Ended == "" {
			if _, err := d.changes.Decided(ctx, dec.Put.ID, decisionOf(dec)); err != nil {
				return fmt.Errorf("the change %d was decided, but: %w", dec.Put.ID, err)
			}
		}
	}
	if dec.owed == "" {
		return nil
	}
	return d.owe(dec, "")
}

// decisionOf gives how a change ended, as Changes is told it: by the
// decider who ended it, or by Bulldog, by its name, where nobody on the
// change's walk was left to decide it.
func decisionOf(dec *Decision) changes.Decision {
	by := dec.By
	name := by.Name
	if by.Kind == requests.IsBulldog {
		name = by.Called
	}
	return changes.Decision{By: name, Kind: by.Kind, Called: by.Called, Yes: dec.Ended == Yes, Note: by.Note, At: by.At}
}

// owing gives what a decision owes once it has ended: its entries in the
// request's thread, and then what lies beyond this area.
func owing(dec *Decision) string {
	if dec.Request != 0 {
		return oweEntries
	}
	return owedBeyond(dec)
}

// owedBeyond gives what an ended decision owes beyond this area: a
// disclosed answer to be said, work to be told how a disclosure ended, or
// Changes how a change did.
func owedBeyond(dec *Decision) string {
	switch {
	case dec.Kind == OfDisclosure && dec.Ended == Yes && dec.Said.What == SaysAnswer:
		return oweSay
	case dec.Kind == OfDisclosure && dec.Ended != Withdrawn:
		return oweLet
	case dec.Kind == OfChange && dec.Ended != Withdrawn:
		return oweDecided
	}
	return ""
}

// owe keeps what an ended decision still owes.
func (d *Decisions) owe(dec *Decision, owed string) error {
	if _, err := d.db.Exec(`UPDATE decisions_decisions SET owed = ? WHERE id = ?`, owed, dec.ID); err != nil {
		return failed(err)
	}
	dec.owed = owed
	return nil
}

// advance walks a decision on from where it stands, over the lists as the
// office is read now, until it waits on someone or ends.
func (d *Decisions) advance(o *office.Office, dec *Decision, fx *effects) error {
	fx.touch(dec.ID)
	for dec.Ended == "" {
		if dec.Kind == OfDisclosure && o.Teams[dec.Team] != nil && !o.Teams[dec.Team].HasDisclosure {
			return d.end(dec, Yes, Word{Kind: requests.IsBulldog, Called: "Bulldog", Note: called(o, dec.Team) + " no longer sets a disclosure check"}, fx)
		}
		stages, why := walk(o, dec)
		s, i, ok := next(stages, dec)
		switch {
		case !ok && dec.Kind == OfChange && dec.Put.Person:
			return d.end(dec, Yes, d.personWord(o, dec.Put.By, ""), fx)
		case !ok:
			if o.Teams[dec.Team] == nil {
				why = dec.Team + " is no longer in the office"
			}
			return d.end(dec, Refused, Word{Kind: requests.IsBulldog, Called: "Bulldog", Note: "nobody can decide this: " + why}, fx)
		case s.Kind == IsPeople:
			return d.askPeople(o, dec, s)
		case dec.At != nil && s.is(dec.At.Kind, dec.At.Name):
			d.goOn(o, dec, stages, i)
			return nil
		case ownWorker(dec, s):
			d.stopRun(dec.ID)
			dec.At = nil
			if err := d.refer(o, dec, Word{Kind: s.Kind, Name: s.Name, Called: s.Name,
				Note: "a worker never decides its own action, so being reached counts as its referral", At: d.Now()}); err != nil {
				return err
			}
		default:
			return d.ask(o, dec, stages, i)
		}
	}
	return nil
}

// ask asks a rule or a worker to decide, as the next step of its walk.
func (d *Decisions) ask(o *office.Office, dec *Decision, stages []Stage, i int) error {
	d.stopRun(dec.ID)
	dec.Step++
	dec.At = &Stand{Kind: stages[i].Kind, Name: stages[i].Name, Since: d.Now()}
	if err := save(d.db, dec); err != nil {
		return err
	}
	d.goOn(o, dec, stages, i)
	return nil
}

// goOn has the rule or worker a decision stands at run on it, unless its
// run is going already: after a restart, its run is taken up again.
func (d *Decisions) goOn(o *office.Office, dec *Decision, stages []Stage, i int) {
	d.state.Lock()
	g := d.running[dec.ID]
	d.state.Unlock()
	if g != nil && g.step == dec.Step {
		return
	}
	s := Start{Decision: dec.ID, Step: dec.Step, Kind: stages[i].Kind, Name: stages[i].Name, Team: dec.Team, Request: dec.Request,
		Labels: map[string]string{requests.LabelTeam: dec.Team, LabelKind: Deciding,
			LabelDecision: strconv.FormatInt(dec.ID, 10), LabelStep: strconv.Itoa(dec.Step)}}
	if dec.Request != 0 {
		s.Labels[requests.LabelRequest] = strconv.FormatInt(dec.Request, 10)
	}
	input, said, err := d.input(o, dec, stages, i)
	if err != nil {
		d.launch(s, fmt.Errorf("%w: what it is to decide could not be put together: %v", ErrNotStarted, err))
		return
	}
	s.Input, s.Said = input, said
	d.launch(s, nil)
}

// launch starts a decider's run on its own goroutine, and takes its
// result once it ends; a run cancelled is not read.
func (d *Decisions) launch(s Start, unstarted error) {
	ctx, cancel := context.WithCancel(d.base)
	d.state.Lock()
	if d.base.Err() != nil {
		// Closed: the run is taken up when the decisions are opened again.
		d.state.Unlock()
		cancel()
		return
	}
	if g := d.running[s.Decision]; g != nil {
		g.cancel()
	}
	d.running[s.Decision] = &going{step: s.Step, cancel: cancel}
	d.runs.Add(1)
	d.state.Unlock()
	go func() {
		defer d.runs.Done()
		defer cancel()
		out, err := []byte(nil), unstarted
		switch {
		case unstarted != nil:
		case d.Run == nil:
			err = fmt.Errorf("%w: nothing in Bulldog runs rules and workers", ErrNotStarted)
		default:
			out, err = d.Run(ctx, s)
		}
		if ctx.Err() != nil {
			return
		}
		d.trouble(d.taken(context.WithoutCancel(ctx), s.Decision, s.Step, out, err))
	}()
}

// stopRun cancels the run a decision waits on, if any: what it ends with
// is not read.
func (d *Decisions) stopRun(id int64) {
	d.state.Lock()
	defer d.state.Unlock()
	if g := d.running[id]; g != nil {
		g.cancel()
		delete(d.running, id)
	}
}

// taken takes what a rule or worker ended with, once, from the run the
// decision waits on at that step, and walks on.
func (d *Decisions) taken(ctx context.Context, id int64, step int, out []byte, failure error) error {
	return d.changingOne(ctx, id, func(o *office.Office, dec *Decision, fx *effects) error {
		if dec.Ended != "" || dec.At == nil || dec.At.Kind == IsPeople || dec.Step != step || d.base.Err() != nil {
			return nil
		}
		d.state.Lock()
		if g := d.running[id]; g != nil && g.step == step {
			delete(d.running, id)
		}
		d.state.Unlock()
		says, note := result(out, failure)
		w := Word{Kind: dec.At.Kind, Name: dec.At.Name, Called: calledOf(o, dec.At.Kind, dec.At.Name), Note: note, At: d.Now()}
		if dec.Kind == OfChange && says == SaysYes {
			says, w.Note = SaysRefer, "it said yes, and only a person's yes approves a change, so it is taken as a referral"+noted(note)
		}
		switch says {
		case SaysYes:
			return d.end(dec, Yes, w, fx)
		case SaysRefuse:
			return d.end(dec, Refused, w, fx)
		}
		dec.At = nil
		if err := d.refer(o, dec, w); err != nil {
			return err
		}
		return d.advance(o, dec, fx)
	})
}

// result reads what a rule or a worker ended with: { says, note }, or a
// referral saying why it is not a decision.
func result(out []byte, failure error) (says, note string) {
	switch {
	case errors.Is(failure, ErrNotStarted):
		return SaysRefer, failure.Error()
	case failure != nil:
		return SaysRefer, "it failed: " + failure.Error()
	}
	var r struct {
		Says *string `json:"says"`
		Note any     `json:"note"`
	}
	ok := json.Unmarshal(out, &r) == nil && r.Says != nil
	if !ok {
		var text string
		if json.Unmarshal(out, &text) == nil && json.Unmarshal([]byte(strings.TrimSpace(text)), &r) == nil && r.Says != nil {
			ok = true
		}
	}
	if !ok || !slices.Contains([]string{SaysYes, SaysRefuse, SaysRefer}, *r.Says) {
		return SaysRefer, "its answer was not a decision, which is { says: yes, refuse or refer, note }: " + shorten(string(out), 200)
	}
	switch n := r.Note.(type) {
	case nil:
	case string:
		note = n
	default:
		note = jsonOf(n)
	}
	return *r.Says, note
}

// refer keeps a rule's or worker's referral, as an entry to whoever the
// decision is for.
func (d *Decisions) refer(o *office.Office, dec *Decision, w Word) error {
	dec.Referrals = append(dec.Referrals, w)
	if err := save(d.db, dec); err != nil {
		return err
	}
	if dec.Request == 0 {
		return nil
	}
	to, err := d.recipient(dec)
	if err != nil {
		return err
	}
	_, err = d.requests.Write(dec.Request, requests.Writing{Kind: KindReferred, By: d.whoOf(dec, w), To: to, Read: true, Decision: dec.ID,
		Text: "referred by " + w.Called + noted(w.Note)})
	return err
}

// askPeople asks the people of a stage, all at once, in one entry that
// waits on them all, and tells whoever was asked before and is not now
// that they are asked no longer.
func (d *Decisions) askPeople(o *office.Office, dec *Decision, s Stage) error {
	if dec.At != nil && dec.At.Kind == IsPeople && slices.Equal(dec.Asked, s.People) {
		return nil
	}
	d.stopRun(dec.ID)
	var added, removed []string
	for _, p := range s.People {
		if !slices.Contains(dec.Asked, p) {
			added = append(added, p)
		}
	}
	for _, p := range dec.Asked {
		if !slices.Contains(s.People, p) {
			removed = append(removed, p)
		}
	}
	dec.At, dec.Asked = &Stand{Kind: IsPeople, Since: d.Now()}, s.People
	for _, p := range added {
		if !slices.Contains(dec.WasAsked, p) {
			dec.WasAsked = append(dec.WasAsked, p)
		}
	}
	if err := save(d.db, dec); err != nil {
		return err
	}
	if dec.Request == 0 {
		return nil
	}
	persons := d.persons(o, s.People)
	switch {
	case dec.Entry == 0 || len(added) > 0:
		if err := d.close(dec); err != nil {
			return err
		}
		text, err := d.shown(o, dec, s.People)
		if err != nil {
			return err
		}
		e, err := d.requests.Write(dec.Request, requests.Writing{Kind: KindDecide, By: requests.Bulldog, To: persons, Waits: persons, Read: true, Decision: dec.ID, Text: text})
		if err != nil {
			return err
		}
		dec.Entry = e.ID
		if err := save(d.db, dec); err != nil {
			return err
		}
	default:
		if err := d.requests.Rewait(dec.Entry, persons); err != nil {
			return err
		}
	}
	if err := d.notice(o, dec, removed, "You are no longer asked to decide this: "+d.doesInSentence(o, dec)+"."); err != nil {
		return err
	}
	if err := d.unsent(o, dec, added); err != nil {
		return err
	}
	return d.record(o, dec, fmt.Sprintf("%s: waiting on %s", d.Does(o, dec), names(o, s.People)), nil)
}

// unsent writes in the thread that a direct message to a person asked
// could not be sent on a bridge of theirs that a problem stops: they are
// asked on the screens and their other bridges all the same.
func (d *Decisions) unsent(o *office.Office, dec *Decision, people []string) error {
	for _, name := range people {
		for _, id := range o.People[name].Reached {
			if b := o.Bridges[id.Bridge]; b == nil || b.Runs() {
				continue
			}
			text := fmt.Sprintf("The direct message asking %s to decide could not be sent on %s, which a problem of the office stops: %s is asked on the screens and on any other bridge.",
				o.People[name].Called, capital(id.Bridge), o.People[name].Called)
			if _, err := d.requests.Write(dec.Request, requests.Writing{Kind: KindNotice, By: requests.Bulldog, Read: true, Decision: dec.ID, Text: text}); err != nil {
				return err
			}
		}
	}
	return nil
}

// end ends a decision with a yes or a refusal, which stands: it is kept
// first, with the ask's answer once every job of the ask is decided and
// with what it owes, and only then is anyone told or anything answered. A
// decision that cannot be kept is left as it was.
func (d *Decisions) end(dec *Decision, ended string, by Word, fx *effects) error {
	if by.At.IsZero() {
		by.At = d.Now()
	}
	done := *dec
	done.Ended, done.By, done.Decided, done.At = ended, &by, by.At, nil
	if by.Kind == requests.IsBulldog {
		done.Why = by.Note
	}
	done.owed = owing(&done)
	v, err := d.keepEnd(&done)
	if err != nil {
		return err
	}
	*dec = done
	d.stopRun(dec.ID)
	fx.touch(dec.ID)
	if v != nil {
		fx.verdicts = append(fx.verdicts, verdict{ask: dec.Ask, v: *v})
	}
	if dec.owed != "" {
		fx.owed = append(fx.owed, dec)
	}
	return nil
}

// keepEnd keeps an ended decision and, for an action whose ask has every
// job decided now, the ask's answer, both or neither. It gives that
// answer.
func (d *Decisions) keepEnd(dec *Decision) (*Verdict, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, failed(err)
	}
	defer tx.Rollback()
	if err := save(tx, dec); err != nil {
		return nil, err
	}
	var v *Verdict
	if dec.Kind == OfAction {
		if v, err = verdictOf(tx, dec.Ask); err != nil {
			return nil, err
		}
	}
	if v != nil {
		if _, err := tx.Exec(`UPDATE decisions_asks SET verdict = ? WHERE run = ?`, jsonOf(v), dec.Ask); err != nil {
			return nil, failed(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, failed(err)
	}
	return v, nil
}

// ended writes how a decision ended in the request's thread: the
// decider's word, with its note, to whoever the decision is for, and the
// line of what was done, to whoever else was asked.
func (d *Decisions) ended(o *office.Office, dec *Decision) error {
	to, err := d.recipient(dec)
	if err != nil {
		return err
	}
	w := requests.Writing{Kind: KindDecided, By: d.whoOf(dec, *dec.By), To: to, Read: true, Answers: dec.Entry, Decision: dec.ID, Text: how(dec) + noted(dec.By.Note)}
	w.Working = dec.Ended == Refused && dec.Kind == OfDisclosure && dec.Said.What == SaysAnswer
	if _, err := d.requests.Write(dec.Request, w); err != nil {
		return err
	}
	others := slices.DeleteFunc(slices.Clone(dec.WasAsked), func(p string) bool { return dec.By.Kind == office.IsPerson && p == dec.By.Name })
	return d.record(o, dec, d.line(o, dec), others)
}

// withdraw ends a decision as withdrawn: whatever runs on it is cancelled,
// and those asked are told they are asked no longer.
func (d *Decisions) withdraw(dec *Decision, why string, fx *effects) error {
	if dec.Ended != "" {
		return nil
	}
	dec.Ended, dec.Why, dec.Decided, dec.At = Withdrawn, why, d.Now(), nil
	dec.owed = owing(dec)
	if err := save(d.db, dec); err != nil {
		return err
	}
	d.stopRun(dec.ID)
	fx.touch(dec.ID)
	if dec.owed != "" {
		fx.owed = append(fx.owed, dec)
	}
	return nil
}

// withdrawn writes in the request's thread that a decision was withdrawn,
// closing what its people are asked in and telling them.
func (d *Decisions) withdrawn(o *office.Office, dec *Decision) error {
	if err := d.close(dec); err != nil {
		return err
	}
	if err := d.notice(o, dec, dec.Asked, fmt.Sprintf("%s: withdrawn, so you are no longer asked: %s.", d.Does(o, dec), dec.Why)); err != nil {
		return err
	}
	return d.record(o, dec, d.line(o, dec), nil)
}

// close closes the entry the people asked are asked in.
func (d *Decisions) close(dec *Decision) error {
	if dec.Entry == 0 || len(dec.Asked) == 0 {
		return nil
	}
	err := d.requests.Rewait(dec.Entry, nil)
	var refusal requests.Refusal
	if errors.As(err, &refusal) {
		// It was closed already, as a request stopped closes what waits
		// in it.
		return nil
	}
	return err
}

// verdictOf gives how an ask is answered once every job it holds is
// decided, naming the jobs that may run and joining the notes, each by
// what its tool says it does, or nil while one waits.
func verdictOf(q querier, ask string) (*Verdict, error) {
	decs, err := some(q, `ask = ?`, ask)
	if err != nil {
		return nil, err
	}
	v := Verdict{Approve: []string{}}
	var notes []string
	for _, dec := range decs {
		switch dec.Ended {
		case "":
			return nil, nil
		case Yes:
			v.Approve = append(v.Approve, dec.Job.Name)
		case Withdrawn:
			notes = append(notes, fmt.Sprintf("%s: withdrawn, %s.", dec.Job.does(), dec.Why))
		default:
			notes = append(notes, fmt.Sprintf("%s: %s", dec.Job.does(), how(dec))+noted(dec.By.Note))
		}
	}
	v.Note = strings.Join(notes, "; ")
	return &v, nil
}

// record writes what was done of a decision: what it is, who it waits on
// and how it ended, to whoever else was asked. Of a disclosure nothing is
// what was done until it is let go, so its record is what was read of the
// team.
func (d *Decisions) record(o *office.Office, dec *Decision, text string, to []string) error {
	_, err := d.requests.Write(dec.Request, requests.Writing{Kind: KindDecision, By: requests.Bulldog, To: d.persons(o, to),
		Read: dec.Kind == OfDisclosure, Decision: dec.ID, Text: text})
	return err
}

// notice tells people of a decision something, in an entry to them.
func (d *Decisions) notice(o *office.Office, dec *Decision, people []string, text string) error {
	to := d.persons(o, people)
	if len(to) == 0 {
		return nil
	}
	_, err := d.requests.Write(dec.Request, requests.Writing{Kind: KindNotice, By: requests.Bulldog, To: to, Read: true, Decision: dec.ID, Text: text})
	return err
}

// line gives what was done of a decision as one line, built as it is
// written: "Refund one charge: approved by Lee, 14:02".
func (d *Decisions) line(o *office.Office, dec *Decision) string {
	at := dec.Decided.In(o.Zone).Format("15:04")
	switch dec.Ended {
	case "":
		return d.Does(o, dec) + ": " + d.waitsOn(o, dec)
	case Withdrawn:
		return fmt.Sprintf("%s: withdrawn, %s: %s", d.Does(o, dec), at, dec.Why)
	}
	return fmt.Sprintf("%s: %s, %s", d.Does(o, dec), how(dec), at)
}

// how says who ended a decision with a yes or a refusal: "approved by
// Lee", "refused by Lee".
func how(dec *Decision) string {
	if dec.Ended == Refused {
		return "refused by " + dec.By.Called
	}
	return "approved by " + dec.By.Called
}

// waitsOn says whom a decision waits on now.
func (d *Decisions) waitsOn(o *office.Office, dec *Decision) string {
	switch {
	case dec.At == nil:
		return "not yet asked"
	case dec.At.Kind == IsPeople:
		return "waiting on " + names(o, dec.Asked)
	}
	return "with " + calledOf(o, dec.At.Kind, dec.At.Name)
}

// recipient gives whom a decider's word on a decision is to: the worker
// whose action it is, the team's lead for a disclosure, and whoever drafted
// a change.
func (d *Decisions) recipient(dec *Decision) ([]requests.Who, error) {
	if dec.Kind != OfDisclosure {
		if dec.Whose.Kind == "" {
			return nil, nil
		}
		return []requests.Who{dec.Whose}, nil
	}
	name, person, err := d.requests.Lead(dec.Request)
	switch {
	case err != nil:
		return nil, err
	case name == "":
		return nil, nil
	case person:
		return []requests.Who{d.requests.Person(name)}, nil
	}
	return []requests.Who{d.requests.Worker(name)}, nil
}

// whoOf gives who a decider's word on a decision is by, in the thread: a
// rule as of the team whose decision it is.
func (d *Decisions) whoOf(dec *Decision, w Word) requests.Who {
	switch w.Kind {
	case office.IsPerson:
		return d.requests.Person(w.Name)
	case office.IsWorker:
		return d.requests.Worker(w.Name)
	case office.IsRule:
		return d.requests.Rule(w.Name, dec.Team)
	}
	return requests.Bulldog
}

// persons gives the people of the office among names, as entries are to
// them.
func (d *Decisions) persons(o *office.Office, people []string) []requests.Who {
	var whos []requests.Who
	for _, p := range people {
		if o.People[p] != nil {
			whos = append(whos, d.requests.Person(p))
		}
	}
	return whos
}

// personWord gives a person's word.
func (d *Decisions) personWord(o *office.Office, name, note string) Word {
	return Word{Kind: office.IsPerson, Name: name, Called: calledOf(o, office.IsPerson, name), Note: note, At: d.Now()}
}

// names gives people as a sentence names them: "Lee, Priya".
func names(o *office.Office, people []string) string {
	var called []string
	for _, p := range people {
		called = append(called, calledOf(o, office.IsPerson, p))
	}
	return strings.Join(called, ", ")
}

func noted(note string) string {
	if note == "" {
		return "."
	}
	return ": " + note
}

// lower gives a sentence with its first letter small, and capital with it
// large.
func lower(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

func capital(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// shorten gives at most n letters of a text, saying where it was cut.
func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
