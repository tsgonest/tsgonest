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

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "esm", nil)

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

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "cjs", nil)

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

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "esm", nil)

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

	result := rewriteMarkers(input, "/dist/user.controller.js", calls, companionMap, "esm", nil)

	// Should be unchanged
	if result != input {
		t.Errorf("already rewritten file should not be modified:\n%s", result)
	}
}

func TestRewriteMarkers_NoCalls(t *testing.T) {
	input := `console.log("hello");`
	result := rewriteMarkers(input, "/dist/test.js", nil, nil, "esm", nil)
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

func TestRewriteMarkers_CJSInterop(t *testing.T) {
	input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const tsgonest_1 = require("tsgonest");
function toThing(value) {
    return (0, tsgonest_1.assert)(value);
}
const ok = tsgonest_1.is(body);`

	calls := []MarkerCall{
		{FunctionName: "assert", TypeName: "SomeInterface", SourcePos: 0},
		{FunctionName: "is", TypeName: "SomeInterface", SourcePos: 1},
	}

	companionMap := map[string]string{
		"SomeInterface": "/dist/file.SomeInterface.tsgonest.js",
	}

	result := rewriteMarkers(input, "/dist/file.js", calls, companionMap, "cjs", nil)

	if !strings.Contains(result, "return assertSomeInterface(value);") {
		t.Errorf("interop call (0, tsgonest_1.assert)(x) should rewrite to companion call, got:\n%s", result)
	}
	if !strings.Contains(result, "const ok = isSomeInterface(body);") {
		t.Errorf("member call tsgonest_1.is(x) should rewrite to companion call, got:\n%s", result)
	}
	if strings.Contains(result, "tsgonest_1") {
		t.Errorf("no tsgonest_1 references may survive (ReferenceError at runtime), got:\n%s", result)
	}
}

func TestRewriteMarkers_CJSInterop_OccurrenceOrder(t *testing.T) {
	input := `const tsgonest_1 = require("tsgonest");
const a = (0, tsgonest_1.assert)(x);
const b = (0, tsgonest_1.assert)(y);`

	calls := []MarkerCall{
		{FunctionName: "assert", TypeName: "First", SourcePos: 0},
		{FunctionName: "assert", TypeName: "Second", SourcePos: 1},
	}

	companionMap := map[string]string{
		"First":  "/dist/a.First.tsgonest.js",
		"Second": "/dist/b.Second.tsgonest.js",
	}

	result := rewriteMarkers(input, "/dist/file.js", calls, companionMap, "cjs", nil)

	if !strings.Contains(result, "const a = assertFirst(x);") || !strings.Contains(result, "const b = assertSecond(y);") {
		t.Errorf("interop calls must map to companions in occurrence order, got:\n%s", result)
	}
}

func TestRewriteMarkers_MissingCompanion_KeepsImportAndCall(t *testing.T) {
	input := `const tsgonest_1 = require("tsgonest");
const a = (0, tsgonest_1.assert)(x);
const b = (0, tsgonest_1.assert)(y);`

	calls := []MarkerCall{
		{FunctionName: "assert", TypeName: "Known", SourcePos: 0},
		{FunctionName: "assert", TypeName: "Unknown", SourcePos: 1},
	}

	companionMap := map[string]string{
		"Known": "/dist/a.Known.tsgonest.js",
	}

	var warnings []string
	result := rewriteMarkers(input, "/dist/file.js", calls, companionMap, "cjs", func(r string) {
		warnings = append(warnings, r)
	})

	if !strings.Contains(result, "const a = assertKnown(x);") {
		t.Errorf("call with companion should still be rewritten, got:\n%s", result)
	}
	if !strings.Contains(result, "const b = (0, tsgonest_1.assert)(y);") {
		t.Errorf("call without companion must be left untouched, got:\n%s", result)
	}
	if !strings.Contains(result, `require("tsgonest")`) {
		t.Errorf("tsgonest import must be kept when any call is unrewritten, got:\n%s", result)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Unknown") {
		t.Errorf("expected one warning naming the missing type, got: %v", warnings)
	}
}
