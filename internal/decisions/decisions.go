// Package decisions is every yes that work waits for: an action Corbie
// holds, what a team with a disclosure check says outside itself, and a
// change to the office's files. Each is asked of its team's deciders in
// order, rules and workers one at a time and then people all at once, and
// the first yes or refusal stands. What each decider said is an entry in
// the request's thread; the walk itself is kept here, so it goes on where
// it was after a restart. This area never speaks to Corbie: work hands it
// what is held, runs its rules and deciding workers for it, and answers
// Corbie with what it decides.
package decisions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// Area is this part's name in the store, and Version the shape of its
// tables.
const (
	Area    = "decisions"
	Version = 2
)

// The kinds of decision.
const (
	OfAction     = "action"
	OfDisclosure = "disclosure"
	OfChange     = "change"
)

// How a decision ends. One that has not ended has Ended "".
const (
	Yes       = "yes"
	Refused   = "refused"
	Withdrawn = "withdrawn"
)

// What a decider answers: a rule or a worker any of the three, and a
// person yes or refuse alone. The yes of a rule or a worker that advises
// ends nothing, and goes on as a recommendation.
const (
	SaysYes    = "yes"
	SaysRefuse = "refuse"
	SaysRefer  = "refer"
)

// The kinds of entry this area writes in a request's thread.
const (
	// KindDecision is what was done of a decision: what it is, who it
	// waits on, and how it ended.
	KindDecision = "decision"
	// KindDecide is a decision put to people, with what they are shown
	// and how to answer, which waits on them.
	KindDecide = "decide"
	// KindDecided is a decider's yes or refusal, with its note.
	KindDecided = "decided"
	// KindReferred is a rule's or a worker's referral, with its note.
	KindReferred = "referred"
	// KindRecommended is the yes of a rule or a worker that advises,
	// passed on as a recommendation, with its note.
	KindRecommended = "recommended"
	// KindNotice is Bulldog telling someone asked that they are asked no
	// longer, or that a direct message could not be sent.
	KindNotice = "notice"
)

// The limits on a rule's and a deciding worker's run, from its start.
const (
	RuleLimit   = 15 * time.Minute
	WorkerLimit = time.Hour
)

// LabelKind is the label work's runs carry saying what each is for, and
// Deciding its value on a run asked to decide; LabelDecision and LabelStep
// name the decision and its step.
const (
	LabelKind     = "kind"
	Deciding      = "decide"
	LabelDecision = "decision"
	LabelStep     = "step"
)

// ErrNotStarted is what Run's error wraps when the rule or the worker
// could not be started at all, rather than failing once it was.
var ErrNotStarted = errors.New("it could not be started")

// A Start is one rule or worker asked to decide one decision: its run
// belongs to the team decided for, and carries the labels given.
type Start struct {
	Decision int64
	Step     int
	// Kind is office.IsRule or office.IsWorker; Name is the rule's path in
	// the office, or the worker's name.
	Kind string
	Name string
	// Team is the team whose decision it is, whose run it is, and Request
	// the request it serves, 0 for a person's own change from the screens.
	Team    string
	Request int64
	Labels  map[string]string
	// Input is the decision as data, a rule's input; Said is the same in
	// words, as a worker is told it.
	Input map[string]any
	Said  string
}

// A Verdict is how an ask's ticket is answered, once every job it holds is
// decided: the jobs that may run, by their names, and the notes joined.
type Verdict struct {
	Approve []string `json:"approve"`
	Note    string   `json:"note"`
}

// Decisions is an office's decisions: those waiting, and how each ended.
type Decisions struct {
	db       *sql.DB
	requests *requests.Store
	changes  *changes.Changes

	// Now is the clock decisions are timed by.
	Now func() time.Time
	// Run runs one rule or worker asked to decide, and gives what it
	// ended with. A run already started with the same labels, before a
	// restart, is waited on rather than started again. Cancelling the
	// context cancels the run. Work sets it.
	Run func(ctx context.Context, s Start) (json.RawMessage, error)
	// Answer keeps what was decided of every job an ask's run holds, with
	// which work answers the ask's ticket. It is given the same verdict
	// again for an ask handed in again. Work sets it.
	Answer func(ctx context.Context, ask string, v Verdict) error
	// Let tells work how a disclosure ended, so it says what was disclosed
	// and gives a waiting tool its refusal. A disclosure is told once it has
	// ended and is kept, and told again when the office's decisions are
	// opened again if Bulldog stopped, or Let failed, before it was told:
	// work may be told one disclosure twice. Work sets it.
	Let func(ctx context.Context, l Let) error
	// Trouble is told what went wrong where no caller waits to be told,
	// such as in taking a rule's result. The server sets it.
	Trouble func(error)

	// mu is held for each change to a decision, so they come one at a
	// time.
	mu sync.Mutex

	state    sync.Mutex
	office   *office.Office
	running  map[int64]*going
	watchers map[int]func(int64)
	next     int
	base     context.Context
	stop     context.CancelFunc
	runs     sync.WaitGroup
}

// A going is the run of the rule or worker a decision waits on now.
type going struct {
	step   int
	cancel context.CancelFunc
}

const schema = `
CREATE TABLE decisions_decisions(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL, request INTEGER NOT NULL, ask TEXT NOT NULL DEFAULT '', put INTEGER NOT NULL DEFAULT 0,
	ended TEXT NOT NULL DEFAULT '', owed TEXT NOT NULL DEFAULT '', data TEXT NOT NULL);
CREATE INDEX decisions_decisions_ask ON decisions_decisions(ask);
CREATE INDEX decisions_decisions_open ON decisions_decisions(ended);
CREATE INDEX decisions_decisions_owed ON decisions_decisions(owed);
CREATE TABLE decisions_asks(run TEXT PRIMARY KEY, verdict TEXT NOT NULL DEFAULT '');
`

// Open opens the decisions' part of the store, making it the first time,
// and has the requests hand it each answer of a person a disclosure check
// holds. A part made by an older Bulldog is refused. Nothing is decided
// until the office has been read with Read.
func Open(db *sql.DB, store *requests.Store, ch *changes.Changes) (*Decisions, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS parts(area TEXT PRIMARY KEY, version INTEGER)`); err != nil {
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	}
	var version int
	switch err := db.QueryRow(`SELECT version FROM parts WHERE area = ?`, Area).Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		var tables int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'decisions\_%' ESCAPE '\'`).Scan(&tables); err != nil {
			return nil, fmt.Errorf("the store cannot be read: %w", err)
		}
		if tables > 0 {
			return nil, older(0)
		}
		if err := makePart(db); err != nil {
			return nil, fmt.Errorf("the decisions' part of the store cannot be made: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("the store cannot be read: %w", err)
	case version != Version:
		return nil, older(version)
	}
	base, stop := context.WithCancel(context.Background())
	d := &Decisions{db: db, requests: store, changes: ch, Now: time.Now, running: map[int64]*going{}, base: base, stop: stop}
	store.Held = d.held
	return d, nil
}

func older(version int) error {
	return fmt.Errorf("the decisions in the store were kept by an older Bulldog (version %d, and this one keeps version %d), and are not read: remove .bulldog/bulldog.db and start again", version, Version)
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

// Close cancels the runs this Decisions waits on, as a server stopping
// does, and waits until each has given up; no run is waited on after it.
// The runs go on in Corbie, and their results are read when the office's
// decisions are opened again.
func (d *Decisions) Close() {
	d.state.Lock()
	d.stop()
	d.state.Unlock()
	d.runs.Wait()
}

// look gives the office as it was last read.
func (d *Decisions) look() (*office.Office, error) {
	d.state.Lock()
	defer d.state.Unlock()
	if d.office == nil {
		return nil, errors.New("the office has not been read, so nothing can be decided")
	}
	return d.office, nil
}

// trouble reports what went wrong where no caller waits to be told.
func (d *Decisions) trouble(err error) {
	if err == nil {
		return
	}
	if d.Trouble != nil {
		d.Trouble(err)
		return
	}
	fmt.Fprintln(os.Stderr, "bulldog: decisions:", err)
}

func jsonOf(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
