/**
 * Seeded deterministic data generators for all 8 nestia benchmark shapes.
 */

// Simple seeded PRNG (mulberry32)
function createRng(seed: number) {
  let s = seed | 0;
  return () => {
    s = (s + 0x6d2b79f5) | 0;
    let t = Math.imul(s ^ (s >>> 15), 1 | s);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const rng = createRng(42);

function randInt(min: number, max: number): number {
  return Math.floor(rng() * (max - min + 1)) + min;
}

function randFloat(min: number, max: number): number {
  return rng() * (max - min) + min;
}

function randString(len: number): string {
  const chars = "abcdefghijklmnopqrstuvwxyz";
  let s = "";
  for (let i = 0; i < len; i++) s += chars[randInt(0, chars.length - 1)];
  return s;
}

function randPick<T>(arr: T[]): T {
  return arr[randInt(0, arr.length - 1)];
}

// ── Object Simple: IBox3D ──
function genPoint3D() {
  return { x: randFloat(-100, 100), y: randFloat(-100, 100), z: randFloat(-100, 100) };
}

export function generateObjectSimple() {
  return {
    scale: genPoint3D(),
    position: genPoint3D(),
    rotate: genPoint3D(),
    pivot: genPoint3D(),
  };
}

// ── Object Hierarchical: ICustomer ──
function genAccount() {
  return { id: randInt(1, 10000), code: randString(8), balance: randFloat(0, 100000) };
}

function genMember() {
  return {
    id: randInt(1, 10000),
    account: genAccount(),
    name: randString(10),
    age: randInt(18, 80),
    sex: randPick(["male", "female", "other"] as const),
    deceased: rng() > 0.9,
  };
}

function genChannel() {
  return {
    id: randInt(1, 1000),
    code: randString(6),
    name: randString(12),
    sequence: randInt(1, 100),
    exclusive: rng() > 0.5,
    priority: randInt(1, 10),
  };
}

export function generateObjectHierarchical() {
  return {
    id: randInt(1, 10000),
    channel: genChannel(),
    member: rng() > 0.3 ? genMember() : null,
    account: rng() > 0.3 ? genAccount() : null,
  };
}

// ── Object Recursive: IDepartment ──
function genDepartment(depth: number): any {
  return {
    id: randInt(1, 10000),
    code: randString(6),
    name: randString(12),
    sales: randFloat(0, 1000000),
    created_at: new Date(randInt(2000, 2024), randInt(0, 11), randInt(1, 28)).toISOString().split("T")[0],
    children: depth > 0 ? Array.from({ length: randInt(1, 3) }, () => genDepartment(depth - 1)) : [],
  };
}

export function generateObjectRecursive() {
  return genDepartment(3);
}

// ── Object Union Explicit: IShape[] ──
function genShapePoint(): any {
  return { type: "point", x: randFloat(-100, 100), y: randFloat(-100, 100) };
}

function genShape(): any {
  const types = ["point", "line", "triangle", "rectangle", "polyline", "polygon", "circle"] as const;
  const t = randPick([...types]);
  switch (t) {
    case "point":
      return genShapePoint();
    case "line":
      return { type: "line", p1: genShapePoint(), p2: genShapePoint() };
    case "triangle":
      return { type: "triangle", p1: genShapePoint(), p2: genShapePoint(), p3: genShapePoint() };
    case "rectangle":
      return {
        type: "rectangle",
        p1: genShapePoint(),
        p2: genShapePoint(),
        p3: genShapePoint(),
        p4: genShapePoint(),
      };
    case "polyline":
      return {
        type: "polyline",
        points: Array.from({ length: randInt(3, 8) }, () => genShapePoint()),
      };
    case "polygon":
      return {
        type: "polygon",
        outer: { type: "polyline" as const, points: Array.from({ length: randInt(4, 8) }, () => genShapePoint()) },
        inner: Array.from({ length: randInt(0, 2) }, () => ({
          type: "polyline" as const,
          points: Array.from({ length: randInt(3, 5) }, () => genShapePoint()),
        })),
      };
    case "circle":
      return { type: "circle", center: genShapePoint(), radius: randFloat(1, 50) };
  }
}

export function generateObjectUnionExplicit() {
  return Array.from({ length: randInt(5, 10) }, () => genShape());
}

// ── Array Simple: IPerson[] ──
function genHobby() {
  return { name: randString(8), rank: randInt(0, 10), body: randString(20) };
}

function genPerson() {
  return {
    name: randString(10),
    age: randInt(18, 80),
    hobbies: Array.from({ length: randInt(1, 5) }, () => genHobby()),
  };
}

export function generateArraySimple() {
  return Array.from({ length: 10 }, () => genPerson());
}

// ── Array Hierarchical: ICompany[] ──
function genEmployee() {
  return { id: randInt(1, 10000), name: randString(10), age: randInt(22, 65), grade: randInt(1, 10) };
}

function genHierarchyDepartment() {
  return {
    id: randInt(1, 10000),
    code: randString(6),
    name: randString(12),
    sales: randFloat(0, 1000000),
    employees: Array.from({ length: randInt(3, 10) }, () => genEmployee()),
  };
}

function genCompany() {
  return {
    id: randInt(1, 10000),
    serial: randInt(1, 100000),
    name: randString(15),
    established_at: new Date(randInt(1950, 2020), randInt(0, 11), randInt(1, 28)).toISOString().split("T")[0],
    departments: Array.from({ length: randInt(2, 5) }, () => genHierarchyDepartment()),
  };
}

export function generateArrayHierarchical() {
  return Array.from({ length: 3 }, () => genCompany());
}

// ── Array Recursive: ICategory tree ──
function genCategory(depth: number, branching: number): any {
  return {
    id: randInt(1, 10000),
    code: randString(6),
    name: randString(12),
    sequence: randInt(1, 100),
    children:
      depth > 0 ? Array.from({ length: branching }, () => genCategory(depth - 1, branching)) : [],
  };
}

export function generateArrayRecursive() {
  return Array.from({ length: 2 }, () => genCategory(4, 2));
}

// ── Array Recursive Union: IBucket file system ──
function genBucket(depth: number): any {
  if (depth <= 0) {
    // Leaf node: random file type
    const fileTypes = ["image", "text", "zip"] as const;
    const ft = randPick([...fileTypes]);
    const base = { id: randInt(1, 10000), name: randString(8), path: "/" + randString(12) };
    switch (ft) {
      case "image":
        return { ...base, type: "image", width: randInt(100, 4000), height: randInt(100, 4000), url: "https://example.com/" + randString(8) + ".png", size: randInt(1000, 10000000) };
      case "text":
        return { ...base, type: "text", size: randInt(100, 100000), content: randString(50), encoding: "utf-8" };
      case "zip":
        return { ...base, type: "zip", size: randInt(1000, 10000000), count: randInt(1, 100) };
    }
  }
  const isShared = rng() > 0.7;
  return {
    type: isShared ? "shared-directory" : "directory",
    id: randInt(1, 10000),
    name: randString(8),
    path: "/" + randString(12),
    ...(isShared ? { access: randPick(["read", "write"] as const) } : {}),
    children: Array.from({ length: randInt(2, 4) }, () => genBucket(depth - 1)),
  };
}

export function generateArrayRecursiveUnion() {
  return Array.from({ length: 3 }, () => genBucket(3));
}
