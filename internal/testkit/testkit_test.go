package testkit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

func TestMain(m *testing.M) { Main(m) }

// start starts a run of a definition given with its files, and gives the
// run's name.
func start(t *testing.T, corbie *Corbie, src string, files map[string]string, input any) string {
	t.Helper()
	draft, err := machine.Parse([]byte(src), machine.Given(files))
	if err != nil {
		t.Fatal(err)
	}
	run, err := corbie.Start(context.Background(), draft, client.Start{Input: input, Labels: map[string]string{"kind": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestAFlowRunsToItsEnd(t *testing.T) {
	t.Parallel()
	corbie := NewCorbie(t)
	run := start(t, corbie, `flow: adds
input: { n: number }
stages:
  add: { job: { script: ./add.js, input: { n: "${input.n}" } } }
output: ${stages.add.output}
`, map[string]string{"add.js": `return input.n + 1`}, map[string]any{"n": 1})
	ended := corbie.Ended(t, run)
	if ended.Status != "done" || ended.Output != 2.0 {
		t.Fatalf("the flow ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
}

func TestAnAgentIsAnsweredByTheModel(t *testing.T) {
	t.Parallel()
	corbie := NewCorbie(t)
	model := NewModel(t, func(turn Turn) Reply {
		if turn.Number == 1 {
			return Use("shout", map[string]any{"word": "hello"})
		}
		return Say("It said " + turn.Last() + ".")
	})
	run := start(t, corbie, `agent: echo
input: { said: string }
model: `+model.Line()+`
system: You echo.
prompt: ${input.said}
tools:
  shout: { does: Shout a word, args: { word: string }, job: { script: ./shout.js, input: { word: "${args.word}" } } }
`, map[string]string{"shout.js": `return input.word.toUpperCase()`}, map[string]any{"said": "Say hello."})
	ended := corbie.Ended(t, run)
	if ended.Status != "done" {
		t.Fatalf("the agent ended %s: %s", ended.Status, ended.Error)
	}
	if reply, _ := ended.Output.(map[string]any)["reply"].(string); reply != "It said HELLO." {
		t.Errorf("the agent replied %#v", ended.Output)
	}
	turns := model.Turns()
	if len(turns) != 2 {
		t.Fatalf("the model had %d turns: %#v", len(turns), turns)
	}
	first, second := turns[0], turns[1]
	if first.Number != 1 || first.System != "You echo." || first.Prompt() != "Say hello." || first.Tools["shout"].Description != "Shout a word" {
		t.Errorf("the first turn is %#v", first)
	}
	if second.Number != 2 || second.Last() != "HELLO" {
		t.Errorf("the second turn is %#v", second)
	}
}

func TestAStatechartWaitsOnATicket(t *testing.T) {
	t.Parallel()
	corbie := NewCorbie(t)
	run := start(t, corbie, `statechart: asks
initial: asking
states:
  asking: { ticket: answer, on: { answer: answered } }
  answered: { end: done, output: "${event.data}" }
`, nil, nil)
	ticket := corbie.Waiting(t, run, "answer")
	if _, err := corbie.Reply(context.Background(), run, ticket, "yes", 0); err != nil {
		t.Fatal(err)
	}
	if ended := corbie.Ended(t, run); ended.Status != "done" || ended.Output != "yes" {
		t.Fatalf("the statechart ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
}

func TestATimerFallsDueWhenTheClockMoves(t *testing.T) {
	t.Parallel()
	corbie := NewCorbie(t)
	run := start(t, corbie, `statechart: waits
initial: waiting
states:
  waiting: { timers: { due: 2h }, on: { due: woken } }
  woken: { end: done, output: woken }
`, nil, nil)
	corbie.Until(t, "the statechart has a timer", func() bool {
		shown, err := corbie.Show(context.Background(), run)
		return err == nil && shown.Timers == 1
	})
	corbie.Clock.Move(2 * time.Hour)
	ended := corbie.Ended(t, run)
	if ended.Status != "done" || ended.Output != "woken" || !ended.Ended.Equal(Monday.Add(2*time.Hour)) {
		t.Fatalf("the statechart ended %s with %#v at %v: %s", ended.Status, ended.Output, ended.Ended, ended.Error)
	}
}

func TestAKeptValueOutlivesItsRun(t *testing.T) {
	t.Parallel()
	corbie := NewCorbie(t)
	keeps := `flow: keeps
stages:
  it: { job: { script: ./it.js, allow: ["kept.read:notes/**", "kept.write:notes/**"] } }
output: ${stages.it.output}
`
	wrote := corbie.Ended(t, start(t, corbie, keeps, map[string]string{"it.js": `kept.write("notes/tea", { text: "Tea is up." })`}, nil))
	if wrote.Status != "done" {
		t.Fatalf("the run that keeps a value ended %s: %s", wrote.Status, wrote.Error)
	}
	read := corbie.Ended(t, start(t, corbie, keeps, map[string]string{"it.js": `return kept.read("notes/tea")`}, nil))
	if read.Status != "done" || read.Output != "Tea is up." {
		t.Fatalf("a later run ended %s with %#v: %s", read.Status, read.Output, read.Error)
	}
}

func TestAScriptAnswersByTurn(t *testing.T) {
	t.Parallel()
	answer := Script(Say("One."), Use("look", nil).And("ask", map[string]any{"who": "sam"}))
	if got := answer(Turn{Number: 1}); got.Text != "One." {
		t.Errorf("the first turn is %#v", got)
	}
	if got := answer(Turn{Number: 2}); len(got.Calls) != 2 || got.Calls[0].Name != "look" || got.Calls[1].Name != "ask" {
		t.Errorf("the second turn is %#v", got)
	}
	if got := answer(Turn{Number: 3}); got.Text != "done" {
		t.Errorf("a turn after the script is %#v", got)
	}
}

func TestADirIsWrittenFromItsFiles(t *testing.T) {
	t.Parallel()
	dir := Dir(t, map[string]string{"office.yaml": "name: A\n", "garden/team.yaml": "name: B\n"})
	files := Files(t, os.DirFS(dir))
	if len(files) != 2 || files["office.yaml"] != "name: A\n" || files["garden/team.yaml"] != "name: B\n" {
		t.Errorf("the directory holds %#v", files)
	}
}
