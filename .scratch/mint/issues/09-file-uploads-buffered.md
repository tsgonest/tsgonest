# 09: File uploads — buffered (`File`, `MaxSize`, `MimeTypes`)

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

Users declare `@Body() data: { title: string; image: File & MaxSize<5_000_000> & MimeTypes<'image/png' | 'image/jpeg'> }` on a handler. tsgonest analyzer recognizes the `File` type and the new tag constraints. Codegen generates a multipart parser that reads `event.body.formData()`, extracts each named entry, validates `File` entries against `MaxSize`/`MinSize`/`MimeTypes`, and throws `TsgonestValidationError` on violation. Handler receives Web standard `File` instances.

New tag types in `@tsgonest/types`: `MaxSize<N>`, `MinSize<N>`, `MimeTypes<U>` (with wildcard support like `'image/*'`).

## Acceptance criteria

- [ ] `File`, `MaxSize<N>`, `MinSize<N>`, `MimeTypes<U>` types are added to `@tsgonest/types` (or runtime types) and recognized by the analyzer.
- [ ] Wildcard MIME patterns (`'image/*'`) match correctly.
- [ ] `@Body()` with file fields parses multipart via `event.body.formData()`, validates per-file constraints, and passes `File` instances to the handler.
- [ ] Validation failure produces an RFC 9457 problem-details response with per-field detail.
- [ ] Array fields (`photos: Array<File & ...>`) parse and validate as a list.
- [ ] OpenAPI emits `multipart/form-data` request body schemas for these routes.
- [ ] E2E: round-trip a buffered file upload (valid → 200, oversize → 400, wrong MIME → 400).
- [ ] Analyzer/codegen unit tests cover the new types and the generated parser shape.

## Blocked by

- [02: Typed parameters and validated returns](./02-typed-parameters.md)
