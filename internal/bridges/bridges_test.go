package bridges

import (
	"fmt"
	"io"
	"maps"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
)

// What is said at a listed place starts a request of the place's team, by
// the person whose entry gives the sender's identity, or by a guest where
// the place lets guests ask. What is said at a place the settings do not
// list, by a guest where guests may not ask, or more than a day before it
// is read starts nothing. A text too long is cut, saying so.
func TestBridgesTakeWhatIsSaid(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	old := said("C-help/0", "u-guest", "#help", "C-help/0", "An old question.")
	old["at"] = w.corbie.Clock.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	roses := said("C-garden/1", "u-kit", "#garden", "C-garden/1/t", "The roses need water.")
	roses["files"] = []any{map[string]any{"name": "roses.txt", "text": "Pink ones."}}
	roses["mentioned"] = true
	weeds := said("C-garden/2", "u-kit", "#garden", "C-garden/2", strings.Repeat("Weeds. ", 6000))
	w.say("pager",
		old,
		said("C-random/1", "u-guest", "#random", "C-random/1", "Is anyone in #random?"),
		said("C-garden/0", "u-guest", "#garden", "C-garden/0", "May a guest ask the garden?"),
		roses,
		weeds,
		said("C-help/1", "u-guest", "#help", "C-help/1", "Is the boiler on?"),
		said("u-sam:1", "u-sam", "direct", "u-sam", "Turn the heating on."),
	)
	// Things are taken in order, and Corbie has taken every request
	// started before the last one once that one's question is carried.
	heating := w.request("Turn the heating on.")
	w.question("Turn the heating on.")
	boiler := w.request("Is the boiler on?")
	if n := len(w.requests()); n != 4 {
		t.Errorf("%d requests started, not 4", n)
	}

	for run, want := range map[string]string{
		boiler.ID:  "map[by:map[guest:u-guest] heard:channel mentioned:false place:map[bridge:pager guests:true place:#help thing:C-help/1 where:C-help/1] team:office]",
		heating.ID: "map[by:map[person:sam] heard:direct mentioned:false place:map[bridge:pager guests:false place:direct thing:u-sam:1 where:u-sam] team:office]",
	} {
		given := w.given(run)
		if got := fmt.Sprint(map[string]any{"by": given["by"], "heard": given["heard"], "mentioned": given["mentioned"], "place": given["place"], "team": given["team"]}); got != want {
			t.Errorf("the request is given %s, not %s", got, want)
		}
	}
	if !regexp.MustCompile(`^client/20261005T090000\.000Z-[0-9a-f]{16}$`).MatchString(boiler.ID) {
		t.Errorf("the request heard is named %s, not from the thing's time", boiler.ID)
	}

	garden := w.list(client.Filter{Labels: map[string]string{"kind": "request", "team": "garden"}})
	if len(garden) != 2 {
		t.Fatalf("the garden has %d requests, not 2", len(garden))
	}
	for _, run := range garden {
		input := w.input(run.ID)
		said, _ := input["said"].(string)
		switch {
		case said == "The roses need water.":
			if given := w.given(run.ID); fmt.Sprint(given["by"], given["mentioned"]) != "map[person:kit] true" {
				t.Errorf("the roses are asked %v", given)
			}
			reader, file, err := w.engine.File(ctx, run.ID, 0)
			if err != nil {
				t.Fatal(err)
			}
			if data, _ := io.ReadAll(reader); string(data) != "Pink ones." || file.Name != "roses.txt" {
				t.Errorf("the file said with the roses is %+v, holding %q", file, data)
			}
		case strings.HasPrefix(said, "Weeds. "):
			if !strings.HasSuffix(said, "\n\n[Bulldog cut this message: it is 42 KiB, and Bulldog passes on the first 32 KiB.]") || len(said) > 33<<10 {
				t.Errorf("the long text is given as %d bytes, ending %q", len(said), said[len(said)-100:])
			}
		default:
			t.Errorf("the garden is asked %q", said)
		}
	}
}

// TestBridgesIdentityIsTheSender: a press answers the wait its ref names
// only for a person the wait waits on, known by the sender's identity, in
// a channel as in a direct message. What a person says is theirs, wherever
// it is said.
func TestBridgesIdentityIsTheSender(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.say("pager", said("C-help/1", "u-guest", "#help", "C-help/1", "Is the boiler on?"))
	boiler := w.request("Is the boiler on?")
	r, _ := ref(w.question("Is the boiler on?"))
	inHelp := press("press:3", "u-sam", r, "It is on.")
	inHelp["place"], inHelp["where"] = "#help", "C-help/1"
	w.say("pager",
		press("press:1", "u-kit", r, "It is off."),
		press("press:2", "u-guest", r, "It is off."),
		said("u-sam:2", "u-kit", "direct", "u-sam", "It is off, says Kit."),
		said("C-help/2", "u-sam", "#help", "C-help/2", "Sam asks in #help."),
		inHelp,
	)
	if answer := w.answered(boiler.ID); answer != "It is on." {
		t.Errorf("the boiler's question is answered %q", answer)
	}
	for text, by := range map[string]string{"It is off, says Kit.": "map[person:kit]", "Sam asks in #help.": "map[person:sam]"} {
		if got := fmt.Sprint(w.given(w.request(text).ID)["by"]); got != by {
			t.Errorf("%q is asked by %s, not %s", text, got, by)
		}
	}
	replies := w.replies(boiler.ID)
	if input := w.input(replies[0].ID); len(replies) != 1 || fmt.Sprintln(input["to"], input["answers"], input["direct"], input["text"]) != "C-help/1 C-help/1 false It is on.\n" {
		t.Errorf("the answer is carried by %d runs, the first given %v", len(replies), input)
	}
}

// TestBridgesTypedAnswerTakenOnce: something said at direct that replies
// to nothing answers the latest question carried to its sender before it
// was heard; one that replies to a carried question answers that one.
// Read again after a restart, an answer answers no newer question, and
// starts no request.
func TestBridgesTypedAnswerTakenOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.say("pager", said("C-help/1", "u-guest", "#help", "C-help/1", "Question A?"))
	a := w.request("Question A?")
	w.question("Question A?")
	w.say("pager", said("u-sam:1", "u-sam", "direct", "u-sam", "Answer A."))
	if answer := w.answered(a.ID); answer != "Answer A." {
		t.Errorf("A is answered %q", answer)
	}
	w.say("pager", said("C-help/2", "u-guest", "#help", "C-help/2", "Question B?"))
	b := w.request("Question B?")
	_, sent := ref(w.question("Question B?"))

	w.restart()
	reply := said("u-sam:2", "u-sam", "direct", "u-sam", "Answer B.")
	reply["answers"] = sent
	w.say("pager", reply)
	if answer := w.answered(b.ID); answer != "Answer B." {
		t.Errorf("B is answered %q", answer)
	}
	for _, run := range w.requests() {
		if said := w.input(run.ID)["said"]; said != "Question A?" && said != "Question B?" {
			t.Errorf("a request is asked %q", said)
		}
	}
}

// TestBridgesInRunKept: Bulldog keeps one in run for each bridge, labelled
// with a fingerprint of its definitions and settings, and gives a new
// bridge's first run no cursor. When the bridge changes, or its run ends,
// Bulldog starts a new one, given the last cursor the old one emitted. The
// same thing handed over by two in runs starts one request.
func TestBridgesInRunKept(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	first := w.resting("pager")
	if l := first.Labels; len(l) != 4 || l["bulldog"] != w.id || l["kind"] != "bridge" || l["bridge"] != "pager" || !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(l["made"]) {
		t.Errorf("the in run is labelled %v", l)
	}
	if input := w.input(first.ID); input["cursor"] != nil || fmt.Sprint(input["settings"]) != fmt.Sprintf("map[feed:%s/pager.json places:map[#garden:map[team:garden] #help:map[guests:true] direct:map[]]]", w.feeds) {
		t.Errorf("the first in run is given %v", input)
	}
	boiler := said("C-help/1", "u-guest", "#help", "C-help/1", "Is the boiler on?")
	w.say("pager", boiler)
	w.request("Is the boiler on?")

	// The thing is in the feed twice, so the next run, which hears from
	// the cursor after it, hands it over again.
	w.resting("pager")
	w.things["pager"] = append(w.things["pager"], maps.Clone(boiler))
	w.write("pager")
	w.change(map[string]string{"office.yaml": strings.Replace(theOffice, `"#garden": { team: garden }`, `"#garden": { team: garden }
      "#news": { team: garden }`, 1)})
	second := w.after("pager", first.ID)
	if shown := w.show(first.ID); shown.Status != "cancelled" || second.ID == first.ID || second.Labels["made"] == first.Labels["made"] {
		t.Errorf("the first in run is %s, and the second is labelled %v", shown.Status, second.Labels)
	}
	if cursor := w.input(second.ID)["cursor"]; cursor != 1.0 {
		t.Errorf("the second in run is given the cursor %v", cursor)
	}
	w.say("pager", said("C-help/2", "u-guest", "#help", "C-help/2", "Is the light on?"))
	w.request("Is the light on?")
	if n := len(w.requests()); n != 2 {
		t.Errorf("%d requests started, not 2", n)
	}

	// An in run that ends is followed by a new one, at once when it was
	// cancelled.
	w.resting("pager")
	if _, err := w.corbie.Cancel(ctx, second.ID, 0); err != nil {
		t.Fatal(err)
	}
	third := w.after("pager", second.ID)
	if n := len(w.inRuns("pager")); n != 3 {
		t.Errorf("the bridge has %d in runs, not 3", n)
	}
	if cursor := w.input(third.ID)["cursor"]; cursor != 3.0 || third.Labels["made"] != second.Labels["made"] {
		t.Errorf("the third in run is given the cursor %v, labelled %v", cursor, third.Labels)
	}
	if state := w.bridges.State("pager"); state.Status != "running" {
		t.Errorf("the bridge stands %+v", state)
	}
}

// Of two in runs of one bridge, which a restart finds, the older is
// cancelled, and the newer goes on.
func TestBridgesSecondInRunCancelled(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	first := w.resting("pager")
	o := w.engine.Reading().Office
	in, _, _ := parse(o, o.Bridge("pager"))
	w.corbie.Clock.Move(time.Millisecond)
	second, err := w.corbie.Start(ctx, in, client.Start{Input: w.input(first.ID), Labels: first.Labels})
	if err != nil {
		t.Fatal(err)
	}
	w.after("pager", first.ID)
	w.restart()
	if shown := w.corbie.Ended(t, first.ID); shown.Status != "cancelled" {
		t.Errorf("the older in run ended %s", shown.Status)
	}
	if shown := w.show(second); shown.Status != "running" {
		t.Errorf("the newer in run is %s", shown.Status)
	}
	w.state("pager", "running", "")
}

// TestBridgesAfterARestart: after a restart Bulldog reads again what every
// in run handed over, and doing a thing twice changes nothing: no request
// starts again, no question or answer is carried again, an out run that
// failed is not tried again, and the in run goes on.
func TestBridgesAfterARestart(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.say("pager", said("C-help/1", "u-guest", "#help", "C-help/1", "Is the boiler on?"))
	boiler := w.request("Is the boiler on?")
	r, _ := ref(w.question("Is the boiler on?"))
	w.say("pager", press("press:1", "u-sam", r, "Tell them to refuse this."))
	replies := w.replies(boiler.ID)
	look, err := w.engine.Look(ctx, boiler.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(look.Replies) != 1 || look.Replies[0].Status != "failed" || !strings.Contains(look.Replies[0].Error, "The stand-in refuses the message.") {
		t.Errorf("the answer is carried by %+v", look.Replies)
	}
	in := w.inRuns("pager")

	w.restart()
	w.say("pager", said("C-help/2", "u-guest", "#help", "C-help/2", "Is the light on?"))
	light := w.request("Is the light on?")
	r, _ = ref(w.question("Is the light on?"))
	w.say("pager", press("press:2", "u-sam", r, "It is on."))
	w.replies(light.ID)
	if again := w.list(client.Filter{Labels: map[string]string{"kind": "out", "request": boiler.ID}}); len(again) != len(replies) {
		t.Errorf("the boiler's answer is carried by %d runs", len(again))
	}
	if n := len(w.list(client.Filter{Labels: map[string]string{"kind": "out", "wait": ""}})); n != 2 {
		t.Errorf("%d questions are carried, not 2", n)
	}
	if n := len(w.requests()); n != 2 {
		t.Errorf("%d requests started, not 2", n)
	}
	if again := w.inRuns("pager"); len(again) != 1 || again[0].ID != in[0].ID {
		t.Errorf("the in runs are %v, and were %v", again, in)
	}
}

// A bridge whose job fails stands failing, with Corbie's error, until a
// job goes well.
func TestBridgesFailing(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.resting("pager")
	w.state("pager", "running", "")
	w.write("pager", "The stand-in refuses the token.")
	w.corbie.Clock.Move(time.Minute)
	w.state("pager", "failing", "The stand-in refuses the token.")
	w.resting("pager")
	w.write("pager")
	w.corbie.Clock.Move(time.Minute)
	w.state("pager", "running", "")
	if n := len(w.inRuns("pager")); n != 1 {
		t.Errorf("the bridge has %d in runs", n)
	}
}

// bell is a stand-in whose in run fails when a job of it fails.
func bell(files map[string]string) {
	files["office.yaml"] = strings.Replace(theOffice, "bridges:\n", "bridges:\n  bell:\n    feed: FEEDS/bell.json\n    places: {}\n", 1)
	for name, text := range standIn {
		files["bridges/bell/"+name] = strings.Replace(text, "on: { done: resting, failed: resting }", "on: { done: resting }", 1)
	}
}

// A bridge whose in run keeps ending is started again after a pause that
// grows, and stands failing, with Corbie's error, until a run goes well.
func TestBridgesRunThatKeepsEnding(t *testing.T) {
	t.Parallel()
	w := newOffice(t, bell)
	w.write("bell", "The stand-in refuses the token.")
	w.start()
	w.state("bell", "failing", "The stand-in refuses the token.")
	if runs := w.inRuns("bell"); len(runs) != 1 || w.show(runs[0].ID).Status != "failed" {
		t.Fatalf("the bell's in runs are %v", runs)
	}
	w.corbie.Clock.Move(time.Second)
	w.corbie.Until(t, "the bell's second in run fails", func() bool {
		runs := w.inRuns("bell")
		return len(runs) == 2 && w.show(runs[1].ID).Status == "failed"
	})
	w.write("bell")
	w.corbie.Clock.Move(2 * time.Second)
	w.after("bell", w.inRuns("bell")[1].ID)
	w.state("bell", "running", "")
	if n := len(w.inRuns("bell")); n != 3 {
		t.Errorf("the bell has %d in runs, not 3", n)
	}
	w.state("pager", "running", "")
}

func TestPauseGrowsToAMinute(t *testing.T) {
	t.Parallel()
	for ended, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 6: 32 * time.Second, 7: time.Minute, 100: time.Minute} {
		if got := pause(ended); got != want {
			t.Errorf("after %d runs ended, the pause is %v, not %v", ended, got, want)
		}
	}
}

// The last cursor of a run is the one its last job emitted: a job asked
// again comes after the one asked before it, whatever order a record
// gives them in.
func TestBridgesLastCursorByJob(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		a, b   string
		before bool
	}{
		{"hearing", "hearing#2", true},
		{"hearing#2", "hearing", false},
		{"hearing#9", "hearing#10", true},
		{"hearing#2", "hearing#2@2", true},
		{"hearing#3", "hearing#2@2", false},
	} {
		if got := before(client.Entry{Time: at, Job: c.a}, client.Entry{Time: at, Job: c.b}); got != c.before {
			t.Errorf("%s before %s is %v", c.a, c.b, got)
		}
	}
	if !before(client.Entry{Time: at, Job: "hearing#2"}, client.Entry{Time: at.Add(time.Millisecond), Job: "hearing"}) {
		t.Error("an emit written later does not come later")
	}
}

// TestBridgesCarry: a message is carried by one run of the bridge's out,
// under the name it gives and labelled kind=out and bridge, and a run that
// fails is not tried again. A bridge reaches a person directly when its
// out takes direct.
func TestBridgesCarry(t *testing.T) {
	t.Parallel()
	w := newWorld(t, board)
	b := w.bridges
	if !b.Direct("pager") || b.Direct("board") || b.Direct("nothing") {
		t.Errorf("pager, board and nothing reach a person directly: %v, %v, %v", b.Direct("pager"), b.Direct("board"), b.Direct("nothing"))
	}
	name, err := w.corbie.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	m := requests.Message{Bridge: "pager", To: "u-sam", Direct: true, Text: "Hello, Sam.", Choices: []string{"yes", "no"}, Ref: "0123456789abcdef",
		Run: name, Labels: map[string]string{"wait": "client/20261005T090000.000Z-0000000000000000"}}
	for range 2 {
		if err := b.Carry(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	out := w.corbie.Ended(t, name)
	if l := out.Labels; len(l) != 4 || l["bulldog"] != w.id || l["kind"] != "out" || l["bridge"] != "pager" || l["wait"] != m.Labels["wait"] {
		t.Errorf("the out run is labelled %v", l)
	}
	input, _ := out.Input.(map[string]any)
	delete(input, "settings")
	if fmt.Sprint(input, out.Output) != "map[answers:<nil> choices:[yes no] direct:true ref:0123456789abcdef text:Hello, Sam. to:u-sam] map[sent:u-sam/0123456789abcdef]" {
		t.Errorf("the out run is given %v, and gives %v", input, out.Output)
	}

	refused, _ := w.corbie.NewRun()
	m = requests.Message{Bridge: "board", To: "news/1", Text: "Please refuse this.", Answers: "news/0", Run: refused}
	if err := b.Carry(ctx, m); err != nil {
		t.Fatal(err)
	}
	if out := w.corbie.Ended(t, refused); out.Status != "failed" || !strings.Contains(out.Error, "The stand-in refuses the message.") {
		t.Errorf("the refused out run ended %s: %s", out.Status, out.Error)
	}
	if err := b.Carry(ctx, m); err != nil {
		t.Errorf("carrying the refused message again: %v", err)
	}
	if err := b.Carry(ctx, requests.Message{Bridge: "nothing", To: "u-sam", Text: "Hello?"}); err != nil {
		t.Errorf("carrying on no bridge: %v", err)
	}
	if n := len(w.list(client.Filter{Labels: map[string]string{"kind": "out"}})); n != 2 {
		t.Errorf("%d out runs started, not 2", n)
	}
}

// board is a second stand-in, listed before pager, whose out takes no
// direct. Sam is on it, and Kit only on it.
func board(files map[string]string) {
	files["office.yaml"] = strings.NewReplacer(
		"bridges:\n", "bridges:\n  board:\n    feed: FEEDS/board.json\n    places:\n      \"#news\": {}\n",
		"sam: { name: Sam, pager: u-sam }", "sam: { name: Sam, pager: u-sam, board: b-sam }",
		"kit: { name: Kit, pager: u-kit }", "kit: { name: Kit, board: b-kit }",
	).Replace(theOffice)
	for name, text := range standIn {
		files["bridges/board/"+name] = text
	}
	files["bridges/board/out.yaml"] = `flow: board-out
input: { settings: map, to: string, text: string }
stages:
  say: { job: { script: ./say.js, input: { to: "${input.to}", text: "${input.text}" } } }
output: ${stages.say.output}
`
}

// TestBridgesTicketNeverSentOut: a question's message carries its ref, a
// short keyed hash of the wait, and never the ticket's value.
func TestBridgesTicketNeverSentOut(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.say("pager", said("C-help/1", "u-guest", "#help", "C-help/1", "Is the boiler on?"))
	question := w.question("Is the boiler on?")
	wait := question.Labels["wait"]
	ticket := w.corbie.Waiting(t, wait, "answer")
	r, _ := ref(question)
	if found, err := w.engine.WaitOf(ctx, r); err != nil || found == nil || found.ID != wait || !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(r) {
		t.Errorf("the ref %s names %+v (%v)", r, found, err)
	}
	if carried := fmt.Sprint(question.ID, question.Labels, question.Input); strings.Contains(carried, ticket) {
		t.Errorf("the question carries the ticket's value: %s", carried)
	}
}

// TestBridgesOnlyDirectToAPerson: a question goes to a person only in
// their direct messages, on a bridge whose out takes direct. A person no
// bridge reaches directly is asked on the screens alone.
func TestBridgesOnlyDirectToAPerson(t *testing.T) {
	t.Parallel()
	w := newWorld(t, func(files map[string]string) {
		board(files)
		files["garden/team.yaml"] = "name: The garden\nabout: The beds and the shed.\npeople: [kit, sam]\nlead: asker\n"
		files["garden/asker.yaml"] = `flow: asker
input: { said: string, context: string, request: map, files: "list?" }
stages:
  question:
    job:
      machine: ./.bulldog/wait/wait.yaml
      input: { to: { people: [kit, sam] }, said: "${input.said}", choices: ["yes", "no"] }
output: { answer: "${stages.question.output.answer}" }
`
	})
	w.say("pager", said("C-garden/1", "u-sam", "#garden", "C-garden/1", "May the shed be painted?"))
	question := w.question("May the shed be painted?")
	carried := w.list(client.Filter{Labels: map[string]string{"kind": "out", "wait": question.Labels["wait"]}})
	if input := w.input(question.ID); len(carried) != 1 || question.Labels["bridge"] != "pager" || input["to"] != "u-sam" || input["direct"] != true {
		t.Errorf("the question is carried by %d runs, the one on %s given %v", len(carried), question.Labels["bridge"], input)
	}
	waiting, err := w.engine.Waiting(ctx, "kit")
	if err != nil || len(waiting) != 1 || waiting[0].ID != question.Labels["wait"] {
		t.Errorf("the questions waiting on Kit are %v (%v)", waiting, err)
	}
}

// TestBridgesChecked: a bridge's in.yaml and out.yaml are checked with
// Corbie's parser when the office is read. What it refuses is a problem of
// the file, which stops that bridge alone.
func TestBridgesChecked(t *testing.T) {
	t.Parallel()
	w := newOffice(t, func(files map[string]string) {
		files["office.yaml"] = strings.Replace(theOffice, "bridges:\n", "bridges:\n  broken:\n    places: {}\n", 1)
		for name, text := range standIn {
			files["bridges/broken/"+name] = text
		}
		files["bridges/broken/out.yaml"] = strings.Replace(standIn["out.yaml"], "stages:", "colour: blue\nstages:", 1)
	})
	w.start()
	o := w.engine.Reading().Office
	broken := o.Bridge("broken")
	if len(broken.Problems) != 1 || broken.Problems[0].File != "bridges/broken/out.yaml" || !strings.Contains(broken.Problems[0].Text, "colour") || len(o.Problems) != 1 {
		t.Errorf("the broken bridge has the problems %v, and the office %v", broken.Problems, o.Problems)
	}
	w.resting("pager")
	if state, runs := w.bridges.State("broken"), w.inRuns("broken"); state.Status != "stopped" || len(runs) != 0 || w.bridges.Direct("broken") {
		t.Errorf("the broken bridge stands %+v, with %d in runs", state, len(runs))
	}
}
