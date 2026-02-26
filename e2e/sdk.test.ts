import { describe, it, expect, beforeAll } from "vitest";
import { spawnSync } from "child_process";
import {
  existsSync,
  readFileSync,
  rmSync,
  mkdtempSync,
  writeFileSync,
  statSync,
} from "fs";
import { tmpdir } from "os";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

// ─── Group 1: tsgonest sdk CLI basics ──────────────────────────────────────

describe("tsgonest sdk CLI", () => {
  it("should generate SDK from basic fixture", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-basic-"));
    const { exitCode, stdout } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/basic.openapi.json",
      "--output",
      outputDir,
    ]);
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

    rmSync(outputDir, { recursive: true });
  });

  it("should generate SDK from versioned fixture", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-ver-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/versioned.openapi.json",
      "--output",
      outputDir,
    ]);
    expect(exitCode).toBe(0);

    expect(existsSync(resolve(outputDir, "health/index.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "v1/orders/index.ts"))).toBe(true);
    expect(existsSync(resolve(outputDir, "v2/orders/index.ts"))).toBe(true);

    rmSync(outputDir, { recursive: true });
  });

  it("should generate SDK with SSE and file uploads", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-fu-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/file-uploads.openapi.json",
      "--output",
      outputDir,
    ]);
    expect(exitCode).toBe(0);

    const sseContent = readFileSync(resolve(outputDir, "sse.ts"), "utf-8");
    expect(sseContent).toContain("SSEConnection");
    const formDataContent = readFileSync(resolve(outputDir, "form-data.ts"), "utf-8");
    expect(formDataContent).toContain("buildFormData");

    rmSync(outputDir, { recursive: true });
  });

  it("should generate SDK from edge-cases fixture", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-edge-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/edge-cases.openapi.json",
      "--output",
      outputDir,
    ]);
    expect(exitCode).toBe(0);

    const ctrlContent = readFileSync(
      resolve(outputDir, "items/index.ts"),
      "utf-8"
    );
    // Deprecated JSDoc
    expect(ctrlContent).toContain("@deprecated");
    // SSE + JSON coexist
    expect(ctrlContent).toContain("SSEConnection");

    rmSync(outputDir, { recursive: true });
  });

  it("should fail with missing --input", () => {
    const { exitCode, stderr } = runTsgonest(["sdk"]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("--input");
  });

  it("should fail with nonexistent input file", () => {
    const { exitCode, stderr } = runTsgonest([
      "sdk",
      "--input",
      "nonexistent.json",
      "--output",
      "/tmp/sdk-out",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("error");
  });

  it("should generate SDK from empty fixture", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-empty-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/empty.openapi.json",
      "--output",
      outputDir,
    ]);
    expect(exitCode).toBe(0);

    // types.ts should have export {} (no schemas)
    const typesContent = readFileSync(resolve(outputDir, "types.ts"), "utf-8");
    expect(typesContent).toContain("export {}");

    rmSync(outputDir, { recursive: true });
  });
});

// ─── Group 2: Generated TypeScript compiles with tsc ───────────────────────

describe("generated SDK compiles with tsc", () => {
  function generateAndCompile(fixture: string, extraName: string) {
    const outputDir = mkdtempSync(resolve(tmpdir(), `tsgonest-tsc-${extraName}-`));
    const { exitCode: genCode } = runTsgonest([
      "sdk",
      "--input",
      fixture,
      "--output",
      outputDir,
    ]);
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

    rmSync(outputDir, { recursive: true });
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

  beforeAll(() => {
    sdkDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-runtime-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/basic.openapi.json",
      "--output",
      sdkDir,
    ]);
    expect(exitCode).toBe(0);
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

describe("SSEConnection runtime", () => {
  let SSEConnection: any;

  beforeAll(async () => {
    const sdkDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-sse-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/file-uploads.openapi.json",
      "--output",
      sdkDir,
    ]);
    expect(exitCode).toBe(0);
    const sseMod = await import(resolve(sdkDir, "sse.ts"));
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
    const sdkDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-form-"));
    const { exitCode } = runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/file-uploads.openapi.json",
      "--output",
      sdkDir,
    ]);
    expect(exitCode).toBe(0);
    const formDataMod = await import(resolve(sdkDir, "form-data.ts"));
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

  it("should handle nested objects as JSON blobs", () => {
    const fd = buildFormData({ meta: { key: "value" } });
    const blob = fd.get("meta");
    expect(blob).toBeInstanceOf(Blob);
  });
});

// ─── Group 4: Write-if-changed behavior ────────────────────────────────────

describe("SDK write-if-changed behavior", () => {
  it("re-run with same input should preserve file mtimes", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-mtime-"));
    const fixture = "testdata/sdkgen/basic.openapi.json";

    // First generation
    const { exitCode: first } = runTsgonest([
      "sdk",
      "--input",
      fixture,
      "--output",
      outputDir,
    ]);
    expect(first).toBe(0);

    // Record mtimes
    const typesPath = resolve(outputDir, "types.ts");
    const mtime1 = statSync(typesPath).mtimeMs;

    // Second generation with same input
    const { exitCode: second } = runTsgonest([
      "sdk",
      "--input",
      fixture,
      "--output",
      outputDir,
    ]);
    expect(second).toBe(0);

    // mtime should be unchanged (write-if-changed)
    const mtime2 = statSync(typesPath).mtimeMs;
    expect(mtime2).toBe(mtime1);

    rmSync(outputDir, { recursive: true });
  });

  it("re-run with different input should update files", () => {
    const outputDir = mkdtempSync(resolve(tmpdir(), "tsgonest-sdk-update-"));

    // Generate from basic fixture
    runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/basic.openapi.json",
      "--output",
      outputDir,
    ]);

    const content1 = readFileSync(resolve(outputDir, "types.ts"), "utf-8");

    // Generate from a different fixture into the same dir
    runTsgonest([
      "sdk",
      "--input",
      "testdata/sdkgen/file-uploads.openapi.json",
      "--output",
      outputDir,
    ]);

    const content2 = readFileSync(resolve(outputDir, "types.ts"), "utf-8");
    // Content should be different (different schemas)
    expect(content2).not.toBe(content1);

    rmSync(outputDir, { recursive: true });
  });
});
