package work

import (
	"strings"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/bridges"
)

func TestASignInLinkIsCarriedOnceAskedFor(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.listening("telegram")
	b.still()
	if _, err := b.b.SignIn("telegram", "sam", "Open https://house.example/signin/abc to sign in."); err != nil {
		t.Fatal(err)
	}
	// Nothing that is written to a request tells of the link: it is
	// carried because the work is told to carry.
	b.still()
	if going := b.going(bridges.KindSignIn, "telegram"); len(going) != 0 {
		t.Fatalf("the link was carried before the work was told to carry: %v", going)
	}
	b.w.Carry()
	b.until("the link is carried to Sam", func() bool {
		for _, r := range b.going(bridges.KindSignIn, "telegram") {
			if in := b.input(r.ID); in["to"] == "4471" && strings.Contains(in["text"].(string), "/signin/abc") {
				return true
			}
		}
		return false
	})
}

func TestTheReadingEveryAreaTookIsGiven(t *testing.T) {
	t.Parallel()
	b := newBench(t, house(t))
	b.until("the first reading is given", func() bool { return b.w.Read() == b.keeper.Office() })
	before := b.w.Read()
	b.rewrite(map[string]string{"office.yaml": strings.Replace(house(t)["office.yaml"], `telegram: "4472"`, `telegram: "9999"`, 1)})
	b.until("the new reading is given", func() bool { return b.w.Read() != before })
	if got := b.w.Read().People["kit"].Reached[0].ID; got != "9999" {
		t.Errorf("the reading given reaches Kit at %s", got)
	}
}
