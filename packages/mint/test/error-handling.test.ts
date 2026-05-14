import { describe, it, expect, vi, afterEach } from 'vitest'
import { createApp } from '../src/app'
import { defineModule } from '../src/module'
import {
  HttpError,
  ConflictError,
  BadRequestError,
  NotFoundError,
} from '../src/errors'

class HelloController {}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('handler throws + error mapper integration', () => {
  it('thrown HttpError maps to the carried status', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.router.add('GET', '/r', async () => {
      throw new ConflictError({ code: 'CONFLICTED' })
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(409)
    expect(res.headers.get('content-type')).toContain('application/json')
    expect(await res.json()).toEqual({ code: 'CONFLICTED' })
  })

  it('thrown plain HttpError honours the message → body fallback', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.router.add('GET', '/r', async () => {
      throw new NotFoundError(undefined, 'no such doc')
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(404)
    expect(await res.json()).toEqual({ error: 'no such doc' })
  })

  it('TsgonestValidationError-shaped throw produces RFC 9457 problem-details', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    class FakeValErr extends Error {
      readonly errors: Array<{ path: string; expected: string; received: string }>
      constructor() {
        super('Validation failed: 1 error(s)')
        this.name = 'TsgonestValidationError'
        this.errors = [{ path: 'x', expected: 'string', received: 'number' }]
      }
    }
    app.router.add('GET', '/r', async () => {
      throw new FakeValErr()
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(400)
    expect(res.headers.get('content-type')).toContain('application/problem+json')
    const body = (await res.json()) as Record<string, unknown>
    expect(body.title).toBe('Validation Failed')
    expect(body.status).toBe(400)
    expect(body.errors).toEqual([
      { path: 'x', expected: 'string', received: 'number' },
    ])
  })

  it('random throw → 500 problem-details with no leak; full error is logged', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.router.add('GET', '/r', async () => {
      throw new Error('top-secret stack trace')
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(500)
    expect(res.headers.get('content-type')).toContain('application/problem+json')
    const body = (await res.json()) as Record<string, unknown>
    expect(body.title).toBe('Internal Server Error')
    expect(JSON.stringify(body)).not.toContain('top-secret')
    expect(spy).toHaveBeenCalled()
  })
})

describe('app.onError — user-registered handlers', () => {
  it('takes precedence over the default HttpError mapping', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.onError(
      (e) => e instanceof ConflictError,
      () =>
        new Response(JSON.stringify({ overridden: true }), {
          status: 409,
          headers: { 'content-type': 'application/json' },
        }),
    )
    app.router.add('GET', '/r', async () => {
      throw new ConflictError({ code: 'x' })
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(409)
    expect(await res.json()).toEqual({ overridden: true })
  })

  it('first match wins among multiple user handlers', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.onError(
      (e) => e instanceof HttpError,
      () => new Response('http-handler', { status: 418 }),
    )
    app.onError(
      (e) => e instanceof BadRequestError,
      () => new Response('br-handler', { status: 400 }),
    )
    app.router.add('GET', '/r', async () => {
      throw new BadRequestError({ x: 1 })
    })
    const res = await app.fetch(new Request('http://x/r'))
    // The first predicate matches and wins, even though the second is more
    // specific.
    expect(res.status).toBe(418)
    expect(await res.text()).toBe('http-handler')
  })

  it('passes the event to the handler so middleware-set tokens are visible', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    let seenUrl: string | undefined
    app.onError(
      () => true,
      (_err, event) => {
        seenUrl = event.request.url
        return new Response('ok', { status: 200 })
      },
    )
    app.router.add('GET', '/r', async () => {
      throw new Error('x')
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(200)
    expect(seenUrl).toContain('/r')
  })

  it('falls through to defaults when no predicate matches', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.onError(
      (e) => e instanceof BadRequestError,
      () => new Response('never', { status: 200 }),
    )
    app.router.add('GET', '/r', async () => {
      throw new ConflictError({ code: 'c' })
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(409)
    expect(await res.json()).toEqual({ code: 'c' })
  })

  it('throw inside middleware also goes through onError', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.onError(
      (e) => e instanceof Error && (e as Error).message === 'auth',
      () => new Response('unauth', { status: 401 }),
    )
    app.use(async () => {
      throw new Error('auth')
    })
    app.router.add('GET', '/r', async () => new Response('ok'))
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(401)
    expect(await res.text()).toBe('unauth')
  })
})
