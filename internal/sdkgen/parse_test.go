package sdkgen

import (
	"encoding/json"
	"os"
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
