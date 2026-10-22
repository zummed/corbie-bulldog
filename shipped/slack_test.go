package shipped_test

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

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// The bot token and the app-level token the stand-in Slack takes, and the
// app's own user id there.
const (
	slackToken    = "xoxb-the-apps-own-token"
	slackAppToken = "xapp-the-apps-level-token"
	slackApp      = "U0APP000"
)

// A slack stands in for Slack's Web API and its Socket Mode: it knows a
// few channels, keeps what the app posted, answers as Slack does, and
// sends a connection the envelopes a test gives it, telling the test each
// one acknowledged. It gives every file but one under /files/gone/, and
// names every channel but a private one, whose id begins G.
type slack struct {
	*httptest.Server
	t      *testing.T
	mu     sync.Mutex
	posted []map[string]any
	ts     int
	asked  map[string]int
	// gone are threads whose first message was deleted.
	gone map[string]bool
	// fail, when it gives true, answers a method in place of Slack, with an
	// error.
	fail      func(method string) (string, bool)
	envelopes chan map[string]any
	acks      chan string
	stop      chan struct{}
}

func newSlack(t *testing.T) *slack {
	sl := &slack{t: t, ts: 1700000000, gone: map[string]bool{}, asked: map[string]int{},
		envelopes: make(chan map[string]any), acks: make(chan string, 16), stop: make(chan struct{})}
	sl.Server = httptest.NewServer(http.HandlerFunc(sl.serve))
	t.Cleanup(func() {
		close(sl.stop)
		sl.Close()
	})
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
	if strings.HasPrefix(r.URL.Path, "/files/") {
		sl.asked["files"]++
		switch {
		case r.Header.Get("Authorization") != "Bearer "+slackToken:
			w.WriteHeader(403)
		case strings.HasPrefix(r.URL.Path, "/files/gone/"):
			w.WriteHeader(404)
		default:
			w.Write([]byte("the file " + r.URL.Path))
		}
		return
	}
	method, _ := strings.CutPrefix(r.URL.Path, "/api/")
	sl.asked[method]++
	if method == "apps.connections.open" {
		if r.Header.Get("Authorization") != "Bearer "+slackAppToken {
			answer(map[string]any{"ok": false, "error": "invalid_auth"})
			return
		}
		answer(map[string]any{"ok": true, "url": "ws://" + strings.TrimPrefix(sl.URL, "http://") + "/link/?ticket=t1"})
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+slackToken {
		answer(map[string]any{"ok": false, "error": "invalid_auth"})
		return
	}
	if sl.fail != nil {
		if code, failed := sl.fail(method); failed {
			answer(map[string]any{"ok": false, "error": code})
			return
		}
	}
	var body map[string]any
	data, _ := io.ReadAll(r.Body)
	json.Unmarshal(data, &body)
	switch method {
	case "auth.test":
		answer(map[string]any{"ok": true, "user_id": slackApp, "bot_id": "B01", "user": "gromit"})
	case "conversations.info":
		channel := r.URL.Query().Get("channel")
		if strings.HasPrefix(channel, "G") {
			answer(map[string]any{"ok": false, "error": "missing_scope"})
			return
		}
		answer(map[string]any{"ok": true, "channel": map[string]any{"id": channel, "name": map[string]string{"C0HELP00": "help", "C0REFUND": "refunds", "C0GENERL": "general"}[channel], "is_im": strings.HasPrefix(channel, "D")}})
	case "conversations.open":
		users := fmt.Sprint(body["users"])
		if users == "U0GONE00" {
			answer(map[string]any{"ok": false, "error": "user_not_found"})
			return
		}
		answer(map[string]any{"ok": true, "channel": map[string]any{"id": "D" + users[1:]}})
	case "chat.postMessage":
		markdown, _ := body["markdown_text"].(string)
		thread, _ := body["thread_ts"].(string)
		blocks, _ := body["blocks"].([]any)
		unfetched, badButton := false, false
		for _, b := range blocks {
			b := b.(map[string]any)
			if b["type"] == "image" {
				unfetched = unfetched || strings.Contains(b["image_url"].(string), "unreachable")
			}
			if b["type"] == "actions" {
				for _, button := range b["elements"].([]any) {
					text := button.(map[string]any)["text"].(map[string]any)["text"].(string)
					badButton = badButton || text == "" || len([]rune(text)) > 75
				}
			}
		}
		switch {
		case body["markdown_text"] != nil && (body["text"] != nil || body["blocks"] != nil):
			answer(map[string]any{"ok": false, "error": "markdown_text_conflict"})
			return
		case len([]rune(markdown)) > 12000:
			answer(map[string]any{"ok": false, "error": "msg_too_long"})
			return
		case unfetched || badButton:
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
	default:
		answer(map[string]any{"ok": false, "error": "unknown_method"})
	}
}

// connected is a Socket Mode connection: it says hello, then sends each
// envelope the test gives it, and tells the test the id of each envelope
// acknowledged, until either side closes it, Slack warns it will, or the
// test ends.
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
		case <-sl.stop:
			return
		case e := <-sl.envelopes:
			send(e)
			// Slack closes a connection after it warns it will.
			if e["type"] == "disconnect" {
				return
			}
		}
	}
}

// newSlackBridge gives the shipped Slack bridge, pointed at a stand-in
// Slack, in a Corbie of the test's own.
func newSlackBridge(t *testing.T) (*slack, *service) {
	sl := newSlack(t)
	return sl, newService(t, "slack", sl.Server, map[string]string{"SLACK_TOKEN": slackToken, "SLACK_APP_TOKEN": slackAppToken},
		map[string]any{"places": map[string]any{"direct": map[string]any{}, "#help": map[string]any{"guests": true}}},
		"files.slack.com", "slack.com", "wss-primary.slack.com")
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

// message gives an event of a message said at a channel.
func message(channel, user, ts, text string) map[string]any {
	kind := "channel"
	if strings.HasPrefix(channel, "D") {
		kind = "im"
	}
	return map[string]any{"type": "message", "channel": channel, "channel_type": kind, "user": user, "text": text, "ts": ts}
}

// send sends an envelope over the connection open, and stays until it is
// acknowledged.
func (sl *slack) send(e map[string]any) {
	sl.t.Helper()
	select {
	case sl.envelopes <- e:
	case <-time.After(testkit.Patience):
		sl.t.Fatalf("no in run listened for %v", e)
	}
	select {
	case ack := <-sl.acks:
		if ack != e["envelope_id"] {
			sl.t.Errorf("%v is acknowledged as %q", e["envelope_id"], ack)
		}
	case <-time.After(testkit.Patience):
		sl.t.Fatalf("%v was not acknowledged", e["envelope_id"])
	}
}

func TestSlackHearsAMessageAndAPress(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	run := s.listen(nil)
	// Dana asks in #help, Al answers under it, and Dana names the app.
	sl.send(event("Ev1", message("C0HELP00", "U0DANA00", "1700000001.000100", "Was order 4411 refunded?")))
	under := message("C0HELP00", "U0AL0000", "1700000009.000100", "I think so")
	under["thread_ts"] = "1700000001.000100"
	sl.send(event("Ev2", under))
	sl.send(event("Ev3", message("C0HELP00", "U0DANA00", "1700000010.000100", "<@"+slackApp+"> was it?")))
	// Lee says something to the app directly, with a file.
	file := message("D0LEE000", "U0LEE000", "1700000011.000100", "this one")
	file["subtype"], file["files"] = "file_share", []any{map[string]any{"name": "receipt.png", "url_private": sl.URL + "/files/T01/receipt.png"}}
	sl.send(event("Ev4", file))
	things, cursor := s.hears(run, 4)
	var got []string
	for _, th := range things {
		got = append(got, fieldsOf(th, "id", "at", "kind", "from", "place", "where", "text", "mentioned", "answers", "ref", "choice"))
	}
	want := []string{
		"C0HELP00/1700000001.000100 | 2023-11-14T22:13:21.000Z | said | U0DANA00 | #help | C0HELP00/1700000001.000100 | Was order 4411 refunded? | false | <nil> | <nil> | <nil>",
		"C0HELP00/1700000009.000100 | 2023-11-14T22:13:29.000Z | said | U0AL0000 | #help | C0HELP00/1700000001.000100 | I think so | false | C0HELP00/1700000001.000100 | <nil> | <nil>",
		"C0HELP00/1700000010.000100 | 2023-11-14T22:13:30.000Z | said | U0DANA00 | #help | C0HELP00/1700000010.000100 | @gromit was it? | true | <nil> | <nil> | <nil>",
		"D0LEE000/1700000011.000100 | 2023-11-14T22:13:31.000Z | said | U0LEE000 | direct | D0LEE000/1700000011.000100 | this one | false | <nil> | <nil> | <nil>",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || cursor != nil {
		t.Errorf("Slack hands over\n%s\nnot\n%s\nwith the cursor %v", strings.Join(got, "\n"), strings.Join(want, "\n"), cursor)
	}
	if files := things[3]["files"].([]any); len(files) != 1 || files[0].(map[string]any)["name"] != "receipt.png" || files[0].(map[string]any)["data"] == nil {
		t.Errorf("the file is heard as %v", files)
	}
	// A decision to Lee directly has a button for each choice, and his
	// press is heard with its ref and choice.
	id := sent(t, s.say(map[string]any{"to": "U0LEE000", "direct": true, "text": "Refund one charge?", "choices": []any{"yes", "no"}, "ref": "r12"}))
	sl.mu.Lock()
	question := sl.posted[0]
	sl.mu.Unlock()
	actions, _ := json.Marshal(question["blocks"].([]any)[1])
	if question["channel"] != "D0LEE000" || id != "D0LEE000/"+question["ts"].(string) || !strings.Contains(string(actions), `"value":"r12:yes"`) || !strings.Contains(string(actions), `"value":"r12:no"`) {
		t.Errorf("the decision is posted as %v, and named %s", question, id)
	}
	sl.send(envelope("interactive", map[string]any{"type": "block_actions", "user": map[string]any{"id": "U0LEE000"},
		"channel":   map[string]any{"id": "D0LEE000"},
		"container": map[string]any{"message_ts": question["ts"], "channel_id": "D0LEE000"},
		"actions":   []any{map[string]any{"action_id": "choice-0", "value": "r12:yes", "action_ts": "1700000020.000100"}}}))
	things, _ = s.hears(run, 5)
	if got := fieldsOf(things[4], "id", "kind", "from", "place", "where", "text", "files", "ref", "choice"); got != "press:U0LEE000:1700000020.000100 | press | U0LEE000 | direct | "+id+" |  | [] | r12 | yes" {
		t.Errorf("Lee's press is heard as %s", got)
	}
}

// A file Slack gives no address for, one too large and one Slack will not
// give are left out and named, and the message is heard. A file said at a
// place the settings do not list is not read.
func TestSlackLeavesOutAFile(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	run := s.listen(nil)
	files := message("D0LEE000", "U0LEE000", "1700000001.000100", "here they are")
	files["subtype"], files["files"] = "file_share", []any{
		map[string]any{"name": "connect.pdf", "file_access": "check_file_info"},
		map[string]any{"name": "gone.png", "url_private": sl.URL + "/files/gone/gone.png"},
		map[string]any{"name": "video.mp4", "size": 200 << 20, "url_private": sl.URL + "/files/T01/video.mp4"},
		map[string]any{"name": "receipt.png", "url_private": sl.URL + "/files/T01/receipt.png"},
	}
	general := message("C0GENERL", "U0LEE000", "1700000002.000100", "the team photo")
	general["subtype"], general["files"] = "file_share", []any{map[string]any{"name": "team.png", "url_private": sl.URL + "/files/T01/team.png"}}
	sl.send(event("Ev1", files))
	sl.send(event("Ev2", general))
	things, _ := s.hears(run, 2)
	want := "here they are\n[a file left out: connect.pdf, as Slack gives no address for it]\n[a file left out: gone.png, as Slack answered status 404]\n[a file left out: video.mp4, as too large]"
	if got := things[0]["files"].([]any); things[0]["text"] != want || len(got) != 1 || got[0].(map[string]any)["name"] != "receipt.png" {
		t.Errorf("the message is heard saying %q, with the files %v", things[0]["text"], got)
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if got := things[1]["files"].([]any); things[1]["place"] != "#general" || len(got) != 0 || sl.asked["files"] != 2 {
		t.Errorf("at #general the files %v are heard, and Slack was asked for %d files", got, sl.asked["files"])
	}
}

// What is said in a channel Slack will not name is left out, and what
// comes after it is heard. Slack is asked about each channel once a job.
func TestSlackLeavesOutWhatItCannotPlace(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	run := s.listen(nil)
	sl.send(event("Ev1", message("G0PRIVAT", "U0DANA00", "1700000001.000100", "in a private channel")))
	sl.send(event("Ev2", message("C0HELP00", "U0DANA00", "1700000002.000100", "one")))
	sl.send(event("Ev3", message("C0HELP00", "U0DANA00", "1700000003.000100", "two")))
	things, _ := s.hears(run, 2)
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if len(things) != 2 || things[0]["text"] != "one" || fmt.Sprint(s.logged(run)) != "[left out Ev1 in G0PRIVAT: Slack refused conversations.info: missing_scope]" || sl.asked["conversations.info"] != 2 {
		t.Errorf("Slack hands over %v, logs %q, and was asked about %d channels", things, s.logged(run), sl.asked["conversations.info"])
	}
}

func TestSlackListensAgainWhenSlackCloses(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	run := s.listen(nil)
	sl.send(event("Ev1", message("C0HELP00", "U0DANA00", "1700000001.000100", "one")))
	// Slack warns that it will close the connection: the job ends, and
	// the next connects again.
	sl.envelopes <- map[string]any{"type": "disconnect", "reason": "warning"}
	sl.send(event("Ev2", message("C0HELP00", "U0DANA00", "1700000002.000100", "two")))
	things, _ := s.hears(run, 2)
	sl.mu.Lock()
	defer sl.mu.Unlock()
	texts := []string{things[0]["text"].(string), things[1]["text"].(string)}
	if fmt.Sprint(texts) != "[one two]" || sl.asked["apps.connections.open"] != 2 {
		t.Errorf("Slack hands over %v, over %d connections", texts, sl.asked["apps.connections.open"])
	}
}

func TestSlackSaysOneMessage(t *testing.T) {
	t.Parallel()
	sl, s := newSlackBridge(t)
	last := func() map[string]any {
		sl.mu.Lock()
		defer sl.mu.Unlock()
		return sl.posted[len(sl.posted)-1]
	}
	// An answer goes in its thread, as Markdown.
	text := "**Refunded** order 4411.\n- one charge\nSee [the ledger](https://ledger.example.com/4411)."
	id := sent(t, s.say(map[string]any{"to": "C0HELP00/1700000001.000100", "text": text}))
	if m := last(); m["channel"] != "C0HELP00" || m["thread_ts"] != "1700000001.000100" || m["markdown_text"] != text || id != "C0HELP00/"+m["ts"].(string) {
		t.Errorf("the answer is posted as %v, and named %s", m, id)
	}
	// A thread whose first message is gone: the answer goes in the channel.
	sl.mu.Lock()
	sl.gone["1700000002.000100"] = true
	sl.mu.Unlock()
	sent(t, s.say(map[string]any{"to": "C0HELP00/1700000002.000100", "text": "after the thread was deleted"}))
	if m := last(); m["thread_ts"] != nil || m["channel"] != "C0HELP00" {
		t.Errorf("the answer is posted as %v", m)
	}
	// A long text goes in as many messages as it takes.
	long := strings.Repeat("A line of a long answer from the office.\n", 600)
	sl.mu.Lock()
	before := len(sl.posted)
	sl.mu.Unlock()
	sent(t, s.say(map[string]any{"to": "C0HELP00/1700000001.000100", "text": long}))
	sl.mu.Lock()
	joined := ""
	for _, p := range sl.posted[before:] {
		joined += p["markdown_text"].(string)
	}
	n := len(sl.posted) - before
	sl.mu.Unlock()
	if n != 3 || joined != long {
		t.Errorf("the long text is posted in %d messages", n)
	}
	// A chart fence is shown as Slack's chart, beside the Markdown.
	chart := "Refunds are down.\n```chart\n" + `{"type": "bar", "title": "Refunds by week", "categories": ["W1", "W2"], "series": [{"name": "Refunds", "values": [4, 7]}]}` + "\n```\n"
	sent(t, s.say(map[string]any{"to": "C0HELP00/1700000001.000100", "text": chart}))
	var kinds []string
	for _, b := range last()["blocks"].([]any) {
		kinds = append(kinds, b.(map[string]any)["type"].(string))
	}
	if fmt.Sprint(kinds) != "[markdown data_visualization]" {
		t.Errorf("a chart is posted as %v", kinds)
	}
	// A choice longer than a button shows is cut on its button, and its
	// press gives it back whole.
	policy := "Full refund within 30 days of purchase, including shipping costs and any fees"
	sent(t, s.say(map[string]any{"to": "U0LEE000", "direct": true, "text": "Which refund policy applies?", "choices": []any{policy, "Store credit"}, "ref": "r13"}))
	button := last()["blocks"].([]any)[1].(map[string]any)["elements"].([]any)[0].(map[string]any)
	if shown := button["text"].(map[string]any)["text"].(string); shown != policy[:75] || button["value"] != "r13:"+policy {
		t.Errorf("the long choice is a button showing %q, giving %q", shown, button["value"])
	}
	// What Slack refuses fails the out with Slack's word.
	if r := s.say(map[string]any{"to": "U0GONE00", "direct": true, "text": "hello"}); r.Status != "failed" || !strings.Contains(r.Error, "user_not_found") {
		t.Errorf("a person Slack does not know ends %s: %s", r.Status, r.Error)
	}
	sl.mu.Lock()
	sl.fail = func(method string) (string, bool) { return "channel_not_found", method == "chat.postMessage" }
	sl.mu.Unlock()
	if r := s.say(map[string]any{"to": "C0GONE00/1.0", "text": "hello"}); r.Status != "failed" || !strings.Contains(r.Error, "channel_not_found") || strings.Contains(r.Error, slackToken) {
		t.Errorf("channel_not_found ends %s: %s", r.Status, r.Error)
	}
}

func slackFromIsLasting(t *testing.T) {
	sl, s := newSlackBridge(t)
	run := s.listen(nil)
	said := message("C0HELP00", "U0DANA00", "1700000001.000100", "hello")
	said["user_profile"] = map[string]any{"name": "dana.s", "display_name": "Dana"}
	sl.send(event("Ev1", said))
	if things, _ := s.hears(run, 1); things[0]["from"] != "U0DANA00" {
		t.Errorf("Dana is heard as %v", things[0]["from"])
	}
}

func slackBotsNotHeard(t *testing.T) {
	sl, s := newSlackBridge(t)
	run := s.listen(nil)
	other := message("C0HELP00", "U0OTHER0", "1700000002.000100", "the words of another bot")
	other["bot_id"] = "B07"
	own := message("C0HELP00", "", "1700000003.000100", "the app's own words")
	own["bot_id"] = "B01"
	edited := map[string]any{"type": "message", "subtype": "message_changed", "channel": "C0HELP00", "ts": "1700000004.000100"}
	sl.send(event("Ev5", other))
	sl.send(event("Ev6", own))
	sl.send(event("Ev7", edited))
	sl.send(event("Ev8", message("C0HELP00", "U0DANA00", "1700000005.000100", "and the lawn")))
	things, _ := s.hears(run, 1)
	want := "[left out Ev5 in C0HELP00: a bot said it left out Ev6 in C0HELP00: a bot said it left out Ev7 in C0HELP00: message_changed]"
	if len(things) != 1 || things[0]["text"] != "and the lawn" || fmt.Sprint(s.logged(run)) != want {
		t.Errorf("Slack hands over %v, and logs %q", things, s.logged(run))
	}
}
