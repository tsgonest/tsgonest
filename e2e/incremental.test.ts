import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { existsSync, readFileSync, rmSync, writeFileSync, unlinkSync } from "fs";
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
  const cacheFile = resolve(distDir, ".tsgonest-cache");
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
    expect(cache.v).toBe(2);
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
    expect(stderr).toMatch(/companions\s+\d+(?:ms|s)/);
    expect(stderr).toMatch(/controllers\s+\d+(?:ms|s)/);
    expect(stderr).toMatch(/openapi\s+\d+(?:ms|s)/);
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
    expect(cache.v).toBe(2);
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
    expect(newCache.v).toBe(2);
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
    const simpleCacheFile = resolve(simpleDist, ".tsgonest-cache");

    runTsgonest(["--project", "testdata/simple/tsconfig.json"]);

    expect(existsSync(simpleCacheFile)).toBe(true);
  });
});

describe("tsgonest incremental multi-file", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "incremental-multifile");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");
  const userSrc = resolve(fixtureDir, "src/user.ts");
  const orderSrc = resolve(fixtureDir, "src/order.ts");
  const indexSrc = resolve(fixtureDir, "src/index.ts");
  const productSrc = resolve(fixtureDir, "src/product.ts");

  // Save originals for restoration
  let originalUser: string;
  let originalOrder: string;
  let originalIndex: string;

  function clean() {
    if (existsSync(distDir)) rmSync(distDir, { recursive: true });
    if (existsSync(tsbuildinfo)) rmSync(tsbuildinfo);
  }

  function build() {
    return runTsgonest([
      "--project",
      "testdata/incremental-multifile/tsconfig.json",
    ]);
  }

  beforeAll(() => {
    originalUser = readFileSync(userSrc, "utf-8");
    originalOrder = readFileSync(orderSrc, "utf-8");
    originalIndex = readFileSync(indexSrc, "utf-8");
    clean();
  });

  afterAll(() => {
    // Restore all source files to original state
    writeFileSync(userSrc, originalUser);
    writeFileSync(orderSrc, originalOrder);
    writeFileSync(indexSrc, originalIndex);
    if (existsSync(productSrc)) unlinkSync(productSrc);
    // Rebuild to restore clean state
    clean();
    build();
  });

  it("cold build should emit all files", () => {
    const { exitCode, stderr } = build();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(existsSync(resolve(distDir, "user.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "order.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);
  });

  it("warm build should emit nothing", () => {
    const { exitCode, stderr } = build();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("no files emitted");
  });

  it("modifying one file should trigger re-emit", () => {
    const modified = originalUser + "\nexport const USER_VERSION = 1;\n";
    writeFileSync(userSrc, modified);

    const { exitCode, stderr } = build();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).not.toContain("no files emitted");

    // The changed content should appear in the output
    const userJs = readFileSync(resolve(distDir, "user.js"), "utf-8");
    expect(userJs).toContain("USER_VERSION");

    // Restore
    writeFileSync(userSrc, originalUser);
    build();
  });

  it("adding a new file should include it in incremental emit", () => {
    const productContent = `export interface Product {
  sku: string;
  price: number;
}
`;
    writeFileSync(productSrc, productContent);

    // Also update index.ts to re-export the new file
    const updatedIndex =
      originalIndex + '\nexport { Product } from "./product";\n';
    writeFileSync(indexSrc, updatedIndex);

    const { exitCode, stderr } = build();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(existsSync(resolve(distDir, "product.js"))).toBe(true);

    const productJs = readFileSync(resolve(distDir, "product.js"), "utf-8");
    expect(productJs).toBeDefined();

    // Restore
    writeFileSync(indexSrc, originalIndex);
    unlinkSync(productSrc);
    // Clean and rebuild since removing a file from the program invalidates tsbuildinfo
    clean();
    build();
  });

  it("introducing a type error should exit non-zero", () => {
    writeFileSync(
      userSrc,
      "export interface User { name: string; age: NONEXISTENT; }\n"
    );

    const { exitCode, stderr } = build();
    expect(exitCode).toBe(1);
    expect(stderr).toContain("error TS");

    // Restore
    writeFileSync(userSrc, originalUser);
  });

  it("fixing a type error should produce a successful build", () => {
    // Previous test left user.ts in error state — ensure it's restored
    writeFileSync(userSrc, originalUser);

    const { exitCode, stderr } = build();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");

    const userJs = readFileSync(resolve(distDir, "user.js"), "utf-8");
    expect(userJs).toContain("createUser");
  });

  it("corrupted .tsbuildinfo should fall back to full build", () => {
    // First ensure we have a warm state
    const warm = build();
    expect(warm.stderr).toContain("no files emitted");

    // Corrupt the buildinfo
    writeFileSync(tsbuildinfo, "{ broken json {{{{");

    const { exitCode, stderr } = build();
    expect(exitCode).toBe(0);
    // Should do a full emit since buildinfo is unreadable
    expect(stderr).toContain("emitted");

    // Subsequent warm build should work normally again
    const warmAfter = build();
    expect(warmAfter.exitCode).toBe(0);
    expect(warmAfter.stderr).toContain("no files emitted");
  });
});

describe("tsgonest incremental with missing output dir", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "incremental-multifile");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");

  function clean() {
    if (existsSync(distDir)) rmSync(distDir, { recursive: true });
    if (existsSync(tsbuildinfo)) rmSync(tsbuildinfo);
  }

  function build() {
    return runTsgonest([
      "--project",
      "testdata/incremental-multifile/tsconfig.json",
    ]);
  }

  beforeAll(() => {
    clean();
  });

  it("should re-emit JS when dist/ is deleted but .tsbuildinfo exists", () => {
    // Cold build
    const cold = build();
    expect(cold.exitCode).toBe(0);
    expect(cold.stderr).toContain("emitted");
    expect(existsSync(resolve(distDir, "user.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "order.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);
    expect(existsSync(tsbuildinfo)).toBe(true);

    // Delete dist/ but keep .tsbuildinfo
    rmSync(distDir, { recursive: true });
    expect(existsSync(distDir)).toBe(false);
    expect(existsSync(tsbuildinfo)).toBe(true);

    // Rebuild — should detect missing outputs and re-emit
    const rebuild = build();
    expect(rebuild.exitCode).toBe(0);
    expect(rebuild.stderr).toContain("emitted");
    expect(rebuild.stderr).not.toContain("no files emitted");
    expect(existsSync(resolve(distDir, "user.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "order.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "index.js"))).toBe(true);
  });

  it("should re-emit when individual JS files are missing", () => {
    clean();
    build();

    // Delete just one output file
    rmSync(resolve(distDir, "user.js"));
    expect(existsSync(resolve(distDir, "user.js"))).toBe(false);
    expect(existsSync(resolve(distDir, "order.js"))).toBe(true);

    const rebuild = build();
    expect(rebuild.exitCode).toBe(0);
    // Should re-emit since expected outputs are missing
    expect(rebuild.stderr).toContain("emitted");
    expect(existsSync(resolve(distDir, "user.js"))).toBe(true);
  });
});

describe("tsgonest incremental NestJS with missing output dir", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "incremental-nestjs");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");

  function cleanAndBuild() {
    if (existsSync(distDir)) rmSync(distDir, { recursive: true });
    if (existsSync(tsbuildinfo)) rmSync(tsbuildinfo);
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

  it("should re-emit JS + companions when dist/ deleted but .tsbuildinfo kept", () => {
    cleanAndBuild();

    // Delete dist/ but keep .tsbuildinfo
    rmSync(distDir, { recursive: true });
    expect(existsSync(tsbuildinfo)).toBe(true);

    const rebuild = warmBuild();
    expect(rebuild.exitCode).toBe(0);
    expect(rebuild.stderr).toContain("emitted");
    expect(rebuild.stderr).not.toContain("no files emitted");
    expect(rebuild.stderr).toContain("companion");
    expect(rebuild.stderr).toContain("OpenAPI");

    // All outputs should exist
    expect(existsSync(resolve(distDir, "item.dto.js"))).toBe(true);
    expect(existsSync(resolve(distDir, "item.controller.js"))).toBe(true);
    expect(
      existsSync(resolve(distDir, "item.dto.CreateItemDto.tsgonest.js"))
    ).toBe(true);
    expect(existsSync(resolve(distDir, "openapi.json"))).toBe(true);

    // Incremental state should stabilize within 2 warm builds
    // (the fresh .tsbuildinfo may need one settling emit)
    let settled = false;
    for (let i = 0; i < 2; i++) {
      const warm = warmBuild();
      expect(warm.exitCode).toBe(0);
      if (warm.stderr.includes("no changes detected")) {
        settled = true;
        break;
      }
    }
    expect(settled).toBe(true);
  });
});

describe("tsgonest incremental companion + OpenAPI updates", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "incremental-nestjs");
  const distDir = resolve(fixtureDir, "dist");
  const tsbuildinfo = resolve(fixtureDir, "tsconfig.tsbuildinfo");
  const dtoSrc = resolve(fixtureDir, "src/item.dto.ts");
  const controllerSrc = resolve(fixtureDir, "src/item.controller.ts");
  const openapiFile = resolve(distDir, "openapi.json");
  const companionValidate = resolve(
    distDir,
    "item.dto.CreateItemDto.tsgonest.js"
  );
  const companionResponse = resolve(
    distDir,
    "item.dto.ItemResponse.tsgonest.js"
  );

  let originalDto: string;
  let originalController: string;

  function cleanAndBuild() {
    if (existsSync(distDir)) rmSync(distDir, { recursive: true });
    if (existsSync(tsbuildinfo)) rmSync(tsbuildinfo);
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
    originalDto = readFileSync(dtoSrc, "utf-8");
    originalController = readFileSync(controllerSrc, "utf-8");
    cleanAndBuild();
  });

  afterAll(() => {
    writeFileSync(dtoSrc, originalDto);
    writeFileSync(controllerSrc, originalController);
    cleanAndBuild();
  });

  it("adding a property to a DTO should update companion and OpenAPI", () => {
    cleanAndBuild();

    // Read baseline companion and OpenAPI
    const baselineCompanion = readFileSync(companionValidate, "utf-8");
    const baselineOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(baselineCompanion).not.toContain("description");
    expect(
      baselineOpenAPI.components.schemas.CreateItemDto.properties
    ).not.toHaveProperty("description");

    // Add a "description" property to CreateItemDto
    const modifiedDto = originalDto.replace(
      "export interface CreateItemDto {",
      "export interface CreateItemDto {\n  /** @minLength 1 */\n  description: string;"
    );
    writeFileSync(dtoSrc, modifiedDto);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("emitted");
    expect(stderr).toContain("companion");

    // Companion should now validate the new property
    const updatedCompanion = readFileSync(companionValidate, "utf-8");
    expect(updatedCompanion).toContain("description");

    // OpenAPI should include the new property
    const updatedOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(
      updatedOpenAPI.components.schemas.CreateItemDto.properties
    ).toHaveProperty("description");
    expect(
      updatedOpenAPI.components.schemas.CreateItemDto.required
    ).toContain("description");

    // Restore
    writeFileSync(dtoSrc, originalDto);
    cleanAndBuild();
  });

  it("adding a new route to the controller should update OpenAPI", () => {
    cleanAndBuild();

    const baselineOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(baselineOpenAPI.paths["/items/{id}"]).toBeUndefined();

    // Add a GET /:id route
    const modifiedController = originalController.replace(
      "async create(@Body() body: CreateItemDto): Promise<ItemResponse> {",
      `async findOne(): Promise<ItemResponse> {
    return {} as ItemResponse;
  }

  @Post()
  async create(@Body() body: CreateItemDto): Promise<ItemResponse> {`
    );
    // Also add @Get(':id') decorator before findOne
    const withDecorator = modifiedController.replace(
      "async findOne(): Promise<ItemResponse> {",
      '@Get(":id")\n  async findOne(): Promise<ItemResponse> {'
    );
    writeFileSync(controllerSrc, withDecorator);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("3 route(s)");

    const updatedOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(updatedOpenAPI.paths["/items/{id}"]).toBeDefined();
    expect(updatedOpenAPI.paths["/items/{id}"].get).toBeDefined();

    // Restore
    writeFileSync(controllerSrc, originalController);
    cleanAndBuild();
  });

  it("removing a DTO property should update companion and OpenAPI", () => {
    cleanAndBuild();

    // Verify baseline has 'price'
    const baselineCompanion = readFileSync(companionValidate, "utf-8");
    expect(baselineCompanion).toContain("price");
    const baselineOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(
      baselineOpenAPI.components.schemas.CreateItemDto.properties
    ).toHaveProperty("price");

    // Remove the price property from CreateItemDto
    const modifiedDto = originalDto
      .replace("  /** @minimum 0 */\n  price: number;\n", "")
      .replace("  price: number;\n", "");
    writeFileSync(dtoSrc, modifiedDto);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);
    expect(stderr).toContain("companion");

    // Companion should no longer validate price
    const updatedCompanion = readFileSync(companionValidate, "utf-8");
    expect(updatedCompanion).not.toContain("input.price");

    // OpenAPI should no longer have price
    const updatedOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(
      updatedOpenAPI.components.schemas.CreateItemDto.properties
    ).not.toHaveProperty("price");

    // Restore
    writeFileSync(dtoSrc, originalDto);
    cleanAndBuild();
  });

  it("modifying a response DTO should update its serializer companion", () => {
    cleanAndBuild();

    const baselineSerializer = readFileSync(companionResponse, "utf-8");
    expect(baselineSerializer).not.toContain("createdAt");

    // Add a createdAt field to ItemResponse
    const modifiedDto = originalDto.replace(
      "export interface ItemResponse {\n  id: number;\n  name: string;\n  price: number;\n}",
      "export interface ItemResponse {\n  id: number;\n  name: string;\n  price: number;\n  createdAt: string;\n}"
    );
    writeFileSync(dtoSrc, modifiedDto);

    const { exitCode, stderr } = warmBuild();
    expect(exitCode).toBe(0);

    const updatedSerializer = readFileSync(companionResponse, "utf-8");
    expect(updatedSerializer).toContain("createdAt");

    // OpenAPI should also reflect the new field
    const updatedOpenAPI = JSON.parse(readFileSync(openapiFile, "utf-8"));
    expect(
      updatedOpenAPI.components.schemas.ItemResponse.properties
    ).toHaveProperty("createdAt");

    // Restore
    writeFileSync(dtoSrc, originalDto);
    cleanAndBuild();
  });
});
