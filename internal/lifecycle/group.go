package lifecycle

import (
	"context"
	"sync"
)

// Group owns cancellable work. Its zero value is ready to use; stopping closes
// admission before waiting, so a new task cannot race with the final Wait.
type Group struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	closed bool
	wg     sync.WaitGroup
}

func (g *Group) contextLocked() context.Context {
	if g.ctx == nil {
		g.ctx, g.cancel = context.WithCancel(context.Background())
	}
	return g.ctx
}

func (g *Group) Context() context.Context {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.contextLocked()
}

func (g *Group) Acquire() (context.Context, func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ctx := g.contextLocked()
	if g.closed {
		return ctx, nil, context.Canceled
	}
	g.wg.Add(1)
	return ctx, g.wg.Done, nil
}

func (g *Group) Go(work func(context.Context)) bool {
	ctx, done, err := g.Acquire()
	if err != nil {
		return false
	}
	go func() {
		defer done()
		work(ctx)
	}()
	return true
}

func (g *Group) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.contextLocked()
	g.closed = true
	g.cancel()
}

func (g *Group) Wait() { g.wg.Wait() }

func (g *Group) Close() {
	g.Stop()
	g.Wait()
}
