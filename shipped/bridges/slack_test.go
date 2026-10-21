package bridges_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// The bot token and the app-level token the stand-in Slack holds, and the
// app's own user id there.
const (
	slackToken    = "xoxb-the-apps-own-token"
	slackAppToken = "xapp-the-apps-level-token"
	slackApp      = "U0APP000"
)

// A slack stands in for Slack's Web API and its Socket Mode: it knows a
// few channels and who is in them, keeps what the app posted, answers as
// Slack does, and sends a connection the envelopes a test gives it,
// telling the test each one acknowledged.
type slack struct {
	*httptest.Server
	t      *testing.T
	s      *service
	mu     sync.Mutex
	posted []map[string]any
	ts     int
	// asked counts the calls of each method.
	asked map[string]int
	// gone are threads whose first message was deleted.
	gone map[string]bool
	// fail, when set, answers a method in place of Slack, with an error.
	fail func(method string) (string, bool)
	// envelopes are sent to the connection open, and the id of each
	// envelope it acknowledges is sent to acks.
	envelopes chan map[string]any
	acks      chan string
}

func newSlack(t *testing.T) *slack {
	sl := &slack{t: t, ts: 1700000000, gone: map[string]bool{}, asked: map[string]int{}, envelopes: make(chan map[string]any), acks: make(chan string, 16)}
	sl.Server = httptest.NewServer(http.HandlerFunc(sl.serve))
	t.Cleanup(sl.Close)
	return sl
}

func (sl *slack) serve(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/link/") {
		sl.connected(w, r)
		return
	}
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
	case r.URL.Path == "/api/apps.connections.open":
		sl.asked["apps.connections.open"]++
		if r.Header.Get("Authorization") != "Bearer "+slackAppToken {
			answer(map[string]any{"ok": false, "error": "invalid_auth"})
			return
		}
		answer(map[string]any{"ok": true, "url": "ws://" + strings.TrimPrefix(sl.URL, "http://") + "/link/?ticket=t1"})
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
	sl.asked[method]++
	switch method {
	case "auth.test":
		answer(map[string]any{"ok": true, "user_id": slackApp, "bot_id": "B01", "user": "gromit"})
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
		markdown, _ := body["markdown_text"].(string)
		thread, _ := body["thread_ts"].(string)
		blocks, _ := body["blocks"].([]any)
		inBlocks, charts, unfetched := 0, 0, false
		for _, b := range blocks {
			switch b := b.(map[string]any); b["type"] {
			case "markdown":
				inBlocks += len([]rune(b["text"].(string)))
			case "data_visualization":
				charts++
			case "image":
				unfetched = unfetched || strings.Contains(b["image_url"].(string), "unreachable")
			}
		}
		switch {
		case body["markdown_text"] != nil && (body["text"] != nil || body["blocks"] != nil):
			answer(map[string]any{"ok": false, "error": "markdown_text_conflict"})
			return
		case len([]rune(text)) > 40000 || len([]rune(markdown)) > 12000:
			answer(map[string]any{"ok": false, "error": "msg_too_long"})
			return
		case inBlocks > 12000 || charts > 2 || len(blocks) > 50 || unfetched:
			answer(map[string]any{"ok": false, "error": "invalid_blocks"})
			return
		case sl.gone[thread]:
			answer(map[string]any{"ok": false, "error": "thread_not_found"})
			return
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

// connected is a Socket Mode connection, as Slack keeps one: it says
// hello, then sends each envelope the test gives it, and tells the test
// the id of each envelope acknowledged, until the bridge closes it.
func (sl *slack) connected(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := context.Background()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var ack struct {
				EnvelopeID string `json:"envelope_id"`
			}
			json.Unmarshal(data, &ack)
			sl.acks <- ack.EnvelopeID
		}
	}()
	send := func(v map[string]any) {
		data, _ := json.Marshal(v)
		conn.Write(ctx, websocket.MessageText, data)
	}
	send(map[string]any{"type": "hello", "num_connections": 1})
	for {
		select {
		case <-closed:
			return
		case e := <-sl.envelopes:
			send(e)
		}
	}
}

// newSlackBridge gives the shipped Slack bridge, pointed at a stand-in
// Slack, in a Corbie of the test's own.
func newSlackBridge(t *testing.T) (*slack, *service) {
	sl := newSlack(t)
	sl.s = newService(t, pointed(t, "slack", sl.Server, "files.slack.com", "slack.com", "wss-primary.slack.com"),
		map[string]string{"SLACK_TOKEN": slackToken, "SLACK_APP_TOKEN": slackAppToken})
	return sl, sl.s
}

// envelopes counts the envelopes the stand-ins send, so each has an id of
// its own.
var envelopes struct {
	sync.Mutex
	n int
}

// envelope gives what Slack sends over a connection: a payload of a kind,
// in an envelope with an id of its own.
func envelope(kind string, payload any) map[string]any {
	envelopes.Lock()
	defer envelopes.Unlock()
	envelopes.n++
	return map[string]any{"envelope_id": fmt.Sprintf("env-%d", envelopes.n), "type": kind, "payload": payload, "accepts_response_payload": false}
}

// event gives an envelope of an event with an id.
func event(id string, e map[string]any) map[string]any {
	return envelope("events_api", map[string]any{"type": "event_callback", "event_id": id, "event": e})
}

// hears has a listening run of the in machine sent envelopes, and gives
// what it handed over.
func (sl *slack) hears(sent ...map[string]any) []map[string]any {
	sl.t.Helper()
	things, _, _ := sl.hearsLogged(sent...)
	return things
}

// hearsLogged has a listening run of the in machine sent envelopes, each
// once the one before is acknowledged, and then Slack's warning that it
// will close the connection, and gives what it handed over, the lines it
// logged and the ids of the envelopes acknowledged.
func (sl *slack) hearsLogged(sent ...map[string]any) ([]map[string]any, []string, []string) {
	sl.t.Helper()
	run := sl.s.start("in.yaml", map[string]any{"settings": map[string]any{"places": map[string]any{"#help": map[string]any{"guests": true}}}, "cursor": nil})
	var acks []string
	for _, e := range sent {
		sl.send(e)
		acks = append(acks, sl.acked())
	}
	sl.send(map[string]any{"type": "disconnect", "reason": "warning"})
	ended := sl.s.corbie.Ended(sl.t, run)
	out, _ := ended.Output.(map[string]any)
	if ended.Status != "done" || out["cursor"] == nil || len(out["heard"].([]any)) != 0 {
		sl.t.Fatalf("the in run ended %s with %v: %s", ended.Status, ended.Output, ended.Error)
	}
	return sl.handed(run), sl.s.logged(run), acks
}

// send sends the connection open an envelope.
func (sl *slack) send(e map[string]any) {
	sl.t.Helper()
	select {
	case sl.envelopes <- e:
	case <-time.After(testkit.Patience):
		sl.t.Fatalf("no in run listened for %v", e)
	}
}

// acked gives the id of the next envelope acknowledged.
func (sl *slack) acked() string {
	sl.t.Helper()
	select {
	case ack := <-sl.acks:
		return ack
	case <-time.After(testkit.Patience):
		sl.t.Fatal("no envelope was acknowledged")
	}
	return ""
}

// handed gives the things a run handed over, as its record holds them.
func (sl *slack) handed(run string) []map[string]any {
	sl.t.Helper()
	var things []map[string]any
	_, err := sl.s.corbie.Record(context.Background(), run, func(e client.Entry) {
		if e.Job != "" && e.Name == "heard" {
			for _, t := range e.Data.([]any) {
				things = append(things, t.(map[string]any))
			}
		}
	})
	if err != nil {
		sl.t.Fatal(err)
	}
	return things
}

func TestBridgesSlackEveryEnvelopeAcknowledged(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	run := s.start("in.yaml", map[string]any{"settings": map[string]any{}, "cursor": nil})
	// A message is handed over while the run goes on, and acknowledged.
	said := event("Ev1", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": "hello", "ts": "1700000001.000100"})
	sl.send(said)
	if ack := sl.acked(); ack != said["envelope_id"] {
		t.Errorf("the message is acknowledged as %q", ack)
	}
	s.corbie.Until(t, "the message is handed over", func() bool { return len(sl.handed(run)) == 1 })
	if r, err := s.corbie.Show(context.Background(), run); err != nil || r.HasEnded() {
		t.Fatalf("the run %s has ended after one message: %v", run, err)
	}
	// What is left out is acknowledged too, as is every kind of envelope.
	left := []map[string]any{
		event("Ev2", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "bot_id": "B07", "text": "a bot", "ts": "1700000002.000100"}),
		envelope("slash_commands", map[string]any{"command": "/refund", "text": "4411", "channel_id": "C0HELP00"}),
		envelope("interactive", map[string]any{"type": "view_submission"}),
	}
	for _, e := range left {
		sl.send(e)
		if ack := sl.acked(); ack != e["envelope_id"] {
			t.Errorf("%v is acknowledged as %q", e["type"], ack)
		}
	}
	sl.send(map[string]any{"type": "disconnect", "reason": "warning"})
	ended := s.corbie.Ended(t, run)
	sl.mu.Lock()
	connections := sl.asked["apps.connections.open"]
	sl.mu.Unlock()
	if ended.Status != "done" || len(sl.handed(run)) != 1 || connections != 1 {
		t.Errorf("the run ended %s with %v, having handed over %v", ended.Status, ended.Output, sl.handed(run))
	}
}

func TestBridgesSlackPlaceIsTheThread(t *testing.T) {
	t.Parallel()
	sl, _ := newSlackBridge(t)
	top := event("Ev1", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": "at the top", "ts": "1700000001.000100"})
	under := event("Ev2", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0AL0000", "text": "under it", "ts": "1700000009.000100", "thread_ts": "1700000001.000100"})
	a := sl.hears(top)
	b := sl.hears(under)
	if len(a) != 1 || len(b) != 1 || a[0]["place"] != "C0HELP00/1700000001.000100" || b[0]["place"] != a[0]["place"] || a[0]["heard_in"] != "#help" || b[0]["heard_in"] != "#help" {
		t.Errorf("the top is heard as %v, and the reply as %v", a, b)
	}
	if readers, _ := json.Marshal(a[0]["readers"]); string(readers) != `["U0DANA00","U0AL0000","U0GUEST0"]` || a[0]["id"] != "Ev1" {
		t.Errorf("the top is heard as %v", a[0])
	}
	// What is said to the app directly is the sender's direct place.
	dm := event("Ev3", map[string]any{"type": "message", "channel": "D0DANA00", "channel_type": "im", "user": "U0DANA00", "text": "to you", "ts": "1700000010.000100"})
	if c := sl.hears(dm); len(c) != 1 || c[0]["heard_in"] != "direct" || c[0]["place"] != "U0DANA00" {
		t.Errorf("a direct message is heard as %v", c)
	}
	// What the app itself posted is not heard.
	own := event("Ev4", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "bot_id": "B01", "text": "the answer", "ts": "1700000011.000100"})
	if c := sl.hears(own); len(c) != 0 {
		t.Errorf("the app's own message is heard as %v", c)
	}
	// A file said is read out of Slack.
	file := event("Ev5", map[string]any{"type": "message", "subtype": "file_share", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": "this",
		"ts": "1700000012.000100", "files": []any{map[string]any{"name": "receipt.png", "url_private": sl.URL + "/files/T01/receipt.png"}}})
	c := sl.hears(file)
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
	press := envelope("interactive", map[string]any{"type": "block_actions", "user": map[string]any{"id": "U0LEE000", "username": "lee"},
		"channel":   map[string]any{"id": "D0LEE000"},
		"container": map[string]any{"message_ts": "1700000001.000100", "channel_id": "D0LEE000"},
		"actions":   []any{map[string]any{"action_id": "choice-0", "value": "r12:yes", "action_ts": "1700000002.1"}}})
	things := sl.hears(press)
	if len(things) != 1 || things[0]["kind"] != "choice" || things[0]["choice"] != "yes" || things[0]["answers"] != "r12" ||
		things[0]["from"] != "U0LEE000" || things[0]["heard_in"] != "direct" || things[0]["place"] != "U0LEE000" {
		t.Errorf("Lee's press is heard as %v", things)
	}
	// An answer goes into its thread, with its files.
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": "Duplicate confirmed.",
		"files": []file{{"ledger.csv", []byte("a,b")}}}))
	if last := sl.posted[len(sl.posted)-1]; last["channel"] != "C0HELP00" || last["thread_ts"] != "1700000001.000100" || !strings.Contains(last["markdown_text"].(string), "ledger.csv") {
		t.Errorf("the answer is posted as %v", last)
	}
}

func TestBridgesSlackDirectReplyAnswersWhatWasSent(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	// A question carried to Priya directly, and her typed reply in her
	// conversation with the app.
	ref := sent(t, s.carry(map[string]any{"to": "U0PRI000", "direct": true, "text": "Which card?", "choices": []any{}, "ref": "r7"}))
	reply := event("Ev7", map[string]any{"type": "message", "channel": "D0PRI000", "channel_type": "im", "user": "U0PRI000", "text": "the visa", "ts": "1800000000.000100"})
	things := sl.hears(reply)
	if len(things) != 1 || things[0]["answers"] != ref || things[0]["place"] != "U0PRI000" || things[0]["heard_in"] != "direct" {
		t.Errorf("the reply is heard as %v, and what was sent is %s", things, ref)
	}
	// A reply under the app's message answers that message.
	under := event("Ev8", map[string]any{"type": "message", "channel": "D0PRI000", "channel_type": "im", "user": "U0PRI000", "text": "under it",
		"ts": "1800000001.000100", "thread_ts": "1700000099.000100"})
	if things := sl.hears(under); len(things) != 1 || things[0]["answers"] != "D0PRI000/1700000099.000100" {
		t.Errorf("a reply under a message is heard as %v", things)
	}
	// In a conversation the app has said nothing in, a message answers
	// nothing.
	first := event("Ev9", map[string]any{"type": "message", "channel": "D0DANA00", "channel_type": "im", "user": "U0DANA00", "text": "hello", "ts": "1800000002.000100"})
	if things := sl.hears(first); len(things) != 1 || things[0]["answers"] != nil {
		t.Errorf("a first message is heard as %v", things)
	}
}

func testSlackIdentityIsLasting(t *testing.T) {
	sl, _ := newSlackBridge(t)
	body := event("Ev1", map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00",
		"user_profile": map[string]any{"name": "dana.s", "display_name": "Dana"}, "text": "hello", "ts": "1700000001.000100"})
	if things := sl.hears(body); len(things) != 1 || things[0]["from"] != "U0DANA00" {
		t.Errorf("Slack: Dana is heard as %v", things)
	}
}

func testSlackLongTextSplit(t *testing.T) {
	sl, s := newSlackBridge(t)
	text := strings.Repeat("A line of a long answer from the office.\n", 600)
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": text}))
	joined := ""
	for _, p := range sl.posted {
		joined += p["markdown_text"].(string)
	}
	if len(sl.posted) != 3 || joined != text {
		t.Errorf("Slack: the text is posted in %d messages, joining to %d of %d", len(sl.posted), len(joined), len(text))
	}
	sl.posted = nil
	why := strings.Repeat("Why. ", 3000)
	sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": why, "choices": []any{"yes", "no"}, "ref": "r1"}))
	if len(sl.posted) != 2 || sl.posted[1]["blocks"] == nil ||
		sl.posted[0]["markdown_text"].(string)+sl.posted[1]["blocks"].([]any)[0].(map[string]any)["text"].(string) != why {
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

func TestBridgesSlackMentionOfTheAppMarked(t *testing.T) {
	t.Parallel()
	sl, _ := newSlackBridge(t)
	said := func(id, text string) map[string]any {
		t.Helper()
		body := event(id, map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": text, "ts": "1700000001.000100"})
		things := sl.hears(body)
		if len(things) != 1 {
			t.Fatalf("%q is heard as %v", text, things)
		}
		return things[0]
	}
	asked := func() int {
		sl.mu.Lock()
		defer sl.mu.Unlock()
		return sl.asked["auth.test"]
	}
	for text, marked := range map[string]bool{
		"<@" + slackApp + "> is the API down?": true,
		"is it down, <@" + slackApp + "|bot>?": true,
		"<@U0RAJ000> can you look?":            false,
	} {
		got := said("Ev-"+text, text)
		if mentioned, given := got["mentioned"]; given != marked || given && mentioned != true {
			t.Errorf("%q is heard as %v", text, got)
		}
	}
	if n := asked(); n != 3 {
		t.Errorf("Slack was asked who the app is %d times for three texts that name someone", n)
	}
	if got := said("Ev9", "is the API down?"); got["mentioned"] != nil || asked() != 3 {
		t.Errorf("a text that names nobody is heard as %v, and Slack was asked who the app is %d times", got, asked())
	}
}

func testSlackEveryMessageHandedOver(t *testing.T) {
	sl, _ := newSlackBridge(t)
	at := func(id string, e map[string]any) map[string]any {
		e["channel"], e["channel_type"] = "C0HELP00", "channel"
		return event(id, e)
	}
	for id, e := range map[string]map[string]any{
		"Ev1": {"type": "message", "user": "U0DANA00", "text": "at the top", "ts": "1700000001.000100"},
		"Ev2": {"type": "message", "user": "U0AL0000", "text": "under it", "ts": "1700000002.000100", "thread_ts": "1700000001.000100"},
		"Ev3": {"type": "message", "subtype": "thread_broadcast", "user": "U0AL0000", "text": "under it, and in the channel", "ts": "1700000003.000100", "thread_ts": "1700000001.000100"},
		"Ev4": {"type": "message", "subtype": "file_share", "user": "U0GUEST0", "text": "this", "ts": "1700000004.000100",
			"files": []any{map[string]any{"name": "receipt.png", "url_private": sl.URL + "/files/T01/receipt.png"}}},
		"Ev9": {"type": "message", "subtype": "me_message", "user": "U0DANA00", "text": "is looking", "ts": "1700000009.000100"},
	} {
		if things, logged, _ := sl.hearsLogged(at(id, e)); len(things) != 1 || things[0]["id"] != id || len(logged) != 0 {
			t.Errorf("Slack: %s is heard as %v, and logged %q", id, things, logged)
		}
	}
	for id, e := range map[string]map[string]any{
		"Ev5": {"type": "message", "bot_id": "B07", "user": "U0OTHER0", "text": "the words of another bot", "ts": "1700000005.000100"},
		"Ev6": {"type": "message", "bot_id": "B01", "text": "the app's own words", "ts": "1700000006.000100"},
		"Ev7": {"type": "message", "subtype": "message_changed", "message": map[string]any{"user": "U0DANA00", "text": "the words edited"}, "ts": "1700000007.000100"},
		"Ev8": {"type": "member_joined_channel", "user": "U0RAJ000"},
	} {
		want := map[string]string{"Ev5": "bot", "Ev6": "bot", "Ev7": "message_changed", "Ev8": "member_joined_channel"}[id]
		if things, logged, _ := sl.hearsLogged(at(id, e)); len(things) != 0 || fmt.Sprint(logged) != "[left out "+id+" in C0HELP00: "+want+"]" {
			t.Errorf("Slack: %s is heard as %v, and logged %q", id, things, logged)
		}
	}
}

func TestBridgesSlackMarkdownShown(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	text := "**Refunded** order 2231.\n# What was done\n- one charge\nSee [the ledger](https://ledger.example.com/2231)."
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": text}))
	if len(sl.posted) != 1 || sl.posted[0]["markdown_text"] != text || sl.posted[0]["text"] != nil || sl.posted[0]["blocks"] != nil {
		t.Fatalf("an answer is posted as %v", sl.posted)
	}
	// With buttons, the Markdown is a block beside them, and a short
	// plain text stands in where Slack shows no blocks.
	sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": text, "choices": []any{"yes", "no"}, "ref": "r3"}))
	last := sl.posted[len(sl.posted)-1]
	blocks, _ := last["blocks"].([]any)
	if len(blocks) != 2 || last["markdown_text"] != nil || last["text"] != "**Refunded** order 2231." {
		t.Fatalf("a decision is posted as %v", last)
	}
	if shown := blocks[0].(map[string]any); shown["type"] != "markdown" || shown["text"] != text || blocks[1].(map[string]any)["type"] != "actions" {
		t.Errorf("a decision's blocks are %v", blocks)
	}
	// Slack refusing Markdown beside other text is refused once, by its
	// code, and not tried again.
	tries := 0
	sl.fail = func(method string) (string, bool) {
		if method != "chat.postMessage" {
			return "", false
		}
		tries++
		return "markdown_text_conflict", true
	}
	r := s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": text})
	if refused, _ := r.Output.(map[string]any)["refused"].(string); r.Status != "done" || !strings.Contains(refused, "markdown_text_conflict") || tries != 1 {
		t.Errorf("markdown_text_conflict ends %s with %#v after %d tries", r.Status, r.Output, tries)
	}
}

func TestBridgesSlackAppMentionIsItsName(t *testing.T) {
	t.Parallel()
	sl, _ := newSlackBridge(t)
	for text, want := range map[string]string{
		"<@" + slackApp + "> what's the refund policy?":     "@gromit what's the refund policy?",
		"ask <@" + slackApp + "|gromit> and <@U0RAJ000>":    "ask @gromit and <@U0RAJ000>",
		"<@U0RAJ000> can you look?":                         "<@U0RAJ000> can you look?",
		"<@" + slackApp + "> and again <@" + slackApp + ">": "@gromit and again @gromit",
	} {
		said := event("Ev-"+text, map[string]any{"type": "message", "channel": "C0HELP00", "channel_type": "channel", "user": "U0DANA00", "text": text, "ts": "1700000001.000100"})
		if things := sl.hears(said); len(things) != 1 || things[0]["text"] != want || things[0]["from"] != "U0DANA00" {
			t.Errorf("%q is heard as %v", text, things)
		}
	}
}

func TestBridgesSlackChartsAndImagesShown(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	chart := "```chart\n" + `{"type": "bar", "title": "Refunds by week", "categories": ["W1", "W2", "W3"],
 "series": [{"name": "Refunds", "values": [4, 7, 3]}, {"name": "Charges", "values": [40, 52, 47]}], "y_label": "Count"}` + "\n```\n"
	image := "![The refunds dashboard](https://ledger.example.com/refunds.png)\n"
	text := "**Refunds** are down this week.\n\n" + chart + "\n" + image + "See [the ledger](https://ledger.example.com)."
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": text}))
	if len(sl.posted) != 1 || sl.posted[0]["markdown_text"] != nil || sl.posted[0]["text"] != "**Refunds** are down this week." {
		t.Fatalf("an answer with a chart is posted as %v", sl.posted)
	}
	shown, _ := json.Marshal(sl.posted[0]["blocks"])
	want := `[{"text":"**Refunds** are down this week.","type":"markdown"},` +
		`{"chart":{"axis_config":{"categories":["W1","W2","W3"],"y_label":"Count"},"series":[` +
		`{"data":[{"label":"W1","value":4},{"label":"W2","value":7},{"label":"W3","value":3}],"name":"Refunds"},` +
		`{"data":[{"label":"W1","value":40},{"label":"W2","value":52},{"label":"W3","value":47}],"name":"Charges"}],"type":"bar"},` +
		`"title":"Refunds by week","type":"data_visualization"},` +
		`{"alt_text":"The refunds dashboard","image_url":"https://ledger.example.com/refunds.png","type":"image"},` +
		`{"text":"See [the ledger](https://ledger.example.com).","type":"markdown"}]`
	if string(shown) != want {
		t.Errorf("the blocks are\n%s\nnot\n%s", shown, want)
	}

	// A pie is a slice for each category.
	sl.posted = nil
	pie := "```chart\n" + `{"type": "pie", "title": "Where refunds go", "categories": ["Card", "Bank"], "series": [{"name": "Refunds", "values": [9, 3]}]}` + "\n```"
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": pie}))
	if shown, _ := json.Marshal(sl.posted[0]["blocks"]); string(shown) != `[{"chart":{"segments":[{"label":"Card","value":9},{"label":"Bank","value":3}],"type":"pie"},"title":"Where refunds go","type":"data_visualization"}]` ||
		sl.posted[0]["text"] != "Where refunds go" {
		t.Errorf("a pie is posted as %v", sl.posted)
	}

	// A chart Slack could not show as written, and any other fence, are
	// said as written.
	for _, written := range []string{
		"```chart\n{\"type\": \"bar\", \"title\": \"Refunds\", \"categories\": [\"a week far too long to be a label\"], \"series\": [{\"name\": \"n\", \"values\": [1]}]}\n```",
		"```chart\n{\"type\": \"pie\", \"title\": \"Refunds\", \"categories\": [\"a\"], \"series\": [{\"name\": \"n\", \"values\": [0]}]}\n```",
		"```chart\nnot JSON\n```",
		"```chart\n{\"type\": \"bar\", \"categories\": [\"a\"], \"series\": [{\"name\": \"n\", \"values\": [1]}]}\n```",
		"```chart\n{\"type\": \"bar\", \"title\": \"T\", \"categories\": [\"a\", \"a\"], \"series\": [{\"name\": \"n\", \"values\": [1, 2]}]}\n```",
		"```chart\n{\"type\": \"bar\", \"title\": \"T\", \"categories\": [\"a\", \"b\"], \"series\": [{\"name\": \"n\", \"values\": [1]}]}\n```",
		"    " + image,
		"```chart\n{\"type\": \"bar\"}",
		"```json\n" + `{"type": "bar", "title": "T", "categories": ["a"], "series": [{"name": "n", "values": [1]}]}` + "\n```",
		"```\n" + image + "```",
	} {
		sl.posted = nil
		sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": written}))
		if len(sl.posted) != 1 || sl.posted[0]["markdown_text"] != written {
			t.Errorf("%q is posted as %v", written, sl.posted)
		}
	}

	// Three charts go two to a message, in order, and a decision's buttons
	// are beside the last.
	sl.posted = nil
	three := "First.\n" + chart + "Second.\n" + chart + "Third.\n" + chart + "Refund them?"
	sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": three, "choices": []any{"yes", "no"}, "ref": "r4"}))
	kinds := [][]string{}
	for _, p := range sl.posted {
		kind := []string{}
		for _, b := range p["blocks"].([]any) {
			kind = append(kind, b.(map[string]any)["type"].(string))
		}
		kinds = append(kinds, kind)
	}
	if fmt.Sprint(kinds) != "[[markdown data_visualization markdown data_visualization markdown] [data_visualization markdown actions]]" {
		t.Errorf("three charts are posted as %v", kinds)
	}

	// An image Slack cannot fetch is said as written, the message it was in
	// with it.
	sl.posted = nil
	gone := "The dashboard:\n![the dashboard](https://unreachable.example.com/d.png)\nas it was."
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": gone}))
	if len(sl.posted) != 1 || sl.posted[0]["markdown_text"] != gone {
		t.Errorf("an image Slack cannot fetch is posted as %v", sl.posted)
	}
	// In a decision, it is said with the buttons beside it, and what was
	// sent is that message.
	sl.posted = nil
	ref := sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": gone, "choices": []any{"yes", "no"}, "ref": "r9"}))
	if len(sl.posted) != 1 || len(sl.posted[0]["blocks"].([]any)) != 2 || sl.posted[0]["blocks"].([]any)[0].(map[string]any)["text"] != gone ||
		ref != "D0LEE000/"+sl.posted[0]["ts"].(string) {
		t.Errorf("a decision with an image Slack cannot fetch is posted as %v, and sent as %s", sl.posted, ref)
	}

	// Markdown beside charts goes on in the next message past 12,000
	// characters, and images past 49 blocks, leaving room for the buttons.
	sl.posted = nil
	long := strings.Repeat("a", 6000) + "\n" + chart + strings.Repeat("b", 6001)
	sent(t, s.carry(map[string]any{"to": "C0HELP00/1700000001.000100", "text": long}))
	if len(sl.posted) != 2 || len(sl.posted[0]["blocks"].([]any)) != 2 || len(sl.posted[1]["blocks"].([]any)) != 1 {
		t.Errorf("long Markdown beside a chart is posted as %v", sl.posted)
	}
	sl.posted = nil
	sent(t, s.carry(map[string]any{"to": "U0LEE000", "direct": true, "text": strings.Repeat(image, 60), "choices": []any{"yes"}, "ref": "r10"}))
	if len(sl.posted) != 2 || len(sl.posted[0]["blocks"].([]any)) != 49 || len(sl.posted[1]["blocks"].([]any)) != 12 {
		t.Errorf("60 images are posted as %d messages", len(sl.posted))
	}
}
