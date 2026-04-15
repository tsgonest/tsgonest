import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync, writeFileSync } from "fs";
import { spawnSync } from "child_process";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

// Regression: when a DTO field is typed as a recursive type defined in a file
// that is NOT directly referenced by any controller (the same situation as
// recursive types imported from external packages like @tiptap/core), the
// generated companion previously emitted calls to isJSONContent /
// validateJSONContent / assertJSONContent / serializeJSONContent without ever
// declaring or importing those helpers — producing a runtime ReferenceError.
describe("recursive external type: JSONContent-like shape", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "issue-recursive-external");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("builds successfully", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/issue-recursive-external/tsconfig.json",
      "--config",
      "testdata/issue-recursive-external/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
  });

  it("emits helper functions for the recursive external type locally", () => {
    const companionFile = resolve(
      distDir,
      "notes.dto.NotesResponse.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);

    const src = readFileSync(companionFile, "utf-8");

    // All four helper kinds must be defined locally in the same file.
    expect(src).toMatch(/function isJSONContent\s*\(/);
    expect(src).toMatch(/function _validateJSONContent\s*\(/);
    expect(src).toMatch(/function _assertJSONContent\s*\(/);
    expect(src).toMatch(/function serializeJSONContent\s*\(/);

    // And there must be no import pointing at a non-existent companion file
    // for the external recursive type.
    expect(src).not.toMatch(/JSONContent\.tsgonest\.js/);
  });

  it("executes without a ReferenceError and round-trips a real payload", () => {
    // Write a tiny ESM exerciser alongside the emitted files.
    const exerciser = resolve(distDir, "__exercise.mjs");
    const pkgJson = resolve(distDir, "package.json");
    writeFileSync(pkgJson, JSON.stringify({ type: "module" }));
    writeFileSync(
      exerciser,
      [
        `import * as M from "./notes.dto.NotesResponse.tsgonest.js";`,
        `const payload = {`,
        `  notes: {`,
        `    type: "doc",`,
        `    content: [{ type: "paragraph", content: [{ type: "text", text: "hi" }] }],`,
        `  },`,
        `};`,
        `if (!M.isNotesResponse(payload)) { console.error("isNotesResponse=false"); process.exit(2); }`,
        `const v = M.validateNotesResponse(payload);`,
        `if (!v.success) { console.error("validate failed:", JSON.stringify(v.errors)); process.exit(3); }`,
        `M.assertNotesResponse(payload);`,
        `const s = M.stringifyNotesResponse(payload);`,
        `const parsed = JSON.parse(s);`,
        `if (JSON.stringify(parsed) !== JSON.stringify(payload)) { console.error("round-trip mismatch:", s); process.exit(4); }`,
        `console.log("OK");`,
      ].join("\n")
    );

    const result = spawnSync("node", [exerciser], { encoding: "utf-8", cwd: distDir });
    // The pre-fix failure mode was:
    //   ReferenceError: isJSONContent is not defined
    // This test guards against that specific regression.
    expect(result.stderr).not.toMatch(/ReferenceError/);
    expect(result.stdout).toContain("OK");
    expect(result.status).toBe(0);
  });
});
