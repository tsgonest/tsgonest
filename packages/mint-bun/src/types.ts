/**
 * Adapter-internal type alias for Bun's `Server`. We use the global `Bun`
 * namespace (registered by `bun-types`/`@types/bun`) rather than
 * `import('bun').Server` because rolldown's d.ts bundler does not resolve
 * `declare module "bun"` exports and emits `undefined` for the unresolved
 * type — which would erase consumer type safety.
 *
 * When `bun-types` is not installed, `Bun` falls back to `any`; the token still
 * works at runtime, just without static checking on the consumer side.
 */
type AnyBunServer = Bun.Server<unknown> | Bun.Server<any>

export type BunServer = AnyBunServer
