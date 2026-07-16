import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { execSync } from "child_process";
import type { ChildProcess } from "child_process";
import { buildIntegrationFixture, startServer } from "./integration-helpers";

let compiled = false;

function ensureCompiled() {
  if (!compiled) {
    const result = buildIntegrationFixture();
    if (result.exitCode !== 0) {
      throw new Error(
        `integration fixture build failed (exit ${result.exitCode}).\n` +
          `--- stdout ---\n${result.stdout}\n` +
          `--- stderr ---\n${result.stderr}\n`,
      );
    }
    compiled = true;
  }
}

/** Shared upload test cases — same assertions for every platform. */
function uploadTests(getUrl: () => string) {
  it("should upload a single file with validated metadata", async () => {
    const formData = new FormData();
    formData.append(
      "file",
      new Blob(["hello world"], { type: "text/plain" }),
      "test.txt"
    );
    formData.append("title", "My Upload");
    formData.append("category", "42");
    formData.append("isLegacy", "true");

    const res = await fetch(`${getUrl()}/upload/single`, {
      method: "POST",
      body: formData,
    });

    expect(res.status).toBe(201);
    const body = await res.json();
    expect(body.fileName).toBe("test.txt");
    expect(body.title).toBe("My Upload");
    // category should be coerced from string "42" to number 42
    expect(body.category).toBe(42);
    // isLegacy should be coerced from string "true" to boolean true (issue #213)
    expect(body.isLegacy).toBe(true);
  });

  it("should upload multiple files", async () => {
    const formData = new FormData();
    formData.append(
      "images",
      new Blob(["img1"], { type: "image/png" }),
      "photo1.png"
    );
    formData.append(
      "images",
      new Blob(["img2"], { type: "image/png" }),
      "photo2.png"
    );
    formData.append(
      "images",
      new Blob(["img3"], { type: "image/png" }),
      "photo3.png"
    );
    formData.append("albumName", "Vacation");

    const res = await fetch(`${getUrl()}/upload/gallery`, {
      method: "POST",
      body: formData,
    });

    expect(res.status).toBe(201);
    const body = await res.json();
    expect(body.fileCount).toBe(3);
    expect(body.albumName).toBe("Vacation");
  });

  it("should reject upload with invalid metadata (empty title)", async () => {
    const formData = new FormData();
    formData.append(
      "file",
      new Blob(["data"], { type: "text/plain" }),
      "file.txt"
    );
    formData.append("title", ""); // violates @minLength 1
    formData.append("category", "1");

    const res = await fetch(`${getUrl()}/upload/validate`, {
      method: "POST",
      body: formData,
    });

    // Should fail validation — tsgonest injects assert which throws TsgonestValidationError
    // NestJS default exception filter turns this into a 400 or 500
    expect(res.status).toBeGreaterThanOrEqual(400);
  });

  it("should reject upload with invalid category (0 violates @minimum 1)", async () => {
    const formData = new FormData();
    formData.append(
      "file",
      new Blob(["data"], { type: "text/plain" }),
      "file.txt"
    );
    formData.append("title", "Valid Title");
    formData.append("category", "0"); // violates @minimum 1

    const res = await fetch(`${getUrl()}/upload/validate`, {
      method: "POST",
      body: formData,
    });

    expect(res.status).toBeGreaterThanOrEqual(400);
  });
}

// ── Express ──────────────────────────────────────────────────────

describe("File upload / multipart form-data integration (Express)", () => {
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

  uploadTests(() => url);
});

// ── Fastify ──────────────────────────────────────────────────────

describe("File upload / multipart form-data integration (Fastify)", () => {
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

  uploadTests(() => url);
});

// ── Bun ──────────────────────────────────────────────────────────

function hasBun(): boolean {
  try {
    execSync("bun --version", { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const describeBun = hasBun() ? describe : describe.skip;

describeBun("File upload / multipart form-data integration (Bun)", () => {
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

  uploadTests(() => url);
});
