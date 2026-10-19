package bridges_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	jiraEmail  = "bulldog@example.com"
	jiraToken  = "ATATT-the-bridges-own-token"
	jiraBridge = "5b10bulldog0000000000001"
)

// A jira stands in for Jira Cloud's REST API on one project: it keeps
// its issues as search gives them, and the comments the bridge made.
type jira struct {
	*httptest.Server
	mu       sync.Mutex
	issues   []map[string]any
	comments []map[string]any
	status   int
}

func newJira(t *testing.T) *jira {
	j := &jira{}
	j.Server = httptest.NewServer(http.HandlerFunc(j.serve))
	t.Cleanup(j.Close)
	return j
}

// stamp writes a time as Jira does.
func stamp(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000-0700") }

func (j *jira) serve(w http.ResponseWriter, r *http.Request) {
	j.mu.Lock()
	defer j.mu.Unlock()
	answer := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	if r.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte(jiraEmail+":"+jiraToken)) {
		answer(401, map[string]any{"errorMessages": []string{"You are not signed in."}})
		return
	}
	switch path := strings.TrimPrefix(r.URL.Path, "/rest/api/2"); {
	case path == "/myself":
		answer(200, map[string]any{"accountId": jiraBridge, "emailAddress": jiraEmail})
	case path == "/search/jql":
		if !strings.Contains(r.URL.Query().Get("jql"), `project in ("OPS")`) {
			answer(400, map[string]any{"errorMessages": []string{"bad jql"}})
			return
		}
		// Searches give a page at a time, as many as maxResults asks.
		per, from := 50, 0
		fmt.Sscan(r.URL.Query().Get("maxResults"), &per)
		fmt.Sscan(r.URL.Query().Get("nextPageToken"), &from)
		to := min(from+per, len(j.issues))
		page := map[string]any{"issues": append([]map[string]any{}, j.issues[from:to]...), "isLast": to == len(j.issues)}
		if to < len(j.issues) {
			page["nextPageToken"] = fmt.Sprint(to)
		}
		answer(200, page)
	case path == "/issue/OPS-12/comment" && r.Method == "POST":
		if j.status != 0 {
			answer(j.status, map[string]any{"errorMessages": []string{"no"}})
			return
		}
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		body["id"] = "100" + string(rune('0'+len(j.comments)))
		j.comments = append(j.comments, body)
		answer(201, body)
	default:
		answer(404, map[string]any{"errorMessages": []string{"Issue does not exist or you do not have permission to see it."}})
	}
}

func newJiraBridge(t *testing.T) (*jira, *service) {
	j := newJira(t)
	s := newService(t, pointed(t, "jira", j.Server), map[string]string{"JIRA_TOKEN": jiraToken})
	s.site = strings.TrimPrefix(j.URL, "http://")
	return j, s
}

var jiraSettings = map[string]any{"email": jiraEmail, "places": map[string]any{"OPS": map[string]any{"team": "platform"}}}

// listensToJira runs the Jira bridge's in machine from a cursor, moving the
// clock on each time it rests, until it hears something.
func (s *service) listensToJira(cursor any, happen func()) ([]map[string]any, any) {
	s.t.Helper()
	run := s.start("in.yaml", map[string]any{"settings": jiraSettings, "cursor": cursor})
	s.waitsToAsk(run)
	happen()
	s.corbie.Clock.Move(30 * time.Second)
	ended := s.corbie.Ended(s.t, run)
	if ended.Status != "done" {
		s.t.Fatalf("the in run ended %s: %s", ended.Status, ended.Error)
	}
	out := ended.Output.(map[string]any)
	var things []map[string]any
	for _, t := range out["heard"].([]any) {
		things = append(things, t.(map[string]any))
	}
	return things, out["cursor"]
}

func (j *jira) opened(at time.Time, status, category string, comments []any, histories []any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.issues = []map[string]any{{"id": "10012", "key": "OPS-12", "fields": map[string]any{
		"summary": "Deploys hang", "description": "Since Tuesday.", "created": stamp(at),
		"reporter": map[string]any{"accountId": "5b10a2844c20165700ede21g", "displayName": "Raj"},
		"status":   map[string]any{"name": status, "statusCategory": map[string]any{"key": category}},
		"comment":  map[string]any{"comments": comments},
	}, "changelog": map[string]any{"histories": histories}}}
}

func TestBridgesJiraAnswersAndHearsStatus(t *testing.T) {
	t.Parallel()
	j, s := newJiraBridge(t)
	opened := time.Now().Add(time.Second)
	things, cursor := s.listensToJira(nil, func() { j.opened(opened, "To Do", "new", nil, nil) })
	if len(things) != 1 || things[0]["kind"] != "said" || things[0]["place"] != "OPS-12" || things[0]["heard_in"] != "OPS" ||
		things[0]["from"] != "5b10a2844c20165700ede21g" || things[0]["text"] != "Deploys hang\n\nSince Tuesday." {
		t.Fatalf("the new issue is heard as %v", things)
	}
	sent(t, s.carry(map[string]any{"settings": jiraSettings, "to": "OPS-12", "text": "The runner is out of disk."}))
	if len(j.comments) != 1 || j.comments[0]["body"] != "The runner is out of disk." {
		t.Errorf("Jira holds the comments %v", j.comments)
	}
	closed := opened.Add(time.Second)
	j.opened(opened, "Done", "done", []any{map[string]any{"id": "2001", "body": "Fixed.", "created": stamp(closed), "author": map[string]any{"accountId": "5b10ac8d82e05b22cc7d4ef5"}}},
		[]any{map[string]any{"id": "3001", "created": stamp(closed), "author": map[string]any{"accountId": "5b10ac8d82e05b22cc7d4ef5"},
			"items": []any{map[string]any{"field": "status", "fromString": "To Do", "toString": "Done"}}}})
	things, _ = s.heard(map[string]any{"settings": jiraSettings, "cursor": cursor})
	if len(things) != 2 || things[0]["text"] != "Fixed." || things[1]["kind"] != "status" || things[1]["status"] != "Done" || things[1]["closed"] != true ||
		things[1]["about"].(map[string]any)["link"] == "" {
		t.Errorf("the close is heard as %v", things)
	}
}

func TestBridgesJiraHearsAllOnceAndNotItself(t *testing.T) {
	t.Parallel()
	j, s := newJiraBridge(t)
	// Sixty issues, beyond one search's page, three opened at each moment;
	// a comment of the bridge's own on each, as the out machine makes, is
	// not heard back.
	opened := func() {
		at := time.Now().Add(time.Second)
		j.mu.Lock()
		defer j.mu.Unlock()
		for i := range 60 {
			created := at.Add(time.Duration(i/3) * time.Millisecond)
			j.issues = append(j.issues, map[string]any{"id": fmt.Sprint(20000 + i), "key": fmt.Sprintf("OPS-%d", 100+i), "fields": map[string]any{
				"summary": fmt.Sprintf("issue %d", i), "created": stamp(created),
				"reporter": map[string]any{"accountId": "5b10a2844c20165700ede21g"},
				"comment": map[string]any{"comments": []any{map[string]any{"id": fmt.Sprint(30000 + i), "body": "on it", "created": stamp(created),
					"author": map[string]any{"accountId": jiraBridge}}}},
			}})
		}
	}
	heard := map[string]int{}
	took := func(things []map[string]any) {
		for _, th := range things {
			heard[th["id"].(string)]++
			if th["from"] == jiraBridge {
				t.Errorf("the bridge's own comment is heard: %v", th)
			}
		}
	}
	things, cursor := s.listensToJira(nil, opened)
	took(things)
	// Each run hears at most twenty; one with nothing new would rest, so
	// no more are run than the sixty take.
	for range 2 {
		things, cursor = s.heard(map[string]any{"settings": jiraSettings, "cursor": cursor})
		took(things)
	}
	if len(heard) != 60 {
		t.Errorf("of sixty issues, %d were heard", len(heard))
	}
	for id, n := range heard {
		if n != 1 {
			t.Errorf("%s was heard %d times", id, n)
		}
	}
}

func testJiraIdentityIsLasting(t *testing.T) {
	j, s := newJiraBridge(t)
	things, _ := s.listensToJira(nil, func() { j.opened(time.Now().Add(time.Second), "To Do", "new", nil, nil) })
	if len(things) != 1 || things[0]["from"] != "5b10a2844c20165700ede21g" {
		t.Errorf("Jira: Raj is heard as %v", things)
	}
}

func testJiraLongTextSplit(t *testing.T) {
	j, s := newJiraBridge(t)
	text := strings.Repeat("A line of a long answer.\n", 3000)
	sent(t, s.carry(map[string]any{"settings": jiraSettings, "to": "OPS-12", "text": text}))
	joined := ""
	for _, c := range j.comments {
		joined += c["body"].(string)
		if len(c["body"].(string)) > 32767 {
			t.Errorf("Jira: a comment holds %d characters", len(c["body"].(string)))
		}
	}
	if len(j.comments) != 3 || joined != text {
		t.Errorf("Jira: the text is in %d comments", len(j.comments))
	}
}

func testJiraErrorsByCode(t *testing.T) {
	j, s := newJiraBridge(t)
	if r := s.carry(map[string]any{"settings": jiraSettings, "to": "OPS-99", "text": "hello"}); r.Status != "done" || r.Output.(map[string]any)["refused"] == nil {
		t.Errorf("Jira: an issue not there ends %s with %#v", r.Status, r.Output)
	}
	j.status = 429
	if r := s.carry(map[string]any{"settings": jiraSettings, "to": "OPS-12", "text": "hello"}); r.Status != "failed" {
		t.Errorf("Jira: a 429 ends %s with %#v", r.Status, r.Output)
	}
}
