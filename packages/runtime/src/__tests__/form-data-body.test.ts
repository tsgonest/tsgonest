import { describe, it, expect } from "vitest";
import "reflect-metadata";
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
});
