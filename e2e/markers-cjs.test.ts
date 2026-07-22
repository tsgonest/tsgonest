import { describe, it, expect, beforeAll } from "vitest";
import { execFileSync } from "child_process";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

// Regression for issues #224 and #225: under CommonJS emit, tsgo compiles
// `assert<Foo>(x)` to the interop form `(0, tsgonest_1.assert)(x)`, and a
// marker type argument that is a type alias to an object literal must still
// generate a companion.
describe("tsgonest marker rewriting under CommonJS emit", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "rewrite-markers-cjs");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/rewrite-markers-cjs/tsconfig.json",
      "--config",
      "testdata/rewrite-markers-cjs/tsgonest.config.json",
    ]);
    expect(stderr).toContain("emitted");
    expect(exitCode).toBe(0);
  });

  it("generates a companion for a type-alias marker argument", () => {
    expect(existsSync(resolve(distDir, "main.Foo.tsgonest.js"))).toBe(true);
  });

  it("rewrites the CJS interop call and strips the namespace require", () => {
    const content = readFileSync(resolve(distDir, "main.js"), "utf-8");
    expect(content).toContain("assertFoo(");
    expect(content).not.toContain("tsgonest_1");
    expect(content).not.toContain('require("tsgonest")');
  });

  it("validates at runtime when the emitted JS is executed", () => {
    const output = execFileSync("node", [resolve(distDir, "main.js")], {
      encoding: "utf-8",
    });
    expect(output).toContain('valid:{"b":"hi"}');
    expect(output).toContain("invalid:rejected");
  });
});
