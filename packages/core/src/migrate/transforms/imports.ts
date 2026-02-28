/**
 * Import cleanup utilities.
 *
 * After all transforms run, some imports may be empty (all named imports removed).
 * This pass removes those empty import declarations.
 */

import { SourceFile } from "ts-morph";

/**
 * Ensure the file has an import for `{ tags }` from `@tsgonest/types`.
 * If one already exists, this is a no-op.
 */
export function ensureTagsImport(file: SourceFile): void {
  const existing = file.getImportDeclaration(
    (d) => d.getModuleSpecifierValue() === "@tsgonest/types",
  );
  if (!existing) {
    file.addImportDeclaration({
      moduleSpecifier: "@tsgonest/types",
      namedImports: ["tags"],
    });
  }
}

/** Packages whose imports should be pruned after all transforms. */
const PRUNE_PACKAGES = new Set([
  "class-validator",
  "class-transformer",
  "@nestjs/swagger",
]);

/**
 * Remove import declarations that have no named imports, no default import,
 * and no namespace import (i.e., completely empty after transforms).
 * Also prune individual unused named imports from migration-target packages.
 */
export function cleanupImports(file: SourceFile): void {
  const imports = file.getImportDeclarations();

  // Build the file text once (without import lines) for reference checking
  const fullText = file.getFullText();

  for (const imp of [...imports]) {
    const hasDefault = imp.getDefaultImport() !== undefined;
    const hasNamespace = imp.getNamespaceImport() !== undefined;
    const hasNamed = imp.getNamedImports().length > 0;
    const moduleSpecifier = imp.getModuleSpecifierValue();

    // Remove completely empty imports (had {} but all named imports were removed)
    const isSideEffect = !hasDefault && !hasNamespace && !hasNamed;
    if (isSideEffect && imp.getText().includes("{")) {
      imp.remove();
      continue;
    }

    // For migration-target packages, prune individual unused named imports.
    // Other transforms (e.g., class→interface conversion) may have removed all
    // usages of a named import without touching the import declaration.
    if (PRUNE_PACKAGES.has(moduleSpecifier) && hasNamed) {
      const importText = imp.getText();
      const withoutImport = fullText.replace(importText, "");
      const namedImports = imp.getNamedImports();
      const unused = namedImports.filter((n) => !withoutImport.includes(n.getName()));

      if (unused.length === namedImports.length) {
        imp.remove();
      } else if (unused.length > 0) {
        for (const n of unused.reverse()) {
          n.remove();
        }
      }
    }
  }

  // Collapse consecutive blank/whitespace-only lines left by decorator removal.
  // Any run of 2+ lines that are empty or whitespace-only → single blank line.
  const text = file.getFullText();
  const collapsed = text.replace(/\n([ \t]*\n){2,}/g, "\n\n");
  if (collapsed !== text) {
    file.replaceWithText(collapsed);
  }
}
