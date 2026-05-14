import { spawn, type ChildProcess } from "child_process";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { describe, it, expect, beforeAll } from "vitest";
import { FIXTURES_DIR, runTsgonest } from "./helpers";

const FIXTURE_DIR = resolve(FIXTURES_DIR, "mint-typed");
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

describe("Mint Phase 2: typed parameters and validated returns", () => {
  beforeAll(() => {
    if (existsSync(FIXTURE_DIST)) {
      rmSync(FIXTURE_DIST, { recursive: true });
    }
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/mint-typed/tsconfig.json",
      "--config",
      "testdata/mint-typed/tsgonest.config.json",
    ]);
    if (exitCode !== 0) {
      throw new Error(`tsgonest build failed: ${stderr}`);
    }
  });

  it("emits DTO companions and the register wrapper", () => {
    expect(
      existsSync(
        resolve(FIXTURE_DIST, "users.controller.UsersController.tsgonest.js"),
      ),
    ).toBe(true);
    expect(
      existsSync(resolve(FIXTURE_DIST, "users.dto.CreateUserDto.tsgonest.js")),
    ).toBe(true);
    expect(
      existsSync(resolve(FIXTURE_DIST, "users.dto.UserResponse.tsgonest.js")),
    ).toBe(true);
    expect(
      existsSync(resolve(FIXTURE_DIST, "users.dto.ListQuery.tsgonest.js")),
    ).toBe(true);

    const register = readFileSync(
      resolve(FIXTURE_DIST, "users.controller.UsersController.tsgonest.js"),
      "utf-8",
    );
    expect(register).toContain("assertCreateUserDto");
    expect(register).toContain("assertListQuery");
    expect(register).toContain("stringifyUserResponse");
    expect(register).toContain("serializeUserResponse");
  });

  it("POST /users with a valid body → 200 + JSON response", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(`${server.url}/users`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          name: "ada",
          email: "ada@example.com",
          age: 36,
        }),
      });
      expect(res.status).toBe(200);
      expect(res.headers.get("content-type")).toMatch(/application\/json/);
      const json = (await res.json()) as {
        id: string;
        name: string;
        email: string;
      };
      expect(json.id).toMatch(/^[0-9a-f-]{36}$/);
      expect(json.name).toBe("ada");
      expect(json.email).toBe("ada@example.com");
    } finally {
      await server.stop();
    }
  });

  it("POST /users with an invalid body → 400 problem+json with errors", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(`${server.url}/users`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "", email: "not-an-email", age: -1 }),
      });
      expect(res.status).toBe(400);
      expect(res.headers.get("content-type")).toMatch(
        /application\/problem\+json/,
      );
      const body = (await res.json()) as {
        type: string;
        title: string;
        status: number;
        detail: string;
        errors: unknown[];
      };
      expect(body.status).toBe(400);
      expect(body.title).toBe("Validation Failed");
      expect(Array.isArray(body.errors)).toBe(true);
      expect(body.errors.length).toBeGreaterThan(0);
    } finally {
      await server.stop();
    }
  });

  it("GET /users?limit=2 → 200 with array body coerced through ListQuery", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(`${server.url}/users?limit=2`);
      expect(res.status).toBe(200);
      expect(res.headers.get("content-type")).toMatch(/application\/json/);
      const arr = (await res.json()) as Array<{ id: string }>;
      expect(Array.isArray(arr)).toBe(true);
      expect(arr.length).toBe(2);
    } finally {
      await server.stop();
    }
  });

  it("GET /users?limit=999 → 400 problem+json (Maximum<100>)", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(`${server.url}/users?limit=999`);
      expect(res.status).toBe(400);
      expect(res.headers.get("content-type")).toMatch(
        /application\/problem\+json/,
      );
      const body = (await res.json()) as {
        errors: Array<{ path: string; expected: string }>;
      };
      expect(body.errors.some((e) => e.path.includes("limit"))).toBe(true);
    } finally {
      await server.stop();
    }
  });

  it("GET /users/:id extracts the path param", async () => {
    const server = await startBunServer();
    try {
      const res = await fetch(
        `${server.url}/users/11111111-1111-1111-1111-111111111111`,
      );
      expect(res.status).toBe(200);
      expect(res.headers.get("content-type")).toMatch(/application\/json/);
      const json = (await res.json()) as { id: string };
      // The handler echoes the path id when it's the right length; otherwise
      // falls back to a stable UUID. Either way, the param round-trips through
      // the router → event.params.id → assertion.
      expect(json.id).toBe("11111111-1111-1111-1111-111111111111");
    } finally {
      await server.stop();
    }
  });

  it("OpenAPI document includes request/response schemas", () => {
    const openapi = JSON.parse(
      readFileSync(resolve(FIXTURE_DIST, "openapi.json"), "utf-8"),
    );
    expect(openapi.openapi).toMatch(/^3\.[12](\.\d+)?$/);
    expect(openapi.paths["/users"]).toBeDefined();
    expect(openapi.paths["/users"].post).toBeDefined();
    expect(openapi.paths["/users"].get).toBeDefined();
    expect(openapi.paths["/users/{id}"]).toBeDefined();
    expect(openapi.components.schemas.CreateUserDto).toBeDefined();
    expect(openapi.components.schemas.UserResponse).toBeDefined();
    // ListQuery is flattened into the `parameters` array (one per property)
    // — OpenAPI doesn't model query DTOs as schemas, so it doesn't end up
    // under components.schemas.
    const listOp = openapi.paths["/users"].get;
    expect(Array.isArray(listOp.parameters)).toBe(true);
    expect(
      listOp.parameters.some(
        (p: { name: string; in: string }) =>
          p.name === "limit" && p.in === "query",
      ),
    ).toBe(true);
  });
});
