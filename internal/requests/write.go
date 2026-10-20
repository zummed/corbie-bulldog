package requests

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// Each write below is one thing done, kept whole or not at all, and the
// watchers are told of it once it is kept.

// AskInChat makes a request a person asks in the built-in chat, to the
// team they pick, or the office.
func (s *Store) AskInChat(person, team, text string, files []File) (*Request, error) {
	if team == "" {
		team = office.Top
	}
	var r *Request
	err := s.write(func(c *change) error {
		if c.o.People[person] == nil {
			return refuse("%s is not a person of the office, so nothing was asked", person)
		}
		var err error
		r, err = c.ask(team, c.person(person), Origin{Kind: InChat}, "", Entry{Text: text, In: Screens}, files)
		return err
	})
	return r, err
}

// AskAt makes a request of what a bridge heard said in a place it listens
// in, from the identity it checked: to the team the listed place names,
// with where it was said, and the link of the source thing it is about if
// any, as its origin, and whether the bridge marked it as naming its own
// bot. From a place not listed, or from a guest where guests may not ask,
// it makes nothing.
func (s *Store) AskAt(bridge, heard, place, about, from, text string, files []File, mentioned bool) (*Request, error) {
	var r *Request
	err := s.write(func(c *change) error {
		p, ok := c.o.Place(bridge, heard)
		if !ok {
			return refuse("%s does not listen in %s, so nothing was asked", bridge, heard)
		}
		asker := Identity(bridge, from)
		if c.personOf(asker) == "" && !p.Guests {
			return refuse("%s on %s matches no person of the office, and guests may not ask in %s, so nothing was asked", from, bridge, heard)
		}
		if place == "" {
			place = heard
		}
		var err error
		r, err = c.ask(p.Team, asker, Origin{Kind: AtPlace, Bridge: bridge, Heard: heard, Place: place, About: about}, "",
			Entry{Text: text, In: Where{Bridge: bridge, Place: place}, Mentioned: mentioned}, files)
		return err
	})
	return r, err
}

// AskByRoutine makes the request a team's routine asks when its time
// comes, or when a person runs it now, who is by: asked by the team, with
// the routine as its origin, and who ran it, said to the worker the
// routine names. At its time, by is no one.
func (s *Store) AskByRoutine(team string, routine office.Routine, by Who) (*Request, error) {
	var r *Request
	err := s.write(func(c *change) error {
		switch w := c.o.Workers[routine.To]; {
		case w == nil || w.Team != team:
			return refuse("the routine at %s is said to %s, which is not a worker of %s, so nothing was asked", routine.At, routine.To, team)
		case !w.Runs():
			return refuse("the routine at %s is said to %s, which cannot run until its file is mended, so nothing was asked", routine.At, routine.To)
		case by != Who{} && (by.Kind != IsPerson || c.personOf(by) == ""):
			return refuse("a routine is run now only by a person of the office, so nothing was asked")
		}
		origin := Origin{Kind: ByRoutine, Team: team, Known: c.era(IsTeam, team), At: routine.At, To: routine.To, Say: routine.Say, By: by}
		var err error
		r, err = c.ask(team, c.team(team), origin, routine.To, Entry{Text: routine.Say}, nil)
		return err
	})
	return r, err
}

// ask makes a request to a team that is in the office and has a lead
// that can answer for it, with its asking as its first entry: what was
// asked, where it came in and whether it was marked mentioned.
func (c *change) ask(team string, asker Who, origin Origin, worker string, asking Entry, files []File) (*Request, error) {
	if err := c.askable(team); err != nil {
		return nil, err
	}
	r := &Request{Team: team, TeamKnown: c.era(IsTeam, team), Asker: asker, Origin: origin, Worker: worker}
	if err := c.keep(r); err != nil {
		return nil, err
	}
	asking.Kind, asking.By, asking.To = KindAsked, asker, []Who{teamOf(r)}
	_, err := c.add(r, asking, files, nil)
	return r, err
}

// askable refuses a team that is not in the office, or has no lead that
// can answer for it: a person of the team, or a worker of it that runs.
func (c *change) askable(team string) error {
	t := c.o.Teams[team]
	if t == nil {
		return refuse("%s is not a team of the office, so nothing was asked", team)
	}
	switch w := c.o.Workers[t.Lead]; {
	case c.o.People[t.Lead] != nil && slices.Contains(t.People, t.Lead):
		return nil
	case w == nil || w.Team != team:
		return refuse("%s has no lead that can answer for it, so nothing was asked", t.Called)
	case !w.Runs():
		return refuse("%s is the lead of %s, and cannot run until its file is mended, so nothing was asked", t.Lead, t.Called)
	}
	return nil
}

// Sub makes a sub-request: what a worker of a request's team asks another
// team, kept with the Corbie ticket its run waits on, with the question as
// its first entry and an entry in the asking request that leads to it. A
// ticket that already has one gives that one, and makes no second.
func (s *Store) Sub(parent int64, by Who, team, question, ticket string) (*Request, error) {
	var r *Request
	err := s.write(func(c *change) error {
		if ticket == "" {
			return errors.New("a sub-request is kept with the Corbie ticket its asking run waits on, and none was given")
		}
		switch n, ok, err := c.ofTicket(ticket); {
		case err != nil:
			return err
		case ok:
			r, err = c.req(n)
			return err
		}
		above, err := c.req(parent)
		if err != nil {
			return err
		}
		switch {
		case !c.workerOf(above, by):
			return refuse("%s is not a worker of %s, which request %d went to, so nothing was asked", c.calledOf(by), called(c.o, above.Team), parent)
		case above.Stopped:
			return refuse("request %d was stopped, so nothing more is asked in it", parent)
		case by.Team == team:
			return refuse("%s is %s's own team, and inside a team workers use each other as tools, so nothing was asked", team, by.Name)
		}
		if err := c.askable(team); err != nil {
			return err
		}
		r = &Request{Team: team, TeamKnown: c.era(IsTeam, team), Asker: by, Parent: parent, Ticket: ticket}
		if err := c.keep(r); err != nil {
			return err
		}
		to := []Who{teamOf(r)}
		if _, err := c.add(r, Entry{Kind: KindAsked, By: by, To: to, Text: question}, nil, nil); err != nil {
			return err
		}
		_, err = c.add(above, Entry{Kind: KindSub, By: by, To: to, Text: question, Sub: r.Number}, nil, nil)
		return err
	})
	return r, err
}

// Say writes what someone who may say more in a request says in it, as an
// entry to its team, which makes an answered or stopped request working
// again. What its lead says, when that is a person, is its answer, or,
// for a team that sets a disclosure check, an answer held until the check
// lets it go, which is what was read of the team, and is handed to the
// check by Held once it is kept. One that could not be handed is given
// with the error, and is held no longer, so the lead is waited on again.
// To someone who may not see it the request is not there.
func (s *Store) Say(n int64, by Who, text string, files []File, in Where, mentioned bool) (*Entry, error) {
	e, err := s.say(n, by, text, files, in, mentioned)
	if err != nil || e.Kind != KindHeld || s.Held == nil {
		return e, err
	}
	if err := s.Held(e); err != nil {
		return e, errors.Join(fmt.Errorf("what was said is kept as what was read of the team, but could not be handed to its disclosure check, so it is not said, and the lead is waited on again: %w", err), s.unhold(e))
	}
	return e, nil
}

func (s *Store) say(n int64, by Who, text string, files []File, in Where, mentioned bool) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		may, err := c.maySay(r, by, in)
		if err != nil {
			return err
		}
		if !may {
			return c.mayNotSay(r, by)
		}
		if p := c.personOf(by); p != "" && c.leadPerson(r) == p {
			answer := Entry{Kind: KindAnswer, By: by, To: []Who{r.Asker}, Text: text, In: in, Mentioned: mentioned}
			if c.o.Teams[r.Team].HasDisclosure {
				answer = Entry{Kind: KindHeld, By: by, To: []Who{teamOf(r)}, Text: text, In: in, Read: r.Team, ReadKnown: r.TeamKnown, Mentioned: mentioned}
			}
			if e, err = c.add(r, answer, files, nil); err != nil {
				return err
			}
			r.Answered, r.Stopped, r.Held = answer.Kind == KindAnswer, false, answer.Kind == KindHeld
			return c.set(r)
		}
		to := []Who{teamOf(r)}
		read, known := c.layer(r, by, to, in)
		e, err = c.add(r, Entry{Kind: KindSaid, By: by, To: to, Text: text, In: in, Read: read, ReadKnown: known, Mentioned: mentioned}, files, nil)
		if err != nil {
			return err
		}
		if by.Kind == IsWorker {
			return c.workerSaid(r)
		}
		r.Answered, r.Stopped = false, false
		return c.set(r)
	})
	return e, err
}

// unhold has an answer that was held, and could not be handed to its
// disclosure check, held no longer, unless a later one is held in its place.
func (s *Store) unhold(held *Entry) error {
	return s.write(func(c *change) error {
		r, err := c.req(held.Request)
		if err != nil {
			return err
		}
		es, err := c.entries(r.Number)
		if err != nil {
			return err
		}
		if !r.Held || slices.ContainsFunc(es, func(e Entry) bool { return e.Kind == KindHeld && e.ID > held.ID }) {
			return nil
		}
		r.Held = false
		return c.set(r)
	})
}

// mayNotSay refuses someone who may not say more in a request, saying why;
// to one who sees nothing of it, it is not there.
func (c *change) mayNotSay(r *Request, by Who) error {
	switch by.Kind {
	case IsPerson, IsIdentity:
		if err := c.unseen(r, by); err != nil {
			return err
		}
		return refuse("you may not say more in request %d: you see it from the request it was asked from", r.Number)
	case IsWorker:
		return refuse("%s may not say more in request %d: of workers, only those of the team that asked it do, through their runs", by.Name, r.Number)
	}
	return refuse("%s says nothing more in a request: only people do, and the workers of a team that asked one", c.calledOf(by))
}

// workerSaid makes a sub-request working again once a worker of the team
// that asked it has said more in it. One that was stopped is opened again
// only while the request above it is not stopped too: a run that a stop
// above cancelled may still say something, and that opens nothing.
func (c *change) workerSaid(r *Request) error {
	above, err := c.req(r.Parent)
	if err != nil {
		return err
	}
	if r.Stopped && !above.Stopped {
		r.Answered, r.Stopped = false, false
		return c.set(r)
	}
	return c.working(r)
}

// Put writes a question or a tell by a worker of a request's team, to its
// asker or to a person of its team: a question waits on them until they
// answer. One for anyone else is refused, and nobody is asked in their
// place. What is put makes an answered request working again.
func (s *Store) Put(n int64, by Who, kind string, to Who, text string, files []File) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		switch {
		case kind != KindQuestion && kind != KindTold:
			return fmt.Errorf("%s is neither a question nor a tell", kind)
		case !c.workerOf(r, by):
			return refuse("%s is not a worker of %s, so it puts nothing to anyone in request %d", c.calledOf(by), called(c.o, r.Team), n)
		case kind == KindQuestion && r.Stopped:
			return refuse("request %d was stopped, so nothing more is asked in it", n)
		}
		if c.same(to, r.Asker) {
			if r.Asker.Kind != IsIdentity && r.Asker.Kind != IsPerson {
				return refuse("the asker of request %d is %s, which reads only the answer", n, c.calledOf(r.Asker))
			}
		} else if p := c.personOf(to); p == "" || !c.isOf(r.Team, r.TeamKnown, p) {
			return refuse("%s is neither a person of %s nor the asker of request %d, so nothing was said, and nobody was asked in their place",
				c.calledOf(to), called(c.o, r.Team), n)
		}
		read, known := c.layer(r, by, []Who{to}, Where{})
		var waits []Who
		if kind == KindQuestion {
			waits = []Who{to}
		}
		if e, err = c.add(r, Entry{Kind: kind, By: by, To: []Who{to}, Text: text, Read: read, ReadKnown: known}, files, waits); err != nil {
			return err
		}
		return c.working(r)
	})
	return e, err
}

// Reply writes someone's answer to what was put to them, which closes it,
// to whoever put it, and makes the request working again. A guest is
// reached at the origin alone: to a guest's answer from anywhere else, the
// request is not there, as it is to what they say.
func (s *Store) Reply(entry int64, by Who, text string, files []File, in Where) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		put, err := c.entry(entry)
		if err != nil {
			return err
		}
		r, err := c.req(put.Request)
		if err != nil {
			return err
		}
		if by.Kind == IsIdentity && c.personOf(by) == "" && !c.atOrigin(r, in) {
			return ErrNotThere
		}
		puts, err := c.puts(r.Number)
		if err != nil {
			return err
		}
		if !c.toThem(put, puts, c.personOf(by)) && !slices.Contains(put.To, by) {
			if err := c.unseen(r, by); err != nil {
				return err
			}
		}
		if put.Kind != KindQuestion {
			return refuse("entry %d is a %s, not a question: it is answered where it was put", entry, put.Kind)
		}
		ws, err := c.waits(r.Number)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(ws, func(w wait) bool { return w.entry == entry && c.same(w.who, by) }) {
			if !c.toThem(put, puts, c.personOf(by)) && !slices.Contains(put.To, by) {
				return ErrNotThere
			}
			return refuse("nothing put to you waits there any more: it was answered or closed")
		}
		e, err = c.add(r, Entry{Kind: KindReplied, By: by, To: []Who{put.By}, Text: text, In: in, Read: put.Read, ReadKnown: put.ReadKnown,
			Answers: entry}, files, nil)
		if err != nil {
			return err
		}
		if err := c.close(entry); err != nil {
			return err
		}
		return c.working(r)
	})
	return e, err
}

// Answer writes a request's answer by its lead, a worker, to its asker,
// and the request is answered, or, while something put to someone in it
// still waits, waits on them. What is open beneath it goes on. An empty
// answer to a request asked by a routine or at a channel place says
// nothing: the request is answered with no entry, and Answer gives none. An answer to a request that was
// stopped is kept, and opens nothing.
func (s *Store) Answer(n int64, by Who, text string, files []File) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		if !c.isLead(r, by) {
			return refuse("%s does not answer request %d, so its answer was not kept", c.calledOf(by), n)
		}
		if !r.Origin.Quiet() || strings.TrimSpace(text) != "" || len(files) > 0 {
			e, err = c.add(r, Entry{Kind: KindAnswer, By: by, To: []Who{r.Asker}, Text: text}, files, nil)
		}
		if err != nil || r.Stopped {
			return err
		}
		r.Answered = true
		return c.set(r)
	})
	return e, err
}

// Noted writes what a team's heed said of the messages it noted, the
// asking or more said: for each, an entry by the rule to the team,
// answering it and holding the rule's note, or else that the rule noted
// it, which is what was read of the team and goes to nobody. The request
// is answered with nothing said, as by an answer, and a stopped one is
// left as it is. A message is never changed, and one noted already is
// passed over, so writing it again writes nothing.
func (s *Store) Noted(n int64, rule Who, note string, messages []int64) ([]Entry, error) {
	var noted []Entry
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		if rule.Kind != IsRule {
			return fmt.Errorf("%s is no rule, so it notes nothing", c.calledOf(rule))
		}
		es, err := c.entries(n)
		if err != nil {
			return err
		}
		text := cmp.Or(strings.TrimSpace(note), "Noted by "+rule.Name)
		for _, id := range messages {
			i := slices.IndexFunc(es, func(e Entry) bool { return e.ID == id })
			switch {
			case i < 0:
				return fmt.Errorf("entry %d is not in request %d, so it is not noted", id, n)
			case es[i].Kind != KindAsked && es[i].Kind != KindSaid:
				return fmt.Errorf("entry %d of request %d is a %s, not a message, so it is not noted", id, n, es[i].Kind)
			case es[i].Noted:
				continue
			}
			e, err := c.add(r, Entry{Kind: KindNoted, By: rule, To: []Who{teamOf(r)}, Text: text, Read: r.Team, ReadKnown: r.TeamKnown,
				Answers: id}, nil, nil)
			if err != nil {
				return err
			}
			es[i].Noted = true
			noted = append(noted, *e)
		}
		if len(noted) == 0 || r.Stopped {
			return nil
		}
		r.Answered = true
		return c.set(r)
	})
	return noted, err
}

// isLead says whether someone is the worker who answers a request: the
// one its routine names, or else its team's lead.
func (c *change) isLead(r *Request, by Who) bool {
	t := c.teamNow(r.Team, r.TeamKnown)
	return t != nil && c.workerOf(r, by) && by.Name == cmp.Or(r.Worker, t.Lead)
}

// Disclose lets go an answer its team's disclosure check held, of a lead who
// is a person: it becomes the request's answer, by them, with its files,
// and is what was done.
func (s *Store) Disclose(held int64) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		put, err := c.entry(held)
		if err != nil {
			return err
		}
		r, err := c.req(put.Request)
		if err != nil {
			return err
		}
		es, err := c.entries(r.Number)
		if err != nil {
			return err
		}
		if put.Kind != KindHeld || !r.Held || slices.ContainsFunc(es, func(e Entry) bool { return e.Kind == KindHeld && e.ID > put.ID }) {
			return refuse("entry %d is not the answer request %d holds, so nothing was let go", held, r.Number)
		}
		files, err := c.files(held)
		if err != nil {
			return err
		}
		if e, err = c.add(r, Entry{Kind: KindAnswer, By: put.By, To: []Who{r.Asker}, Text: put.Text, In: put.In}, files, nil); err != nil {
			return err
		}
		r.Answered, r.Held = true, false
		return c.set(r)
	})
	return e, err
}

// files reads the files kept on an entry.
func (c *change) files(entry int64) ([]File, error) {
	rows, err := c.tx.Query(`SELECT name, data FROM requests_files WHERE entry = ? ORDER BY rowid`, entry)
	if err != nil {
		return nil, failed(err)
	}
	defer rows.Close()
	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Name, &f.Data); err != nil {
			return nil, failed(err)
		}
		files = append(files, f)
	}
	return files, failedOr(rows.Err())
}

// Late keeps a sub-request's answer that came when no run waited for it,
// as an entry in the request that asked, to the worker that asked, and
// makes that request working again. Each answer is kept so once: given
// again, it gives the entry kept, and writes nothing.
func (s *Store) Late(sub int64) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		r, err := c.req(sub)
		if err != nil {
			return err
		}
		if r.Parent == 0 {
			return fmt.Errorf("request %d is no sub-request, so its answer is no late one", sub)
		}
		es, err := c.entries(sub)
		if err != nil {
			return err
		}
		answer := c.answerOf(r, es)
		if answer == nil {
			return fmt.Errorf("request %d has no answer yet", sub)
		}
		above, err := c.req(r.Parent)
		if err != nil {
			return err
		}
		kept, err := c.entries(above.Number)
		if err != nil {
			return err
		}
		// Entries are numbered in the order they are written, so a late
		// entry of this sub-request written after its answer was written
		// for that answer.
		if i := slices.IndexFunc(kept, func(k Entry) bool { return k.Kind == KindLate && k.Sub == sub && k.ID > answer.ID }); i >= 0 {
			e = &kept[i]
			return nil
		}
		e, err = c.add(above, Entry{Kind: KindLate, By: answer.By, To: []Who{r.Asker}, Text: answer.Text, Sub: sub}, nil, nil)
		if err != nil {
			return err
		}
		return c.working(above)
	})
	return e, err
}

// Failed writes that a request's run ended with no answer because it
// failed: to the team, what failed, which is what was read of it and told
// to its people as they are when it is shown or sent; and to its asker,
// only that the team could not answer, unless the request was stopped,
// whose last word to its asker is who stopped it. With nothing else open
// in it, the request is answered.
func (s *Store) Failed(n int64, why string) error {
	return s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		team := called(c.o, r.Team)
		if _, err := c.add(r, Entry{Kind: KindBulldog, By: Bulldog, To: []Who{teamOf(r)}, Text: fmt.Sprintf("%s's work failed: %s", team, why),
			Read: r.Team, ReadKnown: r.TeamKnown}, nil, nil); err != nil {
			return err
		}
		if r.Stopped {
			return nil
		}
		if _, err := c.add(r, Entry{Kind: KindBulldog, By: Bulldog, To: []Who{r.Asker}, Text: team + " could not answer."}, nil, nil); err != nil {
			return err
		}
		open, err := c.open(r)
		if err != nil || open {
			return err
		}
		r.Answered = true
		return c.set(r)
	})
}

// teamOf gives the team a request went to, as the one it went to, which a
// team written later under the same name is not.
func teamOf(r *Request) Who { return Who{Kind: IsTeam, Name: r.Team, Known: r.TeamKnown} }

// open says whether anything is open in a request: something put to
// someone, or a sub-request not answered.
func (c *change) open(r *Request) (bool, error) {
	ws, err := c.waits(0)
	if err != nil {
		return false, err
	}
	subs, err := c.subs(r.Number)
	return slices.ContainsFunc(ws, func(w wait) bool { return w.request == r.Number }) ||
		slices.ContainsFunc(subs, func(s *Request) bool { return !c.answered(s, ws) }), err
}

// Stop stops a request and every sub-request still open beneath it, an
// answered one in which something still waits among them, and nothing
// above it: what is put to anyone in them is closed, and each is answered
// with Bulldog's entry saying who stopped it. It gives the requests it
// stopped, whose runs are to be cancelled.
func (s *Store) Stop(n int64, by Who) ([]int64, error) {
	var stopped []int64
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		text := "stopped by " + c.calledOf(by)
		stopped, err = c.stop(r, text, text)
		return err
	})
	return stopped, err
}

// StopWith answers a request as Bulldog with a line of its own, as when its
// team did not disclose its answer: it is stopped as Stop stops it, by
// Bulldog, but its asker is told only the line, and what is stopped
// beneath it is stopped by Bulldog. It gives the requests it stopped,
// whose runs are to be cancelled.
func (s *Store) StopWith(n int64, line string) ([]int64, error) {
	var stopped []int64
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		stopped, err = c.stop(r, line, "stopped by "+c.calledOf(Bulldog))
		return err
	})
	return stopped, err
}

// stop stops a request, its asker told text, and what is open beneath it,
// each of their askers told below.
func (c *change) stop(r *Request, text, below string) ([]int64, error) {
	stopped := []int64{r.Number}
	if err := c.stopOne(r, text); err != nil {
		return nil, err
	}
	ws, err := c.waits(0)
	if err != nil {
		return nil, err
	}
	under := []int64{r.Number}
	for len(under) > 0 {
		subs, err := c.subs(under[0])
		if err != nil {
			return nil, err
		}
		under = under[1:]
		for _, sub := range subs {
			under = append(under, sub.Number)
			if c.answered(sub, ws) {
				continue
			}
			if err := c.stopOne(sub, below); err != nil {
				return nil, err
			}
			stopped = append(stopped, sub.Number)
		}
	}
	return stopped, nil
}

func (c *change) stopOne(r *Request, text string) error {
	if err := c.closeAll(r.Number); err != nil {
		return err
	}
	if _, err := c.add(r, Entry{Kind: KindBulldog, By: Bulldog, To: []Who{r.Asker}, Text: text}, nil, nil); err != nil {
		return err
	}
	r.Answered, r.Stopped, r.Held = true, true, false
	return c.set(r)
}

// Source keeps the status a bridge reported for the thing at a place, in
// place of the one before, for the latest request whose origin it is. It
// is no entry and starts nothing, but for the thing being closed, which
// stops the request's work as a stop does. It gives the requests stopped.
func (s *Store) Source(bridge, place string, st Status) ([]int64, error) {
	var stopped []int64
	err := s.write(func(c *change) error {
		n, ok, err := c.latestAt(bridge, place)
		if err != nil || !ok {
			return err
		}
		r, err := c.req(n)
		if err != nil {
			return err
		}
		was := r.Source
		r.Source = &st
		if err := c.set(r); err != nil {
			return err
		}
		if st.Closed && (was == nil || !was.Closed) {
			text := fmt.Sprintf("closed at %s by %s", bridge, c.calledOf(st.By))
			stopped, err = c.stop(r, text, text)
		}
		return err
	})
	return stopped, err
}

// Readers keeps the identities a bridge reported as reading a place, in
// place of those before. A thing handed in with no readers reports none,
// and leaves them as they were.
func (s *Store) Readers(bridge, place string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.write(func(c *change) error {
		if _, err := c.tx.Exec(`INSERT INTO requests_readers(bridge, place, ids) VALUES (?, ?, ?)
			ON CONFLICT(bridge, place) DO UPDATE SET ids = excluded.ids`, bridge, place, jsonOf(ids)); err != nil {
			return failed(err)
		}
		rows, err := c.tx.Query(`SELECT number FROM requests_requests WHERE bridge = ? AND place = ? ORDER BY number`, bridge, place)
		if err != nil {
			return failed(err)
		}
		defer rows.Close()
		for rows.Next() {
			var n int64
			if err := rows.Scan(&n); err != nil {
				return failed(err)
			}
			c.touch(n)
		}
		return failedOr(rows.Err())
	})
}

// A Writing is an entry as another area writes it, a decision's among
// them: what it is, whether it is what was read of the request's team or
// what was done, the open entry it answers, which it closes, and whom it
// waits on. Working says it starts the request's lead again: an answered
// request is working again, and a held answer of a lead who is a person
// is no longer held, so they are waited on again.
type Writing struct {
	Kind    string
	By      Who
	To      []Who
	Text    string
	Files   []File
	In      Where
	Read    bool
	Answers int64
	Waits   []Who
	Working bool
}

// Write writes an entry as another area gives it. Whom it is to who is a
// person is a person of the office now, or its asker; whom it waits on
// are people of the office now, and never a guest. What waits on someone
// makes an answered request working again, and nothing waits in a request
// that was stopped.
func (s *Store) Write(n int64, w Writing) (*Entry, error) {
	var e *Entry
	err := s.write(func(c *change) error {
		r, err := c.req(n)
		if err != nil {
			return err
		}
		if err := c.recipients(r, w.To); err != nil {
			return err
		}
		if err := c.persons(w.Waits); err != nil {
			return err
		}
		if len(w.Waits) > 0 && r.Stopped {
			return refuse("request %d was stopped, so nothing more is put to anyone in it", n)
		}
		if w.Answers != 0 {
			put, err := c.entry(w.Answers)
			if err != nil {
				return err
			}
			if put.Request != n {
				return fmt.Errorf("entry %d is not in request %d", w.Answers, n)
			}
		}
		entry := Entry{Kind: w.Kind, By: w.By, To: w.To, Text: w.Text, In: w.In, Answers: w.Answers}
		if w.Read {
			entry.Read, entry.ReadKnown = r.Team, r.TeamKnown
		}
		if e, err = c.add(r, entry, w.Files, w.Waits); err != nil {
			return err
		}
		if w.Answers != 0 {
			if err := c.close(w.Answers); err != nil {
				return err
			}
		}
		if w.Working && r.Held {
			r.Held = false
			if err := c.set(r); err != nil {
				return err
			}
		}
		if w.Working || len(w.Waits) > 0 {
			return c.working(r)
		}
		return nil
	})
	return e, err
}

// recipients refuses an entry to a person who is not one of the office
// now, or to an identity that matches no person and is not the request's
// asker: nobody is given it in their place.
func (c *change) recipients(r *Request, to []Who) error {
	for _, w := range to {
		if (w.Kind == IsPerson || w.Kind == IsIdentity) && c.personOf(w) == "" && w != r.Asker {
			return refuse("%s is not a person of the office, so nothing was said to them, and nobody was told in their place", c.calledOf(w))
		}
	}
	return nil
}

// Rewait puts an entry that still waits to others in place of those it
// waited on, as when a decision walks on past someone; put to nobody, it
// is closed. Those it was put to before still see it, with what answers it.
func (s *Store) Rewait(entry int64, whos []Who) error {
	return s.write(func(c *change) error {
		e, err := c.entry(entry)
		if err != nil {
			return err
		}
		ws, err := c.waits(e.Request)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(ws, func(w wait) bool { return w.entry == entry }) {
			return refuse("entry %d waits on nobody any more: it was answered or closed, so it is put to nobody else", entry)
		}
		if err := c.persons(whos); err != nil {
			return err
		}
		if err := c.close(entry); err != nil {
			return err
		}
		c.touch(e.Request)
		return c.wait(e.Request, entry, whos)
	})
}

// persons refuses any who is not a person of the office now.
func (c *change) persons(whos []Who) error {
	for _, w := range whos {
		if c.personOf(w) == "" {
			return refuse("%s is not a person of the office, so nothing can wait on them, and nobody waits in their place", c.calledOf(w))
		}
	}
	return nil
}

// working makes a request working again, unless it was stopped.
func (c *change) working(r *Request) error {
	if r.Stopped || !r.Answered {
		c.touch(r.Number)
		return nil
	}
	r.Answered = false
	return c.set(r)
}

// keep keeps a new request, with its number.
func (c *change) keep(r *Request) error {
	r.Made = c.at.In(c.o.Zone)
	var ticket any
	if r.Ticket != "" {
		ticket = r.Ticket
	}
	res, err := c.tx.Exec(`INSERT INTO requests_requests(team, team_known, asker, origin, bridge, place, parent, ticket, worker, made)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, r.Team, r.TeamKnown, jsonOf(r.Asker), jsonOf(r.Origin), r.Origin.Bridge, r.Origin.Place,
		r.Parent, ticket, r.Worker, stamp(r.Made))
	if err != nil {
		return failed(err)
	}
	if r.Number, err = res.LastInsertId(); err != nil {
		return failed(err)
	}
	c.reqs[r.Number] = r
	c.touch(r.Number)
	return nil
}

// set keeps where a request stands, a source's time as an instant.
func (c *change) set(r *Request) error {
	source := ""
	if r.Source != nil {
		st := *r.Source
		st.At = st.At.UTC()
		source = jsonOf(st)
	}
	if _, err := c.tx.Exec(`UPDATE requests_requests SET answered = ?, stopped = ?, held = ?, source = ? WHERE number = ?`,
		r.Answered, r.Stopped, r.Held, source, r.Number); err != nil {
		return failed(err)
	}
	c.touch(r.Number)
	return nil
}

// add keeps an entry with its files, and whom it waits on.
func (c *change) add(r *Request, e Entry, files []File, waits []Who) (*Entry, error) {
	if err := checkSize(e.Text, files); err != nil {
		return nil, err
	}
	if e.To == nil {
		e.To = []Who{}
	}
	res, err := c.tx.Exec(`INSERT INTO requests_entries(request, kind, by_, to_, at, text, came_in, read_team, read_known, answers, sub, mentioned)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, r.Number, e.Kind, jsonOf(e.By), jsonOf(e.To), stamp(c.at), e.Text, jsonOf(e.In),
		e.Read, e.ReadKnown, e.Answers, e.Sub, e.Mentioned)
	if err != nil {
		return nil, failed(err)
	}
	if e.ID, err = res.LastInsertId(); err != nil {
		return nil, failed(err)
	}
	e.Request, e.At = r.Number, c.at.In(c.o.Zone)
	for _, f := range files {
		if _, err := c.tx.Exec(`INSERT INTO requests_files(entry, name, data) VALUES (?, ?, ?)`, e.ID, f.Name, f.Data); err != nil {
			return nil, failed(err)
		}
		e.Files = append(e.Files, FileHead{Name: f.Name, Size: len(f.Data)})
	}
	c.touch(r.Number)
	return &e, c.wait(r.Number, e.ID, waits)
}

// wait has an entry wait on each of whos, again for one it was put to
// before.
func (c *change) wait(n, entry int64, whos []Who) error {
	for _, w := range whos {
		if _, err := c.tx.Exec(`INSERT INTO requests_waits(entry, request, who) VALUES (?, ?, ?)
			ON CONFLICT(entry, who) DO UPDATE SET open = 1`, entry, n, jsonOf(w)); err != nil {
			return failed(err)
		}
	}
	return nil
}

// close closes what an entry waits on. Whom it was put to is kept, for
// sight.
func (c *change) close(entry int64) error {
	if _, err := c.tx.Exec(`UPDATE requests_waits SET open = 0 WHERE entry = ?`, entry); err != nil {
		return failed(err)
	}
	return nil
}

// closeAll closes everything that waits in a request.
func (c *change) closeAll(n int64) error {
	if _, err := c.tx.Exec(`UPDATE requests_waits SET open = 0 WHERE request = ?`, n); err != nil {
		return failed(err)
	}
	return nil
}

// checkSize refuses a text or a file larger than a request keeps, whole:
// nothing is ever kept cut short.
func checkSize(text string, files []File) error {
	if len(text) > MaxText {
		return refuse("the text is %d bytes, more than the %d a request keeps, so nothing was kept", len(text), MaxText)
	}
	var names []string
	for _, f := range files {
		switch {
		case f.Name == "":
			return refuse("a file has no name, so nothing was kept")
		case slices.Contains(names, f.Name):
			return refuse("two files are named %s, so nothing was kept", f.Name)
		case len(f.Data) > MaxFile:
			return refuse("%s is %d bytes, more than the %d a request keeps of a file, so nothing was kept", f.Name, len(f.Data), MaxFile)
		}
		names = append(names, f.Name)
	}
	return nil
}

// Read takes a new reading of the office, from which everything is shown
// and judged until the next. A person or a team it finds gone is gone for
// good: one written later under the same name is someone new, and
// inherits nothing. A request open to a team that has gone is answered
// saying so, and a question to someone who can no longer see the request
// it is in is answered saying so.
func (s *Store) Read(o *office.Office) error {
	var known map[name]kept
	changed, err := s.commit(func(tx *sql.Tx) (*change, error) {
		s.mu.Lock()
		before := s.known
		s.mu.Unlock()
		known = maps.Clone(before)
		c := &change{look: &look{q: tx, o: o, known: known, reqs: map[int64]*Request{}}, tx: tx, at: s.now().UTC()}
		gone, err := c.reread()
		if err != nil {
			return nil, err
		}
		return c, c.closeGone(before, gone)
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.office, s.known = o, known
	s.mu.Unlock()
	s.tell(changed)
	return nil
}

// reread keeps each person and team of the reading as it has them, and
// gives those it finds gone, as the reading before had them.
func (c *change) reread() (map[name]kept, error) {
	now := map[name]kept{}
	for _, n := range c.o.Everyone {
		p := c.o.People[n]
		now[name{IsPerson, n}] = kept{called: p.Called, reached: p.Reached}
	}
	for _, n := range c.o.Order {
		now[name{IsTeam, n}] = kept{called: c.o.Teams[n].Called}
	}
	gone := map[name]kept{}
	for n, k := range c.known {
		if _, ok := now[n]; ok {
			continue
		}
		if _, err := c.tx.Exec(`UPDATE requests_known SET gone = 1 WHERE id = ?`, k.id); err != nil {
			return nil, failed(err)
		}
		gone[n] = k
		delete(c.known, n)
	}
	for _, n := range slices.SortedFunc(maps.Keys(now), cmpNames) {
		k := now[n]
		switch had, ok := c.known[n]; {
		case ok && had.called == k.called && slices.Equal(had.reached, k.reached):
			continue
		case ok:
			k.id = had.id
			if _, err := c.tx.Exec(`UPDATE requests_known SET called = ?, reached = ? WHERE id = ?`, k.called, jsonOf(k.reached), k.id); err != nil {
				return nil, failed(err)
			}
		default:
			res, err := c.tx.Exec(`INSERT INTO requests_known(kind, name, called, reached) VALUES (?, ?, ?, ?)`, n.kind, n.name, k.called, jsonOf(k.reached))
			if err != nil {
				return nil, failed(err)
			}
			if k.id, err = res.LastInsertId(); err != nil {
				return nil, failed(err)
			}
		}
		c.known[n] = k
	}
	return gone, nil
}

func cmpNames(a, b name) int {
	return cmp.Or(strings.Compare(a.kind, b.kind), strings.Compare(a.name, b.name))
}

// closeGone answers what was open to a team that has gone, and the
// questions put to people who can no longer see the request they are in,
// from the people and teams as the reading before had them.
func (c *change) closeGone(before, gone map[name]kept) error {
	calledGone := func(n name) string { return cmp.Or(gone[n].called, n.name) }
	rs, err := c.all()
	if err != nil {
		return err
	}
	open, err := c.waits(0)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if c.answered(r, open) || gone[name{IsTeam, r.Team}].id != r.TeamKnown {
			continue
		}
		if err := c.closeAll(r.Number); err != nil {
			return err
		}
		if _, err := c.add(r, Entry{Kind: KindBulldog, By: Bulldog, To: []Who{r.Asker},
			Text: calledGone(name{IsTeam, r.Team}) + " is no longer in the office."}, nil, nil); err != nil {
			return err
		}
		r.Answered, r.Held = true, false
		if err := c.set(r); err != nil {
			return err
		}
	}
	ws, err := c.waits(0)
	if err != nil {
		return err
	}
	for _, w := range ws {
		put, err := c.entry(w.entry)
		if err != nil {
			return err
		}
		if put.Kind != KindQuestion || w.who.Kind != IsPerson && w.who.Kind != IsIdentity {
			continue
		}
		r, err := c.req(w.request)
		if err != nil {
			return err
		}
		var text string
		switch p, had := c.personOf(w.who), had(before, w.who); {
		case p == "" && had == "":
			continue
		case p == "" && (w.who.Kind == IsPerson || c.o.People[had] == nil):
			text = calledGone(name{IsPerson, had}) + " is no longer in the office."
		case p == "":
			text = fmt.Sprintf("%s is no longer reached as %s on %s.", c.o.People[had].Called, w.who.ID, w.who.Bridge)
		default:
			done, err := c.seesDone(r, p)
			if err != nil {
				return err
			}
			if done || c.seesRead(r.Team, r.TeamKnown, p) {
				continue
			}
			text = fmt.Sprintf("%s is no longer in %s.", c.o.People[p].Called, inSentence(called(c.o, r.Team)))
		}
		if _, err := c.add(r, Entry{Kind: KindReplied, By: Bulldog, To: []Who{put.By}, Text: text, Read: put.Read, ReadKnown: put.ReadKnown,
			Answers: put.ID}, nil, nil); err != nil {
			return err
		}
		if err := c.close(put.ID); err != nil {
			return err
		}
		if err := c.working(r); err != nil {
			return err
		}
	}
	return nil
}

// had gives the person someone was at the reading before, if anyone.
func had(before map[name]kept, w Who) string {
	switch w.Kind {
	case IsPerson:
		if w.Known != 0 && before[name{IsPerson, w.Name}].id == w.Known {
			return w.Name
		}
	case IsIdentity:
		for _, n := range slices.SortedFunc(maps.Keys(before), cmpNames) {
			if n.kind == IsPerson && slices.Contains(before[n].reached, office.Identity{Bridge: w.Bridge, ID: w.ID}) {
				return n.name
			}
		}
	}
	return ""
}

// inSentence gives what a team is called as it is written inside a
// sentence: "The house" as "the house".
func inSentence(called string) string {
	if rest, ok := strings.CutPrefix(called, "The "); ok {
		return "the " + rest
	}
	return called
}
