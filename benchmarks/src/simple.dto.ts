/**
 * Simple DTO — 5 required fields with constraints.
 * Used for baseline benchmarking of validation and serialization.
 */
export interface CreateUserDto {
  /** @minLength 1 @maxLength 255 */
  name: string;
  /** @format email */
  email: string;
  /** @minimum 0 @maximum 150 */
  age: number;
  isActive: boolean;
  role: "admin" | "user" | "moderator";
}
