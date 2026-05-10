package main

import (
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

// copyAssets copies files matching a glob pattern from srcDir to destDir,
// preserving the relative directory structure.
//
// Unlike filepath.Glob, this supports ** (double-star) patterns so that
// patterns like **/*.json match files at any depth. Input patterns are
// normalised to forward slashes before matching (required by doublestar),
// and returned paths are converted back to the OS-native separator.
func copyAssets(srcDir, destDir, pattern string) (int, error) {
	fsys := os.DirFS(srcDir)
	matches, err := doublestar.Glob(fsys, filepath.ToSlash(pattern))
	if err != nil {
		return 0, err
	}

	count := 0
	for _, match := range matches {
		nativeMatch := filepath.FromSlash(match)
		abs := filepath.Join(srcDir, nativeMatch)

		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			continue
		}

		dest := filepath.Join(destDir, nativeMatch)

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return count, err
		}

		data, err := os.ReadFile(abs)
		if err != nil {
			return count, err
		}
		if err := os.WriteFile(dest, data, info.Mode()); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
