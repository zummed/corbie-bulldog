package bridges_test

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
	"unicode/utf16"
)

// The token the stand-in Telegram takes, as the Corbie secret holds it.
const telegramToken = "123456:AAH-the-bots-own-token"

// A telegram stands in for Telegram's Bot API: it keeps the updates a
// test has it hear, and what the bot sent, and answers as Telegram does.
type telegram struct {
	*httptest.Server
	t       *testing.T
	mu      sync.Mutex
	updates []map[string]any
	next    int
	message int
	// messages are those the chats hold, by chat:id.
	messages map[string]bool
	sent     []map[string]any
	calls    []string
	// fail, when set, answers a method in place of Telegram, with an
	// error code, a description and a status.
	fail func(method string) (int, string, bool)
	// echo has every call fail with the address it was called at.
	echo bool
}

func newTelegram(t *testing.T) *telegram {
	tg := &telegram{t: t, next: 100, message: 50, messages: map[string]bool{}}
	tg.Server = httptest.NewServer(http.HandlerFunc(tg.serve))
	t.Cleanup(tg.Close)
	return tg
}

// hear has the bot receive an update, which is given its id.
func (tg *telegram) hear(kind string, update map[string]any) int {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.next++
	if m, ok := update["chat"]; ok {
		tg.message++
		update["message_id"] = tg.message
		tg.messages[fmt.Sprintf("%v:%d", m.(map[string]any)["id"], tg.message)] = true
	}
	tg.updates = append(tg.updates, map[string]any{"update_id": tg.next, kind: update})
	return tg.next
}

// says has someone say something to the bot directly.
func (tg *telegram) says(id int, username, text string) int {
	return tg.hear("message", map[string]any{"from": map[string]any{"id": id, "is_bot": false, "username": username},
		"chat": map[string]any{"id": id, "type": "private"}, "text": text})
}

func (tg *telegram) serve(w http.ResponseWriter, r *http.Request) {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	answer := func(status int, v map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	if tg.echo {
		answer(500, map[string]any{"ok": false, "error_code": 500, "description": "Internal Server Error at " + r.URL.String()})
		return
	}
	if file, ok := strings.CutPrefix(r.URL.Path, "/file/bot"+telegramToken+"/"); ok {
		w.Write([]byte("the bytes of " + file))
		return
	}
	path, ok := strings.CutPrefix(r.URL.Path, "/bot"+telegramToken+"/")
	if !ok {
		answer(401, map[string]any{"ok": false, "error_code": 401, "description": "Unauthorized"})
		return
	}
	tg.calls = append(tg.calls, path)
	if tg.fail != nil {
		if code, description, ok := tg.fail(path); ok {
			answer(code, map[string]any{"ok": false, "error_code": code, "description": description})
			return
		}
	}
	var body map[string]any
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.ParseMultipartForm(1 << 20)
		body = map[string]any{"chat_id": r.FormValue("chat_id"), "document": r.MultipartForm.File["document"][0].Filename}
	} else {
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
	}
	ok200 := func(result any) { answer(200, map[string]any{"ok": true, "result": result}) }
	switch path {
	case "getUpdates":
		offset := int(body["offset"].(float64))
		switch {
		case offset < 0 && len(tg.updates) > 0:
			tg.updates = tg.updates[len(tg.updates)-1:]
		case offset > 0:
			tg.updates = slices.DeleteFunc(tg.updates, func(u map[string]any) bool { return u["update_id"].(int) < offset })
		}
		ok200(append([]map[string]any{}, tg.updates...))
	case "sendMessage", "sendDocument":
		chat := fmt.Sprint(body["chat_id"])
		text, _ := body["text"].(string)
		switch {
		case chat == "403":
			answer(403, map[string]any{"ok": false, "error_code": 403, "description": "Forbidden: bot was blocked by the user"})
			return
		case path == "sendMessage" && (len(utf16.Encode([]rune(text))) > 4096 || text == ""):
			answer(400, map[string]any{"ok": false, "error_code": 400, "description": "Bad Request: message is too long"})
			return
		}
		if reply, ok := body["reply_parameters"].(map[string]any); ok {
			if !tg.messages[fmt.Sprintf("%s:%v", chat, reply["message_id"])] && reply["allow_sending_without_reply"] != true {
				answer(400, map[string]any{"ok": false, "error_code": 400, "description": "Bad Request: message to be replied not found"})
				return
			}
		}
		tg.message++
		tg.messages[fmt.Sprintf("%s:%d", chat, tg.message)] = true
		body["method"] = path
		tg.sent = append(tg.sent, body)
		ok200(map[string]any{"message_id": tg.message, "chat": map[string]any{"id": chat}})
	case "answerCallbackQuery":
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
	return tg, newService(t, pointed(t, "telegram", tg.Server, "api.telegram.org"), map[string]string{"TELEGRAM_TOKEN": telegramToken})
}

func listen(cursor any) map[string]any {
	return map[string]any{"settings": map[string]any{"places": map[string]any{"direct": map[string]any{}}}, "cursor": cursor}
}

func TestBridgesTelegramFirstRunHearsWhatComesAfter(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	for _, text := range []string{"old one", "old two", "old three"} {
		tg.says(4471, "sam", text)
	}
	things, cursor := s.heard(listen(nil))
	if len(things) != 0 || cursor != float64(104) {
		t.Fatalf("the first run heard %v, ending with the cursor %v", things, cursor)
	}
	tg.says(4471, "sam", "the kitchen socket sparked")
	things, cursor = s.heard(listen(cursor))
	if len(things) != 1 || things[0]["text"] != "the kitchen socket sparked" || things[0]["from"] != "4471" || things[0]["heard_in"] != "direct" ||
		things[0]["place"] != "4471" || things[0]["kind"] != "said" || cursor != float64(105) {
		t.Errorf("the next run heard %v, ending with %v", things, cursor)
	}
}

func TestBridgesTelegramChoicesAreButtons(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	_, cursor := s.heard(listen(nil))
	ref := sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": "Cut the kitchen circuit?", "choices": []any{"yes", "no"}, "ref": "r12"}))
	if len(tg.sent) != 1 {
		t.Fatalf("Telegram was sent %v", tg.sent)
	}
	keyboard, _ := json.Marshal(tg.sent[0]["reply_markup"])
	if string(keyboard) != `{"inline_keyboard":[[{"callback_data":"r12:0","text":"yes"},{"callback_data":"r12:1","text":"no"}]]}` || fmt.Sprint(tg.sent[0]["chat_id"]) != "4471" {
		t.Errorf("the decision is sent as %v", tg.sent[0])
	}
	message, _ := strings.CutPrefix(ref, "4471:")
	tg.hear("callback_query", map[string]any{"id": "q1", "from": map[string]any{"id": 4471, "username": "sam"}, "data": "r12:0",
		"message": map[string]any{"message_id": message, "chat": map[string]any{"id": 4471, "type": "private"}, "reply_markup": tg.sent[0]["reply_markup"]}})
	things, _ := s.heard(listen(cursor))
	if len(things) != 1 || things[0]["kind"] != "choice" || things[0]["choice"] != "yes" || things[0]["answers"] != "r12" || things[0]["from"] != "4471" || things[0]["place"] != "4471" {
		t.Errorf("Sam's press is heard as %v", things)
	}
	if !slices.Contains(tg.calls, "answerCallbackQuery") {
		t.Errorf("the press was not answered: %v", tg.calls)
	}
	// A wordy choice still fits the 64 bytes Telegram takes in a button's
	// data, and its press is heard as the choice whole.
	wordy := "Refund the second charge of the twelfth and tell the customer by email"
	sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": "Which?", "choices": []any{"Refund the first", wordy}, "ref": "r123456789"}))
	markup := tg.sent[len(tg.sent)-1]["reply_markup"].(map[string]any)
	for _, b := range markup["inline_keyboard"].([]any)[0].([]any) {
		if data := b.(map[string]any)["callback_data"].(string); len(data) > 64 {
			t.Errorf("a button's data is %d bytes", len(data))
		}
	}
	_, cursor = s.heard(listen(cursor))
	tg.hear("callback_query", map[string]any{"id": "q2", "from": map[string]any{"id": 4471}, "data": "r123456789:1",
		"message": map[string]any{"message_id": 1, "chat": map[string]any{"id": 4471, "type": "private"}, "reply_markup": markup}})
	if things, _ := s.heard(listen(cursor)); len(things) != 1 || things[0]["choice"] != wordy || things[0]["answers"] != "r123456789" {
		t.Errorf("the wordy press is heard as %v", things)
	}
	// A question asks for a reply, and the reply answers it.
	ref = sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": "Which socket?", "choices": []any{}, "ref": "r13"}))
	if markup, _ := json.Marshal(tg.sent[len(tg.sent)-1]["reply_markup"]); string(markup) != `{"force_reply":true}` {
		t.Errorf("the question is sent as %v", tg.sent[len(tg.sent)-1])
	}
	_, cursor = s.heard(listen(cursor))
	_, id, _ := strings.Cut(ref, ":")
	tg.hear("message", map[string]any{"from": map[string]any{"id": 4471}, "chat": map[string]any{"id": 4471, "type": "private"}, "text": "the kitchen",
		"reply_to_message": map[string]any{"message_id": atoi(id)}})
	if things, _ := s.heard(listen(cursor)); len(things) != 1 || things[0]["answers"] != ref {
		t.Errorf("the reply is heard as %v", things)
	}
}

func atoi(s string) int {
	n := 0
	fmt.Sscan(s, &n)
	return n
}

func TestBridgesShippedIdentityIsLasting(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	_, cursor := s.heard(listen(nil))
	tg.says(4471, "sam_h", "one")
	tg.says(4471, "sam_house", "two")
	things, _ := s.heard(listen(cursor))
	if len(things) != 2 || things[0]["from"] != "4471" || things[1]["from"] != "4471" {
		t.Errorf("Sam is heard as %v", things)
	}
	testSlackIdentityIsLasting(t)
	testGitLabIdentityIsLasting(t)
	testJiraIdentityIsLasting(t)
}

func TestBridgesShippedLongTextSplit(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	text := strings.Repeat("A line of the answer, with a 🔌 in it.\n", 300)
	sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": text}))
	var joined string
	for _, m := range tg.sent {
		joined += m["text"].(string)
	}
	if len(tg.sent) < 3 || joined != text {
		t.Errorf("the text is sent in %d messages, which make %d characters of %d", len(tg.sent), len(joined), len(text))
	}
	// Text with no line or space in it is cut between characters, never
	// inside one.
	tg.sent = nil
	text = strings.Repeat("🔌", 5000)
	sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": text}))
	joined = ""
	for _, m := range tg.sent {
		joined += m["text"].(string)
	}
	if joined != text {
		t.Error("the plugs were cut")
	}
	testSlackLongTextSplit(t)
	testGitLabLongTextSplit(t)
	testJiraLongTextSplit(t)
}

func TestBridgesEveryMessageAtAListedPlaceHandedOver(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	_, cursor := s.heard(listen(nil))
	tg.says(4471, "sam", "the hedge wants cutting")
	group := map[string]any{"id": -1001, "type": "supergroup"}
	tg.says(4472, "kit", "and the lawn")
	bot := tg.hear("message", map[string]any{"from": map[string]any{"id": 777, "is_bot": true}, "chat": group, "text": "the words of another bot"})
	tg.mu.Lock()
	botMessage := tg.message
	tg.mu.Unlock()
	edited := tg.hear("edited_message", map[string]any{"from": map[string]any{"id": 4471, "is_bot": false}, "chat": group, "text": "the words edited"})
	things, _, logged := s.heardLogged(listen(cursor))
	want := []string{fmt.Sprintf("left out -1001:%d in -1001: bot", botMessage), fmt.Sprintf("left out update %d in -1001: edited_message", edited)}
	if len(things) != 2 || things[0]["text"] != "the hedge wants cutting" || things[1]["text"] != "and the lawn" || fmt.Sprint(logged) != fmt.Sprint(want) {
		t.Errorf("Telegram: what is heard is %v, and %d and %d are logged as %q", things, bot, edited, logged)
	}
	testSlackEveryMessageHandedOver(t)
	testGitLabEveryMessageHandedOver(t)
	testJiraEveryMessageHandedOver(t)
}

func TestBridgesTelegramThreadlessEveryMessageKept(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	_, cursor := s.heard(listen(nil))
	tg.says(4471, "sam", "the kitchen socket sparked")
	if things, _ := s.heard(listen(cursor)); len(things) != 1 || things[0]["threadless"] != true {
		t.Errorf("Sam's message is heard as %v", things)
	}
	text := strings.Repeat("A line of the answer.\n", 400)
	r := s.carry(map[string]any{"to": "4471", "direct": true, "text": text, "files": []file{{"receipt.pdf", []byte("%PDF")}}})
	sent(t, r)
	got, _ := r.Output.(map[string]any)["sent"].([]any)
	if len(tg.sent) < 3 || len(got) != len(tg.sent) {
		t.Fatalf("%d messages were sent, and the run names %v", len(tg.sent), got)
	}
	for i, ref := range got {
		if !tg.messages[ref.(string)] || i > 0 && ref == got[i-1] {
			t.Errorf("the run names %v", got)
		}
	}
}

func TestBridgesShippedErrorsByCode(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	// The words say the opposite of the codes: the codes are what count.
	tg.fail = func(method string) (int, string, bool) {
		if method != "sendMessage" {
			return 0, "", false
		}
		return 403, "Too Many Requests: retry after 5", true
	}
	r := s.carry(map[string]any{"to": "4471", "direct": true, "text": "hello"})
	if out, _ := r.Output.(map[string]any); r.Status != "done" || out["refused"] == nil {
		t.Errorf("a 403 ends %s with %#v", r.Status, r.Output)
	}
	tg.fail = func(method string) (int, string, bool) { return 429, "Forbidden: bot was blocked by the user", true }
	if r := s.carry(map[string]any{"to": "4471", "direct": true, "text": "hello"}); r.Status != "failed" {
		t.Errorf("a 429 ends %s with %#v", r.Status, r.Output)
	}
	testSlackErrorsByCode(t)
	testGitLabErrorsByCode(t)
	testJiraErrorsByCode(t)
}

func TestBridgesShippedReplyToGoneMessageStillSent(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": "after the first was deleted", "answers": "4471:999"}))
	if len(tg.sent) != 1 || tg.sent[0]["text"] != "after the first was deleted" {
		t.Errorf("Telegram was sent %v", tg.sent)
	}
	testSlackReplyToGoneThread(t)
}

func TestBridgesShippedErrorsHoldNoKey(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	tg.echo = true
	in := s.run("in.yaml", listen(nil))
	out := s.carry(map[string]any{"to": "4471", "direct": true, "text": "hello"})
	for _, r := range []*struct {
		status, err string
	}{{in.Status, in.Error}, {out.Status, out.Error}} {
		if r.status != "failed" || strings.Contains(r.err, telegramToken) || strings.Contains(r.err, "AAH-the") || !strings.Contains(r.err, "TELEGRAM_TOKEN") {
			t.Errorf("a run ended %s: %s", r.status, r.err)
		}
	}
	// A refusal whose words repeat the address is masked by Corbie.
	tg.echo = false
	tg.fail = func(method string) (int, string, bool) {
		return 400, "Bad Request: wrong chat in /bot" + telegramToken + "/sendMessage", true
	}
	r := s.carry(map[string]any{"to": "4471", "direct": true, "text": "hello"})
	refused, _ := r.Output.(map[string]any)["refused"].(string)
	if refused == "" || strings.Contains(refused, telegramToken) {
		t.Errorf("the refusal is %q", refused)
	}
}

func TestBridgesShippedKeysAreCorbieSecrets(t *testing.T) {
	t.Parallel()
	for name, secret := range map[string][]string{
		"telegram": {"secret:TELEGRAM_TOKEN@net:api.telegram.org"},
		"slack":    {"secret:SLACK_TOKEN@net:slack.com", "secret:SLACK_APP_TOKEN@net:slack.com"},
		"gitlab":   {"secret:GITLAB_TOKEN@net:${input.settings.site}"},
		"jira":     {"secret:JIRA_TOKEN@net:${input.settings.site}"},
	} {
		files := folder(t, name)
		for _, s := range secret {
			if !strings.Contains(files["in.yaml"]+files["out.yaml"], s) {
				t.Errorf("%s's machines do not name %s", name, s)
			}
		}
		if strings.Contains(files[name+".js"], "env.") || strings.Contains(files[name+".js"], `require("env")`) {
			t.Errorf("%s's script reads the environment", name)
		}
	}
}

func TestBridgesTelegramFilesBothWays(t *testing.T) {
	t.Parallel()
	tg, s := newTelegramBridge(t)
	_, cursor := s.heard(listen(nil))
	tg.hear("message", map[string]any{"from": map[string]any{"id": 4471}, "chat": map[string]any{"id": 4471, "type": "private"}, "caption": "this socket",
		"photo": []any{map[string]any{"file_id": "small", "file_size": 10}, map[string]any{"file_id": "large", "file_size": 900}}})
	ended := s.run("in.yaml", listen(cursor))
	if ended.Status != "done" {
		t.Fatalf("the in run ended %s: %s", ended.Status, ended.Error)
	}
	thing := ended.Output.(map[string]any)["heard"].([]any)[0].(map[string]any)
	files := thing["files"].([]any)
	if thing["text"] != "this socket" || len(files) != 1 {
		t.Fatalf("the photo is heard as %v", thing)
	}
	photo := files[0].(map[string]any)
	blob, _ := photo["data"].(map[string]any)
	if photo["name"] == "" || blob["$blob"] == nil {
		t.Fatalf("the photo is heard as %#v", photo)
	}
	data, _, err := s.corbie.Fetch(t.Context(), ended.ID, blob["$blob"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(data); string(got) != "the bytes of photos/large.jpg" {
		t.Errorf("the photo holds %q", got)
	}
	sent(t, s.carry(map[string]any{"to": "4471", "direct": true, "text": "the receipt", "files": []file{{"receipt.pdf", []byte("%PDF")}}}))
	if len(tg.sent) != 2 || tg.sent[1]["method"] != "sendDocument" || tg.sent[1]["document"] != "receipt.pdf" {
		t.Errorf("Telegram was sent %v", tg.sent)
	}
	// A file Telegram refuses to give is left out and named, and what is
	// heard after it is heard all the same.
	_, cursor = s.heard(listen(cursor))
	tg.fail = func(method string) (int, string, bool) {
		return 400, "Bad Request: file is too big", method == "getFile"
	}
	tg.hear("message", map[string]any{"from": map[string]any{"id": 4471}, "chat": map[string]any{"id": 4471, "type": "private"}, "caption": "the big one",
		"document": map[string]any{"file_id": "big", "file_name": "video.mp4", "file_size": 900}})
	tg.says(4471, "sam", "and after it")
	things, _ := s.heard(listen(cursor))
	if len(things) != 2 || !strings.Contains(things[0]["text"].(string), "video.mp4") || len(things[0]["files"].([]any)) != 0 || things[1]["text"] != "and after it" {
		t.Errorf("a file Telegram refuses is heard as %v", things)
	}
}
