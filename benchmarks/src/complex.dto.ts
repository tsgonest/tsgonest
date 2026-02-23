/**
 * Complex DTO — nested objects, arrays, optional fields, union types.
 * Represents a realistic e-commerce order creation payload.
 */
export interface CreateOrderDto {
  /** @format uuid */
  userId: string;
  items: OrderItem[];
  shippingAddress: Address;
  billingAddress?: Address;
  /** @pattern ^[A-Z0-9]{6,12}$ */
  couponCode?: string;
  paymentMethod: "card" | "bank" | "crypto";
  /** @minimum 0 */
  totalAmount: number;
  notes?: string;
}

export interface OrderItem {
  /** @format uuid */
  productId: string;
  /** @minimum 1 @maximum 999 */
  quantity: number;
  /** @minimum 0 */
  unitPrice: number;
  /** @minLength 1 */
  name: string;
}

export interface Address {
  /** @minLength 1 */
  street: string;
  /** @minLength 1 */
  city: string;
  state?: string;
  /** @minLength 1 */
  country: string;
  /** @pattern ^[0-9]{5}(-[0-9]{4})?$ */
  zipCode: string;
}
