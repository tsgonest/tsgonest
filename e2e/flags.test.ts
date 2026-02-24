import { describe, it, expect } from "vitest";
import { existsSync, rmSync, mkdtempSync, cpSync } from "fs";
import { tmpdir } from "os";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgo flag passthrough", () => {
  it("should accept --noEmit and not produce output files", () => {
    const tempDir = mkdtempSync(resolve(tmpdir(), "tsgonest-noemit-"));
    cpSync(resolve(FIXTURES_DIR, "branded"), tempDir, { recursive: true });

    const distDir = resolve(tempDir, "dist");
    if (existsSync(distDir)) rmSync(distDir, { recursive: true });

    const { exitCode } = runTsgonest(
      ["build", "--noEmit", "--project", "tsconfig.json"],
      { cwd: tempDir }
    );
    expect(exitCode).toBe(0);
    expect(existsSync(resolve(distDir, "index.js"))).toBe(false);

    rmSync(tempDir, { recursive: true });
  });

  it("should reject unknown flags with tsgo error message", () => {
    const { stderr, exitCode } = runTsgonest([
      "build",
      "--invalidFlagXyz",
      "--project",
      "testdata/branded/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("Unknown compiler option");
  });

  it("should reject invalid flag values with tsgo error message", () => {
    const { stderr, exitCode } = runTsgonest([
      "build",
      "--target",
      "es1999",
      "--project",
      "testdata/branded/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("--target");
  });

  it("should pass --strict through to compilation", () => {
    const { exitCode, stderr } = runTsgonest([
      "build",
      "--strict",
      "--noEmit",
      "--project",
      "testdata/branded/tsconfig.json",
    ]);
    expect(typeof exitCode).toBe("number");
    expect(stderr).not.toContain("Unknown compiler option");
  });

  it("should accept --target with valid value", () => {
    const { stderr, exitCode } = runTsgonest([
      "build",
      "--target",
      "es2022",
      "--noEmit",
      "--project",
      "testdata/branded/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("Unknown compiler option");
  });

  it("should mix tsgonest and tsgo flags without error", () => {
    const { stderr, exitCode } = runTsgonest([
      "build",
      "--clean",
      "--strict",
      "--noEmit",
      "--project",
      "testdata/branded/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("Unknown compiler option");
  });
});
