package constraints

import (
	"fmt"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// SetBranded sets a constraint field from a branded type phantom property key
// and its literal value metadata. Returns true if a constraint was successfully set.
// This is the canonical implementation replacing analyzer.extractConstraintValue.
func SetBranded(c *metadata.Constraints, key string, typeMeta *metadata.Metadata) bool {
	switch FieldName(key) {
	// String constraints
	case FieldFormat:
		if s, ok := LiteralString(typeMeta); ok {
			c.Format = &s
			return true
		}
	case FieldMinLength:
		if n, ok := LiteralInt(typeMeta); ok {
			c.MinLength = &n
			return true
		}
	case FieldMaxLength:
		if n, ok := LiteralInt(typeMeta); ok {
			c.MaxLength = &n
			return true
		}
	case FieldPattern:
		if s, ok := LiteralString(typeMeta); ok {
			c.Pattern = &s
			return true
		}
	case FieldStartsWith:
		if s, ok := LiteralString(typeMeta); ok {
			c.StartsWith = &s
			return true
		}
	case FieldEndsWith:
		if s, ok := LiteralString(typeMeta); ok {
			c.EndsWith = &s
			return true
		}
	case FieldIncludes:
		if s, ok := LiteralString(typeMeta); ok {
			c.Includes = &s
			return true
		}

	// Numeric constraints
	case FieldMinimum:
		if f, ok := LiteralFloat(typeMeta); ok {
			c.Minimum = &f
			return true
		}
	case FieldMaximum:
		if f, ok := LiteralFloat(typeMeta); ok {
			c.Maximum = &f
			return true
		}
	case FieldExclusiveMinimum:
		if f, ok := LiteralFloat(typeMeta); ok {
			c.ExclusiveMinimum = &f
			return true
		}
	case FieldExclusiveMaximum:
		if f, ok := LiteralFloat(typeMeta); ok {
			c.ExclusiveMaximum = &f
			return true
		}
	case FieldMultipleOf:
		if f, ok := LiteralFloat(typeMeta); ok {
			c.MultipleOf = &f
			return true
		}
	case FieldNumericType:
		if s, ok := LiteralString(typeMeta); ok {
			c.NumericType = &s
			return true
		}

	// Array constraints
	case FieldMinItems:
		if n, ok := LiteralInt(typeMeta); ok {
			c.MinItems = &n
			return true
		}
	case FieldMaxItems:
		if n, ok := LiteralInt(typeMeta); ok {
			c.MaxItems = &n
			return true
		}
	case FieldUniqueItems:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.UniqueItems = &b
			return true
		}

	// File upload constraints
	case FieldMaxSize:
		if f, ok := LiteralFloat(typeMeta); ok && f >= 0 {
			n := uint64(f)
			c.MaxSize = &n
			return true
		}
	case FieldMinSize:
		if f, ok := LiteralFloat(typeMeta); ok && f >= 0 {
			n := uint64(f)
			c.MinSize = &n
			return true
		}
	case FieldMimeTypes:
		mimes := collectStringLiterals(typeMeta)
		if len(mimes) > 0 {
			c.MimeTypes = mimes
			return true
		}

	// String case validation
	case FieldUppercase:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.Uppercase = &b
			return true
		}
	case FieldLowercase:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.Lowercase = &b
			return true
		}

	// Transforms
	case FieldTransformTrim:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, string(TransformTrim))
			return true
		}
	case FieldTransformToLowerCase:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, string(TransformToLowerCase))
			return true
		}
	case FieldTransformToUpperCase:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.Transforms = append(c.Transforms, string(TransformToUpperCase))
			return true
		}

	// Custom error message
	case FieldError:
		if s, ok := LiteralString(typeMeta); ok {
			c.ErrorMessage = &s
			return true
		}

	// Default value
	case FieldDefault:
		if s, ok := LiteralString(typeMeta); ok {
			c.Default = &s
			return true
		}
		// Also support numeric/boolean defaults as string representation
		if f, ok := LiteralFloat(typeMeta); ok {
			s := fmt.Sprintf("%v", f)
			c.Default = &s
			return true
		}
		if b, ok := LiteralBool(typeMeta); ok {
			s := fmt.Sprintf("%v", b)
			c.Default = &s
			return true
		}

	// Coercion
	case FieldCoerce:
		if b, ok := LiteralBool(typeMeta); ok && b {
			c.Coerce = &b
			return true
		}
	}
	return false
}
