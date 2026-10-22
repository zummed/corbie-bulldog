package screens

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// ids gives the ids of the requests a person's list holds.
func ids(list map[string]any) []string {
	var found []string
	for _, r := range list["requests"].([]any) {
		found = append(found, r.(map[string]any)["id"].(string))
	}
	return found
}

// nodeOf gives a node of a request's view by its stage.
func nodeOf(v map[string]any, stage string) map[string]any {
	for _, n := range v["nodes"].([]any) {
		if n := n.(map[string]any); n["stage"] == stage {
			return n
		}
	}
	return nil
}

// A person sees what they asked, sealed, and their team's requests whole;
// a team with no people is seen by the office's people.
func TestScreensSight(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	dana, lee, al, priya := w.as("dana"), w.as("lee"), w.as("al"), w.as("priya")
	asked := dana.ask("payments", "Was order 4411 refunded?")
	w.ended(asked)

	if !slices.Contains(ids(dana.get("/api/requests")), asked) || !slices.Contains(ids(lee.get("/api/requests")), asked) {
		t.Error("Dana or Lee does not see Dana's request in the list")
	}
	if slices.Contains(ids(al.get("/api/requests")), asked) {
		t.Error("Al sees Dana's request of payments")
	}
	sealed, whole := dana.get("/api/requests/"+asked), lee.get("/api/requests/"+asked)
	if sealed["read"].(map[string]any)["sealed"] != true || whole["read"].(map[string]any)["sealed"] != false {
		t.Errorf("Dana sees %v and Lee %v", sealed["read"], whole["read"])
	}
	if status, _ := al.api("GET", "/api/requests/"+asked, nil); status != http.StatusNotFound {
		t.Errorf("Al is answered %d", status)
	}
	if sealed["status"] != "answered" || sealed["answer"] != "Refunded £40 on 3 May." || sealed["canSay"] != true {
		t.Errorf("Dana's request is %s", jsonOf(sealed))
	}
	if nodeOf(sealed, "source")["line"] != "Dana · on the screens" || nodeOf(sealed, "reply")["line"] != "shown on the screens" ||
		nodeOf(sealed, "check")["state"] != "passed" || nodeOf(sealed, "check")["line"] != nil || nodeOf(sealed, "heed")["state"] != "skipped" {
		t.Errorf("Dana's request's nodes are %s", jsonOf(sealed["nodes"]))
	}
	check := nodeOf(whole, "check")
	if check["line"] != "no-account-numbers · yes" ||
		jsonOf(check["deciders"]) != `[{"kind":"rule","name":"./payments/no-account-numbers.js","note":"No account numbers.","says":"yes"}]` ||
		!strings.HasPrefix(nodeOf(whole, "work")["line"].(string), "ledger · ") {
		t.Errorf("Lee sees the nodes %s", jsonOf(whole["nodes"]))
	}
	read := whole["read"].(map[string]any)
	if !strings.Contains(jsonOf(read["calls"]), `"tool":"read_doc"`) || !strings.Contains(jsonOf(read["turns"]), "Was order 4411 refunded?") {
		t.Errorf("Lee sees what the team read as %s", jsonOf(read))
	}

	// Platform has no people, so the office's people, Al, see its requests.
	up := lee.ask("platform", "Is the API up?")
	w.ended(up)
	if v := al.get("/api/requests/" + up); v["read"].(map[string]any)["sealed"] != false || v["answer"] != "The API is up." {
		t.Errorf("Al sees platform's request as %s", jsonOf(v))
	}
	if v := lee.get("/api/requests/" + up); v["read"].(map[string]any)["sealed"] != true {
		t.Errorf("Lee sees platform's request as %s", jsonOf(v["read"]))
	}
	if status, _ := priya.api("GET", "/api/requests/"+up, nil); status != http.StatusNotFound {
		t.Errorf("Priya is answered %d for platform's request", status)
	}
}

// says, for a request the bot leads: it asks payments, then says what
// payments answered.
func asksPayments(turn testkit.Turn) testkit.Reply {
	switch prompt := turn.Prompt(); {
	case strings.Contains(prompt, "Ask payments") && turn.Number == 1:
		return testkit.Use("ask_team", map[string]any{"team": "payments", "question": "Was order 4411 refunded?"})
	case strings.Contains(prompt, "Ask payments"):
		return testkit.Say("Payments says it was refunded.")
	}
	return says(turn)
}

func TestScreensOutsiderSeesNothingTheTeamRead(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, asksPayments, nil)
	dana, lee, al := w.as("dana"), w.as("lee"), w.as("al")

	asked := dana.ask("payments", "Was order 4411 refunded?")
	w.ended(asked)
	_, sealed := dana.body("/api/requests/" + asked)
	for _, read := range []string{"read_doc", "orders.md", "No account numbers", "ledger", "keep payments' ledger", `"calls"`, `"turns"`, `"deciders"`} {
		if strings.Contains(sealed, read) {
			t.Errorf("Dana is shown %q of what payments read:\n%s", read, sealed)
		}
	}
	if !strings.Contains(sealed, "Refunded £40 on 3 May.") || !strings.Contains(sealed, "Was order 4411 refunded?") {
		t.Errorf("Dana is not shown what she asked and what left payments:\n%s", sealed)
	}

	// Dana asks the office, whose bot asks payments: Dana sees whom it
	// asked, when, and how it ended; Al, of the office, sees the question
	// and the answer, and the request it asked.
	bot := dana.ask("office", "Ask payments about order 4411.")
	w.ended(bot)
	forked := dana.get("/api/requests/" + bot)["forks"].([]any)
	if len(forked) != 1 || jsonOf(forked[0].(map[string]any)["party"]) != `{"id":"payments","kind":"team","name":"Payments"}` {
		t.Fatalf("Dana sees the forks %s", jsonOf(forked))
	}
	f := forked[0].(map[string]any)
	if f["kind"] != "team" || f["status"] != "answered" || f["at"] == nil || f["ended"] == nil || f["question"] != nil || f["answer"] != nil || f["request"] != nil {
		t.Errorf("Dana sees the fork %s", jsonOf(f))
	}
	f = al.get("/api/requests/" + bot)["forks"].([]any)[0].(map[string]any)
	sub, _ := f["request"].(string)
	if f["question"] != "Was order 4411 refunded?" || f["answer"] != "Refunded £40 on 3 May." || sub == "" {
		t.Fatalf("Al sees the fork %s", jsonOf(f))
	}
	asking := al.get("/api/requests/" + sub)
	if asking["source"].(map[string]any)["request"] != bot || asking["read"].(map[string]any)["sealed"] != true || nodeOf(asking, "source")["line"] != "Platform support asked" {
		t.Errorf("Al sees payments' request as %s", jsonOf(asking))
	}
	if v := lee.get("/api/requests/" + sub); v["source"].(map[string]any)["request"] != nil || v["read"].(map[string]any)["sealed"] != false {
		t.Errorf("Lee sees payments' request as %s", jsonOf(v))
	}
	if status, _ := dana.api("GET", "/api/requests/"+sub, nil); status != http.StatusNotFound {
		t.Errorf("Dana is answered %d for payments' request", status)
	}

	// The ledger puts a card's number to Dana, whom payments may not ask.
	// Bulldog refuses it, so it was put to nobody: Dana sees only whom it
	// named and that it was refused, and Lee sees the question.
	charged := dana.ask("payments", "Was my card charged twice?")
	w.ended(charged)
	_, body := dana.body("/api/requests/" + charged)
	if strings.Contains(body, "4111") || strings.Contains(body, "may not ask") {
		t.Errorf("Dana is shown the question payments was refused:\n%s", body)
	}
	f = dana.get("/api/requests/" + charged)["forks"].([]any)[0].(map[string]any)
	if f["kind"] != "person" || f["status"] != "refused" || f["party"].(map[string]any)["id"] != "dana" || f["card"] != nil {
		t.Errorf("Dana sees the refused question as %s", jsonOf(f))
	}
	f = lee.get("/api/requests/" + charged)["forks"].([]any)[0].(map[string]any)
	if !strings.Contains(f["question"].(string), "4111") || !strings.HasPrefix(f["answer"].(string), "Payments may not ask dana") {
		t.Errorf("Lee sees the refused question as %s", jsonOf(f))
	}
	if list := dana.get("/api/requests"); len(list["waiting"].([]any)) != 0 {
		t.Errorf("Dana is put the refused question: %s", jsonOf(list["waiting"]))
	}
}

func TestScreensUnseenIsNotThere(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	dana, lee, al := w.as("dana"), w.as("lee"), w.as("al")
	status, out := lee.api("POST", "/api/requests", map[string]any{"team": "payments", "said": "A file for the ledger.",
		"files": []any{map[string]any{"name": "note.txt", "data": base64.StdEncoding.EncodeToString([]byte("a note"))}}})
	if status != http.StatusOK {
		t.Fatalf("asking with a file answered %d: %v", status, out)
	}
	asked := out["id"].(string)
	w.ended(asked)
	if status, _ := lee.api("PUT", "/api/teams/payments/docs/refunds.md", map[string]any{"text": "Within 30 days."}); status != http.StatusOK {
		t.Fatalf("Lee's doc is answered %d", status)
	}

	same := func(what string, unseen, missing *browser, a, b string) {
		t.Helper()
		s1, body1 := unseen.body(a)
		s2, body2 := missing.body(b)
		if s1 != http.StatusNotFound || s1 != s2 || body1 != body2 {
			t.Errorf("%s: what may not be seen is answered %d %s, and what is not there %d %s", what, s1, body1, s2, body2)
		}
	}
	same("a request", al, lee, "/api/requests/"+asked, "/api/requests/20261005T090000.000Z-0123456789abcdef")
	same("a request", al, lee, "/api/requests/"+asked, "/api/requests/nothing")
	same("a file", al, lee, "/api/requests/"+asked+"/files/0", "/api/requests/"+asked+"/files/1")
	same("a doc", dana, lee, "/api/teams/payments/docs/refunds.md", "/api/teams/payments/docs/nothing.md")
	for _, act := range []string{"say", "stop"} {
		s1, a := al.api("POST", "/api/requests/"+asked+"/"+act, map[string]any{"said": "More."})
		s2, b := lee.api("POST", "/api/requests/20261005T090000.000Z-0123456789abcdef/"+act, map[string]any{"said": "More."})
		if s1 != http.StatusNotFound || s1 != s2 || jsonOf(a) != jsonOf(b) {
			t.Errorf("%s: what may not be seen is answered %d %v, and what is not there %d %v", act, s1, a, s2, b)
		}
	}
	if status, _ := dana.api("PUT", "/api/teams/payments/docs/refunds.md", map[string]any{"text": "Never."}); status != http.StatusNotFound {
		t.Errorf("Dana writing payments' doc is answered %d", status)
	}
	if doc := lee.get("/api/teams/payments/docs/refunds.md"); doc["text"] != "Within 30 days." {
		t.Errorf("payments' doc is %v", doc)
	}
}

// A decision is put to the people it asks as a card, and answered from
// the screens, with one of its choices, by one of them.
func TestScreensDecisionAnsweredFromTheScreens(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	lee, priya, dana := w.as("lee"), w.as("priya"), w.as("dana")
	asked := lee.ask("payments", "Refund ch_1.")
	c := lee.card()
	if c["kind"] != "decision" || c["request"] != asked || c["team"] != "payments" || c["teamName"] != "Payments" ||
		jsonOf(c["choices"]) != `["yes","no"]` || !strings.Contains(c["said"].(string), `refund: {"charge":"ch_1"}`) {
		t.Errorf("Lee's card is %s", jsonOf(c))
	}
	if other := priya.card(); other["ref"] != c["ref"] {
		t.Errorf("Priya's card is %s", jsonOf(other))
	}
	if list := dana.get("/api/requests"); len(list["waiting"].([]any)) != 0 || len(list["requests"].([]any)) != 0 {
		t.Errorf("Dana sees %s", jsonOf(list))
	}
	v := lee.get("/api/requests/" + asked)
	forks := v["forks"].([]any)
	if len(forks) != 1 || forks[0].(map[string]any)["kind"] != "decision" || forks[0].(map[string]any)["card"].(map[string]any)["ref"] != c["ref"] ||
		v["status"] != "waiting" || nodeOf(v, "work")["line"] != "ledger · waiting on a decision" {
		t.Errorf("Lee's waiting request is %s", jsonOf(v))
	}

	answer := "/api/waits/" + c["ref"].(string) + "/answer"
	if status, out := dana.api("POST", answer, map[string]any{"answer": "yes"}); status != http.StatusNotFound {
		t.Errorf("Dana's answer is answered %d %v", status, out)
	}
	if status, out := lee.api("POST", answer, map[string]any{"answer": "maybe"}); status != http.StatusBadRequest || out["error"] != "Answer with one of the choices: yes or no." {
		t.Errorf("Lee's maybe is answered %d %v", status, out)
	}
	if status, out := lee.api("POST", answer, map[string]any{"answer": "yes"}); status != http.StatusOK || jsonOf(out) != "{}" {
		t.Errorf("Lee's yes is answered %d %v", status, out)
	}
	w.ended(asked)
	if status, _ := priya.api("POST", answer, map[string]any{"answer": "no"}); status != http.StatusNotFound {
		t.Errorf("Priya's answer after Lee's is answered %d", status)
	}
	f := lee.get("/api/requests/" + asked)["forks"].([]any)[0].(map[string]any)
	if f["status"] != "answered" || f["answer"] != "yes (Lee)" || jsonOf(f["party"]) != `{"id":"lee","kind":"person","name":"Lee"}` || f["card"] != nil ||
		jsonOf(f["jobs"]) != `["refund"]` || jsonOf(f["deciders"]) != `[{"kind":"person","name":"Lee","says":"yes"}]` {
		t.Errorf("the decision is %s", jsonOf(f))
	}

	// A decision that said no shows it.
	again := lee.ask("payments", "Refund ch_3.")
	if status, _ := priya.api("POST", "/api/waits/"+priya.card()["ref"].(string)+"/answer", map[string]any{"answer": "No"}); status != http.StatusOK {
		t.Fatalf("Priya's no is answered %d", status)
	}
	w.ended(again)
	if f := lee.get("/api/requests/" + again)["forks"].([]any)[0].(map[string]any); f["status"] != "no" || f["answer"] != "no (Priya)" {
		t.Errorf("the decision Priya said no to is %s", jsonOf(f))
	}
}

// A png: a picture of one dot.
var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")

func TestScreensAskWithFilesAndSayMore(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	dana, lee, al := w.as("dana"), w.as("lee"), w.as("al")
	status, out := dana.api("POST", "/api/requests", map[string]any{"team": "payments", "said": "Here is the screenshot.",
		"files": []any{map[string]any{"name": "shot.png", "data": base64.StdEncoding.EncodeToString(png)}}})
	if status != http.StatusOK {
		t.Fatalf("asking answered %d %v", status, out)
	}
	asked := out["id"].(string)
	w.ended(asked)
	shown, err := w.corbie.Show(ctx, w.run(asked))
	if err != nil {
		t.Fatal(err)
	}
	files := shown.Input.(map[string]any)["files"].([]any)
	if data := files[0].(map[string]any)["data"].(map[string]any); data["$blob"] == nil || data["type"] != "image/png" {
		t.Errorf("payments' request is given the file as %v", files)
	}
	given := dana.get("/api/requests/" + asked)["source"].(map[string]any)["files"].([]any)
	link := given[0].(map[string]any)
	if link["name"] != "shot.png" || link["type"] != "image/png" || link["href"] != "/api/requests/"+asked+"/files/0" {
		t.Errorf("the file is shown as %v", given)
	}
	res := dana.do("GET", link["href"].(string), nil)
	if body, _ := io.ReadAll(res.Body); res.StatusCode != http.StatusOK || string(body) != string(png) {
		t.Errorf("the file is served as %s", res.Status)
	}

	w.corbie.Clock.Move(time.Minute)
	status, out = dana.api("POST", "/api/requests/"+asked+"/say", map[string]any{"said": "And order 4412?"})
	if status != http.StatusOK {
		t.Fatalf("saying more answered %d %v", status, out)
	}
	more := out["id"].(string)
	w.ended(more)
	first, next := w.ended(asked), w.ended(more)
	if first.Labels["thread"] != next.Labels["thread"] || next.Labels["team"] != "payments" {
		t.Errorf("the request said after is labelled %v, the first %v", next.Labels, first.Labels)
	}
	thread := dana.get("/api/requests/" + more)["thread"].([]any)
	if len(thread) != 2 || thread[0].(map[string]any)["id"] != asked || thread[1].(map[string]any)["id"] != more ||
		thread[0].(map[string]any)["answer"] != "Refunded £40 on 3 May." {
		t.Errorf("the thread is %s", jsonOf(thread))
	}
	if status, _ := lee.api("POST", "/api/requests/"+asked+"/say", map[string]any{"said": "Lee adds this."}); status != http.StatusOK {
		t.Errorf("Lee saying more is answered %d", status)
	}
	if status, _ := al.api("POST", "/api/requests/"+asked+"/say", map[string]any{"said": "Al adds this."}); status != http.StatusNotFound {
		t.Errorf("Al saying more is answered %d", status)
	}
	for _, bad := range []map[string]any{{"team": "payments", "said": " "}, {"team": "nothing", "said": "Hello."}} {
		if status, out := dana.api("POST", "/api/requests", bad); status == http.StatusOK || out["error"] == nil {
			t.Errorf("asking %v answered %d %v", bad, status, out)
		}
	}

	// A team whose files have problems takes no requests: asking it, and
	// saying more to it, are refused alike.
	w.rewrite("payments/ledger.yaml", func(string) string { return "agent: ledger\n  tools: [\n" })
	for path, body := range map[string]map[string]any{"/api/requests": {"team": "payments", "said": "Hello."}, "/api/requests/" + asked + "/say": {"said": "Hello."}} {
		if status, out := dana.api("POST", path, body); status != http.StatusConflict || out["error"] != "Payments takes no requests while its files have problems. Its page shows them." {
			t.Errorf("POST %s to a team with problems answered %d %v", path, status, out)
		}
	}
}

func TestScreensFilesServedHarmlessly(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	dana := w.as("dana")
	kept := map[string]string{
		"page.png": "<html><script>alert(1)</script></html>",
		"pic.svg":  `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"dot.txt":  string(png),
		"doc.pdf":  "%PDF-1.4\n%âãÏÓ\n",
	}
	names := []string{"page.png", "pic.svg", "dot.txt", "doc.pdf"}
	var files []any
	for _, name := range names {
		files = append(files, map[string]any{"name": name, "data": base64.StdEncoding.EncodeToString([]byte(kept[name]))})
	}
	status, out := dana.api("POST", "/api/requests", map[string]any{"team": "payments", "said": "Four files.", "files": files})
	if status != http.StatusOK {
		t.Fatalf("asking answered %d %v", status, out)
	}
	asked := out["id"].(string)
	w.ended(asked)
	for i, want := range []struct{ kind, disposition string }{
		{"application/octet-stream", "attachment"},
		{"application/octet-stream", "attachment"},
		{"image/png", "inline"},
		{"application/pdf", "inline"},
	} {
		res := dana.do("GET", fmt.Sprintf("/api/requests/%s/files/%d", asked, i), nil)
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != want.kind ||
			res.Header.Get("Content-Disposition") != want.disposition+`; filename=`+names[i] ||
			res.Header.Get("Content-Security-Policy") != "sandbox; default-src 'none'" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s is served %s as %v", names[i], res.Status, res.Header)
		}
	}
}

func TestScreensStop(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	lee, dana := w.as("lee"), w.as("dana")
	asked := lee.ask("payments", "Refund ch_2.")
	lee.card()
	if v := lee.get("/api/requests/" + asked); v["canStop"] != true {
		t.Errorf("Lee may not stop the request: %s", jsonOf(v))
	}
	if status, _ := dana.api("POST", "/api/requests/"+asked+"/stop", nil); status != http.StatusNotFound {
		t.Errorf("Dana stopping Lee's request is answered %d", status)
	}
	if status, out := lee.api("POST", "/api/requests/"+asked+"/stop", nil); status != http.StatusOK {
		t.Fatalf("Lee stopping it is answered %d %v", status, out)
	}
	if run := w.ended(asked); run.Status != "cancelled" {
		t.Errorf("the request ended %s", run.Status)
	}
	v := lee.until("the stopped request's wait has ended", "/api/requests/"+asked, func(v map[string]any) bool {
		return v["forks"].([]any)[0].(map[string]any)["status"] == "stopped"
	})
	if v["status"] != "stopped" || v["canStop"] != false || nodeOf(v, "reply")["state"] != "waiting-to-start" {
		t.Errorf("the stopped request is %s", jsonOf(v))
	}
	if status, out := lee.api("POST", "/api/requests/"+asked+"/stop", nil); status != http.StatusConflict {
		t.Errorf("stopping it again is answered %d %v", status, out)
	}
	if list := lee.get("/api/requests"); len(list["waiting"].([]any)) != 0 {
		t.Errorf("Lee is still waited on: %s", jsonOf(list["waiting"]))
	}
}

// house gives the house's workers their words: the steward hands a repair
// to the fixer, the fixer orders a part for £35, and the clerk passes.
func house(turn testkit.Turn) testkit.Reply {
	switch {
	case strings.Contains(turn.System, "You keep the house's accounts"):
		return testkit.Say(`{"says": "pass", "note": "The house has the money for it."}`)
	case strings.Contains(turn.System, "You mend things") && turn.Number == 1:
		return testkit.Use("order_part", map[string]any{"part": "a 13A socket", "pounds": 35})
	case strings.Contains(turn.System, "You mend things"):
		return testkit.Say("I ordered a socket.")
	case turn.Number == 1:
		return testkit.Use("fixer", map[string]any{"repair": turn.Prompt()})
	}
	return testkit.Say("The fixer has ordered a socket, and fits it on Thursday.")
}

// Sam is shown the fixer's order as a card, with the notes of the deciders
// before him, above yes and no, and Kit's request waiting on it.
func TestScreensDecisionShowsItsNotes(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, house, nil)
	kit, sam := w.as("kit"), w.as("sam")
	asked := kit.ask("office", "The socket by the boiler sparks.")
	c := sam.card()
	said := c["said"].(string)
	for _, shown := range []string{`order_part: {"part":"a 13A socket","pounds":35}`, "./small-orders.js passed: More than I order alone.", "clerk passed: The house has the money for it."} {
		if !strings.Contains(said, shown) {
			t.Errorf("Sam's card does not show %q:\n%s", shown, said)
		}
	}
	list := sam.get("/api/requests")
	row := list["requests"].([]any)[0].(map[string]any)
	if row["id"] != asked || row["status"] != "waiting" || jsonOf(row["by"]) != `{"id":"kit","kind":"person","name":"Kit"}` ||
		jsonOf(row["nodes"]) != `[{"stage":"source","state":"passed"},{"stage":"heed","state":"skipped"},{"stage":"work","state":"waiting"},{"stage":"check","state":"skipped"},{"stage":"reply","state":"waiting-to-start"}]` {
		t.Errorf("Sam's list shows %s", jsonOf(row))
	}
	if kits := kit.get("/api/requests"); len(kits["waiting"].([]any)) != 0 {
		t.Errorf("Kit is shown the cards %s", jsonOf(kits["waiting"]))
	}
	if status, _ := sam.api("POST", "/api/waits/"+c["ref"].(string)+"/answer", map[string]any{"answer": "yes"}); status != http.StatusOK {
		t.Fatalf("Sam's yes is answered %d", status)
	}
	v := kit.until("the house answers", "/api/requests/"+asked, func(v map[string]any) bool { return v["status"] == "answered" })
	if v["answer"] != "The fixer has ordered a socket, and fits it on Thursday." {
		t.Errorf("Kit is answered %v", v["answer"])
	}

	// Kit, of the house, sees the decision: the held job, Sam's yes, and
	// what each decider before him said; and under the steward's call of
	// the fixer, the fixer's order.
	forks := v["forks"].([]any)
	f := forks[len(forks)-1].(map[string]any)
	deciders := jsonOf(f["deciders"])
	if f["kind"] != "decision" || jsonOf(f["jobs"]) != `["order_part"]` || f["answer"] != "yes (Sam)" ||
		!strings.Contains(deciders, `{"kind":"rule","name":"./small-orders.js","note":"More than I order alone.","says":"pass"}`) ||
		!strings.Contains(deciders, `{"kind":"worker","name":"clerk","note":"The house has the money for it.","says":"pass"}`) {
		t.Errorf("Kit sees the decision as %s", jsonOf(f))
	}
	calls := v["read"].(map[string]any)["calls"].([]any)
	if fixer := calls[0].(map[string]any); fixer["tool"] != "fixer" || !strings.Contains(jsonOf(fixer["calls"]), `"tool":"order_part"`) {
		t.Errorf("the steward's calls are %s", jsonOf(calls))
	}
}

// A request heard on a bridge shows its place, and whether its answer was
// carried back.
func TestScreensHeardOnABridge(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	dana, al := w.as("dana"), w.as("al")
	heard := func(by requests.By, place string, said string) string {
		run, err := w.engine.Ask(ctx, requests.Ask{Team: map[string]string{"#refunds": "payments", "#help": "office"}[place], By: by, Said: said, Mentioned: true,
			Place: &requests.Place{Bridge: "slack", Place: place, Guests: place == "#help", Where: "C01/" + said, Thing: "C01/" + said}, At: testkit.Monday})
		if err != nil {
			t.Fatal(err)
		}
		return w.screens.id(run)
	}
	refunds := heard(requests.By{Person: "dana"}, "#refunds", "@bot was order 4411 refunded?")
	v := dana.until("the answer is carried", "/api/requests/"+refunds, func(v map[string]any) bool { return nodeOf(v, "reply")["state"] == "passed" })
	if nodeOf(v, "source")["line"] != "Dana · slack › #refunds" || v["source"].(map[string]any)["said"] != "@bot was order 4411 refunded?" ||
		nodeOf(v, "heed")["state"] != "passed" || nodeOf(v, "reply")["line"] != "sent" {
		t.Errorf("Dana's request in #refunds is %s", jsonOf(v))
	}
	if sent := w.bridges.carried(); len(sent) != 1 || sent[0].To != "C01/@bot was order 4411 refunded?" || sent[0].Text != "Refunded £40 on 3 May." {
		t.Errorf("the answer is carried as %+v", sent)
	}

	help := heard(requests.By{Guest: "U0GUEST"}, "#help", "@bot is the API up?")
	v = al.until("the guest is answered", "/api/requests/"+help, func(v map[string]any) bool { return v["status"] == "answered" })
	if nodeOf(v, "source")["line"] != "a guest · slack › #help" {
		t.Errorf("the guest's request is %s", jsonOf(v))
	}
	row := al.get("/api/requests")["requests"].([]any)[0].(map[string]any)
	if row["id"] != help || jsonOf(row["by"]) != `{"id":"slack:U0GUEST","kind":"guest","name":"slack U0GUEST"}` {
		t.Errorf("the guest's row is %s", jsonOf(row))
	}

	// An answer the bridge refuses to carry shows REPLY not sent: with the
	// error to the team's people, and without it to Dana.
	refused := heard(requests.By{Person: "dana"}, "#refunds", "@bot the stand-in refuses this")
	v = w.as("lee").until("the answer is not carried", "/api/requests/"+refused, func(v map[string]any) bool { return nodeOf(v, "reply")["state"] == "failed" })
	if line := nodeOf(v, "reply")["line"].(string); !strings.HasPrefix(line, "not sent: ") || !strings.Contains(line, "The stand-in refuses the message.") {
		t.Errorf("Lee sees the reply as %q", line)
	}
	if line := nodeOf(dana.get("/api/requests/"+refused), "reply")["line"]; line != "not sent" {
		t.Errorf("Dana sees the reply as %q", line)
	}
}
