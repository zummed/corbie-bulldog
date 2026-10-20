package decisions_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/changes"
	"github.com/zummed/corbie-bulldog/internal/decisions"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/requests"
)

// draft has the builder write files into a request's draft and call
// put_draft, which the office tools have Corbie hold, and gives the held
// put's decision.
func (b *bench) draft(n int64, why string, files map[string]string) *decisions.Decision {
	b.t.Helper()
	for path, text := range files {
		args, _ := json.Marshal(map[string]string{"path": path, "text": text})
		if _, err := b.ch.Answer(ctx, changes.Call{Request: n, Worker: "builder", Tool: changes.ToolWrite, Args: args}); err != nil {
			b.t.Fatal(err)
		}
	}
	job := decisions.Job{Name: changes.ToolPut, Asked: &decisions.Asked{Does: "Put your draft", Args: raw(map[string]string{"why": why})}}
	return b.one("ask of "+why, office.Top, n, []string{"builder", "steward"}, job)
}

// runs stays until a held put's ask is answered approving it, and does the
// call its job then runs, as work hands it on; it gives how the change
// ended.
func (b *bench) runs(dec *decisions.Decision) changes.Outcome {
	b.t.Helper()
	if v := b.verdict(dec.Ask); !slices.Equal(v.Approve, []string{dec.Job.Name}) {
		b.t.Fatalf("the put's ask is answered %+v", v)
	}
	args, _ := json.Marshal(map[string]string{"why": dec.Put.Why})
	if _, err := b.ch.Answer(ctx, changes.Call{Request: dec.Request, Worker: "builder", Tool: changes.ToolPut, Args: args}); err != nil {
		b.t.Fatal(err)
	}
	return b.outcome(dec.Put.ID)
}

// putOwn has a person put a change of their own from the screens, naming
// Kit in the garden.
func (b *bench) putOwn(person, why string) changes.Put {
	b.t.Helper()
	edited := kitInTheGarden(b.t)["garden/team.yaml"]
	changed, _, err := b.ch.Edited(changes.Edit{Files: map[string]*string{"garden/team.yaml": &edited}})
	if err != nil {
		b.t.Fatal(err)
	}
	check, err := b.ch.Check(ctx, changed, changes.Drafter{Name: person, Person: true})
	if err != nil {
		b.t.Fatal(err)
	}
	p, err := b.ch.PutOwn(ctx, person, why, changed, check.Mark)
	if err != nil {
		b.t.Fatal(err)
	}
	return p
}

// outcome gives how a change ended, once it has.
func (b *bench) outcome(put int64) changes.Outcome {
	b.t.Helper()
	var got *changes.Outcome
	b.until("the change has ended", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, o := range b.told {
			if o.Put == put {
				got = &o
			}
		}
		return got != nil
	})
	return *got
}

// kitInTheGarden is the garden's team file with Kit named in it.
func kitInTheGarden(t *testing.T) map[string]string {
	text := house(t)["garden/team.yaml"]
	return map[string]string{"garden/team.yaml": strings.Replace(text, "people: [sam]", "people: [sam, kit]", 1)}
}

func TestAHeldPutIsADecisionOfTheTeamWhoseFilesItWrites(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "let me into the garden")
	// It writes the garden's team file, so the house decides it.
	dec := b.draft(n, "Name Kit in the garden\n\nKit helps with the lawn now.", kitInTheGarden(t))
	if dec.Kind != decisions.OfAction || dec.Job.Name != changes.ToolPut || dec.Put == nil || dec.Team != office.Top || dec.Whose.Name != "builder" || !slices.Equal(dec.Asked, []string{"sam"}) {
		t.Fatalf("the put's decision is %+v", dec)
	}
	text := b.waitingFor("sam")[0].Text
	for _, want := range []string{"builder", "Kit helps with the lawn now.", "kit is named in the garden", "garden/team.yaml, changed:", "- people: [sam]", "+ people: [sam, kit]"} {
		if !strings.Contains(text, want) {
			t.Errorf("Sam is not shown %q: %s", want, text)
		}
	}
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	if text, _ := os.ReadFile(filepath.Join(b.dir, "garden", "team.yaml")); string(text) != house(t)["garden/team.yaml"] {
		t.Fatal("the put was written before its job ran")
	}
	if o := b.runs(dec); o.Ended != changes.Written || o.Decider != "sam" {
		t.Fatalf("the change ended %+v", o)
	}
	history, err := b.ch.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	if history[0].DecidedBy != "sam" || history[0].By != "builder" {
		t.Fatalf("the newest commit is %+v", history[0])
	}
	has(t, b.thread(n), "decided by sam", "approved by Sam")
	has(t, b.seen(n, "kit"), "Name Kit in the garden: approved by Sam")
	// A note of the garden's alone is the garden's to decide.
	other := b.chat("kit", office.Top, "the shed")
	if dec := b.draft(other, "The shed", map[string]string{"garden/notes/shed.md": "# The shed\n\nLocked.\n"}); dec.Team != "garden" {
		t.Errorf("a change of the garden's note is %s's to decide", dec.Team)
	}
	// The garden, within the house, is not shown what the house was asked.
	for _, m := range b.waitingFor("sam") {
		if strings.Contains(m.Text, "The shed") && strings.Contains(m.Text, "For request") {
			t.Errorf("the garden is shown the house's request: %s", m.Text)
		}
	}
}

func TestAHeldPutsAskWaitsUntilItsYesIsKept(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "let me into the garden")
	dec := b.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	if _, err := b.db.Exec(`CREATE TRIGGER full BEFORE UPDATE ON changes_puts BEGIN SELECT RAISE(FAIL, 'the disk is full'); END`); err != nil {
		t.Fatal(err)
	}
	// Sam's yes is taken, but Changes cannot keep it, so Corbie is not
	// answered: the job would run a write that names nobody.
	r, err := b.d.Decide(ctx, dec.ID, b.reqs.Person("sam"), decisions.SaysYes, "")
	if !r.Taken || err == nil || !strings.Contains(err.Error(), "is not answered until") {
		t.Fatalf("a yes Changes could not keep is answered %+v, %v", r, err)
	}
	if got := b.get(dec.ID); got.Ended != decisions.Yes || b.answers(dec.Ask) != 0 {
		t.Fatalf("the ask was answered before its yes was kept, %d times: %+v", b.answers(dec.Ask), got)
	}
	if _, err := b.db.Exec(`DROP TRIGGER full`); err != nil {
		t.Fatal(err)
	}
	// As Bulldog starts again, the yes is kept, and the ask found waiting
	// is answered; the write its job runs names Sam.
	b.restart()
	b.held(decisions.Ask{Run: dec.Ask, Team: office.Top, Request: n, Asking: []string{"builder", "steward"}, Jobs: []decisions.Job{*dec.Job}})
	if o := b.runs(dec); o.Ended != changes.Written || o.Decider != "sam" || b.answers(dec.Ask) != 1 {
		t.Fatalf("the change ended %+v, its ask answered %d times", o, b.answers(dec.Ask))
	}
}

func TestAnUnfitPutIsRefusedByBulldog(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "do nothing")
	dec := b.one("ask of nothing", office.Top, n, []string{"builder", "steward"}, decisions.Job{Name: changes.ToolPut, Asked: &decisions.Asked{Args: raw(map[string]string{"why": "Nothing"})}})
	if got := b.ended(dec.ID); got.Ended != decisions.Refused || got.By.Kind != requests.IsBulldog || !strings.Contains(got.By.Note, "empty") || len(got.WasAsked) != 0 {
		t.Fatalf("an empty put is %+v", got)
	}
	if v := b.verdict(dec.Ask); len(v.Approve) != 0 || !strings.Contains(v.Note, "empty") {
		t.Errorf("the ask is answered %+v", v)
	}
}

func TestRefusedChangeNotWritten(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "let me into the garden")
	dec := b.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	b.decide(dec.ID, "sam", decisions.SaysRefuse, "not the fixer's job")
	o := b.outcome(dec.Put.ID)
	if o.Ended != changes.Refused || o.Decider != "sam" || o.Note != "not the fixer's job" {
		t.Fatalf("the change ended %+v", o)
	}
	if v := b.verdict(dec.Ask); len(v.Approve) != 0 || !strings.Contains(v.Note, "not the fixer's job") {
		t.Errorf("the ask is answered %+v", v)
	}
	if text, err := os.ReadFile(filepath.Join(b.dir, "garden", "team.yaml")); err != nil || string(text) != house(t)["garden/team.yaml"] {
		t.Fatalf("the refused change was written: %s, %v", text, err)
	}
	e := b.entries(n, decisions.KindDecided)[0]
	if e.To[0].Name != "builder" || !strings.Contains(e.Text, "not the fixer's job") {
		t.Fatalf("the builder is told %+v", e)
	}
}

func TestAHeldPutIsWalkedOverItsList(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  put_draft: [./no-secrets.js, sam]\n", 1)
	files["no-secrets.js"] = `return { says: "yes" }`
	b := newBench(t, files)
	n := b.chat("kit", office.Top, "let me into the garden")
	dec := b.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	r := b.runner.next(t)
	change, _ := r.Input["change"].(map[string]any)
	if r.Name != "no-secrets.js" || r.Input["kind"] != decisions.OfAction || change["why"] != "Name Kit in the garden" || !strings.Contains(r.Input["ends"].(string), "Your yes or your refusal ends it") {
		t.Fatalf("the put is put to %s, given %v", r.Name, r.Input)
	}
	// A rule's yes approves it, as any held action.
	r.says(decisions.SaysYes, "no secret in it")
	if o := b.runs(dec); o.Ended != changes.Written || o.Decider != "no-secrets.js" {
		t.Fatalf("the change ended %+v", o)
	}
	other := b.chat("kit", office.Top, "and the shed")
	files["office.yaml"] = strings.Replace(files["office.yaml"], "put_draft: [./no-secrets.js, sam]", "put_draft: [./no-secrets.js: advise, sam]", 1)
	b.rewrite(files)
	dec = b.draft(other, "Name Kit in the shed", map[string]string{"notes/shed.md": "# The shed\n\nKit has a key.\n"})
	b.runner.next(t).says(decisions.SaysYes, "looks fine")
	b.until("Sam is asked", func() bool { return len(b.waitingFor("sam")) == 1 })
	if text := b.waitingFor("sam")[0].Text; !strings.Contains(text, "no-secrets.js recommends yes: looks fine") {
		t.Errorf("Sam is shown %s", text)
	}
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	if o := b.runs(dec); o.Ended != changes.Written || o.Decider != "sam" {
		t.Fatalf("the advised change ended %+v", o)
	}
}

func TestPersonsOwnChangeDecidedByTheirYes(t *testing.T) {
	t.Parallel()
	// In the house nobody is asked before Sam: it is written as it is put.
	h := newBench(t, house(t))
	p := h.putOwn("sam", "Kit helps in the garden")
	if o := h.outcome(p.ID); o.Ended != changes.Written || o.Decider != "sam" || len(h.runner.started()) != 0 {
		t.Fatalf("Sam's own change in the house ended %+v", o)
	}
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  put_draft: [./no-secrets.js, sam]\n", 1)
	files["no-secrets.js"] = `return { says: "refer" }`
	b := newBench(t, files)
	p = b.putOwn("sam", "Kit helps in the garden")
	b.runner.next(t).says(decisions.SaysRefuse, "not today")
	if o := b.outcome(p.ID); o.Ended != changes.Refused || o.Decider != "no-secrets.js" {
		t.Fatalf("Sam's own change ended %+v", o)
	}
	p = b.putOwn("sam", "Kit helps in the garden, again")
	b.runner.next(t).says(decisions.SaysRefer, "")
	if o := b.outcome(p.ID); o.Ended != changes.Written || o.Decider != "sam" {
		t.Fatalf("Sam's own change ended %+v", o)
	}
}

func TestChangePutAgainIsWithdrawn(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "let me into the garden")
	first := b.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	second := b.draft(n, "Name Kit in the garden, and say so", kitInTheGarden(t))
	if got := b.get(first.ID); got.Ended != decisions.Withdrawn {
		t.Fatalf("the change put first is %+v", got)
	}
	if text := b.refused(first.ID, b.reqs.Person("sam"), decisions.SaysYes); !strings.Contains(text, "withdrawn") {
		t.Fatalf("Sam's yes on the withdrawn change is answered %q", text)
	}
	if ws := b.waitingFor("sam"); len(ws) != 1 || ws[0].Entry != second.Entry {
		t.Fatalf("Sam waits on %+v", ws)
	}
}

// A put of the garden's alone waits on Sam, and the garden then leaves
// the office: nobody can decide it.
func TestChangeRefusedByBulldog(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "tidy the shed")
	dec := b.draft(n, "Note the shed", map[string]string{"garden/notes/beds.md": "# The beds\n"})
	if dec.Team != "garden" {
		t.Fatalf("the put is %s's", dec.Team)
	}
	if err := os.RemoveAll(filepath.Join(b.dir, "garden")); err != nil {
		t.Fatal(err)
	}
	read, err := office.Read(b.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.reqs.Read(read); err != nil {
		t.Fatal(err)
	}
	if told := b.d.Read(ctx, read); told != nil {
		t.Fatal(told)
	}
	got := b.get(dec.ID)
	const why = "nobody can decide this: garden is no longer in the office"
	if got.Ended != decisions.Refused || got.By.Kind != requests.IsBulldog || got.By.Note != why {
		t.Fatalf("the change nobody can decide is %+v", got)
	}
	if out := b.outcome(dec.Put.ID); out.Ended != changes.Refused || out.Decider != "Bulldog" || out.Note != why {
		t.Fatalf("the change ended %+v", out)
	}
	// Nothing is owed, so a restart has nothing to tell Changes again.
	b.restart()
	if got := b.get(dec.ID); got.Ended != decisions.Refused {
		t.Fatalf("after a restart the change nobody can decide is %+v", got)
	}
}
