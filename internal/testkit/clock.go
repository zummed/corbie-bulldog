package testkit

import (
	"sync"
	"time"
)

// Monday is where a test's clock starts: nine o'clock UTC on a Monday,
// which is ten in London.
var Monday = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

// A Clock is a time that moves only when the test moves it. Whatever
// watches it is told each time it moves.
type Clock struct {
	mu      sync.Mutex
	now     time.Time
	watches map[chan struct{}]bool
}

// NewClock makes a clock that stands at a time.
func NewClock(at time.Time) *Clock {
	return &Clock{now: at, watches: map[chan struct{}]bool{}}
}

// Now gives the time the clock stands at.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Move moves the clock on.
func (c *Clock) Move(d time.Duration) { c.Set(c.Now().Add(d)) }

// Set puts the clock at a time.
func (c *Clock) Set(at time.Time) {
	c.mu.Lock()
	c.now = at
	for ch := range c.watches {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	c.mu.Unlock()
}

// Watch gives a channel that is sent to when the clock moves, and a func
// that ends it.
func (c *Clock) Watch() (<-chan struct{}, func()) {
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
