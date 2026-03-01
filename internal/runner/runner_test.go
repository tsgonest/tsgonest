package runner

import (
	"testing"
	"time"
)

func TestRunner_StartStop(t *testing.T) {
	r := New("sleep", []string{"10"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !r.Running() {
		t.Error("expected process to be running")
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRunner_Restart(t *testing.T) {
	r := New("sleep", []string{"10"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := r.Restart(); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}
	if !r.Running() {
		t.Error("expected process to be running after restart")
	}
	r.Stop()
}

func TestRunner_StopWithoutStart(t *testing.T) {
	r := New("echo", []string{"hello"}, "")
	// Should not panic
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop without start should not error: %v", err)
	}
}

func TestRunner_Wait(t *testing.T) {
	// Run a short command and wait for it to finish
	r := New("sleep", []string{"0.1"}, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited naturally
	case <-time.After(3 * time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestRunner_DisableStdin(t *testing.T) {
	// With DisableStdin=true, the child process should NOT receive stdin.
	// We verify this by running "cat" which reads from stdin and checking
	// that it exits immediately (no stdin = EOF = exit).
	r := New("cat", nil, "")
	r.DisableStdin = true
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// cat received EOF and exited — DisableStdin worked
	case <-time.After(3 * time.Second):
		r.Stop()
		t.Fatal("cat should have exited immediately with no stdin (DisableStdin=true)")
	}
}

func TestRunner_DisableStdin_DefaultFalse(t *testing.T) {
	// By default, DisableStdin should be false
	r := New("echo", []string{"hello"}, "")
	if r.DisableStdin {
		t.Error("expected DisableStdin to default to false")
	}
}

func TestRunner_RunningAfterExit(t *testing.T) {
	// Run a command that exits quickly
	r := New("true", nil, "")
	if err := r.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	r.Wait()

	// Give a moment for ProcessState to be set
	time.Sleep(50 * time.Millisecond)

	if r.Running() {
		t.Error("expected process to not be running after exit")
	}
}
