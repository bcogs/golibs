package clock

import (
	"time"
)

type watcher struct {
	C         chan time.Time
	Threshold time.Time
}

type watcherHeap []watcher

func (w watcherHeap) Len() int { return len(w) }

func (w watcherHeap) Less(i, j int) bool { return w[i].Threshold.Before(w[j].Threshold) }

func (w watcherHeap) Swap(i, j int) { w[i], w[j] = w[j], w[i] }

func (w *watcherHeap) Push(x interface{}) { *w = append(*w, x.(watcher)) }

func (w *watcherHeap) Pop() interface{} {
	result := (*w)[len(*w)-1]
	*w = (*w)[:len(*w)-1]
	return result
}

func (w watcherHeap) First() watcher { return w[0] }
