package bridges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// stale is how old a thing may be when it is read: one older is let go.
const stale = 24 * time.Hour

// longest is the most of a message Bulldog passes on, in bytes, so a
// request and an answer stay well within what a job's input may be.
const longest = 32 << 10

// A thing is one message, or one press of a button, that an in run hands
// over.
type thing struct {
	ID    string    `json:"id"`
	At    time.Time `json:"at"`
	Kind  string    `json:"kind"` // said or press
	From  string    `json:"from"`
	Place string    `json:"place"`
	Where string    `json:"where"`
	Text  string    `json:"text"`
	Files []struct {
		Name string `json:"name"`
		Data any    `json:"data"`
	} `json:"files"`
	Mentioned bool   `json:"mentioned"`
	Answers   string `json:"answers"`
	Ref       string `json:"ref"`
	Choice    string `json:"choice"`
}

// heard takes each thing of one heard emit of an in run, in order. A
// thing that cannot be read is reported and let go, and so is what a
// bridge the office no longer has heard.
func (b *Bridges) heard(ctx context.Context, id string, data any) error {
	b.mu.Lock()
	bridge := b.runs[id].bridge
	b.mu.Unlock()
	b.taking.Lock()
	defer b.taking.Unlock()
	r := b.Engine.Reading()
	br := r.Office.Bridge(bridge)
	if br == nil {
		return nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var emitted struct {
		Things []json.RawMessage `json:"things"`
	}
	if err := json.Unmarshal(encoded, &emitted); err != nil {
		b.Report(fmt.Errorf("the in run %s of the bridge %s emitted heard as no list of things, and it is let go: %w", id, bridge, err))
		return nil
	}
	for _, raw := range emitted.Things {
		var th thing
		err := json.Unmarshal(raw, &th)
		if err == nil && (th.ID == "" || th.At.IsZero()) {
			err = errors.New("it has no id, or no at")
		}
		if err != nil {
			b.Report(fmt.Errorf("the in run %s of the bridge %s handed over a thing Bulldog cannot read, and it is let go: %w", id, bridge, err))
			continue
		}
		if err := b.take(ctx, r, br, id, th); err != nil {
			return fmt.Errorf("the thing %s: %w", th.ID, err)
		}
	}
	return nil
}

// take acts on one thing an in run handed over:
//
//  1. A press answers the wait its ref names.
//  2. Something said at direct answers the question it replies to, or,
//     replying to nothing, the latest question carried to its sender,
//     while that question waits.
//  3. Something said at a listed place starts a request of its team.
//  4. Anything else is let go, and so is a thing more than a day old.
func (b *Bridges) take(ctx context.Context, r *requests.Reading, br *office.Bridge, in string, th thing) error {
	if b.corbie.Now().Sub(th.At) > stale {
		return nil
	}
	sender := r.Office.PersonOn(br.Name, th.From)
	switch {
	case th.Kind == "press":
		return b.press(ctx, sender, th)
	case th.Kind != "said":
		return nil
	case th.Place == "direct" && sender != nil:
		answer, err := b.typed(ctx, br.Name, sender.ID, th)
		if answer || err != nil {
			return err
		}
	}
	return b.ask(ctx, r, br, in, sender, th)
}

// press answers the wait a press's ref names with the choice pressed, for
// the person whose identity pressed it. A press that gives no choice is
// let go: the person may still type the answer.
func (b *Bridges) press(ctx context.Context, sender *office.Person, th thing) error {
	if sender == nil || th.Ref == "" || th.Choice == "" {
		return nil
	}
	w, err := b.Engine.WaitOf(ctx, th.Ref)
	if err != nil || w == nil {
		return err
	}
	return refused(b.Engine.Answer(ctx, w.ID, sender.ID, th.Choice, th.ID))
}

// typed answers a question with something said at direct, while the
// question waits: the question whose message it replies to, or, when it
// replies to nothing, the latest question carried to its sender on the
// bridge before it was heard. A text that is one of the question's
// choices, trimmed and in any case, answers with that choice; a decision
// takes nothing else. It says whether the thing is an answer: one given
// now, or before, which the wait's answer holds.
func (b *Bridges) typed(ctx context.Context, bridge, person string, th thing) (bool, error) {
	var w *requests.Wait
	var err error
	if th.Answers != "" {
		w, err = b.Engine.Replied(ctx, bridge, person, th.Answers)
	} else {
		w, err = b.Engine.Latest(ctx, bridge, person, th.At)
	}
	switch {
	case err != nil || w == nil:
		return false, err
	case w.Status != "waiting":
		return b.answeredBy(ctx, w.ID, th.ID)
	}
	answer := cut(th.Text)
	if i := slices.IndexFunc(w.Choices, func(c string) bool { return strings.EqualFold(strings.TrimSpace(th.Text), c) }); i >= 0 {
		answer = w.Choices[i]
	} else if w.Decision != "" {
		return false, nil
	}
	return true, refused(b.Engine.Answer(ctx, w.ID, person, answer, th.ID))
}

// answeredBy says whether a wait was answered by a thing.
func (b *Bridges) answeredBy(ctx context.Context, wait, thing string) (bool, error) {
	run, err := b.corbie.Show(ctx, wait)
	if err != nil {
		return false, err
	}
	output, _ := run.Output.(map[string]any)
	return output["thing"] == thing, nil
}

// ask starts a request of a listed place's team with something said
// there: by the person whose entry gives the sender's identity, or by a
// guest where the place lets guests ask. Nothing starts at a place the
// settings do not list, or for a team that takes no requests.
func (b *Bridges) ask(ctx context.Context, r *requests.Reading, br *office.Bridge, in string, sender *office.Person, th thing) error {
	i := slices.IndexFunc(br.Places, func(p office.Place) bool { return p.Name == th.Place })
	if i < 0 {
		return nil
	}
	place := br.Places[i]
	var by requests.By
	switch {
	case sender != nil:
		by.Person = sender.ID
	case place.Guests && th.From != "":
		by.Guest = th.From
	default:
		return nil
	}
	if m := r.Machines[place.Team]; m == nil || m.Draft == nil {
		return nil
	}
	var files []requests.File
	for _, f := range th.Files {
		files = append(files, requests.File{Name: f.Name, Data: carried(f.Data, in)})
	}
	_, err := b.Engine.Ask(ctx, requests.Ask{Team: place.Team, By: by, Said: cut(th.Text), Files: files, At: th.At, Mentioned: th.Mentioned,
		Place: &requests.Place{Bridge: br.Name, Place: place.Name, Guests: place.Guests, Where: th.Where, Thing: th.ID}})
	return err
}

// carried gives a file's data as a request is given it: a blob of the in
// run, as the run showed it, with the run added, so the request carries
// a copy.
func carried(data any, in string) any {
	blob, ok := data.(map[string]any)
	if !ok || blob["$blob"] == nil {
		return data
	}
	blob = maps.Clone(blob)
	blob["run"] = in
	return blob
}

// refused lets go of an answer the wait refuses: from someone it does not
// wait on, after it was answered, or not one of a decision's choices.
func refused(err error) error {
	if errors.Is(err, requests.ErrNoQuestion) || errors.Is(err, requests.ErrNotAChoice) {
		return nil
	}
	return err
}

// cut gives a message whole, or, when it is longer than Bulldog passes on,
// its start and a line saying it was cut.
func cut(text string) string {
	if len(text) <= longest {
		return text
	}
	end := longest
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return fmt.Sprintf("%s\n\n[Bulldog cut this message: it is %d KiB, and Bulldog passes on the first %d KiB.]", text[:end], (len(text)+1023)>>10, longest>>10)
}
