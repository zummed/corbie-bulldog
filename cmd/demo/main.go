// Demo serves the house with nothing set up: a whole Corbie in this
// process, a stand-in model that plays the house's workers, and a
// Telegram bridge of the house's own that reaches no one. Kit asks the
// house about a socket that sparks, and the house asks Sam to cut its
// power. Nothing reaches the network, and nothing costs anything.
//
//	go run ./cmd/demo            (or: mise run demo)
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/zummed/corbie-bulldog/cmd/internal/serve"
	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/screens"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/permission"
	"github.com/zummed/corbie/runner"
	"github.com/zummed/corbie/secret"

	// The machine types a definition may be, and the plugins its scripts
	// may call, here and in each job this program runs.
	_ "github.com/zummed/corbie/agent"
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/plugin/bundled"
	_ "github.com/zummed/corbie/statechart"
)

func main() {
	// This program is also the process the runner starts for each job.
	if len(os.Args) > 1 && os.Args[1] == runner.JobArg {
		runner.JobMain()
		return
	}
	listen := flag.String("listen", "127.0.0.1:8485", "where the screens listen")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := demo(ctx, *listen); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(1)
	}
}

// demo starts a Corbie and the stand-in model in this process, and serves
// the house on them until the context ends.
func demo(ctx context.Context, listen string) error {
	model := testkit.StartModel(play)
	defer model.Close()
	corbie, err := client.NewLocal(ctx, permission.Layer{Allow: []string{"all"}}, func(role string, err error) {
		log.Printf("Corbie's %s: %v", role, err)
	})
	if err != nil {
		return err
	}
	defer corbie.Close()
	if err := secret.Set(ctx, corbie.Controller.Store, corbie.Controller.ID, testkit.Key, testkit.KeyValue); err != nil {
		return err
	}
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	return run(ctx, corbie.Client, model, l, os.Stdout)
}

// run serves the house on a Corbie and a stand-in model, with the screens
// on a listener, until the context ends. The house is written to a
// directory that is removed afterwards. Kit asks it about the socket
// first.
func run(ctx context.Context, corbie *client.Client, model *testkit.Model, l net.Listener, out io.Writer) error {
	dir, err := os.MkdirTemp("", "bulldog-demo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := house(dir, model.Line()); err != nil {
		return err
	}
	r, err := serve.Read(dir)
	if err != nil {
		return err
	}
	key := make([]byte, 32)
	rand.Read(key)
	address := "http://" + l.Addr().String()
	b := serve.New(corbie, r, key, address, out)
	links := map[string]string{}
	for _, person := range []string{"sam", "kit"} {
		if links[person], err = screens.KeyLink(address, key, r.Office, person); err != nil {
			return err
		}
	}
	asked := requests.Ask{Team: "office", By: requests.By{Person: "kit"}, Said: socket,
		Files: []requests.File{{Name: "socket.png", Data: client.InputFile{Name: "socket.png", Contents: bytes.NewReader(photo())}}}}
	if _, err := b.Engine.Ask(ctx, asked); err != nil {
		return err
	}
	fmt.Fprintf(out, `The demo serves the house at %s until it is stopped.
Corbie runs in this process, a stand-in answers for the model, and nothing reaches the network.

Each link signs a person in:

  Sam   %s
  Kit   %s

Kit has asked the house about a socket that sparks. Sign in as Sam to answer what the house asks him.

`, address, links["sam"], links["kit"])
	return b.Run(ctx, l)
}

// models are the models the house's team files name.
var models = regexp.MustCompile(`\{ name: claude-[a-z0-9-]+, secret: ANTHROPIC_KEY \}`)

// telegram is the demo's Telegram bridge, which reaches no one: its in
// waits for an event that never comes, and its out gives each message the
// id of the one it is to.
var telegram = map[string]string{
	"bridges/telegram/in.yaml": `statechart: telegram-in
keep: 12h
input: { settings: map, cursor: any }
initial: listening
states:
  listening:
    on: { heard: listening }
`,
	"bridges/telegram/out.yaml": `flow: telegram-out
input: { settings: map, to: string, direct: boolean, text: string, choices: any, ref: any, answers: any }
stages:
  say: { job: { script: ./say.js, input: { to: "${input.to}" } } }
output: ${stages.say.output}
`,
	"bridges/telegram/say.js": `return { sent: input.to }
`,
}

// house writes the house into a directory, with every model the stand-in
// and the demo's own Telegram bridge.
func house(dir, model string) error {
	if err := os.CopyFS(dir, examples.House); err != nil {
		return err
	}
	files := map[string]string{}
	for name, text := range telegram {
		files[name] = text
	}
	for _, name := range []string{"office.yaml", "garden/team.yaml"} {
		text, err := fs.ReadFile(examples.House, name)
		if err != nil {
			return err
		}
		files[name] = strings.Replace(models.ReplaceAllLiteralString(string(text), model), "    from: ./.bulldog/bridges/telegram\n", "", 1)
	}
	for name, text := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}
