package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// address gives where a serving listens, as its log says.
func (s *serving) address() string {
	s.b.t.Helper()
	for _, line := range s.lines() {
		if _, at, ok := strings.Cut(line, "listening on "); ok {
			return "http://" + at
		}
	}
	s.b.t.Fatalf("the server is not listening:\n%s", strings.Join(s.lines(), "\n"))
	return ""
}

// A browser calls a server as a person's browser does: with their key in
// its cookie, or with none.
type browser struct {
	s      *serving
	key    string
	header http.Header
}

// as gives the browser of a person signed in with their key.
func (s *serving) as(person string) *browser {
	s.b.t.Helper()
	key, err := keyOf(s.b.store(), person)
	if err != nil {
		s.b.t.Fatal(err)
	}
	return &browser{s: s, key: key, header: http.Header{}}
}

// nobody gives a browser with no key.
func (s *serving) nobody() *browser { return &browser{s: s, header: http.Header{}} }

// A reply is how the server answered.
type reply struct {
	t      *testing.T
	status int
	header http.Header
	body   []byte
}

// json gives the reply's body as JSON.
func (r reply) json() map[string]any {
	r.t.Helper()
	var out map[string]any
	if err := json.Unmarshal(r.body, &out); err != nil {
		r.t.Fatalf("the answer %d is not JSON: %v\n%s", r.status, err, r.body)
	}
	return out
}

// into reads the reply's body into what it is to be.
func (r reply) into(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		r.t.Fatalf("the answer %d cannot be read: %v\n%s", r.status, err, r.body)
	}
}

// is fails the test unless the reply has a status.
func (r reply) is(status int) reply {
	r.t.Helper()
	if r.status != status {
		r.t.Fatalf("answered %d, not %d: %s", r.status, status, r.body)
	}
	return r
}

// cookie gives the key a reply gave the browser to keep, or "".
func (r reply) cookie() *http.Cookie {
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == Cookie {
			return c
		}
	}
	return nil
}

var noRedirects = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// do sends a request with a body of the type given.
func (br *browser) do(method, path, kind string, body []byte) reply {
	br.s.b.t.Helper()
	req, err := http.NewRequest(method, br.s.address()+path, bytes.NewReader(body))
	if err != nil {
		br.s.b.t.Fatal(err)
	}
	maps.Copy(req.Header, br.header)
	if kind != "" {
		req.Header.Set("Content-Type", kind)
	}
	if br.key != "" {
		req.AddCookie(&http.Cookie{Name: Cookie, Value: br.key})
	}
	res, err := noRedirects.Do(req)
	if err != nil {
		br.s.b.t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		br.s.b.t.Fatal(err)
	}
	return reply{t: br.s.b.t, status: res.StatusCode, header: res.Header, body: data}
}

func (br *browser) get(path string) reply { return br.do(http.MethodGet, path, "", nil) }

// post sends a body as JSON: a string as it is, and anything else
// written as JSON.
func (br *browser) post(path string, body any) reply {
	br.s.b.t.Helper()
	if text, ok := body.(string); ok {
		return br.do(http.MethodPost, path, "application/json", []byte(text))
	}
	data, err := json.Marshal(body)
	if err != nil {
		br.s.b.t.Fatal(err)
	}
	return br.do(http.MethodPost, path, "application/json", data)
}

// form sends a form of several parts: its fields, and each file.
func (br *browser) form(path string, fields map[string]string, files map[string][]byte) reply {
	br.s.b.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for name, value := range fields {
		w.WriteField(name, value)
	}
	for name, data := range files {
		part, _ := w.CreateFormFile("files", name)
		part.Write(data)
	}
	w.Close()
	return br.do(http.MethodPost, path, w.FormDataContentType(), body.Bytes())
}

// An event is one of a stream's.
type event struct {
	id, name, data string
	comment        bool
}

// A stream is a stream of events a browser is reading.
type stream struct {
	t      *testing.T
	res    *http.Response
	events chan event
	stop   func()
}

// stream opens a stream, and reads its events as they come until the
// test ends.
func (br *browser) stream(path string) *stream {
	br.s.b.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, br.s.address()+path, nil)
	maps.Copy(req.Header, br.header)
	if br.key != "" {
		req.AddCookie(&http.Cookie{Name: Cookie, Value: br.key})
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		br.s.b.t.Fatal(err)
	}
	st := &stream{t: br.s.b.t, res: res, events: make(chan event, 1000), stop: cancel}
	br.s.b.t.Cleanup(func() {
		cancel()
		res.Body.Close()
	})
	go func() {
		defer close(st.events)
		lines := bufio.NewScanner(res.Body)
		var e event
		for lines.Scan() {
			line := lines.Text()
			switch {
			case line == "":
				if e != (event{}) {
					st.events <- e
				}
				e = event{}
			case strings.HasPrefix(line, ":"):
				e.comment = true
			case strings.HasPrefix(line, "id: "):
				e.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				e.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				e.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return st
}

// watching gives the address of a run's stream: a run's name holds a /,
// which is escaped.
func watching(run string) string { return "/api/runs/" + url.PathEscape(run) + "/watch" }

// watches says how a browser's watch of a run is answered, without
// staying to read what the run does.
func (br *browser) watches(run string) int {
	br.s.b.t.Helper()
	st := br.stream(watching(run))
	st.stop()
	return st.res.StatusCode
}

// next gives the stream's next event, failing the test if none comes.
func (st *stream) next() event {
	st.t.Helper()
	select {
	case e, ok := <-st.events:
		if !ok {
			st.t.Fatal("the stream ended")
		}
		return e
	case <-time.After(testkit.Patience):
		st.t.Fatalf("waited %v for an event of the stream", testkit.Patience)
		return event{}
	}
}

// until reads the stream until an event that is wanted, and gives it.
func (st *stream) until(what string, wanted func(event) bool) event {
	st.t.Helper()
	limit := time.After(testkit.Patience)
	for {
		select {
		case e, ok := <-st.events:
			if !ok {
				st.t.Fatalf("the stream ended before %s", what)
			}
			if wanted(e) {
				return e
			}
		case <-limit:
			st.t.Fatalf("waited %v, and the stream never had %s", testkit.Patience, what)
		}
	}
}

// none says whether the stream has nothing more for now: once the test's
// clock has moved, and every stream has been looked at again, nothing
// waits in it.
func (st *stream) none() (event, bool) {
	select {
	case e := <-st.events:
		return e, false
	default:
		return event{}, true
	}
}

// going gives the runs at the top of a kind that have not ended, of a
// bridge when one is named, oldest first.
func (b *bench) going(kind, bridge string) []*client.Run {
	b.t.Helper()
	labels := map[string]string{work.LabelKind: kind}
	if bridge != "" {
		labels[bridges.LabelBridge] = bridge
	}
	runs := slices.DeleteFunc(b.listed(b.identity(), labels), func(r *client.Run) bool { return r.HasEnded() })
	slices.SortFunc(runs, func(a, c *client.Run) int { return strings.Compare(a.ID, c.ID) })
	return runs
}

// input gives the input a run was started with, and the text it carries
// read from its blob.
func (b *bench) input(run string) map[string]any {
	b.t.Helper()
	var in map[string]any
	b.until("the run "+run+" has started", func() bool {
		step, err := b.corbie.Step(ctx, run, 1)
		if errors.Is(err, client.ErrNoRecord) || errors.Is(err, client.ErrNoRun) {
			return false
		}
		if err != nil {
			b.t.Fatal(err)
		}
		in, _ = step.Events[0].Data.(map[string]any)
		return true
	})
	if blob, ok := in["text"].(map[string]any); ok {
		in["text"] = b.fetch(run, blob["$blob"].(string))
	}
	return in
}

// fetch gives a blob of a run's, by the path it is shown with.
func (b *bench) fetch(run, path string) string {
	b.t.Helper()
	r, _, err := b.corbie.Fetch(ctx, run, path)
	if err != nil {
		b.t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		b.t.Fatal(err)
	}
	return string(data)
}

// waiting stays until a run of a kind on a bridge waits on a stand-in's
// ticket, carrying to a place when one is named, and gives it and its
// input.
func (b *bench) waiting(kind, bridge, ticket, to string) (string, map[string]any) {
	b.t.Helper()
	run := ""
	var in map[string]any
	b.until(fmt.Sprintf("a %s run of %s waits on %s, to %q", kind, bridge, ticket, to), func() bool {
		for _, r := range b.going(kind, bridge) {
			if !slices.Contains(r.Tickets, ticket) {
				continue
			}
			if got := b.input(r.ID); to == "" || got["to"] == to {
				run, in = r.ID, got
			}
		}
		return run != ""
	})
	return run, in
}

// answerTicket answers the ticket a run waits on.
func (b *bench) answerTicket(run, ticket string, value any) {
	b.t.Helper()
	if _, err := b.corbie.Reply(ctx, run, b.corbie.Waiting(b.t, run, ticket), value, 0); err != nil {
		b.t.Fatal(err)
	}
}

// things counts the things the stand-ins hear, so each has an id of its
// own.
var things atomic.Int64

// thing gives a thing a stand-in in run hears.
func thing(kind, from, heardIn, place string, more ...any) map[string]any {
	t := map[string]any{"id": fmt.Sprintf("t%d", things.Add(1)), "kind": kind, "from": from, "heard_in": heardIn, "place": place}
	for i := 0; i+1 < len(more); i += 2 {
		t[more[i].(string)] = more[i+1]
	}
	return t
}

// delivered has a bridge's address called, as its service calls it: its
// hearing run ends with what it heard, and the service is answered. A
// bridge not yet read hears nothing, and is called again.
func (s *serving) delivered(bridge string, heard ...map[string]any) reply {
	s.b.t.Helper()
	answered := make(chan reply, 1)
	call := func() { answered <- s.nobody().post("/bridges/"+bridge, "{}") }
	go call()
	run := ""
	s.b.until("the hearing run of "+bridge+" waits", func() bool {
		select {
		case r := <-answered:
			if r.status != http.StatusNotFound && r.status != http.StatusServiceUnavailable {
				s.b.t.Fatalf("the bridge's address answered %d %s before it heard anything", r.status, r.body)
			}
			s.b.clock.Move(0)
			go call()
		default:
		}
		for _, r := range s.b.going(bridges.KindIn, bridge) {
			if slices.Contains(r.Tickets, standin.Heard) {
				run = r.ID
			}
		}
		return run != ""
	})
	list := []any{}
	for _, h := range heard {
		list = append(list, h)
	}
	s.b.answerTicket(run, standin.Heard, map[string]any{"heard": list})
	return <-answered
}

// at gives the latest request whose origin is a place on a bridge, once
// there is one.
func (s *serving) at(bridge, place string) int64 {
	s.b.t.Helper()
	var n int64
	s.b.until(fmt.Sprintf("a request is asked on %s at %s", bridge, place), func() bool {
		got, ok, err := s.areasOf().Requests.LatestAt(bridge, place)
		if err != nil {
			s.b.t.Fatal(err)
		}
		n = got
		return ok
	})
	return n
}

// subOf stays until a request has a sub-request to a team, and gives it.
func (s *serving) subOf(n int64, team string) int64 {
	s.b.t.Helper()
	var sub int64
	s.b.until(fmt.Sprintf("request %d asks %s", n, team), func() bool {
		tree, err := s.areasOf().Requests.Tree(n)
		if err != nil {
			s.b.t.Fatal(err)
		}
		for _, t := range tree {
			if t.Team == team {
				sub = t.Number
			}
		}
		return sub != 0
	})
	return sub
}

// waitsOn stays until something in a request waits on a person, and gives
// it.
func (s *serving) waitsOn(n int64, person string) requests.Wait {
	s.b.t.Helper()
	var w requests.Wait
	s.b.until(fmt.Sprintf("request %d waits on %s", n, person), func() bool {
		waits, err := s.areasOf().Requests.WaitingOn(person)
		if err != nil {
			s.b.t.Fatal(err)
		}
		for _, wt := range waits {
			if wt.Request == n && wt.Entry != 0 {
				w = wt
				return true
			}
		}
		return false
	})
	return w
}

// texts gives the texts of what a request's view holds.
func texts(view map[string]any) string {
	var b strings.Builder
	for _, e := range view["entries"].([]any) {
		b.WriteString(e.(map[string]any)["text"].(string) + "\n")
	}
	return b.String()
}
