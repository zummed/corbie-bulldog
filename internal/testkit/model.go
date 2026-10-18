package testkit

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zummed/corbie/plugin/llm/standin"
)

// A Model stands in for a model service that speaks as Anthropic's does,
// as Corbie's own stand-in. Each turn an agent takes is given to the test,
// which says what the model replies; every turn is kept, for the test to
// read what was shown.
type Model struct {
	// URL is where the service is.
	URL    string
	server *httptest.Server
	answer func(Turn) Reply
	calls  atomic.Int64

	mu    sync.Mutex
	turns []Turn
}

// A Turn is one call to the model: what it was shown.
type Turn struct {
	// System is the system text.
	System string
	// Messages are the conversation so far, as the service is sent them.
	Messages []Message
	// Tools are the names of the tools the model is offered.
	Tools []string
	// ToolsGiven are the tools as the model is told of them.
	ToolsGiven []ToolGiven
	// Number counts the turns of one conversation from 1.
	Number int
}

// A Message is one message of a conversation, as the service is sent it:
// content sent as text alone is a block of text.
type Message struct {
	Role    string           `json:"role"`
	Content []map[string]any `json:"content"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	var sent struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	if err := json.Unmarshal(data, &sent); err != nil {
		return err
	}
	m.Role, m.Content = sent.Role, nil
	switch content := sent.Content.(type) {
	case string:
		m.Content = []map[string]any{{"type": "text", "text": content}}
	case []any:
		for _, block := range content {
			if b, ok := block.(map[string]any); ok {
				m.Content = append(m.Content, b)
			}
		}
	}
	return nil
}

// A ToolGiven is a tool as the model is told of it.
type ToolGiven struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"input_schema"`
}

// Prompt is the text of the conversation's first message.
func (t Turn) Prompt() string {
	if len(t.Messages) == 0 {
		return ""
	}
	return textOf(t.Messages[0].Content)
}

// Last is the text of the latest message: what the model was last told,
// tool results among it.
func (t Turn) Last() string {
	if len(t.Messages) == 0 {
		return ""
	}
	return textOf(t.Messages[len(t.Messages)-1].Content)
}

// textOf gives the text in a message's content, a tool result's text
// among it, one part to a line.
func textOf(content []map[string]any) string {
	var parts []string
	for _, block := range content {
		switch block["type"] {
		case "text":
			parts = append(parts, fmt.Sprint(block["text"]))
		case "tool_result":
			switch inner := block["content"].(type) {
			case string:
				parts = append(parts, inner)
			case []any:
				for _, item := range inner {
					if m, ok := item.(map[string]any); ok && m["type"] == "text" {
						parts = append(parts, fmt.Sprint(m["text"]))
					}
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

// A Reply is what the model says in a turn: words, and the tools it calls.
// A reply that calls no tool ends the agent's turn with its words.
type Reply struct {
	Text  string
	Calls []Call
}

// A Call is one tool the model calls, with its arguments.
type Call struct {
	Name string
	Args any
}

// Say is a reply of words alone.
func Say(text string) Reply { return Reply{Text: text} }

// Use is a reply that calls a tool.
func Use(name string, args any) Reply { return Reply{Calls: []Call{{name, args}}} }

// And adds a call to a reply.
func (r Reply) And(name string, args any) Reply {
	r.Calls = append(r.Calls, Call{name, args})
	return r
}

// Script answers each turn of a conversation with the reply of its place:
// the first turn the first, and so on. A turn past the last is answered
// with "done".
func Script(replies ...Reply) func(Turn) Reply {
	return func(t Turn) Reply {
		if t.Number <= len(replies) {
			return replies[t.Number-1]
		}
		return Say("done")
	}
}

// NewModel starts a model that answers each turn as answer says, and stops
// it when the test ends. Turns may come side by side, and answer may take
// as long as it likes: a turn the test holds holds the agent, and the
// model is stopped only once every answer has been given, so a turn held
// is let go before the test ends, by a defer or a cleanup.
func NewModel(t testing.TB, answer func(Turn) Reply) *Model {
	m := StartModel(answer)
	t.Cleanup(m.Close)
	return m
}

// StartModel starts a model as NewModel does, for a program that is not a
// test, such as the demo, which stops it with Close.
func StartModel(answer func(Turn) Reply) *Model {
	m := &Model{answer: answer}
	m.server = httptest.NewServer(&standin.Server{Answer: m.reply})
	m.URL = m.server.URL
	return m
}

// Close stops the model, once every turn being answered has its answer.
func (m *Model) Close() {
	m.server.CloseClientConnections()
	m.server.Close()
}

// Line is the model as a definition or a team file writes it.
func (m *Model) Line() string {
	return fmt.Sprintf(`{ name: claude-test, service: anthropic, address: "%s", secret: %s }`, m.URL, Key)
}

// Turns gives every turn taken so far, in the order they came.
func (m *Model) Turns() []Turn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Turn(nil), m.turns...)
}

// reply answers one call as the test says, and keeps it as a turn.
func (m *Model) reply(c standin.Call) standin.Reply {
	turn, err := turnOf(c)
	if err != nil {
		return standin.Reply{Error: "the stand-in model cannot read what it was sent: " + err.Error()}
	}
	m.mu.Lock()
	m.turns = append(m.turns, turn)
	m.mu.Unlock()
	said := m.answer(turn)
	out := standin.Reply{Text: said.Text}
	for _, call := range said.Calls {
		args, err := json.Marshal(call.Args)
		if err != nil || call.Args == nil {
			args = []byte("{}")
		}
		out.Calls = append(out.Calls, standin.ToolCall{ID: fmt.Sprintf("toolu_%d", m.calls.Add(1)), Name: call.Name, Args: string(args)})
	}
	return out
}

// turnOf gives what a call showed the model as a turn of its
// conversation.
func turnOf(c standin.Call) (Turn, error) {
	data, err := json.Marshal(map[string]any{"messages": c.Messages, "tools": c.Tools})
	if err != nil {
		return Turn{}, err
	}
	var sent struct {
		Messages []Message   `json:"messages"`
		Tools    []ToolGiven `json:"tools"`
	}
	if err := json.Unmarshal(data, &sent); err != nil {
		return Turn{}, err
	}
	turn := Turn{System: c.System, Messages: sent.Messages, ToolsGiven: sent.Tools, Number: (len(sent.Messages) + 1) / 2}
	for _, tool := range sent.Tools {
		turn.Tools = append(turn.Tools, tool.Name)
	}
	return turn, nil
}
