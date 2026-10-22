package screens

import (
	"slices"

	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// How a person sees a request.
type sight int

const (
	unseen sight = iota // as one that is not there
	sealed              // its source, whom it waits on, how it ended; nothing the team read
	whole               // everything, as a person of its team
)

// sightOf says how a person sees a request. A person of its team sees it
// whole; a team with no people has the office's people as its people. The
// person who asked it, a person of the team that asked it, and a person a
// question or a decision of it waits on see it sealed.
func sightOf(o *office.Office, person string, q *requests.Request, waitsOn bool) sight {
	switch {
	case of(o, q.Team, person):
		return whole
	case q.By.Person == person, q.By.Team != "" && of(o, q.By.Team, person), waitsOn:
		return sealed
	}
	return unseen
}

// of says whether a person is of a team.
func of(o *office.Office, team, person string) bool {
	t := o.Team(team)
	return t != nil && slices.Contains(t.People, person)
}

// mayAct says whether a person may say more in a request's thread, or stop
// it: the person who asked it, and the people of its team.
func mayAct(o *office.Office, person string, q *requests.Request) bool {
	return q.By.Person == person || of(o, q.Team, person)
}
