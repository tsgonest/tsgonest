import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("realworld fixture", () => {
  const realworldDist = resolve(FIXTURES_DIR, "realworld/dist");
  const openapiFile = resolve(FIXTURES_DIR, "realworld/dist/openapi.json");

  beforeAll(() => {
    if (existsSync(realworldDist)) {
      rmSync(realworldDist, { recursive: true });
    }
  });

  it("should compile realworld fixture without errors", () => {
    const { stderr, exitCode } = runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
  });

  it("should generate companion files for realworld fixture", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    expect(stderr).toContain("companion");
    const companions = [
      "realworld/dist/auth/auth.dto.LoginDto.tsgonest.js",
      "realworld/dist/auth/auth.dto.RegisterDto.tsgonest.js",
      "realworld/dist/auth/auth.dto.AuthTokenResponse.tsgonest.js",
      "realworld/dist/user/user.dto.UserResponse.tsgonest.js",
      "realworld/dist/article/article.dto.CreateArticleDto.tsgonest.js",
      "realworld/dist/article/article.dto.Comment.tsgonest.js",
      "realworld/dist/payment/payment.dto.PaymentResponse.tsgonest.js",
    ];
    for (const comp of companions) {
      expect(
        existsSync(resolve(FIXTURES_DIR, comp)),
        `companion file should exist: ${comp}`
      ).toBe(true);
    }
  });

  it("should generate manifest for realworld fixture", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    const manifestDir = resolve(FIXTURES_DIR, "realworld/dist");
    const possiblePaths = [
      resolve(manifestDir, "__tsgonest_manifest.json"),
      resolve(manifestDir, "common/__tsgonest_manifest.json"),
      resolve(manifestDir, "auth/__tsgonest_manifest.json"),
    ];
    const manifestExists = possiblePaths.some((p) => existsSync(p));
    expect(manifestExists).toBe(true);
  });

  it("should find 5 controllers with 24 routes", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    expect(stderr).toContain("5 controller(s)");
    expect(stderr).toContain("24 route(s)");
  });

  it("should generate valid OpenAPI document", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    expect(existsSync(openapiFile)).toBe(true);

    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(doc.openapi).toBe("3.2.0");
    expect(doc.info).toBeDefined();
    expect(doc.paths).toBeDefined();
  });

  it("should have correct paths in OpenAPI document", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const paths = Object.keys(doc.paths);
    expect(paths).toContain("/auth/login");
    expect(paths).toContain("/auth/register");
    expect(paths).toContain("/users");
    expect(paths).toContain("/users/{id}");
    expect(paths).toContain("/articles");
    expect(paths).toContain("/articles/{slug}");
    expect(paths).toContain("/payments");
    expect(paths).toContain("/payments/{id}");
    expect(paths).toContain("/admin/dashboard");
    expect(paths).toContain("/admin/config");
  });

  it("should have component schemas for DTOs", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const schemas = Object.keys(doc.components?.schemas ?? {});
    expect(schemas).toContain("LoginDto");
    expect(schemas).toContain("RegisterDto");
    expect(schemas).toContain("AuthTokenResponse");
    expect(schemas).toContain("CreateArticleDto");
    expect(schemas).toContain("Comment");
  });

  it("should validate LoginDto at runtime", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);

    const validatePath = resolve(
      FIXTURES_DIR,
      "realworld/dist/auth/auth.dto.LoginDto.tsgonest.js"
    );
    if (!existsSync(validatePath)) {
      throw new Error(`companion file not found: ${validatePath}`);
    }

    const mod = require(validatePath);
    const validResult = mod.validateLoginDto({
      email: "test@example.com",
      password: "securepass123",
    });
    expect(validResult.success).toBe(true);

    const invalidResult = mod.validateLoginDto({});
    expect(invalidResult.success).toBe(false);
    expect(invalidResult.errors.length).toBeGreaterThan(0);
  });

  it("should validate LoginDto constraints at runtime", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);

    const validatePath = resolve(
      FIXTURES_DIR,
      "realworld/dist/auth/auth.dto.LoginDto.tsgonest.js"
    );
    const mod = require(validatePath);

    const result = mod.validateLoginDto({
      email: "not-an-email",
      password: "short",
    });
    expect(result.success).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
  });

  it("should serialize AuthTokenResponse at runtime", () => {
    runTsgonest([
      "--project",
      "testdata/realworld/tsconfig.json",
      "--config",
      "testdata/realworld/tsgonest.config.json",
    ]);

    const companionPath = resolve(
      FIXTURES_DIR,
      "realworld/dist/auth/auth.dto.AuthTokenResponse.tsgonest.js"
    );
    if (!existsSync(companionPath)) {
      throw new Error(`companion file not found: ${companionPath}`);
    }

    const mod = require(companionPath);
    const json = mod.serializeAuthTokenResponse({
      accessToken: "abc123",
      refreshToken: "def456",
      expiresIn: 3600,
      tokenType: "Bearer",
    });
    const parsed = JSON.parse(json);
    expect(parsed.accessToken).toBe("abc123");
    expect(parsed.expiresIn).toBe(3600);
    expect(parsed.tokenType).toBe("Bearer");
  });
});
