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

	result := schemaToTSCore(node, visited)

	// Append nullable suffix. Skip if the composition already includes null
	// (e.g., anyOf: [string, null]) to avoid double "| null".
	if node.Nullable && !strings.HasSuffix(result, "| null") {
		if strings.Contains(result, " | ") || strings.Contains(result, " & ") {
			result = "(" + result + ") | null"
		} else {
			result += " | null"
		}
	}

	return result
}

// schemaToTSCore handles the core type conversion without nullable wrapping.
func schemaToTSCore(node *SchemaNode, visited map[string]bool) string {
	// Handle $ref
	if node.Ref != "" {
		return node.Ref
	}

	// Handle enum
	if len(node.Enum) > 0 {
		return enumToTSUnion(node.Enum)
	}

	// Handle const
	if node.HasConst {
		return constToTS(node.Const)
	}

	// Handle composition
	if len(node.AnyOf) > 0 {
		return compositionToTS(node.AnyOf, " | ", visited)
	}
	if len(node.OneOf) > 0 {
		if node.Discriminator != nil && node.Discriminator.PropertyName != "" {
			return discriminatedUnionToTS(node, visited)
		}
		return compositionToTS(node.OneOf, " | ", visited)
	}
	if len(node.AllOf) > 0 {
		return compositionToTS(node.AllOf, " & ", visited)
	}

	switch node.Type {
	case "string":
		if node.Format == "binary" {
			return "File | Blob"
		}
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "null":
		return "null"
	case "array":
		// Tuple: explicit prefixItems
		if len(node.PrefixItems) > 0 {
			var elements []string
			for _, item := range node.PrefixItems {
				elements = append(elements, SchemaToTS(item, visited))
			}
			if node.Items != nil {
				rest := SchemaToTS(node.Items, visited)
				elements = append(elements, "..."+rest+"[]")
			}
			return "[" + strings.Join(elements, ", ") + "]"
		}
		// Tuple: homogeneous array with minItems == maxItems (capped at 10)
		if node.MinItems != nil && node.MaxItems != nil &&
			*node.MinItems == *node.MaxItems &&
			*node.MinItems > 0 && *node.MinItems <= 10 {
			itemType := SchemaToTS(node.Items, visited)
			elements := make([]string, *node.MinItems)
			for i := range elements {
				elements[i] = itemType
			}
			return "[" + strings.Join(elements, ", ") + "]"
		}
		// Regular array
		itemType := SchemaToTS(node.Items, visited)
		if (strings.Contains(itemType, " | ") || strings.Contains(itemType, " & ")) && !strings.HasPrefix(itemType, "(") {
			return "(" + itemType + ")[]"
		}
		return itemType + "[]"
	case "object":
		if len(node.Properties) > 0 {
			return objectToInlineTS(node, visited)
		}
		if node.AdditionalProperties != nil {
			valType := SchemaToTS(node.AdditionalProperties, visited)
			return "Record<string, " + valType + ">"
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

// discriminatedUnionToTS generates a TypeScript discriminated union type.
// For each oneOf variant with a known discriminator value, it emits:
//
//	(Omit<VariantType, 'discProp'> & { discProp: "value" })
//
// This ensures TypeScript's control-flow narrowing works via switch/if on the
// discriminator property, even if the source schemas don't constrain it to a literal.
func discriminatedUnionToTS(node *SchemaNode, visited map[string]bool) string {
	disc := node.Discriminator
	var parts []string
	hasNull := false

	// Build reverse mapping: $ref name → discriminator value
	refToValue := make(map[string]string)
	if disc.Mapping != nil {
		for val, ref := range disc.Mapping {
			refName := strings.TrimPrefix(ref, "#/components/schemas/")
			refToValue[refName] = val
		}
	}

	for _, variant := range node.OneOf {
		if variant.Type == "null" {
			hasNull = true
			continue
		}

		variantTS := SchemaToTS(variant, visited)
		discValue := refToValue[variant.Ref]

		if discValue != "" {
			parts = append(parts, fmt.Sprintf(
				"(Omit<%s, %q> & { %s: %q })",
				variantTS, disc.PropertyName,
				tsPropertyKey(disc.PropertyName), discValue,
			))
		} else {
			parts = append(parts, variantTS)
		}
	}

	result := strings.Join(parts, " | ")
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
		readonlyMod := ""
		if prop.ReadOnly {
			readonlyMod = "readonly "
		}
		fields = append(fields, fmt.Sprintf("%s%s%s: %s", readonlyMod, tsPropertyKey(name), opt, tsType))
	}

	if node.AdditionalProperties != nil {
		valType := SchemaToTS(node.AdditionalProperties, visited)
		fields = append(fields, fmt.Sprintf("[key: string]: %s | undefined", valType))
	}

	return "{ " + strings.Join(fields, "; ") + " }"
}

// GenerateInterface emits a TypeScript interface for a named object schema.
func GenerateInterface(name string, node *SchemaNode, visited map[string]bool) string {
	if node == nil {
		return fmt.Sprintf("export type %s = unknown;\n", name)
	}

	// Object with additionalProperties but no declared properties:
	// emit as interface with index signature instead of Record<> type alias.
	// Interfaces support self-referential types (e.g., JsonObject where
	// additionalProperties references JsonValue which includes JsonObject),
	// while type aliases would cause TS2456: "Type alias circularly references itself".
	if node.Type == "object" && len(node.Properties) == 0 && node.AdditionalProperties != nil {
		valType := SchemaToTS(node.AdditionalProperties, visited)
		var sb strings.Builder
		if node.Description != "" {
			sb.WriteString(buildSchemaJSDoc(node.Description))
		}
		fmt.Fprintf(&sb, "export interface %s { [key: string]: %s }\n", name, valType)
		return sb.String()
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
		jsdoc := buildPropertyJSDocWithMeta(prop.Description, prop.Deprecated, prop.HasDefault, prop.Default)
		if jsdoc != "" {
			sb.WriteString(jsdoc)
		}
		readonlyMod := ""
		if prop.ReadOnly {
			readonlyMod = "readonly "
		}
		fmt.Fprintf(&sb, "  %s%s%s: %s;\n", readonlyMod, tsPropertyKey(propName), opt, tsType)
	}
	if node.AdditionalProperties != nil {
		valType := SchemaToTS(node.AdditionalProperties, visited)
		fmt.Fprintf(&sb, "  [key: string]: %s | undefined;\n", valType)
	}
	sb.WriteString("}\n")
	return sb.String()
}

// isJSIdentifier reports whether name is a valid JavaScript/TypeScript identifier
// (ASCII subset: [a-zA-Z_$] followed by [a-zA-Z0-9_$]).
func isJSIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, r := range name {
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

// escapeJSString escapes a string for safe embedding inside a JS double-quoted
// string literal. Handles backslashes, double quotes, and control characters.
func escapeJSString(s string) string {
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

// tsPropertyKey returns a properly quoted TypeScript property key.
// Valid identifiers are returned as-is, while names containing spaces
// or other non-identifier characters are double-quoted and escaped.
func tsPropertyKey(name string) string {
	if isJSIdentifier(name) {
		return name
	}
	return `"` + escapeJSString(name) + `"`
}

// tsPropAccess returns a TypeScript property access expression.
// Valid identifiers use dot notation (obj.name), non-identifiers
// use bracket notation (obj["first.name"]).
func tsPropAccess(accessor, name string) string {
	if isJSIdentifier(name) {
		return accessor + "." + name
	}
	return accessor + `["` + escapeJSString(name) + `"]`
}

// tsOptionalAccess returns a TypeScript optional chaining property access.
// Valid identifiers use obj?.name, non-identifiers use obj?.["first.name"].
func tsOptionalAccess(accessor, name string) string {
	if isJSIdentifier(name) {
		return accessor + "?." + name
	}
	return accessor + `?.["` + escapeJSString(name) + `"]`
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

// buildPropertyJSDocWithMeta generates a JSDoc comment for a property,
// including @deprecated and @default tags when present.
func buildPropertyJSDocWithMeta(description string, deprecated bool, hasDefault bool, defaultVal any) string {
	var parts []string
	if description != "" {
		parts = append(parts, description)
	}
	if deprecated {
		parts = append(parts, "@deprecated")
	}
	if hasDefault {
		parts = append(parts, fmt.Sprintf("@default %s", formatJSDocDefault(defaultVal)))
	}
	if len(parts) == 0 {
		return ""
	}
	return buildPropertyJSDoc(strings.Join(parts, "\n"))
}

// formatJSDocDefault serializes a default value for a @default JSDoc tag.
func formatJSDocDefault(val any) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
