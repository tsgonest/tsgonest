package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_BuildSnapshot(t *testing.T) {
	// Create temp dir with some .ts files
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "foo.ts"), []byte("export const x = 1;"), 0644)
	os.WriteFile(filepath.Join(dir, "bar.txt"), []byte("not ts"), 0644)

	w := New([]string{dir}, []string{".ts"}, 100*time.Millisecond, nil)
	snap := w.buildSnapshot()

	if len(snap) != 1 {
		t.Fatalf("expected 1 file in snapshot, got %d", len(snap))
	}

	// Verify the .ts file is in the snapshot
	tsPath := filepath.Join(dir, "foo.ts")
	if _, ok := snap[tsPath]; !ok {
		t.Fatalf("expected %s in snapshot", tsPath)
	}
}

func TestWatcher_BuildSnapshot_SubDirs(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(dir, "root.ts"), []byte("export const a = 1;"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.ts"), []byte("export const b = 2;"), 0644)
	os.WriteFile(filepath.Join(subDir, "style.css"), []byte("body {}"), 0644)

	w := New([]string{dir}, []string{".ts"}, 100*time.Millisecond, nil)
	snap := w.buildSnapshot()

	if len(snap) != 2 {
		t.Fatalf("expected 2 files in snapshot, got %d", len(snap))
	}
}

func TestWatcher_BuildSnapshot_MultipleExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.ts"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.tsx"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "c.js"), []byte("c"), 0644)

	w := New([]string{dir}, []string{".ts", ".tsx"}, 100*time.Millisecond, nil)
	snap := w.buildSnapshot()

	if len(snap) != 2 {
		t.Fatalf("expected 2 files in snapshot, got %d", len(snap))
	}
}

func TestWatcher_Diff_Create(t *testing.T) {
	w := &Watcher{}
	old := map[string]fileInfo{}
	new := map[string]fileInfo{
		"/a.ts": {modTime: time.Now(), size: 10},
	}
	events := w.diff(old, new)
	if len(events) != 1 || events[0].Op != "create" {
		t.Errorf("expected 1 create event, got %v", events)
	}
}

func TestWatcher_Diff_Write(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	old := map[string]fileInfo{"/a.ts": {modTime: now, size: 10}}
	new := map[string]fileInfo{"/a.ts": {modTime: now.Add(time.Second), size: 15}}
	events := w.diff(old, new)
	if len(events) != 1 || events[0].Op != "write" {
		t.Errorf("expected 1 write event, got %v", events)
	}
}

func TestWatcher_Diff_Remove(t *testing.T) {
	w := &Watcher{}
	old := map[string]fileInfo{"/a.ts": {modTime: time.Now(), size: 10}}
	new := map[string]fileInfo{}
	events := w.diff(old, new)
	if len(events) != 1 || events[0].Op != "remove" {
		t.Errorf("expected 1 remove event, got %v", events)
	}
}

func TestWatcher_Diff_NoChange(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	snap := map[string]fileInfo{"/a.ts": {modTime: now, size: 10}}
	events := w.diff(snap, snap)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %v", events)
	}
}

func TestWatcher_Diff_MultipleEvents(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	old := map[string]fileInfo{
		"/a.ts": {modTime: now, size: 10},
		"/b.ts": {modTime: now, size: 20},
	}
	new := map[string]fileInfo{
		"/a.ts": {modTime: now.Add(time.Second), size: 15}, // modified
		"/c.ts": {modTime: now, size: 30},                  // created
		// /b.ts removed
	}
	events := w.diff(old, new)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(events), events)
	}

	ops := make(map[string]bool)
	for _, e := range events {
		ops[e.Op] = true
	}
	if !ops["write"] || !ops["create"] || !ops["remove"] {
		t.Errorf("expected write, create, and remove events, got %v", events)
	}
}
