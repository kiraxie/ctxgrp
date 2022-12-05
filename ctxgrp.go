package ctxgrp

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type ContextFunc func(ctx context.Context) error

type Group struct {
	ctx     context.Context
	cancel  func()
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
	done    atomic.Value
}

func (t *Group) Deadline() (time.Time, bool) {
	return t.ctx.Deadline()
}

func (t *Group) Err() error {
	return t.err
}

func (t *Group) Value(key any) any {
	return t.ctx.Value(key)
}

func (t *Group) Go(f ContextFunc) {
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()

		if err := f(t.ctx); err != nil {
			t.CancelError(err)
		}
	}()
}

func (t *Group) GoCancel(f ContextFunc) context.CancelFunc {
	t.wg.Add(1)
	ctx, cancel := context.WithCancel(t.ctx)

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.CancelError(err)
		}
	}()

	return cancel
}

func (t *Group) GoTimeout(timeout time.Duration, f ContextFunc) context.CancelFunc {
	t.wg.Add(1)
	ctx, cancel := context.WithTimeout(t.ctx, timeout)

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.CancelError(err)
		}
	}()

	return cancel
}

func (t *Group) Fork(f func(ctx context.Context) error) {
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()

		if err := f(context.Background()); err != nil {
			t.CancelError(err)
		}
	}()
}

// ForkTimeout fork with cancel
func (t *Group) ForkCancel(f func(ctx context.Context) error) context.CancelFunc {
	t.wg.Add(1)

	ctx, cancel := defaultContextWithCancel()

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.CancelError(err)
		}
	}()

	return cancel
}

// ForkTimeout fork with timeout
func (t *Group) ForkTimeout(timeout time.Duration, f func(ctx context.Context) error) context.CancelFunc {
	t.wg.Add(1)

	ctx, cancel := defaultContextWithTimeout(timeout)

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.CancelError(err)
		}
	}()

	return cancel
}

func (t *Group) Context() context.Context {
	return t.ctx
}

func (t *Group) Done() <-chan struct{} {
	v := t.done.Load()
	if v != nil {
		return v.(chan struct{})
	}
	ch := make(chan struct{})
	if !t.done.CompareAndSwap(nil, ch) {
		return t.done.Load().(chan struct{})
	}
	go func() {
		t.wg.Wait()
		t.cancel()
		ch <- struct{}{}
	}()

	return ch
}

func (t *Group) Wait() error {
	t.wg.Wait()
	t.cancel()

	return t.err
}

func (t *Group) Cancel() {
	t.cancel()
}

func (t *Group) CancelError(err error) {
	t.errOnce.Do(func() {
		t.err = err
		t.cancel()
	})
}

func (t *Group) Close() (err error) {
	t.cancel()
	t.wg.Wait()

	return t.err
}

func New(ctx context.Context) (t *Group) {
	t = &Group{}
	t.ctx, t.cancel = context.WithCancel(ctx)

	return
}

func defaultContextWithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func defaultContextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func SleepWithContext(ctx context.Context, d time.Duration) (done bool) {
	if d < 1 {
		return false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}
