package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// decider runs one rule or worker asked to decide, for the decisions, and
// gives what it ended with. Its run is of the team it decides for, and
// carries the labels it is given; a run already started with the same
// labels, before Bulldog stopped, is waited on rather than started again.
// Cancelling the context stops the run, but while Bulldog itself stops,
// when the run goes on in Corbie and is waited on again once it is back.
func (w *Work) decider(ctx context.Context, s decisions.Start) (json.RawMessage, error) {
	labels := w.labels(s.Labels)
	n, err := w.notedAs(labels)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", decisions.ErrNotStarted, err)
	}
	name := ""
	if n != nil {
		name = n.Name
	} else if name, err = w.startDecider(ctx, s, labels); err != nil {
		return nil, fmt.Errorf("%w: %v", decisions.ErrNotStarted, err)
	}
	run, err := w.corbie.Wait(ctx, name, w.Interval)
	if ctx.Err() != nil {
		// While Bulldog stops, the run goes on in Corbie, and is waited on
		// again when it is back; otherwise it is no longer wanted.
		if w.running.Load() {
			if err := w.stopRun(name, stopped); err != nil {
				w.Log("the run %s of %s, no longer waited for, could not be stopped: %v", name, s.Name, err)
			}
			w.stopsNoted()
		}
		return nil, ctx.Err()
	}
	switch {
	case errors.Is(err, client.ErrNoRun):
		return nil, fmt.Errorf("%w: Corbie never took its run up", decisions.ErrNotStarted)
	case err != nil:
		return nil, err
	case run.Status != "done":
		_, how := outcome(run)
		return nil, errors.New(how)
	}
	return json.Marshal(output(run))
}

// startDecider starts a rule or a worker asked to decide, noted first.
func (w *Work) startDecider(ctx context.Context, s decisions.Start, labels map[string]string) (string, error) {
	o := w.keeper.Office()
	if o == nil {
		return "", errors.New("the office has not been read")
	}
	var draft *machine.Draft
	var input any
	switch s.Kind {
	case office.IsRule:
		rule := o.Rules[s.Name]
		switch {
		case rule == nil:
			return "", fmt.Errorf("%s is no rule of the office", s.Name)
		case !rule.Runs():
			return "", fmt.Errorf("%s cannot run until its file is mended: %v", s.Name, rule.Problems)
		case rule.Script:
			var err error
			if draft, err = client.Script(path.Base(rule.Path), rule.Written, client.ScriptJob{}); err != nil {
				return "", err
			}
		default:
			draft = rule.Draft
		}
		input = s.Input
	case office.IsWorker:
		worker := o.Workers[s.Name]
		if worker == nil || !worker.Runs() {
			return "", errors.New(problem(o, s.Name))
		}
		draft = worker.Draft
		given, err := w.deciding(o, s)
		if err != nil {
			return "", err
		}
		input = given
	default:
		return "", fmt.Errorf("a %s does not decide", s.Kind)
	}
	name, err := w.corbie.NewRun()
	if err != nil {
		return "", err
	}
	n := &note{Name: name, Kind: labels[LabelKind], Team: s.Team, Request: s.Request, Worker: s.Name, Labels: labels, Noted: w.clock.Now()}
	if err := w.keep(n); err != nil {
		return "", err
	}
	if _, err := w.corbie.Start(ctx, draft, client.Start{Run: name, Input: input, Labels: labels}); err != nil {
		w.endUnsent(name, err)
		return "", err
	}
	return name, nil
}

// deciding gives the three inputs of a worker asked to decide for a team:
// the decision in words, the team it decides for with that team's notes,
// and the decision as data.
func (w *Work) deciding(o *office.Office, s decisions.Start) (map[string]any, error) {
	team := o.Teams[s.Team]
	if team == nil {
		return nil, fmt.Errorf("%s is no team of the office", s.Team)
	}
	notes, err := o.Notes(s.Team)
	if err != nil {
		return nil, fmt.Errorf("the notes of %s could not be read: %w", team.Called, err)
	}
	now := w.clock.Now().In(o.Zone)
	shown, left := shownNotes(notes)
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("It is %s, in the office's time zone, %s.", now.Format("Monday 2 January 2006, 15:04 MST"), o.Zone)
	line("")
	line("You are %s, deciding for %s (%s): %s", s.Name, team.Called, team.Name, orNothing(team.About))
	if len(shown) > 0 {
		line("")
		notesText(line, team.Called, shown, left)
	}
	line("")
	line("%s", s.Said)
	line("")
	line("Your last reply is your decision.")
	return map[string]any{"said": s.Said, "context": b.String(), "request": map[string]any{
		"number": s.Request, "now": now.Format(time.RFC3339), "zone": o.Zone.String(),
		"team":     map[string]any{"name": team.Name, "called": team.Called, "about": team.About},
		"decision": s.Input, "notes": notesData(shown), "notes_not_shown": left,
	}}, nil
}
