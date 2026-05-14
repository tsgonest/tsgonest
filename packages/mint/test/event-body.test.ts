import { describe, it, expect } from 'vitest'
import { Event } from '../src/event'
import { Container } from '../src/container'

function bodyEvent(body: BodyInit, headers?: Record<string, string>): Event {
  return new Event(
    new Request('http://x/', { method: 'POST', body, headers }),
    new Container(),
  )
}

describe('Event.body', () => {
  it('bytes() returns the raw body and is memoized', async () => {
    const event = bodyEvent('hello')
    const a = await event.body.bytes()
    const b = await event.body.bytes()
    expect(a).toBeInstanceOf(Uint8Array)
    expect(new TextDecoder().decode(a)).toBe('hello')
    // Memoized: same reference on repeated calls.
    expect(a).toBe(b)
  })

  it('text() decodes the bytes and is memoized', async () => {
    const event = bodyEvent('hello')
    expect(await event.body.text()).toBe('hello')
    expect(await event.body.text()).toBe('hello')
  })

  it('json() parses the body', async () => {
    const event = bodyEvent(JSON.stringify({ a: 1 }), {
      'content-type': 'application/json',
    })
    expect(await event.body.json<{ a: number }>()).toEqual({ a: 1 })
    expect(await event.body.json()).toEqual({ a: 1 })
  })

  it('formData() works for application/x-www-form-urlencoded', async () => {
    const event = bodyEvent('a=1&b=two', {
      'content-type': 'application/x-www-form-urlencoded',
    })
    const fd = await event.body.formData()
    expect(fd.get('a')).toBe('1')
    expect(fd.get('b')).toBe('two')
  })

  it('formData() works for multipart/form-data', async () => {
    const form = new FormData()
    form.append('name', 'jane')
    form.append('file', new Blob(['hello']), 'hello.txt')
    const event = new Event(
      new Request('http://x/', { method: 'POST', body: form }),
      new Container(),
    )
    const fd = await event.body.formData()
    expect(fd.get('name')).toBe('jane')
    const file = fd.get('file') as File | null
    expect(file).toBeInstanceOf(Blob)
    expect(await (file as Blob).text()).toBe('hello')
  })

  it('formData() can be called after bytes() (re-parsed from cached bytes)', async () => {
    const event = bodyEvent('a=1&b=2', {
      'content-type': 'application/x-www-form-urlencoded',
    })
    const bytes = await event.body.bytes()
    expect(bytes).toBeInstanceOf(Uint8Array)
    const fd = await event.body.formData()
    expect(fd.get('a')).toBe('1')
    expect(fd.get('b')).toBe('2')
  })

  it('stream() throws if bytes() was already called', async () => {
    const event = bodyEvent('x')
    await event.body.bytes()
    expect(() => event.body.stream()).toThrow(/already consumed/)
  })

  it('bytes() throws if stream() was already taken', async () => {
    const event = bodyEvent('x')
    const stream = event.body.stream()
    expect(stream).toBeInstanceOf(ReadableStream)
    await expect(event.body.bytes()).rejects.toThrow(/already consumed/)
  })

  it('stream() can only be taken once', async () => {
    const event = bodyEvent('x')
    event.body.stream()
    expect(() => event.body.stream()).toThrow(/already consumed/)
  })
})

describe('Event.waitUntil', () => {
  it('records promises in the internal set', () => {
    const event = new Event(new Request('http://x/'), new Container())
    const p1 = Promise.resolve(1)
    const p2 = Promise.resolve(2)
    event.waitUntil(p1)
    event.waitUntil(p2)
    expect(event.waitUntilPromises.size).toBe(2)
    expect(event.waitUntilPromises.has(p1)).toBe(true)
    expect(event.waitUntilPromises.has(p2)).toBe(true)
  })
})
