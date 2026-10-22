// Package screens serves the screens: the pages, the API behind them, and
// signing in. Each answer shows only what the person looking may see.
package screens

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
)

// Bridges is what the screens use of the bridges: carrying a message out,
// as a sign-in link is, and how each bridge's in run stands. The program
// gives the screens the office's bridges as they are.
type Bridges interface {
	requests.Carrier
	State(bridge string) bridges.State
}

// The screens' own files: the page, its styles, scripts and fonts.
//
//go:embed static
var static embed.FS

// policy lets a page load only from the screens themselves.
const policy = "default-src 'self'"

// Screens serves the screens of one office to its people.
type Screens struct {
	corbie  *client.Client
	engine  *requests.Engine
	bridges Bridges
	secret  []byte
	address string
	handler http.Handler
	sending sync.WaitGroup // the sign-in links being carried

	// Report is told what went wrong that no answer says, for a person to
	// read. By default it is written to the standard log.
	Report func(error)
	// Interval is how often a stream of changes looks at the store when
	// nothing written in this process wakes it, and looks for the office
	// read again.
	Interval time.Duration
}

// New makes the screens of the office an engine drives, as a client of its
// Corbie, with the office's secret. The office is read from the engine's
// reading each time it is asked for, so it follows the office as it is
// read again. Links in messages start with the address the screens are
// reached at, such as https://bulldog.example.
func New(corbie *client.Client, engine *requests.Engine, bridges Bridges, secret []byte, address string) *Screens {
	s := &Screens{corbie: corbie, engine: engine, bridges: bridges, secret: secret, address: strings.TrimSuffix(address, "/"),
		Report: func(err error) { log.Print(err) }, Interval: 5 * time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", page("index.html"))
	mux.HandleFunc("GET /static/{name}", file)

	mux.HandleFunc("GET /signin", s.signInPage)
	mux.HandleFunc("POST /signin", s.askLink)
	mux.HandleFunc("GET /signin/link", s.linkPage)
	mux.HandleFunc("POST /signin/link", s.pressLink)
	mux.HandleFunc("GET /key", s.keyPage)
	mux.HandleFunc("POST /key", s.pressKey)
	mux.HandleFunc("POST /signout", s.signOut)

	mux.HandleFunc("GET /api/me", s.api(s.me))
	mux.HandleFunc("GET /api/requests", s.api(s.list))
	mux.HandleFunc("POST /api/requests", s.api(s.ask))
	mux.HandleFunc("GET /api/requests/{id}", s.api(s.request))
	mux.HandleFunc("GET /api/requests/{id}/files/{n}", s.keyed(s.file))
	mux.HandleFunc("POST /api/requests/{id}/say", s.api(s.say))
	mux.HandleFunc("POST /api/requests/{id}/stop", s.api(s.stop))
	mux.HandleFunc("POST /api/waits/{ref}/answer", s.api(s.answer))
	mux.HandleFunc("GET /api/office", s.api(s.office))
	mux.HandleFunc("GET /api/teams/{team}", s.api(s.team))
	mux.HandleFunc("GET /api/teams/{team}/docs/{name}", s.api(s.readDoc))
	mux.HandleFunc("PUT /api/teams/{team}/docs/{name}", s.api(s.writeDoc))
	mux.HandleFunc("GET /api/events", s.keyed(s.events))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		answer(w, http.StatusNotFound, "There is nothing here.")
	})

	// A POST or a PUT that a page of another site sends is refused, so
	// that no other site acts as the one signed in, or signs a browser in
	// as someone else.
	guard := http.NewCrossOriginProtection()
	guard.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer(w, http.StatusForbidden, "The screens refuse what a page of another site sends.")
	}))
	s.handler = guard.Handler(mux)
	return s
}

// ServeHTTP answers a request of the screens. Every answer is served with
// a policy that lets a page load nothing from any other site, and lets
// nothing be read as another type than it is served as.
func (s *Screens) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", policy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.handler.ServeHTTP(w, r)
}

// types are the types of the screens' own files, by their ending.
var types = map[string]string{
	".html":  "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".woff2": "font/woff2",
}

// page serves one of the screens' own files.
func page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(static, "static/"+name)
		kind, ok := types[path.Ext(name)]
		if err != nil || !ok {
			answer(w, http.StatusNotFound, "There is nothing here.")
			return
		}
		w.Header().Set("Content-Type", kind)
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	}
}

// file serves one of the screens' own files by the name in its address.
func file(w http.ResponseWriter, r *http.Request) {
	page(r.PathValue("name"))(w, r)
}

// A refusal is something not done, with the status that says why and a
// sentence a person can read.
type refusal struct {
	status int
	text   string
}

func (r refusal) Error() string { return r.text }

// The sentences of what is not there. What the one looking may not see is
// answered with the same sentence.
var (
	noRequest  = refusal{http.StatusNotFound, "There is no such request."}
	noTeam     = refusal{http.StatusNotFound, "There is no such team."}
	noDoc      = refusal{http.StatusNotFound, "There is no such doc."}
	noFile     = refusal{http.StatusNotFound, "There is no such file."}
	noQuestion = refusal{http.StatusNotFound, "There is no such question waiting on you."}
)

// A handler of the API is given the request and who is looking, and gives
// what is answered, as JSON, or why not.
type handler func(r *http.Request, me string) (any, error)

// api answers a request of the API for a person signed in.
func (s *Screens) api(h handler) http.HandlerFunc {
	return s.keyed(func(w http.ResponseWriter, r *http.Request, me string) {
		out, err := h(r, me)
		if err != nil {
			s.refuse(w, err)
			return
		}
		data, err := json.Marshal(out)
		if err != nil {
			s.refuse(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
	})
}

// keyed answers a request only for a person signed in, as that person.
func (s *Screens) keyed(h func(w http.ResponseWriter, r *http.Request, me string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me := s.who(r)
		if me == "" {
			answer(w, http.StatusUnauthorized, "Sign in first.")
			return
		}
		h(w, r, me)
	}
}

// refuse answers with why something was not done. What is not there, or
// may not be seen, is answered alike; anything else is a failure, which
// is also reported, but for a request the browser gave up on.
func (s *Screens) refuse(w http.ResponseWriter, err error) {
	var r refusal
	switch {
	case errors.As(err, &r):
		answer(w, r.status, r.text)
	case errors.Is(err, requests.ErrNoRequest):
		answer(w, noRequest.status, noRequest.text)
	case errors.Is(err, requests.ErrNoQuestion):
		answer(w, noQuestion.status, noQuestion.text)
	default:
		if !errors.Is(err, context.Canceled) {
			s.Report(err)
		}
		answer(w, http.StatusInternalServerError, "Bulldog failed: "+err.Error())
	}
}

// answer answers with a sentence a person can read, as JSON.
func answer(w http.ResponseWriter, status int, sentence string) {
	data, _ := json.Marshal(map[string]string{"error": sentence})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write(data)
}

// readJSON reads a body of JSON into what it is to be, taking no more than
// a limit.
func readJSON(r *http.Request, limit int64, into any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return refusal{http.StatusBadRequest, "The body is not the JSON this takes: " + err.Error() + "."}
	}
	return nil
}
