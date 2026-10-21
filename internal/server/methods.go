package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// A sign-in by a method sends the browser to the method's provider with a
// state, a nonce and a challenge, and the provider sends it back with a
// code. Bulldog never speaks to the provider: the method's machine, which
// alone holds the provider's secret, takes the code to it and answers who
// signed in, and Bulldog signs in the one person the office writes with
// that account by the method.

// Signing is how long a sign-in by a method may take, from sending the
// browser to the provider to its coming back; SigningCookie is the cookie
// that binds the sign-in to the browser that began it.
const (
	Signing       = 10 * time.Minute
	SigningCookie = "bulldog_signing"
)

// noAccount is what the sign-in page says when a method's machine
// answered an account the office does not write, the same whatever the
// account.
const noAccount = "No one of the office signs in with that %s account."

// noURL is why every sign-in method signs nobody in on a server started
// with no --url: a provider is given the address it sends the browser
// back to, exactly as it was written there, and the address the server
// listens on is seldom one a provider takes.
const noURL = "the server was started with no --url, which the address its provider sends the browser back to is made with"

// backOf gives the address a method's provider sends the browser back to.
func backOf(base, method string) string { return base + "/signin/" + method + "/back" }

// runs says whether a sign-in method signs anyone in on this server: it
// has no problem, and the server was given its --url.
func (s *site) runs(m *office.SignIn) bool { return m.Runs() && s.urlGiven != "" }

// stoppedSaid is what a browser sent to a method that does not run is
// told, sending it nowhere.
func (s *site) stoppedSaid(m *office.SignIn) string {
	if !m.Runs() {
		return fmt.Sprintf("Signing in with %s is stopped until the office's problems with it are mended.", methodCalled(m))
	}
	return fmt.Sprintf("Signing in with %s is stopped: %s.", methodCalled(m), noURL)
}

// linkOrMethod is a sign-in method's start, at its name, or else the page
// of a one-time link: no method is named as a link's random words are.
func (s *site) linkOrMethod(w http.ResponseWriter, r *http.Request) {
	if o := s.Keeper.Office(); o != nil {
		if m := o.SignIns[r.PathValue("link")]; m != nil {
			s.toProvider(w, r, o, m)
			return
		}
	}
	s.linkPage(w, r)
}

// methodCalled gives what the sign-in page calls a method: its label, or else
// its name.
func methodCalled(m *office.SignIn) string {
	if label := m.Setting("label"); label != "" {
		return label
	}
	return m.Name
}

// toProvider sends the browser to a method's provider, with a state, a
// nonce and a challenge made for this sign-in alone, kept with the hash of
// what the browser's cookie is given, so that only this browser, within
// Signing, comes back with it once.
func (s *site) toProvider(w http.ResponseWriter, r *http.Request, o *office.Office, m *office.SignIn) {
	if !s.runs(m) {
		s.signInPage(w, http.StatusServiceUnavailable, s.stoppedSaid(m))
		return
	}
	sg := signing{state: random(32), method: m.Name, browser: random(32), nonce: random(32), verifier: random(32), made: s.clock.Now()}
	to, err := authorizeOf(m, backOf(s.urlGiven, m.Name), sg)
	if err != nil {
		s.say("signing in with %s: %v", m.Name, err)
		s.signInPage(w, http.StatusServiceUnavailable, fmt.Sprintf("Signing in with %s cannot begin: %v.", methodCalled(m), err))
		return
	}
	if err := keepSigning(s.db, sg); err != nil {
		s.failed(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: SigningCookie, Value: sg.browser, Path: "/signin/", HttpOnly: true, Secure: overTLS(r),
		SameSite: http.SameSiteLaxMode, MaxAge: int(Signing / time.Second)})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// setting is a {name} in a method's authorize, filled from its setting of
// that name.
var setting = regexp.MustCompile(`\{([a-z_]+)\}`)

// authorizeOf gives the provider's address a browser is sent to, built
// from the method's authorize, client_id and scope, as written or as its
// machine's input gives them, each {name} in authorize filled from the
// setting of that name: a request for a code, sent back to back, with the
// state, the nonce and the challenge of the verifier.
func authorizeOf(m *office.SignIn, back string, sg signing) (string, error) {
	filled := setting.ReplaceAllStringFunc(m.Setting("authorize"), func(name string) string {
		return url.PathEscape(m.Setting(name[1 : len(name)-1]))
	})
	u, err := url.Parse(filled)
	if err != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("its authorize, %q, is not an address", filled)
	}
	client := m.Setting("client_id")
	if client == "" {
		return "", errors.New("it has no client_id")
	}
	challenge := sha256.Sum256([]byte(sg.verifier))
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", client)
	q.Set("redirect_uri", back)
	q.Set("scope", m.Setting("scope"))
	q.Set("state", sg.state)
	q.Set("nonce", sg.nonce)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// back takes the browser the provider sent back: its state must be one
// this browser began, within Signing, and it is used up whatever follows.
// A method stopped since signs nobody in. The method's machine is given
// the code, and the one person whose account
// by the method it answers is signed in, as their key in an address does.
func (s *site) back(w http.ResponseWriter, r *http.Request) {
	o := s.Keeper.Office()
	if o == nil {
		s.signInPage(w, http.StatusNotFound, "")
		return
	}
	m := o.SignIns[r.PathValue("method")]
	if m == nil {
		s.signInPage(w, http.StatusNotFound, "")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: SigningCookie, Value: "", Path: "/signin/", HttpOnly: true, Secure: overTLS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	q := r.URL.Query()
	sg, err := takeSigning(s.db, q.Get("state"), s.clock.Now())
	if err != nil {
		s.failed(w, r, err)
		return
	}
	c, _ := r.Cookie(SigningCookie)
	if sg == nil || sg.method != m.Name || c == nil || subtle.ConstantTimeCompare([]byte(hashed(c.Value)), []byte(sg.browser)) != 1 {
		s.signInPage(w, http.StatusBadRequest, fmt.Sprintf("This sign-in with %s was not begun in this browser, is more than ten minutes old, or was used already: begin it again.", methodCalled(m)))
		return
	}
	if !s.runs(m) {
		s.signInPage(w, http.StatusServiceUnavailable, s.stoppedSaid(m))
		return
	}
	if q.Get("code") == "" {
		said := q.Get("error_description")
		if said == "" {
			said = q.Get("error")
		}
		if said == "" {
			said = "it gave no code"
		}
		s.signInPage(w, http.StatusForbidden, fmt.Sprintf("%s did not sign you in: %s.", methodCalled(m), said))
		return
	}
	signed, err := s.Work.SignIn(m, map[string]any{"code": q.Get("code"), "verifier": sg.verifier, "redirect": backOf(s.urlGiven, m.Name), "nonce": sg.nonce})
	switch {
	case errors.Is(err, work.ErrSigningBusy):
		s.signInPage(w, http.StatusServiceUnavailable, fmt.Sprintf("So many are signing in with %s just now that this sign-in is not taken further: begin it again in a moment.", methodCalled(m)))
		return
	case err != nil:
		s.say("signing in with %s: %v", m.Name, err)
		s.signInPage(w, http.StatusServiceUnavailable, fmt.Sprintf("Signing in with %s failed, and the failure is in the server's log.", methodCalled(m)))
		return
	}
	var out work.SignedIn
	select {
	case <-r.Context().Done():
		return
	case out = <-signed:
	}
	switch {
	case out.Failed != "":
		s.say("signing in with %s failed: %s", m.Name, out.Failed)
		s.signInPage(w, http.StatusServiceUnavailable, fmt.Sprintf("Signing in with %s failed, and the failure is in the server's log.", methodCalled(m)))
		return
	case out.Refused != "":
		s.signInPage(w, http.StatusForbidden, out.Refused)
		return
	}
	// The office as it is now names whom the account is.
	o = s.Keeper.Office()
	p := o.Account(m.Name, out.Identity)
	if p == nil {
		s.signInPage(w, http.StatusForbidden, fmt.Sprintf(noAccount, methodCalled(m)))
		return
	}
	key, err := keyOf(s.db, p.Name)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	keep(w, r, key)
	s.movingOn(w)
}

// A signing is a sign-in by a method under way: its state, the method,
// the browser that began it, by what its cookie holds, the nonce and the
// verifier sent with it, and when it began.
type signing struct {
	state, method, browser, nonce, verifier string
	made                                    time.Time
}

// keepSigning keeps a sign-in under way, by the hash of its state and of
// the browser's cookie, and lets go of those older than Signing.
func keepSigning(db *sql.DB, sg signing) error {
	if _, err := db.Exec(`DELETE FROM server_signing WHERE made < ?`, stamp(sg.made.Add(-Signing))); err != nil {
		return fmt.Errorf("the sign-ins under way cannot be let go: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO server_signing (state, method, browser, nonce, verifier, made) VALUES (?, ?, ?, ?, ?, ?)`,
		hashed(sg.state), sg.method, hashed(sg.browser), sg.nonce, sg.verifier, stamp(sg.made)); err != nil {
		return fmt.Errorf("a sign-in with %s cannot be kept: %w", sg.method, err)
	}
	return nil
}

// takeSigning uses up the sign-in under way of a state, and gives it, with
// the browser's cookie by its hash, or nil for one that is not there or is
// older than Signing.
func takeSigning(db *sql.DB, state string, now time.Time) (*signing, error) {
	if state == "" {
		return nil, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("the sign-ins under way cannot be read: %w", err)
	}
	defer tx.Rollback()
	sg := signing{}
	var made string
	switch err := tx.QueryRow(`SELECT method, browser, nonce, verifier, made FROM server_signing WHERE state = ?`, hashed(state)).Scan(&sg.method, &sg.browser, &sg.nonce, &sg.verifier, &made); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("the sign-ins under way cannot be read: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM server_signing WHERE state = ?`, hashed(state)); err != nil {
		return nil, fmt.Errorf("a sign-in under way cannot be used up: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("a sign-in under way cannot be used up: %w", err)
	}
	t, err := time.Parse(stamped, made)
	if err != nil {
		return nil, fmt.Errorf("a sign-in under way is kept with a time that cannot be read, %q: %w", made, err)
	}
	if now.Sub(t) > Signing {
		return nil, nil
	}
	sg.made = t
	return &sg, nil
}
