package decisions

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// A Decision is one yes that work waits for, and how far it has gone.
type Decision struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	// Team is the team whose decision it is, and Request the request it
	// serves, 0 for a person's own change from the screens.
	Team    string `json:"team"`
	Request int64  `json:"request,omitzero"`
	// Whose is the worker whose action it is, who said what is to be
	// released, or who drafted the change; for a held job no worker of the
	// office holds, nobody.
	Whose requests.Who `json:"whose"`
	// Does is the action as its tool describes itself, what is to be
	// released, or the change, in a line.
	Does string `json:"does"`

	// Ask is the ask's run that holds the job, and Job the job.
	Ask string `json:"ask,omitzero"`
	Job *Job   `json:"job,omitzero"`
	// Said is what a team would say outside itself.
	Said *Said `json:"said,omitzero"`
	// Put is the change put to be decided.
	Put *changes.Put `json:"put,omitzero"`

	// Step counts the rules and workers asked so far; At is the step the
	// walk stands at now, nil before it starts and once it has ended.
	Step   int    `json:"step"`
	At     *Stand `json:"at,omitzero"`
	Passes []Pass `json:"passes,omitzero"`
	// Asked are the people asked now, and WasAsked everyone ever asked;
	// Entry is the entry they are asked in.
	Asked    []string `json:"asked,omitzero"`
	WasAsked []string `json:"was_asked,omitzero"`
	Entry    int64    `json:"entry,omitzero"`

	// Ended is how it ended, "" while it waits; By is who decided, with
	// their note, at Decided. Why says why it was withdrawn, or why
	// Bulldog refused it.
	Ended   string    `json:"ended,omitzero"`
	By      *Pass     `json:"by,omitzero"`
	Why     string    `json:"why,omitzero"`
	Decided time.Time `json:"decided,omitzero"`
	Made    time.Time `json:"made"`

	// owed is what an ended release or change has still to do beyond this
	// area, kept with it so what Bulldog stopped before doing is done once
	// it is back: "" once all is done.
	owed string
}

// What an ended decision may still owe: how it ended written in the
// request's thread; a released answer to be said and then work told; work
// told how a release ended; Changes told how a change ended.
const (
	oweEntries = "entries"
	oweSay     = "say"
	oweLet     = "let"
	oweDecided = "decided"
)

// A Stand is where a walk stands: a rule or a worker asked, since a time,
// or people.
type Stand struct {
	Kind  string    `json:"kind"`
	Name  string    `json:"name,omitzero"`
	Since time.Time `json:"since"`
}

// A Pass is one decider's word on a decision: a pass with its note, or the
// yes or refusal that ended it.
type Pass struct {
	// Kind is a person, a worker, a rule, or Bulldog's own, and Name is
	// the person's or the worker's name or the rule's path.
	Kind   string    `json:"kind"`
	Name   string    `json:"name,omitzero"`
	Called string    `json:"called"`
	Note   string    `json:"note,omitzero"`
	At     time.Time `json:"at"`
}

// An Ask is an ask's run waiting on its ticket, as work hands it in.
type Ask struct {
	// Run is the ask's run, by which it is known.
	Run string
	// Team and Request are the labels of the run whose job is held.
	Team    string
	Request int64
	// Asking are the machines from the one whose job is held up to the
	// run Bulldog started, by their names.
	Asking []string
	Jobs   []Job
}

// A Job is one job Corbie holds, as the ask is told of it.
type Job struct {
	// Name is the job's name as Corbie gives it, which the ask's answer
	// names; a call of a model's tool is named after the tool.
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitzero"`
	// Asked is what a model's call shows: what its tool does, the
	// arguments, and what the model said with it. A job no model called
	// has none.
	Asked *Asked `json:"asked,omitzero"`
}

// Asked is what the call of a model's tool shows the ask. A call of a
// tool a list: entry gives names the entry, as github, and the tool as its
// server lists it, as create_issue.
type Asked struct {
	Does  string          `json:"does,omitzero"`
	Args  json.RawMessage `json:"args,omitzero"`
	Said  string          `json:"said,omitzero"`
	Entry string          `json:"entry,omitzero"`
	Tool  string          `json:"tool,omitzero"`
}

// JobsOf reads the input Corbie gives an ask: the machine asking, and the
// jobs it holds.
func JobsOf(input json.RawMessage) (asking string, jobs []Job, err error) {
	var given struct {
		Asking struct {
			Machine string `json:"machine"`
		} `json:"asking"`
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(input, &given); err != nil {
		return "", nil, fmt.Errorf("what the ask was given cannot be read: %w", err)
	}
	return given.Asking.Machine, given.Jobs, nil
}

// load reads a decision.
func load(q querier, id int64) (*Decision, error) {
	decs, err := some(q, `id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(decs) == 0 {
		return nil, errNoDecision(id)
	}
	return decs[0], nil
}

// some reads the decisions a query of their rows gives, oldest first.
func some(q querier, query string, args ...any) ([]*Decision, error) {
	rows, err := q.Query(`SELECT id, owed, data FROM decisions_decisions WHERE `+query+` ORDER BY id`, args...)
	if err != nil {
		return nil, failed(err)
	}
	defer rows.Close()
	var decs []*Decision
	for rows.Next() {
		var id int64
		var owed, data string
		if err := rows.Scan(&id, &owed, &data); err != nil {
			return nil, failed(err)
		}
		var dec Decision
		if err := json.Unmarshal([]byte(data), &dec); err != nil {
			return nil, failed(err)
		}
		dec.ID, dec.owed = id, owed
		decs = append(decs, &dec)
	}
	return decs, failedOr(rows.Err())
}

// keepNew keeps a new decision, whole, and gives it its number.
func keepNew(q querier, dec *Decision) error {
	put := int64(0)
	if dec.Put != nil {
		put = dec.Put.ID
	}
	res, err := q.Exec(`INSERT INTO decisions_decisions(kind, request, ask, put, ended, owed, data) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		dec.Kind, dec.Request, dec.Ask, put, dec.Ended, dec.owed, jsonOf(dec))
	if err != nil {
		return failed(err)
	}
	if dec.ID, err = res.LastInsertId(); err != nil {
		return failed(err)
	}
	return nil
}

// save keeps a decision as it now stands.
func save(q querier, dec *Decision) error {
	if _, err := q.Exec(`UPDATE decisions_decisions SET ended = ?, owed = ?, data = ? WHERE id = ?`, dec.Ended, dec.owed, jsonOf(dec), dec.ID); err != nil {
		return failed(err)
	}
	return nil
}

// A querier is the store, or one write to it.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func failed(err error) error {
	return fmt.Errorf("the decisions in the store cannot be read or written: %w", err)
}

func failedOr(err error) error {
	if err != nil {
		return failed(err)
	}
	return nil
}

// ErrNoDecision is what a decision that is not there gives.
var ErrNoDecision = errors.New("there is no such decision")

func errNoDecision(id int64) error { return fmt.Errorf("%w: %d", ErrNoDecision, id) }

// Get gives a decision.
func (d *Decisions) Get(id int64) (*Decision, error) { return load(d.db, id) }

// Waiting gives the decisions that have not ended, oldest first.
func (d *Decisions) Waiting() ([]*Decision, error) { return some(d.db, `ended = ''`) }

// OfRequest gives a request's decisions, oldest first.
func (d *Decisions) OfRequest(n int64) ([]*Decision, error) { return some(d.db, `request = ?`, n) }

// OfEntry gives the decision whose people are asked in an entry, and
// whether there is one, so a bridge carries it with its choices.
func (d *Decisions) OfEntry(entry int64) (*Decision, bool, error) {
	e, err := d.requests.Entry(entry)
	if err != nil {
		return nil, false, err
	}
	if e.Kind != KindDecide {
		return nil, false, nil
	}
	decs, err := d.OfRequest(e.Request)
	if err != nil {
		return nil, false, err
	}
	for _, dec := range decs {
		if dec.Entry == entry {
			return dec, true, nil
		}
	}
	return nil, false, nil
}
