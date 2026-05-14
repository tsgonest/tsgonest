import { describe, it, expect, expectTypeOf } from 'vitest'
import { createApp, createContext } from '../src/app'
import type { App, Context } from '../src/app'
import { defineModule } from '../src/module'
import { defineToken } from '../src/token'

describe('createContext — headless', () => {
  it('boots and resolves providers without a router', async () => {
    const TOKEN = defineToken<{ port: number }>('CFG')
    class Service {
      static inject = [TOKEN] as const
      constructor(public cfg: { port: number }) {}
    }
    const ctx = await createContext({
      imports: [
        defineModule({
          providers: [{ provide: TOKEN, useValue: { port: 9000 } }, Service],
          exports: [TOKEN],
        }),
      ],
    })
    expect(ctx.resolve(Service).cfg.port).toBe(9000)
    expect((ctx as unknown as { fetch?: unknown }).fetch).toBeUndefined()
    expect((ctx as unknown as { router?: unknown }).router).toBeUndefined()
    await ctx[Symbol.asyncDispose]()
  })

  it('runs init() and dispose for headless contexts too', async () => {
    const order: string[] = []
    class Svc {
      async init() {
        order.push('init')
      }
      async [Symbol.asyncDispose]() {
        order.push('dispose')
      }
    }
    const ctx = await createContext({
      imports: [defineModule({ providers: [Svc] })],
    })
    expect(order).toEqual(['init'])
    await ctx[Symbol.asyncDispose]()
    expect(order).toEqual(['init', 'dispose'])
  })

  it('silently ignores controllers in imported modules', async () => {
    class Ctrl {
      hello() {
        return 'hi'
      }
    }
    const ctx = await createContext({
      imports: [defineModule({ controllers: [Ctrl] })],
    })
    expect(ctx.resolve(Ctrl).hello()).toBe('hi')
    expect((ctx as unknown as { router?: unknown }).router).toBeUndefined()
    await ctx[Symbol.asyncDispose]()
  })

  it('App is structurally assignable to Context', async () => {
    const app = await createApp({ imports: [defineModule({})] })
    const asCtx: Context = app
    expect(typeof asCtx.resolve).toBe('function')
    expect(typeof asCtx[Symbol.asyncDispose]).toBe('function')
    await app[Symbol.asyncDispose]()
    expectTypeOf<App>().toExtend<Context>()
  })

  it('await using auto-disposes the context on scope exit', async () => {
    const events: string[] = []
    class Svc {
      async [Symbol.asyncDispose]() {
        events.push('disposed')
      }
    }
    {
      await using _ctx = await createContext({
        imports: [defineModule({ providers: [Svc] })],
      })
      events.push('inside')
    }
    expect(events).toEqual(['inside', 'disposed'])
  })

  it('await using auto-disposes even on throw', async () => {
    const events: string[] = []
    class Svc {
      async [Symbol.asyncDispose]() {
        events.push('disposed')
      }
    }
    await expect(
      (async () => {
        await using _ctx = await createContext({
          imports: [defineModule({ providers: [Svc] })],
        })
        events.push('before-throw')
        throw new Error('boom')
      })(),
    ).rejects.toThrow(/boom/)
    expect(events).toEqual(['before-throw', 'disposed'])
  })

  it('module visibility validation runs for createContext too', async () => {
    const HIDDEN = defineToken<number>('HIDDEN_CTX')
    class C {
      static inject = [HIDDEN] as const
      constructor(public x: number) {}
    }
    const inner = defineModule({
      providers: [{ provide: HIDDEN, useValue: 1 }],
    })
    const outer = defineModule({ imports: [inner], providers: [C] })
    await expect(createContext({ imports: [outer] })).rejects.toThrow(/visibility|HIDDEN/)
  })
})
