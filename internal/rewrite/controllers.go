package rewrite

import (
	"regexp"
	"strings"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// rewriteController injects @Body() parameter validation and return value
// transformation into a controller file's emitted JS.
// For body params: inserts `paramName = assertTypeName(paramName);` at method start.
// For return values: wraps `return EXPR;` with `return transformTypeName(await EXPR);`.
func rewriteController(text string, outputFile string, controllers []analyzer.ControllerInfo, companionMap map[string]string, moduleFormat string) string {
	// Collect all body parameters with named types from matching controllers
	type bodyValidation struct {
		methodName string
		paramName  string
		typeName   string
	}

	// Collect return transformations
	type returnTransform struct {
		methodName    string
		typeName      string
		isArray       bool
	}

	var validations []bodyValidation
	var transforms []returnTransform
	neededTypes := make(map[string]bool)
	neededTransformTypes := make(map[string]bool)

	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			// Skip raw response routes
			if route.UsesRawResponse {
				continue
			}

			// Body validation collection
			for _, param := range route.Parameters {
				if param.Category != "body" {
					continue
				}
				// Skip validation for multipart/form-data — multer handles parsing
				if param.ContentType == "multipart/form-data" {
					continue
				}
				typeName := resolveParamTypeName(&param)
				if typeName == "" {
					continue
				}
				// Only inject if we have a companion for this type
				if _, ok := companionMap[typeName]; !ok {
					continue
				}
				// Use LocalName (the TS variable name) for injection.
				// Fall back to decorator arg Name, then to findBodyParamName.
				paramName := param.LocalName
				if paramName == "" {
					paramName = param.Name
				}
				if paramName == "" {
					paramName = findBodyParamName(text, route.OperationID)
					if paramName == "" {
						continue
					}
				}
				validations = append(validations, bodyValidation{
					methodName: route.OperationID,
					paramName:  paramName,
					typeName:   typeName,
				})
				neededTypes[typeName] = true
			}

			// Return transform collection
			if route.IsSSE {
				continue
			}
			returnTypeName := resolveReturnTypeName(&route.ReturnType)
			if returnTypeName == "" {
				continue
			}
			// Only transform if we have a companion for the return type
			if _, ok := companionMap[returnTypeName]; !ok {
				continue
			}
			isArray := route.ReturnType.Kind == metadata.KindArray
			transforms = append(transforms, returnTransform{
				methodName: route.OperationID,
				typeName:   returnTypeName,
				isArray:    isArray,
			})
			neededTransformTypes[returnTypeName] = true
		}
	}

	if len(validations) == 0 && len(transforms) == 0 {
		return text
	}

	// Inject validation calls into method bodies
	for _, v := range validations {
		assertFunc := companionFuncName("assert", v.typeName)
		assertLine := "    " + v.paramName + " = " + assertFunc + "(" + v.paramName + ");"
		text = injectAtMethodStart(text, v.methodName, assertLine)
	}

	// Wrap return statements with transform calls
	for _, tr := range transforms {
		transformFunc := companionFuncName("transform", tr.typeName)
		text = wrapReturnsInMethod(text, tr.methodName, transformFunc, tr.isArray)
	}

	// Generate companion imports for the types we need
	var markerCalls []MarkerCall
	for typeName := range neededTypes {
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "assert",
			TypeName:     typeName,
		})
	}
	for typeName := range neededTransformTypes {
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "transform",
			TypeName:     typeName,
		})
	}
	importLines := companionImports(markerCalls, companionMap, outputFile, moduleFormat)

	if len(importLines) > 0 {
		// Insert imports at top of file (after sentinel if present)
		text = strings.Join(importLines, "\n") + "\n" + text
	}

	return text
}

// resolveParamTypeName extracts the type name from a route parameter's metadata.
func resolveParamTypeName(param *analyzer.RouteParameter) string {
	m := &param.Type
	if m.Name != "" {
		return m.Name
	}
	if m.Ref != "" {
		return m.Ref
	}
	return ""
}

// resolveReturnTypeName extracts the DTO type name from a route's return type metadata.
// For arrays, it returns the element type name.
// Returns empty string for primitive/any/void types.
func resolveReturnTypeName(m *metadata.Metadata) string {
	switch m.Kind {
	case metadata.KindRef:
		return m.Ref
	case metadata.KindObject:
		return m.Name
	case metadata.KindArray:
		if m.ElementType != nil {
			return resolveReturnTypeName(m.ElementType)
		}
	}
	return ""
}

// findBodyParamName finds the parameter name for an unnamed @Body() decorator
// by looking at the method signature in the emitted JS.
func findBodyParamName(text string, methodName string) string {
	// Look for: async methodName(paramName or methodName(paramName
	pattern := regexp.MustCompile(`(?:async\s+)?` + regexp.QuoteMeta(methodName) + `\s*\(([^,)]+)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// injectAtMethodStart inserts a line of code at the beginning of a method body.
// Scans for `methodName(` within a class body, finds the opening `{`, and inserts after it.
func injectAtMethodStart(text string, methodName string, line string) string {
	// Match: async methodName( or methodName( within a class body
	pattern := regexp.MustCompile(`(?:async\s+)?` + regexp.QuoteMeta(methodName) + `\s*\([^)]*\)\s*\{`)
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return text
	}

	// Find the opening brace position
	bracePos := loc[1] - 1
	if text[bracePos] != '{' {
		// Search for it from loc[0]
		for i := loc[0]; i < loc[1]; i++ {
			if text[i] == '{' {
				bracePos = i
				break
			}
		}
	}

	// Insert after the opening brace
	insertPos := bracePos + 1
	text = text[:insertPos] + "\n" + line + text[insertPos:]

	return text
}

// wrapReturnsInMethod finds a method by name, locates its body boundaries,
// and wraps each top-level `return EXPR;` statement with a transform call.
// Only wraps returns at depth 0 within the method (not inside nested functions/arrows).
func wrapReturnsInMethod(text string, methodName string, transformFunc string, isArray bool) string {
	bodyStart, bodyEnd, found := findMethodBody(text, methodName)
	if !found {
		return text
	}

	// Work on the method body substring
	body := text[bodyStart:bodyEnd]
	newBody := wrapReturnsInBody(body, transformFunc, isArray)
	if body == newBody {
		return text
	}

	return text[:bodyStart] + newBody + text[bodyEnd:]
}

// findMethodBody locates the method body boundaries (inside the opening/closing braces).
// Returns the start (after opening '{') and end (before closing '}') positions, and whether found.
func findMethodBody(text string, methodName string) (bodyStart, bodyEnd int, found bool) {
	// Match: async methodName( or methodName( within a class body
	pattern := regexp.MustCompile(`(?:async\s+)?` + regexp.QuoteMeta(methodName) + `\s*\([^)]*\)\s*\{`)
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return 0, 0, false
	}

	// Find the opening brace position
	bracePos := loc[1] - 1
	if text[bracePos] != '{' {
		for i := loc[0]; i < loc[1]; i++ {
			if text[i] == '{' {
				bracePos = i
				break
			}
		}
	}

	// Count braces to find matching closing brace
	depth := 1
	bodyStart = bracePos + 1
	for i := bodyStart; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return bodyStart, i, true
			}
		case '"', '\'', '`':
			// Skip string literals to avoid counting braces inside them
			i = skipStringLiteral(text, i)
		case '/':
			// Skip comments
			if i+1 < len(text) {
				if text[i+1] == '/' {
					// Line comment — skip to end of line
					for i < len(text) && text[i] != '\n' {
						i++
					}
				} else if text[i+1] == '*' {
					// Block comment — skip to */
					i += 2
					for i+1 < len(text) {
						if text[i] == '*' && text[i+1] == '/' {
							i++
							break
						}
						i++
					}
				}
			}
		}
	}

	return 0, 0, false
}

// skipStringLiteral advances past a string literal starting at position i.
// Handles single-quoted, double-quoted, and template literal strings.
func skipStringLiteral(text string, i int) int {
	quote := text[i]
	i++
	for i < len(text) {
		if text[i] == '\\' {
			i += 2 // skip escaped character
			continue
		}
		if text[i] == quote {
			return i
		}
		i++
	}
	return i
}

// wrapReturnsInBody wraps top-level return statements within a method body.
// Only wraps returns at brace depth 0 (not inside nested functions/arrows/blocks).
func wrapReturnsInBody(body string, transformFunc string, isArray bool) string {
	// We need to find `return EXPR;` patterns at depth 0
	// Depth tracking: nested { } blocks (if/for/arrow functions) increase depth
	var result strings.Builder
	i := 0
	depth := 0

	for i < len(body) {
		ch := body[i]

		// Track brace depth
		if ch == '{' {
			depth++
			result.WriteByte(ch)
			i++
			continue
		}
		if ch == '}' {
			depth--
			result.WriteByte(ch)
			i++
			continue
		}

		// Skip string literals
		if ch == '"' || ch == '\'' || ch == '`' {
			end := skipStringLiteral(body, i)
			result.WriteString(body[i : end+1])
			i = end + 1
			continue
		}

		// Skip comments
		if ch == '/' && i+1 < len(body) {
			if body[i+1] == '/' {
				// Line comment
				start := i
				for i < len(body) && body[i] != '\n' {
					i++
				}
				result.WriteString(body[start:i])
				continue
			}
			if body[i+1] == '*' {
				// Block comment
				start := i
				i += 2
				for i+1 < len(body) {
					if body[i] == '*' && body[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				result.WriteString(body[start:i])
				continue
			}
		}

		// Look for `return` keyword at depth 0
		if depth == 0 && ch == 'r' && i+6 <= len(body) && body[i:i+6] == "return" {
			// Check it's a word boundary (not part of a larger identifier)
			if i > 0 && isIdentChar(body[i-1]) {
				result.WriteByte(ch)
				i++
				continue
			}
			afterReturn := i + 6
			if afterReturn < len(body) && isIdentChar(body[afterReturn]) {
				result.WriteByte(ch)
				i++
				continue
			}

			// Found `return` at depth 0 — extract the expression until `;`
			// Skip whitespace after `return`
			exprStart := afterReturn
			for exprStart < len(body) && (body[exprStart] == ' ' || body[exprStart] == '\t' || body[exprStart] == '\n' || body[exprStart] == '\r') {
				exprStart++
			}

			// Bare `return;` — skip
			if exprStart < len(body) && body[exprStart] == ';' {
				result.WriteString(body[i : exprStart+1])
				i = exprStart + 1
				continue
			}

			// Find the end of the expression (the `;`)
			exprEnd := findExpressionEnd(body, exprStart)
			if exprEnd < 0 {
				// No semicolon found; write through and continue
				result.WriteByte(ch)
				i++
				continue
			}

			expr := strings.TrimSpace(body[exprStart:exprEnd])
			if expr == "" {
				result.WriteString(body[i : exprEnd+1])
				i = exprEnd + 1
				continue
			}

			// Write the wrapped return
			result.WriteString("return ")
			if isArray {
				result.WriteString("(await ")
				result.WriteString(expr)
				result.WriteString(").map(_v => ")
				result.WriteString(transformFunc)
				result.WriteString("(_v))")
			} else {
				result.WriteString(transformFunc)
				result.WriteString("(await ")
				result.WriteString(expr)
				result.WriteString(")")
			}
			result.WriteByte(';')
			i = exprEnd + 1 // skip past the `;`
			continue
		}

		result.WriteByte(ch)
		i++
	}

	return result.String()
}

// findExpressionEnd finds the position of the semicolon ending an expression,
// respecting nested parentheses, brackets, braces, and string literals.
func findExpressionEnd(body string, start int) int {
	depth := 0
	for i := start; i < len(body); i++ {
		ch := body[i]
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case '[':
			depth++
		case ']':
			depth--
		case '{':
			depth++
		case '}':
			depth--
		case '"', '\'', '`':
			i = skipStringLiteral(body, i)
		case ';':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// isIdentChar returns true if the character can be part of a JavaScript identifier.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$'
}
