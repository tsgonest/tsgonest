/**
 * tsgonest migrate — AST-based codemod for migrating to tsgonest.
 *
 * Supports:
 *   - Nestia/Typia → tsgonest (controller decorators + typia tags)
 *   - class-validator → tsgonest (class DTOs → interfaces with branded types)
 *   - class-transformer → tsgonest (transforms, coercion)
 *   - @nestjs/swagger cleanup (remove redundant decorators)
 *   - package.json dependency management
 *   - Config file cleanup (nestia.config.ts, etc.)
 *   - Build/dev script updates
 *
 * Dry-run by default. Pass --apply to write changes.
 *
 * Invoked by the Go binary: `tsgonest migrate [flags]`
 * The Go binary resolves migrate.cjs relative to the tsgonest npm package
 * and runs: node <path>/migrate.cjs [flags]
 */

import { Project, SourceFile } from "ts-morph";
import { resolve, relative, join } from "path";
import { readFileSync, writeFileSync, existsSync } from "fs";
import { parse as parseJsonc, modify, applyEdits, type ModificationOptions } from "jsonc-parser";
import { transformNestia } from "./transforms/nestia.js";
import { transformTypiaTags } from "./transforms/typia-tags.js";
import { transformClassValidator } from "./transforms/class-validator.js";
import { transformClassTransformer } from "./transforms/class-transformer.js";
import { transformSwagger } from "./transforms/swagger.js";
import { cleanupImports } from "./transforms/imports.js";
import { BOLD, RED, GREEN, YELLOW, CYAN, RESET } from "./colors.js";
import { MigrateReport } from "./report.js";
import { isGitRepo, isGitClean, getGitStatus } from "./git.js";
import { confirm, closePrompts } from "./prompts.js";
import {
  detectPackages,
  removeDependencies,
  addTsgonestDependencies,
  updateScripts,
  removeConfigFiles,
  type DetectedPackages,
} from "./packagejson.js";

// ── CLI arg parsing ──────────────────────────────────────────────────────────

interface MigrateOptions {
  apply: boolean;
  include: string[];
  tsconfig: string;
  cwd: string;
  force: boolean;
  yes: boolean; // skip interactive prompts (accept all defaults)
}

function parseArgs(argv: string[]): MigrateOptions {
  const opts: MigrateOptions = {
    apply: false,
    include: [],
    tsconfig: "tsconfig.json",
    cwd: process.cwd(),
    force: false,
    yes: false,
  };

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--apply") {
      opts.apply = true;
    } else if (arg === "--include" && argv[i + 1]) {
      opts.include.push(argv[++i]);
    } else if (arg === "--tsconfig" && argv[i + 1]) {
      opts.tsconfig = argv[++i];
    } else if (arg === "--cwd" && argv[i + 1]) {
      opts.cwd = argv[++i];
    } else if (arg === "--force") {
      opts.force = true;
    } else if (arg === "--yes" || arg === "-y") {
      opts.yes = true;
    } else if (arg === "--help" || arg === "-h") {
      printHelp();
      process.exit(0);
    }
  }

  if (opts.include.length === 0) {
    opts.include = ["src/**/*.controller.ts", "src/**/*.dto.ts", "src/**/main.ts"];
  }

  return opts;
}

function printHelp(): void {
  console.log(`tsgonest migrate — Migrate from Nestia/Typia/class-validator to tsgonest

Usage:
  tsgonest migrate [flags]

Flags:
  --apply              Write changes to disk (default: dry-run preview)
  --include <glob>     Glob pattern for files to migrate (repeatable)
                       Default: src/**/*.controller.ts, src/**/*.dto.ts, src/**/main.ts
  --tsconfig <path>    Path to tsconfig.json (default: tsconfig.json)
  --cwd <path>         Working directory (default: current directory)
  --force              Run even if git working directory is dirty
  --yes, -y            Accept all prompts (non-interactive)
  --help, -h           Print this help message

Examples:
  tsgonest migrate                          # Preview changes (dry-run)
  tsgonest migrate --apply                  # Apply changes (interactive)
  tsgonest migrate --apply --yes            # Apply all changes (non-interactive)
  tsgonest migrate --include 'src/**/*.ts'  # Scan all TS files
`);
}

// ── Detection ────────────────────────────────────────────────────────────────

interface DetectionResult {
  hasNestia: boolean;
  hasTypia: boolean;
  hasClassValidator: boolean;
  hasClassTransformer: boolean;
  hasSwagger: boolean;
}

function detectSources(files: SourceFile[]): DetectionResult {
  const result: DetectionResult = {
    hasNestia: false,
    hasTypia: false,
    hasClassValidator: false,
    hasClassTransformer: false,
    hasSwagger: false,
  };

  for (const file of files) {
    for (const imp of file.getImportDeclarations()) {
      const mod = imp.getModuleSpecifierValue();
      if (mod === "@nestia/core") result.hasNestia = true;
      if (mod === "typia") result.hasTypia = true;
      if (mod === "class-validator") result.hasClassValidator = true;
      if (mod === "class-transformer") result.hasClassTransformer = true;
      if (mod === "@nestjs/swagger") result.hasSwagger = true;
    }
  }

  return result;
}

// ── Diff display ─────────────────────────────────────────────────────────────

function showDiff(filePath: string, original: string, modified: string, cwd: string): boolean {
  if (original === modified) return false;

  const rel = relative(cwd, filePath);
  const origLines = original.split("\n");
  const modLines = modified.split("\n");

  console.log(`\n${BOLD}--- ${rel}${RESET}`);

  const maxLen = Math.max(origLines.length, modLines.length);
  let changes = 0;

  for (let i = 0; i < maxLen; i++) {
    const orig = origLines[i];
    const mod = modLines[i];
    if (orig === mod) continue;

    if (orig !== undefined && mod !== undefined && orig !== mod) {
      console.log(`${RED}- ${orig}${RESET}`);
      console.log(`${GREEN}+ ${mod}${RESET}`);
      changes++;
    } else if (orig !== undefined && mod === undefined) {
      console.log(`${RED}- ${orig}${RESET}`);
      changes++;
    } else if (orig === undefined && mod !== undefined) {
      console.log(`${GREEN}+ ${mod}${RESET}`);
      changes++;
    }
  }

  return changes > 0;
}

// ── Interactive migration plan ───────────────────────────────────────────────

interface MigrationPlan {
  transformSources: boolean;
  removeNestiaDeps: boolean;
  removeClassDeps: boolean;
  removeSwaggerDeps: boolean;
  addTsgonestDeps: boolean;
  updateBuildScripts: boolean;
  removeNestiaConfigs: boolean;
}

async function buildMigrationPlan(
  detected: DetectionResult,
  pkgInfo: DetectedPackages,
  opts: MigrateOptions,
): Promise<MigrationPlan> {
  const plan: MigrationPlan = {
    transformSources: true,
    removeNestiaDeps: false,
    removeClassDeps: false,
    removeSwaggerDeps: false,
    addTsgonestDeps: false,
    updateBuildScripts: false,
    removeNestiaConfigs: false,
  };

  // Non-interactive mode — accept all defaults
  if (opts.yes || !opts.apply) {
    plan.removeNestiaDeps = pkgInfo.hasNestiaDeps;
    plan.removeClassDeps = pkgInfo.hasClassDeps;
    plan.removeSwaggerDeps = pkgInfo.hasSwaggerDeps;
    plan.addTsgonestDeps = !pkgInfo.hasTsgonest;
    plan.updateBuildScripts = true;
    plan.removeNestiaConfigs = pkgInfo.configFiles.length > 0;
    return plan;
  }

  // Interactive mode — ask for each step
  console.log(`\n${BOLD}  Migration plan:${RESET}\n`);

  // Source transforms (always yes — that's the whole point)
  console.log(`  ${CYAN}1.${RESET} Transform source files (AST codemods)`);
  console.log("     Converts decorators and validators to tsgonest equivalents.\n");

  // Remove old dependencies
  if (pkgInfo.hasNestiaDeps) {
    const depList = pkgInfo.nestiaDeps.join(", ");
    plan.removeNestiaDeps = await confirm(
      `${CYAN}2.${RESET} Remove Nestia/Typia packages from package.json? (${depList})`,
    );
  }

  if (pkgInfo.hasClassDeps) {
    const depList = pkgInfo.classDeps.join(", ");
    plan.removeClassDeps = await confirm(
      `${CYAN}3.${RESET} Remove class-validator/class-transformer from package.json? (${depList})`,
    );
  }

  if (pkgInfo.hasSwaggerDeps) {
    const depList = pkgInfo.swaggerDeps.join(", ");
    plan.removeSwaggerDeps = await confirm(
      `${CYAN}4.${RESET} Remove @nestjs/swagger packages from package.json? (${depList})`,
    );
  }

  // Add tsgonest deps
  if (!pkgInfo.hasTsgonest) {
    plan.addTsgonestDeps = await confirm(
      `${CYAN}5.${RESET} Add tsgonest, @tsgonest/runtime, @tsgonest/types to package.json?`,
    );
  }

  // Update scripts
  plan.updateBuildScripts = await confirm(
    `${CYAN}6.${RESET} Update build/dev scripts to use tsgonest?`,
  );

  // Remove nestia config files
  if (pkgInfo.configFiles.length > 0) {
    const fileList = pkgInfo.configFiles.join(", ");
    plan.removeNestiaConfigs = await confirm(
      `${CYAN}7.${RESET} Remove Nestia config files? (${fileList})`,
    );
  }

  console.log();
  return plan;
}

// ── Config generation ────────────────────────────────────────────────────────

function generateConfigFile(cwd: string, report: MigrateReport): void {
  const configPath = join(cwd, "tsgonest.config.ts");
  if (existsSync(configPath)) {
    report.info(configPath, "general", "tsgonest.config.ts already exists, skipping generation");
    return;
  }

  const lines: string[] = [];
  lines.push('import { defineConfig } from "@tsgonest/runtime";');
  lines.push("");
  lines.push("export default defineConfig({");
  lines.push('  controllers: { include: ["src/**/*.controller.ts"] },');
  lines.push("  transforms: { validation: true, serialization: true },");
  lines.push("  openapi: {");
  lines.push('    output: "dist/openapi.json",');

  // Add auto-detected security schemes
  if (report.detectedSecuritySchemes.size > 0) {
    lines.push("    securitySchemes: {");
    for (const [name, config] of report.detectedSecuritySchemes) {
      const entries = Object.entries(config)
        .map(([k, v]) => `${k}: "${v}"`)
        .join(", ");
      lines.push(`      ${name}: { ${entries} },`);
    }
    lines.push("    },");
  }

  lines.push("  },");
  lines.push("});");
  lines.push("");

  writeFileSync(configPath, lines.join("\n"));
  report.configFileGenerated = true;
}

// ── tsconfig fix ─────────────────────────────────────────────────────────────

function fixTsconfig(cwd: string, report: MigrateReport): void {
  const tsconfigPath = join(cwd, "tsconfig.json");
  if (!existsSync(tsconfigPath)) return;

  let text = readFileSync(tsconfigPath, "utf-8");

  const errors: any[] = [];
  const config = parseJsonc(text, errors, { allowTrailingComma: true });
  if (errors.length > 0 || !config) {
    report.warn(tsconfigPath, "general", "Could not parse tsconfig.json — manual review needed");
    return;
  }

  const compilerOptions = config.compilerOptions;
  if (!compilerOptions) return;

  const editOptions: ModificationOptions = {
    formattingOptions: { tabSize: 2, insertSpaces: true },
  };

  // Remove baseUrl (tsgo doesn't support it)
  if (compilerOptions.baseUrl !== undefined) {
    const edits = modify(text, ["compilerOptions", "baseUrl"], undefined, editOptions);
    text = applyEdits(text, edits);
  }

  // Remove nestia/typia plugin entries
  if (Array.isArray(compilerOptions.plugins)) {
    // Find indices of plugins to remove (in reverse order to preserve indices)
    const indicesToRemove: number[] = [];
    for (let i = 0; i < compilerOptions.plugins.length; i++) {
      const transform = compilerOptions.plugins[i]?.transform ?? "";
      if (transform.includes("nestia") || transform.includes("typia")) {
        indicesToRemove.push(i);
      }
    }

    // Remove in reverse order so indices stay valid
    for (let i = indicesToRemove.length - 1; i >= 0; i--) {
      const edits = modify(text, ["compilerOptions", "plugins", indicesToRemove[i]], undefined, editOptions);
      text = applyEdits(text, edits);
    }

    // If all plugins were removed, remove the plugins array itself
    if (indicesToRemove.length === compilerOptions.plugins.length) {
      const edits = modify(text, ["compilerOptions", "plugins"], undefined, editOptions);
      text = applyEdits(text, edits);
    }
  }

  const originalText = readFileSync(tsconfigPath, "utf-8");
  if (text !== originalText) {
    writeFileSync(tsconfigPath, text);
    report.tsconfigModified = true;
  }
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main(): Promise<void> {
  const opts = parseArgs(process.argv.slice(2));
  const cwd = resolve(opts.cwd);

  console.log(`\n${BOLD}tsgonest migrate${RESET}${opts.apply ? "" : ` ${YELLOW}(dry-run)${RESET}`}\n`);

  // ── Step 0: Git clean check ──────────────────────────────────────────────
  if (isGitRepo(cwd) && !isGitClean(cwd) && !opts.force) {
    console.log(`  ${RED}Error: Git working directory has uncommitted changes.${RESET}\n`);
    console.log("  Please commit or stash your changes before migrating.");
    console.log("  This ensures you can easily review and revert the migration.\n");
    const status = getGitStatus(cwd);
    if (status) {
      console.log("  Uncommitted changes:");
      for (const line of status.split("\n").slice(0, 10)) {
        console.log(`    ${line}`);
      }
      if (status.split("\n").length > 10) {
        console.log(`    ... and ${status.split("\n").length - 10} more`);
      }
    }
    console.log(`\n  Use ${BOLD}--force${RESET} to skip this check.\n`);
    process.exit(1);
  }

  console.log(`  cwd: ${cwd}`);

  // ── Step 1: Detect packages in package.json ──────────────────────────────
  const pkgInfo = detectPackages(cwd);

  // ── Step 2: Create ts-morph project and scan files ───────────────────────
  const tsconfigPath = resolve(cwd, opts.tsconfig);
  let project: Project;
  try {
    project = new Project({ tsConfigFilePath: tsconfigPath, skipAddingFilesFromTsConfig: true });
  } catch {
    project = new Project({ compilerOptions: { strict: true } });
  }

  for (const pattern of opts.include) {
    project.addSourceFilesAtPaths(resolve(cwd, pattern));
  }

  const files = project.getSourceFiles();
  if (files.length === 0) {
    console.log("\n  No files matched. Try --include 'src/**/*.ts'");
    closePrompts();
    process.exit(0);
  }

  console.log(`  files: ${files.length}`);

  // ── Step 3: Detect migration sources ─────────────────────────────────────
  const detected = detectSources(files);
  const sources: string[] = [];
  if (detected.hasNestia) sources.push("nestia");
  if (detected.hasTypia) sources.push("typia");
  if (detected.hasClassValidator) sources.push("class-validator");
  if (detected.hasClassTransformer) sources.push("class-transformer");
  if (detected.hasSwagger) sources.push("@nestjs/swagger");

  // Also count package.json-only detections
  const pkgOnly: string[] = [];
  if (pkgInfo.hasNestiaDeps && !detected.hasNestia) pkgOnly.push("nestia (package.json only)");
  if (pkgInfo.hasClassDeps && !detected.hasClassValidator) pkgOnly.push("class-validator (package.json only)");
  if (pkgInfo.hasSwaggerDeps && !detected.hasSwagger) pkgOnly.push("@nestjs/swagger (package.json only)");

  const nothingFound =
    sources.length === 0 &&
    pkgOnly.length === 0 &&
    pkgInfo.configFiles.length === 0 &&
    pkgInfo.hasTsgonest;

  if (nothingFound) {
    console.log("\n  Nothing to migrate — no old imports or dependencies found.");
    closePrompts();
    process.exit(0);
  }

  if (sources.length > 0) console.log(`  detected in source: ${sources.join(", ")}`);
  if (pkgOnly.length > 0) console.log(`  detected in package.json: ${pkgOnly.join(", ")}`);
  if (pkgInfo.configFiles.length > 0) console.log(`  config files: ${pkgInfo.configFiles.join(", ")}`);

  // ── Step 4: Build migration plan (interactive when --apply) ──────────────
  const plan = await buildMigrationPlan(detected, pkgInfo, opts);

  // ── Step 5: Snapshot and transform source files ──────────────────────────
  const snapshots = new Map<SourceFile, string>();
  for (const file of files) {
    snapshots.set(file, file.getFullText());
  }

  const report = new MigrateReport();

  if (plan.transformSources && sources.length > 0) {
    for (const file of files) {
      if (detected.hasNestia) {
        report.stats.nestia += transformNestia(file, report);
      }
      if (detected.hasTypia) {
        report.stats.typia += transformTypiaTags(file, report);
      }
      if (detected.hasClassValidator) {
        report.stats.classValidator += transformClassValidator(file, report);
      }
      if (detected.hasClassTransformer) {
        report.stats.classTransformer += transformClassTransformer(file, report);
      }
      if (detected.hasSwagger) {
        report.stats.swagger += transformSwagger(file, report);
      }
      cleanupImports(file);
    }
  }

  // ── Step 6: Show diffs ───────────────────────────────────────────────────
  for (const file of files) {
    const original = snapshots.get(file)!;
    const modified = file.getFullText();
    if (showDiff(file.getFilePath(), original, modified, cwd)) {
      report.filesChanged.add(file.getFilePath());
    }
  }

  // Show package.json changes preview (dry-run)
  if (!opts.apply) {
    const pkgChanges: string[] = [];
    if (plan.removeNestiaDeps) pkgChanges.push(`remove: ${pkgInfo.nestiaDeps.join(", ")}`);
    if (plan.removeClassDeps) pkgChanges.push(`remove: ${pkgInfo.classDeps.join(", ")}`);
    if (plan.removeSwaggerDeps) pkgChanges.push(`remove: ${pkgInfo.swaggerDeps.join(", ")}`);
    if (plan.addTsgonestDeps) pkgChanges.push("add: tsgonest, @tsgonest/runtime, @tsgonest/types");
    if (plan.updateBuildScripts) pkgChanges.push("update: build/dev scripts");
    if (plan.removeNestiaConfigs) pkgChanges.push(`delete: ${pkgInfo.configFiles.join(", ")}`);

    if (pkgChanges.length > 0) {
      console.log(`\n${BOLD}  package.json changes (planned):${RESET}`);
      for (const change of pkgChanges) {
        console.log(`    ${change}`);
      }
    }
  }

  // Print report summary
  report.printSummary(cwd);

  // ── Step 7: Apply changes ────────────────────────────────────────────────
  if (opts.apply) {
    // 7a. Write source file changes
    if (report.filesChanged.size > 0) {
      await project.save();
    }

    // 7b. Remove old dependencies
    const allRemovedDeps: string[] = [];
    if (plan.removeNestiaDeps) {
      allRemovedDeps.push(...removeDependencies(cwd, pkgInfo.nestiaDeps));
    }
    if (plan.removeClassDeps) {
      allRemovedDeps.push(...removeDependencies(cwd, pkgInfo.classDeps));
    }
    if (plan.removeSwaggerDeps) {
      allRemovedDeps.push(...removeDependencies(cwd, pkgInfo.swaggerDeps));
    }
    if (allRemovedDeps.length > 0) {
      console.log(`\n  Removed dependencies: ${allRemovedDeps.join(", ")}`);
    }

    // 7c. Add tsgonest dependencies
    if (plan.addTsgonestDeps) {
      const added = addTsgonestDependencies(cwd);
      if (added.length > 0) {
        console.log(`  Added dependencies: ${added.join(", ")}`);
      }
    }

    // 7d. Update scripts
    if (plan.updateBuildScripts) {
      const updated = updateScripts(cwd);
      if (updated.length > 0) {
        console.log(`  Updated scripts: ${updated.join(", ")}`);
      }
    }

    // 7e. Remove config files
    if (plan.removeNestiaConfigs) {
      const removed = removeConfigFiles(cwd, pkgInfo.configFiles);
      if (removed.length > 0) {
        console.log(`  Removed config files: ${removed.join(", ")}`);
      }
    }

    // 7f. Auto-generate tsgonest.config.ts
    generateConfigFile(cwd, report);
    if (report.configFileGenerated) {
      console.log(`  Generated: tsgonest.config.ts`);
    }

    // 7g. Auto-fix tsconfig.json
    fixTsconfig(cwd, report);
    if (report.tsconfigModified) {
      console.log(`  Modified: tsconfig.json (removed baseUrl/plugins)`);
    }

    // Track package.json changes in report
    report.packageJsonChanges = {
      removedDeps: allRemovedDeps,
      addedDeps: plan.addTsgonestDeps ? ["tsgonest", "@tsgonest/runtime", "@tsgonest/types"] : [],
      updatedScripts: plan.updateBuildScripts ? ["build", "dev"] : [],
      removedConfigFiles: plan.removeNestiaConfigs ? pkgInfo.configFiles : [],
    };

    // Write markdown report
    const reportPath = join(cwd, "tsgonest-migrate-report.md");
    writeFileSync(reportPath, report.toMarkdown(cwd));

    console.log(`\n  ${GREEN}Migration complete.${RESET}`);
    console.log(`  Report: ${relative(cwd, reportPath)}`);

    // Remind to install
    if (allRemovedDeps.length > 0 || plan.addTsgonestDeps) {
      console.log(`\n  ${BOLD}Next:${RESET} Run your package manager to sync dependencies:`);
      console.log("    npm install  /  pnpm install  /  yarn install\n");
    }
  } else {
    const totalChanges =
      report.filesChanged.size +
      (plan.removeNestiaDeps ? pkgInfo.nestiaDeps.length : 0) +
      (plan.removeClassDeps ? pkgInfo.classDeps.length : 0) +
      (plan.removeSwaggerDeps ? pkgInfo.swaggerDeps.length : 0) +
      (plan.addTsgonestDeps ? 3 : 0) +
      (plan.updateBuildScripts ? 1 : 0) +
      (plan.removeNestiaConfigs ? pkgInfo.configFiles.length : 0);

    if (totalChanges === 0) {
      console.log("\n  Nothing to change.");
    } else {
      console.log(`\n  Run with ${BOLD}--apply${RESET} to write changes.`);
    }
  }

  closePrompts();
}

main().catch((err) => {
  closePrompts();
  console.error("tsgonest migrate: fatal error");
  console.error(err);
  process.exit(1);
});
