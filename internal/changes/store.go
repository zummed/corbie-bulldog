package changes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Area is this part's name in the store, and Schema the shape of its
// tables.
const (
	Area   = "changes"
	Schema = 2
)

// tables makes this part's tables: each request's draft, a file to a row,
// and each change put, by its tool, with the yes kept for a held write
// until its job runs, and how it ended once it has.
const tables = `
CREATE TABLE changes_drafts (
	request INTEGER NOT NULL,
	path    TEXT NOT NULL,
	text    TEXT NOT NULL,
	gone    INTEGER NOT NULL,
	was     TEXT NOT NULL,
	had     INTEGER NOT NULL,
	PRIMARY KEY (request, path)
);
CREATE TABLE changes_puts (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	request INTEGER NOT NULL,
	tool    TEXT NOT NULL,
	put     TEXT NOT NULL,
	yes     TEXT NOT NULL DEFAULT '',
	ended   TEXT NOT NULL DEFAULT '',
	outcome TEXT NOT NULL DEFAULT ''
);
CREATE INDEX changes_puts_waiting ON changes_puts (request, tool, ended);
`

// ready makes this part's tables in a store that has none, and refuses one
// made by another Bulldog.
func ready(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS parts (area TEXT PRIMARY KEY, version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("the store cannot be read: %w", err)
	}
	var version int
	err := db.QueryRowContext(ctx, `SELECT version FROM parts WHERE area = ?`, Area).Scan(&version)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, tables); err != nil {
			return fmt.Errorf("the store's tables of changes cannot be made: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO parts (area, version) VALUES (?, ?)`, Area, Schema); err != nil {
			return err
		}
		return tx.Commit()
	case err != nil:
		return fmt.Errorf("the store cannot be read: %w", err)
	case version != Schema:
		return fmt.Errorf("the store's changes were kept by another Bulldog, in the shape of version %d and not %d: remove .bulldog/bulldog.db and start again", version, Schema)
	}
	return nil
}

// drafted gives a request's draft, a file to each path, in the order of
// their paths.
func drafted(ctx context.Context, q querier, request int64) ([]File, error) {
	rows, err := q.QueryContext(ctx, `SELECT path, text, gone, was, had FROM changes_drafts WHERE request = ? ORDER BY path`, request)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.Text, &f.Gone, &f.Was, &f.Had); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// querier is a store or a transaction of it.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// keepDraft puts one file into a request's draft, with the text it is
// drafted against.
func keepDraft(ctx context.Context, q querier, request int64, f File) error {
	_, err := q.ExecContext(ctx, `INSERT INTO changes_drafts (request, path, text, gone, was, had) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (request, path) DO UPDATE SET text = excluded.text, gone = excluded.gone, was = excluded.was, had = excluded.had`,
		request, f.Path, f.Text, f.Gone, f.Was, f.Had)
	return err
}

// keepTaken adds to a request's draft the files a check added to it: those
// a worker the draft moves takes with it.
func keepTaken(ctx context.Context, q querier, request int64, draft, checked []File) error {
	for _, f := range checked {
		if !slices.ContainsFunc(draft, func(d File) bool { return d.Path == f.Path }) {
			if err := keepDraft(ctx, q, request, f); err != nil {
				return err
			}
		}
	}
	return nil
}

// dropDraft takes one file out of a request's draft.
func dropDraft(ctx context.Context, q querier, request int64, file string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM changes_drafts WHERE request = ? AND path = ?`, request, file)
	return err
}

// keepPut keeps a change put to be decided, and gives its number, which is
// its row's.
func keepPut(ctx context.Context, q querier, p Put) (int64, error) {
	text, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	res, err := q.ExecContext(ctx, `INSERT INTO changes_puts (request, tool, put) VALUES (?, ?, ?)`, p.Request, p.Tool, text)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// gotPut gives a change put, its number its row's, how it ended, ""
// while it waits, and the yes kept for it, if any.
func gotPut(ctx context.Context, q querier, id int64) (Put, Outcome, *Decision, error) {
	var text, yes, ended, outcome string
	err := q.QueryRowContext(ctx, `SELECT put, yes, ended, outcome FROM changes_puts WHERE id = ?`, id).Scan(&text, &yes, &ended, &outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return Put{}, Outcome{}, nil, fmt.Errorf("no change %d was ever put", id)
	}
	if err != nil {
		return Put{}, Outcome{}, nil, err
	}
	return readPut(id, text, yes, ended, outcome)
}

// readPut reads a change put as it is kept.
func readPut(id int64, text, yes, ended, outcome string) (Put, Outcome, *Decision, error) {
	var p Put
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return Put{}, Outcome{}, nil, fmt.Errorf("the change %d put cannot be read: %w", id, err)
	}
	p.ID = id
	var o Outcome
	if ended != "" {
		if err := json.Unmarshal([]byte(outcome), &o); err != nil {
			return Put{}, Outcome{}, nil, fmt.Errorf("how the change %d ended cannot be read: %w", id, err)
		}
	}
	var d *Decision
	if yes != "" {
		d = &Decision{}
		if err := json.Unmarshal([]byte(yes), d); err != nil {
			return Put{}, Outcome{}, nil, fmt.Errorf("the yes kept for the change %d cannot be read: %w", id, err)
		}
	}
	return p, o, d, nil
}

// waitingOf gives the number of the change put by a tool from a request
// that waits for its decision, or nothing.
func waitingOf(ctx context.Context, q querier, request int64, tool string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM changes_puts WHERE request = ? AND tool = ? AND ended = '' AND yes = '' ORDER BY id DESC LIMIT 1`,
		request, tool).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// keepYes keeps the yes on a held write, which the call its job runs
// then writes.
func keepYes(ctx context.Context, q querier, id int64, d Decision) error {
	text, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE changes_puts SET yes = ? WHERE id = ? AND ended = '' AND yes = ''`, text, id)
	return err
}

// approved gives the held write of a tool from a request, with a why,
// that is not yet written, with its yes where it is kept, or nothing.
func approved(ctx context.Context, q querier, request int64, tool, why string) (*Put, *Decision, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, put, yes FROM changes_puts WHERE request = ? AND tool = ? AND ended = '' ORDER BY id DESC`,
		request, tool)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var text, yes string
		if err := rows.Scan(&id, &text, &yes); err != nil {
			return nil, nil, err
		}
		p, _, d, err := readPut(id, text, yes, "", "")
		if err != nil {
			return nil, nil, err
		}
		if p.Why == why {
			return &p, d, nil
		}
	}
	return nil, nil, rows.Err()
}

// endPut keeps how a change put ended. It says whether it was still
// waiting: a change ends once.
func endPut(ctx context.Context, q querier, o Outcome) (bool, error) {
	text, err := json.Marshal(o)
	if err != nil {
		return false, err
	}
	res, err := q.ExecContext(ctx, `UPDATE changes_puts SET ended = ?, outcome = ? WHERE id = ? AND ended = ''`, o.Ended, text, o.Put)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
