# ADR-003: Generic Instantiation Naming Strategy

## Status
Accepted

## Context
When a generic type like `PaginatedResponse<UserDto>` is encountered during type walking, it needs a unique, stable, human-readable name for the OpenAPI schema registry.

## Decision
Composite names are built by `buildGenericInstantiationName`:
- Base name (e.g., `PaginatedResponse`) + underscore-separated argument names (e.g., `UserDto`)
- Result: `PaginatedResponse_UserDto`

The `deriveTypeArgName` function resolves argument names in this order:
1. `typeIdToName` cache — reuse previously registered names
2. Array element recursion — for `T[]`, derive from element type
3. Symbol name — from the type's symbol
4. `Type_alias` recovery — for type aliases
5. Literal union members (up to 4) — e.g., `"a" | "b"` → `a_b`

Each step has guards for internal names (`__type`, `__object`, `\xfe` prefix).

Pre-registered type aliases take precedence: if `type ProductResponse = PaginatedResponse<Product>` exists, it registers as `ProductResponse` (flat, all properties materialized). Later encounters of `PaginatedResponse<Product>` reuse this name via the `typeIdToName` cache.

Anonymous type arguments (e.g., `Wrapper<{ x: number }>`) are inlined rather than named. A deduplicated warning is emitted via `warnedGenericNames`.

## Consequences
- Stable, predictable schema names across compilations.
- User-chosen type alias names win over composite names.
- Opaque `T{id}` fallback names are never generated.
- Deeply nested generics produce verbose names (e.g., `Map_String_Array_UserDto`).
