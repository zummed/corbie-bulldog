package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/store"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

func TestMain(m *testing.M) { testkit.Main(m) }

var ctx = context.Background()

// house gives the files of the house, and workOffice those of the work
// office, each bridge a stand-in that reaches no service: the house's
// Telegram listens and says things to a person directly, the work
// office's Slack hears and does too, and its GitLab listens and has no
// direct place.
func house(t *testing.T) map[string]string {
	files := without(testkit.Files(t, examples.House()), "bridges/telegram")
	maps.Copy(files, standin.In("telegram", standin.Folder(false, true)))
	return files
}

func workOffice(t *testing.T) map[string]string {
	files := without(without(testkit.Files(t, examples.Work()), "bridges/slack"), "bridges/gitlab")
	maps.Copy(files, standin.In("slack", standin.Folder(true, true)))
	maps.Copy(files, standin.In("gitlab", standin.Folder(false, false)))
	return files
}

// newHouse gives the files of the office written into an empty folder.
func newHouse() map[string]string {
	files := map[string]string{}
	for name, contents := range examples.NewHouse() {
		files[name] = string(contents)
	}
	return files
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

// A model is a model a definition or a team file writes in a row.
var model = regexp.MustCompile(`(?m)^(\s*(?:usual|model)): \{[^}\n]*\}`)

// withModel gives files with every model written in them the stand-in.
func withModel(files map[string]string, m *testkit.Model) map[string]string {
	out := map[string]string{}
	for name, text := range files {
		out[name] = model.ReplaceAllString(text, "${1}: "+m.Line())
	}
	return out
}

// A bench is a Corbie in the test's process with a stand-in model, on a
// clock the test moves, and an office's folder, which it serves as the
// program does, as often as the test asks.
type bench struct {
	t      *testing.T
	clock  *testkit.Clock
	corbie *testkit.Corbie
	model  *testkit.Model
	dir    string

	mu     sync.Mutex
	answer func(testkit.Turn) testkit.Reply
	// told is sent to whenever a server logs, begins or ends.
	told chan struct{}
	// heard, when it is set, is told each line a server logs as it logs
	// it.
	heard func(line string)
}

// newBench makes a bench whose folder holds the files given, every model
// in them the stand-in, and none of Bulldog's own yet.
func newBench(t *testing.T, files map[string]string) *bench {
	t.Helper()
	b := &bench{t: t, clock: testkit.NewClock(testkit.Monday), told: make(chan struct{}, 1)}
	b.answer = func(testkit.Turn) testkit.Reply { return testkit.Say("done") }
	b.model = testkit.NewModel(t, func(turn testkit.Turn) testkit.Reply {
		b.mu.Lock()
		answer := b.answer
		b.mu.Unlock()
		return answer(turn)
	})
	b.corbie = testkit.NewCorbie(t, b.clock)
	b.dir = filepath.Join(t.TempDir(), "house")
	if files != nil {
		testkit.Write(t, b.dir, withModel(files, b.model))
	}
	return b
}

// says has the stand-in answer every turn so.
func (b *bench) says(answer func(testkit.Turn) testkit.Reply) {
	b.mu.Lock()
	b.answer = answer
	b.mu.Unlock()
}

func (b *bench) poke() {
	select {
	case b.told <- struct{}{}:
	default:
	}
}

// write writes files into the folder, the stand-in's model in them, and
// removes those given as "".
func (b *bench) write(files map[string]string) {
	b.t.Helper()
	write := map[string]string{}
	for name, text := range files {
		if text == "" {
			if err := os.Remove(filepath.Join(b.dir, filepath.FromSlash(name))); err != nil {
				b.t.Fatal(err)
			}
			continue
		}
		write[name] = text
	}
	testkit.Write(b.t, b.dir, withModel(write, b.model))
}

// until stays until done says so, looking again whenever Corbie's store is
// written, the clock moves or a server logs, begins or ends.
func (b *bench) until(what string, done func() bool) {
	b.t.Helper()
	written, stopWritten := b.corbie.Memory.As(store.Party{Kind: store.Owner}).Notice("")
	defer stopWritten()
	moved, stopMoved := b.clock.Watch()
	defer stopMoved()
	limit := time.NewTimer(testkit.Patience)
	defer limit.Stop()
	for !done() {
		select {
		case <-written:
		case <-moved:
		case <-b.told:
		case <-limit.C:
			b.t.Fatalf("waited %v, and still not: %s", testkit.Patience, what)
		}
	}
}

// store opens the bench's store beside its server, as one more reader of
// the same file.
func (b *bench) store() *sql.DB {
	b.t.Helper()
	db, err := OpenStore(b.dir)
	if err != nil {
		b.t.Fatal(err)
	}
	b.t.Cleanup(func() { db.Close() })
	return db
}

// A serving is one server of the bench's folder, started as the program
// starts one.
type serving struct {
	b      *bench
	opt    Options
	cancel func()
	done   chan struct{}
	err    error

	mu    sync.Mutex
	log   bytes.Buffer
	areas *Areas
}

func (s *serving) Write(p []byte) (int, error) {
	if s.b.heard != nil {
		s.b.heard(string(p))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.b.poke()
	return s.log.Write(p)
}

// lines gives what the server has logged, a line to each.
func (s *serving) lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Split(strings.TrimSuffix(s.log.String(), "\n"), "\n")
}

func (s *serving) logged(part string) bool {
	return slices.ContainsFunc(s.lines(), func(l string) bool { return strings.Contains(l, part) })
}

// begun gives the areas once the server has begun, or nil.
func (s *serving) begun() *Areas {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.areas
}

func (s *serving) ended() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// serve readies the bench's folder and serves it, as the program does with
// the folder named, listening anywhere, and stays until the server has
// begun or ended.
func (b *bench) serve(change ...func(*Options)) *serving {
	b.t.Helper()
	s := b.start(change...)
	b.until("the server has begun or ended", func() bool { return s.begun() != nil || s.ended() })
	// An office that reads is served once every area has taken its
	// reading, which the links say.
	if a := s.begun(); a != nil && a.Keeper.Office() != nil {
		b.until("the links are given", func() bool {
			return !slices.ContainsFunc(a.Keeper.Office().Everyone, func(person string) bool { return !s.logged(person + ", open http") })
		})
	}
	return s
}

// start readies the bench's folder and starts its server, without
// waiting for it.
func (b *bench) start(change ...func(*Options)) *serving {
	b.t.Helper()
	s := &serving{b: b, done: make(chan struct{})}
	s.opt = Options{Dir: b.dir, Corbie: b.corbie.Client, Store: "memory", Clock: b.clock, Listen: "127.0.0.1:0", Out: s,
		Begun: func(a Areas) {
			a.Requests.Watch(func(int64) { b.poke() })
			s.mu.Lock()
			s.areas = &a
			s.mu.Unlock()
			b.poke()
		}}
	for _, c := range change {
		c(&s.opt)
	}
	id, err := Ready(b.dir, Readying{Make: true}, b.clock.Now(), func(format string, args ...any) { fmt.Fprintf(s, "bulldog: "+format+"\n", args...) })
	if err != nil {
		s.err = err
		close(s.done)
		return s
	}
	s.opt.Identity = id
	serveCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go func() {
		defer b.poke()
		defer close(s.done)
		s.err = Serve(serveCtx, s.opt)
	}()
	b.t.Cleanup(func() {
		s.stop()
		if b.t.Failed() {
			b.t.Logf("the log of a server:\n%s", strings.Join(s.lines(), "\n"))
		}
	})
	return s
}

// stop stops the server, as a signal does, and gives how it ended.
func (s *serving) stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
	return s.err
}

// wait stays until the server has ended of itself, and gives how.
func (s *serving) wait() error {
	s.b.t.Helper()
	s.b.until("the server has ended", s.ended)
	return s.err
}

// areasOf gives the areas of a server that has begun.
func (s *serving) areasOf() *Areas {
	s.b.t.Helper()
	a := s.begun()
	if a == nil {
		s.b.t.Fatalf("the server has not begun: %v\n%s", s.err, strings.Join(s.lines(), "\n"))
	}
	return a
}

// chat is a request a person asks in the chat, of a team or the office.
func (s *serving) chat(person, team, text string) int64 {
	s.b.t.Helper()
	r, err := s.areasOf().Requests.AskInChat(person, team, text, nil)
	if err != nil {
		s.b.t.Fatal(err)
	}
	return r.Number
}

// get gives a request.
func (s *serving) get(n int64) *requests.Request {
	s.b.t.Helper()
	r, err := s.areasOf().Requests.Get(n)
	if err != nil {
		s.b.t.Fatal(err)
	}
	return r
}

// answered stays until a request is answered.
func (s *serving) answered(n int64) {
	s.b.t.Helper()
	s.b.until(fmt.Sprintf("request %d is answered", n), func() bool {
		st, err := s.areasOf().Requests.State(n)
		return err == nil && st.Is == requests.Answered
	})
}

// listening stays until a server has started its bridges' runs, which it
// does once it has begun: one stopped before could leave a run arriving
// after it has ended.
func (b *bench) listening(s *serving) *serving {
	b.t.Helper()
	b.until("the bridges listen", func() bool {
		return len(b.listed(b.identity(), map[string]string{work.LabelKind: bridges.KindIn})) > 0
	})
	return s
}

// listed gives the runs at the top labelled with an office's identity and
// the labels given, those ended among them.
func (b *bench) listed(id string, labels map[string]string) []*client.Run {
	b.t.Helper()
	filter := map[string]string{work.LabelBulldog: id}
	maps.Copy(filter, labels)
	runs, err := b.corbie.List(ctx, client.Filter{Labels: filter, Finished: true})
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(runs, func(r *client.Run) bool { return r.Parent != "" })
}

// identity gives the office's identity, as its folder keeps it.
func (b *bench) identity() string {
	b.t.Helper()
	id, err := ReadIdentity(b.dir)
	if err != nil {
		b.t.Fatal(err)
	}
	return id.ID
}

// leftLive starts a run of the office's own live machine, as a server
// that was killed leaves one.
func (b *bench) leftLive() string {
	b.t.Helper()
	contents, _ := shipped.File(shipped.Live)
	draft, err := machine.Parse(contents, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	run, err := b.corbie.Start(ctx, draft, client.Start{Labels: map[string]string{work.LabelBulldog: b.identity(), work.LabelKind: work.KindLive}})
	if err != nil {
		b.t.Fatal(err)
	}
	b.until("the left run is listed as going", func() bool {
		return slices.ContainsFunc(b.listed(b.identity(), map[string]string{work.LabelKind: work.KindLive}), func(r *client.Run) bool { return r.ID == run && !r.HasEnded() })
	})
	return run
}

// index gives where the first line holding a part is in lines, or -1.
func index(lines []string, part string) int {
	return slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, part) })
}
