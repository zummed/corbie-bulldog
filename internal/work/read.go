package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/requests"
)

// records gives what the runs of a request's team on it read, from
// Corbie's record of each run Bulldog noted for them and everything below
// it, for the requests to hand to those who may see it. A bridge's runs
// are never among them: their records hold what was said in every place
// the bridge listens in. A run noted that Corbie keeps no more is past
// keeping.
func (w *Work) records(labels map[string]string) ([]requests.Record, error) {
	ctx := context.Background()
	request, err := requestOf(labels)
	if err != nil {
		return nil, fmt.Errorf("the runs of %v cannot be read: they name no request", labels)
	}
	notes, err := scanNotes(w.db.Query(noteColumns+` WHERE request = ? AND team = ? ORDER BY rowid`, request, labels[requests.LabelTeam]))
	if err != nil {
		return nil, err
	}
	var records []requests.Record
	for _, note := range notes {
		if isBridge(note.Kind) || note.Status == unsent {
			continue
		}
		var entries []client.Entry
		_, err := w.corbie.Record(ctx, note.Name, func(e client.Entry) { entries = append(entries, e) })
		record := requests.Record{Run: note.Name, Team: note.Team, Worker: note.Worker}
		switch {
		case errors.Is(err, client.ErrNoRun):
			record.Gone = true
		case errors.Is(err, client.ErrNoRecord):
		case err != nil:
			return nil, fmt.Errorf("the record of %s could not be read: %w", note.Name, err)
		}
		if record.Events, err = json.Marshal(entries); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func requestOf(labels map[string]string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(labels[requests.LabelRequest], &n)
	return n, err
}

// isBridge says whether a run is a bridge's, which nobody watches.
func isBridge(kind string) bool { return strings.HasPrefix(kind, "bridge") }
