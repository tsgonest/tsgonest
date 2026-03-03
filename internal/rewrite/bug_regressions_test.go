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

	t.Run("Bug19_cjs_chained_assignment_interceptor_not_injected", func(t *testing.T) {
		// Real tsgo CJS output uses: exports.X = X = __decorate([...], X);
		// The rewriter must detect this chained assignment pattern and inject
		// the interceptor into the __decorate array.
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.AuthController = void 0;
const common_1 = require("@nestjs/common");
let AuthController = class AuthController {
    async login(body) {
        return this.authService.login(body);
    }
};
exports.AuthController = AuthController;
__decorate([
    (0, common_1.Post)('login'),
    __param(0, (0, common_1.Body)()),
    __metadata("design:type", Function),
    __metadata("design:paramtypes", [Object]),
    __metadata("design:returntype", Promise)
], AuthController.prototype, "login", null);
exports.AuthController = AuthController = __decorate([
    (0, common_1.Controller)('auth'),
    __metadata("design:paramtypes", [Object])
], AuthController);`

		controllers := []analyzer.ControllerInfo{{
			Name:       "AuthController",
			SourceFile: "/src/auth.controller.ts",
			Routes: []analyzer.Route{{
				OperationID: "login",
				MethodName:  "login",
				Parameters: []analyzer.RouteParameter{{
					Category:  "body",
					LocalName: "body",
					Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "LoginRequest"},
				}},
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "LoginResponse"},
			}},
		}}

		companionMap := map[string]string{
			"LoginRequest":  "/dist/dto/auth.dto.LoginRequest.tsgonest.js",
			"LoginResponse": "/dist/dto/auth.dto.LoginResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/auth.controller.js", controllers, companionMap, "cjs")

		if !strings.Contains(result, "UseInterceptors)(TsgonestSerializeInterceptor)") {
			t.Fatalf("expected interceptor injection for CJS chained assignment, got:\n%s", result)
		}
		if !strings.Contains(result, "assertLoginRequest(body)") {
			t.Errorf("expected body validation, got:\n%s", result)
		}
		if !strings.Contains(result, "stringifyLoginResponse(await") {
			t.Errorf("expected return stringify wrapping, got:\n%s", result)
		}
	})

	t.Run("Bug20_cjs_imports_before_use_strict", func(t *testing.T) {
		// CJS imports must be inserted AFTER "use strict"; directive,
		// not before it (which would make strict mode a no-op).
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const common_1 = require("@nestjs/common");
let UserController = class UserController {
    async findAll() {
        return this.service.findAll();
    }
};
UserController = __decorate([
    (0, common_1.Controller)("users")
], UserController);`

		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				OperationID: "findAll",
				MethodName:  "findAll",
				ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
			}},
		}}

		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "cjs")

		// "use strict" must remain the very first statement
		if !strings.HasPrefix(result, `"use strict";`) {
			t.Fatalf("expected \"use strict\" to remain first line, got:\n%s", result)
		}

		// Imports must come after "use strict"
		useStrictIdx := strings.Index(result, `"use strict";`)
		requireIdx := strings.Index(result, `require(`)
		if requireIdx < useStrictIdx+len(`"use strict";`) {
			t.Fatalf("expected require() after \"use strict\", got:\n%s", result)
		}
	})

	t.Run("Bug21_esm_no_use_strict_unchanged", func(t *testing.T) {
		// ESM files don't have "use strict" — imports should go at position 0.
		input := `var __decorate = (this && this.__decorate) || function() {};
class UserController {
    async findAll() {
        return this.service.findAll();
    }
}
UserController = __decorate([
    Controller("users")
], UserController);`

		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				OperationID: "findAll",
				MethodName:  "findAll",
				ReturnType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
			}},
		}}

		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm")

		// Import should be at the very beginning (no "use strict" to skip past)
		if !strings.HasPrefix(result, `import {`) {
			t.Fatalf("expected ESM import at position 0, got:\n%s", result)
		}
	})

	t.Run("Bug22_markers_rewrite_preserves_use_strict", func(t *testing.T) {
		input := `"use strict";
const { is } = require("tsgonest");
const ok = is(body);`

		calls := []MarkerCall{
			{FunctionName: "is", TypeName: "CreateUserDto", SourcePos: 0},
		}
		companionMap := map[string]string{
			"CreateUserDto": "/dist/user.dto.CreateUserDto.tsgonest.js",
		}

		result := rewriteMarkers(input, "/dist/test.js", calls, companionMap, "cjs")

		// "use strict" must remain first (before sentinel and imports)
		if !strings.HasPrefix(result, `"use strict";`) {
			t.Fatalf("expected \"use strict\" to remain first, got:\n%s", result)
		}

		// Sentinel and imports should come after
		useStrictEnd := strings.Index(result, "\n") + 1
		rest := result[useStrictEnd:]
		if !strings.HasPrefix(rest, rewriteSentinel) {
			t.Fatalf("expected sentinel after \"use strict\", got:\n%s", result)
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
