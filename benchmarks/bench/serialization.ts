import { Bench } from "tinybench";

// ── tsgonest generated serializers ───────────────────────────────────
// @ts-ignore — generated JS, no TS declarations in benchmark dist
import { serializeCreateUserDto } from "../dist/simple.dto.CreateUserDto.tsgonest.js";
// @ts-ignore
import { serializeCreateOrderDto } from "../dist/complex.dto.CreateOrderDto.tsgonest.js";

// ── Test data ────────────────────────────────────────────────────────

const simpleData = {
  name: "John Doe",
  email: "john@example.com",
  age: 30,
  isActive: true,
  role: "admin",
};

const complexData = {
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

// Data with strings that need escaping (worst case for __s() fast path)
const simpleDataWithEscaping = {
  name: 'John "The Dev" O\'Brien\nnewline',
  email: "john@example.com",
  age: 30,
  isActive: true,
  role: "admin",
};

// Larger array payload (10 items)
const complexDataLargeArray = {
  ...complexData,
  items: Array.from({ length: 10 }, (_, i) => ({
    productId: `550e8400-e29b-41d4-a716-44665544${String(i).padStart(4, "0")}`,
    quantity: i + 1,
    unitPrice: 9.99 + i * 10,
    name: `Product ${i + 1}`,
  })),
};

// ── Manual hand-optimized serializer (baseline) ──────────────────────

function manualSerializeSimple(input: any): string {
  return (
    '{"name":' +
    JSON.stringify(input.name) +
    ',"email":' +
    JSON.stringify(input.email) +
    ',"age":' +
    input.age +
    ',"isActive":' +
    input.isActive +
    ',"role":' +
    JSON.stringify(input.role) +
    "}"
  );
}

// ── Run benchmarks ───────────────────────────────────────────────────

async function main() {
  console.log("=== Serialization Benchmark ===\n");
  console.log(`Node.js ${process.version}`);
  console.log(`Date: ${new Date().toISOString()}\n`);

  // Verify output correctness
  const tgSimple = serializeCreateUserDto(simpleData);
  const jsSimple = JSON.stringify(simpleData);
  const parsedTg = JSON.parse(tgSimple);
  const parsedJs = JSON.parse(jsSimple);
  if (JSON.stringify(parsedTg) !== JSON.stringify(parsedJs)) {
    console.error("tsgonest:", tgSimple);
    console.error("JSON.stringify:", jsSimple);
    throw new Error("Simple DTO serialization mismatch!");
  }
  console.log("Correctness verified: tsgonest output matches JSON.stringify\n");

  // ── Simple DTO ───────────────────────────────────────────────────
  console.log("--- Simple DTO (5 fields, clean strings) ---\n");

  const simpleBench = new Bench({ time: 1500, warmupTime: 300 });

  simpleBench
    .add("tsgonest serialize", () => {
      serializeCreateUserDto(simpleData);
    })
    .add("JSON.stringify", () => {
      JSON.stringify(simpleData);
    })
    .add("manual hand-optimized", () => {
      manualSerializeSimple(simpleData);
    });

  await simpleBench.run();
  printResults(simpleBench);

  // ── Simple DTO with escape chars ─────────────────────────────────
  console.log("\n--- Simple DTO (5 fields, strings needing escaping) ---\n");

  const escBench = new Bench({ time: 1500, warmupTime: 300 });

  escBench
    .add("tsgonest serialize (escaping)", () => {
      serializeCreateUserDto(simpleDataWithEscaping);
    })
    .add("JSON.stringify (escaping)", () => {
      JSON.stringify(simpleDataWithEscaping);
    });

  await escBench.run();
  printResults(escBench);

  // ── Complex DTO ──────────────────────────────────────────────────
  console.log("\n--- Complex DTO (nested objects + 2-item array) ---\n");

  const complexBench = new Bench({ time: 1500, warmupTime: 300 });

  complexBench
    .add("tsgonest serialize", () => {
      serializeCreateOrderDto(complexData);
    })
    .add("JSON.stringify", () => {
      JSON.stringify(complexData);
    });

  await complexBench.run();
  printResults(complexBench);

  // ── Complex DTO large array ──────────────────────────────────────
  console.log("\n--- Complex DTO (nested objects + 10-item array) ---\n");

  const largeBench = new Bench({ time: 1500, warmupTime: 300 });

  largeBench
    .add("tsgonest serialize (10 items)", () => {
      serializeCreateOrderDto(complexDataLargeArray);
    })
    .add("JSON.stringify (10 items)", () => {
      JSON.stringify(complexDataLargeArray);
    });

  await largeBench.run();
  printResults(largeBench);
}

function printResults(bench: Bench) {
  const rows = bench.tasks.map((t) => ({
    Name: t.name,
    "ops/sec": Math.round(t.result!.hz).toLocaleString(),
    "avg (ns)": Math.round(t.result!.mean * 1e6).toLocaleString(),
    "p99 (ns)": Math.round(t.result!.p99 * 1e6).toLocaleString(),
    samples: t.result!.samples.length,
  }));
  console.table(rows);

  // Print speedup ratio
  if (bench.tasks.length >= 2) {
    const fastest = bench.tasks.reduce((a, b) =>
      a.result!.hz > b.result!.hz ? a : b
    );
    const slowest = bench.tasks.reduce((a, b) =>
      a.result!.hz < b.result!.hz ? a : b
    );
    if (fastest !== slowest) {
      const ratio = fastest.result!.hz / slowest.result!.hz;
      console.log(
        `  "${fastest.name}" is ${ratio.toFixed(1)}x faster than "${slowest.name}"\n`
      );
    }
  }
}

main().catch(console.error);
