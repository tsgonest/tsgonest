package sdkgen

import (
	"fmt"
	"sort"
	"strings"
)

// SchemaToTS converts a SchemaNode to a TypeScript type string.
func SchemaToTS(node *SchemaNode, visited map[string]bool) string {
	if node == nil {
		return "unknown"
	}

	// Handle $ref
	if node.Ref != "" {
		return node.Ref
	}

	// Handle enum
	if len(node.Enum) > 0 {
		return enumToTSUnion(node.Enum)
	}

	// Handle const
	if node.Const != nil {
		return constToTS(node.Const)
	}

	// Handle composition
	if len(node.AnyOf) > 0 {
		return compositionToTS(node.AnyOf, " | ", visited)
	}
	if len(node.OneOf) > 0 {
		return compositionToTS(node.OneOf, " | ", visited)
	}
	if len(node.AllOf) > 0 {
		return compositionToTS(node.AllOf, " & ", visited)
	}

	switch node.Type {
	case "string":
		if node.Format == "binary" {
			return "Blob"
		}
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "null":
		return "null"
	case "array":
		itemType := SchemaToTS(node.Items, visited)
		// If the item type is a composite that needs wrapping (but isn't already wrapped)
		if (strings.Contains(itemType, " | ") || strings.Contains(itemType, " & ")) && !strings.HasPrefix(itemType, "(") {
			return "(" + itemType + ")[]"
		}
		return itemType + "[]"
	case "object":
		if node.AdditionalProperties != nil {
			valType := SchemaToTS(node.AdditionalProperties, visited)
			return "Record<string, " + valType + ">"
		}
		if len(node.Properties) > 0 {
			return objectToInlineTS(node, visited)
		}
		return "Record<string, unknown>"
	default:
		return "unknown"
	}
}

func compositionToTS(nodes []*SchemaNode, sep string, visited map[string]bool) string {
	var parts []string
	hasNull := false
	for _, n := range nodes {
		ts := SchemaToTS(n, visited)
		if ts == "null" {
			hasNull = true
			continue
		}
		parts = append(parts, ts)
	}
	result := strings.Join(parts, sep)
	if len(parts) > 1 {
		result = "(" + result + ")"
	}
	if hasNull {
		result += " | null"
	}
	return result
}

func objectToInlineTS(node *SchemaNode, visited map[string]bool) string {
	requiredSet := make(map[string]bool)
	for _, r := range node.Required {
		requiredSet[r] = true
	}

	var names []string
	for name := range node.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []string
	for _, name := range names {
		prop := node.Properties[name]
		tsType := SchemaToTS(prop, visited)
		opt := "?"
		if requiredSet[name] {
			opt = ""
		}
		fields = append(fields, fmt.Sprintf("%s%s: %s", tsPropertyKey(name), opt, tsType))
	}

	return "{ " + strings.Join(fields, "; ") + " }"
}

// GenerateInterface emits a TypeScript interface for a named object schema.
func GenerateInterface(name string, node *SchemaNode, visited map[string]bool) string {
	if node == nil {
		return fmt.Sprintf("export type %s = unknown;\n", name)
	}

	// Non-object schemas: emit as type alias
	if node.Type != "object" || len(node.Properties) == 0 {
		ts := SchemaToTS(node, visited)
		var sb strings.Builder
		if node.Description != "" {
			sb.WriteString(buildSchemaJSDoc(node.Description))
		}
		fmt.Fprintf(&sb, "export type %s = %s;\n", name, ts)
		return sb.String()
	}

	requiredSet := make(map[string]bool)
	for _, r := range node.Required {
		requiredSet[r] = true
	}

	var names []string
	for name := range node.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	if node.Description != "" {
		sb.WriteString(buildSchemaJSDoc(node.Description))
	}
	fmt.Fprintf(&sb, "export interface %s {\n", name)
	for _, propName := range names {
		prop := node.Properties[propName]
		tsType := SchemaToTS(prop, visited)
		opt := "?"
		if requiredSet[propName] {
			opt = ""
		}
		if prop.Description != "" {
			sb.WriteString(buildPropertyJSDoc(prop.Description))
		}
		fmt.Fprintf(&sb, "  %s%s: %s;\n", tsPropertyKey(propName), opt, tsType)
	}
	sb.WriteString("}\n")
	return sb.String()
}

// tsPropertyKey returns a properly quoted TypeScript property key.
// Valid identifiers are returned as-is, while names containing spaces
// or other non-identifier characters are double-quoted.
func tsPropertyKey(name string) string {
	if len(name) == 0 {
		return `""`
	}
	for i, r := range name {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == '$') {
				return `"` + name + `"`
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '$') {
				return `"` + name + `"`
			}
		}
	}
	return name
}

// buildSchemaJSDoc generates a JSDoc comment for a schema type or interface.
func buildSchemaJSDoc(description string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 1 {
		return fmt.Sprintf("/** %s */\n", lines[0])
	}
	var sb strings.Builder
	sb.WriteString("/**\n")
	for _, line := range lines {
		if line == "" {
			sb.WriteString(" *\n")
		} else {
			fmt.Fprintf(&sb, " * %s\n", line)
		}
	}
	sb.WriteString(" */\n")
	return sb.String()
}

// buildPropertyJSDoc generates a JSDoc comment for a property within an interface.
func buildPropertyJSDoc(description string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 1 {
		return fmt.Sprintf("  /** %s */\n", lines[0])
	}
	var sb strings.Builder
	sb.WriteString("  /**\n")
	for _, line := range lines {
		if line == "" {
			sb.WriteString("   *\n")
		} else {
			fmt.Fprintf(&sb, "   * %s\n", line)
		}
	}
	sb.WriteString("   */\n")
	return sb.String()
}
