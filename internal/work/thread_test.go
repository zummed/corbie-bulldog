package work

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// reading has the bot read the thread with the arguments given when what
// is new to it, last in its context, holds a word, and answer with what it
// was given.
func reading(word string, args map[string]any) func(testkit.Turn) testkit.Reply {
	return func(turn testkit.Turn) testkit.Reply {
		prompt := turn.Prompt()
		if turn.Number == 1 && strings.Contains(prompt[strings.LastIndex(prompt, "\nNew for you:\n")+1:], word) {
			return testkit.Use("read_thread", args)
		}
		return testkit.Say("bot heard: " + turn.Last())
	}
}

// lastAnswer stays until a request is answered, and gives its latest
// answer.
func (b *bench) lastAnswer(n int64) string {
	b.t.Helper()
	b.settled(n, requests.Answered)
	answers := b.entries(n, requests.KindAnswer)
	if len(answers) == 0 {
		b.t.Fatalf("request %d holds no answer: %v", n, b.thread(n))
	}
	return answers[len(answers)-1].Text
}

func TestNotedMessagesAreMarkedInWhatTheLeadIsGiven(t *testing.T) {
	t.Parallel()
	b := newBench(t, heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs}))
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "Raj, I'll look at it", nil)).Number
	b.heeded(n, 1)
	b.settled(n, requests.Answered)
	b.inThread(n, "slack", "U0DANA00", "@bot what do you think?")
	run := b.started(n, 1)
	b.settled(n, requests.Answered)
	in := b.input(run.Name)
	context := in["context"].(string)
	holds(t, "the bot's context", context, "] Dana to Platform support (noted): Raj, I'll look at it\n")
	holdsNone(t, "the bot's context", context, "Not for the bot.", "for-us.js")
	if fresh := newFor(context); !strings.HasSuffix(fresh, "Dana to Platform support: @bot what do you think?") || strings.Contains(fresh, "Raj") {
		t.Errorf("what is new is given as %q", fresh)
	}
	d := in["request"].(map[string]any)
	entries, _ := d["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("the entries given are %v", entries)
	}
	if e := entries[0].(map[string]any); e["text"] != "Raj, I'll look at it" || e["noted"] != true {
		t.Errorf("the noted message is given as %v", e)
	}
	if e := d["new"].([]any)[0].(map[string]any); e["noted"] != false {
		t.Errorf("what is new is given as %v", e)
	}
}

func TestReadThreadGivesTheThreadFromAnEntryOn(t *testing.T) {
	t.Parallel()
	b := newBench(t, heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs}))
	b.says(reading("@bot read it", map[string]any{}))
	if err := b.reqs.Readers("slack", "#help/1700.1", []string{"U0DANA00", "U0RAJ000"}); err != nil {
		t.Fatal(err)
	}
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "Raj, I'll look at it", nil)).Number
	b.heeded(n, 1)
	b.inThread(n, "slack", "U0RAJ000", "thanks @bot")
	b.started(n, 1)
	b.settled(n, requests.Answered)
	b.inThread(n, "slack", "U0DANA00", "@bot read it")
	b.started(n, 2)
	read := b.lastAnswer(n)
	asked, said := b.entries(n, requests.KindAsked)[0], b.entries(n, requests.KindSaid)
	holds(t, "the thread read", read,
		fmt.Sprintf("bot heard: #%d [Mon 5 Oct 2026 10:00 BST] Dana to Platform support (Asked, noted): Raj, I'll look at it\n", asked.ID),
		fmt.Sprintf("\n#%d [Mon 5 Oct 2026 10:00 BST] Raj to Platform support (Said): thanks @bot\n", said[0].ID),
		"] bot to Dana (Answer): bot heard: ",
		fmt.Sprintf("\n#%d [Mon 5 Oct 2026 10:00 BST] Dana to Platform support (Said): @bot read it\n", said[1].ID))
	holdsNone(t, "the thread read", read, "Not for the bot.", "more after")

	// From an entry on, as many whole entries as fit, and how to read on.
	long := strings.Repeat("x", ThreadLimit/2)
	b.inThread(n, "slack", "U0RAJ000", long)
	b.heeded(n, 4)
	b.inThread(n, "slack", "U0RAJ000", long+"y")
	b.heeded(n, 5)
	b.says(reading("@bot from", map[string]any{"from": float64(said[1].ID)}))
	b.inThread(n, "slack", "U0DANA00", "@bot from there")
	b.started(n, 3)
	read = b.lastAnswer(n)
	said = b.entries(n, requests.KindSaid)
	if want := fmt.Sprintf("bot heard: #%d [", said[1].ID); !strings.HasPrefix(read, want) {
		t.Errorf("the thread read on begins %.100q", read)
	}
	holds(t, "the thread read on", read, "(Said, noted): "+long+"\n")
	holdsNone(t, "the thread read on", read, "@bot from there", "xy")
	if want := fmt.Sprintf("\n2 more after #%d; read on from #%d.", said[2].ID, said[2].ID+1); !strings.HasSuffix(read, want) {
		t.Errorf("the thread read on ends %q, not %q", read[max(0, len(read)-100):], want)
	}
}

func TestReadThreadWithPlaceOnlyWhereAsked(t *testing.T) {
	t.Parallel()
	b := newBench(t, work(t))
	b.says(reading("before?", map[string]any{"place": true}))
	n := b.chat("al", "", "what was said before?")
	holds(t, "the bot's answer in the chat", b.lastAnswer(n),
		fmt.Sprintf("Request %d was asked in the built-in chat, at no place, so nothing was said before it there", n))
}

func TestWorkReadThreadReadsOnlyWhatItsReadersMay(t *testing.T) {
	t.Parallel()
	files := heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs})
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      direct: {}", 1)
	b := newBench(t, files)
	b.says(reading("@bot what was said", map[string]any{"place": true}))
	reads := func(place string, ids ...string) {
		t.Helper()
		if err := b.reqs.Readers("slack", place, ids); err != nil {
			t.Fatal(err)
		}
	}
	noted := func(place, from, text string) int64 {
		t.Helper()
		n := must(b.reqs.AskAt("slack", "#help", place, "", from, text, nil)).Number
		b.heeded(n, 1)
		b.settled(n, requests.Answered)
		return n
	}
	everyone := []string{"U0DANA00", "U0AL0000", "U0GUEST1"}
	// Read by everyone who reads the request asked, and Raj too.
	reads("#help/1", append(everyone, "U0RAJ000")...)
	noted("#help/1", "U0DANA00", "the deploy is at three")
	// Not read by the guest.
	reads("#help/2", "U0DANA00", "U0AL0000")
	noted("#help/2", "U0DANA00", "between us, the card ending 4242")
	// Readers never reported: the asker alone.
	noted("#help/3", "U0DANA00", "a word to nobody")
	// In a direct message.
	direct := must(b.reqs.AskAt("slack", "direct", "D0DANA00", "", "U0DANA00", "my own question", nil)).Number
	b.lastAnswer(direct)

	reads("#help/5", everyone...)
	n := must(b.reqs.AskAt("slack", "#help", "#help/5", "", "U0AL0000", "@bot what was said before?", nil)).Number
	read := b.lastAnswer(n)
	holds(t, "what was said before", read, "] Dana to Platform support (Asked, noted): the deploy is at three\n")
	holdsNone(t, "what was said before", read, "4242", "a word to nobody", "my own question", "what was said before?")

	// What is asked after it is not before it.
	reads("#help/6", everyone...)
	noted("#help/6", "U0DANA00", "later talk")
	b.inThread(n, "slack", "U0AL0000", "@bot what was said, again?")
	b.started(n, 2)
	read = b.lastAnswer(n)
	holds(t, "what was said before, again", read, "the deploy is at three")
	holdsNone(t, "what was said before, again", read, "later talk", "4242")
}
