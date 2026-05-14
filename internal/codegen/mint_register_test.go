package codegen

import "testing"

func TestGenerateMintRegister_SingleGetRoute(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "HelloController",
		ControllerImportPath: "./hello.controller",
		Routes: []MintRouteInfo{
			{Method: "GET", Path: "/hello", MethodName: "hello"},
		},
	})

	wants := []string{
		`import { HelloController } from "./hello.controller";`,
		`export function registerHelloController(app) {`,
		`app.router.add("GET", "/hello", async (event) => {`,
		`const ctrl = event.resolve(HelloController);`,
		`const result = await ctrl.hello(event);`,
		`if (result instanceof Response) return result;`,
		`return new Response(result == null ? "" : String(result));`,
	}
	for _, w := range wants {
		assertContains(t, out, w)
	}
}

func TestGenerateMintRegister_MultipleRoutes(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{Method: "GET", Path: "/users", MethodName: "list"},
			{Method: "POST", Path: "/users", MethodName: "create"},
			{Method: "DELETE", Path: "/users/:id", MethodName: "remove"},
		},
	})

	assertContains(t, out, `export function registerUsersController(app) {`)
	assertContains(t, out, `app.router.add("GET", "/users", async (event) => {`)
	assertContains(t, out, `const result = await ctrl.list(event);`)
	assertContains(t, out, `app.router.add("POST", "/users", async (event) => {`)
	assertContains(t, out, `const result = await ctrl.create(event);`)
	assertContains(t, out, `app.router.add("DELETE", "/users/:id", async (event) => {`)
	assertContains(t, out, `const result = await ctrl.remove(event);`)
}

func TestGenerateMintRegister_ResponsePassthrough(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "PingController",
		ControllerImportPath: "./ping.controller",
		Routes: []MintRouteInfo{
			{Method: "GET", Path: "/ping", MethodName: "ping"},
		},
	})

	assertContains(t, out, "if (result instanceof Response) return result;")
}

func TestGenerateMintRegister_KeepsControllerSuffix(t *testing.T) {
	// Phase 1: do NOT strip "Controller" suffix.
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "HelloController",
		ControllerImportPath: "./hello.controller",
		Routes:               []MintRouteInfo{{Method: "GET", Path: "/", MethodName: "hello"}},
	})
	assertContains(t, out, "registerHelloController")
}

func TestGenerateMintRegisterTypes(t *testing.T) {
	out := GenerateMintRegisterTypes("HelloController")
	assertContains(t, out, "export declare function registerHelloController(app: any): void;")
}
