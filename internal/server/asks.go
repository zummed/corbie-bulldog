package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
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
		Number int64      `json:"number"`
		Whole  bool       `json:"whole"`
		Team   string     `json:"team,omitempty"`
		Asker  *partyOut  `json:"asker,omitempty"`
		Origin *originOut `json:"origin,omitempty"`
		Parent int64      `json:"parent,omitempty"`
		State  string     `json:"state,omitempty"`
		// Queued is its place in the queue, from 1, while it waits its
		// turn to work.
		Queued int        `json:"queued,omitempty"`
		Label  string     `json:"label,omitempty"`
		On     []partyOut `json:"on,omitempty"`
		Source *sourceOut `json:"source,omitempty"`
		First  string     `json:"first,omitempty"`
		Made   *time.Time `json:"made,omitempty"`
	}
	// originOut is where a request was asked, with the person who ran a
	// routine now as the office is read now.
	originOut struct {
		Kind   string    `json:"kind"`
		Bridge string    `json:"bridge,omitempty"`
		Heard  string    `json:"heard,omitempty"`
		Place  string    `json:"place,omitempty"`
		About  string    `json:"about,omitempty"`
		Team   string    `json:"team,omitempty"`
		At     string    `json:"at,omitempty"`
		To     string    `json:"to,omitempty"`
		Say    string    `json:"say,omitempty"`
		By     *partyOut `json:"by,omitempty"`
	}
	// sourceOut is a request's status at its source, with who changed it
	// as the office is read now.
	sourceOut struct {
		Status string    `json:"status"`
		Closed bool      `json:"closed,omitempty"`
		By     partyOut  `json:"by"`
		At     time.Time `json:"at"`
		Link   string    `json:"link,omitempty"`
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
		// Questions are those put to people in it that the one looking
		// sees, and Answer where its answer stands, to whoever sees what
		// was done.
		Questions []questionOut `json:"questions"`
		Answer    *answerOut    `json:"answer,omitempty"`
		// Disclosure, Runs, and Records, what each run read as Corbie
		// keeps it, are given to the people of its team.
		Disclosure *disclosureOut `json:"disclosure,omitempty"`
		Runs       []runOut       `json:"runs,omitempty"`
		Records    []recordOut    `json:"records,omitempty"`
		Waits      []int64        `json:"waits_on_me"`
	}
	subOut struct {
		Number    int64         `json:"number"`
		Team      string        `json:"team"`
		Question  string        `json:"question"`
		State     string        `json:"state"`
		Queued    int           `json:"queued,omitempty"`
		Label     string        `json:"label,omitempty"`
		On        []partyOut    `json:"on,omitempty"`
		Answer    string        `json:"answer,omitempty"`
		Decisions []decisionOut `json:"decisions"`
		Subs      []subOut      `json:"subs"`
	}
	// questionOut is a question put to people, and whether it is
	// answered.
	questionOut struct {
		Entry    int64      `json:"entry"`
		By       partyOut   `json:"by"`
		To       []partyOut `json:"to"`
		At       time.Time  `json:"at"`
		Answered bool       `json:"answered"`
	}
	// answerOut is where a request's answer stands: it waits, is held by
	// its team's disclosure, to the team's people alone, was said, or
	// not delivered, with why, or its lead answered with nothing.
	answerOut struct {
		State string `json:"state"`
		Entry int64  `json:"entry,omitempty"`
		Why   string `json:"why,omitempty"`
	}
	// disclosureOut is whether a request's team sets a disclosure, who
	// decides it in the order the walk asks them, and whether its answer
	// needs it: whether anyone who reads the answer is outside the team.
	disclosureOut struct {
		Set      bool         `json:"set"`
		Needed   bool         `json:"needed"`
		Deciders []deciderOut `json:"deciders,omitempty"`
	}
	runOut struct {
		Name    string     `json:"name"`
		Kind    string     `json:"kind"`
		Team    string     `json:"team"`
		Request int64      `json:"request,omitempty"`
		Worker  string     `json:"worker,omitempty"`
		Started time.Time  `json:"started"`
		Ended   *time.Time `json:"ended,omitempty"`
		Status  string     `json:"status"`
		Says    string     `json:"says,omitempty"`
		How     string     `json:"how,omitempty"`
		Tokens  int        `json:"tokens"`
		// Given is what a lead's run was given, on the run's own page.
		Given *givenOut `json:"given,omitempty"`
	}
	// givenOut is what a lead's run was given, worked out when shown:
	// the team's notes, each with its size and how much of it a run is
	// given, in characters; the docs it read, each with its team, unless
	// Corbie keeps its record no more; the teams it may ask; the thread
	// it was shown, with how many entries were not shown; what was new
	// for it; and the kind of each decision its entries are of.
	givenOut struct {
		Notes      []noteGivenOut `json:"notes"`
		Docs       []docReadOut   `json:"docs"`
		RecordGone bool           `json:"record_gone,omitempty"`
		MayAsk     []string       `json:"may_ask"`
		Thread     []entryOut     `json:"thread"`
		NotShown   int            `json:"not_shown"`
		New        []entryOut     `json:"new"`
		Decisions  []kindOut      `json:"decisions"`
	}
	kindOut struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	noteGivenOut struct {
		Name  string `json:"name"`
		Size  int    `json:"size"`
		Given int    `json:"given"`
	}
	docReadOut struct {
		Name string `json:"name"`
		Team string `json:"team"`
	}
	recordOut struct {
		Run    string          `json:"run"`
		Team   string          `json:"team"`
		Worker string          `json:"worker,omitempty"`
		Gone   bool            `json:"gone,omitempty"`
		Events json.RawMessage `json:"events,omitempty"`
	}
	entryOut struct {
		ID       int64          `json:"id"`
		Kind     string         `json:"kind"`
		By       partyOut       `json:"by"`
		To       []partyOut     `json:"to"`
		At       time.Time      `json:"at"`
		Text     string         `json:"text"`
		Files    []fileOut      `json:"files"`
		In       requests.Where `json:"in"`
		Read     string         `json:"read,omitempty"`
		Answers  int64          `json:"answers,omitempty"`
		Sub      int64          `json:"sub,omitempty"`
		Decision int64          `json:"decision,omitempty"`
		Noted    bool           `json:"noted,omitempty"`
		Carried  []carriedOut   `json:"carried,omitempty"`
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
	// decisionOut is a decision: what it is, who it waits on, and how it
	// ended and who ended it, with the notes, the walk and an action's
	// arguments to its team's people and those asked it, and what a
	// disclosure would let out to those asked it alone.
	decisionOut struct {
		ID        int64           `json:"id"`
		Kind      string          `json:"kind"`
		Team      string          `json:"team"`
		Does      string          `json:"does"`
		What      string          `json:"what,omitempty"`
		Asked     []string        `json:"asked"`
		Ended     string          `json:"ended,omitempty"`
		By        *wordOut        `json:"by,omitempty"`
		Why       string          `json:"why,omitempty"`
		Mine      bool            `json:"mine"`
		Referrals []wordOut       `json:"referrals,omitempty"`
		Stages    []deciderOut    `json:"stages,omitempty"`
		Args      json.RawMessage `json:"args,omitempty"`
		Leaving   *leavingOut     `json:"leaving,omitempty"`
	}
	// leavingOut is what a disclosure would let out: what it is, its
	// text, whom it is to, where it may in the end be read, and the
	// requests it goes through from the top.
	leavingOut struct {
		What    string       `json:"what"`
		Text    string       `json:"text"`
		To      partyOut     `json:"to"`
		ReadAt  *originOut   `json:"read_at"`
		Through []throughOut `json:"through"`
	}
	throughOut struct {
		Number int64  `json:"number,omitempty"`
		Team   string `json:"team"`
	}
	// wordOut is a decider's word, with the team a worker is of.
	wordOut struct {
		decisions.Word
		Team string `json:"team,omitempty"`
	}
)

// decisionsOf gives the decisions of a request as the one looking may
// see them, built from the office as read now: each they were asked; a
// held action or a change, when they see what was done of the request;
// and a disclosure, what was read of its team until it is let go, when
// they are of the team.
func (s *site) decisionsOf(o *office.Office, n int64, person string, done bool) ([]decisionOut, error) {
	ds, err := s.Decisions.OfRequest(n)
	if err != nil {
		return nil, err
	}
	out := []decisionOut{}
	for _, d := range ds {
		people, err := s.Requests.People(d.Team)
		if err != nil {
			return nil, err
		}
		asked, ofTeam := slices.Contains(d.WasAsked, person), slices.Contains(people, person)
		if !asked && !ofTeam && (!done || d.Kind == decisions.OfDisclosure) {
			continue
		}
		dec, err := s.decisionOf(o, d, person, asked, ofTeam)
		if err != nil {
			return nil, err
		}
		out = append(out, dec)
	}
	return out, nil
}

// decisionOf gives a decision: what it is, who it waits on, and how it
// ended and who ended it, to whoever sees it; each note, the walk and an
// action's arguments to the people of its team and those asked it, each
// worker that referred or ended it with the team it is of; and what a
// disclosure would let out to those asked it alone.
func (s *site) decisionOf(o *office.Office, d *decisions.Decision, person string, asked, ofTeam bool) (decisionOut, error) {
	out := decisionOut{ID: d.ID, Kind: d.Kind, Team: d.Team, Does: d.Does(o), Asked: orEmpty(d.Asked),
		Ended: d.Ended, Why: d.Why, Mine: d.Ended == "" && slices.Contains(d.Asked, person)}
	if d.Said != nil {
		out.What = d.Said.What
	}
	sees := asked || ofTeam
	word := func(w decisions.Word) wordOut {
		if !sees {
			w.Note = ""
		}
		return wordOut{Word: w, Team: teamOfWorker(o, w.Kind, w.Name)}
	}
	if d.By != nil {
		by := word(*d.By)
		out.By = &by
	}
	if !sees {
		return out, nil
	}
	out.Referrals = []wordOut{}
	for _, w := range d.Referrals {
		out.Referrals = append(out.Referrals, word(w))
	}
	out.Stages = deciders(o, d.Walked(o))
	out.Args = d.Args()
	if !asked {
		return out, nil
	}
	l, err := s.Decisions.Leaving(d)
	if err != nil || l == nil {
		return out, err
	}
	out.Leaving = &leavingOut{What: l.What, Text: l.Text, To: partyOf(l.To), ReadAt: originOf(&l.ReadAt, s.Requests.Party), Through: []throughOut{}}
	for _, t := range l.Through {
		out.Leaving.Through = append(out.Leaving.Through, throughOut{Number: t.Number, Team: t.Team})
	}
	return out, nil
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

// stateQueued is the state a list may be narrowed to of the requests
// working that wait their turn, which work, not the requests, says.
const stateQueued = "queued"

// list gives the requests the one looking may see, newest first, as the
// address narrows them: to a team, a state, those they asked, those
// waiting on them, or those asked on a bridge and at a place of it; those
// noted only when they are asked for, and how many there are; and of them
// a part, those numbered below before, and how many; with the number to
// give as before for the next part, when more are left.
func (s *site) list(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query()
	var f requests.Filter
	before, limit := int64(0), Requests
	queued, withNoted := false, false
	for name, values := range q {
		if len(values) != 1 {
			return nil, unreadable("The address gives %s more than once.", name)
		}
		switch v := values[0]; name {
		case "team":
			f.Team = v
		case "state":
			if !slices.Contains([]string{requests.Working, stateQueued, requests.Waiting, requests.Answered}, v) {
				return nil, unreadable("A state is working, queued, waiting or answered, and not %q.", v)
			}
			f.State, queued = v, v == stateQueued
			if queued {
				f.State = requests.Working
			}
		case "asked", "waiting":
			if v != "me" {
				return nil, unreadable("%s is me, or not given, and not %q.", name, v)
			}
			f.Asked = f.Asked || name == "asked"
			f.WaitsOnMe = f.WaitsOnMe || name == "waiting"
		case "bridge":
			f.Bridge = v
		case "place":
			f.Place = v
		case "noted":
			if v != "yes" {
				return nil, unreadable("noted is yes, or not given, and not %q.", v)
			}
			withNoted = true
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
	if f.Place != "" && f.Bridge == "" {
		return nil, unreadable("A place is given with its bridge.")
	}
	sums, err := s.Requests.List(person, f)
	if err != nil {
		return nil, err
	}
	queue := s.Work.Queue()
	noted := 0
	sums = slices.DeleteFunc(sums, func(sum requests.Summary) bool {
		if sum.State.Noted {
			noted++
			return !withNoted
		}
		return f.State == requests.Working && slices.Contains(queue, sum.Number) != queued
	})
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
		out = append(out, summaryOf(sum, origin, queue, s.Requests.Party))
	}
	if next != 0 {
		return map[string]any{"requests": out, "noted": noted, "before": next}, nil
	}
	return map[string]any{"requests": out, "noted": noted}, nil
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
// it whole, with its place in the queue while it waits its turn, and
// anyone its origin and source name as party gives them; and its number
// alone to anyone else.
func summaryOf(sum requests.Summary, origin *requests.Origin, queue []int64, party func(requests.Who) requests.Party) summaryOut {
	if !sum.Whole {
		return summaryOut{Number: sum.Number}
	}
	asker := partyOf(sum.Asker)
	made := sum.Made
	out := summaryOut{Number: sum.Number, Whole: true, Team: sum.Team, Asker: &asker, Parent: sum.Parent,
		State: sum.State.Is, Queued: queuedAt(queue, sum), Label: labelOf(sum.State), On: partiesOf(sum.State.On), First: sum.First, Made: &made,
		Origin: originOf(origin, party)}
	if st := sum.Source; st != nil {
		out.Source = &sourceOut{Status: st.Status, Closed: st.Closed, By: partyOf(party(st.By)), At: st.At, Link: st.Link}
	}
	return out
}

// queuedAt gives where a request working waits its turn in the queue,
// from 1, or 0 when it does not.
func queuedAt(queue []int64, sum requests.Summary) int {
	if sum.State.Is != requests.Working {
		return 0
	}
	return slices.Index(queue, sum.Number) + 1
}

// originOf gives where a request was asked, with the person who ran a
// routine now as party gives them.
func originOf(origin *requests.Origin, party func(requests.Who) requests.Party) *originOut {
	if origin == nil {
		return nil
	}
	out := &originOut{Kind: origin.Kind, Bridge: origin.Bridge, Heard: origin.Heard, Place: origin.Place, About: origin.About,
		Team: origin.Team, At: origin.At, To: origin.To, Say: origin.Say}
	if origin.By.Kind != "" {
		by := partyOf(party(origin.By))
		out.By = &by
	}
	return out
}

// The labels of an answered request, one at most: stopped, noted, or
// quiet, when its lead answered with nothing.
const (
	labelStopped = "stopped"
	labelNoted   = "noted"
	labelQuiet   = "quiet"
)

func labelOf(st requests.State) string {
	switch {
	case st.Stopped:
		return labelStopped
	case st.Noted:
		return labelNoted
	case st.Quiet:
		return labelQuiet
	}
	return ""
}

// The states of a request's answer.
const (
	answerWaits        = "waits"
	answerHeld         = "held"
	answerSaid         = "said"
	answerQuiet        = "quiet"
	answerNotDelivered = "not delivered"
)

// answerOf gives where a request's answer stands: none in a noted
// request; held, to the people of its team alone, while its disclosure
// holds it; said, or not delivered when its bridge gave it up, with why;
// quiet when its lead answered with nothing; and otherwise waiting.
func answerOf(v *requests.View, held bool, carried []bridges.Carried) *answerOut {
	switch {
	case v.State.Noted:
		return nil
	case held:
		return &answerOut{State: answerHeld}
	case v.Answer != 0:
		for _, c := range carried {
			if c.Entry == v.Answer && c.State == bridges.GivenUp {
				return &answerOut{State: answerNotDelivered, Entry: v.Answer, Why: c.Why}
			}
		}
		return &answerOut{State: answerSaid, Entry: v.Answer}
	case v.State.Quiet:
		return &answerOut{State: answerQuiet}
	}
	return &answerOut{State: answerWaits}
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
	o, err := s.office()
	if err != nil {
		return nil, err
	}
	queue := s.Work.Queue()
	out := viewOut{summaryOut: summaryOf(sum, v.Origin, queue, s.Requests.Party), Read: v.Read, Entries: []entryOut{}, Questions: questionsOf(v.Entries, s.Requests.Party), Waits: []int64{}}
	if out.Subs, err = s.subsOf(o, v.Subs, queue, person); err != nil {
		return nil, err
	}
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
		out.Entries = append(out.Entries, entryOf(e, carried, s.Requests.Party))
	}
	if out.Decisions, err = s.decisionsOf(o, n, person, v.Whole); err != nil {
		return nil, err
	}
	if !v.Whole {
		return out, nil
	}
	held := v.Read && slices.ContainsFunc(out.Decisions, func(d decisionOut) bool {
		return d.Kind == decisions.OfDisclosure && d.What == decisions.SaysAnswer && d.Ended == ""
	})
	out.Answer = answerOf(v, held, carried)
	if v.Read {
		if out.Disclosure, err = s.disclosureOf(o, n, v.Team); err != nil {
			return nil, err
		}
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

// subsOf gives sub-requests as the request above them shows them, each
// with its decisions as the one looking may see them.
func (s *site) subsOf(o *office.Office, subs []requests.Sub, queue []int64, person string) ([]subOut, error) {
	out := []subOut{}
	for _, sub := range subs {
		ds, err := s.decisionsOf(o, sub.Number, person, true)
		if err != nil {
			return nil, err
		}
		below, err := s.subsOf(o, sub.Subs, queue, person)
		if err != nil {
			return nil, err
		}
		out = append(out, subOut{Number: sub.Number, Team: sub.Team, Question: sub.Question, State: sub.State.Is,
			Queued: queuedAt(queue, requests.Summary{Number: sub.Number, State: sub.State}), Label: labelOf(sub.State),
			On: partiesOf(sub.State.On), Answer: sub.Answer, Decisions: ds, Subs: below})
	}
	return out, nil
}

// questionsOf gives the questions put to people among the entries of a
// request, each with whether it is answered.
func questionsOf(es []requests.Entry, party func(requests.Who) requests.Party) []questionOut {
	out := []questionOut{}
	for _, e := range es {
		if e.Kind != requests.KindQuestion {
			continue
		}
		to := []partyOut{}
		for _, w := range e.To {
			to = append(to, partyOf(party(w)))
		}
		answered := slices.ContainsFunc(es, func(a requests.Entry) bool { return a.Answers == e.ID })
		out = append(out, questionOut{Entry: e.ID, By: partyOf(party(e.By)), To: to, At: e.At, Answered: answered})
	}
	return out
}

// disclosureOf gives whether a request's team sets a disclosure, who
// decides it, and whether its answer needs it, by the test work makes:
// whether anyone who reads it is outside the team.
func (s *site) disclosureOf(o *office.Office, n int64, team string) (*disclosureOut, error) {
	t := o.Teams[team]
	if t == nil {
		return nil, nil
	}
	if !t.HasDisclosure {
		return &disclosureOut{}, nil
	}
	r, err := s.Requests.Get(n)
	if err != nil {
		return nil, err
	}
	needed, err := s.Requests.Leaves(n, r.Asker)
	if err != nil {
		return nil, err
	}
	return &disclosureOut{Set: true, Needed: needed, Deciders: deciders(o, decisions.Disclosure(o, team))}, nil
}

// runOf gives a run on a request as it is kept with it.
func runOf(r work.KeptRun) runOut {
	out := runOut{Name: r.Name, Kind: r.Kind, Team: r.Team, Request: r.Request, Worker: r.Worker, Started: r.Started, Status: r.Status, How: r.How, Tokens: r.Tokens}
	if r.Kind == work.KindHeed && r.Status == "done" {
		out.Says, out.How, _ = strings.Cut(r.How, ": ")
	}
	if !r.Ended.IsZero() {
		out.Ended = &r.Ended
	}
	return out
}

// givenOf gives what a lead's run was given, each entry by and put to
// whoever the office names now, and each of ds its entries are of by its
// kind.
func givenOf(g *work.Given, party func(requests.Who) requests.Party, ds []*decisions.Decision) *givenOut {
	out := &givenOut{Notes: []noteGivenOut{}, Docs: []docReadOut{}, RecordGone: g.RecordGone, MayAsk: orEmpty(g.MayAsk),
		Thread: []entryOut{}, NotShown: g.NotShown, New: []entryOut{}, Decisions: []kindOut{}}
	for _, n := range g.Notes {
		out.Notes = append(out.Notes, noteGivenOut{Name: n.Name, Size: n.Size, Given: n.Given})
	}
	for _, d := range g.Docs {
		out.Docs = append(out.Docs, docReadOut{Name: d.Name, Team: d.Team})
	}
	for _, e := range g.Thread {
		out.Thread = append(out.Thread, entryOf(e, nil, party))
	}
	for _, e := range g.New {
		out.New = append(out.New, entryOf(e, nil, party))
	}
	entries := append(slices.Clone(g.Thread), g.New...)
	for _, d := range ds {
		if slices.ContainsFunc(entries, func(e requests.Entry) bool { return e.Decision == d.ID }) {
			out.Decisions = append(out.Decisions, kindOut{ID: d.ID, Kind: d.Kind})
		}
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

// entryOf gives an entry, with who it is by and to as party gives them,
// and where it was carried.
func entryOf(e requests.Entry, carried []bridges.Carried, party func(requests.Who) requests.Party) entryOut {
	to := []partyOut{}
	for _, w := range e.To {
		to = append(to, partyOf(party(w)))
	}
	out := entryOut{ID: e.ID, Kind: e.Kind, By: partyOf(party(e.By)), To: to, At: e.At, Text: e.Text, Files: []fileOut{}, In: e.In, Read: e.Read,
		Answers: e.Answers, Sub: e.Sub, Decision: e.Decision, Noted: e.Noted}
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
