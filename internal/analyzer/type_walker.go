// Package analyzer provides AST and type analysis utilities for tsgonest.
package analyzer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// maxWalkDepth is the maximum nesting depth for type walking.
// Prevents stack overflow from deeply recursive or infinitely expanding types
// (e.g., TypedOmit<Prisma.Entity, 'x'> creating unique anonymous types at each level).
const maxWalkDepth = 20

// maxTotalTypes is the maximum number of types that can be walked in a single
// TypeWalker session. Prevents excessive memory usage from wide type hierarchies
// (e.g., schema.org-style types with hundreds of interconnected types).
const maxTotalTypes = 500

// TypeWalker extracts Metadata from TypeScript types using the tsgo checker.
type TypeWalker struct {
	checker  *shimchecker.Checker
	registry *metadata.TypeRegistry
	// visited tracks types currently being analyzed to break infinite recursion.
	visiting map[shimchecker.TypeId]bool
	// typeIdToName maps TypeIds of anonymous types (from type aliases) to their
	// registered name. This allows walkObjectType to short-circuit to KindRef
	// for types previously walked via WalkNamedType.
	typeIdToName map[shimchecker.TypeId]string
	// depth tracks the current recursion depth for safety limits.
	depth int
	// totalTypesWalked tracks the total number of types processed to prevent
	// excessive memory usage from wide type hierarchies.
	totalTypesWalked int
	// exactOptionalPropertyTypes mirrors the tsconfig flag of the same name.
	// When true, optional properties cannot have explicit undefined values.
	exactOptionalPropertyTypes bool
}

// NewTypeWalker creates a new TypeWalker.
func NewTypeWalker(checker *shimchecker.Checker) *TypeWalker {
	return &TypeWalker{
		checker:      checker,
		registry:     metadata.NewTypeRegistry(),
		visiting:     make(map[shimchecker.TypeId]bool),
		typeIdToName: make(map[shimchecker.TypeId]string),
	}
}

// SetExactOptionalPropertyTypes configures the walker to mark optional properties
// with ExactOptional when the tsconfig flag is enabled.
func (w *TypeWalker) SetExactOptionalPropertyTypes(v bool) {
	w.exactOptionalPropertyTypes = v
}

// Registry returns the type registry with all discovered named types.
func (w *TypeWalker) Registry() *metadata.TypeRegistry {
	return w.registry
}

// TotalTypesWalked returns the total number of types walked so far.
func (w *TypeWalker) TotalTypesWalked() int {
	return w.totalTypesWalked
}

// WalkNamedType converts a tsgo Type into Metadata, using the given name for
// cycle detection and registry. Use this for type aliases, whose resolved types
// are anonymous but should still be tracked by name.
func (w *TypeWalker) WalkNamedType(name string, t *shimchecker.Type) metadata.Metadata {
	if t == nil {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	// If already registered, return a ref
	if w.registry.Has(name) {
		return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
	}

	// If currently visiting (recursive), return a ref
	if w.visiting[t.Id()] {
		return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
	}

	// Walk the type. If it resolves to an object, register it under the given name.
	w.visiting[t.Id()] = true
	m := w.WalkType(t)
	delete(w.visiting, t.Id())

	// If the result is unnamed, promote it to a named ref so that companion
	// files are generated for the alias name. This handles type aliases to
	// objects, unions, intersections, arrays, etc.
	if m.Name == "" && (m.Kind == metadata.KindObject || m.Kind == metadata.KindUnion || m.Kind == metadata.KindIntersection || m.Kind == metadata.KindArray) {
		m.Name = name
		w.registry.Register(name, &m)
		// Cache the TypeId → name so that subsequent WalkType calls on the same
		// anonymous type (e.g., from controller return type analysis) can
		// short-circuit to KindRef without re-walking.
		// BUT: don't cache phantom objects (branded type building blocks like
		// tags.Format<"email">) — they must remain inlinable so that
		// tryDetectBranded can detect `string & { __tsgonest_format: "email" }`.
		if !isPhantomObject(&m) {
			w.typeIdToName[t.Id()] = name
		}
		return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
	}

	return m
}

// WalkType converts a tsgo Type into a Metadata.
func (w *TypeWalker) WalkType(t *shimchecker.Type) metadata.Metadata {
	if t == nil {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	// Safety depth limit: prevents stack overflow from infinitely expanding types
	if w.depth >= maxWalkDepth {
		return metadata.Metadata{Kind: metadata.KindAny, Name: "depth-exceeded"}
	}
	// Reset breadth counter at the start of each top-level walk.
	// The limit should apply per type-graph traversal (one @Body, one @Param,
	// one return type, etc.), not across the entire program analysis.
	if w.depth == 0 {
		w.totalTypesWalked = 0
	}
	w.depth++
	defer func() { w.depth-- }()

	// Safety breadth limit: prevents excessive memory from wide type hierarchies
	w.totalTypesWalked++
	if w.totalTypesWalked > maxTotalTypes {
		return metadata.Metadata{Kind: metadata.KindAny, Name: "breadth-exceeded"}
	}

	flags := t.Flags()

	// Handle union and intersection first (they may contain null/undefined members)
	if flags&shimchecker.TypeFlagsUnion != 0 {
		return w.walkUnion(t)
	}
	if flags&shimchecker.TypeFlagsIntersection != 0 {
		return w.walkIntersection(t)
	}

	return w.walkSingleType(t)
}

// walkSingleType handles a non-union, non-intersection type.
func (w *TypeWalker) walkSingleType(t *shimchecker.Type) metadata.Metadata {
	flags := t.Flags()

	// Primitives and special types
	if flags&shimchecker.TypeFlagsAny != 0 {
		return metadata.Metadata{Kind: metadata.KindAny}
	}
	if flags&shimchecker.TypeFlagsUnknown != 0 {
		return metadata.Metadata{Kind: metadata.KindUnknown}
	}
	if flags&shimchecker.TypeFlagsNever != 0 {
		return metadata.Metadata{Kind: metadata.KindNever}
	}
	if flags&shimchecker.TypeFlagsVoid != 0 {
		return metadata.Metadata{Kind: metadata.KindVoid}
	}
	if flags&shimchecker.TypeFlagsNull != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "null"}
	}
	if flags&shimchecker.TypeFlagsUndefined != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "undefined"}
	}

	// Literal types
	if flags&shimchecker.TypeFlagsStringLiteral != 0 {
		lit := t.AsLiteralType()
		return metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: lit.Value()}
	}
	if flags&shimchecker.TypeFlagsNumberLiteral != 0 {
		lit := t.AsLiteralType()
		return metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: normalizeLiteralValue(lit.Value())}
	}
	if flags&shimchecker.TypeFlagsBooleanLiteral != 0 {
		// Boolean literals are LiteralType with bool value
		lit := t.AsLiteralType()
		if lit != nil {
			if boolVal, ok := lit.Value().(bool); ok {
				return metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: boolVal}
			}
		}
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}
	}
	if flags&shimchecker.TypeFlagsBigIntLiteral != 0 {
		lit := t.AsLiteralType()
		return metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: fmt.Sprintf("%v", lit.Value())}
	}

	// Atomic types
	if flags&shimchecker.TypeFlagsString != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}
	}
	if flags&shimchecker.TypeFlagsNumber != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}
	}
	if flags&shimchecker.TypeFlagsBoolean != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"}
	}
	if flags&shimchecker.TypeFlagsBigInt != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "bigint"}
	}
	if flags&shimchecker.TypeFlagsESSymbol != 0 {
		return metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "symbol"}
	}

	// Enum literal
	if flags&shimchecker.TypeFlagsEnumLiteral != 0 {
		lit := t.AsLiteralType()
		if lit != nil {
			return metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: lit.Value()}
		}
		return metadata.Metadata{Kind: metadata.KindEnum}
	}

	// Template literal type — extract regex pattern
	if flags&shimchecker.TypeFlagsTemplateLiteral != 0 {
		pattern := w.extractTemplateLiteralPattern(t)
		return metadata.Metadata{
			Kind:            metadata.KindAtomic,
			Atomic:          "string",
			Name:            "template",
			TemplatePattern: pattern,
		}
	}

	// Object type (includes interfaces, classes, arrays, tuples, functions)
	if flags&shimchecker.TypeFlagsObject != 0 {
		return w.walkObjectType(t)
	}

	// Unresolved types: try getBaseConstraintOfType as a fallback
	if flags&(shimchecker.TypeFlagsTypeParameter|shimchecker.TypeFlagsConditional|shimchecker.TypeFlagsIndexedAccess|shimchecker.TypeFlagsIndex) != 0 {
		constraint := shimchecker.Checker_getBaseConstraintOfType(w.checker, t)
		if constraint != nil && constraint != t {
			return w.WalkType(constraint)
		}
	}

	// Fallback
	return metadata.Metadata{Kind: metadata.KindAny, Name: "unsupported"}
}

// walkUnion handles union types (A | B | C).
// It separates null/undefined from the union and wraps the rest.
func (w *TypeWalker) walkUnion(t *shimchecker.Type) metadata.Metadata {
	types := t.Types()
	if len(types) == 0 {
		return metadata.Metadata{Kind: metadata.KindNever}
	}

	var members []metadata.Metadata
	nullable := false
	optional := false

	for _, member := range types {
		flags := member.Flags()
		if flags&shimchecker.TypeFlagsNull != 0 {
			nullable = true
			continue
		}
		if flags&shimchecker.TypeFlagsUndefined != 0 {
			optional = true
			continue
		}
		// Boolean is represented as union of true | false intrinsic types
		if flags&shimchecker.TypeFlagsBooleanLiteral != 0 {
			// Check if this is part of a boolean (true | false) union
			hasBoth := false
			for _, other := range types {
				if other != member && other.Flags()&shimchecker.TypeFlagsBooleanLiteral != 0 {
					hasBoth = true
					break
				}
			}
			if hasBoth {
				// Only add "boolean" once, not both true and false
				found := false
				for _, m := range members {
					if m.Kind == metadata.KindAtomic && m.Atomic == "boolean" {
						found = true
						break
					}
				}
				if !found {
					members = append(members, metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"})
				}
				continue
			}
		}
		members = append(members, w.WalkType(member))
	}

	// If only one real member, unwrap the union
	if len(members) == 1 {
		result := members[0]
		result.Nullable = nullable
		result.Optional = optional
		return result
	}
	if len(members) == 0 {
		// Union was only null/undefined
		m := metadata.Metadata{Kind: metadata.KindAny}
		m.Nullable = nullable
		m.Optional = optional
		return m
	}

	result := metadata.Metadata{
		Kind:         metadata.KindUnion,
		Nullable:     nullable,
		Optional:     optional,
		UnionMembers: members,
	}

	// For all-literal unions, try to extract a name from the type alias or enum symbol.
	// This enables enum deduplication: named enum unions are registered as $ref in OpenAPI
	// instead of being inlined everywhere they appear.
	allLit := true
	for _, m := range members {
		if m.Kind != metadata.KindLiteral {
			allLit = false
			break
		}
	}
	if allLit && len(members) > 1 {
		if name := w.getUnionEnumName(t); name != "" {
			result.Name = name
		}
	}

	// Try to detect a discriminant property for discriminated unions
	if disc := w.detectDiscriminant(members); disc != nil {
		result.Discriminant = disc
	}

	return result
}

// getUnionEnumName extracts a name for an all-literal union type.
// For type aliases like `type OrderStatus = "A" | "B" | "C"`, it returns "OrderStatus".
// For TS enums like `enum Status { Active = "active" }`, it returns "Status".
// Returns empty string if no name is found.
func (w *TypeWalker) getUnionEnumName(t *shimchecker.Type) string {
	// 1. Check for type alias (Prisma-style string union types)
	alias := shimchecker.Type_alias(t)
	if alias != nil {
		aliasSym := alias.Symbol()
		if aliasSym != nil && aliasSym.Name != "" {
			name := aliasSym.Name
			// Filter out internal names
			if name != "__type" && name != "__object" && (len(name) == 0 || name[0] != '\xfe') {
				return name
			}
		}
	}

	// 2. Check for enum symbol (actual TS enum declarations)
	sym := t.Symbol()
	if sym != nil && sym.Name != "" {
		name := sym.Name
		if name != "__type" && name != "__object" && (len(name) == 0 || name[0] != '\xfe') {
			return name
		}
	}

	return ""
}

// detectDiscriminant checks if a union of objects has a common property with
// unique literal values in each member (a discriminated union pattern).
// Returns nil if no discriminant is found.
func (w *TypeWalker) detectDiscriminant(members []metadata.Metadata) *metadata.Discriminant {
	if len(members) < 2 {
		return nil
	}

	// Resolve all members to their properties — all must be objects
	type memberProps struct {
		props []metadata.Property
	}
	var resolved []memberProps
	for _, m := range members {
		p := w.resolveToObjectProperties(&m)
		if p == nil {
			return nil // Not all members are objects
		}
		resolved = append(resolved, memberProps{props: p})
	}

	// For each property that exists in ALL members, check if each member
	// has a unique literal value for that property
	// Get property names from first member
	for _, prop := range resolved[0].props {
		candidateName := prop.Name
		mapping := make(map[string]int)

		valid := true
		for i, mp := range resolved {
			found := false
			for _, p := range mp.props {
				if p.Name == candidateName {
					found = true
					// Check if the property type is a literal
					litVal := extractLiteralValue(&p.Type)
					if litVal == "" {
						valid = false
						break
					}
					if _, exists := mapping[litVal]; exists {
						// Duplicate value — not a discriminant
						valid = false
						break
					}
					mapping[litVal] = i
					break
				}
			}
			if !found || !valid {
				valid = false
				break
			}
		}

		if valid && len(mapping) == len(members) {
			return &metadata.Discriminant{
				Property: candidateName,
				Mapping:  mapping,
			}
		}
	}

	return nil
}

// extractLiteralValue returns the string representation of a literal type's value,
// or empty string if not a literal.
func extractLiteralValue(m *metadata.Metadata) string {
	if m.Kind == metadata.KindLiteral {
		return fmt.Sprintf("%v", m.LiteralValue)
	}
	return ""
}

// walkIntersection handles intersection types (A & B).
// When all members resolve to objects, it flattens them into a single merged object.
// For mixed intersections (e.g., string & { __brand: 'Email' }), it keeps the intersection.
func (w *TypeWalker) walkIntersection(t *shimchecker.Type) metadata.Metadata {
	types := t.Types()
	if len(types) == 0 {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	var members []metadata.Metadata
	for _, member := range types {
		members = append(members, w.WalkType(member))
	}

	if len(members) == 1 {
		return members[0]
	}

	// Try to detect branded types: atomic & { __brand: ... }
	// One member is an atomic type, the others are phantom objects with only
	// brand-like properties (__brand, __meta, __phantom, __type, etc.)
	// Pass raw types alongside walked members for function type resolution (Validate<typeof fn>).
	if branded := w.tryDetectBranded(types, members); branded != nil {
		return *branded
	}

	// Try to flatten: if all members resolve to objects, merge properties.
	return w.tryFlattenIntersection(members)
}

// tryFlattenIntersection checks if all intersection members resolve to objects
// and merges them into a single KindObject. Returns KindIntersection if any
// member is not an object.
func (w *TypeWalker) tryFlattenIntersection(members []metadata.Metadata) metadata.Metadata {
	var allProps []metadata.Property
	for _, m := range members {
		props := w.resolveToObjectProperties(&m)
		if props == nil {
			// Not all members are objects — keep as intersection
			return metadata.Metadata{
				Kind:                metadata.KindIntersection,
				IntersectionMembers: members,
			}
		}
		allProps = append(allProps, props...)
	}

	// Merge properties: later properties win on name conflict (matching typia behavior)
	merged := mergeProperties(allProps)

	return metadata.Metadata{
		Kind:       metadata.KindObject,
		Properties: merged,
	}
}

// resolveToObjectProperties returns the properties of a metadata if it's an object
// (or a ref that resolves to an object). Returns nil if not an object.
func (w *TypeWalker) resolveToObjectProperties(m *metadata.Metadata) []metadata.Property {
	switch m.Kind {
	case metadata.KindObject:
		return m.Properties
	case metadata.KindRef:
		if resolved, ok := w.registry.Types[m.Ref]; ok {
			if resolved.Kind == metadata.KindObject {
				return resolved.Properties
			}
		}
		return nil
	default:
		return nil
	}
}

// mergeProperties merges properties from multiple objects, with later entries
// winning on name conflict.
func mergeProperties(allProps []metadata.Property) []metadata.Property {
	seen := make(map[string]int) // name → index in result
	var result []metadata.Property

	for _, prop := range allProps {
		if idx, ok := seen[prop.Name]; ok {
			// Later property wins (overwrite)
			result[idx] = prop
		} else {
			seen[prop.Name] = len(result)
			result = append(result, prop)
		}
	}

	return result
}

// walkObjectType handles object types (interfaces, arrays, tuples, native types).
func (w *TypeWalker) walkObjectType(t *shimchecker.Type) metadata.Metadata {
	// Check for array first
	if shimchecker.Checker_isArrayType(w.checker, t) {
		typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
		if len(typeArgs) > 0 {
			elem := w.WalkType(typeArgs[0])
			return metadata.Metadata{Kind: metadata.KindArray, ElementType: &elem}
		}
		any := metadata.Metadata{Kind: metadata.KindAny}
		return metadata.Metadata{Kind: metadata.KindArray, ElementType: &any}
	}

	// Check for tuple
	if shimchecker.IsTupleType(t) {
		return w.walkTupleType(t)
	}

	// Check for native/built-in types (Date, RegExp, Map, Set, Promise, etc.)
	sym := t.Symbol()
	if sym != nil {
		name := sym.Name
		switch name {
		case "Date":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: "Date"}
		case "RegExp":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: "RegExp"}
		case "Map":
			return w.walkGenericNative(t, "Map")
		case "Set":
			return w.walkGenericNative(t, "Set")
		case "Promise":
			// Unwrap Promise<T> to T
			typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
			if len(typeArgs) > 0 {
				return w.WalkType(typeArgs[0])
			}
			return metadata.Metadata{Kind: metadata.KindAny}
		case "Observable":
			// Unwrap Observable<T> to T (rxjs Observable, used for SSE endpoints)
			// Combined with Promise unwrapping, this handles Promise<Observable<T>> → T
			typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
			if len(typeArgs) > 0 {
				return w.WalkType(typeArgs[0])
			}
			return metadata.Metadata{Kind: metadata.KindAny}
		case "Uint8Array", "Int8Array", "Uint16Array", "Int16Array",
			"Uint32Array", "Int32Array", "Float32Array", "Float64Array",
			"BigInt64Array", "BigUint64Array":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: name}
		case "ArrayBuffer", "SharedArrayBuffer":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: name}
		case "URL", "URLSearchParams":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: name}
		case "Error":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: "Error"}
		}
	}

	// Check if this is a function type (has call signatures, no properties of interest)
	callSigs := shimchecker.Checker_getSignaturesOfType(w.checker, t, shimchecker.SignatureKindCall)
	props := shimchecker.Checker_getPropertiesOfType(w.checker, t)
	if len(callSigs) > 0 && len(props) == 0 {
		return metadata.Metadata{Kind: metadata.KindAny, Name: "function"}
	}

	// Named object type — check for recursion
	typeName := w.getTypeName(t)
	if typeName != "" {
		if w.visiting[t.Id()] {
			// Recursive type — return a $ref
			return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
		}

		if w.registry.Has(typeName) {
			// Already analyzed — return a $ref
			return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
		}

		// Mark as visiting, analyze, register
		w.visiting[t.Id()] = true
		result := w.analyzeObjectProperties(t, typeName)
		delete(w.visiting, t.Id())
		w.registry.Register(typeName, &result)
		return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
	}

	// Anonymous object type — check if we've seen this TypeId before
	// (type aliases produce anonymous types that were registered by name via WalkNamedType)
	if cachedName, ok := w.typeIdToName[t.Id()]; ok {
		return metadata.Metadata{Kind: metadata.KindRef, Ref: cachedName}
	}

	// Anonymous object type — inline the properties
	return w.analyzeObjectProperties(t, "")
}

// analyzeObjectProperties extracts properties from an object type.
func (w *TypeWalker) analyzeObjectProperties(t *shimchecker.Type, name string) metadata.Metadata {
	props := shimchecker.Checker_getPropertiesOfType(w.checker, t)
	var properties []metadata.Property

	for _, prop := range props {
		propType := shimchecker.Checker_getTypeOfSymbol(w.checker, prop)
		propMeta := w.WalkType(propType)

		isOptional := prop.Flags&ast.SymbolFlagsOptional != 0
		if isOptional {
			propMeta.Optional = true
		}

		isReadonly := shimchecker.Checker_isReadonlySymbol(w.checker, prop)

		// Extract constraints from two sources:
		// 1. Branded phantom types (e.g., string & tags.Format<"email">)
		// 2. JSDoc tags (e.g., @format email)
		// JSDoc takes precedence over branded types for the same constraint.
		var constraints *metadata.Constraints

		// Start with branded type constraints (if any)
		if propMeta.Constraints != nil {
			c := *propMeta.Constraints // copy
			constraints = &c
			propMeta.Constraints = nil // don't leak to codegen
		}

		// Merge JSDoc constraints (takes precedence)
		if prop.ValueDeclaration != nil {
			jsdocConstraints := w.extractJSDocConstraints(prop.ValueDeclaration)
			if jsdocConstraints != nil {
				if constraints == nil {
					constraints = jsdocConstraints
				} else {
					mergeConstraints(constraints, jsdocConstraints)
				}
			}
		}

		properties = append(properties, metadata.Property{
			Name:          prop.Name,
			Type:          propMeta,
			Required:      !isOptional,
			Readonly:      isReadonly,
			ExactOptional: w.exactOptionalPropertyTypes && isOptional,
			Constraints:   constraints,
		})
	}

	result := metadata.Metadata{
		Kind:       metadata.KindObject,
		Name:       name,
		Properties: properties,
	}

	// Extract type-level annotations (@strict, @tsgonest-ignore, etc.)
	typeSym := t.Symbol()
	if typeSym != nil && typeSym.ValueDeclaration != nil {
		strictness, ignore := w.extractTypeLevelAnnotations(typeSym.ValueDeclaration)
		if strictness != "" {
			result.Strictness = strictness
		}
		if ignore != "" {
			result.Ignore = ignore
		}
	}

	// Check for index signatures
	indexInfos := shimchecker.Checker_getIndexInfosOfType(w.checker, t)
	if len(indexInfos) > 0 {
		info := indexInfos[0]
		keyMeta := w.WalkType(shimchecker.IndexInfo_keyType(info))
		valMeta := w.WalkType(shimchecker.IndexInfo_valueType(info))
		result.IndexSignature = &metadata.IndexSignature{
			KeyType:   keyMeta,
			ValueType: valMeta,
		}
	}

	return result
}

// walkTupleType handles tuple types like [string, number].
func (w *TypeWalker) walkTupleType(t *shimchecker.Type) metadata.Metadata {
	typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
	tupleType := t.TargetTupleType()

	var elements []metadata.TupleElement
	var elementInfos []shimchecker.TupleElementInfo
	if tupleType != nil {
		elementInfos = shimchecker.TupleType_elementInfos(tupleType)
	}

	for i, arg := range typeArgs {
		elem := metadata.TupleElement{
			Type: w.WalkType(arg),
		}

		// Check element flags from tuple info
		if elementInfos != nil && i < len(elementInfos) {
			info := elementInfos[i]
			elem.Optional = info.TupleElementFlags()&shimchecker.ElementFlagsOptional != 0
			elem.Rest = info.TupleElementFlags()&shimchecker.ElementFlagsRest != 0
			// Named tuple labels — LabeledDeclaration() returns a node if present
			// We skip label extraction for now; could parse declaration name later
		}

		elements = append(elements, elem)
	}

	return metadata.Metadata{
		Kind:     metadata.KindTuple,
		Elements: elements,
	}
}

// walkGenericNative handles generic native types like Map<K,V> and Set<T>.
func (w *TypeWalker) walkGenericNative(t *shimchecker.Type, name string) metadata.Metadata {
	typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
	var args []metadata.Metadata
	for _, arg := range typeArgs {
		args = append(args, w.WalkType(arg))
	}
	return metadata.Metadata{
		Kind:          metadata.KindNative,
		NativeType:    name,
		TypeArguments: args,
	}
}

// normalizeLiteralValue converts tsgo literal values (e.g., jsnum.Number) to
// standard Go types for consistent handling in metadata.
func normalizeLiteralValue(v any) any {
	// jsnum.Number is defined as `type Number float64` — use type assertion via float64
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		return val
	case bool:
		return val
	default:
		// For jsnum.Number and other types that implement a numeric interface,
		// try to convert via fmt to float64 string then parse
		str := fmt.Sprintf("%v", v)
		var f float64
		if _, err := fmt.Sscanf(str, "%g", &f); err == nil {
			return f
		}
		return v
	}
}

// getTypeName returns the name of a type if it has one, empty string otherwise.
// Only returns a name for types that should be registered (interfaces, classes, named references),
// not for anonymous object literals or type alias targets.
func (w *TypeWalker) getTypeName(t *shimchecker.Type) string {
	// Anonymous objects (inline { x: number }) should not be named
	objFlags := shimchecker.Type_objectFlags(t)
	if objFlags&shimchecker.ObjectFlagsAnonymous != 0 {
		return ""
	}

	sym := t.Symbol()
	if sym == nil {
		return ""
	}
	name := sym.Name

	// Filter out anonymous/structural types
	if name == "" || name == "__type" || name == "__object" || name == "__function" {
		return ""
	}
	// Filter out TypeScript internal anonymous type names (e.g., "\xfetype" from Omit/Pick/etc.)
	if len(name) > 0 && name[0] == '\xfe' {
		return ""
	}

	return name
}

// WalkTypeNode extracts Metadata from an AST type node.
// If the node is a named type reference (e.g., `LoginRequest`), the Name field
// of the result will be set to the type name.
func (w *TypeWalker) WalkTypeNode(node *ast.Node) metadata.Metadata {
	t := shimchecker.Checker_getTypeFromTypeNode(w.checker, node)
	result := w.WalkType(t)

	// Preserve the type name for named type references.
	// Skip wrapper types (Promise, Observable) — their inner type is already unwrapped.
	if result.Name == "" && node.Kind == ast.KindTypeReference {
		ref := node.AsTypeReferenceNode()
		if ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier {
			name := ref.TypeName.Text()
			if name != "Promise" && name != "Observable" && name != "Array" {
				result.Name = name
			}
		}
	}

	return result
}

// extractTemplateLiteralPattern converts a template literal type to a regex pattern.
// e.g., `prefix_${string}` → "^prefix_.*$"
// e.g., `${string}@${string}.${string}` → "^.*@.*\\..*$"
func (w *TypeWalker) extractTemplateLiteralPattern(t *shimchecker.Type) string {
	tlt := t.AsTemplateLiteralType()
	if tlt == nil {
		return ""
	}

	texts := tlt.Texts()
	types := tlt.Types()

	if len(texts) == 0 {
		return ""
	}

	var pattern strings.Builder
	pattern.WriteString("^")

	for i, text := range texts {
		// Escape regex special characters in the literal text
		pattern.WriteString(regexp.QuoteMeta(text))

		// After each text (except the last), add a pattern for the type slot
		if i < len(types) {
			slotType := types[i]
			flags := slotType.Flags()
			if flags&shimchecker.TypeFlagsNumber != 0 {
				pattern.WriteString("[+-]?(\\d+\\.?\\d*|\\.\\d+)")
			} else {
				// string, any, or other — match anything
				pattern.WriteString(".*")
			}
		}
	}

	pattern.WriteString("$")
	return pattern.String()
}
