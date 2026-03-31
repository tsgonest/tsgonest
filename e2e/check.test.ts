import { describe, it, expect } from "vitest";
import { existsSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest check", () => {
  it("should exit 0 on a valid project", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/simple/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("no errors");
  });

  it("should exit 1 on type errors", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/errors-type/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("TS2345");
    expect(stderr).toContain("error");
  });

  it("should exit 1 on syntax errors", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/errors-syntax/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("TS1005");
  });

  it("should not emit any files", () => {
    const distDir = resolve(FIXTURES_DIR, "simple/dist");
    // Clean dist so we can verify check doesn't create it
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode } = runTsgonest([
      "check",
      "--project",
      "testdata/simple/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);

    // check should NOT emit any files
    expect(existsSync(resolve(distDir, "index.js"))).toBe(false);
  });

  it("should skip type errors with --no-check", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/errors-type/tsconfig.json",
      "--no-check",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("TS2345");
    expect(stderr).toContain("no errors");
  });

  it("should still report syntax errors with --no-check", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/errors-syntax/tsconfig.json",
      "--no-check",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("TS1005");
  });

  it("should load config and analyze controllers", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/simple/tsconfig.json",
      "--config",
      "testdata/simple/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("loaded config");
  });

  it("should exit 1 when rootDir is missing from tsconfig", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/missing-rootdir/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("rootDir");
  });

  it("should exit 1 when baseUrl is set in tsconfig", () => {
    const { exitCode, stderr } = runTsgonest([
      "check",
      "--project",
      "testdata/has-baseurl/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("baseUrl");
  });
});
