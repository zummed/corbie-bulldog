package shipped_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

const gitlabToken = "glpat-the-bridges-own-token"

// A gitlab stands in for GitLab's REST API on one project: it keeps the
// project's events, newest first, and the comments the bridge made.
type gitlab struct {
	*httptest.Server
	mu     sync.Mutex
	events []map[string]any
	notes  []map[string]any
	nextID int
}

func newGitLab(t *testing.T) *gitlab {
	gl := &gitlab{nextID: 500}
	gl.Server = httptest.NewServer(http.HandlerFunc(gl.serve))
	t.Cleanup(gl.Close)
	return gl
}

// happens adds an event to the project's, and gives its id.
func (gl *gitlab) happens(e map[string]any) int {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	gl.nextID++
	e["id"] = gl.nextID
	e["created_at"] = testkit.Monday.Format(time.RFC3339)
	gl.events = append([]map[string]any{e}, gl.events...)
	return gl.nextID
}

// comments has someone comment on issue 7.
func (gl *gitlab) comments(author, note int, body string) int {
	return gl.happens(map[string]any{"action_name": "commented on", "target_type": "Note", "author_id": author,
		"note": map[string]any{"id": note, "body": body, "noteable_type": "Issue", "noteable_iid": 7}})
}

func (gl *gitlab) serve(w http.ResponseWriter, r *http.Request) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	answer := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	if r.Header.Get("PRIVATE-TOKEN") != gitlabToken {
		answer(401, map[string]any{"message": "401 Unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v4")
	switch {
	case path == "/user":
		answer(200, map[string]any{"id": 9, "username": "bulldog"})
	case path == "/projects/platform%2Fapi/events":
		per, page := 20, 1
		fmt.Sscan(r.URL.Query().Get("per_page"), &per)
		fmt.Sscan(r.URL.Query().Get("page"), &page)
		from := min((page-1)*per, len(gl.events))
		answer(200, append([]map[string]any{}, gl.events[from:min(from+per, len(gl.events))]...))
	case path == "/projects/platform%2Fapi/issues/7":
		answer(200, map[string]any{"iid": 7, "title": "The webhook retries twice", "description": "Since Tuesday's release."})
	case path == "/projects/platform%2Fapi/issues/7/notes" && r.Method == "POST":
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		gl.nextID++
		body["id"] = gl.nextID
		gl.notes = append(gl.notes, body)
		answer(201, body)
	default:
		answer(404, map[string]any{"message": "404 Not found"})
	}
}

// newGitLabBridge gives the shipped GitLab bridge, given the stand-in's
// address as its site, in a Corbie of the test's own.
func newGitLabBridge(t *testing.T) (*gitlab, *service) {
	gl := newGitLab(t)
	settings := map[string]any{"site": strings.TrimPrefix(gl.URL, "http://"), "places": map[string]any{"platform/api": map[string]any{"team": "platform"}}}
	return gl, newService(t, "gitlab", gl.Server, map[string]string{"GITLAB_TOKEN": gitlabToken}, settings)
}

// asksAgain stays until the in rests, has things happen, and moves the
// clock past its rest.
func (s *service) asksAgain(run string, happen func()) {
	s.t.Helper()
	s.rests(run)
	happen()
	s.corbie.Clock.Move(30 * time.Second)
}

func TestGitLabHearsAnIssueAndAComment(t *testing.T) {
	t.Parallel()
	gl, s := newGitLabBridge(t)
	before := gl.happens(map[string]any{"action_name": "opened", "target_type": "Issue", "target_iid": 3, "author_id": 1187})
	run := s.listen(nil)
	// A new bridge hears nothing that happened before it, and notes where
	// the project stands.
	if things, cursor := s.hears(run, 0); len(things) != 0 || fmt.Sprint(cursor) != fmt.Sprintf("map[platform/api:%d]", before) {
		t.Fatalf("a new bridge heard %v, with the cursor %v", things, cursor)
	}
	var opened int
	s.asksAgain(run, func() {
		opened = gl.happens(map[string]any{"action_name": "opened", "target_type": "Issue", "target_iid": 7, "target_title": "The webhook retries twice", "author_id": 1187})
		gl.comments(1041, 9001, "@bulldog is this the retry bug?")
	})
	things, _ := s.hears(run, 2)
	var got []string
	for _, th := range things {
		got = append(got, fieldsOf(th, "id", "at", "kind", "from", "place", "where", "text", "mentioned", "answers", "ref", "choice"))
	}
	want := []string{
		fmt.Sprintf("event:%d | 2026-10-05T09:00:00.000Z | said | 1187 | platform/api | platform/api#7 | The webhook retries twice\n\nSince Tuesday's release. | false | <nil> | <nil> | <nil>", opened),
		"note:9001 | 2026-10-05T09:00:00.000Z | said | 1041 | platform/api | platform/api#7 | @bulldog is this the retry bug? | true | <nil> | <nil> | <nil>",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("GitLab hands over\n%s\nnot\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// The next job asks from the cursor the last one ended with: only what
	// is new is heard.
	var last int
	s.asksAgain(run, func() { last = gl.comments(1041, 9002, "it happened again") })
	things, cursor := s.hears(run, 3)
	if len(things) != 3 || things[2]["id"] != "note:9002" || fmt.Sprint(cursor) != fmt.Sprintf("map[platform/api:%d]", last) {
		t.Errorf("GitLab hands over %v, with the cursor %v", things, cursor)
	}
}

func TestGitLabSaysOneComment(t *testing.T) {
	t.Parallel()
	gl, s := newGitLabBridge(t)
	id := sent(t, s.say(map[string]any{"to": "platform/api#7", "text": "Merge request 812 broke the webhook retry."}))
	gl.mu.Lock()
	if len(gl.notes) != 1 || gl.notes[0]["body"] != "Merge request 812 broke the webhook retry." || id != fmt.Sprintf("note:%v", gl.notes[0]["id"]) {
		t.Errorf("GitLab holds the comments %v, and the out sent %s", gl.notes, id)
	}
	gl.mu.Unlock()
	// What GitLab refuses fails the out with its words.
	if r := s.say(map[string]any{"to": "platform/api#99", "text": "hello"}); r.Status != "failed" || !strings.Contains(r.Error, "404 Not found") || strings.Contains(r.Error, gitlabToken) {
		t.Errorf("an issue not there ends %s: %s", r.Status, r.Error)
	}
	// Given no site, nothing is sent, and the failure names it.
	s.settings = map[string]any{}
	if r := s.say(map[string]any{"to": "platform/api#7", "text": "hello"}); r.Status != "failed" || !strings.Contains(r.Error, "site") {
		t.Errorf("with no site the out ends %s: %s", r.Status, r.Error)
	}
}

func gitlabFromIsLasting(t *testing.T) {
	gl, s := newGitLabBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	s.asksAgain(run, func() {
		gl.happens(map[string]any{"action_name": "commented on", "target_type": "Note", "author_id": 1187, "author": map[string]any{"username": "raj"},
			"note": map[string]any{"id": 9001, "body": "hello", "noteable_type": "Issue", "noteable_iid": 7}})
	})
	if things, _ := s.hears(run, 1); things[0]["from"] != "1187" {
		t.Errorf("Raj is heard as %v", things[0]["from"])
	}
}

func gitlabBotsNotHeard(t *testing.T) {
	gl, s := newGitLabBridge(t)
	run := s.listen(nil)
	s.hears(run, 0)
	var own, system, closed int
	s.asksAgain(run, func() {
		own = gl.comments(9, 9001, "the bridge's own answer")
		system = gl.happens(map[string]any{"action_name": "commented on", "target_type": "Note", "author_id": 1041,
			"note": map[string]any{"id": 9002, "body": "changed the description", "noteable_type": "Issue", "noteable_iid": 7, "system": true}})
		closed = gl.happens(map[string]any{"action_name": "closed", "target_type": "Issue", "target_iid": 7, "author_id": 1041})
		gl.comments(1041, 9003, "and the lawn")
	})
	things, _ := s.hears(run, 1)
	want := fmt.Sprintf("[left out event:%d in platform/api: own left out event:%d in platform/api: a system note left out event:%d in platform/api: closed]", own, system, closed)
	if len(things) != 1 || things[0]["text"] != "and the lawn" || fmt.Sprint(s.logged(run)) != want {
		t.Errorf("GitLab hands over %v, and logs %q", things, s.logged(run))
	}
}
