import 'reflect-metadata';

/**
 * Metadata key used by @FormDataBody() to store the multer factory function.
 * The FormDataInterceptor reads this to resolve the multer instance.
 */
export const TSGONEST_FORM_DATA_FACTORY = 'TSGONEST_FORM_DATA_FACTORY';

/**
 * Parameter decorator for multipart/form-data endpoints.
 *
 * Drop-in replacement for Nestia's `@TypedFormData.Body(() => multerFactory())`.
 * Stores the multer factory in metadata so `FormDataInterceptor` can parse the request.
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
export function FormDataBody(factory: () => any | Promise<any>): ParameterDecorator {
  return (target, propertyKey, parameterIndex) => {
    Reflect.defineMetadata(
      TSGONEST_FORM_DATA_FACTORY,
      { factory, parameterIndex },
      target,
      propertyKey as string | symbol,
    );
  };
}
