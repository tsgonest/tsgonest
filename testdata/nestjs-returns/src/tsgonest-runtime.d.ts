declare module "@tsgonest/runtime" {
  export interface ReturnsOptions {
    contentType?: string;
    description?: string;
    status?: number;
  }
  export function Returns<T>(options?: ReturnsOptions): MethodDecorator;
}
