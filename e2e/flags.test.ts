import { describe, it, expect } from "vitest";
import {
  existsSync,
  rmSync,
  mkdtempSync,
  cpSync,
  writeFileSync,
} from "fs";
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

describe("deleteOutDir config option", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "delete-outdir");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");

  function clean() {
    if (existsSync(distDir)) rmSync(distDir, { recursive: true });
    if (existsSync(tsbuildinfo)) rmSync(tsbuildinfo);
  }

  it("should clean output directory on every build when deleteOutDir is true", () => {
    clean();

    // First build
    const first = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
      "--config",
      "testdata/delete-outdir/tsgonest.config.json",
    ]);
    expect(first.exitCode).toBe(0);
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);

    // Plant a stale file
    writeFileSync(resolve(distDir, "old-leftover.js"), "stale");
    expect(existsSync(resolve(distDir, "old-leftover.js"))).toBe(true);

    // Second build — deleteOutDir should remove the stale file
    const second = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
      "--config",
      "testdata/delete-outdir/tsgonest.config.json",
    ]);
    expect(second.exitCode).toBe(0);
    expect(second.stderr).toContain("cleaning output directory");
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "old-leftover.js"))).toBe(false);
  });

  it("should not clean when deleteOutDir is false", () => {
    clean();

    // Build without config (no deleteOutDir)
    const first = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
    ]);
    expect(first.exitCode).toBe(0);

    // Plant a stale file
    writeFileSync(resolve(distDir, "leftover.js"), "stale");

    // Build again without config — stale file should survive
    const second = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
    ]);
    expect(second.exitCode).toBe(0);
    expect(second.stderr).not.toContain("cleaning output directory");
    expect(existsSync(resolve(distDir, "leftover.js"))).toBe(true);

    // Cleanup
    clean();
  });

  it("--clean CLI flag should override even without config", () => {
    clean();

    // Build once
    runTsgonest(["--project", "testdata/delete-outdir/tsconfig.json"]);

    // Plant stale file
    writeFileSync(resolve(distDir, "stale.js"), "stale");

    // Build with --clean
    const result = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
      "--clean",
    ]);
    expect(result.exitCode).toBe(0);
    expect(result.stderr).toContain("cleaning output directory");
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "stale.js"))).toBe(false);

    clean();
  });

  it("--no-clean should suppress deleteOutDir (used by `tsgonest dev` watch rebuilds)", () => {
    clean();

    // Initial build with deleteOutDir — establishes a clean dist + .tsbuildinfo.
    const first = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
      "--config",
      "testdata/delete-outdir/tsgonest.config.json",
    ]);
    expect(first.exitCode).toBe(0);
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);

    // Plant a marker that deleteOutDir would normally wipe.
    writeFileSync(resolve(distDir, "watch-marker.js"), "marker");

    // Watch-style rebuild: --no-clean must suppress cfg.DeleteOutDir,
    // so the dist/ contents (including the marker) survive the rebuild.
    const second = runTsgonest([
      "--project",
      "testdata/delete-outdir/tsconfig.json",
      "--config",
      "testdata/delete-outdir/tsgonest.config.json",
      "--no-clean",
    ]);
    expect(second.exitCode).toBe(0);
    expect(second.stderr).not.toContain("cleaning output directory");
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "watch-marker.js"))).toBe(true);

    clean();
  });
});
