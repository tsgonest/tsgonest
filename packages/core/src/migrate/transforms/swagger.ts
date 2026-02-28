/**
 * @nestjs/swagger cleanup.
 *
 * tsgonest generates OpenAPI from static analysis, so most swagger decorators
 * become redundant. Security scheme decorators are auto-detected and collected
 * for the generated tsgonest.config.ts.
 */

import { SourceFile, Node } from "ts-morph";
import { MigrateReport } from "../report.js";

/** Decorators that can be removed outright (functionality covered by tsgonest). */
const REMOVABLE_DECORATORS = new Set([
  "ApiTags",
  "ApiProperty",
  "ApiPropertyOptional",
  "ApiBody",
  "ApiConsumes",
  "ApiOperation",
  "ApiResponse",
  "ApiOkResponse",
  "ApiCreatedResponse",
  "ApiNotFoundResponse",
  "ApiBadRequestResponse",
  "ApiUnauthorizedResponse",
  "ApiForbiddenResponse",
  "ApiInternalServerErrorResponse",
  "ApiQuery",
  "ApiParam",
  "ApiHeader",
  "ApiExcludeEndpoint",
  "ApiExcludeController",
  "ApiExtraModels",
  "ApiHideProperty",
]);

/** Security scheme decorators → auto-detected config. */
const SECURITY_DECORATORS: Record<string, { name: string; config: Record<string, string> }> = {
  ApiBearerAuth: { name: "bearer", config: { type: "http", scheme: "bearer" } },
  ApiBasicAuth: { name: "basic", config: { type: "http", scheme: "basic" } },
  ApiCookieAuth: { name: "cookie", config: { type: "apiKey", in: "cookie", name: "session" } },
  ApiSecurity: { name: "custom", config: { type: "http", scheme: "bearer" } },
  ApiOAuth2: { name: "oauth2", config: { type: "oauth2" } },
};

/**
 * Remove @nestjs/swagger decorators and imports.
 * Security scheme decorators are collected into report.detectedSecuritySchemes
 * for auto-generation of tsgonest.config.ts.
 */
export function transformSwagger(file: SourceFile, report: MigrateReport): number {
  const swaggerImport = file.getImportDeclaration(
    (d) => d.getModuleSpecifierValue() === "@nestjs/swagger",
  );
  if (!swaggerImport) return 0;

  let count = 0;
  const filePath = file.getFilePath();

  // Check for SwaggerModule / DocumentBuilder / NestiaSwaggerComposer in main.ts
  const fullText = file.getFullText();
  if (fullText.includes("SwaggerModule") || fullText.includes("DocumentBuilder")) {
    report.todo(filePath, "swagger",
      "Remove SwaggerModule.setup() and DocumentBuilder — tsgonest generates OpenAPI at build time. Configure in tsgonest.config.ts → openapi.",
    );
  }
  if (fullText.includes("NestiaSwaggerComposer")) {
    report.todo(filePath, "swagger",
      "Remove NestiaSwaggerComposer — tsgonest generates OpenAPI at build time. Configure in tsgonest.config.ts → openapi.",
    );
  }

  // Collect swagger decorators first, then remove in reverse order
  // (removing during forEachDescendant corrupts the AST tree)
  const decoratorsToRemove: Node[] = [];

  file.forEachDescendant((node) => {
    if (!Node.isDecorator(node)) return;

    const name = node.getName();

    if (REMOVABLE_DECORATORS.has(name)) {
      decoratorsToRemove.push(node);
      return;
    }

    if (name in SECURITY_DECORATORS) {
      const schemeInfo = SECURITY_DECORATORS[name];
      report.detectedSecuritySchemes.set(schemeInfo.name, schemeInfo.config);
      report.info(filePath, "swagger",
        `Auto-detected ${name} → securitySchemes.${schemeInfo.name} (will be added to tsgonest.config.ts)`,
        node.getStartLineNumber(),
      );
      decoratorsToRemove.push(node);
    }
  });

  // Remove in reverse source order to avoid position invalidation
  for (const node of decoratorsToRemove.reverse()) {
    (node as import("ts-morph").Decorator).remove();
    count++;
  }

  // NestJS mapped-type helpers that exist in both @nestjs/swagger and @nestjs/mapped-types
  const MAPPED_TYPE_HELPERS = new Set([
    "OmitType", "PickType", "PartialType", "IntersectionType",
  ]);

  // Always clean up @nestjs/swagger import — other transforms (e.g., class-validator
  // class→interface conversion) may have removed decorator usages before us.
  {
    const remainingText = file.getFullText();
    const importText = swaggerImport.getText();
    const withoutImport = remainingText.replace(importText, "");

    // Remove individual named imports that are no longer referenced
    const namedImports = swaggerImport.getNamedImports();
    const toRemove = namedImports.filter((n) => !withoutImport.includes(n.getName()));

    if (toRemove.length === namedImports.length) {
      // All imports unused — remove the entire import declaration
      swaggerImport.remove();
      count++;
    } else if (toRemove.length > 0) {
      // Remove only unused named imports (in reverse to preserve positions)
      for (const imp of toRemove.reverse()) {
        imp.remove();
        count++;
      }

      // After pruning, check if remaining imports are all mapped-type helpers.
      // If so, rewrite the import source from @nestjs/swagger → @nestjs/mapped-types
      const remaining = swaggerImport.getNamedImports();
      if (remaining.length > 0 && remaining.every((n) => MAPPED_TYPE_HELPERS.has(n.getName()))) {
        swaggerImport.setModuleSpecifier("@nestjs/mapped-types");
        report.info(filePath, "swagger",
          `Rewrote mapped-type import from @nestjs/swagger → @nestjs/mapped-types`,
        );
        count++;
      }
    }
  }

  return count;
}
