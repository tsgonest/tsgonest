// This file has a type error — assigning string to number
export function add(a: number, b: number): number {
  return a + b;
}

const result: number = add(1, "two");

export interface User {
  name: string;
  age: number;
}

// Type error: missing 'age' property
const user: User = { name: "Alice" };
