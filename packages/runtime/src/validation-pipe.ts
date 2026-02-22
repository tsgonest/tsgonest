import type { PipeTransform, ArgumentMetadata, Type } from '@nestjs/common';
import { CompanionDiscovery } from './discovery';
import { TsgonestValidationError, ValidationErrorDetail } from './errors';

/**
 * Options for configuring the TsgonestValidationPipe.
 */
export interface ValidationPipeOptions {
  /**
   * Path to the dist directory containing the tsgonest manifest.
   * Defaults to the current working directory's dist/ folder.
   */
  distDir?: string;

  /**
   * Pre-loaded CompanionDiscovery instance.
   * If provided, distDir is ignored.
   */
  discovery?: CompanionDiscovery;

  /**
   * Whether to throw an error if validation fails.
   * Defaults to true. When false, returns null for invalid input.
   */
  throwOnError?: boolean;

  /**
   * HTTP status code to use when throwing validation errors.
   * Defaults to 400 (Bad Request).
   */
  errorHttpStatusCode?: number;
}

/**
 * A NestJS pipe that validates incoming request data using
 * tsgonest-generated companion validation functions.
 *
 * Usage:
 * ```ts
 * import { TsgonestValidationPipe } from '@tsgonest/runtime';
 *
 * // In main.ts:
 * app.useGlobalPipes(new TsgonestValidationPipe({ distDir: 'dist' }));
 *
 * // Or with a pre-loaded discovery:
 * const discovery = new CompanionDiscovery();
 * discovery.loadManifest('dist');
 * app.useGlobalPipes(new TsgonestValidationPipe({ discovery }));
 * ```
 */
export class TsgonestValidationPipe implements PipeTransform {
  private discovery: CompanionDiscovery;
  private throwOnError: boolean;
  private errorHttpStatusCode: number;
  private initialized = false;
  private distDir: string | undefined;

  constructor(options: ValidationPipeOptions = {}) {
    this.throwOnError = options.throwOnError ?? true;
    this.errorHttpStatusCode = options.errorHttpStatusCode ?? 400;

    if (options.discovery) {
      this.discovery = options.discovery;
      this.initialized = true;
    } else {
      this.discovery = new CompanionDiscovery();
      this.distDir = options.distDir;
    }
  }

  /**
   * Ensure the manifest is loaded.
   */
  private ensureInitialized(): void {
    if (this.initialized) return;
    this.initialized = true;

    const distDir = this.distDir || 'dist';
    this.discovery.loadManifest(distDir);
  }

  /**
   * Transform (validate) the incoming value.
   * If a companion validator exists for the metatype, it is called.
   * Otherwise, the value is returned unchanged.
   */
  transform(value: unknown, metadata: ArgumentMetadata): unknown {
    this.ensureInitialized();

    // Only validate body, query, and param types
    if (!metadata.metatype) {
      return value;
    }

    const typeName = this.getTypeName(metadata.metatype);
    if (!typeName) {
      return value;
    }

    // Look up the validator (assert function) for this type
    const validator = this.discovery.getValidator(typeName);
    if (!validator) {
      // No companion found — pass through
      return value;
    }

    try {
      return validator(value);
    } catch (err: unknown) {
      if (!this.throwOnError) {
        return null;
      }

      // Re-throw TsgonestValidationError as a structured HTTP error
      if (err instanceof TsgonestValidationError) {
        throw this.createHttpException(err.errors);
      }

      // For errors from the generated assert functions that throw plain Error
      if (err instanceof Error && err.message) {
        // The generated assert functions throw errors with structured messages
        // Parse them if possible, otherwise wrap
        const details = this.parseAssertError(err.message);
        if (details.length > 0) {
          throw this.createHttpException(details);
        }
      }

      throw err;
    }
  }

  /**
   * Extract the type name from a NestJS metatype.
   * Filters out built-in types (String, Number, Boolean, etc.).
   */
  private getTypeName(metatype: Type<unknown>): string | null {
    const builtInTypes: Function[] = [String, Number, Boolean, Array, Object];
    if (builtInTypes.includes(metatype)) {
      return null;
    }
    return metatype.name || null;
  }

  /**
   * Parse error messages from generated assert functions.
   * The generated code throws errors like:
   *   "Validation failed: input.name expected string, received number"
   */
  private parseAssertError(message: string): ValidationErrorDetail[] {
    const details: ValidationErrorDetail[] = [];
    const lines = message.split('\n');

    for (const line of lines) {
      // Match patterns like "  - input.name: expected string, received number"
      const match = line.match(/^\s*-?\s*(.+?):\s*expected\s+(.+?),\s*received\s+(.+)$/);
      if (match) {
        details.push({
          path: match[1].trim(),
          expected: match[2].trim(),
          received: match[3].trim(),
        });
      }
    }

    return details;
  }

  /**
   * Create an HTTP exception for validation errors.
   * Uses a dynamic import approach to avoid hard NestJS dependency at import time.
   */
  private createHttpException(errors: ValidationErrorDetail[]): Error {
    // Try to create a NestJS HttpException
    try {
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const { HttpException } = require('@nestjs/common');
      return new HttpException(
        {
          statusCode: this.errorHttpStatusCode,
          message: 'Validation failed',
          errors: errors.map(e => ({
            property: e.path,
            constraints: { tsgonest: `expected ${e.expected}, received ${e.received}` },
          })),
        },
        this.errorHttpStatusCode,
      );
    } catch {
      // Fallback if @nestjs/common not available
      const err = new TsgonestValidationError(errors);
      (err as any).status = this.errorHttpStatusCode;
      return err;
    }
  }
}
