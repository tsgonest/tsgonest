package rewrite

import (
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/typescript-go/shim/core"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/formats"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// sseTransformEntry holds the per-event-name assert/stringify function pair
// for @EventStream SSE transform metadata injection.
type sseTransformEntry struct {
	eventName     string // literal event name, or "*" for generic string
	assertFunc    string
	stringifyFunc string
	nullable      bool
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
//
// report is called for each rewrite the function intended to apply but couldn't
// (controller class missing from emit, method missing on located class, etc.).
// Pass nil to discard diagnostics — tests use nil, production threads diagnostics
// back to RewriteContext via MakeWriteFile.
func rewriteController(text string, outputFile string, controllers []analyzer.ControllerInfo, companionMap map[string]string, moduleFormat string, report func(RewriteDiagnostic)) string {
	emit := func(d RewriteDiagnostic) {
		if report != nil {
			report(d)
		}
	}
	// ── Phase 1: Collect transform specifications (unchanged metadata logic) ──

	type bodyValidation struct {
		className      string
		methodName     string
		paramName      string
		typeName       string
		paramIndex     int  // JS parameter index (used for destructured param replacement)
		isDestructured bool // true if the JS param is a destructured pattern
	}
	type returnTransform struct {
		className       string
		methodName      string
		typeName        string
		isArray         bool
		nullable        bool
		elementNullable bool
	}
	type primitiveReturnTransform struct {
		className  string
		methodName string
		atomic     string // "string", "number", or "boolean"
		nullable   bool
	}
	type scalarCoercion struct {
		className  string
		methodName string
		paramName  string
		atomic     string // "number" or "boolean"
	}
	type scalarConstraintCheck struct {
		className   string
		methodName  string
		paramName   string
		atomic      string // "string" or "number"
		constraints *metadata.Constraints
	}

	var validations []bodyValidation
	var transforms []returnTransform
	var primitiveTransforms []primitiveReturnTransform
	var scalarCoercions []scalarCoercion
	var scalarConstraintChecks []scalarConstraintCheck
	var sseTransforms []sseTransform
	neededTypes := make(map[string]bool)
	neededTransformTypes := make(map[string]bool)
	neededSSETypes := make(map[string]bool)
	needsHelpersImport := false
	needsSseInterceptor := false
	controllersWithEventStream := make(map[string]bool)

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
					isDestructured := false
					if paramName == "" {
						// LocalName is empty (destructured param). Use ParameterIndex
						// to find the correct JS parameter instead of always picking
						// the first one.
						paramName = findBodyParamNameByIndex(text, ctrl.Name, route.MethodName, param.ParameterIndex)
						if paramName == "" {
							// Parameter at this index is destructured — generate a synthetic name
							paramName = "__body"
							isDestructured = true
						}
					}
					validations = append(validations, bodyValidation{
						className:      ctrl.Name,
						methodName:     route.MethodName,
						paramName:      paramName,
						typeName:       typeName,
						paramIndex:     param.ParameterIndex,
						isDestructured: isDestructured,
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
							className:  ctrl.Name,
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
								className:  ctrl.Name,
								methodName: route.MethodName,
								paramName:  paramName,
								atomic:     param.Type.Atomic,
							})
							needsHelpersImport = true
						}
						// Constraint-based runtime checks for named scalar string/number
						// params. Strings get checks even without coercion; numbers get
						// checks layered on top of coercion (priority 32 > 31) so the
						// numeric bounds run against the coerced numeric value.
						if param.Type.Kind == metadata.KindAtomic && param.Type.Constraints != nil &&
							(param.Type.Atomic == "string" || param.Type.Atomic == "number") {
							paramName := param.LocalName
							if paramName == "" {
								paramName = param.Name
							}
							scalarConstraintChecks = append(scalarConstraintChecks, scalarConstraintCheck{
								className:   ctrl.Name,
								methodName:  route.MethodName,
								paramName:   paramName,
								atomic:      param.Type.Atomic,
								constraints: param.Type.Constraints,
							})
							needsHelpersImport = true
						}
					}
				}
			}

			// The TsgonestSseInterceptor is always needed for @EventStream routes:
			// it bridges async generators → Observables for NestJS's SSE handler.
			// Track the controller unconditionally; SSE transforms (validation/
			// serialization) remain conditional on companion file availability.
			if route.IsEventStream {
				needsSseInterceptor = true
				controllersWithEventStream[ctrl.Name] = true
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
						nullable:      v.DataType.Nullable || v.DataType.Optional,
					})
					neededSSETypes[typeName] = true
				}
				if len(entries) > 0 {
					sseTransforms = append(sseTransforms, sseTransform{
						className:  ctrl.Name,
						methodName: route.MethodName,
						entries:    entries,
					})
				}
				continue
			}

			if route.IsSSE {
				continue
			}

			primitiveAtomic, primitiveNullable := resolvePrimitiveReturn(&route.ReturnType)
			if primitiveAtomic != "" {
				primitiveTransforms = append(primitiveTransforms, primitiveReturnTransform{
					className:  ctrl.Name,
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
			var elementNullable bool
			if isArray && route.ReturnType.ElementType != nil {
				elementNullable = route.ReturnType.ElementType.Nullable || route.ReturnType.ElementType.Optional
			}
			transforms = append(transforms, returnTransform{
				className:       ctrl.Name,
				methodName:      route.MethodName,
				typeName:        returnTypeName,
				isArray:         isArray,
				nullable:        route.ReturnType.Nullable || route.ReturnType.Optional,
				elementNullable: elementNullable,
			})
			neededTransformTypes[returnTypeName] = true
		}
	}

	if len(validations) == 0 && len(transforms) == 0 && len(primitiveTransforms) == 0 && len(scalarCoercions) == 0 && len(scalarConstraintChecks) == 0 && len(sseTransforms) == 0 && !needsSseInterceptor {
		return text
	}

	// Track which controllers had at least one piece of injection work, so we
	// only error on classes whose absence actually drops a load-bearing
	// rewrite (vs. a controller that happened to need nothing).
	controllersWithWork := make(map[string]bool)
	for _, v := range validations {
		controllersWithWork[v.className] = true
	}
	for _, sc := range scalarCoercions {
		controllersWithWork[sc.className] = true
	}
	for _, sc := range scalarConstraintChecks {
		controllersWithWork[sc.className] = true
	}
	for _, tr := range transforms {
		controllersWithWork[tr.className] = true
	}
	for _, pt := range primitiveTransforms {
		controllersWithWork[pt.className] = true
	}
	for _, st := range sseTransforms {
		controllersWithWork[st.className] = true
	}
	for name := range controllersWithEventStream {
		controllersWithWork[name] = true
	}

	// ── Phase 2: Parse JS and extract AST locations ──

	locs := LocateJS(text)

	// Class-not-located is the catastrophic miss (issue #114): a controller
	// the analyzer recognized has zero injections applied because we couldn't
	// find its class in the emitted JS. Silent no-op = unvalidated input
	// reaches handlers, so this must fail the build.
	for _, ctrl := range controllers {
		if !controllersWithWork[ctrl.Name] {
			continue
		}
		if _, ok := locs.Classes[ctrl.Name]; !ok {
			emit(RewriteDiagnostic{
				Severity:   DiagnosticError,
				OutputFile: outputFile,
				Class:      ctrl.Name,
				Reason:     "controller class not found in emitted JS — validation/serialization injection was skipped (likely an unrecognized tsgo emit shape)",
			})
		}
	}

	// Build method lookup: className → methodName → *MethodLoc
	// Keyed by class name so same-named methods in different controllers
	// resolve to their own MethodLoc and never clobber each other.
	classMethodLookup := make(map[string]map[string]*MethodLoc)
	for className, cls := range locs.Classes {
		methods := make(map[string]*MethodLoc, len(cls.Methods))
		for name, ml := range cls.Methods {
			methods[name] = ml
		}
		classMethodLookup[className] = methods
	}
	lookupMethod := func(className, methodName string) *MethodLoc {
		if cls, ok := classMethodLookup[className]; ok {
			return cls[methodName]
		}
		return nil
	}

	// warnMethodMissing emits a warning when a method-level rewrite couldn't
	// be applied. Suppressed if the class itself wasn't located — that case
	// already produced a class-level error and per-method warnings would just
	// be noise on top of it.
	warnMethodMissing := func(className, methodName, kind string) {
		if _, classFound := classMethodLookup[className]; !classFound {
			return
		}
		emit(RewriteDiagnostic{
			Severity:   DiagnosticWarning,
			OutputFile: outputFile,
			Class:      className,
			Method:     methodName,
			Reason:     kind + " rewrite skipped — method not found on located class (unrecognized tsgo emit shape: getter/setter, computed name, accessor, etc.)",
		})
	}

	// ── Phase 3: Build prioritized edits ──

	var edits []prioritizedEdit

	// (a) Body validation injection at method body start
	for _, v := range validations {
		ml := lookupMethod(v.className, v.methodName)
		if ml == nil {
			warnMethodMissing(v.className, v.methodName, "body/query/param validation")
			continue
		}
		assertFunc := companionFuncName("assert", v.typeName)

		if v.isDestructured && v.paramIndex >= 0 && v.paramIndex < len(ml.ParamLocs) {
			pl := ml.ParamLocs[v.paramIndex]
			// Extract the raw destructured pattern text from the JS source
			destructuredPattern := text[pl.PatStart:pl.PatEnd]

			// Replace the destructured pattern in the parameter list with the synthetic name
			edits = append(edits, prioritizedEdit{
				pos:      pl.PatStart,
				end:      pl.PatEnd,
				priority: 10, // before body injection
				newText:  v.paramName,
			})

			// Inject assertion + destructuring reconstruction at method body start
			assertLine := "\n    " + v.paramName + " = " + assertFunc + "(" + v.paramName + ");"
			destructLine := "\n    const " + destructuredPattern + " = " + v.paramName + ";"
			edits = append(edits, prioritizedEdit{
				pos:      ml.BodyOpenBrace + 1,
				end:      ml.BodyOpenBrace + 1,
				priority: 30,
				newText:  assertLine + destructLine,
			})
		} else {
			assertLine := "\n    " + v.paramName + " = " + assertFunc + "(" + v.paramName + ");"
			edits = append(edits, prioritizedEdit{
				pos:      ml.BodyOpenBrace + 1,
				end:      ml.BodyOpenBrace + 1,
				priority: 30,
				newText:  assertLine,
			})
		}
	}

	// (b) Scalar coercion injection at method body start
	for _, sc := range scalarCoercions {
		ml := lookupMethod(sc.className, sc.methodName)
		if ml == nil {
			warnMethodMissing(sc.className, sc.methodName, "query/param scalar coercion")
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

	// (b2) Inline constraint checks for named scalar @Param/@Query params.
	// Runs at priority 32 — after scalar coercion (31) so number bounds are
	// evaluated against the coerced numeric value, never the original string.
	for _, sc := range scalarConstraintChecks {
		ml := lookupMethod(sc.className, sc.methodName)
		if ml == nil {
			warnMethodMissing(sc.className, sc.methodName, "query/param scalar constraint check")
			continue
		}
		if sc.constraints.ValidateFn != nil {
			emit(RewriteDiagnostic{
				Severity:   DiagnosticWarning,
				OutputFile: outputFile,
				Class:      sc.className,
				Method:     sc.methodName,
				Reason:     fmt.Sprintf("Validate<typeof %s> on scalar param %q is not enforced at runtime (OpenAPI schema only) — use a DTO parameter or a Pattern/Format constraint", *sc.constraints.ValidateFn, sc.paramName),
			})
		}
		checkCode := buildScalarConstraintCheck(sc.paramName, sc.atomic, sc.constraints)
		if checkCode == "" {
			continue
		}
		edits = append(edits, prioritizedEdit{
			pos:      ml.BodyOpenBrace + 1,
			end:      ml.BodyOpenBrace + 1,
			priority: 32,
			newText:  checkCode,
		})
	}

	// (c) Return expression wrapping for DTO transforms
	for _, tr := range transforms {
		ml := lookupMethod(tr.className, tr.methodName)
		if ml == nil {
			warnMethodMissing(tr.className, tr.methodName, "return-value DTO transform")
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
			newExpr := wrapReturnExpression(expr, transformFunc, tr.isArray, isAsync, tr.nullable, tr.elementNullable)
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
		ml := lookupMethod(pt.className, pt.methodName)
		if ml == nil {
			warnMethodMissing(pt.className, pt.methodName, "return-value primitive transform")
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
		// Only inject the interceptor into controllers that actually have return transforms.
		controllersWithTransforms := make(map[string]bool)
		for _, tr := range transforms {
			controllersWithTransforms[tr.className] = true
		}
		for _, pt := range primitiveTransforms {
			controllersWithTransforms[pt.className] = true
		}
		for _, ctrl := range controllers {
			if !controllersWithTransforms[ctrl.Name] {
				continue
			}
			dc := findClassLevelDecorate(locs, ctrl.Name)
			if dc == nil {
				continue
			}
			stmtEnd := dc.StmtEnd
			if stmtEnd > len(text) {
				stmtEnd = len(text)
			}
			if strings.Contains(text[dc.ArrayOpenBracket:stmtEnd], "UseInterceptors)(TsgonestSerializeInterceptor)") {
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
		// Inject the SSE interceptor into all controllers that have @EventStream routes.
		for _, ctrl := range controllers {
			if !controllersWithEventStream[ctrl.Name] {
				continue
			}
			dc := findClassLevelDecorate(locs, ctrl.Name)
			if dc == nil {
				continue
			}
			stmtEnd2 := dc.StmtEnd
			if stmtEnd2 > len(text) {
				stmtEnd2 = len(text)
			}
			// Check for existing interceptor — handles both auto-injected
			// (TsgonestSseInterceptor) and manual module-prefixed forms
			// (runtime_1.TsgonestSseInterceptor, runtime.TsgonestSseInterceptor).
			if strings.Contains(text[dc.ArrayOpenBracket:stmtEnd2], "TsgonestSseInterceptor") {
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
			warnMethodMissing(st.className, st.methodName, "SSE event-stream metadata")
			continue
		}
		var entries []string
		for _, e := range st.entries {
			assertExpr := e.assertFunc
			stringifyExpr := e.stringifyFunc
			if e.nullable {
				assertExpr = fmt.Sprintf("(_d) => _d == null ? _d : %s(_d)", e.assertFunc)
				stringifyExpr = fmt.Sprintf("(_d) => _d == null ? \"null\" : %s(_d)", e.stringifyFunc)
			}
			entries = append(entries, fmt.Sprintf("  %q: [%s, %s]", e.eventName, assertExpr, stringifyExpr))
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

	// (g) Imports after "use strict" directive (or position 0 if none)
	if len(importLines) > 0 {
		insertPos := findUseStrictEnd(text)
		edits = append(edits, prioritizedEdit{
			pos:      insertPos,
			end:      insertPos,
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

// findUseStrictEnd returns the byte offset after the "use strict"; directive prologue
// (including its trailing newline), or 0 if the file doesn't start with one.
func findUseStrictEnd(text string) int {
	for _, prefix := range []string{"\"use strict\";\n", "'use strict';\n"} {
		if strings.HasPrefix(text, prefix) {
			return len(prefix)
		}
	}
	return 0
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
func wrapReturnExpression(expr, transformFunc string, isArray, isAsync, nullable, elementNullable bool) string {
	// Build the element-level mapping expression for arrays.
	// When elements can be null (e.g., (UserDto | null)[]), each element
	// needs a null guard before calling the transform function.
	mapExpr := transformFunc + "(_i)"
	if elementNullable {
		mapExpr = "_i == null ? \"null\" : " + transformFunc + "(_i)"
	}

	if nullable {
		awaitExpr := expr
		if isAsync {
			awaitExpr = "(await " + expr + ")"
		}
		if isArray {
			return "((_v) => _v == null ? \"null\" : \"[\" + _v.map((_i) => " + mapExpr + ").join(\",\") + \"]\")" + awaitExpr
		}
		return "((_v) => _v == null ? \"null\" : " + transformFunc + "(_v))" + awaitExpr
	}
	if isArray {
		var inner string
		if isAsync {
			inner = "(await " + expr + ")"
		} else {
			inner = "(" + expr + ")"
		}
		return "\"[\" + " + inner + ".map((_i) => " + mapExpr + ").join(\",\") + \"]\""
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
			// Reject nested arrays (T[][]) — single-level .map() wrapping
			// can't serialize them correctly. Fall through to NestJS default.
			if m.ElementType.Kind == metadata.KindArray {
				return ""
			}
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

// findBodyParamNameByIndex finds the parameter name at a specific index in the
// method signature. Returns the identifier name, or "" if the parameter at that
// index is destructured or the index is out of range.
func findBodyParamNameByIndex(text string, className string, methodName string, paramIndex int) string {
	locs := LocateJS(text)
	cls, ok := locs.Classes[className]
	if !ok {
		return ""
	}
	m, ok := cls.Methods[methodName]
	if !ok {
		return ""
	}
	if paramIndex < 0 || paramIndex >= len(m.Parameters) {
		return ""
	}
	name := m.Parameters[paramIndex]
	// Reject destructured or empty names
	if name == "" || strings.ContainsAny(name, "{}[]") {
		return ""
	}
	return name
}

// findBodyParamName finds the parameter name for an unnamed @Body() decorator
// by looking at the method signature in the emitted JS via AST parsing.
// className scopes the search to the correct class so that same-named methods
// in different controllers don't return each other's parameter names.
func findBodyParamName(text string, className string, methodName string) string {
	locs := LocateJS(text)
	cls, ok := locs.Classes[className]
	if !ok {
		return ""
	}
	m, ok := cls.Methods[methodName]
	if !ok {
		return ""
	}
	if len(m.Parameters) > 0 {
		name := m.Parameters[0]
		// Reject destructured parameters
		if strings.ContainsAny(name, "{}[]") {
			return ""
		}
		return name
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
		newExpr := wrapReturnExpression(expr, transformFunc, isArray, isAsync, false, false)
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
		stmtEnd := dc.StmtEnd
		if stmtEnd > len(text) {
			stmtEnd = len(text)
		}
		if strings.Contains(text[dc.ArrayOpenBracket:stmtEnd], "UseInterceptors)("+interceptorName+")") {
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

// jsStringEscape makes a Go string safe to embed inside a JS double-quoted
// string literal. Mirrors internal/codegen/validate_util.go jsStringEscape —
// kept local to avoid an internal/codegen → internal/rewrite import cycle.
//
// Omits U+2028/U+2029 escapes intentionally: those only matter when the emit
// is embedded inside an HTML <script> tag (ES5 line-terminator parsing). Our
// emit is consumed by Node, and JSDoc @pattern values don't realistically
// contain these characters.
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

// escapeForRegexLiteral escapes forward slashes for safe embedding in a JS
// regex literal /…/. Mirrors internal/codegen/validate_util.go escapeForRegexLiteral —
// kept local to avoid an internal/codegen → internal/rewrite import cycle.
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

// buildScalarConstraintCheck composes the inline JS for runtime constraint
// checks on a single named scalar @Param/@Query parameter. All applicable
// checks for the param are concatenated into one returned string so the caller
// can emit them as a single prioritized edit. String order: transforms →
// Pattern → MinLength → MaxLength → Format → StartsWith → EndsWith →
// Includes → Uppercase → Lowercase. Number order: Minimum → Maximum →
// MultipleOf. Constraints that don't apply to the atomic are silently
// skipped (defensive — analyzer shouldn't produce them, but we never want
// a misapplied check to slip through).
func buildScalarConstraintCheck(paramName, atomic string, c *metadata.Constraints) string {
	if c == nil {
		return ""
	}
	var b strings.Builder

	if atomic == "string" {
		for _, t := range c.Transforms {
			switch t {
			case "trim", "toLowerCase", "toUpperCase":
				b.WriteString(fmt.Sprintf("\n    %s = %s.%s();", paramName, paramName, t))
			}
		}
		if c.Pattern != nil {
			raw := *c.Pattern
			// raw lands in two slots with different escaping rules:
			//   - regex literal /…/ — `/` must be escaped
			//   - JS string literal "pattern …" — `"`, `\`, control chars must be escaped
			// Without jsStringEscape, a pattern containing `"` would terminate the JS
			// string mid-emit and corrupt the surrounding object literal.
			b.WriteString(fmt.Sprintf("\n    if (!/%s/.test(%s)) throw new __e([{path:%q,expected:\"pattern %s\",received:%s}]);",
				escapeForRegexLiteral(raw), paramName, paramName, jsStringEscape(raw), paramName))
		}
		if c.MinLength != nil {
			n := *c.MinLength
			b.WriteString(fmt.Sprintf("\n    if (%s.length < %d) throw new __e([{path:%q,expected:\"minLength %d\",received:\"length \"+%s.length}]);",
				paramName, n, paramName, n, paramName))
		}
		if c.MaxLength != nil {
			n := *c.MaxLength
			b.WriteString(fmt.Sprintf("\n    if (%s.length > %d) throw new __e([{path:%q,expected:\"maxLength %d\",received:\"length \"+%s.length}]);",
				paramName, n, paramName, n, paramName))
		}
		if c.Format != nil {
			if pattern, ok := formats.Regexes[*c.Format]; ok && pattern != "" {
				regexLiteral := "/" + escapeForRegexLiteral(pattern) + "/" + formats.Flags[*c.Format]
				b.WriteString(fmt.Sprintf("\n    if (!%s.test(%s)) throw new __e([{path:%q,expected:\"format %s\",received:%s}]);",
					regexLiteral, paramName, paramName, jsStringEscape(*c.Format), paramName))
			}
		}
		if c.StartsWith != nil {
			escaped := jsStringEscape(*c.StartsWith)
			b.WriteString(fmt.Sprintf("\n    if (!%s.startsWith(\"%s\")) throw new __e([{path:%q,expected:\"startsWith %s\",received:%s}]);",
				paramName, escaped, paramName, escaped, paramName))
		}
		if c.EndsWith != nil {
			escaped := jsStringEscape(*c.EndsWith)
			b.WriteString(fmt.Sprintf("\n    if (!%s.endsWith(\"%s\")) throw new __e([{path:%q,expected:\"endsWith %s\",received:%s}]);",
				paramName, escaped, paramName, escaped, paramName))
		}
		if c.Includes != nil {
			escaped := jsStringEscape(*c.Includes)
			b.WriteString(fmt.Sprintf("\n    if (!%s.includes(\"%s\")) throw new __e([{path:%q,expected:\"includes %s\",received:%s}]);",
				paramName, escaped, paramName, escaped, paramName))
		}
		if c.Uppercase != nil && *c.Uppercase {
			b.WriteString(fmt.Sprintf("\n    if (%s !== %s.toUpperCase()) throw new __e([{path:%q,expected:\"uppercase\",received:%s}]);",
				paramName, paramName, paramName, paramName))
		}
		if c.Lowercase != nil && *c.Lowercase {
			b.WriteString(fmt.Sprintf("\n    if (%s !== %s.toLowerCase()) throw new __e([{path:%q,expected:\"lowercase\",received:%s}]);",
				paramName, paramName, paramName, paramName))
		}
	}

	if atomic == "number" {
		if c.Minimum != nil {
			v := formatNumber(*c.Minimum)
			b.WriteString(fmt.Sprintf("\n    if (%s < %s) throw new __e([{path:%q,expected:\"minimum %s\",received:\"\"+%s}]);",
				paramName, v, paramName, v, paramName))
		}
		if c.Maximum != nil {
			v := formatNumber(*c.Maximum)
			b.WriteString(fmt.Sprintf("\n    if (%s > %s) throw new __e([{path:%q,expected:\"maximum %s\",received:\"\"+%s}]);",
				paramName, v, paramName, v, paramName))
		}
		if c.MultipleOf != nil {
			v := *c.MultipleOf
			// Only emit the simple integer modulo form. Non-integer multipleOf
			// requires an epsilon comparison (see validate_constraints.go) and
			// is out of scope for this scalar fast-path — skipped silently.
			if v == float64(int64(v)) {
				vs := formatNumber(v)
				b.WriteString(fmt.Sprintf("\n    if (%s %% %s !== 0) throw new __e([{path:%q,expected:\"multipleOf %s\",received:\"\"+%s}]);",
					paramName, vs, paramName, vs, paramName))
			}
		}
	}

	return b.String()
}

// formatNumber renders a float64 as the shortest JS-equivalent literal: integers
// print without a trailing ".0", non-integers use %g.
func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}
