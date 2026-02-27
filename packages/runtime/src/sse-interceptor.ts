import { Observable } from 'rxjs';
import { switchMap } from 'rxjs/operators';
import { TSGONEST_SSE_METADATA, TSGONEST_SSE_TRANSFORMS } from './event-stream';

/**
 * Per-event-name transform pair: [assertFn | null, stringifyFn | null].
 *
 * Keyed by literal event name (e.g., "created", "updated").
 * The special key '*' is the fallback applied when no specific event name matches
 * (used for non-discriminated `SseEvent<string, T>` where the event name is generic).
 */
export interface SseTransformMap {
  [eventName: string]: [
    ((data: any) => any) | null,     // assert (validate) — throws on invalid data
    ((data: any) => string) | null,  // stringify (serialize) — returns JSON string
  ];
}

function isAsyncIterable(value: unknown): value is AsyncIterable<unknown> {
  return (
    value != null &&
    typeof (value as any)[Symbol.asyncIterator] === 'function'
  );
}

function isIterable(value: unknown): value is Iterable<unknown> {
  return (
    value != null &&
    typeof value !== 'string' &&
    typeof (value as any)[Symbol.iterator] === 'function'
  );
}

/**
 * Bridges an async/sync iterable to an Observable<MessageEvent> that NestJS's
 * SseStream can subscribe to. Applies per-event validation and fast serialization.
 *
 * Memory safety:
 * - When NestJS unsubscribes (client disconnect), the teardown calls `iterator.return()`
 *   which triggers the generator's `finally` blocks for resource cleanup.
 * - The `cancelled` flag prevents any further emissions after teardown.
 * - Heartbeat timers are always cleared in teardown.
 * - Errors in the generator are caught and emitted as a typed `event: error` frame,
 *   then the Observable completes gracefully.
 */
function iterableToObservable(
  iterable: AsyncIterable<any> | Iterable<any>,
  transforms: SseTransformMap | undefined,
  heartbeatMs: number,
): Observable<any> {
  return new Observable((subscriber) => {
    const isAsync = typeof (iterable as any)[Symbol.asyncIterator] === 'function';
    const iterator: AsyncIterator<any> | Iterator<any> = isAsync
      ? (iterable as AsyncIterable<any>)[Symbol.asyncIterator]()
      : (iterable as Iterable<any>)[Symbol.iterator]();

    let cancelled = false;
    let heartbeatTimer: ReturnType<typeof setInterval> | null = null;

    // Heartbeat: emit empty-data keep-alive frame at the configured interval.
    // NestJS's SseStream treats falsy `data` as no data line → outputs just "\n"
    // which is enough to keep connections alive through proxies.
    if (heartbeatMs > 0) {
      heartbeatTimer = setInterval(() => {
        if (!cancelled) {
          subscriber.next({ data: '' });
        }
      }, heartbeatMs);
    }

    const pump = async () => {
      try {
        while (!cancelled) {
          const result = await iterator.next();
          if (result.done || cancelled) break;

          const event = result.value;
          const eventName: string | undefined = event.event;

          // Resolve transforms: specific event name → '*' fallback → none
          const pair =
            (eventName && transforms?.[eventName]) ||
            transforms?.['*'];
          const assertFn = pair?.[0];
          const stringifyFn = pair?.[1];

          // Validate: throws TsgonestValidationError on invalid data
          if (assertFn) {
            event.data = assertFn(event.data);
          }

          // Serialize: companion stringify returns a JSON string.
          // When data is already a string, NestJS's SseStream toDataString sends it as-is.
          // When data is an object, NestJS calls JSON.stringify internally.
          // By pre-serializing with the companion, we get fast serialization AND
          // avoid NestJS's generic JSON.stringify.
          const data = stringifyFn ? stringifyFn(event.data) : event.data;

          // Emit as NestJS MessageEvent.
          // NestJS's MessageEvent uses `type` for the event name field
          // (which maps to SSE `event:` on the wire).
          subscriber.next({
            data,
            type: eventName,
            id: event.id,
            retry: event.retry,
          });
        }
        if (!cancelled) {
          subscriber.complete();
        }
      } catch (err) {
        if (!cancelled) {
          // Emit a typed error event that matches the OpenAPI error variant schema,
          // then complete gracefully. This prevents NestJS's own catchError from
          // double-emitting an error event with a different format.
          const message = err instanceof Error ? err.message : String(err);
          subscriber.next({ type: 'error', data: message });
          subscriber.complete();
        }
      }
    };

    pump();

    // Teardown: fires when NestJS unsubscribes (client disconnect via req 'close' event).
    // This is the critical memory-leak prevention mechanism:
    // 1. cancelled = true → stops the pump loop
    // 2. clearInterval → stops heartbeat emissions
    // 3. iterator.return() → triggers generator's finally{} blocks
    //    (close DB cursors, file streams, event subscriptions, etc.)
    return () => {
      cancelled = true;
      if (heartbeatTimer !== null) {
        clearInterval(heartbeatTimer);
        heartbeatTimer = null;
      }
      // iterator.return() on an async generator causes it to:
      // - Resolve any pending `await` with {value: undefined, done: true}
      // - Execute `finally` blocks
      // - Return {value: undefined, done: true}
      (iterator as AsyncIterator<any>).return?.(undefined);
    };
  });
}

/**
 * NestJS interceptor that bridges async iterators to Observable<MessageEvent>
 * for NestJS's native SSE handler (SseStream).
 *
 * Activated only on methods decorated with @EventStream(). For regular @Sse()
 * routes returning Observables, this interceptor passes through unchanged.
 *
 * Reads compile-time injected serialization/validation transforms from
 * `Reflect.getMetadata(TSGONEST_SSE_TRANSFORMS, ...)` set by tsgonest's
 * controller rewriter.
 *
 * Auto-injected by `tsgonest build` on controller classes that have
 * @EventStream() routes.
 */
export class TsgonestSseInterceptor {
  intercept(context: any, next: any): any {
    const handler = context.getHandler();
    const sseMeta = Reflect.getMetadata(TSGONEST_SSE_METADATA, handler);

    // Not an @EventStream() route — pass through unchanged.
    // This allows the interceptor to be applied at class level alongside
    // regular routes and @Sse() routes without interference.
    if (!sseMeta) {
      return next.handle();
    }

    // Read per-event serialization/validation transforms injected by tsgonest.
    // Keyed on controller prototype + method name (set after __decorate).
    const transforms: SseTransformMap | undefined = Reflect.getMetadata(
      TSGONEST_SSE_TRANSFORMS,
      context.getClass().prototype,
      handler.name,
    );

    const heartbeatMs: number = sseMeta.heartbeat ?? 0;

    return next.handle().pipe(
      switchMap((value: any) => {
        // Async iterables (from async generators, AsyncIterable implementations)
        if (isAsyncIterable(value)) {
          return iterableToObservable(value, transforms, heartbeatMs);
        }
        // Sync iterables (from generator functions, arrays of events)
        if (isIterable(value)) {
          return iterableToObservable(value, transforms, heartbeatMs);
        }
        // Observable passthrough: backward compat with NestJS @Sse() + Observable pattern.
        // If the handler returns an Observable (e.g., from rxjs interval().pipe(...)),
        // let it pass through to NestJS's SseStream directly.
        return value;
      }),
    );
  }
}
