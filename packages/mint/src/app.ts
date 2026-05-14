import { Container, type Constructor } from './container'
import { Event } from './event'
import { Router } from './router'
import type { Module } from './module'
import type { Token } from './token'

export interface AppOptions {
  imports: Module[]
}

export interface Context {
  container: Container
  router: Router
}

export interface App extends AsyncDisposable {
  router: Router
  fetch(request: Request): Promise<Response>
  resolve<T>(provider: Token<T> | Constructor<T>): T
}

export async function createApp(options: AppOptions): Promise<App> {
  const container = new Container()
  const router = new Router()

  const modules = flattenModules(options.imports)
  for (const mod of modules) {
    for (const Ctrl of mod.controllers ?? []) {
      if (!container.has(Ctrl)) {
        container.register(Ctrl, () => new Ctrl())
      }
    }
  }

  const app: App = {
    router,
    async fetch(request: Request): Promise<Response> {
      const url = new URL(request.url)
      const match = router.match(request.method, url.pathname)
      if (!match) {
        return new Response('Not Found', {
          status: 404,
          headers: { 'content-type': 'text/plain' },
        })
      }
      const event = new Event(request, container)
      const result = await match.handler(event)
      if (!(result instanceof Response)) {
        // Upgrade is reserved and never produced in Phase 1.
        return new Response('Not Implemented', { status: 501 })
      }
      return result
    },
    resolve(provider) {
      return container.resolve(provider)
    },
    async [Symbol.asyncDispose]() {
      // Phase 1: no lifecycle hooks.
    },
  }

  return app
}

function flattenModules(roots: Module[]): Module[] {
  const seen = new Set<Module>()
  const out: Module[] = []
  const visit = (m: Module): void => {
    if (seen.has(m)) return
    seen.add(m)
    for (const child of m.imports ?? []) visit(child)
    out.push(m)
  }
  for (const m of roots) visit(m)
  return out
}
