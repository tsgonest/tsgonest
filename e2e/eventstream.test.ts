import { describe, it, expect, beforeAll, afterAll } from "vitest";
import type { ChildProcess } from "child_process";
import {
  buildIntegrationFixture,
  startServer,
  collectSseEvents,
} from "./integration-helpers";

let compiled = false;

function sseTests(platform: "express" | "fastify") {
  const entryFile =
    platform === "express" ? "main-express.js" : "main-fastify.js";
  let url: string;
  let serverProcess: ChildProcess;
  let stop: () => Promise<void>;

  beforeAll(async () => {
    if (!compiled) {
      const result = buildIntegrationFixture();
      expect(result.exitCode).toBe(0);
      compiled = true;
    }
    const server = await startServer(entryFile);
    url = server.url;
    serverProcess = server.process;
    stop = server.stop;
  }, 60000);

  afterAll(async () => {
    await stop?.();
  });

  it("should stream discriminated multi-type events", async () => {
    const events = await collectSseEvents(`${url}/sse/events`, {
      timeoutMs: 5000,
    });

    // Should have 2 events: user + notification
    expect(events.length).toBe(2);

    const userEvent = events.find((e) => e.event === "user");
    expect(userEvent).toBeDefined();
    const userData = JSON.parse(userEvent!.data);
    expect(userData).toEqual({ id: 1, name: "Alice", action: "login" });

    const notifEvent = events.find((e) => e.event === "notification");
    expect(notifEvent).toBeDefined();
    const notifData = JSON.parse(notifEvent!.data);
    expect(notifData).toEqual({ message: "System ready", level: "info" });
  });

  it("should stream single-type events", async () => {
    const events = await collectSseEvents(`${url}/sse/notifications`, {
      timeoutMs: 5000,
    });

    expect(events.length).toBe(2);
    expect(events[0].event).toBe("alert");
    expect(events[1].event).toBe("alert");

    const data0 = JSON.parse(events[0].data);
    expect(data0).toEqual({ message: "Hello", level: "info" });

    const data1 = JSON.parse(events[1].data);
    expect(data1).toEqual({ message: "Warn!", level: "warn" });
  });

  it("should stream bad data without transforms (no validation injected)", async () => {
    const events = await collectSseEvents(`${url}/sse/bad-data`, {
      timeoutMs: 5000,
    });

    // Without SSE transforms injected by tsgonest, the invalid data passes through.
    // The event is emitted normally (no error frame).
    expect(events.length).toBe(1);
    expect(events[0].event).toBe("item");
    const data = JSON.parse(events[0].data);
    expect(data).toEqual({ id: 1, name: "", action: "test" });
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
