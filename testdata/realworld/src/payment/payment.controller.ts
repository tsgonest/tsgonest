import {
  Controller,
  Get,
  Post,
  Param,
  Body,
  Query,
  HttpCode,
} from "../common/decorators";
import type {
  PaymentMethodDto,
  PaymentResponse,
  RefundDto,
} from "./payment.dto";
import type { PaginatedResponse, PaginationQuery } from "../common/types";

/**
 * Payment processing controller
 * @tag Payments
 */
@Controller("payments")
export class PaymentController {
  /**
   * Process a payment using card, bank, or crypto
   * @summary Process payment
   */
  @Post()
  async processPayment(
    @Body() body: PaymentMethodDto
  ): Promise<PaymentResponse> {
    return {} as PaymentResponse;
  }

  /**
   * Get payment by ID
   * @summary Get payment
   */
  @Get(":id")
  async getPayment(@Param("id") id: string): Promise<PaymentResponse> {
    return {} as PaymentResponse;
  }

  /**
   * List payments with pagination
   * @summary List payments
   */
  @Get()
  async listPayments(
    @Query() query: PaginationQuery
  ): Promise<PaginatedResponse<PaymentResponse>> {
    return {} as PaginatedResponse<PaymentResponse>;
  }

  /**
   * Refund a payment
   * @summary Refund payment
   */
  @Post(":id/refund")
  @HttpCode(200)
  async refundPayment(
    @Param("id") id: string,
    @Body() body: RefundDto
  ): Promise<PaymentResponse> {
    return {} as PaymentResponse;
  }
}
