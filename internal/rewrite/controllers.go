package rewrite

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// sseTransformEntry holds the per-event-name assert/stringify function pair
// for @EventStream SSE transform metadata injection.
type sseTransformEntry struct {
	eventName     string // literal event name, or "*" for generic string
	assertFunc    string
	stringifyFunc string
}

// sseTransform holds SSE transform metadata for a single @EventStream method.
type sseTransform struct {
	className  string
	methodName string
	entries    []sseTransformEntry
}

// rewriteController injects @Body() parameter validation and return value
// transformation into a controller file's emitted JS.
// For body params: inserts `paramName = assertTypeName(paramName);` at method start.
// For return values: wraps `return EXPR;` with `return transformTypeName(await EXPR);`.
// For @EventStream routes: injects Reflect.defineMetadata with per-event assert/stringify.
func rewriteController(text string, outputFile string, controllers []analyzer.ControllerInfo, companionMap map[string]string, moduleFormat string) string {
	// Collect all body parameters with named types from matching controllers
	type bodyValidation struct {
		methodName string
		paramName  string
		typeName   string
	}

	// Collect return transformations
	type returnTransform struct {
		methodName string
		typeName   string
		isArray    bool
	}

	// scalarCoercion holds info for inline number/boolean coercion on named scalar params
	type scalarCoercion struct {
		methodName string
		paramName  string
		atomic     string // "number" or "boolean"
	}

	var validations []bodyValidation
	var transforms []returnTransform
	var scalarCoercions []scalarCoercion
	var sseTransforms []sseTransform
	neededTypes := make(map[string]bool)
	neededTransformTypes := make(map[string]bool)
	neededSSETypes := make(map[string]bool)
	needsHelpersImport := false
	needsSseInterceptor := false

	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			// Skip raw response routes
			if route.UsesRawResponse {
				continue
			}

			// Parameter validation collection (body, query, headers, param)
			for _, param := range route.Parameters {
				switch param.Category {
				case "body":
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
						paramName = findBodyParamName(text, route.MethodName)
						if paramName == "" {
							continue
						}
					}
					validations = append(validations, bodyValidation{
						methodName: route.MethodName,
						paramName:  paramName,
						typeName:   typeName,
					})
					neededTypes[typeName] = true

				case "query", "headers", "param":
					if param.Name == "" && param.TypeName != "" {
						// Whole-object: inject assert like @Body()
						typeName := resolveParamTypeName(&param)
						if typeName == "" {
							continue
						}
						if _, ok := companionMap[typeName]; !ok {
							continue
						}
						paramName := param.LocalName
						if paramName == "" {
							continue
						}
						validations = append(validations, bodyValidation{
							methodName: route.MethodName,
							paramName:  paramName,
							typeName:   typeName,
						})
						neededTypes[typeName] = true
					} else if param.Name != "" && param.Category != "headers" {
						// Individual named scalar: inline coercion (no companion needed)
						if param.Type.Kind == metadata.KindAtomic && (param.Type.Atomic == "number" || param.Type.Atomic == "boolean") {
							paramName := param.LocalName
							if paramName == "" {
								paramName = param.Name
							}
							scalarCoercions = append(scalarCoercions, scalarCoercion{
								methodName: route.MethodName,
								paramName:  paramName,
								atomic:     param.Type.Atomic,
							})
							needsHelpersImport = true
						}
					}
				}
			}

			// SSE transform collection for @EventStream routes
			if route.IsEventStream && len(route.SSEEventVariants) > 0 {
				var entries []sseTransformEntry
				for _, v := range route.SSEEventVariants {
					typeName := resolveReturnTypeName(&v.DataType)
					if typeName == "" {
						continue
					}
					if _, ok := companionMap[typeName]; !ok {
						continue
					}
					eventKey := v.EventName
					if eventKey == "" {
						eventKey = "*" // generic string event → wildcard
					}
					entries = append(entries, sseTransformEntry{
						eventName:     eventKey,
						assertFunc:    companionFuncName("assert", typeName),
						stringifyFunc: companionFuncName("stringify", typeName),
					})
					neededSSETypes[typeName] = true
				}
				if len(entries) > 0 {
					sseTransforms = append(sseTransforms, sseTransform{
						className:  ctrl.Name,
						methodName: route.MethodName,
						entries:    entries,
					})
					needsSseInterceptor = true
				}
				continue // SSE routes don't get return serialization wrapping
			}

			// Return transform collection (non-SSE routes)
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
				methodName: route.MethodName,
				typeName:   returnTypeName,
				isArray:    isArray,
			})
			neededTransformTypes[returnTypeName] = true
		}
	}

	if len(validations) == 0 && len(transforms) == 0 && len(scalarCoercions) == 0 && len(sseTransforms) == 0 {
		return text
	}

	// Inject validation calls into method bodies
	for _, v := range validations {
		assertFunc := companionFuncName("assert", v.typeName)
		assertLine := "    " + v.paramName + " = " + assertFunc + "(" + v.paramName + ");"
		text = injectAtMethodStart(text, v.methodName, assertLine)
	}

	// Inject inline scalar coercion for individual @Param/@Query params
	for _, sc := range scalarCoercions {
		var coercionCode string
		switch sc.atomic {
		case "number":
			coercionCode = fmt.Sprintf("    %s = +%s; if (Number.isNaN(%s)) throw new __e([{path:\"%s\",expected:\"number\",received:typeof %s}]);",
				sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName)
		case "boolean":
			coercionCode = fmt.Sprintf("    if (%s === \"true\" || %s === \"1\") %s = true; else if (%s === \"false\" || %s === \"0\") %s = false;",
				sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName)
		}
		if coercionCode != "" {
			text = injectAtMethodStart(text, sc.methodName, coercionCode)
		}
	}

	// Wrap return statements with stringify calls
	for _, tr := range transforms {
		if tr.isArray {
			// Arrays: serialize each element and join into JSON array string
			serializeFunc := companionFuncName("serialize", tr.typeName)
			text = wrapReturnsInMethod(text, tr.methodName, serializeFunc, tr.isArray)
		} else {
			stringifyFunc := companionFuncName("stringify", tr.typeName)
			text = wrapReturnsInMethod(text, tr.methodName, stringifyFunc, false)
		}
	}

	// Inject SSE transform metadata after method-level __decorate calls
	for _, st := range sseTransforms {
		text = injectSSETransforms(text, st)
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
		// For arrays, we need serialize; for non-arrays, we need stringify
		// Import both to be safe since companion files export both
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "stringify",
			TypeName:     typeName,
		})
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "serialize",
			TypeName:     typeName,
		})
	}
	// Import assert + stringify for SSE variant data types
	for typeName := range neededSSETypes {
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "assert",
			TypeName:     typeName,
		})
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "stringify",
			TypeName:     typeName,
		})
	}
	importLines := companionImports(markerCalls, companionMap, outputFile, moduleFormat)

	// If we have response stringify transforms, inject the serialize interceptor
	if len(transforms) > 0 {
		if moduleFormat == "cjs" {
			importLines = append(importLines, `const { TsgonestSerializeInterceptor } = require("@tsgonest/runtime");`)
		} else {
			importLines = append(importLines, `import { TsgonestSerializeInterceptor } from "@tsgonest/runtime";`)
		}
		// Inject UseInterceptors decorator on the controller class
		text = injectClassInterceptor(text, controllers, "TsgonestSerializeInterceptor")
	}

	// If we have SSE transforms, inject the SSE interceptor
	if needsSseInterceptor {
		if moduleFormat == "cjs" {
			importLines = append(importLines, `const { TsgonestSseInterceptor } = require("@tsgonest/runtime");`)
		} else {
			importLines = append(importLines, `import { TsgonestSseInterceptor } from "@tsgonest/runtime";`)
		}
		// Inject UseInterceptors decorator on the controller class
		text = injectClassInterceptor(text, controllers, "TsgonestSseInterceptor")
	}

	// If we have scalar coercions, import TsgonestValidationError as __e
	if needsHelpersImport {
		if moduleFormat == "cjs" {
			importLines = append(importLines, `const { TsgonestValidationError: __e } = require("@tsgonest/runtime");`)
		} else {
			importLines = append(importLines, `import { TsgonestValidationError as __e } from "@tsgonest/runtime";`)
		}
	}

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

	// Check if the method is async by looking at the method declaration before the body
	methodDecl := text[:bodyStart]
	isAsync := isMethodAsync(methodDecl, methodName)

	// If the method is not async, make it async so we can safely await the return value.
	// This is necessary because the method may return a Promise (e.g., from an async service call)
	// even though the method itself is not declared async.
	// NestJS handles async methods natively, and adding async to a synchronous method is safe
	// (it just wraps the return in a Promise, which NestJS already expects).
	if !isAsync {
		text = makeMethodAsync(text, methodName)
		isAsync = true
		// Re-find body boundaries since text changed
		bodyStart, bodyEnd, found = findMethodBody(text, methodName)
		if !found {
			return text
		}
	}

	// Work on the method body substring
	body := text[bodyStart:bodyEnd]
	newBody := wrapReturnsInBody(body, transformFunc, isArray, isAsync)
	if body == newBody {
		return text
	}

	return text[:bodyStart] + newBody + text[bodyEnd:]
}

// makeMethodAsync inserts `async` before the method name in its declaration.
// Handles both `methodName(` and indented declarations like `    methodName(`.
func makeMethodAsync(text string, methodName string) string {
	// Find the method declaration: methodName( preceded by whitespace/newline
	// but NOT already preceded by `async`
	pattern := regexp.MustCompile(`(\n[ \t]+)` + regexp.QuoteMeta(methodName) + `\s*\(`)
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return text
	}
	// Find the method name start (skip the newline+whitespace prefix)
	methodStart := loc[0]
	nameIdx := strings.Index(text[methodStart:methodStart+len(text[loc[0]:loc[1]])], methodName)
	if nameIdx < 0 {
		return text
	}
	insertPos := methodStart + nameIdx
	// Check if already async
	before := strings.TrimRight(text[:insertPos], " \t")
	if strings.HasSuffix(before, "async") {
		return text
	}
	return text[:insertPos] + "async " + text[insertPos:]
}

// isMethodAsync checks if a method declaration preceding the body start is async.
func isMethodAsync(textBeforeBody string, methodName string) bool {
	// Find the method name position — look backwards from end for "async methodName("
	idx := strings.LastIndex(textBeforeBody, methodName)
	if idx < 0 {
		return false
	}
	// Check the text before the method name for "async" keyword
	before := strings.TrimRight(textBeforeBody[:idx], " \t\n\r")
	return strings.HasSuffix(before, "async")
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
// When isAsync is false, omits `await` from the wrapping.
func wrapReturnsInBody(body string, transformFunc string, isArray bool, isAsync bool) string {
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
				// Serialize each element and join into JSON array string:
				// return "[" + (await EXPR).map(_v => serializeFunc(_v)).join(",") + "]";
				result.WriteString("\"[\" + ")
				if isAsync {
					result.WriteString("(await ")
					result.WriteString(expr)
					result.WriteString(")")
				} else {
					result.WriteString("(")
					result.WriteString(expr)
					result.WriteString(")")
				}
				result.WriteString(".map(_v => ")
				result.WriteString(transformFunc)
				result.WriteString("(_v)).join(\",\") + \"]\"")
			} else {
				result.WriteString(transformFunc)
				result.WriteString("(")
				if isAsync {
					result.WriteString("await ")
				}
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

// injectClassInterceptor adds UseInterceptors(interceptorName)
// as a class-level decorator on each controller.
// It finds the class-level __decorate([ ... ], ControllerName) call and inserts
// the interceptor decorator after the opening bracket.
func injectClassInterceptor(text string, controllers []analyzer.ControllerInfo, interceptorName string) string {
	for _, ctrl := range controllers {
		className := ctrl.Name
		// Find: ClassName = __decorate([
		pattern := regexp.MustCompile(regexp.QuoteMeta(className) + `\s*=\s*__decorate\(\[`)
		loc := pattern.FindStringIndex(text)
		if loc == nil {
			continue
		}
		// Find the '[' position
		bracketPos := loc[1] - 1
		insertPos := bracketPos + 1

		interceptorLine := "\n    (0, common_1.UseInterceptors)(" + interceptorName + "),"
		text = text[:insertPos] + interceptorLine + text[insertPos:]
	}
	return text
}

// injectSSETransforms injects Reflect.defineMetadata for SSE per-event transform
// maps after the method-level __decorate call for the given method.
//
// It generates code like:
//
//	Reflect.defineMetadata("__tsgonest_sse_transforms__", {
//	  "created": [assertUserDto, stringifyUserDto],
//	  "deleted": [assertDeletePayload, stringifyDeletePayload]
//	}, ClassName.prototype, "methodName");
//
// This metadata is read at request time by TsgonestSseInterceptor to validate
// and serialize each event's data field.
func injectSSETransforms(text string, st sseTransform) string {
	// Find the method-level __decorate call:
	// __decorate([...], ClassName.prototype, "methodName", null);
	pattern := regexp.MustCompile(
		`__decorate\(\[[^\]]*\],\s*` +
			regexp.QuoteMeta(st.className) + `\.prototype,\s*"` +
			regexp.QuoteMeta(st.methodName) + `"[^;]*;`,
	)
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return text
	}

	// Build the transform map object literal
	var entries []string
	for _, e := range st.entries {
		entries = append(entries, fmt.Sprintf("  %q: [%s, %s]", e.eventName, e.assertFunc, e.stringifyFunc))
	}
	transformMap := "{\n" + strings.Join(entries, ",\n") + "\n}"

	metadataCall := fmt.Sprintf(
		"\nReflect.defineMetadata(\"__tsgonest_sse_transforms__\", %s, %s.prototype, %q);",
		transformMap, st.className, st.methodName,
	)

	// Insert after the __decorate call
	insertPos := loc[1]
	text = text[:insertPos] + metadataCall + text[insertPos:]

	return text
}
