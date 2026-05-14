# Mint — Technical Spec

A tiny web framework — DI + modules + routing — built on tsgonest's compile-time pipeline, targeting Bun.

- **Brand:** Mint.
- **Core package:** `@mintkit/core` (workspace dir `packages/mint`).
- **Bun adapter package:** `@mintkit/bun` (workspace dir `packages/mint-bun`, vendored/shadcn-style).
- **Source PRD:** [`../.scratch/mint/PRD.md`](../.scratch/mint/PRD.md).
- **Implementation plan:** [`./mint-plan.md`](./mint-plan.md).

This document is the technical specification: API surfaces, contracts, runtime semantics. The PRD covers the "why" and the user stories; this covers the "how."

---

## Scope

- **Runtime target (v1):** Bun. No Node, Workers, Deno, or Lambda adapters in v1.
- **TypeScript:** 5.2+ at the consumer (required for `using` / `await using`).
- **Compile-time:** tsgonest's existing pipeline, extended additively.
- **Deferred (with reservations):** WebSockets, additional platform adapters, AsyncAPI, shadcn CLI.

---

## Core invariants

These hold across the entire framework. Every other decision derives from them.

1. **Web standards are the lingua franca.** `Request`, `Response`, `Headers`, `File`, `Blob`, `FormData`, `ReadableStream` are first-class. The framework wraps Web primitives only where wrapping solves a real problem (`event.body` memoization).
2. **Decorators are compile-time only.** All framework decorators are statically analyzed by tsgonest and erased at runtime. The runtime does not depend on `reflect-metadata`.
3. **Throw-based error contract.** Handlers and middleware throw to fail; the error mapper converts throws to Responses. No `Result` types in core.
4. **Singleton + transient scope only.** No request scope, no per-connection scope, no child containers.
5. **`createApp` is async; boot is fail-fast.** Partial-init states are not observable from outside.
6. **WS-additive reservations.** Middleware/handler return type is `Response | Upgrade`. Router method is a free-form string. Token store is a shared interface. Compile-time validator codegen is framing-agnostic.

---

## Module layout

13 modules. The first four are deep modules (algorithmic, isolatable, simple interfaces, change rarely).

### Deep core

1. **Container** — DI/provider resolution.
2. **Module resolver** — flatten/dedupe/validate the module graph.
3. **Router** — radix tree, method + path matching.
4. **Middleware chain** — onion composition across four attach points.

### Runtime state

5. **Event** — per-request state; raw `request`, memoized `body`, mutable `response`, token store, `waitUntil`, `resolve`.
6. **Error mapper** — pluggable throw → Response mapping; defaults for `HttpError` and `TsgonestValidationError`.
7. **App / Context factory** — boot orchestration, lifecycle, `Symbol.asyncDispose`.

### Compile-time (extensions to tsgonest)

8. **Decorator analyzer extension** — recognize new decorators in the existing analyzer pipeline.
9. **Route registration codegen** — emit `registerXxxController(app)` exports in `*.tsgonest.js` companions.

### Runtime utilities

10. **`sse()` helper** — async generator → SSE-framed Response.
11. **`HttpError` hierarchy** — typed status-carrying error classes.
12. **Public primitives** — `defineToken`, `defineModule`, `defineHandler`, `defineMiddleware`.

### Vendored (shadcn-style, single package)

13. **Bun adapter** — `wrap(app)`, `BUN_SERVER` token, `gracefulShutdown` helper.

---

## Container

### Token

```ts
class Token<T> {
  declare readonly _type: T;  // phantom, never assigned at runtime
  constructor(public readonly name: string) {}
}

function defineToken<T>(name: string): Token<T>;
```

A class reference is itself a valid `Token<InstanceType<C>>` — class providers are registered under the class as the token.

### Provider forms

```ts
type Provider<T = unknown> =
  | (new (...args: any[]) => T)                                              // class shorthand
  | { provide: Token<T>; useValue: T }                                       // value
  | { provide: Token<T>; useFactory: (...deps: any[]) => T | Promise<T>;     // factory
      inject?: Array<Token<any>>; scope?: 'singleton' | 'transient' };
```

Class providers carry their scope via `@Injectable({ scope?: 'singleton' | 'transient' })` decorator (default singleton).

### Resolution

```ts
interface Container {
  resolve<T>(token: Token<T>): T;  // throws if not registered
}
```

- **Singleton** (default): construct on first resolve, cache for the container's lifetime.
- **Transient**: construct on every resolve; never cached.

### Boot ordering

Providers are constructed in topological order (deps before dependents). `init()` runs in the same topological order after all construction completes. Dispose runs reverse-topologically.

### Cycle detection

Performed at boot. Throws with the full cycle path. No `forwardRef`.

### What's not supported (v1)

- `useExisting` (express as a factory)
- Multi-providers
- String-keyed tokens
- Function-shape providers (`defineService((deps) => ({...}))`)
- Child containers / per-context scopes

---

## Module resolver

### Module shape

```ts
function defineModule(config: {
  imports?:     Module[];
  providers?:   Provider[];
  exports?:     Token<any>[];
  controllers?: ControllerConstructor[];
  middleware?:  Middleware[];
}): Module;
```

`Module` is a plain object; `defineModule` is a typed pass-through.

### Dynamic modules

Functions returning `Module`. There is no `DynamicModule` class.

```ts
function StorageModule(opts: StorageOptions): Module;
function StorageModuleAsync(opts: {
  inject: Token<any>[];
  useFactory: (...deps: any[]) => Promise<StorageOptions> | StorageOptions;
}): Module;
```

### Visibility rules

- Providers are **private to the declaring module** unless listed in `exports`.
- Controllers are **always globally registered** (URL paths are global; scoping them by module would not help).
- Module-level `middleware` binds to **this module's `controllers` only**. Imported modules carry their own bindings; **no cascade to imports**.
- Re-exports through chains are explicit (A imports B, A lists B's token in `exports`, then A's importers can see it).

### Validation at boot

- No duplicate provider tokens across the flattened graph (error: list both declaring modules).
- No cycles between providers (error: print cycle path).
- Every `@Inject(TOKEN)` resolves to a visible provider (error: token name + consumer).
- No glob / auto-discovery.

---

## Router

Radix tree. Method slot is a free-form string (`'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS' | 'WS' | ...`). Param extraction by name (`/:id`).

```ts
interface Router {
  add(method: string, path: string, handler: RouteHandler): void;
  match(method: string, path: string): { handler: RouteHandler; params: Record<string,string> } | undefined;
}
```

Routes are added by tsgonest-generated `registerXxxController(app)` calls or by user code (`app.router.add(...)`).

---

## Middleware chain

### Shape

```ts
type Middleware = (
  event: Event,
  next: () => Promise<Response | Upgrade>,
) => Promise<Response | Upgrade>;

type Upgrade = { _tag: 'upgrade'; /* WS handler refs — v1 unused */ };
```

V1 produces no `Upgrade` values; the union is reserved so adding WS later is non-breaking.

### Attach points

Four, in execution order (outer → inner):

1. **App** — `app.use(mw)` or `app.use('/prefix', mw)`. Registered globally or for a URL prefix.
2. **Module** — `defineModule({ middleware: [...] })`. Applies to that module's declared `controllers` only. No cascade.
3. **Controller** — `@UseMiddleware(mw)` on the class. Applies to every route in that controller.
4. **Method** — `@UseMiddleware(mw)` on the method. Applies to that single route.

### Execution semantics

- **Order:** outer-to-inner across layers; registration order within each layer. No precedence numbers, no `before`/`after` declarations.
- **`await next()`** runs the rest of the chain; returns the downstream Response (or `Upgrade`).
- **Short-circuit:** return a Response without calling `next()`.
- **Throw:** propagates up the chain; unhandled throws hit the error mapper.
- **No class middleware.** Function shape only. DI via `event.resolve(Token)`.
- **No guard sugar.** Guards are middleware that throw on failure. `defineGuard` is intentionally absent — ecosystem modules ship guards as middleware.

### Streaming + header timing

Once a downstream handler returns a streaming Response, headers are committed. Middleware that wants to set headers must do so **before** `await next()`. Documented rule; not runtime-enforced.

---

## Event

The per-request state object. Passed to every middleware and (in generated wrappers) every handler.

```ts
interface Event extends TokenStore {
  request:    Request;                  // raw Web Request
  body:       Body;                     // memoized body accessor
  response:   { headers: Headers; status: number };
  waitUntil(promise: Promise<unknown>): void;
  resolve<T>(token: Token<T>): T;       // DI access from middleware
}

interface TokenStore {
  set<T>(token: Token<T>, value: T): void;
  get<T>(token: Token<T>): T | undefined;
  require<T>(token: Token<T>): T;       // throws if not set
}

interface Body {
  bytes():    Promise<Uint8Array>;      // memoized
  text():     Promise<string>;          // derived from bytes
  json<T = unknown>(): Promise<T>;      // derived from bytes
  formData(): Promise<FormData>;        // derived from bytes
  stream():   ReadableStream<Uint8Array>; // single-use; errors if bytes/text/json/formData already called
}
```

`TokenStore` is declared as a shared interface so a future `Session` (for WS) implements the same surface.

### Response state

`event.response.headers` is a real `Headers` instance (`.set/.append/.delete/.has/.get`). `event.response.status` is a plain number. Both mutated directly — no `setHeader`/`setStatus` framework methods.

### `waitUntil`

Adapter wires this to platform-specific equivalents. On Bun, the default is fire-and-forget; promises run to completion after the response is sent. The abstraction exists so portable middleware can write `event.waitUntil(p)` without branching on platform.

---

## Error mapper

### Default handlers

```ts
class HttpError extends Error {
  constructor(public status: number, public body?: unknown);
}
class BadRequestError      extends HttpError { /* status 400 */ }
class UnauthorizedError    extends HttpError { /* status 401 */ }
class ForbiddenError       extends HttpError { /* status 403 */ }
class NotFoundError        extends HttpError { /* status 404 */ }
class ConflictError        extends HttpError { /* status 409 */ }
class UnprocessableEntityError extends HttpError { /* status 422 */ }
class TooManyRequestsError extends HttpError { /* status 429 */ }
class InternalServerError  extends HttpError { /* status 500 */ }
```

Defaults:

- **`HttpError` subclasses** → JSON response with `error.body`, the carried `status`, content-type `application/json`.
- **`TsgonestValidationError`** (from tsgonest runtime) → RFC 9457 `application/problem+json`, status 400, `type`/`title`/`status`/`detail`/`errors` shape with per-field detail.
- **Unhandled** → status 500, problem-details JSON with minimal detail (no leak), full error logged.

### Extensibility

```ts
app.onError(predicate: (err: unknown) => boolean, handler: (err: unknown, event: Event) => Response | Promise<Response>);
```

User-registered handlers run before defaults. First match wins.

---

## App / Context factory

### `createContext`

```ts
async function createContext(config: { imports: Module[] }): Promise<Context>;

interface Context extends AsyncDisposable {
  resolve<T>(token: Token<T>): T;
}
```

The headless primitive. Use for CLIs, queue consumers, cron, migrations, tests. Controllers in imported modules are silently ignored — the route registration step (step 5 below) is skipped.

### `createApp`

```ts
async function createApp(config: { imports: Module[] }): Promise<App>;

interface App extends Context {
  fetch(request: Request): Promise<Response>;
  use(mw: Middleware): void;
  use(prefix: string, mw: Middleware): void;
  onError(predicate: (e: unknown) => boolean, handler: (e: unknown, ev: Event) => Response | Promise<Response>): void;

  // For adapters
  router: Router;
  createEvent(request: Request, setup?: (event: Event) => void): Event;
}
```

`App extends Context` — code written against `Context` works against `App`.

### Boot sequence (fixed six-step order)

1. **Flatten** module graph; dedupe by identity.
2. **Validate** configuration: no cycles, no duplicate tokens, all `@Inject` targets visible. Fail fast.
3. **Construct** providers in topological order.
4. **Init** — call `init()` on each provider that has one, in topological order. If any throws, dispose already-initialized providers in reverse topological order and reject `createApp`.
5. **Register** routes via tsgonest-generated `registerXxxController(app)` calls (App only; skipped for Context).
6. **Resolve** the promise. `app.fetch` is now ready.

### Lifecycle interfaces (structural, duck-typed)

```ts
interface OnInit {
  init(): Promise<void> | void;
}

// AsyncDisposable comes from lib.esnext.disposable:
// interface AsyncDisposable { [Symbol.asyncDispose](): PromiseLike<void>; }
```

Container duck-types at runtime: `if (typeof p.init === 'function') await p.init();` and `if (typeof p[Symbol.asyncDispose] === 'function') await p[Symbol.asyncDispose]();`.

### Graceful shutdown

`app[Symbol.asyncDispose]()`:

1. Stop accepting new requests (adapter signals this to the platform).
2. Drain in-flight requests.
3. Run each provider's `[Symbol.asyncDispose]` in reverse topological order.
4. Resolve when clean.

The adapter wires the SIGTERM/SIGINT trigger; core does not install signal handlers.

### `await using` pattern

For short-lived contexts (CLI, scripts, tests):

```ts
await using ctx = await createContext({ imports: [...] });
// auto-disposed when scope exits, even on throw
```

Requires TS 5.2+ and Bun (or any runtime supporting TC39 explicit resource management).

---

## Compile-time extensions to tsgonest

These additions live in the existing `internal/analyzer/` and `internal/codegen/` Go packages. They are additive — existing NestJS support is untouched.

### Decorator vocabulary

Recognized via import path (`@mintkit/core`). Factory-wrapped decorators are not supported (existing tsgonest constraint).

- `@Controller(path: string)` — class decorator.
- HTTP verb decorators: `@Get(path?)`, `@Post(path?)`, `@Put(path?)`, `@Patch(path?)`, `@Delete(path?)`, `@Head(path?)`, `@Options(path?)`. Dynamic path args are warned and skipped (existing constraint).
- Parameter decorators: `@Body()`, `@Query()`, `@Param(name?)`, `@Headers(name?)`, `@Ctx(TOKEN)`.
- Middleware attach: `@UseMiddleware(mw)` on class or method.
- DI: `@Injectable({ scope?: 'singleton' | 'transient' })`.
- Param-level injection: `@Inject(TOKEN)` for non-class deps.

### Validation rules

- `@Ctx(TOKEN) name: T` — the parameter's declared `T` must match the token's phantom `T`. Compile-time diagnostic on mismatch.
- `@Body() data: { file: File & MaxSize<N> & MimeTypes<U> }` — file uploads typed declaratively. `File` and `FileStream` types from `@tsgonest/types`.

### Codegen output

In addition to the existing `*.tsgonest.js` companion file, controllers now emit:

```ts
// Generated, inside the controller's companion file
export function registerUserController(app: App): void {
  app.router.add('GET', '/users/:id', async (event) => {
    const ctrl = event.resolve(UserController);
    const id   = validateUUID(event.params.id);
    const q    = validateListQuery(event.query);
    const out  = await ctrl.findOne(id, q);
    return Response.json(serializeUserDto(out), {
      status: event.response.status || 200,
      headers: event.response.headers,
    });
  });
}
```

(Wrapper details: read validated params from generated validators, invoke controller method, serialize return via generated serializer, assemble final Response from `event.response.status` + `event.response.headers` + serialized body.)

User code calls `registerUserController(app)` at boot. Future improvement: auto-import via a generated `modules.gen.ts`.

### Framing-agnostic generator

The validator generator for `@Body()` should be parameterized over "source bytes" rather than "HTTP request body" — so the same generator can emit WS message validators when WS lands. No v1 code change required; the abstraction is "don't hardcode Request reads inside the validator emitter."

---

## File uploads

### Typed declarations

```ts
@Post('/avatar')
async setAvatar(@Body() data: {
  title: string & MinLength<1>;
  image: File & MaxSize<5_000_000> & MimeTypes<'image/png' | 'image/jpeg'>;
}) { ... }

@Post('/uploads/large')
async largeUpload(@Body() data: {
  filename: string;
  file: FileStream & MaxSize<5_000_000_000>;  // 5GB streaming
}) { ... }

@Post('/gallery')
async gallery(@Body() data: {
  title: string;
  photos: Array<File & MaxSize<10_000_000> & MimeTypes<'image/*'>>;
}) { ... }
```

### Runtime behavior

- **Buffered (`File`)** — parser calls `event.body.formData()`, extracts entries, validates `MaxSize`/`MimeTypes`, throws `TsgonestValidationError` on violation, passes `File` to handler. Web standard `File` extends `Blob`.
- **Streaming (`FileStream`)** — parser reads `event.body.stream()` as multipart in streaming mode, yields the file field as `FileStream { name, type, stream }`. `MaxSize` enforced incrementally; over-limit aborts the stream.
- **Array of files** — same parser, array result.

### New tag types (in `@tsgonest/types`)

- `MaxSize<N>`, `MinSize<N>` — byte limits.
- `MimeTypes<U>` — union of allowed MIME types; wildcard `'image/*'` etc. supported.
- `FileStream` — distinct from `File`; signals streaming parse to the codegen.

### New runtime code

A multipart streaming parser (~200 lines), shipped in `@mintkit/core`. Web `Request.formData()` is fine for the buffered case; the streaming case is what Mint adds.

---

## SSE

```ts
function sse<T>(generator: () => AsyncGenerator<T>): Response;

@Get('/events')
events(): Promise<Response> {
  return sse<{ type: 'tick'; ts: number }>(async function* () {
    while (true) {
      yield { type: 'tick', ts: Date.now() };
      await Bun.sleep(1000);
    }
  });
}
```

Sugar around a `ReadableStream` Response with `text/event-stream` content type and proper SSE framing (`data: ...\n\n`). Typed via `SSEStream<T>` so OpenAPI/SDK pick up the event type.

OpenAPI integration follows the existing `@Returns<T>` pattern in tsgonest.

---

## Bun adapter (vendored, shadcn-style)

Single package, copy-pasted into user repos. Owned and customizable by the user.

### `wrap(app)`

```ts
function wrap(app: App): { fetch: (req: Request, server: Server) => Promise<Response> };
```

For zero-config use, `app.fetch` alone is sufficient:

```ts
Bun.serve({ fetch: app.fetch, port: 3000 });
```

For Bun-specific features (the `Server` reference), use `wrap`:

```ts
const handler = wrap(app);  // injects BUN_SERVER token per-request
Bun.serve({ fetch: handler.fetch, port: 3000 });
```

### `BUN_SERVER` token

```ts
export const BUN_SERVER = defineToken<import('bun').Server>('BUN_SERVER');
```

Set by `wrap` on each event before `app.fetch` runs. Handlers/middleware can `event.require(BUN_SERVER)` to access the underlying server (e.g., for future WS upgrades or to inspect socket info).

### `gracefulShutdown`

```ts
function gracefulShutdown(app: App, opts?: {
  signals?:  Array<'SIGINT' | 'SIGTERM' | string>;
  timeout?:  number;  // ms; how long to wait for drain before hard-killing
}): void;
```

Wires the signal handlers, calls `app[Symbol.asyncDispose]()`, enforces the timeout. Vendored because signal handling philosophy varies (immediate vs infinite drain vs hard cap).

---

## What's reserved for WS (no v1 work, no breaking change at WS-add time)

Documented here so future maintainers know these are deliberate.

1. **Middleware/handler return type is `Response | Upgrade`** from v1. V1 never produces `Upgrade`; the union exists so widening is non-breaking.
2. **Router method is a free-form string.** `'WS'` slots in alongside HTTP verbs.
3. **`TokenStore` is a shared interface.** `Event` implements it; a future `Session` will too. No fragmentation of the context API when WS lands.
4. **Validator codegen is framing-agnostic.** The validator emitter doesn't hardcode "read from Request body"; it takes source bytes. Reusable for `@OnMessage` later.

Out of scope for v1: gateway decorators (`@Gateway`, `@OnMessage`, `@OnOpen`, `@OnClose`), session APIs (broadcast, rooms), AsyncAPI generation, reconnect/heartbeat patterns.

---

## Vendored ecosystem module conventions

Documented but not enforced. `tsgonest add <module>` (future CLI) scaffolds into:

```
src/modules/<name>/
  <name>.module.ts         # exports Module(opts) + ModuleAsync({ ... })
  <name>.service.ts        # @Injectable class(es)
  <name>.controller.ts     # @Controller — optional
  <name>.config.ts         # defineToken<Options>(...) + types
  <name>.types.ts          # DTOs
  <name>.middleware.ts     # any module-specific middleware
  README.md                # usage notes
```

The framework does not enforce this layout — users own the code. Ecosystem authors who follow it produce modules that feel native; deviants make their modules feel foreign.

---

## Out of scope (v1)

- WebSocket support
- Node, Cloudflare Workers, Deno, Lambda adapters
- Request scope / per-connection scope / child containers
- `forwardRef`
- String-keyed DI tokens; multi-providers; `useExisting`; function-shape providers
- Glob / auto-discovery
- AsyncAPI
- Runtime serializer overrides (type-tag customization only)
- `tsgonest add <module>` CLI (the module shape is specified; the scaffolder is a separate effort)
- Class-shaped middleware
- `defineGuard` and guard primitives
- `onRequest` / `onResponse` lifecycle hooks separate from middleware
- Cookies, CSRF, CORS, rate-limit, auth, session, observability, file-storage primitives in core (ship as vendored modules)
- Microservice transports (gRPC, NATS, RabbitMQ)
- NestJS migration codemod

---

## Testing strategy

Every module ships with tests. Tests exercise external behavior; internal helpers and intermediate state are not under test. Prior art lives in tsgonest's existing test infrastructure.

### Go unit tests

Continuing the `internal/<pkg>/<pkg>_test.go` pattern with fixtures under `testdata/`.

- **Decorator analyzer extension** — TS fixtures with the new decorators; assert the metadata model captures controllers, routes, parameter bindings, `@Ctx` token references; assert `@Ctx` type-mismatch produces a diagnostic. Mirrors `internal/analyzer/nestjs_test.go`.
- **Route registration codegen** — fixtures with annotated controllers; assert generated `registerXxxController` exports have correct method/path/wrapper shape. Mirrors `internal/codegen/codegen_test.go`.

### TypeScript unit tests (Vitest)

In `packages/mint`, alongside source or under `__tests__`.

- **Container** — register/resolve singleton, transient, class, useValue, useFactory. Topological order on construction. Reverse-topo on dispose. Cycle detection. Missing-token error.
- **Module resolver** — flat graph, nested imports, dedupe by identity, visibility (private-by-default, explicit `exports`, re-exports through chains), duplicate-token detection across modules, cycle detection across modules.
- **Router** — radix tree correctness, param extraction, method matching including `'WS'`, no-match.
- **Middleware chain** — onion composition, registration order, outer-to-inner across layers, short-circuit semantics, throw propagation, response wrapping after `await next()`.
- **Event** — body memoization (`bytes`/`text`/`json`/`formData` cache and stream single-use), `response.headers` is `Headers`, `response.status` mutable, token set/get/require, `waitUntil` delegation (mocked adapter).
- **Error mapper** — default `HttpError` mapping, default `TsgonestValidationError` → RFC 9457 (assert response shape, status, content-type), user-handler precedence, fallthrough 500 for unmatched.
- **App / Context boot** — six-step order via hook assertions, fast-fail on init throw with partial dispose, fast-fail on cycle, `Symbol.asyncDispose` drain semantics, `await using` lifecycle.

### E2E tests (Vitest, spawning the built binary)

Continuing the `e2e/compile.test.ts` pattern. Each scenario writes a TS app using framework decorators, runs `tsgonest build`, boots the resulting JS in a Bun subprocess, sends Web `Request`s via `fetch`, asserts response shapes/codes/headers.

Coverage:

- Validated `@Body`, `@Query`, `@Param`, `@Headers` round-trips.
- File upload (buffered single file, buffered multi-file, streaming file with `MaxSize` enforcement at byte boundary).
- SSE endpoint.
- Validation error response (RFC 9457).
- Thrown `HttpError` → status + body mapping.
- Custom `app.onError` precedence.
- Middleware ordering across all four attach points.
- Module-level middleware scoping (no cascade to imports).
- Headless `createContext` for a one-shot CLI script using `await using`.
- Graceful shutdown drains in-flight requests (start a long handler, signal, assert response completes and provider dispose runs in reverse-topo).

---

## Open questions / future work

These are intentionally not decided and are out of v1 scope.

- **Locked.** Brand is **Mint**. Core is `@mintkit/core` at `packages/mint`. Bun adapter is `@mintkit/bun` at `packages/mint-bun`.
- **NestJS migration codemod.** Decorator vocabulary aligns intentionally; codemod is a separate v2+ effort.
- **WebSocket support.** Reservations in place; full design when WS work begins.
- **Additional adapters.** Node and Workers are the obvious next targets; the core invariants support them without changes.
- **`tsgonest add <module>` CLI.** The module *shape* is specified; the scaffolder is a separate effort.
- **AsyncAPI generation.** Ships with WS support.
- **String tokens.** Can be added later if demand emerges.

---

## Provenance

This spec is the synthesis of a design conversation that worked through every decision in this document. The PRD ([`../.scratch/mint/PRD.md`](../.scratch/mint/PRD.md)) captures the user-facing goals; this spec captures the technical contracts. Implementation issues live under [`../.scratch/mint/issues/`](../.scratch/mint/issues/). The three are intended to be read together.
