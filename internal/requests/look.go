package requests

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// A look is the office as it was last read, with whom each name of it
// stands for now, and the requests read from the store as they are needed.
type look struct {
	q       querier
	o       *office.Office
	known   map[name]kept
	reqs    map[int64]*Request
	readers map[[2]string][]string
}

// era gives which person or team of a name is in the office now, 0 for
// none.
func (l *look) era(kind, n string) int64 { return l.known[name{kind, n}].id }

// who makes the parties of the office as it is now.
func (l *look) person(n string) Who {
	return Who{Kind: IsPerson, Name: n, Known: l.era(IsPerson, n)}
}

func (l *look) team(n string) Who { return Who{Kind: IsTeam, Name: n, Known: l.era(IsTeam, n)} }

func (l *look) worker(n string) Who {
	w := l.o.Workers[n]
	if w == nil {
		return Who{Kind: IsWorker, Name: n}
	}
	return Who{Kind: IsWorker, Name: n, Team: w.Team, Known: l.era(IsTeam, w.Team)}
}

// rule gives a rule as of the team it decides for, whichever team's
// folder holds it.
func (l *look) rule(path, team string) Who {
	return Who{Kind: IsRule, Name: path, Team: team, Known: l.era(IsTeam, team)}
}

// personOf gives the person of the office someone is now, or "" for a
// guest, a person who has gone, or anyone who is not a person.
func (l *look) personOf(w Who) string {
	switch w.Kind {
	case IsPerson:
		if w.Known != 0 && l.era(IsPerson, w.Name) == w.Known && l.o.People[w.Name] != nil {
			return w.Name
		}
	case IsIdentity:
		if p := l.o.Person(w.Bridge, w.ID); p != nil {
			return p.Name
		}
	}
	return ""
}

// teamNow gives a team of the office as it is now, nil when the one known
// so has gone.
func (l *look) teamNow(n string, known int64) *office.Team {
	if known == 0 || l.era(IsTeam, n) != known {
		return nil
	}
	return l.o.Teams[n]
}

// people gives a team's people for every purpose of sight and control: its
// own, or, for a team with none, the people its decisions reach.
func people(o *office.Office, team string) []string {
	t := o.Teams[team]
	if t == nil {
		return nil
	}
	if len(t.People) > 0 {
		return t.People
	}
	return o.Reached(team)
}

// isOf says whether a person is of a team known so, as people gives them.
func (l *look) isOf(n string, known int64, person string) bool {
	return person != "" && l.teamNow(n, known) != nil && slices.Contains(people(l.o, n), person)
}

// inside says whether someone is inside a team known so: one of its
// workers or rules, the team itself, or one of its people.
func (l *look) inside(team string, known int64, w Who) bool {
	if l.teamNow(team, known) == nil {
		return false
	}
	switch w.Kind {
	case IsWorker, IsRule:
		return w.Team == team && w.Known == known
	case IsTeam:
		return w.Name == team && w.Known == known
	}
	return l.isOf(team, known, l.personOf(w))
}

// workerOf says whether someone is a worker of the team a request went
// to, as the office is read now.
func (l *look) workerOf(r *Request, w Who) bool {
	worker := l.o.Workers[w.Name]
	return w.Kind == IsWorker && worker != nil && worker.Team == r.Team && l.inside(r.Team, r.TeamKnown, w)
}

// is says whether someone is the person.
func (l *look) is(w Who, person string) bool {
	return person != "" && l.personOf(w) == person
}

// same says whether two parties are the same one: the same person of the
// office now, or else the same as kept.
func (l *look) same(a, b Who) bool {
	if p := l.personOf(a); p != "" {
		return p == l.personOf(b)
	}
	return a == b
}

// req reads a request.
func (l *look) req(n int64) (*Request, error) {
	if r, ok := l.reqs[n]; ok {
		return r, nil
	}
	rows, err := l.q.Query(requestColumns+` WHERE number = ?`, n)
	if err != nil {
		return nil, failed(err)
	}
	rs, err := l.scanRequests(rows)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, ErrNotThere
	}
	l.reqs[n] = rs[0]
	return rs[0], nil
}

// all reads every request, oldest first.
func (l *look) all() ([]*Request, error) {
	rows, err := l.q.Query(requestColumns + ` ORDER BY number`)
	if err != nil {
		return nil, failed(err)
	}
	rs, err := l.scanRequests(rows)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		l.reqs[r.Number] = r
	}
	return rs, nil
}

// subs reads the sub-requests asked from a request, oldest first.
func (l *look) subs(n int64) ([]*Request, error) {
	rows, err := l.q.Query(requestColumns+` WHERE parent = ? ORDER BY number`, n)
	if err != nil {
		return nil, failed(err)
	}
	rs, err := l.scanRequests(rows)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		l.reqs[r.Number] = r
	}
	return rs, nil
}

// lookUp gives the number a query finds, and whether it finds one.
func (l *look) lookUp(query string, args ...any) (int64, bool, error) {
	var n int64
	switch err := l.q.QueryRow(query, args...).Scan(&n); {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, failed(err)
	}
	return n, true, nil
}

// latestAt gives the latest request asked at a place on a bridge, and
// whether there is one.
func (l *look) latestAt(bridge, place string) (int64, bool, error) {
	return l.lookUp(`SELECT number FROM requests_requests WHERE bridge = ? AND place = ? ORDER BY number DESC LIMIT 1`, bridge, place)
}

// ofTicket gives the sub-request kept with a Corbie ticket, and whether
// there is one.
func (l *look) ofTicket(ticket string) (int64, bool, error) {
	return l.lookUp(`SELECT number FROM requests_requests WHERE ticket = ?`, ticket)
}

const requestColumns = `SELECT number, team, team_known, asker, origin, parent, coalesce(ticket, ''), worker,
	answered, stopped, held, source, made FROM requests_requests`

// scanRequests reads requests, their times in the office's zone.
func (l *look) scanRequests(rows *sql.Rows) ([]*Request, error) {
	defer rows.Close()
	var rs []*Request
	for rows.Next() {
		r := &Request{}
		var asker, origin, source, made string
		if err := rows.Scan(&r.Number, &r.Team, &r.TeamKnown, &asker, &origin, &r.Parent, &r.Ticket, &r.Worker,
			&r.Answered, &r.Stopped, &r.Held, &source, &made); err != nil {
			return nil, failed(err)
		}
		if err := errors.Join(json.Unmarshal([]byte(asker), &r.Asker), json.Unmarshal([]byte(origin), &r.Origin)); err != nil {
			return nil, failed(err)
		}
		if source != "" {
			r.Source = &Status{}
			if err := json.Unmarshal([]byte(source), r.Source); err != nil {
				return nil, failed(err)
			}
			r.Source.At = r.Source.At.In(l.o.Zone)
		}
		at, err := time.Parse(time.RFC3339Nano, made)
		if err != nil {
			return nil, failed(err)
		}
		r.Made = at.In(l.o.Zone)
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, failed(err)
	}
	return rs, nil
}

// entries reads a request's thread, in the order it was written, its times
// in the office's zone, each message its team's heed noted marked so.
func (l *look) entries(n int64) ([]Entry, error) {
	rows, err := l.q.Query(`SELECT id, request, kind, by_, to_, at, text, came_in, read_team, read_known, answers, sub, decision, mentioned, offers
		FROM requests_entries WHERE request = ? ORDER BY id`, n)
	if err != nil {
		return nil, failed(err)
	}
	defer rows.Close()
	var es []Entry
	for rows.Next() {
		var e Entry
		var by, to, at, in, offered string
		if err := rows.Scan(&e.ID, &e.Request, &e.Kind, &by, &to, &at, &e.Text, &in, &e.Read, &e.ReadKnown, &e.Answers, &e.Sub, &e.Decision, &e.Mentioned, &offered); err != nil {
			return nil, failed(err)
		}
		var of offers
		if err := errors.Join(json.Unmarshal([]byte(by), &e.By), json.Unmarshal([]byte(to), &e.To), json.Unmarshal([]byte(in), &e.In), json.Unmarshal([]byte(offered), &of)); err != nil {
			return nil, failed(err)
		}
		e.Choices = of.Choices
		t, err := time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return nil, failed(err)
		}
		e.At = t.In(l.o.Zone)
		es = append(es, e)
	}
	if err := rows.Err(); err != nil {
		return nil, failed(err)
	}
	rows.Close()
	noted := map[int64]bool{}
	for _, e := range es {
		if e.Kind == KindNoted {
			noted[e.Answers] = true
		}
	}
	for i := range es {
		es[i].Noted = noted[es[i].ID]
	}
	heads, err := l.q.Query(`SELECT f.entry, f.name, length(f.data) FROM requests_files f JOIN requests_entries e ON e.id = f.entry
		WHERE e.request = ? ORDER BY f.rowid`, n)
	if err != nil {
		return nil, failed(err)
	}
	defer heads.Close()
	for heads.Next() {
		var id int64
		var h FileHead
		if err := heads.Scan(&id, &h.Name, &h.Size); err != nil {
			return nil, failed(err)
		}
		for i := range es {
			if es[i].ID == id {
				es[i].Files = append(es[i].Files, h)
			}
		}
	}
	return es, failedOr(heads.Err())
}

// entry reads one entry.
func (l *look) entry(id int64) (Entry, error) {
	var n int64
	if err := l.q.QueryRow(`SELECT request FROM requests_entries WHERE id = ?`, id).Scan(&n); errors.Is(err, sql.ErrNoRows) {
		return Entry{}, ErrNotThere
	} else if err != nil {
		return Entry{}, failed(err)
	}
	es, err := l.entries(n)
	if err != nil {
		return Entry{}, err
	}
	for _, e := range es {
		if e.ID == id {
			return e, nil
		}
	}
	return Entry{}, ErrNotThere
}

// A wait is one entry waiting on someone, or, once closed, put to them.
type wait struct {
	entry   int64
	request int64
	who     Who
}

// waits reads what waits in a request, or in every request for 0.
func (l *look) waits(n int64) ([]wait, error) { return l.readWaits(n, true) }

// puts reads whom each entry of a request was ever put to, open or closed:
// they see it, with what answers it.
func (l *look) puts(n int64) ([]wait, error) { return l.readWaits(n, false) }

func (l *look) readWaits(n int64, open bool) ([]wait, error) {
	rows, err := l.q.Query(`SELECT entry, request, who FROM requests_waits WHERE (open = 1 OR ?) AND (request = ? OR ? = 0)
		ORDER BY entry, rowid`, !open, n, n)
	if err != nil {
		return nil, failed(err)
	}
	defer rows.Close()
	var ws []wait
	for rows.Next() {
		var w wait
		var who string
		if err := rows.Scan(&w.entry, &w.request, &who); err != nil {
			return nil, failed(err)
		}
		if err := json.Unmarshal([]byte(who), &w.who); err != nil {
			return nil, failed(err)
		}
		ws = append(ws, w)
	}
	return ws, failedOr(rows.Err())
}

// readersOf gives the identities a bridge last reported as reading a
// place, and whether it ever reported any.
func (l *look) readersOf(bridge, place string) ([]string, bool, error) {
	if l.readers == nil {
		l.readers = map[[2]string][]string{}
		rows, err := l.q.Query(`SELECT bridge, place, ids FROM requests_readers`)
		if err != nil {
			return nil, false, failed(err)
		}
		defer rows.Close()
		for rows.Next() {
			var b, p, ids string
			var list []string
			if err := rows.Scan(&b, &p, &ids); err != nil {
				return nil, false, failed(err)
			}
			if err := json.Unmarshal([]byte(ids), &list); err != nil {
				return nil, false, failed(err)
			}
			l.readers[[2]string{b, p}] = list
		}
		if err := rows.Err(); err != nil {
			return nil, false, failed(err)
		}
	}
	ids, ok := l.readers[[2]string{bridge, place}]
	return ids, ok, nil
}

// A failure is the store failing, which is never told as a request not
// there or not to be seen.
type failure struct{ err error }

func (f failure) Error() string {
	return "the requests' part of the store failed: " + f.err.Error()
}
func (f failure) Unwrap() error { return f.err }

func failed(err error) error {
	var f failure
	if errors.As(err, &f) {
		return err
	}
	return failure{err}
}

func failedOr(err error) error {
	if err == nil {
		return nil
	}
	return failed(err)
}

// called gives what a team is called, as it was when it was last read.
func called(o *office.Office, team string) string {
	if t := o.Teams[team]; t != nil {
		return t.Called
	}
	return team
}

// calledOf gives what a party is called, for Bulldog's entries.
func (l *look) calledOf(w Who) string {
	switch w.Kind {
	case IsBulldog:
		return "Bulldog"
	case IsTeam:
		return called(l.o, w.Name)
	case IsWorker, IsRule:
		return w.Name
	}
	if p := l.personOf(w); p != "" {
		return l.o.People[p].Called
	}
	if w.Kind == IsIdentity {
		return fmt.Sprintf("a guest (%s %s)", w.Bridge, w.ID)
	}
	return w.Name
}

// firstLine gives a text's first line.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
