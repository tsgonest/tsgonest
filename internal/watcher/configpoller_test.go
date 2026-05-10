package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
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

// TestWatchFiles_TransientStatErrorSkipsPoll verifies that a non-ENOENT stat
// error (e.g. lock contention, AV scan, mid-rename) is treated as transient:
// onChange must NOT fire.
func TestWatchFiles_TransientStatErrorSkipsPoll(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{}`), 0644)

	// Inject a stat function that returns a transient (non-ENOENT) error.
	var callCount atomic.Int32
	statMu.Lock()
	orig := statFn
	statFn = func(name string) (os.FileInfo, error) {
		callCount.Add(1)
		return nil, errors.New("sharing violation: file locked by another process")
	}
	statMu.Unlock()
	t.Cleanup(func() {
		statMu.Lock()
		statFn = orig
		statMu.Unlock()
	})

	changed := make(chan string, 1)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	// Wait for several poll cycles.
	time.Sleep(300 * time.Millisecond)

	select {
	case path := <-changed:
		t.Errorf("onChange fired unexpectedly for %s on transient stat error", path)
	default:
		// Good — transient errors were skipped
	}

	if n := callCount.Load(); n == 0 {
		t.Error("statFn was never called — test is not exercising the poller")
	}
}

// TestWatchFiles_DeletionStillFires verifies that a real ENOENT (file removed
// by the user) still triggers onChange exactly once.
func TestWatchFiles_DeletionStillFires(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{}`), 0644)

	changed := make(chan string, 2)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	// Let initial snapshot settle.
	time.Sleep(100 * time.Millisecond)

	os.Remove(configFile)

	select {
	case path := <-changed:
		if path != configFile {
			t.Errorf("expected %s, got %s", configFile, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deletion not detected within 2s")
	}

	// Ensure only one callback fired (no double-fire for the deletion).
	time.Sleep(150 * time.Millisecond)
	select {
	case extra := <-changed:
		t.Errorf("spurious second callback for %s", extra)
	default:
		// Good
	}
}

// TestWatchFiles_AtomicReplaceFiresOnce simulates the Windows atomic-save
// pattern (delete + rename) and verifies that onChange fires at most once
// for the net change, not twice (once for delete, once for re-create).
//
// The test controls timing by using a stat stub that sequences through phases:
//  1. Initial: file exists (snapshot is taken with real mtime)
//  2. Mid-rename polls (up to maxTransient): transient non-ENOENT error
//  3. Post-rename polls: file exists again with a new mtime
func TestWatchFiles_AtomicReplaceFiresOnce(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(configFile, []byte(`{}`), 0644)

	realInfo, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("setup stat failed: %v", err)
	}
	originalMtime := realInfo.ModTime()

	// newMtime is what the file reports after the atomic replace.
	newMtime := originalMtime.Add(2 * time.Second)

	const maxTransient = 3
	var pollCount atomic.Int32

	statMu.Lock()
	orig := statFn
	statFn = func(name string) (os.FileInfo, error) {
		n := int(pollCount.Add(1))
		switch {
		case n == 1:
			// Snapshot poll: return original mtime so snapshot is non-zero.
			return &fakeFileInfo{mtime: originalMtime}, nil
		case n <= 1+maxTransient:
			// Mid-rename window: transient error.
			return nil, errors.New("sharing violation")
		default:
			// Post-rename: file settled with new mtime.
			return &fakeFileInfo{mtime: newMtime}, nil
		}
	}
	statMu.Unlock()
	t.Cleanup(func() {
		statMu.Lock()
		statFn = orig
		statMu.Unlock()
	})

	changed := make(chan string, 4)
	stop := WatchFiles([]string{configFile}, 50*time.Millisecond, func(path string) {
		changed <- path
	})
	defer stop()

	// Wait long enough for snapshot + transient polls + at least one settled poll.
	time.Sleep(time.Duration(1+maxTransient+2) * 60 * time.Millisecond)

	var count int
drain:
	for {
		select {
		case <-changed:
			count++
		default:
			break drain
		}
	}

	if count != 1 {
		t.Errorf("expected onChange to fire exactly once for atomic replace, got %d", count)
	}
}

// fakeFileInfo is a minimal os.FileInfo used by the atomic-replace test.
type fakeFileInfo struct {
	mtime time.Time
}

func (f *fakeFileInfo) Name() string       { return "tsconfig.json" }
func (f *fakeFileInfo) Size() int64        { return 0 }
func (f *fakeFileInfo) Mode() os.FileMode  { return 0644 }
func (f *fakeFileInfo) ModTime() time.Time { return f.mtime }
func (f *fakeFileInfo) IsDir() bool        { return false }
func (f *fakeFileInfo) Sys() any           { return nil }
