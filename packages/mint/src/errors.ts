/**
 * HTTP error hierarchy. Throwing one of these from a handler or middleware is
 * the supported way to produce non-2xx responses without writing a Response by
 * hand. The error mapper maps each subclass to JSON with the carried `status`.
 *
 * `body` is the JSON payload; if `undefined`, the default mapper falls back to
 * `{ error: message }`.
 */
export class HttpError extends Error {
  constructor(
    public readonly status: number,
    public readonly body?: unknown,
    message?: string,
  ) {
    super(message ?? `HTTP ${status}`)
    this.name = new.target.name
  }
}

export class BadRequestError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(400, body, message)
  }
}

export class UnauthorizedError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(401, body, message)
  }
}

export class ForbiddenError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(403, body, message)
  }
}

export class NotFoundError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(404, body, message)
  }
}

export class ConflictError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(409, body, message)
  }
}

export class UnprocessableEntityError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(422, body, message)
  }
}

export class TooManyRequestsError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(429, body, message)
  }
}

export class InternalServerError extends HttpError {
  constructor(body?: unknown, message?: string) {
    super(500, body, message)
  }
}
