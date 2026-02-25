package buildcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCachePath(t *testing.T) {
	t.Run("with outDir", func(t *testing.T) {
		tests := []struct {
			outDir string
			tsconf string
			want   string
		}{
			{"/project/dist", "/project/tsconfig.json", "/project/dist/.tsgonest-cache"},
			{"/project/dist", "/project/tsconfig.build.json", "/project/dist/.tsgonest-cache"},
			{"dist", "tsconfig.json", "dist/.tsgonest-cache"},
		}
		for _, tt := range tests {
			got := CachePath(tt.outDir, tt.tsconf)
			if got != tt.want {
				t.Errorf("CachePath(%q, %q) = %q, want %q", tt.outDir, tt.tsconf, got, tt.want)
			}
		}
	})

	t.Run("without outDir fallback", func(t *testing.T) {
		tests := []struct {
			tsconf string
			want   string
		}{
			{"/foo/tsconfig.json", "/foo/tsconfig.tsgonest-cache"},
			{"/foo/tsconfig.build.json", "/foo/tsconfig.build.tsgonest-cache"},
			{"/foo/bar/tsconfig.app.json", "/foo/bar/tsconfig.app.tsgonest-cache"},
			{"tsconfig.json", "tsconfig.tsgonest-cache"},
		}
		for _, tt := range tests {
			got := CachePath("", tt.tsconf)
			if got != tt.want {
				t.Errorf("CachePath(\"\", %q) = %q, want %q", tt.tsconf, got, tt.want)
			}
		}
	})
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()

	// Hash of existing file
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world"), 0644)
	hash1 := HashFile(path)
	if hash1 == "" {
		t.Fatal("HashFile returned empty for existing file")
	}

	// Same content = same hash
	path2 := filepath.Join(dir, "test2.txt")
	os.WriteFile(path2, []byte("hello world"), 0644)
	hash2 := HashFile(path2)
	if hash1 != hash2 {
		t.Errorf("same content produced different hashes: %q vs %q", hash1, hash2)
	}

	// Different content = different hash
	path3 := filepath.Join(dir, "test3.txt")
	os.WriteFile(path3, []byte("hello world!"), 0644)
	hash3 := HashFile(path3)
	if hash1 == hash3 {
		t.Error("different content produced same hash")
	}

	// Non-existent file = empty string
	hash4 := HashFile(filepath.Join(dir, "nonexistent"))
	if hash4 != "" {
		t.Errorf("HashFile returned %q for non-existent file, want empty", hash4)
	}
}

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "test.tsgonest-cache")

	// Load non-existent = nil
	c := Load(cachePath)
	if c != nil {
		t.Fatal("Load should return nil for non-existent file")
	}

	// Save and reload
	original := New("abc123", []string{"/foo/openapi.json", "/foo/manifest.json"})
	if err := Save(cachePath, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(cachePath)
	if loaded == nil {
		t.Fatal("Load returned nil after Save")
	}

	if loaded.V != original.V {
		t.Errorf("V = %d, want %d", loaded.V, original.V)
	}
	if loaded.ConfigHash != original.ConfigHash {
		t.Errorf("ConfigHash = %q, want %q", loaded.ConfigHash, original.ConfigHash)
	}
	if len(loaded.Outputs) != len(original.Outputs) {
		t.Fatalf("Outputs length = %d, want %d", len(loaded.Outputs), len(original.Outputs))
	}
	for i, o := range loaded.Outputs {
		if o != original.Outputs[i] {
			t.Errorf("Outputs[%d] = %q, want %q", i, o, original.Outputs[i])
		}
	}
}

func TestLoadCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "corrupted.tsgonest-cache")

	// Write garbage
	os.WriteFile(cachePath, []byte("not json at all {{{"), 0644)

	c := Load(cachePath)
	if c != nil {
		t.Fatal("Load should return nil for corrupted JSON")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "empty.tsgonest-cache")

	os.WriteFile(cachePath, []byte(""), 0644)

	c := Load(cachePath)
	if c != nil {
		t.Fatal("Load should return nil for empty file")
	}
}

func TestIsValid_NilCache(t *testing.T) {
	var c *Cache
	if c.IsValid("anything") {
		t.Error("nil cache should not be valid")
	}
}

func TestIsValid_SchemaVersionMismatch(t *testing.T) {
	c := &Cache{
		V:          SchemaVersion + 1, // future version
		ConfigHash: "abc",
		Outputs:    nil,
	}
	if c.IsValid("abc") {
		t.Error("cache with wrong schema version should not be valid")
	}
}

func TestIsValid_ConfigHashMismatch(t *testing.T) {
	c := &Cache{
		V:          SchemaVersion,
		ConfigHash: "old-hash",
		Outputs:    nil,
	}
	if c.IsValid("new-hash") {
		t.Error("cache with mismatched config hash should not be valid")
	}
}

func TestIsValid_OutputFileMissing(t *testing.T) {
	dir := t.TempDir()
	existingFile := filepath.Join(dir, "exists.json")
	os.WriteFile(existingFile, []byte("{}"), 0644)

	c := &Cache{
		V:          SchemaVersion,
		ConfigHash: "abc",
		Outputs: []string{
			existingFile,
			filepath.Join(dir, "missing.json"), // doesn't exist
		},
	}
	if c.IsValid("abc") {
		t.Error("cache with missing output file should not be valid")
	}
}

func TestIsValid_AllChecksPass(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "openapi.json")
	file2 := filepath.Join(dir, "manifest.json")
	os.WriteFile(file1, []byte("{}"), 0644)
	os.WriteFile(file2, []byte("{}"), 0644)

	c := &Cache{
		V:          SchemaVersion,
		ConfigHash: "correct-hash",
		Outputs: []string{
			file1,
			file2,
		},
	}
	if !c.IsValid("correct-hash") {
		t.Error("cache with all checks passing should be valid")
	}
}

func TestIsValid_NoOutputs(t *testing.T) {
	// A project with no OpenAPI or manifest outputs — just validation/serialization
	c := &Cache{
		V:          SchemaVersion,
		ConfigHash: "hash",
		Outputs:    nil,
	}
	if !c.IsValid("hash") {
		t.Error("cache with no output files to check should be valid when hash matches")
	}
}

func TestIsValid_EmptyConfigHash(t *testing.T) {
	// No config file used → both should be empty
	c := &Cache{
		V:          SchemaVersion,
		ConfigHash: "",
		Outputs:    nil,
	}
	if !c.IsValid("") {
		t.Error("cache with empty config hash should be valid when current is also empty")
	}

	// But if someone adds a config, it should invalidate
	if c.IsValid("now-has-config") {
		t.Error("cache with empty config hash should be invalid when config is now present")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "test.tsgonest-cache")

	// Write a cache file
	os.WriteFile(cachePath, []byte(`{"v":1}`), 0644)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatal("cache file should exist before delete")
	}

	// Delete it
	Delete(cachePath)
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not exist after delete")
	}

	// Delete non-existent — should not panic
	Delete(filepath.Join(dir, "nonexistent"))
}

func TestNew(t *testing.T) {
	c := New("hash123", []string{"/a", "/b"})
	if c.V != SchemaVersion {
		t.Errorf("V = %d, want %d", c.V, SchemaVersion)
	}
	if c.ConfigHash != "hash123" {
		t.Errorf("ConfigHash = %q, want %q", c.ConfigHash, "hash123")
	}
	if len(c.Outputs) != 2 {
		t.Fatalf("Outputs length = %d, want 2", len(c.Outputs))
	}
}

func TestSaveAtomicity(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "atomic.tsgonest-cache")

	// Save a cache file
	c := New("hash", nil)
	if err := Save(cachePath, c); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify temp file is cleaned up (no .tmp file should remain)
	tmpPath := cachePath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}

	// Verify the cache file exists and is valid
	loaded := Load(cachePath)
	if loaded == nil {
		t.Fatal("failed to load after atomic save")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "sub", "dir", "cache.tsgonest-cache")

	c := New("hash", nil)
	if err := Save(nestedPath, c); err != nil {
		t.Fatalf("Save failed to create nested dirs: %v", err)
	}

	loaded := Load(nestedPath)
	if loaded == nil {
		t.Fatal("failed to load from nested directory")
	}
}

func TestRoundTripWithRealFiles(t *testing.T) {
	// Simulate a real scenario: config file + output files
	dir := t.TempDir()

	// Create a "config" file and hash it
	configPath := filepath.Join(dir, "tsgonest.config.json")
	os.WriteFile(configPath, []byte(`{"openapi":{"output":"dist/openapi.json"}}`), 0644)
	configHash := HashFile(configPath)
	if configHash == "" {
		t.Fatal("failed to hash config file")
	}

	// Create "output" files
	openapiPath := filepath.Join(dir, "dist", "openapi.json")
	manifestPath := filepath.Join(dir, "dist", "__tsgonest_manifest.json")
	os.MkdirAll(filepath.Join(dir, "dist"), 0755)
	os.WriteFile(openapiPath, []byte(`{"openapi":"3.2.0"}`), 0644)
	os.WriteFile(manifestPath, []byte(`{"validators":{}}`), 0644)

	// Save cache
	cachePath := filepath.Join(dir, "tsconfig.tsgonest-cache")
	c := New(configHash, []string{openapiPath, manifestPath})
	if err := Save(cachePath, c); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Scenario 1: Everything unchanged → valid
	loaded := Load(cachePath)
	if !loaded.IsValid(configHash) {
		t.Error("cache should be valid when nothing changed")
	}

	// Scenario 2: Config file changed → invalid
	os.WriteFile(configPath, []byte(`{"openapi":{"output":"dist/api.json"}}`), 0644)
	newConfigHash := HashFile(configPath)
	if loaded.IsValid(newConfigHash) {
		t.Error("cache should be invalid when config changed")
	}

	// Scenario 3: Output file deleted → invalid
	os.Remove(openapiPath)
	if loaded.IsValid(configHash) {
		t.Error("cache should be invalid when output file deleted")
	}
}
