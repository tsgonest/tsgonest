import { describe, it, expect, vi } from 'vitest';
import { executeMiddlewareChain, pathMatches } from '../bun-middleware';
import type { MiddlewareEntry } from '../interfaces';

describe('executeMiddlewareChain', () => {
  // Helper to create a minimal req/res pair
  function createReqRes(path = '/test') {
    return {
      req: { url: path } as any,
      res: {
        _ended: false,
        headersSent: false,
        status(code: number) {
          (this as any).statusCode = code;
          return this;
        },
        json(body: any) {
          (this as any)._body = body;
          (this as any)._ended = true;
          (this as any).headersSent = true;
        },
      } as any,
    };
  }

  it('calls finalHandler directly when middleware array is empty', async () => {
    const { req, res } = createReqRes();
    const finalHandler = vi.fn();

    await executeMiddlewareChain([], '/test', req, res, finalHandler);
    expect(finalHandler).toHaveBeenCalledOnce();
  });

  it('calls a single sync middleware, then finalHandler', async () => {
    const { req, res } = createReqRes();
    const handler = vi.fn((_req: any, _res: any, next: () => void) => {
      next();
    });
    const finalHandler = vi.fn();

    const middlewares: MiddlewareEntry[] = [
      { path: null, handler },
    ];

    await executeMiddlewareChain(middlewares, '/test', req, res, finalHandler);
    expect(handler).toHaveBeenCalledOnce();
    expect(finalHandler).toHaveBeenCalledOnce();
  });

  it('middleware calling next() advances the chain', async () => {
    const { req, res } = createReqRes();
    const order: number[] = [];

    const mw1: MiddlewareEntry = {
      path: null,
      handler: (_req, _res, next) => { order.push(1); next(); },
    };
    const mw2: MiddlewareEntry = {
      path: null,
      handler: (_req, _res, next) => { order.push(2); next(); },
    };
    const finalHandler = vi.fn(() => { order.push(3); });

    await executeMiddlewareChain([mw1, mw2], '/test', req, res, finalHandler);
    expect(order).toEqual([1, 2, 3]);
  });

  it('middleware NOT calling next() stops the chain', async () => {
    const { req, res } = createReqRes();
    const finalHandler = vi.fn();

    const mw: MiddlewareEntry = {
      path: null,
      handler: (_req, _res, _next) => {
        // Intentionally NOT calling next() — response ended by middleware
        _res._ended = true;
      },
    };

    await executeMiddlewareChain([mw], '/test', req, res, finalHandler);
    expect(finalHandler).not.toHaveBeenCalled();
    expect(res._ended).toBe(true);
  });

  it('path-scoped middleware only executes when path matches', async () => {
    const { req, res } = createReqRes('/api/users');
    const handler = vi.fn((_req: any, _res: any, next: () => void) => {
      next();
    });
    const finalHandler = vi.fn();

    const middlewares: MiddlewareEntry[] = [
      { path: '/api', handler },
    ];

    await executeMiddlewareChain(middlewares, '/api/users', req, res, finalHandler);
    expect(handler).toHaveBeenCalledOnce();
    expect(finalHandler).toHaveBeenCalledOnce();
  });

  it('path-scoped middleware is skipped when path does not match', async () => {
    const { req, res } = createReqRes('/other');
    const handler = vi.fn((_req: any, _res: any, next: () => void) => {
      next();
    });
    const finalHandler = vi.fn();

    const middlewares: MiddlewareEntry[] = [
      { path: '/api', handler },
    ];

    await executeMiddlewareChain(middlewares, '/other', req, res, finalHandler);
    expect(handler).not.toHaveBeenCalled();
    expect(finalHandler).toHaveBeenCalledOnce();
  });

  it('multiple middleware execute in order', async () => {
    const { req, res } = createReqRes();
    const order: string[] = [];

    const middlewares: MiddlewareEntry[] = [
      { path: null, handler: (_req, _res, next) => { order.push('a'); next(); } },
      { path: null, handler: (_req, _res, next) => { order.push('b'); next(); } },
      { path: null, handler: (_req, _res, next) => { order.push('c'); next(); } },
    ];
    const finalHandler = vi.fn(() => { order.push('final'); });

    await executeMiddlewareChain(middlewares, '/test', req, res, finalHandler);
    expect(order).toEqual(['a', 'b', 'c', 'final']);
  });

  it('async middleware is properly awaited', async () => {
    const { req, res } = createReqRes();
    const order: string[] = [];

    const asyncMw: MiddlewareEntry = {
      path: null,
      handler: async (_req, _res, next) => {
        order.push('async-start');
        await new Promise<void>((resolve) => setTimeout(resolve, 10));
        order.push('async-end');
        next();
      },
    };

    const finalHandler = vi.fn(() => { order.push('final'); });
    await executeMiddlewareChain([asyncMw], '/test', req, res, finalHandler);
    expect(order).toEqual(['async-start', 'async-end', 'final']);
  });

  it('middleware error is caught and 500 response is set', async () => {
    const { req, res } = createReqRes();

    const mw: MiddlewareEntry = {
      path: null,
      handler: () => {
        throw new Error('middleware boom');
      },
    };
    const finalHandler = vi.fn();

    await executeMiddlewareChain([mw], '/test', req, res, finalHandler);
    expect(finalHandler).not.toHaveBeenCalled();
    expect(res.statusCode).toBe(500);
    expect(res._body).toEqual({ statusCode: 500, message: 'Internal Server Error' });
  });

  it('middleware error does not set 500 if headers already sent', async () => {
    const { req, res } = createReqRes();
    res.headersSent = true;

    const mw: MiddlewareEntry = {
      path: null,
      handler: () => {
        throw new Error('middleware boom');
      },
    };
    const finalHandler = vi.fn();

    await executeMiddlewareChain([mw], '/test', req, res, finalHandler);
    expect(finalHandler).not.toHaveBeenCalled();
    // Should NOT have set a new status since headers were already sent
    expect(res.statusCode).toBeUndefined();
  });

  it('mixes path-scoped and global middleware correctly', async () => {
    const { req, res } = createReqRes('/api/data');
    const order: string[] = [];

    const middlewares: MiddlewareEntry[] = [
      { path: null, handler: (_req, _res, next) => { order.push('global'); next(); } },
      { path: '/admin', handler: (_req, _res, next) => { order.push('admin'); next(); } },
      { path: '/api', handler: (_req, _res, next) => { order.push('api'); next(); } },
    ];
    const finalHandler = vi.fn(() => { order.push('final'); });

    await executeMiddlewareChain(middlewares, '/api/data', req, res, finalHandler);
    // /admin should be skipped, global and /api should run
    expect(order).toEqual(['global', 'api', 'final']);
  });
});

describe('pathMatches', () => {
  it('"/" matches everything', () => {
    expect(pathMatches('/', '/')).toBe(true);
    expect(pathMatches('/users', '/')).toBe(true);
    expect(pathMatches('/users/123', '/')).toBe(true);
    expect(pathMatches('/api/v2/data', '/')).toBe(true);
  });

  it('"/users" matches "/users" exactly', () => {
    expect(pathMatches('/users', '/users')).toBe(true);
  });

  it('"/users" matches "/users/123"', () => {
    expect(pathMatches('/users/123', '/users')).toBe(true);
  });

  it('"/users" does NOT match "/user"', () => {
    expect(pathMatches('/user', '/users')).toBe(false);
  });

  it('"/users" does NOT match "/usersettings"', () => {
    expect(pathMatches('/usersettings', '/users')).toBe(false);
  });

  it('handles trailing slash normalization — "/users/" treated as "/users"', () => {
    expect(pathMatches('/users', '/users/')).toBe(true);
    expect(pathMatches('/users/123', '/users/')).toBe(true);
  });

  it('"/api" matches "/api/v1/users"', () => {
    expect(pathMatches('/api/v1/users', '/api')).toBe(true);
  });

  it('"/api" does NOT match "/application"', () => {
    expect(pathMatches('/application', '/api')).toBe(false);
  });

  it('exact path match without subpath', () => {
    expect(pathMatches('/health', '/health')).toBe(true);
  });

  it('root path "/" does not wrongly match when used as middleware path', () => {
    // "/" as middleware path should match everything
    expect(pathMatches('/anything', '/')).toBe(true);
  });
});
