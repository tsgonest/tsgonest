package rewrite

import (
	"testing"
)

func TestCompanionFuncName(t *testing.T) {
	tests := []struct {
		marker, typeName, expected string
	}{
		{"is", "CreateUserDto", "isCreateUserDto"},
		{"validate", "CreateUserDto", "validateCreateUserDto"},
		{"assert", "CreateUserDto", "assertCreateUserDto"},
		{"stringify", "UserResponse", "stringifyUserResponse"},
		{"serialize", "UserResponse", "serializeUserResponse"},
	}

	for _, tt := range tests {
		got := companionFuncName(tt.marker, tt.typeName)
		if got != tt.expected {
			t.Errorf("companionFuncName(%q, %q) = %q, want %q", tt.marker, tt.typeName, got, tt.expected)
		}
	}
}

func TestCompanionRelativePath(t *testing.T) {
	tests := []struct {
		fromFile, companionPath, expected string
	}{
		{
			"/dist/controllers/user.controller.js",
			"/dist/dto/user.dto.CreateUserDto.tsgonest.js",
			"../dto/user.dto.CreateUserDto.tsgonest.js",
		},
		{
			"/dist/user.controller.js",
			"/dist/user.dto.CreateUserDto.tsgonest.js",
			"./user.dto.CreateUserDto.tsgonest.js",
		},
		{
			"/dist/a/b/c.js",
			"/dist/a/b/d.tsgonest.js",
			"./d.tsgonest.js",
		},
	}

	for _, tt := range tests {
		got := companionRelativePath(tt.fromFile, tt.companionPath)
		if got != tt.expected {
			t.Errorf("companionRelativePath(%q, %q) = %q, want %q", tt.fromFile, tt.companionPath, got, tt.expected)
		}
	}
}

func TestGenerateESMImport(t *testing.T) {
	got := generateESMImport([]string{"isCreateUserDto", "assertCreateUserDto"}, "./user.dto.CreateUserDto.tsgonest.js")
	expected := `import { isCreateUserDto, assertCreateUserDto } from "./user.dto.CreateUserDto.tsgonest.js";`
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestGenerateCJSRequire(t *testing.T) {
	got := generateCJSRequire([]string{"isCreateUserDto"}, "./user.dto.CreateUserDto.tsgonest.js")
	expected := `const { isCreateUserDto } = require("./user.dto.CreateUserDto.tsgonest.js");`
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestCompanionImports_Dedup(t *testing.T) {
	calls := []MarkerCall{
		{FunctionName: "is", TypeName: "CreateUserDto"},
		{FunctionName: "assert", TypeName: "CreateUserDto"},
		{FunctionName: "is", TypeName: "CreateUserDto"}, // duplicate
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
	}

	imports := companionImports(calls, companionMap, "/dist/controller.js", "esm")

	if len(imports) != 1 {
		t.Fatalf("expected 1 import line, got %d", len(imports))
	}

	// Should have both isCreateUserDto and assertCreateUserDto, but not duplicated isCreateUserDto
	expected := `import { isCreateUserDto, assertCreateUserDto } from "./user.dto.CreateUserDto.tsgonest.js";`
	if imports[0] != expected {
		t.Errorf("got %q, want %q", imports[0], expected)
	}
}
