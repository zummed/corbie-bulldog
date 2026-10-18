package requests

import (
	"slices"
	"time"
)

// The states of a request whose origin keeps no status of its own.
const (
	Working  = "working"
	Waiting  = "waiting"
	Answered = "answered"
)

// A State is where a request stands: working, waiting on someone, whom,
// or answered.
type State struct {
	Is string
	On []Party
}

// The kinds of party a Party describes, beyond those of a Who.
const (
	IsGuest = "guest" // an identity that matches no person
	IsGone  = "gone"  // a person as signed in, who is no longer in the office
)

// A Party is someone as the office is read now: a person, by their name
// and the teams they are in; a guest, by the bridge and the identity it
// vouched for; a worker and its team; a team; a rule; or Bulldog.
type Party struct {
	Kind   string
	Name   string
	Called string
	Teams  []string
	Bridge string
	ID     string
	Team   string
}

func (l *look) party(w Who) Party {
	if p := l.personOf(w); p != "" {
		return Party{Kind: IsPerson, Name: p, Called: l.o.People[p].Called, Teams: l.o.TeamsOf(p), Bridge: w.Bridge, ID: w.ID}
	}
	switch w.Kind {
	case IsIdentity:
		return Party{Kind: IsGuest, Bridge: w.Bridge, ID: w.ID}
	case IsPerson:
		return Party{Kind: IsGone, Name: w.Name}
	case IsTeam:
		return Party{Kind: IsTeam, Name: w.Name, Called: called(l.o, w.Name)}
	}
	return Party{Kind: w.Kind, Name: w.Name, Team: w.Team}
}

// waitingOn gives everyone a request waits on: those its open entries wait
// on, and the person who is its taker while it is not answered and their
// answer is not held.
func (l *look) waitingOn(r *Request, ws []wait) []Who {
	var on []Who
	add := func(w Who) {
		if !slices.ContainsFunc(on, func(o Who) bool { return l.same(o, w) }) {
			on = append(on, w)
		}
	}
	for _, w := range ws {
		if w.request == r.Number {
			add(w.who)
		}
	}
	if taker := l.takerWaits(r); taker != "" {
		add(l.person(taker))
	}
	return on
}

// takerWaits gives the person who takes a request while it waits on them:
// it is not answered, and their answer is not held.
func (l *look) takerWaits(r *Request) string {
	if r.Answered || r.Held {
		return ""
	}
	return l.takerPerson(r)
}

// takerPerson gives the person who takes a request, when its taker is a
// person.
func (l *look) takerPerson(r *Request) string {
	t := l.teamNow(r.Team, r.TeamKnown)
	if t == nil || r.Worker != "" || l.o.People[t.Takes] == nil {
		return ""
	}
	return t.Takes
}

// answered says whether a request is answered: its taker's answer, or
// Bulldog's last entry, has been said and nothing since, and nothing in it
// waits on anyone. It is the one account of it, which every state, list
// and stop reads.
func (l *look) answered(r *Request, ws []wait) bool {
	return r.Answered && !slices.ContainsFunc(ws, func(w wait) bool { return w.request == r.Number })
}

// state gives where a request stands: waiting while anything in it waits
// on someone, answered or not.
func (l *look) state(r *Request, ws []wait) State {
	on := l.waitingOn(r, ws)
	switch {
	case len(on) == 0 && l.answered(r, ws):
		return State{Is: Answered}
	case len(on) == 0:
		return State{Is: Working}
	}
	st := State{Is: Waiting}
	for _, w := range on {
		st.On = append(st.On, l.party(w))
	}
	return st
}

// A Summary is a request as a list shows it. Of a request someone sees
// only an entry of, it holds the number alone.
type Summary struct {
	Number int64
	Whole  bool
	Team   string
	Asker  Party
	State  State
	// First is the first line of what was asked.
	First  string
	Source *Status
	Made   time.Time
	// Parent is the request it was asked from, when they see that too.
	Parent int64
}

func (l *look) summary(r *Request, ws []wait, person string) (Summary, error) {
	es, err := l.entries(r.Number)
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{Number: r.Number, Whole: true, Team: r.Team, Asker: l.party(r.Asker), State: l.state(r, ws),
		Source: r.Source, Made: r.Made}
	if len(es) > 0 {
		sum.First = firstLine(es[0].Text)
	}
	if r.Parent != 0 && person != "" {
		parent, err := l.req(r.Parent)
		if err != nil {
			return Summary{}, err
		}
		if seen, err := l.seesDone(parent, person); err != nil {
			return Summary{}, err
		} else if seen {
			sum.Parent = r.Parent
		}
	}
	return sum, nil
}

// A Filter narrows a list: to one team, to one state, to those waiting on
// the one looking, or to those they asked.
type Filter struct {
	Team      string
	State     string
	WaitsOnMe bool
	Asked     bool
}

// List gives the requests a person may see, newest first, as the filter
// narrows them.
func (s *Store) List(person string, f Filter) ([]Summary, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	rs, err := l.all()
	if err != nil {
		return nil, err
	}
	ws, err := l.waits(0)
	if err != nil {
		return nil, err
	}
	waiting := map[int64]bool{}
	if f.WaitsOnMe {
		for _, w := range l.waitsOn(rs, ws, person) {
			waiting[w.Request] = true
		}
	}
	var list []Summary
	for _, r := range slices.Backward(rs) {
		done, err := l.seesDone(r, person)
		if err != nil {
			return nil, err
		}
		if f.WaitsOnMe && !waiting[r.Number] {
			continue
		}
		if !done {
			if f.WaitsOnMe && f.Team == "" && f.State == "" && !f.Asked {
				list = append(list, Summary{Number: r.Number})
			}
			continue
		}
		sum, err := l.summary(r, ws, person)
		if err != nil {
			return nil, err
		}
		if f.Team != "" && r.Team != f.Team || f.State != "" && sum.State.Is != f.State || f.Asked && !l.is(r.Asker, person) {
			continue
		}
		list = append(list, sum)
	}
	return list, nil
}

// A Wait is one thing waiting on a person: an entry put to them that is
// open, or a request they are the taker of.
type Wait struct {
	Request int64
	// Entry is the open entry, 0 for a request they take; Kind is its kind,
	// or KindTaker.
	Entry int64
	Kind  string
	Text  string
}

// KindTaker is the kind of a Wait for a request a person takes.
const KindTaker = "taker"

// WaitingOn gives what waits on a person, oldest first: the one account
// of it, which every list, count and bridge reads.
func (s *Store) WaitingOn(person string) ([]Wait, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	rs, err := l.all()
	if err != nil {
		return nil, err
	}
	ws, err := l.waits(0)
	if err != nil {
		return nil, err
	}
	waits := l.waitsOn(rs, ws, person)
	for i, w := range waits {
		if w.Entry == 0 {
			es, err := l.entries(w.Request)
			if err != nil {
				return nil, err
			}
			waits[i].Text = es[0].Text
			continue
		}
		e, err := l.entry(w.Entry)
		if err != nil {
			return nil, err
		}
		waits[i].Kind, waits[i].Text = e.Kind, e.Text
	}
	return waits, nil
}

// waitsOn gives what waits on a person, by the request and entry alone.
func (l *look) waitsOn(rs []*Request, ws []wait, person string) []Wait {
	if l.o.People[person] == nil {
		return nil
	}
	var waits []Wait
	for _, r := range rs {
		for _, w := range ws {
			if w.request == r.Number && l.is(w.who, person) {
				waits = append(waits, Wait{Request: r.Number, Entry: w.entry})
			}
		}
		if l.takerWaits(r) == person {
			waits = append(waits, Wait{Request: r.Number, Kind: KindTaker})
		}
	}
	return waits
}

// A View is a request as one person sees it: what was done, when they see
// it, what was read of its team, when they are its person, and what is
// said to them. Of a request they see only an entry of, it holds those
// entries alone.
type View struct {
	Number int64
	Whole  bool
	Team   string
	Asker  Party
	Origin *Origin
	Parent int64
	State  State
	Source *Status
	Made   time.Time
	// Read says they see what was read of its team.
	Read    bool
	Entries []Entry
	Subs    []Sub
}

// A Sub is a sub-request as the request above it shows it: its team, its
// question, where it stands, its answer, and those beneath it.
type Sub struct {
	Number   int64
	Team     string
	Question string
	State    State
	Answer   string
	Subs     []Sub
}

// View gives a request as a person sees it. One they may not see any of is
// not there.
func (s *Store) View(n int64, person string) (*View, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	r, err := l.req(n)
	if err != nil {
		return nil, err
	}
	done, err := l.seesDone(r, person)
	if err != nil {
		return nil, err
	}
	es, err := l.entries(n)
	if err != nil {
		return nil, err
	}
	puts, err := l.puts(n)
	if err != nil {
		return nil, err
	}
	v := &View{Number: n, Whole: done, Entries: l.shown(es, puts, done, person)}
	if !done {
		if len(v.Entries) == 0 {
			return nil, ErrNotThere
		}
		return v, nil
	}
	ws, err := l.waits(n)
	if err != nil {
		return nil, err
	}
	sum, err := l.summary(r, ws, person)
	if err != nil {
		return nil, err
	}
	v.Team, v.Asker, v.State, v.Source, v.Made, v.Parent = sum.Team, sum.Asker, sum.State, sum.Source, sum.Made, sum.Parent
	if r.Parent == 0 {
		origin := r.Origin
		v.Origin = &origin
	}
	v.Read = l.seesRead(r.Team, r.TeamKnown, person)
	if v.Subs, err = l.tree(n); err != nil {
		return nil, err
	}
	return v, nil
}

// tree gives the sub-requests beneath a request, as far down as they go.
func (l *look) tree(n int64) ([]Sub, error) {
	subs, err := l.subs(n)
	if err != nil {
		return nil, err
	}
	var tree []Sub
	for _, r := range subs {
		es, err := l.entries(r.Number)
		if err != nil {
			return nil, err
		}
		ws, err := l.waits(r.Number)
		if err != nil {
			return nil, err
		}
		sub := Sub{Number: r.Number, Team: r.Team, State: l.state(r, ws)}
		sub.Question = es[0].Text
		if answer := l.answerOf(r, es); answer != nil {
			sub.Answer = answer.Text
		}
		if sub.Subs, err = l.tree(r.Number); err != nil {
			return nil, err
		}
		tree = append(tree, sub)
	}
	return tree, nil
}

// answerOf gives the latest answer in a sub-request's thread, of what was
// done: its taker's, or Bulldog's entry to its asker saying why there is
// none. It is nil while there is none.
func (l *look) answerOf(r *Request, es []Entry) *Entry {
	for i, e := range slices.Backward(es) {
		if e.Read == "" && (e.Kind == KindAnswer || e.Kind == KindBulldog && slices.ContainsFunc(e.To, func(w Who) bool { return l.same(w, r.Asker) })) {
			return &es[i]
		}
	}
	return nil
}

// A Delivery is where an entry is carried: to the request's origin, or to
// a person directly, on each bridge that knows them and on the screens.
type Delivery struct {
	Origin *Origin
	Person string
}

// Deliveries gives where an entry is carried, judged as the office is read
// now: what was done that is said to the asker goes to the origin, or for
// a routine to its team's people; what is said to a person goes to them
// directly; and what Bulldog says to a team goes to its people.
// Nothing goes back to whoever said it, and a sub-request's asker is
// answered by the run that asked, never over a bridge.
func (s *Store) Deliveries(entry int64) ([]Delivery, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	e, err := l.entry(entry)
	if err != nil {
		return nil, err
	}
	r, err := l.req(e.Request)
	if err != nil {
		return nil, err
	}
	ws, err := l.waits(e.Request)
	if err != nil {
		return nil, err
	}
	return l.deliveries(r, ws, e), nil
}

// DeliveriesIn gives a request's thread, and where each of its entries is
// carried, by the entry, as Deliveries gives it, in one reading of the
// request.
func (s *Store) DeliveriesIn(n int64) ([]Entry, map[int64][]Delivery, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, nil, err
	}
	r, err := l.req(n)
	if err != nil {
		return nil, nil, err
	}
	thread, err := l.entries(n)
	if err != nil {
		return nil, nil, err
	}
	ws, err := l.waits(n)
	if err != nil {
		return nil, nil, err
	}
	out := map[int64][]Delivery{}
	for _, e := range thread {
		out[e.ID] = l.deliveries(r, ws, e)
	}
	return thread, out, nil
}

// deliveries gives where an entry of a request is carried, with what
// waits in the request.
func (l *look) deliveries(r *Request, ws []wait, e Entry) []Delivery {
	var ds []Delivery
	var persons []string
	origin := r.Parent == 0 && e.Read == "" && slices.ContainsFunc(e.To, func(w Who) bool { return l.same(w, r.Asker) })
	switch {
	case origin && r.Origin.Kind == ByRoutine:
		if l.teamNow(r.Origin.Team, r.Origin.Known) != nil {
			persons = append(persons, people(l.o, r.Origin.Team)...)
		}
	case origin:
		o := r.Origin
		ds = append(ds, Delivery{Origin: &o})
	}
	by := l.personOf(e.By)
	for _, w := range e.To {
		switch {
		case origin && l.same(w, r.Asker):
		case w.Kind == IsTeam && e.By == Bulldog:
			if l.teamNow(w.Name, w.Known) != nil {
				persons = append(persons, people(l.o, w.Name)...)
			}
		default:
			persons = append(persons, l.personOf(w))
		}
	}
	for _, w := range ws {
		if w.entry == e.ID && !(origin && l.same(w.who, r.Asker)) {
			persons = append(persons, l.personOf(w.who))
		}
	}
	for _, p := range persons {
		if p == "" || p == by || slices.ContainsFunc(ds, func(d Delivery) bool { return d.Person == p }) {
			continue
		}
		ds = append(ds, Delivery{Person: p})
	}
	return ds
}
