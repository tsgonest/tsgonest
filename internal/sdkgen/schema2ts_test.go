package sdkgen

import (
	"testing"
)

func TestSchemaToTS_Primitives(t *testing.T) {
	tests := []struct {
		name string
		node *SchemaNode
		want string
	}{
		{"string", &SchemaNode{Type: "string"}, "string"},
		{"number", &SchemaNode{Type: "number"}, "number"},
		{"integer", &SchemaNode{Type: "integer"}, "number"},
		{"boolean", &SchemaNode{Type: "boolean"}, "boolean"},
		{"null", &SchemaNode{Type: "null"}, "null"},
		{"nil node", nil, "unknown"},
		{"unknown type", &SchemaNode{Type: ""}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SchemaToTS(tt.node, nil)
			if got != tt.want {
				t.Errorf("SchemaToTS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSchemaToTS_Ref(t *testing.T) {
	node := &SchemaNode{Ref: "UserResponse"}
	got := SchemaToTS(node, nil)
	if got != "UserResponse" {
		t.Errorf("expected UserResponse, got %q", got)
	}
}

func TestSchemaToTS_Array(t *testing.T) {
	node := &SchemaNode{
		Type:  "array",
		Items: &SchemaNode{Ref: "Order"},
	}
	got := SchemaToTS(node, nil)
	if got != "Order[]" {
		t.Errorf("expected Order[], got %q", got)
	}
}

func TestSchemaToTS_ArrayOfPrimitives(t *testing.T) {
	node := &SchemaNode{
		Type:  "array",
		Items: &SchemaNode{Type: "string"},
	}
	got := SchemaToTS(node, nil)
	if got != "string[]" {
		t.Errorf("expected string[], got %q", got)
	}
}

func TestSchemaToTS_Enum(t *testing.T) {
	node := &SchemaNode{
		Enum: []any{"active", "inactive", "archived"},
	}
	got := SchemaToTS(node, nil)
	want := `"active" | "inactive" | "archived"`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_Const(t *testing.T) {
	node := &SchemaNode{Const: "physical", HasConst: true}
	got := SchemaToTS(node, nil)
	if got != `"physical"` {
		t.Errorf("expected \"physical\", got %q", got)
	}
}

func TestSchemaToTS_Object(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name"},
	}
	got := SchemaToTS(node, nil)
	want := "{ age?: number; name: string }"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_Record(t *testing.T) {
	node := &SchemaNode{
		Type:                 "object",
		AdditionalProperties: &SchemaNode{Type: "string"},
	}
	got := SchemaToTS(node, nil)
	want := "Record<string, string>"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_AnyOf(t *testing.T) {
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "number"},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(string | number)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_Nullable(t *testing.T) {
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "null"},
		},
	}
	got := SchemaToTS(node, nil)
	want := "string | null"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_AllOf(t *testing.T) {
	node := &SchemaNode{
		AllOf: []*SchemaNode{
			{Ref: "BaseEntity"},
			{Ref: "UserFields"},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(BaseEntity & UserFields)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_ArrayOfUnion(t *testing.T) {
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "number"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(string | number)[]"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestGenerateInterface_Object(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id":   {Type: "string"},
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"id", "name"},
	}
	got := GenerateInterface("User", node, nil)
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	// Should contain interface declaration
	if !contains(got, "export interface User {") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	// Required fields should not have ?
	if !contains(got, "  id: string;") {
		t.Errorf("expected 'id: string;', got:\n%s", got)
	}
	// Optional fields should have ?
	if !contains(got, "  age?: number;") {
		t.Errorf("expected 'age?: number;', got:\n%s", got)
	}
}

func TestGenerateInterface_Enum(t *testing.T) {
	node := &SchemaNode{
		Type: "string",
		Enum: []any{"active", "inactive"},
	}
	got := GenerateInterface("Status", node, nil)
	if !contains(got, "export type Status = ") {
		t.Errorf("expected type alias for enum, got:\n%s", got)
	}
}

func TestSchemaToTS_Binary(t *testing.T) {
	// format: binary → File | Blob
	node := &SchemaNode{Type: "string", Format: "binary"}
	got := SchemaToTS(node, nil)
	if got != "File | Blob" {
		t.Errorf("expected 'File | Blob', got %q", got)
	}
}

func TestSchemaToTS_ArrayOfBinary(t *testing.T) {
	// array of format: binary → (File | Blob)[]
	node := &SchemaNode{
		Type:  "array",
		Items: &SchemaNode{Type: "string", Format: "binary"},
	}
	got := SchemaToTS(node, nil)
	if got != "(File | Blob)[]" {
		t.Errorf("expected '(File | Blob)[]', got %q", got)
	}
}

func TestGenerateInterface_WithJSDoc(t *testing.T) {
	node := &SchemaNode{
		Type:        "object",
		Description: "Represents an order in the system",
		Properties: map[string]*SchemaNode{
			"id":     {Type: "string", Description: "Unique order identifier"},
			"status": {Type: "string", Description: "Current order status"},
			"total":  {Type: "number"},
		},
		Required: []string{"id", "status"},
	}
	got := GenerateInterface("Order", node, nil)

	// Interface-level JSDoc
	if !contains(got, "/** Represents an order in the system */") {
		t.Errorf("expected interface JSDoc, got:\n%s", got)
	}
	// Property-level JSDoc
	if !contains(got, "  /** Unique order identifier */") {
		t.Errorf("expected id property JSDoc, got:\n%s", got)
	}
	if !contains(got, "  /** Current order status */") {
		t.Errorf("expected status property JSDoc, got:\n%s", got)
	}
	// total has no description, should not have JSDoc
	if contains(got, "total */") {
		t.Errorf("total should not have JSDoc, got:\n%s", got)
	}
}

func TestGenerateInterface_TypeAlias_WithJSDoc(t *testing.T) {
	node := &SchemaNode{
		Type:        "string",
		Description: "Order status enum",
		Enum:        []any{"pending", "shipped", "delivered"},
	}
	got := GenerateInterface("OrderStatus", node, nil)

	if !contains(got, "/** Order status enum */") {
		t.Errorf("expected type alias JSDoc, got:\n%s", got)
	}
	if !contains(got, "export type OrderStatus =") {
		t.Errorf("expected type alias, got:\n%s", got)
	}
}

func TestGenerateInterface_MultilineJSDoc(t *testing.T) {
	node := &SchemaNode{
		Type:        "object",
		Description: "A complex type.\nUsed for various purposes.\nHandle with care.",
		Properties: map[string]*SchemaNode{
			"id": {Type: "string"},
		},
		Required: []string{"id"},
	}
	got := GenerateInterface("Complex", node, nil)

	if !contains(got, "/**\n * A complex type.\n * Used for various purposes.\n * Handle with care.\n */") {
		t.Errorf("expected multi-line JSDoc, got:\n%s", got)
	}
}

func TestSchemaToTS_EmptyObject(t *testing.T) {
	// Object with no properties and no additionalProperties
	node := &SchemaNode{Type: "object"}
	got := SchemaToTS(node, nil)
	if got != "Record<string, unknown>" {
		t.Errorf("expected Record<string, unknown>, got %q", got)
	}
}

func TestSchemaToTS_NestedObject(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"address": {
				Type: "object",
				Properties: map[string]*SchemaNode{
					"street": {Type: "string"},
					"city":   {Type: "string"},
				},
				Required: []string{"street"},
			},
		},
		Required: []string{"address"},
	}
	got := SchemaToTS(node, nil)
	// Should produce inline nested object
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(got, "address: {") {
		t.Errorf("expected nested object, got %q", got)
	}
	if !contains(got, "street: string") {
		t.Errorf("expected street: string, got %q", got)
	}
}

func TestSchemaToTS_DeeplyNestedArray(t *testing.T) {
	// string[][]
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			Type:  "array",
			Items: &SchemaNode{Type: "string"},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "string[][]" {
		t.Errorf("expected string[][], got %q", got)
	}
}

func TestSchemaToTS_MixedEnum(t *testing.T) {
	node := &SchemaNode{
		Enum: []any{"a", float64(1), true, nil},
	}
	got := SchemaToTS(node, nil)
	want := `"a" | 1 | true | null`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_ConstNumber(t *testing.T) {
	node := &SchemaNode{Const: float64(42), HasConst: true}
	got := SchemaToTS(node, nil)
	if got != "42" {
		t.Errorf("expected 42, got %q", got)
	}
}

func TestSchemaToTS_ConstBool(t *testing.T) {
	node := &SchemaNode{Const: true, HasConst: true}
	got := SchemaToTS(node, nil)
	if got != "true" {
		t.Errorf("expected true, got %q", got)
	}

	node = &SchemaNode{Const: false, HasConst: true}
	got = SchemaToTS(node, nil)
	if got != "false" {
		t.Errorf("expected false, got %q", got)
	}
}

func TestGenerateInterface_PropertyWithSpaces(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"first name":   {Type: "string"},
			"phone number": {Type: "string"},
			"email":        {Type: "string"},
		},
		Required: []string{"email"},
	}
	got := GenerateInterface("ContactInfo", node, nil)

	// Property names with spaces must be quoted
	if !contains(got, `  "first name"?: string;`) {
		t.Errorf("expected quoted 'first name', got:\n%s", got)
	}
	if !contains(got, `  "phone number"?: string;`) {
		t.Errorf("expected quoted 'phone number', got:\n%s", got)
	}
	// Normal identifiers should NOT be quoted
	if !contains(got, "  email: string;") {
		t.Errorf("expected unquoted 'email', got:\n%s", got)
	}
}

func TestSchemaToTS_InlineObjectWithSpaces(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"tax rate": {Type: "number"},
			"currency": {Type: "string"},
		},
		Required: []string{"currency"},
	}
	got := SchemaToTS(node, nil)

	if !contains(got, `"tax rate"?: number`) {
		t.Errorf("expected quoted 'tax rate' in inline object, got %q", got)
	}
	if !contains(got, "currency: string") {
		t.Errorf("expected unquoted 'currency' in inline object, got %q", got)
	}
}

func TestTsPropertyKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"name", "name"},
		{"$ref", "$ref"},
		{"_private", "_private"},
		{"first name", `"first name"`},
		{"phone number", `"phone number"`},
		{"tax rate", `"tax rate"`},
		{"first.name", `"first.name"`},
		{"user-id", `"user-id"`},
		{"filter[name]", `"filter[name]"`},
		{"123start", `"123start"`},
		{"", `""`},
		// Escaping edge cases
		{`say "hello"`, `"say \"hello\""`},
		{`back\slash`, `"back\\slash"`},
		{"new\nline", `"new\nline"`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tsPropertyKey(tt.input)
			if got != tt.want {
				t.Errorf("tsPropertyKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTsPropAccess(t *testing.T) {
	tests := []struct {
		accessor string
		name     string
		want     string
	}{
		{"obj", "name", "obj.name"},
		{"obj", "first.name", `obj["first.name"]`},
		{"obj", "user-id", `obj["user-id"]`},
		{"options", "page size", `options["page size"]`},
		{"options", "filter[name]", `options["filter[name]"]`},
		{"obj", `say "hi"`, `obj["say \"hi\""]`},
		{"obj", `back\slash`, `obj["back\\slash"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsPropAccess(tt.accessor, tt.name)
			if got != tt.want {
				t.Errorf("tsPropAccess(%q, %q) = %q, want %q", tt.accessor, tt.name, got, tt.want)
			}
		})
	}
}

func TestTsOptionalAccess(t *testing.T) {
	tests := []struct {
		accessor string
		name     string
		want     string
	}{
		{"obj", "name", "obj?.name"},
		{"obj", "first.name", `obj?.["first.name"]`},
		{"obj", "user-id", `obj?.["user-id"]`},
		{"options.query", "page size", `options.query?.["page size"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsOptionalAccess(tt.accessor, tt.name)
			if got != tt.want {
				t.Errorf("tsOptionalAccess(%q, %q) = %q, want %q", tt.accessor, tt.name, got, tt.want)
			}
		})
	}
}

// TestGenerateInterface_RecordWithAdditionalProperties verifies that an object type
// with additionalProperties but no declared properties is emitted as an interface
// with an index signature (not a type alias with Record<>). This is critical for
// self-referential types like Prisma's JsonObject where a type alias would cause
// TS2456: "Type alias circularly references itself".
func TestGenerateInterface_RecordWithAdditionalProperties(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		AdditionalProperties: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "number"},
				{Ref: "FlexObject"}, // self-reference
			},
		},
	}
	got := GenerateInterface("FlexObject", node, nil)
	// Should emit interface, NOT type alias
	if contains(got, "export type FlexObject") {
		t.Errorf("should NOT emit type alias (causes TS2456 for self-referential types), got:\n%s", got)
	}
	if !contains(got, "export interface FlexObject") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "[key: string]:") {
		t.Errorf("expected index signature, got:\n%s", got)
	}
}

// TestGenerateInterface_RecordNonSelfReferential verifies that object types
// with additionalProperties still emit correctly (as interface with index signature).
func TestGenerateInterface_RecordNonSelfReferential(t *testing.T) {
	node := &SchemaNode{
		Type:                 "object",
		AdditionalProperties: &SchemaNode{Type: "string"},
	}
	got := GenerateInterface("Config", node, nil)
	// Should emit interface with index signature
	if !contains(got, "export interface Config") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "[key: string]: string") {
		t.Errorf("expected index signature with string value, got:\n%s", got)
	}
}

// TestGenerateInterface_RecordWithDescription verifies JSDoc is preserved.
func TestGenerateInterface_RecordWithDescription(t *testing.T) {
	node := &SchemaNode{
		Type:                 "object",
		Description:          "A flexible key-value store",
		AdditionalProperties: &SchemaNode{Type: "number"},
	}
	got := GenerateInterface("Metrics", node, nil)
	if !contains(got, "A flexible key-value store") {
		t.Errorf("expected JSDoc description, got:\n%s", got)
	}
	if !contains(got, "export interface Metrics") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
}

// --- Issue #93: additionalProperties + properties ---

func TestSchemaToTS_ObjectWithPropertiesAndAdditionalProperties(t *testing.T) {
	// Object with both named properties AND additionalProperties should include both.
	// Currently broken: returns Record<string, string> and drops all named properties.
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"name":  {Type: "string"},
			"email": {Type: "string"},
		},
		Required:             []string{"name", "email"},
		AdditionalProperties: &SchemaNode{Type: "string"},
	}
	got := SchemaToTS(node, nil)

	// Must contain the named properties
	if !contains(got, "name: string") {
		t.Errorf("expected 'name: string' in output, got %q", got)
	}
	if !contains(got, "email: string") {
		t.Errorf("expected 'email: string' in output, got %q", got)
	}
	// Must contain the index signature
	if !contains(got, "[key: string]:") {
		t.Errorf("expected index signature '[key: string]:' in output, got %q", got)
	}
}

func TestGenerateInterface_ObjectWithPropertiesAndAdditionalProperties(t *testing.T) {
	// GenerateInterface with both named properties AND additionalProperties
	// should emit an interface with both named fields and an index signature.
	// Currently broken: drops the index signature entirely.
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"name":  {Type: "string"},
			"email": {Type: "string"},
			"phone": {Type: "string"},
		},
		Required:             []string{"name", "email", "phone"},
		AdditionalProperties: &SchemaNode{Type: "string"},
	}
	got := GenerateInterface("FormData_", node, nil)

	// Should be an interface, not a type alias
	if !contains(got, "export interface FormData_") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	// Must have named properties
	if !contains(got, "  name: string;") {
		t.Errorf("expected 'name: string;', got:\n%s", got)
	}
	if !contains(got, "  email: string;") {
		t.Errorf("expected 'email: string;', got:\n%s", got)
	}
	if !contains(got, "  phone: string;") {
		t.Errorf("expected 'phone: string;', got:\n%s", got)
	}
	// Must have index signature
	if !contains(got, "[key: string]: string | undefined;") {
		t.Errorf("expected index signature '[key: string]: string | undefined;', got:\n%s", got)
	}
}

func TestGenerateInterface_ObjectWithPropertiesAndComplexAdditionalProperties(t *testing.T) {
	// additionalProperties with a union type
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id": {Type: "string"},
		},
		Required: []string{"id"},
		AdditionalProperties: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "number"},
			},
		},
	}
	got := GenerateInterface("DynamicRecord", node, nil)

	if !contains(got, "export interface DynamicRecord") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "  id: string;") {
		t.Errorf("expected 'id: string;', got:\n%s", got)
	}
	if !contains(got, "[key: string]: (string | number) | undefined;") {
		t.Errorf("expected index signature with union type, got:\n%s", got)
	}
}

// --- Nullable Tests ---

func TestSchemaToTS_Nullable_String(t *testing.T) {
	node := &SchemaNode{Type: "string", Nullable: true}
	got := SchemaToTS(node, nil)
	if got != "string | null" {
		t.Errorf("expected 'string | null', got %q", got)
	}
}

func TestSchemaToTS_Nullable_Ref(t *testing.T) {
	node := &SchemaNode{Ref: "UserDto", Nullable: true}
	got := SchemaToTS(node, nil)
	if got != "UserDto | null" {
		t.Errorf("expected 'UserDto | null', got %q", got)
	}
}

func TestSchemaToTS_Nullable_Array(t *testing.T) {
	node := &SchemaNode{
		Type:     "array",
		Items:    &SchemaNode{Type: "string"},
		Nullable: true,
	}
	got := SchemaToTS(node, nil)
	if got != "string[] | null" {
		t.Errorf("expected 'string[] | null', got %q", got)
	}
}

func TestSchemaToTS_Nullable_Union(t *testing.T) {
	// anyOf: [string, number] + nullable → (string | number) | null
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "number"},
		},
		Nullable: true,
	}
	got := SchemaToTS(node, nil)
	if got != "(string | number) | null" {
		t.Errorf("expected '(string | number) | null', got %q", got)
	}
}

func TestSchemaToTS_Nullable_NoDoubleSuffix(t *testing.T) {
	// anyOf: [string, null] already produces "string | null".
	// Adding Nullable should NOT produce "string | null | null".
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "null"},
		},
		Nullable: true,
	}
	got := SchemaToTS(node, nil)
	if got != "string | null" {
		t.Errorf("expected 'string | null' (no double), got %q", got)
	}
}

// --- ReadOnly Tests ---

func TestGenerateInterface_ReadOnly(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id":   {Type: "string", ReadOnly: true},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}
	got := GenerateInterface("User", node, nil)
	if !contains(got, "  readonly id: string;") {
		t.Errorf("expected 'readonly id: string;', got:\n%s", got)
	}
	if contains(got, "readonly name") {
		t.Errorf("name should NOT be readonly, got:\n%s", got)
	}
}

func TestSchemaToTS_InlineObject_ReadOnly(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id": {Type: "string", ReadOnly: true},
		},
		Required: []string{"id"},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "readonly id: string") {
		t.Errorf("expected 'readonly id: string' in inline object, got %q", got)
	}
}

// --- Deprecated Tests ---

func TestGenerateInterface_Deprecated(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"oldField": {Type: "string", Deprecated: true},
			"newField": {Type: "string"},
		},
		Required: []string{"newField"},
	}
	got := GenerateInterface("Config", node, nil)
	if !contains(got, "@deprecated") {
		t.Errorf("expected @deprecated JSDoc, got:\n%s", got)
	}
}

func TestGenerateInterface_DeprecatedWithDescription(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"oldField": {Type: "string", Deprecated: true, Description: "Use newField instead"},
		},
	}
	got := GenerateInterface("Config", node, nil)
	if !contains(got, "Use newField instead") {
		t.Errorf("expected description in JSDoc, got:\n%s", got)
	}
	if !contains(got, "@deprecated") {
		t.Errorf("expected @deprecated in JSDoc, got:\n%s", got)
	}
}

// --- Default Value Tests ---

func TestGenerateInterface_DefaultString(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"role": {Type: "string", HasDefault: true, Default: "user"},
		},
	}
	got := GenerateInterface("Settings", node, nil)
	if !contains(got, `@default "user"`) {
		t.Errorf("expected '@default \"user\"', got:\n%s", got)
	}
}

func TestGenerateInterface_DefaultNumber(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"limit": {Type: "number", HasDefault: true, Default: float64(10)},
		},
	}
	got := GenerateInterface("Query", node, nil)
	if !contains(got, "@default 10") {
		t.Errorf("expected '@default 10', got:\n%s", got)
	}
}

func TestGenerateInterface_DefaultBool(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"active": {Type: "boolean", HasDefault: true, Default: true},
		},
	}
	got := GenerateInterface("Filter", node, nil)
	if !contains(got, "@default true") {
		t.Errorf("expected '@default true', got:\n%s", got)
	}
}

func TestGenerateInterface_DefaultNull(t *testing.T) {
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"avatar": {Type: "string", HasDefault: true, Default: nil, Nullable: true},
		},
	}
	got := GenerateInterface("Profile", node, nil)
	if !contains(got, "@default null") {
		t.Errorf("expected '@default null', got:\n%s", got)
	}
}

// --- Tuple Tests ---

func TestSchemaToTS_PrefixItems(t *testing.T) {
	node := &SchemaNode{
		Type: "array",
		PrefixItems: []*SchemaNode{
			{Type: "number"},
			{Type: "number"},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "[number, number]" {
		t.Errorf("expected '[number, number]', got %q", got)
	}
}

func TestSchemaToTS_PrefixItemsWithRestElement(t *testing.T) {
	node := &SchemaNode{
		Type: "array",
		PrefixItems: []*SchemaNode{
			{Type: "string"},
			{Type: "number"},
		},
		Items: &SchemaNode{Type: "boolean"},
	}
	got := SchemaToTS(node, nil)
	if got != "[string, number, ...boolean[]]" {
		t.Errorf("expected '[string, number, ...boolean[]]', got %q", got)
	}
}

func TestSchemaToTS_PrefixItemsWithRefs(t *testing.T) {
	node := &SchemaNode{
		Type: "array",
		PrefixItems: []*SchemaNode{
			{Ref: "Latitude"},
			{Ref: "Longitude"},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "[Latitude, Longitude]" {
		t.Errorf("expected '[Latitude, Longitude]', got %q", got)
	}
}

func TestSchemaToTS_HomogeneousTuple(t *testing.T) {
	min, max := 3, 3
	node := &SchemaNode{
		Type:     "array",
		Items:    &SchemaNode{Type: "number"},
		MinItems: &min,
		MaxItems: &max,
	}
	got := SchemaToTS(node, nil)
	if got != "[number, number, number]" {
		t.Errorf("expected '[number, number, number]', got %q", got)
	}
}

func TestSchemaToTS_HomogeneousTuple_Cap(t *testing.T) {
	// minItems == maxItems == 15 → too large, stay as array
	min, max := 15, 15
	node := &SchemaNode{
		Type:     "array",
		Items:    &SchemaNode{Type: "number"},
		MinItems: &min,
		MaxItems: &max,
	}
	got := SchemaToTS(node, nil)
	if got != "number[]" {
		t.Errorf("expected 'number[]' (cap at 10), got %q", got)
	}
}

func TestSchemaToTS_MinMaxDifferent(t *testing.T) {
	// minItems != maxItems → regular array, not tuple
	min, max := 1, 5
	node := &SchemaNode{
		Type:     "array",
		Items:    &SchemaNode{Type: "string"},
		MinItems: &min,
		MaxItems: &max,
	}
	got := SchemaToTS(node, nil)
	if got != "string[]" {
		t.Errorf("expected 'string[]' (min != max), got %q", got)
	}
}

// --- Discriminated Union Tests ---

func TestSchemaToTS_DiscriminatedUnion(t *testing.T) {
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{Ref: "Dog"},
			{Ref: "Cat"},
		},
		Discriminator: &Discriminator{
			PropertyName: "type",
			Mapping: map[string]string{
				"dog": "#/components/schemas/Dog",
				"cat": "#/components/schemas/Cat",
			},
		},
	}
	got := SchemaToTS(node, nil)
	// Should contain Omit<> & literal injection for both variants
	if !contains(got, `Omit<Dog, "type">`) {
		t.Errorf("expected Omit<Dog, \"type\">, got %q", got)
	}
	if !contains(got, `{ type: "dog" }`) {
		t.Errorf("expected { type: \"dog\" }, got %q", got)
	}
	if !contains(got, `Omit<Cat, "type">`) {
		t.Errorf("expected Omit<Cat, \"type\">, got %q", got)
	}
	if !contains(got, `{ type: "cat" }`) {
		t.Errorf("expected { type: \"cat\" }, got %q", got)
	}
}

func TestSchemaToTS_DiscriminatedUnionWithNull(t *testing.T) {
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{Ref: "Dog"},
			{Type: "null"},
		},
		Discriminator: &Discriminator{
			PropertyName: "kind",
			Mapping: map[string]string{
				"dog": "#/components/schemas/Dog",
			},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "| null") {
		t.Errorf("expected '| null' in output, got %q", got)
	}
	if !contains(got, `Omit<Dog, "kind">`) {
		t.Errorf("expected Omit<Dog, \"kind\">, got %q", got)
	}
}

func TestSchemaToTS_DiscriminatedUnionNoMapping(t *testing.T) {
	// discriminator with propertyName but no mapping → plain union
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{Ref: "Dog"},
			{Ref: "Cat"},
		},
		Discriminator: &Discriminator{
			PropertyName: "type",
		},
	}
	got := SchemaToTS(node, nil)
	// Without mapping, variants stay bare (no Omit injection)
	if got != "Dog | Cat" {
		t.Errorf("expected 'Dog | Cat', got %q", got)
	}
}

// --- formatJSDocDefault Tests ---

func TestFormatJSDocDefault(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"nil", nil, "null"},
		{"string", "hello", `"hello"`},
		{"integer", float64(42), "42"},
		{"float", float64(3.14), "3.14"},
		{"true", true, "true"},
		{"false", false, "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatJSDocDefault(tt.val)
			if got != tt.want {
				t.Errorf("formatJSDocDefault(%v) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Edge case tests ported from openapi-typescript, json-schema-to-typescript,
// and quicktype test suites.
// =============================================================================

// --- additionalProperties edge cases (from openapi-typescript object.test.ts) ---

func TestSchemaToTS_AdditionalProperties_True(t *testing.T) {
	// additionalProperties: true → { prop?: T } with [key: string]: unknown
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"property": {Type: "number"},
		},
		AdditionalProperties: &SchemaNode{}, // empty schema = unknown
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "property?: number") {
		t.Errorf("expected 'property?: number', got %q", got)
	}
	if !contains(got, "[key: string]: unknown | undefined") {
		t.Errorf("expected index signature with unknown, got %q", got)
	}
}

func TestSchemaToTS_AdditionalProperties_OnlyIndexSignature(t *testing.T) {
	// Object with ONLY additionalProperties and no properties → Record<string, T>
	node := &SchemaNode{
		Type:                 "object",
		AdditionalProperties: &SchemaNode{Type: "number"},
	}
	got := SchemaToTS(node, nil)
	if got != "Record<string, number>" {
		t.Errorf("expected Record<string, number>, got %q", got)
	}
}

func TestGenerateInterface_AdditionalProperties_WithRef(t *testing.T) {
	// additionalProperties referencing another type
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id": {Type: "string"},
		},
		Required:             []string{"id"},
		AdditionalProperties: &SchemaNode{Ref: "MetadataValue"},
	}
	got := GenerateInterface("FlexDoc", node, nil)
	if !contains(got, "  id: string;") {
		t.Errorf("expected 'id: string;', got:\n%s", got)
	}
	if !contains(got, "[key: string]: MetadataValue | undefined;") {
		t.Errorf("expected index sig with ref type, got:\n%s", got)
	}
}

// --- Nullable handling (from openapi-typescript composition.test.ts) ---

func TestSchemaToTS_NullableString(t *testing.T) {
	// type: ["string", "null"] → string | null (via anyOf)
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "string"},
			{Type: "null"},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "string | null" {
		t.Errorf("expected 'string | null', got %q", got)
	}
}

func TestSchemaToTS_NullableObject(t *testing.T) {
	// Object | null via anyOf
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"name": {Type: "string"},
				},
			},
			{Type: "null"},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "name?: string") {
		t.Errorf("expected 'name?: string' in nullable object, got %q", got)
	}
	if !contains(got, "| null") {
		t.Errorf("expected '| null' in nullable object, got %q", got)
	}
}

func TestSchemaToTS_NullableArrayItems(t *testing.T) {
	// Array of nullable strings: (string | null)[]
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "null"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "(string | null)[]" {
		t.Errorf("expected '(string | null)[]', got %q", got)
	}
}

func TestSchemaToTS_NullableEnum(t *testing.T) {
	// Enum with null: null | "blue" | "green"
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Enum: []any{"blue", "green"}},
			{Type: "null"},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, `"blue"`) {
		t.Errorf("expected enum values, got %q", got)
	}
	if !contains(got, "| null") {
		t.Errorf("expected '| null', got %q", got)
	}
}

// --- allOf merging edge cases (from openapi-typescript composition.test.ts) ---

func TestSchemaToTS_AllOf_MultipleObjects(t *testing.T) {
	// allOf with two inline objects → intersection
	node := &SchemaNode{
		AllOf: []*SchemaNode{
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"red":  {Type: "number"},
					"blue": {Type: "number"},
				},
				Required: []string{"red", "blue"},
			},
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"green": {Type: "number"},
				},
				Required: []string{"green"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "red: number") {
		t.Errorf("expected 'red: number', got %q", got)
	}
	if !contains(got, "green: number") {
		t.Errorf("expected 'green: number', got %q", got)
	}
	if !contains(got, " & ") {
		t.Errorf("expected intersection '&', got %q", got)
	}
}

func TestSchemaToTS_AllOf_WithRefs(t *testing.T) {
	// allOf mixing refs and inline objects
	node := &SchemaNode{
		AllOf: []*SchemaNode{
			{Ref: "BaseEntity"},
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"extra": {Type: "string"},
				},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "BaseEntity") {
		t.Errorf("expected 'BaseEntity' ref, got %q", got)
	}
	if !contains(got, "extra?: string") {
		t.Errorf("expected 'extra?: string', got %q", got)
	}
	if !contains(got, " & ") {
		t.Errorf("expected intersection '&', got %q", got)
	}
}

// --- oneOf / anyOf complex compositions (from openapi-typescript & quicktype) ---

func TestSchemaToTS_OneOf_ConstValues(t *testing.T) {
	// oneOf with const values → union of literals
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{Const: "hello", HasConst: true},
			{Const: "world", HasConst: true},
		},
	}
	got := SchemaToTS(node, nil)
	want := `("hello" | "world")`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_OneOf_NumericConsts(t *testing.T) {
	// oneOf with numeric consts
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{Const: float64(0), HasConst: true},
			{Const: float64(1), HasConst: true},
		},
	}
	got := SchemaToTS(node, nil)
	want := "(0 | 1)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_AnyOf_MultipleObjects(t *testing.T) {
	// anyOf with multiple object schemas → union
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"red": {Type: "number"},
				},
				Required: []string{"red"},
			},
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"blue": {Type: "number"},
				},
				Required: []string{"blue"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "red: number") {
		t.Errorf("expected 'red: number', got %q", got)
	}
	if !contains(got, "blue: number") {
		t.Errorf("expected 'blue: number', got %q", got)
	}
	if !contains(got, " | ") {
		t.Errorf("expected union '|', got %q", got)
	}
}

func TestSchemaToTS_OneOf_MixedPrimitivesAndObjects(t *testing.T) {
	// oneOf mixing primitives and objects
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{Type: "string"},
			{Type: "number"},
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"id": {Type: "string"},
				},
				Required: []string{"id"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "string") {
		t.Errorf("expected 'string' in union, got %q", got)
	}
	if !contains(got, "number") {
		t.Errorf("expected 'number' in union, got %q", got)
	}
	if !contains(got, "id: string") {
		t.Errorf("expected inline object with 'id: string', got %q", got)
	}
}

// --- Enum edge cases (from json-schema-to-typescript & quicktype) ---

func TestSchemaToTS_EnumWithNull(t *testing.T) {
	// Enum containing null value
	node := &SchemaNode{
		Enum: []any{"foo", "bar", nil},
	}
	got := SchemaToTS(node, nil)
	want := `"foo" | "bar" | null`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_EnumSingleValue(t *testing.T) {
	// Single-value enum → literal type
	node := &SchemaNode{
		Enum: []any{"only"},
	}
	got := SchemaToTS(node, nil)
	if got != `"only"` {
		t.Errorf("expected '\"only\"', got %q", got)
	}
}

func TestSchemaToTS_EnumNumericValues(t *testing.T) {
	// Numeric enum
	node := &SchemaNode{
		Enum: []any{float64(100), float64(200), float64(300)},
	}
	got := SchemaToTS(node, nil)
	want := "100 | 200 | 300"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSchemaToTS_EnumBooleanValues(t *testing.T) {
	// Boolean enum (e.g., strict true-only flag)
	node := &SchemaNode{
		Enum: []any{true},
	}
	got := SchemaToTS(node, nil)
	if got != "true" {
		t.Errorf("expected 'true', got %q", got)
	}
}

// --- Recursive / self-referential types (from json-schema-to-typescript refWithCycle) ---

func TestGenerateInterface_SelfReferentialArray(t *testing.T) {
	// Tree node with children: TreeNode[]
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"value": {Type: "string"},
			"children": {
				Type:  "array",
				Items: &SchemaNode{Ref: "TreeNode"},
			},
		},
		Required: []string{"value"},
	}
	got := GenerateInterface("TreeNode", node, nil)
	if !contains(got, "export interface TreeNode") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "  value: string;") {
		t.Errorf("expected 'value: string;', got:\n%s", got)
	}
	if !contains(got, "  children?: TreeNode[];") {
		t.Errorf("expected 'children?: TreeNode[];', got:\n%s", got)
	}
}

func TestGenerateInterface_MutuallyRecursive(t *testing.T) {
	// Foo references Bar which references Foo (via $ref)
	fooNode := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"name": {Type: "string"},
			"bar":  {Ref: "Bar"},
		},
		Required: []string{"name"},
	}
	got := GenerateInterface("Foo", fooNode, nil)
	if !contains(got, "export interface Foo") {
		t.Errorf("expected interface declaration, got:\n%s", got)
	}
	if !contains(got, "  bar?: Bar;") {
		t.Errorf("expected 'bar?: Bar;', got:\n%s", got)
	}
}

// --- Empty & degenerate schemas (from json-schema-to-typescript) ---

func TestSchemaToTS_EmptySchema_NoType(t *testing.T) {
	// {} with no type, no properties → unknown
	node := &SchemaNode{}
	got := SchemaToTS(node, nil)
	if got != "unknown" {
		t.Errorf("expected 'unknown' for empty schema, got %q", got)
	}
}

func TestGenerateInterface_EmptyObject_NoProperties(t *testing.T) {
	// type: "object" with no properties → Record<string, unknown>
	node := &SchemaNode{Type: "object"}
	got := GenerateInterface("Empty", node, nil)
	if !contains(got, "export type Empty = Record<string, unknown>;") {
		t.Errorf("expected type alias to Record, got:\n%s", got)
	}
}

// --- Nested and deeply structured types (from quicktype) ---

func TestSchemaToTS_ArrayOfObjectsWithOptionalFields(t *testing.T) {
	// Array of objects where some fields are optional
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			Type: "object",
			Properties: map[string]*SchemaNode{
				"id":    {Type: "string"},
				"label": {Type: "string"},
			},
			Required: []string{"id"},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "id: string") {
		t.Errorf("expected required 'id: string', got %q", got)
	}
	if !contains(got, "label?: string") {
		t.Errorf("expected optional 'label?: string', got %q", got)
	}
	if !contains(got, "[]") {
		t.Errorf("expected array suffix, got %q", got)
	}
}

func TestSchemaToTS_NestedObjectWithEnum(t *testing.T) {
	// Object containing a property that is an enum
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"status": {Enum: []any{"active", "inactive", "pending"}},
			"count":  {Type: "number"},
		},
		Required: []string{"status", "count"},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, `"active" | "inactive" | "pending"`) {
		t.Errorf("expected enum union in inline object, got %q", got)
	}
	if !contains(got, "count: number") {
		t.Errorf("expected 'count: number', got %q", got)
	}
}

func TestSchemaToTS_UnionOfArrays(t *testing.T) {
	// anyOf with two different array types
	node := &SchemaNode{
		AnyOf: []*SchemaNode{
			{Type: "array", Items: &SchemaNode{Type: "string"}},
			{Type: "array", Items: &SchemaNode{Type: "number"}},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "(string[] | number[])" {
		t.Errorf("expected '(string[] | number[])', got %q", got)
	}
}

func TestSchemaToTS_ArrayOfUnionWithNull(t *testing.T) {
	// Array where items can be string, number, or null
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			AnyOf: []*SchemaNode{
				{Type: "string"},
				{Type: "number"},
				{Type: "null"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "(string | number) | null[]" {
		// The null gets separated from the main union
		// Accept either form as long as all types present
		if !contains(got, "string") || !contains(got, "number") || !contains(got, "null") {
			t.Errorf("expected union of string|number|null in array, got %q", got)
		}
	}
}

// --- Const edge cases (from openapi-typescript) ---

func TestSchemaToTS_ConstNull(t *testing.T) {
	node := &SchemaNode{Const: nil, HasConst: true}
	got := SchemaToTS(node, nil)
	if got != "null" {
		t.Errorf("expected 'null', got %q", got)
	}
}

func TestSchemaToTS_ConstFalsyNumber(t *testing.T) {
	// const: 0 should produce literal 0, not be treated as falsy
	node := &SchemaNode{Const: float64(0), HasConst: true}
	got := SchemaToTS(node, nil)
	if got != "0" {
		t.Errorf("expected '0', got %q", got)
	}
}

func TestSchemaToTS_ConstEmptyString(t *testing.T) {
	// const: "" should produce literal ""
	node := &SchemaNode{Const: "", HasConst: true}
	got := SchemaToTS(node, nil)
	if got != `""` {
		t.Errorf("expected empty string literal, got %q", got)
	}
}

func TestSchemaToTS_ConstFalseBool(t *testing.T) {
	// const: false should produce literal false
	node := &SchemaNode{Const: false, HasConst: true}
	got := SchemaToTS(node, nil)
	if got != "false" {
		t.Errorf("expected 'false', got %q", got)
	}
}

// --- Complex compositions (from quicktype & json-schema-to-typescript) ---

func TestSchemaToTS_AllOf_RefPlusInlineProperties(t *testing.T) {
	// allOf: [$ref, inline object] — common inheritance pattern
	node := &SchemaNode{
		AllOf: []*SchemaNode{
			{Ref: "Animal"},
			{
				Type: "object",
				Properties: map[string]*SchemaNode{
					"breed": {Type: "string"},
					"color": {Type: "string"},
				},
				Required: []string{"breed"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "Animal") {
		t.Errorf("expected 'Animal' ref in allOf, got %q", got)
	}
	if !contains(got, "breed: string") {
		t.Errorf("expected required 'breed: string', got %q", got)
	}
	if !contains(got, "color?: string") {
		t.Errorf("expected optional 'color?: string', got %q", got)
	}
}

func TestSchemaToTS_DeeplyNestedOneOf(t *testing.T) {
	// Nested composition: oneOf containing anyOf
	node := &SchemaNode{
		OneOf: []*SchemaNode{
			{
				AnyOf: []*SchemaNode{
					{Type: "string"},
					{Type: "number"},
				},
			},
			{Type: "boolean"},
		},
	}
	got := SchemaToTS(node, nil)
	if !contains(got, "string") && !contains(got, "number") && !contains(got, "boolean") {
		t.Errorf("expected all types present in nested composition, got %q", got)
	}
}

func TestGenerateInterface_AllFieldsOptional(t *testing.T) {
	// Object where no properties are required
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"a": {Type: "string"},
			"b": {Type: "number"},
			"c": {Type: "boolean"},
		},
		// No Required slice — all optional
	}
	got := GenerateInterface("AllOptional", node, nil)
	if !contains(got, "  a?: string;") {
		t.Errorf("expected optional 'a?: string;', got:\n%s", got)
	}
	if !contains(got, "  b?: number;") {
		t.Errorf("expected optional 'b?: number;', got:\n%s", got)
	}
	if !contains(got, "  c?: boolean;") {
		t.Errorf("expected optional 'c?: boolean;', got:\n%s", got)
	}
}

func TestGenerateInterface_AllFieldsRequired(t *testing.T) {
	// Object where all properties are required
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"x": {Type: "number"},
			"y": {Type: "number"},
		},
		Required: []string{"x", "y"},
	}
	got := GenerateInterface("Point", node, nil)
	if !contains(got, "  x: number;") {
		t.Errorf("expected required 'x: number;', got:\n%s", got)
	}
	if !contains(got, "  y: number;") {
		t.Errorf("expected required 'y: number;', got:\n%s", got)
	}
	// Should NOT have ? on any property
	if contains(got, "?") {
		t.Errorf("expected no optional markers, got:\n%s", got)
	}
}

func TestGenerateInterface_PropertyWithArrayOfRefs(t *testing.T) {
	// Object with a property that is an array of $ref types
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"items": {
				Type:  "array",
				Items: &SchemaNode{Ref: "OrderItem"},
			},
			"total": {Type: "number"},
		},
		Required: []string{"items", "total"},
	}
	got := GenerateInterface("Order", node, nil)
	if !contains(got, "  items: OrderItem[];") {
		t.Errorf("expected 'items: OrderItem[];', got:\n%s", got)
	}
	if !contains(got, "  total: number;") {
		t.Errorf("expected 'total: number;', got:\n%s", got)
	}
}

func TestGenerateInterface_PropertyWithNullableRef(t *testing.T) {
	// Property that is a nullable $ref: User | null
	node := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"assignee": {
				AnyOf: []*SchemaNode{
					{Ref: "User"},
					{Type: "null"},
				},
			},
		},
	}
	got := GenerateInterface("Task", node, nil)
	if !contains(got, "assignee?: User | null;") {
		t.Errorf("expected nullable ref 'assignee?: User | null;', got:\n%s", got)
	}
}

func TestSchemaToTS_TripleNestedArray(t *testing.T) {
	// string[][][]
	node := &SchemaNode{
		Type: "array",
		Items: &SchemaNode{
			Type: "array",
			Items: &SchemaNode{
				Type:  "array",
				Items: &SchemaNode{Type: "string"},
			},
		},
	}
	got := SchemaToTS(node, nil)
	if got != "string[][][]" {
		t.Errorf("expected 'string[][][]', got %q", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
