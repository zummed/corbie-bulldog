package requests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/instance"
)

// ErrNoRequest is a run that is no request of the office.
var ErrNoRequest = errors.New("there is no such request")

// A Request is a request as its run shows it.
type Request struct {
	ID     string // its run's name
	Team   string
	By     By
	Heard  string // channel, direct, chat, team or routine
	Place  *Place // where a bridge heard it; for a sub-request, where its chain began
	Said   string
	Files  []client.File
	Chain  []string // the teams whose requests asked this one, top first
	Thread string   // its thread's label, which every request of the thread carries
	Wait   string   // for a sub-request, the wait that asked it
	// Status is working, waiting (on a person or a team), answered,
	// noted, withheld, failed or stopped.
	Status string
	// Answer is what left the team: the request machine's answer, or for
	// a run that failed, the line of a failed request. Nil for none.
	Answer  *string
	Stages  []Stage // heed, work and check, those its machine has
	Started time.Time
	Ended   time.Time
}

// A Stage is one stage of a request's machine as it stands.
type Stage struct {
	Name string // heed, work or check
	// State is "" (not reached), working, waiting, passed, no (a heed or
	// a check that said no), failed or skipped.
	State string
	// What Look adds: when it started and ended, what each decider of a
	// heed or a check said, in order, and the error of a stage that
	// failed.
	Started time.Time
	Ended   time.Time
	Words   []Word
	Error   string
}

// A Word is what one decider said.
type Word struct {
	Kind string // rule, worker or person
	Name string // a rule's path from the office's top, a worker's or a person's name
	Says string // yes, no or pass; failed for a decider that failed
	Note string
}

// A Look is one request in full: what each stage did, the waits below it,
// what its lead read and called, where its answer was carried, and its
// thread.
type Look struct {
	*Request
	Forks   []*Wait    // every wait below it, oldest first
	Turns   []Turn     // the lead's conversation
	Calls   []Call     // the tools the lead called
	Replies []*Reply   // the runs that carried its answer out
	Thread  []*Request // every request of its thread, itself among them, oldest first
}

// A Turn is one part of the lead's conversation: what it was asked, or
// what the model said.
type Turn struct {
	Role string // user or model
	Text string
}

// A Call is one tool the lead called, and what it gave back.
type Call struct {
	Tool   string
	Args   any
	Result any
	At     time.Time
}

// A Reply is a run that carried a request's answer out.
type Reply struct {
	Run    string
	Bridge string
	Status string // waiting, running, done or failed
	Error  string
}

// requestOf reads a request from its run as shown, but its stages.
func (e *Engine) requestOf(run *client.Run) *Request {
	var in struct {
		Said    string `json:"said"`
		Request given  `json:"request"`
		Files   []struct {
			Name string `json:"name"`
			Data struct {
				Type string `json:"type"`
				Size int64  `json:"size"`
			} `json:"data"`
		} `json:"files"`
	}
	decode(run.Input, &in)
	var out struct {
		Outcome string  `json:"outcome"`
		Answer  *string `json:"answer"`
	}
	decode(run.Output, &out)
	g := in.Request
	q := &Request{ID: run.ID, Team: run.Labels["team"], By: g.By, Heard: g.Heard, Place: g.Place, Said: in.Said, Chain: g.Chain,
		Thread: run.Labels["thread"], Wait: run.Labels["wait"], Started: run.Started, Ended: run.Ended}
	for _, f := range in.Files {
		q.Files = append(q.Files, client.File{Name: f.Name, Type: f.Data.Type, Size: f.Data.Size})
	}
	switch run.Status {
	case "done":
		q.Status, q.Answer = out.Outcome, out.Answer
	case "cancelled":
		q.Status = "stopped"
	case "failed":
		name := q.Team
		if t := e.Reading().Office.Team(q.Team); t != nil {
			name = t.Name
		}
		failed := name + " could not answer: its work failed."
		q.Status, q.Answer = "failed", &failed
	default:
		q.Status = "working"
	}
	return q
}

// request shows a run that is a request of the office.
func (e *Engine) request(ctx context.Context, id string) (*client.Run, error) {
	run, err := e.corbie.Show(ctx, id)
	switch {
	case errors.Is(err, client.ErrNoRun):
		return nil, ErrNoRequest
	case err != nil:
		return nil, err
	case run.Parent != "" || run.Labels["kind"] != "request" || run.Labels["bulldog"] != e.Reading().Office.ID:
		return nil, ErrNoRequest
	}
	return run, nil
}

// shown gives a request as its run shows it, but its stages.
func (e *Engine) shown(ctx context.Context, id string) (*Request, error) {
	run, err := e.corbie.Show(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.requestOf(run), nil
}

// top gives the request a run is below, as shown.
func (e *Engine) top(ctx context.Context, id string) (*client.Run, error) {
	for {
		run, err := e.corbie.Show(ctx, id)
		if err != nil || run.Parent == "" {
			return run, err
		}
		id = run.Parent
	}
}

// staged gives a request with the state of each stage, from its latest
// step. A request that waits on a person or a team has the stage that runs
// waiting.
func (e *Engine) staged(ctx context.Context, run *client.Run, waiting bool) (*Request, error) {
	q := e.requestOf(run)
	if waiting && q.Status == "working" {
		q.Status = "waiting"
	}
	if run.Step == 0 {
		return q, nil // waiting to start
	}
	step, err := e.corbie.Step(ctx, run.ID, run.Step)
	if err != nil {
		return nil, err
	}
	var shown struct {
		Stages map[string]string `json:"stages"`
	}
	decode(step.Shown, &shown)
	for _, name := range []string{"heed", "work", "check"} {
		status, ok := shown.Stages[name]
		if !ok {
			continue
		}
		state := ""
		switch {
		case status == "running" && run.HasEnded():
		case status == "running" && waiting:
			state = "waiting"
		case status == "running":
			state = "working"
		case status == "skipped", status == "failed":
			state = status
		case status == "done" && (name == "heed" && q.Status == "noted" || name == "check" && q.Status == "withheld"):
			state = "no"
		case status == "done":
			state = "passed"
		}
		q.Stages = append(q.Stages, Stage{Name: name, State: state})
	}
	return q, nil
}

// below gives the runs of a listing by the request each is below, itself
// among them.
func below(runs []*client.Run) map[string][]*client.Run {
	parent := map[string]string{}
	for _, run := range runs {
		parent[run.ID] = run.Parent
	}
	tree := map[string][]*client.Run{}
	for _, run := range runs {
		top := run.ID
		for parent[top] != "" {
			top = parent[top]
		}
		tree[top] = append(tree[top], run)
	}
	return tree
}

// waiting says whether a wait among runs waits on its answer.
func waiting(runs []*client.Run) bool {
	return slices.ContainsFunc(runs, func(r *client.Run) bool { return r.Machine == "wait" && waits(r, "answer") })
}

// Requests gives every request of the office, newest first, each with the
// state of its stages.
func (e *Engine) Requests(ctx context.Context) ([]*Request, error) {
	runs, err := e.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": e.Reading().Office.ID, "kind": "request"}, Finished: true})
	if err != nil {
		return nil, err
	}
	tree := below(runs)
	var requests []*Request
	for _, run := range runs {
		if run.Parent != "" {
			continue
		}
		shown, err := e.corbie.Show(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		q, err := e.staged(ctx, shown, waiting(tree[run.ID]))
		if err != nil {
			return nil, err
		}
		requests = append(requests, q)
	}
	slices.SortStableFunc(requests, func(a, b *Request) int { return strings.Compare(b.ID, a.ID) })
	return requests, nil
}

// Request gives one request of the office, with the state of its stages;
// ErrNoRequest when there is none.
func (e *Engine) Request(ctx context.Context, id string) (*Request, error) {
	run, err := e.request(ctx, id)
	if err != nil {
		return nil, err
	}
	all, err := e.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": e.Reading().Office.ID, "thread": run.Labels["thread"]}})
	if err != nil {
		return nil, err
	}
	return e.staged(ctx, run, waiting(below(all)[id]))
}

// Stop stops a request: Corbie cancels its run and everything below it,
// and a request it asked of another team is cancelled when the wait that
// asked it ends.
func (e *Engine) Stop(ctx context.Context, id string) error {
	if _, err := e.request(ctx, id); err != nil {
		return err
	}
	_, err := e.corbie.Cancel(ctx, id, 0)
	return err
}

// File gives a file said with a request, by its place among them.
func (e *Engine) File(ctx context.Context, id string, n int) (io.Reader, client.File, error) {
	run, err := e.request(ctx, id)
	if err != nil {
		return nil, client.File{}, err
	}
	var in struct {
		Files []struct {
			Name string `json:"name"`
			Data struct {
				At string `json:"$blob"`
			} `json:"data"`
		} `json:"files"`
	}
	decode(run.Input, &in)
	if n < 0 || n >= len(in.Files) || in.Files[n].Data.At == "" {
		return nil, client.File{}, fmt.Errorf("%w %d", client.ErrNoFile, n)
	}
	reader, file, err := e.corbie.Fetch(ctx, id, in.Files[n].Data.At)
	file.Name = in.Files[n].Name
	return reader, file, err
}

// Look gives one request in full; ErrNoRequest when there is none.
func (e *Engine) Look(ctx context.Context, id string) (*Look, error) {
	run, err := e.request(ctx, id)
	if err != nil {
		return nil, err
	}
	all, err := e.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": e.Reading().Office.ID, "thread": run.Labels["thread"]}, Finished: true})
	if err != nil {
		return nil, err
	}
	tree := below(all)
	q, err := e.staged(ctx, run, waiting(tree[id]))
	if err != nil {
		return nil, err
	}
	look := &Look{Request: q}
	entries := map[string][]client.Entry{}
	if _, err := e.corbie.Record(ctx, id, func(entry client.Entry) { entries[entry.ID] = append(entries[entry.ID], entry) }); err != nil {
		return nil, err
	}
	children := childrenOf(entries)
	for i := range q.Stages {
		s := &q.Stages[i]
		for _, entry := range entries[id] {
			if slices.ContainsFunc(entry.Asks, func(a instance.Ask) bool { return a.Kind == "child" && a.Name == s.Name }) {
				s.Started = entry.Time
			}
			for _, event := range entry.Events {
				if event.From == s.Name && (event.Name == "done" || event.Name == "failed") {
					s.Ended = entry.Time
				}
				if event.From == s.Name && event.Name == "failed" {
					s.Error = errorOf(event.Data)
				}
			}
		}
		if s.Name != "work" {
			s.Words = words(entries[children[id][s.Name]])
		}
	}
	look.Turns, look.Calls = conversation(entries[children[id]["work"]])

	for _, r := range tree[id] {
		if r.Machine != "wait" {
			continue
		}
		shown, err := e.corbie.Show(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		w := e.waitOf(shown)
		w.Request = id
		look.Forks = append(look.Forks, w)
	}
	slices.SortStableFunc(look.Forks, func(a, b *Wait) int { return strings.Compare(a.ID, b.ID) })

	replies, err := e.roots(ctx, client.Filter{Labels: map[string]string{"kind": "out", "request": id}, Finished: true})
	if err != nil {
		return nil, err
	}
	for _, r := range replies {
		shown, err := e.corbie.Show(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		look.Replies = append(look.Replies, &Reply{Run: r.ID, Bridge: r.Labels["bridge"], Status: shown.Status, Error: shown.Error})
	}

	for top := range tree {
		if top == id {
			look.Thread = append(look.Thread, q)
			continue
		}
		other, err := e.corbie.Show(ctx, top)
		if err != nil {
			return nil, err
		}
		if other.Labels["kind"] == "request" && other.Parent == "" {
			look.Thread = append(look.Thread, e.requestOf(other))
		}
	}
	slices.SortStableFunc(look.Thread, func(a, b *Request) int { return strings.Compare(a.ID, b.ID) })
	return look, nil
}
