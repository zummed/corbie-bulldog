package requests

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/client"
)

// childrenOf gives the runs each run of a record started, by the name of
// the job each is: heed, work and check of a request, d1 and people of a
// decision.
func childrenOf(entries map[string][]client.Entry) map[string]map[string]string {
	children := map[string]map[string]string{}
	for run, list := range entries {
		children[run] = map[string]string{}
		for _, entry := range list {
			for _, ask := range entry.Asks {
				if ask.Kind == "child" {
					children[run][ask.Name] = ask.Child
				}
			}
		}
	}
	return children
}

// errorOf gives the error of a failed event's data.
func errorOf(data any) string {
	var failed struct {
		Error string `json:"error"`
	}
	decode(data, &failed)
	return failed.Error
}

// words gives what each decider of a decision said, in order, from the
// record of its chart: a rule's says and note, a worker's reply, and the
// answer of the person who answered first. A decider that is a definition
// is a worker when it is a worker's file, and a rule otherwise.
func words(o *office.Office, entries []client.Entry) []Word {
	deciders := map[string]Word{} // by the state that asks the decider
	var said []Word
	for _, entry := range entries {
		for _, ask := range entry.Asks {
			if ask.Job == nil || (ask.Kind != "script" && ask.Kind != "child") {
				continue
			}
			switch job := ask.Job; {
			case ask.Name == "people":
				deciders[ask.Name] = Word{Kind: "person"}
			case job.Script != "":
				deciders[ask.Name] = Word{Kind: "rule", Name: job.Script}
			default:
				deciders[ask.Name] = Word{Kind: "rule", Name: job.Machine}
				if w := o.WorkerAt(strings.TrimPrefix(job.Machine, "./")); w != nil {
					deciders[ask.Name] = Word{Kind: "worker", Name: w.Name}
				}
			}
		}
		for _, event := range entry.Events {
			w, ok := deciders[event.From]
			if !ok {
				continue
			}
			var data struct {
				Says   string `json:"says"`
				Note   string `json:"note"`
				Answer string `json:"answer"`
				By     string `json:"by"`
				Reply  *struct {
					Says string `json:"says"`
					Note string `json:"note"`
				} `json:"reply"`
			}
			decode(event.Data, &data)
			switch {
			case event.Name == "failed":
				w.Says, w.Note = "failed", errorOf(event.Data)
			case w.Kind == "person":
				w.Name, w.Says = data.By, data.Answer
			case data.Reply != nil:
				w.Says, w.Note = data.Reply.Says, data.Reply.Note
			default:
				w.Says, w.Note = data.Says, data.Note
			}
			said = append(said, w)
		}
	}
	return said
}

// conversation gives the conversation of an agent, from the record of its
// run among the entries of a record: what each turn was asked and what
// the model said, and each tool it called with what the tool gave back.
// A call whose job is a colleague that is an agent holds the colleague's
// calls.
func conversation(entries map[string][]client.Entry, run string) ([]Turn, []Call) {
	var turns []Turn
	var calls []Call
	called := map[string]int{} // each call by its id
	for _, entry := range entries[run] {
		for _, ask := range entry.Asks {
			if ask.Kind != "script" || ask.Job == nil || ask.Job.Name != "turn" {
				continue
			}
			var in struct {
				Add []struct {
					Role    string `json:"role"`
					Content any    `json:"content"`
					Results []struct {
						ID     string `json:"id"`
						Result any    `json:"result"`
						Error  string `json:"error"`
					} `json:"results"`
				} `json:"add"`
			}
			decode(ask.Job.Input, &in)
			for _, add := range in.Add {
				if add.Role == "user" {
					turns = append(turns, Turn{Role: "user", Text: text(add.Content)})
				}
				for _, result := range add.Results {
					if i, ok := called[result.ID]; ok {
						calls[i].Result, calls[i].Error = result.Result, result.Error
					}
				}
			}
		}
		for _, event := range entry.Events {
			if event.From != "turn" || event.Name != "done" {
				continue
			}
			var out struct {
				Text  string `json:"text"`
				Calls []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					Args any    `json:"args"`
				} `json:"calls"`
			}
			decode(event.Data, &out)
			if out.Text != "" {
				turns = append(turns, Turn{Role: "model", Text: out.Text})
			}
			// An agent names each call's job after its tool, with its place
			// for a tool called again in one turn: fixer, fixer.1.
			seen := map[string]int{}
			for _, c := range out.Calls {
				seen[c.Name]++
				job := c.Name
				if seen[c.Name] > 1 {
					job = fmt.Sprintf("%s.%d", c.Name, seen[c.Name]-1)
				}
				call := Call{Tool: c.Name, Args: c.Args, At: entry.Time}
				if child := colleague(entries[run], entry, job); child != "" {
					_, call.Calls = conversation(entries, child)
				}
				called[c.ID] = len(calls)
				calls = append(calls, call)
			}
		}
	}
	return turns, calls
}

// colleague gives the run a call's job started, when the job is a
// machine; "" otherwise. The job is asked for in the step the call's turn
// ended in, and started then, or once the ask approves it. Corbie tells
// apart the jobs asked under one name by adding the times the name asked
// before (fixer#2) and the attempt (fixer@2).
func colleague(entries []client.Entry, turn client.Entry, job string) string {
	asked := ""
	for _, ask := range turn.Asks {
		if ask.Job != nil && ask.Job.Name == job {
			asked, _, _ = strings.Cut(ask.Name, "@")
		}
	}
	if asked == "" {
		return ""
	}
	child := ""
	for _, entry := range entries {
		for _, ask := range entry.Asks {
			if name, _, _ := strings.Cut(ask.Name, "@"); ask.Kind == "child" && name == asked {
				child = ask.Child
			}
		}
	}
	return child
}

// text gives a part of a conversation as text: itself when it is text,
// and otherwise as JSON.
func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}
