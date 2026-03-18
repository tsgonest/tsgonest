/**
 * @minLength 1
 * @maxLength 255
 */
type Name = string;

/**
 * @format email
 */
type Email = string;

/**
 * @minimum 0
 * @maximum 150
 */
type Age = number;

export interface CreateUserDto {
  name: Name;
  email: Email;
  age: Age;
  isActive: boolean;
  role: 'admin' | 'user' | 'moderator';
}

export interface UserDto {
  /** @format uuid */
  id: string;
  name: string;
  email: string;
  age: number;
  isActive: boolean;
  role: 'admin' | 'user' | 'moderator';
  createdAt: string;
}

export interface CreateOrderDto {
  /** @format uuid */
  userId: string;
  items: OrderItemDto[];
  /** @minimum 0 */
  totalAmount: number;
}

export interface OrderItemDto {
  /** @format uuid */
  productId: string;
  /** @minimum 1 */
  quantity: number;
  /** @minimum 0 */
  unitPrice: number;
  name: string;
}

export interface OrderDto {
  /** @format uuid */
  id: string;
  /** @format uuid */
  userId: string;
  items: OrderItemDto[];
  totalAmount: number;
  status: 'pending' | 'confirmed' | 'shipped' | 'delivered';
  createdAt: string;
}
