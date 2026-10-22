package screens

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/store"
)

// The cookies: the key, which signs a browser in, and the browser's own,
// which a sign-in link is signed over.
const (
	keyCookie     = "bulldog-key"
	browserCookie = "bulldog-browser"
)

// linkLife is how long a sign-in link signs in for.
const linkLife = 15 * time.Minute

// sign gives the office secret's signature over parts.
func sign(secret []byte, parts ...string) []byte {
	mac := hmac.New(sha256.New, secret)
	for _, part := range parts {
		mac.Write([]byte(part))
		mac.Write([]byte{0})
	}
	return mac.Sum(nil)
}

// key gives a person's key: the secret's signature over their name and a
// hash of their entry. Changing the entry, or the secret, changes the key.
func key(secret []byte, p *office.Person) string {
	entry, _ := json.Marshal(p)
	sum := sha256.Sum256(entry)
	return hex.EncodeToString(sign(secret, "key", p.ID, hex.EncodeToString(sum[:])))
}

// KeyLink gives the link that signs a person in with their key, at the
// address the screens are reached at.
func KeyLink(address string, secret []byte, o *office.Office, person string) (string, error) {
	p := o.Person(person)
	if p == nil {
		return "", fmt.Errorf("%s is no person of the office: a person is an entry of everyone in office.yaml", person)
	}
	return strings.TrimSuffix(address, "/") + "/key?" + url.Values{"p": {p.ID}, "k": {key(secret, p)}}.Encode(), nil
}

// who gives the person a request is from, by the key its cookie holds, or
// "" for nobody. A key is a person's only while their entry is as it was
// when the key was made.
func (s *Screens) who(r *http.Request) string {
	c, err := r.Cookie(keyCookie)
	if err != nil {
		return ""
	}
	person, given, _ := strings.Cut(c.Value, ".")
	if s.keyOf(person, given) {
		return person
	}
	return ""
}

// keyOf says whether a key given is a person's key, comparing it in
// constant time.
func (s *Screens) keyOf(person, given string) bool {
	p := s.engine.Reading().Office.Person(person)
	return p != nil && hmac.Equal([]byte(given), []byte(key(s.secret, p)))
}

// signIn gives the browser a person's key to keep, and goes on to the
// screens.
func signIn(w http.ResponseWriter, r *http.Request, person, given string) {
	http.SetCookie(w, &http.Cookie{Name: keyCookie, Value: person + "." + given, Path: "/", MaxAge: 400 * 24 * 60 * 60,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// signOut takes the key from the browser.
func signOut(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: keyCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}

// The sign-in pages are plain pages of the screens' own, each a form that
// posts to the screens. A link's page holds only a button, which posts to
// the link's own address.
var pages = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in · {{.Office}}</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<main class="narrow">
<h1>{{.Office}}</h1>
{{if .Said}}<p role="status">{{.Said}}</p>{{end}}
{{if .Button}}<form method="post"><button>{{.Button}}</button></form>{{end}}
{{if .Name}}<form method="post" action="/signin"><label>Your name <input name="name" autocomplete="username" required></label><button>Send me a link</button></form>{{end}}
</main>
</body>
</html>
`))

// show writes a sign-in page.
func (s *Screens) show(w http.ResponseWriter, status int, said, button string, name bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	pages.Execute(w, map[string]any{"Office": s.engine.Reading().Office.Name, "Said": said, "Button": button, "Name": name})
}

func (s *Screens) signInPage(w http.ResponseWriter, r *http.Request) {
	s.show(w, http.StatusOK, "Give your name, and a link that signs you in is sent to you directly, on the first bridge that can message you.", "", true)
}

// sent is what the sign-in page says whatever name it was given.
const sent = "If that is the name of someone Bulldog can message directly, a link is on its way to them. Open it in this browser within 15 minutes."

// askLink sends a sign-in link to the person a name names, signed over a
// cookie of this browser's own. The page answers the same, and at once,
// whatever the name: the link is carried apart.
func (s *Screens) askLink(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	name := strings.TrimSpace(r.PostFormValue("name"))
	browser := ""
	if c, err := r.Cookie(browserCookie); err == nil && len(c.Value) == 32 {
		browser = c.Value
	} else {
		random := make([]byte, 16)
		rand.Read(random)
		browser = hex.EncodeToString(random)
		http.SetCookie(w, &http.Cookie{Name: browserCookie, Value: browser, Path: "/",
			HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	}
	o := s.engine.Reading().Office
	if p := named(o, name); p != nil {
		s.sending.Add(1)
		go func() {
			defer s.sending.Done()
			if err := s.sendLink(context.WithoutCancel(r.Context()), o, p, browser); err != nil {
				s.Report(fmt.Errorf("a sign-in link to %s: %w", p.ID, err))
			}
		}()
	}
	s.show(w, http.StatusOK, sent, "", true)
}

// named gives the person a name names: by the key of their entry, or by
// their name, in any case.
func named(o *office.Office, name string) *office.Person {
	for i := range o.People {
		if p := &o.People[i]; name != "" && (strings.EqualFold(p.ID, name) || strings.EqualFold(p.Name, name)) {
			return p
		}
	}
	return nil
}

// sendLink carries a sign-in link to a person directly, on the first
// bridge that can message them directly. The run that carries it is named
// from the minute and the person, so a person is sent at most one link in
// a minute.
func (s *Screens) sendLink(ctx context.Context, o *office.Office, p *office.Person, browser string) error {
	now := s.corbie.Now()
	for _, b := range o.Bridges {
		to := p.Identities[b.Name]
		if to == "" || !s.bridges.Direct(b.Name) {
			continue
		}
		at := strconv.FormatInt(now.Unix(), 10)
		link := s.address + "/signin/link?" + url.Values{"p": {p.ID}, "t": {at}, "s": {hex.EncodeToString(sign(s.secret, "link", p.ID, at, browser))}}.Encode()
		m := requests.Message{Bridge: b.Name, To: to, Direct: true,
			Text:   fmt.Sprintf("This link signs you in to %s on Bulldog, in the browser you asked from, for 15 minutes:\n%s", o.Name, link),
			Run:    s.corbie.ID.Name + "/" + store.Stamp(now.Truncate(time.Minute)) + "-" + hex.EncodeToString(sign(s.secret, "signin", p.ID)[:8]),
			Labels: map[string]string{"bulldog": o.ID, "kind": "out", "bridge": b.Name}}
		_, err := s.corbie.Show(ctx, m.Run)
		switch {
		case err == nil:
			return nil // a link was sent this minute
		case !errors.Is(err, client.ErrNoRun):
			return err
		}
		return s.bridges.Carry(ctx, m)
	}
	return nil
}

// linkPage is the page a sign-in link opens. Opening it signs nobody in,
// since a service that carries a link opens it to show a preview.
func (s *Screens) linkPage(w http.ResponseWriter, r *http.Request) {
	s.show(w, http.StatusOK, "Sign in, in this browser.", "Sign in", false)
}

// pressLink signs in the person a sign-in link was sent to, when it is
// pressed within 15 minutes in the browser it was asked from.
func (s *Screens) pressLink(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	person, at, signed := q.Get("p"), q.Get("t"), q.Get("s")
	c, err := r.Cookie(browserCookie)
	given, _ := hex.DecodeString(signed)
	unix, _ := strconv.ParseInt(at, 10, 64)
	age := s.corbie.Now().Sub(time.Unix(unix, 0))
	p := s.engine.Reading().Office.Person(person)
	if err != nil || p == nil || age < 0 || age > linkLife || !hmac.Equal(given, sign(s.secret, "link", person, at, c.Value)) {
		s.show(w, http.StatusForbidden, "This link signs nobody in: it is more than 15 minutes old, or it was asked for in another browser. Ask for another.", "", true)
		return
	}
	signIn(w, r, p.ID, key(s.secret, p))
}

// keyPage is the page a key link opens. Opening it signs nobody in.
func (s *Screens) keyPage(w http.ResponseWriter, r *http.Request) {
	s.show(w, http.StatusOK, "Sign in, in this browser, with the key in this link.", "Sign in", false)
}

// pressKey signs in the person a key link is of, while the key is theirs.
func (s *Screens) pressKey(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !s.keyOf(q.Get("p"), q.Get("k")) {
		s.show(w, http.StatusForbidden, "This key signs nobody in: the person's entry has changed since it was made, or the office's secret has. Ask for a link instead.", "", true)
		return
	}
	signIn(w, r, q.Get("p"), q.Get("k"))
}
