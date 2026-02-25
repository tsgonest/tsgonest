// Package metadata defines the type metadata schema used throughout tsgonest.
// This is the Go equivalent of typia's Metadata — a normalized representation
// of TypeScript types suitable for code generation.
package metadata

// Metadata represents the full type information for a TypeScript type.
type Metadata struct {
	// Kind identifies the primary kind of the type.
	Kind Kind `json:"kind"`

	// Nullable is true if the type includes null.
	Nullable bool `json:"nullable,omitempty"`

	// Optional is true if the type includes undefined (or is a property flagged optional).
	Optional bool `json:"optional,omitempty"`

	// Name is the type's name (e.g., "CreateUserDto", "string").
	Name string `json:"name,omitempty"`

	// Atomic holds the specific atomic type (string, number, boolean, bigint).
	// Only set when Kind == KindAtomic.
	Atomic string `json:"atomic,omitempty"`

	// TemplatePattern holds a regex pattern for template literal types.
	// Only set when Kind == KindAtomic and the type was a template literal.
	// e.g., `prefix_${string}` → "^prefix_.*$"
	TemplatePattern string `json:"templatePattern,omitempty"`

	// LiteralValue holds the literal value for KindLiteral types.
	LiteralValue any `json:"literalValue,omitempty"`

	// Properties holds the properties of an object type.
	// Only set when Kind == KindObject.
	Properties []Property `json:"properties,omitempty"`

	// ElementType holds the element type for arrays.
	// Only set when Kind == KindArray.
	ElementType *Metadata `json:"elementType,omitempty"`

	// Elements holds the element types for tuples.
	// Only set when Kind == KindTuple.
	Elements []TupleElement `json:"elements,omitempty"`

	// UnionMembers holds the member types for unions.
	// Only set when Kind == KindUnion.
	UnionMembers []Metadata `json:"unionMembers,omitempty"`

	// Discriminant identifies the discriminant property for discriminated unions.
	// Only set when Kind == KindUnion and all members share a common property with unique literal values.
	Discriminant *Discriminant `json:"discriminant,omitempty"`

	// IntersectionMembers holds the member types for intersections.
	// Only set when Kind == KindIntersection.
	IntersectionMembers []Metadata `json:"intersectionMembers,omitempty"`

	// EnumValues holds the enum member values.
	// Only set when Kind == KindEnum.
	EnumValues []EnumValue `json:"enumValues,omitempty"`

	// NativeType names the native class (e.g., "Date", "RegExp", "Map", "Set").
	// Only set when Kind == KindNative.
	NativeType string `json:"nativeType,omitempty"`

	// TypeArguments holds generic type arguments (e.g., for Map<K,V> or Set<T>).
	TypeArguments []Metadata `json:"typeArguments,omitempty"`

	// IndexSignature holds index signature info for objects with dynamic keys.
	IndexSignature *IndexSignature `json:"indexSignature,omitempty"`

	// Ref is used for recursive types — a reference to a named type defined elsewhere.
	Ref string `json:"$ref,omitempty"`

	// Strictness controls how unknown properties are handled during validation.
	// Only meaningful for KindObject. Values: "", "strict", "passthrough", "strip".
	Strictness string `json:"strictness,omitempty"`

	// Ignore controls codegen exclusion. Values:
	// "all" = skip all companion generation
	// "validation" = skip validation only
	// "serialization" = skip serialization only
	Ignore string `json:"ignore,omitempty"`

	// Constraints holds validation constraints extracted from branded phantom types
	// (e.g., `string & tags.Format<"email">`). These are merged with JSDoc constraints
	// at the property level during object analysis. Only set on atomic types returned
	// from branded type detection.
	Constraints *Constraints `json:"constraints,omitempty"`
}

// Kind represents the primary classification of a type.
type Kind string

const (
	KindAny          Kind = "any"
	KindUnknown      Kind = "unknown"
	KindNever        Kind = "never"
	KindVoid         Kind = "void"
	KindAtomic       Kind = "atomic"       // string, number, boolean, bigint
	KindLiteral      Kind = "literal"      // string literal, number literal, boolean literal
	KindObject       Kind = "object"       // interface, type alias with properties
	KindArray        Kind = "array"        // T[]
	KindTuple        Kind = "tuple"        // [A, B, C]
	KindUnion        Kind = "union"        // A | B
	KindIntersection Kind = "intersection" // A & B
	KindEnum         Kind = "enum"         // enum values
	KindNative       Kind = "native"       // Date, RegExp, Map, Set, etc.
	KindRef          Kind = "ref"          // reference to a named type
)

// Property represents a property in an object type.
type Property struct {
	Name          string       `json:"name"`
	Type          Metadata     `json:"type"`
	Required      bool         `json:"required"`
	Readonly      bool         `json:"readonly,omitempty"`
	ExactOptional bool         `json:"exactOptional,omitempty"`
	Constraints   *Constraints `json:"constraints,omitempty"`
	// Description is from @description JSDoc on the property declaration.
	Description string `json:"description,omitempty"`
	// WriteOnly is from @writeOnly JSDoc on the property declaration.
	WriteOnly bool `json:"writeOnly,omitempty"`
	// Example is from @example JSDoc on the property declaration.
	Example *string `json:"example,omitempty"`
}

// Constraints represents validation constraints extracted from JSDoc tags.
type Constraints struct {
	// Numeric constraints
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`

	// Numeric type constraint (int32, uint32, int64, uint64, float, double)
	NumericType *string `json:"numericType,omitempty"`

	// String length constraints
	MinLength *int `json:"minLength,omitempty"`
	MaxLength *int `json:"maxLength,omitempty"`

	// String format/pattern constraints
	Pattern *string `json:"pattern,omitempty"`
	Format  *string `json:"format,omitempty"` // "email", "url", "uuid", "date-time", etc. (32+ formats)

	// String content checks (Zod parity)
	StartsWith *string `json:"startsWith,omitempty"`
	EndsWith   *string `json:"endsWith,omitempty"`
	Includes   *string `json:"includes,omitempty"`
	Uppercase  *bool   `json:"uppercase,omitempty"` // validate: must be all uppercase
	Lowercase  *bool   `json:"lowercase,omitempty"` // validate: must be all lowercase

	// String content type (schema-only, no runtime validation)
	ContentMediaType *string `json:"contentMediaType,omitempty"`

	// String/number transforms (applied before validation)
	Transforms []string `json:"transforms,omitempty"` // "trim", "toLowerCase", "toUpperCase"

	// Array constraints
	MinItems    *int  `json:"minItems,omitempty"`
	MaxItems    *int  `json:"maxItems,omitempty"`
	UniqueItems *bool `json:"uniqueItems,omitempty"`

	// Schema-only (no runtime validation)
	Default *string `json:"default,omitempty"`

	// Coercion: when true, string inputs are coerced to their declared type
	// before validation. Handles string→number, string→boolean, string→Date.
	// Useful for query params and path params that arrive as strings.
	Coerce *bool `json:"coerce,omitempty"`

	// Custom validator function reference.
	// ValidateFn is the exported function name (e.g., "isValidCard").
	// ValidateModule is the source file path (e.g., "./validators/credit-card").
	// Used with Validate<typeof fn> branded type.
	ValidateFn     *string `json:"validateFn,omitempty"`
	ValidateModule *string `json:"validateModule,omitempty"`

	// Custom error message (global fallback for all checks on this property)
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Per-constraint error messages. Maps constraint key (e.g., "format", "minLength")
	// to a custom error string. Takes precedence over ErrorMessage for matching checks.
	// Populated from __tsgonest_<constraint>_error phantom properties.
	Errors map[string]string `json:"errors,omitempty"`
}

// Discriminant describes the discriminant property of a discriminated union.
// The property has a unique literal value in each union member.
type Discriminant struct {
	// Property is the name of the discriminant property (e.g., "type", "kind").
	Property string `json:"property"`
	// Mapping maps literal values to union member indices.
	// e.g., {"card": 0, "bank": 1, "crypto": 2}
	Mapping map[string]int `json:"mapping"`
}

// TupleElement represents an element in a tuple type.
type TupleElement struct {
	Type     Metadata `json:"type"`
	Optional bool     `json:"optional,omitempty"`
	Rest     bool     `json:"rest,omitempty"`
	Label    string   `json:"label,omitempty"`
}

// EnumValue represents a single enum member value.
type EnumValue struct {
	Name  string `json:"name"`
	Value any    `json:"value"` // string or number
}

// IndexSignature represents an index signature like [key: string]: T.
type IndexSignature struct {
	KeyType   Metadata `json:"keyType"`
	ValueType Metadata `json:"valueType"`
}

// TypeRegistry tracks named types to support $ref and prevent infinite recursion.
type TypeRegistry struct {
	Types map[string]*Metadata
}

// NewTypeRegistry creates an empty type registry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{Types: make(map[string]*Metadata)}
}

// Register adds a named type to the registry.
func (r *TypeRegistry) Register(name string, m *Metadata) {
	r.Types[name] = m
}

// Has checks if a named type is already registered.
func (r *TypeRegistry) Has(name string) bool {
	_, ok := r.Types[name]
	return ok
}
