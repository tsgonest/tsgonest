import { describe, it, expect } from "vitest";
import {
  existsSync,
  readFileSync,
  rmSync,
  writeFileSync,
  mkdtempSync,
  mkdirSync,
  cpSync,
} from "fs";
import { tmpdir } from "os";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest migrate", () => {
  it("should print migrate help", () => {
    const { stdout, exitCode } = runTsgonest(["migrate", "--help"]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("tsgonest migrate");
    expect(stdout).toContain("--apply");
    expect(stdout).toContain("--include");
  });

  it("should dry-run and detect class-validator + swagger in fixture", () => {
    const { stdout, exitCode } = runTsgonest([
      "migrate",
      "--force",
      "--cwd",
      resolve(FIXTURES_DIR, "migrate"),
      "--include",
      "src/**/*.ts",
    ]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("dry-run");
    expect(stdout).toContain("class-validator");
    expect(stdout).toContain("@nestjs/swagger");
  });

  it("should show diff for DTO files", () => {
    const { stdout, exitCode } = runTsgonest([
      "migrate",
      "--force",
      "--cwd",
      resolve(FIXTURES_DIR, "migrate"),
      "--include",
      "src/**/*.ts",
    ]);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("user.dto.ts");
    expect(stdout).toContain("users.controller.ts");
  });

  it("should apply changes and write files", () => {
    const tempDir = mkdtempSync(resolve(tmpdir(), "tsgonest-migrate-"));
    cpSync(resolve(FIXTURES_DIR, "migrate"), tempDir, { recursive: true });

    const { stdout, exitCode } = runTsgonest([
      "migrate",
      "--apply",
      "--force",
      "--yes",
      "--cwd",
      tempDir,
      "--include",
      "src/**/*.ts",
    ]);
    expect(exitCode).toBe(0);

    const dtoContent = readFileSync(
      resolve(tempDir, "src/user.dto.ts"),
      "utf-8"
    );
    expect(dtoContent).toContain("interface CreateUserDto");
    expect(dtoContent).not.toContain("class CreateUserDto");
    expect(dtoContent).toContain("tags.Email");
    expect(dtoContent).toContain("tags.MinLength");
    expect(dtoContent).not.toContain("class-validator");
    expect(dtoContent).not.toContain("@ApiProperty");

    const ctrlContent = readFileSync(
      resolve(tempDir, "src/users.controller.ts"),
      "utf-8"
    );
    expect(ctrlContent).not.toContain("@ApiTags");
    expect(ctrlContent).not.toContain("@ApiOkResponse");
    expect(ctrlContent).toContain("@Controller");
    expect(ctrlContent).toContain("@Get");

    const reportPath = resolve(tempDir, "tsgonest-migrate-report.md");
    expect(existsSync(reportPath)).toBe(true);

    // Config file should be auto-generated
    const configPath = resolve(tempDir, "tsgonest.config.ts");
    expect(existsSync(configPath)).toBe(true);
    const configContent = readFileSync(configPath, "utf-8");
    expect(configContent).toContain("defineConfig");
    expect(configContent).toContain("controllers");
    expect(configContent).toContain("transforms");

    rmSync(tempDir, { recursive: true });
  });

  it("should auto-fix tsconfig.json (remove baseUrl and plugins)", () => {
    const tempDir = mkdtempSync(resolve(tmpdir(), "tsgonest-migrate-tsconfig-"));
    cpSync(resolve(FIXTURES_DIR, "migrate"), tempDir, { recursive: true });

    // Create a tsconfig.json with baseUrl and typia plugins
    writeFileSync(
      resolve(tempDir, "tsconfig.json"),
      JSON.stringify(
        {
          compilerOptions: {
            target: "ES2021",
            module: "commonjs",
            baseUrl: ".",
            plugins: [
              { transform: "typia/lib/transform" },
              { transform: "@nestia/core/lib/transform" },
            ],
          },
        },
        null,
        2,
      ),
    );

    const { exitCode } = runTsgonest([
      "migrate",
      "--apply",
      "--force",
      "--yes",
      "--cwd",
      tempDir,
      "--include",
      "src/**/*.ts",
    ]);
    expect(exitCode).toBe(0);

    const tsconfig = JSON.parse(
      readFileSync(resolve(tempDir, "tsconfig.json"), "utf-8"),
    );
    // baseUrl should be removed
    expect(tsconfig.compilerOptions.baseUrl).toBeUndefined();
    // plugins should be removed (array was only typia/nestia entries)
    expect(tsconfig.compilerOptions.plugins).toBeUndefined();
    // Other options should remain
    expect(tsconfig.compilerOptions.target).toBe("ES2021");

    rmSync(tempDir, { recursive: true });
  });

  it("should report no changes for a clean project", () => {
    const tempDir = mkdtempSync(resolve(tmpdir(), "tsgonest-migrate-clean-"));
    const srcDir = resolve(tempDir, "src");
    mkdirSync(srcDir, { recursive: true });
    writeFileSync(
      resolve(srcDir, "plain.dto.ts"),
      "export interface PlainDto { name: string; }\n"
    );

    const { stdout, exitCode } = runTsgonest([
      "migrate",
      "--force",
      "--cwd",
      tempDir,
      "--include",
      "src/**/*.ts",
    ]);
    expect(exitCode).toBe(0);
    expect(stdout).not.toContain("---");

    rmSync(tempDir, { recursive: true });
  });
});
