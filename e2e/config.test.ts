import { describe, it, expect, beforeAll } from "vitest";
import {
  existsSync,
  rmSync,
  writeFileSync,
  mkdtempSync,
  cpSync,
} from "fs";
import { tmpdir } from "os";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest TypeScript config file support", () => {
  let tempDir: string;

  beforeAll(() => {
    tempDir = mkdtempSync(resolve(tmpdir(), "tsgonest-ts-config-"));
    cpSync(resolve(FIXTURES_DIR, "simple"), tempDir, { recursive: true });

    const jsonConfig = resolve(tempDir, "tsgonest.config.json");
    if (existsSync(jsonConfig)) {
      rmSync(jsonConfig);
    }

    const distDir = resolve(tempDir, "dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
  });

  it("should load TypeScript config with --config flag", () => {
    const tsConfigPath = resolve(tempDir, "tsgonest.config.ts");
    writeFileSync(
      tsConfigPath,
      `export default {
  controllers: {
    include: ["src/**/*.controller.ts"],
  },
  transforms: {
    validation: true,
    serialization: true,
  },
  openapi: {
    output: "dist/openapi.json",
  },
};
`
    );

    const { exitCode, stderr } = runTsgonest(
      ["--project", "tsconfig.json", "--config", "tsgonest.config.ts"],
      { cwd: tempDir }
    );
    expect(exitCode).toBe(0);
    expect(stderr).toContain("loaded config from tsgonest.config.ts");
  });

  it("should auto-discover .ts config over .json", () => {
    writeFileSync(
      resolve(tempDir, "tsgonest.config.ts"),
      `export default {
  controllers: {
    include: ["src/**/*.controller.ts"],
  },
  transforms: {
    validation: true,
    serialization: true,
  },
  openapi: {
    output: "dist/openapi.json",
  },
};
`
    );
    writeFileSync(
      resolve(tempDir, "tsgonest.config.json"),
      JSON.stringify({
        controllers: { include: ["src/**/*.controller.ts"] },
        transforms: { validation: true, serialization: true },
        openapi: { output: "dist/openapi.json" },
      })
    );

    const distDir = resolve(tempDir, "dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest(
      ["--project", "tsconfig.json"],
      { cwd: tempDir }
    );
    expect(exitCode).toBe(0);
    expect(stderr).toContain("loaded config from tsgonest.config.ts");
  });

  it("should fall back to .json when no .ts config exists", () => {
    const tsConfig = resolve(tempDir, "tsgonest.config.ts");
    if (existsSync(tsConfig)) {
      rmSync(tsConfig);
    }

    writeFileSync(
      resolve(tempDir, "tsgonest.config.json"),
      JSON.stringify({
        controllers: { include: ["src/**/*.controller.ts"] },
        transforms: { validation: true, serialization: true },
        openapi: { output: "dist/openapi.json" },
      })
    );

    const distDir = resolve(tempDir, "dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }

    const { exitCode, stderr } = runTsgonest(
      ["--project", "tsconfig.json"],
      { cwd: tempDir }
    );
    expect(exitCode).toBe(0);
    expect(stderr).toContain("loaded config from tsgonest.config.json");
  });

  it("should error on .ts config with no default export", () => {
    const tsConfigPath = resolve(tempDir, "tsgonest.config.ts");
    writeFileSync(
      tsConfigPath,
      `const config = {
  controllers: { include: ["src/**/*.controller.ts"] },
};
`
    );

    const { exitCode, stderr } = runTsgonest(
      ["--project", "tsconfig.json", "--config", "tsgonest.config.ts"],
      { cwd: tempDir }
    );
    expect(exitCode).toBe(1);
    expect(stderr).toContain("error");
  });
});
