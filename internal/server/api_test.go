package server

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestServerEveryRequestOfTheAPI(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(playHouse)
	s := b.serve()
	sam := s.as("sam")
	me := sam.get("/api/me").is(http.StatusOK).json()
	for _, key := range []string{"me", "office", "teams", "people", "waiting", "problems"} {
		if _, ok := me[key]; !ok {
			t.Errorf("/api/me gives no %s: %v", key, me)
		}
	}
	if office := me["office"].(map[string]any); office["name"] != "The house" || office["zone"] != "Europe/London" {
		t.Errorf("/api/me gives the office %v", office)
	}
	// Asked in the chat with a photo, a pump to order, which waits on Sam.
	n := numbered(sam.form("/api/requests", map[string]string{"text": "the boiler pump is dead"}, map[string][]byte{"pump.png": []byte("\x89PNG\r\n\x1a\n the pump")}))
	s.waitsOn(n, "sam")
	if list := sam.get("/api/requests").is(http.StatusOK).json()["requests"].([]any); len(list) != 1 {
		t.Errorf("/api/requests gives %v", list)
	}
	var v struct {
		Entries   []entryOut
		Decisions []decisionOut
		Runs      []struct{ Name, Worker string }
	}
	sam.get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	if len(v.Entries) == 0 || len(v.Decisions) != 1 || len(v.Runs) == 0 || len(v.Entries[0].Files) != 1 {
		t.Fatalf("/api/requests/%d gives %+v", n, v)
	}
	if photo := sam.get(v.Entries[0].Files[0].URL).is(http.StatusOK); string(photo.body) != "\x89PNG\r\n\x1a\n the pump" {
		t.Errorf("the photo is served as %q", photo.body)
	}
	if team := sam.get("/api/teams/office").is(http.StatusOK).json(); team["workers"] == nil {
		t.Errorf("/api/teams/office gives Sam %v", team)
	}
	sam.get("/api/office/files").is(http.StatusOK)
	if text := sam.get("/api/office/file?path=office.yaml").is(http.StatusOK).json()["text"]; !strings.Contains(text.(string), "name: The house") {
		t.Errorf("office.yaml is given as %v", text)
	}
	var history struct{ History []struct{ ID string } }
	sam.get("/api/history").is(http.StatusOK).into(&history)
	if commit := sam.get("/api/history/" + history.History[0].ID).is(http.StatusOK).json(); commit["commit"] == nil || commit["files"] == nil {
		t.Errorf("a commit is given as %v", commit)
	}
	var pages struct{ Pages []pageOut }
	sam.get("/api/manual").is(http.StatusOK).into(&pages)
	if len(pages.Pages) < 2 || pages.Pages[0].Name != "office" || !slices.ContainsFunc(pages.Pages, func(p pageOut) bool { return p.Name == "tools/talk" && p.Text == "" }) {
		t.Errorf("/api/manual gives %+v", pages)
	}
	if page := sam.get("/api/manual/tools/talk").is(http.StatusOK).json(); !strings.Contains(page["text"].(string), "./.bulldog/tools/talk/talk.yaml") {
		t.Errorf("/api/manual/tools/talk gives %v", page)
	}
	sam.get("/api/manual/agent").is(http.StatusNotFound)
	var run runOut
	sam.get("/api/runs/" + url.PathEscape(v.Runs[0].Name)).is(http.StatusOK).into(&run)
	if run.Request != n || run.Team != "office" || run.Worker != v.Runs[0].Worker {
		t.Errorf("/api/runs/%s gives %+v", v.Runs[0].Name, run)
	}
	if e := sam.stream("/api/events").next(); e.name != Hello {
		t.Errorf("the stream of changes begins %+v", e)
	}
	if e := sam.stream(watching(v.Runs[0].Name)).next(); e.id != "1" {
		t.Errorf("the stream of the run begins %+v", e)
	}
	sam.post("/api/requests/"+itoa(n)+"/decisions/"+itoa(v.Decisions[0].ID), map[string]string{"says": "yes"}).is(http.StatusOK)
	s.answered(n)
	sam.post("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": "Thank you."}).is(http.StatusOK)
	sam.post("/api/requests/"+itoa(n)+"/stop", "").is(http.StatusOK)
	s.answered(numbered(sam.post("/api/teams/office/routines/now", map[string]string{"at": "mon 09:00", "to": "steward", "say": "Walk round: what is still open?"})))
	note := map[string]any{"files": map[string]any{"notes/boiler.md": "The pump was changed in October.\n"}}
	check := sam.post("/api/office/check", note).is(http.StatusOK).json()
	note["why"], note["mark"] = "What was done to the boiler.", check["mark"]
	sam.post("/api/office/put", note).is(http.StatusOK)
	sam = s.holding(sam.post("/api/me/key", "").is(http.StatusOK))

	// What is open answers the same with Sam's key and with none.
	for _, open := range []struct{ method, path, body string }{
		{http.MethodPost, "/signin", `{"name":"sam"}`},
		{http.MethodGet, "/signin/0123abcd", ""},
		{http.MethodPost, "/signin/0123abcd", ""},
		{http.MethodPost, "/bridges/telegram", "{}"},
	} {
		with := sam.do(open.method, open.path, "application/json", []byte(open.body))
		without := s.nobody().do(open.method, open.path, "application/json", []byte(open.body))
		if with.status != without.status || !bytes.Equal(with.body, without.body) || with.cookie() != nil {
			t.Errorf("%s %s answers %d %s with a key, and %d %s without", open.method, open.path, with.status, with.body, without.status, without.body)
		}
	}
}

func TestServerSightCheckedOnTheThingNamed(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	// The refund is decided through payments' request, and through no
	// other, whatever its number names there.
	for _, n := range []int64{d.n, d.platform, d.refund} {
		d.s.as("lee").post("/api/requests/"+itoa(n)+"/decisions/"+itoa(d.refund), map[string]string{"says": "yes"}).is(http.StatusNotFound)
	}
	// Dana sees payments' request beneath hers, and not a request of
	// Raj's to platform, whatever she names it by.
	raj := numbered(d.s.as("raj").form("/api/requests", map[string]string{"team": "platform", "text": "is the pipeline green?"}, map[string][]byte{"log.txt": []byte("the pipeline's log")}))
	thread, err := d.s.areasOf().Requests.Thread(raj)
	if err != nil {
		t.Fatal(err)
	}
	first := thread[0].ID
	d.s.as("dana").get("/api/requests/" + itoa(d.payments)).is(http.StatusOK)
	d.s.as("dana").get("/api/requests/" + itoa(raj)).is(http.StatusNotFound)
	d.s.as("dana").get(fmt.Sprintf("/files/%d-0", first)).is(http.StatusNotFound)
	d.s.as("raj").get(fmt.Sprintf("/files/%d-0", first)).is(http.StatusOK)
	d.s.as("al").get(fmt.Sprintf("/files/%d-0", first)).is(http.StatusOK)
	d.s.as("dana").post("/api/requests/"+itoa(d.n)+"/entries", map[string]any{"text": "it is", "re": first}).is(http.StatusNotFound)
}

func TestServerHiddenIsNotThere(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	lee := d.s.as("lee")
	ledger := d.s.runOf("lee", d.payments, "ledger")
	for _, pair := range [][2]string{
		{"/api/requests/" + itoa(d.n), "/api/requests/99999"},
		{"/api/requests/" + itoa(d.platform), "/api/requests/nonsense"},
		{"/api/office/files", "/api/office/nothing"},
		{"/api/history", "/api/history/0000"},
		{fmt.Sprintf("/files/%d-0", d.n), "/files/99999-0"},
	} {
		hidden, missing := lee.get(pair[0]), lee.get(pair[1])
		if hidden.status != http.StatusNotFound || hidden.status != missing.status || !bytes.Equal(hidden.body, missing.body) {
			t.Errorf("%s answers %d %s, and %s %d %s", pair[0], hidden.status, hidden.body, pair[1], missing.status, missing.body)
		}
	}
	hidden, missing := d.s.as("al").stream(watching(ledger)), d.s.as("al").stream(watching("client/nothing"))
	if hidden.res.StatusCode != http.StatusNotFound || missing.res.StatusCode != http.StatusNotFound {
		t.Errorf("a hidden run's watch answers %d, and one not there %d", hidden.res.StatusCode, missing.res.StatusCode)
	}
	for _, path := range []string{"/api/requests/" + itoa(d.n) + "/stop", "/api/requests/99999/stop"} {
		lee.post(path, "").is(http.StatusNotFound)
	}
}

func TestServerEntryOnlyThroughItsRequest(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(playHouse)
	s := b.serve()
	socket := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "the kitchen socket sparked"}))
	question := s.waitsOn(socket, "kit")
	other := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "is the kettle on?"}))
	s.answered(other)
	// Named under another request Kit sees, the question is not there.
	s.as("kit").post("/api/requests/"+itoa(other)+"/entries", map[string]any{"text": "The one by the kettle.", "re": question.Entry}).is(http.StatusNotFound)
	if waits, _ := s.areasOf().Requests.WaitingOn("kit"); len(waits) != 1 {
		t.Errorf("the question was answered through another request: %v", waits)
	}
	s.as("kit").post("/api/requests/"+itoa(socket)+"/entries", map[string]any{"text": "The one by the kettle.", "re": question.Entry}).is(http.StatusOK)
	// A decision, the same.
	pump := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "the boiler pump is dead"}))
	s.waitsOn(pump, "sam")
	ds, err := s.areasOf().Decisions.OfRequest(pump)
	if err != nil || len(ds) != 1 {
		t.Fatalf("the pump's decisions are %v (%v)", ds, err)
	}
	s.as("sam").post("/api/requests/"+itoa(socket)+"/decisions/"+itoa(ds[0].ID), map[string]string{"says": "yes"}).is(http.StatusNotFound)
	if dec, _ := s.areasOf().Decisions.Get(ds[0].ID); dec.Ended != "" {
		t.Errorf("the order was decided through another request: %+v", dec)
	}
}

func TestServerErrorsAreJSON(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	s := b.serve()
	n := numbered(s.as("raj").post("/api/requests", map[string]string{"team": "support", "text": "can you call the customer?"}))
	cases := []struct {
		r      reply
		status int
		says   string
	}{
		{s.as("raj").post("/api/requests", `{"text": `), http.StatusBadRequest, "cannot be read"},
		{s.as("lee").post("/api/requests/"+itoa(n)+"/stop", ""), http.StatusNotFound, notThere},
		{s.as("dana").post("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": strings.Repeat("a", 2<<20)}), http.StatusRequestEntityTooLarge, "1 MiB"},
		{s.nobody().get("/api/me"), http.StatusUnauthorized, "signed in"},
	}
	// A stop the rules refuse: Raj, who reads #help, sees payments'
	// request beneath Dana's, and may not stop it.
	d := refundDay(t, refundOffice(t))
	cases = append(cases, struct {
		r      reply
		status int
		says   string
	}{d.s.as("raj").post("/api/requests/"+itoa(d.payments)+"/stop", ""), http.StatusConflict, "may not stop"})
	for _, c := range cases {
		if c.r.status != c.status || c.r.header.Get("Content-Type") != "application/json" || !strings.Contains(c.r.json()["error"].(string), c.says) {
			t.Errorf("answered %d %s, not %d with %q", c.r.status, c.r.body, c.status, c.says)
		}
	}
	// A store made to fail is a failure, said as one and in the log.
	if _, err := raw(t, b.dir).Exec(`ALTER TABLE requests_requests RENAME TO requests_gone`); err != nil {
		t.Fatal(err)
	}
	r := s.as("raj").get("/api/requests")
	if r.status != http.StatusInternalServerError || !strings.Contains(r.json()["error"].(string), "failed") || !s.logged("a failure in answering GET /api/requests") {
		t.Errorf("a failing store answered %d %s", r.status, r.body)
	}
}

func TestServerFailureNotRefusal(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	s := b.serve()
	n := numbered(s.as("raj").post("/api/requests", map[string]string{"team": "support", "text": "can you call the customer?"}))
	if _, err := raw(t, b.dir).Exec(`ALTER TABLE requests_entries RENAME TO requests_lost`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []reply{
		s.as("raj").get("/api/requests/" + itoa(n)),
		s.as("raj").post("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": "more"}),
		s.as("dana").post("/api/requests/"+itoa(n)+"/stop", ""),
		s.as("raj").post("/api/requests", map[string]string{"text": "again"}),
	} {
		if r.status != http.StatusInternalServerError {
			t.Errorf("a failing store answered %d %s", r.status, r.body)
		}
	}
	// Pressing a one-time link while the store fails is a failure too, and
	// the log names the link by its pattern, never by what it carries.
	if _, err := raw(t, b.dir).Exec(`ALTER TABLE server_links RENAME TO server_lost`); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("ab", 32)
	if r := s.nobody().do(http.MethodPost, "/signin/"+token, "application/x-www-form-urlencoded", nil); r.status != http.StatusInternalServerError {
		t.Errorf("pressing a link while the store fails answered %d %s", r.status, r.body)
	}
	if !s.logged("a failure in answering POST /signin/{link}") || s.logged(token) {
		t.Errorf("the failure is logged as:\n%s", strings.Join(s.lines(), "\n"))
	}
	// Of the changes, an edit the folder cannot take is refused, a change
	// the history does not hold is not there, and a history that cannot
	// be read is a failure.
	al := s.as("al")
	if r := al.post("/api/office/check", map[string]any{"move": map[string]string{"nobody": "support"}}); r.status != http.StatusConflict || !strings.Contains(string(r.body), "there is no worker nobody to move") {
		t.Errorf("moving a worker that is not there answered %d %s", r.status, r.body)
	}
	al.get("/api/history/" + strings.Repeat("0", 40)).is(http.StatusNotFound)
	if err := os.RemoveAll(filepath.Join(b.dir, ".git")); err != nil {
		t.Fatal(err)
	}
	for _, r := range []reply{al.get("/api/history"), al.get("/api/history/" + strings.Repeat("0", 40))} {
		if r.status != http.StatusInternalServerError {
			t.Errorf("a history that cannot be read answered %d %s", r.status, r.body)
		}
	}
}

func TestServerBodyReadStrictly(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	kit := s.as("kit")
	n := numbered(kit.post("/api/requests", map[string]string{"text": "is the kettle on?"}))
	s.answered(n)
	count := func() int {
		list, err := s.areasOf().Requests.List("kit", requests.Filter{})
		if err != nil {
			t.Fatal(err)
		}
		thread, err := s.areasOf().Requests.Thread(n)
		if err != nil {
			t.Fatal(err)
		}
		return len(list)*1000 + len(thread)
	}
	before := count()
	for _, bad := range []struct{ path, body string }{
		{"/api/requests", `{"text": "hello"`},
		{"/api/requests", `{"text": "hello", "urgent": true}`},
		{"/api/requests", `{"text": 5}`},
		{"/api/requests", `{"team": "garden"}`},
		{"/api/requests", `{"text": null}`},
		{"/api/requests", `{"text": "one"} {"text": "two"}`},
		{"/api/requests", ``},
		{"/api/requests/" + itoa(n) + "/entries", `{"text": "more", "re": "seven"}`},
		{"/api/requests/" + itoa(n) + "/entries", `{"re": 7}`},
		{"/api/requests/" + itoa(n) + "/stop", `{"why": "enough"}`},
		{"/api/requests/" + itoa(n) + "/decisions/1", `{"says": "maybe"}`},
		{"/api/teams/office/routines/now", `{"at": "mon 09:00", "to": "steward"}`},
		{"/api/office/check", `{"files": {"a.md": 7}}`},
		{"/api/requests", `{"text": "one", "text": "two"}`},
		{"/api/requests/" + itoa(n) + "/entries", `{"text": "more", "re": 7, "re": 8}`},
		{"/api/office/check", `{"files": {"a.md": "one", "a.md": "two"}}`},
	} {
		if r := kit.post(bad.path, bad.body); r.status != http.StatusBadRequest {
			t.Errorf("%s with %s answered %d %s", bad.path, bad.body, r.status, r.body)
		}
	}
	if r := kit.do(http.MethodPost, "/api/requests", "text/plain", []byte(`{"text": "hello"}`)); r.status != http.StatusBadRequest {
		t.Errorf("a body that is not said to be JSON answered %d", r.status)
	}
	if r := kit.form("/api/requests", map[string]string{"text": "hello", "urgent": "yes"}, nil); r.status != http.StatusBadRequest {
		t.Errorf("a form with a field not taken answered %d", r.status)
	}
	// Two photos of one name, as a phone names them, are refused as the
	// body they are, and not taken for a failure.
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("text", "two photos")
	for range 2 {
		part, _ := w.CreateFormFile("files", "image.png")
		part.Write([]byte("\x89PNG\r\n\x1a\n"))
	}
	w.Close()
	if r := kit.do(http.MethodPost, "/api/requests", w.FormDataContentType(), body.Bytes()); r.status != http.StatusBadRequest || !strings.Contains(r.json()["error"].(string), "image.png") {
		t.Errorf("two files of one name answered %d %s", r.status, r.body)
	}
	if after := count(); after != before {
		t.Errorf("bodies refused kept something: %d, and %d before", after, before)
	}
}

func TestServerTooLargeRefusedNeverCut(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	kit := s.as("kit")
	n := numbered(kit.post("/api/requests", map[string]string{"text": "is the kettle on?"}))
	s.answered(n)
	length := func() int {
		thread, err := s.areasOf().Requests.Thread(n)
		if err != nil {
			t.Fatal(err)
		}
		return len(thread)
	}
	before := length()
	r := kit.post("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": strings.Repeat("a", MaxJSON)})
	if r.status != http.StatusRequestEntityTooLarge || !strings.Contains(r.json()["error"].(string), "1 MiB") {
		t.Errorf("JSON over its limit answered %d %s", r.status, r.body)
	}
	r = kit.form("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": "a photo"}, map[string][]byte{"big.png": bytes.Repeat([]byte("x"), MaxForm)})
	if r.status != http.StatusRequestEntityTooLarge || !strings.Contains(r.json()["error"].(string), "32 MiB") {
		t.Errorf("a form over its limit answered %d %s", r.status, r.body)
	}
	if after := length(); after != before {
		t.Errorf("a body too large kept %d entries", after-before)
	}
}

// longThread keeps, in the bench's store before it is served, a request
// of Raj's to support, whose lead is Dana, with as many entries as asked,
// the tenth of them waiting on Dana; nothing of the office is running yet,
// so nothing wakes at each.
func longThread(t *testing.T, b *bench, entries int) (int64, int64) {
	reqs := kept(t, b)
	r, err := reqs.AskInChat("raj", "support", "entry 1", nil)
	if err != nil {
		t.Fatal(err)
	}
	var waiting int64
	for i := 2; i <= entries; i++ {
		if i == 10 {
			e, err := reqs.Write(r.Number, requests.Writing{Kind: requests.KindQuestion, By: requests.Bulldog, To: []requests.Who{reqs.Person("dana")},
				Waits: []requests.Who{reqs.Person("dana")}, Text: fmt.Sprintf("entry %d", i)})
			if err != nil {
				t.Fatal(err)
			}
			waiting = e.ID
			continue
		}
		if _, err := reqs.Say(r.Number, reqs.Person("raj"), fmt.Sprintf("entry %d", i), nil, requests.Screens, false); err != nil {
			t.Fatal(err)
		}
	}
	return r.Number, waiting
}

// kept gives the requests of the bench's store before it is served, with
// the office read into them; the store is closed when the test ends.
func kept(t *testing.T, b *bench) *requests.Store {
	if _, err := Ready(b.dir, Readying{Make: true}, b.clock.Now(), func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	db, err := OpenStore(b.dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	reqs, err := requests.Open(db, b.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := office.Read(b.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reqs.Read(o); err != nil {
		t.Fatal(err)
	}
	return reqs
}

// manyRequests keeps, in the bench's store before it is served, as many
// requests of Raj's to support as asked, and gives their numbers, oldest
// first.
func manyRequests(t *testing.T, b *bench, many int) []int64 {
	reqs := kept(t, b)
	var numbers []int64
	for i := 1; i <= many; i++ {
		r, err := reqs.AskInChat("raj", "support", fmt.Sprintf("request %d", i), nil)
		if err != nil {
			t.Fatal(err)
		}
		numbers = append(numbers, r.Number)
	}
	return numbers
}

// listPart is a part of a list of requests as the one looking is given it.
type listPart struct {
	Requests []summaryOut
	Before   int64
}

func (l listPart) numbers() []int64 {
	var out []int64
	for _, r := range l.Requests {
		out = append(out, r.Number)
	}
	return out
}

// newest gives numbers newest first.
func newest(numbers []int64) []int64 {
	out := slices.Clone(numbers)
	slices.Reverse(out)
	return out
}

func TestServerRequestsListedInPart(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	numbers := newest(manyRequests(t, b, 620))
	s := b.serve()
	var first listPart
	s.as("raj").get("/api/requests?asked=me").is(http.StatusOK).into(&first)
	if !slices.Equal(first.numbers(), numbers[:100]) || first.Before != numbers[99] {
		t.Fatalf("the first part is %d requests, before %d", len(first.Requests), first.Before)
	}
	var next listPart
	s.as("raj").get(fmt.Sprintf("/api/requests?asked=me&before=%d&limit=1000", first.Before)).is(http.StatusOK).into(&next)
	if !slices.Equal(next.numbers(), numbers[100:600]) || next.Before != numbers[599] {
		t.Fatalf("a limit of 1000 gives %d requests, before %d", len(next.Requests), next.Before)
	}
	var last listPart
	r := s.as("raj").get(fmt.Sprintf("/api/requests?asked=me&before=%d&limit=50", next.Before)).is(http.StatusOK)
	r.into(&last)
	if !slices.Equal(last.numbers(), numbers[600:]) || strings.Contains(string(r.body), `"before"`) {
		t.Errorf("the last part is %v, and %s", last.numbers(), r.body)
	}
	for _, address := range []string{"?limit=lots", "?limit=0", "?before=-3", "?limit=5&limit=6"} {
		if e := s.as("raj").get("/api/requests" + address).is(http.StatusBadRequest).json(); e["error"] == "" {
			t.Errorf("%s is refused with no sentence", address)
		}
	}
}

func TestServerRequestPagesCutByNumber(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	numbers := newest(manyRequests(t, b, 6))
	s := b.serve()
	var first listPart
	s.as("raj").get("/api/requests?asked=me&limit=3").is(http.StatusOK).into(&first)
	// Raj asks again between the two parts: the next part goes on from
	// where the first ended, and the new request is in neither.
	s.chat("raj", "support", "and another")
	var next listPart
	s.as("raj").get(fmt.Sprintf("/api/requests?asked=me&limit=3&before=%d", first.Before)).is(http.StatusOK).into(&next)
	if !slices.Equal(first.numbers(), numbers[:3]) || !slices.Equal(next.numbers(), numbers[3:]) || next.Before != 0 {
		t.Errorf("the parts are %v and %v, before %d", first.numbers(), next.numbers(), next.Before)
	}
}

// part gives a part of a thread as the one looking is given it.
type part struct {
	Entries []entryOut
	Before  int64
}

func (p part) texts() []string {
	var out []string
	for _, e := range p.Entries {
		out = append(out, e.Text)
	}
	return out
}

func TestServerThreadGivenInPart(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	n, _ := longThread(t, b, 300)
	s := b.serve()
	var latest part
	s.as("raj").get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&latest)
	texts := latest.texts()
	if len(texts) != 51 || texts[0] != "entry 1" || texts[1] != "entry 251" || texts[50] != "entry 300" || latest.Before == 0 {
		t.Fatalf("the latest part is %d entries, %v ... %v, before %d", len(texts), texts[:2], texts[len(texts)-1], latest.Before)
	}
	var earlier part
	s.as("raj").get(fmt.Sprintf("/api/requests/%d?before=%d", n, latest.Before)).is(http.StatusOK).into(&earlier)
	if texts := earlier.texts(); len(texts) != 51 || texts[1] != "entry 201" || texts[50] != "entry 250" {
		t.Errorf("the part before is %v ... %v", texts[:2], texts[len(texts)-1])
	}
	var most part
	s.as("raj").get(fmt.Sprintf("/api/requests/%d?size=500", n)).is(http.StatusOK).into(&most)
	if texts := most.texts(); len(texts) != 201 || texts[1] != "entry 101" || most.Before == 0 {
		t.Errorf("a size of 500 gives %d entries, from %v, before %d", len(texts), texts[:2], most.Before)
	}
}

func TestServerWaitingEntryNeverHiddenByPaging(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	n, waiting := longThread(t, b, 120)
	s := b.serve()
	var p part
	s.as("dana").get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&p)
	if !slices.ContainsFunc(p.Entries, func(e entryOut) bool { return e.ID == waiting }) || p.Entries[0].Text != "entry 1" {
		t.Errorf("Dana's part of the thread is %v", p.texts()[:3])
	}
	// Raj, on whom it does not wait, is given it only in its place.
	var raj part
	s.as("raj").get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&raj)
	if slices.ContainsFunc(raj.Entries, func(e entryOut) bool { return e.ID == waiting }) {
		t.Error("Raj is given the entry that waits on Dana out of its place")
	}
}

func TestServerWordsOnABridgeShownAsTheirs(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	reqs := kept(t, b)
	dana := requests.Identity("slack", "U0DANA00")
	r, err := reqs.AskAt("slack", "#help", "#help/1700.1", "", "U0DANA00", "is 4411 refunded?", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Say(r.Number, dana, "and 4412?", nil, requests.Where{Bridge: "slack", Place: "#help/1700.1"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Write(r.Number, requests.Writing{Kind: requests.KindBulldog, By: requests.Bulldog, To: []requests.Who{dana}, Text: "Both are refunded."}); err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Answer(r.Number, reqs.Worker("bot"), "4411 and 4412 are refunded.", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Say(r.Number, reqs.Person("al"), "Closing this one.", nil, requests.Screens, false); err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Source("slack", "#help/1700.1", requests.Status{Status: "done", By: requests.Identity("slack", "U0AL0000"), At: b.clock.Now()}); err != nil {
		t.Fatal(err)
	}
	guest, err := reqs.AskAt("slack", "#help", "#help/1800.1", "", "U0GUEST0", "please refund me", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	walk, err := reqs.AskByRoutine("office", office.Routine{At: "mon 09:00", To: "bot", Say: "What is still open?"}, reqs.Person("al"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Answer(walk.Number, reqs.Worker("bot"), "", nil); err != nil {
		t.Fatal(err)
	}
	s := b.serve()
	// Dana's words on Slack are hers wherever they are shown, as her
	// asking is.
	reply := s.as("al").get("/api/requests/" + itoa(r.Number)).is(http.StatusOK)
	var v viewOut
	reply.into(&v)
	for _, text := range []string{"is 4411 refunded?", "and 4412?"} {
		at := slices.IndexFunc(v.Entries, func(e entryOut) bool { return e.Text == text })
		if at < 0 || v.Entries[at].By.Kind != requests.IsPerson || v.Entries[at].By.Name != "dana" || v.Entries[at].By.Called != "Dana" {
			t.Errorf("%q is shown by %+v", text, v.Entries)
		}
	}
	at := slices.IndexFunc(v.Entries, func(e entryOut) bool { return e.Text == "Both are refunded." })
	if at < 0 || len(v.Entries[at].To) != 1 || v.Entries[at].To[0].Kind != requests.IsPerson || v.Entries[at].To[0].Name != "dana" {
		t.Errorf("Bulldog's entry to Dana is shown to %+v", v.Entries)
	}
	// So are a worker's answer, Al's words on the screens, and his status
	// at the source.
	at = slices.IndexFunc(v.Entries, func(e entryOut) bool { return e.Text == "4411 and 4412 are refunded." })
	if at < 0 || v.Entries[at].By.Kind != requests.IsWorker || v.Entries[at].By.Name != "bot" || v.Entries[at].By.Team != "office" ||
		len(v.Entries[at].To) != 1 || v.Entries[at].To[0].Name != "dana" {
		t.Errorf("the bot's answer is shown as %+v", v.Entries)
	}
	at = slices.IndexFunc(v.Entries, func(e entryOut) bool { return e.Text == "Closing this one." })
	if at < 0 || v.Entries[at].By.Kind != requests.IsPerson || v.Entries[at].By.Name != "al" || v.Entries[at].By.Called != "Al" {
		t.Errorf("Al's words are shown as %+v", v.Entries)
	}
	if v.Source == nil || v.Source.By.Kind != requests.IsPerson || v.Source.By.Name != "al" {
		t.Errorf("the status at the source is shown as %+v", v.Source)
	}
	// Who ran a routine now is shown as the office names them.
	var w viewOut
	routine := s.as("al").get("/api/requests/" + itoa(walk.Number)).is(http.StatusOK)
	routine.into(&w)
	if w.Origin == nil || w.Origin.Kind != requests.ByRoutine || w.Origin.By == nil || w.Origin.By.Kind != requests.IsPerson || w.Origin.By.Name != "al" {
		t.Errorf("the routine run now is shown as %+v", w.Origin)
	}
	// Someone the office does not name stays a guest of the bridge.
	var g viewOut
	s.as("al").get("/api/requests/" + itoa(guest.Number)).is(http.StatusOK).into(&g)
	if by := g.Entries[0].By; by.Kind != requests.IsGuest || by.Bridge != "slack" || by.ID != "U0GUEST0" {
		t.Errorf("the guest's asking is shown by %+v", by)
	}
	// What Bulldog keeps of who someone is never leaves it.
	list := s.as("al").get("/api/requests").is(http.StatusOK)
	for _, body := range [][]byte{reply.body, routine.body, list.body} {
		if bytes.Contains(body, []byte(`"known"`)) {
			t.Errorf("an answer gives known: %s", body)
		}
	}
}
