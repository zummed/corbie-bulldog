package work

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/zummed/corbie/client"

	"github.com/zummed/corbie-bulldog/internal/bridges"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// rules is the house with two more rules: one a bare script that would
// reach the network, and one a definition allowed to read the
// environment.
func rules(t *testing.T) map[string]string {
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  order_part: [./small-orders.js, sam]",
		"  order_part: [./small-orders.js, sam]\n  call_out: [./phone.js, sam]\n  look_round: [./envy.yaml, sam]", 1)
	files["phone.js"] = "return { says: \"yes\", note: String(http.get(\"https://example.com\")) }\n"
	files["envy.yaml"] = "flow: envy\nstages:\n  a: { job: { script: ./envy.js, input: \"${input}\", allow: [env] } }\noutput: ${stages.a.output}\n"
	files["envy.js"] = "return { says: \"yes\", note: String(env.get(\"HOME\", { otherwise: \"none\" }) !== null) }\n"
	return files
}

// deciding gives a decider's start, as the decisions give one.
func deciding(kind, name, team string, request int64, decision int) decisions.Start {
	return decisions.Start{Decision: int64(decision), Step: 1, Kind: kind, Name: name, Team: team, Request: request,
		Labels: map[string]string{requests.LabelTeam: team, requests.LabelRequest: fmt.Sprint(request), LabelKind: decisions.Deciding,
			decisions.LabelDecision: fmt.Sprint(decision), decisions.LabelStep: "1"},
		Input: map[string]any{"kind": "action", "action": map[string]any{"args": map[string]any{"pounds": 12}}}, Said: "Order a socket for £12?"}
}

func TestEveryRunCarriesTheOfficesLabels(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	if _, err := b.w.decider(ctx, deciding(office.IsRule, "small-orders.js", "office", n, 1)); err != nil {
		t.Fatal(err)
	}
	b.listening("telegram")
	runs := b.listed(nil)
	kinds := map[string]int{}
	for _, r := range runs {
		kind := r.Labels[LabelKind]
		kinds[kind]++
		_, numbered := r.Labels[requests.LabelRequest]
		if r.Labels[LabelBulldog] != identity || r.Labels[requests.LabelTeam] == "" ||
			(kind == KindWork || kind == decisions.Deciding) != (r.Labels[requests.LabelRequest] == fmt.Sprint(n)) ||
			kind == KindClock && numbered {
			t.Errorf("the run %s is labelled %v", r.ID, r.Labels)
		}
	}
	// The house's two routines and the garden's, and the Telegram bridge's
	// listening run.
	if kinds[KindWork] != 1 || kinds[decisions.Deciding] != 1 || kinds[KindClock] != 3 || kinds[bridges.KindIn] != 1 || len(runs) != 6 {
		t.Errorf("the runs listed are of kinds %v", kinds)
	}
}

func TestARunBelowCarriesTheSameLabels(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.says(func(turn testkit.Turn) testkit.Reply {
		switch {
		case strings.Contains(turn.Prompt(), "socket sparked") && turn.Number == 1:
			return testkit.Use("fixer", map[string]any{"job": "mend the kitchen socket"})
		case turn.Prompt() == "mend the kitchen socket" && turn.Number == 1:
			return testkit.Use("ask_person", map[string]any{"question": "tell me when the socket arrives"})
		}
		return testkit.Say("done")
	})
	n := b.chat("kit", "", "the kitchen socket sparked")
	steward := b.started(n, 1)
	var call *client.Run
	b.until("the fixer's call waits", func() bool {
		runs, err := b.corbie.List(ctx, client.Filter{Labels: map[string]string{LabelBulldog: identity, requests.LabelRequest: fmt.Sprint(n)}})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			if r.Machine == "call" {
				call = r
			}
		}
		return call != nil
	})
	fixer, err := b.corbie.Show(ctx, call.Parent)
	if err != nil {
		t.Fatal(err)
	}
	if fixer.Machine != "fixer" || fixer.Parent != steward.Name || call.Labels[LabelKind] != KindWork || call.Labels[requests.LabelTeam] != "office" {
		t.Errorf("the call is %+v, below %+v", call, fixer)
	}
}

func TestWorkLabelsHoldNoWords(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	if _, err := b.w.decider(ctx, deciding(office.IsWorker, "clerk", "garden", n, 2)); err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile(`^[a-z0-9-]+$`)
	for _, r := range b.listed(nil) {
		for k, v := range r.Labels {
			if !plain.MatchString(v) || strings.Contains(v, "kit") || strings.Contains(v, "socket") || strings.Contains(v, "house") {
				t.Errorf("the run %s has the label %s=%q", r.ID, k, v)
			}
		}
	}
}

func TestWorkRunNotedBeforeStart(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	listening := b.listening("telegram")
	var mu sync.Mutex
	var seen []string
	b.reach.mu.Lock()
	b.reach.starting = func(s client.Start) error {
		n, err := b.w.noted(s.Run)
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err != nil:
			seen = append(seen, err.Error())
		case n == nil:
			seen = append(seen, s.Run+" not noted")
		default:
			seen = append(seen, fmt.Sprintf("%s %s %s %d", n.Kind, n.Team, n.Worker, n.Request))
		}
		return nil
	}
	b.reach.mu.Unlock()
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	if _, err := b.w.decider(ctx, deciding(office.IsRule, "small-orders.js", "office", n, 1)); err != nil {
		t.Fatal(err)
	}
	// A clock for a new routine, and the next listening run of a bridge.
	b.rewrite(map[string]string{"garden/team.yaml": strings.Replace(b.file("garden/team.yaml"), "routines:\n",
		"routines:\n  - { at: \"07:00\", to: gardener, say: \"Water the beds?\" }\n", 1)})
	b.hear(listening, 1)
	b.until("the next listening run is started", func() bool { return b.listening("telegram") != listening })
	mu.Lock()
	defer mu.Unlock()
	want := []string{fmt.Sprintf("work office steward %d", n), fmt.Sprintf("decide office small-orders.js %d", n), "clock garden gardener 0",
		"bridge-in office telegram 0"}
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Errorf("as each start was sent, the runs noted were %q", seen)
	}
}

func TestADecidersRunIsOfTheTeamItDecidesFor(t *testing.T) {
	t.Parallel()
	b := newBench(t, rules(t))
	g := b.chat("sam", "garden", "may the gate be opened?")
	b.settled(g, requests.Answered)
	out, err := b.w.decider(ctx, deciding(office.IsWorker, "clerk", "garden", g, 3))
	if err != nil || string(out) != `"done"` {
		t.Errorf("the clerk decided %s (%v)", out, err)
	}
	runs := b.listed(map[string]string{LabelKind: decisions.Deciding})
	if len(runs) != 1 || runs[0].Labels[requests.LabelTeam] != "garden" || runs[0].Labels[requests.LabelRequest] != fmt.Sprint(g) {
		t.Fatalf("the clerk's run is %v", runs)
	}
	for person, may := range map[string]bool{"sam": true, "kit": false} {
		if got, err := b.reqs.MayWatch(runs[0].Labels, person); err != nil || got != may {
			t.Errorf("%s may watch the clerk's run: %v (%v)", person, got, err)
		}
	}
	context := b.input(runs[0].ID)["context"].(string)
	holds(t, "the clerk's context", context, "deciding for The garden", "## lawn", "Order a socket for £12?")
	holdsNone(t, "the clerk's context", context, "The fuse board is under the stairs.")

	// A bare script is given no permission: its call to a host is
	// refused. A rule that is a definition has what its file allows.
	n := b.chat("kit", "", "the kitchen socket sparked")
	b.settled(n, requests.Answered)
	if _, err := b.w.decider(ctx, deciding(office.IsRule, "phone.js", "office", n, 4)); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("the bare script reached the network: %v", err)
	}
	out, err = b.w.decider(ctx, deciding(office.IsRule, "envy.yaml", "office", n, 5))
	if err != nil || !strings.Contains(string(out), `"yes"`) {
		t.Errorf("the rule that is a definition decided %s (%v)", out, err)
	}
	for _, r := range b.listed(map[string]string{LabelKind: decisions.Deciding, decisions.LabelDecision: "4"}) {
		def, err := b.corbie.Definition(ctx, r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if def.Type != "flow" || len(def.Allow) != 0 || !strings.Contains(string(def.Body), "./phone.js") {
			t.Errorf("the bare script ran as %s %s allowing %v", def.Type, def.Body, def.Allow)
		}
	}
}
