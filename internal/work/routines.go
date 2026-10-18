package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/shipped"
)

// A routine comes round by a clock of its own, the clock Bulldog ships
// given the routine's time: a chart whose timer is that time, which waits
// on its ticket when the time comes. Bulldog opens the routine's request, notes
// which coming round it was for, and answers the ticket; the clock then
// waits for its next time.

// LabelRoutine names the routine a clock is for, by a fingerprint of what
// is written of it: a routine changed is a new one, with a clock of its
// own.
const LabelRoutine = "routine"

// routineKey gives the fingerprint of a routine as its team writes it.
func routineKey(team string, r office.Routine) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{team, r.Due, r.To, r.Say}, "\n")))
	return hex.EncodeToString(sum[:8])
}

// A written is a routine as the office is read now, with its team.
type written struct {
	team    string
	routine office.Routine
}

// routines gives every routine of the office, by its fingerprint.
func routines(o *office.Office) map[string]written {
	all := map[string]written{}
	for _, team := range o.Order {
		for _, r := range o.Teams[team].Routines {
			all[routineKey(team, r)] = written{team: team, routine: r}
		}
	}
	return all
}

// clocks looks for the routines' clocks by their labels, when the office
// was read again or a clock ended: a routine with none gets one, and a
// clock for a routine no longer written is stopped. What could not be
// read or started is looked at again once Again has passed.
func (w *Work) clocks(ctx context.Context) bool {
	if !w.clockSync || w.held("clocks") {
		return false
	}
	w.clockSync = false
	if err := w.syncClocks(ctx); err != nil {
		w.Log("the routines' clocks: %v; they are looked at again in %v", err, Again)
		w.clockSync = true
		w.again("clocks")
	}
	return true
}

func (w *Work) syncClocks(ctx context.Context) error {
	wanted := routines(w.office)
	runs, err := w.corbie.List(ctx, client.Filter{Labels: w.labels(map[string]string{LabelKind: KindClock})})
	if err != nil {
		return fmt.Errorf("Corbie's clocks could not be listed: %w", err)
	}
	going, err := w.goingNotes()
	if err != nil {
		return err
	}
	// A clock noted as going that Corbie does not list yet is as good as
	// listed: should Corbie never take it up, its end is seen, and the
	// clocks are looked at again.
	names := map[string]bool{}
	for _, r := range runs {
		if r.Parent == "" && !r.HasEnded() {
			names[r.ID] = true
		}
	}
	for _, n := range going {
		if n.Kind == KindClock {
			names[n.Name] = true
		}
	}
	have := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		n, err := w.noted(name)
		if err != nil {
			return err
		}
		if n == nil || n.ended() || n.Stop != "" {
			// One Bulldog has no note of is cancelled as any such run is.
			continue
		}
		key := n.Labels[LabelRoutine]
		if _, ok := wanted[key]; ok && !have[key] {
			have[key] = true
			continue
		}
		if err := w.stopRun(name, "its routine is no longer written"); err != nil {
			return err
		}
		w.cancelling = true
	}
	var failed error
	for _, key := range slices.Sorted(maps.Keys(wanted)) {
		if have[key] {
			continue
		}
		if err := w.startClock(ctx, wanted[key], key); err != nil {
			failed = errors.Join(failed, err)
		}
	}
	return failed
}

// startClock starts a routine's clock, noted before its start is sent.
func (w *Work) startClock(ctx context.Context, wr written, key string) error {
	r := wr.routine
	what := fmt.Sprintf("the clock of the routine at %s of %s", r.At, wr.team)
	contents, _ := shipped.File(shipped.Clock)
	draft, err := machine.Parse(contents, machine.Given(nil))
	if err != nil {
		return fmt.Errorf("%s cannot be made: %w", what, err)
	}
	name, err := w.corbie.NewRun()
	if err != nil {
		return fmt.Errorf("%s has no name for its run: %w", what, err)
	}
	labels := w.labels(map[string]string{LabelKind: KindClock, requests.LabelTeam: wr.team, LabelRoutine: key})
	n := &note{Name: name, Kind: KindClock, Team: wr.team, Worker: r.To, Labels: labels, Noted: w.clock.Now()}
	if err := w.keep(n); err != nil {
		return fmt.Errorf("%s could not be noted: %w", what, err)
	}
	if _, err := w.corbie.Start(ctx, draft, client.Start{Run: name, Input: map[string]any{"due": r.Due}, Labels: labels}); err != nil {
		w.endUnsent(name, err)
		return fmt.Errorf("%s could not be started: %w", what, err)
	}
	return nil
}

// hasCome says whether a run is a clock whose time has come: it waits on
// its ticket.
func hasCome(r *client.Run) bool {
	return r.Labels[LabelKind] == KindClock && slices.Contains(r.Tickets, shipped.ClockTicket)
}

// comings acts on each clock whose time has come, once Bulldog has
// settled: each on its own, and one that cannot be acted on now is looked
// at again once Again has passed.
func (w *Work) comings(ctx context.Context) bool {
	if !w.settled {
		return false
	}
	did := false
	for _, id := range slices.Sorted(maps.Keys(w.came)) {
		if w.held("came:" + id) {
			continue
		}
		delete(w.came, id)
		did = true
		if err := w.come(ctx, id); err != nil {
			w.Log("the time of the clock %s came, and could not be acted on, so it is looked at again in %v: %v", id, Again, err)
			w.came[id] = true
			w.again("came:" + id)
		}
	}
	return did
}

// come acts on a clock waiting on its ticket: a coming round already
// noted is only answered; any other opens the routine's request, or is
// let go by, and is noted, before the ticket is answered.
func (w *Work) come(ctx context.Context, id string) error {
	n, err := w.noted(id)
	if err != nil {
		return err
	}
	if n == nil || n.Kind != KindClock || n.ended() || n.Stop != "" {
		return nil
	}
	tickets, err := w.corbie.Tickets(ctx, id)
	if errors.Is(err, client.ErrNoRun) {
		return nil
	}
	if err != nil {
		return err
	}
	ticket := tickets[shipped.ClockTicket]
	if ticket == "" {
		return nil
	}
	request, kept, err := w.round(id, ticket)
	if err != nil {
		return err
	}
	if !kept {
		if request, err = w.opening(n); err != nil {
			return err
		}
		if err := w.keepRound(id, ticket, request); err != nil {
			return err
		}
	}
	_, err = w.corbie.Reply(ctx, id, ticket, map[string]any{"request": request}, 0)
	return err
}

// opening opens the request a routine's coming round asks, and gives its
// number. A time that comes for a routine no longer written, while the
// routine's last request is still open, or for a worker that cannot run,
// is let go by, and gives 0; it is said in the log, since no request is
// there to say it in.
func (w *Work) opening(n *note) (int64, error) {
	wr, ok := routines(w.office)[n.Labels[LabelRoutine]]
	if !ok {
		w.Log("a clock of %s came for a routine no longer written, so its time is let go by", n.Team)
		return 0, nil
	}
	r := wr.routine
	open, err := w.routineOpen(wr)
	if err != nil {
		return 0, err
	}
	if open != 0 {
		w.Log("the routine at %s of %s came while its last request, %d, is still open, so its time is let go by", r.At, wr.team, open)
		return 0, nil
	}
	if worker := w.office.Workers[r.To]; worker == nil || !worker.Runs() {
		w.Log("the routine at %s of %s came, and its time is let go by: %s", r.At, wr.team, problem(w.office, r.To))
		return 0, nil
	}
	asked, err := w.requests.AskByRoutine(wr.team, r, requests.Who{})
	var refusal requests.Refusal
	if errors.As(err, &refusal) {
		w.Log("the routine at %s of %s came, and its time is let go by: %s", r.At, wr.team, refusal)
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return asked.Number, nil
}

// routineOpen gives the routine's request still open, or 0: one opened for
// a coming round Bulldog stopped before noting is found so too, and not
// opened again.
func (w *Work) routineOpen(wr written) (int64, error) {
	open, err := w.requests.OpenOf(wr.team)
	if err != nil {
		return 0, err
	}
	for _, s := range open {
		if s.Asker.Kind != requests.IsTeam {
			continue
		}
		r, err := w.requests.Get(s.Number)
		if err != nil {
			return 0, err
		}
		o := r.Origin
		if r.Parent == 0 && o.Kind == requests.ByRoutine && o.Team == wr.team && o.At == wr.routine.At && o.To == wr.routine.To && o.Say == wr.routine.Say {
			return r.Number, nil
		}
	}
	return 0, nil
}
