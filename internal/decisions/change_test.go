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
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// draft has the builder write files into a request's draft and put it,
// and gives the change's decision.
func (b *bench) draft(n int64, why string, files map[string]string) *decisions.Decision {
	b.t.Helper()
	for path, text := range files {
		args, _ := json.Marshal(map[string]string{"path": path, "text": text})
		if _, err := b.ch.Answer(ctx, changes.Call{Request: n, Worker: "builder", Tool: changes.ToolWrite, Args: args}); err != nil {
			b.t.Fatal(err)
		}
	}
	args, _ := json.Marshal(map[string]string{"why": why})
	a, err := b.ch.Answer(ctx, changes.Call{Request: n, Worker: "builder", Tool: changes.ToolPut, Args: args})
	if err != nil {
		b.t.Fatal(err)
	}
	if a.Put == 0 {
		b.t.Fatalf("the change was not put: %s", a.Text)
	}
	decs, err := b.d.OfRequest(n)
	if err != nil {
		b.t.Fatal(err)
	}
	for _, dec := range slices.Backward(decs) {
		if dec.Put != nil && dec.Put.ID == a.Put {
			return dec
		}
	}
	b.t.Fatalf("the change %d is no decision", a.Put)
	return nil
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

func TestChangeIsADecisionOfTheTopTeam(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "let me into the garden")
	dec := b.draft(n, "Name Kit in the garden\n\nKit helps with the lawn now.", kitInTheGarden(t))
	if dec.Kind != decisions.OfChange || dec.Team != office.Top || dec.Whose.Name != "builder" || !slices.Equal(dec.Asked, []string{"sam"}) {
		t.Fatalf("the change's decision is %+v", dec)
	}
	text := b.waitingFor("sam")[0].Text
	for _, want := range []string{"builder", "Kit helps with the lawn now.", "kit is named in the garden", "garden/team.yaml"} {
		if !strings.Contains(text, want) {
			t.Errorf("Sam is not shown %q: %s", want, text)
		}
	}
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	if o := b.outcome(dec.Put.ID); o.Ended != changes.Written || o.Decider != "sam" {
		t.Fatalf("the change ended %+v", o)
	}
	history, err := b.ch.History("sam")
	if err != nil {
		t.Fatal(err)
	}
	if history[0].DecidedBy != "sam" || history[0].By != "builder" {
		t.Fatalf("the newest commit is %+v", history[0])
	}
	has(t, b.thread(n), "decided by sam", "yes from Sam")
	has(t, b.seen(n, "kit"), "Name Kit in the garden: yes from Sam")
}

func TestRefusedChangeNotWritten(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	n := b.chat("kit", office.Top, "let me into the garden")
	dec := b.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	b.decide(dec.ID, "sam", decisions.SaysRefuse, "not the fixer's job")
	o := b.outcome(dec.Put.ID)
	if o.Ended != changes.Refused || o.Decider != "sam" || o.Note != "not the fixer's job" || !strings.Contains(o.Text(), "not the fixer's job") {
		t.Fatalf("the change ended %+v", o)
	}
	if text, err := os.ReadFile(filepath.Join(b.dir, "garden", "team.yaml")); err != nil || string(text) != house(t)["garden/team.yaml"] {
		t.Fatalf("the refused change was written: %s, %v", text, err)
	}
	e := b.entries(n, decisions.KindDecided)[0]
	if e.To[0].Name != "builder" || !strings.Contains(e.Text, "not the fixer's job") {
		t.Fatalf("the builder is told %+v", e)
	}
}

func TestChangeWalkedOverItsList(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  change: [./no-secrets.js, sam]\n", 1)
	files["no-secrets.js"] = `return { says: "yes" }`
	b := newBench(t, files)
	n := b.chat("kit", office.Top, "let me into the garden")
	dec := b.draft(n, "Name Kit in the garden", kitInTheGarden(t))
	r := b.runner.next(t)
	if r.Name != "no-secrets.js" || r.Input["kind"] != decisions.OfChange || !strings.Contains(r.Input["ends"].(string), "your yes is taken as a pass") {
		t.Fatalf("the change is put to %s, given %v", r.Name, r.Input)
	}
	r.says(decisions.SaysYes, "no secret in it")
	b.until("Sam is asked", func() bool { return len(b.waitingFor("sam")) == 1 })
	if got := b.get(dec.ID); got.Ended != "" || !strings.Contains(got.Passes[0].Note, "only a person's yes approves a change") {
		t.Fatalf("a rule's yes on a change ended it: %+v", got)
	}
	b.decide(dec.ID, "sam", decisions.SaysYes, "")
	if o := b.outcome(dec.Put.ID); o.Ended != changes.Written {
		t.Fatalf("the change ended %+v", o)
	}

	other := b.chat("kit", office.Top, "and the shed")
	dec = b.draft(other, "Name Kit in the garden again", map[string]string{"garden/notes/shed.md": "# The shed\n\nKit has a key.\n"})
	b.runner.next(t).says(decisions.SaysRefuse, "a key is a secret")
	if o := b.outcome(dec.Put.ID); o.Ended != changes.Refused || o.Decider != "no-secrets.js" {
		t.Fatalf("the change ended %+v", o)
	}
}

func TestPersonsOwnChangeDecidedByTheirYes(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  change: [./no-secrets.js, sam]\n", 1)
	files["no-secrets.js"] = `return { says: "pass" }`
	b := newBench(t, files)
	p := b.putOwn("sam", "Kit helps in the garden")
	b.runner.next(t).says(decisions.SaysRefuse, "not today")
	if o := b.outcome(p.ID); o.Ended != changes.Refused || o.Decider != "no-secrets.js" {
		t.Fatalf("Sam's own change ended %+v", o)
	}
	p = b.putOwn("sam", "Kit helps in the garden, again")
	b.runner.next(t).says(decisions.SaysPass, "")
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

// A change waits on Sam, the one person on its walk, and Sam then leaves
// the office: nobody named on the walk it was put with can decide it.
// Sam merely taken off the change list would still be asked, since a
// change's walk is the one it was put with.
func TestChangeRefusedByBulldog(t *testing.T) {
	t.Parallel()
	files := house(t)
	b := newBench(t, files)
	n := b.chat("kit", office.Top, "tidy the shed")
	dec := b.draft(n, "Note the shed", map[string]string{"notes/shed.md": "# The shed\n"})

	o := files["office.yaml"]
	o = strings.Replace(o, "  sam: { name: Sam, title: Owner. Decides money and the building., telegram: \"4471\" }\n", "", 1)
	o = strings.Replace(o, "people: [sam, kit]", "people: [kit]", 1)
	o = strings.Replace(o, "everything: [sam]", "everything: [kit]", 1)
	o = strings.Replace(o, "order_part: [./small-orders.js, sam]", "order_part: [./small-orders.js, kit]", 1)
	testkit.Write(t, b.dir, map[string]string{"office.yaml": o, "garden/team.yaml": strings.ReplaceAll(files["garden/team.yaml"], "sam", "kit")})
	read, err := office.Read(b.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.reqs.Read(read); err != nil {
		t.Fatal(err)
	}
	told := b.d.Read(ctx, read)

	got := b.get(dec.ID)
	const why = "nobody can decide this: nobody named on its walk is in the office"
	if got.Ended != decisions.Refused || got.By.Kind != requests.IsBulldog || got.Why != why {
		t.Fatalf("the change nobody can decide is %+v", got)
	}
	e := b.entries(n, decisions.KindDecided)[0]
	if len(e.To) != 1 || e.To[0].Name != "builder" || e.Text != "refused by Bulldog: "+why {
		t.Fatalf("the builder is told %+v", e)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "notes", "shed.md")); !os.IsNotExist(err) {
		t.Fatalf("the refused change was written: %v", err)
	}

	if told != nil {
		t.Fatal(told)
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
