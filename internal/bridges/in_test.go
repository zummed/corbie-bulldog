package bridges_test

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

func TestBridgesOneListeningRunAtATime(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	for i := 1; i <= 5; i++ {
		s := d.listening("telegram")
		if again := d.listen(); len(again) != 0 {
			t.Fatalf("a second listening run is started while one is going: %v", again)
		}
		if cursor, _ := s.Input["cursor"].(float64); int(cursor) != i-1 {
			t.Errorf("run %d is given the cursor %v", i, s.Input["cursor"])
		}
		_, err := d.InEnded("telegram", s.Labels["run"], bridges.End{Output: heard(float64(i), said("4471", "direct", "4471", "thing"))})
		must(t, err)
	}
	if got := d.thread(1); len(got) != 5 {
		t.Errorf("Sam's request holds %v", got)
	}
	if s := d.listening("telegram"); s.Input["cursor"] != float64(5) {
		t.Errorf("the cursor held is %v", s.Input["cursor"])
	}
}

func TestBridgesFirstListeningRunHasNoCursor(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	s := d.listening("telegram")
	if cursor, given := s.Input["cursor"]; !given || cursor != nil {
		t.Errorf("the first run is given the cursor %#v", s.Input)
	}
}

func TestBridgesListeningPausesAfterFailure(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	var pauses []time.Duration
	for i := 0; i < 9; i++ {
		s := d.listening("telegram")
		_, err := d.InEnded("telegram", s.Labels["run"], bridges.End{Failed: "the job failed: getUpdates: 502"})
		must(t, err)
		if i == 0 {
			troubles, err := d.Troubles()
			must(t, err)
			if len(troubles) != 1 || troubles[0].Bridge != "telegram" || !strings.Contains(troubles[0].Error, "502") {
				t.Errorf("after one failure the troubles are %v", troubles)
			}
		}
		_, next, err := d.Listen()
		must(t, err)
		if again := d.listen(); len(again) != 0 {
			t.Fatalf("a listening run is started again at once")
		}
		pauses = append(pauses, next.Sub(d.clock.Now()))
		d.clock.Set(next)
	}
	for i, p := range pauses {
		if p > time.Minute || i > 0 && p < pauses[i-1] || i > 0 && p == pauses[i-1] && p != time.Minute {
			t.Errorf("the pauses are %v", pauses)
			break
		}
	}
	if pauses[len(pauses)-1] != time.Minute {
		t.Errorf("the pauses are %v", pauses)
	}
	d.hear("telegram")
	troubles, err := d.Troubles()
	must(t, err)
	if len(troubles) != 0 {
		t.Errorf("after a run ends well the troubles are %v", troubles)
	}
}

func TestBridgesCursorKeptAfterTaking(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	picture := said("4471", "direct", "4471", "look", "files", []any{map[string]any{"name": "socket.jpg", "data": map[string]any{"$blob": "output/heard/0/files/0/data"}}})
	gone := func(string) ([]byte, error) { return nil, errors.New("the run's record is gone") }
	s := d.listening("telegram")
	_, err := d.InEnded("telegram", s.Labels["run"], bridges.End{Output: heard(7, picture), Fetch: gone})
	must(t, err)
	if d.latest() != 0 {
		t.Error("a thing that could not be taken made a request")
	}
	d.later(time.Minute)
	s = d.listening("telegram")
	if s.Input["cursor"] != nil {
		t.Errorf("the cursor was kept before what was heard with it was taken: %v", s.Input["cursor"])
	}
	fetch := func(string) ([]byte, error) { return []byte("jpeg"), nil }
	_, err = d.InEnded("telegram", s.Labels["run"], bridges.End{Output: heard(7, picture), Fetch: fetch})
	must(t, err)
	if got := d.thread(1); len(got) != 1 {
		t.Errorf("the thing is taken as %v", got)
	}
	if s := d.listening("telegram"); s.Input["cursor"] != float64(7) {
		t.Errorf("the cursor kept is %v", s.Input["cursor"])
	}
}

func TestBridgesWhatIsHeard(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "first")
	d.answer(n, "bot", "answered")
	_, err := d.requests.Put(n, d.requests.Worker("bot"), requests.KindQuestion, d.requests.Person("dana"), "which customer?", nil)
	must(t, err)
	decision := d.decide(n, "refund?", "al")
	for _, s := range d.carry() {
		d.ended(s, map[string]any{"sent": "s-" + s.Input["to"].(string)})
	}
	choice := map[string]any{"id": "c1", "kind": "choice", "from": "U0AL0000", "heard_in": "direct", "place": "U0AL0000", "answers": ref(t, d, "slack", "U0AL0000"), "choice": "yes",
		"readers": []any{}, "about": map[string]any{"link": "https://slack.example/1"}}
	data := base64.StdEncoding.EncodeToString([]byte("PNG"))
	words := said("U0DANA00", "#help", "C1/1.0", "4411", "readers", []any{"U0DANA00", "U0AL0000"}, "about", map[string]any{"link": "https://slack.example/2"},
		"files", []any{map[string]any{"name": "a.png", "data": data}})
	d.delivered("slack", words, choice)
	es, err := d.requests.Thread(n)
	must(t, err)
	last := es[len(es)-1]
	if last.Text != "4411" || len(last.Files) != 1 || last.Files[0].Name != "a.png" {
		t.Errorf("what Dana said is %#v", last)
	}
	if file, err := d.requests.File(n, last.ID, "a.png", "dana"); err != nil || string(file) != "PNG" {
		t.Errorf("the file is %q, %v", file, err)
	}
	if readers, _ := d.requests.ReadBy(n); len(readers) != 2 {
		t.Errorf("the readers are %v", readers)
	}
	if len(d.chosen) != 1 || d.chosen[0].Choice != "yes" || d.chosen[0].Person != "al" || d.chosen[0].Entry != decision.ID {
		t.Errorf("the choices given are %#v", d.chosen)
	}
	// A status of a thing at its source.
	g := open(t, work(t))
	g.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	g.hear("gitlab", map[string]any{"id": "s1", "kind": "status", "from": "1187", "heard_in": "platform/api", "place": "platform/api#7",
		"status": "closed", "closed": true, "about": map[string]any{"link": "https://gitlab.example.com/platform/api/-/issues/7"}, "readers": []any{"1187", "1041"}})
	if r, _ := g.requests.Get(1); r.Source == nil || r.Source.Status != "closed" || !r.Source.Closed || r.Source.Link != "https://gitlab.example.com/platform/api/-/issues/7" {
		t.Errorf("the request's source is %#v", r.Source)
	}
	if readers, _ := g.requests.ReadBy(1); len(readers) != 2 {
		t.Errorf("the readers a status gives are kept as %v", readers)
	}
	// A thing with no id, no kind or no from is not taken, and the
	// trouble names what it lacks.
	for _, field := range []string{"id", "kind", "from"} {
		h := open(t, house(t))
		thing := said("4471", "direct", "4471", "no "+field)
		delete(thing, field)
		h.hear("telegram", thing)
		troubles, err := h.Troubles()
		must(t, err)
		if h.latest() != 0 || len(troubles) != 1 || !strings.Contains(troubles[0].Error, "no "+field) {
			t.Errorf("a thing with no %s: troubles %v", field, troubles)
		}
	}
}

// ref gives the reference Bulldog handed the out machine with the latest
// thing carried to a place.
func ref(t *testing.T, d *desk, bridge, place string) string {
	t.Helper()
	for n := d.latest(); n > 0; n-- {
		cs, err := d.Carried(n)
		must(t, err)
		for _, c := range slices.Backward(cs) {
			if c.Bridge == bridge && c.Place == place {
				return "r" + itoa(c.Thing)
			}
		}
	}
	t.Fatalf("nothing was carried to %s on %s", place, bridge)
	return ""
}

func itoa(n int64) string {
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestBridgesTakenOnce(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	m7 := said("4471", "direct", "4471", "once")
	m7["id"] = "m7"
	d.hear("telegram", m7)
	d.hear("telegram", m7)
	_, err := d.InEnded("telegram", "a delivery's run", bridges.End{Output: heard(0, m7)})
	must(t, err)
	if got := d.thread(1); len(got) != 1 {
		t.Errorf("m7 is taken as %v", got)
	}
}

func TestBridgesSaidBecomesAnEntry(t *testing.T) {
	t.Parallel()
	// Sam answers what was sent him for request one, in his direct chat,
	// where his latest request is another.
	h := open(t, house(t))
	h.hear("telegram", said("4472", "direct", "4472", "the socket sparked"))
	_, err := h.requests.Put(1, h.requests.Worker("steward"), requests.KindQuestion, h.requests.Person("sam"), "may I cut the circuit?", nil)
	must(t, err)
	for _, s := range h.carry() {
		h.ended(s, map[string]any{"sent": "4471:9"})
	}
	h.hear("telegram", said("4471", "direct", "4471", "the bins"))
	h.hear("telegram", said("4471", "direct", "4471", "yes", "answers", "4471:9"))
	if got := h.thread(1); got[len(got)-1] != "replied: yes" {
		t.Errorf("request one holds %v", got)
	}
	if got := h.thread(2); len(got) != 1 {
		t.Errorf("request two holds %v", got)
	}
	d := open(t, work(t))
	five := d.ask("C1/5.0", "request five")
	d.delivered("slack", said("U0DANA00", "#help", "C1/5.0", "more on five"))
	if got := d.thread(five); got[len(got)-1] != "said: more on five" {
		t.Errorf("request five holds %v", got)
	}
	d.answer(five, "bot", "done")
	d.delivered("slack", said("U0DANA00", "#help", "C1/5.0", "and again"))
	if got := d.thread(five); got[len(got)-1] != "said: and again" {
		t.Errorf("request five holds %v", got)
	}
	if st, _ := d.requests.State(five); st.Is != requests.Working {
		t.Errorf("request five is %s", st.Is)
	}
	six := d.ask("C1/6.0", "a new thread")
	r, _ := d.requests.Get(six)
	if r.Team != office.Top || r.Origin.Place != "C1/6.0" || r.Origin.Heard != "#help" || r.Asker != requests.Identity("slack", "U0DANA00") {
		t.Errorf("request six is %#v", r)
	}
	if lead, _, _ := d.requests.Lead(six); lead != "bot" {
		t.Errorf("request six goes to %s", lead)
	}
	d.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	seven, _ := d.requests.Get(d.latest())
	if seven.Team != "platform" || seven.Origin.Place != "platform/api#7" {
		t.Errorf("Raj's issue is %#v", seven)
	}
	if lead, _, _ := d.requests.Lead(seven.Number); lead != "ops" {
		t.Errorf("Raj's issue goes to %s", lead)
	}
	// Sam's chat with the bot holds no threads: a reply goes on from what
	// it answers, and anything else begins a request of its own, but for
	// the answer to a question that waits on him.
	g := open(t, house(t))
	chat := func(text string, more ...any) map[string]any {
		return said("4471", "direct", "4471", text, append([]any{"threadless", true}, more...)...)
	}
	sparked := chat("the socket sparked")
	g.hear("telegram", sparked)
	g.answer(1, "steward", "The kitchen circuit is off.")
	for _, s := range g.carry() {
		g.ended(s, map[string]any{"sent": []any{"4471:20", "4471:21"}})
	}
	g.hear("telegram", chat("the bins"))
	g.hear("telegram", chat("and the lights?", "answers", "4471:20"))
	g.hear("telegram", chat("it was the kettle", "answers", sparked["id"]))
	if got := g.thread(1); !slices.Equal(got[len(got)-2:], []string{"said: and the lights?", "said: it was the kettle"}) {
		t.Errorf("request one holds %v", got)
	}
	if got := g.thread(2); !slices.Equal(got, []string{"asked: the bins"}) {
		t.Errorf("request two holds %v", got)
	}
	_, err = g.requests.Put(1, g.requests.Worker("steward"), requests.KindQuestion, g.requests.Person("sam"), "may I cut the lights too?", nil)
	must(t, err)
	for _, s := range g.carry() {
		g.ended(s, map[string]any{"sent": "4471:30"})
	}
	g.hear("telegram", chat("yes"))
	g.hear("telegram", chat("the post"))
	if got := g.thread(1); got[len(got)-1] != "replied: yes" {
		t.Errorf("request one holds %v", got)
	}
	if g.latest() != 3 || !slices.Equal(g.thread(3), []string{"asked: the post"}) {
		t.Errorf("the post is request %d, holding %v", g.latest(), g.thread(g.latest()))
	}
}

func TestBridgesThreadlessPlaceBeginsNewRequest(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	chat := func(text string) map[string]any { return said("4471", "direct", "4471", text, "threadless", true) }
	d.hear("telegram", chat("the socket sparked"))
	d.answer(1, "steward", "The kitchen circuit is off.")
	d.hear("telegram", chat("the bins"))
	// A decision waits on Sam: his words, replying to nothing, are no
	// answer to it.
	d.decide(2, "Order a new bin?", "sam")
	d.drain()
	d.hear("telegram", chat("yes"))
	// A question answered waits no longer.
	_, err := d.requests.Put(3, d.requests.Worker("steward"), requests.KindQuestion, d.requests.Person("sam"), "which bin?", nil)
	must(t, err)
	d.drain()
	d.hear("telegram", chat("the green one"))
	d.hear("telegram", chat("and the blue"))
	want := [][]string{
		{"asked: the socket sparked", "answer: The kitchen circuit is off."},
		{"asked: the bins", "decision: Order a new bin?"},
		{"asked: yes", "question: which bin?", "replied: the green one"},
		{"asked: and the blue"},
	}
	for i, w := range want {
		if got := d.thread(int64(i + 1)); !slices.Equal(got, w) {
			t.Errorf("request %d holds %v", i+1, got)
		}
	}
	if d.latest() != 4 {
		t.Errorf("there are %d requests", d.latest())
	}
	// The same words at a place that holds threads go on in its latest
	// request.
	g := open(t, house(t))
	g.hear("telegram", said("4471", "direct", "4471", "the socket sparked"))
	g.answer(1, "steward", "The kitchen circuit is off.")
	g.hear("telegram", said("4471", "direct", "4471", "the bins"))
	if g.latest() != 1 {
		t.Errorf("at a place with threads, there are %d requests", g.latest())
	}
}

func TestBridgesWhereSaidIsNarrow(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "at the top")
	d.delivered("slack", said("U0AL0000", "#help", "C1/1.0", "a reply under it"))
	r, _ := d.requests.Get(n)
	if r.Origin.Place != "C1/1.0" || r.Origin.Heard != "#help" || len(d.thread(n)) != 2 || d.latest() != n {
		t.Errorf("the request is %#v, holding %v", r.Origin, d.thread(n))
	}
}

func TestBridgesFilesCopiedOutOfTheRun(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	picture := []byte("\x89PNG the whole picture")
	record := map[string][]byte{"output/heard/0/files/0/data": picture}
	fetch := func(path string) ([]byte, error) {
		data, ok := record[path]
		if !ok {
			return nil, errors.New("no such blob")
		}
		return data, nil
	}
	s := d.listening("telegram")
	_, err := d.InEnded("telegram", s.Labels["run"], bridges.End{Fetch: fetch, Output: heard(1,
		said("4471", "direct", "4471", "this socket", "files", []any{map[string]any{"name": "socket.png", "data": map[string]any{"$blob": "output/heard/0/files/0/data"}}}))})
	must(t, err)
	delete(record, "output/heard/0/files/0/data")
	es, _ := d.requests.Thread(1)
	if data, err := d.requests.File(1, es[0].ID, "socket.png", "sam"); err != nil || string(data) != string(picture) {
		t.Errorf("the picture is %q, %v", data, err)
	}
}

func TestBridgesChoiceGivenToItsDecision(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	n := d.ask("C1/1.0", "refund one")
	decision := d.decide(n, "Refund one charge", "lee")
	out := d.carry()
	lee := to(out, "slack", "U0LEE000")
	if len(lee) != 1 || lee[0].Input["ref"] == nil || !slices.Equal(lee[0].Input["choices"].([]string), []string{"yes", "no"}) {
		t.Fatalf("the decision is carried as %v", out)
	}
	d.ended(lee[0], map[string]any{"sent": "s12"})
	d.delivered("slack", map[string]any{"id": "press1", "kind": "choice", "from": "U0LEE000", "heard_in": "direct", "place": "U0LEE000", "answers": lee[0].Input["ref"], "choice": "yes"})
	if len(d.chosen) != 1 || d.chosen[0].Entry != decision.ID || d.chosen[0].Person != "lee" || d.chosen[0].Who != requests.Identity("slack", "U0LEE000") || d.chosen[0].Choice != "yes" {
		t.Errorf("the choice given is %#v", d.chosen)
	}
}

func TestBridgesReadersReplaced(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "one", "readers", []any{"U0DANA00", "U0AL0000", "U0GUEST0"}))
	readers := func() []string {
		ps, err := d.requests.ReadBy(1)
		must(t, err)
		var names []string
		for _, p := range ps {
			names = append(names, p.Name)
		}
		slices.Sort(names)
		return names
	}
	if got := readers(); !slices.Equal(got, []string{"al", "dana"}) {
		t.Errorf("the readers are %v", got)
	}
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "two"))
	if got := readers(); !slices.Equal(got, []string{"al", "dana"}) {
		t.Errorf("the readers are %v", got)
	}
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "three", "readers", []any{"U0DANA00"}))
	if got := readers(); !slices.Equal(got, []string{"dana"}) {
		t.Errorf("the readers are %v", got)
	}
	h := open(t, house(t))
	h.hear("telegram", said("4471", "direct", "4471", "direct"))
	if ps, _ := h.requests.ReadBy(1); len(ps) != 1 || ps[0].Name != "sam" {
		t.Errorf("a direct place is read by %v", ps)
	}
}

func TestBridgesInRunReadAfterRestart(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	s := d.listening("telegram")
	d.restart()
	if again := d.listen(); len(again) != 0 {
		t.Fatalf("a second listening run is started after a restart: %v", again)
	}
	_, err := d.InEnded("telegram", s.Labels["run"], bridges.End{Output: heard(3, said("4471", "direct", "4471", "while away"))})
	must(t, err)
	if got := d.thread(1); len(got) != 1 {
		t.Errorf("what was heard is %v", got)
	}
	if next := d.listening("telegram"); next.Input["cursor"] != float64(3) {
		t.Errorf("the next run is given %v", next.Input["cursor"])
	}
	// A delivery's run going when the server stopped, which its service
	// was answered as received, is found again and taken when it ends.
	w := open(t, work(t))
	going, err := w.Hear("slack", bridges.Delivery{Method: "POST", Body: "{}"})
	must(t, err)
	run := w.name()
	w.restart()
	_, err = w.InEnded(going.Bridge, run, bridges.End{Output: heard(0, said("U0DANA00", "#help", "C1/1.0", "answered as received"))})
	must(t, err)
	if got := w.thread(1); len(got) != 1 {
		t.Errorf("the delivery is taken as %v", got)
	}
}

func TestBridgesListeningRunThatCouldNotStart(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	starts, _, err := d.Listen()
	must(t, err)
	if len(starts) != 1 {
		t.Fatalf("the listening runs are %v", starts)
	}
	_, err = d.InEnded("telegram", "", bridges.End{Failed: "the start cannot be sent"})
	must(t, err)
	d.later(time.Second)
	if again := d.listen(); len(again) != 1 {
		t.Errorf("after a start that failed, the listening runs are %v", again)
	}
}

func TestBridgesListeningRunWithNoCursorPauses(t *testing.T) {
	t.Parallel()
	d := open(t, house(t))
	d.hear("telegram", said("4471", "direct", "4471", "first"))
	for i, output := range []any{"garbage", map[string]any{"heard": []any{}}, map[string]any{"heard": []any{}, "cursor": nil}} {
		s := d.listening("telegram")
		if s.Input["cursor"] != float64(1) {
			t.Fatalf("run %d is given the cursor %v", i, s.Input["cursor"])
		}
		_, err := d.InEnded("telegram", s.Labels["run"], bridges.End{Output: output})
		must(t, err)
		if again := d.listen(); len(again) != 0 {
			t.Fatalf("after ending with %v, a listening run is started at once", output)
		}
		troubles, err := d.Troubles()
		must(t, err)
		if len(troubles) != 1 || troubles[0].Failures != i+1 {
			t.Errorf("after ending with %v, the troubles are %v", output, troubles)
		}
		_, next, err := d.Listen()
		must(t, err)
		d.clock.Set(next)
	}
	if s := d.listening("telegram"); s.Input["cursor"] != float64(1) {
		t.Errorf("the cursor kept is %v", s.Input["cursor"])
	}
}

func TestBridgesReaderNewInAReply(t *testing.T) {
	t.Parallel()
	// Raj first reads Dana's thread when he replies in it: his reply is
	// said in her request, which is still the latest there.
	d := open(t, work(t))
	n := d.ask("C1/1.0", "customer 4411 was charged twice")
	d.delivered("slack", said("U0RAJ000", "#help", "C1/1.0", "I can look at the webhook", "readers", []any{"U0DANA00", "U0RAJ000"}))
	if d.latest() != n {
		t.Fatalf("Raj's reply made request %d", d.latest())
	}
	if got := d.thread(n); got[len(got)-1] != "said: I can look at the webhook" {
		t.Errorf("Dana's request holds %v", got)
	}
	d.delivered("slack", said("U0DANA00", "#help", "C1/1.0", "thanks"))
	if got := d.thread(n); d.latest() != n || got[len(got)-1] != "said: thanks" {
		t.Errorf("what is said there next is in %d, holding %v", d.latest(), got)
	}
	// Priya first reads the thread when she answers what the bot said
	// there.
	d.answer(n, "bot", "refunded")
	out := d.carry()
	if len(out) != 1 {
		t.Fatalf("the answer is carried by %v", places(out))
	}
	d.ended(out[0], map[string]any{"sent": "C1/2.0"})
	d.delivered("slack", said("U0PRI000", "#help", "C1/1.0", "payments refunded it", "answers", "C1/2.0", "readers", []any{"U0DANA00", "U0RAJ000", "U0PRI000"}))
	if got := d.thread(n); d.latest() != n || got[len(got)-1] != "said: payments refunded it" {
		t.Errorf("Priya's answer is in %d, and Dana's request holds %v", d.latest(), got)
	}
}
