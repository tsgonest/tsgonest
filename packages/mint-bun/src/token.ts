import { defineToken } from '@mintkit/core'
import type { BunServer } from './types'

/**
 * Typed reference to Bun's `Server` instance. `wrap(app).fetch(req, server)`
 * sets this on every event so handlers and middleware can read it via
 * `event.require(BUN_SERVER)` — useful for socket inspection or (future) WS
 * upgrades.
 */
export const BUN_SERVER = defineToken<BunServer>('BUN_SERVER')
