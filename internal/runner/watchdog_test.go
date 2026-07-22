//go:build !windows

package runner

import (
	"sync/atomic"
	"testing"
	"time"
)

// The crash watchdog contract: OnUnexpectedExit fires exactly when the child
// exits on its own, and never when Stop or Restart initiated the exit.

func TestRunner_OnUnexpectedExit_FiresOnCrash(t *testing.T) {
	r := New("sh", []string{"-c", "exit 3"}, "")
	codes := make(chan int, 1)
	r.OnUnexpectedExit = func(code int) { codes <- code }

	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	select {
	case code := <-codes:
		if code != 3 {
			t.Fatalf("expected exit code 3, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnUnexpectedExit never fired for a crashed child")
	}
}

func TestRunner_OnUnexpectedExit_FiresOnCleanExit(t *testing.T) {
	r := New("sh", []string{"-c", "exit 0"}, "")
	codes := make(chan int, 1)
	r.OnUnexpectedExit = func(code int) { codes <- code }

	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	select {
	case code := <-codes:
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnUnexpectedExit never fired for a clean self-exit")
	}
}

func TestRunner_OnUnexpectedExit_SilentOnStop(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	var fired atomic.Int32
	r.OnUnexpectedExit = func(int) { fired.Add(1) }

	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Stop returns once done closes; reportExit runs just after, so give the
	// wait goroutine a beat before asserting silence.
	time.Sleep(200 * time.Millisecond)
	if got := fired.Load(); got != 0 {
		t.Fatalf("OnUnexpectedExit fired %d time(s) for a Stop-initiated exit", got)
	}
}

func TestRunner_OnUnexpectedExit_SilentOnRestart(t *testing.T) {
	r := New("sleep", []string{"300"}, "")
	var fired atomic.Int32
	r.OnUnexpectedExit = func(int) { fired.Add(1) }

	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := r.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	defer r.Stop()

	time.Sleep(200 * time.Millisecond)
	if got := fired.Load(); got != 0 {
		t.Fatalf("OnUnexpectedExit fired %d time(s) for a Restart-initiated exit", got)
	}
}
