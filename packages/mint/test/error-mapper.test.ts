import { describe, it, expect, vi, afterEach } from 'vitest'
import { ErrorMapper } from '../src/error-mapper'
import {
  HttpError,
  BadRequestError,
  ConflictError,
  NotFoundError,
} from '../src/errors'
import { Event } from '../src/event'
import { Container } from '../src/container'

function makeEvent(): Event {
  return new Event(new Request('http://x/'), new Container())
}

describe('ErrorMapper — default handlers', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('maps HttpError to a JSON Response with the carried status and body', async () => {
    const mapper = new ErrorMapper()
    const res = await mapper.map(new ConflictError({ code: 'DUPLICATE' }), makeEvent())
    expect(res.status).toBe(409)
    expect(res.headers.get('content-type')).toContain('application/json')
    expect(await res.json()).toEqual({ code: 'DUPLICATE' })
  })

  it('falls back to { error: message } when HttpError body is undefined', async () => {
    const mapper = new ErrorMapper()
    const res = await mapper.map(new NotFoundError(undefined, 'gone'), makeEvent())
    expect(res.status).toBe(404)
    expect(await res.json()).toEqual({ error: 'gone' })
  })

  it('maps a plain HttpError (any status) correctly', async () => {
    const mapper = new ErrorMapper()
    const res = await mapper.map(new HttpError(418, { teapot: true }), makeEvent())
    expect(res.status).toBe(418)
    expect(await res.json()).toEqual({ teapot: true })
  })

  it('maps TsgonestValidationError-named errors to RFC 9457 problem-details', async () => {
    const mapper = new ErrorMapper()
    class FakeValErr extends Error {
      readonly errors: Array<{ path: string; expected: string; received: string }>
      constructor(errors: Array<{ path: string; expected: string; received: string }>) {
        super('Validation failed: 1 error(s)')
        this.name = 'TsgonestValidationError'
        this.errors = errors
      }
    }
    const err = new FakeValErr([
      { path: 'input.name', expected: 'string', received: 'undefined' },
    ])
    const res = await mapper.map(err, makeEvent())
    expect(res.status).toBe(400)
    expect(res.headers.get('content-type')).toContain('application/problem+json')
    const body = (await res.json()) as Record<string, unknown>
    expect(body.type).toBe('about:blank')
    expect(body.title).toBe('Validation Failed')
    expect(body.status).toBe(400)
    expect(body.detail).toBe('Validation failed: 1 error(s)')
    expect(body.errors).toEqual([
      { path: 'input.name', expected: 'string', received: 'undefined' },
    ])
  })

  it('also accepts `details` as a fallback field for validation errors', async () => {
    const mapper = new ErrorMapper()
    class FakeValErr extends Error {
      readonly details: Array<unknown>
      constructor(details: Array<unknown>) {
        super('bad')
        this.name = 'TsgonestValidationError'
        this.details = details
      }
    }
    const res = await mapper.map(new FakeValErr([{ x: 1 }]), makeEvent())
    const body = (await res.json()) as Record<string, unknown>
    expect(body.errors).toEqual([{ x: 1 }])
  })

  it('emits a 500 problem-details and logs the error for unknown throws', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const mapper = new ErrorMapper()
    const res = await mapper.map(new Error('top-secret stack trace'), makeEvent())
    expect(res.status).toBe(500)
    expect(res.headers.get('content-type')).toContain('application/problem+json')
    const body = (await res.json()) as Record<string, unknown>
    expect(body.status).toBe(500)
    expect(body.title).toBe('Internal Server Error')
    expect(body.type).toBe('about:blank')
    // No leak of the original message
    expect(JSON.stringify(body)).not.toContain('top-secret')
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('handles non-Error throws (string/object) without leaking content', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const mapper = new ErrorMapper()
    const res = await mapper.map('plain string boom', makeEvent())
    expect(res.status).toBe(500)
    const body = (await res.json()) as Record<string, unknown>
    expect(JSON.stringify(body)).not.toContain('plain string boom')
    expect(spy).toHaveBeenCalled()
  })
})

describe('ErrorMapper — user handlers', () => {
  it('user handlers run BEFORE defaults; first match wins', async () => {
    const mapper = new ErrorMapper()
    let firstCalled = 0
    let secondCalled = 0
    mapper.register(
      (e) => e instanceof Error && e.message === 'snowflake',
      () => {
        firstCalled++
        return new Response('first', { status: 418 })
      },
    )
    mapper.register(
      (e) => e instanceof Error && e.message === 'snowflake',
      () => {
        secondCalled++
        return new Response('second', { status: 422 })
      },
    )
    const res = await mapper.map(new Error('snowflake'), makeEvent())
    expect(res.status).toBe(418)
    expect(await res.text()).toBe('first')
    expect(firstCalled).toBe(1)
    expect(secondCalled).toBe(0)
  })

  it('user handler takes precedence over the default HttpError mapping', async () => {
    const mapper = new ErrorMapper()
    mapper.register(
      (e) => e instanceof ConflictError,
      () => new Response('custom-conflict', { status: 409, headers: { 'x-overridden': '1' } }),
    )
    const res = await mapper.map(new ConflictError({ code: 'x' }), makeEvent())
    expect(res.status).toBe(409)
    expect(res.headers.get('x-overridden')).toBe('1')
    expect(await res.text()).toBe('custom-conflict')
  })

  it('user handler can return a Promise<Response>', async () => {
    const mapper = new ErrorMapper()
    mapper.register(
      () => true,
      async () => new Response('async', { status: 202 }),
    )
    const res = await mapper.map(new Error('x'), makeEvent())
    expect(res.status).toBe(202)
    expect(await res.text()).toBe('async')
  })

  it('falls through to defaults when no user predicate matches', async () => {
    const mapper = new ErrorMapper()
    mapper.register(
      (e) => e instanceof Error && (e as Error).message === 'other',
      () => new Response('never', { status: 200 }),
    )
    const res = await mapper.map(new BadRequestError({ code: 'bad' }), makeEvent())
    expect(res.status).toBe(400)
    expect(await res.json()).toEqual({ code: 'bad' })
  })

  it('passes the event to the handler', async () => {
    const mapper = new ErrorMapper()
    const event = makeEvent()
    let captured: Event | undefined
    mapper.register(
      () => true,
      (_err, ev) => {
        captured = ev
        return new Response('ok')
      },
    )
    await mapper.map(new Error('x'), event)
    expect(captured).toBe(event)
  })
})
