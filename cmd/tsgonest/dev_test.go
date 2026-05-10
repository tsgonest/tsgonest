package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── hasDistPrefix ────────────────────────────────────────────────────────────

func TestHasDistPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		// Forward-slash forms (all platforms)
		{"dist/main.js", true},
		{"dist/sub/x.js", true},
		{"dist", true},

		// Must NOT match — different prefix
		{"main.js", false},
		{"notdist/x.js", false},
		{"dist-extra/x.js", false}, // substring trap: "dist-" is not "dist/"
		{"src/main.js", false},
		{"", false},
	}

	// On Windows, backslash is the path separator so filepath.ToSlash converts it.
	if runtime.GOOS == "windows" {
		cases = append(cases,
			struct {
				input string
				want  bool
			}{`dist\main.js`, true},
			struct {
				input string
				want  bool
			}{`dist\sub\x.js`, true},
			struct {
				input string
				want  bool
			}{`dist\`, true},
		)
	}

	for _, tc := range cases {
		got := hasDistPrefix(tc.input)
		if got != tc.want {
			t.Errorf("hasDistPrefix(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// ── resolveEntryPoint ────────────────────────────────────────────────────────

// noStat always returns "not found" — simulates the withSR probe failing.
func noStat(string) (os.FileInfo, error) { return nil, errors.New("not found") }

// yesStat always succeeds — simulates the withSR probe succeeding.
func yesStat(string) (os.FileInfo, error) { return nil, nil }

func TestResolveEntryPoint_AbsolutePassthrough(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "absolute", "path", "main.js")
	got := resolveEntryPoint(abs, "/some/cwd", "src", noStat)
	if got != abs {
		t.Errorf("absolute path should be returned unchanged; got %q", got)
	}
}

func TestResolveEntryPoint_AlreadyUnderDist_ForwardSlash(t *testing.T) {
	got := resolveEntryPoint("dist/main.js", "/cwd", "src", noStat)
	if got != "dist/main.js" {
		t.Errorf("dist/ path should be returned unchanged; got %q", got)
	}
}

func TestResolveEntryPoint_BareRelative_FallsBackToDist(t *testing.T) {
	cwd := "/project"
	got := resolveEntryPoint("main.js", cwd, "src", noStat)
	want := filepath.Join(cwd, "dist", "main.js")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveEntryPoint_BareRelative_NoExtension_FallsBackToDist(t *testing.T) {
	cwd := "/project"
	got := resolveEntryPoint("main", cwd, "src", noStat)
	want := filepath.Join(cwd, "dist", "main.js")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveEntryPoint_BareRelative_UsesSourceRoot(t *testing.T) {
	cwd := "/project"
	// yesStat means the withSR probe succeeds
	got := resolveEntryPoint("main.js", cwd, "src", yesStat)
	want := filepath.Join(cwd, "dist", "src", "main.js")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveEntryPoint_BareRelative_CustomSourceRoot(t *testing.T) {
	cwd := "/project"
	got := resolveEntryPoint("main.js", cwd, "lib", yesStat)
	want := filepath.Join(cwd, "dist", "lib", "main.js")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveEntryPoint_WindowsBackslash_NeverDoublesDistPrefix is the core
// regression for #160. On Windows a user may type --entry 'dist\main.js'.
// With the old byte-exact HasPrefix("dist/") check this would enter the
// prefixing branch and produce <cwd>\dist\src\dist\main.js (doubled dist).
// hasDistPrefix normalizes separators first, so the path is left unchanged.
func TestResolveEntryPoint_WindowsBackslash_NeverDoublesDistPrefix(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash as path separator only applies on Windows")
	}
	cwd := `C:\project`
	input := `dist\main.js`
	got := resolveEntryPoint(input, cwd, "src", noStat)
	normalized := filepath.ToSlash(got)
	if strings.Contains(normalized, "dist/dist") {
		t.Errorf("double dist in resolved path: %q (input %q)", got, input)
	}
	if got != input {
		t.Errorf(`dist\ path should be returned unchanged; got %q`, got)
	}
}
