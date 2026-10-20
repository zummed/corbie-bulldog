package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/shipped"
)

// A tool Bulldog ships is a run that waits on its ticket below a run
// Bulldog started. Bulldog reads the call from the run's record, does
// it, keeps the answer and answers the ticket; the answer is the tool's
// result.

// Climb is how many runs up from a call Bulldog looks for the worker that
// made it and the run Bulldog started at the top.
const Climb = 16

// The tools Bulldog ships whose calls it does itself; the builder's are
// handed to Changes.
const (
	toolAskTeam    = "ask_team"
	toolAskPerson  = "ask_person"
	toolTell       = "tell"
	toolReadThread = "read_thread"
	toolWriteNotes = "write_notes"
	toolDocs       = "docs"
	toolReadDoc    = "read_doc"
	toolSearchDocs = "search_docs"
	toolOfficePut  = "office_put"
	toolWriteDoc   = "write_doc"
	// askTool is what an ask's run is kept as among the calls.
	askTool = "ask"
)

// WritesNotes says whether a worker names the notes kit Bulldog ships, and
// so is given its team's notes and may write them.
func WritesNotes(w *office.Worker) bool {
	return slices.ContainsFunc(w.Held, func(t office.Tool) bool { return t.Kind == office.ToolBulldog && t.Name == toolWriteNotes })
}

// WritesDocs says whether a worker holds write_doc, of the docs kit
// Bulldog ships, and so may write its team's docs.
func WritesDocs(w *office.Worker) bool {
	return slices.ContainsFunc(w.Held, func(t office.Tool) bool { return t.Kind == office.ToolBulldog && t.Name == toolWriteDoc })
}

// errGone says a call no longer waits on its ticket: its run has ended.
var errGone = errors.New("it no longer waits on its ticket")

// does says whether Bulldog does a tool: every tool in a tools file it
// ships, and no other.
func does(tool string) bool {
	switch tool {
	case toolAskTeam, toolAskPerson, toolTell, toolReadThread, toolWriteNotes, toolDocs, toolReadDoc, toolSearchDocs, toolWriteDoc:
		return true
	}
	return slices.Contains(shipped.Tools(shipped.Build), tool)
}

// A calling is one call being done: what is kept of it, the request of
// the run it came from, the team, and the worker that made it.
type calling struct {
	row    *callRow
	r      *requests.Request
	team   *office.Team
	worker string
	args   map[string]any
	raw    json.RawMessage
	// again says it was being done when Bulldog stopped.
	again bool
}

// call does a call of a shipped tool once, and gives the request of the
// run Bulldog noted at its top, 0 for a call refused for having none, and
// whether it waits on a person, another team or a decision. A call done
// already is given its kept answer again; a call refused is answered at
// once. An error is what could not be read or reached: the call is looked
// at again.
func (w *Work) call(ctx context.Context, r *client.Run) (int64, bool, error) {
	row, err := w.callOf(r.ID)
	if err != nil {
		return 0, false, err
	}
	if row != nil && row.State != doing {
		waits, err := w.seenAgain(ctx, row)
		return row.Request, waits, err
	}
	in, err := w.inputOf(ctx, r.ID)
	if err != nil {
		return 0, false, err
	}
	again := row != nil
	if row == nil {
		tool, _ := in["tool"].(string)
		row = &callRow{Run: r.ID, Tool: tool, State: doing}
	}
	c, answer, err := w.calling(ctx, r, row)
	if err != nil {
		return 0, false, err
	}
	if c != nil {
		c.again = again
		c.args, _ = in["args"].(map[string]any)
		if c.raw, err = json.Marshal(in["args"]); err != nil {
			return 0, false, err
		}
		if err := w.begin(c); err != nil {
			return 0, false, err
		}
		var waits bool
		if answer, waits, err = w.do(ctx, c); err != nil || waits {
			return row.Request, waits, err
		}
	}
	// A call refused before it was kept is kept refused, and refused again
	// if it is seen again.
	if err := w.answerCall(row, answer, nil); err != nil {
		return 0, false, err
	}
	return row.Request, false, w.giveKept(ctx, row)
}

// begin keeps a call before it is done, with the last entry of its
// request's thread, after which is whatever it puts.
func (w *Work) begin(c *calling) error {
	if c.row.Mark == 0 {
		thread, err := w.requests.Thread(c.r.Number)
		if err != nil {
			return err
		}
		c.row.Mark = thread[len(thread)-1].ID
	}
	return w.keepCall(c.row)
}

// seenAgain takes a call kept already, seen waiting again: after a
// restart, or once its input could be read. One answered is given its
// kept answer; one waiting is looked at for what it waits on.
func (w *Work) seenAgain(ctx context.Context, row *callRow) (bool, error) {
	switch row.State {
	case answered:
		return false, w.giveKept(ctx, row)
	case waiting:
		w.touch(row.Request)
		if row.Sub != 0 {
			w.touch(row.Sub)
		}
	}
	return true, nil
}

// giveKept answers a call's ticket with its kept answer. A call that no
// longer waits is let go when its ending is seen.
func (w *Work) giveKept(ctx context.Context, row *callRow) error {
	if err := w.give(ctx, row.Run, shipped.CallTicket, row.Answer); err != nil && !errors.Is(err, errGone) {
		return fmt.Errorf("the answer kept for %s could not be given: %w", row.Run, err)
	}
	return nil
}

// giveSoon gives a call its kept answer, or, when Corbie cannot be
// reached, again once Again has passed.
func (w *Work) giveSoon(ctx context.Context, row *callRow) {
	if err := w.giveKept(ctx, row); err != nil {
		w.giveLater(row.Run, err)
	}
}

// giveLater has a call given its kept answer once Again has passed.
func (w *Work) giveLater(run string, err error) {
	w.Log("the answer kept for %s could not be given, and is given again in %v: %v", run, Again, err)
	w.again("give:" + run)
}

// give answers the ticket a run waits on.
func (w *Work) give(ctx context.Context, run, ticket string, value any) error {
	tickets, err := w.corbie.Tickets(ctx, run)
	switch {
	case errors.Is(err, client.ErrNoRun):
		return errGone
	case err != nil:
		return err
	case tickets[ticket] == "":
		return errGone
	}
	_, err = w.corbie.Reply(ctx, run, tickets[ticket], value, 0)
	return err
}

// inputOf gives what a run was started with.
func (w *Work) inputOf(ctx context.Context, run string) (map[string]any, error) {
	r, err := w.corbie.Show(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("its input cannot be read yet: %w", err)
	}
	in, ok := r.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("its input is %T, not what a shipped machine is given", r.Input)
	}
	return in, nil
}

// above gives the machines above a run, nearest first, and the run at the
// top, which Bulldog started. A call more than Climb runs below it is
// refused, with a sentence.
func (w *Work) above(ctx context.Context, r *client.Run) ([]string, string, string, error) {
	var machines []string
	at := r.Parent
	for range Climb {
		p, err := w.corbie.Show(ctx, at)
		switch {
		case errors.Is(err, client.ErrNoRun):
			return nil, "", "The run this call comes from is gone, so it was not done.", nil
		case err != nil:
			return nil, "", "", fmt.Errorf("the run %s above it cannot be read: %w", at, err)
		}
		machines = append(machines, p.Machine)
		if p.Parent == "" {
			return machines, p.ID, "", nil
		}
		at = p.Parent
	}
	return nil, "", fmt.Sprintf("Bulldog looks at most %d runs up from a call for the worker that made it, and this one is further down, so it was not done.", Climb), nil
}

// calling finds whose a call is: the run at its top must be one Bulldog
// noted for this office, working on a request, and not ended, and the
// worker that made it the nearest worker of that run's team above it.
// Any other call is refused, with a sentence.
func (w *Work) calling(ctx context.Context, r *client.Run, row *callRow) (*calling, string, error) {
	machines, top, refusal, err := w.above(ctx, r)
	if err != nil || refusal != "" {
		return nil, refusal, err
	}
	n, err := w.noted(top)
	switch {
	case err != nil:
		return nil, "", err
	case n == nil:
		return nil, "This call comes from a run Bulldog did not start for this office, so it was not done.", nil
	case n.Kind != KindWork:
		return nil, "Bulldog does its tools only for a worker's run on a request, so this call was not done.", nil
	case n.ended() || n.Stop != "":
		return nil, "The run this call comes from has ended or is being stopped, so it was not done.", nil
	}
	o := w.office
	team := o.Teams[n.Team]
	if team == nil {
		return nil, fmt.Sprintf("%s is no longer in the office, so this call was not done.", n.Team), nil
	}
	worker := ""
	for _, m := range machines {
		if ow := o.Workers[m]; ow != nil && ow.Team == n.Team {
			worker = m
			break
		}
	}
	if worker == "" {
		return nil, fmt.Sprintf("No worker of %s made this call, so it was not done.", inSentence(team.Called)), nil
	}
	req, err := w.requests.Get(n.Request)
	if err != nil {
		return nil, "", err
	}
	row.Request, row.Worker = n.Request, worker
	return &calling{row: row, r: req, team: team, worker: worker}, "", nil
}

// do does a call, and gives its answer, or says it waits; a refusal is
// the answer, in a sentence.
func (w *Work) do(ctx context.Context, c *calling) (string, bool, error) {
	answer, waits, err := w.doing(ctx, c)
	var refusal requests.Refusal
	if errors.As(err, &refusal) {
		return sentence(string(refusal)), false, nil
	}
	return answer, waits, err
}

func (w *Work) doing(ctx context.Context, c *calling) (string, bool, error) {
	switch tool := c.row.Tool; {
	case tool == toolTell:
		return w.tell(ctx, c)
	case tool == toolAskPerson:
		return w.askPerson(ctx, c)
	case tool == toolAskTeam:
		return w.askTeam(ctx, c)
	case tool == toolReadThread:
		return w.readThread(c)
	case tool == toolWriteNotes:
		return w.writeNotes(ctx, c)
	case tool == toolDocs:
		return w.listDocs(c)
	case tool == toolReadDoc:
		return w.readDoc(c)
	case tool == toolSearchDocs:
		return w.searchDocs(c)
	case tool == toolWriteDoc || does(tool):
		return w.build(ctx, c)
	}
	return fmt.Sprintf("Bulldog does no tool called %q, so nothing was done.", c.row.Tool), false, nil
}

// tell says something that needs no answer to the asker, or to a person
// of the worker's team.
func (w *Work) tell(ctx context.Context, c *calling) (string, bool, error) {
	text := str(c.args["text"])
	if strings.TrimSpace(text) == "" {
		return "A tell needs words, and this one had none, so nobody was told.", false, nil
	}
	to, name, refusal, err := w.whom(c, str(c.args["who"]), "tell", "told")
	if err != nil || refusal != "" {
		return refusal, false, err
	}
	leaves, err := w.leaves(c.team, c.r, to)
	if err != nil {
		return "", false, err
	}
	if leaves {
		return w.disclose(ctx, c, decisions.Said{What: decisions.SaysTell, To: to, Text: text})
	}
	if _, err := w.putOnce(c.row, requests.KindTold, to, text); err != nil {
		return "", false, err
	}
	return toldText(name), false, nil
}

func toldText(name string) string {
	return fmt.Sprintf("Told %s. It asks nothing of them, so wait for no reply.", name)
}

// askPerson asks a person of the worker's team, or the asker, and waits
// for the answer.
func (w *Work) askPerson(ctx context.Context, c *calling) (string, bool, error) {
	question := str(c.args["question"])
	if strings.TrimSpace(question) == "" {
		return "A question needs words, and this one had none, so nobody was asked.", false, nil
	}
	to, _, refusal, err := w.whom(c, str(c.args["who"]), "ask", "asked")
	if err != nil || refusal != "" {
		return refusal, false, err
	}
	leaves, err := w.leaves(c.team, c.r, to)
	if err != nil {
		return "", false, err
	}
	if leaves {
		return w.disclose(ctx, c, decisions.Said{What: decisions.SaysQuestion, To: to, Text: question})
	}
	e, err := w.putOnce(c.row, requests.KindQuestion, to, question)
	if err != nil {
		return "", false, err
	}
	c.row.State, c.row.Entry = waiting, e.ID
	return "", true, w.keepCall(c.row)
}

// askTeam asks another team of the office, as a sub-request, and waits
// for its answer.
func (w *Work) askTeam(ctx context.Context, c *calling) (string, bool, error) {
	name, question := str(c.args["team"]), str(c.args["question"])
	if strings.TrimSpace(question) == "" {
		return "A question needs words, and this one had none, so no team was asked.", false, nil
	}
	_, above, err := w.requests.Top(c.r.Number)
	if err != nil {
		return "", false, err
	}
	o := w.office
	team := teamNamed(o, name)
	switch {
	case team == c.team:
		return fmt.Sprintf("%s is your own team: inside a team, workers use each other as tools, so ask your colleagues as tools, and no team was asked.",
			c.team.Called), false, nil
	case team == nil:
		var may []string
		for _, t := range MayAsk(o, c.team.Name, c.worker, above) {
			may = append(may, t.Name)
		}
		return fmt.Sprintf("The office has no team called %q, so no team was asked: you may ask %s.", name, orList(may)), false, nil
	}
	if err := tooDeep(c.r.Number, above); err != nil {
		return "", false, err
	}
	if i := slices.IndexFunc(above, func(r *requests.Request) bool { return r.Team == team.Name }); i >= 0 {
		return fmt.Sprintf("This request came from %s, in request %d, and your answer goes back to it: answer with what you need from %s, and no team was asked.",
			inSentence(team.Called), above[i].Number, inSentence(team.Called)), false, nil
	}
	if c.team.HasDisclosure {
		return w.disclose(ctx, c, decisions.Said{What: decisions.SaysSub, Team: team.Name, Text: question})
	}
	return "", true, w.sub(c.row, c.r.Number, w.requests.Worker(c.worker), team.Name, question)
}

// sub makes the sub-request a call asks, once: the call's run is its
// ticket.
func (w *Work) sub(row *callRow, n int64, by requests.Who, team, question string) error {
	sub, err := w.requests.Sub(n, by, team, question, row.Run)
	if err != nil {
		return err
	}
	if err := w.keepSub(sub.Number); err != nil {
		return err
	}
	row.State, row.Sub = waiting, sub.Number
	if err := w.keepCall(row); err != nil {
		return err
	}
	// It may hold an answer already, as when Bulldog stopped before it
	// was given.
	w.touch(sub.Number)
	return nil
}

// noteName is what a note of a team is named by: a plain name, with no
// path in it.
var noteName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// writeNotes writes one of the worker's own team's notes, which Changes
// commits.
func (w *Work) writeNotes(ctx context.Context, c *calling) (string, bool, error) {
	name, text, why := str(c.args["name"]), str(c.args["text"]), str(c.args["why"])
	switch {
	case !noteName.MatchString(name):
		return fmt.Sprintf("%q is not a note's name: a note of your team is named by a plain name, such as fuses, with no path in it, so nothing was written.",
			name), false, nil
	case strings.TrimSpace(why) == "":
		return fmt.Sprintf("The note %s says no why, so nothing was written: say why it is written, in a line.", name), false, nil
	}
	commit, err := w.changes.WriteNote(ctx, changes.Note{Team: c.team.Name, Name: name, Text: text, Why: why, Worker: c.worker, Request: c.r.Number})
	written := fmt.Sprintf("The note %s of %s is written, as you gave it.", name, inSentence(c.team.Called))
	switch {
	case errors.Is(err, changes.ErrNoteAsItIs):
		// Written already: as when Bulldog stopped before its answer was
		// kept.
		return written, false, nil
	case err != nil && commit != "":
		w.trouble(c.r.Number, c.r.Team, fmt.Sprintf("%s wrote the note %s, and it is kept, but then: %v", c.worker, name, err))
		return written, false, nil
	case err != nil:
		w.trouble(c.r.Number, c.r.Team, fmt.Sprintf("%s could not write the note %s: %v", c.worker, name, err))
		return fmt.Sprintf("The note %s was not written: Bulldog could not write it, and the people of %s are told why.", name, inSentence(c.team.Called)),
			false, nil
	}
	return written, false, nil
}

// build hands a call of the builder's tools, or of write_doc, to Changes.
// Its job has run, after its yes where it was held, so a write is
// written as Changes answers it.
func (w *Work) build(ctx context.Context, c *calling) (string, bool, error) {
	if c.again && (c.row.Tool == toolOfficePut || c.row.Tool == toolWriteDoc) {
		// Bulldog stopped while it was being written: it is not written
		// twice.
		return "Bulldog stopped while your change was being written, and cannot tell whether it was: look at the office's files before putting it again.",
			false, nil
	}
	a, err := w.changes.Answer(ctx, changes.Call{Request: c.r.Number, Worker: c.worker, Tool: c.row.Tool, Args: c.raw})
	if err != nil {
		w.trouble(c.r.Number, c.r.Team, fmt.Sprintf("%s's %s could not be done: %v", c.worker, c.row.Tool, err))
		return fmt.Sprintf("Your %s was not done: Bulldog failed in doing it, and the people of %s are told why.", c.row.Tool, inSentence(c.team.Called)),
			false, nil
	}
	return a.Text, false, nil
}

// whom gives whom a question or a tell is for: a person of the worker's
// team by their name, or, with nobody named, the asker when that is a
// person or a guest. Anyone else is refused with a sentence, and nobody
// is asked or told in their place.
func (w *Work) whom(c *calling, who, verb, verbed string) (requests.Who, string, string, error) {
	o := w.office
	people, err := w.requests.People(c.r.Team)
	if err != nil {
		return requests.Who{}, "", "", err
	}
	var called []string
	for _, p := range people {
		called = append(called, calledPerson(o, p))
	}
	may := fmt.Sprintf("you may %s %s", verb, orList(called))
	if len(called) == 0 {
		may = fmt.Sprintf("%s has nobody to %s", inSentence(c.team.Called), verb)
	}
	asker, err := w.requests.Asks(c.r.Number)
	if err != nil {
		return requests.Who{}, "", "", err
	}
	if strings.TrimSpace(who) == "" {
		switch asker.Kind {
		case requests.IsPerson:
			return w.requests.Person(asker.Name), asker.Called, "", nil
		case requests.IsGuest:
			if !w.reaches(c.r) {
				return requests.Who{}, "", fmt.Sprintf("Whoever asked request %d is a guest, reached only where they asked, on %s, which carries nothing now, so nobody was %s: %s.",
					c.r.Number, c.r.Origin.Bridge, verbed, may), nil
			}
			return c.r.Asker, "whoever asked", "", nil
		case requests.IsWorker:
			return requests.Who{}, "", fmt.Sprintf("The asker of request %d is %s, a worker of %s, which reads only your answer, so nobody was %s: %s.",
				c.r.Number, asker.Name, inSentence(calledTeam(o, asker.Team)), verbed, may), nil
		case requests.IsTeam:
			return requests.Who{}, "", fmt.Sprintf("Request %d was asked by the routine of %s, which has nobody to %s, so nobody was %s: %s.",
				c.r.Number, inSentence(asker.Called), verb, verbed, may), nil
		}
		return requests.Who{}, "", fmt.Sprintf("Whoever asked request %d is no longer in the office, so nobody was %s: %s.", c.r.Number, verbed, may), nil
	}
	for _, p := range people {
		if strings.EqualFold(who, p) || strings.EqualFold(who, calledPerson(o, p)) {
			return w.requests.Person(p), calledPerson(o, p), "", nil
		}
	}
	if asker.Kind == requests.IsPerson && (strings.EqualFold(who, asker.Name) || strings.EqualFold(who, asker.Called)) {
		return w.requests.Person(asker.Name), asker.Called, "", nil
	}
	return requests.Who{}, "", fmt.Sprintf("%s is neither a person of %s nor whoever asked, so nobody was %s in their place: %s.",
		who, inSentence(c.team.Called), verbed, may), nil
}

// reaches says whether anything reaches a guest asker: they are reached
// only where they asked, on a bridge that runs.
func (w *Work) reaches(r *requests.Request) bool {
	if r.Origin.Kind != requests.AtPlace {
		return false
	}
	b := w.office.Bridges[r.Origin.Bridge]
	return b != nil && b.Runs()
}

// leaves says whether what a team says in a request to someone leaves it
// through its disclosure check: the team sets one, and anyone who reads
// it is outside the team, as the requests say.
func (w *Work) leaves(team *office.Team, r *requests.Request, to requests.Who) (bool, error) {
	if !team.HasDisclosure {
		return false, nil
	}
	return w.requests.Leaves(r.Number, to)
}

// disclose hands what a worker would say outside its team to its disclosure
// check, once, and the call waits until it is let go or refused. One
// handed in before Bulldog stopped is found among the request's
// decisions.
func (w *Work) disclose(ctx context.Context, c *calling, said decisions.Said) (string, bool, error) {
	said.Request, said.By, said.Ticket = c.r.Number, w.requests.Worker(c.worker), c.row.Run
	decs, err := w.decide.OfRequest(c.r.Number)
	if err != nil {
		return "", false, err
	}
	id := int64(0)
	for _, d := range decs {
		if d.Said != nil && d.Said.Ticket == c.row.Run {
			id = d.ID
		}
	}
	if id == 0 {
		if id, err = w.decide.Disclose(ctx, said); err != nil {
			return "", false, err
		}
	}
	c.row.State, c.row.Decision = held, id
	return "", true, w.keepCall(c.row)
}

// putOnce puts a question or a tell for a call, once: one put before
// Bulldog stopped, found in the thread after the call's mark and not kept
// for any other call, is taken as it.
func (w *Work) putOnce(row *callRow, kind string, to requests.Who, text string) (*requests.Entry, error) {
	thread, err := w.requests.Thread(row.Request)
	if err != nil {
		return nil, err
	}
	calls, err := w.callsOf(row.Request)
	if err != nil {
		return nil, err
	}
	claimed := map[int64]bool{}
	for _, c := range calls {
		if c.Run != row.Run {
			claimed[c.Entry] = true
		}
	}
	for _, e := range thread {
		if row.Mark != 0 && e.ID > row.Mark && !claimed[e.ID] && e.Kind == kind && e.By.Kind == requests.IsWorker && e.By.Name == row.Worker &&
			e.Text == text && len(e.To) == 1 && sameWho(e.To[0], to) {
			return &e, nil
		}
	}
	return w.requests.Put(row.Request, w.requests.Worker(row.Worker), kind, to, text, nil)
}

// sameWho says whether two parties are the same one.
func sameWho(a, b requests.Who) bool {
	return a.Kind == b.Kind && a.Name == b.Name && a.Bridge == b.Bridge && a.ID == b.ID
}

// teamNamed gives the team of the office a name names, by its name or by
// what it is called.
func teamNamed(o *office.Office, name string) *office.Team {
	name = strings.TrimSpace(name)
	if t := o.Teams[name]; t != nil {
		return t
	}
	for _, t := range o.Teams {
		if strings.EqualFold(name, t.Name) || strings.EqualFold(name, t.Called) || strings.EqualFold(name, inSentence(t.Called)) {
			return t
		}
	}
	return nil
}

func calledPerson(o *office.Office, name string) string {
	if p := o.People[name]; p != nil {
		return p.Called
	}
	return name
}

// orList writes names as a sentence lists them: "a, b or c".
func orList(names []string) string {
	switch len(names) {
	case 0:
		return "nobody"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// sentence gives words as one sentence: begun with a capital, ended with
// a full stop.
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	s = string(unicode.ToUpper(r)) + s[size:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// str gives an argument as text.
func str(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	}
	b, _ := json.Marshal(v)
	return string(b)
}
