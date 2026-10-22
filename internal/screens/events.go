package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zummed/corbie/client"
)

// quiet is how long a stream goes with nothing sent before it is sent a
// comment, so that nothing between it and the browser takes it for gone.
const quiet = 25 * time.Second

// events streams to one person a change of each request they may see, as
// it changes, and of the office, when it is read again or a bridge's in
// run changes. Sight is read again for each change, and the stream ends
// when the person's key no longer signs them in.
func (s *Screens) events(w http.ResponseWriter, r *http.Request, me string) {
	ctx := r.Context()
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	write := func(text string) {
		fmt.Fprint(w, text)
		if flusher != nil {
			flusher.Flush()
		}
	}
	send := func(what any) {
		data, _ := json.Marshal(what)
		write("event: changed\ndata: " + string(data) + "\n\n")
	}
	write(": changes\n\n")

	updates := make(chan client.Update)
	go func() {
		err := s.corbie.Watch(ctx, client.Filter{Labels: map[string]string{"bulldog": s.engine.Reading().Office.ID}, Finished: true}, s.Interval,
			func(u client.Update) {
				select {
				case updates <- u:
				case <-ctx.Done():
				}
			})
		if err != nil {
			s.Report(fmt.Errorf("the stream of changes to %s: %w", me, err))
		}
	}()
	look := time.NewTicker(s.Interval)
	defer look.Stop()
	alive := time.NewTicker(quiet)
	defer alive.Stop()
	reading := s.engine.Reading()
	heads := map[string]*client.Run{}
	listed := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-alive.C:
			write(": still here\n\n")
		case <-look.C:
			if rd := s.engine.Reading(); rd != reading {
				reading = rd
				if s.who(r) != me {
					return
				}
				send(map[string]bool{"office": true})
			}
		case u := <-updates:
			if u.Listed {
				listed = true
				continue
			}
			heads[u.Run.ID] = u.Run
			if !listed {
				continue
			}
			what, err := s.changed(ctx, me, u.Run, heads)
			if err != nil {
				s.Report(fmt.Errorf("the stream of changes to %s: %w", me, err))
				continue
			}
			if what != nil {
				send(what)
			}
		}
	}
}

// changed gives what a change of a run is to a person: a change of the
// request it is below, when they may see it; a change of the office, for
// a bridge's in run; or nothing.
func (s *Screens) changed(ctx context.Context, me string, run *client.Run, heads map[string]*client.Run) (any, error) {
	top := run
	for top.Parent != "" {
		parent := heads[top.Parent]
		if parent == nil {
			shown, err := s.corbie.Show(ctx, top.Parent)
			if err != nil {
				return nil, err
			}
			heads[shown.ID], parent = shown, shown
		}
		top = parent
	}
	if top.Labels["kind"] == "bridge" {
		return map[string]bool{"office": true}, nil
	}
	id := top.ID
	if top.Labels["kind"] == "out" {
		id = top.Labels["request"]
	}
	if top.Labels["kind"] != "request" && id == top.ID || id == "" {
		return nil, nil
	}
	q, err := s.engine.Request(ctx, id)
	if err != nil {
		return nil, err
	}
	// A question or a decision put to the person shows them its request.
	how := sightOf(s.engine.Reading().Office, me, q, false)
	if how == unseen && run.Machine == "wait" {
		if how, err = s.sight(ctx, me, q); err != nil {
			return nil, err
		}
	}
	if how == unseen {
		return nil, nil
	}
	return map[string]string{"request": s.id(id)}, nil
}
