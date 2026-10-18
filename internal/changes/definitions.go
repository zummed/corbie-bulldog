package changes

import (
	"path"
	"slices"
	"strings"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie/machine"
)

// machines walks a definition in a file and every machine it runs, as
// Corbie read them, calling at with each and the file it is written in. A
// machine named by its path is in that file, beside the one that names it;
// one written in place is in the file of the one it is written in. Where at
// says no, the machines that one runs are not walked.
func machines(file string, d *machine.Draft, at func(file string, d *machine.Draft) bool) {
	if d == nil || !at(file, d) {
		return
	}
	for _, name := range sortedKeys(d.Children) {
		in := file
		if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
			in = path.Join(path.Dir(file), name)
		}
		machines(in, d.Children[name], at)
	}
}

// filesOf gives the files of the office one definition names directly: its
// scripts and the files they load, by their paths in the office. Corbie's
// own are none of them.
func filesOf(file string, d *machine.Draft) []string {
	var out []string
	for name := range d.Files {
		if !strings.HasPrefix(name, "corbie:") {
			out = append(out, path.Join(path.Dir(file), name))
		}
	}
	return out
}

// uses gives the files of the office a definition in a file uses, its own
// among them, as Corbie read it: those of every machine it runs, written in
// place or in a file of its own. A definition Corbie did not read uses its
// own file.
func uses(file string, d *machine.Draft) []string {
	out := []string{file}
	machines(file, d, func(at string, d *machine.Draft) bool {
		out = append(out, at)
		out = append(out, filesOf(at, d)...)
		return true
	})
	slices.Sort(out)
	return slices.Compact(out)
}

// direct gives the files a worker's own definition uses, as Corbie read
// it: its own, its scripts, and the machines it runs with what they use,
// but for another worker's definition, whose files are that worker's. A
// tools file Corbie leaves no trace of is taken from all the worker's
// definition read, so that what may be another's is never taken as its
// alone.
func direct(o *office.Office, w *office.Worker) []string {
	if w.Draft == nil {
		return w.Uses
	}
	others := otherWorkers(o, w.Name)
	out := []string{}
	machines(w.File, w.Draft, func(at string, d *machine.Draft) bool {
		out = append(out, at)
		if others[at] {
			return false
		}
		out = append(out, filesOf(at, d)...)
		return true
	})
	for _, f := range w.Uses {
		if ext := path.Ext(f); (ext == ".yaml" || ext == ".yml") && !others[f] {
			out = append(out, f)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// otherWorkers gives the files of every worker's definition but one's.
func otherWorkers(o *office.Office, but string) map[string]bool {
	out := map[string]bool{}
	for name, w := range o.Workers {
		if name != but {
			out[w.File] = true
		}
	}
	return out
}

// usedIn gives every file of the office that a definition uses, but those
// only one worker's uses, where one is named: each worker's, each rule's
// and each bridge machine's. A bridge Corbie could not read may use any
// file of its folder.
func usedIn(o *office.Office, but string) map[string]bool {
	out := map[string]bool{}
	add := func(files []string) {
		for _, f := range files {
			out[f] = true
		}
	}
	for name, w := range o.Workers {
		if name != but {
			add(direct(o, w))
		}
	}
	for file, r := range o.Rules {
		add(uses(file, r.Draft))
	}
	for name, b := range o.Bridges {
		for _, which := range []string{"in", "out"} {
			add(uses("bridges/"+name+"/"+which+".yaml", machineOf(b, which)))
		}
	}
	return out
}

// alone gives the files only a worker's definition uses, but for its own:
// those no other worker, rule or bridge machine uses. Nothing in a
// bridge's folder is one, since a bridge whose machines Corbie could not
// read may use any of it.
func alone(o *office.Office, w *office.Worker) []string {
	others := usedIn(o, w.Name)
	var out []string
	for _, f := range w.Uses {
		if f != w.File && !others[f] && !strings.HasPrefix(f, "bridges/") {
			out = append(out, f)
		}
	}
	return out
}

// teamOf gives the team whose own folder a file of the office is in: the
// nearest team's folder around it.
func teamOf(o *office.Office, file string) string {
	team, deepest := office.Top, -1
	for name, t := range o.Teams {
		if (t.Dir == "" || strings.HasPrefix(file, t.Dir+"/")) && len(t.Dir) > deepest {
			team, deepest = name, len(t.Dir)
		}
	}
	return team
}

// takes gives where a worker moving to a team's folder takes each file
// only its definition uses, by the paths of both: from its team's own
// folder to the same place in the new one's. A file in a team's folder
// within its own, or outside it, stays where it is.
func takes(o *office.Office, w *office.Worker, to string) map[string]string {
	from := o.Teams[w.Team].Dir
	out := map[string]string{}
	for _, f := range alone(o, w) {
		if teamOf(o, f) != w.Team {
			continue
		}
		rel := f
		if from != "" {
			rel = strings.TrimPrefix(f, from+"/")
		}
		out[f] = path.Join(to, rel)
	}
	return out
}

// A reach is what a definition may reach, as Corbie read it: the
// permissions it is allowed, and those it is refused, each refusal as often
// as it is written, since one taken from one job widens that job though
// another still has it; and what a refusal of it may hold back, which is
// everything it and the machines it runs are allowed, another worker's
// among them, with the limits its jobs put on the machines they start.
type reach struct {
	allow, deny, within []string
}

// reachOf gives what a definition may reach, as Corbie read it: every
// permission its jobs and those of the machines it runs are allowed and
// refused, but those of a machine that is another worker's definition,
// which are said of that worker, and of an agent's own turn, which are its
// model's.
func reachOf(o *office.Office, file string, d *machine.Draft) reach {
	var out reach
	others := map[string]bool{}
	for _, w := range o.Workers {
		others[w.File] = w.File != file
	}
	machines(file, d, func(at string, d *machine.Draft) bool {
		if others[at] {
			return false
		}
		own := ownOf(d.Definition)
		out.allow = append(out.allow, own.allow...)
		out.deny = append(out.deny, own.deny...)
		return true
	})
	machines(file, d, func(at string, d *machine.Draft) bool {
		own := ownOf(d.Definition)
		out.within = append(out.within, own.allow...)
		out.within = append(out.within, own.within...)
		return true
	})
	slices.Sort(out.allow)
	out.allow = slices.Compact(out.allow)
	slices.Sort(out.deny)
	slices.Sort(out.within)
	out.within = slices.Compact(out.within)
	return out
}

// ownOf gives what a definition's own part may reach: the lists written for
// the whole of it, those of its ask, and those of its jobs.
func ownOf(def *machine.Definition) reach {
	out := jobsOf(def)
	out.allow = append(slices.Clone(def.Allow), out.allow...)
	out.deny = append(slices.Clone(def.Deny), out.deny...)
	if def.Ask != nil {
		out.allow = append(out.allow, def.Ask.Allow...)
		out.deny = append(out.deny, def.Ask.Deny...)
	}
	return out
}

// jobsOf gives what the jobs of a definition's own part are allowed and
// refused, as Corbie places them. Each job's deny is its own. A machine's
// job allows nothing of its own, its allow being a limit on the machine,
// whose own definition says what it is allowed; what it refuses, the
// machine is refused. That limit is kept as what a refusal may hold back,
// since a machine made while running is allowed only what it says. An
// agent's turn is its model's, and the ask is the definition's own.
func jobsOf(def *machine.Definition) reach {
	var out reach
	for _, j := range machine.Jobs(def) {
		switch {
		case j.Place == "turn" || j.Place == "ask":
			continue
		case j.Job.IsMachine():
			out.within = append(out.within, j.Job.Allow...)
		default:
			out.allow = append(out.allow, j.Job.Allow...)
		}
		out.deny = append(out.deny, j.Job.Deny...)
	}
	return out
}
