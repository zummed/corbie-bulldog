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

// A party is who asked, or whom a fork waits on.
type party struct {
	Kind string `json:"kind"` // person, guest, team or routine
	ID   string `json:"id"`
	Name string `json:"name"`
}

// A node is one stage of a request's line, from SOURCE to REPLY.
type node struct {
	Stage    string    `json:"stage"` // source, heed, work, check or reply
	State    string    `json:"state"` // waiting-to-start, passed, working, waiting, no, failed or skipped
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

// A card is a question or a decision put to the one looking.
type card struct {
	Ref      string    `json:"ref"`
	Request  string    `json:"request"`
	Team     string    `json:"team"`
	TeamName string    `json:"teamName"`
	Kind     string    `json:"kind"` // question or decision
	Said     string    `json:"said"`
	Choices  []string  `json:"choices"`
	Notes    []any     `json:"notes"`
	Jobs     []any     `json:"jobs"`
	At       time.Time `json:"at"`
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
	Answer   struct {
		Text *string `json:"text"`
		Sent bool    `json:"sent"`
	} `json:"answer"`
	Read    read      `json:"read"`
	Thread  []earlier `json:"thread"`
	CanSay  bool      `json:"canSay"`
	CanStop bool      `json:"canStop"`
}

// A source is where a request came from, and what was said there.
type source struct {
	Kind    string     `json:"kind"` // bridge, chat, team or routine
	Bridge  string     `json:"bridge,omitzero"`
	Place   string     `json:"place,omitzero"`
	By      party      `json:"by"`
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

// A fork is a wait of a request, drawn as a line off WORK.
type fork struct {
	Kind     string    `json:"kind"` // team, person or decision
	Party    party     `json:"party"`
	Status   string    `json:"status"`
	At       time.Time `json:"at"`
	Ended    time.Time `json:"ended,omitzero"`
	Question string    `json:"question,omitzero"`
	Answer   string    `json:"answer,omitzero"`
	Request  string    `json:"request,omitzero"`
	Card     *card     `json:"card,omitzero"`
}

// read is what the team read: sealed to everyone but its people.
type read struct {
	Sealed bool   `json:"sealed"`
	Calls  []call `json:"calls,omitzero"`
	Turns  []turn `json:"turns,omitzero"`
	Error  string `json:"error,omitzero"`
}

type call struct {
	Tool   string    `json:"tool"`
	Args   any       `json:"args"`
	Result any       `json:"result"`
	At     time.Time `json:"at"`
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
	all, err := s.engine.Requests(ctx)
	if err != nil {
		return nil, err
	}
	carried, err := s.carried(ctx)
	if err != nil {
		return nil, err
	}
	rows := []row{}
	for _, q := range all {
		if len(rows) == 200 {
			break
		}
		if sightOf(o, me, q, waitsOn[q.ID]) == unseen {
			continue
		}
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
		Said: w.Said, Choices: w.Choices, Notes: []any{}, Jobs: []any{}, At: w.Started}
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
		Forks: []fork{}, Read: read{Sealed: how != whole}, Thread: []earlier{}}
	v.Source = source{Kind: q.Heard, By: partyOf(o, q), Said: q.Said, Files: []fileLink{}}
	if q.Place != nil && (q.Heard == "channel" || q.Heard == "direct") {
		v.Source.Kind, v.Source.Bridge, v.Source.Place = "bridge", q.Place.Bridge, q.Place.Place
	}
	for i, f := range q.Files {
		v.Source.Files = append(v.Source.Files, fileLink{Name: f.Name, Type: f.Type, Size: f.Size, Href: fmt.Sprintf("/api/requests/%s/files/%d", v.ID, i)})
	}
	if q.Wait != "" {
		if v.Source.Request, err = s.asking(ctx, o, me, q.Wait); err != nil {
			return nil, err
		}
	}

	var carried []string
	for _, reply := range look.Replies {
		carried = append(carried, reply.Status)
	}
	v.Nodes = nodesOf(q, carried)
	v.Nodes[0].Line = sourceLine(o, q)
	v.Nodes[4].Line = replyLine(o, q, carried)
	v.Answer.Text, v.Answer.Sent = q.Answer, v.Nodes[4].State == "passed"
	if how == whole {
		lines(o, q, look.Forks, v.Nodes)
		for _, c := range look.Calls {
			v.Read.Calls = append(v.Read.Calls, call{Tool: c.Tool, Args: c.Args, Result: c.Result, At: c.At})
		}
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
		if w.Decision == "heed" || w.Decision == "check" {
			continue // a decision of a stage, shown at its node, and as a card to whom it is put
		}
		v.Forks = append(v.Forks, s.forkOf(o, me, q, w, how))
	}
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

// forkOf gives a wait of a request as a fork, as the one looking may see
// it. Its question and answer are shown to the team's people and to those
// it is put to; its card, to those it waits on.
func (s *Screens) forkOf(o *office.Office, me string, q *requests.Request, w *requests.Wait, how sight) fork {
	f := fork{Kind: "person", Party: waitedOn(o, w), Status: w.Status, At: w.Started, Ended: w.Ended}
	switch {
	case w.ToTeam != "":
		f.Kind = "team"
		if w.Asked != "" && (of(o, q.Team, me) || of(o, w.ToTeam, me)) {
			f.Request = s.id(w.Asked)
		}
	case w.Decision != "":
		f.Kind = "decision"
	}
	put := slices.Contains(w.People, me)
	if how == whole || put {
		f.Question, f.Answer = w.Said, answerOf(o, w)
	}
	if put && w.Status == "waiting" {
		f.Card = s.cardOf(o, w)
	}
	return f
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

// answerOf says how a wait was answered.
func answerOf(o *office.Office, w *requests.Wait) string {
	switch {
	case w.Status == "refused":
		return w.Refused
	case w.Status == "timed-out":
		return "No answer came within 7 days."
	case w.Answer == nil:
		return ""
	case w.Decision != "" && w.By != "":
		return *w.Answer + " (" + personName(o, w.By) + ")"
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
		for _, w := range st.Words {
			says := w.Says
			n.Deciders = append(n.Deciders, decider{Kind: w.Kind, Name: deciderName(o, w.Kind, w.Name), Says: &says, Note: w.Note})
		}
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
	rd := s.engine.Reading()
	t := rd.Office.Team(body.Team)
	switch {
	case t == nil:
		return nil, noTeam
	case rd.Machines[t.ID].Draft == nil:
		return nil, refusal{http.StatusConflict, t.Name + " takes no requests while its files have problems. Its page shows them."}
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
	waits, err := s.engine.Waiting(ctx, me)
	if err != nil {
		s.refuse(w, err)
		return
	}
	waitsOn := slices.ContainsFunc(waits, func(w *requests.Wait) bool { return w.Request == id })
	if sightOf(s.engine.Reading().Office, me, q, waitsOn) == unseen {
		s.refuse(w, noFile)
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
