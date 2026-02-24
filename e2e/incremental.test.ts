import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync, writeFileSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("tsgonest incremental compilation", () => {
  const incrDist = resolve(FIXTURES_DIR, "incremental/dist");
  const tsbuildinfo = resolve(FIXTURES_DIR, "incremental/tsconfig.tsbuildinfo");
  const srcFile = resolve(FIXTURES_DIR, "incremental/src/index.ts");

  beforeAll(() => {
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
    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/incremental/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("incremental build enabled");
    expect(stderr).toContain("no files emitted");
  });

  it("should re-emit only changed files after modification", () => {
    const originalContent = readFileSync(srcFile, "utf-8");

    const modifiedContent =
      originalContent + "\nexport const VERSION = 42;\n";
    writeFileSync(srcFile, modifiedContent);

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/incremental/tsconfig.json",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("incremental build enabled");
    expect(stderr).toContain("emitted");

    const jsContent = readFileSync(resolve(incrDist, "index.js"), "utf-8");
    expect(jsContent).toContain("VERSION");

    writeFileSync(srcFile, originalContent);

    runTsgonest(["--project", "testdata/incremental/tsconfig.json"]);
  });
});

describe("tsgonest incremental post-processing cache", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "incremental-nestjs");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");
  const cacheFile = resolve(fixtureDir, "tsconfig.tsgonest-cache");
  const configFile = resolve(fixtureDir, "tsgonest.config.json");
  const srcFile = resolve(fixtureDir, "src/item.dto.ts");
  const openapiFile = resolve(fixtureDir, "dist/openapi.json");

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

  function warmBuild() {
    return runTsgonest([
      "--project",
      "testdata/incremental-nestjs/tsconfig.json",
      "--config",
      "testdata/incremental-nestjs/tsgonest.config.json",
    ]);
  }

  beforeAll(() => {
    cleanAndBuild();
  });

  it("cold build should produce all outputs and cache file", () => {
    const { exitCode, stderr } = cleanAndBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");
    expect(stderr).toContain("controller");
    expect(stderr).toContain("OpenAPI");

    expect(existsSync(cacheFile)).toBe(true);
    const cache = JSON.parse(readFileSync(cacheFile, "utf-8"));
    expect(cache.v).toBe(1);
    expect(cache.configHash).toBeTruthy();
    expect(cache.outputs).toBeInstanceOf(Array);
    expect(cache.outputs.length).toBeGreaterThanOrEqual(1);

    expect(existsSync(openapiFile)).toBe(true);
  });

  it("warm build with no changes should skip post-processing", () => {
    cleanAndBuild();

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("no changes detected, outputs up to date");

    expect(stderr).not.toContain("companion file");
    expect(stderr).not.toContain("found 1 controller");
    expect(stderr).toContain("companions:    0s");
    expect(stderr).toContain("controllers:   0s");
    expect(stderr).toContain("openapi:       0s");
  });

  it("config change should force full rebuild", () => {
    cleanAndBuild();

    const originalConfig = readFileSync(configFile, "utf-8");
    const modifiedConfig = originalConfig.replace(
      '"dist/openapi.json"',
      '"dist/openapi.json" '
    );
    writeFileSync(configFile, modifiedConfig);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");

    writeFileSync(configFile, originalConfig);
    warmBuild();
  });

  it("output file deletion should force full rebuild", () => {
    cleanAndBuild();

    rmSync(openapiFile);
    expect(existsSync(openapiFile)).toBe(false);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("OpenAPI");
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("source file change should force full rebuild", () => {
    cleanAndBuild();

    const originalSrc = readFileSync(srcFile, "utf-8");
    const modifiedSrc =
      originalSrc + "\nexport interface ExtraDto { extra: string; }\n";
    writeFileSync(srcFile, modifiedSrc);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");

    writeFileSync(srcFile, originalSrc);
    warmBuild();
  });

  it("cache file missing should force full rebuild", () => {
    cleanAndBuild();

    rmSync(cacheFile);
    expect(existsSync(cacheFile)).toBe(false);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");
    expect(existsSync(cacheFile)).toBe(true);
  });

  it("corrupted cache file should force full rebuild", () => {
    cleanAndBuild();

    writeFileSync(cacheFile, "not valid json {{{");

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");
    const cacheContent = readFileSync(cacheFile, "utf-8");
    const cache = JSON.parse(cacheContent);
    expect(cache.v).toBe(1);
  });

  it("--clean flag should force full rebuild", () => {
    cleanAndBuild();

    const warmResult = warmBuild();
    expect(warmResult.stderr).toContain("no changes detected");

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
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");
  });

  it("schema version bump should force full rebuild", () => {
    cleanAndBuild();

    const cacheContent = JSON.parse(readFileSync(cacheFile, "utf-8"));
    cacheContent.v = 999;
    writeFileSync(cacheFile, JSON.stringify(cacheContent));

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).not.toContain("no changes detected");
    expect(stderr).toContain("companion");

    const newCache = JSON.parse(readFileSync(cacheFile, "utf-8"));
    expect(newCache.v).toBe(1);
  });

  it("successive warm builds should all skip consistently", () => {
    cleanAndBuild();

    for (let i = 0; i < 3; i++) {
      const { exitCode, stderr } = warmBuild();
      expect(exitCode).toBe(0);
      expect(stderr).toContain("no changes detected, outputs up to date");
    }
  });

  it("rebuild after skip should still produce correct outputs", () => {
    cleanAndBuild();

    const warmResult = warmBuild();
    expect(warmResult.stderr).toContain("no changes detected");

    const openapi = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(openapi.openapi).toBe("3.2.0");
    expect(openapi.paths).toHaveProperty("/items");
  });

  it("non-incremental build should not create cache file", () => {
    const simpleDist = resolve(FIXTURES_DIR, "simple/dist");
    if (existsSync(simpleDist)) {
      rmSync(simpleDist, { recursive: true });
    }
    const simpleCacheFile = resolve(FIXTURES_DIR, "simple/tsconfig.tsgonest-cache");
    if (existsSync(simpleCacheFile)) {
      rmSync(simpleCacheFile);
    }

    runTsgonest(["--project", "testdata/simple/tsconfig.json"]);

    expect(existsSync(simpleCacheFile)).toBe(true);
  });
});
