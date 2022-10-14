package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const tolerance = time.Second

func TestRealClockBasicFunctions(t *testing.T) {
	t.Parallel()
	t0 := Real.Now()
	d := time.Now().Sub(t0)
	assert.Less(t, d, tolerance)
	assert.Greater(t, d, -tolerance)
	const sleep = tolerance / 10
	Real.Sleep(sleep)
	t1 := Real.Now()
	d = t1.Sub(t0)
	assert.Greater(t, d, sleep/2)
	assert.Less(t, d, sleep+tolerance)
	now := time.Now()

	d = Real.Since(now)
	assert.GreaterOrEqual(t, d, time.Duration(0))
	assert.Less(t, d, tolerance)
	d = Real.Since(now.Add(-time.Hour))
	assert.GreaterOrEqual(t, d, time.Hour)
	assert.Less(t, d, time.Hour+tolerance)

	d = Real.Until(now.Add(tolerance))
	assert.Greater(t, d, time.Duration(0))
	assert.LessOrEqual(t, d, tolerance)
	d = Real.Until(now.Add(time.Hour + tolerance))
	assert.Greater(t, d, time.Hour)
	assert.LessOrEqual(t, d, time.Hour+tolerance)
}

func TestControllerWithNoOptions(t *testing.T) {
	c := NewController(nil)
	now := time.Now()
	assert.GreaterOrEqual(t, now.Sub(c.Now()), time.Duration(0))
	assert.Less(t, now.Sub(c.Now()), time.Duration(tolerance))
}

func TestControllerBasicFunctions(t *testing.T) {
	t.Parallel()
	now := MustParseInLocation("2006-01-02 15:04", "2022-10-14 10:30", MustLoadLocation("Europe/Madrid"))
	c := NewController(&ControllerOpts{InitialTime: now})
	cNow := c.Now()
	assert.True(t, cNow.Equal(now))
	assert.Equal(t, now.UnixNano(), cNow.UnixNano())
	assert.Equal(t, now.Location(), cNow.Location())
	assert.Equal(t, time.Duration(0), c.Since(now))
	assert.Equal(t, time.Hour, c.Since(now.Add(-time.Hour)))
	assert.Equal(t, time.Duration(0), c.Until(now))
	assert.Equal(t, time.Hour, c.Until(now.Add(time.Hour)))
	c.Sleep(time.Hour)
	assert.Equal(t, time.Hour, c.Now().Sub(now))
}

func TestMustFunctions(t *testing.T) {
	t.Parallel()
	chicago := MustLoadLocation("America/Chicago")
	assert.Equal(t, "America/Chicago", chicago.String())
}
