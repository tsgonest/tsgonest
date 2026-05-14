import type { Constructor } from './container'
import { Event } from './event'
import { Router, type RouteMatch, type RawRoute } from './router'
import type { Module } from './module'
import type { Token } from './token'
import type { Middleware, RouteHandler } from './types'
import { composeChain } from './middleware-chain'
import { ErrorMapper, type ErrorMapHandler, type ErrorPredicate } from './error-mapper'
import { bootGraph, disposeProviders, type Context } from './context'

export type { Context, ContextOptions } from './context'
export { createContext } from './context'

export interface AppOptions {
  imports: Module[]
}

export interface App extends Context {
  router: Router
  fetch(request: Request): Promise<Response>
  /** Register global or prefix-mounted middleware. */
  use(middleware: Middleware): void
  use(prefix: string, middleware: Middleware): void
  /**
   * Register a custom error mapping. User-registered handlers run before the
   * built-in defaults; first match wins.
   */
  onError(predicate: ErrorPredicate, handler: ErrorMapHandler): void
  /**
   * Adapter hook: construct the per-request {@link Event} that `fetch` would
   * otherwise build internally. Adapters that need to seed token-store values
   * (e.g. `BUN_SERVER`) before the handler runs pass a `setup` callback.
   */
  createEvent(request: Request, setup?: (event: Event) => void): Event
}

interface GlobalMiddleware {
  prefix?: string
  middleware: Middleware
}

export async function createApp(options: AppOptions): Promise<App> {
  const boot = await bootGraph(options.imports)
  const router = new Router()
  const errorMapper = new ErrorMapper()
  const globalMiddleware: GlobalMiddleware[] = []

  // Map every controller → the module it was declared in. Module-level
  // middleware only applies to its own module's controllers, not imports.
  const moduleByController = new Map<Constructor, Module>()
  for (const mod of boot.modules) {
    for (const Ctrl of mod.controllers ?? []) {
      moduleByController.set(Ctrl, mod)
    }
  }

  router.setFinalizer((raw: readonly RawRoute[]) => {
    const out = new Map<string, RouteMatch>()
    for (const r of raw) {
      const chain = buildChainFor(r)
      out.set(`${r.method} ${r.path}`, {
        handler: chain,
        params: {},
        controller: r.controller,
      })
    }
    return out
  })

  let closed = false
  let disposed = false
  const inflight = new Set<Promise<unknown>>()

  function buildChainFor(route: RawRoute): RouteHandler {
    const moduleMw: Middleware[] = []
    if (route.controller) {
      const owningModule = moduleByController.get(route.controller)
      if (owningModule?.middleware) moduleMw.push(...owningModule.middleware)
    }

    // The full pipeline: global (with prefix-filter wrappers) → module → handler.
    // Prefix middleware are conditionalised inline so a non-matching path falls
    // through without invoking the user's middleware.
    const ordered: Middleware[] = []
    for (const g of globalMiddleware) {
      if (g.prefix === undefined) {
        ordered.push(g.middleware)
      } else {
        const prefix = g.prefix
        const mw = g.middleware
        ordered.push(async (event, next) => {
          const url = new URL(event.request.url)
          if (matchesPrefix(url.pathname, prefix)) {
            return mw(event, next)
          }
          return next()
        })
      }
    }
    for (const m of moduleMw) ordered.push(m)
    return composeChain(ordered, route.handler)
  }

  const app: App = {
    router,
    createEvent(request: Request, setup?: (event: Event) => void): Event {
      const event = new Event(request, boot.container)
      if (setup) setup(event)
      return event
    },
    use(prefixOrMw: string | Middleware, maybeMw?: Middleware): void {
      if (typeof prefixOrMw === 'string') {
        if (!maybeMw) throw new Error('app.use(prefix, middleware) requires a middleware')
        globalMiddleware.push({ prefix: prefixOrMw, middleware: maybeMw })
      } else {
        globalMiddleware.push({ middleware: prefixOrMw })
      }
    },
    onError(predicate: ErrorPredicate, handler: ErrorMapHandler): void {
      errorMapper.register(predicate, handler)
    },
    async fetch(request: Request): Promise<Response> {
      if (closed) {
        return new Response('Service Unavailable', {
          status: 503,
          headers: { 'content-type': 'text/plain' },
        })
      }
      const url = new URL(request.url)
      const match = router.match(request.method, url.pathname)
      if (!match) {
        return new Response('Not Found', {
          status: 404,
          headers: { 'content-type': 'text/plain' },
        })
      }
      const event = app.createEvent(request)
      event.params = match.params
      const work = (async (): Promise<Response> => {
        try {
          const result = await match.handler(event)
          if (!(result instanceof Response)) {
            return new Response('Not Implemented', { status: 501 })
          }
          return result
        } catch (err) {
          return errorMapper.map(err, event)
        }
      })()
      inflight.add(work)
      try {
        return await work
      } finally {
        inflight.delete(work)
      }
    },
    resolve(provider: Token<unknown> | Constructor<unknown>) {
      return boot.container.resolve(provider) as never
    },
    async [Symbol.asyncDispose]() {
      if (disposed) return
      disposed = true
      closed = true
      if (inflight.size > 0) await Promise.allSettled([...inflight])
      await disposeProviders(boot)
    },
  }

  return app
}

function matchesPrefix(pathname: string, prefix: string): boolean {
  if (!prefix.startsWith('/')) prefix = '/' + prefix
  if (pathname === prefix) return true
  // Treat `/api` as matching `/api/...` but not `/apiary`.
  return pathname.startsWith(prefix.endsWith('/') ? prefix : prefix + '/')
}
