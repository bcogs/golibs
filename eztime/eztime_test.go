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
