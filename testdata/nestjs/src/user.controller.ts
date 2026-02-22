// NOTE: This is a mock NestJS controller for testing purposes.
// In a real project, these decorators would come from @nestjs/common.
// For now we declare stub decorators to make the file parseable by tsgo.

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
function Patch(path?: string): MethodDecorator {
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
function HttpCode(code: number): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

// Custom parameter decorators (simulating createParamDecorator pattern)

/** @in param */
function ExtractUserId(name: string): ParameterDecorator {
  return () => {};
}

// No @in — should be silently skipped in OpenAPI
function CurrentUser(): ParameterDecorator {
  return () => {};
}

import type { CreateUserDto, UpdateUserDto, UserResponse, ListQuery } from "./user.dto";

@Controller("users")
export class UserController {
  @Get()
  async findAll(@Query() query: ListQuery): Promise<UserResponse[]> {
    return [];
  }

  @Get(":id")
  async findOne(@Param("id") id: string): Promise<UserResponse> {
    return {} as UserResponse;
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
  @HttpCode(204)
  async remove(@Param("id") id: string): Promise<void> {}

  // Custom decorator: @ExtractUserId has @in param → should appear in OpenAPI
  // @CurrentUser has no @in → should be silently skipped
  @Get("profile/:userId")
  async getProfile(
    @ExtractUserId("userId") userId: string,
    @CurrentUser() user: { id: number },
  ): Promise<UserResponse> {
    return {} as UserResponse;
  }
}
