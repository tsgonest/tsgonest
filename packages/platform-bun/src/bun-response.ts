/**
 * BunResponse — lightweight response builder for NestJS on Bun.
 *
 * Key performance insight: NO Promise allocation in the constructor.
 * In 99% of API requests, NestJS calls end() synchronously within the
 * handler pipeline. After `await handler(...)` returns, the response is
 * already finalized. We build the Web API Response synchronously via
 * toResponse() — zero Promises, zero microtask hops.
 *
 * For the rare streaming/deferred case (SSE, long-polling), getResponse()
 * lazily creates a Promise on demand.
 */
export class BunResponse {
  statusCode: number = 200;
  /** @internal */
  _headers: Record<string, string> = {};
  /** @internal */
  _body: any = null;
  /** @internal */
  _ended: boolean = false;

  // Lazy promise — only allocated for streaming/deferred responses
  private _resolve: ((response: Response) => void) | null = null;
  private _promise: Promise<Response> | null = null;

  /** Set the HTTP status code. Returns this for chaining. */
  status(code: number): this {
    this.statusCode = code;
    return this;
  }

  getStatus(): number {
    return this.statusCode;
  }

  /** Set a response header. */
  setHeader(name: string, value: string | string[]): this {
    this._headers[name.toLowerCase()] = Array.isArray(value) ? value.join(', ') : value;
    return this;
  }

  /** Get a response header value. */
  getHeader(name: string): string | undefined {
    return this._headers[name.toLowerCase()];
  }

  /** Append a value to an existing header. */
  appendHeader(name: string, value: string): this {
    const key = name.toLowerCase();
    const existing = this._headers[key];
    this._headers[key] = existing ? existing + ', ' + value : value;
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

  /** Finalize the response. If a deferred Promise exists, resolve it. */
  end(body?: any): void {
    if (this._ended) return;
    this._ended = true;

    if (body !== undefined && body !== null) {
      this._body = body;
    }

    // If someone called getResponse() before end() (streaming case),
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
    if (this._ended) {
      return Promise.resolve(this._buildResponse());
    }
    // Lazy Promise — only allocated for streaming edge case
    if (!this._promise) {
      this._promise = new Promise<Response>((resolve) => {
        this._resolve = resolve;
      });
    }
    return this._promise;
  }

  /** Whether end() has been called. */
  get headersSent(): boolean {
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

  // Express compat methods
  type(type: string): this {
    this._headers['content-type'] = type;
    return this;
  }

  /** Express-compatible header setter/getter. */
  header(name: string, value?: string): this | string | undefined {
    if (value === undefined) {
      return this.getHeader(name);
    }
    return this.setHeader(name, value);
  }

  /** @internal Build the final Web API Response. */
  private _buildResponse(): Response {
    const b = this._body;
    let responseBody: BodyInit | null = null;
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

    return new Response(responseBody, {
      status: this.statusCode,
      headers: this._headers,
    });
  }
}

/** Pre-allocated constant for the most common content-type. */
const JSON_CT = 'application/json; charset=utf-8';
