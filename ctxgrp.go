package ctxgrp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrResetTimerFailed = errors.New("reset timer failed")

type ContextFunc func(ctx context.Context) error

type group struct {
	ctx      context.Context
	cancel   func()
	wg       sync.WaitGroup
	errOnce  sync.Once
	err      error
	done     atomic.Value
	listener []chan func(Listener)
	state    State
}

// -----------------------
// |       Context       |
// -----------------------

func (t *group) Deadline() (time.Time, bool) {
	return t.ctx.Deadline()
}

func (t *group) Done() <-chan struct{} {
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

func (t *group) Err() error {
	return t.err
}

func (t *group) Value(key any) any {
	return t.ctx.Value(key)
}

// ------------------------------
// |       Service method       |
// ------------------------------

func (t *group) Loop(f ContextFunc) {
	t.increase()

	go func() {
		defer t.wg.Done()

		for {
			select {
			case <-t.ctx.Done():
				return
			default:
				if err := f(t.ctx); err != nil {
					t.StopError(err)
				}
			}
		}
	}()
}

func (t *group) TimerLoop(d time.Duration, f ContextFunc) {
	t.increase()

	go func() {
		defer t.wg.Done()
		tm := time.NewTimer(0)
		defer tm.Stop()

		for {
			select {
			case <-t.ctx.Done():
				return
			case <-tm.C:
				if err := f(t.ctx); err != nil {
					t.StopError(err)
				}
				if tm.Reset(d) {
					t.StopError(ErrResetTimerFailed)
				}
			}
		}
	}()
}

func (t *group) Go(f ContextFunc) {
	t.increase()

	go func() {
		defer t.wg.Done()

		if err := f(t.ctx); err != nil {
			t.StopError(err)
		}
	}()
}

func (t *group) GoCancel(f ContextFunc) context.CancelFunc {
	t.increase()
	ctx, cancel := context.WithCancel(t.ctx)

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.StopError(err)
		}
	}()

	return cancel
}

func (t *group) GoTimeout(timeout time.Duration, f ContextFunc) context.CancelFunc {
	t.increase()
	ctx, cancel := context.WithTimeout(t.ctx, timeout)

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.StopError(err)
		}
	}()

	return cancel
}

func (t *group) Fork(f ContextFunc) {
	t.increase()

	go func() {
		defer t.wg.Done()

		if err := f(context.Background()); err != nil {
			t.StopError(err)
		}
	}()
}

func (t *group) ForkCancel(f ContextFunc) context.CancelFunc {
	t.increase()

	ctx, cancel := defaultContextWithCancel()

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.StopError(err)
		}
	}()

	return cancel
}

func (t *group) ForkTimeout(timeout time.Duration, f ContextFunc) context.CancelFunc {
	t.increase()

	ctx, cancel := defaultContextWithTimeout(timeout)

	go func() {
		defer t.wg.Done()

		if err := f(ctx); err != nil {
			t.StopError(err)
		}
	}()

	return cancel
}

func (t *group) Wait() error {
	if atomic.CompareAndSwapInt32((*int32)(&t.state), int32(StateNew), int32(StateStopping)) ||
		atomic.CompareAndSwapInt32((*int32)(&t.state), int32(StateRunning), int32(StateStopping)) ||
		atomic.CompareAndSwapInt32((*int32)(&t.state), int32(StateTerminated), int32(StateStopping)) {
		t.notifyListener(func(l Listener) {
			l.Stopping()
		}, true)
	}
	t.wg.Wait()
	t.cancel()

	return t.err
}

func (t *group) Stop() {
	if atomic.CompareAndSwapInt32((*int32)(&t.state), int32(StateNew), int32(StateTerminated)) ||
		atomic.CompareAndSwapInt32((*int32)(&t.state), int32(StateRunning), int32(StateTerminated)) {
		t.notifyListener(func(l Listener) {
			l.Terminated()
		}, false)
	}
	t.cancel()
}

func (t *group) StopError(err error) {
	t.errOnce.Do(func() {
		t.err = err
		t.cancel()
		atomic.StoreInt32((*int32)(&t.state), int32(StateFailed))
		t.notifyListener(func(l Listener) {
			l.Failed(err)
		}, true)
	})
}

func (t *group) Close() (err error) {
	atomic.StoreInt32((*int32)(&t.state), int32(StateStopping))
	t.notifyListener(func(l Listener) {
		l.Stopping()
	}, true)
	t.cancel()
	t.wg.Wait()
	if t.err != nil {
		return t.err
	}
	return nil
}

func (t *group) State() State {
	return t.state
}

func (t *group) AddListener(l Listener) {
	if atomic.LoadInt32((*int32)(&t.state)) > int32(StateRunning) {
		return
	}

	ch := make(chan func(Listener), 4)
	t.listener = append(t.listener, ch)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for fn := range ch {
			fn(l)
		}
	}()
}

func (t *group) increase() {
	t.wg.Add(1)
	if atomic.CompareAndSwapInt32((*int32)(&t.state), int32(StateNew), int32(StateRunning)) {
		t.notifyListener(func(l Listener) {
			l.Running()
		}, false)
	}
}

func (t *group) notifyListener(fn func(l Listener), closeChan bool) {
	for _, ch := range t.listener {
		ch <- fn
		if closeChan {
			close(ch)
		}
	}
}

func New(ctx context.Context) Group {
	t := &group{
		state: StateNew,
	}
	t.ctx, t.cancel = context.WithCancel(ctx)

	return t
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
