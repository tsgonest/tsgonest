import type { User } from "./user";

export interface Order {
  id: number;
  user: User;
  total: number;
}

export function createOrder(id: number, user: User, total: number): Order {
  return { id, user, total };
}
