# 04: Middleware and the token store

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

Function-only middleware shaped as `(event, next) => Promise<Response | Upgrade>`. Four attach points, all wiring into the same flat chain:

- `app.use(mw)` and `app.use('/prefix', mw)` — global and prefix-mounted.
- `defineModule({ middleware: [...] })` — applies to the module's declared controllers; no cascade to imports.
- `@UseMiddleware(mw)` on a controller class — every method in the controller.
- `@UseMiddleware(mw)` on a method — single route.

Execution is outer-to-inner across layers, registration order within each layer.

`Event` gains its full surface: token store (`set`/`get`/`require`), memoized `body` accessor with `bytes`/`text`/`json`/`formData` cached and `stream` single-use, mutable `response.headers` (Web `Headers` instance) and `response.status` (number), `waitUntil`, and `resolve` for DI access.

`@Ctx(TOKEN) name: T` parameter decorator extracts token-stored values into handler signatures with compile-time type checking against the token's `T`.

## Acceptance criteria

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

## Blocked by

- [01: Hello World](./01-hello-world.md)
- [03: DI and dynamic modules](./03-di-and-dynamic-modules.md)
