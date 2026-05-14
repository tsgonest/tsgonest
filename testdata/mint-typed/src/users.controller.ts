import { Body, Controller, Get, Param, Post, Query } from "@mintkit/core";
import type { CreateUserDto, ListQuery, UserResponse } from "./users.dto";

@Controller("/users")
export class UsersController {
  @Get()
  list(@Query() q: ListQuery): UserResponse[] {
    const limit = q.limit ?? 10;
    const out: UserResponse[] = [];
    for (let i = 0; i < limit && i < 3; i++) {
      out.push({
        id: `11111111-1111-1111-1111-${String(i).padStart(12, "0")}`,
        name: `user-${i}`,
        email: `user${i}@example.com`,
      });
    }
    return out;
  }

  @Get(":id")
  one(@Param("id") id: string): UserResponse {
    return {
      id: id.length === 36 ? id : "11111111-1111-1111-1111-111111111111",
      name: "ada",
      email: "ada@example.com",
    };
  }

  @Post()
  create(@Body() body: CreateUserDto): UserResponse {
    return {
      id: "11111111-1111-1111-1111-111111111111",
      name: body.name,
      email: body.email,
    };
  }
}
