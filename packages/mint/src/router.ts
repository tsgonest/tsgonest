import type { RouteHandler } from './types'

export interface RouteMatch {
  handler: RouteHandler
  params: Record<string, string>
}

/**
 * Exact-path router keyed by `${method} ${path}`. Method is free-form so
 * non-HTTP verbs like 'WS' work without a constrained union.
 */
export class Router {
  private readonly routes = new Map<string, RouteHandler>()

  add(method: string, path: string, handler: RouteHandler): void {
    this.routes.set(key(method, path), handler)
  }

  match(method: string, pathname: string): RouteMatch | undefined {
    const handler = this.routes.get(key(method, pathname))
    if (!handler) return undefined
    return { handler, params: {} }
  }
}

function key(method: string, path: string): string {
  return `${method} ${path}`
}
