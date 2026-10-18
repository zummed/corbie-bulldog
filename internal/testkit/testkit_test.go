package testkit

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

func TestMain(m *testing.M) { Main(m) }

// draft fills out a definition with the files beside it.
func draft(t *testing.T, src string, files map[string]string) *machine.Draft {
	t.Helper()
	d, err := machine.Parse([]byte(src), machine.Given(files))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAnAgentIsAnsweredByTheStandIn(t *testing.T) {
	t.Parallel()
	clock := NewClock(Monday)
	corbie := NewCorbie(t, clock)
	model := NewModel(t, func(turn Turn) Reply {
		if turn.Number == 1 {
			return Use("shout", map[string]any{"word": "hello"})
		}
		return Say("it said " + turn.Last())
	})
	d := draft(t, `agent: echo
input: [said]
model: `+model.Line()+`
system: You echo.
prompt: ${input.said}
tools:
  shout: { does: Shout a word, args: { word: string }, job: { script: ./shout.js, input: { word: "${args.word}" } } }
`, map[string]string{"shout.js": `return input.word.toUpperCase()`})
	run, err := corbie.Start(context.Background(), d, client.Start{Input: map[string]any{"said": "say hello"}, Labels: map[string]string{"kind": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	ended := corbie.Ended(t, run)
	if ended.Status != "done" {
		t.Fatalf("the run ended %s: %s", ended.Status, ended.Error)
	}
	if text, _ := ended.Output.(map[string]any)["reply"].(string); !strings.Contains(text, "HELLO") {
		t.Errorf("the reply holds no tool result: %#v", ended.Output)
	}
	turns := model.Turns()
	if len(turns) != 2 || turns[0].Prompt() != "say hello" || turns[0].System != "You echo." || !strings.Contains(strings.Join(turns[0].Tools, " "), "shout") {
		t.Errorf("the turns shown were %#v", turns)
	}
}

func TestAScriptAnswersByTurn(t *testing.T) {
	t.Parallel()
	answer := Script(Say("one"), Use("x", nil))
	if got := answer(Turn{Number: 1}); got.Text != "one" {
		t.Errorf("the first turn is %#v", got)
	}
	if got := answer(Turn{Number: 2}); len(got.Calls) != 1 || got.Calls[0].Name != "x" {
		t.Errorf("the second turn is %#v", got)
	}
	if got := answer(Turn{Number: 3}); got.Text != "done" {
		t.Errorf("a turn past the script is %#v", got)
	}
}

func TestATimerComesWhenTheClockMoves(t *testing.T) {
	t.Parallel()
	clock := NewClock(Monday)
	corbie := NewCorbie(t, clock)
	d := draft(t, `statechart: wait
initial: waiting
states:
  waiting: { timers: { due: 2h }, on: { due: asking } }
  asking: { ticket: answer, on: { answer: answered } }
  answered: { end: done, output: "${event.data}" }
`, nil)
	run, err := corbie.Start(context.Background(), d, client.Start{Labels: map[string]string{"kind": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	corbie.Until(t, "the chart is going", func() bool {
		shown, err := corbie.Show(context.Background(), run)
		return err == nil && shown.Timers == 1
	})
	clock.Move(2 * time.Hour)
	ticket := corbie.Waiting(t, run, "answer")
	if _, err := corbie.Reply(context.Background(), run, ticket, "yes", 0); err != nil {
		t.Fatal(err)
	}
	if ended := corbie.Ended(t, run); ended.Output != "yes" || !ended.Ended.Equal(Monday.Add(2*time.Hour)) {
		t.Errorf("the chart ended with %#v at %v", ended.Output, ended.Ended)
	}
}

func TestAFolderIsWrittenFromItsFiles(t *testing.T) {
	t.Parallel()
	dir := Folder(t, map[string]string{"office.yaml": "name: A\n", "garden/team.yaml": "name: B\n"})
	files := Files(t, os.DirFS(dir))
	if files["office.yaml"] != "name: A\n" || files["garden/team.yaml"] != "name: B\n" || len(files) != 2 {
		t.Errorf("the folder holds %#v", files)
	}
}

func TestAModelStartedWithNoTestStopsWhenClosed(t *testing.T) {
	t.Parallel()
	m := StartModel(Script(Say("hello")))
	resp, err := http.Post(m.URL+"/v1/messages", "application/json", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "message_stop") || len(m.Turns()) != 1 {
		t.Errorf("the model answered %q", body)
	}
	m.Close()
	if _, err := http.Post(m.URL+"/v1/messages", "application/json", strings.NewReader(`{}`)); err == nil {
		t.Error("a closed model still answers")
	}
}
