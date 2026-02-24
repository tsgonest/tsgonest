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
function Put(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Delete(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Param(name: string): ParameterDecorator {
  return () => {};
}
function Body(): ParameterDecorator {
  return () => {};
}
function Query(): ParameterDecorator {
  return () => {};
}

import type { CreateUserDto, UpdateUserDto, UserResponse } from "./user.dto";

@Controller("users")
export class UserController {
  @Get()
  async findAll(): Promise<UserResponse[]> {
    return [];
  }

  @Post()
  async create(@Body() body: CreateUserDto): Promise<UserResponse> {
    return {} as UserResponse;
  }

  @Put(":id")
  async update(
    @Param("id") id: string,
    @Body() body: UpdateUserDto
  ): Promise<UserResponse> {
    return {} as UserResponse;
  }

  @Delete(":id")
  async remove(@Param("id") id: string): Promise<void> {}
}
