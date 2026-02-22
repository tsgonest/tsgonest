export interface User {
  id: number;
  name: string;
  email: string;
}

export function greet(user: User): string {
  return `Hello, ${user.name}!`;
}
