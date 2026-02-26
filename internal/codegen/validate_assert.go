package codegen

import (
	"fmt"
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// generateAssertFunction generates a standalone assert function that throws on first error.
// For non-recursive types: direct inline checks with throw.
// For recursive types: inner function with path parameter.
func generateAssertFunction(e *Emitter, typeName string, meta *metadata.Metadata, registry *metadata.TypeRegistry, ctx *validateCtx) {
	fnName := "assert" + typeName

	// Check if type is recursive
	isRecursive := isRecursiveType(typeName, meta, registry)

	if isRecursive {
		// Generate inner function with path parameter
		innerFn := "_assert" + typeName
		e.Block("function %s(input, _path)", innerFn)
		generateAssertChecks(e, "input", "_path", meta, registry, 0, ctx, true)
		e.Line("return input;")
		e.EndBlock()
		e.Block("export function %s(input)", fnName)
		e.Line("return %s(input, \"input\");", innerFn)
		e.EndBlock()
	} else {
		e.Block("export function %s(input)", fnName)
		generateAssertChecks(e, "input", "\"input\"", meta, registry, 0, ctx, false)
		e.Line("return input;")
		e.EndBlock()
	}
}

// generateAssertChecks generates throw-on-first-error checks for assertX().
func generateAssertChecks(e *Emitter, accessor string, pathExpr string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx, isRecursive bool) {
	// Handle nullable/optional
	if meta.Nullable && meta.Optional {
		e.Block("if (%s !== null && %s !== undefined)", accessor, accessor)
		generateAssertChecksInner(e, accessor, pathExpr, meta, registry, depth, ctx, isRecursive)
		e.EndBlock()
		return
	}
	if meta.Nullable {
		e.Block("if (%s !== null)", accessor)
		generateAssertChecksInner(e, accessor, pathExpr, meta, registry, depth, ctx, isRecursive)
		e.EndBlock()
		return
	}
	if meta.Optional {
		e.Block("if (%s !== undefined)", accessor)
		generateAssertChecksInner(e, accessor, pathExpr, meta, registry, depth, ctx, isRecursive)
		e.EndBlock()
		return
	}
	generateAssertChecksInner(e, accessor, pathExpr, meta, registry, depth, ctx, isRecursive)
}

func generateAssertChecksInner(e *Emitter, accessor string, pathExpr string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx, isRecursive bool) {
	switch meta.Kind {
	case metadata.KindAtomic:
		switch meta.Atomic {
		case "string":
			e.Block("if (typeof %s !== \"string\")", accessor)
			emitAssertThrow(e, pathExpr, "string", fmt.Sprintf("typeof %s", accessor))
			e.EndBlock()
		case "number":
			e.Block("if (typeof %s !== \"number\" || !Number.isFinite(%s))", accessor, accessor)
			emitAssertThrow(e, pathExpr, "number", fmt.Sprintf("typeof %s", accessor))
			e.EndBlock()
		case "boolean":
			e.Block("if (typeof %s !== \"boolean\")", accessor)
			emitAssertThrow(e, pathExpr, "boolean", fmt.Sprintf("typeof %s", accessor))
			e.EndBlock()
		case "bigint":
			e.Block("if (typeof %s !== \"bigint\")", accessor)
			emitAssertThrow(e, pathExpr, "bigint", fmt.Sprintf("typeof %s", accessor))
			e.EndBlock()
		}
		if meta.Atomic == "string" && meta.TemplatePattern != "" {
			e.Block("if (typeof %s === \"string\" && !/%s/.test(%s))", accessor, escapeForRegexLiteral(meta.TemplatePattern), accessor)
			emitAssertThrow(e, pathExpr, fmt.Sprintf("pattern %s", jsStringEscape(meta.TemplatePattern)), fmt.Sprintf("\"\\\"\" + %s + \"\\\"\"", accessor))
			e.EndBlock()
		}

	case metadata.KindLiteral:
		e.Block("if (%s !== %s)", accessor, jsLiteral(meta.LiteralValue))
		emitAssertThrow(e, pathExpr, jsStringEscape(fmt.Sprintf("%v", meta.LiteralValue)), fmt.Sprintf("String(%s)", accessor))
		e.EndBlock()

	case metadata.KindObject:
		e.Block("if (typeof %s !== \"object\" || %s === null)", accessor, accessor)
		emitAssertThrow(e, pathExpr, "object", fmt.Sprintf("typeof %s", accessor))
		e.EndBlockSuffix(" else {")
		e.indent++
		for _, prop := range meta.Properties {
			propAccessor := jsPropAccess(accessor, prop.Name)
			var propPathExpr string
			if isRecursive {
				propPathExpr = fmt.Sprintf("%s + %q", pathExpr, jsPropPathSuffix(prop.Name))
			} else {
				propPathExpr = fmt.Sprintf("%s + %q", pathExpr, jsPropPathSuffix(prop.Name))
			}
			if prop.Required && !prop.Type.Optional {
				e.Block("if (%s === undefined)", propAccessor)
				emitAssertThrow(e, propPathExpr, describeType(&prop.Type), "\"undefined\"")
				e.EndBlockSuffix(" else {")
				e.indent++
				emitAssertPreChecks(e, propAccessor, &prop)
				generateAssertChecks(e, propAccessor, propPathExpr, &prop.Type, registry, depth+1, ctx, isRecursive)
				generateAssertConstraintChecks(e, propAccessor, propPathExpr, &prop)
				e.indent--
				e.Line("}")
			} else if prop.ExactOptional {
				e.Block("if (%q in %s)", prop.Name, accessor)
				e.Block("if (%s === undefined)", propAccessor)
				emitAssertThrow(e, propPathExpr, describeType(&prop.Type), "\"explicit undefined\"")
				e.EndBlockSuffix(" else {")
				e.indent++
				emitAssertPreChecks(e, propAccessor, &prop)
				generateAssertChecks(e, propAccessor, propPathExpr, &prop.Type, registry, depth+1, ctx, isRecursive)
				generateAssertConstraintChecks(e, propAccessor, propPathExpr, &prop)
				e.indent--
				e.Line("}")
				e.EndBlock()
			} else {
				emitAssertPreChecks(e, propAccessor, &prop)
				generateAssertChecks(e, propAccessor, propPathExpr, &prop.Type, registry, depth+1, ctx, isRecursive)
				if prop.Constraints != nil {
					if prop.Type.Optional || !prop.Required {
						e.Block("if (%s !== undefined)", propAccessor)
						generateAssertConstraintChecks(e, propAccessor, propPathExpr, &prop)
						e.EndBlock()
					} else {
						generateAssertConstraintChecks(e, propAccessor, propPathExpr, &prop)
					}
				}
			}
		}
		e.indent--
		e.Line("}")

	case metadata.KindArray:
		e.Block("if (!Array.isArray(%s))", accessor)
		emitAssertThrow(e, pathExpr, "array", fmt.Sprintf("typeof %s", accessor))
		e.EndBlockSuffix(" else {")
		e.indent++
		if meta.ElementType != nil {
			idx := fmt.Sprintf("i%d", depth)
			e.Block("for (let %s = 0; %s < %s.length; %s++)", idx, idx, accessor, idx)
			elemAccessor := fmt.Sprintf("%s[%s]", accessor, idx)
			elemPathExpr := fmt.Sprintf("%s + \"[\" + %s + \"]\"", pathExpr, idx)
			generateAssertChecks(e, elemAccessor, elemPathExpr, meta.ElementType, registry, depth+1, ctx, isRecursive)
			e.EndBlock()
		}
		e.indent--
		e.Line("}")

	case metadata.KindTuple:
		e.Block("if (!Array.isArray(%s))", accessor)
		emitAssertThrow(e, pathExpr, "tuple", fmt.Sprintf("typeof %s", accessor))
		e.EndBlock()
		for i, elem := range meta.Elements {
			if elem.Rest {
				continue
			}
			elemAccessor := fmt.Sprintf("%s[%d]", accessor, i)
			elemPathExpr := fmt.Sprintf("%s + \"[%d]\"", pathExpr, i)
			if elem.Optional {
				e.Block("if (%s.length > %d)", accessor, i)
				generateAssertChecks(e, elemAccessor, elemPathExpr, &elem.Type, registry, depth+1, ctx, isRecursive)
				e.EndBlock()
			} else {
				generateAssertChecks(e, elemAccessor, elemPathExpr, &elem.Type, registry, depth+1, ctx, isRecursive)
			}
		}

	case metadata.KindUnion:
		// For literal unions, inline check
		allLit := true
		for _, m := range meta.UnionMembers {
			if m.Kind != metadata.KindLiteral {
				allLit = false
				break
			}
		}
		if allLit && len(meta.UnionMembers) > 0 {
			vals := make([]string, len(meta.UnionMembers))
			for i, m := range meta.UnionMembers {
				vals[i] = jsLiteral(m.LiteralValue)
			}
			checks := make([]string, len(vals))
			for i, v := range vals {
				checks[i] = fmt.Sprintf("%s === %s", accessor, v)
			}
			e.Block("if (!(%s))", strings.Join(checks, " || "))
			emitAssertThrow(e, pathExpr, jsStringEscape(strings.Join(vals, " | ")), fmt.Sprintf("String(%s)", accessor))
			e.EndBlock()
		} else {
			// For complex unions, delegate to validate + throw
			e.Line("// complex union — delegate to validate")
		}

	case metadata.KindRef:
		if ctx != nil && ctx.generating[meta.Ref] {
			// Recursive ref — call inner assert function
			innerFn := "_assert" + meta.Ref
			e.Line("%s(%s, %s);", innerFn, accessor, pathExpr)
		} else if resolved, ok := registry.Types[meta.Ref]; ok {
			if ctx != nil {
				ctx.generating[meta.Ref] = true
			}
			generateAssertChecks(e, accessor, pathExpr, resolved, registry, depth, ctx, isRecursive)
			if ctx != nil {
				delete(ctx.generating, meta.Ref)
			}
		}

	case metadata.KindEnum:
		if len(meta.EnumValues) > 0 {
			vals := make([]string, len(meta.EnumValues))
			for i, ev := range meta.EnumValues {
				vals[i] = jsLiteral(ev.Value)
			}
			setExpr := "[" + strings.Join(vals, ", ") + "]"
			e.Block("if (!%s.includes(%s))", setExpr, accessor)
			emitAssertThrow(e, pathExpr, "enum value", fmt.Sprintf("String(%s)", accessor))
			e.EndBlock()
		}

	case metadata.KindNative:
		switch meta.NativeType {
		case "Date":
			e.Block("if (!(%s instanceof Date) || isNaN(%s.getTime()))", accessor, accessor)
			emitAssertThrow(e, pathExpr, "Date", fmt.Sprintf("typeof %s", accessor))
			e.EndBlock()
		default:
			e.Block("if (!(%s instanceof %s))", accessor, meta.NativeType)
			emitAssertThrow(e, pathExpr, meta.NativeType, fmt.Sprintf("typeof %s", accessor))
			e.EndBlock()
		}

	case metadata.KindAny, metadata.KindUnknown:
		// No validation

	case metadata.KindNever:
		emitAssertThrow(e, pathExpr, "never", "\"present\"")

	case metadata.KindVoid:
		e.Block("if (%s !== undefined)", accessor)
		emitAssertThrow(e, pathExpr, "void", fmt.Sprintf("typeof %s", accessor))
		e.EndBlock()

	case metadata.KindIntersection:
		for _, member := range meta.IntersectionMembers {
			memberCopy := member
			generateAssertChecksInner(e, accessor, pathExpr, &memberCopy, registry, depth, ctx, isRecursive)
		}
	}
}

// emitAssertThrow emits a throw statement with TsgonestValidationError (__e).
// pathExpr is a JS expression evaluating to the field path string.
// expected is a Go string literal describing the expected type/value.
// receivedExpr is a JS expression evaluating to a string describing the received value.
func emitAssertThrow(e *Emitter, pathExpr string, expected string, receivedExpr string) {
	e.Line("throw new __e([{path: %s, expected: \"%s\", received: %s}]);", pathExpr, expected, receivedExpr)
}

// emitAssertPreChecks emits transforms and coercion that must run BEFORE type checks.
// Called at the property level before generateAssertChecks so that string values
// are coerced to number/boolean before the typeof check rejects them.
func emitAssertPreChecks(e *Emitter, accessor string, prop *metadata.Property) {
	c := prop.Constraints
	if c == nil {
		return
	}
	if len(c.Transforms) > 0 {
		generateTransforms(e, accessor, c.Transforms)
	}
	if c.Coerce != nil && *c.Coerce {
		generateCoercion(e, accessor, &prop.Type)
	}
}

// generateAssertConstraintChecks emits assert-style constraint checks (throw on first failure).
// Uses custom error messages when configured (per-constraint or global).
// Note: transforms and coercion are emitted by emitAssertPreChecks (called before type checks).
func generateAssertConstraintChecks(e *Emitter, accessor string, pathExpr string, prop *metadata.Property) {
	c := prop.Constraints
	if c == nil {
		return
	}

	// Error message helper matching validate behavior
	errMsg := func(constraintKey string, defaultMsg string) string {
		if c.Errors != nil {
			if msg, ok := c.Errors[constraintKey]; ok {
				return jsStringEscape(msg)
			}
		}
		if c.ErrorMessage != nil {
			return jsStringEscape(*c.ErrorMessage)
		}
		return defaultMsg
	}

	if c.Minimum != nil {
		e.Block("if (typeof %s === \"number\" && %s < %v)", accessor, accessor, *c.Minimum)
		emitAssertThrow(e, pathExpr, errMsg("minimum", fmt.Sprintf("minimum %v", *c.Minimum)), fmt.Sprintf("String(%s)", accessor))
		e.EndBlock()
	}
	if c.Maximum != nil {
		e.Block("if (typeof %s === \"number\" && %s > %v)", accessor, accessor, *c.Maximum)
		emitAssertThrow(e, pathExpr, errMsg("maximum", fmt.Sprintf("maximum %v", *c.Maximum)), fmt.Sprintf("String(%s)", accessor))
		e.EndBlock()
	}
	if c.MinLength != nil {
		e.Block("if (typeof %s === \"string\" && %s.length < %d)", accessor, accessor, *c.MinLength)
		emitAssertThrow(e, pathExpr, errMsg("minLength", fmt.Sprintf("minLength %d", *c.MinLength)), fmt.Sprintf("\"length \" + %s.length", accessor))
		e.EndBlock()
	}
	if c.MaxLength != nil {
		e.Block("if (typeof %s === \"string\" && %s.length > %d)", accessor, accessor, *c.MaxLength)
		emitAssertThrow(e, pathExpr, errMsg("maxLength", fmt.Sprintf("maxLength %d", *c.MaxLength)), fmt.Sprintf("\"length \" + %s.length", accessor))
		e.EndBlock()
	}
	if c.Pattern != nil {
		e.Block("if (typeof %s === \"string\" && !/%s/.test(%s))", accessor, escapeForRegexLiteral(*c.Pattern), accessor)
		emitAssertThrow(e, pathExpr, errMsg("pattern", fmt.Sprintf("pattern %s", jsStringEscape(*c.Pattern))), fmt.Sprintf("\"\\\"\" + %s + \"\\\"\"", accessor))
		e.EndBlock()
	}
	if c.Format != nil {
		pattern, ok := formatRegexes[*c.Format]
		if ok && pattern != "" {
			flags := formatFlags[*c.Format]
			var regexLiteral string
			if flags != "" {
				regexLiteral = fmt.Sprintf("/%s/%s", pattern, flags)
			} else {
				regexLiteral = fmt.Sprintf("/%s/", pattern)
			}
			e.Block("if (typeof %s === \"string\" && !%s.test(%s))", accessor, regexLiteral, accessor)
			emitAssertThrow(e, pathExpr, errMsg("format", fmt.Sprintf("format %s", *c.Format)), fmt.Sprintf("\"\\\"\" + %s + \"\\\"\"", accessor))
			e.EndBlock()
		}
	}
}
