package bridges

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
)

// The states of a thing to be carried.
const (
	Waiting   = "waiting"
	Sending   = "sending"
	Sent      = "sent"
	GivenUp   = "given up"
	Withdrawn = "withdrawn"
)

// The kinds of thing carried: an entry of a request, a sentence answering
// what someone said on a bridge, a one-time sign-in link, and a mark of
// what a team is doing at a request's origin, which its text says.
const (
	ofEntry    = "entry"
	ofSentence = "sentence"
	ofSignIn   = "sign-in"
	ofMark     = "mark"
)

// marks says whether a kind of thing is a mark: no entry's carrying, and
// shown on no request.
func marks(kind string) bool { return kind == ofMark }

// Chat is the built-in chat's name as a bridge, which no bridge of the
// office takes.
const Chat = "chat"

// A Carried is one thing carried on a bridge, or to be: the entry it is,
// where it goes, and how its runs went. A thing given up says why. One
// on the built-in chat is no thing kept here, and has no Thing.
type Carried struct {
	Thing  int64
	Entry  int64
	Bridge string
	// Place is where it goes: an origin's place, or a person's identity
	// on the bridge, when Person names them, which on the chat is their
	// name. The chat as an origin has no place.
	Place  string
	Person string
	State  string
	Why    string
	Runs   int
	Sent   string
	Made   time.Time
}

// A thingRow is what is kept of a thing to be carried.
type thingRow struct {
	id      int64
	bridge  string
	place   string
	direct  bool
	person  string
	kind    string
	request int64
	entry   int64
	text    string
	state   string
	run     string
	// ended is the last run whose ending was taken.
	ended string
	runs  int
	next  time.Time
	sent  string
	why   string
	made  time.Time
}

const thingColumns = `id, bridge, place, direct, person, kind, request, entry, text, state, run, ended, runs, next, sent, why, made`

func (b *Bridges) things(where string, args ...any) ([]thingRow, error) {
	rows, err := b.db.Query(`SELECT `+thingColumns+` FROM bridges_things `+where, args...)
	if err != nil {
		return nil, failed(err)
	}
	defer rows.Close()
	var things []thingRow
	for rows.Next() {
		var th thingRow
		var next, made string
		if err := rows.Scan(&th.id, &th.bridge, &th.place, &th.direct, &th.person, &th.kind, &th.request, &th.entry, &th.text,
			&th.state, &th.run, &th.ended, &th.runs, &next, &th.sent, &th.why, &made); err != nil {
			return nil, failed(err)
		}
		th.next, th.made = unstamp(next), unstamp(made)
		things = append(things, th)
	}
	if err := rows.Err(); err != nil {
		return nil, failed(err)
	}
	return things, nil
}

func (b *Bridges) thing(id int64) (*thingRow, error) {
	things, err := b.things(`WHERE id = ?`, id)
	if err != nil || len(things) == 0 {
		return nil, err
	}
	return &things[0], nil
}

// sentThing gives the entry carried on a bridge that the service gave a
// reference for, among the references kept for it one per line, or nil.
// A mark names the message it marks, which was never sent.
func (b *Bridges) sentThing(bridge, sent string) (*thingRow, error) {
	things, err := b.things(`WHERE bridge = ? AND instr(char(10) || sent || char(10), char(10) || ? || char(10)) > 0 AND state = ? AND kind = ? ORDER BY id DESC LIMIT 1`,
		bridge, sent, Sent, ofEntry)
	if err != nil || len(things) == 0 {
		return nil, err
	}
	return &things[0], nil
}

// ref is Bulldog's own reference for a thing carried, handed to the out
// machine with a question or a decision and back with a choice on it. It
// grants nothing: a choice is taken only from the identity it was for.
func ref(id int64) string { return "r" + strconv.FormatInt(id, 10) }

// refThing gives the thing carried on a bridge that Bulldog's reference
// names, or nil.
func (b *Bridges) refThing(bridge, r string) (*thingRow, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r, "r"), 10, 64)
	if !strings.HasPrefix(r, "r") || err != nil {
		return nil, nil
	}
	th, err := b.thing(id)
	if err != nil || th == nil || th.bridge != bridge {
		return nil, err
	}
	return th, nil
}

// add keeps a thing to be carried, once for each entry, place and kind.
// One given up before any run of it was started, while its bridge was
// stopped, is kept again when it is to be carried once more.
func (b *Bridges) add(th thingRow) error {
	if th.entry != 0 {
		var n int
		if err := b.db.QueryRow(`SELECT count(*) FROM bridges_things WHERE entry = ? AND bridge = ? AND place = ? AND kind = ? AND NOT (state = ? AND runs = 0)`,
			th.entry, th.bridge, th.place, th.kind, GivenUp).Scan(&n); err != nil {
			return failed(err)
		}
		if n > 0 {
			return nil
		}
	}
	if th.state == "" {
		th.state = Waiting
	}
	_, err := b.db.Exec(`INSERT INTO bridges_things(bridge, place, direct, person, kind, request, entry, text, state, why, made)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		th.bridge, th.place, th.direct, th.person, th.kind, th.request, th.entry, th.text, th.state, th.why, stamp(b.now()))
	if err != nil {
		return failed(err)
	}
	b.carried(th)
	return nil
}

// sentence keeps a sentence to be carried back to whoever said a thing, at
// the place they said it: said directly, to them directly.
func (b *Bridges) sentence(bridge string, t thing, text string) error {
	if t.HeardIn == office.Direct {
		return b.add(thingRow{bridge: bridge, place: t.From, direct: true, kind: ofSentence, text: text})
	}
	return b.add(thingRow{bridge: bridge, place: t.Place, kind: ofSentence, text: text})
}

// directOrigin says whether a request's origin is its asker's direct place
// itself, so what goes to it goes to them directly; one said in a thread
// of their direct conversation, which some services hold, is that thread,
// and what goes to it goes there, as to a channel's.
func (b *Bridges) directOrigin(origin *requests.Origin, n int64) (bool, error) {
	if origin.Heard != office.Direct {
		return false, nil
	}
	r, err := b.requests.Get(n)
	if err != nil {
		return false, err
	}
	return origin.Place == r.Asker.ID, nil
}

// takesDirect says whether a bridge's out machine says something to a
// person directly: it takes the input direct. A bridge whose service has
// no direct place carries only what goes to an origin.
func takesDirect(bridge *office.Bridge) bool {
	return bridge.Out != nil && office.Takes(bridge.Out, InputDirect)
}

// takesWorking says whether a bridge's out machine marks what a team is
// working on: it takes the input working. Any other is never told.
func takesWorking(bridge *office.Bridge) bool {
	return bridge.Out != nil && office.Takes(bridge.Out, InputWorking)
}

// What a mark says a team is doing in the thread at a request's origin:
// working on it, waiting on a person, or done.
const (
	markWorking = "working"
	markWaiting = "waiting"
	markDone    = "done"
)

// Working keeps a mark, to be carried to a request's origin, that the team
// is working on it: a run of its lead has started. Only a bridge that
// runs, and whose out machine takes working, is told. What waits to be
// carried is looked at first, so the mark goes after everything said
// before it.
func (b *Bridges) Working(request int64) error { return b.marking(request, markWorking) }

// Worked keeps a mark, to be carried to a request's origin, that the team
// is done with it, once its lead's run has ended. What the run said is
// looked at first, so its answer goes before the mark.
func (b *Bridges) Worked(request int64) error { return b.marking(request, markDone) }

func (b *Bridges) marking(request int64, state string) error {
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return err
	}
	if err := b.scan(o); err != nil {
		return err
	}
	return b.markAs(o, request, state)
}

// lastMark gives the latest mark of a request that went, is to go, or was
// given up by a run that may have made it all the same, or nil.
func (b *Bridges) lastMark(request int64) (*thingRow, error) {
	things, err := b.things(`WHERE request = ? AND kind = ? AND state != ? AND NOT (state = ? AND runs = 0) ORDER BY id DESC LIMIT 1`,
		request, ofMark, Withdrawn, GivenUp)
	if err != nil || len(things) == 0 {
		return nil, err
	}
	return &things[0], nil
}

// markAs keeps a mark of what a team is doing at a request's origin,
// unless its latest mark says so already: so done comes only after the
// team was marked at work there, whether that mark went or not, and
// waiting only while it is at work.
func (b *Bridges) markAs(o *office.Office, request int64, state string) error {
	last, err := b.lastMark(request)
	switch {
	case err != nil:
		return err
	case last != nil && last.text == state:
		return nil
	case state != markWorking && (last == nil || last.text == markDone):
		return nil
	}
	r, err := b.requests.Get(request)
	if err != nil {
		return err
	}
	origin := r.Origin
	if origin.Kind != requests.AtPlace {
		return nil
	}
	if bridge := o.Bridges[origin.Bridge]; bridge == nil || !bridge.Runs() || !takesWorking(bridge) {
		return nil
	}
	direct, err := b.directOrigin(&origin, request)
	if err != nil {
		return err
	}
	return b.add(thingRow{bridge: origin.Bridge, place: origin.Place, direct: direct, kind: ofMark, request: request, text: state})
}

// follow keeps, while a request's lead is marked at work at its origin, a
// mark of where the request stands: waiting while it waits on a person,
// as on a question put to them or a decision that has reached them, and
// working again once nothing does.
func (b *Bridges) follow(o *office.Office, n int64) error {
	last, err := b.lastMark(n)
	if err != nil || last == nil || last.text == markDone {
		return err
	}
	st, err := b.requests.State(n)
	if err != nil {
		return err
	}
	switch {
	case st.Is == requests.Waiting && slices.ContainsFunc(st.On, func(p requests.Party) bool { return p.Kind == requests.IsPerson || p.Kind == requests.IsGuest }):
		return b.markAs(o, n, markWaiting)
	case st.Is == requests.Waiting || st.Is == requests.Working:
		return b.markAs(o, n, markWorking)
	}
	return nil
}

// scan looks at every request the store has told of since it was last
// looked at, and the first time at every request, for entries not yet
// looked at, and keeps what each is to be carried as.
func (b *Bridges) scan(o *office.Office) error {
	b.pendingMu.Lock()
	pending := b.pending
	b.pending = map[int64]bool{}
	b.pendingMu.Unlock()
	if !b.scanned {
		for n := int64(1); ; n++ {
			if _, err := b.requests.Get(n); errors.Is(err, requests.ErrNotThere) {
				break
			} else if err != nil {
				b.markAll(slices.Collect(maps.Keys(pending)))
				return err
			}
			pending[n] = true
		}
		b.scanned = true
	}
	numbers := slices.Sorted(maps.Keys(pending))
	for i, n := range numbers {
		if err := b.look(o, n); err != nil {
			b.markAll(numbers[i:])
			return err
		}
	}
	return nil
}

// markAll keeps again the requests a scan did not get to look at, so the
// next scan does.
func (b *Bridges) markAll(numbers []int64) {
	for _, n := range numbers {
		b.mark(n)
	}
}

// look keeps what each entry of a request not yet looked at is to be
// carried as, and then a mark of where it stands, when its lead is marked
// at work.
func (b *Bridges) look(o *office.Office, n int64) error {
	var seen int64
	if err := b.db.QueryRow(`SELECT entry FROM bridges_seen WHERE request = ?`, n).Scan(&seen); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return failed(err)
	}
	thread, err := b.requests.Thread(n)
	if err != nil {
		return err
	}
	for _, e := range thread {
		if e.ID <= seen {
			continue
		}
		if err := b.queue(o, e, "", ""); err != nil {
			return err
		}
		if _, err := b.db.Exec(`INSERT INTO bridges_seen(request, entry) VALUES (?, ?)
			ON CONFLICT(request) DO UPDATE SET entry = excluded.entry`, n, e.ID); err != nil {
			return failed(err)
		}
	}
	return b.follow(o, n)
}

// queue keeps where an entry is carried, as Requests says: to its
// request's origin on a bridge, unless it was said there; and to each
// person it is for, on each bridge that knows them. What goes on a bridge
// that is not in the office, or is stopped by a problem, is given up.
// Given a bridge and a person, it keeps only what goes on that bridge,
// and of what goes to a person directly, only what goes to that one.
func (b *Bridges) queue(o *office.Office, e requests.Entry, bridge, person string) error {
	ds, err := b.requests.Deliveries(e.ID)
	if err != nil {
		return err
	}
	for _, d := range ds {
		switch {
		case d.Origin != nil:
			origin := d.Origin
			if origin.Kind != requests.AtPlace || e.In.Bridge == origin.Bridge && e.In.Place == origin.Place || bridge != "" && origin.Bridge != bridge {
				continue
			}
			direct, err := b.directOrigin(origin, e.Request)
			if err != nil {
				return err
			}
			th := thingRow{bridge: origin.Bridge, place: origin.Place, direct: direct, kind: ofEntry, request: e.Request, entry: e.ID}
			if bridge := o.Bridges[origin.Bridge]; bridge == nil || !bridge.Runs() {
				th.state, th.why = GivenUp, stopped(origin.Bridge, bridge)
			}
			if err := b.add(th); err != nil {
				return err
			}
		case d.Person != "" && (person == "" || d.Person == person):
			if err := b.queuePerson(o, e, d.Person, bridge); err != nil {
				return err
			}
		}
	}
	return nil
}

// queuePerson keeps an entry to be carried to a person directly on each
// bridge that knows them, or on one of them alone. A bridge with no
// direct place carries none. On one stopped by a problem it is given up,
// and once the bridge is mended, what still waits on the person is
// carried as to someone newly reached.
func (b *Bridges) queuePerson(o *office.Office, e requests.Entry, person, only string) error {
	p := o.People[person]
	if p == nil {
		return nil
	}
	for _, id := range p.Reached {
		bridge := o.Bridges[id.Bridge]
		if bridge == nil || only != "" && id.Bridge != only || bridge.Runs() && !takesDirect(bridge) {
			continue
		}
		th := thingRow{bridge: id.Bridge, place: id.ID, direct: true, person: person, kind: ofEntry, request: e.Request, entry: e.ID}
		if !bridge.Runs() {
			th.state, th.why = GivenUp, stopped(id.Bridge, bridge)
		}
		if err := b.add(th); err != nil {
			return err
		}
	}
	return nil
}

// stopped says why a bridge carries nothing.
func stopped(name string, bridge *office.Bridge) string {
	if bridge == nil {
		return fmt.Sprintf("the bridge %s is not in the office", name)
	}
	return fmt.Sprintf("the bridge %s is stopped by a problem of the office, so it carries nothing", name)
}

// Carry gives the out runs to start now: for each place, the first thing
// not yet carried, when no run of it is going and its pause after a
// failure is over, each judged now for whether it is still to go there;
// and when to look again for one whose pause is not over. A thing no
// longer for its place, or whose bridge carries nothing now, is left, and
// the next is looked at.
func (b *Bridges) Carry() ([]Start, time.Time, error) {
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return nil, time.Time{}, err
	}
	if err := b.scan(o); err != nil {
		return nil, time.Time{}, err
	}
	rows, err := b.db.Query(`SELECT DISTINCT bridge, place FROM bridges_things WHERE state IN (?, ?) ORDER BY bridge, place`, Waiting, Sending)
	if err != nil {
		return nil, time.Time{}, failed(err)
	}
	var places [][2]string
	for rows.Next() {
		var bridge, place string
		if err := rows.Scan(&bridge, &place); err != nil {
			rows.Close()
			return nil, time.Time{}, failed(err)
		}
		places = append(places, [2]string{bridge, place})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, failed(err)
	}
	var starts []Start
	var soonest time.Time
	for _, at := range places {
		for {
			things, err := b.things(`WHERE bridge = ? AND place = ? AND state IN (?, ?) ORDER BY id LIMIT 1`, at[0], at[1], Waiting, Sending)
			if err != nil {
				return nil, time.Time{}, err
			}
			if len(things) == 0 || things[0].state == Sending {
				break
			}
			th := things[0]
			if th.next.After(b.now()) {
				if soonest.IsZero() || th.next.Before(soonest) {
					soonest = th.next
				}
				break
			}
			start, state, why, err := b.prepare(o, th)
			if err != nil {
				return nil, time.Time{}, err
			}
			if state != "" {
				if err := b.leave(th, state, why); err != nil {
					return nil, time.Time{}, err
				}
				continue
			}
			if _, err := b.db.Exec(`UPDATE bridges_things SET state = ?, run = '', why = ? WHERE id = ?`, Sending, why, th.id); err != nil {
				return nil, time.Time{}, failed(err)
			}
			b.carried(th)
			starts = append(starts, start)
			break
		}
	}
	return starts, soonest, nil
}

// leave leaves a thing that is not carried now in the state it ends in,
// with why. A sign-in link's text is kept no longer, and the server is
// told how it ended.
func (b *Bridges) leave(th thingRow, state, why string) error {
	if _, err := b.db.Exec(`UPDATE bridges_things SET state = ?, why = ?, run = '', text = CASE WHEN kind = ? THEN '' ELSE text END WHERE id = ?`,
		state, why, ofSignIn, th.id); err != nil {
		return failed(err)
	}
	b.carried(th)
	if th.kind == ofSignIn {
		b.told = append(b.told, SignIn{Thing: th.id, Failed: why})
	}
	return nil
}

// prepare gives the out run that carries a thing, judged now: or the
// state it is to be left in instead, and why.
func (b *Bridges) prepare(o *office.Office, th thingRow) (Start, string, string, error) {
	bridge := o.Bridges[th.bridge]
	switch {
	case marks(th.kind) && (bridge == nil || !bridge.Runs() || !takesWorking(bridge)):
		return Start{}, Withdrawn, fmt.Sprintf("the bridge %s shows nothing now of what a team is doing", th.bridge), nil
	case bridge == nil || !bridge.Runs():
		return Start{}, GivenUp, stopped(th.bridge, bridge), nil
	case th.direct && !takesDirect(bridge):
		return Start{}, GivenUp, fmt.Sprintf("the bridge %s says nothing to anyone directly", th.bridge), nil
	}
	input := map[string]any{
		InputSettings: bridge.Settings, "to": th.place, InputDirect: th.direct,
		"files": []any{}, "answers": nil, "choices": nil, "ref": nil,
	}
	start := Start{Kind: KindOut, Bridge: th.bridge, Thing: th.id, Machine: bridge.Out, Input: input,
		Labels: map[string]string{requests.LabelTeam: office.Top}}
	text, why := th.text, ""
	switch th.kind {
	case ofSignIn:
		start.Kind = KindSignIn
	case ofMark:
		r, err := b.requests.Get(th.request)
		if err != nil {
			return Start{}, "", "", err
		}
		start.Labels = requests.Labels(r)
		input[InputWorking], text = th.text, ""
	case ofEntry:
		still, err := b.stillFor(o, &th)
		if err != nil {
			return Start{}, "", "", err
		}
		if !still {
			return Start{}, Withdrawn, "by the time it was to be carried, it was no longer for whoever reads " + th.place, nil
		}
		e, err := b.requests.Entry(th.entry)
		if err != nil {
			return Start{}, "", "", err
		}
		r, err := b.requests.Get(th.request)
		if err != nil {
			return Start{}, "", "", err
		}
		start.Labels = requests.Labels(r)
		text = e.Text
		files, missing, err := b.filesOf(r, e, th)
		if err != nil {
			return Start{}, "", "", err
		}
		why = missing
		given := []any{}
		for _, f := range files {
			given = append(given, map[string]any{"name": f.Name, "data": client.InputFile{Name: f.Name, Contents: bytes.NewReader(f.Data)}})
		}
		input["files"] = given
		if answers, err := b.answersOf(e, th); err != nil {
			return Start{}, "", "", err
		} else if answers != "" {
			input["answers"] = answers
		}
		var choices []string
		switch {
		case e.Kind == requests.KindQuestion:
			choices = e.Choices
		case b.Choices != nil:
			choices = b.Choices(e)
		}
		if choices != nil || e.Kind == requests.KindQuestion {
			input["choices"], input["ref"] = append([]string{}, choices...), ref(th.id)
		}
	}
	start.Labels[LabelKind], start.Labels[LabelBridge], start.Labels[LabelThing] = start.Kind, th.bridge, strconv.FormatInt(th.id, 10)
	input["text"] = client.InputFile{Name: "text.txt", Contents: strings.NewReader(text)}
	return start, "", why, nil
}

// stillFor says whether an entry carried, or to be, to a place would
// still go there, as the office is read now: to the origin it was for, or
// to the person, at who they are on the bridge now.
func (b *Bridges) stillFor(o *office.Office, th *thingRow) (bool, error) {
	ds, err := b.requests.Deliveries(th.entry)
	if errors.Is(err, requests.ErrNotThere) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, d := range ds {
		switch {
		case th.person != "" && d.Person == th.person:
			p := o.People[th.person]
			if p == nil || !slices.Contains(p.Reached, office.Identity{Bridge: th.bridge, ID: th.place}) {
				return false, nil
			}
			return b.sees(th.request, th.entry, th.person)
		case th.person == "" && d.Origin != nil && d.Origin.Bridge == th.bridge && d.Origin.Place == th.place:
			return true, nil
		}
	}
	return false, nil
}

// sees says whether a person sees an entry of a request, as Requests
// judges it now.
func (b *Bridges) sees(n, entry int64, person string) (bool, error) {
	v, err := b.requests.View(n, person)
	if errors.Is(err, requests.ErrNotThere) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(v.Entries, func(e requests.Entry) bool { return e.ID == entry }), nil
}

// filesOf gives the files of an entry to carry, as someone who sees the
// entry where it goes is given them: the person it is for, or a person
// who reads the origin. Where nobody there is a person of the office, they
// are not carried, and it says so. They are read out of Bulldog's own
// file, never from a service, so reading them holds up nothing for long.
func (b *Bridges) filesOf(r *requests.Request, e requests.Entry, th thingRow) ([]requests.File, string, error) {
	if len(e.Files) == 0 {
		return nil, "", nil
	}
	person := th.person
	if person == "" {
		readers, err := b.requests.ReadBy(r.Number)
		if err != nil {
			return nil, "", err
		}
		for _, p := range readers {
			if p.Kind == requests.IsPerson {
				person = p.Name
				break
			}
		}
	}
	if person == "" {
		return nil, "its files were not carried: nobody who reads it there is a person of the office, and a file is given only to a person who sees it", nil
	}
	var files []requests.File
	for _, f := range e.Files {
		data, err := b.requests.File(r.Number, e.ID, f.Name, person)
		if err != nil {
			return nil, "", err
		}
		files = append(files, requests.File{Name: f.Name, Data: data})
	}
	return files, "", nil
}

// answersOf gives the id of the thing heard on the bridge that an entry
// answers: the entry it answers, when that was heard there. A request's
// answer, carried to its origin, answers what was said there, the latest
// of it before the answer: it is said in reply to that, where the service
// can reply to a message.
func (b *Bridges) answersOf(e requests.Entry, th thingRow) (string, error) {
	var id string
	var err error
	switch {
	case e.Answers != 0:
		return b.heardOf(th.bridge, e.Answers)
	case e.Kind == requests.KindAnswer && th.person == "":
		err = b.db.QueryRow(`SELECT id FROM bridges_heard WHERE bridge = ? AND request = ? AND place = ? AND entry > 0 AND entry < ? ORDER BY entry DESC LIMIT 1`,
			th.bridge, e.Request, th.place, e.ID).Scan(&id)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", failed(err)
	}
	return id, nil
}

// heardOf gives the id of the thing heard on a bridge that an entry was
// taken from, or "".
func (b *Bridges) heardOf(bridge string, entry int64) (string, error) {
	var id string
	err := b.db.QueryRow(`SELECT id FROM bridges_heard WHERE bridge = ? AND entry = ?`, bridge, entry).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", failed(err)
	}
	return id, nil
}

// OutEnded takes how an out run ended, once: the same run's ending handed
// back again changes nothing. A thing is delivered when its run ends with
// sent, whose references are kept to match what answers it; one the service
// refused is given up at once; one whose run failed is carried again by a
// new run, each later than the last, and given up after MaxRuns. A
// sign-in link is carried by one run alone, and the server is told how it
// ended; a mark is carried by one run alone too, so it never holds up
// what comes after it for long.
func (b *Bridges) OutEnded(id int64, run string, end End) error {
	b.mu.Lock()
	defer b.unlock()
	th, err := b.thing(id)
	if err != nil {
		return err
	}
	switch {
	case th == nil:
		return fmt.Errorf("there is no thing %d to be carried", id)
	case run != "" && th.ended == run:
		return nil
	case th.run != run || th.state != Sending:
		return fmt.Errorf("run %s is not the one carrying thing %d", run, id)
	}
	failure := end.Failed
	var sent, refused string
	if failure == "" {
		out, _ := end.Output.(map[string]any)
		sent, refused = references(out["sent"]), text(out["refused"])
		if sent == "" && refused == "" {
			failure = fmt.Sprintf("the out run ended with %s, and an out machine ends with { sent } or { refused }", shape(end.Output))
		}
	}
	switch {
	case sent != "":
		_, err = b.db.Exec(`UPDATE bridges_things SET state = ?, sent = ?, run = '', ended = ? WHERE id = ?`, Sent, sent, run, id)
	case refused != "":
		_, err = b.db.Exec(`UPDATE bridges_things SET state = ?, why = ?, run = '', ended = ? WHERE id = ?`, GivenUp, refused, run, id)
	case th.kind == ofSignIn || marks(th.kind) || th.runs >= MaxRuns:
		_, err = b.db.Exec(`UPDATE bridges_things SET state = ?, why = ?, run = '', ended = ? WHERE id = ?`, GivenUp, failure, run, id)
	default:
		_, err = b.db.Exec(`UPDATE bridges_things SET state = ?, why = ?, run = '', ended = ?, next = ? WHERE id = ?`,
			Waiting, failure, run, stamp(b.now().Add(pause(firstOutPause, th.runs))), id)
	}
	if err != nil {
		return failed(err)
	}
	b.carried(*th)
	if th.kind == ofSignIn {
		b.told = append(b.told, SignIn{Thing: id, Sent: sent != "", Refused: refused, Failed: failure})
	}
	return nil
}

// references gives what an out run sent, as it is kept: the service's
// reference, or each of a list of them, one for each message sent, one
// per line.
func references(v any) string {
	list, ok := v.([]any)
	if !ok {
		return text(v)
	}
	var refs []string
	for _, r := range list {
		if ref := text(r); ref != "" {
			refs = append(refs, ref)
		}
	}
	return strings.Join(refs, "\n")
}

// SignIn keeps a one-time sign-in link to be carried, by one out run, to a
// person's direct place on a bridge. It is no entry of any request. A
// bridge that is stopped, that says nothing to anyone directly, or that
// does not know the person, carries none.
func (b *Bridges) SignIn(bridge, person, text string) (int64, error) {
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return 0, err
	}
	br := o.Bridges[bridge]
	switch {
	case br == nil || !br.Runs():
		return 0, errors.New(stopped(bridge, br))
	case !takesDirect(br):
		return 0, fmt.Errorf("the bridge %s says nothing to anyone directly, so it carries no sign-in link", bridge)
	}
	p := o.People[person]
	if p == nil {
		return 0, fmt.Errorf("%s is not a person of the office", person)
	}
	i := slices.IndexFunc(p.Reached, func(id office.Identity) bool { return id.Bridge == bridge })
	if i < 0 {
		return 0, fmt.Errorf("%s is not reached on %s", person, bridge)
	}
	res, err := b.db.Exec(`INSERT INTO bridges_things(bridge, place, direct, person, kind, text, state, made) VALUES (?, ?, 1, ?, ?, ?, ?, ?)`,
		bridge, p.Reached[i].ID, person, ofSignIn, text, Waiting, stamp(b.now()))
	if err != nil {
		return 0, failed(err)
	}
	return res.LastInsertId()
}

// Carried gives every thing of a request's entries carried, or to be,
// entry by entry, each entry's in the order they were made: what was
// delivered, what is being sent, what waits, and what was given up, with
// why. What goes to the built-in chat, the request's origin when it was
// asked there, or a person on their screens, is delivered as soon as it is
// written, with no run, and is given first, as on the bridge Chat. It
// takes no viewer: each is of one entry, and whoever shows it shows it
// only to someone who sees that entry, since where it went, a person's
// identity on a bridge among them, is as much the entry's as its text.
func (b *Bridges) Carried(request int64) ([]Carried, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	thread, deliveries, err := b.requests.DeliveriesIn(request)
	if err != nil {
		return nil, err
	}
	things, err := b.things(`WHERE request = ? AND kind = ? ORDER BY id`, request, ofEntry)
	if err != nil {
		return nil, err
	}
	byEntry := map[int64][]thingRow{}
	for _, th := range things {
		byEntry[th.entry] = append(byEntry[th.entry], th)
	}
	var out []Carried
	for _, e := range thread {
		for _, d := range deliveries[e.ID] {
			switch {
			case d.Origin != nil && d.Origin.Kind == requests.InChat:
				out = append(out, Carried{Entry: e.ID, Bridge: Chat, State: Sent, Made: e.At})
			case d.Person != "":
				out = append(out, Carried{Entry: e.ID, Bridge: Chat, Place: d.Person, Person: d.Person, State: Sent, Made: e.At})
			}
		}
		for _, th := range byEntry[e.ID] {
			out = append(out, Carried{Thing: th.id, Entry: th.entry, Bridge: th.bridge, Place: th.place, Person: th.person,
				State: th.state, Why: th.why, Runs: th.runs, Sent: th.sent, Made: th.made})
		}
	}
	return out, nil
}
