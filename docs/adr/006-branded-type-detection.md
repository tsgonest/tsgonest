# ADR-006: Branded Type Detection for Constraints

## Status
Accepted

## Context
tsgonest supports two ways to annotate types with validation constraints:
1. **JSDoc tags**: `/** @minimum 0 @maximum 100 */` — zero dependencies, works everywhere.
2. **Branded phantom types**: `string & tags.Format<"email">` — type-safe, with IDE autocomplete.

Both approaches need to produce the same `metadata.Constraints` output for codegen and OpenAPI.

## Decision
Branded types use phantom properties with the `__tsgonest_` prefix:
- `__tsgonest_format?: "email"` → `Format: "email"`
- `__tsgonest_minLength?: 1` → `MinLength: 1`
- `__tsgonest_transform_trim?: true` → `Transforms: ["trim"]`
- `__tsgonest_validate?: typeof fn` → custom validator

Per-constraint error messages use the `_error` suffix:
- `__tsgonest_format_error?: "Invalid email"` → `Errors["format"]: "Invalid email"`

For typia migration compatibility, tsgonest also detects `"typia.tag"` properties with `{target, kind, value}` structure and maps them to the same constraint system.

The `@tsgonest/types` package provides zero-runtime branded types (`tags.Email`, `tags.MinLength<N>`, etc.) that produce these phantom properties.

## Consequences
- Both JSDoc and branded types produce identical `metadata.Constraints` — codegen and OpenAPI are agnostic to the source.
- The `__tsgonest_` prefix is reserved and must not collide with user property names.
- typia users can migrate incrementally — their existing branded types work out of the box.
- Phantom properties are stripped during type walking and never appear in generated schemas.
