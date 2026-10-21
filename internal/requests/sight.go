package requests

import (
	"database/sql"
	"errors"
	"slices"
	"strconv"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// seesDone says whether a person sees what was done of a request: its
// asker, the people of its team, the people who can read its origin, and,
// for a sub-request, whoever sees the request it was asked from.
func (l *look) seesDone(r *Request, person string) (bool, error) {
	if person == "" || l.o.People[person] == nil {
		return false, nil
	}
	for {
		if l.is(r.Asker, person) || l.isOf(r.Team, r.TeamKnown, person) {
			return true, nil
		}
		if r.Parent == 0 {
			return l.readsOrigin(r, person)
		}
		parent, err := l.req(r.Parent)
		if err != nil {
			return false, err
		}
		r = parent
	}
}

// readsOrigin says whether a person can read a request's origin.
func (l *look) readsOrigin(r *Request, person string) (bool, error) {
	readers, err := l.originReaders(r)
	return slices.ContainsFunc(readers, func(w Who) bool { return l.is(w, person) }), err
}

// originReaders gives who reads a request's origin, as the office is read
// now: on a bridge, the people whose identities it last reported as
// reading the place, each once, or the asker alone where it never
// reported any; the chat, the asker alone; a routine, the people of its
// team. A sub-request has no origin, and none.
func (l *look) originReaders(r *Request) ([]Who, error) {
	var readers []Who
	switch r.Origin.Kind {
	case AtPlace:
		ids, reported, err := l.readersOf(r.Origin.Bridge, r.Origin.Place)
		if err != nil {
			return nil, err
		}
		if !reported {
			return []Who{r.Asker}, nil
		}
		for _, id := range ids {
			w := Identity(r.Origin.Bridge, id)
			if p := l.personOf(w); p != "" && !slices.ContainsFunc(readers, func(o Who) bool { return l.is(o, p) }) {
				readers = append(readers, w)
			}
		}
	case InChat:
		readers = []Who{r.Asker}
	case ByRoutine:
		if l.teamNow(r.Origin.Team, r.Origin.Known) != nil {
			for _, p := range people(l.o, r.Origin.Team) {
				readers = append(readers, l.person(p))
			}
		}
	}
	return readers, nil
}

// readersAt gives every identity that reads a request asked at a place,
// a person of the office or not: those its bridge last reported as
// reading the place, or the asker alone where it never reported any.
func (l *look) readersAt(r *Request) ([]Who, error) {
	ids, reported, err := l.readersOf(r.Origin.Bridge, r.Origin.Place)
	if err != nil || !reported {
		return []Who{r.Asker}, err
	}
	var readers []Who
	for _, id := range ids {
		readers = append(readers, Identity(r.Origin.Bridge, id))
	}
	return readers, nil
}

// leaves says whether what a request's team says in it to someone is read
// by anyone outside the team. What is said to the asker of a request
// asked at a place is read by everyone who reads the place, an identity
// the office names as no person among them, who is outside every team,
// and by any guest at all where guests may read it unreported; anything
// else is read by whom it is said to alone.
func (l *look) leaves(r *Request, to Who) (bool, error) {
	readers := []Who{to}
	if r.Parent == 0 && r.Origin.Kind == AtPlace && l.same(to, r.Asker) {
		_, anyone, err := l.guestsAt(r)
		if err != nil || anyone {
			return anyone, err
		}
		at, err := l.readersAt(r)
		if err != nil {
			return false, err
		}
		readers = append(readers, at...)
	}
	return slices.ContainsFunc(readers, func(w Who) bool { return !l.inside(r.Team, r.TeamKnown, w) }), nil
}

// guestsAt gives how many of the identities a bridge last reported as
// reading a request's place the office names as no person, and whether
// any guest at all may read it: where the bridge never reported who
// reads a place guests may ask in, but its direct place, which only the
// asker reads.
func (l *look) guestsAt(r *Request) (int, bool, error) {
	if r.Origin.Kind != AtPlace {
		return 0, false, nil
	}
	ids, reported, err := l.readersOf(r.Origin.Bridge, r.Origin.Place)
	if err != nil {
		return 0, false, err
	}
	if !reported {
		p, _ := l.o.Place(r.Origin.Bridge, r.Origin.Heard)
		return 0, p.Guests && r.Origin.Heard != office.Direct, nil
	}
	guests := 0
	for _, id := range ids {
		if l.personOf(Identity(r.Origin.Bridge, id)) == "" {
			guests++
		}
	}
	return guests, false, nil
}

// Leaves says whether what a request's team says in it to someone is read
// by anyone outside the team, which is what its disclosure check, where
// it sets one, is asked of.
func (s *Store) Leaves(n int64, to Who) (bool, error) {
	l, err := s.look(s.db)
	if err != nil {
		return false, err
	}
	r, err := l.req(n)
	if err != nil {
		return false, err
	}
	return l.leaves(r, to)
}

// readsAll says whether every reader of a request asked at a place reads
// another: one the office does not name is the same reader only as the
// same identity.
func (l *look) readsAll(r, other *Request) (bool, error) {
	readers, err := l.readersAt(r)
	if err != nil {
		return false, err
	}
	theirs, err := l.readersAt(other)
	if err != nil {
		return false, err
	}
	for _, w := range readers {
		if !slices.ContainsFunc(theirs, func(o Who) bool { return l.same(w, o) }) {
			return false, nil
		}
	}
	return true, nil
}

// seesRead says whether a person sees what was read of a team known so:
// only its people do, and of a team that has gone, nobody.
func (l *look) seesRead(team string, known int64, person string) bool {
	return l.o.People[person] != nil && l.isOf(team, known, person)
}

// toThem says whether an entry is said to a person, or was put to them.
// What was read of a team that has gone is said to nobody.
func (l *look) toThem(e Entry, puts []wait, person string) bool {
	if person == "" || e.Read != "" && l.teamNow(e.Read, e.ReadKnown) == nil {
		return false
	}
	for _, w := range e.To {
		if l.is(w, person) {
			return true
		}
	}
	for _, w := range puts {
		if w.entry == e.ID && l.is(w.who, person) {
			return true
		}
	}
	return false
}

// shown gives the entries of a thread a person sees: what was done when
// they see it, what was read of each team whose person they are, and what
// is said or was put to them with what answers it.
func (l *look) shown(es []Entry, puts []wait, done bool, person string) []Entry {
	var shown []Entry
	to := map[int64]bool{}
	for _, e := range es {
		switch {
		case e.Read == "" && done,
			e.Read != "" && l.seesRead(e.Read, e.ReadKnown, person),
			l.toThem(e, puts, person),
			e.Answers != 0 && to[e.Answers]:
		default:
			continue
		}
		if l.toThem(e, puts, person) {
			to[e.ID] = true
		}
		shown = append(shown, e)
	}
	return shown
}

// maySay says whether someone may say more in a request: its asker; the
// people of its team; for a request with an origin, the people who can
// read it, a guest asker only there; and for a sub-request, the people of
// the team that asked it and, through their runs, its workers.
func (l *look) maySay(r *Request, w Who, in Where) (bool, error) {
	person := l.personOf(w)
	switch {
	case w.Kind == IsIdentity && person == "":
		return w == r.Asker && l.atOrigin(r, in), nil
	case w.Kind == IsWorker:
		return r.Parent != 0 && r.Asker.Kind == IsWorker && w.Team == r.Asker.Team && w.Known == r.Asker.Known &&
			l.teamNow(w.Team, w.Known) != nil, nil
	case person == "":
		return false, nil
	case l.is(r.Asker, person), l.isOf(r.Team, r.TeamKnown, person):
		return true, nil
	case r.Parent != 0:
		return r.Asker.Kind == IsWorker && l.isOf(r.Asker.Team, r.Asker.Known, person), nil
	}
	return l.readsOrigin(r, person)
}

// atOrigin says whether something came in at a request's origin.
func (l *look) atOrigin(r *Request, in Where) bool {
	switch r.Origin.Kind {
	case AtPlace:
		return in.Bridge == r.Origin.Bridge && in.Place == r.Origin.Place
	case InChat:
		return in.Screens
	}
	return false
}

// layer gives whose what was read an entry is, or "" for what was done:
// what is said at the origin, by or to the asker, or across the team's
// wall is what was done; what is said between the team's own people and
// workers away from the origin is what was read of it.
func (l *look) layer(r *Request, by Who, to []Who, in Where) (string, int64) {
	if l.same(by, r.Asker) || slices.ContainsFunc(to, func(w Who) bool { return l.same(w, r.Asker) }) || l.atOrigin(r, in) {
		return "", 0
	}
	if !l.inside(r.Team, r.TeamKnown, by) {
		return "", 0
	}
	for _, w := range to {
		if !l.inside(r.Team, r.TeamKnown, w) {
			return "", 0
		}
	}
	return r.Team, r.TeamKnown
}

// People gives a team's people for every purpose of sight and control: its
// own, or, for a team with none, the people its decisions reach, those of
// the first everything list from it outward that names any.
func (s *Store) People(team string) ([]string, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	return people(l.o, team), nil
}

// Sees says whether a person sees what was done of a request.
func (s *Store) Sees(n int64, person string) (bool, error) {
	l, err := s.look(s.db)
	if err != nil {
		return false, err
	}
	r, err := l.req(n)
	if errors.Is(err, ErrNotThere) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return l.seesDone(r, person)
}

// SeesAny says whether a person sees anything of a request, as View gives
// it: what was done of it, or the entries of it they see alone. A request
// that is not there they see nothing of.
func (s *Store) SeesAny(n int64, person string) (bool, error) {
	l, err := s.look(s.db)
	if err != nil {
		return false, err
	}
	r, err := l.req(n)
	if errors.Is(err, ErrNotThere) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	done, err := l.seesDone(r, person)
	if err != nil || done {
		return done, err
	}
	_, shown, err := l.seen(n, false, person)
	return len(shown) > 0, err
}

// MaySay says whether someone may say more in a request, as it came in.
// One who may not see it is told it is not there.
func (s *Store) MaySay(n int64, w Who, in Where) (bool, error) {
	l, err := s.look(s.db)
	if err != nil {
		return false, err
	}
	r, err := l.req(n)
	if err != nil {
		return false, err
	}
	may, err := l.maySay(r, w, in)
	if err != nil || may {
		return may, err
	}
	return false, l.unseen(r, w)
}

// unseen gives ErrNotThere for someone who sees nothing of a request, and
// nothing for one who sees it.
func (l *look) unseen(r *Request, w Who) error {
	if w.Kind == IsIdentity && l.personOf(w) == "" {
		return ErrNotThere
	}
	if w.Kind != IsPerson && w.Kind != IsIdentity {
		return nil
	}
	seen, err := l.seesDone(r, l.personOf(w))
	if err != nil || seen {
		return err
	}
	return ErrNotThere
}

// MayWatch says whether a person may watch a run with these labels, and
// see what it read: the people of the team its labels name, while the
// team the request went to is still that team.
func (s *Store) MayWatch(labels map[string]string, person string) (bool, error) {
	l, err := s.look(s.db)
	if err != nil {
		return false, err
	}
	team := labels[LabelTeam]
	number, ok := labels[LabelRequest]
	if !ok {
		return l.seesRead(team, l.era(IsTeam, team), person), nil
	}
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return false, nil
	}
	r, err := l.req(n)
	if errors.Is(err, ErrNotThere) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return r.Team == team && l.seesRead(r.Team, r.TeamKnown, person), nil
}

// WhatWasRead gives what the runs of a request read, from Corbie's record,
// to a person who sees what was read of its team; to one who sees only
// what was done it gives nothing.
func (s *Store) WhatWasRead(n int64, person string) ([]Record, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	r, err := l.req(n)
	if err != nil {
		return nil, err
	}
	if !l.seesRead(r.Team, r.TeamKnown, person) {
		done, err := l.seesDone(r, person)
		if err != nil {
			return nil, err
		}
		if !done {
			return nil, ErrNotThere
		}
		return nil, nil
	}
	if s.Runs == nil {
		return nil, errors.New("what the runs read cannot be shown: nothing reads Corbie's records")
	}
	return s.Runs(Labels(r))
}

// File gives a file of an entry to a person who sees the entry; to anyone
// else it is not there.
func (s *Store) File(n, entry int64, file, person string) ([]byte, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	r, err := l.req(n)
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
	done, err := l.seesDone(r, person)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(l.shown(es, puts, done, person), func(e Entry) bool { return e.ID == entry }) {
		return nil, ErrNotThere
	}
	var data []byte
	switch err := s.db.QueryRow(`SELECT data FROM requests_files WHERE entry = ? AND name = ?`, entry, file).Scan(&data); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotThere
	case err != nil:
		return nil, failed(err)
	}
	return data, nil
}
