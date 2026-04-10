import 'reflect-metadata';
import { Body } from '@nestjs/common';

/**
 * Metadata key used by @FormDataBody() to store the multer factory function.
 * The FormDataInterceptor reads this to resolve the multer instance.
 */
export const TSGONEST_FORM_DATA_FACTORY = 'TSGONEST_FORM_DATA_FACTORY';

/**
 * Parameter decorator for multipart/form-data endpoints.
 *
 * Drop-in replacement for Nestia's `@TypedFormData.Body(() => multerFactory())`.
 * Stores the multer factory in metadata so `FormDataInterceptor` can resolve it,
 * and composes `@Body()` so NestJS injects the parsed body into the parameter
 * after the interceptor populates `req.body`.
 *
 * @example
 * ```ts
 * import { FormDataBody, FormDataInterceptor } from '@tsgonest/runtime';
 *
 * @Post()
 * @UseInterceptors(FormDataInterceptor)
 * upload(@FormDataBody(() => imageMulter()) body: UploadDto): void {}
 * ```
 */
export function FormDataBody(factory?: () => any | Promise<any>): ParameterDecorator {
  const bodyDecorator = Body();
  return (target, propertyKey, parameterIndex) => {
    bodyDecorator(target, propertyKey as string | symbol, parameterIndex);
    Reflect.defineMetadata(
      TSGONEST_FORM_DATA_FACTORY,
      { factory, parameterIndex },
      target,
      propertyKey as string | symbol,
    );
  };
}
