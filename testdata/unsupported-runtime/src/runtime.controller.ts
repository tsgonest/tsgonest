function Controller(path: string): ClassDecorator {
  return (target) => target;
}

function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

function Sse(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

const dynamicRoute = "dynamic";
const dynamicSseRoute = "events";
const dynamicControllerPath = "dynamic-controller";

@Controller("static")
export class StaticController {
  @Get("ok")
  ok(): string {
    return "ok";
  }

  @Get(dynamicRoute)
  skippedDynamicRoute(): string {
    return "nope";
  }

  @Sse(dynamicSseRoute)
  skippedDynamicSseRoute(): string {
    return "nope";
  }
}

@Controller(dynamicControllerPath)
export class DynamicControllerPathController {
  @Get("ok")
  skippedByDynamicControllerPath(): string {
    return "nope";
  }
}

export const makeRuntimeController = (prefix: string) => {
  @Controller(prefix)
  class RuntimeController {
    @Get("inside")
    skippedRuntimeControllerRoute(): string {
      return "nope";
    }
  }

  return RuntimeController;
};
