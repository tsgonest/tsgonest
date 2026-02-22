/** Base payment method fields */
interface BasePayment {
  /** @minimum 0.01 */
  amount: number;
  currency: string;
  description?: string;
}

/** Card payment method */
export interface CardPayment extends BasePayment {
  type: "card";
  /** @pattern ^[0-9]{13,19}$ */
  cardNumber: string;
  /** @pattern ^(0[1-9]|1[0-2])\/[0-9]{2}$ */
  expiryDate: string;
  /** @pattern ^[0-9]{3,4}$ */
  cvv: string;
  cardholderName: string;
}

/** Bank transfer payment method */
export interface BankPayment extends BasePayment {
  type: "bank";
  bankName: string;
  accountNumber: string;
  routingNumber: string;
}

/** Cryptocurrency payment method */
export interface CryptoPayment extends BasePayment {
  type: "crypto";
  walletAddress: string;
  network: "bitcoin" | "ethereum" | "solana";
}

/**
 * Discriminated union of payment methods.
 * The `type` field discriminates between card, bank, and crypto.
 */
export type PaymentMethodDto = CardPayment | BankPayment | CryptoPayment;

/** Payment status */
export enum PaymentStatus {
  PENDING = "pending",
  PROCESSING = "processing",
  COMPLETED = "completed",
  FAILED = "failed",
  REFUNDED = "refunded",
}

/** Payment response */
export interface PaymentResponse {
  id: string;
  status: PaymentStatus;
  amount: number;
  currency: string;
  method: PaymentMethodDto;
  createdAt: string;
  completedAt: string | null;
}

/** Refund request */
export interface RefundDto {
  /** @minimum 0.01 */
  amount?: number;
  reason: string;
}
