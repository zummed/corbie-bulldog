// Demo serves the house, with its Telegram bridge, with nothing set up: a
// whole Corbie in this process, a stand-in model that plays the house's
// workers through the house's day, and a stand-in for the Telegram bridge.
// It opens with the first part of the day played, as Sam and Kit would on
// the screens. It reaches no network, needs no key and costs nothing. It
// is the quickest way to see the screens at work.
//
//	go run ./cmd/demo            (or: mise run demo)
package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zummed/corbie/client"
	"github.com/zummed/corbie/permission"
	"github.com/zummed/corbie/runner"
	"github.com/zummed/corbie/secret"
	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/bridges/standin"
	"github.com/zummed/corbie-bulldog/internal/server"
	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// Listen is where the demo listens when BULLDOG_LISTEN does not say: beside
// a Bulldog that may be serving the real thing.
const Listen = "127.0.0.1:7420"

func main() {
	// This program is also the process the runner starts for each job.
	if len(os.Args) > 1 && os.Args[1] == runner.JobArg {
		runner.JobMain()
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(1)
	}
}

// run serves the demo until the context ends: the house in a folder of
// its own, which is removed afterwards.
func run(ctx context.Context, out io.Writer, begun func(server.Areas)) error {
	model := testkit.StartModel(play)
	defer model.Close()
	dir, err := os.MkdirTemp("", "bulldog-demo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	files, err := house(model.Line())
	if err != nil {
		return err
	}
	if err := server.WriteFolder(dir, files); err != nil {
		return err
	}
	say := server.Log(out)
	id, err := server.Ready(dir, server.Readying{}, time.Now(), say)
	if err != nil {
		return err
	}
	corbie, err := client.NewLocal(ctx, permission.Layer{Allow: []string{"all"}}, func(role string, err error) {
		fmt.Fprintf(out, "demo: corbie's %s: %v\n", role, err)
	})
	if err != nil {
		return err
	}
	defer corbie.Close()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	corbie.Runner.Command = []string{self, runner.JobArg}
	if err := secret.Set(ctx, corbie.Controller.Store, corbie.Controller.ID, testkit.Key, testkit.KeyValue); err != nil {
		return err
	}
	fmt.Fprintln(out, "demo: the house, on a Corbie in this process, with a stand-in for its model and for its Telegram bridge: nothing reaches the network, and nothing costs anything")
	// Once the work has begun, the first part of the day is played while
	// the office is served, until the serving ends; whatever stops it is
	// said, and the office is served all the same.
	clock := work.Wall{}
	opening, stopOpening := context.WithCancel(ctx)
	var opened sync.WaitGroup
	begin := func(a server.Areas) {
		if begun != nil {
			begun(a)
		}
		opened.Go(func() {
			if err := openDay(opening, a, clock); err != nil && opening.Err() == nil {
				fmt.Fprintf(out, "demo: the house's day could not be opened: %v\n", err)
			}
		})
	}
	err = server.Serve(ctx, server.Options{Dir: dir, Identity: id, Corbie: corbie.Client, Store: "in this process's memory", Clock: clock,
		Listen: cmp.Or(os.Getenv("BULLDOG_LISTEN"), Listen), URL: os.Getenv("BULLDOG_URL"), Out: out, Begun: begin})
	stopOpening()
	opened.Wait()
	return err
}

// house gives the files of the house, with its Telegram bridge a stand-in
// that reaches no service, and every model the stand-in.
func house(standIn string) (map[string][]byte, error) {
	var model yaml.Node
	if err := yaml.Unmarshal([]byte(standIn), &model); err != nil {
		return nil, fmt.Errorf("the stand-in's model cannot be read: %w", err)
	}
	files := map[string][]byte{}
	err := fs.WalkDir(examples.House(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(name, "bridges/telegram/") {
			return err
		}
		contents, err := fs.ReadFile(examples.House(), name)
		if err != nil || path.Ext(name) != ".yaml" {
			files[name] = contents
			return err
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(contents, &doc); err != nil {
			return fmt.Errorf("%s cannot be read: %w", name, err)
		}
		standInFor(&doc, model.Content[0])
		files[name], err = yaml.Marshal(&doc)
		return err
	})
	for name, text := range standin.In("telegram", standin.Folder(false, true)) {
		files[name] = []byte(text)
	}
	return files, err
}

// standInFor puts the stand-in in place of every model written out in a
// file: the model of a definition, and each a team's models give a name.
// A model named by one word is left as it is, since the team file that
// gives the word is given the stand-in.
func standInFor(n *yaml.Node, model *yaml.Node) {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, value := n.Content[i].Value, n.Content[i+1]
			switch {
			case key == "model" && value.Kind == yaml.MappingNode:
				n.Content[i+1] = model
			case key == "models" && value.Kind == yaml.MappingNode:
				for j := 1; j < len(value.Content); j += 2 {
					value.Content[j] = model
				}
			}
		}
	}
	for _, c := range n.Content {
		standInFor(c, model)
	}
}
