package work

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/instance"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// A run is watched as it happens from Corbie's own record of it, read as
// it is written, with everything below it. It is what was read, so it is
// shown only to those who may see what the run's team read.

// ErrNoWatch is what someone is told of a run they may not watch, as of a
// run that is not there: the two are told alike.
var ErrNoWatch = errors.New("there is no such run to watch")

// What a watcher is shown of a run, by its kind.
const (
	// SeenWords is a piece of a model's reply, as it arrives.
	SeenWords = "words"
	// SeenCall is a tool, or a stage, being started, with its input.
	SeenCall = "call"
	// SeenResult is what a tool or a stage gave back.
	SeenResult = "result"
	// SeenLog is a line a job logged.
	SeenLog = "log"
	// SeenRefused is something a job tried that was refused: what was
	// tried, which layer refused it and the line that would fix it.
	SeenRefused = "refused"
	// SeenFailed is a tool or a stage that failed some other way.
	SeenFailed = "failed"
	// SeenEmit is anything else a job sent up.
	SeenEmit = "emit"
	// SeenEnded is the run itself ending, and how.
	SeenEnded = "ended"
)

// A Seen is one thing a run did, as a watcher is shown it.
type Seen struct {
	At time.Time
	// Run is the instance it happened in, the run watched or one below it,
	// and Machine the machine that instance runs.
	Run     string
	Machine string
	Kind    string
	// Name is the tool, stage or job it is of.
	Name string
	Text string
	// For a refusal: what was tried, the layer that refused it, and the
	// line that would fix it.
	Refused string
	Layer   string
	Fix     string
}

// MayWatch says whether a person may watch a run as it happens: a run
// Bulldog started for this office, as it noted it, whose team's reading
// the requests say they may see; never a bridge's, whose record holds what
// was said in every place it listens in, or a sign-in link. A run below
// one Bulldog started is watched with it.
func (w *Work) MayWatch(run, person string) (bool, error) {
	n, err := w.noted(run)
	if err != nil || n == nil || isBridge(n.Kind) || n.Status == unsent {
		return false, err
	}
	labels := map[string]string{requests.LabelTeam: n.Team}
	if n.Request != 0 {
		labels[requests.LabelRequest] = fmt.Sprint(n.Request)
	}
	return w.requests.MayWatch(labels, person)
}

// Follow shows a person who may watch a run each thing it does as it does
// it, with everything below it, from its beginning, and ends once the run
// has ended, saying how, or the context ends. To anyone else, and once
// Corbie keeps it no more, the run is not there.
func (w *Work) Follow(ctx context.Context, run, person string, tell func(Seen)) error {
	may, err := w.MayWatch(run, person)
	if err != nil {
		return err
	}
	if !may {
		return ErrNoWatch
	}
	machines := map[string]string{}
	machine := func(id string) string {
		if m, ok := machines[id]; ok {
			return m
		}
		r, err := w.corbie.Show(ctx, id)
		if err != nil {
			// Shown by its name alone, until its head can be read.
			return id
		}
		machines[id] = r.Machine
		return r.Machine
	}
	end, err := w.corbie.Follow(ctx, run, w.Interval, func(e client.Entry) {
		for _, s := range seenOf(e) {
			s.Machine = machine(e.ID)
			tell(s)
		}
	})
	switch {
	case errors.Is(err, client.ErrNoRun):
		// Corbie keeps it no more.
		return ErrNoWatch
	case err != nil:
		return fmt.Errorf("the run %s could not be followed in Corbie: %w", run, err)
	}
	if end != nil && end.HasEnded() {
		status, how := outcome(end)
		tell(Seen{At: end.Ended, Run: end.ID, Machine: end.Machine, Kind: SeenEnded, Text: cmp.Or(how, status),
			Refused: end.Refused, Layer: end.Layer, Fix: end.Fix})
	}
	return nil
}

// seenOf gives what an entry of a run's record shows a watcher: what a
// job sent up, the results of the tools and stages a step took, and the
// tools and stages it started. An agent's own turns are not tools.
func seenOf(e client.Entry) []Seen {
	at := Seen{At: e.Time, Run: e.ID}
	if e.Job != "" {
		s := at
		switch e.Name {
		case "text":
			s.Kind, s.Text = SeenWords, str(e.Data)
		case instance.EmitLog:
			s.Kind, s.Name, s.Text = SeenLog, e.Job, str(e.Data)
		default:
			s.Kind, s.Name, s.Text = SeenEmit, e.Job+": "+e.Name, str(e.Data)
		}
		return []Seen{s}
	}
	var seen []Seen
	for _, event := range e.Events {
		if event.From == "" || own(event.From) || event.Name != machine.Done && event.Name != machine.Failed {
			continue
		}
		s := at
		s.Name = toolName(event.From)
		if event.Name == machine.Done {
			s.Kind, s.Text = SeenResult, resultText(event.Data)
			seen = append(seen, s)
			continue
		}
		failure, _ := event.Data.(map[string]any)
		s.Kind, s.Text = SeenFailed, strings.TrimSpace(str(failure["status"])+": "+str(failure["error"]))
		if failure["status"] == machine.StatusDenied {
			s.Kind, s.Refused, s.Layer, s.Fix = SeenRefused, str(failure["refused"]), str(failure["layer"]), str(failure["fix"])
		}
		seen = append(seen, s)
	}
	for _, ask := range e.Asks {
		if ask.Kind != instance.AskScript && ask.Kind != instance.AskChild || ask.Job == nil || own(ask.Name) {
			continue
		}
		s := at
		s.Kind, s.Name, s.Text = SeenCall, toolName(ask.Name), str(ask.Job.Input)
		seen = append(seen, s)
	}
	return seen
}

// own says whether a job is one of an agent's own, and not a tool: a turn,
// or its look at a new blob.
func own(name string) bool {
	switch toolName(name) {
	case "turn", "preview":
		return true
	}
	return false
}

// toolName gives a job's name without what Corbie adds to it for a job
// asked for again or among several.
func toolName(job string) string { return decisions.Repeats.ReplaceAllString(job, "") }

// resultText writes what a tool gave back as a watcher reads it: for a
// tool that was an agent, what it replied.
func resultText(data any) string {
	if out, ok := data.(map[string]any); ok && out["conversation"] != nil {
		if reply, ok := out["reply"]; ok {
			return str(reply)
		}
	}
	return str(data)
}
