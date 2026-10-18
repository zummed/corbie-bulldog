package server

import (
	"net"
	"strings"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/testkit"
	"github.com/zummed/corbie-bulldog/internal/work"
)

func TestServerStartsInOrder(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	before := -1
	b.heard = func(line string) {
		if strings.Contains(line, "listening on") {
			before = len(b.listed(b.identity(), nil))
		}
	}
	s := b.serve()
	s.answered(s.chat("kit", "", "the kitchen socket sparked"))
	lines := s.lines()
	corbie, office, listening, sam, kit := index(lines, "on the Corbie of the controller"), index(lines, "The house: 2 teams, 8 workers, 2 people"),
		index(lines, "listening on 127.0.0.1:"), index(lines, "sam, open http://127.0.0.1:"), index(lines, "kit, open http://127.0.0.1:")
	// What readying did comes first, and then which Corbie, before
	// anything else.
	if corbie < 0 || index(lines[:corbie], "git repository now") < 0 || office != corbie+1 || listening <= office || sam <= listening || kit <= sam {
		t.Errorf("the log's lines come out of order:\n%s", strings.Join(lines, "\n"))
	}
	// No run is started before the listening line, and the server's own
	// is among those after it.
	if before != 0 {
		t.Errorf("%d runs were started before the server listened", before)
	}
	if runs := b.listed(b.identity(), map[string]string{work.LabelKind: work.KindLive}); len(runs) != 1 || runs[0].HasEnded() {
		t.Errorf("the server's own runs are %v", kinds(runs))
	}
}

// kinds gives the kind and status of each run.
func kinds(runs []*client.Run) []string {
	var out []string
	for _, r := range runs {
		out = append(out, r.Labels[work.LabelKind]+" "+r.Status)
	}
	return out
}

func TestServerListensBeforeAnyWork(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	s := b.serve(func(o *Options) { o.Listen = taken.Addr().String() })
	err = s.wait()
	if err == nil || !strings.Contains(err.Error(), taken.Addr().String()) {
		t.Fatalf("a server that cannot listen ended with %v", err)
	}
	if s.begun() != nil {
		t.Error("a server that cannot listen began")
	}
	if runs := b.listed(b.identity(), nil); len(runs) != 0 {
		t.Errorf("a server that cannot listen started %v", kinds(runs))
	}
	if s.logged("open http") || s.logged("listening on") {
		t.Errorf("a server that cannot listen printed:\n%s", strings.Join(s.lines(), "\n"))
	}
}

func TestServerNeedsNothingElseOfCorbie(t *testing.T) {
	t.Parallel()
	// The house runs to an answer on a Corbie in the test's process, with
	// nothing listening for jobs and no token set.
	b := newBench(t, house(t))
	b.says(func(turn testkit.Turn) testkit.Reply { return testkit.Say("Sparks will look at it.") })
	s := b.serve()
	n := s.chat("kit", "", "the kitchen socket sparked")
	s.answered(n)
	thread, err := s.areasOf().Requests.Thread(n)
	if err != nil || thread[len(thread)-1].Text != "Sparks will look at it." {
		t.Errorf("the request's thread is %+v, %v", thread, err)
	}
}
