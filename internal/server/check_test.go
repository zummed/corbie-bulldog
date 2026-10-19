package server

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/office"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// stamps gives each file of a folder with its size and time.
func stamps(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[path] = fmt.Sprint(info.ModTime(), info.Mode(), info.Size())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestServerCheckListsTheWorkOffice(t *testing.T) {
	t.Parallel()
	files := testkit.Files(t, examples.Work())
	files["payments/team.yaml"] += "models:\n  usual: { name: claude-opus-5-5, secret: PAYMENTS_KEY }\n"
	files["office.yaml"] += "heed: { rule: ./for-us.js, timeout: 5s }\n"
	files["for-us.js"] = "return { says: \"act\" }\n"
	dir := testkit.Folder(t, files)
	before := stamps(t, dir)
	var out strings.Builder
	if err := Check(dir, &out); err != nil {
		t.Fatalf("the work office is checked with %v:\n%s", err, out.String())
	}
	listing := out.String()
	for _, part := range []string{
		"Payments (payments), within Platform support: Charges, refunds and the ledger. Holds customer data.\n  people: lee, priya\n  lead: ledger\n",
		"  disclosure check: ./no-account-numbers.yaml\n  heed: none\n",
		"  heed: ./for-us.js, with { timeout: 5s }\n",
		"  ledger, an agent, with the model { name: claude-opus-5-5, secret: PAYMENTS_KEY }, named usual, from Payments\n",
		"    refund waits for a yes, decided by lee, then priya\n",
		"  bot, an agent, with the model { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }, named usual, from Platform support\n",
		"Platform (platform), within Platform support: GitLab, the pipelines and deploys.\n  people: none\n  lead: ops\n  decides everything: al, from the everything of Platform support\n",
		"Support (support), within Platform support",
		"slack, a bridge that ",
		"gitlab, a bridge that ",
		"3 runs at work at once\n",
	} {
		if !strings.Contains(listing, part) {
			t.Errorf("the listing holds no %q:\n%s", part, listing)
		}
	}
	if strings.Count(listing, ", the top team") != 1 || strings.Count(listing, "), within ") != 3 {
		t.Errorf("the listing does not hold four teams:\n%s", listing)
	}
	after := stamps(t, dir)
	if len(after) != len(before) {
		t.Errorf("the check wrote into the folder: %d files and folders, and %d before", len(after), len(before))
	}
	for path, stamp := range before {
		if after[path] != stamp {
			t.Errorf("the check touched %s", path)
		}
	}
	// A model named by a word no team gives is the ledger's problem, and
	// the rest of the office reads.
	files["payments/ledger.yaml"] = strings.Replace(files["payments/ledger.yaml"], "model: usual", "model: quick", 1)
	dir = testkit.Folder(t, files)
	out.Reset()
	err := Check(dir, &out)
	if err == nil || !strings.Contains(err.Error(), "payments/ledger.yaml") || !strings.Contains(err.Error(), "quick") {
		t.Fatalf("a model no team gives was checked as %v", err)
	}
	if !strings.Contains(out.String(), "ledger, an agent") || !strings.Contains(out.String(), "it cannot run") || !strings.Contains(out.String(), "  bot, an agent, with the model") {
		t.Errorf("the rest of the office does not read:\n%s", out.String())
	}
}

func TestServerCheckReportsEveryProblem(t *testing.T) {
	t.Parallel()
	files := house(t)
	files["fixer.yaml"] = strings.Replace(files["fixer.yaml"], "model: usual", "model: quick", 1)
	files["clerk.yaml"] = "agent: clerk\ninput: [said]\nmodel: usual\nprompt: ${input.said}\ntools:\n  sums: { does: Add, args: {}, job: { script: ./tools/missing.js } }\n"
	files["office.yaml"] = strings.Replace(files["office.yaml"], "order_part: [./small-orders.js, sam]", "order_part: [./big-orders.js, sam]", 1)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "      direct: {}", "      direct: {}\n  slack:\n    places: {}", 1)
	files["bridges/gitlab/in.yaml"] = "statechart: in\ninitial: a\nstates: { a: { end: done } }\n"
	files["garden/team.yaml"] = strings.Replace(files["garden/team.yaml"], `to: gardener`, `to: gardener, every: day`, 1)
	dir := testkit.Folder(t, files)
	_, read := office.Read(dir)
	var problems office.Problems
	if !errors.As(read, &problems) {
		o, _ := office.Read(dir)
		problems = o.Problems
	}
	var out strings.Builder
	err := Check(dir, &out)
	if err == nil {
		t.Fatalf("an office with problems was checked as good:\n%s", out.String())
	}
	inFiles := map[string]bool{}
	for _, p := range problems {
		inFiles[p.File] = true
		if !strings.Contains(err.Error(), p.String()) {
			t.Errorf("the check leaves out %q:\n%v", p.String(), err)
		}
	}
	for _, file := range []string{"fixer.yaml", "clerk.yaml", "office.yaml", "garden/team.yaml"} {
		if !inFiles[file] {
			t.Errorf("no problem is in %s: %v", file, problems)
		}
	}
	if len(problems) < 6 {
		t.Errorf("the reading found only %v", problems)
	}
}
