import { TSGONEST_FORM_DATA_FACTORY } from './form-data-body';

/**
 * NestJS interceptor that processes multipart/form-data requests.
 *
 * Platform-aware: automatically detects the runtime environment and uses
 * the appropriate parsing strategy:
 *
 * - **Bun**: The BunAdapter's body parser already parsed multipart data via
 *   `Request.formData()`, producing web-native `File` instances. The interceptor
 *   detects this and skips multer entirely — zero dependencies needed.
 *
 * - **Fastify**: Uses the raw `IncomingMessage` stream with multer, since
 *   Fastify does not consume multipart bodies by default.
 *
 * - **Express**: Standard multer middleware path (req, res, next).
 *
 * Usage:
 * ```ts
 * // Express/Fastify — provide a multer factory:
 * @Post()
 * @UseInterceptors(FormDataInterceptor)
 * upload(@FormDataBody(() => imageMulter()) body: UploadDto): void {}
 *
 * // Bun — no multer needed:
 * @Post()
 * @UseInterceptors(FormDataInterceptor)
 * upload(@FormDataBody() body: UploadDto): void {}
 * ```
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

    const req = context.switchToHttp().getRequest();

    // If body already contains File instances (Bun adapter pre-parsed), skip multer.
    if (req.body && hasFileInstances(req.body)) {
      return next.handle();
    }

    const { factory } = meta;

    // No multer factory provided (Bun-only path) — nothing to do
    if (!factory) {
      return next.handle();
    }

    // Resolve and cache multer instance
    let multer = this.multerCache.get(handler);
    if (!multer) {
      multer = await factory();
      this.multerCache.set(handler, multer);
    }

    const res = context.switchToHttp().getResponse();

    // Detect Fastify: req.raw is an IncomingMessage (readable stream).
    // Fastify does not consume multipart bodies by default, so the raw
    // stream is still available for multer/busboy to read from.
    const isFastify = req.raw && typeof req.raw.pipe === 'function'
      && !(req.raw instanceof (globalThis as any).Request);

    if (isFastify) {
      const rawReq = req.raw;
      await new Promise<void>((resolve, reject) => {
        multer.any()(rawReq, res.raw || res, (err: any) => {
          if (err) reject(err);
          else resolve();
        });
      });
      // Transfer parsed body from raw IncomingMessage to Fastify request
      if ((rawReq as any).body) {
        req.body = (rawReq as any).body;
      }
      // Convert multer files to web-native File instances
      mergeMulterFiles(req, (rawReq as any).files);
    } else {
      // Express path: standard multer middleware
      await new Promise<void>((resolve, reject) => {
        multer.any()(req, res, (err: any) => {
          if (err) reject(err);
          else resolve();
        });
      });
      // Convert multer files to web-native File instances
      mergeMulterFiles(req, req.files);
    }

    return next.handle();
  }
}

/**
 * Convert multer's Express.Multer.File objects to web-native File instances
 * and merge them into req.body by field name.
 *
 * Multer produces objects with {buffer, originalname, mimetype, fieldname, ...}
 * which would fail `instanceof File` checks in generated validation code.
 * Converting to web-native File ensures type-safe validation works correctly.
 */
function mergeMulterFiles(req: any, files: any[] | undefined): void {
  if (!files || !Array.isArray(files) || files.length === 0) return;

  if (!req.body) req.body = {};

  const filesByField = new Map<string, File[]>();
  for (const file of files) {
    const webFile = new File([file.buffer], file.originalname, {
      type: file.mimetype,
    });
    const existing = filesByField.get(file.fieldname) || [];
    existing.push(webFile);
    filesByField.set(file.fieldname, existing);
  }
  for (const [fieldName, fieldFiles] of filesByField) {
    req.body[fieldName] = fieldFiles.length === 1 ? fieldFiles[0] : fieldFiles;
  }
}

/**
 * Check if a body object contains any File instances (indicating the adapter
 * already parsed multipart data — e.g., Bun's native formData() parsing).
 */
function hasFileInstances(body: any): boolean {
  if (typeof body !== 'object' || body === null) return false;
  for (const value of Object.values(body)) {
    if (value instanceof File) return true;
    if (Array.isArray(value) && value.some(v => v instanceof File)) return true;
  }
  return false;
}
