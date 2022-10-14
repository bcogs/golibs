package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var madrid = MustLoadLocation("Europe/Madrid")

func TestMustFunctions(t *testing.T) {
	t.Parallel()
	chicago := MustLoadLocation("America/Chicago")
	assert.Equal(t, "America/Chicago", chicago.String())
	assert.Panics(t, func() { MustLoadLocation("noexists") })

	const layout = "2006-01-02 15:04"
	const prettyTime = "2022-10-14 14:30"
	t0 := MustParse(layout, prettyTime)
	assert.Equal(t, prettyTime, t0.Format(layout))
	assert.Panics(t, func() { MustParse(layout, "invalid") })
	assert.Panics(t, func() { MustParse("invalid", prettyTime) })
	t0 = MustParseInLocation(layout, prettyTime, chicago)
	assert.Equal(t, prettyTime, t0.Format(layout))
	assert.Panics(t, func() { MustParseInLocation(layout, "invalid", chicago) })
	assert.Panics(t, func() { MustParseInLocation("invalid", prettyTime, chicago) })

	d := MustParseDuration("1h5m")
	assert.Equal(t, time.Hour+5*time.Minute, d)
	assert.Panics(t, func() { MustParseDuration("invalid") })
}

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

func TestRealClockTimers(t *testing.T) {
	t.Parallel()
	t0 := time.Now()
	timer := Real.NewTimer(time.Second)
	<-Real.After(time.Second)
	<-timer.C
	d := time.Now().Sub(t0)
	assert.Greater(t, d, time.Second/2)
	assert.Less(t, d, time.Second+tolerance)
}

func TestControllerWithNoOptions(t *testing.T) {
	t.Parallel()
	c := NewController(nil)
	now := time.Now()
	assert.GreaterOrEqual(t, now.Sub(c.Now()), time.Duration(0))
	assert.Less(t, now.Sub(c.Now()), time.Duration(tolerance))
}

func TestControllerBasicFunctions(t *testing.T) {
	t.Parallel()
	now := MustParseInLocation("2006-01-02 15:04", "2022-10-14 10:30", madrid)
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

func TestControllerTimers(t *testing.T) {
	t.Parallel()
	const layout = "2006-01-02 15:04"
	t0 := MustParseInLocation(layout, "2022-10-14 10:30", madrid)
	c := NewController(&ControllerOpts{InitialTime: t0})
	timer0, after0 := c.NewTimer(-time.Hour), c.After(-time.Hour)
	timer1, after1 := c.NewTimer(time.Hour), c.After(time.Hour)
	assert.Equal(t, t0.Format(layout), (<-timer0.C).Format(layout))
	assert.Equal(t, t0.Format(layout), (<-after0).Format(layout))
	c.Sleep(59 * time.Minute)
	select {
	case <-timer1.C:
		t.Error("timer1 shouldn't trigger")
	case <-after1:
		t.Error("after1 shouldn't trigger")
	case <-time.After(time.Second / 10): // nothing
	}
	go c.Sleep(time.Minute)
	t1 := t0.Add(time.Hour)
	assert.Eventually(t, func() bool { return t1.Equal(c.Now()) }, 3*time.Second, time.Second/10)
	assert.Equal(t, t1.Format(layout), (<-after1).Format(layout))
	assert.Equal(t, t1.Format(layout), (<-timer1.C).Format(layout))
}
