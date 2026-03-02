# ADR-004: Regex-Based JS Rewriting (No AST)

## Status
Accepted

## Context
tsgonest injects validation calls, serialization wrapping, and interceptor decorators into tsgo's compiled JavaScript output. This requires modifying JS source text during the emit phase.

Two approaches were considered:
1. **AST-based**: Parse the JS into an AST, transform nodes, and re-emit. Precise but requires a full JS parser in Go.
2. **Regex/text-based**: Use regex patterns and character-by-character scanning to locate and modify specific code patterns. Simpler but more fragile.

## Decision
tsgonest uses regex and character-by-character scanning for all JS rewriting:
- Method signatures are located via regex patterns matching `methodName(...) {`.
- Return statement wrapping uses a character-by-character scanner that tracks brace depth, string literals, and comments.
- Decorator injection uses regex to find `__decorate([...], ClassName.prototype, ...)` patterns.
- Class-level interceptor injection finds `ClassName = __decorate([` patterns.

## Rationale
- tsgo's emitted JS follows a predictable pattern — the output format is controlled, not arbitrary user code.
- A full JS parser in Go (or CGo binding) would add significant complexity and build-time dependency.
- The regex approach is fast — no parsing overhead, operates on the string directly.
- The approach is battle-tested through extensive e2e tests (100+ test cases).

## Consequences
- Changes to tsgo's output format may break rewrite patterns.
- Complex patterns (nested functions, template literals with `${...}`) require careful character-by-character handling.
- Duplicated scanning logic exists between `wrapReturnsInBody` and `wrapPrimitiveReturnsInBody` — this is a known maintainability debt.
- Any future simplification should extract shared scanning into a common helper, not replace with a full AST approach.
