import { describe, it, expect } from 'vitest'
import { createApp, defineModule } from '@mintkit/core'
import { wrap } from '../src/wrap'
import { BUN_SERVER } from '../src/token'

class HelloController {}

describe('wrap(app)', () => {
  it('exposes a fetch(req, server) handler', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    const handler = wrap(app)
    expect(typeof handler.fetch).toBe('function')
    expect(handler.fetch.length).toBeGreaterThanOrEqual(1)
  })

  it('makes BUN_SERVER resolvable inside a handler', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })

    let captured: unknown
    app.router.add('GET', '/server', async (event) => {
      captured = event.require(BUN_SERVER)
      return new Response('ok')
    })

    const handler = wrap(app)
    const fakeServer = { id: 'fake' } as any
    const res = await handler.fetch(new Request('http://x/server'), fakeServer)
    expect(res.status).toBe(200)
    expect(captured).toBe(fakeServer)
  })

  it('still returns 404 for unmatched routes', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    const handler = wrap(app)
    const res = await handler.fetch(
      new Request('http://x/missing'),
      {} as any,
    )
    expect(res.status).toBe(404)
  })

  it('passes a different server instance per call', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })

    const seen: unknown[] = []
    app.router.add('GET', '/r', async (event) => {
      seen.push(event.require(BUN_SERVER))
      return new Response('')
    })

    const handler = wrap(app)
    const s1 = { id: 1 } as any
    const s2 = { id: 2 } as any
    await handler.fetch(new Request('http://x/r'), s1)
    await handler.fetch(new Request('http://x/r'), s2)
    expect(seen).toEqual([s1, s2])
  })
})
