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
)

// StreamQuiet is how long a stream goes with nothing sent before it is sent a
// comment, so that nothing between it and the browser takes it for gone.
const StreamQuiet = 25 * time.Second

// Behind is how much a screen may be behind before what it has not taken
// is let go, and it is sent hello.
const Behind = 64

// A screen is one stream of changes, with what it is still to be sent. A
// change is put to it and never waits on it: one that has fallen Behind
// lets go of what it has not taken, and is sent hello next.
type screen struct {
	mu      sync.Mutex
	pending []string
	wake    chan struct{}
}

func newScreen() *screen { return &screen{wake: make(chan struct{}, 1)} }

// put has the screen sent what changed, once, after what it is sent
// already.
func (sc *screen) put(what string) {
	sc.mu.Lock()
	switch {
	case len(sc.pending) >= Behind:
		sc.pending = []string{Hello}
	case len(sc.pending) == 0 || sc.pending[len(sc.pending)-1] != what:
		sc.pending = append(sc.pending, what)
	}
	sc.mu.Unlock()
	select {
	case sc.wake <- struct{}{}:
	default:
	}
}

// take gives what the screen is to be sent now.
func (sc *screen) take() []string {
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

// office tells every screen the office was read again.
func (ss *screens) office() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for sc := range ss.all {
		sc.put(Office)
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
// them, or the office read again, as it is judged when it changes; and a
// comment when nothing has been sent for StreamQuiet.
func (s *site) events(person string, w http.ResponseWriter, r *http.Request) {
	sc := newScreen()
	sc.put(Hello)
	unwatch := s.Requests.WatchAs(person, func(n int64, err error) {
		if err != nil {
			s.say("what %s may see of a change could not be judged, so their screen is sent hello: %v", person, err)
			sc.put(Hello)
			return
		}
		sc.put(fmt.Sprintf("request:%d", n))
	})
	defer unwatch()
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
			for _, what := range sc.take() {
				if what == Hello {
					fmt.Fprintf(w, "event: %s\ndata: \n\n", Hello)
				} else {
					fmt.Fprintf(w, "event: change\ndata: %s\n\n", what)
				}
				now, err := s.Requests.WaitingOn(person)
				if err != nil {
					s.say("what waits on %s could not be read for their screen: %v", person, err)
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
