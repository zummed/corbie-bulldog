package work

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/instance"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// watching is the house with a fixer whose tools read a meter, which logs
// what it reads, and peek at what they may not.
func watching(t *testing.T) map[string]string {
	files := house(t)
	files["fixer.yaml"] = "agent: fixer\nmodel: usual\ninput: [said]\nprompt: ${input.said}\ntools_file: [./.bulldog/tools/talk/talk.yaml]\ntools:\n" +
		"  meter: { does: Read the meter, args: {}, job: { script: ./tools/meter.js } }\n" +
		"  peek: { does: Look at what it may not, args: {}, job: { script: ./tools/peek.js } }\n"
	files["tools/meter.js"] = "log(\"the meter reads 42\")\nreturn 42\n"
	files["tools/peek.js"] = "return env.get(\"HOME\")\n"
	return files
}

// A watcher is what someone watching a run has been shown so far, and how
// the watching ended.
type watcher struct {
	mu   sync.Mutex
	seen []Seen
	done chan error
}

// watch has a person watch a run, on a goroutine of its own, as the
// screens would.
func (b *bench) watch(run, person string) *watcher {
	w := &watcher{done: make(chan error, 1)}
	go func() {
		w.done <- b.w.Follow(ctx, run, person, func(s Seen) {
			w.mu.Lock()
			w.seen = append(w.seen, s)
			w.mu.Unlock()
			b.poke()
		})
		b.poke()
	}()
	return w
}

// shown gives what the watcher was shown of a kind.
func (w *watcher) shown(kind string) []Seen {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(w.seen), func(s Seen) bool { return s.Kind != kind })
}

// words gives the words of a machine's model the watcher was shown, in
// the pieces they came in.
func (w *watcher) words(machine string) []string {
	var pieces []string
	for _, s := range w.shown(SeenWords) {
		if s.Machine == machine {
			pieces = append(pieces, s.Text)
		}
	}
	return pieces
}

func TestARunIsWatchedAsItHappens(t *testing.T) {
	t.Parallel()
	b := newBench(t, watching(t))
	const mended = "The kitchen socket is mended, and the circuit is back on."
	gate := make(chan struct{})
	var once sync.Once
	let := func() { once.Do(func() { close(gate) }) }
	defer let()
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case turn.Prompt() == "mend" && turn.Number == 1:
			return testkit.Use("meter", map[string]any{})
		case turn.Prompt() == "mend" && turn.Number == 2:
			return testkit.Use("peek", map[string]any{})
		case turn.Prompt() == "mend" && turn.Number == 3:
			return testkit.Use("ask_person", map[string]any{"question": "which socket?"})
		case turn.Prompt() == "mend":
			return testkit.Say(mended)
		case turn.Number == 1:
			return testkit.Use("fixer", map[string]any{"job": "mend"})
		}
		<-gate
		return testkit.Say("done")
	})
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	w := b.watch(run.Name, "kit")
	q := b.waitsOn(n, requests.KindQuestion)
	if _, err := b.reqs.Reply(q.ID, b.reqs.Person("kit"), "the one by the kettle", nil, requests.Screens); err != nil {
		t.Fatal(err)
	}
	// The steward is held after the fixer's answer: the run goes on, and
	// the watcher has seen the fixer's reply arrive in pieces.
	b.until("the fixer's reply is watched", func() bool { return strings.Join(w.words("fixer"), "") == mended })
	if pieces := w.words("fixer"); len(pieces) < 2 {
		t.Errorf("the fixer's reply came in %d pieces: %q", len(pieces), pieces)
	}
	if ended := w.shown(SeenEnded); len(ended) != 0 {
		t.Fatalf("the run was shown as ended while it went: %+v", ended)
	}
	called := func(kind, name, part string) bool {
		return slices.ContainsFunc(w.shown(kind), func(s Seen) bool { return s.Name == name && strings.Contains(s.Text, part) })
	}
	if !called(SeenCall, "ask_person", "which socket?") || !called(SeenResult, "ask_person", "the one by the kettle") {
		t.Errorf("the question is not watched with its answer: %+v %+v", w.shown(SeenCall), w.shown(SeenResult))
	}
	if !called(SeenLog, "meter", "the meter reads 42") || !called(SeenResult, "meter", "42") {
		t.Errorf("the meter's log is not watched: %+v", w.shown(SeenLog))
	}
	refused := w.shown(SeenRefused)
	if len(refused) != 1 || refused[0].Name != "peek" || refused[0].Refused != "env:HOME" || refused[0].Layer == "" || refused[0].Fix == "" {
		t.Errorf("the refusal is watched as %+v", refused)
	}
	let()
	if err := <-w.done; err != nil {
		t.Fatal(err)
	}
	if ended := w.shown(SeenEnded); len(ended) != 1 || ended[0].Text != done || ended[0].Machine != "steward" {
		t.Errorf("the run is shown ending as %+v", ended)
	}
	// Nobody else may watch it, and to them it is not there.
	if err := b.w.Follow(ctx, run.Name, "dana", func(Seen) {}); !errors.Is(err, ErrNoWatch) {
		t.Errorf("someone of no team of the house watched it: %v", err)
	}
	if err := b.w.Follow(ctx, "no such run", "kit", func(Seen) {}); !errors.Is(err, ErrNoWatch) {
		t.Errorf("a run that is not there was watched: %v", err)
	}
}

func TestWorkRefusalShownWithFix(t *testing.T) {
	t.Parallel()
	b := newBench(t, desk(house(t), `return env.get("HOME")`))
	n := b.chat("kit", "desk", "file the bill")
	run := b.started(n, 1)
	w := b.watch(run.Name, "kit")
	b.settled(n, requests.Answered)
	if err := <-w.done; err != nil {
		t.Fatal(err)
	}
	refused := w.shown(SeenRefused)
	if len(refused) != 1 {
		t.Fatalf("the refusals watched are %+v", refused)
	}
	r := refused[0]
	if r.Refused != "env:HOME" || !strings.Contains(r.Layer, "filer") || !strings.Contains(r.Fix, "env:HOME") {
		t.Errorf("the refusal is shown as %+v", r)
	}
	if ended := w.shown(SeenEnded); len(ended) != 1 || ended[0].Fix != r.Fix || !strings.Contains(ended[0].Text, "To let it: "+r.Fix) {
		t.Errorf("the run is shown ending as %+v", ended)
	}
}

func TestWorkWatchNamesEachToolCall(t *testing.T) {
	t.Parallel()
	// Corbie names a job beyond its tool when it is one of several asked
	// for in one step, asked for again, or another attempt: a turn tried
	// again and a second preview are still the agent's own, and a second
	// call of a tool in one turn is still that tool.
	job := &machine.Job{}
	e := client.Entry{ID: "r", Asks: []instance.Ask{
		{Kind: instance.AskScript, Name: "turn@2", Job: job},
		{Kind: instance.AskScript, Name: "preview.1", Job: job},
		{Kind: instance.AskChild, Name: "shelf", Job: job},
		{Kind: instance.AskChild, Name: "shelf.1", Job: job},
		{Kind: instance.AskScript, Name: "meter#2@3", Job: job},
	}, Events: []instance.Arrived{
		{Event: machine.Event{From: "turn@2", Name: machine.Done}},
		{Event: machine.Event{From: "shelf.1", Name: machine.Done}},
	}}
	var seen []string
	for _, s := range seenOf(e) {
		seen = append(seen, s.Kind+" "+s.Name)
	}
	if want := []string{SeenResult + " shelf", SeenCall + " shelf", SeenCall + " shelf", SeenCall + " meter"}; !slices.Equal(seen, want) {
		t.Errorf("the watcher is shown %q", seen)
	}
}
