package changes

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
)

// own checks and puts a person's own edit, as the screens do.
func (f *officeAt) own(person, why string, e Edit) (Put, error) {
	f.t.Helper()
	files, _, err := f.c.Edited(e)
	if err != nil {
		return Put{}, err
	}
	check, err := f.c.Check(ctx, files, Drafter{Name: person, Person: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return f.c.PutOwn(ctx, person, why, files, check.Mark)
}

func TestChangesAPersonDraftsOnTheScreens(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	files, told, err := f.c.Edited(Edit{Set: []Setting{{Path: "clerk.yaml", Field: []string{"system"}, Value: "You keep the house's accounts, and the receipts."}}})
	if err != nil || len(told) != 0 || len(files) != 1 {
		t.Fatalf("the edit gives %+v, %v, %v", files, told, err)
	}
	if want := replaced(t, house(t), "clerk.yaml", "system: You keep the house's accounts. Say what was spent, what is owed and what is on its way, and, asked about an order, whether the house has the money for it.", "system: You keep the house's accounts, and the receipts."); files[0].Text != want {
		t.Errorf("the clerk would be\n%s", files[0].Text)
	}
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	if !check.Fit() || len(check.Says) != 1 || check.Says[0] != "clerk.yaml is changed." {
		t.Errorf("the check is %+v", check)
	}
	files, _, _ = f.c.Edited(Edit{Set: []Setting{{Path: "clerk.yaml", Field: []string{"turns"}, Value: "many"}}})
	check, _ = f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	has(t, problemLines(check.Problems), "clerk.yaml", "turns")
}

func TestChangesAPersonsDraftIsNotKept(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	before := f.head().Hash
	text := clerkAgain(t, "x")
	files, _, _ := f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	var drafts, puts int
	f.db.QueryRow(`SELECT count(*) FROM changes_drafts`).Scan(&drafts)
	f.db.QueryRow(`SELECT count(*) FROM changes_puts`).Scan(&puts)
	if drafts != 0 || puts != 0 || f.head().Hash != before || f.text("clerk.yaml") != house(t)["clerk.yaml"] {
		t.Errorf("a person's draft left %d drafts and %d puts", drafts, puts)
	}
}

// shared gives the house with the clerk using the fixer's order script
// too, so that the fixer uses one script alone and shares another.
func shared(t *testing.T) map[string]string {
	files := house(t)
	files["clerk.yaml"] += "tools:\n  order: { does: Order, args: { part: string }, job: { script: ./tools/order.js, input: { part: \"${args.part}\", pounds: 1 } } }\n"
	return files
}

func TestChangesAMovedWorkerTakesWhatIsItsAlone(t *testing.T) {
	t.Parallel()
	f := newOffice(t, shared(t))
	files, _, err := f.c.Edited(Edit{Move: map[string]string{"fixer": "garden"}})
	if err != nil {
		t.Fatal(err)
	}
	does := map[string]string{}
	var moved string
	for _, file := range files {
		does[file.Path] = file.Does()
		if file.Path == "garden/fixer.yaml" {
			moved = file.Text
		}
	}
	want := map[string]string{"fixer.yaml": IsGone, "garden/fixer.yaml": IsNew, "tools/cut-power.js": IsGone, "garden/tools/cut-power.js": IsNew}
	for file, d := range want {
		if does[file] != d {
			t.Errorf("%s is %q in the change %v", file, does[file], does)
		}
	}
	if _, ok := does["tools/order.js"]; ok {
		t.Error("the script the clerk shares moved")
	}
	for _, named := range []string{"script: ../tools/order.js", "script: ./tools/cut-power.js", "machine: ../sparks.yaml", "tools_file: [../.bulldog/tools/talk/talk.yaml]"} {
		if !strings.Contains(moved, named) {
			t.Errorf("the moved fixer does not name %s:\n%s", named, moved)
		}
	}
	// The fixer's own script is shown with the change.
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	if !slices.ContainsFunc(check.Files, func(file File) bool { return file.Path == "garden/tools/cut-power.js" }) {
		t.Errorf("the change shows %+v", check.Files)
	}
	// A builder that moves the fixer by writing its definition whole in the
	// garden: its own script is added to the draft and goes with it.
	f.call(1, ToolRemove, map[string]any{"path": "fixer.yaml"})
	f.write(1, "garden/fixer.yaml", moved)
	check = f.check(1)
	does = map[string]string{}
	for _, file := range check.Files {
		does[file.Path] = file.Does()
	}
	for file, d := range want {
		if does[file] != d {
			t.Errorf("in the builder's move %s is %q, in %v", file, does[file], does)
		}
	}
	if len(check.Taken) != 1 || !strings.Contains(check.Text(), "tools/cut-power.js goes with fixer, to garden/tools/cut-power.js") {
		t.Errorf("the builder's move is checked:\n%s", check.Text())
	}
	f.call(1, ToolCheck, nil)
	if listed := f.call(1, ToolFiles, nil).Text; !strings.Contains(listed, "garden/tools/cut-power.js: new") || !strings.Contains(listed, "tools/cut-power.js: gone") {
		t.Errorf("the builder's draft is:\n%s", listed)
	}
	// A builder whose moved definition names the script where it is leaves
	// it there.
	g := newOffice(t, shared(t))
	g.call(1, ToolRemove, map[string]any{"path": "fixer.yaml"})
	g.write(1, "garden/fixer.yaml", strings.Replace(moved, "script: ./tools/cut-power.js", "script: ../tools/cut-power.js", 1))
	if check := g.check(1); len(check.Taken) != 0 || len(check.Files) != 2 {
		t.Errorf("the builder's move naming the script where it is is checked %+v", check)
	}
	// What a builder's move takes is put, written and cleared from its
	// draft with it.
	office := house(t)
	office["porter.yaml"] = "agent: porter\ninput: { said: string }\nmodel: usual\nsystem: You carry.\nprompt: ${input.said}\ntools:\n  carry: { does: Carry, args: {}, job: { script: ./tools/carry.js } }\n"
	office["tools/carry.js"] = "return 1\n"
	b := newOffice(t, office)
	b.call(1, ToolRemove, map[string]any{"path": "porter.yaml"})
	b.write(1, "garden/porter.yaml", office["porter.yaml"])
	p := b.put(1, "The porter works in the garden")
	if !slices.ContainsFunc(p.Files, func(file File) bool { return file.Path == "garden/tools/carry.js" && !file.Had }) || !slices.ContainsFunc(p.Files, func(file File) bool { return file.Path == "tools/carry.js" && file.Gone }) {
		t.Errorf("the move is put with %+v", p.Files)
	}
	if o := b.yes(p.ID, "sam", ""); o.Ended != Written || b.text("tools/carry.js") != "" || b.text("garden/tools/carry.js") != "return 1\n" {
		t.Errorf("the builder's move ended %+v", o)
	}
	if draft, _ := b.c.Drafted(ctx, 1); len(draft) != 0 {
		t.Errorf("the draft holds %+v", draft)
	}
	// A worker of the top team takes only what is in the top team's own
	// folder: a script it alone uses in the garden's stays where it is.
	office = house(t)
	office["porter.yaml"] = "agent: porter\ninput: { said: string }\nmodel: usual\nsystem: You carry.\nprompt: ${input.said}\ntools:\n  carry: { does: Carry, args: {}, job: { script: ./garden/tools/carry.js } }\n"
	office["garden/tools/carry.js"] = "return 1\n"
	h := newOffice(t, office)
	changed, _, err := h.c.Edited(Edit{Move: map[string]string{"porter": "garden"}})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, file := range changed {
		paths = append(paths, file.Path)
		if file.Path == "garden/porter.yaml" && !strings.Contains(file.Text, "script: ./tools/carry.js") {
			t.Errorf("the moved porter is\n%s", file.Text)
		}
	}
	if !slices.Equal(paths, []string{"garden/porter.yaml", "porter.yaml"}) {
		t.Errorf("moving the porter changes %v", paths)
	}
}

func TestChangesAMoveNeverWritesOverAFile(t *testing.T) {
	t.Parallel()
	files := shared(t)
	files["garden/tools/cut-power.js"] = "// the garden's own\n"
	f := newOffice(t, files)
	if _, _, err := f.c.Edited(Edit{Move: map[string]string{"fixer": "garden"}}); err == nil || !strings.Contains(err.Error(), "garden/tools/cut-power.js") {
		t.Errorf("the move: %v", err)
	}
	// Nor is a builder's: its fixer in the garden would read the garden's
	// script, and leave its own for nothing to read.
	moved := strings.NewReplacer("./tools/cut-power.js", "./tools/cut-power.js", "./tools/order.js", "../tools/order.js", "./sparks.yaml", "../sparks.yaml", "./.bulldog/", "../.bulldog/").Replace(files["fixer.yaml"])
	f.call(1, ToolRemove, map[string]any{"path": "fixer.yaml"})
	f.write(1, "garden/fixer.yaml", moved)
	if check := f.check(1); check.Fit() || !slices.ContainsFunc(check.Refused, func(r string) bool {
		return strings.Contains(r, "garden/tools/cut-power.js is there already") && strings.Contains(r, "tools/cut-power.js, which only fixer used")
	}) {
		t.Errorf("the builder's move is checked %v", check.Refused)
	}
}

func TestChangesAWorkerGoesWithWhatItAloneUses(t *testing.T) {
	t.Parallel()
	f := newOffice(t, shared(t))
	files, _, err := f.c.Edited(Edit{Remove: []string{"fixer"}})
	if err != nil {
		t.Fatal(err)
	}
	var gone []string
	for _, file := range files {
		if file.Gone {
			gone = append(gone, file.Path)
		}
	}
	if !slices.Equal(gone, []string{"fixer.yaml", "tools/cut-power.js"}) {
		t.Errorf("taking the fixer away removes %v", gone)
	}
	// The bins flow runs one script, its alone.
	files, _, _ = f.c.Edited(Edit{Remove: []string{"bins"}})
	gone = nil
	for _, file := range files {
		gone = append(gone, file.Path)
	}
	if !slices.Equal(gone, []string{"bins.yaml", "tools/bins.js"}) {
		t.Errorf("taking bins away removes %v", gone)
	}
	// A script a bridge's machine uses too is never taken, whether the
	// worker goes or moves.
	office := house(t)
	office["bridges/telegram/out.yaml"] = replaced(t, office, "bridges/telegram/out.yaml", "script: ./telegram.js", "script: ./telegram.js\n      files: [../../tools/circuits.js]")
	g := newOffice(t, office)
	if len(g.keeper.Problems()) != 0 {
		t.Fatal(g.keeper.Problems())
	}
	for _, e := range []Edit{{Remove: []string{"sparks"}}, {Move: map[string]string{"sparks": "garden"}}} {
		changed, _, err := g.c.Edited(e)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(changed, func(file File) bool { return strings.HasSuffix(file.Path, "tools/circuits.js") }) {
			t.Errorf("%+v takes the script the bridge uses: %+v", e, changed)
		}
	}
}

func TestChangesAPersonPutsTheirOwn(t *testing.T) {
	t.Parallel()
	// Sam's own change is handed on to be decided as a put_draft of his
	// would be, of the team whose folder holds it; whom it is asked of is
	// Decisions'. His yes writes it, by him.
	h := newOffice(t, house(t))
	before := h.head().Hash
	sams := clerkAgain(t, "sam's")
	p, err := h.own("sam", "Sam's clerk", Edit{Files: map[string]*string{"clerk.yaml": &sams}})
	if err != nil {
		t.Fatal(err)
	}
	if put := h.decided(); len(put) != 1 || put[0].ID != p.ID || !put[0].Person || put[0].By != "sam" || put[0].Tool != ToolPut || put[0].Team != office.Top {
		t.Fatalf("Sam's own change is handed on as %+v", put)
	}
	if h.text("clerk.yaml") != house(t)["clerk.yaml"] || h.head().Hash != before {
		t.Fatal("Sam's own change was written before it was decided")
	}
	if o := h.yes(p.ID, "sam", ""); o.Ended != Written || o.Decider != "sam" || h.text("clerk.yaml") != sams || h.head().Author.Name != "sam" || !strings.Contains(h.head().Message, "Decided-by: sam") {
		t.Errorf("Sam's own change ended %+v", o)
	}
	// A rule that refuses ends it: nothing is written, and Sam is told the
	// rule's note.
	r := newOffice(t, house(t))
	before = r.head().Hash
	p, err = r.own("sam", "Sam's clerk", Edit{Files: map[string]*string{"clerk.yaml": &sams}})
	if err != nil {
		t.Fatal(err)
	}
	o, err := r.c.Decided(ctx, p.ID, Decision{By: "no-secrets.js", Kind: office.IsRule, Note: "no secrets in a worker"})
	if err != nil || o.Ended != Refused {
		t.Fatalf("the rule's refusal ended it %+v, %v", o, err)
	}
	if told := r.outcomes(); len(told) != 1 || told[0].By != "sam" || told[0].Decider != "no-secrets.js" || !strings.Contains(told[0].Text(), "no secrets in a worker") {
		t.Errorf("Sam is told %+v", told)
	}
	if r.text("clerk.yaml") != house(t)["clerk.yaml"] || r.head().Hash != before {
		t.Error("a refused change was written")
	}
	if _, err := r.c.Decided(ctx, p.ID, Decision{By: "sam", Kind: office.IsPerson, Yes: true}); err == nil || r.text("clerk.yaml") != house(t)["clerk.yaml"] {
		t.Errorf("Sam's yes after the refusal: %v", err)
	}
	// A change of the garden alone is the garden's to decide.
	beds := "Beds.\n"
	if p, err := h.own("sam", "Beds", Edit{Files: map[string]*string{"garden/notes/beds.md": &beds}}); err != nil || p.Team != "garden" {
		t.Errorf("Sam's change of the garden is %s's to decide, %v", p.Team, err)
	}
}
func TestChangesAPersonWhoDoesNotDecideCannotPutOne(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	text := clerkAgain(t, "x")
	if _, err := f.own("kit", "Kit's", Edit{Files: map[string]*string{"clerk.yaml": &text}}); err == nil || !strings.Contains(err.Error(), "ask for it in a request") {
		t.Errorf("Kit's change: %v", err)
	}
	if len(f.decided()) != 0 {
		t.Error("Kit's change was put")
	}
}

func TestChangesAShippedBridgeIsSetByItsEntry(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for file, text := range examples.NewHouse() {
		files[file] = string(text)
	}
	f := newOffice(t, files)
	entry := map[string]any{"from": "./.bulldog/bridges/telegram", "places": map[string]any{"direct": map[string]any{}}}
	changed, _, err := f.c.Edited(Edit{Set: []Setting{{Path: "office.yaml", Field: []string{"bridges", "telegram"}, Value: entry}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0].Path != "office.yaml" || !strings.Contains(changed[0].Text, "telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }") {
		t.Fatalf("the draft holds %+v", changed)
	}
	check, _ := f.c.Check(ctx, changed, Drafter{Name: "sam", Person: true})
	if !check.Fit() {
		t.Fatalf("the check is %+v", check)
	}
	has(t, check.LetsMoreBy, "the bridge telegram is new")
	p, err := f.c.PutOwn(ctx, "sam", "Telegram", changed, check.Mark)
	if err != nil {
		t.Fatal(err)
	}
	f.yes(p.ID, "sam", "")
	if h := f.history()[0]; !slices.Equal(h.Files, []string{"office.yaml"}) {
		t.Errorf("the commit holds %v", h.Files)
	}
	if o := f.keeper.Office(); o == nil || !o.Bridges["telegram"].Runs() || o.Bridges["telegram"].Dir() != ".bulldog/bridges/telegram" {
		t.Errorf("the house reads with %v", f.keeper.Problems())
	}
}

func TestChangesAPersonsChangeGoesTheOneWay(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	bad := replaced(t, house(t), "garden/team.yaml", "people: [sam]", "people: [jo]")
	if _, err := f.own("sam", "Jo", Edit{Files: map[string]*string{"garden/team.yaml": &bad}}); err == nil || !strings.Contains(err.Error(), "jo is nobody") {
		t.Errorf("a person's change with a problem: %v", err)
	}
	f.edit(map[string]string{"clerk.yaml": clerkAgain(t, "by hand")})
	text := clerkAgain(t, "sam's")
	if _, err := f.own("sam", "Clerk", Edit{Files: map[string]*string{"clerk.yaml": &text}}); err == nil || !strings.Contains(err.Error(), "edit by hand") {
		t.Errorf("a person's change over an edit by hand: %v", err)
	}
	// It may hold any file a draft may, and nothing of Bulldog's.
	own := "x"
	if _, _, err := f.c.Edited(Edit{Files: map[string]*string{".bulldog/tools/talk/talk.yaml": &own}}); err == nil {
		t.Error("a person's change holds Bulldog's own")
	}
}

func TestChangesAPersonWritesWhatTheyWereShown(t *testing.T) {
	t.Parallel()
	f := newOffice(t, house(t))
	text := clerkAgain(t, "sam's")
	files, _, _ := f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	check, _ := f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	// The folder moves on before Sam puts what Sam was shown.
	f.edit(map[string]string{"clerk.yaml": clerkAgain(t, "later")})
	f.commitByHand("Later")
	again, _, _ := f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	if _, err := f.c.PutOwn(ctx, "sam", "Clerk", again, check.Mark); err == nil || !strings.Contains(err.Error(), "Look at it again") {
		t.Errorf("a change other than was shown: %v", err)
	}
	// Nor is it put where another file of the folder changed since.
	files, _, _ = f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	check, _ = f.c.Check(ctx, files, Drafter{Name: "sam", Person: true})
	f.edit(map[string]string{"notes/rooms.md": "Changed.\n"})
	f.commitByHand("Rooms")
	if _, err := f.c.PutOwn(ctx, "sam", "Clerk", files, check.Mark); err == nil || !strings.Contains(err.Error(), "Look at it again") {
		t.Errorf("a change shown against another folder: %v", err)
	}
	// Nor is one put with no mark, which was shown to nobody, though the
	// folder moved on outside the files it touches.
	files, _, _ = f.c.Edited(Edit{Files: map[string]*string{"clerk.yaml": &text}})
	f.edit(map[string]string{"notes/rooms.md": "Changed again.\n"})
	f.commitByHand("Rooms again")
	if _, err := f.c.PutOwn(ctx, "sam", "Clerk", files, ""); err == nil || !strings.Contains(err.Error(), "Look at it again") {
		t.Errorf("a change put with no mark: %v", err)
	}
	// And one that waits on a rule while the folder moves past is not
	// written.
	ruled := house(t)
	ruled["no-secrets.js"] = "return { says: \"refer\" }\n"
	ruled["office.yaml"] = replaced(t, ruled, "office.yaml", "  everything: [sam]\n", "  everything: [sam]\n  put_draft: [./no-secrets.js, sam]\n")
	g := newOffice(t, ruled)
	p, err := g.own("sam", "Clerk", Edit{Files: map[string]*string{"clerk.yaml": &text}})
	if err != nil {
		t.Fatal(err)
	}
	g.edit(map[string]string{"clerk.yaml": clerkAgain(t, "later still")})
	g.commitByHand("Later still")
	if o := g.yes(p.ID, "sam", ""); o.Ended != NotWritten {
		t.Errorf("it ended %+v", o)
	}
}

// forty is a definition of forty lines, as a person writes one.
func forty() string {
	var b strings.Builder
	b.WriteString("# The scribe writes things down.\nagent: scribe\ninput: { said: string }\nmodel: usual\n")
	b.WriteString("system: |\n  You write things down.\n\n  Shortly.\nprompt: ${input.said}\ntools:\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "  # tool %d\n  t%d: { does: Tool %d, args: {}, job: { script: ./tools/circuits.js } }\n", i, i, i)
	}
	for b.Len() > 0 && strings.Count(b.String(), "\n") < 40 {
		b.WriteString("# more\n")
	}
	return b.String()
}

func TestChangesARewriteChangesOnlyTheLinesThatChange(t *testing.T) {
	t.Parallel()
	src := forty()
	out, kept, err := setField(src, []string{"model"}, "quick")
	if err != nil || !kept {
		t.Fatal(kept, err)
	}
	a, b := strings.Split(src, "\n"), strings.Split(out, "\n")
	if len(a) != len(b) {
		t.Fatalf("%d lines became %d", len(a), len(b))
	}
	var changed []string
	for i := range a {
		if a[i] != b[i] {
			changed = append(changed, b[i])
		}
	}
	if !slices.Equal(changed, []string{"model: quick"}) {
		t.Errorf("the lines changed are %q", changed)
	}
}

func TestChangesARewriteKeepsHowAFileIsSetOut(t *testing.T) {
	t.Parallel()
	src := "# the clerk\r\nagent: clerk\r\n\r\nmodel:    { name: sonnet, service: claude-code }   # its model\r\ntools:\r\n    order:\r\n        does: Order\r\n        job: { script: ./o.js }"
	out, kept, err := setField(src, []string{"model", "effort"}, "high")
	if err != nil || !kept {
		t.Fatal(kept, err)
	}
	want := "# the clerk\r\nagent: clerk\r\n\r\nmodel:    { name: sonnet, service: claude-code, effort: high }   # its model\r\ntools:\r\n    order:\r\n        does: Order\r\n        job: { script: ./o.js }"
	if out != want {
		t.Errorf("the file is\n%q\nand not\n%q", out, want)
	}
	// A new key of a block mapping is set in as the file sets in its lines.
	out, kept, _ = setField(src, []string{"tools", "order", "args"}, map[string]any{"part": "string"})
	if !kept || !strings.Contains(out, "\r\n        job: { script: ./o.js }\r\n        args: { part: string }") {
		t.Errorf("the file is\n%q", out)
	}
	// An empty mapping in a row is filled as the file spaces its rows.
	out, kept, _ = setField("a: { b: 1 }\nc: {}\n", []string{"c", "d"}, 2)
	if !kept || out != "a: { b: 1 }\nc: { d: 2 }\n" {
		t.Errorf("the file is %q", out)
	}
}

func TestChangesARewriteKeepsABlockOfText(t *testing.T) {
	t.Parallel()
	src := "agent: scribe\nsystem: |\n  First.\n\n  Second.\nmodel: usual\n"
	out, kept, err := setField(src, []string{"model"}, "quick")
	if err != nil || !kept || out != "agent: scribe\nsystem: |\n  First.\n\n  Second.\nmodel: quick\n" {
		t.Errorf("the file is %q, kept %v, %v", out, kept, err)
	}
	var v struct{ System string }
	yaml.Unmarshal([]byte(out), &v)
	if v.System != "First.\n\nSecond.\n" {
		t.Errorf("the block reads %q", v.System)
	}
	// Setting the block itself keeps the lines after it.
	out, kept, _ = setField(src, []string{"system"}, "Only.")
	if !kept || out != "agent: scribe\nsystem: Only.\nmodel: usual\n" {
		t.Errorf("the file is %q", out)
	}
}

func TestChangesARewriteIsCheckedAgainstThePlainText(t *testing.T) {
	t.Parallel()
	// A key whose value runs over two lines as plain text cannot be found
	// in its place: the file is written plainly, and the person told.
	src := "agent: scribe\nabout: one\n  two\nmodel: usual\n"
	out, kept, err := setField(src, []string{"about"}, "three")
	if err != nil || kept || !sameValue(out, "agent: scribe\nabout: three\nmodel: usual\n") {
		t.Errorf("the file is %q, kept %v, %v", out, kept, err)
	}
	f := newOffice(t, house(t))
	f.edit(map[string]string{"notes/plain.yaml": src})
	f.commitByHand("Plain")
	_, told, err := f.c.Edited(Edit{Set: []Setting{{Path: "notes/plain.yaml", Field: []string{"about"}, Value: "three"}}})
	if err != nil || len(told) != 1 || !strings.Contains(told[0], "could not be kept") {
		t.Errorf("the person is told %v, %v", told, err)
	}
}

func TestChangesAFileWithDocumentMarksIsNotRewritten(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"---\nagent: a\n", "agent: a\n...\n", "--- # one\nagent: a\n"} {
		if _, _, err := setField(src, []string{"agent"}, "b"); err == nil || !strings.Contains(err.Error(), "change it whole") {
			t.Errorf("%q: %v", src, err)
		}
	}
	f := newOffice(t, house(t))
	f.edit(map[string]string{"clerk.yaml": "---\n" + house(t)["clerk.yaml"]})
	f.commitByHand("Marked")
	if _, _, err := f.c.Edited(Edit{Set: []Setting{{Path: "clerk.yaml", Field: []string{"model"}, Value: "quick"}}}); err == nil || !strings.Contains(err.Error(), "clerk.yaml") {
		t.Errorf("a marked file is set: %v", err)
	}
}
