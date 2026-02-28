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
  // Typegoose / Mongoose
  "Prop", "prop",
]);

/** Map class-validator decorator → branded type expressions (or empty to just remove). */
function decoratorToBrandedType(
  decorator: Decorator,
): { tags: string[]; makeOptional: boolean; known: boolean } {
  const name = decorator.getName();
  const args = decorator.getArguments().map((a) => a.getText());

  switch (name) {
    // Constraints → branded types
    case "Min":
      return { tags: args[0] ? [`tags.Minimum<${args[0]}>`] : [], makeOptional: false, known: true };
    case "Max":
      return { tags: args[0] ? [`tags.Maximum<${args[0]}>`] : [], makeOptional: false, known: true };
    case "MinLength":
      return { tags: args[0] ? [`tags.MinLength<${args[0]}>`] : [], makeOptional: false, known: true };
    case "MaxLength":
      return { tags: args[0] ? [`tags.MaxLength<${args[0]}>`] : [], makeOptional: false, known: true };
    case "IsEmail":
      return { tags: ["tags.Email"], makeOptional: false, known: true };
    case "IsUUID":
      return { tags: ["tags.Uuid"], makeOptional: false, known: true };
    case "IsUrl":
    case "IsURL":
      return { tags: ["tags.Url"], makeOptional: false, known: true };
    case "Matches": {
      if (args[0]) {
        const pattern = args[0].replace(/^\/|\/[gimsuy]*$/g, "");
        return { tags: [`tags.Pattern<${JSON.stringify(pattern)}>`], makeOptional: false, known: true };
      }
      return { tags: [], makeOptional: false, known: true };
    }
    case "IsDateString":
      return { tags: [`tags.Format<"date-time">`], makeOptional: false, known: true };
    case "IsNotEmpty":
      return { tags: ["tags.MinLength<1>"], makeOptional: false, known: true };
    case "IsPositive":
      return { tags: ["tags.Positive"], makeOptional: false, known: true };
    case "IsNegative":
      return { tags: ["tags.Negative"], makeOptional: false, known: true };
    case "IsInt":
      return { tags: ["tags.Int"], makeOptional: false, known: true };
    case "ArrayMinSize":
      return { tags: args[0] ? [`tags.MinItems<${args[0]}>`] : [], makeOptional: false, known: true };
    case "ArrayMaxSize":
      return { tags: args[0] ? [`tags.MaxItems<${args[0]}>`] : [], makeOptional: false, known: true };

    // Additional built-in class-validator decorators with branded type equivalents
    case "ArrayNotEmpty":
      return { tags: ["tags.MinItems<1>"], makeOptional: false, known: true };
    case "IsDate":
      return { tags: [`tags.Format<"date-time">`], makeOptional: false, known: true };
    case "IsISO8601":
      return { tags: [`tags.Format<"date-time">`], makeOptional: false, known: true };
    case "IsJSON":
      return { tags: [], makeOptional: false, known: true }; // TS type handles this
    case "IsIn": {
      // @IsIn(['a', 'b']) — can't easily express as branded type, flag for manual
      return { tags: [], makeOptional: false, known: false };
    }
    case "IsTimeZone":
      return { tags: [], makeOptional: false, known: true }; // No branded type equivalent, but recognized
    case "IsIP":
      return { tags: [`tags.Format<"ipv4">`], makeOptional: false, known: true };
    case "IsCreditCard":
      return { tags: [], makeOptional: false, known: true };
    case "IsPhoneNumber":
      return { tags: [], makeOptional: false, known: true };
    case "IsHexColor":
      return { tags: [], makeOptional: false, known: true };
    case "IsMACAddress":
      return { tags: [], makeOptional: false, known: true };
    case "IsPort":
      return { tags: [], makeOptional: false, known: true };
    case "IsMimeType":
      return { tags: [], makeOptional: false, known: true };
    case "IsSemVer":
      return { tags: [], makeOptional: false, known: true };
    case "IsAlpha":
      return { tags: [], makeOptional: false, known: true };
    case "IsAlphanumeric":
      return { tags: [], makeOptional: false, known: true };
    case "IsNumberString":
      return { tags: [], makeOptional: false, known: true };
    case "IsBase64":
      return { tags: [], makeOptional: false, known: true };
    case "IsMongoId":
      return { tags: [], makeOptional: false, known: true };
    case "IsCurrency":
      return { tags: [], makeOptional: false, known: true };
    case "Contains":
      return { tags: [], makeOptional: false, known: true };
    case "NotContains":
      return { tags: [], makeOptional: false, known: true };
    case "IsHash":
      return { tags: [], makeOptional: false, known: true };
    case "IsLatitude":
      return { tags: [], makeOptional: false, known: true };
    case "IsLongitude":
      return { tags: [], makeOptional: false, known: true };
    case "IsLatLong":
      return { tags: [], makeOptional: false, known: true };
    case "Length": {
      // @Length(min, max) → both tags.MinLength<min> and tags.MaxLength<max>
      const result: string[] = [];
      if (args[0]) result.push(`tags.MinLength<${args[0]}>`);
      if (args[1]) result.push(`tags.MaxLength<${args[1]}>`);
      return { tags: result, makeOptional: false, known: true };
    }

    // Optional marker
    case "IsOptional":
      return { tags: [], makeOptional: true, known: true };

    // Validation group / conditional — just remove
    case "Allow":
      return { tags: [], makeOptional: false, known: true };

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
      return { tags: [], makeOptional: false, known: true };

    default:
      return { tags: [], makeOptional: false, known: false };
  }
}

/**
 * Transform class-validator DTOs to interfaces with branded types.
 * Processes any file that imports from class-validator. Skips NestJS framework
 * classes and entity/ORM classes via decorator detection.
 */
export function transformClassValidator(file: SourceFile, report: MigrateReport): number {
  const cvImport = file.getImportDeclaration(
    (d) => d.getModuleSpecifierValue() === "class-validator",
  );
  if (!cvImport) return 0;

  let count = 0;
  const filePath = file.getFilePath();
  let needsTagsImport = false;

  // Track unknown decorators for TODOs
  const unknownDecorators = new Set<string>();

  // NestJS mapped-type helper functions — when a class extends one of these,
  // it cannot be converted to an interface (interfaces can't extend function calls).
  const MAPPED_TYPE_HELPERS = new Set([
    "OmitType", "PickType", "PartialType", "IntersectionType",
  ]);

  // Process each class in the file
  const classes = file.getClasses();
  for (const cls of classes) {
    // Skip NestJS framework classes and entity/ORM classes (not DTOs)
    const classDecorators = cls.getDecorators();
    const isNestOrEntityClass = classDecorators.some((d) =>
      [
        // NestJS framework classes
        "Controller", "Module", "Injectable", "Guard", "Interceptor", "Pipe", "Filter",
        // ORM entity decorators — these classes have their own lifecycle
        "Entity", "Schema", "Table", "Document", "Model",
        "ViewEntity", "ChildEntity", "Embeddable",
      ].includes(d.getName()),
    );
    if (isNestOrEntityClass) continue;

    // Check if the class extends a mapped type helper (OmitType, PickType, etc.)
    // These can't become interfaces — keep as class, just remove decorators in-place.
    const extendsExpr = cls.getExtends();
    const extendsMappedType = extendsExpr && MAPPED_TYPE_HELPERS.has(
      extendsExpr.getExpression().getText().split("(")[0].trim(),
    );

    if (extendsMappedType) {
      // In-place decorator removal: strip class-validator decorators and update types
      for (const prop of cls.getProperties()) {
        const brandedTypes: string[] = [];
        let isOptional = prop.hasQuestionToken();
        const decoratorsToRemove: Decorator[] = [];

        for (const dec of prop.getDecorators()) {
          const decName = dec.getName();
          if (decName.startsWith("Api")) continue;
          if (NON_CLASS_VALIDATOR_DECORATORS.has(decName)) continue;

          const result = decoratorToBrandedType(dec);
          if (result.tags.length > 0) {
            brandedTypes.push(...result.tags);
            needsTagsImport = true;
          }
          if (result.makeOptional) isOptional = true;
          if (!result.known) {
            unknownDecorators.add(decName);
            continue; // Don't remove unknown decorators — they may be from other libraries
          }
          decoratorsToRemove.push(dec);
        }

        // Remove decorators in reverse to preserve positions
        for (const dec of decoratorsToRemove.reverse()) {
          dec.remove();
          count++;
        }

        // Update type with branded types
        if (brandedTypes.length > 0) {
          const currentType = prop.getTypeNode()?.getText() ?? "any";
          prop.setType(`${currentType} & ${brandedTypes.join(" & ")}`);
        }
        if (isOptional && !prop.hasQuestionToken()) {
          prop.setHasQuestionToken(true);
        }
      }

      const className = cls.getName();
      if (className) {
        report.info(filePath, "class-validator",
          `Stripped decorators from class ${className} (kept as class — extends mapped type)`,
        );
        count++;
      }
      continue;
    }

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
        if (result.tags.length > 0) {
          brandedTypes.push(...result.tags);
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
