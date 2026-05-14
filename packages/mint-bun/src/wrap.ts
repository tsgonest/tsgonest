import type { App, Upgrade } from '@mintkit/core'
import { BUN_SERVER } from './token'
import type { BunServer } from './types'

export interface BunHandler {
  fetch(request: Request, server: BunServer): Promise<Response>
}

/**
 * Wraps a Mint app so Bun's `Server` reference is exposed to every handler via
 * the `BUN_SERVER` token. Use the returned `{ fetch }` directly with
 * `Bun.serve` when you need platform features:
 *
 * ```ts
 * Bun.serve({ fetch: wrap(app).fetch, port: 3000 })
 * ```
 *
 * Zero-config users who never need the `Server` reference can keep passing
 * `app.fetch` to `Bun.serve` — this wrapper is opt-in.
 */
export function wrap(app: App): BunHandler {
  return {
    async fetch(request: Request, server: BunServer): Promise<Response> {
      // Mirror app.fetch's dispatch loop, but build the event with a setup
      // callback that seeds BUN_SERVER. We can't reuse app.fetch directly
      // because it has no per-call hook for the token store.
      const url = new URL(request.url)
      const match = app.router.match(request.method, url.pathname)
      if (!match) {
        return new Response('Not Found', {
          status: 404,
          headers: { 'content-type': 'text/plain' },
        })
      }
      const event = app.createEvent(request, (ev) => {
        ev.set(BUN_SERVER, server)
      })
      const result: Response | Upgrade = await match.handler(event)
      if (!(result instanceof Response)) {
        // `Upgrade` is reserved for a future WS path and not produced in v1.
        return new Response('Not Implemented', { status: 501 })
      }
      return result
    },
  }
}
