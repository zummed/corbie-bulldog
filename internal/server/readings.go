package server

import (
	"context"
	"database/sql"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/work"
)

// readings says what each reading of the office found: the office's line
// once it has read well, each problem new in a reading, and a link for
// each person new in a good one, whose key it makes, once every area has
// taken it, as taken gives it and took tells.
type readings struct {
	say    func(format string, args ...any)
	db     *sql.DB
	keeper *office.Keeper
	clock  work.Clock
	dir    string
	url    string
	taken  func() *office.Office
	took   chan struct{}

	// logged are the problems of the latest reading, as they were said;
	// read is the good reading the people's keys were last made for.
	logged map[string]bool
	read   *office.Office
	// named says the office's line has been said.
	named bool
	// changed, when it is set, is told each time a reading is not as the
	// one before it: read well again, or with other problems.
	changed func()
	shown   *office.Office
}

// office says the office's name with how many teams, workers and people
// it has, or that it does not read.
func (r *readings) office() {
	o := r.keeper.Office()
	if o == nil {
		r.say("the office in %s does not read well, so it starts no work, and nobody signs in until it does", r.dir)
		return
	}
	r.named = true
	r.say("%s: %s, %s, %s", o.Name(), count(len(o.Teams), "team", "teams"), count(len(o.Workers), "worker", "workers"), count(len(o.People), "person", "people"))
}

// problems says each problem of the latest reading that was not said of
// the reading before it, and whether the problems are not those before.
func (r *readings) problems() bool {
	now := map[string]bool{}
	other := false
	for _, p := range r.keeper.Problems() {
		line := p.String()
		if !r.logged[line] && !now[line] {
			r.say("a problem: %s", line)
			other = true
		}
		now[line] = true
	}
	other = other || len(now) != len(r.logged)
	r.logged = now
	return other
}

// links makes a key for each person new in the latest reading every area
// has taken, and destroys the key of each who has left it, and the
// password of each who may no longer set one, and gives a link to each
// person new in it: to every person, at the first. A link is given only
// once every area has taken the reading, so that whoever opens it finds
// the office as it read. Before them it says, of each sign-in method new
// in the reading, the address its provider sends the browser back to.
func (r *readings) links() {
	o := r.taken()
	if o == nil || o == r.read {
		return
	}
	made, gone, unset, err := keyed(r.db, o, r.clock.Now())
	if err != nil {
		r.say("%v", err)
		return
	}
	for _, person := range gone {
		r.say("%s is no longer in the office, so their key is destroyed", person)
	}
	for _, person := range unset {
		r.say("%s's entry no longer says password: true, so their password is destroyed", person)
	}
	if !r.named {
		r.office()
	}
	for _, name := range o.SignInOrder {
		if r.read == nil || r.read.SignIns[name] == nil {
			r.say("%s signs people in by their provider sending them back to %s, which the provider must have as a redirect URI", name, backOf(r.url, name))
		}
	}
	given := made
	if r.read == nil {
		given = o.Everyone
	}
	r.read = o
	for _, person := range given {
		key, err := keyOf(r.db, person)
		if err != nil {
			r.say("%v", err)
			continue
		}
		r.say("%s, open %s/?key=%s", person, r.url, key)
	}
}

// tell is told, on any goroutine, that every area has taken a new
// reading, for its links to be given.
func (r *readings) tell() {
	select {
	case r.took <- struct{}{}:
	default:
	}
}

// watch looks at the folder each time the clock moves, as moved tells,
// until the context ends, and says what each new reading found, and gives
// the links of each reading every area has taken.
func (r *readings) watch(ctx context.Context, moved <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.took:
			r.links()
			continue
		case <-moved:
		}
		r.keeper.Look()
		other := r.problems()
		r.links()
		if o := r.keeper.Office(); (other || o != r.shown) && r.changed != nil {
			r.shown = o
			r.changed()
		}
	}
}
