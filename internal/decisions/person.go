package decisions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// A Reply is what a person is told of their answer to a decision, in one
// sentence: that it was taken, or why not.
type Reply struct {
	Taken bool
	Text  string
}

// Ref gives the reference a bridge carries a decision with, and hands
// back with a choice made on it.
func Ref(id int64) string { return "d" + strconv.FormatInt(id, 10) }

// Choices are what a person may answer a decision with.
func Choices() []string { return []string{SaysYes, SaysRefuse} }

// Choose takes a choice a bridge heard, on the decision its reference
// names, from the identity the bridge checked.
func (d *Decisions) Choose(ctx context.Context, ref string, from requests.Who, choice string) (Reply, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(ref, "d"), 10, 64)
	if err != nil || !strings.HasPrefix(ref, "d") {
		return Reply{Text: "That is not a decision of this office, so nothing was taken."}, nil
	}
	return d.Decide(ctx, id, from, choice, "")
}

// Decide takes a person's yes or refusal on a decision, with a note: from
// the person signed in on the screens, or from the identity a bridge
// checked, and from nobody else. They may decide it while it waits on
// them, or before the walk reaches them, when it would. The first answer
// stands; any later one is told who decided and when. An answer that
// cannot be kept is not taken, and the action stays held. The reply is
// the one sentence they are told; an error beside it is Bulldog's own.
func (d *Decisions) Decide(ctx context.Context, id int64, by requests.Who, says, note string) (Reply, error) {
	var reply Reply
	err := d.changing(ctx, func(o *office.Office, fx *effects) error {
		person := d.personOf(o, by)
		if person == "" {
			reply.Text = "You are not a person of this office, so you decide nothing in it."
			return nil
		}
		dec, err := load(d.db, id)
		if errors.Is(err, ErrNoDecision) {
			reply.Text = "There is no such decision, so nothing was taken."
			return nil
		}
		if err != nil {
			reply.Text = "Your answer was not taken, since the decision could not be read: try again."
			return err
		}
		if says != SaysYes && says != SaysRefuse {
			reply.Text = "Nothing was taken: answer yes or refuse, with a note if you like."
			return nil
		}
		if dec.Ended != "" {
			reply.Text = d.already(o, dec)
			return nil
		}
		if !d.mayDecide(o, dec, person) {
			reply.Text = d.notYours(o, dec, person)
			return nil
		}
		ended := Yes
		if says == SaysRefuse {
			ended = Refused
		}
		reply = Reply{Taken: true, Text: fmt.Sprintf("Taken: your %s on %s.", says, lower(dec.Does(o)))}
		err = d.end(dec, ended, d.personWord(o, person, strings.TrimSpace(note)), fx)
		if err != nil && dec.Ended == "" {
			reply = Reply{Text: "Your answer was not taken, since it could not be kept, and nothing was done: try again."}
		}
		return err
	})
	return reply, err
}

// personOf gives the person of the office an answer is from: one signed
// in, while they are still the one of that name, or the person a bridge's
// checked identity names. A guest is nobody.
func (d *Decisions) personOf(o *office.Office, by requests.Who) string {
	switch by.Kind {
	case requests.IsPerson:
		if o.People[by.Name] != nil && by.Known != 0 && d.requests.Person(by.Name).Known == by.Known {
			return by.Name
		}
	case requests.IsIdentity:
		if p := o.Person(by.Bridge, by.ID); p != nil {
			return p.Name
		}
	}
	return ""
}

// mayDecide says whether a person may decide a decision now: one of the
// people it waits on, or of those its walk would reach.
func (d *Decisions) mayDecide(o *office.Office, dec *Decision, person string) bool {
	if dec.At != nil && dec.At.Kind == IsPeople {
		return slices.Contains(dec.Asked, person)
	}
	stages, _ := walk(o, dec)
	return slices.Contains(reach(stages, dec), person)
}

// already tells a later answer how the decision ended.
func (d *Decisions) already(o *office.Office, dec *Decision) string {
	at := dec.Decided.In(o.Zone).Format("15:04")
	if dec.Ended == Withdrawn {
		return fmt.Sprintf("Not taken: it was withdrawn at %s, since %s.", at, dec.Why)
	}
	return fmt.Sprintf("Not taken: it was decided already, %s at %s.", how(dec), at)
}

// notYours tells someone who may not decide a decision why.
func (d *Decisions) notYours(o *office.Office, dec *Decision, person string) string {
	if slices.Contains(dec.WasAsked, person) {
		return "Not taken: you are no longer asked to decide it."
	}
	asked := dec.Asked
	if dec.At == nil || dec.At.Kind != IsPeople {
		stages, _ := walk(o, dec)
		asked = reach(stages, dec)
	}
	if len(asked) == 0 {
		return "Not taken: you do not decide it."
	}
	return fmt.Sprintf("Not taken: you do not decide it; %s does.", strings.ReplaceAll(names(o, asked), ", ", " or "))
}
