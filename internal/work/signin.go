package work

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// A sign-in method's machine is run as any worker is, labelled with its
// kind and the method, and is given what a provider gave a person's
// browser. Nobody watches it, and nothing waits on it but the browser.
const (
	KindSignIn  = "signin"
	LabelSignIn = "signin"
)

// SignIns is the most runs of one sign-in method that go at once: anyone
// may begin a sign-in, and each run of it reaches the provider with the
// client's secret.
const SignIns = 8

// ErrSigningBusy is a sign-in refused because SignIns runs of its method
// go already.
var ErrSigningBusy = errors.New("as many sign-ins by the method go as may at once")

// A signingRun is a sign-in method's run going, by its method, with the
// browser that waits for how it ends.
type signingRun struct {
	method string
	ended  chan SignedIn
}

// A SignedIn is how a sign-in method's run ended: the account its machine
// answered, or the sentence it refused with, or, when it failed, what
// failed.
type SignedIn struct {
	Identity string
	Refused  string
	Failed   string
}

// SignIn starts a sign-in method's machine on what a provider gave a
// person's browser, given as code, verifier, redirect and nonce, with the
// method's settings beside them, and gives a channel sent how it ended,
// once. While SignIns runs of the method go, it starts none, and gives
// ErrSigningBusy.
func (w *Work) SignIn(m *office.SignIn, given map[string]any) (<-chan SignedIn, error) {
	if !m.Runs() {
		return nil, fmt.Errorf("the sign-in method %s has problems, and signs nobody in until they are mended", m.Name)
	}
	name, err := w.corbie.NewRun()
	if err != nil {
		return nil, fmt.Errorf("no name could be chosen for the run of the sign-in method %s: %w", m.Name, err)
	}
	signed := make(chan SignedIn, 1)
	w.mu.Lock()
	going := 0
	for _, run := range w.signing {
		if run.method == m.Name {
			going++
		}
	}
	if going >= SignIns {
		w.mu.Unlock()
		return nil, ErrSigningBusy
	}
	w.signing[name] = signingRun{method: m.Name, ended: signed}
	w.mu.Unlock()
	unsigned := func() {
		w.mu.Lock()
		delete(w.signing, name)
		w.mu.Unlock()
	}
	labels := w.labels(map[string]string{LabelKind: KindSignIn, LabelSignIn: m.Name})
	n := &note{Name: name, Kind: KindSignIn, Worker: m.Name, Labels: labels, Noted: w.clock.Now()}
	if err := w.keep(n); err != nil {
		unsigned()
		return nil, fmt.Errorf("the run of the sign-in method %s could not be noted: %w", m.Name, err)
	}
	input := maps.Clone(given)
	input["settings"] = m.Settings
	if err := w.send(context.Background(), m.Machine, client.Start{Run: name, Input: input, Labels: labels}); err != nil {
		unsigned()
		w.endUnsent(name, err)
		return nil, fmt.Errorf("the sign-in method %s could not be started, as Corbie was not reached: %w", m.Name, err)
	}
	return signed, nil
}

// signedIn sends how a sign-in method's run ended to the browser that
// waits for it, once: what its machine answered, or what failed.
func (w *Work) signedIn(run string, ended *client.Run, e ending) {
	w.mu.Lock()
	signed := w.signing[run].ended
	delete(w.signing, run)
	w.mu.Unlock()
	if signed == nil {
		return
	}
	var out SignedIn
	var said map[string]any
	if ended != nil {
		said, _ = output(ended).(map[string]any)
	}
	identity, _ := said["identity"].(string)
	refused, _ := said["refused"].(string)
	switch {
	case e.status != done:
		out.Failed = e.how
	case identity != "":
		out.Identity = identity
	case refused != "":
		out.Refused = refused
	default:
		out.Failed = "its machine answered neither identity nor refused"
	}
	signed <- out
}
