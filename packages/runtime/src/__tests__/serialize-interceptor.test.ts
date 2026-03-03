import { describe, it, expect } from "vitest";
import { TsgonestSerializeInterceptor } from "../serialize-interceptor";

describe("TsgonestSerializeInterceptor", () => {
  it("sets Content-Type to application/json", () => {
    const interceptor = new TsgonestSerializeInterceptor();
    const headers: Record<string, string> = {};

    const context = {
      switchToHttp: () => ({
        getResponse: () => ({
          header: (key: string, value: string) => {
            headers[key] = value;
          },
        }),
      }),
    };

    const handlerResult = { pipe: () => {} };
    const next = { handle: () => handlerResult };

    const result = interceptor.intercept(context, next);

    expect(headers["Content-Type"]).toBe("application/json; charset=utf-8");
    expect(result).toBe(handlerResult);
  });

  it("sets header before handler executes", () => {
    const interceptor = new TsgonestSerializeInterceptor();
    let headerSetBeforeHandle = false;
    let headerWasSet = false;

    const context = {
      switchToHttp: () => ({
        getResponse: () => ({
          header: () => {
            headerWasSet = true;
          },
        }),
      }),
    };

    const next = {
      handle: () => {
        headerSetBeforeHandle = headerWasSet;
        return {};
      },
    };

    interceptor.intercept(context, next);
    expect(headerSetBeforeHandle).toBe(true);
  });

  it("does not interfere with handler return value", () => {
    const interceptor = new TsgonestSerializeInterceptor();
    const context = {
      switchToHttp: () => ({
        getResponse: () => ({
          header: () => {},
        }),
      }),
    };

    // Simulate a pre-serialized JSON string (what stringify returns)
    const jsonString = '{"id":1,"name":"Alice"}';
    const observable = { subscribe: () => {}, pipe: () => jsonString };
    const next = { handle: () => observable };

    const result = interceptor.intercept(context, next);
    expect(result).toBe(observable);
  });
});
