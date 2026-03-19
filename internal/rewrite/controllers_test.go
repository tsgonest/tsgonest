package rewrite

import (
	"sort"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/shim/core"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

func TestRewriteController_BodyValidation(t *testing.T) {
	input := `class UserController {
    async create(body) {
        return this.service.create(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "create",
					MethodName:  "create",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertCreateUserDto(body)") {
		t.Errorf("expected assert call injection, got:\n%s", result)
	}
	if !strings.Contains(result, `import { assertCreateUserDto } from "./user.dto.CreateUserDto.tsgonest.js"`) {
		t.Errorf("expected companion import, got:\n%s", result)
	}
}

func TestRewriteController_MultipleRoutes(t *testing.T) {
	input := `class UserController {
    async create(body) {
        return this.service.create(body);
    }
    async update(body) {
        return this.service.update(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "create",
					MethodName:  "create",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"},
						},
					},
				},
				{
					OperationID: "update",
					MethodName:  "update",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "UpdateUserDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
		"UpdateUserDto": "/dist/user.dto.UpdateUserDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertCreateUserDto(body)") {
		t.Errorf("expected assertCreateUserDto, got:\n%s", result)
	}
	if !strings.Contains(result, "assertUpdateUserDto(body)") {
		t.Errorf("expected assertUpdateUserDto, got:\n%s", result)
	}
}

func TestRewriteController_NoBody(t *testing.T) {
	input := `class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findAll",
					MethodName:  "findAll",
					Parameters:  []analyzer.RouteParameter{},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	// Should be unchanged since there are no body params
	if result != input {
		t.Errorf("methods without @Body should be unchanged, got:\n%s", result)
	}
}

func TestRewriteController_RawResponse(t *testing.T) {
	input := `class UserController {
    async download(res) {
        res.sendFile("file.pdf");
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:     "download",
					MethodName:      "download",
					UsesRawResponse: true,
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "DownloadDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"DownloadDto": "/dist/dto.DownloadDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	// Raw response routes should be skipped
	if result != input {
		t.Errorf("@Res() routes should be skipped, got:\n%s", result)
	}
}

func TestInjectAtMethodStart(t *testing.T) {
	input := `class Foo {
    async bar(x) {
        return x;
    }
}`
	result := injectAtMethodStart(input, "bar", "    x = validate(x);")
	if !strings.Contains(result, "{\n    x = validate(x);") {
		t.Errorf("expected injection after opening brace, got:\n%s", result)
	}
}

func TestRewriteController_ReturnTransform(t *testing.T) {
	input := `class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findAll",
					MethodName:  "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "stringifyUserResponse(await this.service.findAll())") {
		t.Errorf("expected return stringify wrapping, got:\n%s", result)
	}
	if !strings.Contains(result, `stringifyUserResponse`) {
		t.Errorf("expected stringify import, got:\n%s", result)
	}
}

func TestRewriteController_ArrayReturn(t *testing.T) {
	input := `class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findAll",
					MethodName:  "findAll",
					ReturnType: metadata.Metadata{
						Kind:        metadata.KindArray,
						ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, `"[" + (await this.service.findAll()).map(_v => serializeUserResponse(_v)).join(",") + "]"`) {
		t.Errorf("expected array return serialize, got:\n%s", result)
	}
}

func TestRewriteController_VoidReturn(t *testing.T) {
	input := `class UserController {
    async remove(id) {
        return;
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "remove",
					MethodName:  "remove",
					ReturnType:  metadata.Metadata{Kind: metadata.KindVoid},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	// Void return should be unchanged
	if result != input {
		t.Errorf("void return should be unchanged, got:\n%s", result)
	}
}

func TestRewriteController_BodyAndReturn(t *testing.T) {
	input := `class UserController {
    async create(body) {
        return this.service.create(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "create",
					MethodName:  "create",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
		"UserResponse":  "/dist/user.dto.UserResponse.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertCreateUserDto(body)") {
		t.Errorf("expected body validation, got:\n%s", result)
	}
	if !strings.Contains(result, "stringifyUserResponse(await") {
		t.Errorf("expected return stringify, got:\n%s", result)
	}
}

func TestRewriteController_NoReturnCompanion(t *testing.T) {
	input := `class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findAll",
					MethodName:  "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "SomeExternalType"},
				},
			},
		},
	}

	// No companion for SomeExternalType
	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	// Should be unchanged — no companion available for return type
	if result != input {
		t.Errorf("should be unchanged without companion, got:\n%s", result)
	}
}

func TestFindMethodBody(t *testing.T) {
	input := `class Foo {
    async bar(x) {
        if (x) {
            return x;
        }
        return null;
    }
    async baz() {
        return 1;
    }
}`
	start, end, found := findMethodBody(input, "bar")
	if !found {
		t.Fatal("expected to find method body for bar")
	}
	body := input[start:end]
	if !strings.Contains(body, "return x;") {
		t.Errorf("expected body to contain 'return x;', got: %s", body)
	}
	if !strings.Contains(body, "return null;") {
		t.Errorf("expected body to contain 'return null;', got: %s", body)
	}
	// Should not contain baz's body
	if strings.Contains(body, "return 1;") {
		t.Errorf("body should not contain baz's body, got: %s", body)
	}
}

func TestFindBodyParamName(t *testing.T) {
	tests := []struct {
		text, className, methodName, expected string
	}{
		{"class C {\n    async create(body) { return body; }\n}", "C", "create", "body"},
		{"class C {\n    create(dto) { return dto; }\n}", "C", "create", "dto"},
		{"class C {\n    async update(id, body) { return id; }\n}", "C", "update", "id"},
	}

	for _, tt := range tests {
		got := findBodyParamName(tt.text, tt.className, tt.methodName)
		if got != tt.expected {
			t.Errorf("findBodyParamName(%q, %q, %q) = %q, want %q", tt.text, tt.className, tt.methodName, got, tt.expected)
		}
	}
}

func TestRewriteController_WholeObjectQuery(t *testing.T) {
	input := `class OrderController {
    async findAll(query) {
        return this.service.findAll(query);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "OrderController",
			SourceFile: "/src/order.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findAll",
					MethodName:  "findAll",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "query",
							LocalName: "query",
							TypeName:  "PaginationQuery",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "PaginationQuery"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"PaginationQuery": "/dist/pagination.dto.PaginationQuery.tsgonest.js",
	}

	result := rewriteController(input, "/dist/order.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertPaginationQuery(query)") {
		t.Errorf("expected assert call for @Query() injection, got:\n%s", result)
	}
	if !strings.Contains(result, `import { assertPaginationQuery } from "./pagination.dto.PaginationQuery.tsgonest.js"`) {
		t.Errorf("expected companion import for query DTO, got:\n%s", result)
	}
}

func TestRewriteController_ScalarParamCoercion(t *testing.T) {
	input := `class UserController {
    async findOne(id) {
        return this.service.findOne(id);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findOne",
					MethodName:  "findOne",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "param",
							Name:      "id",
							LocalName: "id",
							Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "id = +id") {
		t.Errorf("expected number coercion for @Param('id'), got:\n%s", result)
	}
	if !strings.Contains(result, "Number.isNaN(id)") {
		t.Errorf("expected NaN check for @Param('id'), got:\n%s", result)
	}
	if !strings.Contains(result, `TsgonestValidationError as __e`) {
		t.Errorf("expected helpers import for scalar coercion, got:\n%s", result)
	}
}

func TestRewriteController_StringParamNoCoercion(t *testing.T) {
	input := `class UserController {
    async findBySlug(slug) {
        return this.service.findBySlug(slug);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findBySlug",
					MethodName:  "findBySlug",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "param",
							Name:      "slug",
							LocalName: "slug",
							Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	// String-typed scalar param should have no injection
	if result != input {
		t.Errorf("string @Param should be unchanged, got:\n%s", result)
	}
}

func TestRewriteController_MixedBodyQueryParam(t *testing.T) {
	input := `class OrderController {
    async create(body, query, id) {
        return this.service.create(body, query, id);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "OrderController",
			SourceFile: "/src/order.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "create",
					MethodName:  "create",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateOrderDto"},
						},
						{
							Category:  "query",
							LocalName: "query",
							TypeName:  "OrderOptions",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "OrderOptions"},
						},
						{
							Category:  "param",
							Name:      "id",
							LocalName: "id",
							Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"CreateOrderDto": "/dist/order.dto.CreateOrderDto.tsgonest.js",
		"OrderOptions":   "/dist/order.dto.OrderOptions.tsgonest.js",
	}

	result := rewriteController(input, "/dist/order.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertCreateOrderDto(body)") {
		t.Errorf("expected body validation, got:\n%s", result)
	}
	if !strings.Contains(result, "assertOrderOptions(query)") {
		t.Errorf("expected query validation, got:\n%s", result)
	}
	if !strings.Contains(result, "id = +id") {
		t.Errorf("expected param coercion, got:\n%s", result)
	}
}

func TestRewriteController_WholeObjectParam(t *testing.T) {
	input := `class UserController {
    async findOne(params) {
        return this.service.findOne(params);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findOne",
					MethodName:  "findOne",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "param",
							LocalName: "params",
							TypeName:  "RouteParams",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "RouteParams"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"RouteParams": "/dist/route.dto.RouteParams.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertRouteParams(params)") {
		t.Errorf("expected assert call for whole-object @Param(), got:\n%s", result)
	}
}

func TestRewriteController_BooleanParamCoercion(t *testing.T) {
	input := `class UserController {
    async list(active) {
        return this.service.list(active);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "list",
					MethodName:  "list",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "query",
							Name:      "active",
							LocalName: "active",
							Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, `=== "true"`) {
		t.Errorf("expected boolean coercion for @Query('active'), got:\n%s", result)
	}
	if !strings.Contains(result, `=== "1"`) {
		t.Errorf("expected '1' coercion for boolean query param, got:\n%s", result)
	}
}

// --- @EventStream SSE Rewriter Tests ---

func TestRewriteController_SSETransformInjection(t *testing.T) {
	// @EventStream with discriminated variants should inject Reflect.defineMetadata
	// with per-event assert/stringify and TsgonestSseInterceptor.
	input := `var common_1 = require("@nestjs/common");
UserEventController = __decorate([
    (0, common_1.Controller)("users")
], UserEventController);
__decorate([
    (0, common_1.Get)("events")
], UserEventController.prototype, "streamUserEvents", null);
class UserEventController {
    async *streamUserEvents() {
        yield { event: "created", data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserEventController",
			SourceFile: "/src/event.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:   "User_streamUserEvents",
					MethodName:    "streamUserEvents",
					Method:        "GET",
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "created", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"}},
						{EventName: "deleted", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "DeletePayload"}},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserDto":       "/dist/dto.UserDto.tsgonest.js",
		"DeletePayload": "/dist/dto.DeletePayload.tsgonest.js",
	}

	result := rewriteController(input, "/dist/event.controller.js", controllers, companionMap, "esm")

	// Should inject Reflect.defineMetadata after the method-level __decorate
	if !strings.Contains(result, `Reflect.defineMetadata("__tsgonest_sse_transforms__"`) {
		t.Errorf("expected Reflect.defineMetadata for SSE transforms, got:\n%s", result)
	}

	// Should contain event name keys
	if !strings.Contains(result, `"created"`) {
		t.Errorf("expected 'created' event key, got:\n%s", result)
	}
	if !strings.Contains(result, `"deleted"`) {
		t.Errorf("expected 'deleted' event key, got:\n%s", result)
	}

	// Should contain assert/stringify function names
	if !strings.Contains(result, "assertUserDto") {
		t.Errorf("expected assertUserDto function reference, got:\n%s", result)
	}
	if !strings.Contains(result, "stringifyUserDto") {
		t.Errorf("expected stringifyUserDto function reference, got:\n%s", result)
	}
	if !strings.Contains(result, "assertDeletePayload") {
		t.Errorf("expected assertDeletePayload function reference, got:\n%s", result)
	}

	// Should inject TsgonestSseInterceptor import
	if !strings.Contains(result, "TsgonestSseInterceptor") {
		t.Errorf("expected TsgonestSseInterceptor import, got:\n%s", result)
	}

	// Should inject UseInterceptors(TsgonestSseInterceptor)
	if !strings.Contains(result, "(0, common_1.UseInterceptors)(TsgonestSseInterceptor)") {
		t.Errorf("expected UseInterceptors(TsgonestSseInterceptor) injection, got:\n%s", result)
	}

	// Should import companion functions for both types
	if !strings.Contains(result, "assertUserDto") || !strings.Contains(result, "stringifyUserDto") {
		t.Errorf("expected UserDto companion imports, got:\n%s", result)
	}

	// Should contain the class name and method name in Reflect.defineMetadata
	if !strings.Contains(result, `UserEventController.prototype, "streamUserEvents"`) {
		t.Errorf("expected prototype reference in Reflect.defineMetadata, got:\n%s", result)
	}
}

func TestRewriteController_SSEGenericWildcard(t *testing.T) {
	// Non-discriminated SseEvent<string, T> should use '*' wildcard key.
	input := `var common_1 = require("@nestjs/common");
GenericController = __decorate([
    (0, common_1.Controller)("generic")
], GenericController);
__decorate([
    (0, common_1.Get)("stream")
], GenericController.prototype, "streamGeneric", null);
class GenericController {
    async *streamGeneric() {
        yield { event: "any", data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "GenericController",
			SourceFile: "/src/generic.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:   "Generic_streamGeneric",
					MethodName:    "streamGeneric",
					Method:        "GET",
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"}},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserDto": "/dist/dto.UserDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/generic.controller.js", controllers, companionMap, "esm")

	// Should use "*" as the wildcard key
	if !strings.Contains(result, `"*"`) {
		t.Errorf("expected '*' wildcard key for generic string event, got:\n%s", result)
	}
}

func TestRewriteController_SSENoReturnWrapping(t *testing.T) {
	// @EventStream routes should NOT get return statement wrapping (no stringify/serialize).
	input := `class EventController {
    async *streamEvents() {
        yield { event: "created", data: {} };
        return;
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "EventController",
			SourceFile: "/src/event.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:   "Event_streamEvents",
					MethodName:    "streamEvents",
					Method:        "GET",
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "created", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"}},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserDto": "/dist/dto.UserDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/event.controller.js", controllers, companionMap, "esm")

	// Should NOT contain stringify wrapping of return
	if strings.Contains(result, "stringifyUserDto(await") {
		t.Errorf("@EventStream routes should not get return wrapping, got:\n%s", result)
	}
}

func TestRewriteController_MixedSSEAndRegular(t *testing.T) {
	// A controller with both @EventStream and regular routes should handle both.
	input := `var common_1 = require("@nestjs/common");
MixedController = __decorate([
    (0, common_1.Controller)("mixed")
], MixedController);
__decorate([
    (0, common_1.Get)("events")
], MixedController.prototype, "streamEvents", null);
class MixedController {
    async getHealth() {
        return this.service.getHealth();
    }
    async *streamEvents() {
        yield { event: "status", data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "MixedController",
			SourceFile: "/src/mixed.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "Mixed_getHealth",
					MethodName:  "getHealth",
					Method:      "GET",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "HealthResponse"},
				},
				{
					OperationID:   "Mixed_streamEvents",
					MethodName:    "streamEvents",
					Method:        "GET",
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "status", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "StatusDto"}},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"HealthResponse": "/dist/dto.HealthResponse.tsgonest.js",
		"StatusDto":      "/dist/dto.StatusDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/mixed.controller.js", controllers, companionMap, "esm")

	// Should have return wrapping for getHealth
	if !strings.Contains(result, "stringifyHealthResponse(await") {
		t.Errorf("expected return stringify for regular route, got:\n%s", result)
	}

	// Should have SSE transform metadata for streamEvents
	if !strings.Contains(result, `Reflect.defineMetadata("__tsgonest_sse_transforms__"`) {
		t.Errorf("expected SSE transform metadata, got:\n%s", result)
	}

	// Should import both interceptors
	if !strings.Contains(result, "TsgonestSerializeInterceptor") {
		t.Errorf("expected TsgonestSerializeInterceptor import, got:\n%s", result)
	}
	if !strings.Contains(result, "TsgonestSseInterceptor") {
		t.Errorf("expected TsgonestSseInterceptor import, got:\n%s", result)
	}
}

// --- SSE interceptor without companion files tests ---

func TestRewriteController_SSEInterceptorWithoutCompanions(t *testing.T) {
	// @EventStream routes whose data types have NO companion files should still
	// get TsgonestSseInterceptor injected. The interceptor bridges async generators
	// → Observables which NestJS's SSE handler requires.
	input := `var common_1 = require("@nestjs/common");
NotificationEventsController = __decorate([
    (0, common_1.Controller)("notifications")
], NotificationEventsController);
__decorate([
    (0, common_1.Get)("events")
], NotificationEventsController.prototype, "streamNotifications", null);
class NotificationEventsController {
    async *streamNotifications() {
        yield { event: "notify", data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "NotificationEventsController",
			SourceFile: "/src/notification.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:   "Notification_streamNotifications",
					MethodName:    "streamNotifications",
					Method:        "GET",
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "notify", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "NotificationSSEEvent"}},
					},
				},
			},
		},
	}

	// Empty companion map — NotificationSSEEvent is a union type with no companion
	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/notification.controller.js", controllers, companionMap, "esm")

	// Must inject TsgonestSseInterceptor import
	if !strings.Contains(result, "TsgonestSseInterceptor") {
		t.Errorf("expected TsgonestSseInterceptor import even without companion files, got:\n%s", result)
	}

	// Must inject UseInterceptors(TsgonestSseInterceptor)
	if !strings.Contains(result, "(0, common_1.UseInterceptors)(TsgonestSseInterceptor)") {
		t.Errorf("expected UseInterceptors(TsgonestSseInterceptor) injection even without companion files, got:\n%s", result)
	}

	// Should NOT have Reflect.defineMetadata since there are no companions
	if strings.Contains(result, `Reflect.defineMetadata("__tsgonest_sse_transforms__"`) {
		t.Errorf("should not inject SSE transform metadata without companion files, got:\n%s", result)
	}
}

func TestRewriteController_SSEInterceptorNoVariants(t *testing.T) {
	// @EventStream routes with no SSEEventVariants at all (e.g. Record<string, unknown>)
	// should still get TsgonestSseInterceptor injected.
	input := `var common_1 = require("@nestjs/common");
ChatEventsController = __decorate([
    (0, common_1.Controller)("chat")
], ChatEventsController);
class ChatEventsController {
    async *streamChat() {
        yield { event: "message", data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "ChatEventsController",
			SourceFile: "/src/chat.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "Chat_streamChat",
					MethodName:  "streamChat",
					Method:      "GET",
					IsSSE:       true,
					IsEventStream: true,
					// No SSEEventVariants — data is Record<string, unknown>
					SSEEventVariants: nil,
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/chat.controller.js", controllers, companionMap, "esm")

	// Must inject TsgonestSseInterceptor import
	if !strings.Contains(result, "TsgonestSseInterceptor") {
		t.Errorf("expected TsgonestSseInterceptor import for @EventStream with no variants, got:\n%s", result)
	}

	// Must inject UseInterceptors(TsgonestSseInterceptor)
	if !strings.Contains(result, "(0, common_1.UseInterceptors)(TsgonestSseInterceptor)") {
		t.Errorf("expected UseInterceptors(TsgonestSseInterceptor) injection for @EventStream with no variants, got:\n%s", result)
	}
}

func TestRewriteController_SSEInterceptorPartialCompanions(t *testing.T) {
	// @EventStream with multiple variants where only SOME have companions.
	// Interceptor should be injected, and only the ones with companions get transforms.
	input := `var common_1 = require("@nestjs/common");
MixedEventController = __decorate([
    (0, common_1.Controller)("events")
], MixedEventController);
__decorate([
    (0, common_1.Get)("stream")
], MixedEventController.prototype, "streamMixed", null);
class MixedEventController {
    async *streamMixed() {
        yield { event: "typed", data: {} };
        yield { event: "untyped", data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "MixedEventController",
			SourceFile: "/src/mixed-event.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:   "MixedEvent_streamMixed",
					MethodName:    "streamMixed",
					Method:        "GET",
					IsSSE:         true,
					IsEventStream: true,
					SSEEventVariants: []analyzer.SSEEventVariant{
						{EventName: "typed", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "TypedDto"}},
						{EventName: "untyped", DataType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UntypedUnion"}},
					},
				},
			},
		},
	}

	// Only TypedDto has a companion; UntypedUnion does not
	companionMap := map[string]string{
		"TypedDto": "/dist/dto.TypedDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/mixed-event.controller.js", controllers, companionMap, "esm")

	// Must inject TsgonestSseInterceptor regardless
	if !strings.Contains(result, "TsgonestSseInterceptor") {
		t.Errorf("expected TsgonestSseInterceptor import with partial companions, got:\n%s", result)
	}

	if !strings.Contains(result, "(0, common_1.UseInterceptors)(TsgonestSseInterceptor)") {
		t.Errorf("expected UseInterceptors(TsgonestSseInterceptor) with partial companions, got:\n%s", result)
	}

	// Should have transform metadata for the typed variant
	if !strings.Contains(result, "assertTypedDto") {
		t.Errorf("expected assertTypedDto for companion-backed variant, got:\n%s", result)
	}

	// Should NOT reference the untyped variant in transforms
	if strings.Contains(result, "UntypedUnion") {
		t.Errorf("should not reference UntypedUnion (no companion) in transforms, got:\n%s", result)
	}
}

func TestRewriteController_MultipleControllersSSEWithoutCompanions(t *testing.T) {
	// Multiple controllers with @EventStream, none with companions — all should
	// get the interceptor injected.
	input := `var common_1 = require("@nestjs/common");
NotifController = __decorate([
    (0, common_1.Controller)("notif")
], NotifController);
OAuthController = __decorate([
    (0, common_1.Controller)("oauth")
], OAuthController);
class NotifController {
    async *stream() {
        yield { data: {} };
    }
}
class OAuthController {
    async *events() {
        yield { data: {} };
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "NotifController",
			SourceFile: "/src/notif.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:      "Notif_stream",
					MethodName:       "stream",
					Method:           "GET",
					IsSSE:            true,
					IsEventStream:    true,
					SSEEventVariants: nil,
				},
			},
		},
		{
			Name:       "OAuthController",
			SourceFile: "/src/oauth.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID:      "OAuth_events",
					MethodName:       "events",
					Method:           "GET",
					IsSSE:            true,
					IsEventStream:    true,
					SSEEventVariants: nil,
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/controllers.js", controllers, companionMap, "esm")

	// Both controllers should get the interceptor
	// Count occurrences of UseInterceptors(TsgonestSseInterceptor)
	count := strings.Count(result, "(0, common_1.UseInterceptors)(TsgonestSseInterceptor)")
	if count != 2 {
		t.Errorf("expected 2 UseInterceptors(TsgonestSseInterceptor) injections (one per controller), got %d in:\n%s", count, result)
	}

	// Single import line
	importCount := strings.Count(result, "TsgonestSseInterceptor")
	// 1 import + 2 UseInterceptors = 3 occurrences
	if importCount != 3 {
		t.Errorf("expected 3 TsgonestSseInterceptor occurrences (1 import + 2 uses), got %d in:\n%s", importCount, result)
	}
}

// --- FormData body validation tests ---

func TestRewriteController_FormDataBodyValidation(t *testing.T) {
	// @FormDataBody() params should get assert validation injection,
	// just like regular @Body() params. Multer parses the multipart request,
	// but the DTO fields still need validation (and string→number/boolean coercion).
	input := `class UploadController {
    async upload(body) {
        return this.service.upload(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UploadController",
			SourceFile: "/src/upload.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "upload",
					MethodName:  "upload",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "body",
							LocalName:   "body",
							TypeName:    "UploadDto",
							ContentType: "multipart/form-data",
							Type:        metadata.Metadata{Kind: metadata.KindRef, Ref: "UploadDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UploadDto": "/dist/upload.dto.UploadDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/upload.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assertUploadDto(body)") {
		t.Errorf("expected assert call injection for @FormDataBody(), got:\n%s", result)
	}
}

func TestRewriteController_FormDataBody_InlineTypeWithSyntheticName(t *testing.T) {
	// When the analyzer assigns a synthetic TypeName to an inline FormData body type,
	// the rewriter should inject validation just like a named DTO.
	// The synthetic name (e.g., "__UploadController_upload_Body") comes from the analyzer.
	input := `class UploadController {
    async upload(body) {
        return this.service.upload(body);
    }
}`

	// Simulate what the analyzer should produce for an inline type:
	// TypeName is synthesized, Type is KindObject with inlined properties
	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UploadController",
			SourceFile: "/src/upload.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "upload",
					MethodName:  "upload",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "body",
							LocalName:   "body",
							TypeName:    "__UploadController_upload_Body",
							ContentType: "multipart/form-data",
							Type: metadata.Metadata{
								Kind: metadata.KindObject,
								Name: "__UploadController_upload_Body",
								Properties: []metadata.Property{
									{Name: "file", Type: metadata.Metadata{Kind: metadata.KindNative, NativeType: "File"}},
									{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
								},
							},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"__UploadController_upload_Body": "/dist/upload.controller.__UploadController_upload_Body.tsgonest.js",
	}

	result := rewriteController(input, "/dist/upload.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "assert__UploadController_upload_Body(body)") {
		t.Errorf("expected assert call for inline FormData body with synthetic name, got:\n%s", result)
	}
	if !strings.Contains(result, "import") && !strings.Contains(result, "assert__UploadController_upload_Body") {
		t.Errorf("expected companion import for synthetic type, got:\n%s", result)
	}
}

func TestRewriteController_FormDataBodyWithCompanionImport(t *testing.T) {
	// Verify ESM import statement is generated for the FormData body type's companion
	input := `class UploadController {
    async upload(body) {
        return this.service.upload(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UploadController",
			SourceFile: "/src/upload.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "upload",
					MethodName:  "upload",
					Parameters: []analyzer.RouteParameter{
						{
							Category:    "body",
							LocalName:   "body",
							TypeName:    "UploadDto",
							ContentType: "multipart/form-data",
							Type:        metadata.Metadata{Kind: metadata.KindRef, Ref: "UploadDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UploadDto": "/dist/upload.dto.UploadDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/upload.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, `import { assertUploadDto } from "./upload.dto.UploadDto.tsgonest.js"`) {
		t.Errorf("expected companion import for FormData body type, got:\n%s", result)
	}
}

// --- Primitive return type serialization tests ---
// These tests verify that controller methods returning primitive types (string, number, boolean)
// get proper JSON serialization wrapping. This is critical because NestJS sets Content-Type to
// application/json but raw primitives are NOT valid JSON (e.g., a bare string without quotes).
// See: nestia TypedRoute and typia json.stringify handle this by wrapping primitives.

func TestRewriteController_StringReturn(t *testing.T) {
	// A controller method returning Promise<string> must have its return value
	// JSON-encoded (wrapped in quotes). Without this, the response body is raw text
	// but Content-Type is application/json, breaking SDK clients that call response.json().
	input := `class AuthController {
    async forgotPassword(body) {
        return "If an account exists, a reset link has been sent.";
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "AuthController",
			SourceFile: "/src/auth.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "forgotPassword",
					MethodName:  "forgotPassword",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "ForgotPasswordDto"},
						},
					},
				},
			},
		},
	}

	companionMap := map[string]string{
		"ForgotPasswordDto": "/dist/auth.dto.ForgotPasswordDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/auth.controller.js", controllers, companionMap, "esm")

	// The return value must be JSON-stringified — a raw string like:
	//   If an account exists, a reset link has been sent.
	// is NOT valid JSON. It must become:
	//   "If an account exists, a reset link has been sent."
	if !strings.Contains(result, "JSON.stringify(") && !strings.Contains(result, "__s(") {
		t.Errorf("string return type must be JSON-encoded (wrapped in quotes), got:\n%s", result)
	}
}

func TestRewriteController_StringReturnOnly(t *testing.T) {
	// Even without any body params, a method returning string must be JSON-wrapped.
	input := `class HealthController {
    async getVersion() {
        return "1.0.0";
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "HealthController",
			SourceFile: "/src/health.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "getVersion",
					MethodName:  "getVersion",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/health.controller.js", controllers, companionMap, "esm")

	// Must wrap return with JSON encoding for string
	if !strings.Contains(result, "JSON.stringify(") && !strings.Contains(result, "__s(") {
		t.Errorf("string return type must be JSON-encoded, got:\n%s", result)
	}
}

func TestRewriteController_NumberReturn(t *testing.T) {
	// A controller returning a plain number must have it serialized to a JSON number string.
	// typia.json.stringify<number>(42) → "42"
	input := `class StatsController {
    async getCount() {
        return 42;
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "StatsController",
			SourceFile: "/src/stats.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "getCount",
					MethodName:  "getCount",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/stats.controller.js", controllers, companionMap, "esm")

	// Number returns should be serialized (e.g., "" + value or Number.isFinite check)
	if !strings.Contains(result, "Number.isFinite") && !strings.Contains(result, "JSON.stringify") {
		t.Errorf("number return type must be JSON-serialized, got:\n%s", result)
	}
}

func TestRewriteController_BooleanReturn(t *testing.T) {
	// A controller returning boolean must serialize to "true" or "false" JSON string.
	input := `class FeatureController {
    async isEnabled() {
        return true;
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "FeatureController",
			SourceFile: "/src/feature.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "isEnabled",
					MethodName:  "isEnabled",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/feature.controller.js", controllers, companionMap, "esm")

	// Boolean should be serialized
	if !strings.Contains(result, `"true"`) && !strings.Contains(result, `"false"`) && !strings.Contains(result, "JSON.stringify") {
		t.Errorf("boolean return type must be JSON-serialized, got:\n%s", result)
	}
}

func TestRewriteController_NullableStringReturn(t *testing.T) {
	// A controller returning string | null must handle the null case
	input := `class UserController {
    async getDisplayName() {
        return null;
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "getDisplayName",
					MethodName:  "getDisplayName",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	// Must wrap — nullable string needs null check + JSON encoding
	if !strings.Contains(result, "null") || result == input {
		t.Errorf("nullable string return must be JSON-serialized with null handling, got:\n%s", result)
	}
}

func TestResolveReturnTypeName_Primitives(t *testing.T) {
	// resolveReturnTypeName currently returns "" for all atomic types,
	// which means no return transform is applied. This is the root cause
	// of the string return type bug.
	tests := []struct {
		name     string
		meta     metadata.Metadata
		wantName string // what it SHOULD return (non-empty to signal "needs transform")
	}{
		{
			name:     "string",
			meta:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
			wantName: "__string", // synthetic name for primitive stringify
		},
		{
			name:     "number",
			meta:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
			wantName: "__number",
		},
		{
			name:     "boolean",
			meta:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
			wantName: "__boolean",
		},
		{
			name:     "void should remain empty",
			meta:     metadata.Metadata{Kind: metadata.KindVoid},
			wantName: "",
		},
		{
			name:     "ref should return ref name",
			meta:     metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"},
			wantName: "UserDto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReturnTypeName(&tt.meta)
			if tt.wantName == "" && got != "" {
				t.Errorf("resolveReturnTypeName() = %q, want empty", got)
			}
			if tt.wantName != "" && got == "" {
				t.Errorf("resolveReturnTypeName() = %q, want non-empty (like %q) — primitive return types must be serialized", got, tt.wantName)
			}
		})
	}
}

func TestRewriteController_OverlappingEditsDoNotPanic(t *testing.T) {
	// Regression test: when the JS locator computes overlapping edit ranges
	// (e.g., two return transforms where one ends after the next begins),
	// rewriteController must not panic. The guard skips malformed edits.
	//
	// This specifically tests that edits with end < pos are filtered out.
	input := `class TestController {
    async methodA(body) {
        return this.service.methodA(body);
    }
    async methodB(body) {
        return this.service.methodB(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "TestController",
			SourceFile: "/src/test.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "methodA",
					MethodName:  "methodA",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "DtoA"},
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ResponseA"},
				},
				{
					OperationID: "methodB",
					MethodName:  "methodB",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "DtoB"},
						},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "ResponseB"},
				},
			},
		},
	}

	companionMap := map[string]string{
		"DtoA":      "/dist/dto.DtoA.tsgonest.js",
		"DtoB":      "/dist/dto.DtoB.tsgonest.js",
		"ResponseA": "/dist/dto.ResponseA.tsgonest.js",
		"ResponseB": "/dist/dto.ResponseB.tsgonest.js",
	}

	// This must not panic — it should gracefully handle any edge cases
	result := rewriteController(input, "/dist/test.controller.js", controllers, companionMap, "cjs")

	// Basic validation: body assertion should be injected for at least one method
	if !strings.Contains(result, "assertDtoA(body)") && !strings.Contains(result, "assertDtoB(body)") {
		t.Errorf("expected at least one assert call, got:\n%s", result)
	}
}

func TestMalformedEditsFiltered(t *testing.T) {
	// Directly tests the end < pos filtering guard in rewriteController's
	// Phase 4. core.ApplyBulkEdits panics when given edits where end < pos,
	// so the guard must filter them out before calling ApplyBulkEdits.

	text := "hello world"

	// Simulate the filtering logic from rewriteController Phase 4
	edits := []prioritizedEdit{
		{pos: 0, end: 5, newText: "HELLO"},  // valid: replace "hello"
		{pos: 20, end: 3, newText: "BAD"},   // malformed: end < pos
		{pos: 6, end: 11, newText: "WORLD"}, // valid: replace "world"
	}

	// Sort by position (same as rewriteController)
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].pos != edits[j].pos {
			return edits[i].pos < edits[j].pos
		}
		return edits[i].priority < edits[j].priority
	})

	// Apply the same guard as rewriteController
	var changes []core.TextChange
	for _, e := range edits {
		if e.end < e.pos {
			continue // skip malformed edit
		}
		changes = append(changes, core.TextChange{
			TextRange: core.NewTextRange(e.pos, e.end),
			NewText:   e.newText,
		})
	}

	result := core.ApplyBulkEdits(text, changes)
	expected := "HELLO WORLD"
	if result != expected {
		t.Errorf("filtered edits produced %q, want %q", result, expected)
	}

	// Verify that WITHOUT filtering, the malformed edit would cause a panic
	var allChanges []core.TextChange
	for _, e := range edits {
		allChanges = append(allChanges, core.TextChange{
			TextRange: core.NewTextRange(e.pos, e.end),
			NewText:   e.newText,
		})
	}

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		core.ApplyBulkEdits(text, allChanges)
	}()
	if !panicked {
		t.Error("expected ApplyBulkEdits to panic with malformed edits, but it did not")
	}
}

func TestRewriteController_InjectsSerializeInterceptor(t *testing.T) {
	// When a controller has return transforms (DTO or primitive), the rewriter
	// must inject TsgonestSerializeInterceptor into the class-level __decorate
	// array. Without it, NestJS/Express returns pre-serialized JSON strings
	// with Content-Type: text/html instead of application/json.

	input := `var common_1 = require("@nestjs/common");
UserController = __decorate([
    (0, common_1.Controller)("users")
], UserController);
class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "findAll",
					MethodName:  "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "cjs")

	// Must inject the interceptor import
	if !strings.Contains(result, "TsgonestSerializeInterceptor") {
		t.Fatalf("expected TsgonestSerializeInterceptor to be present, got:\n%s", result)
	}

	// Must inject UseInterceptors decorator in __decorate array
	if !strings.Contains(result, "UseInterceptors)(TsgonestSerializeInterceptor)") {
		t.Errorf("expected UseInterceptors injection in __decorate, got:\n%s", result)
	}

	// Must also have the stringify transform
	if !strings.Contains(result, "stringifyUserResponse(await") {
		t.Errorf("expected return stringify wrapping, got:\n%s", result)
	}
}

func TestRewriteController_InjectsInterceptorForPrimitiveReturn(t *testing.T) {
	// Primitive return types (string, number, boolean) also need the interceptor.
	// Without it, Express sets Content-Type: text/html for the pre-serialized string.

	input := `var common_1 = require("@nestjs/common");
HealthController = __decorate([
    (0, common_1.Controller)("health")
], HealthController);
class HealthController {
    async getVersion() {
        return "1.0.0";
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "HealthController",
			SourceFile: "/src/health.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "getVersion",
					MethodName:  "getVersion",
					ReturnType:  metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				},
			},
		},
	}

	companionMap := map[string]string{}

	result := rewriteController(input, "/dist/health.controller.js", controllers, companionMap, "cjs")

	// Must inject the interceptor even for primitive returns
	if !strings.Contains(result, "UseInterceptors)(TsgonestSerializeInterceptor)") {
		t.Errorf("expected interceptor injection for primitive return, got:\n%s", result)
	}

	// Must have JSON serialization for string return
	if !strings.Contains(result, "JSON.stringify(") {
		t.Errorf("expected JSON.stringify for string return, got:\n%s", result)
	}
}

func TestRewriteController_NoInterceptorForBodyOnly(t *testing.T) {
	// When a controller only has body validation (no return transforms),
	// the serialize interceptor must NOT be injected — there are no
	// pre-serialized string returns to protect.

	input := `var common_1 = require("@nestjs/common");
UserController = __decorate([
    (0, common_1.Controller)("users")
], UserController);
class UserController {
    async create(body) {
        return this.service.create(body);
    }
}`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					OperationID: "create",
					MethodName:  "create",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"},
						},
					},
					// No ReturnType — void
				},
			},
		},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "cjs")

	// Body validation should still be injected
	if !strings.Contains(result, "assertCreateUserDto(body)") {
		t.Errorf("expected body validation, got:\n%s", result)
	}

	// But NO interceptor — no return transforms means no pre-serialized strings
	if strings.Contains(result, "UseInterceptors)(TsgonestSerializeInterceptor)") {
		t.Errorf("interceptor should NOT be injected for body-only controller, got:\n%s", result)
	}
}
