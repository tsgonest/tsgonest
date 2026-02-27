/**
 * A typed Server-Sent Event. Replaces NestJS's weakly-typed MessageEvent.
 *
 * NestJS's MessageEvent has `data: string | object` and `type?: string` —
 * no generic type parameter, no narrowing on event names.
 *
 * SseEvent fixes both: `data` is generically typed via `T`, and `event` is
 * a string literal type via `E` that acts as a discriminant in unions.
 *
 * @typeParam E - Event name as a string literal (discriminant in unions)
 * @typeParam T - Payload type for the `data` field
 *
 * @example
 * ```ts
 * // Single event type
 * SseEvent<'notification', NotificationDto>
 *
 * // Discriminated union via SseEvents utility
 * SseEvents<{ created: UserDto; deleted: { id: string } }>
 * ```
 */
export interface SseEvent<E extends string = string, T = unknown> {
  /** Maps to the SSE `event:` field. Used as discriminant in unions. */
  event: E;
  /** Payload. Serialized via companion stringify or JSON.stringify. */
  data: T;
  /** Maps to the SSE `id:` field. Auto-incremented if omitted. */
  id?: string;
  /** Maps to the SSE `retry:` field. Client reconnection interval in ms. */
  retry?: number;
}

/**
 * Converts an event-map record to a discriminated union of SseEvent types.
 *
 * Each key becomes a literal `event` discriminant, and each value becomes the
 * corresponding `data` type. TypeScript narrows `data` when you check `event`.
 *
 * @typeParam M - Record mapping event names to data payload types
 *
 * @example
 * ```ts
 * type UserEvents = SseEvents<{
 *   created: UserDto;
 *   updated: UserDto;
 *   deleted: { id: string };
 * }>;
 * // Equivalent to:
 * // | SseEvent<'created', UserDto>
 * // | SseEvent<'updated', UserDto>
 * // | SseEvent<'deleted', { id: string }>
 *
 * // TypeScript narrows correctly:
 * function handle(e: UserEvents) {
 *   if (e.event === 'deleted') {
 *     e.data.id; // ✅ { id: string }
 *   }
 * }
 * ```
 */
export type SseEvents<M extends Record<string, unknown>> = {
  [K in keyof M & string]: SseEvent<K, M[K]>;
}[keyof M & string];
