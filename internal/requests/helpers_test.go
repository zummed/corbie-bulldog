package requests_test

import (
	"database/sql"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// A desk is a store of requests on a file of the test's own, with the
// office it was last given, on a clock the test moves.
type desk struct {
	*requests.Store
	t     *testing.T
	db    *sql.DB
	file  string
	clock *testkit.Clock
	files map[string]string
	o     *office.Office
}

// open opens a store on a new file and reads an office into it.
func open(t *testing.T, files map[string]string) *desk {
	t.Helper()
	d := &desk{t: t, file: filepath.Join(t.TempDir(), "bulldog.db"), clock: testkit.NewClock(testkit.Monday)}
	d.connect()
	d.read(files)
	return d
}

func (d *desk) connect() {
	d.t.Helper()
	db, err := sql.Open("sqlite", d.file+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(func() { db.Close() })
	store, err := requests.Open(db, d.clock.Now)
	if err != nil {
		d.t.Fatal(err)
	}
	d.db, d.Store = db, store
}

// restart opens the file again, as a server started again would, and
// reads the office into it.
func (d *desk) restart() {
	d.t.Helper()
	d.db.Close()
	d.connect()
	d.read(d.files)
}

// read reads an office's files, which must read, into the store.
func (d *desk) read(files map[string]string) {
	d.t.Helper()
	given := map[string][]byte{}
	for name, text := range files {
		given[name] = []byte(text)
	}
	o, problems := office.Parse(d.t.TempDir(), given)
	if problems.Refuse() {
		d.t.Fatalf("the office does not read:\n%v", problems)
	}
	d.files, d.o = files, o
	must(d.t, d.Read(o))
}

// house gives the house's files, its Telegram bridge listening in a place
// the garden takes and in one where guests may ask, beside direct.
func house(t *testing.T) map[string]string {
	files := testkit.Files(t, examples.House())
	return replace(t, files, "office.yaml", "      direct: {}", "      garden-chat: { team: garden }\n      porch: { guests: true }\n      direct: {}")
}

func work(t *testing.T) map[string]string { return testkit.Files(t, examples.Work()) }

// replace gives files with one file's text changed: old must be in it.
func replace(t *testing.T, files map[string]string, file, old, new string) map[string]string {
	t.Helper()
	if !strings.Contains(files[file], old) {
		t.Fatalf("%s does not hold %q", file, old)
	}
	out := maps.Clone(files)
	out[file] = strings.Replace(files[file], old, new, 1)
	return out
}

// without gives files with every file under a folder taken out.
func without(files map[string]string, dir string) map[string]string {
	out := map[string]string{}
	for name, text := range files {
		if !strings.HasPrefix(name, dir+"/") {
			out[name] = text
		}
	}
	return out
}

// moved gives files with a folder renamed.
func moved(files map[string]string, from, to string) map[string]string {
	out := map[string]string{}
	for name, text := range files {
		if rest, ok := strings.CutPrefix(name, from+"/"); ok {
			name = to + "/" + rest
		}
		out[name] = text
	}
	return out
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func got[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		must(t, err)
		return v
	}
}

// The identities of the work office's people on Slack, and Al's on GitLab.
const (
	slackAl    = "U0AL0000"
	slackDana  = "U0DANA00"
	slackLee   = "U0LEE000"
	slackPriya = "U0PRI000"
	slackRaj   = "U0RAJ000"
	gitlabAl   = "1041"
	thread     = "#help/1700.01"
)

// A day is the work day: Dana asks in her thread in #help, which the
// bridge reports Dana, Al and Raj as reading, and the bot asks payments
// and platform.
type day struct {
	*desk
	dana, payments, platform int64
}

func workDay(t *testing.T) *day {
	t.Helper()
	d := &day{desk: open(t, work(t))}
	must(t, d.Readers("slack", thread, []string{slackDana, slackAl, slackRaj}))
	d.dana = got[*requests.Request](t)(d.AskAt("slack", "#help", thread, slackDana, "customer 4411 was charged twice after Tuesday's release; refund one and tell me what broke.", nil)).Number
	d.payments = got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "payments", "Refund one of 4411's two charges.", "ticket-payments")).Number
	d.platform = got[*requests.Request](t)(d.Sub(d.dana, d.Worker("bot"), "platform", "What broke the webhook retry?", "ticket-platform")).Number
	return d
}

// sees says whether a person sees what was done of a request.
func (d *desk) sees(n int64, person string) bool {
	d.t.Helper()
	return got[bool](d.t)(d.Sees(n, person))
}

// shows gives the texts of the entries a person sees in a request, none
// when they see none of it.
func (d *desk) shows(n int64, person string) []string {
	d.t.Helper()
	v, err := d.View(n, person)
	if errors.Is(err, requests.ErrNotThere) {
		return nil
	}
	must(d.t, err)
	var texts []string
	for _, e := range v.Entries {
		texts = append(texts, e.Text)
	}
	return texts
}

func (d *desk) state(n int64) requests.State {
	d.t.Helper()
	return got[requests.State](d.t)(d.State(n))
}

func (d *desk) thread(n int64) []requests.Entry {
	d.t.Helper()
	return got[[]requests.Entry](d.t)(d.Thread(n))
}

func (d *desk) last(n int64) requests.Entry {
	d.t.Helper()
	es := d.thread(n)
	return es[len(es)-1]
}

// on gives the names of those a state waits on.
func on(st requests.State) []string {
	var names []string
	for _, p := range st.On {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	return names
}

// holds says whether any of the texts holds the words.
func holds(texts []string, words string) bool {
	return slices.ContainsFunc(texts, func(s string) bool { return strings.Contains(s, words) })
}

// count counts the rows of a table.
func (d *desk) count(table string) int {
	d.t.Helper()
	var n int
	must(d.t, d.db.QueryRow("SELECT count(*) FROM "+table).Scan(&n))
	return n
}
