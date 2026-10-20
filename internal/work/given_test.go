package work

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// prompt gives the first prompt the stand-in was shown that holds a word.
func (b *bench) prompt(word string) string {
	b.t.Helper()
	var prompt string
	b.until("a prompt holds "+word, func() bool {
		for _, turn := range b.model.Turns() {
			if turn.Number == 1 && strings.Contains(turn.Prompt(), word) {
				prompt = turn.Prompt()
				return true
			}
		}
		return false
	})
	return prompt
}

// newFor gives what a run's context says is new for it.
func newFor(context string) string {
	_, fresh, _ := strings.Cut(context, "\nNew for you:\n")
	fresh, _, _ = strings.Cut(fresh, "\n\nYour last reply")
	return fresh
}

// holds fails the test for each part a text does not hold.
func holds(t *testing.T, what, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%s does not hold %q:\n%s", what, part, text)
		}
	}
}

func holdsNone(t *testing.T, what, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if strings.Contains(text, part) {
			t.Errorf("%s holds %q:\n%s", what, part, text)
		}
	}
}

// echo is a desk team whose lead is a flow that answers with the
// request it is given, as data, and a note.
func echo(files map[string]string) map[string]string {
	files["desk/team.yaml"] = "name: The desk\nabout: Papers.\npeople: [sam]\nlead: echo\n"
	files["desk/echo.yaml"] = "flow: echo\ninput: { request: map }\nstages:\n  a: { job: { script: ./echo.js, input: { request: \"${input.request}\" } } }\noutput: ${stages.a.output}\n"
	files["desk/echo.js"] = "return input.request\n"
	files["desk/notes/stamps.md"] = "First class only.\n"
	return files
}

func TestARunIsGivenItsDefinitionAsWritten(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "lead: gardener", "lead: groundsman", 1)
	b := newBench(t, files)
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	b.settled(n, requests.Answered)
	def, err := b.corbie.Definition(ctx, run.Name)
	if err != nil {
		t.Fatal(err)
	}
	body := string(def.Body)
	if def.Name != "steward" || !strings.Contains(body, b.model.URL) || def.Ask == nil {
		t.Errorf("the steward's run was started from %s %s, asking %v", def.Name, body, def.Ask)
	}
	in := b.input(run.Name)
	for _, key := range []string{"said", "context", "request"} {
		if in[key] == nil {
			t.Errorf("the run is not given %s", key)
		}
	}
	// The groundsman reads only said, and runs.
	g := b.chat("sam", "garden", "mow the lawn")
	b.started(g, 1)
	b.settled(g, requests.Answered)
	if got := b.runs(g)[0]; got.Status != done {
		t.Errorf("the groundsman's run ended %s: %s", got.Status, got.How)
	}
	// A model written out in full is sent as it is written.
	written := fmt.Sprintf(`{ name: claude-written, service: anthropic, address: "%s", secret: %s }`, b.model.URL, testkit.Key)
	b.rewrite(map[string]string{"garden/groundsman.yaml": strings.Replace(files["garden/groundsman.yaml"], "model: usual", "model: "+written, 1)})
	b.say(g, "sam", "and the hedge")
	run = b.started(g, 2)
	b.settled(g, requests.Answered)
	if def, err := b.corbie.Definition(ctx, run.Name); err != nil || !strings.Contains(string(def.Body), "claude-written") || strings.Contains(string(def.Body), "claude-test") {
		t.Errorf("the groundsman's model written in full was sent as %s (%v)", def.Body, err)
	}
}

func TestTheContextSaysWhatTheRunIsToKnow(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	prompt := b.prompt("the kitchen socket sparked")
	holds(t, "the steward's context", prompt, "Monday 5 October 2026, 10:00 BST", "Europe/London", "The house",
		"Sam: Owner. Decides money and the building.", "Kit: Lives here.", "fixer", "sparks", "clerk", "bins", "builder",
		"garden (The garden): The beds, the lawn and the shed.", "Asked by Kit, a person of the office, who is in The house",
		"in the built-in chat", "Who will read the answer: Kit", "the kitchen socket sparked",
		"Your last reply is your answer, said to Kit in the built-in chat.")
	holdsNone(t, "the steward's context", prompt, "gardener", "groundsman")
	b.settled(n, requests.Answered)

	files := house(t)
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "about: The beds, the lawn and the shed.",
		"about: The beds, the lawn, the shed and the pond.", 1)
	b.rewrite(files)
	b.chat("kit", "", "is the pond frozen?")
	holds(t, "the steward's next context", b.prompt("is the pond frozen?"), "The beds, the lawn, the shed and the pond.")
}

func TestAGuestIsNamedByTheIdentityVouchedFor(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	if _, err := b.reqs.AskAt("slack", "#help", "#help/1700.3", "", "U0GUEST1", "is the API down?", nil, false); err != nil {
		t.Fatal(err)
	}
	holds(t, "the bot's context", b.prompt("is the API down?"), "Asked by a guest, whom slack vouches for as U0GUEST1")
}

func TestTheRequestAsData(t *testing.T) {
	t.Parallel()
	b := newBench(t, echo(house(t)))
	n := b.chat("sam", "desk", "file the bill")
	b.settled(n, requests.Answered)
	// The garden is held on the sub-request, so its answer comes after
	// what Sam says, and is not what the desk's next run is started on.
	b.hold("shed", testkit.Say("locked"))
	if _, err := b.reqs.Sub(n, b.reqs.Worker("echo"), "garden", "is the shed locked?", "ticket-1"); err != nil {
		t.Fatal(err)
	}
	b.say(n, "sam", "and the receipt")
	b.started(n, 2)
	b.settled(n, requests.Answered)
	answers := b.entries(n, requests.KindAnswer)
	var d map[string]any
	if err := json.Unmarshal([]byte(answers[len(answers)-1].Text), &d); err != nil {
		t.Fatalf("the answer is not the request written out: %v\n%s", err, answers[len(answers)-1].Text)
	}
	if d["number"] != float64(n) || d["said"] != "and the receipt" {
		t.Errorf("the request's number and said are %v and %v", d["number"], d["said"])
	}
	asker, _ := d["asker"].(map[string]any)
	if asker["kind"] != "person" || asker["name"] != "sam" {
		t.Errorf("the asker is %v", asker)
	}
	if origin, _ := d["origin"].(map[string]any); origin["kind"] != requests.InChat {
		t.Errorf("the origin is %v", origin)
	}
	if readers, _ := d["readers"].([]any); len(readers) != 1 || readers[0].(map[string]any)["name"] != "sam" {
		t.Errorf("the readers are %v", readers)
	}
	entries, _ := d["entries"].([]any)
	fresh, _ := d["new"].([]any)
	if len(entries) != 3 || len(fresh) != 1 || fresh[0].(map[string]any)["text"] != "and the receipt" {
		t.Fatalf("the entries are %v, and those new %v", entries, fresh)
	}
	first := entries[0].(map[string]any)
	if first["kind"] != requests.KindAsked || first["text"] != "file the bill" || first["by"].(map[string]any)["name"] != "sam" ||
		first["to"].([]any)[0].(map[string]any)["name"] != "desk" || first["time"] != "2026-10-05T10:00:00+01:00" {
		t.Errorf("the first entry is %v", first)
	}
	subs, _ := d["subs"].([]any)
	if len(subs) != 1 || subs[0].(map[string]any)["team"] != "garden" || subs[0].(map[string]any)["state"] == "" {
		t.Errorf("the sub-requests are %v", subs)
	}
	if notes, _ := d["notes"].([]any); len(notes) != 1 || notes[0].(map[string]any)["text"] != "First class only.\n" {
		t.Errorf("the notes are %v", notes)
	}
}

func TestWorkWorkerThatIsNoModelGivenData(t *testing.T) {
	t.Parallel()
	b := newBench(t, echo(house(t)))
	n := b.chat("sam", "desk", "file the bill")
	run := b.started(n, 1)
	b.settled(n, requests.Answered)
	in := b.input(run.Name)
	context, _ := in["context"].(string)
	d, _ := in["request"].(map[string]any)
	text, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	// What the context says in words, the request holds as data.
	for _, part := range []string{"Sam", "Owner. Decides money and the building.", "The garden", "The beds, the lawn and the shed.",
		"First class only.", "file the bill", "Europe/London", "Your last reply is your answer, said to Sam in the built-in chat."} {
		if strings.Contains(context, part) && !strings.Contains(string(text), part) {
			t.Errorf("the context says %q, and the request as data does not", part)
		}
	}
}

func TestWorkSubRequestGivenItsOwnThreadOnly(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	_, let := b.hold("PARENTWORD", testkit.Say("asked the garden"))
	n := b.chat("kit", "", "the shed door PARENTWORD is stuck")
	b.started(n, 1)
	sub, err := b.reqs.Sub(n, b.reqs.Worker("steward"), "garden", "is the shed door stuck?", "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	let()
	run := b.started(sub.Number, 1)
	in := b.input(run.Name)
	text := fmt.Sprint(in)
	holdsNone(t, "the gardener's input", text, "PARENTWORD")
	context, _ := in["context"].(string)
	holds(t, "the gardener's context", context, "Asked by steward, a worker of The house.", "is the shed door stuck?",
		"in the built-in chat, by Kit")
}

func TestWorkRunToldTrulyWhereItsAnswerGoes(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], "people: [sam]", "people: [sam, kit]", 1)
	b := newBench(t, files)
	n := b.chat("kit", "", "the kitchen socket sparked")
	holds(t, "a request's run", b.prompt("socket sparked"), "Your last reply is your answer, said to Kit in the built-in chat.")
	holdsNone(t, "a request's run", b.prompt("socket sparked"), "An empty reply says nothing.")
	b.settled(n, requests.Answered)
	if _, err := b.reqs.Sub(n, b.reqs.Worker("steward"), "garden", "is the shed locked?", "ticket-1"); err != nil {
		t.Fatal(err)
	}
	holds(t, "a sub-request's run", b.prompt("is the shed locked?"), "Your last reply is your answer, given to steward of The house.")
	routine := b.office().Teams["garden"].Routines[0]
	if _, err := b.reqs.AskByRoutine("garden", routine, requests.Who{}); err != nil {
		t.Fatal(err)
	}
	holds(t, "a routine's run", b.prompt(routine.Say), "Your last reply is your answer, said to the people of The garden: Sam, Kit. An empty reply says nothing.")

	// A guest asks payments in #refunds, and its rule refuses every
	// answer: the ledger, started again on the refusal, is not told that
	// an empty reply says nothing, for its empty reply is not disclosed.
	files = payments(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`,
		`"#help": { guests: true }`+"\n      \"#refunds\": { team: payments, guests: true }", 1)
	files["payments/team.yaml"] = strings.Replace(files["payments/team.yaml"], "disclosure: [./no-cards.js]", "disclosure: [./refuse-all.js]", 1)
	files["payments/refuse-all.js"] = "return { says: \"refuse\", note: \"never the card ending 1111\" }\n"
	b = newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		if strings.Contains(turn.Prompt(), "never the card ending 1111") {
			return testkit.Say("")
		}
		return testkit.Say("Duplicate on the card ending 1111.")
	})
	refunds := must(b.reqs.AskAt("slack", "#refunds", "#refunds/1700.2", "", "U0GUEST1", "was 4411 charged twice?", nil, false)).Number
	again := b.prompt("never the card ending 1111")
	holds(t, "a run started again on a refusal", again, "Your last reply is your answer, said to a guest (U0GUEST1 on slack) on slack, in #refunds/1700.2.")
	holds(t, "a run where nobody reported who reads", again, "Who will read the answer: a guest (U0GUEST1 on slack), and any guest there.")
	holdsNone(t, "a run started again on a refusal", again, "An empty reply says nothing.")
	b.settled(refunds, requests.Answered)
	if told := b.toldAsker(refunds); !slices.Equal(told, []string{"payments did not disclose its answer (refuse-all.js)"}) {
		t.Errorf("Bulldog told the guest %q", told)
	}
}

func TestWorkTimesInTheOfficeZone(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.later(2 * time.Hour)
	n := b.chat("kit", "", "the kitchen socket sparked")
	run := b.started(n, 1)
	in := b.input(run.Name)
	context, _ := in["context"].(string)
	holds(t, "the context", context, "Monday 5 October 2026, 12:00 BST, in the office's time zone, Europe/London",
		"[Mon 5 Oct 2026 12:00 BST] Kit to The house: the kitchen socket sparked")
	d, _ := in["request"].(map[string]any)
	entry := d["new"].([]any)[0].(map[string]any)
	if d["now"] != "2026-10-05T12:00:00+01:00" || d["zone"] != "Europe/London" || entry["time"] != "2026-10-05T12:00:00+01:00" {
		t.Errorf("the request's times are %v, %v and %v", d["now"], d["zone"], entry["time"])
	}
}

func TestTheThreadIsCutToItsNewest(t *testing.T) {
	t.Parallel()
	var thread []requests.Entry
	for i := range 40 {
		thread = append(thread, requests.Entry{ID: int64(i + 1), Text: fmt.Sprintf("%03d", i) + strings.Repeat("x", 997)})
	}
	text := func(e requests.Entry) string { return e.Text }
	shown, left := window(thread, text)
	if shown[0].ID != 1 || shown[len(shown)-1].ID != 40 || left != 40-len(shown) {
		t.Errorf("the window shows %d entries from %d to %d, and leaves %d", len(shown), shown[0].ID, shown[len(shown)-1].ID, left)
	}
	total := 0
	for i, e := range shown {
		total += len(e.Text)
		if i > 1 && e.ID != shown[i-1].ID+1 {
			t.Errorf("the newest entries shown are not one after another: %d after %d", e.ID, shown[i-1].ID)
		}
	}
	if total > ThreadLimit || total < ThreadLimit-1000 {
		t.Errorf("the entries shown hold %d characters", total)
	}

	b := newBench(t, house(t))
	_, let := b.hold("first words", testkit.Say("on it"))
	n := b.chat("kit", "", "first words "+strings.Repeat("y", 988))
	b.started(n, 1)
	for i := range 39 {
		b.say(n, "kit", fmt.Sprintf("%03d", i)+strings.Repeat("z", 997))
	}
	let()
	b.started(n, 2)
	b.settled(n, requests.Answered)
	b.say(n, "kit", "last")
	run := b.started(n, 3)
	b.settled(n, requests.Answered)
	context := b.input(run.Name)["context"].(string)
	thread2 := context[strings.Index(context, "The thread so far:"):strings.Index(context, "New for you:")]
	holds(t, "the thread given", thread2, "first words", "entries are not shown here.", "Kit to The house: 038z")
	holds(t, "what is new", newFor(context), "Kit to The house: last")
	if utf8.RuneCountInString(thread2) > 21_000 {
		t.Errorf("the thread given holds %d characters", utf8.RuneCountInString(thread2))
	}
}

func TestWorkNewEntriesShownWithWhoSaidThem(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "hello")
	run := b.started(n, 1)
	b.settled(n, requests.Answered)
	context := b.input(run.Name)["context"].(string)
	holds(t, "the first run's context", context, "\nNew for you:\n[Mon 5 Oct 2026 10:00 BST] Kit to The house: hello\n\nYour last reply")
	holdsNone(t, "the first run's context", context, "The thread so far:")

	long := "the log: " + strings.Repeat("x", ThreadLimit+500) + " END"
	b.say(n, "sam", long)
	run = b.started(n, 2)
	b.settled(n, requests.Answered)
	in := b.input(run.Name)
	context = in["context"].(string)
	thread := context[strings.Index(context, "The thread so far:"):strings.Index(context, "New for you:")]
	holds(t, "the thread given", thread, "Kit to The house: hello")
	holdsNone(t, "the thread given", thread, "the log: ")
	if fresh := newFor(context); fresh != "[Mon 5 Oct 2026 10:00 BST] Sam to The house: "+long {
		t.Errorf("what is new is given as %.200q", fresh)
	}
	d := in["request"].(map[string]any)
	fresh, _ := d["new"].([]any)
	if len(fresh) != 1 || fresh[0].(map[string]any)["text"] != long || fresh[0].(map[string]any)["by"].(map[string]any)["name"] != "sam" {
		t.Errorf("what is new is given as data as %.200v", fresh)
	}
	for _, e := range d["entries"].([]any) {
		if e.(map[string]any)["text"] == long {
			t.Errorf("what is new is among the entries of the thread")
		}
	}
}

func TestWorkTextNeverCutInsideACharacter(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("é€😀", 10)
	for i := range 31 {
		if got := cut(text, i); !utf8.ValidString(got) || utf8.RuneCountInString(got) != i {
			t.Errorf("cut to %d gives %q", i, got)
		}
	}
	notes, left := shownNotes([]office.Note{{Name: "a", Text: strings.Repeat("é", NotesLimit-1) + "😀😀"}})
	if !utf8.ValidString(notes[0].Text) || utf8.RuneCountInString(notes[0].Text) != NotesLimit || left == "" {
		t.Errorf("the notes are cut to %d characters, valid %v, saying %q", utf8.RuneCountInString(notes[0].Text), utf8.ValidString(notes[0].Text), left)
	}
	long := []requests.Entry{{ID: 1, Text: strings.Repeat("😀", ThreadLimit+10)}}
	shown, _ := window(long, func(e requests.Entry) string { return e.Text })
	if !utf8.ValidString(shown[0].Text) || utf8.RuneCountInString(shown[0].Text) != ThreadLimit {
		t.Errorf("an opening entry past the limit is cut to %d characters", utf8.RuneCountInString(shown[0].Text))
	}
}

func TestEveryRunOfATeamIsGivenItsNotes(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	g := b.chat("sam", "garden", "how is the lawn?")
	prompt := b.prompt("how is the lawn?")
	notes := b.office()
	garden, err := notes.Notes("garden")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range garden {
		holds(t, "the gardener's context", prompt, "## "+n.Name+"\n"+strings.TrimRight(n.Text, "\n"))
	}
	holdsNone(t, "the gardener's context", prompt, "The fuse board is under the stairs.")
	b.settled(g, requests.Answered)

	// Bins names no notes tool, and is given the house's notes on its
	// routine.
	var bins office.Routine
	for _, r := range b.office().TopTeam().Routines {
		if r.To == "bins" {
			bins = r
		}
	}
	r, err := b.reqs.AskByRoutine("office", bins, requests.Who{})
	if err != nil {
		t.Fatal(err)
	}
	run := b.started(r.Number, 1)
	d := b.input(run.Name)["request"].(map[string]any)
	if notes, _ := d["notes"].([]any); len(notes) != 1 || notes[0].(map[string]any)["name"] != "rooms" {
		t.Errorf("bins is given the notes %v", d["notes"])
	}
	b.settled(r.Number, requests.Answered)

	// Notes past the limit are given as their first part, with a line.
	b.rewrite(map[string]string{"garden/notes/lawn.md": strings.Repeat("l", 40_000), "garden/notes/shed.md": strings.Repeat("s", 20_000)})
	b.chat("sam", "garden", "and the shed?")
	prompt = b.prompt("and the shed?")
	holds(t, "the gardener's context", prompt, strings.Repeat("s", 10_000),
		"Not shown, past the 50000 characters of notes a run is given: the last 10000 of the 20000 characters of shed.")
	holdsNone(t, "the gardener's context", prompt, strings.Repeat("s", 10_001))
}

func TestARoutineIsGivenItsTeamsOpenRequests(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "lead: steward", "lead: sam", 1)
	b := newBench(t, files)
	first := b.chat("kit", "", "the kitchen socket sparked")
	b.chat("sam", "garden", "how is the lawn?")
	routine := b.office().TopTeam().Routines[0]
	r, err := b.reqs.AskByRoutine("office", routine, requests.Who{})
	if err != nil {
		t.Fatal(err)
	}
	run := b.started(r.Number, 1)
	in := b.input(run.Name)
	holds(t, "the routine's context", in["context"].(string),
		fmt.Sprintf("- Request %d, asked by Kit, waiting on Sam: the kitchen socket sparked", first))
	holdsNone(t, "the routine's context", in["context"].(string), "how is the lawn?")
	open := in["request"].(map[string]any)["open"].([]any)
	if len(open) != 1 || open[0].(map[string]any)["number"] != float64(first) || open[0].(map[string]any)["first"] != "the kitchen socket sparked" {
		t.Errorf("the routine's request holds %v", open)
	}
	b.settled(r.Number, requests.Answered)

	for i := range 52 {
		b.chat("kit", "", fmt.Sprint("job ", i))
	}
	r, err = b.reqs.AskByRoutine("office", routine, requests.Who{})
	if err != nil {
		t.Fatal(err)
	}
	run = b.started(r.Number, 1)
	in = b.input(run.Name)
	holds(t, "the routine's context", in["context"].(string), "job 51", "3 more are open.")
	holdsNone(t, "the routine's context", in["context"].(string), "the kitchen socket sparked", ": job 0\n", ": job 1\n")
	d := in["request"].(map[string]any)
	if len(d["open"].([]any)) != OpenLimit || d["open_not_shown"] != float64(3) {
		t.Errorf("the routine's request holds %d open and %v more", len(d["open"].([]any)), d["open_not_shown"])
	}
}
