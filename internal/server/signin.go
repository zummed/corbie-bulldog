package server

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// The key's cookie, and how long a browser keeps it.
const (
	Cookie     = "bulldog_key"
	CookieLife = 400 * 24 * time.Hour
)

// Linked is how long a one-time link signs a person in for, and Relinked
// how long a person waits between one link and the next.
const (
	Linked   = 10 * time.Minute
	Relinked = time.Minute
)

// Asked is how many names asked for a link may wait to be looked at.
const Asked = 64

// who gives the person a request is from, by the key its cookie holds, or
// "" for nobody: nobody signs in while the office has never read well, and
// a key is that of a person only while they are in it.
func (s *site) who(r *http.Request) (string, error) {
	c, err := r.Cookie(Cookie)
	if err != nil || c.Value == "" {
		return "", nil
	}
	return s.whose(c.Value)
}

// whose gives the person whose key it is, while they are in the office.
func (s *site) whose(key string) (string, error) {
	o := s.Keeper.Office()
	if o == nil {
		return "", nil
	}
	person, err := whoseKey(s.db, key)
	if err != nil || o.People[person] == nil {
		return "", err
	}
	return person, nil
}

// keep gives a browser a person's key to hold, held to TLS exactly when
// the browser reached the server over it.
func keep(w http.ResponseWriter, r *http.Request, key string) {
	http.SetCookie(w, &http.Cookie{Name: Cookie, Value: key, Path: "/", HttpOnly: true, Secure: overTLS(r),
		SameSite: http.SameSiteStrictMode, MaxAge: int(CookieLife / time.Second)})
}

// overTLS says whether a request came over TLS, directly or through a
// proxy in front of the server that says so.
func overTLS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// home is the screens for a person signed in, the sign-in page for anyone
// else; with a person's key in its address, it gives the browser the key
// and moves on to itself without it, by a page and not a redirect, since a
// strict cookie is not sent on a move another site began.
func (s *site) home(w http.ResponseWriter, r *http.Request) {
	if key := r.URL.Query().Get("key"); key != "" {
		person, err := s.whose(key)
		switch {
		case err != nil:
			s.failed(w, r, err)
		case person == "":
			s.signInPage(w, http.StatusOK, "")
		default:
			keep(w, r, key)
			s.movingOn(w)
		}
		return
	}
	person, err := s.who(r)
	switch {
	case err != nil:
		s.failed(w, r, err)
	case person == "":
		s.signInPage(w, http.StatusOK, "")
	default:
		theScreens(w)
	}
}

// signInPage gives each way to sign in the office has, saying first what
// came of signing in, when anything did: a button for each sign-in method
// that runs; a name and password, when anyone may set a password; and a
// name, to send that person a one-time link. What it shows says nothing
// of who is in the office.
func (s *site) signInPage(w http.ResponseWriter, status int, said string) {
	o := s.Keeper.Office()
	if o == nil {
		s.page(w, status, `<p>The office does not read well yet, so nobody signs in until it does. Its problems are in the server's log.</p>`)
		return
	}
	var b strings.Builder
	if said != "" {
		fmt.Fprintf(&b, "<p class=\"signin-said\" role=\"alert\">%s</p>\n", html.EscapeString(said))
	}
	var methods []string
	for _, name := range o.SignInOrder {
		if m := o.SignIns[name]; s.runs(m) {
			methods = append(methods, fmt.Sprintf(`<a class="signin-with" href="/signin/%s">Sign in with %s</a>`, html.EscapeString(name), html.EscapeString(methodCalled(m))))
		}
	}
	if len(methods) > 0 {
		fmt.Fprintf(&b, "<div class=\"signin-methods\">%s</div>\n", strings.Join(methods, ""))
	}
	if slices.ContainsFunc(o.Everyone, func(name string) bool { return o.People[name].Password }) {
		b.WriteString(`<form id="password"><label>Your name <input name="name" autocomplete="username" required></label><label>Password <input name="password" type="password" autocomplete="current-password" required></label><button>Sign in</button></form>
<p id="password-said" class="signin-said" role="status"></p>
`)
	}
	b.WriteString(`<form id="signin"><label>Your name <input name="name" autocomplete="username" required></label><button>Send me a link</button></form>
<p class="signin-about">A link that signs you in once is sent to you on the first of your bridges that carries it, or open the link the server printed for you.</p>
<p id="said" role="status"></p>
<script src="/static/signin.js"></script>`)
	s.page(w, status, b.String())
}

// movingOn is the page that moves on to the screens once the key is kept.
func (s *site) movingOn(w http.ResponseWriter) {
	s.page(w, http.StatusOK, `<meta http-equiv="refresh" content="0;url=/"><p>You are signed in. <a href="/">Go on to the office.</a></p>`)
}

// page writes a small page of the office's own.
func (s *site) page(w http.ResponseWriter, status int, body string) {
	name := "Bulldog"
	if o := s.Keeper.Office(); o != nil {
		name = o.Name()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>%[1]s</title><link rel="stylesheet" href="/static/style.css">
<main class="narrow"><h1>%[1]s</h1>
%[2]s
</main></html>`, html.EscapeString(name), body)
}

// signedOut is what a link that signs nobody in is answered with.
const signedOut = `<p>This link signs nobody in: it has been used, or is more than ten minutes old. <a href="/">Ask for another.</a></p>`

// asked is what the sign-in page is told whatever name it was given.
const asked = "If that is the name of someone in the office, a link that signs them in once, within ten minutes, is on its way to them on the first of their bridges that carries it."

// askLink is a one-time link asked for by name. It is answered the same
// at once whatever the name, and the name is looked at apart.
func (s *site) askLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name *string `json:"name"`
	}
	if err := readJSON(w, r, &body); err != nil {
		s.refuse(w, r, err)
		return
	}
	if body.Name == nil {
		s.refuse(w, r, unreadable("The body gives no name."))
		return
	}
	select {
	case s.asked <- *body.Name:
	default:
		s.say("so many one-time links are asked for at once that one was not looked at")
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\"said\":%q}\n", asked)
}

// linkPage is the page a one-time link opens, which uses nothing up: a
// service that carries a link opens it to show a preview.
func (s *site) linkPage(w http.ResponseWriter, r *http.Request) {
	s.page(w, http.StatusOK, fmt.Sprintf(`<form method="post" action="/signin/%s"><button>Sign in</button></form>`, html.EscapeString(r.PathValue("link"))))
}

// pressLink signs in the person a one-time link was sent to, as their key
// in an address does, and uses it up.
func (s *site) pressLink(w http.ResponseWriter, r *http.Request) {
	if s.Keeper.Office() == nil {
		s.signInPage(w, http.StatusNotFound, "")
		return
	}
	person, err := useLink(s.db, r.PathValue("link"), s.clock.Now())
	if err != nil {
		s.failed(w, r, err)
		return
	}
	if person == "" || s.Keeper.Office().People[person] == nil {
		s.page(w, http.StatusNotFound, signedOut)
		return
	}
	key, err := keyOf(s.db, person)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	keep(w, r, key)
	s.movingOn(w)
}

// replaceKey gives the one looking a new key, which signs out every
// browser that held the old one, and gives this browser the new one.
func (s *site) replaceKey(person string, w http.ResponseWriter, r *http.Request) (any, error) {
	if err := readNothing(w, r); err != nil {
		return nil, err
	}
	key, err := newKey(s.db, person, s.clock.Now())
	if err != nil {
		return nil, err
	}
	keep(w, r, key)
	return map[string]string{"said": "Your key is replaced: every other browser that held it is signed out."}, nil
}

// signing looks at each name a link was asked for, one after another,
// once every area has taken a reading of the office, until the context
// ends.
func (s *site) signing(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case name := <-s.asked:
			if !s.taken(ctx) {
				return
			}
			if err := s.sendLink(name); err != nil {
				s.say("a one-time link could not be sent: %v", err)
			}
		}
	}
}

// taken stays until every area has taken a reading of the office, as the
// clock tells, and says whether one has before the context ended.
func (s *site) taken(ctx context.Context) bool {
	moved, unwatch := s.clock.Watch()
	defer unwatch()
	for s.Work.Read() == nil {
		select {
		case <-ctx.Done():
			return false
		case <-moved:
		}
	}
	return true
}

// sendLink sends a person of the office a one-time link, at most one in
// Relinked, to their direct place on the first of their bridges that
// carries it, as the office is read by the bridges now. A name that is
// nobody's sends nothing.
func (s *site) sendLink(name string) error {
	o := s.Work.Read()
	if o == nil || o.People[name] == nil {
		return nil
	}
	now := s.clock.Now()
	last, err := lastLink(s.db, name)
	if err != nil || !last.IsZero() && now.Sub(last) < Relinked {
		return err
	}
	return s.carryOn(o, link{person: name, made: now, rest: o.People[name].Reached})
}

// carryOn has a new link of a person carried on the first of the bridges
// still to be tried that takes it: one that is stopped, has no direct
// place, or no longer reaches them as it did when the link was asked for,
// takes none. What a link carries is made for the one bridge that carries
// it, and only its hash is kept.
func (s *site) carryOn(o *office.Office, l link) error {
	p := o.People[l.person]
	if p == nil {
		s.say("%s is no longer in the office, so their one-time link is carried no further", l.person)
		return nil
	}
	for i, id := range l.rest {
		if !slices.Contains(p.Reached, id) {
			s.say("the one-time link of %s is not carried on %s, which no longer reaches them as it did when the link was asked for", l.person, id.Bridge)
			continue
		}
		token := random(32)
		text := fmt.Sprintf("To sign in to %s on its screens, open %s/signin/%s and press Sign in. The link works once, within ten minutes. If you did not ask for it, do nothing.", o.Name(), s.url, token)
		thing, err := s.Bridges.SignIn(id.Bridge, l.person, text)
		if err != nil {
			s.say("the one-time link of %s is not carried on %s: %v", l.person, id.Bridge, err)
			continue
		}
		l.bridge, l.thing, l.rest = id.Bridge, thing, l.rest[i+1:]
		if err := keepLink(s.db, token, l); err != nil {
			return err
		}
		s.Work.Carry()
		return nil
	}
	s.say("no bridge of %s carries their one-time link, so none was sent", l.person)
	return nil
}

// signedIn is told how a link's carrying ended: one that was not sent is
// used up, and a new one carried on the next of the person's bridges as
// their entry named them when it was asked for.
func (s *site) signedIn(si bridges.SignIn) {
	if err := s.carriedLink(si); err != nil {
		s.say("a one-time link's carrying could not be taken: %v", err)
	}
}

func (s *site) carriedLink(si bridges.SignIn) error {
	l, err := linkOf(s.db, si.Thing)
	if err != nil || l == nil || si.Sent {
		return err
	}
	why := si.Refused
	if why == "" {
		why = si.Failed
	}
	s.say("the one-time link of %s was not sent on %s: %s", l.person, l.bridge, why)
	if err := spent(s.db, l.hash, s.clock.Now()); err != nil {
		return err
	}
	o := s.Work.Read()
	if o == nil {
		return nil
	}
	return s.carryOn(o, *l)
}
