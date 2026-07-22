declare module "tsgonest" {
  export interface ValidationResult<T> {
    success: boolean;
    data?: T;
    errors?: Array<{ path: string; expected: string; received: string }>;
  }
  export function is<T>(input: unknown): input is T;
  export function validate<T>(input: unknown): ValidationResult<T>;
  export function assert<T>(input: unknown): T;
  export function stringify<T>(input: T): string;
  export function serialize<T>(input: T): string;
}
