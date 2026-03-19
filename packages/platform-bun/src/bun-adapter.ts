import { AbstractHttpAdapter } from '@nestjs/core';
import { RequestMethod } from '@nestjs/common';
import type { NestApplicationOptions, VersioningOptions } from '@nestjs/common';
import type { VersionValue } from '@nestjs/common/interfaces';
import { BunRequest } from './bun-request';
import { BunResponse } from './bun-response';
import { executeMiddlewareChain } from './bun-middleware';
import type { MiddlewareEntry } from './interfaces';

/** HTTP methods used for route registration. */
const METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS', 'HEAD'] as const;

/**
 * BunAdapter — NestJS HTTP adapter backed by Bun.serve().
 *
 * Architecture:
 * 1. Collect phase: NestJS registers routes during bootstrap (get/post/put/...)
 * 2. Compile phase: At listen(), convert collected routes into Bun.serve()'s
 *    native `routes` object for C++ radix-tree routing
 * 3. Serve phase: Bun's native router dispatches requests; `fetch` handles fallback
 */
/**
 * Minimal EventEmitter-compatible stub so NestJS core can call
 * httpServer.once('error', ...) and httpServer.address() before listen().
 */
function createServerStub(): any {
  const listeners: Map<string, Function[]> = new Map();
  return {
    on(event: string, fn: Function) { const arr = listeners.get(event) || []; arr.push(fn); listeners.set(event, arr); return this; },
    once(event: string, fn: Function) { this.on(event, fn); return this; },
    off(event: string, fn: Function) { const arr = listeners.get(event); if (arr) listeners.set(event, arr.filter(f => f !== fn)); return this; },
    emit(event: string, ...args: any[]) { const arr = listeners.get(event); if (arr) arr.forEach(fn => fn(...args)); },
    removeListener(event: string, fn: Function) { return this.off(event, fn); },
    address() { return null; },
    close(cb?: Function) { if (cb) cb(); },
    listen() {},
  };
}

export class BunAdapter extends AbstractHttpAdapter {
  private _server: any = null;
  private _routeMap: Map<string, Map<string, (...args: any[]) => any>> = new Map();
  private _middlewares: MiddlewareEntry[] = [];
  private _tlsOptions: any = undefined;
  private _notFoundHandler: ((...args: any[]) => any) | null = null;
  private _errorHandler: ((...args: any[]) => any) | null = null;
  private _isListening: boolean = false;

  constructor() {
    // Pass a stub with EventEmitter methods so NestJS core can call
    // httpServer.once('error', ...) during bootstrap before listen() is called
    super(createServerStub());
    this.httpServer = createServerStub();
  }

  /** Initialize HTTP server options. Server creation is deferred to listen(). */
  public initHttpServer(options: NestApplicationOptions): void {
    if (options?.httpsOptions) {
      this._tlsOptions = options.httpsOptions;
    }
    // Ensure httpServer stub is set for NestJS core
    if (!this.httpServer) {
      this.httpServer = createServerStub();
    }
  }

  /** Start the Bun HTTP server with collected routes compiled into native routing. */
  public async listen(port: string | number, ...args: any[]): Promise<any> {
    const portNum = typeof port === 'string' ? parseInt(port, 10) : port;

    let hostname: string | undefined;
    let callback: (() => void) | undefined;

    for (const arg of args) {
      if (typeof arg === 'string') hostname = arg;
      if (typeof arg === 'function') callback = arg;
    }

    // Compile collected routes into Bun.serve()'s native `routes` object.
    // Bun routes use a C++ radix-tree for O(1) matching — significantly faster
    // than JavaScript regex matching for parameterized routes.
    const nativeRoutes = this._buildNativeRoutes();
    const adapter = this;

    this._server = Bun.serve({
      port: portNum,
      hostname,
      tls: this._tlsOptions,

      // Native C++ radix-tree routing — handles all registered routes
      routes: nativeRoutes,

      // Fallback: only reached for routes not registered via NestJS (404s)
      async fetch(req: Request, _server: any): Promise<Response> {
        const bunReq = new BunRequest(req);
        const bunRes = new BunResponse();

        if (adapter._notFoundHandler) {
          adapter._notFoundHandler(bunReq, bunRes);
        } else {
          bunRes.status(404).json({
            statusCode: 404,
            message: 'Cannot ' + bunReq.method + ' ' + bunReq.url,
          });
        }

        if (bunRes._ended) return bunRes.toResponse();
        return bunRes.getResponse();
      },

      error(): Response {
        return new Response(
          '{"statusCode":500,"message":"Internal Server Error"}',
          { status: 500, headers: { 'content-type': 'application/json' } },
        );
      },
    });

    this._isListening = true;

    // Update the stub's address() to return real server info.
    // NestJS core calls httpServer.address() after the listen callback
    // to verify the server is ready.
    const serverRef = this._server;
    this.httpServer.address = () => ({
      address: hostname || '0.0.0.0',
      family: 'IPv4',
      port: serverRef.port,
    });

    if (callback) callback();
    return this.httpServer;
  }

  /** Stop the Bun HTTP server. */
  public async close(): Promise<void> {
    if (this._server) {
      this._server.stop(true);
      this._server = null;
      this._isListening = false;
      this.httpServer.address = () => null;
    }
  }

  // ── Route registration (collect phase) ────────────────────────────

  public get(path: string, handler: (...args: any[]) => any): void;
  public get(handler: (...args: any[]) => any): void;
  public get(pathOrHandler: string | ((...args: any[]) => any), handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'GET', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'GET', handler!);
    }
  }

  public post(handler: (...args: any[]) => any): void;
  public post(path: any, handler: (...args: any[]) => any): void;
  public post(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'POST', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'POST', handler!);
    }
  }

  public put(handler: (...args: any[]) => any): void;
  public put(path: any, handler: (...args: any[]) => any): void;
  public put(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'PUT', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'PUT', handler!);
    }
  }

  public delete(handler: (...args: any[]) => any): void;
  public delete(path: any, handler: (...args: any[]) => any): void;
  public delete(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'DELETE', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'DELETE', handler!);
    }
  }

  public patch(handler: (...args: any[]) => any): void;
  public patch(path: any, handler: (...args: any[]) => any): void;
  public patch(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'PATCH', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'PATCH', handler!);
    }
  }

  public options(handler: (...args: any[]) => any): void;
  public options(path: any, handler: (...args: any[]) => any): void;
  public options(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'OPTIONS', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'OPTIONS', handler!);
    }
  }

  public head(handler: (...args: any[]) => any): void;
  public head(path: any, handler: (...args: any[]) => any): void;
  public head(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    if (typeof pathOrHandler === 'function') {
      this._addRoute('/', 'HEAD', pathOrHandler);
    } else {
      this._addRoute(pathOrHandler, 'HEAD', handler!);
    }
  }

  public all(handler: (...args: any[]) => any): void;
  public all(path: any, handler: (...args: any[]) => any): void;
  public all(pathOrHandler: any, handler?: (...args: any[]) => any): void {
    const fn = typeof pathOrHandler === 'function' ? pathOrHandler : handler!;
    const path = typeof pathOrHandler === 'string' ? pathOrHandler : '/';
    for (const method of METHODS) {
      this._addRoute(path, method, fn);
    }
  }

  // ── Middleware ────────────────────────────────────────────────────

  public use(...args: any[]): void {
    if (args.length === 1 && typeof args[0] === 'function') {
      // Global middleware: use(handler)
      this._middlewares.push({ path: null, handler: args[0] });
    } else if (args.length === 2 && typeof args[0] === 'string' && typeof args[1] === 'function') {
      // Path-scoped middleware: use(path, handler)
      this._middlewares.push({ path: args[0], handler: args[1] });
    }
  }

  // ── Response helpers (called by NestJS internals) ─────────────────

  public reply(response: BunResponse, body: any, statusCode?: number): any {
    if (statusCode) {
      response.status(statusCode);
    }

    if (body === undefined || body === null) {
      response.end();
      return;
    }

    if (typeof body === 'string') {
      response.send(body);
    } else if (typeof body === 'object') {
      // Check for StreamableFile (from @nestjs/common)
      if (typeof body.getStream === 'function') {
        const stream = body.getStream();
        if (body.getHeaders) {
          const headers = body.getHeaders();
          if (headers.type) response.setHeader('content-type', headers.type);
          if (headers.disposition) response.setHeader('content-disposition', headers.disposition);
          if (headers.length !== undefined) response.setHeader('content-length', String(headers.length));
        }
        response.end(stream);
      } else {
        response.json(body);
      }
    } else {
      response.send(String(body));
    }
  }

  public status(response: BunResponse, statusCode: number): BunResponse {
    response.status(statusCode);
    return response;
  }

  public end(response: BunResponse, message?: string): void {
    response.end(message);
  }

  public render(response: BunResponse, view: string, options: any): any {
    throw new Error('Render is not supported by BunAdapter. Use a dedicated template engine.');
  }

  public redirect(response: BunResponse, statusCode: number, url: string): void {
    response.redirect(statusCode, url);
  }

  public setErrorHandler(handler: (...args: any[]) => any): void {
    this._errorHandler = handler;
  }

  public setNotFoundHandler(handler: (...args: any[]) => any): void {
    this._notFoundHandler = handler;
  }

  public isHeadersSent(response: BunResponse): boolean {
    return response.headersSent;
  }

  public setHeader(response: BunResponse, name: string, value: string): BunResponse {
    response.setHeader(name, value);
    return response;
  }

  public appendHeader(response: BunResponse, name: string, value: string): BunResponse {
    response.appendHeader(name, value);
    return response;
  }

  public getRequestHostname(request: BunRequest): string {
    return request.hostname;
  }

  public getRequestMethod(request: BunRequest): string {
    return request.method;
  }

  public getRequestUrl(request: BunRequest): string {
    return request.originalUrl;
  }

  public useStaticAssets(..._args: any[]): void {
    throw new Error('Static assets are not supported by BunAdapter. Use Bun.file() or a reverse proxy.');
  }

  public setViewEngine(_engine: string): void {
    throw new Error('View engine is not supported by BunAdapter. Use a dedicated template engine.');
  }

  public getHeader(response: BunResponse, name: string): string | string[] | undefined {
    return response.getHeader(name);
  }

  public applyVersionFilter(
    handler: Function,
    _version: VersionValue,
    _versioningOptions: VersioningOptions,
  ): (req: BunRequest, res: BunResponse, next: () => void) => Function {
    // Basic pass-through — versioning is handled at the NestJS routing level
    return handler as any;
  }

  public getType(): string {
    return 'bun';
  }

  public registerParserMiddleware(): void {
    // Bun has built-in body parsing — register a middleware that eagerly parses
    this._middlewares.unshift({
      path: null,
      handler: (req: BunRequest, _res: BunResponse, next: () => void) => {
        if (req.body !== undefined) {
          next();
          return;
        }

        // Skip body parsing for methods that typically have no body
        const m = req.method;
        if (m === 'GET' || m === 'HEAD' || m === 'OPTIONS') {
          next();
          return;
        }

        // Async body parsing — only reached for POST/PUT/PATCH/DELETE
        parseRequestBody(req).then(next, next);
        return;
      },
    });
  }

  public enableCors(options?: any): void {
    const opts = {
      origin: '*',
      methods: 'GET,HEAD,PUT,PATCH,POST,DELETE',
      preflightContinue: false,
      optionsSuccessStatus: 204,
      credentials: false,
      ...options,
    };

    this._middlewares.push({
      path: null,
      handler: (req: BunRequest, res: BunResponse, next: () => void) => {
        const origin = req.get('origin') || '';

        // Set CORS headers
        if (typeof opts.origin === 'string') {
          res.setHeader('access-control-allow-origin', opts.origin);
        } else if (typeof opts.origin === 'function') {
          const result = opts.origin(origin);
          if (result) res.setHeader('access-control-allow-origin', typeof result === 'string' ? result : origin);
        } else if (opts.origin === true) {
          res.setHeader('access-control-allow-origin', origin);
        }

        if (opts.credentials) {
          res.setHeader('access-control-allow-credentials', 'true');
        }

        if (typeof opts.methods === 'string') {
          res.setHeader('access-control-allow-methods', opts.methods);
        }

        if (opts.allowedHeaders) {
          const headers = Array.isArray(opts.allowedHeaders)
            ? opts.allowedHeaders.join(',')
            : opts.allowedHeaders;
          res.setHeader('access-control-allow-headers', headers);
        }

        if (opts.exposedHeaders) {
          const headers = Array.isArray(opts.exposedHeaders)
            ? opts.exposedHeaders.join(',')
            : opts.exposedHeaders;
          res.setHeader('access-control-expose-headers', headers);
        }

        if (opts.maxAge !== undefined) {
          res.setHeader('access-control-max-age', String(opts.maxAge));
        }

        // Handle preflight
        if (req.method === 'OPTIONS' && !opts.preflightContinue) {
          res.status(opts.optionsSuccessStatus).end();
          return;
        }

        next();
      },
    });
  }

  public createMiddlewareFactory(requestMethod: RequestMethod): (path: string, callback: Function) => void {
    return (path: string, callback: Function) => {
      this._middlewares.push({ path, handler: callback as MiddlewareEntry['handler'] });
    };
  }

  public getHttpServer(): any {
    return this.httpServer;
  }

  public getInstance(): any {
    return this._server;
  }

  // ── Internal helpers ──────────────────────────────────────────────

  private _addRoute(path: string, method: string, handler: (...args: any[]) => any): void {
    // Normalize path
    const normalizedPath = path.startsWith('/') ? path : '/' + path;

    if (!this._routeMap.has(normalizedPath)) {
      this._routeMap.set(normalizedPath, new Map());
    }
    this._routeMap.get(normalizedPath)!.set(method.toUpperCase(), handler);
  }

  /**
   * Build a Bun.serve() native `routes` object from collected NestJS routes.
   *
   * Bun's native router uses a C++ radix-tree that matches URL patterns
   * (including :param and * wildcards) before JavaScript even runs.
   * Each route handler wraps the NestJS handler with middleware and error
   * handling, then returns a web-native Response.
   */
  private _buildNativeRoutes(): Record<string, any> {
    const routes: Record<string, any> = {};
    const adapter = this;
    const middlewares = this._middlewares;
    const hasMiddleware = middlewares.length > 0;
    const noopNext = () => {};

    for (const [path, methods] of this._routeMap) {
      const methodHandlers: Record<string, (req: Request) => Promise<Response>> = {};

      for (const [method, handler] of methods) {
        methodHandlers[method] = async (req: Request): Promise<Response> => {
          const bunReq = new BunRequest(req);
          const bunRes = new BunResponse();

          // Bun's native router populates req.params for :param routes
          const params = (req as any).params;
          if (params) bunReq.params = params;

          await handleRoute(adapter, middlewares, hasMiddleware, handler, bunReq, bunRes, noopNext);

          if (bunRes._ended) return bunRes.toResponse();
          return bunRes.getResponse();
        };
      }

      routes[path] = methodHandlers;
    }

    return routes;
  }
}

/** Parse request body based on content-type. Extracted from the body parser
 *  middleware so the early-exit path (GET/HEAD/OPTIONS) stays synchronous. */
async function parseRequestBody(req: BunRequest): Promise<void> {
  const contentType = req.get('content-type') || '';
  const raw = req.raw;

  try {
    if (contentType.includes('application/json')) {
      req.body = await raw.json();
    } else if (contentType.includes('text/')) {
      req.body = await raw.text();
    } else if (contentType.includes('multipart/form-data')) {
      const formData = await raw.formData();
      const body: Record<string, any> = {};
      for (const [key, value] of formData.entries()) {
        if (value instanceof File) {
          const existing = body[key];
          if (existing instanceof File) {
            body[key] = [existing, value];
          } else if (Array.isArray(existing)) {
            existing.push(value);
          } else {
            body[key] = value;
          }
        } else {
          body[key] = value;
        }
      }
      req.body = body;
    } else if (contentType.includes('application/x-www-form-urlencoded')) {
      const text = await raw.text();
      const params = new URLSearchParams(text);
      const body: Record<string, string> = {};
      params.forEach((value, key) => { body[key] = value; });
      req.body = body;
    } else if (raw.body) {
      const text = await raw.text();
      try {
        req.body = JSON.parse(text);
      } catch {
        req.body = text;
      }
    }
  } catch {
    req.body = undefined;
  }
}

/** Execute a route handler with middleware and error handling. */
async function handleRoute(
  adapter: BunAdapter,
  middlewares: MiddlewareEntry[],
  hasMiddleware: boolean,
  handler: Function,
  bunReq: BunRequest,
  bunRes: BunResponse,
  noopNext: () => void,
): Promise<void> {
  if (hasMiddleware) {
    await executeMiddlewareChain(
      middlewares,
      bunReq.url,
      bunReq,
      bunRes,
      async () => {
        try {
          await handler(bunReq, bunRes, noopNext);
        } catch (err: any) {
          handleError(adapter, err, bunReq, bunRes);
        }
      },
    );
  } else {
    try {
      await handler(bunReq, bunRes, noopNext);
    } catch (err: any) {
      handleError(adapter, err, bunReq, bunRes);
    }
  }
}

/** Handle a route error via the adapter's error handler or default 500. */
function handleError(adapter: BunAdapter, err: any, bunReq: BunRequest, bunRes: BunResponse): void {
  if (bunRes.headersSent) return;
  if (adapter['_errorHandler']) {
    adapter['_errorHandler'](err, bunReq, bunRes);
  } else {
    bunRes.status(500).json({
      statusCode: 500,
      message: err?.message || 'Internal Server Error',
    });
  }
}
