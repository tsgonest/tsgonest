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

  it("runs multer and merges single file into body", async () => {
    const interceptor = new FormDataInterceptor();

    // Mock multer that simulates file parsing
    const mockFile = { fieldname: "avatar", originalname: "photo.jpg" };
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
    // File should be merged into body as single value (not array)
    expect(req.body.avatar).toBe(mockFile);
    // Original text fields preserved
    expect(req.body.name).toBe("test");
  });

  it("merges multiple files with same field as array", async () => {
    const interceptor = new FormDataInterceptor();

    const file1 = { fieldname: "images", originalname: "a.jpg" };
    const file2 = { fieldname: "images", originalname: "b.jpg" };
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
    expect(req.body.images).toEqual([file1, file2]);
  });
});
