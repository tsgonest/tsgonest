import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest @Query/@Param validation + coercion", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "rewrite-query-param");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should build successfully", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/rewrite-query-param/tsconfig.json",
      "--config",
      "testdata/rewrite-query-param/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("controller");
  });

  it("should inject assertPaginationQuery for whole-object @Query()", () => {
    const controllerFile = resolve(distDir, "order.controller.js");
    const content = readFileSync(controllerFile, "utf-8");
    expect(content).toContain("assertPaginationQuery(query)");
  });

  it("should inject inline number coercion for @Param('id')", () => {
    const controllerFile = resolve(distDir, "order.controller.js");
    const content = readFileSync(controllerFile, "utf-8");
    expect(content).toContain("id = +id");
    expect(content).toContain("Number.isNaN(id)");
  });

  it("should not inject coercion for string @Param('status')", () => {
    const controllerFile = resolve(distDir, "order.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // findByStatus has string param — no coercion line for 'status'
    const lines = content.split("\n");
    const methodIdx = lines.findIndex((l) => l.includes("findByStatus("));
    if (methodIdx >= 0) {
      const methodBody = lines.slice(methodIdx, methodIdx + 5).join("\n");
      expect(methodBody).not.toContain("status = +status");
      expect(methodBody).not.toContain('=== "true"');
    }
  });

  it("should handle mixed @Body + @Query + @Param in one method", () => {
    const controllerFile = resolve(distDir, "order.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // addItem method should have all three
    expect(content).toContain("assertOrderResponse(body)");
    expect(content).toContain("assertPaginationQuery(query)");
    // id coercion should also appear (from the second @Param("id"))
    const idCoercions = content.match(/id = \+id/g);
    expect(idCoercions).not.toBeNull();
    expect(idCoercions!.length).toBeGreaterThanOrEqual(2); // findOne + addItem
  });

  it("should generate companion for PaginationQuery with coercion", () => {
    const companion = resolve(distDir, "dto.PaginationQuery.tsgonest.js");
    expect(existsSync(companion)).toBe(true);

    const content = readFileSync(companion, "utf-8");
    expect(content).toContain("export function assertPaginationQuery");
    // Should have string→number coercion in validate/assert
    expect(content).toContain('typeof input.page === "string"');
    expect(content).toContain('typeof input.limit === "string"');
    // Should have boolean coercion with "1"/"0"
    expect(content).toContain('=== "true"');
    expect(content).toContain('=== "1"');
  });

  it("should import TsgonestValidationError for inline scalar coercion", () => {
    const controllerFile = resolve(distDir, "order.controller.js");
    const content = readFileSync(controllerFile, "utf-8");
    expect(content).toContain("TsgonestValidationError as __e");
  });

  it("should generate companion for OrderResponse", () => {
    const companion = resolve(distDir, "dto.OrderResponse.tsgonest.js");
    expect(existsSync(companion)).toBe(true);

    const content = readFileSync(companion, "utf-8");
    expect(content).toContain("export function assertOrderResponse");
    expect(content).toContain("export function stringifyOrderResponse");
  });

  it("should generate valid runtime code for PaginationQuery assert", async () => {
    const companion = resolve(distDir, "dto.PaginationQuery.tsgonest.js");
    const helpers = resolve(distDir, "_tsgonest_helpers.js");
    expect(existsSync(companion)).toBe(true);
    expect(existsSync(helpers)).toBe(true);

    // Import and run the assert function
    const { assertPaginationQuery } = await import(companion);

    // Should coerce strings to numbers and validate
    const result = assertPaginationQuery({ page: "2", limit: "10" });
    expect(result.page).toBe(2);
    expect(result.limit).toBe(10);

    // Should coerce boolean strings
    const result2 = assertPaginationQuery({
      page: "1",
      limit: "50",
      ascending: "true",
    });
    expect(result2.ascending).toBe(true);

    // "1" should also work for boolean
    const result3 = assertPaginationQuery({
      page: "1",
      limit: "50",
      ascending: "1",
    });
    expect(result3.ascending).toBe(true);

    // Should reject invalid values
    expect(() => assertPaginationQuery({ page: "abc", limit: "10" })).toThrow();
  });
});
