package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// extractConstraintValue sets a constraint field from a branded type literal value.
// Returns true if a constraint was successfully extracted.
func extractConstraintValue(c *metadata.Constraints, key string, typeMeta *metadata.Metadata) bool {
	switch key {
	// String constraints
	case "format":
		if s, ok := literalString(typeMeta); ok {
			c.Format = &s
			return true
		}
	case "minLength":
		if n, ok := literalInt(typeMeta); ok {
			c.MinLength = &n
			return true
		}
	case "maxLength":
		if n, ok := literalInt(typeMeta); ok {
			c.MaxLength = &n
			return true
		}
	case "pattern":
		if s, ok := literalString(typeMeta); ok {
			c.Pattern = &s
			return true
		}
	case "startsWith":
		if s, ok := literalString(typeMeta); ok {
			c.StartsWith = &s
			return true
		}
	case "endsWith":
		if s, ok := literalString(typeMeta); ok {
			c.EndsWith = &s
			return true
		}
	case "includes":
		if s, ok := literalString(typeMeta); ok {
			c.Includes = &s
			return true
		}

	// Numeric constraints
	case "minimum":
		if f, ok := literalFloat(typeMeta); ok {
			c.Minimum = &f
			return true
		}
	case "maximum":
		if f, ok := literalFloat(typeMeta); ok {
			c.Maximum = &f
			return true
		}
	case "exclusiveMinimum":
		if f, ok := literalFloat(typeMeta); ok {
			c.ExclusiveMinimum = &f
			return true
		}
	case "exclusiveMaximum":
		if f, ok := literalFloat(typeMeta); ok {
			c.ExclusiveMaximum = &f
			return true
		}
	case "multipleOf":
		if f, ok := literalFloat(typeMeta); ok {
			c.MultipleOf = &f
			return true
		}
	case "type":
		if s, ok := literalString(typeMeta); ok {
			c.NumericType = &s
			return true
		}

	// Array constraints
	case "minItems":
		if n, ok := literalInt(typeMeta); ok {
			c.MinItems = &n
			return true
		}
	case "maxItems":
		if n, ok := literalInt(typeMeta); ok {
			c.MaxItems = &n
			return true
		}
	case "uniqueItems":
		if b, ok := literalBool(typeMeta); ok && b {
			c.UniqueItems = &b
			return true
		}

	// String case validation
	case "uppercase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Uppercase = &b
			return true
		}
	case "lowercase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Lowercase = &b
			return true
		}

	// Transforms (applied before validation)
	case "transform_trim":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, "trim")
			return true
		}
	case "transform_toLowerCase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, "toLowerCase")
			return true
		}
	case "transform_toUpperCase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, "toUpperCase")
			return true
		}

	// Custom error message
	case "error":
		if s, ok := literalString(typeMeta); ok {
			c.ErrorMessage = &s
			return true
		}

	// Default value
	case "default":
		if s, ok := literalString(typeMeta); ok {
			c.Default = &s
			return true
		}
		// Also support numeric/boolean defaults as string representation
		if f, ok := literalFloat(typeMeta); ok {
			s := fmt.Sprintf("%v", f)
			c.Default = &s
			return true
		}
		if b, ok := literalBool(typeMeta); ok {
			s := fmt.Sprintf("%v", b)
			c.Default = &s
			return true
		}

	// Coercion (string→number, string→boolean, string→Date)
	case "coerce":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Coerce = &b
			return true
		}
	}
	return false
}

// literalString extracts a string value from a literal metadata.
func literalString(m *metadata.Metadata) (string, bool) {
	if m.Kind == metadata.KindLiteral {
		if s, ok := m.LiteralValue.(string); ok {
			return s, true
		}
	}
	return "", false
}

// literalFloat extracts a float64 value from a literal metadata.
func literalFloat(m *metadata.Metadata) (float64, bool) {
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

// literalInt extracts an int value from a literal metadata.
func literalInt(m *metadata.Metadata) (int, bool) {
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

// literalBool extracts a boolean value from a literal metadata.
func literalBool(m *metadata.Metadata) (bool, bool) {
	if m.Kind == metadata.KindLiteral {
		if b, ok := m.LiteralValue.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// mergeConstraints merges src constraints into dst. src values take precedence
// (override dst values when both are set).
func mergeConstraints(dst, src *metadata.Constraints) {
	if src.Minimum != nil {
		dst.Minimum = src.Minimum
	}
	if src.Maximum != nil {
		dst.Maximum = src.Maximum
	}
	if src.ExclusiveMinimum != nil {
		dst.ExclusiveMinimum = src.ExclusiveMinimum
	}
	if src.ExclusiveMaximum != nil {
		dst.ExclusiveMaximum = src.ExclusiveMaximum
	}
	if src.MultipleOf != nil {
		dst.MultipleOf = src.MultipleOf
	}
	if src.NumericType != nil {
		dst.NumericType = src.NumericType
	}
	if src.MinLength != nil {
		dst.MinLength = src.MinLength
	}
	if src.MaxLength != nil {
		dst.MaxLength = src.MaxLength
	}
	if src.Pattern != nil {
		dst.Pattern = src.Pattern
	}
	if src.Format != nil {
		dst.Format = src.Format
	}
	if src.StartsWith != nil {
		dst.StartsWith = src.StartsWith
	}
	if src.EndsWith != nil {
		dst.EndsWith = src.EndsWith
	}
	if src.Includes != nil {
		dst.Includes = src.Includes
	}
	if src.Uppercase != nil {
		dst.Uppercase = src.Uppercase
	}
	if src.Lowercase != nil {
		dst.Lowercase = src.Lowercase
	}
	if src.ContentMediaType != nil {
		dst.ContentMediaType = src.ContentMediaType
	}
	if len(src.Transforms) > 0 {
		dst.Transforms = src.Transforms
	}
	if src.MinItems != nil {
		dst.MinItems = src.MinItems
	}
	if src.MaxItems != nil {
		dst.MaxItems = src.MaxItems
	}
	if src.UniqueItems != nil {
		dst.UniqueItems = src.UniqueItems
	}
	if src.Default != nil {
		dst.Default = src.Default
	}
	if src.Coerce != nil {
		dst.Coerce = src.Coerce
	}
	if src.ValidateFn != nil {
		dst.ValidateFn = src.ValidateFn
	}
	if src.ValidateModule != nil {
		dst.ValidateModule = src.ValidateModule
	}
	if src.ErrorMessage != nil {
		dst.ErrorMessage = src.ErrorMessage
	}
}

// extractJSDocConstraints parses JSDoc tags on a declaration node to extract
// validation constraints like @minimum, @maximum, @minLength, @maxLength,
// @pattern, @format, @minItems, @maxItems.
func (w *TypeWalker) extractJSDocConstraints(node *ast.Node) *metadata.Constraints {
	if node == nil {
		return nil
	}

	// Get JSDoc comments attached to this node
	// JSDoc() takes a *SourceFile parameter; pass nil to search from the node directly
	jsdocs := node.JSDoc(nil)
	if len(jsdocs) == 0 {
		return nil
	}

	var c metadata.Constraints
	found := false

	// Process the last JSDoc comment (the most recent one)
	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()
	if jsdoc.Tags == nil {
		return nil
	}

	for _, tagNode := range jsdoc.Tags.Nodes {
		// Custom tags (@minimum, @maxLength, etc.) are KindJSDocTag (unknown tags)
		// We need to safely get the tag name and comment.
		tagName, comment := extractJSDocTagInfo(tagNode)
		if tagName == "" {
			continue
		}
		tagName = strings.ToLower(tagName)

		switch tagName {
		case "minimum", "min":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.Minimum = &v
				found = true
			}
		case "maximum", "max":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.Maximum = &v
				found = true
			}
		case "minlength":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinLength = &v
				found = true
			}
		case "maxlength":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MaxLength = &v
				found = true
			}
		case "len", "length":
			// Shorthand: sets both minLength and maxLength to the same value
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinLength = &v
				c.MaxLength = &v
				found = true
			}
		case "items":
			// Shorthand: sets both minItems and maxItems to the same value
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinItems = &v
				c.MaxItems = &v
				found = true
			}
		case "pattern":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.Pattern = &s
				found = true
			}
		case "format":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.Format = &s
				found = true
			}
		case "exclusiveminimum":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.ExclusiveMinimum = &v
				found = true
			}
		case "exclusivemaximum":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.ExclusiveMaximum = &v
				found = true
			}
		case "multipleof":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.MultipleOf = &v
				found = true
			}
		case "type", "numerictype":
			s := strings.TrimSpace(comment)
			validTypes := map[string]bool{
				"int32": true, "uint32": true, "int64": true,
				"uint64": true, "float": true, "double": true,
			}
			if validTypes[s] {
				c.NumericType = &s
				found = true
			}
		case "minitems":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinItems = &v
				found = true
			}
		case "maxitems":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MaxItems = &v
				found = true
			}
		case "uniqueitems":
			b := true
			c.UniqueItems = &b
			found = true
		case "default":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.Default = &s
				found = true
			}
		case "contentmediatype":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.ContentMediaType = &s
				found = true
			}

		// --- Shorthand tags (Zod parity) ---
		case "positive":
			v := float64(0)
			c.ExclusiveMinimum = &v
			found = true
		case "negative":
			v := float64(0)
			c.ExclusiveMaximum = &v
			found = true
		case "nonnegative":
			v := float64(0)
			c.Minimum = &v
			found = true
		case "nonpositive":
			v := float64(0)
			c.Maximum = &v
			found = true
		case "int":
			s := "int32"
			c.NumericType = &s
			found = true
		case "safe":
			s := "int64"
			c.NumericType = &s
			found = true
		case "finite":
			s := "float"
			c.NumericType = &s
			found = true

		// --- String transform tags ---
		case "trim":
			c.Transforms = append(c.Transforms, "trim")
			found = true
		case "tolowercase":
			c.Transforms = append(c.Transforms, "toLowerCase")
			found = true
		case "touppercase":
			c.Transforms = append(c.Transforms, "toUpperCase")
			found = true

		// --- String content checks ---
		case "startswith":
			s := strings.TrimSpace(comment)
			// Remove surrounding quotes if present
			s = stripQuotes(s)
			if s != "" {
				c.StartsWith = &s
				found = true
			}
		case "endswith":
			s := strings.TrimSpace(comment)
			s = stripQuotes(s)
			if s != "" {
				c.EndsWith = &s
				found = true
			}
		case "includes":
			s := strings.TrimSpace(comment)
			s = stripQuotes(s)
			if s != "" {
				c.Includes = &s
				found = true
			}
		case "uppercase":
			b := true
			c.Uppercase = &b
			found = true
		case "lowercase":
			b := true
			c.Lowercase = &b
			found = true

		// --- Custom error messages ---
		case "error":
			s := strings.TrimSpace(comment)
			s = stripQuotes(s)
			if s != "" {
				c.ErrorMessage = &s
				found = true
			}

		// --- Coercion ---
		case "coerce":
			b := true
			c.Coerce = &b
			found = true
		}
	}

	if !found {
		return nil
	}
	return &c
}

// extractJSDocTagInfo safely extracts the tag name and comment from a JSDoc tag node.
// Custom tags (like @minimum, @maxLength) are KindJSDocTag (unknown tags), while
// known tags (like @param, @returns) have specific kinds.
func extractJSDocTagInfo(tagNode *ast.Node) (tagName string, comment string) {
	if tagNode == nil {
		return "", ""
	}

	// For unknown/custom tags (KindJSDocTag), use AsJSDocUnknownTag
	if tagNode.Kind == ast.KindJSDocTag {
		unknownTag := tagNode.AsJSDocUnknownTag()
		if unknownTag == nil || unknownTag.TagName == nil {
			return "", ""
		}
		tagName = unknownTag.TagName.Text()
		if unknownTag.Comment != nil {
			comment = extractNodeListText(unknownTag.Comment)
		}
		return tagName, comment
	}

	// For known tags that implement JSDocTagBase pattern
	// Try to access via the common tag structure
	// All JSDoc tags have a TagName child — we can search for it
	if tagNode.Kind == ast.KindJSDocDeprecatedTag {
		// @deprecated — not a constraint, skip
		return "deprecated", ""
	}

	// Handle @type tag — TypeScript recognizes it as KindJSDocTypeTag.
	// We extract the type name text (e.g., "int32") from the type expression.
	// When user writes `@type {int32}`, TS parses `int32` as a TypeReference inside JSDocTypeExpression.
	// When user writes `@type int32` (without braces), TS may parse differently.
	if tagNode.Kind == ast.KindJSDocTypeTag {
		typeTag := tagNode.AsJSDocTypeTag()
		if typeTag != nil {
			// Try to extract from comment first (when TS doesn't parse it as type expression)
			if typeTag.Comment != nil {
				text := extractNodeListText(typeTag.Comment)
				if text != "" {
					return "type", text
				}
			}
			// Try to extract from the type expression's inner type reference name
			if typeTag.TypeExpression != nil {
				typeExpr := typeTag.TypeExpression.AsJSDocTypeExpression()
				if typeExpr != nil && typeExpr.Type != nil {
					// The inner type should be a TypeReference with an identifier name
					if typeExpr.Type.Kind == ast.KindTypeReference {
						ref := typeExpr.Type.AsTypeReferenceNode()
						if ref != nil && ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier {
							return "type", ref.TypeName.Text()
						}
					}
					// Fallback: try identifier directly
					if typeExpr.Type.Kind == ast.KindIdentifier {
						return "type", typeExpr.Type.Text()
					}
				}
			}
		}
		return "type", ""
	}

	// For other known tag kinds, try to get TagName from the first identifier child
	// This is a safe fallback
	return "", ""
}

// extractTypeLevelAnnotations parses JSDoc tags on a type declaration to extract
// type-level annotations like @strict, @passthrough, @strip, @tsgonest-ignore.
func (w *TypeWalker) extractTypeLevelAnnotations(node *ast.Node) (strictness string, ignore string) {
	if node == nil {
		return "", ""
	}

	jsdocs := node.JSDoc(nil)
	if len(jsdocs) == 0 {
		return "", ""
	}

	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()
	if jsdoc.Tags == nil {
		return "", ""
	}

	for _, tagNode := range jsdoc.Tags.Nodes {
		tagName, _ := extractJSDocTagInfo(tagNode)
		if tagName == "" {
			continue
		}
		tagName = strings.ToLower(tagName)

		switch tagName {
		case "strict":
			strictness = "strict"
		case "passthrough":
			strictness = "passthrough"
		case "strip":
			strictness = "strip"
		case "tsgonest-ignore":
			ignore = "all"
		case "tsgonest-ignore-validation":
			ignore = "validation"
		case "tsgonest-ignore-serialization":
			ignore = "serialization"
		}
	}
	return strictness, ignore
}

// stripQuotes removes surrounding double or single quotes from a string.
// e.g., `"hello"` → `hello`, `'world'` → `world`
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// propertyAnnotations holds OpenAPI-only annotations extracted from a property's JSDoc.
// These are separate from validation constraints — they only affect the schema output.
type propertyAnnotations struct {
	Description string
	WriteOnly   bool
	Example     *string
}

// extractPropertyAnnotations extracts OpenAPI-relevant JSDoc annotations from a property declaration:
//   - @description <text> — property description in the schema
//   - @writeOnly — marks the property as write-only in the schema
//   - @example <value> — example value in the schema
//
// Only explicit tags are used — JSDoc body text is NOT extracted.
func extractPropertyAnnotations(node *ast.Node) propertyAnnotations {
	var ann propertyAnnotations
	if node == nil {
		return ann
	}
	// Walk up to find JSDoc (it may be on the parent VariableStatement or PropertySignature)
	for n := node; n != nil; n = n.Parent {
		jsdocs := n.JSDoc(nil)
		if len(jsdocs) > 0 {
			jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()
			if jsdoc.Tags != nil {
				for _, tagNode := range jsdoc.Tags.Nodes {
					tagName, comment := extractJSDocTagInfo(tagNode)
					switch strings.ToLower(tagName) {
					case "description":
						ann.Description = strings.TrimSpace(comment)
					case "writeonly":
						ann.WriteOnly = true
					case "example":
						v := strings.TrimSpace(comment)
						if v != "" {
							ann.Example = &v
						}
					}
				}
			}
			return ann // Found JSDoc, don't walk further
		}
		// Stop walking at statement-level nodes
		if n.Kind == ast.KindPropertySignature || n.Kind == ast.KindPropertyDeclaration ||
			n.Kind == ast.KindVariableStatement || n.Kind == ast.KindClassDeclaration {
			break
		}
	}
	return ann
}

// extractNodeListText concatenates text from a NodeList of JSDoc text/link nodes.
func extractNodeListText(nodeList *ast.NodeList) string {
	if nodeList == nil {
		return ""
	}
	var parts []string
	for _, commentNode := range nodeList.Nodes {
		switch commentNode.Kind {
		case ast.KindJSDocText:
			parts = append(parts, commentNode.Text())
		case ast.KindJSDocLink, ast.KindJSDocLinkCode, ast.KindJSDocLinkPlain:
			parts = append(parts, commentNode.Text())
		}
	}
	return strings.Join(parts, "")
}
