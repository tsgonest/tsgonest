// Package buildcache provides an incremental post-processing cache for tsgonest.
//
// When tsgo's incremental mode determines that no files changed (emittedFiles == 0),
// tsgonest can skip expensive post-processing (controller analysis, companion generation,
// manifest, OpenAPI) — but ONLY if the tsgonest config AND critical output files are
// also unchanged.
//
// The cache is intentionally conservative: if ANY check fails, the entire post-processing
// pipeline runs from scratch. There are no partial invalidation shortcuts, because a DTO
// change can affect any controller that imports it and we don't track the analysis-level
// import graph.
package buildcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SchemaVersion is bumped when the cache format or analysis output format changes.
// A mismatch forces a full rebuild, ensuring binary upgrades don't produce stale outputs.
const SchemaVersion = 1

// Cache represents the on-disk post-processing cache.
// It records what was true when post-processing last ran successfully.
type Cache struct {
	// V is the schema version. Must match SchemaVersion or cache is invalid.
	V int `json:"v"`

	// ConfigHash is the SHA-256 hex digest of the tsgonest config file content.
	// Empty string means no config file was used.
	ConfigHash string `json:"configHash"`

	// Outputs lists the absolute paths of critical output files that must
	// still exist on disk for the cache to be valid. Typically:
	// - OpenAPI JSON path
	// - Manifest JSON path
	Outputs []string `json:"outputs"`
}

// CachePath returns the cache file path inside the output directory.
// The cache lives at `<outDir>/.tsgonest-cache` so that deleting the output
// directory (deleteOutDir) also removes the cache, guaranteeing a fresh build.
//
// If outDir is empty (no output directory configured), it falls back to a
// sibling file next to the tsconfig: "tsconfig.build.json" → "tsconfig.build.tsgonest-cache".
func CachePath(outDir string, tsconfigPath string) string {
	if outDir != "" {
		return filepath.Join(outDir, ".tsgonest-cache")
	}
	// Fallback: sibling of tsconfig
	dir := filepath.Dir(tsconfigPath)
	base := filepath.Base(tsconfigPath)
	name := strings.TrimSuffix(base, ".json")
	return filepath.Join(dir, name+".tsgonest-cache")
}

// Load reads and parses a cache file from disk.
// Returns nil if the file doesn't exist, is unreadable, or is invalid JSON.
// Callers should treat nil as "cache miss" and run full post-processing.
func Load(path string) *Cache {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}

	return &c
}

// Save writes the cache to disk atomically (write to temp, rename).
// Returns an error if the write fails, but callers may choose to log and continue
// (a failed cache save just means the next build won't benefit from caching).
func Save(path string, cache *Cache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cache directory %s: %w", dir, err)
	}

	// Write to temp file first, then rename for atomicity
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing cache temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		// Clean up temp file on rename failure
		os.Remove(tmp)
		return fmt.Errorf("renaming cache file: %w", err)
	}

	return nil
}

// Delete removes the cache file from disk. Errors are ignored (file may not exist).
func Delete(path string) {
	os.Remove(path)
}

// IsValid checks whether the cache can be trusted to skip post-processing.
// ALL of the following must be true simultaneously:
//
//  1. Schema version matches (catches binary upgrades)
//  2. Config hash matches current config file content
//  3. All critical output files still exist on disk
//
// The caller is responsible for the "no emitted files" check (from tsgo's incremental
// program), which is a prerequisite before calling IsValid.
func (c *Cache) IsValid(currentConfigHash string) bool {
	if c == nil {
		return false
	}

	// Check 1: Schema version
	if c.V != SchemaVersion {
		return false
	}

	// Check 2: Config file hash
	if c.ConfigHash != currentConfigHash {
		return false
	}

	// Check 3: Output files still exist on disk
	for _, path := range c.Outputs {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}

	return true
}

// HashFile computes the SHA-256 hex digest of a file's contents.
// Returns empty string if the file doesn't exist or can't be read.
func HashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// New creates a new Cache with the current schema version.
func New(configHash string, outputs []string) *Cache {
	return &Cache{
		V:          SchemaVersion,
		ConfigHash: configHash,
		Outputs:    outputs,
	}
}
