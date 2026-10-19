package server

import (
	"context"
	"database/sql"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/work"
	"github.com/zummed/corbie-bulldog/shipped"
)

// open opens every area of the office in its store and wires them to its
// work, which is wired to Corbie. Nothing is started.
func open(ctx context.Context, db *sql.DB, opt Options, keeper *office.Keeper) (Areas, error) {
	now := opt.Clock.Now
	reqs, err := requests.Open(db, now)
	if err != nil {
		return Areas{}, err
	}
	ch, err := changes.Open(ctx, db, opt.Dir, keeper)
	if err != nil {
		return Areas{}, err
	}
	ch.Now = now
	ch.Shipped = shipped.Bridge
	d, err := decisions.Open(db, reqs, ch)
	if err != nil {
		return Areas{}, err
	}
	d.Now = now
	ch.Decide = d.Change
	b, err := bridges.Open(db, reqs, now)
	if err != nil {
		d.Close()
		return Areas{}, err
	}
	a := work.Areas{Keeper: keeper, Requests: reqs, Decisions: d, Changes: ch, Bridges: b}
	w, err := work.New(db, opt.Corbie, opt.Clock, opt.Identity.ID, a)
	if err != nil {
		d.Close()
		b.Close()
		return Areas{}, err
	}
	return Areas{Areas: a, Work: w}, nil
}
