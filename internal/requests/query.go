package requests

import (
	"slices"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// What follows is for the areas that drive the work, and judges no sight:
// it is never shown to a person as it is.

// Get gives a request.
func (s *Store) Get(n int64) (*Request, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	return l.req(n)
}

// Thread gives a request's whole thread, in the order it was written, its
// times in the office's zone.
func (s *Store) Thread(n int64) ([]Entry, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	if _, err := l.req(n); err != nil {
		return nil, err
	}
	return l.entries(n)
}

// HeldAnswers gives the answer each request holds for its team's disclosure
// check: the latest a lead who is a person said, of each request whose
// answer is held.
func (s *Store) HeldAnswers() ([]Entry, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT number FROM requests_requests WHERE held = 1 ORDER BY number`)
	if err != nil {
		return nil, failed(err)
	}
	var ns []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, failed(err)
		}
		ns = append(ns, n)
	}
	if err := failedOr(rows.Err()); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	var held []Entry
	for _, n := range ns {
		es, err := l.entries(n)
		if err != nil {
			return nil, err
		}
		for _, e := range slices.Backward(es) {
			if e.Kind == KindHeld {
				held = append(held, e)
				break
			}
		}
	}
	return held, nil
}

// Entry gives one entry.
func (s *Store) Entry(id int64) (Entry, error) {
	l, err := s.look(s.db)
	if err != nil {
		return Entry{}, err
	}
	return l.entry(id)
}

// State gives where a request stands.
func (s *Store) State(n int64) (State, error) {
	l, err := s.look(s.db)
	if err != nil {
		return State{}, err
	}
	r, err := l.req(n)
	if err != nil {
		return State{}, err
	}
	ws, err := l.waits(n)
	if err != nil {
		return State{}, err
	}
	es, err := l.entries(n)
	if err != nil {
		return State{}, err
	}
	return l.state(r, ws, es), nil
}

// Tree gives the sub-requests beneath a request, as far down as they go,
// each with its team, question, state and answer.
func (s *Store) Tree(n int64) ([]Sub, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	if _, err := l.req(n); err != nil {
		return nil, err
	}
	return l.tree(n)
}

// Top gives the request at the top of a request's tree, whose origin is
// who in the end reads what is said beneath it, and how many levels below
// it the request is.
func (s *Store) Top(n int64) (*Request, int, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, 0, err
	}
	r, err := l.req(n)
	for depth := 0; err == nil; depth++ {
		if r.Parent == 0 {
			return r, depth, nil
		}
		r, err = l.req(r.Parent)
	}
	return nil, 0, err
}

// Asks gives who asks a request, as the office is read now: a person, by
// their name and the teams they are in; a guest, by the bridge and the
// identity it vouched for; for a sub-request, the worker that asked and
// its team; for a routine, the team.
func (s *Store) Asks(n int64) (Party, error) {
	l, err := s.look(s.db)
	if err != nil {
		return Party{}, err
	}
	r, err := l.req(n)
	if err != nil {
		return Party{}, err
	}
	return l.party(r.Asker), nil
}

// Lead gives who answers a request: the worker a routine names, or else
// the team's lead as the office is read now, and whether that is a
// person. A request whose team has gone has none.
func (s *Store) Lead(n int64) (string, bool, error) {
	l, err := s.look(s.db)
	if err != nil {
		return "", false, err
	}
	r, err := l.req(n)
	if err != nil {
		return "", false, err
	}
	t := l.teamNow(r.Team, r.TeamKnown)
	switch {
	case t == nil:
		return "", false, nil
	case r.Worker != "":
		return r.Worker, false, nil
	}
	return t.Lead, l.o.People[t.Lead] != nil, nil
}

// OpenOf gives a team's open requests, those working or waiting on
// someone, newest first, as its people see what was done of them.
func (s *Store) OpenOf(team string) ([]Summary, error) {
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
	var open []Summary
	for _, r := range slices.Backward(rs) {
		if l.answered(r, ws) || r.Team != team || l.teamNow(r.Team, r.TeamKnown) == nil {
			continue
		}
		sum, err := l.summary(r, ws, "")
		if err != nil {
			return nil, err
		}
		open = append(open, sum)
	}
	return open, nil
}

// SaidBefore gives what was said before a request where it was asked: in
// its team's other requests asked before it at the same listed place of
// the same bridge, each read by every reader of it, their messages, noted
// or not, and the team's answers, newest first. A request asked at no
// place gives nothing.
func (s *Store) SaidBefore(n int64) ([]Entry, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	r, err := l.req(n)
	if err != nil || r.Origin.Kind != AtPlace {
		return nil, err
	}
	rs, err := l.all()
	if err != nil {
		return nil, err
	}
	var said []Entry
	for _, o := range slices.Backward(rs) {
		if o.Number >= n || o.Team != r.Team || o.TeamKnown != r.TeamKnown || o.Origin.Kind != AtPlace ||
			o.Origin.Bridge != r.Origin.Bridge || o.Origin.Heard != r.Origin.Heard {
			continue
		}
		reads, err := l.readsAll(r, o)
		if err != nil {
			return nil, err
		}
		if !reads {
			continue
		}
		es, err := l.entries(o.Number)
		if err != nil {
			return nil, err
		}
		for _, e := range slices.Backward(es) {
			switch e.Kind {
			case KindAsked, KindSaid, KindAnswer:
				if e.Read == "" {
					said = append(said, e)
				}
			}
		}
	}
	return said, nil
}

// LatestAt gives the latest request asked at a place on a bridge, open or
// answered, and whether there is one.
func (s *Store) LatestAt(bridge, place string) (int64, bool, error) {
	return (&look{q: s.db}).latestAt(bridge, place)
}

// OfTicket gives the sub-request kept with a Corbie ticket, and whether
// there is one.
func (s *Store) OfTicket(ticket string) (int64, bool, error) {
	return (&look{q: s.db}).ofTicket(ticket)
}

// Who gives the parties of the office as it is now, by name: a person as
// signed in, a worker, a team, or a rule by its path as of the team it
// decides for.
func (s *Store) Person(name string) Who { return s.who(func(l *look) Who { return l.person(name) }) }
func (s *Store) Worker(name string) Who { return s.who(func(l *look) Who { return l.worker(name) }) }
func (s *Store) Team(name string) Who   { return s.who(func(l *look) Who { return l.team(name) }) }
func (s *Store) Rule(path, team string) Who {
	return s.who(func(l *look) Who { return l.rule(path, team) })
}

func (s *Store) who(of func(*look) Who) Who {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.office
	if o == nil {
		o = &office.Office{Workers: map[string]*office.Worker{}, Rules: map[string]*office.Rule{}}
	}
	return of(&look{o: o, known: s.known})
}

// ReadBy gives who reads the answer of a request asked at an origin, as
// the office is read now: on a bridge, the people its in machine last
// reported as reading the place, or the asker alone where it never
// reported any; in the chat, the asker; for a routine, its team's people.
// A sub-request is read by the run that asked it, and gives none.
func (s *Store) ReadBy(n int64) ([]Party, error) {
	l, err := s.look(s.db)
	if err != nil {
		return nil, err
	}
	r, err := l.req(n)
	if err != nil {
		return nil, err
	}
	whos, err := l.originReaders(r)
	if err != nil {
		return nil, err
	}
	var readers []Party
	for _, w := range whos {
		readers = append(readers, l.party(w))
	}
	return readers, nil
}
