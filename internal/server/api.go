package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// A site is what the server answers: the screens, signing in, the API
// and the address of each bridge that hears.
type site struct {
	Areas
	db    *sql.DB
	clock work.Clock
	// url is how a person reaches the server, with no / at its end.
	url string
	say func(format string, args ...any)
	// screens are the screens following the stream of changes.
	screens *screens
	// asked are the names a one-time link was asked for, which are
	// looked at one after another, apart from the asking.
	asked chan string
	// version is which Bulldog serves the office, and before the one that
	// ran before it, "" when none did.
	version, before string
}

// The most a body is read of: JSON, and a form with files.
const (
	MaxJSON = 1 << 20
	MaxForm = 32 << 20
)

// The sentences the API answers with that are the same whoever asks.
const (
	notThere = "There is nothing here by that name."
	noKey    = "You are not signed in: open the link the server printed for you, or ask for a one-time link on the sign-in page at /."
)

// routes gives what the server answers. Only the screens' first page, its
// own files, the sign-in and the address of each bridge that hears are
// answered without a key.
// after gives a channel told once a time has passed by the office's
// clock: the computer's own timer for the wall clock, which is told to
// move only once a second, and one that moves with any other, as a
// test's, which then never waits on the computer's.
func after(clock work.Clock) func(time.Duration) <-chan time.Time {
	if _, wall := clock.(work.Wall); wall {
		return time.After
	}
	return func(d time.Duration) <-chan time.Time {
		at, passed := clock.Now().Add(d), make(chan time.Time, 1)
		moved, unwatch := clock.Watch()
		go func() {
			defer unwatch()
			for clock.Now().Before(at) {
				<-moved
			}
			passed <- clock.Now()
		}()
		return passed
	}
}

func routes(s *site) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /static/{name}", static)
	mux.HandleFunc("POST /signin", s.askLink)
	mux.HandleFunc("GET /signin/{link}", s.linkPage)
	mux.HandleFunc("POST /signin/{link}", s.pressLink)
	mux.Handle("POST /bridges/{name}", &bridges.Address{Bridges: s.Bridges, Hear: s.Work.Hear, After: after(s.clock)})

	mux.HandleFunc("GET /api/me", s.api(s.me))
	mux.HandleFunc("POST /api/me/key", s.api(s.replaceKey))
	mux.HandleFunc("GET /api/requests", s.api(s.list))
	mux.HandleFunc("POST /api/requests", s.api(s.ask))
	mux.HandleFunc("GET /api/requests/{n}", s.api(s.request))
	mux.HandleFunc("POST /api/requests/{n}/entries", s.api(s.sayMore))
	mux.HandleFunc("POST /api/requests/{n}/decisions/{id}", s.api(s.decide))
	mux.HandleFunc("POST /api/requests/{n}/stop", s.api(s.stop))
	mux.HandleFunc("GET /api/teams/{team}", s.api(s.team))
	mux.HandleFunc("GET /api/teams/{team}/docs/{doc...}", s.api(s.doc))
	mux.HandleFunc("POST /api/teams/{team}/routines/now", s.api(s.routineNow))
	mux.HandleFunc("GET /api/shipped", s.api(s.shipped))
	mux.HandleFunc("GET /api/defined", s.api(s.defined))
	mux.HandleFunc("GET /api/manual", s.api(s.manualPages))
	mux.HandleFunc("GET /api/manual/{page...}", s.api(s.manualPage))
	mux.HandleFunc("GET /api/office/files", s.api(s.files))
	mux.HandleFunc("GET /api/office/file", s.api(s.file))
	mux.HandleFunc("POST /api/office/check", s.api(s.check))
	mux.HandleFunc("POST /api/office/put", s.api(s.put))
	mux.HandleFunc("POST /api/office/changes/{id}", s.api(s.decideOwn))
	mux.HandleFunc("GET /api/history", s.api(s.history))
	mux.HandleFunc("GET /api/history/{id}", s.api(s.commit))
	mux.HandleFunc("GET /api/events", s.keyed(s.events))
	mux.HandleFunc("GET /api/runs/{run}", s.api(s.run))
	mux.HandleFunc("GET /api/runs/{run}/watch", s.keyed(s.watch))
	mux.HandleFunc("GET /files/{id}", s.keyed(s.entryFile))
	mux.HandleFunc("/", s.keyed(func(person string, w http.ResponseWriter, r *http.Request) {
		answer(w, http.StatusNotFound, notThere)
	}))
	return guarded(mux)
}

// guarded serves every answer with a policy that lets a page load nothing
// from anywhere but the office, and lets nothing be read as another type.
func guarded(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		h.ServeHTTP(w, r)
	})
}

// policy lets a page load only from the office itself.
const policy = "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"

// keyed gives a handler that is answered only for a person signed in, as
// that person: anyone else is told how to sign in.
func (s *site) keyed(h func(person string, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		person, err := s.who(r)
		switch {
		case err != nil:
			s.failed(w, r, err)
		case person == "":
			answer(w, http.StatusUnauthorized, noKey)
		default:
			h(person, w, r)
		}
	}
}

// api gives a handler of the API, which answers with what it gives, as
// JSON, or with why not.
func (s *site) api(h func(person string, w http.ResponseWriter, r *http.Request) (any, error)) http.HandlerFunc {
	return s.keyed(func(person string, w http.ResponseWriter, r *http.Request) {
		out, err := h(person, w, r)
		if err != nil {
			s.refuse(w, r, err)
			return
		}
		var b bytes.Buffer
		if err := json.NewEncoder(&b).Encode(out); err != nil {
			s.failed(w, r, fmt.Errorf("the answer cannot be written as JSON: %w", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b.Bytes())
	})
}

// answer answers with a sentence a person can read, as JSON.
func answer(w http.ResponseWriter, status int, sentence string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": sentence})
}

// A refusal is something the server's own rules do not allow, said in a
// sentence for whoever tried.
type refusal string

func (r refusal) Error() string { return string(r) }

// A badBody is a body that is not taken, with the status that says why:
// one that cannot be read, or one that is too large.
type badBody struct {
	status int
	why    string
}

func (b *badBody) Error() string { return b.why }

func unreadable(format string, args ...any) error {
	return &badBody{status: http.StatusBadRequest, why: fmt.Sprintf(format, args...)}
}

// errNotThere is what anything the one looking may not see is, as much as
// anything that is not there.
var errNotThere = errors.New(notThere)

// refuse answers with why something was not done: what is not there, or
// may not be seen, alike; what the rules refuse; a body not taken; and
// anything else as the failure it is, which is also logged.
func (s *site) refuse(w http.ResponseWriter, r *http.Request, err error) {
	var bad *badBody
	var theirs requests.Refusal
	var change changes.Refusal
	var ours refusal
	switch {
	case errors.As(err, &bad):
		answer(w, bad.status, bad.why)
	case errors.Is(err, errNotThere), errors.Is(err, requests.ErrNotThere), errors.Is(err, work.ErrNoWatch), errors.Is(err, decisions.ErrNoDecision),
		errors.Is(err, changes.ErrNotThere):
		answer(w, http.StatusNotFound, notThere)
	case errors.As(err, &theirs):
		answer(w, http.StatusConflict, string(theirs))
	case errors.As(err, &change):
		answer(w, http.StatusConflict, string(change))
	case errors.As(err, &ours):
		answer(w, http.StatusConflict, string(ours))
	default:
		s.failed(w, r, err)
	}
}

// failed answers a failure as one, and logs it, with the address it was
// asked at, but for a one-time link, which is said by its pattern.
func (s *site) failed(w http.ResponseWriter, r *http.Request, err error) {
	at := r.URL.Path
	if r.PathValue("link") != "" {
		_, at, _ = strings.Cut(r.Pattern, " ")
	}
	s.say("a failure in answering %s %s: %v", r.Method, at, err)
	answer(w, http.StatusInternalServerError, "Bulldog failed, and the failure is in the server's log: "+err.Error())
}

// readJSON reads a body of JSON whole into what it is to be: one value,
// with no field it does not take, no field given twice, each field of the
// kind it takes, and no more than MaxJSON. A body that is not so is taken
// in no part.
func readJSON(w http.ResponseWriter, r *http.Request, into any) error {
	body := http.MaxBytesReader(w, r.Body, MaxJSON)
	data, err := io.ReadAll(body)
	if err != nil {
		return tooLarge(err, MaxJSON)
	}
	if t, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); t != "application/json" {
		return unreadable("The body is to be JSON, sent as application/json.")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return unreadable("The body cannot be read: %v.", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return unreadable("The body holds more than one value.")
	}
	if name := twice(data); name != "" {
		return unreadable("The body gives %s twice.", name)
	}
	return nil
}

// twice gives the first field that one object of a body of JSON, well
// formed, gives twice, or "".
func twice(data []byte) string {
	type level struct {
		object, key bool
		given       map[string]bool
	}
	var in []*level
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		var at *level
		if len(in) > 0 {
			at = in[len(in)-1]
		}
		if at != nil && at.key {
			if tok == json.Delim('}') {
				in = in[:len(in)-1]
				continue
			}
			name, _ := tok.(string)
			if at.given[name] {
				return name
			}
			at.given[name], at.key = true, false
			continue
		}
		if at != nil && at.object {
			at.key = true
		}
		switch tok {
		case json.Delim('{'):
			in = append(in, &level{object: true, key: true, given: map[string]bool{}})
		case json.Delim('['):
			in = append(in, &level{})
		case json.Delim(']'):
			in = in[:len(in)-1]
		}
	}
}

// readNothing reads a body that is to say nothing: empty, or {}.
func readNothing(w http.ResponseWriter, r *http.Request) error {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxJSON))
	if err != nil {
		return tooLarge(err, MaxJSON)
	}
	if t := strings.TrimSpace(string(data)); t != "" && t != "{}" {
		return unreadable("This takes no body, and was given one.")
	}
	return nil
}

// tooLarge gives what a body that could not be read whole is.
func tooLarge(err error, limit int64) error {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		return &badBody{status: http.StatusRequestEntityTooLarge, why: fmt.Sprintf("The body is larger than %s, the most taken, so none of it is kept.", mib(limit))}
	}
	return unreadable("The body cannot be read: %v.", err)
}

func mib(n int64) string { return fmt.Sprintf("%d MiB", n>>20) }

// A said is what a person says with files or none: a body of JSON, or a
// form of several parts, its fields and its files together. Each field is
// kept as its kind, and a field not given is not there.
type said struct {
	texts   map[string]string
	numbers map[string]int64
	files   []requests.File
}

// The kinds of field a said takes.
const (
	kindText   = "text"
	kindNumber = "number"
)

// readSaid reads a body that may carry files: JSON with the fields it
// takes, each of its kind, or a form of several parts holding them as
// text, and each file as a part named files. Any other field, a field
// given twice, one of the wrong kind, or two files of one name, is
// refused.
func readSaid(w http.ResponseWriter, r *http.Request, kinds map[string]string) (said, error) {
	out := said{texts: map[string]string{}, numbers: map[string]int64{}}
	if t, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); t != "multipart/form-data" {
		var fields map[string]json.RawMessage
		if err := readJSON(w, r, &fields); err != nil {
			return said{}, err
		}
		for name, raw := range fields {
			if kinds[name] == "" {
				return said{}, unreadable("The body gives %s, which is not taken here.", name)
			}
			if err := out.take(name, kinds[name], raw); err != nil {
				return said{}, err
			}
		}
		return out, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxForm)
	parts, err := r.MultipartReader()
	if err != nil {
		return said{}, unreadable("The form cannot be read: %v.", err)
	}
	for {
		part, err := parts.NextPart()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return said{}, tooLarge(err, MaxForm)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return said{}, tooLarge(err, MaxForm)
		}
		name, file := part.FormName(), part.FileName()
		switch {
		case name == "files" && file != "":
			if slices.ContainsFunc(out.files, func(f requests.File) bool { return f.Name == file }) {
				return said{}, unreadable("The form gives two files named %s: give each file a name of its own.", file)
			}
			out.files = append(out.files, requests.File{Name: file, Data: data})
		case kinds[name] == "" || file != "":
			return said{}, unreadable("The form gives %s, which is not taken here.", name)
		case kinds[name] == kindNumber:
			n, err := strconv.ParseInt(string(data), 10, 64)
			if err != nil {
				return said{}, unreadable("The form's %s is not a number.", name)
			}
			if err := out.give(name, nil, &n); err != nil {
				return said{}, err
			}
		default:
			text := string(data)
			if err := out.give(name, &text, nil); err != nil {
				return said{}, err
			}
		}
	}
}

// take takes a field of JSON as its kind.
func (s said) take(name, kind string, raw json.RawMessage) error {
	var err error
	switch kind {
	case kindText:
		var text string
		if err = json.Unmarshal(raw, &text); err == nil && string(raw) != "null" {
			return s.give(name, &text, nil)
		}
	case kindNumber:
		var n int64
		if err = json.Unmarshal(raw, &n); err == nil && string(raw) != "null" {
			return s.give(name, nil, &n)
		}
	}
	return unreadable("The body's %s is not a %s.", name, kind)
}

// give keeps a field, which is refused when it is given twice.
func (s said) give(name string, text *string, n *int64) error {
	_, isText := s.texts[name]
	_, isNumber := s.numbers[name]
	if isText || isNumber {
		return unreadable("The form gives %s twice.", name)
	}
	if text != nil {
		s.texts[name] = *text
	} else {
		s.numbers[name] = *n
	}
	return nil
}

// number reads a number from the address: one that is no number is
// nothing there.
func number(r *http.Request, name string) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || n <= 0 {
		return 0, errNotThere
	}
	return n, nil
}
