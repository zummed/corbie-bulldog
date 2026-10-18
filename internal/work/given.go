package work

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// What a run of a team's worker is given is put together here, from the
// files and the request as they are when it starts, and kept nowhere.

// given gives the three inputs of a taker's run on a request: what is new
// to it, as text; everything it is to know, as text a model reads well;
// and the same as data.
func (w *Work) given(o *office.Office, rd ready) (map[string]any, error) {
	r := rd.r
	team := o.Teams[r.Team]
	if team == nil {
		return nil, fmt.Errorf("%s is no longer a team of the office", r.Team)
	}
	asker, err := w.requests.Asks(r.Number)
	if err != nil {
		return nil, err
	}
	top, _, err := w.requests.Top(r.Number)
	if err != nil {
		return nil, err
	}
	readers, err := w.requests.ReadBy(top.Number)
	if err != nil {
		return nil, err
	}
	subs, err := w.requests.Tree(r.Number)
	if err != nil {
		return nil, err
	}
	notes, err := o.Notes(r.Team)
	if err != nil {
		return nil, fmt.Errorf("the notes of %s could not be read: %w", team.Called, err)
	}
	var open []requests.Summary
	if r.Origin.Kind == requests.ByRoutine {
		all, err := w.requests.OpenOf(r.Team)
		if err != nil {
			return nil, err
		}
		for _, s := range all {
			if s.Number != r.Number {
				open = append(open, s)
			}
		}
	}
	g := &giving{o: o, now: w.clock.Now().In(o.Zone), r: r, team: team, worker: rd.worker.Name, asker: asker, top: top, readers: readers,
		subs: subs, notes: notes, open: open, thread: rd.thread, fresh: rd.fresh}
	return map[string]any{"said": g.said(), "context": g.context(), "request": g.data()}, nil
}

// A giving is everything a run is given, read once at its start.
type giving struct {
	o       *office.Office
	now     time.Time
	r       *requests.Request
	team    *office.Team
	worker  string
	asker   requests.Party
	top     *requests.Request
	readers []requests.Party
	subs    []requests.Sub
	notes   []office.Note
	open    []requests.Summary
	thread  []requests.Entry
	fresh   []requests.Entry
}

// said gives what is new for the run: one thing as it was said, and more
// each with who said it, in the order they were said.
func (g *giving) said() string {
	if len(g.fresh) == 1 {
		return g.fresh[0].Text
	}
	var parts []string
	for _, e := range g.fresh {
		parts = append(parts, called(g.o, e.By)+": "+e.Text)
	}
	return strings.Join(parts, "\n\n")
}

// context gives everything the run is to know, as text.
func (g *giving) context() string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("It is %s, in the office's time zone, %s. Every time below is in that zone.", g.now.Format("Monday 2 January 2006, 15:04 MST"), g.o.Zone)
	line("")
	line("You are %s, a worker of %s (%s): %s", g.worker, g.team.Called, g.team.Name, orNothing(g.team.About))
	if len(g.team.People) > 0 {
		line("Its people:")
		for _, p := range g.team.People {
			line("- %s", personText(g.o, p))
		}
	} else if reached := g.o.Reached(g.team.Name); len(reached) > 0 {
		line("It has no people of its own; its decisions reach %s, who are its people.", names(g.o, reached))
	}
	if sit := colleagues(g.team, g.worker); len(sit) > 0 {
		line("The workers who sit with you, whom you use as tools: %s.", strings.Join(sit, ", "))
	}
	line("")
	if teams := askable(g.o, g.team.Name); len(teams) > 0 {
		line("The teams of the office you may ask, by name:")
		for _, t := range teams {
			line("- %s (%s): %s", t.Name, t.Called, orNothing(t.About))
		}
		line("")
	}
	if notes, left := g.shownNotes(); len(notes) > 0 {
		notesText(line, g.team.Called, notes, left)
		line("")
	}
	line("Request %d. %s", g.r.Number, g.askedText())
	line("%s", g.whereText())
	line("")
	line("The thread so far:")
	shown, left := window(g.thread, g.entryText)
	for i, e := range shown {
		line("%s", g.entryText(e))
		if i == 0 && left > 0 {
			line("(%d entries are not shown here.)", left)
		}
	}
	if len(g.open) > 0 {
		line("")
		line("The open requests of %s, newest first; you act on none of them:", g.team.Called)
		for i, s := range g.open {
			if i == OpenLimit {
				line("%d more are open.", len(g.open)-OpenLimit)
				break
			}
			line("- Request %d, asked by %s, %s: %s", s.Number, partyText(s.Asker), stateText(s.State), s.First)
		}
	}
	line("")
	line("What is new for you: %s", g.said())
	line("")
	line("%s", g.answerText())
	return b.String()
}

// askedText says who asks the request.
func (g *giving) askedText() string {
	switch g.asker.Kind {
	case requests.IsPerson:
		return fmt.Sprintf("Asked by %s, a person of the office, who is in %s.", g.asker.Called, teamsText(g.o, g.asker.Teams))
	case requests.IsGuest:
		return fmt.Sprintf("Asked by a guest, whom %s vouches for as %s, and who is no person of the office.", g.asker.Bridge, g.asker.ID)
	case requests.IsWorker:
		return fmt.Sprintf("Asked by %s, a worker of %s.", g.asker.Name, calledTeam(g.o, g.asker.Team))
	case requests.IsTeam:
		return fmt.Sprintf("Asked by %s itself, by its routine at %s.", g.asker.Called, g.r.Origin.At)
	}
	return fmt.Sprintf("Asked by %s, who is no longer in the office.", g.asker.Name)
}

// whereText says where the request was asked, and who will read its
// answer at the end.
func (g *giving) whereText() string {
	readers := partiesText(g.readers)
	if g.r.Parent != 0 {
		return fmt.Sprintf("It is a request from another team, asked at no place: your answer goes to %s, as its tool's result. "+
			"What %s says may in the end be read %s, by %s.", g.asker.Name, calledTeam(g.o, g.asker.Team), originText(g.top.Origin), readers)
	}
	return fmt.Sprintf("It was asked %s. Who will read the answer: %s.", originText(g.r.Origin), readers)
}

// answerText says where the run's answer goes.
func (g *giving) answerText() string {
	switch {
	case g.r.Parent != 0:
		return fmt.Sprintf("Your last reply is your answer, given to %s of %s.", g.asker.Name, calledTeam(g.o, g.asker.Team))
	case g.r.Origin.Kind == requests.ByRoutine:
		return fmt.Sprintf("Your last reply is your answer, said to the people of %s: %s.", g.team.Called, partiesText(g.readers))
	}
	return fmt.Sprintf("Your last reply is your answer, said to %s %s.", partyText(g.asker), originText(g.r.Origin))
}

// entryText gives an entry of the thread as a line.
func (g *giving) entryText(e requests.Entry) string {
	var to []string
	for _, w := range e.To {
		to = append(to, called(g.o, w))
	}
	head := fmt.Sprintf("[%s] %s", g.at(e.At), called(g.o, e.By))
	if len(to) > 0 {
		head += " to " + strings.Join(to, ", ")
	}
	return head + ": " + e.Text
}

// at gives a time as text, in the office's zone, naming it.
func (g *giving) at(t time.Time) string { return t.In(g.o.Zone).Format("Mon 2 Jan 2006 15:04 MST") }

// shownNotes gives the team's notes within NotesLimit characters in all,
// and a line naming what is not shown.
func (g *giving) shownNotes() ([]office.Note, string) { return shownNotes(g.notes) }

func shownNotes(notes []office.Note) ([]office.Note, string) {
	var shown []office.Note
	var left []string
	room := NotesLimit
	for _, n := range notes {
		size := utf8.RuneCountInString(n.Text)
		switch {
		case size <= room:
			shown = append(shown, n)
			room -= size
		case room > 0:
			shown = append(shown, office.Note{Name: n.Name, Text: cut(n.Text, room)})
			left = append(left, fmt.Sprintf("the last %d of the %d characters of %s", size-room, size, n.Name))
			room = 0
		default:
			left = append(left, fmt.Sprintf("all %d characters of %s", size, n.Name))
		}
	}
	if len(left) == 0 {
		return shown, ""
	}
	return shown, fmt.Sprintf("Not shown, past the %d characters of notes a run is given: %s.", NotesLimit, strings.Join(left, "; "))
}

// notesText writes a team's notes as a run is given them, each whole
// under its name, and a line naming what is not shown.
func notesText(line func(string, ...any), called string, notes []office.Note, left string) {
	line("The notes of %s:", called)
	for _, n := range notes {
		line("")
		line("## %s", n.Name)
		line("%s", strings.TrimRight(n.Text, "\n"))
	}
	if left != "" {
		line("")
		line("%s", left)
	}
}

// notesData gives a team's notes as data, each with its name.
func notesData(notes []office.Note) []any {
	var data []any
	for _, n := range notes {
		data = append(data, map[string]any{"name": n.Name, "text": n.Text})
	}
	return data
}

// data gives the same as context, as data, for a worker that is no model.
func (g *giving) data() map[string]any {
	shown, left := window(g.thread, g.entryText)
	var entries []any
	for _, e := range shown {
		var to []any
		for _, w := range e.To {
			to = append(to, whoData(g.o, w))
		}
		entries = append(entries, map[string]any{"id": e.ID, "kind": e.Kind, "by": whoData(g.o, e.By), "to": to, "time": g.dataTime(e.At),
			"text": e.Text})
	}
	var subs []any
	for _, s := range g.subs {
		subs = append(subs, subData(s))
	}
	notes, notesLeft := g.shownNotes()
	noteData := notesData(notes)
	var people []any
	for _, p := range g.team.People {
		if person := g.o.People[p]; person != nil {
			people = append(people, map[string]any{"name": p, "called": person.Called, "title": person.Title})
		}
	}
	var teams []any
	for _, t := range askable(g.o, g.team.Name) {
		teams = append(teams, map[string]any{"name": t.Name, "called": t.Called, "about": t.About})
	}
	var readers []any
	for _, p := range g.readers {
		readers = append(readers, partyData(p))
	}
	d := map[string]any{
		"number": g.r.Number, "now": g.dataTime(g.now), "zone": g.o.Zone.String(),
		"team":           map[string]any{"name": g.team.Name, "called": g.team.Called, "about": g.team.About, "people": people},
		"worker":         g.worker,
		"workers":        colleagues(g.team, g.worker),
		"teams":          teams,
		"asker":          partyData(g.asker),
		"origin":         originData(g.r, g.top),
		"readers":        readers,
		"answer_goes_to": g.answerText(),
		"entries":        entries, "entries_not_shown": left,
		"subs":  subs,
		"notes": noteData, "notes_not_shown": notesLeft,
		"said": g.said(),
	}
	if g.r.Origin.Kind == requests.ByRoutine {
		var open []any
		for i, s := range g.open {
			if i == OpenLimit {
				break
			}
			open = append(open, map[string]any{"number": s.Number, "asker": partyData(s.Asker), "state": s.State.Is,
				"waiting_on": statePeople(s.State), "first": s.First})
		}
		d["open"] = open
		d["open_not_shown"] = max(0, len(g.open)-OpenLimit)
	}
	return d
}

// dataTime gives a time as data: in the office's zone, with its offset.
func (g *giving) dataTime(t time.Time) string { return t.In(g.o.Zone).Format(time.RFC3339) }

// window gives the entries of a thread a run is given: the opening one,
// always, and the newest that fit within ThreadLimit characters, with how
// many are not shown. An opening entry larger than the limit is cut.
func window(thread []requests.Entry, text func(requests.Entry) string) ([]requests.Entry, int) {
	if len(thread) == 0 {
		return nil, 0
	}
	first := thread[0]
	room := ThreadLimit - utf8.RuneCountInString(text(first))
	if room < 0 {
		first.Text = cut(first.Text, max(0, utf8.RuneCountInString(first.Text)+room))
		room = 0
	}
	from := len(thread)
	for from > 1 {
		size := utf8.RuneCountInString(text(thread[from-1]))
		if size > room {
			break
		}
		room -= size
		from--
	}
	return append([]requests.Entry{first}, thread[from:]...), from - 1
}

// cut gives the first characters of a text, never cutting inside one.
func cut(text string, characters int) string {
	if characters <= 0 {
		return ""
	}
	i := 0
	for at := range text {
		if i == characters {
			return text[:at]
		}
		i++
	}
	return text
}

// called gives what someone an entry is by or to is called.
func called(o *office.Office, w requests.Who) string {
	switch w.Kind {
	case requests.IsBulldog:
		return "Bulldog"
	case requests.IsTeam:
		return calledTeam(o, w.Name)
	case requests.IsPerson:
		if p := o.People[w.Name]; p != nil {
			return p.Called
		}
	case requests.IsIdentity:
		if p := o.Person(w.Bridge, w.ID); p != nil {
			return p.Called
		}
		return fmt.Sprintf("a guest (%s on %s)", w.ID, w.Bridge)
	}
	return w.Name
}

func calledTeam(o *office.Office, name string) string {
	if t := o.Teams[name]; t != nil {
		return t.Called
	}
	return name
}

// whoData gives someone an entry is by or to, as data.
func whoData(o *office.Office, w requests.Who) map[string]any {
	d := map[string]any{"kind": w.Kind, "called": called(o, w)}
	switch w.Kind {
	case requests.IsIdentity:
		d["bridge"], d["id"] = w.Bridge, w.ID
		if p := o.Person(w.Bridge, w.ID); p != nil {
			d["kind"], d["name"] = requests.IsPerson, p.Name
		} else {
			d["kind"] = requests.IsGuest
		}
	case requests.IsWorker, requests.IsRule:
		d["name"], d["team"] = w.Name, w.Team
	case requests.IsPerson, requests.IsTeam:
		d["name"] = w.Name
	}
	return d
}

// partyText and partyData give someone as the office is read now.
func partyText(p requests.Party) string {
	switch p.Kind {
	case requests.IsPerson, requests.IsTeam:
		return p.Called
	case requests.IsGuest:
		return fmt.Sprintf("a guest (%s on %s)", p.ID, p.Bridge)
	}
	return p.Name
}

func partiesText(ps []requests.Party) string {
	if len(ps) == 0 {
		return "nobody"
	}
	var all []string
	for _, p := range ps {
		all = append(all, partyText(p))
	}
	return strings.Join(all, ", ")
}

func partyData(p requests.Party) map[string]any {
	d := map[string]any{"kind": p.Kind, "called": partyText(p)}
	switch p.Kind {
	case requests.IsPerson:
		d["name"], d["teams"] = p.Name, p.Teams
	case requests.IsGuest:
		d["bridge"], d["id"] = p.Bridge, p.ID
	case requests.IsWorker:
		d["name"], d["team"] = p.Name, p.Team
	default:
		d["name"] = p.Name
	}
	return d
}

// originText says where a request was asked.
func originText(o requests.Origin) string {
	switch o.Kind {
	case requests.InChat:
		return "in the built-in chat"
	case requests.AtPlace:
		return fmt.Sprintf("on %s, in %s", o.Bridge, o.Place)
	case requests.ByRoutine:
		return fmt.Sprintf("by the routine at %s", o.At)
	}
	return "at no place"
}

func originData(r, top *requests.Request) map[string]any {
	if r.Parent != 0 {
		return map[string]any{"kind": "request", "top": originData(top, top)}
	}
	d := map[string]any{"kind": r.Origin.Kind}
	switch r.Origin.Kind {
	case requests.AtPlace:
		d["bridge"], d["place"] = r.Origin.Bridge, r.Origin.Place
	case requests.ByRoutine:
		d["at"], d["to"], d["say"] = r.Origin.At, r.Origin.To, r.Origin.Say
	}
	return d
}

func subData(s requests.Sub) map[string]any {
	var below []any
	for _, b := range s.Subs {
		below = append(below, subData(b))
	}
	return map[string]any{"number": s.Number, "team": s.Team, "question": s.Question, "state": s.State.Is,
		"waiting_on": statePeople(s.State), "answer": s.Answer, "subs": below}
}

// stateText says where a request stands.
func stateText(st requests.State) string {
	if st.Is == requests.Waiting {
		return "waiting on " + partiesText(st.On)
	}
	return st.Is
}

func statePeople(st requests.State) []string {
	var on []string
	for _, p := range st.On {
		on = append(on, partyText(p))
	}
	return on
}

// personText gives a person with their title.
func personText(o *office.Office, name string) string {
	p := o.People[name]
	if p == nil {
		return name
	}
	if p.Title == "" {
		return p.Called
	}
	return p.Called + ": " + p.Title
}

func names(o *office.Office, people []string) string {
	var all []string
	for _, p := range people {
		all = append(all, personText(o, p))
	}
	return strings.Join(all, ", ")
}

func teamsText(o *office.Office, teams []string) string {
	var all []string
	for _, t := range teams {
		all = append(all, calledTeam(o, t))
	}
	if len(all) == 0 {
		return "no team"
	}
	return strings.Join(all, ", ")
}

// colleagues gives the workers who sit with a worker in its team.
func colleagues(t *office.Team, worker string) []string {
	var sit []string
	for _, w := range t.Workers {
		if w != worker {
			sit = append(sit, w)
		}
	}
	return sit
}

// askable gives the teams of the office a team's workers may ask: every
// other.
func askable(o *office.Office, team string) []*office.Team {
	var teams []*office.Team
	for _, name := range o.Order {
		if name != team {
			teams = append(teams, o.Teams[name])
		}
	}
	return teams
}

func orNothing(about string) string {
	if about == "" {
		return "it says nothing of what it looks after."
	}
	return about
}
