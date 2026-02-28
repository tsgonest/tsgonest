/**
 * baseUrl-relative import rewriting.
 *
 * When tsconfig.json has `"baseUrl": "."`, projects often use bare imports like:
 *   import { X } from 'src/users/dto/user.dto'
 *
 * Since tsgonest migrate removes `baseUrl` (tsgo doesn't support it), these
 * imports break. This transform rewrites them to relative paths before other
 * transforms run.
 */

import { SourceFile } from "ts-morph";
import { resolve, relative, dirname, sep } from "path";
import { existsSync } from "fs";
import { MigrateReport } from "../report.js";

const TS_EXTENSIONS = [".ts", ".tsx", "/index.ts", "/index.tsx"];

/**
 * Rewrite baseUrl-relative imports to relative paths.
 * Must run BEFORE other transforms — operates on raw import specifiers.
 *
 * @param file       The source file to transform
 * @param baseDir    The absolute directory that baseUrl resolves to (e.g., project root)
 * @param report     Migration report for tracking changes
 * @param fileExists Optional file existence checker (defaults to fs.existsSync, injectable for tests)
 * @returns Number of rewrites applied
 */
export function rewriteBaseUrlImports(
  file: SourceFile,
  baseDir: string,
  report: MigrateReport,
  fileExists: (path: string) => boolean = existsSync,
): number {
  let count = 0;
  const filePath = file.getFilePath();
  const fileDir = dirname(filePath);

  for (const imp of file.getImportDeclarations()) {
    const specifier = imp.getModuleSpecifierValue();

    // Skip relative imports (already correct)
    if (specifier.startsWith(".")) continue;

    // Skip scoped packages (@nestjs/common, @tsgonest/types, etc.)
    if (specifier.startsWith("@")) continue;

    // Skip node built-ins and obvious package names (no slash or starts with known prefix)
    // We only care about specifiers that look like project paths (e.g., 'src/users/dto/...')
    // Heuristic: try to resolve against baseDir
    const candidate = resolve(baseDir, specifier);

    let resolvedPath: string | null = null;

    // Try exact path first (e.g., 'src/utils' → 'src/utils.ts')
    for (const ext of TS_EXTENSIONS) {
      const tryPath = candidate + ext;
      if (fileExists(tryPath)) {
        resolvedPath = tryPath;
        break;
      }
    }

    // Also try if the candidate itself exists (e.g., .js files, though rare in TS projects)
    if (!resolvedPath && fileExists(candidate)) {
      resolvedPath = candidate;
    }

    if (!resolvedPath) continue;

    // Compute relative path from the importing file's directory
    let relativePath = relative(fileDir, resolvedPath);

    // Normalize to forward slashes (Windows)
    relativePath = relativePath.split(sep).join("/");

    // Strip .ts/.tsx extension (TypeScript import convention)
    relativePath = relativePath.replace(/\.tsx?$/, "");

    // Strip /index suffix (TypeScript resolves index files automatically)
    relativePath = relativePath.replace(/\/index$/, "");

    // Ensure ./ prefix for same-directory or child imports
    if (!relativePath.startsWith(".")) {
      relativePath = "./" + relativePath;
    }

    imp.setModuleSpecifier(relativePath);
    count++;

    report.info(filePath, "general",
      `Rewrote baseUrl import '${specifier}' → '${relativePath}'`,
    );
  }

  return count;
}
