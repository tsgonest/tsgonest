package watcher

import (
	"os"
	"sync"
	"time"
)

// statFn is the os.Stat implementation used by WatchFiles.
// Tests may replace it to inject transient or permanent errors.
var statFn = os.Stat

// statMu guards reads and writes to statFn so tests can safely swap it.
var statMu sync.RWMutex

func stat(name string) (os.FileInfo, error) {
	statMu.RLock()
	fn := statFn
	statMu.RUnlock()
	return fn(name)
}

// WatchFiles polls specific file paths for changes (modification, creation,
// or deletion). When any watched file changes, onChange is called with the
// file path. Returns a stop function that halts polling.
//
// This is designed for watching config files (tsconfig.json, tsgonest.config.ts)
// that need to trigger a full dev-loop restart when modified.
func WatchFiles(paths []string, pollInterval time.Duration, onChange func(path string)) (stop func()) {
	stopCh := make(chan struct{})

	go func() {
		// Take initial snapshot of mod times.
		// Files that don't exist get zero-value time.
		snapshot := snapshotFiles(paths)

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				for _, path := range paths {
					info, err := stat(path)

					oldTime := snapshot[path]
					var newTime time.Time
					if err == nil {
						newTime = info.ModTime()
					} else if !os.IsNotExist(err) {
						// Transient: lock contention, AV scan, mid-rename window.
						// Skip this poll; next poll will see the settled state.
						continue
					}
					// zero → non-zero: file created
					// non-zero → different: file modified
					// non-zero → zero: file deleted
					if newTime != oldTime {
						snapshot[path] = newTime
						onChange(path)
					}
				}
			}
		}
	}()

	// Idempotent stop — safe to call multiple times (matches Watcher.Stop pattern)
	var once sync.Once
	return func() {
		once.Do(func() { close(stopCh) })
	}
}

// snapshotFiles records the current mod time for each path.
// Files that don't exist get a zero-value time.
func snapshotFiles(paths []string) map[string]time.Time {
	snap := make(map[string]time.Time, len(paths))
	for _, p := range paths {
		info, err := stat(p)
		if err == nil {
			snap[p] = info.ModTime()
		}
		// Missing files get zero time (default map value)
	}
	return snap
}
