package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// serializeCtx tracks codegen state to prevent infinite recursion on recursive types.
type serializeCtx struct {
	// generating tracks type names currently being generated to detect recursion.
	generating map[string]bool
}

// generateSerializeFunction generates: export function serialize<Name>(input) { ... }
func generateSerializeFunction(e *Emitter, typeName string, meta *metadata.Metadata, registry *metadata.TypeRegistry, ctx *serializeCtx) {
	fnName := "serialize" + typeName
	e.Block("export function %s(input)", fnName)
	e.Line("return %s;", generateSerializeExpr("input", meta, registry, 0, ctx))
	e.EndBlock()
}

// generateSerializeExpr returns a JS expression that serializes the value at `accessor`.
func generateSerializeExpr(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	// Handle nullable/optional wrapping
	if meta.Nullable || meta.Optional {
		inner := *meta
		inner.Nullable = false
		inner.Optional = false
		nullCheck := accessor + " == null"
		return fmt.Sprintf("(%s ? \"null\" : %s)", nullCheck, generateSerializeExpr(accessor, &inner, registry, depth, ctx))
	}

	switch meta.Kind {
	case metadata.KindAtomic:
		return generateSerializeAtomic(accessor, meta.Atomic)

	case metadata.KindLiteral:
		return generateSerializeLiteral(meta.LiteralValue)

	case metadata.KindObject:
		return generateSerializeObject(accessor, meta, registry, depth, ctx)

	case metadata.KindArray:
		return generateSerializeArray(accessor, meta, registry, depth, ctx)

	case metadata.KindTuple:
		return generateSerializeTuple(accessor, meta, registry, depth, ctx)

	case metadata.KindUnion:
		return generateSerializeUnion(accessor, meta, registry, depth, ctx)

	case metadata.KindRef:
		// For recursive references, emit a function call to prevent infinite codegen recursion
		if ctx != nil && ctx.generating[meta.Ref] {
			fnName := "serialize" + meta.Ref
			return fmt.Sprintf("%s(%s)", fnName, accessor)
		}
		if resolved, ok := registry.Types[meta.Ref]; ok {
			if ctx != nil {
				ctx.generating[meta.Ref] = true
			}
			result := generateSerializeExpr(accessor, resolved, registry, depth, ctx)
			if ctx != nil {
				delete(ctx.generating, meta.Ref)
			}
			return result
		}
		return fmt.Sprintf("JSON.stringify(%s)", accessor)

	case metadata.KindNative:
		return generateSerializeNative(accessor, meta)

	case metadata.KindIntersection:
		// For intersection types, if all members are objects, merge properties
		// Otherwise fallback to JSON.stringify
		allObjects := true
		for _, member := range meta.IntersectionMembers {
			resolved := &member
			if member.Kind == metadata.KindRef {
				if r, ok := registry.Types[member.Ref]; ok {
					resolved = r
				}
			}
			if resolved.Kind != metadata.KindObject {
				allObjects = false
				break
			}
		}
		if allObjects && len(meta.IntersectionMembers) > 0 {
			// Merge all properties into a single object metadata and serialize
			merged := &metadata.Metadata{Kind: metadata.KindObject}
			for _, member := range meta.IntersectionMembers {
				resolved := &member
				if member.Kind == metadata.KindRef {
					if r, ok := registry.Types[member.Ref]; ok {
						resolved = r
					}
				}
				merged.Properties = append(merged.Properties, resolved.Properties...)
			}
			return generateSerializeExpr(accessor, merged, registry, depth, ctx)
		}
		return fmt.Sprintf("JSON.stringify(%s)", accessor)

	case metadata.KindEnum:
		if len(meta.EnumValues) > 0 {
			allStrings := true
			allNumbers := true
			for _, ev := range meta.EnumValues {
				switch ev.Value.(type) {
				case string:
					allNumbers = false
				case float64:
					allStrings = false
				default:
					allStrings = false
					allNumbers = false
				}
			}
			if allStrings {
				return fmt.Sprintf("__s(%s)", accessor)
			}
			if allNumbers {
				return fmt.Sprintf("(Number.isFinite(%s) ? \"\" + %s : \"null\")", accessor, accessor)
			}
			// Mixed enum
			return fmt.Sprintf("(typeof %s === \"string\" ? __s(%s) : (Number.isFinite(%s) ? \"\" + %s : \"null\"))", accessor, accessor, accessor, accessor)
		}
		return fmt.Sprintf("JSON.stringify(%s)", accessor)

	default:
		return fmt.Sprintf("JSON.stringify(%s)", accessor)
	}
}

// generateSerializeAtomic generates serialization for atomic types.
// When inTemplate is true, the result will be used inside a template literal ${...},
// so numbers don't need explicit coercion.
func generateSerializeAtomic(accessor string, atomic string) string {
	return generateSerializeAtomicCtx(accessor, atomic, false)
}

func generateSerializeAtomicCtx(accessor string, atomic string, inTemplate bool) string {
	switch atomic {
	case "string":
		return fmt.Sprintf("__s(%s)", accessor)
	case "number":
		if inTemplate {
			return fmt.Sprintf("(Number.isFinite(%s) ? %s : \"null\")", accessor, accessor)
		}
		return fmt.Sprintf("(Number.isFinite(%s) ? \"\" + %s : \"null\")", accessor, accessor)
	case "bigint":
		if inTemplate {
			return accessor
		}
		return fmt.Sprintf("\"\" + %s", accessor)
	case "boolean":
		return fmt.Sprintf("(%s ? \"true\" : \"false\")", accessor)
	default:
		return fmt.Sprintf("JSON.stringify(%s)", accessor)
	}
}

func generateSerializeLiteral(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", fmt.Sprintf("%q", v)) // pre-encoded
	case float64:
		return fmt.Sprintf("\"%v\"", v)
	case bool:
		if v {
			return "\"true\""
		}
		return "\"false\""
	default:
		return fmt.Sprintf("\"%v\"", v)
	}
}

func generateSerializeObject(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	if len(meta.Properties) == 0 {
		return fmt.Sprintf("JSON.stringify(%s)", accessor)
	}

	// Check if any property is optional
	hasOptional := false
	for _, prop := range meta.Properties {
		if prop.Type.Optional || !prop.Required {
			hasOptional = true
			break
		}
	}

	// If all properties are required, use the fast inline concatenation approach
	if !hasOptional {
		return generateSerializeObjectAllRequired(accessor, meta, registry, depth, ctx)
	}

	// For objects with optional properties, use a parts-based approach
	// that conditionally includes keys
	return generateSerializeObjectWithOptional(accessor, meta, registry, depth, ctx)
}

// generateSerializeObjectAllRequired generates a template literal for objects
// where all properties are required (no conditional key inclusion needed).
// V8 pre-computes static segment sizes and fills dynamic parts in one pass.
func generateSerializeObjectAllRequired(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	var buf strings.Builder
	buf.WriteString("`{")

	for i, prop := range meta.Properties {
		propAccessor := jsPropAccess(accessor, prop.Name)

		if i > 0 {
			buf.WriteByte(',')
		}
		// Static key portion — use jsonKeyInTemplate for correct double-escaping
		buf.WriteString(fmt.Sprintf(`\"%s\":`, jsonKeyInTemplate(prop.Name)))
		// Dynamic value as ${...} interpolation
		valExpr := generateSerializeExprTemplate(propAccessor, &prop.Type, registry, depth+1, ctx)
		buf.WriteString("${")
		buf.WriteString(valExpr)
		buf.WriteString("}")
	}

	buf.WriteString("}`")
	return buf.String()
}

// generateSerializeExprTemplate returns a JS expression for use inside a template literal ${...}.
// For atomic number types, this avoids unnecessary "" + coercion since template literals
// handle it natively.
func generateSerializeExprTemplate(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	// Handle nullable/optional wrapping
	if meta.Nullable || meta.Optional {
		inner := *meta
		inner.Nullable = false
		inner.Optional = false
		nullCheck := accessor + " == null"
		return fmt.Sprintf("(%s ? \"null\" : %s)", nullCheck, generateSerializeExprTemplate(accessor, &inner, registry, depth, ctx))
	}

	// For atomic types, use template-aware version
	if meta.Kind == metadata.KindAtomic {
		return generateSerializeAtomicCtx(accessor, meta.Atomic, true)
	}

	// For all other types, delegate to normal expr generation
	return generateSerializeExpr(accessor, meta, registry, depth, ctx)
}

// generateSerializeObjectWithOptional generates serialization for objects with
// optional properties. Required properties are emitted first as a template literal,
// then each optional property is appended via ternary expressions. No IIFE needed.
func generateSerializeObjectWithOptional(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	// Partition into required and optional, preserving relative order within each group
	var required []metadata.Property
	var optional []metadata.Property
	for _, prop := range meta.Properties {
		if prop.Type.Optional || !prop.Required {
			optional = append(optional, prop)
		} else {
			required = append(required, prop)
		}
	}

	// Build template literal for required portion: `{"id":${...},"name":${...}`
	var buf strings.Builder
	buf.WriteString("`{")
	for i, prop := range required {
		propAccessor := jsPropAccess(accessor, prop.Name)
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(fmt.Sprintf(`\"%s\":`, jsonKeyInTemplate(prop.Name)))
		valExpr := generateSerializeExprTemplate(propAccessor, &prop.Type, registry, depth+1, ctx)
		buf.WriteString("${")
		buf.WriteString(valExpr)
		buf.WriteString("}")
	}
	buf.WriteString("`")

	// Append optional properties as ternary concatenation
	for _, prop := range optional {
		propAccessor := jsPropAccess(accessor, prop.Name)
		valExpr := generateSerializeExpr(propAccessor, &prop.Type, registry, depth+1, ctx)

		// Build the key string literal as a JS string: ",\"name\":"
		// Use jsonKeyInString for correct double-escaping (JSON layer + JS string layer)
		keyLiteral := fmt.Sprintf(`",\"%s\":"`, jsonKeyInString(prop.Name))
		buf.WriteString(fmt.Sprintf(` + (%s !== undefined ? %s + %s : "")`, propAccessor, keyLiteral, valExpr))
	}

	// If all props are optional, we might have a leading comma — handle this edge case
	if len(required) == 0 && len(optional) > 0 {
		// Wrap: result starts with `{` and may have leading comma after it
		// Use a trailing fixup: replace leading `{,` with `{`
		return fmt.Sprintf(`(function() { var _r = %s + "}"; return _r[1] === "," ? "{" + _r.slice(2) : _r; }())`, buf.String())
	}

	buf.WriteString(` + "}"`)
	return buf.String()
}

func generateSerializeArray(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	if meta.ElementType == nil {
		return fmt.Sprintf("JSON.stringify(%s)", accessor)
	}

	// Use __sa helper with arrow function for element serialization.
	// __sa is emitted as a companion-level helper alongside __s.
	elemVar := fmt.Sprintf("_v%d", depth)
	elemExpr := generateSerializeExpr(elemVar, meta.ElementType, registry, depth+1, ctx)

	return fmt.Sprintf(`__sa(%s, %s => %s)`, accessor, elemVar, elemExpr)
}

func generateSerializeTuple(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	if len(meta.Elements) == 0 {
		return "\"[]\""
	}

	// Use template literal for tuple serialization
	var buf strings.Builder
	buf.WriteString("`[")

	for i, elem := range meta.Elements {
		if i > 0 {
			buf.WriteByte(',')
		}
		elemAccessor := fmt.Sprintf("%s[%d]", accessor, i)
		valExpr := generateSerializeExprTemplate(elemAccessor, &elem.Type, registry, depth+1, ctx)
		buf.WriteString("${")
		buf.WriteString(valExpr)
		buf.WriteString("}")
	}

	buf.WriteString("]`")
	return buf.String()
}

func generateSerializeNative(accessor string, meta *metadata.Metadata) string {
	switch meta.NativeType {
	case "Date":
		return fmt.Sprintf("\"\\\"\" + %s.toISOString() + \"\\\"\"", accessor)
	default:
		return fmt.Sprintf("JSON.stringify(%s)", accessor)
	}
}

// generateSerializeUnion generates serialization for union types.
// Handles four patterns in priority order:
//  1. Discriminated unions (switch on discriminant property)
//  2. Literal unions (all members are literals → direct output)
//  3. Nullable unions (T | null → null check + serialize T)
//  4. Atomic unions (string | number → typeof switch)
//
// Falls back to JSON.stringify for complex unions that don't match any pattern.
func generateSerializeUnion(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	members := meta.UnionMembers
	if len(members) == 0 {
		return fmt.Sprintf("JSON.stringify(%s)", accessor)
	}

	// 1. Discriminated union: switch on discriminant, serialize each branch
	if meta.Discriminant != nil && len(meta.Discriminant.Mapping) > 0 {
		return generateSerializeDiscriminatedUnion(accessor, meta, registry, depth, ctx)
	}

	// 2. All-literal union: the value is already a primitive, just serialize it directly
	if allLiterals(members) {
		return generateSerializeLiteralUnion(accessor, members)
	}

	// 3. Nullable union: exactly one null/undefined + one non-null member
	if nonNull, ok := extractNullableUnion(members); ok {
		innerExpr := generateSerializeExpr(accessor, nonNull, registry, depth, ctx)
		return fmt.Sprintf("(%s == null ? \"null\" : %s)", accessor, innerExpr)
	}

	// 4. Atomic union: all members are atomic types (string | number | boolean etc.)
	if allAtomics(members) {
		return generateSerializeAtomicUnion(accessor, members)
	}

	// 5. Fallback: JSON.stringify
	return fmt.Sprintf("JSON.stringify(%s)", accessor)
}

// generateSerializeDiscriminatedUnion emits a switch on the discriminant property,
// serializing each branch with its specific schema. O(1) dispatch.
func generateSerializeDiscriminatedUnion(accessor string, meta *metadata.Metadata, registry *metadata.TypeRegistry, depth int, ctx *serializeCtx) string {
	disc := meta.Discriminant
	discAccessor := jsPropAccess(accessor, disc.Property)

	// Collect sorted keys for deterministic output
	keys := make([]string, 0, len(disc.Mapping))
	for k := range disc.Mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build switch expression as IIFE
	var parts []string
	parts = append(parts, fmt.Sprintf("(function() { switch (%s) {", discAccessor))

	for _, val := range keys {
		idx := disc.Mapping[val]
		if idx < 0 || idx >= len(meta.UnionMembers) {
			continue
		}
		member := meta.UnionMembers[idx]
		memberExpr := generateSerializeExpr(accessor, &member, registry, depth+1, ctx)
		parts = append(parts, fmt.Sprintf("case %s: return %s;", jsLiteral(val), memberExpr))
	}

	parts = append(parts, fmt.Sprintf("default: return JSON.stringify(%s); } }())", accessor))
	return strings.Join(parts, " ")
}

// generateSerializeLiteralUnion serializes a union where all members are literals.
// For string literals, use __s(); for number/boolean, direct coercion.
func generateSerializeLiteralUnion(accessor string, members []metadata.Metadata) string {
	// Check if all are string literals → use __s()
	allString := true
	for _, m := range members {
		if _, ok := m.LiteralValue.(string); !ok {
			allString = false
			break
		}
	}
	if allString {
		return fmt.Sprintf("__s(%s)", accessor)
	}

	// Mixed or numeric literals: use typeof switch
	return fmt.Sprintf("(typeof %s === \"string\" ? __s(%s) : \"\" + %s)", accessor, accessor, accessor)
}

// generateSerializeAtomicUnion serializes unions of atomic types using typeof dispatch.
func generateSerializeAtomicUnion(accessor string, members []metadata.Metadata) string {
	// Collect unique atomic types
	atomics := make(map[string]bool)
	for _, m := range members {
		if m.Kind == metadata.KindAtomic {
			atomics[m.Atomic] = true
		}
	}

	// Single atomic type (shouldn't happen but handle gracefully)
	if len(atomics) == 1 {
		for a := range atomics {
			return generateSerializeAtomic(accessor, a)
		}
	}

	// Build typeof chain. Most common case: string | number
	var cases []string
	if atomics["string"] {
		cases = append(cases, fmt.Sprintf("typeof %s === \"string\" ? __s(%s)", accessor, accessor))
	}
	if atomics["boolean"] {
		cases = append(cases, fmt.Sprintf("typeof %s === \"boolean\" ? (%s ? \"true\" : \"false\")", accessor, accessor))
	}

	// Number and bigint both just need coercion
	if atomics["number"] || atomics["bigint"] {
		if atomics["number"] && !atomics["bigint"] {
			cases = append(cases, fmt.Sprintf("(Number.isFinite(%s) ? \"\" + %s : \"null\")", accessor, accessor))
		} else if atomics["bigint"] && !atomics["number"] {
			cases = append(cases, fmt.Sprintf("\"\" + %s", accessor))
		} else {
			// Both number and bigint
			cases = append(cases, fmt.Sprintf("(typeof %s === \"number\" ? (Number.isFinite(%s) ? \"\" + %s : \"null\") : \"\" + %s)", accessor, accessor, accessor, accessor))
		}
	} else {
		// Final fallback
		cases = append(cases, fmt.Sprintf("JSON.stringify(%s)", accessor))
	}

	if len(cases) == 1 {
		return cases[0]
	}

	// Chain ternaries: cond1 ? expr1 : cond2 ? expr2 : fallback
	result := cases[len(cases)-1]
	for i := len(cases) - 2; i >= 0; i-- {
		result = fmt.Sprintf("(%s : %s)", cases[i], result)
	}
	return result
}

// allLiterals returns true if every union member is a literal.
func allLiterals(members []metadata.Metadata) bool {
	for _, m := range members {
		if m.Kind != metadata.KindLiteral {
			return false
		}
	}
	return true
}

// allAtomics returns true if every union member is an atomic type.
func allAtomics(members []metadata.Metadata) bool {
	for _, m := range members {
		if m.Kind != metadata.KindAtomic {
			return false
		}
	}
	return true
}

// extractNullableUnion checks if this is a T | null union (exactly one null and one non-null).
// Returns the non-null member if so.
func extractNullableUnion(members []metadata.Metadata) (*metadata.Metadata, bool) {
	if len(members) != 2 {
		return nil, false
	}

	isNull := func(m *metadata.Metadata) bool {
		return m.Kind == metadata.KindLiteral && m.LiteralValue == nil ||
			m.Kind == metadata.KindAtomic && m.Atomic == "null"
	}

	if isNull(&members[0]) {
		return &members[1], true
	}
	if isNull(&members[1]) {
		return &members[0], true
	}
	return nil, false
}
