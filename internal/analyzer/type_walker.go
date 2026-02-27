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
	// pendingName maps TypeIds to alias names during WalkNamedType walks.
	// Used by walkIntersection and walkUnion to detect self-referential types
	// (e.g., type Message = Entity & { replies: Message[] }) and return a $ref
	// instead of expanding infinitely.
	pendingName map[shimchecker.TypeId]string
	// depth tracks the current recursion depth for safety limits.
	depth int
	// totalTypesWalked tracks the total number of types processed to prevent
	// excessive memory usage from wide type hierarchies.
	totalTypesWalked int
	// exactOptionalPropertyTypes mirrors the tsconfig flag of the same name.
	// When true, optional properties cannot have explicit undefined values.
	exactOptionalPropertyTypes bool
	// warnings collects actionable diagnostics emitted during type walking
	// (e.g., generic types with anonymous type arguments that can't be named).
	warnings []string
	// warnedGenericNames tracks generic type base names that have already emitted
	// a warning, to avoid flooding the output with duplicate messages.
	warnedGenericNames map[string]bool
}

// NewTypeWalker creates a new TypeWalker.
func NewTypeWalker(checker *shimchecker.Checker) *TypeWalker {
	return &TypeWalker{
		checker:            checker,
		registry:           metadata.NewTypeRegistry(),
		visiting:           make(map[shimchecker.TypeId]bool),
		typeIdToName:       make(map[shimchecker.TypeId]string),
		pendingName:        make(map[shimchecker.TypeId]string),
		warnedGenericNames: make(map[string]bool),
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

// Warnings returns actionable diagnostics collected during type walking.
func (w *TypeWalker) Warnings() []string {
	return w.warnings
}

// warnAnonymousTypeArgs emits a deduplicated warning about a generic type
// whose type arguments cannot be named for OpenAPI schema registration.
func (w *TypeWalker) warnAnonymousTypeArgs(baseName string) {
	if w.warnedGenericNames[baseName] {
		return
	}
	w.warnedGenericNames[baseName] = true
	w.warnings = append(w.warnings, fmt.Sprintf(
		"generic type %s has anonymous type arguments that cannot be named in OpenAPI — consider creating a named type alias (e.g., type My%s = %s<...>)",
		baseName, baseName, baseName,
	))
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

	// If currently visiting (recursive), return a ref.
	// This catches cross-alias recursion where the same underlying type is being
	// walked by another WalkNamedType call, or by walkObjectType/walkIntersection/walkUnion.
	if w.visiting[t.Id()] {
		return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
	}

	// Register a pending name so that self-referential intersection/union types
	// (e.g., type Message = Entity & { replies: Message[] }) can resolve back
	// to a $ref during walkIntersection/walkUnion recursion. This is separate
	// from typeIdToName to avoid short-circuiting the initial walkObjectType call.
	w.pendingName[t.Id()] = name

	// Walk the type. Don't set visiting here — let walkObjectType,
	// walkIntersection, and walkUnion manage their own recursion guards.
	// Setting visiting here would cause walkIntersection to short-circuit
	// on the first call, preventing the intersection from being analyzed.
	m := w.WalkType(t)
	delete(w.pendingName, t.Id())

	// If the result is unnamed, promote it to a named ref so that companion
	// files are generated for the alias name. This handles type aliases to
	// objects, unions, intersections, etc.
	// KindArray is intentionally excluded: named array type aliases (e.g.,
	// type ShipmentItemSnapshot = {...}[]) should NOT become named $ref schemas
	// in OpenAPI. Registering them causes double-nesting: when a property uses
	// SomeArrayType[], the client generator sees an array-of-arrays.
	if m.Name == "" && (m.Kind == metadata.KindObject || m.Kind == metadata.KindUnion || m.Kind == metadata.KindIntersection) {
		// Don't register phantom objects (branded type building blocks like
		// tags.Format<"email"> or tags.Email). They must remain inlinable
		// so that tryDetectBranded can detect `string & { __tsgonest_format: "email" }`.
		// Registering them would cause sub-field branded types to see KindRef
		// instead of the inline phantom properties.
		if isPhantomObject(&m) {
			return m
		}
		m.Name = name
		w.registry.Register(name, &m)
		w.typeIdToName[t.Id()] = name
		return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
	}

	// For ref types already registered under a mechanical name (e.g., generic
	// instantiations like PaginatedResponse_ThreadMessageResponse), propagate
	// the alias name so downstream consumers (OpenAPI, SDK) can use the
	// user-defined name (e.g., GetAllThreadsResponse).
	if m.Kind == metadata.KindRef && m.Name == "" {
		m.Name = name
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

// resolveAliasName attempts to derive a registration name from a type's alias symbol.
// This is used by walkIntersection/walkUnion recursion guards as a fallback when
// no pendingName was set by WalkNamedType (i.e., the self-referential type was
// encountered as a sub-field, not walked directly at the top level).
// Returns "" if no name can be derived.
func (w *TypeWalker) resolveAliasName(t *shimchecker.Type) string {
	alias := shimchecker.Type_alias(t)
	if alias == nil {
		return ""
	}
	aliasSym := alias.Symbol()
	if aliasSym == nil {
		return ""
	}
	name := aliasSym.Name
	if name == "" || name == "__type" || name == "__object" || (len(name) > 0 && name[0] == '\xfe') {
		return ""
	}
	aliasTypeArgs := alias.TypeArguments()
	if len(aliasTypeArgs) > 0 {
		if compositeName, ok := w.buildGenericInstantiationName(name, aliasTypeArgs); ok {
			return compositeName
		}
		return "" // anonymous type args — can't resolve
	}
	return name
}

// walkUnion handles union types (A | B | C).
// It separates null/undefined from the union and wraps the rest.
func (w *TypeWalker) walkUnion(t *shimchecker.Type) metadata.Metadata {
	// Recursion guard: detect self-referential union types.
	// Same rationale as walkIntersection — union types are dispatched before
	// reaching walkObjectType, so they need their own guard.
	if w.visiting[t.Id()] {
		if cachedName, ok := w.pendingName[t.Id()]; ok {
			return metadata.Metadata{Kind: metadata.KindRef, Ref: cachedName}
		}
		// Fallback: derive name from type alias for sub-field self-referential types
		// (e.g., type JsonValue = string | JsonObject | JsonArray where JsonObject
		// references JsonValue). Without this, recursive sub-fields degrade to KindAny.
		if name := w.resolveAliasName(t); name != "" {
			return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
		}
		return metadata.Metadata{Kind: metadata.KindAny}
	}
	w.visiting[t.Id()] = true
	defer delete(w.visiting, t.Id())

	types := t.Types()
	if len(types) == 0 {
		return metadata.Metadata{Kind: metadata.KindNever}
	}

	var members []metadata.Metadata
	var brandedConstraints *metadata.Constraints // constraints from branded literal intersections
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
		// Handle string and number literals directly without calling WalkType.
		// Large unions (e.g., 180+ currency codes) would otherwise burn through
		// the maxTotalTypes breadth limit, causing subsequent properties to degrade
		// to KindAny. This matches the boolean literal optimization above.
		if flags&shimchecker.TypeFlagsStringLiteral != 0 {
			lit := member.AsLiteralType()
			members = append(members, metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: lit.Value()})
			continue
		}
		if flags&shimchecker.TypeFlagsNumberLiteral != 0 {
			lit := member.AsLiteralType()
			members = append(members, metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: normalizeLiteralValue(lit.Value())})
			continue
		}
		// Handle branded literal intersections without walking each member.
		// When a literal union has branded constraints (e.g., TCurrencyCode4217 & tags.MaxLength<3>),
		// TS distributes the intersection: ("USD" & phantom) | ("EUR" & phantom) | ...
		// Each intersection member has TypeFlagsIntersection, not TypeFlagsStringLiteral.
		// Walking all 163+ intersections would burn ~500 types from the breadth limit.
		// Instead, detect the pattern on the first intersection and extract just the
		// literal value for subsequent ones (constraints are identical across all members).
		if flags&shimchecker.TypeFlagsIntersection != 0 {
			if brandedConstraints != nil {
				// We already confirmed this union has branded literal intersections.
				// Just extract the literal value without walking phantoms again.
				if litVal, ok := extractBrandedLiteralValue(member); ok {
					members = append(members, metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: litVal})
					continue
				}
			}
			if litMeta, ok := w.tryFastBrandedLiteral(member); ok {
				// First branded literal intersection — capture constraints.
				if litMeta.Constraints != nil {
					c := *litMeta.Constraints
					brandedConstraints = &c
				}
				litMeta.Constraints = nil // Constraints will be applied at the union level
				members = append(members, litMeta)
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
		// Preserve branded constraints from the fast-path (e.g., nullable branded
		// literal like ('USD' | null) & MaxLength<3> which unwraps to a single member).
		if brandedConstraints != nil && result.Constraints == nil && result.Kind == metadata.KindLiteral {
			result.Constraints = brandedConstraints
		}
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
		// Only apply branded constraints when all members are literals.
		// This guards against the theoretical case where a union has a mix of
		// branded literal intersections and non-branded members.
		if brandedConstraints != nil {
			result.Constraints = brandedConstraints
		}
	}

	// Try to detect a discriminant property for discriminated unions
	if disc := w.detectDiscriminant(members); disc != nil {
		result.Discriminant = disc
	}

	// If the union has a type alias name (depth > 1), register it as a named type
	// so it becomes a $ref instead of being inlined. This catches named union aliases
	// used as sub-fields (e.g., `status: OrderStatus` where OrderStatus = 'a' | 'b').
	// For generic instantiations, build a composite name to avoid collisions.
	if w.depth > 1 {
		alias := shimchecker.Type_alias(t)
		if alias != nil {
			if aliasSym := alias.Symbol(); aliasSym != nil {
				aliasName := aliasSym.Name
				if aliasName != "" && aliasName != "__type" && aliasName != "__object" && (len(aliasName) == 0 || aliasName[0] != '\xfe') {
					registrationName := aliasName
					aliasTypeArgs := alias.TypeArguments()
					if len(aliasTypeArgs) > 0 {
						if compositeName, ok := w.buildGenericInstantiationName(aliasName, aliasTypeArgs); ok {
							registrationName = compositeName
						} else {
							// Anonymous type args — skip registration, inline, and warn
							w.warnAnonymousTypeArgs(aliasName)
							return result
						}
					}
					if w.registry.Has(registrationName) {
						return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName, Nullable: nullable, Optional: optional}
					}
					result.Name = registrationName
					w.registry.Register(registrationName, &result)
					w.typeIdToName[t.Id()] = registrationName
					return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName, Nullable: nullable, Optional: optional}
				}
			}
		}
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

// tryFastBrandedLiteral checks if an intersection type is a literal + phantom objects
// pattern (e.g., "USD" & { __tsgonest_maxLength: 3 } & { __tsgonest_pattern: "^[A-Z]{3}$" }).
// This is used as a fast-path optimization in walkUnion to avoid walking each member
// of a large branded literal union (e.g., TCurrencyCode4217 & tags.MaxLength<3>) through
// the full WalkType machinery, which would exhaust the breadth limit.
// Returns the branded literal metadata if detected, or (Metadata{}, false) if not.
//
// Unlike tryDetectBranded, this operates directly on raw types using the checker API
// to avoid incrementing the breadth counter for each of the 163+ union members.
func (w *TypeWalker) tryFastBrandedLiteral(t *shimchecker.Type) (metadata.Metadata, bool) {
	types := t.Types()
	if len(types) == 0 {
		return metadata.Metadata{}, false
	}

	var litMeta *metadata.Metadata
	var phantomMembers []*metadata.Metadata
	var rawPhantomTypes []*shimchecker.Type

	for _, member := range types {
		memberFlags := member.Flags()
		if memberFlags&shimchecker.TypeFlagsStringLiteral != 0 {
			if litMeta != nil {
				return metadata.Metadata{}, false // multiple literals
			}
			lit := member.AsLiteralType()
			m := metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: lit.Value()}
			litMeta = &m
		} else if memberFlags&shimchecker.TypeFlagsNumberLiteral != 0 {
			if litMeta != nil {
				return metadata.Metadata{}, false // multiple literals
			}
			lit := member.AsLiteralType()
			m := metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: normalizeLiteralValue(lit.Value())}
			litMeta = &m
		} else if memberFlags&shimchecker.TypeFlagsObject != 0 {
			// Check if this object is a phantom using the checker API directly,
			// avoiding WalkType and the breadth counter.
			props := shimchecker.Checker_getPropertiesOfType(w.checker, member)
			if len(props) == 0 {
				return metadata.Metadata{}, false
			}
			allPhantom := true
			var propMetadata []metadata.Property
			for _, prop := range props {
				if !isPhantomPropertyName(prop.Name) {
					allPhantom = false
					break
				}
				// Walk just the property type for constraint extraction
				propType := shimchecker.Checker_getTypeOfSymbol(w.checker, prop)
				propMeta := w.WalkType(propType)
				propMetadata = append(propMetadata, metadata.Property{
					Name: prop.Name,
					Type: propMeta,
				})
			}
			if !allPhantom {
				return metadata.Metadata{}, false // non-phantom object member
			}
			walked := metadata.Metadata{Kind: metadata.KindObject, Properties: propMetadata}
			phantomMembers = append(phantomMembers, &walked)
			rawPhantomTypes = append(rawPhantomTypes, member)
		} else {
			return metadata.Metadata{}, false // non-literal, non-object member
		}
	}

	if litMeta != nil && len(phantomMembers) > 0 {
		result := *litMeta
		constraints := w.extractBrandedConstraints(rawPhantomTypes, phantomMembers)
		if constraints != nil {
			result.Constraints = constraints
		}
		return result, true
	}

	return metadata.Metadata{}, false
}

// extractBrandedLiteralValue extracts the literal value from an intersection type
// that is known to be a branded literal pattern (literal & phantom...).
// This is the ultra-fast path: no WalkType calls, no breadth counter increments.
// Returns the literal value and true if found, or ("", false) if the pattern doesn't match.
func extractBrandedLiteralValue(t *shimchecker.Type) (interface{}, bool) {
	types := t.Types()
	for _, member := range types {
		memberFlags := member.Flags()
		if memberFlags&shimchecker.TypeFlagsStringLiteral != 0 {
			lit := member.AsLiteralType()
			return lit.Value(), true
		}
		if memberFlags&shimchecker.TypeFlagsNumberLiteral != 0 {
			lit := member.AsLiteralType()
			return normalizeLiteralValue(lit.Value()), true
		}
	}
	return nil, false
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
	// Recursion guard: detect self-referential intersection types.
	// walkObjectType has its own guard for named objects, but intersection types
	// are dispatched before reaching walkObjectType. Without this guard,
	// type Message = Entity & { replies: Message[] } would expand infinitely
	// until hitting the depth limit, degrading all properties to empty schemas.
	if w.visiting[t.Id()] {
		if cachedName, ok := w.pendingName[t.Id()]; ok {
			return metadata.Metadata{Kind: metadata.KindRef, Ref: cachedName}
		}
		// Fallback: derive name from type alias for sub-field self-referential types
		// (e.g., type Thread = Entity & { replies?: Thread[] } used as a property
		// of another type). Without this, recursive sub-fields degrade to KindAny.
		if name := w.resolveAliasName(t); name != "" {
			return metadata.Metadata{Kind: metadata.KindRef, Ref: name}
		}
		return metadata.Metadata{Kind: metadata.KindAny}
	}
	w.visiting[t.Id()] = true
	defer delete(w.visiting, t.Id())

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
	result := w.tryFlattenIntersection(members)

	// If the intersection has a type alias name (e.g., type ShippingAddress = Address & { ... }),
	// register the flattened result so it becomes a $ref instead of being inlined.
	// Only for sub-field types (depth > 1). For generic aliases, build composite names.
	if w.depth > 1 && result.Kind == metadata.KindObject {
		alias := shimchecker.Type_alias(t)
		if alias != nil && w.pendingName[t.Id()] == "" {
			if aliasSym := alias.Symbol(); aliasSym != nil {
				aliasName := aliasSym.Name
				if aliasName != "" && aliasName != "__type" && aliasName != "__object" && (len(aliasName) == 0 || aliasName[0] != '\xfe') {
					registrationName := aliasName
					aliasTypeArgs := alias.TypeArguments()
					if len(aliasTypeArgs) > 0 {
						if compositeName, ok := w.buildGenericInstantiationName(aliasName, aliasTypeArgs); ok {
							registrationName = compositeName
						} else {
							// Anonymous type args — skip registration, inline, and warn
							w.warnAnonymousTypeArgs(aliasName)
							return result
						}
					}
					if w.registry.Has(registrationName) {
						return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName}
					}
					if !isPhantomObject(&result) {
						result.Name = registrationName
						w.registry.Register(registrationName, &result)
						w.typeIdToName[t.Id()] = registrationName
						return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName}
					}
				}
			}
		}
	}

	return result
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

	// Check for interfaces extending Array<T> (e.g., Prisma's JsonArray).
	// isArrayType returns false for named interfaces that extend Array, but their
	// base types include the Array<T> instantiation. Detect this and treat as array.
	// Only check class/interface types — getBaseTypes panics on anonymous object types.
	objFlags := shimchecker.Type_objectFlags(t)
	if objFlags&shimchecker.ObjectFlagsClassOrInterface != 0 {
		baseTypes := shimchecker.Checker_getBaseTypes(w.checker, t)
		for _, base := range baseTypes {
			if shimchecker.Checker_isArrayType(w.checker, base) {
				typeArgs := shimchecker.Checker_getTypeArguments(w.checker, base)
				if len(typeArgs) > 0 {
					elem := w.WalkType(typeArgs[0])
					return metadata.Metadata{Kind: metadata.KindArray, ElementType: &elem}
				}
				any := metadata.Metadata{Kind: metadata.KindAny}
				return metadata.Metadata{Kind: metadata.KindArray, ElementType: &any}
			}
		}
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
		case "AsyncGenerator":
			// Unwrap AsyncGenerator<Y, R, N> → Y (yield type, first type arg).
			// Used by @EventStream() which returns AsyncGenerator<SseEvent<...>>.
			typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
			if len(typeArgs) > 0 {
				return w.WalkType(typeArgs[0])
			}
			return metadata.Metadata{Kind: metadata.KindAny}
		case "AsyncIterable", "AsyncIterableIterator":
			// Unwrap AsyncIterable<T> / AsyncIterableIterator<T> → T.
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
		case "File", "Blob":
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
		// Check if this is a generic type instantiation (e.g., PaginatedResponse<UserDto>).
		// Different instantiations share the same symbol name but have different type arguments.
		// Generate a unique composite name per instantiation to prevent the first one from
		// "winning" and all others incorrectly referencing it.
		// Only check type arguments on Reference types (generic instantiations have ObjectFlagsReference).
		objFlags := shimchecker.Type_objectFlags(t)
		if objFlags&shimchecker.ObjectFlagsReference != 0 {
			typeArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
			if len(typeArgs) > 0 {
				if compositeName, ok := w.buildGenericInstantiationName(typeName, typeArgs); ok {
					typeName = compositeName
				} else {
					// Type arguments are anonymous/unnameable — inline the type and warn
					w.warnAnonymousTypeArgs(typeName)
					return w.analyzeObjectProperties(t, "")
				}
			}
		}

		if w.visiting[t.Id()] {
			// Recursive type — return a $ref
			return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
		}

		if w.registry.Has(typeName) {
			// Already analyzed — return a $ref
			return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
		}

		// Mark as visiting, analyze, register.
		// Save and reset the breadth counter so this named type gets its own
		// budget. Without this, a large union (e.g., 163 currency codes) in a
		// sibling property would exhaust the counter before we even start walking
		// this type's properties, causing them all to degrade to KindAny.
		savedBreadth := w.totalTypesWalked
		w.totalTypesWalked = 0
		w.visiting[t.Id()] = true
		result := w.analyzeObjectProperties(t, typeName)
		delete(w.visiting, t.Id())
		w.totalTypesWalked = savedBreadth // restore parent's counter
		w.registry.Register(typeName, &result)
		return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
	}

	// Anonymous object type — check if we've seen this TypeId before
	// (type aliases produce anonymous types that were registered by name via WalkNamedType)
	if cachedName, ok := w.typeIdToName[t.Id()]; ok {
		return metadata.Metadata{Kind: metadata.KindRef, Ref: cachedName}
	}

	// Type alias encountered as sub-field — derive name from alias symbol.
	// Type aliases resolve to anonymous types (ObjectFlagsAnonymous), so
	// getTypeName returns "". But Type_alias preserves the alias declaration,
	// letting us recover the original name (e.g., CustomerShippingAddressResponse)
	// and register it for $ref extraction in OpenAPI.
	// Only use Type_alias for sub-field types (depth > 1), not the top-level
	// type being walked. WalkNamedType handles registration for top-level types.
	// Also skip when being walked by WalkNamedType (has a pendingName entry).
	// Skip phantom objects (branded type building blocks like tags.Format<"email">)
	// — they must remain inlinable so tryDetectBranded can detect them.
	if w.depth > 1 {
		alias := shimchecker.Type_alias(t)
		if alias != nil && w.pendingName[t.Id()] == "" {
			if aliasSym := alias.Symbol(); aliasSym != nil {
				aliasName := aliasSym.Name
				if aliasName != "" && aliasName != "__type" && aliasName != "__object" && (len(aliasName) == 0 || aliasName[0] != '\xfe') {
					// For generic instantiations (e.g., PaginatedResponse<User>, Omit<Product, 'x'>),
					// build a composite name so each instantiation gets its own schema.
					// For non-generic aliases, use the bare alias name.
					registrationName := aliasName
					aliasTypeArgs := alias.TypeArguments()
					if len(aliasTypeArgs) > 0 {
						if compositeName, ok := w.buildGenericInstantiationName(aliasName, aliasTypeArgs); ok {
							registrationName = compositeName
						} else {
							// Anonymous type args — skip registration, inline, and warn
							w.warnAnonymousTypeArgs(aliasName)
							return w.analyzeObjectProperties(t, "")
						}
					}

					if w.registry.Has(registrationName) {
						return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName}
					}
					// Analyze properties first to check if it's a phantom object.
					// Save and reset breadth counter for the same reason as named objects above.
					savedBreadth := w.totalTypesWalked
					w.totalTypesWalked = 0
					w.visiting[t.Id()] = true
					result := w.analyzeObjectProperties(t, registrationName)
					delete(w.visiting, t.Id())
					w.totalTypesWalked = savedBreadth
					if isPhantomObject(&result) {
						// Don't register phantom objects — they're branded type building blocks
						return result
					}
					w.registry.Register(registrationName, &result)
					w.typeIdToName[t.Id()] = registrationName
					return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName}
				}
			}
		}
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

		// Merge JSDoc constraints (takes precedence) and extract property annotations
		var ann propertyAnnotations
		if prop.ValueDeclaration != nil {
			jsdocConstraints := w.extractJSDocConstraints(prop.ValueDeclaration)
			if jsdocConstraints != nil {
				if constraints == nil {
					constraints = jsdocConstraints
				} else {
					mergeConstraints(constraints, jsdocConstraints)
				}
			}
			ann = extractPropertyAnnotations(prop.ValueDeclaration)
		}

		properties = append(properties, metadata.Property{
			Name:          prop.Name,
			Type:          propMeta,
			Required:      !isOptional,
			Readonly:      isReadonly,
			ExactOptional: w.exactOptionalPropertyTypes && isOptional,
			Constraints:   constraints,
			Description:   ann.Description,
			WriteOnly:     ann.WriteOnly,
			Example:       ann.Example,
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
			if name != "Promise" && name != "Observable" && name != "Array" &&
				name != "AsyncGenerator" && name != "AsyncIterable" && name != "AsyncIterableIterator" {
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

// buildGenericInstantiationName creates a unique schema name for a generic type
// instantiation by appending type argument names to the base name.
// e.g., PaginatedResponse<UserDto> → ("PaginatedResponse_UserDto", true)
// Returns ("", false) when any type argument is anonymous/unnameable — callers
// should inline the type rather than register it under an opaque generated name.
func (w *TypeWalker) buildGenericInstantiationName(baseName string, typeArgs []*shimchecker.Type) (string, bool) {
	var sb strings.Builder
	sb.WriteString(baseName)
	for _, arg := range typeArgs {
		argName, ok := w.deriveTypeArgName(arg)
		if !ok {
			return "", false
		}
		sb.WriteString("_")
		sb.WriteString(argName)
	}
	return sb.String(), true
}

// deriveTypeArgName returns a human-readable name for a type, for use in
// composite generic instantiation names. Returns ("", false) when the type
// is anonymous and no name can be derived — the caller should inline instead.
func (w *TypeWalker) deriveTypeArgName(t *shimchecker.Type) (string, bool) {
	if t == nil {
		return "", false
	}
	flags := t.Flags()

	// Primitives
	if flags&shimchecker.TypeFlagsString != 0 {
		return "String", true
	}
	if flags&shimchecker.TypeFlagsNumber != 0 {
		return "Number", true
	}
	if flags&shimchecker.TypeFlagsBoolean != 0 {
		return "Boolean", true
	}
	if flags&shimchecker.TypeFlagsVoid != 0 {
		return "Void", true
	}
	if flags&shimchecker.TypeFlagsNull != 0 {
		return "Null", true
	}
	if flags&shimchecker.TypeFlagsUndefined != 0 {
		return "Undefined", true
	}
	if flags&shimchecker.TypeFlagsAny != 0 {
		return "Any", true
	}
	if flags&shimchecker.TypeFlagsNever != 0 {
		return "Never", true
	}

	// String literal
	if flags&shimchecker.TypeFlagsStringLiteral != 0 {
		lit := t.AsLiteralType()
		if lit != nil {
			if s, ok := lit.Value().(string); ok && s != "" {
				return strings.ToUpper(s[:1]) + s[1:], true
			}
		}
		return "", false
	}

	// Number literal
	if flags&shimchecker.TypeFlagsNumberLiteral != 0 {
		return fmt.Sprintf("N%v", t.AsLiteralType().Value()), true
	}

	// Object types (interfaces, classes, arrays)
	if flags&shimchecker.TypeFlagsObject != 0 {
		// Check the typeIdToName cache first — the pre-registration pass maps
		// type alias TypeIds to their declared names. This is the most reliable
		// way to recover names for type alias targets (anonymous objects) that
		// appear as type arguments in generic instantiations.
		if cachedName, ok := w.typeIdToName[t.Id()]; ok {
			return cachedName, true
		}

		// Arrays: derive from element type
		if shimchecker.Checker_isArrayType(w.checker, t) {
			elemArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
			if len(elemArgs) > 0 {
				if elemName, ok := w.deriveTypeArgName(elemArgs[0]); ok {
					return elemName + "Array", true
				}
			}
			return "", false
		}

		// Named type — use symbol name
		sym := t.Symbol()
		if sym != nil && sym.Name != "" && sym.Name != "__type" && sym.Name != "__object" {
			name := sym.Name
			if len(name) > 0 && name[0] == '\xfe' {
				return "", false
			}
			// For nested generic instantiations, recurse (only Reference types have type args)
			objFlags := shimchecker.Type_objectFlags(t)
			if objFlags&shimchecker.ObjectFlagsReference != 0 {
				innerArgs := shimchecker.Checker_getTypeArguments(w.checker, t)
				if len(innerArgs) > 0 {
					return w.buildGenericInstantiationName(name, innerArgs)
				}
			}
			return name, true
		}

		// Anonymous object — check for type alias name
		alias := shimchecker.Type_alias(t)
		if alias != nil {
			if aliasSym := alias.Symbol(); aliasSym != nil && aliasSym.Name != "" {
				name := aliasSym.Name
				if name != "__type" && name != "__object" && (len(name) == 0 || name[0] != '\xfe') {
					aliasArgs := alias.TypeArguments()
					if len(aliasArgs) > 0 {
						return w.buildGenericInstantiationName(name, aliasArgs)
					}
					return name, true
				}
			}
		}

		return "", false
	}

	// Union types
	if flags&shimchecker.TypeFlagsUnion != 0 {
		// Named union alias (e.g., type Status = 'active' | 'inactive')
		alias := shimchecker.Type_alias(t)
		if alias != nil {
			if aliasSym := alias.Symbol(); aliasSym != nil && aliasSym.Name != "" {
				return aliasSym.Name, true
			}
		}
		// Try to build a name from literal union members (common in Pick<T, 'a' | 'b'>).
		// Only attempt for small unions to avoid absurdly long names.
		unionType := t.AsUnionType()
		if unionType != nil {
			members := unionType.Types()
			if len(members) > 0 && len(members) <= 4 {
				var parts []string
				allLiterals := true
				for _, m := range members {
					mf := m.Flags()
					if mf&shimchecker.TypeFlagsStringLiteral != 0 {
						lit := m.AsLiteralType()
						if lit != nil {
							if s, ok := lit.Value().(string); ok && s != "" {
								parts = append(parts, strings.ToUpper(s[:1])+s[1:])
								continue
							}
						}
					}
					if mf&shimchecker.TypeFlagsNumberLiteral != 0 {
						parts = append(parts, fmt.Sprintf("N%v", m.AsLiteralType().Value()))
						continue
					}
					allLiterals = false
					break
				}
				if allLiterals && len(parts) > 0 {
					return strings.Join(parts, ""), true
				}
			}
		}
		return "", false
	}

	return "", false
}
