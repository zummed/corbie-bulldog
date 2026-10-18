package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"slices"
	"time"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/server"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// Patience is how long the opening waits for any one step to settle
// before it says it did not: a stand-in model's step takes moments, and
// what stopped one is said while whoever watches is still watching.
const Patience = 30 * time.Second

// The day the demo opens with, as Sam and Kit ask it.
const (
	Socket = "the kitchen socket sparked"
	Fan    = "the bathroom fan has stopped"
	Lawn   = "is the lawn ok after the rain?"
)

// openDay plays the first part of the house's day, as Sam and Kit would on
// the screens, each step once the one before it has settled: Kit asks
// about the socket, with a photo of it, and Sam says yes to cutting its
// circuit, which leaves it waiting on Kit to say the new socket came; Sam
// asks about the fan, which waits on his yes to an order the house does
// not make alone; Kit asks after the lawn, which the garden answers; and
// Sam runs the bins routine now. Nothing is asked until every area has
// taken its reading of the office, as the clock tells.
func openDay(ctx context.Context, a server.Areas, clock work.Clock) error {
	if err := read(ctx, a, clock); err != nil {
		return err
	}
	pic, err := photo()
	if err != nil {
		return err
	}
	socket, err := a.Requests.AskInChat("kit", office.Top, Socket, []requests.File{{Name: "socket.png", Data: pic}})
	if err != nil {
		return err
	}
	cut, err := askedOfSam(ctx, a, socket.Number, "cut_power")
	if err != nil {
		return err
	}
	reply, err := a.Decisions.Decide(ctx, cut.ID, a.Requests.Person("sam"), decisions.SaysYes, "")
	if err != nil {
		return err
	}
	if !reply.Taken {
		return fmt.Errorf("Sam's yes to cutting the circuit was not taken: %s", reply.Text)
	}
	if err := settle(ctx, a, socket.Number, "waiting on Kit", func(st requests.State) bool {
		return st.Is == requests.Waiting && slices.ContainsFunc(st.On, func(p requests.Party) bool { return p.Name == "kit" })
	}); err != nil {
		return err
	}

	fan, err := a.Requests.AskInChat("sam", office.Top, Fan, nil)
	if err != nil {
		return err
	}
	if _, err := askedOfSam(ctx, a, fan.Number, "order_part"); err != nil {
		return err
	}

	lawn, err := a.Requests.AskInChat("kit", office.Top, Lawn, nil)
	if err != nil {
		return err
	}
	if err := settle(ctx, a, lawn.Number, "answered", answered); err != nil {
		return err
	}

	o := a.Work.Read()
	at := slices.IndexFunc(o.Teams[office.Top].Routines, func(r office.Routine) bool { return r.To == "bins" })
	if at < 0 {
		return errors.New("the house has no routine said to bins")
	}
	bins, err := a.Requests.AskByRoutine(office.Top, o.Teams[office.Top].Routines[at], a.Requests.Person("sam"))
	if err != nil {
		return err
	}
	return settle(ctx, a, bins.Number, "answered", answered)
}

// read stays until every area has taken its reading of the office,
// looking again each time the clock moves.
func read(ctx context.Context, a server.Areas, clock work.Clock) error {
	moved, unwatch := clock.Watch()
	defer unwatch()
	limit := time.NewTimer(Patience)
	defer limit.Stop()
	for a.Work.Read() == nil {
		select {
		case <-moved:
		case <-limit.C:
			return fmt.Errorf("the office was not read within %v", Patience)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func answered(st requests.State) bool { return st.Is == requests.Answered }

// askedOfSam stays until a request's held job waits on Sam's decision, and
// gives the decision.
func askedOfSam(ctx context.Context, a server.Areas, n int64, job string) (*decisions.Decision, error) {
	var asked *decisions.Decision
	var failed error
	err := settle(ctx, a, n, "waiting on Sam's yes to "+job, func(requests.State) bool {
		all, err := a.Decisions.OfRequest(n)
		if err != nil {
			failed = err
			return true
		}
		for _, d := range all {
			if d.Job != nil && d.Job.Name == job && d.Ended == "" && slices.Contains(d.Asked, "sam") {
				asked = d
			}
		}
		return asked != nil
	})
	return asked, errors.Join(err, failed)
}

// settle stays until a request stands as wanted, looking again each time a
// request changes. A request answered when that is not what was wanted
// has gone wrong, and so has one that is not as wanted within Patience.
func settle(ctx context.Context, a server.Areas, n int64, what string, wanted func(requests.State) bool) error {
	changed := make(chan struct{}, 1)
	defer a.Requests.Watch(func(int64) {
		select {
		case changed <- struct{}{}:
		default:
		}
	})()
	limit := time.NewTimer(Patience)
	defer limit.Stop()
	for {
		st, err := a.Requests.State(n)
		if err != nil {
			return err
		}
		if wanted(st) {
			return nil
		}
		if st.Is == requests.Answered {
			return fmt.Errorf("request %d was answered before it was %s%s", n, what, lastSaid(a, n))
		}
		select {
		case <-changed:
		case <-limit.C:
			return fmt.Errorf("request %d was not %s within %v%s", n, what, Patience, lastSaid(a, n))
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// lastSaid gives the last thing said in a request, to say what went wrong.
func lastSaid(a server.Areas, n int64) string {
	thread, err := a.Requests.Thread(n)
	if err != nil || len(thread) == 0 {
		return ""
	}
	return ": its last entry says " + fmt.Sprintf("%q", thread[len(thread)-1].Text)
}

// photo gives a small photo of the socket, as a PNG: a faceplate with its
// two pins, scorched between them.
func photo() ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			c := color.NRGBA{240, 236, 228, 255}
			switch {
			case (x == 5 || x == 10) && y >= 5 && y < 10:
				c = color.NRGBA{40, 36, 32, 255}
			case x > 5 && x < 10 && y >= 6 && y < 9:
				c = color.NRGBA{120, 90, 60, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	err := png.Encode(&b, img)
	return b.Bytes(), err
}
