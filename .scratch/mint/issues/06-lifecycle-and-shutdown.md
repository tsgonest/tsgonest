# 06: Provider lifecycle and graceful shutdown

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

Providers can implement `OnInit` (`init(): void | Promise<void>`) and/or `AsyncDisposable` (`[Symbol.asyncDispose](): PromiseLike<void>`). Container duck-types both at runtime. Boot runs `init()` after all construction, in topological order. If any `init()` throws, the framework disposes already-initialized providers in reverse topological order and rejects `createApp`. Cycles between providers throw at boot with the cycle path printed.

`app[Symbol.asyncDispose]()` stops accepting new requests, drains in-flight requests, then runs each provider's `Symbol.asyncDispose` in reverse topological order.

This issue nails the runtime lifecycle. The Bun-side SIGTERM trigger is wired in #12.

## Acceptance criteria

- [ ] `OnInit` interface is exported; providers implementing `init()` have it called after construction.
- [ ] `Symbol.asyncDispose` on providers is called on app shutdown.
- [ ] Construction order is topological; init order is topological; dispose order is reverse-topological.
- [ ] Failed `init()` triggers reverse-topo dispose of already-initialized providers and rejects `createApp`.
- [ ] Boot detects provider cycles and throws with the cycle path before any provider construction.
- [ ] `app[Symbol.asyncDispose]()` drains in-flight requests (long handlers complete) before disposing providers.
- [ ] Unit tests cover topological vs reverse-topological order, partial-dispose on init failure, and cycle detection.

## Blocked by

- [03: DI and dynamic modules](./03-di-and-dynamic-modules.md)
