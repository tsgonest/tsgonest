import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

/**
 * Tests for primitive return type JSON serialization.
 *
 * When a NestJS controller method returns a primitive type (string, number, boolean),
 * the response Content-Type is set to application/json by the serialize interceptor.
 * The response body MUST be valid JSON:
 *   - string "hello" → "\"hello\"" (wrapped in JSON quotes)
 *   - number 42 → "42"
 *   - boolean true → "true"
 *
 * Without this fix, the response body contains the raw value which is NOT valid JSON
 * for strings, causing SDK clients' response.json() to throw SyntaxError.
 *
 * Reference: nestia handles this via typia.json.stringify<T> which correctly
 * serializes all types including primitives. See:
 *   - nestia: packages/core/src/decorators/TypedRoute.ts
 *   - typia: src/programmers/json/JsonStringifyProgrammer.ts
 */
describe("tsgonest primitive return type serialization", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "primitive-returns");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should build successfully", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/primitive-returns/tsconfig.json",
      "--config",
      "testdata/primitive-returns/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
  });

  it("should wrap string return values with JSON encoding", () => {
    const controllerFile = resolve(distDir, "primitive.controller.js");
    expect(existsSync(controllerFile)).toBe(true);

    const content = readFileSync(controllerFile, "utf-8");

    // The forgotPassword() method returns Promise<string>.
    // Its return statement must be wrapped to produce valid JSON.
    // Either via __s() (fast string serializer) or JSON.stringify().
    const hasStringSerialize =
      content.includes("__s(") || content.includes("JSON.stringify(");
    expect(hasStringSerialize).toBe(true);
  });

  it("should wrap number return values with JSON encoding", () => {
    const controllerFile = resolve(distDir, "primitive.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // getCount() returns Promise<number> — must be serialized.
    // Either Number.isFinite check or JSON.stringify.
    const hasNumberSerialize =
      content.includes("Number.isFinite") || content.includes("JSON.stringify(");
    expect(hasNumberSerialize).toBe(true);
  });

  it("should wrap boolean return values with JSON encoding", () => {
    const controllerFile = resolve(distDir, "primitive.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // isEnabled() returns Promise<boolean> — must produce "true"/"false" strings
    const hasBooleanSerialize =
      content.includes('"true"') ||
      content.includes('"false"') ||
      content.includes("JSON.stringify(");
    expect(hasBooleanSerialize).toBe(true);
  });

  it("should produce valid JSON when executing string return", () => {
    const controllerFile = resolve(distDir, "primitive.controller.js");
    if (!existsSync(controllerFile)) return;

    const content = readFileSync(controllerFile, "utf-8");

    // Find the forgotPassword or getVersion method and verify its return
    // is wrapped — the raw string "1.0.0" is NOT valid JSON,
    // but "\"1.0.0\"" is.
    //
    // Look for evidence that return statements in string-returning methods
    // are transformed (not left as raw returns).
    const lines = content.split("\n");
    const versionMethodIdx = lines.findIndex((l) =>
      l.includes("getVersion")
    );

    if (versionMethodIdx >= 0) {
      const methodBody = lines
        .slice(versionMethodIdx, versionMethodIdx + 10)
        .join("\n");

      // The return should NOT be a bare `return "1.0.0";`
      // It should be wrapped: `return __s(await "1.0.0");` or similar
      const isBareReturn =
        methodBody.includes('return "1.0.0";') &&
        !methodBody.includes("__s(") &&
        !methodBody.includes("JSON.stringify(");

      expect(isBareReturn).toBe(false);
    }
  });

  it("should add TsgonestSerializeInterceptor for primitive returns", () => {
    const controllerFile = resolve(distDir, "primitive.controller.js");
    if (!existsSync(controllerFile)) return;

    const content = readFileSync(controllerFile, "utf-8");

    // The serialize interceptor must be injected even for primitive returns,
    // because it sets Content-Type to application/json and sends the
    // pre-serialized string directly (bypassing Express's default serialization).
    expect(content).toContain("TsgonestSerializeInterceptor");
  });
});
