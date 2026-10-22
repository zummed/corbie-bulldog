package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

// secret is the office's secret in every test.
var secret = []byte("the office's secret in the tests")

// bridged gives the work office with three bridges: Telegram, where Dana
// and Lee ask the office directly; Slack, where payments is asked in
// #refunds and anyone may ask the office in #help; and GitLab, which
// reaches nobody directly.
func bridged(files map[string]string) {
	files["office.yaml"] = strings.Replace(files["office.yaml"], `everyone:
  dana: { name: Dana }
  lee: { name: Lee }
  priya: { name: Priya }
  al: { name: Al }
`, `everyone:
  dana: { name: Dana, telegram: "4401", slack: U0DANA }
  lee: { name: Lee, telegram: "4402", slack: U0LEE }
  priya: { name: Priya, slack: U0PRIYA }
  al: { name: Al, gitlab: "1041" }
bridges:
  telegram:
    from: ./.bulldog/bridges/telegram
    places:
      direct: {}
  slack:
    from: ./.bulldog/bridges/slack
    places:
      "#help": { guests: true }
      "#refunds": { team: payments }
  gitlab:
    from: ./.bulldog/bridges/gitlab
    site: gitlab.example
    places:
      ops/api: {}
`, 1)
}

// A carrier stands in for the bridges. It carries a message by a run that
// gives { sent }, and keeps every message it carried. Telegram and Slack
// reach people directly; GitLab does not.
type carrier struct {
	corbie *testkit.Corbie
	draft  *machine.Draft
	mu     sync.Mutex
	sent   []Message
}

func newCarrier(t *testing.T, corbie *testkit.Corbie) *carrier {
	files := map[string]string{
		"out.yaml": "flow: out\ninput: { sent: string }\nstages:\n  send: { job: { script: ./send.js, input: { sent: \"${input.sent}\" } } }\noutput: ${stages.send.output}\n",
		"send.js":  "return { sent: input.sent }",
	}
	draft, err := machine.Parse([]byte(files["out.yaml"]), func(name string) ([]byte, error) { return []byte(files[name]), nil })
	if err != nil {
		t.Fatal(err)
	}
	return &carrier{corbie: corbie, draft: draft}
}

func (c *carrier) Carry(ctx context.Context, m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sent := fmt.Sprintf("%s:%d", m.To, len(c.sent)+1)
	if _, err := c.corbie.Start(ctx, c.draft, client.Start{Run: m.Run, Labels: m.Labels, Input: map[string]any{"sent": sent}}); err != nil {
		return err
	}
	c.sent = append(c.sent, m)
	return nil
}

func (c *carrier) Direct(bridge string) bool { return bridge == "telegram" || bridge == "slack" }

// carried gives every message carried so far.
func (c *carrier) carried() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.sent)
}

// A driven office is a world with a stand-in carrier, for an engine to
// drive.
type driven struct {
	*world
	carrier *carrier
}

// newOffice writes the work office with its bridges, changed as the test
// changes it, and gives it with a stand-in carrier.
func newOffice(t *testing.T, answer func(testkit.Turn) testkit.Reply, change func(files map[string]string)) *driven {
	w := newWorld(t, answer, func(files map[string]string) {
		bridged(files)
		if change != nil {
			change(files)
		}
	})
	if len(w.office.Problems) > 0 {
		t.Fatalf("the office has problems:\n%v", office.Problems(w.office.Problems))
	}
	return &driven{world: w, carrier: newCarrier(t, w.corbie)}
}

// engine makes an engine of the office, which fails the test on anything
// it reports.
func (o *driven) engine() *Engine {
	e := New(o.corbie.Client, o.carrier, secret, &Reading{Office: o.office, Machines: o.made})
	e.Report = func(err error) { o.t.Errorf("the engine reports: %v", err) }
	return e
}

// run starts an engine of the office, and gives what stops it, which the
// end of the test does too.
func (o *driven) run() (*Engine, func()) {
	e := o.engine()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			if err := <-done; err != nil {
				o.t.Errorf("the engine ended: %v", err)
			}
		})
	}
	o.t.Cleanup(stop)
	return e, stop
}

// read reads the office again, changed as the test changes it.
func (o *driven) read(change func(files map[string]string)) *Reading {
	o.t.Helper()
	files := map[string]string{}
	change(files)
	testkit.Write(o.t, o.office.Dir, files)
	read, err := office.Read(o.office.Dir, shipped.Files)
	if err != nil {
		o.t.Fatal(err)
	}
	return NewReading(read)
}

// given gives what a request's run was given: what was said, the context,
// and the request as data.
func (o *driven) given(run string) (said, context string, request map[string]any) {
	o.t.Helper()
	var input map[string]any
	o.corbie.Until(o.t, "the run "+run+" has started", func() bool {
		shown, err := o.corbie.Show(ctx, run)
		if err != nil && !errors.Is(err, client.ErrNoRun) {
			o.t.Fatal(err)
		}
		input, _ = shown.Input.(map[string]any)
		return input != nil
	})
	return input["said"].(string), input["context"].(string), input["request"].(map[string]any)
}

// requests gives the requests of a team, going or ended, by their run.
func (o *driven) requests(team string) []*client.Run {
	o.t.Helper()
	runs, err := o.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": o.office.ID, "kind": "request", "team": team}, Finished: true})
	if err != nil {
		o.t.Fatal(err)
	}
	return slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Parent != "" })
}

// until stays until the carrier has carried n messages, and gives them.
func (o *driven) until(n int) []Message {
	o.t.Helper()
	o.corbie.Until(o.t, fmt.Sprintf("%d messages are carried", n), func() bool { return len(o.carrier.carried()) >= n })
	return o.carrier.carried()
}

// ended stays until a run has ended, and gives it as shown.
func (o *driven) ended(run string) *client.Run {
	o.t.Helper()
	return o.corbie.Ended(o.t, run)
}

// waitFor stays until a wait waits on its answer whose input holds a text,
// and gives it.
func (o *driven) waitFor(text string) *client.Run {
	o.t.Helper()
	var found *client.Run
	o.corbie.Until(o.t, "a wait asks "+text, func() bool {
		runs, err := o.corbie.List(ctx, client.Filter{Labels: map[string]string{"bulldog": o.office.ID}})
		if err != nil {
			o.t.Fatal(err)
		}
		for _, r := range runs {
			if r.Machine != "wait" || !waits(r, "answer") {
				continue
			}
			shown, err := o.corbie.Show(ctx, r.ID)
			if err != nil {
				o.t.Fatal(err)
			}
			if strings.Contains(fmt.Sprint(shown.Input), text) {
				found = shown
				return true
			}
		}
		return false
	})
	return found
}

// chat asks a team in the chat, as Dana.
func (o *driven) chat(e *Engine, team, said string) string {
	o.t.Helper()
	run, err := e.Ask(ctx, Ask{Team: team, By: By{Person: "dana"}, Said: said})
	if err != nil {
		o.t.Fatal(err)
	}
	return run
}

// at gives a place on Slack, its where and the id of the thing heard.
func at(place, where, thing string) *Place {
	return &Place{Bridge: "slack", Place: place, Where: where, Thing: thing}
}

// answered stays until a request has ended, and gives its answer.
func (o *driven) answered(run string) string {
	o.t.Helper()
	out := o.output(run)
	answer, _ := out["answer"].(string)
	return answer
}

// stamp is the time a request heard on a bridge was said, in the tests.
var stamp = testkit.Monday.Add(-time.Minute)
