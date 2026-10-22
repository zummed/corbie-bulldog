package bridges

import (
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/machine"
)

// defs are a bridge's definitions, filled out and checked by Corbie's
// parser.
type defs struct {
	in, out *machine.Draft
	made    string // the fingerprint of in, out and the settings
	direct  bool   // whether out takes direct
	err     error  // what Corbie's parser refuses
}

// usable says whether a bridge has definitions to run.
func (d *defs) usable() bool { return d != nil && d.err == nil }

// Check fills out each bridge of an office with Corbie's parser, and adds
// what the parser refuses to the bridge's problems and the office's, which
// stops the bridge. It is called once for a reading, before the office is
// shared, as requests.Make is.
func Check(o *office.Office) {
	for _, br := range o.Bridges {
		if len(br.Problems) > 0 {
			continue
		}
		_, _, problems := parse(o, br)
		br.Problems = append(br.Problems, problems...)
		o.Problems = append(o.Problems, problems...)
	}
}

// parse fills out a bridge's in.yaml and out.yaml, each with its paths
// read from the bridge's directory, and gives what Corbie's parser
// refuses, each a problem of its file.
func parse(o *office.Office, br *office.Bridge) (in, out *machine.Draft, problems []office.Problem) {
	load := func(name string) ([]byte, error) { return o.Load(path.Join(br.Dir, name)) }
	one := func(file string) *machine.Draft {
		src, err := load(file)
		var draft *machine.Draft
		if err == nil {
			draft, err = machine.Parse(src, load)
		}
		var found machine.Problems
		switch {
		case errors.As(err, &found):
			for _, p := range found {
				text := p.Text
				if p.Place != "" {
					text = p.Place + ": " + text
				}
				problems = append(problems, office.Problem{File: path.Join(br.Dir, cmp.Or(p.File, file)), Text: text})
			}
		case err != nil:
			problems = append(problems, office.Problem{File: path.Join(br.Dir, file), Text: err.Error()})
		}
		return draft
	}
	return one("in.yaml"), one("out.yaml"), problems
}

// definitions gives the definitions of each bridge of a reading that has
// no problem, by its name. They are made once for each reading.
func (b *Bridges) definitions(r *requests.Reading) map[string]*defs {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.reading == r {
		return b.defs
	}
	all := map[string]*defs{}
	for _, br := range r.Office.Bridges {
		if len(br.Problems) > 0 {
			continue
		}
		in, out, problems := parse(r.Office, br)
		d := &defs{in: in, out: out}
		if len(problems) > 0 {
			d.err = office.Problems(problems)
		} else {
			d.made, d.err = b.fingerprint(in, out, br.Settings)
			properties, _ := out.Definition.Input["properties"].(map[string]any)
			_, d.direct = properties["direct"]
		}
		all[br.Name] = d
	}
	b.reading, b.defs = r, all
	return all
}

// fingerprint gives a keyed hash of a bridge's definitions, with every
// file they name, and its settings: an in run labelled with it runs the
// bridge as the office has it.
func (b *Bridges) fingerprint(in, out *machine.Draft, settings map[string]any) (string, error) {
	data, err := json.Marshal(map[string]any{"in": in, "out": out, "settings": settings})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, b.secret)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)[:8]), nil
}
