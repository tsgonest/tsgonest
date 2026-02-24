/**
 * Package.json manipulation for the migrate command.
 *
 * Handles:
 *   - Detecting old dependencies (nestia, class-validator, etc.)
 *   - Removing old dependencies
 *   - Adding tsgonest dependencies
 *   - Updating build/dev scripts
 *   - Detecting and removing old config files
 */

import { readFileSync, writeFileSync, existsSync, unlinkSync } from "fs";
import { join, relative } from "path";

// ── Dependency groups ────────────────────────────────────────────────────────

/** Packages to remove when migrating from Nestia/Typia. */
const NESTIA_DEPS = [
  "@nestia/core",
  "@nestia/sdk",
  "@nestia/e2e",
  "@nestia/fetcher",
  "nestia",
  "typia",
];

/** Packages to remove when migrating from class-validator/class-transformer. */
const CLASS_DEPS = [
  "class-validator",
  "class-transformer",
];

/** Packages to remove when migrating from @nestjs/swagger. */
const SWAGGER_DEPS = [
  "@nestjs/swagger",
  "swagger-ui-express",
  "@nestjs/swagger-cli",
];

/** Packages tsgonest adds. */
const TSGONEST_DEPS = {
  devDependencies: {
    tsgonest: "latest",
  },
  dependencies: {
    "@tsgonest/runtime": "latest",
    "@tsgonest/types": "latest",
  },
};

// ── Config files ─────────────────────────────────────────────────────────────

/** Nestia-related config files to detect and remove. */
const NESTIA_CONFIG_FILES = [
  "nestia.config.ts",
  "nestia.config.js",
  "nestia.config.json",
  "nestia.config.mjs",
  "nestia.config.cjs",
];

// ── Types ────────────────────────────────────────────────────────────────────

interface PackageJson {
  name?: string;
  scripts?: Record<string, string>;
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
  peerDependencies?: Record<string, string>;
  [key: string]: unknown;
}

export interface DetectedPackages {
  hasNestiaDeps: boolean;
  nestiaDeps: string[];
  hasClassDeps: boolean;
  classDeps: string[];
  hasSwaggerDeps: boolean;
  swaggerDeps: string[];
  hasTsgonest: boolean;
  configFiles: string[];
  currentScripts: Record<string, string>;
}

// ── Detection ────────────────────────────────────────────────────────────────

/**
 * Read and parse package.json from the given directory.
 * Returns null if not found.
 */
export function readPackageJson(cwd: string): PackageJson | null {
  const pkgPath = join(cwd, "package.json");
  if (!existsSync(pkgPath)) return null;
  try {
    return JSON.parse(readFileSync(pkgPath, "utf-8"));
  } catch {
    return null;
  }
}

/**
 * Scan package.json for old dependencies, existing tsgonest deps,
 * config files, and current scripts.
 */
export function detectPackages(cwd: string): DetectedPackages {
  const pkg = readPackageJson(cwd);
  const result: DetectedPackages = {
    hasNestiaDeps: false,
    nestiaDeps: [],
    hasClassDeps: false,
    classDeps: [],
    hasSwaggerDeps: false,
    swaggerDeps: [],
    hasTsgonest: false,
    configFiles: [],
    currentScripts: {},
  };

  if (!pkg) return result;

  const allDeps = {
    ...pkg.dependencies,
    ...pkg.devDependencies,
    ...pkg.peerDependencies,
  };

  // Check for nestia/typia
  for (const dep of NESTIA_DEPS) {
    if (dep in allDeps) {
      result.nestiaDeps.push(dep);
    }
  }
  result.hasNestiaDeps = result.nestiaDeps.length > 0;

  // Check for class-validator/class-transformer
  for (const dep of CLASS_DEPS) {
    if (dep in allDeps) {
      result.classDeps.push(dep);
    }
  }
  result.hasClassDeps = result.classDeps.length > 0;

  // Check for swagger
  for (const dep of SWAGGER_DEPS) {
    if (dep in allDeps) {
      result.swaggerDeps.push(dep);
    }
  }
  result.hasSwaggerDeps = result.swaggerDeps.length > 0;

  // Check for tsgonest
  result.hasTsgonest =
    "tsgonest" in allDeps ||
    "@tsgonest/runtime" in allDeps ||
    "@tsgonest/types" in allDeps;

  // Check for nestia config files
  for (const file of NESTIA_CONFIG_FILES) {
    if (existsSync(join(cwd, file))) {
      result.configFiles.push(file);
    }
  }

  // Current scripts
  result.currentScripts = pkg.scripts ?? {};

  return result;
}

// ── Mutations ────────────────────────────────────────────────────────────────

/**
 * Remove the given package names from all dependency sections.
 * Returns the list of actually removed packages.
 */
export function removeDependencies(cwd: string, packages: string[]): string[] {
  const pkgPath = join(cwd, "package.json");
  const pkg = readPackageJson(cwd);
  if (!pkg) return [];

  const removed: string[] = [];
  const sections: (keyof PackageJson)[] = ["dependencies", "devDependencies", "peerDependencies"];

  for (const section of sections) {
    const deps = pkg[section] as Record<string, string> | undefined;
    if (!deps) continue;
    for (const name of packages) {
      if (name in deps) {
        delete deps[name];
        if (!removed.includes(name)) removed.push(name);
      }
    }
  }

  if (removed.length > 0) {
    writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  }

  return removed;
}

/**
 * Add tsgonest packages to the appropriate dependency sections.
 * Returns list of added package names.
 */
export function addTsgonestDependencies(cwd: string): string[] {
  const pkgPath = join(cwd, "package.json");
  const pkg = readPackageJson(cwd);
  if (!pkg) return [];

  const added: string[] = [];

  // Add devDependencies
  if (!pkg.devDependencies) pkg.devDependencies = {};
  for (const [name, version] of Object.entries(TSGONEST_DEPS.devDependencies)) {
    if (!(name in (pkg.dependencies ?? {})) && !(name in pkg.devDependencies)) {
      pkg.devDependencies[name] = version;
      added.push(name);
    }
  }

  // Add dependencies
  if (!pkg.dependencies) pkg.dependencies = {};
  for (const [name, version] of Object.entries(TSGONEST_DEPS.dependencies)) {
    if (!(name in pkg.dependencies) && !(name in (pkg.devDependencies ?? {}))) {
      pkg.dependencies[name] = version;
      added.push(name);
    }
  }

  if (added.length > 0) {
    writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  }

  return added;
}

/**
 * Update build and dev scripts in package.json.
 * Returns list of updated script names.
 */
export function updateScripts(cwd: string): string[] {
  const pkgPath = join(cwd, "package.json");
  const pkg = readPackageJson(cwd);
  if (!pkg) return [];

  if (!pkg.scripts) pkg.scripts = {};
  const updated: string[] = [];

  // Build script: replace nest build / tsc / npx nestia with tsgonest build
  const buildScript = pkg.scripts.build ?? "";
  if (
    buildScript.includes("nest build") ||
    buildScript.includes("nestia") ||
    buildScript === "tsc" ||
    buildScript.includes("tsc ")
  ) {
    pkg.scripts.build = "tsgonest build";
    updated.push("build");
  } else if (!pkg.scripts.build) {
    pkg.scripts.build = "tsgonest build";
    updated.push("build");
  }

  // Dev script: replace nest start --watch / ts-node-dev / nodemon with tsgonest dev
  const devScript = pkg.scripts["start:dev"] ?? pkg.scripts.dev ?? "";
  if (
    devScript.includes("nest start") ||
    devScript.includes("ts-node-dev") ||
    devScript.includes("nodemon") ||
    devScript.includes("nestia")
  ) {
    // Use the same key that existed (start:dev or dev)
    const key = pkg.scripts["start:dev"] !== undefined ? "start:dev" : "dev";
    pkg.scripts[key] = "tsgonest dev";
    updated.push(key);
  } else if (!pkg.scripts["start:dev"] && !pkg.scripts.dev) {
    pkg.scripts.dev = "tsgonest dev";
    updated.push("dev");
  }

  if (updated.length > 0) {
    writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  }

  return updated;
}

/**
 * Remove nestia config files from the project.
 * Returns list of actually removed file names.
 */
export function removeConfigFiles(cwd: string, files: string[]): string[] {
  const removed: string[] = [];
  for (const file of files) {
    const fullPath = join(cwd, file);
    if (existsSync(fullPath)) {
      unlinkSync(fullPath);
      removed.push(file);
    }
  }
  return removed;
}
