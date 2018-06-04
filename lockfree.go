package lock

import (
	"runtime"
	"sync/atomic"
)

// lockfree simple lock free
type lockfree struct {
	lk int32
}

func (c *lockfree) Lock() {
	for {
		if atomic.CompareAndSwapInt32(&c.lk, 0, 1) {
			return
		}
		runtime.Gosched()
	}
}

func (c *lockfree) Unlock() {
	atomic.StoreInt32(&c.lk, 0)
}
