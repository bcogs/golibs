// Package oil is stuff that should be language builtins: min, max, tuples,
// optionals, functions to access maps and while creating the entry if it
// doesn't exist...
// A lot of small things to oil your cogs.
package oil

import (
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
