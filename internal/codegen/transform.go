package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// transformCtx tracks codegen state to prevent infinite recursion on recursive types.
type transformCtx struct {
	// generating tracks type names currently being generated to detect recursion.
	generating map[string]bool
}

// generateTransformFunction generates: export function transform<Name>(input) { return ...; }
func generateTransformFunction(e *Emitter, typeName string, meta *metadata.Metadata, registry *metadata.TypeRegistry, ctx *transformCtx) {
	fnName := "transform" + typeName
	e.Block("export function %s(input)", fnName)
	e.Line("return %s;", generateTransformExpr("input", meta, registry, 0, ctx))
	e.EndBlock()
}

// generateTransformExpr returns a JS expression that transforms the value at `accessor`,
// stripping extra properties and returning a clean plain object.
func generateTransformExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	// Handle nullable/optional wrapping
	if meta.Nullable || meta.Optional {
		inner := *meta
		inner.Nullable = false
		inner.Optional = false
		nullCheck := accessor + " == null"
		return fmt.Sprintf("(%s ? null : %s)", nullCheck, generateTransformExpr(accessor, &inner, registry, depth, ctx))
	}

	switch meta.Kind {
	case metadata.KindAtomic, metadata.KindLiteral, metadata.KindEnum:
		// Pass through: primitives, literals, enums need no transformation
		return accessor

	case metadata.KindObject:
		return generateTransformObject(accessor, meta, registry, depth, ctx)

	case metadata.KindArray:
		return generateTransformArray(accessor, meta, registry, depth, ctx)

	case metadata.KindRef:
		// For recursive references, emit a function call to prevent infinite codegen recursion
		if ctx != nil && ctx.generating[meta.Ref] {
			fnName := "transform" + meta.Ref
			return fmt.Sprintf("%s(%s)", fnName, accessor)
		}
		if resolved, ok := registry.Types[meta.Ref]; ok {
			if ctx != nil {
				ctx.generating[meta.Ref] = true
			}
			result := generateTransformExpr(accessor, resolved, registry, depth, ctx)
			if ctx != nil {
				delete(ctx.generating, meta.Ref)
			}
			return result
		}
		// Unknown ref: pass through
		return accessor

	case metadata.KindUnion:
		return generateTransformUnion(accessor, meta, registry, depth, ctx)

	case metadata.KindNative:
		// Date and other natives: pass through
		return accessor

	case metadata.KindAny, metadata.KindUnknown:
		// Any/unknown: pass through
		return accessor

	default:
		return accessor
	}
}

// generateTransformObject generates a plain object literal with only declared properties.
func generateTransformObject(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	if len(meta.Properties) == 0 {
		// Objects with no properties but possibly an index signature: pass through
		if meta.IndexSignature != nil {
			return accessor
		}
		return "{}"
	}

	// If the object has an index signature, we can't strip unknown keys
	if meta.IndexSignature != nil {
		return accessor
	}

	// Check if any property is optional
	hasOptional := false
	for _, prop := range meta.Properties {
		if prop.Type.Optional || !prop.Required {
			hasOptional = true
			break
		}
	}

	if !hasOptional {
		return generateTransformObjectAllRequired(accessor, meta, registry, depth, ctx)
	}

	return generateTransformObjectWithOptional(accessor, meta, registry, depth, ctx)
}

// generateTransformObjectAllRequired generates a plain object literal for objects
// where all properties are required.
func generateTransformObjectAllRequired(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	var parts []string
	for _, prop := range meta.Properties {
		propAccessor := accessor + "." + prop.Name
		valExpr := generateTransformExpr(propAccessor, &prop.Type, registry, depth+1, ctx)
		parts = append(parts, fmt.Sprintf("%s: %s", prop.Name, valExpr))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// generateTransformObjectWithOptional generates an IIFE that conditionally includes optional properties.
func generateTransformObjectWithOptional(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	var buf strings.Builder
	buf.WriteString("(function() { var _r = {")

	// Required properties first
	first := true
	for _, prop := range meta.Properties {
		if prop.Type.Optional || !prop.Required {
			continue
		}
		if !first {
			buf.WriteString(", ")
		}
		propAccessor := accessor + "." + prop.Name
		valExpr := generateTransformExpr(propAccessor, &prop.Type, registry, depth+1, ctx)
		buf.WriteString(fmt.Sprintf("%s: %s", prop.Name, valExpr))
		first = false
	}

	buf.WriteString("};")

	// Optional properties with conditional inclusion
	for _, prop := range meta.Properties {
		if !prop.Type.Optional && prop.Required {
			continue
		}
		propAccessor := accessor + "." + prop.Name
		valExpr := generateTransformExpr(propAccessor, &prop.Type, registry, depth+1, ctx)
		buf.WriteString(fmt.Sprintf(" if (%s !== undefined) _r.%s = %s;", propAccessor, prop.Name, valExpr))
	}

	buf.WriteString(" return _r; }())")
	return buf.String()
}

// generateTransformArray generates a .map() call for arrays.
func generateTransformArray(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	if meta.ElementType == nil {
		return accessor
	}

	elemVar := fmt.Sprintf("_v%d", depth)
	elemExpr := generateTransformExpr(elemVar, meta.ElementType, registry, depth+1, ctx)

	// If the element expression is just the variable itself, no transform needed
	if elemExpr == elemVar {
		return accessor
	}

	return fmt.Sprintf("%s.map(%s => %s)", accessor, elemVar, elemExpr)
}

// generateTransformUnion generates transformation for union types.
func generateTransformUnion(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	members := meta.UnionMembers
	if len(members) == 0 {
		return accessor
	}

	// Discriminated union: switch on discriminant
	if meta.Discriminant != nil && len(meta.Discriminant.Mapping) > 0 {
		return generateTransformDiscriminatedUnion(accessor, meta, registry, depth, ctx)
	}

	// Nullable union: T | null
	if nonNull, ok := extractNullableUnion(members); ok {
		innerExpr := generateTransformExpr(accessor, nonNull, registry, depth, ctx)
		return fmt.Sprintf("(%s == null ? null : %s)", accessor, innerExpr)
	}

	// Literal/atomic unions: pass through
	if allLiterals(members) || allAtomics(members) {
		return accessor
	}

	// Complex unions we can't handle: pass through
	return accessor
}

// generateTransformDiscriminatedUnion emits a switch on the discriminant property.
func generateTransformDiscriminatedUnion(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *transformCtx) string {
	disc := meta.Discriminant
	discAccessor := fmt.Sprintf("%s[%q]", accessor, disc.Property)

	// Collect sorted keys for deterministic output
	keys := make([]string, 0, len(disc.Mapping))
	for k := range disc.Mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	parts = append(parts, fmt.Sprintf("(function() { switch (%s) {", discAccessor))

	for _, val := range keys {
		idx := disc.Mapping[val]
		if idx < 0 || idx >= len(meta.UnionMembers) {
			continue
		}
		member := meta.UnionMembers[idx]
		memberExpr := generateTransformExpr(accessor, &member, registry, depth+1, ctx)
		parts = append(parts, fmt.Sprintf("case %s: return %s;", jsLiteral(val), memberExpr))
	}

	parts = append(parts, fmt.Sprintf("default: return %s; } }())", accessor))
	return strings.Join(parts, " ")
}
