package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWatch_FsnotifyDetectsWrite verifies that the fsnotify-backed Watch()
// detects file modifications with low latency.
func TestWatch_FsnotifyDetectsWrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.ts"), []byte("const x = 1;"), 0644)

	changed := make(chan []Event, 1)
	w := New([]string{dir}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		changed <- events
	})

	go w.Watch()
	defer w.Stop()

	// Let the watcher initialize
	time.Sleep(200 * time.Millisecond)

	// Modify the file
	os.WriteFile(filepath.Join(dir, "app.ts"), []byte("const x = 2;"), 0644)

	select {
	case events := <-changed:
		if len(events) == 0 {
			t.Fatal("expected at least 1 event")
		}
		found := false
		for _, e := range events {
			if filepath.Base(e.Path) == "app.ts" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected event for app.ts, got %v", events)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("fsnotify did not detect file write within 3s")
	}
}

// TestWatch_FsnotifyDetectsCreate verifies that new files are detected.
func TestWatch_FsnotifyDetectsCreate(t *testing.T) {
	dir := t.TempDir()

	changed := make(chan []Event, 1)
	w := New([]string{dir}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		changed <- events
	})

	go w.Watch()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	// Create a new .ts file
	os.WriteFile(filepath.Join(dir, "new.ts"), []byte("export {}"), 0644)

	select {
	case events := <-changed:
		found := false
		for _, e := range events {
			if filepath.Base(e.Path) == "new.ts" && e.Op == "create" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected create event for new.ts, got %v", events)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("fsnotify did not detect file creation within 3s")
	}
}

// TestWatch_FsnotifyFiltersExtensions verifies that non-matching extensions are ignored.
func TestWatch_FsnotifyFiltersExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.ts"), []byte("const x = 1;"), 0644)

	changed := make(chan []Event, 1)
	w := New([]string{dir}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		changed <- events
	})

	go w.Watch()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	// Write a .json file — should NOT trigger callback
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)

	select {
	case events := <-changed:
		t.Errorf("unexpected events for .json file: %v", events)
	case <-time.After(500 * time.Millisecond):
		// Good — no false positive
	}
}

// TestWatch_FsnotifyDetectsSubdirChanges verifies that changes in subdirectories
// are detected (recursive watching).
func TestWatch_FsnotifyDetectsSubdirChanges(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "controllers")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "app.controller.ts"), []byte("@Controller()"), 0644)

	changed := make(chan []Event, 1)
	w := New([]string{dir}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		changed <- events
	})

	go w.Watch()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	// Modify in subdirectory
	os.WriteFile(filepath.Join(subDir, "app.controller.ts"), []byte("@Controller('api')"), 0644)

	select {
	case events := <-changed:
		found := false
		for _, e := range events {
			if filepath.Base(e.Path) == "app.controller.ts" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected event for app.controller.ts in subdir, got %v", events)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("fsnotify did not detect subdirectory file change within 3s")
	}
}

// TestWatch_FsnotifyNewSubdir verifies that files created in a newly created
// subdirectory are detected (the new directory is dynamically added to the watch).
func TestWatch_FsnotifyNewSubdir(t *testing.T) {
	dir := t.TempDir()

	changed := make(chan []Event, 2)
	w := New([]string{dir}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {
		changed <- events
	})

	go w.Watch()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	// Create a new subdirectory and a file in it
	newDir := filepath.Join(dir, "services")
	os.MkdirAll(newDir, 0755)
	// Small delay to let fsnotify pick up the new directory
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(filepath.Join(newDir, "user.service.ts"), []byte("export class UserService {}"), 0644)

	// We may get the directory creation event first, then the file event
	timeout := time.After(3 * time.Second)
	for {
		select {
		case events := <-changed:
			for _, e := range events {
				if filepath.Base(e.Path) == "user.service.ts" {
					return // Success
				}
			}
		case <-timeout:
			t.Fatal("fsnotify did not detect file in newly created subdirectory within 3s")
		}
	}
}

// TestWatch_StopIsIdempotent verifies that calling Stop() multiple times doesn't panic.
func TestWatch_StopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	w := New([]string{dir}, []string{".ts"}, 50*time.Millisecond, func(events []Event) {})

	go w.Watch()
	time.Sleep(100 * time.Millisecond)

	w.Stop()
	w.Stop() // should not panic
}
