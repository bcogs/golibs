// Package singleton implements singletons and maps of singletons.
//
// To create a singleton of type T:
//   var myfoo singleton.Singleton[T]
//   theOnlyFoo := myfoo.GetOrCreate(func() T { return T{} })
//   // have fun with theOnlyFoo
//
// If you have rather a map of keys of type K to singletons of type V:
//   var mybars singleton.SingletonMap[K, V]
//   key := some key of type K
//   val := mybars.GetOrCreate(key, func(k K) V { return V{} })
//   // have fun with val, it's the one V for the key
package singleton

import (
	"sync"
)

// Singleton is a singleton that can be used concurrently.
// It mustn't be copied after being used.
type Singleton[T any] struct {
	mu       sync.RWMutex
	created  bool
	instance T
}

// GetOrCreate returns the singleton as an interface{}.
// It takes in argument a creation function that, if called, creates the singleton.
// The creation function will be called only once for a given Singleton
// instance, and all calls to GetOrCreate for that Singleton will return
// whatever that unique call to the creation function returned.
func (s *Singleton[T]) GetOrCreate(create func() T) T {
	s.mu.RLock()
	if s.created {
		defer s.mu.RUnlock()
		return s.instance
	}
	s.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.created { // we need to test again, it might have been set in the mean time
		return s.instance
	}
	result := create()
	s.instance, s.created = result, true
	return result
}

// SingletonMap is a map of singletons that can be used concurrently.
// It mustn't be copied after being used.
type SingletonMap[K comparable, V any] struct {
	mu        sync.RWMutex
	instances map[K]V
}

// GetOrCreate returns the singleton for a key as an interface{}.
// It takes in argument a key and a creation function that, if called, creates the singleton for that key.
// The creation function will be called only once for any given SingletonMap
// instance and key combination, and all calls to GetOrCreate for that
// SingletonMap and key will return whatever that unique call to the creation
// function returned.
// The creation function receives in argument the same key passed to GetOrCreate.
func (sm *SingletonMap[K, V]) GetOrCreate(key K, create func(key K) V) V {
	sm.mu.Lock()
	result, ok := sm.instances[key]
	sm.mu.Unlock()
	if !ok {
		sm.mu.Lock()
		result, ok = sm.instances[key]
		if !ok { // we need to test again, it might have been set in the mean time
			result = create(key)
			if sm.instances == nil {
				sm.instances = make(map[K]V)
			}
			sm.instances[key] = result
		}
		sm.mu.Unlock()
	}
	return result
}

// GetOrCreateOrFail is the same as GetOrCreate but allows the creation to fail.
func (sm *SingletonMap[K, V]) GetOrCreateOrFail(key K, create func(key K) (V, error)) (V, error) {
	sm.mu.Lock()
	result, ok := sm.instances[key]
	sm.mu.Unlock()
	if !ok {
		var err error
		sm.mu.Lock()
		defer sm.mu.Unlock()
		result, ok = sm.instances[key]
		if !ok { // we need to test again, it might have been set in the mean time
			result, err = create(key)
			if err != nil {
				return result, err
			}
			if sm.instances == nil {
				sm.instances = make(map[K]V)
			}
			sm.instances[key] = result
		}
	}
	return result, nil
}
