/**
 * Nestia → tsgonest controller transforms.
 *
 * - @TypedRoute.Get() → @Get()  (same for Post, Put, Patch, Delete)
 * - @TypedBody() → @Body()
 * - @TypedParam<T>('name') → @Param('name')
 * - @TypedQuery() → @Query()
 * - @TypedFormData.Body() → @FormDataBody() + @UseInterceptors(FormDataInterceptor)
 * - SwaggerCustomizer → remove + TODO
 * - Remove @nestia/core import, merge into @nestjs/common
 */

import { SourceFile, Node } from "ts-morph";
import { MigrateReport } from "../report.js";

/** Map of Nestia decorator usage → NestJS common equivalent. */
const TYPED_ROUTE_MAP: Record<string, string> = {
  Get: "Get",
  Post: "Post",
  Put: "Put",
  Patch: "Patch",
  Delete: "Delete",
};

/**
 * Transform Nestia decorators to standard NestJS decorators.
 * Returns the number of transforms applied.
 */
export function transformNestia(file: SourceFile, report: MigrateReport): number {
  const nestiaImport = file.getImportDeclaration(
    (d) => d.getModuleSpecifierValue() === "@nestia/core",
  );
  if (!nestiaImport) return 0;

  let count = 0;
  const neededCommonImports = new Set<string>();
  const neededRuntimeImports = new Set<string>();
  const filePath = file.getFilePath();

  // Track methods that need @UseInterceptors(FormDataInterceptor)
  const methodsNeedingInterceptor = new Set<Node>();

  // Process all decorators in the file
  file.forEachDescendant((node) => {
    if (!Node.isDecorator(node)) return;

    const expr = node.getExpression();
    const text = expr.getText();

    // @TypedRoute.Get('path') → @Get('path')
    for (const [method, replacement] of Object.entries(TYPED_ROUTE_MAP)) {
      const pattern = `TypedRoute.${method}`;
      if (text.startsWith(pattern)) {
        const args = text.slice(pattern.length); // includes parens: ('path') or ()
        node.set({ expression: `${replacement}${args}` });
        neededCommonImports.add(replacement);
        count++;
        break;
      }
    }

    // @TypedBody() → @Body()
    if (text.startsWith("TypedBody")) {
      node.set({ expression: text.replace("TypedBody", "Body") });
      neededCommonImports.add("Body");
      count++;
    }

    // @TypedParam<Type>('name') or @TypedParam('name') → @Param('name')
    if (text.startsWith("TypedParam")) {
      const callMatch = text.match(/TypedParam(?:<[^>]*>)?\(([^)]*)\)/);
      if (callMatch) {
        node.set({ expression: `Param(${callMatch[1]})` });
        neededCommonImports.add("Param");
        count++;
      }
    }

    // @TypedQuery() → @Query()
    if (text.startsWith("TypedQuery")) {
      node.set({ expression: text.replace("TypedQuery", "Query") });
      neededCommonImports.add("Query");
      count++;
    }

    // @TypedFormData.Body(() => multerFactory()) → @FormDataBody(() => multerFactory())
    if (text.startsWith("TypedFormData.Body") || text.startsWith("TypedFormData")) {
      // Extract the factory argument: TypedFormData.Body(factory) → FormDataBody(factory)
      const newText = text.replace(/TypedFormData\.Body|TypedFormData/, "FormDataBody");
      node.set({ expression: newText });
      neededRuntimeImports.add("FormDataBody");
      neededRuntimeImports.add("FormDataInterceptor");
      neededCommonImports.add("UseInterceptors");

      // Find the parent method to add @UseInterceptors(FormDataInterceptor)
      let parent = node.getParent();
      while (parent && !Node.isMethodDeclaration(parent)) {
        parent = parent.getParent();
      }
      if (parent) {
        methodsNeedingInterceptor.add(parent);
      }

      count++;
    }

    // @SwaggerCustomizer(...) → remove + TODO
    if (text.startsWith("SwaggerCustomizer")) {
      const line = node.getStartLineNumber();
      node.remove();
      report.todo(filePath, "nestia",
        "Removed @SwaggerCustomizer(). Configure OpenAPI customizations in tsgonest.config.ts or via JSDoc tags instead.",
        line,
      );
      count++;
    }
  });

  // Add @UseInterceptors(FormDataInterceptor) to methods that use @FormDataBody
  for (const method of methodsNeedingInterceptor) {
    if (Node.isMethodDeclaration(method)) {
      // Check if method already has a @UseInterceptors decorator
      const existingDecorators = method.getDecorators();
      const hasUseInterceptors = existingDecorators.some(
        (d) => d.getExpression().getText().startsWith("UseInterceptors"),
      );
      if (!hasUseInterceptors) {
        method.addDecorator({
          name: "UseInterceptors",
          arguments: ["FormDataInterceptor"],
        });
      }
    }
  }

  // Remove @nestia/core import
  if (count > 0) {
    nestiaImport.remove();

    // Add needed imports to @nestjs/common
    if (neededCommonImports.size > 0) {
      const commonImport = file.getImportDeclaration(
        (d) => d.getModuleSpecifierValue() === "@nestjs/common",
      );

      if (commonImport) {
        const existing = new Set(
          commonImport.getNamedImports().map((n) => n.getName()),
        );
        for (const name of neededCommonImports) {
          if (!existing.has(name)) {
            commonImport.addNamedImport(name);
          }
        }
      } else {
        file.addImportDeclaration({
          moduleSpecifier: "@nestjs/common",
          namedImports: [...neededCommonImports].sort(),
        });
      }
    }

    // Add needed imports to @tsgonest/runtime
    if (neededRuntimeImports.size > 0) {
      const runtimeImport = file.getImportDeclaration(
        (d) => d.getModuleSpecifierValue() === "@tsgonest/runtime",
      );

      if (runtimeImport) {
        const existing = new Set(
          runtimeImport.getNamedImports().map((n) => n.getName()),
        );
        for (const name of neededRuntimeImports) {
          if (!existing.has(name)) {
            runtimeImport.addNamedImport(name);
          }
        }
      } else {
        file.addImportDeclaration({
          moduleSpecifier: "@tsgonest/runtime",
          namedImports: [...neededRuntimeImports].sort(),
        });
      }
    }
  }

  return count;
}
