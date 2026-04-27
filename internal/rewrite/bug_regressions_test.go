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
		got := findBodyParamName(input, "C", "create")
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

		got := rewriteController(input, "/dist/user.controller.js", controllers, map[string]string{}, "esm", nil)
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

		got := rewriteController(input, "/dist/user.controller.js", controllers, map[string]string{}, "esm", nil)
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

		result := rewriteController(input, "/dist/auth.controller.js", controllers, companionMap, "cjs", nil)

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

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "cjs", nil)

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

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

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

	t.Run("Bug23_multiple_controllers_same_method_name_panics", func(t *testing.T) {
		// When two controllers in the same file both have a method with the same name
		// (e.g., "findAll"), methodLookup only retains one MethodLoc for that name.
		// Both transforms then build edits targeting the exact same return statement
		// range [RP, SE]. ApplyBulkEdits applies edit 1, sets lastEnd=SE, then tries
		// text[SE:RP] for edit 2 → panic: slice bounds out of range [SE:RP].
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.RefundsController = exports.PaymentsController = void 0;
const common_1 = require("@nestjs/common");
let PaymentsController = class PaymentsController {
    async findAll() {
        return this.paymentsService.findAll();
    }
};
exports.PaymentsController = PaymentsController;
__decorate([
    (0, common_1.Get)(),
    __metadata("design:type", Function),
    __metadata("design:returntype", Promise)
], PaymentsController.prototype, "findAll", null);
exports.PaymentsController = PaymentsController = __decorate([
    (0, common_1.Controller)('payments'),
    __metadata("design:paramtypes", [Object])
], PaymentsController);
let RefundsController = class RefundsController {
    async findAll() {
        return this.refundsService.findAll();
    }
};
exports.RefundsController = RefundsController;
__decorate([
    (0, common_1.Get)(),
    __metadata("design:type", Function),
    __metadata("design:returntype", Promise)
], RefundsController.prototype, "findAll", null);
exports.RefundsController = RefundsController = __decorate([
    (0, common_1.Controller)('refunds'),
    __metadata("design:paramtypes", [Object])
], RefundsController);`

		controllers := []analyzer.ControllerInfo{
			{
				Name:       "PaymentsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "PaymentDto"},
				}},
			},
			{
				Name:       "RefundsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "RefundDto"},
				}},
			},
		}
		companionMap := map[string]string{
			"PaymentDto": "/dist/payment.dto.PaymentDto.tsgonest.js",
			"RefundDto":  "/dist/refund.dto.RefundDto.tsgonest.js",
		}

		// Should not panic. Each controller's findAll must be wrapped with its own DTO.
		result := rewriteController(input, "/dist/payments.controller.js", controllers, companionMap, "cjs", nil)
		if !strings.Contains(result, "stringifyPaymentDto") {
			t.Errorf("expected PaymentDto stringify in result, got:\n%s", result)
		}
		if !strings.Contains(result, "stringifyRefundDto") {
			t.Errorf("expected RefundDto stringify in result, got:\n%s", result)
		}
	})

	t.Run("Bug24_same_method_name_body_validations_double_inject", func(t *testing.T) {
		// Two controllers in the same file both have a method named "create" with a
		// @Body param. methodLookup["create"] resolves to only ONE MethodLoc, so
		// BOTH assert insertions land at the same BodyOpenBrace+1. Result: one method
		// gets two assertions (assertPaymentInput then assertRefundInput), the other
		// method gets none.
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const common_1 = require("@nestjs/common");
let PaymentsController = class PaymentsController {
    async create(body) {
        return this.paymentsService.create(body);
    }
};
exports.PaymentsController = PaymentsController = __decorate([
    (0, common_1.Controller)('payments')
], PaymentsController);
let RefundsController = class RefundsController {
    async create(body) {
        return this.refundsService.create(body);
    }
};
exports.RefundsController = RefundsController = __decorate([
    (0, common_1.Controller)('refunds')
], RefundsController);`

		controllers := []analyzer.ControllerInfo{
			{
				Name:       "PaymentsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "create",
					Parameters: []analyzer.RouteParameter{{
						Category:  "body",
						LocalName: "body",
						Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreatePaymentInput"},
					}},
				}},
			},
			{
				Name:       "RefundsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "create",
					Parameters: []analyzer.RouteParameter{{
						Category:  "body",
						LocalName: "body",
						Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateRefundInput"},
					}},
				}},
			},
		}
		companionMap := map[string]string{
			"CreatePaymentInput": "/dist/payment.dto.CreatePaymentInput.tsgonest.js",
			"CreateRefundInput":  "/dist/refund.dto.CreateRefundInput.tsgonest.js",
		}

		result := rewriteController(input, "/dist/payments.controller.js", controllers, companionMap, "cjs", nil)
		// Use service-call positions as anchors: paymentsService precedes refundsService in
		// the file. assertCreatePaymentInput must appear before paymentsService (i.e. injected
		// at the start of PaymentsController.create). assertCreateRefundInput must appear after
		// paymentsService but before refundsService (i.e. in RefundsController.create, not
		// accidentally in PaymentsController.create).
		payAssertIdx := strings.Index(result, "assertCreatePaymentInput(body)")
		refAssertIdx := strings.Index(result, "assertCreateRefundInput(body)")
		payServiceIdx := strings.Index(result, "paymentsService")
		refServiceIdx := strings.Index(result, "refundsService")
		if payAssertIdx == -1 || payAssertIdx > payServiceIdx {
			t.Errorf("assertCreatePaymentInput not injected into PaymentsController.create body:\n%s", result)
		}
		if refAssertIdx == -1 || refAssertIdx < payServiceIdx || refAssertIdx > refServiceIdx {
			t.Errorf("assertCreateRefundInput not injected into RefundsController.create body:\n%s", result)
		}
	})

	t.Run("Bug25_same_method_name_primitive_return_panics", func(t *testing.T) {
		// Same root cause as Bug23 but with primitiveTransforms instead of DTO transforms.
		// Both controllers have a method "getStatus" returning string.
		// methodLookup["getStatus"] resolves to one MethodLoc; both primitiveTransforms
		// emit replacement edits [RP, SE] for that same return statement → panic in ApplyBulkEdits.
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const common_1 = require("@nestjs/common");
let PaymentsController = class PaymentsController {
    async getStatus() {
        return this.paymentsService.getStatus();
    }
};
exports.PaymentsController = PaymentsController = __decorate([
    (0, common_1.Controller)('payments')
], PaymentsController);
let RefundsController = class RefundsController {
    async getStatus() {
        return this.refundsService.getStatus();
    }
};
exports.RefundsController = RefundsController = __decorate([
    (0, common_1.Controller)('refunds')
], RefundsController);`

		controllers := []analyzer.ControllerInfo{
			{
				Name:       "PaymentsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "getStatus",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				}},
			},
			{
				Name:       "RefundsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "getStatus",
					ReturnType: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				}},
			},
		}

		// Should not panic. Each controller's getStatus must be wrapped independently.
		result := rewriteController(input, "/dist/payments.controller.js", controllers, map[string]string{}, "cjs", nil)
		if strings.Count(result, "JSON.stringify") != 2 {
			t.Errorf("expected two JSON.stringify wrappings (one per controller), got:\n%s", result)
		}
	})

	t.Run("Bug26_interceptor_injected_into_all_controllers_when_only_one_has_transform", func(t *testing.T) {
		// When ANY controller in the file has a return transform, the interceptor
		// injection loop iterates ALL controllers and injects TsgonestSerializeInterceptor
		// into every class-level __decorate, even controllers that have no return
		// transforms (e.g. only @Body validation). This is wrong: the interceptor
		// should only be added to controllers that actually have serialized return types.
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const common_1 = require("@nestjs/common");
let PaymentsController = class PaymentsController {
    async findAll() {
        return this.paymentsService.findAll();
    }
};
exports.PaymentsController = PaymentsController = __decorate([
    (0, common_1.Controller)('payments')
], PaymentsController);
let WebhooksController = class WebhooksController {
    async handle(body) {
        this.webhooksService.handle(body);
    }
};
exports.WebhooksController = WebhooksController = __decorate([
    (0, common_1.Controller)('webhooks')
], WebhooksController);`

		controllers := []analyzer.ControllerInfo{
			{
				Name:       "PaymentsController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "findAll",
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "PaymentDto"},
				}},
			},
			{
				Name:       "WebhooksController",
				SourceFile: "/src/payments.controller.ts",
				Routes: []analyzer.Route{{
					MethodName: "handle",
					Parameters: []analyzer.RouteParameter{{
						Category:  "body",
						LocalName: "body",
						Type:      metadata.Metadata{Kind: metadata.KindRef, Ref: "WebhookPayload"},
					}},
				}},
			},
		}
		companionMap := map[string]string{
			"PaymentDto":     "/dist/payment.dto.PaymentDto.tsgonest.js",
			"WebhookPayload": "/dist/webhook.dto.WebhookPayload.tsgonest.js",
		}

		result := rewriteController(input, "/dist/payments.controller.js", controllers, companionMap, "cjs", nil)
		// TsgonestSerializeInterceptor must appear only once — in PaymentsController's
		// __decorate. WebhooksController has no return transforms and must not get it.
		count := strings.Count(result, "UseInterceptors)(TsgonestSerializeInterceptor)")
		if count != 1 {
			t.Errorf("expected TsgonestSerializeInterceptor in exactly 1 controller's __decorate, got %d\n%s", count, result)
		}
		// The interceptor must appear before WebhooksController's __decorate bracket.
		interceptorIdx := strings.Index(result, "UseInterceptors)(TsgonestSerializeInterceptor)")
		webhooksDecorateIdx := strings.Index(result, "Controller)('webhooks')")
		if interceptorIdx != -1 && webhooksDecorateIdx != -1 && interceptorIdx > webhooksDecorateIdx {
			t.Errorf("interceptor leaked into WebhooksController's __decorate:\n%s", result)
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

	t.Run("Bug27_interceptor_sentinel_scans_past_current_class", func(t *testing.T) {
		// Bug: text[dc.ArrayOpenBracket:] scans to end-of-file for deduplication.
		// If ControllerB (later in file) already has the interceptor, the sentinel
		// would find it when scanning from ControllerA's bracket position and
		// incorrectly skip injecting into ControllerA.
		// Fixed by scoping the check to text[dc.ArrayOpenBracket:dc.StmtEnd].
		input := `var common_1 = require("@nestjs/common");
let ControllerA = class ControllerA {
    async findAll() { return null; }
};
ControllerA = __decorate([
    (0, common_1.Controller)('a')
], ControllerA);
let ControllerB = class ControllerB {
    async findAll() { return null; }
};
ControllerB = __decorate([
    (0, common_1.UseInterceptors)(TsgonestSerializeInterceptor),
    (0, common_1.Controller)('b')
], ControllerB);`

		controllers := []analyzer.ControllerInfo{
			{Name: "ControllerA"},
			{Name: "ControllerB"},
		}
		got := injectClassInterceptor(input, controllers, "TsgonestSerializeInterceptor")
		// ControllerA needs the interceptor; ControllerB already has it.
		// Both should have exactly one interceptor each.
		count := strings.Count(got, "UseInterceptors)(TsgonestSerializeInterceptor)")
		if count != 2 {
			t.Fatalf("expected 2 interceptors (one per controller), got %d\n%s", count, got)
		}
	})

	// ── Issue #78: Serializer fails for nullable return types (T | null) ──
	//
	// All Bug28-Bug34 tests cover realistic patterns where nullable/optional
	// DTO return types cause runtime TypeErrors because the controller rewriter
	// doesn't add null guards before calling stringify functions.

	t.Run("Bug28_nullable_dto_return_rejects_null_at_runtime", func(t *testing.T) {
		// Most common pattern: Prisma findFirst/findUnique returns T | null.
		//   async findById(id: string): Promise<UserResponse | null> {
		//     return this.userService.findById(id);
		//   }
		// Walker produces KindRef{Ref:"UserResponse", Nullable:true}.
		// Without fix: stringifyUserResponse(await expr) → TypeError on null.
		// With fix: ((_v) => _v == null ? "null" : stringifyUserResponse(_v))(await expr)
		input := `class UserController {
    async getById(id) {
        return this.userService.findById(id);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "getById",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Nullable: true},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		if !strings.Contains(result, "stringifyUserResponse") {
			t.Fatalf("expected stringifyUserResponse in result, got:\n%s", result)
		}
		// The crucial check: null must be checked BEFORE stringify is called.
		if !strings.Contains(result, `== null`) && !strings.Contains(result, `=== null`) {
			t.Fatalf("expected null equality check in return wrapping for nullable DTO return type, got:\n%s", result)
		}
		// The null-check path must produce the JSON literal "null" (the string).
		if !strings.Contains(result, `"null"`) {
			t.Fatalf("expected '\"null\"' JSON string in null branch, got:\n%s", result)
		}
	})

	t.Run("Bug28b_non_nullable_dto_return_no_null_check", func(t *testing.T) {
		// Baseline: non-nullable DTO returns should NOT have a null guard.
		input := `class UserController {
    async getById(id) {
        return this.userService.findById(id);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "getById",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		if !strings.Contains(result, "stringifyUserResponse(await") {
			t.Fatalf("expected direct stringifyUserResponse(await ...) for non-nullable return, got:\n%s", result)
		}
	})

	t.Run("Bug29_nullable_array_return_crashes_on_null_map", func(t *testing.T) {
		// Pattern: search endpoint returns array or null when no results / invalid query.
		//   async search(q: string): Promise<UserResponse[] | null> { ... }
		// Walker produces KindArray{ElementType:KindRef{Ref:"UserResponse"}, Nullable:true}.
		// Without fix: (await expr).map(_v => serializeUserResponse(_v)) → TypeError:
		//   Cannot read properties of null (reading 'map')
		// With fix: null check before .map()
		input := `class UserController {
    async search(q) {
        return this.userService.search(q);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "search",
				ReturnType: metadata.Metadata{
					Kind:     metadata.KindArray,
					Nullable: true,
					ElementType: &metadata.Metadata{
						Kind: metadata.KindRef,
						Ref:  "UserResponse",
					},
				},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Must handle null before calling .map()
		if !strings.Contains(result, `== null`) && !strings.Contains(result, `=== null`) {
			t.Fatalf("expected null check before .map() for nullable array return, got:\n%s", result)
		}
		// The null branch must produce "null" JSON string (not "[]")
		if !strings.Contains(result, `"null"`) {
			t.Fatalf("expected '\"null\"' JSON string for null array, got:\n%s", result)
		}
	})

	t.Run("Bug30_non_async_nullable_dto_return", func(t *testing.T) {
		// Pattern: synchronous cache lookup or default value.
		//   getDefault(): UserResponse | null { return this.cache.get("default"); }
		// The rewriter must insert BOTH `async` AND the null guard.
		input := `class UserController {
    getDefault() {
        return this.cache.get("default");
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "getDefault",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Nullable: true},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Must insert async
		if !strings.Contains(result, "async getDefault") {
			t.Fatalf("expected async keyword insertion for non-async method, got:\n%s", result)
		}
		// Must have null guard
		if !strings.Contains(result, `== null`) && !strings.Contains(result, `=== null`) {
			t.Fatalf("expected null check for non-async nullable return, got:\n%s", result)
		}
	})

	t.Run("Bug31_optional_dto_return_undefined_not_handled", func(t *testing.T) {
		// Pattern: TypeORM v0.2 findOne() or Array.find() returns T | undefined.
		//   async findOne(id: string): Promise<UserResponse | undefined> { ... }
		// Walker produces KindRef{Ref:"UserResponse", Optional:true}.
		// Without fix: stringifyUserResponse(await expr) → TypeError when undefined.
		// JS `== null` catches both null and undefined, so the same fix applies.
		input := `class UserController {
    async findOne(id) {
        return this.users.find(u => u.id === id);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "findOne",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Optional: true},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		if !strings.Contains(result, "stringifyUserResponse") {
			t.Fatalf("expected stringifyUserResponse in result, got:\n%s", result)
		}
		// Optional (undefined) must also be guarded, same as nullable
		if !strings.Contains(result, `== null`) && !strings.Contains(result, `=== null`) {
			t.Fatalf("expected null/undefined check for optional DTO return, got:\n%s", result)
		}
	})

	t.Run("Bug32_nullable_and_optional_dto_return", func(t *testing.T) {
		// Pattern: some ORMs / utility types return T | null | undefined.
		// Walker produces KindRef{Ref:"UserResponse", Nullable:true, Optional:true}.
		input := `class UserController {
    async findOne(id) {
        return this.repo.findOneBy({ id });
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "findOne",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Nullable: true, Optional: true},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Must guard against both null and undefined
		if !strings.Contains(result, `== null`) && !strings.Contains(result, `=== null`) {
			t.Fatalf("expected null/undefined check for nullable+optional return, got:\n%s", result)
		}
	})

	t.Run("Bug33_nullable_dto_return_multiple_return_paths", func(t *testing.T) {
		// Pattern: try/catch with null fallback — every return path must be wrapped.
		//   async findOne(id): Promise<UserResponse | null> {
		//     try { return await this.service.get(id); }
		//     catch { return null; }
		//   }
		// Both `return this.service.get(id)` and `return null` are return statements
		// that get wrapped. Without fix, both become:
		//   return stringifyUserResponse(await this.service.get(id))
		//   return stringifyUserResponse(await null)
		// The second call throws. With fix, both get the null guard.
		input := `class UserController {
    async findOne(id) {
        try {
            return this.service.get(id);
        } catch (e) {
            return null;
        }
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "findOne",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Nullable: true},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Both return paths must have the null guard wrapper
		nullCheckCount := strings.Count(result, `== null`)
		if nullCheckCount < 2 {
			t.Fatalf("expected null check on BOTH return paths (try + catch), got %d null checks:\n%s", nullCheckCount, result)
		}
	})

	t.Run("Bug34_nullable_dto_return_cjs_format", func(t *testing.T) {
		// Same bug in CJS output — many NestJS projects use CommonJS.
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const common_1 = require("@nestjs/common");
let UserController = class UserController {
    async findById(id) {
        return this.userService.findById(id);
    }
};
exports.UserController = UserController = __decorate([
    (0, common_1.Controller)('users')
], UserController);`

		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "findById",
				ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Nullable: true},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "cjs", nil)

		if !strings.Contains(result, "stringifyUserResponse") {
			t.Fatalf("expected stringifyUserResponse in CJS result, got:\n%s", result)
		}
		if !strings.Contains(result, `== null`) && !strings.Contains(result, `=== null`) {
			t.Fatalf("expected null check in CJS nullable DTO return, got:\n%s", result)
		}
	})

	t.Run("Bug35_mixed_nullable_and_non_nullable_returns_in_same_controller", func(t *testing.T) {
		// Pattern: same controller has both nullable and non-nullable DTO returns.
		//   getById(id): Promise<UserResponse | null>     ← needs null guard
		//   getAll():     Promise<UserResponse[]>          ← NO null guard
		// The rewriter must be selective: only nullable returns get the guard.
		input := `class UserController {
    async getById(id) {
        return this.service.findById(id);
    }
    async getAll() {
        return this.service.findAll();
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{
				{
					MethodName: "getById",
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse", Nullable: true},
				},
				{
					MethodName: "getAll",
					ReturnType: metadata.Metadata{
						Kind: metadata.KindArray,
						ElementType: &metadata.Metadata{
							Kind: metadata.KindRef,
							Ref:  "UserResponse",
						},
					},
				},
			},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// getById: must have null guard
		getByIdIdx := strings.Index(result, "getById")
		getAllIdx := strings.Index(result, "getAll")
		if getByIdIdx == -1 || getAllIdx == -1 {
			t.Fatalf("expected both methods in result, got:\n%s", result)
		}
		getByIdSection := result[getByIdIdx:getAllIdx]
		if !strings.Contains(getByIdSection, `== null`) && !strings.Contains(getByIdSection, `=== null`) {
			t.Fatalf("expected null check in getById (nullable), got:\n%s", result)
		}

		// getAll: must NOT have null guard (non-nullable array)
		getAllSection := result[getAllIdx:]
		if strings.Contains(getAllSection, `== null`) || strings.Contains(getAllSection, `=== null`) {
			t.Fatalf("unexpected null check in getAll (non-nullable array return), got:\n%s", result)
		}
	})

	t.Run("Bug36_array_of_nullable_elements_crashes_on_null_element", func(t *testing.T) {
		// Pattern: Promise.all over findFirst results → (UserDto | null)[]
		//   async getByIds(ids: string[]): Promise<(UserResponse | null)[]> {
		//     return Promise.all(ids.map(id => this.findById(id)));
		//   }
		// Metadata: KindArray{ElementType: KindRef{Ref:"UserResponse", Nullable:true}}
		// Without fix: (await expr).map(_v => serializeUserResponse(_v))
		//   serializeUserResponse(null) crashes with TypeError
		// With fix: (await expr).map((_i) => _i == null ? "null" : serializeUserResponse(_i))
		input := `class UserController {
    async getByIds(ids) {
        return Promise.all(ids.map(id => this.findById(id)));
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "getByIds",
				ReturnType: metadata.Metadata{
					Kind: metadata.KindArray,
					ElementType: &metadata.Metadata{
						Kind:     metadata.KindRef,
						Ref:      "UserResponse",
						Nullable: true,
					},
				},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// The .map() callback must have a null guard for each element
		if !strings.Contains(result, `== null`) {
			t.Fatalf("expected element-level null check in .map() for array of nullable DTOs, got:\n%s", result)
		}
		// The null guard should produce "null" JSON string for null elements
		if !strings.Contains(result, `"null"`) {
			t.Fatalf("expected '\"null\"' JSON string for null array elements, got:\n%s", result)
		}
		// Should still contain serializeUserResponse for non-null elements
		if !strings.Contains(result, "serializeUserResponse") {
			t.Fatalf("expected serializeUserResponse for non-null elements, got:\n%s", result)
		}
	})

	t.Run("Bug36b_array_of_non_nullable_elements_no_element_null_check", func(t *testing.T) {
		// Baseline: (UserResponse)[] — elements are NOT nullable.
		// No element-level null check should be injected.
		input := `class UserController {
    async findAll() {
        return this.service.findAll();
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "findAll",
				ReturnType: metadata.Metadata{
					Kind: metadata.KindArray,
					ElementType: &metadata.Metadata{
						Kind: metadata.KindRef,
						Ref:  "UserResponse",
					},
				},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Non-nullable elements: .map(_v => serializeUserResponse(_v)) without null guard
		if strings.Contains(result, `== null`) {
			t.Fatalf("unexpected element null check for non-nullable array elements, got:\n%s", result)
		}
	})

	t.Run("Bug37_nullable_array_of_nullable_elements", func(t *testing.T) {
		// Double nullable: Promise<(UserResponse | null)[] | null>
		// Both the array itself AND its elements can be null.
		// Metadata: KindArray{Nullable:true, ElementType: KindRef{Ref:"UserResponse", Nullable:true}}
		input := `class UserController {
    async search(q) {
        return this.service.search(q);
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "search",
				ReturnType: metadata.Metadata{
					Kind:     metadata.KindArray,
					Nullable: true,
					ElementType: &metadata.Metadata{
						Kind:     metadata.KindRef,
						Ref:      "UserResponse",
						Nullable: true,
					},
				},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Must guard the array-level null AND element-level null
		nullChecks := strings.Count(result, `== null`)
		if nullChecks < 2 {
			t.Fatalf("expected at least 2 null checks (array-level + element-level), got %d:\n%s", nullChecks, result)
		}
	})

	// ── Complex controller logic patterns ──────────────────────────────
	// Tests for developers who write substantial inline logic in controllers
	// rather than delegating to services. All of these use wrapReturnsInMethod
	// which exercises the JS locator + wrapping on arbitrary controller methods.

	t.Run("Bug38_inline_object_construction_in_return", func(t *testing.T) {
		// Pattern: building response object inline in controller
		//   return { id: user.id, name: user.firstName + " " + user.lastName, ...extra };
		input := `class C {
    async m(user) {
        const extra = { role: "admin" };
        return { id: user.id, name: user.firstName + " " + user.lastName, ...extra };
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		// The full object literal must be captured and wrapped
		if !strings.Contains(got, "stringifyUser(await { id: user.id") {
			t.Fatalf("expected object literal to be wrapped, got:\n%s", got)
		}
		if !strings.Contains(got, "...extra }") {
			t.Fatalf("expected spread operator to be preserved, got:\n%s", got)
		}
	})

	t.Run("Bug39_await_in_return_expression", func(t *testing.T) {
		// Pattern: awaiting a service call directly in the return
		//   return await this.repo.save(entity);
		// The wrapping adds its own await: stringifyT(await await this.repo.save(entity))
		// Double-await is harmless (await on a non-Promise is a no-op).
		input := `class C {
    async m(entity) {
        return await this.repo.save(entity);
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "stringifyUser(await await this.repo.save(entity))") {
			t.Fatalf("expected double-await wrapping (harmless), got:\n%s", got)
		}
	})

	t.Run("Bug40_multiline_return_expression", func(t *testing.T) {
		// Pattern: multiline chained operations in return
		input := `class C {
    async m(data) {
        return this.items
            .filter(item => item.active)
            .map(item => this.transform(item));
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyList", false)
		// The full chained expression should be wrapped
		if !strings.Contains(got, "stringifyList(await this.items") {
			t.Fatalf("expected multiline chained expression to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug41_conditional_early_returns_with_complex_logic", func(t *testing.T) {
		// Pattern: controller with multiple early returns, guards, and transformations
		input := `class C {
    async m(id, user) {
        if (!user.isAdmin) {
            return this.getPublicView(id);
        }
        const entity = await this.repo.findOne(id);
        if (!entity) {
            throw new NotFoundException();
        }
        return this.toDto(entity);
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		// Both return paths must be wrapped
		if !strings.Contains(got, "stringifyUser(await this.getPublicView(id))") {
			t.Fatalf("expected first return to be wrapped, got:\n%s", got)
		}
		if !strings.Contains(got, "stringifyUser(await this.toDto(entity))") {
			t.Fatalf("expected second return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug42_nested_arrow_fn_return_not_wrapped", func(t *testing.T) {
		// Pattern: controller uses .map/.filter with arrow function returns.
		// The arrow function returns must NOT be wrapped — only the method return.
		input := `class C {
    async m() {
        const items = await this.service.list();
        const transformed = items.map(item => {
            return { ...item, processed: true };
        });
        return transformed;
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyList", false)
		// Only the method-level return should be wrapped
		if !strings.Contains(got, "stringifyList(await transformed)") {
			t.Fatalf("expected only method return to be wrapped, got:\n%s", got)
		}
		// The arrow function's return should NOT be wrapped
		if strings.Contains(got, "stringifyList(await { ...item") {
			t.Fatalf("arrow function return should NOT be wrapped, got:\n%s", got)
		}
	})

	t.Run("Bug43_iife_in_return_expression", func(t *testing.T) {
		// Pattern: IIFE for inline complex transformation
		input := `class C {
    async m() {
        return (() => {
            const base = this.getBase();
            return { ...base, extra: true };
        })();
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		// The method-level return (the IIFE call) should be wrapped
		if !strings.Contains(got, "stringifyUser(await (() =>") {
			t.Fatalf("expected IIFE return to be wrapped at method level, got:\n%s", got)
		}
		// Inner return (inside arrow) should NOT be wrapped
		if strings.Count(got, "stringifyUser") != 1 {
			t.Fatalf("expected exactly 1 stringifyUser call (method level only), got:\n%s", got)
		}
	})

	t.Run("Bug44_nested_array_return_silently_skipped", func(t *testing.T) {
		// Pattern: Promise<UserDto[][]> — nested array return.
		// resolveReturnTypeName used to recurse through all array levels and return
		// the base element type name. The .map() wrapper then called serializeUserDto
		// on UserDto[] (an array) instead of a single UserDto → runtime crash.
		// Fix: detect nested arrays and skip the transform entirely, letting NestJS
		// handle serialization with its default JSON.stringify.
		input := `class UserController {
    async getMatrix() {
        return this.service.getMatrix();
    }
}`
		controllers := []analyzer.ControllerInfo{{
			Name:       "UserController",
			SourceFile: "/src/user.controller.ts",
			Routes: []analyzer.Route{{
				MethodName: "getMatrix",
				ReturnType: metadata.Metadata{
					Kind: metadata.KindArray,
					ElementType: &metadata.Metadata{
						Kind: metadata.KindArray,
						ElementType: &metadata.Metadata{
							Kind: metadata.KindRef,
							Ref:  "UserResponse",
						},
					},
				},
			}},
		}}
		companionMap := map[string]string{
			"UserResponse": "/dist/user.dto.UserResponse.tsgonest.js",
		}

		result := rewriteController(input, "/dist/user.controller.js", controllers, companionMap, "esm", nil)

		// Nested arrays must NOT be serialized — the .map() wrapper can't handle them.
		// The output should be unchanged (no serialize/stringify injection).
		if strings.Contains(result, "serializeUserResponse") || strings.Contains(result, "stringifyUserResponse") {
			t.Fatalf("expected nested array return to be skipped (not serialized), got:\n%s", result)
		}
	})

	t.Run("Bug45_sse_nullable_event_data_wraps_assert_and_stringify", func(t *testing.T) {
		// Pattern: SSE event where data can be null.
		//   yield { event: 'deleted', data: null }  // UserDto | null
		// The SSE interceptor calls assertFn(event.data) then stringifyFn(event.data).
		// Without fix: stringifyUserDto(null) crashes.
		// With fix: nullable entries get wrapper lambdas that check for null first.
		input := `"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const common_1 = require("@nestjs/common");
let EventController = class EventController {
    async *stream() {
        yield { event: "deleted", data: null };
    }
};
__decorate([
    (0, common_1.Get)('stream'),
    __metadata("design:type", Function),
    __metadata("design:returntype", Object)
], EventController.prototype, "stream", null);
exports.EventController = EventController = __decorate([
    (0, common_1.Controller)('events')
], EventController);`

		controllers := []analyzer.ControllerInfo{{
			Name:       "EventController",
			SourceFile: "/src/event.controller.ts",
			Routes: []analyzer.Route{{
				MethodName:    "stream",
				IsEventStream: true,
				SSEEventVariants: []analyzer.SSEEventVariant{
					{
						EventName: "deleted",
						DataType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto", Nullable: true},
					},
					{
						EventName: "created",
						DataType:  metadata.Metadata{Kind: metadata.KindRef, Ref: "UserDto"},
					},
				},
			}},
		}}
		companionMap := map[string]string{
			"UserDto": "/dist/user.dto.UserDto.tsgonest.js",
		}

		result := rewriteController(input, "/dist/event.controller.js", controllers, companionMap, "cjs", nil)

		// The "deleted" event has nullable data → assert and stringify must be wrapped
		if !strings.Contains(result, `_d == null ? _d : assertUserDto(_d)`) {
			t.Fatalf("expected nullable SSE assert wrapper for 'deleted' event, got:\n%s", result)
		}
		if !strings.Contains(result, `_d == null ? "null" : stringifyUserDto(_d)`) {
			t.Fatalf("expected nullable SSE stringify wrapper for 'deleted' event, got:\n%s", result)
		}
		// The "created" event is NOT nullable → should use bare functions
		if !strings.Contains(result, `"created": [assertUserDto, stringifyUserDto]`) {
			t.Fatalf("expected bare assert/stringify for non-nullable 'created' event, got:\n%s", result)
		}
	})

	t.Run("optional_chaining_in_return_is_wrapped", func(t *testing.T) {
		// Optional chaining ?. in the return expression must be preserved and wrapped.
		input := `class C {
    async m() {
        return this.service?.getUser();
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "stringifyUser(await this.service?.getUser())") {
			t.Fatalf("expected optional chaining to be preserved in wrapped return, got:\n%s", got)
		}
	})

	t.Run("super_delegation_in_return_is_wrapped", func(t *testing.T) {
		// super.method() delegation in a return must be wrapped.
		input := `class C {
    async m() {
        return super.findAll();
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "stringifyUser(await super.findAll())") {
			t.Fatalf("expected super delegation to be preserved in wrapped return, got:\n%s", got)
		}
	})

	t.Run("ternary_in_return_is_wrapped", func(t *testing.T) {
		// Ternary expression in return: condition ? branchA : branchB.
		// Since await has higher precedence than ?:, the expression becomes
		// await(condition) ? branchA : branchB. When condition is a resolved
		// boolean (not a Promise), this evaluates correctly.
		input := `class C {
    async m() {
        const user = await this.service.get();
        return user ? user : null;
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "stringifyUser(await user ? user : null)") {
			t.Fatalf("expected ternary in return to be wrapped, got:\n%s", got)
		}
	})

	t.Run("non_async_method_made_async_on_wrap", func(t *testing.T) {
		// When a sync method has its return wrapped, wrapReturnsInMethod must
		// insert async keyword before the method name.
		input := `class C {
    m() {
        return this.service.getUser();
    }
}`
		got := wrapReturnsInMethod(input, "m", "stringifyUser", false)
		if !strings.Contains(got, "async m()") {
			t.Fatalf("expected async to be inserted before method name, got:\n%s", got)
		}
		if !strings.Contains(got, "stringifyUser(await this.service.getUser())") {
			t.Fatalf("expected return to be wrapped with await, got:\n%s", got)
		}
	})

}
