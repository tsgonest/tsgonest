package diagnostic

import (
	"fmt"
	"strings"
)

// Severity represents the severity level of a diagnostic.
type Severity int

const (
	SeverityWarning Severity = iota
	SeverityError
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Category classifies diagnostics for filtering.
type Category string

const (
	CategoryTypeUnsupported   Category = "type-unsupported"
	CategoryConstraintInvalid Category = "constraint-invalid"
	CategoryParameterInvalid  Category = "parameter-invalid"
	CategoryConfigInvalid     Category = "config-invalid"
	CategoryOpenAPICompliance Category = "openapi-compliance"
	CategoryDeprecated        Category = "deprecated"
	CategoryPerformance       Category = "performance"
)

// Diagnostic represents a structured diagnostic message.
type Diagnostic struct {
	Severity Severity
	Category Category
	File     string // source file path
	Line     int    // 1-based line number (0 = unknown)
	Column   int    // 1-based column number (0 = unknown)
	Message  string
	Hint     string // optional suggestion for fixing the issue
}

// String formats the diagnostic for display.
func (d Diagnostic) String() string {
	var sb strings.Builder

	// File location
	if d.File != "" {
		sb.WriteString(d.File)
		if d.Line > 0 {
			sb.WriteString(fmt.Sprintf(":%d", d.Line))
			if d.Column > 0 {
				sb.WriteString(fmt.Sprintf(":%d", d.Column))
			}
		}
		sb.WriteString(" - ")
	}

	// Severity
	sb.WriteString(d.Severity.String())
	sb.WriteString(": ")

	// Category
	if d.Category != "" {
		sb.WriteString("[")
		sb.WriteString(string(d.Category))
		sb.WriteString("] ")
	}

	// Message
	sb.WriteString(d.Message)

	// Hint
	if d.Hint != "" {
		sb.WriteString("\n  hint: ")
		sb.WriteString(d.Hint)
	}

	return sb.String()
}

// Collector collects diagnostics during analysis.
type Collector struct {
	diagnostics []Diagnostic
	strict      bool // if true, warnings become errors
	quiet       bool // if true, suppress warnings
}

// NewCollector creates a new diagnostic collector.
func NewCollector(strict, quiet bool) *Collector {
	return &Collector{
		strict: strict,
		quiet:  quiet,
	}
}

// Warn adds a warning diagnostic.
func (c *Collector) Warn(category Category, file string, line int, message string) {
	if c == nil || c.quiet {
		return
	}
	sev := SeverityWarning
	if c.strict {
		sev = SeverityError
	}
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Severity: sev,
		Category: category,
		File:     file,
		Line:     line,
		Message:  message,
	})
}

// WarnWithHint adds a warning with a suggestion.
func (c *Collector) WarnWithHint(category Category, file string, line int, message, hint string) {
	if c == nil || c.quiet {
		return
	}
	sev := SeverityWarning
	if c.strict {
		sev = SeverityError
	}
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Severity: sev,
		Category: category,
		File:     file,
		Line:     line,
		Message:  message,
		Hint:     hint,
	})
}

// Error adds an error diagnostic.
func (c *Collector) Error(category Category, file string, line int, message string) {
	if c == nil {
		return
	}
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Severity: SeverityError,
		Category: category,
		File:     file,
		Line:     line,
		Message:  message,
	})
}

// Info adds an informational diagnostic.
func (c *Collector) Info(category Category, file string, line int, message string) {
	if c == nil || c.quiet {
		return
	}
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Severity: SeverityInfo,
		Category: category,
		File:     file,
		Line:     line,
		Message:  message,
	})
}

// Diagnostics returns all collected diagnostics.
func (c *Collector) Diagnostics() []Diagnostic {
	if c == nil {
		return nil
	}
	return c.diagnostics
}

// HasErrors returns true if any error-level diagnostics exist.
func (c *Collector) HasErrors() bool {
	if c == nil {
		return false
	}
	for _, d := range c.diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error diagnostics.
func (c *Collector) ErrorCount() int {
	if c == nil {
		return 0
	}
	count := 0
	for _, d := range c.diagnostics {
		if d.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning diagnostics.
func (c *Collector) WarningCount() int {
	if c == nil {
		return 0
	}
	count := 0
	for _, d := range c.diagnostics {
		if d.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// FormatAll formats all diagnostics as a multi-line string.
func (c *Collector) FormatAll() string {
	if c == nil || len(c.diagnostics) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, d := range c.diagnostics {
		sb.WriteString(d.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// Summary returns a summary line like "2 warning(s), 1 error(s)".
func (c *Collector) Summary() string {
	if c == nil {
		return ""
	}
	warnings := c.WarningCount()
	errors := c.ErrorCount()

	parts := []string{}
	if errors > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", errors))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
	}
	if len(parts) == 0 {
		return "no issues"
	}
	return strings.Join(parts, ", ")
}
