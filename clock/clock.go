// Package clock makes the functionality of the time package injectable in unit tests and other code.
// It also provides a few convenient goodies, that reduce the work needed to write unit tests.
// To benefit from this package, instead of calling time.Now directly, pass
// clock.Real (type: clock.Clock) to your functions or structs, and call its
// Now methods: Now, Sleep etc.
// Then in unit tests, pass a clock that you control, created with clock.NewController.
package clock

import (
	"container/heap"
	"sync"
	"time"
)

// Clock is the top level interface that encapsulates the time package functionality.
type Clock interface {
	After(time.Duration) <-chan time.Time
	NewTimer(d time.Duration) *Timer
	Now() time.Time
	Since(time.Time) time.Duration
	Sleep(time.Duration)
	Until(time.Time) time.Duration
}

// Timer is an equivalent of time.Timer with a reduced API.
type Timer struct {
	C <-chan time.Time
}

type realClock struct{}

// Real is a Clock whose methods call the functions of the time package with the same name.
var Real = Clock(realClock{})

func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }                 // After wraps time.After.
func (realClock) NewTimer(d time.Duration) *Timer        { return &Timer{C: time.NewTimer(d).C} } // NewTimer wraps time.NewTimer.
func (realClock) Now() time.Time                         { return time.Now() }                    // Now wraps time.Now.
func (realClock) Since(t time.Time) time.Duration        { return time.Since(t) }                 // Since wraps time.Since.
func (realClock) Sleep(d time.Duration)                  { time.Sleep(d) }                        // Sleep wraps time.Sleep.
func (realClock) Until(t time.Time) time.Duration        { return time.Until(t) }                 // Until wraps time.Until.

// Controller is a Clock implementation that gives you control over time!
type Controller struct {
	Opts ControllerOpts

	mu       sync.RWMutex // PROTECTS EVERYTHING BELOW
	watchers watcherHeap
	now      time.Time
}

// ControllerOpts are options for Controller configuration.
type ControllerOpts struct {
	InitialTime time.Time // optional; if zero, the initial time is time.Now()
}

// NewController creates a new controller.  The options argument can be nil.
func NewController(options *ControllerOpts) *Controller {
	c := new(Controller)
	if options != nil {
		c.Opts = *options
	}
	o := c.Opts
	if o.InitialTime.IsZero() {
		c.now = time.Now()
	} else {
		c.now = o.InitialTime
	}
	return c
}

// After is equivalent to NewTimer(d).C.
func (c *Controller) After(d time.Duration) <-chan time.Time { return c.NewTimer(d).C }

// NewTimer creates a new Timer that will send the current simulated time on its channel after it becomes >= c.Now().Add(d).
func (c *Controller) NewTimer(d time.Duration) *Timer {
	if d <= 0 {
		ch := make(chan time.Time, 1)
		ch <- c.Now()
		return &Timer{C: ch}
	}
	watcher := watcher{C: make(chan time.Time, 1)}
	c.mu.Lock()
	watcher.Threshold = c.now.Add(d)
	c.watchers.Push(watcher)
	c.mu.Unlock()
	return &Timer{C: watcher.C}
}

// Now returns the simulated current time.
func (c *Controller) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *Controller) Since(t time.Time) time.Duration { return c.Now().Sub(t) } // Since is shorthand for time.Now().Sub(t).

// Sleep advances the simulated current time by the passed duration, it it's >0.
func (c *Controller) Sleep(d time.Duration) {
	if d > 0 {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.now = c.now.Add(d)
		for len(c.watchers) > 0 {
			w := c.watchers.First()
			if w.Threshold.After(c.now) {
				return
			}
			w.C <- w.Threshold
			heap.Pop(&c.watchers)
		}
	}
}

func (c *Controller) Until(t time.Time) time.Duration { return t.Sub(c.Now()) } // Until is shorthand for t.Sub(c.Now()).

// MustLoadLocation is a wrapper around LoadLocation that panics on error.
func MustLoadLocation(name string) *time.Location {
	result, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return result
}

// MustParse is a wrapper around time.Parse that panics on parse error.
func MustParse(layout, value string) time.Time {
	result, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return result
}

// MustParseInLocation is a wrapper around time.ParseInLocation that panics on parse error.
func MustParseInLocation(layout, value string, loc *time.Location) time.Time {
	result, err := time.ParseInLocation(layout, value, loc)
	if err != nil {
		panic(err)
	}
	return result
}

// MustParseDuration is a wrapper around time.ParseDuration that panics on parse error.
func MustParseDuration(s string) time.Duration {
	result, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return result
}
