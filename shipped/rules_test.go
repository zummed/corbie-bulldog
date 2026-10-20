package shipped_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

// A systemOne is a stand-in System One service: it answers each call with
// how sure it is, or, while sure gives a negative number, never answers.
type systemOne struct {
	*httptest.Server
	sure float64
	// called is sent to as each call comes, which writes nothing to the
	// store a test could wait on.
	called chan struct{}
	mu     sync.Mutex
	asked  []map[string]any
	keys   []string
}

func newSystemOne(t *testing.T, sure float64) *systemOne {
	s := &systemOne{sure: sure, called: make(chan struct{}, 8)}
	stop := make(chan struct{})
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var asked map[string]any
		if r.URL.Path != "/v1/systemone" || json.NewDecoder(r.Body).Decode(&asked) != nil {
			http.Error(w, "not a call to decide", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.asked, s.keys = append(s.asked, asked), append(s.keys, r.Header.Get("Authorization"))
		s.mu.Unlock()
		select {
		case s.called <- struct{}{}:
		default:
		}
		if s.sure < 0 {
			select {
			case <-r.Context().Done():
			case <-stop:
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"model": "jev-1.13.0", "usage": map[string]any{"input_tokens": 10, "output_tokens": 1},
			"answers": map[string]any{"for_us": map[string]any{"type": "noul", "noul": s.sure}}})
	}))
	t.Cleanup(func() {
		close(stop)
		s.Close()
	})
	return s
}

// calls gives what each call to the stand-in asked, and with which key.
func (s *systemOne) calls() ([]map[string]any, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.asked...), append([]string(nil), s.keys...)
}

// forUs gives the for-us heed as shipped, with its service's host in place
// of the stand-in's, reached over a plain connection.
func forUs(t *testing.T, s *systemOne) *machine.Draft {
	t.Helper()
	addr := strings.TrimPrefix(s.URL, "http://")
	pointed := func(name string) ([]byte, error) {
		contents, ok := shipped.File(path.Join(path.Dir(shipped.ForUs), name))
		if !ok {
			return nil, fmt.Errorf("there is no %s", name)
		}
		text := strings.ReplaceAll(string(contents), "@net:api.typesafe.ai", "@net.unencrypted:"+addr)
		text = strings.ReplaceAll(text, "net:api.typesafe.ai", "net:"+addr)
		return []byte(strings.ReplaceAll(text, "https://api.typesafe.ai", "http://"+addr)), nil
	}
	src, _ := pointed(path.Base(shipped.ForUs))
	d, err := machine.Parse(src, pointed)
	if err != nil {
		t.Fatalf("for-us is refused:\n%v", err)
	}
	return d
}

// dana is Dana as a heed is given her, said through Slack.
var dana = map[string]any{"kind": "person", "name": "dana", "called": "Dana", "bridge": "slack", "id": "U0DANA00"}

// raj is Raj, said through Slack.
var raj = map[string]any{"kind": "person", "name": "raj", "called": "Raj", "bridge": "slack", "id": "U0RAJ000"}

// weighing gives a heed's input: Dana's messages, new to the lead, after
// the entries before them, with the settings. Each entry is numbered in
// the order given, but for one that already has its number.
func weighing(before []any, settings map[string]any, said ...string) map[string]any {
	if before == nil {
		before = []any{map[string]any{"kind": "asked", "by": dana, "text": "is the API down?"}}
	}
	var fresh []any
	for _, text := range said {
		fresh = append(fresh, map[string]any{"kind": "said", "by": dana, "text": text, "mentioned": strings.Contains(text, "<@")})
	}
	for i, e := range append(slices.Clone(before), fresh...) {
		if e := e.(map[string]any); e["id"] == nil {
			e["id"] = i + 1
		}
	}
	return map[string]any{
		"said": "[09:00] Dana: " + strings.Join(said, "\n[09:00] Dana: "),
		"request": map[string]any{"number": 1, "worker": "bot", "team": map[string]any{"name": "office", "called": "Platform support", "about": "The API."},
			"entries": before, "entries_not_shown": 0, "new": fresh},
		"settings": settings,
	}
}

// heedSays starts a heed and gives what it ended saying.
func heedSays(t *testing.T, corbie *testkit.Corbie, d *machine.Draft, input map[string]any) map[string]any {
	t.Helper()
	run, err := corbie.Start(ctx, d, client.Start{Input: input, Secrets: map[string]string{"TYPESAFE_KEY": "ts-key"}, Labels: map[string]string{"test": "heed"}})
	if err != nil {
		t.Fatal(err)
	}
	ended := corbie.Ended(t, run)
	out, _ := ended.Output.(map[string]any)
	if ended.Status != "done" || out == nil {
		t.Fatalf("the heed ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
	return out
}

func TestTheMentionedHeedActsOnWhatIsAddressed(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t, testkit.NewClock(testkit.Monday))
	contents, _ := shipped.File(shipped.Mentioned)
	d, err := client.Script(path.Base(shipped.Mentioned), contents, client.ScriptJob{})
	if err != nil {
		t.Fatal(err)
	}
	answered := []any{
		map[string]any{"kind": "asked", "by": dana, "text": "is the API down?"},
		map[string]any{"kind": "answer", "by": map[string]any{"kind": "worker", "name": "bot"}, "to": []any{dana}, "text": "It is up."},
	}
	// The bot's answer came after what Dana said while it was answering.
	answeredLate := []any{answered[0], map[string]any{"kind": "answer", "by": map[string]any{"kind": "worker", "name": "bot"}, "to": []any{dana}, "text": "It is up.", "id": 4}}
	for _, c := range []struct {
		name       string
		before     []any
		said       []string
		says, note string
	}{
		{"named", nil, []string{"<@U0APP000> and now?"}, "act", "Names bot."},
		{"one of two named", nil, []string{"hm", "<@U0APP000> and now?"}, "act", "Names bot."},
		{"after its answer", answered, []string{"and now?"}, "act", "Follows bot's answer."},
		{"not straight after its answer", append(answered[:2:2], map[string]any{"kind": "said", "by": raj, "text": "same here"}), []string{"and now?"}, "note", "Not addressed to bot."},
		{"talk", nil, []string{"Raj, I'll look at it"}, "note", "Not addressed to bot."},
		{"while it answers", answeredLate, []string{"the EU one"}, "act", "Said while bot was answering."},
	} {
		if out := heedSays(t, corbie, d, weighing(c.before, map[string]any{}, c.said...)); out["says"] != c.says || out["note"] != c.note {
			t.Errorf("%s: the heed says %v", c.name, out)
		}
	}
	// Someone else straight after the bot's answer to Dana is not
	// addressing it.
	input := weighing(answered, map[string]any{}, "me too")
	input["request"].(map[string]any)["new"].([]any)[0].(map[string]any)["by"] = map[string]any{"kind": "guest", "called": "a guest", "bridge": "slack", "id": "U0GUEST1"}
	if out := heedSays(t, corbie, d, input); out["says"] != "note" {
		t.Errorf("a guest after the bot's answer to Dana: the heed says %v", out)
	}
	// Raj, while the bot answered Dana, does not stand between its answer
	// and what she says after it; and he is not who it was answering.
	input = weighing(answeredLate, map[string]any{}, "same here", "and now?")
	fresh := input["request"].(map[string]any)["new"].([]any)
	fresh[0].(map[string]any)["by"], fresh[1].(map[string]any)["id"] = raj, 5
	if out := heedSays(t, corbie, d, input); out["says"] != "act" || out["note"] != "Follows bot's answer." {
		t.Errorf("Dana after the bot's answer, Raj before it: the heed says %v", out)
	}
	fresh = fresh[:1]
	input["request"].(map[string]any)["new"] = fresh
	if out := heedSays(t, corbie, d, input); out["says"] != "note" {
		t.Errorf("Raj while the bot answered Dana: the heed says %v", out)
	}
}

func TestTheForUsHeedAsksItsModel(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t, testkit.NewClock(testkit.Monday))
	sure := newSystemOne(t, 0.55)
	d := forUs(t, sure)
	// With its defaults: the model is asked its question, and 0.55 is sure
	// enough.
	if out := heedSays(t, corbie, d, weighing(nil, map[string]any{}, "Raj, I'll look at it")); out["says"] != "act" || out["note"] != "For Platform support (0.55)" {
		t.Errorf("the heed says %v", out)
	}
	asked, keys := sure.calls()
	if len(asked) != 1 || asked[0]["model"] != "jev-latest" || keys[0] != "Bearer ts-key" {
		t.Fatalf("the model was asked %v, with %v", asked, keys)
	}
	question, _ := json.Marshal(asked[0]["questions"])
	if state := asked[0]["state"].(string); !strings.Contains(state, "Platform support") || !strings.Contains(state, "Dana: is the API down?") || !strings.Contains(state, "Raj, I'll look at it") ||
		string(question) != `{"for_us":{"instructions":"The newest message asks this team for something, or answers what it last said.","type":"noul"}}` {
		t.Errorf("the model was given %q, and asked %s", state, question)
	}
	// Its settings, as the team file writes them.
	settings := map[string]any{"threshold": 0.6, "model": "jev-1.12", "question": "It is for the API team."}
	if out := heedSays(t, corbie, d, weighing(nil, settings, "lovely weather")); out["says"] != "note" || out["note"] != "Not for Platform support (0.55)" {
		t.Errorf("the heed says %v", out)
	}
	asked, _ = sure.calls()
	question, _ = json.Marshal(asked[len(asked)-1]["questions"])
	if len(asked) != 2 || asked[1]["model"] != "jev-1.12" || !strings.Contains(string(question), "It is for the API team.") {
		t.Errorf("the model was asked %v", asked)
	}
	// What names the bot is acted on with no model asked.
	if out := heedSays(t, corbie, d, weighing(nil, settings, "<@U0APP000> is it?")); out["says"] != "act" || out["note"] != "Names bot." {
		t.Errorf("the heed says %v", out)
	}
	// And so is what whoever it answers says while it is answering.
	late := []any{map[string]any{"kind": "asked", "by": dana, "text": "is the API down?"},
		map[string]any{"kind": "answer", "by": map[string]any{"kind": "worker", "name": "bot"}, "to": []any{dana}, "text": "It is up.", "id": 4}}
	if out := heedSays(t, corbie, d, weighing(late, settings, "the EU one")); out["says"] != "act" || out["note"] != "Said while bot was answering." {
		t.Errorf("the heed says %v", out)
	}
	if asked, _ = sure.calls(); len(asked) != 2 {
		t.Errorf("the model was asked %d times", len(asked))
	}
}

func TestWorkForUsActsWhenItsTimeRunsOut(t *testing.T) {
	t.Parallel()
	clock := testkit.NewClock(testkit.Monday)
	corbie := testkit.NewCorbie(t, clock)
	silent := newSystemOne(t, -1)
	d := forUs(t, silent)
	for _, c := range []struct {
		settings map[string]any
		limit    time.Duration
	}{{map[string]any{}, 10 * time.Second}, {map[string]any{"timeout": "5s"}, 5 * time.Second}} {
		run, err := corbie.Start(ctx, d, client.Start{Input: weighing(nil, c.settings, "lovely weather"), Secrets: map[string]string{"TYPESAFE_KEY": "ts-key"}, Labels: map[string]string{"test": "out"}})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-silent.called:
		case <-time.After(testkit.Patience):
			t.Fatalf("waited %v, and the model was not asked", testkit.Patience)
		}
		clock.Move(c.limit - time.Second)
		if shown, err := corbie.Show(ctx, run); err != nil || shown.HasEnded() {
			t.Fatalf("the heed ended a second before its time: %v %v", shown, err)
		}
		clock.Move(time.Second)
		ended := corbie.Ended(t, run)
		if out, _ := ended.Output.(map[string]any); ended.Status != "done" || out["says"] != "act" || out["note"] != "timed out" {
			t.Errorf("with %v, the heed ended %s with %#v", c.settings, ended.Status, ended.Output)
		}
	}
}
