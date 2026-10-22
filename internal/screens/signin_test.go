package screens

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

// cookie gives the cookie of a name an answer sets, or nil.
func cookie(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// links gives the sign-in link each message carried.
func links(t *testing.T, sent []requests.Message) []string {
	t.Helper()
	var found []string
	for _, m := range sent {
		if link := regexp.MustCompile(`http://\S+/signin/link\?\S+`).FindString(m.Text); link != "" {
			found = append(found, link)
		}
	}
	return found
}

// askLink asks for a sign-in link in a browser, and stays until it is
// carried, if it is.
func (b *browser) askLink(name string) *http.Response {
	b.w.t.Helper()
	res := b.do("POST", "/signin", form(url.Values{"name": {name}}))
	b.w.screens.sending.Wait()
	return res
}

func TestScreensKeyLinkSignsIn(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	b := w.browser()
	link, err := KeyLink(w.server.URL, secret, w.office, "sam")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, w.server.URL+"/key?") {
		t.Errorf("the key link is %s", link)
	}
	page := b.do("GET", link, nil)
	text, _ := io.ReadAll(page.Body)
	if page.StatusCode != http.StatusOK || !strings.Contains(string(text), "Sign in as Sam, in this browser.") ||
		!strings.Contains(string(text), `<form method="post"><button>Sign in</button></form>`) || cookie(page, keyCookie) != nil {
		t.Errorf("the key link's page is %s, %s", page.Status, text)
	}
	if status, _ := b.api("GET", "/api/me", nil); status != http.StatusUnauthorized {
		t.Errorf("opening the key link signed the browser in: %d", status)
	}
	pressed := b.do("POST", link, nil)
	if pressed.StatusCode != http.StatusSeeOther || pressed.Header.Get("Location") != "/" || cookie(pressed, keyCookie) == nil {
		t.Errorf("pressing the button answered %s, to %s", pressed.Status, pressed.Header.Get("Location"))
	}
	me := b.get("/api/me")
	if jsonOf(me) != `{"office":{"id":"`+w.office.ID+`","name":"The house","zone":"Europe/London"},"person":{"id":"sam","name":"Sam","teams":["office","garden"]}}` {
		t.Errorf("Sam is %s", jsonOf(me))
	}
	if _, err := KeyLink(w.server.URL, secret, w.office, "jo"); err == nil {
		t.Error("a key link is made for nobody")
	}
	if wrong := w.browser().do("POST", strings.Replace(link, "k=", "k=0", 1), nil); wrong.StatusCode != http.StatusForbidden || cookie(wrong, keyCookie) != nil {
		t.Errorf("a wrong key answered %s", wrong.Status)
	}
}

// The office is read again with office.yaml changed, as the program does.
func (w *world) reread(change func(string) string) { w.rewrite("office.yaml", change) }

// The office is read again with one of its files changed.
func (w *world) rewrite(name string, change func(string) string) {
	w.t.Helper()
	data, err := os.ReadFile(filepath.Join(w.office.Dir, name))
	if err != nil {
		w.t.Fatal(err)
	}
	testkit.Write(w.t, w.office.Dir, map[string]string{name: change(string(data))})
	o, err := office.Read(w.office.Dir, shipped.Files)
	if err != nil {
		w.t.Fatal(err)
	}
	w.office = o
	w.engine.Swap(requests.NewReading(o))
}

func TestScreensKeyEndsWithEntry(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	sam, kit := w.as("sam"), w.as("kit")
	w.reread(func(text string) string {
		return strings.Replace(text, `sam: { name: Sam, telegram: "4471" }`, `sam: { name: Sam, telegram: "4479" }`, 1)
	})
	if status, _ := sam.api("GET", "/api/me", nil); status != http.StatusUnauthorized {
		t.Errorf("Sam's key signs in after Sam's entry changed: %d", status)
	}
	if status, _ := kit.api("GET", "/api/me", nil); status != http.StatusOK {
		t.Errorf("Kit's key is refused after Sam's entry changed: %d", status)
	}
	w.reread(func(text string) string {
		text = strings.Replace(text, "  kit: { name: Kit, telegram: \"4472\" }\n", "", 1)
		return strings.Replace(text, "people: [sam, kit]", "people: [sam]", 1)
	})
	if status, _ := kit.api("GET", "/api/me", nil); status != http.StatusUnauthorized {
		t.Errorf("Kit's key signs in after Kit's entry was removed: %d", status)
	}

	// Sam's new key, under a new secret, signs nobody in.
	sam = w.as("sam")
	replaced := New(w.corbie.Client, w.engine, w.bridges, []byte("a new secret"), w.server.URL)
	req := httptest.NewRequest("GET", w.server.URL+"/api/me", nil)
	for _, c := range sam.client.Jar.Cookies(req.URL) {
		req.AddCookie(c)
	}
	answered := httptest.NewRecorder()
	replaced.ServeHTTP(answered, req)
	if answered.Code != http.StatusUnauthorized {
		t.Errorf("Sam's key signs in after the secret was replaced: %d", answered.Code)
	}
}

func TestScreensSignInByLink(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	b := w.browser()
	asked := b.askLink("sam")
	if c := cookie(asked, browserCookie); asked.StatusCode != http.StatusOK || c == nil || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("asking for a link answered %s, with %v", asked.Status, c)
	}
	w.corbie.Until(t, "the link's run has ended", func() bool { return len(w.bridges.carried()) == 1 })
	m := w.bridges.carried()[0]
	if m.Bridge != "telegram" || m.To != "4471" || !m.Direct || m.Labels["kind"] != "out" || m.Labels["bridge"] != "telegram" || m.Labels["bulldog"] != w.office.ID {
		t.Errorf("the link is carried as %+v", m)
	}
	link := links(t, w.bridges.carried())[0]
	w.corbie.Ended(t, m.Run)

	// In another browser, one that asked for nothing or for another link,
	// the link signs nobody in.
	other := w.browser()
	if res := other.do("POST", link, nil); res.StatusCode != http.StatusForbidden || cookie(res, keyCookie) != nil {
		t.Errorf("the link pressed in another browser answered %s", res.Status)
	}
	other.askLink("kit")
	if res := other.do("POST", link, nil); res.StatusCode != http.StatusForbidden || cookie(res, keyCookie) != nil {
		t.Errorf("the link pressed in Kit's browser answered %s", res.Status)
	}

	// In the browser that asked, it signs Sam in.
	pressed := b.do("POST", link, nil)
	if c := cookie(pressed, keyCookie); pressed.StatusCode != http.StatusSeeOther || c == nil || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("the link pressed answered %s", pressed.Status)
	}
	if me := b.get("/api/me"); me["person"].(map[string]any)["id"] != "sam" {
		t.Errorf("the link signed in %v", me)
	}

	// 16 minutes after it was sent, a link signs nobody in.
	late := w.browser()
	w.corbie.Clock.Move(time.Minute)
	late.askLink("sam")
	link = links(t, w.bridges.carried())[2]
	w.corbie.Clock.Move(16 * time.Minute)
	if res := late.do("POST", link, nil); res.StatusCode != http.StatusForbidden || cookie(res, keyCookie) != nil {
		t.Errorf("a link 16 minutes old answered %s", res.Status)
	}
}

func TestScreensLinkSurvivesPreview(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	b := w.browser()
	b.askLink("kit")
	w.corbie.Until(t, "the link is carried", func() bool { return len(w.bridges.carried()) == 1 })
	link := links(t, w.bridges.carried())[0]
	for range 2 {
		page := b.do("GET", link, nil)
		text, _ := io.ReadAll(page.Body)
		if page.StatusCode != http.StatusOK || cookie(page, keyCookie) != nil || strings.Contains(string(text), url.QueryEscape(link)) {
			t.Errorf("opening the link answered %s, %s", page.Status, text)
		}
		if status, _ := b.api("GET", "/api/me", nil); status != http.StatusUnauthorized {
			t.Errorf("opening the link signed the browser in: %d", status)
		}
	}
	if pressed := b.do("POST", link, nil); pressed.StatusCode != http.StatusSeeOther {
		t.Errorf("the link pressed after its previews answered %s", pressed.Status)
	}
	if me := b.get("/api/me"); me["person"].(map[string]any)["id"] != "kit" {
		t.Errorf("the link signed in %v", me)
	}
}

func TestScreensSignInLinksLimited(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	answered := map[string]string{}
	for _, name := range []string{"sam", "Sam", "SAM", "nobody", "", "4471"} {
		res := w.browser().askLink(name)
		text, _ := io.ReadAll(res.Body)
		answered[name] = res.Status + " " + string(text)
	}
	for name, page := range answered {
		if page != answered["sam"] {
			t.Errorf("the page answers %q otherwise than sam:\n%s\n%s", name, page, answered["sam"])
		}
	}
	w.corbie.Until(t, "the link is carried", func() bool { return len(w.bridges.carried()) == 1 })
	if sent := w.bridges.carried(); len(sent) != 1 || sent[0].To != "4471" {
		t.Errorf("asked by name three times in a minute, %d links are carried: %+v", len(sent), sent)
	}
	w.corbie.Clock.Move(time.Minute)
	w.browser().askLink("sam")
	w.corbie.Until(t, "a second link is carried", func() bool { return len(w.bridges.carried()) == 2 })
}

func TestScreensKeyOnlyInCookie(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.House, says, nil)
	link, _ := KeyLink(w.server.URL, secret, w.office, "sam")
	given, _ := url.Parse(link)
	k := given.Query().Get("k")
	b := w.browser()
	page := b.do("GET", link, nil)
	text, _ := io.ReadAll(page.Body)
	if strings.Contains(string(text), k) {
		t.Errorf("the key link's page holds the key: %s", text)
	}
	pressed := b.do("POST", link, nil)
	c := cookie(pressed, keyCookie)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || !strings.HasSuffix(c.Value, "."+k) {
		t.Fatalf("the key is kept as %+v", c)
	}
	for _, path := range []string{"/", "/api/me", "/api/requests", "/api/office", "/api/teams/office"} {
		if _, body := b.body(path); strings.Contains(body, k) {
			t.Errorf("%s holds the key", path)
		}
	}
	if res := b.do("POST", "/signout", nil); cookie(res, keyCookie) == nil || cookie(res, keyCookie).MaxAge >= 0 {
		t.Errorf("signing out answered %s", res.Status)
	}
	if status, _ := b.api("GET", "/api/me", nil); status != http.StatusUnauthorized {
		t.Errorf("signed out, the browser is answered %d", status)
	}

	// Reached at an https:// address, behind whatever ends TLS, the
	// screens' cookies are sent only over https.
	at := New(w.corbie.Client, w.engine, w.bridges, secret, "https://bulldog.example")
	for _, req := range []*http.Request{httptest.NewRequest("POST", strings.TrimPrefix(link, w.server.URL), nil),
		httptest.NewRequest("POST", "/signin", strings.NewReader("name=kit"))} {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		answered := httptest.NewRecorder()
		at.ServeHTTP(answered, req)
		if cookies := answered.Result().Cookies(); len(cookies) != 1 || !cookies[0].Secure {
			t.Errorf("%s at an https address sets %v", req.URL.Path, cookies)
		}
	}
	at.sending.Wait()
}

// A POST or a PUT that a page of another site sends is refused, and so is
// one from a page of a sibling site; one from the screens' own pages, and
// one from outside a browser, is taken.
func TestScreensOtherSitesRefused(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	link, _ := KeyLink(w.server.URL, secret, w.office, "al")
	dana := w.as("dana")
	sent := func(b *browser, method, address, body string, header map[string]string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, w.server.URL+strings.TrimPrefix(address, w.server.URL), strings.NewReader(body))
		for k, v := range header {
			req.Header.Set(k, v)
		}
		res, err := b.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	crossSite := map[string]string{"Origin": "https://elsewhere.example", "Sec-Fetch-Site": "cross-site"}
	if res := sent(w.browser(), "POST", link, "", crossSite); res.StatusCode != http.StatusForbidden || cookie(res, keyCookie) != nil {
		t.Errorf("another site's press on Al's key link answered %s", res.Status)
	}
	if res := sent(dana, "POST", "/signout", "", crossSite); res.StatusCode != http.StatusForbidden || cookie(res, keyCookie) != nil {
		t.Errorf("another site signing Dana out answered %s", res.Status)
	}
	sibling := map[string]string{"Content-Type": "text/plain", "Sec-Fetch-Site": "same-site"}
	if res := sent(dana, "POST", "/api/requests", `{"team":"payments","said":"x = y"}`, sibling); res.StatusCode != http.StatusForbidden {
		t.Errorf("a sibling site asking as Dana answered %s", res.Status)
	}
	if res := sent(dana, "PUT", "/api/teams/support/docs/x.md", `{"text":"x"}`, map[string]string{"Origin": "https://elsewhere.example"}); res.StatusCode != http.StatusForbidden {
		t.Errorf("another site writing a doc as Dana answered %s", res.Status)
	}
	if list := dana.get("/api/requests"); len(list["requests"].([]any)) != 0 {
		t.Errorf("another site asked as Dana: %s", jsonOf(list))
	}
	if res := sent(w.browser(), "POST", link, "", map[string]string{"Origin": w.server.URL, "Sec-Fetch-Site": "same-origin"}); res.StatusCode != http.StatusSeeOther || cookie(res, keyCookie) == nil {
		t.Errorf("the press on Al's key link's own page answered %s", res.Status)
	}
}
