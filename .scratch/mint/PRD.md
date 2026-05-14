# PRD: Mint — tiny web framework for Bun, built on tsgonest

Status: ready-for-agent

> Brand: **Mint**. Core package: `@mintkit/core`. Bun adapter package: `@mintkit/bun`.
> Companion technical spec: [`../../plans/mint-spec.md`](../../plans/mint-spec.md)
> Implementation plan: [`../../plans/mint-plan.md`](../../plans/mint-plan.md)
> Implementation issues: [`./issues/`](./issues/)

## Problem Statement

I (a backend developer) am hitting a wall with NestJS + Node. The NestJS mental model (DI, modules, Angular-style organization) fits my brain, but the rest of the stack drags: pipes/interceptors/HttpException ceremony, opaque factory-style decorators that defeat static analysis, runtime baggage inherited from Express, and a request-scoped DI model I've never wanted. I want to keep the parts that work — module system, dependency injection, decorator-driven routing — and discard everything else. Bun is my deployment target; I don't care about Node, Workers, or Deno today, but I don't want the framework to paint itself into a single-platform corner. I need to keep tsgonest's compile-time benefits: validated `@Body`/`@Query`/`@Param`/`@Headers` parameters, OpenAPI 3.2 generation, and SDK generation from TypeScript types.

## Solution

A small web framework whose entire runtime is: a DI container (singleton + transient scope only, no request scope), a module system, a radix router, an `Event` abstraction over Web `Request`/`Response`, an onion-style middleware chain, and a lifecycle subsystem. Decorators (`@Controller`, `@Get`, `@Body`, etc.) are compile-time sugar — at runtime, the framework sees plain function calls that register routes. The runtime depends on no decorator-metadata reflection. Everything beyond core primitives (auth, observability, file storage, rate limiting, CORS, cookies) ships as shadcn-style vendored modules that the user owns and customizes inside their own repo. tsgonest's existing compile-time pipeline is extended (not replaced) to recognize the new framework's decorators and emit registration calls in the generated `*.tsgonest.js` companion files. Web standards are the lingua franca: `Request`, `Response`, `Headers`, `File`, `FormData`, `ReadableStream`. Bun is the only adapter shipped in v1; the design reserves four micro-extensions (middleware return type widened to `Response | Upgrade`, router method slot as a free-form string, token-store as a shared interface, framing-agnostic validator codegen) so that WebSockets and other platform adapters slot in additively rather than as breaking changes.

## User Stories

1. As a backend dev, I want to define a service with constructor-injected dependencies, so that I can write business logic without manually wiring imports.
2. As a backend dev, I want to declare a controller with route decorators, so that I can group endpoints by domain.
3. As a backend dev, I want my `@Body` parameter validated against its TypeScript type at request time, so that I never write manual validation.
4. As a backend dev, I want validation errors to return a standards-compliant problem-details JSON response, so that clients can parse errors uniformly.
5. As a backend dev, I want to define a module with `imports`, `providers`, `exports`, `controllers`, and module-level `middleware`, so that I can organize features by domain boundary.
6. As a backend dev, I want to declare a dynamic module as a function returning a static module shape, so that I can pass configuration at boot.
7. As a backend dev, I want an async-factory provider with `inject`/`useFactory`, so that I can resolve config from another provider before constructing a downstream service.
8. As a backend dev, I want providers to support an `init()` hook, so that I can open database connections and warm caches before traffic starts.
9. As a backend dev, I want providers to support `Symbol.asyncDispose`, so that on graceful shutdown I can close resources via the TC39 standard.
10. As a backend dev, I want the framework to fail-fast on dependency cycles between providers, so that I never debug runtime DI ordering issues.
11. As a backend dev, I want the framework to fail-fast if any provider's `init()` throws, so that I never serve traffic with a partially-initialized app.
12. As a backend dev, I want middleware shaped as `(event, next) => Promise<Response | Upgrade>`, so that I compose cross-cutting concerns with async/await rather than Express callbacks.
13. As a backend dev, I want `app.use(mw)` and `app.use('/prefix', mw)`, so that observability and logging attach app-wide or to URL prefixes.
14. As a backend dev, I want `defineModule({ middleware: [...] })` to bind middleware to that module's declared controllers without cascading to imports, so that vendored modules are self-contained and predictable.
15. As a backend dev, I want `@UseMiddleware(mw)` on a controller class or method, so that I attach middleware to a specific scope without route-config noise.
16. As a backend dev, I want middleware to `await next()` and modify the downstream response, so that I can wrap handlers (timing, headers, transforms).
17. As a backend dev, I want middleware to throw to short-circuit, so that auth failures stop the chain without callback contortions.
18. As a backend dev, I want middleware to pass data downstream via typed tokens (`event.set(USER, u)`, `event.require(USER)`), so that handlers receive typed context without `declare module` augmentation.
19. As a backend dev, I want to define typed context tokens with `defineToken<User>(...)`, so that set/get sites are type-checked at compile time.
20. As a backend dev, I want `event.request` to be the raw Web `Request`, so that I work with headers, method, and URL via standards I already know.
21. As a backend dev, I want `event.body.bytes()` / `text()` / `json()` / `formData()` to memoize, so that middleware and the handler can both read the body without manual caching.
22. As a backend dev, I want `event.body.stream()` to return the underlying ReadableStream once, so that I can pipe large payloads to disk without buffering.
23. As a backend dev, I want `event.response.headers` (a real `Headers` instance) and `event.response.status` (a number), so that I work with the Web platform API directly.
24. As a backend dev, I want file uploads typed as `File` (buffered) or `FileStream` (streaming) with `MaxSize<N>`, `MinSize<N>`, `MimeTypes<U>` tag constraints, so that uploads validate at the byte level without a multer/decorator pipeline.
25. As a backend dev, I want streaming uploads to abort the connection at the configured byte limit, so that abuse cannot fill server memory.
26. As a backend dev, I want SSE handlers via `sse(async function* () { yield ...; })`, so that I can implement server-sent events without writing the framing code.
27. As a backend dev, I want errors to be thrown (not returned as `Result`), so that I use try/catch and async/await naturally.
28. As a backend dev, I want a typed `HttpError` hierarchy (`BadRequestError`, `NotFoundError`, `UnauthorizedError`, `ForbiddenError`, `ConflictError`, `InternalServerError`, …), so that I express HTTP semantics by throwing.
29. As a backend dev, I want to register custom mappings via `app.onError`, so that domain exceptions map to HTTP responses outside the domain layer.
30. As a backend dev, I want `event.waitUntil(promise)` for after-response work, so that logging/metrics run after responding without blocking and remain portable across platforms.
31. As a backend dev, I want all framework decorators to be fully erased at runtime, so that nothing depends on `reflect-metadata`.
32. As a backend dev, I want `@Ctx(TOKEN)` on handler parameters, so that I declare typed context dependencies per-handler with type checking against the token's `T`.
33. As a backend dev, I want `createContext({ imports })` for headless apps, so that I can use DI/modules for CLIs, queue consumers, cron, and migrations without HTTP.
34. As a backend dev, I want headless contexts to support `await using`, so that short-lived CLI tools auto-dispose on scope exit.
35. As a backend dev, I want `App extends Context`, so that helpers written against `Context` work inside HTTP apps too.
36. As a backend dev, I want decorator metadata to be statically analyzable by the existing tsgonest analyzer, so that no new infrastructure is needed for validators or OpenAPI.
37. As a backend dev using Bun, I want a vendored `wrap(app)` adapter plus a vendored `gracefulShutdown(app, { signals, timeout })` helper, so that I can serve traffic and shut down cleanly on SIGTERM.
38. As a backend dev, I want a `BUN_SERVER` token from the Bun adapter, so that I can access Bun's `Server` instance for advanced cases (e.g., future WS upgrades) without leaking platform types into core.
39. As an ecosystem module author, I want to ship a shadcn-style vendored module with its own typed tokens, so that consumers import named tokens rather than augmenting global types.
40. As an ecosystem module author, I want to declare module-level middleware that auto-attaches to my module's controllers, so that vendoring my module gives users the expected behavior without per-route wiring.
41. As an ecosystem module author, I want to ship both `Module(opts)` and `ModuleAsync({ inject, useFactory })` shapes, so that consumers configure synchronously or via injection.
42. As an ecosystem module author, I want the module-shape conventions (`*.module.ts`, `*.service.ts`, `*.controller.ts`, `*.config.ts`, `*.types.ts`, `*.middleware.ts`) to be documented but not enforced, so that my recipes feel native while users retain full ownership.
43. As a future maintainer, I want WS support to require no breaking changes to the middleware return type, so that I can add WebSockets later additively.
44. As a future maintainer, I want the router's method slot to accept arbitrary strings (`'GET' | 'POST' | ... | 'WS'`), so that WS routes register through the existing matcher.
45. As a future maintainer, I want the token-store interface shared by `Event` and a future `Session`, so that adding WS does not fragment the context API.
46. As a future maintainer, I want compile-time validator generation to be framing-agnostic, so that the same generator emits HTTP body validators today and WS message validators later.
47. As an ops engineer, I want graceful shutdown to drain in-flight requests before disposing providers, so that no request dies mid-flight on deploys.
48. As an ops engineer, I want `Symbol.asyncDispose` to run providers in reverse topological order, so that downstream services aren't torn down before their dependents.
49. As a CI engineer, I want fast-fail boot semantics, so that bad config or missing dependencies surface at deploy rather than under load.
50. As a backend dev, I want the boot sequence documented as a fixed six-step order, so that I know where to hang startup logic.
51. As a backend dev, I want explicit module imports (no glob auto-discovery), so that the dependency graph is grep-able and reproducible.
52. As a backend dev, I want providers private to their declaring module by default, so that I get encapsulation without ceremony.
53. As a backend dev, I want to opt providers into cross-module visibility via `exports`, so that the API surface of each module is explicit.
54. As a backend dev, I want re-exports through import chains to be explicit, so that there are no surprise leaks.
55. As a backend dev, I want serializer behavior customizable via type tags (`Date & DateFormat<'iso'>`), so that customization stays at the type-system layer and the compile-time guarantee holds.
56. As a backend dev, I want OpenAPI 3.2 + SDK generation to include all framework routes automatically, so that I never write API docs by hand.
57. As a backend dev, I want middleware to access DI via `event.resolve(Token)`, so that middleware can use injected services without class wrappers.
58. As a backend dev, I want only function-shaped middleware (no class middleware), so that I have a single mental model for composition.
59. As a backend dev, I want no built-in `defineGuard` sugar, so that guards are just middleware and arrive via the ecosystem rather than core API surface.
60. As a backend dev, I want the framework footprint to remain small (just container, router, event, middleware chain, lifecycle), so that my bundle stays small and the framework doesn't dictate my architecture.
61. As a backend dev, I want shadcn-style ecosystem modules (auth, storage, observability) installed via copy-paste into my repo, so that I own and customize them rather than depending on a versioned third-party package.
62. As a backend dev, I want a strict middleware order (registration order at each layer; outer-to-inner across `app` → module → controller → method), so that I never have to memorize precedence rules.
63. As a backend dev, I want `event.get(TOKEN)` to return `T | undefined` and `event.require(TOKEN)` to return `T` or throw, so that I have explicit handling for optional vs required context.
64. As a backend dev, I want the headless `Context` API to expose only `resolve` and `Symbol.asyncDispose`, so that the surface is minimal and discoverable.
65. As a backend dev, I want no string-key DI tokens in v1, so that all DI is type-safe; string tokens can be added later if real demand emerges.

## Implementation Decisions

### Modules (13)

The framework decomposes into 13 modules. Each is sized to be testable in isolation; the first four are deep modules (encapsulated complexity, simple interface).

**Deep core modules:**

1. **Container.** Token-typed DI. `Token<T>` is a class with a phantom generic carrying the value type; `defineToken<T>(name)` produces one. Two provider forms: class shorthand (`UserService`) and object form (`{ provide: TOKEN, useValue }` or `{ provide: TOKEN, useFactory, inject }`). Singleton (default) and transient scope only. Topological construction order. Cycle detection at boot. No `useExisting`, no multi-providers, no string tokens (v1). Resolves via `container.resolve(token)`; class refs are tokens for free.
2. **Module resolver.** Flattens import graph, dedupes by identity, validates: no duplicate provider tokens across the graph, no cycles between providers, every `@Inject(TOKEN)` resolves to a visible provider. Enforces visibility — providers are private unless listed in `exports`; controllers are always globally registered.
3. **Router.** Radix tree. Method slot is a free-form string (`'GET'`, `'POST'`, …, `'WS'`). Param extraction by name. Returns route metadata (the registered handler reference, params).
4. **Middleware chain.** Onion-style composition. Four attach points: app (global or prefix), module (declaration-site, no cascade to imports), controller (`@UseMiddleware` on class), method (`@UseMiddleware` on method). Execution order: outer-to-inner across layers, registration order within each layer. `await next()` returns the downstream Response. Throwing skips to the error mapper. Handlers and middleware return `Promise<Response | Upgrade>`; v1 produces no `Upgrade` values, but the union is reserved for future WS without breaking changes.

**Runtime state modules:**

5. **Event.** Per-request state object. `event.request: Request` (raw Web Request). `event.body` is a memoized accessor: `bytes()`/`text()`/`json()`/`formData()` read once and cache; `stream()` returns the raw `ReadableStream` once. `event.response = { headers: Headers, status: number }` — mutated directly via the Web `Headers` API and a plain number. Token store: `event.set(token, value)`, `event.get(token): T | undefined`, `event.require(token): T`. `event.waitUntil(promise)` for after-response work; delegates to platform via adapter. `event.resolve(token)` for DI access from middleware.
6. **Error mapper.** Pluggable. Default handlers map `HttpError` subclasses to their status + JSON body, map `TsgonestValidationError` to RFC 9457 `application/problem+json` (status 400, `type`/`title`/`detail`/per-field errors), and fall through to a 500 mapper for unhandled errors. Users register additional handlers via `app.onError(predicate, handler)`.
7. **App / Context factory.** `createContext({ imports })` is the headless primitive; `createApp({ imports })` extends it with router + middleware chain + `fetch`. Both are async. `App extends Context`. The boot sequence is fixed:
   1. Flatten module graph; dedupe by identity.
   2. Validate configuration (cycles, duplicate tokens, missing exports). Fail fast.
   3. Construct providers in topological order (deps before dependents).
   4. Run `init()` on each provider that implements it, in topological order. If any throws, dispose already-initialized providers in reverse topological order and reject.
   5. Register all controllers' routes via tsgonest-generated `registerXxxController(app)` calls (app only, skipped for context-only).
   6. Resolve `createApp` / `createContext`. App is `fetch`-ready.

   `Symbol.asyncDispose` drains in-flight requests, then runs each provider's `Symbol.asyncDispose` in reverse topological order.

**Compile-time extensions to tsgonest:**

8. **Decorator analyzer extension.** Recognizes the framework's decorators: `@Controller(path)`, the HTTP verb decorators, `@Body`, `@Query`, `@Param`, `@Headers`, `@Ctx`, `@UseMiddleware`. Resolves them through TypeScript's checker the same way the existing NestJS analyzer does — supporting import aliases, blocking factory-wrapped decorators (per the existing tsgonest constraint). Records controller/route metadata into the existing metadata model. Validates that `@Ctx(TOKEN) name: T` parameters type-match the token's `T`.
9. **Route registration codegen.** Generates per-controller companion exports inside the existing `*.tsgonest.js` files: `registerXxxController(app)` functions that call `app.router.add(method, path, wrappedHandler)`. The wrapped handler reads validated parameters from the event, invokes the controller method, and serializes the return value. Reuses the existing companion-emission infrastructure; the new code is additive (current controller-skip logic gives way to controller-registration emission).

**Runtime utilities:**

10. **`sse()` helper.** `sse(generator: () => AsyncGenerator<T>)` returns a streaming `Response` with `text/event-stream` content type and proper framing. Typed via `SSEStream<T>` so OpenAPI/SDK can pick up the event type.
11. **`HttpError` hierarchy.** `class HttpError extends Error` carries `status: number` and `body: unknown`. Subclasses: `BadRequestError`, `UnauthorizedError`, `ForbiddenError`, `NotFoundError`, `ConflictError`, `UnprocessableEntityError`, `TooManyRequestsError`, `InternalServerError`. Default error mapper handles all of them.
12. **Public primitives.** `defineToken`, `defineModule`, `defineHandler`, `defineMiddleware` — typed pass-throughs for inference; exist to give users one canonical surface for each concept.

**Vendored adapter (single package, copy-paste into user repo):**

13. **Bun adapter.** Single vendored package providing `wrap(app)` (passes `app.fetch` to `Bun.serve` plus per-request token injection), `BUN_SERVER` token (typed Bun `Server` reference, set on each event by the adapter), and `gracefulShutdown(app, { signals, timeout })` helper that wires SIGTERM/SIGINT to `app[Symbol.asyncDispose]()` with a configurable drain timeout.

### Architectural decisions

- **Web standards as the lingua franca.** `Request`, `Response`, `Headers`, `File`, `Blob`, `FormData`, `ReadableStream`. No framework-specific wrappers for things the platform already provides. The narrow exceptions: `event.body` (memoized over `Request.body`) to solve the once-only-read footgun, and the framework's own `Event`/`Token`/lifecycle primitives.
- **Decorators are pure compile-time anchors.** Erased at runtime. tsgonest's analyzer reads them; the runtime sees only function calls registering routes. No `reflect-metadata`.
- **Throw-based error contract.** No `Result` types in core. `app.onError(predicate, handler)` is the extensibility point. RFC 9457 problem-details is the default response shape; users can override.
- **Singleton + transient only.** No request scope, no per-connection scope. Per-request state lives on the event (via tokens or `event.set/get`). This is the single biggest source of complexity in Nest's DI that we deliberately omit.
- **Module-level middleware binds at declaration site.** No cascade to imported modules' controllers. Imported modules carry their own bindings.
- **Boot is async, fail-fast, fully torn down on partial failure.** No partial-init state.
- **No `forwardRef`.** Cycles error at boot.
- **Headless context is the primitive; HTTP is a layer.** `createContext` returns a `Context` with `resolve` + `Symbol.asyncDispose`. `createApp` extends with router/middleware/`fetch`. `await using` is the canonical short-lived pattern.
- **WS reservations.** Four micro-extensions ship in v1 even though WS itself does not: (a) middleware/handler return type is `Response | Upgrade`; (b) router method is a free-form string; (c) token store is an interface that `Event` and future `Session` both implement; (d) compile-time validator codegen is framing-agnostic, ready to emit message validators alongside body validators.

### API contracts (key shapes, prose where possible)

- `Token<T>` is a phantom-typed class; `defineToken<T>(name): Token<T>`.
- `defineModule({ imports?, providers?, exports?, controllers?, middleware? })` — plain object pass-through; no class form.
- `Provider = ClassConstructor | { provide: Token, useValue } | { provide: Token, useFactory, inject }`.
- `Lifecycle: OnInit | AsyncDisposable | both` — structural interfaces, duck-typed at runtime.
- `Middleware = (event: Event, next: () => Promise<Response | Upgrade>) => Promise<Response | Upgrade>`.
- `Event = { request, body, response, set, get, require, waitUntil, resolve }`.
- `createContext({ imports }): Promise<Context>`; `Context = { resolve, [Symbol.asyncDispose] }`.
- `createApp({ imports }): Promise<App>`; `App extends Context` with additional `fetch(request): Promise<Response>`, `use(mw)` / `use(prefix, mw)`, `onError(predicate, handler)`, internal `router` and `createEvent` for adapters.
- HTTP error classes carry `status: number` and `body: unknown`.

### Build / packaging decisions

- The framework's core lives at `packages/mint` (npm name `@mintkit/core`) within the existing pnpm workspace; `packages/core` is already in use by the `tsgonest` npm package. The Bun adapter lives at `packages/mint-bun` (npm name `@mintkit/bun`). Tsgonest analyzer + codegen changes happen in the existing Go module (`internal/analyzer`, `internal/codegen`).
- TypeScript 5.2+ required at the consumer side (for `using` keyword).
- Bun is the only v1 runtime target.

## Testing Decisions

A good test exercises external behavior, not implementation details. Tests should be written against each module's public interface; internal helpers and intermediate state are off-limits. Tests must remain valid through reasonable refactors of internals.

All 13 modules require tests. Prior art lives in the existing tsgonest test infrastructure:

**Go unit tests** (continuing the `internal/*/*_test.go` pattern with fixtures in `testdata/`):
- **Decorator analyzer extension** — fixtures with TS source containing the new decorators; assert the metadata model captures controllers, routes, parameter bindings, `@Ctx` token references. Mirrors `internal/analyzer/nestjs_test.go`.
- **Route registration codegen** — fixtures with annotated controllers; assert the generated `registerXxxController` exports have correct method/path/handler shape. Mirrors `internal/codegen/codegen_test.go`.

**TypeScript unit tests** (Vitest, located in the new framework package's `__tests__` or alongside source per existing `packages/runtime/test` pattern):
- **Container** — register/resolve for singleton, transient, class, useValue, useFactory; topological order on construction; reverse-topo on dispose; cycle detection; missing-token error.
- **Module resolver** — flat graph, nested imports, dedupe by identity, visibility (private-by-default, explicit `exports`, re-exports through chains), duplicate-token detection, cycle detection across modules.
- **Router** — radix tree correctness, param extraction (single, multiple, wildcard if supported), method matching (`GET`/`POST`/`WS`/etc.), no-match behavior.
- **Middleware chain** — onion composition, registration order at each layer, outer-to-inner across layers, short-circuit (no `next()` call), throw propagation, response wrapping (`await next()` then modify).
- **Event** — body memoization (`bytes`/`text`/`json`/`formData` cache; `stream` single-use), `response.headers` is a Headers instance, `response.status` is mutable, token set/get/require, `waitUntil` delegation (mocked).
- **Error mapper** — default `HttpError` mapping, default `TsgonestValidationError` → RFC 9457 mapping (assert response shape, status, content-type), user-registered handler precedence, fallthrough 500 for unmatched.
- **App / Context boot** — six-step order via assertions on hook invocations, fast-fail on init throw (assert partial dispose in reverse-topo), fast-fail on cycle, `Symbol.asyncDispose` drain semantics.

**E2E tests** (continuing the `e2e/compile.test.ts` pattern in this repo — spawn the built tsgonest binary against a controller fixture, then run the emitted JS in a Bun subprocess):
- Full pipeline: write a TS app using framework decorators, build via `tsgonest build`, boot the resulting JS in Bun, send Web `Request`s via `fetch`, assert response shapes/codes/headers.
- Coverage: validated body/query/param/header, file upload (buffered and streaming), SSE response, validation error response (RFC 9457), thrown `HttpError` mapping, custom `onError`, middleware ordering across all four attach points, module-level middleware scoping, headless context for a one-shot CLI script (via `await using`).

## Out of Scope

- WebSocket support (deferred; reservations only).
- Node, Cloudflare Workers, Deno adapters (Bun-only in v1).
- Request-scoped DI; per-connection scope; child containers.
- `forwardRef` / circular-dependency resolution mechanisms.
- String-keyed DI tokens; multi-providers; `useExisting`; function-shaped provider declarations.
- Glob / auto-discovery of modules and controllers.
- AsyncAPI generation (will accompany WS when added).
- Runtime serializer-override APIs (customization stays at type-tag level).
- A `tsgonest add <module>` CLI (the shadcn-vendored module *shape* is specified; the CLI to scaffold modules into a user's repo is a separate effort).
- Class-shaped middleware.
- `defineGuard` / guard primitives. Guards are middleware; ecosystem modules ship them.
- `onRequest` / `onResponse` lifecycle hooks separate from middleware.
- Cookies API in core (ships as a vendored module).
- CSRF, CORS, rate-limit, auth, session, observability, file-storage primitives (all ship as vendored ecosystem modules).
- Microservice transport adapters (gRPC, NATS, RabbitMQ) — orthogonal effort; design preserves the flexibility for users to roll their own on top of the headless `createContext`.

## Further Notes

- **Naming.** Brand is **Mint**. Core package is `@mintkit/core` (workspace dir `packages/mint`). Vendored Bun adapter is `@mintkit/bun` (workspace dir `packages/mint-bun`). Internal docs may refer to "Mint" interchangeably with "the framework."
- **Relationship to tsgonest.** This is a new framework that depends on tsgonest as a compile-time tool. tsgonest's analyzer and codegen pipelines are extended (additively) to recognize the framework's decorators and emit the registration calls. No changes to tsgonest's existing NestJS support — this is a parallel decorator vocabulary, identified via import path.
- **Migration story from NestJS.** Out of scope as a v1 deliverable, but the decorator surface (`@Controller`, `@Get`, `@Body`, `@Param`, `@Query`, `@Headers`, `@Injectable`, `@Module`) is intentionally aligned so a future codemod is straightforward. Differences a migration must address: no pipes/interceptors/guards, no `HttpException` (replaced by `HttpError` hierarchy), no `forwardRef`, no request scope, no `DynamicModule` class (replaced by module functions).
- **TC39 alignment.** The framework leans on stage-3 / shipped TC39 features: `Symbol.asyncDispose`, `using`/`await using`. TypeScript 5.2+ required at the consumer.
- **Shadcn philosophy commitment.** Once shipped, core primitives (middleware shape, event API, container, router) change rarely and with care. Every vendored ecosystem module assumes the v1 contract.
- **Conversation provenance.** This PRD is synthesized from a design conversation that worked through DI style, transport primitive, module shape, lifecycle semantics, middleware shape and the four attach points, error contract, response mutation, after-response work, body parsing edges, file uploads, headless contexts, adapter contract, validation/serialization edges, and WebSocket reservations. The full decision trail and trade-off rationales are captured implicitly in the Implementation Decisions section.
