# tsgonest Development Progress

## v0.4.0 (preparing)

- Phase 5: `tsgonest migrate` command (AST-based codemods)
- Phase 6: FormDataBody decorator + JSONC parser fix
- SDK generation, controller rewriting, flag passthrough, tsdown migration

## Phase 6: FormDataBody + JSONC Parser Fix — COMPLETE

- **`@FormDataBody()` decorator** in `@tsgonest/runtime` — drop-in replacement for Nestia's `@TypedFormData.Body()`
- **`FormDataInterceptor`** — NestJS interceptor that runs multer and merges files into req.body
- **JSONC parser fix** — replaced hand-rolled `stripJsoncComments()` with `jsonc-parser` library; fixes glob patterns in tsconfig paths (e.g., `@db/client/*`)
- **Migrate transform updated** — `@TypedFormData.Body(() => factory())` → `@FormDataBody(() => factory())` + `@UseInterceptors(FormDataInterceptor)`, no more manual TODOs
- **Go analyzer updated** — recognizes `@FormDataBody` decorator, sets `ContentType = "multipart/form-data"`, skips validation injection for form-data params
- **Tested on ecom-bot** — 11 TypedFormData usages across 8 controllers, 2,950 total transforms, 0 errors

## v0.3.0 Released

- Phase 1-4 complete (serialization perf, union serialization, TS config support, benchmarks)

## Phase 5: `tsgonest migrate` — AST Transforms COMPLETE

All 5 transform modules implemented and tested:

- **nestia.ts** — Nestia `@TypedRoute.*` / `@TypedBody()` / `@TypedParam()` → NestJS equivalents
- **typia-tags.ts** — `typia.tags.*` → `@tsgonest/types` branded tags
- **class-validator.ts** — class-validator decorators → interfaces with branded types (`*.dto.ts` files get class→interface conversion)
- **class-transformer.ts** — `@Exclude()` / `@Transform()` → TODO comments (not silently dropped)
- **swagger.ts** — `@nestjs/swagger` decorators removed; `@ApiBearerAuth()` → TODO for config migration
- **imports.ts** — Post-transform empty import cleanup

### Testing Results

| Project | Files Modified | Crashes | Errors |
|---------|---------------|---------|--------|
| ever-gauzy (~2k stars, 3123 TS files) | 479 | 0 | 0 |
| twenty (40k stars, 4796 TS files) | 83 | 0 | 0 |
| nestjs-boilerplate (156 TS files) | 38 | 0 | 0 |

- 40 unit tests all passing
- 5 e2e tests all passing

## tsgo Flag Passthrough — COMPLETE

- `parseBuildArgs()` separates tsgonest flags from tsgo flags
- `parseTsgoFlags()` passes unknown flags to `shimtsoptions.ParseCommandLine()`
- `ParseTSConfig` accepts optional `*core.CompilerOptions` for CLI overrides
- 30 unit tests (11 parseBuildArgs + 10 parseTsgoFlags + 4 ParseTSConfig integration + 5 pipeline)
- 6 e2e tests passing

## tsdown Migration — COMPLETE

Replaced esbuild with tsdown v0.20.3 across all workspace packages:

- **`@tsgonest/runtime`**: Dual CJS+ESM with `.d.cts`/`.d.mts` declarations and source maps
- **`@tsgonest/types`**: Dual CJS+ESM with two entries (`index.ts`, `tags.ts`)
- **`packages/core`**: migrate.cjs bundle (10.7MB, down from 12.4MB with esbuild)
- esbuild removed from the workspace entirely
- Exports maps fixed for dual CJS+ESM

## Enhanced Migrate Command — COMPLETE

Industry-standard migration workflow (modeled after Next.js codemods / Angular schematics):

### Features
- **Git clean check** — aborts if uncommitted changes (unless `--force`)
- **Interactive prompts** — per-step Y/n confirmation using Node.js readline
- **`--yes`/`-y` flag** — non-interactive mode (accept all defaults)
- **Package.json manipulation**:
  - Detect old deps (nestia, class-validator, swagger) in both source files AND package.json
  - Remove old dependencies from all dep sections
  - Add tsgonest, @tsgonest/runtime, @tsgonest/types
  - Update build/dev scripts to use tsgonest
- **Config file cleanup** — detect and remove nestia.config.{ts,js,json,mjs,cjs}
- **Structured report** — transform counts, manual TODOs with file:line, package.json changes
- **Markdown report** — `tsgonest-migrate-report.md` written on `--apply`
- **Enhanced dry-run** — shows planned package.json changes alongside source diffs

### Files
| File | Purpose |
|------|---------|
| `packages/core/src/migrate/index.ts` | Main entry — git check, prompts, package.json ops, full flow |
| `packages/core/src/migrate/git.ts` | `isGitRepo()`, `isGitClean()`, `getGitStatus()` |
| `packages/core/src/migrate/prompts.ts` | `confirm()` Y/n prompts, `closePrompts()` |
| `packages/core/src/migrate/packagejson.ts` | `detectPackages()`, `removeDependencies()`, `addTsgonestDependencies()`, `updateScripts()`, `removeConfigFiles()` |
| `packages/core/src/migrate/report.ts` | `MigrateReport` with `packageJsonChanges` field and markdown output |

### Go Binary Changes
- `cmd/tsgonest/main.go` — Updated help text with `--force` and `--yes` flags
- `cmd/tsgonest/migrate.go` — Added dev layout resolution for `migrate.cjs` (packages/core/bin/)

### Test Results
- 40 migrate unit tests passing
- 5 migrate e2e tests passing
- 112 total e2e tests passing
- ~560+ Go unit tests passing
- Smoke tested on nestjs-boilerplate (38 files, 13 manual TODOs, 0 errors)
