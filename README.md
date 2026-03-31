<p align="center">
  <img src="apps/docs/public/logo-mark.svg" alt="tsgonest" width="80" height="80" />
</p>

<h1 align="center">tsgonest</h1>

<p align="center">
  <strong>Drop-in NestJS compiler. Validates, serializes, and documents your API — from plain TypeScript types.</strong>
</p>

<p align="center">
  <a href="https://github.com/tsgonest/tsgonest/releases"><img src="https://img.shields.io/github/v/release/tsgonest/tsgonest?style=flat-square" alt="Release" /></a>
  <a href="https://github.com/tsgonest/tsgonest/actions"><img src="https://img.shields.io/github/actions/workflow/status/tsgonest/tsgonest/ci.yml?style=flat-square&label=CI" alt="CI" /></a>
  <a href="https://github.com/tsgonest/tsgonest/blob/main/LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" /></a>
  <a href="https://www.npmjs.com/package/tsgonest"><img src="https://img.shields.io/npm/v/tsgonest?style=flat-square" alt="npm" /></a>
</p>

---

tsgonest replaces `tsc`, `class-validator`, `class-transformer`, and `@nestjs/swagger` with a single build step. Write your types, run `tsgonest build`, and get validation, fast JSON serialization, and OpenAPI 3.2 — all generated at compile time.

```ts
export interface CreateUserDto {
  name: string & tags.Trim & tags.Min<1> & tags.Max<255>;
  email: string & tags.Email;
  age: number & tags.Min<0> & tags.Max<150>;
}

@Controller('user')
export class UserController {
  @Post()
  createUser(@Body() body: CreateUserDto) {
    // body is validated at compile time — no pipes, no interceptors
  }
}
```

```bash
npx tsgonest build
```

## Why

- **~10x faster compilation** via [typescript-go](https://github.com/microsoft/typescript-go)
- **Zero runtime overhead** — validation and serialization are injected at compile time
- **Zero decorators** — OpenAPI generated from static analysis, not `@ApiProperty()`
- **Drop-in** — works with existing NestJS projects, no `ts-patch` or plugins

## Install

```bash
# Migrate from class-validator / nestia (recommended)
npx tsgonest@latest migrate

# Or install manually
pnpm install tsgonest @tsgonest/runtime @tsgonest/types
```

## CLI

```bash
tsgonest build                  # production build
tsgonest dev                    # watch + auto-restart
tsgonest check                  # type check (no emit)
tsgonest check --watch          # continuous checking
tsgonest openapi                # generate OpenAPI only
tsgonest openapi --name public  # specific output
tsgonest sdk                    # generate typed client SDK
tsgonest migrate                # codemod migration
```

## Constraints

Annotate with branded types (`@tsgonest/types`) or JSDoc — both can be mixed.

```ts
import { tags } from '@tsgonest/types';

export interface CreateUserDto {
  name: string & tags.Trim & tags.MinLength<1> & tags.MaxLength<255>;
  email: string & tags.Email & tags.Error<'Please provide a valid email'>;
  age: number & tags.Min<0> & tags.Max<150> & tags.Type<'int32'>;
  score?: number & tags.Default<0>;
  tags: string[] & tags.MinItems<1> & tags.UniqueItems;
}
```

32 built-in formats (`email`, `uuid`, `url`, `ipv4`, `date-time`, `jwt`, ...), numeric types (`int32`, `uint32`, `float`, ...), custom validators via `tags.Validate<typeof fn>`, and custom error messages via `tags.Error<"msg">`.

[Full constraint reference](https://tsgonest.dev)

## Packages

| Package | Description |
| --- | --- |
| [`tsgonest`](https://www.npmjs.com/package/tsgonest) | CLI + compiler |
| [`@tsgonest/runtime`](https://www.npmjs.com/package/@tsgonest/runtime) | `defineConfig`, `TsgonestValidationError`, `Returns`, `FormDataBody`, `EventStream` |
| [`@tsgonest/types`](https://www.npmjs.com/package/@tsgonest/types) | Branded phantom types — `tags.Email`, `tags.Min`, `tags.Trim`, etc. |

## Platform support

macOS (ARM64, x64) / Linux (x64, ARM64 — static, works on glibc + musl) / Windows (x64, ARM64)

## Sponsors

<h3 align="center">Gold Sponsor</h3>

<p align="center">
  <a href="https://tixio.io?ref=tsgonest" target="_blank">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://app.tixio.io/images/tixioLogo.png">
      <img alt="Tixio" src="https://app.tixio.io/images/tixioLogo.png" width="50">
    </picture>
  </a>
</p>

<p align="center">
  <a href="https://tixio.io?ref=tsgonest" target="_blank"><strong>Tixio</strong></a><br>
  <sub>Stop wasting time and money on slack jira clickup and 10+ other tools, get tixio instead. use code `tsgonest` for 20% discount</sub>
</p>

<br>

<p align="center">
  <sub>
    <a href="https://github.com/sponsors/shahriar-shojib">Become a sponsor</a>
  </sub>
</p>

## Documentation

[tsgonest.dev](https://tsgonest.dev)

## Acknowledgments

- [typescript-go](https://github.com/microsoft/typescript-go) — Microsoft's Go port of TypeScript
- [typia](https://github.com/samchon/typia) — Pioneered type-driven validation; tsgonest's branded types are inspired by typia
- [nestia](https://github.com/samchon/nestia) — Decorator-free NestJS validation and OpenAPI via typia
- [tsgolint](https://github.com/nicolo-ribaudo/tsgolint) — Established the shim pattern for accessing tsgo internals

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE.md)
