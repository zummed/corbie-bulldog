package bridges_test

import (
	"slices"
	"testing"

	"github.com/zummed/corbie-bulldog/internal/requests"
)

// closed gives the status of an issue closed in GitLab by someone.
func closed(from, place string) map[string]any {
	return map[string]any{"id": "s" + itoa(ids.Add(1)), "kind": "status", "from": from, "heard_in": "platform/api", "place": place,
		"status": "closed", "closed": true, "about": map[string]any{"link": "https://gitlab.example.com/" + place}}
}

func TestBridgesStatusHandedToItsRequest(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	n := d.latest()
	d.hear("gitlab", map[string]any{"id": "s0", "kind": "status", "from": "1187", "heard_in": "platform/api", "place": "platform/api#7", "status": "in review"})
	at := d.clock.Now()
	taken := d.hear("gitlab", closed("1187", "platform/api#7"))
	r, _ := d.requests.Get(n)
	if r.Source == nil || r.Source.Status != "closed" || !r.Source.Closed || r.Source.By != requests.Identity("gitlab", "1187") ||
		!r.Source.At.Equal(at) || r.Source.Link != "https://gitlab.example.com/platform/api#7" {
		t.Errorf("the request's source is %#v", r.Source)
	}
	if !slices.Contains(taken.Stopped, n) {
		t.Errorf("the close stopped %v", taken.Stopped)
	}
	if v, err := d.requests.View(n, "raj"); err != nil || v.Source.Status != "closed" {
		t.Errorf("Raj sees the source as %#v, %v", v, err)
	}
}

func TestBridgesStatusOfNoRequestMakesNothing(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	taken := d.hear("gitlab", closed("1187", "platform/api#99"))
	if d.latest() != 0 || len(taken.Stopped) != 0 || len(d.carry()) != 0 {
		t.Error("a status of an issue that began no request made something")
	}
	if listening := d.listen(); len(listening) != 1 {
		t.Errorf("the listening runs are %v", listening)
	}
}

func TestBridgesNoStatusCopy(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	d.hear("gitlab", said("1187", "platform/api", "platform/api#7", "the webhook retries twice"))
	n := d.latest()
	d.hear("gitlab", map[string]any{"id": "s0", "kind": "status", "from": "1187", "heard_in": "platform/api", "place": "platform/api#7", "status": "in review"})
	d.hear("gitlab", map[string]any{"id": "s1", "kind": "status", "from": "1041", "heard_in": "platform/api", "place": "platform/api#7", "status": "reopened"})
	for _, s := range d.carry() {
		if s.Bridge == "gitlab" {
			t.Errorf("an out run is started on GitLab: %#v", s.Input)
		}
	}
	r, _ := d.requests.Get(n)
	if r.Source.Status != "reopened" {
		t.Errorf("the status kept is %#v", r.Source)
	}
	if got := d.carried(n); len(got) != 0 {
		t.Errorf("the statuses are carried as %v", got)
	}
}
