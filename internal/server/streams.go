package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/zummed/corbie-bulldog/internal/work"
)

// What the stream of changes names, beside a request by its number.
const (
	Hello   = "hello"
	Waiting = "waiting"
	Office  = "office"
	Runs    = "runs"
)

// StreamQuiet is how long a stream goes with nothing sent before it is sent a
// comment, so that nothing between it and the browser takes it for gone.
const StreamQuiet = 25 * time.Second

// Behind is how much a screen may be behind before what it has not taken
// is let go, and it is sent hello. It counts what was marked for the
// screen, whether the one looking sees it or not, so it is far more than
// an office writes while one screen judges what it took: about a second
// of writes at the store's pace, a few tens of kilobytes a screen.
const Behind = 1024

// A mark is one thing that changed, put to a screen as it is written and
// judged for the one looking only as the screen is to be sent it: a
// request by its number, a decision by its id, or what everyone is sent.
type mark struct {
	request  int64
	decision int64
	what     string
}

// A screen is one stream of changes, with what it is still to be sent. A
// change is put to it and never waits on it: one that has fallen Behind
// lets go of what it has not taken, and is sent hello next.
type screen struct {
	mu      sync.Mutex
	pending []mark
	wake    chan struct{}
}

func newScreen() *screen { return &screen{wake: make(chan struct{}, 1)} }

// put has the screen sent what changed, once, after what it is sent
// already.
func (sc *screen) put(m mark) {
	sc.mu.Lock()
	switch {
	case len(sc.pending) >= Behind:
		sc.pending = []mark{{what: Hello}}
	case len(sc.pending) == 0 || sc.pending[len(sc.pending)-1] != m:
		sc.pending = append(sc.pending, m)
	}
	sc.mu.Unlock()
	select {
	case sc.wake <- struct{}{}:
	default:
	}
}

// take gives what the screen is to be sent now.
func (sc *screen) take() []mark {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	out := sc.pending
	sc.pending = nil
	return out
}

// screens are the streams of changes open now.
type screens struct {
	mu  sync.Mutex
	all map[*screen]bool
}

func (ss *screens) add(sc *screen) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.all == nil {
		ss.all = map[*screen]bool{}
	}
	ss.all[sc] = true
}

func (ss *screens) remove(sc *screen) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.all, sc)
}

// tell tells every screen what changed that everyone sees: the office
// read again, or how many runs are at work or wait their turn.
func (ss *screens) tell(what string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for sc := range ss.all {
		sc.put(mark{what: what})
	}
}

// streaming begins a stream of server-sent events, which nothing in front
// of the server is to keep or hold back.
func streaming(w http.ResponseWriter) http.Flusher {
	f, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if f != nil {
		f.Flush()
	}
	return f
}

// events is the stream of changes a person may see: hello when it
// connects, then each change naming a request they may see, what waits on
// them, the office read again, or how many runs are at work or wait their
// turn, as it is judged when it is sent; and a comment when nothing has
// been sent for StreamQuiet. Whoever writes a change only marks it for
// the screen: what the person may see of it is judged here, at once for
// all that was marked since the screen was last sent anything.
func (s *site) events(person string, w http.ResponseWriter, r *http.Request) {
	sc := newScreen()
	sc.put(mark{what: Hello})
	unwatch := s.Requests.Watch(func(n int64) { sc.put(mark{request: n}) })
	defer unwatch()
	undecided := s.Decisions.Watch(func(id int64) { sc.put(mark{decision: id}) })
	defer undecided()
	s.screens.add(sc)
	defer s.screens.remove(sc)
	moved, unclock := s.clock.Watch()
	defer unclock()
	f := streaming(w)
	sent := s.clock.Now()
	waits := -1
	for {
		select {
		case <-r.Context().Done():
			return
		case <-moved:
			if s.clock.Now().Sub(sent) < StreamQuiet {
				continue
			}
			fmt.Fprint(w, ": nothing has changed\n\n")
		case <-sc.wake:
			sends := s.judged(person, sc.take())
			if len(sends) == 0 {
				continue
			}
			now, err := s.Requests.WaitingOn(person)
			if err != nil {
				s.say("what waits on %s could not be read for their screen: %v", person, err)
			}
			for _, what := range sends {
				if what == Hello {
					fmt.Fprintf(w, "event: %s\ndata: \n\n", Hello)
				} else {
					fmt.Fprintf(w, "event: change\ndata: %s\n\n", what)
				}
				if err != nil {
					continue
				}
				if waits >= 0 && len(now) != waits && what != Hello {
					fmt.Fprintf(w, "event: change\ndata: %s\n\n", Waiting)
				}
				waits = len(now)
			}
		}
		sent = s.clock.Now()
		if f != nil {
			f.Flush()
		}
	}
}

// judged gives what a person's screen is sent of what was marked for it,
// as they may see it now: each request they see any of, what waits on
// them for a person's own change they see, and what everyone is sent,
// never the same twice in a row. Each request and each decision is judged
// once, however often it was marked.
func (s *site) judged(person string, marks []mark) []string {
	ofRequest, ofDecision := map[int64]string{}, map[int64]string{}
	var out []string
	for _, m := range marks {
		what := m.what
		switch {
		case m.request != 0:
			if _, ok := ofRequest[m.request]; !ok {
				ofRequest[m.request] = s.seesChange(person, m.request)
			}
			what = ofRequest[m.request]
		case m.decision != 0:
			if _, ok := ofDecision[m.decision]; !ok {
				ofDecision[m.decision] = s.seesOwnChange(person, m.decision)
			}
			what = ofDecision[m.decision]
		}
		if what != "" && (len(out) == 0 || out[len(out)-1] != what) {
			out = append(out, what)
		}
	}
	return out
}

// seesChange names a change to a request to a person who sees any of it.
// When the store fails in judging whether they do, their screen is sent
// hello.
func (s *site) seesChange(person string, n int64) string {
	sees, err := s.Requests.SeesAny(n, person)
	switch {
	case err != nil:
		s.say("what %s may see of a change could not be judged, so their screen is sent hello: %v", person, err)
		return Hello
	case sees:
		return fmt.Sprintf("request:%d", n)
	}
	return ""
}

// seesOwnChange names what waits on a person when a decision that changed
// is of a person's own change they see: it serves no request, so nothing
// else names it.
func (s *site) seesOwnChange(person string, id int64) string {
	d, err := s.Decisions.Get(id)
	o := s.Keeper.Office()
	if err != nil || o == nil || d.Request != 0 || d.Put == nil {
		return ""
	}
	if _, _, sees, err := s.seesOwn(o, d, person); err == nil && sees {
		return Waiting
	}
	return ""
}

// watch is the stream of what one run of the office does, as Corbie
// records it, from its beginning, each event with its place in the run:
// a screen that connects again with the last place it had is sent only
// what comes after it. Only the people its labels name watch it; nobody
// watches a bridge's run.
func (s *site) watch(person string, w http.ResponseWriter, r *http.Request) {
	run := r.PathValue("run")
	after := 0
	if last := r.Header.Get("Last-Event-ID"); last != "" {
		n, err := strconv.Atoi(last)
		if err != nil || n < 0 {
			s.refuse(w, r, unreadable("Last-Event-ID is to be the place of an event of the run, and is %q.", last))
			return
		}
		after = n
	}
	may, err := s.Work.MayWatch(run, person)
	if err != nil {
		s.refuse(w, r, err)
		return
	}
	if !may {
		s.refuse(w, r, work.ErrNoWatch)
		return
	}
	f := streaming(w)
	place := 0
	send := func(kind string, v any) {
		place++
		if place <= after {
			return
		}
		data, err := json.Marshal(v)
		if err != nil {
			data, _ = json.Marshal(map[string]string{"text": err.Error()})
		}
		fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", place, kind, data)
		if f != nil {
			f.Flush()
		}
	}
	err = s.Work.Follow(r.Context(), run, person, func(seen work.Seen) { send(seen.Kind, seenOf(seen)) })
	switch {
	case r.Context().Err() != nil:
	case errors.Is(err, work.ErrNoWatch):
		send(work.SeenEnded, seenOut{Kind: work.SeenEnded, Run: run, Text: "Corbie keeps this run's record no longer."})
	case err != nil:
		s.say("the run %s could not be followed for %s: %v", run, person, err)
		send(work.SeenFailed, seenOut{Kind: work.SeenFailed, Run: run, Text: "Bulldog failed in following the run, and the failure is in the server's log: " + err.Error()})
	}
}

// run gives a run to whoever may watch it, as it is kept: its kind, its
// team and request, its worker and how it stands; and, of a lead's run,
// what it was given.
func (s *site) run(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	name := r.PathValue("run")
	kept, err := s.Work.Watched(name, person)
	if err != nil {
		return nil, err
	}
	out := runOf(kept)
	given, err := s.Work.Given(r.Context(), name, person)
	if err != nil {
		return nil, err
	}
	if given == nil {
		return out, nil
	}
	ds, err := s.Decisions.OfRequest(kept.Request)
	if err != nil {
		return nil, err
	}
	out.Given = givenOf(given, s.Requests.Party, ds)
	return out, nil
}

// A seenOut is one thing a run did, as a screen watching it is sent it:
// the instance it happened in and its machine, the tool, stage or job it
// is of, its words, and for a refusal what was tried, the layer that
// refused it and the line that would fix it.
type seenOut struct {
	At      *time.Time `json:"at,omitempty"`
	Run     string     `json:"run"`
	Machine string     `json:"machine,omitempty"`
	Kind    string     `json:"kind"`
	Name    string     `json:"name,omitempty"`
	Text    string     `json:"text,omitempty"`
	Refused string     `json:"refused,omitempty"`
	Layer   string     `json:"layer,omitempty"`
	Fix     string     `json:"fix,omitempty"`
}

func seenOf(s work.Seen) seenOut {
	out := seenOut{Run: s.Run, Machine: s.Machine, Kind: s.Kind, Name: s.Name, Text: s.Text, Refused: s.Refused, Layer: s.Layer, Fix: s.Fix}
	if !s.At.IsZero() {
		out.At = &s.At
	}
	return out
}
