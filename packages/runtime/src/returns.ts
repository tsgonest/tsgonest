/**
 * Options for the @Returns decorator.
 */
export interface ReturnsOptions {
  /**
   * Override the response content type in OpenAPI.
   * Defaults to `application/json`.
   *
   * @example 'application/pdf'
   * @example 'text/csv'
   */
  contentType?: string;

  /**
   * Human-readable description for the OpenAPI response.
   *
   * @example 'PDF report document'
   */
  description?: string;

  /**
   * Override the HTTP status code for the success response in OpenAPI.
   * When set, takes precedence over `@HttpCode()`.
   */
  status?: number;
}

/**
 * Declares the response type for a controller method that uses `@Res()`.
 *
 * This is a **compile-time only** decorator — it has no effect at runtime.
 * tsgonest's static analyzer reads the type parameter `<T>` to generate
 * the correct OpenAPI response schema for routes that bypass NestJS's
 * automatic response serialization (i.e., routes using `@Res()`).
 *
 * Without `@Returns`, routes using `@Res()` produce an empty (void) response
 * in OpenAPI and emit a warning.
 *
 * @example
 * ```ts
 * import { Returns } from '@tsgonest/runtime';
 *
 * // Typed JSON response via raw response object
 * @Returns<UserDto>()
 * @Get(':id')
 * async getUser(@Param('id') id: string, @Res() res: FastifyReply) {
 *   const user = await this.userService.findOne(id);
 *   res.send(user);
 * }
 *
 * // Binary response with custom content type
 * @Returns<Uint8Array>({ contentType: 'application/pdf', description: 'Invoice PDF' })
 * @Get(':id/invoice')
 * async getInvoice(@Param('id') id: string, @Res() res: FastifyReply) {
 *   const pdf = await this.invoiceService.generatePdf(id);
 *   res.header('Content-Type', 'application/pdf').send(pdf);
 * }
 * ```
 */
export function Returns<T>(
  options?: ReturnsOptions,
): MethodDecorator {
  // No-op at runtime. tsgonest reads the type parameter <T> via static analysis.
  return (_target, _propertyKey, descriptor) => descriptor;
}
