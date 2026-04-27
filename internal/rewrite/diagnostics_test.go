package rewrite

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// TestRewriteController_ErrorWhenClassNotLocated covers the issue #114 footprint:
// the analyzer recognizes a controller and there's injection work to do, but
// the locator can't find the class in emitted JS. This must surface as an
// error so the build fails — silently skipping means unvalidated input hits
// the handler.
func TestRewriteController_ErrorWhenClassNotLocated(t *testing.T) {
	// Emitted JS with no class at all — controller name won't be located.
	input := `"use strict";
const someValue = 42;
`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "GhostController",
			SourceFile: "/src/ghost.controller.ts",
			Routes: []analyzer.Route{
				{
					MethodName: "create",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "body",
							LocalName: "body",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateGhostDto"},
						},
					},
				},
			},
		},
	}
	companionMap := map[string]string{
		"CreateGhostDto": "/dist/ghost.dto.CreateGhostDto.tsgonest.js",
	}

	var diags []RewriteDiagnostic
	report := func(d RewriteDiagnostic) { diags = append(diags, d) }

	rewriteController(input, "/dist/ghost.controller.js", controllers, companionMap, "esm", report)

	var errors []RewriteDiagnostic
	for _, d := range diags {
		if d.IsError() {
			errors = append(errors, d)
		}
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error diagnostic, got %d (all diagnostics: %+v)", len(errors), diags)
	}
	if errors[0].Class != "GhostController" {
		t.Errorf("expected error for GhostController, got class=%q", errors[0].Class)
	}
	if !strings.Contains(errors[0].Reason, "controller class not found") {
		t.Errorf("expected reason to mention class-not-found, got: %s", errors[0].Reason)
	}
}

// TestRewriteController_ChainedAssignmentClassFound is the regression for #114
// itself: when a class is emitted as `let X = X_1 = class X { ... }`, the
// locator must find it and validation injection must succeed.
func TestRewriteController_ChainedAssignmentClassFound(t *testing.T) {
	input := `"use strict";
var FooController_1;
let FooController = FooController_1 = class FooController {
    logger = new Logger(FooController_1.name);
    async listThings(query) {
        return this.svc.list(query);
    }
};`

	controllers := []analyzer.ControllerInfo{
		{
			Name:       "FooController",
			SourceFile: "/src/foo.controller.ts",
			Routes: []analyzer.Route{
				{
					MethodName: "listThings",
					Parameters: []analyzer.RouteParameter{
						{
							Category:  "query",
							TypeName:  "ListThingsQueryDTO",
							LocalName: "query",
							Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "ListThingsQueryDTO"},
						},
					},
				},
			},
		},
	}
	companionMap := map[string]string{
		"ListThingsQueryDTO": "/dist/foo.dto.ListThingsQueryDTO.tsgonest.js",
	}

	var diags []RewriteDiagnostic
	result := rewriteController(input, "/dist/foo.controller.js", controllers, companionMap, "esm", func(d RewriteDiagnostic) {
		diags = append(diags, d)
	})

	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error diagnostic: %s", d.Format())
		}
	}
	if !strings.Contains(result, "assertListThingsQueryDTO(query)") {
		t.Errorf("expected validation injection inside chained-assignment class, got:\n%s", result)
	}
}

// TestRewriteController_WarnWhenMethodNotLocated covers the recoverable
// degradation case: the class is found but the route's method isn't (e.g.
// emitted as a getter, computed name, or some other shape we don't recognize).
// Other methods keep working; a warning surfaces so operators see it.
func TestRewriteController_WarnWhenMethodNotLocated(t *testing.T) {
	// Class is present and one method is found, but the route refers to a
	// method name that doesn't exist in the emitted JS.
	input := `"use strict";
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
					MethodName: "ghostMethod", // not in the JS
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

	var diags []RewriteDiagnostic
	rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", func(d RewriteDiagnostic) {
		diags = append(diags, d)
	})

	var errors, warnings int
	for _, d := range diags {
		if d.IsError() {
			errors++
		} else {
			warnings++
		}
	}
	if errors != 0 {
		t.Errorf("expected 0 errors, got %d (diags: %+v)", errors, diags)
	}
	if warnings != 1 {
		t.Fatalf("expected 1 warning, got %d (diags: %+v)", warnings, diags)
	}
	w := diags[0]
	if w.Class != "UserController" || w.Method != "ghostMethod" {
		t.Errorf("expected warning for UserController.ghostMethod, got class=%q method=%q", w.Class, w.Method)
	}
}

// TestRewriteController_NoMethodWarningsWhenClassMissing ensures that when a
// class itself is missing (an error), we don't drown the user in per-method
// warnings on top of the class-level error.
func TestRewriteController_NoMethodWarningsWhenClassMissing(t *testing.T) {
	input := `"use strict";
const noClassHere = true;
`

	controllers := []analyzer.ControllerInfo{
		{
			Name: "GhostController",
			Routes: []analyzer.Route{
				{
					MethodName: "a",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", LocalName: "body",
							Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Dto"}},
					},
				},
				{
					MethodName: "b",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", LocalName: "body",
							Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "Dto"}},
					},
				},
			},
		},
	}
	companionMap := map[string]string{
		"Dto": "/dist/dto.tsgonest.js",
	}

	var diags []RewriteDiagnostic
	rewriteController(input, "/dist/ghost.controller.js", controllers, companionMap, "esm", func(d RewriteDiagnostic) {
		diags = append(diags, d)
	})

	var errors, warnings int
	for _, d := range diags {
		if d.IsError() {
			errors++
		} else {
			warnings++
		}
	}
	if errors != 1 {
		t.Errorf("expected exactly 1 class-level error, got %d", errors)
	}
	if warnings != 0 {
		t.Errorf("expected no per-method warnings when class is missing, got %d (diags: %+v)", warnings, diags)
	}
}

// TestRewriteContext_DiagnosticThreadSafety ensures concurrent appends from
// the WriteFile callback don't race or corrupt the diagnostic slice.
func TestRewriteContext_DiagnosticThreadSafety(t *testing.T) {
	ctx := &RewriteContext{}
	const goroutines = 50
	const perGoroutine = 20

	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(gi int) {
			for j := 0; j < perGoroutine; j++ {
				ctx.addDiagnostic(RewriteDiagnostic{
					Severity:   DiagnosticWarning,
					OutputFile: "/dist/x.js",
					Class:      "C",
					Method:     "m",
					Reason:     "r",
				})
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	got := ctx.Diagnostics()
	want := goroutines * perGoroutine
	if len(got) != want {
		t.Fatalf("expected %d diagnostics, got %d", want, len(got))
	}
	if ctx.HasErrors() {
		t.Error("expected no errors among warnings")
	}
}
