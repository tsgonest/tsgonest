package codegen

import (
	"fmt"
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- Helpers ---

func unionSaveVar(depth int) string {
	return fmt.Sprintf("_ue%d", depth)
}

func unionValidVar(depth int) string {
	return fmt.Sprintf("_uv%d", depth)
}

func jsLiteral(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// joinQuoted joins strings as JavaScript quoted values: "a", "b", "c"
func joinQuoted(keys []string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%q", k)
	}
	return strings.Join(parts, ", ")
}

// defaultToJSLiteral converts a @default string value to an appropriate JS literal.
// The type hint is used to determine parsing:
//   - string type: wraps in quotes ("hello")
//   - number type: outputs as-is (42, 3.14)
//   - boolean type: outputs true/false
//   - "null" string: outputs null
//   - otherwise: wraps in quotes as fallback
func defaultToJSLiteral(raw string, propType *metadata.Metadata) string {
	// Strip surrounding quotes from the raw value if present
	raw = stripDefaultQuotes(raw)

	// Handle explicit null
	if raw == "null" {
		return "null"
	}

	// Determine type from property metadata
	if propType != nil && propType.Kind == metadata.KindAtomic {
		switch propType.Atomic {
		case "number":
			// Validate it looks like a number, output as-is
			raw = strings.TrimSpace(raw)
			if _, err := fmt.Sscanf(raw, "%f", new(float64)); err == nil {
				return raw
			}
			return "0" // fallback for invalid number
		case "boolean":
			raw = strings.TrimSpace(strings.ToLower(raw))
			if raw == "true" {
				return "true"
			}
			return "false"
		}
	}

	// Default: treat as string
	return fmt.Sprintf("\"%s\"", jsStringEscape(raw))
}

// stripDefaultQuotes removes surrounding double or single quotes from a @default value.
func stripDefaultQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isJSIdentifier reports whether s is a valid JavaScript identifier that can
// be used in dot-notation property access (e.g., `obj.foo`). Names containing
// spaces, hyphens, or starting with a digit must use bracket notation instead.
func isJSIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == '$') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '$') {
				return false
			}
		}
	}
	return true
}

// jsPropAccess returns a JavaScript property access expression. It uses dot
// notation for valid identifiers (`obj.foo`) and bracket notation for names
// that are not valid identifiers (`obj["antall ansatte"]`).
// The special name `__proto__` always uses bracket notation to make the
// access explicit and avoid confusion with the prototype accessor.
func jsPropAccess(accessor, propName string) string {
	if propName == "__proto__" {
		return accessor + "[\"__proto__\"]"
	}
	if isJSIdentifier(propName) {
		return accessor + "." + propName
	}
	return accessor + "[\"" + jsStringEscape(propName) + "\"]"
}

// jsObjectKey returns a JavaScript object literal key. For valid identifiers
// it returns the name as-is (`foo`), for others it returns a quoted string
// (`"antall ansatte"`). The special name `__proto__` uses computed property
// name syntax (`["__proto__"]`) to avoid triggering the prototype setter
// in object literals.
func jsObjectKey(propName string) string {
	if propName == "__proto__" {
		return `["__proto__"]`
	}
	if isJSIdentifier(propName) {
		return propName
	}
	return "\"" + jsStringEscape(propName) + "\""
}

// jsPropPathSuffix returns the path suffix for a property name as it would
// appear in human-readable error paths. Valid identifiers use ".foo", while
// non-identifiers use `["antall ansatte"]`.
func jsPropPathSuffix(propName string) string {
	if propName == "__proto__" {
		return "[\"__proto__\"]"
	}
	if isJSIdentifier(propName) {
		return "." + propName
	}
	return "[\"" + jsStringEscape(propName) + "\"]"
}

// jsStringEscape escapes a string so it can be safely embedded inside a
// JavaScript double-quoted string literal. It handles backslashes, quotes,
// control characters (< 0x20), and Unicode line/paragraph separators.
func jsStringEscape(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			buf.WriteString(`\\`)
		case '"':
			buf.WriteString(`\"`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\u2028':
			buf.WriteString(`\u2028`)
		case '\u2029':
			buf.WriteString(`\u2029`)
		default:
			if r < 0x20 {
				buf.WriteString(fmt.Sprintf(`\x%02x`, r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

// jsonKeyInTemplate escapes a property name for embedding as a JSON object key
// inside a JavaScript template literal. The caller wraps the result with
// escaped-quote delimiters: `\"RESULT\":${...}`.
//
// Two layers of escaping are applied:
//  1. JSON escaping (for the JSON output): \ → \\, " → \", control chars → \uXXXX
//  2. Template literal escaping: \ → \\, ` → \`, ${ → \${
func jsonKeyInTemplate(s string) string {
	var buf strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\\':
			buf.WriteString(`\\\\`) // JSON: \\  → template: \\\\
		case '"':
			buf.WriteString(`\\\"`) // JSON: \"  → template: \\\"
		case '`':
			buf.WriteString("\\`") // template: \`
		case '\n':
			buf.WriteString(`\\n`) // JSON: \n  → template: \\n
		case '\r':
			buf.WriteString(`\\r`)
		case '\t':
			buf.WriteString(`\\t`)
		case '\b':
			buf.WriteString(`\\b`)
		case '\f':
			buf.WriteString(`\\f`)
		case '$':
			if i+1 < len(runes) && runes[i+1] == '{' {
				buf.WriteString(`\${`) // template: \${
				i++                    // skip '{'
			} else {
				buf.WriteRune(r)
			}
		default:
			if r < 0x20 {
				buf.WriteString(fmt.Sprintf(`\\u%04x`, r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

// jsonKeyInString escapes a property name for embedding as a JSON object key
// inside a JavaScript string literal. The caller wraps the result with
// escaped-quote delimiters: ",\"RESULT\":".
//
// Two layers of escaping are applied:
//  1. JSON escaping (for the JSON output): \ → \\, " → \", control chars → \uXXXX
//  2. JS string literal escaping: \ → \\, " → \"
func jsonKeyInString(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			buf.WriteString(`\\\\`) // JSON: \\  → JS str: \\\\
		case '"':
			buf.WriteString(`\\\"`) // JSON: \"  → JS str: \\\"
		case '\n':
			buf.WriteString(`\\n`) // JSON: \n  → JS str: \\n
		case '\r':
			buf.WriteString(`\\r`)
		case '\t':
			buf.WriteString(`\\t`)
		case '\b':
			buf.WriteString(`\\b`)
		case '\f':
			buf.WriteString(`\\f`)
		default:
			if r < 0x20 {
				buf.WriteString(fmt.Sprintf(`\\u%04x`, r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

// escapeForRegexLiteral escapes forward slashes in a regex pattern string
// so it can be safely embedded in a JavaScript regex literal (/pattern/).
// Already-escaped slashes (\/) are left unchanged.
func escapeForRegexLiteral(pattern string) string {
	var buf strings.Builder
	escaped := false
	for _, r := range pattern {
		if escaped {
			buf.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			buf.WriteRune(r)
			escaped = true
			continue
		}
		if r == '/' {
			buf.WriteString(`\/`)
			continue
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

func describeType(meta *metadata.Metadata) string {
	switch meta.Kind {
	case metadata.KindAtomic:
		return meta.Atomic
	case metadata.KindLiteral:
		return fmt.Sprintf("%v", meta.LiteralValue)
	case metadata.KindObject:
		if meta.Name != "" {
			return meta.Name
		}
		return "object"
	case metadata.KindArray:
		return "array"
	case metadata.KindTuple:
		return "tuple"
	case metadata.KindUnion:
		return describeUnion(meta)
	case metadata.KindRef:
		return meta.Ref
	case metadata.KindNative:
		return meta.NativeType
	default:
		return string(meta.Kind)
	}
}

func describeUnion(meta *metadata.Metadata) string {
	parts := make([]string, len(meta.UnionMembers))
	for i, m := range meta.UnionMembers {
		parts[i] = describeType(&m)
	}
	return strings.Join(parts, " | ")
}

// generateStandardSchemaWrapper generates a Standard Schema v1 wrapper object
// that wraps the validate function for cross-framework interoperability.
func generateStandardSchemaWrapper(e *Emitter, typeName string) {
	schemaName := "schema" + typeName
	validateFn := "validate" + typeName
	e.Line("// Standard Schema v1 wrapper")
	e.Line("export const %s = {", schemaName)
	e.Indent()
	e.Line("\"~standard\": {")
	e.Indent()
	e.Line("version: 1,")
	e.Line("vendor: \"tsgonest\",")
	e.Block("validate(value)")
	e.Line("const result = %s(value);", validateFn)
	e.Block("if (result.success)")
	e.Line("return { value: result.data };")
	e.EndBlock()
	e.Line("return {")
	e.Indent()
	e.Line("issues: result.errors.map(e => ({")
	e.Indent()
	e.Line("message: e.message || (\"Validation failed at \" + e.path),")
	e.Line("path: e.path ? e.path.split(\".\").map(k => ({ key: k })) : []")
	e.Dedent()
	e.Line("}))")
	e.Dedent()
	e.Line("};")
	e.EndBlock() // validate(value)
	e.Dedent()
	e.Line("}")
	e.Dedent()
	e.Line("};")
}
