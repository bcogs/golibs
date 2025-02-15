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

func TestAtoi(t *testing.T) {
	for _, tc := range []struct {
		a                  string
		min, max, expected int
	}{
		{"10", 0, 100, 10},
		{"10", 0, 10, 10},
		{"5", 5, 10, 5},
		{"-5", -10, 10, -5},
		{"11", 0, 10, -42},
		{"-1", 0, 10, -42},
	} {
		k, err := oil.Atoi(tc.a, "foo", tc.min, tc.max)
		if tc.expected == -42 {
			assert.ErrorContainsf(t, err, "invalid foo", "%+v", tc)
		} else {
			assert.NoErrorf(t, err, "%+v", tc)
			assert.Equalf(t, tc.expected, k, "%+v", tc)
		}
	}
}

func TestAtou(t *testing.T) {
	for _, tc := range []struct {
		a        string
		min, max uint
		expected int
	}{
		{"10", 0, 100, 10},
		{"10", 0, 10, 10},
		{"5", 5, 10, 5},
		{"11", 0, 10, -42},
		{"3", 4, 10, -42},
	} {
		k, err := oil.Atou(tc.a, "foo", tc.min, tc.max)
		if tc.expected == -42 {
			assert.ErrorContainsf(t, err, "invalid foo", "%+v", tc)
		} else {
			assert.NoErrorf(t, err, "%+v", tc)
			assert.Equalf(t, uint(tc.expected), k, "%+v", tc)
		}
	}
}

func TestFirst(t *testing.T) {
	assert.Equal(t, 1, oil.First(1))
	assert.Equal(t, "foo", oil.First("foo", "bar"))
}

func TestSecond(t *testing.T) {
	assert.Equal(t, 2, oil.Second("foo", 2))
	assert.Equal(t, "bar", oil.Second(1, "bar", "baz"))
}

func TestThird(t *testing.T) {
	assert.Equal(t, 3, oil.Third("foo", "bar", 3))
	assert.Equal(t, "bar", oil.Third(1, 2, "bar"))
}

func TestFourth(t *testing.T) {
	assert.Equal(t, 4, oil.Fourth("foo", "bar", "baz", 4))
	assert.Equal(t, "bar", oil.Fourth(1, 2, 3, "bar"))
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
	assert.Equal(t, 2, oil.MapSetDefault(m, 1, 3))
	assert.Equal(t, map[int]int{1: 2}, m)
	assert.Equal(t, 3, oil.MapSetDefault(m, 5, 3))
	assert.Equal(t, map[int]int{1: 2, 5: 3}, m)
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

func TestMapFromSlice(t *testing.T) {
	assert.Equal(t, map[int]float64{1: 5, 3: 5}, oil.MapFromSlice([]int{1, 3}, 5.))
}

func TestFanIn(t *testing.T) {
	const N = 50
	consumer := make(chan int, N)
	producer1, producer2 := make(chan int, 1), make(chan int, 1)
	go oil.FanIn(consumer, producer1, producer2)
	for i := 0; i < N; i++ {
		go func(i int) { oil.If(i%5 > 5/2, producer1, producer2) <- i }(i)
	}
	m := make(map[int]bool)
	for n := 0; n < N; n++ {
		i := <-consumer
		assert.GreaterOrEqual(t, i, 0)
		assert.Less(t, i, N)
		m[i] = true
	}
	assert.Equal(t, N, len(m))
	close(producer1)
	producer2 <- N
	assert.Equal(t, N, <-consumer)
	close(producer2)
	_, ok := <-consumer
	assert.False(t, ok)
}

func TestFanOut(t *testing.T) {
	producer := make(chan int, 1)
	consumer1, consumer2 := make(chan int, 1), make(chan int, 1)
	go oil.FanOut(producer, consumer1, consumer2)
	producer <- 1
	assert.Equal(t, 1, <-consumer1)
	assert.Equal(t, 1, <-consumer2)
	close(producer)
	_, ok := <-consumer1
	assert.False(t, ok)
	_, ok = <-consumer2
	assert.False(t, ok)
}
