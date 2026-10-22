package shipped_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// The token the stand-in Telegram takes, as the Corbie secret holds it,
// and the bot it is.
const (
	telegramToken = "123456:AAH-the-bots-own-token"
	telegramBot   = 777000
)

// A telegram stands in for Telegram's Bot API: it keeps the updates a test
// has the bot receive until they are asked past, holds a long poll until
// one comes, and keeps what the bot sent.
type telegram struct {
	*httptest.Server
	mu       sync.Mutex
	updates  []map[string]any
	next     int
	message  int
	sent     []map[string]any
	calls    []string
	offsets  []int
	answered []string
	// fail, when it gives true, answers a method in place of Telegram, with
	// an error code and a description.
	fail func(method string) (int, string, bool)
	// news is closed when an update comes, and made again.
	news chan struct{}
	stop chan struct{}
}

func newTelegram(t *testing.T) *telegram {
	tg := &telegram{next: 100, message: 50, news: make(chan struct{}), stop: make(chan struct{})}
	tg.Server = httptest.NewServer(http.HandlerFunc(tg.serve))
	t.Cleanup(func() {
		close(tg.stop)
		tg.Close()
	})
	return tg
}

// hear has the bot receive an update, and gives the update's id. A message
// is given its id and its date.
func (tg *telegram) hear(kind string, update map[string]any) int {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.next++
	if _, ok := update["chat"]; ok {
		tg.message++
		update["message_id"] = tg.message
		update["date"] = testkit.Monday.Unix()
	}
	tg.updates = append(tg.updates, map[string]any{"update_id": tg.next, kind: update})
	close(tg.news)
	tg.news = make(chan struct{})
	return tg.next
}

// says has a person say something to the bot in their chat with it.
func (tg *telegram) says(id int, text string) int {
	return tg.hear("message", map[string]any{"from": map[string]any{"id": id, "is_bot": false}, "chat": map[string]any{"id": id, "type": "private"}, "text": text})
}

// lastMessage gives the id of the last message, said or sent.
func (tg *telegram) lastMessage() int {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	return tg.message
}

func (tg *telegram) serve(w http.ResponseWriter, r *http.Request) {
	answer := func(status int, v map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	if file, ok := strings.CutPrefix(r.URL.Path, "/file/bot"+telegramToken+"/"); ok {
		w.Write([]byte("the bytes of " + file))
		return
	}
	method, ok := strings.CutPrefix(r.URL.Path, "/bot"+telegramToken+"/")
	if !ok {
		answer(401, map[string]any{"ok": false, "error_code": 401, "description": "Unauthorized"})
		return
	}
	var body map[string]any
	data, _ := io.ReadAll(r.Body)
	json.Unmarshal(data, &body)
	ok200 := func(result any) { answer(200, map[string]any{"ok": true, "result": result}) }
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.calls = append(tg.calls, method)
	if method == "getUpdates" {
		tg.offsets = append(tg.offsets, int(body["offset"].(float64)))
	}
	if tg.fail != nil {
		if code, description, failed := tg.fail(method); failed {
			answer(code, map[string]any{"ok": false, "error_code": code, "description": description})
			return
		}
	}
	switch method {
	case "getMe":
		ok200(map[string]any{"id": telegramBot, "is_bot": true, "username": "House_bot"})
	case "getUpdates":
		offset := int(body["offset"].(float64))
		if offset < 0 && len(tg.updates) > 0 {
			tg.updates = tg.updates[len(tg.updates)-1:]
		}
		if offset > 0 {
			tg.updates = slices.DeleteFunc(tg.updates, func(u map[string]any) bool { return u["update_id"].(int) < offset })
		}
		wait, _ := body["timeout"].(float64)
		until := time.After(time.Duration(wait * float64(time.Second)))
		for len(tg.updates) == 0 && wait > 0 {
			news := tg.news
			tg.mu.Unlock()
			select {
			case <-news:
			case <-until:
			case <-tg.stop:
			case <-r.Context().Done():
			}
			tg.mu.Lock()
			select {
			case <-news:
			default:
				wait = 0
			}
		}
		ok200(append([]map[string]any{}, tg.updates...))
	case "sendMessage":
		chat := fmt.Sprint(body["chat_id"])
		text, _ := body["text"].(string)
		switch {
		case chat == "403":
			answer(403, map[string]any{"ok": false, "error_code": 403, "description": "Forbidden: bot was blocked by the user"})
			return
		case len(utf16.Encode([]rune(text))) > 4096 || text == "":
			answer(400, map[string]any{"ok": false, "error_code": 400, "description": "Bad Request: message is too long"})
			return
		}
		tg.message++
		body["message_id"] = tg.message
		tg.sent = append(tg.sent, body)
		ok200(map[string]any{"message_id": tg.message, "chat": map[string]any{"id": chat}})
	case "answerCallbackQuery":
		tg.answered = append(tg.answered, fmt.Sprint(body["callback_query_id"]))
		ok200(true)
	case "getFile":
		ok200(map[string]any{"file_id": body["file_id"], "file_path": "photos/" + fmt.Sprint(body["file_id"]) + ".jpg"})
	default:
		answer(404, map[string]any{"ok": false, "error_code": 404, "description": "Not Found: method not found"})
	}
}

// newTelegramBridge gives the shipped Telegram bridge, pointed at a
// stand-in Telegram, in a Corbie of the test's own.
func newTelegramBridge(t *testing.T) (*telegram, *service) {
	tg := newTelegram(t)
	return tg, newService(t, "telegram", tg.Server, map[string]string{"TELEGRAM_TOKEN": telegramToken},
		map[string]any{"places": map[string]any{"direct": map[string]any{}, "-1001": map[string]any{"guests": true}}}, "api.telegram.org")
}

func TestTelegramHearsAMessageAndAPress(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	tg.says(4471, "said before the bridge")
	run := s.listen(nil)
	// A new bridge hears nothing said before it, and emits its cursor.
	if things, cursor := s.hears(run, 0); len(things) != 0 || cursor != float64(102) {
		t.Fatalf("a new bridge heard %v, with the cursor %v", things, cursor)
	}
	// Kit sends the house a photo.
	tg.hear("message", map[string]any{"from": map[string]any{"id": 4472, "is_bot": false}, "chat": map[string]any{"id": 4472, "type": "private"},
		"caption": "The socket by the boiler sparks.", "photo": []any{map[string]any{"file_id": "small", "file_size": 10}, map[string]any{"file_id": "large", "file_size": 900}}})
	things, cursor := s.hears(run, 1)
	photo := things[0]
	if got := fieldsOf(photo, "id", "at", "kind", "from", "place", "where", "text", "mentioned", "answers", "ref", "choice"); got != "4472:52 | 2026-10-05T09:00:00.000Z | said | 4472 | direct | 4472 | The socket by the boiler sparks. | false | <nil> | <nil> | <nil>" || cursor != float64(103) {
		t.Errorf("the photo is heard as %s, with the cursor %v", got, cursor)
	}
	files := photo["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("the photo's files are %v", files)
	}
	file := files[0].(map[string]any)
	blob, _ := file["data"].(map[string]any)
	if file["name"] != "photo-52.jpg" || blob["$blob"] == nil {
		t.Fatalf("the photo is %#v", file)
	}
	data, _, err := s.corbie.Fetch(ctx, run, blob["$blob"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(data); string(got) != "the bytes of photos/large.jpg" {
		t.Errorf("the photo holds %q", got)
	}
	// A question to Kit directly, with a button for each choice; Kit
	// presses yes.
	id := sent(t, s.say(map[string]any{"to": "4472", "direct": true, "text": "May the fixer order the part?", "choices": []any{"yes", "no"}, "ref": "r7f3"}))
	tg.mu.Lock()
	question := tg.sent[0]
	tg.mu.Unlock()
	keyboard, _ := json.Marshal(question["reply_markup"])
	if string(keyboard) != `{"inline_keyboard":[[{"callback_data":"r7f3:0","text":"yes"},{"callback_data":"r7f3:1","text":"no"}]]}` || fmt.Sprint(question["chat_id"]) != "4472" || id != fmt.Sprintf("4472:%d", tg.lastMessage()) {
		t.Errorf("the question is sent as %v, and named %s", question, id)
	}
	tg.hear("callback_query", map[string]any{"id": "q1", "from": map[string]any{"id": 4472, "is_bot": false}, "data": "r7f3:0",
		"message": map[string]any{"message_id": tg.lastMessage(), "chat": map[string]any{"id": 4472, "type": "private"}, "reply_markup": question["reply_markup"]}})
	things, _ = s.hears(run, 2)
	if got := fieldsOf(things[1], "id", "kind", "from", "place", "where", "text", "files", "ref", "choice"); got != "press:q1 | press | 4472 | direct | 4472 |  | [] | r7f3 | yes" {
		t.Errorf("the press is heard as %s", got)
	}
	tg.mu.Lock()
	defer tg.mu.Unlock()
	if !slices.Equal(tg.answered, []string{"q1"}) {
		t.Errorf("the presses answered are %v", tg.answered)
	}
}

func TestTelegramCarriesItsCursor(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	tg.says(4471, "before")
	after := tg.says(4471, "after")
	run := s.listen(float64(after))
	things, cursor := s.hears(run, 1)
	if len(things) != 1 || things[0]["text"] != "after" || cursor != float64(after+1) {
		t.Fatalf("from the cursor %d, the bridge heard %v, with the cursor %v", after, things, cursor)
	}
	// Telegram refuses the ask after "one": the job fails, and the in rests
	// 30 seconds, then hears again from the cursor it was given.
	tg.mu.Lock()
	refused := false
	tg.fail = func(method string) (int, string, bool) {
		if method != "getUpdates" || refused || len(tg.offsets) == 0 || tg.offsets[len(tg.offsets)-1] != after+2 {
			return 0, "", false
		}
		refused = true
		return 429, "Too Many Requests: retry after 5", true
	}
	tg.mu.Unlock()
	tg.says(4471, "one")
	s.hears(run, 2)
	s.rests(run)
	tg.says(4471, "two")
	s.corbie.Clock.Move(30 * time.Second)
	things, cursor = s.hears(run, 4)
	var texts []string
	for _, th := range things {
		texts = append(texts, th["text"].(string))
	}
	tg.mu.Lock()
	defer tg.mu.Unlock()
	if fmt.Sprint(texts) != "[after one one two]" || cursor != float64(after+3) || !slices.Contains(tg.offsets[len(tg.offsets)-2:], after) {
		t.Errorf("the bridge heard %v, with the cursor %v, asking from %v", texts, cursor, tg.offsets)
	}
}

func TestTelegramMentionedWhenTheBotIsNamed(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	group := map[string]any{"id": -1001, "type": "supergroup"}
	said := func(text string, entities []any) {
		tg.hear("message", map[string]any{"from": map[string]any{"id": 4471, "is_bot": false}, "chat": group, "text": text, "entities": entities})
	}
	said("@house_bot is the boiler on?", []any{map[string]any{"type": "mention", "offset": 0, "length": 10}})
	said("ask Sam", []any{map[string]any{"type": "text_mention", "offset": 4, "length": 3, "user": map[string]any{"id": 4471}}})
	said("ask the bot", []any{map[string]any{"type": "text_mention", "offset": 8, "length": 3, "user": map[string]any{"id": telegramBot}}})
	said("@kit_s look", []any{map[string]any{"type": "mention", "offset": 0, "length": 6}})
	things, _ := s.hears(run, 4)
	var got []string
	for _, th := range things {
		got = append(got, fieldsOf(th, "place", "where", "mentioned"))
	}
	if fmt.Sprint(got) != "[-1001 | -1001 | true -1001 | -1001 | false -1001 | -1001 | true -1001 | -1001 | false]" {
		t.Errorf("the messages are heard as %v", got)
	}
}

func TestTelegramSaysOneMessage(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	last := func() map[string]any {
		tg.mu.Lock()
		defer tg.mu.Unlock()
		return tg.sent[len(tg.sent)-1]
	}
	markup := func() string {
		b, _ := json.Marshal(last()["reply_markup"])
		return string(b)
	}
	// An answer in a topic of a group, replying to what it answers.
	sent(t, s.say(map[string]any{"to": "-1001/7", "text": "The hedge is cut.", "answers": "-1001:52"}))
	if m := last(); fmt.Sprintf("%v %v %v %v", m["chat_id"], m["message_thread_id"], m["reply_parameters"], m["reply_markup"]) != "-1001 7 map[allow_sending_without_reply:true message_id:52] <nil>" {
		t.Errorf("the answer is sent as %v", m)
	}
	// A question without choices asks for a reply.
	sent(t, s.say(map[string]any{"to": "4471", "direct": true, "text": "Which socket?", "ref": "r20"}))
	if got := markup(); got != `{"force_reply":true}` {
		t.Errorf("a question is sent with %s", got)
	}
	// Four choices, or long ones, are each on a row of their own.
	sent(t, s.say(map[string]any{"to": "4471", "direct": true, "text": "When may I come?", "choices": []any{"Mon", "Tue", "Wed", "Thu"}, "ref": "r21"}))
	if got := markup(); got != `{"inline_keyboard":[[{"callback_data":"r21:0","text":"Mon"}],[{"callback_data":"r21:1","text":"Tue"}],[{"callback_data":"r21:2","text":"Wed"}],[{"callback_data":"r21:3","text":"Thu"}]]}` {
		t.Errorf("four choices are sent as %s", got)
	}
	// A long text goes in as many messages as it takes, cut between
	// characters, and the last is what was sent.
	text := strings.Repeat("A line of the answer, with a 🔌 in it.\n", 300)
	id := sent(t, s.say(map[string]any{"to": "4471", "direct": true, "text": text}))
	tg.mu.Lock()
	var joined string
	n := 0
	for _, m := range tg.sent[3:] {
		joined += m["text"].(string)
		n++
	}
	tg.mu.Unlock()
	if n < 3 || joined != text || id != fmt.Sprintf("4471:%d", tg.lastMessage()) {
		t.Errorf("the text is sent in %d messages, joined %d of %d characters, and named %s", n, len(joined), len(text), id)
	}
	// What Telegram refuses fails the out with Telegram's words, and never
	// the token.
	r := s.say(map[string]any{"to": "403", "direct": true, "text": "hello"})
	if r.Status != "failed" || !strings.Contains(r.Error, "bot was blocked by the user") || strings.Contains(r.Error, "AAH-the") {
		t.Errorf("a refusal ends %s: %s", r.Status, r.Error)
	}
	tg.mu.Lock()
	tg.fail = func(string) (int, string, bool) {
		return 400, "Bad Request: wrong chat in /bot" + telegramToken + "/sendMessage", true
	}
	tg.mu.Unlock()
	if r := s.say(map[string]any{"to": "4471", "text": "hello"}); r.Status != "failed" || strings.Contains(r.Error, "AAH-the") {
		t.Errorf("a refusal that repeats the address ends %s: %s", r.Status, r.Error)
	}
}

func telegramFromIsLasting(t *testing.T) {
	tg, s := newTelegramBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	for _, name := range []string{"sam_h", "sam_house"} {
		tg.hear("message", map[string]any{"from": map[string]any{"id": 4471, "is_bot": false, "username": name, "first_name": "Sam"}, "chat": map[string]any{"id": 4471, "type": "private"}, "text": "hello"})
	}
	if things, _ := s.hears(run, 2); things[0]["from"] != "4471" || things[1]["from"] != "4471" {
		t.Errorf("Sam is heard as %v and %v", things[0]["from"], things[1]["from"])
	}
}

func telegramBotsNotHeard(t *testing.T) {
	tg, s := newTelegramBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	group := map[string]any{"id": -1001, "type": "supergroup"}
	tg.hear("message", map[string]any{"from": map[string]any{"id": 888, "is_bot": true}, "chat": group, "text": "the words of another bot"})
	bot := tg.lastMessage()
	edited := tg.hear("edited_message", map[string]any{"from": map[string]any{"id": 4471, "is_bot": false}, "chat": group, "text": "the words edited"})
	tg.hear("message", map[string]any{"from": map[string]any{"id": 4471, "is_bot": false}, "chat": group, "text": "the lawn wants cutting"})
	things, _ := s.hears(run, 1)
	want := []string{fmt.Sprintf("left out -1001:%d: a bot said it", bot), fmt.Sprintf("left out update %d: edited_message", edited)}
	if len(things) != 1 || things[0]["text"] != "the lawn wants cutting" || fmt.Sprint(s.logged(run)) != fmt.Sprint(want) {
		t.Errorf("Telegram hands over %v, and logs %q", things, s.logged(run))
	}
}
