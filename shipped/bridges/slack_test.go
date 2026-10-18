package bridges_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// The bot token and the signing secret the stand-in Slack holds.
const (
	slackToken  = "xoxb-the-apps-own-token"
	slackSigner = "the-signing-secret-8f2a"
)

// A slack stands in for Slack's Web API: it knows a few channels and who
// is in them, keeps what the app posted, and answers as Slack does.
type slack struct {
	*httptest.Server
	t      *testing.T
	mu     sync.Mutex
	posted []map[string]any
	ts     int
	// gone are threads whose first message was deleted.
	gone map[string]bool
	// fail, when set, answers a method in place of Slack, with an error.
	fail func(method string) (string, bool)
}

func newSlack(t *testing.T) *slack {
	sl := &slack{t: t, ts: 1700000000, gone: map[string]bool{}}
	sl.Server = httptest.NewServer(http.HandlerFunc(sl.serve))
	t.Cleanup(sl.Close)
	return sl
}

func (sl *slack) serve(w http.ResponseWriter, r *http.Request) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	answer := func(v map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/files/"):
		if r.Header.Get("Authorization") != "Bearer "+slackToken {
			w.WriteHeader(403)
			return
		}
		w.Write([]byte("the file " + r.URL.Path))
		return
	}
	method, _ := strings.CutPrefix(r.URL.Path, "/api/")
	if r.Header.Get("Authorization") != "Bearer "+slackToken {
		answer(map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}
	if sl.fail != nil {
		if code, ok := sl.fail(method); ok {
			answer(map[string]any{"ok": false, "error": code})
			return
		}
	}
	var body map[string]any
	data, _ := io.ReadAll(r.Body)
	json.Unmarshal(data, &body)
	q := r.URL.Query()
	switch method {
	case "conversations.info":
		channel := q.Get("channel")
		answer(map[string]any{"ok": true, "channel": map[string]any{"id": channel, "name": map[string]string{"C0HELP00": "help"}[channel], "is_im": strings.HasPrefix(channel, "D")}})
	case "conversations.members":
		if q.Get("cursor") == "" {
			answer(map[string]any{"ok": true, "members": []string{"U0DANA00", "U0AL0000"}, "response_metadata": map[string]any{"next_cursor": "page2"}})
		} else {
			answer(map[string]any{"ok": true, "members": []string{"U0GUEST0"}, "response_metadata": map[string]any{"next_cursor": ""}})
		}
	case "conversations.open":
		users := fmt.Sprint(body["users"])
		if users == "U0GONE00" {
			answer(map[string]any{"ok": false, "error": "user_not_found"})
			return
		}
		answer(map[string]any{"ok": true, "channel": map[string]any{"id": "D" + users[1:]}})
	case "chat.postMessage":
		text, _ := body["text"].(string)
		thread, _ := body["thread_ts"].(string)
		switch {
		case len([]rune(text)) > 40000:
			answer(map[string]any{"ok": false, "error": "msg_too_long"})
			return
		case sl.gone[thread]:
			answer(map[string]any{"ok": false, "error": "thread_not_found"})
			return
		}
		if blocks, ok := body["blocks"].([]any); ok {
			if section := blocks[0].(map[string]any)["text"].(map[string]any)["text"].(string); len([]rune(section)) > 3000 {
				answer(map[string]any{"ok": false, "error": "invalid_blocks"})
				return
			}
		}
		sl.ts++
		body["ts"] = fmt.Sprintf("%d.000100", sl.ts)
		sl.posted = append(sl.posted, body)
		answer(map[string]any{"ok": true, "channel": body["channel"], "ts": body["ts"]})
	case "conversations.history":
		// What the app posted in a conversation, newest first, before the
		// latest asked for.
		messages := []any{}
		for _, p := range sl.posted {
			if p["channel"] == q.Get("channel") && p["ts"].(string) < q.Get("latest") {
				messages = append([]any{map[string]any{"type": "message", "bot_id": "B01", "ts": p["ts"], "text": p["text"]}}, messages...)
			}
		}
		answer(map[string]any{"ok": true, "messages": messages})
	default:
		answer(map[string]any{"ok": false, "error": "unknown_method"})
	}
}

// newSlackBridge gives the shipped Slack bridge, pointed at a stand-in
// Slack, in a Corbie of the test's own.
func newSlackBridge(t *testing.T) (*slack, *service) {
	sl := newSlack(t)
	return sl, newService(t, pointed(t, "slack", sl.Server, "files.slack.com", "slack.com"),
		map[string]string{"SLACK_TOKEN": slackToken, "SLACK_SIGNING_SECRET": slackSigner})
}

// delivery gives what Slack sends the bridge's address with a body, signed
// with a secret at a time.
func delivery(body, contentType, secret string, at time.Time) map[string]any {
	ts := fmt.Sprint(at.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + ts + ":" + body))
	return map[string]any{"method": "POST", "body": body, "headers": map[string]any{
		"Content-Type": contentType, "X-Slack-Request-Timestamp": ts, "X-Slack-Signature": "v0=" + hex.EncodeToString(mac.Sum(nil)),
	}}
}

func event(id string, e map[string]any) string {
	data, _ := json.Marshal(map[string]any{"type": "event_callback", "event_id": id, "event": e})
	return string(data)
}

// hears runs the in machine on a delivery, and gives what it heard and
// what it says to answer Slack.
func (s *service) hears(d map[string]any) ([]map[string]any, map[string]any) {
	s.t.Helper()
	ended := s.run("in.yaml", map[string]any{"settings": map[string]any{"places": map[string]any{"#help": map[string]any{"guests": true}}}, "delivery": d})
	if ended.Status != "done" {
		s.t.Fatalf("the in run ended %s: %s", ended.Status, ended.Error)
	}
	out := ended.Output.(map[string]any)
	var things []map[string]any
	for _, t := range out["heard"].([]any) {
		things = append(things, t.(map[string]any))
	}
	answer, _ := out["answer"].(map[string]any)
	return things, answer
}

func TestBridgesSlackDeliverySignedAndFresh(t *testing.T) {
	t.Parallel()
	_, s := newSlackBridge(t)
	body := event("Ev1", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": "hello", "ts": "1700000001.000100"})
	if things, answer := s.hears(delivery(body, "application/json", slackSigner, time.Now())); len(things) != 1 || answer["status"] != float64(200) {
		t.Errorf("a signed delivery is heard as %v, answered %v", things, answer)
	}
	if things, answer := s.hears(delivery(body, "application/json", "another-secret", time.Now())); len(things) != 0 || answer["status"] != float64(401) {
		t.Errorf("a delivery signed with another secret is heard as %v, answered %v", things, answer)
	}
	if things, answer := s.hears(delivery(body, "application/json", slackSigner, time.Now().Add(-6*time.Minute))); len(things) != 0 || answer["status"] != float64(401) {
		t.Errorf("a delivery signed six minutes ago is heard as %v, answered %v", things, answer)
	}
	tampered := delivery(body, "application/json", slackSigner, time.Now())
	tampered["body"] = strings.Replace(body, "U0DANA00", "U0LEE000", 1)
	if things, _ := s.hears(tampered); len(things) != 0 {
		t.Errorf("a delivery changed after it was signed is heard as %v", things)
	}
	challenge := `{"type":"url_verification","challenge":"challenge-1"}`
	if _, answer := s.hears(delivery(challenge, "application/json", slackSigner, time.Now())); answer["body"] != "challenge-1" {
		t.Errorf("Slack's challenge is answered %v", answer)
	}
}

func TestBridgesSlackPlaceIsTheThread(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	top := event("Ev1", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": "at the top", "ts": "1700000001.000100"})
	under := event("Ev2", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0AL0000", "text": "under it", "ts": "1700000009.000100", "thread_ts": "1700000001.000100"})
	a, _ := s.hears(delivery(top, "application/json", slackSigner, time.Now()))
	b, _ := s.hears(delivery(under, "application/json", slackSigner, time.Now()))
	if len(a) != 1 || len(b) != 1 || a[0]["place"] != "C0HELP00/1700000001.000100" || b[0]["place"] != a[0]["place"] || a[0]["heard_in"] != "#help" || b[0]["heard_in"] != "#help" {
		t.Errorf("the top is heard as %v, and the reply as %v", a, b)
	}
	if readers, _ := json.Marshal(a[0]["readers"]); string(readers) != `["U0DANA00","U0AL0000","U0GUEST0"]` || a[0]["id"] != "Ev1" {
		t.Errorf("the top is heard as %v", a[0])
	}
	// What is said to the app directly is the sender's direct place.
	dm := event("Ev3", map[string]any{"type": "message", "channel": "D0DANA00", "channel_type": "im", "user": "U0DANA00", "text": "to you", "ts": "1700000010.000100"})
	if c, _ := s.hears(delivery(dm, "application/json", slackSigner, time.Now())); len(c) != 1 || c[0]["heard_in"] != "direct" || c[0]["place"] != "U0DANA00" {
		t.Errorf("a direct message is heard as %v", c)
	}
	// What the app itself posted is not heard.
	own := event("Ev4", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "bot_id": "B01", "text": "the answer", "ts": "1700000011.000100"})
	if c, _ := s.hears(delivery(own, "application/json", slackSigner, time.Now())); len(c) != 0 {
		t.Errorf("the app's own message is heard as %v", c)
	}
	// A file said is read out of Slack.
	file := event("Ev5", map[string]any{"type": "message", "subtype": "file_share", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": "this",
		"ts": "1700000012.000100", "files": []any{map[string]any{"name": "receipt.png", "url_private": sl.URL + "/files/T01/receipt.png"}}})
	c, _ := s.hears(delivery(file, "application/json", slackSigner, time.Now()))
	if len(c) != 1 || len(c[0]["files"].([]any)) != 1 {
		t.Fatalf("a file is heard as %v", c)
	}
	if got := c[0]["files"].([]any)[0].(map[string]any); got["name"] != "receipt.png" || got["data"] == nil {
		t.Errorf("the file is heard as %v", got)
	}
}

func TestBridgesSlackChoicesAreButtons(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": "Refund one charge?", "choices": []any{"yes", "no"}, "ref": "r12"}))
	if len(sl.posted) != 1 || sl.posted[0]["channel"] != "D0LEE000" {
		t.Fatalf("Slack was posted %v", sl.posted)
	}
	actions, _ := json.Marshal(sl.posted[0]["blocks"].([]any)[1])
	if !strings.Contains(string(actions), `"value":"r12:yes"`) || !strings.Contains(string(actions), `"value":"r12:no"`) {
		t.Errorf("the buttons are %s", actions)
	}
	payload, _ := json.Marshal(map[string]any{"type": "block_actions", "user": map[string]any{"id": "U0LEE000", "username": "lee"},
		"channel":   map[string]any{"id": "D0LEE000"},
		"container": map[string]any{"message_ts": "1700000001.000100", "channel_id": "D0LEE000"},
		"actions":   []any{map[string]any{"action_id": "choice-0", "value": "r12:yes", "action_ts": "1700000002.1"}}})
	form := "payload=" + url.QueryEscape(string(payload))
	things, answer := s.hears(delivery(form, "application/x-www-form-urlencoded", slackSigner, time.Now()))
	if len(things) != 1 || things[0]["kind"] != "choice" || things[0]["choice"] != "yes" || things[0]["answers"] != "r12" ||
		things[0]["from"] != "U0LEE000" || things[0]["heard_in"] != "direct" || things[0]["place"] != "U0LEE000" || answer["status"] != float64(200) {
		t.Errorf("Lee's press is heard as %v, answered %v", things, answer)
	}
	// An answer goes into its thread, with its files.
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": "Duplicate confirmed.",
		"files": []file{{"ledger.csv", []byte("a,b")}}}))
	if last := sl.posted[len(sl.posted)-1]; last["channel"] != "C0HELP00" || last["thread_ts"] != "1700000001.000100" || !strings.Contains(last["text"].(string), "ledger.csv") {
		t.Errorf("the answer is posted as %v", last)
	}
}

func TestBridgesSlackDirectReplyAnswersWhatWasSent(t *testing.T) {
	t.Parallel()
	_, s := newSlackBridge(t)
	// A question carried to Priya directly, and her typed reply in her
	// conversation with the app.
	ref := sent(t, s.carry(map[string]any{"to": "U0PRI000", "direct": true, "text": "Which card?", "choices": []any{}, "ref": "r7"}))
	reply := event("Ev7", map[string]any{"type": "message", "channel": "D0PRI000", "channel_type": "im", "user": "U0PRI000", "text": "the visa", "ts": "1800000000.000100"})
	things, _ := s.hears(delivery(reply, "application/json", slackSigner, time.Now()))
	if len(things) != 1 || things[0]["answers"] != ref || things[0]["place"] != "U0PRI000" || things[0]["heard_in"] != "direct" {
		t.Errorf("the reply is heard as %v, and what was sent is %s", things, ref)
	}
	// A reply under the app's message answers that message.
	under := event("Ev8", map[string]any{"type": "message", "channel": "D0PRI000", "channel_type": "im", "user": "U0PRI000", "text": "under it",
		"ts": "1800000001.000100", "thread_ts": "1700000099.000100"})
	if things, _ := s.hears(delivery(under, "application/json", slackSigner, time.Now())); len(things) != 1 || things[0]["answers"] != "D0PRI000/1700000099.000100" {
		t.Errorf("a reply under a message is heard as %v", things)
	}
	// In a conversation the app has said nothing in, a message answers
	// nothing.
	first := event("Ev9", map[string]any{"type": "message", "channel": "D0DANA00", "channel_type": "im", "user": "U0DANA00", "text": "hello", "ts": "1800000002.000100"})
	if things, _ := s.hears(delivery(first, "application/json", slackSigner, time.Now())); len(things) != 1 || things[0]["answers"] != nil {
		t.Errorf("a first message is heard as %v", things)
	}
}

func testSlackIdentityIsLasting(t *testing.T) {
	_, s := newSlackBridge(t)
	body := event("Ev1", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00",
		"user_profile": map[string]any{"name": "dana.s", "display_name": "Dana"}, "text": "hello", "ts": "1700000001.000100"})
	if things, _ := s.hears(delivery(body, "application/json", slackSigner, time.Now())); len(things) != 1 || things[0]["from"] != "U0DANA00" {
		t.Errorf("Slack: Dana is heard as %v", things)
	}
}

func testSlackLongTextSplit(t *testing.T) {
	sl, s := newSlackBridge(t)
	text := strings.Repeat("A line of a long answer from the office.\n", 2500)
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": text}))
	joined := ""
	for _, p := range sl.posted {
		joined += p["text"].(string)
	}
	if len(sl.posted) != 3 || joined != text {
		t.Errorf("Slack: the text is posted in %d messages, joining to %d of %d", len(sl.posted), len(joined), len(text))
	}
	sl.posted = nil
	sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": strings.Repeat("Why. ", 1000), "choices": []any{"yes", "no"}, "ref": "r1"}))
	if len(sl.posted) != 2 || sl.posted[1]["blocks"] == nil {
		t.Errorf("Slack: a long decision is posted as %d messages", len(sl.posted))
	}
}

func testSlackErrorsByCode(t *testing.T) {
	sl, s := newSlackBridge(t)
	sl.fail = func(method string) (string, bool) { return "channel_not_found", method == "chat.postMessage" }
	if r := s.carry(map[string]any{"to": "C0GONE00/1.0", "text": "hello"}); r.Status != "done" || r.Output.(map[string]any)["refused"] == nil {
		t.Errorf("Slack: channel_not_found ends %s with %#v", r.Status, r.Output)
	}
	sl.fail = func(method string) (string, bool) { return "ratelimited", true }
	if r := s.carry(map[string]any{"to": "C0HELP00/1.0", "text": "hello"}); r.Status != "failed" {
		t.Errorf("Slack: ratelimited ends %s with %#v", r.Status, r.Output)
	}
	sl.fail = nil
	if r := s.carry(map[string]any{"to": "U0GONE00", "direct": true, "text": "hello"}); r.Status != "done" || r.Output.(map[string]any)["refused"] == nil {
		t.Errorf("Slack: a person Slack does not know ends %s with %#v", r.Status, r.Output)
	}
}

func testSlackReplyToGoneThread(t *testing.T) {
	sl, s := newSlackBridge(t)
	sl.gone["1700000001.000100"] = true
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": "after the thread's start was deleted"}))
	if len(sl.posted) != 1 || sl.posted[0]["thread_ts"] != nil || sl.posted[0]["channel"] != "C0HELP00" {
		t.Errorf("Slack: the text is posted as %v", sl.posted)
	}
}
