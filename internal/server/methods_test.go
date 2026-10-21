package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// signingIn gives the house signing in by google, a stand-in of its own
// whose machine answers the code it is given as the account, with Sam's
// account by it, and Sam able to set a password of his own.
func signingIn(t *testing.T) map[string]string {
	t.Helper()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "google: sam@example.com", "google: Sam@Example.com", 1)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  google:\n    from: ./.bulldog/signin/google\n    client_id: 1234567890-house.apps.googleusercontent.com   # from the Google Cloud console; Corbie keeps GOOGLE_CLIENT_SECRET\n",
		"  google: { client_id: house-client, authorize: \"https://accounts.example/auth\", scope: openid email, label: Google }\n", 1)
	for name, text := range testkit.SignIn() {
		files["signin/google/"+name] = text
	}
	return files
}

// begin has a browser begin to sign in by a method, and gives where it was
// sent and the cookie that binds the sign-in to it.
func (s *serving) begin(method string) (*url.URL, *http.Cookie) {
	s.b.t.Helper()
	r := s.nobody().get("/signin/" + method).is(http.StatusSeeOther)
	to, err := url.Parse(r.header.Get("Location"))
	if err != nil {
		s.b.t.Fatal(err)
	}
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == SigningCookie {
			return to, c
		}
	}
	s.b.t.Fatalf("beginning to sign in by %s gave no cookie", method)
	return nil, nil
}

// back has a browser holding a cookie, or none, come back from the
// provider with a code and a state.
func (s *serving) back(method, code, state string, bound *http.Cookie) reply {
	s.b.t.Helper()
	br := s.nobody()
	if bound != nil {
		br.header.Set("Cookie", SigningCookie+"="+bound.Value)
	}
	return br.get("/signin/" + method + "/back?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state))
}

// signInBy has a browser sign in by a method with a code, from its start,
// and gives what it came back to.
func (s *serving) signInBy(method, code string) reply {
	s.b.t.Helper()
	to, bound := s.begin(method)
	return s.back(method, code, to.Query().Get("state"), bound)
}

func TestServerSignInByMethodGivesTheKey(t *testing.T) {
	t.Parallel()
	b := newBench(t, signingIn(t))
	s := b.serve()
	if !s.logged("google signs people in by their provider sending them back to " + s.address() + "/signin/google/back") {
		t.Errorf("the log does not name google's redirect URI:\n%s", strings.Join(s.lines(), "\n"))
	}
	page := string(s.nobody().get("/").is(http.StatusOK).body)
	if !strings.Contains(page, `<a class="signin-with" href="/signin/google">Sign in with Google</a>`) || !strings.Contains(page, `id="password"`) || !strings.Contains(page, `id="signin"`) {
		t.Errorf("the sign-in page is %s", page)
	}

	to, bound := s.begin("google")
	q := to.Query()
	if to.Host != "accounts.example" || to.Path != "/auth" || q.Get("response_type") != "code" || q.Get("client_id") != "house-client" ||
		q.Get("redirect_uri") != s.address()+"/signin/google/back" || q.Get("scope") != "openid email" || q.Get("code_challenge_method") != "S256" ||
		len(q.Get("state")) < 32 || len(q.Get("nonce")) < 32 || len(q.Get("code_challenge")) != 43 {
		t.Errorf("the browser was sent to %s", to)
	}
	if !bound.HttpOnly || bound.SameSite != http.SameSiteLaxMode || bound.Path != "/signin/" || bound.MaxAge != 600 || bound.Secure {
		t.Errorf("the cookie that binds the sign-in is %+v", bound)
	}
	r := s.back("google", "sam@example.com", q.Get("state"), bound).is(http.StatusOK)
	if !strings.Contains(string(r.body), `http-equiv="refresh" content="0;url=/"`) {
		t.Errorf("the page does not move on to /: %s", r.body)
	}
	if s.holding(r).whoIs() != "sam" {
		t.Error("signing in by google gave Sam no key")
	}

	// The method's machine ran once, labelled, given the nonce sent and
	// the verifier of the challenge.
	runs := b.listed(b.identity(), map[string]string{work.LabelKind: work.KindSignIn, work.LabelSignIn: "google"})
	if len(runs) != 1 {
		t.Fatalf("the sign-in runs are %v", runs)
	}
	step, err := b.corbie.Step(ctx, runs[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	in, _ := step.Events[0].Data.(map[string]any)
	verifier, _ := in["verifier"].(string)
	sum := sha256.Sum256([]byte(verifier))
	if in["nonce"] != q.Get("nonce") || base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") || in["redirect"] != q.Get("redirect_uri") {
		t.Errorf("the run was given %v", in)
	}
}

func TestServerUnwrittenAccountAnsweredAlike(t *testing.T) {
	t.Parallel()
	s := newBench(t, signingIn(t)).serve()
	stranger := s.signInBy("google", "stranger@gmail.com").is(http.StatusForbidden)
	near := s.signInBy("google", "sam@example.org").is(http.StatusForbidden)
	if stranger.cookie() != nil || near.cookie() != nil || string(stranger.body) != string(near.body) || !strings.Contains(string(near.body), "No one of the office signs in with that Google account.") {
		t.Errorf("two accounts the office does not write are answered %s and %s", stranger.body, near.body)
	}
	// What the machine refuses is said, and what failed is said to be in
	// the log.
	refused := s.signInBy("google", "refuse:Google has not verified the address.").is(http.StatusForbidden)
	if refused.cookie() != nil || !strings.Contains(string(refused.body), "Google has not verified the address.") {
		t.Errorf("a refusal is answered %s", refused.body)
	}
	failed := s.signInBy("google", "fail").is(http.StatusServiceUnavailable)
	if failed.cookie() != nil || !strings.Contains(string(failed.body), "failure is in the server&#39;s log") || !s.logged("signing in with google failed") {
		t.Errorf("a failure is answered %s", failed.body)
	}
	// What the provider says when it gives no code is said.
	to, bound := s.begin("google")
	br := s.nobody()
	br.header.Set("Cookie", SigningCookie+"="+bound.Value)
	if r := br.get("/signin/google/back?error=access_denied&state=" + to.Query().Get("state")).is(http.StatusForbidden); !strings.Contains(string(r.body), "Google did not sign you in: access_denied.") {
		t.Errorf("a provider's refusal is answered %s", r.body)
	}
}

func TestServerSigningStateBoundUsedOnceAndShort(t *testing.T) {
	t.Parallel()
	b := newBench(t, signingIn(t))
	s := b.serve()
	refused := func(what string, r reply) {
		t.Helper()
		if r.status != http.StatusBadRequest || r.cookie() != nil || !strings.Contains(string(r.body), "begin it again") {
			t.Errorf("%s is answered %d: %s", what, r.status, r.body)
		}
	}
	// A state that is no sign-in's, and one with no cookie, which is used
	// up all the same.
	to, bound := s.begin("google")
	state := to.Query().Get("state")
	refused("another state", s.back("google", "sam@example.com", "not-a-state", bound))
	refused("no cookie", s.back("google", "sam@example.com", state, nil))
	refused("the state once tried", s.back("google", "sam@example.com", state, bound))
	// Another browser's cookie.
	to, _ = s.begin("google")
	_, other := s.begin("google")
	refused("another browser's cookie", s.back("google", "sam@example.com", to.Query().Get("state"), other))
	// A state used once.
	to, bound = s.begin("google")
	s.back("google", "sam@example.com", to.Query().Get("state"), bound).is(http.StatusOK)
	refused("a used state", s.back("google", "sam@example.com", to.Query().Get("state"), bound))
	// One more than ten minutes old.
	to, bound = s.begin("google")
	b.clock.Move(Signing + time.Second)
	refused("an old state", s.back("google", "sam@example.com", to.Query().Get("state"), bound))
	// A state another method began.
	files := signingIn(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "signin:\n", "signin:\n  other: { client_id: c, authorize: \"https://other.example/auth\", scope: openid }\n", 1)
	for name, text := range testkit.SignIn() {
		files["signin/other/"+name] = text
	}
	b.write(map[string]string{office.OfficeFile: files["office.yaml"], "signin/other/signin.yaml": files["signin/other/signin.yaml"], "signin/other/answer.js": files["signin/other/answer.js"]})
	b.until("other is read", func() bool {
		b.clock.Move(0)
		o := s.areasOf().Keeper.Office()
		return o != nil && o.SignIns["other"] != nil
	})
	to, bound = s.begin("other")
	refused("another method's state", s.back("google", "sam@example.com", to.Query().Get("state"), bound))
}

func TestServerStoppedSignInMethodBeginsNothing(t *testing.T) {
	t.Parallel()
	files := signingIn(t)
	files["signin/google/signin.yaml"] = "flow: x\nstages: [\n"
	s := newBench(t, files).serve()
	r := s.nobody().get("/signin/google").is(http.StatusServiceUnavailable)
	if !strings.Contains(string(r.body), "Signing in with Google is stopped") || r.header.Get("Location") != "" {
		t.Errorf("a stopped method is answered %s", r.body)
	}
	if page := string(s.nobody().get("/").body); strings.Contains(page, "/signin/google") {
		t.Errorf("the sign-in page offers a stopped method: %s", page)
	}
	me := s.as("sam").get("/api/me").is(http.StatusOK).json()
	methods := me["office"].(map[string]any)["signin"].([]any)
	if len(methods) != 1 || methods[0].(map[string]any)["status"] != "stopped" {
		t.Errorf("the office's methods are %v", methods)
	}
}

func TestServerMeSaysHowOneSignsIn(t *testing.T) {
	t.Parallel()
	s := newBench(t, signingIn(t)).serve()
	sam := s.as("sam").get("/api/me").is(http.StatusOK).json()
	me := sam["me"].(map[string]any)
	accounts := me["accounts"].([]any)
	if len(accounts) != 1 || accounts[0].(map[string]any)["id"] != "Sam@Example.com" || accounts[0].(map[string]any)["called"] != "Google" {
		t.Errorf("Sam's accounts are %v", accounts)
	}
	if pw, _ := me["password"].(map[string]any); pw == nil || pw["set"] != false {
		t.Errorf("Sam's password is %v", me["password"])
	}
	// Kit sees that Sam signs in by google, and none of his account.
	kit := s.as("kit").get("/api/me").is(http.StatusOK).json()
	for _, p := range kit["people"].([]any) {
		person := p.(map[string]any)
		if person["name"] == "sam" && (len(person["signin"].([]any)) != 1 || person["signin"].([]any)[0] != "google" || person["accounts"] != nil) {
			t.Errorf("Kit sees Sam as %v", person)
		}
	}
	if k := kit["me"].(map[string]any); k["password"] != nil || k["accounts"] != nil {
		t.Errorf("Kit is %v", k)
	}
	office := kit["office"].(map[string]any)
	if !office["password"].(bool) || office["signin"].([]any)[0].(map[string]any)["back"] != s.address()+"/signin/google/back" {
		t.Errorf("the office is %v", office)
	}
}

func TestServerPasswordSetAndSignedInWith(t *testing.T) {
	t.Parallel()
	s := newBench(t, signingIn(t)).serve()
	sam := s.as("sam")
	other := s.as("sam")
	if r := s.as("kit").post("/api/me/password", map[string]string{"password": "a long enough one"}).is(http.StatusConflict); !strings.Contains(r.json()["error"].(string), "password: true") {
		t.Errorf("Kit setting a password is answered %s", r.body)
	}
	sam.post("/api/me/password", map[string]string{"password": "too short"}).is(http.StatusConflict)
	sam.post("/api/me/password", map[string]string{"password": strings.Repeat("x", 73)}).is(http.StatusConflict)
	if r := sam.post("/api/me/password", map[string]string{"password": "correct horse battery"}).is(http.StatusOK); r.json()["said"] != "Your password is set." {
		t.Errorf("setting it is answered %s", r.body)
	}
	if r := s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "correct horse battery"}).is(http.StatusOK); s.holding(r).whoIs() != "sam" {
		t.Error("Sam's password does not sign him in")
	}
	// A password is changed only with the one it replaces, and setting it
	// signs nobody out.
	sam.post("/api/me/password", map[string]string{"password": "staple battery horse"}).is(http.StatusConflict)
	sam.post("/api/me/password", map[string]string{"current": "wrong one here", "password": "staple battery horse"}).is(http.StatusConflict)
	if r := sam.post("/api/me/password", map[string]string{"current": "correct horse battery", "password": "staple battery horse"}).is(http.StatusOK); r.json()["said"] != "Your password is changed." {
		t.Errorf("changing it is answered %s", r.body)
	}
	if other.whoIs() != "sam" {
		t.Error("setting a password signed another browser out")
	}
	s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "correct horse battery"}).is(http.StatusForbidden)
	s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "staple battery horse"}).is(http.StatusOK)
	// The hash is all that is kept.
	var hash string
	if err := s.b.store().QueryRow(`SELECT hash FROM server_passwords WHERE person = 'sam'`).Scan(&hash); err != nil || !strings.HasPrefix(hash, "$2") || strings.Contains(hash, "staple") {
		t.Errorf("Sam's password is kept as %q: %v", hash, err)
	}
}

func TestServerPasswordWrongNameAnsweredAlike(t *testing.T) {
	t.Parallel()
	s := newBench(t, signingIn(t)).serve()
	s.as("sam").post("/api/me/password", map[string]string{"password": "correct horse battery"}).is(http.StatusOK)
	wrong := s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "not his"}).is(http.StatusForbidden)
	for _, name := range []string{"nobody", "kit", "Sam"} {
		r := s.nobody().post("/signin/password", map[string]string{"name": name, "password": "correct horse battery"}).is(http.StatusForbidden)
		if string(r.body) != string(wrong.body) || r.cookie() != nil {
			t.Errorf("%s is answered %s, and a wrong password %s", name, r.body, wrong.body)
		}
	}
	// A house where nobody may set a password offers none.
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], ", password: true", "", 1)
	if page := string(newBench(t, files).serve().nobody().get("/").body); strings.Contains(page, `id="password"`) {
		t.Error("the house, where nobody may set a password, offers one")
	}
}

func TestServerPasswordTriesLimited(t *testing.T) {
	t.Parallel()
	b := newBench(t, signingIn(t))
	s := b.serve()
	s.as("sam").post("/api/me/password", map[string]string{"password": "correct horse battery"}).is(http.StatusOK)
	for _, name := range []string{"sam", "nobody"} {
		for range Tries {
			s.nobody().post("/signin/password", map[string]string{"name": name, "password": "a wrong guess"}).is(http.StatusForbidden)
		}
		r := s.nobody().post("/signin/password", map[string]string{"name": name, "password": "correct horse battery"}).is(http.StatusTooManyRequests)
		if r.cookie() != nil || !strings.Contains(r.json()["error"].(string), "try again after 10:01") {
			t.Errorf("%s's next try is answered %s", name, r.body)
		}
	}
	// Kit's name is no less tried for Sam's tries.
	s.nobody().post("/signin/password", map[string]string{"name": "kit", "password": "a wrong guess"}).is(http.StatusForbidden)
	// Once the wait has passed, the right password signs Sam in, and his
	// tries begin again; the wait after one more wrong one is longer.
	b.clock.Move(TriedWait)
	s.nobody().post("/signin/password", map[string]string{"name": "nobody", "password": "a wrong guess"}).is(http.StatusForbidden)
	if r := s.nobody().post("/signin/password", map[string]string{"name": "nobody", "password": "a wrong guess"}); r.status != http.StatusTooManyRequests || !strings.Contains(r.json()["error"].(string), "10:03") {
		t.Errorf("nobody's seventh try is answered %d %s", r.status, r.body)
	}
	if r := s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "correct horse battery"}).is(http.StatusOK); s.holding(r).whoIs() != "sam" {
		t.Error("the right password after the wait signs nobody in")
	}
	for range Tries - 1 {
		s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "a wrong guess"}).is(http.StatusForbidden)
	}
	s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "correct horse battery"}).is(http.StatusOK)
}

func TestServerPasswordDestroyedWithEntry(t *testing.T) {
	t.Parallel()
	b := newBench(t, signingIn(t))
	s := b.serve()
	s.as("sam").post("/api/me/password", map[string]string{"password": "correct horse battery"}).is(http.StatusOK)
	text, err := os.ReadFile(filepath.Join(b.dir, office.OfficeFile))
	if err != nil {
		t.Fatal(err)
	}
	b.write(map[string]string{office.OfficeFile: strings.Replace(string(text), ", password: true", "", 1)})
	b.until("Sam's password is destroyed", func() bool {
		b.clock.Move(0)
		return s.logged("sam's entry no longer says password: true, so their password is destroyed")
	})
	s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "correct horse battery"}).is(http.StatusForbidden)
	// Written again, it lets him set one, and he has none.
	b.write(map[string]string{office.OfficeFile: string(text)})
	b.until("Sam may set a password again", func() bool {
		b.clock.Move(0)
		o := s.areasOf().Work.Read()
		return o != nil && o.People["sam"].Password
	})
	if pw := s.as("sam").get("/api/me").is(http.StatusOK).json()["me"].(map[string]any)["password"].(map[string]any); pw["set"] != false {
		t.Errorf("Sam's password is %v", pw)
	}
	s.nobody().post("/signin/password", map[string]string{"name": "sam", "password": "correct horse battery"}).is(http.StatusForbidden)
}

func TestServerBuiltInListsSignInMethods(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	var out shippedOut
	s.as("kit").get("/api/shipped").is(http.StatusOK).into(&out)
	things := map[string]thingOut{}
	for _, t := range out.Things {
		things[t.Path] = t
	}
	google, entra := things["signin/google"], things["signin/entra"]
	if google.Kind != "sign-in" || google.Status != "used" || len(google.Uses) != 1 || google.Uses[0].SignIn != "google" || google.Uses[0].Line != "from: ./.bulldog/signin/google" {
		t.Errorf("signin/google is %+v", google)
	}
	if entra.Kind != "sign-in" || entra.Status != "unused" || entra.Line != "from: ./.bulldog/signin/entra" ||
		!slices.ContainsFunc(entra.Settings, func(s shipped.Setting) bool { return s.Name == "tenant" }) {
		t.Errorf("signin/entra is %+v", entra)
	}
	if !slices.Equal(entra.Needs.Hosts, []string{"login.microsoftonline.com"}) {
		t.Errorf("signin/entra reaches %+v", entra.Needs)
	}
}
