import type { Event } from './event'

/**
 * Reserved marker for a future WebSocket-upgrade response. Never produced in
 * Phase 1, but part of the handler return type so the surface stays stable.
 */
export interface Upgrade {
  readonly _tag: 'upgrade'
}

export type RouteHandler = (event: Event) => Promise<Response | Upgrade>

export type Middleware = (
  event: Event,
  next: () => Promise<Response | Upgrade>,
) => Promise<Response | Upgrade>
