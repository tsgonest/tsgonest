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
					ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
				},
			},
		},
	}

	companionMap := map[string]string{
		"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
	}

	result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

	if !strings.Contains(result, "transformUserResponse(await this.service.findAll())") {
		t.Errorf("expected return transform wrapping, got:\n%s", result)
	}
	if !strings.Contains(result, `import { transformUserResponse }`) {
		t.Errorf("expected transform import, got:\n%s", result)
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

	if !strings.Contains(result, "(await this.service.findAll()).map(_v => transformUserResponse(_v))") {
		t.Errorf("expected array return transform, got:\n%s", result)
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
	if !strings.Contains(result, "transformUserResponse(await") {
		t.Errorf("expected return transform, got:\n%s", result)
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
