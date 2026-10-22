package screens

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/store"
)

// A party is who asked, or whom a fork went to.
type party struct {
	Kind string `json:"kind"` // person, guest, team or routine; for a decision, rule or worker
	ID   string `json:"id"`
	Name string `json:"name"`
}

// A node is one stage of a request's line, from SOURCE to REPLY.
type node struct {
	Stage    string    `json:"stage"` // source, heed, work, check or reply
	State    string    `json:"state"` // waiting-to-start, passed, working, waiting, no, failed, stopped or skipped
	Line     string    `json:"line,omitzero"`
	Deciders []decider `json:"deciders,omitzero"`
}

// A decider is one decider of a heed or a check, and what it said.
type decider struct {
	Kind string  `json:"kind"` // rule, worker or person
	Name string  `json:"name"`
	Says *string `json:"says"` // yes, no, pass or failed; null while it is asked
	Note string  `json:"note,omitzero"`
}

// A row is a request in the list.
type row struct {
	ID       string    `json:"id"`
	Team     string    `json:"team"`
	TeamName string    `json:"teamName"`
	By       party     `json:"by"`
	Said     string    `json:"said"`
	At       time.Time `json:"at"`
	Status   string    `json:"status"`
	Nodes    []node    `json:"nodes"`
}

// A card is a question or a decision put to the one looking: for a
// decision, what each decider before the people said, and decide's held
// jobs.
type card struct {
	Ref      string         `json:"ref"`
	Request  string         `json:"request"`
	Team     string         `json:"team"`
	TeamName string         `json:"teamName"`
	Kind     string         `json:"kind"` // question or decision
	Said     string         `json:"said"`
	Choices  []string       `json:"choices"`
	Notes    []decider      `json:"notes,omitzero"`
	Jobs     []requests.Job `json:"jobs,omitzero"`
	At       time.Time      `json:"at"`
}

// A view is one request as the one looking may see it.
type view struct {
	ID       string    `json:"id"`
	Team     string    `json:"team"`
	TeamName string    `json:"teamName"`
	Status   string    `json:"status"`
	At       time.Time `json:"at"`
	Ended    time.Time `json:"ended,omitzero"`
	Source   source    `json:"source"`
	Nodes    []node    `json:"nodes"`
	Forks    []fork    `json:"forks"`
	Answer   *string   `json:"answer"` // what left the team
	Read     read      `json:"read"`
	Thread   []earlier `json:"thread"`
	CanSay   bool      `json:"canSay"`
	CanStop  bool      `json:"canStop"`
}

// A source is what was said where a request came from; SOURCE's line
// says who said it, and where.
type source struct {
	Said    string     `json:"said"`
	Files   []fileLink `json:"files"`
	Request string     `json:"request,omitzero"` // the request that asked a sub-request, when the one looking may see it
}

// A fileLink is a file said with a request.
type fileLink struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	Href string `json:"href"`
}

// A fork is a question of a request, or a decision on its held jobs,
// drawn as a line off WORK.
type fork struct {
	Kind     string         `json:"kind"`   // team, person or decision
	Party    party          `json:"party"`  // whom it went to; for a decision, who decided
	Status   string         `json:"status"` // waiting, working, answered, no (a decision that said no), refused, timed-out, failed or stopped
	At       time.Time      `json:"at"`
	Ended    time.Time      `json:"ended,omitzero"`
	Question string         `json:"question,omitzero"`
	Answer   string         `json:"answer,omitzero"`
	Jobs     []requests.Job `json:"jobs,omitzero"`     // a decision's held jobs
	Deciders []decider      `json:"deciders,omitzero"` // what each decider of a decision said
	Request  string         `json:"request,omitzero"`
	Card     *card          `json:"card,omitzero"`
}

// read is what the team read: sealed to everyone but its people.
type read struct {
	Sealed bool   `json:"sealed"`
	Calls  []call `json:"calls,omitzero"`
	Turns  []turn `json:"turns,omitzero"`
	Error  string `json:"error,omitzero"`
}

// A call is a tool the lead called, and what it gave back or why it
// failed; for a colleague that is an agent, the tools it called.
type call struct {
	Tool   string    `json:"tool"`
	Args   any       `json:"args"`
	Result any       `json:"result"`
	Error  string    `json:"error,omitzero"`
	At     time.Time `json:"at"`
	Calls  []call    `json:"calls,omitzero"`
}

func callsOf(calls []requests.Call) []call {
	var out []call
	for _, c := range calls {
		out = append(out, call{Tool: c.Tool, Args: c.Args, Result: c.Result, Error: c.Error, At: c.At, Calls: callsOf(c.Calls)})
	}
	return out
}

type turn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// earlier is a request of a request's thread.
type earlier struct {
	ID     string    `json:"id"`
	Said   string    `json:"said"`
	Answer *string   `json:"answer"`
	At     time.Time `json:"at"`
	Status string    `json:"status"`
}

// run gives the full name of a request's run from the id the screens
// know it by, which leaves out the client's name.
func (s *Screens) run(id string) (string, error) {
	run := s.corbie.ID.Name + "/" + id
	if store.CheckInstance(run) != nil {
		return "", noRequest
	}
	return run, nil
}

// id gives the id the screens know a run by.
func (s *Screens) id(run string) string {
	return strings.TrimPrefix(run, s.corbie.ID.Name+"/")
}

// asked gives when a request was asked: the time in its run's name.
func asked(run string) time.Time {
	at, _ := store.InstanceTime(run)
	return at
}

// list gives the questions and decisions waiting on the one looking,
// oldest first, and the requests they may see, newest first.
func (s *Screens) list(r *http.Request, me string) (any, error) {
	ctx := r.Context()
	o := s.engine.Reading().Office
	waits, err := s.engine.Waiting(ctx, me)
	if err != nil {
		return nil, err
	}
	cards, waitsOn := []card{}, map[string]bool{}
	for _, w := range waits {
		cards = append(cards, *s.cardOf(o, w))
		waitsOn[w.Request] = true
	}
	seen, err := s.engine.Newest(ctx, func(q *requests.Request) bool { return sightOf(o, me, q, waitsOn[q.ID]) != unseen }, 200)
	if err != nil {
		return nil, err
	}
	carried, err := s.carried(ctx)
	if err != nil {
		return nil, err
	}
	rows := []row{}
	for _, q := range seen {
		nodes := nodesOf(q, carried[q.ID])
		rows = append(rows, row{ID: s.id(q.ID), Team: q.Team, TeamName: teamName(o, q.Team), By: partyOf(o, q),
			Said: strings.SplitN(q.Said, "\n", 2)[0], At: asked(q.ID), Status: q.Status, Nodes: nodes})
	}
	return map[string]any{"waiting": cards, "requests": rows}, nil
}

// carried gives the statuses of the runs that carried each request's
// answer out, by the request.
func (s *Screens) carried(ctx context.Context) (map[string][]string, error) {
	runs, err := s.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": s.engine.Reading().Office.ID, "kind": "out", "request": ""}, Finished: true})
	carried := map[string][]string{}
	for _, run := range runs {
		carried[run.Labels["request"]] = append(carried[run.Labels["request"]], run.Status)
	}
	return carried, err
}

// cardOf gives a wait as a card for the one it waits on.
func (s *Screens) cardOf(o *office.Office, w *requests.Wait) *card {
	kind := "question"
	if w.Decision != "" {
		kind = "decision"
	}
	return &card{Ref: w.Ref, Request: s.id(w.Request), Team: w.Team, TeamName: teamName(o, w.Team), Kind: kind,
		Said: asks(w), Choices: w.Choices, Notes: decidersOf(o, w.Notes), Jobs: w.Jobs, At: w.Started}
}

// asks gives what a wait asks, as a card shows it. A decision's said
// holds its notes, and decide's held jobs, as text for a bridge, and the
// card shows them on their own.
func asks(w *requests.Wait) string {
	if w.Decision == "decide" {
		first, _, _ := strings.Cut(w.Said, "\n")
		return first
	}
	if len(w.Notes) == 0 {
		return w.Said
	}
	lines := make([]string, len(w.Notes))
	for i, n := range w.Notes {
		lines[i] = n.Name + " passed."
		if n.Note != "" {
			lines[i] = n.Name + " passed: " + n.Note
		}
	}
	return strings.TrimSuffix(w.Said, "\n\n"+strings.Join(lines, "\n"))
}

// putTo says whether a question or a decision is put to a person: it
// waits on them, or they answered it. One Bulldog refused, since its team
// may not ask a person it names, is put to nobody, and the engine gives
// none that waits until it is refused.
func putTo(w *requests.Wait, person string) bool {
	return w.By == person || w.Status == "waiting" && slices.Contains(w.People, person)
}

// sight says how a person sees a request, looking at the waits on them
// only when nothing else lets them see it.
func (s *Screens) sight(ctx context.Context, me string, q *requests.Request) (sight, error) {
	o := s.engine.Reading().Office
	if how := sightOf(o, me, q, false); how != unseen {
		return how, nil
	}
	waits, err := s.engine.Waiting(ctx, me)
	return sightOf(o, me, q, slices.ContainsFunc(waits, func(w *requests.Wait) bool { return w.Request == q.ID })), err
}

// request gives one request as the one looking may see it.
func (s *Screens) request(r *http.Request, me string) (any, error) {
	ctx := r.Context()
	id, err := s.run(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	look, err := s.engine.Look(ctx, id)
	if err != nil {
		return nil, err
	}
	o := s.engine.Reading().Office
	q := look.Request
	waitsOn := slices.ContainsFunc(look.Forks, func(w *requests.Wait) bool { return w.Status == "waiting" && slices.Contains(w.People, me) })
	how := sightOf(o, me, q, waitsOn)
	if how == unseen {
		return nil, noRequest
	}

	v := view{ID: s.id(q.ID), Team: q.Team, TeamName: teamName(o, q.Team), Status: q.Status, At: asked(q.ID), Ended: q.Ended,
		Answer: q.Answer, Forks: []fork{}, Read: read{Sealed: how != whole}, Thread: []earlier{}}
	v.Source = source{Said: q.Said, Files: []fileLink{}}
	for i, f := range q.Files {
		v.Source.Files = append(v.Source.Files, fileLink{Name: f.Name, Type: f.Type, Size: f.Size, Href: fmt.Sprintf("/api/requests/%s/files/%d", v.ID, i)})
	}
	if q.Wait != "" {
		if v.Source.Request, err = s.asking(ctx, o, me, q.Wait); err != nil {
			return nil, err
		}
	}

	var carried []string
	failed := "" // why the first run that failed to carry the answer failed
	for _, reply := range look.Replies {
		carried = append(carried, reply.Status)
		if reply.Status == "failed" && failed == "" {
			failed = reply.Error
		}
	}
	v.Nodes = nodesOf(q, carried)
	v.Nodes[0].Line = sourceLine(o, q)
	v.Nodes[4].Line = replyLine(o, q, carried)
	// A stage running when the request was stopped says so to everyone;
	// to the team's people, WORK's line gives the lead and the time too.
	for i, n := range v.Nodes {
		if n.State == "stopped" {
			v.Nodes[i].Line = "stopped"
		}
	}
	if how == whole {
		if v.Nodes[4].State == "failed" && failed != "" {
			v.Nodes[4].Line += ": " + failed
		}
		lines(o, q, look.Forks, v.Nodes)
		v.Read.Calls = callsOf(look.Calls)
		for _, t := range look.Turns {
			v.Read.Turns = append(v.Read.Turns, turn{Role: t.Role, Text: t.Text})
		}
		for _, st := range q.Stages {
			if st.Error != "" && v.Read.Error == "" {
				v.Read.Error = st.Name + " failed: " + st.Error
			}
		}
	}

	for _, w := range look.Forks {
		if w.Decision != "" {
			continue // a heed's or a check's is shown at its node, and as a card to whom it is put; decide's with its decision
		}
		v.Forks = append(v.Forks, s.forkOf(o, me, q, w, how))
	}
	for _, d := range look.Decisions {
		if d.Wait != nil || how == whole {
			v.Forks = append(v.Forks, s.decisionFork(o, me, d, how))
		}
	}
	slices.SortStableFunc(v.Forks, func(a, b fork) int { return a.At.Compare(b.At) })
	for _, t := range look.Thread {
		if t.ID == q.ID || sightOf(o, me, t, false) != unseen {
			v.Thread = append(v.Thread, earlier{ID: s.id(t.ID), Said: t.Said, Answer: t.Answer, At: asked(t.ID), Status: t.Status})
		}
	}
	v.CanSay = mayAct(o, me, q)
	v.CanStop = v.CanSay && (q.Status == "working" || q.Status == "waiting")
	return v, nil
}

// asking gives the request that asked a sub-request, by the wait that
// asked it, when the one looking may see it.
func (s *Screens) asking(ctx context.Context, o *office.Office, me, wait string) (string, error) {
	top := wait
	for {
		run, err := s.corbie.Show(ctx, top)
		if err != nil {
			return "", err
		}
		if run.Parent == "" {
			break
		}
		top = run.Parent
	}
	q, err := s.engine.Request(ctx, top)
	if err != nil || sightOf(o, me, q, false) == unseen {
		return "", err
	}
	return s.id(top), nil
}

// forkOf gives a question of a request as a fork, as the one looking may
// see it. Its question and answer are shown to the team's people and to
// those it is put to; its card, to those it waits on.
func (s *Screens) forkOf(o *office.Office, me string, q *requests.Request, w *requests.Wait, how sight) fork {
	f := fork{Kind: "person", Party: waitedOn(o, w), Status: w.Status, At: w.Started, Ended: w.Ended}
	if w.ToTeam != "" {
		f.Kind = "team"
		if w.Asked != "" && (of(o, q.Team, me) || of(o, w.ToTeam, me)) {
			f.Request = s.id(w.Asked)
		}
	}
	put := putTo(w, me)
	if how == whole || put {
		f.Question, f.Answer = w.Said, answerOf(w)
	}
	if put && w.Status == "waiting" {
		f.Card = s.cardOf(o, w)
	}
	return f
}

// decisionFork gives a decision on a request's held jobs as a fork. Its
// jobs, its answer and what each decider said are shown to the team's
// people and to those it is put to; its card, to those it waits on. To
// anyone else, it shows who, when and how it ended.
func (s *Screens) decisionFork(o *office.Office, me string, d *requests.Decision, how sight) fork {
	w := d.Wait
	f := fork{Kind: "decision", Status: d.Status, At: d.Started, Ended: d.Ended}
	switch {
	case w != nil:
		f.Party = waitedOn(o, w)
	case len(d.Words) > 0:
		last := d.Words[len(d.Words)-1]
		f.Party = party{Kind: last.Kind, ID: last.Name, Name: deciderName(o, last.Kind, last.Name)}
	}
	switch {
	case d.Says == "no":
		f.Status = "no"
	case d.Status == "waiting" && (w == nil || w.Status != "waiting"):
		f.Status = "working" // its rules and workers decide
	}
	put := w != nil && putTo(w, me)
	if how == whole || put {
		f.Jobs, f.Answer, f.Deciders = jobsOf(d), d.Says, decidersOf(o, d.Words)
		if w != nil && w.By != "" && d.Says != "" {
			f.Answer += " (" + personName(o, w.By) + ")"
		}
	}
	if put && w.Status == "waiting" {
		f.Card = s.cardOf(o, w)
	}
	return f
}

// jobsOf gives a decision's held jobs: as its people were given them, or
// by name alone when it came to no person.
func jobsOf(d *requests.Decision) []requests.Job {
	if d.Wait != nil {
		return d.Wait.Jobs
	}
	jobs := make([]requests.Job, len(d.Jobs))
	for i, name := range d.Jobs {
		jobs[i] = requests.Job{Name: name}
	}
	return jobs
}

// waitedOn gives whom a wait waits on: the team, the person who answered,
// or the people it asks.
func waitedOn(o *office.Office, w *requests.Wait) party {
	switch {
	case w.ToTeam != "":
		return party{Kind: "team", ID: w.ToTeam, Name: teamName(o, w.ToTeam)}
	case w.By != "":
		return party{Kind: "person", ID: w.By, Name: personName(o, w.By)}
	case len(w.People) == 0:
		return party{Kind: "person"}
	}
	names := make([]string, len(w.People))
	for i, p := range w.People {
		names[i] = personName(o, p)
	}
	return party{Kind: "person", ID: w.People[0], Name: strings.Join(names, " or ")}
}

// answerOf says how a question was answered.
func answerOf(w *requests.Wait) string {
	switch {
	case w.Status == "refused":
		return w.Refused
	case w.Status == "timed-out":
		return "No answer came within 7 days."
	case w.Answer == nil:
		return ""
	}
	return *w.Answer
}

// nodesOf gives a request's line, each node's state.
func nodesOf(q *requests.Request, carried []string) []node {
	nodes := []node{{Stage: "source", State: "passed"}}
	for _, stage := range []string{"heed", "work", "check"} {
		n := node{Stage: stage, State: "skipped"}
		for _, st := range q.Stages {
			if st.Name == stage {
				n.State = or(st.State, "waiting-to-start")
			}
		}
		nodes = append(nodes, n)
	}
	return append(nodes, node{Stage: "reply", State: replyState(q, carried)})
}

// ended says whether a request has ended, by its status.
func ended(q *requests.Request) bool {
	return q.Status != "working" && q.Status != "waiting"
}

// replyState says whether a request's answer left: sent, nothing to send,
// or on its way. The chat's and a sub-request's answer leave when the
// request ends; another's when the runs that carry it have.
func replyState(q *requests.Request, carried []string) string {
	switch {
	case !ended(q) || q.Status == "stopped":
		return "waiting-to-start"
	case q.Answer == nil || *q.Answer == "":
		return "no"
	case q.Heard == "chat" || q.Heard == "team" || q.Heard == "routine" && len(carried) == 0:
		return "passed"
	case slices.Contains(carried, "failed"):
		return "failed"
	case len(carried) > 0 && !slices.ContainsFunc(carried, func(s string) bool { return s != "done" }):
		return "passed"
	}
	return "working"
}

// replyLine says where a request's answer went.
func replyLine(o *office.Office, q *requests.Request, carried []string) string {
	switch state := replyState(q, carried); {
	case state == "waiting-to-start":
		return ""
	case state == "no":
		return "nothing to send"
	case state == "failed":
		return "not sent"
	case state == "working":
		return "sending"
	case q.Heard == "team":
		return "given to " + teamName(o, q.By.Team)
	case q.Heard == "chat", len(carried) == 0:
		return "shown on the screens"
	}
	return "sent"
}

// sourceLine says who asked a request, and where.
func sourceLine(o *office.Office, q *requests.Request) string {
	who := partyOf(o, q).Name
	switch q.Heard {
	case "channel", "direct":
		if q.By.Guest != "" {
			who = "a guest"
		}
		if q.Place != nil {
			return who + " · " + q.Place.Bridge + " › " + q.Place.Place
		}
	case "chat":
		return who + " · on the screens"
	case "team":
		return who + " asked"
	case "routine":
		return "routine " + q.By.Routine
	}
	return who
}

// lines writes, for the team's people, the line under each stage's node:
// the decider that answered and its word, and the lead and how long it
// took; and what each decider of a heed or a check said.
func lines(o *office.Office, q *requests.Request, waits []*requests.Wait, nodes []node) {
	for _, st := range q.Stages {
		n := &nodes[slices.IndexFunc(nodes, func(n node) bool { return n.Stage == st.Name })]
		if st.Name == "work" {
			n.Line = workLine(o, q, st, waits)
			continue
		}
		var asked []string
		n.Deciders = decidersOf(o, st.Words)
		for _, w := range waits {
			if w.Decision == st.Name && w.Status == "waiting" {
				for _, p := range w.People {
					asked = append(asked, personName(o, p))
					n.Deciders = append(n.Deciders, decider{Kind: "person", Name: personName(o, p)})
				}
			}
		}
		switch st.State {
		case "waiting":
			n.Line = "waiting on " + strings.Join(asked, " or ")
		case "passed", "no", "failed":
			n.Line = "every decider passed · no"
			for _, w := range st.Words {
				if w.Says == "yes" || w.Says == "no" || w.Says == "failed" {
					n.Line = short(o, w.Kind, w.Name) + " · " + w.Says
				}
			}
		}
	}
}

// workLine gives the lead of a request's team, and how its work stands.
func workLine(o *office.Office, q *requests.Request, st requests.Stage, waits []*requests.Wait) string {
	lead := q.Team
	if t := o.Team(q.Team); t != nil {
		lead = or(t.Lead.Worker, personName(o, t.Lead.Person))
	}
	switch st.State {
	case "passed":
		return lead + " · " + took(st.Ended.Sub(st.Started))
	case "failed":
		return lead + " · failed after " + took(st.Ended.Sub(st.Started))
	case "stopped":
		return lead + " · stopped after " + took(q.Ended.Sub(st.Started))
	case "working":
		return lead + " · working"
	case "waiting":
		var on []string
		for _, w := range waits {
			who := waitedOn(o, w).Name
			if w.Decision != "" {
				who = "a decision"
			}
			if w.Status == "waiting" && !slices.Contains(on, who) {
				on = append(on, who)
			}
		}
		return lead + " · waiting on " + strings.Join(on, ", ")
	}
	return ""
}

// took says how long something took: 40s, 2m 05s, 3h 10m.
func took(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %02dm", s/3600, s/60%60)
}

// partyOf gives who asked a request.
func partyOf(o *office.Office, q *requests.Request) party {
	switch by := q.By; {
	case by.Person != "":
		return party{Kind: "person", ID: by.Person, Name: personName(o, by.Person)}
	case by.Guest != "":
		bridge := ""
		if q.Place != nil {
			bridge = q.Place.Bridge
		}
		return party{Kind: "guest", ID: bridge + ":" + by.Guest, Name: strings.TrimSpace(bridge + " " + by.Guest)}
	case by.Team != "":
		return party{Kind: "team", ID: by.Team, Name: teamName(o, by.Team)}
	}
	return party{Kind: "routine", ID: q.By.Routine, Name: q.By.Routine}
}

// decidersOf gives what each decider said.
func decidersOf(o *office.Office, words []requests.Word) []decider {
	var out []decider
	for _, w := range words {
		says := w.Says
		out = append(out, decider{Kind: w.Kind, Name: deciderName(o, w.Kind, w.Name), Says: &says, Note: w.Note})
	}
	return out
}

// deciderName gives a decider by the name the screens show it by: a
// rule's path, a worker's name, a person's name.
func deciderName(o *office.Office, kind, name string) string {
	if kind == "person" {
		return personName(o, name)
	}
	return name
}

// short gives a decider's name for a line: a rule's file's name without
// its ending.
func short(o *office.Office, kind, name string) string {
	if kind == "rule" {
		base := path.Base(name)
		return strings.TrimSuffix(base, path.Ext(base))
	}
	return deciderName(o, kind, name)
}

// teamName gives a team's name, or its ID when the office has it no more.
func teamName(o *office.Office, id string) string {
	if t := o.Team(id); t != nil {
		return t.Name
	}
	return id
}

// personName gives a person's name, or their ID when the office has them
// no more.
func personName(o *office.Office, id string) string {
	if p := o.Person(id); p != nil {
		return p.Name
	}
	return id
}

// ask asks a team in the chat, with files.
func (s *Screens) ask(r *http.Request, me string) (any, error) {
	var body struct {
		Team  string `json:"team"`
		Said  string `json:"said"`
		Files []struct {
			Name string `json:"name"`
			Data []byte `json:"data"`
		} `json:"files"`
	}
	if err := readJSON(r, 64<<20, &body); err != nil {
		return nil, err
	}
	t, err := s.takes(body.Team)
	switch {
	case err != nil:
		return nil, err
	case strings.TrimSpace(body.Said) == "":
		return nil, refusal{http.StatusBadRequest, "Say something to the team."}
	}
	a := requests.Ask{Team: t.ID, By: requests.By{Person: me}, Said: strings.TrimSpace(body.Said)}
	for _, f := range body.Files {
		a.Files = append(a.Files, requests.File{Name: f.Name, Data: client.InputFile{Name: f.Name, Contents: bytes.NewReader(f.Data)}})
	}
	id, err := s.engine.Ask(r.Context(), a)
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": s.id(id)}, nil
}

// takes gives a team of the office that takes requests.
func (s *Screens) takes(team string) (*office.Team, error) {
	rd := s.engine.Reading()
	t := rd.Office.Team(team)
	switch {
	case t == nil:
		return nil, noTeam
	case !rd.Takes(t.ID):
		return nil, refusal{http.StatusConflict, t.Name + " takes no requests while its files have problems. Its page shows them."}
	}
	return t, nil
}

// acted gives a request the one looking may say more in, or stop: one
// they asked, or one of their team's.
func (s *Screens) acted(r *http.Request, me string) (*requests.Request, error) {
	id, err := s.run(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	q, err := s.engine.Request(r.Context(), id)
	if err != nil {
		return nil, err
	}
	o := s.engine.Reading().Office
	switch {
	case sightOf(o, me, q, false) == unseen:
		return nil, noRequest
	case !mayAct(o, me, q):
		return nil, refusal{http.StatusForbidden, "Only the person who asked, and the team's people, may do that."}
	}
	return q, nil
}

// say says more in a request's thread: a new request of the same team.
func (s *Screens) say(r *http.Request, me string) (any, error) {
	var body struct {
		Said string `json:"said"`
	}
	if err := readJSON(r, 1<<20, &body); err != nil {
		return nil, err
	}
	q, err := s.acted(r, me)
	if err != nil {
		return nil, err
	}
	if _, err := s.takes(q.Team); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Said) == "" {
		return nil, refusal{http.StatusBadRequest, "Say something to the team."}
	}
	id, err := s.engine.Ask(r.Context(), requests.Ask{By: requests.By{Person: me}, Said: strings.TrimSpace(body.Said), After: q.ID})
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": s.id(id)}, nil
}

// stop stops a request and everything below it.
func (s *Screens) stop(r *http.Request, me string) (any, error) {
	q, err := s.acted(r, me)
	if err != nil {
		return nil, err
	}
	if ended(q) {
		return nil, refusal{http.StatusConflict, "This request has ended."}
	}
	return struct{}{}, s.engine.Stop(r.Context(), q.ID)
}

// answer answers a question or a decision waiting on the one looking.
func (s *Screens) answer(r *http.Request, me string) (any, error) {
	var body struct {
		Answer string `json:"answer"`
	}
	if err := readJSON(r, 1<<20, &body); err != nil {
		return nil, err
	}
	w, err := s.engine.WaitOf(r.Context(), r.PathValue("ref"))
	switch {
	case err != nil:
		return nil, err
	case w == nil || !slices.Contains(w.People, me):
		return nil, noQuestion
	case strings.TrimSpace(body.Answer) == "":
		return nil, refusal{http.StatusBadRequest, "Give an answer."}
	}
	err = s.engine.Answer(r.Context(), w.ID, me, strings.TrimSpace(body.Answer), "")
	if errors.Is(err, requests.ErrNotAChoice) {
		return nil, refusal{http.StatusBadRequest, "Answer with one of the choices: " + strings.Join(w.Choices, " or ") + "."}
	}
	return struct{}{}, err
}

// file serves a file said with a request to whoever may see the request,
// so that it can do nothing: its type is told from what it holds, never
// from its name; only a picture other than SVG, or a PDF, is shown, and
// anything else is given to be saved; and nothing in it may run.
func (s *Screens) file(w http.ResponseWriter, r *http.Request, me string) {
	ctx := r.Context()
	id, err := s.run(r.PathValue("id"))
	n, nerr := strconv.Atoi(r.PathValue("n"))
	if err != nil || nerr != nil {
		s.refuse(w, noFile)
		return
	}
	q, err := s.engine.Request(ctx, id)
	if errors.Is(err, requests.ErrNoRequest) {
		err = noFile
	}
	if err != nil {
		s.refuse(w, err)
		return
	}
	how, err := s.sight(ctx, me, q)
	if err == nil && how == unseen {
		err = noFile
	}
	if err != nil {
		s.refuse(w, err)
		return
	}
	contents, f, err := s.engine.File(ctx, id, n)
	if errors.Is(err, client.ErrNoFile) {
		err = noFile
	}
	if err != nil {
		s.refuse(w, err)
		return
	}
	head := make([]byte, 512)
	k, err := io.ReadFull(contents, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		s.refuse(w, err)
		return
	}
	kind, disposition := http.DetectContentType(head[:k]), "inline"
	if !(strings.HasPrefix(kind, "image/") && kind != "image/svg+xml" || kind == "application/pdf") {
		kind, disposition = "application/octet-stream", "attachment"
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(head[:k])
	io.Copy(w, contents)
}
