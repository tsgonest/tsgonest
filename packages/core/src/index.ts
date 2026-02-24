export interface ValidationResult<T> {
  success: boolean;
  data?: T;
  errors?: Array<{ path: string; expected: string; received: string }>;
}

// Marker functions — identity/no-op at runtime, replaced by tsgonest at compile time.
// Code works without tsgonest compilation (just slower, no validation).
export function is<T>(input: unknown): input is T {
  return true;
}
export function validate<T>(input: unknown): ValidationResult<T> {
  return { success: true, data: input as T };
}
export function assert<T>(input: unknown): T {
  return input as T;
}
export function stringify<T>(input: T): string {
  return JSON.stringify(input);
}
export function serialize<T>(input: T): string {
  return JSON.stringify(input);
}
