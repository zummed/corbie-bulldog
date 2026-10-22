package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/client"
)

// timedOut is the note wait.yaml ends with when nobody answers it.
const timedOut = "No answer came within 7 days."

// letGo is how long after a request ended its answer is still carried: a
// run that carried it may be past keeping after that, and a run of its
// name would start again.
const letGo = 24 * time.Hour

// A Wait is a run of wait.yaml below a request: a question to a team or to
// people, or a decision put to people.
type Wait struct {
	ID       string    // its run's name
	Ref      string    // what a message carrying it holds, and a press gives back
	Request  string    // the request it is below
	Team     string    // the asking team
	ToTeam   string    // the team asked, for a question to a team
	People   []string  // the people asked
	Said     string    // the question, or what is decided and the notes before
	Choices  []string  // offered as buttons
	Decision string    // heed, check or decide for a decision; empty for a question
	Status   string    // waiting, answered, refused, timed-out or stopped
	Answer   *string   // the answer given, if any
	By       string    // the person who answered
	Refused  string    // why Bulldog refused the question
	Asked    string    // for a question to a team, not refused: the run of the request it asks
	Started  time.Time //
	Ended    time.Time //
}

// ErrNoQuestion is an answer to a wait that is not waiting on the one who
// answers: it waits on others, or it no longer waits.
var ErrNoQuestion = errors.New("there is no such question waiting on you")

// ErrNotAChoice is an answer to a decision that is not one of its choices.
var ErrNotAChoice = errors.New("the answer to a decision is one of the choices it offers")

// waitOf reads a wait from its run as shown.
func (e *Engine) waitOf(run *client.Run) *Wait {
	var in struct {
		To struct {
			Team   string   `json:"team"`
			People []string `json:"people"`
		} `json:"to"`
		Said     string   `json:"said"`
		Choices  []string `json:"choices"`
		Decision string   `json:"decision"`
	}
	decode(run.Input, &in)
	var out struct {
		Answer  *string `json:"answer"`
		By      string  `json:"by"`
		Refused string  `json:"refused"`
		Note    string  `json:"note"`
	}
	decode(run.Output, &out)
	w := &Wait{ID: run.ID, Ref: e.hash("ref", run.ID), Team: run.Labels["team"], ToTeam: in.To.Team, People: in.To.People,
		Said: in.Said, Choices: in.Choices, Decision: in.Decision, Answer: out.Answer, By: out.By, Refused: out.Refused,
		Started: run.Started, Ended: run.Ended}
	if w.ToTeam != "" && w.Refused == "" {
		w.Asked = e.name(named(run.ID), "team", run.ID)
	}
	switch {
	case !run.HasEnded():
		w.Status = "waiting"
	case run.Status != "done":
		w.Status = "stopped"
	case out.Refused != "":
		w.Status = "refused"
	case out.Answer == nil && out.Note == timedOut:
		w.Status = "timed-out"
	default:
		w.Status = "answered"
	}
	return w
}

// wait gives a wait by its run's name.
func (e *Engine) wait(ctx context.Context, id string) (*Wait, error) {
	run, err := e.corbie.Show(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.Machine != "wait" || run.Labels["bulldog"] != e.Reading().Office.ID {
		return nil, ErrNoQuestion
	}
	return e.waitOf(run), nil
}

// mayAskPerson says whether a team may ask a person: one of the office,
// of the team or named in its team file.
func mayAskPerson(o *office.Office, t *office.Team, person string) bool {
	return slices.Contains(t.MayAsk(), person) && o.Person(person) != nil
}

// mayAskAll says whether the asking team may ask every person a wait asks.
// A wait that asks another is refused, and is no question to anyone.
func mayAskAll(r *Reading, w *Wait) bool {
	t := r.Office.Team(w.Team)
	return t != nil && !slices.ContainsFunc(w.People, func(p string) bool { return !mayAskPerson(r.Office, t, p) })
}

// asked acts on a wait that waits on its answer: a question to a team, or
// to people. The asking team is the run's team label, which it carries
// from its request.
func (e *Engine) asked(ctx context.Context, r *Reading, run *client.Run) error {
	w, err := e.wait(ctx, run.ID)
	if err != nil {
		return err
	}
	asking := r.Office.Team(w.Team)
	switch {
	case w.ToTeam != "":
		return e.askTeam(ctx, r, w, asking)
	case len(w.People) > 0:
		return e.askPeople(ctx, r, w, asking)
	}
	return e.refuse(ctx, w, "A question names the team or the people it asks.")
}

// refuse answers a wait as refused, saying why.
func (e *Engine) refuse(ctx context.Context, w *Wait, why string) error {
	return e.reply(ctx, w.ID, map[string]any{"answer": nil, "refused": why})
}

// askTeam starts a request of the team a wait asks, named from the wait,
// or refuses the question: to the asking team itself, to a team of its
// chain, and to a team that takes no requests.
func (e *Engine) askTeam(ctx context.Context, r *Reading, w *Wait, asking *office.Team) error {
	top, err := e.Top(ctx, w.ID)
	if err != nil {
		return err
	}
	q := e.requestOf(top)
	why := ""
	switch {
	case asking == nil:
		why = fmt.Sprintf("The team %s is no team of the office now.", w.Team)
	case w.ToTeam == asking.ID:
		why = fmt.Sprintf("%s cannot ask itself.", asking.Name)
	case slices.Contains(q.Chain, w.ToTeam):
		why = fmt.Sprintf("%s asked this request, so it is not asked again.", teamName(r.Office, w.ToTeam))
	case r.Office.Team(w.ToTeam) == nil:
		why = fmt.Sprintf("There is no team %s.", w.ToTeam)
	case !r.Takes(w.ToTeam):
		why = fmt.Sprintf("%s takes no requests now: its files have problems.", teamName(r.Office, w.ToTeam))
	}
	if why != "" {
		var may []string
		for _, n := range mayAsk(r, w.Team, q.Chain) {
			may = append(may, n.Name)
		}
		return e.refuse(ctx, w, why+mayAskOf(may))
	}
	var place *Place
	if q.Place != nil {
		place = &Place{Bridge: q.Place.Bridge, Place: q.Place.Place, Guests: q.Place.Guests}
	}
	g := given{Team: w.ToTeam, By: By{Team: asking.ID}, Heard: "team", Place: place, Chain: append(slices.Clip(q.Chain), asking.ID)}
	return e.once(ctx, w.Asked, func() error {
		return e.start(ctx, r, w.Asked, g, w.Said, nil, e.labels("request", "team", w.ToTeam, "thread", e.hash("team", top.ID, w.ToTeam), "wait", w.ID))
	})
}

// askPeople carries a wait's question to each person it asks, in their
// direct messages on each bridge that reaches people directly, each once.
// A person may be asked only when they are of the asking team or named in
// its team file; otherwise the question is refused and nobody is asked.
// A question offers at most 8 choices, each of at most 75 characters, as
// a bridge's buttons take them.
func (e *Engine) askPeople(ctx context.Context, r *Reading, w *Wait, asking *office.Team) error {
	if asking == nil {
		return e.refuse(ctx, w, "The team "+w.Team+" is no team of the office now.")
	}
	for _, p := range w.People {
		if !mayAskPerson(r.Office, asking, p) {
			return e.refuse(ctx, w, fmt.Sprintf("%s may not ask %s: a team asks its own people, and those its team file names.%s",
				asking.Name, p, mayAskOf(asking.MayAsk())))
		}
	}
	if len(w.Choices) > 8 || slices.ContainsFunc(w.Choices, func(c string) bool { return utf8.RuneCountInString(c) > 75 }) {
		return e.refuse(ctx, w, "A question offers at most 8 choices, each of at most 75 characters.")
	}
	m := Message{Text: asking.Name + " asks:\n\n" + w.Said, Choices: w.Choices, Ref: w.Ref}
	return e.direct(ctx, r, w.People, m, func(person, bridge string) (string, map[string]string) {
		return e.name(named(w.ID), "ask", w.ID, person, bridge), e.labels("out", "bridge", bridge, "wait", w.ID, "to", e.hash("person", person))
	})
}

// direct carries a message to people in their direct messages, on each
// bridge whose out reaches people directly, each once, by a run named and
// labelled for the person and the bridge.
func (e *Engine) direct(ctx context.Context, r *Reading, people []string, m Message, run func(person, bridge string) (string, map[string]string)) error {
	var errs []error
	for _, p := range people {
		person := r.Office.Person(p)
		for _, b := range r.Office.Bridges {
			to := person.Identities[b.Name]
			if to == "" || !e.carrier.Direct(b.Name) {
				continue
			}
			m := m
			m.Bridge, m.To, m.Direct = b.Name, to, true
			m.Run, m.Labels = run(p, b.Name)
			errs = append(errs, e.once(ctx, m.Run, func() error { return e.carrier.Carry(ctx, m) }))
		}
	}
	return errors.Join(errs...)
}

// waitEnded cancels the request a question to a team started, when the
// wait has ended before it: its request was stopped, or 7 days passed.
func (e *Engine) waitEnded(ctx context.Context, run *client.Run) error {
	w, err := e.wait(ctx, run.ID)
	if err != nil || w.Asked == "" {
		return err
	}
	asked, err := e.corbie.Show(ctx, w.Asked)
	switch {
	case errors.Is(err, client.ErrNoRun):
		return nil
	case err != nil || asked.HasEnded():
		return err
	}
	_, err = e.corbie.Cancel(ctx, asked.ID, 0)
	return err
}

// reply answers a wait's ticket, while it waits. A wait that is gone has
// nothing to answer.
func (e *Engine) reply(ctx context.Context, wait string, value any) error {
	tickets, err := e.corbie.Tickets(ctx, wait)
	if errors.Is(err, client.ErrNoRun) {
		return nil
	}
	if err != nil || tickets["answer"] == "" {
		return err
	}
	_, err = e.corbie.Reply(ctx, wait, tickets["answer"], value, 0)
	return err
}

// Answer answers a wait for a person: only one the wait waits on, while
// it waits, and for a decision with one of its choices, whatever its case
// and the spaces around it. thing is the id of the thing a bridge heard
// the answer as; none on the screens.
func (e *Engine) Answer(ctx context.Context, wait, person, answer, thing string) error {
	w, err := e.wait(ctx, wait)
	switch {
	case errors.Is(err, client.ErrNoRun):
		return ErrNoQuestion
	case err != nil:
		return err
	case w.Status != "waiting" || !slices.Contains(w.People, person) || !mayAskAll(e.Reading(), w):
		return ErrNoQuestion
	}
	if w.Decision != "" {
		i := slices.IndexFunc(w.Choices, func(c string) bool { return strings.EqualFold(c, strings.TrimSpace(answer)) })
		if i < 0 {
			return ErrNotAChoice
		}
		answer = w.Choices[i]
	}
	value := map[string]any{"answer": answer, "by": person}
	if thing != "" {
		value["thing"] = thing
	}
	return e.reply(ctx, wait, value)
}

// ended carries a request's answer back to where it was asked: to the
// place on a bridge it was heard at, or for a routine to the team's people
// directly; a sub-request's answers the wait that asked it. The chat's is
// on the screens. An empty answer, and a noted request, say nothing, and
// neither does a request that ended more than a day ago.
func (e *Engine) ended(ctx context.Context, r *Reading, run *client.Run) error {
	shown, err := e.corbie.Show(ctx, run.ID)
	if err != nil {
		return err
	}
	q := e.requestOf(shown)
	if q.Heard == "team" && q.Wait != "" {
		return e.reply(ctx, q.Wait, map[string]any{"answer": q.Answer})
	}
	if e.corbie.Now().Sub(shown.Ended) > letGo || q.Answer == nil || *q.Answer == "" || (q.Heard == "channel" || q.Heard == "direct") && q.Place == nil {
		return nil
	}
	switch q.Heard {
	case "channel", "direct":
		m := Message{Bridge: q.Place.Bridge, To: q.Place.Where, Text: *q.Answer, Answers: q.Place.Thing,
			Run: e.name(shown.Ended, "answer", run.ID), Labels: e.labels("out", "bridge", q.Place.Bridge, "request", run.ID)}
		return e.once(ctx, m.Run, func() error { return e.carrier.Carry(ctx, m) })
	case "routine":
		t := r.Office.Team(q.Team)
		if t == nil {
			return nil
		}
		m := Message{Text: fmt.Sprintf("%s, on %q:\n\n%s", t.Name, q.Said, *q.Answer)}
		return e.direct(ctx, r, t.People, m, func(person, bridge string) (string, map[string]string) {
			return e.name(shown.Ended, "answer", run.ID, person, bridge), e.labels("out", "bridge", bridge, "request", run.ID)
		})
	}
	return nil
}

// waits gives every wait of the office that waits on its answer and may
// be put to the people it names, oldest first, each with the request it is
// below.
func (e *Engine) waits(ctx context.Context) ([]*Wait, error) {
	r := e.Reading()
	runs, err := e.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": r.Office.ID}})
	if err != nil {
		return nil, err
	}
	parent := map[string]string{}
	for _, run := range runs {
		parent[run.ID] = run.Parent
	}
	var found []*Wait
	for _, run := range runs {
		if run.Machine != "wait" || !waits(run, "answer") {
			continue
		}
		w, err := e.wait(ctx, run.ID)
		if errors.Is(err, client.ErrNoRun) {
			continue // ended and gone since the listing
		}
		if err != nil {
			return nil, err
		}
		if !mayAskAll(r, w) {
			continue
		}
		for w.Request = run.ID; parent[w.Request] != ""; {
			w.Request = parent[w.Request]
		}
		found = append(found, w)
	}
	return found, nil
}

// Waiting gives the waits that wait on a person, oldest first.
func (e *Engine) Waiting(ctx context.Context, person string) ([]*Wait, error) {
	waits, err := e.waits(ctx)
	return slices.DeleteFunc(waits, func(w *Wait) bool { return !slices.Contains(w.People, person) }), err
}

// WaitOf gives the wait a ref names, among those that wait; nil when none
// does.
func (e *Engine) WaitOf(ctx context.Context, ref string) (*Wait, error) {
	waits, err := e.waits(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range waits {
		if w.Ref == ref {
			return w, nil
		}
	}
	return nil, nil
}

// Replied gives the wait whose question was carried to a person on a
// bridge as the message sent; nil when none was.
func (e *Engine) Replied(ctx context.Context, bridge, person, sent string) (*Wait, error) {
	carried, err := e.questions(ctx, bridge, person)
	if err != nil {
		return nil, err
	}
	for _, run := range carried {
		if run.Status != "done" {
			continue
		}
		shown, err := e.corbie.Show(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		var out struct {
			Sent string `json:"sent"`
		}
		decode(shown.Output, &out)
		if out.Sent == sent {
			return e.below(ctx, run.Labels["wait"])
		}
	}
	return nil, nil
}

// Latest gives the wait whose question was the latest carried to a person
// on a bridge before a time, whether or not it still waits; nil when none
// was. A run that carries a question starts before it sends it.
func (e *Engine) Latest(ctx context.Context, bridge, person string, before time.Time) (*Wait, error) {
	carried, err := e.questions(ctx, bridge, person)
	if err != nil {
		return nil, err
	}
	var latest *client.Run
	for _, run := range carried {
		if (run.Status == "running" || run.Status == "done") && !run.Started.After(before) && (latest == nil || run.Started.After(latest.Started)) {
			latest = run
		}
	}
	if latest == nil {
		return nil, nil
	}
	return e.below(ctx, latest.Labels["wait"])
}

// below gives a wait by its run's name, with the request it is below.
func (e *Engine) below(ctx context.Context, id string) (*Wait, error) {
	w, err := e.wait(ctx, id)
	if err != nil {
		return nil, err
	}
	top, err := e.Top(ctx, id)
	if err != nil {
		return nil, err
	}
	w.Request = top.ID
	return w, nil
}

// questions lists the runs that carried a question to a person on a
// bridge.
func (e *Engine) questions(ctx context.Context, bridge, person string) ([]*client.Run, error) {
	return e.roots(ctx, client.Filter{Labels: map[string]string{"kind": "out", "bridge": bridge, "to": e.hash("person", person)}, Finished: true})
}

// mayAskOf says whom a refused question may ask instead, after a space;
// nothing when it may ask no one.
func mayAskOf(may []string) string {
	if len(may) == 0 {
		return ""
	}
	return " It may ask " + strings.Join(may, ", ") + "."
}
