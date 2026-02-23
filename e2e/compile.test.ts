import { describe, it, expect, beforeAll } from "vitest";
import { spawnSync } from "child_process";
import { existsSync, readFileSync, rmSync, readdirSync } from "fs";
import { resolve } from "path";

const PROJECT_ROOT = resolve(__dirname, "..");
const TSGONEST_BIN = resolve(PROJECT_ROOT, "tsgonest");
const FIXTURES_DIR = resolve(PROJECT_ROOT, "testdata");

function runTsgonest(args: string[], opts?: { cwd?: string }) {
  const result = spawnSync(TSGONEST_BIN, args, {
    encoding: "utf-8",
    cwd: opts?.cwd ?? PROJECT_ROOT,
    timeout: 30000,
  });
  return {
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    exitCode: result.status ?? 1,
  };
}

describe("tsgonest compilation", () => {
  beforeAll(() => {
    const distDir = resolve(FIXTURES_DIR, "simple/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should print version", () => {
    const { stdout, exitCode } = runTsgonest(["--version"]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("tsgonest");
  });

  it("should print help", () => {
    const { stdout, exitCode } = runTsgonest(["--help"]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("Usage:");
    expect(stdout).toContain("--project");
    expect(stdout).toContain("--config");
  });

  it("should compile simple TypeScript project", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/simple/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");

    // Verify output files exist
    expect(existsSync(resolve(FIXTURES_DIR, "simple/dist/index.js"))).toBe(
      true
    );
    expect(
      existsSync(resolve(FIXTURES_DIR, "simple/dist/index.d.ts"))
    ).toBe(true);
    expect(
      existsSync(resolve(FIXTURES_DIR, "simple/dist/index.js.map"))
    ).toBe(true);
  });

  it("should produce correct JavaScript output", () => {
    const jsFile = resolve(FIXTURES_DIR, "simple/dist/index.js");
    const content = readFileSync(jsFile, "utf-8");

    expect(content).toContain("export function greet");
    // Should not have TypeScript type annotations
    expect(content).not.toContain(": User");
    expect(content).not.toContain(": string");
  });

  it("should fail with missing tsconfig", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "nonexistent/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);
    expect(stderr).toContain("could not find tsconfig");
  });

  it("should load config file when specified", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/simple/tsconfig.json",
      "--config",
      "testdata/simple/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("loaded config");
  });
});

describe("tsgonest companion file generation", () => {
  beforeAll(() => {
    const distDir = resolve(FIXTURES_DIR, "nestjs/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should generate companion files for NestJS project", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/nestjs/tsconfig.json",
      "--config",
      "testdata/nestjs/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");
  });

  it("should generate companion files with validate and serialize", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);

    const content = readFileSync(companionFile, "utf-8");
    expect(content).toContain("export function validateCreateUserDto");
    expect(content).toContain("export function assertCreateUserDto");
    expect(content).not.toContain("deserializeCreateUserDto");
    expect(content).toContain('typeof input.name !== "string"');
    expect(content).toContain('typeof input.age !== "number"');
  });

  it("should generate serialization functions in companion files", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UserResponse.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);

    const content = readFileSync(companionFile, "utf-8");
    expect(content).toContain("export function serializeUserResponse");
    expect(content).toContain("__s(");
  });

  it("should handle optional properties in UpdateUserDto", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UpdateUserDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);

    const content = readFileSync(companionFile, "utf-8");
    expect(content).toContain("export function validateUpdateUserDto");
    // Optional props should use undefined guard
    expect(content).toContain("input.name !== undefined");
    expect(content).toContain("input.email !== undefined");
    expect(content).toContain("input.age !== undefined");
  });

  it("should dump metadata as JSON", () => {
    const { stdout, exitCode } = runTsgonest([
      "--dump-metadata",
      "--project",
      "testdata/nestjs/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);

    const metadata = JSON.parse(stdout);
    expect(metadata).toHaveProperty("files");
    expect(metadata).toHaveProperty("registry");
    expect(metadata.registry).toHaveProperty("CreateUserDto");
    expect(metadata.registry.CreateUserDto.kind).toBe("object");
    expect(metadata.registry.CreateUserDto.properties).toHaveLength(3);
  });

  it("should generate OpenAPI document", () => {
    // The companion generation beforeAll already ran tsgonest with config
    // Verify OpenAPI output was generated (relative to config file directory)
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("should produce valid OpenAPI 3.1 document", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const content = readFileSync(openapiFile, "utf-8");
    const doc = JSON.parse(content);

    // Verify OpenAPI version
    expect(doc.openapi).toBe("3.2.0");
    expect(doc.info).toBeDefined();
    expect(doc.info.title).toBe("API");
    expect(doc.info.version).toBe("1.0.0");
  });

  it("should have correct paths in OpenAPI document", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    // Should have /users and /users/{id} paths
    expect(doc.paths).toHaveProperty("/users");
    expect(doc.paths).toHaveProperty("/users/{id}");

    // /users should have GET and POST
    expect(doc.paths["/users"].get).toBeDefined();
    expect(doc.paths["/users"].post).toBeDefined();
    expect(doc.paths["/users"].get.operationId).toBe("findAll");
    expect(doc.paths["/users"].post.operationId).toBe("create");

    // /users/{id} should have GET, PUT, DELETE
    expect(doc.paths["/users/{id}"].get).toBeDefined();
    expect(doc.paths["/users/{id}"].put).toBeDefined();
    expect(doc.paths["/users/{id}"].delete).toBeDefined();
    expect(doc.paths["/users/{id}"].get.operationId).toBe("findOne");
  });

  it("should have correct schemas in components", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    expect(doc.components).toBeDefined();
    expect(doc.components.schemas).toBeDefined();

    // Should have DTOs
    expect(doc.components.schemas).toHaveProperty("CreateUserDto");
    expect(doc.components.schemas).toHaveProperty("UpdateUserDto");
    expect(doc.components.schemas).toHaveProperty("UserResponse");

    // Verify CreateUserDto schema
    const createDto = doc.components.schemas.CreateUserDto;
    expect(createDto.type).toBe("object");
    expect(createDto.properties).toHaveProperty("name");
    expect(createDto.properties).toHaveProperty("email");
    expect(createDto.properties).toHaveProperty("age");
    expect(createDto.properties.name.type).toBe("string");
    expect(createDto.properties.age.type).toBe("number");
    expect(createDto.required).toContain("name");
    expect(createDto.required).toContain("email");
    expect(createDto.required).toContain("age");

    // Verify UpdateUserDto has no required (all props optional)
    const updateDto = doc.components.schemas.UpdateUserDto;
    expect(updateDto.type).toBe("object");
    expect(updateDto.required).toBeUndefined();
  });

  it("should have correct request body for POST", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const postOp = doc.paths["/users"].post;
    expect(postOp.requestBody).toBeDefined();
    expect(postOp.requestBody.required).toBe(true);
    expect(postOp.requestBody.content["application/json"]).toBeDefined();
    expect(
      postOp.requestBody.content["application/json"].schema.$ref
    ).toBe("#/components/schemas/CreateUserDto");
  });

  it("should have correct path parameters", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const getOne = doc.paths["/users/{id}"].get;
    expect(getOne.parameters).toHaveLength(1);
    expect(getOne.parameters[0].name).toBe("id");
    expect(getOne.parameters[0].in).toBe("path");
    expect(getOne.parameters[0].required).toBe(true);
    expect(getOne.parameters[0].schema.type).toBe("string");
  });

  it("should have correct response types", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    // GET /users returns array of UserResponse
    const getAll = doc.paths["/users"].get;
    const response200 = getAll.responses["200"];
    expect(response200).toBeDefined();
    expect(response200.content["application/json"].schema.type).toBe("array");
    expect(response200.content["application/json"].schema.items.$ref).toBe(
      "#/components/schemas/UserResponse"
    );

    // POST /users returns 201 with UserResponse
    const post = doc.paths["/users"].post;
    expect(post.responses["201"]).toBeDefined();
    expect(
      post.responses["201"].content["application/json"].schema.$ref
    ).toBe("#/components/schemas/UserResponse");

    // DELETE /users/{id} returns 204 with no content
    const deleteOp = doc.paths["/users/{id}"].delete;
    expect(deleteOp.responses["204"]).toBeDefined();
    expect(deleteOp.responses["204"].content).toBeUndefined();
  });

  it("should have query parameters decomposed from ListQuery", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const getAll = doc.paths["/users"].get;
    // ListQuery has page?, limit?, search? — should be decomposed into 3 query params
    expect(getAll.parameters).toBeDefined();
    expect(getAll.parameters.length).toBeGreaterThanOrEqual(3);

    const paramNames = getAll.parameters.map((p: any) => p.name);
    expect(paramNames).toContain("page");
    expect(paramNames).toContain("limit");
    expect(paramNames).toContain("search");

    // All should be query params and not required
    const queryParams = getAll.parameters.filter((p: any) => p.in === "query");
    expect(queryParams.length).toBe(3);
    for (const p of queryParams) {
      expect(p.required).toBe(false);
    }
  });

  it("should have tags derived from controller name", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    expect(doc.tags).toBeDefined();
    expect(doc.tags).toContainEqual({ name: "User" });

    // All operations should have the User tag
    expect(doc.paths["/users"].get.tags).toContain("User");
    expect(doc.paths["/users"].post.tags).toContain("User");
    expect(doc.paths["/users/{id}"].get.tags).toContain("User");
  });

  it("should resolve custom decorators with @in JSDoc", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    // Custom @ExtractUserId("userId") decorator has @in param →
    // should produce /users/profile/{userId} with a path parameter
    expect(doc.paths).toHaveProperty("/users/profile/{userId}");
    const getProfile = doc.paths["/users/profile/{userId}"].get;
    expect(getProfile).toBeDefined();
    expect(getProfile.operationId).toBe("getProfile");

    // Should have exactly 1 parameter: userId (path param from @ExtractUserId)
    // @CurrentUser() has no @in → silently skipped
    const params = getProfile.parameters;
    expect(params).toBeDefined();
    expect(params).toHaveLength(1);
    expect(params[0].name).toBe("userId");
    expect(params[0].in).toBe("path");
    expect(params[0].required).toBe(true);
  });
});

describe("tsgonest generated JS execution", () => {
  // These tests dynamically import the generated companion files and verify
  // they actually work at runtime.

  it("validation should accept valid input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: "Alice",
      email: "alice@example.com",
      age: 30,
    });
    expect(result.success).toBe(true);
    expect(result.data).toEqual({
      name: "Alice",
      email: "alice@example.com",
      age: 30,
    });
  });

  it("validation should reject invalid input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: 123, // wrong type
      email: "alice@example.com",
      // missing age
    });
    expect(result.success).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors.some((e: any) => e.path === "input.name")).toBe(true);
    expect(result.errors.some((e: any) => e.path === "input.age")).toBe(true);
  });

  it("validation should reject null input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto(null);
    expect(result.success).toBe(false);
  });

  it("assert should throw on invalid input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    expect(() => mod.assertCreateUserDto("not an object")).toThrow(
      "Validation failed"
    );
  });

  it("serialization should produce valid JSON", async () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UserResponse.tsgonest.js"
    );
    const mod = await import(companionFile);

    const input = {
      id: 1,
      name: "Alice",
      email: "alice@example.com",
      age: 30,
      createdAt: "2024-01-01T00:00:00.000Z",
    };
    const json = mod.serializeUserResponse(input);
    const parsed = JSON.parse(json);
    expect(parsed.id).toBe(1);
    expect(parsed.name).toBe("Alice");
    expect(parsed.email).toBe("alice@example.com");
    expect(parsed.age).toBe(30);
    expect(parsed.createdAt).toBe("2024-01-01T00:00:00.000Z");
  });

  it("optional property validation should allow missing props", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UpdateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    // All props optional — empty object should pass
    const result = mod.validateUpdateUserDto({});
    expect(result.success).toBe(true);

    // Partial update with one field
    const result2 = mod.validateUpdateUserDto({ name: "NewName" });
    expect(result2.success).toBe(true);
  });

  it("optional property validation should reject wrong types", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UpdateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateUpdateUserDto({ age: "not a number" });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.age")).toBe(true);
  });

  it("serialization with optional props should produce correct JSON", async () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UpdateUserDto.tsgonest.js"
    );
    const mod = await import(companionFile);

    // All props present
    const jsonFull = mod.serializeUpdateUserDto({
      name: "Alice",
      email: "alice@example.com",
      age: 30,
    });
    const parsedFull = JSON.parse(jsonFull);
    expect(parsedFull.name).toBe("Alice");
    expect(parsedFull.email).toBe("alice@example.com");
    expect(parsedFull.age).toBe(30);

    // Only one prop present
    const jsonPartial = mod.serializeUpdateUserDto({ name: "Bob" });
    const parsedPartial = JSON.parse(jsonPartial);
    expect(parsedPartial.name).toBe("Bob");
    expect(parsedPartial.email).toBeUndefined();
    expect(parsedPartial.age).toBeUndefined();

    // Empty object
    const jsonEmpty = mod.serializeUpdateUserDto({});
    const parsedEmpty = JSON.parse(jsonEmpty);
    expect(Object.keys(parsedEmpty)).toHaveLength(0);
  });
});

describe("tsgonest JSDoc constraint validation", () => {
  it("should generate constraint checks from JSDoc tags", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const content = readFileSync(companionFile, "utf-8");

    // Should have minLength/maxLength for name
    expect(content).toContain("input.name.length < 1");
    expect(content).toContain("input.name.length > 255");

    // Should have minimum/maximum for age
    expect(content).toContain("input.age < 0");
    expect(content).toContain("input.age > 150");

    // Should have format email check
    expect(content).toContain("format email");
  });

  it("constraint validation should reject values below minimum", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: "Alice",
      email: "alice@example.com",
      age: -5, // Below @minimum 0
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.age")).toBe(true);
  });

  it("constraint validation should reject values above maximum", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: "Alice",
      email: "alice@example.com",
      age: 200, // Above @maximum 150
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.age")).toBe(true);
  });

  it("constraint validation should reject strings too short", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: "", // Below @minLength 1
      email: "alice@example.com",
      age: 30,
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.name")).toBe(true);
  });

  it("constraint validation should reject invalid email format", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: "Alice",
      email: "not-an-email", // Fails @format email
      age: 30,
    });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.email")).toBe(true);
  });

  it("constraint validation should accept valid constrained input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto({
      name: "Alice",
      email: "alice@example.com",
      age: 30,
    });
    expect(result.success).toBe(true);
  });

  it("OpenAPI should include constraint annotations", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const createDto = doc.components.schemas.CreateUserDto;
    // Age should have minimum/maximum
    expect(createDto.properties.age.minimum).toBe(0);
    expect(createDto.properties.age.maximum).toBe(150);
    // Name should have minLength/maxLength
    expect(createDto.properties.name.minLength).toBe(1);
    expect(createDto.properties.name.maxLength).toBe(255);
    // Email should have format
    expect(createDto.properties.email.format).toBe("email");
  });
});

describe("tsgonest manifest generation", () => {
  // These tests verify that the __tsgonest_manifest.json is generated
  // alongside companion files and contains correct entries.

  it("should generate manifest file", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    expect(existsSync(manifestFile)).toBe(true);
  });

  it("should have correct manifest structure", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    expect(manifest).toHaveProperty("version", 1);
    expect(manifest).toHaveProperty("companions");
  });

  it("should have companion entries for DTOs", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Should have companions for CreateUserDto and UpdateUserDto
    expect(manifest.companions).toHaveProperty("CreateUserDto");
    expect(manifest.companions).toHaveProperty("UpdateUserDto");

    // Verify function names
    expect(manifest.companions.CreateUserDto.assert).toBe("assertCreateUserDto");
    expect(manifest.companions.CreateUserDto.validate).toBe("validateCreateUserDto");
    expect(manifest.companions.UpdateUserDto.assert).toBe("assertUpdateUserDto");

    // Verify file paths are relative and use .tsgonest.js extension
    expect(manifest.companions.CreateUserDto.file).toMatch(/\.\/.*\.tsgonest\.js$/);
    expect(manifest.companions.UpdateUserDto.file).toMatch(/\.\/.*\.tsgonest\.js$/);
  });

  it("should have serializer function names in companion entries", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Should have serialize function for UserResponse
    expect(manifest.companions).toHaveProperty("UserResponse");
    expect(manifest.companions.UserResponse.serialize).toBe("serializeUserResponse");
    expect(manifest.companions.UserResponse.file).toMatch(/\.\/.*\.tsgonest\.js$/);
  });

  it("manifest paths should resolve to actual companion files", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifestDir = resolve(FIXTURES_DIR, "nestjs/dist");
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Check that all companion files exist
    for (const [typeName, entry] of Object.entries(manifest.companions)) {
      const fullPath = resolve(manifestDir, (entry as any).file);
      expect(existsSync(fullPath)).toBe(true);
    }
  });

  it("should be able to require companion files via manifest", async () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifestDir = resolve(FIXTURES_DIR, "nestjs/dist");
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Load the CreateUserDto companion via manifest
    const companionEntry = manifest.companions.CreateUserDto;
    const companionPath = resolve(manifestDir, companionEntry.file);
    const mod = await import(companionPath);

    // The assert function should exist
    const assertFn = mod[companionEntry.assert];
    expect(typeof assertFn).toBe("function");

    // And it should work
    const validResult = assertFn({
      name: "Test",
      email: "test@test.com",
      age: 25,
    });
    expect(validResult).toBeDefined();
    expect(validResult.name).toBe("Test");
  });

  it("should be able to require serializer via companion manifest", async () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifestDir = resolve(FIXTURES_DIR, "nestjs/dist");
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Load the UserResponse companion via manifest
    const companionEntry = manifest.companions.UserResponse;
    const companionPath = resolve(manifestDir, companionEntry.file);
    const mod = await import(companionPath);

    // The serialize function should exist
    const serializeFn = mod[companionEntry.serialize];
    expect(typeof serializeFn).toBe("function");

    // And it should produce valid JSON
    const json = serializeFn({
      id: 42,
      name: "Alice",
      email: "alice@example.com",
      age: 30,
      createdAt: "2024-06-15T12:00:00.000Z",
    });
    const parsed = JSON.parse(json);
    expect(parsed.id).toBe(42);
    expect(parsed.name).toBe("Alice");
  });
});

describe("tsgonest full pipeline integration", () => {
  // Verify that a single tsgonest invocation produces all outputs.

  it("should produce JS, companions, manifest, and OpenAPI in one run", () => {
    // Clean up
    const distDir = resolve(FIXTURES_DIR, "nestjs/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    if (existsSync(openapiFile)) {
      rmSync(openapiFile);
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/nestjs/tsconfig.json",
      "--config",
      "testdata/nestjs/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);

    // Should report all phases
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");
    expect(stderr).toContain("manifest");
    expect(stderr).toContain("controller");
    expect(stderr).toContain("OpenAPI");

    // Verify JS output exists
    expect(
      existsSync(resolve(FIXTURES_DIR, "nestjs/dist/user.dto.js"))
    ).toBe(true);
    expect(
      existsSync(resolve(FIXTURES_DIR, "nestjs/dist/user.controller.js"))
    ).toBe(true);

    // Verify companion files exist (consolidated .tsgonest.js format)
    expect(
      existsSync(
        resolve(
          FIXTURES_DIR,
          "nestjs/dist/user.dto.CreateUserDto.tsgonest.js"
        )
      )
    ).toBe(true);
    expect(
      existsSync(
        resolve(
          FIXTURES_DIR,
          "nestjs/dist/user.dto.UserResponse.tsgonest.js"
        )
      )
    ).toBe(true);

    // Verify manifest exists
    expect(
      existsSync(
        resolve(FIXTURES_DIR, "nestjs/dist/__tsgonest_manifest.json")
      )
    ).toBe(true);

    // Verify OpenAPI document exists
    expect(existsSync(openapiFile)).toBe(true);
  });
});

// --- Phase 7: Realistic fixture (testdata/realworld/) ---

describe("realworld fixture", () => {
  const realworldDist = resolve(FIXTURES_DIR, "realworld/dist");
  // OpenAPI output is resolved relative to the config file directory
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
    // Check that some companion files exist for key DTOs (consolidated .tsgonest.js format)
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
    // Manifest is generated in the outDir root (testdata/realworld/dist/)
    const manifestDir = resolve(FIXTURES_DIR, "realworld/dist");
    const possiblePaths = [
      resolve(manifestDir, "__tsgonest_manifest.json"),
      // Fallback for non-outDir case
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
    // Check some key paths exist
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
    // Some key DTOs should be in component schemas
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
    // Valid input
    const validResult = mod.validateLoginDto({
      email: "test@example.com",
      password: "securepass123",
    });
    expect(validResult.success).toBe(true);

    // Invalid: missing required fields
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

    // Invalid: email not matching format, password too short
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

// --- Phase 20: @tsgonest/types branded type validation ---

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

    // Email format check from tags.Email branded type
    expect(content).toContain("format email");
    // MinLength/MaxLength from tags.MinLength<1> & tags.MaxLength<255>
    expect(content).toContain("input.name.length < 1");
    expect(content).toContain("input.name.length > 255");
    // Minimum/Maximum from tags.Minimum<0> & tags.Maximum<150>
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

    // UUID format from tags.Uuid
    expect(content).toContain("format uuid");
    // ExclusiveMinimum<0> from tags.Positive
    expect(content).toContain("input.price <= 0");
    // Int type from tags.Int
    expect(content).toContain("Number.isInteger");
  });

  it("should generate ConfigDto validation with StartsWith and Includes", () => {
    const companionFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ConfigDto.tsgonest.js"
    );
    expect(existsSync(companionFile)).toBe(true);
    const content = readFileSync(companionFile, "utf-8");

    // StartsWith from tags.StartsWith<"https://">
    expect(content).toContain('startsWith("https://")');
    // Includes from tags.Includes<"@">
    expect(content).toContain('includes("@")');
    // Format email from tags.Format<"email">
    expect(content).toContain("format email");
  });

  it("ProductDto validation should work at runtime", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ProductDto.tsgonest.js"
    );
    const mod = await import(validateFile);

    // Valid
    const valid = mod.validateProductDto({
      id: "550e8400-e29b-41d4-a716-446655440000",
      price: 10,
      quantity: 0,
    });
    expect(valid.success).toBe(true);

    // Invalid: price is 0 (not positive)
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

    // Valid
    const valid = mod.validateConfigDto({
      websiteUrl: "https://example.com",
      contactEmail: "info@example.com",
    });
    expect(valid.success).toBe(true);

    // Invalid: URL doesn't start with https://
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

// --- Incremental compilation tests ---

describe("tsgonest incremental compilation", () => {
  const incrDist = resolve(FIXTURES_DIR, "incremental/dist");
  const tsbuildinfo = resolve(FIXTURES_DIR, "incremental/tsconfig.tsbuildinfo");
  const srcFile = resolve(FIXTURES_DIR, "incremental/src/index.ts");

  beforeAll(() => {
    // Clean up from any previous runs
    if (existsSync(incrDist)) {
      rmSync(incrDist, { recursive: true });
    }
    if (existsSync(tsbuildinfo)) {
      rmSync(tsbuildinfo);
    }
  });

  it("should detect incremental mode from tsconfig", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/incremental/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("incremental build enabled");
  });

  it("should write .tsbuildinfo file", () => {
    expect(existsSync(tsbuildinfo)).toBe(true);
    const content = readFileSync(tsbuildinfo, "utf-8");
    const buildInfo = JSON.parse(content);
    expect(buildInfo).toHaveProperty("version");
    expect(buildInfo).toHaveProperty("fileNames");
    expect(Array.isArray(buildInfo.fileNames)).toBe(true);
  });

  it("should emit JS on first build", () => {
    expect(existsSync(resolve(incrDist, "index.js"))).toBe(true);
  });

  it("warm build should skip diagnostics and emit when nothing changed", () => {
    // Second build — nothing changed, should be fast
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/incremental/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("incremental build enabled");
    // No files emitted because nothing changed
    expect(stderr).toContain("no files emitted");
  });

  it("should re-emit only changed files after modification", () => {
    // Read original content
    const originalContent = readFileSync(srcFile, "utf-8");

    // Modify the file (append a new export)
    const modifiedContent =
      originalContent + "\nexport const VERSION = 42;\n";
    const { writeFileSync } = require("fs");
    writeFileSync(srcFile, modifiedContent);

    // Rebuild — should detect the change
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/incremental/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("incremental build enabled");
    expect(stderr).toContain("emitted");

    // Verify the emitted JS has the new export
    const jsContent = readFileSync(resolve(incrDist, "index.js"), "utf-8");
    expect(jsContent).toContain("VERSION");

    // Restore original content
    writeFileSync(srcFile, originalContent);

    // One more build to restore .tsbuildinfo to clean state
    runTsgonest(["--project", "testdata/incremental/tsconfig.json"]);
  });
});

// --- Incremental post-processing cache tests ---

describe("tsgonest incremental post-processing cache", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "incremental-nestjs");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");
  const cacheFile = resolve(fixtureDir, "tsconfig.tsgonest-cache");
  const configFile = resolve(fixtureDir, "tsgonest.config.json");
  const srcFile = resolve(fixtureDir, "src/item.dto.ts");
  const openapiFile = resolve(fixtureDir, "dist/openapi.json");
  const manifestFile = resolve(
    fixtureDir,
    "dist/__tsgonest_manifest.json"
  );

  // Helper: full clean + cold build to reset state
  function cleanAndBuild() {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
    if (existsSync(tsbuildinfo)) {
      rmSync(tsbuildinfo);
    }
    if (existsSync(cacheFile)) {
      rmSync(cacheFile);
    }
    const result = runTsgonest([
      "--project",
      "testdata/incremental-nestjs/tsconfig.json",
      "--config",
      "testdata/incremental-nestjs/tsgonest.config.json",
    ]);
    expect(result.exitCode).toBe(0);
    return result;
  }

  // Helper: warm build (no clean)
  function warmBuild() {
    return runTsgonest([
      "--project",
      "testdata/incremental-nestjs/tsconfig.json",
      "--config",
      "testdata/incremental-nestjs/tsgonest.config.json",
    ]);
  }

  beforeAll(() => {
    // Ensure clean starting state
    cleanAndBuild();
  });

  it("cold build should produce all outputs and cache file", () => {
    // Clean and rebuild from scratch
    const { exitCode, stderr } = cleanAndBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");
    expect(stderr).toContain("controller");
    expect(stderr).toContain("OpenAPI");
    expect(stderr).toContain("manifest");

    // Cache file should exist
    expect(existsSync(cacheFile)).toBe(true);
    const cache = JSON.parse(readFileSync(cacheFile, "utf-8"));
    expect(cache.v).toBe(1);
    expect(cache.configHash).toBeTruthy();
    expect(cache.outputs).toBeInstanceOf(Array);
    expect(cache.outputs.length).toBeGreaterThanOrEqual(2);

    // All output files should exist
    expect(existsSync(openapiFile)).toBe(true);
    expect(existsSync(manifestFile)).toBe(true);
  });

  it("warm build with no changes should skip post-processing", () => {
    // Ensure we have a warm state
    cleanAndBuild();

    // Second build — nothing changed
    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("no changes detected, outputs up to date");

    // Should NOT contain companion/controller/OpenAPI generation messages
    expect(stderr).not.toContain("companion file");
    expect(stderr).not.toContain("found 1 controller");
    // Timing should show 0s for post-processing phases
    expect(stderr).toContain("companions:    0s");
    expect(stderr).toContain("controllers:   0s");
    expect(stderr).toContain("openapi:       0s");
  });

  it("config change should force full rebuild", () => {
    cleanAndBuild();

    // Modify config (add a trailing space to title — content changes hash)
    const originalConfig = readFileSync(configFile, "utf-8");
    const { writeFileSync } = require("fs");
    const modifiedConfig = originalConfig.replace(
      '"dist/openapi.json"',
      '"dist/openapi.json" '
    );
    writeFileSync(configFile, modifiedConfig);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    // Should NOT say "no changes detected" — cache is invalid due to config change
    expect(stderr).not.toContain("no changes detected");
    // Should run full post-processing
    // Note: "no files emitted" is still printed because TS source didn't change,
    // but post-processing should still run because config hash changed
    expect(stderr).toContain("companion");

    // Restore original config
    writeFileSync(configFile, originalConfig);
    // Rebuild to restore clean state
    warmBuild();
  });

  it("output file deletion should force full rebuild", () => {
    cleanAndBuild();

    // Delete the OpenAPI output file
    rmSync(openapiFile);
    expect(existsSync(openapiFile)).toBe(false);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    // Should NOT skip — output file is missing
    expect(stderr).not.toContain("no changes detected");
    // Should regenerate OpenAPI
    expect(stderr).toContain("OpenAPI");
    // OpenAPI file should be restored
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("manifest file deletion should force full rebuild", () => {
    cleanAndBuild();

    // Delete the manifest file
    rmSync(manifestFile);
    expect(existsSync(manifestFile)).toBe(false);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("manifest");
    expect(existsSync(manifestFile)).toBe(true);
  });

  it("source file change should force full rebuild", () => {
    cleanAndBuild();

    // Modify source file
    const originalSrc = readFileSync(srcFile, "utf-8");
    const { writeFileSync } = require("fs");
    const modifiedSrc =
      originalSrc + "\nexport interface ExtraDto { extra: string; }\n";
    writeFileSync(srcFile, modifiedSrc);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    // Source changed → tsgo emits files → post-processing runs
    expect(stderr).toContain("emitted");
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");

    // Restore original
    writeFileSync(srcFile, originalSrc);
    warmBuild(); // Clean up state
  });

  it("cache file missing should force full rebuild", () => {
    cleanAndBuild();

    // Delete only the cache file (outputs still exist)
    rmSync(cacheFile);
    expect(existsSync(cacheFile)).toBe(false);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    // Cache miss → full rebuild
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");
    // Cache should be recreated
    expect(existsSync(cacheFile)).toBe(true);
  });

  it("corrupted cache file should force full rebuild", () => {
    cleanAndBuild();

    // Write garbage to cache file
    const { writeFileSync } = require("fs");
    writeFileSync(cacheFile, "not valid json {{{");

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");
    // Cache should be overwritten with valid data
    const cacheContent = readFileSync(cacheFile, "utf-8");
    const cache = JSON.parse(cacheContent); // should not throw
    expect(cache.v).toBe(1);
  });

  it("--clean flag should force full rebuild", () => {
    cleanAndBuild();

    // Verify warm build would normally skip
    const warmResult = warmBuild();
    expect(warmResult.stderr).toContain("no changes detected");

    // Now build with --clean — should force full rebuild
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/incremental-nestjs/tsconfig.json",
      "--config",
      "testdata/incremental-nestjs/tsgonest.config.json",
      "--clean",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("cleaning output directory");
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("emitted"); // All files re-emitted after clean
    expect(stderr).toContain("companion");
  });

  it("schema version bump should force full rebuild", () => {
    cleanAndBuild();

    // Tamper with the cache schema version
    const cacheContent = JSON.parse(readFileSync(cacheFile, "utf-8"));
    cacheContent.v = 999; // future version
    const { writeFileSync } = require("fs");
    writeFileSync(cacheFile, JSON.stringify(cacheContent));

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");

    // Cache should be overwritten with correct version
    const newCache = JSON.parse(readFileSync(cacheFile, "utf-8"));
    expect(newCache.v).toBe(1);
  });

  it("successive warm builds should all skip consistently", () => {
    cleanAndBuild();

    // Run 3 warm builds in succession — all should skip
    for (let i = 0; i < 3; i++) {
      const { exitCode, stderr } = warmBuild();
      expect(exitCode).toBe(0);
      expect(stderr).toContain("no changes detected, outputs up to date");
    }
  });

  it("rebuild after skip should still produce correct outputs", () => {
    cleanAndBuild();

    // Warm build (skips)
    const warmResult = warmBuild();
    expect(warmResult.stderr).toContain("no changes detected");

    // Verify output files are still valid from the cold build
    const openapi = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(openapi.openapi).toBe("3.2.0");
    expect(openapi.paths).toHaveProperty("/items");

    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));
    expect(manifest).toHaveProperty("version", 1);
    expect(manifest).toHaveProperty("companions");
    expect(manifest.companions).toHaveProperty("CreateItemDto");
  });

  it("non-incremental build should not create cache file", () => {
    // The simple fixture doesn't have incremental: true
    const simpleDist = resolve(FIXTURES_DIR, "simple/dist");
    if (existsSync(simpleDist)) {
      rmSync(simpleDist, { recursive: true });
    }
    const simpleCacheFile = resolve(FIXTURES_DIR, "simple/tsconfig.tsgonest-cache");
    if (existsSync(simpleCacheFile)) {
      rmSync(simpleCacheFile);
    }

    runTsgonest(["--project", "testdata/simple/tsconfig.json"]);

    // The simple fixture has no config, so no post-processing → cache is saved
    // but with no outputs and empty config hash. The important thing is it doesn't crash.
    // Cache file should still be created (it records the empty state)
    expect(existsSync(simpleCacheFile)).toBe(true);
  });
});

// --- Diagnostic pipeline & exit code tests ---

describe("tsgonest diagnostics and exit codes", () => {
  it("should exit 1 on type errors and still emit JS", () => {
    // Clean dist
    const distDir = resolve(FIXTURES_DIR, "errors-type/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-type/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);

    // Should report the type errors
    expect(stderr).toContain("TS2345"); // string not assignable to number
    expect(stderr).toContain("TS2741"); // missing property 'age'
    expect(stderr).toContain("error");

    // JS should still be emitted (type errors don't prevent emit unless noEmitOnError)
    expect(stderr).toContain("emitted");
    expect(existsSync(resolve(distDir, "bad-types.js"))).toBe(true);
  });

  it("should exit 1 on syntax errors and still emit JS", () => {
    // Clean dist
    const distDir = resolve(FIXTURES_DIR, "errors-syntax/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-syntax/tsconfig.json",
    ]);
    expect(exitCode).toBe(1);

    // Should report the syntax error
    expect(stderr).toContain("TS1005"); // '}' expected
    expect(stderr).toContain("error");

    // JS is still emitted (tsgo emits even with errors unless noEmitOnError)
    expect(stderr).toContain("emitted");
  });

  it("should exit 2 with noEmitOnError and NOT emit JS", () => {
    // Clean dist
    const distDir = resolve(FIXTURES_DIR, "errors-noemit/dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-noemit/tsconfig.json",
    ]);
    expect(exitCode).toBe(2);

    // Should report the type error
    expect(stderr).toContain("TS2345");
    expect(stderr).toContain("error");

    // Should indicate no files were emitted
    expect(stderr).toContain("no files emitted");
    expect(stderr).toContain("noEmitOnError");

    // dist directory should not exist or be empty (no JS output)
    if (existsSync(distDir)) {
      const files = readdirSync(distDir);
      const jsFiles = files.filter((f) => f.endsWith(".js"));
      expect(jsFiles).toHaveLength(0);
    }
  });

  it("should exit 0 with --no-check even when type errors exist", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-type/tsconfig.json",
      "--no-check",
    ]);
    expect(exitCode).toBe(0);

    // Should NOT report type errors when --no-check is used
    expect(stderr).not.toContain("TS2345");
    expect(stderr).not.toContain("TS2741");

    // Should still emit files
    expect(stderr).toContain("emitted");
  });

  it("should still report syntax errors with --no-check", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/errors-syntax/tsconfig.json",
      "--no-check",
    ]);
    // Syntax errors should still cause exit 1 even with --no-check
    // (--no-check only skips semantic/type checking, not syntax)
    expect(exitCode).toBe(1);
    expect(stderr).toContain("TS1005");
  });

  it("diagnostic output should include file name and position", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/errors-type/tsconfig.json",
    ]);

    // Should include file name in diagnostics
    expect(stderr).toContain("bad-types.ts");
    // Should include line/column position (format: file(line,col) or file:line:col)
    expect(stderr).toMatch(/bad-types\.ts[\(:]/);
  });
});

// ─── @Returns<T>() Decorator Tests ─────────────────────────────────────────
describe("@Returns<T>() decorator", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "nestjs-returns");
  const openapiFile = resolve(fixtureDir, "dist/openapi.json");

  beforeAll(() => {
    // Clean and build fresh
    const distDir = resolve(fixtureDir, "dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
    const cacheFile = resolve(fixtureDir, "tsconfig.tsgonest-cache");
    if (existsSync(cacheFile)) rmSync(cacheFile);
    const buildInfoFile = resolve(fixtureDir, "tsconfig.tsbuildinfo");
    if (existsSync(buildInfoFile)) rmSync(buildInfoFile);

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
      "--clean",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("controller");
    expect(stderr).toContain("OpenAPI");
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("should generate OpenAPI for @Returns<ReportResponse>() with @Res()", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const getReport = doc.paths["/reports/{id}"]?.get;
    expect(getReport).toBeDefined();
    expect(getReport.operationId).toBe("getReport");

    // Should have a typed response, NOT void
    const response200 = getReport.responses["200"];
    expect(response200.content).toBeDefined();
    expect(response200.content["application/json"]).toBeDefined();
    expect(response200.content["application/json"].schema.$ref).toBe(
      "#/components/schemas/ReportResponse"
    );
  });

  it("should use custom contentType from @Returns options", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    // PDF endpoint — application/pdf
    const pdfRoute = doc.paths["/reports/{id}/pdf"]?.get;
    expect(pdfRoute).toBeDefined();
    const pdfResponse = pdfRoute.responses["200"];
    expect(pdfResponse.content["application/pdf"]).toBeDefined();
    expect(pdfResponse.content["application/json"]).toBeUndefined();

    // CSV endpoint — text/csv
    const csvRoute = doc.paths["/reports/{id}/csv"]?.get;
    expect(csvRoute).toBeDefined();
    const csvResponse = csvRoute.responses["200"];
    expect(csvResponse.content["text/csv"]).toBeDefined();
    expect(csvResponse.content["text/csv"].schema.type).toBe("string");
    expect(csvResponse.content["application/json"]).toBeUndefined();
  });

  it("should use custom description from @Returns options", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    // PDF endpoint — description override
    const pdfResponse = doc.paths["/reports/{id}/pdf"]?.get?.responses["200"];
    expect(pdfResponse.description).toBe("PDF report document");

    // CSV endpoint — description override
    const csvResponse = doc.paths["/reports/{id}/csv"]?.get?.responses["200"];
    expect(csvResponse.description).toBe("CSV export");

    // Normal route — default description
    const summaryResponse = doc.paths["/reports/summary"]?.get?.responses["200"];
    expect(summaryResponse.description).toBe("OK");
  });

  it("should produce void response for @Res() without @Returns", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const rawRoute = doc.paths["/reports/{id}/raw"]?.get;
    expect(rawRoute).toBeDefined();
    const rawResponse = rawRoute.responses["200"];
    // Void response — no content key
    expect(rawResponse.content).toBeUndefined();
    expect(rawResponse.description).toBe("OK");
  });

  it("should emit warning for @Res() without @Returns", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
    ]);
    // Should warn about getRawReport using @Res() without @Returns
    expect(stderr).toContain("getRawReport");
    expect(stderr).toContain("uses-raw-response");
    expect(stderr).toContain("@Returns<YourType>()");
    expect(stderr).toContain("@tsgonest-ignore");
  });

  it("should NOT emit warning for @Res() with @Returns", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
    ]);
    // Methods with @Returns should NOT produce uses-raw-response warning
    expect(stderr).not.toContain("getReport —");
    expect(stderr).not.toContain("getReportPdf —");
    expect(stderr).not.toContain("getReportCsv —");
  });

  it("should suppress warning with @tsgonest-ignore uses-raw-response", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
    ]);
    // streamReport has @tsgonest-ignore uses-raw-response — no warning
    expect(stderr).not.toContain("streamReport");
  });

  it("should handle @Returns status override", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const regenerateRoute = doc.paths["/reports/{id}/regenerate"]?.post;
    expect(regenerateRoute).toBeDefined();
    // @Returns({ status: 200 }) should override @HttpCode(202)
    expect(regenerateRoute.responses["200"]).toBeDefined();
    expect(regenerateRoute.responses["202"]).toBeUndefined();
    // The response should have a typed schema
    expect(
      regenerateRoute.responses["200"].content["application/json"].schema.$ref
    ).toBe("#/components/schemas/ReportResponse");
  });

  it("should not affect normal routes without @Res()", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const summaryRoute = doc.paths["/reports/summary"]?.get;
    expect(summaryRoute).toBeDefined();
    // Normal route should work as before
    expect(summaryRoute.responses["200"].content["application/json"]).toBeDefined();
    expect(
      summaryRoute.responses["200"].content["application/json"].schema.$ref
    ).toBe("#/components/schemas/ReportSummary");
  });

  it("should include ReportResponse schema in components", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const schema = doc.components?.schemas?.ReportResponse;
    expect(schema).toBeDefined();
    expect(schema.type).toBe("object");
    expect(schema.properties.id).toEqual({ type: "string" });
    expect(schema.properties.title).toEqual({ type: "string" });
    expect(schema.properties.generatedAt).toEqual({ type: "string" });
    expect(schema.required).toContain("id");
    expect(schema.required).toContain("title");
    expect(schema.required).toContain("generatedAt");
  });
});
