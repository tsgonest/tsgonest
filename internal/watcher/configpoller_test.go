package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWatchFiles_DetectsChange verifies that modifying a watched file
// triggers the onChange callback with the correct path.
func TestWatchFiles_DetectsChange(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{"compilerOptions": {}}`), 0644)

	changed := make(chan string, 1)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	// Let the poller take an initial snapshot
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	os.WriteFile(configFile, []byte(`{"compilerOptions": {"strict": true}}`), 0644)

	select {
	case path := <-changed:
		if path != configFile {
			t.Errorf("expected changed path %s, got %s", configFile, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("config change not detected within 2s")
	}
}

// TestWatchFiles_DetectsMultipleFiles verifies that the poller watches
// all supplied files and reports the specific file that changed.
func TestWatchFiles_DetectsMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	tsconfig := filepath.Join(dir, "tsconfig.json")
	tsgonestConfig := filepath.Join(dir, "tsgonest.config.json")

	os.WriteFile(tsconfig, []byte(`{}`), 0644)
	os.WriteFile(tsgonestConfig, []byte(`{}`), 0644)

	changed := make(chan string, 2)
	stop := WatchFiles([]string{tsconfig, tsgonestConfig}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	time.Sleep(100 * time.Millisecond)

	// Modify only the tsgonest config
	os.WriteFile(tsgonestConfig, []byte(`{"entryFile": "main"}`), 0644)

	select {
	case path := <-changed:
		if path != tsgonestConfig {
			t.Errorf("expected %s, got %s", tsgonestConfig, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tsgonest config change not detected")
	}

	// Now modify tsconfig
	os.WriteFile(tsconfig, []byte(`{"compilerOptions": {"strict": true}}`), 0644)

	select {
	case path := <-changed:
		if path != tsconfig {
			t.Errorf("expected %s, got %s", tsconfig, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tsconfig change not detected")
	}
}

// TestWatchFiles_NoFalsePositives verifies that the poller does not fire
// when watched files are not modified.
func TestWatchFiles_NoFalsePositives(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{}`), 0644)

	changed := make(chan string, 1)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	// Wait several poll cycles without modifying anything
	select {
	case path := <-changed:
		t.Errorf("unexpected change detected for %s — file was not modified", path)
	case <-time.After(500 * time.Millisecond):
		// Good — no false positive
	}
}

// TestWatchFiles_StopsCleanly verifies that calling stop() halts polling
// and no further callbacks are fired.
func TestWatchFiles_StopsCleanly(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{}`), 0644)

	changed := make(chan string, 1)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})

	time.Sleep(100 * time.Millisecond)
	stop()

	// Modify after stop — should NOT trigger callback
	os.WriteFile(configFile, []byte(`{"modified": true}`), 0644)

	select {
	case path := <-changed:
		t.Errorf("callback fired after stop for %s", path)
	case <-time.After(500 * time.Millisecond):
		// Good — no callback after stop
	}
}

// TestWatchFiles_IgnoresNonExistentFiles verifies that the poller handles
// files that don't exist at startup (e.g., auto-discovered config that
// hasn't been created yet). If the file is later created, it should
// be detected as a change.
func TestWatchFiles_IgnoresNonExistentFiles(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsgonest.config.json")
	// File does NOT exist at startup

	changed := make(chan string, 1)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	time.Sleep(100 * time.Millisecond)

	// Create the file — should be detected as a change
	os.WriteFile(configFile, []byte(`{}`), 0644)

	select {
	case path := <-changed:
		if path != configFile {
			t.Errorf("expected %s, got %s", configFile, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("newly created config file not detected")
	}
}

// TestWatchFiles_DetectsFileDeletion verifies that the poller detects
// when a watched file is deleted.
func TestWatchFiles_DetectsFileDeletion(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{}`), 0644)

	changed := make(chan string, 1)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	time.Sleep(100 * time.Millisecond)

	// Delete the file
	os.Remove(configFile)

	select {
	case path := <-changed:
		if path != configFile {
			t.Errorf("expected %s, got %s", configFile, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("config file deletion not detected")
	}
}

// TestWatchFiles_EmptyPaths verifies that the poller handles an empty
// file list gracefully (no panic, no callback, stop works).
func TestWatchFiles_EmptyPaths(t *testing.T) {
	changed := make(chan string, 1)
	stop := WatchFiles(nil, 50*time.Millisecond, func(path string) {
		changed <- path
	})

	select {
	case <-changed:
		t.Error("unexpected callback with no files to watch")
	case <-time.After(200 * time.Millisecond):
		// Good
	}

	stop() // Should not panic
}
