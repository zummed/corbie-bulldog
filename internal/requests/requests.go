// Package requests keeps every request of an office and everything said in
// it: who asked, where, the team it went to, its thread of entries, its
// sub-requests, and what waits on whom. It decides who may see which part
// of a request and who may say more in it, reading the office as it was
// last read each time. It knows Corbie's runs only by their labels: what a
// run read is never kept here, and is handed in by whoever reads Corbie.
package requests

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// Area is this part's name in the store, and Version the shape of its
// tables.
const (
	Area    = "requests"
	Version = 4
)

// The most a request keeps of one text and of one file. What is larger is
// refused whole.
const (
	MaxText = 1 << 20
	MaxFile = 16 << 20
)

// The kinds of party.
const (
	IsIdentity = "identity" // who someone is on a bridge, as it checked them
	IsPerson   = "person"   // a person as signed in on the screens
	IsWorker   = "worker"
	IsTeam     = "team"
	IsRule     = "rule"
	IsBulldog  = office.IsBulldog
)

// A Who is someone an entry is by or to, or who asked, kept as what it is
// and never as a bare name.
type Who struct {
	Kind string `json:"kind"`
	// Name is a person's, a worker's or a team's name, or a rule's path in
	// the office.
	Name string `json:"name,omitempty"`
	// Team is a worker's or a rule's team.
	Team string `json:"team,omitempty"`
	// Bridge and ID are an identity.
	Bridge string `json:"bridge,omitempty"`
	ID     string `json:"id,omitempty"`
	// Known says which person or team of that name it is, of a worker or
	// a rule its team's: a name written again for someone new is known
	// anew, and inherits nothing.
	Known int64 `json:"known,omitempty"`
}

// Bulldog is who Bulldog's own entries are by. No worker or person is
// taken for it.
var Bulldog = Who{Kind: IsBulldog}

// Identity is who someone is on a bridge, as the bridge checked them.
func Identity(bridge, id string) Who { return Who{Kind: IsIdentity, Bridge: bridge, ID: id} }

// The kinds of origin.
const (
	InChat    = "chat"
	AtPlace   = "place"
	ByRoutine = "routine"
)

// An Origin is where a request was asked. A sub-request has none.
type Origin struct {
	Kind string `json:"kind"`
	// Bridge and Place are where on a bridge it was asked, as narrow as the
	// bridge names it; Heard is the listed place it was heard in.
	Bridge string `json:"bridge,omitempty"`
	Heard  string `json:"heard,omitempty"`
	Place  string `json:"place,omitempty"`
	// About is the link of the source thing, such as an issue, that its
	// asking opened at the place, if the bridge named one.
	About string `json:"about,omitempty"`
	// Team, At, To and Say are the routine that asked it, of the team
	// known as Known, and By the person who ran it now, rather than at its
	// time.
	Team  string `json:"team,omitempty"`
	Known int64  `json:"known,omitempty"`
	At    string `json:"at,omitempty"`
	To    string `json:"to,omitempty"`
	Say   string `json:"say,omitempty"`
	By    Who    `json:"by,omitzero"`
}

// Quiet says whether an empty answer to a request asked here says
// nothing: one asked by a routine, or at a channel place, any place on a
// bridge but what is said to it directly.
func (o Origin) Quiet() bool {
	return o.Kind == ByRoutine || o.Kind == AtPlace && o.Heard != office.Direct
}

// A Where is where an entry came in: a bridge's place, or the screens.
// What a run said came in nowhere.
type Where struct {
	Screens bool   `json:"screens,omitempty"`
	Bridge  string `json:"bridge,omitempty"`
	Place   string `json:"place,omitempty"`
}

// Screens is where what is said on the screens, the chat among them, comes
// in.
var Screens = Where{Screens: true}

// A File is one file on an entry, as it is given to be kept.
type File struct {
	Name string
	Data []byte
}

// A Status is a thing's status at its source, as its bridge last reported
// it. It is never set here.
type Status struct {
	Status string    `json:"status"`
	Closed bool      `json:"closed,omitempty"`
	By     Who       `json:"by"`
	At     time.Time `json:"at"`
	Link   string    `json:"link,omitempty"`
}

// A Request is one thing someone asked.
type Request struct {
	Number int64
	// Team is the team it went to, known as TeamKnown.
	Team      string
	TeamKnown int64
	Asker     Who
	Origin    Origin
	// Parent is the request a sub-request was asked from, 0 for one asked
	// at an origin; Ticket is the Corbie ticket its asking run waits on.
	Parent int64
	Ticket string
	// Worker is the worker a routine's request is said to; any other goes
	// to its team's lead.
	Worker string
	// Answered says its lead's answer, or Bulldog's last entry, has been
	// said and nothing since; Stopped that it was stopped, and nothing
	// since has opened it; Held that the answer of its lead, a person, is
	// held by its team's disclosure check, and not yet said. A request whose
	// answer is said still waits while anything in it waits on someone.
	Answered bool
	Stopped  bool
	Held     bool
	// Source is the status of the thing it came from, once its bridge has
	// reported one.
	Source *Status
	Made   time.Time
}

// The kinds of entry this area writes. Decisions and Changes write kinds
// of their own.
const (
	KindAsked    = "asked"    // the asking, first in every request
	KindSaid     = "said"     // more said by someone who may
	KindQuestion = "question" // a worker's question to a person, open until answered
	KindTold     = "told"     // a worker's tell, which waits for nothing
	KindReplied  = "replied"  // a person's answer to what was put to them
	KindSub      = "sub"      // a worker's asking of another team, which leads to a sub-request
	KindAnswer   = "answer"   // the lead's answer
	KindLate     = "late"     // a sub-request's answer that came when no run waited for it
	KindBulldog  = "bulldog"  // Bulldog's own: a failure, a stop or a line in its place, a close at the source, a going
	KindHeld     = "held"     // the answer of a lead who is a person, held by its team's disclosure check: what was read of the team
	KindNoted    = "noted"    // a team's heed's word on a message it noted, which it answers: what was read of the team
)

// An Entry is one thing said in a request. It is written once and never
// changed.
type Entry struct {
	ID      int64
	Request int64
	Kind    string
	By      Who
	To      []Who
	// At is in the office's zone.
	At    time.Time
	Text  string
	Files []FileHead
	In    Where
	// Read is the team whose what was read it is, known as ReadKnown; ""
	// for what was done.
	Read      string
	ReadKnown int64
	// Answers is the entry it answers; Sub the sub-request it leads to;
	// Decision the decision it is of.
	Answers  int64
	Sub      int64
	Decision int64
	// Noted says a message, the asking or more said, was noted by its
	// team's heed: worked out when it is read, as the message itself is
	// never changed.
	Noted bool
	// Mentioned says the bridge it was heard on marked it as naming the
	// bridge's own bot.
	Mentioned bool
}

// A FileHead names a file kept on an entry.
type FileHead struct {
	Name string
	Size int
}

// A Record is what one run read, as Corbie keeps it, handed in by whoever
// reads Corbie. Nothing of it is kept here.
type Record struct {
	Run    string
	Team   string
	Worker string
	// Gone says Corbie no longer keeps the record of the run.
	Gone bool
	// Events are the record as Corbie gives it.
	Events json.RawMessage
}

// The labels of a run that name what it is for.
const (
	LabelTeam    = "team"
	LabelRequest = "request"
)

// Labels gives the labels a run of a request carries that name it: its
// team and its number. A run deciding for a team is of that team, which
// is the request's.
func Labels(r *Request) map[string]string {
	return map[string]string{LabelTeam: r.Team, LabelRequest: fmt.Sprint(r.Number)}
}

// ErrNotThere is what anyone is told of a request that is not there, and
// of one they may not see: the two are told alike.
var ErrNotThere = errors.New("there is no such request")

// A Refusal is something this area does not allow, said in a sentence for
// whoever tried. Nothing was written.
type Refusal string

func (r Refusal) Error() string { return string(r) }

func refuse(format string, args ...any) error { return Refusal(fmt.Sprintf(format, args...)) }

// A Store is the requests' part of Bulldog's one file.
type Store struct {
	db  *sql.DB
	now func() time.Time

	// Runs gives what the runs labelled so read, from Corbie's record:
	// whoever reads Corbie sets it.
	Runs func(labels map[string]string) ([]Record, error)
	// Held is handed each answer a disclosure check holds, once it is kept,
	// to be decided; an error says it was not handed. Decisions sets it.
	Held func(held *Entry) error

	// writing is held for the whole of each write, so writes come one at
	// a time.
	writing sync.Mutex

	mu       sync.Mutex
	office   *office.Office
	known    map[name]kept
	watchers map[int]func(int64)
	next     int
}

// A name is a person or a team, by its kind and name.
type name struct{ kind, name string }

// A kept is a person or a team as the last reading of the office had them:
// which one of that name it is, what it was called, and, of a person, who
// they were on each bridge. It outlives a restart, so what was open to one
// who has gone since is closed saying who.
type kept struct {
	id      int64
	called  string
	reached []office.Identity
}

const schema = `
CREATE TABLE requests_requests(
	number INTEGER PRIMARY KEY AUTOINCREMENT,
	team TEXT NOT NULL, team_known INTEGER NOT NULL,
	asker TEXT NOT NULL, origin TEXT NOT NULL,
	bridge TEXT NOT NULL DEFAULT '', place TEXT NOT NULL DEFAULT '',
	parent INTEGER NOT NULL DEFAULT 0, ticket TEXT UNIQUE,
	worker TEXT NOT NULL DEFAULT '',
	answered INTEGER NOT NULL DEFAULT 0, stopped INTEGER NOT NULL DEFAULT 0, held INTEGER NOT NULL DEFAULT 0,
	source TEXT NOT NULL DEFAULT '', made TEXT NOT NULL);
CREATE INDEX requests_requests_parent ON requests_requests(parent);
CREATE INDEX requests_requests_place ON requests_requests(bridge, place);
CREATE TABLE requests_entries(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	request INTEGER NOT NULL REFERENCES requests_requests(number),
	kind TEXT NOT NULL, by_ TEXT NOT NULL, to_ TEXT NOT NULL, at TEXT NOT NULL,
	text TEXT NOT NULL, came_in TEXT NOT NULL,
	read_team TEXT NOT NULL DEFAULT '', read_known INTEGER NOT NULL DEFAULT 0,
	answers INTEGER NOT NULL DEFAULT 0, sub INTEGER NOT NULL DEFAULT 0, decision INTEGER NOT NULL DEFAULT 0,
	mentioned INTEGER NOT NULL DEFAULT 0);
CREATE INDEX requests_entries_request ON requests_entries(request, id);
CREATE TABLE requests_files(
	entry INTEGER NOT NULL REFERENCES requests_entries(id), name TEXT NOT NULL, data BLOB NOT NULL,
	PRIMARY KEY(entry, name));
CREATE TABLE requests_waits(
	entry INTEGER NOT NULL REFERENCES requests_entries(id), request INTEGER NOT NULL, who TEXT NOT NULL,
	open INTEGER NOT NULL DEFAULT 1, PRIMARY KEY(entry, who));
CREATE INDEX requests_waits_request ON requests_waits(request);
CREATE TABLE requests_readers(bridge TEXT NOT NULL, place TEXT NOT NULL, ids TEXT NOT NULL, PRIMARY KEY(bridge, place));
CREATE TABLE requests_known(id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, name TEXT NOT NULL,
	called TEXT NOT NULL, reached TEXT NOT NULL, gone INTEGER NOT NULL DEFAULT 0);
`

// Open opens the requests' part of the store, making it the first time.
// A part made by an older Bulldog is refused. Nothing is shown or written
// until the office has been read with Read.
func Open(db *sql.DB, now func() time.Time) (*Store, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS parts(area TEXT PRIMARY KEY, version INTEGER)`); err != nil {
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	}
	var version int
	switch err := db.QueryRow(`SELECT version FROM parts WHERE area = ?`, Area).Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		var tables int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'requests\_%' ESCAPE '\'`).Scan(&tables); err != nil {
			return nil, fmt.Errorf("the store cannot be read: %w", err)
		}
		if tables > 0 {
			return nil, older(0)
		}
		if err := makePart(db); err != nil {
			return nil, fmt.Errorf("the requests' part of the store cannot be made: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	case version != Version:
		return nil, older(version)
	}
	s := &Store{db: db, now: now, known: map[name]kept{}, watchers: map[int]func(int64){}}
	rows, err := db.Query(`SELECT id, kind, name, called, reached FROM requests_known WHERE gone = 0`)
	if err != nil {
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k kept
		var n name
		var reached string
		if err := rows.Scan(&k.id, &n.kind, &n.name, &k.called, &reached); err != nil {
			return nil, fmt.Errorf("the store cannot be read: %w", err)
		}
		if err := json.Unmarshal([]byte(reached), &k.reached); err != nil {
			return nil, fmt.Errorf("the store cannot be read: %w", err)
		}
		s.known[n] = k
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	}
	return s, nil
}

func older(version int) error {
	return fmt.Errorf("the requests in the store were kept by an older Bulldog (version %d, and this one keeps version %d), and are not read: remove .bulldog/bulldog.db and start again", version, Version)
}

func makePart(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO parts(area, version) VALUES (?, ?)`, Area, Version); err != nil {
		return err
	}
	return tx.Commit()
}

// Watch has fn told the number of each request a write changed, once for
// each, after the write is kept. It gives a func that ends the watch.
func (s *Store) Watch(fn func(number int64)) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.next
	s.next++
	s.watchers[id] = fn
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.watchers, id)
	}
}

// WatchAs has fn told of each change to a request the person may see any
// of, and of no other. When the store fails in judging whether they may,
// fn is told the failure, with no number.
func (s *Store) WatchAs(person string, fn func(number int64, err error)) func() {
	return s.Watch(func(n int64) {
		switch _, err := s.View(n, person); {
		case err == nil:
			fn(n, nil)
		case !errors.Is(err, ErrNotThere):
			fn(0, err)
		}
	})
}

// A querier is the store, or one write to it.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// look gives the office as it was last read, with the store to read
// requests from.
func (s *Store) look(q querier) (*look, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.office == nil {
		return nil, errors.New("the office has not been read, so no request can be shown or made")
	}
	return &look{q: q, o: s.office, known: s.known, reqs: map[int64]*Request{}}, nil
}

// A change is one write: everything in it is kept, or nothing, and
// changed are the requests it changed, in the order it changed them.
type change struct {
	*look
	tx      *sql.Tx
	at      time.Time
	changed []int64
}

func (c *change) touch(n int64) {
	if !slices.Contains(c.changed, n) {
		c.changed = append(c.changed, n)
	}
}

// write makes one write, and then tells the watchers of each request it
// changed.
func (s *Store) write(do func(c *change) error) error {
	changed, err := s.commit(func(tx *sql.Tx) (*change, error) {
		l, err := s.look(tx)
		if err != nil {
			return nil, err
		}
		c := &change{look: l, tx: tx, at: s.now().UTC()}
		return c, do(c)
	})
	if err != nil {
		return err
	}
	s.tell(changed)
	return nil
}

// commit makes one write, one at a time, kept whole or not at all, and
// gives the requests it changed.
func (s *Store) commit(do func(tx *sql.Tx) (*change, error)) ([]int64, error) {
	s.writing.Lock()
	defer s.writing.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("the store cannot be written: %w", err)
	}
	defer tx.Rollback()
	c, err := do(tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("the store cannot be written: %w", err)
	}
	return c.changed, nil
}

// Tell tells the watchers of each request that what another area keeps
// of it changed, as how an entry of it is carried, as a write to the
// request itself would.
func (s *Store) Tell(numbers ...int64) { s.tell(numbers) }

func (s *Store) tell(changed []int64) {
	s.mu.Lock()
	var watchers []func(int64)
	for _, id := range slices.Sorted(maps.Keys(s.watchers)) {
		watchers = append(watchers, s.watchers[id])
	}
	s.mu.Unlock()
	for _, n := range changed {
		for _, fn := range watchers {
			fn(n)
		}
	}
}

// stamp writes an instant as the store keeps it.
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func jsonOf(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
