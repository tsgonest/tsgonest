package analyzer_test

import (
	"testing"

	"github.com/microsoft/typescript-go/shim/ast"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/tsgonest/tsgonest/internal/analyzer"
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
	if r0.OperationID != "findAll" {
		t.Errorf("route 0: expected OperationID='findAll', got %q", r0.OperationID)
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
	if r0.OperationID != "findAllV1" {
		t.Errorf("route 0: expected OperationID='findAllV1', got %q", r0.OperationID)
	}

	r1 := controllers[0].Routes[1]
	if r1.Version != "2" {
		t.Errorf("route 1: expected Version='2', got %q", r1.Version)
	}
	if r1.OperationID != "findAllV2" {
		t.Errorf("route 1: expected OperationID='findAllV2', got %q", r1.OperationID)
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

// The following ensures we're using the shimcompiler import correctly.
var _ = (*shimcompiler.Program)(nil)
