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

func TestGenerateStandaloneFunction_SSETyped_UsesJSONParse(t *testing.T) {
	method := SDKMethod{
		Name:                "streamOrders",
		HTTPMethod:          "GET",
		Path:                "/orders/stream",
		ResponseContentType: "text/event-stream",
		SSEEventType:        "OrderUpdate",
	}

	code := generateStandaloneFunction("OrdersController", method)

	// Should use JSON.parse for typed SSE
	if !strings.Contains(code, "JSON.parse(s)") {
		t.Error("typed SSE should use JSON.parse parser")
	}
	// Should have SSEConnection<OrderUpdate>
	if !strings.Contains(code, "SSEConnection<OrderUpdate>") {
		t.Error("expected SSEConnection<OrderUpdate>")
	}
	// responseType should be 'sse' (not 'sse-raw')
	if strings.Contains(code, "sse-raw") {
		t.Error("typed SSE should use responseType 'sse', not 'sse-raw'")
	}
}

func TestGenerateStandaloneFunction_SSEUntyped_UsesRawIdentity(t *testing.T) {
	method := SDKMethod{
		Name:                "streamRaw",
		HTTPMethod:          "GET",
		Path:                "/raw/stream",
		ResponseContentType: "text/event-stream",
		SSEEventType:        "", // untyped
	}

	code := generateStandaloneFunction("RawController", method)

	// Should use identity parser for untyped SSE
	if !strings.Contains(code, "(s: string) => s") {
		t.Error("untyped SSE should use identity parser")
	}
	// Should have SSEConnection<string>
	if !strings.Contains(code, "SSEConnection<string>") {
		t.Error("expected SSEConnection<string>")
	}
	// Should NOT use JSON.parse
	if strings.Contains(code, "JSON.parse") {
		t.Error("untyped SSE should not use JSON.parse")
	}
	// responseType should be 'sse-raw'
	if !strings.Contains(code, "sse-raw") {
		t.Error("untyped SSE should use responseType 'sse-raw'")
	}
}

func TestGenerateStandaloneFunction_SSEUnionType_UsesJSONParse(t *testing.T) {
	// When @EventStream has multiple typed variants, SSEEventType is a union
	method := SDKMethod{
		Name:                "streamEvents",
		HTTPMethod:          "GET",
		Path:                "/events/stream",
		ResponseContentType: "text/event-stream",
		SSEEventType:        "OrderCreated | OrderUpdated",
	}

	code := generateStandaloneFunction("EventsController", method)

	// Should use JSON.parse for typed union SSE
	if !strings.Contains(code, "JSON.parse(s)") {
		t.Error("typed union SSE should use JSON.parse parser")
	}
	// Should have SSEConnection<OrderCreated | OrderUpdated>
	if !strings.Contains(code, "SSEConnection<OrderCreated | OrderUpdated>") {
		t.Error("expected SSEConnection<OrderCreated | OrderUpdated>")
	}
}

// --- Client: RequestContext, hooks, timeout, extractErrorMessage ---

func TestGenerateClient_RequestContextInterface(t *testing.T) {
	code := generateClient()
	assertContains(t, code, "export interface RequestContext {", "client.ts should have RequestContext interface")
	assertContains(t, code, "method: string;", "RequestContext should have method field")
	assertContains(t, code, "path: string;", "RequestContext should have path field")
	assertContains(t, code, "url: string;", "RequestContext should have url field")
}

func TestGenerateClient_HookTypeSignatures(t *testing.T) {
	code := generateClient()
	assertContains(t, code, "onResponse?: (response: Response, context: RequestContext) => Response | void | Promise<Response | void>", "should have onResponse hook signature")
	assertContains(t, code, "onError?: (error: SDKError, context: RequestContext) => SDKError | void | Promise<SDKError | void>", "should have onError hook signature")
}

func TestGenerateClient_TimeoutConfig(t *testing.T) {
	code := generateClient()
	assertContains(t, code, "timeout?: number;", "ClientConfig should have timeout field")
	assertContains(t, code, "AbortSignal.timeout(config.timeout)", "should use AbortSignal.timeout")
	assertContains(t, code, "AbortSignal.any([init.signal, timeoutSignal])", "should use AbortSignal.any to combine signals")
}

func TestGenerateClient_ThrowOnError(t *testing.T) {
	code := generateClient()
	assertContains(t, code, "throwOnError?: boolean;", "ClientConfig should have throwOnError field")
	assertContains(t, code, "if (config.throwOnError)", "should check throwOnError")
	assertContains(t, code, "throw error;", "should throw error when throwOnError is true")
}

func TestGenerateClient_ExtractErrorMessage(t *testing.T) {
	code := generateClient()
	assertContains(t, code, "export function extractErrorMessage(body: unknown, fallback: string): string", "should export extractErrorMessage")
	// NestJS default: body.message (string)
	assertContains(t, code, "typeof b.message === 'string'", "should check body.message string")
	// NestJS validation pipes: body.message[0] (array)
	assertContains(t, code, "Array.isArray(b.message)", "should check body.message array")
	// Express nested: body.error.message
	assertContains(t, code, "nested.message", "should check body.error.message")
	// Simple: body.error (string)
	assertContains(t, code, "typeof b.error === 'string'", "should check body.error string")
}

func TestGenerateClient_ExecutionOrder(t *testing.T) {
	code := generateClient()

	// onResponse must fire before !response.ok check
	onResponseIdx := strings.Index(code, "config.onResponse")
	notOkIdx := strings.Index(code, "!response.ok")
	if onResponseIdx == -1 || notOkIdx == -1 {
		t.Fatal("expected both onResponse and !response.ok in output")
	}
	if onResponseIdx >= notOkIdx {
		t.Error("onResponse hook should fire before !response.ok check")
	}

	// extractErrorMessage must fire before onError
	extractIdx := strings.Index(code, "extractErrorMessage(errorBody")
	onErrorIdx := strings.Index(code, "config.onError")
	if extractIdx == -1 || onErrorIdx == -1 {
		t.Fatal("expected both extractErrorMessage and onError in output")
	}
	if extractIdx >= onErrorIdx {
		t.Error("extractErrorMessage should fire before onError hook")
	}

	// onError must fire before throwOnError
	throwIdx := strings.Index(code, "config.throwOnError")
	if throwIdx == -1 {
		t.Fatal("expected throwOnError in output")
	}
	if onErrorIdx >= throwIdx {
		t.Error("onError hook should fire before throwOnError check")
	}
}

func TestGenerateClient_RequestContextBuilt(t *testing.T) {
	code := generateClient()
	assertContains(t, code, "const context: RequestContext = { method, path, url: fullUrl }", "should build RequestContext with method, path, url")
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
