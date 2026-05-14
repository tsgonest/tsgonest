import type { Event } from './event'
import type { Middleware, RouteHandler, Upgrade } from './types'

/**
 * Compose an onion of middleware around a route handler. The returned function
 * runs each middleware outer-to-inner; each middleware decides whether to call
 * `await next()` (continue) or short-circuit by returning a `Response` without
 * advancing.
 */
export function composeChain(
  middleware: readonly Middleware[],
  handler: RouteHandler,
): RouteHandler {
  if (middleware.length === 0) return handler

  return async (event: Event): Promise<Response | Upgrade> => {
    let i = -1
    const dispatch = async (idx: number): Promise<Response | Upgrade> => {
      if (idx <= i) {
        throw new Error('next() called multiple times in the same middleware')
      }
      i = idx
      if (idx === middleware.length) return handler(event)
      const mw = middleware[idx]
      return mw(event, () => dispatch(idx + 1))
    }
    return dispatch(0)
  }
}
