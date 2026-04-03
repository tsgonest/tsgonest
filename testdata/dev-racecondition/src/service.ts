import { formatName, add } from "./helper.js";

export class UserService {
  greet(first: string, last: string): string {
    return `Hello, ${formatName(first, last)}!`;
  }

  sum(a: number, b: number): number {
    return add(a, b);
  }
}
