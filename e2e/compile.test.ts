import { describe, it, expect, beforeAll } from "vitest";
import { spawnSync } from "child_process";
import { existsSync, readFileSync, rmSync } from "fs";
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

  it("should generate validation companion files", () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
    );
    expect(existsSync(validateFile)).toBe(true);

    const content = readFileSync(validateFile, "utf-8");
    expect(content).toContain("export function validateCreateUserDto");
    expect(content).toContain("export function assertCreateUserDto");
    expect(content).toContain("export function deserializeCreateUserDto");
    expect(content).toContain('typeof input.name !== "string"');
    expect(content).toContain('typeof input.age !== "number"');
  });

  it("should generate serialization companion files", () => {
    const serializeFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UserResponse.serialize.js"
    );
    expect(existsSync(serializeFile)).toBe(true);

    const content = readFileSync(serializeFile, "utf-8");
    expect(content).toContain("export function serializeUserResponse");
    expect(content).toContain("__jsonStr");
  });

  it("should handle optional properties in UpdateUserDto", () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UpdateUserDto.validate.js"
    );
    expect(existsSync(validateFile)).toBe(true);

    const content = readFileSync(validateFile, "utf-8");
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
    expect(doc.openapi).toBe("3.1.0");
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateCreateUserDto(null);
    expect(result.success).toBe(false);
  });

  it("assert should throw on invalid input", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
    );
    const mod = await import(validateFile);

    expect(() => mod.assertCreateUserDto("not an object")).toThrow(
      "Validation failed"
    );
  });

  it("deserialize should parse and validate JSON", async () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
    );
    const mod = await import(validateFile);

    const json = JSON.stringify({
      name: "Bob",
      email: "bob@example.com",
      age: 25,
    });
    const result = mod.deserializeCreateUserDto(json);
    expect(result).toEqual({ name: "Bob", email: "bob@example.com", age: 25 });
  });

  it("serialization should produce valid JSON", async () => {
    const serializeFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UserResponse.serialize.js"
    );
    const mod = await import(serializeFile);

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
      "nestjs/dist/user.dto.UpdateUserDto.validate.js"
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
      "nestjs/dist/user.dto.UpdateUserDto.validate.js"
    );
    const mod = await import(validateFile);

    const result = mod.validateUpdateUserDto({ age: "not a number" });
    expect(result.success).toBe(false);
    expect(result.errors.some((e: any) => e.path === "input.age")).toBe(true);
  });

  it("serialization with optional props should produce correct JSON", async () => {
    const serializeFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.UpdateUserDto.serialize.js"
    );
    const mod = await import(serializeFile);

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
    const validateFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
    );
    const content = readFileSync(validateFile, "utf-8");

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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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
      "nestjs/dist/user.dto.CreateUserDto.validate.js"
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

    expect(manifest).toHaveProperty("validators");
    expect(manifest).toHaveProperty("serializers");
  });

  it("should have validator entries for DTOs", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Should have validators for CreateUserDto and UpdateUserDto
    expect(manifest.validators).toHaveProperty("CreateUserDto");
    expect(manifest.validators).toHaveProperty("UpdateUserDto");

    // Verify function names
    expect(manifest.validators.CreateUserDto.fn).toBe("assertCreateUserDto");
    expect(manifest.validators.UpdateUserDto.fn).toBe("assertUpdateUserDto");

    // Verify file paths are relative
    expect(manifest.validators.CreateUserDto.file).toMatch(/\.\/.*\.validate\.js$/);
    expect(manifest.validators.UpdateUserDto.file).toMatch(/\.\/.*\.validate\.js$/);
  });

  it("should have serializer entries for response DTOs", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Should have serializer for UserResponse
    expect(manifest.serializers).toHaveProperty("UserResponse");
    expect(manifest.serializers.UserResponse.fn).toBe("serializeUserResponse");
    expect(manifest.serializers.UserResponse.file).toMatch(/\.\/.*\.serialize\.js$/);
  });

  it("manifest paths should resolve to actual companion files", () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifestDir = resolve(FIXTURES_DIR, "nestjs/dist");
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Check that all validator files exist
    for (const [typeName, entry] of Object.entries(manifest.validators)) {
      const fullPath = resolve(manifestDir, (entry as any).file);
      expect(existsSync(fullPath)).toBe(true);
    }

    // Check that all serializer files exist
    for (const [typeName, entry] of Object.entries(manifest.serializers)) {
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

    // Load the CreateUserDto validator via manifest
    const validatorEntry = manifest.validators.CreateUserDto;
    const validatorPath = resolve(manifestDir, validatorEntry.file);
    const mod = await import(validatorPath);

    // The assert function should exist
    const assertFn = mod[validatorEntry.fn];
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

  it("should be able to require serializer files via manifest", async () => {
    const manifestFile = resolve(
      FIXTURES_DIR,
      "nestjs/dist/__tsgonest_manifest.json"
    );
    const manifestDir = resolve(FIXTURES_DIR, "nestjs/dist");
    const manifest = JSON.parse(readFileSync(manifestFile, "utf-8"));

    // Load the UserResponse serializer via manifest
    const serializerEntry = manifest.serializers.UserResponse;
    const serializerPath = resolve(manifestDir, serializerEntry.file);
    const mod = await import(serializerPath);

    // The serialize function should exist
    const serializeFn = mod[serializerEntry.fn];
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

    // Verify companion files exist
    expect(
      existsSync(
        resolve(
          FIXTURES_DIR,
          "nestjs/dist/user.dto.CreateUserDto.validate.js"
        )
      )
    ).toBe(true);
    expect(
      existsSync(
        resolve(
          FIXTURES_DIR,
          "nestjs/dist/user.dto.UserResponse.serialize.js"
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
    // Check that some companion files exist for key DTOs
    const companions = [
      "realworld/dist/auth/auth.dto.LoginDto.validate.js",
      "realworld/dist/auth/auth.dto.RegisterDto.validate.js",
      "realworld/dist/auth/auth.dto.AuthTokenResponse.serialize.js",
      "realworld/dist/user/user.dto.UserResponse.serialize.js",
      "realworld/dist/article/article.dto.CreateArticleDto.validate.js",
      "realworld/dist/article/article.dto.Comment.validate.js",
      "realworld/dist/payment/payment.dto.PaymentResponse.serialize.js",
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
    expect(doc.openapi).toBe("3.1.0");
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
      "realworld/dist/auth/auth.dto.LoginDto.validate.js"
    );
    if (!existsSync(validatePath)) {
      throw new Error(`validate file not found: ${validatePath}`);
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
      "realworld/dist/auth/auth.dto.LoginDto.validate.js"
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

    const serializePath = resolve(
      FIXTURES_DIR,
      "realworld/dist/auth/auth.dto.AuthTokenResponse.serialize.js"
    );
    if (!existsSync(serializePath)) {
      throw new Error(`serialize file not found: ${serializePath}`);
    }

    const mod = require(serializePath);
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
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.CreateUserDto.validate.js"
    );
    expect(existsSync(validateFile)).toBe(true);
    const content = readFileSync(validateFile, "utf-8");

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
      "branded/dist/user.dto.CreateUserDto.validate.js"
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
      "branded/dist/user.dto.CreateUserDto.validate.js"
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
      "branded/dist/user.dto.CreateUserDto.validate.js"
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
      "branded/dist/user.dto.CreateUserDto.validate.js"
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
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ProductDto.validate.js"
    );
    expect(existsSync(validateFile)).toBe(true);
    const content = readFileSync(validateFile, "utf-8");

    // UUID format from tags.Uuid
    expect(content).toContain("format uuid");
    // ExclusiveMinimum<0> from tags.Positive
    expect(content).toContain("input.price <= 0");
    // Int type from tags.Int
    expect(content).toContain("Number.isInteger");
  });

  it("should generate ConfigDto validation with StartsWith and Includes", () => {
    const validateFile = resolve(
      FIXTURES_DIR,
      "branded/dist/user.dto.ConfigDto.validate.js"
    );
    expect(existsSync(validateFile)).toBe(true);
    const content = readFileSync(validateFile, "utf-8");

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
      "branded/dist/user.dto.ProductDto.validate.js"
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
      "branded/dist/user.dto.ConfigDto.validate.js"
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
