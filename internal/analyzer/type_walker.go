// Package analyzer provides AST and type analysis utilities for tsgonest.
package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// maxWalkDepth is the maximum nesting depth for type walking.
// Prevents stack overflow from deeply recursive or infinitely expanding types
// (e.g., TypedOmit<Prisma.Entity, 'x'> creating unique anonymous types at each level).
const maxWalkDepth = 20

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

// Registry returns the type registry with all discovered named types.
func (w *TypeWalker) Registry() *metadata.TypeRegistry {
	return w.registry
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

	// If the result is an inline object, promote it to a named ref
	if m.Kind == metadata.KindObject && m.Name == "" {
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
	w.depth++
	defer func() { w.depth-- }()

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

	// Try to detect a discriminant property for discriminated unions
	if disc := w.detectDiscriminant(members); disc != nil {
		result.Discriminant = disc
	}

	return result
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

// tryDetectBranded checks if an intersection is a branded type pattern like
// `string & { __brand: 'Email' }`. Returns the atomic type if detected,
// otherwise nil. Also extracts validation constraints from phantom properties
// with the `__tsgonest_` prefix (from @tsgonest/types branded types) and
// the `__typia_tag_` prefix (for typia migration compatibility).
// rawTypes are the original shimchecker types corresponding to members, used for
// function type resolution (Validate<typeof fn>).
func (w *TypeWalker) tryDetectBranded(rawTypes []*shimchecker.Type, members []metadata.Metadata) *metadata.Metadata {
	var atomicMember *metadata.Metadata
	var phantomMembers []*metadata.Metadata
	var rawPhantomTypes []*shimchecker.Type
	phantomCount := 0

	for i := range members {
		m := &members[i]
		if m.Kind == metadata.KindAtomic {
			if atomicMember != nil {
				return nil // Multiple atomics — not a branded type
			}
			atomicMember = m
		} else if m.Kind == metadata.KindObject && isPhantomObject(m) {
			phantomMembers = append(phantomMembers, m)
			if i < len(rawTypes) {
				rawPhantomTypes = append(rawPhantomTypes, rawTypes[i])
			}
			phantomCount++
		} else {
			return nil // Non-atomic, non-phantom member — not a branded type
		}
	}

	if atomicMember != nil && phantomCount > 0 {
		// Return the atomic type, stripping the phantom objects
		result := *atomicMember

		// Extract constraints from __tsgonest_* and __typia_tag_* phantom properties
		constraints := w.extractBrandedConstraints(rawPhantomTypes, phantomMembers)
		if constraints != nil {
			result.Constraints = constraints
		}

		return &result
	}
	return nil
}

// extractBrandedConstraints inspects phantom object members for constraint info:
// 1. __tsgonest_* properties (from @tsgonest/types branded types)
// 2. "typia.tag" property (for typia migration compatibility)
// rawPhantomTypes are the original shimchecker types for function type resolution.
// Returns extracted constraints or nil.
func (w *TypeWalker) extractBrandedConstraints(rawPhantomTypes []*shimchecker.Type, phantomMembers []*metadata.Metadata) *metadata.Constraints {
	var c metadata.Constraints
	found := false

	for phantomIdx, m := range phantomMembers {
		for _, prop := range m.Properties {
			name := prop.Name

			// 1. @tsgonest/types: __tsgonest_<constraint> → constraint field
			if len(name) > 11 && name[:11] == "__tsgonest_" {
				constraintKey := name[11:] // strip "__tsgonest_"

				// Check for per-constraint error: __tsgonest_<key>_error
				if strings.HasSuffix(constraintKey, "_error") {
					baseKey := constraintKey[:len(constraintKey)-6] // strip "_error"
					// Don't match transform_* keys (transform names won't end in _error)
					if baseKey != "" && !strings.HasPrefix(baseKey, "transform_") {
						if s, ok := literalString(&prop.Type); ok {
							if c.Errors == nil {
								c.Errors = make(map[string]string)
							}
							c.Errors[baseKey] = s
							found = true
						}
					}
				} else if constraintKey == "validate" {
					// Special case: __tsgonest_validate holds a function type reference.
					// We need the raw type to resolve the function's symbol name and source file.
					if phantomIdx < len(rawPhantomTypes) {
						if w.extractValidateFnConstraint(&c, rawPhantomTypes[phantomIdx], name) {
							found = true
						}
					}
				} else if extractConstraintValue(&c, constraintKey, &prop.Type) {
					found = true
				}
			}

			// 2. typia migration: "typia.tag" → extract kind+value from object type
			if name == "typia.tag" {
				tagType := &prop.Type
				// Handle optional "typia.tag?": type may be wrapped in a union with undefined
				if tagType.Kind == metadata.KindUnion {
					for i := range tagType.UnionMembers {
						um := &tagType.UnionMembers[i]
						if um.Kind == metadata.KindObject {
							tagType = um
							break
						}
					}
				}
				if tagType.Kind == metadata.KindObject {
					if extractTypiaTagConstraint(&c, tagType) {
						found = true
					}
				}
			}
		}
	}

	if found {
		return &c
	}
	return nil
}

// extractValidateFnConstraint resolves a Validate<typeof fn> branded type.
// rawPhantomType is the raw shimchecker.Type for the phantom object that contains
// the __tsgonest_validate property. propName is the full property name.
// Extracts the function's symbol name and source file path into c.ValidateFn and c.ValidateModule.
func (w *TypeWalker) extractValidateFnConstraint(c *metadata.Constraints, rawPhantomType *shimchecker.Type, propName string) bool {
	// Get the __tsgonest_validate property from the raw phantom type
	validatePropSym := shimchecker.Checker_getPropertyOfType(w.checker, rawPhantomType, propName)
	if validatePropSym == nil {
		return false
	}

	// Get the type of the __tsgonest_validate property — this is the function type
	fnType := shimchecker.Checker_getTypeOfSymbol(w.checker, validatePropSym)
	if fnType == nil {
		return false
	}

	// The function type's symbol gives us the function name
	fnSym := fnType.Symbol()
	if fnSym == nil {
		return false
	}

	fnName := fnSym.Name
	if fnName == "" {
		return false
	}

	// Get the source file of the function's declaration
	var sourceFilePath string
	if fnSym.ValueDeclaration != nil {
		sf := ast.GetSourceFileOfNode(fnSym.ValueDeclaration)
		if sf != nil {
			sourceFilePath = sf.FileName()
		}
	}

	c.ValidateFn = &fnName
	if sourceFilePath != "" {
		c.ValidateModule = &sourceFilePath
	}
	return true
}

// extractTypiaTagConstraint extracts a constraint from a typia "typia.tag" property.
// Typia's tag structure: { target: "string"; kind: "format"; value: "email"; ... }
// We extract `kind` and `value` to map to our constraint system.
func extractTypiaTagConstraint(c *metadata.Constraints, tagType *metadata.Metadata) bool {
	var kind string
	var valueMeta *metadata.Metadata

	for i := range tagType.Properties {
		p := &tagType.Properties[i]
		switch p.Name {
		case "kind":
			if s, ok := literalString(&p.Type); ok {
				kind = s
			}
		case "value":
			valueMeta = &p.Type
		}
	}

	if kind == "" || valueMeta == nil {
		return false
	}

	// Map typia kind to our constraint keys
	return extractConstraintValue(c, kind, valueMeta)
}

// extractConstraintValue sets a constraint field from a branded type literal value.
// Returns true if a constraint was successfully extracted.
func extractConstraintValue(c *metadata.Constraints, key string, typeMeta *metadata.Metadata) bool {
	switch key {
	// String constraints
	case "format":
		if s, ok := literalString(typeMeta); ok {
			c.Format = &s
			return true
		}
	case "minLength":
		if n, ok := literalInt(typeMeta); ok {
			c.MinLength = &n
			return true
		}
	case "maxLength":
		if n, ok := literalInt(typeMeta); ok {
			c.MaxLength = &n
			return true
		}
	case "pattern":
		if s, ok := literalString(typeMeta); ok {
			c.Pattern = &s
			return true
		}
	case "startsWith":
		if s, ok := literalString(typeMeta); ok {
			c.StartsWith = &s
			return true
		}
	case "endsWith":
		if s, ok := literalString(typeMeta); ok {
			c.EndsWith = &s
			return true
		}
	case "includes":
		if s, ok := literalString(typeMeta); ok {
			c.Includes = &s
			return true
		}

	// Numeric constraints
	case "minimum":
		if f, ok := literalFloat(typeMeta); ok {
			c.Minimum = &f
			return true
		}
	case "maximum":
		if f, ok := literalFloat(typeMeta); ok {
			c.Maximum = &f
			return true
		}
	case "exclusiveMinimum":
		if f, ok := literalFloat(typeMeta); ok {
			c.ExclusiveMinimum = &f
			return true
		}
	case "exclusiveMaximum":
		if f, ok := literalFloat(typeMeta); ok {
			c.ExclusiveMaximum = &f
			return true
		}
	case "multipleOf":
		if f, ok := literalFloat(typeMeta); ok {
			c.MultipleOf = &f
			return true
		}
	case "type":
		if s, ok := literalString(typeMeta); ok {
			c.NumericType = &s
			return true
		}

	// Array constraints
	case "minItems":
		if n, ok := literalInt(typeMeta); ok {
			c.MinItems = &n
			return true
		}
	case "maxItems":
		if n, ok := literalInt(typeMeta); ok {
			c.MaxItems = &n
			return true
		}
	case "uniqueItems":
		if b, ok := literalBool(typeMeta); ok && b {
			c.UniqueItems = &b
			return true
		}

	// String case validation
	case "uppercase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Uppercase = &b
			return true
		}
	case "lowercase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Lowercase = &b
			return true
		}

	// Transforms (applied before validation)
	case "transform_trim":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, "trim")
			return true
		}
	case "transform_toLowerCase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, "toLowerCase")
			return true
		}
	case "transform_toUpperCase":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, "toUpperCase")
			return true
		}

	// Custom error message
	case "error":
		if s, ok := literalString(typeMeta); ok {
			c.ErrorMessage = &s
			return true
		}

	// Default value
	case "default":
		if s, ok := literalString(typeMeta); ok {
			c.Default = &s
			return true
		}
		// Also support numeric/boolean defaults as string representation
		if f, ok := literalFloat(typeMeta); ok {
			s := fmt.Sprintf("%v", f)
			c.Default = &s
			return true
		}
		if b, ok := literalBool(typeMeta); ok {
			s := fmt.Sprintf("%v", b)
			c.Default = &s
			return true
		}

	// Coercion (string→number, string→boolean, string→Date)
	case "coerce":
		if b, ok := literalBool(typeMeta); ok && b {
			c.Coerce = &b
			return true
		}
	}
	return false
}

// literalString extracts a string value from a literal metadata.
func literalString(m *metadata.Metadata) (string, bool) {
	if m.Kind == metadata.KindLiteral {
		if s, ok := m.LiteralValue.(string); ok {
			return s, true
		}
	}
	return "", false
}

// literalFloat extracts a float64 value from a literal metadata.
func literalFloat(m *metadata.Metadata) (float64, bool) {
	if m.Kind == metadata.KindLiteral {
		switch v := m.LiteralValue.(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		}
	}
	return 0, false
}

// literalInt extracts an int value from a literal metadata.
func literalInt(m *metadata.Metadata) (int, bool) {
	if m.Kind == metadata.KindLiteral {
		switch v := m.LiteralValue.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case int64:
			return int(v), true
		}
	}
	return 0, false
}

// literalBool extracts a boolean value from a literal metadata.
func literalBool(m *metadata.Metadata) (bool, bool) {
	if m.Kind == metadata.KindLiteral {
		if b, ok := m.LiteralValue.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// mergeConstraints merges src constraints into dst. src values take precedence
// (override dst values when both are set).
func mergeConstraints(dst, src *metadata.Constraints) {
	if src.Minimum != nil {
		dst.Minimum = src.Minimum
	}
	if src.Maximum != nil {
		dst.Maximum = src.Maximum
	}
	if src.ExclusiveMinimum != nil {
		dst.ExclusiveMinimum = src.ExclusiveMinimum
	}
	if src.ExclusiveMaximum != nil {
		dst.ExclusiveMaximum = src.ExclusiveMaximum
	}
	if src.MultipleOf != nil {
		dst.MultipleOf = src.MultipleOf
	}
	if src.NumericType != nil {
		dst.NumericType = src.NumericType
	}
	if src.MinLength != nil {
		dst.MinLength = src.MinLength
	}
	if src.MaxLength != nil {
		dst.MaxLength = src.MaxLength
	}
	if src.Pattern != nil {
		dst.Pattern = src.Pattern
	}
	if src.Format != nil {
		dst.Format = src.Format
	}
	if src.StartsWith != nil {
		dst.StartsWith = src.StartsWith
	}
	if src.EndsWith != nil {
		dst.EndsWith = src.EndsWith
	}
	if src.Includes != nil {
		dst.Includes = src.Includes
	}
	if src.Uppercase != nil {
		dst.Uppercase = src.Uppercase
	}
	if src.Lowercase != nil {
		dst.Lowercase = src.Lowercase
	}
	if src.ContentMediaType != nil {
		dst.ContentMediaType = src.ContentMediaType
	}
	if len(src.Transforms) > 0 {
		dst.Transforms = src.Transforms
	}
	if src.MinItems != nil {
		dst.MinItems = src.MinItems
	}
	if src.MaxItems != nil {
		dst.MaxItems = src.MaxItems
	}
	if src.UniqueItems != nil {
		dst.UniqueItems = src.UniqueItems
	}
	if src.Default != nil {
		dst.Default = src.Default
	}
	if src.Coerce != nil {
		dst.Coerce = src.Coerce
	}
	if src.ValidateFn != nil {
		dst.ValidateFn = src.ValidateFn
	}
	if src.ValidateModule != nil {
		dst.ValidateModule = src.ValidateModule
	}
	if src.ErrorMessage != nil {
		dst.ErrorMessage = src.ErrorMessage
	}
}

// isPhantomObject checks if an object type only has "phantom" properties
// (properties that exist only for type branding, not runtime data).
// Common patterns: __brand, __meta, __phantom, __type, __tag, __opaque
// Also recognizes "typia.tag" for typia migration compatibility.
func isPhantomObject(m *metadata.Metadata) bool {
	if len(m.Properties) == 0 {
		return false
	}
	for _, prop := range m.Properties {
		name := prop.Name
		if isPhantomPropertyName(name) {
			continue
		}
		return false // Non-phantom property found
	}
	return true
}

// isPhantomPropertyName returns true if the property name is a phantom/branding property.
func isPhantomPropertyName(name string) bool {
	// Standard phantom: starts with __
	if len(name) >= 2 && name[:2] == "__" {
		return true
	}
	// Typia phantom: "typia.tag"
	if name == "typia.tag" {
		return true
	}
	return false
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
			Name:        prop.Name,
			Type:        propMeta,
			Required:    !isOptional,
			Readonly:    isReadonly,
			Constraints: constraints,
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
func (w *TypeWalker) WalkTypeNode(node *ast.Node) metadata.Metadata {
	t := shimchecker.Checker_getTypeFromTypeNode(w.checker, node)
	return w.WalkType(t)
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
		pattern.WriteString(regexEscape(text))

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

// regexEscape escapes regex special characters in a string.
func regexEscape(s string) string {
	special := `\.+*?^${}()|[]`
	var result strings.Builder
	for _, ch := range s {
		for _, sp := range special {
			if ch == sp {
				result.WriteRune('\\')
				break
			}
		}
		result.WriteRune(ch)
	}
	return result.String()
}

// extractJSDocConstraints parses JSDoc tags on a declaration node to extract
// validation constraints like @minimum, @maximum, @minLength, @maxLength,
// @pattern, @format, @minItems, @maxItems.
func (w *TypeWalker) extractJSDocConstraints(node *ast.Node) *metadata.Constraints {
	if node == nil {
		return nil
	}

	// Get JSDoc comments attached to this node
	// JSDoc() takes a *SourceFile parameter; pass nil to search from the node directly
	jsdocs := node.JSDoc(nil)
	if len(jsdocs) == 0 {
		return nil
	}

	var c metadata.Constraints
	found := false

	// Process the last JSDoc comment (the most recent one)
	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()
	if jsdoc.Tags == nil {
		return nil
	}

	for _, tagNode := range jsdoc.Tags.Nodes {
		// Custom tags (@minimum, @maxLength, etc.) are KindJSDocTag (unknown tags)
		// We need to safely get the tag name and comment.
		tagName, comment := extractJSDocTagInfo(tagNode)
		if tagName == "" {
			continue
		}
		tagName = strings.ToLower(tagName)

		switch tagName {
		case "minimum", "min":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.Minimum = &v
				found = true
			}
		case "maximum", "max":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.Maximum = &v
				found = true
			}
		case "minlength":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinLength = &v
				found = true
			}
		case "maxlength":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MaxLength = &v
				found = true
			}
		case "len", "length":
			// Shorthand: sets both minLength and maxLength to the same value
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinLength = &v
				c.MaxLength = &v
				found = true
			}
		case "items":
			// Shorthand: sets both minItems and maxItems to the same value
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinItems = &v
				c.MaxItems = &v
				found = true
			}
		case "pattern":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.Pattern = &s
				found = true
			}
		case "format":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.Format = &s
				found = true
			}
		case "exclusiveminimum":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.ExclusiveMinimum = &v
				found = true
			}
		case "exclusivemaximum":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.ExclusiveMaximum = &v
				found = true
			}
		case "multipleof":
			if v, err := strconv.ParseFloat(strings.TrimSpace(comment), 64); err == nil {
				c.MultipleOf = &v
				found = true
			}
		case "type", "numerictype":
			s := strings.TrimSpace(comment)
			validTypes := map[string]bool{
				"int32": true, "uint32": true, "int64": true,
				"uint64": true, "float": true, "double": true,
			}
			if validTypes[s] {
				c.NumericType = &s
				found = true
			}
		case "minitems":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MinItems = &v
				found = true
			}
		case "maxitems":
			if v, err := strconv.Atoi(strings.TrimSpace(comment)); err == nil {
				c.MaxItems = &v
				found = true
			}
		case "uniqueitems":
			b := true
			c.UniqueItems = &b
			found = true
		case "default":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.Default = &s
				found = true
			}
		case "contentmediatype":
			s := strings.TrimSpace(comment)
			if s != "" {
				c.ContentMediaType = &s
				found = true
			}

		// --- Shorthand tags (Zod parity) ---
		case "positive":
			v := float64(0)
			c.ExclusiveMinimum = &v
			found = true
		case "negative":
			v := float64(0)
			c.ExclusiveMaximum = &v
			found = true
		case "nonnegative":
			v := float64(0)
			c.Minimum = &v
			found = true
		case "nonpositive":
			v := float64(0)
			c.Maximum = &v
			found = true
		case "int":
			s := "int32"
			c.NumericType = &s
			found = true
		case "safe":
			s := "int64"
			c.NumericType = &s
			found = true
		case "finite":
			s := "float"
			c.NumericType = &s
			found = true

		// --- String transform tags ---
		case "trim":
			c.Transforms = append(c.Transforms, "trim")
			found = true
		case "tolowercase":
			c.Transforms = append(c.Transforms, "toLowerCase")
			found = true
		case "touppercase":
			c.Transforms = append(c.Transforms, "toUpperCase")
			found = true

		// --- String content checks ---
		case "startswith":
			s := strings.TrimSpace(comment)
			// Remove surrounding quotes if present
			s = stripQuotes(s)
			if s != "" {
				c.StartsWith = &s
				found = true
			}
		case "endswith":
			s := strings.TrimSpace(comment)
			s = stripQuotes(s)
			if s != "" {
				c.EndsWith = &s
				found = true
			}
		case "includes":
			s := strings.TrimSpace(comment)
			s = stripQuotes(s)
			if s != "" {
				c.Includes = &s
				found = true
			}
		case "uppercase":
			b := true
			c.Uppercase = &b
			found = true
		case "lowercase":
			b := true
			c.Lowercase = &b
			found = true

		// --- Custom error messages ---
		case "error":
			s := strings.TrimSpace(comment)
			s = stripQuotes(s)
			if s != "" {
				c.ErrorMessage = &s
				found = true
			}

		// --- Coercion ---
		case "coerce":
			b := true
			c.Coerce = &b
			found = true
		}
	}

	if !found {
		return nil
	}
	return &c
}

// extractJSDocTagInfo safely extracts the tag name and comment from a JSDoc tag node.
// Custom tags (like @minimum, @maxLength) are KindJSDocTag (unknown tags), while
// known tags (like @param, @returns) have specific kinds.
func extractJSDocTagInfo(tagNode *ast.Node) (tagName string, comment string) {
	if tagNode == nil {
		return "", ""
	}

	// For unknown/custom tags (KindJSDocTag), use AsJSDocUnknownTag
	if tagNode.Kind == ast.KindJSDocTag {
		unknownTag := tagNode.AsJSDocUnknownTag()
		if unknownTag == nil || unknownTag.TagName == nil {
			return "", ""
		}
		tagName = unknownTag.TagName.Text()
		if unknownTag.Comment != nil {
			comment = extractNodeListText(unknownTag.Comment)
		}
		return tagName, comment
	}

	// For known tags that implement JSDocTagBase pattern
	// Try to access via the common tag structure
	// All JSDoc tags have a TagName child — we can search for it
	if tagNode.Kind == ast.KindJSDocDeprecatedTag {
		// @deprecated — not a constraint, skip
		return "deprecated", ""
	}

	// Handle @type tag — TypeScript recognizes it as KindJSDocTypeTag.
	// We extract the type name text (e.g., "int32") from the type expression.
	// When user writes `@type {int32}`, TS parses `int32` as a TypeReference inside JSDocTypeExpression.
	// When user writes `@type int32` (without braces), TS may parse differently.
	if tagNode.Kind == ast.KindJSDocTypeTag {
		typeTag := tagNode.AsJSDocTypeTag()
		if typeTag != nil {
			// Try to extract from comment first (when TS doesn't parse it as type expression)
			if typeTag.Comment != nil {
				text := extractNodeListText(typeTag.Comment)
				if text != "" {
					return "type", text
				}
			}
			// Try to extract from the type expression's inner type reference name
			if typeTag.TypeExpression != nil {
				typeExpr := typeTag.TypeExpression.AsJSDocTypeExpression()
				if typeExpr != nil && typeExpr.Type != nil {
					// The inner type should be a TypeReference with an identifier name
					if typeExpr.Type.Kind == ast.KindTypeReference {
						ref := typeExpr.Type.AsTypeReferenceNode()
						if ref != nil && ref.TypeName != nil {
							return "type", ref.TypeName.Text()
						}
					}
					// Fallback: try identifier directly
					if typeExpr.Type.Kind == ast.KindIdentifier {
						return "type", typeExpr.Type.Text()
					}
				}
			}
		}
		return "type", ""
	}

	// For other known tag kinds, try to get TagName from the first identifier child
	// This is a safe fallback
	return "", ""
}

// extractTypeLevelAnnotations parses JSDoc tags on a type declaration to extract
// type-level annotations like @strict, @passthrough, @strip, @tsgonest-ignore.
func (w *TypeWalker) extractTypeLevelAnnotations(node *ast.Node) (strictness string, ignore string) {
	if node == nil {
		return "", ""
	}

	jsdocs := node.JSDoc(nil)
	if len(jsdocs) == 0 {
		return "", ""
	}

	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()
	if jsdoc.Tags == nil {
		return "", ""
	}

	for _, tagNode := range jsdoc.Tags.Nodes {
		tagName, _ := extractJSDocTagInfo(tagNode)
		if tagName == "" {
			continue
		}
		tagName = strings.ToLower(tagName)

		switch tagName {
		case "strict":
			strictness = "strict"
		case "passthrough":
			strictness = "passthrough"
		case "strip":
			strictness = "strip"
		case "tsgonest-ignore":
			ignore = "all"
		case "tsgonest-ignore-validation":
			ignore = "validation"
		case "tsgonest-ignore-serialization":
			ignore = "serialization"
		}
	}
	return strictness, ignore
}

// stripQuotes removes surrounding double or single quotes from a string.
// e.g., `"hello"` → `hello`, `'world'` → `world`
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// extractNodeListText concatenates text from a NodeList of JSDoc text/link nodes.
func extractNodeListText(nodeList *ast.NodeList) string {
	if nodeList == nil {
		return ""
	}
	var parts []string
	for _, commentNode := range nodeList.Nodes {
		switch commentNode.Kind {
		case ast.KindJSDocText:
			parts = append(parts, commentNode.Text())
		case ast.KindJSDocLink, ast.KindJSDocLinkCode, ast.KindJSDocLinkPlain:
			parts = append(parts, commentNode.Text())
		}
	}
	return strings.Join(parts, "")
}
