# 08: Headless context (`createContext`, `await using`)

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

`createContext({ imports }): Promise<Context>` returns a `Context` with `resolve` and `Symbol.asyncDispose` — no router, no fetch, no middleware. Useful for CLIs, queue consumers, cron, migrations, tests. `App extends Context` so helpers written against `Context` work in `App` too. Controllers in modules used by `createContext` are silently ignored (no warnings).

`await using` is the canonical pattern for short-lived contexts; auto-dispose on scope exit, even on throw.

## Acceptance criteria

- [ ] `createContext({ imports })` exists and resolves to a `Context` with `resolve` and `Symbol.asyncDispose`.
- [ ] `Context` shares the same module/provider/lifecycle plumbing as `createApp` (no duplication).
- [ ] `App extends Context` structurally; helpers typed against `Context` work inside `createApp` returns.
- [ ] Controllers in imported modules are accepted but unused when boot is via `createContext`.
- [ ] E2E: a CLI fixture using `await using ctx = await createContext(...)` runs a service method and exits with provider dispose having fired.
- [ ] Unit tests cover `createContext` boot, `App extends Context` structural compatibility, and `await using` integration.

## Blocked by

- [06: Provider lifecycle and graceful shutdown](./06-lifecycle-and-shutdown.md)
