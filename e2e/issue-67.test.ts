import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("issue #67: boolean discriminant + body param index", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "issue-67");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should build successfully", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/issue-67/tsconfig.json",
      "--config",
      "testdata/issue-67/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
  });

  // --- Bug 1: Boolean discriminant ---

  it("should emit boolean case labels (not string) in validate companion", () => {
    const companionFile = resolve(
      distDir,
      "types.SubmitResultDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);

    const content = readFileSync(companionFile, "utf-8");

    // Must have boolean case labels, not string-quoted
    expect(content).toContain("case false:");
    expect(content).toContain("case true:");
    expect(content).not.toContain('case "false"');
    expect(content).not.toContain('case "true"');
  });

  // --- Bug 2: Body param assertion targets correct parameter ---

  it("should inject body assertion on correct parameter (not first param)", () => {
    const controllerFile = resolve(distDir, "prompt.controller.js");
    expect(existsSync(controllerFile)).toBe(true);

    const content = readFileSync(controllerFile, "utf-8");

    // Must NOT assert companyID (the @Param) with the body assertion
    expect(content).not.toMatch(
      /companyID\s*=\s*assertUpdatePromptDto\(companyID\)/
    );

    // Should assert the body parameter (synthetic __body for destructured)
    expect(content).toContain("assertUpdatePromptDto(__body)");

    // Should reconstruct destructuring inside method body
    expect(content).toContain("const { content } = __body");
  });

  it("should handle second destructured body param correctly", () => {
    const controllerFile = resolve(distDir, "prompt.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // Must NOT assert orgId with the body assertion
    expect(content).not.toMatch(
      /orgId\s*=\s*assertPatchSettingsDto\(orgId\)/
    );

    // Should assert the body parameter
    expect(content).toContain("assertPatchSettingsDto(__body)");

    // Should reconstruct destructuring for theme and fontSize
    expect(content).toMatch(/const \{ theme, fontSize \} = __body/);
  });
});
