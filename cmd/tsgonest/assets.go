package main

import (
	"os"
	"path/filepath"
)

// copyAssets copies files matching a glob pattern from srcDir to destDir,
// preserving the relative directory structure.
func copyAssets(srcDir, destDir, pattern string) (int, error) {
	matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
	if err != nil {
		return 0, err
	}

	count := 0
	for _, match := range matches {
		rel, err := filepath.Rel(srcDir, match)
		if err != nil {
			continue
		}
		dest := filepath.Join(destDir, rel)

		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return count, err
		}

		data, err := os.ReadFile(match)
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
