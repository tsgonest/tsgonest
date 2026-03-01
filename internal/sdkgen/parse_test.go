package sdkgen

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestParseOpenAPI_Basic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sdkgen/basic.openapi.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	doc, err := ParseOpenAPIBytes(data)
	if err != nil {
		t.Fatalf("ParseOpenAPIBytes: %v", err)
	}

	// Should have 1 version group (unversioned)
	if len(doc.Versions) != 1 {
		t.Fatalf("expected 1 version group, got %d", len(doc.Versions))
	}
	ver := doc.Versions[0]
	if ver.Version != "" {
		t.Errorf("expected empty version, got %q", ver.Version)
	}

	// Should have 2 controllers: Orders and Products
	if len(ver.Controllers) != 2 {
		t.Fatalf("expected 2 controllers, got %d", len(ver.Controllers))
	}

	// Controllers should be sorted: OrdersController, ProductsController
	if ver.Controllers[0].Name != "OrdersController" {
		t.Errorf("expected first controller OrdersController, got %q", ver.Controllers[0].Name)
	}
	if ver.Controllers[1].Name != "ProductsController" {
		t.Errorf("expected second controller ProductsController, got %q", ver.Controllers[1].Name)
	}

	// OrdersController should have 4 methods
	orders := ver.Controllers[0]
	if len(orders.Methods) != 4 {
		t.Fatalf("expected 4 methods in OrdersController, got %d", len(orders.Methods))
	}

	// Check listOrders method
	var listOrders *SDKMethod
	for i := range orders.Methods {
		if orders.Methods[i].Name == "listOrders" {
			listOrders = &orders.Methods[i]
			break
		}
	}
	if listOrders == nil {
		t.Fatal("expected listOrders method")
	}
	if listOrders.HTTPMethod != "GET" {
		t.Errorf("expected GET, got %q", listOrders.HTTPMethod)
	}
	if len(listOrders.QueryParams) != 2 {
		t.Errorf("expected 2 query params, got %d", len(listOrders.QueryParams))
	}
	if listOrders.ResponseType != "Order[]" {
		t.Errorf("expected response type Order[], got %q", listOrders.ResponseType)
	}

	// Check deleteOrder — should be void
	var deleteOrder *SDKMethod
	for i := range orders.Methods {
		if orders.Methods[i].Name == "deleteOrder" {
			deleteOrder = &orders.Methods[i]
			break
		}
	}
	if deleteOrder == nil {
		t.Fatal("expected deleteOrder method")
	}
	if !deleteOrder.IsVoid {
		t.Error("expected deleteOrder to be void")
	}
	if deleteOrder.ResponseStatus != 204 {
		t.Errorf("expected status 204, got %d", deleteOrder.ResponseStatus)
	}
	if len(deleteOrder.PathParams) != 1 {
		t.Errorf("expected 1 path param, got %d", len(deleteOrder.PathParams))
	}

	// Check createOrder — should have body
	var createOrder *SDKMethod
	for i := range orders.Methods {
		if orders.Methods[i].Name == "createOrder" {
			createOrder = &orders.Methods[i]
			break
		}
	}
	if createOrder == nil {
		t.Fatal("expected createOrder method")
	}
	if createOrder.Body == nil {
		t.Fatal("expected createOrder to have body")
	}
	if createOrder.Body.TSType != "CreateOrderDto" {
		t.Errorf("expected body type CreateOrderDto, got %q", createOrder.Body.TSType)
	}
	if !createOrder.Body.Required {
		t.Error("expected body to be required")
	}

	// Should have 4 schemas
	if len(doc.Schemas) != 4 {
		t.Errorf("expected 4 schemas, got %d", len(doc.Schemas))
	}
}

func TestParseOpenAPI_NoExtensions(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sdkgen/no-extensions.openapi.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	doc, err := ParseOpenAPIBytes(data)
	if err != nil {
		t.Fatalf("ParseOpenAPIBytes: %v", err)
	}

	// Without extensions, should fall back to tag-based grouping
	if len(doc.Versions) != 1 {
		t.Fatalf("expected 1 version group, got %d", len(doc.Versions))
	}

	ver := doc.Versions[0]
	// Should have 2 controllers: UsersController (from tag) and PostsController (from path)
	if len(ver.Controllers) < 2 {
		t.Fatalf("expected at least 2 controllers, got %d", len(ver.Controllers))
	}

	// Check that controllers have reasonable names
	names := make(map[string]bool)
	for _, ctrl := range ver.Controllers {
		names[ctrl.Name] = true
	}
	if !names["UsersController"] {
		t.Error("expected UsersController from tag fallback")
	}

	// The posts endpoint has no tag and no operationId — should use path segment
	if !names["PostsController"] {
		t.Error("expected PostsController from path segment fallback")
	}
}

func TestParseOpenAPI_Versioned(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sdkgen/versioned.openapi.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	doc, err := ParseOpenAPIBytes(data)
	if err != nil {
		t.Fatalf("ParseOpenAPIBytes: %v", err)
	}

	// Should have 3 version groups: "", "v1", "v2"
	if len(doc.Versions) != 3 {
		t.Fatalf("expected 3 version groups, got %d", len(doc.Versions))
	}

	// First should be unversioned (empty string sorts first)
	if doc.Versions[0].Version != "" {
		t.Errorf("expected first version to be empty, got %q", doc.Versions[0].Version)
	}
	if doc.Versions[1].Version != "v1" {
		t.Errorf("expected second version to be v1, got %q", doc.Versions[1].Version)
	}
	if doc.Versions[2].Version != "v2" {
		t.Errorf("expected third version to be v2, got %q", doc.Versions[2].Version)
	}

	// Unversioned should have HealthController
	unversioned := doc.Versions[0]
	if len(unversioned.Controllers) != 1 {
		t.Fatalf("expected 1 unversioned controller, got %d", len(unversioned.Controllers))
	}
	if unversioned.Controllers[0].Name != "HealthController" {
		t.Errorf("expected HealthController, got %q", unversioned.Controllers[0].Name)
	}

	// v1 should have OrdersController
	v1 := doc.Versions[1]
	if len(v1.Controllers) != 1 {
		t.Fatalf("expected 1 v1 controller, got %d", len(v1.Controllers))
	}
	if v1.Controllers[0].Name != "OrdersController" {
		t.Errorf("expected OrdersController in v1, got %q", v1.Controllers[0].Name)
	}

	// v2 should have OrdersController
	v2 := doc.Versions[2]
	if len(v2.Controllers) != 1 {
		t.Fatalf("expected 1 v2 controller, got %d", len(v2.Controllers))
	}
	if v2.Controllers[0].Name != "OrdersController" {
		t.Errorf("expected OrdersController in v2, got %q", v2.Controllers[0].Name)
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		path    string
		version string
	}{
		{"/v1/orders", "v1"},
		{"/v2/orders/{id}", "v2"},
		{"/v10/items", "v10"},
		{"/orders", ""},
		{"/api/v1/orders", ""},    // v1 not at start
		{"/version/1/orders", ""}, // not /vN/ format
	}

	for _, tt := range tests {
		got := extractVersion(tt.path)
		if got != tt.version {
			t.Errorf("extractVersion(%q) = %q, want %q", tt.path, got, tt.version)
		}
	}
}

func TestResolveControllerName_Fallbacks(t *testing.T) {
	// With extension
	op := openAPIOperation{XTsgonestController: "MyController"}
	if name := resolveControllerName(op, "/users", ""); name != "MyController" {
		t.Errorf("expected MyController, got %q", name)
	}

	// With tag
	op = openAPIOperation{Tags: []string{"Users"}}
	if name := resolveControllerName(op, "/users", ""); name != "UsersController" {
		t.Errorf("expected UsersController, got %q", name)
	}

	// With path segment (no version)
	op = openAPIOperation{}
	if name := resolveControllerName(op, "/orders/123", ""); name != "OrdersController" {
		t.Errorf("expected OrdersController, got %q", name)
	}

	// With path segment (versioned — should skip version prefix)
	op = openAPIOperation{}
	if name := resolveControllerName(op, "/v1/products", "v1"); name != "ProductsController" {
		t.Errorf("expected ProductsController, got %q", name)
	}
}

func TestParseOpenAPIBytes_MalformedJSON(t *testing.T) {
	_, err := ParseOpenAPIBytes([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseOpenAPIBytes_EmptyPaths(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sdkgen/empty.openapi.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	doc, err := ParseOpenAPIBytes(data)
	if err != nil {
		t.Fatalf("ParseOpenAPIBytes: %v", err)
	}

	// Empty paths should produce no versions/controllers
	if len(doc.Versions) != 0 {
		t.Errorf("expected 0 version groups, got %d", len(doc.Versions))
	}
	if len(doc.Schemas) != 0 {
		t.Errorf("expected 0 schemas, got %d", len(doc.Schemas))
	}
}

func TestResolveResponse_No2xxResponses(t *testing.T) {
	responses := map[string]*openAPIResponse{
		"400": {Description: "Bad Request"},
		"500": {Description: "Internal Server Error"},
	}
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	resp := resolveResponse(responses, resolver)
	if !resp.isVoid {
		t.Error("expected void when no 2xx responses")
	}
	if resp.tsType != "void" {
		t.Errorf("expected tsType void, got %q", resp.tsType)
	}
}

func TestResolveResponse_204NoContent(t *testing.T) {
	responses := map[string]*openAPIResponse{
		"204": {Description: "No Content"},
	}
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	resp := resolveResponse(responses, resolver)
	if !resp.isVoid {
		t.Error("expected void for 204")
	}
	if resp.status != 204 {
		t.Errorf("expected status 204, got %d", resp.status)
	}
}

func TestResolveResponse_Multiple2xx(t *testing.T) {
	// When both 200 and 201 exist, 200 should be picked (priority order)
	responses := map[string]*openAPIResponse{
		"200": {
			Description: "OK",
			Content: map[string]openAPIMediaType{
				"application/json": {
					Schema: json.RawMessage(`{"$ref": "#/components/schemas/Item"}`),
				},
			},
		},
		"201": {
			Description: "Created",
			Content: map[string]openAPIMediaType{
				"application/json": {
					Schema: json.RawMessage(`{"$ref": "#/components/schemas/Item"}`),
				},
			},
		},
	}
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	resp := resolveResponse(responses, resolver)
	if resp.status != 200 {
		t.Errorf("expected status 200 (priority), got %d", resp.status)
	}
	if resp.tsType != "Item" {
		t.Errorf("expected tsType Item, got %q", resp.tsType)
	}
}

func TestContentTypeToTSType_AllBranches(t *testing.T) {
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	tests := []struct {
		contentType string
		media       openAPIMediaType
		want        string
	}{
		{"application/json", openAPIMediaType{Schema: json.RawMessage(`{"type": "string"}`)}, "string"},
		{"text/event-stream", openAPIMediaType{}, "ReadableStream<Uint8Array>"},
		{"text/plain", openAPIMediaType{}, "string"},
		{"text/html", openAPIMediaType{}, "string"},
		{"text/csv", openAPIMediaType{}, "string"},
		{"application/pdf", openAPIMediaType{}, "Blob"},
		{"application/octet-stream", openAPIMediaType{}, "Blob"},
		{"image/png", openAPIMediaType{}, "Blob"},
		{"audio/mpeg", openAPIMediaType{}, "Blob"},
		{"video/mp4", openAPIMediaType{}, "Blob"},
		{"multipart/form-data", openAPIMediaType{}, "FormData"},
		// Unknown with schema
		{"application/xml", openAPIMediaType{Schema: json.RawMessage(`{"type": "string"}`)}, "string"},
		// Unknown without schema
		{"application/xml", openAPIMediaType{}, "Blob"},
	}
	for _, tt := range tests {
		got := contentTypeToTSType(tt.contentType, tt.media, resolver)
		if got != tt.want {
			t.Errorf("contentTypeToTSType(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}

func TestExtractSSEEventType_NoItemSchema(t *testing.T) {
	responses := map[string]*openAPIResponse{
		"200": {
			Content: map[string]openAPIMediaType{
				"text/event-stream": {
					Schema: json.RawMessage(`{"type": "string"}`),
					// No ItemSchema
				},
			},
		},
	}
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	got := extractSSEEventType(responses, resolver)
	if got != "" {
		t.Errorf("expected empty string for no itemSchema, got %q", got)
	}
}

func TestExtractSSEEventType_MalformedItemSchema(t *testing.T) {
	responses := map[string]*openAPIResponse{
		"200": {
			Content: map[string]openAPIMediaType{
				"text/event-stream": {
					Schema:     json.RawMessage(`{"type": "string"}`),
					ItemSchema: json.RawMessage(`{not valid json`),
				},
			},
		},
	}
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	got := extractSSEEventType(responses, resolver)
	if got != "" {
		t.Errorf("expected empty string for malformed itemSchema, got %q", got)
	}
}

func TestExtractSSEEventType_InlineSchema(t *testing.T) {
	responses := map[string]*openAPIResponse{
		"200": {
			Content: map[string]openAPIMediaType{
				"text/event-stream": {
					Schema: json.RawMessage(`{"type": "string"}`),
					ItemSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"data": {
								"type": "string",
								"contentSchema": {"type": "object", "properties": {"msg": {"type": "string"}}}
							}
						}
					}`),
				},
			},
		},
	}
	resolver := &schemaResolver{schemas: map[string]*SchemaNode{}}
	resolver.initFingerprints()

	got := extractSSEEventType(responses, resolver)
	// Inline schema without $ref should produce an inline object type
	if got == "" {
		t.Error("expected non-empty result for inline contentSchema")
	}
}

// --- Fingerprint tests ---

// TestRawPropFingerprint_InlineObjectsAreDifferent verifies that two inline
// objects with different properties produce different fingerprints. This prevents
// paginated response wrappers from collapsing to the same named type.
func TestRawPropFingerprint_InlineObjectsAreDifferent(t *testing.T) {
	obj1 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "string"},
			"name": {"type": "string"}
		},
		"required": ["id", "name"]
	}`)
	obj2 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "string"},
			"email": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["id", "email", "age"]
	}`)

	fp1 := rawPropFingerprint(obj1)
	fp2 := rawPropFingerprint(obj2)

	if fp1 == fp2 {
		t.Errorf("inline objects with different properties should have different fingerprints\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
	// Both should contain "object{" indicating they recurse into properties
	if fp1 == "object" {
		t.Errorf("inline object fingerprint should recurse into properties, got bare 'object'")
	}
}

// TestRawPropFingerprint_IdenticalInlineObjectsMatch verifies that two inline
// objects with the same structure produce the same fingerprint.
func TestRawPropFingerprint_IdenticalInlineObjectsMatch(t *testing.T) {
	obj1 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "string"},
			"name": {"type": "string"}
		},
		"required": ["id", "name"]
	}`)
	obj2 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"id": {"type": "string"}
		},
		"required": ["name", "id"]
	}`)

	fp1 := rawPropFingerprint(obj1)
	fp2 := rawPropFingerprint(obj2)

	if fp1 != fp2 {
		t.Errorf("identical inline objects (different key order) should have same fingerprint\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
}

// TestRawPropFingerprint_ObjectWithoutProperties verifies that inline objects
// without a "properties" key don't recurse (e.g., Record<string, unknown>).
func TestRawPropFingerprint_ObjectWithoutProperties(t *testing.T) {
	obj := json.RawMessage(`{
		"type": "object",
		"additionalProperties": {"type": "string"}
	}`)

	fp := rawPropFingerprint(obj)
	if fp != "object" {
		t.Errorf("object without properties should fingerprint as 'object', got %q", fp)
	}
}

// TestRawSchemaFingerprint_PaginatedWithDifferentItems verifies the real-world
// case: two paginated response schemas with identical wrapper properties but
// different inline item types must produce different fingerprints.
func TestRawSchemaFingerprint_PaginatedWithDifferentItems(t *testing.T) {
	// PaginatedResponse wrapping Model items
	paginatedModels := map[string]json.RawMessage{
		"type": json.RawMessage(`"object"`),
		"properties": json.RawMessage(`{
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"name": {"type": "string"},
						"apiKey": {"type": "string"}
					},
					"required": ["id", "name", "apiKey"]
				}
			},
			"totalCount": {"type": "number"},
			"currentPage": {"type": "number"}
		}`),
		"required": json.RawMessage(`["items", "totalCount", "currentPage"]`),
	}
	// PaginatedResponse wrapping Scenario items
	paginatedScenarios := map[string]json.RawMessage{
		"type": json.RawMessage(`"object"`),
		"properties": json.RawMessage(`{
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"category": {"type": "string"},
						"datasetId": {"type": "string"}
					},
					"required": ["id", "category", "datasetId"]
				}
			},
			"totalCount": {"type": "number"},
			"currentPage": {"type": "number"}
		}`),
		"required": json.RawMessage(`["items", "totalCount", "currentPage"]`),
	}

	fp1 := rawSchemaFingerprint(paginatedModels)
	fp2 := rawSchemaFingerprint(paginatedScenarios)

	if fp1 == fp2 {
		t.Errorf("paginated schemas with different inline item types should have different fingerprints\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
}

// TestRawSchemaFingerprint_PaginatedWithRefItems verifies that paginated
// schemas using $ref for items produce different fingerprints per ref target.
func TestRawSchemaFingerprint_PaginatedWithRefItems(t *testing.T) {
	paginatedUsers := map[string]json.RawMessage{
		"type": json.RawMessage(`"object"`),
		"properties": json.RawMessage(`{
			"items": {
				"type": "array",
				"items": {"$ref": "#/components/schemas/UserResponse"}
			},
			"totalCount": {"type": "number"}
		}`),
		"required": json.RawMessage(`["items", "totalCount"]`),
	}
	paginatedOrders := map[string]json.RawMessage{
		"type": json.RawMessage(`"object"`),
		"properties": json.RawMessage(`{
			"items": {
				"type": "array",
				"items": {"$ref": "#/components/schemas/OrderResponse"}
			},
			"totalCount": {"type": "number"}
		}`),
		"required": json.RawMessage(`["items", "totalCount"]`),
	}

	fp1 := rawSchemaFingerprint(paginatedUsers)
	fp2 := rawSchemaFingerprint(paginatedOrders)

	if fp1 == fp2 {
		t.Errorf("paginated schemas with different $ref items should have different fingerprints\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
}

// TestSchemaResolver_InlinePaginatedNotCollapsed verifies end-to-end that two
// operations returning paginated responses with different inline item types
// do NOT get collapsed to the same named type via fingerprint matching.
func TestSchemaResolver_InlinePaginatedNotCollapsed(t *testing.T) {
	// Register one named paginated type (ListModelsResponse) in component schemas
	schemas := map[string]*SchemaNode{
		"ListModelsResponse": {
			Type: "object",
			Properties: map[string]*SchemaNode{
				"items": {
					Type: "array",
					Items: &SchemaNode{
						Type: "object",
						Properties: map[string]*SchemaNode{
							"id":     {Type: "string"},
							"name":   {Type: "string"},
							"apiKey": {Type: "string"},
						},
						Required: []string{"id", "name", "apiKey"},
					},
				},
				"totalCount":  {Type: "number"},
				"currentPage": {Type: "number"},
			},
			Required: []string{"items", "totalCount", "currentPage"},
		},
	}

	resolver := &schemaResolver{schemas: schemas}
	resolver.initFingerprints()

	// Inline schema that matches ListModelsResponse exactly
	matchingInline := json.RawMessage(`{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"name": {"type": "string"},
						"apiKey": {"type": "string"}
					},
					"required": ["id", "name", "apiKey"]
				}
			},
			"totalCount": {"type": "number"},
			"currentPage": {"type": "number"}
		},
		"required": ["items", "totalCount", "currentPage"]
	}`)

	// Inline schema with different item properties (scenarios, not models)
	differentInline := json.RawMessage(`{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"category": {"type": "string"},
						"datasetId": {"type": "string"}
					},
					"required": ["id", "category", "datasetId"]
				}
			},
			"totalCount": {"type": "number"},
			"currentPage": {"type": "number"}
		},
		"required": ["items", "totalCount", "currentPage"]
	}`)

	// The matching inline should resolve to ListModelsResponse
	matchResult := resolver.schemaToTS(matchingInline)
	if matchResult != "ListModelsResponse" {
		t.Errorf("inline schema matching ListModelsResponse should resolve to it, got %q", matchResult)
	}

	// The different inline should NOT resolve to ListModelsResponse
	diffResult := resolver.schemaToTS(differentInline)
	if diffResult == "ListModelsResponse" {
		t.Errorf("inline schema with different item types should NOT resolve to ListModelsResponse, but it did")
	}
	// It should be an inline object type (not a named ref)
	if diffResult == "" || diffResult == "unknown" {
		t.Errorf("expected inline object type, got %q", diffResult)
	}
}

// TestRawPropFingerprint_SameNamesDifferentTypes verifies that two inline
// objects sharing property names but with different types are distinguished.
func TestRawPropFingerprint_SameNamesDifferentTypes(t *testing.T) {
	obj1 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"value": {"type": "string"},
			"count": {"type": "number"}
		},
		"required": ["value", "count"]
	}`)
	obj2 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"value": {"type": "number"},
			"count": {"type": "string"}
		},
		"required": ["value", "count"]
	}`)

	fp1 := rawPropFingerprint(obj1)
	fp2 := rawPropFingerprint(obj2)

	if fp1 == fp2 {
		t.Errorf("objects with same names but different types should differ\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
}

// TestRawPropFingerprint_NestedInlineObjects verifies that deeply nested
// inline objects are fingerprinted recursively.
func TestRawPropFingerprint_NestedInlineObjects(t *testing.T) {
	obj1 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"meta": {
				"type": "object",
				"properties": {
					"key": {"type": "string"}
				},
				"required": ["key"]
			}
		},
		"required": ["meta"]
	}`)
	obj2 := json.RawMessage(`{
		"type": "object",
		"properties": {
			"meta": {
				"type": "object",
				"properties": {
					"value": {"type": "number"}
				},
				"required": ["value"]
			}
		},
		"required": ["meta"]
	}`)

	fp1 := rawPropFingerprint(obj1)
	fp2 := rawPropFingerprint(obj2)

	if fp1 == fp2 {
		t.Errorf("nested objects with different inner properties should differ\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
}

// TestRawPropFingerprint_RefVsInline verifies that a $ref property and an
// inline object property produce different fingerprints.
func TestRawPropFingerprint_RefVsInline(t *testing.T) {
	ref := json.RawMessage(`{"$ref": "#/components/schemas/UserDto"}`)
	inline := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "string"},
			"name": {"type": "string"}
		},
		"required": ["id", "name"]
	}`)

	fpRef := rawPropFingerprint(ref)
	fpInline := rawPropFingerprint(inline)

	if fpRef == fpInline {
		t.Errorf("$ref and inline object should have different fingerprints\n  ref: %s\n  inline: %s", fpRef, fpInline)
	}
	if fpRef != "$UserDto" {
		t.Errorf("expected $ref fingerprint '$UserDto', got %q", fpRef)
	}
}

// TestRawPropFingerprint_ArrayOfRefVsArrayOfInline verifies that array[Ref]
// and array[inline object] produce different fingerprints.
func TestRawPropFingerprint_ArrayOfRefVsArrayOfInline(t *testing.T) {
	arrRef := json.RawMessage(`{
		"type": "array",
		"items": {"$ref": "#/components/schemas/Item"}
	}`)
	arrInline := json.RawMessage(`{
		"type": "array",
		"items": {
			"type": "object",
			"properties": {
				"id": {"type": "string"}
			},
			"required": ["id"]
		}
	}`)

	fp1 := rawPropFingerprint(arrRef)
	fp2 := rawPropFingerprint(arrInline)

	if fp1 == fp2 {
		t.Errorf("array of $ref and array of inline object should differ\n  fp1: %s\n  fp2: %s", fp1, fp2)
	}
}

// --- Fix 4/7: prefix stripping and versioning tests ---

func TestStripGlobalPrefix(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   string
	}{
		{"/api/v1/users", "api", "/v1/users"},
		{"/api/v1/users", "/api", "/v1/users"},
		{"/api/v1/users", "/api/", "/v1/users"},
		{"/my-api/v1/users", "my-api", "/v1/users"},
		{"/v1/users", "api", "/v1/users"},       // no match, return unchanged
		{"/api", "api", "/"},                     // exact prefix match
		{"/api-extra/v1/users", "api", "/api-extra/v1/users"}, // prefix must match full segment
		{"/users", "", "/users"},                 // empty prefix
	}

	for _, tt := range tests {
		got := stripGlobalPrefix(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("stripGlobalPrefix(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestExtractVersionWithRe(t *testing.T) {
	// Default "v" prefix regex
	defaultRe := versionPrefixRe

	tests := []struct {
		path    string
		prefix  string
		version string
	}{
		{"/v1/orders", "v", "v1"},
		{"/v2/orders/{id}", "v", "v2"},
		{"/v10/items", "v", "v10"},
		{"/orders", "v", ""},
		{"/version/1/orders", "v", ""},
	}

	for _, tt := range tests {
		got := extractVersionWithRe(tt.path, defaultRe, tt.prefix)
		if got != tt.version {
			t.Errorf("extractVersionWithRe(%q, defaultRe, %q) = %q, want %q", tt.path, tt.prefix, got, tt.version)
		}
	}

	// Custom version prefix "ver"
	customRe := regexp.MustCompile(`^/ver(\d+)(/|$)`)
	customTests := []struct {
		path    string
		version string
	}{
		{"/ver1/orders", "ver1"},
		{"/ver2/orders", "ver2"},
		{"/v1/orders", ""},  // doesn't match custom prefix
	}

	for _, tt := range customTests {
		got := extractVersionWithRe(tt.path, customRe, "ver")
		if got != tt.version {
			t.Errorf("extractVersionWithRe(%q, customRe, \"ver\") = %q, want %q", tt.path, got, tt.version)
		}
	}
}

func TestParseOpenAPIBytesWithOptions_GlobalPrefix(t *testing.T) {
	// Build a minimal OpenAPI doc with paths prefixed by /api
	doc := `{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/api/v1/users": {
				"get": {
					"operationId": "getUsers",
					"x-tsgonest-controller": "UsersController",
					"responses": {
						"200": {
							"description": "OK",
							"content": {"application/json": {"schema": {"type": "array", "items": {"type": "string"}}}}
						}
					}
				}
			},
			"/api/v2/users": {
				"get": {
					"operationId": "getUsersV2",
					"x-tsgonest-controller": "UsersController",
					"responses": {
						"200": {
							"description": "OK",
							"content": {"application/json": {"schema": {"type": "string"}}}
						}
					}
				}
			}
		}
	}`

	// Without options: /api/v1/users -> version not detected (v1 not at start after /api prefix)
	sdkDoc, err := ParseOpenAPIBytes([]byte(doc))
	if err != nil {
		t.Fatalf("ParseOpenAPIBytes: %v", err)
	}
	// Without prefix stripping, there should be 1 unversioned group
	hasVersioned := false
	for _, v := range sdkDoc.Versions {
		if v.Version != "" {
			hasVersioned = true
		}
	}
	if hasVersioned {
		t.Error("without globalPrefix option, expected no versioned groups (v1 not at start)")
	}

	// With globalPrefix: should properly detect v1 and v2
	sdkDoc2, err := ParseOpenAPIBytesWithOptions([]byte(doc), &GenerateOptions{GlobalPrefix: "api"})
	if err != nil {
		t.Fatalf("ParseOpenAPIBytesWithOptions: %v", err)
	}

	versions := make(map[string]bool)
	for _, v := range sdkDoc2.Versions {
		versions[v.Version] = true
	}
	if !versions["v1"] {
		t.Error("with globalPrefix 'api', expected v1 version group")
	}
	if !versions["v2"] {
		t.Error("with globalPrefix 'api', expected v2 version group")
	}

	// Verify SDK method paths have prefix stripped
	for _, ver := range sdkDoc2.Versions {
		for _, ctrl := range ver.Controllers {
			for _, m := range ctrl.Methods {
				if strings.HasPrefix(m.Path, "/api") {
					t.Errorf("SDK method path should have prefix stripped, got %q", m.Path)
				}
			}
		}
	}
}

func TestParseOpenAPIBytesWithOptions_OpenAPIExtensions(t *testing.T) {
	// OpenAPI doc with x-tsgonest-global-prefix in info
	doc := `{
		"openapi": "3.1.0",
		"info": {
			"title": "Test",
			"version": "1.0.0",
			"x-tsgonest-global-prefix": "api",
			"x-tsgonest-version-prefix": "v"
		},
		"paths": {
			"/api/v1/items": {
				"get": {
					"operationId": "getItems",
					"x-tsgonest-controller": "ItemsController",
					"responses": {
						"200": {
							"description": "OK",
							"content": {"application/json": {"schema": {"type": "string"}}}
						}
					}
				}
			}
		}
	}`

	// Parse without explicit options — should pick up extensions from info
	sdkDoc, err := ParseOpenAPIBytesWithOptions([]byte(doc), nil)
	if err != nil {
		t.Fatalf("ParseOpenAPIBytesWithOptions: %v", err)
	}

	hasV1 := false
	for _, v := range sdkDoc.Versions {
		if v.Version == "v1" {
			hasV1 = true
		}
	}
	if !hasV1 {
		t.Error("expected v1 version group from x-tsgonest-global-prefix extension")
	}
}

func TestResolveMethodName_Synthesis(t *testing.T) {
	// No operationId or x-tsgonest-method → synthesize from method+path
	op := openAPIOperation{}
	got := resolveMethodName(op, "GET", "/users/{id}")
	if got != "getUsers_id" {
		t.Errorf("expected synthesized name getUsers_id, got %q", got)
	}

	got = resolveMethodName(op, "POST", "/items")
	if got != "postItems" {
		t.Errorf("expected synthesized name postItems, got %q", got)
	}

	// With operationId, should use it directly
	op = openAPIOperation{OperationID: "createUser"}
	got = resolveMethodName(op, "POST", "/users")
	if got != "createUser" {
		t.Errorf("expected createUser, got %q", got)
	}
}
