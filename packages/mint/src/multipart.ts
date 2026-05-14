/**
 * Streaming multipart/form-data parser.
 *
 * Reads from a `ReadableStream<Uint8Array>` boundary-delimited body and yields
 * each part as `{ name, filename?, type, stream }`. Each yielded part's
 * `stream` MUST be fully consumed (or cancelled) before the next part is
 * requested. The parser pulls bytes from the source lazily, so memory stays
 * bounded by roughly `boundary + chunk` size.
 *
 * Caller can throttle a single part by calling
 * `iter.setLimit(maxBytes, path)` after yielding it; when the part exceeds
 * `maxBytes`, the next read on its stream rejects with a
 * `MultipartByteLimitError`.
 */

export interface MultipartPart {
  readonly name: string
  readonly filename?: string
  readonly type: string
  readonly stream: ReadableStream<Uint8Array>
}

export interface MultipartParserOptions {
  stream: ReadableStream<Uint8Array>
  /** boundary without the leading `--`. */
  boundary: string
}

export interface MultipartIterator extends AsyncIterableIterator<MultipartPart> {
  /**
   * Set a maximum byte count for the currently-yielded part. Reads past the
   * limit reject with `MultipartByteLimitError`. Must be called after the
   * part is yielded; resets per part.
   */
  setLimit(maxBytes: number, path?: string): void
}

/** Error thrown when a part exceeds its configured `MaxSize`. */
export class MultipartByteLimitError extends Error {
  override readonly name = 'MultipartByteLimitError'
  constructor(
    readonly limit: number,
    readonly path: string,
  ) {
    super(`multipart part ${path} exceeded ${limit} bytes`)
  }
}

const CR = 13
const LF = 10
const DASH = 45

export function parseMultipartStream(opts: MultipartParserOptions): MultipartIterator {
  const reader = opts.stream.getReader()
  const boundaryBytes = encodeUtf8(`--${opts.boundary}`)

  // Source buffer holding bytes pulled but not yet routed.
  let buf = new Uint8Array(0)
  let sourceDone = false

  // Active part state. `null` when no part is currently being yielded.
  let active: {
    controller: ReadableStreamDefaultController<Uint8Array>
    bytes: number
    limit: number | null
    path: string
    closed: boolean
  } | null = null

  // Pulls more bytes from the source. Returns false on EOF.
  const pump = async (): Promise<boolean> => {
    if (sourceDone) return false
    const { value, done } = await reader.read()
    if (done) {
      sourceDone = true
      return false
    }
    if (value && value.length > 0) {
      buf = concat(buf, value)
    }
    return true
  }

  const ensure = async (n: number): Promise<boolean> => {
    while (buf.length < n) {
      const ok = await pump()
      if (!ok) return false
    }
    return true
  }

  // Hand `chunk` to the active part. Throws/errors the stream on overflow.
  const writeToActive = (chunk: Uint8Array): boolean => {
    if (!active || active.closed) return true
    if (chunk.length === 0) return true
    active.bytes += chunk.length
    if (active.limit !== null && active.bytes > active.limit) {
      const err = new MultipartByteLimitError(active.limit, active.path)
      active.controller.error(err)
      active.closed = true
      return false
    }
    active.controller.enqueue(chunk)
    return true
  }

  // Drives ONE pump+forward iteration for the active part. Closes the part
  // when the next boundary is encountered or the source is exhausted.
  const advance = async (): Promise<void> => {
    if (!active || active.closed) return
    const idx = indexOf(buf, boundaryBytes)
    if (idx >= 0) {
      // Content ends at the boundary; strip the trailing CRLF (or LF) that
      // separates the part body from the boundary line.
      let end = idx
      if (end >= 2 && buf[end - 2] === CR && buf[end - 1] === LF) end -= 2
      else if (end >= 1 && buf[end - 1] === LF) end -= 1
      if (end > 0) {
        if (!writeToActive(buf.slice(0, end))) return
      }
      buf = buf.slice(idx)
      active.controller.close()
      active.closed = true
      return
    }
    // No boundary yet: forward all but the trailing `boundary - 1` bytes,
    // which might contain a partial match.
    const safeLen = buf.length - boundaryBytes.length
    if (safeLen > 0) {
      if (!writeToActive(buf.slice(0, safeLen))) return
      buf = buf.slice(safeLen)
    }
    const ok = await pump()
    if (!ok) {
      // Source done — flush remaining buffer (no more boundaries).
      if (buf.length > 0) {
        if (!writeToActive(buf)) return
        buf = new Uint8Array(0)
      }
      active.controller.close()
      active.closed = true
    }
  }

  // Drain the active part to its conclusion. Used when the consumer drops
  // a part (doesn't await its reader) — we still need to advance past it.
  const drainActive = async (): Promise<void> => {
    while (active && !active.closed) {
      await advance()
    }
  }

  // Read headers + content-disposition for the next part. Returns null when
  // the final boundary (`--boundary--`) is reached.
  const readNextHeaders = async (): Promise<{
    name: string
    filename?: string
    type: string
  } | null> => {
    // Locate the boundary marker (it may be at offset 0 for the first part,
    // or arrive after one or more pump() calls when chunks are small).
    while (true) {
      const idx = indexOf(buf, boundaryBytes)
      if (idx >= 0) {
        buf = buf.slice(idx + boundaryBytes.length)
        break
      }
      // Keep the last (boundary - 1) bytes in case the boundary straddles.
      if (buf.length > boundaryBytes.length) {
        buf = buf.slice(buf.length - boundaryBytes.length)
      }
      const ok = await pump()
      if (!ok) return null
    }
    // After the boundary, expect either `--` (terminator) or CRLF / LF (next part).
    if (!(await ensure(2))) return null
    if (buf[0] === DASH && buf[1] === DASH) {
      return null
    }
    // Strip a single line separator (CRLF or LF) after the boundary.
    if (buf[0] === CR && buf.length > 1 && buf[1] === LF) {
      buf = buf.slice(2)
    } else if (buf[0] === LF) {
      buf = buf.slice(1)
    }
    // Read header block terminated by \r\n\r\n (or \n\n).
    while (true) {
      const idx = findHeaderEnd(buf)
      if (idx !== -1) {
        const headerBytes = buf.slice(0, idx.end)
        buf = buf.slice(idx.advance)
        return parseHeaderBlock(headerBytes)
      }
      const ok = await pump()
      if (!ok) return null
    }
  }

  let parserDone = false

  const next = async (): Promise<IteratorResult<MultipartPart>> => {
    if (parserDone) return { done: true, value: undefined }
    if (active && !active.closed) {
      // Consumer skipped reading — drain to boundary.
      await drainActive()
    }
    active = null
    const headers = await readNextHeaders()
    if (!headers) {
      parserDone = true
      try {
        reader.releaseLock()
      } catch {
        /* noop */
      }
      return { done: true, value: undefined }
    }

    let myActive: NonNullable<typeof active>
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        myActive = {
          controller,
          bytes: 0,
          limit: null,
          path: `body.${headers.name}`,
          closed: false,
        }
        active = myActive
      },
      pull: async () => {
        if (!myActive || myActive.closed) return
        // One advance call ≈ one chunk forwarded (or end-of-part).
        try {
          await advance()
        } catch {
          /* errored via controller.error already */
        }
      },
      cancel: async () => {
        if (myActive) myActive.closed = true
      },
    })

    return {
      done: false,
      value: {
        name: headers.name,
        filename: headers.filename,
        type: headers.type,
        stream,
      },
    }
  }

  const iterator: MultipartIterator = {
    next,
    setLimit(maxBytes: number, path?: string) {
      if (active) {
        active.limit = maxBytes
        if (path) active.path = path
      }
    },
    return: async () => {
      // Only release the underlying reader when no active part is consuming
      // it. Otherwise the active FileStream's pull() would lose access mid-read.
      parserDone = true
      if (!active || active.closed) {
        try {
          reader.releaseLock()
        } catch {
          /* noop */
        }
      }
      return { done: true, value: undefined }
    },
    throw: async (err) => {
      parserDone = true
      if (!active || active.closed) {
        try {
          reader.releaseLock()
        } catch {
          /* noop */
        }
      }
      throw err
    },
    [Symbol.asyncIterator]() {
      return this
    },
  }
  return iterator
}

// ─── byte helpers ─────────────────────────────────────────────────────────

function startsWith(buf: Uint8Array, prefix: Uint8Array): boolean {
  if (buf.length < prefix.length) return false
  for (let i = 0; i < prefix.length; i++) {
    if (buf[i] !== prefix[i]) return false
  }
  return true
}

function indexOf(haystack: Uint8Array, needle: Uint8Array): number {
  if (needle.length === 0) return 0
  if (haystack.length < needle.length) return -1
  const last = haystack.length - needle.length
  outer: for (let i = 0; i <= last; i++) {
    for (let j = 0; j < needle.length; j++) {
      if (haystack[i + j] !== needle[j]) continue outer
    }
    return i
  }
  return -1
}

function concat(a: Uint8Array, b: Uint8Array): Uint8Array {
  if (a.length === 0) return b
  if (b.length === 0) return a
  const out = new Uint8Array(a.length + b.length)
  out.set(a, 0)
  out.set(b, a.length)
  return out
}

function encodeUtf8(s: string): Uint8Array {
  return new TextEncoder().encode(s)
}

function findHeaderEnd(buf: Uint8Array): { end: number; advance: number } | -1 {
  // \r\n\r\n
  const crlfcrlf = indexOf(buf, new Uint8Array([CR, LF, CR, LF]))
  if (crlfcrlf >= 0) return { end: crlfcrlf, advance: crlfcrlf + 4 }
  // \n\n (tolerant)
  const lflf = indexOf(buf, new Uint8Array([LF, LF]))
  if (lflf >= 0) return { end: lflf, advance: lflf + 2 }
  return -1
}

function parseHeaderBlock(bytes: Uint8Array): {
  name: string
  filename?: string
  type: string
} {
  const text = new TextDecoder().decode(bytes)
  let cdisp = ''
  let ctype = ''
  for (const line of text.split(/\r?\n/)) {
    const idx = line.indexOf(':')
    if (idx < 0) continue
    const name = line.slice(0, idx).trim().toLowerCase()
    const value = line.slice(idx + 1).trim()
    if (name === 'content-disposition') cdisp = value
    else if (name === 'content-type') ctype = value
  }
  const { name, filename } = parseContentDisposition(cdisp)
  return { name, filename, type: ctype }
}

function parseContentDisposition(value: string): { name: string; filename?: string } {
  const out: { name: string; filename?: string } = { name: '' }
  for (const segment of value.split(';')) {
    const trimmed = segment.trim()
    const eq = trimmed.indexOf('=')
    if (eq < 0) continue
    const key = trimmed.slice(0, eq).trim().toLowerCase()
    let v = trimmed.slice(eq + 1).trim()
    if (v.startsWith('"') && v.endsWith('"')) {
      v = v.slice(1, -1)
    }
    if (key === 'name') out.name = v
    else if (key === 'filename') out.filename = v
  }
  return out
}
