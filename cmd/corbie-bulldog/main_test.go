package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client/cli"
)

// asProgram, set in a process's environment, makes the test binary run as
// corbie bulldog.
const asProgram = "BULLDOG_TEST_AS_PROGRAM"

// TestMain keeps every office's secret in a config directory of the
// tests' own, which the processes the tests start share.
func TestMain(m *testing.M) {
	if os.Getenv(asProgram) != "" {
		main()
		return
	}
	home, err := os.MkdirTemp("", "bulldog-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", home)
	os.Setenv("HOME", home)
	os.Unsetenv("CORBIE_CONFIG")
	os.Unsetenv("CORBIE_STORE")
	status := m.Run()
	os.RemoveAll(home)
	os.Exit(status)
}

// bulldog runs corbie bulldog in this process, and gives its exit status,
// what it printed and what it said along the way.
func bulldog(args ...string) (int, string, string) {
	var out, said bytes.Buffer
	status := cli.Program("bulldog", commands, args, strings.NewReader(""), &out, &said)
	return status, out.String(), said.String()
}

// write writes an office into a directory of the test's own, with a
// change made to its files, and gives the directory.
func write(t *testing.T, tree fs.FS, change func(files map[string]string)) string {
	files := testkit.Files(t, tree)
	if change != nil {
		change(files)
	}
	return testkit.Dir(t, files)
}

func TestCheckPrintsNothingForTheExamples(t *testing.T) {
	t.Parallel()
	for _, tree := range []fs.FS{examples.House, examples.Work} {
		status, out, said := bulldog("check", write(t, tree, nil))
		if status != 0 || out != "" || said != "" {
			t.Fatalf("check ended %d, and printed %q and said %q", status, out, said)
		}
	}
}

func TestCheckPrintsEveryProblem(t *testing.T) {
	t.Parallel()
	// A problem of a team file refuses the reading. Each is printed with its
	// file and line.
	dir := write(t, examples.House, func(files map[string]string) {
		files["garden/team.yaml"] += "budget: 40\n"
		files["office.yaml"] = strings.Replace(files["office.yaml"], "people: [sam, kit]", "people: [sam, jo]", 1)
	})
	status, out, said := bulldog("check", dir)
	if status != 1 || said != "corbie bulldog check: the office has 2 problems\n" {
		t.Fatalf("check ended %d, and said %q", status, said)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "garden/team.yaml:9: budget is not a key") || !strings.HasPrefix(lines[1], "office.yaml:14: people: jo is not a person") {
		t.Fatalf("check printed:\n%s", out)
	}
	_, out, _ = bulldog("check", dir, "--json")
	var problems []struct{ File, Text string }
	if err := json.Unmarshal([]byte(out), &problems); err != nil || len(problems) != 2 || problems[0].File != "garden/team.yaml" {
		t.Fatalf("check --json printed %s", out)
	}

	// A problem of a worker stops only the teams that use it, and is printed
	// as well.
	dir = write(t, examples.House, func(files map[string]string) {
		files["fixer.yaml"] = strings.Replace(files["fixer.yaml"], "model: usual", "model: fast", 1)
	})
	status, out, said = bulldog("check", dir)
	if status != 1 || !strings.HasPrefix(out, "fixer.yaml") || !strings.Contains(out, "model: fast") || said != "corbie bulldog check: the office has 1 problem\n" {
		t.Fatalf("check ended %d, printed %q and said %q", status, out, said)
	}
}

func TestADirectoryThatIsNotAnOfficeIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	status, _, said := bulldog(dir)
	if status != 1 || !strings.Contains(said, "holds no office.yaml, so it is not an office") || !strings.Contains(said, "lead: sam") {
		t.Fatalf("serving ended %d, and said %q", status, said)
	}
	status, out, _ := bulldog("check", dir)
	if status != 1 || !strings.Contains(out, "holds no office.yaml") {
		t.Fatalf("check ended %d, and printed %q", status, out)
	}
	if status, _, said := bulldog("key", dir, "sam"); status != 1 || !strings.Contains(said, "holds no office.yaml") {
		t.Fatalf("key ended %d, and said %q", status, said)
	}
	// Written as it says, the smallest office reads.
	_, smallest, _ := strings.Cut(said, ":\n\n")
	testkit.Write(t, dir, map[string]string{"office.yaml": smallest})
	if status, out, said := bulldog("check", dir); status != 0 {
		t.Fatalf("check of the smallest office ended %d:\n%s%s", status, out, said)
	}
}

func TestKeyNamesAPersonOfTheOffice(t *testing.T) {
	t.Parallel()
	dir := write(t, examples.House, nil)
	status, out, _ := bulldog("key", dir, "sam", "--url", "https://house.example/")
	if status != 0 || !strings.HasPrefix(out, "https://house.example/key?k=") || !strings.HasSuffix(out, "&p=sam\n") {
		t.Fatalf("key ended %d, and printed %q", status, out)
	}
	if status, again, _ := bulldog("key", dir, "sam", "--url", "https://house.example/"); status != 0 || again != out {
		t.Fatalf("key printed %q, then %q", out, again)
	}
	if status, _, said := bulldog("key", dir, "jo"); status != 1 || !strings.Contains(said, "jo is no person of the office") {
		t.Fatalf("key for jo ended %d, and said %q", status, said)
	}
	if status, _, _ := bulldog("key", dir, "sam", "--store", t.TempDir()); status != 2 {
		t.Fatalf("key with --store ended %d", status)
	}
}

func TestManualHasEachShippedPage(t *testing.T) {
	t.Parallel()
	status, list, _ := bulldog("manual")
	if status != 0 || !strings.Contains(list, "What Bulldog ships at ./.bulldog/\n") {
		t.Fatalf("manual ended %d, and printed %q", status, list)
	}
	readmes, _ := fs.Glob(shipped.Files, "*/README.md")
	deeper, _ := fs.Glob(shipped.Files, "*/*/README.md")
	for _, readme := range append(readmes, deeper...) {
		page := strings.TrimSuffix(readme, "/README.md")
		text, _ := fs.ReadFile(shipped.Files, readme)
		title, _, _ := strings.Cut(string(text), "\n")
		if !strings.Contains(list, page+" ") || !strings.Contains(list, title+"\n") {
			t.Errorf("the manual does not list %s, %q", page, title)
		}
		if status, out, _ := bulldog("manual", page); status != 0 || out != strings.TrimRight(string(text), "\n")+"\n" {
			t.Errorf("manual %s ended %d, and printed %q", page, status, out)
		}
	}
}

// TestServeSignsInWithAKeyLink serves the house as a client of a Corbie
// set up in directories of the test's own, signs Sam in with the link key
// prints, and stops the program.
func TestServeSignsInWithAKeyLink(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if status, out, said := corbie("init", "--store", filepath.Join(t.TempDir(), "store"), "--name", "box", "--config", config); status != 0 {
		t.Fatalf("corbie init ended %d:\n%s%s", status, out, said)
	}
	dir := write(t, examples.House, nil)
	program := exec.Command(os.Args[0], dir, "--config", config, "--listen", "127.0.0.1:0")
	program.Env = append(os.Environ(), asProgram+"=1")
	var said bytes.Buffer
	program.Stderr = &said
	printed, err := program.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := program.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			program.Process.Kill()
			program.Wait()
		}
	}()

	// It prints where the screens are, and how to get a key link.
	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		for read := bufio.NewScanner(printed); read.Scan(); {
			lines <- read.Text()
		}
	}()
	var address, how string
	limit := time.After(testkit.Patience)
	for address == "" || how == "" {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("the program ended: %v\n%s", program.Wait(), said.String())
			}
			if at, ok := strings.CutPrefix(line, "  screens    "); ok {
				address = at
			}
			if at, ok := strings.CutPrefix(line, "  key link   "); ok {
				how = at
			}
		case <-limit:
			t.Fatalf("waited %v for the program to say where it serves", testkit.Patience)
		}
	}
	if !strings.HasPrefix(address, "http://127.0.0.1:") || how != "corbie bulldog key "+dir+" <person> --url "+address {
		t.Fatalf("the program serves at %q, and says %q prints a key link", address, how)
	}

	// The link key prints signs Sam in.
	status, link, _ := bulldog("key", dir, "sam", "--url", address)
	if status != 0 {
		t.Fatalf("key ended %d", status)
	}
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	pressed, err := browser.Post(strings.TrimSpace(link), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	pressed.Body.Close()
	if pressed.StatusCode != http.StatusSeeOther {
		t.Fatalf("pressing the key link gave %s", pressed.Status)
	}
	me, err := browser.Get(address + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	var who struct {
		Person struct{ ID string }
		Office struct{ Name string }
	}
	err = json.NewDecoder(me.Body).Decode(&who)
	me.Body.Close()
	if err != nil || who.Person.ID != "sam" || who.Office.Name != "The house" {
		t.Fatalf("/api/me gave %s: %+v, %v", me.Status, who, err)
	}

	// Stopped with a stream of changes open, it ends well, having said
	// nothing.
	events, err := browser.Get(address + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer events.Body.Close()
	if err := program.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	stopped = true
	ended := make(chan error, 1)
	go func() { ended <- program.Wait() }()
	select {
	case err := <-ended:
		if err != nil || said.Len() > 0 {
			t.Fatalf("the program ended with %v, and said:\n%s", err, said.String())
		}
	case <-time.After(testkit.Patience):
		program.Process.Kill()
		t.Fatalf("waited %v for the program to stop", testkit.Patience)
	}
}

// corbie runs corbie in this process, and gives its exit status, what it
// printed and what it said along the way.
func corbie(args ...string) (int, string, string) {
	var out, said bytes.Buffer
	status := cli.Run(context.Background(), args, strings.NewReader(""), &out, &said)
	return status, out.String(), said.String()
}
