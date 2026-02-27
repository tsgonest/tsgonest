import 'reflect-metadata';

/**
 * Metadata key set on the method descriptor by @EventStream().
 * The TsgonestSseInterceptor checks for this to activate iterator→Observable bridging.
 */
export const TSGONEST_SSE_METADATA = '__tsgonest_sse__';

/**
 * Metadata key set by the tsgonest compile-time rewriter on the controller prototype.
 * Contains per-event-name [assertFn, stringifyFn] pairs for validation and serialization.
 */
export const TSGONEST_SSE_TRANSFORMS = '__tsgonest_sse_transforms__';

/**
 * Options for the @EventStream() decorator.
 */
export interface EventStreamOptions {
  /**
   * Heartbeat interval in milliseconds. When set, the interceptor emits empty
   * keep-alive frames at this interval to prevent proxy/load-balancer timeouts.
   * Set to 0 or omit to disable.
   */
  heartbeat?: number;
}

/**
 * Declares a controller method as a Server-Sent Events endpoint that returns
 * an async iterator (or sync iterator) instead of an rxjs Observable.
 *
 * Replaces NestJS's `@Sse()` with full type safety:
 * - Typed data payloads via `SseEvent<E, T>`
 * - Discriminated event unions via `SseEvents<M>`
 * - Compile-time validation and fast serialization of each event's data
 * - Automatic resource cleanup on client disconnect (`iterator.return()`)
 *
 * Sets three NestJS-compatible metadata keys (`path`, `method=GET`, `__sse__`)
 * so NestJS's router handles the SSE protocol natively (headers, SseStream,
 * socket tuning, Express/Fastify compatibility).
 *
 * @param path  - Route path (defaults to '/')
 * @param options - Optional configuration (heartbeat interval, etc.)
 *
 * @example
 * ```ts
 * import { EventStream, SseEvent, SseEvents } from '@tsgonest/runtime';
 *
 * // Simple single-type SSE
 * @EventStream('notifications')
 * async *stream(): AsyncGenerator<SseEvent<'notification', NotificationDto>> {
 *   for await (const n of this.service.watch()) {
 *     yield { event: 'notification', data: n };
 *   }
 * }
 *
 * // Discriminated multi-type SSE with heartbeat
 * type UserEvents = SseEvents<{
 *   created: UserDto;
 *   updated: UserDto;
 *   deleted: { id: string };
 * }>;
 *
 * @EventStream('events', { heartbeat: 30_000 })
 * async *events(): AsyncGenerator<UserEvents> {
 *   const cursor = await this.db.watchChanges();
 *   try {
 *     for await (const c of cursor) {
 *       yield { event: c.type, data: c.doc };
 *     }
 *   } finally {
 *     await cursor.close(); // runs on client disconnect
 *   }
 * }
 * ```
 */
export function EventStream(
  path?: string,
  options?: EventStreamOptions,
): MethodDecorator {
  return (
    target: object,
    propertyKey: string | symbol,
    descriptor: TypedPropertyDescriptor<any>,
  ) => {
    const resolvedPath = path && path.length ? path : '/';

    // NestJS routing metadata — identical to what @Sse() from @nestjs/common sets.
    // Using string literals directly so @tsgonest/runtime has zero @nestjs/common imports.
    Reflect.defineMetadata('path', resolvedPath, descriptor.value);
    Reflect.defineMetadata('method', 0 /* RequestMethod.GET */, descriptor.value);
    Reflect.defineMetadata('__sse__', true, descriptor.value);

    // tsgonest-specific: mark this as an iterator-based SSE endpoint with options.
    Reflect.defineMetadata(TSGONEST_SSE_METADATA, {
      heartbeat: options?.heartbeat ?? 0,
    }, descriptor.value);

    return descriptor;
  };
}
