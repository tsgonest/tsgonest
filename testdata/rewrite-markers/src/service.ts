import { is, assert, validate } from "tsgonest";
import type { CreateUserDto, UserResponse } from "./types";

export function processUser(input: unknown) {
  if (is<CreateUserDto>(input)) {
    console.log("Valid user:", input.name);
  }

  const user = assert<CreateUserDto>(input);
  console.log("Asserted user:", user.name);

  const result = validate<CreateUserDto>(input);
  if (result.success) {
    console.log("Validated:", result.data);
  }
}
