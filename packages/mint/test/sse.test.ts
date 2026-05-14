import { describe, it, expect } from 'vitest'
import { sse, type SSEMessage, type SSEStream } from '../src/sse'

async function readAll(res: Response): Promise<string> {
  // Use the body reader to avoid runtime-dependent helpers.
  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let out = ''
  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    out += decoder.decode(value, { stream: true })
  }
  out += decoder.decode()
  return out
}

describe('sse()', () => {
  it('sets text/event-stream headers and streams a Uint8Array body', async () => {
    const res = sse<number>(async function* () {
      yield 1
    })
    expect(res.headers.get('content-type')).toBe('text/event-stream')
    expect(res.headers.get('cache-control')).toBe('no-cache')
    expect(res.headers.get('connection')).toBe('keep-alive')
    expect(res.body).toBeInstanceOf(ReadableStream)
  })

  it('frames plain values as JSON data lines terminated by a blank line', async () => {
    const res = sse<{ n: number }>(async function* () {
      yield { n: 1 }
      yield { n: 2 }
      yield { n: 3 }
    })
    const text = await readAll(res)
    expect(text).toBe(
      'data: {"n":1}\n\n' + 'data: {"n":2}\n\n' + 'data: {"n":3}\n\n',
    )
  })

  it('emits event/id/retry/data lines for SSEMessage objects', async () => {
    const res = sse<{ tick: number }>(async function* () {
      const msg: SSEMessage<{ tick: number }> = {
        data: { tick: 7 },
        event: 'tick',
        id: '1',
        retry: 2500,
      }
      yield msg
    })
    const text = await readAll(res)
    expect(text).toContain('event: tick\n')
    expect(text).toContain('id: 1\n')
    expect(text).toContain('retry: 2500\n')
    expect(text).toContain('data: {"tick":7}\n')
    expect(text.endsWith('\n\n')).toBe(true)
  })

  it('closes cleanly when the generator returns', async () => {
    const res = sse<number>(async function* () {
      yield 1
      return
    })
    const text = await readAll(res)
    expect(text).toBe('data: 1\n\n')
  })

  it("calls generator.return() so finally blocks run when the consumer cancels", async () => {
    let cleaned = 0
    let yielded = 0
    const res = sse<number>(async function* () {
      try {
        while (true) {
          yielded++
          yield yielded
          // Cooperative pause so the consumer can cancel before we loop again.
          await new Promise<void>((r) => setTimeout(r, 0))
        }
      } finally {
        cleaned++
      }
    })
    const reader = res.body!.getReader()
    const first = await reader.read()
    expect(first.done).toBe(false)
    await reader.cancel()
    // Give the generator one tick to run its finally block.
    await new Promise<void>((r) => setTimeout(r, 10))
    expect(cleaned).toBe(1)
  })

  it('terminates without a half-finished frame when the generator throws', async () => {
    const res = sse<number>(async function* () {
      yield 1
      throw new Error('boom')
    })
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    const first = await reader.read()
    expect(first.done).toBe(false)
    expect(decoder.decode(first.value!)).toBe('data: 1\n\n')
    // The second read surfaces the throw — there is no partial frame in the
    // buffer because the error was raised at a frame boundary.
    await expect(reader.read()).rejects.toThrow('boom')
  })

  it('compiles with a typed event payload via SSEStream<T>', () => {
    // Compile-time check only: the call expression must type-check and the
    // phantom SSEStream<T> alias must exist as an exported type.
    type Tick = { type: 'tick'; ts: number }
    const _typed: () => Response = () =>
      sse<Tick>(async function* () {
        yield { type: 'tick', ts: 0 }
      })
    const _phantom: SSEStream<Tick> | undefined = undefined
    expect(typeof _typed).toBe('function')
    expect(_phantom).toBeUndefined()
  })
})
