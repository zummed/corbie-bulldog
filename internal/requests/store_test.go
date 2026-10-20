package requests_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

func TestRequestsListed(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	kit := got[*requests.Request](t)(h.AskInChat("kit", "", "the kitchen socket sparked", nil)).Number
	must(t, errOf(h.Answer(kit, h.Worker("steward"), "Replaced.", nil)))
	monday := got[*requests.Request](t)(h.AskByRoutine(office.Top, h.o.TopTeam().Routines[0], requests.Who{})).Number
	lights := got[*requests.Request](t)(h.AskInChat("sam", "", "the hall lights flicker", nil)).Number
	cut := h.decide(lights, "fixer", "Turn one circuit off", "cut_power(circuit: 4)", "sam")
	garden := got[*requests.Request](t)(h.AskInChat("sam", "garden", "the hedge", nil)).Number
	numbers := func(list []requests.Summary) []int64 {
		var ns []int64
		for _, s := range list {
			ns = append(ns, s.Number)
		}
		return ns
	}
	if list := got[[]requests.Summary](t)(h.List("sam", requests.Filter{Team: office.Top, State: requests.Answered})); !slices.Equal(numbers(list), []int64{kit}) {
		t.Errorf("Sam's answered requests of the house are %v", numbers(list))
	}
	if list := got[[]requests.Summary](t)(h.List("sam", requests.Filter{Team: office.Top, State: requests.Working})); !slices.Equal(numbers(list), []int64{monday}) {
		t.Errorf("Sam's working requests of the house are %v", numbers(list))
	}
	if list := got[[]requests.Summary](t)(h.List("sam", requests.Filter{Team: office.Top, State: requests.Waiting})); !slices.Equal(numbers(list), []int64{lights}) {
		t.Errorf("Sam's waiting requests of the house are %v", numbers(list))
	}
	if list := got[[]requests.Summary](t)(h.List("sam", requests.Filter{})); !slices.Equal(numbers(list), []int64{garden, lights, monday, kit}) {
		t.Errorf("Sam's requests are %v", numbers(list))
	}
	if ws := got[[]requests.Wait](t)(h.WaitingOn("sam")); len(ws) != 1 || ws[0].Entry != cut.ID || ws[0].Text != "Turn one circuit off" {
		t.Errorf("what waits on Sam is %+v", ws)
	}
	if list := got[[]requests.Summary](t)(h.List("kit", requests.Filter{Asked: true})); !slices.Equal(numbers(list), []int64{kit}) || list[0].First != "the kitchen socket sparked" {
		t.Errorf("what Kit asked is %+v", list)
	}
	if open := got[[]requests.Summary](t)(h.OpenOf(office.Top)); !slices.Equal(numbers(open), []int64{lights, monday}) || open[0].State.Is != requests.Waiting || open[1].State.Is != requests.Working {
		t.Errorf("the house's open requests are %+v", open)
	}
	direct := got[*requests.Request](t)(h.AskAt("telegram", office.Direct, "4472", "", "4472", "hello", nil, false)).Number
	hedge := got[*requests.Request](t)(h.AskAt("telegram", "garden-chat", "garden-chat", "", "4472", "the hedge again", nil, false)).Number
	if list := got[[]requests.Summary](t)(h.List("sam", requests.Filter{Bridge: "telegram"})); !slices.Equal(numbers(list), []int64{hedge, direct}) {
		t.Errorf("Sam's requests asked on Telegram are %v", numbers(list))
	}
	if list := got[[]requests.Summary](t)(h.List("sam", requests.Filter{Bridge: "telegram", Place: "garden-chat"})); !slices.Equal(numbers(list), []int64{hedge}) {
		t.Errorf("Sam's requests asked in Telegram's garden-chat are %v", numbers(list))
	}
}

func TestRequestsOneThingOneWrite(t *testing.T) {
	t.Parallel()
	h := open(t, house(t))
	kit := got[*requests.Request](t)(h.AskInChat("kit", "", "the socket", nil)).Number
	must(t, errOf(h.Put(kit, h.Worker("fixer"), requests.KindQuestion, h.Person("sam"), "may I?", nil)))
	_, err := h.db.Exec(`CREATE TRIGGER fail_entry BEFORE INSERT ON requests_entries WHEN NEW.text LIKE 'fail%' OR NEW.text LIKE 'stopped%'
		BEGIN SELECT RAISE(ABORT, 'the disk is full'); END`)
	must(t, err)
	before := [3]int{h.count("requests_requests"), h.count("requests_entries"), h.count("requests_waits")}
	if _, err := h.AskInChat("kit", "", "fail to ask", nil); err == nil {
		t.Error("a request whose asking cannot be kept is made")
	}
	if _, err := h.Sub(kit, h.Worker("steward"), "garden", "fail to ask the garden", "t"); err == nil {
		t.Error("a sub-request whose question cannot be kept is made")
	}
	if _, err := h.Stop(kit, h.Person("sam")); err == nil {
		t.Error("a stop whose entry cannot be kept stops")
	}
	after := [3]int{h.count("requests_requests"), h.count("requests_entries"), h.count("requests_waits")}
	if before != after {
		t.Errorf("what failed part of the way left %v, from %v", after, before)
	}
	if r := got[*requests.Request](t)(h.Get(kit)); r.Stopped || r.Answered {
		t.Errorf("the failed stop left the request %+v", r)
	}
}

func TestRequestsToldAfterWritten(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	must(t, d.Readers("slack", thread, []string{slackDana}))
	var told []int64
	var seen []int
	stop := d.Watch(func(n int64) {
		told = append(told, n)
		seen = append(seen, len(d.thread(n)))
	})
	defer stop()
	dana := got[*requests.Request](t)(d.AskAt("slack", "#help", thread, "", slackDana, "help", nil, false)).Number
	if !slices.Equal(told, []int64{dana}) || seen[0] != 1 {
		t.Fatalf("asking told %v, seeing %v", told, seen)
	}
	told, seen = nil, nil
	sub := got[*requests.Request](t)(d.Sub(dana, d.Worker("bot"), "payments", "refund", "t")).Number
	if !slices.Equal(told, []int64{sub, dana}) || !slices.Equal(seen, []int{1, 2}) {
		t.Errorf("the sub-request told %v, seeing %v", told, seen)
	}
	told, seen = nil, nil
	must(t, errOf(d.Stop(dana, d.Person("al"))))
	if !slices.Equal(told, []int64{dana, sub}) || !slices.Equal(seen, []int{3, 2}) {
		t.Errorf("the stop told %v, seeing %v", told, seen)
	}
}

func TestRequestsFailureNotARefusal(t *testing.T) {
	t.Parallel()
	d := workDay(t)
	d.db.Close()
	var refusal requests.Refusal
	for _, err := range []error{
		errOf(d.View(d.dana, "dana")), errOf(d.List("dana", requests.Filter{})), errOf(d.WaitingOn("dana")),
		errOf(d.Sees(d.dana, "dana")), errOf(d.MaySay(d.dana, d.Person("dana"), requests.Screens)),
		errOf(d.Say(d.dana, d.Person("dana"), "more", nil, requests.Screens, false)), errOf(d.AskInChat("dana", "", "new", nil)),
	} {
		if err == nil || errors.Is(err, requests.ErrNotThere) || errors.As(err, &refusal) {
			t.Errorf("a store that cannot be read is told as %v", err)
		}
	}
	// One watching is told the store failed, with no number, and not
	// nothing.
	w := workDay(t)
	w.Watch(func(int64) { must(t, errOf(w.db.Exec(`ALTER TABLE requests_files RENAME TO requests_lost`))) })
	var told []error
	stop := w.WatchAs("raj", func(n int64, err error) {
		if n != 0 {
			t.Errorf("the failure is told with %d", n)
		}
		told = append(told, err)
	})
	defer stop()
	must(t, errOf(w.AskInChat("dana", "", "in the chat", nil)))
	if len(told) != 1 || told[0] == nil || errors.Is(told[0], requests.ErrNotThere) {
		t.Errorf("the watcher is told %v", told)
	}
}

func TestRequestsOlderStoreRefused(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "bulldog.db")
	db, err := sql.Open("sqlite", file)
	must(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE parts(area TEXT PRIMARY KEY, version INTEGER); INSERT INTO parts VALUES ('requests', 0)`)
	must(t, err)
	if _, err := requests.Open(db, testkit.NewClock(testkit.Monday).Now); err == nil || !strings.Contains(err.Error(), "remove .bulldog/bulldog.db and start again") {
		t.Errorf("an older store is told %v", err)
	}
	other := filepath.Join(t.TempDir(), "bulldog.db")
	db2, err := sql.Open("sqlite", other)
	must(t, err)
	defer db2.Close()
	_, err = db2.Exec(`CREATE TABLE requests_requests(number INTEGER)`)
	must(t, err)
	if _, err := requests.Open(db2, testkit.NewClock(testkit.Monday).Now); err == nil || !strings.Contains(err.Error(), "older Bulldog") {
		t.Errorf("a store from before parts were kept is told %v", err)
	}
	fresh, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "bulldog.db"))
	must(t, err)
	defer fresh.Close()
	s, err := requests.Open(fresh, testkit.NewClock(testkit.Monday).Now)
	must(t, err)
	if _, err := s.AskInChat("kit", "", "hello", nil); err == nil || !strings.Contains(err.Error(), "not been read") {
		t.Errorf("asking before the office is read is told %v", err)
	}
	var version int
	must(t, fresh.QueryRow(`SELECT version FROM parts WHERE area = 'requests'`).Scan(&version))
	if version != requests.Version {
		t.Errorf("the part is kept as version %d", version)
	}
}
