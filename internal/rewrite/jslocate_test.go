package rewrite

import (
	"testing"
)

func TestLocateJS_BasicClass(t *testing.T) {
	input := `class UserController {
    async create(body) {
        return this.service.create(body);
    }
}`
	locs := LocateJS(input)

	cls, ok := locs.Classes["UserController"]
	if !ok {
		t.Fatal("expected to find class UserController")
	}

	m, ok := cls.Methods["create"]
	if !ok {
		t.Fatal("expected to find method create")
	}

	if !m.IsAsync {
		t.Error("expected method to be async")
	}

	if len(m.Parameters) != 1 || m.Parameters[0] != "body" {
		t.Errorf("expected parameters [body], got %v", m.Parameters)
	}

	if input[m.BodyOpenBrace] != '{' {
		t.Errorf("expected '{' at BodyOpenBrace, got %q", string(input[m.BodyOpenBrace]))
	}
	if input[m.BodyCloseBrace] != '}' {
		t.Errorf("expected '}' at BodyCloseBrace, got %q", string(input[m.BodyCloseBrace]))
	}

	if len(m.Returns) != 1 {
		t.Fatalf("expected 1 return, got %d", len(m.Returns))
	}
	ret := m.Returns[0]
	if ret.ExprStart < 0 {
		t.Error("expected non-bare return")
	}
	expr := input[ret.ExprStart:ret.ExprEnd]
	if expr != "this.service.create(body)" {
		t.Errorf("expected return expression 'this.service.create(body)', got %q", expr)
	}
	if !ret.HasSemicolon {
		t.Error("expected semicolon-terminated return")
	}
}

func TestLocateJS_NonAsyncMethod(t *testing.T) {
	input := `class Foo {
    bar(x) {
        return x;
    }
}`
	locs := LocateJS(input)

	cls := locs.Classes["Foo"]
	if cls == nil {
		t.Fatal("expected class Foo")
	}
	m := cls.Methods["bar"]
	if m == nil {
		t.Fatal("expected method bar")
	}
	if m.IsAsync {
		t.Error("expected method to not be async")
	}
	if len(m.Parameters) != 1 || m.Parameters[0] != "x" {
		t.Errorf("expected parameters [x], got %v", m.Parameters)
	}
}

func TestLocateJS_MultipleReturns(t *testing.T) {
	input := `class Foo {
    bar(x) {
        if (x) {
            return x;
        }
        return null;
    }
}`
	locs := LocateJS(input)
	m := locs.Classes["Foo"].Methods["bar"]
	if len(m.Returns) != 2 {
		t.Fatalf("expected 2 returns, got %d", len(m.Returns))
	}

	expr0 := input[m.Returns[0].ExprStart:m.Returns[0].ExprEnd]
	if expr0 != "x" {
		t.Errorf("expected first return expr 'x', got %q", expr0)
	}
	expr1 := input[m.Returns[1].ExprStart:m.Returns[1].ExprEnd]
	if expr1 != "null" {
		t.Errorf("expected second return expr 'null', got %q", expr1)
	}
}

func TestLocateJS_BareReturn(t *testing.T) {
	input := `class Foo {
    bar() {
        return;
    }
}`
	locs := LocateJS(input)
	m := locs.Classes["Foo"].Methods["bar"]
	if len(m.Returns) != 1 {
		t.Fatalf("expected 1 return, got %d", len(m.Returns))
	}
	if m.Returns[0].ExprStart != -1 {
		t.Error("expected bare return (ExprStart = -1)")
	}
}

func TestLocateJS_NestedFunctionReturnsExcluded(t *testing.T) {
	input := `class Foo {
    bar() {
        const fn = function() { return 42; };
        return fn();
    }
}`
	locs := LocateJS(input)
	m := locs.Classes["Foo"].Methods["bar"]
	// Should only have the outer return, not the nested function's return
	if len(m.Returns) != 1 {
		t.Fatalf("expected 1 return (nested excluded), got %d", len(m.Returns))
	}
	expr := input[m.Returns[0].ExprStart:m.Returns[0].ExprEnd]
	if expr != "fn()" {
		t.Errorf("expected 'fn()', got %q", expr)
	}
}

func TestLocateJS_ArrowFunctionReturnsExcluded(t *testing.T) {
	input := `class Foo {
    bar() {
        const fn = () => { return 42; };
        return fn();
    }
}`
	locs := LocateJS(input)
	m := locs.Classes["Foo"].Methods["bar"]
	if len(m.Returns) != 1 {
		t.Fatalf("expected 1 return (arrow excluded), got %d", len(m.Returns))
	}
}

func TestLocateJS_MultipleMethods(t *testing.T) {
	input := `class Foo {
    bar(x) {
        return x;
    }
    baz() {
        return 1;
    }
}`
	locs := LocateJS(input)
	cls := locs.Classes["Foo"]
	if len(cls.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(cls.Methods))
	}
	if _, ok := cls.Methods["bar"]; !ok {
		t.Error("expected method bar")
	}
	if _, ok := cls.Methods["baz"]; !ok {
		t.Error("expected method baz")
	}
}

func TestLocateJS_MultipleClasses(t *testing.T) {
	input := `class A {
    foo() { return 1; }
}
class B {
    bar() { return 2; }
}`
	locs := LocateJS(input)
	if len(locs.Classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(locs.Classes))
	}
	if locs.Classes["A"] == nil {
		t.Error("expected class A")
	}
	if locs.Classes["B"] == nil {
		t.Error("expected class B")
	}
}

func TestLocateJS_MethodLevelDecorate(t *testing.T) {
	input := `__decorate([
    (0, common_1.Get)("events")
], UserController.prototype, "findAll", null);`

	locs := LocateJS(input)
	if len(locs.DecorateCalls) != 1 {
		t.Fatalf("expected 1 decorate call, got %d", len(locs.DecorateCalls))
	}
	dc := locs.DecorateCalls[0]
	if dc.IsClassLevel {
		t.Error("expected method-level decorate")
	}
	if dc.ClassName != "UserController" {
		t.Errorf("expected className UserController, got %q", dc.ClassName)
	}
	if dc.MethodName != "findAll" {
		t.Errorf("expected methodName findAll, got %q", dc.MethodName)
	}
	if input[dc.ArrayOpenBracket] != '[' {
		t.Errorf("expected '[' at ArrayOpenBracket, got %q", string(input[dc.ArrayOpenBracket]))
	}
}

func TestLocateJS_ClassLevelDecorate(t *testing.T) {
	input := `UserController = __decorate([
    (0, common_1.Controller)("users")
], UserController);`

	locs := LocateJS(input)
	if len(locs.DecorateCalls) != 1 {
		t.Fatalf("expected 1 decorate call, got %d", len(locs.DecorateCalls))
	}
	dc := locs.DecorateCalls[0]
	if !dc.IsClassLevel {
		t.Error("expected class-level decorate")
	}
	if dc.ClassName != "UserController" {
		t.Errorf("expected className UserController, got %q", dc.ClassName)
	}
	if dc.MethodName != "" {
		t.Errorf("expected empty methodName for class-level, got %q", dc.MethodName)
	}
	if input[dc.ArrayOpenBracket] != '[' {
		t.Errorf("expected '[' at ArrayOpenBracket, got %q", string(input[dc.ArrayOpenBracket]))
	}
}

func TestLocateJS_MixedDecorateAndClass(t *testing.T) {
	input := `var common_1 = require("@nestjs/common");
UserController = __decorate([
    (0, common_1.Controller)("users")
], UserController);
__decorate([
    (0, common_1.Get)("events")
], UserController.prototype, "findAll", null);
class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`
	locs := LocateJS(input)

	// Should find both class-level and method-level decorate calls
	if len(locs.DecorateCalls) != 2 {
		t.Fatalf("expected 2 decorate calls, got %d", len(locs.DecorateCalls))
	}

	// Should find the class
	cls := locs.Classes["UserController"]
	if cls == nil {
		t.Fatal("expected class UserController")
	}
	m := cls.Methods["findAll"]
	if m == nil {
		t.Fatal("expected method findAll")
	}
}

func TestLocateJS_TemplateLiteralInReturn(t *testing.T) {
	input := "class Foo {\n    bar(x) {\n        return `hello ${x} world`;\n    }\n}"
	locs := LocateJS(input)
	m := locs.Classes["Foo"].Methods["bar"]
	if len(m.Returns) != 1 {
		t.Fatalf("expected 1 return, got %d", len(m.Returns))
	}
	expr := input[m.Returns[0].ExprStart:m.Returns[0].ExprEnd]
	if expr != "`hello ${x} world`" {
		t.Errorf("expected template literal expression, got %q", expr)
	}
}

func TestLocateJS_GeneratorMethod(t *testing.T) {
	// async generator methods (async *methodName) should be detected
	input := `class Foo {
    async *streamEvents() {
        yield { event: "created", data: {} };
        return;
    }
}`
	locs := LocateJS(input)
	cls := locs.Classes["Foo"]
	if cls == nil {
		t.Fatal("expected class Foo")
	}
	m := cls.Methods["streamEvents"]
	if m == nil {
		t.Fatal("expected method streamEvents")
	}
	if !m.IsAsync {
		t.Error("expected method to be async")
	}
}

func TestLocateJS_ClassLevelDecorate_ChainedAssignment(t *testing.T) {
	// CJS tsgo output uses: exports.X = X = __decorate([...], X);
	input := `exports.AuthPublicController = AuthPublicController = __decorate([
    (0, common_1.Controller)('auth/public'),
    __metadata("design:paramtypes", [auth_service_1.AuthService])
], AuthPublicController);`

	locs := LocateJS(input)
	if len(locs.DecorateCalls) != 1 {
		t.Fatalf("expected 1 decorate call, got %d", len(locs.DecorateCalls))
	}
	dc := locs.DecorateCalls[0]
	if !dc.IsClassLevel {
		t.Error("expected class-level decorate")
	}
	if dc.ClassName != "AuthPublicController" {
		t.Errorf("expected className AuthPublicController, got %q", dc.ClassName)
	}
	if input[dc.ArrayOpenBracket] != '[' {
		t.Errorf("expected '[' at ArrayOpenBracket, got %q", string(input[dc.ArrayOpenBracket]))
	}
}

func TestLocateJS_SelfReferencingClass(t *testing.T) {
	// tsgo emits `let X = X_1 = class X { ... }` when the class body references
	// the class itself (e.g. `new Logger(X.name)` instance field initializer).
	// The locator must walk through the chained assignment to find the inner
	// ClassExpression — see issue #114.
	input := `var FooController_1;
let FooController = FooController_1 = class FooController {
    logger = new Logger(FooController_1.name);
    async listThings(companyId, query) {
        return this.svc.list(companyId, query);
    }
};`
	locs := LocateJS(input)
	cls, ok := locs.Classes["FooController"]
	if !ok {
		t.Fatal("expected to find class FooController via chained-assignment initializer")
	}
	if _, ok := cls.Methods["listThings"]; !ok {
		t.Fatal("expected to find method listThings on chained-assigned class")
	}
}

func TestLocateJS_MultipleParams(t *testing.T) {
	input := `class Foo {
    bar(a, b, c) {
        return a + b + c;
    }
}`
	locs := LocateJS(input)
	m := locs.Classes["Foo"].Methods["bar"]
	if len(m.Parameters) != 3 {
		t.Fatalf("expected 3 params, got %d", len(m.Parameters))
	}
	expected := []string{"a", "b", "c"}
	for i, p := range m.Parameters {
		if p != expected[i] {
			t.Errorf("param %d: expected %q, got %q", i, expected[i], p)
		}
	}
}
