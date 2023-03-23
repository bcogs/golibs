package eztime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestCancellableSleep(t *testing.T) {
	t0 := time.Now()
	CancellableSleep(-time.Hour, make(chan struct{}))
	t1 := time.Now()
	assert.Less(t, t1.Sub(t0), time.Second)
	CancellableSleep(time.Second/10, make(chan struct{}))
	t2 := time.Now()
	assert.Less(t, t2.Sub(t1), time.Second)
	assert.Greater(t, t2.Sub(t1), time.Second/20)
	c := make(chan int, 1)
	c <- 3
	assert.Equal(t, 3, CancellableSleep(time.Hour, c))
	assert.Less(t, time.Now().Sub(t2), time.Second)
}
