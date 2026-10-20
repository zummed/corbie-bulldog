package bridges

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// The kinds of thing an in run hears, as its kind says: something said, a
// choice on what was sent, or a status at the source.
const (
	HeardSaid   = "said"
	HeardChoice = "choice"
	HeardStatus = "status"
)

// ErrNotHearing is a delivery to a name that is no bridge of the office
// that hears: nothing is started for it.
var ErrNotHearing = errors.New("there is no bridge here that hears")

// A Delivery is what a service sent to a bridge's address, unread: its
// method, its headers and its body.
type Delivery struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// An Answer is what the service that called a bridge's address is
// answered, as the in run said.
type Answer struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

// Taken is what came of an in run: the answer its service is given, and
// the requests a thing closed at its source stopped, whose runs are to be
// cancelled.
type Taken struct {
	Answer  *Answer
	Stopped []int64
}

// Listen gives the listening runs to start now, one for each bridge that
// listens and runs and has none going, and when to look again for one
// whose pause after failing is not over.
func (b *Bridges) Listen() ([]Start, time.Time, error) {
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return nil, time.Time{}, err
	}
	var starts []Start
	var soonest time.Time
	for _, name := range o.BridgeOrder {
		bridge := o.Bridges[name]
		if !bridge.Runs() || !bridge.Listens {
			continue
		}
		row, err := b.bridgeRow(name)
		if err != nil {
			return nil, time.Time{}, err
		}
		if row.run != "" || row.claimed {
			continue
		}
		if row.next.After(b.now()) {
			if soonest.IsZero() || row.next.Before(soonest) {
				soonest = row.next
			}
			continue
		}
		if _, err := b.db.Exec(`INSERT INTO bridges_bridges(bridge, claimed) VALUES (?, 1)
			ON CONFLICT(bridge) DO UPDATE SET claimed = 1`, name); err != nil {
			return nil, time.Time{}, failed(err)
		}
		starts = append(starts, inRun(name, bridge, InputCursor, row.cursor))
	}
	return starts, soonest, nil
}

// Hear gives the run to start for a delivery to a bridge's address: of its
// in machine, given the delivery as it came. A name that is no bridge of
// the office that hears and runs starts nothing.
func (b *Bridges) Hear(name string, d Delivery) (Start, error) {
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return Start{}, err
	}
	bridge := o.Bridges[name]
	if bridge == nil || !bridge.Runs() || !bridge.Hears {
		return Start{}, ErrNotHearing
	}
	return inRun(name, bridge, InputDelivery, d), nil
}

// inRun gives a run of a bridge's in machine, given its settings and a
// cursor or a delivery. It is of the top team, whose bridges they are.
func inRun(name string, bridge *office.Bridge, input string, value any) Start {
	return Start{
		Kind: KindIn, Bridge: name, Machine: bridge.In,
		Input:  map[string]any{InputSettings: bridge.Settings, input: value},
		Labels: map[string]string{LabelKind: KindIn, requests.LabelTeam: office.Top, LabelBridge: name},
	}
}

// Started keeps the name a run was given, before it is sent to be
// started: a listening run is then the bridge's one, and an out run the
// one its thing is being carried by. A sign-in link, which is carried by
// that one run alone, is kept no longer once the run is given it.
func (b *Bridges) Started(s Start, run string) error {
	b.mu.Lock()
	defer b.unlock()
	var err error
	_, listening := s.Input[InputCursor]
	switch {
	case s.Kind == KindIn && listening:
		_, err = b.db.Exec(`UPDATE bridges_bridges SET run = ?, claimed = 1 WHERE bridge = ?`, run, s.Bridge)
	case s.Kind == KindOut || s.Kind == KindSignIn:
		_, err = b.db.Exec(`UPDATE bridges_things SET run = ?, runs = runs + 1, text = CASE WHEN kind = ? THEN '' ELSE text END WHERE id = ?`, run, ofSignIn, s.Thing)
	}
	if err != nil {
		return failed(err)
	}
	return nil
}

// A bridgeRow is what is kept of a bridge's runs.
type bridgeRow struct {
	cursor   any
	run      string
	claimed  bool
	failures int
	next     time.Time
	trouble  string
	since    time.Time
}

func (b *Bridges) bridgeRow(name string) (bridgeRow, error) {
	var row bridgeRow
	var cursor, next, since string
	err := b.db.QueryRow(`SELECT cursor, run, claimed, failures, next, trouble, since FROM bridges_bridges WHERE bridge = ?`, name).
		Scan(&cursor, &row.run, &row.claimed, &row.failures, &next, &row.trouble, &since)
	if errors.Is(err, sql.ErrNoRows) {
		return row, nil
	}
	if err != nil {
		return row, failed(err)
	}
	if err := json.Unmarshal([]byte(cursor), &row.cursor); err != nil {
		return row, failed(err)
	}
	row.next, row.since = unstamp(next), unstamp(since)
	return row, nil
}

// InEnded takes what an in run heard, once it has ended: each thing once,
// by its id, what a listening run handed over as it went first, and for a
// listening run the cursor it ended with, kept only after everything heard
// with it is taken. What each thing said as a blob of the run is read out
// of it first, before this area's work, since a slow read holds up no
// other bridge. A thing that cannot be read is not taken, and is the
// bridge's trouble; one that could not be taken for a fault of Bulldog's
// keeps the cursor where it was, so the next run hears it again. A run
// that failed, that ended with no list of things, or, when it listens,
// with no cursor to go on from, is the bridge's trouble, its cursor is
// kept as it was, and it is started again after a pause that grows with
// each failure in a row; what it handed over before it failed is taken
// all the same, since the service was told it was received.
func (b *Bridges) InEnded(name, run string, end End) (Taken, error) {
	out := readHanded(end.Handed)
	if end.Failed == "" {
		output := readHeard(end.Output)
		out.things = append(out.things, output.things...)
		out.problems = append(out.problems, output.problems...)
		out.cursor, out.answer, out.fault = output.cursor, output.answer, output.fault
	}
	readAll(out.things, end.Fetch)
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return Taken{}, err
	}
	row, err := b.bridgeRow(name)
	if err != nil {
		return Taken{}, err
	}
	// A listening run that could not be started ends with no name.
	listening := row.run == run && (run != "" || row.claimed)
	taken := Taken{Answer: out.answer}
	var faults []string
	problems := out.problems
	switch {
	case end.Failed != "":
		faults = append(faults, end.Failed)
	case out.fault != "":
		faults = append(faults, out.fault)
	case listening && out.cursor == nil:
		faults = append(faults, "the in run ended with no cursor to go on from, so the one before is kept")
	}
	if o.Bridges[name] == nil {
		faults = append(faults, fmt.Sprintf("the bridge %s is not in the office, so what it heard was not taken", name))
	} else {
		var notTaken []string
		taken.Stopped, notTaken, problems = b.takeAll(o, name, out.things, problems)
		faults = append(faults, notTaken...)
	}
	if listening && len(faults) == 0 {
		kept, err := json.Marshal(out.cursor)
		if err != nil {
			return taken, fmt.Errorf("the cursor %s heard with cannot be kept: %w", name, err)
		}
		if _, err := b.db.Exec(`UPDATE bridges_bridges SET cursor = ?, run = '', claimed = 0, failures = 0, next = '' WHERE bridge = ?`, string(kept), name); err != nil {
			return taken, failed(err)
		}
	}
	if listening && len(faults) > 0 {
		next := b.now().Add(pause(firstInPause, row.failures+1))
		if _, err := b.db.Exec(`UPDATE bridges_bridges SET run = '', claimed = 0, failures = failures + 1, next = ? WHERE bridge = ?`, stamp(next), name); err != nil {
			return taken, failed(err)
		}
	}
	if all := append(faults, problems...); len(all) > 0 {
		return taken, b.trouble(name, strings.Join(all, "; "))
	}
	if _, err := b.db.Exec(`UPDATE bridges_bridges SET trouble = '', since = '' WHERE bridge = ?`, name); err != nil {
		return taken, failed(err)
	}
	return taken, nil
}

// Handed takes what a bridge's listening run hands over while it goes on,
// each thing once, by its id, as InEnded takes it; the cursor and the run
// are kept as they are. A thing that cannot be taken is the bridge's
// trouble, and is looked at again when the run ends, since everything the
// run handed over is handed back then.
func (b *Bridges) Handed(name string, end End) (Taken, error) {
	out := readHanded(end.Handed)
	readAll(out.things, end.Fetch)
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil || o.Bridges[name] == nil {
		return Taken{}, err
	}
	var taken Taken
	var faults, problems []string
	taken.Stopped, faults, problems = b.takeAll(o, name, out.things, out.problems)
	if all := append(faults, problems...); len(all) > 0 {
		return taken, b.trouble(name, strings.Join(all, "; "))
	}
	return taken, nil
}

// Listening gives the run a bridge listens by while one goes, and "" when
// none does.
func (b *Bridges) Listening(name string) (string, error) {
	b.mu.Lock()
	defer b.unlock()
	row, err := b.bridgeRow(name)
	return row.run, err
}

// takeAll takes each thing heard, and gives the requests a close at the
// source stopped, the faults of Bulldog's that kept a thing from being
// taken, and the problems given with what each thing's taking found.
func (b *Bridges) takeAll(o *office.Office, name string, things []thing, problems []string) ([]int64, []string, []string) {
	var stopped []int64
	var faults []string
	for _, t := range things {
		s, problem, err := b.take(o, name, t)
		if err != nil {
			faults = append(faults, fmt.Sprintf("thing %s could not be taken: %v", t.ID, err))
			continue
		}
		if problem != "" {
			problems = append(problems, problem)
		}
		stopped = append(stopped, s...)
	}
	return stopped, faults, problems
}

// trouble keeps what went wrong with a bridge's run, to be shown until a
// run of it ends well.
func (b *Bridges) trouble(name, what string) error {
	_, err := b.db.Exec(`INSERT INTO bridges_bridges(bridge, trouble, since) VALUES (?, ?, ?)
		ON CONFLICT(bridge) DO UPDATE SET trouble = excluded.trouble,
			since = CASE WHEN bridges_bridges.since = '' THEN excluded.since ELSE bridges_bridges.since END`,
		name, what, stamp(b.now()))
	if err != nil {
		return failed(err)
	}
	return nil
}

// Troubles gives the bridges whose runs are failing, in the office's
// order: what the latest said, since when, and how many in a row.
func (b *Bridges) Troubles() ([]Trouble, error) {
	b.mu.Lock()
	defer b.unlock()
	o, err := b.office()
	if err != nil {
		return nil, err
	}
	var troubles []Trouble
	for _, name := range o.BridgeOrder {
		row, err := b.bridgeRow(name)
		if err != nil {
			return nil, err
		}
		if row.trouble != "" {
			troubles = append(troubles, Trouble{Bridge: name, Error: row.trouble, Since: row.since, Failures: row.failures})
		}
	}
	return troubles, nil
}

// A thing is one thing an in run heard, as it gave it.
type thing struct {
	ID      string
	Kind    string
	From    string
	HeardIn string
	Place   string
	// Threadless says its place holds no threads, as a chat does: what
	// went before is said more in only by a reply.
	Threadless bool
	// Mentioned says the service named the bridge's own bot in it.
	Mentioned bool
	Readers   []string
	Link      string
	Answers   string
	Text      string
	// TextBlob is where the text is, when it was given as a blob.
	TextBlob string
	Files    []any
	Choice   string
	Status   string
	Closed   bool
	// got are its files, and unread why its text or files could not be
	// read out of the run.
	got    []requests.File
	unread error
}

// What an in run ended with: the things it heard, each with what it must
// have; the cursor to go on from; what to answer the service; what was
// wrong with one thing, which is not taken; and what was wrong with the
// whole, which takes nothing.
type ran struct {
	things   []thing
	cursor   any
	answer   *Answer
	problems []string
	fault    string
}

// readHeard reads an in run's output. A thing without its id, its kind or
// who it is from is not taken, and is said among the problems; an output
// that is not { heard, cursor, answer } is a fault.
func readHeard(output any) ran {
	out, ok := output.(map[string]any)
	if !ok {
		return ran{fault: fmt.Sprintf("the in run ended with %s, and an in machine ends with { heard, cursor }", shape(output))}
	}
	list, ok := out["heard"].([]any)
	if out["heard"] != nil && !ok {
		return ran{fault: fmt.Sprintf("heard is %s, and is a list of things", shape(out["heard"]))}
	}
	r := readThings(list)
	r.cursor, r.answer = out["cursor"], readAnswer(out["answer"])
	return r
}

// readHanded reads what a listening run handed over as it went, each a
// list of things. One that is not a list is a problem, and takes nothing.
func readHanded(handed []any) ran {
	var r ran
	for _, value := range handed {
		list, ok := value.([]any)
		if !ok {
			r.problems = append(r.problems, fmt.Sprintf("the in run handed over %s as %s, and hands over a list of things", shape(value), EmitHeard))
			continue
		}
		got := readThings(list)
		r.things = append(r.things, got.things...)
		r.problems = append(r.problems, got.problems...)
	}
	return r
}

// readAll reads out of the run what each thing said as a blob.
func readAll(things []thing, fetch func(string) ([]byte, error)) {
	for i := range things {
		things[i].unread = readBlobs(&things[i], fetch)
	}
}

// readThings reads a list of things heard. A thing without its id, its
// kind or who it is from is not taken, and is said among the problems.
func readThings(list []any) ran {
	var r ran
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			r.problems = append(r.problems, fmt.Sprintf("thing %d heard is %s, and each is a mapping", i+1, shape(item)))
			continue
		}
		t := thing{
			ID: text(m["id"]), Kind: text(m["kind"]), From: text(m["from"]), HeardIn: text(m["heard_in"]), Place: text(m["place"]),
			Answers: text(m["answers"]), Text: text(m["text"]), Choice: text(m["choice"]), Status: text(m["status"]),
		}
		if blob, ok := m["text"].(map[string]any); ok {
			t.TextBlob = text(blob["$blob"])
		}
		t.Closed, _ = m["closed"].(bool)
		t.Threadless, _ = m["threadless"].(bool)
		t.Mentioned, _ = m["mentioned"].(bool)
		for _, r := range asList(m["readers"]) {
			if id := text(r); id != "" {
				t.Readers = append(t.Readers, id)
			}
		}
		t.Files = asList(m["files"])
		switch about := m["about"].(type) {
		case string:
			t.Link = about
		case map[string]any:
			t.Link = text(about["link"])
		}
		var missing []string
		for field, value := range map[string]string{"id": t.ID, "kind": t.Kind, "from": t.From} {
			if value == "" {
				missing = append(missing, field)
			}
		}
		slices.Sort(missing)
		switch {
		case len(missing) > 0:
			r.problems = append(r.problems, fmt.Sprintf("thing %d heard has no %s, so it was not taken", i+1, strings.Join(missing, " and no ")))
		case t.Kind != HeardSaid && t.Kind != HeardChoice && t.Kind != HeardStatus:
			r.problems = append(r.problems, fmt.Sprintf("thing %s is of the kind %q, and a thing is said, choice or status", t.ID, t.Kind))
		default:
			if t.Place == "" && t.HeardIn == office.Direct {
				t.Place = t.From
			} else if t.Place == "" {
				t.Place = t.HeardIn
			}
			r.things = append(r.things, t)
		}
	}
	return r
}

// readAnswer reads what the in run says to answer the service: a status,
// headers and a body, which is sent as it is when it is text and as JSON
// when it is not.
func readAnswer(v any) *Answer {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	a := &Answer{Status: http.StatusOK, Headers: map[string]string{}}
	if status, ok := m["status"].(float64); ok && status >= 100 && status < 600 {
		a.Status = int(status)
	}
	if headers, ok := m["headers"].(map[string]any); ok {
		for name, value := range headers {
			a.Headers[name] = text(value)
		}
	}
	switch body := m["body"].(type) {
	case nil:
	case string:
		a.Body = []byte(body)
	default:
		a.Body, _ = json.Marshal(body)
		if _, ok := a.Headers["Content-Type"]; !ok {
			a.Headers["Content-Type"] = "application/json"
		}
	}
	return a
}

// readBlobs reads out of the run what a thing said: its text, when it was
// given as a blob, and its files.
func readBlobs(t *thing, fetch func(string) ([]byte, error)) error {
	if t.Kind != HeardSaid {
		return nil
	}
	if t.TextBlob != "" {
		if fetch == nil {
			return errors.New("its text is a blob of the run, which cannot be read")
		}
		data, err := fetch(t.TextBlob)
		if err != nil {
			return fmt.Errorf("its text cannot be read out of the run: %w", err)
		}
		t.Text = string(data)
	}
	var err error
	t.got, err = readFiles(t.Files, fetch)
	return err
}

// A handing is what came of handing a thing on: the request and the entry
// it was taken into, the requests a close at the source stopped, and a
// refusal that is the bridge's trouble, which the office's people read.
type handing struct {
	request int64
	entry   int64
	stopped []int64
	problem string
}

// take takes one thing heard, once: the first time its id is heard it is
// kept as heard, then handed on as its kind says, and each later time it
// is passed over. A thing whose handing on failed before anything was
// written of it is kept as heard no longer, so it is taken when it is
// heard again; once something is written of it, it is not taken again,
// and what failed after is the bridge's trouble.
func (b *Bridges) take(o *office.Office, bridge string, t thing) ([]int64, string, error) {
	if t.unread != nil {
		return nil, "", t.unread
	}
	res, err := b.db.Exec(`INSERT INTO bridges_heard(bridge, id, place, at) VALUES (?, ?, ?, ?) ON CONFLICT(bridge, id) DO NOTHING`,
		bridge, t.ID, t.Place, stamp(b.now()))
	if err != nil {
		return nil, "", failed(err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, "", failed(err)
	} else if n == 0 {
		return nil, "", nil
	}
	var h handing
	switch t.Kind {
	case HeardSaid:
		h, err = b.said(o, bridge, t)
	case HeardChoice:
		h, err = b.chose(o, bridge, t)
	case HeardStatus:
		h, err = b.status(o, bridge, t)
	}
	if err != nil && h.request == 0 {
		if _, gone := b.db.Exec(`DELETE FROM bridges_heard WHERE bridge = ? AND id = ?`, bridge, t.ID); gone != nil {
			return nil, "", errors.Join(err, failed(gone))
		}
		return nil, "", err
	}
	if _, err := b.db.Exec(`UPDATE bridges_heard SET request = ?, entry = ? WHERE bridge = ? AND id = ?`, h.request, h.entry, bridge, t.ID); err != nil {
		return h.stopped, "", failed(err)
	}
	if err != nil {
		return h.stopped, fmt.Sprintf("thing %s was taken into request %d, but then: %v", t.ID, h.request, err), nil
	}
	return h.stopped, h.problem, nil
}

// The sentences carried back to someone whose thing is not taken. Each is
// written for a person, and holds nothing from inside the office.
const (
	notYours      = "That was sent to someone else, so nothing you said was taken."
	notWaiting    = "That is no longer waiting on you, so nothing you said was taken."
	mayNotSay     = "You may not say more there, so nothing you said was taken."
	cannotTake    = "Bulldog could not take that just now; the office's people have been told."
	noSuchThing   = "That choice is on nothing that waits, so it was not taken."
	notHere       = "Nothing is taken from there, so your choice was not taken."
	notFromGuests = "Choices are taken there only from people of the office, so yours was not taken."
)

// said takes something said, as an entry in the request whose sent thing
// it answers, when its sender may still answer it; else in the request of
// the thing heard it answers, when they may say more in it; else where it
// goes on from what went before at its place (goesOn); else, heard in a
// listed place, as a new request to the team the place names.
// A guest's is taken only from a listed place that lets guests ask; from
// anywhere else nothing is written, the readers it gives included, and
// nothing is sent back. The readers a thing heard in a listed place gives
// are kept before its sender is judged, since they say who reads the
// place now, its sender among them. A thing answers a sent thing only at
// the place it was sent to, and one sent to someone directly only from
// them.
func (b *Bridges) said(o *office.Office, bridge string, t thing) (handing, error) {
	who := requests.Identity(bridge, t.From)
	person := o.Person(bridge, t.From)
	in := requests.Where{Bridge: bridge, Place: t.Place}
	p, listed := o.Place(bridge, t.HeardIn)
	listed = listed && t.HeardIn != ""
	if person == nil && !(listed && p.Guests) {
		return handing{}, nil
	}
	if listed {
		if err := b.requests.Readers(bridge, t.Place, t.Readers); err != nil {
			return handing{}, err
		}
	}
	if t.Answers != "" {
		sent, err := b.sentThing(bridge, t.Answers)
		if err != nil {
			return handing{}, err
		}
		switch {
		case sent == nil || sent.entry == 0 || sent.place != t.Place:
		case sent.direct && t.From != sent.place:
			return handing{}, b.sentence(bridge, t, notYours)
		case sent.person != "":
			still, err := b.stillFor(o, sent)
			if err != nil {
				return handing{}, err
			}
			if !still {
				return handing{}, b.sentence(bridge, t, notWaiting)
			}
			return b.answer(sent, who, person, t, in)
		case listed:
			may, err := b.maySay(sent.request, who, in)
			if err != nil {
				return handing{}, err
			}
			if may {
				return b.answer(sent, who, person, t, in)
			}
		}
	}
	if !listed {
		return handing{}, nil
	}
	if t.Answers != "" {
		n, err := b.heardRequest(bridge, t.Answers, t.Place)
		if err != nil {
			return handing{}, err
		}
		if n != 0 {
			may, err := b.maySay(n, who, in)
			if err != nil {
				return handing{}, err
			}
			if may {
				return b.say(n, who, t, in)
			}
		}
	}
	if h, ok, err := b.goesOn(o, bridge, t, who, in); err != nil || ok {
		return h, err
	}
	r, err := b.requests.AskAt(bridge, t.HeardIn, t.Place, t.Link, t.From, t.Text, t.got, t.Mentioned)
	if isRefusal(err) {
		return handing{problem: fmt.Sprintf("what %s said in %s was not asked: %v", t.From, t.HeardIn, err)}, b.sentence(bridge, t, cannotTake)
	}
	if err != nil {
		return handing{}, err
	}
	thread, err := b.requests.Thread(r.Number)
	if err != nil {
		return handing{request: r.Number}, err
	}
	return handing{request: r.Number, entry: thread[0].ID}, nil
}

// heardRequest gives the request a thing heard at a place was taken into,
// by its id, or 0.
func (b *Bridges) heardRequest(bridge, id, place string) (int64, error) {
	var n int64
	err := b.db.QueryRow(`SELECT request FROM bridges_heard WHERE bridge = ? AND id = ? AND place = ?`, bridge, id, place).Scan(&n)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, failed(err)
	}
	return n, nil
}

// goesOn takes something said that answers nothing in particular where it
// goes on from what went before at its place, and says whether it was
// taken. At a place that holds threads, it is said in the latest request
// whose origin is the place, when its sender may say more in it, and
// answers the question put there that waits on them, if one does. A
// threadless place holds every conversation its people have there, so
// there it only answers the latest question carried there, while that
// still waits on its sender.
func (b *Bridges) goesOn(o *office.Office, bridge string, t thing, who requests.Who, in requests.Where) (handing, bool, error) {
	if t.Threadless {
		sent, err := b.latestQuestion(`WHERE bridge = ? AND place = ? AND kind = ? AND state = ? ORDER BY id DESC`, bridge, t.Place, ofEntry, Sent)
		if err != nil || sent == nil {
			return handing{}, false, err
		}
		if sent.person != "" {
			still, err := b.stillFor(o, sent)
			if err != nil || !still {
				return handing{}, false, err
			}
		}
		return b.reply(sent, who, t, in)
	}
	n, ok, err := b.requests.LatestAt(bridge, t.Place)
	if err != nil || !ok {
		return handing{}, false, err
	}
	may, err := b.maySay(n, who, in)
	if err != nil || !may {
		return handing{}, false, err
	}
	sent, err := b.latestQuestion(`WHERE request = ? AND bridge = ? AND place = ? AND kind = ? AND state = ? ORDER BY id DESC`, n, bridge, t.Place, ofEntry, Sent)
	if err != nil {
		return handing{}, false, err
	}
	if sent != nil {
		if h, ok, err := b.reply(sent, who, t, in); err != nil || ok {
			return h, ok, err
		}
	}
	h, err := b.say(n, who, t, in)
	return h, true, err
}

// reply answers the question a thing carried put, and says whether it was
// taken: an answer from someone it does not wait on, or to a question
// that waits no longer, is not.
func (b *Bridges) reply(sent *thingRow, who requests.Who, t thing, in requests.Where) (handing, bool, error) {
	e, err := b.requests.Reply(sent.entry, who, t.Text, t.got, in)
	switch {
	case err == nil:
		return handing{request: sent.request, entry: e.ID}, true, nil
	case isRefusal(err) || errors.Is(err, requests.ErrNotThere):
		return handing{}, false, nil
	}
	return handing{}, false, err
}

// answer takes what someone said in answer to a thing that was carried to
// them: words on a decision go to the decision; an answer to a question
// still put to them answers it; anything else is said in the request.
func (b *Bridges) answer(sent *thingRow, who requests.Who, person *office.Person, t thing, in requests.Where) (handing, error) {
	e, err := b.requests.Entry(sent.entry)
	if err != nil {
		return handing{}, err
	}
	if b.Choices != nil && b.Choices(e) != nil {
		return b.choose(t, Choice{Bridge: in.Bridge, Who: who, Person: nameOf(person), Request: sent.request, Entry: sent.entry, Text: t.Text, In: in})
	}
	if e.Kind == requests.KindQuestion {
		reply, err := b.requests.Reply(sent.entry, who, t.Text, t.got, in)
		if err == nil {
			return handing{request: sent.request, entry: reply.ID}, nil
		}
		if !isRefusal(err) && !errors.Is(err, requests.ErrNotThere) {
			return handing{}, err
		}
	}
	if may, err := b.maySay(sent.request, who, in); err != nil {
		return handing{}, err
	} else if !may {
		return handing{}, b.sentence(in.Bridge, t, mayNotSay)
	}
	return b.say(sent.request, who, t, in)
}

// say writes what someone said in a request they may say more in.
func (b *Bridges) say(n int64, who requests.Who, t thing, in requests.Where) (handing, error) {
	e, err := b.requests.Say(n, who, t.Text, t.got, in, t.Mentioned)
	switch {
	case e != nil:
		return handing{request: n, entry: e.ID}, err
	case isRefusal(err) || errors.Is(err, requests.ErrNotThere):
		return handing{}, b.sentence(in.Bridge, t, mayNotSay)
	}
	return handing{}, err
}

// maySay says whether someone may say more in a request, as Requests
// judges it now; one who sees nothing of it may not.
func (b *Bridges) maySay(n int64, who requests.Who, in requests.Where) (bool, error) {
	may, err := b.requests.MaySay(n, who, in)
	if errors.Is(err, requests.ErrNotThere) || isRefusal(err) {
		return false, nil
	}
	return may, err
}

// latestQuestion gives the first question among the things carried that
// where picks out, for words said that answer no message in particular.
func (b *Bridges) latestQuestion(where string, args ...any) (*thingRow, error) {
	things, err := b.things(where, args...)
	if err != nil {
		return nil, err
	}
	for _, th := range things {
		e, err := b.requests.Entry(th.entry)
		if err != nil {
			return nil, err
		}
		if e.Kind == requests.KindQuestion {
			return &th, nil
		}
	}
	return nil, nil
}

// chose gives a choice to the decision whose carried thing it names, with
// the identity of whoever made it, and carries back the sentence the
// decision gives. A choice is taken from a listed place, or on what was
// sent to its maker directly; one made anywhere else is not given to the
// decision. A guest's is taken only from a listed place that lets guests
// ask, whatever it is on, and from anywhere else nothing is written, the
// readers it gives included. Every choice heard is answered.
func (b *Bridges) chose(o *office.Office, bridge string, t thing) (handing, error) {
	sent, err := b.refThing(bridge, t.Answers)
	if err != nil {
		return handing{}, err
	}
	if sent == nil {
		if sent, err = b.sentThing(bridge, t.Answers); err != nil {
			return handing{}, err
		}
	}
	person := o.Person(bridge, t.From)
	p, listed := o.Place(bridge, t.HeardIn)
	listed = listed && t.HeardIn != ""
	switch {
	case person == nil && !(listed && p.Guests):
		return handing{}, b.sentence(bridge, t, notFromGuests)
	case sent == nil || sent.entry == 0 || sent.place != t.Place:
		return handing{}, b.sentence(bridge, t, noSuchThing)
	case sent.direct && t.From != sent.place:
		return handing{}, b.sentence(bridge, t, notYours)
	case !sent.direct && !listed:
		return handing{}, b.sentence(bridge, t, notHere)
	}
	if listed {
		if err := b.requests.Readers(bridge, t.Place, t.Readers); err != nil {
			return handing{}, err
		}
	}
	who := requests.Identity(bridge, t.From)
	return b.choose(t, Choice{Bridge: bridge, Who: who, Person: nameOf(person), Request: sent.request, Entry: sent.entry, Choice: t.Choice,
		In: requests.Where{Bridge: bridge, Place: t.Place}})
}

// choose hands a choice, or words on a decision, to Decisions and carries
// back the sentence it gives. One Decisions could not take is answered
// that it could not, and is heard again.
func (b *Bridges) choose(t thing, c Choice) (handing, error) {
	if b.Choose == nil {
		return handing{}, errors.Join(b.sentence(c.Bridge, t, cannotTake), errors.New("nothing takes a choice: decisions are not set up"))
	}
	answer, err := b.Choose(c)
	if err != nil {
		return handing{}, errors.Join(b.sentence(c.Bridge, t, cannotTake), err)
	}
	return handing{request: c.Request}, b.sentence(c.Bridge, t, answer)
}

// status hands a thing's status at its source to the request whose origin
// it is, with who changed it, when, and a link; one for a thing that began
// no request makes nothing. It keeps the readers it gives of a listed
// place, and gives the requests a close stopped.
func (b *Bridges) status(o *office.Office, bridge string, t thing) (handing, error) {
	if _, listed := o.Place(bridge, t.HeardIn); listed && t.HeardIn != "" {
		if err := b.requests.Readers(bridge, t.Place, t.Readers); err != nil {
			return handing{}, err
		}
	}
	stopped, err := b.requests.Source(bridge, t.Place, requests.Status{
		Status: t.Status, Closed: t.Closed, By: requests.Identity(bridge, t.From), At: b.now(), Link: t.Link,
	})
	return handing{stopped: stopped}, err
}

// readFiles reads the files a thing said: each a name and its contents,
// given in the output as base64 or as a blob of the run's, which is read
// out of the run now.
func readFiles(list []any, fetch func(string) ([]byte, error)) ([]requests.File, error) {
	var files []requests.File
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("file %d is %s, and a file is { name, data }", i+1, shape(item))
		}
		f := requests.File{Name: text(m["name"])}
		switch data := m["data"].(type) {
		case string:
			bytes, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return nil, fmt.Errorf("file %s is not base64: %v", f.Name, err)
			}
			f.Data = bytes
		case map[string]any:
			path := text(data["$blob"])
			if path == "" || fetch == nil {
				return nil, fmt.Errorf("file %s is %s, which cannot be read", f.Name, shape(data))
			}
			bytes, err := fetch(path)
			if err != nil {
				return nil, fmt.Errorf("file %s cannot be read out of the run: %w", f.Name, err)
			}
			f.Data = bytes
		default:
			return nil, fmt.Errorf("file %s has %s as its data, and data is base64 or a blob", f.Name, shape(m["data"]))
		}
		files = append(files, f)
	}
	return files, nil
}

// isRefusal says whether an error is a refusal of Requests', in a sentence
// for whoever tried, and nothing written.
func isRefusal(err error) bool {
	var r requests.Refusal
	return errors.As(err, &r)
}

func nameOf(p *office.Person) string {
	if p == nil {
		return ""
	}
	return p.Name
}

// text gives a value of a run's output as text: text as it is, a number
// as it is written, and nothing for anything else.
func text(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	}
	return ""
}

func asList(v any) []any {
	list, _ := v.([]any)
	return list
}

// shape says what a value of a run's output is, for a problem.
func shape(v any) string {
	switch v := v.(type) {
	case nil:
		return "nothing"
	case string:
		return fmt.Sprintf("the text %q", v)
	case []any:
		return "a list"
	case map[string]any:
		return "a mapping of " + strings.Join(slices.Sorted(maps.Keys(v)), ", ")
	}
	return fmt.Sprintf("%v", v)
}
