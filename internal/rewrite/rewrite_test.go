package rewrite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
)

func TestWriteFileToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.js")

	err := writeFileToDisk(path, "console.log('hello');", false)
	if err != nil {
		t.Fatalf("writeFileToDisk failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "console.log('hello');" {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestWriteFileToDisk_BOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bom.js")

	err := writeFileToDisk(path, "test", true)
	if err != nil {
		t.Fatalf("writeFileToDisk failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if !strings.HasPrefix(string(content), "\xEF\xBB\xBF") {
		t.Error("expected BOM prefix")
	}
}

func TestBuildOutputToSourceMap(t *testing.T) {
	sourceToOutput := map[string]string{
		"/src/user.dto.ts": "/dist/user.dto.ts",
	}

	result := BuildOutputToSourceMap(sourceToOutput)

	if src, ok := result["/dist/user.dto.js"]; !ok || src != "/src/user.dto.ts" {
		t.Errorf("expected /dist/user.dto.js → /src/user.dto.ts, got: %v", result)
	}
}

func TestBuildCompanionMap(t *testing.T) {
	sourceToOutput := map[string]string{
		"/src/user.dto.ts": "/dist/user.dto.ts",
	}
	typesByFile := map[string][]string{
		"/src/user.dto.ts": {"CreateUserDto", "UpdateUserDto"},
	}

	result := BuildCompanionMap(sourceToOutput, typesByFile)

	if path, ok := result["CreateUserDto"]; !ok || path != "/dist/user.dto.CreateUserDto.tsgonest.js" {
		t.Errorf("CreateUserDto companion path: %v", result)
	}
	if path, ok := result["UpdateUserDto"]; !ok || path != "/dist/user.dto.UpdateUserDto.tsgonest.js" {
		t.Errorf("UpdateUserDto companion path: %v", result)
	}
}

func TestDetectModuleFormat(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"commonjs", "cjs"},
		{"cjs", "cjs"},
		{"CommonJS", "cjs"},
		{"esnext", "esm"},
		{"es2020", "esm"},
		{"", "esm"},
		{"nodenext", "esm"},
	}

	for _, tt := range tests {
		got := DetectModuleFormat(tt.input)
		if got != tt.expected {
			t.Errorf("DetectModuleFormat(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBuildControllerSourceFiles(t *testing.T) {
	controllers := []analyzer.ControllerInfo{
		{SourceFile: "/src/user.controller.ts"},
		{SourceFile: "/src/auth.controller.ts"},
	}

	result := BuildControllerSourceFiles(controllers)

	if !result["/src/user.controller.ts"] {
		t.Error("expected user.controller.ts in set")
	}
	if !result["/src/auth.controller.ts"] {
		t.Error("expected auth.controller.ts in set")
	}
	if result["/src/other.ts"] {
		t.Error("unexpected file in set")
	}
}

func TestWriteFileCallback_Integration(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "user.controller.js")

	ctx := &RewriteContext{
		CompanionMap: map[string]string{
			"CreateUserDto": filepath.Join(dir, "user.dto.CreateUserDto.tsgonest.js"),
		},
		MarkerCalls: map[string][]MarkerCall{
			"/src/user.controller.ts": {
				{FunctionName: "assert", TypeName: "CreateUserDto", SourcePos: 0},
			},
		},
		Controllers:           nil,
		ControllerSourceFiles: make(map[string]bool),
		ModuleFormat:          "esm",
		SourceToOutput: map[string]string{
			"/src/user.controller.ts": filepath.Join(dir, "user.controller.ts"),
		},
		OutputToSource: map[string]string{
			outputFile: "/src/user.controller.ts",
		},
	}

	writeFile := ctx.MakeWriteFile()

	input := `import { assert } from "tsgonest";
const user = assert(body);
console.log(user);`

	err := writeFile(outputFile, input, false, nil)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	result := string(content)
	if !strings.Contains(result, "assertCreateUserDto(body)") {
		t.Errorf("expected assertCreateUserDto call, got:\n%s", result)
	}
	if strings.Contains(result, `from "tsgonest"`) {
		t.Errorf("tsgonest import should be removed, got:\n%s", result)
	}
}
