// Stub decorators for testing. In a real project, these come from @nestjs/common.

export function Controller(path: string): ClassDecorator {
  return (target) => target;
}
export function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Put(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Delete(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Patch(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Head(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Param(name: string): ParameterDecorator {
  return () => {};
}
export function Body(): ParameterDecorator {
  return () => {};
}
export function Query(): ParameterDecorator {
  return () => {};
}
export function Headers(): ParameterDecorator {
  return () => {};
}
export function HttpCode(code: number): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
export function Version(version: string | string[]): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
