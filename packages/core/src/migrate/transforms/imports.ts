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

/**
 * Remove import declarations that have no named imports, no default import,
 * and no namespace import (i.e., completely empty after transforms).
 */
export function cleanupImports(file: SourceFile): void {
  const imports = file.getImportDeclarations();

  for (const imp of imports) {
    const hasDefault = imp.getDefaultImport() !== undefined;
    const hasNamespace = imp.getNamespaceImport() !== undefined;
    const hasNamed = imp.getNamedImports().length > 0;

    // Side-effect imports (import 'reflect-metadata') have none of the above
    // but should be kept
    const moduleSpecifier = imp.getModuleSpecifierValue();
    const isSideEffect = !hasDefault && !hasNamespace && !hasNamed;

    // Only remove if it had imports before (not a side-effect import)
    // We can detect this by checking if the import text contains {}
    if (isSideEffect && imp.getText().includes("{")) {
      imp.remove();
    }
  }

  // Also sort remaining imports (optional, nice to have)
  // Skip this to minimize diff noise
}
