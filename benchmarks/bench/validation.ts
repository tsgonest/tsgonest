import "reflect-metadata";
import { Bench } from "tinybench";
import {
  IsString,
  IsEmail,
  IsNumber,
  IsBoolean,
  IsIn,
  MinLength,
  MaxLength,
  Min,
  Max,
  IsUUID,
  IsArray,
  ValidateNested,
  IsOptional,
  Matches,
  validateSync,
} from "class-validator";
import { Type } from "class-transformer";

// ── tsgonest generated validators ────────────────────────────────────
// @ts-ignore — generated JS, no TS declarations in benchmark dist
import { validateCreateUserDto } from "../dist/simple.dto.CreateUserDto.tsgonest.js";
// @ts-ignore
import { validateCreateOrderDto } from "../dist/complex.dto.CreateOrderDto.tsgonest.js";

// ── class-validator DTOs (equivalent to tsgonest DTOs) ───────────────

class CVCreateUserDto {
  @IsString()
  @MinLength(1)
  @MaxLength(255)
  name!: string;

  @IsEmail()
  email!: string;

  @IsNumber()
  @Min(0)
  @Max(150)
  age!: number;

  @IsBoolean()
  isActive!: boolean;

  @IsIn(["admin", "user", "moderator"])
  role!: string;
}

class CVAddress {
  @IsString()
  @MinLength(1)
  street!: string;

  @IsString()
  @MinLength(1)
  city!: string;

  @IsOptional()
  @IsString()
  state?: string;

  @IsString()
  @MinLength(1)
  country!: string;

  @IsString()
  @Matches(/^[0-9]{5}(-[0-9]{4})?$/)
  zipCode!: string;
}

class CVOrderItem {
  @IsUUID()
  productId!: string;

  @IsNumber()
  @Min(1)
  @Max(999)
  quantity!: number;

  @IsNumber()
  @Min(0)
  unitPrice!: number;

  @IsString()
  @MinLength(1)
  name!: string;
}

class CVCreateOrderDto {
  @IsUUID()
  userId!: string;

  @IsArray()
  @ValidateNested({ each: true })
  @Type(() => CVOrderItem)
  items!: CVOrderItem[];

  @ValidateNested()
  @Type(() => CVAddress)
  shippingAddress!: CVAddress;

  @IsOptional()
  @ValidateNested()
  @Type(() => CVAddress)
  billingAddress?: CVAddress;

  @IsOptional()
  @IsString()
  @Matches(/^[A-Z0-9]{6,12}$/)
  couponCode?: string;

  @IsIn(["card", "bank", "crypto"])
  paymentMethod!: string;

  @IsNumber()
  @Min(0)
  totalAmount!: number;

  @IsOptional()
  @IsString()
  notes?: string;
}

// ── Test data ────────────────────────────────────────────────────────

const simpleValid = {
  name: "John Doe",
  email: "john@example.com",
  age: 30,
  isActive: true,
  role: "admin",
};

const simpleInvalid = {
  name: "",
  email: "not-an-email",
  age: -5,
  isActive: "yes",
  role: "superadmin",
};

const complexValid = {
  userId: "550e8400-e29b-41d4-a716-446655440000",
  items: [
    {
      productId: "550e8400-e29b-41d4-a716-446655440001",
      quantity: 2,
      unitPrice: 29.99,
      name: "Widget Pro",
    },
    {
      productId: "550e8400-e29b-41d4-a716-446655440002",
      quantity: 1,
      unitPrice: 49.99,
      name: "Gadget X",
    },
  ],
  shippingAddress: {
    street: "123 Main St",
    city: "Springfield",
    state: "IL",
    country: "US",
    zipCode: "62701",
  },
  couponCode: "SAVE20",
  paymentMethod: "card",
  totalAmount: 109.97,
  notes: "Please deliver before 5pm",
};

const complexInvalid = {
  userId: "not-a-uuid",
  items: [
    {
      productId: "also-not-uuid",
      quantity: 0,
      unitPrice: -10,
      name: "",
    },
  ],
  shippingAddress: {
    street: "",
    city: "",
    country: "",
    zipCode: "invalid",
  },
  paymentMethod: "bitcoin",
  totalAmount: -50,
};

// ── class-validator helper (creates class instances) ─────────────────

function toClassInstance<T>(cls: new () => T, plain: any): T {
  const instance = new cls();
  Object.assign(instance, plain);
  return instance;
}

function toOrderInstance(plain: any): CVCreateOrderDto {
  const instance = new CVCreateOrderDto();
  Object.assign(instance, plain);
  if (plain.items) {
    instance.items = plain.items.map((item: any) => {
      const i = new CVOrderItem();
      Object.assign(i, item);
      return i;
    });
  }
  if (plain.shippingAddress) {
    const addr = new CVAddress();
    Object.assign(addr, plain.shippingAddress);
    instance.shippingAddress = addr;
  }
  if (plain.billingAddress) {
    const addr = new CVAddress();
    Object.assign(addr, plain.billingAddress);
    instance.billingAddress = addr;
  }
  return instance;
}

// ── Manual validation (baseline: what a dev writes by hand) ──────────

function manualValidateSimple(input: any): { success: boolean; errors?: string[] } {
  const errors: string[] = [];
  if (typeof input !== "object" || input === null) return { success: false, errors: ["not an object"] };
  if (typeof input.name !== "string" || input.name.length < 1 || input.name.length > 255)
    errors.push("invalid name");
  if (typeof input.email !== "string" || !input.email.includes("@"))
    errors.push("invalid email");
  if (typeof input.age !== "number" || input.age < 0 || input.age > 150)
    errors.push("invalid age");
  if (typeof input.isActive !== "boolean") errors.push("invalid isActive");
  if (!["admin", "user", "moderator"].includes(input.role)) errors.push("invalid role");
  return errors.length > 0 ? { success: false, errors } : { success: true };
}

// ── Run benchmarks ───────────────────────────────────────────────────

async function main() {
  console.log("=== Validation Benchmark ===\n");
  console.log(`Node.js ${process.version}`);
  console.log(`Date: ${new Date().toISOString()}\n`);

  // Pre-create class-validator instances
  const cvSimpleValid = toClassInstance(CVCreateUserDto, simpleValid);
  const cvSimpleInvalid = toClassInstance(CVCreateUserDto, simpleInvalid);
  const cvComplexValid = toOrderInstance(complexValid);
  const cvComplexInvalid = toOrderInstance(complexInvalid);

  // Warmup: verify all validators produce correct results
  const tgSimpleResult = validateCreateUserDto(simpleValid);
  if (!tgSimpleResult.success) throw new Error("tsgonest simple valid should pass");
  const tgSimpleInvalidResult = validateCreateUserDto(simpleInvalid);
  if (tgSimpleInvalidResult.success) throw new Error("tsgonest simple invalid should fail");
  const cvSimpleErrors = validateSync(cvSimpleValid);
  if (cvSimpleErrors.length > 0) throw new Error("class-validator simple valid should pass");
  const manualResult = manualValidateSimple(simpleValid);
  if (!manualResult.success) throw new Error("manual simple valid should pass");

  // ── Simple DTO benchmarks ────────────────────────────────────────
  console.log("--- Simple DTO (5 fields, valid input) ---\n");

  const simpleBench = new Bench({ time: 1500, warmupTime: 300 });

  simpleBench
    .add("tsgonest validate (valid)", () => {
      validateCreateUserDto(simpleValid);
    })
    .add("class-validator validateSync (valid)", () => {
      validateSync(cvSimpleValid);
    })
    .add("manual validation (valid)", () => {
      manualValidateSimple(simpleValid);
    });

  await simpleBench.run();
  console.table(
    simpleBench.tasks.map((t) => ({
      Name: t.name,
      "ops/sec": Math.round(t.result!.hz).toLocaleString(),
      "avg (ns)": Math.round(t.result!.mean * 1e6).toLocaleString(),
      "p99 (ns)": Math.round(t.result!.p99 * 1e6).toLocaleString(),
      samples: t.result!.samples.length,
    }))
  );

  // ── Simple DTO invalid ───────────────────────────────────────────
  console.log("\n--- Simple DTO (5 fields, invalid input) ---\n");

  const simpleInvalidBench = new Bench({ time: 1500, warmupTime: 300 });

  simpleInvalidBench
    .add("tsgonest validate (invalid)", () => {
      validateCreateUserDto(simpleInvalid);
    })
    .add("class-validator validateSync (invalid)", () => {
      validateSync(cvSimpleInvalid);
    })
    .add("manual validation (invalid)", () => {
      manualValidateSimple(simpleInvalid);
    });

  await simpleInvalidBench.run();
  console.table(
    simpleInvalidBench.tasks.map((t) => ({
      Name: t.name,
      "ops/sec": Math.round(t.result!.hz).toLocaleString(),
      "avg (ns)": Math.round(t.result!.mean * 1e6).toLocaleString(),
      "p99 (ns)": Math.round(t.result!.p99 * 1e6).toLocaleString(),
      samples: t.result!.samples.length,
    }))
  );

  // ── Complex DTO benchmarks ───────────────────────────────────────
  console.log("\n--- Complex DTO (nested objects + arrays, valid input) ---\n");

  const complexBench = new Bench({ time: 1500, warmupTime: 300 });

  complexBench
    .add("tsgonest validate (valid)", () => {
      validateCreateOrderDto(complexValid);
    })
    .add("class-validator validateSync (valid)", () => {
      validateSync(cvComplexValid);
    });

  await complexBench.run();
  console.table(
    complexBench.tasks.map((t) => ({
      Name: t.name,
      "ops/sec": Math.round(t.result!.hz).toLocaleString(),
      "avg (ns)": Math.round(t.result!.mean * 1e6).toLocaleString(),
      "p99 (ns)": Math.round(t.result!.p99 * 1e6).toLocaleString(),
      samples: t.result!.samples.length,
    }))
  );

  // ── Complex DTO invalid ──────────────────────────────────────────
  console.log("\n--- Complex DTO (nested objects + arrays, invalid input) ---\n");

  const complexInvalidBench = new Bench({ time: 1500, warmupTime: 300 });

  complexInvalidBench
    .add("tsgonest validate (invalid)", () => {
      validateCreateOrderDto(complexInvalid);
    })
    .add("class-validator validateSync (invalid)", () => {
      validateSync(cvComplexInvalid);
    });

  await complexInvalidBench.run();
  console.table(
    complexInvalidBench.tasks.map((t) => ({
      Name: t.name,
      "ops/sec": Math.round(t.result!.hz).toLocaleString(),
      "avg (ns)": Math.round(t.result!.mean * 1e6).toLocaleString(),
      "p99 (ns)": Math.round(t.result!.p99 * 1e6).toLocaleString(),
      samples: t.result!.samples.length,
    }))
  );
}

main().catch(console.error);
