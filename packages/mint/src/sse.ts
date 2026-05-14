/**
 * Server-Sent Events helper.
 *
 * `sse(generator)` turns an `AsyncGenerator` into a streaming `Response` framed
 * per the SSE spec: each event is one or more `field: value\n` lines followed
 * by a blank line (`\n\n`). Yields are JSON-encoded by default; yielding an
 * {@link SSEMessage} allows you to set `event`, `id`, and `retry` alongside
 * the payload.
 *
 * Phase 11 supports JSON-shaped payloads — multi-line raw strings are not split
 * across `data:` lines. Use `SSEMessage<string>` if you need that and pre-split
 * it yourself.
 */

export interface SSEMessage<T = unknown> {
  data: T
  event?: string
  id?: string
  retry?: number
}

/**
 * Phantom type used by the analyzer and OpenAPI generator to recognize SSE
 * handler return types. Never produced at runtime; only meaningful at the type
 * level via `@Returns<SSEStream<T>>` or a typed handler return.
 */
export interface SSEStream<T = unknown> {
  readonly __mintkit_sse_event: T
}

const encoder = new TextEncoder()

function formatFrame<T>(value: T | SSEMessage<T>): string {
  if (isSSEMessage(value)) {
    let frame = ''
    if (value.event !== undefined) frame += `event: ${value.event}\n`
    if (value.id !== undefined) frame += `id: ${value.id}\n`
    if (value.retry !== undefined) frame += `retry: ${value.retry}\n`
    frame += `data: ${JSON.stringify(value.data)}\n\n`
    return frame
  }
  return `data: ${JSON.stringify(value)}\n\n`
}

function isSSEMessage<T>(value: unknown): value is SSEMessage<T> {
  return (
    typeof value === 'object' &&
    value !== null &&
    'data' in (value as Record<string, unknown>)
  )
}

export function sse<T>(
  generator: () => AsyncGenerator<T | SSEMessage<T>, void, unknown>,
): Response {
  let iter: AsyncGenerator<T | SSEMessage<T>, void, unknown> | undefined

  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      iter = generator()
      try {
        for (;;) {
          const { value, done } = await iter.next()
          if (done) break
          controller.enqueue(encoder.encode(formatFrame(value)))
        }
        controller.close()
      } catch (err) {
        // Don't emit a half-finished frame; just terminate cleanly and surface
        // the error to the consumer via the stream.
        controller.error(err)
      } finally {
        // No-op; cancel() handles the cleanup path when the consumer aborts.
      }
    },
    async cancel() {
      // Consumer canceled the stream — let the generator run its finally
      // block so users can clean up resources held inside `try { ... }`.
      if (iter?.return) {
        try {
          await iter.return()
        } catch {
          // Swallow: cancel is best-effort cleanup and shouldn't throw upward.
        }
      }
    },
  })

  return new Response(stream, {
    headers: {
      'content-type': 'text/event-stream',
      'cache-control': 'no-cache',
      connection: 'keep-alive',
    },
  })
}
