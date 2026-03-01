package sdkgen

import (
	"strings"
	"testing"
)

func TestBuildResponseType_SSE_Typed(t *testing.T) {
	method := SDKMethod{
		ResponseContentType: "text/event-stream",
		SSEEventType:        "OrderUpdate",
	}
	got := buildResponseType(method)
	want := "SSEConnection<OrderUpdate>"
	if got != want {
		t.Errorf("buildResponseType() = %q, want %q", got, want)
	}
}

func TestBuildResponseType_SSE_Untyped(t *testing.T) {
	method := SDKMethod{
		ResponseContentType: "text/event-stream",
		SSEEventType:        "",
	}
	got := buildResponseType(method)
	want := "SSEConnection<string>"
	if got != want {
		t.Errorf("buildResponseType() = %q, want %q", got, want)
	}
}

func TestBuildResponseType_Void(t *testing.T) {
	method := SDKMethod{IsVoid: true}
	got := buildResponseType(method)
	if got != "void" {
		t.Errorf("buildResponseType() = %q, want %q", got, "void")
	}
}

func TestBuildResponseType_Regular(t *testing.T) {
	method := SDKMethod{ResponseType: "Order[]"}
	got := buildResponseType(method)
	if got != "Order[]" {
		t.Errorf("buildResponseType() = %q, want %q", got, "Order[]")
	}
}

func TestResponseTypeHint_AllBranches(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"text/event-stream", "sse"},
		{"text/plain", "text"},
		{"text/html", "text"},
		{"text/csv", "text"},
		{"application/pdf", "blob"},
		{"application/octet-stream", "blob"},
		{"image/png", "blob"},
		{"image/jpeg", "blob"},
		{"audio/mpeg", "blob"},
		{"video/mp4", "blob"},
		{"application/json", "json"},
		{"application/xml", "json"},
	}
	for _, tt := range tests {
		got := responseTypeHint(tt.contentType)
		if got != tt.want {
			t.Errorf("responseTypeHint(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}

func TestCollectRefs_SubstringFalsePositive(t *testing.T) {
	// Schema "Id" should match "OrderId" because collectRefs uses substring matching.
	// This documents the known behavior.
	schemas := map[string]*SchemaNode{
		"Id":      {Type: "string"},
		"OrderId": {Type: "string"},
	}
	refs := make(map[string]bool)
	collectRefs("OrderId", schemas, refs)
	// Both "Id" and "OrderId" match because "Id" is a substring of "OrderId"
	if !refs["Id"] {
		t.Error("expected Id to be matched (substring false positive)")
	}
	if !refs["OrderId"] {
		t.Error("expected OrderId to be matched")
	}
}

func TestCollectRefs_SSEEventType(t *testing.T) {
	schemas := map[string]*SchemaNode{
		"StreamEvent": {Type: "object"},
	}
	ctrl := ControllerGroup{
		Name: "EventsController",
		Methods: []SDKMethod{
			{
				Name:         "streamEvents",
				SSEEventType: "StreamEvent",
			},
		},
	}
	imports := collectTypeImports(ctrl, schemas)
	found := false
	for _, imp := range imports {
		if imp == "StreamEvent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected StreamEvent in type imports, got %v", imports)
	}
}

func TestCollectRefs_NoSchemas(t *testing.T) {
	schemas := map[string]*SchemaNode{}
	refs := make(map[string]bool)
	collectRefs("Order", schemas, refs)
	if len(refs) != 0 {
		t.Errorf("expected empty refs, got %v", refs)
	}
}

func TestHasSSEResponse(t *testing.T) {
	withSSE := ControllerGroup{
		Methods: []SDKMethod{
			{ResponseContentType: "application/json"},
			{ResponseContentType: "text/event-stream"},
		},
	}
	if !hasSSEResponse(withSSE) {
		t.Error("expected hasSSEResponse to return true")
	}

	withoutSSE := ControllerGroup{
		Methods: []SDKMethod{
			{ResponseContentType: "application/json"},
		},
	}
	if hasSSEResponse(withoutSSE) {
		t.Error("expected hasSSEResponse to return false")
	}
}

func TestBuildMethodJSDoc_DeprecatedOnly(t *testing.T) {
	method := SDKMethod{Deprecated: true}
	got := buildMethodJSDoc(method)
	if got != "  /** @deprecated */\n" {
		t.Errorf("expected single-line @deprecated JSDoc, got:\n%s", got)
	}
}

func TestBuildMethodJSDoc_SummaryEqualsDescription(t *testing.T) {
	method := SDKMethod{
		Summary:     "Get an item",
		Description: "Get an item",
	}
	got := buildMethodJSDoc(method)
	// When summary == description, description should be deduped
	if got != "  /** Get an item */\n" {
		t.Errorf("expected single-line JSDoc (deduped), got:\n%s", got)
	}
}

func TestBuildOptionsType_NoParams(t *testing.T) {
	method := SDKMethod{Name: "healthCheck"}
	got := buildOptionsType(method)
	if got != "" {
		t.Errorf("expected empty options type, got %q", got)
	}
}

func TestHasRequiredOptions_Combinations(t *testing.T) {
	tests := []struct {
		name   string
		method SDKMethod
		want   bool
	}{
		{
			name:   "path params",
			method: SDKMethod{PathParams: []SDKParam{{Name: "id", Required: true}}},
			want:   true,
		},
		{
			name:   "required body",
			method: SDKMethod{Body: &SDKBody{Required: true}},
			want:   true,
		},
		{
			name:   "required query",
			method: SDKMethod{QueryParams: []SDKParam{{Name: "status", Required: true}}},
			want:   true,
		},
		{
			name:   "optional body only",
			method: SDKMethod{Body: &SDKBody{Required: false}},
			want:   false,
		},
		{
			name:   "optional query only",
			method: SDKMethod{QueryParams: []SDKParam{{Name: "limit", Required: false}}},
			want:   false,
		},
		{
			name:   "no params",
			method: SDKMethod{},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasRequiredOptions(tt.method)
			if got != tt.want {
				t.Errorf("hasRequiredOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Fix 8: SDK request function override tests ---

func TestBuildOptionsTypeDecl_IncludesOverrideFields(t *testing.T) {
	method := SDKMethod{
		Name:       "getUser",
		HTTPMethod: "GET",
		Path:       "/users/{id}",
		PathParams: []SDKParam{
			{Name: "id", TSType: "string", Required: true},
		},
		ResponseType: "User",
	}

	decl := buildOptionsTypeDecl(method, "GetUserOptions")
	if decl == "" {
		t.Fatal("expected non-empty options type declaration")
	}

	// Should contain responseType override field
	if !strings.Contains(decl, "responseType?: 'json' | 'blob' | 'text' | 'stream'") {
		t.Error("options type should include responseType override field")
	}

	// Should contain contentType override field
	if !strings.Contains(decl, "contentType?: string") {
		t.Error("options type should include contentType override field")
	}

	// Should contain JSDoc comments for overrides
	if !strings.Contains(decl, "Override the response type handling") {
		t.Error("options type should include responseType JSDoc")
	}
	if !strings.Contains(decl, "Override the request content type") {
		t.Error("options type should include contentType JSDoc")
	}
}

func TestGenerateStandaloneFunction_OverridesInRequestCall(t *testing.T) {
	method := SDKMethod{
		Name:       "getUser",
		HTTPMethod: "GET",
		Path:       "/users/{id}",
		PathParams: []SDKParam{
			{Name: "id", TSType: "string", Required: true},
		},
		ResponseType: "User",
	}

	code := generateStandaloneFunction("UsersController", method)

	// Should spread responseType and contentType overrides into request options
	if !strings.Contains(code, "options?.responseType && { responseType: options.responseType }") {
		t.Error("standalone function should spread responseType override into request options")
	}
	if !strings.Contains(code, "options?.contentType && { contentType: options.contentType }") {
		t.Error("standalone function should spread contentType override into request options")
	}
}

func TestGenerateStandaloneFunction_NoParamsHasOverrides(t *testing.T) {
	// Method with no path/query/body params should still have override fields
	// in the inline options type
	method := SDKMethod{
		Name:                "healthCheck",
		HTTPMethod:          "GET",
		Path:                "/health",
		ResponseType:        "string",
		ResponseContentType: "application/json",
	}

	code := generateStandaloneFunction("HealthController", method)

	// The inline options type should include responseType and contentType
	if !strings.Contains(code, "responseType?: 'json' | 'blob' | 'text' | 'stream'") {
		t.Error("inline options type should include responseType override")
	}
	if !strings.Contains(code, "contentType?: string") {
		t.Error("inline options type should include contentType override")
	}
}

func TestGenerateController_InterfaceHasOverrides(t *testing.T) {
	ctrl := ControllerGroup{
		Name: "HealthController",
		Methods: []SDKMethod{
			{
				Name:                "check",
				HTTPMethod:          "GET",
				Path:                "/health",
				ResponseType:        "string",
				ResponseContentType: "application/json",
			},
		},
	}
	doc := &SDKDocument{Schemas: map[string]*SchemaNode{}}

	code := generateController(ctrl, doc, "")

	// Interface method signature for no-params method should have inline override fields
	if !strings.Contains(code, "responseType?: 'json' | 'blob' | 'text' | 'stream'") {
		t.Error("interface should include responseType in inline options type")
	}
}

func TestGenerateController_MixedSSEAndJSON(t *testing.T) {
	doc := &SDKDocument{
		Schemas: map[string]*SchemaNode{
			"Order":       {Type: "object", Properties: map[string]*SchemaNode{"id": {Type: "string"}}},
			"StreamEvent": {Type: "object", Properties: map[string]*SchemaNode{"type": {Type: "string"}}},
		},
	}
	ctrl := ControllerGroup{
		Name: "ItemsController",
		Methods: []SDKMethod{
			{
				Name:         "listItems",
				HTTPMethod:   "GET",
				Path:         "/items",
				ResponseType: "Order[]",
			},
			{
				Name:                "streamEvents",
				HTTPMethod:          "GET",
				Path:                "/events/stream",
				ResponseContentType: "text/event-stream",
				SSEEventType:        "StreamEvent",
			},
		},
	}
	output := generateController(ctrl, doc, "")

	// Should have SSEConnection import
	if !strings.Contains(output, "import { SSEConnection }") {
		t.Error("expected SSEConnection import in output")
	}
	// Should have type imports for both Order and StreamEvent
	if !strings.Contains(output, "Order") {
		t.Error("expected Order type import")
	}
	if !strings.Contains(output, "StreamEvent") {
		t.Error("expected StreamEvent type import")
	}
	// Should have SSEConnection<StreamEvent> in response type
	if !strings.Contains(output, "SSEConnection<StreamEvent>") {
		t.Error("expected SSEConnection<StreamEvent> response type")
	}
}
