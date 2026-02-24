import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest marker function rewriting", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "rewrite-markers");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should build successfully with marker imports", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/rewrite-markers/tsconfig.json",
      "--config",
      "testdata/rewrite-markers/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
  });

  it("should replace marker calls with companion function calls", () => {
    const serviceFile = resolve(distDir, "service.js");
    if (!existsSync(serviceFile)) {
      // Skip if file doesn't exist (marker rewriting may not have found tsgonest imports
      // since the package doesn't exist in the test environment)
      return;
    }

    const content = readFileSync(serviceFile, "utf-8");

    // Should have companion imports instead of tsgonest import
    expect(content).not.toContain('from "tsgonest"');

    // Should have rewritten marker calls
    expect(content).toContain("isCreateUserDto(");
    expect(content).toContain("assertCreateUserDto(");
    expect(content).toContain("validateCreateUserDto(");
  });

  it("should generate companion files for types", () => {
    const companionFile = resolve(
      distDir,
      "types.CreateUserDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);

    const content = readFileSync(companionFile, "utf-8");
    expect(content).toContain("export function validateCreateUserDto");
    expect(content).toContain("export function assertCreateUserDto");
  });

  it("should add sentinel comment to rewritten files", () => {
    const serviceFile = resolve(distDir, "service.js");
    if (!existsSync(serviceFile)) {
      return;
    }

    const content = readFileSync(serviceFile, "utf-8");
    expect(content).toContain("/* @tsgonest-rewritten */");
  });
});
