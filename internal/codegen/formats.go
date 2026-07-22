package codegen

import "github.com/tsgonest/tsgonest/internal/formats"

// Format regex tables live in internal/formats so the rewrite package can
// share them without an import cycle (codegen imports rewrite).
var (
	formatRegexes = formats.Regexes
	formatFlags   = formats.Flags
)

// FormatNames returns all supported format names.
func FormatNames() []string {
	names := make([]string, 0, len(formatRegexes))
	for name := range formatRegexes {
		names = append(names, name)
	}
	return names
}
