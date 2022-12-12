package oil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bcogs/golibs/oil"
)

func TestAbs(t *testing.T) {
	assert.Equal(t, 3, oil.Abs(-3))
	assert.Equal(t, 3, oil.Abs(3))
	assert.Equal(t, 3.0, oil.Abs(-3.0))
	assert.Equal(t, 3.0, oil.Abs(3.0))
}

func TestIf(t *testing.T) {
	assert.Equal(t, 0, oil.If(false, 1, 0))
	assert.Equal(t, 1, oil.If(true, 1, 0))
}

func TestMax(t *testing.T) {
	assert.Equal(t, int64(-4), oil.Max(int64(-8), int64(-4)))
	assert.Equal(t, 3.2, oil.Max(3.2, 1.))
	assert.Equal(t, "foo", oil.Max("bar", "foo"))
}

func TestMin(t *testing.T) {
	assert.Equal(t, uint(3), oil.Min(uint(3), uint(5)))
	assert.Equal(t, 3.2, oil.Min(8., 3.2))
	assert.Equal(t, "bar", oil.Min("bar", "foo"))
}

func TestPair(t *testing.T) {
	assert.Equal(t, oil.Pair[int, string]{First: 1, Second: "a"}, oil.NewPair(1, "a"))
}

func TestNewTriplet(t *testing.T) {
	assert.Equal(t, oil.Triplet[int, string, float64]{First: 1, Second: "a", Third: 1.}, oil.NewTriplet(1, "a", 1.))
}

func TestNewQuadruplet(t *testing.T) {
	assert.Equal(t, oil.Quadruplet[int, string, float64, uint]{First: 1, Second: "a", Third: 1., Fourth: uint(2)}, oil.NewQuadruplet(1, "a", 1., uint(2)))
}

func TestOptional(t *testing.T) {
	assert.Equal(t, oil.Optional[int]{Val: 1, IsSet: true}, oil.NewOptional(1, true))
	assert.Equal(t, oil.Optional[int]{Val: 0, IsSet: false}, oil.NewOptional(0, false))
	assert.Equal(t, 1, oil.NewOptional(1, true).Get(0))
	assert.Equal(t, 0, oil.NewOptional(1, false).Get(0))
	assert.Equal(t, 1, oil.Optional[int]{}.Get(1))
	var o oil.Optional[int]
	assert.Equal(t, 1, o.SetDefault(1).Get(0))
	assert.Equal(t, 1, o.SetDefault(2).Get(0))
	assert.Equal(t, 2, o.Set(2).Get(0))
	assert.False(t, o.Set(3).Unset().IsSet)
}

func TestMapDefaults(t *testing.T) {
	m := map[int]int{1: 2}
	assert.Equal(t, 5, oil.MapGet(m, 8, 5))
	assert.Equal(t, 2, oil.MapGet(m, 1, 5))
	m2 := oil.MapSetDefault(m, 1, 3)
	assert.Equal(t, map[int]int{1: 2}, m)
	assert.Equal(t, m, m2)
	m2 = oil.MapSetDefault(m, 5, 3)
	assert.Equal(t, map[int]int{1: 2, 5: 3}, m)
	assert.Equal(t, m, m2)
}

func TestMapGetOrNew(t *testing.T) {
	m := map[int]int{1: 2}
	assert.Equal(t, 2, oil.MapGetOrNew(m, 1, func() int { return 3 }))
	assert.Equal(t, 3, oil.MapGetOrNew(m, 4, func() int { return 3 }))
	assert.Equal(t, map[int]int{1: 2, 4: 3}, m)
}

func TestMapGetOrNewRef(t *testing.T) {
	two := 2
	m := map[int]*int{1: &two}
	assert.Equal(t, 2, *oil.MapGetOrNewRef(m, 1))
	assert.Equal(t, 0, *oil.MapGetOrNewRef(m, 4))
	assert.Equal(t, 2, len(m))
	assert.Equal(t, 2, *m[1])
	assert.Equal(t, 0, *m[4])
}
