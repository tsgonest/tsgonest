import { describe, it, expect } from 'vitest'
import { createApp } from '../src/app'
import { defineModule } from '../src/module'
import { defineToken } from '../src/token'

describe('Module visibility', () => {
  it('private providers are not visible to importing modules', async () => {
    const SECRET = defineToken<string>('SECRET')
    class Consumer {
      static inject = [SECRET] as const
      constructor(public secret: string) {}
    }
    const inner = defineModule({
      providers: [{ provide: SECRET, useValue: 'shh' }],
    })
    const outer = defineModule({
      imports: [inner],
      providers: [Consumer],
    })
    await expect(createApp({ imports: [outer] })).rejects.toThrow(/visibility|SECRET/)
  })

  it('exports list makes the provider visible to importers', async () => {
    const TOKEN = defineToken<string>('GREET')
    class Consumer {
      static inject = [TOKEN] as const
      constructor(public greeting: string) {}
    }
    const inner = defineModule({
      providers: [{ provide: TOKEN, useValue: 'hi' }],
      exports: [TOKEN],
    })
    const outer = defineModule({
      imports: [inner],
      providers: [Consumer],
    })
    const app = await createApp({ imports: [outer] })
    expect(app.resolve(Consumer).greeting).toBe('hi')
  })

  it('re-exports through chains require explicit export at every hop', async () => {
    const TOKEN = defineToken<number>('VAL')
    class Consumer {
      static inject = [TOKEN] as const
      constructor(public v: number) {}
    }
    const a = defineModule({
      providers: [{ provide: TOKEN, useValue: 7 }],
      exports: [TOKEN],
    })
    const b = defineModule({
      imports: [a],
    })
    const c = defineModule({
      imports: [b],
      providers: [Consumer],
    })
    await expect(createApp({ imports: [c] })).rejects.toThrow(/visibility|VAL/)
  })

  it('re-exports through chains work when every hop exports', async () => {
    const TOKEN = defineToken<number>('VAL2')
    class Consumer {
      static inject = [TOKEN] as const
      constructor(public v: number) {}
    }
    const a = defineModule({
      providers: [{ provide: TOKEN, useValue: 99 }],
      exports: [TOKEN],
    })
    const b = defineModule({
      imports: [a],
      exports: [TOKEN],
    })
    const c = defineModule({
      imports: [b],
      providers: [Consumer],
    })
    const app = await createApp({ imports: [c] })
    expect(app.resolve(Consumer).v).toBe(99)
  })

  it('rejects duplicate provider tokens across the flattened graph', async () => {
    const TOKEN = defineToken<number>('DUP_X')
    const a = defineModule({ providers: [{ provide: TOKEN, useValue: 1 }] })
    const b = defineModule({ providers: [{ provide: TOKEN, useValue: 2 }] })
    await expect(createApp({ imports: [a, b] })).rejects.toThrow(/duplicate/i)
  })

  it('rejects a missing token referenced by a consumer class', async () => {
    const MISSING = defineToken<string>('NOT_THERE')
    class Consumer {
      static inject = [MISSING] as const
      constructor(public x: string) {}
    }
    const m = defineModule({ providers: [Consumer] })
    await expect(createApp({ imports: [m] })).rejects.toThrow(/NOT_THERE/)
  })

  it('a class can inject from its own module without exporting', async () => {
    const TOKEN = defineToken<string>('INTERNAL')
    class Consumer {
      static inject = [TOKEN] as const
      constructor(public v: string) {}
    }
    const m = defineModule({
      providers: [{ provide: TOKEN, useValue: 'private' }, Consumer],
    })
    const app = await createApp({ imports: [m] })
    expect(app.resolve(Consumer).v).toBe('private')
  })

  it('controllers are always globally registered (resolvable from app)', async () => {
    class Ctrl {
      ping() {
        return 'pong'
      }
    }
    const inner = defineModule({ controllers: [Ctrl] })
    const outer = defineModule({ imports: [inner] })
    const app = await createApp({ imports: [outer] })
    expect(app.resolve(Ctrl).ping()).toBe('pong')
  })
})
