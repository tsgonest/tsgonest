# AGENTS.md - AI Assistant Guide for tsgonest

tsgonest is a Go binary wrapping typescript-go (tsgo) that:

1. Compiles TypeScript via tsgo (fast, native-speed)
2. Generates companion files (`.tsgonest.js`) with runtime validators and fast JSON serializers
3. Generates an OpenAPI 3.2 document from NestJS controllers via static analysis
4. Provides a lightweight `@tsgonest/runtime` npm package with a `ValidationPipe` and `SerializationInterceptor`
5. Replaces `nest` CLI with `tsgonest dev` (watch+reload) and `tsgonest build` (with tsconfig path alias resolution)

## CRITICAL: typescript-go Submodule Warning

**DO NOT COMMIT SUBMODULE CHANGES WHEN FINALIZING WORK**

The `typescript-go/` directory is a Git submodule referencing Microsoft's TypeScript Go port.

### During Development

- Changes ARE allowed to the typescript-go submodule for testing
- You can freely modify and commit within the `typescript-go/` folder

### Before Finalizing Work

- Convert any typescript-go changes into patch files in `patches/`
- **NEVER** commit the submodule pointer changes to the tsgonest repository
- When you see `modified: typescript-go (new commits)` in git status, do NOT stage/commit it

### Creating Permanent Changes

1. Test your changes locally in the typescript-go directory
2. Create a patch file in `patches/` using `git format-patch`
3. Reset the typescript-go submodule to its original state
4. Patches are applied during `just init` via `git am`

### Exposing New Functions

1. Add the function to the appropriate `shim/*/extra-shim.json`
2. Run `just shim` (or `go run tools/gen_shims/main.go`) to regenerate shim files
3. Commit both the shim config changes and regenerated shim files

## Repository Structure

This is a **pnpm workspace monorepo**. The root `pnpm-workspace.yaml` covers
`apps/*` and `packages/*`. Go source lives at the repo root.

```
# ── Root files ──────────────────────────────────────────────
justfile                   Task runner recipes (just init, just build, just test, …)
pnpm-workspace.yaml        Workspace definition (apps/*, packages/*)
package.json               Workspace root — scripts for build, dev, test

apps/
  docs/                    Next.js + Fumadocs documentation site (static export)
                             pnpm --filter docs dev|build|start

packages/
  core/                    tsgonest — main npm package (name: "tsgonest")
                             • installs the correct platform binary via postinstall
                             • workspace:* deps on @tsgonest/runtime and @tsgonest/types
  runtime/                 @tsgonest/runtime — NestJS ValidationPipe, SerializationInterceptor,
                             FastInterceptor, CompanionDiscovery
  types/                   @tsgonest/types — zero-runtime branded phantom types
                             (string & tags.Email, tags.MinLength<1>, …)
  cli-darwin-arm64/        @tsgonest/cli-darwin-arm64 — macOS Apple Silicon binary
  cli-darwin-x64/          @tsgonest/cli-darwin-x64  — macOS Intel binary
  cli-linux-arm64/         @tsgonest/cli-linux-arm64  — Linux ARM64 binary
  cli-linux-x64/           @tsgonest/cli-linux-x64   — Linux x64 binary
  cli-win32-x64/           @tsgonest/cli-win32-x64   — Windows x64 binary

# ── Go source (repo root) ────────────────────────────────────
cmd/tsgonest/              CLI entry point (main.go, build.go, dev.go, assets.go)
internal/
  collections/             Copied from typescript-go/internal/collections during init
  compiler/                tsgo program creation, host, and emission
  config/                  Config file parsing and validation
  analyzer/                AST analysis (type walking, NestJS decorator recognition)
  metadata/                Type metadata schema (Go equivalent of typia's Metadata)
  codegen/                 Code generation (companions: validate/assert/serialize/schema, manifest)
  openapi/                 OpenAPI 3.2 document assembly
  testutil/                Test utilities (OverlayVFS)
shim/                      [GENERATED] Go bindings to tsgo internals (DO NOT EDIT)
patches/                   Git patches applied to typescript-go during init
tools/gen_shims/           Shim code generator
e2e/                       End-to-end tests (Vitest) — run after building the Go binary
testdata/                  Test fixtures (TypeScript files, NestJS controllers, branded types)
typescript-go/             [SUBMODULE] Microsoft's TypeScript Go port
```

## pnpm Workspace — Key Commands

```bash
# Install all workspace packages from the root
pnpm install

# Build everything (runs "build" script in every workspace package)
pnpm build

# Build only npm packages (runtime, types, core)
pnpm build:packages

# Docs — development server
pnpm dev:docs

# Docs — production static export (output in apps/docs/out/)
pnpm build:docs

# Run e2e tests (requires the Go binary to be built first)
pnpm test:e2e
```

Individual packages can still be worked on in isolation:

```bash
pnpm --filter @tsgonest/runtime build
pnpm --filter @tsgonest/types   build
pnpm --filter docs              dev
```

## Generated Code Warning

The code within `./shim/` is generated. **Do not edit it.** Regenerate with `just shim` (or `go run tools/gen_shims/main.go`).

## Architecture Overview

### Compilation Pipeline

1. Parse CLI args + config (`cmd/tsgonest/main.go`)
2. Create tsgo program (incremental or non-incremental) (`internal/compiler/program.go`)
3. Gather diagnostics (`GetDiagnosticsOfAnyProgram`) — type errors are fatal by default (exit 1)
4. Emit JS via tsgo emitter
5. Check post-processing cache — skip steps 6-9 if nothing changed (`internal/buildcache/`)
6. Walk AST with type checker to extract type metadata (`internal/analyzer/`)
7. Generate companion files (`*.tsgonest.js` + `*.tsgonest.d.ts`) (`internal/codegen/`)
8. Generate manifest (`internal/codegen/manifest.go`)
9. Generate OpenAPI 3.2 document (`internal/openapi/`)
10. Save post-processing cache

### Design Decisions

- **Language**: Go — direct access to tsgo internals via shims
- **User API**: Type-annotation driven — BOTH JSDoc tags (zero-dep) AND branded phantom types (`@tsgonest/types` for type safety + autocomplete)
- **Validation feel**: Zod-elegant JSDoc, or type-safe branded types (`string & tags.Email`)
- **Branded types**: `@tsgonest/types` package with `__tsgonest_*` phantom properties, also detects typia's `"typia.tag"` for migration
- **Output**: Companion files (`*.tsgonest.js` + `*.tsgonest.d.ts`) alongside tsgo output — each companion exports validate, assert, serialize, and schema functions
- **OpenAPI**: 3.2 only, static analysis (no runtime/reflect-metadata)
- **Config**: JSON now (`tsgonest.config.json`), TypeScript config support planned
- **Runtime**: `@tsgonest/runtime` npm package provides `TsgonestValidationPipe` + `TsgonestSerializationInterceptor`
- **Standard Schema**: v1 wrappers for 60+ framework interop
- **CLI replacement**: `tsgonest dev` + `tsgonest build` replace `nest start --watch` + `nest build`
- **Distribution**: Per-platform npm packages + GitHub releases

### Companion Files

Each non-controller type gets a single `.tsgonest.js` companion file (with `.tsgonest.d.ts` types):

```
dist/
  user.dto.js                    # tsgo output
  user.dto.UserDto.tsgonest.js   # companion: validate + assert + serialize + schema
  user.dto.UserDto.tsgonest.d.ts # companion types
```

Controller classes are detected via `@Controller()` decorator and **skipped** for companion generation.

### Manifest Format

```json
{
  "version": 1,
  "companions": {
    "UserDto": {
      "file": "./user.dto.UserDto.tsgonest.js",
      "validate": "validateUserDto",
      "assert": "assertUserDto",
      "serialize": "serializeUserDto",
      "schema": "schemaUserDto"
    }
  },
  "routes": { ... }
}
```

### Shim Layer

tsgonest uses the same shim pattern as tsgolint to access tsgo's internal APIs:

- Each `shim/` package has a `go.mod` + `extra-shim.json`
- `go:linkname` directives reach into unexported functions
- `unsafe.Pointer` field accessors for unexported struct fields
- The root `go.mod` has `replace` directives pointing to local `./shim/` paths
- `go.work` workspace includes both the root module and typescript-go

### packages/core — Binary Distribution

`packages/core` is the `tsgonest` npm package users install. Its `install.js`
postinstall script detects `process.platform + process.arch` and copies the
correct pre-built binary from the matching `@tsgonest/cli-*` optional dep into
`packages/core/bin/tsgonest`. This is the standard pattern used by tools like
esbuild and @biomejs/biome.

In the workspace, the `@tsgonest/cli-*` optional deps are resolved via
`workspace:*` (linked locally). When publishing to npm they are replaced by
the actual version number via `pnpm publish`.

## Common Tasks

This project uses [`just`](https://just.systems) as a task runner. All recipes
are defined in `justfile` at the repo root.

### First-time Setup

```bash
just init    # Clone submodule, apply patches, copy collections
```

### Building

```bash
just build                              # Build Go binary + copy to packages/core/bin
go run tools/gen_shims/main.go          # Regenerate shims (or: just shim)
```

### Running Tests

```bash
just test          # Build + Go unit tests + e2e tests (full suite)
just test-unit     # Go unit tests only
just test-e2e      # Build + e2e tests only
```

### Formatting & Pre-commit

```bash
just fmt     # gofmt on internal/, cmd/, tools/
just ready   # fmt + full test suite — run before committing
```

### Working on the runtime package

```bash
pnpm --filter @tsgonest/runtime build   # Compile TypeScript → dist/
pnpm --filter @tsgonest/runtime test    # Run vitest
```

### Working on the types package

```bash
pnpm --filter @tsgonest/types build     # Compile TypeScript → dist/
pnpm --filter @tsgonest/types test      # tsc --noEmit
```

### Working on the docs

```bash
pnpm --filter docs dev     # Dev server with hot reload
pnpm --filter docs build   # Static export → apps/docs/out/
pnpm --filter docs start   # Serve the static export locally
```

### Adding a New Shim Method

1. Find the method in typescript-go source (e.g., `checker.getTypeOfExpression`)
2. Add it to the appropriate `shim/*/extra-shim.json` under `ExtraMethods`
3. Run `just shim` to regenerate
4. Use it in Go code as `checker.Checker_getTypeOfExpression(recv, args...)`

### Working with Type Metadata

- Type analysis code lives in `internal/analyzer/`
- Metadata schema is in `internal/metadata/`
- Use tsgo checker shims to inspect types: `checker.Checker_getPropertiesOfType(c, t)`
- Always track visited types to prevent infinite recursion on recursive types

## File Modification Guidelines

### Safe to Modify

- `internal/*` — All internal packages
- `cmd/tsgonest/*` — CLI implementation
- `e2e/*` — End-to-end tests
- `testdata/*` — Test fixtures
- `packages/runtime/*` — npm runtime package source
- `packages/types/*` — npm types package source
- `packages/core/*` — main tsgonest npm package
- `apps/docs/*` — documentation site
- Documentation files (`*.md`)

### DO NOT Modify

- `typescript-go/*` — Submodule (use `patches/` for permanent changes)
- `shim/*/shim.go` — Generated code (regenerate with `go run tools/gen_shims/main.go`)
- `.gitmodules` — Submodule configuration
- `internal/collections/*` — Synced from typescript-go by init
- `packages/cli-*/` — Platform binary packages; populated by CI, not hand-edited

### Modify with Caution

- `patches/*` — TypeScript-go patches (document thoroughly)
- `tools/gen_shims/*` — Shim generator (affects all shims)
- `go.mod`, `go.work` — Module configuration
- `shim/*/extra-shim.json` — Shim configuration (regenerate shims after)
- `pnpm-workspace.yaml`, root `package.json` — Workspace configuration

## Testing Strategy

### Go Unit Tests (~546+ tests)

- Config parsing: `internal/config/config_test.go`
- Type walker: `internal/analyzer/type_walker_test.go` (JSDoc, branded types, discriminants)
- NestJS analyzer: `internal/analyzer/nestjs_test.go`
- Code generation: `internal/codegen/codegen_test.go` (companions, constraints, formats, numerics)
- Emitter: `internal/codegen/emitter_test.go`
- OpenAPI schemas: `internal/openapi/schema_test.go`
- OpenAPI generator: `internal/openapi/generator_test.go`
- Manifest: `internal/codegen/manifest_test.go`
- Build cache: `internal/buildcache/cache_test.go`
- Each test uses fixtures from `testdata/`

### E2e Tests (Vitest, 97 tests)

- `e2e/compile.test.ts` — Full pipeline: compilation, companions, manifest, OpenAPI, constraint validation, realworld fixture, branded types, diagnostics, exit codes, incremental, post-processing cache, @Returns decorator
- Run the `tsgonest` binary as a subprocess
- Verify output files, content, and error handling
- Execute generated JS in Node.js to verify runtime correctness

## Key Dependencies

- `typescript-go` — Microsoft's Go port of TypeScript (submodule)
- `golang.org/x/tools/go/packages` — For shim generation
- `golang.org/x/text` — For text case conversion in shim generator
- Vitest — E2e test framework + runtime tests
- Next.js + Fumadocs — Documentation site framework

## Common Pitfalls

1. **Modifying typescript-go without patches**: Changes will be lost on submodule update
2. **Editing shim files directly**: Will be overwritten on shim regeneration
3. **Forgetting `go mod tidy`**: After adding new shim imports
4. **Recursive types in type walker**: Always use visited-type tracking
5. **tsgo API changes**: When updating the submodule, shims may break — regenerate and fix
6. **E2e tests expect built binary**: Always build before running e2e tests
7. **OpenAPI output path**: Resolves relative to config file directory, not CWD
8. **JS string escaping**: Use `jsStringEscape()` when embedding strings in JS string literals
9. **Boolean representation**: TS `boolean` is `true | false` union — walker coalesces them
10. **Branded type detection**: `__tsgonest_*` phantom properties extracted as constraints; `"typia.tag"` also recognized
11. **rootDir auto-inference**: tsgonest computes rootDir from source files — output is flat in `dist/` (no `dist/src/` nesting)
12. **Path alias algorithm**: Adapted from esbuild — exact match first, then longest-prefix wildcard
13. **pnpm workspace:\* versions**: In `packages/core/package.json`, platform CLI deps use `workspace:*` locally; `pnpm publish` replaces these with the actual version number automatically
14. **docs is at apps/docs**: The Next.js documentation site is under `apps/docs/`, not `docs/` — update any scripts or CI steps that reference the old path
15. **Companion file naming**: Files use `.tsgonest.js`/`.tsgonest.d.ts` suffix (NOT `.validate.js`/`.serialize.js` — those are old format)
16. **Controller classes have no companions**: `@Controller()` classes are detected and skipped for companion generation
17. **@Res() routes are never serialized**: `@Returns<T>()` is purely for OpenAPI; the response is handled manually by the developer
18. **`ManifestEntry` is now `CompanionEntry`**: The runtime exports `CompanionEntry` (not the old `ManifestEntry`) from `packages/runtime/src/discovery.ts`
