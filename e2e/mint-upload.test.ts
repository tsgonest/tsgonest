import { spawn, type ChildProcess } from "child_process";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { describe, it, expect, beforeAll } from "vitest";
import { FIXTURES_DIR, runTsgonest } from "./helpers";

const FIXTURE_DIR = resolve(FIXTURES_DIR, "mint-upload");
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

// Build a minimal PNG (8 bytes signature + IHDR). Enough to satisfy
// content-type sniffing in this test — we never decode it.
function tinyPng(size: number): Uint8Array {
  const out = new Uint8Array(size);
  out.set([137, 80, 78, 71, 13, 10, 26, 10], 0);
  // Fill remainder with deterministic bytes.
  for (let i = 8; i < size; i++) out[i] = i & 0xff;
  return out;
}

describe("Mint Phase 9: buffered file uploads", () => {
  beforeAll(() => {
    if (existsSync(FIXTURE_DIST)) {
      rmSync(FIXTURE_DIST, { recursive: true });
    }
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/mint-upload/tsconfig.json",
      "--config",
      "testdata/mint-upload/tsgonest.config.json",
    ]);
    if (exitCode !== 0) {
      throw new Error(`tsgonest build failed: ${stderr}`);
    }
  });

  it("emits the multipart-aware register wrapper", () => {
    const register = readFileSync(
      resolve(FIXTURE_DIST, "upload.controller.UploadController.tsgonest.js"),
      "utf-8",
    );
    expect(register).toContain("event.body.formData()");
    expect(register).toContain("matchMimeType");
    expect(register).toContain("parseMultipartStream");
    expect(register).toContain("__iter.setLimit");
  });

  it("OpenAPI emits multipart/form-data for the avatar route", () => {
    const openapi = JSON.parse(
      readFileSync(resolve(FIXTURE_DIST, "openapi.json"), "utf-8"),
    );
    const avatarOp = openapi.paths["/uploads/avatar"].post;
    expect(avatarOp.requestBody).toBeDefined();
    expect(avatarOp.requestBody.content["multipart/form-data"]).toBeDefined();
    expect(
      avatarOp.requestBody.content["multipart/form-data"].schema,
    ).toBeDefined();
  });

  it("POST /uploads/avatar with a valid PNG → 200 + JSON response", async () => {
    const server = await startBunServer();
    try {
      const form = new FormData();
      form.append("title", "my avatar");
      form.append(
        "image",
        new Blob([tinyPng(512)], { type: "image/png" }),
        "avatar.png",
      );
      const res = await fetch(`${server.url}/uploads/avatar`, {
        method: "POST",
        body: form,
      });
      expect(res.status).toBe(200);
      const body = (await res.json()) as {
        ok: boolean;
        name: string;
        size: number;
        type: string;
      };
      expect(body.ok).toBe(true);
      expect(body.name).toBe("my avatar");
      expect(body.type).toBe("image/png");
      expect(body.size).toBe(512);
    } finally {
      await server.stop();
    }
  });

  it("POST /uploads/avatar with oversize file → 400 problem+json", async () => {
    const server = await startBunServer();
    try {
      const form = new FormData();
      form.append("title", "big");
      form.append(
        "image",
        new Blob([tinyPng(20_000)], { type: "image/png" }),
        "big.png",
      );
      const res = await fetch(`${server.url}/uploads/avatar`, {
        method: "POST",
        body: form,
      });
      expect(res.status).toBe(400);
      expect(res.headers.get("content-type")).toMatch(
        /application\/problem\+json/,
      );
      const body = (await res.json()) as {
        errors: Array<{ path: string; expected: string }>;
      };
      expect(
        body.errors.some((e) => e.path === "body.image" && /maxSize/.test(e.expected)),
      ).toBe(true);
    } finally {
      await server.stop();
    }
  });

  it("POST /uploads/avatar with wrong MIME → 400 problem+json", async () => {
    const server = await startBunServer();
    try {
      const form = new FormData();
      form.append("title", "txt");
      form.append(
        "image",
        new Blob(["hello"], { type: "text/plain" }),
        "note.txt",
      );
      const res = await fetch(`${server.url}/uploads/avatar`, {
        method: "POST",
        body: form,
      });
      expect(res.status).toBe(400);
      const body = (await res.json()) as {
        errors: Array<{ path: string; expected: string }>;
      };
      expect(
        body.errors.some((e) => e.path === "body.image" && /mimeTypes/.test(e.expected)),
      ).toBe(true);
    } finally {
      await server.stop();
    }
  });

  it("POST /uploads/avatar with empty title → 400 problem+json", async () => {
    const server = await startBunServer();
    try {
      const form = new FormData();
      form.append("title", "");
      form.append(
        "image",
        new Blob([tinyPng(256)], { type: "image/png" }),
        "a.png",
      );
      const res = await fetch(`${server.url}/uploads/avatar`, {
        method: "POST",
        body: form,
      });
      expect(res.status).toBe(400);
      const body = (await res.json()) as {
        errors: Array<{ path: string }>;
      };
      expect(body.errors.some((e) => e.path === "body.title")).toBe(true);
    } finally {
      await server.stop();
    }
  });
});

describe("Mint Phase 10: streaming file uploads", () => {
  it("POST /uploads/large with a streaming file → 200 with byte count", async () => {
    const server = await startBunServer();
    try {
      // Build a multipart body manually so we exercise the streaming path.
      const boundary = "X-STREAM-TEST-BOUNDARY";
      const enc = new TextEncoder();
      const payload = new Uint8Array(1024 * 256); // 256KB
      for (let i = 0; i < payload.length; i++) payload[i] = i & 0xff;

      const parts: Uint8Array[] = [];
      parts.push(enc.encode(`--${boundary}\r\n`));
      parts.push(
        enc.encode(
          'Content-Disposition: form-data; name="filename"\r\n\r\n',
        ),
      );
      parts.push(enc.encode("my-upload.bin"));
      parts.push(enc.encode(`\r\n--${boundary}\r\n`));
      parts.push(
        enc.encode(
          'Content-Disposition: form-data; name="file"; filename="payload.bin"\r\nContent-Type: application/octet-stream\r\n\r\n',
        ),
      );
      parts.push(payload);
      parts.push(enc.encode(`\r\n--${boundary}--\r\n`));

      let total = 0;
      for (const p of parts) total += p.length;
      const body = new Uint8Array(total);
      let off = 0;
      for (const p of parts) {
        body.set(p, off);
        off += p.length;
      }

      const res = await fetch(`${server.url}/uploads/large`, {
        method: "POST",
        headers: { "content-type": `multipart/form-data; boundary=${boundary}` },
        body,
      });
      expect(res.status).toBe(200);
      const json = (await res.json()) as {
        ok: boolean;
        filename: string;
        bytes: number;
      };
      expect(json.ok).toBe(true);
      expect(json.filename).toBe("my-upload.bin");
      expect(json.bytes).toBe(payload.length);
    } finally {
      await server.stop();
    }
  });

  it("POST /uploads/large with oversize stream → 400/413 problem+json", async () => {
    const server = await startBunServer();
    try {
      const boundary = "X-STREAM-LIMIT";
      const enc = new TextEncoder();
      // 6MB — exceeds the 5MB MaxSize set in the DTO.
      const payload = new Uint8Array(6 * 1024 * 1024);
      for (let i = 0; i < payload.length; i++) payload[i] = i & 0xff;

      const parts: Uint8Array[] = [];
      parts.push(enc.encode(`--${boundary}\r\n`));
      parts.push(
        enc.encode(
          'Content-Disposition: form-data; name="filename"\r\n\r\n',
        ),
      );
      parts.push(enc.encode("too-big.bin"));
      parts.push(enc.encode(`\r\n--${boundary}\r\n`));
      parts.push(
        enc.encode(
          'Content-Disposition: form-data; name="file"; filename="big.bin"\r\nContent-Type: application/octet-stream\r\n\r\n',
        ),
      );
      parts.push(payload);
      parts.push(enc.encode(`\r\n--${boundary}--\r\n`));

      let total = 0;
      for (const p of parts) total += p.length;
      const body = new Uint8Array(total);
      let off = 0;
      for (const p of parts) {
        body.set(p, off);
        off += p.length;
      }

      const res = await fetch(`${server.url}/uploads/large`, {
        method: "POST",
        headers: { "content-type": `multipart/form-data; boundary=${boundary}` },
        body,
      });
      // The byte-limit error surfaces as a 400 problem+json via the validation
      // error mapper. (Mint can be configured to map this to 413 if desired.)
      expect([400, 413]).toContain(res.status);
      const json = (await res.json()) as {
        errors: Array<{ path: string; expected: string }>;
      };
      expect(
        json.errors.some(
          (e) => e.path === "body.file" && /maxSize/.test(e.expected),
        ),
      ).toBe(true);
    } finally {
      await server.stop();
    }
  });
});
