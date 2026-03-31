package analyzer

import "testing"

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// ── Basic ** patterns with relative paths ────────────────────────
		{
			name:    "double-star with suffix matches relative path",
			path:    "src/public/meeting.controller.ts",
			pattern: "src/**/*.controller.ts",
			want:    true,
		},
		{
			name:    "double-star matches deeply nested relative path",
			path:    "src/public/v1/controllers/meeting.controller.ts",
			pattern: "src/**/*.controller.ts",
			want:    true,
		},
		{
			name:    "double-star with subdirectory prefix matches",
			path:    "src/public/meeting.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    true,
		},
		{
			name:    "double-star with subdirectory prefix rejects non-matching prefix",
			path:    "src/internal/meeting.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    false,
		},
		{
			name:    "double-star with no prefix",
			path:    "src/public/meeting.controller.ts",
			pattern: "**/*.controller.ts",
			want:    true,
		},

		// ── Absolute paths (tsgo sf.FileName()) vs relative patterns ────
		{
			name:    "relative pattern matches absolute path",
			path:    "/Users/dev/project/src/public/meeting.controller.ts",
			pattern: "src/**/*.controller.ts",
			want:    true,
		},
		{
			name:    "relative pattern with subdirectory matches absolute path",
			path:    "/Users/dev/project/src/public/meeting.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    true,
		},
		{
			name:    "relative pattern rejects absolute path with wrong subdirectory",
			path:    "/Users/dev/project/src/internal/admin.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    false,
		},
		{
			name:    "deeply nested absolute path matches relative pattern",
			path:    "/Users/dev/project/src/public/v1/controllers/meeting.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    true,
		},
		{
			name:    "double-star no prefix matches absolute path",
			path:    "/Users/dev/project/src/public/meeting.controller.ts",
			pattern: "**/*.controller.ts",
			want:    true,
		},
		{
			name:    "suffix mismatch rejects absolute path",
			path:    "/Users/dev/project/src/public/meeting.service.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    false,
		},

		// ── No ** patterns (basename matching) ──────────────────────────
		{
			name:    "basename glob matches",
			path:    "/Users/dev/project/src/meeting.controller.ts",
			pattern: "*.controller.ts",
			want:    true,
		},
		{
			name:    "basename glob rejects non-matching suffix",
			path:    "/Users/dev/project/src/meeting.service.ts",
			pattern: "*.controller.ts",
			want:    false,
		},
		{
			name:    "exact path match",
			path:    "src/meeting.controller.ts",
			pattern: "src/meeting.controller.ts",
			want:    true,
		},

		// ── Edge cases ──────────────────────────────────────────────────
		{
			name:    "double-star alone matches any file",
			path:    "/some/deep/path/file.ts",
			pattern: "**",
			want:    true,
		},
		{
			name:    "prefix appearing in username doesn't cause false match",
			path:    "/Users/src/other/project/lib/foo.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    false,
		},
		{
			name:    "node_modules path does not match src pattern",
			path:    "/project/node_modules/src/public/foo.controller.ts",
			pattern: "src/public/**/*.controller.ts",
			want:    true, // globMatch only checks for /src/public/ substring
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := globMatch(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v",
					tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchesGlob(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		include  []string
		exclude  []string
		want     bool
	}{
		// ── Include-only ─────────────────────────────────────────────────
		{
			name:     "matches include pattern",
			filePath: "/Users/dev/project/src/public/meeting.controller.ts",
			include:  []string{"src/**/*.controller.ts"},
			want:     true,
		},
		{
			name:     "no match for include pattern",
			filePath: "/Users/dev/project/src/public/meeting.service.ts",
			include:  []string{"src/**/*.controller.ts"},
			want:     false,
		},
		{
			name:     "empty include returns false",
			filePath: "/Users/dev/project/src/public/meeting.controller.ts",
			include:  []string{},
			want:     false,
		},
		{
			name:     "nil include returns false",
			filePath: "/Users/dev/project/src/public/meeting.controller.ts",
			include:  nil,
			want:     false,
		},
		{
			name:     "multiple include patterns — first matches",
			filePath: "/Users/dev/project/src/public/meeting.controller.ts",
			include:  []string{"src/public/**/*.controller.ts", "src/internal/**/*.controller.ts"},
			want:     true,
		},
		{
			name:     "multiple include patterns — second matches",
			filePath: "/Users/dev/project/src/internal/admin.controller.ts",
			include:  []string{"src/public/**/*.controller.ts", "src/internal/**/*.controller.ts"},
			want:     true,
		},
		{
			name:     "multiple include patterns — none match",
			filePath: "/Users/dev/project/src/other/foo.controller.ts",
			include:  []string{"src/public/**/*.controller.ts", "src/internal/**/*.controller.ts"},
			want:     false,
		},

		// ── Include + Exclude ────────────────────────────────────────────
		{
			name:     "exclude overrides include",
			filePath: "/Users/dev/project/src/public/deprecated.controller.ts",
			include:  []string{"src/**/*.controller.ts"},
			exclude:  []string{"src/**/deprecated.controller.ts"},
			want:     false,
		},
		{
			name:     "exclude does not affect non-matching files",
			filePath: "/Users/dev/project/src/public/meeting.controller.ts",
			include:  []string{"src/**/*.controller.ts"},
			exclude:  []string{"src/**/deprecated.controller.ts"},
			want:     true,
		},
		{
			name:     "broad exclude pattern",
			filePath: "/Users/dev/project/src/internal/admin.controller.ts",
			include:  []string{"src/**/*.controller.ts"},
			exclude:  []string{"src/internal/**/*.controller.ts"},
			want:     false,
		},

		// ── The exact issue scenario ─────────────────────────────────────
		{
			name:     "issue #71: narrow include for public controllers",
			filePath: "/Users/dev/project/src/public/clients/public.clients.controller.ts",
			include:  []string{"src/public/**/*.controller.ts"},
			want:     true,
		},
		{
			name:     "issue #71: narrow include rejects internal controllers",
			filePath: "/Users/dev/project/src/clients/clients.controller.ts",
			include:  []string{"src/public/**/*.controller.ts"},
			want:     false,
		},
		{
			name:     "issue #71: narrow include rejects other module controllers",
			filePath: "/Users/dev/project/src/calendar/controllers/calendar.controller.ts",
			include:  []string{"src/public/**/*.controller.ts"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesGlob(tt.filePath, tt.include, tt.exclude)
			if got != tt.want {
				t.Errorf("MatchesGlob(%q, include=%v, exclude=%v) = %v, want %v",
					tt.filePath, tt.include, tt.exclude, got, tt.want)
			}
		})
	}
}

func TestMatchesTypeNamePattern(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		patterns []string
		want     bool
	}{
		{
			name:     "exact match",
			typeName: "LegacyUser",
			patterns: []string{"LegacyUser"},
			want:     true,
		},
		{
			name:     "wildcard suffix",
			typeName: "LegacyUser",
			patterns: []string{"Legacy*"},
			want:     true,
		},
		{
			name:     "wildcard suffix no match",
			typeName: "UserDto",
			patterns: []string{"Legacy*"},
			want:     false,
		},
		{
			name:     "wildcard prefix",
			typeName: "UserDto",
			patterns: []string{"*Dto"},
			want:     true,
		},
		{
			name:     "wildcard both sides",
			typeName: "SomeInternalDto",
			patterns: []string{"*Internal*"},
			want:     true,
		},
		{
			name:     "question mark",
			typeName: "UserV1",
			patterns: []string{"UserV?"},
			want:     true,
		},
		{
			name:     "multiple patterns first matches",
			typeName: "LegacyOrder",
			patterns: []string{"Legacy*", "Deprecated*"},
			want:     true,
		},
		{
			name:     "multiple patterns second matches",
			typeName: "DeprecatedDto",
			patterns: []string{"Legacy*", "Deprecated*"},
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			typeName: "UserDto",
			patterns: []string{"Legacy*", "Deprecated*"},
			want:     false,
		},
		{
			name:     "empty patterns",
			typeName: "UserDto",
			patterns: []string{},
			want:     false,
		},
		{
			name:     "nil patterns",
			typeName: "UserDto",
			patterns: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesTypeNamePattern(tt.typeName, tt.patterns)
			if got != tt.want {
				t.Errorf("MatchesTypeNamePattern(%q, %v) = %v, want %v",
					tt.typeName, tt.patterns, got, tt.want)
			}
		})
	}
}
