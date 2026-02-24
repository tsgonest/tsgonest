/**
 * class-transformer → tsgonest transforms.
 *
 * - @Type(() => Number) → adds & tags.Coerce to the property type
 * - @Transform(({ value }) => value?.trim()) → adds & tags.Trim
 * - @Transform(({ value }) => value?.toLowerCase()) → adds & tags.ToLowerCase
 * - @Transform(({ value }) => value?.toUpperCase()) → adds & tags.ToUpperCase
 * - @Exclude() → keep property, add TODO comment (user decides whether to omit from response type)
 * - @Expose() → remove decorator (no-op in tsgonest, property stays)
 * - @plainToInstance / @instanceToPlain → remove
 */

import { SourceFile, Decorator } from "ts-morph";
import { MigrateReport } from "../report.js";
import { ensureTagsImport } from "./imports.js";

interface DecoratorAction {
  tag: string | null;
  addTodo: string | null;
}

function mapTransformerDecorator(decorator: Decorator): DecoratorAction {
  const name = decorator.getName();
  const args = decorator.getArguments();

  switch (name) {
    case "Type": {
      const argText = args[0]?.getText() ?? "";
      if (argText.includes("Number") || argText.includes("Boolean")) {
        return { tag: "tags.Coerce", addTodo: null };
      }
      return { tag: null, addTodo: null };
    }

    case "Transform": {
      const argText = args[0]?.getText() ?? "";
      if (argText.includes(".trim()") || argText.includes(".trim?.()")) {
        return { tag: "tags.Trim", addTodo: null };
      }
      if (argText.includes(".toLowerCase()") || argText.includes(".toLowerCase?.()")) {
        return { tag: "tags.ToLowerCase", addTodo: null };
      }
      if (argText.includes(".toUpperCase()") || argText.includes(".toUpperCase?.()")) {
        return { tag: "tags.ToUpperCase", addTodo: null };
      }
      return { tag: null, addTodo: "@Transform() was here — migrate manually" };
    }

    case "Exclude":
      return {
        tag: null,
        addTodo: "@Exclude() was here — omit this property from your response type/interface if you don't want it serialized",
      };

    case "Expose":
    case "plainToInstance":
    case "instanceToPlain":
    case "plainToClass":
    case "classToPlain":
      return { tag: null, addTodo: null };

    default:
      return { tag: null, addTodo: null };
  }
}

/**
 * Transform class-transformer decorators.
 */
export function transformClassTransformer(file: SourceFile, report: MigrateReport): number {
  const classTransformerImport = file.getImportDeclaration(
    (d) => d.getModuleSpecifierValue() === "class-transformer",
  );
  if (!classTransformerImport) return 0;

  let count = 0;
  const filePath = file.getFilePath();
  let needsTagsImport = false;

  for (const cls of file.getClasses()) {
    for (const prop of cls.getProperties()) {
      const brandedTypes: string[] = [];
      const decoratorsToRemove: Decorator[] = [];

      for (const dec of prop.getDecorators()) {
        if (["Type", "Transform", "Exclude", "Expose", "plainToInstance", "instanceToPlain", "plainToClass", "classToPlain"].includes(dec.getName())) {
          const result = mapTransformerDecorator(dec);
          if (result.tag) {
            brandedTypes.push(result.tag);
            needsTagsImport = true;
          }
          if (result.addTodo) {
            report.todo(filePath, "class-transformer", result.addTodo, dec.getStartLineNumber());
          }
          decoratorsToRemove.push(dec);
        }
      }

      // Remove class-transformer decorators
      for (const dec of decoratorsToRemove) {
        dec.remove();
        count++;
      }

      // Append branded types to property type
      if (brandedTypes.length > 0) {
        const currentType = prop.getTypeNode()?.getText() ?? "any";
        prop.setType(`${currentType} & ${brandedTypes.join(" & ")}`);
        count++;
      }
    }
  }

  // Remove class-transformer import
  if (count > 0) {
    classTransformerImport.remove();

    if (needsTagsImport) {
      ensureTagsImport(file);
    }
  }

  return count;
}
