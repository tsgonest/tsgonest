import { spawn, type ChildProcess } from "child_process";
import { resolve } from "path";
import { rmSync, existsSync, writeFileSync } from "fs";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

export const INTEGRATION_DIR = resolve(FIXTURES_DIR, "integration");
export const INTEGRATION_DIST = resolve(INTEGRATION_DIR, "dist");

/**
 * Compile the integration fixture with tsgonest.
 * Returns the build result for assertions.
 *
 * Uses a filesystem marker (.tsgonest-e2e-built) to coordinate across parallel
 * Vitest workers — only the first worker cleans and rebuilds. Others skip if
 * the marker already exists (indicating a recent build in this test run).
 */
export function buildIntegrationFixture() {
  const distDir = resolve(INTEGRATION_DIR, "dist");
  const marker = resolve(distDir, ".tsgonest-e2e-built");

  // If another worker already compiled in this run, reuse it.
  if (existsSync(marker)) {
    return { exitCode: 0, stdout: "(cached)", stderr: "" };
  }

  if (existsSync(distDir)) {
    rmSync(distDir, { recursive: true });
  }
  const result = runTsgonest([
    "--project",
    "testdata/integration/tsconfig.json",
    "--config",
    "testdata/integration/tsgonest.config.json",
  ]);

  // Write marker so parallel workers skip recompilation.
  if (result.exitCode === 0 && existsSync(distDir)) {
    writeFileSync(marker, new Date().toISOString());
  }
  return result;
}

/**
 * Start a NestJS server from compiled output and wait for it to be ready.
 * Returns the base URL and a cleanup function.
 */
export async function startServer(
  entryFile: string,
  opts?: { runtime?: string }
): Promise<{ url: string; process: ChildProcess; stop: () => Promise<void> }> {
  const entryPath = resolve(INTEGRATION_DIST, entryFile);
  const runtime = opts?.runtime || "node";

  return new Promise((resolvePromise, reject) => {
    const child = spawn(runtime, [entryPath], {
      cwd: INTEGRATION_DIR,
      env: { ...process.env, PORT: "0", NODE_OPTIONS: "--enable-source-maps" },
      stdio: ["pipe", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";
    const timeout = setTimeout(() => {
      child.kill();
      reject(
        new Error(
          `Server did not start within 15s.\nstdout: ${stdout}\nstderr: ${stderr}`
        )
      );
    }, 15000);

    child.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString();
      const match = stdout.match(/LISTENING:(https?:\/\/[^\s\n]+)/);
      if (match) {
        clearTimeout(timeout);
        const url = match[1].replace("[::1]", "127.0.0.1");
        resolvePromise({
          url,
          process: child,
          stop: () =>
            new Promise<void>((res) => {
              child.on("exit", () => res());
              child.kill("SIGTERM");
              // Force kill after 5s
              setTimeout(() => {
                child.kill("SIGKILL");
                res();
              }, 5000);
            }),
        });
      }
    });

    child.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString();
    });

    child.on("error", (err) => {
      clearTimeout(timeout);
      reject(err);
    });

    child.on("exit", (code) => {
      clearTimeout(timeout);
      if (!stdout.includes("LISTENING:")) {
        reject(
          new Error(
            `Server exited with code ${code} before listening.\nstdout: ${stdout}\nstderr: ${stderr}`
          )
        );
      }
    });
  });
}

/**
 * Consume an SSE stream and collect events until the connection closes or timeout.
 */
export async function collectSseEvents(
  url: string,
  opts?: { maxEvents?: number; timeoutMs?: number; abortAfterMs?: number }
): Promise<
  Array<{ event?: string; data: string; id?: string; retry?: string }>
> {
  const maxEvents = opts?.maxEvents ?? 100;
  const timeoutMs = opts?.timeoutMs ?? 10000;
  const abortAfterMs = opts?.abortAfterMs;

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  if (abortAfterMs !== undefined) {
    setTimeout(() => controller.abort(), abortAfterMs);
  }

  const events: Array<{
    event?: string;
    data: string;
    id?: string;
    retry?: string;
  }> = [];

  try {
    const response = await fetch(url, {
      headers: { Accept: "text/event-stream" },
      signal: controller.signal,
    });

    const reader = response.body!.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let currentEvent: Record<string, string> = {};

    while (events.length < maxEvents) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop()!; // keep incomplete line

      for (const line of lines) {
        if (line === "") {
          // Empty line = end of event block
          if (
            currentEvent.data !== undefined ||
            currentEvent.event !== undefined ||
            currentEvent.id !== undefined
          ) {
            events.push({
              event: currentEvent.event,
              data: currentEvent.data ?? "",
              id: currentEvent.id,
              retry: currentEvent.retry,
            });
          }
          currentEvent = {};
          if (events.length >= maxEvents) break;
        } else if (line.startsWith("data:")) {
          const val = line.slice(5).trimStart();
          currentEvent.data =
            currentEvent.data !== undefined
              ? currentEvent.data + "\n" + val
              : val;
        } else if (line.startsWith("event:")) {
          currentEvent.event = line.slice(6).trimStart();
        } else if (line.startsWith("id:")) {
          currentEvent.id = line.slice(3).trimStart();
        } else if (line.startsWith("retry:")) {
          currentEvent.retry = line.slice(6).trimStart();
        }
      }
    }
  } catch (err: any) {
    if (err.name !== "AbortError") throw err;
  } finally {
    clearTimeout(timeout);
  }

  return events;
}
