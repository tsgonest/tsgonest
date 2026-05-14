import { describe, it, expect } from 'vitest'
import { createApp } from '../src/app'
import { defineModule } from '../src/module'
import type { Middleware } from '../src/types'

class HelloController {}

const tag = (label: string, log: string[]): Middleware => {
  return async (_event, next) => {
    log.push(`>${label}`)
    const res = await next()
    log.push(`<${label}`)
    return res
  }
}

describe('app.use — global middleware', () => {
  it('runs outer-to-inner; handler last', async () => {
    const log: string[] = []
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.use(tag('A', log))
    app.use(tag('B', log))
    app.router.add('GET', '/r', async () => {
      log.push('handler')
      return new Response('hi')
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(200)
    expect(log).toEqual(['>A', '>B', 'handler', '<B', '<A'])
  })

  it('preserves registration order across multiple use() calls', async () => {
    const log: string[] = []
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.use(tag('first', log))
    app.use(tag('second', log))
    app.use(tag('third', log))
    app.router.add('GET', '/r', async () => new Response(''))
    await app.fetch(new Request('http://x/r'))
    expect(log).toEqual([
      '>first',
      '>second',
      '>third',
      '<third',
      '<second',
      '<first',
    ])
  })
})

describe('app.use(prefix, mw) — prefix middleware', () => {
  it('runs only for matching pathname prefixes', async () => {
    const log: string[] = []
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.use('/api', tag('api', log))
    app.router.add('GET', '/api/users', async () => new Response('a'))
    app.router.add('GET', '/public', async () => new Response('b'))

    await app.fetch(new Request('http://x/api/users'))
    await app.fetch(new Request('http://x/public'))

    // 'api' should have wrapped only the /api/users call.
    expect(log).toEqual(['>api', '<api'])
  })

  it('runs in combination with global middleware in registration order', async () => {
    const log: string[] = []
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.use(tag('global', log))
    app.use('/api', tag('api', log))
    app.router.add('GET', '/api/x', async () => new Response(''))
    await app.fetch(new Request('http://x/api/x'))
    expect(log).toEqual(['>global', '>api', '<api', '<global'])
  })
})

describe('short-circuit + throw semantics', () => {
  it('middleware that returns without next() prevents handler execution', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    let handlerRan = false
    app.use(async () => new Response('gate', { status: 401 }))
    app.router.add('GET', '/r', async () => {
      handlerRan = true
      return new Response('hi')
    })
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(401)
    expect(await res.text()).toBe('gate')
    expect(handlerRan).toBe(false)
  })

  it('thrown error in middleware hits the error mapper', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.use(async () => {
      throw new Error('boom')
    })
    app.router.add('GET', '/r', async () => new Response('hi'))
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.status).toBe(500)
    const body = (await res.json()) as Record<string, unknown>
    expect(body.status).toBe(500)
    expect(body.title).toBe('Internal Server Error')
  })

  it('middleware can wrap the downstream response', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })
    app.use(async (_event, next) => {
      const res = (await next()) as Response
      const headers = new Headers(res.headers)
      headers.set('x-wrapped', '1')
      return new Response(res.body, { status: res.status, headers })
    })
    app.router.add('GET', '/r', async () => new Response('body'))
    const res = await app.fetch(new Request('http://x/r'))
    expect(res.headers.get('x-wrapped')).toBe('1')
  })
})

describe('module middleware', () => {
  it('applies only to that module\'s controllers, not imports', async () => {
    const log: string[] = []
    class CtrlA {}
    class CtrlB {}
    const inner = defineModule({
      controllers: [CtrlA],
      middleware: [tag('inner', log)],
    })
    const outer = defineModule({
      imports: [inner],
      controllers: [CtrlB],
      middleware: [tag('outer', log)],
    })
    const app = await createApp({ imports: [outer] })
    app.router.addForController(CtrlA, 'GET', '/a', async () => new Response('a'))
    app.router.addForController(CtrlB, 'GET', '/b', async () => new Response('b'))

    await app.fetch(new Request('http://x/a'))
    expect(log).toEqual(['>inner', '<inner'])

    log.length = 0
    await app.fetch(new Request('http://x/b'))
    expect(log).toEqual(['>outer', '<outer'])
  })

  it('runs after global middleware in execution order', async () => {
    const log: string[] = []
    class Ctrl {}
    const mod = defineModule({
      controllers: [Ctrl],
      middleware: [tag('mod', log)],
    })
    const app = await createApp({ imports: [mod] })
    app.use(tag('global', log))
    app.router.addForController(Ctrl, 'GET', '/r', async () => {
      log.push('handler')
      return new Response('hi')
    })

    await app.fetch(new Request('http://x/r'))
    expect(log).toEqual(['>global', '>mod', 'handler', '<mod', '<global'])
  })

  it('falls back to global-only when route has no associated controller', async () => {
    const log: string[] = []
    class Ctrl {}
    const mod = defineModule({
      controllers: [Ctrl],
      middleware: [tag('mod', log)],
    })
    const app = await createApp({ imports: [mod] })
    app.use(tag('global', log))
    app.router.add('GET', '/loose', async () => new Response(''))
    await app.fetch(new Request('http://x/loose'))
    expect(log).toEqual(['>global', '<global'])
  })
})
