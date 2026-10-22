package screens

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/examples"
	"github.com/zummed/corbie-bulldog/internal/testkit"
)

// stream opens the stream of changes as the browser, and gives the data of
// each change as it comes.
func (b *browser) stream() <-chan map[string]any {
	b.w.t.Helper()
	open, stop := context.WithCancel(ctx)
	b.w.t.Cleanup(stop)
	req, _ := http.NewRequestWithContext(open, "GET", b.w.server.URL+"/api/events", nil)
	res, err := b.client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		b.w.t.Fatalf("the stream answered %v (%v)", res, err)
	}
	changes := make(chan map[string]any, 1000)
	go func() {
		defer close(changes)
		defer res.Body.Close()
		lines := bufio.NewScanner(res.Body)
		event := ""
		for lines.Scan() {
			line := lines.Text()
			if name, ok := strings.CutPrefix(line, "event: "); ok {
				event = name
			}
			if data, ok := strings.CutPrefix(line, "data: "); ok && event == "changed" {
				var change map[string]any
				json.Unmarshal([]byte(data), &change)
				changes <- change
			}
		}
	}()
	return changes
}

// until reads changes until one is the change given, and gives every
// change read.
func until(t *testing.T, changes <-chan map[string]any, want string) []string {
	t.Helper()
	var read []string
	limit := time.NewTimer(testkit.Patience)
	defer limit.Stop()
	for {
		select {
		case change, ok := <-changes:
			if !ok {
				t.Fatalf("the stream ended before %s, after %v", want, read)
			}
			read = append(read, jsonOf(change))
			if read[len(read)-1] == want {
				return read
			}
		case <-limit.C:
			t.Fatalf("waited %v for %s, and read only %v", testkit.Patience, want, read)
		}
	}
}

func TestScreensEventsFilteredBySight(t *testing.T) {
	t.Parallel()
	w := newWorld(t, examples.Work, says, nil)
	w.screens.Interval = 20 * time.Millisecond
	dana, lee := w.as("dana"), w.as("lee")
	danas, lees := dana.stream(), lee.stream()

	// Lee's request of payments is named to Lee, and never to Dana, though
	// the ledger names her in a question payments may not put to her.
	his := lee.ask("payments", "Was the card of order 4411 refunded?")
	w.ended(his)
	until(t, lees, `{"request":"`+his+`"}`)
	hers := dana.ask("payments", "Was order 4412 refunded?")
	w.ended(hers)
	read := until(t, danas, `{"request":"`+hers+`"}`)
	again := dana.ask("support", "Tell me when 4412 is refunded.")
	read = append(read, until(t, danas, `{"request":"`+again+`"}`)...)
	for _, change := range read {
		if strings.Contains(change, his) {
			t.Errorf("Dana is told of Lee's request: %v", read)
		}
	}
	until(t, lees, `{"request":"`+hers+`"}`)

	// The office read again is named to everyone.
	w.reread(func(text string) string {
		return strings.Replace(text, "name: Platform support", "name: Support of the platform", 1)
	})
	until(t, danas, `{"office":true}`)
	until(t, lees, `{"office":true}`)
	if me := dana.get("/api/me"); me["office"].(map[string]any)["name"] != "Support of the platform" {
		t.Errorf("Dana's office is %v", me["office"])
	}

	// A person whose entry changes is signed out, and their stream ends.
	w.reread(func(text string) string {
		return strings.Replace(text, "dana:  { name: Dana, slack: U0DANA00 }", "dana:  { name: Dana, slack: U0DANA01 }", 1)
	})
	limit := time.NewTimer(testkit.Patience)
	defer limit.Stop()
	for {
		select {
		case _, open := <-danas:
			if !open {
				return
			}
		case <-limit.C:
			t.Fatalf("waited %v, and Dana's stream has not ended", testkit.Patience)
		}
	}
}
