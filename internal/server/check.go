package server

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// Check reads the office in a folder as the server would, with no Corbie
// and nothing written, and says what it found: each team, its workers
// with the model each is sent and its tools, stages or jobs, who decides
// each that waits for a yes, each bridge, and how many runs the office has
// at work at once. Every problem of the reading is its error, in the words
// the reading gives it, each with its file.
func Check(dir string, out io.Writer) error {
	o, err := office.Read(dir)
	if o == nil {
		if problems, ok := err.(office.Problems); ok {
			return fmt.Errorf("the office in %s does not read, and %s", dir, listed(problems))
		}
		return err
	}
	for _, name := range o.Order {
		listTeam(out, o, o.Teams[name])
	}
	for _, name := range o.BridgeOrder {
		listBridge(out, o, o.Bridges[name])
	}
	for _, name := range o.SignInOrder {
		listSignIn(out, o, o.SignIns[name])
	}
	fmt.Fprintf(out, "%s at work at once\n", count(o.Runs, "run", "runs"))
	if len(o.Problems) > 0 {
		return errors.New("the office reads, and " + listed(o.Problems))
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

// listTeam says what a team is, and each of its workers.
func listTeam(out io.Writer, o *office.Office, t *office.Team) {
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
	disclosure := "none"
	if t.HasDisclosure {
		disclosure = written(t.Disclosure)
	}
	fmt.Fprintf(out, "  disclosure check: %s\n", disclosure)
	heed := "none"
	if h := t.Heed; h != nil {
		heed = h.Written
		if h.Row != "" {
			heed += ", with " + h.Row
		}
	}
	fmt.Fprintf(out, "  heed: %s\n", heed)
	for _, r := range t.Routines {
		fmt.Fprintf(out, "  routine at %s, to %s: %s\n", r.At, r.To, r.Say)
	}
	for _, name := range t.Workers {
		listWorker(out, o, t, o.Workers[name])
	}
}

// listWorker says what a worker is: its type, the model it is sent, its
// tools, a flow's stages or a statechart's jobs, and who decides each that
// waits for a yes.
func listWorker(out io.Writer, o *office.Office, t *office.Team, w *office.Worker) {
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
		fmt.Fprintf(out, "    %ss: %s\n", w.Part(), strings.Join(w.Tools, ", "))
	}
	for _, tool := range w.Held {
		if tool.Waits {
			fmt.Fprintf(out, "    %s waits for a yes, decided by %s\n", tool.Name, decidedBy(o, t.Name, tool.Name))
		}
	}
}

// decidedBy says who decides a key of a team, in the order the walk asks
// them: each rule by its path in the office and each worker, saying when
// it advises, then the people it ends at, asked at once; a list of a team
// around it is named.
func decidedBy(o *office.Office, team, key string) string {
	var said []string
	for _, s := range decisions.Walk(o, team, key) {
		who := s.Name
		if s.Kind == decisions.IsPeople {
			who = strings.Join(s.People, " or ")
		}
		if s.Advises {
			who += " (advises)"
		}
		if s.Team != team {
			who += fmt.Sprintf(", from the %s of %s", s.List, o.Teams[s.Team].Called)
		}
		said = append(said, who)
	}
	if len(said) == 0 {
		return "nobody, so what waits on it is refused"
	}
	return strings.Join(said, ", then ")
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

// listSignIn says what a sign-in method is, the folder it runs, and who
// may sign in by it.
func listSignIn(out io.Writer, o *office.Office, m *office.SignIn) {
	var who []string
	for _, name := range o.Everyone {
		if slices.ContainsFunc(o.People[name].Accounts, func(a office.Account) bool { return a.Method == m.Name }) {
			who = append(who, name)
		}
	}
	line := fmt.Sprintf("%s, a sign-in method running %s, for %s", m.Name, m.Dir(), orNone(strings.Join(who, ", ")))
	if !m.Runs() {
		line += ": it signs nobody in, for its problems below"
	}
	fmt.Fprintln(out, line)
}

// written gives a list of deciders as it is written, saying which
// advise.
func written(deciders []office.Decider) string {
	var names []string
	for _, d := range deciders {
		if d.Advises {
			names = append(names, d.Written+" (advises)")
			continue
		}
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
