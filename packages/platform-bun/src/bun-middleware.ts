import type { MiddlewareEntry } from './interfaces';

/**
 * Executes a middleware chain sequentially, calling next() to advance.
 * Matches Express middleware convention: (req, res, next) => void.
 *
 * Performance: avoids wrapping in Promise.resolve() when the entire
 * chain is synchronous.
 */
export function executeMiddlewareChain(
  middlewares: MiddlewareEntry[],
  path: string,
  req: any,
  res: any,
  finalHandler: () => Promise<void> | void,
): Promise<void> | void {
  let index = 0;

  function next(): Promise<void> | void {
    while (index < middlewares.length) {
      const mw = middlewares[index++];

      // Skip path-scoped middleware that doesn't match
      if (mw.path !== null && !pathMatches(path, mw.path)) {
        continue;
      }

      // Call middleware with Express-style (req, res, next) signature
      try {
        const result = mw.handler(req, res, next);
        // If middleware returned a promise, propagate it
        if (result && typeof result.then === 'function') {
          return result;
        }
        // If middleware called next() synchronously, we already advanced
        // via the recursive call inside next. If it didn't call next(),
        // the response was likely ended — either way, return.
        return;
      } catch (err) {
        if (!res.headersSent) {
          res.status(500).json({ statusCode: 500, message: 'Internal Server Error' });
        }
        return;
      }
    }

    return finalHandler();
  }

  return next();
}

/** Check if a request path matches a middleware path prefix. */
export function pathMatches(requestPath: string, middlewarePath: string): boolean {
  if (middlewarePath === '/') return true;
  const normalized = middlewarePath.endsWith('/')
    ? middlewarePath.slice(0, -1)
    : middlewarePath;
  return requestPath === normalized || requestPath.startsWith(normalized + '/');
}
