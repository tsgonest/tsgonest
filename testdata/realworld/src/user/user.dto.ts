import type { Address, PaginationQuery } from "../common/types";

/** User roles */
export enum UserRole {
  ADMIN = "admin",
  MODERATOR = "moderator",
  USER = "user",
}

/** Create user request DTO */
export interface CreateUserDto {
  /**
   * @minLength 2
   * @maxLength 50
   */
  username: string;
  /** @format email */
  email: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  password: string;
  displayName?: string;
  /** @maxLength 500 */
  bio?: string;
  role: UserRole;
  address?: Address;
  /** @minItems 0 @maxItems 10 */
  tags?: string[];
}

/** Update user DTO — all fields optional */
export type UpdateUserDto = Partial<CreateUserDto>;

/** User profile response */
export interface UserResponse {
  id: number;
  username: string;
  email: string;
  displayName: string | null;
  bio: string | null;
  role: UserRole;
  address: Address | null;
  tags: string[];
  createdAt: string;
  updatedAt: string;
}

/** User search/list query */
export interface UserListQuery extends PaginationQuery {
  role?: UserRole;
  search?: string;
}

/** Profile update response — omits password, adds metadata */
export type ProfileResponse = Omit<UserResponse, "role"> & {
  isVerified: boolean;
  lastLoginAt: string | null;
};
