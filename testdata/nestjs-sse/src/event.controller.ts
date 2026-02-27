// NOTE: Stub decorators for testing (same pattern as other testdata fixtures).
// In real projects these come from @nestjs/common and @tsgonest/runtime.

function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Query(name?: string): ParameterDecorator {
  return () => {};
}

// --- @tsgonest/runtime stubs ---

interface SseEvent<E extends string = string, T = unknown> {
  event: E;
  data: T;
  id?: string;
  retry?: number;
}

type SseEvents<M extends Record<string, unknown>> = {
  [K in keyof M & string]: SseEvent<K, M[K]>;
}[keyof M & string];

function EventStream(path?: string, options?: { heartbeat?: number }): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

// --- DTOs ---

import type { UserDto, NotificationDto, DeletePayload } from "./dto";

// ═══════════════════════════════════════════════════════════════════
// Case 1: Discriminated union — multiple event types with SseEvents<M>
// ═══════════════════════════════════════════════════════════════════

type UserEvents = SseEvents<{
  created: UserDto;
  updated: UserDto;
  deleted: DeletePayload;
}>;

@Controller("users")
export class UserEventController {
  @EventStream("events", { heartbeat: 30000 })
  async *streamUserEvents(): AsyncGenerator<UserEvents> {
    yield { event: "created", data: {} as UserDto };
  }
}

// ═══════════════════════════════════════════════════════════════════
// Case 2: Single typed event — SseEvent<'notification', NotificationDto>
// ═══════════════════════════════════════════════════════════════════

@Controller("notifications")
export class NotificationController {
  @EventStream("stream")
  async *streamNotifications(): AsyncGenerator<SseEvent<"notification", NotificationDto>> {
    yield { event: "notification", data: {} as NotificationDto };
  }
}

// ═══════════════════════════════════════════════════════════════════
// Case 3: Non-discriminated — generic event name (string, not literal)
// Falls back to '*' key for serialization
// ═══════════════════════════════════════════════════════════════════

@Controller("generic")
export class GenericEventController {
  @EventStream("stream")
  async *streamGeneric(): AsyncGenerator<SseEvent<string, UserDto>> {
    yield { event: "any-name", data: {} as UserDto };
  }
}

// ═══════════════════════════════════════════════════════════════════
// Case 4: EventStream with @Query() parameter — confirms parameter
// decorators work alongside @EventStream
// ═══════════════════════════════════════════════════════════════════

@Controller("filtered")
export class FilteredEventController {
  @EventStream("stream")
  async *streamFiltered(@Query("topic") topic: string): AsyncGenerator<SseEvent<"update", UserDto>> {
    yield { event: "update", data: {} as UserDto };
  }
}

// ═══════════════════════════════════════════════════════════════════
// Case 5: EventStream with no path — defaults to '/'
// ═══════════════════════════════════════════════════════════════════

@Controller("default-path")
export class DefaultPathController {
  @EventStream()
  async *streamDefault(): AsyncGenerator<SseEvent<"ping", { ts: number }>> {
    yield { event: "ping", data: { ts: Date.now() } };
  }
}

// ═══════════════════════════════════════════════════════════════════
// Case 6: Mixed controller — @EventStream + regular @Get in same class
// Confirms interceptor only activates on SSE routes
// ═══════════════════════════════════════════════════════════════════

@Controller("mixed")
export class MixedController {
  @Get("health")
  health(): string {
    return "ok";
  }

  @EventStream("events")
  async *streamMixed(): AsyncGenerator<SseEvent<"status", { online: boolean }>> {
    yield { event: "status", data: { online: true } };
  }
}

// ═══════════════════════════════════════════════════════════════════
// Case 7: Union at yield level — SseEvent<'a', A> | SseEvent<'b', B>
// (inline union rather than SseEvents<M>)
// ═══════════════════════════════════════════════════════════════════

@Controller("inline-union")
export class InlineUnionController {
  @EventStream("events")
  async *streamInlineUnion(): AsyncGenerator<
    SseEvent<"user", UserDto> | SseEvent<"notification", NotificationDto>
  > {
    yield { event: "user", data: {} as UserDto };
  }
}
