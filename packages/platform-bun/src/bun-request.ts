/**
 * BunRequest wraps a Web API Request with Express-like properties
 * expected by NestJS internals.
 *
 * Performance: lazy parsing — URL is sliced (not `new URL()`), headers and
 * query params are parsed on first access, not eagerly in the constructor.
 */
export class BunRequest {
  /** Pre-parsed body (set by body parser middleware). */
  body: any = undefined;

  /** Route params (set by router). */
  params: Record<string, string> = emptyParams;

  /** HTTP method. */
  readonly method: string;

  /** Request pathname (without query string). */
  readonly url: string;

  /** Original full URL string (pathname + search). */
  readonly originalUrl: string;

  /** Client IP address (set after server is available). */
  ip: string = '';

  /** The underlying Web API Request. */
  readonly raw: Request;

  /**
   * Stub socket for NestJS SSE compatibility.
   * SseStream calls req.socket.setKeepAlive/setNoDelay/setTimeout —
   * Bun manages TCP settings internally, so these are no-ops.
   */
  readonly socket = sseSocketStub;

  // Lazily initialized backing fields
  private _headers: Record<string, string> | null = null;
  private _query: Record<string, string> | null = null;
  private _hostname: string | null = null;
  private _search: string;

  // Real event listeners for 'close' — needed for SSE disconnect cleanup
  private _closeListeners: Array<() => void> | null = null;

  constructor(request: Request) {
    this.raw = request;
    // Attach Node.js compat stubs to the raw Request for SSE support.
    // NestJS SSE does: (req as any).raw || req → gets this.raw
    // SseStream expects req.socket and req.on('close', ...)
    const self = this;
    (request as any).socket = sseSocketStub;
    (request as any).on = (event: string, listener: Function) => {
      if (event === 'close') self.on('close', listener as any);
      return request;
    };
    this.method = request.method;

    // Fast URL parsing via string slicing — avoids `new URL()` overhead.
    // request.url is always "http://host:port/path?query" from Bun.serve().
    const url = request.url;
    // Find start of path (skip scheme + "://" + host)
    const pathStart = url.indexOf('/', url.indexOf('://') + 3);
    const qIdx = url.indexOf('?', pathStart);
    if (qIdx === -1) {
      this.url = url.substring(pathStart);
      this.originalUrl = this.url;
      this._search = '';
    } else {
      this.url = url.substring(pathStart, qIdx);
      this._search = url.substring(qIdx);
      this.originalUrl = this.url + this._search;
    }
  }

  /** Lazily parsed headers — only materialized when first accessed. */
  get headers(): Record<string, string> {
    if (this._headers === null) {
      const hdrs: Record<string, string> = {};
      this.raw.headers.forEach((value, key) => { hdrs[key] = value; });
      this._headers = hdrs;
    }
    return this._headers;
  }

  /** Lazily parsed query parameters. */
  get query(): Record<string, string> {
    if (this._query === null) {
      if (this._search === '') {
        this._query = emptyParams;
      } else {
        const q: Record<string, string> = {};
        const params = new URLSearchParams(this._search);
        params.forEach((value, key) => { q[key] = value; });
        this._query = q;
      }
    }
    return this._query;
  }

  /** Lazily resolved hostname. */
  get hostname(): string {
    if (this._hostname === null) {
      // Fast: read the Host header directly instead of parsing URL
      this._hostname = this.raw.headers.get('host')?.split(':')[0] || '';
    }
    return this._hostname;
  }

  /** Get a header value (case-insensitive). */
  get(name: string): string | undefined {
    // Fast path: read directly from raw headers to avoid materializing all headers
    return this.raw.headers.get(name) ?? undefined;
  }

  header(name: string): string | undefined {
    return this.get(name);
  }

  // EventEmitter-like interface for NestJS compat.
  // 'close' listeners are real — fired when SSE client disconnects.
  on(event: string, listener: (...args: any[]) => void): this {
    if (event === 'close') {
      if (!this._closeListeners) this._closeListeners = [];
      this._closeListeners.push(listener);
    }
    return this;
  }
  once(event: string, listener: (...args: any[]) => void): this {
    return this.on(event, listener);
  }
  off(_event: string, _listener: (...args: any[]) => void): this { return this; }
  emit(_event: string, ..._args: any[]): boolean { return false; }

  /**
   * @internal Fire 'close' event — called by the adapter when an SSE
   * ReadableStream is cancelled (client disconnected).
   */
  _emitClose(): void {
    if (this._closeListeners) {
      for (const listener of this._closeListeners) {
        listener();
      }
    }
  }
}

/** Shared empty params object — avoids allocation when route has no params. */
const emptyParams: Record<string, string> = Object.freeze(Object.create(null));

/** Shared stub socket for SSE compatibility — no-op TCP methods. */
const sseSocketStub = Object.freeze({
  setKeepAlive() {},
  setNoDelay() {},
  setTimeout() {},
});
