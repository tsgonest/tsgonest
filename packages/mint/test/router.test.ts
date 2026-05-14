import { describe, it, expect } from 'vitest'
import { Router } from '../src/router'
import type { RouteHandler } from '../src/types'

const noop: RouteHandler = async () => new Response('')

describe('Router', () => {
  it('matches a registered route', () => {
    const r = new Router()
    const handler: RouteHandler = async () => new Response('hi')
    r.add('GET', '/hello', handler)

    const m = r.match('GET', '/hello')
    expect(m).toBeDefined()
    expect(m!.handler).toBe(handler)
  })

  it('returns undefined for unknown path', () => {
    const r = new Router()
    r.add('GET', '/hello', noop)
    expect(r.match('GET', '/missing')).toBeUndefined()
  })

  it('returns undefined for unknown method', () => {
    const r = new Router()
    r.add('GET', '/hello', noop)
    expect(r.match('POST', '/hello')).toBeUndefined()
  })

  it('accepts arbitrary method strings such as WS', () => {
    const r = new Router()
    const handler: RouteHandler = async () => new Response('ws')
    // Method slot is a free-form string, not a constrained union.
    r.add('WS', '/socket', handler)
    expect(r.match('WS', '/socket')!.handler).toBe(handler)
  })
})
