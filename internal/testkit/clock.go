package testkit

import (
	"sync"
	"time"
)

// Monday is where a test's clock starts: 09:00 UTC on Monday 5 October
// 2026, which is 10:00 in London.
var Monday = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

// A Clock is a time that moves only when the test moves it.
type Clock struct {
	mu      sync.Mutex
	now     time.Time
	watches map[chan struct{}]bool
}

// NewClock makes a clock set at a time.
func NewClock(at time.Time) *Clock {
	return &Clock{now: at, watches: map[chan struct{}]bool{}}
}

// Now gives the clock's time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Move moves the clock on by a duration, and tells each watch.
func (c *Clock) Move(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	for ch := range c.watches {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// watch gives a channel that is sent to each time the clock moves, and a
// function that stops it.
func (c *Clock) watch() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	c.mu.Lock()
	c.watches[ch] = true
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		delete(c.watches, ch)
		c.mu.Unlock()
	}
}
