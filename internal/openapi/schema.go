// Package openapi generates OpenAPI 3.1 documents from NestJS controller analysis.
package openapi

import (
	"encoding/json"
	"regexp"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// SchemaOrBool represents a value that can be either a Schema object or a boolean.
// In OpenAPI 3.1, additionalProperties can be either a schema or false.
type SchemaOrBool struct {
	Schema *Schema
	Bool   *bool
}

// MarshalJSON implements json.Marshaler.
func (s SchemaOrBool) MarshalJSON() ([]byte, error) {
	if s.Bool != nil {
		return json.Marshal(*s.Bool)
	}
	if s.Schema != nil {
		return json.Marshal(s.Schema)
	}
	return []byte("{}"), nil
}

// Schema represents a JSON Schema (OpenAPI 3.1 compatible).
type Schema struct {
	Type          string             `json:"type,omitempty"`
	Format        string             `json:"format,omitempty"`
	Properties    map[string]*Schema `json:"properties,omitempty"`
	Required      []string           `json:"required,omitempty"`
	Items         *Schema            `json:"items,omitempty"`
	PrefixItems   []*Schema          `json:"prefixItems,omitempty"`
	Enum          []any              `json:"enum,omitempty"`
	Const         any                `json:"const,omitempty"`
	AnyOf         []*Schema          `json:"anyOf,omitempty"`
	OneOf         []*Schema          `json:"oneOf,omitempty"`
	AllOf         []*Schema          `json:"allOf,omitempty"`
	Ref           string             `json:"$ref,omitempty"`
	Nullable      bool               `json:"-"` // handled by adding null to anyOf
	Description   string             `json:"description,omitempty"`
	Discriminator *Discriminator     `json:"discriminator,omitempty"`

	// Additional schema properties
	AdditionalProperties *SchemaOrBool `json:"additionalProperties,omitempty"`

	// Validation constraints (from JSDoc tags)
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`
	MinLength        *int     `json:"minLength,omitempty"`
	MaxLength        *int     `json:"maxLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty"`
	MinItems         *int     `json:"minItems,omitempty"`
	MaxItems         *int     `json:"maxItems,omitempty"`
	UniqueItems      *bool    `json:"uniqueItems,omitempty"`
	Default          any      `json:"default,omitempty"`
	ContentMediaType string   `json:"contentMediaType,omitempty"`
	ContentSchema    *Schema  `json:"contentSchema,omitempty"` // JSON Schema for content-encoded data (e.g., SSE data field)
	ReadOnly         *bool    `json:"readOnly,omitempty"`
	WriteOnly        *bool    `json:"writeOnly,omitempty"`
	Example          *string  `json:"example,omitempty"`
}

// Discriminator represents an OpenAPI discriminator object for discriminated unions.
type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

// SchemaGenerator converts type metadata into JSON Schema objects.
type SchemaGenerator struct {
	// schemas collects all named schemas for components/schemas.
	schemas map[string]*Schema
	// registry holds the type metadata registry for resolving $ref types.
	registry *metadata.TypeRegistry
}

// NewSchemaGenerator creates a new schema generator.
func NewSchemaGenerator(registry *metadata.TypeRegistry) *SchemaGenerator {
	return &SchemaGenerator{
		schemas:  make(map[string]*Schema),
		registry: registry,
	}
}

// Schemas returns all collected named schemas.
func (g *SchemaGenerator) Schemas() map[string]*Schema {
	return g.schemas
}

// MetadataToSchema converts a Metadata into a JSON Schema.
// Named types are registered as components/schemas and referenced via $ref.
func (g *SchemaGenerator) MetadataToSchema(m *metadata.Metadata) *Schema {
	schema := g.convertType(m)

	// Handle nullable: wrap in anyOf with null
	if m.Nullable {
		schema = wrapNullable(schema)
	}

	return schema
}

// convertType handles the core type conversion.
func (g *SchemaGenerator) convertType(m *metadata.Metadata) *Schema {
	switch m.Kind {
	case metadata.KindAtomic:
		return g.convertAtomic(m)
	case metadata.KindLiteral:
		return g.convertLiteral(m)
	case metadata.KindObject:
		return g.convertObject(m)
	case metadata.KindArray:
		return g.convertArray(m)
	case metadata.KindTuple:
		return g.convertTuple(m)
	case metadata.KindUnion:
		return g.convertUnion(m)
	case metadata.KindIntersection:
		return g.convertIntersection(m)
	case metadata.KindEnum:
		return g.convertEnum(m)
	case metadata.KindNative:
		return g.convertNative(m)
	case metadata.KindRef:
		return g.convertRef(m)
	case metadata.KindVoid:
		// void maps to no content (empty object in JSON Schema context)
		return &Schema{}
	case metadata.KindAny, metadata.KindUnknown:
		return &Schema{} // empty schema (accepts anything)
	case metadata.KindNever:
		// never type — no valid values
		return &Schema{Type: "object", Description: "never (no valid values)"}
	default:
		return &Schema{}
	}
}

// convertAtomic converts atomic types (string, number, boolean, bigint).
func (g *SchemaGenerator) convertAtomic(m *metadata.Metadata) *Schema {
	switch m.Atomic {
	case "string":
		return &Schema{Type: "string"}
	case "number":
		return &Schema{Type: "number"}
	case "boolean":
		return &Schema{Type: "boolean"}
	case "bigint":
		return &Schema{Type: "integer", Format: "int64"}
	case "null":
		return &Schema{Type: "null"}
	case "undefined":
		// undefined doesn't have a direct JSON Schema equivalent
		return &Schema{}
	default:
		return &Schema{Type: "string"}
	}
}

// convertLiteral converts literal types to const values.
func (g *SchemaGenerator) convertLiteral(m *metadata.Metadata) *Schema {
	return &Schema{Const: m.LiteralValue}
}

// convertObject converts an object type to a JSON Schema with properties.
// Named objects are registered as components and referenced via $ref.
func (g *SchemaGenerator) convertObject(m *metadata.Metadata) *Schema {
	// If this is a named type, register it and return a $ref
	if m.Name != "" {
		if _, exists := g.schemas[m.Name]; !exists {
			// Register a placeholder first to handle recursion
			g.schemas[m.Name] = &Schema{}
			schema := g.buildObjectSchema(m)
			g.schemas[m.Name] = schema
		}
		return &Schema{Ref: "#/components/schemas/" + m.Name}
	}

	// Anonymous inline object
	return g.buildObjectSchema(m)
}

// buildObjectSchema builds the actual schema for an object type.
func (g *SchemaGenerator) buildObjectSchema(m *metadata.Metadata) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	var requiredProps []string
	for _, prop := range m.Properties {
		propSchema := g.MetadataToSchema(&prop.Type)

		// Apply JSDoc constraints to the property schema
		if prop.Constraints != nil {
			applyConstraints(propSchema, prop.Constraints)
		}

		// Apply property annotations from JSDoc
		if prop.Description != "" {
			propSchema.Description = prop.Description
		}
		if prop.Example != nil {
			propSchema.Example = prop.Example
		}
		schema.Properties[prop.Name] = propSchema
		if prop.Readonly {
			t := true
			propSchema.ReadOnly = &t
		}
		if prop.WriteOnly {
			t := true
			propSchema.WriteOnly = &t
		}

		if prop.Required {
			requiredProps = append(requiredProps, prop.Name)
		}
	}

	if len(requiredProps) > 0 {
		schema.Required = requiredProps
	}

	// Handle index signatures
	if m.IndexSignature != nil {
		valSchema := g.MetadataToSchema(&m.IndexSignature.ValueType)
		schema.AdditionalProperties = &SchemaOrBool{Schema: valSchema}
	}

	// @strict → additionalProperties: false
	if m.Strictness == "strict" && schema.AdditionalProperties == nil {
		f := false
		schema.AdditionalProperties = &SchemaOrBool{Bool: &f}
	}

	return schema
}

// convertArray converts an array type.
func (g *SchemaGenerator) convertArray(m *metadata.Metadata) *Schema {
	if m.ElementType == nil {
		return &Schema{Type: "array"}
	}
	return &Schema{
		Type:  "array",
		Items: g.MetadataToSchema(m.ElementType),
	}
}

// convertTuple converts a tuple type to JSON Schema using prefixItems.
func (g *SchemaGenerator) convertTuple(m *metadata.Metadata) *Schema {
	schema := &Schema{Type: "array"}

	var prefixItems []*Schema
	for _, elem := range m.Elements {
		prefixItems = append(prefixItems, g.MetadataToSchema(&elem.Type))
	}

	if len(prefixItems) > 0 {
		schema.PrefixItems = prefixItems
		// In JSON Schema, items: false means no additional items beyond prefixItems
		minLen := len(prefixItems)
		schema.MinItems = &minLen
		schema.MaxItems = &minLen
	}

	return schema
}

// convertUnion converts a union type.
func (g *SchemaGenerator) convertUnion(m *metadata.Metadata) *Schema {
	if len(m.UnionMembers) == 0 {
		return &Schema{}
	}

	// Check if all members are literals — use enum
	allLiterals := true
	var enumValues []any
	for _, member := range m.UnionMembers {
		if member.Kind != metadata.KindLiteral {
			allLiterals = false
			break
		}
		enumValues = append(enumValues, member.LiteralValue)
	}
	if allLiterals && len(enumValues) > 0 {
		// Named literal union (e.g., Prisma enum type alias or TS enum) → register as $ref
		if m.Name != "" {
			if _, exists := g.schemas[m.Name]; !exists {
				g.schemas[m.Name] = &Schema{Enum: enumValues}
			}
			return &Schema{Ref: "#/components/schemas/" + m.Name}
		}
		return &Schema{Enum: enumValues}
	}

	// Named complex union (e.g., type Result = SuccessDto | ErrorDto) → register as $ref
	if m.Name != "" {
		if _, exists := g.schemas[m.Name]; !exists {
			g.schemas[m.Name] = &Schema{} // placeholder for recursion
			schema := g.buildUnionSchema(m)
			g.schemas[m.Name] = schema
		}
		return &Schema{Ref: "#/components/schemas/" + m.Name}
	}

	return g.buildUnionSchema(m)
}

// buildUnionSchema builds the actual schema for a union type (discriminated or general).
func (g *SchemaGenerator) buildUnionSchema(m *metadata.Metadata) *Schema {
	// Check for discriminated union
	if m.Discriminant != nil && m.Discriminant.Property != "" {
		var schemas []*Schema
		mapping := make(map[string]string)
		for _, member := range m.UnionMembers {
			memberSchema := g.MetadataToSchema(&member)
			schemas = append(schemas, memberSchema)
			// Build mapping from discriminant values to $ref
			if memberSchema.Ref != "" {
				for val, idx := range m.Discriminant.Mapping {
					// Find which member this index corresponds to
					if idx >= 0 && idx < len(m.UnionMembers) {
						target := m.UnionMembers[idx]
						if target.Ref == member.Ref && target.Name == member.Name {
							mapping[val] = memberSchema.Ref
						}
					}
				}
			}
		}
		schema := &Schema{OneOf: schemas}
		if len(mapping) > 0 {
			schema.Discriminator = &Discriminator{
				PropertyName: m.Discriminant.Property,
				Mapping:      mapping,
			}
		} else {
			schema.Discriminator = &Discriminator{
				PropertyName: m.Discriminant.Property,
			}
		}
		return schema
	}

	// General union — use anyOf
	var schemas []*Schema
	for _, member := range m.UnionMembers {
		schemas = append(schemas, g.MetadataToSchema(&member))
	}

	return &Schema{AnyOf: schemas}
}

// convertIntersection converts an intersection type using allOf.
// Named intersections are registered as components/schemas and referenced via $ref.
func (g *SchemaGenerator) convertIntersection(m *metadata.Metadata) *Schema {
	if len(m.IntersectionMembers) == 0 {
		return &Schema{}
	}

	// Named intersection → register and return $ref
	if m.Name != "" {
		if _, exists := g.schemas[m.Name]; !exists {
			g.schemas[m.Name] = &Schema{} // placeholder for recursion
			var schemas []*Schema
			for _, member := range m.IntersectionMembers {
				schemas = append(schemas, g.MetadataToSchema(&member))
			}
			g.schemas[m.Name] = &Schema{AllOf: schemas}
		}
		return &Schema{Ref: "#/components/schemas/" + m.Name}
	}

	// Anonymous intersection
	var schemas []*Schema
	for _, member := range m.IntersectionMembers {
		schemas = append(schemas, g.MetadataToSchema(&member))
	}

	return &Schema{AllOf: schemas}
}

// convertEnum converts an enum type to a schema with enum values.
// Named enums are registered as components/schemas and referenced via $ref.
func (g *SchemaGenerator) convertEnum(m *metadata.Metadata) *Schema {
	var values []any
	for _, ev := range m.EnumValues {
		values = append(values, ev.Value)
	}
	// Named enum → register as $ref for deduplication
	if m.Name != "" {
		if _, exists := g.schemas[m.Name]; !exists {
			g.schemas[m.Name] = &Schema{Enum: values}
		}
		return &Schema{Ref: "#/components/schemas/" + m.Name}
	}
	return &Schema{Enum: values}
}

// convertNative converts native types to JSON Schema equivalents.
func (g *SchemaGenerator) convertNative(m *metadata.Metadata) *Schema {
	switch m.NativeType {
	case "Date":
		return &Schema{Type: "string", Format: "date-time"}
	case "RegExp":
		return &Schema{Type: "string", Format: "regex"}
	case "URL":
		return &Schema{Type: "string", Format: "uri"}
	case "Map":
		// Map<K,V> → object with additionalProperties
		schema := &Schema{Type: "object"}
		if len(m.TypeArguments) >= 2 {
			schema.AdditionalProperties = &SchemaOrBool{Schema: g.MetadataToSchema(&m.TypeArguments[1])}
		}
		return schema
	case "Set":
		// Set<T> → array with unique items
		schema := &Schema{Type: "array"}
		if len(m.TypeArguments) >= 1 {
			schema.Items = g.MetadataToSchema(&m.TypeArguments[0])
		}
		return schema
	case "Uint8Array", "Int8Array", "Uint16Array", "Int16Array",
		"Uint32Array", "Int32Array", "Float32Array", "Float64Array":
		return &Schema{Type: "array", Items: &Schema{Type: "number"}}
	case "ArrayBuffer", "SharedArrayBuffer":
		return &Schema{Type: "string", Format: "binary"}
	case "Error":
		return &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"message": {Type: "string"},
				"name":    {Type: "string"},
			},
		}
	default:
		return &Schema{Type: "object"}
	}
}

// convertRef converts a $ref type to a JSON Schema $ref.
func (g *SchemaGenerator) convertRef(m *metadata.Metadata) *Schema {
	refName := m.Ref
	if refName == "" {
		return &Schema{}
	}

	// Resolve through registry and register if needed
	if regType, ok := g.registry.Types[refName]; ok {
		if _, exists := g.schemas[refName]; !exists {
			// Register placeholder for recursion protection
			g.schemas[refName] = &Schema{}
			// Build the schema based on the registered type's Kind.
			// We call buildRefSchema instead of convertType to avoid
			// re-entering convertObject/convertUnion/convertIntersection
			// which would see the placeholder and short-circuit.
			schema := g.buildRefSchema(regType)
			g.schemas[refName] = schema
		}
	}

	return &Schema{Ref: "#/components/schemas/" + refName}
}

// buildRefSchema builds a schema for a registry type, dispatching by Kind.
// Unlike convertType, this does not re-enter the named-type registration logic.
func (g *SchemaGenerator) buildRefSchema(m *metadata.Metadata) *Schema {
	switch m.Kind {
	case metadata.KindObject:
		return g.buildObjectSchema(m)
	case metadata.KindUnion:
		return g.buildUnionSchema(m)
	case metadata.KindIntersection:
		if len(m.IntersectionMembers) == 0 {
			return &Schema{}
		}
		var schemas []*Schema
		for _, member := range m.IntersectionMembers {
			schemas = append(schemas, g.MetadataToSchema(&member))
		}
		return &Schema{AllOf: schemas}
	case metadata.KindArray:
		return g.convertArray(m)
	case metadata.KindEnum:
		var values []any
		for _, ev := range m.EnumValues {
			values = append(values, ev.Value)
		}
		return &Schema{Enum: values}
	default:
		return g.convertType(m)
	}
}

// applyConstraints applies JSDoc constraints to a property schema.
func applyConstraints(schema *Schema, c *metadata.Constraints) {
	if c.Minimum != nil {
		schema.Minimum = c.Minimum
	}
	if c.Maximum != nil {
		schema.Maximum = c.Maximum
	}
	if c.ExclusiveMinimum != nil {
		schema.ExclusiveMinimum = c.ExclusiveMinimum
	}
	if c.ExclusiveMaximum != nil {
		schema.ExclusiveMaximum = c.ExclusiveMaximum
	}
	if c.MultipleOf != nil {
		schema.MultipleOf = c.MultipleOf
	}
	if c.MinLength != nil {
		schema.MinLength = c.MinLength
	}
	if c.MaxLength != nil {
		schema.MaxLength = c.MaxLength
	}
	if c.Pattern != nil {
		schema.Pattern = *c.Pattern
	}
	if c.Format != nil {
		schema.Format = *c.Format
	}
	if c.MinItems != nil {
		schema.MinItems = c.MinItems
	}
	if c.MaxItems != nil {
		schema.MaxItems = c.MaxItems
	}
	if c.UniqueItems != nil {
		schema.UniqueItems = c.UniqueItems
	}
	if c.Default != nil {
		schema.Default = *c.Default
	}
	if c.ContentMediaType != nil {
		schema.ContentMediaType = *c.ContentMediaType
	}
	// String content checks map to pattern or x-extensions in OpenAPI.
	// startsWith/endsWith/includes → pattern (best approximation).
	// Note: these override any existing pattern. If @pattern is also set, it takes precedence above.
	if c.StartsWith != nil && c.Pattern == nil {
		p := "^" + regexp.QuoteMeta(*c.StartsWith)
		schema.Pattern = p
	}
	if c.EndsWith != nil && c.Pattern == nil && c.StartsWith == nil {
		p := regexp.QuoteMeta(*c.EndsWith) + "$"
		schema.Pattern = p
	}
}

// wrapNullable wraps a schema to also allow null.
func wrapNullable(schema *Schema) *Schema {
	// If it's a $ref, use anyOf
	if schema.Ref != "" {
		return &Schema{
			AnyOf: []*Schema{
				schema,
				{Type: "null"},
			},
		}
	}

	// For simple types, use anyOf with null
	return &Schema{
		AnyOf: []*Schema{
			schema,
			{Type: "null"},
		},
	}
}
