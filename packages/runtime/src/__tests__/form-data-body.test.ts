import { describe, it, expect } from "vitest";
import "reflect-metadata";
import { Readable } from "node:stream";
import { FormDataBody, TSGONEST_FORM_DATA_FACTORY } from "../form-data-body";
import { FormDataInterceptor } from "../form-data-interceptor";

describe("FormDataBody", () => {
  it("stores factory in metadata", () => {
    const factory = () => ({ any: () => {} });

    class TestController {
      upload(
        @FormDataBody(factory) body: any,
      ) {}
    }

    const meta = Reflect.getMetadata(
      TSGONEST_FORM_DATA_FACTORY,
      TestController.prototype,
      "upload",
    );

    expect(meta).toBeDefined();
    expect(meta.factory).toBe(factory);
    expect(meta.parameterIndex).toBe(0);
  });

  it("stores correct parameterIndex for non-first parameter", () => {
    const factory = () => ({});

    class TestController {
      upload(
        _req: any,
        @FormDataBody(factory) body: any,
      ) {}
    }

    const meta = Reflect.getMetadata(
      TSGONEST_FORM_DATA_FACTORY,
      TestController.prototype,
      "upload",
    );

    expect(meta.parameterIndex).toBe(1);
  });
});

describe("FormDataInterceptor", () => {
  it("calls next.handle() when no FormDataBody metadata exists", async () => {
    const interceptor = new FormDataInterceptor();
    const mockResult = { pipe: () => {} };
    const context = {
      getHandler: () => ({ name: "someMethod" }),
      getClass: () => ({ prototype: {} }),
    };
    const next = {
      handle: () => mockResult,
    };

    const result = await interceptor.intercept(context, next);
    expect(result).toBe(mockResult);
  });

  it("runs multer and merges single file into body as web File", async () => {
    const interceptor = new FormDataInterceptor();

    // Mock multer file with buffer (what multer actually produces)
    const mockFile = {
      fieldname: "avatar",
      originalname: "photo.jpg",
      mimetype: "image/jpeg",
      buffer: Buffer.from("fake-data"),
      size: 9,
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [mockFile];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = { body: { name: "test" }, files: undefined as any };
    const res = {};
    const mockResult = { pipe: () => {} };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => res,
      }),
    };
    const next = { handle: () => mockResult };

    const result = await interceptor.intercept(context, next);
    expect(result).toBe(mockResult);
    // File should be a web-native File instance (not raw multer object)
    expect(req.body.avatar).toBeInstanceOf(File);
    expect(req.body.avatar.name).toBe("photo.jpg");
    // Original text fields preserved
    expect(req.body.name).toBe("test");
  });

  it("merges multiple files with same field as array of web Files", async () => {
    const interceptor = new FormDataInterceptor();

    const file1 = {
      fieldname: "images",
      originalname: "a.jpg",
      mimetype: "image/jpeg",
      buffer: Buffer.from("img1"),
    };
    const file2 = {
      fieldname: "images",
      originalname: "b.jpg",
      mimetype: "image/jpeg",
      buffer: Buffer.from("img2"),
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [file1, file2];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = { body: {}, files: undefined as any };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => {},
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });
    expect(req.body.images).toHaveLength(2);
    expect(req.body.images[0]).toBeInstanceOf(File);
    expect(req.body.images[1]).toBeInstanceOf(File);
  });

  it("converts single multer file to web-native File instance", async () => {
    const interceptor = new FormDataInterceptor();

    // Simulate Express.Multer.File with buffer (what multer actually produces)
    const multerFile = {
      fieldname: "avatar",
      originalname: "photo.jpg",
      mimetype: "image/jpeg",
      buffer: Buffer.from("fake-image-data"),
      size: 15,
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [multerFile];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = { body: {}, files: undefined as any };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => {},
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    // The file should be converted to a web-native File instance
    expect(req.body.avatar).toBeInstanceOf(File);
    expect(req.body.avatar.name).toBe("photo.jpg");
    expect(req.body.avatar.type).toBe("image/jpeg");
    expect(req.body.avatar.size).toBe(15);
  });

  it("converts multiple multer files to web-native File instances", async () => {
    const interceptor = new FormDataInterceptor();

    const file1 = {
      fieldname: "images",
      originalname: "a.jpg",
      mimetype: "image/jpeg",
      buffer: Buffer.from("img1"),
      size: 4,
    };
    const file2 = {
      fieldname: "images",
      originalname: "b.png",
      mimetype: "image/png",
      buffer: Buffer.from("img2"),
      size: 4,
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [file1, file2];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = { body: {}, files: undefined as any };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => {},
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    // Both files should be web-native File instances
    expect(req.body.images).toHaveLength(2);
    expect(req.body.images[0]).toBeInstanceOf(File);
    expect(req.body.images[0].name).toBe("a.jpg");
    expect(req.body.images[0].type).toBe("image/jpeg");
    expect(req.body.images[1]).toBeInstanceOf(File);
    expect(req.body.images[1].name).toBe("b.png");
    expect(req.body.images[1].type).toBe("image/png");
  });

  it("converted files pass instanceof File check (for validation)", async () => {
    const interceptor = new FormDataInterceptor();

    const multerFile = {
      fieldname: "document",
      originalname: "report.pdf",
      mimetype: "application/pdf",
      buffer: Buffer.from("pdf-content"),
      size: 11,
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [multerFile];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = { body: {}, files: undefined as any };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => {},
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    // This is the critical check — generated assert functions use `instanceof File`
    // Raw multer objects would fail this check
    const file = req.body.document;
    expect(file instanceof File).toBe(true);
  });

  it("preserves file content through conversion", async () => {
    const interceptor = new FormDataInterceptor();

    const content = "hello world";
    const multerFile = {
      fieldname: "file",
      originalname: "test.txt",
      mimetype: "text/plain",
      buffer: Buffer.from(content),
      size: content.length,
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [multerFile];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = { body: {}, files: undefined as any };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => {},
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    // Verify the file content is preserved through conversion
    const file = req.body.file as File;
    const text = await file.text();
    expect(text).toBe(content);
  });

  it("uses native web FormData parsing on Node 20+ (no multer needed)", async () => {
    // Build a real multipart payload using web APIs (same as undici/fetch produces)
    const formData = new FormData();
    const file = new File(["hello world"], "test.txt", { type: "text/plain" });
    formData.append("avatar", file, file.name);
    formData.append("title", "My Upload");
    formData.append("category", "42");

    // Serialize via web Request to get exact content-type with boundary
    const serializer = new Request("http://localhost/", { method: "POST", body: formData });
    const contentType = serializer.headers.get("content-type")!;
    const rawBody = Buffer.from(await serializer.arrayBuffer());

    // Create a mock IncomingMessage-like readable stream
    const stream = Readable.from(rawBody) as any;
    stream.headers = { "content-type": contentType };
    stream.method = "POST";
    stream.url = "/upload";

    const interceptor = new FormDataInterceptor();
    const mockFactory = () => ({ any: () => () => {} }); // unused — native path takes priority

    class TestController {
      upload(@FormDataBody(mockFactory) _body: any) {}
    }

    const mockResult = { pipe: () => {} };
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => stream,
        getResponse: () => ({}),
      }),
    };
    const next = { handle: () => mockResult };

    const result = await interceptor.intercept(context, next);
    expect(result).toBe(mockResult);

    // Verify fields were parsed
    expect(stream.body.title).toBe("My Upload");
    expect(stream.body.category).toBe("42");

    // Verify file was parsed as web-native File
    expect(stream.body.avatar).toBeInstanceOf(File);
    expect(stream.body.avatar.name).toBe("test.txt");
    expect(stream.body.avatar.type).toBe("text/plain");
    const text = await stream.body.avatar.text();
    expect(text).toBe("hello world");
  });

  it("native parsing handles multiple files with same field name", async () => {
    const formData = new FormData();
    formData.append("images", new File(["img1"], "a.png", { type: "image/png" }), "a.png");
    formData.append("images", new File(["img2"], "b.png", { type: "image/png" }), "b.png");
    formData.append("albumName", "Vacation");

    const serializer = new Request("http://localhost/", { method: "POST", body: formData });
    const contentType = serializer.headers.get("content-type")!;
    const rawBody = Buffer.from(await serializer.arrayBuffer());

    const stream = Readable.from(rawBody) as any;
    stream.headers = { "content-type": contentType };
    stream.method = "POST";
    stream.url = "/upload";

    const interceptor = new FormDataInterceptor();
    const mockFactory = () => ({ any: () => () => {} });

    class TestController {
      upload(@FormDataBody(mockFactory) _body: any) {}
    }

    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => stream,
        getResponse: () => ({}),
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    expect(stream.body.albumName).toBe("Vacation");
    expect(stream.body.images).toHaveLength(2);
    expect(stream.body.images[0]).toBeInstanceOf(File);
    expect(stream.body.images[0].name).toBe("a.png");
    expect(stream.body.images[1]).toBeInstanceOf(File);
    expect(stream.body.images[1].name).toBe("b.png");
  });

  it("parses via rawBody when stream is already ended (Node 24 body-parser scenario)", async () => {
    // Simulate Node 24 + Express: body-parser middleware runs before the interceptor,
    // consuming/ending the stream. The raw buffer is stored on req.rawBody via the
    // verify callback (common NestJS pattern).
    const formData = new FormData();
    const file = new File(["avatar-data"], "photo.png", { type: "image/png" });
    formData.append("avatar", file, file.name);
    formData.append("username", "test-user");

    const serializer = new Request("http://localhost/", { method: "POST", body: formData });
    const contentType = serializer.headers.get("content-type")!;
    const rawBody = Buffer.from(await serializer.arrayBuffer());

    // Create a stream that's already ended (simulating body-parser consuming it)
    const stream = new Readable({ read() {} }) as any;
    stream.push(null);
    const endPromise = new Promise<void>(resolve => stream.on("end", resolve));
    stream.resume(); // switch to flowing mode so 'end' fires
    await endPromise;
    expect(stream.readableEnded).toBe(true);

    stream.headers = { "content-type": contentType };
    stream.method = "POST";
    stream.url = "/upload";
    stream.rawBody = rawBody; // set by bodyParser verify callback

    const interceptor = new FormDataInterceptor();

    class TestController {
      upload(@FormDataBody() _body: any) {}
    }

    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => stream,
        getResponse: () => ({}),
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    expect(stream.body.username).toBe("test-user");
    expect(stream.body.avatar).toBeInstanceOf(File);
    expect(stream.body.avatar.name).toBe("photo.png");
    expect(stream.body.avatar.type).toBe("image/png");
    const text = await stream.body.avatar.text();
    expect(text).toBe("avatar-data");
  });

  it("parses from internal buffer when stream ended but no rawBody", async () => {
    // Simulate Node 24 where undici eagerly buffers the body and emits 'end',
    // but no body-parser verify callback stored rawBody.
    const formData = new FormData();
    formData.append("name", "buffered-test");
    formData.append("doc", new File(["doc-content"], "file.txt", { type: "text/plain" }), "file.txt");

    const serializer = new Request("http://localhost/", { method: "POST", body: formData });
    const contentType = serializer.headers.get("content-type")!;
    const rawBody = Buffer.from(await serializer.arrayBuffer());

    // Readable.from pushes all chunks into the internal buffer and ends
    // the source — readableEnded becomes true, but read() still returns data.
    const stream = Readable.from(rawBody) as any;
    await new Promise(resolve => setTimeout(resolve, 10));

    stream.headers = { "content-type": contentType };
    stream.method = "POST";
    stream.url = "/upload";

    const interceptor = new FormDataInterceptor();

    class TestController {
      upload(@FormDataBody() _body: any) {}
    }

    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => stream,
        getResponse: () => ({}),
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    expect(stream.body.name).toBe("buffered-test");
    expect(stream.body.doc).toBeInstanceOf(File);
    expect(stream.body.doc.name).toBe("file.txt");
  });

  it("falls through to multer when stream ended and no data available", async () => {
    // Edge case: stream ended, no rawBody, no buffered data — should fall through
    // to multer without crashing.
    const stream = new Readable({ read() {} }) as any;
    stream.push(null);
    const endPromise = new Promise<void>(resolve => stream.on("end", resolve));
    stream.resume();
    await endPromise;

    stream.headers = { "content-type": "multipart/form-data; boundary=---test" };
    stream.method = "POST";
    stream.url = "/upload";

    const multerFile = {
      fieldname: "file",
      originalname: "test.txt",
      mimetype: "text/plain",
      buffer: Buffer.from("multer-fallback"),
    };
    const mockMulter = {
      any: () => (req: any, _res: any, cb: (err: any) => void) => {
        req.files = [multerFile];
        cb(null);
      },
    };
    const factory = () => mockMulter;

    const interceptor = new FormDataInterceptor();

    class TestController {
      upload(@FormDataBody(factory) _body: any) {}
    }

    const req = Object.assign(stream, { body: undefined, files: undefined });
    const context = {
      getHandler: () => TestController.prototype.upload,
      getClass: () => TestController,
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => ({}),
      }),
    };

    await interceptor.intercept(context, { handle: () => ({}) });

    // Multer fallback should have been used
    expect(req.body.file).toBeInstanceOf(File);
    expect(req.body.file.name).toBe("test.txt");
  });
});
