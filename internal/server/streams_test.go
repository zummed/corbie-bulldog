package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// changed gives the change event that names a request.
func changed(n int64) func(event) bool {
	return func(e event) bool { return e.name == "change" && e.data == fmt.Sprintf("request:%d", n) }
}

func TestServerEventsStream(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	kit, sam := s.as("kit").stream("/api/events"), s.as("sam").stream("/api/events")
	for _, st := range []*stream{kit, sam} {
		if e := st.next(); e.name != Hello {
			t.Fatalf("a stream begins %+v", e)
		}
	}
	n := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "is the kettle on?"}))
	kit.until("Kit's request is named", changed(n))
	// A run at work is told to every stream.
	for _, st := range []*stream{kit, sam} {
		st.until("the runs at work", func(e event) bool { return e.name == "change" && e.data == Runs })
	}
	s.answered(n)
	// The office read again is told to every stream.
	b.write(map[string]string{"notes/rooms.md": "# rooms\n\nThe kitchen has a new socket.\n"})
	b.clock.Move(0)
	for _, st := range []*stream{kit, sam} {
		st.until("the office read again", func(e event) bool { return e.name == "change" && e.data == Office })
	}
	// A comment once 25 seconds have gone with nothing to send.
	b.clock.Move(StreamQuiet)
	kit.until("a comment", func(e event) bool { return e.comment })
}

func TestServerEventsTellARunAsItStartsAndEnds(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, kit]\nruns: 1", 1)
	b := newBench(t, files)
	// The steward's turn on each request is held until the test lets it
	// go.
	gates := map[string]chan struct{}{"kettle": make(chan struct{}), "door": make(chan struct{})}
	var once sync.Map
	let := func(word string) {
		if _, done := once.LoadOrStore(word, true); !done {
			close(gates[word])
		}
	}
	t.Cleanup(func() { let("kettle"); let("door") })
	var mu sync.Mutex
	held := map[string]bool{}
	b.says(func(turn testkit.Turn) testkit.Reply {
		for word, gate := range gates {
			if strings.Contains(turn.Prompt(), word) {
				mu.Lock()
				held[word] = true
				mu.Unlock()
				b.poke()
				<-gate
			}
		}
		return testkit.Say("done")
	})
	s := b.serve()
	first := s.chat("kit", "", "is the kettle on?")
	b.until("the first is held", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return held["kettle"]
	})
	kit := s.as("kit").stream("/api/events")
	kit.next()
	second := s.chat("kit", "", "is the door locked?")
	kit.until("the second is asked", changed(second))
	for !slices.Contains(s.areasOf().Work.Queue(), second) {
		kit.until("the runs are counted again", func(e event) bool { return e.data == Runs })
	}
	// Nothing is written to the second as its run starts, and it cannot
	// end while its turn is held: its start alone names it.
	let("kettle")
	s.answered(first)
	kit.until("the second's run starts", changed(second))
	let("door")
	kit.until("the second's run ends", changed(second))
	s.answered(second)
}

func TestServerEventsFilteredBySight(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	dana, al := d.s.as("dana").stream("/api/events"), d.s.as("al").stream("/api/events")
	dana.next()
	al.next()
	raj := numbered(d.s.as("raj").post("/api/requests", map[string]string{"team": "platform", "text": "is the pipeline green?"}))
	al.until("Raj's request to platform is named to Al", changed(raj))
	own := numbered(d.s.as("dana").post("/api/requests", map[string]string{"team": "support", "text": "a note to myself"}))
	dana.until("Dana's own request", func(e event) bool {
		if changed(raj)(e) {
			t.Errorf("Dana's stream names Raj's request to platform")
		}
		return changed(own)(e)
	})
	// What waits on someone is named to them.
	lee := d.s.as("lee").stream("/api/events")
	lee.next()
	d.s.as("lee").post("/api/requests/"+itoa(d.payments)+"/decisions/"+itoa(d.refund), map[string]string{"says": "yes"}).is(http.StatusOK)
	lee.until("what waits on Lee changed", func(e event) bool { return e.name == "change" && e.data == Waiting })
}

func TestServerEventsBeginWithHello(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	for range 2 {
		st := s.as("kit").stream("/api/events")
		if e := st.next(); e.name != Hello {
			t.Errorf("a stream begins %+v", e)
		}
		st.stop()
	}
}

// A stalled is a browser's end of a stream that takes the first write and
// then takes nothing until it is let go, as a slow browser behind a full
// connection does.
type stalled struct {
	// wrote is told of each write once it is taken.
	wrote   func()
	header  http.Header
	gate    chan struct{}
	waiting chan struct{}
	once    sync.Once

	mu     sync.Mutex
	writes int
	out    strings.Builder
}

func newStalled(wrote func()) *stalled {
	return &stalled{wrote: wrote, header: http.Header{}, gate: make(chan struct{}), waiting: make(chan struct{})}
}

func (w *stalled) Header() http.Header { return w.header }
func (w *stalled) WriteHeader(int)     {}
func (w *stalled) Flush()              {}

func (w *stalled) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.writes++
	first := w.writes == 1
	w.mu.Unlock()
	if !first {
		w.once.Do(func() { close(w.waiting) })
		<-w.gate
	}
	w.mu.Lock()
	defer w.wrote()
	defer w.mu.Unlock()
	return w.out.Write(p)
}

func (w *stalled) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.out.String()
}

func TestServerSlowScreenNeverBlocks(t *testing.T) {
	t.Parallel()
	sc := newScreen()
	for i := range 10 * Behind {
		sc.put(mark{request: int64(i + 1)})
	}
	got := sc.take()
	if len(got) == 0 || got[0] != (mark{what: Hello}) || len(got) > Behind {
		t.Errorf("a screen that fell behind is sent %v", got)
	}
	// What is put while the screen is judging what it took wakes it
	// again, so nothing put is left unsent.
	<-sc.wake
	sc.put(mark{request: 1})
	taken := sc.take()
	sc.put(mark{request: 2})
	select {
	case <-sc.wake:
	default:
		t.Error("a change put while the screen judged what it took does not wake it")
	}
	if taken = append(taken, sc.take()...); len(taken) != 2 {
		t.Errorf("the screen takes %v", taken)
	}

	// A screen whose browser takes nothing keeps nobody who changes
	// anything waiting. Once it takes again, what it was marked for and
	// may not see is sent as nothing while it is less than Behind, and
	// past Behind the screen is sent hello.
	b := newBench(t, house(t))
	held, let := b.hold()
	defer let()
	s := b.serve()
	a := s.areasOf()
	st := &site{Areas: *a, clock: b.clock, say: func(string, ...any) {}, screens: &screens{}}
	hedge, roses := s.chat("sam", "garden", "the hedge"), s.chat("sam", "garden", "the roses")
	kits := s.chat("kit", "", "the first question")
	b.until("the steward has a turn", func() bool { return held() > 0 })
	kitSays := func(text string) {
		if _, err := a.Requests.Say(kits, a.Requests.Person("kit"), text, nil, requests.Screens, false); err != nil {
			t.Fatal(err)
		}
	}
	// stall gives Kit's screen stalled in sending her a change to her own
	// request, and what lets it take again.
	stall := func() (*stalled, func()) {
		w := newStalled(b.poke)
		watching, stop := context.WithCancel(ctx)
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			st.events("kit", w, httptest.NewRequestWithContext(watching, http.MethodGet, "/api/events", nil))
		}()
		b.until("the screen is sent hello", func() bool { return strings.Contains(w.String(), "event: hello") })
		kitSays("is it on yet?")
		<-w.waiting
		t.Cleanup(func() { stop(); <-ended })
		return w, func() { close(w.gate) }
	}
	// samSays has Sam say more in his requests to the garden, which Kit
	// does not see, one and then the other, as many times as is given.
	samSays := func(times int) {
		for i := range times {
			n := []int64{hedge, roses}[i%2]
			if _, err := a.Requests.Say(n, a.Requests.Person("sam"), fmt.Sprintf("and %d", i), nil, requests.Screens, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	names := func(out string, ns ...int64) bool {
		return slices.ContainsFunc(ns, func(n int64) bool { return strings.Contains(out, fmt.Sprintf("data: request:%d\n", n)) })
	}
	w, take := stall()
	samSays(Behind / 2)
	take()
	kitSays("and now?")
	b.until("Kit's own change is sent after Sam's", func() bool { return strings.Count(w.String(), fmt.Sprintf("data: request:%d\n", kits)) == 2 })
	if strings.Count(w.String(), "event: hello") != 1 || names(w.String(), hedge, roses) {
		t.Errorf("stalled while Sam said more %d times, Kit's screen is sent:%s", Behind/2, w.String())
	}
	w, take = stall()
	samSays(Behind + 1)
	take()
	b.until("the screen is sent hello once it takes again", func() bool { return strings.Count(w.String(), "event: hello") == 2 })
	if names(w.String(), hedge, roses) {
		t.Errorf("once it fell behind, the screen is sent:%s", w.String())
	}
}

func TestServerStreamsNotBuffered(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	for _, st := range []*stream{d.s.as("lee").stream("/api/events"), d.s.as("lee").stream(watching(d.s.runOf("lee", d.payments, "ledger")))} {
		h := st.res.Header
		if st.res.StatusCode != http.StatusOK || h.Get("Cache-Control") != "no-store" || h.Get("X-Accel-Buffering") != "no" || h.Get("Content-Type") != "text/event-stream" {
			t.Errorf("a stream is answered %d with %v", st.res.StatusCode, h)
		}
	}
}

// seen reads a run's stream to its end.
func (st *stream) seen() []event {
	st.t.Helper()
	var all []event
	for {
		e := st.next()
		all = append(all, e)
		if e.name == work.SeenEnded {
			return all
		}
	}
}

func TestServerRunWatch(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	ledger := d.s.runOf("lee", d.payments, "ledger")
	st := d.s.as("lee").stream(watching(ledger))
	// While the refund waits, Lee sees it asked for, with its input.
	st.until("the refund held", func(e event) bool { return e.name == work.SeenCall && strings.Contains(e.data, `\"refund\"`) })
	d.decided()
	var words strings.Builder
	kinds := map[string]bool{}
	for _, e := range st.seen() {
		kinds[e.name] = true
		if e.name == work.SeenWords {
			var seen work.Seen
			if err := json.Unmarshal([]byte(e.data), &seen); err != nil {
				t.Fatal(err)
			}
			words.WriteString(seen.Text)
		}
		if e.name == work.SeenCall && strings.Contains(e.data, `"name":"refund"`) && !strings.Contains(e.data, "ch_2") {
			t.Errorf("the refund is called without its input: %s", e.data)
		}
	}
	if !kinds[work.SeenWords] || !kinds[work.SeenCall] || !kinds[work.SeenResult] || !kinds[work.SeenEnded] {
		t.Errorf("Lee sees the run's events of the kinds %v", kinds)
	}
	if !strings.Contains(words.String(), "Duplicate confirmed") {
		t.Errorf("Lee sees the ledger's words as %s", words.String())
	}
}

func TestServerRunWatchShowsLogsAndRefusals(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["steward.yaml"] += "tools:\n" +
		"  meter: { does: Read the meter, args: {}, job: { script: ./tools/meter.js } }\n" +
		"  peek: { does: Look at what it may not, args: {}, job: { script: ./tools/peek.js } }\n"
	files["tools/meter.js"] = "log(\"the meter reads 42\")\nreturn 42\n"
	files["tools/peek.js"] = "return env.get(\"HOME\")\n"
	b := newBench(t, files)
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch turn.Number {
		case 1:
			return testkit.Use("meter", map[string]any{})
		case 2:
			return testkit.Use("peek", map[string]any{})
		}
		return testkit.Say("The meter reads 42.")
	})
	s := b.serve()
	n := s.chat("kit", "", "what does the meter read?")
	s.answered(n)
	var logged, refused []map[string]any
	for _, e := range s.as("kit").stream(watching(s.runOf("kit", n, "steward"))).seen() {
		var seen map[string]any
		if err := json.Unmarshal([]byte(e.data), &seen); err != nil {
			t.Fatalf("an event of the run is %q: %v", e.data, err)
		}
		switch e.name {
		case work.SeenLog:
			logged = append(logged, seen)
		case work.SeenRefused:
			refused = append(refused, seen)
		}
	}
	if len(logged) != 1 || logged[0]["name"] != "meter" || logged[0]["text"] != "the meter reads 42" {
		t.Errorf("the meter's log is watched as %v", logged)
	}
	if len(refused) != 1 || refused[0]["name"] != "peek" || refused[0]["refused"] != "env:HOME" || refused[0]["layer"] == nil || refused[0]["fix"] == nil {
		t.Errorf("the refusal is watched as %v", refused)
	}
}

func TestServerRunWatchResumes(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	d.decided()
	ledger := d.s.runOf("lee", d.payments, "ledger")
	whole := d.s.as("lee").stream(watching(ledger)).seen()
	for i, e := range whole {
		if e.id != fmt.Sprint(i+1) {
			t.Fatalf("the event %d of the run is at %q", i, e.id)
		}
	}
	lee := d.s.as("lee")
	lee.header.Set("Last-Event-ID", "3")
	again := lee.stream(watching(ledger)).seen()
	if len(again) != len(whole)-3 {
		t.Fatalf("connected again after 3, it is sent %d of %d", len(again), len(whole))
	}
	for i, e := range again {
		if e != whole[i+3] {
			t.Errorf("connected again, it is sent %+v in place of %+v", e, whole[i+3])
		}
	}
}

func TestServerWatchOnlyThisOfficesRuns(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	contents, _ := shipped.File(shipped.Live)
	draft, err := machine.Parse(contents, nil)
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{work.LabelBulldog: "another office", work.LabelKind: work.KindWork, "team": "payments", "request": itoa(d.payments)}
	other, err := d.b.corbie.Start(ctx, draft, client.Start{Labels: labels})
	if err != nil {
		t.Fatal(err)
	}
	watched(t, d.s.as("lee"), other, http.StatusNotFound)
	// Nor is the server's own run.
	live := d.b.listed(d.b.identity(), map[string]string{work.LabelKind: work.KindLive})
	watched(t, d.s.as("al"), live[0].ID, http.StatusNotFound)
}
