package screens

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie/client"
)

// The office's page draws its teams as rooms, and shows its people and its
// bridges, each with how its in run stands.
func TestScreensOffice(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	o := w.as("dana").get("/api/office")
	if o["id"] != w.office.ID || o["name"] != "Platform support" || o["zone"] != "Europe/London" || jsonOf(o["problems"]) != "[]" ||
		o["about"] != "Help with the platform and with payments, asked in #help." {
		t.Errorf("the office is %s", jsonOf(o))
	}
	room := o["team"].(map[string]any)
	var within []string
	for _, r := range room["teams"].([]any) {
		within = append(within, r.(map[string]any)["id"].(string))
	}
	if room["id"] != "office" || jsonOf(room["lead"]) != `{"kind":"worker","name":"bot"}` || jsonOf(room["people"]) != `["al"]` ||
		strings.Join(within, " ") != "payments platform support" || room["problems"] != 0.0 {
		t.Errorf("the office's room is %s", jsonOf(room))
	}
	people := jsonOf(o["people"])
	if !strings.Contains(people, `{"bridges":["slack","gitlab"],"id":"al","name":"Al","teams":["office","platform"]}`) ||
		!strings.Contains(people, `{"bridges":["slack"],"id":"dana","name":"Dana","teams":["support"]}`) {
		t.Errorf("the people are %s", people)
	}
	bridges := jsonOf(o["bridges"])
	if !strings.Contains(bridges, `{"from":"./.bulldog/bridges/slack","in":{"status":"running"},"name":"slack","places":[{"guests":true,"place":"#help","team":"office"},{"guests":false,"place":"#refunds","team":"payments"},{"guests":false,"place":"direct","team":"office"}]}`) ||
		!strings.Contains(bridges, `"in":{"error":"GitLab answered 401 Unauthorized","status":"failing"}`) {
		t.Errorf("the bridges are %s", bridges)
	}
}

// A team's page shows what is written for each node, its people, workers,
// routines, problems and request machine, and, to its people, its docs.
func TestScreensTeam(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	lee, dana := w.as("lee"), w.as("dana")
	p := lee.get("/api/teams/payments")
	flow := p["flow"].(map[string]any)
	if jsonOf(flow["source"]) != `[{"bridge":"slack","guests":false,"place":"#refunds"}]` ||
		jsonOf(flow["heed"]) != `[{"kind":"rule","name":"./.bulldog/rules/mentioned/mentioned.js"}]` ||
		jsonOf(flow["check"]) != `[{"kind":"rule","name":"./payments/no-account-numbers.js"},{"kind":"person","name":"lee"},{"kind":"person","name":"priya"}]` ||
		jsonOf(flow["decide"]) != `[{"kind":"person","name":"lee"},{"kind":"person","name":"priya"}]` {
		t.Errorf("payments' flow is %s", jsonOf(flow))
	}
	work := flow["work"].(map[string]any)
	tools := jsonOf(work["tools"])
	if jsonOf(work["lead"]) != `{"kind":"worker","name":"ledger"}` || jsonOf(work["colleagues"]) != "[]" ||
		!strings.Contains(tools, `{"does":"Refund one charge, whole, by its id","held":true,"name":"refund"}`) ||
		!strings.Contains(tools, `"held":true,"name":"ask_team"`) || !strings.Contains(tools, `"held":false,"name":"read_doc"`) {
		t.Errorf("payments' work is %s", jsonOf(work))
	}
	workers := p["workers"].([]any)
	ledger := workers[0].(map[string]any)
	if len(workers) != 1 || ledger["name"] != "ledger" || ledger["type"] != "agent" ||
		!strings.Contains(ledger["definition"].(string), "name: claude-test, secret: TEST_KEY") {
		t.Errorf("payments' workers are %s", jsonOf(workers))
	}
	if jsonOf(p["people"]) != `[{"id":"lee","name":"Lee"},{"id":"priya","name":"Priya"}]` ||
		jsonOf(p["routines"]) != "[]" || jsonOf(p["problems"]) != "[]" || !strings.HasPrefix(p["machine"].(string), "flow: payments\n") ||
		jsonOf(p["docs"]) != "[]" {
		t.Errorf("payments' page is %s", jsonOf(p))
	}
	if d := dana.get("/api/teams/payments"); d["docs"] != nil {
		t.Errorf("Dana is shown payments' docs %v", d["docs"])
	}
	support := dana.get("/api/teams/support")["flow"].(map[string]any)
	if jsonOf(support["work"].(map[string]any)["lead"]) != `{"kind":"person","name":"dana"}` || jsonOf(support["decide"]) != `[{"kind":"person","name":"dana"}]` {
		t.Errorf("support's flow is %s", jsonOf(support))
	}
}

// The house's lead uses the fixer as its colleague, which uses sparks and
// its own held jobs; a question to a team is held only with a check; and
// each routine shows when it is next due.
func TestScreensTeamColleaguesAndRoutines(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	sam := w.as("sam")
	house := sam.get("/api/teams/office")
	work := house["flow"].(map[string]any)["work"].(map[string]any)
	tools := jsonOf(work["tools"])
	if jsonOf(work["colleagues"]) != `[{"colleagues":[{"colleagues":[],"name":"sparks","tools":[]}],"name":"fixer","tools":[`+
		`{"does":"Turn one circuit off at the fuse box","held":true,"name":"cut_power"},{"does":"Order a part for a repair, at its price in pounds","held":true,"name":"order_part"}]}]` ||
		!strings.Contains(tools, `"held":true,"name":"write_doc"`) || !strings.Contains(tools, `"held":false,"name":"ask_team"`) || strings.Contains(tools, "blob_") {
		t.Errorf("the house's work is %s", jsonOf(work))
	}
	if jsonOf(house["routines"]) != `[{"at":"mon 09:00","next":"2026-10-12T08:00:00Z","say":"What is still open?"}]` {
		t.Errorf("the house's routines are %s", jsonOf(house["routines"]))
	}
	if r := sam.get("/api/teams/garden")["routines"]; jsonOf(r) != `[{"at":"fri 16:00","next":"2026-10-09T15:00:00Z","say":"What needs doing before the weekend?"}]` {
		t.Errorf("the garden's routines are %s", jsonOf(r))
	}
}

// A team's people read and write its docs through runs of docs.yaml,
// started for the team, and its lead reads what they wrote.
func TestScreensDocs(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	lee, priya, dana := w.as("lee"), w.as("priya"), w.as("dana")
	if status, out := lee.api("PUT", "/api/teams/payments/docs/refunds.md", map[string]any{"text": "Refund within 30 days."}); status != http.StatusOK || jsonOf(out) != "{}" {
		t.Fatalf("Lee writing a doc is answered %d %v", status, out)
	}
	if doc := priya.get("/api/teams/payments/docs/refunds.md"); jsonOf(doc) != `{"name":"refunds.md","text":"Refund within 30 days."}` {
		t.Errorf("Priya reads %s", jsonOf(doc))
	}
	if docs := lee.get("/api/teams/payments")["docs"]; jsonOf(docs) != `["refunds.md"]` {
		t.Errorf("payments' docs are %s", jsonOf(docs))
	}
	if status, out := lee.api("PUT", "/api/teams/payments/docs/Refunds.MD", map[string]any{"text": "No."}); status != http.StatusBadRequest || !strings.Contains(out["error"].(string), "could not be written") {
		t.Errorf("a doc named Refunds.MD is answered %d %v", status, out)
	}
	if status, _ := dana.api("PUT", "/api/teams/payments/docs/refunds.md", map[string]any{"text": "Never."}); status != http.StatusNotFound {
		t.Errorf("Dana writing payments' doc is answered %d", status)
	}
	runs, err := w.corbie.List(ctx, client.Filter{Labels: map[string]string{"kind": "docs"}, Finished: true})
	if err != nil || len(runs) == 0 {
		t.Fatalf("the docs runs are %v (%v)", runs, err)
	}
	for _, run := range runs {
		if run.Labels["team"] != "payments" || run.Labels["bulldog"] != w.office.ID {
			t.Errorf("a docs run is labelled %v", run.Labels)
		}
	}

	asked := dana.ask("payments", "What is the refund rule?")
	if v := dana.until("payments answers", "/api/requests/"+asked, func(v map[string]any) bool { return v["status"] == "answered" }); !strings.Contains(v["answer"].(string), "Refund within 30 days.") {
		t.Errorf("the ledger answers %v", v["answer"])
	}
}

// Every request of the API answers JSON; without a key, 401.
func TestScreensAPI(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	nobody, dana := w.browser(), w.as("dana")
	for _, route := range []string{"GET /api/me", "GET /api/requests", "POST /api/requests", "GET /api/requests/x", "GET /api/requests/x/files/0",
		"POST /api/requests/x/say", "POST /api/requests/x/stop", "POST /api/waits/x/answer", "GET /api/office", "GET /api/teams/office",
		"GET /api/teams/office/docs/a", "PUT /api/teams/office/docs/a", "GET /api/events"} {
		method, path, _ := strings.Cut(route, " ")
		if status, out := nobody.api(method, path, map[string]any{}); status != http.StatusUnauthorized || out["error"] != "Sign in first." {
			t.Errorf("%s with no key is answered %d %v", route, status, out)
		}
	}
	res := dana.do("POST", "/api/requests", strings.NewReader("Was it refunded?"))
	if res.StatusCode != http.StatusBadRequest || res.Header.Get("Content-Type") != "application/json" {
		t.Errorf("a body that is not JSON is answered %s", res.Status)
	}
	if status, out := dana.api("GET", "/api/nothing", nil); status != http.StatusNotFound || out["error"] != "There is nothing here." {
		t.Errorf("an address of nothing is answered %d %v", status, out)
	}
	if list := dana.get("/api/requests"); jsonOf(list) != `{"requests":[],"waiting":[]}` {
		t.Errorf("Dana's list is %s", jsonOf(list))
	}
	for path, kind := range map[string]string{"/": "text/html; charset=utf-8", "/static/style.css": "text/css; charset=utf-8",
		"/static/core.js": "text/javascript; charset=utf-8", "/static/plex-sans.woff2": "font/woff2"} {
		if res := nobody.do("GET", path, nil); res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != kind {
			t.Errorf("%s is served %s as %s", path, res.Status, res.Header.Get("Content-Type"))
		}
	}
	if res := nobody.do("GET", "/static/nothing.js", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a file the screens do not have is served %s", res.Status)
	}

	// A request the browser gave up on is no failure to report.
	quiet := &Screens{Report: func(err error) { t.Errorf("the screens report %v", err) }}
	quiet.refuse(httptest.NewRecorder(), fmt.Errorf("listing the runs: %w", context.Canceled))
}

func TestScreensLoadNothingFromElsewhere(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	nobody, sam := w.browser(), w.as("sam")
	for _, path := range []string{"/", "/static/core.js", "/static/style.css", "/signin", "/key?p=sam&k=0", "/api/me", "/api/nothing"} {
		for _, b := range []*browser{nobody, sam} {
			if res := b.do("GET", path, nil); res.Header.Get("Content-Security-Policy") != "default-src 'self'" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("%s is served with %v", path, res.Header)
			}
		}
	}
	err := fs.WalkDir(static, "static", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(name, ".woff2") {
			return err
		}
		text, err := fs.ReadFile(static, name)
		for _, elsewhere := range []string{"http:", "https:", "src=\"//", "href=\"//", "url(//"} {
			if strings.Contains(string(text), elsewhere) {
				t.Errorf("%s names somewhere else: %s", name, elsewhere)
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
