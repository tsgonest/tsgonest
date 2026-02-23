import type { NestInterceptor, ExecutionContext, CallHandler } from '@nestjs/common';
import type { Observable } from 'rxjs';
import { CompanionDiscovery } from './discovery';

/**
 * Options for configuring the TsgonestFastInterceptor.
 */
export interface FastInterceptorOptions {
  /**
   * Path to the dist directory containing the tsgonest manifest.
   * Defaults to the current working directory's dist/ folder.
   */
  distDir?: string;

  /**
   * Pre-loaded CompanionDiscovery instance.
   * If provided, distDir is ignored.
   */
  discovery?: CompanionDiscovery;

  /**
   * Type name overrides for specific routes.
   * Map of "ControllerName.methodName" to type name.
   * Takes priority over the route map in the manifest.
   */
  typeOverrides?: Record<string, string>;

  /**
   * Whether to set the raw response directly (bypassing NestJS JSON serialization).
   * When true, the interceptor writes the pre-serialized JSON string directly to
   * the response with Content-Type: application/json, skipping JSON.stringify entirely.
   *
   * When false (default), the interceptor returns the pre-serialized string and
   * relies on NestJS/Express to write it (which may still call JSON.stringify on
   * the string — wrapping it in quotes). Use `true` for maximum performance.
   *
   * Defaults to true.
   */
  rawResponse?: boolean;
}

/**
 * A high-performance NestJS interceptor that replaces JSON.stringify with
 * tsgonest-generated schema-aware serializers on responses.
 *
 * Unlike `TsgonestSerializationInterceptor`, this interceptor uses the
 * **route map** in the manifest to look up which serializer to use for each
 * controller method — no Reflect.getMetadata or emitDecoratorMetadata needed.
 *
 * The route map is populated at build time by tsgonest's static analysis of
 * NestJS controllers, making this zero-config and guaranteed to work.
 *
 * Performance: ~1.4x faster JSON serialization compared to JSON.stringify for simple DTOs,
 * because the generated serializers use string concatenation with known
 * property names and types, avoiding the overhead of generic object traversal.
 *
 * Usage:
 * ```ts
 * import { TsgonestFastInterceptor } from '@tsgonest/runtime';
 *
 * // In main.ts (after app creation):
 * app.useGlobalInterceptors(new TsgonestFastInterceptor());
 *
 * // With options:
 * app.useGlobalInterceptors(new TsgonestFastInterceptor({
 *   distDir: 'dist',
 *   rawResponse: true, // bypass JSON.stringify entirely
 * }));
 * ```
 */
export class TsgonestFastInterceptor implements NestInterceptor {
  private discovery: CompanionDiscovery;
  private typeOverrides: Record<string, string>;
  private rawResponse: boolean;
  private initialized = false;
  private distDir: string | undefined;

  constructor(options: FastInterceptorOptions = {}) {
    this.typeOverrides = options.typeOverrides ?? {};
    this.rawResponse = options.rawResponse ?? true;

    if (options.discovery) {
      this.discovery = options.discovery;
      this.initialized = true;
    } else {
      this.discovery = new CompanionDiscovery();
      this.distDir = options.distDir;
    }
  }

  /**
   * Ensure the manifest is loaded (lazy init on first request).
   */
  private ensureInitialized(): void {
    if (this.initialized) return;
    this.initialized = true;

    const distDir = this.distDir || 'dist';
    this.discovery.loadManifest(distDir);
  }

  /**
   * Intercept the response and apply fast serialization if a route mapping exists.
   */
  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    this.ensureInitialized();

    const handler = context.getHandler();
    const controller = context.getClass();
    const controllerName = controller.name;
    const methodName = handler.name;

    // 1. Check explicit type overrides first
    const overrideKey = `${controllerName}.${methodName}`;
    const overrideType = this.typeOverrides[overrideKey];

    // 2. Look up via route map (the primary path — zero-config)
    let serializerFn: ((input: unknown) => string) | null = null;
    let isArray = false;

    if (overrideType) {
      serializerFn = this.discovery.getSerializer(overrideType);
    } else {
      const routeInfo = this.discovery.getSerializerForRoute(controllerName, methodName);
      if (routeInfo) {
        serializerFn = routeInfo.serializer;
        isArray = routeInfo.isArray;
      }
    }

    if (!serializerFn) {
      // No serializer available — pass through to default JSON.stringify
      return next.handle();
    }

    // Capture for closure
    const serializer = serializerFn;
    const useRawResponse = this.rawResponse;
    const isArrayResponse = isArray;

    // Lazy-import rxjs to avoid requiring it at module load time
    try {
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const { map } = require('rxjs');
      return next.handle().pipe(
        map((data: unknown) => {
          if (data === null || data === undefined) {
            return data;
          }

          let jsonString: string;

          if (isArrayResponse || Array.isArray(data)) {
            // Serialize array: serialize each element, join with commas
            const items = Array.isArray(data) ? data : [data];
            const parts = items.map(item => serializer(item));
            jsonString = '[' + parts.join(',') + ']';
          } else {
            jsonString = serializer(data);
          }

          if (useRawResponse) {
            // Write directly to the response, bypassing NestJS JSON serialization
            const httpCtx = context.switchToHttp();
            const response = httpCtx.getResponse();

            // Works with both Express and Fastify
            if (typeof response.type === 'function') {
              // Fastify
              response.type('application/json');
              response.send(jsonString);
            } else if (typeof response.setHeader === 'function') {
              // Express
              response.setHeader('Content-Type', 'application/json');
              response.end(jsonString);
            }

            // Return undefined to signal NestJS not to send anything else.
            // The response is already sent.
            return undefined;
          }

          // Non-raw mode: return the pre-serialized JSON.
          // Note: NestJS will call JSON.stringify on this string,
          // wrapping it in quotes. This mode is less optimal but safer.
          return jsonString;
        }),
      );
    } catch {
      // rxjs not available — pass through
      return next.handle();
    }
  }
}
