import { describe, it, expect } from 'vitest'
import { Container } from '../src/container'
import { defineToken } from '../src/token'

describe('Container — provider forms', () => {
  it('registers a class shorthand and resolves an instance', async () => {
    class Service {
      readonly tag = 'service'
    }
    const c = new Container()
    c.registerProvider(Service)
    await c.boot()
    const s = c.resolve(Service)
    expect(s).toBeInstanceOf(Service)
    expect(s.tag).toBe('service')
  })

  it('useValue provider returns the exact value', async () => {
    const TOKEN = defineToken<{ port: number }>('CONFIG')
    const value = { port: 8080 }
    const c = new Container()
    c.registerProvider({ provide: TOKEN, useValue: value })
    await c.boot()
    expect(c.resolve(TOKEN)).toBe(value)
  })

  it('useFactory provider runs at boot and caches the result', async () => {
    const TOKEN = defineToken<{ id: number }>('ID')
    let calls = 0
    const c = new Container()
    c.registerProvider({
      provide: TOKEN,
      useFactory: () => {
        calls++
        return { id: 42 }
      },
    })
    await c.boot()
    expect(calls).toBe(1)
    expect(c.resolve(TOKEN)).toEqual({ id: 42 })
    expect(c.resolve(TOKEN)).toBe(c.resolve(TOKEN))
    expect(calls).toBe(1)
  })

  it('useFactory awaits async factories during boot', async () => {
    const TOKEN = defineToken<{ name: string }>('CFG')
    const c = new Container()
    c.registerProvider({
      provide: TOKEN,
      useFactory: async () => {
        await new Promise<void>((r) => setTimeout(r, 1))
        return { name: 'resolved' }
      },
    })
    await c.boot()
    expect(c.resolve(TOKEN)).toEqual({ name: 'resolved' })
  })

  it('useFactory with inject resolves dependencies first', async () => {
    const DB_URL = defineToken<string>('DB_URL')
    const DB = defineToken<{ url: string }>('DB')
    const c = new Container()
    c.registerProvider({ provide: DB_URL, useValue: 'postgres://x' })
    c.registerProvider({
      provide: DB,
      useFactory: (url: string) => ({ url }),
      inject: [DB_URL],
    })
    await c.boot()
    expect(c.resolve(DB)).toEqual({ url: 'postgres://x' })
  })

  it('transient factory re-runs on each resolve', async () => {
    const TOKEN = defineToken<{ n: number }>('TR')
    let calls = 0
    const c = new Container()
    c.registerProvider({
      provide: TOKEN,
      useFactory: () => ({ n: ++calls }),
      scope: 'transient',
    })
    await c.boot()
    expect(c.resolve(TOKEN).n).toBe(1)
    expect(c.resolve(TOKEN).n).toBe(2)
    expect(c.resolve(TOKEN).n).toBe(3)
  })

  it('class with static inject array resolves constructor args', async () => {
    const NAME = defineToken<string>('NAME')
    class Greeter {
      static inject = [NAME] as const
      constructor(public name: string) {}
      hello() {
        return `hi ${this.name}`
      }
    }
    const c = new Container()
    c.registerProvider({ provide: NAME, useValue: 'world' })
    c.registerProvider(Greeter)
    await c.boot()
    expect(c.resolve(Greeter).hello()).toBe('hi world')
  })

  it('classes get constructed in topological order', async () => {
    const order: string[] = []
    class A {
      constructor() {
        order.push('A')
      }
    }
    class B {
      static inject = [A] as const
      constructor(public a: A) {
        order.push('B')
      }
    }
    class C {
      static inject = [B, A] as const
      constructor(
        public b: B,
        public a: A,
      ) {
        order.push('C')
      }
    }
    const c = new Container()
    c.registerProvider(C)
    c.registerProvider(B)
    c.registerProvider(A)
    await c.boot()
    expect(order).toEqual(['A', 'B', 'C'])
  })

  it('throws on a provider cycle with the cycle path', async () => {
    const A = defineToken<number>('A')
    const B = defineToken<number>('B')
    const c = new Container()
    c.registerProvider({ provide: A, useFactory: (b: number) => b + 1, inject: [B] })
    c.registerProvider({ provide: B, useFactory: (a: number) => a + 1, inject: [A] })
    await expect(c.boot()).rejects.toThrow(/cycle/i)
  })

  it('throws on a missing token at boot', async () => {
    const MISSING = defineToken<string>('MISSING')
    class Needs {
      static inject = [MISSING] as const
      constructor(public x: string) {}
    }
    const c = new Container()
    c.registerProvider(Needs)
    await expect(c.boot()).rejects.toThrow(/MISSING/)
  })

  it('throws on duplicate provider registration', async () => {
    const T = defineToken<number>('DUP')
    const c = new Container()
    c.registerProvider({ provide: T, useValue: 1 })
    expect(() => c.registerProvider({ provide: T, useValue: 2 })).toThrow(/duplicate/i)
  })
})
