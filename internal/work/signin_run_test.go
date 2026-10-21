package work

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// signingIn gives the house with a stand-in sign-in method, test, and
// Sam's account by it.
func signingIn(t *testing.T) map[string]string {
	t.Helper()
	files := house(t)
	files["office.yaml"] = strings.Replace(files["office.yaml"], "signin:\n", "signin:\n  test: { label: Test }\n", 1)
	for name, text := range testkit.SignIn() {
		files["signin/test/"+name] = text
	}
	return files
}

func TestASignInMethodsRunIsLabelledAndAnswered(t *testing.T) {
	t.Parallel()
	b := newBench(t, signingIn(t))
	b.still()
	m := b.office().SignIns["test"]
	ended := func(code string) SignedIn {
		t.Helper()
		signed, err := b.w.SignIn(m, map[string]any{"code": code, "verifier": "v", "redirect": "https://house.example/signin/test/back", "nonce": "n"})
		if err != nil {
			t.Fatal(err)
		}
		var out SignedIn
		b.until("the sign-in run of "+code+" has ended", func() bool {
			select {
			case out = <-signed:
				return true
			default:
				return false
			}
		})
		return out
	}
	if got := ended("sam@example.com"); got != (SignedIn{Identity: "sam@example.com"}) {
		t.Errorf("the run answered %+v", got)
	}
	if got := ended("refuse:Not verified."); got != (SignedIn{Refused: "Not verified."}) {
		t.Errorf("the run answered %+v", got)
	}
	if got := ended("fail"); got.Failed == "" || got.Identity != "" {
		t.Errorf("the run that failed answered %+v", got)
	}
	// Each is found by its labels, the office's among them, and is given
	// the method's settings beside what the browser brought.
	runs := b.listed(map[string]string{LabelKind: KindSignIn, LabelSignIn: "test"})
	if len(runs) != 3 {
		t.Fatalf("the sign-in runs are %v", runs)
	}
	in := b.input(runs[0].ID)
	if settings, _ := in["settings"].(map[string]any); settings["label"] != "Test" || in["verifier"] != "v" || in["nonce"] != "n" {
		t.Errorf("the run was given %v", in)
	}

	// While as many of the method's runs go as may, another is not
	// started; once one has ended, it is.
	b.w.mu.Lock()
	for i := range SignIns {
		b.w.signing[fmt.Sprintf("going-%d", i)] = signingRun{method: "test", ended: make(chan SignedIn, 1)}
	}
	b.w.mu.Unlock()
	if _, err := b.w.SignIn(m, map[string]any{"code": "sam@example.com", "verifier": "v", "redirect": "r", "nonce": "n"}); !errors.Is(err, ErrSigningBusy) {
		t.Errorf("a sign-in past the most that go at once gave %v", err)
	}
	if runs := b.listed(map[string]string{LabelKind: KindSignIn, LabelSignIn: "test"}); len(runs) != 3 {
		t.Errorf("a sign-in past the most that go at once started a run: %v", runs)
	}
	b.w.signedIn("going-0", nil, ending{status: lost, how: "lost"})
	if got := ended("sam@example.com"); got != (SignedIn{Identity: "sam@example.com"}) {
		t.Errorf("once a run ended, the next answered %+v", got)
	}
}
