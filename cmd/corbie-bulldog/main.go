// Command corbie-bulldog is Bulldog, which corbie bulldog runs. It serves
// one office: a folder of teams, their people and their workers, asked
// things through bridges, with decisions that wait for the right yes.
package main

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/zummed/corbie/client/cli"
	"github.com/zummed/corbie/machine"
	"github.com/zummed/corbie/manual"
	"github.com/zummed/corbie/plugin"

	"github.com/zummed/corbie-bulldog/internal/server"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// commands is the table of corbie bulldog's commands, read as corbie reads
// its own.
var commands = []cli.Command{
	{Name: "", Usage: "[<folder>] [--listen <address>] [--url <url>]", Summary: "serve an office until stopped", Stays: true, Run: serve, Help: `
With no folder named, it serves the folder it is run in when that holds an
office.yaml, and otherwise ~/bulldog, which is given the example office the
first time. A folder named that is empty is given the example; one that is
not there is refused, and corbie bulldog init makes it.

  --listen <address>   where it listens: BULLDOG_LISTEN, or else 127.0.0.1:7410
  --url <url>          how a person reaches it, and what every link the office
                       gives begins with: BULLDOG_URL, or else http:// and the
                       listen address

It is a client of the Corbie set up on this computer, and runs no role:
corbie start controller runner bulldog runs the roles with Bulldog beside
them. Another Corbie is named as corbie itself is told, by --config or
CORBIE_CONFIG and --store or CORBIE_STORE: see corbie --help.`},
	{Name: "check", Usage: "[<folder>]", Summary: "read an office as the server would, and say what is wrong", Run: check, Help: `
It starts nothing and writes nothing, uses no Corbie, and takes no --config
or --store.`},
	{Name: "init", Usage: "[<folder>]", Summary: "ready a folder as the server would, without serving it", Run: begin, Help: `
A folder that is empty or not there is given the example office, and an
office in no git repository is made one. In an office that was moved, it
takes the move, keeping its identity, its store and its runs. It uses no
Corbie, and takes no --config or --store.`},
	{Name: "version", Usage: "", Summary: "print the versions, the machine types and the plugins", Run: printVersion},
}

// version is which Bulldog this is. A release build is given it; any
// other build reads it from the module's build information.
var version string

// versions gives which Bulldog this is, and which Corbie it is built on,
// each as its module's build information has it, without the v.
func versions() (bulldog, corbie string) {
	bulldog, corbie = cmp.Or(version, "dev"), "dev"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return bulldog, corbie
	}
	if version == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		bulldog = strings.TrimPrefix(info.Main.Version, "v")
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/zummed/corbie" && dep.Version != "(devel)" {
			corbie = strings.TrimPrefix(dep.Version, "v")
		}
	}
	return bulldog, corbie
}

// printVersion prints which Bulldog this is, the Corbie it is built on,
// and the machine types and plugins that Corbie holds, as corbie version
// does.
func printVersion(e *cli.Env, args []string) error {
	if _, err := e.Parse(args); err != nil {
		return err
	}
	bulldog, corbie := versions()
	holds := map[string]any{"version": bulldog, "corbie": corbie, "machines": machine.Types(), "plugins": plugin.Names()}
	return e.Print(holds, func() string {
		return fmt.Sprintf("corbie bulldog %s\nbuilt on corbie %s\nmachine types: %s\nplugins and libraries: %s\n", bulldog, corbie,
			strings.Join(machine.Types(), ", "), strings.Join(plugin.Names(), ", "))
	})
}

// bulldogPage is the manual's page bulldog, which a model is told when it
// connects to corbie bulldog mcp: what Bulldog is, and how to write an
// office for a person.
//
//go:embed bulldog.md
var bulldogPage string

func init() {
	manual.Add(func() []manual.Page {
		return []manual.Page{{Name: "bulldog", Group: "Bulldog", Summary: "Bulldog: an office of teams on Corbie, and how to write one for a person", Text: bulldogPage}}
	})
}

func main() {
	os.Exit(cli.Program("bulldog", commands, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// serve serves an office until it is stopped. Only the folder it serves
// when none is named is made where it is not there.
func serve(e *cli.Env, args []string) error {
	listen := e.Flags.String("listen", "", "")
	url := e.Flags.String("url", "", "")
	folders, err := e.Parse(args)
	switch {
	case err != nil:
		return err
	case len(folders) > 1:
		return cli.Mistake("it serves one office, and %d folders are named", len(folders))
	case e.Given("json"):
		return cli.Mistake("it prints no JSON")
	}
	named := append(folders, "")[0]
	dir, err := server.Folder(named)
	if err != nil {
		return err
	}
	id, err := server.Ready(dir, server.Readying{Make: named == ""}, time.Now(), server.Log(e.Out))
	if err != nil {
		return err
	}
	c, err := e.Client()
	if err != nil {
		return err
	}
	l, u := where(*listen, *url)
	err = server.Serve(e.Context(), server.Options{Dir: dir, Identity: id, Corbie: c, Store: e.Address(), Clock: work.Wall{},
		Listen: l, URL: u, Out: e.Out})
	// A server that was stopped has done what it was asked, which Corbie's
	// program would take for a failure.
	if errors.Is(err, context.Canceled) && e.Context().Err() != nil {
		return nil
	}
	return err
}

// where gives where the server listens and how a person reaches it: what
// the command line says, or else what Bulldog's own variables say. No
// other variable is read for either.
func where(listen, url string) (string, string) {
	return cmp.Or(listen, os.Getenv("BULLDOG_LISTEN"), server.Listen), cmp.Or(url, os.Getenv("BULLDOG_URL"))
}

// check reads an office as the server would, with no Corbie, and says
// what it found and everything wrong.
func check(e *cli.Env, args []string) error {
	dir, err := oneFolder(e, args)
	if err != nil {
		return err
	}
	return server.Check(dir, e.Out)
}

// begin readies a folder as the server would, without serving it, and
// says what it did.
func begin(e *cli.Env, args []string) error {
	dir, err := oneFolder(e, args)
	if err != nil {
		return err
	}
	var did bytes.Buffer
	say := server.Log(&did)
	// What it did is said whatever becomes of it.
	defer func() { fmt.Fprint(e.Out, did.String()) }()
	if _, err := server.Ready(dir, server.Readying{Make: true, Move: true}, time.Now(), say); err != nil {
		return err
	}
	store := filepath.Join(dir, shipped.Dir, server.StoreFile)
	_, err = os.Stat(store)
	had := err == nil
	db, err := server.OpenStore(dir)
	if err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("the store, %s, cannot be closed: %w", store, err)
	}
	if !had {
		say("%s is the office's store now, with nothing in it yet", store)
	}
	if did.Len() == 0 {
		say("%s is an office ready to be served already, so there was nothing to do", dir)
	}
	say("serve it with: corbie bulldog %s", dir)
	return nil
}

// oneFolder reads the line of a command that uses no Corbie: a folder or
// none, and no --config, --store or --json.
func oneFolder(e *cli.Env, args []string) (string, error) {
	folders, err := e.Parse(args)
	switch {
	case err != nil:
		return "", err
	case len(folders) > 1:
		return "", cli.Mistake("it takes one folder, and %d are named", len(folders))
	case e.Given("config"), e.Given("store"):
		return "", cli.Mistake("it uses no Corbie, and takes no --config or --store")
	case e.Given("json"):
		return "", cli.Mistake("it prints no JSON")
	}
	return server.Folder(append(folders, "")[0])
}
