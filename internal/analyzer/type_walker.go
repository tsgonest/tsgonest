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
	// nameToTypeId maps registered type names to their TypeId.
	// Used to detect name collisions where different types share the same name.
	nameToTypeId map[string]shimchecker.TypeId
	// warnedGenericNames tracks generic type base names that have already emitted
	// a warning, to avoid flooding the output with duplicate messages.
	warnedGenericNames map[string]bool
	// currentRootContext identifies the top-level type currently being walked
	// (e.g., "AbandonedCartResponse (/src/dto.ts:45)"). Set by callers via
	// SetRootContext before invoking Walk*. Included in warnings so users can
	// trace which of their types consumes a generic with anonymous args.
	currentRootContext string
	// insideNamedType tracks whether we're currently inside a WalkNamedType call.
	// When > 0, anonymous inner types are being inlined into a named parent schema,
	// so "anonymous type arguments" warnings are suppressed — the parent already
	// provides the OpenAPI schema name.
	insideNamedType int
}

// NewTypeWalker creates a new TypeWalker.
func NewTypeWalker(checker *shimchecker.Checker) *TypeWalker {
	return &TypeWalker{
		checker:            checker,
		registry:           metadata.NewTypeRegistry(),
		visiting:           make(map[shimchecker.TypeId]bool),
		typeIdToName:       make(map[shimchecker.TypeId]string),
		pendingName:        make(map[shimchecker.TypeId]string),
		nameToTypeId:       make(map[string]shimchecker.TypeId),
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

// SetRootContext sets the context for the type currently being walked
// (e.g., "CreateUserDto (/src/users/dto.ts:12)"). Included in warnings
// so users can trace which of their types is the consumer. Call with ""
// to clear after the walk.
func (w *TypeWalker) SetRootContext(ctx string) {
	w.currentRootContext = ctx
}

// Warnings returns actionable diagnostics collected during type walking.
func (w *TypeWalker) Warnings() []string {
	return w.warnings
}

// warnAnonymousTypeArgs emits a deduplicated warning when a generic type
// is used with anonymous type arguments that can't be named for OpenAPI.
// The warning points at the consumer type (currentRootName) so the user
// knows which of their types needs attention.
func (w *TypeWalker) warnAnonymousTypeArgs(baseName string) {
	// When inside a WalkNamedType call, the anonymous inner type is being
	// inlined into a named parent schema (e.g., MeetingResponse). The parent
	// provides the OpenAPI schema name, so the warning is noise — the user
	// doesn't need to name the inner Omit/Pick/Partial separately.
	if w.insideNamedType > 0 {
		return
	}
	if w.warnedGenericNames[baseName] {
		return
	}
	w.warnedGenericNames[baseName] = true

	ctx := w.currentRootContext
	if ctx == "" {
		w.warnings = append(w.warnings, fmt.Sprintf(
			"%s is used with anonymous type arguments that cannot be named in OpenAPI — the type will be inlined instead of a named $ref",
			baseName,
		))
		return
	}

	w.warnings = append(w.warnings, fmt.Sprintf(
		"%s uses %s with anonymous type arguments — the type will be inlined in OpenAPI instead of a named $ref",
		ctx, baseName,
	))
}

// hasNamedAlias returns true if the type has a named type alias declaration
// (e.g., `type MeetingResponse = Omit<...> & {...}`). Used to suppress
// anonymous type argument warnings when the parent type already provides a name.
func (w *TypeWalker) hasNamedAlias(t *shimchecker.Type) bool {
	alias := shimchecker.Type_alias(t)
	if alias == nil {
		return false
	}
	sym := alias.Symbol()
	if sym == nil {
		return false
	}
	name := sym.Name
	return name != "" && name != "__type" && name != "__object" && (len(name) == 0 || name[0] != '\xfe')
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
	w.insideNamedType++

	// Walk the type. Don't set visiting here — let walkObjectType,
	// walkIntersection, and walkUnion manage their own recursion guards.
	// Setting visiting here would cause walkIntersection to short-circuit
	// on the first call, preventing the intersection from being analyzed.
	m := w.WalkType(t)
	w.insideNamedType--
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
		w.nameToTypeId[name] = t.Id()
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

	var result metadata.Metadata
	switch {
	case flags&shimchecker.TypeFlagsUnion != 0:
		result = w.walkUnion(t)
	case flags&shimchecker.TypeFlagsIntersection != 0:
		result = w.walkIntersection(t)
	default:
		result = w.walkSingleType(t)
	}

	return w.applyAliasJSDoc(t, result)
}

// extractAliasJSDocFromTypeNode returns JSDoc constraints found on the alias
// declaration that the given type node references, or nil if no alias-JSDoc
// is found. Handles primitive aliases like `type MongoId = string;` where the
// resolved Type carries no AliasSymbol (TypeScript collapses primitive aliases
// to the shared singleton, so we recover the alias via the AST type node).
func (w *TypeWalker) extractAliasJSDocFromTypeNode(typeNode *ast.Node) *metadata.Constraints {
	if typeNode == nil {
		return nil
	}
	// Unwrap parenthesized types so `(MongoId)` is handled too.
	for typeNode != nil && typeNode.Kind == ast.KindParenthesizedType {
		typeNode = typeNode.Type()
	}
	if typeNode == nil || typeNode.Kind != ast.KindTypeReference {
		return nil
	}
	ref := typeNode.AsTypeReferenceNode()
	if ref == nil || ref.TypeName == nil {
		return nil
	}
	sym := w.checker.GetSymbolAtLocation(ref.TypeName)
	if sym == nil {
		return nil
	}
	// For cross-file imports, GetSymbolAtLocation returns an import-alias symbol
	// whose declarations are ImportSpecifier nodes. Resolve to the original
	// symbol so we reach the TypeAliasDeclaration carrying the JSDoc.
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		if original := w.checker.GetAliasedSymbol(sym); original != nil {
			sym = original
		}
	}
	if sym.Declarations == nil {
		return nil
	}
	for _, decl := range sym.Declarations {
		if decl == nil || decl.Kind != ast.KindTypeAliasDeclaration {
			continue
		}
		if c := w.extractJSDocConstraints(decl); c != nil {
			return c
		}
	}
	return nil
}

// applyAliasJSDoc enriches result.Constraints with JSDoc tags found on the
// type's alias declaration (e.g. /** @pattern */ type MongoId = string).
// Only applies to atomic/array kinds where JSDoc validation tags are
// semantically meaningful. Existing constraints (branded inline tags) take
// precedence over alias-site JSDoc, mirroring the broader → narrower
// precedence already used between branded and property-site JSDoc.
//
// Note: this only catches aliases whose info is attached directly on the type
// (object/array aliases). Primitive aliases (`type MongoId = string`) collapse
// to a shared singleton with no AliasSymbol, so AST-position-based extraction
// via extractAliasJSDocFromTypeNode is required at call sites that have the
// originating TypeNode in hand (e.g. WalkTypeNode, analyzeObjectProperties).
func (w *TypeWalker) applyAliasJSDoc(t *shimchecker.Type, result metadata.Metadata) metadata.Metadata {
	if result.Kind != metadata.KindAtomic && result.Kind != metadata.KindArray {
		return result
	}
	var aliasJSDoc *metadata.Constraints

	// Fast path: alias info is attached directly on the type (object/array aliases).
	if alias := shimchecker.Type_alias(t); alias != nil {
		if sym := alias.Symbol(); sym != nil {
			name := sym.Name
			if !(name == "" || name == "__type" || name == "__object" || (len(name) > 0 && name[0] == '\xfe')) && sym.Declarations != nil {
				for _, decl := range sym.Declarations {
					if decl == nil || decl.Kind != ast.KindTypeAliasDeclaration {
						continue
					}
					if c := w.extractJSDocConstraints(decl); c != nil {
						aliasJSDoc = c
						break
					}
				}
			}
		}
	}

	return w.mergeAliasJSDocInto(result, aliasJSDoc)
}

// mergeAliasJSDocInto merges alias-site JSDoc constraints into result.Constraints
// using the precedence rule shared by applyAliasJSDoc and WalkTypeNode: existing
// (branded/inline) constraints win over alias-site JSDoc.
func (w *TypeWalker) mergeAliasJSDocInto(result metadata.Metadata, aliasJSDoc *metadata.Constraints) metadata.Metadata {
	if aliasJSDoc == nil {
		return result
	}
	if result.Constraints == nil {
		result.Constraints = aliasJSDoc
		return result
	}
	merged := *aliasJSDoc
	mergeConstraints(&merged, result.Constraints)
	result.Constraints = &merged
	return result
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
		constraint := w.checker.GetBaseConstraintOfType(t)
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

	// If this union has a named alias, suppress anonymous type arg warnings for
	// inner members — they get inlined into the named parent schema.
	if w.hasNamedAlias(t) {
		w.insideNamedType++
		defer func() { w.insideNamedType-- }()
	}

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
		values := make(map[string]any)

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
					values[litVal] = p.Type.LiteralValue
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
				Values:   values,
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
			props := w.checker.GetPropertiesOfType(member)
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
				propType := w.checker.GetTypeOfSymbol(prop)
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

	// If this intersection has a named alias (e.g., type MeetingResponse = Omit<...> & {...}),
	// suppress anonymous type arg warnings for inner members — they get inlined into
	// the named parent schema, so having their own $ref name is unnecessary.
	namedParent := w.hasNamedAlias(t)
	if namedParent {
		w.insideNamedType++
		defer func() { w.insideNamedType-- }()
	}

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
		typeArgs := w.checker.GetTypeArguments(t)
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
	objFlags := t.ObjectFlags()
	if objFlags&shimchecker.ObjectFlagsClassOrInterface != 0 {
		baseTypes := w.checker.GetBaseTypes(t)
		for _, base := range baseTypes {
			if shimchecker.Checker_isArrayType(w.checker, base) {
				typeArgs := w.checker.GetTypeArguments(base)
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
			typeArgs := w.checker.GetTypeArguments(t)
			if len(typeArgs) > 0 {
				return w.WalkType(typeArgs[0])
			}
			return metadata.Metadata{Kind: metadata.KindAny}
		case "Observable":
			// Unwrap Observable<T> to T (rxjs Observable, used for SSE endpoints)
			// Combined with Promise unwrapping, this handles Promise<Observable<T>> → T
			typeArgs := w.checker.GetTypeArguments(t)
			if len(typeArgs) > 0 {
				return w.WalkType(typeArgs[0])
			}
			return metadata.Metadata{Kind: metadata.KindAny}
		case "AsyncGenerator":
			// Unwrap AsyncGenerator<Y, R, N> → Y (yield type, first type arg).
			// Used by @EventStream() which returns AsyncGenerator<SseEvent<...>>.
			typeArgs := w.checker.GetTypeArguments(t)
			if len(typeArgs) > 0 {
				return w.WalkType(typeArgs[0])
			}
			return metadata.Metadata{Kind: metadata.KindAny}
		case "AsyncIterable", "AsyncIterableIterator":
			// Unwrap AsyncIterable<T> / AsyncIterableIterator<T> → T.
			typeArgs := w.checker.GetTypeArguments(t)
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
		case "File", "Blob", "StreamableFile":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: name}
		case "Error":
			return metadata.Metadata{Kind: metadata.KindNative, NativeType: "Error"}
		}
	}

	// Check if this is a function type (has call signatures, no properties of interest)
	callSigs := w.checker.GetSignaturesOfType(t, shimchecker.SignatureKindCall)
	props := w.checker.GetPropertiesOfType(t)
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
		objFlags := t.ObjectFlags()
		if objFlags&shimchecker.ObjectFlagsReference != 0 {
			typeArgs := w.checker.GetTypeArguments(t)
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
			if w.nameToTypeId[typeName] == t.Id() || w.nameToTypeId[typeName] == 0 {
				// Same type — return a $ref
				return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
			}
			// Name collision — different type with same name, disambiguate
			typeName = w.disambiguateName(typeName, t)
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
		w.nameToTypeId[typeName] = t.Id()
		return metadata.Metadata{Kind: metadata.KindRef, Ref: typeName}
	}

	// Anonymous object type — check if we've seen this TypeId before
	// (type aliases produce anonymous types that were registered by name via WalkNamedType)
	if cachedName, ok := w.typeIdToName[t.Id()]; ok {
		return metadata.Metadata{Kind: metadata.KindRef, Ref: cachedName}
	}

	// Generic type alias recovery — works at ANY depth.
	// Type aliases resolve to anonymous types (ObjectFlagsAnonymous), so
	// getTypeName returns "". But Type_alias preserves the alias declaration,
	// letting us recover the original name. Generic instantiations (with type
	// args) need composite naming at every depth, including depth 0/1 where
	// controller return types are walked via WalkTypeNode/WalkType. Without
	// this, all instantiations of e.g. PaginatedResponse<T> collapse to a
	// single schema because the anonymous type has no name.
	// Skip when being walked by WalkNamedType (has a pendingName entry).
	if w.pendingName[t.Id()] == "" {
		alias := shimchecker.Type_alias(t)
		if alias != nil {
			if aliasSym := alias.Symbol(); aliasSym != nil {
				aliasName := aliasSym.Name
				if aliasName != "" && aliasName != "__type" && aliasName != "__object" && (len(aliasName) == 0 || aliasName[0] != '\xfe') {
					aliasTypeArgs := alias.TypeArguments()
					if len(aliasTypeArgs) > 0 {
						// Generic type alias instantiation (e.g., PaginatedResponse<User>).
						// Build composite name so each instantiation gets its own schema.
						compositeName, ok := w.buildGenericInstantiationName(aliasName, aliasTypeArgs)
						if !ok {
							// Anonymous type args — skip registration, inline, and warn
							w.warnAnonymousTypeArgs(aliasName)
							return w.analyzeObjectProperties(t, "")
						}
						return w.registerAnonymousAlias(t, compositeName)
					}
					// Non-generic alias: only recover at depth > 1.
					// WalkNamedType handles registration for top-level types.
					if w.depth > 1 {
						return w.registerAnonymousAlias(t, aliasName)
					}
				}
			}
		}
	}

	// Anonymous object type — inline the properties
	return w.analyzeObjectProperties(t, "")
}

// registerAnonymousAlias checks if registrationName is already registered,
// returning a KindRef if so. Otherwise it analyzes the object properties,
// skips phantom objects, and registers the type under registrationName.
// This is shared by both generic and non-generic alias recovery paths.
func (w *TypeWalker) registerAnonymousAlias(t *shimchecker.Type, registrationName string) metadata.Metadata {
	if w.registry.Has(registrationName) {
		if w.nameToTypeId[registrationName] == t.Id() || w.nameToTypeId[registrationName] == 0 {
			return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName}
		}
		// Name collision — different type with same name, disambiguate
		registrationName = w.disambiguateName(registrationName, t)
	}
	// Save and reset breadth counter so this named type gets its own budget.
	savedBreadth := w.totalTypesWalked
	w.totalTypesWalked = 0
	w.visiting[t.Id()] = true
	result := w.analyzeObjectProperties(t, registrationName)
	delete(w.visiting, t.Id())
	w.totalTypesWalked = savedBreadth
	if isPhantomObject(&result) {
		return result
	}
	w.registry.Register(registrationName, &result)
	w.typeIdToName[t.Id()] = registrationName
	w.nameToTypeId[registrationName] = t.Id()
	return metadata.Metadata{Kind: metadata.KindRef, Ref: registrationName}
}

// disambiguateName generates a unique name by appending _2, _3, etc.
// when two different types share the same symbol name. Emits a warning.
func (w *TypeWalker) disambiguateName(baseName string, t *shimchecker.Type) string {
	candidate := baseName
	for i := 2; w.registry.Has(candidate) && w.nameToTypeId[candidate] != t.Id(); i++ {
		candidate = fmt.Sprintf("%s_%d", baseName, i)
	}
	if !w.warnedGenericNames[baseName+"_collision"] {
		w.warnedGenericNames[baseName+"_collision"] = true
		ctx := ""
		if w.currentRootContext != "" {
			ctx = " (while walking " + w.currentRootContext + ")"
		}
		w.warnings = append(w.warnings, fmt.Sprintf(
			"type-name-collision: Multiple types named %q with different definitions were found%s — the second was registered as %q. Consider renaming one of them to avoid confusion.",
			baseName, ctx, candidate))
	}
	return candidate
}

// analyzeObjectProperties extracts properties from an object type.
func (w *TypeWalker) analyzeObjectProperties(t *shimchecker.Type, name string) metadata.Metadata {
	props := w.checker.GetPropertiesOfType(t)
	var properties []metadata.Property

	for _, prop := range props {
		propType := w.checker.GetTypeOfSymbol(prop)

		propMeta := w.WalkType(propType)

		isOptional := prop.Flags&ast.SymbolFlagsOptional != 0
		if isOptional {
			propMeta.Optional = true
		}

		isReadonly := shimchecker.Checker_isReadonlySymbol(w.checker, prop)

		// Extract constraints from three sources, broadest → narrowest:
		// 1. Alias-site JSDoc (e.g. /** @pattern */ type MongoId = string)
		// 2. Branded phantom types (e.g., string & tags.Format<"email">)
		// 3. Property-site JSDoc (e.g. /** @format email */ on the field)
		// Later sources override earlier ones — property-site is the most specific.
		var constraints *metadata.Constraints

		// Seed with alias-site JSDoc when the property type is a TypeReference
		// to a type alias whose declaration has JSDoc tags. Primitive aliases
		// like `type MongoId = string` lose AliasSymbol on the resolved Type,
		// so we recover the alias via the property's AST type node.
		if prop.ValueDeclaration != nil {
			if aliasJSDoc := w.extractAliasJSDocFromTypeNode(prop.ValueDeclaration.Type()); aliasJSDoc != nil {
				c := *aliasJSDoc // copy so subsequent merges don't mutate the source
				constraints = &c
			}
		}

		// Layer in branded type constraints (override alias-site for any same fields)
		if propMeta.Constraints != nil {
			if constraints == nil {
				c := *propMeta.Constraints // copy
				constraints = &c
			} else {
				mergeConstraints(constraints, propMeta.Constraints)
			}
			propMeta.Constraints = nil // don't leak to codegen
		}

		// Element-level alias-JSDoc: for `userIds: MongoId[]`, the property's type
		// node is an ArrayType. Recurse into its ElementType to recover alias-JSDoc
		// on the element, and attach to propMeta.ElementType.Constraints so codegen's
		// per-element loop can emit the runtime check.
		if prop.ValueDeclaration != nil && propMeta.Kind == metadata.KindArray && propMeta.ElementType != nil {
			if typeNode := prop.ValueDeclaration.Type(); typeNode != nil && typeNode.Kind == ast.KindArrayType {
				elemNode := typeNode.AsArrayTypeNode().ElementType
				if elemAliasJSDoc := w.extractAliasJSDocFromTypeNode(elemNode); elemAliasJSDoc != nil {
					merged := w.mergeAliasJSDocInto(*propMeta.ElementType, elemAliasJSDoc)
					propMeta.ElementType = &merged
				}
			}
		}

		// Merge property-site JSDoc (most specific — takes precedence) and extract annotations
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
	indexInfos := w.checker.GetIndexInfosOfType(t)
	if len(indexInfos) > 0 {
		info := indexInfos[0]
		keyMeta := w.WalkType(info.KeyType())
		valMeta := w.WalkType(info.ValueType())
		result.IndexSignature = &metadata.IndexSignature{
			KeyType:   keyMeta,
			ValueType: valMeta,
		}
	}

	return result
}

// walkTupleType handles tuple types like [string, number].
func (w *TypeWalker) walkTupleType(t *shimchecker.Type) metadata.Metadata {
	typeArgs := w.checker.GetTypeArguments(t)
	tupleType := t.TargetTupleType()

	var elements []metadata.TupleElement
	var elementInfos []shimchecker.TupleElementInfo
	if tupleType != nil {
		elementInfos = tupleType.ElementInfos()
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
	typeArgs := w.checker.GetTypeArguments(t)
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
	objFlags := t.ObjectFlags()
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
	t := w.checker.GetTypeFromTypeNode(node)
	result := w.WalkType(t)

	// Recover alias-site JSDoc constraints for primitive aliases via the AST
	// position. TypeScript collapses `type MongoId = string` to the shared
	// primitive singleton, so applyAliasJSDoc's type-driven fast path misses
	// these. extractAliasJSDocFromTypeNode resolves the symbol referenced at
	// this exact AST position, which is unambiguous even when many aliases
	// share the same underlying primitive type.
	if result.Kind == metadata.KindAtomic || result.Kind == metadata.KindArray {
		if aliasJSDoc := w.extractAliasJSDocFromTypeNode(node); aliasJSDoc != nil {
			result = w.mergeAliasJSDocInto(result, aliasJSDoc)
		}
	}

	// Element-level alias-JSDoc on top-level array walks (e.g. `@Param('ids') ids: MongoId[]`
	// or method return type `MongoId[]`). Attaches to result.ElementType.Constraints.
	if result.Kind == metadata.KindArray && result.ElementType != nil && node != nil && node.Kind == ast.KindArrayType {
		elemNode := node.AsArrayTypeNode().ElementType
		if elemAliasJSDoc := w.extractAliasJSDocFromTypeNode(elemNode); elemAliasJSDoc != nil {
			merged := w.mergeAliasJSDocInto(*result.ElementType, elemAliasJSDoc)
			result.ElementType = &merged
		}
	}

	// Preserve the type name for named type references.
	// Skip wrapper types (Promise, Observable) — their inner type is already unwrapped.
	if result.Name == "" && node.Kind == ast.KindTypeReference {
		ref := node.AsTypeReferenceNode()
		if ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier {
			name := ref.TypeName.Text()
			if name != "Promise" && name != "Observable" && name != "Array" &&
				name != "AsyncGenerator" && name != "AsyncIterable" && name != "AsyncIterableIterator" {
				// Only set Name when it matches the resolved Ref (or when the result is
				// not a KindRef at all). For generic instantiations like PaginatedResponse<LeadDto>,
				// WalkType produces Ref="PaginatedResponse_LeadDto" (composite). Setting
				// Name to "PaginatedResponse" would cause convertRef in the OpenAPI generator
				// to use "PaginatedResponse" as the schema name, collapsing all instantiations
				// to the same (wrong) schema.
				if result.Kind != metadata.KindRef || result.Ref == name {
					result.Name = name
				}
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
			elemArgs := w.checker.GetTypeArguments(t)
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
			objFlags := t.ObjectFlags()
			if objFlags&shimchecker.ObjectFlagsReference != 0 {
				innerArgs := w.checker.GetTypeArguments(t)
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
