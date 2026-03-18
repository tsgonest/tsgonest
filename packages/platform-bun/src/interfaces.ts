import type { TlsOptions } from 'node:tls';

/** Options for BunAdapter.listen(). */
export interface BunAdapterOptions {
  /** TLS configuration for HTTPS. */
  tls?: TlsOptions;
}

/** Stored route handler. */
export interface RouteHandler {
  method: string;
  path: string;
  handler: (...args: any[]) => any;
}

/** Stored middleware entry. */
export interface MiddlewareEntry {
  /** null = global middleware, string = path-scoped */
  path: string | null;
  handler: (req: any, res: any, next: () => void) => any;
}
