package ctxgrp

type listener struct {
	runningFn    func()
	terminatedFn func()
	stoppingFn   func()
	failedFn     func(failure error)
}

func (f listener) Running() {
	if f.runningFn != nil {
		f.runningFn()
	}
}

func (f listener) Terminated() {
	if f.terminatedFn != nil {
		f.terminatedFn()
	}
}

func (f listener) Stopping() {
	if f.stoppingFn != nil {
		f.stoppingFn()
	}
}

func (f listener) Failed(failure error) {
	if f.failedFn != nil {
		f.failedFn(failure)
	}
}

func NewListener(running, stopping, terminated func(), failed func(error)) Listener {
	return &listener{
		runningFn:    running,
		terminatedFn: terminated,
		stoppingFn:   stopping,
		failedFn:     failed,
	}
}
