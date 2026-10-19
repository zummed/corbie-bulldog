package decisions

import (
	"context"
	"fmt"
	"slices"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// What a team says outside itself, which its disclosure check holds.
const (
	SaysAnswer   = "answer"
	SaysQuestion = "question"
	SaysTell     = "tell"
	SaysSub      = "sub"
)

// Said is something a team with a disclosure check would say outside itself:
// its answer, a question or a tell to someone outside it, or a
// sub-request it would make.
type Said struct {
	What string `json:"what"`
	// Request is the request it is said in, of the team that says it.
	Request int64 `json:"request"`
	// By is the worker or the person who said it: for an answer, the
	// team's lead.
	By requests.Who `json:"by"`
	// To is whom a question or a tell is to, and Team the team a
	// sub-request would ask.
	To   requests.Who `json:"to,omitzero"`
	Team string       `json:"team,omitzero"`
	Text string       `json:"text"`
	// Files are an answer's files.
	Files []requests.File `json:"files,omitzero"`
	// Held is the entry the answer of a lead who is a person is held as.
	Held int64 `json:"held,omitzero"`
	// Ticket is the ticket of the tool that waits on it, which work
	// answers; "" for an answer.
	Ticket string `json:"ticket,omitzero"`
}

// Let is how a disclosure ended, as work is told it: disclosed, to be said
// as it was decided, or refused, by whom, with their note.
type Let struct {
	Decision  int64
	Said      Said
	Disclosed bool
	By        string
	Note      string
}

// Text gives a refused disclosure as the tool that said it is answered: not
// disclosed, by whom, with the note.
func (l Let) Text() string {
	if l.Disclosed {
		return ""
	}
	return fmt.Sprintf("Your %s was not disclosed: %s refused it%s It was not said, and nobody outside your team was told of it.",
		whatOf(l.Said.What), l.By, noted(l.Note))
}

func whatOf(what string) string {
	if what == SaysSub {
		return "question to another team"
	}
	return what
}

// Disclose holds what a team with a disclosure check would say outside
// itself until its disclosure check lets it go. A disclosed answer is written
// as the request's answer exactly as it was said; anything else disclosed,
// and every refusal, is told to work by Let. A refused disclosure goes back,
// as the decider's entry, to the team's lead, and a refused answer makes
// the request working again. It gives the decision.
func (d *Decisions) Disclose(ctx context.Context, s Said) (int64, error) {
	var id int64
	err := d.changing(ctx, func(o *office.Office, fx *effects) error {
		var err error
		id, err = d.keepDisclosure(o, s, fx)
		return err
	})
	return id, err
}

// keepDisclosure keeps a disclosure and walks it, and gives it. The held
// answer of a lead who is a person is handed once, and handed again
// gives the disclosure it was first handed to; one whose team no longer
// sets a disclosure check is let go by the walk, since the requests hold
// it already.
func (d *Decisions) keepDisclosure(o *office.Office, s Said, fx *effects) (int64, error) {
	r, err := d.requests.Get(s.Request)
	if err != nil {
		return 0, err
	}
	if s.What != SaysAnswer && s.What != SaysQuestion && s.What != SaysTell && s.What != SaysSub {
		return 0, fmt.Errorf("%q is not something a team says outside itself", s.What)
	}
	if s.Held != 0 {
		decs, err := some(d.db, `request = ? AND kind = ?`, s.Request, OfDisclosure)
		if err != nil {
			return 0, err
		}
		if i := slices.IndexFunc(decs, func(dec *Decision) bool { return dec.Said.Held == s.Held }); i >= 0 {
			return decs[i].ID, nil
		}
	} else if t := o.Teams[r.Team]; t != nil && !t.HasDisclosure {
		return 0, fmt.Errorf("%s sets no disclosure check, so nothing it says is held", capital(inSentence(t.Called)))
	}
	dec := &Decision{Kind: OfDisclosure, Team: r.Team, Request: s.Request, Whose: s.By, Said: &s, Made: d.Now(),
		Does: fmt.Sprintf("%s of %s, to disclose", capital(whatOf(s.What)), inSentence(called(o, r.Team)))}
	if err := keepNew(d.db, dec); err != nil {
		return 0, err
	}
	return dec.ID, d.advance(o, dec, fx)
}

// heldSaid gives the answer of a lead who is a person, which the
// requests hold for its team's disclosure check, as the check is handed it.
func heldSaid(e requests.Entry) Said {
	return Said{What: SaysAnswer, Request: e.Request, By: e.By, Text: e.Text, Held: e.ID}
}

// held hands the answer of a lead who is a person, which the requests
// hold for its team's disclosure check, to the check, however it was said.
// Once the disclosure is kept it is handed, and what goes wrong after is
// trouble. One that Bulldog stopped before handing is handed when the
// office is read.
func (d *Decisions) held(e *requests.Entry) error {
	id, err := d.Disclose(d.base, heldSaid(*e))
	if id != 0 {
		d.trouble(err)
		return nil
	}
	return err
}

// say says the answer a disclosure let go, as the request's answer, exactly
// as it was held.
func (d *Decisions) say(dec *Decision) error {
	s := dec.Said
	var err error
	if s.Held != 0 {
		_, err = d.requests.Disclose(s.Held)
	} else {
		_, err = d.requests.Answer(s.Request, s.By, s.Text, s.Files)
	}
	if err != nil {
		return fmt.Errorf("the answer in request %d was disclosed, but could not be said: %w", s.Request, err)
	}
	return nil
}

// letOf gives how a disclosure ended, as work is told it.
func letOf(dec *Decision) Let {
	return Let{Decision: dec.ID, Said: *dec.Said, Disclosed: dec.Ended == Yes, By: dec.By.Called, Note: dec.By.Note}
}

// NotDisclosed gives the one line whoever asked a team is told when it
// does not disclose its answer, naming the team and who refused it last:
// "payments did not disclose its answer (Lee)". It is there only while the
// latest answer the team would have given in the request was refused, and
// not while a later one waits on its disclosure check.
func (d *Decisions) NotDisclosed(n int64) (string, bool, error) {
	decs, err := some(d.db, `request = ? AND kind = ?`, n, OfDisclosure)
	if err != nil {
		return "", false, err
	}
	for _, dec := range slices.Backward(decs) {
		if dec.Said.What != SaysAnswer || dec.Ended == Withdrawn {
			continue
		}
		if dec.Ended != Refused {
			return "", false, nil
		}
		return fmt.Sprintf("%s did not disclose its answer (%s)", dec.Team, dec.By.Called), true, nil
	}
	return "", false, nil
}
