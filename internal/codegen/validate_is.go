package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// generateIsFunction generates a pure boolean type-check function with zero allocations.
// Returns a single expression composed with && chains.
func generateIsFunction(e *Emitter, typeName string, meta *metadata.Metadata, registry *metadata.TypeRegistry, ctx *validateCtx) {
	fnName := "is" + typeName
	e.Block("export function %s(input)", fnName)
	expr := generateIsExpr("input", meta, registry, 0, ctx)
	e.Line("return %s;", expr)
	e.EndBlock()
}

// generateIsExpr returns a JS boolean expression that checks if the value at `accessor`
// matches the given type. Composes with && for objects, || for unions.
func generateIsExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx) string {
	// Handle nullable/optional wrapping
	if meta.Nullable && meta.Optional {
		inner := *meta
		inner.Nullable = false
		inner.Optional = false
		innerExpr := generateIsExpr(accessor, &inner, registry, depth, ctx)
		return fmt.Sprintf("(%s === null || %s === undefined || %s)", accessor, accessor, innerExpr)
	}
	if meta.Nullable {
		inner := *meta
		inner.Nullable = false
		innerExpr := generateIsExpr(accessor, &inner, registry, depth, ctx)
		return fmt.Sprintf("(%s === null || %s)", accessor, innerExpr)
	}
	if meta.Optional {
		inner := *meta
		inner.Optional = false
		innerExpr := generateIsExpr(accessor, &inner, registry, depth, ctx)
		return fmt.Sprintf("(%s === undefined || %s)", accessor, innerExpr)
	}

	return generateIsExprInner(accessor, meta, registry, depth, ctx)
}

func generateIsExprInner(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx) string {
	switch meta.Kind {
	case metadata.KindAtomic:
		return generateIsAtomicExpr(accessor, meta)

	case metadata.KindLiteral:
		return fmt.Sprintf("%s === %s", accessor, jsLiteral(meta.LiteralValue))

	case metadata.KindObject:
		return generateIsObjectExpr(accessor, meta, registry, depth, ctx)

	case metadata.KindArray:
		return generateIsArrayExpr(accessor, meta, registry, depth, ctx)

	case metadata.KindTuple:
		parts := []string{fmt.Sprintf("Array.isArray(%s)", accessor)}
		for i, elem := range meta.Elements {
			if elem.Rest {
				continue
			}
			elemAccessor := fmt.Sprintf("%s[%d]", accessor, i)
			if elem.Optional {
				parts = append(parts, fmt.Sprintf("(%s.length <= %d || %s)", accessor, i, generateIsExpr(elemAccessor, &elem.Type, registry, depth+1, ctx)))
			} else {
				parts = append(parts, generateIsExpr(elemAccessor, &elem.Type, registry, depth+1, ctx))
			}
		}
		return "(" + strings.Join(parts, " && ") + ")"

	case metadata.KindUnion:
		return generateIsUnionExpr(accessor, meta, registry, depth, ctx)

	case metadata.KindRef:
		if ctx != nil && ctx.generating[meta.Ref] {
			// Recursive ref — call is function
			return fmt.Sprintf("is%s(%s)", meta.Ref, accessor)
		}
		if resolved, ok := registry.Types[meta.Ref]; ok {
			if ctx != nil {
				ctx.generating[meta.Ref] = true
			}
			result := generateIsExpr(accessor, resolved, registry, depth, ctx)
			if ctx != nil {
				delete(ctx.generating, meta.Ref)
			}
			return result
		}
		return "true"

	case metadata.KindEnum:
		if len(meta.EnumValues) > 0 {
			vals := make([]string, len(meta.EnumValues))
			for i, ev := range meta.EnumValues {
				vals[i] = fmt.Sprintf("%s === %s", accessor, jsLiteral(ev.Value))
			}
			return "(" + strings.Join(vals, " || ") + ")"
		}
		return "true"

	case metadata.KindNative:
		switch meta.NativeType {
		case "Date":
			return fmt.Sprintf("(%s instanceof Date && !isNaN(%s.getTime()))", accessor, accessor)
		default:
			return fmt.Sprintf("(%s instanceof %s)", accessor, meta.NativeType)
		}

	case metadata.KindAny, metadata.KindUnknown:
		return "true"

	case metadata.KindNever:
		return "false"

	case metadata.KindVoid:
		return fmt.Sprintf("%s === undefined", accessor)

	case metadata.KindIntersection:
		parts := make([]string, len(meta.IntersectionMembers))
		for i, member := range meta.IntersectionMembers {
			memberCopy := member
			parts[i] = generateIsExprInner(accessor, &memberCopy, registry, depth+1, ctx)
		}
		if len(parts) == 0 {
			return "true"
		}
		return "(" + strings.Join(parts, " && ") + ")"

	default:
		return "true"
	}
}

func generateIsAtomicExpr(accessor string, meta *metadata.Metadata) string {
	var parts []string
	switch meta.Atomic {
	case "string":
		parts = append(parts, fmt.Sprintf("typeof %s === \"string\"", accessor))
	case "number":
		parts = append(parts, fmt.Sprintf("typeof %s === \"number\" && Number.isFinite(%s)", accessor, accessor))
	case "boolean":
		parts = append(parts, fmt.Sprintf("typeof %s === \"boolean\"", accessor))
	case "bigint":
		parts = append(parts, fmt.Sprintf("typeof %s === \"bigint\"", accessor))
	default:
		return "true"
	}
	if meta.Atomic == "string" && meta.TemplatePattern != "" {
		parts = append(parts, fmt.Sprintf("/%s/.test(%s)", escapeForRegexLiteral(meta.TemplatePattern), accessor))
	}
	return strings.Join(parts, " && ")
}

func generateIsObjectExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx) string {
	parts := []string{fmt.Sprintf("typeof %s === \"object\" && %s !== null", accessor, accessor)}
	for _, prop := range meta.Properties {
		propAccessor := jsPropAccess(accessor, prop.Name)
		if prop.Required && !prop.Type.Optional {
			parts = append(parts, fmt.Sprintf("%s !== undefined", propAccessor))
			parts = append(parts, generateIsExpr(propAccessor, &prop.Type, registry, depth+1, ctx))
		} else {
			// Optional — only validate if present
			innerExpr := generateIsExpr(propAccessor, &prop.Type, registry, depth+1, ctx)
			if prop.ExactOptional {
				parts = append(parts, fmt.Sprintf("(!(%q in %s) || %s)", prop.Name, accessor, innerExpr))
			} else {
				parts = append(parts, fmt.Sprintf("(%s === undefined || %s)", propAccessor, innerExpr))
			}
		}
		// Inline constraint checks for is()
		if prop.Constraints != nil {
			constraintExprs := generateIsConstraintExprs(propAccessor, &prop)
			parts = append(parts, constraintExprs...)
		}
	}
	return "(" + strings.Join(parts, " && ") + ")"
}

// generateIsConstraintExprs returns JS boolean expressions for constraints.
func generateIsConstraintExprs(accessor string, prop *metadata.Property) []string {
	c := prop.Constraints
	if c == nil {
		return nil
	}
	var exprs []string

	if c.Minimum != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"number\" || %s >= %v)", accessor, accessor, *c.Minimum))
	}
	if c.Maximum != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"number\" || %s <= %v)", accessor, accessor, *c.Maximum))
	}
	if c.ExclusiveMinimum != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"number\" || %s > %v)", accessor, accessor, *c.ExclusiveMinimum))
	}
	if c.ExclusiveMaximum != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"number\" || %s < %v)", accessor, accessor, *c.ExclusiveMaximum))
	}
	if c.MinLength != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"string\" || %s.length >= %d)", accessor, accessor, *c.MinLength))
	}
	if c.MaxLength != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"string\" || %s.length <= %d)", accessor, accessor, *c.MaxLength))
	}
	if c.Pattern != nil {
		exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"string\" || /%s/.test(%s))", accessor, escapeForRegexLiteral(*c.Pattern), accessor))
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
			exprs = append(exprs, fmt.Sprintf("(typeof %s !== \"string\" || %s.test(%s))", accessor, regexLiteral, accessor))
		}
	}

	return exprs
}

func generateIsArrayExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx) string {
	if meta.ElementType == nil {
		return fmt.Sprintf("Array.isArray(%s)", accessor)
	}
	elemVar := fmt.Sprintf("_v%d", depth)
	elemExpr := generateIsExpr(elemVar, meta.ElementType, registry, depth+1, ctx)
	return fmt.Sprintf("(Array.isArray(%s) && %s.every(%s => %s))", accessor, accessor, elemVar, elemExpr)
}

func generateIsUnionExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx) string {
	// Literal unions: direct === checks
	allLit := true
	for _, m := range meta.UnionMembers {
		if m.Kind != metadata.KindLiteral {
			allLit = false
			break
		}
	}
	if allLit && len(meta.UnionMembers) > 0 {
		checks := make([]string, len(meta.UnionMembers))
		for i, m := range meta.UnionMembers {
			checks[i] = fmt.Sprintf("%s === %s", accessor, jsLiteral(m.LiteralValue))
		}
		return "(" + strings.Join(checks, " || ") + ")"
	}

	// Discriminated unions
	if meta.Discriminant != nil && len(meta.Discriminant.Mapping) > 0 {
		return generateIsDiscriminatedUnionExpr(accessor, meta, registry, depth, ctx)
	}

	// General unions: || of member checks
	parts := make([]string, len(meta.UnionMembers))
	for i, m := range meta.UnionMembers {
		memberCopy := m
		parts[i] = generateIsExprInner(accessor, &memberCopy, registry, depth+1, ctx)
	}
	return "(" + strings.Join(parts, " || ") + ")"
}

func generateIsDiscriminatedUnionExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *validateCtx) string {
	// Use IIFE with switch for discriminated unions
	disc := meta.Discriminant
	discAccessor := jsPropAccess(accessor, disc.Property)

	keys := make([]string, 0, len(disc.Mapping))
	for k := range disc.Mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	parts = append(parts, fmt.Sprintf("(typeof %s === \"object\" && %s !== null && (function() { switch (%s) {", accessor, accessor, discAccessor))
	for _, val := range keys {
		idx := disc.Mapping[val]
		if idx < 0 || idx >= len(meta.UnionMembers) {
			continue
		}
		member := meta.UnionMembers[idx]
		memberExpr := generateIsExprInner(accessor, &member, registry, depth+1, ctx)
		parts = append(parts, fmt.Sprintf("case %s: return %s;", jsLiteral(val), memberExpr))
	}
	parts = append(parts, "default: return false; } }()))")
	return strings.Join(parts, " ")
}
