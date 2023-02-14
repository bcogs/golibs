// Package nummap provides maps of arbitrary keys to numeric values that can be
// accessed concurrently and read-edit-written atomically.
package nummap

import (
	"sync"

	"github.com/bcogs/golibs/oil"
)

// NumMap maps any type of key to any type of number and allows to manipulate
// those numbers in a concurrency safe fashion.
type NumMap[K comparable, V oil.Number] struct {
	mu sync.Mutex // PROTECTS EVERYTHING BELOW
	m  map[K]V
}

// NewNumMap creates a NumMap.
func NewNumMap[K comparable, V oil.Number]() *NumMap[K, V] { return &NumMap[K, V]{m: make(map[K]V)} }

// Add adds a value to an entry of the map and returns the result.
func (cm *NumMap[K, V]) Add(key K, value V) V {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	v := cm.m[key]
	v += value
	cm.m[key] = v
	return v
}

// Apply applies an arbitrary function to an entry of the map and returns the result and the initial value.
func (cm *NumMap[K, V]) Apply(key K, f func(v V) V) (before, after V) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	before = cm.m[key]
	after = f(before)
	cm.m[key] = after
	return
}

// Delete deletes an entry from the NumMap.
func (cm *NumMap[K, V]) Delete(key K) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.m, key)
}

// Get reads an entry of the map.
func (cm *NumMap[K, V]) Get(k K) V {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.m[k]
}

// Len returns the NumMap len.
func (cm *NumMap[K, V]) Len() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.m)
}

// Set sets an entry of the map to a value.
func (cm *NumMap[K, V]) Set(k K, v V) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.m[k] = v
}

// Snapshot returns a snapshot copy of the map.
func (cm *NumMap[K, V]) Snapshot() map[K]V {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	m := make(map[K]V, len(cm.m))
	for k, v := range cm.m {
		m[k] = v
	}
	return m
}

// Sub subtracts a value from an entry of the map and returns the result.
func (cm *NumMap[K, V]) Sub(key K, value V) V {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	v := cm.m[key]
	v -= value
	cm.m[key] = v
	return v
}
