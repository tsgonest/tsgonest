# 12: Bun adapter (vendored package)

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

A single vendored package (intended to be copy-pasted into user repos under the shadcn philosophy) providing:

- `wrap(app): { fetch }` — for users who need access to Bun's `Server` instance. Injects `BUN_SERVER` token onto each event before `app.fetch` runs.
- `BUN_SERVER` token — typed reference to Bun's `Server` (e.g., for future WebSocket upgrades or socket inspection).
- `gracefulShutdown(app, opts?)` helper — wires SIGTERM/SIGINT (configurable) to `app[Symbol.asyncDispose]()` with a configurable drain timeout.

Zero-config users continue to write `Bun.serve({ fetch: app.fetch })`. The adapter is only needed when platform features are wanted.

## Acceptance criteria

- [ ] Vendored package exists with `wrap`, `BUN_SERVER`, and `gracefulShutdown` exports.
- [ ] `wrap(app)` returns a `{ fetch }` handler that injects `BUN_SERVER` on each event before delegating to `app.fetch`.
- [ ] `BUN_SERVER` token resolves to Bun's `Server` inside handlers/middleware via `event.require(BUN_SERVER)`.
- [ ] `gracefulShutdown` registers the configured signal handlers, calls `app[Symbol.asyncDispose]()`, enforces the timeout (hard-exits if drain exceeds).
- [ ] E2E: SIGTERM during in-flight requests drains them, fires provider dispose in reverse-topo, and exits cleanly within the timeout window.
- [ ] Documentation in the adapter's README walks through the canonical wiring and the customization points users are expected to edit.

## Blocked by

- [06: Provider lifecycle and graceful shutdown](./06-lifecycle-and-shutdown.md)
