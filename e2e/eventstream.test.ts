import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { execSync } from "child_process";
import type { ChildProcess } from "child_process";
import {
  buildIntegrationFixture,
  startServer,
  collectSseEvents,
} from "./integration-helpers";

let compiled = false;

function ensureCompiled() {
  if (!compiled) {
    const result = buildIntegrationFixture();
    expect(result.exitCode).toBe(0);
    compiled = true;
  }
}

function sseTests(platform: "express" | "fastify" | "bun") {
  const entryFile =
    platform === "express" ? "main-express.js" :
    platform === "fastify" ? "main-fastify.js" : "main-bun.js";
  const runtime = platform === "bun" ? "bun" : "node";
  let url: string;
  let serverProcess: ChildProcess;
  let stop: () => Promise<void>;

  beforeAll(async () => {
    ensureCompiled();
    const server = await startServer(entryFile, { runtime });
    url = server.url;
    serverProcess = server.process;
    stop = server.stop;
  }, 60000);

  afterAll(async () => {
    await stop?.();
  });

  it("should stream discriminated multi-type events with companion serialization", async () => {
    const events = await collectSseEvents(`${url}/sse/events`, {
      timeoutMs: 5000,
    });

    // Should have 2 events: user + notification
    expect(events.length).toBe(2);

    const userEvent = events.find((e) => e.event === "user");
    expect(userEvent).toBeDefined();
    // Data goes through companion stringifyUserEvent → pre-serialized JSON string
    const userData = JSON.parse(userEvent!.data);
    expect(userData).toEqual({ id: 1, name: "Alice", action: "login" });

    const notifEvent = events.find((e) => e.event === "notification");
    expect(notifEvent).toBeDefined();
    // Data goes through companion stringifyNotificationEvent
    const notifData = JSON.parse(notifEvent!.data);
    expect(notifData).toEqual({ message: "System ready", level: "info" });
  });

  it("should serialize single-type events via companion stringify", async () => {
    const events = await collectSseEvents(`${url}/sse/notifications`, {
      timeoutMs: 5000,
    });

    expect(events.length).toBe(2);
    expect(events[0].event).toBe("alert");
    expect(events[1].event).toBe("alert");

    // Companion stringifyNotificationEvent produces pre-serialized JSON
    const data0 = JSON.parse(events[0].data);
    expect(data0).toEqual({ message: "Hello", level: "info" });

    const data1 = JSON.parse(events[1].data);
    expect(data1).toEqual({ message: "Warn!", level: "warn" });
  });

  it("should reject invalid SSE data with error frame (validation)", async () => {
    const events = await collectSseEvents(`${url}/sse/bad-data`, {
      timeoutMs: 5000,
    });

    // With companion-backed SSE transforms, assertUserEvent catches
    // name="" (violates @minLength 1) and the interceptor emits an error frame.
    expect(events.length).toBe(1);
    expect(events[0].event).toBe("error");
    // Error frame data contains the TsgonestValidationError message
    expect(events[0].data).toContain("Validation failed");
  });

  it("should emit heartbeat frames before real event", async () => {
    const events = await collectSseEvents(`${url}/sse/heartbeat`, {
      timeoutMs: 3000,
    });

    // The generator waits 500ms, heartbeat is 200ms.
    // Heartbeat frames are empty (id-only, no event/data fields).
    // Real events have event: "ping" with data.
    const heartbeats = events.filter(
      (e) => e.event === undefined && (!e.data || e.data === "")
    );
    const pings = events.filter((e) => e.event === "ping");

    expect(heartbeats.length).toBeGreaterThanOrEqual(1);
    expect(pings.length).toBe(1);

    const pingData = JSON.parse(pings[0].data);
    expect(pingData).toHaveProperty("ts");
    expect(typeof pingData.ts).toBe("number");
  });

  it("should clean up generator on client disconnect", async () => {
    // Connect and receive a few ticks, then abort
    const events = await collectSseEvents(`${url}/sse/disconnect`, {
      abortAfterMs: 500,
      timeoutMs: 3000,
    });

    // Should have received some tick events before we aborted
    const ticks = events.filter((e) => e.event === "tick");
    expect(ticks.length).toBeGreaterThan(0);

    // Verify tick data is sequential
    for (let i = 0; i < ticks.length; i++) {
      const data = JSON.parse(ticks[i].data);
      expect(data.n).toBe(i);
    }
  });
}

describe("EventStream integration (Express)", () => {
  sseTests("express");
});

describe("EventStream integration (Fastify)", () => {
  sseTests("fastify");
});

function hasBun(): boolean {
  try {
    execSync("bun --version", { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const describeBun = hasBun() ? describe : describe.skip;

describeBun("EventStream integration (Bun)", () => {
  sseTests("bun");
});
