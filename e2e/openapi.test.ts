import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest openapi multi-output", () => {
  const distDir = resolve(FIXTURES_DIR, "multi-output/dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should generate both specs from multi-output config", () => {
    const { exitCode, stderr } = runTsgonest([
      "openapi",
      "--project",
      "testdata/multi-output/tsconfig.json",
      "--config",
      "testdata/multi-output/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("analyzed 2 controller(s)");

    expect(existsSync(resolve(distDir, "public-api.json"))).toBe(true);
    expect(existsSync(resolve(distDir, "internal-api.json"))).toBe(true);
  });

  it("public spec should only contain public controller paths and schemas", () => {
    const doc = JSON.parse(
      readFileSync(resolve(distDir, "public-api.json"), "utf-8")
    );
    expect(doc.info.title).toBe("Public API");
    expect(Object.keys(doc.paths)).toEqual(["/public"]);
    expect(Object.keys(doc.components.schemas)).toEqual(["PublicDto"]);
    // Should NOT contain internal paths
    expect(doc.paths["/internal"]).toBeUndefined();
  });

  it("internal spec should only contain internal controller paths and schemas", () => {
    const doc = JSON.parse(
      readFileSync(resolve(distDir, "internal-api.json"), "utf-8")
    );
    expect(doc.info.title).toBe("Internal API");
    expect(Object.keys(doc.paths)).toEqual(["/internal"]);
    expect(Object.keys(doc.components.schemas)).toEqual(["InternalDto"]);
    expect(doc.paths["/public"]).toBeUndefined();
  });
});

describe("tsgonest openapi --name", () => {
  const distDir = resolve(FIXTURES_DIR, "multi-output/dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should generate only the named output", () => {
    const { exitCode, stderr } = runTsgonest([
      "openapi",
      "--project",
      "testdata/multi-output/tsconfig.json",
      "--config",
      "testdata/multi-output/tsgonest.config.json",
      "--name",
      "public",
    ]);
    expect(exitCode).toBe(0);

    // Only public should be generated
    expect(existsSync(resolve(distDir, "public-api.json"))).toBe(true);
    expect(existsSync(resolve(distDir, "internal-api.json"))).toBe(false);
  });

  it("should fail for nonexistent name", () => {
    const { exitCode, stderr } = runTsgonest([
      "openapi",
      "--project",
      "testdata/multi-output/tsconfig.json",
      "--config",
      "testdata/multi-output/tsgonest.config.json",
      "--name",
      "nonexistent",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("no OpenAPI output named");
  });
});
