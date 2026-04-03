import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

/**
 * Issue #78: Serializer fails for nullable return types (T | null)
 *
 * When a controller method returns Promise<T | null>, the generated serializer
 * must handle null/undefined values. Without the fix, stringifyT(null) throws:
 *   TypeError: Serialization type check failed for T
 * and (await null).map(...) throws:
 *   TypeError: Cannot read properties of null (reading 'map')
 *
 * These tests cover realistic NestJS patterns:
 *   1. Prisma findFirst/findUnique → T | null
 *   2. Search returning array or null → T[] | null
 *   3. Non-nullable baseline (should NOT get null guard)
 *   4. Array.find / Map.get → T | undefined
 *   5. Synchronous cache lookup → non-async T | null
 *   6. Try/catch with null fallback → multiple return paths
 */
describe("tsgonest nullable DTO return type serialization (issue #78)", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "nullable-returns");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
    const cacheFile = resolve(fixtureDir, "tsconfig.tsgonest-cache");
    if (existsSync(cacheFile)) rmSync(cacheFile);
    const buildInfoFile = resolve(fixtureDir, "tsconfig.tsbuildinfo");
    if (existsSync(buildInfoFile)) rmSync(buildInfoFile);
  });

  it("should build successfully", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/nullable-returns/tsconfig.json",
      "--config",
      "testdata/nullable-returns/tsgonest.config.json",
      "--clean",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
  });

  it("should generate companion file for ItemResponse", () => {
    const companion = resolve(distDir, "item.dto.ItemResponse.tsgonest.js");
    expect(existsSync(companion)).toBe(true);

    const content = readFileSync(companion, "utf-8");
    expect(content).toContain("export function stringifyItemResponse");
    expect(content).toContain("export function isItemResponse");
    expect(content).toContain("export function serializeItemResponse");
  });

  describe("Case 1: Promise<ItemResponse | null> (Prisma pattern)", () => {
    it("should add null guard before stringify for getById", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      expect(existsSync(controllerFile)).toBe(true);

      const content = readFileSync(controllerFile, "utf-8");

      // Must import stringify from companion
      expect(content).toContain("stringifyItemResponse");

      // Find the getById method section
      const lines = content.split("\n");
      const getByIdIdx = lines.findIndex((l) => l.includes("getById"));
      expect(getByIdIdx).toBeGreaterThan(-1);

      // Extract ~10 lines of method body
      const methodBody = lines
        .slice(getByIdIdx, getByIdIdx + 10)
        .join("\n");

      // Must have null check — stringify(null) would throw TypeError
      expect(methodBody).toMatch(/==\s*null|===\s*null/);
    });
  });

  describe("Case 2: Promise<ItemResponse[] | null> (nullable array)", () => {
    it("should add null guard before .map() for search", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      const content = readFileSync(controllerFile, "utf-8");

      // Find the search method section
      const lines = content.split("\n");
      const searchIdx = lines.findIndex((l) => l.includes("search"));
      expect(searchIdx).toBeGreaterThan(-1);

      const methodBody = lines
        .slice(searchIdx, searchIdx + 15)
        .join("\n");

      // (await null).map() would throw TypeError: Cannot read properties of null
      // The return wrapping must guard against null BEFORE calling .map()
      expect(methodBody).toMatch(/==\s*null|===\s*null/);
    });
  });

  describe("Case 3: Promise<ItemResponse[]> (non-nullable baseline)", () => {
    it("should NOT add null guard for findAll", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      const content = readFileSync(controllerFile, "utf-8");

      // Find the findAll method section
      const lines = content.split("\n");
      const findAllIdx = lines.findIndex((l) => l.includes("findAll"));
      expect(findAllIdx).toBeGreaterThan(-1);

      // Find end of method (next method or closing brace)
      const nextMethodIdx = lines.findIndex(
        (l, i) =>
          i > findAllIdx + 1 &&
          (l.includes("getDefault") || l.includes("getCached") || l.includes("search"))
      );
      const endIdx =
        nextMethodIdx > -1 ? nextMethodIdx : findAllIdx + 10;

      const methodBody = lines
        .slice(findAllIdx, endIdx)
        .join("\n");

      // Non-nullable array return: .map() is called directly, no null guard needed.
      // If a null guard appears here, the fix is being too aggressive.
      expect(methodBody).not.toMatch(/_v\s*==\s*null/);
    });
  });

  describe("Case 4: Promise<ItemResponse | undefined> (optional DTO)", () => {
    it("should add null/undefined guard for getDefault", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      const content = readFileSync(controllerFile, "utf-8");

      const lines = content.split("\n");
      const getDefaultIdx = lines.findIndex((l) =>
        l.includes("getDefault")
      );
      expect(getDefaultIdx).toBeGreaterThan(-1);

      const methodBody = lines
        .slice(getDefaultIdx, getDefaultIdx + 10)
        .join("\n");

      // undefined must also be guarded — JS `== null` catches both
      expect(methodBody).toMatch(/==\s*null|===\s*(null|undefined)/);
    });
  });

  describe("Case 5: non-async ItemResponse | null (sync method)", () => {
    it("should insert async AND null guard for getCached", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      const content = readFileSync(controllerFile, "utf-8");

      const lines = content.split("\n");
      const getCachedIdx = lines.findIndex((l) =>
        l.includes("getCached")
      );
      expect(getCachedIdx).toBeGreaterThan(-1);

      const methodBody = lines
        .slice(getCachedIdx, getCachedIdx + 10)
        .join("\n");

      // Must have async inserted (needed for await in the null-guard wrapper)
      expect(methodBody).toContain("async");

      // Must have null guard
      expect(methodBody).toMatch(/==\s*null|===\s*null/);
    });
  });

  describe("Case 6: try/catch with null fallback (multiple returns)", () => {
    it("should wrap ALL return paths with null guard for getSafe", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      const content = readFileSync(controllerFile, "utf-8");

      const lines = content.split("\n");
      const getSafeIdx = lines.findIndex((l) => l.includes("getSafe"));
      expect(getSafeIdx).toBeGreaterThan(-1);

      // Extract generous chunk for try/catch
      const methodBody = lines
        .slice(getSafeIdx, getSafeIdx + 20)
        .join("\n");

      // Both the try-path return and the catch-path return must be wrapped.
      // Count null checks: should be >= 2 (one per return path).
      const nullChecks = (methodBody.match(/==\s*null|===\s*null/g) || [])
        .length;
      expect(nullChecks).toBeGreaterThanOrEqual(2);
    });
  });

  describe("Case 7: (ItemResponse | null)[] (array of nullable elements)", () => {
    it("should add element-level null guard inside .map() for getByIds", () => {
      const controllerFile = resolve(distDir, "item.controller.js");
      const content = readFileSync(controllerFile, "utf-8");

      const lines = content.split("\n");
      const getByIdsIdx = lines.findIndex((l) =>
        l.includes("getByIds")
      );
      expect(getByIdsIdx).toBeGreaterThan(-1);

      const methodBody = lines
        .slice(getByIdsIdx, getByIdsIdx + 10)
        .join("\n");

      // Each element in the array could be null.
      // The .map() callback must guard: _i == null ? "null" : serializeItemResponse(_i)
      // Without fix: serializeItemResponse(null) crashes
      expect(methodBody).toMatch(/==\s*null/);
    });
  });
});
