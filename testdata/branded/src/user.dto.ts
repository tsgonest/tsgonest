import type {
  Format, MinLength, MaxLength, Minimum, Maximum, ExclusiveMinimum,
  Int, Email, Uuid, Positive, NonNegative,
  StartsWith, Includes,
} from "./tags.js";

/**
 * DTO using branded types for validation constraints.
 * tsgonest should extract constraints from __tsgonest_* phantom properties.
 */
export interface CreateUserDto {
  email: string & Email;
  name: string & MinLength<1> & MaxLength<255>;
  age: number & Minimum<0> & Maximum<150>;
}

/**
 * DTO with numeric constraints via branded types.
 */
export interface ProductDto {
  id: string & Uuid;
  price: number & Positive & Int;
  quantity: number & NonNegative;
}

/**
 * DTO with string content checks.
 */
export interface ConfigDto {
  websiteUrl: string & StartsWith<"https://">;
  contactEmail: string & Format<"email"> & Includes<"@">;
}

/**
 * Response DTO for serialization testing.
 */
export interface UserResponse {
  id: string & Uuid;
  email: string & Email;
  name: string;
  age: number;
  createdAt: string;
}
