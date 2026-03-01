// Test fixture: controller methods returning primitive types.
// These must be JSON-encoded in the response body.
// nestia/typia handle this via typia.json.stringify<T> which wraps:
//   string → "\"hello\""
//   number → "42"
//   boolean → "true"
//
// Without this, the response Content-Type is application/json but the body
// is a raw value, causing SDK clients' response.json() to throw SyntaxError.

function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Body(): ParameterDecorator {
  return () => {};
}
function Param(name: string): ParameterDecorator {
  return () => {};
}

// DTO for body params
export interface ForgotPasswordDto {
  email: string;
}

@Controller("test")
export class PrimitiveReturnController {
  // Returns a plain string — must be JSON-encoded as "\"message\""
  @Post("forgot-password")
  async forgotPassword(@Body() body: ForgotPasswordDto): Promise<string> {
    return "If an account with that email exists, a password reset link has been sent.";
  }

  // Returns a plain string with no body param
  @Get("version")
  async getVersion(): Promise<string> {
    return "1.0.0";
  }

  // Returns a plain number — must be JSON-encoded as "42"
  @Get("count")
  async getCount(): Promise<number> {
    return 42;
  }

  // Returns a plain boolean — must be JSON-encoded as "true" or "false"
  @Get("enabled")
  async isEnabled(): Promise<boolean> {
    return true;
  }

  // Returns string | null — must handle null case
  @Get("display-name")
  async getDisplayName(): Promise<string | null> {
    return null;
  }

  // Returns a number literal union — should still be serialized
  @Get("status-code")
  async getStatusCode(): Promise<200 | 404 | 500> {
    return 200;
  }
}
