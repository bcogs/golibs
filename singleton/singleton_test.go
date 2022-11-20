package singleton_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bcogs/golibs/singleton"
)

// createlog provides the create functions that GetOrCreate wants in argument,
// and keeps a log of the calls to those functions
type createlog struct{ created chan int }

func newCreatelog(size int) *createlog { return &createlog{created: make(chan int, size)} }

// create is meant to be passed in argument to Singleton.GetOrCreate
func (c *createlog) create() interface{} {
	c.created <- -1
	return -1
}

// createWithKey is meant to be passed in argument to SingletonMap.GetOrCreate
func (c *createlog) createWithKey(key interface{}) interface{} {
	keyString := key.(int)
	c.created <- keyString
	return keyString
}

// all returns the log of calls to create / createWithKey:
//   - each call to create is represented by a -1
//   - each call to createWithKey is represented by the key
func (c *createlog) all() []int {
	result := make([]int, 0)
	for {
		select {
		case s := <-c.created:
			result = append(result, s)
		default:
			return result
		}
	}
}

func TestSingletonBasics(t *testing.T) {
	t.Parallel()
	var s singleton.Singleton
	createlog := newCreatelog(100)
	assert.Equal(t, -1, s.GetOrCreate(createlog.create).(int))
	assert.Equal(t, -1, s.GetOrCreate(createlog.create).(int))
	assert.Equal(t, -1, s.GetOrCreate(createlog.create).(int))
	assert.Equal(t, createlog.all(), []int{-1})
}

func TestSingletonRaces(t *testing.T) {
	t.Parallel()
	var s singleton.Singleton
	const P = 100
	const Q = 100
	createlog := newCreatelog(P * Q * 2)
	leash := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(P * Q)
	for i := 0; i < P; i++ {
		for j := 0; j < Q; j++ {
			go func(i int) {
				<-leash
				assert.Equal(t, -1, s.GetOrCreate(createlog.create).(int))
				wg.Done()
			}(i)
		}
	}
	close(leash)
	wg.Wait()
	assert.Equal(t, []int{-1}, createlog.all())
}

func TestSingletonMapBasics(t *testing.T) {
	t.Parallel()
	var sm singleton.SingletonMap
	createlog := newCreatelog(100)
	assert.Equal(t, 1, sm.GetOrCreate(1, createlog.createWithKey).(int))
	assert.Equal(t, 2, sm.GetOrCreate(2, createlog.createWithKey).(int))
	assert.Equal(t, 1, sm.GetOrCreate(1, createlog.createWithKey).(int))
	assert.Equal(t, createlog.all(), []int{1, 2})
}

func TestSingletonMapRaces(t *testing.T) {
	t.Parallel()
	var sm singleton.SingletonMap
	const P = 100
	const Q = 100
	createlog := newCreatelog(P * Q * 2)
	expected := make([]int, 0)
	leash := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(P * Q)
	for i := 0; i < P; i++ {
		expected = append(expected, i)
		for j := 0; j < Q; j++ {
			go func(i int) {
				<-leash
				assert.Equal(t, i, sm.GetOrCreate(i, createlog.createWithKey).(int))
				wg.Done()
			}(i)
		}
	}
	close(leash)
	wg.Wait()
	assert.ElementsMatch(t, expected, createlog.all())
}
