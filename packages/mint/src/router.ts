import type { Constructor } from './container'
import type { RouteHandler } from './types'

export interface RouteMatch {
  handler: RouteHandler
  params: Record<string, string>
  /**
   * The controller class this route belongs to, if `addForController` was used.
   * The middleware finalizer reads this to pick module-level bindings.
   */
  controller?: Constructor
}

interface RawRoute {
  method: string
  path: string
  handler: RouteHandler
  controller?: Constructor
  /** Pre-parsed segments for `:name` extraction. */
  segments: RouteSegment[]
  hasParams: boolean
}

interface RouteSegment {
  /** Literal text or param name. */
  value: string
  /** When true, this segment captures the URL segment under `value`. */
  isParam: boolean
}

/**
 * Phase 2 router. Supports static segments and `:name` path params.
 *
 * Matching strategy is split-and-compare: each registered route is broken into
 * segments at boot, and `match()` splits the incoming pathname the same way.
 * Static segments must match exactly; `:name` segments capture into `params`.
 *
 * For static-only routes (no `:name` segments) we keep a direct lookup map for
 * O(1) match. Param routes go through a linear scan; that's fine for the
 * route counts a single service produces.
 *
 * Routes are buffered as raw entries when added. The first `match()` call
 * triggers `finalize` (when wired up by `App`), which wraps each raw handler
 * with the full middleware chain. After finalization the router serves
 * pre-wrapped handlers.
 */
export class Router {
  private readonly raw: RawRoute[] = []
  /** Finalized static routes, keyed by `${method} ${path}`. */
  private readonly finalizedStatic = new Map<string, RouteMatch>()
  /**
   * Finalized parameterised routes, indexed by method. Linear scan within a
   * method bucket — fast enough for typical route counts.
   */
  private readonly finalizedParam = new Map<string, FinalizedParamRoute[]>()
  private finalizer?: (raw: readonly RawRoute[]) => Map<string, RouteMatch>
  private isFinalized = false

  add(method: string, path: string, handler: RouteHandler): void {
    const { segments, hasParams } = parsePath(path)
    this.raw.push({ method, path, handler, segments, hasParams })
  }

  /**
   * Records a route together with the controller class it belongs to. Used by
   * tsgonest-generated `registerXxxController(app)` companions so module-level
   * middleware can be scoped correctly.
   */
  addForController(
    controller: Constructor,
    method: string,
    path: string,
    handler: RouteHandler,
  ): void {
    const { segments, hasParams } = parsePath(path)
    this.raw.push({ method, path, handler, controller, segments, hasParams })
  }

  /**
   * Lazy hook used by `App` to install a function that wraps each raw route's
   * handler with the assembled middleware chain. The finalizer runs once, on
   * the next `match()` call.
   */
  setFinalizer(fn: (raw: readonly RawRoute[]) => Map<string, RouteMatch>): void {
    this.finalizer = fn
  }

  match(method: string, pathname: string): RouteMatch | undefined {
    if (!this.isFinalized) this.finalize()

    const staticHit = this.finalizedStatic.get(key(method, pathname))
    if (staticHit) return staticHit

    const bucket = this.finalizedParam.get(method)
    if (!bucket || bucket.length === 0) return undefined

    const pathSegments = splitPath(pathname)
    for (const route of bucket) {
      const params = matchSegments(route.segments, pathSegments)
      if (!params) continue
      return {
        handler: route.match.handler,
        params,
        controller: route.match.controller,
      }
    }
    return undefined
  }

  private finalize(): void {
    if (this.isFinalized) return
    this.isFinalized = true

    let wrapped: Map<string, RouteMatch>
    if (this.finalizer) {
      wrapped = this.finalizer(this.raw)
    } else {
      wrapped = new Map()
      for (const r of this.raw) {
        wrapped.set(key(r.method, r.path), {
          handler: r.handler,
          params: {},
          controller: r.controller,
        })
      }
    }

    for (const r of this.raw) {
      const finalized = wrapped.get(key(r.method, r.path))
      if (!finalized) continue
      if (!r.hasParams) {
        this.finalizedStatic.set(key(r.method, r.path), finalized)
        continue
      }
      const bucket = this.finalizedParam.get(r.method) ?? []
      bucket.push({ segments: r.segments, match: finalized })
      this.finalizedParam.set(r.method, bucket)
    }
  }
}

interface FinalizedParamRoute {
  segments: RouteSegment[]
  match: RouteMatch
}

function key(method: string, path: string): string {
  return `${method} ${path}`
}

function parsePath(path: string): { segments: RouteSegment[]; hasParams: boolean } {
  const raw = splitPath(path)
  let hasParams = false
  const segments: RouteSegment[] = raw.map((seg) => {
    if (seg.startsWith(':')) {
      hasParams = true
      return { value: seg.slice(1), isParam: true }
    }
    return { value: seg, isParam: false }
  })
  return { segments, hasParams }
}

function splitPath(path: string): string[] {
  // Treat '/' as a single empty-segment route by returning a sentinel ''.
  if (path === '/' || path === '') return ['']
  // Strip leading '/' so '/a/b' and 'a/b' both split into ['a','b'].
  const trimmed = path.startsWith('/') ? path.slice(1) : path
  // Strip trailing '/' so '/a/b/' matches '/a/b'.
  const noTail = trimmed.endsWith('/') ? trimmed.slice(0, -1) : trimmed
  if (noTail === '') return ['']
  return noTail.split('/')
}

function matchSegments(
  template: RouteSegment[],
  pathSegments: string[],
): Record<string, string> | undefined {
  if (template.length !== pathSegments.length) return undefined
  const params: Record<string, string> = {}
  for (let i = 0; i < template.length; i++) {
    const t = template[i]
    const p = pathSegments[i]
    if (t.isParam) {
      // Empty path segment must not match a :param — '/users/' shouldn't bind
      // 'id' to ''. Empty-string params are basically always a bug.
      if (p === '') return undefined
      params[t.value] = decodeURIComponent(p)
      continue
    }
    if (t.value !== p) return undefined
  }
  return params
}

export type { RawRoute }
