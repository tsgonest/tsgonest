package rewrite

import (
	"context"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/shim/ast"
	"github.com/microsoft/typescript-go/shim/bundled"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/core"
	"github.com/microsoft/typescript-go/shim/tsoptions"
	"github.com/microsoft/typescript-go/shim/tspath"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/testutil"
)

// rewriteIntegrationEnv mirrors analyzer_test.walkerEnv but lives in package rewrite
// so the integration tests below can drive both layers (analyzer + rewriter) from
// the same compiled tsgo program.
type rewriteIntegrationEnv struct {
	program    *shimcompiler.Program
	checker    *shimchecker.Checker
	sourceFile *ast.SourceFile
	release    func()
}

func setupRewriteIntegration(t *testing.T, tsSource string) *rewriteIntegrationEnv {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	rootDir := path.Join(path.Dir(filename), "..", "..", "testdata", "walker")
	fileName := "test.ts"
	filePath := tspath.ResolvePath(rootDir, fileName)

	fs := testutil.NewDefaultOverlayVFS(map[string]string{filePath: tsSource})
	host := shimcompiler.NewCompilerHost(rootDir, fs, bundled.LibPath(), nil, nil)

	parsed, diags := tsoptions.GetParsedCommandLineOfConfigFile("tsconfig.json", &core.CompilerOptions{}, nil, host, nil)
	if len(diags) > 0 {
		t.Fatalf("tsconfig parse errors: %v", diags[0].String())
	}

	program := shimcompiler.NewProgram(shimcompiler.ProgramOptions{
		Config:                      parsed,
		SingleThreaded:              core.TSTrue,
		Host:                        host,
		UseSourceOfProjectReference: true,
	})
	if program == nil {
		t.Fatal("failed to create program")
	}
	program.BindSourceFiles()

	sf := program.GetSourceFile(fileName)
	if sf == nil {
		t.Fatalf("source file %q not found", fileName)
	}
	ck, rel := program.GetTypeChecker(context.Background())
	if ck == nil {
		t.Fatal("failed to get type checker")
	}
	return &rewriteIntegrationEnv{program: program, checker: ck, sourceFile: sf, release: rel}
}

// TestIntegration_MongoIdAliasOnNamedParam_EmitsRuntimeRegexCheck exercises the
// full pipeline reported in the issue: a JSDoc-annotated string alias used as
// the type of a named @Param() must flow through the analyzer (Gap B) and into
// the controller rewriter (Gap A) to produce an inline regex check in JS.
//
// The test deliberately asserts the analyzer-side outcome first, then the
// rewriter-side outcome. A failure in the first assertion pinpoints the analyzer
// layer; a failure only in the second pinpoints the rewriter layer.
func TestIntegration_MongoIdAliasOnNamedParam_EmitsRuntimeRegexCheck(t *testing.T) {
	env := setupRewriteIntegration(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Get(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Param(name: string): ParameterDecorator { return () => {}; }

		/** @pattern ^[0-9a-fA-F]{24}$ */
		type MongoId = string;

		@Controller("projects")
		class ProjectController {
			@Get(":projectID")
			findOne(@Param("projectID") projectID: MongoId): {} { return {}; }
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

	// Analyzer-side assertion (Gap B): the JSDoc @pattern on the MongoId alias
	// must surface as Constraints.Pattern on the route parameter's Type.
	if param.Type.Constraints == nil {
		t.Fatal("analyzer gap: expected Constraints on @Param('projectID') projectID: MongoId, got nil")
	}
	if param.Type.Constraints.Pattern == nil {
		t.Fatal("analyzer gap: expected Constraints.Pattern set from JSDoc @pattern, got nil")
	}
	if got := *param.Type.Constraints.Pattern; got != "^[0-9a-fA-F]{24}$" {
		t.Fatalf("analyzer gap: expected Pattern=%q, got %q", "^[0-9a-fA-F]{24}$", got)
	}

	// Rewriter-side assertion (Gap A): given the analyzer captured the pattern,
	// the controller rewriter must inline a runtime regex check for the named
	// scalar @Param. The synthetic JS mirrors what tsgo would emit for the
	// stripped controller body.
	syntheticJS := `class ProjectController {
    findOne(projectID) {
        return {};
    }
}`

	result := rewriteController(syntheticJS, "/dist/project.controller.js", controllers, map[string]string{}, "esm", nil)

	if !strings.Contains(result, `/^[0-9a-fA-F]{24}$/.test(projectID)`) {
		t.Errorf("rewriter gap: expected regex check `/^[0-9a-fA-F]{24}$/.test(projectID)`, got:\n%s", result)
	}
	if !strings.Contains(result, "throw new __e(") {
		t.Errorf("rewriter gap: expected `throw new __e(` to surround the regex check, got:\n%s", result)
	}
}

// TestIntegration_MongoIdAliasOnPropertyOfBody_EmitsCompanionConstraint proves
// that Gap B also reaches DTO properties: the JSDoc @pattern on the MongoId
// alias propagates onto a property of a CreateDto used as @Body(). This is the
// metadata the .tsgonest.js companion codegen consumes. The rewriter does NOT
// emit inline checks for @Body — it emits an `assertCreateDto(body)` call that
// defers to the companion file — so this test stops at the analyzer assertion.
func TestIntegration_MongoIdAliasOnPropertyOfBody_EmitsCompanionConstraint(t *testing.T) {
	env := setupRewriteIntegration(t, `
		function Controller(path: string): ClassDecorator { return (target) => target; }
		function Post(path?: string): MethodDecorator { return (t, k, d) => d; }
		function Body(): ParameterDecorator { return () => {}; }

		/** @pattern ^[0-9a-fA-F]{24}$ */
		type MongoId = string;

		interface CreateDto {
			projectID: MongoId;
		}

		@Controller("projects")
		class ProjectController {
			@Post()
			create(@Body() dto: CreateDto): {} { return {}; }
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
		t.Fatalf("expected Category='body', got %q", param.Category)
	}

	// Resolve KindRef to the registered object so we can inspect properties.
	bodyType := param.Type
	if bodyType.Kind == "ref" {
		reg := ca.Registry()
		if resolved, ok := reg.Types[bodyType.Ref]; ok && resolved != nil {
			bodyType = *resolved
		}
	}
	if len(bodyType.Properties) == 0 {
		t.Fatalf("expected CreateDto to have properties, got %d", len(bodyType.Properties))
	}

	var projectIDProp = -1
	for i := range bodyType.Properties {
		if bodyType.Properties[i].Name == "projectID" {
			projectIDProp = i
			break
		}
	}
	if projectIDProp < 0 {
		t.Fatal("expected property 'projectID' on CreateDto")
	}
	prop := bodyType.Properties[projectIDProp]

	// Gap B reaches into property types too: the JSDoc @pattern on MongoId
	// must propagate to the projectID property's Constraints.Pattern, which is
	// what companion codegen reads when emitting assertCreateDto.
	if prop.Constraints == nil {
		t.Fatal("analyzer gap: expected Constraints on projectID property, got nil")
	}
	if prop.Constraints.Pattern == nil {
		t.Fatal("analyzer gap: expected Constraints.Pattern on projectID property from JSDoc @pattern, got nil")
	}
	if got := *prop.Constraints.Pattern; got != "^[0-9a-fA-F]{24}$" {
		t.Fatalf("analyzer gap: expected Pattern=%q on projectID property, got %q", "^[0-9a-fA-F]{24}$", got)
	}
}
