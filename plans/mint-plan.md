# Plan: Mint — tiny web framework for Bun

> Brand: **Mint**. Core package: `@mintkit/core`. Bun adapter: `@mintkit/bun`.
> Source PRD: [`../.scratch/mint/PRD.md`](../.scratch/mint/PRD.md)
> Companion technical spec: [`mint-spec.md`](./mint-spec.md)
> Implementation issues: [`../.scratch/mint/issues/`](../.scratch/mint/issues/)

12 phases. Each phase is a vertical tracer bullet through compile-time (tsgonest analyzer + codegen extensions) and runtime (Mint package) and Bun execution. Each phase is independently demoable.

## Architectural decisions

Durable decisions that apply across every phase:

- **Package layout**: Mint's core lives at `packages/mint` with npm name `@mintkit/core`. The Bun adapter (vendored/shadcn-style) lives at `packages/mint-bun` with npm name `@mintkit/bun`. Compile-time extensions live in the existing Go module (`internal/analyzer`, `internal/codegen`) alongside the current NestJS support — additive, not a fork.
- **Runtime target**: Bun. `app.fetch(Request): Promise<Response>` is the universal entry point. v1 also ships a single vendored adapter package providing `wrap(app)`, `BUN_SERVER` token, and `gracefulShutdown` helper.
- **Decorator vocabulary**: `@Controller(path)`, HTTP verb decorators (`@Get`/`@Post`/`@Put`/`@Patch`/`@Delete`/`@Head`/`@Options`), parameter decorators (`@Body`, `@Query`, `@Param`, `@Headers`, `@Ctx`), middleware attach (`@UseMiddleware`), DI (`@Injectable`, `@Inject`). Recognized via import path. Factory-wrapped decorators are unsupported (existing tsgonest constraint).
- **WS reservations baked in from Phase 1**:
  - Middleware/handler return type is `Response | Upgrade` (`Upgrade` is reserved; never produced in v1).
  - Router method slot is a free-form string (`'WS'` registers without code changes when added).
  - `TokenStore` is a shared interface that `Event` implements; future `Session` will implement the same surface.
  - Validator codegen consumes "source bytes" rather than "HTTP request body" — framing-agnostic from day one.
- **Web standards as primitive**: `Request`, `Response`, `Headers`, `File`, `Blob`, `FormData`, `ReadableStream`. The framework wraps Web primitives only where wrapping solves a real problem (`event.body` memoization).
- **Error contract**: throw-only. Default mapper produces RFC 9457 `application/problem+json` for `TsgonestValidationError`, status-keyed JSON for `HttpError` subclasses, generic 500 for everything else. Users register custom mappings via `app.onError`.
- **DI scope**: singleton (default) and transient only. No request scope, no child containers, no `forwardRef`.
- **Boot semantics**: async, fail-fast, partial-dispose on init failure. Boot follows a fixed six-step sequence (flatten → validate → construct → init → register routes → ready).
- **Lifecycle interfaces**: `OnInit` (framework-defined) and `AsyncDisposable` (TC39 standard `Symbol.asyncDispose`). Duck-typed at runtime.
- **TypeScript**: 5.2+ at the consumer (for `using` / `await using`).
- **Test stack**: Vitest for TS modules (continuing the `packages/runtime/test` pattern), Go unit tests for analyzer/codegen (continuing the `internal/*/_test.go` pattern with `testdata/` fixtures), e2e tests that build the binary and run controllers in a Bun subprocess (continuing the `e2e/compile.test.ts` pattern).

---

## Phase 1: Hello World

**User stories**: 2, 30, 31, 35, 36, 50.

### What to build

A user can write a single TypeScript file with one class decorated `@Controller('/hello')` containing one method decorated `@Get()` that returns a string. Running `tsgonest build` extends the existing pipeline to:

- Recognize Mint's decorators by import path (`@mintkit/core`).
- Emit a companion file with a `register…Controller(app)` export wiring the route into Mint's router.
- Include the route in the generated `openapi.json`.

The user writes a `main.ts` that creates a single module with that controller, calls `await createApp({ imports: [...] })`, hands `app.fetch` to `Bun.serve`, and `curl http://localhost:3000/hello` returns the string with a 200.

This phase establishes the entire compile-time → runtime seam end-to-end. Every subsequent phase is incremental thickening.

### Acceptance criteria

- [ ] `@mintkit/core` package exists at `packages/mint` with `createApp`, `defineModule`, and the public decorator surface.
- [ ] tsgonest analyzer recognizes Mint's decorators (resolved via `@mintkit/core` import path) without breaking existing NestJS support.
- [ ] tsgonest codegen emits a `register…Controller(app)` export per controller in the existing `*.tsgonest.js` companion files.
- [ ] Router accepts method + path registration and matches on incoming `Request`.
- [ ] `Event` exposes at minimum the raw `Request`; handler return value is serialized to a `Response`.
- [ ] Container constructs the controller (no deps in this phase) and resolves it for each request.
- [ ] `createApp` returns a `Promise<App>`; `app.fetch(Request): Promise<Response>` works.
- [ ] Middleware/handler return type is typed `Response | Upgrade` (Upgrade never produced).
- [ ] Router method slot is `string` (not a constrained verb union).
- [ ] OpenAPI 3.2 document includes the route.
- [ ] E2E: build a fixture project, boot in Bun, `curl /hello` returns the expected body.
- [ ] Unit tests: container resolve, router add/match, decorator analyzer fixture, codegen fixture.

---

## Phase 2: Typed parameters and validated returns

**User stories**: 3, 4, 53, 55, 56.

### What to build

The user can declare `@Body() body: SomeDto`, `@Query() q: ListQuery`, `@Param('id') id: string & tags.UUID`, and `@Headers('x-foo') header: string` on handler methods. tsgonest's existing validator/serializer generation is reused: parameter types feed the existing analyzer; the codegen's `register…Controller(app)` wrapper invokes the validators before calling the handler and the serializer on the return value. Validation failures throw `TsgonestValidationError`; the framework's error mapper converts that to RFC 9457 `application/problem+json` with per-field detail.

OpenAPI emits request/response schemas for these parameters from the same type metadata.

### Acceptance criteria

- [ ] `@Body`/`@Query`/`@Param`/`@Headers` parameters are extracted from the request and validated against their TypeScript types before the handler runs.
- [ ] Validators and serializers are the existing tsgonest-generated ones (no fork).
- [ ] Return values are serialized via generated fast-JSON serializers; response `Content-Type` is `application/json`.
- [ ] Validation failure produces a `400 application/problem+json` response with `type`/`title`/`status`/`detail`/`errors` per RFC 9457.
- [ ] OpenAPI generates request/response schemas matching the declared types.
- [ ] E2E: round-trip a typed body (valid → 200 with serialized return, invalid → 400 with problem-details).
- [ ] Unit tests cover analyzer recognition of each parameter decorator and codegen wrapper shape.

---

## Phase 3: Dependency injection and dynamic modules

**User stories**: 1, 6, 7, 40, 41.

### What to build

Users can mark classes `@Injectable()` and inject them via constructor. Providers can be declared in three forms within `defineModule({ providers: [...] })`:

- Class shorthand (the class is the token).
- `{ provide: TOKEN, useValue }`.
- `{ provide: TOKEN, useFactory, inject?: [...], scope?: 'singleton' | 'transient' }` — the factory's deps come from the same container.

Tokens are typed via `defineToken<T>(name)` and produce `Token<T>` instances with a phantom generic. Class references are themselves valid tokens. `@Inject(TOKEN)` parameter decorator injects non-class tokens into constructors. Container builds the dependency graph in topological order.

Dynamic modules are functions: `StorageModule(opts): Module` and `StorageModuleAsync({ inject, useFactory }): Module`. Both return the same plain-object module shape.

### Acceptance criteria

- [ ] `defineToken<T>(name)` returns a typed `Token<T>`; class refs are valid tokens.
- [ ] `@Injectable({ scope? })` decorator is recognized; default scope is singleton.
- [ ] All three provider forms register and resolve correctly.
- [ ] Constructor injection works for classes; `@Inject(TOKEN)` resolves non-class deps.
- [ ] Transient providers return a fresh instance on every resolve; singletons are cached.
- [ ] `useFactory` providers can declare async factories (`Promise<T>` return); await happens during boot.
- [ ] Dynamic module functions (sync and async-factory shapes) compose with `imports: [Module(opts), ModuleAsync({...})]` at the app level.
- [ ] E2E: a controller injects a service that depends on a useFactory-provided config token; the full chain resolves and the endpoint returns config-derived data.
- [ ] Unit tests cover topological construction order, transient vs singleton semantics, and all three provider forms.

---

## Phase 4: Middleware and the token store

**User stories**: 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 30, 32, 57, 58, 59, 62, 63.

### What to build

Function-only middleware shaped as `(event, next) => Promise<Response | Upgrade>`. Four attach points, all wiring into the same flat chain:

- `app.use(mw)` and `app.use('/prefix', mw)` — global and prefix-mounted.
- `defineModule({ middleware: [...] })` — applies to the module's declared controllers; no cascade to imports.
- `@UseMiddleware(mw)` on a controller class — every method in the controller.
- `@UseMiddleware(mw)` on a method — single route.

Execution is outer-to-inner across layers, registration order within each layer.

`Event` gains its full surface: token store (`set`/`get`/`require`), memoized `body` accessor with `bytes`/`text`/`json`/`formData` cached and `stream` single-use, mutable `response.headers` (Web `Headers` instance) and `response.status` (number), `waitUntil`, and `resolve` for DI access.

`@Ctx(TOKEN) name: T` parameter decorator extracts token-stored values into handler signatures with compile-time type checking against the token's `T`.

### Acceptance criteria

- [ ] All four middleware attach points register and execute in the documented order.
- [ ] `await next()` returns the downstream response; middleware can wrap (modify headers/status, transform body via new Response).
- [ ] Short-circuit by returning a Response without calling `next()`.
- [ ] Thrown errors in middleware propagate and hit the error mapper.
- [ ] `event.set(TOKEN, value)` / `event.get(TOKEN)` / `event.require(TOKEN)` are type-checked against the token's `T`; `require` throws if not set.
- [ ] `event.body` memoizes `bytes`/`text`/`json`/`formData`; `event.body.stream()` errors if the body has already been consumed.
- [ ] `event.response.headers` is a `Headers` instance; `event.response.status` is a settable number.
- [ ] `event.waitUntil(promise)` is callable; on Bun it is a fire-and-forget that runs to completion after response.
- [ ] `@Ctx(TOKEN) user: User` produces a compile-time diagnostic if the parameter type does not match the token's `T`.
- [ ] Module-level middleware does **not** cascade to imported modules' controllers.
- [ ] E2E: a request flows through global + module + controller + method middleware in the correct order; an auth middleware sets a token and a handler reads it via `@Ctx`.
- [ ] Unit tests cover chain composition order, short-circuit, throw propagation, body memoization semantics, and type-store behavior.

---

## Phase 5: Errors

**User stories**: 4, 27, 28, 29.

### What to build

The `HttpError` hierarchy is shipped: `BadRequestError`, `UnauthorizedError`, `ForbiddenError`, `NotFoundError`, `ConflictError`, `UnprocessableEntityError`, `TooManyRequestsError`, `InternalServerError` — each carries a status and a JSON body. Default error mapper maps `HttpError` subclasses to JSON responses with the carried status, maps `TsgonestValidationError` to RFC 9457 problem-details (already in Phase 2), and falls through to status 500 for unknown errors (with the full error logged, not leaked to the response).

`app.onError(predicate, handler)` registers user-defined mappings. User handlers take precedence over defaults; first match wins.

### Acceptance criteria

- [ ] `HttpError` base class and all listed subclasses exist with correct status codes.
- [ ] Throwing an `HttpError` subclass from a handler produces a Response with the carried status and body.
- [ ] `app.onError` registered handlers run before defaults; first match wins.
- [ ] Unmatched throws produce a 500 problem-details JSON; the underlying error is logged.
- [ ] E2E: a controller throws a custom domain error; an `app.onError` registration maps it to a `409 Conflict`.
- [ ] Unit tests cover precedence between user handlers and defaults, and the 500 fallthrough.

---

## Phase 6: Lifecycle and graceful shutdown

**User stories**: 8, 9, 10, 11, 47, 48, 50.

### What to build

Providers can implement `OnInit` (`init(): void | Promise<void>`) and/or `AsyncDisposable` (`[Symbol.asyncDispose](): PromiseLike<void>`). Container duck-types both at runtime. Boot runs `init()` after all construction, in topological order. If any `init()` throws, the framework disposes already-initialized providers in reverse topological order and rejects `createApp`. Cycles between providers throw at boot with the cycle path printed.

`app[Symbol.asyncDispose]()` stops accepting new requests, drains in-flight requests, then runs each provider's `Symbol.asyncDispose` in reverse topological order.

This phase nails the runtime lifecycle. The Bun-side SIGTERM trigger is wired in Phase 12.

### Acceptance criteria

- [ ] `OnInit` interface is exported; providers implementing `init()` have it called after construction.
- [ ] `Symbol.asyncDispose` on providers is called on app shutdown.
- [ ] Construction order is topological; init order is topological; dispose order is reverse-topological.
- [ ] Failed `init()` triggers reverse-topo dispose of already-initialized providers and rejects `createApp`.
- [ ] Boot detects provider cycles and throws with the cycle path before any provider construction.
- [ ] `app[Symbol.asyncDispose]()` drains in-flight requests (long handlers complete) before disposing providers.
- [ ] Unit tests cover topological vs reverse-topological order, partial-dispose on init failure, and cycle detection.

---

## Phase 7: Module visibility and boot validation

**User stories**: 5, 49, 51, 52, 54.

### What to build

Module resolver enforces visibility rules: providers are private to their declaring module unless listed in `exports`. Importing module A from module B does not auto-leak A's providers; if B wants to re-export A's provider, it must list it in B's `exports`. Controllers are always globally registered.

Boot-time validation:

- Detect duplicate provider tokens across the flattened graph; error with both declaring modules named.
- Detect cycles between modules and between providers.
- Verify every `@Inject(TOKEN)` resolves to a visible provider; error with the consumer and unresolved token.

No glob / auto-discovery. All imports are explicit.

### Acceptance criteria

- [ ] Private-by-default providers: importing module A in module B does not let B's providers inject A's non-exported services.
- [ ] `exports` lists make providers visible to importers.
- [ ] Re-exports through chains (A imports B, A exports B's token) are explicit and required for transitive visibility.
- [ ] Duplicate tokens across the flattened module graph throw at boot with both source modules in the message.
- [ ] Unresolved `@Inject(TOKEN)` references throw at boot with consumer + token info.
- [ ] No file-system / glob-based module loading anywhere in the boot path.
- [ ] Unit tests cover all visibility scenarios (private, exported, re-exported, leak attempt) and each validation error class.

---

## Phase 8: Headless context

**User stories**: 33, 34, 35, 64.

### What to build

`createContext({ imports }): Promise<Context>` returns a `Context` with `resolve` and `Symbol.asyncDispose` — no router, no fetch, no middleware. Useful for CLIs, queue consumers, cron, migrations, tests. `App extends Context` so helpers written against `Context` work in `App` too. Controllers in modules used by `createContext` are silently ignored (no warnings).

`await using` is the canonical pattern for short-lived contexts; auto-dispose on scope exit, even on throw.

### Acceptance criteria

- [ ] `createContext({ imports })` exists and resolves to a `Context` with `resolve` and `Symbol.asyncDispose`.
- [ ] `Context` shares the same module/provider/lifecycle plumbing as `createApp` (no duplication).
- [ ] `App extends Context` structurally; helpers typed against `Context` work inside `createApp` returns.
- [ ] Controllers in imported modules are accepted but unused when boot is via `createContext`.
- [ ] E2E: a CLI fixture using `await using ctx = await createContext(...)` runs a service method and exits with provider dispose having fired.
- [ ] Unit tests cover `createContext` boot, `App extends Context` structural compatibility, and `await using` integration.

---

## Phase 9: File uploads (buffered)

**User stories**: 24.

### What to build

Users declare `@Body() data: { title: string; image: File & MaxSize<5_000_000> & MimeTypes<'image/png' | 'image/jpeg'> }` on a handler. tsgonest analyzer recognizes the `File` type and the new tag constraints. Codegen generates a multipart parser that reads `event.body.formData()`, extracts each named entry, validates `File` entries against `MaxSize`/`MinSize`/`MimeTypes`, and throws `TsgonestValidationError` on violation. Handler receives Web standard `File` instances.

New tag types in `@tsgonest/types`: `MaxSize<N>`, `MinSize<N>`, `MimeTypes<U>` (with wildcard support like `'image/*'`).

### Acceptance criteria

- [ ] `File`, `MaxSize<N>`, `MinSize<N>`, `MimeTypes<U>` types are added to `@tsgonest/types` (or runtime types) and recognized by the analyzer.
- [ ] Wildcard MIME patterns (`'image/*'`) match correctly.
- [ ] `@Body()` with file fields parses multipart via `event.body.formData()`, validates per-file constraints, and passes `File` instances to the handler.
- [ ] Validation failure produces an RFC 9457 problem-details response with per-field detail.
- [ ] Array fields (`photos: Array<File & ...>`) parse and validate as a list.
- [ ] OpenAPI emits `multipart/form-data` request body schemas for these routes.
- [ ] E2E: round-trip a buffered file upload (valid → 200, oversize → 400, wrong MIME → 400).
- [ ] Analyzer/codegen unit tests cover the new types and the generated parser shape.

---

## Phase 10: File uploads (streaming)

**User stories**: 24, 25.

### What to build

Users declare `@Body() data: { filename: string; file: FileStream & MaxSize<5_000_000_000> }`. `FileStream` is distinct from `File` and signals streaming parse. Codegen emits a streaming multipart parser (a runtime-shipped utility) that reads `event.body.stream()`, yields each field as it arrives, and aborts the connection when a `FileStream`'s incremental byte count exceeds `MaxSize`. The handler receives `FileStream { name, type, stream }` where `stream` is a `ReadableStream<Uint8Array>`.

### Acceptance criteria

- [ ] `FileStream` type exists and is recognized by the analyzer as distinct from `File`.
- [ ] A multipart streaming parser ships in the runtime (or framework package) and reads from a `ReadableStream<Uint8Array>` source.
- [ ] When any `FileStream` field exceeds its `MaxSize` mid-stream, the connection aborts and a 413 (or comparable problem-details) response is sent.
- [ ] Memory usage during a multi-GB upload stays bounded (verified via integration test or a documented design constraint).
- [ ] Mixed multipart (string fields + streaming files) parses correctly without buffering the entire body.
- [ ] E2E: stream a large file to disk via the handler, then assert the file on disk matches the input; oversize uploads abort early.
- [ ] Unit tests cover the parser's boundary detection, field dispatch, and abort semantics.

---

## Phase 11: SSE

**User stories**: 26.

### What to build

A core runtime helper `sse<T>(generator: () => AsyncGenerator<T>): Response` returns a streaming `Response` with `text/event-stream` content type and proper SSE framing (`data: <json>\n\n`). Typed via `SSEStream<T>` so the existing `@Returns<SSEStream<T>>` pattern in tsgonest picks up the event type for OpenAPI/SDK generation.

Handlers return `sse(async function* () { yield ...; })` or are typed as `Promise<SSEStream<EventType>>`.

### Acceptance criteria

- [ ] `sse()` helper exists in the runtime package and produces a correctly framed SSE response.
- [ ] `SSEStream<T>` type is recognized by the analyzer; OpenAPI emits the event payload schema using the existing `@Returns<T>` machinery.
- [ ] Generators can yield, await between yields, and close the stream by returning.
- [ ] Client disconnects propagate to the generator (abort signal in the response stream).
- [ ] E2E: connect a Bun client to an SSE endpoint, receive at least N events in framed form, disconnect, verify the generator cleanup runs.
- [ ] Unit tests cover framing correctness and abort propagation.

---

## Phase 12: Bun adapter — `@mintkit/bun` (vendored package)

**User stories**: 37, 38, 45, 46.

### What to build

A single vendored package at `packages/mint-bun` (npm name `@mintkit/bun`), intended to be copy-pasted into user repos under the shadcn philosophy, providing:

- `wrap(app): { fetch }` — for users who need access to Bun's `Server` instance. Injects `BUN_SERVER` token onto each event before `app.fetch` runs.
- `BUN_SERVER` token — typed reference to Bun's `Server` (e.g., for future WebSocket upgrades or socket inspection).
- `gracefulShutdown(app, opts?)` helper — wires SIGTERM/SIGINT (configurable) to `app[Symbol.asyncDispose]()` with a configurable drain timeout.

Zero-config users continue to write `Bun.serve({ fetch: app.fetch })`. The adapter is only needed when platform features are wanted.

### Acceptance criteria

- [ ] `@mintkit/bun` package exists at `packages/mint-bun` with `wrap`, `BUN_SERVER`, and `gracefulShutdown` exports.
- [ ] `wrap(app)` returns a `{ fetch }` handler that injects `BUN_SERVER` on each event before delegating to `app.fetch`.
- [ ] `BUN_SERVER` token resolves to Bun's `Server` inside handlers/middleware via `event.require(BUN_SERVER)`.
- [ ] `gracefulShutdown` registers the configured signal handlers, calls `app[Symbol.asyncDispose]()`, enforces the timeout (hard-exits if drain exceeds).
- [ ] E2E: SIGTERM during in-flight requests drains them, fires provider dispose in reverse-topo, and exits cleanly within the timeout window.
- [ ] Documentation in the adapter's README walks through the canonical wiring and the customization points users are expected to edit.
