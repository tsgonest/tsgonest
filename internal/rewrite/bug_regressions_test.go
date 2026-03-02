package rewrite

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

func TestBugRegressions_Rewrite(t *testing.T) {
	t.Run("Bug01_if_block_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m(x) {
        if (x) {
            return x;
        }
        throw new Error("nope");
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await x);") {
			t.Fatalf("expected nested if return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug02_else_block_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m(x) {
        if (x) {
            return x;
        } else {
            return 0;
        }
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await 0);") {
			t.Fatalf("expected else return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug03_switch_case_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m(x) {
        switch (x) {
            case 1:
                return 1;
            default:
                return 2;
        }
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await 1);") {
			t.Fatalf("expected switch return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug04_try_block_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m() {
        try {
            return this.service.load();
        } catch (e) {
            throw e;
        }
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await this.service.load())") {
			t.Fatalf("expected try return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug05_catch_block_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m() {
        try {
            throw new Error("x");
        } catch (e) {
            return e.message;
        }
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await e.message);") {
			t.Fatalf("expected catch return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug06_for_loop_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m(items) {
        for (const item of items) {
            return item;
        }
        return null;
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await item);") {
			t.Fatalf("expected loop return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug07_while_loop_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m(flag) {
        while (flag) {
            return flag;
        }
        return false;
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await flag);") {
			t.Fatalf("expected while-loop return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug08_asi_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m() {
        return this.service.load()
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await this.service.load())") {
			t.Fatalf("expected ASI return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug09_nested_asi_return_not_wrapped", func(t *testing.T) {
		input := `class C {
    async m(x) {
        if (x) {
            return x
        }
        return null
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "return stringifyUser(await x)") {
			t.Fatalf("expected nested ASI return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug10_make_method_async_fails_single_line_class", func(t *testing.T) {
		input := `class C { foo(){ return 1; } }`
		got := wrapReturnsInMethod(input, "foo", "stringifyUser", false)
		if !strings.Contains(got, "async foo(") {
			t.Fatalf("expected async keyword injection for single-line class methods, got:\n%s", got)
		}
	})

	t.Run("Bug11_make_method_async_fails_unindented_method", func(t *testing.T) {
		input := "class C {\nfoo(){ return 1; }\n}"
		got := wrapReturnsInMethod(input, "foo", "stringifyUser", false)
		if !strings.Contains(got, "async foo(") {
			t.Fatalf("expected async keyword injection for unindented methods, got:\n%s", got)
		}
	})

	t.Run("Bug12_destructured_body_param_name_not_rejected", func(t *testing.T) {
		input := `class C {
    create({ id }) {
        return id;
    }
}`
		got := findBodyParamName(input, "create")
		if got != "" {
			t.Fatalf("expected destructured @Body param to be rejected (empty name), got %q", got)
		}
	})

	t.Run("Bug13_number_null_union_not_detected_as_primitive", func(t *testing.T) {
		m := metadata.Metadata{
			Kind: metadata.KindUnion,
			UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindAtomic, Atomic: "number"},
				{Kind: metadata.KindAtomic, Atomic: "null"},
			},
		}
		atomic, nullable := resolvePrimitiveReturn(&m)
		if atomic != "number" || !nullable {
			t.Fatalf("expected number|null to resolve as nullable primitive number, got atomic=%q nullable=%v", atomic, nullable)
		}
	})

	t.Run("Bug14_string_literal_union_with_null_not_detected", func(t *testing.T) {
		m := metadata.Metadata{
			Kind: metadata.KindUnion,
			UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: "a"},
				{Kind: metadata.KindLiteral, LiteralValue: "b"},
				{Kind: metadata.KindAtomic, Atomic: "null"},
			},
		}
		atomic, nullable := resolvePrimitiveReturn(&m)
		if atomic != "string" || !nullable {
			t.Fatalf("expected ('a'|'b'|null) to resolve as nullable primitive string, got atomic=%q nullable=%v", atomic, nullable)
		}
	})

	t.Run("Bug15_boolean_literal_union_with_null_not_detected", func(t *testing.T) {
		m := metadata.Metadata{
			Kind: metadata.KindUnion,
			UnionMembers: []metadata.Metadata{
				{Kind: metadata.KindLiteral, LiteralValue: true},
				{Kind: metadata.KindLiteral, LiteralValue: false},
				{Kind: metadata.KindAtomic, Atomic: "null"},
			},
		}
		atomic, nullable := resolvePrimitiveReturn(&m)
		if atomic != "boolean" || !nullable {
			t.Fatalf("expected (true|false|null) to resolve as nullable primitive boolean, got atomic=%q nullable=%v", atomic, nullable)
		}
	})

	t.Run("Bug16_boolean_query_coercion_missing_invalid_value_guard", func(t *testing.T) {
		input := `class UserController {
    async list(active) {
        return this.service.list(active);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "list",
				Parameters: []analyzer.RouteParameter{{
					Category:  "query",
					Name:      "active",
					LocalName: "active",
					Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
				}},
			}},
		}}

		got := rewriteController(input, "/dist/user.controller.js", controllers, map[string]string{}, "esm")
		if !strings.Contains(got, "throw new __e") {
			t.Fatalf("expected invalid boolean value guard to throw validation error, got:\n%s", got)
		}
	})

	t.Run("Bug17_number_query_coercion_missing_empty_string_guard", func(t *testing.T) {
		input := `class UserController {
    async list(page) {
        return this.service.list(page);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "list",
				Parameters: []analyzer.RouteParameter{{
					Category:  "query",
					Name:      "page",
					LocalName: "page",
					Type:      metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				}},
			}},
		}}

		got := rewriteController(input, "/dist/user.controller.js", controllers, map[string]string{}, "esm")
		if !strings.Contains(got, `=== ""`) {
			t.Fatalf("expected empty-string guard before numeric coercion, got:\n%s", got)
		}
	})

	t.Run("Bug18_duplicate_class_interceptor_injection", func(t *testing.T) {
		input := `var common_1 = require("@nestjs/common");
UserController = __decorate([
    (0, common_1.Controller)("users"),
    (0, common_1.UseInterceptors)(TsgonestSerializeInterceptor)
], UserController);
class UserController {}
`
		controllers := []analyzer.ControllerInfo{{Name: "UserController"}}
		got := injectClassInterceptor(input, controllers, "TsgonestSerializeInterceptor")
		if count := strings.Count(got, "UseInterceptors)(TsgonestSerializeInterceptor)"); count != 1 {
			t.Fatalf("expected interceptor injection to be deduplicated, got %d occurrences\n%s", count, got)
		}
	})

}
