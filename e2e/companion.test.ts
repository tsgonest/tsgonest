import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

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
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("should produce valid OpenAPI 3.1 document", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const content = readFileSync(openapiFile, "utf-8");
    const doc = JSON.parse(content);

    expect(doc.openapi).toBe("3.2.0");
    expect(doc.info).toBeDefined();
    expect(doc.info.title).toBe("API");
    expect(doc.info.version).toBe("1.0.0");
  });

  it("should have correct paths in OpenAPI document", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    expect(doc.paths).toHaveProperty("/users");
    expect(doc.paths).toHaveProperty("/users/{id}");

    expect(doc.paths["/users"].get).toBeDefined();
    expect(doc.paths["/users"].post).toBeDefined();
    expect(doc.paths["/users"].get.operationId).toBe("findAll");
    expect(doc.paths["/users"].post.operationId).toBe("create");

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

    expect(doc.components.schemas).toHaveProperty("CreateUserDto");
    expect(doc.components.schemas).toHaveProperty("UpdateUserDto");
    expect(doc.components.schemas).toHaveProperty("UserResponse");

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

    const getAll = doc.paths["/users"].get;
    const response200 = getAll.responses["200"];
    expect(response200).toBeDefined();
    expect(response200.content["application/json"].schema.type).toBe("array");
    expect(response200.content["application/json"].schema.items.$ref).toBe(
      "#/components/schemas/UserResponse"
    );

    const post = doc.paths["/users"].post;
    expect(post.responses["201"]).toBeDefined();
    expect(
      post.responses["201"].content["application/json"].schema.$ref
    ).toBe("#/components/schemas/UserResponse");

    const deleteOp = doc.paths["/users/{id}"].delete;
    expect(deleteOp.responses["204"]).toBeDefined();
    expect(deleteOp.responses["204"].content).toBeUndefined();
  });

  it("should have query parameters decomposed from ListQuery", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const getAll = doc.paths["/users"].get;
    expect(getAll.parameters).toBeDefined();
    expect(getAll.parameters.length).toBeGreaterThanOrEqual(3);

    const paramNames = getAll.parameters.map((p: any) => p.name);
    expect(paramNames).toContain("page");
    expect(paramNames).toContain("limit");
    expect(paramNames).toContain("search");

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

    expect(doc.paths["/users"].get.tags).toContain("User");
    expect(doc.paths["/users"].post.tags).toContain("User");
    expect(doc.paths["/users/{id}"].get.tags).toContain("User");
  });

  it("should resolve custom decorators with @in JSDoc", () => {
    const openapiFile = resolve(FIXTURES_DIR, "nestjs/dist/openapi.json");
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    expect(doc.paths).toHaveProperty("/users/profile/{userId}");
    const getProfile = doc.paths["/users/profile/{userId}"].get;
    expect(getProfile).toBeDefined();
    expect(getProfile.operationId).toBe("getProfile");

    const params = getProfile.parameters;
    expect(params).toBeDefined();
    expect(params).toHaveLength(1);
    expect(params[0].name).toBe("userId");
    expect(params[0].in).toBe("path");
    expect(params[0].required).toBe(true);
  });
});

describe("tsgonest generated JS execution", () => {
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
      name: 123,
      email: "alice@example.com",
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
      "Validation failed: 1 error(s)"
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

    const result = mod.validateUpdateUserDto({});
    expect(result.success).toBe(true);

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

    const jsonFull = mod.serializeUpdateUserDto({
      name: "Alice",
      email: "alice@example.com",
      age: 30,
    });
    const parsedFull = JSON.parse(jsonFull);
    expect(parsedFull.name).toBe("Alice");
    expect(parsedFull.email).toBe("alice@example.com");
    expect(parsedFull.age).toBe(30);

    const jsonPartial = mod.serializeUpdateUserDto({ name: "Bob" });
    const parsedPartial = JSON.parse(jsonPartial);
    expect(parsedPartial.name).toBe("Bob");
    expect(parsedPartial.email).toBeUndefined();
    expect(parsedPartial.age).toBeUndefined();

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

    expect(content).toContain("input.name.length < 1");
    expect(content).toContain("input.name.length > 255");
    expect(content).toContain("input.age < 0");
    expect(content).toContain("input.age > 150");
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
      age: -5,
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
      age: 200,
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
      name: "",
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
      email: "not-an-email",
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
    expect(createDto.properties.age.minimum).toBe(0);
    expect(createDto.properties.age.maximum).toBe(150);
    expect(createDto.properties.name.minLength).toBe(1);
    expect(createDto.properties.name.maxLength).toBe(255);
    expect(createDto.properties.email.format).toBe("email");
  });
});
