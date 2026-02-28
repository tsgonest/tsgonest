/**
 * Typia → tsgonest type tag transforms.
 *
 * The main transform is changing the import source:
 *   import { tags } from 'typia' → import { tags } from '@tsgonest/types'
 *
 * @tsgonest/types deliberately mirrors typia's tag names (MinLength, MaxLength,
 * Minimum, Maximum, Format, MinItems, etc.) so type annotations don't change.
 *
 * Also handles:
 *   - Removing typia.assert<T>() / typia.is<T>() calls (replaced by ValidationPipe)
 *   - Removing typia.json.stringify<T>() calls (replaced by FastInterceptor)
 *   - TODO on tags.TagBase<> (custom validators need manual migration)
 */

import { SourceFile, Node } from "ts-morph";
import { MigrateReport } from "../report.js";

export function transformTypiaTags(file: SourceFile, report: MigrateReport): number {
  let count = 0;
  const filePath = file.getFilePath();

  // 1. Change import source: 'typia' → '@tsgonest/types'
  const typiaImports = file.getImportDeclarations().filter(
    (d) => d.getModuleSpecifierValue() === "typia",
  );

  for (const imp of typiaImports) {
    const namedImports = imp.getNamedImports();
    const defaultImport = imp.getDefaultImport();
    const hasTagsImport = namedImports.some((n) => n.getName() === "tags");
    const hasDefaultImport = defaultImport !== undefined;

    if (hasTagsImport && !hasDefaultImport) {
      // import { tags } from 'typia' → import { tags } from '@tsgonest/types'
      imp.setModuleSpecifier("@tsgonest/types");
      count++;
    } else if (hasTagsImport && hasDefaultImport) {
      // import typia, { tags } from 'typia'
      imp.setModuleSpecifier("@tsgonest/types");
      imp.removeDefaultImport();
      count++;
    } else if (hasDefaultImport && !hasTagsImport) {
      // import typia from 'typia' — handled below after stripping calls
    }
  }

  // 2. TODO on tags.TagBase<> usage (custom validators need manual review)
  const fullText = file.getFullText();
  if (fullText.includes("tags.TagBase") || fullText.includes("TagBase<")) {
    report.todo(filePath, "typia",
      "Found tags.TagBase<> (custom typia validator). Migrate manually to @tsgonest/types Validate<typeof fn> pattern.",
    );
  }

  // 3. Remove typia.assert<T>() / typia.is<T>() / typia.json.stringify<T>() calls
  file.forEachDescendant((node) => {
    if (!Node.isCallExpression(node)) return;

    const expr = node.getExpression().getText();

    // typia.assert<T>(input) → input
    if (expr.startsWith("typia.assert") || expr.startsWith("typia.is")) {
      const args = node.getArguments();
      if (args.length === 1) {
        const argText = args[0].getText();
        const line = node.getStartLineNumber();
        node.replaceWithText(argText);
        report.info(filePath, "typia",
          `Removed ${expr.split("(")[0]}() call — validation is handled by TsgonestValidationPipe.`,
          line,
        );
        count++;
      }
    }

    // typia.json.stringify<T>(input) → JSON.stringify(input)
    if (expr.startsWith("typia.json.stringify")) {
      const args = node.getArguments();
      if (args.length === 1) {
        const argText = args[0].getText();
        const line = node.getStartLineNumber();
        node.replaceWithText(`JSON.stringify(${argText})`);
        report.todo(filePath, "typia",
          "Replaced typia.json.stringify() with JSON.stringify(). For fast serialization, use TsgonestFastInterceptor instead of manual calls.",
          line,
        );
        count++;
      }
    }

    // typia.json.assertStringify<T>(input) → JSON.stringify(input)
    if (expr.startsWith("typia.json.assertStringify")) {
      const args = node.getArguments();
      if (args.length === 1) {
        const argText = args[0].getText();
        const line = node.getStartLineNumber();
        node.replaceWithText(`JSON.stringify(${argText})`);
        report.todo(filePath, "typia",
          "Replaced typia.json.assertStringify() with JSON.stringify(). Validation + serialization are handled by tsgonest runtime.",
          line,
        );
        count++;
      }
    }

    // typia.json.assertParse<T>(input) → JSON.parse(input)
    if (expr.startsWith("typia.json.assertParse") || expr.startsWith("typia.json.parse")) {
      const args = node.getArguments();
      if (args.length === 1) {
        const argText = args[0].getText();
        const line = node.getStartLineNumber();
        node.replaceWithText(`JSON.parse(${argText})`);
        report.todo(filePath, "typia",
          "Replaced typia.json.assertParse() with JSON.parse(). Add manual validation if needed.",
          line,
        );
        count++;
      }
    }
  });

  // 4. Remove default typia import if no longer referenced
  for (const imp of file.getImportDeclarations()) {
    if (imp.getModuleSpecifierValue() !== "typia") continue;
    const defaultImport = imp.getDefaultImport();
    if (!defaultImport) continue;

    const remaining = file.getFullText();
    const importLine = imp.getText();
    const withoutImport = remaining.replace(importLine, "");
    if (!withoutImport.includes("typia.")) {
      if (imp.getNamedImports().length === 0) {
        imp.remove();
        count++;
      } else {
        imp.removeDefaultImport();
        count++;
      }
    }
  }

  return count;
}
