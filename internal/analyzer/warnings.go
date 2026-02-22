package analyzer

// Warning represents a diagnostic warning during analysis.
type Warning struct {
	// File is the source file path where the warning was raised.
	File string
	// Message is a human-readable description of the issue.
	Message string
	// Kind classifies the warning type: "query-complex-type", "header-null", "param-non-scalar".
	Kind string
}

// WarningCollector collects warnings during analysis.
type WarningCollector struct {
	Warnings []Warning
}

// NewWarningCollector creates a new, empty warning collector.
func NewWarningCollector() *WarningCollector {
	return &WarningCollector{}
}

// Add records a new warning.
func (wc *WarningCollector) Add(file, kind, message string) {
	wc.Warnings = append(wc.Warnings, Warning{File: file, Kind: kind, Message: message})
}
