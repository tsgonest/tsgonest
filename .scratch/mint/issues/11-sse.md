# 11: SSE helper and OpenAPI integration

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

A core runtime helper `sse<T>(generator: () => AsyncGenerator<T>): Response` returns a streaming `Response` with `text/event-stream` content type and proper SSE framing (`data: <json>\n\n`). Typed via `SSEStream<T>` so the existing `@Returns<SSEStream<T>>` pattern in tsgonest picks up the event type for OpenAPI/SDK generation.

Handlers return `sse(async function* () { yield ...; })` or are typed as `Promise<SSEStream<EventType>>`.

## Acceptance criteria

- [ ] `sse()` helper exists in the runtime package and produces a correctly framed SSE response.
- [ ] `SSEStream<T>` type is recognized by the analyzer; OpenAPI emits the event payload schema using the existing `@Returns<T>` machinery.
- [ ] Generators can yield, await between yields, and close the stream by returning.
- [ ] Client disconnects propagate to the generator (abort signal in the response stream).
- [ ] E2E: connect a Bun client to an SSE endpoint, receive at least N events in framed form, disconnect, verify the generator cleanup runs.
- [ ] Unit tests cover framing correctness and abort propagation.

## Blocked by

- [02: Typed parameters and validated returns](./02-typed-parameters.md)
