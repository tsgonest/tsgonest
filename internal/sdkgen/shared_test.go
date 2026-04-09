package sdkgen

import (
	"testing"
)

// --- safeTSName Tests ---

func TestSafeTSName_BuiltinCollision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Record", "Record_"},
		{"Array", "Array_"},
		{"Promise", "Promise_"},
		{"Map", "Map_"},
		{"Set", "Set_"},
		{"Date", "Date_"},
		{"Error", "Error_"},
		{"Boolean", "Boolean_"},
		{"Number", "Number_"},
		{"String", "String_"},
		{"Symbol", "Symbol_"},
		{"Object", "Object_"},
		{"Function", "Function_"},
		{"Partial", "Partial_"},
		{"Required", "Required_"},
		{"Readonly", "Readonly_"},
		{"Pick", "Pick_"},
		{"Omit", "Omit_"},
		{"Exclude", "Exclude_"},
		{"Extract", "Extract_"},
		{"NonNullable", "NonNullable_"},
		{"ReturnType", "ReturnType_"},
		{"ReadonlyArray", "ReadonlyArray_"},
		{"BigInt", "BigInt_"},
		{"ArrayBuffer", "ArrayBuffer_"},
		{"Uint8Array", "Uint8Array_"},
		{"WeakMap", "WeakMap_"},
		{"WeakSet", "WeakSet_"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := safeTSName(tt.input)
			if got != tt.want {
				t.Errorf("safeTSName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafeTSName_NoCollision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MyType", "MyType"},
		{"UserDto", "UserDto"},
		{"Order", "Order"},
		{"CreateOrderDto", "CreateOrderDto"},
		{"record", "record"},   // lowercase — not a built-in
		{"RECORD", "RECORD"},   // uppercase — not a built-in
		{"Records", "Records"}, // pluralized — not a built-in
		{"MyRecord", "MyRecord"},
		{"RecordItem", "RecordItem"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := safeTSName(tt.input)
			if got != tt.want {
				t.Errorf("safeTSName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- replaceBareName Tests ---

func TestReplaceBareName_BareOccurrence(t *testing.T) {
	// Bare "Record" at end of string should be replaced
	got := replaceBareName("Record", "Record", "Record_")
	if got != "Record_" {
		t.Errorf("expected 'Record_', got %q", got)
	}
}

func TestReplaceBareName_NotGeneric(t *testing.T) {
	// "Record<string, number>" — the built-in generic usage should NOT be replaced
	got := replaceBareName("Record<string, number>", "Record", "Record_")
	if got != "Record<string, number>" {
		t.Errorf("expected unchanged 'Record<string, number>', got %q", got)
	}
}

func TestReplaceBareName_InArray(t *testing.T) {
	// "Record[]" — bare Record followed by [] should be replaced
	got := replaceBareName("Record[]", "Record", "Record_")
	if got != "Record_[]" {
		t.Errorf("expected 'Record_[]', got %q", got)
	}
}

func TestReplaceBareName_InUnion(t *testing.T) {
	// "Record | null" — bare Record in union should be replaced
	got := replaceBareName("Record | null", "Record", "Record_")
	if got != "Record_ | null" {
		t.Errorf("expected 'Record_ | null', got %q", got)
	}
}

func TestReplaceBareName_PartOfLargerIdentifier(t *testing.T) {
	// "RecordItem" — "Record" is part of a larger identifier, should NOT be replaced
	got := replaceBareName("RecordItem", "Record", "Record_")
	if got != "RecordItem" {
		t.Errorf("expected unchanged 'RecordItem', got %q", got)
	}
}

func TestReplaceBareName_PrefixedIdentifier(t *testing.T) {
	// "MyRecord" — "Record" is preceded by identifier char, should NOT be replaced
	got := replaceBareName("MyRecord", "Record", "Record_")
	if got != "MyRecord" {
		t.Errorf("expected unchanged 'MyRecord', got %q", got)
	}
}

func TestReplaceBareName_MultipleOccurrences(t *testing.T) {
	// Multiple bare Record occurrences should all be replaced
	got := replaceBareName("Record | Record[]", "Record", "Record_")
	if got != "Record_ | Record_[]" {
		t.Errorf("expected 'Record_ | Record_[]', got %q", got)
	}
}

func TestReplaceBareName_NoOccurrence(t *testing.T) {
	got := replaceBareName("Order", "Record", "Record_")
	if got != "Order" {
		t.Errorf("expected unchanged 'Order', got %q", got)
	}
}

func TestReplaceBareName_MixedBareAndGeneric(t *testing.T) {
	// "Record | Record<string, number>" — only bare Record should be replaced
	got := replaceBareName("Record | Record<string, number>", "Record", "Record_")
	if got != "Record_ | Record<string, number>" {
		t.Errorf("expected 'Record_ | Record<string, number>', got %q", got)
	}
}

// --- renameBuiltinCollisions Tests ---

func TestRenameBuiltinCollisions_RenamesSchemaKeys(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Record": {Type: "object", Properties: map[string]*SchemaNode{
				"key":   {Type: "string"},
				"value": {Type: "string"},
			}},
			"Order": {Type: "object", Properties: map[string]*SchemaNode{
				"id": {Type: "string"},
			}},
		},
		Versions: []VersionGroup{},
	}

	renameBuiltinCollisions(doc)

	// "Record" should be renamed to "Record_"
	if _, ok := doc.Schemas["Record_"]; !ok {
		t.Error("expected schema key 'Record_' after renaming")
	}
	if _, ok := doc.Schemas["Record"]; ok {
		t.Error("expected old schema key 'Record' to be removed")
	}

	// "Order" should remain unchanged
	if _, ok := doc.Schemas["Order"]; !ok {
		t.Error("expected schema key 'Order' to remain")
	}
}

func TestRenameBuiltinCollisions_UpdatesRefs(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Record": {Type: "object", Properties: map[string]*SchemaNode{
				"id": {Type: "string"},
			}},
			"Order": {Type: "object", Properties: map[string]*SchemaNode{
				"record": {Ref: "Record"},
			}},
		},
		Versions: []VersionGroup{},
	}

	renameBuiltinCollisions(doc)

	// The $ref in Order.record should now point to "Record_"
	orderSchema := doc.Schemas["Order"]
	if orderSchema == nil {
		t.Fatal("Order schema not found")
	}
	recordProp := orderSchema.Properties["record"]
	if recordProp == nil {
		t.Fatal("record property not found in Order")
	}
	if recordProp.Ref != "Record_" {
		t.Errorf("expected $ref to be 'Record_', got %q", recordProp.Ref)
	}
}

func TestRenameBuiltinCollisions_UpdatesControllerMethodTypes(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Record": {Type: "object"},
		},
		Versions: []VersionGroup{
			{
				Version: "",
				Controllers: []ControllerGroup{
					{
						Name: "TestController",
						Methods: []SDKMethod{
							{
								Name:         "getRecord",
								ResponseType: "Record",
								Body:         &SDKBody{TSType: "Record"},
								QueryParams:  []SDKParam{{Name: "filter", TSType: "Record"}},
								PathParams:   []SDKParam{{Name: "id", TSType: "Record"}},
							},
						},
					},
				},
			},
		},
	}

	renameBuiltinCollisions(doc)

	method := doc.Versions[0].Controllers[0].Methods[0]
	if method.ResponseType != "Record_" {
		t.Errorf("expected ResponseType='Record_', got %q", method.ResponseType)
	}
	if method.Body.TSType != "Record_" {
		t.Errorf("expected Body.TSType='Record_', got %q", method.Body.TSType)
	}
	if method.QueryParams[0].TSType != "Record_" {
		t.Errorf("expected QueryParams[0].TSType='Record_', got %q", method.QueryParams[0].TSType)
	}
	if method.PathParams[0].TSType != "Record_" {
		t.Errorf("expected PathParams[0].TSType='Record_', got %q", method.PathParams[0].TSType)
	}
}

func TestRenameBuiltinCollisions_UpdatesSSEEventType(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Error": {Type: "object"},
		},
		Versions: []VersionGroup{
			{
				Version: "",
				Controllers: []ControllerGroup{
					{
						Name: "EventsController",
						Methods: []SDKMethod{
							{
								Name:         "streamErrors",
								SSEEventType: "Error",
							},
						},
					},
				},
			},
		},
	}

	renameBuiltinCollisions(doc)

	method := doc.Versions[0].Controllers[0].Methods[0]
	if method.SSEEventType != "Error_" {
		t.Errorf("expected SSEEventType='Error_', got %q", method.SSEEventType)
	}
}

func TestRenameBuiltinCollisions_NoCollisions(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Order":          {Type: "object"},
			"CreateOrderDto": {Type: "object"},
		},
		Versions: []VersionGroup{},
	}

	renameBuiltinCollisions(doc)

	// Nothing should change
	if _, ok := doc.Schemas["Order"]; !ok {
		t.Error("expected Order to remain")
	}
	if _, ok := doc.Schemas["CreateOrderDto"]; !ok {
		t.Error("expected CreateOrderDto to remain")
	}
}

func TestRenameBuiltinCollisions_NestedRefs(t *testing.T) {
	// Test that refs inside Items, AnyOf, AllOf, OneOf, AdditionalProperties are updated
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Array": {Type: "object", Properties: map[string]*SchemaNode{
				"data": {Type: "string"},
			}},
			"Container": {Type: "object", Properties: map[string]*SchemaNode{
				"items": {
					Type:  "array",
					Items: &SchemaNode{Ref: "Array"},
				},
				"extra": {
					AnyOf: []*SchemaNode{
						{Ref: "Array"},
						{Type: "null"},
					},
				},
				"all": {
					AllOf: []*SchemaNode{
						{Ref: "Array"},
					},
				},
				"one": {
					OneOf: []*SchemaNode{
						{Ref: "Array"},
					},
				},
				"map": {
					Type:                 "object",
					AdditionalProperties: &SchemaNode{Ref: "Array"},
				},
			}},
		},
		Versions: []VersionGroup{},
	}

	renameBuiltinCollisions(doc)

	container := doc.Schemas["Container"]
	if container == nil {
		t.Fatal("Container schema not found")
	}

	// Items ref
	if container.Properties["items"].Items.Ref != "Array_" {
		t.Errorf("expected items.$ref = 'Array_', got %q", container.Properties["items"].Items.Ref)
	}
	// AnyOf ref
	if container.Properties["extra"].AnyOf[0].Ref != "Array_" {
		t.Errorf("expected anyOf[0].$ref = 'Array_', got %q", container.Properties["extra"].AnyOf[0].Ref)
	}
	// AllOf ref
	if container.Properties["all"].AllOf[0].Ref != "Array_" {
		t.Errorf("expected allOf[0].$ref = 'Array_', got %q", container.Properties["all"].AllOf[0].Ref)
	}
	// OneOf ref
	if container.Properties["one"].OneOf[0].Ref != "Array_" {
		t.Errorf("expected oneOf[0].$ref = 'Array_', got %q", container.Properties["one"].OneOf[0].Ref)
	}
	// AdditionalProperties ref
	if container.Properties["map"].AdditionalProperties.Ref != "Array_" {
		t.Errorf("expected additionalProperties.$ref = 'Array_', got %q", container.Properties["map"].AdditionalProperties.Ref)
	}
}

func TestRenameBuiltinCollisions_GenericUsagePreserved(t *testing.T) {
	// A method that uses Record<string, number> as a response type — the built-in
	// generic usage should NOT be renamed, only bare schema references.
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Record": {Type: "object"},
		},
		Versions: []VersionGroup{
			{
				Version: "",
				Controllers: []ControllerGroup{
					{
						Name: "TestController",
						Methods: []SDKMethod{
							{
								Name:         "getBareRecord",
								ResponseType: "Record",
							},
							{
								Name:         "getGenericRecord",
								ResponseType: "Record<string, number>",
							},
						},
					},
				},
			},
		},
	}

	renameBuiltinCollisions(doc)

	methods := doc.Versions[0].Controllers[0].Methods

	// Bare Record should be renamed
	if methods[0].ResponseType != "Record_" {
		t.Errorf("bare Record should be renamed to 'Record_', got %q", methods[0].ResponseType)
	}

	// Generic Record<string, number> should NOT be renamed
	if methods[1].ResponseType != "Record<string, number>" {
		t.Errorf("generic Record<string, number> should be preserved, got %q", methods[1].ResponseType)
	}
}

// --- renameBuiltinCollisions Warning Tests ---

func TestRenameBuiltinCollisions_ReturnsWarnings(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Record": {Type: "object", Properties: map[string]*SchemaNode{
				"key": {Type: "string"},
			}},
			"Date": {Type: "object", Properties: map[string]*SchemaNode{
				"value": {Type: "string"},
			}},
			"Order": {Type: "object", Properties: map[string]*SchemaNode{
				"id": {Type: "string"},
			}},
		},
		Versions: []VersionGroup{},
	}

	warnings := renameBuiltinCollisions(doc)

	// Should return warnings for Record and Date (not Order)
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}

	// Check that warnings mention the renamed types
	hasRecord := false
	hasDate := false
	for _, w := range warnings {
		if contains(w, "Record") && contains(w, `"Record_"`) {
			hasRecord = true
		}
		if contains(w, "Date") && contains(w, `"Date_"`) {
			hasDate = true
		}
	}
	if !hasRecord {
		t.Errorf("expected warning about Record → Record_ rename, got: %v", warnings)
	}
	if !hasDate {
		t.Errorf("expected warning about Date → Date_ rename, got: %v", warnings)
	}
}

func TestRenameBuiltinCollisions_NoWarningsWhenNoCollisions(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Order":          {Type: "object"},
			"CreateOrderDto": {Type: "object"},
		},
		Versions: []VersionGroup{},
	}

	warnings := renameBuiltinCollisions(doc)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warnings), warnings)
	}
}

// --- TS Enum Tests ---

func TestGenerateTSEnum_StringValues(t *testing.T) {
	node := &SchemaNode{
		Type: "string",
		Enum: []any{"active", "inactive", "archived"},
	}
	got := generateTSEnum("Status", node)
	if !contains(got, "export enum Status {") {
		t.Errorf("expected enum declaration, got:\n%s", got)
	}
	if !contains(got, `Active = "active"`) {
		t.Errorf("expected Active member, got:\n%s", got)
	}
	if !contains(got, `Inactive = "inactive"`) {
		t.Errorf("expected Inactive member, got:\n%s", got)
	}
	if !contains(got, `Archived = "archived"`) {
		t.Errorf("expected Archived member, got:\n%s", got)
	}
}

func TestGenerateTSEnum_NumericValues(t *testing.T) {
	node := &SchemaNode{
		Type: "integer",
		Enum: []any{float64(0), float64(1), float64(2)},
	}
	got := generateTSEnum("Priority", node)
	if !contains(got, "export enum Priority {") {
		t.Errorf("expected enum declaration, got:\n%s", got)
	}
	if !contains(got, "VALUE_0 = 0") {
		t.Errorf("expected VALUE_0 member, got:\n%s", got)
	}
}

func TestGenerateTSEnum_KebabCase(t *testing.T) {
	node := &SchemaNode{
		Type: "string",
		Enum: []any{"pending-review", "in-progress"},
	}
	got := generateTSEnum("TaskStatus", node)
	if !contains(got, `PendingReview = "pending-review"`) {
		t.Errorf("expected PendingReview member, got:\n%s", got)
	}
	if !contains(got, `InProgress = "in-progress"`) {
		t.Errorf("expected InProgress member, got:\n%s", got)
	}
}

func TestGenerateTSEnum_DuplicateMemberNames(t *testing.T) {
	// "pending_review" and "PENDING-REVIEW" both PascalCase to "PendingReview"
	node := &SchemaNode{
		Type: "string",
		Enum: []any{"pending_review", "PENDING-REVIEW", "pending review"},
	}
	got := generateTSEnum("Status", node)
	// First occurrence keeps the name
	if !contains(got, `PendingReview = "pending_review"`) {
		t.Errorf("expected first PendingReview member, got:\n%s", got)
	}
	// Second gets a suffix
	if !contains(got, `PendingReview2 = "PENDING-REVIEW"`) {
		t.Errorf("expected PendingReview2 for duplicate, got:\n%s", got)
	}
	// Third gets a suffix
	if !contains(got, `PendingReview3 = "pending review"`) {
		t.Errorf("expected PendingReview3 for second duplicate, got:\n%s", got)
	}
}

func TestGenerateTSEnum_WithDescription(t *testing.T) {
	node := &SchemaNode{
		Type:        "string",
		Enum:        []any{"a", "b"},
		Description: "Letter enum",
	}
	got := generateTSEnum("Letter", node)
	if !contains(got, "/** Letter enum */") {
		t.Errorf("expected JSDoc, got:\n%s", got)
	}
}

func TestToEnumMemberName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"active", "Active"},
		{"ACTIVE", "Active"},
		{"pending-review", "PendingReview"},
		{"IN_PROGRESS", "InProgress"},
		{"404_NOT_FOUND", "Value404NotFound"},
		{"hello world", "HelloWorld"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toEnumMemberName(tt.input)
			if got != tt.want {
				t.Errorf("toEnumMemberName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- PrefixItems in renameSchemaRefs ---

func TestRenameSchemaRefs_PrefixItems(t *testing.T) {
	node := &SchemaNode{
		Type: "array",
		PrefixItems: []*SchemaNode{
			{Ref: "Array"},
			{Type: "string"},
		},
	}
	renames := map[string]string{"Array": "Array_"}
	renameSchemaRefs(node, renames)
	if node.PrefixItems[0].Ref != "Array_" {
		t.Errorf("expected PrefixItems[0].$ref = 'Array_', got %q", node.PrefixItems[0].Ref)
	}
}

// --- generateSharedTypes with tsEnums ---

func TestGenerateSharedTypes_WithTSEnums(t *testing.T) {
	schemas := map[string]*SchemaNode{
		"Status": {
			Type: "string",
			Enum: []any{"active", "inactive"},
		},
		"User": {
			Type: "object",
			Properties: map[string]*SchemaNode{
				"id": {Type: "string"},
			},
			Required: []string{"id"},
		},
	}
	got := generateSharedTypes(schemas, true)
	// Status should be an enum
	if !contains(got, "export enum Status") {
		t.Errorf("expected 'export enum Status', got:\n%s", got)
	}
	// User should still be an interface
	if !contains(got, "export interface User") {
		t.Errorf("expected 'export interface User', got:\n%s", got)
	}
}

func TestGenerateSharedTypes_WithoutTSEnums(t *testing.T) {
	schemas := map[string]*SchemaNode{
		"Status": {
			Type: "string",
			Enum: []any{"active", "inactive"},
		},
	}
	got := generateSharedTypes(schemas, false)
	// Should be a type alias, not an enum
	if !contains(got, "export type Status =") {
		t.Errorf("expected 'export type Status =', got:\n%s", got)
	}
	if contains(got, "export enum Status") {
		t.Errorf("should NOT contain 'export enum Status', got:\n%s", got)
	}
}

// --- Fingerprint Tests ---

func TestSchemaFingerprint_IncludesPropertyTypes(t *testing.T) {
	// Two schemas with the same property names but different types must produce
	// different fingerprints. Previously, only property names and required markers
	// were used, causing false matches (e.g., all paginated response wrappers matched).
	schemaA := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"items": {Type: "array", Items: &SchemaNode{Ref: "OrderResponse"}},
			"total": {Type: "number"},
		},
		Required: []string{"items", "total"},
	}
	schemaB := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"items": {Type: "array", Items: &SchemaNode{Ref: "ProductResponse"}},
			"total": {Type: "number"},
		},
		Required: []string{"items", "total"},
	}

	fpA := schemaFingerprint(schemaA)
	fpB := schemaFingerprint(schemaB)

	if fpA == fpB {
		t.Errorf("schemas with same property names but different item types should have different fingerprints:\n  A: %s\n  B: %s", fpA, fpB)
	}
}

func TestSchemaFingerprint_SameTypesSameFingerprint(t *testing.T) {
	// Two schemas with identical structures should produce the same fingerprint.
	schema1 := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id":   {Type: "string"},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}
	schema2 := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"id":   {Type: "string"},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}

	fp1 := schemaFingerprint(schema1)
	fp2 := schemaFingerprint(schema2)

	if fp1 != fp2 {
		t.Errorf("identical schemas should have same fingerprint:\n  1: %s\n  2: %s", fp1, fp2)
	}
}

func TestSchemaFingerprint_RefVsInline(t *testing.T) {
	// A property with a $ref should have a different fingerprint than
	// the same property with an inline type.
	schemaRef := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"user": {Ref: "UserResponse"},
		},
		Required: []string{"user"},
	}
	schemaInline := &SchemaNode{
		Type: "object",
		Properties: map[string]*SchemaNode{
			"user": {Type: "object"},
		},
		Required: []string{"user"},
	}

	fpRef := schemaFingerprint(schemaRef)
	fpInline := schemaFingerprint(schemaInline)

	if fpRef == fpInline {
		t.Errorf("$ref and inline object should have different fingerprints:\n  ref: %s\n  inline: %s", fpRef, fpInline)
	}
}
