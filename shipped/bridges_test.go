package shipped_test

import (
	"fmt"
	"maps"
	"net/http/httptest"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
)

// A service is a shipped bridge pointed at a stand-in for its outside
// service, in a Corbie of the test's own.
type service struct {
	t       *testing.T
	corbie  *testkit.Corbie
	files   map[string]string
	secrets map[string]string
	// settings are the bridge's settings, given to its in and its out.
	settings map[string]any
}

// newService gives a shipped bridge with its service's hosts changed for
// the stand-in's address, reached over a plain connection. A bridge that
// reaches the site its settings give is told only to use a plain
// connection; the test gives the stand-in's address as its site.
func newService(t *testing.T, name string, srv *httptest.Server, secrets map[string]string, settings map[string]any, hosts ...string) *service {
	t.Helper()
	addr := strings.TrimPrefix(srv.URL, "http://")
	files := map[string]string{}
	for file, text := range testkit.Files(t, shipped.Files) {
		dir, base := path.Split(file)
		if dir != "bridges/"+name+"/" {
			continue
		}
		for _, host := range hosts {
			text = strings.ReplaceAll(text, "@net:"+host, "@net.unencrypted:"+addr)
			text = strings.ReplaceAll(text, "net:"+host, "net:"+addr)
			text = strings.ReplaceAll(text, "https://"+host, "http://"+addr)
		}
		text = strings.ReplaceAll(text, "@net:${input.settings.site}", "@net.unencrypted:${input.settings.site}")
		text = strings.ReplaceAll(text, `"https://" + input.settings.site`, `"http://" + input.settings.site`)
		files[base] = text
	}
	return &service{t: t, corbie: testkit.NewCorbie(t), files: files, secrets: secrets, settings: settings}
}

// draft gives one of the bridge's machines, filled out and checked.
func (s *service) draft(which string) *machine.Draft {
	s.t.Helper()
	d, err := machine.Parse([]byte(s.files[which]), machine.Given(s.files))
	if err != nil {
		s.t.Fatalf("%s: %v", which, err)
	}
	return d
}

// listen starts the bridge's in from a cursor, and cancels it when the
// test ends.
func (s *service) listen(cursor any) string {
	s.t.Helper()
	run := start(s.t, s.corbie, s.draft("in.yaml"), client.Start{Input: map[string]any{"settings": s.settings, "cursor": cursor}, Secrets: s.secrets})
	s.t.Cleanup(func() {
		if _, err := s.corbie.Cancel(ctx, run, 0); err != nil {
			s.t.Error(err)
		}
		s.corbie.Ended(s.t, run)
	})
	return run
}

// emitted gives each heard an in run has emitted, in the order its jobs
// ran. A record gives a job's emits in order, and its jobs by name, where
// hearing#2 comes before hearing.
func (s *service) emitted(run string) []map[string]any {
	s.t.Helper()
	type emit struct {
		job  int
		data map[string]any
	}
	var heard []emit
	_, err := s.corbie.Record(ctx, run, func(e client.Entry) {
		if e.Job != "" && e.Name == "heard" {
			job := 1
			if _, n, ok := strings.Cut(e.Job, "#"); ok {
				job, _ = strconv.Atoi(n)
			}
			heard = append(heard, emit{job, e.Data.(map[string]any)})
		}
	})
	if err != nil {
		s.t.Fatal(err)
	}
	slices.SortStableFunc(heard, func(a, b emit) int { return a.job - b.job })
	var data []map[string]any
	for _, h := range heard {
		data = append(data, h.data)
	}
	return data
}

// hears stays until an in run has emitted heard, and handed over at least
// n things in all, and gives them, with the cursor it emitted last.
func (s *service) hears(run string, n int) ([]map[string]any, any) {
	s.t.Helper()
	var things []map[string]any
	var cursor any
	s.corbie.Until(s.t, fmt.Sprintf("the in run hands over %d things", n), func() bool {
		heard := s.emitted(run)
		things, cursor = nil, nil
		for _, h := range heard {
			for _, th := range h["things"].([]any) {
				things = append(things, th.(map[string]any))
			}
			cursor = h["cursor"]
		}
		return len(heard) > 0 && len(things) >= n
	})
	return things, cursor
}

// logged gives the lines a run has logged.
func (s *service) logged(run string) []string {
	s.t.Helper()
	var lines []string
	_, err := s.corbie.Record(ctx, run, func(e client.Entry) {
		if e.Job != "" && e.Name == "log" {
			lines = append(lines, e.Data.(string))
		}
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return lines
}

// rests stays until an in run rests on its timer.
func (s *service) rests(run string) {
	s.t.Helper()
	s.corbie.Until(s.t, "the in run rests", func() bool {
		shown, err := s.corbie.Show(ctx, run)
		return err == nil && shown.Timers == 1 && shown.Jobs == 0
	})
}

// say runs the bridge's out to its end, given what Bulldog gives an out:
// every input, none left out.
func (s *service) say(input map[string]any) *client.Run {
	s.t.Helper()
	whole := map[string]any{"settings": s.settings, "to": "", "direct": false, "text": "", "choices": nil, "ref": nil, "answers": nil}
	maps.Copy(whole, input)
	return s.corbie.Ended(s.t, start(s.t, s.corbie, s.draft("out.yaml"), client.Start{Input: whole, Secrets: s.secrets}))
}

// sent gives the id of what an out sent, and fails the test when it did
// not send.
func sent(t *testing.T, r *client.Run) string {
	t.Helper()
	out, _ := r.Output.(map[string]any)
	id, _ := out["sent"].(string)
	if r.Status != "done" || id == "" {
		t.Fatalf("the out ended %s with %#v: %s", r.Status, r.Output, r.Error)
	}
	return id
}

// fieldsOf gives a thing's fields as one line, to compare: those named,
// in order, after a mark for each field missing or extra, since a thing
// has every field, null where it has none.
func fieldsOf(th map[string]any, names ...string) string {
	var parts []string
	for _, f := range []string{"id", "at", "kind", "from", "place", "where", "text", "files", "mentioned", "answers", "ref", "choice"} {
		if _, ok := th[f]; !ok {
			parts = append(parts, "missing "+f)
		}
	}
	if len(th) != 12 {
		parts = append(parts, fmt.Sprintf("%d fields", len(th)))
	}
	for _, f := range names {
		parts = append(parts, fmt.Sprint(th[f]))
	}
	return strings.Join(parts, " | ")
}
