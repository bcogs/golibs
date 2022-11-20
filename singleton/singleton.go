// Package singleton implements singletons and maps of singletons.
//
// To create a singleton of type foo:
// var myfoo singleton.Singleton
// theOnlyFoo := myfoo.GetOrCreate(func() interface{} { return new(foo) }).(foo)
// // have fun with theOnlyFoo
//
// If you have rather a map of keys of type K to singletons of type V:
// var mybars singleton.SingletonMap
// key := some key of type K
// val := mybars.GetOrCreate(key,
//
//	func(k interface{} /* it's a K */) interface{} { return new(bar) }).(bar)
//
// // have fun with val, it's the one bar for the key
package singleton

import (
	"sync"
)

// Singleton is a singleton that can be used concurrently.
// It mustn't be copied after being used.
type Singleton struct {
	mu       sync.RWMutex
	instance interface{}
}

// GetOrCreate returns the singleton as an interface{}.
// It takes in argument a creation function that, if called, creates the singleton.
// The creation function will be called only once for a given Singleton
// instance, and all calls to GetOrCreate for that Singleton will return
// whatever that unique call to the creation function returned.
func (s *Singleton) GetOrCreate(create func() interface{}) interface{} {
	s.mu.RLock()
	result := s.instance
	s.mu.RUnlock()
	if result == nil {
		s.mu.Lock()
		result = s.instance
		if result == nil { // we need to test again, it might have been set in the mean time
			result = create()
			s.instance = result
		}
		s.mu.Unlock()
	}
	return result
}

// SingletonMap is a map of singletons that can be used concurrently.
// It mustn't be copied after being used.
type SingletonMap struct {
	mu        sync.RWMutex
	instances map[interface{}]interface{}
}

// GetOrCreate returns the singleton for a key as an interface{}.
// It takes in argument a key and a creation function that, if called, creates the singleton for that key.
// The creation function will be called only once for any given SingletonMap
// instance and key combination, and all calls to GetOrCreate for that
// SingletonMap and key will return whatever that unique call to the creation
// function returned.
// The creation function receives in argument the same key passed to GetOrCreate.
func (sm *SingletonMap) GetOrCreate(key interface{}, create func(key interface{}) interface{}) interface{} {
	sm.mu.Lock()
	result := sm.instances[key]
	sm.mu.Unlock()
	if result == nil {
		sm.mu.Lock()
		result = sm.instances[key]
		if result == nil { // we need to test again, it might have been set in the mean time
			result = create(key)
			if sm.instances == nil {
				sm.instances = make(map[interface{}]interface{})
			}
			sm.instances[key] = result
		}
		sm.mu.Unlock()
	}
	return result
}
