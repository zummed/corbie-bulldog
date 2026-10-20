package work

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// Area is this part's name in the store, and Version the shape of its
// tables.
const (
	Area    = "work"
	Version = 7
)

// How a run noted in the store stands.
const (
	going     = "going"     // noted, and sent or about to be
	done      = "done"      // ended with an output
	failed    = "failed"    // ended failing, out of time or refused
	cancelled = "cancelled" // ended cancelled in Corbie
	lost      = "lost"      // Corbie never took it up, or has no record of it
	unsent    = "unsent"    // its start could not be sent
)

// stopped is why a run is stopped when a person stopped its request, or
// its request was closed at its source: its request says so already.
const stopped = "stopped"

const schema = `
CREATE TABLE work_runs(
	name TEXT PRIMARY KEY,
	kind TEXT NOT NULL, team TEXT NOT NULL, request INTEGER NOT NULL DEFAULT 0,
	worker TEXT NOT NULL DEFAULT '', labels TEXT NOT NULL,
	seen INTEGER NOT NULL DEFAULT 0, since INTEGER NOT NULL DEFAULT 0,
	noted TEXT NOT NULL, started TEXT NOT NULL DEFAULT '', ended TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL, how TEXT NOT NULL DEFAULT '',
	stop TEXT NOT NULL DEFAULT '', cancel_sent INTEGER NOT NULL DEFAULT 0,
	tokens INTEGER NOT NULL DEFAULT 0,
	again INTEGER NOT NULL DEFAULT 0);
CREATE INDEX work_runs_request ON work_runs(request);
CREATE INDEX work_runs_status ON work_runs(status);
CREATE INDEX work_runs_labels ON work_runs(labels);
CREATE TABLE work_requests(
	request INTEGER PRIMARY KEY,
	seen INTEGER NOT NULL DEFAULT 0,
	pending INTEGER NOT NULL DEFAULT 0,
	again INTEGER NOT NULL DEFAULT 0,
	acted INTEGER NOT NULL DEFAULT 0,
	told TEXT NOT NULL DEFAULT '');
CREATE TABLE work_calls(
	run TEXT PRIMARY KEY,
	request INTEGER NOT NULL, tool TEXT NOT NULL, worker TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL, mark INTEGER NOT NULL DEFAULT 0,
	entry INTEGER NOT NULL DEFAULT 0, sub INTEGER NOT NULL DEFAULT 0,
	put INTEGER NOT NULL DEFAULT 0, decision INTEGER NOT NULL DEFAULT 0,
	answer TEXT NOT NULL DEFAULT '');
CREATE INDEX work_calls_request ON work_calls(request);
CREATE INDEX work_calls_put ON work_calls(put);
CREATE TABLE work_given(entry INTEGER PRIMARY KEY, request INTEGER NOT NULL);
CREATE INDEX work_given_request ON work_given(request);
CREATE TABLE work_subs(sub INTEGER PRIMARY KEY, given INTEGER NOT NULL DEFAULT 0, open INTEGER NOT NULL DEFAULT 1);
CREATE TABLE work_lets(id INTEGER PRIMARY KEY, told TEXT NOT NULL, done TEXT NOT NULL DEFAULT '');
CREATE TABLE work_outcomes(id INTEGER PRIMARY KEY, told TEXT NOT NULL, done TEXT NOT NULL DEFAULT '');
CREATE TABLE work_verdicts(run TEXT PRIMARY KEY, told TEXT NOT NULL, done TEXT NOT NULL DEFAULT '');
CREATE TABLE work_rounds(
	clock TEXT NOT NULL, ticket TEXT NOT NULL,
	request INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY(clock, ticket));
`

// open makes this area's part of the store the first time, and refuses a
// part made by an older Bulldog.
func open(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS parts(area TEXT PRIMARY KEY, version INTEGER)`); err != nil {
		return fmt.Errorf("the store cannot be read: %w", err)
	}
	var version int
	switch err := db.QueryRow(`SELECT version FROM parts WHERE area = ?`, Area).Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		var tables int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'work\_%' ESCAPE '\'`).Scan(&tables); err != nil {
			return fmt.Errorf("the store cannot be read: %w", err)
		}
		if tables > 0 {
			return older(0)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("the work's part of the store cannot be made: %w", err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec(schema); err != nil {
			return fmt.Errorf("the work's part of the store cannot be made: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO parts(area, version) VALUES (?, ?)`, Area, Version); err != nil {
			return fmt.Errorf("the work's part of the store cannot be made: %w", err)
		}
		return tx.Commit()
	case err != nil:
		return fmt.Errorf("the store cannot be read: %w", err)
	case version != Version:
		return older(version)
	}
	return nil
}

func older(version int) error {
	return fmt.Errorf("the runs in the store were kept by an older Bulldog (version %d, and this one keeps version %d), and are not read: remove .bulldog/bulldog.db and start again", version, Version)
}

// A note is a run as this area noted it, before its start was sent.
type note struct {
	Name    string
	Kind    string
	Team    string
	Request int64
	// Worker is the worker, the rule's path in the office, or the bridge
	// it is a run of.
	Worker string
	Labels map[string]string
	// Seen is the last entry of its request's thread when it was noted:
	// what came after is new to the next run.
	Seen int64
	// Since is what its request was dealt with up to when it was noted:
	// what came after was new to it.
	Since   int64
	Noted   time.Time
	Started time.Time
	Ended   time.Time
	Status  string
	How     string
	// Stop says why it is being stopped, "" while it is not; CancelSent
	// that the cancel has gone to Corbie.
	Stop       string
	CancelSent bool
	Tokens     int
	// Again says it was started on a decider's refusal of what its
	// team would have said.
	Again bool
}

// ended says the run has ended, or never started.
func (n *note) ended() bool { return n.Status != going }

// labelsKey writes labels as they are kept, the same for the same labels.
func labelsKey(labels map[string]string) string {
	b, err := json.Marshal(labels)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// stamp writes an instant as the store keeps it; unstamp reads it back.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func unstamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("the time %q kept in the store cannot be read: %w", s, err)
	}
	return t, nil
}

func storeFailed(err error) error { return fmt.Errorf("the work's part of the store failed: %w", err) }

// keep notes a run before its start is sent, and tells whoever watches
// its request.
func (w *Work) keep(n *note) error {
	_, err := w.db.Exec(`INSERT INTO work_runs(name, kind, team, request, worker, labels, seen, since, noted, status, again)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, n.Name, n.Kind, n.Team, n.Request, n.Worker, labelsKey(n.Labels), n.Seen, n.Since, stamp(n.Noted), going, n.Again)
	if err != nil {
		return storeFailed(err)
	}
	if n.Request != 0 {
		w.requests.Tell(n.Request)
	}
	return nil
}

// sent keeps that a lead's run on a request was started, once its start
// has gone: the request has nothing waiting to start or let through by its
// heed. What was given to waiting tools is kept, so what was new for each
// run can be worked out again when it is shown.
func (w *Work) sent(n *note) error {
	if _, err := w.db.Exec(`INSERT INTO work_requests(request, pending, again, acted) VALUES (?, 0, 0, 0)
		ON CONFLICT(request) DO UPDATE SET pending = 0, again = 0, acted = 0`, n.Request); err != nil {
		return storeFailed(err)
	}
	return nil
}

const noteColumns = `SELECT name, kind, team, request, worker, labels, seen, since, noted, started, ended, status, how, stop, cancel_sent, tokens, again FROM work_runs`

func scanNotes(rows *sql.Rows, err error) ([]*note, error) {
	if err != nil {
		return nil, storeFailed(err)
	}
	defer rows.Close()
	var notes []*note
	for rows.Next() {
		n := &note{}
		var labels, noted, started, ended string
		if err := rows.Scan(&n.Name, &n.Kind, &n.Team, &n.Request, &n.Worker, &labels, &n.Seen, &n.Since, &noted, &started, &ended,
			&n.Status, &n.How, &n.Stop, &n.CancelSent, &n.Tokens, &n.Again); err != nil {
			return nil, storeFailed(err)
		}
		if err := json.Unmarshal([]byte(labels), &n.Labels); err != nil {
			return nil, storeFailed(err)
		}
		for _, at := range []struct {
			to   *time.Time
			from string
		}{{&n.Noted, noted}, {&n.Started, started}, {&n.Ended, ended}} {
			if *at.to, err = unstamp(at.from); err != nil {
				return nil, storeFailed(err)
			}
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, storeFailed(err)
	}
	return notes, nil
}

// noted gives the run noted under a name, or nil.
func (w *Work) noted(name string) (*note, error) {
	notes, err := scanNotes(w.db.Query(noteColumns+` WHERE name = ?`, name))
	if err != nil || len(notes) == 0 {
		return nil, err
	}
	return notes[0], nil
}

// notedAs gives the latest run noted with exactly these labels that was
// sent, or nil.
func (w *Work) notedAs(labels map[string]string) (*note, error) {
	notes, err := scanNotes(w.db.Query(noteColumns+` WHERE labels = ? AND status != ? ORDER BY rowid DESC LIMIT 1`, labelsKey(labels), unsent))
	if err != nil || len(notes) == 0 {
		return nil, err
	}
	return notes[0], nil
}

// goingNotes gives every run noted that has not ended, oldest first.
func (w *Work) goingNotes() ([]*note, error) {
	return scanNotes(w.db.Query(noteColumns+` WHERE status = ? ORDER BY rowid`, going))
}

// notesOf gives the runs of a request, oldest first.
func (w *Work) notesOf(request int64, kind string) ([]*note, error) {
	return scanNotes(w.db.Query(noteColumns+` WHERE request = ? AND kind = ? ORDER BY rowid`, request, kind))
}

// An ending is how a run ended, as it is kept with it.
type ending struct {
	status, how    string
	started, ended time.Time
	tokens         int
}

// execer is the store, or a write being made in it.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// endIn keeps how a run that was going ended, and says whether it was
// going: an ending kept already is not kept again.
func endIn(db execer, name string, e ending) (bool, error) {
	res, err := db.Exec(`UPDATE work_runs SET status = ?, how = ?, started = ?, ended = ?, tokens = ? WHERE name = ? AND status = ?`,
		e.status, e.how, stamp(e.started), stamp(e.ended), e.tokens, name, going)
	if err != nil {
		return false, storeFailed(err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return false, storeFailed(err)
	}
	return changed == 1, nil
}

// end keeps how a run ended, once: it gives false for a run whose ending
// was kept already.
func (w *Work) end(name string, e ending) (bool, error) { return endIn(w.db, name, e) }

// endWork keeps how a run on a request ended, once, and marks its request
// to be looked at for its next run, in one write, which whoever watches
// the request is told of, the work among them; for a heed that let through
// what it weighed, acted keeps that the lead starts on what is new.
func (w *Work) endWork(n *note, e ending, acted bool) (bool, error) {
	tx, err := w.db.Begin()
	if err != nil {
		return false, storeFailed(err)
	}
	defer tx.Rollback()
	if first, err := endIn(tx, n.Name, e); err != nil || !first {
		return false, err
	}
	if _, err := tx.Exec(`INSERT INTO work_requests(request, pending, acted) VALUES (?, 1, ?)
		ON CONFLICT(request) DO UPDATE SET pending = 1, acted = max(acted, excluded.acted)`, n.Request, acted); err != nil {
		return false, storeFailed(err)
	}
	if err := tx.Commit(); err != nil {
		return false, storeFailed(err)
	}
	w.requests.Tell(n.Request)
	return true, nil
}

// endStray keeps how a run ended whose start was taken as not sent, and
// which Corbie ran all the same: it is kept with that run, which stays as
// not sent, and changes nothing else.
func (w *Work) endStray(name string, e ending) error {
	if _, err := w.db.Exec(`UPDATE work_runs SET how = ?, started = ?, ended = ?, tokens = ? WHERE name = ? AND status = ?`,
		e.how, stamp(e.started), stamp(e.ended), e.tokens, name, unsent); err != nil {
		return storeFailed(err)
	}
	return nil
}

// stopping marks the runs of requests going as being stopped, for why,
// before any cancel is sent.
func (w *Work) stopping(requests []int64, why string) error {
	for _, n := range requests {
		if _, err := w.db.Exec(`UPDATE work_runs SET stop = ? WHERE request = ? AND status = ? AND stop = ''`, why, n, going); err != nil {
			return storeFailed(err)
		}
	}
	return nil
}

// stopRun marks one run as being stopped, for why.
func (w *Work) stopRun(name, why string) error {
	if _, err := w.db.Exec(`UPDATE work_runs SET stop = ? WHERE name = ? AND status = ? AND stop = ''`, why, name, going); err != nil {
		return storeFailed(err)
	}
	return nil
}

// toCancel gives the runs being stopped whose cancel has not been sent.
func (w *Work) toCancel() ([]*note, error) {
	return scanNotes(w.db.Query(noteColumns+` WHERE status = ? AND stop != '' AND cancel_sent = 0 ORDER BY rowid`, going))
}

func (w *Work) cancelSent(name string) error {
	if _, err := w.db.Exec(`UPDATE work_runs SET cancel_sent = 1 WHERE name = ?`, name); err != nil {
		return storeFailed(err)
	}
	return nil
}

// mark gives the last entry of a request already dealt with: given to a
// lead's run that was sent, or answered or noted without one.
func (w *Work) mark(request int64) (int64, error) {
	var seen int64
	err := w.db.QueryRow(`SELECT max(coalesce((SELECT max(seen) FROM work_runs WHERE request = ? AND kind = ? AND status != ?), 0),
		coalesce((SELECT seen FROM work_requests WHERE request = ?), 0))`, request, KindWork, unsent, request).Scan(&seen)
	if err != nil {
		return 0, storeFailed(err)
	}
	return seen, nil
}

// dealt keeps a request's thread as dealt with up to an entry, with no
// run, and clears it from those to look at.
func (w *Work) dealt(request, seen int64) error {
	if _, err := w.db.Exec(`INSERT INTO work_requests(request, seen, pending, again, acted) VALUES (?, ?, 0, 0, 0)
		ON CONFLICT(request) DO UPDATE SET seen = max(seen, excluded.seen), pending = 0, again = 0, acted = 0`, request, seen); err != nil {
		return storeFailed(err)
	}
	return nil
}

// pend has a request looked at for its next run, also after a restart.
func (w *Work) pend(request int64) error {
	if _, err := w.db.Exec(`INSERT INTO work_requests(request, pending) VALUES (?, 1)
		ON CONFLICT(request) DO UPDATE SET pending = 1`, request); err != nil {
		return storeFailed(err)
	}
	return nil
}

// refused has a request's lead started again on a decider's refusal of
// its answer, also after a restart.
func (w *Work) refused(request int64) error {
	if _, err := w.db.Exec(`INSERT INTO work_requests(request, pending, again) VALUES (?, 1, 1)
		ON CONFLICT(request) DO UPDATE SET pending = 1, again = 1`, request); err != nil {
		return storeFailed(err)
	}
	return nil
}

// isAgain says whether a request's lead is to start again on a refusal,
// and acted whether its heed let through what is new, so its lead starts
// on it.
func (w *Work) isAgain(request int64) (again, acted bool, err error) {
	err = w.db.QueryRow(`SELECT again, acted FROM work_requests WHERE request = ?`, request).Scan(&again, &acted)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, false, storeFailed(err)
	}
	return again, acted, nil
}

// pendingRequests gives the requests whose run ended and which have not
// been looked at for their next.
func (w *Work) pendingRequests() ([]int64, error) {
	return w.numbers(`SELECT request FROM work_requests WHERE pending = 1 ORDER BY request`)
}

// told gives the problem a request's team was last told it cannot run
// for, and keepTold keeps it.
func (w *Work) told(request int64) (string, error) {
	var told string
	err := w.db.QueryRow(`SELECT told FROM work_requests WHERE request = ?`, request).Scan(&told)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", storeFailed(err)
	}
	return told, nil
}

func (w *Work) keepTold(request int64, told string) error {
	if _, err := w.db.Exec(`INSERT INTO work_requests(request, told) VALUES (?, ?)
		ON CONFLICT(request) DO UPDATE SET told = excluded.told`, request, told); err != nil {
		return storeFailed(err)
	}
	return nil
}

// How a call kept in the store stands.
const (
	// doing: it is being done, and what it did may not be kept yet.
	doing = "doing"
	// waiting: it waits on a person, another team or a change's decision.
	waiting = "waiting"
	// held: what it would say waits on its team's disclosure check.
	held = "held"
	// answered: its answer is kept, and given whenever it is seen waiting.
	answered = "answered"
)

// A callRow is a call of a shipped tool, or an ask, as it is kept while
// its run waits: what it did and the answer kept for it.
type callRow struct {
	Run     string
	Request int64
	Tool    string
	Worker  string
	State   string
	// Mark is the last entry of the request's thread when the call began:
	// what it put is after it.
	Mark int64
	// Entry is the question or the tell it put, Sub the sub-request it
	// made, Put the change it put, and Decision the disclosure its words wait
	// on.
	Entry    int64
	Sub      int64
	Put      int64
	Decision int64
	Answer   string
}

const callColumns = `SELECT run, request, tool, worker, state, mark, entry, sub, put, decision, answer FROM work_calls`

func scanCalls(rows *sql.Rows, err error) ([]*callRow, error) {
	if err != nil {
		return nil, storeFailed(err)
	}
	defer rows.Close()
	var calls []*callRow
	for rows.Next() {
		c := &callRow{}
		if err := rows.Scan(&c.Run, &c.Request, &c.Tool, &c.Worker, &c.State, &c.Mark, &c.Entry, &c.Sub, &c.Put, &c.Decision, &c.Answer); err != nil {
			return nil, storeFailed(err)
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, storeFailed(err)
	}
	return calls, nil
}

// callOf gives the call kept for a run, or nil.
func (w *Work) callOf(run string) (*callRow, error) {
	calls, err := scanCalls(w.db.Query(callColumns+` WHERE run = ?`, run))
	if err != nil || len(calls) == 0 {
		return nil, err
	}
	return calls[0], nil
}

// callsOf gives the calls kept for a request's runs, oldest first.
func (w *Work) callsOf(request int64) ([]*callRow, error) {
	return scanCalls(w.db.Query(callColumns+` WHERE request = ? ORDER BY rowid`, request))
}

// callOfPut gives the call that put a change, or nil.
func (w *Work) callOfPut(put int64) (*callRow, error) {
	calls, err := scanCalls(w.db.Query(callColumns+` WHERE put = ?`, put))
	if err != nil || len(calls) == 0 {
		return nil, err
	}
	return calls[0], nil
}

// everyCall gives every call kept.
func (w *Work) everyCall() ([]*callRow, error) {
	return scanCalls(w.db.Query(callColumns + ` ORDER BY rowid`))
}

// keepCall keeps where a call stands, in place of what was kept.
func (w *Work) keepCall(c *callRow) error {
	_, err := w.db.Exec(`INSERT INTO work_calls(run, request, tool, worker, state, mark, entry, sub, put, decision, answer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run) DO UPDATE SET request = excluded.request, tool = excluded.tool, worker = excluded.worker, state = excluded.state,
			mark = excluded.mark, entry = excluded.entry, sub = excluded.sub, put = excluded.put, decision = excluded.decision,
			answer = excluded.answer`,
		c.Run, c.Request, c.Tool, c.Worker, c.State, c.Mark, c.Entry, c.Sub, c.Put, c.Decision, c.Answer)
	if err != nil {
		return storeFailed(err)
	}
	return nil
}

// answerCall keeps a call's answer, with the entry given to it, which so
// starts no run, or the answer of the sub-request it made, which so is
// given nowhere else: all in one write. A call refused before it was kept
// is kept with its answer.
func (w *Work) answerCall(c *callRow, answer string, given *requests.Entry) error {
	tx, err := w.db.Begin()
	if err != nil {
		return storeFailed(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO work_calls(run, request, tool, worker, state, mark, entry, sub, put, decision, answer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run) DO UPDATE SET state = excluded.state, answer = excluded.answer`,
		c.Run, c.Request, c.Tool, c.Worker, answered, c.Mark, c.Entry, c.Sub, c.Put, c.Decision, answer); err != nil {
		return storeFailed(err)
	}
	switch {
	case given == nil:
	case given.Request == c.Request:
		if _, err := tx.Exec(`INSERT INTO work_given(entry, request) VALUES (?, ?) ON CONFLICT(entry) DO NOTHING`, given.ID, given.Request); err != nil {
			return storeFailed(err)
		}
	default:
		if _, err := tx.Exec(`INSERT INTO work_subs(sub, given) VALUES (?, ?) ON CONFLICT(sub) DO UPDATE SET given = excluded.given`,
			given.Request, given.ID); err != nil {
			return storeFailed(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return storeFailed(err)
	}
	c.State, c.Answer = answered, answer
	return nil
}

// dropCall lets a call's kept answer go, once its run has ended; an
// entry given to it that it never took is given to the next run instead.
func (w *Work) dropCall(run string, untaken int64) error {
	tx, err := w.db.Begin()
	if err != nil {
		return storeFailed(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM work_calls WHERE run = ?`, run); err != nil {
		return storeFailed(err)
	}
	if untaken != 0 {
		if _, err := tx.Exec(`DELETE FROM work_given WHERE entry = ?`, untaken); err != nil {
			return storeFailed(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return storeFailed(err)
	}
	return nil
}

// givenTo gives the entries of a request given to a waiting tool, which
// start no run.
func (w *Work) givenTo(request int64) (map[int64]bool, error) {
	rows, err := w.db.Query(`SELECT entry FROM work_given WHERE request = ?`, request)
	if err != nil {
		return nil, storeFailed(err)
	}
	defer rows.Close()
	given := map[int64]bool{}
	for rows.Next() {
		var e int64
		if err := rows.Scan(&e); err != nil {
			return nil, storeFailed(err)
		}
		given[e] = true
	}
	if err := rows.Err(); err != nil {
		return nil, storeFailed(err)
	}
	return given, nil
}

// subGiven gives the last answer of a sub-request given to the run that
// asked it or to the request above; keepSub keeps a sub-request made, and
// subDone how far its answers were given.
func (w *Work) subGiven(sub int64) (int64, error) {
	var given int64
	err := w.db.QueryRow(`SELECT given FROM work_subs WHERE sub = ?`, sub).Scan(&given)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, storeFailed(err)
	}
	return given, nil
}

func (w *Work) keepSub(sub int64) error {
	if _, err := w.db.Exec(`INSERT INTO work_subs(sub) VALUES (?) ON CONFLICT(sub) DO NOTHING`, sub); err != nil {
		return storeFailed(err)
	}
	return nil
}

func (w *Work) subDone(sub, given int64, open bool) error {
	if _, err := w.db.Exec(`INSERT INTO work_subs(sub, given, open) VALUES (?, ?, ?)
		ON CONFLICT(sub) DO UPDATE SET given = max(given, excluded.given), open = excluded.open`, sub, given, open); err != nil {
		return storeFailed(err)
	}
	return nil
}

// openSubs gives the sub-requests whose answers may not all have been
// given.
func (w *Work) openSubs() ([]int64, error) {
	return w.numbers(`SELECT sub FROM work_subs WHERE open = 1 ORDER BY sub`)
}

func (w *Work) numbers(query string, args ...any) ([]int64, error) {
	rows, err := w.db.Query(query, args...)
	if err != nil {
		return nil, storeFailed(err)
	}
	defer rows.Close()
	var ns []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return nil, storeFailed(err)
		}
		ns = append(ns, n)
	}
	if err := rows.Err(); err != nil {
		return nil, storeFailed(err)
	}
	return ns, nil
}

// keepOwed keeps what another area told work, to be done on work's own
// goroutine: how a disclosure ended, in work_lets by its decision, or how a
// change put ended, in work_outcomes by its put. Told twice, it is kept
// once, and done once.
func (w *Work) keepOwed(table string, id int64, told any) error {
	b, err := json.Marshal(told)
	if err != nil {
		return err
	}
	if _, err := w.db.Exec(`INSERT INTO `+table+`(id, told) VALUES (?, ?) ON CONFLICT(id) DO NOTHING`, id, string(b)); err != nil {
		return storeFailed(err)
	}
	return nil
}

// owed gives what is kept in a table of what is owed and not yet done.
func (w *Work) owed(table string) (map[int64]string, error) {
	rows, err := w.db.Query(`SELECT id, told FROM ` + table + ` WHERE done = ''`)
	if err != nil {
		return nil, storeFailed(err)
	}
	defer rows.Close()
	owed := map[int64]string{}
	for rows.Next() {
		var id int64
		var told string
		if err := rows.Scan(&id, &told); err != nil {
			return nil, storeFailed(err)
		}
		owed[id] = told
	}
	if err := rows.Err(); err != nil {
		return nil, storeFailed(err)
	}
	return owed, nil
}

// paid keeps what was owed as done, and lets go what was done longer ago
// than a shipped tool waits: what told work, told it again only before
// then.
func (w *Work) paid(table string, id int64) error {
	now := w.clock.Now()
	if _, err := w.db.Exec(`UPDATE `+table+` SET done = ? WHERE id = ?`, stamp(now), id); err != nil {
		return storeFailed(err)
	}
	if _, err := w.db.Exec(`DELETE FROM `+table+` WHERE done != '' AND done < ?`, stamp(now.Add(-ToolWait))); err != nil {
		return storeFailed(err)
	}
	return nil
}

// keepVerdict keeps what the decisions decided of an ask, by the ask's
// run, to be given to its ticket: told again, it is given again.
func (w *Work) keepVerdict(run string, v decisions.Verdict) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.db.Exec(`INSERT INTO work_verdicts(run, told) VALUES (?, ?)
		ON CONFLICT(run) DO UPDATE SET told = excluded.told, done = ''`, run, string(b)); err != nil {
		return storeFailed(err)
	}
	return nil
}

// verdictsOwed gives the verdicts kept and not yet given, by the ask's
// run.
func (w *Work) verdictsOwed() (map[string]decisions.Verdict, error) {
	rows, err := w.db.Query(`SELECT run, told FROM work_verdicts WHERE done = ''`)
	if err != nil {
		return nil, storeFailed(err)
	}
	defer rows.Close()
	owed := map[string]decisions.Verdict{}
	for rows.Next() {
		var run, told string
		if err := rows.Scan(&run, &told); err != nil {
			return nil, storeFailed(err)
		}
		var v decisions.Verdict
		if err := json.Unmarshal([]byte(told), &v); err != nil {
			return nil, fmt.Errorf("the verdict kept for the ask %s cannot be read: %w", run, err)
		}
		owed[run] = v
	}
	if err := rows.Err(); err != nil {
		return nil, storeFailed(err)
	}
	return owed, nil
}

// verdictGiven keeps a verdict as given, and lets go those given longer
// ago than a shipped tool waits.
func (w *Work) verdictGiven(run string) error {
	now := w.clock.Now()
	if _, err := w.db.Exec(`UPDATE work_verdicts SET done = ? WHERE run = ?`, stamp(now), run); err != nil {
		return storeFailed(err)
	}
	if _, err := w.db.Exec(`DELETE FROM work_verdicts WHERE done != '' AND done < ?`, stamp(now.Add(-ToolWait))); err != nil {
		return storeFailed(err)
	}
	return nil
}

// round gives the request a coming round of a routine's clock opened, by
// the clock's run and the ticket it waits on, and whether it was noted:
// a round let go by opened none.
func (w *Work) round(clock, ticket string) (int64, bool, error) {
	var request int64
	err := w.db.QueryRow(`SELECT request FROM work_rounds WHERE clock = ? AND ticket = ?`, clock, ticket).Scan(&request)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, storeFailed(err)
	}
	return request, true, nil
}

// keepRound notes a coming round with the request it opened, or 0, and
// lets go the rounds of the clock before it.
func (w *Work) keepRound(clock, ticket string, request int64) error {
	tx, err := w.db.Begin()
	if err != nil {
		return storeFailed(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM work_rounds WHERE clock = ?`, clock); err != nil {
		return storeFailed(err)
	}
	if _, err := tx.Exec(`INSERT INTO work_rounds(clock, ticket, request) VALUES (?, ?, ?)`, clock, ticket, request); err != nil {
		return storeFailed(err)
	}
	if err := tx.Commit(); err != nil {
		return storeFailed(err)
	}
	return nil
}

// dropRounds lets go what was noted of a clock's rounds, once it has
// ended.
func (w *Work) dropRounds(clock string) error {
	if _, err := w.db.Exec(`DELETE FROM work_rounds WHERE clock = ?`, clock); err != nil {
		return storeFailed(err)
	}
	return nil
}
