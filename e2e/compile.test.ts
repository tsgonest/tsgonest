import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest compilation", () => {
  beforeAll(() => {
    const distDir = resolve(FIXTURES_DIR, "simple/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should print version", () => {
    const { stdout, exitCode } = runTsgonest(["--version"]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("tsgonest");
  });

  it("should print help", () => {
    const { stdout, exitCode } = runTsgonest(["--help"]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("Usage:");
    expect(stdout).toContain("--project");
    expect(stdout).toContain("--config");
  });

  it("should compile simple TypeScript project", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/simple/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");

    // Verify output files exist
    expect(existsSync(resolve(FIXTURES_DIR, "simple/dist/index.js"))).toBe(
      true
    );
    expect(
      existsSync(resolve(FIXTURES_DIR, "simple/dist/index.d.ts"))
    ).toBe(true);
    expect(
      existsSync(resolve(FIXTURES_DIR, "simple/dist/index.js.map"))
    ).toBe(true);
  });

  it("should produce correct JavaScript output", () => {
    const jsFile = resolve(FIXTURES_DIR, "simple/dist/index.js");
    const content = readFileSync(jsFile, "utf-8");

    expect(content).toContain("export function greet");
    // Should not have TypeScript type annotations
    expect(content).not.toContain(": User");
    expect(content).not.toContain(": string");
  });

  it("should fail with missing tsconfig", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "nonexistent/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("could not find tsconfig");
  });

  it("should load config file when specified", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/simple/tsconfig.json",
      "--config",
      "testdata/simple/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("loaded config");
  });
});
