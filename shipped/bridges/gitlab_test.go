package bridges_test

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
	// status, when set, is what every comment is answered.
	status int
}

func newGitLab(t *testing.T) *gitlab {
	gl := &gitlab{nextID: 500}
	gl.Server = httptest.NewServer(http.HandlerFunc(gl.serve))
	t.Cleanup(gl.Close)
	return gl
}

// happens adds an event to the project's.
func (gl *gitlab) happens(e map[string]any) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	gl.nextID++
	e["id"] = gl.nextID
	gl.events = append([]map[string]any{e}, gl.events...)
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
		answer(200, map[string]any{"iid": 7, "title": "The webhook retries twice", "description": "Since Tuesday's release.", "web_url": gl.URL + "/platform/api/-/issues/7"})
	case strings.HasSuffix(path, "/notes") && r.Method == "POST":
		if gl.status != 0 {
			answer(gl.status, map[string]any{"message": "nope"})
			return
		}
		if !strings.HasPrefix(path, "/projects/platform%2Fapi/issues/7/") {
			answer(404, map[string]any{"message": "404 Not found"})
			return
		}
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		gl.nextID++
		body["id"] = gl.nextID
		gl.notes = append(gl.notes, body)
		answer(201, body)
	case path == "/projects/platform%2Fapi/uploads":
		answer(201, map[string]any{"markdown": "![a file](/uploads/x/file)"})
	default:
		answer(404, map[string]any{"message": "404 Not found"})
	}
}

func newGitLabBridge(t *testing.T) (*gitlab, *service) {
	gl := newGitLab(t)
	return gl, newService(t, pointed(t, "gitlab", gl.Server, "gitlab.com"), map[string]string{"GITLAB_TOKEN": gitlabToken})
}

var gitlabPlaces = map[string]any{"places": map[string]any{"platform/api": map[string]any{"team": "platform"}}}

// waitsToAsk stays until a listening run of the GitLab bridge rests
// between asks, on its timer.
func (s *service) waitsToAsk(run string) {
	s.t.Helper()
	s.corbie.Until(s.t, "the in run rests", func() bool {
		shown, err := s.corbie.Show(s.t.Context(), run)
		return err == nil && shown.Timers == 2
	})
}

// listens runs the GitLab bridge's in machine from a cursor, moving the
// clock on each time it rests, until it hears something.
func (s *service) listens(cursor any, happen func()) ([]map[string]any, any) {
	s.t.Helper()
	run := s.start("in.yaml", map[string]any{"settings": gitlabPlaces, "cursor": cursor})
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

func TestBridgesGitLabAnswersAndHearsClosed(t *testing.T) {
	t.Parallel()
	gl, s := newGitLabBridge(t)
	gl.happens(map[string]any{"action_name": "opened", "target_type": "Issue", "target_iid": 3, "author_id": 1187})
	things, cursor := s.listens(nil, func() {
		gl.happens(map[string]any{"action_name": "opened", "target_type": "Issue", "target_iid": 7, "target_title": "The webhook retries twice", "author_id": 1187})
	})
	if len(things) != 1 || things[0]["kind"] != "said" || things[0]["place"] != "platform/api#7" || things[0]["heard_in"] != "platform/api" ||
		things[0]["from"] != "1187" || !strings.HasPrefix(things[0]["text"].(string), "The webhook retries twice") {
		t.Fatalf("the open issue is heard as %v", things)
	}
	sent(t, s.carry(map[string]any{"to": "platform/api#7", "text": "Merge request 812 broke the webhook retry."}))
	if len(gl.notes) != 1 || gl.notes[0]["body"] != "Merge request 812 broke the webhook retry." {
		t.Errorf("GitLab holds the comments %v", gl.notes)
	}
	// The bridge's own comment is not heard; Raj closing the issue is.
	gl.happens(map[string]any{"action_name": "commented on", "target_type": "Note", "author_id": 9,
		"note": map[string]any{"id": gl.notes[0]["id"], "body": "Merge request 812 broke the webhook retry.", "noteable_type": "Issue", "noteable_iid": 7}})
	gl.happens(map[string]any{"action_name": "closed", "target_type": "Issue", "target_iid": 7, "author_id": 1187})
	things, _ = s.heard(map[string]any{"settings": gitlabPlaces, "cursor": cursor})
	if len(things) != 1 || things[0]["kind"] != "status" || things[0]["status"] != "closed" || things[0]["closed"] != true || things[0]["from"] != "1187" ||
		things[0]["place"] != "platform/api#7" || things[0]["about"].(map[string]any)["link"] == "" {
		t.Errorf("the close is heard as %v", things)
	}
	// What a person says in the issue is heard there.
	gl.happens(map[string]any{"action_name": "commented on", "target_type": "Note", "author_id": 1041,
		"note": map[string]any{"id": 9001, "body": "reopening: it happened again", "noteable_type": "Issue", "noteable_iid": 7}})
	if things, _ = s.heard(map[string]any{"settings": gitlabPlaces, "cursor": cursor}); len(things) != 2 || things[1]["text"] != "reopening: it happened again" || things[1]["id"] != "note:9001" {
		t.Errorf("the comment is heard as %v", things)
	}
	// An issue opened and deleted since, which GitLab no longer gives, is
	// heard by its title, and what follows it is heard all the same.
	gl.happens(map[string]any{"action_name": "opened", "target_type": "Issue", "target_iid": 99, "target_title": "Deleted since", "author_id": 1187})
	gl.happens(map[string]any{"action_name": "commented on", "target_type": "Note", "author_id": 1041,
		"note": map[string]any{"id": 9002, "body": "after it", "noteable_type": "Issue", "noteable_iid": 7}})
	things, _ = s.heard(map[string]any{"settings": gitlabPlaces, "cursor": cursor})
	if n := len(things); n != 4 || things[2]["text"] != "Deleted since" || things[3]["text"] != "after it" {
		t.Errorf("after an issue GitLab no longer gives, what is heard is %v", things)
	}
}

func testGitLabIdentityIsLasting(t *testing.T) {
	gl, s := newGitLabBridge(t)
	_, _ = s.listens(nil, func() {
		gl.happens(map[string]any{"action_name": "closed", "target_type": "Issue", "target_iid": 3, "author_id": 1187, "author": map[string]any{"username": "raj"}})
	})
	things, _ := s.heard(map[string]any{"settings": gitlabPlaces, "cursor": map[string]any{"platform/api": 0}})
	if len(things) != 1 || things[0]["from"] != "1187" {
		t.Errorf("GitLab: Raj is heard as %v", things)
	}
}

func testGitLabLongTextSplit(t *testing.T) {
	gl, s := newGitLabBridge(t)
	text := strings.Repeat("A line of a very long answer.\n", 70000)
	sent(t, s.carry(map[string]any{"to": "platform/api#7", "text": text}))
	joined := ""
	for _, n := range gl.notes {
		joined += n["body"].(string)
		if len(n["body"].(string)) > 1000000 {
			t.Errorf("GitLab: a comment holds %d characters", len(n["body"].(string)))
		}
	}
	if len(gl.notes) != 3 || joined != text {
		t.Errorf("GitLab: the text is in %d comments", len(gl.notes))
	}
}

func testGitLabErrorsByCode(t *testing.T) {
	gl, s := newGitLabBridge(t)
	if r := s.carry(map[string]any{"to": "platform/api#99", "text": "hello"}); r.Status != "done" || r.Output.(map[string]any)["refused"] == nil {
		t.Errorf("GitLab: an issue not there ends %s with %#v", r.Status, r.Output)
	}
	gl.status = 429
	if r := s.carry(map[string]any{"to": "platform/api#7", "text": "hello"}); r.Status != "failed" {
		t.Errorf("GitLab: a 429 ends %s with %#v", r.Status, r.Output)
	}
}

var _ = testkit.Monday
