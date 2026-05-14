// Package constraints provides the canonical constraint model for tsgonest.
// It defines typed constants for all constraint names, transform names,
// numeric type names, and format names, eliminating bare string literals
// scattered across analyzer, codegen, and openapi packages.
package constraints

// FieldName identifies a constraint field on metadata.Constraints.
type FieldName string

const (
	// String constraints
	FieldFormat     FieldName = "format"
	FieldMinLength  FieldName = "minLength"
	FieldMaxLength  FieldName = "maxLength"
	FieldPattern    FieldName = "pattern"
	FieldStartsWith FieldName = "startsWith"
	FieldEndsWith   FieldName = "endsWith"
	FieldIncludes   FieldName = "includes"

	// Numeric constraints
	FieldMinimum          FieldName = "minimum"
	FieldMaximum          FieldName = "maximum"
	FieldExclusiveMinimum FieldName = "exclusiveMinimum"
	FieldExclusiveMaximum FieldName = "exclusiveMaximum"
	FieldMultipleOf       FieldName = "multipleOf"
	FieldNumericType      FieldName = "type"

	// Array constraints
	FieldMinItems    FieldName = "minItems"
	FieldMaxItems    FieldName = "maxItems"
	FieldUniqueItems FieldName = "uniqueItems"

	// File upload constraints
	FieldMaxSize   FieldName = "maxSize"
	FieldMinSize   FieldName = "minSize"
	FieldMimeTypes FieldName = "mimeTypes"

	// String case validation
	FieldUppercase FieldName = "uppercase"
	FieldLowercase FieldName = "lowercase"

	// Transforms (branded type keys use "transform_" prefix)
	FieldTransformTrim        FieldName = "transform_trim"
	FieldTransformToLowerCase FieldName = "transform_toLowerCase"
	FieldTransformToUpperCase FieldName = "transform_toUpperCase"

	// Meta
	FieldError   FieldName = "error"
	FieldDefault FieldName = "default"
	FieldCoerce  FieldName = "coerce"

	// Custom validator
	FieldValidate FieldName = "validate"

	// Schema-only
	FieldContentMediaType FieldName = "contentMediaType"
)

// Transform identifies a string transform applied before validation.
type Transform string

const (
	TransformTrim        Transform = "trim"
	TransformToLowerCase Transform = "toLowerCase"
	TransformToUpperCase Transform = "toUpperCase"
)

// NumericType identifies a numeric subtype constraint.
type NumericType string

const (
	NumericInt32  NumericType = "int32"
	NumericUint32 NumericType = "uint32"
	NumericInt64  NumericType = "int64"
	NumericUint64 NumericType = "uint64"
	NumericFloat  NumericType = "float"
	NumericDouble NumericType = "double"
)

// ValidNumericTypes is the set of recognized numeric type constraint values.
var ValidNumericTypes = map[string]bool{
	string(NumericInt32):  true,
	string(NumericUint32): true,
	string(NumericInt64):  true,
	string(NumericUint64): true,
	string(NumericFloat):  true,
	string(NumericDouble): true,
}
