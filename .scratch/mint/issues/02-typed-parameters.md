# 02: Typed parameters and validated returns

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

The user can declare `@Body() body: SomeDto`, `@Query() q: ListQuery`, `@Param('id') id: string & tags.UUID`, and `@Headers('x-foo') header: string` on handler methods. tsgonest's existing validator/serializer generation is reused: parameter types feed the existing analyzer; the codegen's `register…Controller(app)` wrapper invokes the validators before calling the handler and the serializer on the return value. Validation failures throw `TsgonestValidationError`; the framework's error mapper converts that to RFC 9457 `application/problem+json` with per-field detail.

OpenAPI emits request/response schemas for these parameters from the same type metadata.

## Acceptance criteria

- [ ] `@Body`/`@Query`/`@Param`/`@Headers` parameters are extracted from the request and validated against their TypeScript types before the handler runs.
- [ ] Validators and serializers are the existing tsgonest-generated ones (no fork).
- [ ] Return values are serialized via generated fast-JSON serializers; response `Content-Type` is `application/json`.
- [ ] Validation failure produces a `400 application/problem+json` response with `type`/`title`/`status`/`detail`/`errors` per RFC 9457.
- [ ] OpenAPI generates request/response schemas matching the declared types.
- [ ] E2E: round-trip a typed body (valid → 200 with serialized return, invalid → 400 with problem-details).
- [ ] Unit tests cover analyzer recognition of each parameter decorator and codegen wrapper shape.

## Blocked by

- [01: Hello World](./01-hello-world.md)
