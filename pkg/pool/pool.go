// Package pool provides a generic, type-safe wrapper around sync.Pool for
// values whose type knows how to reset itself, e.g. via generated Reset()
// methods (see cmd/reset).
package pool

import "sync"

// Resettable is implemented by any type whose state can be zeroed in place,
// making it safe to hand back out of a Pool without leaking previous data.
type Resettable interface {
	Reset()
}

// Pool is a generic, type-safe wrapper around sync.Pool: it only ever hands
// out and accepts values of type T, and Put() resets a value before
// returning it to the underlying pool, so a caller can never observe stale
// state coming out of Get().
type Pool[T Resettable] struct {
	pool sync.Pool
}

// New creates a Pool whose Get() calls newFunc to produce a new value
// whenever the pool has none to reuse. A nil newFunc is allowed, mirroring
// sync.Pool itself: Get() then returns the zero value of T instead of
// panicking.
func New[T Resettable](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				if newFunc == nil {
					var zero T
					return zero
				}
				return newFunc()
			},
		},
	}
}

// Get returns a value from the pool, creating one via the Pool's newFunc if
// the pool is currently empty.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put resets v and returns it to the pool for later reuse.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}
