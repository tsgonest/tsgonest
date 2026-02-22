/**
 * @tsgonest/types — Zero-runtime branded types for type-safe validation constraints.
 *
 * This package provides phantom branded types that the tsgonest compiler
 * inspects at build time to generate validation and serialization code.
 * No runtime code is emitted — these types exist purely for type safety
 * and IDE autocomplete.
 *
 * @example
 * ```ts
 * import { tags } from "@tsgonest/types";
 *
 * interface CreateUserDto {
 *   email: string & tags.Format<"email">;
 *   name: string & tags.MinLength<1> & tags.MaxLength<255>;
 *   age: number & tags.Minimum<0> & tags.Maximum<150>;
 *   tags: string[] & tags.MinItems<1> & tags.UniqueItems;
 * }
 * ```
 *
 * Convenience aliases for common patterns:
 * ```ts
 * import { tags } from "@tsgonest/types";
 *
 * interface User {
 *   email: string & tags.Email;           // same as tags.Format<"email">
 *   id: string & tags.Uuid;               // same as tags.Format<"uuid">
 *   website: string & tags.Url;           // same as tags.Format<"url">
 *   age: number & tags.Int & tags.NonNegative; // int32 & >= 0
 * }
 * ```
 */
export * as tags from "./tags.js";
