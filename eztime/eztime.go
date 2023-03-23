// Package eztime provides helpers to simplify frequent things done with the standard library time package.
package eztime

import (
	"time"
)

// MustLoadLocation is a wrapper around time.LoadLocation that panics on error.
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

// MustParseInLocation is a wrapper around time.ParseInLocation that panics on error.
func MustParseInLocation(layout, value string, loc *time.Location) time.Time {
	result, err := time.ParseInLocation(layout, value, loc)
	if err != nil {
		panic(err)
	}
	return result
}

// MustParseDuration is a wrapper around time.ParseDuration that panics on error.
func MustParseDuration(s string) time.Duration {
	result, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return result
}

// CancellableSleep sleeps for a certain duration at least, or until a read from
// a channel returns something.
// Usually, the chan is a ctx.Done(), but it doesn't have to be.
func CancellableSleep[T any](d time.Duration, c <-chan T) T {
	var result T
	if d < 0 {
		return result
	}
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
	case result = <-c:
		timer.Stop()
	}
	return result
}
