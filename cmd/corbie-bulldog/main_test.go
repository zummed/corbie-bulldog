package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/zummed/corbie/client/cli"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/server"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
)

// Program is this program's name on PATH, as corbie start finds it.
const Program = "corbie-bulldog"

// TestMain is also corbie-bulldog itself, when the test binary is run
// under that name, and the process a runner starts for each job.
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == Program {
		main()
		return
	}
	testkit.Main(m)
}

// onPath puts this program on PATH, as corbie-bulldog.
func onPath(t *testing.T) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(self, filepath.Join(bin, Program)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// run runs corbie bulldog's command line, and gives its exit status, what
// it printed and what it said.
func run(args ...string) (int, string, string) {
	return runReading("", args...)
}

// runReading runs corbie bulldog's command line reading stdin.
func runReading(stdin string, args ...string) (int, string, string) {
	var out, said bytes.Buffer
	status := cli.Program("bulldog", commands, args, strings.NewReader(stdin), &out, &said)
	return status, out.String(), said.String()
}

// corbie sets a Corbie up in a folder of the test's own, as corbie init
// does on a computer, and gives the folder of its config.
func corbie(t *testing.T, name string, more ...string) string {
	t.Helper()
	config := filepath.Join(t.TempDir(), "config")
	var out, said bytes.Buffer
	if status := cli.Run(context.Background(), append([]string{"init", "--config", config, "--name", name}, more...), strings.NewReader(""), &out, &said); status != 0 {
		t.Fatalf("corbie init ended %d: %s%s", status, out.String(), said.String())
	}
	return config
}

// storeOf gives the store a Corbie set up by corbie init keeps.
func storeOf(config string) string { return filepath.Join(config, "store") }

// isolated runs the test in a folder of its own, with a home of its own
// and none of the variables a person may have set.
func isolated(t *testing.T) (here, home string) {
	t.Helper()
	here, home = t.TempDir(), t.TempDir()
	t.Chdir(here)
	t.Setenv("HOME", home)
	for _, name := range []string{"CORBIE_CONFIG", "CORBIE_STORE", "BULLDOG_LISTEN", "BULLDOG_URL"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	return here, home
}

func TestServerHelpEndsWithItsCommands(t *testing.T) {
	isolated(t)
	status, out, said := run("--help")
	if status != 0 || said != "" || !strings.HasPrefix(out, "corbie bulldog [<folder>]") {
		t.Fatalf("status %d, printed %q, said %q", status, out, said)
	}
	_, commands, _ := strings.Cut(out, "The commands of corbie bulldog:\n\n")
	var names []string
	for _, line := range strings.Split(commands, "\n") {
		if strings.HasPrefix(line, "  ") {
			names = append(names, strings.Fields(line)[0])
		}
	}
	if strings.Join(names, " ") != "check init version manual mcp" || strings.Count(commands, "\n\n") != 1 {
		t.Errorf("the help does not end with check, init, version, manual and mcp:\n%s", out)
	}
	for _, args := range [][]string{{"-h"}, {"check", "--help"}, {"init", "-h"}} {
		if status, out, said := run(args...); status != 0 || out == "" || said != "" {
			t.Errorf("%v: status %d, printed %q, said %q", args, status, out, said)
		}
	}
}

func TestServerMistakesAndFailures(t *testing.T) {
	here, _ := isolated(t)
	for _, args := range [][]string{{"check", "a", "b"}, {"--json"}, {"init", "a", "b"}, {"check", "--json"}, {"init", "--store", "x"}, {"check", "--config", "x"}, {"init", "--listen", ":1"}} {
		status, out, said := run(args...)
		named := "corbie bulldog"
		if args[0] == "check" || args[0] == "init" {
			named += " " + args[0]
		}
		if status != 2 || out != "" || !strings.HasPrefix(said, named+": ") || !strings.Contains(said, "\n\nusage: "+named) {
			t.Errorf("%v: status %d, printed %q, said %q", args, status, out, said)
		}
	}
	// A check that finds a problem fails.
	files := testkit.Files(t, examples.House())
	files["fixer.yaml"] = strings.Replace(files["fixer.yaml"], "model: usual", "model: quick", 1)
	testkit.Write(t, filepath.Join(here, "broken"), files)
	status, out, said := run("check", "broken")
	if status != 1 || !strings.HasPrefix(said, "corbie bulldog check: ") || !strings.Contains(said, "fixer.yaml") || !strings.Contains(out, "The house, the top team") {
		t.Errorf("checking a house with a problem: status %d, printed %q, said %q", status, out, said)
	}
	if in, _ := os.ReadDir(here); len(in) != 1 {
		t.Errorf("mistakes made something in the folder: %v", in)
	}
}

func TestServerWordsAreNeverFolders(t *testing.T) {
	here, home := isolated(t)
	// check and init are commands, and ./check a folder.
	if status, out, _ := run("init", "./check"); status != 0 || !strings.Contains(out, "the example was written there") {
		t.Fatalf("init ./check: status %d, printed %q", status, out)
	}
	if _, err := os.Stat(filepath.Join(here, "check", "office.yaml")); err != nil {
		t.Errorf("./check was not made an office: %v", err)
	}
	if status, out, said := run("check", "./check"); status != 0 || !strings.Contains(out, "The house, the top team") {
		t.Errorf("check ./check: status %d, printed %q, said %q", status, out, said)
	}
	// A word mistyped, or help, is no folder made into an office.
	for _, word := range []string{"chek", "help"} {
		if status, _, said := run(word); status != 1 || !strings.Contains(said, "corbie bulldog init") {
			t.Errorf("%s: status %d, said %q", word, status, said)
		}
		if _, err := os.Stat(filepath.Join(here, word)); err == nil {
			t.Errorf("%s was made into a folder", word)
		}
	}
	if in, _ := os.ReadDir(home); len(in) != 0 {
		t.Errorf("the home was given %v", in)
	}
}

func TestServerInitSaysWhatItDid(t *testing.T) {
	here, _ := isolated(t)
	dir := filepath.Join(here, "house")
	status, out, _ := run("init", dir)
	if status != 0 || !strings.Contains(out, "the example was written there") || !strings.Contains(out, "is a git repository now") {
		t.Fatalf("init of a folder not there: status %d, printed %q", status, out)
	}
	for _, name := range []string{"identity", "bulldog.db", "ask.yaml", "call.yaml", "live.yaml", "clock.yaml", "tools/talk/talk.yaml", "tools/talk/README.md", "tools/notes/notes.yaml", "tools/build/build.yaml", "bridges/telegram/in.yaml", "bridges/slack/README.md", "rules/mentioned/mentioned.js", "rules/for-us/for-us.yaml", "rules/for-us/README.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".bulldog", name)); err != nil {
			t.Errorf(".bulldog/ holds no %s: %v", name, err)
		}
	}
	status, out, _ = run("init", dir)
	if status != 0 || !strings.Contains(out, "there was nothing to do") || strings.Contains(out, "written") {
		t.Errorf("init again: status %d, printed %q", status, out)
	}
	// A store removed to start again is made, and init says so.
	if err := os.Remove(filepath.Join(dir, ".bulldog", "bulldog.db")); err != nil {
		t.Fatal(err)
	}
	status, out, _ = run("init", dir)
	if status != 0 || !strings.Contains(out, "bulldog.db is the office's store now") || strings.Contains(out, "nothing to do") {
		t.Errorf("init with no store: status %d, printed %q", status, out)
	}
}

func TestServerOnlyBulldogVariablesRead(t *testing.T) {
	isolated(t)
	for _, name := range []string{"LISTEN", "PORT", "HOST", "ADDRESS", "URL", "BASE_URL", "PUBLIC_URL"} {
		t.Setenv(name, "10.0.0.1:9999")
	}
	if listen, url := where("", ""); listen != server.Listen || url != "" {
		t.Errorf("with other variables set, it listens on %q, reached at %q", listen, url)
	}
	t.Setenv("BULLDOG_LISTEN", "127.0.0.1:7499")
	t.Setenv("BULLDOG_URL", "https://house.example")
	if listen, url := where("", ""); listen != "127.0.0.1:7499" || url != "https://house.example" {
		t.Errorf("with Bulldog's set, it listens on %q, reached at %q", listen, url)
	}
	if listen, url := where("127.0.0.1:7498", "https://flat.example"); listen != "127.0.0.1:7498" || url != "https://flat.example" {
		t.Errorf("with the command line's, it listens on %q, reached at %q", listen, url)
	}
}

// A writer that many goroutines may write and read, which tells each time
// it is written.
type shared struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	written chan struct{}
}

func (s *shared) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.written <- struct{}{}:
	default:
	}
	return s.buf.Write(p)
}

func (s *shared) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// served runs corbie bulldog serving, in the background, and gives what it
// prints and says as it goes, and its status once it ends.
func served(args ...string) (out, said *shared, ended chan int) {
	written := make(chan struct{}, 1)
	out, said, ended = &shared{written: written}, &shared{written: written}, make(chan int, 1)
	go func() { ended <- cli.Program("bulldog", commands, args, strings.NewReader(""), out, said) }()
	return out, said, ended
}

// until stays until a server has printed a part, failing if it ends
// first or the test's patience runs out.
func until(t *testing.T, out, said *shared, ended chan int, part string) {
	t.Helper()
	limit := time.NewTimer(testkit.Patience)
	defer limit.Stop()
	for !strings.Contains(out.String(), part) {
		select {
		case <-out.written:
		case status := <-ended:
			t.Fatalf("the server ended %d before it printed %q: %s%s", status, part, out, said)
		case <-limit.C:
			t.Fatalf("waited %v, and the server has not printed %q: %s%s", testkit.Patience, part, out, said)
		}
	}
}

// taken gives an address something already listens on.
func taken(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l.Addr().String()
}

func TestServerCorbieNamedAsCorbieIsTold(t *testing.T) {
	here, _ := isolated(t)
	config, other := corbie(t, "desk"), corbie(t, "laptop")
	dir := filepath.Join(here, "house")
	if status, _, said := run("init", dir); status != 0 {
		t.Fatal(said)
	}
	busy := taken(t)
	t.Setenv("CORBIE_STORE", storeOf(other))
	// The store on the command line is used in place of the environment's,
	// written before the folder or after it.
	for _, args := range [][]string{
		{"--config", config, "--store", storeOf(config), "--listen", busy, dir},
		{dir, "--listen", busy, "--config", config, "--store", storeOf(config)},
	} {
		status, out, said := run(args...)
		if status != 1 || !strings.Contains(out, "on the Corbie of the controller desk-controller, whose store is "+storeOf(config)+"\n") || !strings.Contains(said, busy) {
			t.Errorf("%v: status %d, printed %q, said %q", args, status, out, said)
		}
	}
	// With none on the command line, the environment's.
	status, out, _ := run("--config", config, "--listen", busy, dir)
	if status != 1 || !strings.Contains(out, "whose store is "+storeOf(other)+"\n") {
		t.Errorf("with CORBIE_STORE: status %d, printed %q", status, out)
	}
	// The config on the command line is used in place of CORBIE_CONFIG,
	// and CORBIE_CONFIG in place of none.
	os.Unsetenv("CORBIE_STORE")
	t.Setenv("CORBIE_CONFIG", other)
	for args, controller := range map[string]string{"--config " + config: "desk-controller", "": "laptop-controller"} {
		status, out, _ := run(append(strings.Fields(args), "--listen", busy, dir)...)
		if status != 1 || !strings.Contains(out, "on the Corbie of the controller "+controller+",") {
			t.Errorf("with CORBIE_CONFIG and %q: status %d, printed %q", args, status, out)
		}
	}
}

func TestServerListensWhereItIsTold(t *testing.T) {
	here, _ := isolated(t)
	config := corbie(t, "desk")
	dir := filepath.Join(here, "house")
	if status, _, said := run("init", dir); status != 0 {
		t.Fatal(said)
	}
	// --listen is used in place of BULLDOG_LISTEN.
	t.Setenv("BULLDOG_LISTEN", taken(t))
	listen := free(t)
	out, said, ended := served("--config", config, "--listen", listen, dir)
	until(t, out, said, ended, "listening on "+listen+"\n")
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if status := <-ended; status != 0 {
		t.Fatalf("stopped, it ended %d: %s%s", status, out, said)
	}
	// BULLDOG_LISTEN is used in place of nothing, and 127.0.0.1:7410 when
	// neither is given: each taken, the server ends naming it.
	busy := taken(t)
	t.Setenv("BULLDOG_LISTEN", busy)
	if status, _, said := run("--config", config, dir); status != 1 || !strings.Contains(said, busy) {
		t.Errorf("with BULLDOG_LISTEN: status %d, said %q", status, said)
	}
	os.Unsetenv("BULLDOG_LISTEN")
	if l, err := net.Listen("tcp", server.Listen); err == nil {
		defer l.Close()
	}
	if status, _, said := run("--config", config, dir); status != 1 || !strings.Contains(said, server.Listen) {
		t.Errorf("with neither: status %d, said %q", status, said)
	}
}

// free gives an address nothing listens on.
func free(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestServerStoreAddressNeverEchoed(t *testing.T) {
	here, _ := isolated(t)
	config := corbie(t, "desk")
	dir := filepath.Join(here, "house")
	if status, _, said := run("init", dir); status != 0 {
		t.Fatal(said)
	}
	status, out, said := run("--config", config, "--store", "s3://key:hunter2@bucket/corbie?token=abc", dir)
	if status != 1 || strings.Contains(out+said, "hunter2") || strings.Contains(out+said, "abc") {
		t.Errorf("a store that cannot be opened: status %d, printed %q, said %q", status, out, said)
	}
}

func TestServerConfigWithNoClientRefused(t *testing.T) {
	here, _ := isolated(t)
	dir := filepath.Join(here, "house")
	if status, _, said := run("init", dir); status != 0 {
		t.Fatal(said)
	}
	status, out, said := run("--config", t.TempDir(), dir)
	if status != 1 || strings.Contains(out, "on the Corbie") || strings.Contains(out, "listening") || !strings.HasPrefix(said, "corbie bulldog: ") || !strings.Contains(said, "corbie init") {
		t.Errorf("status %d, printed %q, said %q", status, out, said)
	}
}

func TestServerStoppedBySignalSucceeds(t *testing.T) {
	here, _ := isolated(t)
	config := corbie(t, "desk")
	dir := filepath.Join(here, "house")
	if status, _, said := run("init", dir); status != 0 {
		t.Fatal(said)
	}
	out, said, ended := served("--config", config, "--listen", "127.0.0.1:0", dir)
	until(t, out, said, ended, "kit, open")
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if status := <-ended; status != 0 {
		t.Errorf("a server stopped by a signal ended %d: %s%s", status, out, said)
	}
	// The store is closed: the last to close it removes its log.
	if _, err := os.Stat(filepath.Join(dir, shipped.Dir, server.StoreFile+"-wal")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the store's log is left once the server stopped: %v", err)
	}
}

func TestServerRunBesideCorbiesRoles(t *testing.T) {
	here, _ := isolated(t)
	onPath(t)
	config := corbie(t, "desk", "--ceiling", "all")
	dir := filepath.Join(here, "house")
	if status, _, said := run("init", dir); status != 0 {
		t.Fatal(said)
	}
	// corbie start runs the controller and the runner with Bulldog beside
	// them, and Bulldog serves the house.
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	written := make(chan struct{}, 1)
	out, said, ended := &shared{written: written}, &shared{written: written}, make(chan int, 1)
	go func() {
		ended <- cli.Run(ctx, []string{"start", "--config", config, "controller", "runner", "bulldog", dir, "--listen", "127.0.0.1:0"}, strings.NewReader(""), out, said)
	}()
	until(t, out, said, ended, "kit, open http://")
	_, link, _ := strings.Cut(out.String(), "kit, open ")
	link, _, _ = strings.Cut(link, "\n")
	res, err := http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || len(res.Cookies()) == 0 {
		t.Fatalf("Kit's link answered %d with %v", res.StatusCode, res.Cookies())
	}
	// Stopping the roles stops Bulldog: once corbie start has ended,
	// nothing answers at the house's address.
	stop()
	<-ended
	if _, err := http.Get(link); err == nil {
		t.Error("Bulldog still answers once the roles have stopped")
	}
	if !strings.Contains(out.String(), "controller desk-controller") {
		t.Errorf("Bulldog is not on the controller corbie start ran: %s", out)
	}

	// And when Bulldog stops, the roles stop: one told to listen where it
	// cannot ends, and corbie start with it.
	busy := taken(t)
	status := cli.Run(context.Background(), []string{"start", "--config", config, "controller", "runner", "bulldog", dir, "--listen", busy}, strings.NewReader(""), out, said)
	if status == 0 || !strings.Contains(said.String(), "the program bulldog stopped") {
		t.Errorf("with Bulldog stopped, corbie start ended %d: %s", status, said)
	}
}

func TestServerManualAndModelServer(t *testing.T) {
	here, _ := isolated(t)
	status, out, _ := run("manual")
	for _, page := range []string{"office", "tools/talk", "bridges/slack", "bulldog", "corbie bulldog check", "agent", "corbie run submit"} {
		if status != 0 || !strings.Contains(out, "  "+page+" ") {
			t.Fatalf("corbie bulldog manual ended %d and lists no page %s:\n%s", status, page, out)
		}
	}
	if status, out, _ := run("manual", "office"); status != 0 || !strings.Contains(out, "agent: weather") {
		t.Errorf("corbie bulldog manual office ended %d and printed %q", status, out)
	}
	if status, out, _ := run("manual", "tools/talk"); status != 0 || !strings.Contains(out, "tools_file: [./.bulldog/tools/talk/talk.yaml]") {
		t.Errorf("corbie bulldog manual tools/talk ended %d and printed %q", status, out)
	}
	if status, out, _ := run("manual", "bulldog", "check"); status != 0 || !strings.HasPrefix(out, "corbie bulldog check [<folder>]") {
		t.Errorf("corbie bulldog manual bulldog check ended %d and printed %q", status, out)
	}

	if status, out, said := run("init", "house"); status != 0 {
		t.Fatalf("init house ended %d: %s%s", status, out, said)
	}
	calls := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"bulldog","arguments":{"args":["check","house"]}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"bulldog","arguments":{"args":["init","elsewhere"]}}}`,
	}, "\n") + "\n"
	// A Corbie named by the environment is the server's, and no command
	// is given it as a flag: check takes none.
	t.Setenv("CORBIE_CONFIG", t.TempDir())
	status, out, said := runReading(calls, "mcp", "--allow", "check")
	answers := strings.Split(strings.TrimSpace(out), "\n")
	slices.Sort(answers)
	if status != 0 || len(answers) != 4 {
		t.Fatalf("corbie bulldog mcp ended %d, said %q and answered four calls with:\n%s", status, said, out)
	}
	var started struct{ Result struct{ Instructions string } }
	if err := json.Unmarshal([]byte(answers[0]), &started); err != nil || started.Result.Instructions != bulldogPage {
		t.Errorf("a model connecting was told %s", answers[0])
	}
	var listed struct {
		Result struct{ Tools []struct{ Name string } }
	}
	if err := json.Unmarshal([]byte(answers[1]), &listed); err != nil || len(listed.Result.Tools) != 2 || listed.Result.Tools[0].Name != "manual" || listed.Result.Tools[1].Name != "bulldog" {
		t.Errorf("the tools were listed as %s", answers[1])
	}
	var checked struct {
		Result struct{ Content []struct{ Text string } }
	}
	if err := json.Unmarshal([]byte(answers[2]), &checked); err != nil || len(checked.Result.Content) != 1 ||
		!strings.Contains(checked.Result.Content[0].Text, `"ended":0`) || !strings.Contains(checked.Result.Content[0].Text, "The house, the top team") {
		t.Errorf("check was answered %s", answers[2])
	}
	if !strings.Contains(answers[3], "not one this server was started with permission for") {
		t.Errorf("init, not allowed, was answered %s", answers[3])
	}
	if _, err := os.Stat(filepath.Join(here, "elsewhere")); err == nil {
		t.Errorf("init, not allowed, made a folder")
	}
}

func TestServerVersion(t *testing.T) {
	isolated(t)
	status, out, said := run("version")
	lines := strings.Split(out, "\n")
	if status != 0 || said != "" || len(lines) != 5 || !strings.HasPrefix(lines[0], "corbie bulldog ") || !strings.HasPrefix(lines[1], "built on corbie ") ||
		!strings.Contains(lines[2], "agent") || !strings.Contains(lines[3], "http") {
		t.Fatalf("version ended %d, printed %q and said %q", status, out, said)
	}
	status, out, _ = run("version", "--json")
	var holds struct {
		Version, Corbie   string
		Machines, Plugins []string
	}
	if err := json.Unmarshal([]byte(out), &holds); status != 0 || err != nil || "corbie bulldog "+holds.Version != lines[0] || "built on corbie "+holds.Corbie != lines[1] ||
		!slices.Contains(holds.Machines, "agent") || !slices.Contains(holds.Plugins, "http") {
		t.Errorf("version --json ended %d and printed %q", status, out)
	}
}
