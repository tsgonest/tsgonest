package main

import (
	"io"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeShutdownProc records the order of Stop and Kill and lets a test hold
// Stop open, the way a child that takes its time over SIGTERM does.
type fakeShutdownProc struct {
	mu          sync.Mutex
	calls       []string
	releaseStop chan struct{}
}

func newFakeShutdownProc() *fakeShutdownProc {
	return &fakeShutdownProc{releaseStop: make(chan struct{})}
}

func (f *fakeShutdownProc) Stop() error {
	f.record("stop")
	<-f.releaseStop
	return nil
}

func (f *fakeShutdownProc) Kill() { f.record("kill") }

func (f *fakeShutdownProc) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
}

func (f *fakeShutdownProc) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// fakeClock hands out a controllable time so the coalescing window is exact.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func runShutdown(t *testing.T, sigCh chan os.Signal, proc *fakeShutdownProc, clock *fakeClock) (exited chan int, finished chan struct{}) {
	t.Helper()
	exited = make(chan int, 1)
	finished = make(chan struct{})
	go func() {
		shutdownOnSignal(sigCh, proc, io.Discard, clock.Now, func(code int) { exited <- code })
		close(finished)
	}()
	return exited, finished
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not happen", what)
	}
}

// One Ctrl-C under turbo or pnpm arrives as two or three SIGINTs within
// milliseconds. That is one request: the child gets its graceful stop and
// nothing is force-killed.
func TestShutdownOnSignal_CoalescesCopiesOfOneKeypress(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	proc := newFakeShutdownProc()
	clock := &fakeClock{}
	sigCh <- syscall.SIGINT
	sigCh <- syscall.SIGINT
	exited, finished := runShutdown(t, sigCh, proc, clock)

	// Give the handler a moment to see both buffered signals.
	time.Sleep(50 * time.Millisecond)
	close(proc.releaseStop)
	waitFor(t, finished, "graceful shutdown")

	select {
	case code := <-exited:
		t.Fatalf("exit(%d) called for a coalesced signal", code)
	default:
	}
	if got := proc.Calls(); len(got) != 1 || got[0] != "stop" {
		t.Fatalf("expected a single Stop, got %v", got)
	}
}

// A second, later signal is a real force-kill request. The child must be
// killed before exit, or it outlives tsgonest on its port (the EADDRINUSE on
// the next start).
func TestShutdownOnSignal_SecondSignalKillsChildBeforeExit(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	proc := newFakeShutdownProc()
	clock := &fakeClock{}
	exited, finished := runShutdown(t, sigCh, proc, clock)

	time.Sleep(50 * time.Millisecond)
	clock.Advance(signalCoalesceWindow + time.Millisecond)
	sigCh <- syscall.SIGINT
	waitFor(t, finished, "forced shutdown")

	select {
	case code := <-exited:
		if code != 1 {
			t.Fatalf("exit code %d, want 1", code)
		}
	default:
		t.Fatal("exit was not called on the second signal")
	}
	got := proc.Calls()
	if len(got) != 2 || got[0] != "stop" || got[1] != "kill" {
		t.Fatalf("expected Stop then Kill before exit, got %v", got)
	}
	close(proc.releaseStop)
}

func TestShutdownOnSignal_ReturnsWhenStopCompletes(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	proc := newFakeShutdownProc()
	clock := &fakeClock{}
	exited, finished := runShutdown(t, sigCh, proc, clock)

	close(proc.releaseStop)
	waitFor(t, finished, "graceful shutdown")
	select {
	case code := <-exited:
		t.Fatalf("exit(%d) called after a clean stop", code)
	default:
	}
}

// No child yet (the initial build failed): the handler must still return so
// the loop can exit, and must not touch a nil runner.
func TestShutdownOnSignal_NilProc(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	clock := &fakeClock{}
	finished := make(chan struct{})
	go func() {
		shutdownOnSignal(sigCh, nil, io.Discard, clock.Now, func(int) { t.Error("exit called") })
		close(finished)
	}()
	waitFor(t, finished, "shutdown with no child")
}
