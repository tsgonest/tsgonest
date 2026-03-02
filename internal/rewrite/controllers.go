package rewrite

import (
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/typescript-go/shim/core"
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

// prioritizedEdit wraps a core.TextChange with a priority for deterministic
// ordering when multiple edits target the same position.
type prioritizedEdit struct {
	pos      int    // sort key: position in original text
	priority int    // tie-breaker: lower = applied first
	newText  string // replacement text
	end      int    // end position (pos == end for pure insertions)
}

// rewriteController injects @Body() parameter validation and return value
// transformation into a controller file's emitted JS.
// For body params: inserts `paramName = assertTypeName(paramName);` at method start.
// For return values: wraps `return EXPR;` with `return transformTypeName(await EXPR);`.
// For @EventStream routes: injects Reflect.defineMetadata with per-event assert/stringify.
//
// Uses AST-based position extraction (LocateJS) and bulk edits for a single-pass rewrite.
func rewriteController(text string, outputFile string, controllers []analyzer.ControllerInfo, companionMap map[string]string, moduleFormat string) string {
	// ── Phase 1: Collect transform specifications (unchanged metadata logic) ──

	type bodyValidation struct {
		methodName string
		paramName  string
		typeName   string
	}
	type returnTransform struct {
		methodName string
		typeName   string
		isArray    bool
	}
	type primitiveReturnTransform struct {
		methodName string
		atomic     string // "string", "number", or "boolean"
		nullable   bool
	}
	type scalarCoercion struct {
		methodName string
		paramName  string
		atomic     string // "number" or "boolean"
	}

	var validations []bodyValidation
	var transforms []returnTransform
	var primitiveTransforms []primitiveReturnTransform
	var scalarCoercions []scalarCoercion
	var sseTransforms []sseTransform
	neededTypes := make(map[string]bool)
	neededTransformTypes := make(map[string]bool)
	neededSSETypes := make(map[string]bool)
	needsHelpersImport := false
	needsSseInterceptor := false

	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			if route.UsesRawResponse {
				continue
			}

			for _, param := range route.Parameters {
				switch param.Category {
				case "body":
					typeName := resolveParamTypeName(&param)
					if typeName == "" {
						continue
					}
					if _, ok := companionMap[typeName]; !ok {
						continue
					}
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
						eventKey = "*"
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
				continue
			}

			if route.IsSSE {
				continue
			}

			primitiveAtomic, primitiveNullable := resolvePrimitiveReturn(&route.ReturnType)
			if primitiveAtomic != "" {
				primitiveTransforms = append(primitiveTransforms, primitiveReturnTransform{
					methodName: route.MethodName,
					atomic:     primitiveAtomic,
					nullable:   primitiveNullable,
				})
				continue
			}

			returnTypeName := resolveReturnTypeName(&route.ReturnType)
			if returnTypeName == "" {
				continue
			}
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

	if len(validations) == 0 && len(transforms) == 0 && len(primitiveTransforms) == 0 && len(scalarCoercions) == 0 && len(sseTransforms) == 0 {
		return text
	}

	// ── Phase 2: Parse JS and extract AST locations ──

	locs := LocateJS(text)

	// Build method lookup: methodName → *MethodLoc (across all classes)
	methodLookup := make(map[string]*MethodLoc)
	for _, cls := range locs.Classes {
		for name, ml := range cls.Methods {
			methodLookup[name] = ml
		}
	}

	// ── Phase 3: Build prioritized edits ──

	var edits []prioritizedEdit

	// (a) Body validation injection at method body start
	for _, v := range validations {
		ml := methodLookup[v.methodName]
		if ml == nil {
			continue
		}
		assertFunc := companionFuncName("assert", v.typeName)
		assertLine := "\n    " + v.paramName + " = " + assertFunc + "(" + v.paramName + ");"
		edits = append(edits, prioritizedEdit{
			pos:      ml.BodyOpenBrace + 1,
			end:      ml.BodyOpenBrace + 1,
			priority: 30,
			newText:  assertLine,
		})
	}

	// (b) Scalar coercion injection at method body start
	for _, sc := range scalarCoercions {
		ml := methodLookup[sc.methodName]
		if ml == nil {
			continue
		}
		var coercionCode string
		switch sc.atomic {
		case "number":
			coercionCode = fmt.Sprintf("\n    if (%s === \"\") throw new __e([{path:\"%s\",expected:\"number\",received:\"string\"}]); %s = +%s; if (Number.isNaN(%s)) throw new __e([{path:\"%s\",expected:\"number\",received:typeof %s}]);",
				sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName)
		case "boolean":
			coercionCode = fmt.Sprintf("\n    if (%s === \"true\" || %s === \"1\") %s = true; else if (%s === \"false\" || %s === \"0\") %s = false; else throw new __e([{path:\"%s\",expected:\"boolean\",received:typeof %s}]);",
				sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName, sc.paramName)
		}
		if coercionCode != "" {
			edits = append(edits, prioritizedEdit{
				pos:      ml.BodyOpenBrace + 1,
				end:      ml.BodyOpenBrace + 1,
				priority: 31,
				newText:  coercionCode,
			})
		}
	}

	// (c) Return expression wrapping for DTO transforms
	for _, tr := range transforms {
		ml := methodLookup[tr.methodName]
		if ml == nil {
			continue
		}
		isAsync := ml.IsAsync
		// If method is not async, we need to insert "async " before the method name
		if !isAsync {
			edits = append(edits, prioritizedEdit{
				pos:      ml.MethodNamePos,
				end:      ml.MethodNamePos,
				priority: 20,
				newText:  "async ",
			})
			isAsync = true
		}

		var transformFunc string
		if tr.isArray {
			transformFunc = companionFuncName("serialize", tr.typeName)
		} else {
			transformFunc = companionFuncName("stringify", tr.typeName)
		}

		for _, ret := range ml.Returns {
			if ret.ExprStart < 0 {
				continue // bare return
			}
			expr := text[ret.ExprStart:ret.ExprEnd]
			newExpr := wrapReturnExpression(expr, transformFunc, tr.isArray, isAsync)
			// Replace: from 'return' keyword through the end of the statement
			suffix := ";"
			edits = append(edits, prioritizedEdit{
				pos:      ret.ReturnKeywordPos,
				end:      ret.StmtEnd,
				priority: 40,
				newText:  "return " + newExpr + suffix,
			})
		}
	}

	// (d) Return expression wrapping for primitive transforms
	for _, pt := range primitiveTransforms {
		ml := methodLookup[pt.methodName]
		if ml == nil {
			continue
		}
		isAsync := ml.IsAsync
		if !isAsync {
			edits = append(edits, prioritizedEdit{
				pos:      ml.MethodNamePos,
				end:      ml.MethodNamePos,
				priority: 20,
				newText:  "async ",
			})
			isAsync = true
		}

		for _, ret := range ml.Returns {
			if ret.ExprStart < 0 {
				continue // bare return
			}
			expr := text[ret.ExprStart:ret.ExprEnd]
			newExpr := wrapPrimitiveExpression(expr, pt.atomic, pt.nullable)
			suffix := ";"
			edits = append(edits, prioritizedEdit{
				pos:      ret.ReturnKeywordPos,
				end:      ret.StmtEnd,
				priority: 40,
				newText:  "return " + newExpr + suffix,
			})
		}
	}

	// (e) Class interceptor injection at __decorate bracket
	// Build import lines first to know which interceptors to inject
	var markerCalls []MarkerCall
	for typeName := range neededTypes {
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "assert",
			TypeName:     typeName,
		})
	}
	for typeName := range neededTransformTypes {
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "stringify",
			TypeName:     typeName,
		})
		markerCalls = append(markerCalls, MarkerCall{
			FunctionName: "serialize",
			TypeName:     typeName,
		})
	}
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

	if len(transforms) > 0 || len(primitiveTransforms) > 0 {
		if moduleFormat == "cjs" {
			importLines = append(importLines, `const { TsgonestSerializeInterceptor } = require("@tsgonest/runtime");`)
		} else {
			importLines = append(importLines, `import { TsgonestSerializeInterceptor } from "@tsgonest/runtime";`)
		}
		// Inject interceptor at class-level __decorate bracket
		for _, ctrl := range controllers {
			dc := findClassLevelDecorate(locs, ctrl.Name)
			if dc == nil {
				continue
			}
			if strings.Contains(text[dc.ArrayOpenBracket:], "UseInterceptors)(TsgonestSerializeInterceptor)") {
				continue
			}
			edits = append(edits, prioritizedEdit{
				pos:      dc.ArrayOpenBracket + 1,
				end:      dc.ArrayOpenBracket + 1,
				priority: 10,
				newText:  "\n    (0, common_1.UseInterceptors)(TsgonestSerializeInterceptor),",
			})
		}
	}

	if needsSseInterceptor {
		if moduleFormat == "cjs" {
			importLines = append(importLines, `const { TsgonestSseInterceptor } = require("@tsgonest/runtime");`)
		} else {
			importLines = append(importLines, `import { TsgonestSseInterceptor } from "@tsgonest/runtime";`)
		}
		for _, ctrl := range controllers {
			dc := findClassLevelDecorate(locs, ctrl.Name)
			if dc == nil {
				continue
			}
			if strings.Contains(text[dc.ArrayOpenBracket:], "UseInterceptors)(TsgonestSseInterceptor)") {
				continue
			}
			edits = append(edits, prioritizedEdit{
				pos:      dc.ArrayOpenBracket + 1,
				end:      dc.ArrayOpenBracket + 1,
				priority: 11,
				newText:  "\n    (0, common_1.UseInterceptors)(TsgonestSseInterceptor),",
			})
		}
	}

	if needsHelpersImport {
		if moduleFormat == "cjs" {
			importLines = append(importLines, `const { TsgonestValidationError: __e } = require("@tsgonest/runtime");`)
		} else {
			importLines = append(importLines, `import { TsgonestValidationError as __e } from "@tsgonest/runtime";`)
		}
	}

	// (f) SSE metadata injection after method-level __decorate calls
	for _, st := range sseTransforms {
		dc := findMethodLevelDecorate(locs, st.className, st.methodName)
		if dc == nil {
			continue
		}
		var entries []string
		for _, e := range st.entries {
			entries = append(entries, fmt.Sprintf("  %q: [%s, %s]", e.eventName, e.assertFunc, e.stringifyFunc))
		}
		transformMap := "{\n" + strings.Join(entries, ",\n") + "\n}"
		metadataCall := fmt.Sprintf(
			"\nReflect.defineMetadata(\"__tsgonest_sse_transforms__\", %s, %s.prototype, %q);",
			transformMap, st.className, st.methodName,
		)
		edits = append(edits, prioritizedEdit{
			pos:      dc.StmtEnd,
			end:      dc.StmtEnd,
			priority: 50,
			newText:  metadataCall,
		})
	}

	// (g) Imports at position 0
	if len(importLines) > 0 {
		edits = append(edits, prioritizedEdit{
			pos:      0,
			end:      0,
			priority: 0,
			newText:  strings.Join(importLines, "\n") + "\n",
		})
	}

	// ── Phase 4: Sort and apply edits ──

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].pos != edits[j].pos {
			return edits[i].pos < edits[j].pos
		}
		return edits[i].priority < edits[j].priority
	})

	// Convert to core.TextChange slice, skipping invalid edits (end < pos)
	var changes []core.TextChange
	for _, e := range edits {
		if e.end < e.pos {
			continue // skip malformed edit
		}
		changes = append(changes, core.TextChange{
			TextRange: core.NewTextRange(e.pos, e.end),
			NewText:   e.newText,
		})
	}

	return core.ApplyBulkEdits(text, changes)
}

// findClassLevelDecorate finds the class-level __decorate call for a given class name.
func findClassLevelDecorate(locs *JSLocations, className string) *DecorateCallLoc {
	for i := range locs.DecorateCalls {
		dc := &locs.DecorateCalls[i]
		if dc.IsClassLevel && dc.ClassName == className {
			return dc
		}
	}
	return nil
}

// findMethodLevelDecorate finds the method-level __decorate call for a given class+method.
func findMethodLevelDecorate(locs *JSLocations, className, methodName string) *DecorateCallLoc {
	for i := range locs.DecorateCalls {
		dc := &locs.DecorateCalls[i]
		if !dc.IsClassLevel && dc.ClassName == className && dc.MethodName == methodName {
			return dc
		}
	}
	return nil
}

// wrapReturnExpression wraps a DTO return expression with a transform call.
func wrapReturnExpression(expr, transformFunc string, isArray, isAsync bool) string {
	if isArray {
		var inner string
		if isAsync {
			inner = "(await " + expr + ")"
		} else {
			inner = "(" + expr + ")"
		}
		return "\"[\" + " + inner + ".map(_v => " + transformFunc + "(_v)).join(\",\") + \"]\""
	}
	if isAsync {
		return transformFunc + "(await " + expr + ")"
	}
	return transformFunc + "(" + expr + ")"
}

// wrapPrimitiveExpression wraps a primitive return expression with inline serialization.
func wrapPrimitiveExpression(expr, atomic string, nullable bool) string {
	awaitExpr := "(await " + expr + ")"
	if nullable {
		return "((_v) => _v == null ? \"null\" : " + primitiveSerializeExpr("_v", atomic) + ")" + awaitExpr
	}
	return primitiveSerializeExpr(awaitExpr, atomic)
}

// primitiveSerializeExpr returns the inline JS expression to serialize a primitive value.
func primitiveSerializeExpr(expr string, atomic string) string {
	switch atomic {
	case "string":
		return "JSON.stringify(" + expr + ")"
	case "number":
		return "(Number.isFinite(" + expr + ") ? \"\" + " + expr + " : \"null\")"
	case "boolean":
		return expr + " ? \"true\" : \"false\""
	default:
		return "JSON.stringify(" + expr + ")"
	}
}

// ─── Pure metadata extraction (unchanged) ───────────────────────────────────

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
// For primitives (string, number, boolean), returns a synthetic "__type" name.
// Returns empty string for any/void types.
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
	case metadata.KindAtomic:
		if m.Atomic == "string" || m.Atomic == "number" || m.Atomic == "boolean" {
			return "__" + m.Atomic
		}
	}
	return ""
}

// resolvePrimitiveReturn checks if a return type is a primitive that needs inline serialization.
// Returns the atomic type name and whether it's nullable.
// Returns ("", false) if not a primitive return type.
func resolvePrimitiveReturn(m *metadata.Metadata) (atomic string, nullable bool) {
	switch m.Kind {
	case metadata.KindAtomic:
		if m.Atomic == "string" || m.Atomic == "number" || m.Atomic == "boolean" {
			return m.Atomic, m.Nullable
		}
	case metadata.KindUnion:
		var foundAtomic string
		nullable := m.Nullable
		for _, member := range m.UnionMembers {
			if member.Kind == metadata.KindAtomic {
				if member.Atomic == "null" || member.Atomic == "undefined" {
					nullable = true
					continue
				}
				if foundAtomic == "" {
					foundAtomic = member.Atomic
				} else if foundAtomic != member.Atomic {
					return "", false
				}
			} else if member.Kind == metadata.KindLiteral {
				switch member.LiteralValue.(type) {
				case float64, int, int64:
					if foundAtomic == "" {
						foundAtomic = "number"
					} else if foundAtomic != "number" {
						return "", false
					}
				case string:
					if foundAtomic == "" {
						foundAtomic = "string"
					} else if foundAtomic != "string" {
						return "", false
					}
				case bool:
					if foundAtomic == "" {
						foundAtomic = "boolean"
					} else if foundAtomic != "boolean" {
						return "", false
					}
				default:
					return "", false
				}
			} else {
				return "", false
			}
		}
		if foundAtomic != "" {
			return foundAtomic, nullable
		}
	}
	return "", false
}

// ─── Backward-compatible thin wrappers (used by tests) ──────────────────────

// findBodyParamName finds the parameter name for an unnamed @Body() decorator
// by looking at the method signature in the emitted JS via AST parsing.
func findBodyParamName(text string, methodName string) string {
	locs := LocateJS(text)
	for _, cls := range locs.Classes {
		if m, ok := cls.Methods[methodName]; ok {
			if len(m.Parameters) > 0 {
				name := m.Parameters[0]
				// Reject destructured parameters
				if strings.ContainsAny(name, "{}[]") {
					return ""
				}
				return name
			}
		}
	}
	return ""
}

// findMethodBody locates the method body boundaries (inside the opening/closing braces).
// Returns the start (after opening '{') and end (at closing '}') positions, and whether found.
func findMethodBody(text string, methodName string) (bodyStart, bodyEnd int, found bool) {
	locs := LocateJS(text)
	for _, cls := range locs.Classes {
		if m, ok := cls.Methods[methodName]; ok {
			return m.BodyOpenBrace + 1, m.BodyCloseBrace, true
		}
	}
	return 0, 0, false
}

// wrapReturnsInMethod finds a method by name and wraps each top-level return
// statement with a transform call. Thin wrapper over AST-based approach.
func wrapReturnsInMethod(text string, methodName string, transformFunc string, isArray bool) string {
	locs := LocateJS(text)
	var ml *MethodLoc
	for _, cls := range locs.Classes {
		if m, ok := cls.Methods[methodName]; ok {
			ml = m
			break
		}
	}
	if ml == nil {
		return text
	}

	isAsync := ml.IsAsync
	var edits []prioritizedEdit

	// If not async, insert async keyword
	if !isAsync {
		edits = append(edits, prioritizedEdit{
			pos:      ml.MethodNamePos,
			end:      ml.MethodNamePos,
			priority: 20,
			newText:  "async ",
		})
		isAsync = true
	}

	for _, ret := range ml.Returns {
		if ret.ExprStart < 0 {
			continue
		}
		expr := text[ret.ExprStart:ret.ExprEnd]
		newExpr := wrapReturnExpression(expr, transformFunc, isArray, isAsync)
		edits = append(edits, prioritizedEdit{
			pos:      ret.ReturnKeywordPos,
			end:      ret.StmtEnd,
			priority: 40,
			newText:  "return " + newExpr + ";",
		})
	}

	if len(edits) == 0 {
		return text
	}

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].pos != edits[j].pos {
			return edits[i].pos < edits[j].pos
		}
		return edits[i].priority < edits[j].priority
	})

	changes := make([]core.TextChange, len(edits))
	for i, e := range edits {
		changes[i] = core.TextChange{
			TextRange: core.NewTextRange(e.pos, e.end),
			NewText:   e.newText,
		}
	}
	return core.ApplyBulkEdits(text, changes)
}

// injectClassInterceptor adds UseInterceptors(interceptorName) as a class-level
// decorator. Thin wrapper over AST-based approach.
func injectClassInterceptor(text string, controllers []analyzer.ControllerInfo, interceptorName string) string {
	locs := LocateJS(text)
	var edits []prioritizedEdit

	for _, ctrl := range controllers {
		dc := findClassLevelDecorate(locs, ctrl.Name)
		if dc == nil {
			continue
		}
		if strings.Contains(text[dc.ArrayOpenBracket:], "UseInterceptors)("+interceptorName+")") {
			continue
		}
		edits = append(edits, prioritizedEdit{
			pos:      dc.ArrayOpenBracket + 1,
			end:      dc.ArrayOpenBracket + 1,
			priority: 10,
			newText:  "\n    (0, common_1.UseInterceptors)(" + interceptorName + "),",
		})
	}

	if len(edits) == 0 {
		return text
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].pos < edits[j].pos
	})

	changes := make([]core.TextChange, len(edits))
	for i, e := range edits {
		changes[i] = core.TextChange{
			TextRange: core.NewTextRange(e.pos, e.end),
			NewText:   e.newText,
		}
	}
	return core.ApplyBulkEdits(text, changes)
}

// injectAtMethodStart inserts a line of code at the beginning of a method body.
// Thin wrapper over AST-based location extraction.
func injectAtMethodStart(text string, methodName string, line string) string {
	locs := LocateJS(text)
	for _, cls := range locs.Classes {
		if m, ok := cls.Methods[methodName]; ok {
			insertPos := m.BodyOpenBrace + 1
			return text[:insertPos] + "\n" + line + text[insertPos:]
		}
	}
	return text
}
