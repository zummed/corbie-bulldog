package changes

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/office"
)

// clerkAgain is the clerk with a line more, a change that reads well.
func clerkAgain(t *testing.T, line string) string { return house(t)["clerk.yaml"] + "# " + line + "\n" }

func TestBuilderIsGivenTheOutcome(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	// Written, with the commit.
	f.write(1, "clerk.yaml", clerkAgain(t, "one"))
	p := f.put(1, "One")
	o := f.yes(p.ID, "sam", "")
	if o.Ended != Written || o.Commit != f.head().Hash.String() || !strings.Contains(o.Text(), "written, with the yes of Sam") || !strings.Contains(o.Text(), o.Commit) {
		t.Errorf("a yes ended it %+v: %s", o, o.Text())
	}
	// Refused, with who and the note.
	f.write(2, "clerk.yaml", clerkAgain(t, "two"))
	p = f.put(2, "Two")
	o, err := f.c.Decided(ctx, p.ID, Decision{By: "sam", Kind: office.IsPerson, Called: f.called("sam"), Note: "not the clerk's job"})
	if err != nil || o.Ended != Refused || o.Decider != "sam" || !strings.Contains(o.Text(), "refused by Sam") || !strings.Contains(o.Text(), "not the clerk's job") {
		t.Errorf("a refusal ended it %+v, %v: %s", o, err, o.Text())
	}
	// Not written, with the file that changed: the folder is changed by
	// hand and committed before the yes.
	f.write(3, "steward.yaml", house(t)["steward.yaml"]+"# three\n")
	p = f.put(3, "Three")
	f.edit(map[string]string{"steward.yaml": house(t)["steward.yaml"] + "# by hand\n"})
	f.commitByHand("By hand")
	o = f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Text(), "steward.yaml") {
		t.Errorf("a stale change ended %+v: %s", o, o.Text())
	}
	// Withdrawn: a later run on the same request changes the draft and
	// puts it again.
	f.write(4, "clerk.yaml", clerkAgain(t, "four"))
	first := f.put(4, "Four")
	f.write(4, "clerk.yaml", clerkAgain(t, "four, again"))
	second := f.put(4, "Four again")
	_, ended, _ := f.c.Put(ctx, first.ID)
	if ended.Ended != Withdrawn || !slices.ContainsFunc(f.outcomes(), func(o Outcome) bool { return o.Put == first.ID && o.Ended == Withdrawn && o.Request == 4 }) {
		t.Errorf("the change put first ended %+v", ended)
	}
	if _, ended, _ := f.c.Put(ctx, second.ID); ended.Ended != "" {
		t.Errorf("the change put again ended %+v", ended)
	}
}

func TestChangesPutAsADecisionOfTheTopTeam(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		decide string
		walk   []string
	}{
		{"decide: { change: [./small-orders.js, sam] }", []string{"small-orders.js", "sam"}},
		{"decide: { everything: [kit] }", []string{"kit"}},
		{"", []string{"sam", "kit"}},
	} {
		files := house(t)
		files["office.yaml"] = strings.Replace(files["office.yaml"], "decide:\n  everything: [sam]\n  order_part: [./small-orders.js, clerk: advise, sam]\n", c.decide+"\n", 1)
		f := newOffice(t, files)
		f.write(1, "clerk.yaml", clerkAgain(t, "x"))
		p := f.put(1, "X")
		var walk []string
		for _, d := range p.Walk {
			walk = append(walk, d.Name)
		}
		if !slices.Equal(walk, c.walk) {
			t.Errorf("with %q the change is asked of %v", c.decide, walk)
		}
		if got := f.decided(); len(got) != 1 || got[0].ID != p.ID {
			t.Errorf("with %q it was put %+v", c.decide, got)
		}
	}
}

func TestChangesWalkGoesOnToEverything(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["no-secrets.js"] = "return { says: \"yes\", note: \"no secrets\" }\n"
	files["reviewer.yaml"] = "agent: reviewer\ninput: { said: string }\nmodel: usual\nsystem: You review.\nprompt: ${input.said}\n"
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [reviewer, sam]\n  change: [./no-secrets.js]\n", 1)
	f := newOffice(t, files)
	var names []string
	for _, d := range Walk(f.keeper.Office()) {
		names = append(names, d.Name)
	}
	if !slices.Equal(names, []string{"no-secrets.js", "reviewer", "sam"}) {
		t.Errorf("the walk is %v", names)
	}
	// With a person on the change list, the walk ends with them.
	files["office.yaml"] = strings.Replace(files["office.yaml"], "change: [./no-secrets.js]", "change: [./no-secrets.js, sam]", 1)
	f = newOffice(t, files)
	names = nil
	for _, d := range Walk(f.keeper.Office()) {
		names = append(names, d.Name)
	}
	if !slices.Equal(names, []string{"no-secrets.js", "sam"}) {
		t.Errorf("the walk is %v", names)
	}
}

func TestChangesWhoDecidesAChange(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		decide string
		who    []string
	}{
		{"decide: { everything: [kit], change: [./small-orders.js, sam] }", []string{"sam"}},
		{"decide: { everything: [kit], change: [./small-orders.js] }", []string{"kit"}},
		{"", []string{"sam", "kit"}},
	} {
		files := house(t)
		files["office.yaml"] = strings.Replace(files["office.yaml"], "decide:\n  everything: [sam]\n  order_part: [./small-orders.js, clerk: advise, sam]\n", c.decide+"\n", 1)
		o, err := office.Read(newOffice(t, files).dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := Deciders(o); !slices.Equal(got, c.who) {
			t.Errorf("with %q the people who decide a change are %v", c.decide, got)
		}
	}
}

func TestChangesDeciderIsShownTheChange(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(12, "clerk.yaml", clerkAgain(t, "changed"))
	f.write(12, "notes/fuses.md", "Breaker 4.\n")
	p := f.put(12, "Note the fuses\n\nThe kitchen breaker keeps tripping.")
	shown := f.decided()[0]
	if shown.Why != "Note the fuses\n\nThe kitchen breaker keeps tripping." || shown.Request != 12 || shown.By != "builder" || shown.ID != p.ID {
		t.Errorf("the decider is shown %+v", shown)
	}
	has(t, shown.Says, "note fuses is new")
	var clerk, fuses File
	for _, file := range shown.Files {
		switch file.Path {
		case "clerk.yaml":
			clerk = file
		case "notes/fuses.md":
			fuses = file
		}
	}
	if clerk.Was != house(t)["clerk.yaml"] || clerk.Text != clerkAgain(t, "changed") || fuses.Text != "Breaker 4.\n" || fuses.Had {
		t.Errorf("the files shown are %+v", shown.Files)
	}
}

func TestChangesRefusalWritesNothing(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	before := f.head().Hash
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(1, "X")
	o, err := f.c.Decided(ctx, p.ID, Decision{By: "sam", Kind: office.IsPerson, Note: "no"})
	if err != nil || o.Ended != Refused {
		t.Fatal(o, err)
	}
	if f.head().Hash != before || f.text("clerk.yaml") != house(t)["clerk.yaml"] {
		t.Error("a refusal wrote something")
	}
	if got := f.call(1, ToolFiles, nil).Text; !strings.Contains(got, "clerk.yaml: changed") {
		t.Errorf("after a refusal the draft is:\n%s", got)
	}
}

func TestChangesBulldogRefusesWhatNobodyCanDecide(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(1, "X")
	const why = "nobody can decide this: nobody named on its walk is in the office"
	if _, err := f.c.Decided(ctx, p.ID, Decision{By: "kit", Kind: office.IsPerson, Note: "no"}); err == nil || !strings.Contains(err.Error(), "not asked") {
		t.Errorf("Kit, not on the walk, refused it: %v", err)
	}
	if _, err := f.c.Decided(ctx, p.ID, Decision{By: "Bulldog", Kind: office.IsBulldog, Yes: true, Note: why}); err == nil {
		t.Error("Bulldog's yes was taken")
	}
	if _, ended, _ := f.c.Put(ctx, p.ID); ended.Ended != "" || f.text("clerk.yaml") != house(t)["clerk.yaml"] {
		t.Fatalf("Bulldog's yes ended it %+v", ended)
	}
	o, err := f.c.Decided(ctx, p.ID, Decision{By: "Bulldog", Kind: office.IsBulldog, Note: why})
	if err != nil || o.Ended != Refused || o.Decider != "Bulldog" || o.Note != why {
		t.Fatalf("Bulldog's refusal ended it %+v, %v", o, err)
	}
	if text := o.Text(); text != "The change was refused by Bulldog, and nothing was written. Bulldog's note: "+why+" The draft is kept, to be changed and put again." {
		t.Errorf("the builder is told %q", text)
	}
	if told := f.outcomes(); len(told) != 1 || told[0].Request != 1 || told[0].Ended != Refused || told[0].Decider != "Bulldog" {
		t.Errorf("the request is told %+v", told)
	}
	if f.text("clerk.yaml") != house(t)["clerk.yaml"] || !strings.Contains(f.call(1, ToolFiles, nil).Text, "clerk.yaml: changed") {
		t.Error("Bulldog's refusal wrote something, or lost the draft")
	}
}

func TestChangesLateDecisionIsActedOn(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(5, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(5, "X")
	// Seven days on, the tool has stopped waiting, and the change still
	// waits: Sam's yes on the eighth day writes it, and the request is told.
	f.clock.Move(8 * 24 * time.Hour)
	o := f.yes(p.ID, "sam", "")
	if o.Ended != Written || f.text("clerk.yaml") != clerkAgain(t, "x") {
		t.Fatalf("the late yes ended it %+v", o)
	}
	told := f.outcomes()
	if len(told) != 1 || told[0].Request != 5 || told[0].By != "builder" || told[0].Commit == "" {
		t.Errorf("the request is told %+v", told)
	}
	if !f.history()[0].At.Equal(f.clock.Now()) {
		t.Errorf("the commit is dated %v", f.history()[0].At)
	}
}

func TestChangesOnlyAPersonsYesWritesAChange(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  change: [./small-orders.js]\n", 1)
	f := newOffice(t, files)
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(1, "X")
	if _, err := f.c.Decided(ctx, p.ID, Decision{By: "small-orders.js", Kind: office.IsRule, Yes: true, Note: "fine"}); err == nil {
		t.Error("a rule's yes was taken")
	}
	if _, ended, _ := f.c.Put(ctx, p.ID); ended.Ended != "" || f.text("clerk.yaml") != house(t)["clerk.yaml"] {
		t.Errorf("a rule's yes ended it %+v", ended)
	}
	// A rule may refuse one.
	o, err := f.c.Decided(ctx, p.ID, Decision{By: "small-orders.js", Kind: office.IsRule, Note: "secrets"})
	if err != nil || o.Ended != Refused || o.Decider != "small-orders.js" {
		t.Errorf("a rule's refusal ended it %+v, %v", o, err)
	}
}

func TestChangesJudgedByTheOfficeAsItIs(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "  everything: [sam]\n", "  everything: [sam]\n  change: [./small-orders.js, sam]\n", 1)
	f := newOffice(t, files)
	// Sam's own change takes the rule off the change list: it is still
	// asked of the rule, as the office is now.
	edited := strings.Replace(files["office.yaml"], "  change: [./small-orders.js, sam]\n", "  change: [sam]\n", 1)
	changed, _, err := f.c.Edited(Edit{Files: map[string]*string{"office.yaml": &edited}})
	if err != nil {
		t.Fatal(err)
	}
	check, err := f.c.Check(ctx, changed, Drafter{Name: "sam", Person: true})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.c.PutOwn(ctx, "sam", "Take the rule off", changed, check.Mark)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Walk) != 1 || p.Walk[0].Name != "small-orders.js" {
		t.Errorf("it is asked of %+v", p.Walk)
	}
}

func TestChangesAYesWritesWhatWasShown(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "shown"))
	p := f.put(1, "X")
	// The draft changes while the change waits.
	f.write(1, "clerk.yaml", clerkAgain(t, "not shown"))
	if o := f.yes(p.ID, "sam", ""); o.Ended != Written {
		t.Fatal(o)
	}
	if got := f.text("clerk.yaml"); got != clerkAgain(t, "shown") {
		t.Errorf("the yes wrote %q", got)
	}
	// What was drafted after it is kept, drafted now against what was
	// written, and may be put in its turn.
	if draft, _ := f.c.Drafted(ctx, 1); len(draft) != 1 || draft[0].Text != clerkAgain(t, "not shown") || draft[0].Was != clerkAgain(t, "shown") {
		t.Errorf("the draft is %+v", draft)
	}
	if check := f.check(1); !check.Fit() {
		t.Errorf("what was drafted after is checked %v", check.Refused)
	}
}

func TestChangesAnUnfitChangeIsNotPut(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	try := func(request int64, why, says string) {
		t.Helper()
		if !strings.Contains(f.call(request, ToolCheck, nil).Text, says) && strings.TrimSpace(why) != "" {
			t.Errorf("the check does not say %q:\n%s", says, f.call(request, ToolCheck, nil).Text)
		}
		a := f.call(request, ToolPut, map[string]any{"why": why})
		if a.Put != 0 || !strings.Contains(a.Text, says) {
			t.Errorf("put: %+v", a)
		}
	}
	try(1, "Nothing", "empty")
	f.write(2, "clerk.yaml", clerkAgain(t, "x"))
	try(2, "  ", "no why")
	f.write(3, "garden/team.yaml", strings.Replace(house(t)["garden/team.yaml"], "people: [sam]", "people: [jo]", 1))
	try(3, "Jo", "jo is nobody")
	f.write(4, "clerk.yaml", clerkAgain(t, "y"))
	f.edit(map[string]string{"clerk.yaml": clerkAgain(t, "by hand")})
	try(4, "Y", "edit by hand")
	if len(f.decided()) != 0 {
		t.Errorf("an unfit change was put: %+v", f.decided())
	}
}

func TestChangesWrittenAndReadAgain(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "office.yaml", strings.Replace(house(t)["office.yaml"], "lead: steward", "lead: clerk", 1))
	p := f.put(1, "The clerk is the lead")
	o := f.yes(p.ID, "sam", "")
	if o.Ended != Written {
		t.Fatal(o)
	}
	if draft, _ := f.c.Drafted(ctx, 1); len(draft) != 0 {
		t.Errorf("the draft holds %+v", draft)
	}
	if h := f.history()[0]; h.ID != o.Commit || !slices.Equal(h.Files, []string{"office.yaml"}) {
		t.Errorf("the newest commit is %+v", h)
	}
	if got := f.keeper.Office().TopTeam().Lead; got != "clerk" {
		t.Errorf("the office read again has the lead %s", got)
	}
}

func TestChangesWorkersTakenAwayAreNamed(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.call(1, ToolRemove, map[string]any{"path": "clerk.yaml"})
	f.write(1, "crew.yaml", strings.Replace(house(t)["crew.yaml"], "clerk:\n  does: Money and orders. What was spent, what is owed and what is on its way\n  args: { question: string }\n  job: { machine: ./clerk.yaml, input: { said: \"${args.question}\" } }\n", "", 1))
	f.write(1, "office.yaml", strings.Replace(house(t)["office.yaml"], "clerk: advise, ", "", 1))
	p := f.put(1, "No clerk")
	if o := f.yes(p.ID, "sam", ""); o.Ended != Written || !slices.Equal(o.Gone, []string{"clerk"}) {
		t.Errorf("the change ended %+v", o)
	}
}

func TestChangesCheckedAgainWhenWritten(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "garden/team.yaml", strings.Replace(house(t)["garden/team.yaml"], "people: [sam]", "people: [sam, kit]", 1))
	p := f.put(1, "Kit in the garden")
	// Kit is taken out of the office by hand, and committed.
	f.edit(map[string]string{"office.yaml": strings.Replace(strings.Replace(house(t)["office.yaml"], "  kit: { name: Kit, title: Lives here., telegram: \"4472\" }\n", "", 1), "people: [sam, kit]", "people: [sam]", 1)})
	f.commitByHand("Kit leaves")
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Why, "kit is nobody") {
		t.Errorf("it ended %+v", o)
	}
	if strings.Contains(f.text("garden/team.yaml"), "kit") {
		t.Error("it was written")
	}
}

func TestChangesAStaleFileIsRefused(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.write(1, "notes/fuses.md", "Breaker 4.\n")
	p := f.put(1, "X")
	f.edit(map[string]string{"clerk.yaml": clerkAgain(t, "later")})
	f.commitByHand("Later")
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Why, "clerk.yaml") {
		t.Errorf("it ended %+v", o)
	}
	if f.text("notes/fuses.md") != "" || f.text("clerk.yaml") != clerkAgain(t, "later") {
		t.Error("some of the change was written")
	}
	// Drafted anew, against the folder as it is now, it may be put.
	if check := f.check(1); check.Fit() || !strings.Contains(strings.Join(check.Refused, "\n"), "draft it anew") {
		t.Errorf("the stale draft is checked %v", check.Refused)
	}
	if said := f.call(1, ToolWrite, map[string]any{"path": "clerk.yaml", "text": clerkAgain(t, "later") + "# x\n"}).Text; !strings.Contains(said, "drafted anew") {
		t.Errorf("the builder is told %s", said)
	}
	if check := f.check(1); !check.Fit() {
		t.Errorf("the draft made anew is checked %v", check.Refused)
	}
}

func TestChangesAHandEditIsNeverFoldedIn(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(1, "X")
	// After the change was put, the clerk is changed by hand and committed,
	// and then changed back by hand: it is as it was drafted against, and
	// differs from the last commit by an edit by hand.
	f.edit(map[string]string{"clerk.yaml": clerkAgain(t, "by hand")})
	f.commitByHand("By hand")
	f.edit(map[string]string{"clerk.yaml": house(t)["clerk.yaml"]})
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Why, "clerk.yaml") || !strings.Contains(o.Why, "commit it, or undo it") || strings.Contains(o.Why, "no longer as it was") {
		t.Errorf("it ended %+v", o)
	}
	if f.head().Author.Name == "builder" {
		t.Error("the hand edit was committed as the builder's")
	}
}

func TestChangesAllFilesOrNone(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a folder that may not be written is written all the same by root")
	}
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.call(1, ToolRemove, map[string]any{"path": "bins.yaml"})
	f.write(1, "office.yaml", strings.Replace(house(t)["office.yaml"], "  - { at: \"18:00\", to: bins, say: \"Which bin goes out tonight?\" }\n", "", 1))
	f.write(1, "garden/notes/lawn.md", "Mown on Sundays.\n")
	p := f.put(1, "X")
	// The garden's notes may not be written: lawn.md, the third of the four
	// files, fails, after two are written.
	notes := filepath.Join(f.dir, "garden", "notes")
	if err := os.Chmod(notes, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(notes, 0o755) })
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Why, "garden/notes/lawn.md") {
		t.Errorf("it ended %+v", o)
	}
	for file, text := range map[string]string{"clerk.yaml": house(t)["clerk.yaml"], "bins.yaml": house(t)["bins.yaml"], "office.yaml": house(t)["office.yaml"], "garden/notes/lawn.md": house(t)["garden/notes/lawn.md"]} {
		if f.text(file) != text {
			t.Errorf("%s was not put back", file)
		}
	}
	if byHand, _ := f.c.ByHand(); len(byHand) != 0 {
		t.Errorf("the folder is left as %v", byHand)
	}
}

func TestChangesAFileIsNeverHalfWritten(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	big := strings.Repeat("# a line of the clerk\n", 20000)
	f.write(1, "clerk.yaml", house(t)["clerk.yaml"]+big)
	p := f.put(1, "A big clerk")
	done := make(chan struct{})
	seen := make(chan string, 1)
	go func() {
		defer close(seen)
		for {
			select {
			case <-done:
				return
			default:
			}
			text := f.text("clerk.yaml")
			if text != house(t)["clerk.yaml"] && text != house(t)["clerk.yaml"]+big {
				seen <- text
				return
			}
		}
	}()
	o := f.yes(p.ID, "sam", "")
	close(done)
	if half, ok := <-seen; ok {
		t.Errorf("a reader saw %d bytes of the clerk", len(half))
	}
	if o.Ended != Written {
		t.Fatal(o)
	}
	entries, _ := os.ReadDir(f.dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".new") {
			t.Errorf("%s is left beside the clerk", e.Name())
		}
	}
}

func TestChangesAFailedCommitPutsTheFilesBack(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a folder that may not be written is written all the same by root")
	}
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	f.write(1, "notes/fuses.md", "Breaker 4.\n")
	p := f.put(1, "X")
	// The repository may not be written.
	objects := filepath.Join(f.dir, ".git", "objects")
	var dirs []string
	filepath.WalkDir(objects, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	for _, d := range dirs {
		os.Chmod(d, 0o555)
	}
	t.Cleanup(func() {
		for _, d := range dirs {
			os.Chmod(d, 0o755)
		}
	})
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || !strings.Contains(o.Why, "committed") {
		t.Errorf("it ended %+v", o)
	}
	if f.text("clerk.yaml") != house(t)["clerk.yaml"] || f.text("notes/fuses.md") != "" {
		t.Error("the files were not put back")
	}
}

func TestChangesAnUnwrittenYesEndsTheDecision(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(1, "X")
	f.edit(map[string]string{"clerk.yaml": clerkAgain(t, "by hand")})
	o := f.yes(p.ID, "sam", "")
	if o.Ended != NotWritten || o.Why == "" || o.Decider != "sam" {
		t.Fatalf("it ended %+v", o)
	}
	if _, ended, _ := f.c.Put(ctx, p.ID); ended.Ended != NotWritten {
		t.Errorf("the change is kept as %+v", ended)
	}
	if told := f.outcomes(); len(told) != 1 || told[0].Ended != NotWritten || told[0].Request != 1 {
		t.Errorf("it was told %+v", told)
	}
	if draft, _ := f.c.Drafted(ctx, 1); len(draft) != 1 {
		t.Errorf("the draft is %+v", draft)
	}
	if _, err := f.c.Decided(ctx, p.ID, Decision{By: "sam", Kind: office.IsPerson, Yes: true}); err == nil || !strings.Contains(err.Error(), "ended already") {
		t.Errorf("a second yes: %v", err)
	}
}

func TestChangesWrittenOnceCommitted(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a file that may not be written is written all the same by root")
	}
	// The repository's index cannot be written: the commit is made all the
	// same, and the change is written, with the error beside it.
	f := newOffice(t, house(t))
	f.write(1, "clerk.yaml", clerkAgain(t, "x"))
	p := f.put(1, "X")
	index := filepath.Join(f.dir, ".git", "index")
	if err := os.Chmod(index, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(index, 0o644) })
	o, err := f.c.Decided(ctx, p.ID, Decision{By: "sam", Kind: office.IsPerson, Yes: true})
	if o.Ended != Written || o.Commit != f.head().Hash.String() || err == nil || !strings.Contains(err.Error(), "index") {
		t.Errorf("it ended %+v, %v", o, err)
	}
	if f.text("clerk.yaml") != clerkAgain(t, "x") {
		t.Error("the written change was put back")
	}
	if draft, _ := f.c.Drafted(ctx, 1); len(draft) != 0 {
		t.Errorf("the draft holds %+v", draft)
	}
	// The draft cannot be cleared: the change is written all the same, its
	// workers taken away named, and the office read again.
	g := newOffice(t, house(t))
	g.call(1, ToolRemove, map[string]any{"path": "clerk.yaml"})
	g.write(1, "crew.yaml", strings.Replace(house(t)["crew.yaml"], "clerk:\n  does: Money and orders. What was spent, what is owed and what is on its way\n  args: { question: string }\n  job: { machine: ./clerk.yaml, input: { said: \"${args.question}\" } }\n", "", 1))
	g.write(1, "office.yaml", strings.Replace(house(t)["office.yaml"], "clerk: advise, ", "", 1))
	p = g.put(1, "No clerk")
	if _, err := g.db.Exec(`DROP TABLE changes_drafts`); err != nil {
		t.Fatal(err)
	}
	o, err = g.c.Decided(ctx, p.ID, Decision{By: "sam", Kind: office.IsPerson, Yes: true})
	if o.Ended != Written || !slices.Equal(o.Gone, []string{"clerk"}) || err == nil || !strings.Contains(err.Error(), "draft could not be cleared") {
		t.Errorf("it ended %+v, %v", o, err)
	}
	if g.keeper.Office().Workers["clerk"] != nil {
		t.Error("the office was not read again")
	}
	if told := g.outcomes(); len(told) != 1 || told[0].Ended != Written {
		t.Errorf("it was told %+v", told)
	}
}
