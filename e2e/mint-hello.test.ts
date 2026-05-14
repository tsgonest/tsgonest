import { spawn, type ChildProcess } from "child_process";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { FIXTURES_DIR, runTsgonest } from "./helpers";

const FIXTURE_DIR = resolve(FIXTURES_DIR, "mint-hello");
const FIXTURE_DIST = resolve(FIXTURE_DIR, "dist");

async function startBunServer(): Promise<{
  url: string;
  process: ChildProcess;
  stop: () => Promise<void>;
}> {
  return new Promise((resolvePromise, reject) => {
    const child = spawn("bun", [resolve(FIXTURE_DIST, "main.js")], {
      cwd: FIXTURE_DIR,
      env: { ...process.env, PORT: "0" },
      stdio: ["pipe", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";
    const timeout = setTimeout(() => {
      child.kill();
      reject(
        new Error(
          `bun did not listen within 10s.\nstdout: ${stdout}\nstderr: ${stderr}`,
        ),
      );
    }, 10000);

    child.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString();
      const match = stdout.match(/LISTENING:(https?:\/\/[^\s\n/]+)\/?/);
      if (!match) return;
      clearTimeout(timeout);
      resolvePromise({
        url: match[1].replace("[::1]", "127.0.0.1"),
        process: child,
        stop: () =>
          new Promise<void>((res) => {
            child.on("exit", () => res());
            child.kill("SIGTERM");
            setTimeout(() => {
              child.kill("SIGKILL");
              res();
            }, 3000);
          }),
      });
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
            `bun exited with ${code} before listening.\nstdout: ${stdout}\nstderr: ${stderr}`,
          ),
        );
      }
    });
  });
}

describe("Mint Phase 1: Hello World seam", () => {
  beforeAll(() => {
    if (existsSync(FIXTURE_DIST)) {
      rmSync(FIXTURE_DIST, { recursive: true });
    }
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/mint-hello/tsconfig.json",
      "--config",
      "testdata/mint-hello/tsgonest.config.json",
    ]);
    if (exitCode !== 0) {
      throw new Error(`tsgonest build failed: ${stderr}`);
    }
  });

  it("emits the registerHelloController companion", () => {
    const companionJs = resolve(
      FIXTURE_DIST,
      "hello.controller.HelloController.tsgonest.js",
    );
    expect(existsSync(companionJs)).toBe(true);
    const content = readFileSync(companionJs, "utf-8");
    expect(content).toContain(
      'import { HelloController } from "./hello.controller"',
    );
    expect(content).toContain("export function registerHelloController(app)");
    expect(content).toContain('app.router.add("GET", "/hello"');
  });

  it("emits a clean controller JS without NestJS interceptor injections", () => {
    const ctrlJs = readFileSync(
      resolve(FIXTURE_DIST, "hello.controller.js"),
      "utf-8",
    );
    expect(ctrlJs).not.toContain("TsgonestSerializeInterceptor");
    expect(ctrlJs).not.toContain("UseInterceptors");
    expect(ctrlJs).toContain('from "@mintkit/core"');
    expect(ctrlJs).toContain('return "Hello from Mint!"');
  });

  it("includes the route in the OpenAPI 3.2 document", () => {
    const openapi = JSON.parse(
      readFileSync(resolve(FIXTURE_DIST, "openapi.json"), "utf-8"),
    );
    expect(openapi.openapi).toMatch(/^3\.[12](\.\d+)?$/);
    expect(openapi.paths["/hello"]).toBeDefined();
    expect(openapi.paths["/hello"].get).toBeDefined();
    expect(openapi.paths["/hello"].get.operationId).toBe("Hello_hello");
  });

  it("serves /hello via Bun.serve(app.fetch) with 200 + JSON body", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(`${server.url}/hello`);
      expect(res.status).toBe(200);
      // Phase 2: all return values are JSON-serialized so SDKs can consume
      // responses without sniffing the handler signature. Strings become a
      // quoted JSON literal.
      expect(res.headers.get("content-type")).toMatch(/application\/json/);
      expect(await res.text()).toBe('"Hello from Mint!"');
    } finally {
      await server.stop();
    }
  });

  it("returns 404 for unknown routes", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(`${server.url}/nope`);
      expect(res.status).toBe(404);
    } finally {
      await server.stop();
    }
  });

  afterAll(() => {
    // dist is left in place so it can be inspected; the next run cleans it.
  });
});
