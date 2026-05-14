import { tags } from "@tsgonest/types";

export interface CreateUserDto {
  name: string & tags.MinLength<1>;
  email: string & tags.Email;
  age: number & tags.Minimum<0>;
}

export interface UserResponse {
  id: string & tags.Uuid;
  name: string;
  email: string;
}

export interface ListQuery {
  limit?: number & tags.Maximum<100>;
  cursor?: string;
}
