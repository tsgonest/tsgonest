package analyzer

import (
	"path/filepath"
	"strings"
)

// MatchesGlob checks if a file path matches any of the include patterns
// and does not match any of the exclude patterns.
func MatchesGlob(filePath string, includePatterns []string, excludePatterns []string) bool {
	if len(includePatterns) == 0 {
		return false
	}

	// Normalize path separators
	filePath = filepath.ToSlash(filePath)

	// Check exclude first
	for _, pattern := range excludePatterns {
		pattern = filepath.ToSlash(pattern)
		if globMatch(filePath, pattern) {
			return false
		}
	}

	// Check include
	for _, pattern := range includePatterns {
		pattern = filepath.ToSlash(pattern)
		if globMatch(filePath, pattern) {
			return true
		}
	}

	return false
}

// MatchesTypeNamePattern checks if a type name matches any of the given patterns.
// Patterns support basic glob: * matches any sequence, ? matches one character.
// For example, "Legacy*" matches "LegacyUser", "LegacyOrder", etc.
func MatchesTypeNamePattern(typeName string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, typeName); matched {
			return true
		}
	}
	return false
}

// globMatch matches a path against a glob pattern with ** support.
// The matching is done against suffixes of the path — if the pattern
// is "src/**/*.controller.ts", it matches any file under a "src/" directory
// whose name matches "*.controller.ts".
func globMatch(filePath, pattern string) bool {
	// Try exact match first
	if matched, _ := filepath.Match(pattern, filePath); matched {
		return true
	}

	// Handle ** glob patterns
	if strings.Contains(pattern, "**") {
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		if prefix == "" {
			// Pattern like **/*.controller.ts — match suffix against any file
			if suffix == "" {
				return true
			}
			fileName := filepath.Base(filePath)
			if matched, _ := filepath.Match(suffix, fileName); matched {
				return true
			}
		} else {
			// Pattern like src/**/*.controller.ts — find prefix in path, then match suffix
			searchStr := "/" + prefix + "/"
			idx := strings.Index(filePath, searchStr)
			if idx >= 0 {
				remaining := filePath[idx+len(searchStr):]
				if suffix == "" {
					return true
				}
				fileName := filepath.Base(remaining)
				if matched, _ := filepath.Match(suffix, fileName); matched {
					return true
				}
				if matched, _ := filepath.Match(suffix, remaining); matched {
					return true
				}
			}
		}
	} else {
		// No ** — try matching just the basename
		baseName := filepath.Base(filePath)
		patternBase := filepath.Base(pattern)
		if matched, _ := filepath.Match(patternBase, baseName); matched {
			return true
		}
	}

	return false
}
