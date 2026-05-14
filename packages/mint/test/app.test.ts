import { describe, it, expect } from 'vitest'
import { createApp } from '../src/app'
import { defineModule } from '../src/module'

class HelloController {
  hello() {
    return 'hi'
  }
}

describe('createApp', () => {
  it('resolves controllers from modules', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })

    const ctrl = app.resolve(HelloController)
    expect(ctrl).toBeInstanceOf(HelloController)
    expect(app.resolve(HelloController)).toBe(ctrl)
  })

  it('returns 404 when no route matches', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })

    const res = await app.fetch(new Request('http://x/missing'))
    expect(res.status).toBe(404)
  })

  it('dispatches a manually registered route', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })

    app.router.add('GET', '/hello', async () => new Response('hi'))
    const res = await app.fetch(new Request('http://x/hello'))
    expect(res.status).toBe(200)
    expect(await res.text()).toBe('hi')
  })

  it('flattens nested imports and dedupes by identity', async () => {
    const inner = defineModule({ controllers: [HelloController] })
    const outer = defineModule({ imports: [inner, inner] })
    const app = await createApp({ imports: [outer, inner] })

    const ctrl = app.resolve(HelloController)
    expect(ctrl).toBeInstanceOf(HelloController)
  })

  it('event.resolve delegates to the container', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [HelloController] })],
    })

    app.router.add('GET', '/who', async (event) => {
      const ctrl = event.resolve(HelloController)
      return new Response(ctrl.hello())
    })

    const res = await app.fetch(new Request('http://x/who'))
    expect(await res.text()).toBe('hi')
  })
})
