import { describe, it, expect } from 'vitest'
import { Container } from '../src/container'
import { defineToken } from '../src/token'

describe('Container', () => {
  it('returns the same instance on repeated resolve (singleton)', () => {
    class Service {
      readonly id = Math.random()
    }
    const c = new Container()
    c.register(Service, () => new Service())

    const a = c.resolve(Service)
    const b = c.resolve(Service)
    expect(a).toBe(b)
  })

  it('resolves a Token by identity', () => {
    const TOKEN = defineToken<{ value: number }>('value')
    const c = new Container()
    c.register(TOKEN, () => ({ value: 42 }))

    expect(c.resolve(TOKEN)).toEqual({ value: 42 })
    expect(c.resolve(TOKEN)).toBe(c.resolve(TOKEN))
  })

  it('throws when resolving an unregistered token', () => {
    class Unknown {}
    const c = new Container()
    expect(() => c.resolve(Unknown)).toThrow()
  })
})
