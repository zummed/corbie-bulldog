package signin_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// The client a test's office writes, and the secret Corbie holds for it.
const (
	clientID = "1234-abc.apps.example"
	secret   = "the-clients-own-secret"
	tenant   = "72f988bf-86f1-41af-91ab-2d7cd011db47"
)

// A provider stands in for a provider's token endpoint: it takes a code it
// was told of, with the client's secret and the verifier, and answers an
// ID token of the claims the test gives the code.
type provider struct {
	*httptest.Server
	t     *testing.T
	mu    sync.Mutex
	codes map[string]map[string]any
	// forms are what each call to it was sent.
	forms []map[string]string
}

func newProvider(t *testing.T) *provider {
	p := &provider{t: t, codes: map[string]map[string]any{}}
	p.Server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.Close)
	return p
}

// code has the provider give an ID token of the claims for a code.
func (p *provider) code(code string, claims map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.codes[code] = claims
}

func (p *provider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r.ParseForm()
	form := map[string]string{"path": r.URL.Path}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	p.forms = append(p.forms, form)
	w.Header().Set("Content-Type", "application/json")
	claims, ok := p.codes[form["code"]]
	if !ok || form["client_secret"] != secret || form["client_id"] != clientID || form["code_verifier"] != "the-verifier" || form["grant_type"] != "authorization_code" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "Bad Request"})
		return
	}
	delete(p.codes, form["code"])
	body, _ := json.Marshal(claims)
	token := "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(body) + ".c2lnbmVk"
	json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "id_token": token, "token_type": "Bearer"})
}

// pointed gives a shipped way to sign in, its machine and script, with its
// provider's host in place of the stand-in's, reached over a plain
// connection. Nothing else of them is changed.
func pointed(t *testing.T, thing string, p *provider, host string) map[string]string {
	t.Helper()
	addr := strings.TrimPrefix(p.URL, "http://")
	files := map[string]string{}
	for _, name := range shipped.Files(thing) {
		contents, _ := shipped.File(path.Join(thing, name))
		text := string(contents)
		text = strings.ReplaceAll(text, "@net:"+host, "@net.unencrypted:"+addr)
		text = strings.ReplaceAll(text, "net:"+host, "net:"+addr)
		text = strings.ReplaceAll(text, "https://"+host, "http://"+addr)
		files[name] = text
	}
	return files
}

// signIn runs a way to sign in, pointed at the stand-in, as Bulldog starts
// it on a code it was given, and gives what it ended with.
func signIn(t *testing.T, files map[string]string, code string, settings map[string]any) *client.Run {
	t.Helper()
	d, err := machine.Parse([]byte(files[shipped.SignIn]), machine.Given(files))
	if err != nil {
		t.Fatalf("%s is refused: %v", shipped.SignIn, err)
	}
	c := testkit.NewCorbie(t, testkit.NewClock(testkit.Monday))
	input := map[string]any{"code": code, "verifier": "the-verifier", "redirect": "https://office.example/signin/google/back", "nonce": "the-nonce", "settings": settings}
	run, err := c.Start(context.Background(), d, client.Start{Input: input, Secrets: secrets(), Labels: map[string]string{"kind": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	return c.Ended(t, run)
}

func secrets() map[string]string {
	return map[string]string{"GOOGLE_CLIENT_SECRET": secret, "ENTRA_CLIENT_SECRET": secret}
}

// said gives what a run ended with, failing the test when it did not end
// well.
func said(t *testing.T, r *client.Run) map[string]any {
	t.Helper()
	out, _ := r.Output.(map[string]any)
	if r.Status != "done" || out == nil {
		t.Fatalf("the run ended %s with %#v: %s", r.Status, r.Output, r.Error)
	}
	return out
}

// later is a time in an ID token after the test, and earlier one before:
// the script reads the time of the computer it runs on.
func later() float64   { return float64(time.Now().Add(time.Hour).Unix()) }
func earlier() float64 { return float64(time.Now().Add(-time.Hour).Unix()) }

func google(claims map[string]any) map[string]any {
	out := map[string]any{"iss": "https://accounts.google.com", "aud": clientID, "exp": later(), "nonce": "the-nonce", "email": "Sam@Example.com", "email_verified": true, "sub": "1001"}
	for k, v := range claims {
		out[k] = v
	}
	return out
}

func TestGoogleGivesAVerifiedAddress(t *testing.T) {
	t.Parallel()
	p := newProvider(t)
	files := pointed(t, "signin/google", p, "oauth2.googleapis.com")
	settings := map[string]any{"from": "./.bulldog/signin/google", "client_id": clientID}
	p.code("good", google(nil))
	if out := said(t, signIn(t, files, "good", settings)); out["identity"] != "sam@example.com" {
		t.Errorf("the sign-in ended with %v", out)
	}
	if f := p.forms[0]; f["path"] != "/token" || f["redirect_uri"] != "https://office.example/signin/google/back" {
		t.Errorf("the token endpoint was sent %v", f)
	}
	// Each check refuses, saying why, and gives no identity.
	for code, claims := range map[string]map[string]any{
		"unverified": google(map[string]any{"email_verified": false}),
		"other":      google(map[string]any{"aud": "someone-else"}),
		"issuer":     google(map[string]any{"iss": "https://accounts.example"}),
		"expired":    google(map[string]any{"exp": earlier()}),
		"nonce":      google(map[string]any{"nonce": "another"}),
	} {
		p.code(code, claims)
		out := said(t, signIn(t, files, code, settings))
		if out["identity"] != nil || out["refused"] == nil || out["refused"] == "" {
			t.Errorf("%s ended with %v", code, out)
		}
	}
	// A code Google does not take is refused with what Google said.
	if out := said(t, signIn(t, files, "used", settings)); out["identity"] != nil || !strings.Contains(out["refused"].(string), "Bad Request") {
		t.Errorf("a used code ended with %v", out)
	}
}

func entra(claims map[string]any) map[string]any {
	out := map[string]any{"iss": "https://login.microsoftonline.com/" + tenant + "/v2.0", "aud": clientID, "tid": tenant, "exp": later(), "nonce": "the-nonce", "preferred_username": "Al@Corp.example"}
	for k, v := range claims {
		out[k] = v
	}
	return out
}

func TestEntraGivesTheSignInNameOfItsDirectory(t *testing.T) {
	t.Parallel()
	p := newProvider(t)
	files := pointed(t, "signin/entra", p, "login.microsoftonline.com")
	settings := map[string]any{"from": "./.bulldog/signin/entra", "tenant": tenant, "client_id": clientID}
	p.code("good", entra(nil))
	if out := said(t, signIn(t, files, "good", settings)); out["identity"] != "al@corp.example" {
		t.Errorf("the sign-in ended with %v", out)
	}
	if f := p.forms[0]; f["path"] != "/"+tenant+"/oauth2/v2.0/token" {
		t.Errorf("the token endpoint was sent %v", f)
	}
	other := "0b1c2d3e-4f50-6172-8394-a5b6c7d8e9f0"
	for code, claims := range map[string]map[string]any{
		"tenant":  entra(map[string]any{"tid": other, "iss": "https://login.microsoftonline.com/" + other + "/v2.0"}),
		"issuer":  entra(map[string]any{"iss": "https://login.example/" + tenant + "/v2.0"}),
		"other":   entra(map[string]any{"aud": "someone-else"}),
		"expired": entra(map[string]any{"exp": earlier()}),
		"nonce":   entra(map[string]any{"nonce": "another"}),
		"nobody":  entra(map[string]any{"preferred_username": nil}),
	} {
		p.code(code, claims)
		out := said(t, signIn(t, files, code, settings))
		if out["identity"] != nil || out["refused"] == nil || out["refused"] == "" {
			t.Errorf("%s ended with %v", code, out)
		}
	}
	// One with no sign-in name but an address is known by it.
	p.code("email", entra(map[string]any{"preferred_username": nil, "email": "Al@Corp.example"}))
	if out := said(t, signIn(t, files, "email", settings)); out["identity"] != "al@corp.example" {
		t.Errorf("the sign-in by address ended with %v", out)
	}
}

func TestSettingsAreChecked(t *testing.T) {
	t.Parallel()
	for thing, settings := range map[string]map[string]any{
		"signin/google": {"from": "./.bulldog/signin/google"},
		"signin/entra":  {"client_id": clientID, "tenant": "corp.example"},
	} {
		contents, _ := shipped.File(path.Join(thing, shipped.SignIn))
		d, err := machine.Parse(contents, func(name string) ([]byte, error) {
			c, _ := shipped.File(path.Join(thing, name))
			return c, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		schema, err := machine.CompileShape(d.Definition.Input)
		if err != nil {
			t.Fatal(err)
		}
		input := map[string]any{"code": "c", "verifier": "v", "redirect": "r", "nonce": "n", "settings": settings}
		if machine.Fits(schema, input) == nil {
			t.Errorf("%s takes the settings %v", thing, settings)
		}
	}
}
