/**
 * @tsgonest/types — Zero-runtime branded types for type-safe validation.
 *
 * These types exist purely at compile time. They produce NO runtime code.
 * The tsgonest compiler reads phantom properties (`__tsgonest_*`) to generate validators.
 *
 * Every constraint supports two forms:
 *   - Simple:   Min<0>              — clean, no custom error
 *   - Extended: Min<{value: 0, error: "Must be positive"}>  — with per-constraint error
 *
 * @example
 *   import { Email, Min, Max, Trim, Coerce } from "@tsgonest/types";
 *
 *   interface CreateUserDto {
 *     email: string & Email;
 *     name: string & Trim & Min<1> & Max<255>;
 *     age: number & Min<0> & Max<150>;
 *   }
 *
 *   // With per-constraint errors:
 *   interface StrictDto {
 *     email: string & Format<{type: "email", error: "Must be a valid email"}>;
 *     age: number & Min<{value: 0, error: "Age cannot be negative"}>;
 *   }
 *
 *   // Query params with coercion:
 *   interface QueryDto {
 *     page: number & Coerce;
 *     active: boolean & Coerce;
 *   }
 */

// ═══════════════════════════════════════════════════════════════════════════════
// Internal helpers (not exported)
// ═══════════════════════════════════════════════════════════════════════════════

/** Extract the value from a dual-form number constraint. */
type NumVal<N extends number | { value: number; error?: string }> =
  N extends { value: infer V } ? V : N;

/** Extract the value from a dual-form string constraint. */
type StrVal<S extends string | { value: string; error?: string }> =
  S extends { value: infer V } ? V : S;

/** Extract the type from a dual-form type constraint (Format, Type). */
type TypeVal<T, Base> =
  T extends { type: infer V } ? V : T;

/** Conditionally add a _error phantom property. */
type WithErr<Prefix extends string, C> =
  C extends { error: infer E extends string }
    ? { readonly [K in `${Prefix}_error`]: E }
    : {};

// ═══════════════════════════════════════════════════════════════════════════════
// String Format
// ═══════════════════════════════════════════════════════════════════════════════

/** All supported string format values. */
export type FormatValue =
  | "email" | "idn-email"
  | "url" | "uri" | "uri-reference" | "uri-template" | "iri" | "iri-reference"
  | "uuid"
  | "ipv4" | "ipv6"
  | "hostname" | "idn-hostname"
  | "date-time" | "date" | "time" | "duration"
  | "json-pointer" | "relative-json-pointer"
  | "byte" | "password" | "regex"
  | "nanoid" | "cuid" | "cuid2" | "ulid" | "jwt"
  | "base64url" | "hex" | "mac"
  | "cidrv4" | "cidrv6" | "emoji";

/**
 * Validate a string matches a specific format.
 *
 * @example
 *   Format<"email">
 *   Format<{type: "email", error: "Must be a valid email"}>
 */
export type Format<F extends FormatValue | { type: FormatValue; error?: string }> = {
  readonly __tsgonest_format: TypeVal<F, FormatValue>;
} & WithErr<"__tsgonest_format", F>;

// ═══════════════════════════════════════════════════════════════════════════════
// String Constraints
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Minimum string length.
 * @example MinLength<1>  or  MinLength<{value: 1, error: "Cannot be empty"}>
 */
export type MinLength<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_minLength: NumVal<N>;
} & WithErr<"__tsgonest_minLength", N>;

/**
 * Maximum string length.
 * @example MaxLength<255>  or  MaxLength<{value: 255, error: "Too long"}>
 */
export type MaxLength<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_maxLength: NumVal<N>;
} & WithErr<"__tsgonest_maxLength", N>;

/**
 * Regex pattern constraint.
 * @example Pattern<"^[a-z]+$">  or  Pattern<{value: "^[a-z]+$", error: "Letters only"}>
 */
export type Pattern<P extends string | { value: string; error?: string }> = {
  readonly __tsgonest_pattern: StrVal<P>;
} & WithErr<"__tsgonest_pattern", P>;

/**
 * String must start with prefix.
 * @example StartsWith<"https://">
 */
export type StartsWith<S extends string | { value: string; error?: string }> = {
  readonly __tsgonest_startsWith: StrVal<S>;
} & WithErr<"__tsgonest_startsWith", S>;

/**
 * String must end with suffix.
 * @example EndsWith<".json">
 */
export type EndsWith<S extends string | { value: string; error?: string }> = {
  readonly __tsgonest_endsWith: StrVal<S>;
} & WithErr<"__tsgonest_endsWith", S>;

/**
 * String must contain substring.
 * @example Includes<"@">
 */
export type Includes<S extends string | { value: string; error?: string }> = {
  readonly __tsgonest_includes: StrVal<S>;
} & WithErr<"__tsgonest_includes", S>;

// ═══════════════════════════════════════════════════════════════════════════════
// Numeric Constraints
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Minimum value (inclusive): value >= N.
 * @example Minimum<0>  or  Min<0>  or  Min<{value: 0, error: "Must be non-negative"}>
 */
export type Minimum<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_minimum: NumVal<N>;
} & WithErr<"__tsgonest_minimum", N>;

/**
 * Maximum value (inclusive): value <= N.
 * @example Maximum<100>  or  Max<100>
 */
export type Maximum<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_maximum: NumVal<N>;
} & WithErr<"__tsgonest_maximum", N>;

/**
 * Exclusive minimum: value > N.
 * @example ExclusiveMinimum<0>  or  Gt<0>
 */
export type ExclusiveMinimum<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_exclusiveMinimum: NumVal<N>;
} & WithErr<"__tsgonest_exclusiveMinimum", N>;

/**
 * Exclusive maximum: value < N.
 * @example ExclusiveMaximum<100>  or  Lt<100>
 */
export type ExclusiveMaximum<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_exclusiveMaximum: NumVal<N>;
} & WithErr<"__tsgonest_exclusiveMaximum", N>;

/**
 * Value must be a multiple of N.
 * @example MultipleOf<2>  or  Step<0.01>
 */
export type MultipleOf<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_multipleOf: NumVal<N>;
} & WithErr<"__tsgonest_multipleOf", N>;

// ═══════════════════════════════════════════════════════════════════════════════
// Numeric Type Constraints
// ═══════════════════════════════════════════════════════════════════════════════

/** Valid numeric type values. */
export type NumericTypeValue =
  | "int32" | "uint32" | "int64" | "uint64" | "float" | "double";

/**
 * Constrain number to a specific numeric type.
 * @example Type<"int32">  or  Type<{type: "int32", error: "Must be integer"}>
 */
export type Type<T extends NumericTypeValue | { type: NumericTypeValue; error?: string }> = {
  readonly __tsgonest_type: TypeVal<T, NumericTypeValue>;
} & WithErr<"__tsgonest_type", T>;

// ═══════════════════════════════════════════════════════════════════════════════
// Array Constraints
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Minimum array length.
 * @example MinItems<1>
 */
export type MinItems<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_minItems: NumVal<N>;
} & WithErr<"__tsgonest_minItems", N>;

/**
 * Maximum array length.
 * @example MaxItems<100>
 */
export type MaxItems<N extends number | { value: number; error?: string }> = {
  readonly __tsgonest_maxItems: NumVal<N>;
} & WithErr<"__tsgonest_maxItems", N>;

/**
 * Array items must be unique.
 * @example UniqueItems  or  Unique  or  Unique<{error: "No duplicates"}>
 */
export type UniqueItems<C extends { error?: string } = {}> = {
  readonly __tsgonest_uniqueItems: true;
} & WithErr<"__tsgonest_uniqueItems", C>;

// ═══════════════════════════════════════════════════════════════════════════════
// String Case Validation
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * String must be all uppercase.
 * @example Uppercase  or  Uppercase<{error: "Must be uppercase"}>
 */
export type Uppercase<C extends { error?: string } = {}> = {
  readonly __tsgonest_uppercase: true;
} & WithErr<"__tsgonest_uppercase", C>;

/**
 * String must be all lowercase.
 * @example Lowercase  or  Lowercase<{error: "Must be lowercase"}>
 */
export type Lowercase<C extends { error?: string } = {}> = {
  readonly __tsgonest_lowercase: true;
} & WithErr<"__tsgonest_lowercase", C>;

// ═══════════════════════════════════════════════════════════════════════════════
// Transforms (applied before validation, never fail)
// ═══════════════════════════════════════════════════════════════════════════════

/** Trim whitespace before validation. */
export type Trim = { readonly __tsgonest_transform_trim: true };

/** Convert to lowercase before validation. */
export type ToLowerCase = { readonly __tsgonest_transform_toLowerCase: true };

/** Convert to uppercase before validation. */
export type ToUpperCase = { readonly __tsgonest_transform_toUpperCase: true };

// ═══════════════════════════════════════════════════════════════════════════════
// Coercion
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Coerce string inputs to the declared type before validation.
 * - "123" → 123 (string to number)
 * - "true"/"false" → true/false (string to boolean)
 *
 * @example
 *   page: number & Coerce
 *   active: boolean & Coerce
 */
export type Coerce = { readonly __tsgonest_coerce: true };

// ═══════════════════════════════════════════════════════════════════════════════
// Custom Validators (function reference)
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Validate using a custom function. The function must be a predicate:
 * `(value: T) => boolean`. tsgonest resolves the function's source file
 * and emits an import + call in the generated validator.
 *
 * @example
 *   import { isValidCard } from "./validators/credit-card";
 *
 *   interface PaymentDto {
 *     card: string & Validate<typeof isValidCard>;
 *     card: string & Validate<{fn: typeof isValidCard, error: "Invalid card"}>;
 *   }
 */
export type Validate<
  F extends ((...args: any[]) => boolean) | { fn: (...args: any[]) => boolean; error?: string }
> = {
  readonly __tsgonest_validate: F extends { fn: infer Fn } ? Fn : F;
} & WithErr<"__tsgonest_validate", F>;

// ═══════════════════════════════════════════════════════════════════════════════
// Meta Types
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Global error message — applies to all validation failures on this property.
 * Per-constraint errors (via `error` field) take precedence.
 * @example string & Format<"email"> & Error<"Invalid email">
 */
export type Error<M extends string> = { readonly __tsgonest_error: M };

/**
 * Default value for optional properties. Assigned when value is undefined.
 * @example theme?: string & Default<"light">
 */
export type Default<V extends string | number | boolean> = {
  readonly __tsgonest_default: V;
};

// ═══════════════════════════════════════════════════════════════════════════════
// Compound Constraints
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * Exact length (sets both MinLength and MaxLength).
 * @example Length<2>  or  Length<{value: 2, error: "Must be exactly 2 chars"}>
 */
export type Length<N extends number | { value: number; error?: string }> =
  MinLength<N> & MaxLength<N>;

/**
 * Numeric range (inclusive). Sets Minimum and Maximum.
 * @example Range<{min: 0, max: 100}>  or  Range<{min: 0, max: 100, error: "Out of range"}>
 */
export type Range<R extends { min: number; max: number; error?: string }> =
  Minimum<R extends { error: string } ? { value: R["min"]; error: R["error"] } : R["min"]>
  & Maximum<R extends { error: string } ? { value: R["max"]; error: R["error"] } : R["max"]>;

/**
 * String length range. Sets MinLength and MaxLength.
 * @example Between<{min: 1, max: 255}>  or  Between<{min: 1, max: 255, error: "Bad length"}>
 */
export type Between<R extends { min: number; max: number; error?: string }> =
  MinLength<R extends { error: string } ? { value: R["min"]; error: R["error"] } : R["min"]>
  & MaxLength<R extends { error: string } ? { value: R["max"]; error: R["error"] } : R["max"]>;

// ═══════════════════════════════════════════════════════════════════════════════
// Short Aliases (Zod-style)
// ═══════════════════════════════════════════════════════════════════════════════

/** Alias for Minimum. `Min<0>` = `Minimum<0>` */
export type Min<N extends number | { value: number; error?: string }> = Minimum<N>;

/** Alias for Maximum. `Max<100>` = `Maximum<100>` */
export type Max<N extends number | { value: number; error?: string }> = Maximum<N>;

/** Alias for ExclusiveMinimum. `Gt<0>` = "greater than 0" */
export type Gt<N extends number | { value: number; error?: string }> = ExclusiveMinimum<N>;

/** Alias for ExclusiveMaximum. `Lt<100>` = "less than 100" */
export type Lt<N extends number | { value: number; error?: string }> = ExclusiveMaximum<N>;

/** Alias for Minimum + ExclusiveMinimum combo. `Gte<0>` = `Min<0>` */
export type Gte<N extends number | { value: number; error?: string }> = Minimum<N>;

/** Alias for Maximum + ExclusiveMaximum combo. `Lte<100>` = `Max<100>` */
export type Lte<N extends number | { value: number; error?: string }> = Maximum<N>;

/** Alias for MultipleOf. `Step<0.01>` */
export type Step<N extends number | { value: number; error?: string }> = MultipleOf<N>;

/** Alias for UniqueItems. `Unique` */
export type Unique<C extends { error?: string } = {}> = UniqueItems<C>;

// ═══════════════════════════════════════════════════════════════════════════════
// Format Aliases (direct exports, no namespace needed)
// ═══════════════════════════════════════════════════════════════════════════════

export type Email = Format<"email">;
export type Uuid = Format<"uuid">;
export type Url = Format<"url">;
export type Uri = Format<"uri">;
export type IPv4 = Format<"ipv4">;
export type IPv6 = Format<"ipv6">;
export type DateTime = Format<"date-time">;
export type DateOnly = Format<"date">;
export type Time = Format<"time">;
export type Duration = Format<"duration">;
export type Jwt = Format<"jwt">;
export type Ulid = Format<"ulid">;
export type Cuid = Format<"cuid">;
export type Cuid2 = Format<"cuid2">;
export type NanoId = Format<"nanoid">;

// ═══════════════════════════════════════════════════════════════════════════════
// Numeric Aliases
// ═══════════════════════════════════════════════════════════════════════════════

/** number & Gt<0> */
export type Positive = ExclusiveMinimum<0>;

/** number & Lt<0> */
export type Negative = ExclusiveMaximum<0>;

/** number & Min<0> */
export type NonNegative = Minimum<0>;

/** number & Max<0> */
export type NonPositive = Maximum<0>;

/** number & Type<"int32"> */
export type Int = Type<"int32">;

/** number & Type<"int64"> (JS safe integer range) */
export type SafeInt = Type<"int64">;

/** number & Type<"float"> (finite, no Infinity/NaN) */
export type Finite = Type<"float">;

/** number & Type<"uint32"> */
export type Uint = Type<"uint32">;

/** number & Type<"double"> */
export type Double = Type<"double">;
