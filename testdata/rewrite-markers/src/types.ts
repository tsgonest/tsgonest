export interface CreateUserDto {
  name: string;
  email: string;
  age: number;
}

export interface UserResponse {
  id: number;
  name: string;
  email: string;
}
