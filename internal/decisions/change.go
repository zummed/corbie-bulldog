package decisions

import (
	"encoding/json"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// Change takes a person's own change, as Changes hands it on: it is
// decided as an office_put of theirs would be, a held action of the team
// whose decision Changes says it is, walked over its office_put list or
// else its everything. Once every rule and worker before the first people
// has referred it or recommended yes, it is approved with the person's own
// yes where they are among those people, and asked of those people where
// they are not. How it ends is handed back to Changes, to write it or not:
// the hook Changes calls gives no context, so what follows is cancelled
// only when the decisions are closed.
func (d *Decisions) Change(p changes.Put) error {
	return d.changing(d.base, func(o *office.Office, fx *effects) error {
		args := jsonOf(map[string]string{"why": p.Why})
		job := &Job{Name: changes.ToolPut, Input: json.RawMessage(args), Asked: &Asked{Does: "Put a change to the office", Args: json.RawMessage(args)}}
		dec := &Decision{Kind: OfAction, Team: p.Team, Request: p.Request, Whose: d.requests.Person(p.By), Job: job, Put: &p, Made: d.Now()}
		if err := keepNew(d.db, dec); err != nil {
			return err
		}
		if dec.Request != 0 {
			if err := d.record(o, dec, d.Does(o, dec), nil); err != nil {
				return err
			}
		}
		return d.advance(o, dec, fx)
	})
}

// Told is told how each change put ended, as Changes tells it: one
// withdrawn while it waits, because its draft was put again, is withdrawn
// here too. A change that ended any other way was ended here.
func (d *Decisions) Told(out changes.Outcome) {
	if out.Ended != changes.Withdrawn {
		return
	}
	d.trouble(d.changing(d.base, func(o *office.Office, fx *effects) error {
		return d.withdrawPut(out, fx)
	}))
}

// withdrawPut withdraws the decisions still waiting on a change put that
// was withdrawn.
func (d *Decisions) withdrawPut(out changes.Outcome, fx *effects) error {
	decs, err := some(d.db, `put = ? AND ended = ''`, out.Put)
	if err != nil {
		return err
	}
	for _, dec := range decs {
		if err := d.withdraw(dec, out.Why, fx); err != nil {
			return err
		}
	}
	return nil
}
