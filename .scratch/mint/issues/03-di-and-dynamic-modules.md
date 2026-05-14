# 03: Dependency injection and dynamic modules

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

Users can mark classes `@Injectable()` and inject them via constructor. Providers can be declared in three forms within `defineModule({ providers: [...] })`:

- Class shorthand (the class is the token).
- `{ provide: TOKEN, useValue }`.
- `{ provide: TOKEN, useFactory, inject?: [...], scope?: 'singleton' | 'transient' }` — the factory's deps come from the same container.

Tokens are typed via `defineToken<T>(name)` and produce `Token<T>` instances with a phantom generic. Class references are themselves valid tokens. `@Inject(TOKEN)` parameter decorator injects non-class tokens into constructors. Container builds the dependency graph in topological order.

Dynamic modules are functions: `StorageModule(opts): Module` and `StorageModuleAsync({ inject, useFactory }): Module`. Both return the same plain-object module shape.

## Acceptance criteria

- [ ] `defineToken<T>(name)` returns a typed `Token<T>`; class refs are valid tokens.
- [ ] `@Injectable({ scope? })` decorator is recognized; default scope is singleton.
- [ ] All three provider forms register and resolve correctly.
- [ ] Constructor injection works for classes; `@Inject(TOKEN)` resolves non-class deps.
- [ ] Transient providers return a fresh instance on every resolve; singletons are cached.
- [ ] `useFactory` providers can declare async factories (`Promise<T>` return); await happens during boot.
- [ ] Dynamic module functions (sync and async-factory shapes) compose with `imports: [Module(opts), ModuleAsync({...})]` at the app level.
- [ ] E2E: a controller injects a service that depends on a useFactory-provided config token; the full chain resolves and the endpoint returns config-derived data.
- [ ] Unit tests cover topological construction order, transient vs singleton semantics, and all three provider forms.

## Blocked by

- [01: Hello World](./01-hello-world.md)
