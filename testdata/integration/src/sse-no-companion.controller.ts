import { Controller } from "@nestjs/common";
import { EventStream, SseEvent } from "@tsgonest/runtime";

// This controller intentionally does NOT add @UseInterceptors(TsgonestSseInterceptor).
// tsgonest should auto-inject it for any controller with @EventStream() routes.
// Without the interceptor, async generators cannot be bridged to Observables
// and NestJS's SSE handler (SseStream) will fail to subscribe.

// ── Controller 1: Multiple @EventStream endpoints in the same controller ─────

@Controller("sse-auto")
export class SseAutoController {
  // Case 1: Inline data type — no companion file will be generated
  @EventStream("simple")
  async *simple(): AsyncGenerator<SseEvent<"ping", { ts: number }>> {
    yield { event: "ping", data: { ts: Date.now() } };
    yield { event: "ping", data: { ts: Date.now() + 1 } };
  }

  // Case 2: Record<string, unknown> — no SSE event variants detected by analyzer
  @EventStream("dynamic")
  async *dynamic(): AsyncGenerator<
    SseEvent<"update", Record<string, unknown>>
  > {
    yield { event: "update", data: { key: "status", value: "active" } };
    yield { event: "update", data: { key: "count", value: 42 } };
  }

  // Case 3: Multiple yields then end — verifies generator completes and
  // the Observable completes, closing the SSE connection cleanly.
  @EventStream("burst")
  async *burst(): AsyncGenerator<SseEvent<"item", { seq: number }>> {
    for (let i = 0; i < 3; i++) {
      yield { event: "item", data: { seq: i } };
    }
  }
}

// ── Controller 2: Second SSE controller in the same file ─────────────────────
// Tests that tsgonest injects the interceptor into BOTH controllers when they
// are co-located in a single source file.

@Controller("sse-extra")
export class SseExtraController {
  @EventStream("health")
  async *health(): AsyncGenerator<SseEvent<"status", { ok: boolean }>> {
    yield { event: "status", data: { ok: true } };
  }

  @EventStream("logs")
  async *logs(): AsyncGenerator<SseEvent<"log", { msg: string; ts: number }>> {
    yield { event: "log", data: { msg: "started", ts: Date.now() } };
    yield { event: "log", data: { msg: "ready", ts: Date.now() + 1 } };
  }
}
