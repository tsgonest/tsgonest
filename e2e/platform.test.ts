import { describe, it, expect } from "vitest";
import { existsSync, statSync } from "fs";
import { TSGONEST_BIN, runTsgonest } from "./helpers";

// Sanity checks that fail fast with a clear error if the e2e harness was
// pointed at a binary path that doesn't exist on the current platform.
// Catches regressions in the platform-aware binary-path logic in helpers.ts
// (issue #151) and surfaces "wrong binary name" failures as a single named
// failure instead of cascading through every other e2e test.
describe("e2e harness platform sanity", () => {
  it("resolves to a real native binary on this platform", () => {
    expect(
      existsSync(TSGONEST_BIN),
      `e2e harness expected the native binary at ${TSGONEST_BIN}. ` +
        `On Windows that is "tsgonest.exe"; on Unix it is "tsgonest-native". ` +
        `Run "just build" first.`
    ).toBe(true);
    expect(statSync(TSGONEST_BIN).isFile()).toBe(true);
  });

  it("the binary is executable end-to-end", () => {
    const { stdout, exitCode } = runTsgonest(["--version"]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("tsgonest");
  });
});
