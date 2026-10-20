package work

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// read_thread gives a run more of what it is given, and nothing it could
// not be: its request's thread as a run of it is given it, from an entry
// on, or what was said before its request where it was asked, of the
// requests every reader of it reads.

// entryLabels are the words a thread's entries are read by, for their kinds;
// Bulldog's own entries have none.
var entryLabels = map[string]string{
	requests.KindAsked:     "Asked",
	requests.KindSaid:      "Said",
	requests.KindQuestion:  "Question",
	requests.KindTold:      "Told",
	requests.KindReplied:   "Reply",
	requests.KindSub:       "Sub-request",
	requests.KindAnswer:    "Answer",
	requests.KindLate:      "Late answer",
	requests.KindHeld:      "Held",
	decisions.KindDecision: "Decision",
	decisions.KindDecide:   "Asked to decide",
	decisions.KindDecided:  "Verdict",
	decisions.KindReferred: "Referred",
	decisions.KindNotice:   "Notice",
}

// readThread gives the thread of the call's request, oldest first, from
// an entry on; or, with place, what was said before it where it was
// asked, newest first, from an entry back. Each gives as many whole
// entries as fit in ThreadLimit characters, and how to read on.
func (w *Work) readThread(c *calling) (string, bool, error) {
	from, _ := c.args["from"].(float64)
	if place, _ := c.args["place"].(bool); place {
		return w.readPlace(c, int64(from))
	}
	thread, err := w.requests.Thread(c.r.Number)
	if err != nil {
		return "", false, err
	}
	var entries []requests.Entry
	for _, e := range threadGiven(thread) {
		if e.ID >= int64(from) {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return fmt.Sprintf("The thread of request %d holds nothing from #%d on.", c.r.Number, int64(from)), false, nil
	}
	text, shown := threadText(w.office, entries, false)
	if left := len(entries) - shown; left > 0 {
		last := entries[shown-1].ID
		text += fmt.Sprintf("%d more after #%d; read on from #%d.", left, last, last+1)
	}
	return text, false, nil
}

// readPlace gives what was said before the call's request at the listed
// place it was asked at, in its team's other requests there that every
// reader of it reads, newest first, from an entry back. A request asked
// at no place has nothing said before it there.
func (w *Work) readPlace(c *calling, from int64) (string, bool, error) {
	r := c.r
	nowhere := ""
	switch {
	case r.Parent != 0:
		nowhere = fmt.Sprintf("Request %d is a request from another team, asked at no place", r.Number)
	case r.Origin.Kind == requests.ByRoutine:
		nowhere = fmt.Sprintf("Request %d was asked by a routine, at no place", r.Number)
	case r.Origin.Kind != requests.AtPlace:
		nowhere = fmt.Sprintf("Request %d was asked %s, at no place", r.Number, originText(r.Origin))
	}
	if nowhere != "" {
		return nowhere + ", so nothing was said before it there: read its own thread, without place.", false, nil
	}
	said, err := w.requests.SaidBefore(r.Number)
	if err != nil {
		return "", false, err
	}
	var entries []requests.Entry
	for _, e := range said {
		if from == 0 || e.ID <= from {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return fmt.Sprintf("Nothing that every reader of request %d may read was said before it on %s, in %s.", r.Number, r.Origin.Bridge, r.Origin.Heard),
			false, nil
	}
	text, shown := threadText(w.office, entries, true)
	if left := len(entries) - shown; left > 0 {
		last := entries[shown-1].ID
		text += fmt.Sprintf("%d more before #%d; read on from #%d.", left, last, last-1)
	}
	return text, false, nil
}

// threadText writes entries one to a line, each with its number, as many
// whole as fit in ThreadLimit characters; with request, each names its
// request. It gives how many it wrote.
func threadText(o *office.Office, entries []requests.Entry, request bool) (string, int) {
	var lines []string
	for _, e := range entries {
		lines = append(lines, threadLine(o, e, request))
	}
	return fit(lines, ThreadLimit)
}

// fit writes lines, one to a line, as many whole as fit in a limit of
// characters, a first longer than that cut. It gives how many it wrote.
func fit(lines []string, limit int) (string, int) {
	var b strings.Builder
	room := limit
	for i, line := range lines {
		line += "\n"
		size := utf8.RuneCountInString(line)
		if size > room {
			if i > 0 {
				return b.String(), i
			}
			line = cut(line, room-1) + "\n"
		}
		b.WriteString(line)
		room -= size
	}
	return b.String(), len(lines)
}

// threadLine gives an entry as read_thread gives it: its number, its
// request's with request, when, by whom and to whom, its label, and
// whether its team's heed noted it.
func threadLine(o *office.Office, e requests.Entry, request bool) string {
	head := fmt.Sprintf("#%d ", e.ID)
	if request {
		head += fmt.Sprintf("in request %d ", e.Request)
	}
	if label := entryLabels[e.Kind]; label != "" {
		return head + entryLine(o, e, label)
	}
	return head + entryLine(o, e)
}
