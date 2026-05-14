import { describe, expect, it } from 'vitest'
import {
  parseMultipartStream,
  MultipartByteLimitError,
} from '../src/multipart'

const BOUNDARY = 'X-TEST-BOUNDARY'

function streamFromBytes(bytes: Uint8Array): ReadableStream<Uint8Array> {
  // Emit a few small chunks to exercise the boundary lookahead logic.
  return new ReadableStream<Uint8Array>({
    start(controller) {
      const chunkSize = 17 // intentionally awkward
      for (let i = 0; i < bytes.length; i += chunkSize) {
        controller.enqueue(bytes.slice(i, i + chunkSize))
      }
      controller.close()
    },
  })
}

function buildMultipart(parts: Array<{
  name: string
  value: Uint8Array | string
  filename?: string
  type?: string
}>): Uint8Array {
  const enc = new TextEncoder()
  const chunks: Uint8Array[] = []
  for (const p of parts) {
    chunks.push(enc.encode(`--${BOUNDARY}\r\n`))
    let cd = `Content-Disposition: form-data; name="${p.name}"`
    if (p.filename) cd += `; filename="${p.filename}"`
    chunks.push(enc.encode(`${cd}\r\n`))
    if (p.type) chunks.push(enc.encode(`Content-Type: ${p.type}\r\n`))
    chunks.push(enc.encode(`\r\n`))
    chunks.push(typeof p.value === 'string' ? enc.encode(p.value) : p.value)
    chunks.push(enc.encode(`\r\n`))
  }
  chunks.push(enc.encode(`--${BOUNDARY}--\r\n`))
  return concatBytes(chunks)
}

function concatBytes(chunks: Uint8Array[]): Uint8Array {
  let total = 0
  for (const c of chunks) total += c.length
  const out = new Uint8Array(total)
  let off = 0
  for (const c of chunks) {
    out.set(c, off)
    off += c.length
  }
  return out
}

async function readAll(stream: ReadableStream<Uint8Array>): Promise<Uint8Array> {
  const chunks: Uint8Array[] = []
  const reader = stream.getReader()
  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    if (value) chunks.push(value)
  }
  return concatBytes(chunks)
}

describe('parseMultipartStream', () => {
  it('yields text and file parts in order', async () => {
    const bytes = buildMultipart([
      { name: 'title', value: 'hello world' },
      {
        name: 'avatar',
        value: new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8]),
        filename: 'a.bin',
        type: 'application/octet-stream',
      },
    ])
    const iter = parseMultipartStream({
      stream: streamFromBytes(bytes),
      boundary: BOUNDARY,
    })
    const parts: Array<{ name: string; body: Uint8Array; type: string; filename?: string }> = []
    for await (const part of iter) {
      const body = await readAll(part.stream)
      parts.push({ name: part.name, body, type: part.type, filename: part.filename })
    }
    expect(parts).toHaveLength(2)
    expect(parts[0].name).toBe('title')
    expect(new TextDecoder().decode(parts[0].body)).toBe('hello world')
    expect(parts[1].name).toBe('avatar')
    expect(parts[1].filename).toBe('a.bin')
    expect(parts[1].type).toBe('application/octet-stream')
    expect(Array.from(parts[1].body)).toEqual([1, 2, 3, 4, 5, 6, 7, 8])
  })

  it('aborts a part that exceeds setLimit', async () => {
    const big = new Uint8Array(1024)
    for (let i = 0; i < big.length; i++) big[i] = i & 0xff
    const bytes = buildMultipart([{ name: 'file', value: big, filename: 'f.bin' }])
    const iter = parseMultipartStream({
      stream: streamFromBytes(bytes),
      boundary: BOUNDARY,
    })
    const { value: part } = await iter.next()
    expect(part).toBeDefined()
    iter.setLimit(100, 'body.file')

    let err: unknown
    try {
      await readAll(part!.stream)
    } catch (e) {
      err = e
    }
    expect(err).toBeInstanceOf(MultipartByteLimitError)
    if (err instanceof MultipartByteLimitError) {
      expect(err.limit).toBe(100)
      expect(err.path).toBe('body.file')
    }
  })

  it('ignores trailing bytes after the final boundary', async () => {
    const enc = new TextEncoder()
    const base = buildMultipart([{ name: 'a', value: '1' }])
    const trailing = enc.encode('garbage data should be ignored')
    const combined = concatBytes([base, trailing])

    const iter = parseMultipartStream({
      stream: streamFromBytes(combined),
      boundary: BOUNDARY,
    })
    const collected: Array<{ name: string; value: string }> = []
    for await (const part of iter) {
      const body = await readAll(part.stream)
      collected.push({ name: part.name, value: new TextDecoder().decode(body) })
    }
    expect(collected).toEqual([{ name: 'a', value: '1' }])
  })

  it('handles multiple binary parts without buffering everything', async () => {
    const a = new Uint8Array(32)
    const b = new Uint8Array(48)
    for (let i = 0; i < a.length; i++) a[i] = i
    for (let i = 0; i < b.length; i++) b[i] = 255 - i
    const bytes = buildMultipart([
      { name: 'a', value: a, filename: 'a.bin' },
      { name: 'b', value: b, filename: 'b.bin' },
    ])
    const iter = parseMultipartStream({
      stream: streamFromBytes(bytes),
      boundary: BOUNDARY,
    })
    const got: Record<string, Uint8Array> = {}
    for await (const part of iter) {
      got[part.name] = await readAll(part.stream)
    }
    expect(Array.from(got['a'])).toEqual(Array.from(a))
    expect(Array.from(got['b'])).toEqual(Array.from(b))
  })

  it('treats parts whose body equals the source byte-by-byte', async () => {
    // Verify boundary lookahead doesn't accidentally swallow content.
    const value = new TextEncoder().encode('hi -- ' + 'X-TEST-BO ' + 'no boundary here')
    const bytes = buildMultipart([{ name: 'a', value, filename: 'a.txt', type: 'text/plain' }])
    const iter = parseMultipartStream({
      stream: streamFromBytes(bytes),
      boundary: BOUNDARY,
    })
    const { value: part } = await iter.next()
    const body = await readAll(part!.stream)
    expect(new TextDecoder().decode(body)).toBe(new TextDecoder().decode(value))
  })
})
