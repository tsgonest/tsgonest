package constraints

import "github.com/tsgonest/tsgonest/internal/metadata"

// LiteralString extracts a string value from a literal metadata.
func LiteralString(m *metadata.Metadata) (string, bool) {
	if m.Kind == metadata.KindLiteral {
		if s, ok := m.LiteralValue.(string); ok {
			return s, true
		}
	}
	return "", false
}

// LiteralFloat extracts a float64 value from a literal metadata.
func LiteralFloat(m *metadata.Metadata) (float64, bool) {
	if m.Kind == metadata.KindLiteral {
		switch v := m.LiteralValue.(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		}
	}
	return 0, false
}

// LiteralInt extracts an int value from a literal metadata.
func LiteralInt(m *metadata.Metadata) (int, bool) {
	if m.Kind == metadata.KindLiteral {
		switch v := m.LiteralValue.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case int64:
			return int(v), true
		}
	}
	return 0, false
}

// LiteralBool extracts a boolean value from a literal metadata.
func LiteralBool(m *metadata.Metadata) (bool, bool) {
	if m.Kind == metadata.KindLiteral {
		if b, ok := m.LiteralValue.(bool); ok {
			return b, true
		}
	}
	return false, false
}
