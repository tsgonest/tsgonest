/**
 * Unit tests for tsgonest migrate transforms.
 *
 * Each test creates an in-memory ts-morph SourceFile, runs a transform,
 * and asserts the resulting source text.
 */

import { describe, it, expect, beforeEach } from "vitest";
import { Project, SourceFile } from "ts-morph";
import { MigrateReport } from "../report.js";
import { transformClassValidator } from "../transforms/class-validator.js";
import { transformSwagger } from "../transforms/swagger.js";
import { transformNestia } from "../transforms/nestia.js";
import { transformTypiaTags } from "../transforms/typia-tags.js";
import { transformClassTransformer } from "../transforms/class-transformer.js";
import { cleanupImports } from "../transforms/imports.js";
import { rewriteBaseUrlImports } from "../transforms/baseurl-imports.js";

let project: Project;
let report: MigrateReport;

function createFile(code: string, filePath = "/test/src/test.dto.ts"): SourceFile {
  return project.createSourceFile(filePath, code, { overwrite: true });
}

/** Normalize whitespace for comparison (trim lines, collapse blank lines). */
function normalize(s: string): string {
  return s
    .split("\n")
    .map((l) => l.trimEnd())
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

beforeEach(() => {
  project = new Project({ useInMemoryFileSystem: true });
  report = new MigrateReport();
});

// ──────────────────────────────────────────────────────────────────────
// class-validator transforms
// ──────────────────────────────────────────────────────────────────────

describe("transformClassValidator", () => {
  it("converts a basic DTO class to interface with branded types", () => {
    const file = createFile(`
import { IsEmail, IsNotEmpty, MinLength } from 'class-validator';

export class CreateUserDto {
  @IsEmail()
  @IsNotEmpty()
  email: string;

  @MinLength(6)
  password: string;

  @IsNotEmpty()
  name: string;
}
`);
    const count = transformClassValidator(file, report);
    expect(count).toBeGreaterThan(0);

    const text = normalize(file.getFullText());
    // Should be an interface now
    expect(text).toContain("export interface CreateUserDto");
    expect(text).not.toContain("export class CreateUserDto");
    // Branded types
    expect(text).toContain("tags.Email");
    expect(text).toContain("tags.MinLength<6>");
    expect(text).toContain("tags.MinLength<1>");
    // Import should be @tsgonest/types
    expect(text).toContain('@tsgonest/types');
    // class-validator import should be gone
    expect(text).not.toContain("class-validator");
  });

  it("handles @IsOptional correctly", () => {
    const file = createFile(`
import { IsOptional, IsString } from 'class-validator';

export class UpdateDto {
  @IsOptional()
  @IsString()
  name: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("name?:");
  });

  it("maps @IsUUID to tags.Uuid", () => {
    const file = createFile(`
import { IsUUID } from 'class-validator';

export class IdDto {
  @IsUUID()
  id: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("tags.Uuid");
  });

  it("maps @IsURL to tags.Url", () => {
    const file = createFile(`
import { IsURL } from 'class-validator';

export class LinkDto {
  @IsURL()
  url: string;
}
`);
    transformClassValidator(file, report);
    expect(normalize(file.getFullText())).toContain("tags.Url");
  });

  it("maps @ArrayNotEmpty to tags.MinItems<1>", () => {
    const file = createFile(`
import { ArrayNotEmpty } from 'class-validator';

export class ListDto {
  @ArrayNotEmpty()
  items: string[];
}
`);
    transformClassValidator(file, report);
    expect(normalize(file.getFullText())).toContain("tags.MinItems<1>");
  });

  it("maps @IsDate to tags.Format<\"date-time\">", () => {
    const file = createFile(`
import { IsDate } from 'class-validator';

export class DateDto {
  @IsDate()
  createdAt: Date;
}
`);
    transformClassValidator(file, report);
    expect(normalize(file.getFullText())).toContain('tags.Format<"date-time">');
  });

  it("maps @IsPositive and @IsInt correctly", () => {
    const file = createFile(`
import { IsPositive, IsInt } from 'class-validator';

export class CountDto {
  @IsPositive()
  @IsInt()
  count: number;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("tags.Positive");
    expect(text).toContain("tags.Int");
  });

  it("removes type-only validators without adding branded types", () => {
    const file = createFile(`
import { IsString, IsNumber, IsBoolean, IsArray } from 'class-validator';

export class TypesDto {
  @IsString()
  name: string;

  @IsNumber()
  age: number;

  @IsBoolean()
  active: boolean;

  @IsArray()
  tags: string[];
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    // Should not have any branded types — type validators are redundant with TS
    expect(text).not.toContain("tags.");
    // Should not have @tsgonest/types import
    expect(text).not.toContain("@tsgonest/types");
    // Should still be an interface
    expect(text).toContain("export interface TypesDto");
  });

  it("skips @Injectable classes", () => {
    const file = createFile(`
import { IsString } from 'class-validator';

@Injectable()
export class SomeService {
  @IsString()
  name: string;
}
`);
    transformClassValidator(file, report);
    // Should remain a class — @Injectable means it's a NestJS provider, not a DTO
    const text = normalize(file.getFullText());
    expect(text).toContain("class SomeService");
  });

  it("processes .request.ts and .model.ts files (no filename filter)", () => {
    const requestFile = createFile(
      `
import { IsEmail } from 'class-validator';

export class CreateRequest {
  @IsEmail()
  email: string;
}
`,
      "/test/src/create.request.ts",
    );
    const count1 = transformClassValidator(requestFile, report);
    expect(count1).toBeGreaterThan(0);
    expect(normalize(requestFile.getFullText())).toContain("export interface CreateRequest");

    const modelFile = createFile(
      `
import { IsNotEmpty } from 'class-validator';

export class UserModel {
  @IsNotEmpty()
  name: string;
}
`,
      "/test/src/user.model.ts",
    );
    report = new MigrateReport();
    const count2 = transformClassValidator(modelFile, report);
    expect(count2).toBeGreaterThan(0);
    expect(normalize(modelFile.getFullText())).toContain("export interface UserModel");
  });

  it("skips NestJS framework classes (Controller, Injectable, etc.)", () => {
    const file = createFile(`
import { IsString } from 'class-validator';

@Controller()
export class UsersController {
  @IsString()
  name: string;
}
`);
    transformClassValidator(file, report);
    // Should remain a class
    const text = normalize(file.getFullText());
    expect(text).toContain("class UsersController");
  });

  it("preserves extends clause", () => {
    const file = createFile(`
import { IsString } from 'class-validator';

export class ExtendedDto extends BaseDto {
  @IsString()
  name: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("interface ExtendedDto extends BaseDto");
  });

  it("skips GraphQL decorators (@Field, @HideField)", () => {
    const file = createFile(`
import { IsNotEmpty } from 'class-validator';

export class GqlDto {
  @Field()
  @IsNotEmpty()
  name: string;
}
`);
    transformClassValidator(file, report);
    // Should not flag @Field as unknown
    const todos = report.todos;
    const hasFieldTodo = todos.some((t) => t.message.includes("@Field"));
    expect(hasFieldTodo).toBe(false);
  });

  it("skips TypeORM decorators (@Column, @ManyToOne)", () => {
    const file = createFile(`
import { IsNotEmpty } from 'class-validator';

export class EntityDto {
  @Column()
  @IsNotEmpty()
  name: string;

  @ManyToOne()
  parent: any;
}
`);
    transformClassValidator(file, report);
    const todos = report.todos;
    const hasOrmTodo = todos.some((t) => t.message.includes("@Column") || t.message.includes("@ManyToOne"));
    expect(hasOrmTodo).toBe(false);
  });

  it("reports truly unknown custom decorators as TODOs", () => {
    const file = createFile(`
import { IsNotEmpty } from 'class-validator';

export class CustomDto {
  @IsNotEmpty()
  @MyCustomValidator()
  name: string;
}
`);
    transformClassValidator(file, report);
    const todos = report.todos;
    const hasCustomTodo = todos.some((t) => t.message.includes("@MyCustomValidator"));
    expect(hasCustomTodo).toBe(true);
  });

  it("handles @Allow (removes silently)", () => {
    const file = createFile(`
import { Allow } from 'class-validator';

export class TokenDto {
  @Allow()
  token: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("export interface TokenDto");
    expect(text).not.toContain("@Allow");
    // @Allow is known, no TODO
    const todos = report.todos;
    expect(todos.some((t) => t.message.includes("@Allow"))).toBe(false);
  });

  it("combines multiple validators on one property", () => {
    const file = createFile(`
import { IsEmail, MinLength, MaxLength } from 'class-validator';

export class ContactDto {
  @IsEmail()
  @MinLength(5)
  @MaxLength(100)
  email: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("tags.Email");
    expect(text).toContain("tags.MinLength<5>");
    expect(text).toContain("tags.MaxLength<100>");
  });

  it("maps @Length(min, max) to both tags.MinLength + tags.MaxLength", () => {
    const file = createFile(`
import { Length } from 'class-validator';

export class CodeDto {
  @Length(2, 10)
  code: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("tags.MinLength<2>");
    expect(text).toContain("tags.MaxLength<10>");
  });

  it("maps @Length(min) with only min arg", () => {
    const file = createFile(`
import { Length } from 'class-validator';

export class ShortDto {
  @Length(3)
  code: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("tags.MinLength<3>");
    expect(text).not.toContain("MaxLength");
  });

  it("skips @Entity classes (ORM entities)", () => {
    const file = createFile(`
import { IsNotEmpty } from 'class-validator';

@Entity()
export class UserEntity {
  @IsNotEmpty()
  name: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    // Should remain a class — @Entity means it's an ORM entity
    expect(text).toContain("class UserEntity");
    expect(text).not.toContain("interface UserEntity");
  });

  it("skips @Schema classes (Mongoose schemas)", () => {
    const file = createFile(`
import { IsEmail } from 'class-validator';

@Schema()
export class UserSchema {
  @IsEmail()
  email: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("class UserSchema");
    expect(text).not.toContain("interface UserSchema");
  });

  it("keeps class when extending OmitType/PickType (mapped types)", () => {
    const file = createFile(`
import { IsEmail, MinLength } from 'class-validator';
import { OmitType } from '@nestjs/swagger';

class BaseEntity {
  id: string;
  email: string;
  password: string;
}

export class CreateDto extends OmitType(BaseEntity, ['id']) {
  @IsEmail()
  email: string;

  @MinLength(6)
  password: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    // Should remain a class (not interface) because extends OmitType(...)
    expect(text).toContain("export class CreateDto extends OmitType");
    expect(text).not.toContain("export interface CreateDto");
    // But decorators should be removed and branded types added
    expect(text).toContain("tags.Email");
    expect(text).toContain("tags.MinLength<6>");
    expect(text).not.toContain("@IsEmail");
    expect(text).not.toContain("@MinLength");
  });

  it("keeps class when extending PickType (mapped types)", () => {
    const file = createFile(`
import { IsNotEmpty } from 'class-validator';
import { PickType } from '@nestjs/swagger';

class UserEntity {
  name: string;
  email: string;
}

export class UpdateDto extends PickType(UserEntity, ['name']) {
  @IsNotEmpty()
  name: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("export class UpdateDto extends PickType");
    expect(text).toContain("tags.MinLength<1>");
    expect(text).not.toContain("@IsNotEmpty");
  });

  it("does not remove unknown decorators in mapped-type class path", () => {
    const file = createFile(`
import { IsString } from 'class-validator';
import { OmitType } from '@nestjs/swagger';

class Base { id: string; }

export class Dto extends OmitType(Base, ['id']) {
  @Prop({ required: true })
  @IsString()
  name: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    // @Prop should remain (it's from typegoose, not class-validator)
    expect(text).toContain("@Prop");
    // @IsString should be removed (it's class-validator)
    expect(text).not.toContain("@IsString");
  });

  it("handles multiple classes in one DTO file", () => {
    const file = createFile(`
import { IsEmail, IsString } from 'class-validator';

export class CreateUserDto {
  @IsEmail()
  email: string;
}

export class UpdateUserDto {
  @IsString()
  name: string;
}
`);
    transformClassValidator(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("export interface CreateUserDto");
    expect(text).toContain("export interface UpdateUserDto");
    expect(text).not.toContain("export class");
  });
});

// ──────────────────────────────────────────────────────────────────────
// swagger transforms
// ──────────────────────────────────────────────────────────────────────

describe("transformSwagger", () => {
  it("removes @ApiTags, @ApiProperty, @ApiOkResponse", () => {
    const file = createFile(
      `
import { ApiTags, ApiProperty, ApiOkResponse } from '@nestjs/swagger';
import { Controller, Get } from '@nestjs/common';

@ApiTags('Users')
@Controller('users')
export class UsersController {
  @ApiOkResponse({ type: String })
  @Get()
  findAll() {}
}
`,
      "/test/src/users.controller.ts",
    );
    const count = transformSwagger(file, report);
    expect(count).toBeGreaterThan(0);

    const text = normalize(file.getFullText());
    expect(text).not.toContain("@ApiTags");
    expect(text).not.toContain("@ApiOkResponse");
    expect(text).not.toContain("@nestjs/swagger");
    // NestJS common imports should remain
    expect(text).toContain("@nestjs/common");
  });

  it("collects @ApiBearerAuth into detectedSecuritySchemes", () => {
    const file = createFile(
      `
import { ApiBearerAuth } from '@nestjs/swagger';
import { Controller } from '@nestjs/common';

@ApiBearerAuth()
@Controller('auth')
export class AuthController {}
`,
      "/test/src/auth.controller.ts",
    );
    transformSwagger(file, report);

    // Security scheme should be auto-detected, not a TODO
    expect(report.detectedSecuritySchemes.has("bearer")).toBe(true);
    expect(report.detectedSecuritySchemes.get("bearer")).toEqual({
      type: "http",
      scheme: "bearer",
    });
    expect(normalize(file.getFullText())).not.toContain("@ApiBearerAuth");
  });

  it("adds TODO for SwaggerModule in main.ts", () => {
    const file = createFile(
      `
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';

const config = new DocumentBuilder().setTitle('API').build();
const doc = SwaggerModule.createDocument(app, config);
SwaggerModule.setup('api', app, doc);
`,
      "/test/src/main.ts",
    );
    transformSwagger(file, report);

    const todos = report.todos;
    expect(todos.some((t) => t.message.includes("SwaggerModule"))).toBe(true);
  });

  it("removes only unused named imports from @nestjs/swagger", () => {
    const file = createFile(
      `
import { ApiProperty, OmitType } from '@nestjs/swagger';

export class MyDto extends OmitType(BaseEntity, ['id']) {
  @ApiProperty()
  name: string;
}
`,
      "/test/src/my.dto.ts",
    );
    transformSwagger(file, report);
    const text = normalize(file.getFullText());
    // ApiProperty decorator and import should be removed
    expect(text).not.toContain("@ApiProperty");
    expect(text).not.toContain("ApiProperty");
    // OmitType should remain (still referenced in extends clause)
    // Since only mapped-type helpers remain, import source is rewritten
    expect(text).toContain("OmitType");
    expect(text).toContain("@nestjs/mapped-types");
  });

  it("removes entire @nestjs/swagger import when all named imports are unused", () => {
    const file = createFile(
      `
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { Controller, Get } from '@nestjs/common';

@ApiTags('Test')
@Controller('test')
export class TestController {
  @ApiOperation({ summary: 'test' })
  @Get()
  test() {}
}
`,
      "/test/src/test.controller.ts",
    );
    transformSwagger(file, report);
    const text = normalize(file.getFullText());
    expect(text).not.toContain("@nestjs/swagger");
  });

  it("returns 0 when no @nestjs/swagger import exists", () => {
    const file = createFile(
      `
import { Controller } from '@nestjs/common';

@Controller('test')
export class TestController {}
`,
      "/test/src/test.controller.ts",
    );
    expect(transformSwagger(file, report)).toBe(0);
  });

  it("rewrites @nestjs/swagger → @nestjs/mapped-types when only mapped-type helpers remain", () => {
    const file = createFile(
      `
import { ApiProperty, OmitType, PickType } from '@nestjs/swagger';

export class CreateDto extends OmitType(BaseEntity, ['id']) {
  @ApiProperty()
  name: string;
}

export class UpdateDto extends PickType(BaseEntity, ['name']) {
  name: string;
}
`,
      "/test/src/combined.dto.ts",
    );
    transformSwagger(file, report);
    const text = normalize(file.getFullText());
    // ApiProperty should be removed
    expect(text).not.toContain("@ApiProperty");
    expect(text).not.toContain("ApiProperty");
    // OmitType/PickType should remain, but import source should be @nestjs/mapped-types
    expect(text).toContain("OmitType");
    expect(text).toContain("PickType");
    expect(text).toContain("@nestjs/mapped-types");
    expect(text).not.toContain("@nestjs/swagger");
  });

  it("keeps @nestjs/swagger when non-mapped-type helpers remain", () => {
    const file = createFile(
      `
import { ApiProperty, OmitType, SwaggerModule } from '@nestjs/swagger';

const doc = SwaggerModule.createDocument(app, config);

export class MyDto extends OmitType(BaseEntity, ['id']) {
  @ApiProperty()
  name: string;
}
`,
      "/test/src/main-dto.ts",
    );
    transformSwagger(file, report);
    const text = normalize(file.getFullText());
    // SwaggerModule is still referenced → import stays as @nestjs/swagger
    expect(text).toContain("@nestjs/swagger");
    expect(text).not.toContain("@nestjs/mapped-types");
  });
});

// ──────────────────────────────────────────────────────────────────────
// nestia transforms
// ──────────────────────────────────────────────────────────────────────

describe("transformNestia", () => {
  it("converts @TypedRoute.Get → @Get and updates imports", () => {
    const file = createFile(
      `
import { TypedRoute, TypedBody } from '@nestia/core';
import { Controller } from '@nestjs/common';

@Controller('users')
export class UsersController {
  @TypedRoute.Get()
  findAll() {}

  @TypedRoute.Post()
  create(@TypedBody() body: any) {}
}
`,
      "/test/src/users.controller.ts",
    );
    const count = transformNestia(file, report);
    expect(count).toBeGreaterThan(0);

    const text = normalize(file.getFullText());
    expect(text).toContain("@Get()");
    expect(text).toContain("@Post()");
    expect(text).toContain("@Body()");
    expect(text).not.toContain("TypedRoute");
    expect(text).not.toContain("TypedBody");
    expect(text).not.toContain("@nestia/core");
    // Get, Post, Body should be added to @nestjs/common import
    expect(text).toContain("@nestjs/common");
  });

  it("converts @TypedParam<T>('name') → @Param('name')", () => {
    const file = createFile(
      `
import { TypedParam } from '@nestia/core';
import { Controller, Get } from '@nestjs/common';

@Controller('users')
export class UsersController {
  @Get(':id')
  findOne(@TypedParam<string>('id') id: string) {}
}
`,
      "/test/src/users.controller.ts",
    );
    transformNestia(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("@Param('id')");
    expect(text).not.toContain("TypedParam");
  });

  it("converts @TypedQuery → @Query", () => {
    const file = createFile(
      `
import { TypedQuery } from '@nestia/core';
import { Controller, Get } from '@nestjs/common';

@Controller('search')
export class SearchController {
  @Get()
  search(@TypedQuery() query: any) {}
}
`,
      "/test/src/search.controller.ts",
    );
    transformNestia(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("@Query()");
    expect(text).not.toContain("TypedQuery");
  });

  it("returns 0 when no @nestia/core import exists", () => {
    const file = createFile(
      `
import { Controller } from '@nestjs/common';

@Controller()
export class PlainController {}
`,
      "/test/src/plain.controller.ts",
    );
    expect(transformNestia(file, report)).toBe(0);
  });

  it("merges into existing @nestjs/common import without duplicates", () => {
    const file = createFile(
      `
import { TypedRoute } from '@nestia/core';
import { Controller, Get } from '@nestjs/common';

@Controller('test')
export class TestController {
  @TypedRoute.Get()
  test() {}
}
`,
      "/test/src/test.controller.ts",
    );
    transformNestia(file, report);
    const text = normalize(file.getFullText());
    // Should not have duplicate Get import
    const matches = text.match(/Get/g);
    // @Get() decorator + import named Get = at least 2, but no duplicated import specifier
    expect(text).toContain("@nestjs/common");
  });

  it("converts @TypedFormData.Body() → @FormDataBody() with @UseInterceptors", () => {
    const file = createFile(
      `
import { TypedFormData } from '@nestia/core';
import { Controller, Post } from '@nestjs/common';

@Controller('upload')
export class UploadController {
  @Post()
  upload(@TypedFormData.Body() body: any) {}
}
`,
      "/test/src/upload.controller.ts",
    );
    transformNestia(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("@FormDataBody()");
    expect(text).not.toContain("@Body()");
    expect(text).not.toContain("TypedFormData");
    // Should add @UseInterceptors(FormDataInterceptor) to the method
    expect(text).toContain("@UseInterceptors(FormDataInterceptor)");
    // Should import from @tsgonest/runtime
    expect(text).toContain("@tsgonest/runtime");
    expect(text).toContain("FormDataBody");
    expect(text).toContain("FormDataInterceptor");
    // UseInterceptors should be added to @nestjs/common imports
    expect(text).toContain("UseInterceptors");
    // Should NOT generate a TODO for form-data
    expect(report.todos.some((t) => t.message.includes("Multipart"))).toBe(false);
  });

  it("preserves multer factory arg in @TypedFormData.Body()", () => {
    const file = createFile(
      `
import { TypedFormData } from '@nestia/core';
import { Controller, Post } from '@nestjs/common';

@Controller('upload')
export class UploadController {
  @Post()
  upload(@TypedFormData.Body(() => imageMulter()) body: any) {}
}
`,
      "/test/src/upload.controller.ts",
    );
    transformNestia(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("@FormDataBody(() => imageMulter())");
    expect(text).toContain("@UseInterceptors(FormDataInterceptor)");
  });

  it("removes @SwaggerCustomizer with TODO", () => {
    const file = createFile(
      `
import { SwaggerCustomizer, TypedRoute } from '@nestia/core';
import { Controller } from '@nestjs/common';

@Controller('docs')
export class DocsController {
  @SwaggerCustomizer((schema) => { schema.summary = 'custom'; })
  @TypedRoute.Get()
  getDocs() {}
}
`,
      "/test/src/docs.controller.ts",
    );
    transformNestia(file, report);
    const text = normalize(file.getFullText());
    expect(text).not.toContain("SwaggerCustomizer");
    expect(report.todos.some((t) => t.message.includes("SwaggerCustomizer"))).toBe(true);
  });
});

// ──────────────────────────────────────────────────────────────────────
// typia-tags transforms
// ──────────────────────────────────────────────────────────────────────

describe("transformTypiaTags", () => {
  it("rewrites import source from typia to @tsgonest/types", () => {
    const file = createFile(
      `
import { tags } from 'typia';

export type UserDto = {
  email: string & tags.Format<'email'>;
  name: string & tags.MinLength<1>;
};
`,
      "/test/src/user.dto.ts",
    );
    const count = transformTypiaTags(file, report);
    expect(count).toBeGreaterThan(0);

    const text = normalize(file.getFullText());
    expect(text).toContain("@tsgonest/types");
    expect(text).not.toContain("from 'typia'");
    // Type annotations unchanged
    expect(text).toContain("tags.Format<'email'>");
    expect(text).toContain("tags.MinLength<1>");
  });

  it("removes typia.assert<T>() calls, keeping the argument", () => {
    const file = createFile(
      `
import typia from 'typia';

function validate(input: unknown) {
  return typia.assert<User>(input);
}
`,
      "/test/src/validate.ts",
    );
    transformTypiaTags(file, report);
    const text = normalize(file.getFullText());
    expect(text).not.toContain("typia.assert");
    expect(text).toContain("input");
  });

  it("replaces typia.json.stringify<T>() with JSON.stringify()", () => {
    const file = createFile(
      `
import typia from 'typia';

function serialize(data: User) {
  return typia.json.stringify<User>(data);
}
`,
      "/test/src/serialize.ts",
    );
    transformTypiaTags(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("JSON.stringify(data)");
    expect(text).not.toContain("typia.json.stringify");
  });

  it("replaces typia.json.assertParse<T>() with JSON.parse()", () => {
    const file = createFile(
      `
import typia from 'typia';

function parse(input: string) {
  return typia.json.assertParse<User>(input);
}
`,
      "/test/src/parse.ts",
    );
    transformTypiaTags(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("JSON.parse(input)");
    expect(text).not.toContain("typia.json.assertParse");
    // Should add a TODO
    expect(report.todos.some((t) => t.message.includes("assertParse"))).toBe(true);
  });

  it("adds TODO for tags.TagBase usage", () => {
    const file = createFile(
      `
import { tags } from 'typia';

type MongoId = string & tags.TagBase<{ kind: 'mongoId'; target: 'string'; value: undefined; validate: string }>;
`,
      "/test/src/types.dto.ts",
    );
    transformTypiaTags(file, report);
    const todos = report.todos;
    expect(todos.some((t) => t.message.includes("TagBase"))).toBe(true);
  });

  it("returns 0 when no typia import exists", () => {
    const file = createFile(
      `
export type PlainDto = { name: string };
`,
      "/test/src/plain.dto.ts",
    );
    expect(transformTypiaTags(file, report)).toBe(0);
  });
});

// ──────────────────────────────────────────────────────────────────────
// class-transformer transforms
// ──────────────────────────────────────────────────────────────────────

describe("transformClassTransformer", () => {
  it("converts @Type(() => Number) to tags.Coerce", () => {
    const file = createFile(
      `
import { Type } from 'class-transformer';

export class QueryDto {
  @Type(() => Number)
  page: number;
}
`,
      "/test/src/query.dto.ts",
    );
    const count = transformClassTransformer(file, report);
    expect(count).toBeGreaterThan(0);

    const text = normalize(file.getFullText());
    expect(text).toContain("tags.Coerce");
    expect(text).not.toContain("@Type");
    expect(text).not.toContain("class-transformer");
  });

  it("converts @Transform with trim to tags.Trim", () => {
    const file = createFile(
      `
import { Transform } from 'class-transformer';

export class NameDto {
  @Transform(({ value }) => value?.trim())
  name: string;
}
`,
      "/test/src/name.dto.ts",
    );
    transformClassTransformer(file, report);
    const text = normalize(file.getFullText());
    expect(text).toContain("tags.Trim");
  });

  it("converts @Transform with toLowerCase to tags.ToLowerCase", () => {
    const file = createFile(
      `
import { Transform } from 'class-transformer';

export class EmailDto {
  @Transform(({ value }) => value?.toLowerCase())
  email: string;
}
`,
      "/test/src/email.dto.ts",
    );
    transformClassTransformer(file, report);
    expect(normalize(file.getFullText())).toContain("tags.ToLowerCase");
  });

  it("adds TODO for @Exclude and keeps the property", () => {
    const file = createFile(
      `
import { Exclude } from 'class-transformer';

export class UserEntity {
  @Exclude()
  password: string;
}
`,
      "/test/src/user.entity.ts",
    );
    transformClassTransformer(file, report);

    const text = normalize(file.getFullText());
    // Property should still exist
    expect(text).toContain("password: string");
    // @Exclude decorator should be removed
    expect(text).not.toContain("@Exclude");
    // TODO should be added
    const todos = report.todos;
    expect(todos.some((t) => t.message.includes("@Exclude"))).toBe(true);
  });

  it("removes @Expose silently", () => {
    const file = createFile(
      `
import { Expose } from 'class-transformer';

export class ProfileDto {
  @Expose()
  name: string;
}
`,
      "/test/src/profile.dto.ts",
    );
    transformClassTransformer(file, report);
    const text = normalize(file.getFullText());
    expect(text).not.toContain("@Expose");
    expect(text).not.toContain("class-transformer");
  });

  it("returns 0 when no class-transformer import exists", () => {
    const file = createFile(
      `
export class PlainDto { name: string; }
`,
      "/test/src/plain.dto.ts",
    );
    expect(transformClassTransformer(file, report)).toBe(0);
  });

  it("adds TODO for unrecognized @Transform patterns", () => {
    const file = createFile(
      `
import { Transform } from 'class-transformer';

export class WeirdDto {
  @Transform(({ value }) => value * 2)
  score: number;
}
`,
      "/test/src/weird.dto.ts",
    );
    transformClassTransformer(file, report);
    const todos = report.todos;
    expect(todos.some((t) => t.message.includes("@Transform"))).toBe(true);
  });

  it("converts @Transform with toUpperCase to tags.ToUpperCase", () => {
    const file = createFile(
      `
import { Transform } from 'class-transformer';

export class CodeDto {
  @Transform(({ value }) => value?.toUpperCase())
  code: string;
}
`,
      "/test/src/code.dto.ts",
    );
    transformClassTransformer(file, report);
    expect(normalize(file.getFullText())).toContain("tags.ToUpperCase");
  });
});

// ──────────────────────────────────────────────────────────────────────
// baseUrl import rewriting
// ──────────────────────────────────────────────────────────────────────

describe("rewriteBaseUrlImports", () => {
  it("rewrites src/-prefixed imports to relative paths", () => {
    const file = createFile(
      `
import { UserDto } from 'src/users/dto/user.dto';
import { AuthService } from 'src/auth/auth.service';
import { Controller } from '@nestjs/common';

@Controller('test')
export class TestController {}
`,
      "/project/src/test/test.controller.ts",
    );
    // Mock: pretend these files exist
    const existingFiles = new Set([
      "/project/src/users/dto/user.dto.ts",
      "/project/src/auth/auth.service.ts",
    ]);
    const count = rewriteBaseUrlImports(file, "/project", report, (p) => existingFiles.has(p));
    expect(count).toBe(2);

    const text = normalize(file.getFullText());
    expect(text).toContain("from '../users/dto/user.dto'");
    expect(text).toContain("from '../auth/auth.service'");
    expect(text).not.toContain("from 'src/");
    // @nestjs/common should be untouched
    expect(text).toContain("from '@nestjs/common'");
  });

  it("skips relative imports", () => {
    const file = createFile(
      `
import { Foo } from './foo';
import { Bar } from '../bar';
`,
      "/project/src/test.ts",
    );
    const count = rewriteBaseUrlImports(file, "/project", report, () => false);
    expect(count).toBe(0);
  });

  it("skips scoped packages", () => {
    const file = createFile(
      `
import { Controller } from '@nestjs/common';
import { tags } from '@tsgonest/types';
`,
      "/project/src/test.ts",
    );
    const count = rewriteBaseUrlImports(file, "/project", report, () => false);
    expect(count).toBe(0);
  });

  it("skips non-project imports (unresolvable)", () => {
    const file = createFile(
      `
import { Injectable } from '@nestjs/common';
import * as path from 'path';
import express from 'express';
`,
      "/project/src/test.ts",
    );
    // No files exist → nothing resolves
    const count = rewriteBaseUrlImports(file, "/project", report, () => false);
    expect(count).toBe(0);
  });

  it("handles same-directory rewrites with ./ prefix", () => {
    const file = createFile(
      `
import { Helper } from 'src/utils/helper';
`,
      "/project/src/utils/main.ts",
    );
    const existingFiles = new Set(["/project/src/utils/helper.ts"]);
    const count = rewriteBaseUrlImports(file, "/project", report, (p) => existingFiles.has(p));
    expect(count).toBe(1);
    expect(normalize(file.getFullText())).toContain("from './helper'");
  });

  it("resolves index files", () => {
    const file = createFile(
      `
import { Utils } from 'src/utils';
`,
      "/project/src/test.ts",
    );
    const existingFiles = new Set(["/project/src/utils/index.ts"]);
    const count = rewriteBaseUrlImports(file, "/project", report, (p) => existingFiles.has(p));
    expect(count).toBe(1);
    expect(normalize(file.getFullText())).toContain("from './utils'");
  });
});

// ──────────────────────────────────────────────────────────────────────
// import cleanup
// ──────────────────────────────────────────────────────────────────────

describe("cleanupImports", () => {
  it("removes empty named imports (import {} from 'x')", () => {
    const file = createFile(
      `
import {} from 'some-removed-lib';
import { Controller } from '@nestjs/common';

@Controller()
export class TestController {}
`,
      "/test/src/test.controller.ts",
    );
    cleanupImports(file);
    const text = normalize(file.getFullText());
    expect(text).not.toContain("some-removed-lib");
    expect(text).toContain("@nestjs/common");
  });

  it("preserves side-effect imports", () => {
    const file = createFile(
      `
import 'reflect-metadata';
import { Controller } from '@nestjs/common';

@Controller()
export class TestController {}
`,
      "/test/src/test.controller.ts",
    );
    cleanupImports(file);
    const text = normalize(file.getFullText());
    expect(text).toContain("reflect-metadata");
  });

  it("prunes unused named imports from class-validator/class-transformer", () => {
    const file = createFile(
      `
import { Expose } from 'class-transformer';
import { Controller } from '@nestjs/common';

@Controller()
export class TestController {}
`,
      "/test/src/test.controller.ts",
    );
    cleanupImports(file);
    const text = normalize(file.getFullText());
    // Expose is not used anywhere → import should be removed entirely
    expect(text).not.toContain("class-transformer");
  });

  it("keeps class-transformer import if specifiers are still referenced", () => {
    const file = createFile(
      `
import { plainToInstance } from 'class-transformer';

const x = plainToInstance(Cls, data);
`,
      "/test/src/util.ts",
    );
    cleanupImports(file);
    const text = normalize(file.getFullText());
    expect(text).toContain("plainToInstance");
    expect(text).toContain("class-transformer");
  });

  it("preserves namespace imports", () => {
    const file = createFile(
      `
import * as path from 'path';

export const dir = path.resolve('.');
`,
      "/test/src/util.ts",
    );
    cleanupImports(file);
    expect(normalize(file.getFullText())).toContain("import * as path from 'path'");
  });
});
