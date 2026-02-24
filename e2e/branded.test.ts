import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("branded type validation (@tsgonest/types)", () => {
  const brandedDist = resolve(FIXTURES_DIR, "branded/dist");

  beforeAll(() => {
    if (existsSync(brandedDist)) {
      rmSync(brandedDist, { recursive: true });
    }
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/branded/tsconfig.json",
      "--config",
      "testdata/branded/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");
  });

  it("should generate validation with email format from branded type", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);
    const content = readFileSync(companionFile, "utf-8");

    expect(content).toContain("format email");
    expect(content).toContain("input.name.length < 1");
    expect(content).toContain("input.name.length > 255");
    expect(content).toContain("input.age < 0");
    expect(content).toContain("input.age > 150");
  });

  it("branded validation should accept valid input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      email: "alice@example.com",
      name: "Alice",
      age: 30,
    });
    expect(result.success).toBe(true);
  });

  it("branded validation should reject invalid email format", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      email: "not-an-email",
      name: "Alice",
      age: 30,
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.email")).toBe(true);
  });

  it("branded validation should reject name below minLength", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      email: "alice@example.com",
      name: "",
      age: 30,
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.name")).toBe(true);
  });

  it("branded validation should reject age outside range", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      email: "alice@example.com",
      name: "Alice",
      age: 200,
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.age")).toBe(true);
  });

  it("should generate ProductDto validation with ExclusiveMinimum and Int", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ProductDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);
    const content = readFileSync(companionFile, "utf-8");

    expect(content).toContain("format uuid");
    expect(content).toContain("input.price <= 0");
    expect(content).toContain("Number.isInteger");
  });

  it("should generate ConfigDto validation with StartsWith and Includes", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ConfigDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);
    const content = readFileSync(companionFile, "utf-8");

    expect(content).toContain('startsWith("https://")');
    expect(content).toContain('includes("@")');
    expect(content).toContain("format email");
  });

  it("ProductDto validation should work at runtime", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ProductDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const valid = mod.validateProductDto({
      id: "550e8400-e29b-41d4-a716-446655440000",
      price: 10,
      quantity: 0,
    });
    expect(valid.success).toBe(true);

    const invalid = mod.validateProductDto({
      id: "550e8400-e29b-41d4-a716-446655440000",
      price: 0,
      quantity: 5,
    });
    expect(invalid.success).toBe(false);
    expect(invalid.errors.some((e: any) => e.path === "input.price")).toBe(
      true
    );
  });

  it("ConfigDto validation should work at runtime", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ConfigDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const valid = mod.validateConfigDto({
      websiteUrl: "https://example.com",
      contactEmail: "info@example.com",
    });
    expect(valid.success).toBe(true);

    const invalidUrl = mod.validateConfigDto({
      websiteUrl: "http://example.com",
      contactEmail: "info@example.com",
    });
    expect(invalidUrl.success).toBe(false);
    expect(
      invalidUrl.errors.some((e: any) => e.path === "input.websiteUrl")
    ).toBe(true);
  });
});
