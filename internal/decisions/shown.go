package decisions

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// input gives a decision as a rule or a worker is given it, at the step it
// is asked: what it decides, the request it serves, every referral before
// it, and where its own referral would go. A disclosure is given the team's notes as
// they are now, and what would leave as its text.
func (d *Decisions) input(o *office.Office, dec *Decision, stages []Stage, at int) (map[string]any, string, error) {
	in := map[string]any{"kind": dec.Kind, "decision": dec.ID, "team": dec.Team, "team_called": called(o, dec.Team), "does": dec.Does(o)}
	referrals := []map[string]any{}
	for _, w := range dec.Referrals {
		referrals = append(referrals, map[string]any{"by": w.Called, "kind": w.Kind, "note": w.Note})
	}
	in["referrals"] = referrals
	after := beyond(stages, dec, at)
	goes := []map[string]any{}
	for _, s := range after {
		goes = append(goes, map[string]any{"kind": s.Kind, "to": d.stageCalled(o, s), "list": s.List, "team": s.Team})
	}
	in["referral_goes_to"] = goes
	in["ends"] = ends(dec, d.referWords(o, dec, after))
	asked, err := d.served(o, dec)
	if err != nil {
		return nil, "", err
	}
	if asked != nil {
		in["request"] = asked
	}
	switch dec.Kind {
	case OfAction:
		in["action"] = map[string]any{"name": dec.Job.key(), "job": dec.Job.Name, "does": dec.Job.does(), "args": raw(dec.Job.args())}
		if dec.Job.Asked != nil {
			in["said"] = dec.Job.Asked.Said
		}
		in["worker"] = dec.Whose.Name
	case OfDisclosure:
		disclosure, err := d.disclosure(o, dec)
		if err != nil {
			return nil, "", err
		}
		in["disclosure"] = disclosure
		in["text"] = dec.Said.Text
		in["notes"] = disclosure["notes"]
	case OfChange:
		p := dec.Put
		var files []map[string]any
		for _, f := range p.Files {
			files = append(files, map[string]any{"path": f.Path, "text": f.Text, "gone": f.Gone, "was": f.Was, "had": f.Had})
		}
		in["change"] = map[string]any{"by": p.By, "person": p.Person, "why": p.Why, "files": files, "says": p.Says, "lets_more_by": p.LetsMoreBy}
	}
	said, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, "", err
	}
	words := fmt.Sprintf("You are asked to decide this for %s. %s\nEnd with only { \"says\": \"yes\", \"refuse\" or \"refer\", \"note\": your reason }.\n\n%s",
		inSentence(called(o, dec.Team)), in["ends"], said)
	return in, words, nil
}

// beyond gives the stages a referral at a stage would go on to, in order,
// as far as the people the walk ends at, walked as the decision would be:
// each decider that has referred it already is skipped, as is a worker's
// own action. Where a person's own change has none left, it is
// the person, whose yes a referral is taken as.
func beyond(stages []Stage, dec *Decision, at int) []Stage {
	then := *dec
	then.At = nil
	then.Referrals = append(slices.Clone(dec.Referrals), Word{Kind: stages[at].Kind, Name: stages[at].Name})
	var after []Stage
	for {
		s, _, ok := next(stages, &then)
		switch {
		case !ok && dec.Kind == OfChange && dec.Put.Person:
			return append(after, Stage{Kind: IsPeople, People: []string{dec.Put.By}, List: drafter})
		case !ok:
			return after
		case s.Kind == IsPeople:
			return append(after, s)
		case !ownWorker(dec, s):
			after = append(after, s)
		}
		then.Referrals = append(then.Referrals, Word{Kind: s.Kind, Name: s.Name})
	}
}

// drafter is the list of the stage that is a person's own yes on the
// change they put.
const drafter = "drafter"

// referWords says where a referral goes, in words.
func (d *Decisions) referWords(o *office.Office, dec *Decision, after []Stage) string {
	if len(after) == 0 {
		return "nobody, so a referral ends it"
	}
	s, last := after[0], after[len(after)-1]
	where := fmt.Sprintf("%s, on %s's %s list", d.stageCalled(o, s), inSentence(called(o, s.Team)), s.List)
	switch {
	case s.List == drafter:
		return fmt.Sprintf("nobody else: it is %s's own change, so a referral is taken as their yes, and the change is written", d.stageCalled(o, s))
	case last.List == drafter:
		where += fmt.Sprintf(", and if every one refers it, it is taken as %s's yes, since it is their own change, and the change is written", d.stageCalled(o, last))
	case len(after) > 1 && last.Kind == IsPeople:
		where += fmt.Sprintf(", and on to %s if every one before refers it", d.stageCalled(o, last))
	}
	return where
}

func ends(dec *Decision, where string) string {
	if dec.Kind == OfChange {
		return "A refusal from you ends it. Only a person approves a change, so your yes is taken as a referral. A referral goes on, with your note, to " + where + "."
	}
	return "Your yes or your refusal ends it. A referral goes on, with your note, to " + where + "."
}

// stageCalled gives a stage as a sentence names it.
func (d *Decisions) stageCalled(o *office.Office, s Stage) string {
	if s.Kind == IsPeople {
		return strings.ReplaceAll(names(o, s.People), ", ", " or ")
	}
	return calledOf(o, s.Kind, s.Name)
}

// Does gives what a decision is, in a line, built from what it holds and
// the office as read now: the action as its tool describes itself; what
// is to be disclosed, a question to another team naming the team asked;
// or the change, by the first line of why it was drafted.
func (dec *Decision) Does(o *office.Office) string {
	switch dec.Kind {
	case OfDisclosure:
		if s := dec.Said; s.What == SaysSub {
			return fmt.Sprintf("Question to %s, to disclose", called(o, s.Team))
		}
		return fmt.Sprintf("%s of %s, to disclose", capital(dec.Said.What), inSentence(called(o, dec.Team)))
	case OfChange:
		first, _, _ := strings.Cut(dec.Put.Why, "\n")
		return "Change to the office: " + strings.TrimSpace(first)
	}
	return dec.Job.does()
}

// served gives the request a decision serves: its number, what was asked
// and who asked it.
func (d *Decisions) served(o *office.Office, dec *Decision) (map[string]any, error) {
	if dec.Request == 0 {
		return nil, nil
	}
	thread, err := d.requests.Thread(dec.Request)
	if err != nil {
		return nil, err
	}
	asks, err := d.requests.Asks(dec.Request)
	if err != nil {
		return nil, err
	}
	asked := ""
	if len(thread) > 0 {
		asked = thread[0].Text
	}
	return map[string]any{"number": dec.Request, "asked": asked, "asker": partyCalled(asks)}, nil
}

// disclosure gives what a disclosure decider is shown: what would leave, and
// whether it is an answer, a question, a tell or a sub-request; whom it is
// to, and where it will be read; and the team's notes, read now.
func (d *Decisions) disclosure(o *office.Office, dec *Decision) (map[string]any, error) {
	s := dec.Said
	out := map[string]any{"what": s.What, "text": s.Text, "said_in": s.Request}
	switch s.What {
	case SaysAnswer:
		asks, err := d.requests.Asks(s.Request)
		if err != nil {
			return nil, err
		}
		out["to"] = partyCalled(asks)
	case SaysSub:
		out["to"] = called(o, s.Team)
	default:
		out["to"] = d.whoCalled(o, s.To)
	}
	readAt, err := d.readAt(s.Request)
	if err != nil {
		return nil, err
	}
	out["read_at"] = readAt
	notes, err := o.Notes(dec.Team)
	if err != nil {
		return nil, fmt.Errorf("the notes of %s cannot be read: %w", dec.Team, err)
	}
	var given []map[string]any
	for _, n := range notes {
		given = append(given, map[string]any{"name": n.Name, "text": n.Text})
	}
	out["notes"] = given
	return out, nil
}

// readAt says where what is said in a request will be read in the end:
// the origin at the top of its tree.
func (d *Decisions) readAt(n int64) (string, error) {
	top, _, err := d.requests.Top(n)
	if err != nil {
		return "", err
	}
	switch o := top.Origin; o.Kind {
	case requests.AtPlace:
		return fmt.Sprintf("%s on %s", cmp.Or(o.Heard, o.Place), o.Bridge), nil
	case requests.InChat:
		return "the chat on the screens", nil
	case requests.ByRoutine:
		return "by the people of " + o.Team, nil
	}
	return "", nil
}

// shown gives what the people asked to decide are shown, and how to
// answer, as one entry to them all.
func (d *Decisions) shown(o *office.Office, dec *Decision, people []string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "A decision for %s: %s\n", inSentence(called(o, dec.Team)), dec.Does(o))
	switch dec.Kind {
	case OfAction:
		who := cmp.Or(dec.Whose.Name, "a run of "+dec.Team)
		fmt.Fprintf(&b, "%s would do it (%s), with: %s\n", who, dec.Job.key(), compact(dec.Job.args()))
		if dec.Job.Asked != nil && dec.Job.Asked.Said != "" {
			fmt.Fprintf(&b, "%s said: %s\n", who, dec.Job.Asked.Said)
		}
	case OfDisclosure:
		r, err := d.disclosure(o, dec)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%s would say this %s, to %s, read at %s:\n%s\n", d.whoCalled(o, dec.Said.By), whatOf(dec.Said.What), r["to"], r["read_at"], dec.Said.Text)
	case OfChange:
		p := dec.Put
		fmt.Fprintf(&b, "Drafted by %s, who says why: %s\n", p.By, p.Why)
		if len(p.Says) > 0 {
			fmt.Fprintf(&b, "What it does:\n- %s\n", strings.Join(p.Says, "\n- "))
		}
		if len(p.LetsMoreBy) > 0 {
			fmt.Fprintf(&b, "What it lets more by:\n- %s\n", strings.Join(p.LetsMoreBy, "\n- "))
		}
		var paths []string
		for _, f := range p.Files {
			paths = append(paths, f.Path)
		}
		fmt.Fprintf(&b, "Files: %s\n", strings.Join(paths, ", "))
	}
	if asked, err := d.served(o, dec); err != nil {
		return "", err
	} else if asked != nil {
		fmt.Fprintf(&b, "For request %d, asked by %s: %s\n", dec.Request, asked["asker"], firstLine(asked["asked"].(string)))
	}
	for _, w := range dec.Referrals {
		fmt.Fprintf(&b, "%s referred it%s\n", w.Called, noted(w.Note))
	}
	fmt.Fprintf(&b, "Asked: %s.\n", names(o, people))
	b.WriteString("Answer yes or refuse, with a note if you like.")
	return b.String(), nil
}

// whoCalled gives someone an entry is by or to as a sentence names them.
func (d *Decisions) whoCalled(o *office.Office, w requests.Who) string {
	switch w.Kind {
	case requests.IsPerson:
		return calledOf(o, office.IsPerson, w.Name)
	case requests.IsIdentity:
		if p := o.Person(w.Bridge, w.ID); p != nil {
			return p.Called
		}
		return "a guest on " + w.Bridge
	case requests.IsTeam:
		return called(o, w.Name)
	}
	return w.Name
}

// partyCalled gives who asks a request as a sentence names them.
func partyCalled(p requests.Party) string {
	switch p.Kind {
	case requests.IsGuest:
		return fmt.Sprintf("a guest on %s (%s)", p.Bridge, p.ID)
	case requests.IsWorker:
		return fmt.Sprintf("%s, of %s", p.Name, p.Team)
	}
	return cmp.Or(p.Called, p.Name)
}

func raw(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

func compact(b json.RawMessage) string {
	if len(b) == 0 {
		return "nothing"
	}
	return string(b)
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}
