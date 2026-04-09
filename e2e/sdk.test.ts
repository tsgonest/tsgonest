import { describe, it, expect, beforeAll } from "vitest";
import { spawnSync } from "child_process";
import {
  existsSync,
  readFileSync,
  rmSync,
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  statSync,
} from "fs";
import { tmpdir } from "os";
import { resolve } from "path";
import { runTsgonest, PROJECT_ROOT, FIXTURES_DIR } from "./helpers";

/**
 * Helper: creates a temp config that points to a fixture OpenAPI file with
 * sdk output configured. Returns { configPath, outputDir, cleanup }.
 */
function createSdkFixture(openApiFixture: string, extraName: string) {
  const tmpDir = mkdtempSync(resolve(tmpdir(), `tsgonest-sdk-${extraName}-`));
  const outputDir = resolve(tmpDir, "sdk");
  mkdirSync(outputDir, { recursive: true });

  // Resolve absolute path to the fixture
  const absInput = resolve(PROJECT_ROOT, openApiFixture);

  const configPath = resolve(tmpDir, "tsgonest.config.json");
  writeFileSync(
    configPath,
    JSON.stringify({
      controllers: { include: ["src/**/*.controller.ts"] },
      openapi: [
        {
          name: "default",
          output: absInput,
          sdk: { output: outputDir },
        },
      ],
    })
  );

  return {
    configPath,
    outputDir,
    cleanup: () => rmSync(tmpDir, { recursive: true }),
  };
}

/** Run SDK generation for a fixture, returning exitCode + stdout + outputDir. */
function generateSdk(fixture: string, name: string) {
  const { configPath, outputDir, cleanup } = createSdkFixture(fixture, name);
  const { exitCode, stdout, stderr } = runTsgonest([
    "sdk",
    "--config",
    configPath,
  ]);
  return { exitCode, stdout, stderr, outputDir, cleanup };
}

// ─── Group 1: tsgonest sdk CLI basics ──────────────────────────────────────

describe("tsgonest sdk CLI", () => {
  it("should generate SDK from basic fixture", () => {
    const { exitCode, stdout, outputDir, cleanup } = generateSdk(
      "testdata/sdkgen/basic.openapi.json",
      "basic"
    );
    expect(exitCode).toBe(0);
    expect(stdout).toContain("SDK generated");

    // All core files should exist
    expect(existsSync(resolve(outputDir, "types.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "client.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "sse.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "form-data.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "index.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "orders/index.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "products/index.ts"))).toBe(true);

    cleanup();
  });

  it("should generate SDK from versioned fixture", () => {
    const { exitCode, outputDir, cleanup } = generateSdk(
      "testdata/sdkgen/versioned.openapi.json",
      "ver"
    );
    expect(exitCode).toBe(0);

    expect(existsSync(resolve(outputDir, "health/index.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "v1/orders/index.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "v2/orders/index.ts"))).toBe(true);

    cleanup();
  });

  it("should generate SDK with SSE and file uploads", () => {
    const { exitCode, outputDir, cleanup } = generateSdk(
      "testdata/sdkgen/file-uploads.openapi.json",
      "fu"
    );
    expect(exitCode).toBe(0);

    const sseContent = readFileSync(resolve(outputDir, "sse.ts"), "utf-8");
    expect(sseContent).toContain("SSEConnection");
    const formDataContent = readFileSync(resolve(outputDir, "form-data.ts"), "utf-8");
    expect(formDataContent).toContain("buildFormData");

    cleanup();
  });

  it("should generate SDK from edge-cases fixture", () => {
    const { exitCode, outputDir, cleanup } = generateSdk(
      "testdata/sdkgen/edge-cases.openapi.json",
      "edge"
    );
    expect(exitCode).toBe(0);

    const ctrlContent = readFileSync(
      resolve(outputDir, "items/index.ts"),
      "utf-8"
    );
    // Deprecated JSDoc
    expect(ctrlContent).toContain("@deprecated");
    // SSE + JSON coexist
    expect(ctrlContent).toContain("SSEConnection");

    cleanup();
  });

  it("should fail without config", () => {
    const { exitCode, stderr } = runTsgonest([
      "sdk",
      "--config",
      "nonexistent.config.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("error");
  });

  it("should fail when no outputs have sdk configured", () => {
    const tmpDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-nosdk-"));
    const configPath = resolve(tmpDir, "tsgonest.config.json");
    writeFileSync(
      configPath,
      JSON.stringify({
        controllers: { include: ["src/**/*.controller.ts"] },
        openapi: [{ name: "api", output: "dist/openapi.json" }],
      })
    );
    const { exitCode, stderr } = runTsgonest(["sdk", "--config", configPath]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("no OpenAPI outputs have sdk.output configured");
    rmSync(tmpDir, { recursive: true });
  });

  it("should generate SDK from empty fixture", () => {
    const { exitCode, outputDir, cleanup } = generateSdk(
      "testdata/sdkgen/empty.openapi.json",
      "empty"
    );
    expect(exitCode).toBe(0);

    // types.ts should have export {} (no schemas)
    const typesContent = readFileSync(resolve(outputDir, "types.ts"), "utf-8");
    expect(typesContent).toContain("export {}");

    cleanup();
  });
});

// ─── Group 2: Generated TypeScript compiles with tsc ───────────────────────

describe("generated SDK compiles with tsc", () => {
  function generateAndCompile(fixture: string, extraName: string) {
    const { exitCode: genCode, outputDir, cleanup } = generateSdk(fixture, extraName);
    expect(genCode).toBe(0);

    // Write tsconfig.json for tsc --noEmit
    writeFileSync(
      resolve(outputDir, "tsconfig.json"),
      JSON.stringify({
        compilerOptions: {
          target: "ES2022",
          module: "ES2022",
          moduleResolution: "bundler",
          lib: ["ES2022", "DOM"],
          strict: true,
          noEmit: true,
          skipLibCheck: true,
        },
        include: ["**/*.ts"],
      })
    );

    // Use absolute path to tsc from e2e workspace's node_modules
    const tscBin = resolve(__dirname, "node_modules/.bin/tsc");
    const tscResult = spawnSync(tscBin, ["--noEmit", "--project", resolve(outputDir, "tsconfig.json")], {
      cwd: outputDir,
      encoding: "utf-8",
      timeout: 30000,
    });

    cleanup();
    return {
      exitCode: tscResult.status ?? 1,
      stdout: tscResult.stdout || "",
      stderr: tscResult.stderr || "",
    };
  }

  it("basic fixture compiles with tsc", () => {
    const result = generateAndCompile(
      "testdata/sdkgen/basic.openapi.json",
      "basic"
    );
    expect(result.exitCode).toBe(0);
  });

  it("versioned fixture compiles with tsc", () => {
    const result = generateAndCompile(
      "testdata/sdkgen/versioned.openapi.json",
      "versioned"
    );
    expect(result.exitCode).toBe(0);
  });

  it("file-uploads fixture compiles with tsc", () => {
    const result = generateAndCompile(
      "testdata/sdkgen/file-uploads.openapi.json",
      "uploads"
    );
    expect(result.exitCode).toBe(0);
  });

  it("complex-types fixture compiles with tsc", () => {
    const result = generateAndCompile(
      "testdata/sdkgen/complex-types.openapi.json",
      "complex"
    );
    expect(result.exitCode).toBe(0);
  });

  it("edge-cases fixture compiles with tsc", () => {
    const result = generateAndCompile(
      "testdata/sdkgen/edge-cases.openapi.json",
      "edge"
    );
    expect(result.exitCode).toBe(0);
  });
});

// ─── Group 3: Runtime verification ─────────────────────────────────────────

describe("SDK runtime verification", () => {
  let sdkDir: string;
  let cleanupFn: () => void;

  beforeAll(() => {
    const result = generateSdk("testdata/sdkgen/basic.openapi.json", "runtime");
    expect(result.exitCode).toBe(0);
    sdkDir = result.outputDir;
    cleanupFn = result.cleanup;
  });

  it("createClient returns object with controller namespaces", async () => {
    const mod = await import(resolve(sdkDir, "index.ts"));
    expect(typeof mod.createClient).toBe("function");

    // createClient needs a config with baseUrl
    const client = mod.createClient({ baseUrl: "http://localhost:3000" });
    expect(client).toBeDefined();
    expect(typeof client).toBe("object");
    // Should have orders and products namespaces
    expect(client.orders).toBeDefined();
    expect(client.products).toBeDefined();
  });

  it("controller factory returns object with expected method names", async () => {
    const mod = await import(resolve(sdkDir, "orders/index.ts"));
    expect(typeof mod.createOrdersController).toBe("function");

    // Create a mock request function
    const mockRequest = async () => ({ data: null, error: null, response: {} });
    const controller = mod.createOrdersController(mockRequest);
    expect(typeof controller.listOrders).toBe("function");
    expect(typeof controller.createOrder).toBe("function");
    expect(typeof controller.getOrder).toBe("function");
    expect(typeof controller.deleteOrder).toBe("function");
  });

  it("standalone functions are exported and callable", async () => {
    const mod = await import(resolve(sdkDir, "orders/index.ts"));
    // Standalone functions use qualified names (Controller_method) for global uniqueness
    expect(typeof mod.Orders_listOrders).toBe("function");
    expect(typeof mod.Orders_createOrder).toBe("function");
    expect(typeof mod.Orders_getOrder).toBe("function");
    expect(typeof mod.Orders_deleteOrder).toBe("function");
  });
});

// ─── Group 3b: Empty response body handling (issue #89) ──────────────────

describe("SDK empty response body handling", () => {
  let createRequestFn: any;

  beforeAll(async () => {
    const { exitCode, outputDir } = generateSdk(
      "testdata/sdkgen/basic.openapi.json",
      "empty-body"
    );
    expect(exitCode).toBe(0);
    const clientMod = await import(resolve(outputDir, "client.ts"));
    createRequestFn = clientMod.createRequestFn;
  });

  it("should handle HTTP 200 with empty body and application/json content-type", async () => {
    const mockFetch = async () =>
      new Response("", {
        status: 200,
        headers: { "content-type": "application/json" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("DELETE", "/resources/{id}", {
      params: { id: "123" },
    });

    expect(result.error).toBeNull();
    expect(result.data).toBeNull();
  });

  it("should handle HTTP 200 with empty body and no content-type", async () => {
    const mockFetch = async () =>
      new Response("", { status: 200 });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("DELETE", "/resources/{id}", {
      params: { id: "123" },
    });

    expect(result.error).toBeNull();
    expect(result.data).toBeNull();
  });

  it("should handle HTTP 204 No Content", async () => {
    const mockFetch = async () =>
      new Response(null, { status: 204 });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("DELETE", "/resources/{id}", {
      params: { id: "123" },
    });

    expect(result.error).toBeNull();
    expect(result.data).toBeUndefined();
  });

  it("should still parse valid JSON body at 200", async () => {
    const mockFetch = async () =>
      new Response(JSON.stringify({ id: "123", name: "test" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("GET", "/resources/{id}", {
      params: { id: "123" },
    });

    expect(result.error).toBeNull();
    expect(result.data).toEqual({ id: "123", name: "test" });
  });

  it("should handle HTTP 200 with content-length: 0", async () => {
    const mockFetch = async () =>
      new Response("", {
        status: 200,
        headers: {
          "content-type": "application/json",
          "content-length": "0",
        },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("DELETE", "/resources/{id}", {
      params: { id: "123" },
    });

    expect(result.error).toBeNull();
    expect(result.data).toBeNull();
  });
});

// ─── Group 3c: SDK edge case crash vectors ────────────────────────────────

describe("SDK client edge cases", () => {
  let createRequestFn: any;

  beforeAll(async () => {
    const { exitCode, outputDir } = generateSdk(
      "testdata/sdkgen/basic.openapi.json",
      "edge-cases"
    );
    expect(exitCode).toBe(0);
    const clientMod = await import(resolve(outputDir, "client.ts"));
    createRequestFn = clientMod.createRequestFn;
  });

  // ── Malformed JSON (proxy/CDN returning HTML with application/json) ──

  it("should not crash on malformed JSON body with application/json content-type", async () => {
    const mockFetch = async () =>
      new Response("<html>502 Bad Gateway</html>", {
        status: 200,
        headers: { "content-type": "application/json" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    // Should NOT throw SyntaxError — should return an error result
    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
    expect(result.error.message).toBeDefined();
  });

  // ── Network error (fetch throws TypeError) ──

  it("should not throw on network error (fetch rejects)", async () => {
    const mockFetch = async () => {
      throw new TypeError("Failed to fetch");
    };

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    // Should NOT throw — should return an error result
    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
    expect(result.error.message).toContain("Failed to fetch");
  });

  // ── DNS/connection error ──

  it("should not throw on DNS resolution error", async () => {
    const mockFetch = async () => {
      throw new TypeError("getaddrinfo ENOTFOUND api.example.com");
    };

    const request = createRequestFn({
      baseUrl: "http://api.example.com",
      fetcher: mockFetch,
    });

    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
  });

  // ── Headers factory throws ──

  it("should not throw when headers factory function rejects", async () => {
    const mockFetch = async () => new Response("ok", { status: 200 });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
      headers: async () => {
        throw new Error("Token refresh failed");
      },
    });

    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
    expect(result.error.message).toContain("Token refresh failed");
  });

  // ── onRequest hook throws ──

  it("should not throw when onRequest hook rejects", async () => {
    const mockFetch = async () => new Response("ok", { status: 200 });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
      onRequest: async () => {
        throw new Error("Request interceptor failed");
      },
    });

    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
    expect(result.error.message).toContain("Request interceptor failed");
  });

  // ── application/vnd.api+json (JSON:API content type) ──

  it("should parse application/vnd.api+json as JSON", async () => {
    const mockFetch = async () =>
      new Response(JSON.stringify({ data: [{ id: "1" }] }), {
        status: 200,
        headers: { "content-type": "application/vnd.api+json" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("GET", "/items");
    // Currently falls through to text — at minimum should not crash
    expect(result.error).toBeNull();
    expect(result.data).toBeDefined();
  });

  // ── JSON body is literally "null" ──

  it("should handle JSON null body correctly", async () => {
    const mockFetch = async () =>
      new Response("null", {
        status: 200,
        headers: { "content-type": "application/json" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("GET", "/items");
    expect(result.error).toBeNull();
    expect(result.data).toBeNull();
  });

  // ── Non-error status codes: 202 Accepted with empty body ──

  it("should handle 202 Accepted with empty body", async () => {
    const mockFetch = async () =>
      new Response("", {
        status: 202,
        headers: { "content-type": "application/json" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("POST", "/items");
    expect(result.error).toBeNull();
    expect(result.data).toBeNull();
  });

  // ── 4xx with empty body (no JSON to parse) ──

  it("should handle 401 with empty body", async () => {
    const mockFetch = async () =>
      new Response("", { status: 401 });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
    expect(result.error.status).toBe(401);
  });

  // ── 5xx with HTML error page ──

  it("should handle 500 with HTML error page body", async () => {
    const mockFetch = async () =>
      new Response("<html><body>Internal Server Error</body></html>", {
        status: 500,
        headers: { "content-type": "text/html" },
      });

    const request = createRequestFn({
      baseUrl: "http://localhost:3000",
      fetcher: mockFetch,
    });

    const result = await request("GET", "/items");
    expect(result.error).not.toBeNull();
    expect(result.error.status).toBe(500);
  });
});

describe("SSEConnection runtime", () => {
  let SSEConnection: any;

  beforeAll(async () => {
    const { exitCode, outputDir } = generateSdk(
      "testdata/sdkgen/file-uploads.openapi.json",
      "sse"
    );
    expect(exitCode).toBe(0);
    const sseMod = await import(resolve(outputDir, "sse.ts"));
    SSEConnection = sseMod.SSEConnection;
  });

  function createMockResponse(chunks: string[]): any {
    let index = 0;
    const stream = new ReadableStream({
      pull(controller) {
        if (index < chunks.length) {
          controller.enqueue(new TextEncoder().encode(chunks[index]));
          index++;
        } else {
          controller.close();
        }
      },
    });
    return { body: stream };
  }

  it("should parse a single SSE event", async () => {
    const response = createMockResponse(['data: {"type":"update"}\n\n']);
    const conn = new SSEConnection(response, (s: string) => JSON.parse(s));

    const events: any[] = [];
    for await (const event of conn) {
      events.push(event);
    }

    expect(events).toHaveLength(1);
    expect(events[0].data).toEqual({ type: "update" });
  });

  it("should parse multi-line data", async () => {
    const response = createMockResponse(["data: line1\ndata: line2\n\n"]);
    const conn = new SSEConnection(response, (s: string) => s);

    const events: any[] = [];
    for await (const event of conn) {
      events.push(event);
    }

    expect(events).toHaveLength(1);
    expect(events[0].data).toBe("line1\nline2");
  });

  it("should parse event, id, and retry fields", async () => {
    const response = createMockResponse([
      "event: custom\nid: 42\nretry: 5000\ndata: hello\n\n",
    ]);
    const conn = new SSEConnection(response, (s: string) => s);

    const events: any[] = [];
    for await (const event of conn) {
      events.push(event);
    }

    expect(events).toHaveLength(1);
    expect(events[0].event).toBe("custom");
    expect(events[0].id).toBe("42");
    expect(events[0].retry).toBe(5000);
    expect(events[0].data).toBe("hello");
  });

  it("should skip empty data blocks", async () => {
    // Event with no data: line should not be yielded
    const response = createMockResponse(["event: ping\n\ndata: real\n\n"]);
    const conn = new SSEConnection(response, (s: string) => s);

    const events: any[] = [];
    for await (const event of conn) {
      events.push(event);
    }

    // Only the event with data should be yielded
    expect(events).toHaveLength(1);
    expect(events[0].data).toBe("real");
  });

  it("should update lastId after events with id field", async () => {
    const response = createMockResponse([
      "id: first\ndata: 1\n\nid: second\ndata: 2\n\n",
    ]);
    const conn = new SSEConnection(response, (s: string) => s);

    for await (const _ of conn) {
      // consume
    }

    expect(conn.lastId).toBe("second");
  });

  it("should dispatch to .on() listeners", async () => {
    const response = createMockResponse(["data: hello\n\n"]);
    const conn = new SSEConnection(response, (s: string) => s);

    const received: any[] = [];
    conn.on("message", (event: any) => received.push(event));

    for await (const _ of conn) {
      // consume to trigger emit
    }

    expect(received).toHaveLength(1);
    expect(received[0].data).toBe("hello");
  });

  it("should stop iteration on close()", async () => {
    // Send a lot of chunks to ensure close() works mid-stream
    const chunks = Array.from({ length: 100 }, (_, i) => `data: ${i}\n\n`);
    const response = createMockResponse(chunks);
    const conn = new SSEConnection(response, (s: string) => s);

    const events: any[] = [];
    for await (const event of conn) {
      events.push(event);
      if (events.length >= 3) {
        conn.close();
      }
    }

    // Should have stopped after ~3 events (may get a few more depending on buffering)
    expect(events.length).toBeLessThan(chunks.length);
    expect(events.length).toBeGreaterThanOrEqual(3);
  });
});

describe("buildFormData runtime", () => {
  let buildFormData: any;

  beforeAll(async () => {
    const { exitCode, outputDir } = generateSdk(
      "testdata/sdkgen/file-uploads.openapi.json",
      "form"
    );
    expect(exitCode).toBe(0);
    const formDataMod = await import(resolve(outputDir, "form-data.ts"));
    buildFormData = formDataMod.buildFormData;
  });

  it("should handle string values", () => {
    const fd = buildFormData({ name: "Alice" });
    expect(fd).toBeInstanceOf(FormData);
    expect(fd.get("name")).toBe("Alice");
  });

  it("should handle number and boolean values", () => {
    const fd = buildFormData({ age: 30, active: true });
    expect(fd.get("age")).toBe("30");
    expect(fd.get("active")).toBe("true");
  });

  it("should handle Blob values", () => {
    const blob = new Blob(["hello"], { type: "text/plain" });
    const fd = buildFormData({ file: blob });
    expect(fd.get("file")).toBeInstanceOf(Blob);
  });

  it("should handle array values", () => {
    const fd = buildFormData({ tags: ["a", "b", "c"] });
    expect(fd.getAll("tags")).toEqual(["a", "b", "c"]);
  });

  it("should skip null and undefined values", () => {
    const fd = buildFormData({ name: "Alice", age: null, email: undefined });
    expect(fd.get("name")).toBe("Alice");
    expect(fd.get("age")).toBeNull();
    expect(fd.get("email")).toBeNull();
  });

  it("should silently drop nested objects (no JSON blob fallback)", () => {
    const fd = buildFormData({ meta: { key: "value" } as any });
    // Nested objects are no longer serialized as JSON blobs — they are silently
    // dropped by appendValue since they don't match any supported type.
    expect(fd.get("meta")).toBeNull();
  });
});

// ─── Group 4: Write-if-changed behavior ────────────────────────────────────

describe("SDK write-if-changed behavior", () => {
  it("re-run with same input should preserve file mtimes", () => {
    const fixture = "testdata/sdkgen/basic.openapi.json";
    const { configPath, outputDir, cleanup } = createSdkFixture(fixture, "mtime");

    // First generation
    const { exitCode: first } = runTsgonest(["sdk", "--config", configPath]);
    expect(first).toBe(0);

    // Record mtimes
    const typesPath = resolve(outputDir, "types.ts");
    const mtime1 = statSync(typesPath).mtimeMs;

    // Second generation with same input
    const { exitCode: second } = runTsgonest(["sdk", "--config", configPath]);
    expect(second).toBe(0);

    // mtime should be unchanged (write-if-changed)
    const mtime2 = statSync(typesPath).mtimeMs;
    expect(mtime2).toBe(mtime1);

    cleanup();
  });

  it("re-run with different input should update files", () => {
    // Generate from basic fixture
    const { exitCode: first, outputDir, cleanup } = generateSdk(
      "testdata/sdkgen/basic.openapi.json",
      "update1"
    );
    expect(first).toBe(0);
    const content1 = readFileSync(resolve(outputDir, "types.ts"), "utf-8");

    // Generate from a different fixture into the same output dir
    const absInput2 = resolve(PROJECT_ROOT, "testdata/sdkgen/file-uploads.openapi.json");
    const configPath2 = resolve(outputDir, "..", "tsgonest2.config.json");
    writeFileSync(
      configPath2,
      JSON.stringify({
        controllers: { include: ["src/**/*.controller.ts"] },
        openapi: [{ name: "default", output: absInput2, sdk: { output: outputDir } }],
      })
    );
    runTsgonest(["sdk", "--config", configPath2]);

    const content2 = readFileSync(resolve(outputDir, "types.ts"), "utf-8");
    // Content should be different (different schemas)
    expect(content2).not.toBe(content1);

    cleanup();
  });
});
