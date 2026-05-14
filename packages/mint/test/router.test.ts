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

  it('extracts a single :param', () => {
    const r = new Router()
    const handler: RouteHandler = async () => new Response('ok')
    r.add('GET', '/users/:id', handler)
    const m = r.match('GET', '/users/42')
    expect(m).toBeDefined()
    expect(m!.params).toEqual({ id: '42' })
  })

  it('extracts multiple :params', () => {
    const r = new Router()
    r.add('GET', '/orgs/:org/users/:id', noop)
    const m = r.match('GET', '/orgs/acme/users/7')
    expect(m).toBeDefined()
    expect(m!.params).toEqual({ org: 'acme', id: '7' })
  })

  it('does not match when segment counts differ', () => {
    const r = new Router()
    r.add('GET', '/users/:id', noop)
    expect(r.match('GET', '/users')).toBeUndefined()
    expect(r.match('GET', '/users/1/posts')).toBeUndefined()
  })

  it('does not bind an empty segment to a :param', () => {
    const r = new Router()
    r.add('GET', '/users/:id', noop)
    expect(r.match('GET', '/users/')).toBeUndefined()
  })

  it('decodes URL-encoded :param values', () => {
    const r = new Router()
    r.add('GET', '/items/:name', noop)
    const m = r.match('GET', '/items/hello%20world')
    expect(m!.params).toEqual({ name: 'hello world' })
  })

  it('still serves static routes alongside :param routes', () => {
    const r = new Router()
    const staticH: RouteHandler = async () => new Response('s')
    const paramH: RouteHandler = async () => new Response('p')
    r.add('GET', '/users', staticH)
    r.add('GET', '/users/:id', paramH)
    expect(r.match('GET', '/users')!.handler).toBe(staticH)
    expect(r.match('GET', '/users/42')!.handler).toBe(paramH)
  })
})
