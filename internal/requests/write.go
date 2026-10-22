package requests

import (
	"fmt"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// wait is the path of the shipped wait.yaml from the office's top.
const wait = "./.bulldog/wait/wait.yaml"

// write gives a team's request machine as it is sent to Corbie.
func write(o *office.Office, t *office.Team) mapping {
	stages := mapping{}
	failed := []string{}
	outcome, answer := "", "" // what each decision adds to the output
	if t.Heed != nil {
		stages = append(stages, entry{"heed", mapping{
			{"when", "${input.request.heard == 'channel'}"},
			{"job", mapping{
				{"input", inline{{"request", "${input.request}"}, {"said", "${input.said}"}}},
				{"machine", heed(t).chart(o)},
			}},
		}})
		failed = append(failed, "stages.heed.failed")
		outcome += "stages.heed.output?.says == 'no' ? 'noted' : "
		answer += "stages.heed.output?.says == 'no' ? null : "
	}
	work := mapping{}
	if t.Heed != nil {
		work = append(work, entry{"when", "${stages.heed.skipped || stages.heed.output.says == 'yes'}"})
	}
	job, reply := lead(o, t)
	stages = append(stages, entry{"work", append(work, entry{"job", job})})
	failed = append(failed, "stages.work.failed")
	if t.Check != nil {
		stages = append(stages, entry{"check", mapping{
			{"when", fmt.Sprintf("${%s != '' && input.request.heard != 'routine' && !%s.includes(input.request.by?.person)}", reply, literals(t.People))},
			{"job", mapping{
				{"input", inline{{"request", "${input.request}"}, {"answer", "${" + reply + "}"}}},
				{"machine", check(t).chart(o)},
			}},
		}})
		failed = append(failed, "stages.check.failed")
		outcome += "stages.check.output?.says == 'no' ? 'withheld' : "
		answer += fmt.Sprintf("stages.check.output?.says == 'no' ? %s : ", literal(t.Name+" did not let its answer out."))
	}
	anyFailed := strings.Join(failed, " || ")
	return mapping{
		{"flow", t.ID},
		{"keep", "30d"},
		{"input", inline{{"said", "string"}, {"context", "string"}, {"request", "map"}, {"files", "list?"}}},
		{"ask", mapping{
			{"input", inline{{"request", "${input.request}"}}},
			{"machine", approval(o, t)},
		}},
		{"stages", stages},
		{"output", mapping{
			{"outcome", fmt.Sprintf("${%s ? 'failed' : %s'answered'}", anyFailed, outcome)},
			{"answer", fmt.Sprintf("${%s ? %s : %s%s}", anyFailed, literal(t.Name+" could not answer: its work failed."), answer, reply)},
		}},
	}
}

// lead gives the work stage's job, the team's lead, and an expression of
// its answer as text: an agent's reply, a person's answer to wait.yaml, or
// a flow's or a statechart's output, each the text or its text answer.
// Anything else is the empty text.
func lead(o *office.Office, t *office.Team) (job mapping, reply string) {
	given := "stages.work.output"
	if t.Lead.Person != "" {
		job = mapping{
			{"machine", wait},
			{"input", inline{{"to", inline{{"people", []any{t.Lead.Person}}}}, {"said", "${input.said}"}}},
		}
	} else {
		w := o.Worker(t.Lead.Worker)
		if w.Type == "agent" {
			given = "stages.work.output?.reply"
		}
		job = mapping{
			{"machine", "./" + w.File},
			{"input", inline{{"said", "${input.said}"}, {"context", "${input.context}"}, {"request", "${input.request}"}, {"files", "${input.files ?? []}"}}},
		}
	}
	reply = fmt.Sprintf("(typeof %[1]s == 'string' ? %[1]s : typeof %[1]s?.answer == 'string' ? %[1]s.answer : '')", given)
	return job, reply
}

// asksTeam is an expression of whether a held job j asks another team: its
// input names the team, as ask_team's does.
const asksTeam = "j.input?.to?.team"

// approval writes the request machine's ask: a flow given the jobs Corbie
// holds in one step. A held job that asks a team goes to the check
// decision, on its question, and is approved at once without a check;
// every other goes to the decide decision. The flow approves the jobs of
// each part that said yes.
func approval(o *office.Office, t *office.Team) mapping {
	stages := mapping{}
	approved := fmt.Sprintf("input.jobs.filter(j => %s).map(j => j.name)", asksTeam)
	note := "stages.decide.output?.says == 'no' ? stages.decide.output.note : null"
	if t.Check != nil {
		stages = append(stages, entry{"check", mapping{
			{"each", fmt.Sprintf("${input.jobs.filter(j => %s)}", asksTeam)},
			{"job", mapping{
				{"input", inline{{"request", "${input.request}"}, {"asking", "${input.asking}"},
					{"question", inline{{"team", "${item.input.to.team}"}, {"said", "${item.input.said}"}}}}},
				{"machine", question(t).chart(o)},
			}},
		}})
		approved = "stages.check.output.filter(c => c.output?.says == 'yes').map(c => c.item.name)"
		note = fmt.Sprintf("[stages.check.output.some(c => c.output?.says != 'yes') ? %s : null, %s].filter(n => n).join(' ') || null",
			literal(t.Name+" did not let its question out."), note)
	}
	stages = append(stages, entry{"decide", mapping{
		{"when", fmt.Sprintf("${input.jobs.some(j => !%s)}", asksTeam)},
		{"job", mapping{
			{"input", inline{{"request", "${input.request}"}, {"asking", "${input.asking}"},
				{"jobs", fmt.Sprintf("${input.jobs.filter(j => !%s)}", asksTeam)}}},
			{"machine", decide(t).chart(o)},
		}},
	}})
	return mapping{
		{"flow", t.ID + "-approval"},
		{"input", inline{{"request", "map"}, {"asking", "map"}, {"jobs", "list"}}},
		{"stages", stages},
		{"output", mapping{
			{"approve", fmt.Sprintf("${[...%s, ...(stages.decide.output?.says == 'yes' ? input.jobs.filter(j => !%s).map(j => j.name) : [])]}", approved, asksTeam)},
			{"note", "${" + note + "}"},
		}},
	}
}

// A decision is one of a team's decisions, as its chart is written.
type decision struct {
	name     string // heed, check or decide, as the people's wait is marked
	named    string // the chart's name, such as payments-check
	deciders []office.Decider
	input    inline // the chart's input
	given    inline // what each decider is given, but the notes
	// asked is what the people are asked, text with expressions in it,
	// above the notes.
	asked string
	// own says a worker passes on a job its own machine holds: the chart
	// is given asking, the machine that holds the jobs.
	own bool
}

func heed(t *office.Team) decision {
	return decision{
		name: "heed", named: t.ID + "-heed", deciders: t.Heed,
		input: inline{{"request", "map"}, {"said", "string"}},
		given: inline{{"request", "${input.request}"}, {"said", "${input.said}"}},
		asked: "Is this for " + t.Name + "?\n\n${input.said}",
	}
}

func check(t *office.Team) decision {
	return decision{
		name: "check", named: t.ID + "-check", deciders: t.Check,
		input: inline{{"request", "map"}, {"answer", "string"}},
		given: inline{{"request", "${input.request}"}, {"answer", "${input.answer}"}},
		asked: "May " + t.Name + " give this answer?\n\n${input.answer}",
	}
}

// question is the check decision on a question the team asks another team.
func question(t *office.Team) decision {
	return decision{
		name: "check", named: t.ID + "-question", deciders: t.Check, own: true,
		input: inline{{"request", "map"}, {"asking", "map"}, {"question", "map"}},
		given: inline{{"request", "${input.request}"}, {"question", "${input.question}"}},
		asked: "May " + t.Name + " ask ${input.question.team} this?\n\n${input.question.said}",
	}
}

// decide is the decide decision: the team file's, or its people's without
// one.
func decide(t *office.Team) decision {
	deciders := t.Decide
	if deciders == nil {
		for _, person := range t.People {
			deciders = append(deciders, office.Decider{Person: person})
		}
	}
	return decision{
		name: "decide", named: t.ID + "-decide", deciders: deciders, own: true,
		input: inline{{"request", "map"}, {"asking", "map"}, {"jobs", "list"}},
		given: inline{{"request", "${input.request}"}, {"jobs", "${input.jobs}"}},
		asked: "May these run?\n\n${input.jobs.map(j => j.name + ': ' + JSON.stringify(j.asked?.args ?? j.input)).join(`\n`)}",
	}
}

// chart writes a decision as a statechart: one state for each decider
// before the people, d1, d2 and on, then people, then the final states
// said_yes, said_no and passed. The first yes or no is the answer, and when
// every decider passes, the answer is no. A decider that fails fails the
// chart. Each decider is given the notes of those before it, and the people
// are shown them above yes and no.
func (d decision) chart(o *office.Office) mapping {
	var before []office.Decider // the deciders before the people
	var people []any
	for _, decider := range d.deciders {
		if decider.Person != "" {
			people = append(people, decider.Person)
		} else {
			before = append(before, decider)
		}
	}
	after := func(i int) string {
		switch {
		case i+1 < len(before):
			return state(i + 1)
		case len(people) > 0:
			return "people"
		}
		return "passed"
	}
	states := mapping{}
	for i, decider := range before {
		// A rule is a script, or a definition Bulldog ships; a worker is a
		// machine, whose reply holds what it says.
		says, job, path := "event.data?.says", "script", decider.Rule
		switch {
		case decider.Worker != "":
			says, job, path = "event.data?.reply?.says", "machine", "./"+o.Worker(decider.Worker).File
		case strings.HasSuffix(decider.Rule, ".yaml"):
			job = "machine"
		}
		var on []any
		if d.own && decider.Worker != "" {
			on = append(on, inline{{"go", after(i)}, {"when", "${" + heldBy(decider.Worker) + "}"}})
		}
		on = append(on,
			inline{{"go", "said_yes"}, {"when", "${" + says + " == 'yes'}"}},
			inline{{"go", "said_no"}, {"when", "${" + says + " == 'no'}"}},
			after(i))
		states = append(states, entry{state(i), mapping{
			{"job", mapping{
				{job, path},
				{"input", append(append(mapping{}, d.given...), entry{"notes", d.notes(before[:i])})},
			}},
			{"on", mapping{{"done", on}}},
		}})
	}
	if len(people) > 0 {
		states = append(states, entry{"people", mapping{
			{"job", mapping{
				{"machine", wait},
				{"input", mapping{
					{"to", inline{{"people", people}}},
					{"said", d.asked + d.noted(before)},
					{"choices", list{"yes", "no"}},
					{"decision", d.name},
				}},
			}},
			{"on", mapping{{"done", []any{inline{{"go", "said_yes"}, {"when", "${event.data.answer == 'yes'}"}}, "said_no"}}}},
		}})
	}
	initial := "passed"
	switch {
	case len(before) > 0:
		initial = state(0)
	case len(people) > 0:
		initial = "people"
	}
	note := "${event.data.reply?.note ?? event.data.note ?? null}"
	if len(d.deciders) > 0 {
		states = append(states,
			entry{"said_yes", inline{{"end", "done"}, {"output", inline{{"says", "yes"}, {"note", note}}}}},
			entry{"said_no", inline{{"end", "done"}, {"output", inline{{"says", "no"}, {"note", note}}}}})
	}
	if len(people) == 0 {
		states = append(states, entry{"passed", inline{{"end", "done"}, {"output", inline{{"says", "no"}, {"note", "Every decider passed."}}}}})
	}
	return mapping{{"statechart", d.named}, {"input", d.input}, {"initial", initial}, {"states", states}}
}

// notes gives what a decider is given of the deciders before it, each of
// which passed: its kind, its name, and its note.
func (d decision) notes(before []office.Decider) list {
	notes := list{}
	for i, decider := range before {
		kind, name := who(decider)
		notes = append(notes, inline{{"kind", kind}, {"name", name}, {"says", "pass"}, {"note", "${" + d.note(i, decider) + " ?? null}"}})
	}
	return notes
}

// noted gives the notes of the deciders before the people as text, a line
// for each, with expressions in it.
func (d decision) noted(before []office.Decider) string {
	if len(before) == 0 {
		return ""
	}
	lines := make([]string, len(before))
	for i, decider := range before {
		_, name := who(decider)
		note := d.note(i, decider)
		lines[i] = fmt.Sprintf("%s passed${%s ? ': ' + %s : '.'}", name, note, note)
	}
	return "\n\n" + strings.Join(lines, "\n")
}

// note is an expression of the note a decider gave as it passed, by its
// place in the list. A worker passes on a job its own machine holds, with
// a note that says so.
func (d decision) note(i int, decider office.Decider) string {
	if decider.Worker == "" {
		return fmt.Sprintf("states.%s.output?.note", state(i))
	}
	note := fmt.Sprintf("states.%s.output?.reply?.note", state(i))
	if d.own {
		note = fmt.Sprintf("(%s ? 'The held job is its own.' : %s)", heldBy(decider.Worker), note)
	}
	return note
}

// heldBy is an expression of whether the jobs a decision is asked about
// are held by a worker's own machine.
func heldBy(worker string) string { return "input.asking.machine == " + literal(worker) }

// who gives a decider's kind, rule or worker, and its name: a rule's path
// from the office's top, or a worker's name.
func who(decider office.Decider) (kind, name string) {
	if decider.Worker != "" {
		return "worker", decider.Worker
	}
	return "rule", decider.Rule
}

// state names the state of the decider at a place in a list: d1, d2 and on.
func state(i int) string { return fmt.Sprintf("d%d", i+1) }
