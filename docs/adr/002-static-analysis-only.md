# ADR-002: Static Analysis Only — No Runtime Reflection

## Status
Accepted

## Context
NestJS's built-in OpenAPI module (`@nestjs/swagger`) uses `reflect-metadata` at runtime to extract type information from decorators. This requires running the application and adds a runtime dependency.

tsgonest aims to extract type information at compile time using TypeScript's type checker, with zero runtime overhead for schema generation.

## Decision
tsgonest uses static analysis exclusively for all type extraction:
- The tsgo type checker resolves types via `checker.getTypeAtLocation()` and related APIs.
- NestJS decorators (`@Controller`, `@Get`, `@Body`, etc.) are recognized by their symbol origin, not by runtime behavior.
- Import aliases (`import { Body as NestBody }`) are supported via checker symbol resolution.
- Factory decorators that wrap core NestJS decorators are intentionally unsupported — static analysis cannot see through factory functions.
- Dynamic decorator paths (e.g., `@Controller(prefix)` where `prefix` is a variable) are unsupported and emit a warning.

## Consequences
- No `reflect-metadata` dependency needed.
- OpenAPI generation happens at compile time — no need to boot the application.
- Runtime-generated controllers and dynamic decorator paths are excluded.
- Type information is limited to what the TypeScript type checker can resolve statically.
- Some patterns that work with `@nestjs/swagger` (e.g., runtime-computed paths) will not work with tsgonest.
