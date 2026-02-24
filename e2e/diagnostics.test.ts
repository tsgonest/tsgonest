import { describe, it, expect } from "vitest";
import { existsSync, readdirSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest diagnostics and exit codes", () => {
  it("should exit 1 on type errors and still emit JS", () => {
    const distDir = resolve(FIXTURES_DIR, "errors-type/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-type/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);

    expect(stderr).toContain("TS2345");
    expect(stderr).toContain("TS2741");
    expect(stderr).toContain("error");

    expect(stderr).toContain("emitted");
    expect(existsSync(resolve(distDir, "bad-types.js"))).toBe(true);
  });

  it("should exit 1 on syntax errors and still emit JS", () => {
    const distDir = resolve(FIXTURES_DIR, "errors-syntax/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-syntax/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);

    expect(stderr).toContain("TS1005");
    expect(stderr).toContain("error");

    expect(stderr).toContain("emitted");
  });

  it("should exit 2 with noEmitOnError and NOT emit JS", () => {
    const distDir = resolve(FIXTURES_DIR, "errors-noemit/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-noemit/tsconfig.json",
    ]);
    expect(exitCode).toBe(2);

    expect(stderr).toContain("TS2345");
    expect(stderr).toContain("error");

    expect(stderr).toContain("no files emitted");
    expect(stderr).toContain("noEmitOnError");

    if (existsSync(distDir)) {
      const files = readdirSync(distDir);
      const jsFiles = files.filter((f) => f.endsWith(".js"));
      expect(jsFiles).toHaveLength(0);
    }
  });

  it("should exit 0 with --no-check even when type errors exist", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-type/tsconfig.json",
      "--no-check",
    ]);
    expect(exitCode).toBe(0);

    expect(stderr).not.toContain("TS2345");
    expect(stderr).not.toContain("TS2741");

    expect(stderr).toContain("emitted");
  });

  it("should still report syntax errors with --no-check", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-syntax/tsconfig.json",
      "--no-check",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("TS1005");
  });

  it("diagnostic output should include file name and position", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/errors-type/tsconfig.json",
    ]);

    expect(stderr).toContain("bad-types.ts");
    expect(stderr).toMatch(/bad-types\.ts[\(:]/);
  });
});
