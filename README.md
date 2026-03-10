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

tsgonest replaces **five tools** with a single build step:

| You drop                                               | tsgonest handles it                                                                                                    |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| `tsc` / `nest build`                                   | Compiles via [typescript-go](https://github.com/microsoft/typescript-go) (~10x faster) with path alias transformations |
| `class-validator` + decorators on every property       | Validates `@Body()`, `@Query()`, `@Param()`, `@Headers()` at compile time                                              |
| `class-transformer`                                    | Generates fast JSON serializers (template literals, not `JSON.stringify`)                                              |
| `@nestjs/swagger` + `@ApiProperty()` on every property | Produces OpenAPI 3.2 from static analysis — zero decorators                                                            |
| `nest start --watch`                                   | `tsgonest dev` with auto-restart                                                                                       |

Write your types. Build. Everything else is generated.

```ts
// dto/create-user.dto.ts
import { tags } from '@tsgonest/types';

export interface CreateUserDto {
  name: string & tags.Trim & tags.Min<1> & tags.Max<255>;
  email: string & tags.Email;
  age: number & tags.Min<0> & tags.Max<150>;
}

@Controller('user')
export class UserController {
  @Post()
  createUser(@Body() body: CreateUserDto) {
    //body is fully validated here
  }
}
```

```bash
npx tsgonest build
```

No pipes. No interceptors. No runtime setup. Validation and serialization are injected into your controllers at compile time.

## Install

### Migrate (Recommended)

Migrate command migrates your controllers and dtos with a codemod if you currently use nestia/typia and class validator and class transformer and cleans up unnecessary dependencies and adds the relevant ones.

```
npx tsgonest@latest migrate
```

### Manual installation

```bash
pnpm install tsgonest @tsgonest/runtime @tsgonest/types
```

## What it generates

For each DTO, tsgonest emits a companion file with five functions:

| Function                 | What it does                                                               |
| ------------------------ | -------------------------------------------------------------------------- |
| `is<Type>(input)`        | Boolean type guard. Zero allocations.                                      |
| `validate<Type>(input)`  | Returns `{ success, data, errors }` with path-level error details.         |
| `assert<Type>(input)`    | Throws `TsgonestValidationError` (400) on first error.                     |
| `serialize<Type>(input)` | Fast JSON via template literals — skips `JSON.stringify` for known shapes. |
| `stringify<Type>(input)` | Validate + serialize in one call.                                          |

Controllers are rewritten at compile time: `@Body()` params get `assert()` injected, return values get `stringify()` wrapped. You don't call these functions — they're wired in for you.

## What it supports

### NestJS decorators

`@Controller`, `@Get`, `@Post`, `@Put`, `@Delete`, `@Patch`, `@Head`, `@Options`, `@All`, `@Body`, `@Query`, `@Param`, `@Headers`, `@Res`, `@HttpCode`, `@Version`, `@UseInterceptors`

### Type system

Interfaces, type aliases, classes, enums (string + numeric), unions, discriminated unions, intersections, tuples, arrays, `Date`, `Map`, `Set`, typed arrays, template literal types, recursive types, generic instantiations, index signatures, nullable/optional properties.

### Constraints — two ways to annotate

Use branded types (`@tsgonest/types`) for autocomplete, or JSDoc tags for zero dependencies. Both can be mixed.

```ts
import { tags } from '@tsgonest/types';

// custom error messages on any constraint
type Email = string & tags.Email & tags.Error<'Invalid email address'>;

// custom validation function — resolved at compile time
const isEven = (n: number) => n % 2 === 0;
type EvenNumber = number &
  tags.Validate<typeof isEven> &
  tags.Error<'Must be even'>;

// compose constraints freely
export interface CreateUserDto {
  name: string & tags.Trim & tags.MinLength<1> & tags.MaxLength<255>;
  email: string & tags.Email & tags.Error<'Please provide a valid email'>;
  age: number & tags.Min<0> & tags.Max<150> & tags.Type<'int32'>;
  score?: number & tags.Default<0> & tags.Min<0>;
  tags: string[] & tags.MinItems<1> & tags.UniqueItems;
}
```

| JSDoc              | Branded type               | Effect                                                                                                            |
| ------------------ | -------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `@format email`    | `tags.Email`               | 32 built-in formats: `email`, `uuid`, `url`, `ipv4`, `ipv6`, `date-time`, `jwt`, [and more](https://tsgonest.dev) |
| `@minimum N`       | `tags.Min<N>`              | `>= N`                                                                                                            |
| `@maximum N`       | `tags.Max<N>`              | `<= N`                                                                                                            |
| `@minLength N`     | `tags.MinLength<N>`        | Min string length                                                                                                 |
| `@maxLength N`     | `tags.MaxLength<N>`        | Max string length                                                                                                 |
| `@pattern "regex"` | `tags.Pattern<"regex">`    | Regex match                                                                                                       |
| `@type int32`      | `tags.Type<'int32'>`       | Numeric type: `int32`, `uint32`, `int64`, `uint64`, `float`, `double`                                             |
| `@minItems N`      | `tags.MinItems<N>`         | Min array length                                                                                                  |
| `@uniqueItems`     | `tags.UniqueItems`         | No duplicate elements                                                                                             |
| `@trim`            | `tags.Trim`                | Trim whitespace before validation                                                                                 |
| `@default value`   | `tags.Default<V>`          | Default for optional properties                                                                                   |
| `@coerce`          | `tags.Coerce`              | Coerce string to target type                                                                                      |
| `@error "msg"`     | `tags.Error<"msg">`        | Custom error message                                                                                              |
| —                  | `tags.Validate<typeof fn>` | Custom validation function                                                                                        |
| `@strict`          | —                          | Reject unknown properties                                                                                         |

[Full constraint reference →](https://tsgonest.dev)

### Query/Param coercion

`@Query()` and `@Param()` values arrive as strings. tsgonest auto-coerces them to `number`, `boolean`, or `string[]` based on your type annotations. No `ParseIntPipe` needed.

### OpenAPI 3.2

Generated from static analysis of your controllers. No `@ApiProperty()`, no `reflect-metadata`.

JSDoc on controller methods: `@summary`, `@description`, `@deprecated`, `@hidden`, `@tag`, `@security`, `@public`, `@throws {status} Type - description`, `@operationid`, `@contenttype`, `@extension x-key value`.

Generic types become flat schemas (`PaginatedResponse<UserDto>` → `PaginatedResponse_UserDto`). This is intentional — OpenAPI has no generics, and flat schemas work with every code generator.

### SSE (Server-Sent Events)

```ts
import { EventStream, SseEvents } from '@tsgonest/runtime';

@EventStream('/events')
async *streamEvents(): AsyncGenerator<SseEvents<{ data: UserDto; ping: void }>> {
  yield { event: 'data', data: user };
}
```

Type-safe event streams with per-event validation and serialization.

### File uploads

```ts
import { FormDataBody, FormDataInterceptor } from '@tsgonest/runtime';

@Post('upload')
@UseInterceptors(FormDataInterceptor)
create(@FormDataBody(() => multer()) body: CreateDto) {}
```

### SDK generation

```bash
tsgonest sdk --input openapi.json --output ./sdk
```

Generates typed TypeScript client from your OpenAPI spec.

### Marker functions

Use tsgonest functions directly outside controllers:

```ts
import { validate, assert, is, stringify, serialize } from 'tsgonest';

const result = validate<CreateUserDto>(input);
const safe = is<CreateUserDto>(input);
const json = stringify<CreateUserDto>(data);
```

These are rewritten to companion imports at compile time.

## What it does NOT support

- **Factory decorators** — functions that return `@Body()`, `@Get()`, etc. are opaque to static analysis
- **Runtime-generated controllers** — `@Controller(variable)` or dynamically registered controllers are skipped (with a warning)
- **Dynamic route paths** — `@Get(variable)` requires a string literal

## CLI

````bash
tsgonest build                           # production build
tsgonest dev                             # watch + auto-restart
tsgonest migrate                         # migrate from class-validator / nestia / @nestjs/swagger
tsgonest sdk                             # generate typed client SDK from OpenAPI

# Build options
tsgonest build -p tsconfig.build.json    # custom tsconfig
tsgonest build --config tsgonest.config.ts    # custom tsgonest config
tsgonest build --clean                   # clean output before build


Type `rs` in dev mode to manually restart.

## Configuration

Create `tsgonest.config.ts` (or `tsgonest.config.json`) at project root.

```ts
// tsgonest.config.ts
import { defineConfig } from '@tsgonest/runtime';

export default defineConfig({
  // Which controllers to analyze for validation injection + OpenAPI
  controllers: {
    include: ['src/**/*.controller.ts'], // default
    exclude: [], // glob patterns to skip
  },

  // Code generation transforms
  transforms: {
    validation: true, // inject @Body/@Query/@Param/@Headers validation (default: true)
    serialization: true, // inject return value serialization (default: true)
    responseSerializer: 'guard', // "guard" (default) | "safe" | "none"
    standardSchema: false, // generate Standard Schema v1 wrappers (default: false)
    include: [], // glob patterns for companion generation (e.g., ["src/**/*.dto.ts"])
    exclude: [], // type name patterns to skip (e.g., ["Legacy*"])
  },

  // OpenAPI 3.2 document generation
  openapi: {
    output: 'dist/openapi.json', // default — set to "" to disable
    title: '',
    description: '',
    version: '',
    termsOfService: '',
    contact: { name: '', url: '', email: '' },
    license: { name: '', url: '' },
    servers: [{ url: 'http://localhost:3000', description: '' }],
    tags: [{ name: 'users', description: 'User operations' }],
    securitySchemes: {
      bearer: { type: 'http', scheme: 'bearer', bearerFormat: 'JWT' },
    },
    security: [{ bearer: [] }], // global security — routes with @public opt out
  },

  // TypeScript SDK generation from OpenAPI
  sdk: {
    output: './sdk', // output directory
    input: '', // defaults to openapi.output
  },

  // NestJS-specific settings
  nestjs: {
    globalPrefix: '', // e.g., "api"
    versioning: {
      type: 'URI', // "URI" (default) | "HEADER" | "MEDIA_TYPE" | "CUSTOM"
      defaultVersion: '', // e.g., "1"
      prefix: 'v', // URI version prefix (default: "v")
    },
  },

  // Dev/build settings
  entryFile: 'main', // entry point name without extension (default: "main")
  sourceRoot: 'src', // source root directory (default: "src")
  deleteOutDir: false, // delete output dir before build (default: false)
  manualRestart: false, // enable "rs" restart in dev mode (default: false)
});
````

## Packages

| Package                                                                | Description                                                                         |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [`tsgonest`](https://www.npmjs.com/package/tsgonest)                   | CLI — auto-installs the right binary for your platform                              |
| [`@tsgonest/runtime`](https://www.npmjs.com/package/@tsgonest/runtime) | `defineConfig`, `TsgonestValidationError`, `Returns`, `FormDataBody`, `EventStream` |
| [`@tsgonest/types`](https://www.npmjs.com/package/@tsgonest/types)     | Branded phantom types — `tags.Email`, `tags.Min`, `tags.Trim`, and more             |

## Platform support

| OS      | Architectures                                      |
| ------- | -------------------------------------------------- |
| macOS   | ARM64, x64                                         |
| Linux   | x64, ARM64 (static binaries — glibc + musl/Alpine) |
| Windows | x64, ARM64                                         |

## How it compares

|                    | tsgonest                          | typia + nestia                | class-validator + @nestjs/swagger | zod                    |
| ------------------ | --------------------------------- | ----------------------------- | --------------------------------- | ---------------------- |
| Source of truth    | TS types                          | TS types                      | Decorators on classes             | Zod schemas            |
| Compilation speed  | ~10x (Go-native)                  | 1x (tsc plugin)               | 1x (tsc)                          | N/A                    |
| Requires ts-patch  | No                                | Yes                           | No                                | No                     |
| Runtime validation | Generated at compile time         | Generated at compile time     | Interpreted at runtime            | Interpreted at runtime |
| JSON serialization | Generated (template literals)     | Generated (template literals) | Manual                            | Manual                 |
| OpenAPI generation | Static analysis                   | SDK tool                      | `@ApiProperty()` decorators       | Separate library       |
| CLI replacement    | `tsgonest dev` / `tsgonest build` | No                            | No                                | No                     |
| Setup              | `npm install` + build             | ts-patch + ttypescript config | Decorators on every field         | Schema per type        |

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

- **[typescript-go](https://github.com/microsoft/typescript-go)** — Microsoft's Go port of TypeScript, used as the compilation engine
- **[typia](https://github.com/samchon/typia)** — Pioneered type-driven validation and serialization; tsgonest's branded types are directly inspired by typia
- **[nestia](https://github.com/samchon/nestia)** — Demonstrated decorator-free NestJS validation and OpenAPI via typia
- **[tsgolint](https://github.com/nicolo-ribaudo/tsgolint)** — Established the shim pattern for accessing tsgo internals

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE.md)
