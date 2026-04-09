import { TSGONEST_FORM_DATA_FACTORY } from './form-data-body';

/**
 * NestJS interceptor that processes multipart/form-data requests.
 *
 * Platform-aware: automatically detects the runtime environment and uses
 * the appropriate parsing strategy:
 *
 * - **Bun**: The BunAdapter's body parser already parsed multipart data via
 *   `Request.formData()`, producing web-native `File` instances. The interceptor
 *   detects this and skips processing entirely — zero dependencies needed.
 *
 * - **Node 20+** (Express/Fastify): Uses the web-native `Request.formData()`
 *   API to parse multipart data directly. This bypasses multer/busboy entirely
 *   and is immune to stream-timing issues on Node 24+. Multer is not required.
 *
 * - **Node < 20** (Express/Fastify fallback): Uses multer middleware to parse
 *   multipart data, then converts multer files to web-native `File` instances.
 *
 * Usage:
 * ```ts
 * // Express/Fastify — provide a multer factory (used as fallback on Node < 20):
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

    // If body already contains File instances (Bun adapter pre-parsed), skip.
    if (req.body && hasFileInstances(req.body)) {
      return next.handle();
    }

    const { factory } = meta;

    // No multer factory provided (Bun-only path) — nothing to do
    if (!factory && !canParseNative()) {
      return next.handle();
    }

    const res = context.switchToHttp().getResponse();

    // Detect Fastify: req.raw is an IncomingMessage (readable stream).
    // Fastify does not consume multipart bodies by default, so the raw
    // stream is still available for parsing.
    const isFastify = req.raw && typeof req.raw.pipe === 'function'
      && !(req.raw instanceof (globalThis as any).Request);

    // The IncomingMessage stream to read from
    const stream = isFastify ? req.raw : req;

    // Primary path: web-native FormData parsing (Node 20+)
    // Uses Request.formData() — immune to multer/busboy stream issues on Node 24+.
    // No readableEnded guard: on Node 24, the stream may be marked ended before
    // the interceptor runs (undici buffers small bodies eagerly), but the data is
    // still available in the internal buffer or via req.rawBody.
    if (canParseNative() && stream && typeof stream.pipe === 'function') {
      const parsed = await parseFormDataNative(req, stream);
      if (parsed) return next.handle();
    }

    // Fallback: multer (Node < 20 or web APIs unavailable)
    if (!factory) {
      return next.handle();
    }

    // Resolve and cache multer instance
    let multer = this.multerCache.get(handler);
    if (!multer) {
      multer = await factory();
      this.multerCache.set(handler, multer);
    }

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
 * Check if web-native FormData parsing is available.
 * Requires Node 20+ with globalThis.Request and globalThis.FormData.
 */
let _canParseNative: boolean | undefined;
function canParseNative(): boolean {
  if (_canParseNative !== undefined) return _canParseNative;
  try {
    _canParseNative =
      typeof globalThis.Request === 'function' &&
      typeof globalThis.FormData === 'function' &&
      typeof globalThis.File === 'function' &&
      typeof require('node:stream').Readable.toWeb === 'function';
  } catch {
    _canParseNative = false;
  }
  return _canParseNative;
}

/**
 * Parse multipart/form-data using web-native Request.formData() API.
 *
 * Converts the Node.js IncomingMessage to a web Request, parses FormData,
 * and populates req.body with fields and File instances — same result as
 * multer + mergeMulterFiles, but without busboy stream dependencies.
 *
 * Handles ended streams (Node 24+): when the stream is already ended (undici
 * buffers small bodies eagerly, or body-parser middleware consumed it), falls
 * back to req.rawBody or drains the internal buffer.
 *
 * Returns true if parsing succeeded, false if no data was available (caller
 * should fall through to the multer path).
 */
async function parseFormDataNative(req: any, stream: any): Promise<boolean> {
  const { Readable } = require('node:stream') as typeof import('node:stream');

  const headers = new Headers();
  const rawHeaders = stream.headers || req.headers;
  for (const [key, val] of Object.entries(rawHeaders as Record<string, string | string[]>)) {
    if (typeof val === 'string') headers.set(key, val);
    else if (Array.isArray(val)) for (const v of val) headers.append(key, v);
  }

  // Determine the request body source.
  // On Node 24+, the stream may be marked ended before the interceptor runs
  // because undici eagerly buffers small request bodies. The data is still
  // available in the stream's internal buffer or via req.rawBody.
  let body: BodyInit;
  const requestInit: RequestInit & Record<string, unknown> = {
    method: req.method || 'POST',
    headers,
  };

  if (!stream.readableEnded) {
    body = Readable.toWeb(stream) as ReadableStream;
    requestInit.duplex = 'half';
  } else {
    // Stream ended — try rawBody (set by body-parser verify callback), then
    // drain any remaining data from the internal buffer.
    const rawBody = req.rawBody || stream.rawBody;
    if (rawBody) {
      body = rawBody;
    } else {
      const chunks: Buffer[] = [];
      let chunk: Buffer | null;
      while ((chunk = stream.read()) !== null) {
        chunks.push(chunk);
      }
      if (chunks.length === 0) return false;
      body = Buffer.concat(chunks);
    }
  }

  requestInit.body = body;

  const webRequest = new Request(`http://localhost${req.url || '/'}`, requestInit);
  const formData = await webRequest.formData();

  if (!req.body) req.body = {};

  for (const [key, value] of formData.entries()) {
    if (value instanceof File) {
      const existing = req.body[key];
      if (existing instanceof File) {
        req.body[key] = [existing, value];
      } else if (Array.isArray(existing)) {
        existing.push(value);
      } else {
        req.body[key] = value;
      }
    } else {
      // String field
      req.body[key] = value;
    }
  }

  return true;
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
