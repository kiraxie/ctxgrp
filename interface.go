package ctxgrp

import (
	"context"
	"fmt"
	"time"
)

type State int32

func (t State) String() string {
	switch t {
	case StateNew:
		return "New"
	case StateRunning:
		return "Running"
	case StateTerminated:
		return "Terminated"
	case StateStopping:
		return "Stopping"
	case StateFailed:
		return "Failed"
	default:
	}

	return fmt.Sprintf("Unknown state: %d", t)
}

const (
	StateNew State = iota
	StateRunning
	StateTerminated
	StateStopping
	StateFailed
)

type Group interface {
	context.Context

	// Run the function forever until context done.
	Loop(f ContextFunc)

	// Run function every `d` duration until context done.
	TimerLoop(d time.Duration, f ContextFunc)

	// Run a goroutine with context.
	Go(f ContextFunc)

	// Run a goroutine with cancel function, the execute function stop when cancel execute or context done.
	// Note that the cancel function only work for this execute function.
	GoCancel(f ContextFunc) context.CancelFunc

	// Run a goroutine with timeout.
	GoTimeout(timeout time.Duration, f ContextFunc) context.CancelFunc

	// Run a goroutine with new independent context.
	Fork(f ContextFunc)

	// Run a goroutine with new independent context, stop when cancel function execute.
	ForkCancel(f ContextFunc) context.CancelFunc

	// Run a goroutine with new independent context and timeout, stop when cancel function execute or timeout.
	ForkTimeout(timeout time.Duration, f ContextFunc) context.CancelFunc

	// Cancel the root context of this service. This method doesn't block and can be called multiple times.
	Stop()

	// Cancel the root context of this service with specific error. The return value of `Wait` will be this specific
	// error what here gave. This method doesn't block and can be called multiple times, but the error only assign once.
	StopError(error)

	// Waits all internal goroutine done. This method blocks until all the goroutine exit.
	Wait() error

	// This function call Stop() and Wait().
	Close() error

	// State returns current state of the service.
	State() State

	// Adds listener to this service. Listener will be notify when state changed of this service.
	AddListener(l Listener)
}

// Listener receives notifications about Service state changes.
type Listener interface {
	// Running is called when the service transitions from NEW to RUNNING.
	Running()

	// Terminated is called when the service transitions to the TERMINATED state.
	Terminated()

	// Stopping is called when the service transitions to the STOPPING state.
	Stopping()

	// Failed is called when the service transitions to the FAILED state.
	Failed(failure error)
}
