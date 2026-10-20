package work

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// forUs is a heed that acts on what names the bot, and notes the rest.
const forUs = `if (String(input.said).includes("@bot")) return { says: "act" }
return { says: "note", note: "Not for the bot." }
`

// heeding gives the work office with the office heeding as written, and
// the files given beside it.
func heeding(t *testing.T, heed string, files map[string]string) map[string]string {
	out := work(t)
	out["office.yaml"] = strings.Replace(out["office.yaml"], "\nlead: bot\n", "\nlead: bot\nheed: "+heed+"\n", 1)
	for name, text := range files {
		out[name] = text
	}
	return out
}

// heeds gives the heed's runs kept with a request.
func (b *bench) heeds(n int64) []KeptRun {
	b.t.Helper()
	return slices.DeleteFunc(must(b.w.Runs(n)), func(r KeptRun) bool { return r.Kind != KindHeed })
}

// heeded stays until a request has had so many runs of its heed, all
// ended, and gives the latest.
func (b *bench) heeded(n int64, count int) KeptRun {
	b.t.Helper()
	var runs []KeptRun
	b.until(fmt.Sprintf("request %d has had %d heed runs, ended", n, count), func() bool {
		runs = b.heeds(n)
		return len(runs) >= count && runs[len(runs)-1].Status != going
	})
	if len(runs) > count {
		b.t.Fatalf("request %d has had %d heed runs, not %d", n, len(runs), count)
	}
	return runs[count-1]
}

// inThread says something more in a request where it was asked on a
// bridge, as the bridge hands it in.
func (b *bench) inThread(n int64, bridge, id, text string) {
	b.t.Helper()
	r := must(b.reqs.Get(n))
	if _, err := b.reqs.Say(n, requests.Identity(bridge, id), text, nil, requests.Where{Bridge: bridge, Place: r.Origin.Place}, false); err != nil {
		b.t.Fatal(err)
	}
}

func TestAHeedWeighsWhatIsSaidAtAChannelPlace(t *testing.T) {
	t.Parallel()
	files := heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs, "platform/for-us.js": forUs})
	files["office.yaml"] = strings.Replace(files["office.yaml"], `"#help": { guests: true }`, `"#help": { guests: true }`+"\n      direct: {}", 1)
	files["platform/team.yaml"] += "heed: ./for-us.js\n"
	b := newBench(t, files)
	// Dana's word to a colleague in #help is noted: the bot never starts.
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "Raj, I'll look at it", nil, false)).Number
	if h := b.heeded(n, 1); h.Status != done || h.How != "note: Not for the bot." || h.Worker != "for-us.js" {
		t.Errorf("the heed's run is %+v", h)
	}
	b.settled(n, requests.Answered)
	if st := must(b.reqs.State(n)); !st.Noted {
		t.Errorf("request %d stands %+v", n, st)
	}
	if runs := b.runs(n); len(runs) != 0 {
		t.Errorf("the bot was started %d times", len(runs))
	}
	if noted := b.entries(n, requests.KindNoted); len(noted) != 1 || noted[0].Text != "Not for the bot." || noted[0].By.Name != "for-us.js" ||
		noted[0].Read != "office" {
		t.Errorf("the noted entries are %v", b.thread(n))
	}
	// Her next word names the bot: the bot starts on it, and only on it.
	b.inThread(n, "slack", "U0DANA00", "@bot what do you think?")
	run := b.started(n, 1)
	if said := fmt.Sprint(b.input(run.Name)["said"]); said != "@bot what do you think?" {
		t.Errorf("the bot is given %q", said)
	}
	b.settled(n, requests.Answered)
	if len(b.heeds(n)) != 2 {
		t.Errorf("the heed weighed %d times", len(b.heeds(n)))
	}
	// Al, on the screens, is not weighed.
	b.say(n, "al", "and the deploy?")
	b.started(n, 2)
	b.settled(n, requests.Answered)
	// Nor is a direct message.
	direct := must(b.reqs.AskAt("slack", "direct", "D0DANA00", "", "U0DANA00", "hello", nil, false)).Number
	b.started(direct, 1)
	b.settled(direct, requests.Answered)
	if len(b.heeds(direct)) != 0 {
		t.Error("a direct message was weighed")
	}
	// The opening of an issue is addressed to whoever watches it, and is
	// not weighed; a comment in it is.
	issue := must(b.reqs.AskAt("gitlab", "platform/api", "platform/api#7", "https://gitlab.com/platform/api/-/issues/7", "1041", "the retry is broken", nil, false)).Number
	b.started(issue, 1)
	b.settled(issue, requests.Answered)
	if len(b.heeds(issue)) != 0 {
		t.Error("the opening of an issue was weighed")
	}
	b.inThread(issue, "gitlab", "1041", "thanks")
	if h := b.heeded(issue, 1); h.How != "note: Not for the bot." || h.Team != "platform" {
		t.Errorf("the comment's heed is %+v", h)
	}
	b.settled(issue, requests.Answered)
	if len(b.runs(issue)) != 1 {
		t.Errorf("ops was started %d times", len(b.runs(issue)))
	}
}

func TestWorkAnAnswerToTheLeadIsNeverHeeded(t *testing.T) {
	t.Parallel()
	b := newBench(t, heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs}))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "eu-west"):
			return testkit.Say("Looking at eu-west.")
		case turn.Number == 1:
			return testkit.Use("ask_person", map[string]any{"question": "which region?"})
		}
		return testkit.Say("done")
	})
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "@bot is the API down?", nil, false)).Number
	q := b.waitsOn(n, requests.KindQuestion)
	b.later(week)
	b.until("the bot's run ends", func() bool { runs := b.runs(n); return len(runs) == 1 && runs[0].Status == done })
	// Dana answers the bot's question in the thread, naming nobody: the bot
	// starts on it, and no heed weighs it.
	if _, err := b.reqs.Reply(q.ID, requests.Identity("slack", "U0DANA00"), "eu-west", nil, requests.Where{Bridge: "slack", Place: "#help/1700.1"}); err != nil {
		t.Fatal(err)
	}
	run := b.started(n, 2)
	if said := fmt.Sprint(b.input(run.Name)["said"]); said != "eu-west" {
		t.Errorf("the bot is given %q", said)
	}
	b.settled(n, requests.Answered)
	if heeds := b.heeds(n); len(heeds) != 1 {
		t.Errorf("the heed weighed %d times", len(heeds))
	}
}

func TestWorkAHeedThatFailsStartsTheLead(t *testing.T) {
	t.Parallel()
	b := newBench(t, heeding(t, "./for-us.js", map[string]string{"for-us.js": `throw new Error("the model is away")` + "\n"}))
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "is the API down?", nil, false)).Number
	if h := b.heeded(n, 1); h.Status != failed {
		t.Errorf("the heed's run is %+v", h)
	}
	b.answered(n)
	told := b.entries(n, requests.KindBulldog)
	if len(told) != 1 || told[0].To[0].Name != "office" || told[0].Read != "office" {
		t.Fatalf("Bulldog wrote %v", b.thread(n))
	}
	holds(t, "what the office's people are told", told[0].Text, "for-us.js failed: ", "the model is away", ". bot was started.")
	// A heed that answers neither act nor note fails the same way.
	b.rewrite(map[string]string{"for-us.js": `return { says: "maybe" }` + "\n"})
	m := must(b.reqs.AskAt("slack", "#help", "#help/1700.2", "", "U0DANA00", "and the deploy?", nil, false)).Number
	if h := b.heeded(m, 1); h.Status != failed || !strings.Contains(h.How, "neither act nor note") {
		t.Errorf("the heed's run is %+v", h)
	}
	b.answered(m)
	if told := b.entries(m, requests.KindBulldog); len(told) != 1 || !strings.Contains(told[0].Text, "neither act nor note") || told[0].To[0].Name != "office" {
		t.Errorf("Bulldog wrote %v", b.thread(m))
	}
}

func TestWorkAHeedIsGivenItsSettings(t *testing.T) {
	t.Parallel()
	const echo = `return { says: "note", note: JSON.stringify(input.settings) + " " + input.request.new.length + " " + input.said }` + "\n"
	b := newBench(t, heeding(t, "{ rule: ./for-us.js, threshold: 0.6, model: jev }", map[string]string{"for-us.js": echo}))
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "lovely weather", nil, false)).Number
	h := b.heeded(n, 1)
	holds(t, "the heed's word", h.How, `{"model":"jev","threshold":0.6} 1 [Mon `, "] Dana to Platform support: lovely weather")
	in := b.input(h.Name)
	if _, ok := in["context"]; ok {
		t.Error("the heed is given a context")
	}
	if settings, ok := in["settings"].(map[string]any); !ok || settings["threshold"] != 0.6 || settings["model"] != "jev" {
		t.Errorf("the heed is given %v", in["settings"])
	}
	// A heed written as a bare path is given no settings, and given them
	// all the same.
	b.rewrite(map[string]string{"office.yaml": strings.Replace(b.file("office.yaml"), "heed: { rule: ./for-us.js, threshold: 0.6, model: jev }", "heed: ./for-us.js", 1)})
	m := must(b.reqs.AskAt("slack", "#help", "#help/1700.2", "", "U0DANA00", "rain later", nil, false)).Number
	holds(t, "the heed's word", b.heeded(m, 1).How, "note: {} 1 ")
	// What the bridge marked as naming the bot is given so.
	mentioned := func(h KeptRun) any {
		request, _ := b.input(h.Name)["request"].(map[string]any)
		fresh, _ := request["new"].([]any)
		if len(fresh) != 1 {
			t.Fatalf("the heed is given as new %v", request["new"])
		}
		return fresh[0].(map[string]any)["mentioned"]
	}
	named := must(b.reqs.AskAt("slack", "#help", "#help/1700.3", "", "U0DANA00", "<@U0APP000> rain later?", nil, true)).Number
	if got, unmarked := mentioned(b.heeded(named, 1)), mentioned(b.heeded(m, 1)); got != true || unmarked != false {
		t.Errorf("the heed is given mentioned %v, and for what is not marked %v", got, unmarked)
	}
}

func TestWorkAHeedRunIsTheTeams(t *testing.T) {
	t.Parallel()
	files := heeding(t, "./for-us.js", map[string]string{"for-us.js": forUs})
	files["office.yaml"] += "runs: 1\n"
	b := newBench(t, files)
	held, let := b.hold("hold on", testkit.Say("held"))
	// The bot works on one request, which is all the office's room.
	busy := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "@bot hold on", nil, false)).Number
	b.until("the bot's run is held", func() bool { return held() == 1 })
	// A heed takes no room: it weighs another request all the same.
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.2", "", "U0DANA00", "Raj, I'll look", nil, false)).Number
	h := b.heeded(n, 1)
	b.settled(n, requests.Answered)
	runs := b.listed(map[string]string{LabelKind: KindHeed, requests.LabelRequest: fmt.Sprint(n)})
	if len(runs) != 1 || runs[0].ID != h.Name || runs[0].Labels[requests.LabelTeam] != "office" || runs[0].Labels[LabelBulldog] != identity {
		t.Errorf("the heed's runs are %+v", runs)
	}
	if noted := b.entries(n, requests.KindNoted); len(noted) != 1 || noted[0].Read != "office" || noted[0].By.Team != "office" {
		t.Errorf("the noted entries are %+v", noted)
	}
	let()
	b.settled(busy, requests.Answered)
}

func TestWorkNoLeadWhileAHeedWeighs(t *testing.T) {
	t.Parallel()
	b := newBench(t, heeding(t, "./for-us.yaml", map[string]string{
		"for-us.yaml": "agent: for-us\ninput: [said]\nmodel: usual\nprompt: \"Weigh: ${input.said}\"\n",
	}))
	held, let := b.hold("Weigh:", testkit.Say(`{ "says": "act" }`))
	n := must(b.reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "@bot first", nil, false)).Number
	b.until("the heed is held", func() bool { return held() == 1 })
	// What Dana says while the heed weighs starts nothing.
	b.inThread(n, "slack", "U0DANA00", "and second")
	b.still()
	if runs := b.runs(n); len(runs) != 0 {
		t.Errorf("the bot was started while the heed weighed: %+v", runs)
	}
	if heeds := b.heeds(n); len(heeds) != 1 || heeds[0].Status != going {
		t.Errorf("the heed's runs are %+v", heeds)
	}
	// It acts, and the bot starts once, on both.
	let()
	run := b.started(n, 1)
	holds(t, "what the bot is given", fmt.Sprint(b.input(run.Name)["said"]), "@bot first", "and second")
	b.settled(n, requests.Answered)
	if heeds := b.heeds(n); len(heeds) != 1 || heeds[0].How != "act" {
		t.Errorf("the heed's runs are %+v", heeds)
	}
}
