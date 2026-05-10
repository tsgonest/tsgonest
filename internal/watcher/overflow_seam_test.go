package watcher

import (
	"syscall"

	"github.com/fsnotify/fsnotify"
)

// This file exposes package-internal seams that exist solely for tests in
// internal/watcher/*_test.go. Nothing here is exported and it must not be
// called from production code.

var (
	errOverflow = fsnotify.ErrEventOverflow
	errENOSPC   = syscall.ENOSPC
)

// recoverHook lets tests force recoverFromOverflow to report failure even
// when the underlying re-walk succeeds. When non-nil, its return value
// replaces the natural recovery outcome.
var recoverHook func() (forceFailure bool)

func (w *Watcher) triggerOverflowForTest() bool {
	recovered := w.recoverFromOverflow(nil)
	if recoverHook != nil && recoverHook() {
		recovered = false
	}

	w.snapMu.Lock()
	if recovered {
		w.overflowFailures = 0
	} else {
		w.overflowFailures++
	}
	failures := w.overflowFailures
	w.snapMu.Unlock()

	if failures >= maxOverflowFailures {
		w.snapMu.Lock()
		w.fallbackToPolling = true
		w.snapMu.Unlock()
		return true
	}
	return false
}

func (w *Watcher) overflowFailuresForTest() int {
	w.snapMu.Lock()
	defer w.snapMu.Unlock()
	return w.overflowFailures
}

func (w *Watcher) fallbackTriggeredForTest() bool {
	return w.shouldFallback()
}

func (w *Watcher) primeSnapshotForTest() {
	w.snapMu.Lock()
	w.prevSnapshot = w.buildSnapshot()
	w.snapMu.Unlock()
}
