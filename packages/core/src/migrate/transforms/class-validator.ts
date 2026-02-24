/**
 * class-validator → tsgonest transforms (*.dto.ts files only).
 *
 * Converts class DTOs with decorator-based validation to interfaces with
 * branded types from @tsgonest/types.
 */

import { SourceFile, Decorator } from "ts-morph";
import { MigrateReport } from "../report.js";
import { ensureTagsImport } from "./imports.js";

/**
 * Decorators from other libraries that happen to be on properties alongside
 * class-validator decorators. We should never flag or remove these.
 */
const NON_CLASS_VALIDATOR_DECORATORS = new Set([
  // @nestjs/graphql
  "Field", "HideField", "IDField", "FilterableField", "ObjectType", "InputType",
  "ArgsType", "Extensions", "Directive", "InterfaceType",
  // @nestjs/swagger (handled by swagger transform)
  "ApiProperty", "ApiPropertyOptional", "ApiHideProperty", "ApiResponseProperty",
  // class-transformer (handled by class-transformer transform)
  "Type", "Transform", "Exclude", "Expose", "plainToInstance", "instanceToPlain",
  "plainToClass", "classToPlain",
  // NestJS core
  "Inject", "Optional",
  // TypeORM / MikroORM / Prisma decorators commonly on entity properties
  "Column", "PrimaryColumn", "PrimaryGeneratedColumn", "CreateDateColumn",
  "UpdateDateColumn", "DeleteDateColumn", "VersionColumn", "Generated",
  "ManyToOne", "OneToMany", "ManyToMany", "OneToOne", "JoinColumn", "JoinTable",
  "Index", "Unique", "RelationId", "TreeParent", "TreeChildren",
  "Property", "Enum", "Formula",
  // Sequelize
  "AllowNull", "Default", "ForeignKey", "BelongsTo", "HasMany",
]);

/** Map class-validator decorator → branded type expression (or null to just remove). */
function decoratorToBrandedType(
  decorator: Decorator,
): { tag: string | null; makeOptional: boolean; known: boolean } {
  const name = decorator.getName();
  const args = decorator.getArguments().map((a) => a.getText());

  switch (name) {
    // Constraints → branded types
    case "Min":
      return { tag: args[0] ? `tags.Minimum<${args[0]}>` : null, makeOptional: false, known: true };
    case "Max":
      return { tag: args[0] ? `tags.Maximum<${args[0]}>` : null, makeOptional: false, known: true };
    case "MinLength":
      return { tag: args[0] ? `tags.MinLength<${args[0]}>` : null, makeOptional: false, known: true };
    case "MaxLength":
      return { tag: args[0] ? `tags.MaxLength<${args[0]}>` : null, makeOptional: false, known: true };
    case "IsEmail":
      return { tag: "tags.Email", makeOptional: false, known: true };
    case "IsUUID":
      return { tag: "tags.Uuid", makeOptional: false, known: true };
    case "IsUrl":
    case "IsURL":
      return { tag: "tags.Url", makeOptional: false, known: true };
    case "Matches": {
      if (args[0]) {
        const pattern = args[0].replace(/^\/|\/[gimsuy]*$/g, "");
        return { tag: `tags.Pattern<${JSON.stringify(pattern)}>`, makeOptional: false, known: true };
      }
      return { tag: null, makeOptional: false, known: true };
    }
    case "IsDateString":
      return { tag: `tags.Format<"date-time">`, makeOptional: false, known: true };
    case "IsNotEmpty":
      return { tag: "tags.MinLength<1>", makeOptional: false, known: true };
    case "IsPositive":
      return { tag: "tags.Positive", makeOptional: false, known: true };
    case "IsNegative":
      return { tag: "tags.Negative", makeOptional: false, known: true };
    case "IsInt":
      return { tag: "tags.Int", makeOptional: false, known: true };
    case "ArrayMinSize":
      return { tag: args[0] ? `tags.MinItems<${args[0]}>` : null, makeOptional: false, known: true };
    case "ArrayMaxSize":
      return { tag: args[0] ? `tags.MaxItems<${args[0]}>` : null, makeOptional: false, known: true };

    // Additional built-in class-validator decorators with branded type equivalents
    case "ArrayNotEmpty":
      return { tag: "tags.MinItems<1>", makeOptional: false, known: true };
    case "IsDate":
      return { tag: `tags.Format<"date-time">`, makeOptional: false, known: true };
    case "IsISO8601":
      return { tag: `tags.Format<"date-time">`, makeOptional: false, known: true };
    case "IsJSON":
      return { tag: null, makeOptional: false, known: true }; // TS type handles this
    case "IsIn": {
      // @IsIn(['a', 'b']) — can't easily express as branded type, flag for manual
      return { tag: null, makeOptional: false, known: false };
    }
    case "IsTimeZone":
      return { tag: null, makeOptional: false, known: true }; // No branded type equivalent, but recognized
    case "IsIP":
      return { tag: `tags.Format<"ipv4">`, makeOptional: false, known: true };
    case "IsCreditCard":
      return { tag: null, makeOptional: false, known: true };
    case "IsPhoneNumber":
      return { tag: null, makeOptional: false, known: true };
    case "IsHexColor":
      return { tag: null, makeOptional: false, known: true };
    case "IsMACAddress":
      return { tag: null, makeOptional: false, known: true };
    case "IsPort":
      return { tag: null, makeOptional: false, known: true };
    case "IsMimeType":
      return { tag: null, makeOptional: false, known: true };
    case "IsSemVer":
      return { tag: null, makeOptional: false, known: true };
    case "IsAlpha":
      return { tag: null, makeOptional: false, known: true };
    case "IsAlphanumeric":
      return { tag: null, makeOptional: false, known: true };
    case "IsNumberString":
      return { tag: null, makeOptional: false, known: true };
    case "IsBase64":
      return { tag: null, makeOptional: false, known: true };
    case "IsMongoId":
      return { tag: null, makeOptional: false, known: true };
    case "IsCurrency":
      return { tag: null, makeOptional: false, known: true };
    case "Contains":
      return { tag: null, makeOptional: false, known: true };
    case "NotContains":
      return { tag: null, makeOptional: false, known: true };
    case "IsHash":
      return { tag: null, makeOptional: false, known: true };
    case "IsLatitude":
      return { tag: null, makeOptional: false, known: true };
    case "IsLongitude":
      return { tag: null, makeOptional: false, known: true };
    case "IsLatLong":
      return { tag: null, makeOptional: false, known: true };
    case "Length":
      return { tag: args[0] ? `tags.MinLength<${args[0]}>` : null, makeOptional: false, known: true };

    // Optional marker
    case "IsOptional":
      return { tag: null, makeOptional: true, known: true };

    // Validation group / conditional — just remove
    case "Allow":
      return { tag: null, makeOptional: false, known: true };

    // Type validators — redundant with TS types, just remove
    case "IsString":
    case "IsNumber":
    case "IsBoolean":
    case "IsArray":
    case "IsObject":
    case "IsEnum":
    case "IsDefined":
    case "IsNotEmptyObject":
    case "ValidateNested":
    case "ValidateIf":
    case "Validate":
    case "IsEmpty":
    case "Equals":
    case "NotEquals":
    case "IsInstance":
      return { tag: null, makeOptional: false, known: true };

    default:
      return { tag: null, makeOptional: false, known: false };
  }
}

/**
 * Transform class-validator DTOs to interfaces with branded types.
 * Only processes *.dto.ts files.
 */
export function transformClassValidator(file: SourceFile, report: MigrateReport): number {
  // Only transform DTO files
  if (!file.getFilePath().endsWith(".dto.ts")) return 0;

  const cvImport = file.getImportDeclaration(
    (d) => d.getModuleSpecifierValue() === "class-validator",
  );
  if (!cvImport) return 0;

  let count = 0;
  const filePath = file.getFilePath();
  let needsTagsImport = false;

  // Track unknown decorators for TODOs
  const unknownDecorators = new Set<string>();

  // Process each class in the file
  const classes = file.getClasses();
  for (const cls of classes) {
    // Skip if it looks like a controller/module/injectable (not a DTO)
    const classDecorators = cls.getDecorators();
    const isNestClass = classDecorators.some((d) =>
      ["Controller", "Module", "Injectable", "Guard", "Interceptor", "Pipe", "Filter"].includes(d.getName()),
    );
    if (isNestClass) continue;

    const properties = cls.getProperties();
    const interfaceProps: string[] = [];

    for (const prop of properties) {
      const propName = prop.getName();
      const typeNode = prop.getTypeNode();
      let typeText = typeNode ? typeNode.getText() : "any";
      let isOptional = prop.hasQuestionToken();
      const brandedTypes: string[] = [];

      // Process decorators
      for (const dec of prop.getDecorators()) {
        const decName = dec.getName();
        // Skip decorators from other libraries (GraphQL, swagger, class-transformer, TypeORM, etc.)
        if (decName.startsWith("Api")) continue;
        if (NON_CLASS_VALIDATOR_DECORATORS.has(decName)) continue;

        const result = decoratorToBrandedType(dec);
        if (result.tag) {
          brandedTypes.push(result.tag);
          needsTagsImport = true;
        }
        if (result.makeOptional) {
          isOptional = true;
        }
        // Track unhandled/unknown decorators (likely custom validators)
        if (!result.known) {
          unknownDecorators.add(decName);
        }
      }

      // Build interface property
      let fullType = typeText;
      if (brandedTypes.length > 0) {
        fullType = `${typeText} & ${brandedTypes.join(" & ")}`;
      }

      // Preserve JSDoc comments
      const jsDocs = prop.getJsDocs();
      const jsDocText = jsDocs.map((d) => d.getText()).join("\n");

      let propLine = "";
      if (jsDocText) {
        propLine += `\t${jsDocText}\n`;
      }
      propLine += `\t${propName}${isOptional ? "?" : ""}: ${fullType};`;

      interfaceProps.push(propLine);
      count++;
    }

    // Replace class with interface
    const className = cls.getName();
    if (!className) continue;

    const isExported = cls.isExported();
    const isDefault = cls.isDefaultExport();

    let interfaceText = "";
    if (isExported && isDefault) {
      interfaceText = `export default interface ${className}`;
    } else if (isExported) {
      interfaceText = `export interface ${className}`;
    } else {
      interfaceText = `interface ${className}`;
    }

    const extendsExpr = cls.getExtends();
    if (extendsExpr) {
      interfaceText += ` extends ${extendsExpr.getText()}`;
    }

    interfaceText += ` {\n${interfaceProps.join("\n")}\n}`;

    cls.replaceWithText(interfaceText);
    report.info(filePath, "class-validator", `Converted class ${className} → interface ${className}`);
    count++;
  }

  // Report unknown decorators as TODOs
  for (const dec of unknownDecorators) {
    report.todo(filePath, "class-validator",
      `Unknown class-validator decorator @${dec}() — may need manual migration.`,
    );
  }

  // Remove class-validator import
  if (count > 0) {
    cvImport.remove();

    if (needsTagsImport) {
      ensureTagsImport(file);
    }
  }

  return count;
}
