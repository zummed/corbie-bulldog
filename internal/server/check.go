package server

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// Check reads the office in a folder as the server would, with no Corbie
// and nothing written, and says what it found: each team, its workers
// with the model each is sent and its tools, who decides each that waits
// for a yes, each bridge, and how many runs the office has at work at
// once. Every problem of the reading is its error, in the words the
// reading gives it, each with its file, together with any worker whose
// tools could not be read.
func Check(dir string, out io.Writer) error {
	o, err := office.Read(dir)
	if o == nil {
		if problems, ok := err.(office.Problems); ok {
			return fmt.Errorf("the office in %s does not read, and %s", dir, listed(problems))
		}
		return err
	}
	var failed []string
	for _, name := range o.Order {
		failed = append(failed, listTeam(out, o, o.Teams[name])...)
	}
	for _, name := range o.BridgeOrder {
		listBridge(out, o, o.Bridges[name])
	}
	fmt.Fprintf(out, "%s at work at once\n", count(o.Runs, "run", "runs"))
	var wrong []string
	if len(o.Problems) > 0 {
		wrong = append(wrong, "the office reads, and "+listed(o.Problems))
	}
	if len(failed) > 0 {
		wrong = append(wrong, "which tools wait for a yes cannot all be read: "+strings.Join(failed, "; "))
	}
	if len(wrong) > 0 {
		return errors.New(strings.Join(wrong, "\n"))
	}
	return nil
}

// listed says every problem, one to a line.
func listed(problems office.Problems) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:", count(len(problems), "problem", "problems"))
	for _, p := range problems {
		b.WriteString("\n  " + strings.ReplaceAll(p.String(), "\n", "\n    "))
	}
	return b.String()
}

// count says how many of something there are.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// listTeam says what a team is, and each of its workers, and gives what
// could not be read of them.
func listTeam(out io.Writer, o *office.Office, t *office.Team) []string {
	head := t.Called + ", the top team"
	if t.Within != "" {
		head = fmt.Sprintf("%s (%s), within %s", t.Called, t.Name, o.Teams[t.Within].Called)
	}
	if t.About != "" {
		head += ": " + t.About
	}
	fmt.Fprintln(out, head)
	fmt.Fprintf(out, "  people: %s\n", orNone(strings.Join(t.People, ", ")))
	fmt.Fprintf(out, "  lead: %s\n", orNone(t.Lead))
	fmt.Fprintf(out, "  decides everything: %s\n", decidedBy(o, t.Name, "everything"))
	for _, l := range t.Decide {
		if l.Key != "everything" {
			fmt.Fprintf(out, "  decides %s: %s\n", l.Key, decidedBy(o, t.Name, l.Key))
		}
	}
	release := "none"
	if t.HasRelease {
		release = written(t.Release)
	}
	fmt.Fprintf(out, "  release check: %s\n", release)
	for _, r := range t.Routines {
		fmt.Fprintf(out, "  routine at %s, to %s: %s\n", r.At, r.To, r.Say)
	}
	var failed []string
	for _, name := range t.Workers {
		if err := listWorker(out, o, t, o.Workers[name]); err != nil {
			failed = append(failed, err.Error())
		}
	}
	return failed
}

// listWorker says what a worker is: its type, the model it is sent, its
// tools, and who decides each that waits for a yes.
func listWorker(out io.Writer, o *office.Office, t *office.Team, w *office.Worker) error {
	line := fmt.Sprintf("  %s, %s", w.Name, article(w.Type))
	if w.Model != "" {
		line += ", with the model " + w.Model
		if w.ModelName != "" {
			if m, ok := o.Model(w.Team, w.ModelName); ok {
				line += fmt.Sprintf(", named %s, from %s", w.ModelName, o.Teams[m.Team].Called)
			}
		}
	}
	if !w.Runs() {
		line += ": it cannot run, for its problems below"
	}
	fmt.Fprintln(out, line)
	if len(w.Tools) > 0 {
		fmt.Fprintf(out, "    tools: %s\n", strings.Join(w.Tools, ", "))
	}
	waits, err := changes.Waits(w)
	for _, tool := range w.Tools {
		if waits[tool] {
			fmt.Fprintf(out, "    %s waits for a yes, decided by %s\n", tool, decidedBy(o, t.Name, tool))
		}
	}
	return err
}

// decidedBy says who a team's list of a key names, its everything list
// standing for a key it writes none of; an everything that names nobody
// is said with the people the team's decisions reach, and the team around
// it whose everything names them.
func decidedBy(o *office.Office, team, key string) string {
	if list, ok := o.List(team, key); ok && key != "everything" {
		return written(list)
	}
	if deciders := o.Everything(team); len(deciders) > 0 {
		return written(deciders)
	}
	reached := o.Reached(team)
	if len(reached) == 0 {
		return "nobody, so what waits on it is refused"
	}
	from := ""
	for t := o.Teams[o.Teams[team].Within]; t != nil; t = o.Teams[t.Within] {
		if slices.ContainsFunc(o.Everything(t.Name), func(d office.Decider) bool { return d.Kind == office.IsPerson }) {
			from = t.Called
			break
		}
	}
	return fmt.Sprintf("%s, from the everything of %s", strings.Join(reached, ", then "), from)
}

// listBridge says what a bridge is, and the places it listens in.
func listBridge(out io.Writer, o *office.Office, b *office.Bridge) {
	how := "a bridge"
	switch {
	case b.Hears:
		how = "a bridge that hears"
	case b.Listens:
		how = "a bridge that listens"
	}
	var places []string
	for _, p := range b.Places {
		place := fmt.Sprintf("%s, where %s is asked", p.Name, o.Teams[p.Team].Called)
		if p.Guests {
			place += ", where guests may ask"
		}
		places = append(places, place)
	}
	line := fmt.Sprintf("%s, %s, in %s", b.Name, how, orNone(strings.Join(places, "; ")))
	if !b.Runs() {
		line += ": it carries nothing, for its problems below"
	}
	fmt.Fprintln(out, line)
}

// written gives a list of deciders as it is written.
func written(deciders []office.Decider) string {
	var names []string
	for _, d := range deciders {
		names = append(names, d.Written)
	}
	return orNone(strings.Join(names, ", then "))
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// article gives a machine's type with its article.
func article(kind string) string {
	if kind == "agent" {
		return "an agent"
	}
	return "a " + kind
}
