package bridges_test

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zummed/corbie-bulldog/internal/bridges"
)

// address serves a bridge's address in front of a desk, handing each
// start to hear.
func address(t *testing.T, d *desk, hear func(bridges.Start) (<-chan *bridges.Answer, error), after func(time.Duration) <-chan time.Time) *httptest.Server {
	srv := httptest.NewServer(&bridges.Address{Bridges: d.Bridges, Hear: hear, After: after})
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, url, body string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	must(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	must(t, err)
	defer res.Body.Close()
	got, err := io.ReadAll(res.Body)
	must(t, err)
	return res, string(got)
}

func TestBridgesDeliveryHandedUnread(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	var starts []bridges.Start
	srv := address(t, d, func(s bridges.Start) (<-chan *bridges.Answer, error) {
		starts = append(starts, s)
		ch := make(chan *bridges.Answer, 1)
		ch <- nil
		return ch, nil
	}, nil)
	body := `{"type":"event_callback","event":{"type":"message","user":"U0DANA00","text":"hi"}}`
	res, _ := post(t, srv.URL+"/bridges/slack", body, map[string]string{"X-Slack-Signature": "v0=wrong", "X-Slack-Request-Timestamp": "1700000000"})
	if res.StatusCode != http.StatusOK || len(starts) != 1 {
		t.Fatalf("the delivery is answered %d, and starts %d runs", res.StatusCode, len(starts))
	}
	got := starts[0].Input["delivery"].(bridges.Delivery)
	if got.Method != "POST" || got.Body != body || got.Headers["X-Slack-Signature"] != "v0=wrong" || got.Headers["X-Slack-Request-Timestamp"] != "1700000000" {
		t.Errorf("the delivery is handed as %#v", got)
	}
	if starts[0].Machine != d.o.Bridges["slack"].In || starts[0].Kind != bridges.KindIn || starts[0].Labels["bridge"] != "slack" {
		t.Errorf("the run is %#v", starts[0])
	}
	// The in machine finds the signature wrong, and hears nothing.
	_, err := d.InEnded("slack", "a run", bridges.End{Output: map[string]any{"heard": []any{}, "answer": map[string]any{"status": 401}}})
	must(t, err)
	if d.latest() != 0 {
		t.Error("something was taken")
	}
}

func TestBridgesAnsweredAsTheRunSays(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	srv := address(t, d, func(s bridges.Start) (<-chan *bridges.Answer, error) {
		ch := make(chan *bridges.Answer, 1)
		taken, err := d.InEnded("slack", "run", bridges.End{Output: map[string]any{"heard": []any{}, "answer": map[string]any{"status": 200, "body": "challenge-1"}}})
		must(t, err)
		ch <- taken.Answer
		return ch, nil
	}, nil)
	if res, body := post(t, srv.URL+"/bridges/slack", `{"challenge":"challenge-1"}`, nil); res.StatusCode != 200 || body != "challenge-1" {
		t.Errorf("the caller is answered %d %q", res.StatusCode, body)
	}
	// A run that takes longer is answered as received, and taken when it
	// ends.
	waited := make(chan time.Time)
	ended, took := make(chan struct{}), make(chan struct{})
	slow := address(t, d, func(s bridges.Start) (<-chan *bridges.Answer, error) {
		ch := make(chan *bridges.Answer, 1)
		go func() {
			<-ended
			taken, err := d.InEnded("slack", "slow run", bridges.End{Output: heard(0, said("U0DANA00", "#help", "C1/1.0", "slow"))})
			if err != nil {
				t.Error(err)
			}
			ch <- taken.Answer
			close(took)
		}()
		return ch, nil
	}, func(d time.Duration) <-chan time.Time {
		if d != bridges.AnswerWithin {
			t.Errorf("the run is waited for %v", d)
		}
		return waited
	})
	answered := make(chan string)
	go func() {
		res, body := post(t, slow.URL+"/bridges/slack", `{}`, nil)
		answered <- res.Status + " " + body
	}()
	waited <- time.Now()
	if got := <-answered; got != "200 OK " {
		t.Errorf("the caller is answered %q", got)
	}
	close(ended)
	<-took
	if d.latest() != 1 {
		t.Error("what the slow run heard was not taken")
	}
}

func TestBridgesOversizedDeliveryRefused(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	started := 0
	srv := address(t, d, func(s bridges.Start) (<-chan *bridges.Answer, error) {
		started++
		return nil, nil
	}, nil)
	res, body := post(t, srv.URL+"/bridges/slack", strings.Repeat("x", bridges.MaxDelivery+1), nil)
	if res.StatusCode != http.StatusRequestEntityTooLarge || started != 0 || !strings.Contains(body, "larger") {
		t.Errorf("an oversized delivery is answered %d %q, starting %d runs", res.StatusCode, body, started)
	}
}

func TestBridgesDeliveryNotLogged(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	before := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(before)
	srv := httptest.NewUnstartedServer(&bridges.Address{Bridges: d.Bridges, Hear: func(s bridges.Start) (<-chan *bridges.Answer, error) {
		ch := make(chan *bridges.Answer, 1)
		ch <- &bridges.Answer{Status: 200}
		return ch, nil
	}})
	srv.Config.ErrorLog = log.New(&logged, "", 0)
	srv.Start()
	defer srv.Close()
	for _, path := range []string{"/bridges/slack", "/bridges/gitlab", "/bridges/nothing"} {
		post(t, srv.URL+path, "body-holding-sk-live-123", map[string]string{"X-Slack-Signature": "v0=sig-holding-key-456"})
	}
	post(t, srv.URL+"/bridges/slack", strings.Repeat("sk-live-123", bridges.MaxDelivery/8), nil)
	if strings.Contains(logged.String(), "sk-live-123") || strings.Contains(logged.String(), "key-456") {
		t.Errorf("the log holds %q", logged.String())
	}
}

func TestBridgesDeliveryOnlyToHearingBridge(t *testing.T) {
	t.Parallel()
	d := open(t, work(t))
	started := 0
	srv := address(t, d, func(s bridges.Start) (<-chan *bridges.Answer, error) {
		started++
		return nil, nil
	}, nil)
	for _, path := range []string{"/bridges/gitlab", "/bridges/jira", "/bridges/", "/bridges/slack/more"} {
		if res, _ := post(t, srv.URL+path, "{}", nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("a delivery to %s is answered %d", path, res.StatusCode)
		}
	}
	if started != 0 {
		t.Errorf("%d runs were started", started)
	}
	if _, err := d.Hear("gitlab", bridges.Delivery{}); err == nil {
		t.Error("a delivery to the GitLab bridge, which listens, is heard")
	}
}
