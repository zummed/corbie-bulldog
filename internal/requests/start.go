package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/instance"
	"github.com/zummed/corbie/machine"
)

// threadLimit is how many characters of its thread a request is given:
// the newest requests whose said and answer fit.
const threadLimit = 20_000

// By is who asked a request. One field is set.
type By struct {
	Person  string `json:"person,omitzero"`
	Guest   string `json:"guest,omitzero"`   // the identity the bridge gave a guest
	Team    string `json:"team,omitzero"`    // the team whose worker asked with ask_team
	Routine string `json:"routine,omitzero"` // the routine's at
}

// A Place is where a bridge heard a request: the bridge, the listed place
// or direct, whether guests may ask there, where it was said, and the id
// of the thing heard, which the answer replies to. A sub-request is given
// the place its chain began at, without where and the thing.
type Place struct {
	Bridge string `json:"bridge"`
	Place  string `json:"place"`
	Guests bool   `json:"guests"`
	Where  string `json:"where,omitzero"`
	Thing  string `json:"thing,omitzero"`
}

// A File is a file said with a request, as it is given to Ask.
type File struct {
	Name string
	// Data is the file's contents: a client.InputFile, or a blob of
	// another of Bulldog's runs as that run shows it, with "run" added.
	Data any
}

// given is what a request's run is given as request: what Bulldog knows of
// it, as data.
type given struct {
	Team      string    `json:"team"`
	By        By        `json:"by"`
	Heard     string    `json:"heard"`
	Place     *Place    `json:"place"`
	Thread    []earlier `json:"thread"`
	Teams     []askable `json:"teams"`
	Open      []earlier `json:"open"`
	Chain     []string  `json:"chain"`
	Docs      string    `json:"docs"`
	Mentioned bool      `json:"mentioned"`
}

// earlier is a request of the thread before this one, with what it
// answered, or a running request of the team.
type earlier struct {
	By     By        `json:"by"`
	Said   string    `json:"said"`
	Answer *string   `json:"answer,omitzero"`
	At     time.Time `json:"at"`
}

// askable is a team a request may ask: its name, and what it looks after.
type askable struct {
	Name  string `json:"name"`
	About string `json:"about"`
}

// An Ask is something a person asks of a team, or a guest at a place that
// lets guests ask: heard on a bridge, or said in the chat on the screens.
type Ask struct {
	Team  string
	By    By // a person, or a guest
	Said  string
	Files []File
	// Place is where a bridge heard it, its Thing the id of the thing
	// heard; nil for the chat.
	Place *Place
	// At is when the bridge's service says it was said.
	At        time.Time
	Mentioned bool
	// After is, in the chat, the request it says more after: it is a
	// request of the same team, in that request's thread.
	After string
}

// Ask starts a request, and gives its run's name. One heard on a bridge is
// named from the thing heard, and is started only when no run has the
// name; the name is given either way.
func (e *Engine) Ask(ctx context.Context, a Ask) (string, error) {
	r := e.Reading()
	g := given{Team: a.Team, By: a.By, Heard: "chat", Place: a.Place, Mentioned: a.Mentioned}
	var name, thread string
	var err error
	switch {
	case a.Place != nil:
		g.Heard = "channel"
		if a.Place.Place == "direct" {
			g.Heard = "direct"
		}
		name = e.name(a.At, "thing", a.Place.Bridge, a.Place.Thing)
		thread = e.hash("where", a.Place.Bridge, a.Place.Where)
	case a.After != "":
		var before *client.Run
		if before, err = e.request(ctx, a.After); err != nil {
			return "", err
		}
		g.Team, thread = before.Labels["team"], before.Labels["thread"]
		name, err = e.corbie.NewRun()
	default:
		name, err = e.corbie.NewRun()
		thread = e.hash("chat", name)
	}
	if err != nil {
		return "", err
	}
	return name, e.once(ctx, name, func() error {
		return e.start(ctx, r, name, g, a.Said, a.Files, e.labels("request", "team", g.Team, "thread", thread))
	})
}

// start starts a request of a team under a name, given what Bulldog knows
// of it. It gives the request its thread, the teams it may ask, the docs'
// place, and for a routine the team's running requests.
func (e *Engine) start(ctx context.Context, r *Reading, name string, g given, said string, files []File, labels map[string]string) error {
	t := r.Office.Team(g.Team)
	if !r.takes(g.Team) {
		return fmt.Errorf("%s takes no requests: it is no team of the office, or its files have problems", g.Team)
	}
	thread, left, err := e.thread(ctx, labels["thread"])
	if err != nil {
		return err
	}
	g.Thread, g.Teams, g.Docs, g.Open = thread, mayAsk(r, g.Team, g.Chain), r.Office.Docs(g.Team), []earlier{}
	if g.Chain == nil {
		g.Chain = []string{}
	}
	if g.Heard == "routine" {
		if g.Open, err = e.open(ctx, g.Team); err != nil {
			return err
		}
	}
	given := []any{}
	for _, f := range files {
		given = append(given, map[string]any{"name": f.Name, "data": f.Data})
	}
	m := r.Machines[g.Team]
	_, err = e.corbie.Start(ctx, m.Draft, client.Start{Run: name, Allow: m.Allow, Labels: labels, Input: map[string]any{
		"said": said, "context": contextOf(r, t, g, left, e.corbie.Now()), "request": g, "files": given}})
	return err
}

// thread gives the requests of a thread, oldest first, the newest within
// threadLimit characters, and how many are left out.
func (e *Engine) thread(ctx context.Context, label string) ([]earlier, int, error) {
	runs, err := e.roots(ctx, client.Filter{Labels: map[string]string{"kind": "request", "thread": label}, Finished: true})
	if err != nil {
		return nil, 0, err
	}
	all := []earlier{}
	for _, run := range runs {
		q, err := e.shown(ctx, run.ID)
		if err != nil {
			return nil, 0, err
		}
		all = append(all, earlier{By: q.By, Said: q.Said, Answer: q.Answer, At: named(run.ID)})
	}
	slices.SortStableFunc(all, func(a, b earlier) int { return a.At.Compare(b.At) })
	from, size := len(all), 0
	for ; from > 0; from-- {
		size += utf8.RuneCountInString(all[from-1].Said)
		if all[from-1].Answer != nil {
			size += utf8.RuneCountInString(*all[from-1].Answer)
		}
		if size > threadLimit {
			break
		}
	}
	return all[from:], from, nil
}

// open gives a team's running requests, oldest first.
func (e *Engine) open(ctx context.Context, team string) ([]earlier, error) {
	runs, err := e.roots(ctx, client.Filter{Labels: map[string]string{"kind": "request", "team": team}})
	if err != nil {
		return nil, err
	}
	open := []earlier{}
	for _, run := range runs {
		q, err := e.shown(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		open = append(open, earlier{By: q.By, Said: q.Said, At: named(run.ID)})
	}
	slices.SortStableFunc(open, func(a, b earlier) int { return a.At.Compare(b.At) })
	return open, nil
}

// mayAsk gives the teams a request of a team may ask: every team that
// takes requests, but itself and those of its chain.
func mayAsk(r *Reading, team string, chain []string) []askable {
	may := []askable{}
	for _, t := range r.Office.Teams {
		if t.ID != team && !slices.Contains(chain, t.ID) && r.takes(t.ID) {
			may = append(may, askable{Name: t.ID, About: t.About})
		}
	}
	return may
}

// contextOf writes what a request is given as data as text a model reads
// well, with the time in the office's zone and the team's people and
// workers.
func contextOf(r *Reading, t *office.Team, g given, left int, now time.Time) string {
	o := r.Office
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	when := func(at time.Time) string { return at.In(o.Zone).Format("Monday 2 January 2006, 15:04") }
	line("It is %s, in %s.", when(now), o.Zone)
	line("You work for %s (%s): %s", t.Name, t.ID, t.About)
	var people, workers []string
	for _, p := range t.People {
		people = append(people, personName(o, p))
	}
	for _, w := range t.Workers {
		workers = append(workers, w.Name)
	}
	line("Its people: %s. Its workers: %s.", listed(people), listed(workers))
	line("%s", askedBy(o, g))
	if len(g.Thread) > 0 || left > 0 {
		line("")
		line("Before this, in the same thread (%d earlier left out):", left)
		for _, q := range g.Thread {
			line("- %s, %s: %q", byName(o, q.By), when(q.At), q.Said)
			if q.Answer != nil {
				line("  Answered: %q", *q.Answer)
			}
		}
	}
	line("")
	if len(g.Teams) == 0 {
		line("You may ask no other team.")
	} else {
		line("You may ask these teams with ask_team:")
		for _, n := range g.Teams {
			line("- %s: %s", n.Name, n.About)
		}
	}
	if g.Heard == "routine" {
		line("")
		line("Requests of %s still open: %d.", t.Name, len(g.Open))
		for _, q := range g.Open {
			line("- %s, %s: %q", byName(o, q.By), when(q.At), q.Said)
		}
	}
	return b.String()
}

// askedBy says who asked a request, and where.
func askedBy(o *office.Office, g given) string {
	who := byName(o, g.By)
	if p := o.Person(g.By.Person); p != nil {
		var teams []string
		for _, t := range o.Teams {
			if slices.Contains(t.People, p.ID) {
				teams = append(teams, t.Name)
			}
		}
		who += ", of " + listed(teams)
	}
	switch g.Heard {
	case "channel":
		return fmt.Sprintf("%s asked, at %s on %s.", who, g.Place.Place, g.Place.Bridge)
	case "direct":
		return fmt.Sprintf("%s asked, in a direct message on %s.", who, g.Place.Bridge)
	case "chat":
		return fmt.Sprintf("%s asked, on Bulldog's screens.", who)
	case "team":
		return fmt.Sprintf("%s asked, with ask_team. The teams whose requests asked this one: %s.", who, strings.Join(g.Chain, ", "))
	}
	return who + " asked."
}

// byName names who asked as a person reads it.
func byName(o *office.Office, by By) string {
	switch {
	case by.Person != "":
		return personName(o, by.Person)
	case by.Guest != "":
		return "A guest, " + by.Guest
	case by.Team != "":
		if t := o.Team(by.Team); t != nil {
			return fmt.Sprintf("%s (%s)", t.Name, t.ID)
		}
		return by.Team
	}
	return fmt.Sprintf("The routine at %s", by.Routine)
}

// personName names a person by their name and their key.
func personName(o *office.Office, id string) string {
	if p := o.Person(id); p != nil && p.Name != id {
		return fmt.Sprintf("%s (%s)", p.Name, id)
	}
	return id
}

// listed writes names in a line, or says there are none.
func listed(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// came acts on a routine's clock. When the clock waits on came, Bulldog
// answers it, and the step that takes the answer starts the routine's
// request, named from the clock and that step, whose time is when the
// answer was taken. So each time the clock changes, the request of the
// last came it took is started, unless a run of its name is there. A
// clock of a routine the office no longer has is left, for keepClocks to
// cancel.
func (e *Engine) came(ctx context.Context, r *Reading, clock *client.Run) error {
	t, routine := e.routine(r, clock.Labels["made"])
	if routine == nil || !r.takes(t.ID) {
		return nil
	}
	var last *client.Entry
	_, err := e.corbie.Record(ctx, clock.ID, func(entry client.Entry) {
		if slices.ContainsFunc(entry.Events, func(a instance.Arrived) bool { return a.Name == "came" && a.Answer }) {
			last = &entry
		}
	})
	if err != nil {
		return err
	}
	if last != nil {
		name := e.name(last.Time, "came", clock.ID, strconv.Itoa(last.Step))
		g := given{Team: t.ID, By: By{Routine: routine.At}, Heard: "routine"}
		err := e.once(ctx, name, func() error {
			return e.start(ctx, r, name, g, routine.Say, nil, e.labels("request", "team", t.ID, "thread", e.hash("routine", name)))
		})
		if err != nil {
			return err
		}
	}
	if !waits(clock, "came") {
		return nil
	}
	tickets, err := e.corbie.Tickets(ctx, clock.ID)
	if err != nil || tickets["came"] == "" {
		return err
	}
	_, err = e.corbie.Reply(ctx, clock.ID, tickets["came"], true, 0)
	return err
}

// made gives the fingerprint of a routine of a team, with the office's
// zone: a clock labelled with it keeps the routine.
func (e *Engine) made(r *Reading, t *office.Team, routine office.Routine) string {
	return e.hash("clock", t.ID, routine.At, r.Office.Zone.String(), routine.Say)
}

// routine gives the routine of the office a clock's fingerprint names,
// with its team; nil when there is none.
func (e *Engine) routine(r *Reading, made string) (*office.Team, *office.Routine) {
	for _, t := range r.Office.Teams {
		for i := range t.Routines {
			if e.made(r, t, t.Routines[i]) == made {
				return t, &t.Routines[i]
			}
		}
	}
	return nil, nil
}

// keepClocks keeps one clock for each routine of every team that takes
// requests: it starts the clocks that are missing, and cancels those of a
// routine the office no longer has, or has changed, and any second clock
// of one routine.
func (e *Engine) keepClocks(ctx context.Context) error {
	r := e.Reading()
	wanted := map[string]*office.Team{}
	var order []string
	for _, t := range r.Office.Teams {
		for _, routine := range t.Routines {
			if made := e.made(r, t, routine); r.takes(t.ID) && wanted[made] == nil {
				wanted[made] = t
				order = append(order, made)
			}
		}
	}
	running, err := e.roots(ctx, client.Filter{Labels: map[string]string{"kind": "clock"}})
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	// A clock started and not yet taken up by Corbie is in no listing.
	for _, clock := range running {
		delete(e.starting, clock.ID)
	}
	for name, made := range e.starting {
		shown, err := e.corbie.Show(ctx, name)
		switch {
		case err == nil && shown.Status == "waiting":
			kept[made] = true
		case err == nil || errors.Is(err, client.ErrNoRun):
			delete(e.starting, name)
		default:
			return err
		}
	}
	var errs []error
	for _, clock := range running {
		if made := clock.Labels["made"]; wanted[made] != nil && !kept[made] {
			kept[made] = true
			continue
		}
		_, err := e.corbie.Cancel(ctx, clock.ID, 0)
		errs = append(errs, err)
	}
	for _, made := range order {
		if !kept[made] {
			errs = append(errs, e.startClock(ctx, r, made))
		}
	}
	return errors.Join(errs...)
}

// startClock starts the shipped clock for a routine.
func (e *Engine) startClock(ctx context.Context, r *Reading, made string) error {
	t, routine := e.routine(r, made)
	src, err := r.Office.Load(".bulldog/clock/clock.yaml")
	if err != nil {
		return err
	}
	draft, err := machine.Parse(src, r.Office.Load)
	if err != nil {
		return err
	}
	name, err := e.corbie.Start(ctx, draft, client.Start{Allow: r.Machines[t.ID].Allow,
		Input:  map[string]any{"at": routine.At + " " + r.Office.Zone.String()},
		Labels: e.labels("clock", "team", t.ID, "made", made)})
	if err == nil {
		e.starting[name] = made
	}
	return err
}
