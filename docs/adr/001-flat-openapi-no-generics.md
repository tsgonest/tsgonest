# ADR-001: Flat OpenAPI Schemas — No Generics

## Status
Accepted

## Context
TypeScript generics like `PaginatedResponse<UserDto>` produce concrete types in compiled output. OpenAPI 3.x has no concept of generics — there is no way to express `PaginatedResponse<T>` as a parameterized schema.

Some projects add `x-` vendor extensions to reconstruct generic relationships in SDK generators. This creates tooling lock-in and breaks compatibility with standard OpenAPI tools.

## Decision
tsgonest generates flat, fully-materialized schemas for every generic instantiation. `PaginatedResponse<UserDto>` becomes a schema named `PaginatedResponse_UserDto` with all properties expanded inline.

- No `x-` extensions for generic type parameters.
- Composite names are built by `buildGenericInstantiationName` using base name + `deriveTypeArgName` per argument.
- Pre-registered type aliases (e.g., `type ProductResponse = PaginatedResponse<Product>`) use the user's chosen name instead of the composite name.
- Anonymous type arguments are inlined, never given opaque `T{id}` fallback names.

## Consequences
- All standard OpenAPI tooling (Swagger UI, Redocly, third-party generators) works without modification.
- Schema names may be verbose for deeply nested generics.
- SDK generators cannot reconstruct the original generic type structure — this is intentional.
- Adding generic support later would be a breaking change to schema naming.
