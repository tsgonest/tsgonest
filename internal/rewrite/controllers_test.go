package rewrite

import (
	"strings"
	"testing"

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
		text, methodName, expected string
	}{
		{`async create(body) {`, "create", "body"},
		{`create(dto) {`, "create", "dto"},
		{`async update(id, body) {`, "update", "id"},
	}

	for _, tt := range tests {
		got := findBodyParamName(tt.text, tt.methodName)
		if got != tt.expected {
			t.Errorf("findBodyParamName(%q, %q) = %q, want %q", tt.text, tt.methodName, got, tt.expected)
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
