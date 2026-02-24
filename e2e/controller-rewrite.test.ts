import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest controller auto-rewriting", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "rewrite-controllers");
  const distDir = resolve(fixtureDir, "dist");

  beforeAll(() => {
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should build successfully with controller rewriting", () => {
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/rewrite-controllers/tsconfig.json",
      "--config",
      "testdata/rewrite-controllers/tsgonest.config.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("controller");
  });

  it("should inject @Body() validation into create method", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    expect(existsSync(controllerFile)).toBe(true);

    const content = readFileSync(controllerFile, "utf-8");

    // Should have assertCreateUserDto injected in create method
    expect(content).toContain("assertCreateUserDto(body)");
  });

  it("should inject @Body() validation into update method", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // Should have assertUpdateUserDto injected in update method
    expect(content).toContain("assertUpdateUserDto(body)");
  });

  it("should not inject validation into methods without @Body()", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // findAll and remove don't have @Body(), so no assert injection there
    // The content should not have arbitrary assert calls
    expect(content).not.toContain("assertUserResponse");
  });

  it("should add companion imports to controller file", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // Should have companion imports at the top
    expect(content).toContain("assertCreateUserDto");
    expect(content).toContain("assertUpdateUserDto");
    expect(content).toContain(".tsgonest.js");
  });

  it("should generate companion files for DTOs", () => {
    const createCompanion = resolve(
      distDir,
      "user.dto.CreateUserDto.tsgonest.js"
    );
    const updateCompanion = resolve(
      distDir,
      "user.dto.UpdateUserDto.tsgonest.js"
    );

    expect(existsSync(createCompanion)).toBe(true);
    expect(existsSync(updateCompanion)).toBe(true);

    const createContent = readFileSync(createCompanion, "utf-8");
    expect(createContent).toContain("export function assertCreateUserDto");

    const updateContent = readFileSync(updateCompanion, "utf-8");
    expect(updateContent).toContain("export function assertUpdateUserDto");
  });

  it("should add transformUserResponse import to controller", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // Should have transformUserResponse imported for return type wrapping
    expect(content).toContain("transformUserResponse");
  });

  it("should wrap return statements with transform call", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // create and update methods return UserResponse — should be wrapped
    expect(content).toContain("transformUserResponse(await");
  });

  it("should wrap array returns with .map()", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // findAll returns UserResponse[] — should use .map()
    expect(content).toContain(".map(_v => transformUserResponse(_v))");
  });

  it("should not wrap void methods", () => {
    const controllerFile = resolve(distDir, "user.controller.js");
    const content = readFileSync(controllerFile, "utf-8");

    // remove method returns void — should NOT be wrapped with transform
    // Count how many times transformUserResponse appears
    // It should NOT appear inside the remove method
    const lines = content.split("\n");
    const removeMethodIndex = lines.findIndex((l) =>
      l.includes("remove(")
    );
    if (removeMethodIndex >= 0) {
      // Check the next few lines don't have transform
      const removeBody = lines.slice(removeMethodIndex, removeMethodIndex + 5).join("\n");
      expect(removeBody).not.toContain("transformUserResponse");
    }
  });

  it("should generate transform functions in companion files", () => {
    const responseCompanion = resolve(
      distDir,
      "user.dto.UserResponse.tsgonest.js"
    );

    expect(existsSync(responseCompanion)).toBe(true);

    const content = readFileSync(responseCompanion, "utf-8");
    expect(content).toContain("export function transformUserResponse");
  });
});
