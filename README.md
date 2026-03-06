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

tsgonest replaces `tsc`, `class-validator`, `class-transformer`, and `@nestjs/swagger` with a single build step. Write plain TypeScript types — get validators, fast JSON serializers, and an OpenAPI 3.2 document at compile time.

## Quick start

```bash
npm install tsgonest @tsgonest/runtime @tsgonest/types
```

Define your DTOs:

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

That's it. tsgonest injects validation and serialization into your controllers at compile time. No pipes, no interceptors, no runtime setup.

## Features

- **~10x faster compilation** via [typescript-go](https://github.com/microsoft/typescript-go)
- **Generated validators** — `@Body()`, `@Query()`, `@Param()`, `@Headers()` validated automatically with coercion
- **Fast JSON serializers** — string concatenation with known property shapes, no generic traversal
- **OpenAPI 3.2** — static analysis of NestJS controllers, zero runtime decorators
- **Watch mode** — `tsgonest dev` with auto-restart
- **Migration CLI** — `tsgonest migrate` from class-validator, Nestia, or @nestjs/swagger
- **Standard Schema v1** — wrappers for 60+ framework interop

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

<!-- ## Sponsors

Your logo here — [become a sponsor](https://github.com/sponsors/tsgonest)

<p align="center">
  <em>Sponsorship slots available</em>
</p> -->

## Acknowledgments

- **[typescript-go (tsgo)](https://github.com/microsoft/typescript-go)** — Microsoft's Go port of the TypeScript compiler, used as the compilation engine
- **[typia](https://github.com/samchon/typia)** — Pioneered type-driven validation and serialization in TypeScript; tsgonest's branded types are directly inspired by typia
- **[nestia](https://github.com/samchon/nestia)** — Demonstrated decorator-free NestJS validation and OpenAPI via typia
- **[tsgolint](https://github.com/oxc-project/tsgolint)** — Established the shim pattern for accessing tsgo internals from external Go code

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE.md)
