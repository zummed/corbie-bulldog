package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/client"
)

// timedOut is the note wait.yaml ends with when nobody answers it.
const timedOut = "No answer came within 7 days."

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
	Refused  string    // why Bulldog refused the ask
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

// asked acts on a wait that waits on its answer: a question to a team, or
// to people. The asking team is the run's team label, which it carries
// from its request.
func (e *Engine) asked(ctx context.Context, r *Reading, run *client.Run) error {
	w, err := e.wait(ctx, run.ID)
	if err != nil {
		return err
	}
	asking := r.Office.Team(w.Team)
	if w.ToTeam != "" {
		return e.askTeam(ctx, r, w, asking)
	}
	return e.askPeople(ctx, r, w, asking)
}

// askTeam starts a request of the team a wait asks, named from the wait,
// or refuses the ask: to the asking team itself, to a team of its chain,
// and to a team that takes no requests.
func (e *Engine) askTeam(ctx context.Context, r *Reading, w *Wait, asking *office.Team) error {
	top, err := e.top(ctx, w.ID)
	if err != nil {
		return err
	}
	q := e.requestOf(top)
	why := ""
	switch {
	case asking == nil:
		why = fmt.Sprintf("the team %s is no team of the office now", w.Team)
	case w.ToTeam == asking.ID:
		why = fmt.Sprintf("%s cannot ask itself", asking.Name)
	case slices.Contains(q.Chain, w.ToTeam):
		why = fmt.Sprintf("the team %s asked this request, so it is not asked again", w.ToTeam)
	case r.Office.Team(w.ToTeam) == nil:
		why = fmt.Sprintf("there is no team %s", w.ToTeam)
	case !r.takes(w.ToTeam):
		why = fmt.Sprintf("the team %s takes no requests now: its files have problems", w.ToTeam)
	}
	if why != "" {
		var may []string
		for _, n := range mayAsk(r, w.Team, q.Chain) {
			may = append(may, n.Name)
		}
		return e.reply(ctx, w.ID, map[string]any{"answer": nil, "refused": capital(why) + ". " + mayAskOf(may, "other team")})
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
// its team file; otherwise the ask is refused and nobody is asked.
func (e *Engine) askPeople(ctx context.Context, r *Reading, w *Wait, asking *office.Team) error {
	if asking == nil {
		return e.reply(ctx, w.ID, map[string]any{"answer": nil, "refused": "The team " + w.Team + " is no team of the office now."})
	}
	may := asking.MayAsk()
	for _, p := range w.People {
		if !slices.Contains(may, p) || r.Office.Person(p) == nil {
			return e.reply(ctx, w.ID, map[string]any{"answer": nil, "refused": fmt.Sprintf(
				"%s may not ask %s: a team asks its own people, and those its team file names. %s", asking.Name, p, mayAskOf(may, "person"))})
		}
	}
	var errs []error
	for _, p := range w.People {
		person := r.Office.Person(p)
		for _, b := range r.Office.Bridges {
			to := person.Identities[b.Name]
			if to == "" || !e.carrier.Direct(b.Name) {
				continue
			}
			m := Message{Bridge: b.Name, To: to, Direct: true, Text: asking.Name + " asks:\n\n" + w.Said, Choices: w.Choices, Ref: w.Ref,
				Run: e.name(named(w.ID), "ask", w.ID, p, b.Name), Labels: e.labels("out", "bridge", b.Name, "wait", w.ID, "to", e.hash("person", p))}
			errs = append(errs, e.once(ctx, m.Run, func() error { return e.carrier.Carry(ctx, m) }))
		}
	}
	return errors.Join(errs...)
}

// waitEnded cancels the request a question to a team started, when the
// wait has ended before it: its request was stopped.
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

// reply answers a wait's ticket, while it waits.
func (e *Engine) reply(ctx context.Context, wait string, value any) error {
	tickets, err := e.corbie.Tickets(ctx, wait)
	if err != nil || tickets["answer"] == "" {
		return err
	}
	_, err = e.corbie.Reply(ctx, wait, tickets["answer"], value, 0)
	return err
}

// Answer answers a wait for a person: only one the wait waits on, while
// it waits, and for a decision with one of its choices. thing is the id
// of the thing a bridge heard the answer as; none on the screens.
func (e *Engine) Answer(ctx context.Context, wait, person, answer, thing string) error {
	w, err := e.wait(ctx, wait)
	switch {
	case errors.Is(err, client.ErrNoRun):
		return ErrNoQuestion
	case err != nil:
		return err
	case w.Status != "waiting" || !slices.Contains(w.People, person):
		return ErrNoQuestion
	case w.Decision != "" && !slices.Contains(w.Choices, answer):
		return ErrNotAChoice
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
// on the screens. An empty answer, and a noted request, say nothing.
func (e *Engine) ended(ctx context.Context, r *Reading, run *client.Run) error {
	shown, err := e.corbie.Show(ctx, run.ID)
	if err != nil {
		return err
	}
	q := e.requestOf(shown)
	if q.Heard == "team" && q.Wait != "" {
		return e.reply(ctx, q.Wait, map[string]any{"answer": q.Answer})
	}
	if q.Answer == nil || *q.Answer == "" || (q.Heard == "channel" || q.Heard == "direct") && q.Place == nil {
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
		var errs []error
		for _, p := range t.People {
			person := r.Office.Person(p)
			for _, b := range r.Office.Bridges {
				to := person.Identities[b.Name]
				if to == "" || !e.carrier.Direct(b.Name) {
					continue
				}
				m := Message{Bridge: b.Name, To: to, Direct: true, Text: fmt.Sprintf("%s, on %q:\n\n%s", t.Name, q.Said, *q.Answer),
					Run: e.name(shown.Ended, "answer", run.ID, p, b.Name), Labels: e.labels("out", "bridge", b.Name, "request", run.ID)}
				errs = append(errs, e.once(ctx, m.Run, func() error { return e.carrier.Carry(ctx, m) }))
			}
		}
		return errors.Join(errs...)
	}
	return nil
}

// Waits gives every wait of the office that waits on its answer, oldest
// first, each with the request it is below.
func (e *Engine) Waits(ctx context.Context) ([]*Wait, error) {
	runs, err := e.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": e.Reading().Office.ID}})
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
		for w.Request = run.ID; parent[w.Request] != ""; {
			w.Request = parent[w.Request]
		}
		found = append(found, w)
	}
	return found, nil
}

// Waiting gives the waits that wait on a person, oldest first.
func (e *Engine) Waiting(ctx context.Context, person string) ([]*Wait, error) {
	waits, err := e.Waits(ctx)
	return slices.DeleteFunc(waits, func(w *Wait) bool { return !slices.Contains(w.People, person) }), err
}

// WaitOf gives the wait a ref names, among those that wait; nil when none
// does.
func (e *Engine) WaitOf(ctx context.Context, ref string) (*Wait, error) {
	waits, err := e.Waits(ctx)
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
	carried, err := e.carried(ctx, bridge, person)
	if err != nil {
		return nil, err
	}
	for _, run := range carried {
		var out struct {
			Sent string `json:"sent"`
		}
		decode(run.Output, &out)
		if out.Sent == sent {
			return e.below(ctx, run.Labels["wait"])
		}
	}
	return nil, nil
}

// Latest gives the wait whose question was the latest carried to a person
// on a bridge before a time, whether or not it still waits; nil when none
// was.
func (e *Engine) Latest(ctx context.Context, bridge, person string, before time.Time) (*Wait, error) {
	carried, err := e.carried(ctx, bridge, person)
	if err != nil {
		return nil, err
	}
	var latest *client.Run
	for _, run := range carried {
		if !run.Ended.After(before) && (latest == nil || run.Ended.After(latest.Ended)) {
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
	top, err := e.top(ctx, id)
	if err != nil {
		return nil, err
	}
	w.Request = top.ID
	return w, nil
}

// carried gives the runs that carried a question to a person on a bridge
// and sent it, as shown.
func (e *Engine) carried(ctx context.Context, bridge, person string) ([]*client.Run, error) {
	runs, err := e.roots(ctx, client.Filter{Labels: map[string]string{"kind": "out", "bridge": bridge, "to": e.hash("person", person)}, Status: "done", Finished: true})
	if err != nil {
		return nil, err
	}
	var shown []*client.Run
	for _, run := range runs {
		s, err := e.corbie.Show(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		shown = append(shown, s)
	}
	return shown, nil
}

// mayAskOf says whom a refused ask may ask instead.
func mayAskOf(may []string, what string) string {
	if len(may) == 0 {
		return "It may ask no " + what + "."
	}
	return "It may ask " + strings.Join(may, ", ") + "."
}

// capital gives a sentence with its first letter a capital.
func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
