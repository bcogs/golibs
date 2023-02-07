// Package oil is stuff that should be language builtins: min, max, tuples,
// optionals, functions to access maps and while creating the entry if it
// doesn't exist...
// A lot of small things to oil your cogs.
package oil

import (
	"fmt"
	"strconv"
	"sync"

	"golang.org/x/exp/constraints"
)

// Number is a constraint that permits any number type.
type Number interface {
	OrderedNumber | constraints.Complex
}

// OrderedNumber is a constraint that permits any ordered number type.
type OrderedNumber interface {
	constraints.Float | constraints.Integer
}

// Abs returns the absolute value of a number.
func Abs[T OrderedNumber](x T) T {
	if x < T(0) {
		return -x
	}
	return x
}

// If mimics the C ternary operator ("?").
// It returns ifTrue or ifFalse depending on the value of a boolean.
func If[T any](b bool, ifTrue, ifFalse T) T {
	if b {
		return ifTrue
	}
	return ifFalse
}

// Max returns the max of two ordered numbers.
func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Min returns the min of two ordered numbers.
func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Atoi parses an integer value, verifies that it's between min and max, and if
// there's a parse error or it's out of bounds, returns an error message that
// looks like: invalid $whatIsIt blah blah
func Atoi[T constraints.Signed](s, whatIsIt string, min T, max T) (T, error) {
	k, err := strconv.ParseInt(s, 0, 64)
	switch {
	case err != nil:
		return 0, fmt.Errorf("invalid %s %q - it should be an integer", whatIsIt, s)
	case T(k) < min:
		return T(k), fmt.Errorf("invalid %s %s - it should be at least %d", whatIsIt, s, min)
	case T(k) > max:
		return T(k), fmt.Errorf("invalid %s %s - it should be at most %d", whatIsIt, s, max)
	}
	return T(k), nil
}

// Atou parses an unsigned integer value, verifies that it's between min and
// max, and if there's a parse error or it's out of bounds, returns an error
// message that looks like: invalid $whatIsIt blah blah
func Atou[T constraints.Unsigned](s, whatIsIt string, min T, max T) (T, error) {
	k, err := strconv.ParseUint(s, 0, 64)
	switch {
	case err != nil:
		return 0, fmt.Errorf("invalid %s %q - it should be an integer", whatIsIt, s)
	case T(k) < min:
		return T(k), fmt.Errorf("invalid %s %s - it should be at least %d", whatIsIt, s, min)
	case T(k) > max:
		return T(k), fmt.Errorf("invalid %s %s - it should be at most %d", whatIsIt, s, max)
	}
	return T(k), nil
}

// Ignore ignores all its arguments and does nothing.
// It's convenient when a linter bugs you about ignoring an error that you really don't care about.
func Ignore(...any) {}

// First returns its first argument.
func First[T any](first T, _ ...any) T { return first }

// Second returns its second argument.
func Second[T any](_ any, second T, _ ...any) T { return second }

// Third returns its third argument.
func Third[T any](_, _ any, third T, _ ...any) T { return third }

// Fourth returns its fourth argument.
func Fourth[T any](_, _, _ any, fourth T, _ ...any) T { return fourth }

// Pair is a pair of values of arbitrary types.
type Pair[T1, T2 any] struct {
	First  T1
	Second T2
}

// NewPair creates a Pair.
func NewPair[T1, T2 any](x1 T1, x2 T2) Pair[T1, T2] { return Pair[T1, T2]{x1, x2} }

// Triplet is a triplet of values of arbitrary types.
type Triplet[T1, T2, T3 any] struct {
	First  T1
	Second T2
	Third  T3
}

// NewTriplet creates a Triplet.
func NewTriplet[T1, T2, T3 any](x1 T1, x2 T2, x3 T3) Triplet[T1, T2, T3] {
	return Triplet[T1, T2, T3]{x1, x2, x3}
}

// Quadruplet is a quadruplet of values of arbitrary types.
type Quadruplet[T1, T2, T3, T4 any] struct {
	First  T1
	Second T2
	Third  T3
	Fourth T4
}

// NewQuadruplet creates a Quadruplet.
func NewQuadruplet[T1, T2, T3, T4 any](x1 T1, x2 T2, x3 T3, x4 T4) Quadruplet[T1, T2, T3, T4] {
	return Quadruplet[T1, T2, T3, T4]{x1, x2, x3, x4}
}

// Optional wraps any type, allowing values to be either set or unset.
type Optional[T any] struct {
	Val   T
	IsSet bool
}

// NewOptional creates a new Optional.
func NewOptional[T any](val T, isSet bool) Optional[T] { return Optional[T]{val, isSet} }

// Get gets the value from an Optional or a default value if it's unset.
func (o Optional[T]) Get(defaultValue T) T {
	if o.IsSet {
		return o.Val
	}
	return defaultValue
}

// Set sets a value in an Optional and returns the Optional itself.
func (o *Optional[T]) Set(val T) *Optional[T] {
	o.Val, o.IsSet = val, true
	return o
}

// SetDefault sets the value of an Optional if it doesn't have a value already, and returns the Optional itself.
func (o *Optional[T]) SetDefault(val T) *Optional[T] {
	if !o.IsSet {
		o.Val, o.IsSet = val, true
	}
	return o
}

// Unset unsets an Optional and returns the Optional itself.
func (o *Optional[T]) Unset() *Optional[T] {
	o.IsSet = false
	return o
}

// MapGet gets a value from a map and returns a default if the map doens't have the specified key.
func MapGet[K comparable, V any](m map[K]V, key K, defaultValue V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultValue
}

// MapSetDefault sets a value in a map for a given key, if that key isn't in the map already.  It returns the map itself.
func MapSetDefault[K comparable, V any](m map[K]V, key K, value V) map[K]V {
	if _, ok := m[key]; !ok {
		m[key] = value
	}
	return m
}

// MapGetOrNew gets a value from a map for a given key, or creates it (and inserts it)
// by calling a user specified function if it doesn't exist.  It returns the value.
func MapGetOrNew[K comparable, V any](m map[K]V, key K, create func() V) V {
	v, ok := m[key]
	if !ok {
		v = create()
		m[key] = v
	}
	return v
}

// MapGetOrNewRef gets a value from a map of references for a given key, or creates
// it (and inserts it) by calling new if it doesn't exist.  It returns the value.
func MapGetOrNewRef[K comparable, V any](m map[K]*V, key K) *V {
	v, ok := m[key]
	if !ok {
		v = new(V)
		m[key] = v
	}
	return v
}

// MapFromSlice creates a map whose keys are the elements of a slice, and values are all the same.
func MapFromSlice[K comparable, V any](slice []K, value V) map[K]V {
	m := make(map[K]V)
	for _, k := range slice {
		m[k] = value
	}
	return m
}

// FanIn writes anything it reads from a number of channels, the producers, to a single channel, the consumer.
// If all the producers get closed, it closes the consumer and returns.
func FanIn[T any](consumer chan<- T, producers ...<-chan T) {
	var wg sync.WaitGroup
	wg.Add(len(producers))
	for _, producer := range producers {
		go func(producer <-chan T) {
			defer wg.Done()
			for x := range producer {
				consumer <- x
			}
		}(producer)
	}
	wg.Wait()
	close(consumer)
}

// FanOut replicates everything it reads from a channel, the producer, to an arbitrary number of channels, the consumers.
// If the producer is closed, FanOut closes the consumers and returns.
func FanOut[T any](producer <-chan T, consumers ...chan<- T) {
	for x := range producer {
		for _, consumer := range consumers {
			consumer <- x
		}
	}
	for _, consumer := range consumers {
		close(consumer)
	}
}
