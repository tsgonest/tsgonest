# 10: File uploads — streaming (`FileStream`, byte-level abort)

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

Users declare `@Body() data: { filename: string; file: FileStream & MaxSize<5_000_000_000> }`. `FileStream` is distinct from `File` and signals streaming parse. Codegen emits a streaming multipart parser (a runtime-shipped utility) that reads `event.body.stream()`, yields each field as it arrives, and aborts the connection when a `FileStream`'s incremental byte count exceeds `MaxSize`. The handler receives `FileStream { name, type, stream }` where `stream` is a `ReadableStream<Uint8Array>`.

## Acceptance criteria

- [ ] `FileStream` type exists and is recognized by the analyzer as distinct from `File`.
- [ ] A multipart streaming parser ships in the runtime (or framework package) and reads from a `ReadableStream<Uint8Array>` source.
- [ ] When any `FileStream` field exceeds its `MaxSize` mid-stream, the connection aborts and a 413 (or comparable problem-details) response is sent.
- [ ] Memory usage during a multi-GB upload stays bounded (verified via integration test or a documented design constraint).
- [ ] Mixed multipart (string fields + streaming files) parses correctly without buffering the entire body.
- [ ] E2E: stream a large file to disk via the handler, then assert the file on disk matches the input; oversize uploads abort early.
- [ ] Unit tests cover the parser's boundary detection, field dispatch, and abort semantics.

## Blocked by

- [09: File uploads — buffered](./09-file-uploads-buffered.md)
