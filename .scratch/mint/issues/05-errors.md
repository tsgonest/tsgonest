# 05: HttpError hierarchy and pluggable error mapper

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

The `HttpError` hierarchy is shipped: `BadRequestError`, `UnauthorizedError`, `ForbiddenError`, `NotFoundError`, `ConflictError`, `UnprocessableEntityError`, `TooManyRequestsError`, `InternalServerError` — each carries a status and a JSON body. Default error mapper maps `HttpError` subclasses to JSON responses with the carried status, maps `TsgonestValidationError` to RFC 9457 problem-details (already in #02), and falls through to status 500 for unknown errors (with the full error logged, not leaked to the response).

`app.onError(predicate, handler)` registers user-defined mappings. User handlers take precedence over defaults; first match wins.

## Acceptance criteria

- [ ] `HttpError` base class and all listed subclasses exist with correct status codes.
- [ ] Throwing an `HttpError` subclass from a handler produces a Response with the carried status and body.
- [ ] `app.onError` registered handlers run before defaults; first match wins.
- [ ] Unmatched throws produce a 500 problem-details JSON; the underlying error is logged.
- [ ] E2E: a controller throws a custom domain error; an `app.onError` registration maps it to a `409 Conflict`.
- [ ] Unit tests cover precedence between user handlers and defaults, and the 500 fallthrough.

## Blocked by

- [02: Typed parameters and validated returns](./02-typed-parameters.md)
