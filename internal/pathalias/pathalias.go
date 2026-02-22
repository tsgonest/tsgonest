// Package pathalias resolves tsconfig path aliases in emitted JavaScript files.
// This replaces the need for tsc-alias or tsconfig-paths at runtime.
//
// The matching algorithm is adapted from esbuild's resolver (MIT licensed).
// It follows the same behavior as TypeScript's tryLoadModuleUsingPaths():
//  1. Exact matches are checked first
//  2. Wildcard patterns are matched by longest prefix (ties broken by longest suffix)
//  3. The matched wildcard text is substituted into fallback paths
//  4. Fallback paths are resolved relative to the tsconfig directory
package pathalias

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathResolver resolves tsconfig path aliases in emitted JavaScript files.
// Path targets are resolved relative to the paths base directory
// (tsconfig dir, or explicit baseUrl if set).
type PathResolver struct {
	pathsBaseDir string              // absolute path to resolve path targets against
	outDir       string              // absolute output directory
	rootDir      string              // absolute root source directory
	aliases      map[string][]string // pattern → fallback paths (e.g., "@app/*" → ["src/*"])
}

// Config holds the resolved tsconfig values needed for path alias resolution.
// These should come from tsgo's parsed config, not hand-rolled JSON parsing.
type Config struct {
	PathsBaseDir string              // absolute dir to resolve path targets against (from GetPathsBasePath or tsconfig dir)
	OutDir       string              // absolute output directory
	RootDir      string              // absolute root source directory
	Paths        map[string][]string // alias pattern → target paths
}

// NewPathResolver creates a resolver from pre-resolved tsconfig values.
func NewPathResolver(cfg Config) *PathResolver {
	return &PathResolver{
		pathsBaseDir: cfg.PathsBaseDir,
		outDir:       cfg.OutDir,
		rootDir:      cfg.RootDir,
		aliases:      cfg.Paths,
	}
}

// HasAliases reports whether the resolver has any path aliases to resolve.
func (r *PathResolver) HasAliases() bool {
	return len(r.aliases) > 0
}

// OutDir returns the resolved absolute output directory path.
func (r *PathResolver) OutDir() string {
	return r.outDir
}

// ResolveImportsInFile reads a JS file, rewrites import specifiers, and writes it back.
func (r *PathResolver) ResolveImportsInFile(filePath string) error {
	if len(r.aliases) == 0 {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	newContent := r.ResolveImports(string(content), filePath)
	if newContent == string(content) {
		return nil // no changes needed
	}

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// ResolveImports rewrites import specifiers in JavaScript source code.
// filePath is used to compute relative paths from the importing file.
func (r *PathResolver) ResolveImports(content string, filePath string) string {
	if len(r.aliases) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = r.resolveImportLine(line, filePath)
	}
	return strings.Join(lines, "\n")
}

// resolveImportLine processes a single line, looking for require() or import from "..." patterns.
func (r *PathResolver) resolveImportLine(line string, fromFile string) string {
	for _, pattern := range []string{`require("`, `require('`, `from "`, `from '`} {
		idx := strings.Index(line, pattern)
		if idx < 0 {
			continue
		}

		quoteChar := pattern[len(pattern)-1]
		specStart := idx + len(pattern)
		specEnd := strings.IndexByte(line[specStart:], quoteChar)
		if specEnd < 0 {
			continue
		}
		specEnd += specStart

		specifier := line[specStart:specEnd]
		resolved := r.matchAndResolve(specifier, fromFile)
		if resolved != specifier {
			line = line[:specStart] + resolved + line[specEnd:]
		}
	}

	return line
}

// matchAndResolve matches an import specifier against path aliases and returns
// the resolved relative path, or the original specifier if no match.
//
// Algorithm adapted from esbuild's matchTSConfigPaths (MIT license):
//  1. Skip relative/absolute imports
//  2. Check exact matches first
//  3. Find longest-prefix wildcard match
//  4. Substitute wildcard text into fallback paths
//  5. Resolve to relative path from importing file
func (r *PathResolver) matchAndResolve(specifier string, fromFile string) string {
	// Don't touch relative imports or absolute paths
	if strings.HasPrefix(specifier, ".") || strings.HasPrefix(specifier, "/") {
		return specifier
	}

	// Phase 1: Exact matches (no wildcard)
	for key, targets := range r.aliases {
		if !strings.Contains(key, "*") && key == specifier {
			if len(targets) == 0 {
				continue
			}
			target := targets[0]
			target = strings.TrimPrefix(target, "./")
			targetPath := filepath.Join(r.pathsBaseDir, target)
			if rel := r.toRelative(targetPath, fromFile); rel != "" {
				return rel
			}
		}
	}

	// Phase 2: Wildcard pattern matches — find longest prefix match
	type wildcardMatch struct {
		prefix  string
		suffix  string
		targets []string
	}

	longestPrefixLen := -1
	longestSuffixLen := -1
	var best wildcardMatch

	for key, targets := range r.aliases {
		starIdx := strings.IndexByte(key, '*')
		if starIdx < 0 {
			continue
		}

		prefix := key[:starIdx]
		suffix := key[starIdx+1:]

		if strings.HasPrefix(specifier, prefix) && strings.HasSuffix(specifier, suffix) &&
			len(specifier) >= len(prefix)+len(suffix) {
			// Longest prefix wins; ties broken by longest suffix (deterministic ordering)
			if len(prefix) > longestPrefixLen ||
				(len(prefix) == longestPrefixLen && len(suffix) > longestSuffixLen) {
				longestPrefixLen = len(prefix)
				longestSuffixLen = len(suffix)
				best = wildcardMatch{prefix: prefix, suffix: suffix, targets: targets}
			}
		}
	}

	if longestPrefixLen >= 0 {
		// Extract the text matched by the wildcard
		matchedText := specifier[len(best.prefix) : len(specifier)-len(best.suffix)]

		for _, target := range best.targets {
			target = strings.TrimPrefix(target, "./")
			// Substitute * in the target with matched text
			resolved := strings.Replace(target, "*", matchedText, 1)
			targetPath := filepath.Join(r.pathsBaseDir, resolved)
			if rel := r.toRelative(targetPath, fromFile); rel != "" {
				return rel
			}
		}
	}

	return specifier
}

// toRelative maps a source path to its output path and returns a relative import
// from the importing file. Returns "" if mapping fails.
func (r *PathResolver) toRelative(targetPath string, fromFile string) string {
	outputPath := r.sourceToOutput(targetPath)

	rel, err := filepath.Rel(filepath.Dir(fromFile), outputPath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}

// sourceToOutput maps a source file path to its expected output path.
func (r *PathResolver) sourceToOutput(srcPath string) string {
	if r.rootDir != "" && r.outDir != "" {
		rel, err := filepath.Rel(r.rootDir, srcPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join(r.outDir, rel)
		}
	}
	if r.outDir != "" {
		return filepath.Join(r.outDir, filepath.Base(srcPath))
	}
	return srcPath
}

// ResolveAllEmittedFiles resolves path aliases in all emitted JS files.
func (r *PathResolver) ResolveAllEmittedFiles(files []string) (int, error) {
	if len(r.aliases) == 0 {
		return 0, nil
	}

	resolved := 0
	for _, f := range files {
		if !strings.HasSuffix(f, ".js") {
			continue
		}
		if err := r.ResolveImportsInFile(f); err != nil {
			return resolved, fmt.Errorf("resolving aliases in %s: %w", f, err)
		}
		resolved++
	}
	return resolved, nil
}

// InferRootDir computes the common root directory from a list of source file paths.
// Returns "" if no common prefix can be determined.
func InferRootDir(fileNames []string) string {
	if len(fileNames) == 0 {
		return ""
	}

	// Get the directory of the first file as the starting common prefix
	common := filepath.Dir(fileNames[0])
	if common == "." {
		return ""
	}

	for _, f := range fileNames[1:] {
		dir := filepath.Dir(f)
		// Walk up until we find a common prefix
		for !strings.HasPrefix(dir, common) && common != "." && common != "/" {
			common = filepath.Dir(common)
		}
		if common == "." || common == "/" {
			return ""
		}
	}

	return common
}
