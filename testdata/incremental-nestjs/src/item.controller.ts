function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Body(): ParameterDecorator {
  return () => {};
}

import type { CreateItemDto, ItemResponse } from "./item.dto";

@Controller("items")
export class ItemController {
  @Get()
  async findAll(): Promise<ItemResponse[]> {
    return [];
  }

  @Post()
  async create(@Body() body: CreateItemDto): Promise<ItemResponse> {
    return {} as ItemResponse;
  }
}
