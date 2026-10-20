package decisions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// Held takes the jobs an ask's run holds, each a decision of its own, of
// the team of the worker whose run holds it: the nearest worker of the
// office among the machines asking. Every job is kept as a decision before
// any is asked of anyone. An ask handed in again, as after a restart, is
// given the answer it was given before, if it was answered, and its
// decisions go on where they were.
func (d *Decisions) Held(ctx context.Context, a Ask) error {
	return d.changing(ctx, func(o *office.Office, fx *effects) error {
		var kept string
		switch err := d.db.QueryRow(`SELECT verdict FROM decisions_asks WHERE run = ?`, a.Run).Scan(&kept); {
		case err == nil && kept != "":
			var v Verdict
			if err := json.Unmarshal([]byte(kept), &v); err != nil {
				return failed(err)
			}
			// What its decisions still owe is done first, as after any
			// ending.
			owed, err := some(d.db, `ask = ? AND owed != ''`, a.Run)
			if err != nil {
				return err
			}
			fx.owed = append(fx.owed, owed...)
			fx.verdicts = append(fx.verdicts, verdict{ask: a.Run, v: v})
			return nil
		case err == nil:
			return nil
		case !errors.Is(err, sql.ErrNoRows):
			return failed(err)
		}
		if len(a.Jobs) == 0 {
			return fmt.Errorf("the ask %s holds no job, so there is nothing to decide", a.Run)
		}
		team, whose := a.Team, requests.Who{}
		for _, m := range a.Asking {
			if w := o.Workers[m]; w != nil {
				team, whose = w.Team, d.requests.Worker(m)
				break
			}
		}
		writes, err := d.writes(o, a, whose, fx)
		if err != nil {
			return err
		}
		decs, err := d.keepAll(a, team, whose, writes)
		if err != nil {
			return err
		}
		for _, dec := range decs {
			if dec.Request != 0 {
				if err := d.record(o, dec, d.Does(o, dec), nil); err != nil {
					return err
				}
			}
			if w := writes[dec.Job.Name]; w.refused != "" {
				if err := d.end(dec, Refused, Word{Kind: requests.IsBulldog, Called: "Bulldog", Note: w.refused, At: d.Now()}, fx); err != nil {
					return err
				}
				continue
			}
			if err := d.advance(o, dec, fx); err != nil {
				return err
			}
		}
		return nil
	})
}

// A write is what Changes made of a held write of the files: the change
// it fixed, or why it could not be put.
type write struct {
	put     *changes.Put
	refused string
}

// writes has Changes fix each held job that is a write of the kits
// Bulldog ships, office_put or write_doc of a worker that holds it from
// them, by its job's name.
func (d *Decisions) writes(o *office.Office, a Ask, whose requests.Who, fx *effects) (map[string]write, error) {
	out := map[string]write{}
	w := o.Workers[whose.Name]
	if whose.Kind != requests.IsWorker || w == nil {
		return out, nil
	}
	for _, job := range a.Jobs {
		key := job.key()
		if !changes.Holds(key) || !slices.ContainsFunc(w.Held, func(t office.Tool) bool { return t.Name == key && t.Kind == office.ToolBulldog }) {
			continue
		}
		p, refused, withdrawn, err := d.changes.Held(d.base, changes.Hold{Request: a.Request, Worker: w.Name, Tool: key, Args: job.args()})
		if withdrawn != nil {
			if err := d.withdrawPut(*withdrawn, fx); err != nil {
				return nil, err
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%s's %s could not be put to be decided: %w", w.Name, key, err)
		}
		if p.ID == 0 {
			out[job.Name] = write{refused: refused}
			continue
		}
		out[job.Name] = write{put: &p}
	}
	return out, nil
}

// keepAll keeps an ask and a decision for each job it holds, all of them
// or none.
// A write is a decision of the team whose decision Changes says it is.
func (d *Decisions) keepAll(a Ask, team string, whose requests.Who, writes map[string]write) ([]*Decision, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, failed(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO decisions_asks(run) VALUES (?)`, a.Run); err != nil {
		return nil, failed(err)
	}
	var decs []*Decision
	for _, job := range a.Jobs {
		dec := &Decision{Kind: OfAction, Team: team, Request: a.Request, Whose: whose, Ask: a.Run, Job: &job, Made: d.Now()}
		if p := writes[job.Name].put; p != nil {
			dec.Team, dec.Put = p.Team, p
		}
		if err := keepNew(tx, dec); err != nil {
			return nil, err
		}
		decs = append(decs, dec)
	}
	if err := tx.Commit(); err != nil {
		return nil, failed(err)
	}
	return decs, nil
}

// AskEnded is told that an ask's run ended without being answered: the
// run that held its jobs was cancelled or failed, or the ask gave up
// waiting. Each of its decisions still waiting is withdrawn, and its
// ticket is not answered: it has ended. Of an ask that was answered, it
// changes nothing.
func (d *Decisions) AskEnded(ctx context.Context, run string) error {
	return d.changing(ctx, func(o *office.Office, fx *effects) error {
		decs, err := some(d.db, `ask = ? AND ended = ''`, run)
		if err != nil {
			return err
		}
		for _, dec := range decs {
			if err := d.withdraw(dec, "the run that asked for it ended before it was decided", fx); err != nil {
				return err
			}
		}
		return nil
	})
}

// Stopped withdraws every decision still waiting in requests that were
// stopped, as Requests' Stop gives them: what waited on people in them is
// closed already, and whatever runs on them is cancelled. A held action
// withdrawn so answers no ticket: its ask's run is ended by work
// cancelling the runs of the stopped requests, and AskEnded then changes
// nothing.
func (d *Decisions) Stopped(ctx context.Context, numbers []int64) error {
	return d.changing(ctx, func(o *office.Office, fx *effects) error {
		for _, n := range numbers {
			decs, err := some(d.db, `request = ? AND ended = ''`, n)
			if err != nil {
				return err
			}
			for _, dec := range decs {
				if err := d.withdraw(dec, fmt.Sprintf("request %d was stopped", n), fx); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
