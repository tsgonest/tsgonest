/**
 * BunResponse — lightweight response builder for NestJS on Bun.
 *
 * Supports two modes:
 * 1. **Single-shot** (99% of API requests): NestJS calls end() synchronously
 *    within the handler pipeline. toResponse() builds the web Response with
 *    zero Promises.
 * 2. **Streaming** (SSE, long-polling): NestJS's SseStream calls writeHead()
 *    then write() repeatedly. A ReadableStream is lazily created, and the
 *    Response resolves via getResponse() once the first write() arrives.
 */
export class BunResponse {
  statusCode: number = 200;
  /** @internal */
  _headers: Record<string, string | string[]> = {};
  /** @internal */
  _body: any = null;
  /** @internal */
  _ended: boolean = false;
  /** @internal Tracks whether any header has array values (Set-Cookie). */
  private _hasArrayHeaders: boolean = false;

  // Lazy promise — only allocated for streaming/deferred responses
  private _resolve: ((response: Response) => void) | null = null;
  private _promise: Promise<Response> | null = null;

  // Streaming state (SSE) — only allocated when write() is called
  /** @internal */
  _streamController: ReadableStreamDefaultController<Uint8Array> | null = null;
  private _stream: ReadableStream<Uint8Array> | null = null;
  /** @internal Callback fired when the ReadableStream is cancelled (client disconnect). */
  _onCancel: (() => void) | null = null;

  /** Set the HTTP status code. Returns this for chaining. */
  status(code: number): this {
    this.statusCode = code;
    return this;
  }

  getStatus(): number {
    return this.statusCode;
  }

  /** Set a response header. Set-Cookie is stored as an array per RFC 6265. */
  setHeader(name: string, value: string | string[]): this {
    const key = name.toLowerCase();
    if (key === 'set-cookie') {
      // Set-Cookie headers must NOT be comma-joined (RFC 6265)
      this._headers[key] = Array.isArray(value) ? value : [value];
      this._hasArrayHeaders = true;
    } else {
      this._headers[key] = Array.isArray(value) ? value.join(', ') : value;
    }
    return this;
  }

  /** Get a response header value. */
  getHeader(name: string): string | string[] | undefined {
    return this._headers[name.toLowerCase()];
  }

  /** Append a value to an existing header. */
  appendHeader(name: string, value: string): this {
    const key = name.toLowerCase();
    const existing = this._headers[key];
    if (key === 'set-cookie') {
      // Set-Cookie must be kept as separate header entries
      this._hasArrayHeaders = true;
      if (Array.isArray(existing)) {
        existing.push(value);
      } else if (existing) {
        this._headers[key] = [existing as string, value];
      } else {
        this._headers[key] = [value];
      }
    } else {
      const prev = Array.isArray(existing) ? existing.join(', ') : existing;
      this._headers[key] = prev ? prev + ', ' + value : value;
    }
    return this;
  }

  /** Remove a response header. */
  removeHeader(name: string): this {
    delete this._headers[name.toLowerCase()];
    return this;
  }

  /** Send a JSON response. */
  json(body: any): void {
    this._headers['content-type'] = JSON_CT;
    this.end(JSON.stringify(body));
  }

  /** Send a response body and finalize the response. */
  send(body?: any): void {
    if (typeof body === 'string') {
      if (!this._headers['content-type']) {
        this._headers['content-type'] = 'text/html; charset=utf-8';
      }
      this.end(body);
    } else if (body !== null && body !== undefined && typeof body === 'object') {
      this.json(body);
    } else {
      this.end(body);
    }
  }

  /** Finalize the response. If streaming, close the stream. */
  end(body?: any): void {
    if (this._ended) return;
    this._ended = true;

    if (this._streamController) {
      // Streaming mode: enqueue final chunk and close
      if (body !== undefined && body !== null) {
        this._enqueueChunk(body);
      }
      try { this._streamController.close(); } catch { /* already closed */ }
      this._streamController = null;
      return;
    }

    if (body !== undefined && body !== null) {
      this._body = body;
    }

    // If someone called getResponse() before end() (deferred case),
    // resolve the waiting Promise now.
    if (this._resolve) {
      this._resolve(this._buildResponse());
      this._resolve = null;
    }
  }

  /**
   * Build a Response synchronously. Call ONLY after end() has been called.
   * This is the hot path — no Promise allocation, no microtask.
   */
  toResponse(): Response {
    return this._buildResponse();
  }

  /**
   * Get Response as a Promise. Used when end() might not have been called yet
   * (streaming, SSE, deferred responses). Lazily allocates a Promise.
   */
  getResponse(): Promise<Response> {
    if (this._ended && !this._stream) {
      return Promise.resolve(this._buildResponse());
    }
    // If a stream was already created (SSE: flushHeaders called before getResponse),
    // resolve immediately with the streaming Response.
    if (this._stream) {
      return Promise.resolve(this._buildStreamingResponse());
    }
    // Lazy Promise — only allocated for streaming/deferred case
    if (!this._promise) {
      this._promise = new Promise<Response>((resolve) => {
        this._resolve = resolve;
      });
    }
    return this._promise;
  }

  /** Whether end() has been called. Alias: writableEnded for Node.js compat. */
  get headersSent(): boolean {
    return this._ended;
  }

  /** Node.js WritableStream compat — used by NestJS SseStream guard check. */
  get writableEnded(): boolean {
    return this._ended;
  }

  /** Redirect to a URL. */
  redirect(url: string): void;
  redirect(status: number, url: string): void;
  redirect(statusOrUrl: number | string, url?: string): void {
    if (typeof statusOrUrl === 'string') {
      this.statusCode = 302;
      this._headers['location'] = statusOrUrl;
    } else {
      this.statusCode = statusOrUrl;
      this._headers['location'] = url!;
    }
    this.end();
  }

  // ── Streaming interface (SSE) ─────────────────────────────────

  /**
   * Set status and headers for a streaming response.
   * Called by NestJS SseStream.pipe() to establish the SSE connection.
   */
  writeHead(statusCode: number, headers?: Record<string, string | string[]>): this {
    this.statusCode = statusCode;
    if (headers) {
      for (const [key, value] of Object.entries(headers)) {
        this.setHeader(key, value);
      }
    }
    return this;
  }

  /** Flush headers — no-op for Bun (headers are sent with the first chunk). */
  flushHeaders(): void {
    // Trigger stream creation so getResponse() resolves with the Response
    this._ensureStream();
  }

  /**
   * Write a chunk to the streaming response.
   * Lazily creates a ReadableStream on first call.
   */
  write(chunk: any, encodingOrCallback?: string | Function, callback?: Function): boolean {
    this._ensureStream();
    this._enqueueChunk(chunk);

    const cb = typeof encodingOrCallback === 'function' ? encodingOrCallback : callback;
    if (cb) (cb as Function)();

    return true;
  }

  /**
   * Node.js stream event stubs for `pipe()` compatibility.
   * NestJS's SseStream (a Transform) calls `this.pipe(response)`.
   * Node.js `pipe()` calls `response.on('drain', ...)` etc.
   */
  on(_event: string, _listener: (...args: any[]) => void): this { return this; }
  once(_event: string, _listener: (...args: any[]) => void): this { return this; }
  off(_event: string, _listener: (...args: any[]) => void): this { return this; }
  removeListener(_event: string, _listener: (...args: any[]) => void): this { return this; }
  emit(_event: string, ..._args: any[]): boolean { return false; }

  // Express compat methods
  type(type: string): this {
    this._headers['content-type'] = type;
    return this;
  }

  /** Express-compatible header setter/getter. */
  header(name: string, value?: string): this | string | string[] | undefined {
    if (value === undefined) {
      return this.getHeader(name);
    }
    return this.setHeader(name, value);
  }

  // ── Internal helpers ──────────────────────────────────────────

  /** Ensure the ReadableStream is created for streaming responses. */
  private _ensureStream(): void {
    if (this._stream) return;

    const res = this;
    this._stream = new ReadableStream<Uint8Array>({
      start(controller) {
        res._streamController = controller;
      },
      cancel() {
        // Client disconnected — notify the request for cleanup
        res._ended = true;
        res._streamController = null;
        if (res._onCancel) res._onCancel();
      },
    });

    // If getResponse() was already called and is waiting, resolve it now
    if (this._resolve) {
      this._resolve(this._buildStreamingResponse());
      this._resolve = null;
    }
  }

  /** Encode and enqueue a chunk into the ReadableStream. */
  private _enqueueChunk(chunk: any): void {
    if (!this._streamController) return;
    try {
      if (typeof chunk === 'string') {
        this._streamController.enqueue(encoder.encode(chunk));
      } else if (chunk instanceof Uint8Array) {
        this._streamController.enqueue(chunk);
      } else if (Buffer.isBuffer(chunk)) {
        this._streamController.enqueue(new Uint8Array(chunk));
      } else {
        this._streamController.enqueue(encoder.encode(String(chunk)));
      }
    } catch {
      // Stream may have been cancelled
    }
  }

  /** Build a Response with a ReadableStream body (SSE / streaming). */
  private _buildStreamingResponse(): Response {
    // Fast path: no array headers
    if (!this._hasArrayHeaders) {
      return new Response(this._stream as any, {
        status: this.statusCode,
        headers: this._headers as Record<string, string>,
      });
    }

    const headers = new Headers();
    for (const [key, val] of Object.entries(this._headers)) {
      if (Array.isArray(val)) {
        for (const v of val) headers.append(key, v);
      } else {
        headers.set(key, val);
      }
    }
    return new Response(this._stream as any, {
      status: this.statusCode,
      headers,
    });
  }

  /** @internal Build the final Web API Response (single-shot). */
  private _buildResponse(): Response {
    // If we have a stream, use the streaming response builder
    if (this._stream) {
      return this._buildStreamingResponse();
    }

    const b = this._body;
    let responseBody: Bun.BodyInit | null = null;
    if (b !== null && b !== undefined) {
      if (typeof b === 'string' || b instanceof Uint8Array || b instanceof ArrayBuffer) {
        responseBody = b;
      } else if (typeof b === 'object' && typeof b.getReader === 'function') {
        responseBody = b; // ReadableStream
      } else if (Buffer.isBuffer(b)) {
        responseBody = b;
      } else {
        responseBody = String(b);
      }
    }

    // Fast path (99% of responses): no array-valued headers, pass plain object
    // directly to Response constructor — avoids allocating a Headers object.
    if (!this._hasArrayHeaders) {
      return new Response(responseBody, {
        status: this.statusCode,
        headers: this._headers as Record<string, string>,
      });
    }

    // Slow path: build Headers object for multi-value headers (Set-Cookie)
    const headers = new Headers();
    for (const [key, val] of Object.entries(this._headers)) {
      if (Array.isArray(val)) {
        for (const v of val) {
          headers.append(key, v);
        }
      } else {
        headers.set(key, val);
      }
    }

    return new Response(responseBody, {
      status: this.statusCode,
      headers,
    });
  }
}

/** Pre-allocated constant for the most common content-type. */
const JSON_CT = 'application/json; charset=utf-8';

/** Shared TextEncoder for streaming chunks. */
const encoder = new TextEncoder();
