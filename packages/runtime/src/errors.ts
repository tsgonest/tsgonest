/**
 * Validation error details for a single field.
 */
export interface ValidationErrorDetail {
  /** The path to the invalid field (e.g., "input.name"). */
  path: string;
  /** The expected type or constraint. */
  expected: string;
  /** The received type or value description. */
  received: string;
}

/**
 * Error thrown when validation fails.
 * Contains structured error details for each invalid field.
 */
export class TsgonestValidationError extends Error {
  public readonly errors: ValidationErrorDetail[];

  constructor(errors: ValidationErrorDetail[]) {
    const message = `Validation failed: ${errors.length} error(s)\n` +
      errors.map(e => `  - ${e.path}: expected ${e.expected}, received ${e.received}`).join('\n');
    super(message);
    this.name = 'TsgonestValidationError';
    this.errors = errors;
  }
}
