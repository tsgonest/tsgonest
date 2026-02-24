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

  // Remove swagger decorators
  file.forEachDescendant((node) => {
    if (!Node.isDecorator(node)) return;

    const name = node.getName();

    if (REMOVABLE_DECORATORS.has(name)) {
      node.remove();
      count++;
      return;
    }

    if (name in SECURITY_DECORATORS) {
      const schemeInfo = SECURITY_DECORATORS[name];
      // Auto-detect and collect the security scheme for config generation
      report.detectedSecuritySchemes.set(schemeInfo.name, schemeInfo.config);
      report.info(filePath, "swagger",
        `Auto-detected ${name} → securitySchemes.${schemeInfo.name} (will be added to tsgonest.config.ts)`,
        node.getStartLineNumber(),
      );
      node.remove();
      count++;
    }
  });

  // Remove @nestjs/swagger import if no identifiers still referenced
  if (count > 0) {
    const remainingText = file.getFullText();
    const namedImports = swaggerImport.getNamedImports().map((n) => n.getName());
    const importText = swaggerImport.getText();
    const withoutImport = remainingText.replace(importText, "");
    const stillUsed = namedImports.some((name) => withoutImport.includes(name));

    if (!stillUsed) {
      swaggerImport.remove();
    }
  }

  return count;
}
