package work

import (
	"context"
	"fmt"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// reread takes a new good reading of the office into every area, in the
// order each needs the one before: the requests, then the decisions, then
// the bridges. Then the runs of workers and teams no longer in it are
// stopped, every open request is looked at again, the routines' clocks
// are looked for and the bridges asked for their runs. An area that
// cannot take the reading is given it again later, and the ones after it
// wait for it. It says whether anything was read.
func (w *Work) reread(ctx context.Context) bool {
	did := false
	if o := w.keeper.Office(); o != nil && o != w.office && !w.held("read") {
		if err := w.requests.Read(o); err != nil {
			w.Log("the requests could not take the office as it was read again, so it is tried again in %v: %v", Again, err)
			w.again("read")
			return false
		}
		w.office = o
		w.scan = true
		w.clockSync, w.listenNow, w.carryNow = true, true, true
		w.sweep(o)
		did = true
	}
	if w.office == nil {
		return did
	}
	for _, area := range []struct {
		name string
		read func() error
	}{
		{"decisions", func() error { return w.decide.Read(ctx, w.office) }},
		{"bridges", func() error { return w.bridges.Read(w.office) }},
	} {
		if w.read[area.name] == w.office {
			continue
		}
		if w.held("read:" + area.name) {
			break
		}
		if err := area.read(); err != nil {
			w.Log("the %s could not take the office as it was read again, so it is tried again in %v: %v", area.name, Again, err)
			w.again("read:" + area.name)
			break
		}
		w.read[area.name] = w.office
		w.listenNow, w.carryNow = true, true
		did = true
	}
	if w.read["decisions"] == w.office && w.read["bridges"] == w.office && w.taken.Load() != w.office {
		w.taken.Store(w.office)
		if w.Took != nil {
			w.Took()
		}
	}
	return did
}

// sweep stops the runs on requests whose worker or team is no longer in
// the office as it is read now, a heed's whose team is not.
func (w *Work) sweep(o *office.Office) {
	notes, err := w.goingNotes()
	if err != nil {
		w.Log("the runs going could not be read, to stop those of workers no longer in the office, so they are read again in %v: %v", Again, err)
		w.again("sweep")
		return
	}
	for _, n := range notes {
		if n.Kind != KindWork && n.Kind != KindHeed || n.Stop != "" {
			continue
		}
		why := ""
		switch worker := o.Workers[n.Worker]; {
		case o.Teams[n.Team] == nil:
			// The request was answered by the requests, saying the team
			// has gone.
			why = stopped
		case n.Kind == KindWork && (worker == nil || worker.Team != n.Team):
			why = fmt.Sprintf("%s is no longer a worker of %s, so its run was stopped", n.Worker, inSentence(o.Teams[n.Team].Called))
		}
		if why == "" {
			continue
		}
		if err := w.stopRun(n.Name, why); err != nil {
			w.trouble(n.Request, n.Team, fmt.Sprintf("Bulldog could not stop the run of %s, which is no longer in the office, and tries again in %v: %v",
				n.Worker, Again, err))
			w.again("sweep")
			continue
		}
		w.cancelling = true
	}
}

// problem gives what stops a worker from running, in words.
func problem(o *office.Office, worker string) string {
	w := o.Workers[worker]
	if w == nil {
		return worker + " is not in the office"
	}
	var lines []string
	for _, p := range o.Problems.OfWorker(worker) {
		lines = append(lines, p.String())
	}
	if len(lines) == 0 {
		return worker + " cannot run"
	}
	return worker + " cannot run until its file is mended: " + strings.Join(lines, "; ")
}

// inSentence gives what a team is called as it is written inside a
// sentence: "The house" as "the house".
func inSentence(called string) string {
	if rest, ok := strings.CutPrefix(called, "The "); ok {
		return "the " + rest
	}
	return called
}
