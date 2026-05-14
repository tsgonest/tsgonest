package codegen

import "strings"

// MintRouteInfo describes a single route to register with a Mint app router.
type MintRouteInfo struct {
	Method     string
	Path       string
	MethodName string
}

// MintRegisterInput describes the data needed to emit a registerXxxController() helper.
type MintRegisterInput struct {
	ControllerName       string
	ControllerImportPath string
	Routes               []MintRouteInfo
}

// GenerateMintRegister returns ESM JS source for a companion file that exports
// `register{ControllerName}(app)`. The companion imports the controller class
// and registers each route with app.router.add(...).
func GenerateMintRegister(input MintRegisterInput) string {
	e := NewEmitter()
	e.Line("import { %s } from %q;", input.ControllerName, input.ControllerImportPath)
	e.Blank()
	e.Block("export function %s(app)", mintRegisterFnName(input.ControllerName))
	for _, r := range input.Routes {
		e.Block("app.router.add(%q, %q, async (event) =>", r.Method, r.Path)
		e.Line("const ctrl = event.resolve(%s);", input.ControllerName)
		e.Line("const result = await ctrl.%s(event);", r.MethodName)
		e.Line("if (result instanceof Response) return result;")
		e.Line("return new Response(result == null ? \"\" : String(result));")
		e.EndBlockSuffix(");")
	}
	e.EndBlock()
	return e.String()
}

// GenerateMintRegisterTypes returns the corresponding .tsgonest.d.ts content.
func GenerateMintRegisterTypes(controllerName string) string {
	return "export declare function " + mintRegisterFnName(controllerName) + "(app: any): void;\n"
}

// MintRegisterPath returns the companion file path for a Mint register helper.
// e.g. "src/hello.controller.ts" + "HelloController" → "src/hello.controller.HelloController.tsgonest.js"
//
// The companion sits next to the controller's emitted JS, mirroring the
// existing companion path convention.
func MintRegisterPath(sourceFileName string, controllerName string) string {
	return companionPath(sourceFileName, controllerName)
}

// mintRegisterFnName returns the exported function name. It does not strip the
// "Controller" suffix — `HelloController` → `registerHelloController`.
func mintRegisterFnName(controllerName string) string {
	return "register" + strings.TrimSpace(controllerName)
}
