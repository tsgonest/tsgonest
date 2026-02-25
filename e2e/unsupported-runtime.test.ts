import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { FIXTURES_DIR, runTsgonest } from "./helpers";

describe("unsupported runtime-generated controllers", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "unsupported-runtime");
  const openapiFile = resolve(fixtureDir, "dist/openapi.json");
  let buildStderr = "";

  beforeAll(() => {
    const distDir = resolve(fixtureDir, "dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const cacheFile = resolve(fixtureDir, "tsconfig.tsgonest-cache");
    if (existsSync(cacheFile)) {
      rmSync(cacheFile);
    }

    const buildInfoFile = resolve(fixtureDir, "tsconfig.tsbuildinfo");
    if (existsSync(buildInfoFile)) {
      rmSync(buildInfoFile);
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/unsupported-runtime/tsconfig.json",
      "--config",
      "testdata/unsupported-runtime/tsgonest.config.json",
      "--clean",
    ]);

    buildStderr = stderr;
    expect(exitCode).toBe(0);
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("emits warnings for unsupported runtime and dynamic controller patterns", () => {
    expect(buildStderr).toContain("runtime-generated controller detected");
    expect(buildStderr).toContain("dynamic @Controller() path is not supported");
    expect(buildStderr).toContain("dynamic @Get() path argument is not supported");
    expect(buildStderr).toContain("dynamic @Sse() path argument is not supported");
    expect(buildStderr).toContain("excluded from OpenAPI");
  });

  it("excludes unsupported routes from OpenAPI while keeping static routes", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const staticRoute = doc.paths["/static/ok"]?.get;
    expect(staticRoute).toBeDefined();

    const operationIds = Object.values(doc.paths)
      .flatMap((pathItem: any) =>
        [
          pathItem?.get,
          pathItem?.post,
          pathItem?.put,
          pathItem?.patch,
          pathItem?.delete,
          pathItem?.head,
          pathItem?.options,
        ]
          .filter(Boolean)
          .map((op) => op.operationId)
      )
      .filter(Boolean);

    expect(operationIds).toContain("ok");
    expect(operationIds).not.toContain("skippedDynamicRoute");
    expect(operationIds).not.toContain("skippedDynamicSseRoute");
    expect(operationIds).not.toContain("skippedByDynamicControllerPath");
    expect(operationIds).not.toContain("skippedRuntimeControllerRoute");
  });
});
