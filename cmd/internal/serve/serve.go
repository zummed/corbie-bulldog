// Package serve serves an office: its requests engine, its bridges and its
// screens on a Corbie, with the office read again whenever a file of it
// changes. The program and the demo serve an office with it.
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

// Look is how often the office's files are looked at for a change.
const Look = 2 * time.Second

// Read reads the office in a directory as serving does: the office, with
// the problems Corbie's parser finds in its bridges and in each team's
// request machine.
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

// An Office is an office served on a Corbie.
type Office struct {
	Engine  *requests.Engine
	Bridges *bridges.Bridges
	Screens *screens.Screens

	log   *log.Logger
	dir   string
	files string // the office's files when it was read last
}

// New makes an office served on a Corbie from a reading of it, with the
// office's secret. The screens are reached at the address, such as
// https://bulldog.example. What goes wrong, and each reading of the office,
// is written to out, a line at a time.
func New(corbie *client.Client, r *requests.Reading, secret []byte, address string, out io.Writer) *Office {
	s := &Office{log: log.New(out, "", log.LstdFlags), dir: r.Office.Dir}
	s.files, _ = files(s.dir)
	report := func(err error) { s.log.Print(err) }
	s.Bridges = bridges.New(corbie, secret)
	s.Engine = requests.New(corbie, s.Bridges, secret, r)
	s.Bridges.Engine = s.Engine
	s.Screens = screens.New(corbie, s.Engine, s.Bridges, secret, address)
	s.Engine.Report, s.Bridges.Report, s.Screens.Report = report, report, report
	return s
}

// Run serves the office, with the screens on a listener, until the context
// ends, then stops each part and waits for it. It returns an error when a
// part ends before then.
func (s *Office) Run(ctx context.Context, l net.Listener) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	if problems := s.Engine.Reading().Office.Problems; len(problems) > 0 {
		s.log.Print("The office has " + Problems(problems))
	}
	server := &http.Server{Handler: s.Screens, BaseContext: func(net.Listener) context.Context { return ctx }, ReadHeaderTimeout: 30 * time.Second}
	ended := make(chan error, 3)
	go func() { ended <- s.Engine.Run(ctx) }()
	go func() { ended <- s.Bridges.Run(ctx) }()
	go func() {
		err := server.Serve(l)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		ended <- err
	}()
	look := time.NewTicker(Look)
	defer look.Stop()
	running, errs := 3, []error{}
serving:
	for {
		select {
		case <-ctx.Done():
			break serving
		case err := <-ended:
			running, errs = running-1, append(errs, err)
			break serving
		case <-look.C:
			s.reread()
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
func (s *Office) reread() {
	now, err := files(s.dir)
	if err != nil {
		s.log.Printf("The office's files cannot be looked at: %v", err)
		return
	}
	if now == s.files {
		return
	}
	s.files = now
	r, err := Read(s.dir)
	var refused office.Problems
	switch {
	case errors.As(err, &refused):
		last := s.Engine.Reading()
		o := *last.Office
		o.Problems = refused
		s.Engine.Swap(&requests.Reading{Office: &o, Machines: last.Machines})
		s.log.Print("The office was read again and refused, so the last good reading stays in force. " + Problems(refused))
	case err != nil:
		s.log.Printf("The office cannot be read again: %v", err)
	case len(r.Office.Problems) > 0:
		s.Engine.Swap(r)
		s.log.Print("The office was read again, with " + Problems(r.Office.Problems))
	default:
		s.Engine.Swap(r)
		s.log.Print("The office was read again.")
	}
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
