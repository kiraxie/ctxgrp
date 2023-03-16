package ctxgrp_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kiraxie/ctxgrp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroup(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	assert.NotNil(assert)
	require := require.New(t)
	require.NotNil(require)

	var count1 int64
	var count2 int64
	grp1 := ctxgrp.New(context.Background())
	defer func() {
		require.NoError(grp1.Close())
	}()
	grp2 := ctxgrp.New(context.Background())
	defer func() {
		require.ErrorIs(context.DeadlineExceeded, grp2.Close())
	}()

	grp1.Go(func(ctx context.Context) error {
		time.Sleep(400 * time.Millisecond)
		atomic.AddInt64(&count1, 1)
		grp1.Cancel()

		return nil
	})
	grp1.Go(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
			atomic.AddInt64(&count1, 1)
		}

		return nil
	})

	grp2.Go(func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		atomic.AddInt64(&count2, 1)

		return context.DeadlineExceeded
	})
	grp2.Go(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
			atomic.AddInt64(&count2, 1)
		}

		return nil
	})

	var firstClose string
	select {
	case <-grp1.Done():
		firstClose = "grp1"
	case <-grp2.Done():
		firstClose = "grp2"
	}

	assert.EqualValues(0, atomic.LoadInt64(&count1))
	assert.EqualValues(1, atomic.LoadInt64(&count2))
	assert.EqualValues("grp2", firstClose)
}

func TestGroupDone(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	assert.NotNil(assert)
	require := require.New(t)
	require.NotNil(require)

	var count1 int64
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	grp1 := ctxgrp.New(ctx)
	defer func() {
		require.NoError(grp1.Close())
	}()

	grp1.Go(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt64(&count1, 1)

		return nil
	})

	var path string
	select {
	case <-ctx.Done():
		path = "ctx"
	case <-grp1.Done():
		path = "grp1"
	}

	assert.EqualValues(1, atomic.LoadInt64(&count1))
	assert.EqualValues("grp1", path)
	assert.NoError(grp1.Wait())
}

func TestGroupFork(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	assert.NotNil(assert)
	require := require.New(t)
	require.NotNil(require)

	const step = 300 * time.Millisecond
	var timeGo1 time.Time
	var timeGo2 time.Time
	var timeGo3 time.Time
	var timeGo4 time.Time
	var timeFork1 time.Time
	var timeFork2 time.Time
	var timeFork3 time.Time
	var timeFork4 time.Time
	ctx, cancel := context.WithTimeout(context.Background(), 3*step)
	defer cancel()
	grp := ctxgrp.New(ctx)
	defer func() {
		require.NoError(grp.Close())
	}()
	start := time.Now()

	grp.Go(func(ctx context.Context) error {
		ctxgrp.SleepWithContext(ctx, 1*step)
		timeGo1 = time.Now()

		return nil
	})
	grp.Go(func(ctx context.Context) error {
		ctxgrp.SleepWithContext(ctx, 5*step)
		timeGo2 = time.Now()

		return nil
	})
	cancelGo3 := grp.GoCancel(func(ctx context.Context) error {
		ctxgrp.SleepWithContext(ctx, 2*step)
		timeGo3 = time.Now()

		return nil
	})
	cancelGo3()
	grp.GoTimeout(1*step, func(ctx context.Context) error {
		ctxgrp.SleepWithContext(ctx, 5*step)
		timeGo4 = time.Now()

		return nil
	})

	grp.Fork(func(ctx context.Context) error {
		ctxgrp.SleepWithContext(ctx, 7*step)
		timeFork1 = time.Now()

		return nil
	})
	cancelFork2 := grp.ForkCancel(func(ctx context.Context) error {
		<-ctx.Done()
		timeFork2 = time.Now()

		return nil
	})
	time.AfterFunc(9*step, cancelFork2)
	grp.ForkTimeout(1*step, func(ctx context.Context) error {
		<-ctx.Done()
		timeFork3 = time.Now()

		return nil
	})
	grp.ForkTimeout(5*step, func(ctx context.Context) error {
		<-ctx.Done()
		timeFork4 = time.Now()

		return nil
	})

	assert.NoError(grp.Wait())
	const delta = step >> 1
	assert.InDelta(1*step, timeGo1.Sub(start), float64(delta))
	assert.InDelta(3*step, timeGo2.Sub(start), float64(delta))
	assert.InDelta(0*step, timeGo3.Sub(start), float64(delta))
	assert.InDelta(1*step, timeGo4.Sub(start), float64(delta))
	assert.InDelta(7*step, timeFork1.Sub(start), float64(delta))
	assert.InDelta(9*step, timeFork2.Sub(start), float64(delta))
	assert.InDelta(1*step, timeFork3.Sub(start), float64(delta))
	assert.InDelta(5*step, timeFork4.Sub(start), float64(delta))
	assert.InDelta(9*step, time.Since(start), float64(delta))
}
