package rewrite

import (
	"strings"
	"testing"
)

func TestRewriteMarkers_ESM(t *testing.T) {
	input := `import { is, assert } from "tsgonest";
const ok = is(body);
const val = assert(body);`

	calls := []MarkerCall{
		{FunctionName: "is", TypeName: "CreateUserDto", SourcePos: 0},
		{FunctionName: "assert", TypeName: "CreateUserDto", SourcePos: 1},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
	}

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "esm")

	if !strings.Contains(result, rewriteSentinel) {
		t.Error("expected sentinel comment")
	}
	if !strings.Contains(result, `import { isCreateUserDto, assertCreateUserDto } from "./user.dto.CreateUserDto.tsgonest.js"`) {
		t.Errorf("expected companion import, got:\n%s", result)
	}
	if !strings.Contains(result, "isCreateUserDto(body)") {
		t.Errorf("expected isCreateUserDto call, got:\n%s", result)
	}
	if !strings.Contains(result, "assertCreateUserDto(body)") {
		t.Errorf("expected assertCreateUserDto call, got:\n%s", result)
	}
	if strings.Contains(result, `from "tsgonest"`) {
		t.Error("tsgonest import should have been removed")
	}
}

func TestRewriteMarkers_CJS(t *testing.T) {
	input := `const { is, assert } = require("tsgonest");
const ok = is(body);
const val = assert(body);`

	calls := []MarkerCall{
		{FunctionName: "is", TypeName: "CreateUserDto", SourcePos: 0},
		{FunctionName: "assert", TypeName: "CreateUserDto", SourcePos: 1},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
	}

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "cjs")

	if !strings.Contains(result, `const { isCreateUserDto, assertCreateUserDto } = require("./user.dto.CreateUserDto.tsgonest.js")`) {
		t.Errorf("expected CJS require, got:\n%s", result)
	}
	if strings.Contains(result, `require("tsgonest")`) {
		t.Error("tsgonest require should have been removed")
	}
}

func TestRewriteMarkers_Multiple(t *testing.T) {
	input := `import { validate, stringify } from "tsgonest";
const v1 = validate(body1);
const v2 = validate(body2);
const s = stringify(user);`

	calls := []MarkerCall{
		{FunctionName: "validate", TypeName: "CreateUserDto", SourcePos: 0},
		{FunctionName: "validate", TypeName: "UpdateUserDto", SourcePos: 1},
		{FunctionName: "stringify", TypeName: "UserResponse", SourcePos: 2},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
		"UpdateUserDto": "/dist/user.dto.UpdateUserDto.tsgonest.js",
		"UserResponse":  "/dist/user.response.UserResponse.tsgonest.js",
	}

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "esm")

	if !strings.Contains(result, "validateCreateUserDto(body1)") {
		t.Errorf("expected validateCreateUserDto, got:\n%s", result)
	}
	if !strings.Contains(result, "validateUpdateUserDto(body2)") {
		t.Errorf("expected validateUpdateUserDto, got:\n%s", result)
	}
	if !strings.Contains(result, "stringifyUserResponse(user)") {
		t.Errorf("expected stringifyUserResponse, got:\n%s", result)
	}
}

func TestRewriteMarkers_AlreadyRewritten(t *testing.T) {
	input := rewriteSentinel + "\n" + `import { isCreateUserDto } from "./user.dto.CreateUserDto.tsgonest.js";
const ok = isCreateUserDto(body);`

	calls := []MarkerCall{
		{FunctionName: "is", TypeName: "CreateUserDto", SourcePos: 0},
	}

	companionMap := map[string]string{
		"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
	}

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "esm")

	// Should be unchanged
	if result != input {
		t.Errorf("already rewritten file should not be modified:\n%s", result)
	}
}

func TestRewriteMarkers_NoCalls(t *testing.T) {
	input := `console.log("hello");`
	result := rewriteMarkers(input, "/dist/test.js", nil, nil, "esm")
	if result != input {
		t.Error("no calls should mean no changes")
	}
}

func TestIsTsgonestImportLine_ESM(t *testing.T) {
	tests := []struct {
		line   string
		expect bool
	}{
		{`import { is, validate } from "tsgonest";`, true},
		{`import { is } from 'tsgonest';`, true},
		{`import { foo } from "other-pkg";`, false},
		{`import { is } from "tsgonest-extra";`, false},
		{`  import { assert } from "tsgonest";`, true},
	}

	for _, tt := range tests {
		got := isTsgonestImportLine(tt.line)
		if got != tt.expect {
			t.Errorf("isTsgonestImportLine(%q) = %v, want %v", tt.line, got, tt.expect)
		}
	}
}

func TestIsTsgonestImportLine_CJS(t *testing.T) {
	tests := []struct {
		line   string
		expect bool
	}{
		{`const { is } = require("tsgonest");`, true},
		{`const { is } = require('tsgonest');`, true},
		{`const foo = require("other");`, false},
		{`let { is } = require("tsgonest");`, true},
		{`var { is } = require("tsgonest");`, true},
	}

	for _, tt := range tests {
		got := isTsgonestImportLine(tt.line)
		if got != tt.expect {
			t.Errorf("isTsgonestImportLine(%q) = %v, want %v", tt.line, got, tt.expect)
		}
	}
}
