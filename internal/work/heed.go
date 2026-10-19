package work

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// What a team's heed says of what it weighed: the lead starts on it, or
// it is kept in the request and nothing runs.
const (
	heedAct  = "act"
	heedNote = "note"
)

// heedOf gives a request's team's heed, and its rule, when the heed
// weighs what is new before the lead starts on it: the request was asked
// at a channel place, and everything new is a message a person or a guest
// said there, through its bridge, but the asking of a source thing such as
// an issue, which is addressed to whoever watches it. Anything else, or a
// heed with a problem, starts the lead on all of it.
func heedOf(o *office.Office, rd ready) (*office.Heed, *office.Rule) {
	team, origin := o.Teams[rd.r.Team], rd.r.Origin
	if team == nil || team.Heed == nil || !team.Heed.Runs() || rd.again || origin.Kind != requests.AtPlace || origin.Heard == office.Direct {
		return nil, nil
	}
	rule := o.Rules[team.Heed.Rule]
	if rule == nil || !rule.Runs() {
		return nil, nil
	}
	for _, e := range rd.fresh {
		switch {
		case e.Kind != requests.KindAsked && e.Kind != requests.KindSaid, !byPerson(e),
			e.In.Bridge != origin.Bridge || e.In.Place != origin.Place,
			e.Kind == requests.KindAsked && origin.About != "":
			return nil, nil
		}
	}
	return team.Heed, rule
}

// weigh starts a team's heed on what is new in a request, a run of the
// team's noted before its start is sent, as a lead's is. It is given what
// is new as text, with when and by whom each was said; the request as its
// lead would be given it, with what is weighed as new; and the heed's
// settings, whole. Bulldog keeps no time limit of its own on it. A start
// that cannot be sent is said to the team's people, and tried again later.
func (w *Work) weigh(ctx context.Context, rd ready, heed *office.Heed, rule *office.Rule) {
	r := rd.r
	fail := func(what string, err error) {
		w.trouble(r.Number, r.Team, fmt.Sprintf("Bulldog could not start %s, the heed of %s, on request %d (%s), and tries again in %v: %v",
			path.Base(rule.Path), w.office.Teams[r.Team].Called, r.Number, what, Again, err))
		w.again(fmt.Sprintf("start:%d", r.Number))
		if err := w.pend(r.Number); err != nil {
			w.Log("request %d: %v", r.Number, err)
		}
	}
	g, err := w.giving(w.office, rd)
	if err != nil {
		fail("what it is given could not be put together", err)
		return
	}
	var said []string
	for _, e := range rd.fresh {
		said = append(said, g.entryText(e))
	}
	input := map[string]any{"said": strings.Join(said, "\n"), "request": g.data(), "settings": heed.Settings}
	draft, err := ruleDraft(rule)
	if err != nil {
		fail("its rule could not be read", err)
		return
	}
	name, err := w.corbie.NewRun()
	if err != nil {
		fail("no name could be chosen for its run", err)
		return
	}
	labels := w.labels(requests.Labels(r))
	labels[LabelKind] = KindHeed
	n := &note{Name: name, Kind: KindHeed, Team: r.Team, Request: r.Number, Worker: rule.Path, Labels: labels,
		Seen: rd.fresh[len(rd.fresh)-1].ID, Noted: w.clock.Now()}
	if err := w.keep(n); err != nil {
		fail("its run could not be noted", err)
		return
	}
	if _, err := w.corbie.Start(ctx, draft, client.Start{Run: name, Input: input, Labels: labels}); err != nil {
		w.endUnsent(name, err)
		fail("Corbie was not reached", err)
		return
	}
	delete(w.troubled, r.Number)
}

// heeded takes how a team's heed ended, once. A note writes the heed's
// word on each message it weighed, and the request is dealt with up to
// them, so its lead does not start on them; a write that fails is said to
// the team's people, and the ending is taken again later. Anything else
// lets through to the lead everything new: an act, and a heed that
// failed, was cancelled or lost, or answered neither act nor note, which
// is said to the team's people and never to the asker. A heed whose
// request was stopped lets nothing through.
func (w *Work) heeded(n *note, run *client.Run, e ending) {
	says, word := "", ""
	if e.status == done {
		says, word = heedSaid(run)
		switch {
		case says == "":
			e.status, e.how = failed, word
		case word == "":
			e.how = says
		default:
			e.how = says + ": " + word
		}
	}
	rule := path.Base(n.Worker)
	if says == heedNote && n.Stop == "" {
		if err := w.noteWeighed(n, word); err != nil {
			w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not write that %s noted what was said, and tries again in %v: %v", rule, Again, err))
			w.showAgain(n.Name)
			return
		}
	}
	first, err := w.endWork(n, e, says != heedNote && n.Stop == "")
	switch {
	case err != nil:
		w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not keep how %s ended, and reads it again in %v: %v", rule, Again, err))
		w.showAgain(n.Name)
		return
	case !first:
		return
	}
	w.touch(n.Request)
	switch {
	case n.Stop != "":
	case says == heedNote:
		w.dealtWith(n.Request, n.Seen)
	case says == "":
		lead := ""
		if team := w.office.Teams[n.Team]; team != nil {
			lead = team.Lead
		}
		w.trouble(n.Request, n.Team, fmt.Sprintf("%s failed: %s. %s was started.", rule, e.how, lead))
	}
}

// noteWeighed writes a heed's word on each message it weighed: those new
// to its request's lead, up to the heed's Seen.
func (w *Work) noteWeighed(n *note, word string) error {
	r, err := w.requests.Get(n.Request)
	if err != nil {
		return err
	}
	thread, err := w.requests.Thread(n.Request)
	if err != nil {
		return err
	}
	mark, err := w.mark(n.Request)
	if err != nil {
		return err
	}
	given, err := w.givenTo(n.Request)
	if err != nil {
		return err
	}
	var weighed []int64
	for _, e := range fresh(r, thread, mark, false, given) {
		if e.ID <= n.Seen && slices.Contains([]string{requests.KindAsked, requests.KindSaid}, e.Kind) {
			weighed = append(weighed, e.ID)
		}
	}
	_, err = w.requests.Noted(n.Request, w.requests.Rule(n.Worker, n.Team), word, weighed)
	return err
}

// heedSaid reads what a heed answered, { says: act or note, note }, from
// its output, or from text that holds it, as an agent replies. says is ""
// for any other answer, and its word then says what it was.
func heedSaid(run *client.Run) (says, word string) {
	out := output(run)
	if text, ok := out.(string); ok {
		var v any
		if json.Unmarshal([]byte(strings.TrimSpace(text)), &v) == nil {
			out = v
		}
	}
	m, _ := out.(map[string]any)
	says, _ = m["says"].(string)
	switch v := m["note"].(type) {
	case nil:
	case string:
		word = v
	default:
		b, _ := json.Marshal(v)
		word = string(b)
	}
	if says != heedAct && says != heedNote {
		b, _ := json.Marshal(out)
		return "", "it answered neither act nor note, as { says: act or note, note }: " + cut(string(b), 200)
	}
	return says, word
}
