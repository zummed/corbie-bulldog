package decisions

import (
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// Change takes a change put to be decided, as Changes hands it on: it is
// a decision of the top team, walked over the deciders it was put with.
// A rule's or a worker's yes refers it on, since only a person approves a
// change. A person's own change, whose walk holds only the rules and
// workers before them, is approved with their own yes once every one has
// referred it. How it ends is handed back to Changes, to write it or not: the
// hook Changes calls gives no context, so what follows is cancelled only
// when the decisions are closed.
func (d *Decisions) Change(p changes.Put) error {
	return d.changing(d.base, func(o *office.Office, fx *effects) error {
		whose := d.requests.Worker(p.By)
		if p.Person {
			whose = d.requests.Person(p.By)
		}
		dec := &Decision{Kind: OfChange, Team: office.Top, Request: p.Request, Whose: whose, Put: &p, Made: d.Now()}
		if err := keepNew(d.db, dec); err != nil {
			return err
		}
		if dec.Request != 0 {
			if err := d.record(o, dec, dec.Does(o), nil); err != nil {
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
	}))
}
