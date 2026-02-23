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

**tsgonest** is a Go CLI that wraps Microsoft's [typescript-go](https://github.com/microsoft/typescript-go) (tsgo) and augments it with everything a NestJS backend needs:

- **10x faster compilation** via tsgo (Go port of the TypeScript compiler)
- **Generated validators** from your TypeScript types — no `class-validator` decorators
- **Fast JSON serializers** (2-5x faster than `JSON.stringify`) — no `class-transformer`
- **OpenAPI 3.2** from static analysis of NestJS controllers — no `@nestjs/swagger` decorators
- **Watch mode** with auto-restart — replaces `nest start --watch`
- **Standard Schema v1** wrappers for 60+ framework interop

Write plain TypeScript types. tsgonest generates everything else at build time.

## Quick start

```bash
npm install tsgonest
```

Create `tsgonest.config.json`:

```json
{
  "controllers": {
    "include": ["src/**/*.controller.ts"]
  },
  "transforms": {
    "validation": true,
    "serialization": true
  },
  "openapi": {
    "output": "dist/openapi.json"
  }
}
```

Define your DTOs with type-safe constraints:

```ts
// src/user/user.dto.ts
import { Min, Max, Email, Trim } from '@tsgonest/types';

export interface CreateUserDto {
  name: string & Trim & Min<1> & Max<255>;
  email: string & Email;
  age: number & Min<0> & Max<150>;
}

export interface UserResponse {
  id: string;
  name: string;
  email: string;
  age: number;
  createdAt: string;
}
```

Build:

```bash
npx tsgonest build
```

Wire into NestJS:

```ts
// src/main.ts
import { NestFactory } from '@nestjs/core';
import { TsgonestValidationPipe, TsgonestFastInterceptor } from '@tsgonest/runtime';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  app.useGlobalPipes(new TsgonestValidationPipe({ distDir: 'dist' }));
  app.useGlobalInterceptors(new TsgonestFastInterceptor({ distDir: 'dist' }));
  await app.listen(3000);
}
bootstrap();
```

That's it. Request bodies are validated, responses are fast-serialized, and `dist/openapi.json` contains your complete API documentation.

## What it replaces

| Concern | Traditional NestJS | tsgonest |
|---|---|---|
| Compilation | `tsc` / `nest build` | `tsgonest build` (tsgo) |
| Watch mode | `nest start --watch` | `tsgonest dev` |
| Validation | `class-validator` + decorators | Generated from types |
| Serialization | `class-transformer` + `ClassSerializerInterceptor` | Generated fast serializers |
| OpenAPI | `@nestjs/swagger` + decorators on every property | Static analysis, zero decorators |

## Two ways to define constraints

**Branded types** (type-safe, with autocomplete):

```ts
import { Min, Max, Email, Trim, Coerce } from '@tsgonest/types';

interface CreateUserDto {
  name: string & Trim & Min<1> & Max<255>;
  email: string & Email;
  age: number & Min<0> & Max<150>;
}

interface PaginationQuery {
  page: number & Coerce & Min<1>;
  limit: number & Coerce & Max<100>;
}
```

**JSDoc tags** (zero dependencies):

```ts
interface CreateUserDto {
  /** @minLength 1 @maxLength 255 @transform trim */
  name: string;

  /** @format email */
  email: string;

  /** @minimum 0 @maximum 150 */
  age: number;
}
```

Both approaches generate identical companion code.

## Output

```
dist/
  user.dto.js                             # tsgo output
  user.dto.CreateUserDto.tsgonest.js      # validate + assert + serialize + schema
  user.dto.CreateUserDto.tsgonest.d.ts    # companion type declarations
  user.dto.UserResponse.tsgonest.js
  user.dto.UserResponse.tsgonest.d.ts
  user.controller.js                      # tsgo output (controllers are skipped)
  __tsgonest_manifest.json                # runtime discovery manifest
  openapi.json                            # OpenAPI 3.2 document
```

Each companion file exports:

```ts
export function validateCreateUserDto(input);  // returns { success, data?, errors? }
export function assertCreateUserDto(input);    // throws on failure
export function serializeCreateUserDto(input); // fast JSON string
export function schemaCreateUserDto();         // Standard Schema v1
```

## CLI

```bash
# Production build
tsgonest build

# Watch mode with auto-restart
tsgonest dev

# Custom tsconfig
tsgonest build --project tsconfig.build.json

# Clean build
tsgonest build --clean

# Skip type checking
tsgonest build --no-check

# Dev with debugger
tsgonest dev --debug

# Dev with env file
tsgonest dev --env-file .env
```

## Packages

| Package | Description |
|---|---|
| [`tsgonest`](https://www.npmjs.com/package/tsgonest) | CLI binary (auto-installs correct platform binary) |
| [`@tsgonest/runtime`](https://www.npmjs.com/package/@tsgonest/runtime) | `TsgonestValidationPipe`, `TsgonestFastInterceptor`, `CompanionDiscovery` |
| [`@tsgonest/types`](https://www.npmjs.com/package/@tsgonest/types) | Zero-runtime branded phantom types (`Email`, `Min`, `Max`, `Trim`, ...) |

## Configuration

```json
{
  "controllers": {
    "include": ["src/**/*.controller.ts"],
    "exclude": ["src/**/*.spec.ts"]
  },
  "transforms": {
    "validation": true,
    "serialization": true,
    "exclude": ["LegacyDto"]
  },
  "openapi": {
    "output": "dist/openapi.json",
    "title": "My API",
    "version": "1.0.0",
    "securitySchemes": {
      "bearer": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    }
  },
  "nestjs": {
    "globalPrefix": "api",
    "versioning": {
      "type": "URI",
      "defaultVersion": "1",
      "prefix": "v"
    }
  }
}
```

## Supported constraints

### String

`Format` (`Email`, `Uuid`, `Url`, `IPv4`, `IPv6`, `DateTime`, `Jwt`, `Ulid`, `Cuid`, `NanoId`, + 20 more), `MinLength`, `MaxLength`, `Pattern`, `StartsWith`, `EndsWith`, `Includes`, `Uppercase`, `Lowercase`

### Number

`Minimum` / `Min`, `Maximum` / `Max`, `ExclusiveMinimum` / `Gt`, `ExclusiveMaximum` / `Lt`, `MultipleOf` / `Step`, `Type` (`Int`, `Uint`, `SafeInt`, `Finite`, `Double`), `Positive`, `Negative`, `NonNegative`, `NonPositive`

### Array

`MinItems`, `MaxItems`, `UniqueItems` / `Unique`

### Transforms & coercion

`Trim`, `ToLowerCase`, `ToUpperCase`, `Coerce`

### Meta

`Default<V>`, `Error<M>`, `Validate<typeof fn>`, `Length<N>`, `Range<R>`, `Between<R>`

## Real-world performance

Tested on a production NestJS project (120 controllers, 838 routes, 2,376 companions):

| Metric | Result |
|---|---|
| Cold build | ~19s |
| Warm build (cached) | ~1.1s |
| Go unit tests | 546+ passing |
| E2E tests | 97 passing |

## Platform support

| Platform | Architecture | Status |
|---|---|---|
| macOS | ARM64 (Apple Silicon) | Supported |
| macOS | x64 (Intel) | Supported |
| Linux | x64 | Supported |
| Linux | ARM64 | Supported |
| Windows | x64 | Supported |

## Documentation

Full documentation is available at [tsgonest.dev](https://tsgonest.dev) or in `apps/docs/`.

- [Getting Started](https://tsgonest.dev/docs/getting-started)
- [CLI Reference](https://tsgonest.dev/docs/cli)
- [Configuration](https://tsgonest.dev/docs/config)
- [Validation](https://tsgonest.dev/docs/validation)
- [Serialization & Runtime](https://tsgonest.dev/docs/serialization-runtime)
- [OpenAPI Generation](https://tsgonest.dev/docs/openapi)
- [Type Tags Reference](https://tsgonest.dev/docs/type-tags)

### Comparisons

- [tsgonest vs NestJS CLI](https://tsgonest.dev/docs/comparisons/vs-nestjs-cli)
- [tsgonest vs Nestia + Typia](https://tsgonest.dev/docs/comparisons/vs-nestia-typia)
- [tsgonest vs tsgo](https://tsgonest.dev/docs/comparisons/vs-tsgo)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and submission guidelines.

## Security

See [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

## License

[MIT](LICENSE.md)
