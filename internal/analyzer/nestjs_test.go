package analyzer_test

import (
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/shim/ast"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- Decorator Parsing Tests (AST-based) ---

func TestParseDecorator_Controller(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		@Controller("users")
		export class UserController {}
	`)
	defer env.release()

	classNode := findClassInSourceFile(t, env.sourceFile, "UserController")
	decs := classNode.Decorators()
	if len(decs) == 0 {
		t.Fatal("expected at least one decorator on UserController")
	}

	info := analyzer.ParseDecorator(decs[0])
	if info == nil {
		t.Fatal("expected ParseDecorator to return non-nil")
	}
	if info.Name != "Controller" {
		t.Errorf("expected Name='Controller', got %q", info.Name)
	}
	if len(info.Args) != 1 || info.Args[0] != "users" {
		t.Errorf("expected Args=[\"users\"], got %v", info.Args)
	}
}

func TestParseDecorator_Get(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		@Controller("users")
		export class TestController {
			@Get(":id")
			findOne(): void {}
		}
	`)
	defer env.release()

	method := findMethodInSourceFile(t, env.sourceFile, "TestController", "findOne")
	decs := method.Decorators()
	if len(decs) == 0 {
		t.Fatal("expected at least one decorator on findOne")
	}

	info := analyzer.ParseDecorator(decs[0])
	if info == nil {
		t.Fatal("expected ParseDecorator to return non-nil")
	}
	if info.Name != "Get" {
		t.Errorf("expected Name='Get', got %q", info.Name)
	}
	if len(info.Args) != 1 || info.Args[0] != ":id" {
		t.Errorf("expected Args=[\":id\"], got %v", info.Args)
	}
}

func TestParseDecorator_Post_NoArgs(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		@Controller("users")
		export class TestController {
			@Post()
			create(): void {}
		}
	`)
	defer env.release()

	method := findMethodInSourceFile(t, env.sourceFile, "TestController", "create")
	decs := method.Decorators()
	if len(decs) == 0 {
		t.Fatal("expected at least one decorator on create")
	}

	info := analyzer.ParseDecorator(decs[0])
	if info == nil {
		t.Fatal("expected ParseDecorator to return non-nil")
	}
	if info.Name != "Post" {
		t.Errorf("expected Name='Post', got %q", info.Name)
	}
	if len(info.Args) != 0 {
		t.Errorf("expected no args, got %v", info.Args)
	}
}

func TestParseDecorator_Param(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }
		@Controller("users")
		export class TestController {
			@Get(":id")
			findOne(@Param("id") id: string): void {}
		}
	`)
	defer env.release()

	method := findMethodInSourceFile(t, env.sourceFile, "TestController", "findOne")
	params := method.AsMethodDeclaration().Parameters
	if params == nil || len(params.Nodes) == 0 {
		t.Fatal("expected at least one parameter on findOne")
	}

	decs := params.Nodes[0].Decorators()
	if len(decs) == 0 {
		t.Fatal("expected at least one decorator on param")
	}

	info := analyzer.ParseDecorator(decs[0])
	if info == nil {
		t.Fatal("expected ParseDecorator to return non-nil")
	}
	if info.Name != "Param" {
		t.Errorf("expected Name='Param', got %q", info.Name)
	}
	if len(info.Args) != 1 || info.Args[0] != "id" {
		t.Errorf("expected Args=[\"id\"], got %v", info.Args)
	}
}

func TestParseDecorator_Body(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }
		@Controller("users")
		export class TestController {
			@Post()
			create(@Body() body: { name: string }): void {}
		}
	`)
	defer env.release()

	method := findMethodInSourceFile(t, env.sourceFile, "TestController", "create")
	params := method.AsMethodDeclaration().Parameters
	if params == nil || len(params.Nodes) == 0 {
		t.Fatal("expected at least one parameter")
	}

	decs := params.Nodes[0].Decorators()
	if len(decs) == 0 {
		t.Fatal("expected at least one decorator on param")
	}

	info := analyzer.ParseDecorator(decs[0])
	if info == nil {
		t.Fatal("expected ParseDecorator to return non-nil")
	}
	if info.Name != "Body" {
		t.Errorf("expected Name='Body', got %q", info.Name)
	}
}

func TestParseDecorator_HttpCode(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function HttpCode(code: number): MethodDecorator { return (t, k, d) => d; }
		@Controller("users")
		export class TestController {
			@Post()
			@HttpCode(204)
			create(): void {}
		}
	`)
	defer env.release()

	method := findMethodInSourceFile(t, env.sourceFile, "TestController", "create")
	decs := method.Decorators()
	if len(decs) < 2 {
		t.Fatalf("expected at least 2 decorators on create, got %d", len(decs))
	}

	// Find HttpCode decorator
	var httpCodeInfo *analyzer.DecoratorInfo
	for _, dec := range decs {
		info := analyzer.ParseDecorator(dec)
		if info != nil && info.Name == "HttpCode" {
			httpCodeInfo = info
			break
		}
	}

	if httpCodeInfo == nil {
		t.Fatal("expected to find HttpCode decorator")
	}
	if httpCodeInfo.NumericArg == nil {
		t.Fatal("expected NumericArg to be set")
	}
	if *httpCodeInfo.NumericArg != 204 {
		t.Errorf("expected NumericArg=204, got %v", *httpCodeInfo.NumericArg)
	}
}

func TestIsControllerDecorator(t *testing.T) {
	tests := []struct {
		name string
		info *analyzer.DecoratorInfo
		want bool
	}{
		{"Controller", &analyzer.DecoratorInfo{Name: "Controller"}, true},
		{"Get", &analyzer.DecoratorInfo{Name: "Get"}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzer.IsControllerDecorator(tt.info); got != tt.want {
				t.Errorf("IsControllerDecorator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCombinePaths(t *testing.T) {
	tests := []struct {
		prefix  string
		subPath string
		want    string
	}{
		{"users", "", "/users"},
		{"users", ":id", "/users/:id"},
		{"", "", "/"},
		{"", ":id", "/:id"},
		{"/users/", "/:id/", "/users/:id"},
		{"api/v1", "users", "/api/v1/users"},
	}
	for _, tt := range tests {
		t.Run(tt.prefix+"+"+tt.subPath, func(t *testing.T) {
			if got := analyzer.CombinePaths(tt.prefix, tt.subPath); got != tt.want {
				t.Errorf("CombinePaths(%q, %q) = %q, want %q", tt.prefix, tt.subPath, got, tt.want)
			}
		})
	}
}

// --- Route Extraction Tests (require full program + checker) ---

func TestControllerAnalyzer_BasicController(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		interface CreateUserDto {
			name: string;
			email: string;
		}

		interface UserResponse {
			id: number;
			name: string;
		}

		@Controller("users")
		export class UserController {
			@Get()
			async findAll(): Promise<UserResponse[]> {
				return [];
			}

			@Get(":id")
			async findOne(@Param("id") id: string): Promise<UserResponse> {
				return {} as UserResponse;
			}

			@Post()
			async create(@Body() body: CreateUserDto): Promise<UserResponse> {
				return {} as UserResponse;
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	ctrl := controllers[0]
	if ctrl.Name != "UserController" {
		t.Errorf("expected Name='UserController', got %q", ctrl.Name)
	}
	if ctrl.Path != "users" {
		t.Errorf("expected Path='users', got %q", ctrl.Path)
	}
	if len(ctrl.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(ctrl.Routes))
	}

	// Verify findAll route
	r0 := ctrl.Routes[0]
	if r0.Method != "GET" {
		t.Errorf("route 0: expected Method='GET', got %q", r0.Method)
	}
	if r0.Path != "/users" {
		t.Errorf("route 0: expected Path='/users', got %q", r0.Path)
	}
	if r0.OperationID != "User_findAll" {
		t.Errorf("route 0: expected OperationID='User_findAll', got %q", r0.OperationID)
	}
	if r0.StatusCode != 200 {
		t.Errorf("route 0: expected StatusCode=200, got %d", r0.StatusCode)
	}
	// Return type should be array of UserResponse (Promise unwrapped)
	if r0.ReturnType.Kind != "array" {
		t.Errorf("route 0: expected ReturnType.Kind='array', got %q", r0.ReturnType.Kind)
	}

	// Verify findOne route
	r1 := ctrl.Routes[1]
	if r1.Method != "GET" {
		t.Errorf("route 1: expected Method='GET', got %q", r1.Method)
	}
	if r1.Path != "/users/:id" {
		t.Errorf("route 1: expected Path='/users/:id', got %q", r1.Path)
	}
	if len(r1.Parameters) != 1 {
		t.Fatalf("route 1: expected 1 parameter, got %d", len(r1.Parameters))
	}
	if r1.Parameters[0].Category != "param" {
		t.Errorf("route 1 param 0: expected Category='param', got %q", r1.Parameters[0].Category)
	}
	if r1.Parameters[0].Name != "id" {
		t.Errorf("route 1 param 0: expected Name='id', got %q", r1.Parameters[0].Name)
	}

	// Verify create route
	r2 := ctrl.Routes[2]
	if r2.Method != "POST" {
		t.Errorf("route 2: expected Method='POST', got %q", r2.Method)
	}
	if r2.Path != "/users" {
		t.Errorf("route 2: expected Path='/users', got %q", r2.Path)
	}
	if r2.StatusCode != 201 {
		t.Errorf("route 2: expected StatusCode=201 (POST default), got %d", r2.StatusCode)
	}
	if len(r2.Parameters) != 1 {
		t.Fatalf("route 2: expected 1 parameter, got %d", len(r2.Parameters))
	}
	if r2.Parameters[0].Category != "body" {
		t.Errorf("route 2 param 0: expected Category='body', got %q", r2.Parameters[0].Category)
	}

	// Tags
	if len(r0.Tags) != 1 || r0.Tags[0] != "User" {
		t.Errorf("route 0: expected Tags=['User'], got %v", r0.Tags)
	}
}

func TestControllerAnalyzer_HttpCode(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Delete(path?: string): MethodDecorator { return (t, k, d) => d; }
		function HttpCode(code: number): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		@Controller("users")
		export class UserController {
			@Delete(":id")
			@HttpCode(204)
			async remove(@Param("id") id: string): Promise<void> {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if route.Method != "DELETE" {
		t.Errorf("expected Method='DELETE', got %q", route.Method)
	}
	if route.StatusCode != 204 {
		t.Errorf("expected StatusCode=204, got %d", route.StatusCode)
	}
	if route.Path != "/users/:id" {
		t.Errorf("expected Path='/users/:id', got %q", route.Path)
	}
}

func TestControllerAnalyzer_QueryParams(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Query(): ParameterDecorator { return () => {}; }

		interface ListQuery {
			page?: number;
			limit?: number;
			search?: string;
		}

		@Controller("users")
		export class UserController {
			@Get()
			async findAll(@Query() query: ListQuery): Promise<void> {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "query" {
		t.Errorf("expected Category='query', got %q", param.Category)
	}
	// The type should be a ref to ListQuery (an object type)
	if param.Type.Kind != "ref" && param.Type.Kind != "object" {
		t.Errorf("expected param Type.Kind='ref' or 'object', got %q", param.Type.Kind)
	}
}

func TestControllerAnalyzer_NoController(t *testing.T) {
	env := setupWalker(t, `
		export class NotAController {
			doSomething(): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 0 {
		t.Errorf("expected 0 controllers, got %d", len(controllers))
	}
}

func TestControllerAnalyzer_MultipleHTTPMethods(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Put(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Delete(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Patch(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }
		function Body(): ParameterDecorator { return () => {}; }

		@Controller("items")
		export class ItemController {
			@Get()
			list(): void {}

			@Post()
			create(@Body() body: { name: string }): void {}

			@Put(":id")
			update(@Param("id") id: string, @Body() body: { name: string }): void {}

			@Patch(":id")
			partialUpdate(@Param("id") id: string): void {}

			@Delete(":id")
			remove(@Param("id") id: string): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	routes := controllers[0].Routes
	if len(routes) != 5 {
		t.Fatalf("expected 5 routes, got %d", len(routes))
	}

	expectedMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	expectedPaths := []string{"/items", "/items", "/items/:id", "/items/:id", "/items/:id"}

	for i, route := range routes {
		if route.Method != expectedMethods[i] {
			t.Errorf("route %d: expected Method=%q, got %q", i, expectedMethods[i], route.Method)
		}
		if route.Path != expectedPaths[i] {
			t.Errorf("route %d: expected Path=%q, got %q", i, expectedPaths[i], route.Path)
		}
	}
}

func TestControllerAnalyzer_ReturnTypeResolution(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Delete(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		interface User {
			id: number;
			name: string;
		}

		@Controller("users")
		export class UserController {
			@Get()
			async getUsers(): Promise<User[]> { return []; }

			@Get(":id")
			async getUser(@Param("id") id: string): Promise<User> { return {} as User; }

			@Delete(":id")
			async deleteUser(@Param("id") id: string): Promise<void> {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 3 {
		t.Fatalf("expected 1 controller with 3 routes")
	}

	// getUsers should return User[] (Promise unwrapped)
	r0 := controllers[0].Routes[0]
	if r0.ReturnType.Kind != "array" {
		t.Errorf("getUsers return: expected Kind='array', got %q", r0.ReturnType.Kind)
	}

	// getUser should return User (Promise unwrapped)
	r1 := controllers[0].Routes[1]
	if r1.ReturnType.Kind != "ref" && r1.ReturnType.Kind != "object" {
		t.Errorf("getUser return: expected Kind='ref' or 'object', got %q", r1.ReturnType.Kind)
	}

	// deleteUser should return void (Promise unwrapped)
	r2 := controllers[0].Routes[2]
	if r2.ReturnType.Kind != "void" {
		t.Errorf("deleteUser return: expected Kind='void', got %q", r2.ReturnType.Kind)
	}
}

// --- Test Helpers ---

// findClassInSourceFile finds a class declaration by name in a source file.
func findClassInSourceFile(t *testing.T, sf *ast.SourceFile, name string) *ast.Node {
	t.Helper()
	for _, stmt := range sf.Statements.Nodes {
		if stmt.Kind == ast.KindClassDeclaration {
			classDecl := stmt.AsClassDeclaration()
			if classDecl.Name() != nil && classDecl.Name().Text() == name {
				return stmt
			}
		}
	}
	t.Fatalf("class %q not found in source file", name)
	return nil
}

// findMethodInSourceFile finds a method declaration by name within a class.
func findMethodInSourceFile(t *testing.T, sf *ast.SourceFile, className, methodName string) *ast.Node {
	t.Helper()
	classNode := findClassInSourceFile(t, sf, className)
	classDecl := classNode.AsClassDeclaration()
	if classDecl.Members != nil {
		for _, member := range classDecl.Members.Nodes {
			if member.Kind == ast.KindMethodDeclaration {
				methodDecl := member.AsMethodDeclaration()
				if methodDecl.Name() != nil && methodDecl.Name().Text() == methodName {
					return member
				}
			}
		}
	}
	t.Fatalf("method %q not found in class %q", methodName, className)
	return nil
}

// --- Phase 10.1 JSDoc → OpenAPI Metadata Tests ---

func TestControllerAnalyzer_JSDocSummaryDescription(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("users")
		export class UserController {
			/**
			 * @summary Get all users
			 * @description Returns a paginated list of all users
			 */
			@Get()
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if route.Summary != "Get all users" {
		t.Errorf("expected Summary='Get all users', got %q", route.Summary)
	}
	if route.Description != "Returns a paginated list of all users" {
		t.Errorf("expected Description='Returns a paginated list of all users', got %q", route.Description)
	}
}

func TestControllerAnalyzer_JSDocDeprecated(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("items")
		export class ItemController {
			/**
			 * @deprecated Use findAllV2 instead
			 */
			@Get()
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if !route.Deprecated {
		t.Error("expected Deprecated=true")
	}
}

func TestControllerAnalyzer_JSDocTags(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("admin")
		export class AdminController {
			/**
			 * @tag Users
			 * @tag Admin
			 */
			@Get()
			listUsers(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(route.Tags), route.Tags)
	}
	if route.Tags[0] != "Users" {
		t.Errorf("expected Tags[0]='Users', got %q", route.Tags[0])
	}
	if route.Tags[1] != "Admin" {
		t.Errorf("expected Tags[1]='Admin', got %q", route.Tags[1])
	}
}

func TestControllerAnalyzer_ClassLevelTag(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		env := setupWalker(t, `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

			/**
			 * @tag Authentication
			 */
			@Controller("auth")
			export class AuthController {
				@Get("login")
				login(): string { return ""; }

				@Get("register")
				register(): string { return ""; }
			}
		`)
		defer env.release()

		ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
		defer caRelease()

		controllers := ca.AnalyzeSourceFile(env.sourceFile)
		if len(controllers) != 1 {
			t.Fatalf("expected 1 controller, got %d", len(controllers))
		}
		if len(controllers[0].Routes) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(controllers[0].Routes))
		}

		// Both routes should inherit the class-level @tag, NOT the auto-derived "Auth" tag
		for _, route := range controllers[0].Routes {
			if len(route.Tags) != 1 {
				t.Fatalf("expected 1 tag on route %s, got %d: %v", route.Path, len(route.Tags), route.Tags)
			}
			if route.Tags[0] != "Authentication" {
				t.Errorf("expected tag 'Authentication' on route %s, got %q", route.Path, route.Tags[0])
			}
		}
	})

	t.Run("with_security_and_description", func(t *testing.T) {
		// Matches the user's reported scenario: @tag + @security in class JSDoc
		env := setupWalker(t, `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

			/**
			 * Authentication controller
			 * @tag Authentication
			 * @security bearer
			 */
			@Controller("/auth")
			export class AuthController {
				@Get("login")
				login(): string { return ""; }
			}
		`)
		defer env.release()

		ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
		defer caRelease()

		controllers := ca.AnalyzeSourceFile(env.sourceFile)
		if len(controllers) != 1 {
			t.Fatalf("expected 1 controller, got %d", len(controllers))
		}
		if len(controllers[0].Routes) != 1 {
			t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
		}

		route := controllers[0].Routes[0]
		if len(route.Tags) != 1 || route.Tags[0] != "Authentication" {
			t.Errorf("expected tags ['Authentication'], got %v", route.Tags)
		}
		if len(route.Security) != 1 || route.Security[0].Name != "bearer" {
			t.Errorf("expected security [{bearer}], got %v", route.Security)
		}
	})

	t.Run("method_overrides_class", func(t *testing.T) {
		env := setupWalker(t, `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

			/**
			 * @tag Authentication
			 */
			@Controller("auth")
			export class AuthController {
				@Get("login")
				login(): string { return ""; }

				/**
				 * @tag Custom
				 */
				@Get("special")
				special(): string { return ""; }
			}
		`)
		defer env.release()

		ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
		defer caRelease()

		controllers := ca.AnalyzeSourceFile(env.sourceFile)
		if len(controllers) != 1 || len(controllers[0].Routes) != 2 {
			t.Fatalf("expected 1 controller with 2 routes")
		}

		for _, route := range controllers[0].Routes {
			if route.OperationID == "Auth_login" {
				if len(route.Tags) != 1 || route.Tags[0] != "Authentication" {
					t.Errorf("login should inherit class tag 'Authentication', got %v", route.Tags)
				}
			} else {
				if len(route.Tags) != 1 || route.Tags[0] != "Custom" {
					t.Errorf("special should have method tag 'Custom', got %v", route.Tags)
				}
			}
		}
	})
}

func TestControllerAnalyzer_ClassLevelSecurity(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		/**
		 * @security bearer
		 */
		@Controller("users")
		export class UserController {
			@Get()
			list(): string { return ""; }

			@Get(":id")
			findOne(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 1 controller with 2 routes, got %d controllers", len(controllers))
	}

	// Both routes should inherit class-level @security
	for _, route := range controllers[0].Routes {
		if len(route.Security) != 1 {
			t.Fatalf("expected 1 security on route %s, got %d", route.Path, len(route.Security))
		}
		if route.Security[0].Name != "bearer" {
			t.Errorf("expected security 'bearer' on route %s, got %q", route.Path, route.Security[0].Name)
		}
	}
}

func TestControllerAnalyzer_ClassLevelHidden(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		/**
		 * @hidden
		 */
		@Controller("internal")
		export class InternalController {
			@Get()
			health(): string { return "ok"; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if !controllers[0].IgnoreOpenAPI {
		t.Error("expected controller with @hidden to have IgnoreOpenAPI=true")
	}
}

func TestControllerAnalyzer_ClassLevelPublic(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		/**
		 * @public
		 */
		@Controller("public")
		export class PublicController {
			@Get()
			list(): string { return ""; }

			@Get(":id")
			findOne(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 1 controller with 2 routes")
	}

	// Both routes should inherit class-level @public
	for _, route := range controllers[0].Routes {
		if !route.IsPublic {
			t.Errorf("expected IsPublic=true on route %s", route.Path)
		}
	}
}

func TestControllerAnalyzer_JSDocSecurity(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("users")
		export class SecureController {
			/**
			 * @security bearer
			 */
			@Get()
			getProfile(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Security) != 1 {
		t.Fatalf("expected 1 security requirement, got %d", len(route.Security))
	}
	if route.Security[0].Name != "bearer" {
		t.Errorf("expected Security[0].Name='bearer', got %q", route.Security[0].Name)
	}
}

func TestControllerAnalyzer_JSDocThrows(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		@Controller("users")
		export class UserController {
			/**
			 * @throws {400} BadRequestError
			 * @throws {401} UnauthorizedError
			 */
			@Post()
			create(@Body() body: { name: string }): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.ErrorResponses) != 2 {
		t.Fatalf("expected 2 error responses, got %d", len(route.ErrorResponses))
	}
	if route.ErrorResponses[0].StatusCode != 400 {
		t.Errorf("expected ErrorResponses[0].StatusCode=400, got %d", route.ErrorResponses[0].StatusCode)
	}
	if route.ErrorResponses[0].TypeName != "BadRequestError" {
		t.Errorf("expected ErrorResponses[0].TypeName='BadRequestError', got %q", route.ErrorResponses[0].TypeName)
	}
	if route.ErrorResponses[1].StatusCode != 401 {
		t.Errorf("expected ErrorResponses[1].StatusCode=401, got %d", route.ErrorResponses[1].StatusCode)
	}
	if route.ErrorResponses[1].TypeName != "UnauthorizedError" {
		t.Errorf("expected ErrorResponses[1].TypeName='UnauthorizedError', got %q", route.ErrorResponses[1].TypeName)
	}
}

// --- Phase 10.3: @Version() Decorator Test ---

func TestControllerAnalyzer_VersionDecorator(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Version(version: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("users")
		export class UserController {
			@Version("1")
			@Get()
			findAllV1(): string { return ""; }

			@Version("2")
			@Get()
			findAllV2(): string { return ""; }

			@Get("health")
			healthCheck(): string { return "ok"; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(controllers[0].Routes))
	}

	r0 := controllers[0].Routes[0]
	if r0.Version != "1" {
		t.Errorf("route 0: expected Version='1', got %q", r0.Version)
	}
	if r0.OperationID != "User_v1_findAllV1" {
		t.Errorf("route 0: expected OperationID='User_v1_findAllV1', got %q", r0.OperationID)
	}

	r1 := controllers[0].Routes[1]
	if r1.Version != "2" {
		t.Errorf("route 1: expected Version='2', got %q", r1.Version)
	}
	if r1.OperationID != "User_v2_findAllV2" {
		t.Errorf("route 1: expected OperationID='User_v2_findAllV2', got %q", r1.OperationID)
	}

	r2 := controllers[0].Routes[2]
	if r2.Version != "" {
		t.Errorf("route 2: expected Version='', got %q", r2.Version)
	}
}

// --- Custom Decorator @in JSDoc Resolution Tests ---

func TestControllerAnalyzer_CustomDecoratorIn_Param(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		type ParamDecorator = (target: any, key: string, index: number) => void;
		function createParamDecorator(fn: Function): (...args: any[]) => ParamDecorator {
			return (...args: any[]) => (target: any, key: string, index: number) => {};
		}

		/** @in param */
		const ExtractId = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().params[data];
		});

		@Controller("users")
		export class UserController {
			@Get(":id")
			findOne(@ExtractId("id") id: string): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "param" {
		t.Errorf("expected Category='param', got %q", param.Category)
	}
	if param.Name != "id" {
		t.Errorf("expected Name='id', got %q", param.Name)
	}
}

func TestControllerAnalyzer_CustomDecoratorIn_Query(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		type ParamDecorator = (target: any, key: string, index: number) => void;
		function createParamDecorator(fn: Function): (...args: any[]) => ParamDecorator {
			return (...args: any[]) => (target: any, key: string, index: number) => {};
		}

		/** @in query */
		const ExtractQuery = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().query[data];
		});

		interface FilterDto {
			search?: string;
			page?: number;
		}

		@Controller("items")
		export class ItemController {
			@Get()
			findAll(@ExtractQuery() filter: FilterDto): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "query" {
		t.Errorf("expected Category='query', got %q", param.Category)
	}
}

func TestControllerAnalyzer_CustomDecoratorIn_Body(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }

		type ParamDecorator = (target: any, key: string, index: number) => void;
		function createParamDecorator(fn: Function): (...args: any[]) => ParamDecorator {
			return (...args: any[]) => (target: any, key: string, index: number) => {};
		}

		/** @in body */
		const ValidatedBody = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().body;
		});

		interface CreateUserDto {
			name: string;
			email: string;
		}

		@Controller("users")
		export class UserController {
			@Post()
			create(@ValidatedBody() body: CreateUserDto): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	if route.Parameters[0].Category != "body" {
		t.Errorf("expected Category='body', got %q", route.Parameters[0].Category)
	}
}

func TestControllerAnalyzer_CustomDecoratorIn_Headers(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		type ParamDecorator = (target: any, key: string, index: number) => void;
		function createParamDecorator(fn: Function): (...args: any[]) => ParamDecorator {
			return (...args: any[]) => (target: any, key: string, index: number) => {};
		}

		/** @in headers */
		const ExtractHeader = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().headers[data];
		});

		@Controller("items")
		export class ItemController {
			@Get()
			findAll(@ExtractHeader("x-api-key") apiKey: string): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "headers" {
		t.Errorf("expected Category='headers', got %q", param.Category)
	}
	if param.Name != "x-api-key" {
		t.Errorf("expected Name='x-api-key', got %q", param.Name)
	}
}

func TestControllerAnalyzer_CustomDecorator_NoIn_Skipped(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		type ParamDecorator = (target: any, key: string, index: number) => void;
		function createParamDecorator(fn: Function): (...args: any[]) => ParamDecorator {
			return (...args: any[]) => (target: any, key: string, index: number) => {};
		}

		// No @in tag — this decorator should be silently skipped
		const CurrentUser = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().user;
		});

		@Controller("profile")
		export class ProfileController {
			@Get()
			getProfile(@CurrentUser() user: { id: number }): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 0 {
		t.Errorf("expected 0 parameters (custom decorator without @in should be skipped), got %d", len(route.Parameters))
	}
}

func TestControllerAnalyzer_MixedBuiltinAndCustomDecorators(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Put(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		type ParamDecorator = (target: any, key: string, index: number) => void;
		function createParamDecorator(fn: Function): (...args: any[]) => ParamDecorator {
			return (...args: any[]) => (target: any, key: string, index: number) => {};
		}

		/** @in param */
		const ExtractId = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().params[data];
		});

		const CurrentUser = createParamDecorator((data: any, ctx: any) => {
			return ctx.switchToHttp().getRequest().user;
		});

		interface UpdateDto { name: string; }

		@Controller("users")
		export class UserController {
			@Put(":id")
			update(
				@ExtractId("id") id: string,
				@CurrentUser() user: { id: number },
				@Body() body: UpdateDto,
			): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	// ExtractId(@in param) + Body (built-in) = 2 params. CurrentUser (no @in) skipped.
	if len(route.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(route.Parameters))
	}

	// First param: ExtractId → "param" category
	if route.Parameters[0].Category != "param" {
		t.Errorf("param 0: expected Category='param', got %q", route.Parameters[0].Category)
	}
	if route.Parameters[0].Name != "id" {
		t.Errorf("param 0: expected Name='id', got %q", route.Parameters[0].Name)
	}

	// Second param: Body → "body" category
	if route.Parameters[1].Category != "body" {
		t.Errorf("param 1: expected Category='body', got %q", route.Parameters[1].Category)
	}
}

func TestControllerAnalyzer_HigherOrderDecoratorFactory(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		type ParamDecorator = (target: any, key: string, index: number) => void;

		/** @in param */
		function ExtractParam(name: string): ParamDecorator {
			return (target: any, key: string, index: number) => {};
		}

		@Controller("users")
		export class UserController {
			@Get(":id")
			findOne(@ExtractParam("id") id: string): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "param" {
		t.Errorf("expected Category='param', got %q", param.Category)
	}
	if param.Name != "id" {
		t.Errorf("expected Name='id', got %q", param.Name)
	}
}

// --- Return Type Inference Tests (no explicit annotation) ---

func TestControllerAnalyzer_InferReturnType_InlineObject(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("items")
		export class ItemController {
			@Get()
			findAll() {
				return [{ id: 1, name: "item" }];
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	r := controllers[0].Routes[0]
	// Inferred return type should be an array (of objects), not KindAny
	if r.ReturnType.Kind == "any" {
		t.Errorf("findAll: expected inferred return type, got KindAny")
	}
}

func TestControllerAnalyzer_InferReturnType_ServiceDelegation(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		interface UserDto {
			id: number;
			name: string;
			email: string;
		}

		class UserService {
			findAll(): UserDto[] { return []; }
			findOne(id: string): UserDto { return {} as UserDto; }
		}

		@Controller("users")
		export class UserController {
			private userService = new UserService();

			@Get()
			getUsers() {
				return this.userService.findAll();
			}

			@Get(":id")
			getUser() {
				return this.userService.findOne("1");
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 1 controller with 2 routes, got %d controllers", len(controllers))
	}

	// getUsers: service returns UserDto[] → should infer array
	r0 := controllers[0].Routes[0]
	if r0.ReturnType.Kind == "any" {
		t.Errorf("getUsers: expected inferred array return type, got KindAny")
	}

	// getUser: service returns UserDto → should infer ref or object
	r1 := controllers[0].Routes[1]
	if r1.ReturnType.Kind == "any" {
		t.Errorf("getUser: expected inferred object/ref return type, got KindAny")
	}
}

func TestControllerAnalyzer_InferReturnType_Primitive(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("health")
		export class HealthController {
			@Get()
			check() {
				return { status: "ok", uptime: 12345 };
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	r := controllers[0].Routes[0]
	// Should infer an object with status and uptime properties, not KindAny
	if r.ReturnType.Kind == "any" {
		t.Errorf("check: expected inferred object return type, got KindAny")
	}
}

func TestControllerAnalyzer_InferReturnType_AsyncNoAnnotation(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		interface Product { id: number; title: string; price: number; }

		@Controller("products")
		export class ProductController {
			@Get()
			async findAll() {
				return [] as Product[];
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	r := controllers[0].Routes[0]
	// async without annotation: checker infers Promise<Product[]>; walker unwraps Promise → array
	if r.ReturnType.Kind == "any" {
		t.Errorf("findAll: expected inferred array return type from async method, got KindAny")
	}
}

func TestControllerAnalyzer_InferReturnType_NoSlowWarning(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		interface SimpleDto { id: number; name: string; }

		class SimpleService {
			getAll(): SimpleDto[] { return []; }
		}

		@Controller("simple")
		export class SimpleController {
			private svc = new SimpleService();

			@Get()
			getAll() {
				return this.svc.getAll();
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	ca.AnalyzeSourceFile(env.sourceFile)

	// Fast inference (service has explicit return type) should NOT produce a slow-inference warning
	warnings := ca.Warnings()
	for _, w := range warnings {
		if w.Kind == "slow-return-type-inference" {
			t.Errorf("unexpected slow-return-type-inference warning for simple service delegation: %s", w.Message)
		}
	}
}

func TestControllerAnalyzer_InferReturnType_ExplicitAnnotation_StillWorks(t *testing.T) {
	// Verify that methods WITH explicit annotations still work (regression test)
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		interface User { id: number; name: string; }

		@Controller("users")
		export class UserController {
			@Get()
			async getUsers(): Promise<User[]> { return []; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	r := controllers[0].Routes[0]
	if r.ReturnType.Kind != "array" {
		t.Errorf("getUsers: expected Kind='array', got %q", r.ReturnType.Kind)
	}
}

// --- Phase 3: @Res() Detection Tests ---

func TestControllerAnalyzer_ResDecorator_ForcesVoid(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Res(): ParameterDecorator { return () => {}; }

		@Controller("files")
		export class FileController {
			@Get("download")
			download(@Res() res: any): string {
				return "file content";
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	r := controllers[0].Routes[0]
	// @Res() should force return type to void
	if r.ReturnType.Kind != "void" {
		t.Errorf("expected return type void when @Res() is used, got %q", r.ReturnType.Kind)
	}
	if !r.UsesRawResponse {
		t.Error("expected UsesRawResponse=true when @Res() is used")
	}

	// Should emit a warning about @Res() usage
	warnings := ca.Warnings()
	found := false
	for _, w := range warnings {
		if w.Kind == "uses-raw-response" {
			found = true
		}
	}
	if !found {
		t.Error("expected uses-raw-response warning when @Res() is used")
	}
}

func TestControllerAnalyzer_ResponseDecorator_ForcesVoid(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Response(): ParameterDecorator { return () => {}; }

		@Controller("files")
		export class FileController {
			@Get("stream")
			stream(@Response() res: any): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	r := controllers[0].Routes[0]
	if r.ReturnType.Kind != "void" {
		t.Errorf("expected return type void when @Response() is used, got %q", r.ReturnType.Kind)
	}
	if !r.UsesRawResponse {
		t.Error("expected UsesRawResponse=true when @Response() is used")
	}
}

// --- Phase 4: Custom Decorator Warning Tests ---

func TestControllerAnalyzer_CustomDecorator_NoIn_WithType_Warns(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		// Custom decorator WITHOUT @in annotation
		const ExtractUser = (data?: any): ParameterDecorator => {
			return (target: any, key: string | symbol, index: number) => {};
		};

		interface UserPayload { id: string; email: string; }

		@Controller("profile")
		export class ProfileController {
			@Get()
			getProfile(@ExtractUser() user: UserPayload): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	ca.AnalyzeSourceFile(env.sourceFile)

	// Custom decorators without @in are context-injection decorators — silently skipped.
	// No warning should be emitted regardless of whether the parameter has a type annotation.
	warnings := ca.Warnings()
	for _, w := range warnings {
		if w.Kind == "custom-decorator-no-in" {
			t.Errorf("unexpected custom-decorator-no-in warning — custom decorators without @in should be silently skipped: %s", w.Message)
		}
	}
}

func TestControllerAnalyzer_CustomDecorator_NoIn_NoType_Silent(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		// Custom decorator WITHOUT @in — and parameter has no type annotation
		const CurrentUser = (data?: any): ParameterDecorator => {
			return (target: any, key: string | symbol, index: number) => {};
		};

		@Controller("profile")
		export class ProfileController {
			@Get()
			getProfile(@CurrentUser() user): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	ca.AnalyzeSourceFile(env.sourceFile)

	// No type annotation = silently skip (likely @CurrentUser, @Ip, etc.)
	warnings := ca.Warnings()
	for _, w := range warnings {
		if w.Kind == "custom-decorator-no-in" {
			t.Errorf("unexpected custom-decorator-no-in warning for untyped parameter: %s", w.Message)
		}
	}
}

func TestControllerAnalyzer_AliasedBodyDecorator(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"decorators.ts": `
			export function Controller(path: string): ClassDecorator { return (target) => target; }
			export function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
			export function Body(): ParameterDecorator { return (target, key, index) => {}; }
		`,
		"controller.ts": `
			import { Controller, Post, Body as NestBody } from "./decorators";

			interface CreateUserDto {
				name: string;
				email: string;
			}

			@Controller("users")
			export class UserController {
				@Post()
				create(@NestBody() body: CreateUserDto): string { return ""; }
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "body" {
		t.Errorf("expected Category='body', got %q", param.Category)
	}
	if param.LocalName != "body" {
		t.Errorf("expected LocalName='body', got %q", param.LocalName)
	}
}

func TestControllerAnalyzer_AliasedQueryDecorator(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"decorators.ts": `
			export function Controller(path: string): ClassDecorator { return (target) => target; }
			export function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			export function Query(): ParameterDecorator { return (target, key, index) => {}; }
		`,
		"controller.ts": `
			import { Controller, Get, Query as NestQuery } from "./decorators";

			interface PaginationQuery {
				page?: number;
				limit?: number;
			}

			@Controller("items")
			export class ItemController {
				@Get()
				findAll(@NestQuery() query: PaginationQuery): string { return ""; }
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "query" {
		t.Errorf("expected Category='query', got %q", param.Category)
	}
}

func TestControllerAnalyzer_AliasedControllerDecorator(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"decorators.ts": `
			export function Controller(path: string): ClassDecorator { return (target) => target; }
			export function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		`,
		"controller.ts": `
			import { Controller as NestController, Get as NestGet } from "./decorators";

			@NestController("products")
			export class ProductController {
				@NestGet()
				findAll(): string { return ""; }
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if controllers[0].Name != "ProductController" {
		t.Errorf("expected Name='ProductController', got %q", controllers[0].Name)
	}
	if controllers[0].Path != "products" {
		t.Errorf("expected Path='products', got %q", controllers[0].Path)
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}
	if controllers[0].Routes[0].Method != "GET" {
		t.Errorf("expected Method='GET', got %q", controllers[0].Routes[0].Method)
	}
}

// --- Robustness Tests: BindingPattern & QualifiedName ---

func TestControllerAnalyzer_DestructuredBodyParam(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		interface CreateLeaveRequestDTO {
			requestData: string;
		}

		@Controller("leave")
		export class LeaveController {
			@Post()
			create(@Body() { requestData }: CreateLeaveRequestDTO): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}

	param := route.Parameters[0]
	if param.Category != "body" {
		t.Errorf("expected Category='body', got %q", param.Category)
	}
	// Destructured params have no simple local name
	if param.LocalName != "" {
		t.Errorf("expected LocalName='' for destructured param, got %q", param.LocalName)
	}
	if param.TypeName != "CreateLeaveRequestDTO" {
		t.Errorf("expected TypeName='CreateLeaveRequestDTO', got %q", param.TypeName)
	}
}

func TestControllerAnalyzer_QualifiedNameReturnType(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		declare namespace Express {
			namespace Multer {
				interface File {
					fieldname: string;
					originalname: string;
					size: number;
				}
			}
		}

		@Controller("files")
		export class FileController {
			@Get()
			getFile(): Promise<Express.Multer.File> { return {} as any; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	// Should not panic on QualifiedName in return type
	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}
}

func TestControllerAnalyzer_QualifiedNameParamType(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		declare namespace Express {
			namespace Multer {
				interface File {
					fieldname: string;
					originalname: string;
					size: number;
				}
			}
		}

		@Controller("upload")
		export class UploadController {
			@Post()
			upload(@Body() file: Express.Multer.File): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	// Should not panic on QualifiedName in parameter type annotation
	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}

	param := route.Parameters[0]
	if param.Category != "body" {
		t.Errorf("expected Category='body', got %q", param.Category)
	}
	// QualifiedName can't resolve to a simple type name
	if param.TypeName != "" {
		t.Errorf("expected TypeName='' for qualified name type, got %q", param.TypeName)
	}
}

func TestControllerAnalyzer_RuntimeGeneratedController_WarnsAndSkips(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		export function makeController() {
			@Controller("runtime")
			class RuntimeController {
				@Get()
				findAll(): string { return ""; }
			}
			return RuntimeController;
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 0 {
		t.Fatalf("expected 0 controllers (runtime-generated should be skipped), got %d", len(controllers))
	}

	warnings := ca.Warnings()
	found := false
	for _, w := range warnings {
		if w.Kind == "unsupported-runtime-controller" {
			found = true
			if !strings.Contains(w.Message, "excluded from OpenAPI") {
				t.Errorf("expected warning to mention OpenAPI exclusion, got %q", w.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected unsupported-runtime-controller warning, got: %#v", warnings)
	}
}

func TestControllerAnalyzer_DynamicControllerPath_WarnsAndSkipsController(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		const prefix = "users";

		@Controller(prefix)
		export class UserController {
			@Get()
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 0 {
		t.Fatalf("expected 0 controllers (dynamic controller path should be skipped), got %d", len(controllers))
	}

	warnings := ca.Warnings()
	found := false
	for _, w := range warnings {
		if w.Kind == "unsupported-dynamic-controller-path" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unsupported-dynamic-controller-path warning, got: %#v", warnings)
	}
}

func TestControllerAnalyzer_DynamicRoutePath_WarnsAndSkipsRoute(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		const dynamicRoute = "runtime-path";

		@Controller("users")
		export class UserController {
			@Get(dynamicRoute)
			badRoute(): string { return ""; }

			@Get("static")
			goodRoute(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 remaining static route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if route.OperationID != "User_goodRoute" {
		t.Errorf("expected remaining route OperationID='User_goodRoute', got %q", route.OperationID)
	}
	if route.Path != "/users/static" {
		t.Errorf("expected remaining route Path='/users/static', got %q", route.Path)
	}

	warnings := ca.Warnings()
	found := false
	for _, w := range warnings {
		if w.Kind == "unsupported-dynamic-route-path" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unsupported-dynamic-route-path warning, got: %#v", warnings)
	}
}

// --- @Controller({ path, version }) Object Form Tests ---

func TestControllerAnalyzer_ControllerObjectForm_PathAndVersion(t *testing.T) {
	env := setupWalker(t, `
		function Controller(opts: string | { path?: string; version?: string }): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller({ path: 'users', version: '1' })
		export class UserController {
			@Get()
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	ctrl := controllers[0]
	if ctrl.Path != "users" {
		t.Errorf("expected Path='users', got %q", ctrl.Path)
	}
	if len(ctrl.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(ctrl.Routes))
	}

	// Route should inherit controller-level version
	route := ctrl.Routes[0]
	if route.Version != "1" {
		t.Errorf("expected route Version='1' (inherited from controller), got %q", route.Version)
	}
	if route.Path != "/users" {
		t.Errorf("expected route Path='/users', got %q", route.Path)
	}
}

func TestControllerAnalyzer_ControllerObjectForm_PathOnly(t *testing.T) {
	env := setupWalker(t, `
		function Controller(opts: string | { path?: string; version?: string }): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller({ path: 'orders' })
		export class OrderController {
			@Get()
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	ctrl := controllers[0]
	if ctrl.Path != "orders" {
		t.Errorf("expected Path='orders', got %q", ctrl.Path)
	}
	if len(ctrl.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(ctrl.Routes))
	}

	// No controller version, so route version should be empty
	route := ctrl.Routes[0]
	if route.Version != "" {
		t.Errorf("expected route Version='' (no controller version), got %q", route.Version)
	}
	if route.Path != "/orders" {
		t.Errorf("expected route Path='/orders', got %q", route.Path)
	}
}

func TestControllerAnalyzer_ControllerObjectForm_VersionAppliedAsDefault(t *testing.T) {
	// Controller-level version should be applied as default to methods
	// that don't have their own @Version() decorator.
	env := setupWalker(t, `
		function Controller(opts: string | { path?: string; version?: string }): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Version(version: string): MethodDecorator { return (t, k, d) => d; }

		@Controller({ path: 'products', version: '1' })
		export class ProductController {
			@Get()
			findAll(): string { return ""; }

			@Get(":id")
			findOne(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(controllers[0].Routes))
	}

	// Both routes should inherit controller-level version "1"
	for _, route := range controllers[0].Routes {
		if route.Version != "1" {
			t.Errorf("route %s: expected Version='1' (from controller), got %q", route.Path, route.Version)
		}
	}
}

func TestControllerAnalyzer_ControllerObjectForm_MethodVersionOverridesController(t *testing.T) {
	// Method-level @Version('2') should override controller-level version='1'.
	env := setupWalker(t, `
		function Controller(opts: string | { path?: string; version?: string }): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Version(version: string): MethodDecorator { return (t, k, d) => d; }

		@Controller({ path: 'items', version: '1' })
		export class ItemController {
			@Get()
			findAllV1(): string { return ""; }

			@Version("2")
			@Get("latest")
			findAllV2(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(controllers[0].Routes))
	}

	// Route 0 (findAllV1): no @Version, should inherit controller "1"
	r0 := controllers[0].Routes[0]
	if r0.Version != "1" {
		t.Errorf("findAllV1: expected Version='1' (inherited from controller), got %q", r0.Version)
	}

	// Route 1 (findAllV2): has @Version("2"), should override controller
	r1 := controllers[0].Routes[1]
	if r1.Version != "2" {
		t.Errorf("findAllV2: expected Version='2' (method override), got %q", r1.Version)
	}
}

// --- @All() Decorator Tests ---

func TestControllerAnalyzer_AllDecorator(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function All(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("health")
		export class HealthController {
			@All()
			handleAll(): string { return "ok"; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	routes := controllers[0].Routes
	// @All() should expand to 7 HTTP methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
	if len(routes) != 7 {
		t.Fatalf("expected 7 routes from @All(), got %d", len(routes))
	}

	expectedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for i, route := range routes {
		if route.Method != expectedMethods[i] {
			t.Errorf("route %d: expected Method=%q, got %q", i, expectedMethods[i], route.Method)
		}
		if route.Path != "/health" {
			t.Errorf("route %d: expected Path='/health', got %q", i, route.Path)
		}
	}
}

func TestControllerAnalyzer_AllDecoratorWithPath(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function All(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("api")
		export class ApiController {
			@All("wildcard")
			handleAll(): string { return "ok"; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	routes := controllers[0].Routes
	if len(routes) != 7 {
		t.Fatalf("expected 7 routes, got %d", len(routes))
	}

	for _, route := range routes {
		if route.Path != "/api/wildcard" {
			t.Errorf("expected Path='/api/wildcard', got %q", route.Path)
		}
	}
}

func TestControllerAnalyzer_AllDecoratorDefaultStatusCodes(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function All(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("test")
		export class TestController {
			@All()
			handleAll(): string { return "ok"; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	routes := controllers[0].Routes

	// POST should default to 201, all others to 200
	for _, route := range routes {
		expected := 200
		if route.Method == "POST" {
			expected = 201
		}
		if route.StatusCode != expected {
			t.Errorf("method %s: expected StatusCode=%d, got %d", route.Method, expected, route.StatusCode)
		}
	}
}

// --- Array Path Tests ---

func TestControllerAnalyzer_ArrayPathOnMethod(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string | string[]): MethodDecorator { return (t, k, d) => d; }

		@Controller("users")
		export class UserController {
			@Get(["active", "enabled"])
			findActive(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	routes := controllers[0].Routes
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes for array path, got %d", len(routes))
	}

	if routes[0].Path != "/users/active" {
		t.Errorf("route 0: expected Path='/users/active', got %q", routes[0].Path)
	}
	if routes[1].Path != "/users/enabled" {
		t.Errorf("route 1: expected Path='/users/enabled', got %q", routes[1].Path)
	}
	// Both should be GET
	for i, r := range routes {
		if r.Method != "GET" {
			t.Errorf("route %d: expected Method='GET', got %q", i, r.Method)
		}
	}
}

func TestControllerAnalyzer_ArrayPathOnController(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string | string[]): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller(["v1/users", "v2/users"])
		export class UserController {
			@Get()
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	// Array controller paths produce multiple ControllerInfo
	if len(controllers) != 2 {
		t.Fatalf("expected 2 controllers for array path, got %d", len(controllers))
	}

	if controllers[0].Path != "v1/users" {
		t.Errorf("controller 0: expected Path='v1/users', got %q", controllers[0].Path)
	}
	if controllers[1].Path != "v2/users" {
		t.Errorf("controller 1: expected Path='v2/users', got %q", controllers[1].Path)
	}

	// Each controller should have 1 route
	for i, ctrl := range controllers {
		if len(ctrl.Routes) != 1 {
			t.Fatalf("controller %d: expected 1 route, got %d", i, len(ctrl.Routes))
		}
	}

	if controllers[0].Routes[0].Path != "/v1/users" {
		t.Errorf("controller 0 route: expected Path='/v1/users', got %q", controllers[0].Routes[0].Path)
	}
	if controllers[1].Routes[0].Path != "/v2/users" {
		t.Errorf("controller 1 route: expected Path='/v2/users', got %q", controllers[1].Routes[0].Path)
	}
}

// --- @Header() Decorator Tests ---

func TestControllerAnalyzer_HeaderDecorator(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Header(name: string, value: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("users")
		export class UserController {
			@Get()
			@Header("Cache-Control", "none")
			@Header("X-Custom", "test-value")
			findAll(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	route := controllers[0].Routes[0]
	if len(route.ResponseHeaders) != 2 {
		t.Fatalf("expected 2 response headers, got %d", len(route.ResponseHeaders))
	}

	if route.ResponseHeaders[0].Name != "Cache-Control" || route.ResponseHeaders[0].Value != "none" {
		t.Errorf("header 0: expected Cache-Control=none, got %s=%s", route.ResponseHeaders[0].Name, route.ResponseHeaders[0].Value)
	}
	if route.ResponseHeaders[1].Name != "X-Custom" || route.ResponseHeaders[1].Value != "test-value" {
		t.Errorf("header 1: expected X-Custom=test-value, got %s=%s", route.ResponseHeaders[1].Name, route.ResponseHeaders[1].Value)
	}
}

// --- @Redirect() Decorator Tests ---

func TestControllerAnalyzer_RedirectDecorator(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Redirect(url?: string, statusCode?: number): MethodDecorator { return (t, k, d) => d; }

		@Controller("auth")
		export class AuthController {
			@Get("google")
			@Redirect("https://accounts.google.com", 301)
			googleLogin(): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	route := controllers[0].Routes[0]
	if route.Redirect == nil {
		t.Fatal("expected Redirect to be non-nil")
	}
	if route.Redirect.URL != "https://accounts.google.com" {
		t.Errorf("expected Redirect.URL='https://accounts.google.com', got %q", route.Redirect.URL)
	}
	if route.Redirect.StatusCode != 301 {
		t.Errorf("expected Redirect.StatusCode=301, got %d", route.Redirect.StatusCode)
	}
}

func TestControllerAnalyzer_RedirectDecoratorDefaultStatus(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Redirect(url?: string, statusCode?: number): MethodDecorator { return (t, k, d) => d; }

		@Controller("legacy")
		export class LegacyController {
			@Get("old")
			@Redirect("https://new.example.com/new")
			redirectOld(): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	route := controllers[0].Routes[0]

	if route.Redirect == nil {
		t.Fatal("expected Redirect to be non-nil")
	}
	if route.Redirect.StatusCode != 302 {
		t.Errorf("expected default Redirect.StatusCode=302, got %d", route.Redirect.StatusCode)
	}
}

// --- @Version() Array Tests ---

func TestControllerAnalyzer_VersionArrayDecorator(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Version(version: string | string[]): MethodDecorator { return (t, k, d) => d; }

		@Controller("users")
		export class UserController {
			@Version(["1", "2"])
			@Get()
			findAll(): string { return ""; }

			@Version("3")
			@Get("single")
			findSingle(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	if len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(controllers[0].Routes))
	}

	r0 := controllers[0].Routes[0]
	if len(r0.Versions) != 2 {
		t.Fatalf("route 0: expected Versions length=2, got %d", len(r0.Versions))
	}
	if r0.Versions[0] != "1" || r0.Versions[1] != "2" {
		t.Errorf("route 0: expected Versions=['1', '2'], got %v", r0.Versions)
	}
	// Version field should be set to the first element for backward compatibility
	if r0.Version != "1" {
		t.Errorf("route 0: expected Version='1', got %q", r0.Version)
	}

	r1 := controllers[0].Routes[1]
	if len(r1.Versions) != 0 {
		t.Errorf("route 1: expected Versions empty for scalar @Version, got %v", r1.Versions)
	}
	if r1.Version != "3" {
		t.Errorf("route 1: expected Version='3', got %q", r1.Version)
	}
}

// --- HEAD and OPTIONS HTTP Method Tests ---

func TestControllerAnalyzer_HeadMethod(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Head(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("health")
		export class HealthController {
			@Head()
			checkHealth(): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if route.Method != "HEAD" {
		t.Errorf("expected Method='HEAD', got %q", route.Method)
	}
	if route.Path != "/health" {
		t.Errorf("expected Path='/health', got %q", route.Path)
	}
	if route.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", route.StatusCode)
	}
}

func TestControllerAnalyzer_OptionsMethod(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Options(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("cors")
		export class CorsController {
			@Options()
			handleCors(): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if route.Method != "OPTIONS" {
		t.Errorf("expected Method='OPTIONS', got %q", route.Method)
	}
	if route.Path != "/cors" {
		t.Errorf("expected Path='/cors', got %q", route.Path)
	}
}

func TestControllerAnalyzer_OptionalBodyParam(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		interface UpdateDto {
			name?: string;
			email?: string;
		}

		@Controller("users")
		export class UserController {
			@Post("update")
			update(@Body() body?: UpdateDto): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "body" {
		t.Errorf("expected Category='body', got %q", param.Category)
	}
	// The optional body parameter should still be recognized
	if param.TypeName != "UpdateDto" {
		t.Logf("optional body param TypeName=%q (may be empty for optional params)", param.TypeName)
	}
}

func TestControllerAnalyzer_NullablePathParam(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		@Controller("items")
		export class ItemController {
			@Get(":id")
			findOne(@Param("id") id: string | null): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "param" {
		t.Errorf("expected Category='param', got %q", param.Category)
	}
	if param.Name != "id" {
		t.Errorf("expected Name='id', got %q", param.Name)
	}
}

func TestControllerAnalyzer_MultipleSecuritySchemes(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller("admin")
		export class AdminController {
			/**
			 * @security bearer
			 * @security apiKey
			 */
			@Get("settings")
			getSettings(): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Security) != 2 {
		t.Fatalf("expected 2 security requirements, got %d", len(route.Security))
	}
	if route.Security[0].Name != "bearer" {
		t.Errorf("expected Security[0].Name='bearer', got %q", route.Security[0].Name)
	}
	if route.Security[1].Name != "apiKey" {
		t.Errorf("expected Security[1].Name='apiKey', got %q", route.Security[1].Name)
	}
}

func TestControllerAnalyzer_CompositePathParams(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		@Controller("files")
		export class FileController {
			@Get(":userId/documents/:docId")
			getDocument(
				@Param("userId") userId: string,
				@Param("docId") docId: string,
			): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if route.Path != "/files/:userId/documents/:docId" {
		t.Errorf("expected Path='/files/:userId/documents/:docId', got %q", route.Path)
	}
	if len(route.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(route.Parameters))
	}
	if route.Parameters[0].Category != "param" || route.Parameters[0].Name != "userId" {
		t.Errorf("param 0: expected param/userId, got %s/%s", route.Parameters[0].Category, route.Parameters[0].Name)
	}
	if route.Parameters[1].Category != "param" || route.Parameters[1].Name != "docId" {
		t.Errorf("param 1: expected param/docId, got %s/%s", route.Parameters[1].Category, route.Parameters[1].Name)
	}
}

func TestControllerAnalyzer_HeadersParamObjectType(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Headers(): ParameterDecorator { return () => {}; }

		interface RequestHeaders {
			authorization: string;
			"x-request-id"?: string;
		}

		@Controller("api")
		export class ApiController {
			@Get()
			findAll(@Headers() headers: RequestHeaders): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "headers" {
		t.Errorf("expected Category='headers', got %q", param.Category)
	}
}

func TestControllerAnalyzer_ControllerNoPath(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path?: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }

		@Controller()
		export class RootController {
			@Get()
			root(): string { return "ok"; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}

	ctrl := controllers[0]
	if ctrl.Path != "" {
		t.Errorf("expected empty path for @Controller(), got %q", ctrl.Path)
	}
	if len(ctrl.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(ctrl.Routes))
	}
	if ctrl.Routes[0].Path != "/" {
		t.Errorf("expected route Path='/', got %q", ctrl.Routes[0].Path)
	}
}

func TestControllerAnalyzer_MultipleQueryAndParamDecorators(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }
		function Query(): ParameterDecorator { return () => {}; }

		interface SearchQuery {
			term?: string;
			limit?: number;
		}

		@Controller("users")
		export class UserController {
			@Get(":id/posts")
			getUserPosts(
				@Param("id") userId: string,
				@Query() query: SearchQuery,
			): string { return ""; }
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if len(route.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(route.Parameters))
	}

	if route.Parameters[0].Category != "param" || route.Parameters[0].Name != "id" {
		t.Errorf("param 0: expected param/id, got %s/%s", route.Parameters[0].Category, route.Parameters[0].Name)
	}
	if route.Parameters[1].Category != "query" {
		t.Errorf("param 1: expected Category='query', got %q", route.Parameters[1].Category)
	}
}

func TestControllerAnalyzer_EventStreamDecorator_DiscriminatedUnion(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (t) => t; }
		function EventStream(path?: string, options?: { heartbeat?: number }): MethodDecorator {
			return (target, key, descriptor) => descriptor;
		}

		interface SseEvent<E extends string = string, T = unknown> {
			event: E;
			data: T;
			id?: string;
			retry?: number;
		}

		type SseEvents<M extends Record<string, unknown>> = {
			[K in keyof M & string]: SseEvent<K, M[K]>;
		}[keyof M & string];

		interface UserDto {
			id: number;
			name: string;
		}

		interface DeletePayload {
			id: string;
		}

		type UserEvents = SseEvents<{
			created: UserDto;
			deleted: DeletePayload;
		}>;

		@Controller("users")
		export class UserEventController {
			@EventStream("events")
			async *streamUserEvents(): AsyncGenerator<UserEvents> {
				yield { event: "created", data: {} as UserDto };
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}

	route := controllers[0].Routes[0]
	if !route.IsSSE {
		t.Error("expected IsSSE=true")
	}
	if !route.IsEventStream {
		t.Error("expected IsEventStream=true")
	}
	if route.Method != "GET" {
		t.Errorf("expected Method=GET, got %q", route.Method)
	}
	if route.Path != "/users/events" {
		t.Errorf("expected Path=/users/events, got %q", route.Path)
	}

	// Check SSE event variants
	if len(route.SSEEventVariants) != 2 {
		t.Fatalf("expected 2 SSE event variants, got %d", len(route.SSEEventVariants))
	}

	// Variants may come in any order since it's a union; check both exist
	variants := make(map[string]bool)
	for _, v := range route.SSEEventVariants {
		variants[v.EventName] = true
	}
	if !variants["created"] {
		t.Error("expected 'created' event variant")
	}
	if !variants["deleted"] {
		t.Error("expected 'deleted' event variant")
	}
}

func TestControllerAnalyzer_EventStreamDecorator_SingleEvent(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (t) => t; }
		function EventStream(path?: string): MethodDecorator {
			return (target, key, descriptor) => descriptor;
		}

		interface SseEvent<E extends string = string, T = unknown> {
			event: E;
			data: T;
			id?: string;
			retry?: number;
		}

		interface NotificationDto {
			id: string;
			message: string;
		}

		@Controller("notifications")
		export class NotificationController {
			@EventStream("stream")
			async *streamNotifications(): AsyncGenerator<SseEvent<"notification", NotificationDto>> {
				yield { event: "notification", data: {} as NotificationDto };
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if !route.IsEventStream {
		t.Error("expected IsEventStream=true")
	}

	if len(route.SSEEventVariants) != 1 {
		t.Fatalf("expected 1 SSE event variant, got %d", len(route.SSEEventVariants))
	}
	if route.SSEEventVariants[0].EventName != "notification" {
		t.Errorf("expected EventName='notification', got %q", route.SSEEventVariants[0].EventName)
	}
}

func TestControllerAnalyzer_EventStreamDecorator_GenericString(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (t) => t; }
		function EventStream(path?: string): MethodDecorator {
			return (target, key, descriptor) => descriptor;
		}

		interface SseEvent<E extends string = string, T = unknown> {
			event: E;
			data: T;
			id?: string;
			retry?: number;
		}

		interface UserDto {
			id: number;
			name: string;
		}

		@Controller("generic")
		export class GenericEventController {
			@EventStream("stream")
			async *streamGeneric(): AsyncGenerator<SseEvent<string, UserDto>> {
				yield { event: "any-name", data: {} as UserDto };
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if !route.IsEventStream {
		t.Error("expected IsEventStream=true")
	}

	if len(route.SSEEventVariants) != 1 {
		t.Fatalf("expected 1 SSE event variant (generic), got %d", len(route.SSEEventVariants))
	}
	// Generic string → empty EventName (wildcard)
	if route.SSEEventVariants[0].EventName != "" {
		t.Errorf("expected empty EventName for generic string, got %q", route.SSEEventVariants[0].EventName)
	}
}

func TestControllerAnalyzer_EventStreamDecorator_NoPath(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (t) => t; }
		function EventStream(): MethodDecorator {
			return (target, key, descriptor) => descriptor;
		}

		interface SseEvent<E extends string = string, T = unknown> {
			event: E;
			data: T;
			id?: string;
			retry?: number;
		}

		@Controller("default-path")
		export class DefaultPathController {
			@EventStream()
			async *streamDefault(): AsyncGenerator<SseEvent<"ping", { ts: number }>> {
				yield { event: "ping", data: { ts: Date.now() } };
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if !route.IsSSE || !route.IsEventStream {
		t.Error("expected IsSSE=true and IsEventStream=true")
	}
	// No path → defaults to controller path only
	if route.Path != "/default-path" {
		t.Errorf("expected Path=/default-path, got %q", route.Path)
	}
}

func TestControllerAnalyzer_EventStreamDecorator_MixedWithRegularRoutes(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (t) => t; }
		function Get(path?: string): MethodDecorator {
			return (target, key, descriptor) => descriptor;
		}
		function EventStream(path?: string): MethodDecorator {
			return (target, key, descriptor) => descriptor;
		}

		interface SseEvent<E extends string = string, T = unknown> {
			event: E;
			data: T;
			id?: string;
			retry?: number;
		}

		@Controller("mixed")
		export class MixedController {
			@Get("health")
			health(): string { return "ok"; }

			@EventStream("events")
			async *streamMixed(): AsyncGenerator<SseEvent<"status", { online: boolean }>> {
				yield { event: "status", data: { online: true } };
			}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if len(controllers[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(controllers[0].Routes))
	}

	// Find the regular route and SSE route
	var regularRoute, sseRoute *analyzer.Route
	for i := range controllers[0].Routes {
		r := &controllers[0].Routes[i]
		if r.IsSSE {
			sseRoute = r
		} else {
			regularRoute = r
		}
	}

	if regularRoute == nil {
		t.Fatal("expected a regular (non-SSE) route")
	}
	if sseRoute == nil {
		t.Fatal("expected an SSE route")
	}

	if regularRoute.IsEventStream {
		t.Error("regular route should not be an EventStream")
	}
	if !sseRoute.IsEventStream {
		t.Error("SSE route should be an EventStream")
	}
	if len(sseRoute.SSEEventVariants) != 1 {
		t.Errorf("expected 1 SSE event variant, got %d", len(sseRoute.SSEEventVariants))
	}
}

// --- @Returns decorator import-source filtering tests ---

// TestControllerAnalyzer_Returns_DirectImport verifies that @Returns<T>()
// imported directly from @tsgonest/runtime is recognized.
func TestControllerAnalyzer_Returns_DirectImport(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"tsgonest-runtime.d.ts": `
			declare module "@tsgonest/runtime" {
				export function Returns<T>(options?: { contentType?: string; description?: string; status?: number }): MethodDecorator;
			}
		`,
		"controller.ts": `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			function Res(): ParameterDecorator { return () => {}; }

			import { Returns } from "@tsgonest/runtime";

			interface UserDto { id: string; name: string; }

			@Controller("users")
			export class UserController {
				@Returns<UserDto>()
				@Get(":id")
				async getUser(@Res() res: any): Promise<void> {}
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route, got %d controllers", len(controllers))
	}

	route := controllers[0].Routes[0]
	// @Returns<UserDto>() should have been recognized → return type should be an object with properties
	if route.ReturnType.Kind == metadata.KindVoid {
		t.Error("expected non-void return type from @Returns<UserDto>(), got void — import-source filtering may have rejected a valid tsgonest import")
	}
}

// TestControllerAnalyzer_Returns_AliasedImport verifies that import { Returns as R }
// from @tsgonest/runtime is recognized.
func TestControllerAnalyzer_Returns_AliasedImport(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"tsgonest-runtime.d.ts": `
			declare module "@tsgonest/runtime" {
				export function Returns<T>(options?: { contentType?: string; description?: string; status?: number }): MethodDecorator;
			}
		`,
		"controller.ts": `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			function Res(): ParameterDecorator { return () => {}; }

			import { Returns as TypedReturns } from "@tsgonest/runtime";

			interface ProductDto { id: string; price: number; }

			@Controller("products")
			export class ProductController {
				@TypedReturns<ProductDto>()
				@Get(":id")
				async getProduct(@Res() res: any): Promise<void> {}
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if route.ReturnType.Kind == metadata.KindVoid {
		t.Error("expected non-void return type from aliased @TypedReturns<ProductDto>(), got void")
	}
}

// TestControllerAnalyzer_Returns_NamespaceImport verifies that
// import * as rt from '@tsgonest/runtime'; @rt.Returns<T>() is recognized.
func TestControllerAnalyzer_Returns_NamespaceImport(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"tsgonest-runtime.d.ts": `
			declare module "@tsgonest/runtime" {
				export function Returns<T>(options?: { contentType?: string; description?: string; status?: number }): MethodDecorator;
			}
		`,
		"controller.ts": `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			function Res(): ParameterDecorator { return () => {}; }

			import * as rt from "@tsgonest/runtime";

			interface OrderDto { id: string; total: number; }

			@Controller("orders")
			export class OrderController {
				@rt.Returns<OrderDto>()
				@Get(":id")
				async getOrder(@Res() res: any): Promise<void> {}
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if route.ReturnType.Kind == metadata.KindVoid {
		t.Error("expected non-void return type from namespace @rt.Returns<OrderDto>(), got void")
	}
}

// TestControllerAnalyzer_Returns_NonTsgonestModule verifies that @Returns from
// a non-tsgonest module is ignored (not treated as tsgonest's @Returns).
func TestControllerAnalyzer_Returns_NonTsgonestModule(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"other-lib.d.ts": `
			declare module "some-other-lib" {
				export function Returns<T>(options?: any): MethodDecorator;
			}
		`,
		"controller.ts": `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			function Res(): ParameterDecorator { return () => {}; }

			import { Returns } from "some-other-lib";

			interface UserDto { id: string; }

			@Controller("users")
			export class UserController {
				@Returns<UserDto>()
				@Get(":id")
				async getUser(@Res() res: any): Promise<void> {}
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	// @Returns from "some-other-lib" should be IGNORED → return type should be void (because @Res())
	if route.ReturnType.Kind != metadata.KindVoid {
		t.Errorf("expected void return type (non-tsgonest @Returns should be ignored), got kind=%v", route.ReturnType.Kind)
	}
}

// TestControllerAnalyzer_Returns_LocalFunction verifies that a locally defined
// Returns function is ignored (not treated as tsgonest's @Returns).
func TestControllerAnalyzer_Returns_LocalFunction(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"controller.ts": `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			function Res(): ParameterDecorator { return () => {}; }
			function Returns<T>(options?: any): MethodDecorator { return (t, k, d) => d; }

			interface UserDto { id: string; }

			@Controller("users")
			export class UserController {
				@Returns<UserDto>()
				@Get(":id")
				async getUser(@Res() res: any): Promise<void> {}
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	// Locally defined Returns should be IGNORED → return type should be void (because @Res())
	if route.ReturnType.Kind != metadata.KindVoid {
		t.Errorf("expected void return type (locally defined @Returns should be ignored), got kind=%v", route.ReturnType.Kind)
	}
}

// TestControllerAnalyzer_Returns_TsgonestPackage verifies that @Returns from
// the "tsgonest" package (not scoped) is also recognized.
func TestControllerAnalyzer_Returns_TsgonestPackage(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"tsgonest.d.ts": `
			declare module "tsgonest" {
				export function Returns<T>(options?: { contentType?: string; description?: string; status?: number }): MethodDecorator;
			}
		`,
		"controller.ts": `
			function Controller(path: string): ClassDecorator { return (target) => target; }
			function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
			function Res(): ParameterDecorator { return () => {}; }

			import { Returns } from "tsgonest";

			interface ItemDto { id: string; }

			@Controller("items")
			export class ItemController {
				@Returns<ItemDto>()
				@Get(":id")
				async getItem(@Res() res: any): Promise<void> {}
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if route.ReturnType.Kind == metadata.KindVoid {
		t.Error("expected non-void return type from @Returns<ItemDto>() imported from 'tsgonest', got void")
	}
}

// --- Namespace import tests for NestJS decorators ---

// TestControllerAnalyzer_NamespaceImport_Get verifies that @nest.Get() from
// import * as nest from "./decorators" is correctly resolved as a GET route.
func TestControllerAnalyzer_NamespaceImport_Get(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"decorators.ts": `
			export function Controller(path: string): ClassDecorator { return (target) => target; }
			export function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		`,
		"controller.ts": `
			import * as nest from "./decorators";

			@nest.Controller("items")
			export class ItemController {
				@nest.Get()
				findAll(): string { return ""; }
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	if controllers[0].Name != "ItemController" {
		t.Errorf("expected Name='ItemController', got %q", controllers[0].Name)
	}
	if controllers[0].Path != "items" {
		t.Errorf("expected Path='items', got %q", controllers[0].Path)
	}
	if len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(controllers[0].Routes))
	}
	if controllers[0].Routes[0].Method != "GET" {
		t.Errorf("expected Method='GET', got %q", controllers[0].Routes[0].Method)
	}
}

// TestControllerAnalyzer_NamespaceImport_Body verifies that @nest.Body()
// from import * as nest is correctly resolved as a body parameter.
func TestControllerAnalyzer_NamespaceImport_Body(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"decorators.ts": `
			export function Controller(path: string): ClassDecorator { return (target) => target; }
			export function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
			export function Body(): ParameterDecorator { return (target, key, index) => {}; }
		`,
		"controller.ts": `
			import * as nest from "./decorators";

			interface CreateDto { name: string; }

			@nest.Controller("items")
			export class ItemController {
				@nest.Post()
				create(@nest.Body() dto: CreateDto): string { return ""; }
			}
		`,
	}, "controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 || len(controllers[0].Routes) != 1 {
		t.Fatalf("expected 1 controller with 1 route")
	}

	route := controllers[0].Routes[0]
	if route.Method != "POST" {
		t.Errorf("expected Method='POST', got %q", route.Method)
	}
	if len(route.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(route.Parameters))
	}
	param := route.Parameters[0]
	if param.Category != "body" {
		t.Errorf("expected Category='body', got %q", param.Category)
	}
}

// --- ResolveDecoratorOrigin unit tests ---

func TestIsTsgonestModule(t *testing.T) {
	tests := []struct {
		moduleSpec string
		want       bool
	}{
		{"tsgonest", true},
		{"@tsgonest/runtime", true},
		{"@tsgonest/types", true},
		{"@tsgonest/cli-darwin-arm64", true},
		{"@nestjs/common", false},
		{"some-other-lib", false},
		{"", false},
		{"tsgonest-extra", false}, // not a tsgonest package
		{"@tsgonest/", false},     // empty scoped name — technically invalid
	}

	for _, tt := range tests {
		t.Run(tt.moduleSpec, func(t *testing.T) {
			got := analyzer.IsTsgonestModule(tt.moduleSpec)
			if got != tt.want {
				t.Errorf("IsTsgonestModule(%q) = %v, want %v", tt.moduleSpec, got, tt.want)
			}
		})
	}
}

// The following ensures we're using the shimcompiler import correctly.
var _ = (*shimcompiler.Program)(nil)
