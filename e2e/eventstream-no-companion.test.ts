/**
 * E2E tests for @EventStream SSE endpoints where data types have NO companion
 * files. This verifies that tsgonest auto-injects TsgonestSseInterceptor
 * regardless of companion file availability.
 *
 * Without the interceptor, async generators cannot be bridged to Observables
 * and NestJS's SSE handler (SseStream) fails to subscribe — connections open
 * then immediately close with zero events.
 *
 * Tests run against Express, Fastify, and Bun adapters.
 */
import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { execSync } from "child_process";
import type { ChildProcess } from "child_process";
import {
  buildIntegrationFixture,
  startServer,
  collectSseEvents,
  INTEGRATION_DIST,
} from "./integration-helpers";
import { readFileSync } from "fs";
import { resolve } from "path";

let compiled = false;

function ensureCompiled() {
  if (!compiled) {
    const result = buildIntegrationFixture();
    expect(result.exitCode).toBe(0);
    compiled = true;
  }
}

/**
 * Shared SSE-without-companion test cases — same assertions for every platform.
 * The controller at /sse-auto uses @EventStream() without manually adding
 * @UseInterceptors(TsgonestSseInterceptor) and its data types have no companions.
 */
function sseNoCompanionTests(getUrl: () => string) {
  it("should stream inline-typed SSE events (no companion)", async () => {
    const events = await collectSseEvents(`${getUrl()}/sse-auto/simple`, {
      timeoutMs: 5000,
    });

    // The generator yields 2 events then completes
    expect(events.length).toBe(2);

    expect(events[0].event).toBe("ping");
    expect(events[1].event).toBe("ping");

    const data0 = JSON.parse(events[0].data);
    expect(data0).toHaveProperty("ts");
    expect(typeof data0.ts).toBe("number");

    const data1 = JSON.parse(events[1].data);
    expect(data1).toHaveProperty("ts");
    expect(data1.ts).toBeGreaterThanOrEqual(data0.ts);
  });

  it("should stream Record<string, unknown> SSE events (no variants)", async () => {
    const events = await collectSseEvents(`${getUrl()}/sse-auto/dynamic`, {
      timeoutMs: 5000,
    });

    expect(events.length).toBe(2);

    expect(events[0].event).toBe("update");
    expect(events[1].event).toBe("update");

    const data0 = JSON.parse(events[0].data);
    expect(data0).toEqual({ key: "status", value: "active" });

    const data1 = JSON.parse(events[1].data);
    expect(data1).toEqual({ key: "count", value: 42 });
  });

  it("should complete cleanly after generator exhaustion (burst)", async () => {
    const events = await collectSseEvents(`${getUrl()}/sse-auto/burst`, {
      timeoutMs: 5000,
    });

    // Generator yields exactly 3 items then returns
    expect(events.length).toBe(3);

    for (let i = 0; i < 3; i++) {
      expect(events[i].event).toBe("item");
      const data = JSON.parse(events[i].data);
      expect(data).toEqual({ seq: i });
    }
  });

  // Multiple @EventStream endpoints in the same controller — all 3 above
  // already test this (simple, dynamic, burst are on the same SseAutoController).
  // This explicit test verifies they can be hit independently without interfering.
  it("should handle multiple SSE endpoints on the same controller independently", async () => {
    // Hit two endpoints concurrently
    const [simpleEvents, burstEvents] = await Promise.all([
      collectSseEvents(`${getUrl()}/sse-auto/simple`, { timeoutMs: 5000 }),
      collectSseEvents(`${getUrl()}/sse-auto/burst`, { timeoutMs: 5000 }),
    ]);

    expect(simpleEvents.length).toBe(2);
    expect(simpleEvents[0].event).toBe("ping");

    expect(burstEvents.length).toBe(3);
    expect(burstEvents[0].event).toBe("item");
  });

  // Second controller in the same file (SseExtraController at /sse-extra)
  // verifies the interceptor is injected per-controller, not just once per file.
  it("should stream from a second SSE controller in the same file", async () => {
    const events = await collectSseEvents(`${getUrl()}/sse-extra/health`, {
      timeoutMs: 5000,
    });

    expect(events.length).toBe(1);
    expect(events[0].event).toBe("status");

    const data = JSON.parse(events[0].data);
    expect(data).toEqual({ ok: true });
  });

  it("should stream multiple events from the second controller", async () => {
    const events = await collectSseEvents(`${getUrl()}/sse-extra/logs`, {
      timeoutMs: 5000,
    });

    expect(events.length).toBe(2);
    expect(events[0].event).toBe("log");
    expect(events[1].event).toBe("log");

    const data0 = JSON.parse(events[0].data);
    expect(data0.msg).toBe("started");

    const data1 = JSON.parse(events[1].data);
    expect(data1.msg).toBe("ready");
  });

  // Cross-controller concurrent test — both controllers in the same file
  // serving SSE simultaneously.
  it("should handle SSE from both controllers in the same file concurrently", async () => {
    const [autoEvents, extraEvents] = await Promise.all([
      collectSseEvents(`${getUrl()}/sse-auto/simple`, { timeoutMs: 5000 }),
      collectSseEvents(`${getUrl()}/sse-extra/health`, { timeoutMs: 5000 }),
    ]);

    expect(autoEvents.length).toBe(2);
    expect(autoEvents[0].event).toBe("ping");

    expect(extraEvents.length).toBe(1);
    expect(extraEvents[0].event).toBe("status");
  });
}

// ── Verify compiled output has auto-injected interceptor ─────────────

describe("EventStream auto-injection (compiled output)", () => {
  beforeAll(() => {
    ensureCompiled();
  });

  it("should auto-inject TsgonestSseInterceptor in compiled controller", () => {
    const compiled = readFileSync(
      resolve(INTEGRATION_DIST, "sse-no-companion.controller.js"),
      "utf-8"
    );

    // The compiled output should contain UseInterceptors(TsgonestSseInterceptor)
    // auto-injected by tsgonest's controller rewriter, even though the source
    // does not manually add it.
    expect(compiled).toContain("TsgonestSseInterceptor");
    expect(compiled).toContain(
      "(0, common_1.UseInterceptors)(TsgonestSseInterceptor)"
    );
  });

  it("should inject interceptor into both controllers in the same file", () => {
    const compiled = readFileSync(
      resolve(INTEGRATION_DIST, "sse-no-companion.controller.js"),
      "utf-8"
    );

    // Count interceptor injections — should be 2 (one per controller class)
    const matches = compiled.match(
      /\(0, common_1\.UseInterceptors\)\(TsgonestSseInterceptor\)/g
    );
    expect(matches).not.toBeNull();
    expect(matches!.length).toBe(2);
  });
});

// ── Express ──────────────────────────────────────────────────────────

describe("EventStream without companions (Express)", () => {
  let url: string;
  let serverProcess: ChildProcess;
  let stop: () => Promise<void>;

  beforeAll(async () => {
    ensureCompiled();
    const server = await startServer("main-express.js");
    url = server.url;
    serverProcess = server.process;
    stop = server.stop;
  }, 60000);

  afterAll(async () => {
    await stop?.();
  });

  sseNoCompanionTests(() => url);
});

// ── Fastify ──────────────────────────────────────────────────────────

describe("EventStream without companions (Fastify)", () => {
  let url: string;
  let serverProcess: ChildProcess;
  let stop: () => Promise<void>;

  beforeAll(async () => {
    ensureCompiled();
    const server = await startServer("main-fastify.js");
    url = server.url;
    serverProcess = server.process;
    stop = server.stop;
  }, 60000);

  afterAll(async () => {
    await stop?.();
  });

  sseNoCompanionTests(() => url);
});

// ── Bun ──────────────────────────────────────────────────────────────

function hasBun(): boolean {
  try {
    execSync("bun --version", { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const describeBun = hasBun() ? describe : describe.skip;

describeBun("EventStream without companions (Bun)", () => {
  let url: string;
  let serverProcess: ChildProcess;
  let stop: () => Promise<void>;

  beforeAll(async () => {
    ensureCompiled();
    const server = await startServer("main-bun.js", { runtime: "bun" });
    url = server.url;
    serverProcess = server.process;
    stop = server.stop;
  }, 60000);

  afterAll(async () => {
    await stop?.();
  });

  sseNoCompanionTests(() => url);
});
