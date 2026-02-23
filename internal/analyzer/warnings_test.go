package analyzer_test

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- Phase 10.5: Warning System Tests ---

func TestWarning_QueryNestedObject(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "query",
		Name:     "filter",
		Type: metadata.Metadata{
			Kind: metadata.KindObject,
			Properties: []metadata.Property{
				{
					Name: "nested",
					Type: metadata.Metadata{
						Kind: metadata.KindObject,
						Properties: []metadata.Property{
							{Name: "deep", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
						},
					},
				},
			},
		},
		Required: false,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(wc.Warnings))
	}
	w := wc.Warnings[0]
	if w.Kind != "query-complex-type" {
		t.Errorf("expected Kind='query-complex-type', got %q", w.Kind)
	}
	if w.File != "test.ts" {
		t.Errorf("expected File='test.ts', got %q", w.File)
	}
	if !strings.Contains(w.Message, "TestController.testMethod()") {
		t.Errorf("expected warning message to contain controller.method location, got %q", w.Message)
	}
	if !strings.Contains(w.Message, "test.ts:1") {
		t.Errorf("expected warning message to contain file:line, got %q", w.Message)
	}
}

func TestWarning_QueryFlatObject_NoWarning(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "query",
		Name:     "filter",
		Type: metadata.Metadata{
			Kind: metadata.KindObject,
			Properties: []metadata.Property{
				{Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
				{Name: "search", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			},
		},
		Required: false,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 0 {
		t.Errorf("expected 0 warnings for flat query object, got %d: %v", len(wc.Warnings), wc.Warnings)
	}
}

func TestWarning_ParamNonScalar_Object(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type: metadata.Metadata{
			Kind: metadata.KindObject,
			Properties: []metadata.Property{
				{Name: "value", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
			},
		},
		Required: true,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(wc.Warnings))
	}
	w := wc.Warnings[0]
	if w.Kind != "param-non-scalar" {
		t.Errorf("expected Kind='param-non-scalar', got %q", w.Kind)
	}
	if !strings.Contains(w.Message, "TestController.testMethod()") {
		t.Errorf("expected warning message to contain controller.method location, got %q", w.Message)
	}
}

func TestWarning_ParamNonScalar_Array(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "ids",
		Type: metadata.Metadata{
			Kind:        metadata.KindArray,
			ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		},
		Required: true,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(wc.Warnings))
	}
	w := wc.Warnings[0]
	if w.Kind != "param-non-scalar" {
		t.Errorf("expected Kind='param-non-scalar', got %q", w.Kind)
	}
}

func TestWarning_ParamScalar_NoWarning(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		Required: true,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 0 {
		t.Errorf("expected 0 warnings for scalar path param, got %d", len(wc.Warnings))
	}
}

func TestWarning_HeaderNull(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "headers",
		Name:     "x-token",
		Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true},
		Required: false,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(wc.Warnings))
	}
	w := wc.Warnings[0]
	if w.Kind != "header-null" {
		t.Errorf("expected Kind='header-null', got %q", w.Kind)
	}
	if !strings.Contains(w.Message, "TestController.testMethod()") {
		t.Errorf("expected warning message to contain controller.method location, got %q", w.Message)
	}
}

func TestWarning_HeaderNonNull_NoWarning(t *testing.T) {
	wc := analyzer.NewWarningCollector()

	param := &analyzer.RouteParameter{
		Category: "headers",
		Name:     "x-token",
		Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		Required: true,
	}

	analyzer.ValidateParameterType(param, wc, "test.ts", "TestController.testMethod() (test.ts:1)")

	if len(wc.Warnings) != 0 {
		t.Errorf("expected 0 warnings for non-null header, got %d", len(wc.Warnings))
	}
}

func TestWarning_NilCollector_NoOp(t *testing.T) {
	// Calling with nil collector should not panic
	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type:     metadata.Metadata{Kind: metadata.KindObject},
		Required: true,
	}

	// This should not panic
	analyzer.ValidateParameterType(param, nil, "test.ts", "TestController.testMethod() (test.ts:1)")
}

// --- Phase 2A: Enhanced Path Parameter Validation ---

func TestWarning_ParamAny(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type:     metadata.Metadata{Kind: metadata.KindAny},
		Required: true,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "param-any" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param-any warning for 'any' path param")
	}
}

func TestWarning_ParamOptional(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Optional: true},
		Required: false,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "param-optional" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param-optional warning for optional path param")
	}
}

func TestWarning_ParamNullable(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string", Nullable: true},
		Required: false,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "param-optional" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param-optional warning for nullable path param")
	}
}

func TestWarning_ParamNoName(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "",
		Type:     metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
		Required: true,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "param-no-name" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param-no-name warning for @Param() without field name")
	}
}

func TestWarning_ParamRef(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "param",
		Name:     "id",
		Type:     metadata.Metadata{Kind: metadata.KindRef, Name: "SomeDto"},
		Required: true,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "param-non-scalar" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param-non-scalar warning for ref-typed path param")
	}
}

// --- Phase 2B: Enhanced Query Parameter Validation ---

func TestWarning_QueryNullable(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "query",
		Name:     "", // whole-object query
		Type: metadata.Metadata{
			Kind:     metadata.KindObject,
			Nullable: true,
			Properties: []metadata.Property{
				{Name: "page", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}},
			},
		},
		Required: false,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "query-nullable" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected query-nullable warning for nullable query object")
	}
}

// --- Phase 2C: Enhanced Header Validation ---

func TestWarning_HeaderNestedObject(t *testing.T) {
	wc := analyzer.NewWarningCollector()
	param := &analyzer.RouteParameter{
		Category: "headers",
		Name:     "", // whole-object headers
		Type: metadata.Metadata{
			Kind: metadata.KindObject,
			Properties: []metadata.Property{
				{Name: "nested", Type: metadata.Metadata{
					Kind: metadata.KindObject,
					Properties: []metadata.Property{
						{Name: "deep", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}},
					},
				}},
			},
		},
		Required: false,
	}
	analyzer.ValidateParameterType(param, wc, "test.ts", "Ctrl.method() (test.ts:1)")
	found := false
	for _, w := range wc.Warnings {
		if w.Kind == "header-complex-type" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected header-complex-type warning for nested header object")
	}
}

// --- Phase 10.4: Content Type via Controller Analyzer ---

func TestControllerAnalyzer_ContentType_StringBody(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		@Controller("messages")
		export class MessageController {
			@Post()
			send(@Body() body: string): void {}
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
	if param.ContentType != "text/plain" {
		t.Errorf("expected ContentType='text/plain', got %q", param.ContentType)
	}
}

func TestControllerAnalyzer_ContentType_ObjectBody_Default(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		interface CreateDto { name: string; }

		@Controller("items")
		export class ItemController {
			@Post()
			create(@Body() body: CreateDto): void {}
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
	// Object body should have empty ContentType (defaults to application/json in generator)
	if param.ContentType != "" {
		t.Errorf("expected ContentType='' (default), got %q", param.ContentType)
	}
}

func TestControllerAnalyzer_ContentType_JSDocOverride(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		interface UploadDto { name: string; }

		@Controller("upload")
		export class UploadController {
			/**
			 * @contentType multipart/form-data
			 */
			@Post()
			upload(@Body() body: UploadDto): void {}
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
	if param.ContentType != "multipart/form-data" {
		t.Errorf("expected ContentType='multipart/form-data', got %q", param.ContentType)
	}
}

func TestControllerAnalyzer_Warnings_PathParamObject(t *testing.T) {
	env := setupWalker(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		@Controller("items")
		export class ItemController {
			@Get(":id")
			get(@Param("id") id: { value: string }): void {}
		}
	`)
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	ca.AnalyzeSourceFile(env.sourceFile)

	warnings := ca.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected at least 1 warning for object path param")
	}

	found := false
	for _, w := range warnings {
		if w.Kind == "param-non-scalar" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'param-non-scalar' warning, got: %v", warnings)
	}
}
