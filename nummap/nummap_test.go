package nummap

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func do(wg *sync.WaitGroup, f func(k, v int) int, k, v int) {
	wg.Add(1)
	go func() { f(k, v); wg.Done() }()
}

func TestNummap(t *testing.T) {
	m := NewNumMap[int, int]()
	var wg sync.WaitGroup
	expected := 0
	for i := 0; i < 10; i++ {
		expected += 1
		expected -= 2
		expected += 3
	}
	for i := 0; i < 100; i++ {
		do(&wg, m.Add, i/10, 1)
		do(&wg, m.Sub, i/10, 2)
		do(&wg, func(k, v int) int {
			before, after := m.Apply(k, func(v int) int { return 3 + v })
			assert.Equal(t, before+3, after)
			return 0 // irrelevant
		}, i/10, 0)
		go m.Get(i / 10)
		go m.Len()
	}
	wg.Wait()
	snapshot := m.Snapshot()
	assert.Equal(t, len(snapshot), 10)
	wg.Add(10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, expected, m.Get(i))
		assert.Equal(t, expected, snapshot[i])
		go func(i int) { m.Set(i/2, i/2); wg.Done(); assert.Equal(t, 10, m.Len()) }(i)
	}
	wg.Wait()
	for i := 0; i < 5; i++ {
		assert.Equal(t, i, m.Get(i))
	}
	wg.Add(35)
	for i := 0; i < 35; i++ {
		go func(i int) { m.Delete(i / 3); assert.Equal(t, 0, m.Get(i/3)); wg.Done() }(i)
	}
	wg.Wait()
	assert.Equal(t, 0, m.Len())
	// check that Snapshot isn't racy
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			wg.Add(1)
			snapshot := m.Snapshot()
			go func(i int) {
				for k, v := range snapshot {
					assert.Less(t, k, i)
					assert.Equal(t, k, v)
				}
				wg.Done()
			}(i)
		}
		go m.Set(i, i)
	}
	wg.Wait()
}
