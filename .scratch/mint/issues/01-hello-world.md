# 01: Hello World — compile-time → runtime → Bun seam

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md) — also see [`../../../plans/mint-spec.md`](../../../plans/mint-spec.md) (technical spec) and [`../../../plans/mint-plan.md`](../../../plans/mint-plan.md) (full plan).

## What to build

A user can write a single TypeScript file with one class decorated `@Controller('/hello')` containing one method decorated `@Get()` that returns a string. Running `tsgonest build` extends the existing pipeline to:

- Recognize Mint's decorators by import path (`@mintkit/core`).
- Emit a companion file with a `register…Controller(app)` export wiring the route into Mint's router.
- Include the route in the generated `openapi.json`.

The user writes a `main.ts` that creates a single module with that controller, calls `await createApp({ imports: [...] })`, hands `app.fetch` to `Bun.serve`, and `curl http://localhost:3000/hello` returns the string with a 200.

This phase establishes the entire compile-time → runtime seam end-to-end. Every subsequent phase is incremental thickening.

## Acceptance criteria

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

## Blocked by

None — can start immediately.
