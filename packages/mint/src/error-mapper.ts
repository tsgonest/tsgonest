import { HttpError } from './errors'
import type { Event } from './event'

export type ErrorPredicate = (err: unknown) => boolean
export type ErrorMapHandler = (err: unknown, event: Event) => Response | Promise<Response>

export interface ErrorHandler {
  predicate: ErrorPredicate
  handle: ErrorMapHandler
}

/**
 * Pluggable throw → Response mapper. User-registered handlers run BEFORE the
 * built-in defaults; first match wins.
 *
 * Defaults:
 *  - `HttpError` subclass → JSON Response with the carried status + body (or
 *    `{ error: message }` if `body` is undefined).
 *  - Validation errors (soft-detected by `err.name === 'TsgonestValidationError'`)
 *    → RFC 9457 `application/problem+json` at status 400.
 *  - Unknown throws → 500 problem-details JSON with no message leak; the full
 *    error is logged via `console.error`.
 *
 * The soft name-check avoids a hard dependency on `@tsgonest/runtime` so
 * `@mintkit/core` stays dep-free.
 */
export class ErrorMapper {
  private readonly handlers: ErrorHandler[] = []

  register(predicate: ErrorPredicate, handle: ErrorMapHandler): void {
    this.handlers.push({ predicate, handle })
  }

  async map(err: unknown, event: Event): Promise<Response> {
    for (const h of this.handlers) {
      if (h.predicate(err)) return await h.handle(err, event)
    }
    return defaultMap(err, event)
  }
}

function defaultMap(err: unknown, _event: Event): Response {
  if (err instanceof HttpError) {
    const body = err.body !== undefined ? err.body : { error: err.message }
    return jsonResponse(body, err.status, 'application/json')
  }
  if (isValidationError(err)) {
    const detail = err instanceof Error ? err.message : String(err)
    const errors = readValidationErrors(err)
    return jsonResponse(
      {
        type: 'about:blank',
        title: 'Validation Failed',
        status: 400,
        detail,
        errors,
      },
      400,
      'application/problem+json',
    )
  }
  console.error(err)
  return jsonResponse(
    {
      type: 'about:blank',
      title: 'Internal Server Error',
      status: 500,
    },
    500,
    'application/problem+json',
  )
}

function isValidationError(err: unknown): err is {
  name: 'TsgonestValidationError'
  message?: string
  errors?: unknown
  details?: unknown
} {
  return (
    typeof err === 'object' &&
    err !== null &&
    (err as { name?: unknown }).name === 'TsgonestValidationError'
  )
}

function readValidationErrors(err: {
  errors?: unknown
  details?: unknown
}): unknown[] {
  if (Array.isArray(err.errors)) return err.errors
  if (Array.isArray(err.details)) return err.details
  return []
}

function jsonResponse(body: unknown, status: number, contentType: string): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': contentType },
  })
}
