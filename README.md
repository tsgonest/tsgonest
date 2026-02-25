<p align="center">
  <img src="apps/docs/public/logo-mark.svg" alt="tsgonest" width="80" height="80" />
</p>

<h1 align="center">tsgonest</h1>

<p align="center">
  Native-speed TypeScript compilation with generated validation, serialization, and OpenAPI for NestJS.
</p>

<p align="center">
  <a href="https://github.com/tsgonest/tsgonest/releases"><img src="https://img.shields.io/github/v/release/tsgonest/tsgonest?style=flat-square" alt="Release" /></a>
  <a href="https://github.com/tsgonest/tsgonest/actions"><img src="https://img.shields.io/github/actions/workflow/status/tsgonest/tsgonest/ci.yml?style=flat-square&label=CI" alt="CI" /></a>
  <a href="https://github.com/tsgonest/tsgonest/blob/main/LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" /></a>
  <a href="https://www.npmjs.com/package/tsgonest"><img src="https://img.shields.io/npm/v/tsgonest?style=flat-square" alt="npm" /></a>
</p>

---

A Go CLI wrapping [typescript-go](https://github.com/microsoft/typescript-go) (tsgo) that replaces `tsc`, `class-validator`, `class-transformer`, and `@nestjs/swagger` with a single build step. Write plain TypeScript types — tsgonest generates validators, fast JSON serializers, and an OpenAPI 3.2 document at compile time.

## Features

- **Fast compilation** via tsgo (Go port of the TypeScript compiler)
- **Generated validators** from TypeScript types — `@Body()`, `@Query()`, `@Param()`, and `@Headers()` validated at compile time with auto-coercion
- **Fast JSON serializers** via string concatenation with known property shapes — no generic object traversal
- **OpenAPI 3.2** from static analysis of NestJS controllers — zero runtime decorators
- **Watch mode** with auto-restart (`tsgonest dev`)
- **Standard Schema v1** wrappers for framework interop

## Quick start

```bash
npm install tsgonest @tsgonest/runtime @tsgonest/types
```

Define DTOs with branded types or JSDoc:

```ts
import { tags } from '@tsgonest/types';

export interface CreateUserDto {
  name: string & tags.Trim & tags.Min<1> & tags.Max<255>;
  email: string & tags.Email;
  age: number & tags.Min<0> & tags.Max<150>;
}
```

Build:

```bash
npx tsgonest build
```

That's it — zero runtime setup needed. tsgonest injects validation and serialization into your controllers at compile time. Request bodies are validated, responses are fast-serialized, and `dist/openapi.json` contains your API docs.

## Static analysis limits

tsgonest intentionally stays compile-time only. A few dynamic NestJS patterns are not supported for OpenAPI extraction:

- Controllers declared inside factory functions (non-top-level `@Controller` classes)
- Dynamic path arguments in `@Controller(...)`, `@Get(...)`, `@Post(...)`, `@Sse(...)`, etc. (for example, `@Controller(prefix)` or `@Get(dynamicPath)`)

When these patterns are detected, tsgonest prints a warning and excludes those controllers/routes from the generated OpenAPI document.

## CLI

```bash
tsgonest build                        # production build
tsgonest dev                          # watch + auto-restart
tsgonest migrate                      # migrate from class-validator/nestia
tsgonest build -p tsconfig.build.json # custom tsconfig
tsgonest build --clean                # clean output before build
tsgonest build --no-check             # skip type checking
tsgonest dev --debug                  # Node.js --inspect
tsgonest dev --env-file .env          # load env file
```

## Packages

| Package | Description |
|---|---|
| [`tsgonest`](https://www.npmjs.com/package/tsgonest) | CLI (auto-installs platform binary) |
| [`@tsgonest/runtime`](https://www.npmjs.com/package/@tsgonest/runtime) | `defineConfig`, `TsgonestValidationError`, `FormDataBody`, `FormDataInterceptor` |
| [`@tsgonest/types`](https://www.npmjs.com/package/@tsgonest/types) | Zero-runtime branded phantom types (`tags.Email`, `tags.Min`, `tags.Trim`, ...) |

## Platform support

macOS (ARM64, x64), Linux (x64, ARM64 — static binaries, glibc + musl), Windows (x64, ARM64).

## Documentation

[tsgonest.dev](https://tsgonest.dev)

## Acknowledgments

tsgonest builds on and is inspired by several projects:

- **[typescript-go (tsgo)](https://github.com/microsoft/typescript-go)** — Microsoft's Go port of the TypeScript compiler, used as the compilation engine
- **[typia](https://github.com/samchon/typia)** — Pioneered type-driven validation and serialization in TypeScript; tsgonest's branded type system and constraint tags are directly inspired by typia, and tsgonest recognizes `typia.tag` properties for migration
- **[nestia](https://github.com/samchon/nestia)** — Demonstrated decorator-free NestJS validation and OpenAPI via typia; tsgonest pursues the same philosophy with a native-speed single-binary approach
- **[tsgolint](https://github.com/oxc-project/tsgolint)** — Established the `go:linkname` shim pattern for accessing tsgo internals from external Go code, which tsgonest adopts
- **[Prisma](https://www.prisma.io/)** — Documentation site design inspiration (via [Fumadocs](https://fumadocs.vercel.app/) + Next.js)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE.md)
