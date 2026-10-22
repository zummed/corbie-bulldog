package screens

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// secret is the office's secret in every test.
var secret = []byte("the office's secret in the tests")

// The program gives the screens the office's bridges as they are.
var _ Bridges = (*bridges.Bridges)(nil)

// standIn stands in for the bridges. A message is carried by a run that
// gives { sent }, and each is kept. Slack and Telegram reach people
// directly; GitLab does not. Every in run runs, but GitLab's, which fails.
type standIn struct {
	corbie *testkit.Corbie
	draft  *machine.Draft
	mu     sync.Mutex
	sent   []requests.Message
}

func newStandIn(t *testing.T, corbie *testkit.Corbie) *standIn {
	files := map[string]string{
		"out.yaml": "flow: out\ninput: { sent: string }\nstages:\n  send: { job: { script: ./send.js, input: { sent: \"${input.sent}\" } } }\noutput: ${stages.send.output}\n",
		"send.js":  "return { sent: input.sent }",
	}
	draft, err := machine.Parse([]byte(files["out.yaml"]), func(name string) ([]byte, error) { return []byte(files[name]), nil })
	if err != nil {
		t.Fatal(err)
	}
	return &standIn{corbie: corbie, draft: draft}
}

func (b *standIn) Carry(ctx context.Context, m requests.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	sent := fmt.Sprintf("%s:%d", m.To, len(b.sent)+1)
	if _, err := b.corbie.Start(ctx, b.draft, client.Start{Run: m.Run, Labels: m.Labels, Input: map[string]any{"sent": sent}}); err != nil {
		return err
	}
	b.sent = append(b.sent, m)
	return nil
}

func (b *standIn) Direct(bridge string) bool { return bridge == "telegram" || bridge == "slack" }

func (b *standIn) State(bridge string) bridges.State {
	if bridge == "gitlab" {
		return bridges.State{Status: "failing", Error: "GitLab answered 401 Unauthorized"}
	}
	return bridges.State{Status: "running"}
}

// carried gives every message carried so far.
func (b *standIn) carried() []requests.Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.sent)
}

// A world is an office served on a Corbie of its own, with a stand-in
// model, stand-in bridges and an engine that drives it.
type world struct {
	t       *testing.T
	corbie  *testkit.Corbie
	model   *testkit.Model
	office  *office.Office
	engine  *requests.Engine
	bridges *standIn
	screens *Screens
	server  *httptest.Server
}

// models are the models an example office names, which a test's model
// stands in for.
var models = regexp.MustCompile(`\{ name: claude-[a-z0-9-]+, secret: ANTHROPIC_KEY \}`)

// newWorld serves an example office, changed as the test changes it, with
// a model that gives each turn what answer gives.
func newWorld(t *testing.T, tree fs.FS, answer func(testkit.Turn) testkit.Reply, change func(files map[string]string)) *world {
	t.Helper()
	model := testkit.NewModel(t, answer)
	files := testkit.Files(t, tree)
	for name, text := range files {
		files[name] = models.ReplaceAllString(text, model.Line())
	}
	if change != nil {
		change(files)
	}
	o, err := office.Read(testkit.Dir(t, files), shipped.Files)
	if err != nil {
		t.Fatal(err)
	}
	reading := requests.NewReading(o)
	if len(o.Problems) > 0 {
		t.Fatalf("the office has problems:\n%v", office.Problems(o.Problems))
	}
	corbie := testkit.NewCorbie(t)
	w := &world{t: t, corbie: corbie, model: model, office: o, bridges: newStandIn(t, corbie)}
	w.engine = requests.New(corbie.Client, w.bridges, secret, reading)
	w.engine.Report = func(err error) { t.Errorf("the engine reports: %v", err) }
	w.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { w.screens.ServeHTTP(rw, r) }))
	w.screens = New(corbie.Client, w.engine, w.bridges, secret, w.server.URL)
	w.screens.Report = func(err error) { t.Errorf("the screens report: %v", err) }
	run, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- w.engine.Run(run) }()
	t.Cleanup(func() {
		w.server.CloseClientConnections()
		w.server.Close()
		w.screens.sending.Wait()
		stop()
		if err := <-done; err != nil {
			t.Errorf("the engine ended: %v", err)
		}
	})
	return w
}

// A browser is one browser at the screens, with its own cookies.
type browser struct {
	w      *world
	client *http.Client
}

// browser opens a browser that has signed nobody in, and follows no
// redirect.
func (w *world) browser() *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{w: w, client: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// as opens a browser signed in as a person, with their key link.
func (w *world) as(person string) *browser {
	w.t.Helper()
	b := w.browser()
	link, err := KeyLink(w.server.URL, secret, w.office, person)
	if err != nil {
		w.t.Fatal(err)
	}
	if res := b.do("POST", link, nil); res.StatusCode != http.StatusSeeOther {
		w.t.Fatalf("%s's key link answered %s", person, res.Status)
	}
	return b
}

// do asks the screens as the browser, and gives the answer with its body
// read.
func (b *browser) do(method, address string, body io.Reader) *http.Response {
	b.w.t.Helper()
	if strings.HasPrefix(address, "/") {
		address = b.w.server.URL + address
	}
	req, err := http.NewRequest(method, address, body)
	if err != nil {
		b.w.t.Fatal(err)
	}
	if method == "POST" && strings.Contains(address, "/signin") && body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	res, err := b.client.Do(req)
	if err != nil {
		b.w.t.Fatal(err)
	}
	data, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		b.w.t.Fatal(err)
	}
	res.Body = io.NopCloser(bytes.NewReader(data))
	return res
}

// api asks the API, with a body as JSON, and gives the status and the
// answer as it reads.
func (b *browser) api(method, path string, body any) (int, map[string]any) {
	b.w.t.Helper()
	var sent io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		sent = bytes.NewReader(data)
	}
	res := b.do(method, path, sent)
	var out map[string]any
	data, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(data, &out); err != nil {
		b.w.t.Fatalf("%s %s answered %s with %q, not JSON", method, path, res.Status, data)
	}
	return res.StatusCode, out
}

// get asks the API for something the browser may see, and gives it.
func (b *browser) get(path string) map[string]any {
	b.w.t.Helper()
	status, out := b.api("GET", path, nil)
	if status != http.StatusOK {
		b.w.t.Fatalf("GET %s answered %d: %v", path, status, out)
	}
	return out
}

// body gives the whole answer to a GET, as text.
func (b *browser) body(path string) (int, string) {
	b.w.t.Helper()
	res := b.do("GET", path, nil)
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(data)
}

// ask asks a team in the chat as the browser, and gives the request's id.
func (b *browser) ask(team, said string) string {
	b.w.t.Helper()
	status, out := b.api("POST", "/api/requests", map[string]any{"team": team, "said": said})
	if status != http.StatusOK {
		b.w.t.Fatalf("asking %s answered %d: %v", team, status, out)
	}
	return out["id"].(string)
}

// run gives the full name of a request's run.
func (w *world) run(id string) string { return w.corbie.ID.Name + "/" + id }

// ended stays until a request has ended.
func (w *world) ended(id string) *client.Run {
	w.t.Helper()
	return w.corbie.Ended(w.t, w.run(id))
}

// until stays until a request the browser asks for shows what done says.
func (b *browser) until(what, path string, done func(map[string]any) bool) map[string]any {
	b.w.t.Helper()
	var out map[string]any
	b.w.corbie.Until(b.w.t, what, func() bool {
		var status int
		status, out = b.api("GET", path, nil)
		return status == http.StatusOK && done(out)
	})
	return out
}

// card stays until a card waits on the browser's person, and gives it.
func (b *browser) card() map[string]any {
	b.w.t.Helper()
	list := b.until("a card waits", "/api/requests", func(out map[string]any) bool { return len(out["waiting"].([]any)) > 0 })
	return list["waiting"].([]any)[0].(map[string]any)
}

// jsonOf writes a value as JSON, for a test to look into.
func jsonOf(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// form gives values as a form's body.
func form(values url.Values) io.Reader { return strings.NewReader(values.Encode()) }

// says gives the work office's ledger and ops their words: the ledger
// reads its doc when asked about the rule, refunds ch_1 when asked to, and
// otherwise reads the refunds doc, then answers.
func says(turn testkit.Turn) testkit.Reply {
	prompt := turn.Prompt()
	switch {
	case strings.Contains(prompt, "Refund ch_") && turn.Number == 1:
		return testkit.Use("refund", map[string]any{"charge": strings.TrimSuffix(prompt[strings.Index(prompt, "ch_"):], ".")})
	case strings.Contains(prompt, "Refund ch_"):
		return testkit.Say("The refund is done.")
	case strings.Contains(prompt, "rule") && turn.Number == 1:
		return testkit.Use("read_doc", map[string]any{"name": "refunds.md"})
	case strings.Contains(prompt, "rule"):
		return testkit.Say("The rule: " + turn.Last())
	case strings.Contains(prompt, "API"):
		return testkit.Say("The API is up.")
	case turn.Number == 1 && turn.Tools["read_doc"].Description != "":
		return testkit.Use("read_doc", map[string]any{"name": "orders.md"})
	}
	return testkit.Say("Refunded £40 on 3 May.")
}
