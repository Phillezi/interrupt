package manager

import (
	"context"
	"sync"
	"sync/atomic"
)

type graceful struct {
	ctx      context.Context
	mu       sync.Mutex
	cond     *sync.Cond
	counter  atomic.Int32
	done     bool
	doneCh   chan struct{}
	waitOnce sync.Once
}

func newGraceful(ctx context.Context) *graceful {
	g := &graceful{
		ctx:    ctx,
		doneCh: make(chan struct{}),
	}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// Add registers a new active task. If shutdown already started it returns a no-op DoneFunc.
func (g *graceful) Add() DoneFunc {
	g.mu.Lock()
	if g.done {
		g.mu.Unlock()
		return func() {}
	}
	// increment active counter
	g.counter.Add(1)
	g.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			// decrement and signal if zero
			g.mu.Lock()
			if g.counter.Add(-1) == 0 {
				g.cond.Broadcast()
			}
			g.mu.Unlock()
		})
	}
}

// Wait returns a channel that closes when all tasks are done OR ctx is cancelled.
func (g *graceful) Wait() <-chan struct{} {
	g.waitOnce.Do(func() {
		// shutdown helper that is safe to call multiple times
		shutdown := func() {
			g.mu.Lock()
			if g.done {
				g.mu.Unlock()
				return
			}
			g.done = true
			close(g.doneCh)
			// wake any cond waiters so they can exit if necessary
			g.cond.Broadcast()
			g.mu.Unlock()
		}

		// goroutine 1: watch for external cancellation
		go func() {
			<-g.ctx.Done()
			shutdown()
		}()

		// goroutine 2: wait for counter to drop to zero
		go func() {
			g.mu.Lock()
			for g.counter.Load() > 0 && !g.done {
				g.cond.Wait()
			}
			g.mu.Unlock()
			shutdown()
		}()
	})
	return g.doneCh
}

func (g *graceful) Done() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.done
}
