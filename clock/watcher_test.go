package clock

import (
	"container/heap"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWatcherHeapPushPop(t *testing.T) {
	// No need to test much here, as the heap package does the job, but a
	// basic test allows to verify our implementation of heap.Interface.
	t.Parallel()
	t0 := time.Date(2022, time.Month(2), 1, 15, 0, 0, 0, time.Local)
	t1 := t0.Add(time.Hour)
	t2 := t1.Add(time.Hour)
	var wh watcherHeap
	heap.Push(&wh, watcher{Threshold: t1})
	heap.Push(&wh, watcher{Threshold: t2})
	heap.Push(&wh, watcher{Threshold: t0})
	const layout = "15:04"
	w0 := heap.Pop(&wh).(watcher)
	assert.Equal(t, t0.Format(layout), w0.Threshold.Format(layout))
	w1 := heap.Pop(&wh).(watcher)
	assert.Equal(t, t1.Format(layout), w1.Threshold.Format(layout))
	w2 := heap.Pop(&wh).(watcher)
	assert.Equal(t, t2.Format(layout), w2.Threshold.Format(layout))
}

func TestWatcherHeapFirst(t *testing.T) {
	// DON'T RUN THIS TEST IN PARALLEL
	// It's a bit expensive, and the other tests don't have a lot of
	// tolerance for delays, so tun that one separately.
	t0 := time.Date(2022, time.Month(2), 1, 15, 0, 0, 0, time.Local)
	var hours []int
	rand.Seed(123456789) // so that the test is reproducible
	for i := 0; i < 1000; i++ {
		i := rand.Int() % 10000
		hours = append(hours, i)
		if rand.Int()%50 == 0 { // then make sure we have a duplicate entry
			hours = append(hours, i)
		}
	}
	sort.Ints(hours)
	var wh watcherHeap
	for _, h := range hours {
		heap.Push(&wh, watcher{Threshold: t0.Add(time.Hour * time.Duration(h))})
	}
	assert.Equal(t, len(hours), wh.Len())
	for _, h := range hours {
		assert.Equal(t, time.Duration(0), wh.First().Threshold.Sub(t0.Add(time.Hour*time.Duration(h))))
		heap.Pop(&wh)
	}
	assert.Equal(t, 0, len(wh))
}
