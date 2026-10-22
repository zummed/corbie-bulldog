// Command corbie-bulldog is Bulldog, which corbie bulldog runs: it serves
// an office of teams on Corbie, checks an office, and prints a person's
// key link.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/zummed/corbie-bulldog/cmd/internal/serve"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/screens"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client/cli"
	"github.com/zummed/corbie/manual"

	// The machine types Corbie's parser checks an office's definitions
	// against.
	_ "github.com/zummed/corbie/agent"
	_ "github.com/zummed/corbie/flow"
	_ "github.com/zummed/corbie/statechart"
)

// listen is where the screens listen by default.
const listen = "127.0.0.1:8484"

// commands is the table of corbie bulldog's commands.
var commands = []cli.Command{
	{Name: "", Usage: "<directory> [--listen <address>] [--url <url>]", Summary: "serve an office until stopped", Stays: true, Run: serveOffice, Help: `
It serves the office's requests, bridges, routines and screens, as a client
of the Corbie that --config and --store name. It reads the office again
when one of its files changes. While a reading is refused, the last good
one stays in force.

  --listen <address>   where the screens listen; default 127.0.0.1:8484
  --url <url>          where people reach the screens, which links to them
                       start with; default http:// and the address listened at

It prints no JSON.`},
	{Name: "check", Usage: "<directory>", Summary: "print every problem of an office, and start nothing", Run: check, Help: `
It reads the office as serving does, prints each problem with its file, and
fails when there is one. With --json it prints the problems as JSON. It uses
no Corbie, and takes no --config or --store.`},
	{Name: "key", Usage: "<directory> <person> [--url <url>]", Summary: "print a person's key link, which signs them in", Run: key, Help: `
The link opens a page whose button signs the person in. Changing or removing
the person's entry in office.yaml signs them out.

  --url <url>   where people reach the screens; default http://127.0.0.1:8484

With --json it prints { link }. It uses no Corbie, and takes no --config or
--store.`},
}

func main() {
	os.Exit(cli.Program("bulldog", commands, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func init() { manual.Add(shippedPages) }

// shippedPages gives the manual the page of each directory Bulldog ships,
// its README.md, named by the directory's path under ./.bulldog/.
func shippedPages() []manual.Page {
	var pages []manual.Page
	fs.WalkDir(shipped.Files, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != "README.md" {
			return err
		}
		text, err := fs.ReadFile(shipped.Files, file)
		title, _, _ := strings.Cut(string(text), "\n")
		pages = append(pages, manual.Page{Name: path.Dir(file), Group: "What Bulldog ships at ./.bulldog/", Summary: title, Text: string(text)})
		return err
	})
	return pages
}

// serveOffice serves an office until it is stopped.
func serveOffice(e *cli.Env, args []string) error {
	at := e.Flags.String("listen", listen, "")
	url := e.Flags.String("url", "", "")
	given, err := named(e, args, "one directory", 1)
	switch {
	case err != nil:
		return err
	case e.Given("json"):
		return cli.Mistake("it prints no JSON")
	}
	r, err := read(given[0])
	if err != nil {
		return err
	}
	secret, err := office.Secret(r.Office.ID)
	if err != nil {
		return err
	}
	corbie, err := e.Client()
	if err != nil {
		return err
	}
	l, err := net.Listen("tcp", *at)
	if err != nil {
		return err
	}
	address := *url
	if address == "" {
		address = "http://" + l.Addr().String()
	}
	how := "corbie bulldog key " + given[0] + " <person>"
	if address != "http://"+listen {
		how += " --url " + address
	}
	fmt.Fprintf(e.Out, "Bulldog serves %s until it is stopped.\n\n  screens    %s\n  key link   %s\n\n", r.Office.Name, address, how)
	return serve.New(corbie, r, secret, address, e.Err).Run(e.Context(), l)
}

// check prints every problem of an office, and fails when there is one.
func check(e *cli.Env, args []string) error {
	given, err := named(e, args, "one directory", 1)
	if err != nil {
		return err
	}
	if err := noCorbie(e); err != nil {
		return err
	}
	problems := office.Problems{}
	r, err := serve.Read(given[0])
	switch {
	case errors.As(err, &problems):
	case err != nil:
		return err
	case len(r.Office.Problems) > 0:
		problems = r.Office.Problems
	}
	err = e.Print(problems, func() string {
		var lines strings.Builder
		for _, p := range problems {
			lines.WriteString(p.String() + "\n")
		}
		return lines.String()
	})
	switch {
	case err != nil:
		return err
	case len(problems) == 1:
		return errors.New("the office has 1 problem")
	case len(problems) > 1:
		return fmt.Errorf("the office has %d problems", len(problems))
	}
	return nil
}

// key prints a person's key link.
func key(e *cli.Env, args []string) error {
	url := e.Flags.String("url", "http://"+listen, "")
	given, err := named(e, args, "a directory and a person", 2)
	if err != nil {
		return err
	}
	if err := noCorbie(e); err != nil {
		return err
	}
	r, err := read(given[0])
	if err != nil {
		return err
	}
	secret, err := office.Secret(r.Office.ID)
	if err != nil {
		return err
	}
	link, err := screens.KeyLink(*url, secret, r.Office, given[1])
	if err != nil {
		return err
	}
	return e.Print(map[string]string{"link": link}, func() string { return link + "\n" })
}

// named reads a command's line, which names as many words as it takes.
func named(e *cli.Env, args []string, takes string, words int) ([]string, error) {
	given, err := e.Parse(args)
	if err == nil && len(given) != words {
		are := fmt.Sprintf("%d are", len(given))
		if len(given) == 1 {
			are = "1 is"
		}
		err = cli.Mistake("it takes %s, and %s named", takes, are)
	}
	return given, err
}

// noCorbie refuses --config and --store, for a command that uses no
// Corbie.
func noCorbie(e *cli.Env) error {
	if e.Given("config") || e.Given("store") {
		return cli.Mistake("it uses no Corbie, and takes no --config or --store")
	}
	return nil
}

// read reads the office in a directory as serving does. A directory with
// no office.yaml is refused with what to write there.
func read(dir string) (*requests.Reading, error) {
	r, err := serve.Read(dir)
	var problems office.Problems
	switch {
	case err == nil:
		return r, nil
	case !errors.As(err, &problems):
		return nil, err
	}
	if _, missing := os.Stat(filepath.Join(dir, "office.yaml")); errors.Is(missing, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s holds no office.yaml, so it is not an office. The smallest office.yaml names one person, who leads it:\n\n%s", dir, smallest)
	}
	return nil, fmt.Errorf("the office is refused, with %s", serve.Problems(problems))
}

// smallest is the smallest office.yaml.
const smallest = `  name: The house
  everyone:
    sam: { name: Sam }
  people: [sam]
  lead: sam`
