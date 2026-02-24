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
function Param(name?: string): ParameterDecorator {
  return () => {};
}
function Body(): ParameterDecorator {
  return () => {};
}
function Query(): ParameterDecorator {
  return () => {};
}

import type { PaginationQuery, OrderResponse } from "./dto";

@Controller("orders")
export class OrderController {
  // Whole-object @Query() — should get assert injection with coercion
  @Get()
  async findAll(@Query() query: PaginationQuery): Promise<OrderResponse[]> {
    return [];
  }

  // Individual scalar @Param('id') with number — should get inline coercion
  @Get(":id")
  async findOne(@Param("id") id: number): Promise<OrderResponse> {
    return {} as OrderResponse;
  }

  // String @Param — no coercion needed
  @Get("by-status/:status")
  async findByStatus(@Param("status") status: string): Promise<OrderResponse[]> {
    return [];
  }

  // Mixed: @Body + @Query + @Param
  @Post(":id/items")
  async addItem(
    @Param("id") id: number,
    @Query() query: PaginationQuery,
    @Body() body: OrderResponse
  ): Promise<OrderResponse> {
    return {} as OrderResponse;
  }
}
