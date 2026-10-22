package shipped_test

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
	jiraRaj    = "5b10a2844c20165700ede21g"
	jiraAl     = "5b10ac8d82e05b22cc7d4ef5"
)

// A jira stands in for Jira Cloud's REST API: it keeps issues as a search
// gives them, and the comments the bridge made.
type jira struct {
	*httptest.Server
	mu       sync.Mutex
	issues   []map[string]any
	comments []map[string]any
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
		answer(200, map[string]any{"issues": append([]map[string]any{}, j.issues...), "isLast": true})
	case path == "/issue/OPS-12/comment" && r.Method == "POST":
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		body["id"] = fmt.Sprint(1000 + len(j.comments))
		j.comments = append(j.comments, body)
		answer(201, body)
	default:
		answer(404, map[string]any{"errorMessages": []string{"Issue does not exist or you do not have permission to see it."}})
	}
}

// opened has Raj open OPS-12 at a time, with its comments.
func (j *jira) opened(at time.Time, comments ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.issues = []map[string]any{{"id": "10012", "key": "OPS-12", "fields": map[string]any{
		"summary": "Deploys hang", "description": "Since Tuesday.", "created": stamp(at),
		"reporter": map[string]any{"accountId": jiraRaj, "accountType": "atlassian"},
		"comment":  map[string]any{"comments": comments},
	}}}
}

// comment is a comment on OPS-12 by an account, made at a time.
func comment(id, account, accountType, body string, at time.Time) map[string]any {
	return map[string]any{"id": id, "body": body, "created": stamp(at),
		"author": map[string]any{"accountId": account, "accountType": accountType}}
}

// newJiraBridge gives the shipped Jira bridge, given the stand-in's
// address as its site, in a Corbie of the test's own.
func newJiraBridge(t *testing.T) (*jira, *service) {
	j := newJira(t)
	settings := map[string]any{"site": strings.TrimPrefix(j.URL, "http://"), "email": jiraEmail, "places": map[string]any{"OPS": map[string]any{"team": "platform"}}}
	return j, newService(t, "jira", j.Server, map[string]string{"JIRA_TOKEN": jiraToken}, settings)
}

func TestJiraHearsAnIssueAndAComment(t *testing.T) {
	t.Parallel()
	j, s := newJiraBridge(t)
	run := s.listen(nil)
	// A new bridge hears from now on.
	if things, cursor := s.hears(run, 0); len(things) != 0 || cursor.(map[string]any)["since"] == nil {
		t.Fatalf("a new bridge heard %v, with the cursor %v", things, cursor)
	}
	// Raj opens OPS-12 a second after the bridge first asked, and Al
	// comments on it.
	opened := time.Now().Add(time.Second)
	asked := comment("2001", jiraAl, "atlassian", "[~accountid:"+jiraBridge+"] is this the runner?", opened.Add(time.Millisecond))
	s.asksAgain(run, func() { j.opened(opened, asked) })
	things, _ := s.hears(run, 2)
	var got []string
	for _, th := range things {
		got = append(got, fieldsOf(th, "id", "kind", "from", "place", "where", "text", "mentioned", "answers", "ref", "choice"))
	}
	want := []string{
		"issue:10012 | said | " + jiraRaj + " | OPS | OPS-12 | Deploys hang\n\nSince Tuesday. | false | <nil> | <nil> | <nil>",
		"comment:2001 | said | " + jiraAl + " | OPS | OPS-12 | [~accountid:" + jiraBridge + "] is this the runner? | true | <nil> | <nil> | <nil>",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Jira hands over\n%s\nnot\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if at, err := time.Parse(time.RFC3339, things[0]["at"].(string)); err != nil || time.Since(at) > time.Minute {
		t.Errorf("the issue is heard at %v", things[0]["at"])
	}
	// The next job asks from the cursor the last one ended with: only what
	// is new is heard.
	s.asksAgain(run, func() {
		j.opened(opened, asked, comment("2002", jiraAl, "atlassian", "it is", opened.Add(time.Second)))
	})
	if things, _ := s.hears(run, 3); len(things) != 3 || things[2]["id"] != "comment:2002" {
		t.Errorf("Jira hands over %v", things)
	}
}

func TestJiraSaysOneMessage(t *testing.T) {
	t.Parallel()
	j, s := newJiraBridge(t)
	id := sent(t, s.say(map[string]any{"to": "OPS-12", "text": "The runner is out of disk."}))
	j.mu.Lock()
	if len(j.comments) != 1 || j.comments[0]["body"] != "The runner is out of disk." || id != "comment:1000" {
		t.Errorf("Jira holds the comments %v, and the out sent %s", j.comments, id)
	}
	j.mu.Unlock()
	// A long text goes in as many comments as it takes.
	text := strings.Repeat("A line of a long answer.\n", 3000)
	if id := sent(t, s.say(map[string]any{"to": "OPS-12", "text": text})); id != "comment:1003" {
		t.Errorf("the long text was sent as %s", id)
	}
	j.mu.Lock()
	joined := ""
	for _, c := range j.comments[1:] {
		joined += c["body"].(string)
	}
	if len(j.comments) != 4 || joined != text {
		t.Errorf("the long text is in %d comments", len(j.comments)-1)
	}
	j.mu.Unlock()
	// What Jira refuses fails the out with its words.
	if r := s.say(map[string]any{"to": "OPS-99", "text": "hello"}); r.Status != "failed" || !strings.Contains(r.Error, "Issue does not exist") || strings.Contains(r.Error, jiraToken) {
		t.Errorf("an issue not there ends %s: %s", r.Status, r.Error)
	}
}

func jiraFromIsLasting(t *testing.T) {
	j, s := newJiraBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	s.asksAgain(run, func() { j.opened(time.Now().Add(time.Second)) })
	if things, _ := s.hears(run, 1); things[0]["from"] != jiraRaj {
		t.Errorf("Raj is heard as %v", things[0]["from"])
	}
}

func jiraBotsNotHeard(t *testing.T) {
	j, s := newJiraBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	opened := time.Now().Add(time.Second)
	s.asksAgain(run, func() {
		j.opened(opened, comment("2001", jiraBridge, "atlassian", "the bridge's own answer", opened.Add(time.Millisecond)),
			comment("2002", "5b10automation000000001", "app", "the words of an app", opened.Add(2*time.Millisecond)),
			comment("2003", jiraAl, "atlassian", "and the lawn", opened.Add(3*time.Millisecond)))
	})
	things, _ := s.hears(run, 2)
	want := "[left out comment:2001 in OPS-12: own left out comment:2002 in OPS-12: an app wrote it]"
	if len(things) != 2 || things[1]["text"] != "and the lawn" || fmt.Sprint(s.logged(run)) != want {
		t.Errorf("Jira hands over %v, and logs %q", things, s.logged(run))
	}
}
