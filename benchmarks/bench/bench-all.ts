import { Bench } from "tinybench";
import { z } from "zod";
import { Type } from "@sinclair/typebox";
import { TypeCompiler } from "@sinclair/typebox/compiler";
import { Value } from "@sinclair/typebox/value";
import {
  generateObjectSimple,
  generateObjectHierarchical,
  generateObjectRecursive,
  generateObjectUnionExplicit,
  generateArraySimple,
  generateArrayHierarchical,
  generateArrayRecursive,
  generateArrayRecursiveUnion,
} from "./generators.js";

// ── tsgonest generated functions ─────────────────────────────────
// @ts-ignore
import { serializeIBox3D, validateIBox3D, isIBox3D, assertIBox3D } from "../dist/object-simple.dto.IBox3D.tsgonest.js";
// @ts-ignore
import { serializeICustomer, validateICustomer, isICustomer } from "../dist/object-hierarchical.dto.ICustomer.tsgonest.js";
// @ts-ignore
import { serializeIDepartment, validateIDepartment, isIDepartment } from "../dist/object-recursive.dto.IDepartment.tsgonest.js";
// @ts-ignore
import { serializeIPerson, validateIPerson, isIPerson } from "../dist/array-simple.dto.IPerson.tsgonest.js";
// @ts-ignore
import { serializeICompany, validateICompany, isICompany } from "../dist/array-hierarchical.dto.ICompany.tsgonest.js";
// @ts-ignore
import { serializeICategory, validateICategory, isICategory } from "../dist/array-recursive.dto.ICategory.tsgonest.js";
// @ts-ignore
import { serializeCreateUserDto, validateCreateUserDto, isCreateUserDto, assertCreateUserDto, stringifyCreateUserDto } from "../dist/simple.dto.CreateUserDto.tsgonest.js";
// @ts-ignore
import { serializeCreateOrderDto, validateCreateOrderDto, isCreateOrderDto } from "../dist/complex.dto.CreateOrderDto.tsgonest.js";

// ── Zod schemas ──────────────────────────────────────────────────

const ZodPoint3D = z.object({ x: z.number(), y: z.number(), z: z.number() });
const ZodBox3D = z.object({ scale: ZodPoint3D, position: ZodPoint3D, rotate: ZodPoint3D, pivot: ZodPoint3D });

const ZodAccount = z.object({ id: z.number(), code: z.string(), balance: z.number() });
const ZodMember = z.object({ id: z.number(), account: ZodAccount, name: z.string(), age: z.number(), sex: z.enum(["male", "female", "other"]), deceased: z.boolean() });
const ZodChannel = z.object({ id: z.number(), code: z.string(), name: z.string(), sequence: z.number(), exclusive: z.boolean(), priority: z.number() });
const ZodCustomer = z.object({ id: z.number(), channel: ZodChannel, member: ZodMember.nullable(), account: ZodAccount.nullable() });

const ZodDepartment: z.ZodType<any> = z.object({ id: z.number(), code: z.string(), name: z.string(), sales: z.number(), created_at: z.string(), children: z.lazy(() => z.array(ZodDepartment)) });

const ZodHobby = z.object({ name: z.string(), rank: z.number().min(0).max(10), body: z.string() });
const ZodPerson = z.object({ name: z.string(), age: z.number(), hobbies: z.array(ZodHobby) });

const ZodEmployee = z.object({ id: z.number(), name: z.string(), age: z.number(), grade: z.number() });
const ZodHierarchyDepartment = z.object({ id: z.number(), code: z.string(), name: z.string(), sales: z.number(), employees: z.array(ZodEmployee) });
const ZodCompany = z.object({ id: z.number(), serial: z.number(), name: z.string(), established_at: z.string(), departments: z.array(ZodHierarchyDepartment) });

const ZodCategory: z.ZodType<any> = z.object({ id: z.number(), code: z.string(), name: z.string(), sequence: z.number(), children: z.lazy(() => z.array(ZodCategory)) });

const ZodCreateUserDto = z.object({
  name: z.string().min(1).max(255),
  email: z.string().email(),
  age: z.number().min(0).max(150),
  isActive: z.boolean(),
  role: z.enum(["admin", "user", "moderator"]),
});

// ── TypeBox schemas ──────────────────────────────────────────────

const TBPoint3D = Type.Object({ x: Type.Number(), y: Type.Number(), z: Type.Number() });
const TBBox3D = Type.Object({ scale: TBPoint3D, position: TBPoint3D, rotate: TBPoint3D, pivot: TBPoint3D });

const TBAccount = Type.Object({ id: Type.Number(), code: Type.String(), balance: Type.Number() });
const TBMember = Type.Object({ id: Type.Number(), account: TBAccount, name: Type.String(), age: Type.Number(), sex: Type.Union([Type.Literal("male"), Type.Literal("female"), Type.Literal("other")]), deceased: Type.Boolean() });
const TBChannel = Type.Object({ id: Type.Number(), code: Type.String(), name: Type.String(), sequence: Type.Number(), exclusive: Type.Boolean(), priority: Type.Number() });
const TBCustomer = Type.Object({ id: Type.Number(), channel: TBChannel, member: Type.Union([TBMember, Type.Null()]), account: Type.Union([TBAccount, Type.Null()]) });

const TBDepartment = Type.Recursive((Self) => Type.Object({ id: Type.Number(), code: Type.String(), name: Type.String(), sales: Type.Number(), created_at: Type.String(), children: Type.Array(Self) }));

const TBHobby = Type.Object({ name: Type.String(), rank: Type.Number({ minimum: 0, maximum: 10 }), body: Type.String() });
const TBPerson = Type.Object({ name: Type.String(), age: Type.Number(), hobbies: Type.Array(TBHobby) });

const TBEmployee = Type.Object({ id: Type.Number(), name: Type.String(), age: Type.Number(), grade: Type.Number() });
const TBHierarchyDepartment = Type.Object({ id: Type.Number(), code: Type.String(), name: Type.String(), sales: Type.Number(), employees: Type.Array(TBEmployee) });
const TBCompany = Type.Object({ id: Type.Number(), serial: Type.Number(), name: Type.String(), established_at: Type.String(), departments: Type.Array(TBHierarchyDepartment) });

const TBCategory = Type.Recursive((Self) => Type.Object({ id: Type.Number(), code: Type.String(), name: Type.String(), sequence: Type.Number(), children: Type.Array(Self) }));

const TBCreateUserDto = Type.Object({
  name: Type.String({ minLength: 1, maxLength: 255 }),
  email: Type.String({ format: "email" }),
  age: Type.Number({ minimum: 0, maximum: 150 }),
  isActive: Type.Boolean(),
  role: Type.Union([Type.Literal("admin"), Type.Literal("user"), Type.Literal("moderator")]),
});

// Pre-compile TypeBox schemas for maximum performance
const TBCBox3D = TypeCompiler.Compile(TBBox3D);
const TBCCustomer = TypeCompiler.Compile(TBCustomer);
const TBCDepartment = TypeCompiler.Compile(TBDepartment);
const TBCPerson = TypeCompiler.Compile(TBPerson);
const TBCCompany = TypeCompiler.Compile(TBCompany);
const TBCCategory = TypeCompiler.Compile(TBCategory);
const TBCCreateUserDto = TypeCompiler.Compile(TBCreateUserDto);

// ── Types ──────────────────────────────────────────────────────────

interface BenchmarkResult {
  hz: number;
  mean_ns: number;
  p99_ns: number;
  samples: number;
}

interface BenchmarkOutput {
  timestamp: string;
  node_version: string;
  results: Record<string, BenchmarkResult>;
}

// ── Configuration ──────────────────────────────────────────────────

const WARMUP_MS = 200;
const TIME_MS = 1000;
const RUNS = 1; // increase for CI noise reduction

const jsonMode = process.argv.includes("--json");

// ── Data generation ────────────────────────────────────────────────

const data = {
  ObjectSimple: generateObjectSimple(),
  ObjectHierarchical: generateObjectHierarchical(),
  ObjectRecursive: generateObjectRecursive(),
  ObjectUnionExplicit: generateObjectUnionExplicit(),
  ArraySimple: generateArraySimple(),
  ArrayHierarchical: generateArrayHierarchical(),
  ArrayRecursive: generateArrayRecursive(),
  ArrayRecursiveUnion: generateArrayRecursiveUnion(),
  SimpleDto: {
    name: "John Doe",
    email: "john@example.com",
    age: 30,
    isActive: true,
    role: "admin" as const,
  },
  ComplexDto: {
    userId: "550e8400-e29b-41d4-a716-446655440000",
    items: [
      { productId: "550e8400-e29b-41d4-a716-446655440001", quantity: 2, unitPrice: 29.99, name: "Widget Pro" },
      { productId: "550e8400-e29b-41d4-a716-446655440002", quantity: 1, unitPrice: 49.99, name: "Gadget X" },
    ],
    shippingAddress: { street: "123 Main St", city: "Springfield", state: "IL", country: "US", zipCode: "62701" },
    couponCode: "SAVE20",
    paymentMethod: "card" as const,
    totalAmount: 109.97,
    notes: "Please deliver before 5pm",
  },
};

// ── Benchmark definitions ──────────────────────────────────────────

interface BenchDef {
  name: string;
  fn: () => void;
}

const serializeBenchmarks: BenchDef[] = [
  // Object shapes
  { name: "ObjectSimple/serialize/tsgonest", fn: () => serializeIBox3D(data.ObjectSimple) },
  { name: "ObjectSimple/serialize/json_stringify", fn: () => JSON.stringify(data.ObjectSimple) },
  { name: "ObjectHierarchical/serialize/tsgonest", fn: () => serializeICustomer(data.ObjectHierarchical) },
  { name: "ObjectHierarchical/serialize/json_stringify", fn: () => JSON.stringify(data.ObjectHierarchical) },
  { name: "ObjectRecursive/serialize/tsgonest", fn: () => serializeIDepartment(data.ObjectRecursive) },
  { name: "ObjectRecursive/serialize/json_stringify", fn: () => JSON.stringify(data.ObjectRecursive) },
  { name: "ObjectUnionExplicit/serialize/json_stringify", fn: () => JSON.stringify(data.ObjectUnionExplicit) },
  // Array shapes
  { name: "ArraySimple/serialize/tsgonest", fn: () => data.ArraySimple.map((p: any) => serializeIPerson(p)) },
  { name: "ArraySimple/serialize/json_stringify", fn: () => JSON.stringify(data.ArraySimple) },
  { name: "ArrayHierarchical/serialize/tsgonest", fn: () => data.ArrayHierarchical.map((c: any) => serializeICompany(c)) },
  { name: "ArrayHierarchical/serialize/json_stringify", fn: () => JSON.stringify(data.ArrayHierarchical) },
  { name: "ArrayRecursive/serialize/tsgonest", fn: () => data.ArrayRecursive.map((c: any) => serializeICategory(c)) },
  { name: "ArrayRecursive/serialize/json_stringify", fn: () => JSON.stringify(data.ArrayRecursive) },
  { name: "ArrayRecursiveUnion/serialize/json_stringify", fn: () => JSON.stringify(data.ArrayRecursiveUnion) },
  // Existing DTOs
  { name: "SimpleDto/serialize/tsgonest", fn: () => serializeCreateUserDto(data.SimpleDto) },
  { name: "SimpleDto/serialize/json_stringify", fn: () => JSON.stringify(data.SimpleDto) },
  { name: "ComplexDto/serialize/tsgonest", fn: () => serializeCreateOrderDto(data.ComplexDto) },
  { name: "ComplexDto/serialize/json_stringify", fn: () => JSON.stringify(data.ComplexDto) },
];

const validateBenchmarks: BenchDef[] = [
  // ── ObjectSimple ──
  { name: "ObjectSimple/validate/tsgonest", fn: () => validateIBox3D(data.ObjectSimple) },
  { name: "ObjectSimple/is/tsgonest", fn: () => isIBox3D(data.ObjectSimple) },
  { name: "ObjectSimple/validate/zod", fn: () => ZodBox3D.safeParse(data.ObjectSimple) },
  { name: "ObjectSimple/validate/typebox_compiled", fn: () => TBCBox3D.Check(data.ObjectSimple) },
  { name: "ObjectSimple/validate/typebox_value", fn: () => Value.Check(TBBox3D, data.ObjectSimple) },

  // ── ObjectHierarchical ──
  { name: "ObjectHierarchical/validate/tsgonest", fn: () => validateICustomer(data.ObjectHierarchical) },
  { name: "ObjectHierarchical/is/tsgonest", fn: () => isICustomer(data.ObjectHierarchical) },
  { name: "ObjectHierarchical/validate/zod", fn: () => ZodCustomer.safeParse(data.ObjectHierarchical) },
  { name: "ObjectHierarchical/validate/typebox_compiled", fn: () => TBCCustomer.Check(data.ObjectHierarchical) },
  { name: "ObjectHierarchical/validate/typebox_value", fn: () => Value.Check(TBCustomer, data.ObjectHierarchical) },

  // ── ObjectRecursive ──
  { name: "ObjectRecursive/validate/tsgonest", fn: () => validateIDepartment(data.ObjectRecursive) },
  { name: "ObjectRecursive/is/tsgonest", fn: () => isIDepartment(data.ObjectRecursive) },
  { name: "ObjectRecursive/validate/zod", fn: () => ZodDepartment.safeParse(data.ObjectRecursive) },
  { name: "ObjectRecursive/validate/typebox_compiled", fn: () => TBCDepartment.Check(data.ObjectRecursive) },
  { name: "ObjectRecursive/validate/typebox_value", fn: () => Value.Check(TBDepartment, data.ObjectRecursive) },

  // ── ArraySimple ──
  { name: "ArraySimple/validate/tsgonest", fn: () => data.ArraySimple.forEach((p: any) => validateIPerson(p)) },
  { name: "ArraySimple/is/tsgonest", fn: () => data.ArraySimple.every((p: any) => isIPerson(p)) },
  { name: "ArraySimple/validate/zod", fn: () => z.array(ZodPerson).safeParse(data.ArraySimple) },
  { name: "ArraySimple/validate/typebox_compiled", fn: () => data.ArraySimple.every((p: any) => TBCPerson.Check(p)) },
  { name: "ArraySimple/validate/typebox_value", fn: () => data.ArraySimple.every((p: any) => Value.Check(TBPerson, p)) },

  // ── ArrayHierarchical ──
  { name: "ArrayHierarchical/validate/tsgonest", fn: () => data.ArrayHierarchical.forEach((c: any) => validateICompany(c)) },
  { name: "ArrayHierarchical/is/tsgonest", fn: () => data.ArrayHierarchical.every((c: any) => isICompany(c)) },
  { name: "ArrayHierarchical/validate/zod", fn: () => z.array(ZodCompany).safeParse(data.ArrayHierarchical) },
  { name: "ArrayHierarchical/validate/typebox_compiled", fn: () => data.ArrayHierarchical.every((c: any) => TBCCompany.Check(c)) },
  { name: "ArrayHierarchical/validate/typebox_value", fn: () => data.ArrayHierarchical.every((c: any) => Value.Check(TBCompany, c)) },

  // ── ArrayRecursive ──
  { name: "ArrayRecursive/validate/tsgonest", fn: () => data.ArrayRecursive.forEach((c: any) => validateICategory(c)) },
  { name: "ArrayRecursive/is/tsgonest", fn: () => data.ArrayRecursive.every((c: any) => isICategory(c)) },
  { name: "ArrayRecursive/validate/zod", fn: () => z.array(ZodCategory).safeParse(data.ArrayRecursive) },
  { name: "ArrayRecursive/validate/typebox_compiled", fn: () => data.ArrayRecursive.every((c: any) => TBCCategory.Check(c)) },
  { name: "ArrayRecursive/validate/typebox_value", fn: () => data.ArrayRecursive.every((c: any) => Value.Check(TBCategory, c)) },

  // ── SimpleDto (with constraints) ──
  { name: "SimpleDto/validate/tsgonest", fn: () => validateCreateUserDto(data.SimpleDto) },
  { name: "SimpleDto/is/tsgonest", fn: () => isCreateUserDto(data.SimpleDto) },
  { name: "SimpleDto/assert/tsgonest", fn: () => assertCreateUserDto(data.SimpleDto) },
  { name: "SimpleDto/validate/zod", fn: () => ZodCreateUserDto.safeParse(data.SimpleDto) },
  { name: "SimpleDto/validate/typebox_compiled", fn: () => TBCCreateUserDto.Check(data.SimpleDto) },
  { name: "SimpleDto/validate/typebox_value", fn: () => Value.Check(TBCreateUserDto, data.SimpleDto) },

  // ── ComplexDto ──
  { name: "ComplexDto/validate/tsgonest", fn: () => validateCreateOrderDto(data.ComplexDto) },
  { name: "ComplexDto/is/tsgonest", fn: () => isCreateOrderDto(data.ComplexDto) },
];

// ── Runner ─────────────────────────────────────────────────────────

async function runBenchGroup(defs: BenchDef[]): Promise<Map<string, BenchmarkResult>> {
  const results = new Map<string, BenchmarkResult>();

  // Run each benchmark individually to avoid OOM from accumulated samples
  for (const def of defs) {
    const runs: BenchmarkResult[] = [];
    for (let run = 0; run < RUNS; run++) {
      const bench = new Bench({ time: TIME_MS, warmupTime: WARMUP_MS });
      bench.add(def.name, def.fn);
      await bench.run();
      const r = bench.tasks[0].result!;
      runs.push({
        hz: r.hz,
        mean_ns: r.mean * 1e6,
        p99_ns: r.p99 * 1e6,
        samples: r.samples.length,
      });
    }
    // Take median by hz
    runs.sort((a, b) => a.hz - b.hz);
    results.set(def.name, runs[Math.floor(runs.length / 2)]);
  }

  return results;
}

async function main() {
  const allBenchmarks = [...serializeBenchmarks, ...validateBenchmarks];
  const results = await runBenchGroup(allBenchmarks);

  if (jsonMode) {
    const output: BenchmarkOutput = {
      timestamp: new Date().toISOString(),
      node_version: process.version,
      results: Object.fromEntries(results),
    };
    console.log(JSON.stringify(output, null, 2));
    return;
  }

  // Human-readable output
  console.log("=== tsgonest Comprehensive Benchmark ===\n");
  console.log(`Node.js ${process.version}`);
  console.log(`Date: ${new Date().toISOString()}`);
  console.log(`Runs: ${RUNS} (median taken)\n`);

  // Group by shape
  const groups = new Map<string, Map<string, BenchmarkResult>>();
  for (const [name, result] of results) {
    const [shape, ...rest] = name.split("/");
    const key = rest.join("/");
    if (!groups.has(shape)) groups.set(shape, new Map());
    groups.get(shape)!.set(key, result);
  }

  for (const [shape, benchmarks] of groups) {
    console.log(`--- ${shape} ---\n`);
    const rows = [];
    for (const [name, result] of benchmarks) {
      rows.push({
        Benchmark: name,
        "ops/sec": Math.round(result.hz).toLocaleString(),
        "avg (ns)": Math.round(result.mean_ns).toLocaleString(),
        "p99 (ns)": Math.round(result.p99_ns).toLocaleString(),
        samples: result.samples,
      });
    }
    console.table(rows);

    // Print speedup comparisons
    const tgSerialize = benchmarks.get("serialize/tsgonest");
    const jsStringify = benchmarks.get("serialize/json_stringify");
    if (tgSerialize && jsStringify) {
      const ratio = tgSerialize.hz / jsStringify.hz;
      console.log(`  serialize: tsgonest is ${ratio.toFixed(2)}x ${ratio > 1 ? "faster" : "slower"} than JSON.stringify`);
    }

    const tgValidate = benchmarks.get("validate/tsgonest");
    const tgIs = benchmarks.get("is/tsgonest");
    const zodValidate = benchmarks.get("validate/zod");
    const tbCompiled = benchmarks.get("validate/typebox_compiled");

    if (tgValidate && zodValidate) {
      const ratio = tgValidate.hz / zodValidate.hz;
      console.log(`  validate: tsgonest is ${ratio.toFixed(2)}x ${ratio > 1 ? "faster" : "slower"} than zod`);
    }
    if (tgIs && zodValidate) {
      const ratio = tgIs.hz / zodValidate.hz;
      console.log(`  is(): tsgonest is ${ratio.toFixed(2)}x ${ratio > 1 ? "faster" : "slower"} than zod`);
    }
    if (tgValidate && tbCompiled) {
      const ratio = tgValidate.hz / tbCompiled.hz;
      console.log(`  validate: tsgonest is ${ratio.toFixed(2)}x ${ratio > 1 ? "faster" : "slower"} than typebox (compiled)`);
    }
    if (tgIs && tbCompiled) {
      const ratio = tgIs.hz / tbCompiled.hz;
      console.log(`  is(): tsgonest is ${ratio.toFixed(2)}x ${ratio > 1 ? "faster" : "slower"} than typebox (compiled)`);
    }
    console.log("");
  }
}

main().catch(console.error);
