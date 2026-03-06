import { Controller, Query, UseInterceptors } from "@nestjs/common";
import {
  EventStream,
  SseEvent,
  SseEvents,
  TsgonestSseInterceptor,
} from "@tsgonest/runtime";
import type { UserEvent, NotificationEvent } from "./dto";

// Discriminated multi-event union
type AppEvents = SseEvents<{
  user: UserEvent;
  notification: NotificationEvent;
}>;

@Controller("sse")
@UseInterceptors(TsgonestSseInterceptor)
export class SseController {
  // Case 1: Discriminated multi-type SSE with validation + serialization
  @EventStream("events")
  async *events(): AsyncGenerator<AppEvents> {
    yield {
      event: "user",
      data: { id: 1, name: "Alice", action: "login" },
    };
    yield {
      event: "notification",
      data: { message: "System ready", level: "info" },
    };
  }

  // Case 2: Single-type SSE
  @EventStream("notifications")
  async *notifications(): AsyncGenerator<
    SseEvent<"alert", NotificationEvent>
  > {
    yield {
      event: "alert",
      data: { message: "Hello", level: "info" },
    };
    yield {
      event: "alert",
      data: { message: "Warn!", level: "warn" },
    };
  }

  // Case 3: SSE that yields invalid data — should emit error frame
  @EventStream("bad-data")
  async *badData(): AsyncGenerator<SseEvent<"item", UserEvent>> {
    // This data is invalid: name="" violates @minLength 1
    yield {
      event: "item",
      data: { id: 1, name: "", action: "test" } as UserEvent,
    };
  }

  // Case 4: Heartbeat test — yields one event then ends
  @EventStream("heartbeat", { heartbeat: 200 })
  async *heartbeat(): AsyncGenerator<SseEvent<"ping", { ts: number }>> {
    // Wait 500ms so at least 1-2 heartbeat frames arrive before the real event
    await new Promise((r) => setTimeout(r, 500));
    yield { event: "ping", data: { ts: Date.now() } };
  }

  // Case 5: Client disconnect — generator cleanup via finally{}
  @EventStream("disconnect")
  async *disconnect(
    @Query("signal") signal?: string
  ): AsyncGenerator<SseEvent<"tick", { n: number }>> {
    let i = 0;
    try {
      while (true) {
        yield { event: "tick", data: { n: i++ } };
        await new Promise((r) => setTimeout(r, 100));
      }
    } finally {
      // Signal cleanup happened by writing to a global
      (globalThis as any).__sseCleanedUp = true;
    }
  }
}
