import type { NestInterceptor, ExecutionContext, CallHandler } from '@nestjs/common';
import type { Observable } from 'rxjs';
import { CompanionDiscovery } from './discovery';

/**
 * Options for configuring the TsgonestSerializationInterceptor.
 */
export interface SerializationInterceptorOptions {
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
   * Map of "ControllerName.methodName" → type name.
   */
  typeOverrides?: Record<string, string>;
}

/**
 * A NestJS interceptor that uses tsgonest-generated companion serialization
 * functions for fast JSON output.
 *
 * When a serializer companion exists for a controller method's return type,
 * this interceptor replaces the default JSON.stringify with the generated
 * fast serializer (string concatenation approach).
 *
 * Usage:
 * ```ts
 * import { TsgonestSerializationInterceptor } from '@tsgonest/runtime';
 *
 * // In main.ts:
 * app.useGlobalInterceptors(new TsgonestSerializationInterceptor({ distDir: 'dist' }));
 * ```
 */
export class TsgonestSerializationInterceptor implements NestInterceptor {
  private discovery: CompanionDiscovery;
  private typeOverrides: Record<string, string>;
  private initialized = false;
  private distDir: string | undefined;

  constructor(options: SerializationInterceptorOptions = {}) {
    this.typeOverrides = options.typeOverrides ?? {};

    if (options.discovery) {
      this.discovery = options.discovery;
      this.initialized = true;
    } else {
      this.discovery = new CompanionDiscovery();
      this.distDir = options.distDir;
    }
  }

  /**
   * Ensure the manifest is loaded.
   */
  private ensureInitialized(): void {
    if (this.initialized) return;
    this.initialized = true;

    const distDir = this.distDir || 'dist';
    this.discovery.loadManifest(distDir);
  }

  /**
   * Intercept the response and apply fast serialization if available.
   */
  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    this.ensureInitialized();

    const handler = context.getHandler();
    const controller = context.getClass();

    // Determine the return type name
    const typeName = this.resolveTypeName(controller, handler);

    if (!typeName) {
      return next.handle();
    }

    // Look up the serializer for this type
    const serializer = this.discovery.getSerializer(typeName);
    if (!serializer) {
      return next.handle();
    }

    // Lazy-import rxjs map to avoid requiring rxjs at import time
    try {
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const { map } = require('rxjs');
      return next.handle().pipe(
        map((data: unknown) => {
          if (data === null || data === undefined) {
            return data;
          }

          // If the data is an array, serialize each element
          if (Array.isArray(data)) {
            const parts = data.map(item => serializer(item));
            return '[' + parts.join(',') + ']';
          }

          return serializer(data);
        }),
      );
    } catch {
      // rxjs not available — pass through
      return next.handle();
    }
  }

  /**
   * Resolve the return type name for a controller method.
   *
   * Strategy:
   * 1. Check typeOverrides for "ControllerName.methodName"
   * 2. Check Reflect metadata (if available) for return type
   * 3. Check the manifest for all registered serializer types
   *    and match by method name convention (e.g., "findAll" → look for array type)
   */
  private resolveTypeName(
    controller: Function,
    handler: Function,
  ): string | null {
    const controllerName = controller.name;
    const methodName = handler.name;

    // 1. Check explicit overrides
    const overrideKey = `${controllerName}.${methodName}`;
    if (this.typeOverrides[overrideKey]) {
      return this.typeOverrides[overrideKey];
    }

    // 2. Check Reflect metadata for return type (if emitDecoratorMetadata is enabled)
    if (typeof Reflect !== 'undefined' && Reflect.getMetadata) {
      const returnType = Reflect.getMetadata('design:returntype', controller.prototype, methodName);
      if (returnType && returnType.name && returnType.name !== 'Promise' && returnType.name !== 'Object') {
        const typeName = returnType.name;
        if (this.discovery.hasSerializer(typeName)) {
          return typeName;
        }
      }
    }

    // 3. Check custom metadata set by tsgonest (via __tsgonest_return_type__)
    if (typeof Reflect !== 'undefined' && Reflect.getMetadata) {
      const tsgonestReturnType = Reflect.getMetadata('__tsgonest_return_type__', controller.prototype, methodName);
      if (tsgonestReturnType && this.discovery.hasSerializer(tsgonestReturnType)) {
        return tsgonestReturnType;
      }
    }

    return null;
  }
}
