export interface CreateUserDto {
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
  /** @format email */
  email: string;
  /**
   * @minimum 0
   * @maximum 150
   */
  age: number;
}

export interface UpdateUserDto {
  name?: string;
  email?: string;
  age?: number;
}

export interface UserResponse {
  id: number;
  name: string;
  email: string;
  age: number;
  createdAt: string;
}

export interface ListQuery {
  page?: number;
  limit?: number;
  search?: string;
}
