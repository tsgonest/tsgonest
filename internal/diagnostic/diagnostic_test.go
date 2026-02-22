package diagnostic

import (
	"strings"
	"testing"
)

func TestDiagnostic_String(t *testing.T) {
	d := Diagnostic{
		Severity: SeverityWarning,
		Category: CategoryTypeUnsupported,
		File:     "src/user.dto.ts",
		Line:     10,
		Column:   5,
		Message:  "type 'Map<string, any>' is not fully supported",
		Hint:     "use Record<string, any> instead",
	}

	s := d.String()
	if !strings.Contains(s, "src/user.dto.ts:10:5") {
		t.Errorf("expected file:line:col, got %q", s)
	}
	if !strings.Contains(s, "warning") {
		t.Errorf("expected 'warning', got %q", s)
	}
	if !strings.Contains(s, "[type-unsupported]") {
		t.Errorf("expected category, got %q", s)
	}
	if !strings.Contains(s, "hint:") {
		t.Errorf("expected hint, got %q", s)
	}
}

func TestCollector_WarnAndError(t *testing.T) {
	c := NewCollector(false, false)
	c.Warn(CategoryConstraintInvalid, "test.ts", 5, "invalid constraint")
	c.Error(CategoryConfigInvalid, "", 0, "missing config field")

	if c.WarningCount() != 1 {
		t.Errorf("expected 1 warning, got %d", c.WarningCount())
	}
	if c.ErrorCount() != 1 {
		t.Errorf("expected 1 error, got %d", c.ErrorCount())
	}
	if !c.HasErrors() {
		t.Error("expected HasErrors() = true")
	}
}

func TestCollector_StrictMode(t *testing.T) {
	c := NewCollector(true, false) // strict mode
	c.Warn(CategoryTypeUnsupported, "test.ts", 1, "unsupported type")

	// In strict mode, warnings become errors
	if c.ErrorCount() != 1 {
		t.Errorf("expected 1 error (strict mode), got %d", c.ErrorCount())
	}
	if c.WarningCount() != 0 {
		t.Errorf("expected 0 warnings (strict mode), got %d", c.WarningCount())
	}
}

func TestCollector_QuietMode(t *testing.T) {
	c := NewCollector(false, true) // quiet mode
	c.Warn(CategoryTypeUnsupported, "test.ts", 1, "unsupported type")
	c.Info(CategoryPerformance, "test.ts", 1, "slow operation")
	c.Error(CategoryConfigInvalid, "", 0, "real error") // errors still show

	if len(c.Diagnostics()) != 1 {
		t.Errorf("expected 1 diagnostic (only error), got %d", len(c.Diagnostics()))
	}
}

func TestCollector_Summary(t *testing.T) {
	c := NewCollector(false, false)
	c.Warn(CategoryConstraintInvalid, "a.ts", 1, "warn1")
	c.Warn(CategoryConstraintInvalid, "b.ts", 2, "warn2")
	c.Error(CategoryConfigInvalid, "", 0, "err1")

	summary := c.Summary()
	if !strings.Contains(summary, "1 error") {
		t.Errorf("expected '1 error' in summary, got %q", summary)
	}
	if !strings.Contains(summary, "2 warning") {
		t.Errorf("expected '2 warning' in summary, got %q", summary)
	}
}

func TestCollector_NilSafe(t *testing.T) {
	var c *Collector
	// Should not panic
	c.Warn(CategoryTypeUnsupported, "", 0, "test")
	c.Error(CategoryConfigInvalid, "", 0, "test")
	if c.HasErrors() {
		t.Error("nil collector should not have errors")
	}
	if c.Summary() != "" {
		t.Error("nil collector should return empty summary")
	}
}

func TestCollector_FormatAll(t *testing.T) {
	c := NewCollector(false, false)
	c.Warn(CategoryTypeUnsupported, "test.ts", 10, "type not supported")

	formatted := c.FormatAll()
	if !strings.Contains(formatted, "test.ts:10") {
		t.Errorf("expected formatted output with file:line, got %q", formatted)
	}
}

func TestCollector_WarnWithHint(t *testing.T) {
	c := NewCollector(false, false)
	c.WarnWithHint(CategoryTypeUnsupported, "test.ts", 5, "Map not supported", "use Record instead")

	diags := c.Diagnostics()
	if len(diags) != 1 || diags[0].Hint != "use Record instead" {
		t.Errorf("expected hint, got %v", diags)
	}
}
