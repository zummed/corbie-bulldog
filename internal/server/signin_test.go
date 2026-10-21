package server

import (
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// holding gives a browser holding the key a reply gave it.
func (s *serving) holding(r reply) *browser {
	s.b.t.Helper()
	c := r.cookie()
	if c == nil {
		s.b.t.Fatalf("the reply gave no key: %d %s", r.status, r.body)
	}
	return &browser{s: s, key: c.Value, header: http.Header{}}
}

// whoIs gives whose browser it is, as /api/me says, or "" for nobody's.
func (br *browser) whoIs() string {
	br.s.b.t.Helper()
	r := br.get("/api/me")
	if r.status == http.StatusUnauthorized {
		return ""
	}
	return r.is(http.StatusOK).json()["me"].(map[string]any)["name"].(string)
}

func TestServerKeyGivesItsCookieAndMovesOn(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	key := s.as("sam").key
	r := s.nobody().get("/?key=" + key).is(http.StatusOK)
	if c := r.cookie(); c == nil || c.Value != key {
		t.Fatalf("the cookie given is %v", c)
	}
	if !strings.Contains(string(r.body), `http-equiv="refresh" content="0;url=/"`) {
		t.Errorf("the page does not move on to /: %s", r.body)
	}
	if who := s.holding(r).whoIs(); who != "sam" {
		t.Errorf("the cookie is %q's", who)
	}
	wrong := s.nobody().get("/?key=wrong").is(http.StatusOK)
	if wrong.cookie() != nil || !strings.Contains(string(wrong.body), `id="signin"`) {
		t.Errorf("a wrong key gave %v and %s", wrong.cookie(), wrong.body)
	}
}

func TestServerKeyOnlyInCookie(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	key := s.as("sam").key
	for _, path := range []string{"/api/me?key=" + key, "/api/requests?key=" + key, "/static/app.js?key=" + key + "&x=1"} {
		r := s.nobody().get(path)
		if strings.HasPrefix(path, "/api/") && r.status != http.StatusUnauthorized {
			t.Errorf("%s with the key in the address answered %d", path, r.status)
		}
		if r.cookie() != nil {
			t.Errorf("%s gave a cookie", path)
		}
	}
	// A key in an address of anything else signs nobody in.
	if r := s.nobody().do(http.MethodPost, "/api/requests?key="+key, "application/json", []byte(`{"text":"hello"}`)); r.status != http.StatusUnauthorized {
		t.Errorf("asking with the key in the address answered %d", r.status)
	}
}

func TestServerSignInMovesOnByPage(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	for _, r := range []reply{s.nobody().get("/?key=" + s.as("kit").key), s.press(s.linkFor("sam"))} {
		if r.status >= 300 && r.status < 400 || r.header.Get("Location") != "" {
			t.Errorf("a sign-in answered %d, to %q", r.status, r.header.Get("Location"))
		}
		if r.status != http.StatusOK || r.cookie() == nil || !strings.Contains(string(r.body), `content="0;url=/"`) {
			t.Errorf("a sign-in answered %d %s", r.status, r.body)
		}
	}
}

func TestServerCookieSecureWhenTLS(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	key := s.as("sam").key
	plain := s.nobody().get("/?key=" + key).cookie()
	proxied := s.nobody()
	proxied.header.Set("X-Forwarded-Proto", "https")
	secure := proxied.get("/?key=" + key).cookie()
	for _, c := range []*http.Cookie{plain, secure} {
		if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.MaxAge != 400*24*3600 || c.Path != "/" {
			t.Fatalf("the cookie is %+v", c)
		}
	}
	if plain.Secure || !secure.Secure {
		t.Errorf("over a plain connection the cookie is secure %v; through a proxy over TLS %v", plain.Secure, secure.Secure)
	}
	// What says it came over TLS is the proxy's header and nothing else.
	r := httptestRequest("http://house.example/")
	if overTLS(r) {
		t.Error("a request with no header is taken as over TLS")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !overTLS(r) {
		t.Error("a request a proxy says came over TLS is not taken so")
	}
}

func httptestRequest(url string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, url, nil)
	return r
}

func TestServerReplacedKeySignsOutOtherBrowsers(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	one, two := s.as("sam"), s.as("sam")
	printed := one.key
	r := one.post("/api/me/key", "").is(http.StatusOK)
	one = s.holding(r)
	if one.key == printed || one.whoIs() != "sam" {
		t.Fatalf("the new key is %q, Sam's %v", one.key, one.whoIs())
	}
	if got := two.get("/api/me"); got.status != http.StatusUnauthorized {
		t.Errorf("the other browser is answered %d", got.status)
	}
	if got := s.nobody().get("/?key=" + printed); got.cookie() != nil {
		t.Error("the printed link still signs Sam in")
	}
}

// pigeon and board are stand-in bridges of the test's: pigeon says things
// to a person directly, and board says nothing to anyone directly.
func withMoreBridges(files map[string]string, sam string) map[string]string {
	files = maps.Clone(files)
	maps.Copy(files, standin.In("pigeon", standin.Folder(false, true)))
	maps.Copy(files, standin.In("board", standin.Folder(false, false)))
	files["office.yaml"] = strings.Replace(files["office.yaml"], "bridges:\n", "bridges:\n  pigeon: { places: { direct: {} } }\n  board: { places: {} }\n", 1)
	files["office.yaml"] = strings.Replace(files["office.yaml"], `sam: { name: Sam, title: Owner. Decides money and the building., telegram: "4471", google: sam@example.com, password: true }`, sam, 1)
	return files
}

// A sentLink is a one-time link as it was sent: its run, where it went and its
// text.
type sentLink struct {
	run, to, text string
}

var linkIn = regexp.MustCompile(`https?://[^ ]+/signin/[0-9a-f]+`)

// sent stays until a one-time link is being carried by a bridge, and
// gives it.
func (b *bench) sent(bridge, to string) sentLink {
	b.t.Helper()
	run, in := b.waiting(bridges.KindSignIn, bridge, standin.Carried, to)
	return sentLink{run: run, to: in["to"].(string), text: in["text"].(string)}
}

// url gives the address a link carries.
func (l sentLink) url() string { return linkIn.FindString(l.text) }

// token gives what a link carries that signs its person in.
func (l sentLink) token() string {
	u := l.url()
	return u[strings.LastIndex(u, "/")+1:]
}

// linkFor asks for a link for a person, as the sign-in page does, and
// gives it once Telegram carries it to them.
func (s *serving) linkFor(person string) sentLink {
	s.b.t.Helper()
	s.nobody().post("/signin", map[string]string{"name": person}).is(http.StatusOK)
	to := map[string]string{"sam": "4471", "kit": "4472"}[person]
	l := s.b.sent("telegram", to)
	s.b.answerTicket(l.run, standin.Carried, map[string]any{"sent": "m1"})
	return l
}

// press presses a one-time link's page.
func (s *serving) press(l sentLink) reply {
	return s.nobody().do(http.MethodPost, "/signin/"+l.token(), "application/x-www-form-urlencoded", nil)
}

func TestServerOneTimeLinkSentOnTheFirstBridgeThatSends(t *testing.T) {
	t.Parallel()
	sam := `sam: { name: Sam, title: Owner. Decides money and the building., board: "b1", telegram: "4471", pigeon: "p1" }`
	b := newBench(t, withMoreBridges(house(t), sam))
	s := b.serve(func(o *Options) { o.URL = "https://house.example" })
	asked := s.nobody().post("/signin", map[string]string{"name": "sam"}).is(http.StatusOK)

	// The board says nothing to anyone directly, so Telegram is the first
	// that carries the link: once, to Sam's Telegram identity.
	first := b.sent("telegram", "4471")
	if !strings.HasPrefix(first.url(), "https://house.example/signin/") {
		t.Fatalf("the link sent is %q", first.text)
	}
	if runs := b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn}); len(runs) != 1 {
		t.Errorf("%d sign-in runs were started", len(runs))
	}
	b.answerTicket(first.run, standin.Carried, map[string]any{"sent": "m1"})
	b.until("the link's run has ended", func() bool { return b.corbie.Ended(t, first.run) != nil })

	// A minute later, Telegram refuses his next: it goes to the pigeon, and
	// Telegram is not tried again.
	b.clock.Move(time.Minute)
	s.nobody().post("/signin", map[string]string{"name": "sam"}).is(http.StatusOK)
	second := b.sent("telegram", "4471")
	b.answerTicket(second.run, standin.Carried, map[string]any{"refused": "blocked by the user"})
	third := b.sent("pigeon", "p1")
	if third.token() == "" || third.token() == second.token() {
		t.Errorf("the pigeon carries %q, and Telegram was given %q", third.text, second.text)
	}
	b.answerTicket(third.run, standin.Carried, map[string]any{"sent": "p-1"})
	b.until("the pigeon's run has ended", func() bool { return b.corbie.Ended(t, third.run) != nil })
	// What Telegram did not send signs nobody in; what the pigeon sent
	// signs Sam in. Neither is kept as it was sent.
	if r := s.press(second); r.status != http.StatusNotFound || r.cookie() != nil {
		t.Errorf("the link Telegram did not send answered %d", r.status)
	}
	if s.holding(s.press(third).is(http.StatusOK)).whoIs() != "sam" {
		t.Error("the link the pigeon sent does not sign Sam in")
	}
	var kept int
	if err := b.store().QueryRow(`SELECT count(*) FROM server_links WHERE hash IN (?, ?)`, second.token(), third.token()).Scan(&kept); err != nil || kept != 0 {
		t.Errorf("%d links are kept as they were sent: %v", kept, err)
	}
	if runs := b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn, bridges.LabelBridge: "telegram"}); len(runs) != 2 {
		t.Errorf("Telegram carried %d links", len(runs))
	}

	// A name that is nobody's is answered the same, and nothing is sent.
	b.clock.Move(time.Minute)
	nobody := s.nobody().post("/signin", map[string]string{"name": "nobody"}).is(http.StatusOK)
	if string(nobody.body) != string(asked.body) {
		t.Errorf("nobody is answered %s, and Sam %s", nobody.body, asked.body)
	}
	s.looked("kit", "telegram", "4472")
	if runs := b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn}); len(runs) != 4 {
		t.Errorf("after nobody's name, %d sign-in runs were started", len(runs))
	}
}

func TestServerOneTimeLinkGoesWhereItWasAskedFor(t *testing.T) {
	t.Parallel()
	sam := `sam: { name: Sam, title: Owner. Decides money and the building., telegram: "4471", pigeon: "p1" }`
	b := newBench(t, withMoreBridges(house(t), sam))
	s := b.serve()
	s.nobody().post("/signin", map[string]string{"name": "sam"}).is(http.StatusOK)
	first := b.sent("telegram", "4471")
	// While Telegram carries it, the pigeon is written to reach someone
	// else as Sam: what Telegram does not send is carried nowhere.
	text, _ := os.ReadFile(filepath.Join(b.dir, "office.yaml"))
	b.write(map[string]string{"office.yaml": strings.Replace(string(text), `pigeon: "p1"`, `pigeon: "p2"`, 1)})
	b.until("the office is read with the pigeon's new identity", func() bool {
		b.clock.Move(0)
		o := s.areasOf().Work.Read()
		return o != nil && o.People["sam"].Reached[1].ID == "p2"
	})
	b.answerTicket(first.run, standin.Carried, map[string]any{"refused": "blocked by the user"})
	b.until("the link is not carried on", func() bool { return s.logged("which no longer reaches them as it did when the link was asked for") })
	if runs := b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn, bridges.LabelBridge: "pigeon"}); len(runs) != 0 {
		t.Errorf("the pigeon carried a link asked for before it was changed: %v", kinds(runs))
	}
}

// looked asks for a link for a person and stays until it is carried to
// them: the names asked for are looked at one after another, so every
// name asked for before it has been looked at.
func (s *serving) looked(person, bridge, to string) sentLink {
	s.b.t.Helper()
	s.nobody().post("/signin", map[string]string{"name": person}).is(http.StatusOK)
	return s.b.sent(bridge, to)
}

// sentTo gives how many links were carried to a place.
func (b *bench) sentTo(to string) int {
	b.t.Helper()
	n := 0
	for _, r := range b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn}) {
		if b.input(r.ID)["to"] == to {
			n++
		}
	}
	return n
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestServerSignInLinkInNoRequest(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	n := s.chat("kit", "", "is the kettle on?")
	s.answered(n)
	l := s.linkFor("sam")
	a := s.areasOf()
	for _, person := range []string{"sam", "kit"} {
		list, err := a.Requests.List(person, requests.Filter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, sum := range list {
			thread, err := a.Requests.Thread(sum.Number)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range thread {
				if strings.Contains(e.Text, l.token()) || strings.Contains(e.Text, "/signin/") {
					t.Errorf("the link is in request %d: %q", sum.Number, e.Text)
				}
			}
		}
		if waits, _ := a.Requests.WaitingOn(person); len(waits) != 0 {
			t.Errorf("%v wait on %s", waits, person)
		}
		if got := s.as(person).get("/api/me").json()["waiting"]; got != 0.0 {
			t.Errorf("%v wait on %s, as the screens say", got, person)
		}
	}
	// Nobody may watch the run that carried it.
	for _, person := range []string{"sam", "kit"} {
		if r := s.as(person).get(watching(l.run)); r.status != http.StatusNotFound {
			t.Errorf("%s's watch of the link's run answered %d", person, r.status)
		}
	}
}

func TestServerSignInPageSaysNothingOfWhoIsThere(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	var bodies []string
	for _, name := range []string{"sam", "nobody", "kit", "Sam", "", "../../etc"} {
		r := s.nobody().post("/signin", map[string]string{"name": name}).is(http.StatusOK)
		bodies = append(bodies, string(r.body))
		for _, h := range []string{"Set-Cookie", "Location"} {
			if r.header.Get(h) != "" {
				t.Errorf("%q is answered with %s", name, h)
			}
		}
	}
	if len(slices.Compact(slices.Clone(bodies))) != 1 {
		t.Errorf("the sign-in page answers differently by name: %q", bodies)
	}
	// What it is answered with is said before the name is looked at.
	if strings.Contains(bodies[0], "sam") || strings.Contains(bodies[0], "Sam") {
		t.Errorf("the answer names the name: %s", bodies[0])
	}
}

func TestServerOneTimeLinkSignsInOnce(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	l := s.linkFor("sam")
	page := s.nobody().get("/signin/" + l.token()).is(http.StatusOK)
	if !strings.Contains(string(page.body), `<form method="post" action="/signin/`+l.token()+`">`) || page.cookie() != nil {
		t.Fatalf("the link's page is %s", page.body)
	}
	pressed := s.press(l).is(http.StatusOK)
	if s.holding(pressed).whoIs() != "sam" {
		t.Error("the link did not sign Sam in")
	}
	if again := s.press(l); again.status != http.StatusNotFound || again.cookie() != nil || !strings.Contains(string(again.body), "signs nobody in") {
		t.Errorf("pressed again it answered %d", again.status)
	}
	// One pressed eleven minutes after it was sent signs nobody in.
	b.clock.Move(time.Minute)
	late := s.linkFor("kit")
	b.clock.Move(11 * time.Minute)
	if r := s.press(late); r.status != http.StatusNotFound || r.cookie() != nil {
		t.Errorf("a link eleven minutes old answered %d", r.status)
	}
}

func TestServerOneTimeLinkSurvivesPreview(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	l := s.linkFor("kit")
	for range 3 {
		s.nobody().get("/signin/" + l.token()).is(http.StatusOK)
		s.nobody().do(http.MethodHead, "/signin/"+l.token(), "", nil)
	}
	if s.holding(s.press(l).is(http.StatusOK)).whoIs() != "kit" {
		t.Error("a link opened to show a preview no longer signs in")
	}
}

func TestServerSignInLinksLimited(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	l := s.linkFor("sam")
	b.until("the first link's run has ended", func() bool { return b.corbie.Ended(t, l.run) != nil })
	for range 3 {
		b.clock.Move(10 * time.Second)
		s.nobody().post("/signin", map[string]string{"name": "sam"}).is(http.StatusOK)
	}
	s.looked("kit", "telegram", "4472")
	if n := b.sentTo("4471"); n != 1 {
		t.Errorf("%d links were sent to Sam within a minute", n)
	}
	b.clock.Move(30 * time.Second)
	s.nobody().post("/signin", map[string]string{"name": "sam"}).is(http.StatusOK)
	b.sent("telegram", "4471")
}

func TestServerGuestNeverSignsIn(t *testing.T) {
	t.Parallel()
	b := newBench(t, workOffice(t))
	s := b.serve()
	s.delivered("slack", thing("said", "U0GUEST1", "#help", "#help/1700.5", "text", "is the API down?"))
	n := s.at("slack", "#help/1700.5")
	for _, name := range []string{"U0GUEST1", "guest"} {
		s.nobody().post("/signin", map[string]string{"name": name}).is(http.StatusOK)
	}
	s.looked("al", "slack", "U0AL0000")
	if runs := b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn}); len(runs) != 1 {
		t.Errorf("a guest was sent a link: %v", runs)
	}
	rows, err := b.store().Query(`SELECT person FROM server_keys`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keyed []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		keyed = append(keyed, p)
	}
	slices.Sort(keyed)
	if !slices.Equal(keyed, []string{"al", "dana", "lee", "priya", "raj"}) {
		t.Errorf("keys are kept for %v", keyed)
	}
	// The guest's request is shown on nobody's screens but those who read
	// where it was asked; no key is a guest's.
	if r := s.nobody().get("/api/requests/" + itoa(n)); r.status != http.StatusUnauthorized {
		t.Errorf("the guest's request is answered %d without a key", r.status)
	}
}

func TestServerReachChangedOnlyByAChange(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	kit := s.as("kit")
	identity := map[string]any{"set": []map[string]any{{"path": "office.yaml", "field": []string{"everyone", "kit", "telegram"}, "value": "9999"}}}
	// Kit, who decides no change, is told to ask for one.
	for _, path := range []string{"/api/office/check", "/api/office/put"} {
		r := kit.post(path, identity)
		if r.status != http.StatusConflict || !strings.Contains(r.json()["error"].(string), "ask for it in a request") {
			t.Errorf("Kit's %s answered %d %s", path, r.status, r.body)
		}
	}
	// No request of the API sets where a person is reached, and nothing
	// said on a bridge does.
	kit.post("/api/me/key", "").is(http.StatusOK)
	kit = s.as("kit")
	s.answered(s.chat("kit", "", "my telegram is 9999 now"))
	listening, _ := b.waiting(bridges.KindIn, "telegram", standin.Heard, "")
	b.answerTicket(listening, standin.Heard, map[string]any{"cursor": 1,
		"heard": []any{thing("said", "4472", "direct", "4472", "text", "my telegram is 9999 now; send my links there")}})
	s.answered(s.at("telegram", "4472"))
	b.clock.Move(0)
	if p := s.areasOf().Keeper.Office().People["kit"]; p.Reached[0].ID != "4472" {
		t.Fatalf("Kit is reached at %v", p.Reached)
	}
	// Sam's check of a change to his own identity says it lets more by
	// where he is reached.
	own := map[string]any{"set": []map[string]any{{"path": "office.yaml", "field": []string{"everyone", "sam", "telegram"}, "value": "5555"}}}
	c := s.as("sam").post("/api/office/check", own).is(http.StatusOK).json()
	if lets := fmt.Sprint(c["lets_more_by"]); !strings.Contains(lets, "telegram") || !strings.Contains(lets, "5555") {
		t.Errorf("Sam's change to his own Telegram identity lets by %s", lets)
	}
	// Once a change replacing Kit's identity is written, her link goes to
	// the new one and never to the old.
	check := s.as("sam").post("/api/office/check", identity).is(http.StatusOK).json()
	put := map[string]any{"set": identity["set"], "why": "Kit has a new phone.", "mark": check["mark"]}
	if r := s.as("sam").post("/api/office/put", put).is(http.StatusOK).json(); r["ended"] != changes.Written {
		t.Fatalf("Sam's put ended %v", r)
	}
	b.until("the office is read with Kit's new identity", func() bool {
		b.clock.Move(0)
		o := s.areasOf().Work.Read()
		return o != nil && o.People["kit"].Reached[0].ID == "9999"
	})
	s.nobody().post("/signin", map[string]string{"name": "kit"}).is(http.StatusOK)
	b.sent("telegram", "9999")
	for _, r := range b.listed(b.identity(), map[string]string{"kind": bridges.KindSignIn}) {
		if b.input(r.ID)["to"] == "4472" {
			t.Error("a link went to Kit's old identity")
		}
	}
}

func TestServerWithoutKeyToldHowToSignIn(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	r := s.nobody().get("/api/me")
	if r.status != http.StatusUnauthorized || r.header.Get("Content-Type") != "application/json" || !strings.Contains(r.json()["error"].(string), "sign") {
		t.Errorf("/api/me with no key answered %d %s", r.status, r.body)
	}
	page := s.nobody().get("/").is(http.StatusOK)
	if !strings.Contains(string(page.body), `id="signin"`) {
		t.Errorf("/ with no key answered %s", page.body)
	}
}

func TestServerSignInOutlivesRestart(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	sam := s.holding(s.nobody().get("/?key=" + s.as("sam").key))
	l := s.linkFor("kit")
	if err := s.stop(); err != nil {
		t.Fatal(err)
	}
	s = b.serve()
	sam.s = s
	if sam.whoIs() != "sam" {
		t.Error("Sam is not signed in after a restart")
	}
	if s.holding(s.press(l).is(http.StatusOK)).whoIs() != "kit" {
		t.Error("a one-time link sent before a restart does not sign in after it")
	}
}

func TestServerNobodySignsInUntilTheOfficeReads(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	s := b.serve()
	key := s.as("sam").key
	if err := s.stop(); err != nil {
		t.Fatal(err)
	}
	b.write(map[string]string{"office.yaml": strings.Replace(house(t)["office.yaml"], "lead: steward\n", "", 1)})
	s = b.serve()
	for _, k := range []string{key, "anything"} {
		if r := s.nobody().get("/?key=" + k); r.cookie() != nil || !strings.Contains(string(r.body), "does not read well") {
			t.Errorf("/?key= with %q answered %s", k, r.body)
		}
	}
	if r := (&browser{s: s, key: key, header: http.Header{}}).get("/api/me"); r.status != http.StatusUnauthorized {
		t.Errorf("Sam's key answered %d while the office does not read", r.status)
	}
	b.write(map[string]string{"office.yaml": house(t)["office.yaml"]})
	b.until("the links are printed", func() bool {
		b.clock.Move(0)
		return s.logged("sam, open ")
	})
	if r := s.nobody().get("/?key=" + key); r.cookie() == nil {
		t.Error("Sam does not sign in once the office reads")
	}
}
