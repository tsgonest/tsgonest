import { TSGONEST_FORM_DATA_FACTORY } from './form-data-body';

/**
 * NestJS interceptor that processes multipart/form-data requests using a multer
 * instance provided via @FormDataBody().
 *
 * Usage:
 * ```ts
 * @Post()
 * @UseInterceptors(FormDataInterceptor)
 * upload(@FormDataBody(() => imageMulter()) body: UploadDto): void {}
 * ```
 *
 * The interceptor:
 * 1. Reads the multer factory from parameter metadata set by @FormDataBody()
 * 2. Creates/caches the multer instance
 * 3. Runs multer.any() to parse the multipart request
 * 4. Merges parsed files into req.body by field name
 */
export class FormDataInterceptor {
  private multerCache = new WeakMap<Function, any>();

  async intercept(context: any, next: any): Promise<any> {
    const handler = context.getHandler();
    const target = context.getClass().prototype;

    const meta = Reflect.getMetadata(TSGONEST_FORM_DATA_FACTORY, target, handler.name);
    if (!meta) {
      return next.handle();
    }

    const { factory } = meta;

    // Resolve and cache multer instance
    let multer = this.multerCache.get(handler);
    if (!multer) {
      multer = await factory();
      this.multerCache.set(handler, multer);
    }

    const req = context.switchToHttp().getRequest();
    const res = context.switchToHttp().getResponse();

    // Run multer.any() as middleware
    await new Promise<void>((resolve, reject) => {
      multer.any()(req, res, (err: any) => {
        if (err) reject(err);
        else resolve();
      });
    });

    // Merge files into body by field name, converting multer files to web-native File instances.
    // Multer produces Express.Multer.File objects (with buffer, originalname, mimetype, etc.)
    // which would fail `instanceof File` checks in generated validation code.
    // Converting to web-native File ensures type-safe validation works correctly.
    if (req.files && Array.isArray(req.files)) {
      const filesByField = new Map<string, File[]>();
      for (const file of req.files) {
        const webFile = new File([file.buffer], file.originalname, {
          type: file.mimetype,
        });
        const existing = filesByField.get(file.fieldname) || [];
        existing.push(webFile);
        filesByField.set(file.fieldname, existing);
      }
      for (const [fieldName, files] of filesByField) {
        req.body[fieldName] = files.length === 1 ? files[0] : files;
      }
    }

    return next.handle();
  }
}
