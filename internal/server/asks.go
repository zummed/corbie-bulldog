package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// How much of a thread is given at a time: as many entries as Thread,
// unless more or fewer are asked for, and no more than ThreadMost.
const (
	Thread     = 50
	ThreadMost = 200
)

// How many requests a list gives at a time: as many as Requests, unless
// more or fewer are asked for, and no more than RequestsMost.
const (
	Requests     = 100
	RequestsMost = 500
)

// A request as the API gives it.
type (
	summaryOut struct {
		Number int64            `json:"number"`
		Whole  bool             `json:"whole"`
		Team   string           `json:"team,omitempty"`
		Asker  *partyOut        `json:"asker,omitempty"`
		Origin *requests.Origin `json:"origin,omitempty"`
		Parent int64            `json:"parent,omitempty"`
		State  string           `json:"state,omitempty"`
		On     []partyOut       `json:"on,omitempty"`
		Source *requests.Status `json:"source,omitempty"`
		First  string           `json:"first,omitempty"`
		Made   *time.Time       `json:"made,omitempty"`
	}
	partyOut struct {
		Kind   string   `json:"kind"`
		Name   string   `json:"name,omitempty"`
		Called string   `json:"called,omitempty"`
		Teams  []string `json:"teams,omitempty"`
		Bridge string   `json:"bridge,omitempty"`
		ID     string   `json:"id,omitempty"`
		Team   string   `json:"team,omitempty"`
	}
	viewOut struct {
		summaryOut
		// Read says the one looking sees what was read of its team.
		Read    bool       `json:"read"`
		Entries []entryOut `json:"entries"`
		// Before is the entry to name to reach further back, when there
		// is more of the thread before what is given.
		Before    int64         `json:"before,omitempty"`
		Subs      []subOut      `json:"subs"`
		Decisions []decisionOut `json:"decisions"`
		// Runs are its runs, and Records what each read, as Corbie keeps
		// it, to the people of its team.
		Runs    []runOut    `json:"runs,omitempty"`
		Records []recordOut `json:"records,omitempty"`
		Waits   []int64     `json:"waits_on_me"`
	}
	subOut struct {
		Number   int64      `json:"number"`
		Team     string     `json:"team"`
		Question string     `json:"question"`
		State    string     `json:"state"`
		On       []partyOut `json:"on,omitempty"`
		Answer   string     `json:"answer,omitempty"`
		Subs     []subOut   `json:"subs"`
	}
	runOut struct {
		Name    string     `json:"name"`
		Kind    string     `json:"kind"`
		Team    string     `json:"team"`
		Worker  string     `json:"worker,omitempty"`
		Started time.Time  `json:"started"`
		Ended   *time.Time `json:"ended,omitempty"`
		Status  string     `json:"status"`
		How     string     `json:"how,omitempty"`
		Tokens  int        `json:"tokens"`
	}
	recordOut struct {
		Run    string          `json:"run"`
		Team   string          `json:"team"`
		Worker string          `json:"worker,omitempty"`
		Gone   bool            `json:"gone,omitempty"`
		Events json.RawMessage `json:"events,omitempty"`
	}
	entryOut struct {
		ID      int64          `json:"id"`
		Kind    string         `json:"kind"`
		By      requests.Who   `json:"by"`
		To      []requests.Who `json:"to"`
		At      time.Time      `json:"at"`
		Text    string         `json:"text"`
		Files   []fileOut      `json:"files"`
		In      requests.Where `json:"in"`
		Read    string         `json:"read,omitempty"`
		Answers int64          `json:"answers,omitempty"`
		Sub     int64          `json:"sub,omitempty"`
		Carried []carriedOut   `json:"carried,omitempty"`
	}
	fileOut struct {
		Name string `json:"name"`
		Size int    `json:"size"`
		URL  string `json:"url"`
	}
	carriedOut struct {
		Bridge string `json:"bridge"`
		Place  string `json:"place,omitempty"`
		Person string `json:"person,omitempty"`
		State  string `json:"state"`
		Why    string `json:"why,omitempty"`
	}
	decisionOut struct {
		ID        int64     `json:"id"`
		Kind      string    `json:"kind"`
		Team      string    `json:"team"`
		Does      string    `json:"does"`
		Asked     []string  `json:"asked"`
		Referrals []wordOut `json:"referrals"`
		Ended     string    `json:"ended,omitempty"`
		By        *wordOut  `json:"by,omitempty"`
		Why       string    `json:"why,omitempty"`
		Mine      bool      `json:"mine"`
	}
	// wordOut is a decider's word, with the team a worker is of.
	wordOut struct {
		decisions.Word
		Team string `json:"team,omitempty"`
	}
)

// decisionOf gives a decision as the one looking was asked it, built from
// the office as read now, each worker that referred or ended it with the
// team it is of.
func decisionOf(o *office.Office, d *decisions.Decision, person string) decisionOut {
	out := decisionOut{ID: d.ID, Kind: d.Kind, Team: d.Team, Does: d.Does(o), Asked: orEmpty(d.Asked), Referrals: []wordOut{},
		Ended: d.Ended, Why: d.Why, Mine: d.Ended == "" && slices.Contains(d.Asked, person)}
	word := func(w decisions.Word) wordOut { return wordOut{Word: w, Team: teamOfWorker(o, w.Kind, w.Name)} }
	for _, w := range d.Referrals {
		out.Referrals = append(out.Referrals, word(w))
	}
	if d.By != nil {
		by := word(*d.By)
		out.By = &by
	}
	return out
}

// partyOf gives someone as the office is read now.
func partyOf(p requests.Party) partyOut {
	return partyOut{Kind: p.Kind, Name: p.Name, Called: p.Called, Teams: p.Teams, Bridge: p.Bridge, ID: p.ID, Team: p.Team}
}

func partiesOf(ps []requests.Party) []partyOut {
	out := []partyOut{}
	for _, p := range ps {
		out = append(out, partyOf(p))
	}
	return out
}

// list gives the requests the one looking may see, newest first, as the
// address narrows them: to a team, a state, those they asked, or those
// waiting on them; and of them a part, those numbered below before, and
// how many; with the number to give as before for the next part, when
// more are left.
func (s *site) list(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query()
	var f requests.Filter
	before, limit := int64(0), Requests
	for name, values := range q {
		if len(values) != 1 {
			return nil, unreadable("The address gives %s more than once.", name)
		}
		switch v := values[0]; name {
		case "team":
			f.Team = v
		case "state":
			if !slices.Contains([]string{requests.Working, requests.Waiting, requests.Answered}, v) {
				return nil, unreadable("A state is working, waiting or answered, and not %q.", v)
			}
			f.State = v
		case "asked", "waiting":
			if v != "me" {
				return nil, unreadable("%s is me, or not given, and not %q.", name, v)
			}
			f.Asked = f.Asked || name == "asked"
			f.WaitsOnMe = f.WaitsOnMe || name == "waiting"
		case "before":
			n, err := counted(name, v)
			if err != nil {
				return nil, err
			}
			before = n
		case "limit":
			n, err := counted(name, v)
			if err != nil {
				return nil, err
			}
			limit = int(min(n, RequestsMost))
		default:
			return nil, unreadable("The address gives %s, which is not taken here.", name)
		}
	}
	sums, err := s.Requests.List(person, f)
	if err != nil {
		return nil, err
	}
	shown, next := listPage(sums, before, limit)
	out := []summaryOut{}
	for _, sum := range shown {
		// Where a request was asked is given of a request at the top of
		// its tree; a sub-request was asked at no place, and the one
		// looking may not see the request it was asked from.
		var origin *requests.Origin
		if sum.Whole {
			req, err := s.Requests.Get(sum.Number)
			if err != nil {
				return nil, err
			}
			if req.Parent == 0 {
				origin = &req.Origin
			}
		}
		out = append(out, summaryOf(sum, origin))
	}
	if next != 0 {
		return map[string]any{"requests": out, "before": next}, nil
	}
	return map[string]any{"requests": out}, nil
}

// listPage gives part of a list of requests, newest first: as many as the
// limit of those numbered below before, or of all; and the number to give
// as before for the next part, when more are left. A part is cut by
// number, so a request made since the last part was given neither moves
// nor repeats one.
func listPage(sums []requests.Summary, before int64, limit int) ([]requests.Summary, int64) {
	if before != 0 {
		sums = slices.DeleteFunc(slices.Clone(sums), func(sum requests.Summary) bool { return sum.Number >= before })
	}
	if len(sums) <= limit {
		return sums, 0
	}
	return sums[:limit], sums[limit-1].Number
}

// summaryOf gives a request as a list shows it: all of it to whoever sees
// it whole, and its number alone to anyone else.
func summaryOf(sum requests.Summary, origin *requests.Origin) summaryOut {
	if !sum.Whole {
		return summaryOut{Number: sum.Number}
	}
	asker := partyOf(sum.Asker)
	made := sum.Made
	return summaryOut{Number: sum.Number, Whole: true, Team: sum.Team, Asker: &asker, Origin: origin, Parent: sum.Parent,
		State: sum.State.Is, On: partiesOf(sum.State.On), Source: sum.Source, First: sum.First, Made: &made}
}

// request gives one request as the one looking sees it: what was done,
// what was read of its team when they are its person, its thread in part,
// its sub-requests as they see them, the decisions they were asked, and
// its runs to its team's people.
func (s *site) request(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	n, err := number(r, "n")
	if err != nil {
		return nil, err
	}
	before, size, err := paging(r)
	if err != nil {
		return nil, err
	}
	v, err := s.Requests.View(n, person)
	if err != nil {
		return nil, err
	}
	waits, err := s.Requests.WaitingOn(person)
	if err != nil {
		return nil, err
	}
	sum := requests.Summary{Number: v.Number, Whole: v.Whole, Team: v.Team, Asker: v.Asker, State: v.State, Source: v.Source, Made: v.Made, Parent: v.Parent}
	out := viewOut{summaryOut: summaryOf(sum, v.Origin), Read: v.Read, Entries: []entryOut{}, Subs: subsOf(v.Subs), Decisions: []decisionOut{}, Waits: []int64{}}
	open := map[int64]bool{}
	for _, wt := range waits {
		if wt.Request == n && wt.Entry != 0 {
			open[wt.Entry] = true
			out.Waits = append(out.Waits, wt.Entry)
		}
	}
	shown, back, err := page(v.Entries, before, size, open)
	if err != nil {
		return nil, err
	}
	out.Before = back
	carried, err := s.Bridges.Carried(n)
	if err != nil {
		return nil, err
	}
	for _, e := range shown {
		out.Entries = append(out.Entries, entryOf(e, carried))
	}
	ds, err := s.Decisions.OfRequest(n)
	if err != nil {
		return nil, err
	}
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		if slices.Contains(d.WasAsked, person) {
			out.Decisions = append(out.Decisions, decisionOf(o, d, person))
		}
	}
	if v.Read {
		runs, err := s.Work.Runs(n)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			out.Runs = append(out.Runs, runOf(run))
		}
		records, err := s.Requests.WhatWasRead(n, person)
		if err != nil {
			return nil, err
		}
		for _, rec := range records {
			out.Records = append(out.Records, recordOut{Run: rec.Run, Team: rec.Team, Worker: rec.Worker, Gone: rec.Gone, Events: rec.Events})
		}
	}
	return out, nil
}

// subsOf gives sub-requests as the request above them shows them.
func subsOf(subs []requests.Sub) []subOut {
	out := []subOut{}
	for _, sub := range subs {
		out = append(out, subOut{Number: sub.Number, Team: sub.Team, Question: sub.Question, State: sub.State.Is, On: partiesOf(sub.State.On),
			Answer: sub.Answer, Subs: subsOf(sub.Subs)})
	}
	return out
}

// runOf gives a run on a request as it is kept with it.
func runOf(r work.KeptRun) runOut {
	out := runOut{Name: r.Name, Kind: r.Kind, Team: r.Team, Worker: r.Worker, Started: r.Started, Status: r.Status, How: r.How, Tokens: r.Tokens}
	if !r.Ended.IsZero() {
		out.Ended = &r.Ended
	}
	return out
}

// paging reads which part of a thread is asked for: before an entry, and
// how many.
func paging(r *http.Request) (before int64, size int, err error) {
	size = Thread
	for name, values := range r.URL.Query() {
		if len(values) != 1 {
			return 0, 0, unreadable("The address gives %s more than once.", name)
		}
		n, err := counted(name, values[0])
		if err != nil {
			return 0, 0, err
		}
		switch name {
		case "before":
			before = n
		case "size":
			size = int(min(n, ThreadMost))
		default:
			return 0, 0, unreadable("The address gives %s, which is not taken here.", name)
		}
	}
	return before, size, nil
}

// counted reads a number above nothing that the address gives.
func counted(name, value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, unreadable("The address's %s is to be a number above nothing, and is %q.", name, value)
	}
	return n, nil
}

// page gives part of a thread: the size latest entries before the entry
// named, or the latest of all; with them always its first entry, and each
// earlier one that waits on the one looking; and the entry to name to
// reach further back, when there is more before them.
func page(entries []requests.Entry, before int64, size int, open map[int64]bool) ([]requests.Entry, int64, error) {
	end := len(entries)
	if before != 0 {
		end = slices.IndexFunc(entries, func(e requests.Entry) bool { return e.ID == before })
		if end < 0 {
			return nil, 0, errNotThere
		}
	}
	start := max(0, end-size)
	var shown []requests.Entry
	more := false
	for i, e := range entries[:start] {
		if i == 0 || open[e.ID] {
			shown = append(shown, e)
		} else {
			more = true
		}
	}
	shown = append(shown, entries[start:end]...)
	var back int64
	if more {
		back = entries[start].ID
	}
	return shown, back, nil
}

// entryOf gives an entry, with where it was carried.
func entryOf(e requests.Entry, carried []bridges.Carried) entryOut {
	out := entryOut{ID: e.ID, Kind: e.Kind, By: e.By, To: orEmpty(e.To), At: e.At, Text: e.Text, Files: []fileOut{}, In: e.In, Read: e.Read, Answers: e.Answers, Sub: e.Sub}
	for i, f := range e.Files {
		out.Files = append(out.Files, fileOut{Name: f.Name, Size: f.Size, URL: fmt.Sprintf("/files/%d-%d", e.ID, i)})
	}
	for _, c := range carried {
		if c.Entry == e.ID {
			out.Carried = append(out.Carried, carriedOut{Bridge: c.Bridge, Place: c.Place, Person: c.Person, State: c.State, Why: c.Why})
		}
	}
	return out
}

// ask is something asked in the chat, of a team or of the office.
func (s *site) ask(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	in, err := readSaid(w, r, map[string]string{"team": kindText, "text": kindText})
	if err != nil {
		return nil, err
	}
	words, ok := in.texts["text"]
	if !ok {
		return nil, unreadable("The body gives no text.")
	}
	team := in.texts["team"]
	if team == "" {
		team = office.Top
	}
	asked, err := s.Requests.AskInChat(person, team, words, in.files)
	if err != nil {
		return nil, err
	}
	return map[string]int64{"number": asked.Number}, nil
}

// sayMore says more in a request, or, with re, answers what was put to
// the one looking in it.
func (s *site) sayMore(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	n, err := number(r, "n")
	if err != nil {
		return nil, err
	}
	in, err := readSaid(w, r, map[string]string{"text": kindText, "re": kindNumber})
	if err != nil {
		return nil, err
	}
	words, ok := in.texts["text"]
	if !ok {
		return nil, unreadable("The body gives no text.")
	}
	if _, err := s.Requests.View(n, person); err != nil {
		return nil, err
	}
	who := s.Requests.Person(person)
	if id, ok := in.numbers["re"]; ok {
		if err := s.within(n, id); err != nil {
			return nil, err
		}
		e, err := s.Requests.Reply(id, who, words, in.files, requests.Screens)
		if err != nil {
			return nil, err
		}
		return map[string]int64{"entry": e.ID}, nil
	}
	e, err := s.Requests.Say(n, who, words, in.files, requests.Screens, false)
	if err != nil {
		return nil, err
	}
	return map[string]int64{"entry": e.ID}, nil
}

// within says an entry is one of a request's: named under another, it is
// not there.
func (s *site) within(n, entry int64) error {
	e, err := s.Requests.Entry(entry)
	if err != nil {
		return err
	}
	if e.Request != n {
		return errNotThere
	}
	return nil
}

// decide is a person's yes or refusal on a decision of a request, with
// their note.
func (s *site) decide(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	n, err := number(r, "n")
	if err != nil {
		return nil, err
	}
	id, err := number(r, "id")
	if err != nil {
		return nil, err
	}
	var body struct {
		Says *string `json:"says"`
		Note *string `json:"note"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return nil, err
	}
	if body.Says == nil || *body.Says != decisions.SaysYes && *body.Says != decisions.SaysRefuse {
		return nil, unreadable("The body's says is to be yes or refuse.")
	}
	if _, err := s.Requests.View(n, person); err != nil {
		return nil, err
	}
	d, err := s.Decisions.Get(id)
	if err != nil {
		return nil, err
	}
	if d.Request != n {
		return nil, errNotThere
	}
	note := ""
	if body.Note != nil {
		note = *body.Note
	}
	reply, err := s.Decisions.Decide(r.Context(), id, s.Requests.Person(person), *body.Says, note)
	if err != nil {
		return nil, err
	}
	if !reply.Taken {
		return nil, refusal(reply.Text)
	}
	return map[string]string{"said": reply.Text}, nil
}

// stop stops a request, and what is open beneath it, for someone who may.
func (s *site) stop(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	n, err := number(r, "n")
	if err != nil {
		return nil, err
	}
	if err := readNothing(w, r); err != nil {
		return nil, err
	}
	if _, err := s.Requests.View(n, person); err != nil {
		return nil, err
	}
	may, err := s.mayStop(n, person)
	if err != nil {
		return nil, err
	}
	if !may {
		return nil, refusal("You may not stop this request: its asker, the people of the team it went to, and whoever may stop a request it is beneath may.")
	}
	if err := s.Work.Stop(r.Context(), n, s.Requests.Person(person)); err != nil {
		return nil, err
	}
	return map[string]string{"said": "It is stopped, and what was open beneath it."}, nil
}

// mayStop says whether a person may stop a request: one they asked, one
// whose team they are among the people of, or one beneath a request they
// may stop.
func (s *site) mayStop(n int64, person string) (bool, error) {
	for n != 0 {
		r, err := s.Requests.Get(n)
		if err != nil {
			return false, err
		}
		asker, err := s.Requests.Asks(n)
		if err != nil {
			return false, err
		}
		if asker.Kind == requests.IsPerson && asker.Name == person {
			return true, nil
		}
		people, err := s.Requests.People(r.Team)
		if err != nil {
			return false, err
		}
		if slices.Contains(people, person) {
			return true, nil
		}
		n = r.Parent
	}
	return false, nil
}

// routineNow runs one of a team's routines now, named by what is written
// in it, for one of its people.
func (s *site) routineNow(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	var body struct {
		At  *string `json:"at"`
		To  *string `json:"to"`
		Say *string `json:"say"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return nil, err
	}
	if body.At == nil || body.To == nil || body.Say == nil {
		return nil, unreadable("The body names a routine by its at, to and say, each as it is written.")
	}
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	t := o.Teams[r.PathValue("team")]
	if t == nil {
		return nil, errNotThere
	}
	people, err := s.Requests.People(t.Name)
	if err != nil {
		return nil, err
	}
	at := slices.IndexFunc(t.Routines, func(rt office.Routine) bool { return rt.At == *body.At && rt.To == *body.To && rt.Say == *body.Say })
	if !slices.Contains(people, person) || at < 0 {
		return nil, errNotThere
	}
	asked, err := s.Requests.AskByRoutine(t.Name, t.Routines[at], s.Requests.Person(person))
	if err != nil {
		return nil, err
	}
	return map[string]int64{"number": asked.Number}, nil
}
