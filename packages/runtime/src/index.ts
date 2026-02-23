// Validation
export { TsgonestValidationPipe } from './validation-pipe';
export type { ValidationPipeOptions } from './validation-pipe';

// Serialization (legacy — uses Reflect.getMetadata)
export { TsgonestSerializationInterceptor } from './serialization';
export type { SerializationInterceptorOptions } from './serialization';

// Fast Serialization (recommended — uses route map, no Reflect needed)
export { TsgonestFastInterceptor } from './fast-interceptor';
export type { FastInterceptorOptions } from './fast-interceptor';

// Errors
export { TsgonestValidationError } from './errors';
export type { ValidationErrorDetail } from './errors';

// Discovery
export { CompanionDiscovery } from './discovery';
export type { TsgonestManifest, CompanionEntry, RouteMapping } from './discovery';

// Config
export { defineConfig } from './config';
export type { TsgonestConfig } from './config';
