import type { Container, Constructor } from './container'
import { Token } from './token'

export interface EventResponseDraft {
  headers: Headers
  status: number
}

export interface Body {
  /** Reads the raw request body once and caches it for subsequent calls. */
  bytes(): Promise<Uint8Array>
  /** Decodes the cached bytes as UTF-8 text. */
  text(): Promise<string>
  /** Parses the cached text as JSON. */
  json<T = unknown>(): Promise<T>
  /** Returns a `FormData` parsed from the cached body. */
  formData(): Promise<FormData>
  /**
   * Hands back the original `ReadableStream` for streaming use cases. Mutually
   * exclusive with `bytes`/`text`/`json`/`formData` — calling either side after
   * the other throws.
   */
  stream(): ReadableStream<Uint8Array>
}

/**
 * Per-request execution context. Carries the incoming `Request`, a mutable
 * response draft for middleware to influence headers/status, and a small
 * token-store backed by a `Map`. Adapters may seed token-store values via the
 * `setup` callback on `App.createEvent`.
 */
export class Event {
  readonly request: Request
  readonly response: EventResponseDraft
  readonly body: Body
  /**
   * Path params extracted from the route template (e.g. `/users/:id` →
   * `{ id: '42' }`). Empty object for routes without `:name` segments.
   * Populated by `App.fetch` from the router match result.
   */
  params: Record<string, string>
  /**
   * Promises passed to `waitUntil`. The adapter (e.g. `@mintkit/bun`) is
   * responsible for awaiting them after the response is sent; core only
   * records them.
   */
  readonly waitUntilPromises = new Set<Promise<unknown>>()
  private readonly container: Container
  private readonly store = new Map<Token<unknown>, unknown>()

  constructor(request: Request, container: Container) {
    this.request = request
    this.container = container
    this.response = { headers: new Headers(), status: 200 }
    this.body = createBody(request)
    this.params = {}
  }

  resolve<T>(provider: Token<T> | Constructor<T>): T {
    return this.container.resolve(provider)
  }

  set<T>(token: Token<T>, value: T): void {
    this.store.set(token as Token<unknown>, value)
  }

  get<T>(token: Token<T>): T | undefined {
    return this.store.get(token as Token<unknown>) as T | undefined
  }

  require<T>(token: Token<T>): T {
    if (!this.store.has(token as Token<unknown>)) {
      throw new Error(`Event store has no entry for Token(${token.name})`)
    }
    return this.store.get(token as Token<unknown>) as T
  }

  /**
   * Records a promise that should outlive the response. The runtime adapter
   * (Bun in v1) decides what "outlive" means; core just stores the reference.
   */
  waitUntil(promise: Promise<unknown>): void {
    this.waitUntilPromises.add(promise)
  }
}

function createBody(request: Request): Body {
  let bytesPromise: Promise<Uint8Array> | undefined
  let textPromise: Promise<string> | undefined
  let streamTaken = false

  const readBytes = (): Promise<Uint8Array> => {
    if (streamTaken) {
      return Promise.reject(new Error('body stream already consumed'))
    }
    if (!bytesPromise) {
      bytesPromise = request
        .arrayBuffer()
        .then((buf) => new Uint8Array(buf))
    }
    return bytesPromise
  }

  return {
    bytes: readBytes,
    async text() {
      if (!textPromise) {
        textPromise = readBytes().then((b) => new TextDecoder().decode(b))
      }
      return textPromise
    },
    async json<T = unknown>(): Promise<T> {
      const t = await this.text()
      return JSON.parse(t) as T
    },
    async formData(): Promise<FormData> {
      const bytes = await readBytes()
      // Build a fake Response carrying the cached bytes + original content-type
      // and use the platform's FormData parser. Works for both
      // `application/x-www-form-urlencoded` and `multipart/form-data`.
      const ct = request.headers.get('content-type') ?? ''
      return await new Response(bytes, {
        headers: { 'content-type': ct },
      }).formData()
    },
    stream(): ReadableStream<Uint8Array> {
      if (bytesPromise !== undefined) {
        throw new Error('body stream already consumed')
      }
      if (streamTaken) {
        throw new Error('body stream already consumed')
      }
      streamTaken = true
      const body = request.body
      if (!body) {
        // Synthesize an empty stream for GETs / no-body requests so callers
        // can always assume the return type.
        return new ReadableStream<Uint8Array>({
          start(controller) {
            controller.close()
          },
        })
      }
      return body as ReadableStream<Uint8Array>
    },
  }
}
