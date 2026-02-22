package pathalias

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Common NestJS-like tsconfig:
//   paths: { "@app/*": ["src/*"] }
//   rootDir: "./src"
//   outDir: "./dist"
//
// Path targets are resolved relative to pathsBaseDir (tsconfig directory).
// @app/services/user → /project/src/services/user (source)
//                     → /project/dist/services/user (output, after rootDir→outDir mapping)

func makeResolver(pathsBaseDir, outDir, rootDir string, paths map[string][]string) *PathResolver {
	return NewPathResolver(Config{
		PathsBaseDir: pathsBaseDir,
		OutDir:       outDir,
		RootDir:      rootDir,
		Paths:        paths,
	})
}

func TestResolveImports_WildcardAlias(t *testing.T) {
	resolver := makeResolver(
		"/project",      // pathsBaseDir (project root)
		"/project/dist", // outDir
		"/project/src",  // rootDir
		map[string][]string{
			"@app/*": {"src/*"},
		},
	)

	input := `const foo = require("@app/services/user");`
	result := resolver.ResolveImports(input, "/project/dist/controllers/user.controller.js")

	if !strings.Contains(result, "../services/user") {
		t.Errorf("expected relative import containing ../services/user, got: %s", result)
	}
	if strings.Contains(result, "@app/") {
		t.Errorf("alias should have been resolved, got: %s", result)
	}
}

func TestResolveImports_ESMImport(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	input := `import { UserService } from "@app/services/user";`
	result := resolver.ResolveImports(input, "/project/dist/controllers/user.controller.js")

	if !strings.Contains(result, "../services/user") {
		t.Errorf("expected relative import containing ../services/user, got: %s", result)
	}
}

func TestResolveImports_NoChange(t *testing.T) {
	resolver := makeResolver("", "", "", map[string][]string{"@app/*": {"src/*"}})

	input := `import { foo } from "./local";`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if result != input {
		t.Errorf("expected no change for relative import, got: %s", result)
	}
}

func TestResolveImports_ExactAlias(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@config": {"src/config"}},
	)

	input := `const cfg = require("@config");`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if !strings.Contains(result, "./config") {
		t.Errorf("expected relative import to config, got: %s", result)
	}
}

func TestResolveImports_MultipleAliases(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{
			"@app/*": {"src/*"},
			"@lib/*": {"src/lib/*"},
		},
	)

	input := "import { A } from \"@app/a\";\nimport { B } from \"@lib/b\";"
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if strings.Contains(result, "@app/") {
		t.Errorf("@app alias not resolved: %s", result)
	}
	if strings.Contains(result, "@lib/") {
		t.Errorf("@lib alias not resolved: %s", result)
	}
}

func TestResolveImports_NodeModules(t *testing.T) {
	resolver := makeResolver("", "", "", map[string][]string{"@app/*": {"src/*"}})

	input := `import express from "express";`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if result != input {
		t.Errorf("expected no change for node_modules import, got: %s", result)
	}
}

func TestResolveImports_EmptyAliases(t *testing.T) {
	resolver := makeResolver("", "", "", nil)

	input := `import { foo } from "@app/bar";`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if result != input {
		t.Errorf("expected no change with no aliases, got: %s", result)
	}
}

func TestResolveImports_AbsolutePathSpecifier(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	input := `import { foo } from "/absolute/path";`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if result != input {
		t.Errorf("expected no change for absolute path import, got: %s", result)
	}
}

func TestResolveImports_SingleQuotes(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	input := `const foo = require('@app/services/user');`
	result := resolver.ResolveImports(input, "/project/dist/controllers/user.controller.js")

	if !strings.Contains(result, "../services/user") {
		t.Errorf("expected relative import for single-quoted require, got: %s", result)
	}
}

func TestResolveImports_DeepNestedImport(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	input := `import { Foo } from "@app/modules/auth/guards/jwt.guard";`
	result := resolver.ResolveImports(input, "/project/dist/modules/users/controllers/users.controller.js")

	if strings.Contains(result, "@app/") {
		t.Errorf("alias should have been resolved, got: %s", result)
	}
	if !strings.Contains(result, "../../auth/guards/jwt.guard") {
		t.Errorf("expected ../../auth/guards/jwt.guard, got: %s", result)
	}
}

func TestResolveImports_SameDirectory(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	input := `import { Foo } from "@app/utils/helper";`
	result := resolver.ResolveImports(input, "/project/dist/utils/index.js")

	if !strings.Contains(result, "./helper") {
		t.Errorf("expected ./helper for same-directory import, got: %s", result)
	}
}

func TestResolveImports_DynamicImport(t *testing.T) {
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	input := `const mod = require("@app/dynamic/loader");`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	if strings.Contains(result, "@app/") {
		t.Errorf("alias should have been resolved, got: %s", result)
	}
	if !strings.Contains(result, "./dynamic/loader") {
		t.Errorf("expected ./dynamic/loader, got: %s", result)
	}
}

func TestResolveImports_LongestPrefixWins(t *testing.T) {
	// esbuild behavior: longest prefix match wins
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{
			"@app/*":      {"src/*"},
			"@app/auth/*": {"src/auth/*"},
		},
	)

	input := `import { Guard } from "@app/auth/jwt.guard";`
	result := resolver.ResolveImports(input, "/project/dist/index.js")

	// @app/auth/* should win (longer prefix) and resolve to src/auth/jwt.guard
	if !strings.Contains(result, "./auth/jwt.guard") {
		t.Errorf("expected longest-prefix match to win, got: %s", result)
	}
}

func TestHasAliases(t *testing.T) {
	r1 := makeResolver("", "", "", nil)
	if r1.HasAliases() {
		t.Error("expected no aliases")
	}

	r2 := makeResolver("", "", "", map[string][]string{"@app/*": {"src/*"}})
	if !r2.HasAliases() {
		t.Error("expected aliases")
	}
}

func TestResolveImportsInFile(t *testing.T) {
	dir := t.TempDir()

	resolver := makeResolver(
		dir,
		filepath.Join(dir, "dist"),
		filepath.Join(dir, "src"),
		map[string][]string{"@app/*": {"src/*"}},
	)

	distCtrl := filepath.Join(dir, "dist", "controllers")
	if err := os.MkdirAll(distCtrl, 0755); err != nil {
		t.Fatal(err)
	}

	jsContent := `"use strict";
const user_service_1 = require("@app/services/user");
module.exports = { UserController: class {} };
`
	jsPath := filepath.Join(distCtrl, "user.controller.js")
	if err := os.WriteFile(jsPath, []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := resolver.ResolveImportsInFile(jsPath)
	if err != nil {
		t.Fatalf("ResolveImportsInFile error: %v", err)
	}

	result, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if strings.Contains(string(result), "@app/") {
		t.Errorf("alias should have been resolved in file, got: %s", string(result))
	}
	if !strings.Contains(string(result), "../services/user") {
		t.Errorf("expected ../services/user, got: %s", string(result))
	}
}

func TestResolveImportsInFile_NoChanges(t *testing.T) {
	dir := t.TempDir()

	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{"@app/*": {"src/*"}},
	)

	jsContent := `"use strict";
const x = require("./local");
`
	jsPath := filepath.Join(dir, "index.js")
	if err := os.WriteFile(jsPath, []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := resolver.ResolveImportsInFile(jsPath)
	if err != nil {
		t.Fatalf("ResolveImportsInFile error: %v", err)
	}

	result, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != jsContent {
		t.Errorf("file should not have been modified, got: %s", string(result))
	}
}

func TestResolveAllEmittedFiles(t *testing.T) {
	dir := t.TempDir()

	resolver := makeResolver(
		dir,
		filepath.Join(dir, "dist"),
		filepath.Join(dir, "src"),
		map[string][]string{"@app/*": {"src/*"}},
	)

	distDir := filepath.Join(dir, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatal(err)
	}

	jsFile1 := filepath.Join(distDir, "index.js")
	jsFile2 := filepath.Join(distDir, "helper.js")
	dtsFile := filepath.Join(distDir, "index.d.ts")

	if err := os.WriteFile(jsFile1, []byte(`const x = require("./local");`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsFile2, []byte(`const y = 42;`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dtsFile, []byte(`export declare const x: any;`), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := resolver.ResolveAllEmittedFiles([]string{jsFile1, jsFile2, dtsFile})
	if err != nil {
		t.Fatalf("ResolveAllEmittedFiles error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 JS files processed, got %d", count)
	}
}

func TestResolveAllEmittedFiles_NoAliases(t *testing.T) {
	resolver := makeResolver("", "", "", nil)

	count, err := resolver.ResolveAllEmittedFiles([]string{"/project/dist/index.js"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files processed with no aliases, got %d", count)
	}
}

func TestSourceToOutput(t *testing.T) {
	tests := []struct {
		name     string
		resolver *PathResolver
		srcPath  string
		want     string
	}{
		{
			name:     "rootDir and outDir",
			resolver: makeResolver("/project", "/project/dist", "/project/src", nil),
			srcPath:  "/project/src/services/user.ts",
			want:     "/project/dist/services/user.ts",
		},
		{
			name:     "outDir only",
			resolver: makeResolver("", "/project/dist", "", nil),
			srcPath:  "/project/src/services/user.ts",
			want:     "/project/dist/user.ts",
		},
		{
			name:     "no outDir",
			resolver: makeResolver("", "", "", nil),
			srcPath:  "/project/src/services/user.ts",
			want:     "/project/src/services/user.ts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resolver.sourceToOutput(tt.srcPath)
			if got != tt.want {
				t.Errorf("sourceToOutput(%s) = %s, want %s", tt.srcPath, got, tt.want)
			}
		})
	}
}

func TestInferRootDir(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{
			name:  "all under src",
			files: []string{"/project/src/main.ts", "/project/src/app.module.ts", "/project/src/auth/auth.ts"},
			want:  "/project/src",
		},
		{
			name:  "different roots",
			files: []string{"/project/src/main.ts", "/project/test/app.spec.ts"},
			want:  "/project",
		},
		{
			name:  "empty",
			files: nil,
			want:  "",
		},
		{
			name:  "single file",
			files: []string{"/project/src/main.ts"},
			want:  "/project/src",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferRootDir(tt.files)
			if got != tt.want {
				t.Errorf("InferRootDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveImports_ExactMatchBeforeWildcard(t *testing.T) {
	// esbuild behavior: exact match takes precedence
	resolver := makeResolver(
		"/project", "/project/dist", "/project/src",
		map[string][]string{
			"@db/client":   {"src/infrastructure/db/client"},
			"@db/client/*": {"src/infrastructure/db/client/*"},
		},
	)

	input := `const db = require("@db/client");`
	result := resolver.ResolveImports(input, "/project/dist/app.module.js")

	if strings.Contains(result, "@db/client") {
		t.Errorf("alias should have been resolved, got: %s", result)
	}
	if !strings.Contains(result, "./infrastructure/db/client") {
		t.Errorf("expected ./infrastructure/db/client, got: %s", result)
	}
}
