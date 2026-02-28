package analyzer

import "testing"

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
