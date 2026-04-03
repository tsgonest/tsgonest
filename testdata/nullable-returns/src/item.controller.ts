// Stub decorators for testing
function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Param(name: string): ParameterDecorator {
  return () => {};
}
function Query(): ParameterDecorator {
  return () => {};
}

import type { ItemResponse } from "./item.dto";

@Controller("items")
export class ItemController {
  // ── Case 1: Basic nullable DTO return (Prisma findFirst pattern) ──
  @Get(":id")
  async getById(@Param("id") id: string): Promise<ItemResponse | null> {
    // Simulates: return this.prisma.item.findFirst({ where: { id } });
    return null;
  }

  // ── Case 2: Nullable array return (search with no results = null) ──
  @Get("search")
  async search(@Query() q: string): Promise<ItemResponse[] | null> {
    if (!q) return null;
    return [];
  }

  // ── Case 3: Non-nullable return (baseline — should NOT get null guard) ──
  @Get()
  async findAll(): Promise<ItemResponse[]> {
    return [];
  }

  // ── Case 4: Optional (undefined) DTO return (Array.find / Map.get pattern) ──
  @Get("default")
  async getDefault(): Promise<ItemResponse | undefined> {
    return undefined;
  }

  // ── Case 5: Non-async nullable return (synchronous cache lookup) ──
  @Get("cached")
  getCached(): ItemResponse | null {
    return null;
  }

  // ── Case 6: Multiple return paths — try/catch with null fallback ──
  @Get("safe/:id")
  async getSafe(@Param("id") id: string): Promise<ItemResponse | null> {
    try {
      const item = {} as ItemResponse;
      return item;
    } catch {
      return null;
    }
  }

  // ── Case 7: Array of nullable elements (Promise.all over findFirst) ──
  @Post("batch")
  async getByIds(): Promise<(ItemResponse | null)[]> {
    return [null, {} as ItemResponse];
  }
}
