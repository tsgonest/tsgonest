/** Contact info for the OpenAPI document. */
export interface OpenAPIContact {
  name?: string;
  url?: string;
  email?: string;
}

/** License info for the OpenAPI document. */
export interface OpenAPILicense {
  name: string;
  url?: string;
}

/** Server info for the OpenAPI document. */
export interface OpenAPIServer {
  url: string;
  description?: string;
}

/** Security scheme for the OpenAPI document. */
export interface OpenAPISecurityScheme {
  type: string;
  scheme?: string;
  bearerFormat?: string;
  in?: string;
  name?: string;
  description?: string;
}

/** API versioning settings. */
export interface VersioningConfig {
  /** Versioning strategy: "URI" (default), "HEADER", "MEDIA_TYPE", "CUSTOM". */
  type: string;
  /** Default version (e.g., "1"). */
  defaultVersion?: string;
  /** URI version prefix (default: "v"). */
  prefix?: string;
}

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
    /** Glob patterns for source files to generate companions for (e.g., ["src/**\/*.dto.ts"]). */
    include?: string[];
    /** Type name patterns to exclude from codegen (e.g., "Legacy*", "SomeInternalDto"). */
    exclude?: string[];
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
    /** Contact info for the OpenAPI info section. */
    contact?: OpenAPIContact;
    /** License info for the OpenAPI info section. */
    license?: OpenAPILicense;
    /** Server list for the OpenAPI document. */
    servers?: OpenAPIServer[];
    /** Named security schemes for the OpenAPI document. */
    securitySchemes?: Record<string, OpenAPISecurityScheme>;
  };

  /** NestJS-specific settings. */
  nestjs?: {
    /** Global route prefix (e.g., "api"). */
    globalPrefix?: string;
    /** API versioning settings. */
    versioning?: VersioningConfig;
  };

  /** Entry point name without extension (default: "main"). */
  entryFile?: string;
  /** Source root directory (default: "src"). */
  sourceRoot?: string;
  /** Delete output directory before build (like --clean). */
  deleteOutDir?: boolean;
  /** Enable "rs" manual restart in dev mode. */
  manualRestart?: boolean;
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
