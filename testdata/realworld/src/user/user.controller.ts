import {
  Controller,
  Get,
  Post,
  Put,
  Delete,
  Param,
  Body,
  Query,
  HttpCode,
} from "../common/decorators";
import type {
  CreateUserDto,
  UpdateUserDto,
  UserResponse,
  UserListQuery,
  ProfileResponse,
} from "./user.dto";
import type { PaginatedResponse } from "../common/types";

/**
 * User management controller
 * @tag Users
 */
@Controller("users")
export class UserController {
  /**
   * List all users with pagination and filtering
   * @summary List users
   */
  @Get()
  async findAll(
    @Query() query: UserListQuery
  ): Promise<PaginatedResponse<UserResponse>> {
    return {} as PaginatedResponse<UserResponse>;
  }

  /**
   * Get a single user by ID
   * @summary Get user
   */
  @Get(":id")
  async findOne(@Param("id") id: string): Promise<UserResponse> {
    return {} as UserResponse;
  }

  /**
   * Create a new user
   * @summary Create user
   */
  @Post()
  async create(@Body() body: CreateUserDto): Promise<UserResponse> {
    return {} as UserResponse;
  }

  /**
   * Update an existing user
   * @summary Update user
   */
  @Put(":id")
  async update(
    @Param("id") id: string,
    @Body() body: UpdateUserDto
  ): Promise<UserResponse> {
    return {} as UserResponse;
  }

  /**
   * Delete a user
   * @summary Delete user
   */
  @Delete(":id")
  @HttpCode(204)
  async remove(@Param("id") id: string): Promise<void> {}

  /**
   * Get current user's profile
   * @summary Get profile
   */
  @Get("me/profile")
  async getProfile(): Promise<ProfileResponse> {
    return {} as ProfileResponse;
  }
}
