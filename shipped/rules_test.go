package shipped_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

// dana and raj ask in #refunds; a guest asks there too.
var (
	dana  = map[string]any{"person": "dana"}
	raj   = map[string]any{"person": "raj"}
	guest = map[string]any{"guest": "slack U0GUEST1"}
)

// asked gives a request as a heed is given it: said at a channel, by
// someone, after the thread.
func asked(by map[string]any, mentioned bool, thread ...any) map[string]any {
	return map[string]any{"team": "payments", "by": by, "heard": "channel", "mentioned": mentioned, "thread": thread}
}

// earlier is a request before this one in its thread.
func earlier(by map[string]any, at, said string, answer any) map[string]any {
	return map[string]any{"by": by, "said": said, "answer": answer, "at": "2026-10-05T" + at + ":00Z"}
}

// heedSays runs a heed and gives what it said.
func heedSays(t *testing.T, corbie *testkit.Corbie, d *machine.Draft, input map[string]any) map[string]any {
	t.Helper()
	run := start(t, corbie, d, client.Start{Input: input, Secrets: map[string]string{"TYPESAFE_KEY": "ts-key"}})
	ended := corbie.Ended(t, run)
	out, _ := ended.Output.(map[string]any)
	if ended.Status != "done" || out == nil {
		t.Fatalf("the heed ended %s with %#v: %s", ended.Status, ended.Output, ended.Error)
	}
	return out
}

func TestMentionedSaysYesToWhatIsAddressed(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	src, _ := shipped.Read("rules/mentioned/mentioned.js")
	d, err := client.Script("mentioned.js", src, client.ScriptJob{})
	if err != nil {
		t.Fatal(err)
	}
	refunded := earlier(dana, "09:00", "was 4411 refunded?", "Refunded £40.")
	for _, c := range []struct {
		name    string
		request map[string]any
		says    string
		note    string
	}{
		{"named", asked(dana, true), "yes", "The message names the bot."},
		{"after its answer to her", asked(dana, false, refunded), "yes", "The message follows the answer to its sender."},
		{"after its answer to someone else", asked(raj, false, refunded), "no", ""},
		{"after a request not yet answered", asked(dana, false, earlier(dana, "09:00", "and 4412?", nil)), "no", ""},
		{"after someone else came between", asked(dana, false, refunded, earlier(raj, "09:05", "lunch?", nil)), "no", ""},
		{"a guest after its answer to that guest", asked(guest, false, earlier(guest, "09:00", "hello?", "Hello.")), "yes", "The message follows the answer to its sender."},
		{"talk", asked(dana, false), "no", ""},
	} {
		out := heedSays(t, corbie, d, map[string]any{"request": c.request, "notes": []any{}})
		if out["says"] != c.says || c.note != "" && out["note"] != c.note || out["note"] == "" {
			t.Errorf("%s: the heed says %v", c.name, out)
		}
	}
}

// A systemOne stands in for System One: it answers each call with how sure
// it is, or, while sure is below 0, holds the call until the test ends.
type systemOne struct {
	*httptest.Server
	sure   float64
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
		sure := s.sure
		s.mu.Unlock()
		select {
		case s.called <- struct{}{}:
		default:
		}
		if sure < 0 {
			select {
			case <-r.Context().Done():
			case <-stop:
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"model": "jev-1.13.0", "usage": map[string]any{"input_tokens": 10, "output_tokens": 1},
			"answers": map[string]any{"for_us": map[string]any{"type": "noul", "noul": sure}}})
	}))
	t.Cleanup(func() {
		close(stop)
		s.Close()
	})
	return s
}

func (s *systemOne) calls() ([]map[string]any, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.asked...), append([]string(nil), s.keys...)
}

// forUs gives the shipped for-us rule, its service's host changed for the
// stand-in's, reached over a plain connection.
func forUs(t *testing.T, s *systemOne) *machine.Draft {
	t.Helper()
	addr := strings.TrimPrefix(s.URL, "http://")
	pointed := func(name string) ([]byte, error) {
		contents, err := shipped.Read(path.Join("rules/for-us", name))
		if err != nil {
			return nil, err
		}
		text := strings.ReplaceAll(string(contents), "@net:api.typesafe.ai", "@net.unencrypted:"+addr)
		text = strings.ReplaceAll(text, "net:api.typesafe.ai", "net:"+addr)
		return []byte(strings.ReplaceAll(text, "https://api.typesafe.ai", "http://"+addr)), nil
	}
	src, _ := pointed("for-us.yaml")
	d, err := machine.Parse(src, pointed)
	if err != nil {
		t.Fatalf("for-us is refused:\n%v", err)
	}
	return d
}

func TestForUsAsksItsModel(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	sure := newSystemOne(t, 0.55)
	d := forUs(t, sure)
	thread := earlier(dana, "09:00", "is the API down?", "It is up.")
	if out := heedSays(t, corbie, d, map[string]any{"request": asked(raj, false, thread), "said": "Raj here: the EU one is down", "notes": []any{}}); out["says"] != "yes" || out["note"] != "For payments (0.55)." {
		t.Errorf("the heed says %v", out)
	}
	asked0, keys := sure.calls()
	if len(asked0) != 1 || asked0[0]["model"] != "jev-latest" || keys[0] != "Bearer ts-key" {
		t.Fatalf("the model was asked %v, with %v", asked0, keys)
	}
	question, _ := json.Marshal(asked0[0]["questions"])
	state, _ := asked0[0]["state"].(string)
	for _, said := range []string{"payments", "dana: is the API down?", "payments: It is up.", "The newest, from raj:", "Raj here: the EU one is down"} {
		if !strings.Contains(state, said) {
			t.Errorf("the model was given %q, without %q", state, said)
		}
	}
	if string(question) != `{"for_us":{"instructions":"The newest message asks this team for something, or answers what it last said.","type":"noul"}}` {
		t.Errorf("the model was asked %s", question)
	}
	// Below 0.5, it says no.
	sure.mu.Lock()
	sure.sure = 0.45
	sure.mu.Unlock()
	if out := heedSays(t, corbie, d, map[string]any{"request": asked(raj, false), "said": "lovely weather", "notes": []any{}}); out["says"] != "no" || out["note"] != "Not for payments (0.45)." {
		t.Errorf("the heed says %v", out)
	}
	asked1, _ := sure.calls()
	if len(asked1) != 2 || !strings.Contains(asked1[1]["state"].(string), "lovely weather") {
		t.Errorf("the model was asked %v", asked1)
	}
	// What names the bot, and what follows its answer to the sender, come
	// in with no model asked.
	if out := heedSays(t, corbie, d, map[string]any{"request": asked(raj, true), "said": "@bot is it?"}); out["says"] != "yes" || out["note"] != "The message names the bot." {
		t.Errorf("the heed says %v", out)
	}
	if out := heedSays(t, corbie, d, map[string]any{"request": asked(dana, false, thread), "said": "thanks"}); out["says"] != "yes" || out["note"] != "The message follows the answer to its sender." {
		t.Errorf("the heed says %v", out)
	}
	if asked2, _ := sure.calls(); len(asked2) != 2 {
		t.Errorf("the model was asked %d times", len(asked2))
	}
}

func TestForUsSaysYesWhenItsTimeRunsOut(t *testing.T) {
	t.Parallel()
	corbie := testkit.NewCorbie(t)
	silent := newSystemOne(t, -1)
	d := forUs(t, silent)
	run := start(t, corbie, d, client.Start{Input: map[string]any{"request": asked(raj, false), "said": "lovely weather", "notes": []any{}}, Secrets: map[string]string{"TYPESAFE_KEY": "ts-key"}})
	select {
	case <-silent.called:
	case <-time.After(testkit.Patience):
		t.Fatalf("waited %v, and the model was not asked", testkit.Patience)
	}
	corbie.Clock.Move(9 * time.Second)
	if shown, err := corbie.Show(ctx, run); err != nil || shown.HasEnded() {
		t.Fatalf("the heed ended a second before its time: %v %v", shown, err)
	}
	corbie.Clock.Move(time.Second)
	ended := corbie.Ended(t, run)
	if out, _ := ended.Output.(map[string]any); ended.Status != "done" || out["says"] != "yes" || out["note"] != "The model did not answer in time, so the message comes in." {
		t.Errorf("the heed ended %s with %#v", ended.Status, ended.Output)
	}
}
