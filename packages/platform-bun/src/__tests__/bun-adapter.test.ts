import { describe, it, expect, vi, beforeEach } from 'vitest';
import { BunAdapter } from '../bun-adapter';
import { BunRequest } from '../bun-request';
import { BunResponse } from '../bun-response';

/**
 * Unit tests for BunAdapter — tests route collection, response helpers,
 * and adapter methods. No Bun.serve() or network calls.
 *
 * We access private fields via type assertions where needed to verify
 * internal state without starting a server.
 */

describe('BunAdapter', () => {
  let adapter: BunAdapter;

  beforeEach(() => {
    adapter = new BunAdapter();
  });

  describe('getType()', () => {
    it('returns "bun"', () => {
      expect(adapter.getType()).toBe('bun');
    });
  });

  describe('route registration', () => {
    // Helper to access the private _routeMap
    function getRouteMap(a: BunAdapter): Map<string, Map<string, Function>> {
      return (a as any)._routeMap;
    }

    it('get(path, handler) stores a GET route', () => {
      const handler = vi.fn();
      adapter.get('/users', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.has('/users')).toBe(true);
      expect(routeMap.get('/users')!.get('GET')).toBe(handler);
    });

    it('post(path, handler) stores a POST route', () => {
      const handler = vi.fn();
      adapter.post('/users', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/users')!.get('POST')).toBe(handler);
    });

    it('put(path, handler) stores a PUT route', () => {
      const handler = vi.fn();
      adapter.put('/users', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/users')!.get('PUT')).toBe(handler);
    });

    it('delete(path, handler) stores a DELETE route', () => {
      const handler = vi.fn();
      adapter.delete('/users', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/users')!.get('DELETE')).toBe(handler);
    });

    it('patch(path, handler) stores a PATCH route', () => {
      const handler = vi.fn();
      adapter.patch('/items', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/items')!.get('PATCH')).toBe(handler);
    });

    it('options(path, handler) stores an OPTIONS route', () => {
      const handler = vi.fn();
      adapter.options('/preflight', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/preflight')!.get('OPTIONS')).toBe(handler);
    });

    it('head(path, handler) stores a HEAD route', () => {
      const handler = vi.fn();
      adapter.head('/ping', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/ping')!.get('HEAD')).toBe(handler);
    });

    it('get(handler) defaults path to "/"', () => {
      const handler = vi.fn();
      adapter.get(handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/')!.get('GET')).toBe(handler);
    });

    it('post(handler) defaults path to "/"', () => {
      const handler = vi.fn();
      adapter.post(handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.get('/')!.get('POST')).toBe(handler);
    });

    it('all(path, handler) registers for all 7 methods', () => {
      const handler = vi.fn();
      adapter.all('/everything', handler);

      const routeMap = getRouteMap(adapter);
      const methods = routeMap.get('/everything')!;
      expect(methods.get('GET')).toBe(handler);
      expect(methods.get('POST')).toBe(handler);
      expect(methods.get('PUT')).toBe(handler);
      expect(methods.get('DELETE')).toBe(handler);
      expect(methods.get('PATCH')).toBe(handler);
      expect(methods.get('OPTIONS')).toBe(handler);
      expect(methods.get('HEAD')).toBe(handler);
      expect(methods.size).toBe(7);
    });

    it('all(handler) defaults path to "/"', () => {
      const handler = vi.fn();
      adapter.all(handler);

      const routeMap = getRouteMap(adapter);
      const methods = routeMap.get('/')!;
      expect(methods.size).toBe(7);
    });

    it('normalizes path to start with /', () => {
      const handler = vi.fn();
      adapter.get('users', handler);

      const routeMap = getRouteMap(adapter);
      expect(routeMap.has('/users')).toBe(true);
      expect(routeMap.has('users')).toBe(false);
    });

    it('registers multiple methods on the same path', () => {
      const getHandler = vi.fn();
      const postHandler = vi.fn();
      adapter.get('/items', getHandler);
      adapter.post('/items', postHandler);

      const routeMap = getRouteMap(adapter);
      const methods = routeMap.get('/items')!;
      expect(methods.get('GET')).toBe(getHandler);
      expect(methods.get('POST')).toBe(postHandler);
    });
  });

  describe('middleware registration', () => {
    function getMiddlewares(a: BunAdapter): Array<{ path: string | null; handler: Function }> {
      return (a as any)._middlewares;
    }

    it('use(handler) registers global middleware', () => {
      const handler = vi.fn();
      adapter.use(handler);

      const mws = getMiddlewares(adapter);
      expect(mws).toHaveLength(1);
      expect(mws[0].path).toBeNull();
      expect(mws[0].handler).toBe(handler);
    });

    it('use(path, handler) registers path-scoped middleware', () => {
      const handler = vi.fn();
      adapter.use('/api', handler);

      const mws = getMiddlewares(adapter);
      expect(mws).toHaveLength(1);
      expect(mws[0].path).toBe('/api');
      expect(mws[0].handler).toBe(handler);
    });

    it('use() handles multiple registrations', () => {
      adapter.use(vi.fn());
      adapter.use('/admin', vi.fn());
      adapter.use(vi.fn());

      const mws = getMiddlewares(adapter);
      expect(mws).toHaveLength(3);
      expect(mws[0].path).toBeNull();
      expect(mws[1].path).toBe('/admin');
      expect(mws[2].path).toBeNull();
    });
  });

  describe('setErrorHandler() / setNotFoundHandler()', () => {
    it('stores error handler', () => {
      const handler = vi.fn();
      adapter.setErrorHandler(handler);
      expect((adapter as any)._errorHandler).toBe(handler);
    });

    it('stores not found handler', () => {
      const handler = vi.fn();
      adapter.setNotFoundHandler(handler);
      expect((adapter as any)._notFoundHandler).toBe(handler);
    });
  });

  describe('reply()', () => {
    it('calls send() for string body', () => {
      const res = new BunResponse();
      adapter.reply(res, 'hello');

      expect(res._body).toBe('hello');
      expect(res._ended).toBe(true);
    });

    it('calls json() for object body', () => {
      const res = new BunResponse();
      adapter.reply(res, { id: 1 });

      expect(res._body).toBe('{"id":1}');
      expect(res.getHeader('content-type')).toBe('application/json; charset=utf-8');
      expect(res._ended).toBe(true);
    });

    it('calls end() for null body', () => {
      const res = new BunResponse();
      adapter.reply(res, null);

      expect(res._ended).toBe(true);
      expect(res._body).toBeNull();
    });

    it('calls end() for undefined body', () => {
      const res = new BunResponse();
      adapter.reply(res, undefined);

      expect(res._ended).toBe(true);
    });

    it('sets status code before sending when provided', () => {
      const res = new BunResponse();
      adapter.reply(res, { ok: true }, 201);

      expect(res.statusCode).toBe(201);
      expect(res._ended).toBe(true);
    });

    it('does not set status when statusCode is not provided', () => {
      const res = new BunResponse();
      adapter.reply(res, 'ok');

      expect(res.statusCode).toBe(200); // default
    });

    it('handles StreamableFile-like objects', () => {
      const res = new BunResponse();
      const mockStream = { pipe: vi.fn() };
      const streamableFile = {
        getStream: () => mockStream,
        getHeaders: () => ({
          type: 'application/octet-stream',
          disposition: 'attachment; filename="file.bin"',
          length: 1024,
        }),
      };

      adapter.reply(res, streamableFile);

      expect(res.getHeader('content-type')).toBe('application/octet-stream');
      expect(res.getHeader('content-disposition')).toBe('attachment; filename="file.bin"');
      expect(res.getHeader('content-length')).toBe('1024');
      expect(res._body).toBe(mockStream);
      expect(res._ended).toBe(true);
    });

    it('sends numeric body as string', () => {
      const res = new BunResponse();
      adapter.reply(res, 42);

      expect(res._body).toBe('42');
      expect(res._ended).toBe(true);
    });
  });

  describe('status()', () => {
    it('sets status on BunResponse and returns it', () => {
      const res = new BunResponse();
      const result = adapter.status(res, 404);

      expect(res.statusCode).toBe(404);
      expect(result).toBe(res);
    });
  });

  describe('setHeader() / appendHeader() / getHeader()', () => {
    it('setHeader delegates to BunResponse', () => {
      const res = new BunResponse();
      const result = adapter.setHeader(res, 'X-Test', 'value');

      expect(res.getHeader('x-test')).toBe('value');
      expect(result).toBe(res);
    });

    it('appendHeader delegates to BunResponse', () => {
      const res = new BunResponse();
      res.setHeader('X-Multi', 'a');
      const result = adapter.appendHeader(res, 'X-Multi', 'b');

      expect(res.getHeader('x-multi')).toBe('a, b');
      expect(result).toBe(res);
    });

    it('getHeader delegates to BunResponse', () => {
      const res = new BunResponse();
      res.setHeader('X-Custom', 'test');
      const value = adapter.getHeader(res, 'X-Custom');

      expect(value).toBe('test');
    });
  });

  describe('isHeadersSent()', () => {
    it('returns false before response is ended', () => {
      const res = new BunResponse();
      expect(adapter.isHeadersSent(res)).toBe(false);
    });

    it('returns true after response is ended', () => {
      const res = new BunResponse();
      res.end();
      expect(adapter.isHeadersSent(res)).toBe(true);
    });
  });

  describe('getRequestHostname()', () => {
    it('returns hostname from BunRequest', () => {
      const req = new BunRequest(new Request('http://example.com:3000/test', {
        headers: { host: 'example.com:3000' },
      }));
      expect(adapter.getRequestHostname(req)).toBe('example.com');
    });
  });

  describe('getRequestMethod()', () => {
    it('returns method from BunRequest', () => {
      const req = new BunRequest(new Request('http://localhost:3000/', { method: 'POST' }));
      expect(adapter.getRequestMethod(req)).toBe('POST');
    });
  });

  describe('getRequestUrl()', () => {
    it('returns originalUrl from BunRequest', () => {
      const req = new BunRequest(new Request('http://localhost:3000/users?page=1'));
      expect(adapter.getRequestUrl(req)).toBe('/users?page=1');
    });

    it('returns url without query string', () => {
      const req = new BunRequest(new Request('http://localhost:3000/health'));
      expect(adapter.getRequestUrl(req)).toBe('/health');
    });
  });

  describe('unsupported methods', () => {
    it('useStaticAssets() throws an error', () => {
      expect(() => adapter.useStaticAssets()).toThrow(
        'Static assets are not supported by BunAdapter',
      );
    });

    it('setViewEngine() throws an error', () => {
      expect(() => adapter.setViewEngine('ejs')).toThrow(
        'View engine is not supported by BunAdapter',
      );
    });

    it('render() throws an error', () => {
      const res = new BunResponse();
      expect(() => adapter.render(res, 'index', {})).toThrow(
        'Render is not supported by BunAdapter',
      );
    });
  });

  describe('initHttpServer()', () => {
    it('stores TLS options', () => {
      const tlsOpts = { key: 'keydata', cert: 'certdata' };
      adapter.initHttpServer({ httpsOptions: tlsOpts } as any);
      expect((adapter as any)._tlsOptions).toBe(tlsOpts);
    });

    it('does not set TLS options when not provided', () => {
      adapter.initHttpServer({} as any);
      expect((adapter as any)._tlsOptions).toBeUndefined();
    });
  });

  describe('end()', () => {
    it('delegates to BunResponse.end()', () => {
      const res = new BunResponse();
      adapter.end(res, 'done');
      expect(res._ended).toBe(true);
      expect(res._body).toBe('done');
    });

    it('works without a message', () => {
      const res = new BunResponse();
      adapter.end(res);
      expect(res._ended).toBe(true);
    });
  });

  describe('redirect()', () => {
    it('delegates to BunResponse.redirect() with status and url', () => {
      const res = new BunResponse();
      adapter.redirect(res, 301, '/new');
      expect(res.statusCode).toBe(301);
      expect(res.getHeader('location')).toBe('/new');
      expect(res._ended).toBe(true);
    });
  });

  describe('applyVersionFilter()', () => {
    it('returns the handler as pass-through', () => {
      const handler = vi.fn();
      const result = adapter.applyVersionFilter(handler, '1', {} as any);
      expect(result).toBe(handler);
    });
  });

  describe('getHttpServer()', () => {
    it('returns the httpServer stub', () => {
      const server = adapter.getHttpServer();
      expect(server).toBeDefined();
      expect(typeof server.on).toBe('function');
      expect(typeof server.address).toBe('function');
    });
  });

  describe('registerParserMiddleware()', () => {
    it('unshifts a body parser middleware', () => {
      const mws = (adapter as any)._middlewares;
      expect(mws).toHaveLength(0);

      adapter.registerParserMiddleware();

      expect(mws).toHaveLength(1);
      expect(mws[0].path).toBeNull();
      expect(typeof mws[0].handler).toBe('function');
    });
  });

  describe('enableCors()', () => {
    it('registers a CORS middleware', () => {
      const mws = (adapter as any)._middlewares;
      expect(mws).toHaveLength(0);

      adapter.enableCors();

      expect(mws).toHaveLength(1);
      expect(mws[0].path).toBeNull();
    });
  });

  describe('createMiddlewareFactory()', () => {
    it('returns a function that registers path-scoped middleware', () => {
      const factory = adapter.createMiddlewareFactory(0 /* GET */);
      const callback = vi.fn();
      factory('/api', callback);

      const mws = (adapter as any)._middlewares;
      expect(mws).toHaveLength(1);
      expect(mws[0].path).toBe('/api');
      expect(mws[0].handler).toBe(callback);
    });
  });
});
