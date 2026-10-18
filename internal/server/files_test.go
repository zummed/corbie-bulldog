package server

import (
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// fileOn gives the address of the first file on a request's entry, as
// the one looking is given it.
func fileOn(t *testing.T, br *browser, n int64, name string) string {
	t.Helper()
	var v struct{ Entries []entryOut }
	br.get("/api/requests/" + itoa(n)).is(http.StatusOK).into(&v)
	for _, e := range v.Entries {
		for _, f := range e.Files {
			if f.Name == name {
				return f.URL
			}
		}
	}
	t.Fatalf("there is no file %s in request %d", name, n)
	return ""
}

func TestServerFileOnAnEntry(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	photo := []byte("\x89PNG\r\n\x1a\n the socket")
	n := numbered(s.as("kit").form("/api/requests", map[string]string{"text": "this socket"}, map[string][]byte{"socket.png": photo}))
	url := fileOn(t, s.as("kit"), n, "socket.png")
	for _, person := range []string{"kit", "sam"} {
		if r := s.as(person).get(url).is(http.StatusOK); string(r.body) != string(photo) {
			t.Errorf("%s is given the photo as %q", person, r.body)
		}
	}

	d := refundDay(t, refundOffice(t))
	d.s.as("lee").form("/api/requests/"+itoa(d.payments)+"/entries", map[string]string{"text": "the ledger's export"}, map[string][]byte{"charges.csv": []byte("ch_1,40\nch_2,40\n")}).is(http.StatusOK)
	export := fileOn(t, d.s.as("lee"), d.payments, "charges.csv")
	for person, status := range map[string]int{"lee": http.StatusOK, "priya": http.StatusOK, "al": http.StatusNotFound, "dana": http.StatusNotFound} {
		d.s.as(person).get(export).is(status)
	}
}

func TestServerFilesServedHarmlessly(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	files := map[string][]byte{
		"photo.png":  []byte("\x89PNG\r\n\x1a\n a picture"),
		"page.png":   []byte("<!doctype html><script>fetch('/api/me')</script>"),
		"logo.svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"report.pdf": []byte("%PDF-1.4\n%a report\n"),
		"notes.txt":  []byte("plain words"),
	}
	n := numbered(s.as("kit").form("/api/requests", map[string]string{"text": "files"}, files))
	shown := map[string]string{"photo.png": "image/png", "report.pdf": "application/pdf"}
	for name := range files {
		r := s.as("kit").get(fileOn(t, s.as("kit"), n, name)).is(http.StatusOK)
		h := r.header
		if h.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(h.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s is served with %v", name, h)
		}
		disposition := h.Get("Content-Disposition")
		if kind, ok := shown[name]; ok {
			if h.Get("Content-Type") != kind || !strings.HasPrefix(disposition, "inline") {
				t.Errorf("%s is served as %s, %s", name, h.Get("Content-Type"), disposition)
			}
			continue
		}
		if h.Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(disposition, "attachment") || !strings.Contains(disposition, name) {
			t.Errorf("%s is served as %s, %s", name, h.Get("Content-Type"), disposition)
		}
	}
}

func TestServerScreensFromTheProgram(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	r := s.as("sam").get("/").is(http.StatusOK)
	if !strings.Contains(r.header.Get("Content-Type"), "text/html") || !strings.Contains(string(r.body), `<script src="/static/app.js"></script>`) {
		t.Errorf("the screens are %s", r.body)
	}
	for _, name := range []string{"app.js", "style.css", "signin.js"} {
		if got := s.nobody().get("/static/" + name).is(http.StatusOK); len(got.body) == 0 {
			t.Errorf("%s is empty", name)
		}
	}
	s.nobody().get("/static/").is(http.StatusUnauthorized)
	s.nobody().get("/static/nothing.js").is(http.StatusNotFound)
}

// The screens' scripts are read as a browser reads them, and each element
// they look for by its id is on the page they run in: a mistake in them
// would show nothing, and fail no other test.
func TestServerScreensScriptsRead(t *testing.T) {
	t.Parallel()
	err := fs.WalkDir(screenFiles, "screens", func(name string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(name, ".js") {
			return err
		}
		text, err := fs.ReadFile(screenFiles, name)
		if err != nil {
			return err
		}
		if _, err := goja.Compile(name, string(text), true); err != nil {
			t.Errorf("%s does not read: %v", name, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	app, _ := fs.ReadFile(screenFiles, "screens/app.js")
	page, _ := fs.ReadFile(screenFiles, "screens/index.html")
	for _, id := range regexp.MustCompile(`getElementById\("([^"]+)"\)`).FindAllStringSubmatch(string(app), -1) {
		if !strings.Contains(string(page), `id="`+id[1]+`"`) {
			t.Errorf("app.js looks for #%s, which is not on the screens' page", id[1])
		}
	}
}

func TestServerScreensLoadNothingFromElsewhere(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	err := fs.WalkDir(screenFiles, "screens", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(screenFiles, path)
		if strings.Contains(string(data), "://") || strings.Contains(string(data), `"//`) || strings.Contains(string(data), "@import") {
			t.Errorf("%s names another site", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []reply{s.as("sam").get("/"), s.nobody().get("/"), s.nobody().get("/signin/0123"), s.nobody().get("/static/app.js"), s.as("sam").get("/api/me")} {
		policy := page.header.Get("Content-Security-Policy")
		for _, part := range []string{"default-src 'none'", "script-src 'self'", "style-src 'self'", "font-src 'self'", "connect-src 'self'", "frame-ancestors 'none'"} {
			if !strings.Contains(policy, part) {
				t.Errorf("a page is served with the policy %q", policy)
			}
		}
	}
}

func TestServerOnlySignInAndBridgesOpen(t *testing.T) {
	t.Parallel()
	s := newBench(t, house(t)).serve()
	keyed := []string{
		"GET /api/me", "POST /api/me/key", "GET /api/requests", "POST /api/requests", "GET /api/requests/1", "POST /api/requests/1/entries",
		"POST /api/requests/1/decisions/1", "POST /api/requests/1/stop", "GET /api/teams/office", "POST /api/teams/office/routines/now",
		"GET /api/office/files", "GET /api/office/file?path=office.yaml", "GET /api/office/bridges", "POST /api/office/check", "POST /api/office/put",
		"GET /api/history", "GET /api/history/abc", "GET /api/events", "GET /api/runs/abc/watch", "GET /files/1-0",
		"GET /api/nothing", "DELETE /api/me", "PUT /api/requests", "GET /anything/else", "GET /signin", "GET /bridges",
	}
	for _, route := range keyed {
		method, path, _ := strings.Cut(route, " ")
		r := s.nobody().do(method, path, "application/json", []byte("{}"))
		if r.status != http.StatusUnauthorized || r.header.Get("Content-Type") != "application/json" {
			t.Errorf("%s with no key answered %d %s", route, r.status, r.body)
		}
	}
	for _, route := range []string{"GET /", "GET /static/app.js", "POST /signin", "GET /signin/abc", "POST /signin/abc", "POST /bridges/telegram"} {
		method, path, _ := strings.Cut(route, " ")
		body := []byte("{}")
		if path == "/signin" {
			body = []byte(`{"name": "sam"}`)
		}
		if r := s.nobody().do(method, path, "application/json", body); r.status == http.StatusUnauthorized {
			t.Errorf("%s with no key answered %d", route, r.status)
		}
	}
}
