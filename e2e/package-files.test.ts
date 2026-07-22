import { describe, it, expect, beforeAll } from "vitest";
import { execSync } from "child_process";
import { resolve } from "path";
import { PROJECT_ROOT } from "./helpers";

// Issue #223: the published tsgonest@0.15.0 tarball was missing dist/ because
// tsdown never built the src/index.ts entry — marker imports failed to
// typecheck (TS2307) in consumer projects. This test builds the package and
// verifies the packed tarball actually contains the runtime entry points.
describe("tsgonest npm package contents (issue #223)", () => {
  const corePkg = resolve(PROJECT_ROOT, "packages", "core");
  let files: string[] = [];

  beforeAll(() => {
    execSync("pnpm --filter tsgonest build", {
      cwd: PROJECT_ROOT,
      stdio: "pipe",
      timeout: 120_000,
    });
    // --ignore-scripts: prepack already ran via the explicit build above
    const out = execSync("npm pack --dry-run --json --ignore-scripts", {
      cwd: corePkg,
      stdio: "pipe",
      timeout: 120_000,
    }).toString();
    const parsed = JSON.parse(out);
    files = parsed[0].files.map((f: { path: string }) => f.path);
  }, 180_000);

  it("includes the dist/ marker entry points for ESM and CJS", () => {
    for (const expected of [
      "dist/index.mjs",
      "dist/index.cjs",
      "dist/index.d.mts",
      "dist/index.d.cts",
    ]) {
      expect(files).toContain(expected);
    }
  });

  it("includes the bin launcher but not a stray native binary", () => {
    expect(files).toContain("bin/tsgonest");
    expect(files).not.toContain("bin/tsgonest-native");
  });

  it("includes package.json and the migrate entry", () => {
    expect(files).toContain("package.json");
    expect(files).toContain("bin/migrate.cjs");
  });
});
