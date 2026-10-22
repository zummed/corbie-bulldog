// Package serve serves an office on a Corbie: its requests engine, its
// bridges and its screens, with the office read again whenever one of its
// files changes. The program and the demo serve an office with it.
package serve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/screens"
	"github.com/zummed/corbie-bulldog/shipped"
	"github.com/zummed/corbie/client"
)

// look is how often the office's files are looked at for a change.
const look = 2 * time.Second

// Read reads the office in a directory as serving does: the office, with
// the problems Corbie's parser finds in its bridges and in each team's
// request machine. A reading that is refused gives office.Problems.
func Read(dir string) (*requests.Reading, error) {
	o, err := office.Read(dir, shipped.Files)
	if err != nil {
		return nil, err
	}
	bridges.Check(o)
	return requests.NewReading(o), nil
}

// Problems gives problems counted, then each on a line of its own.
func Problems(problems []office.Problem) string {
	counted := fmt.Sprintf("%d problems:", len(problems))
	if len(problems) == 1 {
		counted = "1 problem:"
	}
	for _, p := range problems {
		counted += "\n  " + p.String()
	}
	return counted
}

// A Bulldog serves one office on a Corbie.
type Bulldog struct {
	Engine  *requests.Engine
	Bridges *bridges.Bridges
	Screens *screens.Screens

	log   *log.Logger
	dir   string
	files string // the office's files when it was read last
}

// New makes a Bulldog that serves an office on a Corbie, from a reading of
// the office and with its secret. The screens are reached at the address,
// such as https://bulldog.example. What goes wrong, and each reading of
// the office, is written to out, a line at a time.
func New(corbie *client.Client, r *requests.Reading, secret []byte, address string, out io.Writer) *Bulldog {
	b := &Bulldog{log: log.New(out, "", log.LstdFlags), dir: r.Office.Dir}
	b.files, _ = files(b.dir)
	report := func(err error) { b.log.Print(err) }
	b.Bridges = bridges.New(corbie, secret)
	b.Engine = requests.New(corbie, b.Bridges, secret, r)
	b.Bridges.Engine = b.Engine
	b.Screens = screens.New(corbie, b.Engine, b.Bridges, secret, address)
	b.Engine.Report, b.Bridges.Report, b.Screens.Report = report, report, report
	return b
}

// Run serves the office, with the screens on a listener, until the context
// ends, then stops each part and waits for it. It returns an error when a
// part ends before then.
func (b *Bulldog) Run(ctx context.Context, l net.Listener) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	if problems := b.Engine.Reading().Office.Problems; len(problems) > 0 {
		b.log.Print("The office has " + Problems(problems))
	}
	server := &http.Server{Handler: b.Screens, BaseContext: func(net.Listener) context.Context { return ctx }, ReadHeaderTimeout: 30 * time.Second}
	ended := make(chan error, 3)
	go func() { ended <- b.Engine.Run(ctx) }()
	go func() { ended <- b.Bridges.Run(ctx) }()
	go func() {
		err := server.Serve(l)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		ended <- err
	}()
	ticker := time.NewTicker(look)
	defer ticker.Stop()
	running, errs := 3, []error{}
serving:
	for {
		select {
		case <-ctx.Done():
			break serving
		case err := <-ended:
			running, errs = running-1, append(errs, err)
			break serving
		case <-ticker.C:
			b.reread()
		}
	}
	stop()
	errs = append(errs, server.Shutdown(context.Background()))
	for range running {
		errs = append(errs, <-ended)
	}
	return errors.Join(errs...)
}

// reread reads the office again when its files have changed since it was
// read last. A reading that is refused leaves the last good one in force,
// with the refused reading's problems in place of its own.
func (b *Bulldog) reread() {
	now, err := files(b.dir)
	if err != nil {
		b.log.Printf("The office's files cannot be looked at: %v", err)
		return
	}
	if now == b.files {
		return
	}
	b.files = now
	last := b.Engine.Reading()
	r, err := Read(b.dir)
	var refused office.Problems
	switch {
	case err == nil && r.Office.ID != last.Office.ID:
		refused = office.Problems{{File: ".bulldog/id", Text: fmt.Sprintf("the office's id was %s and is now %s; Bulldog serves the office as %s until it is started again", last.Office.ID, r.Office.ID, last.Office.ID)}}
	case errors.As(err, &refused):
	case err != nil:
		b.log.Printf("The office cannot be read again: %v", err)
		return
	case len(r.Office.Problems) > 0:
		b.Engine.Swap(r)
		b.log.Print("The office was read again, with " + Problems(r.Office.Problems))
		return
	default:
		b.Engine.Swap(r)
		b.log.Print("The office was read again.")
		return
	}
	o := *last.Office
	o.Problems = refused
	b.Engine.Swap(&requests.Reading{Office: &o, Machines: last.Machines})
	b.log.Print("The office was read again and refused, so the last good reading stays in force. " + Problems(refused))
}

// files gives the name, size and time of change of each file of an
// office, but those whose names, or whose directories' names, start with a
// dot, such as .git and .bulldog.
func files(dir string) (string, error) {
	var seen strings.Builder
	err := filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case name != dir && strings.HasPrefix(d.Name(), ".") && d.IsDir():
			return filepath.SkipDir
		case strings.HasPrefix(d.Name(), ".") || d.IsDir():
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&seen, "%s %d %d\n", name, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return seen.String(), err
}
