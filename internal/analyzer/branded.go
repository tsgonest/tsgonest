package analyzer

import (
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

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
					if extractBrandedTagConstraint(&c, tagType) {
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

// extractBrandedTagConstraint extracts a constraint from a typia "typia.tag" property.
// Typia's tag structure: { target: "string"; kind: "format"; value: "email"; ... }
// We extract `kind` and `value` to map to our constraint system.
func extractBrandedTagConstraint(c *metadata.Constraints, tagType *metadata.Metadata) bool {
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
