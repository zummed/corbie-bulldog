// Package bridges is the contract between Bulldog and its bridges: what an
// in machine's run is given and what it heard means, what an out machine's
// run carries and what its end means, the places a bridge listens in, who
// a speaker is, and how each thing is carried once, in order, place by
// place. It never speaks to Corbie: whoever drives Corbie asks it which
// runs to start, starts them with the labels it gives, and hands it back
// how each ended. What it keeps, it keeps in its own part of the store:
// each bridge's cursor and the trouble of its last run, the things heard,
// and every thing carried, with how its runs went.
package bridges

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// Area is this part's name in the store, and Version the shape of its
// tables.
const (
	Area    = "bridges"
	Version = 3
)

// The kinds of run this area asks for, as their kind label says, and the
// labels it names them by beside a team's and a request's. An out run
// that carries an entry of a request carries that request's labels, its
// team among them; every other run of a bridge, in, sign-in or a sentence
// carried back, is of the top team, whose bridges they are.
const (
	KindIn     = "bridge-in"
	KindOut    = "bridge-out"
	KindSignIn = "bridge-sign-in"

	LabelKind   = "kind"
	LabelBridge = "bridge"
	LabelThing  = "thing"
)

// The inputs a bridge's machines are given. An in machine that takes
// cursor listens, and one that takes delivery hears; an out machine that
// takes direct can say something to a person directly, and is told so.
const (
	InputSettings = "settings"
	InputCursor   = "cursor"
	InputDelivery = "delivery"
	InputDirect   = "direct"
)

// The most a delivery may be, which is well within what Corbie sends a run
// as its input; the runs a thing is carried by at most; how long the slowest pause between runs grows to; how long the
// service that calls a bridge's address waits for its answer; and the
// first pause after a listening run, or an out run, that failed.
const (
	MaxDelivery   = 128 << 10
	MaxRuns       = 5
	LongestPause  = time.Minute
	AnswerWithin  = 2 * time.Second
	firstInPause  = time.Second
	firstOutPause = 2 * time.Second
)

const schema = `
CREATE TABLE bridges_bridges(
	bridge TEXT PRIMARY KEY, cursor TEXT NOT NULL DEFAULT 'null',
	run TEXT NOT NULL DEFAULT '', claimed INTEGER NOT NULL DEFAULT 0,
	failures INTEGER NOT NULL DEFAULT 0, next TEXT NOT NULL DEFAULT '',
	trouble TEXT NOT NULL DEFAULT '', since TEXT NOT NULL DEFAULT '');
CREATE TABLE bridges_heard(
	bridge TEXT NOT NULL, id TEXT NOT NULL, place TEXT NOT NULL,
	request INTEGER NOT NULL DEFAULT 0, entry INTEGER NOT NULL DEFAULT 0, at TEXT NOT NULL,
	PRIMARY KEY(bridge, id));
CREATE INDEX bridges_heard_entry ON bridges_heard(entry);
CREATE INDEX bridges_heard_request ON bridges_heard(request);
CREATE TABLE bridges_things(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	bridge TEXT NOT NULL, place TEXT NOT NULL, direct INTEGER NOT NULL DEFAULT 0,
	person TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL,
	request INTEGER NOT NULL DEFAULT 0, entry INTEGER NOT NULL DEFAULT 0, text TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL, run TEXT NOT NULL DEFAULT '', ended TEXT NOT NULL DEFAULT '', runs INTEGER NOT NULL DEFAULT 0,
	next TEXT NOT NULL DEFAULT '', sent TEXT NOT NULL DEFAULT '', why TEXT NOT NULL DEFAULT '',
	made TEXT NOT NULL);
CREATE INDEX bridges_things_place ON bridges_things(bridge, place, id);
CREATE INDEX bridges_things_sent ON bridges_things(bridge, sent);
CREATE INDEX bridges_things_request ON bridges_things(request);
CREATE TABLE bridges_seen(request INTEGER PRIMARY KEY, entry INTEGER NOT NULL);
`

// A Start is a run this area asks for: of which machine, with what input
// and which labels. Whoever starts it adds the office's own label.
type Start struct {
	Kind   string
	Bridge string
	// Thing is the thing an out run carries.
	Thing   int64
	Machine *machine.Draft
	Input   map[string]any
	Labels  map[string]string
}

// An End is how a run ended: its output, or the error Corbie gave for it.
// Fetch reads a blob of the run's output by the path it was shown with,
// for the files an in run heard; it is used before the run's record can go.
type End struct {
	Output any
	Failed string
	Fetch  func(path string) ([]byte, error)
}

// A Trouble is a bridge whose runs are failing, with what Corbie said of
// the latest, since when, and how many in a row. It is shown to the
// office's people until a run ends well.
type Trouble struct {
	Bridge   string
	Error    string
	Since    time.Time
	Failures int
}

// A Choice is a choice heard on a bridge, on a question or a decision
// that was carried there, or words said in answer to a decision: what was
// chosen, or said, by whom as the bridge checked them, and where.
type Choice struct {
	Bridge string
	Who    requests.Who
	// Person is the person the identity is, or "" for a guest.
	Person  string
	Request int64
	Entry   int64
	Choice  string
	Text    string
	In      requests.Where
}

// A SignIn is how a one-time sign-in link's carrying ended: sent, refused
// with the service's words, or failed, with Corbie's error, or with why
// no run of it was started, such as its bridge having left the office.
type SignIn struct {
	Thing   int64
	Sent    bool
	Refused string
	Failed  string
}

// Bridges is this area's part of Bulldog's one file, with the office as it
// was last read and the requests it reads and writes.
type Bridges struct {
	// Choose gives a choice to the decision it is on, and gives the one
	// sentence carried back to whoever made it. Decisions sets it. It is
	// called while this area works, and must not call back into it.
	Choose func(Choice) (string, error)
	// Choices gives the choices an entry offers when it is a decision,
	// and nil for any other entry. Decisions sets it. It is called while
	// this area works, and must not call back into it.
	Choices func(requests.Entry) []string
	// SignedIn is told how each sign-in link's carrying ended, once this
	// area's work on it is done, so it may ask for the link to be carried
	// on another bridge. The server sets it.
	SignedIn func(SignIn)

	db       *sql.DB
	requests *requests.Store
	now      func() time.Time
	unwatch  func()

	// mu makes this area's work one thing at a time. told are the sign-in
	// links whose carrying ended while it was held, told of once it is let
	// go.
	mu      sync.Mutex
	o       *office.Office
	scanned bool
	told    []SignIn

	// pending are the requests the store has told of since they were last
	// looked at, kept apart since the store tells of them while it writes.
	pendingMu sync.Mutex
	pending   map[int64]bool
}

// Open opens this area's part of the store, making it the first time, and
// watches the requests for what is to be carried; it writes nothing else.
// A part made by an older Bulldog is refused. Nothing is carried or heard
// until the office has been read with Read.
func Open(db *sql.DB, store *requests.Store, now func() time.Time) (*Bridges, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS parts(area TEXT PRIMARY KEY, version INTEGER)`); err != nil {
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	}
	var version int
	switch err := db.QueryRow(`SELECT version FROM parts WHERE area = ?`, Area).Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		var tables int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'bridges\_%' ESCAPE '\'`).Scan(&tables); err != nil {
			return nil, fmt.Errorf("the store cannot be read: %w", err)
		}
		if tables > 0 {
			return nil, older(0)
		}
		if err := makePart(db); err != nil {
			return nil, fmt.Errorf("the bridges' part of the store cannot be made: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	case version != Version:
		return nil, older(version)
	}
	b := &Bridges{db: db, requests: store, now: now, pending: map[int64]bool{}}
	b.unwatch = store.Watch(b.mark)
	return b, nil
}

// Close ends the watch on the requests.
func (b *Bridges) Close() { b.unwatch() }

// unlock lets this area's work go, and then tells the server how each
// sign-in link whose carrying ended meanwhile ended.
func (b *Bridges) unlock() {
	told := b.told
	b.told = nil
	b.mu.Unlock()
	for _, s := range told {
		if b.SignedIn != nil {
			b.SignedIn(s)
		}
	}
}

func older(version int) error {
	return fmt.Errorf("the bridges' part of the store was kept by an older Bulldog (version %d, and this one keeps version %d), and is not read: remove .bulldog/bulldog.db and start again", version, Version)
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

// mark keeps a request the store told of, to be looked at for what is to
// be carried. It is told while the store writes, so it does nothing more.
func (b *Bridges) mark(n int64) {
	b.pendingMu.Lock()
	b.pending[n] = true
	b.pendingMu.Unlock()
}

// office gives the office as it was last read, or an error before it has
// been read.
func (b *Bridges) office() (*office.Office, error) {
	if b.o == nil {
		return nil, errors.New("the office has not been read, so no bridge is run")
	}
	return b.o, nil
}

// Read takes a new reading of the office, after the requests have taken
// it. What was said before it and not yet looked at is looked at first,
// as the earlier reading reaches people. A person newly reached on a
// bridge, or reached on one newly listed or newly mended, is then carried
// every question and decision that waits on them now, and nothing said
// before. Who was reached before is the earlier reading's, and nothing of
// anyone is kept here besides: the first reading since the server started
// has none before it, so what waits on everyone is looked at again, and
// what was already kept to be carried is kept once. What waits to be
// carried on a bridge no longer in the office is given up, and a run that
// was asked for before then and never named, which was never started, is
// asked for again.
func (b *Bridges) Read(o *office.Office) error {
	b.mu.Lock()
	defer b.unlock()
	before := b.o
	if before != nil {
		if err := b.scan(before); err != nil {
			return err
		}
	} else if err := b.unclaim(); err != nil {
		return err
	}
	b.o = o
	for _, name := range o.Everyone {
		for _, id := range o.People[name].Reached {
			if bridge := o.Bridges[id.Bridge]; bridge == nil || !bridge.Runs() || reached(before, name, id) {
				continue
			}
			if err := b.reach(o, id.Bridge, name); err != nil {
				return err
			}
		}
	}
	waiting, err := b.things(`WHERE state = ? AND run = '' ORDER BY id`, Waiting)
	if err != nil {
		return err
	}
	for _, th := range waiting {
		if o.Bridges[th.bridge] != nil {
			continue
		}
		if err := b.leave(th, GivenUp, stopped(th.bridge, nil)); err != nil {
			return err
		}
	}
	return nil
}

// unclaim has each run asked for and never named, which was never
// started, asked for again: a bridge's listening run, and an out run.
func (b *Bridges) unclaim() error {
	if _, err := b.db.Exec(`UPDATE bridges_bridges SET claimed = 0 WHERE run = ''`); err != nil {
		return failed(err)
	}
	if _, err := b.db.Exec(`UPDATE bridges_things SET state = ? WHERE state = ? AND run = ''`, Waiting, Sending); err != nil {
		return failed(err)
	}
	return nil
}

// reached says whether an earlier reading of the office reached a person
// at an identity on a bridge that ran.
func reached(o *office.Office, person string, id office.Identity) bool {
	if o == nil || o.Bridges[id.Bridge] == nil || !o.Bridges[id.Bridge].Runs() || o.People[person] == nil {
		return false
	}
	return slices.Contains(o.People[person].Reached, id)
}

// reach queues for a person newly reached on a bridge each question and
// decision that waits on them now, where it goes on that bridge: to them
// directly, or to the origin of a request they asked.
func (b *Bridges) reach(o *office.Office, bridge, person string) error {
	waits, err := b.requests.WaitingOn(person)
	if err != nil {
		return err
	}
	for _, w := range waits {
		if w.Entry == 0 {
			continue
		}
		e, err := b.requests.Entry(w.Entry)
		if err != nil {
			return err
		}
		if err := b.queue(o, e, bridge, person); err != nil {
			return err
		}
	}
	return nil
}

// failed is an error of the store, said for whoever reads it.
func failed(err error) error {
	return fmt.Errorf("the bridges' part of the store cannot be read or written: %w", err)
}

// stamp writes an instant as the store keeps it, and unstamp reads one.
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func unstamp(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// pause gives how long to wait before the next run after failures in a
// row: a second, doubling, never more than LongestPause.
func pause(first time.Duration, failures int) time.Duration {
	d := first
	for i := 1; i < failures && d < LongestPause; i++ {
		d *= 2
	}
	return min(d, LongestPause)
}
