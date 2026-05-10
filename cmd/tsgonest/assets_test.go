package main

import (
	"os"
	"path/filepath"
	"testing"
)

// makeFixture creates a directory tree under base. Each entry in files is a
// slash-separated relative path; directories are created as needed.
func makeFixture(t *testing.T, base string, files []string) {
	t.Helper()
	for _, f := range files {
		full := filepath.Join(base, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte("content of "+f), 0644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

// assertCopied checks that every path in want exists under destDir.
func assertCopied(t *testing.T, destDir string, want []string) {
	t.Helper()
	for _, rel := range want {
		p := filepath.Join(destDir, filepath.FromSlash(rel))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected %s to be copied, but it is missing", rel)
		}
	}
}

// assertNotCopied checks that none of the paths in unwanted exist under destDir.
func assertNotCopied(t *testing.T, destDir string, unwanted []string) {
	t.Helper()
	for _, rel := range unwanted {
		p := filepath.Join(destDir, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err == nil {
			t.Errorf("expected %s NOT to be copied, but it exists", rel)
		}
	}
}

func TestCopyAssets_DoubleStarJSON(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"a.json",
		"sub/b.json",
		"sub/deep/c.json",
		"sub/deep/deeper/d.json",
		"sub/other.ts",
		"ignore.ts",
	})

	count, err := copyAssets(src, dst, "**/*.json")
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}
	if count != 4 {
		t.Errorf("count = %d, want 4", count)
	}

	assertCopied(t, dst, []string{
		"a.json",
		"sub/b.json",
		"sub/deep/c.json",
		"sub/deep/deeper/d.json",
	})
	assertNotCopied(t, dst, []string{"sub/other.ts", "ignore.ts"})
}

func TestCopyAssets_SingleStarFlatOnly(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"top.json",
		"also.json",
		"nested/deep.json",
	})

	count, err := copyAssets(src, dst, "*.json")
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (only top-level)", count)
	}

	assertCopied(t, dst, []string{"top.json", "also.json"})
	assertNotCopied(t, dst, []string{"nested/deep.json"})
}

func TestCopyAssets_PrefixedDoubleStarPattern(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"src/index.ts",
		"src/sub/util.ts",
		"src/sub/deep/helper.ts",
		"root.ts",
		"other/thing.ts",
	})

	count, err := copyAssets(src, dst, "src/**/*.ts")
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	assertCopied(t, dst, []string{
		"src/index.ts",
		"src/sub/util.ts",
		"src/sub/deep/helper.ts",
	})
	assertNotCopied(t, dst, []string{"root.ts", "other/thing.ts"})
}

func TestCopyAssets_NoMatches(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{"a.ts", "b.ts"})

	count, err := copyAssets(src, dst, "**/*.json")
	if err != nil {
		t.Fatalf("copyAssets error (no matches should not error): %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestCopyAssets_DeeplyNested(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"a/b/c/d/e.json",
		"a/b/c/d/e/f/g.json",
		"a/b/c/d/e/f/g/h/i.json",
		"skip.ts",
	})

	count, err := copyAssets(src, dst, "**/*.json")
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	assertCopied(t, dst, []string{
		"a/b/c/d/e.json",
		"a/b/c/d/e/f/g.json",
		"a/b/c/d/e/f/g/h/i.json",
	})
}

func TestCopyAssets_WindowsBackslashPattern(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"assets/icon.png",
		"assets/sub/logo.png",
	})

	// Simulate a Windows-style pattern with backslashes; filepath.ToSlash inside
	// copyAssets must normalise it before doublestar sees it.
	pattern := filepath.FromSlash("assets/**/*.png")

	count, err := copyAssets(src, dst, pattern)
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	assertCopied(t, dst, []string{
		"assets/icon.png",
		"assets/sub/logo.png",
	})
}

func TestCopyAssets_PreservesRelativeStructure(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"locales/en/messages.json",
		"locales/fr/messages.json",
	})

	_, err := copyAssets(src, dst, "**/*.json")
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}

	assertCopied(t, dst, []string{
		"locales/en/messages.json",
		"locales/fr/messages.json",
	})
}

func TestCopyAssets_ExactFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	makeFixture(t, src, []string{
		"assets/icon.png",
		"assets/other.svg",
	})

	count, err := copyAssets(src, dst, "assets/icon.png")
	if err != nil {
		t.Fatalf("copyAssets error: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	assertCopied(t, dst, []string{"assets/icon.png"})
	assertNotCopied(t, dst, []string{"assets/other.svg"})
}
