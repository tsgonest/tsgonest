import { describe, it, expect } from 'vitest'
import { createApp } from '../src/app'
import { defineModule } from '../src/module'
import { defineToken } from '../src/token'

describe('Lifecycle — init and asyncDispose', () => {
  it('calls init() on every provider with one, in topological order', async () => {
    const order: string[] = []
    class A {
      async init() {
        order.push('A')
      }
    }
    class B {
      static inject = [A] as const
      constructor(public a: A) {}
      async init() {
        order.push('B')
      }
    }
    class C {
      static inject = [B] as const
      constructor(public b: B) {}
      async init() {
        order.push('C')
      }
    }
    await createApp({
      imports: [defineModule({ providers: [A, B, C] })],
    })
    expect(order).toEqual(['A', 'B', 'C'])
  })

  it('rejects createApp when an init throws and disposes already-init providers in reverse order', async () => {
    const events: string[] = []
    class A {
      async init() {
        events.push('init:A')
      }
      async [Symbol.asyncDispose]() {
        events.push('dispose:A')
      }
    }
    class B {
      static inject = [A] as const
      constructor(public a: A) {}
      async init() {
        events.push('init:B')
      }
      async [Symbol.asyncDispose]() {
        events.push('dispose:B')
      }
    }
    class C {
      static inject = [B] as const
      constructor(public b: B) {}
      async init() {
        events.push('init:C')
        throw new Error('C failed')
      }
      async [Symbol.asyncDispose]() {
        events.push('dispose:C')
      }
    }
    await expect(
      createApp({
        imports: [defineModule({ providers: [A, B, C] })],
      }),
    ).rejects.toThrow(/C failed/)
    expect(events).toEqual(['init:A', 'init:B', 'init:C', 'dispose:B', 'dispose:A'])
  })

  it('asyncDispose runs provider dispose in reverse topological order', async () => {
    const order: string[] = []
    class A {
      async [Symbol.asyncDispose]() {
        order.push('A')
      }
    }
    class B {
      static inject = [A] as const
      constructor(public a: A) {}
      async [Symbol.asyncDispose]() {
        order.push('B')
      }
    }
    const app = await createApp({
      imports: [defineModule({ providers: [A, B] })],
    })
    await app[Symbol.asyncDispose]()
    expect(order).toEqual(['B', 'A'])
  })

  it('drains in-flight fetch invocations before disposing', async () => {
    const events: string[] = []
    class Svc {
      async [Symbol.asyncDispose]() {
        events.push('disposed')
      }
    }
    const app = await createApp({
      imports: [defineModule({ providers: [Svc] })],
    })

    let resolveHandler!: () => void
    app.router.add('GET', '/slow', async () => {
      await new Promise<void>((r) => {
        resolveHandler = r
      })
      events.push('handler-done')
      return new Response('ok')
    })

    const inflight = app.fetch(new Request('http://x/slow'))
    await new Promise<void>((r) => setTimeout(r, 5))

    const disposed = app[Symbol.asyncDispose]()
    await new Promise<void>((r) => setTimeout(r, 5))

    expect(events).toEqual([])
    resolveHandler()
    const res = await inflight
    expect(res.status).toBe(200)
    await disposed
    expect(events).toEqual(['handler-done', 'disposed'])
  })

  it('returns 503 from fetch once asyncDispose has started', async () => {
    const app = await createApp({
      imports: [defineModule({})],
    })
    app.router.add('GET', '/ping', async () => new Response('pong'))
    await app[Symbol.asyncDispose]()
    const res = await app.fetch(new Request('http://x/ping'))
    expect(res.status).toBe(503)
  })

  it('throws AggregateError when multiple disposers throw', async () => {
    class A {
      async [Symbol.asyncDispose]() {
        throw new Error('A boom')
      }
    }
    class B {
      static inject = [A] as const
      constructor(public a: A) {}
      async [Symbol.asyncDispose]() {
        throw new Error('B boom')
      }
    }
    const app = await createApp({
      imports: [defineModule({ providers: [A, B] })],
    })
    let err: unknown
    try {
      await app[Symbol.asyncDispose]()
    } catch (e) {
      err = e
    }
    expect(err).toBeInstanceOf(AggregateError)
    expect((err as AggregateError).errors).toHaveLength(2)
  })

  it('rethrows a single disposer error directly', async () => {
    class Only {
      async [Symbol.asyncDispose]() {
        throw new Error('solo')
      }
    }
    const app = await createApp({ imports: [defineModule({ providers: [Only] })] })
    await expect(app[Symbol.asyncDispose]()).rejects.toThrow(/solo/)
  })

  it('detects provider cycles at boot with the cycle path', async () => {
    const A = defineToken<number>('CYCLE_A')
    const B = defineToken<number>('CYCLE_B')
    await expect(
      createApp({
        imports: [
          defineModule({
            providers: [
              { provide: A, useFactory: (b: number) => b + 1, inject: [B] },
              { provide: B, useFactory: (a: number) => a + 1, inject: [A] },
            ],
          }),
        ],
      }),
    ).rejects.toThrow(/cycle/i)
  })
})
