// Local copy of @tsgonest/types branded types for testing.
// In real projects, users would: import { tags } from "@tsgonest/types"

export type Format<F extends string> = { readonly __tsgonest_format: F };
export type MinLength<N extends number> = { readonly __tsgonest_minLength: N };
export type MaxLength<N extends number> = { readonly __tsgonest_maxLength: N };
export type Pattern<P extends string> = { readonly __tsgonest_pattern: P };
export type StartsWith<S extends string> = { readonly __tsgonest_startsWith: S };
export type EndsWith<S extends string> = { readonly __tsgonest_endsWith: S };
export type Includes<S extends string> = { readonly __tsgonest_includes: S };
export type Minimum<N extends number> = { readonly __tsgonest_minimum: N };
export type Maximum<N extends number> = { readonly __tsgonest_maximum: N };
export type ExclusiveMinimum<N extends number> = { readonly __tsgonest_exclusiveMinimum: N };
export type ExclusiveMaximum<N extends number> = { readonly __tsgonest_exclusiveMaximum: N };
export type MultipleOf<N extends number> = { readonly __tsgonest_multipleOf: N };
export type Type<T extends string> = { readonly __tsgonest_type: T };
export type MinItems<N extends number> = { readonly __tsgonest_minItems: N };
export type MaxItems<N extends number> = { readonly __tsgonest_maxItems: N };
export type UniqueItems = { readonly __tsgonest_uniqueItems: true };

// Convenience aliases
export type Email = Format<"email">;
export type Uuid = Format<"uuid">;
export type Url = Format<"url">;
export type Int = Type<"int32">;
export type Positive = ExclusiveMinimum<0>;
export type NonNegative = Minimum<0>;
