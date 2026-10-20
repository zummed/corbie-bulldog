package bridges_test

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"maps"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/instance"
	"github.com/zummed/corbie/machine"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

// folder gives the files of a bridge Bulldog ships, by their names in its
// folder: all but its entry.
func folder(t *testing.T, name string) map[string]string {
	t.Helper()
	tree, _, ok := shipped.Bridge(name)
	if !ok {
		t.Fatalf("Bulldog ships no bridge %s", name)
	}
	files := map[string]string{}
	err := fs.WalkDir(tree, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || file == shipped.Settings {
			return err
		}
		contents, err := fs.ReadFile(tree, file)
		files[file] = string(contents)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// pointed gives a shipped bridge's files with its service's hosts in place
// of a stand-in's, at an address of the test's own, reached over a plain
// connection. A bridge that reaches the site its settings give is only
// told to reach it over a plain connection: the test gives the stand-in's
// address as its site. Nothing else of them is changed.
func pointed(t *testing.T, name string, srv *httptest.Server, hosts ...string) map[string]string {
	t.Helper()
	addr := strings.TrimPrefix(srv.URL, "http://")
	files := folder(t, name)
	for file, text := range files {
		for _, host := range hosts {
			text = strings.ReplaceAll(text, "@net:"+host, "@net.unencrypted:"+addr)
			text = strings.ReplaceAll(text, "net:"+host, "net:"+addr)
			text = strings.ReplaceAll(text, "https://"+host, "http://"+addr)
		}
		text = strings.ReplaceAll(text, "@net:${input.settings.site}", "@net.unencrypted:${input.settings.site}")
		text = strings.ReplaceAll(text, `"https://" + input.settings.site`, `"http://" + input.settings.site`)
		files[file] = text
	}
	return files
}

// A service is a whole Corbie in the test's process, with a shipped
// bridge's machines pointed at a stand-in. A bridge that reaches the site
// its settings give is given site, the stand-in's address, among them.
type service struct {
	t       *testing.T
	corbie  *testkit.Corbie
	files   map[string]string
	secrets map[string]string
	site    string
}

func newService(t *testing.T, files map[string]string, secrets map[string]string) *service {
	return &service{t: t, corbie: testkit.NewCorbie(t, testkit.NewClock(testkit.Monday)), files: files, secrets: secrets}
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

// start starts one of the bridge's machines with an input, and gives the
// run.
func (s *service) start(which string, input map[string]any) string {
	s.t.Helper()
	run, err := s.corbie.Start(context.Background(), s.draft(which), client.Start{Input: s.sited(input), Secrets: s.secrets, Labels: map[string]string{"kind": "test"}})
	if err != nil {
		s.t.Fatal(err)
	}
	return run
}

// sited gives a machine's input with the service's site among its
// settings, when it has one.
func (s *service) sited(input map[string]any) map[string]any {
	settings, _ := input["settings"].(map[string]any)
	if s.site == "" || settings == nil {
		return input
	}
	out := maps.Clone(input)
	out["settings"] = maps.Clone(settings)
	out["settings"].(map[string]any)["site"] = s.site
	return out
}

// run runs one of the bridge's machines to its end.
func (s *service) run(which string, input map[string]any) *client.Run {
	s.t.Helper()
	return s.corbie.Ended(s.t, s.start(which, input))
}

// heard runs the in machine to its end, which must be well, and gives
// what it heard and the cursor it ended with.
func (s *service) heard(input map[string]any) ([]map[string]any, any) {
	s.t.Helper()
	things, cursor, _ := s.heardLogged(input)
	return things, cursor
}

// heardLogged runs the in machine to its end, which must be well, and
// gives what it heard, the cursor it ended with and the lines it logged.
func (s *service) heardLogged(input map[string]any) ([]map[string]any, any, []string) {
	s.t.Helper()
	ended := s.run("in.yaml", input)
	if ended.Status != "done" {
		s.t.Fatalf("the in run ended %s: %s", ended.Status, ended.Error)
	}
	out, _ := ended.Output.(map[string]any)
	var things []map[string]any
	for _, t := range out["heard"].([]any) {
		things = append(things, t.(map[string]any))
	}
	return things, out["cursor"], s.logged(ended.ID)
}

// logged gives the lines a run logged, as its record holds them.
func (s *service) logged(run string) []string {
	s.t.Helper()
	var lines []string
	_, err := s.corbie.Record(context.Background(), run, func(e client.Entry) {
		if e.Job != "" && e.Name == instance.EmitLog {
			lines = append(lines, fmt.Sprint(e.Data))
		}
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return lines
}

// A file is one an out run is given, as Bulldog gives it.
type file struct {
	name string
	data []byte
}

// carry runs the out machine to its end, given what Bulldog gives one: the
// text, and each file's data, as a blob of the run's input.
func (s *service) carry(input map[string]any) *client.Run {
	s.t.Helper()
	whole := map[string]any{"settings": map[string]any{}, "to": "", "direct": false, "text": "", "files": []any{}, "answers": nil, "choices": nil, "ref": nil}
	for k, v := range input {
		whole[k] = v
	}
	if files, ok := whole["files"].([]file); ok {
		given := []any{}
		for _, f := range files {
			given = append(given, map[string]any{"name": f.name, "data": client.InputFile{Name: f.name, Contents: bytes.NewReader(f.data)}})
		}
		whole["files"] = given
	}
	whole["text"] = client.InputFile{Name: "text.txt", Contents: strings.NewReader(whole["text"].(string))}
	run, err := s.corbie.Start(context.Background(), s.draft("out.yaml"), client.Start{Input: s.sited(whole), Secrets: s.secrets, Labels: map[string]string{"kind": "test"}})
	if err != nil {
		s.t.Fatal(err)
	}
	return s.corbie.Ended(s.t, run)
}

// sent gives what an out run ended with as sent, the last message of it
// when it sent several, failing the test when it did not end well with
// sent.
func sent(t *testing.T, r *client.Run) string {
	t.Helper()
	out, _ := r.Output.(map[string]any)
	if r.Status != "done" || out["sent"] == nil {
		t.Fatalf("the out run ended %s with %#v: %s", r.Status, r.Output, r.Error)
	}
	if refs, ok := out["sent"].([]any); ok {
		return refs[len(refs)-1].(string)
	}
	return out["sent"].(string)
}
