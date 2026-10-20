package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// numbered gives the number of the request an ask made.
func numbered(r reply) int64 {
	r.t.Helper()
	return int64(r.is(http.StatusOK).json()["number"].(float64))
}

func TestServerAskInTheChat(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	kit := s.as("kit")
	garden := numbered(kit.post("/api/requests", map[string]string{"team": "garden", "text": "the lawn is long"}))
	house := numbered(kit.post("/api/requests", map[string]string{"text": "is the kettle on?"}))
	for n, team := range map[int64]string{garden: "garden", house: "office"} {
		r := s.get(n)
		if r.Team != team || r.Asker.Kind != requests.IsPerson || r.Asker.Name != "kit" || r.Origin.Kind != requests.InChat {
			t.Errorf("Kit's request %d is %+v", n, r)
		}
	}
	// A request with files is a form of several parts: the text and the
	// photo are its first entry.
	photo := []byte("\x89PNG\r\n\x1a\n a photo of the socket")
	n := numbered(kit.form("/api/requests", map[string]string{"text": "this socket sparked"}, map[string][]byte{"socket.png": photo}))
	thread, err := s.areasOf().Requests.Thread(n)
	if err != nil {
		t.Fatal(err)
	}
	if first := thread[0]; first.Text != "this socket sparked" || len(first.Files) != 1 || first.Files[0].Name != "socket.png" || first.Files[0].Size != len(photo) {
		t.Errorf("the request with a photo begins %+v", first)
	}
}

// playHouse plays the house's workers for the screens' tests: the
// steward asks whoever asked which socket, when asked about one, and has
// the fixer order a part when asked to; the fixer orders a £300 part,
// which the small-orders rule refers on to Sam.
func playHouse(turn testkit.Turn) testkit.Reply {
	switch {
	case strings.Contains(turn.System, "You look after the house"):
		switch asked := turn.Prompt(); {
		case turn.Number == 1 && strings.Contains(asked, "socket"):
			return testkit.Use("ask_person", map[string]any{"question": "Which socket?"})
		case turn.Number == 1 && strings.Contains(asked, "pump"):
			return testkit.Use("fixer", map[string]any{"job": "Order a new boiler pump."})
		}
		return testkit.Say("It is seen to.")
	case strings.Contains(turn.System, "You do the repairs"):
		if turn.Number == 1 {
			return testkit.Use("order_part", map[string]any{"part": "boiler pump", "pounds": 300})
		}
		return testkit.Say("The pump is ordered.")
	}
	return testkit.Say("done")
}

func TestServerSayMore(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(playHouse)
	s := b.serve()
	n := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "the kitchen socket sparked"}))
	question := s.waitsOn(n, "kit")
	// The steward's question to Kit is answered by naming it.
	s.as("kit").post("/api/requests/"+itoa(n)+"/entries", map[string]any{"text": "The one by the kettle.", "re": question.Entry}).is(http.StatusOK)
	s.answered(n)
	for _, person := range []string{"kit", "sam"} {
		e := s.as(person).post("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": person + " says more"}).is(http.StatusOK).json()
		entry, err := s.areasOf().Requests.Entry(int64(e["entry"].(float64)))
		if err != nil || entry.By.Name != person || entry.Request != n || !entry.In.Screens {
			t.Errorf("%s's entry is %+v (%v)", person, entry, err)
		}
	}

	d := refundDay(t, refundOffice(t))
	raj := d.s.as("raj").post("/api/requests/"+itoa(d.n)+"/entries", map[string]string{"text": "Raj looks too"}).is(http.StatusOK).json()
	if e, _ := d.s.areasOf().Requests.Entry(int64(raj["entry"].(float64))); len(e.To) != 1 || e.To[0].Kind != requests.IsTeam || e.To[0].Name != "office" {
		t.Errorf("Raj's entry is to %+v", e.To)
	}
	al := d.s.as("al").post("/api/requests/"+itoa(d.payments)+"/entries", map[string]string{"text": "Al asks payments"}).is(http.StatusOK).json()
	if e, _ := d.s.areasOf().Requests.Entry(int64(al["entry"].(float64))); len(e.To) != 1 || e.To[0].Name != "payments" {
		t.Errorf("Al's entry is to %+v", e.To)
	}
	if r := d.s.as("dana").post("/api/requests/"+itoa(d.payments)+"/entries", map[string]string{"text": "Dana tries"}); r.status != http.StatusConflict || r.json()["error"] == "" {
		t.Errorf("Dana saying more in payments' request answered %d %s", r.status, r.body)
	}
	d.s.as("lee").post("/api/requests/"+itoa(d.n)+"/entries", map[string]string{"text": "Lee tries"}).is(http.StatusNotFound)
}

func TestServerDecide(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	path := "/api/requests/" + itoa(d.payments) + "/decisions/" + itoa(d.refund)
	if r := d.s.as("dana").post(path, map[string]string{"says": "yes"}); r.status != http.StatusConflict || r.json()["error"] == "" {
		t.Errorf("Dana's yes answered %d %s", r.status, r.body)
	}
	d.s.as("lee").post(path, map[string]string{"says": "yes", "note": "A duplicate, plainly."}).is(http.StatusOK)
	dec, err := d.s.areasOf().Decisions.Get(d.refund)
	if err != nil || dec.Ended != "yes" || dec.By.Name != "lee" || dec.By.Note != "A duplicate, plainly." {
		t.Errorf("the refund ended %+v (%v)", dec, err)
	}

	// In the house, a £300 order the small-orders rule referred reaches Sam
	// with its note.
	b := newBench(t, house(t))
	b.says(playHouse)
	s := b.serve()
	n := numbered(s.as("kit").post("/api/requests", map[string]string{"text": "the boiler pump is dead"}))
	s.waitsOn(n, "sam")
	var v struct{ Decisions []decisionOut }
	s.as("sam").get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	if len(v.Decisions) != 1 || !v.Decisions[0].Mine || len(v.Decisions[0].Referrals) != 1 || v.Decisions[0].Referrals[0].Note != "£300 is more than I order alone." {
		t.Errorf("Sam is shown the order as %+v", v.Decisions)
	}
}

func TestServerStop(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	stop := func(person string, n int64) reply { return d.s.as(person).post("/api/requests/"+itoa(n)+"/stop", "") }
	if r := stop("raj", d.payments); r.status != http.StatusConflict || r.json()["error"] == "" {
		t.Errorf("Raj's stop of payments' request answered %d %s", r.status, r.body)
	}
	stop("lee", d.payments).is(http.StatusOK)
	if r := d.s.get(d.payments); !r.Stopped {
		t.Errorf("payments' request is %+v", r)
	}
	platform := numbered(d.s.as("raj").post("/api/requests", map[string]string{"team": "platform", "text": "is the pipeline green?"}))
	stop("dana", platform).is(http.StatusNotFound)
	stop("al", platform).is(http.StatusOK)
	stop("al", d.n).is(http.StatusOK)
	own := numbered(d.s.as("dana").post("/api/requests", map[string]string{"team": "support", "text": "a note to myself"}))
	stop("dana", own).is(http.StatusOK)
	for _, n := range []int64{platform, d.n, own} {
		if r := d.s.get(n); !r.Stopped {
			t.Errorf("request %d is %+v", n, r)
		}
	}
}

func TestServerRoutineNow(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	walk := map[string]string{"at": "mon 09:00", "to": "steward", "say": "Walk round: what is still open?"}
	n := numbered(s.as("sam").post("/api/teams/office/routines/now", walk))
	if r := s.get(n); r.Origin.Kind != requests.ByRoutine || r.Worker != "steward" || r.Team != "office" || r.Origin.By.Kind != requests.IsPerson || r.Origin.By.Name != "sam" {
		t.Errorf("the routine's request is %+v", r)
	}
	s.answered(n)
	garden := map[string]string{"at": "fri 16:00", "to": "gardener", "say": "What needs doing before the weekend?"}
	s.as("kit").post("/api/teams/garden/routines/now", garden).is(http.StatusNotFound)
	otherwise := map[string]string{"at": "mon 09:00", "to": "steward", "say": "Walk round."}
	s.as("sam").post("/api/teams/office/routines/now", otherwise).is(http.StatusNotFound)
}

func TestServerDraftCheckPut(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	draft := map[string]any{"files": map[string]any{"porter.yaml": "agent: porter\nmodel: usual\nsystem: You carry things.\nprompt: ${input}\n"}}
	before, err := os.ReadFile(filepath.Join(b.dir, "office.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var c checkOut
	s.as("sam").post("/api/office/check", draft).is(http.StatusOK).into(&c)
	if len(c.Files) != 1 || c.Files[0].Path != "porter.yaml" || len(c.Says) == 0 || c.Mark == "" || c.Problems == nil || c.LetsMoreBy == nil {
		t.Errorf("Sam's check gives %+v", c)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "porter.yaml")); !os.IsNotExist(err) {
		t.Error("the check wrote the new worker")
	}
	if after, _ := os.ReadFile(filepath.Join(b.dir, "office.yaml")); string(after) != string(before) {
		t.Error("the check changed office.yaml")
	}
	if r := s.as("kit").post("/api/office/check", draft); r.status != http.StatusConflict || !strings.Contains(r.json()["error"].(string), "ask for it") {
		t.Errorf("Kit's check answered %d %s", r.status, r.body)
	}
	// A why of nothing but spaces is refused, as changes refuses it, and
	// is no failure.
	blank := map[string]any{"files": draft["files"], "why": "  ", "mark": c.Mark}
	if r := s.as("sam").post("/api/office/put", blank); r.status != http.StatusBadRequest || s.logged("a failure") {
		t.Errorf("a put with a blank why answered %d %s", r.status, r.body)
	}
	// Put with the mark of what Sam was shown, it is written at once:
	// nothing in the house is asked of a change before Sam.
	put := map[string]any{"files": draft["files"], "why": "A porter for the parcels.", "mark": c.Mark}
	if r := s.as("sam").post("/api/office/put", put).is(http.StatusOK).json(); r["ended"] != changes.Written {
		t.Errorf("Sam's put gives %v", r)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "porter.yaml")); err != nil {
		t.Errorf("the put did not write the new worker: %v", err)
	}
	// Put again with a mark that is not the folder's now, it is told to
	// look again.
	if r := s.as("sam").post("/api/office/put", put); r.status != http.StatusConflict || !strings.Contains(r.json()["error"].(string), "Look at it again") || s.logged("a failure") {
		t.Errorf("a put with an old mark answered %d %s", r.status, r.body)
	}

	// In the house as the server writes it, a bridge Bulldog ships is set
	// by its entry alone, naming the folder it runs.
	fresh := newBench(t, newHouse()).serve()
	var added checkOut
	entry := map[string]any{"from": "./.bulldog/bridges/telegram", "places": map[string]any{"direct": map[string]any{}}}
	fresh.as("sam").post("/api/office/check", map[string]any{"set": []any{map[string]any{"path": "office.yaml", "field": []string{"bridges", "telegram"}, "value": entry}}}).is(http.StatusOK).into(&added)
	if len(added.Files) != 1 || !added.Fit || !strings.Contains(added.Files[0].Text, "telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }") {
		t.Errorf("setting Telegram gives %+v", added)
	}
}

func TestServerEveryActionHasItsPerson(t *testing.T) {
	t.Parallel()
	d := refundDay(t, refundOffice(t))
	a := d.s.areasOf()
	n := numbered(d.s.as("raj").post("/api/requests", map[string]string{"team": "support", "text": "can you call the customer?"}))
	if r := d.s.get(n); r.Asker.Kind != requests.IsPerson || r.Asker.Name != "raj" {
		t.Errorf("Raj's request was asked by %+v", r.Asker)
	}
	said := d.s.as("dana").post("/api/requests/"+itoa(n)+"/entries", map[string]string{"text": "I will."}).is(http.StatusOK).json()
	if e, _ := a.Requests.Entry(int64(said["entry"].(float64))); e.By.Kind != requests.IsPerson || e.By.Name != "dana" {
		t.Errorf("Dana's answer is by %+v", e.By)
	}
	d.s.as("lee").post("/api/requests/"+itoa(d.payments)+"/decisions/"+itoa(d.refund), map[string]string{"says": "refuse", "note": "Not yet."}).is(http.StatusOK)
	if dec, _ := a.Decisions.Get(d.refund); dec.By == nil || dec.By.Name != "lee" {
		t.Errorf("the refund was decided by %+v", dec.By)
	}
	d.s.as("al").post("/api/requests/"+itoa(d.n)+"/stop", "").is(http.StatusOK)
	thread, _ := a.Requests.Thread(d.n)
	if last := thread[len(thread)-1]; last.Text != "stopped by Al" {
		t.Errorf("Dana's request ends %+v", last)
	}
	check := d.s.as("al").post("/api/office/check", map[string]any{"files": map[string]any{"support/notes/calls.md": "Call back within a day.\n"}}).is(http.StatusOK).json()
	d.s.as("al").post("/api/office/put", map[string]any{"files": map[string]any{"support/notes/calls.md": "Call back within a day.\n"}, "why": "How support calls back.", "mark": check["mark"]}).is(http.StatusOK)
	history, err := a.Changes.History("al")
	if err != nil || history[0].By != "al" || history[0].DecidedBy != "al" {
		t.Errorf("the change was made by %+v (%v)", history[0], err)
	}
}
