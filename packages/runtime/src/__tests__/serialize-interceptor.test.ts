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
});
