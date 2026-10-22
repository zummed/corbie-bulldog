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

// A Model is a stand-in for a model service that speaks as Anthropic's
// does. The test gives each turn of an agent the model's reply, and every
// turn is kept for the test to read. A call without KeyValue as its key is
// refused.
type Model struct {
	// URL is the address of the service.
	URL    string
	server *httptest.Server
	answer func(Turn) Reply
	calls  atomic.Int64

	mu    sync.Mutex
	turns []Turn
}

// A Turn is one call to the model: what it was sent.
type Turn struct {
	System string
	// Messages are the conversation so far, as the service was sent it.
	Messages []Message
	// Tools are the tools the model is offered, by name.
	Tools map[string]Tool
	// Number counts the turns of one conversation from 1.
	Number int
}

// A Message is one message of a conversation. Content sent as a string is
// one block of text.
type Message struct {
	Role    string           `json:"role"`
	Content []map[string]any `json:"content"`
}

// UnmarshalJSON reads a message as the service was sent it.
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

// A Tool is a tool as the model is told of it.
type Tool struct {
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

// Last is the text of the latest message, tool results among it.
func (t Turn) Last() string {
	if len(t.Messages) == 0 {
		return ""
	}
	return textOf(t.Messages[len(t.Messages)-1].Content)
}

// textOf gives the text of a message's content, each block of text and each
// tool result on a line of its own.
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

// A Reply is what the model says in a turn: text, and the tools it calls.
// A reply that calls no tool ends the agent's turn with its text.
type Reply struct {
	Text  string
	Calls []Call
}

// A Call is one tool the model calls, with its arguments.
type Call struct {
	Name string
	Args any
}

// Say is a reply of text alone.
func Say(text string) Reply { return Reply{Text: text} }

// Use is a reply that calls a tool.
func Use(name string, args any) Reply { return Reply{Calls: []Call{{name, args}}} }

// And adds a call to a reply.
func (r Reply) And(name string, args any) Reply {
	r.Calls = append(r.Calls, Call{name, args})
	return r
}

// Script answers the turns of a conversation in order: the first turn with
// the first reply, and so on. A turn after the last reply is answered
// "done".
func Script(replies ...Reply) func(Turn) Reply {
	return func(t Turn) Reply {
		if t.Number <= len(replies) {
			return replies[t.Number-1]
		}
		return Say("done")
	}
}

// NewModel starts a model that gives each turn the reply answer gives, and
// stops it when the test ends. Turns may come side by side. A turn waits
// while answer runs, and the model stops only once every answer has
// returned, so a test that holds a turn lets it go before it ends.
func NewModel(t testing.TB, answer func(Turn) Reply) *Model {
	m := StartModel(answer)
	t.Cleanup(m.Close)
	return m
}

// StartModel starts a model as NewModel does, for a program that is not a
// test, which stops it with Close.
func StartModel(answer func(Turn) Reply) *Model {
	m := &Model{answer: answer}
	m.server = httptest.NewServer(&standin.Server{Answer: m.reply, Key: KeyValue})
	m.URL = m.server.URL
	return m
}

// Close stops the model once every answer has returned.
func (m *Model) Close() {
	m.server.CloseClientConnections()
	m.server.Close()
}

// Line is the model as a definition or a team file writes it, naming the
// secret Key.
func (m *Model) Line() string {
	return fmt.Sprintf(`{ name: claude-test, service: anthropic, address: "%s", secret: %s }`, m.URL, Key)
}

// Turns gives every turn so far, in the order they came.
func (m *Model) Turns() []Turn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Turn(nil), m.turns...)
}

// reply keeps a call as a turn, and answers it with the test's reply.
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
		args := []byte("{}")
		if call.Args != nil {
			if args, err = json.Marshal(call.Args); err != nil {
				return standin.Reply{Error: fmt.Sprintf("the test's arguments to %s are not JSON: %v", call.Name, err)}
			}
		}
		out.Calls = append(out.Calls, standin.ToolCall{ID: fmt.Sprintf("toolu_%d", m.calls.Add(1)), Name: call.Name, Args: string(args)})
	}
	return out
}

// turnOf reads what a call sent the model as a turn of its conversation.
func turnOf(c standin.Call) (Turn, error) {
	data, err := json.Marshal(map[string]any{"messages": c.Messages, "tools": c.Tools})
	if err != nil {
		return Turn{}, err
	}
	var sent struct {
		Messages []Message `json:"messages"`
		Tools    []struct {
			Name string `json:"name"`
			Tool
		} `json:"tools"`
	}
	if err := json.Unmarshal(data, &sent); err != nil {
		return Turn{}, err
	}
	turn := Turn{System: c.System, Messages: sent.Messages, Tools: map[string]Tool{}, Number: (len(sent.Messages) + 1) / 2}
	for _, tool := range sent.Tools {
		turn.Tools[tool.Name] = tool.Tool
	}
	return turn, nil
}
