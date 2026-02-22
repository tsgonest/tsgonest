/**
 * Configuration for tsgonest.
 */
export interface TsgonestConfig {
  /** Controller file discovery patterns. */
  controllers?: {
    /** Glob patterns for controller files to include. */
    include?: string[];
    /** Glob patterns for files to exclude. */
    exclude?: string[];
  };

  /** Code transformation settings. */
  transforms?: {
    /** Generate validation companion files. */
    validation?: boolean;
    /** Generate serialization companion files. */
    serialization?: boolean;
  };

  /** OpenAPI document generation settings. */
  openapi?: {
    /** Output path for the generated OpenAPI document. */
    output?: string;
    /** API title for the OpenAPI info section. */
    title?: string;
    /** API version for the OpenAPI info section. */
    version?: string;
    /** API description for the OpenAPI info section. */
    description?: string;
  };
}

/**
 * Type-safe config helper for tsgonest.config.ts.
 * Provides autocomplete and validation for the config object.
 *
 * @example
 * ```ts
 * import { defineConfig } from "@tsgonest/runtime";
 *
 * export default defineConfig({
 *   controllers: {
 *     include: ["src/**\/*.controller.ts"],
 *   },
 *   transforms: {
 *     validation: true,
 *     serialization: true,
 *   },
 *   openapi: {
 *     output: "dist/openapi.json",
 *   },
 * });
 * ```
 */
export function defineConfig(config: TsgonestConfig): TsgonestConfig {
  return config;
}
